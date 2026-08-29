'use strict'

// ML-2E (ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version)
// — REQ-2026-08-28 AC9: `trackfw update`'s ci-workflow target must manage the SAME
// file pair Go manages (.github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml).
//
// ML-2G (AC17) expanded the target's scope: it now ALSO manages
// .github/workflows/trackfw-validate.yml (owned by discover.js), but only
// refresh-if-present — `update` never creates it (AC17(b)). See
// update_ci_workflow_target_discover_workflow.test.js for that half of the
// contract; the tests below (unchanged since ML-2E, except where AC17
// changed the observable `path` string) keep proving the trackfw-gate.yml/
// .gitlab-ci-trackfw.yml half. See internal/generators/update.go:1925-1938
// for the Go reference behavior this file mirrors.
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
const { version: PACKAGE_VERSION } = require('../package.json')

function scratch(extraYaml = '') {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-update-ci-target-test-'))
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

const GH_WORKFLOW_REL = path.join('.github', 'workflows', 'trackfw-gate.yml')
const GH_VALIDATE_REL = path.join('.github', 'workflows', 'trackfw-validate.yml')
const GITLAB_REL = '.gitlab-ci-trackfw.yml'

// --- AC9: reports updated and rewrites a stale pin, for GitHub Actions ---

test('ci-workflow target manages trackfw-gate.yml, rewrites a stale pin, and reports updated (AC9)', () => {
  const { projectRoot, homeRoot } = scratch('ci: github-actions\n')
  const workflowPath = path.join(projectRoot, GH_WORKFLOW_REL)
  fs.mkdirSync(path.dirname(workflowPath), { recursive: true })
  fs.writeFileSync(
    workflowPath,
    `name: trackfw-gate\nenv:\n  TRACKFW_VERSION: "0.0.1-stale"\nsteps: []\n`,
    'utf8',
  )

  const result = run(['update', '--json', '--targets', 'ci-workflow'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.equal(doc.targets.length, 1)
  assert.equal(doc.targets[0].id, 'ci-workflow')
  assert.equal(doc.targets[0].state, 'updated')

  const after = fs.readFileSync(workflowPath, 'utf8')
  assert.ok(
    after.includes(`TRACKFW_VERSION: "${PACKAGE_VERSION}"`),
    `expected the pin rewritten to the package.json version "${PACKAGE_VERSION}" in:\n${after}`,
  )
  assert.ok(!after.includes('0.0.1-stale'), 'stale pin must be gone')
})

// --- Idempotency: running twice with the same binary does not report updated twice ---

test('ci-workflow target is idempotent: a second update reports skipped, not updated', () => {
  const { projectRoot, homeRoot } = scratch('ci: github-actions\n')

  const first = run(['update', '--json', '--install-missing', '--targets', 'ci-workflow'], projectRoot, homeRoot)
  assert.equal(first.status, 0, first.stderr)
  assert.equal(JSON.parse(first.stdout).targets[0].state, 'updated')

  const second = run(['update', '--json', '--targets', 'ci-workflow'], projectRoot, homeRoot)
  assert.equal(second.status, 0, second.stderr)
  assert.equal(JSON.parse(second.stdout).targets[0].state, 'skipped')
})

// --- cfg.ci === 'gitlab-ci' manages .gitlab-ci-trackfw.yml, like Go ---

test('ci-workflow target manages .gitlab-ci-trackfw.yml when cfg.ci is gitlab-ci', () => {
  const { projectRoot, homeRoot } = scratch('ci: gitlab-ci\n')

  const result = run(['update', '--json', '--install-missing', '--targets', 'ci-workflow'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.equal(doc.targets[0].state, 'updated')

  const gitlabPath = path.join(projectRoot, GITLAB_REL)
  assert.ok(fs.existsSync(gitlabPath), '.gitlab-ci-trackfw.yml must have been written')
  const content = fs.readFileSync(gitlabPath, 'utf8')
  assert.ok(content.includes(`TRACKFW_VERSION: "${PACKAGE_VERSION}"`))
  assert.equal(fs.existsSync(path.join(projectRoot, GH_WORKFLOW_REL)), false, 'GitHub workflow must not be written for gitlab-ci')
})

// --- cfg.ci absent or none: target does not appear at all ---

test('ci-workflow target does not appear when cfg.ci is absent', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.ok(!doc.targets.some((t) => t.id === 'ci-workflow'), 'ci-workflow must not be a target when cfg.ci is absent')
})

test('ci-workflow target does not appear when cfg.ci is none', () => {
  const { projectRoot, homeRoot } = scratch('ci: none\n')
  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.ok(!doc.targets.some((t) => t.id === 'ci-workflow'), 'ci-workflow must not be a target when cfg.ci is none')
})

// --- Falsification: this test must FAIL if the target regresses to writing
// trackfw-validate.yml (the discover.js file) when it was never present —
// AC17(b): refresh-only-if-present, never create. ---

test('regression guard: ci-workflow target does NOT create trackfw-validate.yml when absent', () => {
  const { projectRoot, homeRoot } = scratch('ci: github-actions\n')

  const result = run(['update', '--json', '--install-missing', '--targets', 'ci-workflow'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)

  // Since ML-2G (AC17), the declared path for this target references all
  // three files it manages, trackfw-validate.yml included — it is declared,
  // even though this scratch project never had it installed.
  assert.ok(
    doc.targets[0].path.includes('trackfw-gate.yml'),
    `expected target path to reference trackfw-gate.yml, got: ${doc.targets[0].path}`,
  )
  assert.ok(
    doc.targets[0].path.includes('trackfw-validate.yml'),
    `expected target path to also reference trackfw-validate.yml (AC17), got: ${doc.targets[0].path}`,
  )
  // But it must not have CREATED that file as a side effect: it was never
  // installed by discover in this fixture, and AC17(b) forbids `update`
  // from installing it.
  assert.equal(
    fs.existsSync(path.join(projectRoot, GH_VALIDATE_REL)),
    false,
    'ci-workflow target must not create .github/workflows/trackfw-validate.yml when it was never present (AC17(b))',
  )
  assert.ok(fs.existsSync(path.join(projectRoot, GH_WORKFLOW_REL)), 'ci-workflow target must write trackfw-gate.yml')
})
