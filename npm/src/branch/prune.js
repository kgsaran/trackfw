'use strict'

/**
 * branch/prune.js — Core implementation of `trackfw branch prune`.
 *
 * Mirrors internal/commands/branch_prune.go byte-for-byte in behavior and message text — Go is
 * the behavioral reference (docs/cli-parity.md: "Go is the behavioral reference").
 *
 * Decides whether a local branch is safe to delete using the touched-files heuristic documented
 * in CLAUDE.md §1 and REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-
 * deteccao-correta-de-squash-merge.md — NOT git's own ancestry check (`git branch -d`, which
 * always refuses squash-merged branches) and NOT a naive bidirectional diff against origin/main
 * (which false-positives on a branch that is merged but stale, once main has advanced further).
 *
 * evaluateBranchIntegration is the single shared decision function — ML-2A
 * (npm/src/ship/runner.js's detectPendingSquashMerges) is expected to call it instead of its own
 * bidirectional diff.
 */

const { spawnSync } = require('child_process')

// The only source of truth this command consults: the local tracking ref for the default branch.
// Per REQ-2026-08-18 decision 2, there is no forge lookup and no network call — offline and
// deterministic by construction. If this ref cannot be resolved (no remote configured, or never
// fetched), the whole command refuses and deletes nothing.
const DEFAULT_REMOTE_REF = 'origin/main'

// The local branch name matching DEFAULT_REMOTE_REF. Always excluded as a prune candidate —
// evaluating it against itself would report "no own work" and offer to delete the branch the user
// is meant to keep (merge-base origin/main main == main's own tip, so "touched" is trivially
// empty). This is the highest-severity bug the naive heuristic contains.
const DEFAULT_LOCAL_NAME = 'main'

const DECISION = {
  DEFAULT_BRANCH: 'default_branch',
  CURRENT_BRANCH: 'current_branch',
  WORKTREE: 'worktree_branch',
  NO_OWN_WORK: 'no_own_work',
  IDENTICAL: 'content_identical',
  PENDING_WORK: 'pending_work',
  NO_MERGE_BASE: 'no_merge_base',
  EVAL_ERROR: 'eval_error',
  // REVIEW_DOC_CONFIG is a NON-deletable category, distinct from PENDING_WORK: every diverging
  // file is doc/config (isDocOrConfigPath), so it is probably housekeeping residue from a
  // squash-merge rather than genuine pending work — but it is never auto-deleted. CLAUDE.md §1's
  // own procedure treats this case as "housekeeping, apagar" but with a human already in the
  // loop reading the diff; a destructive command has no human in the loop by construction, so
  // REQ-2026-08-18 (ML-1B) deliberately narrows that step to "flag for confirmation" instead of
  // "delete automatically". See isDeletable below.
  REVIEW_DOC_CONFIG: 'review_doc_config',
}

// DOC_CONFIG_EXTENSIONS/DOC_CONFIG_BASENAMES mirror internal/commands/branch_prune.go's
// branchPruneDocConfigExtensions/branchPruneDocConfigBasenames — deliberately conservative and
// best-effort. Misclassifying a file here never causes a deletion: it only changes which
// non-deletable category (REVIEW_DOC_CONFIG vs PENDING_WORK) a kept branch is reported under.
const DOC_CONFIG_EXTENSIONS = ['.yaml', '.yml', '.json', '.toml', '.ini', '.cfg']
const DOC_CONFIG_BASENAMES = new Set(['.gitignore', '.gitattributes', '.editorconfig', 'trackfw.yaml', 'LICENSE'])

/**
 * isDocsFile reports whether path lives under docs/ or vault/, or has a .md extension. Mirrors
 * internal/commands/commit.go's isDocsFile (same package there; standalone here).
 * @param {string} path
 * @returns {boolean}
 */
function isDocsFile(path) {
  if (path.startsWith('docs/') || path.startsWith('vault/')) return true
  return path.endsWith('.md')
}

/**
 * isDocOrConfigPath reports whether path is a doc file (isDocsFile) or a well-known non-runtime
 * config file/extension. Used only to route an otherwise PENDING_WORK branch into the
 * REVIEW_DOC_CONFIG category — never to make anything deletable.
 * @param {string} path
 * @returns {boolean}
 */
function isDocOrConfigPath(path) {
  if (isDocsFile(path)) return true
  const base = path.includes('/') ? path.slice(path.lastIndexOf('/') + 1) : path
  if (DOC_CONFIG_BASENAMES.has(base)) return true
  return DOC_CONFIG_EXTENSIONS.some((ext) => path.endsWith(ext))
}

/**
 * allDocOrConfig reports whether every path in paths is doc/config (isDocOrConfigPath). Empty
 * input returns false.
 * @param {string[]} paths
 * @returns {boolean}
 */
