'use strict'

// Corrective falsifier for the symlink-follow arbitrary-write reported by
// hades-tf's final barrier review (2026-08-28): `trackfw update` and
// `trackfw discover --init` decided the presence of
// .github/workflows/trackfw-validate.yml with fs.existsSync/fs.statSync
// (both follow symlinks). A LIVE symlink at that path pointing OUTSIDE the
// project let `trackfw update` overwrite the linked-to file even in a
// `ci: none` project (which never asked for CI management at all); a
// DANGLING symlink let `trackfw discover --init` CREATE a file at whatever
// path the attacker chose. Fixed by deciding presence with fs.lstatSync and
// refusing (loudly, on stderr) to write through a symlink either way.
//
// See internal/generators/update_test.go's
// TestUpdateNeverWritesThroughSymlinkAtDiscoverWorkflowPath /
// TestUpdateNeverWritesThroughDanglingSymlinkAtDiscoverWorkflowPath and
// internal/discover/discover_test.go's
// TestWriteCIWorkflow_NeverWritesThroughLiveSymlink /
// TestWriteCIWorkflow_NeverWritesThroughDanglingSymlink (Go, canonical
// reference these mirror) and pypi/tests' equivalent.
//
// EVERY test in this file redirects HOME to a scratch directory — never run
// `trackfw update`/`trackfw discover` against the real HOME here.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const bin = path.resolve(__dirname, '../bin/trackfw')

function scratch(extraYaml = '') {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-symlink-guard-test-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  const outsideRoot = path.join(base, 'outside')
  fs.mkdirSync(projectRoot, { recursive: true })
  fs.mkdirSync(homeRoot, { recursive: true })
  fs.mkdirSync(outsideRoot, { recursive: true })
  fs.writeFileSync(path.join(projectRoot, 'trackfw.yaml'), `hooks: none\n${extraYaml}`)
  return { base, projectRoot, homeRoot, outsideRoot }
}

function run(args, cwd, homeRoot) {
  return spawnSync(process.execPath, [bin, ...args], {
    cwd,
    env: { ...process.env, HOME: homeRoot },
    encoding: 'utf8',
  })
}

const VALIDATE_ABS_REL = path.join('.github', 'workflows', 'trackfw-validate.yml')

// symlinkOrSkip mirrors fs.symlinkSync, but if creation fails because the
// process lacks the privilege Windows requires to create symlinks
// (Developer Mode or an elevated process — WinError 1314,
// ERROR_PRIVILEGE_NOT_HELD), it skips the calling test via node:test's
// TestContext#skip instead of throwing, naming the guarantee that was not
// exercised. Detection is on the CONDITION (the failed syscall), not on
// process.platform: on a Windows runner with Developer Mode enabled, or on
// Linux/macOS, fs.symlinkSync succeeds and the test executes normally.
// Returns true if the symlink was created (caller should proceed), false if
// the test was skipped (caller must return immediately).
function symlinkOrSkip(t, target, link) {
  try {
    fs.symlinkSync(target, link)
    return true
  } catch (err) {
    if (err && (err.code === 'EPERM' || err.code === 'EACCES')) {
      t.skip(`guarda de escrita através de symlink não exercitada: criação de symlink exige Developer Mode (ou processo elevado) neste Windows: ${err.message}`)
      return false
    }
    throw err
  }
}

test('trackfw update (ci: none) never writes through a live symlink pointing outside the project', (t) => {
  const { projectRoot, homeRoot, outsideRoot } = scratch('ci: none\n')
  const victim = path.join(outsideRoot, 'vitima.txt')
  const originalContent = 'CONTEUDO ORIGINAL DA VITIMA\n'
  fs.writeFileSync(victim, originalContent, 'utf8')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, victim, link)) return

  const result = run(['update'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  const after = fs.readFileSync(victim, 'utf8')
  assert.equal(after, originalContent, 'symlink-follow arbitrary write: victim file outside the project was overwritten')

  const linkInfo = fs.lstatSync(link)
  assert.ok(linkInfo.isSymbolicLink(), 'trackfw-validate.yml symlink should remain untouched')
})

