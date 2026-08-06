'use strict'

// ML-6C (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador)
// — see docs/cli-parity.md, "`trackfw update` vs `trackfw update harness`".
//
// EVERY test in this file redirects HOME to a scratch directory (via
// spawnSync env, never process.env.HOME mutation in-process) and NEVER
// invokes `trackfw update harness` against the real HOME.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const bin = path.resolve(__dirname, '../bin/trackfw')
const { HARNESS_TARGET_IDS, claudeSkillContent } = require('../src/commands/update-harness')

function scratchHome() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-harness-test-'))
}

function run(args, homeRoot, cwd) {
  return spawnSync(process.execPath, [bin, ...args], {
    cwd: cwd || homeRoot,
    env: { ...process.env, HOME: homeRoot },
    encoding: 'utf8',
  })
}

test('update harness never requires trackfw.yaml or a project cwd', () => {
  const homeRoot = scratchHome()
  const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-harness-nowhere-'))
  const result = run(['update', 'harness', '--json'], homeRoot, cwd)
  assert.equal(result.status, 0, result.stderr)
  assert.doesNotThrow(() => JSON.parse(result.stdout))
})

test('update harness runs fine from inside a project directory that has its own trackfw.yaml', () => {
  const homeRoot = scratchHome()
  const projectRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-harness-inside-project-'))
  fs.writeFileSync(path.join(projectRoot, 'trackfw.yaml'), 'hooks: none\nci: none\n')
  const result = run(['update', 'harness', '--json'], homeRoot, projectRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.equal(doc.scope, 'harness')
})

test('an empty harness reports every declared target missing and exits 0', () => {
  const homeRoot = scratchHome()
  const result = run(['update', 'harness', '--json'], homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.equal(doc.scope, 'harness')
  assert.equal(doc.targets.length, HARNESS_TARGET_IDS.length)
  assert.deepEqual(doc.targets.map(t => t.id), HARNESS_TARGET_IDS)
  for (const t of doc.targets) assert.equal(t.state, 'missing', `${t.id} expected missing`)
  assert.equal(doc.summary.missing, HARNESS_TARGET_IDS.length)
  assert.equal(doc.summary.updated + doc.summary.skipped + doc.summary.failed, 0)
})

test('JSON document has the exact frozen key order: scope, dry_run, targets, summary', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(run(['update', 'harness', '--json'], homeRoot).stdout)
  assert.deepEqual(Object.keys(doc), ['scope', 'dry_run', 'targets', 'summary'])
  assert.deepEqual(Object.keys(doc.summary), ['updated', 'skipped', 'missing', 'failed'])
  for (const t of doc.targets) assert.deepEqual(Object.keys(t), ['id', 'state', 'path'])
})

test('the four states appear in one document: missing, updated (via --install-missing), skipped (re-run), and failed', () => {
  const homeRoot = scratchHome()

  // updated: install-missing on a fresh target
  const installed = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-skill'], homeRoot).stdout
  )
  assert.equal(installed.targets[0].state, 'updated')
  assert.equal(fs.readFileSync(path.join(homeRoot, '.claude', 'skills', 'trackfw', 'SKILL.md'), 'utf8'), claudeSkillContent())

  // skipped: re-run, content already current
  const skipped = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'claude-skill'], homeRoot).stdout
  )
  assert.equal(skipped.targets[0].state, 'skipped')

  // missing: a target that was never installed
  const missing = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'codex-agents'], homeRoot).stdout
  )
  assert.equal(missing.targets[0].state, 'missing')

  // failed: block the write path (a file sits where a directory must be created)
  const homeRoot2 = scratchHome()
  fs.mkdirSync(path.join(homeRoot2, '.claude', 'skills'), { recursive: true })
  fs.writeFileSync(path.join(homeRoot2, '.claude', 'skills', 'trackfw'), 'blocking file, not a directory\n')
  const failed = run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-skill'], homeRoot2)
  assert.notEqual(failed.status, 0)
  const failedDoc = JSON.parse(failed.stdout)
  assert.equal(failedDoc.targets[0].state, 'failed')
  assert.equal(failedDoc.summary.failed, 1)
  assert.ok(failedDoc.targets[0].message, 'failed target must carry a message')
  assert.deepEqual(Object.keys(failedDoc.targets[0]), ['id', 'state', 'path', 'message'])
})

test('--dry-run never writes to HOME even with --install-missing', () => {
  const homeRoot = scratchHome()
  const before = fs.existsSync(path.join(homeRoot, '.claude'))
  const result = run(['update', 'harness', '--json', '--dry-run', '--install-missing'], homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.equal(doc.dry_run, true)
  assert.ok(doc.targets.some(t => t.state === 'updated'), 'dry-run should still predict updates')
  assert.equal(before, false)
  assert.equal(fs.existsSync(path.join(homeRoot, '.claude')), false, '--dry-run must not create ~/.claude')
})

test('--targets restricts both computation and side effects to the requested subset', () => {
  const homeRoot = scratchHome()
  const result = run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-skill'], homeRoot)
  assert.equal(result.status, 0, result.stderr)
  const doc = JSON.parse(result.stdout)
  assert.deepEqual(doc.targets.map(t => t.id), ['claude-skill'])
  // claude-agents is a distinct, always-installable target — it must not
  // have been written just because it exists in the full universe.
  assert.equal(fs.existsSync(path.join(homeRoot, '.claude', 'agents')), false)
})

test('unknown --targets id is a usage error with non-zero exit, and touches nothing', () => {
  const homeRoot = scratchHome()
  const result = run(['update', 'harness', '--targets', 'not-a-real-target'], homeRoot)
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /Unknown update target/)
  assert.equal(fs.existsSync(path.join(homeRoot, '.claude')), false)
})

