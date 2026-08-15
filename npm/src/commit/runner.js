'use strict'

/**
 * commit/runner.js — Core implementation of `trackfw commit`.
 *
 * Mirrors internal/commands/commit.go byte-for-byte in behavior and message text — Go is the
 * behavioral reference (docs/cli-parity.md: "Go is the behavioral reference"). Reuses the same
 * governance matching primitives already shared by `trackfw branch new` (../branch/runner.js) and
 * `trackfw validate`/`trackfw ship` (../validator): validator.branchSlugMatchesRoadmap,
 * validator.branchGovernanceOrientation, validator.branchNoMatchingRoadmapMessage,
 * validator.normalizeBranchSlug — never duplicate this logic.
 *
 * Mirrors Go's stdout/stderr split exactly (internal/commands/commit.go + root.go Execute()):
 * the primary governance message (block reason, or the housekeeping-branch warning) goes to
 * stdout via `writeln`; the short "blocked: ..." error / usage error that Execute() prints from
 * the returned error goes to stderr via `writeErr`.
 *
 * All git operations are injectable for testability — never runs a real `git commit` in unit
 * tests.
 */

const { spawnSync } = require('child_process')
const config = require('../config')
const validator = require('../validator')

// commitProtectedBranches lists branches where a direct `trackfw commit` is never allowed,
// regardless of governance state — mirrors the same hard rule `trackfw ship` already enforces.
// Mirrors Go's commitProtectedBranches: literal "main"/"master" only, no dynamic resolution of
// the repo's configured default branch.
const COMMIT_PROTECTED_BRANCHES = new Set(['main', 'master'])

// commitGovernedPrefixes lists the branch-type prefixes that require a matching roadmap in
// wip/ or done/ before a commit is allowed — the same vocabulary `trackfw branch new` and the
// branch_has_wip_roadmap governance rule already enforce. Mirrors Go's commitGovernedPrefixes.
const COMMIT_GOVERNED_PREFIXES = ['feat/', 'fix/', 'refactor/']

/**
 * defaultExecGit runs `git <args...>` and returns { stdout, error }. Mirrors
 * ship/runner.js's defaultExecGit (same pattern, kept local here to avoid a cross-module runtime
 * dependency for a single helper).
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
 * defaultStagedNameStatus runs `git diff --cached --name-status` and returns its raw output.
 * Mirrors Go's defaultStagedNameStatus (internal/commands/commit.go), which reuses the same git
 * subcommand. Used only by buildSuggestedMessage — never touched by the normal `-m` commit flow.
 * @param {function(string[]): {stdout: string, error: Error|null}} execGit
 * @returns {string}
 * @throws {Error} when git fails (e.g. not a git repo)
 */
function defaultStagedNameStatus(execGit) {
  const result = execGit(['diff', '--cached', '--name-status'])
  if (result.error) throw result.error
  return result.stdout
}

/**
 * defaultCurrentBranch reads the current branch via `git rev-parse --abbrev-ref HEAD` — mirrors
 * Go's defaultCurrentBranch (internal/commands/commit.go), which reuses the same git subcommand.
 * @param {function(string[]): {stdout: string, error: Error|null}} execGit
 * @returns {string}
 * @throws {Error} when git fails (e.g. not a git repo)
 */
function defaultCurrentBranch(execGit) {
  const result = execGit(['rev-parse', '--abbrev-ref', 'HEAD'])
  if (result.error) throw result.error
  return result.stdout.trim()
}

/**
 * defaultGitCommit runs `git commit -m <message>` with inherited stdio, so Git's own output
 * reaches the user unmodified. Returns Git's own numeric exit code (0 on success), never a
 * hardcoded 1 — same contract as branch/runner.js's defaultGitCheckout.
 * @param {string} message
 * @returns {number} exit code
 */
function defaultGitCommit(message) {
  const result = spawnSync('git', ['commit', '-m', message], { stdio: 'inherit' })
  if (result.error) return 1
  if (result.status === null) return 1
  return result.status
}

/**
 * commitGovernedBranchPrefix returns the matched prefix (e.g. "feat/") when branch starts with
 * one of COMMIT_GOVERNED_PREFIXES, or null otherwise. Mirrors Go's commitGovernedBranchPrefix.
 * @param {string} branch
 * @returns {string|null}
 */
function commitGovernedBranchPrefix(branch) {
  for (const prefix of COMMIT_GOVERNED_PREFIXES) {
    if (branch.startsWith(prefix)) return prefix
  }
  return null
}

/**
 * isCommitGatedBranch returns true when branch matches feat|fix|refactor/<slug> — the branches
 * this command requires a matching wip/done roadmap for.
 * @param {string} branch
 * @returns {boolean}
 */
function isCommitGatedBranch(branch) {
  return commitGovernedBranchPrefix(branch) !== null
}