function allDocOrConfig(paths) {
  if (paths.length === 0) return false
  return paths.every(isDocOrConfigPath)
}

/**
 * isDeletable reports whether decision, on its own, makes a branch a deletion candidate. Both
 * no_own_work (squash-merge with no ancestry — the `git branch -d` false negative) and
 * content_identical (defasada porém integrada — the naive `git diff` false positive) are safe to
 * delete; every other decision keeps the branch.
 * @param {string} decision
 * @returns {boolean}
 */
function isDeletable(decision) {
  return decision === DECISION.NO_OWN_WORK || decision === DECISION.IDENTICAL
}

/**
 * defaultExecGit runs `git <args...>` and returns { stdout, error }. stdout is trimmed the same
 * way internal/commands/ship.go's defaultGitExec trims via strings.TrimSpace — NUL bytes (used as
 * the -z separator below) are not whitespace and are never stripped.
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
 * splitNulPaths splits a NUL-separated `git diff --name-only -z` output into a sorted, non-empty
 * path list. A trailing NUL (git always emits one after the last entry) produces one empty
 * trailing element, which is dropped.
 * @param {string} raw
 * @returns {string[]}
 */
function splitNulPaths(raw) {
  return raw.split('\x00').filter((p) => p !== '').sort()
}

/**
 * evaluateBranchIntegration decides whether branch is safe to delete relative to
 * DEFAULT_REMOTE_REF, using the touched-files heuristic:
 *
 *   mb      = git merge-base origin/main <branch>
 *   touched = git diff --name-only mb <branch>                       (what the branch touched)
 *   diverg  = git diff --name-only origin/main <branch> -- touched   (what still differs there)
 *
 * touched empty     -> no_own_work (deletable)       -- the squash-merge / -d false negative
 * diverg empty      -> content_identical (deletable) -- the naive-diff false positive (stale
 *                                                        branch, main advanced by other PRs)
 * diverg non-empty  -> pending_work (kept, explained)
 *
 * Both diff calls use -z (NUL-separated, unquoted paths) so filenames with spaces or non-ASCII
 * bytes are never mis-split — the exact class of bug that would make a branch with pending work in
 * "foo bar.md" read as an empty diverg and get deleted.
 *
 * @param {string} branch
 * @param {function(string[]): {stdout: string, error: Error|null}} execGit
 * @returns {{ name: string, decision: string, reason: string, touched: string[], diverged: string[] }}
 */
function evaluateBranchIntegration(branch, execGit) {
  const mbResult = execGit(['merge-base', DEFAULT_REMOTE_REF, branch])
  const mb = (mbResult.stdout || '').trim()
  if (mbResult.error || mb === '') {
    return {
      name: branch,
      decision: DECISION.NO_MERGE_BASE,
      reason: `no merge-base with ${DEFAULT_REMOTE_REF} — refusing (unrelated history or bad ref)`,
      touched: [],
      diverged: [],
    }
  }

  const touchedResult = execGit(['diff', '--name-only', '-z', mb, branch])
  if (touchedResult.error) {
    return {
      name: branch,
      decision: DECISION.EVAL_ERROR,
      reason: `git diff --name-only -z ${mb} ${branch} failed: ${touchedResult.error.message}`,
      touched: [],
      diverged: [],
    }
  }
  const touched = splitNulPaths(touchedResult.stdout)

  if (touched.length === 0) {
    return {
      name: branch,
      decision: DECISION.NO_OWN_WORK,
      reason: `no own work relative to ${DEFAULT_REMOTE_REF} — safe to delete`,
      touched: [],
      diverged: [],
    }
  }

  const divergResult = execGit(['diff', '--name-only', '-z', DEFAULT_REMOTE_REF, branch, '--', ...touched])
  if (divergResult.error) {
    return {
      name: branch,
      decision: DECISION.EVAL_ERROR,
      reason: `git diff --name-only -z ${DEFAULT_REMOTE_REF} ${branch} -- <touched> failed: ${divergResult.error.message}`,
      touched,
      diverged: [],
    }
  }
  const diverg = splitNulPaths(divergResult.stdout)

  if (diverg.length === 0) {
    return {
      name: branch,
      decision: DECISION.IDENTICAL,
      reason: `squash-merged into ${DEFAULT_REMOTE_REF} — content identical in touched files, safe to delete`,
      touched,
      diverged: [],
    }
  }

  // review_doc_config requires diverg to be a PROPER subset of touched (diverg.length <
  // touched.length) — not just "all doc/config". diverg is always a subset of touched by
  // construction (the second git diff is scoped `-- touched`), so this is the same as: at
  // least one touched file made it into main. That distinguishes genuine housekeeping residue
  // from a squash-merge from a branch whose ENTIRE touched set — doc/config or not — never
  // reached main (diverg == touched): that is pending work, regardless of file type.
  if (diverg.length < touched.length && allDocOrConfig(diverg)) {
    return {
      name: branch,
      decision: DECISION.REVIEW_DOC_CONFIG,
      reason: `only doc/config files diverge from ${DEFAULT_REMOTE_REF} (${diverg.join(', ')}) — probable housekeeping, confirm and delete manually`,
      touched,
      diverged: diverg,
    }
  }

  return {
    name: branch,
    decision: DECISION.PENDING_WORK,
    reason: `pending work vs ${DEFAULT_REMOTE_REF}: ${diverg.join(', ')}`,
    touched,
    diverged: diverg,
  }
}

