'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const config = require('../src/config/index.js')
const { generateCredentialGuardScript, generateAttentionScripts } = require('../src/generators/hooks.js')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

function withTmpDir(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-credential-guard-'))
  try {
    fn(tmp)
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

// ---------------------------------------------------------------------------
// Generator: file creation (ML-1A) — não injeta em nenhum hooks.json/settings.json
// de CLI (escopo da Wave 2).
// ---------------------------------------------------------------------------

test('generateCredentialGuardScript cria scripts/trackfw-credential-guard.sh executável', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    const scriptPath = path.join(tmp, 'scripts', 'trackfw-credential-guard.sh')
    const stat = fs.statSync(scriptPath)
    assert.ok(stat.mode & 0o100, 'script deveria ser executável')
    const content = fs.readFileSync(scriptPath, 'utf8')
    assert.ok(content.startsWith('#!/usr/bin/env bash'))
  })
})

// ---------------------------------------------------------------------------
// Config: credential_guard.mode
// ---------------------------------------------------------------------------

test('credential_guard.mode default é warn quando ausente', () => {
  withTmpDir((tmp) => {
    config.reset()
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.credentialGuard.mode, 'warn')
  })
})

test('credential_guard: {mode: block} é lido corretamente', () => {
  withTmpDir((tmp) => {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'credential_guard:\n  mode: block\n', 'utf8')
    config.reset()
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.credentialGuard.mode, 'block')
  })
})

test('valor de mode inválido cai para warn (fallback silencioso)', () => {
  withTmpDir((tmp) => {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'credential_guard:\n  mode: nonsense\n', 'utf8')
    config.reset()
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.credentialGuard.mode, 'warn')
  })
})

// ---------------------------------------------------------------------------
// Behavior — invoca o script real como subprocesso.
// ---------------------------------------------------------------------------

const SYNTHETIC_JWT = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.abc123def456ghi789'

function runScript(tmp, stdin) {
  const scriptPath = path.join(tmp, 'scripts', 'trackfw-credential-guard.sh')
  const result = spawnSync('bash', [scriptPath], { cwd: tmp, input: stdin, encoding: 'utf8' })
  return { code: result.status, stdout: result.stdout || '', stderr: result.stderr || '' }
}

function attentionFileExists(tmp) {
  return fs.existsSync(path.join(tmp, 'docs', 'roadmaps', '.trackfw-credential-guard.json'))
}

test('sem match, script é no-op silencioso', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const { code } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: 'echo hello' } }))
    assert.strictEqual(code, 0)
    assert.ok(!attentionFileExists(tmp))
  })
})

test('JWT impresso no stdout — modo warn (default) alerta e sai 0', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const { code, stderr } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
    assert.strictEqual(code, 0)
    assert.ok(stderr.includes('JWT'))
    assert.ok(attentionFileExists(tmp))
  })
})

test('JWT redirecionado para /dev/null é destino efêmero — sem alerta', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const { code } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT} > /dev/null` } }))
    assert.strictEqual(code, 0)
    assert.ok(!attentionFileExists(tmp))
  })
})

test('JWT redirecionado para arquivo comum não é efêmero — alerta (caso do incidente real)', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const { code } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT} > /tmp/token.txt` } }))
    assert.strictEqual(code, 0)
    assert.ok(attentionFileExists(tmp))
  })
})

test('modo block sai com exit code 2 e não escreve attention.json', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'credential_guard:\n  mode: block\n', 'utf8')
    const { code } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
    assert.strictEqual(code, 2)
    assert.ok(!attentionFileExists(tmp))
  })
})

// trackfw-attention-cleanup.sh apaga incondicionalmente $ROADMAP_DIR/.trackfw-attention.json — em
// harnesses que rodam hooks do mesmo evento concorrentemente (ex.: Codex CLI, PostToolUse com
// matchers ".*" e "Bash" ambos batendo em uma chamada Bash), isso podia apagar o aviso do
// credential-guard antes de este ser lido. O credential-guard agora usa um arquivo dedicado
// (.trackfw-credential-guard.json), então o cleanup não deve mais afetá-lo.
test('trackfw-attention-cleanup.sh não apaga .trackfw-credential-guard.json (arquivo dedicado)', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    generateAttentionScripts(null, tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')

    const { code } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
    assert.strictEqual(code, 0)
    assert.ok(attentionFileExists(tmp))

    const cleanupPath = path.join(tmp, 'scripts', 'trackfw-attention-cleanup.sh')
    const result = spawnSync('bash', [cleanupPath], { cwd: tmp, encoding: 'utf8' })
    assert.strictEqual(result.status, 0)

    assert.ok(attentionFileExists(tmp), '.trackfw-credential-guard.json não deveria ter sido apagado pelo cleanup')
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
