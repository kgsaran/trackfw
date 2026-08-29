'use strict'

// ML-2G (ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version)
// — REQ-2026-08-28 AC17: the doctor's remedy for a stale
// .github/workflows/trackfw-validate.yml (`trackfw update`) was inert — no
// `update` target touched that file. `ci-workflow` now manages it too, with
// four rules:
//   (a) existence on disk (not cfg.ci) is the inclusion/refresh criterion —
//       same criterion the doctor (ML-2F) already uses.
//   (b) `update` NEVER creates the file for a project that doesn't have it.
//   (c) the target is declared when EITHER cfg.ci opts into github-actions/
//       gitlab-ci OR trackfw-validate.yml exists on disk.
//   (d) idempotent — a second run with the same binary does not report
//       "updated" again.
//
// See internal/generators/update.go's TestUpdateCiWorkflow* (Go, canonical
// reference this file mirrors) and pypi/tests' equivalent.
//
// EVERY test in this file redirects HOME to a scratch directory — never run
// `trackfw update` against the real HOME here (see update.test.js header note).

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const bin = path.resolve(__dirname, '../bin/trackfw')

const {
  writeCIWorkflow,
  buildDiscoverGitHubActionsWorkflowContent,
  DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH,
} = require('../src/commands/discover')
const { runScaffoldDoctor, SCAFFOLD_DIVERGENT } = require('../src/integrations/scaffold_doctor')

function scratch(extraYaml = '') {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-update-discover-workflow-test-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot, { recursive: true })
  fs.mkdirSync(homeRoot, { recursive: true })
  fs.writeFileSync(path.join(projectRoot, 'trackfw.yaml'), `hooks: none\n${extraYaml}`)
  return { base, projectRoot, homeRoot }
}

function run(args, cwd, homeRoot) {
  return spawnSync(process.execPath, [bin, ...args], {
    cwd,
    env: { ...process.env, HOME: homeRoot },
    encoding: 'utf8',
  })
}

const VALIDATE_ABS_REL = path.join('.github', 'workflows', 'trackfw-validate.yml')

function ciWorkflowTarget(doc) {
  return doc.targets.find((t) => t.id === 'ci-workflow')
}

// --- AC17(a)/(c): existence-on-disk is the criterion for a ci:none project ---

