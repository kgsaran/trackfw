'use strict'

/**
 * branch/runner.js — Core implementation of `trackfw branch new`.
 *
 * Moves the branch_has_wip_roadmap governance gate (already enforced by `trackfw validate` and
 * `trackfw ship`) to before branch creation, instead of after. Mirrors internal/commands/branch.go
 * byte-for-byte in behavior and message text — Go is the behavioral reference
 * (docs/cli-parity.md: "Go is the behavioral reference").
 *
 * All git write operations are injectable for testability — never runs a real `git checkout -b`
 * in unit tests.
 */

const { spawnSync } = require('child_process')
const config = require('../config')
const validator = require('../validator')

// branchValidTypes is the full vocabulary accepted by `trackfw branch new`. feat/fix/refactor are
// gated on a matching REQ + roadmap already in wip/ or done/ (branchGatedTypes below); chore/docs
// are housekeeping types — already treated as roadmap-exempt by `trackfw ship` and `trackfw
// commit` — and create the branch without that gate.
const branchValidTypes = new Set(['feat', 'fix', 'refactor', 'chore', 'docs'])

// branchGatedTypes is the subset of branchValidTypes that requires a matching REQ + roadmap
// already in wip/ or done/ before the branch is created. Keep this in sync with the pattern
// `trackfw ship`/`trackfw commit` use to decide when the branch_has_wip_roadmap gate applies.
const branchGatedTypes = new Set(['feat', 'fix', 'refactor'])

/**
 * parseBranchSpec splits "<type>/<slug>" and validates both parts. type must be one of
 * feat, fix, refactor, chore, docs (branchValidTypes); slug must be non-empty.
 * @param {string} spec
 * @returns {{ branchType: string, slug: string }}
 * @throws {Error} when spec is malformed
 */
function parseBranchSpec(spec) {
  const value = String(spec)
  const idx = value.indexOf('/')
  if (idx === -1 || value.slice(0, idx) === '') {
    throw new Error(`invalid branch spec "${spec}" — expected <type>/<slug> with type in feat, fix, refactor, chore, docs`)
  }
  const branchType = value.slice(0, idx)
  const slug = value.slice(idx + 1)
  if (!branchValidTypes.has(branchType)) {
    throw new Error(`invalid branch type "${branchType}" — must be one of feat, fix, refactor, chore, docs`)
  }
  if (slug.trim() === '') {
    throw new Error(`branch slug is required — expected <type>/<slug>, got "${spec}"`)
  }
  return { branchType, slug }
}

/**
 * defaultGitCheckout runs `git checkout -b <branchName>` with inherited stdio, so Git's own output
 * (including branch-already-exists errors) reaches the user unmodified.
 *
 * Returns Git's own numeric exit code (0 on success), never a generic 1 — this command's contract
 * is to propagate Git's exit status literally (see internal/commands/branch.go's
 * defaultGitCheckout, the behavioral reference: it exits the process directly with
 * exitErr.ExitCode() rather than a hardcoded 1). A spawn failure (e.g. the git binary is missing)
 * or a signal-terminated process has no Git-produced numeric code to propagate, so both fall back
 * to 1 — the same convention already used for signal-killed gate commands
 * (docs/cli-parity.md, `trackfw barrier`, "Gate process terminated by a signal ... Recorded as exit 1").
 * @param {string} branchName
 * @returns {number} exit code
 */
function defaultGitCheckout(branchName) {
  const result = spawnSync('git', ['checkout', '-b', branchName], { stdio: 'inherit' })
  if (result.error) return 1
  if (result.status === null) return 1
  return result.status
}

/**
 * runBranchNew implements the `trackfw branch new <type>/<slug>` flow described in
 * docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md.
 *
 * Mirrors Go's stdout/stderr split exactly (internal/commands/branch.go + root.go Execute()):
 * the primary governance/dry-run message goes to stdout via `writeln`; usage errors and the
 * "blocked: ..." wrapper (Go's `Execute()` printing the returned error) go to stderr via
 * `writeErr`. Git's own output is inherited directly by the child process and is never touched
 * here.
 *
 * @param {string} spec "<type>/<slug>"
 * @param {boolean} dryRun
 * @param {{ loadConfig?: function, resolveWIPDirs?: function, resolveDoneDirs?: function,
 *           matchSlug?: function(string, string[], string[]): {matched: boolean, candidates: string[]},
 *           execGitCheckout?: function(string): number, writeln?: function(string): void,
 *           writeErr?: function(string): void }} deps
 * @returns {number} exit code (0 = success, 1 = blocked/usage error, or Git's own exit code when
 *   `git checkout -b` fails — propagated literally, never hardcoded)
 */
function runBranchNew(spec, dryRun, deps = {}) {
  const loadConfig = deps.loadConfig || config.load
  const resolveWIPDirs = deps.resolveWIPDirs || validator.resolveWIPDirs
  const resolveDoneDirs = deps.resolveDoneDirs || validator.resolveDoneDirs
  const matchSlug = deps.matchSlug || validator.branchSlugMatchesRoadmap
  const execGitCheckout = deps.execGitCheckout || defaultGitCheckout
  const writeln = deps.writeln || ((s) => process.stdout.write(s + '\n'))
  const writeErr = deps.writeErr || ((s) => process.stderr.write(s + '\n'))

  let branchType, slug
  try {
    ({ branchType, slug } = parseBranchSpec(spec))
  } catch (err) {
    writeErr(err.message)
    return 1
  }

  const branchName = `${branchType}/${slug}`

  // chore/docs are housekeeping types — already treated as roadmap-exempt by `trackfw ship`
  // and `trackfw commit` — so the branch_has_wip_roadmap gate below does not apply to them.
  if (branchGatedTypes.has(branchType)) {
    const cfg = loadConfig()
    const wipDirs = resolveWIPDirs(cfg)
    const doneDirs = resolveDoneDirs(cfg)

    const normalizedSlug = validator.normalizeBranchSlug(slug)
    const { matched, candidates } = matchSlug(normalizedSlug, wipDirs, doneDirs)

    if (!matched) {
      const msg = (!candidates || candidates.length === 0)
        ? validator.branchGovernanceOrientation(branchName)
        : validator.branchNoMatchingRoadmapMessage(branchName, candidates)
      if (dryRun) {
        writeln(`[dry-run] would block: ${msg}`)
      } else {
        writeln(msg)
      }
      writeErr(`blocked: no matching roadmap in wip/ nor done/ for "${branchName}"`)
      return 1
    }
  }

  if (dryRun) {
    writeln(`[dry-run] would create branch "${branchName}" (git checkout -b ${branchName})`)
    return 0
  }

  // Git's own stderr (inherited stdio) already reached the user unmodified — do not reformat,
  // duplicate, or replace this exit code with a generic 1.
  return execGitCheckout(branchName)
}

module.exports = {
  branchValidTypes,
  branchGatedTypes,
  parseBranchSpec,
  defaultGitCheckout,
  runBranchNew,
}
