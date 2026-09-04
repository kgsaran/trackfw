'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const { execBitRepresentavelPara, execBitNaoExercitado } = require('./exec-bit')
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
    if (execBitRepresentavelPara(scriptPath)) {
      assert.ok(stat.mode & 0o100, 'script deveria ser executável')
    } else {
      execBitNaoExercitado(scriptPath)
    }
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
    if (execBitRepresentavelPara(scriptPath)) {
      assert.ok(stat.mode & 0o100, 'script global deveria ser executável')
    } else {
      execBitNaoExercitado(scriptPath)
    }
    const content = fs.readFileSync(scriptPath, 'utf8')
    assert.ok(content.startsWith('#!/usr/bin/env bash'))
    assert.ok(!content.includes('[ -f "trackfw.yaml" ] || exit 0'), 'script global não deve conter a guarda de projeto')
  })
})

test('generateGlobalCredentialGuardScript com home vazio lança erro', () => {
  assert.throws(() => generateGlobalCredentialGuardScript(''))
})

test('script global detecta JWT mesmo fora de um projeto trackfw (sem trackfw.yaml)', () => {
  // Ao contrário da variante de projeto, o script global NÃO é no-op fora de um projeto trackfw --
  // esse é o propósito da mudança. Sem trackfw.yaml no cwd, o fallback de modo é "block"
  // (ADR-2026-08-06 emenda 6).
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      const { code, stderr } = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(code, 2, 'modo block (fallback sem trackfw.yaml)')
      assert.ok(stderr.includes('JWT'))
    })
  })
})

test('script global detecta AWS key igual à variante de projeto (mesmo payload sintético)', () => {
  // Prova que a detecção (mesmo payload sintético) é idêntica entre projeto e global -- os modos
  // default divergem por design (projeto: warn; global sem trackfw.yaml: block), então os exit
  // codes divergem, mas ambos devem mencionar AWS.
  const SYNTHETIC_AWS_KEY = 'AKIAABCDEFGHIJKLMNOP'
  withTmpDir((projectDir) => {
    generateCredentialGuardScript(projectDir)
    fs.writeFileSync(path.join(projectDir, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const projectResult = runScript(projectDir, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_AWS_KEY}` } }))

    withTmpDir((fakeHome) => {
      generateGlobalCredentialGuardScript(fakeHome)
      withTmpDir((cwd) => {
        const globalResult = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_AWS_KEY}` } }))
        assert.strictEqual(projectResult.code, 0, 'projeto (fallback warn)')
        assert.strictEqual(globalResult.code, 2, 'global sem trackfw.yaml (fallback block)')
        assert.ok(projectResult.stderr.includes('AWS'))
        assert.ok(globalResult.stderr.includes('AWS'))
      })
    })
  })
})

// ADR-2026-08-06 emenda 6 (2026-08-08): o script global agora reusa a mesma leitura de
// credential_guard.mode de trackfw.yaml que a variante de projeto já faz -- quando o cwd tem
// trackfw.yaml com mode explícito, esse valor é respeitado (warn ou block); sem trackfw.yaml ou
// sem a chave, o fallback deixa de ser "warn" e passa a ser "block" (ver testes abaixo).
test('script global respeita trackfw.yaml com mode: block explícito no cwd (bloqueia)', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      fs.writeFileSync(path.join(cwd, 'trackfw.yaml'), 'credential_guard:\n  mode: block\n', 'utf8')
      const { code, stderr } = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(code, 2, 'trackfw.yaml com mode: block explícito deve bloquear')
      assert.ok(stderr.includes('blocked'))
    })
  })
})

test('script global respeita trackfw.yaml com mode: warn explícito no cwd (não bloqueia)', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      fs.writeFileSync(path.join(cwd, 'trackfw.yaml'), 'credential_guard:\n  mode: warn\n', 'utf8')
      const { code, stderr } = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(code, 0, 'trackfw.yaml com mode: warn explícito não deve bloquear')
      assert.ok(stderr.includes('warning'))
    })
  })
})

