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

// countJSONLeafMatches parses raw as JSON and counts how many leaf string
// values, anywhere in the tree, are exactly equal to want. This is
// deliberately a value comparison on the DECODED document, not a substring
// search on the raw serialized text: JSON.stringify escapes every "\" as
// "\\" when it writes a native Windows path into a string field, so a raw
// path.join("\"-separated) needle never matches the escaped haystack — see
// G4 in docs/portabilidade/2026-09-04-retriagem-do-residuo-de-windows-por-mecanismo.md.
// Parsing first makes the comparison agnostic to how the serializer chose to
// escape the value.
function countJSONLeafMatches(raw, want) {
  const doc = JSON.parse(raw)
  const count = (v) => {
    if (typeof v === 'string') return v === want ? 1 : 0
    if (Array.isArray(v)) return v.reduce((sum, e) => sum + count(e), 0)
    if (v && typeof v === 'object') return Object.values(v).reduce((sum, e) => sum + count(e), 0)
    return 0
  }
  return count(doc)
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

// ---------------------------------------------------------------------------
// `codex-credential-guard` — global-scope credential-guard hook wiring for
// Codex CLI, ROADMAP-2026-08-06 Wave 2 ML-2B. Mirrors the claude-credential-
// guard tests above and internal/generators/update_test.go's Codex tests.
// ---------------------------------------------------------------------------

test('codex-credential-guard is missing without --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'codex-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'missing')
  assert.equal(fs.existsSync(path.join(homeRoot, '.codex', 'hooks.json')), false)
})

test('codex-credential-guard installs the absolute global script path with --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'codex-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(doc.targets[0].path, '~/.codex/hooks.json')

  const hooksPath = path.join(homeRoot, '.codex', 'hooks.json')
  const hooksDoc = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  assert.ok(path.isAbsolute(wantScript))

  for (const event of ['PreToolUse', 'PostToolUse']) {
    const bashEntries = (hooksDoc.hooks[event] || []).filter((e) => e.matcher === 'Bash')
    assert.equal(bashEntries.length, 1)
    const commands = bashEntries[0].hooks.map((h) => h.command)
    assert.ok(commands.includes(wantScript))
  }
})

test('codex-credential-guard is idempotent', () => {
  const homeRoot = scratchHome()
  run(['update', 'harness', '--json', '--install-missing', '--targets', 'codex-credential-guard'], homeRoot)
  const hooksPath = path.join(homeRoot, '.codex', 'hooks.json')
  const firstRun = fs.readFileSync(hooksPath, 'utf8')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'codex-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'skipped')
  const secondRun = fs.readFileSync(hooksPath, 'utf8')
  assert.equal(firstRun, secondRun)

  const hooksDoc = JSON.parse(secondRun)
  const bashEntries = hooksDoc.hooks.PreToolUse.filter((e) => e.matcher === 'Bash')
  assert.equal(bashEntries.length, 1)
})

test('codex-credential-guard --dry-run does not write', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(
      ['update', 'harness', '--json', '--install-missing', '--dry-run', '--targets', 'codex-credential-guard'],
      homeRoot
    ).stdout
  )
  assert.equal(doc.dry_run, true)
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(fs.existsSync(path.join(homeRoot, '.codex', 'hooks.json')), false)
})

