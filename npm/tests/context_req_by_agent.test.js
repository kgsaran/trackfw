'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

function writeFile(filePath, content) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, content, 'utf8')
}

// 🔴 Este arquivo já duplicou a lógica de coleta do context.js e, por isso, ficou VÁCUO: continuou
// verde enquanto a produção divergia. Agora chama a função de produção (collectReqEntries), que lê
// pelo ponto único resolveReqFiles (ADR-2026-09-03, D3/D4).
const { collectReqEntries: collectReqs } = require('../src/commands/context')

// --- Teste 1: by_agent encontra REQ em subdir agente/estado ---

test('by_agent: encontra REQ em subdir claude/wip/', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ctx-req-by-agent-'))
  try {
    const reqDir = path.join(tmp, 'req')
    writeFile(path.join(reqDir, 'claude', 'wip', 'req.md'), '---\nstatus: Open\n---\n# REQ\n')

    const reqs = collectReqs({ reqDir, roadmap_namespacing: 'by_agent', adrDirs: [] })

    assert.strictEqual(reqs.length, 1, `Esperava 1 REQ, got ${reqs.length}`)
    assert.strictEqual(reqs[0].file, 'req.md')
    assert.strictEqual(reqs[0].state, 'wip')
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// --- Teste 2: flat sem by_agent (sem regressão) ---

test('flat: encontra REQ na raiz de reqDir (sem by_agent)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ctx-req-flat-'))
  try {
    const reqDir = path.join(tmp, 'req')
    writeFile(path.join(reqDir, 'req.md'), '---\nstatus: Open\n---\n# REQ\n')

    const reqs = collectReqs({ reqDir, adrDirs: [] })

    assert.strictEqual(reqs.length, 1, `Esperava 1 REQ, got ${reqs.length}`)
    assert.strictEqual(reqs[0].file, 'req.md')
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// --- Teste 3: by_agent encontra REQ no layout CANÔNICO agente/*.md (ADR-2026-09-03, D2) ---

test('by_agent: encontra REQ no canônico claude/*.md (sem pasta de estado)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ctx-req-canonico-'))
  try {
    const reqDir = path.join(tmp, 'req')
    writeFile(path.join(reqDir, 'claude', 'req.md'), '---\nstatus: Open\n---\n# REQ\n')

    const reqs = collectReqs({ reqDir, roadmap_namespacing: 'by_agent', agents: ['claude'], adrDirs: [] })

    assert.strictEqual(reqs.length, 1, `Esperava 1 REQ, got ${reqs.length}`)
    assert.strictEqual(reqs[0].file, 'req.md')
    assert.strictEqual(reqs[0].state, undefined, 'REQ não tem dimensão de estado (invariante D1)')
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