/**
 * defaultListLocalBranches runs `git branch --format=%(refname:short)` and returns one name per
 * non-empty line.
 * @param {function(string[]): {stdout: string, error: Error|null}} execGit
 * @returns {{ branches: string[], error: Error|null }}
 */
function defaultListLocalBranches(execGit) {
  const result = execGit(['branch', '--format=%(refname:short)'])
  if (result.error) return { branches: [], error: result.error }
  const branches = (result.stdout || '')
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l !== '')
  return { branches, error: null }
}

/**
 * defaultCurrentBranch returns the checked-out branch's short name, or '' on detached HEAD.
 * @param {function(string[]): {stdout: string, error: Error|null}} execGit
 * @returns {string}
 */
function defaultCurrentBranch(execGit) {
  const result = execGit(['symbolic-ref', '--quiet', '--short', 'HEAD'])
  if (result.error) return ''
  return (result.stdout || '').trim()
}

/**
 * defaultWorktreeBranches parses `git worktree list --porcelain` and returns the set of branch
 * short names checked out in any worktree. Uses the porcelain "branch refs/heads/<name>" line, not
 * the human-readable format.
 * @param {function(string[]): {stdout: string, error: Error|null}} execGit
 * @returns {Set<string>}
 */
function defaultWorktreeBranches(execGit) {
  const result = execGit(['worktree', 'list', '--porcelain'])
  const set = new Set()
  if (result.error) return set
  const prefix = 'branch refs/heads/'
  for (const rawLine of (result.stdout || '').split('\n')) {
    const line = rawLine.trim()
    if (line.startsWith(prefix)) {
      set.add(line.slice(prefix.length))
    }
  }
  return set
}

/**
 * defaultDeleteBranch tries `git branch -d <name>` first. When the branch happens to have
 * fast-forward ancestry with main too (a plain merge, not a squash), -d succeeds and confirms the
 * integration via git's own independent check — no need for -D at all. Falls back to
 * `git branch -D <name>` only when -d refuses, the expected outcome for squash-merged branches
 * (no ancestry by construction): all safety already lives in evaluateBranchIntegration and the
 * re-check immediately before this call, not in which flag performs the deletion.
 * @param {function(string[]): {stdout: string, error: Error|null}} execGit
 * @param {string} name
 * @returns {Error|null}
 */
function defaultDeleteBranch(execGit, name) {
  const dResult = execGit(['branch', '-d', name])
  if (!dResult.error) return null
  return execGit(['branch', '-D', name]).error
}

/**
 * runBranchPrune implements `trackfw branch prune`.
 *
 * --dry-run is the default (`apply === false`): without --apply, nothing is ever deleted, even
 * the clearly integrated. The current branch, any branch checked out in another worktree, and the
 * default branch (main) are always kept and never evaluated for deletion. Without origin/main
 * resolvable (offline, no remote, never fetched), the whole command refuses and deletes nothing.
 *
 * @param {boolean} apply
 * @param {{ execGit?: function, listLocalBranches?: function, currentBranch?: function,
 *           worktreeBranches?: function, deleteBranch?: function, writeln?: function,
 *           writeErr?: function }} deps
 * @returns {number} exit code (0 = ran to completion, 1 = origin/main unresolvable)
 */
