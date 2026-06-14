'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const config = require('../src/config')
const validator = require('../src/validator')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

function writeFile(filePath, content) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, content, 'utf8')
}

// Cria uma estrutura mínima de projeto em tmp e retorna o caminho raiz.
// reqDir, roadmapDir e adrDir são criados dentro de tmp.
function setupProject(tmp, { reqFiles = [], blockedFiles = [], adrFiles = [], rules = '' } = {}) {
  const reqDir = path.join(tmp, 'docs', 'req')
  const roadmapDir = path.join(tmp, 'docs', 'roadmaps')
  const adrDir = path.join(tmp, 'docs', 'adr')

  fs.mkdirSync(reqDir, { recursive: true })
  fs.mkdirSync(path.join(roadmapDir, 'blocked'), { recursive: true })
  fs.mkdirSync(adrDir, { recursive: true })

  for (const [name, content] of reqFiles) {
    writeFile(path.join(reqDir, name), content)
  }
  for (const [name, content] of blockedFiles) {
    writeFile(path.join(roadmapDir, 'blocked', name), content)
  }
  for (const [name, content] of adrFiles) {
    writeFile(path.join(adrDir, name), content)
  }

  let yamlContent = `req_dir: ${reqDir}\nroadmap_dir: ${roadmapDir}\nadr_dirs:\n  - ${adrDir}\n`
  if (rules) {
    yamlContent += `rules:\n${rules}`
  }
  writeFile(path.join(tmp, 'trackfw.yaml'), yamlContent)
  return tmp
}

// REQ sem ADR (viola req_has_adr e req_has_roadmap)
const REQ_SEM_ADR_E_ROADMAP = '# REQ-001\n\nDescrição sem ADR nem Roadmap.\n'
// REQ com ADR mas sem Roadmap (viola apenas req_has_roadmap)
const REQ_COM_ADR_SEM_ROADMAP = '# REQ-001\n\nADR: ADR-001.md\n'
// REQ com ADR e com Roadmap (não viola nada)
const REQ_COMPLETO = '# REQ-001\n\nADR: ADR-001.md\nRoadmap: docs/roadmaps/wip/RM-001.md\n'
// Roadmap bloqueado sem REQ (viola blocked_has_req)
const ROADMAP_BLOCKED_SEM_REQ = '# Roadmap bloqueado\n\nSem link de REQ.\n'

// =========================================================
// req_has_adr
// =========================================================

