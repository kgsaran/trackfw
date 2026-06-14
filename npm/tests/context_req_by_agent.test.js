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

/**
 * Lógica de coleta de REQs extraída do context.js para ser testável sem invocar process.exit.
 * Espelha exatamente o código de context.js.
 */
function collectEntries(dir, type, state) {
  const entries = []
  let files = []
  try {
    files = fs.readdirSync(dir).filter(f => f.endsWith('.md') && !fs.statSync(path.join(dir, f)).isDirectory())
  } catch (_) {
    return entries
  }
  for (const file of files) {
    let content = ''
    try { content = fs.readFileSync(path.join(dir, file), 'utf8') } catch (_) {}
    const entry = { type, file, status: state || 'unknown' }
    if (state) entry.state = state
    entries.push(entry)
  }
  return entries
}

function collectReqs(cfg) {
  const reqs = []
  const reqDir = cfg.reqDir || cfg.req_dir || 'docs/req'
  const reqNamespacing = cfg.roadmapNamespacing || cfg.roadmap_namespacing || ''
  if (reqNamespacing === 'by_agent') {
    const STATES = ['backlog', 'wip', 'blocked', 'done', 'abandoned']
    let agents = cfg.agents || []
    if (!agents.length) {
      try {
        agents = fs.readdirSync(reqDir).filter(f => {
          try { return fs.statSync(path.join(reqDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) {}
    }
    for (const agent of agents) {
      for (const state of STATES) {
        reqs.push(...collectEntries(path.join(reqDir, agent, state), 'REQ', state))
      }
    }
  } else {
    reqs.push(...collectEntries(reqDir, 'REQ'))
  }
  return reqs
}

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

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
