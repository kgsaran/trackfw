'use strict'

// fatal-error.test.js — Handler global de erro no entrypoint Node
// (npm/bin/trackfw). Prova que um erro não tratado que escapa de QUALQUER
// action() do commander — síncrono ou promise rejeitada via parseAsync() —
// produz mensagem limpa em stderr + exit != 0, sem stack trace, sem caminho
// absoluto de instalação, sem "Node.js vX" — e que TRACKFW_DEBUG=1 restaura
// a stack completa.
//
// REQ: docs/req/REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-stack-trace-
// caminhos-absolutos-e-versao-do-runtime.md

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')
const { Command } = require('commander')

const { reportFatalError } = require('../src/lib/fatal-error')

const CLI = path.resolve(__dirname, '../bin/trackfw')

function runCLI(args, options = {}) {
  return spawnSync(process.execPath, [CLI, ...args], { encoding: 'utf8', ...options })
}

// ---------------------------------------------------------------------------
// reportFatalError — unidade
// ---------------------------------------------------------------------------

// reportFatalError usa fs.writeSync(2, ...) — não process.stderr.write() —
// deliberadamente (ver comentário em fatal-error.js: process.exit() logo
// depois pode descartar uma escrita assíncrona pendente quando stderr é um
// pipe). Os testes de unidade interceptam fs.writeSync, não
// process.stderr.write, para exercitar o caminho real.
function withMockedWriteSync(fn) {
  const originalWriteSync = fs.writeSync
  const chunks = []
  fs.writeSync = (fd, chunk) => {
    if (fd === 2) { chunks.push(chunk); return chunk.length }
    return originalWriteSync(fd, chunk)
  }
  try {
    fn()
  } finally {
    fs.writeSync = originalWriteSync
  }
  return chunks.join('')
}

test('reportFatalError sem TRACKFW_DEBUG imprime só "Error: <mensagem>" em stderr (fd 2, síncrono)', () => {
  const originalDebug = process.env.TRACKFW_DEBUG
  delete process.env.TRACKFW_DEBUG
  let out
  try {
    out = withMockedWriteSync(() => reportFatalError(new Error('boom: /Users/someone/project/lib.js:42')))
  } finally {
    if (originalDebug !== undefined) process.env.TRACKFW_DEBUG = originalDebug
  }
  assert.strictEqual(out, 'Error: boom: /Users/someone/project/lib.js:42\n')
  assert.doesNotMatch(out, /at /)
})

test('reportFatalError com TRACKFW_DEBUG=1 imprime a stack completa (mensagem só uma vez)', () => {
  const originalDebug = process.env.TRACKFW_DEBUG
  process.env.TRACKFW_DEBUG = '1'
  let out
  try {
    out = withMockedWriteSync(() => reportFatalError(new Error('boom with stack')))
  } finally {
    if (originalDebug === undefined) delete process.env.TRACKFW_DEBUG
    else process.env.TRACKFW_DEBUG = originalDebug
  }
  const occurrences = out.split('boom with stack').length - 1
  assert.strictEqual(occurrences, 1, `mensagem duplicada na saída de debug: "${out}"`)
  assert.match(out, /^Error: boom with stack\n {4}at /)
})

// Mensagem multi-linha sobrevive íntegra — não só a mensagem de domínio real
// (que hoje é de uma linha só), mas qualquer mensagem futura com múltiplas
// linhas (ex.: "Adopt it with: ..." citado no ML). Trava contra uma futura
// "sanitização" que quebrasse/truncasse a mensagem na primeira quebra de linha.
test('reportFatalError preserva mensagem multi-linha íntegra, sem truncar na primeira quebra de linha', () => {
  const originalDebug = process.env.TRACKFW_DEBUG
  delete process.env.TRACKFW_DEBUG
  let out
  try {
    out = withMockedWriteSync(() => reportFatalError(new Error('line one\nAdopt it with: trackfw agents install --force')))
  } finally {
    if (originalDebug !== undefined) process.env.TRACKFW_DEBUG = originalDebug
  }
  assert.strictEqual(out, 'Error: line one\nAdopt it with: trackfw agents install --force\n')
})

// ---------------------------------------------------------------------------
// Rejeição assíncrona — o entrypoint usa parseAsync(); um throw DEPOIS de um
// await dentro de uma action() chega como promise rejeitada, não como throw
// síncrono. Prova isolada, sem depender de nenhum subcomando real.
// ---------------------------------------------------------------------------

test('erro lançado depois de um await dentro de action() é capturado (rejeição assíncrona)', async () => {
  const program = new Command()
  program.exitOverride()
  program.command('boom').action(async () => {
    await new Promise((resolve) => setTimeout(resolve, 5))
    throw new Error('late async failure')
  })

  let caught = null
  await program.parseAsync(['node', 'trackfw', 'boom']).catch((err) => { caught = err })

  assert.ok(caught instanceof Error, 'a rejeição da promise deveria propagar até o .catch() do entrypoint')
  assert.strictEqual(caught.message, 'late async failure')
})

