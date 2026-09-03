'use strict'
/**
 * context_cli.test.js — executa o BINÁRIO `npm/bin/trackfw context` como processo.
 *
 * Por que processo e não import com mock: o defeito que este teste existe para
 * pegar (`const { violations, warnings } = validate()` sem `await`, com
 * `validate` sendo `async function`) mora na FRONTEIRA entre o comando e o
 * validator — dentro de nenhum dos dois. Um teste que importasse `context.js` e
 * mockasse `validate` com uma função síncrona não reproduziria o defeito, que
 * sobreviveu desde a origem do pacote justamente por não haver teste que
 * rodasse o CLI. Portanto: sem mock de `validate`, sem stub do validator.
 *
 * ML-1A · ROADMAP-2026-09-02-context-do-cli-node-aguarda-validate-e-ganha-teste-que-executa-o-binario
 */
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')

const BIN = path.join(__dirname, '..', 'bin', 'trackfw')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

/**
 * makeProject — projeto de governança mínimo e determinístico em tmpdir, para
 * que as asserções não dependam do estado do repositório real.
 * @returns {string} caminho do projeto
 */
function makeProject() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ctx-cli-'))
  fs.writeFileSync(
    path.join(dir, 'trackfw.yaml'),
    'adr_dirs:\n  - docs/adr\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\n',
    'utf8'
  )
  for (const sub of ['docs/adr', 'docs/req', 'docs/roadmaps/wip', 'docs/roadmaps/backlog']) {
    fs.mkdirSync(path.join(dir, sub), { recursive: true })
  }
  fs.writeFileSync(
    path.join(dir, 'docs/adr/ADR-2026-09-02-teste.md'),
    '---\nstatus: Accepted\n---\n\n# ADR de teste\n',
    'utf8'
  )
  return dir
}

/**
 * runContext — executa o binário no projeto e devolve status/stdout/stderr.
 * @param {string} cwd
 * @param {string[]} args
 */
function runContext(cwd, args) {
  return spawnSync(process.execPath, [BIN, 'context', ...args], {
    cwd,
    encoding: 'utf8',
    // HOME isolado: o validate() consulta regras de credential-guard em escopo
    // GLOBAL ($HOME). Sem isolar, o resultado depende da máquina do dev e da
    // do runner — classe de flake já registrada em
    // vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md.
    env: { ...process.env, HOME: cwd, USERPROFILE: cwd },
  })
}

// --- Teste 1: o binário roda e imprime o contexto (formato md) ---

test('binário: `trackfw context` sai 0 e imprime o contexto em markdown', () => {
  const dir = makeProject()
  try {
    const r = runContext(dir, [])
    const out = `${r.stdout}${r.stderr}`

    // Oráculo do defeito de origem: a Promise desestruturada dava
    // "Cannot read properties of undefined (reading 'length')" em
    // `violations.length`, com exit 1 e stdout vazio.
    assert.ok(
      !out.includes('Cannot read properties of undefined'),
      `saída contém o erro do validate sem await:\n${out}`
    )
    assert.strictEqual(r.status, 0, `esperava exit 0, got ${r.status}. Saída:\n${out}`)
    assert.ok(
      r.stdout.includes('# trackfw governance context'),
      `stdout sem o cabeçalho do contexto:\n${out}`
    )
    assert.ok(
      r.stdout.includes('**Governance score:**'),
      `stdout sem a linha de score:\n${out}`
    )
    assert.ok(r.stdout.includes('## ADRs (1)'), `stdout sem a seção de ADRs:\n${out}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// --- Teste 2: formato json, com oráculo estrutural ---

test('binário: `trackfw context --format json` sai 0 e emite JSON com violations/warnings', () => {
  const dir = makeProject()
  try {
    const r = runContext(dir, ['--format', 'json'])
    const out = `${r.stdout}${r.stderr}`

    assert.ok(
      !out.includes('Cannot read properties of undefined'),
      `saída contém o erro do validate sem await:\n${out}`
    )
    assert.strictEqual(r.status, 0, `esperava exit 0, got ${r.status}. Saída:\n${out}`)

    let parsed
    try { parsed = JSON.parse(r.stdout) }
    catch (e) { throw new Error(`stdout não é JSON válido (${e.message}):\n${out}`) }

    // O score depende de `violations.length` — a mesma leitura que estourava.
    assert.strictEqual(typeof parsed.score, 'number', 'score deve ser número')
    assert.ok(Array.isArray(parsed.violations), 'violations deve ser array (não undefined)')
    assert.ok(Array.isArray(parsed.warnings), 'warnings deve ser array (não undefined)')
    assert.ok(Array.isArray(parsed.adrs), 'adrs deve ser array')
    assert.strictEqual(parsed.adrs.length, 1, `esperava 1 ADR, got ${parsed.adrs.length}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// --- Teste 3: o exit code do binário reflete o comando, não só a impressão ---

test('binário: nenhuma unhandled rejection escapa do comando context', () => {
  const dir = makeProject()
  try {
    const r = runContext(dir, [])
    assert.ok(
      !r.stderr.includes('UnhandledPromiseRejection') &&
      !r.stderr.includes('unhandled') &&
      !r.stderr.includes('Error:'),
      `stderr deveria estar limpo:\n${r.stderr}`
    )
    assert.strictEqual(r.status, 0, `esperava exit 0, got ${r.status}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
