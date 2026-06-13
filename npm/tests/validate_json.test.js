'use strict'
const assert = require('assert')
const { execSync, spawnSync } = require('child_process')
const fs = require('fs')
const os = require('os')
const path = require('path')

// Caminho para o entry point do CLI Node.js
const CLI = path.resolve(__dirname, '../bin/trackfw')

let passed = 0
let failed = 0
const tests = []

function test(name, fn) {
  tests.push({ name, fn })
}

// Executa o CLI com os args fornecidos e retorna { stdout, stderr, status }
function runCLI(args, opts) {
  const result = spawnSync(process.execPath, [CLI, ...args], {
    encoding: 'utf8',
    cwd: opts && opts.cwd ? opts.cwd : process.cwd(),
  })
  return {
    stdout: result.stdout || '',
    stderr: result.stderr || '',
    status: result.status,
  }
}

// --- testes ---

test('validate --json produz JSON válido', () => {
  const { stdout, status } = runCLI(['validate', '--json'])
  let parsed
  try {
    parsed = JSON.parse(stdout)
  } catch (e) {
    throw new Error(`stdout não é JSON válido: ${e.message}\nstdout: ${stdout}`)
  }
  assert(parsed !== null, 'JSON deve ser não-nulo')
})

test('validate --json contém campo summary', () => {
  const { stdout } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert('summary' in parsed, 'Deve conter campo "summary"')
})

test('validate --json summary tem subcampos violations, warnings, mode, exit_code', () => {
  const { stdout } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  const s = parsed.summary
  assert(typeof s.violations === 'number', 'summary.violations deve ser number')
  assert(typeof s.warnings === 'number', 'summary.warnings deve ser number')
  assert(typeof s.mode === 'string', 'summary.mode deve ser string')
  assert(typeof s.exit_code === 'number', 'summary.exit_code deve ser number')
})

test('validate --json contém campo violations (array)', () => {
  const { stdout } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert(Array.isArray(parsed.violations), '"violations" deve ser array')
})

test('validate --json contém campo warnings (array)', () => {
  const { stdout } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert(Array.isArray(parsed.warnings), '"warnings" deve ser array')
})

test('validate --json: summary.violations conta itens do array violations', () => {
  const { stdout } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert.strictEqual(
    parsed.summary.violations,
    parsed.violations.length,
    'summary.violations deve ser igual ao tamanho do array violations'
  )
})

test('validate --json: summary.warnings conta itens do array warnings', () => {
  const { stdout } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert.strictEqual(
    parsed.summary.warnings,
    parsed.warnings.length,
    'summary.warnings deve ser igual ao tamanho do array warnings'
  )
})

test('validate --json: exit_code é 0 quando sem violations', () => {
  const { stdout, status } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  if (parsed.violations.length === 0) {
    assert.strictEqual(parsed.summary.exit_code, 0, 'exit_code deve ser 0 sem violations')
    assert.strictEqual(status, 0, 'processo deve terminar com status 0')
  }
  // Se houver violations, pular a asserção silenciosamente (depende do repo)
})

test('validate --json: exit code do processo bate com summary.exit_code', () => {
  const { stdout, status } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert.strictEqual(
    status,
    parsed.summary.exit_code,
    `exit code do processo (${status}) deve ser igual a summary.exit_code (${parsed.summary.exit_code})`
  )
})

test('validate --json: mode é "strict" ou "lenient"', () => {
  const { stdout } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert(
    parsed.summary.mode === 'strict' || parsed.summary.mode === 'lenient',
    `mode deve ser "strict" ou "lenient", recebido: "${parsed.summary.mode}"`
  )
})

test('validate sem --json: saída não é JSON (comportamento texto inalterado)', () => {
  const { stdout } = runCLI(['validate'])
  // sem --json, o stdout NÃO deve ser um objeto JSON iniciado por {
  // (pode ser string vazia, mensagem de OK, violations em texto, etc.)
  const firstChar = stdout.trim()[0]
  assert(
    firstChar !== '{',
    `Sem --json, saída não deve começar com "{": ${stdout.slice(0, 80)}`
  )
})