test('codex-credential-guard preserves pre-existing content in ~/.codex/hooks.json', () => {
  const homeRoot = scratchHome()
  const hooksPath = path.join(homeRoot, '.codex', 'hooks.json')
  fs.mkdirSync(path.dirname(hooksPath), { recursive: true })
  fs.writeFileSync(
    hooksPath,
    JSON.stringify(
      {
        hooks: {
          PermissionRequest: [
            {
              matcher: '.*',
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
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'codex-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')

  const hooksDoc = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(hooksDoc.userSetting, 'keep-me')
  const permEntries = hooksDoc.hooks.PermissionRequest.filter((e) => e.matcher === '.*')
  assert.equal(permEntries.length, 1)
  for (const event of ['PreToolUse', 'PostToolUse']) {
    const bashEntries = hooksDoc.hooks[event].filter((e) => e.matcher === 'Bash')
    assert.equal(bashEntries.length, 1)
  }
})

// ---------------------------------------------------------------------------
// `gemini-credential-guard` — global-scope credential-guard hook wiring for
// Gemini CLI, ROADMAP-2026-08-06 Wave 2 ML-2C. Mirrors the codex-credential-
// guard tests above and internal/generators/update_test.go's Gemini tests —
// only the event names differ (BeforeTool/AfterTool, matcher
// "run_shell_command" instead of PreToolUse/PostToolUse, matcher "Bash").
// ---------------------------------------------------------------------------

test('gemini-credential-guard is missing without --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'gemini-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'missing')
  assert.equal(fs.existsSync(path.join(homeRoot, '.gemini', 'settings.json')), false)
})

test('gemini-credential-guard installs the absolute global script path with --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'gemini-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(doc.targets[0].path, '~/.gemini/settings.json')

  const settingsPath = path.join(homeRoot, '.gemini', 'settings.json')
  const settingsDoc = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  assert.ok(path.isAbsolute(wantScript))

  for (const event of ['BeforeTool', 'AfterTool']) {
    const shellEntries = (settingsDoc.hooks[event] || []).filter((e) => e.matcher === 'run_shell_command')
    assert.equal(shellEntries.length, 1)
    const commands = shellEntries[0].hooks.map((h) => h.command)
    assert.ok(commands.includes(wantScript))
  }
})

test('gemini-credential-guard is idempotent', () => {
  const homeRoot = scratchHome()
  run(['update', 'harness', '--json', '--install-missing', '--targets', 'gemini-credential-guard'], homeRoot)
  const settingsPath = path.join(homeRoot, '.gemini', 'settings.json')
  const firstRun = fs.readFileSync(settingsPath, 'utf8')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'gemini-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'skipped')
  const secondRun = fs.readFileSync(settingsPath, 'utf8')
  assert.equal(firstRun, secondRun)

  const settingsDoc = JSON.parse(secondRun)
  const shellEntries = settingsDoc.hooks.BeforeTool.filter((e) => e.matcher === 'run_shell_command')
  assert.equal(shellEntries.length, 1)
})

test('gemini-credential-guard --dry-run does not write', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(
      ['update', 'harness', '--json', '--install-missing', '--dry-run', '--targets', 'gemini-credential-guard'],
      homeRoot
    ).stdout
  )
  assert.equal(doc.dry_run, true)
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(fs.existsSync(path.join(homeRoot, '.gemini', 'settings.json')), false)
})