test('script global sem trackfw.yaml no cwd usa fallback block (default seguro)', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      const { code, stderr } = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_JWT}` } }))
      assert.strictEqual(code, 2, 'fallback sem trackfw.yaml deve bloquear (ADR-2026-08-06 emenda 6)')
      assert.ok(stderr.includes('blocked'))
    })
  })
})

test('script global só escreve .trackfw-credential-guard.json se docs/roadmaps já existir no cwd (modo warn)', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      // mode: warn explícito para exercitar a checagem de ROADMAP_DIR independente do fallback
      // default de modo global (block, ADR-2026-08-06 emenda 6).
      fs.writeFileSync(path.join(cwd, 'trackfw.yaml'), 'credential_guard:\n  mode: warn\n', 'utf8')
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

// ---------------------------------------------------------------------------
// ML-3B (ROADMAP-2026-08-08) — os 3 cenários do REQ que escapavam antes desta REQ.
// ---------------------------------------------------------------------------

// (a) trackfw.yaml ausente (ou sem credential_guard.mode) no cwd → script GLOBAL bloqueia
// (exit code 2, não warn) quando payload tem JWT/AWS key. Já coberto estruturalmente pelos
// testes acima ("script global sem trackfw.yaml no cwd usa fallback block (default seguro)" e
// "script global detecta AWS key igual à variante de projeto") — deixado explícito aqui como
// cenário (a) da REQ, com asserção adicional de que nenhum trackfw.yaml existe no cwd.
test('(a) sem trackfw.yaml no cwd, script global bloqueia (exit 2) payload com AWS key', () => {
  withTmpDir((fakeHome) => {
    generateGlobalCredentialGuardScript(fakeHome)
    withTmpDir((cwd) => {
      assert.ok(!fs.existsSync(path.join(cwd, 'trackfw.yaml')), 'cwd não deveria ter trackfw.yaml')
      const SYNTHETIC_AWS_KEY = 'AKIAABCDEFGHIJKLMNOP'
      const { code, stderr } = runGlobalScript(fakeHome, cwd, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `echo ${SYNTHETIC_AWS_KEY}` } }))
      assert.strictEqual(code, 2, 'ausência de trackfw.yaml deve resolver para modo block (fallback)')
      assert.ok(stderr.includes('blocked'))
    })
  })
})

// (b) Payload de tool call Read/Write/Edit com JWT/AWS key → injectClaudeHooks (e ao menos mais
// um CLI) grava a entrada de matcher Read/Write|Edit esperada no arquivo de hooks gerado.
// A cobertura estrutural completa (todos os CLIs) já está em npm/tests/generators.test.js
// (injectClaudeHooks/injectGeminiHooks/injectKiroHooks/injectCopilotHooks/injectCursorHooks).
// Aqui validamos ponta-a-ponta: o comando registrado no matcher Read/Write|Edit do Claude é o
// mesmo script que, invocado com um payload real de tool call Read/Write contendo JWT/AWS key,
// efetivamente detecta o segredo -- provando que o wiring aponta para um script funcional.
test('(b) matcher Read/Write|Edit do injectClaudeHooks aponta para o script que detecta JWT em payload de tool call Read', () => {
  const { injectClaudeHooks } = require('../src/generators/hooks.js')
  // injectClaudeHooks pula o wiring de projeto quando o credential-guard GLOBAL já está
  // instalado para o Claude no $HOME real (dedup — ver globalCredentialGuardInstalledClaude).
  // Isola $HOME num diretório vazio para este teste não depender do estado da máquina que roda
  // a suíte (mesma técnica usada em npm/tests/generators.test.js).
  const origHome = process.env.HOME
  process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-home-iso-'))
  try {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    injectClaudeHooks(tmp)

    const settings = JSON.parse(fs.readFileSync(path.join(tmp, '.claude', 'settings.json'), 'utf8'))
    const readEntry = settings.hooks.PreToolUse.find(e => e.matcher === 'Read')
    assert.ok(readEntry, 'PreToolUse deveria ter entrada com matcher Read')
    const readCommand = readEntry.hooks[0].command
    assert.ok(readCommand.includes('trackfw-credential-guard.sh'))

    const writeEntry = settings.hooks.PreToolUse.find(e => e.matcher === 'Write|Edit')
    assert.ok(writeEntry, 'PreToolUse deveria ter entrada com matcher Write|Edit')

    // Simula o payload que o Claude Code enviaria para o hook Read/Write|Edit -- o script não
    // distingue tool_name, só escaneia tool_input; o wiring é o que direciona o evento certo.
    const readPayload = JSON.stringify({ tool_name: 'Read', tool_input: { file_path: '/tmp/x', content: `token=${SYNTHETIC_JWT}` } })
    const { code: readCode, stderr: readStderr } = runScript(tmp, readPayload)
    assert.strictEqual(readCode, 0, 'modo warn (default) não bloqueia')
    assert.ok(readStderr.includes('JWT'))

    const SYNTHETIC_AWS_KEY = 'AKIAABCDEFGHIJKLMNOP'
    const writePayload = JSON.stringify({ tool_name: 'Write', tool_input: { file_path: '/tmp/y', content: `key=${SYNTHETIC_AWS_KEY}` } })
    const { code: writeCode, stderr: writeStderr } = runScript(tmp, writePayload)
    assert.strictEqual(writeCode, 0)
    assert.ok(writeStderr.includes('AWS'))
  })
  } finally {
    process.env.HOME = origHome
  }
})

// (b), segundo CLI (Cursor) -- mesma prova ponta-a-ponta que o teste acima faz para Claude,
// exigida explicitamente pelo handoff ("injectClaudeHooks e pelo menos mais um CLI").
test('(b) matcher Read/Write do injectCursorHooks aponta para o script que detecta AWS key em payload de tool call Write', () => {
  const { injectCursorHooks } = require('../src/generators/hooks.js')
  const origHome = process.env.HOME
  process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-home-iso-'))
  try {
    withTmpDir((tmp) => {
      generateCredentialGuardScript(tmp)
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
      injectCursorHooks(tmp)

      const settings = JSON.parse(fs.readFileSync(path.join(tmp, '.cursor', 'hooks.json'), 'utf8'))
      const readEntry = settings.hooks.preToolUse.find(e => e.command === 'scripts/trackfw-credential-guard.sh' && e.matcher === 'Read')
      assert.ok(readEntry, 'preToolUse deveria ter entrada com matcher Read apontando para o script')
      const writeEntry = settings.hooks.preToolUse.find(e => e.command === 'scripts/trackfw-credential-guard.sh' && e.matcher === 'Write')
      assert.ok(writeEntry, 'preToolUse deveria ter entrada com matcher Write apontando para o script')

      const SYNTHETIC_AWS_KEY = 'AKIAABCDEFGHIJKLMNOP'
      const writePayload = JSON.stringify({ tool_name: 'Write', tool_input: { file_path: '/tmp/y', content: `key=${SYNTHETIC_AWS_KEY}` } })
      const { code, stderr } = runScript(tmp, writePayload)
      assert.strictEqual(code, 0, 'modo warn (default) não bloqueia')
      assert.ok(stderr.includes('AWS'))
    })
  } finally {
    process.env.HOME = origHome
  }
})

// (c) Comando Bash referenciando arquivo com segredo por caminho, sem o segredo literal no
// comando (ex.: cat /tmp/fixture-com-jwt.txt, head -c 50 arquivo) → script captura via segunda
// camada de detecção (scan_file_for_pattern em CG_DETECTION_CORE).
test('(c) "cat arquivo.txt" sem JWT literal no comando é capturado pela segunda camada de detecção', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const fixture = path.join(tmp, 'fixture-com-jwt.txt')
    fs.writeFileSync(fixture, `access_token=${SYNTHETIC_JWT}\n`, 'utf8')

    const { code, stderr } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `cat ${fixture}` } }))
    assert.strictEqual(code, 0, 'modo warn (default) não bloqueia')
    assert.ok(stderr.includes('JWT'), 'deveria detectar o JWT dentro do arquivo referenciado por cat')
    assert.ok(attentionFileExists(tmp))
  })
})

test('(c) "head -c 50 arquivo.txt" sem AWS key literal no comando é capturado pela segunda camada, modo block bloqueia', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'credential_guard:\n  mode: block\n', 'utf8')
    const SYNTHETIC_AWS_KEY = 'AKIAABCDEFGHIJKLMNOP'
    const fixture = path.join(tmp, 'fixture-com-aws-key.txt')
    fs.writeFileSync(fixture, `aws_access_key_id=${SYNTHETIC_AWS_KEY}\n`, 'utf8')

    const { code, stderr } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `head -c 50 ${fixture}` } }))
    assert.strictEqual(code, 2, 'modo block deve interromper a ação')
    assert.ok(stderr.includes('AWS'), 'deveria detectar a AWS key dentro do arquivo referenciado por head')
    assert.ok(!attentionFileExists(tmp), 'modo block não escreve attention.json')
  })
})

test('(c) comando "cat" que NÃO referencia arquivo com segredo continua no-op (sem falso positivo)', () => {
  withTmpDir((tmp) => {
    generateCredentialGuardScript(tmp)
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
    const fixture = path.join(tmp, 'arquivo-comum.txt')
    fs.writeFileSync(fixture, 'nada de sensivel aqui\n', 'utf8')

    const { code } = runScript(tmp, JSON.stringify({ tool_name: 'Bash', tool_input: { command: `cat ${fixture}` } }))
    assert.strictEqual(code, 0)
    assert.ok(!attentionFileExists(tmp))
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
