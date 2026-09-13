'use strict'

// Tests for ADR-2026-08-28 (gate de CI gerado nasce pinado na versão que o gerou):
// buildGitHubActionsWorkflowContent / buildGitLabCIWorkflowContent must pin
// TRACKFW_VERSION to the version of the trackfw binary that generated the
// workflow (npm/package.json's "version"), never a literal. Mirrors
// internal/generators/scaffold_test.go (ML-2A) and pypi/tests equivalent (ML-2C).
//
// Covers ROADMAP-2026-08-28 ML-2B acceptance criteria: AC6, AC7 (Node),
// AC10, AC11.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const {
  buildGitHubActionsWorkflowContent,
  buildGitLabCIWorkflowContent,
  generateCIWorkflow,
  writeTrackfwConfig,
} = require('../src/generators/init')
const { runScaffoldDoctor, SCAFFOLD_DIVERGENT } = require('../src/integrations/scaffold_doctor')

const { version: PACKAGE_VERSION } = require('../package.json')

const EMPTY_CFG = { backend: null, frontend: null, pkgManager: null, ci: 'github-actions' }

function withTmpProject(fn) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ci-pin-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    return fn(tmpDir)
  } finally {
    process.chdir(origCwd)
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
}

// --- AC6/AC7: builders pin TRACKFW_VERSION to package.json's version ---

test('buildGitHubActionsWorkflowContent pins TRACKFW_VERSION to package.json version', () => {
  const content = buildGitHubActionsWorkflowContent(EMPTY_CFG)
  assert.ok(
    content.includes(`TRACKFW_VERSION: "${PACKAGE_VERSION}"`),
    `expected TRACKFW_VERSION: "${PACKAGE_VERSION}" in:\n${content}`,
  )
  assert.ok(content.includes('timeout-minutes: 10'), 'expected timeout-minutes: 10')
  // env: block must sit under the governance job, before steps:
  const envIdx = content.indexOf('env:')
  const stepsIdx = content.indexOf('steps:')
  assert.ok(envIdx > -1 && stepsIdx > -1 && envIdx < stepsIdx, 'env: must precede steps:')
})

test('buildGitLabCIWorkflowContent pins TRACKFW_VERSION to package.json version', () => {
  const content = buildGitLabCIWorkflowContent(EMPTY_CFG)
  assert.ok(
    content.includes(`TRACKFW_VERSION: "${PACKAGE_VERSION}"`),
    `expected TRACKFW_VERSION: "${PACKAGE_VERSION}" in:\n${content}`,
  )
  const varsIdx = content.indexOf('variables:')
  const beforeScriptIdx = content.indexOf('before_script:')
  assert.ok(varsIdx > -1 && beforeScriptIdx > -1 && varsIdx < beforeScriptIdx, 'variables: must precede before_script:')
  assert.ok(
    content.includes('timeout: 10 minutes'),
    'expected timeout: 10 minutes (GitLab analogue of GitHub Actions\' timeout-minutes: 10 — ADR-2026-08-28)',
  )
  const timeoutIdx = content.indexOf('timeout: 10 minutes')
  assert.ok(timeoutIdx > -1 && timeoutIdx < varsIdx, 'timeout: 10 minutes must precede variables:')
})