test('gemini-credential-guard preserves pre-existing content in ~/.gemini/settings.json', () => {
  const homeRoot = scratchHome()
  const settingsPath = path.join(homeRoot, '.gemini', 'settings.json')
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(
    settingsPath,
    JSON.stringify(
      {
        hooks: {
          Notification: [
            {
              matcher: 'ToolPermission',
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
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'gemini-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')

  const settingsDoc = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(settingsDoc.userSetting, 'keep-me')
  const notifEntries = settingsDoc.hooks.Notification.filter((e) => e.matcher === 'ToolPermission')
  assert.equal(notifEntries.length, 1)
  for (const event of ['BeforeTool', 'AfterTool']) {
    const shellEntries = settingsDoc.hooks[event].filter((e) => e.matcher === 'run_shell_command')
    assert.equal(shellEntries.length, 1)
  }
})

// ---------------------------------------------------------------------------
// `cursor-credential-guard` — global-scope credential-guard hook wiring for
// Cursor, ROADMAP-2026-08-06 Wave 2 ML-2D. Mirrors the gemini-credential-
// guard tests above, but reads hooks[event] as a flat array of
// {"command":"..."} entries — no "matcher" — since Cursor's hooks.json
// schema differs structurally from Claude/Codex/Gemini's (see
// generators/hooks.js:injectCursorHooks).
// ---------------------------------------------------------------------------

test('cursor-credential-guard is missing without --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'cursor-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'missing')
  assert.equal(fs.existsSync(path.join(homeRoot, '.cursor', 'hooks.json')), false)
})

test('cursor-credential-guard installs the absolute global script path with --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'cursor-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(doc.targets[0].path, '~/.cursor/hooks.json')

  const hooksPath = path.join(homeRoot, '.cursor', 'hooks.json')
  const hooksDoc = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(hooksDoc.version, 1)
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  assert.ok(path.isAbsolute(wantScript))

  for (const event of ['beforeShellExecution', 'afterShellExecution']) {
    const commands = (hooksDoc.hooks[event] || []).map((e) => e.command)
    assert.ok(commands.includes(wantScript))
  }
})

test('cursor-credential-guard is idempotent', () => {
  const homeRoot = scratchHome()
  run(['update', 'harness', '--json', '--install-missing', '--targets', 'cursor-credential-guard'], homeRoot)
  const hooksPath = path.join(homeRoot, '.cursor', 'hooks.json')
  const firstRun = fs.readFileSync(hooksPath, 'utf8')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'cursor-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'skipped')
  const secondRun = fs.readFileSync(hooksPath, 'utf8')
  assert.equal(firstRun, secondRun)

  const hooksDoc = JSON.parse(secondRun)
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  const shellEntries = hooksDoc.hooks.beforeShellExecution.filter((e) => e.command === wantScript)
  assert.equal(shellEntries.length, 1)
})

test('cursor-credential-guard --dry-run does not write', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(
      ['update', 'harness', '--json', '--install-missing', '--dry-run', '--targets', 'cursor-credential-guard'],
      homeRoot
    ).stdout
  )
  assert.equal(doc.dry_run, true)
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(fs.existsSync(path.join(homeRoot, '.cursor', 'hooks.json')), false)
})

test('cursor-credential-guard preserves pre-existing content in ~/.cursor/hooks.json', () => {
  const homeRoot = scratchHome()
  const hooksPath = path.join(homeRoot, '.cursor', 'hooks.json')
  fs.mkdirSync(path.dirname(hooksPath), { recursive: true })
  fs.writeFileSync(
    hooksPath,
    JSON.stringify(
      {
        version: 1,
        hooks: {
          preToolUse: [{ command: 'scripts/trackfw-attention-signal.sh' }],
        },
        userSetting: 'keep-me',
      },
      null,
      2
    )
  )

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'cursor-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')

  const hooksDoc = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(hooksDoc.userSetting, 'keep-me')
  assert.equal(hooksDoc.hooks.preToolUse.length, 1)
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  for (const event of ['beforeShellExecution', 'afterShellExecution']) {
    const commands = hooksDoc.hooks[event].map((e) => e.command)
    assert.ok(commands.includes(wantScript))
  }
})

// ---------------------------------------------------------------------------
// `copilot-credential-guard` — global-scope credential-guard hook wiring for
// GitHub Copilot, ROADMAP-2026-08-06 Wave 2 ML-2E. Mirrors the cursor-
// credential-guard tests above, but reads hooks[event] as an array of
// {"type":"command","matcher":"bash","bash":"...",...} entries (matched on
// "bash", not "command") — Copilot's ~/.copilot/settings.json entry shape
// (confirmed by generators/hooks.js:mergeCopilotHookArray) matches the
// project-scope entries injectCopilotHooks already emits.
// ---------------------------------------------------------------------------

test('copilot-credential-guard is missing without --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'copilot-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'missing')
  assert.equal(fs.existsSync(path.join(homeRoot, '.copilot', 'settings.json')), false)
})

test('copilot-credential-guard installs the absolute global script path with --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'copilot-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(doc.targets[0].path, '~/.copilot/settings.json')

  const settingsPath = path.join(homeRoot, '.copilot', 'settings.json')
  const settingsDoc = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(settingsDoc.version, undefined, '~/.copilot/settings.json is a general config file — no unconfirmed top-level "version" key')
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  assert.ok(path.isAbsolute(wantScript))

  for (const event of ['preToolUse', 'postToolUse']) {
    const entries = settingsDoc.hooks[event] || []
    const entry = entries.find((e) => e.bash === wantScript)
    assert.ok(entry, `${event} missing entry pointing at ${wantScript}`)
    assert.equal(entry.type, 'command')
    assert.equal(entry.matcher, 'bash')
  }
})

