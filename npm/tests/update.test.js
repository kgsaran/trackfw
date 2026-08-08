'use strict'

// ML-6C (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador)
// — see docs/cli-parity.md, "`trackfw update` vs `trackfw update harness`".
//
// EVERY test in this file redirects HOME to a scratch directory. Never run
// `trackfw update` / `trackfw update harness` against the real HOME here.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const bin = path.resolve(__dirname, '../bin/trackfw')

function scratch() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-update-test-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot, { recursive: true })
  fs.mkdirSync(homeRoot, { recursive: true })
  fs.writeFileSync(path.join(projectRoot, 'trackfw.yaml'), 'hooks: none\nci: none\n')
  return { base, projectRoot, homeRoot }
}

function run(args, cwd, homeRoot) {
  return spawnSync(process.execPath, [bin, ...args], {
    cwd,
    env: { ...process.env, HOME: homeRoot },
    encoding: 'utf8',
  })
}

test('update requires trackfw.yaml and exits non-zero without it', () => {
  const { projectRoot, homeRoot } = scratch()
  fs.rmSync(path.join(projectRoot, 'trackfw.yaml'))
  const result = run(['update'], projectRoot, homeRoot)
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /trackfw\.yaml/)
})

test('update --json emits scope=project, dry_run flag, and the four-key summary in fixed key order', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.deepEqual(Object.keys(doc), ['scope', 'dry_run', 'targets', 'summary'])
  assert.equal(doc.scope, 'project')
  assert.equal(doc.dry_run, true)
  assert.deepEqual(Object.keys(doc.summary), ['updated', 'skipped', 'missing', 'failed'])
  for (const t of doc.targets) assert.deepEqual(Object.keys(t), ['id', 'state', 'path'])
})

test('a fresh project reports every target missing under --dry-run and writes nothing', () => {
  const { projectRoot, homeRoot } = scratch()
  const before = fs.readdirSync(projectRoot).sort()
  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.ok(doc.targets.length > 0)
  for (const t of doc.targets) assert.equal(t.state, 'missing', `${t.id} expected missing`)
  assert.equal(doc.summary.missing, doc.targets.length)
  assert.equal(doc.summary.updated, 0)
  const after = fs.readdirSync(projectRoot).sort()
  assert.deepEqual(before, after, '--dry-run must not create any file in the project')
})

test('missing never installs without --install-missing, and installs only that subset with it', () => {
  const { projectRoot, homeRoot } = scratch()
  const dry = JSON.parse(run(['update', '--json', '--targets', 'validate-script'], projectRoot, homeRoot).stdout)
  assert.equal(dry.targets[0].state, 'missing')
  assert.equal(fs.existsSync(path.join(projectRoot, 'scripts', 'trackfw-validate.sh')), false)

  const installed = run(['update', '--json', '--install-missing', '--targets', 'validate-script'], projectRoot, homeRoot)
  assert.equal(installed.status, 0, installed.stderr)
  const doc = JSON.parse(installed.stdout)
  assert.equal(doc.targets.length, 1)
  assert.equal(doc.targets[0].id, 'validate-script')
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(fs.existsSync(path.join(projectRoot, 'scripts', 'trackfw-validate.sh')), true)
})

test('--targets restricts which targets are computed AND applied — unselected targets are left untouched', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['update', '--json', '--install-missing', '--targets', 'validate-script'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.deepEqual(doc.targets.map(t => t.id), ['validate-script'])
  // claude-commands is a real, always-applicable target in the full
  // universe — it must NOT have been written just because it exists in the
  // default target list.
  assert.equal(fs.existsSync(path.join(projectRoot, '.claude', 'commands', 'trackfw')), false)
})

test('re-running update after install-missing reports skipped (idempotent, no further writes)', () => {
  const { projectRoot, homeRoot } = scratch()
  run(['update', '--json', '--install-missing', '--targets', 'validate-script'], projectRoot, homeRoot)
  const script = path.join(projectRoot, 'scripts', 'trackfw-validate.sh')
  const before = fs.readFileSync(script, 'utf8')
  const again = JSON.parse(run(['update', '--json', '--targets', 'validate-script'], projectRoot, homeRoot).stdout)
  assert.equal(again.targets[0].state, 'skipped')
  assert.equal(fs.readFileSync(script, 'utf8'), before)
})

test('unknown --targets id is a usage error with non-zero exit', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['update', '--targets', 'not-a-real-target'], projectRoot, homeRoot)
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /Unknown update target/)
})