test('req_has_adr warning: REQ sem ADR → warning, não violation', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-adr-warn-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      reqFiles: [['REQ-001.md', REQ_SEM_ADR_E_ROADMAP]],
      rules: '  req_has_adr: warning\n  req_has_roadmap: off\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateREQsHaveADR()
    const violations = []
    const warnings = []
    validator.applyRule('req_has_adr', msgs, violations, warnings)

    assert.strictEqual(warnings.length, 1, `Esperava 1 warning, got ${warnings.length}`)
    assert.strictEqual(violations.length, 0, `Esperava 0 violations, got ${violations.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('req_has_adr off: REQ sem ADR → silenciado (nem violation nem warning)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-adr-off-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      reqFiles: [['REQ-001.md', REQ_SEM_ADR_E_ROADMAP]],
      rules: '  req_has_adr: off\n  req_has_roadmap: off\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateREQsHaveADR()
    const violations = []
    const warnings = []
    validator.applyRule('req_has_adr', msgs, violations, warnings)

    assert.strictEqual(violations.length, 0, `Esperava 0 violations, got ${violations.length}`)
    assert.strictEqual(warnings.length, 0, `Esperava 0 warnings, got ${warnings.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('req_has_adr default (error): REQ sem ADR → violation', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-adr-err-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      reqFiles: [['REQ-001.md', REQ_SEM_ADR_E_ROADMAP]],
      rules: '  req_has_roadmap: off\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateREQsHaveADR()
    const violations = []
    const warnings = []
    validator.applyRule('req_has_adr', msgs, violations, warnings)

    assert(violations.length >= 1, `Esperava ao menos 1 violation, got ${violations.length}`)
    assert.strictEqual(warnings.length, 0, `Esperava 0 warnings, got ${warnings.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// =========================================================
// blocked_has_req
// =========================================================

test('blocked_has_req warning: roadmap blocked sem REQ → warning, não violation', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-blocked-warn-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      blockedFiles: [['RM-001.md', ROADMAP_BLOCKED_SEM_REQ]],
      rules: '  blocked_has_req: warning\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateBlockedHasREQ()
    const violations = []
    const warnings = []
    validator.applyRule('blocked_has_req', msgs, violations, warnings)

    assert.strictEqual(warnings.length, 1, `Esperava 1 warning, got ${warnings.length}`)
    assert.strictEqual(violations.length, 0, `Esperava 0 violations, got ${violations.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('blocked_has_req off: roadmap blocked sem REQ → silenciado', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-blocked-off-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      blockedFiles: [['RM-001.md', ROADMAP_BLOCKED_SEM_REQ]],
      rules: '  blocked_has_req: off\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateBlockedHasREQ()
    const violations = []
    const warnings = []
    validator.applyRule('blocked_has_req', msgs, violations, warnings)

    assert.strictEqual(violations.length, 0, `Esperava 0 violations, got ${violations.length}`)
    assert.strictEqual(warnings.length, 0, `Esperava 0 warnings, got ${warnings.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('blocked_has_req default (error): roadmap blocked sem REQ → violation', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-blocked-err-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      blockedFiles: [['RM-001.md', ROADMAP_BLOCKED_SEM_REQ]],
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateBlockedHasREQ()
    const violations = []
    const warnings = []
    validator.applyRule('blocked_has_req', msgs, violations, warnings)

    assert(violations.length >= 1, `Esperava ao menos 1 violation, got ${violations.length}`)
    assert.strictEqual(warnings.length, 0, `Esperava 0 warnings, got ${warnings.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// =========================================================
// req_has_roadmap
// =========================================================

test('req_has_roadmap warning: REQ sem Roadmap → warning, não violation', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-rm-warn-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      reqFiles: [['REQ-001.md', REQ_COM_ADR_SEM_ROADMAP]],
      rules: '  req_has_roadmap: warning\n  req_has_adr: off\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateREQsHaveRoadmap()
    const violations = []
    const warnings = []
    validator.applyRule('req_has_roadmap', msgs, violations, warnings)

    assert.strictEqual(warnings.length, 1, `Esperava 1 warning, got ${warnings.length}`)
    assert.strictEqual(violations.length, 0, `Esperava 0 violations, got ${violations.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('req_has_roadmap off: REQ sem Roadmap → silenciado', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-rm-off-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      reqFiles: [['REQ-001.md', REQ_COM_ADR_SEM_ROADMAP]],
      rules: '  req_has_roadmap: off\n  req_has_adr: off\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateREQsHaveRoadmap()
    const violations = []
    const warnings = []
    validator.applyRule('req_has_roadmap', msgs, violations, warnings)

    assert.strictEqual(violations.length, 0, `Esperava 0 violations, got ${violations.length}`)
    assert.strictEqual(warnings.length, 0, `Esperava 0 warnings, got ${warnings.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('req_has_roadmap default (error): REQ sem Roadmap → violation', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tfw-rm-err-'))
  const origCwd = process.cwd()
  try {
    setupProject(tmp, {
      reqFiles: [['REQ-001.md', REQ_COM_ADR_SEM_ROADMAP]],
      rules: '  req_has_adr: off\n',
    })
    process.chdir(tmp)
    config.reset()

    const msgs = validator.validateREQsHaveRoadmap()
    const violations = []
    const warnings = []
    validator.applyRule('req_has_roadmap', msgs, violations, warnings)

    assert(violations.length >= 1, `Esperava ao menos 1 violation, got ${violations.length}`)
    assert.strictEqual(warnings.length, 0, `Esperava 0 warnings, got ${warnings.length}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
