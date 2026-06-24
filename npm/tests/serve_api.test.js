'use strict'

const assert = require('assert')
const fs = require('fs')
const path = require('path')
const os = require('os')

const { handleBoard } = require('../src/serve/api_board')
const { handleFile } = require('../src/serve/api_file')
const { handleMetrics } = require('../src/serve/api_metrics')
const { getAttention } = require('../src/serve/api_attention')

let passed = 0, failed = 0
const tests = []

function test(name, fn) {
  tests.push({ name, fn })
}

// Helpers

function mkdirp(p) {
  fs.mkdirSync(p, { recursive: true })
}

function writeFile(p, content) {
  mkdirp(path.dirname(p))
  fs.writeFileSync(p, content, 'utf8')
}

/**
 * Cria um objeto res mock que captura status e body.
 */
function mockRes() {
  const r = { statusCode: null, headers: {}, body: '' }
  r.writeHead = (code, headers) => { r.statusCode = code; Object.assign(r.headers, headers || {}) }
  r.end = (data) => { r.body += (data || '') }
  return r
}

/**
 * Cria um objeto req mock com url opcional.
 */
function mockReq(url) {
  return { url: url || '/' }
}

// ─── api_board ────────────────────────────────────────────────────────────────

test('api_board — flat mode retorna columns e agents', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-board-'))
  try {
    // Criar 2 roadmaps em estados diferentes
    writeFile(path.join(tmp, 'wip', 'ROADMAP-001.md'), '# Roadmap Em Progresso\n')
    writeFile(path.join(tmp, 'backlog', 'ROADMAP-002.md'), '# Roadmap No Backlog\n')

    const cfg = { roadmapDir: tmp, roadmapNamespacing: 'flat' }
    const res = mockRes()
    handleBoard(cfg, mockReq('/api/board'), res)

    assert.strictEqual(res.statusCode, 200, 'deve retornar 200')
    const data = JSON.parse(res.body)
    assert(data.columns, 'deve ter campo columns')
    assert(Array.isArray(data.columns.wip), 'columns.wip deve ser array')
    assert(Array.isArray(data.columns.backlog), 'columns.backlog deve ser array')
    assert.strictEqual(data.columns.wip.length, 1, 'deve ter 1 item em wip')
    assert.strictEqual(data.columns.backlog.length, 1, 'deve ter 1 item em backlog')
    assert.strictEqual(data.columns.wip[0].title, 'Roadmap Em Progresso', 'titulo deve vir do h1')
    assert(Array.isArray(data.agents), 'deve ter campo agents (array)')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('api_board — by_agent mode inclui agent no card', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-board-'))
  try {
    // Estrutura: rootDir/agente/estado/arquivo.md
    writeFile(path.join(tmp, 'apolo', 'wip', 'ROADMAP-A.md'), '# Roadmap Apolo WIP\n')
    writeFile(path.join(tmp, 'artemis', 'done', 'ROADMAP-B.md'), '# Roadmap Artemis Done\n')

    const cfg = { roadmapDir: tmp, roadmapNamespacing: 'by_agent' }
    const res = mockRes()
    handleBoard(cfg, mockReq('/api/board'), res)

    assert.strictEqual(res.statusCode, 200, 'deve retornar 200')
    const data = JSON.parse(res.body)
    assert(Array.isArray(data.agents), 'deve ter campo agents')
    assert(data.agents.includes('apolo'), 'agents deve incluir apolo')
    assert(data.agents.includes('artemis'), 'agents deve incluir artemis')

    const wipCard = data.columns.wip.find(c => c.file === 'ROADMAP-A.md')
    assert(wipCard, 'deve ter card ROADMAP-A.md em wip')
    assert.strictEqual(wipCard.agent, 'apolo', 'agent do card deve ser apolo')

    const doneCard = data.columns.done.find(c => c.file === 'ROADMAP-B.md')
    assert(doneCard, 'deve ter card ROADMAP-B.md em done')
    assert.strictEqual(doneCard.agent, 'artemis', 'agent do card deve ser artemis')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('api_board — board vazio retorna columns vazias sem erro', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-board-'))
  try {
    const cfg = { roadmapDir: tmp, roadmapNamespacing: 'flat' }
    const res = mockRes()
    handleBoard(cfg, mockReq('/api/board'), res)

    assert.strictEqual(res.statusCode, 200, 'deve retornar 200 mesmo sem arquivos')
    const data = JSON.parse(res.body)
    assert.strictEqual(data.columns.wip.length, 0, 'wip deve estar vazio')
    assert.strictEqual(data.columns.backlog.length, 0, 'backlog deve estar vazio')
    assert.strictEqual(data.columns.done.length, 0, 'done deve estar vazio')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

// ─── api_file ─────────────────────────────────────────────────────────────────

test('api_file — path valido retorna 200 com conteudo', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-file-'))
  try {
    const roadmapDir = path.join(tmp, 'roadmaps')
    mkdirp(roadmapDir)
    const filePath = path.join(roadmapDir, 'ROADMAP-001.md')
    writeFile(filePath, '# Roadmap Teste\nConteudo do arquivo.\n')

    const cfg = { roadmapDir }
    const req = mockReq('/api/file?path=' + encodeURIComponent(filePath))
    req.url = '/api/file?path=' + filePath  // handleFile usa req.url diretamente
    const res = mockRes()
    handleFile(cfg, req, res)

    assert.strictEqual(res.statusCode, 200, 'deve retornar 200')
    assert(res.body.includes('Roadmap Teste'), 'corpo deve ter o conteudo do arquivo')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('api_file — path traversal retorna 403', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-file-'))
  try {
    const roadmapDir = path.join(tmp, 'roadmaps')
    mkdirp(roadmapDir)

    const cfg = { roadmapDir }
    const req = mockReq()
    req.url = '/api/file?path=../../../etc/passwd'
    const res = mockRes()
    handleFile(cfg, req, res)

    assert.strictEqual(res.statusCode, 403, 'path traversal deve retornar 403')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('api_file — path fora dos dirs permitidos retorna 403', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-file-'))
  try {
    const roadmapDir = path.join(tmp, 'roadmaps')
    mkdirp(roadmapDir)

    // Criar arquivo fora dos dirs permitidos
    const outsideFile = path.join(tmp, 'secret.txt')
    fs.writeFileSync(outsideFile, 'segredo', 'utf8')

    const cfg = { roadmapDir }
    const req = mockReq()
    req.url = '/api/file?path=' + outsideFile
    const res = mockRes()
    handleFile(cfg, req, res)

    assert.strictEqual(res.statusCode, 403, 'arquivo fora dos dirs permitidos deve retornar 403')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

// ─── api_metrics ──────────────────────────────────────────────────────────────

test('api_metrics — sem trackfw-log retorna zeros sem erro', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-metrics-'))
  try {
    const roadmapDir = path.join(tmp, 'roadmaps')
    mkdirp(roadmapDir)
    // Nao criar .trackfw-log

    const cfg = { roadmapDir }
    const res = mockRes()
    handleMetrics(cfg, mockReq('/api/metrics'), res)

    assert.strictEqual(res.statusCode, 200, 'deve retornar 200 sem log')
    const data = JSON.parse(res.body)
    assert.strictEqual(data.cycle_time_avg_days, 0, 'cycle_time_avg_days deve ser 0 sem log')
    assert.strictEqual(data.lead_time_avg_days, 0, 'lead_time_avg_days deve ser 0 sem log')
    assert(Array.isArray(data.burndown), 'burndown deve ser array')
    assert.strictEqual(data.burndown.length, 0, 'burndown deve ser vazio sem log')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('api_metrics — com log valido calcula cycle_time_avg_days', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-metrics-'))
  try {
    const roadmapDir = path.join(tmp, 'roadmaps')
    mkdirp(roadmapDir)

    // Criar .trackfw-log com transicoes para um roadmap
    // calculate() usa a PRIMEIRA entrada em backlog ou wip como startTs
    // Formato: YYYY-MM-DD HH:MM  basename  from → to
    // backlog em 2026-01-01, done em 2026-01-06 = 5 dias
    const logContent = [
      '2026-01-01 10:00  ROADMAP-001.md  created → backlog',
      '2026-01-03 10:00  ROADMAP-001.md  backlog → wip',
      '2026-01-06 10:00  ROADMAP-001.md  wip → done',
    ].join('\n') + '\n'

    const logPath = path.join(roadmapDir, '.trackfw-log')
    fs.writeFileSync(logPath, logContent, 'utf8')

    const cfg = { roadmapDir }
    const res = mockRes()
    handleMetrics(cfg, mockReq('/api/metrics'), res)

    assert.strictEqual(res.statusCode, 200, 'deve retornar 200 com log')
    const data = JSON.parse(res.body)
    assert(data.cycle_time_avg_days > 0, 'cycle_time_avg_days deve ser positivo com log valido')
    // startTs = 2026-01-01 (primeiro to=backlog), doneTs = 2026-01-06 = 5 dias
    assert.strictEqual(data.cycle_time_avg_days, 5, 'cycle_time deve ser 5 dias')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('api_attention — ausente retorna inactive', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-attention-'))
  try {
    const cfg = { roadmapDir: path.join(tmp, 'roadmaps') }
    assert.deepStrictEqual(getAttention(cfg), { active: false })
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

test('api_attention — arquivo valido retorna active', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-attention-'))
  try {
    const roadmapDir = path.join(tmp, 'roadmaps')
    mkdirp(roadmapDir)
    fs.writeFileSync(
      path.join(roadmapDir, '.trackfw-attention.json'),
      JSON.stringify({ message: 'Review required', timestamp: '2026-06-24T12:00:00Z' })
    )
    const result = getAttention({ roadmapDir })
    assert.strictEqual(result.active, true)
    assert.strictEqual(result.message, 'Review required')
  } finally {
    fs.rmSync(tmp, { recursive: true })
  }
})

// ─── Runner ──────────────────────────────────────────────────────────────────

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