test('copilot-credential-guard is idempotent', () => {
  const homeRoot = scratchHome()
  run(['update', 'harness', '--json', '--install-missing', '--targets', 'copilot-credential-guard'], homeRoot)
  const settingsPath = path.join(homeRoot, '.copilot', 'settings.json')
  const firstRun = fs.readFileSync(settingsPath, 'utf8')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'copilot-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'skipped')
  const secondRun = fs.readFileSync(settingsPath, 'utf8')
  assert.equal(firstRun, secondRun)

  const settingsDoc = JSON.parse(secondRun)
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  const shellEntries = settingsDoc.hooks.preToolUse.filter((e) => e.bash === wantScript)
  assert.equal(shellEntries.length, 1)
})

test('copilot-credential-guard --dry-run does not write', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(
      ['update', 'harness', '--json', '--install-missing', '--dry-run', '--targets', 'copilot-credential-guard'],
      homeRoot
    ).stdout
  )
  assert.equal(doc.dry_run, true)
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(fs.existsSync(path.join(homeRoot, '.copilot', 'settings.json')), false)
})

test('copilot-credential-guard preserves pre-existing content in ~/.copilot/settings.json', () => {
  const homeRoot = scratchHome()
  const settingsPath = path.join(homeRoot, '.copilot', 'settings.json')
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(
    settingsPath,
    JSON.stringify(
      {
        model: 'gpt-5',
        hooks: {
          preToolUse: [{ type: 'command', matcher: 'curl', bash: 'echo hi' }],
        },
        userSetting: 'keep-me',
      },
      null,
      2
    )
  )

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'copilot-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')

  const settingsDoc = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(settingsDoc.userSetting, 'keep-me')
  assert.equal(settingsDoc.model, 'gpt-5')
  assert.equal(settingsDoc.hooks.preToolUse.length, 2)
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  const guardEntries = settingsDoc.hooks.preToolUse.filter((e) => e.bash === wantScript)
  assert.equal(guardEntries.length, 1)
})

// ---------------------------------------------------------------------------
// `kiro-credential-guard` — global-scope credential-guard hook wiring for
// Kiro, ROADMAP-2026-08-06 Wave 2 ML-2F. Unlike claude/codex/gemini/cursor/
// copilot-credential-guard above, ~/.kiro/hooks/trackfw-credential-guard.json
// is a DEDICATED file (only trackfw ever writes it) — mirrors
// claude-skill's wholesale-overwrite contract, not the merge-and-preserve
// contract of the settings-file targets.
// ---------------------------------------------------------------------------

test('kiro-credential-guard is missing without --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--targets', 'kiro-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'missing')
  assert.equal(fs.existsSync(path.join(homeRoot, '.kiro', 'hooks', 'trackfw-credential-guard.json')), false)
})

test('kiro-credential-guard installs the absolute global script path with --install-missing', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'kiro-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(doc.targets[0].path, '~/.kiro/hooks/trackfw-credential-guard.json')

  const hookPath = path.join(homeRoot, '.kiro', 'hooks', 'trackfw-credential-guard.json')
  const hooksDoc = JSON.parse(fs.readFileSync(hookPath, 'utf8'))
  assert.equal(hooksDoc.version, 'v1')
  const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  assert.ok(path.isAbsolute(wantScript))
  assert.equal(hooksDoc.hooks.length, 2)
  const triggers = hooksDoc.hooks.map((h) => h.trigger).sort()
  assert.deepEqual(triggers, ['PostToolUse', 'PreToolUse'])
  for (const entry of hooksDoc.hooks) {
    assert.equal(entry.matcher, 'shell')
    assert.equal(entry.action.type, 'command')
    assert.equal(entry.action.command, wantScript)
  }
})