test('no hardcoded version literal in the CI workflow builders (source-level)', () => {
  const initSrc = fs.readFileSync(path.join(__dirname, '..', 'src', 'generators', 'init.js'), 'utf8')
  const start = initSrc.indexOf('function buildGitHubActionsWorkflowContent')
  const end = initSrc.indexOf('function generateGitHubActionsWorkflow')
  assert.ok(start > -1 && end > start, 'could not locate builder block in init.js')
  const block = initSrc.slice(start, end)
  // The only version-shaped token allowed in the block is the ${PACKAGE_VERSION}
  // interpolation itself — no literal semver string may appear (including pre-release forms
  // such as "8.0.0-rc1" or "v8.0.0-rc1" that the original three-component regex missed).
  const semverLiteral = /["'`]v?\d+\.\d+\.\d+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?["'`]/
  assert.ok(!semverLiteral.test(block), `found a literal semver string in builder block:\n${block}`)
  assert.ok(block.includes('${PACKAGE_VERSION}'), 'builders must interpolate PACKAGE_VERSION, not a literal')
})

// Falsification — direction A: literal prerelease injected → guard reproves.
// Conclusion: semverLiteral catches "X.Y.Z-rc1" (and "vX.Y.Z-rc1") — the gap that
// existed before this fix, where only stable "X.Y.Z" literals were detected.
test('no hardcoded version literal guard detects injected prerelease literal (falsify direction-A)', () => {
  const semverLiteral = /["'`]v?\d+\.\d+\.\d+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?["'`]/
  // Simulate a builder block that accidentally contains a hardcoded prerelease literal.
  const tamperedBlock = 'const v = "8.0.0-rc1"; // literal instead of ${PACKAGE_VERSION}'
  assert.ok(
    semverLiteral.test(tamperedBlock),
    'guard must DETECT the injected prerelease literal "8.0.0-rc1" — if this fails the fix is broken',
  )
  const tamperedBlockV = 'const v = "v8.0.0-rc1"; // literal with v-prefix'
  assert.ok(
    semverLiteral.test(tamperedBlockV),
    'guard must DETECT the injected v-prefix prerelease literal "v8.0.0-rc1" — if this fails the fix is broken',
  )
})

// Falsification — direction B: no literal injected → guard passes.
// Conclusion: semverLiteral does NOT match ${PACKAGE_VERSION} interpolations,
// so correctly governed blocks remain green under the stricter regex.
test('no hardcoded version literal guard passes clean block (falsify direction-B)', () => {
  const semverLiteral = /["'`]v?\d+\.\d+\.\d+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?["'`]/
  const cleanBlock = 'const v = `TRACKFW_VERSION: "${PACKAGE_VERSION}"`'
  assert.ok(
    !semverLiteral.test(cleanBlock),
    'guard must NOT flag a block using ${PACKAGE_VERSION} interpolation — if this fails the fix over-matches',
  )
})

// --- Idempotency: generating twice produces byte-identical output ---

test('buildGitHubActionsWorkflowContent is idempotent across calls', () => {
  const a = buildGitHubActionsWorkflowContent(EMPTY_CFG)
  const b = buildGitHubActionsWorkflowContent(EMPTY_CFG)
  assert.equal(a, b)
})

test('buildGitLabCIWorkflowContent is idempotent across calls', () => {
  const a = buildGitLabCIWorkflowContent(EMPTY_CFG)
  const b = buildGitLabCIWorkflowContent(EMPTY_CFG)
  assert.equal(a, b)
})

// --- AC11: doctor reports no mismatches right after generation ---

test('doctor reports no scaffold-divergent for ci-workflow right after init (AC11)', () => {
  withTmpProject((root) => {
    writeTrackfwConfig(EMPTY_CFG)
    generateCIWorkflow(EMPTY_CFG)
    const findings = runScaffoldDoctor(root)
    const ciFindings = findings.filter((f) => f.destination === '.github/workflows/trackfw-gate.yml')
    assert.deepEqual(ciFindings, [], `expected no findings for ci-workflow, got: ${JSON.stringify(ciFindings)}`)
  })
})

// --- AC10: doctor reports scaffold-divergent when the pin is hand-edited to another version ---

test('doctor reports scaffold-divergent when TRACKFW_VERSION pin is hand-edited (AC10)', () => {
  withTmpProject((root) => {
    writeTrackfwConfig(EMPTY_CFG)
    generateCIWorkflow(EMPTY_CFG)
    const workflowPath = path.join(root, '.github', 'workflows', 'trackfw-gate.yml')
    const original = fs.readFileSync(workflowPath, 'utf8')
    const tampered = original.replace(`TRACKFW_VERSION: "${PACKAGE_VERSION}"`, 'TRACKFW_VERSION: "0.0.1-not-the-real-version"')
    assert.notEqual(tampered, original, 'precondition: tamper must actually change the file')
    fs.writeFileSync(workflowPath, tampered, 'utf8')

    const findings = runScaffoldDoctor(root)
    const ciFinding = findings.find((f) => f.destination === '.github/workflows/trackfw-gate.yml')
    assert.ok(ciFinding, 'expected a finding for the hand-edited ci-workflow')
    assert.equal(ciFinding.finding, SCAFFOLD_DIVERGENT)
  })
})

test('doctor reports no scaffold-divergent for gitlab-ci pin right after init (AC11)', () => {
  withTmpProject((root) => {
    const cfg = { ...EMPTY_CFG, ci: 'gitlab-ci' }
    writeTrackfwConfig(cfg)
    generateCIWorkflow(cfg)
    const findings = runScaffoldDoctor(root)
    const ciFindings = findings.filter((f) => f.destination === '.gitlab-ci-trackfw.yml')
    assert.deepEqual(ciFindings, [], `expected no findings for gitlab-ci workflow, got: ${JSON.stringify(ciFindings)}`)
  })
})
