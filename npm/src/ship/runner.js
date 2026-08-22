'use strict'

/**
 * ship/runner.js — Core implementation of `trackfw ship`.
 *
 * All git write operations are injectable for testability.
 * Never passes "add ." or "add -A" to any git executor.
 */

const { spawnSync } = require('child_process')
const { load: loadConfig, reset: resetConfig } = require('../config')
const { resolve: forgeResolve } = require('../forge/resolve')
const { forgeAdapter } = require('../forge/adapter')
const validator = require('../validator')
const { evaluateBranchIntegration, DECISION: BRANCH_PRUNE_DECISION } = require('../branch/prune')

// Git subcommands that modify local or remote state.
// In --dry-run mode these are printed but not executed.
const GIT_WRITE_COMMANDS = new Set(['commit', 'push', 'fetch'])

// force-with-lease refusal messages. Named constants so the ML-1B parity gate has a single
// place to compare byte-for-byte across the 3 CLIs. Byte-identical to Go's
// forceLease*Msg/Fmt constants (internal/commands/ship.go) and Python's FORCE_LEASE_* below.
// Never expose "--force" (raw) as a flag — see
// ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.
const FORCE_LEASE_NO_FORGE_CLI_MSG =
  'trackfw ship --force-with-lease requires a forge CLI (gh, glab, or az) to confirm an open pull/merge request before rewriting remote history. No forge CLI is available for this repository — install and authenticate it, or push without --force-with-lease.'

function forceLeaseNoPROpenMsg(branch) {
  return `trackfw ship --force-with-lease refuses to run: branch "${branch}" has no open pull/merge request. Open the PR/MR first (trackfw ship without --force-with-lease, or your forge's web UI), then retry.`
}

function forceLeaseCannotVerifyMsg(branch, cliName, errMessage) {
  return `trackfw ship --force-with-lease could not verify whether branch "${branch}" has an open pull/merge request (${cliName} CLI error: ${errMessage}). Refusing rather than risking a force push without a verified PR — check your ${cliName} CLI authentication and retry.`
}

/**
 * defaultCheckPROpen queries the resolved forge CLI for an open PR/MR whose source branch is
 * `branch`, using the same list-based query shape for every forge: empty result means "no PR"
 * (exit 0), any non-zero exit or unparseable output means "cannot verify" (thrown — never
 * conflated with "no PR"). bitbucket and "manual" never reach here: runShip only calls
 * checkPROpen when adapter.available is true, and bitbucket's adapter is always available=false.
 * @param {object} adapter
 * @param {string} branch
 * @returns {boolean}
 */
function defaultCheckPROpen(adapter, branch) {
  let args
  switch (adapter.forge) {
    case 'github':
      args = ['pr', 'list', '--head', branch, '--state', 'open', '--json', 'number']
      break
    case 'gitlab':
      // glab mr list: --source-branch filters by source branch, --state opened matches
      // gh's "open" state, -F json requests machine-readable output (glab's own flag,
      // not an external jq/GNU dependency).
      args = ['mr', 'list', '--source-branch', branch, '--state', 'opened', '-F', 'json']
      break
    case 'azure':
      // az defaults to --output json; passed explicitly here for clarity, not reliance
      // on the ambient default.
      args = ['repos', 'pr', 'list', '--source-branch', branch, '--status', 'active', '--output', 'json']
      break
    default:
      throw new Error(`no PR/MR query defined for forge "${adapter.forge}"`)
  }

  const result = spawnSync(adapter.cliName, args, { encoding: 'utf8' })
  if (result.status !== 0) {
    const msg = (result.stderr || '').trim() || `${adapter.cliName} exited with ${result.status}`
    throw new Error(msg)
  }

  let parsed
  try {
    parsed = JSON.parse(result.stdout || '[]')
  } catch (e) {
    throw new Error(`could not parse ${adapter.cliName} output: ${e.message}`)
  }
  return Array.isArray(parsed) && parsed.length > 0
}