// ---------------------------------------------------------------------------
// installGlobalHandlers() — a rede de segurança FORA da promise chain do
// parseAsync().catch() (ex.: promise "solta" que nenhum caller aguarda).
// Único caminho sem teste que dependesse de fs.writeSync ser síncrono: os
// dois listeners chamam process.exit() logo após reportFatalError(), e
// stderr aqui é literalmente um PIPE (spawnSync) — a configuração exata em
// que process.stderr.write() (assíncrono) poderia perder a escrita para
// process.exit(). Repetido várias vezes para não confiar num único run.
// ---------------------------------------------------------------------------

test('unhandledRejection (promise solta, fora do parseAsync) imprime mensagem íntegra em stderr via pipe, exit != 0', () => {
  const script = "require('" + path.resolve(__dirname, '../src/lib/fatal-error').replace(/\\/g, '\\\\') +
    "').installGlobalHandlers(); Promise.reject(new Error('floating-boom'))"
  for (let i = 0; i < 20; i++) {
    const result = spawnSync(process.execPath, ['-e', script], { encoding: 'utf8' })
    assert.notStrictEqual(result.status, 0, `iteração ${i}: exit code deve ser != 0`)
    assert.strictEqual(result.stderr, 'Error: floating-boom\n', `iteração ${i}: mensagem incompleta/perdida (stderr é um pipe): "${result.stderr}"`)
  }
})

// ---------------------------------------------------------------------------
// trackfw agents update --force sobre artefato unmanaged — o vazamento medido
// na REQ. Fixture real via bin/trackfw, não simulação.
// ---------------------------------------------------------------------------

function unmanagedArtifactRepro() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-fatal-error-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot)
  fs.mkdirSync(homeRoot)
  const env = { ...process.env, HOME: homeRoot }

  const install = runCLI(
    ['agents', 'install', '--scope', 'project', '--targets', 'codex', '--items', 'iac'],
    { cwd: projectRoot, env }
  )
  assert.strictEqual(install.status, 0, install.stderr)

  const manifestPath = path.join(projectRoot, '.trackfw/integrations-manifest.json')
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
  const key = Object.keys(manifest.artifacts).find((k) => k.includes('iac'))
  delete manifest.artifacts[key]
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2))
  fs.appendFileSync(path.join(projectRoot, '.codex/agents/trackfw-iac.toml'), '\n# tampered\n')

  return { projectRoot, env }
}

test('agents update --force sobre artefato unmanaged: sem stack, sem caminho de instalação, sem versão do runtime, exit != 0', () => {
  const { projectRoot, env } = unmanagedArtifactRepro()
  const result = runCLI(
    ['agents', 'update', '--force', '--scope', 'project', '--targets', 'codex', '--items', 'iac'],
    { cwd: projectRoot, env }
  )

  assert.notStrictEqual(result.status, 0, 'exit code deve ser != 0')
  const stderr = result.stderr || ''
  assert.match(stderr, /^Error: /, `mensagem deve começar com "Error: ", obteve: "${stderr}"`)
  // AC1: nada de stack, caminho de instalação do trackfw, linha de fonte ou versão do runtime.
  assert.doesNotMatch(stderr, /\n {4}at /, `stderr não deve conter frames de stack: "${stderr}"`)
  assert.doesNotMatch(stderr, /npm[/\\]src[/\\]/, `stderr não deve vazar o caminho de instalação: "${stderr}"`)
  assert.doesNotMatch(stderr, /Node\.js v\d/, `stderr não deve vazar a versão do runtime: "${stderr}"`)
  // A mensagem de domínio (existente, inalterada) permanece — inclusive o
  // caminho absoluto do ARTEFATO adulterado, que não é o caminho de
  // instalação do trackfw e não faz parte do escopo deste ML.
  assert.match(stderr, /does not match a trackfw template/)
})

test('mesmo cenário com TRACKFW_DEBUG=1: stack completa presente', () => {
  const { projectRoot, env } = unmanagedArtifactRepro()
  const result = runCLI(
    ['agents', 'update', '--force', '--scope', 'project', '--targets', 'codex', '--items', 'iac'],
    { cwd: projectRoot, env: { ...env, TRACKFW_DEBUG: '1' } }
  )

  assert.notStrictEqual(result.status, 0)
  const stderr = result.stderr || ''
  assert.match(stderr, /\n {4}at /, `TRACKFW_DEBUG=1 deveria restaurar a stack, obteve: "${stderr}"`)
  assert.match(stderr, /npm[/\\]src[/\\]integrations[/\\]manager\.js/)
})

// ---------------------------------------------------------------------------
// Não-regressão — caminhos já limpos hoje permanecem byte-idênticos.
// ---------------------------------------------------------------------------

test('trackfw roadmap move <inexistente> permanece byte-idêntico (não passou a ter "Error: " prefixado)', () => {
  const result = runCLI(['roadmap', 'move', 'nao-existe-xyz', 'wip'])
  assert.strictEqual(result.status, 1)
  assert.strictEqual((result.stderr || '').trim(), 'roadmap "nao-existe-xyz" not found in any state directory')
})

test('trackfw comando-inexistente permanece byte-idêntico', () => {
  const result = runCLI(['comando-inexistente-xyz'])
  assert.strictEqual(result.status, 1)
  assert.strictEqual(
    (result.stderr || '').trim(),
    'Error: unknown command "comando-inexistente-xyz" for "trackfw"\n' +
      "Run 'trackfw --help' for usage."
  )
})
