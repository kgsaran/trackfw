'use strict'

// ROADMAP-2026-08-17 Wave 2/ML-2B -- injectXHooks (project scope) must skip the
// git-branch-guard entry when the corresponding global-scope wiring (installed
// via `trackfw update harness --targets <tool>-git-branch-guard`, ML-2A) is
// already present, so the guard doesn't fire (and print its block message)
// twice per Bash call -- and must fail-open (fall back to always adding the
// project-scope entry) if the global file is missing, unreadable, or
// unparseable.
//
// Mirrors internal/generators/git_branch_guard_dedup_test.go (Go).

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const {
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectCursorHooks,
  injectCopilotHooks,
  generateGitBranchGuardScript,
  generateGlobalGitBranchGuardScript,
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

function claudeHookCommands(data, event, matcher) {
  const arr = (data.hooks && data.hooks[event]) || []
  const out = []
  for (const e of arr) {
    if (!e || e.matcher !== matcher || !Array.isArray(e.hooks)) continue
    for (const h of e.hooks) {
      if (h && typeof h.command === 'string') out.push(h.command)
    }
  }
  return out
}

let origHome
test.beforeEach(() => {
  origHome = process.env.HOME
})
test.afterEach(() => {
  process.env.HOME = origHome
})

function isolatedHome() {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-home-'))
  process.env.HOME = home
  return home
}

function gbgScriptPathFor(home) {
  return path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
}

test('injectClaudeHooks skips project-scope git-branch-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.claude', 'settings.json'), {
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: scriptPath }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'), false)
  // credential-guard's own global was NOT installed by this fixture -- its entry must
  // still be added, proving the two guards dedup independently.
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'), true)
})

test('injectCodexHooks skips project-scope git-branch-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.codex', 'hooks.json'), {
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: scriptPath }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectCodexHooks(dir)

  const data = readJSON(path.join(dir, '.codex', 'hooks.json'))
  const codexGitGuardCmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"'
  const codexGuardCmd = '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"'
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', codexGitGuardCmd), false)
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', codexGuardCmd), true)
})

