'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const config = require('../src/config/index.js')
const { generateCredentialGuardScript, generateGlobalCredentialGuardScript, generateAttentionScripts } = require('../src/generators/hooks.js')

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

// ---------------------------------------------------------------------------
// generateGlobalCredentialGuardScript — escopo global (~/.trackfw/scripts/), ML-1A do roadmap
// ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-
// update-harness.md. Usa SEMPRE um HOME de fixture (withTmpDir) — nunca o HOME real de quem roda
// a suíte.
// ---------------------------------------------------------------------------

function runGlobalScript(fakeHome, cwd, stdin) {
  const scriptPath = path.join(fakeHome, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
  const result = spawnSync('bash', [scriptPath], { cwd, input: stdin, encoding: 'utf8' })
  return { code: result.status, stdout: result.stdout || '', stderr: result.stderr || '' }
}

test('generateGlobalCredentialGuardScript cria ~/.trackfw/scripts/trackfw-credential-guard.sh executável, sem a guarda de projeto', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    const scriptPath = path.join(fakeHome, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
    const stat = fs.statSync(scriptPath)
    assert.ok(stat.mode & 0o100, 'script global deveria ser executável')
    const content = fs.readFileSync(scriptPath, 'utf8')
    assert.ok(content.startsWith('#!/usr/bin/env bash'))
    assert.ok(!content.includes('[ -f "trackfw.yaml" ] || exit 0'), 'script global não deve conter a guarda de projeto')
  })
})

test('generateGlobalCredentialGuardScript com home vazio lança erro', () => {
  assert.throws(() => generateGlobalCredentialGuardScript(''))
})

test('script global detecta JWT mesmo fora de um projeto trackfw (sem trackfw.yaml)', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      const { code, stderr } = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(code, 0)
      assert.ok(stderr.includes('JWT'))
    })
  })
})

test('script global detecta AWS key igual à variante de projeto (mesmo payload sintético)', () => {
  const SYNTHETIC_AWS_KEY = 'AKIAABCDEFGHIJKLMNOP'
  withTmpDir((projectDir) => {
    generateCredentialGuardScript(projectDir)
    fs.writeFileSync(path.join(projectDir, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const projectResult = runScript(projectDir, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_AWS_KEY}` } }))

    withTmpDir((fakeHome) => {
      generateGlobalCredentialGuardScript(fakeHome)
      withTmpDir((cwd) => {
        const globalResult = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_AWS_KEY}` } }))
        assert.strictEqual(projectResult.code, 0)
        assert.strictEqual(globalResult.code, 0)
        assert.ok(projectResult.stderr.includes('AWS'))
        assert.ok(globalResult.stderr.includes('AWS'))
      })
    })
  })
})

test('script global sempre usa modo warn, ignorando trackfw.yaml com mode: block no cwd', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      fs.writeFileSync(path.join(cwd, 'trackfw.yaml'), 'credential_guard:\n  mode: block\n', 'utf8')
      const { code, stderr } = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(code, 0, 'script global nunca deve bloquear (sempre warn)')
      assert.ok(stderr.includes('warning'))
    })
  })
})

test('script global só escreve .trackfw-credential-guard.json se docs/roadmaps já existir no cwd', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      const noRoadmapsResult = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(noRoadmapsResult.code, 0)
      assert.ok(noRoadmapsResult.stderr.includes('JWT'))
      assert.ok(!attentionFileExists(cwd), 'não deveria criar docs/roadmaps num projeto qualquer')

      fs.mkdirSync(path.join(cwd, 'docs', 'roadmaps'), { recursive: true })
      const withRoadmapsResult = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(withRoadmapsResult.code, 0)
      assert.ok(attentionFileExists(cwd), 'deveria escrever attention signal quando docs/roadmaps existe')
    })
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
