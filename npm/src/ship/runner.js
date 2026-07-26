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

  // Resolve roadmap dir from trackfw.yaml (default: docs/roadmaps/claude)
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
 * resolveRoadmapDir reads trackfw.yaml to find roadmap_dir (default: docs/roadmaps/claude).
 * @returns {string}
 */
function resolveRoadmapDir() {
  try {
    const yaml = fs.readFileSync('trackfw.yaml', 'utf8')
    for (const line of yaml.split('\n')) {
      const m = line.match(/^roadmap_dir:\s*(.+)/)
      if (m) return m[1].trim()
    }
  } catch (_) {}
  return 'docs/roadmaps/claude'
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
 * runShip executes the six-step ship sequence.
 *
 * @param {{ message: string, dryRun: boolean }} opts
 * @param {{ execGit?: function, checkGovernance?: function, writeln?: function }} deps
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
    writeln('\nship complete.')
  }

  return 0
}

module.exports = {
  runShip,
  isShipBranch,
  isGitWriteCmd,
  normalizeBranchSlug,
  checkShipGovernance,
  GIT_WRITE_COMMANDS,
}