test('ci-workflow target is declared and refreshes a stale trackfw-validate.yml even when cfg.ci is none (AC17(a)/(c))', () => {
  const { projectRoot, homeRoot } = scratch('ci: none\n')
  writeCIWorkflow(projectRoot)
  const workflowPath = path.join(projectRoot, VALIDATE_ABS_REL)
  fs.writeFileSync(
    workflowPath,
    'name: trackfw validate\non: [push, pull_request]\njobs:\n  governance:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v0.0.1\n      - run: trackfw validate\n',
    'utf8',
  )

  const result = run(['update', '--json'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  const target = ciWorkflowTarget(doc)
  assert.ok(target, 'ci-workflow must be declared for ci:none when trackfw-validate.yml exists on disk')
  assert.equal(target.state, 'updated')

  const after = fs.readFileSync(workflowPath, 'utf8')
  assert.equal(after, buildDiscoverGitHubActionsWorkflowContent(), 'trackfw-validate.yml must be refreshed to the current template')
})

// --- AC17(b): `update` never creates trackfw-validate.yml ---

test('ci-workflow target never creates trackfw-validate.yml for a project that never had it, even with ci: github-actions and --install-missing (AC17(b))', () => {
  const { projectRoot, homeRoot } = scratch('ci: github-actions\n')

  const result = run(['update', '--json', '--install-missing'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  const target = ciWorkflowTarget(doc)
  assert.ok(target, 'ci-workflow must be declared for ci: github-actions')
  // trackfw-gate.yml gets installed (missing -> updated with --install-missing),
  // but that must not have created the sibling discover workflow.
  assert.equal(
    fs.existsSync(path.join(projectRoot, VALIDATE_ABS_REL)),
    false,
    'AC17(b) violated: trackfw update created trackfw-validate.yml for a project that never had it',
  )
})

// --- AC17(c): negative control — absent from disk AND ci:none means the target is not declared at all ---

test('ci-workflow target is not declared when cfg.ci is none and trackfw-validate.yml is absent', () => {
  const { projectRoot, homeRoot } = scratch('ci: none\n')
  const result = run(['update', '--json'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.ok(!ciWorkflowTarget(doc), 'ci-workflow must not be declared without cfg.ci opt-in or an existing trackfw-validate.yml')
})

// --- AC17(d): idempotency ---

test('ci-workflow target is idempotent for trackfw-validate.yml: second run reports skipped, not updated', () => {
  const { projectRoot, homeRoot } = scratch('ci: none\n')
  writeCIWorkflow(projectRoot)

  const first = run(['update', '--json'], projectRoot, homeRoot)
  assert.equal(first.status, 0, first.stderr)
  assert.equal(ciWorkflowTarget(JSON.parse(first.stdout)).state, 'skipped', 'writeCIWorkflow already wrote the current template, so the first update run should already be a no-op')

  const second = run(['update', '--json'], projectRoot, homeRoot)
  assert.equal(second.status, 0, second.stderr)
  assert.equal(ciWorkflowTarget(JSON.parse(second.stdout)).state, 'skipped')
})

// --- End-to-end: the doctor's remedy stops being inert ---

test('trackfw update closes the doctor scaffold-divergent finding for trackfw-validate.yml (end-to-end proof the remedy is no longer inert)', () => {
  const { projectRoot, homeRoot } = scratch('ci: none\n')
  writeCIWorkflow(projectRoot)
  const workflowPath = path.join(projectRoot, VALIDATE_ABS_REL)
  fs.writeFileSync(
    workflowPath,
    'name: trackfw validate\non: [push, pull_request]\njobs:\n  governance:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v0.0.1\n      - run: trackfw validate\n',
    'utf8',
  )

  const before = runScaffoldDoctor(projectRoot)
  const beforeFinding = before.find((f) => f.destination === DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
  assert.ok(beforeFinding, `doctor did not flag the stale ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} before update — test precondition failed`)
  assert.equal(beforeFinding.finding, SCAFFOLD_DIVERGENT)

  const result = run(['update', '--json'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  const after = runScaffoldDoctor(projectRoot)
  const afterFinding = after.find((f) => f.destination === DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH)
  assert.equal(afterFinding, undefined, `doctor still flags ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} after trackfw update — remedy is still inert`)
})

// --- Bare command (no flags): ML-2H corrective — the exact remedy the doctor
// prints ("trackfw update") is the bare command, no --json/--targets. In
// Go/Python this runtime has TWO code paths (a "simple" one used by the bare
// command, and a "targets" one used by --json/--targets) that had drifted;
// ML-2G only fixed the targets path there. Node.js never had that split —
// buildProjectTargets is the one function both the bare command and --json
// call (see commands/update.js's cmd.action) — but this test exercises the
// literal bare-command invocation anyway so a future refactor that
// reintroduces a second Node.js path is caught by the same proof used for
// the other two CLIs.

test('bare `trackfw update` (no flags) refreshes a stale trackfw-validate.yml and clears the doctor finding (ML-2H, bare-command proof)', () => {
  const { projectRoot, homeRoot } = scratch('ci: github-actions\n')
  writeCIWorkflow(projectRoot)
  const workflowPath = path.join(projectRoot, VALIDATE_ABS_REL)
  fs.writeFileSync(
    workflowPath,
    'name: trackfw validate\non: [push, pull_request]\njobs:\n  governance:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v7.0.0\n      - run: trackfw validate\n',
    'utf8',
  )

  const before = runScaffoldDoctor(projectRoot)
  assert.ok(
    before.find((f) => f.destination === DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH),
    'doctor did not flag the stale trackfw-validate.yml before update — test precondition failed',
  )

  const result = run(['update'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  const refreshed = fs.readFileSync(workflowPath, 'utf8')
  assert.equal(refreshed, buildDiscoverGitHubActionsWorkflowContent(), 'bare `trackfw update` did not refresh trackfw-validate.yml')

  const after = runScaffoldDoctor(projectRoot)
  assert.equal(
    after.find((f) => f.destination === DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH),
    undefined,
    'doctor still flags trackfw-validate.yml after bare `trackfw update` — remedy is still inert',
  )
})