test('trackfw discover --init never creates a file through a dangling symlink outside the project', (t) => {
  const { projectRoot, homeRoot, outsideRoot } = scratch('ci: github-actions\n')
  const danglingTarget = path.join(outsideRoot, 'does-not-exist-yet')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, danglingTarget, link)) return

  const result = run(['discover', '--init'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  assert.equal(
    fs.existsSync(danglingTarget),
    false,
    'dangling-symlink arbitrary write: trackfw discover --init created a file outside the project',
  )
})

test('discover.writeCIWorkflow refuses a live symlink pointing outside the project and warns on stderr', (t) => {
  const { writeCIWorkflow, DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH } = require('../src/commands/discover')
  const { projectRoot, outsideRoot } = scratch('ci: github-actions\n')
  const victim = path.join(outsideRoot, 'vitima.txt')
  const originalContent = 'CONTEUDO ORIGINAL DA VITIMA\n'
  fs.writeFileSync(victim, originalContent, 'utf8')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, victim, link)) return

  const originalError = console.error
  let stderrOutput = ''
  console.error = (msg) => { stderrOutput += String(msg) }
  try {
    writeCIWorkflow(projectRoot)
  } finally {
    console.error = originalError
  }

  assert.equal(fs.readFileSync(victim, 'utf8'), originalContent)
  assert.ok(stderrOutput.includes(DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH) && stderrOutput.includes('symlink'), `expected a warning naming ${DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} as a symlink, got: ${stderrOutput}`)
})

test('update.js refreshDiscoverGitHubActionsWorkflowIfPresent (via ci: github-actions update) refuses a live symlink and warns on stderr', (t) => {
  const { projectRoot, homeRoot, outsideRoot } = scratch('ci: github-actions\n')
  const victim = path.join(outsideRoot, 'vitima.txt')
  const originalContent = 'CONTEUDO ORIGINAL DA VITIMA\n'
  fs.writeFileSync(victim, originalContent, 'utf8')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, victim, link)) return

  const result = run(['update'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.match(result.stderr, /trackfw-validate\.yml.*symlink/)

  assert.equal(fs.readFileSync(victim, 'utf8'), originalContent, 'update wrote through the symlink despite cfg.ci=github-actions declaring ci-workflow')
})

// ML-3E (audit of ML-3D): with `ci: none`, discoverWorkflowPresent(cwd)
// treats a symlink at trackfw-validate.yml as NOT present, so the
// `ci-workflow` target used to go entirely undeclared for that case — no
// write (correct, ML-3D), but also NO WARNING (the silent-failure gap this
// ML closes). Go/Python call the refresh-if-present writer unconditionally
// from the bare `update` path regardless of whether ci-workflow is declared
// (internal/generators/update.go:107, pypi/trackfw/commands/update.py:462-463)
// — this exact string is byte-identical across all three runtimes (see
// internal/generators/update.go:1899,
// pypi/trackfw/commands/update.py:221-223).
const EXPECTED_SYMLINK_WARNING = 'aviso: .github/workflows/trackfw-validate.yml é um symlink; trackfw update não escreve através de symlinks — arquivo não foi tocado'

test('trackfw update (ci: none, bare command) warns on stderr about a live symlink at trackfw-validate.yml instead of going silent', (t) => {
  const { projectRoot, homeRoot, outsideRoot } = scratch('ci: none\n')
  const victim = path.join(outsideRoot, 'vitima.txt')
  const originalContent = 'CONTEUDO ORIGINAL DA VITIMA\n'
  fs.writeFileSync(victim, originalContent, 'utf8')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, victim, link)) return

  const result = run(['update'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.ok(
    result.stderr.includes(EXPECTED_SYMLINK_WARNING),
    `expected the byte-identical Go/Python symlink warning on stderr, got: ${JSON.stringify(result.stderr)}`,
  )

  assert.equal(fs.readFileSync(victim, 'utf8'), originalContent, 'victim file outside the project was touched')
  assert.ok(fs.lstatSync(link).isSymbolicLink(), 'trackfw-validate.yml symlink should remain untouched')
})

