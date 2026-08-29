'use strict'

/**
 * scaffold_doctor.js — scaffold artifact coverage for `trackfw doctor`
 * (ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-
 * com-propriedade-dada-pelo-caminho.md).
 *
 * Mirrors internal/generators/scaffold_doctor.go (Go, canonical source of truth).
 * Same design decisions apply — see that file's doc comment for the full rationale.
 *
 * Key decisions:
 *   - Property by path, not manifest (AC3): scaffold artifacts are identified by
 *     well-known namespace paths. No manifest entry is written or read.
 *   - Sibling classifier (AC15): scaffold artifacts are never in the manifest, so
 *     routing them through classifyDoctor would produce wrong remedies. The finding
 *     kinds SCAFFOLD_DIVERGENT / SCAFFOLD_MISSING are separate, with zero claim and
 *     a trackfw-update remedy.
 *   - Config-rendered templates (AC12): buildValidateScript(cfg) varies with
 *     cfg.backend/cfg.frontend — rendered from the project's own trackfw.yaml.
 *   - Eligibility for slash commands (AC14): only checked when
 *     .claude/commands/trackfw/ directory already exists.
 *   - Conditional artifacts (AC13): CI workflow only when cfg.ci declares it.
 *   - Neutral blame message (AC16): binary version stated; direction ambiguous.
 *   - Execute-bit checking (REQ-2026-08-28, AC2–AC5, AC10, AC11):
 *     The five scripts written with mode 0o755 are additionally checked for the
 *     owner-execute bit. The check uses (stat.mode & 0o100) !== 0 (not === 0o755)
 *     so umask-narrowed modes (0o750, 0o700) are accepted (AC10). Non-executable
 *     artifacts (slash commands, CI workflows) carry execBit=false and are never
 *     accused of a missing execute bit (AC4/AC11). Content divergence takes
 *     precedence over mode (at most one finding per artifact). Content correct +
 *     bit missing → SCAFFOLD_WRONG_MODE (AC3 distinct state). On Windows
 *     (process.platform === 'win32') the execute bit is not representable on NTFS,
 *     so the mode check is suppressed entirely — AC5.
 *
 * _platform is a module-level constant seeded from process.platform so that unit
 * tests can override it via the exported _setPlatformForTest helper (AC5 testability).
 */

const fs = require('fs')
const path = require('path')
const yaml = require('yaml')

const { SIGNAL_SCRIPT, CLEANUP_SCRIPT, CREDENTIAL_GUARD_SCRIPT, GIT_BRANCH_GUARD_SCRIPT } =
  require('../generators/hooks')
const { CLAUDE_COMMANDS, buildValidateScript, buildGitHubActionsWorkflowContent, buildGitLabCIWorkflowContent } =
  require('../generators/init')
const {
  buildDiscoverGitHubActionsWorkflowContent,
  DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
} = require('../commands/discover')

// Finding kind constants — mirrors Go's DoctorScaffoldDivergent / DoctorScaffoldMissing /
// DoctorScaffoldWrongMode.
const SCAFFOLD_DIVERGENT = 'scaffold-divergent'
const SCAFFOLD_MISSING = 'scaffold-missing'
// SCAFFOLD_WRONG_MODE: content correct but owner-execute bit missing (AC3).
// Only emitted for artifacts with execBit=true; never on Windows (AC5).
const SCAFFOLD_WRONG_MODE = 'scaffold-wrong-mode'

// PYTHON_VALIDATE_SCRIPT_FORM is the byte-exact content Python's `trackfw init` and
// `trackfw update` (validate-script target) write to scripts/trackfw-validate.sh.
// Accepted by the set-membership check in checkValidateScriptArtifact so that a
// project initialized by the Python runtime does not produce a false-positive
// scaffold-divergent finding in the Node.js doctor.
//
// Scope of the exception: ONLY scripts/trackfw-validate.sh uses set-membership.
// All other scaffold artifacts use single-template equality.
// See docs/cli-parity.md, "validate.sh — pertencimento a conjunto (set-membership, escopado)".
const PYTHON_VALIDATE_SCRIPT_FORM = '#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n'

// Paths used by the scaffold doctor (mirrors Go's exported constants).
const CLAUDE_COMMANDS_DIR_PATH = '.claude/commands/trackfw'
const GITHUB_ACTIONS_WORKFLOW_PATH = '.github/workflows/trackfw-gate.yml'
const GITLAB_CI_WORKFLOW_PATH = '.gitlab-ci-trackfw.yml'

