'use strict'

/**
 * ship/runner.js — Core implementation of `trackfw ship`.
 *
 * All git write operations are injectable for testability.
 * Never passes "add ." or "add -A" to any git executor.
 */

const { spawnSync } = require('child_process')
const fs = require('fs')
const path = require('path')
const { load: loadConfig, reset: resetConfig } = require('../config')
const { resolve: forgeResolve } = require('../forge/resolve')
const { forgeAdapter } = require('../forge/adapter')

// Git subcommands that modify local or remote state.
// In --dry-run mode these are printed but not executed.
const GIT_WRITE_COMMANDS = new Set(['commit', 'push', 'fetch'])

/**
 * isGitWriteCmd returns true when the first arg is a write-mode git subcommand.
 * @param {string[]} args
 * @returns {boolean}
 */
function isGitWriteCmd(args) {
  return args.length > 0 && GIT_WRITE_COMMANDS.has(args[0])
}

/**
 * isShipBranch returns true when branch matches feat|fix|refactor/<slug>.
 * @param {string} branch
 * @returns {boolean}
 */
function isShipBranch(branch) {
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
 * Checks:
 *   1. Current branch has a matching roadmap in wip/
 *   2. WIP roadmaps have a linked REQ
 * @returns {string[]} violation messages
 */
function checkShipGovernance() {
  const violations = []

  // Resolve roadmap dir via config module (single source of truth; default: docs/roadmaps)
  const roadmapDir = resolveRoadmapDir()
  const wipDir = path.join(roadmapDir, 'wip')

  // Get branch name
  const branchResult = defaultExecGit(['symbolic-ref', '--short', 'HEAD'])
  const branch = branchResult.error ? '' : branchResult.stdout.trim()

  // Check branch has matching roadmap in wip/
  if (branch && isShipBranch(branch)) {
    const slug = normalizeBranchSlug(branch.split('/').slice(1).join('/'))
    let hasMatch = false
    let wipFiles = []

    if (fs.existsSync(wipDir)) {
      wipFiles = fs.readdirSync(wipDir).filter(f => f.endsWith('.md'))
      for (const f of wipFiles) {
        if (normalizeBranchSlug(f).includes(slug)) {
          hasMatch = true
          break
        }
      }
    }

    if (wipFiles.length === 0) {
      violations.push(
        `branch "${branch}" is a feat/fix/refactor branch but no roadmap is in wip/ — create governance artifacts first:\n` +
        '  trackfw req new "<title>"\n' +
        '  trackfw roadmap new "<title>"\n' +
        '  trackfw roadmap move <name> wip'
      )
    } else if (!hasMatch) {
      violations.push(
        `branch "${branch}" has no matching roadmap in wip/ (found: ${wipFiles.join(', ')}) — ` +
        'include the branch slug in the roadmap filename'
      )
    }

    // Check WIP roadmaps have a linked REQ
    if (hasMatch && fs.existsSync(wipDir)) {
      for (const f of wipFiles) {
        const content = fs.readFileSync(path.join(wipDir, f), 'utf8')
        if (!content.includes('REQ:') && !content.includes('req:')) {
          violations.push(`roadmap "${f}" is in wip but has no linked REQ`)
        }
      }
    }
  }

  return violations
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
 * detectPendingSquashMerges warns about remote branches with non-empty diffs vs origin/main.
 * Non-blocking.
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

    const { stdout: diff, error: derr } = execGit(['diff', 'origin/main', candidate, '--stat'])
    if (derr) continue
    if (diff.trim()) {
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

  // Inner git wrapper: skips write commands in dry-run mode.
  function git(args) {
    if (opts.dryRun && isGitWriteCmd(args)) {
      writeln(`[dry-run] git ${args.join(' ')}`)
      return { stdout: '', error: null }
    }
    return execGit(args)
  }

  // ─── Step 1: Branch validation ─────────────────────────────────────────────
  const branchResult = execGit(['symbolic-ref', '--short', 'HEAD'])
  if (branchResult.error) {
    writeln(`error: could not determine current branch (are you in a git repo?): ${branchResult.error.message}`)
    return 1
  }
  const branch = branchResult.stdout.trim()

  if (branch === 'main' || branch === 'master') {
    writeln(`error: trackfw ship cannot run on "${branch}" — use a feature branch:\n  git checkout -b feat/<slug>`)
    return 1
  }

  if (!isShipBranch(branch)) {
    writeln(
      `error: branch "${branch}" does not match the required pattern feat|fix|refactor/<slug>\n` +
      'Rename your branch or create a new one:\n  git checkout -b feat/<slug>'
    )
    return 1
  }

  writeln(`Branch: ${branch}`)

  // ─── Step 2: Governance ────────────────────────────────────────────────────
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
    writeln(`\nerror: governance check failed: ${violations.length} violation(s)`)
    return 1
  }

  writeln('Governance: OK')

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

  const { stdout: cachedFiles } = execGit(['diff', '--cached', '--name-only'])
  if (!cachedFiles.trim()) {
    writeln(
      'error: nothing is staged — stage your files explicitly before running ship:\n' +
      '  git add <file1> <file2> ...\n' +
      "Never use 'git add .' or 'git add -A'"
    )
    return 1
  }

  // ─── Step 5: Commit ────────────────────────────────────────────────────────
  if (!opts.message) {
    writeln(
      'error: commit message is required — use -m:\n' +
      '  trackfw ship -m "feat(<scope>): <description>"'
    )
    return 1
  }

  const { error: commitErr } = git(['commit', '-m', opts.message])
  if (commitErr) {
    writeln(`error: git commit failed: ${commitErr.message}`)
    return 1
  }

  if (!opts.dryRun) {
    writeln(`Committed: ${opts.message}`)
  }

  // ─── Step 6: Push ──────────────────────────────────────────────────────────
  const pushArgs = buildPushArgs(branch, execGit)
  const { error: pushErr } = git(pushArgs)
  if (pushErr) {
    writeln(`error: git push failed: ${pushErr.message}`)
    return 1
  }

  if (!opts.dryRun) {
    writeln(`Pushed:    ${branch} → origin/${branch}`)
  }

  // ─── Step 7: Open PR/MR ────────────────────────────────────────────────────
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

  if (opts.dryRun) {
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
  const title = firstLine(opts.message || '')
  const body = `Branch: ${branch}\n\nCreated by trackfw ship.`
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
  isGitWriteCmd,
  normalizeBranchSlug,
  checkShipGovernance,
  resolveRoadmapDir,
  resetConfig,
  GIT_WRITE_COMMANDS,
  buildForgeCreateArgs,
  firstLine,
}
