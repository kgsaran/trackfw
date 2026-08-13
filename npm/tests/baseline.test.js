'use strict'
const assert = require('assert')
const fs = require('fs')
const path = require('path')
const os = require('os')
const config = require('../src/config')
const validator = require('../src/validator')

let passed = 0, failed = 0
const tests = []

function test(name, fn) {
  tests.push({ name, fn })
}

function mkdirs(base, dirs) {
  for (const d of dirs) fs.mkdirSync(path.join(base, d), { recursive: true })
}

function writeFile(base, rel, content) {
  const full = path.join(base, rel)
  fs.mkdirSync(path.dirname(full), { recursive: true })
  fs.writeFileSync(full, content)
}

test('saveBaseline cria .trackfw-baseline.json', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bl-'))
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    validator.saveBaseline(['violation 1'], ['warning 1'])
    const data = JSON.parse(fs.readFileSync('.trackfw-baseline.json', 'utf8'))
    assert.deepStrictEqual(data.violations, ['violation 1'])
    assert.deepStrictEqual(data.warnings, ['warning 1'])
    assert(data.created, 'deve ter campo created')
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('loadBaseline retorna null se arquivo não existe', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bl-'))
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.loadBaseline()
    assert.strictEqual(result, null)
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('validate filtra violations do baseline', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bl-'))
  mkdirs(tmp, ['docs/adr', 'docs/req', 'docs/roadmaps/wip',
    'docs/roadmaps/backlog', 'docs/roadmaps/blocked', 'docs/roadmaps/done'])
  // roadmap em wip sem REQ → violation
  writeFile(tmp, 'docs/roadmaps/wip/RM-001.md',
    '---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    // Salvar baseline com a violation atual
    const raw = await validator.validateUnfiltered()
    validator.saveBaseline(raw.violations, raw.warnings)

    // validate() deve filtrar a violation do baseline
    const result = await validator.validate()
    assert(!result.violations.some(v => v.includes('RM-001')),
      'violations do baseline devem ser filtradas: ' + JSON.stringify(result.violations))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('validate reporta violations novas (não no baseline)', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bl-'))
  mkdirs(tmp, ['docs/adr', 'docs/req', 'docs/roadmaps/wip',
    'docs/roadmaps/backlog', 'docs/roadmaps/blocked', 'docs/roadmaps/done'])
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    // Baseline vazio
    validator.saveBaseline([], [])

    // Nova violation
    writeFile(tmp, 'docs/roadmaps/wip/RM-002.md',
      '---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n')

    const result = await validator.validate()
    assert(result.violations.some(v => v.includes('RM-002')),
      'nova violation deve aparecer: ' + JSON.stringify(result.violations))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('baseline filtra warnings baselined', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bl-'))
  mkdirs(tmp, ['docs/adr', 'docs/req', 'docs/roadmaps/wip',
    'docs/roadmaps/backlog', 'docs/roadmaps/blocked', 'docs/roadmaps/done'])
  // ADR sem REQ vinculada → gera adr_orphan como warning (severidade padrão)
  writeFile(tmp, 'docs/adr/ADR-001.md', '---\nstatus: Approved\n---\n# ADR-001\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    // Capturar warnings reais
    const raw = await validator.validateUnfiltered()
    assert(raw.warnings.some(w => w.includes('ADR-001')),
      'ADR-001 deve gerar um warning adr_orphan: ' + JSON.stringify(raw.warnings))

    // Salvar baseline com os warnings atuais
    validator.saveBaseline(raw.violations, raw.warnings)

    // validate() deve filtrar os warnings do baseline
    const result = await validator.validate()
    assert(!result.warnings.some(w => w.includes('ADR-001')),
      'warnings do baseline devem ser filtrados: ' + JSON.stringify(result.warnings))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('lenient + baseline: warning baselined não reaparece', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bl-'))
  mkdirs(tmp, ['docs/adr', 'docs/req', 'docs/roadmaps/wip',
    'docs/roadmaps/backlog', 'docs/roadmaps/blocked', 'docs/roadmaps/done'])
  // ADR sem REQ vinculada → gera adr_orphan como warning
  writeFile(tmp, 'docs/adr/ADR-002.md', '---\nstatus: Approved\n---\n# ADR-002\n')
  // Ativar modo lenient sem data de expiração (sempre ativo)
  writeFile(tmp, 'trackfw.yaml', 'governance_mode: lenient\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    // Capturar warnings reais e salvar baseline
    const raw = await validator.validateUnfiltered()
    assert(raw.warnings.some(w => w.includes('ADR-002')),
      'ADR-002 deve gerar um warning: ' + JSON.stringify(raw.warnings))
    validator.saveBaseline(raw.violations, raw.warnings)

    // Em modo lenient + baseline, o warning baselined não deve reaparecer
    const result = await validator.validate()
    assert(!result.warnings.some(w => w.includes('ADR-002')),
      'warning baselined não deve reaparecer em modo lenient: ' + JSON.stringify(result.warnings))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard,
// ADR-2026-08-12-severidade-das-regras-de-credential-guard-...: carve-out do baseline — violations
// das 3 regras de credential-guard continuam sendo reportadas por validate() mesmo depois de
// "baselined" — .trackfw-baseline.json não é um canal válido para tolerá-las (ao contrário de
// qualquer outra regra, provado acima por "validate filtra violations do baseline").
test('baseline NÃO tolera violation de credential_guard_mode_downgrade', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bl-cg-'))
  const { execFileSync } = require('child_process')
  const git = (...args) => execFileSync('git', args, { cwd: tmp, stdio: ['ignore', 'pipe', 'pipe'] })
  git('init')
  git('config', 'user.email', 'test@test.com')
  git('config', 'user.name', 'test')
  writeFile(tmp, 'trackfw.yaml', 'credential_guard:\n  mode: block\n')
  git('add', 'trackfw.yaml')
  git('commit', '-m', 'trackfw.yaml')
  fs.rmSync(path.join(tmp, 'trackfw.yaml')) // HEAD: block; disco: deletado → dispara a regra
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const raw = await validator.validateUnfiltered()
    assert(raw.violations.some(v => v.includes('credential_guard.mode: block')),
      'esperado violation de credential_guard_mode_downgrade antes do baseline: ' + JSON.stringify(raw.violations))

    // Tentar tolerar via baseline — como qualquer outra violation seria.
    validator.saveBaseline(raw.violations, raw.warnings)

    const result = await validator.validate()
    assert(result.violations.some(v => v.includes('credential_guard.mode: block')),
      'violation de credential_guard_mode_downgrade NÃO deveria ser tolerável via baseline: ' + JSON.stringify(result.violations))
  } finally {
    process.chdir(origDir)
    config.reset()
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
