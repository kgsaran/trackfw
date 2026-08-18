'use strict'

// ROADMAP-2026-08-06 Wave 3/ML-3A — injectXHooks (project scope) must skip the
// credential-guard entry when the corresponding global-scope wiring (installed
// via `trackfw update harness --targets <tool>-credential-guard`,
// npm/src/commands/update-harness.js) is already present, and must fail-open
// (fall back to the pre-ML-3A behavior of always adding the project-scope
// entry) if the global file is missing, unreadable, or unparseable.
//
// Mirrors internal/generators/credential_guard_dedup_test.go (Go).

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const {
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectCursorHooks,
  injectCopilotHooks,
  injectKiroHooks,
} = require('../src/generators/hooks')

function writeJSON(filePath, data) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, JSON.stringify(data, null, 2) + '\n', 'utf8')
}

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'))
}

function hasClaudeHook(data, event, matcher, command) {
  const arr = (data.hooks && data.hooks[event]) || []
  return arr.some(e => e && e.matcher === matcher && Array.isArray(e.hooks) && e.hooks.some(h => h && h.command === command))
}

let origHome
test.beforeEach(() => {
  origHome = process.env.HOME
})
test.afterEach(() => {
  process.env.HOME = origHome
})

function isolatedHome() {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-home-'))
  process.env.HOME = home
  return home
}

function scriptPathFor(home) {
  return path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
}

test('injectClaudeHooks skips project-scope credential-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = scriptPathFor(home)
  writeJSON(path.join(home, '.claude', 'settings.json'), {
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: scriptPath }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-project-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', 'scripts/trackfw-credential-guard.sh'), false)
  assert.equal(hasClaudeHook(data, 'PostToolUse', 'Bash', 'scripts/trackfw-credential-guard.sh'), false)
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'AskUserQuestion', '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh'), true)
  assert.equal(hasClaudeHook(data, 'PostToolUse', 'AskUserQuestion', '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'), true)
})

test('injectCodexHooks skips project-scope credential-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = scriptPathFor(home)
  writeJSON(path.join(home, '.codex', 'hooks.json'), {
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: scriptPath }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-project-'))
  injectCodexHooks(dir)

  const data = readJSON(path.join(dir, '.codex', 'hooks.json'))
  // ROADMAP-2026-08-11 ML-3A: Codex commands now wrap $(git rev-parse --show-toplevel) in literal
  // quotes (ADR-2026-08-11) -- see CODEX_*_CMD in generators.test.js for the shared literal.
  const codexGuardCmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"'
  const codexSignalCmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-signal.sh"'
  const codexCleanupCmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-cleanup.sh"'
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', codexGuardCmd), false)
  assert.equal(hasClaudeHook(data, 'PostToolUse', 'Bash', codexGuardCmd), false)
  assert.equal(hasClaudeHook(data, 'PermissionRequest', '.*', codexSignalCmd), true)
  assert.equal(hasClaudeHook(data, 'PostToolUse', '.*', codexCleanupCmd), true)
})

