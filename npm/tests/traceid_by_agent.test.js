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

function writeFile(filePath, content) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, content, 'utf8')
}

// --- Testes do layout by_agent (roadmaps/<agente>/<estado>/) ---

test('by_agent: REQ plana + Roadmap em subdir de agente sem par → orphan em ambos os lados', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'traceid-by-agent-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')

    // REQ na raiz do reqDir (sem subdir de estado) com req_id sem roadmap correspondente
    writeFile(
      path.join(reqDir, 'REQ-001.md'),
      '---\nreq_id: orphan-001\n---\n# REQ 001\n'
    )

    // Roadmap em subdir de agente (layout by_agent) com req_id sem REQ correspondente
    writeFile(
      path.join(roadmapDir, 'claude', 'wip', 'rm.md'),
      '---\nreq_id: orphan-002\n---\n# Roadmap Claude WIP\n'
    )

    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')

    assert(
      violations.some(v => v.includes('traceid_orphan_req') && v.includes('orphan-001')),
      `Esperava traceid_orphan_req para orphan-001. Violations: ${JSON.stringify(violations)}`
    )
    assert(
      violations.some(v => v.includes('traceid_orphan_roadmap') && v.includes('orphan-002')),
      `Esperava traceid_orphan_roadmap para orphan-002. Violations: ${JSON.stringify(violations)}`
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('by_agent: par REQ + Roadmap válido em layout by_agent → sem violations traceid', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'traceid-by-agent-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')

    // REQ dentro de subdir de estado
    writeFile(
      path.join(reqDir, 'wip', 'REQ-010.md'),
      '---\nreq_id: REQ-010\n---\n# REQ 010\n'
    )

    // Roadmap em subdir de agente no mesmo estado (by_agent layout)
    writeFile(
      path.join(roadmapDir, 'apolo', 'wip', 'roadmap-010.md'),
      '---\nreq_id: REQ-010\n---\n# Roadmap Apolo WIP\n'
    )

    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')
    const traceViolations = violations.filter(v => v.startsWith('traceid_'))

    assert.strictEqual(
      traceViolations.length, 0,
      `Esperava 0 violations traceid para par válido by_agent. Violations: ${JSON.stringify(violations)}`
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('by_agent: state mismatch — REQ em done/, Roadmap em wip/ via by_agent → traceid_state_mismatch', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'traceid-by-agent-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')

    writeFile(
      path.join(reqDir, 'done', 'REQ-020.md'),
      '---\nreq_id: REQ-020\n---\n# REQ 020\n'
    )

    // Roadmap em agente "zeus", estado "wip" — deve resultar em state_mismatch com done
    writeFile(
      path.join(roadmapDir, 'zeus', 'wip', 'roadmap-020.md'),
      '---\nreq_id: REQ-020\n---\n# Roadmap Zeus WIP\n'
    )

    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')

    assert(
      violations.some(v => v.includes('traceid_state_mismatch') && v.includes('REQ-020')),
      `Esperava traceid_state_mismatch para REQ-020. Violations: ${JSON.stringify(violations)}`
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// --- Teste da salvaguarda zero-entradas ---

test('salvaguarda: dirs vazios com fieldName definido → traceid_config_warning', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'traceid-by-agent-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')
    fs.mkdirSync(reqDir, { recursive: true })
    fs.mkdirSync(roadmapDir, { recursive: true })

    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')

    assert(
      violations.some(v => v.includes('traceid_config_warning')),
      `Esperava traceid_config_warning para dirs vazios. Violations: ${JSON.stringify(violations)}`
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('salvaguarda: dirs com .md sem req_id no frontmatter → traceid_config_warning', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'traceid-by-agent-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')

    // Arquivo .md mas sem o campo req_id — não deve ser indexado
    writeFile(
      path.join(reqDir, 'wip', 'REQ-sem-id.md'),
      '---\ntitle: Sem ID\n---\n# Sem req_id\n'
    )
    writeFile(
      path.join(roadmapDir, 'claude', 'wip', 'rm-sem-id.md'),
      '---\ntitle: Roadmap sem ID\n---\n# Roadmap sem req_id\n'
    )

    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')

    assert(
      violations.some(v => v.includes('traceid_config_warning')),
      `Esperava traceid_config_warning quando nenhum arquivo tem req_id. Violations: ${JSON.stringify(violations)}`
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