// _platform is seeded from process.platform at module load time.
// Tests override it via _setPlatformForTest to exercise the Windows guard (AC5).
let _platform = process.platform

/**
 * _setPlatformForTest overrides the platform string used by mode checks.
 * ONLY for use in unit tests. Returns a restore function.
 *
 * @param {string} plat - e.g. 'win32' or 'darwin'
 * @returns {function} call to restore the original value
 */
function _setPlatformForTest(plat) {
  const prev = _platform
  _platform = plat
  return () => { _platform = prev }
}

/**
 * execBitPresent returns true when the file at absPath has the owner-execute bit set
 * (stat.mode & 0o100 !== 0). Returns false on any stat error.
 *
 * Uses bit mask rather than equality to 0o755 so that umask-narrowed modes like
 * 0o750 or 0o700 are also accepted — AC10.
 *
 * @param {string} absPath - absolute path to the file
 * @returns {boolean}
 */
function execBitPresent(absPath) {
  try {
    const stat = fs.statSync(absPath)
    return (stat.mode & 0o100) !== 0
  } catch (_) {
    return false
  }
}

/**
 * scaffoldRemedy returns a ready-to-copy remedy command for a scaffold finding.
 * The message is neutral about blame direction (AC16).
 */
function scaffoldRemedy(action, relPath) {
  // Lazy-require version to avoid a circular dependency at module load time.
  let ver = 'unknown'
  try {
    const pkg = require('../../package.json')
    ver = pkg.version || 'unknown'
  } catch (_) {}
  return `trackfw update   # ${action} ${relPath}: content differs from the template trackfw v${ver} generates; if this project was initialized with a newer binary, update the binary instead`
}

/**
 * scaffoldWrongModeRemedy returns a remedy command for the scaffold-wrong-mode finding.
 * Names the missing execute bit explicitly to distinguish it from content divergence (AC3).
 * The message is runtime-neutral so the parity check can diff Go, Node, and Python outputs
 * byte-for-byte on this finding kind.
 */
function scaffoldWrongModeRemedy(relPath) {
  return `trackfw update   # restore execute bit on ${relPath}: content is correct but the owner-execute bit is missing (mode 0755 required); trackfw update now restores the mode unconditionally on existing files`
}

/**
 * loadProjectConfig reads trackfw.yaml from projectRoot and returns a cfg object
 * with the same keys that buildValidateScript / buildCIWorkflowContent expect.
 * Missing keys default to undefined/null — identical to Go's loadUpdateConfig().
 */
function loadProjectConfig(projectRoot) {
  try {
    const raw = fs.readFileSync(path.join(projectRoot, 'trackfw.yaml'), 'utf8')
    const parsed = yaml.parse(raw) || {}
    return {
      backend: parsed.backend || null,
      frontend: parsed.frontend || null,
      pkgManager: parsed.pkg_manager || null,
      ci: parsed.ci || null,
    }
  } catch (_) {
    return { backend: null, frontend: null, pkgManager: null, ci: null }
  }
}

/**
 * checkValidateScriptArtifact checks scripts/trackfw-validate.sh using set-membership:
 * accepted if the file content matches EITHER Go/Node's cfg-rendered form OR Python's
 * fixed form (PYTHON_VALIDATE_SCRIPT_FORM). A file matching NO known form is accused.
 * All other scaffold artifacts use checkScaffoldArtifact (single-template equality).
 *
 * After content membership passes, the execute bit is checked unless on Windows (AC5).
 * Content divergence always takes precedence over mode (at most one finding per artifact).
 *
 * @param {string} absPath - absolute path to the file
 * @param {string} relPath - relative path (used as destination in findings)
 * @param {object} cfg - project config from trackfw.yaml
 * @returns {object|null} finding or null
 */
