'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { checkTraceIds } = require('../src/validator/traceid.js')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

// Cria estrutura de dirs temporários e retorna { tmp, reqDir, roadmapDir }
function setup() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-traceid-'))
  const reqDir = path.join(tmp, 'req')
  const roadmapDir = path.join(tmp, 'roadmaps')
  fs.mkdirSync(reqDir, { recursive: true })
  for (const state of ['wip', 'backlog', 'done', 'blocked', 'abandoned']) {
    fs.mkdirSync(path.join(reqDir, state), { recursive: true })
    fs.mkdirSync(path.join(roadmapDir, state), { recursive: true })
  }
  return { tmp, reqDir, roadmapDir }
}

function teardown(tmp) {
  fs.rmSync(tmp, { recursive: true, force: true })
}

function writeFile(filePath, content) {
  fs.writeFileSync(filePath, content, 'utf8')
}

// --- testes ---

test('Roadmap com req_id sem REQ correspondente → traceid_orphan_roadmap', () => {
  const { tmp, reqDir, roadmapDir } = setup()
  try {
    writeFile(
      path.join(roadmapDir, 'wip', 'roadmap-a.md'),
      '---\nreq_id: REQ-001\nstatus: wip\n---\n# Roadmap A\n'
    )
    // nenhuma REQ com REQ-001
    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')
    assert(
      violations.some(v => v.includes('traceid_orphan_roadmap') && v.includes('REQ-001')),
      `Esperava traceid_orphan_roadmap para REQ-001. Violations: ${JSON.stringify(violations)}`
    )
  } finally { teardown(tmp) }
})

test('REQ com req_id sem Roadmap correspondente → traceid_orphan_req', () => {
  const { tmp, reqDir, roadmapDir } = setup()
  try {
    writeFile(
      path.join(reqDir, 'wip', 'REQ-002.md'),
      '---\nreq_id: REQ-002\nstatus: wip\n---\n# REQ 002\n'
    )
    // nenhum Roadmap com REQ-002
    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')
    assert(
      violations.some(v => v.includes('traceid_orphan_req') && v.includes('REQ-002')),
      `Esperava traceid_orphan_req para REQ-002. Violations: ${JSON.stringify(violations)}`
    )
  } finally { teardown(tmp) }
})

test('REQ em done/, Roadmap em wip/, mesmo req_id → traceid_state_mismatch', () => {
  const { tmp, reqDir, roadmapDir } = setup()
  try {
    writeFile(
      path.join(reqDir, 'done', 'REQ-003.md'),
      '---\nreq_id: REQ-003\nstatus: done\n---\n# REQ 003\n'
    )
    writeFile(
      path.join(roadmapDir, 'wip', 'roadmap-003.md'),
      '---\nreq_id: REQ-003\nstatus: wip\n---\n# Roadmap 003\n'
    )
    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')
    assert(
      violations.some(v => v.includes('traceid_state_mismatch') && v.includes('REQ-003')),
      `Esperava traceid_state_mismatch para REQ-003. Violations: ${JSON.stringify(violations)}`
    )
  } finally { teardown(tmp) }
})

test('2 REQs com mesmo req_id → traceid_duplicate_req', () => {
  const { tmp, reqDir, roadmapDir } = setup()
  try {
    writeFile(
      path.join(reqDir, 'wip', 'REQ-004a.md'),
      '---\nreq_id: REQ-004\nstatus: wip\n---\n# REQ 004a\n'
    )
    writeFile(
      path.join(reqDir, 'wip', 'REQ-004b.md'),
      '---\nreq_id: REQ-004\nstatus: wip\n---\n# REQ 004b\n'
    )
    // Adiciona roadmap correspondente para o mesmo req_id para isolar a violation de duplicate
    writeFile(
      path.join(roadmapDir, 'wip', 'roadmap-004.md'),
      '---\nreq_id: REQ-004\nstatus: wip\n---\n# Roadmap 004\n'
    )
    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')
    assert(
      violations.some(v => v.includes('traceid_duplicate_req') && v.includes('REQ-004')),
      `Esperava traceid_duplicate_req para REQ-004. Violations: ${JSON.stringify(violations)}`
    )
  } finally { teardown(tmp) }
})

test('Par válido REQ e Roadmap no mesmo estado → sem violations traceid', () => {
  const { tmp, reqDir, roadmapDir } = setup()
  try {
    writeFile(
      path.join(reqDir, 'wip', 'REQ-005.md'),
      '---\nreq_id: REQ-005\nstatus: wip\n---\n# REQ 005\n'
    )
    writeFile(
      path.join(roadmapDir, 'wip', 'roadmap-005.md'),
      '---\nreq_id: REQ-005\nstatus: wip\n---\n# Roadmap 005\n'
    )
    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')
    const traceViolations = violations.filter(v => v.startsWith('traceid_'))
    assert.strictEqual(
      traceViolations.length, 0,
      `Esperava 0 violations traceid para par válido. Violations: ${JSON.stringify(violations)}`
    )
  } finally { teardown(tmp) }
})

test('Sem fieldName (traceIdField vazio) → nenhuma verificação executada', () => {
  const { tmp, reqDir, roadmapDir } = setup()
  try {
    // Arquivo com req_id — mas fieldName está em branco
    writeFile(
      path.join(roadmapDir, 'wip', 'roadmap-x.md'),
      '---\nreq_id: REQ-999\nstatus: wip\n---\n# Roadmap X\n'
    )
    const violations = checkTraceIds(reqDir, roadmapDir, '')
    assert.strictEqual(violations.length, 0, 'Sem fieldName deve retornar array vazio')
  } finally { teardown(tmp) }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