/**
 * isGitWriteCmd returns true when the first arg is a write-mode git subcommand.
 * @param {string[]} args
 * @returns {boolean}
 */
function isGitWriteCmd(args) {
  return args.length > 0 && GIT_WRITE_COMMANDS.has(args[0])
}

/**
 * isShipBranch returns true when branch matches feat|fix|refactor|chore|docs/<slug> — the full
 * vocabulary `trackfw ship` accepts on the branch name. feat/fix/refactor are gated on Step 2's
 * branch_has_wip_roadmap governance check (a hard gate not affected by lenient mode); chore/docs
 * are housekeeping types — already exempted from that gate by `trackfw branch new` and
 * `trackfw commit` — and ship without it too.
 * @param {string} branch
 * @returns {boolean}
 */
function isShipBranch(branch) {
  return /^(feat|fix|refactor|chore|docs)\/.+/.test(branch)
}

/**
 * isGatedShipBranch returns true when branch matches feat|fix|refactor/<slug> — the subset of
 * isShipBranch's vocabulary that requires Step 2's branch_has_wip_roadmap governance check.
 * chore/docs branches satisfy isShipBranch but return false here.
 * @param {string} branch
 * @returns {boolean}
 */
function isGatedShipBranch(branch) {
  return /^(feat|fix|refactor)\/.+/.test(branch)
}

/**
 * defaultExecGit runs git with the provided args and returns { stdout, error }.
 * @param {string[]} args
 * @returns {{ stdout: string, error: Error|null }}
 */