test('kiro-credential-guard is idempotent', () => {
  const homeRoot = scratchHome()
  run(['update', 'harness', '--json', '--install-missing', '--targets', 'kiro-credential-guard'], homeRoot)
  const hookPath = path.join(homeRoot, '.kiro', 'hooks', 'trackfw-credential-guard.json')
  const firstRun = fs.readFileSync(hookPath, 'utf8')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'kiro-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'skipped')
  const secondRun = fs.readFileSync(hookPath, 'utf8')
  assert.equal(firstRun, secondRun)
})

test('kiro-credential-guard --dry-run does not write', () => {
  const homeRoot = scratchHome()
  const doc = JSON.parse(
    run(
      ['update', 'harness', '--json', '--install-missing', '--dry-run', '--targets', 'kiro-credential-guard'],
      homeRoot
    ).stdout
  )
  assert.equal(doc.dry_run, true)
  assert.equal(doc.targets[0].state, 'updated')
  assert.equal(fs.existsSync(path.join(homeRoot, '.kiro', 'hooks', 'trackfw-credential-guard.json')), false)
})

test('kiro-credential-guard rewrites stale content (dedicated file, never merged)', () => {
  const homeRoot = scratchHome()
  const hookPath = path.join(homeRoot, '.kiro', 'hooks', 'trackfw-credential-guard.json')
  fs.mkdirSync(path.dirname(hookPath), { recursive: true })
  fs.writeFileSync(hookPath, JSON.stringify({ version: 'v1', hooks: [{ name: 'stale' }] }))

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', 'kiro-credential-guard'], homeRoot).stdout
  )
  assert.equal(doc.targets[0].state, 'updated')
  const rewritten = fs.readFileSync(hookPath, 'utf8')
  assert.ok(!rewritten.includes('"stale"'))
})

// ---------------------------------------------------------------------------
// `<tool>-git-branch-guard` — global-scope git-branch-guard hook wiring,
// ROADMAP-2026-08-17 Wave 2/ML-2A. Table-driven mirror of the six
// `<tool>-credential-guard` sections above: same 4-state contract, same
// displayPath per tool, only the referenced script differs
// (trackfw-git-branch-guard.sh instead of trackfw-credential-guard.sh) and
// Kiro gets its OWN dedicated file (trackfw-git-branch-guard.json, never
// trackfw-credential-guard.json — sharing would break idempotency, see
// gitBranchGuardTargetKiro's doc comment in update-harness.js).
// ---------------------------------------------------------------------------
const GIT_BRANCH_GUARD_CASES = [
  { tool: 'claude', relPath: ['.claude', 'settings.json'], displayPath: '~/.claude/settings.json' },
  { tool: 'codex', relPath: ['.codex', 'hooks.json'], displayPath: '~/.codex/hooks.json' },
  { tool: 'gemini', relPath: ['.gemini', 'settings.json'], displayPath: '~/.gemini/settings.json' },
  { tool: 'cursor', relPath: ['.cursor', 'hooks.json'], displayPath: '~/.cursor/hooks.json' },
  { tool: 'copilot', relPath: ['.copilot', 'settings.json'], displayPath: '~/.copilot/settings.json' },
  {
    tool: 'kiro',
    relPath: ['.kiro', 'hooks', 'trackfw-git-branch-guard.json'],
    displayPath: '~/.kiro/hooks/trackfw-git-branch-guard.json',
  },
]

