'use strict'

// Mirrors internal/commands/doctor_remote.go (Go, canonical source of truth for wording/
// semantics) — see that file's doc comments for the full rationale. Implements the --remote
// modality of `trackfw doctor` (ADR-2026-09-02, ML-3A): GitHub branch protection
// (required_status_checks, enforce_admins) plus local core.hooksPath neutralization. Never
// runs unless explicitly requested via --remote.

const { spawnSync } = require('child_process')
const {
  REQUIRED_STATUS_CHECKS_MISSING,
  ENFORCE_ADMINS_DISABLED,
  HOOKS_PATH_NEUTRALIZED,
  NOT_EVALUATED,
} = require('./doctor')
const { resolve: resolveForge } = require('../forge/resolve')
const { forgeAdapter } = require('../forge/adapter')

// defaultExecGit runs a git command and returns { stdout, error } — stdout trimmed, error null
// on success. Mirrors release/runner.js's defaultExecGit.
function defaultExecGit(args) {
  const result = spawnSync('git', args, { encoding: 'utf8' })
  if (result.status !== 0) {
    const msg = (result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`
    return { stdout: '', error: new Error(msg) }
  }
  return { stdout: (result.stdout || '').trim(), error: null }
}

// defaultExecForgeAPI runs a forge CLI command (gh api ...) and returns { stdout, error }.
// Mirrors release/runner.js's defaultExecForgeAPI.
function defaultExecForgeAPI(name, args, stdin) {
  const result = spawnSync(name, args, { input: stdin || '', encoding: 'utf8' })
  if (result.status !== 0) {
    const msg = (result.stderr || '').trim() || `${name} ${args.join(' ')} exited with ${result.status}`
    return { stdout: (result.stdout || '').trim(), error: new Error(msg) }
  }
  return { stdout: (result.stdout || '').trim(), error: null }
}

// Values that discard every git hook invocation on each OS git supports as a hooksPath target.
// Anything else (including unset) is left alone — a custom husky/lefthook directory is
// legitimate and must never be flagged.
const NEUTRALIZED_HOOKS_PATH_VALUES = new Set(['/dev/null', 'NUL'])

/**
 * runDoctorRemote implements the --remote modality: every branch either produces a genuine
 * finding (evaluated, and wrong) or a NOT_EVALUATED finding (could not evaluate) — never
 * silence, which would read as "ok" to a report that treats an empty finding list as a clean
 * bill of health.
 *
 * @param {object} deps
 * @param {function} [deps.execGit] - (args: string[]) => { stdout, error }
 * @param {function} [deps.execForgeAPI] - (name, args, stdin) => { stdout, error }
 * @param {function} [deps.availFn] - (name: string) => boolean
 * @param {string} [deps.configForge] - forge: field from trackfw.yaml
 * @param {string} [deps.repoDir] - repo root, for CI file detection during forge resolution
 * @returns {Array<object>} findings
 */
function runDoctorRemote(deps = {}) {
  const execGit = deps.execGit || defaultExecGit
  const execForgeAPI = deps.execForgeAPI || defaultExecForgeAPI
  const availFn = deps.availFn
  const configForge = deps.configForge || ''
  const repoDir = deps.repoDir !== undefined ? deps.repoDir : ''

  const findings = []

  // ── Local check: core.hooksPath neutralized (no network needed) ──────────────────────
  const hooksPathResp = execGit(['config', '--get', 'core.hooksPath'])
  if (!hooksPathResp.error && NEUTRALIZED_HOOKS_PATH_VALUES.has(hooksPathResp.stdout.trim())) {
    findings.push({
      finding: HOOKS_PATH_NEUTRALIZED,
      destination: 'git:core.hooksPath',
      remedy: `git config --unset core.hooksPath   # currently "${hooksPathResp.stdout.trim()}" discards every hook invocation; unset to restore .git/hooks, or point it at your real hooks directory`,
    })
  }

  // ── Forge resolution: only GitHub is evaluated; every other forge is not applicable ──
  const remoteURLResp = execGit(['remote', 'get-url', 'origin'])
  const remoteURL = remoteURLResp.error ? '' : remoteURLResp.stdout.trim()
  let resolution
  try {
    resolution = resolveForge({ configForge, remoteURL, repoDir })
  } catch (e) {
    findings.push({
      finding: NOT_EVALUATED,
      destination: 'branch-protection',
      remedy: `not applicable: branch protection is checked only on GitHub, and this repository's forge resolved to "unknown". Not a failure — no action needed unless this repository is actually hosted on GitHub, in which case set forge: github in trackfw.yaml or add a github.com origin remote.`,
    })
    return findings
  }
  if (resolution.forge !== 'github') {
    findings.push({
      finding: NOT_EVALUATED,
      destination: 'branch-protection',
      remedy: `not applicable: branch protection is checked only on GitHub, and this repository's forge resolved to "${resolution.forge}". Not a failure — no action needed unless this repository is actually hosted on GitHub, in which case set forge: github in trackfw.yaml or add a github.com origin remote.`,
    })
    return findings
  }

  // ── gh CLI availability ───────────────────────────────────────────────────────────────
  const adapter = forgeAdapter('github', availFn)
  if (!adapter.available) {
    findings.push({
      finding: NOT_EVALUATED,
      destination: 'branch-protection',
      remedy: 'install the GitHub CLI (gh) to evaluate branch protection remotely: https://cli.github.com, then retry with --remote',
    })
    return findings
  }

  // ── Credential presence: distinct from credential SCOPE below (ADR-2026-09-02) ────────
  const authResp = execForgeAPI('gh', ['auth', 'status'], '')
  if (authResp.error) {
    findings.push({
      finding: NOT_EVALUATED,
      destination: 'branch-protection',
      remedy: 'GitHub CLI has no credential — authenticate first: gh auth login (or set GITHUB_TOKEN/GH_TOKEN), then retry with --remote',
    })
    return findings
  }

  // ── Repository info: default branch + whether this credential has admin access ────────
  const repoInfoResp = execForgeAPI('gh', ['api', 'repos/{owner}/{repo}'], '')
  if (repoInfoResp.error) {
    findings.push({
      finding: NOT_EVALUATED,
      destination: 'branch-protection',
      remedy: `could not reach the GitHub API to resolve this repository: ${repoInfoResp.error.message}. Check network connectivity and retry with --remote`,
    })
    return findings
  }
  let repoInfo
  try {
    repoInfo = JSON.parse(repoInfoResp.stdout)
  } catch (e) {
    repoInfo = null
  }
  if (!repoInfo || !repoInfo.default_branch) {
    findings.push({
      finding: NOT_EVALUATED,
      destination: 'branch-protection',
      remedy: `could not parse the repository response from the GitHub API: ${repoInfoResp.stdout}. Retry with --remote`,
    })
    return findings
  }

  // Credential SCOPE: reading branch protection requires admin access to the repository.
  // Distinct remedy from "no credential" above — one is fixed by authenticating, this one by
  // being granted admin access (or using a token for a repo you administer).
  const isAdmin = Boolean(repoInfo.permissions && repoInfo.permissions.admin)
  if (!isAdmin) {
    findings.push({
      finding: NOT_EVALUATED,
      destination: 'branch-protection',
      remedy: 'the authenticated GitHub credential lacks admin access to this repository — reading branch protection requires it. Ask a repository admin to grant access, or authenticate as an account that has it, then retry with --remote',
    })
    return findings
  }

  // ── Branch protection itself ────────────────────────────────────────────────────────────
  const defaultBranch = repoInfo.default_branch
  const protectionResp = execForgeAPI('gh', ['api', `repos/{owner}/{repo}/branches/${defaultBranch}/protection`], '')
  if (protectionResp.error) {
    if (protectionResp.error.message.includes('(HTTP 404)')) {
      // Evaluated (admin confirmed above): the branch genuinely has no protection at all,
      // which means both checks fail — GitHub does not return the two settings separately
      // when there is no rule to read them from.
      findings.push({
        finding: REQUIRED_STATUS_CHECKS_MISSING,
        destination: `branch-protection:${defaultBranch}:required_status_checks`,
        remedy: `configure required status checks: GitHub repo Settings > Branches > Branch protection rules > ${defaultBranch} > Require status checks to pass before merging`,
      })
      findings.push({
        finding: ENFORCE_ADMINS_DISABLED,
        destination: `branch-protection:${defaultBranch}:enforce_admins`,
        remedy: `gh api repos/{owner}/{repo}/branches/${defaultBranch}/protection/enforce_admins --method POST`,
      })
      return findings
    }
    findings.push({
      finding: NOT_EVALUATED,
      destination: `branch-protection:${defaultBranch}`,
      remedy: `could not read branch protection from the GitHub API: ${protectionResp.error.message}. This may be transient (rate limit, network) — retry with --remote`,
    })
    return findings
  }

  let protection
  try {
    protection = JSON.parse(protectionResp.stdout)
  } catch (e) {
    findings.push({
      finding: NOT_EVALUATED,
      destination: `branch-protection:${defaultBranch}`,
      remedy: `could not parse the branch protection response from the GitHub API: ${protectionResp.stdout}. Retry with --remote`,
    })
    return findings
  }

  const rsc = protection.required_status_checks
  const contexts = (rsc && rsc.contexts) || []
  const checks = (rsc && rsc.checks) || []
  if (!rsc || (contexts.length === 0 && checks.length === 0)) {
    findings.push({
      finding: REQUIRED_STATUS_CHECKS_MISSING,
      destination: `branch-protection:${defaultBranch}:required_status_checks`,
      remedy: `configure required status checks: GitHub repo Settings > Branches > Branch protection rules > ${defaultBranch} > Require status checks to pass before merging`,
    })
  }
  const enforceAdmins = protection.enforce_admins
  if (!enforceAdmins || !enforceAdmins.enabled) {
    findings.push({
      finding: ENFORCE_ADMINS_DISABLED,
      destination: `branch-protection:${defaultBranch}:enforce_admins`,
      remedy: `gh api repos/{owner}/{repo}/branches/${defaultBranch}/protection/enforce_admins --method POST`,
    })
  }

  return findings
}

module.exports = { runDoctorRemote, defaultExecGit, defaultExecForgeAPI }