test('injectGeminiHooks skips project-scope git-branch-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.gemini', 'settings.json'), {
    hooks: { BeforeTool: [{ matcher: 'run_shell_command', hooks: [{ type: 'command', command: scriptPath }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectGeminiHooks(dir)

  const data = readJSON(path.join(dir, '.gemini', 'settings.json'))
  const geminiGitGuardCmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'
  const geminiGuardCmd = '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh'
  assert.equal(hasClaudeHook(data, 'BeforeTool', 'run_shell_command', geminiGitGuardCmd), false)
  assert.equal(hasClaudeHook(data, 'BeforeTool', 'run_shell_command', geminiGuardCmd), true)
})

test('injectCursorHooks skips project-scope git-branch-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.cursor', 'hooks.json'), {
    version: 1,
    hooks: { beforeShellExecution: [{ command: scriptPath }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectCursorHooks(dir)

  const data = readJSON(path.join(dir, '.cursor', 'hooks.json'))
  const before = (data.hooks.beforeShellExecution || [])
  assert.equal(before.length, 1)
  assert.equal(before[0].command, 'scripts/trackfw-credential-guard.sh')
})

test('injectCursorHooks: both globals installed -> beforeShellExecution key absent, not empty', () => {
  const home = isolatedHome()
  const credPath = path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  const gbgPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.cursor', 'hooks.json'), {
    version: 1,
    hooks: { beforeShellExecution: [{ command: credPath }, { command: gbgPath }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectCursorHooks(dir)

  const data = readJSON(path.join(dir, '.cursor', 'hooks.json'))
  // Both dedups skip: the key must be ABSENT, not a present-but-empty array --
  // matches Go's InjectCursorHooks (see the equivalent Go test), which
  // check-agent-hooks-parity.sh's structural comparator treats as significant.
  assert.equal(Object.prototype.hasOwnProperty.call(data.hooks, 'beforeShellExecution'), false)
})

test('injectCopilotHooks skips project-scope git-branch-guard when global installed', () => {
  const home = isolatedHome()
  const scriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.copilot', 'settings.json'), {
    hooks: { preToolUse: [{ type: 'command', matcher: 'bash', bash: scriptPath, cwd: '.', timeoutSec: 10 }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectCopilotHooks(dir)

  const data = readJSON(path.join(dir, '.github', 'hooks', 'trackfw-attention.json'))
  const pre = data.hooks.preToolUse
  assert.equal(pre.some(e => e.bash === 'scripts/trackfw-git-branch-guard.sh'), false)
  assert.equal(pre.some(e => e.bash === 'scripts/trackfw-credential-guard.sh'), true)
})

// ROADMAP-2026-08-17 ML-2C: reproduces the root cause directly at the dedup
// level (not just the comparator unit test in guard_path_normalize.test.js)
// -- the "command" value stored in ~/.claude/settings.json is built with raw
// string concatenation (as a hand-edited config, or a $HOME captured with a
// trailing slash before normalization, would produce) instead of
// path.join(), so it textually differs from what globalGitBranchGuardScriptPath()
// computes today even though it names the SAME file.
test('injectClaudeHooks skips project-scope git-branch-guard despite // formatting in stored global command', () => {
  const home = isolatedHome()
  const rawStoredCommand = home + '//' + '.trackfw/scripts/trackfw-git-branch-guard.sh'
  writeJSON(path.join(home, '.claude', 'settings.json'), {
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: rawStoredCommand }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'), false)
})

// ---------------------------------------------------------------------------
// ROADMAP-2026-08-17 ML-4B -- hades-tf ML-4A barrier finding: a global entry
// with the CORRECT command but MISSING "type":"command" (hand-edited config,
// older trackfw version, another tool's merge) is silently never executed by
// Claude Code/GitHub Copilot CLI. Before this ML, hookArrayHasCommand/
// hasEntryPath still read such an entry as "installed", so the project-scope
// entry was skipped in favor of a global entry that never fires -- "nenhum
// dos dois escopos protege". Cursor is the exception: its schema never
// carries a "type" field, so a missing "type" there is normal, not
// malformed -- see the // formatting test above, whose fixture already has
// no "type" field and must keep skipping. Mirrors
// TestGBGDedup_Claude_ReWiresProjectEntryWhenGlobalEntryMissingType /
// TestGBGDedup_Copilot_ReWiresProjectEntryWhenGlobalEntryMissingType (Go).
// ---------------------------------------------------------------------------

test('injectClaudeHooks RE-WIRES project-scope git-branch-guard when global entry is missing "type":"command"', () => {
  const home = isolatedHome()
  const scriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.claude', 'settings.json'), {
    // Deliberately missing "type":"command" -- the ML-4A barrier finding's
    // exact malformed shape.
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ command: scriptPath }] }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(
    hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'),
    true,
    'the malformed global entry (missing "type") is never executed by Claude Code, so the project-scope entry must be re-wired'
  )
})

test('injectCopilotHooks RE-WIRES project-scope git-branch-guard when global entry is missing "type":"command"', () => {
  const home = isolatedHome()
  const scriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.copilot', 'settings.json'), {
    // Deliberately missing "type":"command" -- same ML-4A finding, Copilot's
    // own schema.
    hooks: { preToolUse: [{ matcher: 'bash', bash: scriptPath, cwd: '.', timeoutSec: 10 }] },
  })

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-project-'))
  injectCopilotHooks(dir)

  const data = readJSON(path.join(dir, '.github', 'hooks', 'trackfw-attention.json'))
  const pre = data.hooks.preToolUse
  assert.equal(pre.some(e => e.bash === 'scripts/trackfw-git-branch-guard.sh'), true)
})

// --- Fail-open ---

test('injectClaudeHooks fail-open: no global file -> project git-branch-guard entry still added', () => {
  isolatedHome() // empty $HOME, no global files at all

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-failopen-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'), true)
})

test('injectClaudeHooks fail-open: corrupted global file -> project git-branch-guard entry still added', () => {
  const home = isolatedHome()
  const settingsPath = path.join(home, '.claude', 'settings.json')
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, '{not valid json', 'utf8')

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-failopen-'))
  injectClaudeHooks(dir)

  const data = readJSON(path.join(dir, '.claude', 'settings.json'))
  assert.equal(hasClaudeHook(data, 'PreToolUse', 'Bash', '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'), true)
})

// ---------------------------------------------------------------------------
// "Message once" -- proved by EXECUTING the generated hook entries (not by
// counting JSON entries), per the architect's explicit AC3 wording ("prove
// executando, não por contagem de entradas no JSON"). Mirrors
// TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled in Go.
// ---------------------------------------------------------------------------

function runEntries(projectDir, scriptPaths) {
  let blocked = 0
  for (const script of scriptPaths) {
    try {
      execFileSync('bash', [script, 'git', 'push'], { cwd: projectDir, encoding: 'utf8' })
      // exit 0 -> allow, not counted.
    } catch (err) {
      const stderr = err.stderr || ''
      if (err.status === 2 && stderr.includes('git push bruto bloqueado')) {
        blocked++
      } else {
        throw new Error(`unexpected script outcome for ${script}: status=${err.status} stderr=${stderr}`)
      }
    }
  }
  return blocked
}

test('git-branch-guard block message appears exactly once when both scopes are installed', () => {
  const home = isolatedHome()
  generateGlobalGitBranchGuardScript(home)
  const globalScriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.claude', 'settings.json'), {
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: globalScriptPath }] }] },
  })

  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-exec-'))
  fs.writeFileSync(path.join(projectDir, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
  generateGitBranchGuardScript(projectDir)

  injectClaudeHooks(projectDir)

  const projectData = readJSON(path.join(projectDir, '.claude', 'settings.json'))
  const globalData = readJSON(path.join(home, '.claude', 'settings.json'))
  const scriptPaths = [
    ...claudeHookCommands(projectData, 'PreToolUse', 'Bash'),
    ...claudeHookCommands(globalData, 'PreToolUse', 'Bash'),
  ]
  const gitGuardEntries = scriptPaths.filter(p => p.includes('trackfw-git-branch-guard.sh'))
  assert.equal(gitGuardEntries.length, 1, `expected exactly 1 git-branch-guard entry across project+global, got ${JSON.stringify(gitGuardEntries)}`)

  const resolved = gitGuardEntries.map(p => p.replace('$CLAUDE_PROJECT_DIR', projectDir))
  const blocked = runEntries(projectDir, resolved)
  assert.equal(blocked, 1)
})