for (const { tool, relPath, displayPath } of GIT_BRANCH_GUARD_CASES) {
  const targetId = `${tool}-git-branch-guard`

  test(`${targetId} is missing without --install-missing`, () => {
    const homeRoot = scratchHome()
    const doc = JSON.parse(run(['update', 'harness', '--json', '--targets', targetId], homeRoot).stdout)
    assert.equal(doc.targets[0].state, 'missing')
    assert.equal(fs.existsSync(path.join(homeRoot, ...relPath)), false)
  })

  test(`${targetId} installs the absolute global git-branch-guard script path with --install-missing`, () => {
    const homeRoot = scratchHome()
    const doc = JSON.parse(
      run(['update', 'harness', '--json', '--install-missing', '--targets', targetId], homeRoot).stdout
    )
    assert.equal(doc.targets[0].state, 'updated')
    assert.equal(doc.targets[0].path, displayPath)

    const written = fs.readFileSync(path.join(homeRoot, ...relPath), 'utf8')
    const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    assert.ok(
      countJSONLeafMatches(written, wantScript) > 0,
      `${relPath.join('/')} does not reference ${wantScript} (decoded JSON has no leaf equal to it):\n${written}`
    )

    const credScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
    if (tool !== 'kiro') {
      assert.equal(
        countJSONLeafMatches(written, credScript),
        0,
        `${relPath.join('/')} unexpectedly references trackfw-credential-guard.sh`
      )
    }
  })

  test(`${targetId} is idempotent`, () => {
    const homeRoot = scratchHome()
    run(['update', 'harness', '--json', '--install-missing', '--targets', targetId], homeRoot)
    const first = fs.readFileSync(path.join(homeRoot, ...relPath), 'utf8')

    const doc = JSON.parse(
      run(['update', 'harness', '--json', '--install-missing', '--targets', targetId], homeRoot).stdout
    )
    assert.equal(doc.targets[0].state, 'skipped')
    const second = fs.readFileSync(path.join(homeRoot, ...relPath), 'utf8')
    assert.equal(first, second)
  })

  test(`${targetId} --dry-run does not write`, () => {
    const homeRoot = scratchHome()
    const doc = JSON.parse(
      run(
        ['update', 'harness', '--json', '--install-missing', '--dry-run', '--targets', targetId],
        homeRoot
      ).stdout
    )
    assert.equal(doc.dry_run, true)
    assert.equal(doc.targets[0].state, 'updated')
    assert.equal(fs.existsSync(path.join(homeRoot, ...relPath)), false)
  })
}

test('claude-credential-guard and claude-git-branch-guard coexist in the same file, each with exactly 2 references (Pre+Post), and both stay idempotent across a second run', () => {
  const homeRoot = scratchHome()
  const targets = 'claude-credential-guard,claude-git-branch-guard'
  run(['update', 'harness', '--json', '--install-missing', '--targets', targets], homeRoot)

  const settingsPath = path.join(homeRoot, '.claude', 'settings.json')
  const first = fs.readFileSync(settingsPath, 'utf8')
  const credScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  const branchScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
  assert.equal(countJSONLeafMatches(first, credScript), 2, 'expected exactly 2 references to trackfw-credential-guard.sh (Pre+Post)')
  assert.equal(countJSONLeafMatches(first, branchScript), 2, 'expected exactly 2 references to trackfw-git-branch-guard.sh (Pre+Post)')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', targets], homeRoot).stdout
  )
  for (const target of doc.targets) {
    assert.equal(target.state, 'skipped', `${target.id} should be skipped on the second (idempotent) run`)
  }
  const second = fs.readFileSync(settingsPath, 'utf8')
  assert.equal(first, second, 'settings.json content must not change on an idempotent re-run')
})

test('kiro-credential-guard and kiro-git-branch-guard write two SEPARATE files, neither one flapping across repeated runs', () => {
  const homeRoot = scratchHome()
  const targets = 'kiro-credential-guard,kiro-git-branch-guard'
  run(['update', 'harness', '--json', '--install-missing', '--targets', targets], homeRoot)

  const credPath = path.join(homeRoot, '.kiro', 'hooks', 'trackfw-credential-guard.json')
  const branchPath = path.join(homeRoot, '.kiro', 'hooks', 'trackfw-git-branch-guard.json')
  assert.ok(fs.existsSync(credPath))
  assert.ok(fs.existsSync(branchPath))
  const credBefore = fs.readFileSync(credPath, 'utf8')
  const branchBefore = fs.readFileSync(branchPath, 'utf8')

  const doc = JSON.parse(
    run(['update', 'harness', '--json', '--install-missing', '--targets', targets], homeRoot).stdout
  )
  for (const target of doc.targets) {
    assert.equal(target.state, 'skipped', `${target.id} should be skipped on the second run — files must not flap`)
  }
  assert.equal(fs.readFileSync(credPath, 'utf8'), credBefore)
  assert.equal(fs.readFileSync(branchPath, 'utf8'), branchBefore)
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
