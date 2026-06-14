'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { resolveReqFiles } = require('../src/validator/index.js')
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

// --- Teste 1: resolveReqFiles flat ---

test('resolveReqFiles flat: retorna path completo do arquivo .md na raiz do reqDir', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'req-flat-'))
  try {
    const reqDir = path.join(tmp, 'req')
    writeFile(path.join(reqDir, 'REQ-001.md'), '# REQ 001\n')

    const files = resolveReqFiles({ reqDir })

    assert.strictEqual(files.length, 1, `Esperava 1 arquivo, got ${files.length}`)
    assert.strictEqual(files[0], path.join(reqDir, 'REQ-001.md'))
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// --- Teste 2: resolveReqFiles by_agent ---

test('resolveReqFiles by_agent: retorna path completo de REQ em subdir agente/estado/', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'req-by-agent-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const reqFile = path.join(reqDir, 'claude', 'wip', 'REQ-002.md')
    writeFile(reqFile, '# REQ 002\n')

    const files = resolveReqFiles({ reqDir, roadmap_namespacing: 'by_agent' })

    assert(files.length >= 1, `Esperava ao menos 1 arquivo, got ${files.length}`)
    assert(
      files.some(f => f === reqFile),
      `Esperava ${reqFile} na lista. Got: ${JSON.stringify(files)}`
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// --- Teste 3: resolveReqFiles by_agent cobre todos os estados ---

test('resolveReqFiles by_agent: cobre todos os estados (backlog, wip, blocked, done, abandoned)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'req-states-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const STATES = ['backlog', 'wip', 'blocked', 'done', 'abandoned']
    for (const state of STATES) {
      writeFile(path.join(reqDir, 'zeus', state, `REQ-${state}.md`), `# REQ ${state}\n`)
    }

    const files = resolveReqFiles({ reqDir, roadmap_namespacing: 'by_agent' })

    assert.strictEqual(
      files.length, STATES.length,
      `Esperava ${STATES.length} arquivos (um por estado), got ${files.length}: ${JSON.stringify(files)}`
    )
    for (const state of STATES) {
      assert(
        files.some(f => f.includes(state)),
        `Esperava arquivo no estado "${state}". Files: ${JSON.stringify(files)}`
      )
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// --- Teste 4: salvaguarda one-sided (Roadmaps indexados, REQs vazias) ---

test('salvaguarda one-sided: Roadmap indexado + reqDir vazio → traceid_config_warning com "REQs (0)"', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'req-onesided-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')

    // reqDir existe mas está vazio (sem .md com req_id)
    fs.mkdirSync(reqDir, { recursive: true })

    // Roadmap com req_id mas sem REQ correspondente
    writeFile(
      path.join(roadmapDir, 'claude', 'wip', 'rm.md'),
      '---\nreq_id: RID-1\n---\n# Roadmap Claude WIP\n'
    )

    const violations = checkTraceIds(reqDir, roadmapDir, 'req_id')

    assert(
      violations.some(v => v.includes('traceid_config_warning') && v.includes('REQs (0)')),
      `Esperava traceid_config_warning com "REQs (0)". Violations: ${JSON.stringify(violations)}`
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
