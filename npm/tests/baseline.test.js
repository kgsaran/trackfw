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
