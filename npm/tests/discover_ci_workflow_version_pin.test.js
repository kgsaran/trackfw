'use strict'

// Tests for ROADMAP-2026-08-28 ML-2F (corrective, Wave 2): the SECOND install
// mechanism — `go install github.com/kgsaran/trackfw/cmd/trackfw@latest`, written to
// .github/workflows/trackfw-validate.yml by `trackfw discover --init` (installGates) —
// was never pinned by ML-2A/2B/2C, which only covered trackfw-gate.yml (the install.sh
// mechanism). Mirrors internal/discover/discover_test.go (Go, canonical source of
// truth for this second surface) and pypi/tests equivalent.
//
// Covers ML-2F acceptance criteria: pin to package.json version (never literal),
// byte-identity with the Go template for the same version, doctor coverage
// (no mismatches right after generation; scaffold-divergent when the pin is stale),
// idempotence.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const {
  writeCIWorkflow,
  buildDiscoverGitHubActionsWorkflowContent,
  DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
} = require('../src/commands/discover')
const { runScaffoldDoctor, SCAFFOLD_DIVERGENT } = require('../src/integrations/scaffold_doctor')

const { version: PACKAGE_VERSION } = require('../package.json')

function withTmpProject(fn) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-discover-ci-pin-'))
  try {
    return fn(tmpDir)
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
}

function readWorkflow(tmpDir) {
  return fs.readFileSync(path.join(tmpDir, '.github', 'workflows', 'trackfw-validate.yml'), 'utf8')
}

// --- pin to package.json version, never literal ---

test('writeCIWorkflow pins go install to package.json version, not @latest', () => {
  withTmpProject((tmpDir) => {
    writeCIWorkflow(tmpDir)
    const content = readWorkflow(tmpDir)
    assert.ok(
      content.includes(`go install github.com/kgsaran/trackfw/cmd/trackfw@v${PACKAGE_VERSION}`),
      `expected pinned go install line for v${PACKAGE_VERSION} in:\n${content}`,
    )
    assert.ok(!content.includes('trackfw/cmd/trackfw@latest'), `unpinned @latest still present:\n${content}`)
  })
})

test('go install pin is not hardcoded — tracks package.json version when it changes', () => {
  // Falsifies the specific regression the ADR warns against: a template with `@v7.3.0`
  // typed literally into the generator source would pass the assertion above today,
  // because PACKAGE_VERSION happens to equal "7.3.0" right now. Mutating the cached
  // package.json object (require() caches it, and buildDiscoverGitHubActionsWorkflowContent
  // re-requires it lazily on every call) and re-generating is the only way to prove the
  // pin tracks the variable, not a literal.
  const pkg = require('../package.json')
  const origVersion = pkg.version
  pkg.version = '9.9.9-stub'
  try {
    withTmpProject((tmpDir) => {
      writeCIWorkflow(tmpDir)
      const content = readWorkflow(tmpDir)
      assert.ok(
        content.includes('trackfw@v9.9.9-stub'),
        `stubbed package.json version did not propagate to the template, got:\n${content}`,
      )
      assert.ok(
        !content.includes(origVersion),
        `template still contains the real version ${origVersion} after stubbing — pin looks hardcoded, got:\n${content}`,
      )
    })
  } finally {
    pkg.version = origVersion
  }
})

test('buildDiscoverGitHubActionsWorkflowContent output is exactly what writeCIWorkflow writes', () => {
  withTmpProject((tmpDir) => {
    writeCIWorkflow(tmpDir)
    const onDisk = readWorkflow(tmpDir)
    assert.equal(onDisk, buildDiscoverGitHubActionsWorkflowContent())
  })
})

test('buildDiscoverGitHubActionsWorkflowContent is idempotent across calls', () => {
  const first = buildDiscoverGitHubActionsWorkflowContent()
  const second = buildDiscoverGitHubActionsWorkflowContent()
  assert.equal(first, second)
})

test('writeCIWorkflow does not overwrite an existing file (idempotent across process runs)', () => {
  withTmpProject((tmpDir) => {
    const dir = path.join(tmpDir, '.github', 'workflows')
    fs.mkdirSync(dir, { recursive: true })
    fs.writeFileSync(path.join(dir, 'trackfw-validate.yml'), '# existing\n', 'utf8')
    writeCIWorkflow(tmpDir)
    assert.equal(readWorkflow(tmpDir), '# existing\n')
  })
})

// --- doctor coverage (AC10/AC11 applied to the second surface) ---

test('doctor reports no mismatch for trackfw-validate.yml right after generation', () => {
  withTmpProject((tmpDir) => {
    fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'backend: go\n', 'utf8')
    writeCIWorkflow(tmpDir)
    const findings = runScaffoldDoctor(tmpDir)
    const hit = findings.find((f) => f.destination === DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
    assert.equal(hit, undefined, `expected no finding, got: ${JSON.stringify(hit)}`)
  })
})

test('doctor reports scaffold-divergent when the pin is manually changed', () => {
  withTmpProject((tmpDir) => {
    fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'backend: go\n', 'utf8')
    writeCIWorkflow(tmpDir)
    const workflowPath = path.join(tmpDir, '.github', 'workflows', 'trackfw-validate.yml')
    const original = fs.readFileSync(workflowPath, 'utf8')
    const tampered = original.replace(`trackfw@v${PACKAGE_VERSION}`, 'trackfw@v0.0.1-stale')
    assert.notEqual(tampered, original, 'tamper substitution did not change content — test setup broken')
    fs.writeFileSync(workflowPath, tampered, 'utf8')

    const findings = runScaffoldDoctor(tmpDir)
    const hit = findings.find((f) => f.destination === DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
    assert.ok(hit, `expected a finding for ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH}, got none in: ${JSON.stringify(findings)}`)
    assert.equal(hit.finding, SCAFFOLD_DIVERGENT)
  })
})

test('doctor does not report anything for trackfw-validate.yml when the file is absent', () => {
  withTmpProject((tmpDir) => {
    fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'backend: go\n', 'utf8')
    const findings = runScaffoldDoctor(tmpDir)
    const hit = findings.find((f) => f.destination === DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
    assert.equal(hit, undefined, `expected no finding for an absent file, got: ${JSON.stringify(hit)}`)
  })
})