function defaultExecGit(args) {
  const result = spawnSync('git', args, { encoding: 'utf8' })
  if (result.status !== 0) {
    const msg = (result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`
    return { stdout: '', error: new Error(msg) }
  }
  return { stdout: (result.stdout || '').trim(), error: null }
}

/**
 * defaultCheckGovernance returns violation messages from the governance gate.
 * Uses validator equivalent: looks for wip roadmap and req link in the filesystem.
 * Returns [] when governance passes.
 * @returns {string[]}
 */
function defaultCheckGovernance() {
  return checkShipGovernance()
}

/**
 * checkShipGovernance — hard gate (bypasses config/baseline/lenient).
 *
 * Delegates entirely to the shared validator functions already used by `trackfw validate`,
 * `trackfw branch new` and `trackfw commit` — never reimplement this logic locally. Byte-
 * identical to Go's CheckShipGovernance (internal/validator/validator.go), which has the same
 * no-args shape and the same two checks:
 *   1. validateBranchHasWIPRoadmap — current branch (feat/fix/refactor only; re-derived
 *      internally from TRACKFW_BRANCH/git, same as Go) has a matching roadmap in wip/ OR done/
 *   2. validateWIPHasREQ — every roadmap in wip/ has a linked REQ
 * Before this ML, this function reimplemented both checks locally with its own wording (no
 * "nor done/" clause, no done/ directory scan at all) — see
 * vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md. Duplicating
 * the check was the actual root cause of the drift, not just the wording.
 * @returns {string[]} violation messages
 */
function checkShipGovernance() {
  return [
    ...validator.validateBranchHasWIPRoadmap(),
    ...validator.validateWIPHasREQ(),
  ]
}

/**
 * resolveRoadmapDir delegates to config.load() — single source of truth for roadmap_dir.
 * Accepts an optional cwd for testability (passed through to config.load).
 * Default when no trackfw.yaml is present: docs/roadmaps.
 * @param {string} [cwd]
 * @returns {string}
 */
function resolveRoadmapDir(cwd) {
  return loadConfig(cwd).roadmapDir
}

/**
 * normalizeBranchSlug converts a string to a lowercase dash-only slug
 * (same algorithm as Go's normalizeBranchSlug).
 * @param {string} value
 * @returns {string}
 */
function normalizeBranchSlug(value) {
  let out = ''
  let lastDash = false
  for (const ch of value.toLowerCase()) {
    if (/[a-z0-9]/.test(ch)) {
      out += ch
      lastDash = false
    } else if (!lastDash) {
      out += '-'
      lastDash = true
    }
  }
  return out.replace(/^-|-$/g, '')
}

/**
 * detectPendingSquashMerges warns about remote branches with genuinely pending work vs
 * origin/main. Non-blocking.
 *
 * Reuses evaluateBranchIntegration (branch/prune.js) — the same touched-files heuristic
 * `trackfw branch prune` uses — instead of a naive bidirectional `git diff origin/main <branch>
 * --stat`. The naive check false-positives on a branch that was squash-merged and is now merely
 * stale (main advanced further afterwards): it always shows a non-empty diff even though nothing
 * from the branch is actually missing from main. Only DECISION.PENDING_WORK — genuine,
 * unintegrated work — surfaces this warning; every other decision (no_own_work,
 * content_identical, review_doc_config, no_merge_base, eval_error) stays silent, the same
 * posture the naive check had on error (skip, no warning).
 * @param {string} currentBranch
 * @param {function} execGit
 * @param {function} writeln
 */
function detectPendingSquashMerges(currentBranch, execGit, writeln) {
  const { stdout, error } = execGit(['branch', '-r', '--no-merged', 'origin/main'])
  if (error || !stdout.trim()) return

  for (const raw of stdout.split('\n')) {
    const candidate = raw.trim()
    if (!candidate || candidate.includes('HEAD')) continue
    const shortName = candidate.replace(/^origin\//, '')
    if (shortName === currentBranch) continue

    const evalResult = evaluateBranchIntegration(candidate, execGit)
    if (evalResult.decision === BRANCH_PRUNE_DECISION.PENDING_WORK) {
      writeln(`Warning: branch "${shortName}" appears to have unmerged changes vs origin/main.`)
    }
  }
}

/**
 * buildPushArgs returns the push args, adding -u if no upstream is configured.
 * @param {string} branch
 * @param {function} execGit
 * @returns {string[]}
 */
function buildPushArgs(branch, execGit) {
  const { error } = execGit(['rev-parse', '--abbrev-ref', '--symbolic-full-name', '@{u}'])
  if (error) {
    return ['push', '-u', 'origin', branch]
  }
  return ['push', 'origin', branch]
}

/**
 * defaultExecForgeCLI invokes a forge CLI (gh, glab, az) inheriting stdio.
 * @param {string} name
 * @param {string[]} args
 * @returns {Error|null}
 */
function defaultExecForgeCLI(name, args) {
  const result = spawnSync(name, args, { stdio: 'inherit' })
  if (result.status !== 0) {
    return new Error(`${name} exited with ${result.status}`)
  }
  return null
}

/**
 * firstLine returns only the first line of s.
 * @param {string} s
 * @returns {string}
 */
function firstLine(s) {
  const idx = s.indexOf('\n')
  return idx >= 0 ? s.slice(0, idx) : s
}

/**
 * splitNonEmptyLines splits git output (e.g. `diff --cached --name-only`) into an array
 * of trimmed, non-empty lines.
 * @param {string} s
 * @returns {string[]}
 */
function splitNonEmptyLines(s) {
  const trimmed = (s || '').trim()
  if (!trimmed) return []
  return trimmed
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
}

/**
 * allDocOnly returns true when there is at least one staged file and every staged file is
 * doc-only: under docs/ or vault/ (path prefix), or has a .md extension. A single file
 * outside that criterion makes it return false. Mirrors the doc-only exception documented
 * in CLAUDE.md §7 ("Alteração doc-only (markdown, comentários)").
 * @param {string[]} files
 * @returns {boolean}
 */
function allDocOnly(files) {
  if (!files || files.length === 0) return false
  for (const f of files) {
    if (f.startsWith('docs/') || f.startsWith('vault/') || f.endsWith('.md')) continue
    return false
  }
  return true
}

/**
 * commitMessageSep delimits full commit messages (%B) in the output of gitCommitsSince's
 * `git log --format=%B<sep>`. Same non-printable separator used by the Go implementation
 * (internal/commands/ship.go) — cannot appear in a real commit message.
 */
const COMMIT_MESSAGE_SEP = '\x1e'

// GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX is the fixed prefix `git symbolic-ref
// refs/remotes/origin/HEAD` always returns before the branch name, because "origin" is the
// literal ref namespace queried — never derived from the output itself. Stripping this exact
// prefix (instead of cutting at the last '/') is what makes defaultBaseBranch correct for a
// default branch that itself contains a slash (e.g. "release/7.2"): lastIndexOf('/') used to
// cut at "7.2", discarding "release/". Mirrors Go's ship.go gitSymbolicRefOriginHeadPrefix.
const GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX = 'refs/remotes/origin/'

/**
 * defaultBaseBranch resolves the repository's default branch for `git log <base>..HEAD`.
 * Tries `git symbolic-ref refs/remotes/origin/HEAD` and falls back to "main" when that fails or
 * yields nothing.
 * @param {function} execGit
 * @returns {string}
 */
function defaultBaseBranch(execGit) {
  const { stdout, error } = execGit(['symbolic-ref', 'refs/remotes/origin/HEAD'])
  if (error) return 'main'
  const out = (stdout || '').trim()
  if (!out.startsWith(GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX)) return 'main'
  const name = out.slice(GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX.length)
  return name === '' ? 'main' : name
}

/**
 * gitCommitsSince returns the full message (subject + body) of every non-merge commit in
 * base..HEAD, most-recent-first (git log's natural order). Returns [] on any git error or
 * when the range is empty.
 * @param {string} base
 * @param {function} execGit
 * @returns {string[]}
 */
function gitCommitsSince(base, execGit) {
  const { stdout, error } = execGit(['log', `${base}..HEAD`, '--no-merges', `--format=%B${COMMIT_MESSAGE_SEP}`])
  if (error) return []
  const out = (stdout || '').trim()
  if (!out) return []
  const parts = out.split(COMMIT_MESSAGE_SEP)
  const commits = []
  for (let p of parts) {
    p = p.replace(/^\n+|\n+$/g, '')
    if (p.trim() === '') continue
    commits.push(p)
  }
  return commits
}

/**
 * buildPRBody constructs the PR/MR body. With 0 or 1 non-merge commit on the branch (the
 * trivial case — just the commit `ship` itself made), it keeps the original minimal body,
 * not a regression. With 2+ commits, it aggregates the branch's commit history:
 *
 *   ## Commits
 *   - <subject of commit 1>
 *   - <subject of commit 2>
 *
 *   ## Detalhes
 *   <full body of each commit that has one, in blocks>
 *
 *   ---
 *   Branch: <branch>
 *
 * @param {string} branch
 * @param {string[]} commits
 * @returns {string}
 */
function buildPRBody(branch, commits) {
  if (!commits || commits.length <= 1) {
    return `Branch: ${branch}\n\nCreated by trackfw ship.`
  }

  const subjects = []
  const details = []
  for (const c of commits) {
    const nlIdx = c.indexOf('\n')
    const subject = (nlIdx >= 0 ? c.slice(0, nlIdx) : c).trim()
    if (subject === '') continue
    subjects.push(subject)
    if (nlIdx >= 0) {
      const bodyText = c.slice(nlIdx + 1).trim()
      if (bodyText !== '') {
        details.push(`**${subject}**\n\n${bodyText}`)
      }
    }
  }

  let b = '## Commits\n\n'
  for (const s of subjects) {
    b += `- ${s}\n`
  }
  if (details.length > 0) {
    b += '\n## Detalhes\n\n'
    b += details.join('\n\n')
    b += '\n'
  }
  b += `\n---\nBranch: ${branch}\n`
  return b
}

/**
 * buildForgeCreateArgs appends --title and --body (or --description for azure)
 * to a copy of adapter.cliArgs. Never mutates the original array.
 * @param {object} adapter
 * @param {string} title
 * @param {string} body
 * @returns {string[]}
 */
function buildForgeCreateArgs(adapter, title, body) {
  const args = [...adapter.cliArgs, '--title', title]
  if (adapter.forge === 'azure') {
    args.push('--description', body)
  } else {
    args.push('--body', body)
  }
  return args
}

/**
 * runShip executes the seven-step ship sequence.
 *
 * @param {{ message: string, dryRun: boolean, noPR?: boolean, forge?: string }} opts
 * @param {{ execGit?: function, checkGovernance?: function, writeln?: function,
 *           configForge?: string, repoDir?: string,
 *           availFn?: function, execForgeCLI?: function }} deps
 * @returns {number} exit code (0 = success)
 */
function runShip(opts, deps = {}) {
  const execGit = deps.execGit || defaultExecGit
  const checkGovernanceFn = deps.checkGovernance || defaultCheckGovernance
  const writeln = deps.writeln || ((s) => process.stdout.write(s + '\n'))
  // writeErr — the terminal error line for every abort path below, written to STDERR with the
  // "Error: " prefix. Mirrors Go exactly: ship.go returns these same messages as a bare
  // fmt.Errorf(...), and internal/commands/root.go's Execute() wrapper prints them to stderr as
  // `fmt.Fprintln(os.Stderr, cmd.ErrPrefix(), err.Error())` — cmd.ErrPrefix() is "Error:", and
  // Fprintln inserts exactly one space between operands, so the wire format is
  // "Error: <message>\n". Every multi-line detail printed BEFORE the abort (violation lists,
  // remediation hints, "Note: ..." blocks) stays on stdout via writeln, same as Go's deps.out —
  // only this final one-line (or one-block) summary moves to stderr.
  const writeErr = deps.writeErr || ((s) => process.stderr.write(`Error: ${s}\n`))

  // Inner git wrapper: skips write commands in dry-run mode.
  function git(args) {
    if (opts.dryRun && isGitWriteCmd(args)) {
      writeln(`[dry-run] git ${args.join(' ')}`)
      return { stdout: '', error: null }
    }
    return execGit(args)
  }

  // ─── Step 0: staged files ───────────────────────────────────────────────────
  // Read once, up front, so Steps 1 and 2 can grant a doc-only exception before they run —
  // and so Step 4 below reuses the same read instead of querying git twice.
  const { stdout: stagedOut } = execGit(['diff', '--cached', '--name-only'])
  const stagedFiles = splitNonEmptyLines(stagedOut)
  const docOnly = allDocOnly(stagedFiles)

  // ─── Step 1: Branch validation ─────────────────────────────────────────────
  const branchResult = execGit(['symbolic-ref', '--short', 'HEAD'])
  if (branchResult.error) {
    writeErr(`could not determine current branch (are you in a git repo?): ${branchResult.error.message}`)
    return 1
  }
  const branch = branchResult.stdout.trim()

  // main/master is blocked unconditionally — the doc-only exception never applies here.
  if (branch === 'main' || branch === 'master') {
    writeErr(`trackfw ship cannot run on "${branch}" — use a feature branch:\n  git checkout -b feat/<slug>`)
    return 1
  }

  if (!docOnly && !isShipBranch(branch)) {
    writeErr(
      `branch "${branch}" does not match the required pattern feat|fix|refactor|chore|docs/<slug>\n` +
      'Rename your branch or create a new one:\n  git checkout -b feat/<slug>'
    )
    return 1
  }

  writeln(`Branch: ${branch}`)

  // ─── Step 2: Governance ────────────────────────────────────────────────────
  // Doc-only changes (all staged files under docs/, vault/, or *.md) are exempt from
  // REQ+roadmap governance — mirrors the CLAUDE.md §7 exception for doc-only changes.
  if (docOnly) {
    writeln('Governance: skipped (doc-only change)')
  } else if (isShipBranch(branch) && !isGatedShipBranch(branch)) {
    // chore/docs: housekeeping types already exempted from this gate by
    // `trackfw branch new` and `trackfw commit` — ship without it too.
    writeln('Governance: skipped (chore/docs branch)')
  } else {
    const violations = checkGovernanceFn()
    if (violations.length > 0) {
      writeln('\nGovernance check failed:')
      for (const v of violations) {
        writeln(`  ${v}`)
      }
      writeln('\nCreate the required artifacts before running ship:')
      writeln('  trackfw req new "<title>"')
      writeln('  trackfw roadmap new "<title>"')
      writeln('  trackfw roadmap move <name> wip')
      writeln("\nNote: this governance check is a hard gate — it is not affected by lenient")
      writeln("mode or per-rule severity configured in trackfw.yaml. If 'trackfw validate'")
      writeln("passes but 'trackfw ship' aborts here, you likely have lenient mode")
      writeln("configured — ship always requires REQ + roadmap in wip/.")
      writeErr(`governance check failed: ${violations.length} violation(s)`)
      return 1
    }

    writeln('Governance: OK')
  }

  // ─── Step 2.5: force-with-lease gate ───────────────────────────────────────
  // Runs before any write (commit/push) — a refusal here must never leave a local
  // commit the caller cannot push. Read-only, so it runs in --dry-run too, same
  // posture as the read-only calls in Step 0 / Step 7.
  //
  // forceLeaseAdapter/forceLeaseResolution are reused by Step 7 below to avoid a
  // second forge resolution and a duplicate "Forge: ..." line, and because Step 7
  // must skip PR/MR creation entirely once this gate has confirmed one is already
  // open — creating a second one would be a spurious failure on every successful
  // force push.
  let forceLeaseAdapter = null
  let forceLeaseResolution = null
  if (opts.forceWithLease) {
    const { stdout: gateRemoteURLRaw } = execGit(['remote', 'get-url', 'origin'])
    const gateRemoteURL = (gateRemoteURLRaw || '').trim()

    let resolution
    try {
      resolution = forgeResolve({
        flagForge: opts.forge || '',
        configForge: deps.configForge || '',
        remoteURL: gateRemoteURL,
        repoDir: deps.repoDir || '',
      })
    } catch (resErr) {
      writeErr(resErr.message)
      return 1
    }

    const adapter = forgeAdapter(resolution.forge, deps.availFn || undefined)
    if (resolution.forge === 'manual' || !adapter.available) {
      writeErr(FORCE_LEASE_NO_FORGE_CLI_MSG)
      return 1
    }

    const checkPROpen = deps.checkPROpen || defaultCheckPROpen
    let open
    try {
      open = checkPROpen(adapter, branch)
    } catch (prErr) {
      writeErr(forceLeaseCannotVerifyMsg(branch, adapter.cliName, prErr.message))
      return 1
    }
    if (!open) {
      writeErr(forceLeaseNoPROpenMsg(branch))
      return 1
    }

    writeln(`force-with-lease: open ${adapter.noun} confirmed for "${branch}" (${resolution.forge}).`)
    forceLeaseAdapter = adapter
    forceLeaseResolution = resolution
  }

  // ─── Step 3: Squash-merge detection ────────────────────────────────────────
  if (opts.dryRun) {
    writeln('[dry-run] git fetch origin --prune')
  } else {
    const { error: fetchErr } = execGit(['fetch', 'origin', '--prune'])
    if (fetchErr) {
      writeln('Warning: could not fetch origin (offline or no remote); skipping squash-merge check.')
    } else {
      detectPendingSquashMerges(branch, execGit, writeln)
    }
  }

  // ─── Step 4: Review staged ─────────────────────────────────────────────────
  const { stdout: statusOut } = execGit(['status', '--short'])
  const { stdout: diffStatOut } = execGit(['diff', '--cached', '--stat'])

  writeln('\n── Staged changes ──────────────────────────────────────')
  if (statusOut) writeln(statusOut)
  if (diffStatOut) writeln(diffStatOut)
  writeln('────────────────────────────────────────────────────────\n')

  // Reuses stagedFiles read at the top of the function (Step 0) — never re-query git here.
  //
  // --force-with-lease push-only mode: a rebase that resolved conflicts already
  // committed the result (the index is clean afterwards) — there is nothing left to
  // stage or commit, only to push. Only --force-with-lease grants this exception;
  // without it, "nothing staged" still aborts exactly as before (non-regression).
  const pushOnly = opts.forceWithLease && stagedFiles.length === 0

  if (stagedFiles.length === 0 && !opts.forceWithLease) {
    writeErr(
      'nothing is staged — stage your files explicitly before running ship:\n' +
      '  git add <file1> <file2> ...\n' +
      "Never use 'git add .' or 'git add -A'"
    )
    return 1
  }

  // ─── Step 5: Commit ────────────────────────────────────────────────────────
  if (pushOnly) {
    writeln('Nothing staged — --force-with-lease pushes existing commits only, no new commit.')
  } else {
    if (!opts.message) {
      writeErr(
        'commit message is required — use -m:\n' +
        '  trackfw ship -m "feat(<scope>): <description>"'
      )
      return 1
    }

    const { error: commitErr } = git(['commit', '-m', opts.message])
    if (commitErr) {
      writeErr(`git commit failed: ${commitErr.message}`)
      return 1
    }

    if (!opts.dryRun) {
      writeln(`Committed: ${opts.message}`)
    }
  }

  // ─── Step 6: Push ──────────────────────────────────────────────────────────
  let pushArgs = buildPushArgs(branch, execGit)
  if (opts.forceWithLease) {
    // Fixed position: push --force-with-lease [-u] origin <branch> — identical
    // across the 3 CLIs (ML-1B's parity gate compares this literally).
    pushArgs = [pushArgs[0], '--force-with-lease', ...pushArgs.slice(1)]
  }
  const { error: pushErr } = git(pushArgs)
  if (pushErr) {
    writeErr(`git push failed: ${pushErr.message}`)
    return 1
  }

  if (!opts.dryRun) {
    writeln(`Pushed:    ${branch} → origin/${branch}`)
  }

  // ─── Step 7: Open PR/MR ────────────────────────────────────────────────────
  // --force-with-lease only ever reaches here after Step 2.5 confirmed a PR/MR is
  // already open on this branch — creating another one would be a spurious failure
  // on every successful force push. Reuses the adapter/resolution Step 2.5 already
  // computed instead of resolving the forge a second time.
  if (opts.forceWithLease) {
    writeln(`Forge:     ${forceLeaseResolution.forge} (source: ${forceLeaseResolution.source})`)
    writeln(`${forceLeaseAdapter.noun} already open — skipping creation (--force-with-lease).`)
    writeln('\nship complete.')
    return 0
  }

  const { stdout: remoteURLRaw } = execGit(['remote', 'get-url', 'origin'])
  const remoteURL = (remoteURLRaw || '').trim()

  let resolution
  try {
    resolution = forgeResolve({
      flagForge: opts.forge || '',
      configForge: deps.configForge || '',
      remoteURL,
      repoDir: deps.repoDir || '',
    })
  } catch (resErr) {
    writeln(`Warning: forge resolution error: ${resErr.message} — open PR/MR manually.`)
    writeln('\nship complete.')
    return 0
  }

  const adapter = forgeAdapter(resolution.forge, deps.availFn || undefined)
  writeln(`Forge:     ${resolution.forge} (source: ${resolution.source})`)

  if (opts.noPR) {
    writeln(`--no-pr: skipping ${adapter.noun} creation.`)
    writeln('\nship complete.')
    return 0
  }

  // Title/body computed once for every remaining branch below (dry-run and real CLI
  // invocation alike). git log/diff are read-only — they run in --dry-run mode too, same
  // as the staged-files read in Step 0.
  //
  // Design decision (documented per roadmap ML-1A, mirrored from the Go implementation):
  // the title is always firstLine(opts.message), the -m message passed to this very `ship`
  // call, even when the branch carries multiple prior commits. Deriving a distinct "PR
  // title" from N unrelated commit subjects would need a heuristic with no unambiguous
  // answer — the simplest, least surprising rule is that -m is the PR's summary.
  const base = defaultBaseBranch(execGit)
  const commits = gitCommitsSince(base, execGit)
  const title = firstLine(opts.message || '')
  const body = buildPRBody(branch, commits)

  if (opts.dryRun) {
    writeln(`[dry-run] Title: ${title}`)
    writeln(`[dry-run] Body:\n${body}`)
    if (!adapter.available && resolution.forge !== 'manual') {
      const url = adapter.fallbackURL(remoteURL, branch)
      if (url) {
        writeln(`[dry-run] ${adapter.noun} CLI (${adapter.cliName}) not available — would open in browser:\n  ${url}`)
      } else {
        writeln(`[dry-run] ${adapter.noun} CLI (${adapter.cliName}) not available — would open ${adapter.noun} manually`)
      }
    } else {
      writeln(`[dry-run] would open ${adapter.noun} via ${resolution.forge} CLI`)
    }
    return 0
  }

  if (resolution.forge === 'manual') {
    writeln(`\nOpen your ${adapter.noun} manually at:\n  ${remoteURL}`)
    writeln('\nship complete.')
    return 0
  }

  if (!adapter.available) {
    const url = adapter.fallbackURL(remoteURL, branch)
    if (url) {
      writeln(`${adapter.noun} CLI (${adapter.cliName}) not available — open in browser:\n  ${url}`)
    } else {
      writeln(`${adapter.noun} CLI (${adapter.cliName}) not available — open ${adapter.noun} manually.`)
    }
    writeln('\nship complete.')
    return 0
  }

  // CLI is available — invoke it.
  const cliArgs = buildForgeCreateArgs(adapter, title, body)
  const execForgeCLI = deps.execForgeCLI || defaultExecForgeCLI
  const cliErr = execForgeCLI(adapter.cliName, cliArgs)
  if (cliErr) {
    const url = adapter.fallbackURL(remoteURL, branch)
    writeln(`Warning: ${adapter.noun} CLI failed (${cliErr.message}).`)
    if (url) writeln(`Open in browser:\n  ${url}`)
  } else {
    writeln(`${adapter.noun} created.`)
  }

  writeln('\nship complete.')
  return 0
}

module.exports = {
  runShip,
  isShipBranch,
  isGatedShipBranch,
  isGitWriteCmd,
  normalizeBranchSlug,
  checkShipGovernance,
  resolveRoadmapDir,
  resetConfig,
  GIT_WRITE_COMMANDS,
  buildForgeCreateArgs,
  firstLine,
  allDocOnly,
  splitNonEmptyLines,
  defaultBaseBranch,
  gitCommitsSince,
  buildPRBody,
  COMMIT_MESSAGE_SEP,
  detectPendingSquashMerges,
  defaultCheckPROpen,
  FORCE_LEASE_NO_FORGE_CLI_MSG,
  forceLeaseNoPROpenMsg,
  forceLeaseCannotVerifyMsg,
  // Exported for reuse by push/runner.js (AC2 of REQ-2026-08-22-trackfw-push).
  buildPushArgs,
  defaultExecGit,
}