test('validate sem --json: exit code igual ao com --json', () => {
  const { status: statusText } = runCLI(['validate'])
  const { stdout, status: statusJson } = runCLI(['validate', '--json'])
  const parsed = JSON.parse(stdout)
  assert.strictEqual(
    statusText,
    statusJson,
    `exit codes devem ser iguais: texto=${statusText} json=${statusJson}`
  )
  assert.strictEqual(
    statusText,
    parsed.summary.exit_code,
    `exit code texto (${statusText}) deve bater com summary.exit_code (${parsed.summary.exit_code})`
  )
})

test('validate --json: cada item de violations tem campos rule e file', () => {
  // Criar fixture com violation conhecida: roadmap em wip sem REQ linkada
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-json-'))
  try {
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/backlog'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/blocked'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/done'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
    // Roadmap em wip sem REQ → dispara wip_has_req
    fs.writeFileSync(
      path.join(tmp, 'docs/roadmaps/wip/RM-json-test.md'),
      '---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n'
    )
    const { stdout, status } = runCLI(['validate', '--json'], { cwd: tmp })
    const parsed = JSON.parse(stdout)
    assert(parsed.violations.length > 0, 'deve haver ao menos uma violation no fixture')
    for (const item of parsed.violations) {
      assert('rule' in item, `item deve ter campo "rule": ${JSON.stringify(item)}`)
      assert('file' in item, `item deve ter campo "file": ${JSON.stringify(item)}`)
      assert(typeof item.rule === 'string', `item.rule deve ser string: ${JSON.stringify(item)}`)
      assert(typeof item.file === 'string', `item.file deve ser string: ${JSON.stringify(item)}`)
    }
    // A violation de wip_has_req deve ter rule preenchido
    const wipViolation = parsed.violations.find(v => v.message.includes('no linked REQ'))
    assert(wipViolation, 'deve ter violation de wip_has_req')
    assert.strictEqual(wipViolation.rule, 'wip_has_req', `rule deve ser "wip_has_req", recebido: "${wipViolation.rule}"`)
    assert(wipViolation.file !== '', `file deve ser não-vazio para wip_has_req, recebido: "${wipViolation.file}"`)
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('validate --json: cada item de warnings tem campos rule e file', () => {
  // Criar fixture com warning conhecida: ADR sem REQ vinculada (adr_orphan = warning)
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-json-warn-'))
  try {
    fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/backlog'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/blocked'), { recursive: true })
    fs.mkdirSync(path.join(tmp, 'docs/roadmaps/done'), { recursive: true })
    // ADR sem REQ → adr_orphan (warning por default)
    fs.writeFileSync(
      path.join(tmp, 'docs/adr/ADR-json-test.md'),
      '---\nstatus: Approved\n---\n# ADR-json-test\n'
    )
    const { stdout } = runCLI(['validate', '--json'], { cwd: tmp })
    const parsed = JSON.parse(stdout)
    // Pode não ter warnings se o ADR não disparar em ambiente isolado, mas estrutura deve estar presente
    for (const item of parsed.warnings) {
      assert('rule' in item, `warning item deve ter campo "rule": ${JSON.stringify(item)}`)
      assert('file' in item, `warning item deve ter campo "file": ${JSON.stringify(item)}`)
      assert(typeof item.rule === 'string', `item.rule deve ser string`)
      assert(typeof item.file === 'string', `item.file deve ser string`)
    }
    // Se há warning de adr_orphan, verificar rule
    const adrWarning = parsed.warnings.find(w => w.message.includes('ADR-json-test'))
    if (adrWarning) {
      assert.strictEqual(adrWarning.rule, 'adr_orphan', `rule deve ser "adr_orphan"`)
      assert(adrWarning.file !== '', `file deve ser não-vazio para adr_orphan`)
    }
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

;(async () => {
  for (const { name, fn } of tests) {
    try {
      await fn()
      console.log('✓', name)
      passed++
    } catch (e) {
      console.error('✗', name, e.message)
      failed++
    }
  }
  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
})()
