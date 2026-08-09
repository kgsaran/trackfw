'use strict'

// ML-4A -- teste de sabotagem end-to-end. Diferente de credential_guard.test.js (que já
// invoca o script real como subprocesso, mas com um payload JSON genérico escrito à mão),
// este arquivo:
//
//   1. Gera o wiring REAL de cada CLI via injectXHooks (o mesmo gerador exercitado por
//      generators.test.js), confirmando que o hooks.json/settings.json resultante de fato
//      referencia "scripts/trackfw-credential-guard.sh".
//   2. Constrói o payload JSON EXATO que aquele CLI envia via stdin ao hook, conforme o
//      schema documentado em docs/cli-parity.md por CLI.
//   3. Materializa um JWT sintético (nunca hardcoded como token real plausível) dentro do
//      payload.
//   4. Invoca o script gerado (não uma cópia, não uma reimplementação da regex) como
//      subprocesso, passando o payload por stdin.
//   5. Confirma detecção nos dois modos (warn/block) e prova negativa.
//
// Cobertura por CLI -- ver a nota equivalente em
// internal/generators/credential_guard_sabotage_test.go (Go) para o detalhe completo do
// motivo de Codex/Gemini/Copilot ficarem de fora: schema de payload de stdin não confirmado
// com confiança suficiente em docs/cli-parity.md (só o formato do arquivo de configuração
// hooks.json é confirmado para esses três, não o payload de runtime).
//   - Claude Code: COBERTO (obrigatório pelo AC da REQ).
//   - Cursor: COBERTO.
//   - Kiro: COBERTO.
//   - Codex, Gemini CLI, GitHub Copilot: SEM teste de sabotagem end-to-end.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')
const { generateCredentialGuardScript } = require('../src/generators/hooks.js')
const { injectClaudeHooks, injectCursorHooks, injectKiroHooks } = require('../src/generators/hooks')

const SYNTHETIC_JWT = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.abc123def456ghi789'

function withTmpDir(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-sabotage-'))
  try {
    return fn(tmp)
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

function setupSabotageFixture(tmp, injectHooks, trackfwYAML) {
  // Isolate global credential-guard dedup check (ML-3A) from the real $HOME.
  const origHome = process.env.HOME
  process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-sabotage-home-'))
  try {
    generateCredentialGuardScript(tmp)
    injectHooks(tmp)
  } finally {
    process.env.HOME = origHome
  }
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), trackfwYAML || 'roadmap_dir: docs/roadmaps\n', 'utf8')
  return path.join(tmp, 'scripts', 'trackfw-credential-guard.sh')
}

function runScript(tmp, scriptPath, stdin) {
  const result = spawnSync('bash', [scriptPath], { cwd: tmp, input: stdin, encoding: 'utf8' })
  return { code: result.status, stdout: result.stdout || '', stderr: result.stderr || '' }
}

function attentionFileExists(tmp) {
  return fs.existsSync(path.join(tmp, 'docs', 'roadmaps', '.trackfw-credential-guard.json'))
}

function readJSON(p) {
  return JSON.parse(fs.readFileSync(p, 'utf8'))
}

// ---------------------------------------------------------------------------
// Claude Code -- PreToolUse/PostToolUse, matcher "Bash".
// Schema confirmado: {"tool_name":"Bash","tool_input":{"command":"..."}}
// ---------------------------------------------------------------------------

function claudeCodePreToolUsePayload(command) {
  return JSON.stringify({ tool_name: 'Bash', tool_input: { command } })
}

test('Sabotage/ClaudeCode: wiring referencia o script real', () => {
  withTmpDir((tmp) => {
    setupSabotageFixture(tmp, injectClaudeHooks, '')
    const data = readJSON(path.join(tmp, '.claude', 'settings.json'))
    const pre = data.hooks.PreToolUse.find((e) => e.matcher === 'Bash')
    assert.ok(pre, 'PreToolUse[Bash] ausente')
    assert.ok(
      pre.hooks.some((h) => h.command === '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'),
      'PreToolUse[Bash] não referencia trackfw-credential-guard.sh'
    )
  })
})

test('Sabotage/ClaudeCode: JWT sintético no comando Bash -- modo warn detecta', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectClaudeHooks, '')
    const { code } = runScript(tmp, scriptPath, claudeCodePreToolUsePayload(`echo ${SYNTHETIC_JWT}`))
    assert.strictEqual(code, 0)
    assert.ok(attentionFileExists(tmp), '.trackfw-credential-guard.json deveria ter sido escrito')
  })
})

test('Sabotage/ClaudeCode: JWT sintético no comando Bash -- modo block sai com exit 2', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectClaudeHooks, 'credential_guard:\n  mode: block\n')
    const { code } = runScript(tmp, scriptPath, claudeCodePreToolUsePayload(`echo ${SYNTHETIC_JWT}`))
    assert.strictEqual(code, 2)
  })
})

test('Sabotage/ClaudeCode: prova negativa -- payload sem JWT não detecta', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectClaudeHooks, '')
    const { code } = runScript(tmp, scriptPath, claudeCodePreToolUsePayload('git status'))
    assert.strictEqual(code, 0)
    assert.ok(!attentionFileExists(tmp), 'prova negativa falhou: teste não pode ser vácuo/sempre-verde')
  })
})