test('injectGeminiHooks skips project-scope credential-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = scriptPathFor(home)
  writeJSON(path.join(home, '.gemini', 'settings.json'), {
    hooks: { BeforeTool: [{ matcher: 'run_shell_command', hooks: [{ type: 'command', command: scriptPath }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-project-'))
  injectGeminiHooks(dir)

  const data = readJSON(path.join(dir, '.gemini', 'settings.json'))
  // ROADMAP-2026-08-11 ML-4A: Gemini commands now use $GEMINI_PROJECT_DIR (ADR-2026-08-11) --
  // see GEMINI_*_CMD in generators.test.js for the shared literal.
  const geminiGuardCmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh'
  const geminiSignalCmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-signal.sh'
  const geminiCleanupCmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'
  assert.equal(hasClaudeHook(data, 'BeforeTool', 'run_shell_command', geminiGuardCmd), false)
  assert.equal(hasClaudeHook(data, 'AfterTool', 'run_shell_command', geminiGuardCmd), false)
  assert.equal(hasClaudeHook(data, 'Notification', 'ToolPermission', geminiSignalCmd), true)
  assert.equal(hasClaudeHook(data, 'AfterTool', '*', geminiCleanupCmd), true)
})

test('injectCursorHooks skips project-scope credential-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = scriptPathFor(home)
  writeJSON(path.join(home, '.cursor', 'hooks.json'), {
    version: 1,
    hooks: { beforeShellExecution: [{ command: scriptPath }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-project-'))
  injectCursorHooks(dir)

  const data = readJSON(path.join(dir, '.cursor', 'hooks.json'))
  // ROADMAP-2026-08-17 Wave 2/ML-2B: git-branch-guard now has its own global dedup
  // (globalGitBranchGuardInstalledCursor), but this fixture only wired the GLOBAL
  // credential-guard entry, not git-branch-guard's -- so the git-branch-guard dedup
  // check finds no matching command and fails open, keeping its project-scope entry.
  // See git_branch_guard_dedup.test.js for the case where the git-branch-guard global IS
  // installed.
  assert.equal((data.hooks.beforeShellExecution || []).length, 1)
  assert.equal(data.hooks.beforeShellExecution[0].command, 'scripts/trackfw-git-branch-guard.sh')
  assert.equal((data.hooks.afterShellExecution || []).length, 0)
  assert.equal(data.hooks.preToolUse.length, 1)
  assert.equal(data.hooks.preToolUse[0].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.postToolUse.length, 1)
  assert.equal(data.hooks.postToolUse[0].command, 'scripts/trackfw-attention-cleanup.sh')
})

test('injectCopilotHooks skips project-scope credential-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = scriptPathFor(home)
  writeJSON(path.join(home, '.copilot', 'settings.json'), {
    hooks: { preToolUse: [{ type: 'command', matcher: 'bash', bash: scriptPath, cwd: '.', timeoutSec: 10 }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-project-'))
  injectCopilotHooks(dir)

  const data = readJSON(path.join(dir, '.github', 'hooks', 'trackfw-attention.json'))
  // ROADMAP-2026-08-17 Wave 2/ML-2B: git-branch-guard now has its own global dedup
  // (globalGitBranchGuardInstalledCopilot), but this fixture only wired the GLOBAL
  // credential-guard entry, not git-branch-guard's -- so the git-branch-guard dedup
  // check finds no matching command and fails open, keeping its project-scope entry
  // alongside the always-on attention-signal entry.
  assert.equal(data.hooks.preToolUse.length, 2)
  assert.equal(data.hooks.preToolUse[0].bash, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.preToolUse[1].bash, 'scripts/trackfw-git-branch-guard.sh')
  assert.equal(data.hooks.postToolUse.length, 1)
  assert.equal(data.hooks.postToolUse[0].bash, 'scripts/trackfw-attention-cleanup.sh')
})

test('injectKiroHooks skips project-scope credential-guard when global installed', () => {
  const home = isolatedHome()
  const globalKiroPath = path.join(home, '.kiro', 'hooks', 'trackfw-credential-guard.json')
  writeJSON(globalKiroPath, { version: 'v1', hooks: [] })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-project-'))
  injectKiroHooks(dir)

  const data = readJSON(path.join(dir, '.kiro', 'hooks', 'trackfw-attention.json'))
  assert.equal(data.hooks.length, 2)
  for (const h of data.hooks) {
    assert.notEqual(h.name, 'trackfw-credential-guard-pre')
    assert.notEqual(h.name, 'trackfw-credential-guard-post')
  }
})

// --- Fail-open ---

test('injectClaudeHooks fail-open: no global file -> project entry still added', () => {
  isolatedHome() // empty $HOME, no global files at all

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-failopen-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'), true)
})

test('injectClaudeHooks fail-open: corrupted global file -> project entry still added', () => {
  const home = isolatedHome()
  const settingsPath = path.join(home, '.claude', 'settings.json')
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, '{not valid json', 'utf8')

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-dedup-failopen-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'), true)
})
