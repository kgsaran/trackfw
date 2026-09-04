'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const { execBitRepresentavelPara, execBitNaoExercitado } = require('./exec-bit')
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
    if (execBitRepresentavelPara(scriptPath)) {
      assert.ok(stat.mode & 0o100, 'script deveria ser executável')
    } else {
      execBitNaoExercitado(scriptPath)
    }
    const content = fs.readFileSync(scriptPath, 'utf8')
    assert.ok(content.startsWith('#!/usr/bin/env bash'))
  })
})

test('generateGlobalGitBranchGuardScript escreve em <home>/.trackfw/scripts/', () => {
  withTmpDir((fakeHome) => {
    generateGlobalGitBranchGuardScript(fakeHome)
    const scriptPath = path.join(fakeHome, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    const stat = fs.statSync(scriptPath)
    if (execBitRepresentavelPara(scriptPath)) {
      assert.ok(stat.mode & 0o100, 'script global deveria ser executável')
    } else {
      execBitNaoExercitado(scriptPath)
      // Unico assert deste teste sobre o artefato: no ramo suprimido, medir o que E
      // representavel em NTFS em vez de nao medir nada.
      assert.ok(stat.size > 0, 'script global está vazio')
    }
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

// setupFixture cria um diretório com trackfw.yaml na raiz — o guard só bloqueia DENTRO de um
// projeto trackfw (ML-1A, ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-
// trackfw.md). setupFixtureWithoutTrackfwYAML é o par usado pelos testes de no-op.
function setupFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-git-branch-guard-fx-'))
  generateGitBranchGuardScript(dir)
  fs.writeFileSync(path.join(dir, 'trackfw.yaml'), 'project_name: fixture\n', 'utf8')
  return { dir, scriptPath: path.join(dir, 'scripts', 'trackfw-git-branch-guard.sh') }
}

function setupFixtureWithoutTrackfwYAML() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-git-branch-guard-fx-noop-'))
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

// --- Regressão: 3 bugs reais achados no teste manual end-to-end do ML-4A ---

test('comando encadeado bloqueia o git push do segundo comando (bug 1)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git status; git push origin HEAD' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
    assert.ok(stdout.includes('trackfw ship'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('path absoluto para o git bloqueia por basename (bug 2)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: '/usr/bin/git commit -m x' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
    assert.ok(stdout.includes('trackfw commit'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('prosa mencionando "git commit" dentro de string entre aspas NÃO bloqueia (bug 3, falso positivo crítico)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'bin/trackfw commit -m "test message mentioning git commit inside"' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
    assert.strictEqual(stdout, '')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// --- ML-1A (ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-
// release-7-0-0.md): item 1 (falso-positivo por prosa que COMEÇA a linha) + item 2 (brecha
// `git switch -c`) -----------------------------------------------------------------------

test('linha de mensagem de commit começando com "git checkout -b" (via heredoc, convenção do próprio CLAUDE.md) NÃO bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const cmd = "bin/trackfw commit -m \"$(cat <<'EOF'\n" +
      '  git checkout -b            -> bloqueado pelo guard\n' +
      '  trackfw branch new chore/  -> recusado\n' +
      'EOF\n' +
      ')"'
    const payload = JSON.stringify({ tool_input: { command: cmd } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
    assert.strictEqual(stdout, '')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('não-regressão: -m fechado seguido de git push real (; e &&) continua bloqueando', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    for (const cmd of ['git commit -m "x"; git push', 'git commit -m "x" && git push']) {
      const payload = JSON.stringify({ tool_input: { command: cmd } })
      const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
      assert.strictEqual(code, 2, `cmd=${cmd} stderr: ${stderr}`)
      assert.ok(stdout.includes('trackfw commit'), `cmd=${cmd} stdout: ${stdout}`)
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('não-regressão: heredoc mal-formado não esconde git push real (fallback de segurança)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const cmd = "git status <<'EOF'\nwhatever\nNOTEOF\ngit push origin main"
    const payload = JSON.stringify({ tool_input: { command: cmd } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
    assert.ok(stdout.includes('trackfw ship'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git switch -c bloqueia (forma alternativa a checkout -b)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git switch -c feat/x' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
    assert.ok(stdout.includes('trackfw branch new'))
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git switch --track -c feat/x (flag antes de -c) bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git switch --track -c feat/x' } })
    const { code, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git switch main (sem -c) não bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git switch main' } })
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

test('campo tool_info.command_line (payload real do Windsurf pre_run_command) bloqueia', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_info: { command_line: 'git commit -m "x"' } })
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

// Synthetic, empty $HOME: every test wrapped in this helper assumes NO global-scope guard
// (credential-guard or git-branch-guard) is wired for the CLI under test -- an empty tmpdir has
// no ~/.claude/settings.json, ~/.codex/hooks.json, etc. to read, so globalGitBranchGuardInstalled*
// (npm/src/generators/hooks.js) always resolves to false and the project-scope entry is added.
// The complementary state -- global wiring present, so the project-scope entry is legitimately
// SKIPPED (dedup) -- is intentionally NOT covered by these unit tests; it's covered end-to-end by
// scripts/check-gates-falsify.sh Scenario 67 (`trackfw discover --init` against a synthetic $HOME
// with the global guard actually wired, baseline + reverse-vacuity + detection). Declared here,
// rather than inherited by accident, per ROADMAP-2026-08-19 ML-3B.
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
  withIsolatedHome(() => {
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
  withIsolatedHome(() => {
    withTmpDir((tmp) => {
      injectCopilotHooks(tmp)
      const data = JSON.parse(fs.readFileSync(path.join(tmp, '.github', 'hooks', 'trackfw-attention.json'), 'utf8'))
      const entry = data.hooks.preToolUse.find(e => e.matcher === 'bash' && e.bash === 'scripts/trackfw-git-branch-guard.sh')
      assert.ok(entry, 'preToolUse missing git-branch-guard bash entry')
    })
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

test('injectWindsurfHooks writes .windsurf/hooks.json with hooks.pre_run_command, idempotently', () => {
  withTmpDir((tmp) => {
    injectWindsurfHooks(tmp)
    injectWindsurfHooks(tmp)
    const filePath = path.join(tmp, '.windsurf', 'hooks.json')
    const data = JSON.parse(fs.readFileSync(filePath, 'utf8'))
    const pre = data.hooks.pre_run_command
    assert.equal(pre.length, 1, 'expected exactly 1 pre_run_command entry (idempotent across 2 runs)')
    assert.equal(pre[0].command, 'bash scripts/trackfw-git-branch-guard.sh')
    assert.equal(pre[0].show_output, true)
  })
})

test('injectWindsurfHooks migrates the legacy .windsurf/hooks/trackfw-git-branch-guard.json file', () => {
  withTmpDir((tmp) => {
    const legacyDir = path.join(tmp, '.windsurf', 'hooks')
    fs.mkdirSync(legacyDir, { recursive: true })
    fs.writeFileSync(path.join(legacyDir, 'trackfw-git-branch-guard.json'), JSON.stringify({ version: 1, hooks: [] }), 'utf8')

    injectWindsurfHooks(tmp)

    assert.ok(!fs.existsSync(path.join(legacyDir, 'trackfw-git-branch-guard.json')), 'expected legacy hook file to be removed')
    assert.ok(fs.existsSync(path.join(tmp, '.windsurf', 'hooks.json')), 'expected .windsurf/hooks.json to be written')
  })
})

test('injectWindsurfHooks preserves other pre-existing hooks.json events/entries', () => {
  withTmpDir((tmp) => {
    const dir = path.join(tmp, '.windsurf')
    fs.mkdirSync(dir, { recursive: true })
    fs.writeFileSync(path.join(dir, 'hooks.json'), JSON.stringify({
      hooks: {
        post_run_command: [{ command: 'echo done', show_output: false }],
        pre_run_command: [{ command: 'some-other-tool-hook', show_output: true }],
      },
    }), 'utf8')

    injectWindsurfHooks(tmp)

    const data = JSON.parse(fs.readFileSync(path.join(dir, 'hooks.json'), 'utf8'))
    assert.equal(data.hooks.post_run_command.length, 1, 'expected pre-existing post_run_command entry to survive')
    const pre = data.hooks.pre_run_command
    assert.equal(pre.length, 2, 'expected 2 pre_run_command entries (pre-existing + git-guard)')
    assert.ok(pre.some(e => e.command === 'some-other-tool-hook'), 'pre-existing pre_run_command entry was lost')
    assert.ok(pre.some(e => e.command === 'bash scripts/trackfw-git-branch-guard.sh'), 'git-branch-guard entry was not added')
  })
})

test('injectAmazonQHooks creates .amazonq/cli-agents/q_cli_default.json with a valid custom agent + hook + deniedCommands, idempotently', () => {
  withTmpDir((tmp) => {
    injectAmazonQHooks(tmp)
    injectAmazonQHooks(tmp)
    const filePath = path.join(tmp, '.amazonq', 'cli-agents', 'q_cli_default.json')
    const data = JSON.parse(fs.readFileSync(filePath, 'utf8'))

    assert.equal(data.name, 'q_cli_default')
    assert.ok(typeof data.description === 'string' && data.description.length > 0)
    assert.deepEqual(data.tools, ['*'])

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

test('injectAmazonQHooks preserves pre-existing custom agent settings', () => {
  withTmpDir((tmp) => {
    const dir = path.join(tmp, '.amazonq', 'cli-agents')
    fs.mkdirSync(dir, { recursive: true })
    fs.writeFileSync(path.join(dir, 'q_cli_default.json'), JSON.stringify({
      someOtherSetting: 'keep-me',
      toolsSettings: { execute_bash: { deniedCommands: ['^rm -rf /'] } },
    }), 'utf8')

    injectAmazonQHooks(tmp)

    const data = JSON.parse(fs.readFileSync(path.join(dir, 'q_cli_default.json'), 'utf8'))
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
    assert.ok(fs.existsSync(path.join(tmp, '.amazonq', 'cli-agents', 'q_cli_default.json')), 'expected .amazonq/cli-agents/q_cli_default.json to be written by injectHooksDetected')
  })
})

// ---------------------------------------------------------------------------
// ML-1A (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-
// independente-de-fiacao.md): no-op fora de projeto trackfw.
// ---------------------------------------------------------------------------

test('fixture sem trackfw.yaml não tem trackfw.yaml em nenhum ancestral (não-vacuidade)', () => {
  const { dir } = setupFixtureWithoutTrackfwYAML()
  try {
    let d = dir
    for (;;) {
      assert.ok(!fs.existsSync(path.join(d, 'trackfw.yaml')), `premissa violada: trackfw.yaml em ${d}`)
      const parent = path.dirname(d)
      if (parent === d) break
      d = parent
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git push sem trackfw.yaml é no-op (exit 0)', () => {
  const { dir, scriptPath } = setupFixtureWithoutTrackfwYAML()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git push' } })
    const { code, stdout, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 0, `stderr: ${stderr}`)
    assert.strictEqual(stdout, '')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git commit/checkout -b/branch/switch -c sem trackfw.yaml são no-op (exit 0)', () => {
  const { dir, scriptPath } = setupFixtureWithoutTrackfwYAML()
  try {
    for (const cmd of ['git commit -m "x"', 'git checkout -b feat/x', 'git branch nova', 'git switch -c feat/x']) {
      const payload = JSON.stringify({ tool_input: { command: cmd } })
      const { code, stderr } = runGuard(dir, scriptPath, null, payload)
      assert.strictEqual(code, 0, `cmd=${cmd} stderr: ${stderr}`)
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('git push com trackfw.yaml continua bloqueando (reverse-vacuity)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const payload = JSON.stringify({ tool_input: { command: 'git push' } })
    const { code, stderr } = runGuard(dir, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('subdiretório de projeto trackfw continua protegido (raiz encontrada subindo)', () => {
  const { dir, scriptPath } = setupFixture()
  try {
    const sub = path.join(dir, 'a', 'b', 'c')
    fs.mkdirSync(sub, { recursive: true })
    const payload = JSON.stringify({ tool_input: { command: 'git push' } })
    const { code, stderr } = runGuard(sub, scriptPath, null, payload)
    assert.strictEqual(code, 2, `stderr: ${stderr}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