function checkValidateScriptArtifact(absPath, relPath, cfg) {
  let actual
  try {
    actual = fs.readFileSync(absPath, 'utf8')
  } catch (err) {
    if (err.code === 'ENOENT') {
      return {
        finding: SCAFFOLD_MISSING,
        claim: { kind: '', item: '', target: '', surface: '', scope: '' },
        destination: relPath,
        remedy: scaffoldRemedy('restore', relPath),
      }
    }
    return {
      finding: SCAFFOLD_DIVERGENT,
      claim: { kind: '', item: '', target: '', surface: '', scope: '' },
      destination: relPath,
      remedy: scaffoldRemedy('resync', relPath),
    }
  }
  const goNodeForm = buildValidateScript(cfg)
  if (actual !== goNodeForm && actual !== PYTHON_VALIDATE_SCRIPT_FORM) {
    // Content diverges — scaffold-divergent takes precedence over any mode issue.
    return {
      finding: SCAFFOLD_DIVERGENT,
      claim: { kind: '', item: '', target: '', surface: '', scope: '' },
      destination: relPath,
      remedy: scaffoldRemedy('resync', relPath),
    }
  }
  // Content accepted. Check the execute bit (AC2/AC3).
  // Suppressed on Windows where the bit is not representable (AC5).
  if (_platform !== 'win32' && !execBitPresent(absPath)) {
    return {
      finding: SCAFFOLD_WRONG_MODE,
      claim: { kind: '', item: '', target: '', surface: '', scope: '' },
      destination: relPath,
      remedy: scaffoldWrongModeRemedy(relPath),
    }
  }
  return null
}

/**
 * checkScaffoldArtifact compares the on-disk content at absPath against expected.
 * Returns a finding if the file is divergent or (when reportMissing=true) absent.
 * relPath is used as destination in the finding.
 *
 * @param {string} absPath - absolute path to the file
 * @param {string} relPath - relative path (destination in findings)
 * @param {string} expected - expected file content
 * @param {boolean} reportMissing - if true, absence is a finding
 * @param {boolean} execBit - if true, check the owner-execute bit after content passes
 *   true  → artifact was written with 0o755; bit absence → SCAFFOLD_WRONG_MODE (AC2/AC3)
 *   false → artifact is 0o644 (slash commands, CI workflows); never mode-checked (AC4/AC11)
 * @returns {object|null} finding or null
 */
function checkScaffoldArtifact(absPath, relPath, expected, reportMissing, execBit) {
  let actual
  try {
    actual = fs.readFileSync(absPath, 'utf8')
  } catch (err) {
    if (err.code === 'ENOENT') {
      if (!reportMissing) return null
      return {
        finding: SCAFFOLD_MISSING,
        claim: { kind: '', item: '', target: '', surface: '', scope: '' },
        destination: relPath,
        remedy: scaffoldRemedy('restore', relPath),
      }
    }
    // Unreadable artifact: report as divergent so the user is informed.
    return {
      finding: SCAFFOLD_DIVERGENT,
      claim: { kind: '', item: '', target: '', surface: '', scope: '' },
      destination: relPath,
      remedy: scaffoldRemedy('resync', relPath),
    }
  }
  if (actual !== expected) {
    // Content diverges — takes precedence over any mode issue (at most one finding
    // per artifact; update fixes both content and mode anyway).
    return {
      finding: SCAFFOLD_DIVERGENT,
      claim: { kind: '', item: '', target: '', surface: '', scope: '' },
      destination: relPath,
      remedy: scaffoldRemedy('resync', relPath),
    }
  }
  // Content matches. Check the execute bit when required (AC2/AC3).
  // Suppressed on Windows where the bit is not representable (AC5).
  if (execBit && _platform !== 'win32' && !execBitPresent(absPath)) {
    return {
      finding: SCAFFOLD_WRONG_MODE,
      claim: { kind: '', item: '', target: '', surface: '', scope: '' },
      destination: relPath,
      remedy: scaffoldWrongModeRemedy(relPath),
    }
  }
  return null
}

/**
 * runScaffoldDoctor compares scaffold artifacts on disk against the templates the
 * currently installed binary would generate (given the project's own trackfw.yaml),
 * and returns findings for any artifact that is divergent or missing.
 *
 * @param {string} projectRoot - absolute path to the project root
 * @returns {Array} findings
 */
