'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const {
  generateGitBranchGuardScript,
  generateGlobalGitBranchGuardScript,
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectCopilotHooks,
  injectCursorHooks,
  injectWindsurfHooks,
  injectAmazonQHooks,
  injectHooksDetected,
} = require('../src/generators/hooks.js')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

function withTmpDir(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-git-branch-guard-'))
  try {
    fn(tmp)
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

// ---------------------------------------------------------------------------
// Generator: file creation (ML-1A port) — não injeta em nenhum hooks.json/
// settings.json de CLI (isso é escopo da Wave 3).
// ---------------------------------------------------------------------------

test('generateGitBranchGuardScript cria scripts/trackfw-git-branch-guard.sh executável', () => {
  withTmpDir((tmp) => {
    generateGitBranchGuardScript(tmp)
    const scriptPath = path.join(tmp, 'scripts', 'trackfw-git-branch-guard.sh')
    const stat = fs.statSync(scriptPath)
    assert.ok(stat.mode & 0o100, 'script deveria ser executável')
    const content = fs.readFileSync(scriptPath, 'utf8')
    assert.ok(content.startsWith('#!/usr/bin/env bash'))
  })
})

test('generateGlobalGitBranchGuardScript escreve em <home>/.trackfw/scripts/', () => {
  withTmpDir((fakeHome) => {
    generateGlobalGitBranchGuardScript(fakeHome)
    const scriptPath = path.join(fakeHome, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    const stat = fs.statSync(scriptPath)
    assert.ok(stat.mode & 0o100, 'script global deveria ser executável')
  })
})

test('generateGlobalGitBranchGuardScript com home vazio lança erro', () => {
  assert.throws(() => generateGlobalGitBranchGuardScript(''))
})

test('generateGitBranchGuardScript não injeta em nenhum hooks.json/settings.json', () => {
  withTmpDir((tmp) => {
    generateGitBranchGuardScript(tmp)
    for (const p of [
      '.claude/settings.json',
      '.codex/hooks.json',
      '.gemini/settings.json',
      '.github/hooks/hooks.json',
      '.cursor/hooks.json',
    ]) {
      assert.ok(!fs.existsSync(path.join(tmp, p)), `não deveria criar ${p} (escopo da Wave 3)`)
    }
  })
})

// ---------------------------------------------------------------------------
// Behavior — invoca o script real como subprocesso, mesmo padrão de
// credential_guard.test.js/runScript.
// ---------------------------------------------------------------------------

function setupFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-git-branch-guard-fx-'))
  generateGitBranchGuardScript(dir)
  return { dir, scriptPath: path.join(dir, 'scripts', 'trackfw-git-branch-guard.sh') }
}

function runGuard(dir, scriptPath, args, stdin, extraEnv) {
  const result = spawnSync('bash', [scriptPath, ...(args || [])], {
    cwd: dir,
    input: stdin === undefined ? '' : stdin,
    encoding: 'utf8',
    env: extraEnv ? { ...process.env, ...extraEnv } : process.env,
  })
  return { code: result.status, stdout: result.stdout || '', stderr: result.stderr || '' }
}

// --- Bloqueio: git commit ---------------------------------------------------

test('git commit via stdin JSON tool_input.command bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_name: 'Bash', tool_input: { command: 'git commit -m "x"' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2)
    assert.ok(stdout.includes('"decision":"block"'))
    assert.ok(stdout.includes('trackfw commit'))
    assert.ok(stderr.includes('CLAUDE.md'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git commit via argv bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const { code, stdout, stderr } = runGuard(dir, scriptPath, ['git', 'commit', '-m', 'x'], '')
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
    assert.ok(stdout.includes('"decision":"block"'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// --- Bloqueio: git push -----------------------------------------------------

test('git push via stdin JSON campo "command" bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ command: 'git push' })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
    assert.ok(stdout.includes('trackfw ship'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git --no-pager push (flag antes do subcomando) bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git --no-pager push' } })
    const { code, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// --- Bloqueio: git checkout -b ---------------------------------------------

test('git checkout -b bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git checkout -b feat/x' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
    assert.ok(stdout.includes('trackfw branch new'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git -C . checkout -b (flag antes do subcomando) bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git -C . checkout -b feat/x' } })
    const { code, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git checkout sem -b não bloqueia (allow silencioso)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git checkout feat/x' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
    assert.strictEqual(stdout, '')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// --- Allow: comandos git inofensivos ----------------------------------------

test('git status não bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git status' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
    assert.strictEqual(stdout, '')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git diff não bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git diff origin/main' } })
    const { code, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git log não bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git log --oneline -5' } })
    const { code, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('sem comando algum, allow por omissão', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const { code, stderr } = runGuard(dir, scriptPath, null, '')
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// --- Formatos de entrada -----------------------------------------------------

test('campo hook_input.command bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ hook_input: { command: 'git commit -m "x"' } })
    const { code, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('stdin cru não-JSON bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const { code, stderr } = runGuard(dir, scriptPath, null, 'git push')
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('fallback via variável de ambiente TRACKFW_GIT_COMMAND bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const { code, stderr } = runGuard(dir, scriptPath, null, undefined, { TRACKFW_GIT_COMMAND: 'git commit -m x' })
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// ---------------------------------------------------------------------------
// Wave 3 (ML-3B) — per-runtime wiring, ported from
// internal/generators/agentfiles_test.go's git-branch-guard section.
// ---------------------------------------------------------------------------

function withIsolatedHome(fn) {
  const origHome = process.env.HOME
  process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-home-iso-'))
  try {
    fn()
  } finally {
    process.env.HOME = origHome
  }
}

test('injectClaudeHooks wires PreToolUse[Bash] with the git branch guard command, idempotently', () => {
  withIsolatedHome(() => {
    withTmpDir((tmp) => {
      injectClaudeHooks(tmp)
      injectClaudeHooks(tmp)
      const data = JSON.parse(fs.readFileSync(path.join(tmp, '.claude', 'settings.json'), 'utf8'))
      const bashEntry = data.hooks.PreToolUse.find(e => e.matcher === 'Bash')
      const commands = bashEntry.hooks.map(h => h.command)
      assert.ok(commands.includes('$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))
      assert.equal(commands.filter(c => c === '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh').length, 1, 'idempotent across 2 runs')
    })
  })
})

test('injectCodexHooks wires PreToolUse[Bash] with the git branch guard command, idempotently', () => {
  withTmpDir((tmp) => {
    injectCodexHooks(tmp)
    injectCodexHooks(tmp)
    const data = JSON.parse(fs.readFileSync(path.join(tmp, '.codex', 'hooks.json'), 'utf8'))
    const bashEntry = data.hooks.PreToolUse.find(e => e.matcher === 'Bash')
    const commands = bashEntry.hooks.map(h => h.command)
    assert.ok(commands.includes('"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"'))
    assert.equal(commands.filter(c => c === '"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"').length, 1, 'idempotent across 2 runs')
  })
})

test('injectGeminiHooks wires BeforeTool[run_shell_command] with the git branch guard command, idempotently', () => {
  withIsolatedHome(() => {
    withTmpDir((tmp) => {
      injectGeminiHooks(tmp)
      injectGeminiHooks(tmp)
      const data = JSON.parse(fs.readFileSync(path.join(tmp, '.gemini', 'settings.json'), 'utf8'))
      const entry = data.hooks.BeforeTool.find(e => e.matcher === 'run_shell_command')
      const commands = entry.hooks.map(h => h.command)
      assert.ok(commands.includes('$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))
      assert.equal(commands.filter(c => c === '$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh').length, 1, 'idempotent across 2 runs')
    })
  })
})

test('injectCopilotHooks wires preToolUse[bash] with the git branch guard command', () => {
  withTmpDir((tmp) => {
    injectCopilotHooks(tmp)
    const data = JSON.parse(fs.readFileSync(path.join(tmp, '.github', 'hooks', 'trackfw-attention.json'), 'utf8'))
    const entry = data.hooks.preToolUse.find(e => e.matcher === 'bash' && e.bash === 'scripts/trackfw-git-branch-guard.sh')
    assert.ok(entry, 'preToolUse missing git-branch-guard bash entry')
  })
})

test('injectCursorHooks wires beforeShellExecution with the git branch guard command, idempotently', () => {
  withIsolatedHome(() => {
    withTmpDir((tmp) => {
      injectCursorHooks(tmp)
      injectCursorHooks(tmp)
      const data = JSON.parse(fs.readFileSync(path.join(tmp, '.cursor', 'hooks.json'), 'utf8'))
      const entries = data.hooks.beforeShellExecution.filter(e => e.command === 'scripts/trackfw-git-branch-guard.sh')
      assert.equal(entries.length, 1, 'idempotent across 2 runs')
    })
  })
})

test('injectWindsurfHooks writes .windsurf/hooks/trackfw-git-branch-guard.json, idempotently', () => {
  withTmpDir((tmp) => {
    injectWindsurfHooks(tmp)
    injectWindsurfHooks(tmp)
    const filePath = path.join(tmp, '.windsurf', 'hooks', 'trackfw-git-branch-guard.json')
    const data = JSON.parse(fs.readFileSync(filePath, 'utf8'))
    assert.equal(data.version, 1)
    assert.equal(data.hooks.length, 1, 'expected exactly 1 hook entry (idempotent across 2 runs)')
    assert.equal(data.hooks[0].trigger, 'pre_run_command')
    assert.equal(data.hooks[0].action.command, 'scripts/trackfw-git-branch-guard.sh')
  })
})

test('injectAmazonQHooks creates .amazonq/settings.json with hook + deniedCommands, idempotently', () => {
  withTmpDir((tmp) => {
    injectAmazonQHooks(tmp)
    injectAmazonQHooks(tmp)
    const data = JSON.parse(fs.readFileSync(path.join(tmp, '.amazonq', 'settings.json'), 'utf8'))

    const pre = data.hooks.preToolUse
    assert.equal(pre.length, 1, 'expected exactly 1 preToolUse matcher entry (idempotent across 2 runs)')
    assert.equal(pre[0].matcher, 'execute_bash')
    assert.equal(pre[0].hooks.length, 1, 'expected exactly 1 command inside preToolUse[execute_bash]')
    assert.equal(pre[0].hooks[0].command, 'scripts/trackfw-git-branch-guard.sh')

    const denied = data.toolsSettings.execute_bash.deniedCommands
    assert.equal(denied.length, 1, 'idempotent across 2 runs')
    assert.equal(denied[0], '^git (commit|push|checkout -b)')
  })
})

test('injectAmazonQHooks preserves pre-existing settings', () => {
  withTmpDir((tmp) => {
    fs.mkdirSync(path.join(tmp, '.amazonq'), { recursive: true })
    fs.writeFileSync(path.join(tmp, '.amazonq', 'settings.json'), JSON.stringify({
      someOtherSetting: 'keep-me',
      toolsSettings: { execute_bash: { deniedCommands: ['^rm -rf /'] } },
    }), 'utf8')

    injectAmazonQHooks(tmp)

    const data = JSON.parse(fs.readFileSync(path.join(tmp, '.amazonq', 'settings.json'), 'utf8'))
    assert.equal(data.someOtherSetting, 'keep-me')
    const denied = data.toolsSettings.execute_bash.deniedCommands
    assert.equal(denied.length, 2)
    assert.ok(denied.includes('^rm -rf /'))
    assert.ok(denied.includes('^git (commit|push|checkout -b)'))
  })
})

test('injectHooksDetected dispatches Amazon Q when .amazonq dir exists', () => {
  withTmpDir((tmp) => {
    fs.mkdirSync(path.join(tmp, '.amazonq'), { recursive: true })
    injectHooksDetected(tmp)
    assert.ok(fs.existsSync(path.join(tmp, '.amazonq', 'settings.json')), 'expected .amazonq/settings.json to be written by injectHooksDetected')
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