test('trackfw update (ci: none, bare command) warns on stderr about a DANGLING symlink at trackfw-validate.yml', (t) => {
  const { projectRoot, homeRoot, outsideRoot } = scratch('ci: none\n')
  const danglingTarget = path.join(outsideRoot, 'does-not-exist-yet')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, danglingTarget, link)) return

  const result = run(['update'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.ok(
    result.stderr.includes(EXPECTED_SYMLINK_WARNING),
    `expected the byte-identical Go/Python symlink warning on stderr, got: ${JSON.stringify(result.stderr)}`,
  )
  assert.equal(fs.existsSync(danglingTarget), false, 'dangling symlink target should not have been created')
})

test('trackfw update --targets ci-workflow (ci: none) warns on stderr about a live symlink at trackfw-validate.yml', (t) => {
  const { projectRoot, homeRoot, outsideRoot } = scratch('ci: none\n')
  const victim = path.join(outsideRoot, 'vitima.txt')
  const originalContent = 'CONTEUDO ORIGINAL DA VITIMA\n'
  fs.writeFileSync(victim, originalContent, 'utf8')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, victim, link)) return

  const result = run(['update', '--targets', 'ci-workflow'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.ok(
    result.stderr.includes(EXPECTED_SYMLINK_WARNING),
    `expected the byte-identical Go/Python symlink warning on stderr, got: ${JSON.stringify(result.stderr)}`,
  )
  assert.equal(fs.readFileSync(victim, 'utf8'), originalContent, 'victim file outside the project was touched')
})

test('trackfw update --targets ci-workflow --json (ci: none) still warns on stderr about the symlink and keeps stdout as clean JSON', (t) => {
  const { projectRoot, homeRoot, outsideRoot } = scratch('ci: none\n')
  const victim = path.join(outsideRoot, 'vitima.txt')
  const originalContent = 'CONTEUDO ORIGINAL DA VITIMA\n'
  fs.writeFileSync(victim, originalContent, 'utf8')

  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const link = path.join(workflowsDir, 'trackfw-validate.yml')
  if (!symlinkOrSkip(t, victim, link)) return

  const result = run(['update', '--targets', 'ci-workflow', '--json'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.ok(
    result.stderr.includes(EXPECTED_SYMLINK_WARNING),
    `expected the byte-identical Go/Python symlink warning on stderr, got: ${JSON.stringify(result.stderr)}`,
  )
  assert.equal(fs.readFileSync(victim, 'utf8'), originalContent, 'victim file outside the project was touched')

  const doc = JSON.parse(result.stdout)
  assert.equal(doc.scope, 'project')
  assert.deepEqual(doc.targets, [], 'ci-workflow is not declared when ci: none and the file on disk is a symlink — stdout must stay the same clean JSON document, warning is stderr-only')
})

test('trackfw update (ci: none) still refreshes a REGULAR (non-symlink) trackfw-validate.yml, without a symlink warning', () => {
  const { projectRoot, homeRoot } = scratch('ci: none\n')
  const workflowsDir = path.join(projectRoot, '.github', 'workflows')
  fs.mkdirSync(workflowsDir, { recursive: true })
  const target = path.join(workflowsDir, 'trackfw-validate.yml')
  fs.writeFileSync(target, 'OLD PINNED CONTENT\n', 'utf8')

  const result = run(['update'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.ok(!result.stderr.includes('symlink'), `regular file must not trigger the symlink warning, got: ${JSON.stringify(result.stderr)}`)

  const after = fs.readFileSync(target, 'utf8')
  assert.notEqual(after, 'OLD PINNED CONTENT\n', 'regular trackfw-validate.yml with a stale pin should have been refreshed (ML-3D behaviour, no regression)')
})

test('trackfw update (ci: none) does not create trackfw-validate.yml when absent, and does not warn', () => {
  const { projectRoot, homeRoot } = scratch('ci: none\n')

  const result = run(['update'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.ok(!result.stderr.includes('symlink'), `absent file must not trigger the symlink warning, got: ${JSON.stringify(result.stderr)}`)

  const target = path.join(projectRoot, '.github', 'workflows', 'trackfw-validate.yml')
  assert.equal(fs.existsSync(target), false, 'update must never create trackfw-validate.yml (AC17(b))')
})