// ROADMAP-2026-08-17 ML-4B -- proves the fix end-to-end via EXECUTION, not
// just JSON presence. claudeHookCommandsWithType filters on type==='command'
// (unlike claudeHookCommands above), modeling what a real Claude Code
// runtime would actually fire -- the malformed global entry is present in
// the combined hook set but must be excluded from what executes. Before
// this ML: the malformed entry made the dedup skip the project entry AND
// the malformed entry itself never fires -- 0 blocks. After: the project
// entry is re-wired and, being structurally valid, executes -- 1 block.
// Mirrors TestGBGDedup_MalformedGlobalEntry_ProjectStillProtects (Go).
function claudeHookCommandsWithType(data, event, matcher) {
  const arr = (data.hooks && data.hooks[event]) || []
  const out = []
  for (const e of arr) {
    if (!e || e.matcher !== matcher || !Array.isArray(e.hooks)) continue
    for (const h of e.hooks) {
      if (h && h.type === 'command' && typeof h.command === 'string') out.push(h.command)
    }
  }
  return out
}

test('git-branch-guard: malformed global entry (missing "type") does not defeat protection', () => {
  const home = isolatedHome()
  generateGlobalGitBranchGuardScript(home)
  const globalScriptPath = gbgScriptPathFor(home)
  writeJSON(path.join(home, '.claude', 'settings.json'), {
    hooks: { PreToolUse: [{ matcher: 'Bash', hooks: [{ command: globalScriptPath }] }] },
  })

  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-exec-'))
  fs.writeFileSync(path.join(projectDir, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
  generateGitBranchGuardScript(projectDir)

  injectClaudeHooks(projectDir)

  const projectData = readJSON(path.join(projectDir, '.claude', 'settings.json'))
  const globalData = readJSON(path.join(home, '.claude', 'settings.json'))
  const executable = [
    ...claudeHookCommandsWithType(projectData, 'PreToolUse', 'Bash'),
    ...claudeHookCommandsWithType(globalData, 'PreToolUse', 'Bash'),
  ]
  const resolved = executable
    .filter(p => p.includes('trackfw-git-branch-guard.sh'))
    .map(p => p.replace('$CLAUDE_PROJECT_DIR', projectDir))

  const blocked = runEntries(projectDir, resolved)
  assert.equal(blocked, 1, `expected exactly 1 block (the re-wired, structurally-valid project entry); executable entries: ${JSON.stringify(resolved)}`)
})

test('non-vacuity: with both entries wired (pre-dedup simulation), the message appears twice', () => {
  const home = isolatedHome()
  generateGlobalGitBranchGuardScript(home)
  const globalScriptPath = gbgScriptPathFor(home)

  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-dedup-exec-'))
  fs.writeFileSync(path.join(projectDir, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
  generateGitBranchGuardScript(projectDir)
  const projectScriptPath = path.join(projectDir, 'scripts', 'trackfw-git-branch-guard.sh')

  const blocked = runEntries(projectDir, [projectScriptPath, globalScriptPath])
  assert.equal(blocked, 2, 'non-vacuity check failed: the test harness itself is broken, not proving anything about the dedup fix')
})