/**
 * runCommit implements the `trackfw commit -m "<message>"` flow described in ML-2A/ML-2B of
 * docs/roadmaps/wip/ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md.
 *
 *   0. Requires a non-empty message (mirrors Go's cobra RunE pre-check, before touching git).
 *   1. Reads the current branch. On "main"/"master": always blocked.
 *   2. On a feat/fix/refactor branch: requires a matching roadmap in wip/ or done/ — the same
 *      matching logic `trackfw branch new` already uses. No match: blocks with the same
 *      governance orientation message `trackfw validate` already prints for
 *      branch_has_wip_roadmap.
 *   3. Any other branch shape: allowed, only logs a warning — orchestration/housekeeping
 *      branches are not gated by this rule.
 *   4. If it passed (1)-(3): runs `git commit -m <message>`, propagating Git's own output and
 *      exit status literally.
 *
 * @param {string} message commit message (-m value)
 * @param {{ loadConfig?: function, resolveWIPDirs?: function, resolveDoneDirs?: function,
 *           matchSlug?: function(string, string[], string[]): {matched: boolean, candidates: string[]},
 *           execGit?: function(string[]): {stdout: string, error: Error|null},
 *           currentBranch?: function(): string,
 *           execGitCommit?: function(string): number, writeln?: function(string): void,
 *           writeErr?: function(string): void }} deps
 * @returns {number} exit code (0 = success, 1 = blocked/usage error, or Git's own exit code when
 *   `git commit` fails — propagated literally, never hardcoded)
 */
function runCommit(message, deps = {}) {
  const loadConfig = deps.loadConfig || config.load
  const resolveWIPDirs = deps.resolveWIPDirs || validator.resolveWIPDirs
  const resolveDoneDirs = deps.resolveDoneDirs || validator.resolveDoneDirs
  const matchSlug = deps.matchSlug || validator.branchSlugMatchesRoadmap
  const execGit = deps.execGit || defaultExecGit
  const currentBranch = deps.currentBranch || (() => defaultCurrentBranch(execGit))
  const execGitCommit = deps.execGitCommit || defaultGitCommit
  const writeln = deps.writeln || ((s) => process.stdout.write(s + '\n'))
  const writeErr = deps.writeErr || ((s) => process.stderr.write(s + '\n'))

  if (!message || String(message).trim() === '') {
    writeErr('commit message is required — use -m:\n  trackfw commit -m "feat(<scope>): <description>"')
    return 1
  }

  let branch
  try {
    branch = currentBranch()
  } catch (err) {
    writeErr(`could not determine current branch (are you in a git repo?): ${err.message}`)
    return 1
  }
  branch = String(branch || '').trim()

  // (a) main/master: always blocked.
  if (COMMIT_PROTECTED_BRANCHES.has(branch)) {
    const msg = `trackfw commit: commit direto em "${branch}" não é permitido. Use 'trackfw branch new <type>/<slug>' primeiro. Ver CLAUDE.md §1.`
    writeln(msg)
    writeErr(`blocked: commit directly on "${branch}" is not permitted`)
    return 1
  }

  // (b) feat/fix/refactor: require a matching roadmap in wip/ or done/.
  const governedPrefix = commitGovernedBranchPrefix(branch)
  if (governedPrefix) {
    const cfg = loadConfig()
    const wipDirs = resolveWIPDirs(cfg)
    const doneDirs = resolveDoneDirs(cfg)
    const slug = branch.slice(governedPrefix.length)
    const normalizedSlug = validator.normalizeBranchSlug(slug)
    const { matched, candidates } = matchSlug(normalizedSlug, wipDirs, doneDirs)

    if (!matched) {
      const msg = (!candidates || candidates.length === 0)
        ? validator.branchGovernanceOrientation(branch)
        : validator.branchNoMatchingRoadmapMessage(branch, candidates)
      writeln(msg)
      writeErr(`blocked: no matching roadmap in wip/ nor done/ for "${branch}"`)
      return 1
    }
  } else {
    // (c) branches outside the feat/fix/refactor pattern (e.g. doc/housekeeping branches):
    // allow without requiring a roadmap, but warn.
    writeln(`trackfw commit: branch "${branch}" does not follow feat/fix/refactor — committing without a roadmap check.`)
  }

  // (d) passed all checks: commit. Git's own stdio (inherited) already reached the user
  // unmodified — do not reformat, duplicate, or replace this exit code with a generic 1.
  return execGitCommit(message)
}

// commitCommandDirs lists the directories (across the 3 supported CLIs) where a new (status "A")
// file signals a new CLI command was added — used by the "feat" heuristic rule below. Mirrors
// Go's commitCommandDirs.
const COMMIT_COMMAND_DIRS = ['internal/commands/', 'npm/src/commands/', 'pypi/trackfw/commands/']

/**
 * parseStagedNameStatus parses raw `git diff --cached --name-status` output (tab-separated
 * "<status>\t<path>" lines) into { status, path } entries, skipping blank lines. Mirrors Go's
 * parseStagedNameStatus.
 * @param {string} raw
 * @returns {{status: string, path: string}[]}
 */