test('update never touches global state (HOME) — only project files are written', () => {
  const { projectRoot, homeRoot } = scratch()
  const homeBefore = fs.readdirSync(homeRoot)
  run(['update', '--install-missing'], projectRoot, homeRoot)
  const homeAfter = fs.readdirSync(homeRoot)
  assert.deepEqual(homeBefore, homeAfter, 'trackfw update must not create anything under HOME')
  assert.equal(fs.existsSync(path.join(homeRoot, '.claude')), false)
})

test('update --json output is pure JSON with no human progress noise mixed in', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['update', '--json', '--install-missing'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)
  assert.doesNotThrow(() => JSON.parse(result.stdout))
})

// ML-1B (ROADMAP-2026-08-08-conectar-adrs-globais...) — ensureGlobalAdrDirRegistered
// registers ~/.trackfw/adr in trackfw.yaml's adr_dirs, surgically and only
// when the global ADR dir exists and has >=1 ADR-*.md file.

function writeGlobalAdr(homeRoot, filename = 'ADR-2026-08-08-example.md') {
  const globalAdrDir = path.join(homeRoot, '.trackfw', 'adr')
  fs.mkdirSync(globalAdrDir, { recursive: true })
  fs.writeFileSync(path.join(globalAdrDir, filename), '# Example ADR\n')
  return globalAdrDir
}

test('adr_dirs auto-register: no adr_dirs key + global ADR dir with 1 ADR -> gains block with both paths', () => {
  const { projectRoot, homeRoot } = scratch()
  writeGlobalAdr(homeRoot)
  const yamlPath = path.join(projectRoot, 'trackfw.yaml')
  const before = fs.readFileSync(yamlPath, 'utf8')

  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  const after = fs.readFileSync(yamlPath, 'utf8')
  assert.notEqual(after, before)
  assert.match(after, /adr_dirs:\n  - docs\/adr\n  - ~\/\.trackfw\/adr\n/)
  // Original content preserved verbatim, just appended to.
  assert.ok(after.startsWith(before))
})

test('adr_dirs auto-register: global ADR dir does not exist -> trackfw.yaml unchanged byte-for-byte', () => {
  const { projectRoot, homeRoot } = scratch()
  const yamlPath = path.join(projectRoot, 'trackfw.yaml')
  const before = fs.readFileSync(yamlPath, 'utf8')

  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  assert.equal(fs.readFileSync(yamlPath, 'utf8'), before)
})

test('adr_dirs auto-register: global ADR dir exists but is empty -> trackfw.yaml unchanged byte-for-byte', () => {
  const { projectRoot, homeRoot } = scratch()
  fs.mkdirSync(path.join(homeRoot, '.trackfw', 'adr'), { recursive: true })
  const yamlPath = path.join(projectRoot, 'trackfw.yaml')
  const before = fs.readFileSync(yamlPath, 'utf8')

  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  assert.equal(fs.readFileSync(yamlPath, 'utf8'), before)
})

test('adr_dirs auto-register: entry already present -> idempotent, no further writes', () => {
  const { projectRoot, homeRoot } = scratch()
  writeGlobalAdr(homeRoot)
  const yamlPath = path.join(projectRoot, 'trackfw.yaml')
  fs.writeFileSync(yamlPath, 'hooks: none\nci: none\nadr_dirs:\n  - docs/adr\n  - ~/.trackfw/adr\n')
  const before = fs.readFileSync(yamlPath, 'utf8')

  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  assert.equal(fs.readFileSync(yamlPath, 'utf8'), before)
})

test('adr_dirs auto-register: preserves comments and other keys, only appends the new entry', () => {
  const { projectRoot, homeRoot } = scratch()
  writeGlobalAdr(homeRoot)
  const yamlPath = path.join(projectRoot, 'trackfw.yaml')
  const original = '# trackfw config\nhooks: none\nci: none\n# custom adr dirs\nadr_dirs:\n  - docs/adr\n  - docs/custom-adr\n\nbackend: none\n'
  fs.writeFileSync(yamlPath, original)

  const result = run(['update', '--json', '--dry-run'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  const after = fs.readFileSync(yamlPath, 'utf8')
  assert.equal(
    after,
    '# trackfw config\nhooks: none\nci: none\n# custom adr dirs\nadr_dirs:\n  - docs/adr\n  - docs/custom-adr\n  - ~/.trackfw/adr\n\nbackend: none\n'
  )
})
