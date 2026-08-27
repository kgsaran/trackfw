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
 */

const fs = require('fs')
const path = require('path')
const yaml = require('yaml')

const { SIGNAL_SCRIPT, CLEANUP_SCRIPT, CREDENTIAL_GUARD_SCRIPT, GIT_BRANCH_GUARD_SCRIPT } =
  require('../generators/hooks')
const { CLAUDE_COMMANDS, buildValidateScript, buildGitHubActionsWorkflowContent, buildGitLabCIWorkflowContent } =
  require('../generators/init')

// Finding kind constants — mirrors Go's DoctorScaffoldDivergent / DoctorScaffoldMissing.
const SCAFFOLD_DIVERGENT = 'scaffold-divergent'
const SCAFFOLD_MISSING = 'scaffold-missing'

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
  if (actual === goNodeForm || actual === PYTHON_VALIDATE_SCRIPT_FORM) return null
  return {
    finding: SCAFFOLD_DIVERGENT,
    claim: { kind: '', item: '', target: '', surface: '', scope: '' },
    destination: relPath,
    remedy: scaffoldRemedy('resync', relPath),
  }
}

/**
 * checkScaffoldArtifact compares the on-disk content at absPath against expected.
 * Returns a finding if the file is divergent or (when reportMissing=true) absent.
 * relPath is used as destination in the finding.
 */
function checkScaffoldArtifact(absPath, relPath, expected, reportMissing) {
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
  if (actual === expected) return null
  return {
    finding: SCAFFOLD_DIVERGENT,
    claim: { kind: '', item: '', target: '', surface: '', scope: '' },
    destination: relPath,
    remedy: scaffoldRemedy('resync', relPath),
  }
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
  const validateF = checkValidateScriptArtifact(
    path.join(projectRoot, 'scripts/trackfw-validate.sh'),
    'scripts/trackfw-validate.sh',
    cfg,
  )
  if (validateF) findings.push(validateF)

  const staticScripts = [
    { relPath: 'scripts/trackfw-attention-signal.sh', expected: SIGNAL_SCRIPT },
    { relPath: 'scripts/trackfw-attention-cleanup.sh', expected: CLEANUP_SCRIPT },
    { relPath: 'scripts/trackfw-credential-guard.sh', expected: CREDENTIAL_GUARD_SCRIPT },
    { relPath: 'scripts/trackfw-git-branch-guard.sh', expected: GIT_BRANCH_GUARD_SCRIPT },
  ]
  for (const { relPath, expected } of staticScripts) {
    const f = checkScaffoldArtifact(path.join(projectRoot, relPath), relPath, expected, true)
    if (f) findings.push(f)
  }

  // --- Slash commands (AC14: only when the directory already exists) ---
  const claudeDir = path.join(projectRoot, CLAUDE_COMMANDS_DIR_PATH)
  let claudeDirExists = false
  try {
    fs.statSync(claudeDir)
    claudeDirExists = true
  } catch (_) {}
  if (claudeDirExists) {
    for (const [filename, content] of Object.entries(CLAUDE_COMMANDS)) {
      const relPath = `${CLAUDE_COMMANDS_DIR_PATH}/${filename}`
      const f = checkScaffoldArtifact(path.join(projectRoot, relPath), relPath, content, true)
      if (f) findings.push(f)
    }
  }

  // --- CI workflow (AC13: conditional on ci: in trackfw.yaml) ---
  if (cfg.ci === 'github-actions') {
    const relPath = GITHUB_ACTIONS_WORKFLOW_PATH
    const f = checkScaffoldArtifact(
      path.join(projectRoot, relPath),
      relPath,
      buildGitHubActionsWorkflowContent(cfg),
      true,
    )
    if (f) findings.push(f)
  } else if (cfg.ci === 'gitlab-ci') {
    const relPath = GITLAB_CI_WORKFLOW_PATH
    const f = checkScaffoldArtifact(
      path.join(projectRoot, relPath),
      relPath,
      buildGitLabCIWorkflowContent(cfg),
      true,
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
  PYTHON_VALIDATE_SCRIPT_FORM,
}
