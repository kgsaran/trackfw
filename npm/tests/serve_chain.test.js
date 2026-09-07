'use strict'

const assert = require('assert')
const fs = require('fs')
const path = require('path')
const os = require('os')

const { handleChain } = require('../src/serve/api_chain')

let passed = 0, failed = 0
const tests = []

function test(name, fn) {
  tests.push({ name, fn })
}

function mkdirp(p) {
  fs.mkdirSync(p, { recursive: true })
}

function writeFile(p, content) {
  mkdirp(path.dirname(p))
  fs.writeFileSync(p, content, 'utf8')
}

function mockRes() {
  const r = { statusCode: null, headers: {}, body: '' }
  r.writeHead = (code, headers) => { r.statusCode = code; Object.assign(r.headers, headers || {}) }
  r.end = (data) => { r.body += (data || '') }
  return r
}

// ML-3D: reconciliação — cada teste afirma qual conclusão deste ML ele mede.

// AFIRMAÇÃO: o formato canônico gravado por `trackfw req new` (frontmatter `roadmap: ""`
// sempre vazio, valor real em "## Linked Roadmap / Roadmap: <path>" no corpo) produz a aresta
// REQ→Roadmap no /api/chain. Antes do ML-3D, o parser lia só o bloco de frontmatter e a aresta
// nunca era desenhada para NENHUMA REQ gerada pelo próprio CLI — não era caso de borda de um
// vínculo específico, era o formato canônico inteiro nunca resolvendo.
test('api_chain — REQ com vínculo Roadmap só no corpo (formato canônico do gerador) produz aresta', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-chain-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')
    const wipDir = path.join(roadmapDir, 'wip')
    writeFile(path.join(wipDir, 'ROADMAP-canon.md'), '# Roadmap canonico\n')

    const roadmapRef = path.join(wipDir, 'ROADMAP-canon.md')
    const reqContent = [
      '---',
      'status: Open',
      'adr: ""',
      'roadmap: ""',
      '---',
      '# REQ canonica',
      '',
      '## Linked Roadmap',
      `Roadmap: ${roadmapRef}`,
      '',
    ].join('\n')
    writeFile(path.join(reqDir, 'REQ-canon.md'), reqContent)

    const cfg = { adrDirs: [path.join(tmp, 'adr')], reqDir, roadmapDir, roadmapNamespacing: 'flat' }
    const res = mockRes()
    handleChain(cfg, {}, res)
    const body = JSON.parse(res.body)

    const roadmapNode = body.nodes.find(n => n.type === 'roadmap')
    assert.ok(roadmapNode, `nó do roadmap não encontrado; nodes: ${JSON.stringify(body.nodes)}`)

    const found = body.edges.some(e => e.to === roadmapNode.id)
    assert.ok(found, `aresta REQ->Roadmap não encontrada; edges: ${JSON.stringify(body.edges)}`)
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

// AFIRMAÇÃO: um vínculo `Roadmap:` cujo basename não existe em estado algum não produz aresta
// para node nenhum — guarda de vacuidade, não inventa nó.
test('api_chain — vínculo Roadmap para basename inexistente não inventa nó', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-chain-'))
  try {
    const reqDir = path.join(tmp, 'req')
    const roadmapDir = path.join(tmp, 'roadmaps')
    const wipDir = path.join(roadmapDir, 'wip')
    mkdirp(wipDir)

    const missingRef = path.join(wipDir, 'ROADMAP-nunca-existiu.md')
    const reqContent = [
      '---',
      'status: Open',
      'adr: ""',
      'roadmap: ""',
      '---',
      '# REQ orfa',
      '',
      '## Linked Roadmap',
      `Roadmap: ${missingRef}`,
      '',
    ].join('\n')
    writeFile(path.join(reqDir, 'REQ-orfa.md'), reqContent)

    const cfg = { adrDirs: [path.join(tmp, 'adr')], reqDir, roadmapDir, roadmapNamespacing: 'flat' }
    const res = mockRes()
    handleChain(cfg, {}, res)
    const body = JSON.parse(res.body)

    const nodeIds = new Set(body.nodes.map(n => n.id))
    // Nenhuma aresta pode apontar para um id que não é node real (guarda de vacuidade).
    for (const e of body.edges) {
      assert.ok(nodeIds.has(e.to), `aresta aponta para nó inventado: ${e.to}`)
    }
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

;(async () => {
  for (const { name, fn } of tests) {
    try {
      await fn()
      console.log('v', name)
      passed++
    } catch (e) {
      console.error('x', name, e.message)
      failed++
    }
  }
  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
})()
