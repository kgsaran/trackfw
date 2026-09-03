'use strict'
/**
 * req_layout_union.test.js — contrato do resolvedor de REQ (ADR-2026-09-03):
 * leitura é UNIÃO dos 4 layouts, escrita é ÚNICA, e o diretório de escrita está contido na união
 * por construção (D2/D3/D4). Espelha internal/validator/validator_req_layout_test.go e
 * pypi/tests/test_req_layout_union.py.
 */
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { resolveReqFiles, reqWriteDir } = require('../src/validator')

let passed = 0, failed = 0
function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

function writeREQ(filePath) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, '---\nstatus: Open\ndate: 2026-09-03\n---\n\n# REQ: fixture\n', 'utf8')
}

function withTmp(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'req-layout-union-'))
  try { fn(tmp) } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
}

test('by_agent: os 4 layouts são lidos JUNTOS, nunca em exclusão mútua', () => {
  withTmp((tmp) => {
    const reqDir = path.join(tmp, 'docs', 'req')
    writeREQ(path.join(reqDir, 'REQ-flat.md'))                     // (1) flat legado
    writeREQ(path.join(reqDir, 'backlog', 'REQ-estado.md'))        // (2) por-estado legado
    writeREQ(path.join(reqDir, 'claude', 'REQ-canonico.md'))       // (3) CANÔNICO
    writeREQ(path.join(reqDir, 'claude', 'wip', 'REQ-legado.md'))  // (4) legado

    const got = resolveReqFiles({ reqDir, roadmapNamespacing: 'by_agent', agents: ['claude'] })
      .map(p => path.basename(p)).sort()
    assert.deepStrictEqual(got, ['REQ-canonico.md', 'REQ-estado.md', 'REQ-flat.md', 'REQ-legado.md'])
  })
})

test('by_agent: <estado>/ e <agente>/ colidem e a REQ é contada UMA vez (dedup)', () => {
  withTmp((tmp) => {
    const reqDir = path.join(tmp, 'docs', 'req')
    writeREQ(path.join(reqDir, 'backlog', 'REQ-uma-so.md'))
    const got = resolveReqFiles({ reqDir, roadmapNamespacing: 'by_agent', agents: ['claude'] })
    assert.strictEqual(got.length, 1, `esperado 1 arquivo deduplicado, got ${JSON.stringify(got)}`)
  })
})

test('D4: REQ criada em reqWriteDir é encontrada por resolveReqFiles (flat e by_agent)', () => {
  for (const [ns, suffix] of [['', 'docs/req'], ['by_agent', path.join('docs', 'req', 'claude')]]) {
    withTmp((tmp) => {
      const cfg = { reqDir: path.join(tmp, 'docs', 'req'), roadmapNamespacing: ns, agents: ['claude'] }
      const dir = reqWriteDir(cfg)
      assert.strictEqual(dir, path.join(tmp, suffix), `reqWriteDir divergiu em ns=${ns || 'flat'}`)
      writeREQ(path.join(dir, 'REQ-nova.md'))
      const found = resolveReqFiles(cfg).some(p => path.basename(p) === 'REQ-nova.md')
      assert.ok(found, `REQ criada em ${dir} não foi encontrada pelo resolvedor`)
    })
  }
})

test('D4: by_agent SEM agents: declarada — REQ em default/ volta pelo DISCO', () => {
  withTmp((tmp) => {
    const cfg = { reqDir: path.join(tmp, 'docs', 'req'), roadmapNamespacing: 'by_agent' }
    const dir = reqWriteDir(cfg)
    writeREQ(path.join(dir, 'REQ-nova.md'))
    const got = resolveReqFiles(cfg).map(p => path.basename(p))
    assert.deepStrictEqual(got, ['REQ-nova.md'], `esperado achar a REQ criada em ${dir}`)
  })
})

test('reqWriteDir: sem agents: declarados, o namespace de escrita é "default"', () => {
  assert.strictEqual(
    reqWriteDir({ reqDir: 'docs/req', roadmapNamespacing: 'by_agent' }),
    path.join('docs/req', 'default')
  )
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