test('paths are tilde-abbreviated relative to HOME, never the absolute filesystem path', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(run(['update', 'harness', '--json'], homeRoot).stdout)
  const claudeSkill = doc.targets.find(t => t.id === 'claude-skill')
  assert.equal(claudeSkill.path, '~/.claude/skills/trackfw/SKILL.md')
  for (const t of doc.targets) assert.ok(!t.path.includes(homeRoot), `${t.id} path leaked the absolute HOME: ${t.path}`)
})

// ---------------------------------------------------------------------------
// `claude-credential-guard` — global-scope credential-guard hook wiring for
// Claude Code, ROADMAP-2026-08-06 Wave 2 ML-2A. Mirrors the Go tests in
// internal/generators/update_test.go/internal/commands/update_harness_test.go.
// ---------------------------------------------------------------------------

test('claude-credential-guard is missing without --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'claude-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'missing')
  assert.equal(fs.existsSync(path.join(homeRoot, '.claude', 'settings.json')), false)
})

test('claude-credential-guard installs the absolute global script path with --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(doc.targets[0].path, '~/.claude/settings.json')

  const settingsPath = path.join(homeRoot, '.claude', 'settings.json')
  const settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  assert.ok(path.isAbsolute(wantScript))

  for (const event of ['PreToolUse', 'PostToolUse']) {
    const bashEntries = (settings.hooks[event] || []).filter((e) => e.matcher === 'Bash')
    assert.equal(bashEntries.length, 1)
    const commands = bashEntries[0].hooks.map((h) => h.command)
    assert.ok(commands.includes(wantScript))
  }
})

test('claude-credential-guard is idempotent', () => {
  const homeRoot = scratchHome()
  run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-credential-guard'], homeRoot)
  const settingsPath = path.join(homeRoot, '.claude', 'settings.json')
  const firstRun = fs.readFileSync(settingsPath, 'utf8')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'skipped')
  const secondRun = fs.readFileSync(settingsPath, 'utf8')
  assert.equal(firstRun, secondRun)

  const settings = JSON.parse(secondRun)
  const bashEntries = settings.hooks.PreToolUse.filter((e) => e.matcher === 'Bash')
  assert.equal(bashEntries.length, 1)
})

test('claude-credential-guard --dry-run does not write', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(
      ['update', 'harness', '--json', '--install-missing', '--dry-run', '--targets', 'claude-credential-guard'],
      homeRoot
    ).stdout
  )
  assert.equal(doc.dry_run, true)
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(fs.existsSync(path.join(homeRoot, '.claude', 'settings.json')), false)
})

test('claude-credential-guard preserves pre-existing content in ~/.claude/settings.json', () => {
  const homeRoot = scratchHome()
  const settingsPath = path.join(homeRoot, '.claude', 'settings.json')
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(
    settingsPath,
    JSON.stringify(
      {
        hooks: {
          PreToolUse: [
            {
              matcher: 'AskUserQuestion',
              hooks: [{ type: 'command', command: 'scripts/trackfw-attention-signal.sh' }],
            },
          ],
        },
        userSetting: 'keep-me',
      },
      null,
      2
    )
  )

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')

  const settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(settings.userSetting, 'keep-me')
  const askEntries = settings.hooks.PreToolUse.filter((e) => e.matcher === 'AskUserQuestion')
  assert.equal(askEntries.length, 1)
  assert.equal(askEntries[0].hooks[0].command, 'scripts/trackfw-attention-signal.sh')
  for (const event of ['PreToolUse', 'PostToolUse']) {
    const bashEntries = settings.hooks[event].filter((e) => e.matcher === 'Bash')
    assert.equal(bashEntries.length, 1)
  }
})

test('a project-scoped catalog install under HOME is not touched by trackfw update (project scope stays project scope)', () => {
  const homeRoot = scratchHome()
  const projectRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-harness-project-'))
  fs.writeFileSync(path.join(projectRoot, 'trackfw.yaml'), 'hooks: none\nci: none\n')
  run(['update', 'harness', '--json', '--install-missing', '--targets', 'claude-agents'], homeRoot)
  const before = fs.readFileSync(path.join(homeRoot, '.claude', 'agents', 'trackfw-architect.md'), 'utf8')
  run(['update', '--install-missing'], projectRoot, homeRoot)
  const after = fs.readFileSync(path.join(homeRoot, '.claude', 'agents', 'trackfw-architect.md'), 'utf8')
  assert.equal(before, after, 'project-scoped `update` must never touch the global harness')
})