function runBranchPrune(apply, deps = {}) {
  const execGit = deps.execGit || defaultExecGit
  const listLocalBranches = deps.listLocalBranches || defaultListLocalBranches
  const currentBranchFn = deps.currentBranch || defaultCurrentBranch
  const worktreeBranchesFn = deps.worktreeBranches || defaultWorktreeBranches
  const deleteBranch = deps.deleteBranch || defaultDeleteBranch
  const writeln = deps.writeln || ((s) => process.stdout.write(s + '\n'))
  const writeErr = deps.writeErr || ((s) => process.stderr.write(s + '\n'))

  // Best-effort `git fetch origin --prune`, per CLAUDE.md §1 step 1. Failure is non-blocking —
  // offline is a legitimate use case, the same posture `trackfw ship`'s squash-merge check
  // already takes (ship/runner.js) — but unlike ship (which skips its check entirely on fetch
  // failure), evaluation below still proceeds against whatever origin/main ref is already
  // resolvable locally: a stale ref only ever makes the result MORE conservative, never less.
  const fetchResult = execGit(['fetch', 'origin', '--prune'])
  if (fetchResult.error) {
    writeln(
      'Warning: could not fetch origin (offline, no remote, or fetch failed) — evaluating with possibly stale data; a branch merged upstream since the last fetch may still be reported as pending.'
    )
  }

  const originCheck = execGit(['rev-parse', '--verify', '-q', DEFAULT_REMOTE_REF])
  if (originCheck.error) {
    writeln(
      `trackfw branch prune: ${DEFAULT_REMOTE_REF} not found — offline, no remote configured, or never fetched. Refusing to evaluate any branch; nothing deleted.`
    )
    // Mirrors internal/commands/branch_prune.go: the human-readable message goes to stdout
    // above, and the bare error (matching Go's cmd.SilenceErrors, no "Error: " prefix,
    // propagated once by Execute()) goes to stderr — same split branch new already uses.
    writeErr(`branch prune: ${DEFAULT_REMOTE_REF} not resolvable`)
    return 1
  }

  const { branches: rawBranches, error: listErr } = listLocalBranches(execGit)
  if (listErr) {
    writeln(`trackfw branch prune: failed to list local branches: ${listErr.message}`)
    // Mirrors Go: returns the raw error, propagated bare by Execute() — not a re-wrapped string.
    writeErr(listErr.message)
    return 1
  }
  const branches = [...rawBranches].sort()

  const current = currentBranchFn(execGit)
  const worktreed = worktreeBranchesFn(execGit)

  writeln(`trackfw branch prune — evaluating ${branches.length} local branch(es) against ${DEFAULT_REMOTE_REF}\n`)

  const toDelete = []
  const toReview = []
  for (const b of branches) {
    let evalResult
    if (b === DEFAULT_LOCAL_NAME) {
      evalResult = { name: b, decision: DECISION.DEFAULT_BRANCH, reason: 'default branch — never pruned' }
    } else if (b === current) {
      evalResult = { name: b, decision: DECISION.CURRENT_BRANCH, reason: 'current branch — never pruned' }
    } else if (worktreed.has(b)) {
      evalResult = { name: b, decision: DECISION.WORKTREE, reason: 'checked out in another worktree — never pruned' }
    } else {
      evalResult = evaluateBranchIntegration(b, execGit)
    }

    let action = 'keep'
    if (isDeletable(evalResult.decision)) {
      action = 'delete'
      toDelete.push(b)
    } else if (evalResult.decision === DECISION.REVIEW_DOC_CONFIG) {
      action = 'review'
      toReview.push(b)
    }
    writeln(`  ${evalResult.name.padEnd(30)} ${action.padEnd(7)} ${evalResult.reason}`)
  }

  writeln('')
  if (toReview.length > 0) {
    writeln(`${toReview.length} branch(es) need manual review (only doc/config diverges, never auto-deleted): ${toReview.join(', ')}`)
  }
  if (!apply) {
    if (toDelete.length === 0) {
      writeln('[dry-run] nothing to delete.')
    } else {
      writeln(`[dry-run] would delete ${toDelete.length} branch(es): ${toDelete.join(', ')}. Rerun with --apply to delete.`)
    }
    return 0
  }

  if (toDelete.length === 0) {
    writeln('nothing to delete.')
    return 0
  }

  const deleted = []
  for (const b of toDelete) {
    // Re-check current/worktree status immediately before each delete — belt-and-suspenders
    // against the branch changing state between the report above and this loop.
    if (b === currentBranchFn(execGit)) {
      writeln(`skip ${b}: became the current branch — refusing to delete`)
      continue
    }
    if (worktreeBranchesFn(execGit).has(b)) {
      writeln(`skip ${b}: became checked out in a worktree — refusing to delete`)
      continue
    }
    const delErr = deleteBranch(execGit, b)
    if (delErr) {
      writeln(`failed to delete ${b}: ${delErr.message}`)
      continue
    }
    deleted.push(b)
  }

  if (deleted.length === 0) {
    writeln('deleted 0 branch(es).')
  } else {
    writeln(`deleted ${deleted.length} branch(es): ${deleted.join(', ')}`)
  }
  return 0
}

module.exports = {
  DEFAULT_REMOTE_REF,
  DEFAULT_LOCAL_NAME,
  DECISION,
  isDeletable,
  isDocOrConfigPath,
  allDocOrConfig,
  splitNulPaths,
  evaluateBranchIntegration,
  defaultExecGit,
  defaultListLocalBranches,
  defaultCurrentBranch,
  defaultWorktreeBranches,
  defaultDeleteBranch,
  runBranchPrune,
}