// ---------------------------------------------------------------------------
// Cursor -- beforeShellExecution/afterShellExecution.
// Schema confirmado (docs/cli-parity.md, "Cursor wiring (ML-2E)"):
// {"command":"...","cwd":"...","sandbox":false}
// ---------------------------------------------------------------------------

function cursorBeforeShellExecutionPayload(command) {
  return JSON.stringify({ command, cwd: '/tmp/fixture', sandbox: false })
}

test('Sabotage/Cursor: wiring referencia o script real', () => {
  withTmpDir((tmp) => {
    setupSabotageFixture(tmp, injectCursorHooks, '')
    const data = readJSON(path.join(tmp, '.cursor', 'hooks.json'))
    assert.ok(
      data.hooks.beforeShellExecution.some((e) => e.command === 'scripts/trackfw-credential-guard.sh'),
      'hooks.beforeShellExecution não referencia trackfw-credential-guard.sh'
    )
  })
})

test('Sabotage/Cursor: JWT sintético no comando de shell -- modo warn detecta', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectCursorHooks, '')
    const { code } = runScript(tmp, scriptPath, cursorBeforeShellExecutionPayload(`echo ${SYNTHETIC_JWT}`))
    assert.strictEqual(code, 0)
    assert.ok(attentionFileExists(tmp), '.trackfw-credential-guard.json deveria ter sido escrito')
  })
})

test('Sabotage/Cursor: JWT sintético no comando de shell -- modo block sai com exit 2', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectCursorHooks, 'credential_guard:\n  mode: block\n')
    const { code } = runScript(tmp, scriptPath, cursorBeforeShellExecutionPayload(`echo ${SYNTHETIC_JWT}`))
    assert.strictEqual(code, 2)
  })
})

test('Sabotage/Cursor: prova negativa -- payload sem JWT não detecta', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectCursorHooks, '')
    const { code } = runScript(tmp, scriptPath, cursorBeforeShellExecutionPayload('git status'))
    assert.strictEqual(code, 0)
    assert.ok(!attentionFileExists(tmp))
  })
})

test('Sabotage/Cursor: JWT apenas na saída capturada (afterShellExecution) -- modo warn detecta', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectCursorHooks, '')
    const payload = JSON.stringify({
      command: 'curl https://internal.example/token',
      cwd: '/tmp/fixture',
      sandbox: false,
      output: `token=${SYNTHETIC_JWT}`,
      duration: 123,
    })
    const { code } = runScript(tmp, scriptPath, payload)
    assert.strictEqual(code, 0)
    assert.ok(attentionFileExists(tmp), 'JWT no campo output do afterShellExecution deveria ser detectado')
  })
})

// ---------------------------------------------------------------------------
// Kiro -- PreToolUse/PostToolUse, matcher "shell".
// Schema confirmado (docs/cli-parity.md, "Kiro wiring (ML-2F)"):
// {"hook_event_name","cwd","session_id","tool_name","tool_input"}
// ---------------------------------------------------------------------------

function kiroPreToolUsePayload(cwd, command) {
  return JSON.stringify({
    hook_event_name: 'PreToolUse',
    cwd,
    session_id: 'sess-sabotage-test',
    tool_name: 'execute_bash',
    tool_input: { command },
  })
}

test('Sabotage/Kiro: wiring referencia o script real', () => {
  withTmpDir((tmp) => {
    setupSabotageFixture(tmp, injectKiroHooks, '')
    const data = readJSON(path.join(tmp, '.kiro', 'hooks', 'trackfw-attention.json'))
    const guardEntries = data.hooks.filter((h) => h.action && h.action.command === 'scripts/trackfw-credential-guard.sh')
    assert.ok(guardEntries.some((h) => h.trigger === 'PreToolUse'), 'PreToolUse guard ausente')
    assert.ok(guardEntries.some((h) => h.trigger === 'PostToolUse'), 'PostToolUse guard ausente')
  })
})

test('Sabotage/Kiro: JWT sintético em tool_input -- modo warn detecta', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectKiroHooks, '')
    const { code } = runScript(tmp, scriptPath, kiroPreToolUsePayload(tmp, `echo ${SYNTHETIC_JWT}`))
    assert.strictEqual(code, 0)
    assert.ok(attentionFileExists(tmp), '.trackfw-credential-guard.json deveria ter sido escrito')
  })
})

test('Sabotage/Kiro: JWT sintético em tool_input -- modo block sai com exit 2', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectKiroHooks, 'credential_guard:\n  mode: block\n')
    const { code } = runScript(tmp, scriptPath, kiroPreToolUsePayload(tmp, `echo ${SYNTHETIC_JWT}`))
    assert.strictEqual(code, 2)
  })
})

test('Sabotage/Kiro: prova negativa -- payload sem JWT não detecta', () => {
  withTmpDir((tmp) => {
    const scriptPath = setupSabotageFixture(tmp, injectKiroHooks, '')
    const { code } = runScript(tmp, scriptPath, kiroPreToolUsePayload(tmp, 'git status'))
    assert.strictEqual(code, 0)
    assert.ok(!attentionFileExists(tmp))
  })
})