function runScaffoldDoctor(projectRoot) {
  // Eligibility: trackfw.yaml must exist.
  try {
    fs.statSync(path.join(projectRoot, 'trackfw.yaml'))
  } catch (_) {
    return []
  }

  const cfg = loadProjectConfig(projectRoot)
  const findings = []

  // --- Scripts (always in scope when trackfw.yaml is present) ---
  //
  // scripts/trackfw-validate.sh uses set-membership (Go/Node form OR Python form).
  // The four remaining scripts use single-template equality via checkScaffoldArtifact.
  // All five have execBit=true: the generator writes them with mode 0o755 (AC11).
  const validateF = checkValidateScriptArtifact(
    path.join(projectRoot, 'scripts/trackfw-validate.sh'),
    'scripts/trackfw-validate.sh',
    cfg,
  )
  if (validateF) findings.push(validateF)

  const staticScripts = [
    { relPath: 'scripts/trackfw-attention-signal.sh', expected: SIGNAL_SCRIPT, execBit: true },
    { relPath: 'scripts/trackfw-attention-cleanup.sh', expected: CLEANUP_SCRIPT, execBit: true },
    { relPath: 'scripts/trackfw-credential-guard.sh', expected: CREDENTIAL_GUARD_SCRIPT, execBit: true },
    { relPath: 'scripts/trackfw-git-branch-guard.sh', expected: GIT_BRANCH_GUARD_SCRIPT, execBit: true },
  ]
  for (const { relPath, expected, execBit } of staticScripts) {
    const f = checkScaffoldArtifact(path.join(projectRoot, relPath), relPath, expected, true, execBit)
    if (f) findings.push(f)
  }

  // --- Slash commands (AC14: only when the directory already exists) ---
  //
  // Slash commands are markdown files written with mode 0644 — execBit=false (AC11).
  const claudeDir = path.join(projectRoot, CLAUDE_COMMANDS_DIR_PATH)
  let claudeDirExists = false
  try {
    fs.statSync(claudeDir)
    claudeDirExists = true
  } catch (_) {}
  if (claudeDirExists) {
    for (const [filename, content] of Object.entries(CLAUDE_COMMANDS)) {
      const relPath = `${CLAUDE_COMMANDS_DIR_PATH}/${filename}`
      const f = checkScaffoldArtifact(path.join(projectRoot, relPath), relPath, content, true, false)
      if (f) findings.push(f)
    }
  }

  // --- CI workflow (AC13: conditional on ci: in trackfw.yaml) ---
  //
  // CI workflow YAML files are written with mode 0644 — execBit=false (AC11).
  if (cfg.ci === 'github-actions') {
    const relPath = GITHUB_ACTIONS_WORKFLOW_PATH
    const f = checkScaffoldArtifact(
      path.join(projectRoot, relPath),
      relPath,
      buildGitHubActionsWorkflowContent(cfg),
      true,
      false,
    )
    if (f) findings.push(f)
  } else if (cfg.ci === 'gitlab-ci') {
    const relPath = GITLAB_CI_WORKFLOW_PATH
    const f = checkScaffoldArtifact(
      path.join(projectRoot, relPath),
      relPath,
      buildGitLabCIWorkflowContent(cfg),
      true,
      false,
    )
    if (f) findings.push(f)
  }

  // --- Discover CI workflow (second, independent install mechanism) ---
  //
  // trackfw-validate.yml (written by `trackfw discover --init`, installGates) is a
  // separate artifact from trackfw-gate.yml above — both can coexist in the same
  // project (ADR-2026-08-28). Only checked when the file is already present, mirroring
  // the "conditional artifact" treatment of trackfw-gate.yml above but using
  // presence-on-disk instead of cfg.ci, because installGates decides on its own
  // discovery signal (github-actions detection), not on trackfw.yaml's `ci:` key — a
  // project can have discover's workflow without cfg.ci ever being set.
  let discoverWorkflowExists = false
  try {
    fs.statSync(path.join(projectRoot, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH))
    discoverWorkflowExists = true
  } catch (_) {}
  if (discoverWorkflowExists) {
    const f = checkScaffoldArtifact(
      path.join(projectRoot, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH),
      DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
      buildDiscoverGitHubActionsWorkflowContent(),
      true,
      false,
    )
    if (f) findings.push(f)
  }

  // Deterministic output (AC7): sort by destination.
  findings.sort((a, b) => (a.destination < b.destination ? -1 : a.destination > b.destination ? 1 : 0))

  return findings
}

module.exports = {
  runScaffoldDoctor,
  checkValidateScriptArtifact,
  SCAFFOLD_DIVERGENT,
  SCAFFOLD_MISSING,
  SCAFFOLD_WRONG_MODE,
  PYTHON_VALIDATE_SCRIPT_FORM,
  _setPlatformForTest,
}