function parseStagedNameStatus(raw) {
  const files = []
  for (const rawLine of String(raw || '').split('\n')) {
    const line = rawLine.replace(/\r+$/, '')
    if (line.trim() === '') continue
    const tabIdx = line.indexOf('\t')
    if (tabIdx === -1) continue
    const status = line.slice(0, tabIdx).trim()
    const path = line.slice(tabIdx + 1)
    files.push({ status, path })
  }
  return files
}

/**
 * isTestFile reports whether path matches one of the recognized test-file naming conventions
 * across the 3 supported stacks: *_test.go, *.test.js, test_*.py, *_test.py. Mirrors Go's
 * isTestFile.
 * @param {string} path
 * @returns {boolean}
 */
function isTestFile(path) {
  const idx = path.lastIndexOf('/')
  const base = idx === -1 ? path : path.slice(idx + 1)
  if (base.endsWith('_test.go')) return true
  if (base.endsWith('.test.js')) return true
  if (base.startsWith('test_') && base.endsWith('.py')) return true
  if (base.endsWith('_test.py')) return true
  return false
}

/**
 * isDocsFile reports whether path lives under docs/ or vault/, or has a .md extension. Mirrors
 * Go's isDocsFile.
 * @param {string} path
 * @returns {boolean}
 */
function isDocsFile(path) {
  if (path.startsWith('docs/') || path.startsWith('vault/')) return true
  return path.endsWith('.md')
}

/**
 * isUnderAnyDir reports whether path starts with any of the given directory prefixes. Mirrors
 * Go's isUnderAnyDir.
 * @param {string} path
 * @param {string[]} dirs
 * @returns {boolean}
 */
function isUnderAnyDir(path, dirs) {
  return dirs.some((d) => path.startsWith(d))
}

/**
 * suggestedCommitType returns the Conventional Commits type suggested for a set of staged files,
 * following the fixed-priority heuristic documented in ML-1A/ML-2A of
 * docs/roadmaps/wip/ROADMAP-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md
 * (first matching rule wins — this is a deliberately simple heuristic, not an attempt at perfect
 * classification). Mirrors Go's suggestedCommitType.
 *  1. every staged file matches a test-file pattern -> "test"
 *  2. every staged file is under docs/ or vault/, or has a .md extension -> "docs"
 *  3. at least one new ("A") file lives under one of COMMIT_COMMAND_DIRS -> "feat"
 *  4. otherwise -> "fix"
 * @param {{status: string, path: string}[]} files
 * @returns {string}
 */
function suggestedCommitType(files) {
  let allTests = true
  let allDocs = true
  let hasNewCommandFile = false

  for (const f of files) {
    if (!isTestFile(f.path)) allTests = false
    if (!isDocsFile(f.path)) allDocs = false
    if (f.status === 'A' && isUnderAnyDir(f.path, COMMIT_COMMAND_DIRS)) hasNewCommandFile = true
  }

  if (allTests) return 'test'
  if (allDocs) return 'docs'
  if (hasNewCommandFile) return 'feat'
  return 'fix'
}

/**
 * buildSuggestedMessage implements `trackfw commit --suggest`: it reads the staged diff via
 * deps.stagedNameStatus, classifies it with suggestedCommitType, and renders the heuristic
 * Conventional Commits skeleton described in ML-1A/ML-2A. It never calls deps.execGitCommit — no
 * commit ever happens as a side effect of this function. Mirrors Go's buildSuggestedMessage
 * byte-for-byte in output template.
 * @param {{ stagedNameStatus?: function(): string }} deps
 * @returns {string}
 * @throws {Error} when staged files cannot be read, or nothing is staged
 */
function buildSuggestedMessage(deps = {}) {
  const stagedNameStatus = deps.stagedNameStatus || (() => defaultStagedNameStatus(deps.execGit || defaultExecGit))

  let raw
  try {
    raw = stagedNameStatus()
  } catch (err) {
    throw new Error(`could not read staged changes (are you in a git repo?): ${err.message}`)
  }

  const files = parseStagedNameStatus(raw)
  if (files.length === 0) {
    throw new Error('nothing staged — `git add` files first')
  }

  const commitType = suggestedCommitType(files)

  const lines = []
  lines.push('# Sugestão heurística — NÃO é uma mensagem pronta, revise antes de usar.')
  lines.push(`# Tipo sugerido: ${commitType}`)
  lines.push('')
  lines.push(`${commitType}(<escopo>): <descrição>`)
  lines.push('')
  lines.push('## Arquivos staged')
  for (const f of files) {
    lines.push(`${f.status}  ${f.path}`)
  }

  return lines.join('\n')
}

module.exports = {
  runCommit,
  buildSuggestedMessage,
  defaultExecGit,
  defaultCurrentBranch,
  defaultGitCommit,
  defaultStagedNameStatus,
  isCommitGatedBranch,
  commitGovernedBranchPrefix,
  suggestedCommitType,
  isTestFile,
  isDocsFile,
}
