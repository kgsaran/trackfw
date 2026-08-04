'use strict'
/**
 * req_list_move_subfolders.test.js — cobre o ML-1B (Node.js) do
 * ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md: descoberta recursiva de REQs
 * nos 3 layouts (flat, por-estado, by_agent) e move físico condicional.
 * Espelha os 6 casos de referência do ML-1A (Go).
 */
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { listREQFiles, listREQs, findREQ, moveREQ } = require('../src/generators/req')
const config = require('../src/config')

let passed = 0, failed = 0
function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

function writeFile(filePath, content) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, content, 'utf8')
}

function reqBody(status) {
  return `---\nstatus: ${status}\ndate: 2026-08-04\n---\n\n# REQ: Fixture\n\n> Date: 2026-08-04 | Status: ${status}\n`
}

function withTmpProject(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-req-sub-'))
  const orig = process.cwd()
  try {
    process.chdir(tmp)
    config.reset()
    fn(tmp)
  } finally {
    process.chdir(orig)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

// --- 1. listREQFiles / listREQs por-estado (sem agente) ---
test('listREQFiles: descobre REQ em docs/req/backlog/ (layout por-estado)', () => {
  withTmpProject(() => {
    const cfg = config.load()
    writeFile(path.join(cfg.reqDir, 'backlog', 'REQ-2026-08-04-x.md'), reqBody('backlog'))

    const files = listREQFiles(cfg)

    assert.strictEqual(files.length, 1, `Esperava 1 arquivo, got ${files.length}`)
    assert.strictEqual(files[0], path.join(cfg.reqDir, 'backlog', 'REQ-2026-08-04-x.md'))
  })
})

// --- 2. listREQFiles by_agent ---
test('listREQFiles: descobre REQ em docs/req/claude/wip/ (layout by_agent)', () => {
  withTmpProject(() => {
    writeFile(path.join(process.cwd(), 'trackfw.yaml'), 'roadmap_namespacing: by_agent\nagents:\n  - claude\n')
    const cfg = config.load()
    writeFile(path.join(cfg.reqDir, 'claude', 'wip', 'REQ-2026-08-04-y.md'), reqBody('wip'))

    const files = listREQFiles(cfg)

    assert.strictEqual(files.length, 1, `Esperava 1 arquivo, got ${files.length}`)
    assert.strictEqual(files[0], path.join(cfg.reqDir, 'claude', 'wip', 'REQ-2026-08-04-y.md'))
  })
})

// --- 3. findREQ recursivo ---
test('findREQ: encontra REQ em subpasta de estado (recursivo, case-insensitive)', () => {
  withTmpProject(() => {
    const cfg = config.load()
    const target = path.join(cfg.reqDir, 'wip', 'REQ-2026-08-04-fechar-conta.md')
    writeFile(target, reqBody('wip'))

    const found = findREQ('fechar', cfg)

    assert.strictEqual(found, target)
  })
})

// --- 4. moveREQ move físico — layout por-estado ---
test('moveREQ: move fisicamente o arquivo quando já está em docs/req/<estado>/', () => {
  withTmpProject(() => {
    const reqDir = path.join(process.cwd(), 'docs', 'req')
    const src = path.join(reqDir, 'wip', 'REQ-2026-08-04-fisico.md')
    writeFile(src, reqBody('wip'))

    moveREQ('fisico', 'done')

    const dst = path.join(reqDir, 'done', 'REQ-2026-08-04-fisico.md')
    assert(fs.existsSync(dst), 'REQ deve existir no diretório de destino')
    assert(!fs.existsSync(src), 'REQ não deve mais existir no diretório de origem')
    const content = fs.readFileSync(dst, 'utf8')
    assert(content.includes('status: done'), 'frontmatter status deve refletir o novo estado')
    assert(content.includes('| Status: done'), 'header Status deve refletir o novo estado')
  })
})

// --- 5. moveREQ move físico — layout by_agent ---
test('moveREQ: move fisicamente preservando o agente quando layout é by_agent', () => {
  withTmpProject(() => {
    const reqDir = path.join(process.cwd(), 'docs', 'req')
    const src = path.join(reqDir, 'claude', 'wip', 'REQ-2026-08-04-agente.md')
    writeFile(src, reqBody('wip'))
    writeFile(path.join(process.cwd(), 'trackfw.yaml'), 'roadmap_namespacing: by_agent\nagents:\n  - claude\n')
    config.reset()

    moveREQ('agente', 'done')

    const dst = path.join(reqDir, 'claude', 'done', 'REQ-2026-08-04-agente.md')
    assert(fs.existsSync(dst), 'REQ deve existir no diretório de destino preservando o agente')
    assert(!fs.existsSync(src), 'REQ não deve mais existir no diretório de origem')
  })
})

// --- 6. moveREQ registra transição em .trackfw-log ---
test('moveREQ: registra a transição em docs/req/.trackfw-log ao mover fisicamente', () => {
  withTmpProject(() => {
    const reqDir = path.join(process.cwd(), 'docs', 'req')
    const src = path.join(reqDir, 'backlog', 'REQ-2026-08-04-log.md')
    writeFile(src, reqBody('backlog'))

    moveREQ('log', 'wip')

    const logPath = path.join(reqDir, '.trackfw-log')
    assert(fs.existsSync(logPath), '.trackfw-log deve ser criado')
    const logContent = fs.readFileSync(logPath, 'utf8')
    assert(logContent.includes('REQ-2026-08-04-log.md'), 'log deve referenciar o basename do REQ')
    assert(logContent.includes('backlog → wip'), 'log deve registrar a transição de estado')
  })
})

// --- 7. moveREQ rejeita status inválido no move físico (AC5 — paridade com Go) ---
test('moveREQ: rejeita status inválido quando REQ está em subpasta de estado reconhecida', () => {
  withTmpProject(() => {
    const reqDir = path.join(process.cwd(), 'docs', 'req')
    const src = path.join(reqDir, 'wip', 'REQ-2026-08-04-invalido.md')
    writeFile(src, reqBody('wip'))

    assert.throws(
      () => moveREQ('invalido', 'status-invalido-xyz'),
      /invalid state/,
      'moveREQ deveria lançar erro para status inválido'
    )

    assert(fs.existsSync(src), 'REQ deve permanecer no caminho original')
    assert(!fs.existsSync(path.join(reqDir, 'status-invalido-xyz')), 'não deve criar pasta arbitrária')
  })
})

// --- 8. moveREQ rejeita status inválido no move físico — layout by_agent ---
test('moveREQ: rejeita status inválido quando REQ está em subpasta by_agent reconhecida', () => {
  withTmpProject(() => {
    const reqDir = path.join(process.cwd(), 'docs', 'req')
    const src = path.join(reqDir, 'claude', 'wip', 'REQ-2026-08-04-invalido-agente.md')
    writeFile(src, reqBody('wip'))
    writeFile(path.join(process.cwd(), 'trackfw.yaml'), 'roadmap_namespacing: by_agent\nagents:\n  - claude\n')
    config.reset()

    assert.throws(
      () => moveREQ('invalido-agente', 'status-invalido-xyz'),
      /invalid state/,
      'moveREQ deveria lançar erro para status inválido'
    )

    assert(fs.existsSync(src), 'REQ deve permanecer no caminho original')
    assert(!fs.existsSync(path.join(reqDir, 'claude', 'status-invalido-xyz')), 'não deve criar pasta arbitrária')
  })
})

// --- Bônus: listREQs imprime saída formatada e mensagem de vazio ---
test('listREQs: imprime "No REQs found in <reqDir>" quando não há REQs em nenhum layout', () => {
  withTmpProject(() => {
    const cfg = config.load()
    let captured = ''
    const orig = console.log
    console.log = (msg) => { captured += msg + '\n' }
    try {
      listREQs(cfg)
    } finally {
      console.log = orig
    }
    assert(captured.includes(`No REQs found in ${cfg.reqDir}`), `mensagem inesperada: ${captured}`)
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
