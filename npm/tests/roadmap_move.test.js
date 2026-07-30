'use strict'
/**
 * roadmap_move.test.js — Testes para moveRoadmap e rewriteRoadmapStatus.
 * Cobre: move válido, estado inválido, não encontrado, sincronização de frontmatter
 * (P4: validate após move), escopo do frontmatter, e arquivo sem frontmatter.
 */
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')

const config = require('../src/config/index.js')
const { listRoadmaps, showRoadmap, moveRoadmap, rewriteRoadmapStatus, newRoadmap } = require('../src/generators/roadmap')
const { validateFolderStatusCoherence } = require('../src/validator/index.js')

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

/**
 * Helper: cria tmpdir, configura trackfw.yaml com roadmap_dir apontando para tmpdir,
 * muda cwd, executa fn, restaura.
 */
function withRoadmapDir(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-move-'))
  const origCwd = process.cwd()
  try {
    const roadmapDir = path.join(tmp, 'docs', 'roadmaps')
    fs.mkdirSync(roadmapDir, { recursive: true })
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: docs/roadmaps\n`, 'utf8')
    config.reset()
    process.chdir(tmp)
    fn(tmp, roadmapDir)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

/**
 * Cria os subdiretórios de estado padrão dentro de roadmapDir.
 */
function mkStateDirs(roadmapDir) {
  for (const state of ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']) {
    fs.mkdirSync(path.join(roadmapDir, state), { recursive: true })
  }
}

function canonicalRoadmap(title, state = 'backlog') {
  return `---\nstatus: ${state}\ndate: 2026-07-27\nreq: "docs/req/REQ-demo.md"\nsquad: ""\n---\n\n# Roadmap: ${title}\n\n> Created: 2026-07-27 | Status: ${state}\n`
}

function captureConsoleLog(fn) {
  const original = console.log
  const lines = []
  try {
    console.log = (...args) => lines.push(args.join(' '))
    fn()
  } finally {
    console.log = original
  }
  return lines.join('\n')
}

// ─── Testes básicos de moveRoadmap ────────────────────────────────────────────

test('moveRoadmap — estado inválido seta process.exitCode e retorna', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    mkStateDirs(roadmapDir)
    const savedExit = process.exitCode
    try {
      process.exitCode = undefined
      moveRoadmap('qualquer-coisa', 'inexistente')
      assert.strictEqual(process.exitCode, 1, 'exitCode deve ser 1 para estado inválido')
    } finally {
      process.exitCode = savedExit
    }
  })
})

test('moveRoadmap — roadmap não encontrado seta process.exitCode e retorna', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    mkStateDirs(roadmapDir)
    const savedExit = process.exitCode
    try {
      process.exitCode = undefined
      moveRoadmap('nao-existe', 'wip')
      assert.strictEqual(process.exitCode, 1, 'exitCode deve ser 1 para não encontrado')
    } finally {
      process.exitCode = savedExit
    }
  })
})

test('moveRoadmap — move válido: arquivo em wip, backlog vazio, frontmatter status: wip', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    mkStateDirs(roadmapDir)
    newRoadmap('Node Move Test')

    const backlogBefore = fs.readdirSync(path.join(roadmapDir, 'backlog')).filter(f => f.endsWith('.md'))
    assert.strictEqual(backlogBefore.length, 1, 'deve ter 1 arquivo em backlog antes do move')

    moveRoadmap('node-move-test', 'wip')

    const wip = fs.readdirSync(path.join(roadmapDir, 'wip')).filter(f => f.endsWith('.md'))
    assert.strictEqual(wip.length, 1, 'deve ter 1 arquivo em wip após move')

    const backlogAfter = fs.readdirSync(path.join(roadmapDir, 'backlog')).filter(f => f.endsWith('.md'))
    assert.strictEqual(backlogAfter.length, 0, 'backlog deve estar vazio após move')

    const content = fs.readFileSync(path.join(roadmapDir, 'wip', wip[0]), 'utf8')
    assert.ok(content.includes('status: wip'), `frontmatter deve ter 'status: wip'; obteve:\n${content}`)
    assert.ok(content.includes('| Status: wip'), `cabeçalho deve ter '| Status: wip'; obteve:\n${content}`)
  })
})

// ─── P4: validate após move ───────────────────────────────────────────────────

test('moveRoadmap — validate após move não gera warning folder_status (P4)', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    mkStateDirs(roadmapDir)

    // Criar e mover roadmap real: backlog → wip → done
    newRoadmap('Validate Node Test')
    moveRoadmap('validate-node-test', 'wip')
    moveRoadmap('validate-node-test', 'done')

    // Controle positivo: arquivo em wip com status: backlog DEVE gerar warning
    const controlContent = '---\nstatus: backlog\ndate: 2026-01-01\n---\n# Roadmap: Control\n\n> Created: 2026-01-01 | Status: backlog\n'
    fs.writeFileSync(path.join(roadmapDir, 'wip', 'ROADMAP-control.md'), controlContent, 'utf8')

    const warnings = validateFolderStatusCoherence()

    // O arquivo movido NÃO deve gerar folder_status warning
    const movedWarnings = warnings.filter(w => w.includes('validate-node-test'))
    assert.strictEqual(movedWarnings.length, 0,
      `roadmap movido gerou warning folder_status inesperado: ${movedWarnings}`)

    // O controle positivo DEVE gerar warning
    const controlWarnings = warnings.filter(w => w.includes('ROADMAP-control.md') && w.includes('folder'))
    assert.ok(controlWarnings.length > 0,
      `controle positivo não gerou warning — validador pode não estar inspecionando os arquivos; warnings: ${JSON.stringify(warnings)}`)
  })
})

// ─── Escopo do frontmatter ────────────────────────────────────────────────────

test('moveRoadmap — status: no corpo e | Status: em seção não são tocados', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    mkStateDirs(roadmapDir)

    const bodyContent =
      '---\nstatus: backlog\ndate: 2026-01-01\n---\n' +
      '# Roadmap: Body Scope Test\n\n' +
      '> Created: 2026-01-01 | Status: backlog\n\n' +
      '## Context\n\n' +
      'Tabela com status:\n\n' +
      '| Campo | status: backlog |\n' +
      '|-------|----------------|\n\n' +
      'Código:\n\n' +
      '```\n' +
      '> Created: 2026-01-01 | Status: backlog\n' +
      '```\n'

    fs.writeFileSync(path.join(roadmapDir, 'backlog', 'ROADMAP-body-scope-test.md'), bodyContent, 'utf8')

    moveRoadmap('body-scope-test', 'wip')

    const files = fs.readdirSync(path.join(roadmapDir, 'wip')).filter(f => f.endsWith('.md'))
    assert.strictEqual(files.length, 1)
    const content = fs.readFileSync(path.join(roadmapDir, 'wip', files[0]), 'utf8')

    // Frontmatter sincronizado
    assert.ok(content.includes('status: wip'), `frontmatter deve conter 'status: wip'`)
    // Cabeçalho sincronizado (antes do ## )
    assert.ok(content.includes('| Status: wip'), `cabeçalho deve conter '| Status: wip'`)
    // Corpo intocado: tabela
    assert.ok(content.includes('| Campo | status: backlog |'), `linha do corpo 'status: backlog' foi modificada`)
    // Corpo intocado: bloco de código (após ## )
    assert.ok(
      content.includes('```\n> Created: 2026-01-01 | Status: backlog\n```'),
      `'| Status: backlog' no bloco de código foi modificado; corpo:\n${content}`
    )
  })
})

// ─── Arquivo sem frontmatter ──────────────────────────────────────────────────

test('moveRoadmap — arquivo sem frontmatter: move funciona, conteúdo intacto', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    mkStateDirs(roadmapDir)

    const plainContent = '# Roadmap sem frontmatter\n\nConteúdo simples sem bloco ---.\n'
    fs.writeFileSync(path.join(roadmapDir, 'backlog', 'ROADMAP-no-fm.md'), plainContent, 'utf8')

    moveRoadmap('no-fm', 'wip')

    const files = fs.readdirSync(path.join(roadmapDir, 'wip')).filter(f => f.endsWith('.md'))
    assert.strictEqual(files.length, 1, 'deve ter 1 arquivo em wip')

    const content = fs.readFileSync(path.join(roadmapDir, 'wip', files[0]), 'utf8')
    assert.strictEqual(content, plainContent, 'conteúdo deve ser idêntico ao original')
  })
})

test('moveRoadmap — analyzing flat: move, sincroniza frontmatter/header e registra log', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    for (const state of ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']) {
      fs.mkdirSync(path.join(roadmapDir, state), { recursive: true })
    }
    fs.writeFileSync(
      path.join(roadmapDir, 'backlog', 'ROADMAP-analyze-flat.md'),
      canonicalRoadmap('Analyze Flat'),
      'utf8'
    )

    const savedExit = process.exitCode
    try {
      process.exitCode = undefined
      moveRoadmap('analyze-flat', 'analyzing')
      assert.notStrictEqual(process.exitCode, 1, 'moveRoadmap não deve marcar exitCode=1 para analyzing')
    } finally {
      process.exitCode = savedExit
    }

    const dst = path.join(roadmapDir, 'analyzing', 'ROADMAP-analyze-flat.md')
    const content = fs.readFileSync(dst, 'utf8')
    assert.ok(content.includes('status: analyzing'), 'frontmatter deve sincronizar status: analyzing')
    assert.ok(content.includes('| Status: analyzing'), 'header deve sincronizar | Status: analyzing')
    const log = fs.readFileSync(path.join(roadmapDir, '.trackfw-log'), 'utf8')
    assert.ok(log.includes('ROADMAP-analyze-flat.md') && log.includes('backlog → analyzing'), 'log deve registrar backlog → analyzing')
  })
})

test('moveRoadmap — analyzing by_agent: preserva agente no path e no log', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\nroadmap_namespacing: by_agent\nagents:\n- zeus\n', 'utf8')
    config.reset()
    fs.mkdirSync(path.join(roadmapDir, 'zeus', 'backlog'), { recursive: true })
    fs.mkdirSync(path.join(roadmapDir, 'zeus', 'analyzing'), { recursive: true })
    fs.writeFileSync(
      path.join(roadmapDir, 'zeus', 'backlog', 'ROADMAP-analyze-by-agent.md'),
      canonicalRoadmap('Analyze By Agent'),
      'utf8'
    )

    const savedExit = process.exitCode
    try {
      process.exitCode = undefined
      moveRoadmap('analyze-by-agent', 'analyzing')
      assert.notStrictEqual(process.exitCode, 1, 'moveRoadmap não deve marcar exitCode=1 para analyzing')
    } finally {
      process.exitCode = savedExit
    }

    const dst = path.join(roadmapDir, 'zeus', 'analyzing', 'ROADMAP-analyze-by-agent.md')
    const content = fs.readFileSync(dst, 'utf8')
    assert.ok(content.includes('status: analyzing'), 'frontmatter deve sincronizar status: analyzing')
    assert.ok(content.includes('| Status: analyzing'), 'header deve sincronizar | Status: analyzing')
    const log = fs.readFileSync(path.join(roadmapDir, '.trackfw-log'), 'utf8')
    assert.ok(log.includes('zeus/ROADMAP-analyze-by-agent.md') && log.includes('backlog → analyzing'), 'log deve preservar agente e registrar backlog → analyzing')
  })
})

test('listRoadmaps/showRoadmap — encontram roadmap em analyzing flat', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    mkStateDirs(roadmapDir)
    fs.writeFileSync(
      path.join(roadmapDir, 'backlog', 'ROADMAP-analyze-show-flat.md'),
      canonicalRoadmap('Analyze Show Flat'),
      'utf8'
    )
    moveRoadmap('analyze-show-flat', 'analyzing')

    const listOutput = captureConsoleLog(() => listRoadmaps())
    assert.ok(listOutput.includes('[analyzing]'), `list deve exibir seção analyzing; obteve:\n${listOutput}`)
    assert.ok(listOutput.includes('ROADMAP-analyze-show-flat.md'), `list deve exibir arquivo em analyzing; obteve:\n${listOutput}`)

    const showOutput = captureConsoleLog(() => showRoadmap('analyze-show-flat'))
    assert.ok(showOutput.includes('[ANALYZING]'), `show deve localizar analyzing; obteve:\n${showOutput}`)
    assert.ok(showOutput.includes('status: analyzing'), `show deve imprimir conteúdo sincronizado; obteve:\n${showOutput}`)
  })
})

test('listRoadmaps/showRoadmap — encontram roadmap em analyzing by_agent', () => {
  withRoadmapDir((tmp, roadmapDir) => {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\nroadmap_namespacing: by_agent\nagents:\n- zeus\n', 'utf8')
    config.reset()
    fs.mkdirSync(path.join(roadmapDir, 'zeus', 'backlog'), { recursive: true })
    fs.writeFileSync(
      path.join(roadmapDir, 'zeus', 'backlog', 'ROADMAP-analyze-show-by-agent.md'),
      canonicalRoadmap('Analyze Show By Agent'),
      'utf8'
    )
    moveRoadmap('analyze-show-by-agent', 'analyzing')

    const listOutput = captureConsoleLog(() => listRoadmaps())
    assert.ok(listOutput.includes('[zeus/analyzing]'), `list deve exibir seção zeus/analyzing; obteve:\n${listOutput}`)
    assert.ok(listOutput.includes('ROADMAP-analyze-show-by-agent.md'), `list deve exibir arquivo em analyzing; obteve:\n${listOutput}`)

    const showOutput = captureConsoleLog(() => showRoadmap('analyze-show-by-agent'))
    assert.ok(showOutput.includes('[ANALYZING]'), `show deve localizar analyzing; obteve:\n${showOutput}`)
    assert.ok(showOutput.includes('status: analyzing'), `show deve imprimir conteúdo sincronizado; obteve:\n${showOutput}`)
  })
})

// ─── Testes unitários de rewriteRoadmapStatus ─────────────────────────────────

test('rewriteRoadmapStatus — sem frontmatter: retorna source inalterada, changed=false', () => {
  const src = '# Roadmap sem frontmatter\n\nTexto simples.\n'
  const { content, changed } = rewriteRoadmapStatus(src, 'wip')
  assert.strictEqual(changed, false)
  assert.strictEqual(content, src)
})

test('rewriteRoadmapStatus — sem chave status no frontmatter: retorna source inalterada', () => {
  const src = '---\ndate: 2026-01-01\n---\n# Roadmap\n'
  const { content, changed } = rewriteRoadmapStatus(src, 'wip')
  assert.strictEqual(changed, false)
  assert.strictEqual(content, src)
})

test('rewriteRoadmapStatus — reescreve status: backlog → wip minúsculo', () => {
  const src = '---\nstatus: backlog\ndate: 2026-01-01\n---\n# Roadmap\n\n> Created: 2026-01-01 | Status: backlog\n'
  const { content, changed } = rewriteRoadmapStatus(src, 'wip')
  assert.strictEqual(changed, true)
  assert.ok(content.includes('status: wip'), `deve conter 'status: wip'; obteve:\n${content}`)
  assert.ok(content.includes('| Status: wip'), `deve conter '| Status: wip'; obteve:\n${content}`)
})

test('rewriteRoadmapStatus — preserva aspas ao redor do valor', () => {
  const src = '---\nstatus: "backlog"\ndate: 2026-01-01\n---\n# Roadmap\n'
  const { content, changed } = rewriteRoadmapStatus(src, 'wip')
  assert.strictEqual(changed, true)
  assert.ok(content.includes('status: "wip"'), `deve preservar aspas; obteve:\n${content}`)
})

// ─── ML-2B: syncReqReferences — cinco cardinalidades + idempotência + by_agent ───────────────

const { extractFrontmatterRoadmap, rewriteReqRoadmapRef, syncReqReferences } = require('../src/generators/roadmap')
const { validateRefTargetsExist } = require('../src/validator/index.js')

/**
 * Helper: cria tmpdir com roadmap_dir + req_dir configurados.
 */
function withReqAndRoadmapDir(fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-sync-'))
  const origCwd = process.cwd()
  try {
    const roadmapDir = path.join(tmp, 'docs', 'roadmaps')
    const reqDir = path.join(tmp, 'docs', 'req')
    fs.mkdirSync(roadmapDir, { recursive: true })
    fs.mkdirSync(reqDir, { recursive: true })
    fs.writeFileSync(
      path.join(tmp, 'trackfw.yaml'),
      `roadmap_dir: docs/roadmaps\nreq_dir: docs/req\n`,
      'utf8'
    )
    config.reset()
    process.chdir(tmp)
    fn(tmp, roadmapDir, reqDir)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

function mkAllStateDirs(roadmapDir) {
  for (const s of ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']) {
    fs.mkdirSync(path.join(roadmapDir, s), { recursive: true })
  }
}

/**
 * Cria uma REQ canônica com frontmatter roadmap: e linha Roadmap: no corpo (com backticks).
 */
function makeReqContent(roadmapPath) {
  return (
    `---\nstatus: Open\ndate: 2026-07-30\nauthor: "test"\n` +
    `roadmap: "${roadmapPath}"\n---\n\n` +
    `# REQ: test\n\n` +
    `## Linked Roadmap\n` +
    `Roadmap: \`${roadmapPath}\`\n`
  )
}

function captureOutput(fn) {
  const origLog = console.log
  const origErr = console.error
  const stdout = [], stderr = []
  try {
    console.log = (...a) => stdout.push(a.join(' '))
    console.error = (...a) => stderr.push(a.join(' '))
    fn()
  } finally {
    console.log = origLog
    console.error = origErr
  }
  return { stdout: stdout.join('\n'), stderr: stderr.join('\n') }
}

// ─── Cardinalidade: zero REQs → no-op silencioso ─────────────────────────────

test('syncReqReferences — zero REQs: no-op silencioso, exit 0', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-zero-req.md'
    fs.writeFileSync(
      path.join(roadmapDir, 'backlog', roadmapFile),
      canonicalRoadmap('Zero Req Test'),
      'utf8'
    )

    const savedExit = process.exitCode
    try {
      process.exitCode = undefined
      const { stdout, stderr } = captureOutput(() => moveRoadmap('zero-req', 'wip'))
      // Apenas a linha ✓ moved, sem nenhuma linha ✓ synced
      assert.ok(!stdout.includes('synced'), `não deve imprimir synced; stdout: ${stdout}`)
      assert.strictEqual(stderr, '', `stderr deve estar vazio; stderr: ${stderr}`)
      assert.notStrictEqual(process.exitCode, 1, 'exit code não deve ser 1')
    } finally {
      process.exitCode = savedExit
    }
  })
})

// ─── Cardinalidade: uma REQ → reescreve frontmatter e corpo ──────────────────

test('syncReqReferences — uma REQ: reescreve frontmatter roadmap: e corpo Roadmap: com backticks', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-one-req.md'
    const oldPath = `docs/roadmaps/backlog/${roadmapFile}`
    const newPath = `docs/roadmaps/wip/${roadmapFile}`
    const reqPath = path.join(reqDir, 'REQ-one.md')

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('One Req Test'), 'utf8')
    fs.writeFileSync(reqPath, makeReqContent(oldPath), 'utf8')

    const { stdout } = captureOutput(() => moveRoadmap('one-req', 'wip'))

    assert.ok(stdout.includes(`✓ synced REQ-one.md → ${newPath}`), `deve imprimir synced; stdout: ${stdout}`)

    const reqContent = fs.readFileSync(reqPath, 'utf8')
    assert.ok(reqContent.includes(`roadmap: "${newPath}"`), `frontmatter deve ter novo caminho; got:\n${reqContent}`)
    assert.ok(reqContent.includes(`Roadmap: \`${newPath}\``), `corpo deve ter novo caminho com backticks; got:\n${reqContent}`)
    assert.ok(!reqContent.includes(oldPath), `oldPath não deve aparecer na REQ; got:\n${reqContent}`)
  })
})

// ─── Cardinalidade: várias REQs → todas reescritas ───────────────────────────

test('syncReqReferences — várias REQs: todas reescritas, uma linha cada, sequência lexicográfica por basename', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-multi-req.md'
    const oldPath = `docs/roadmaps/backlog/${roadmapFile}`
    const newPath = `docs/roadmaps/wip/${roadmapFile}`

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('Multi Req Test'), 'utf8')
    fs.writeFileSync(path.join(reqDir, 'REQ-A.md'), makeReqContent(oldPath), 'utf8')
    fs.writeFileSync(path.join(reqDir, 'REQ-B.md'), makeReqContent(oldPath), 'utf8')

    const { stdout } = captureOutput(() => moveRoadmap('multi-req', 'wip'))

    // Ambas as REQs devem aparecer no stdout
    assert.ok(stdout.includes('✓ synced REQ-A.md'), `REQ-A deve ser sincronizada; stdout: ${stdout}`)
    assert.ok(stdout.includes('✓ synced REQ-B.md'), `REQ-B deve ser sincronizada; stdout: ${stdout}`)

    // Sequência deve ser lexicográfica por basename: REQ-A antes de REQ-B
    const posA = stdout.indexOf('✓ synced REQ-A.md')
    const posB = stdout.indexOf('✓ synced REQ-B.md')
    assert.ok(posA < posB, `REQ-A deve ser sincronizada antes de REQ-B (ordem por basename); stdout:\n${stdout}`)

    const cA = fs.readFileSync(path.join(reqDir, 'REQ-A.md'), 'utf8')
    const cB = fs.readFileSync(path.join(reqDir, 'REQ-B.md'), 'utf8')
    assert.ok(cA.includes(`roadmap: "${newPath}"`), `REQ-A frontmatter deve ter novo path`)
    assert.ok(cB.includes(`roadmap: "${newPath}"`), `REQ-B frontmatter deve ter novo path`)
  })
})

// ─── Cardinalidade: aponta para outro roadmap → não toca ─────────────────────

test('syncReqReferences — REQ aponta para outro roadmap: não é tocada', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-move-me.md'
    const otherRoadmap = 'docs/roadmaps/done/ROADMAP-2026-07-30-outro.md'
    const reqPath = path.join(reqDir, 'REQ-other.md')

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('Move Me Test'), 'utf8')
    fs.writeFileSync(reqPath, makeReqContent(otherRoadmap), 'utf8')
    const originalContent = fs.readFileSync(reqPath)

    const { stdout } = captureOutput(() => moveRoadmap('move-me', 'wip'))

    assert.ok(!stdout.includes('synced'), `REQ apontando para outro roadmap não deve ser tocada; stdout: ${stdout}`)
    const after = fs.readFileSync(reqPath)
    assert.ok(originalContent.equals(after), 'conteúdo da REQ deve ser idêntico byte-a-byte')
  })
})

// ─── Cardinalidade: referência já correta → nenhuma escrita (idempotência) ───

test('syncReqReferences — referência já correta: nenhuma escrita, idempotência byte-a-byte', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-idem.md'
    const oldPath = `docs/roadmaps/backlog/${roadmapFile}`
    const newPath = `docs/roadmaps/wip/${roadmapFile}`
    const reqPath = path.join(reqDir, 'REQ-idem.md')

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('Idem Test'), 'utf8')
    fs.writeFileSync(reqPath, makeReqContent(oldPath), 'utf8')

    // Primeiro move: deve reescrever a REQ
    captureOutput(() => moveRoadmap('idem', 'wip'))
    const afterFirst = fs.readFileSync(reqPath)
    assert.ok(afterFirst.toString().includes(`roadmap: "${newPath}"`), 'primeiro move deve atualizar a REQ')

    // Cria o roadmap novamente em wip para simular segundo move (done)
    const newPath2 = `docs/roadmaps/done/${roadmapFile}`
    fs.writeFileSync(path.join(roadmapDir, 'wip', roadmapFile), canonicalRoadmap('Idem Test', 'wip'), 'utf8')

    // Segundo move: deve reescrever a REQ de wip → done
    captureOutput(() => moveRoadmap('idem', 'done'))
    const afterSecond = fs.readFileSync(reqPath)
    assert.ok(afterSecond.toString().includes(`roadmap: "${newPath2}"`), 'segundo move deve atualizar a REQ para done')

    // Terceiro move simulado: REQ já aponta para done — forçar idempotência via terceiro move artificial
    // Recria o roadmap em done para testar que a REQ não é tocada se já correta
    fs.writeFileSync(path.join(reqDir, 'REQ-idem.md'), makeReqContent(newPath2), 'utf8')
    const beforeThird = fs.readFileSync(reqPath)
    fs.writeFileSync(path.join(roadmapDir, 'done', roadmapFile), canonicalRoadmap('Idem Test', 'done'), 'utf8')

    // Simula o sync diretamente: referência já correta → sem escrita
    const cfg = config.load()
    const { stdout } = captureOutput(() => syncReqReferences(roadmapFile, newPath2, cfg))
    const afterThird = fs.readFileSync(reqPath)
    assert.ok(beforeThird.equals(afterThird), 'bytes da REQ não devem mudar quando referência já está correta')
    assert.ok(!stdout.includes('synced'), `não deve imprimir synced quando já correto; stdout: ${stdout}`)
  })
})

// ─── Idempotência byte-a-byte: mover duas vezes, comparar bytes ──────────────

test('syncReqReferences — idempotência byte-a-byte: mover duas vezes não altera bytes da REQ', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-byte-idem.md'
    const oldPath = `docs/roadmaps/backlog/${roadmapFile}`
    const reqPath = path.join(reqDir, 'REQ-byte.md')

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('Byte Idem Test'), 'utf8')
    fs.writeFileSync(reqPath, makeReqContent(oldPath), 'utf8')

    // Primeiro move: backlog → wip
    captureOutput(() => moveRoadmap('byte-idem', 'wip'))
    const afterFirstMove = fs.readFileSync(reqPath)

    // Recria roadmap em wip para segundo move
    fs.writeFileSync(path.join(roadmapDir, 'wip', roadmapFile), canonicalRoadmap('Byte Idem Test', 'wip'), 'utf8')
    // Recria a REQ apontando para wip (simula estado após primeiro move)
    // Segundo move: wip → done
    captureOutput(() => moveRoadmap('byte-idem', 'done'))
    const afterSecondMove = fs.readFileSync(reqPath)

    // Os bytes da REQ após o segundo move devem diferir do primeiro (caminho mudou)
    // mas o conteúdo deve ser consistente e a REQ não deve estar duplicada
    assert.ok(afterSecondMove.toString().includes('docs/roadmaps/done/'), 'REQ deve apontar para done após segundo move')

    // Agora simula: sync com mesma REQ já correta (terceiro chamado, idempotência pura)
    const cfg = config.load()
    const newPath = `docs/roadmaps/done/${roadmapFile}`
    syncReqReferences(roadmapFile, newPath, cfg) // REQ já aponta para done
    const afterThirdCall = fs.readFileSync(reqPath)
    assert.ok(afterSecondMove.equals(afterThirdCall), 'bytes da REQ não devem mudar na terceira chamada (idempotência)')
  })
})

// ─── by_agent: REQ em req_dir/<agente>/<estado>/ é encontrada ────────────────

test('syncReqReferences — by_agent: REQ em req_dir/<agente>/<estado>/ é encontrada e reescrita', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-byagent-'))
  const origCwd = process.cwd()
  try {
    const roadmapDir = path.join(tmp, 'docs', 'roadmaps')
    const reqDir = path.join(tmp, 'docs', 'req')

    fs.mkdirSync(path.join(roadmapDir, 'zeus', 'backlog'), { recursive: true })
    fs.mkdirSync(path.join(roadmapDir, 'zeus', 'wip'), { recursive: true })
    // resolveReqFiles em by_agent usa os estados de roadmap: backlog, wip, done, etc.
    fs.mkdirSync(path.join(reqDir, 'zeus', 'wip'), { recursive: true })
    for (const s of ['analyzing', 'blocked', 'done', 'abandoned']) {
      fs.mkdirSync(path.join(roadmapDir, 'zeus', s), { recursive: true })
    }

    fs.writeFileSync(
      path.join(tmp, 'trackfw.yaml'),
      'roadmap_dir: docs/roadmaps\nreq_dir: docs/req\nroadmap_namespacing: by_agent\nagents:\n- zeus\n',
      'utf8'
    )
    config.reset()
    process.chdir(tmp)

    const roadmapFile = 'ROADMAP-2026-07-30-by-agent-req.md'
    const oldPath = `docs/roadmaps/zeus/backlog/${roadmapFile}`
    const newPath = `docs/roadmaps/zeus/wip/${roadmapFile}`
    const reqPath = path.join(reqDir, 'zeus', 'wip', 'REQ-by-agent.md')

    fs.writeFileSync(
      path.join(roadmapDir, 'zeus', 'backlog', roadmapFile),
      canonicalRoadmap('By Agent Req Test'),
      'utf8'
    )
    fs.writeFileSync(reqPath, makeReqContent(oldPath), 'utf8')

    const { stdout } = captureOutput(() => moveRoadmap('by-agent-req', 'wip'))

    assert.ok(stdout.includes(`✓ synced REQ-by-agent.md → ${newPath}`), `by_agent REQ deve ser encontrada e sincronizada; stdout: ${stdout}`)

    const reqContent = fs.readFileSync(reqPath, 'utf8')
    assert.ok(reqContent.includes(`roadmap: "${newPath}"`), `frontmatter by_agent deve ter novo caminho; got:\n${reqContent}`)
    assert.ok(reqContent.includes(`Roadmap: \`${newPath}\``), `corpo by_agent deve ter novo caminho com backticks; got:\n${reqContent}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ─── ML-2E: ordenação discriminante by_agent — basename vs caminho completo ───
//
// Fixture: agents: [zeus, apolo]
//   docs/req/apolo/done/REQ-zzz.md  → aponta para o roadmap
//   docs/req/zeus/backlog/REQ-aaa.md → aponta para o roadmap
//
// Por caminho: apolo/...zzz < zeus/...aaa → zzz, aaa  (ERRADO)
// Por basename: aaa < zzz               → aaa, zzz  (CORRETO)
//
// Um teste onde os dois critérios coincidem não prova nada.

test('syncReqReferences — by_agent discriminante: ordenação por basename, não por caminho completo', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-sort-discrim-'))
  const origCwd = process.cwd()
  try {
    const roadmapDir = path.join(tmp, 'docs', 'roadmaps')
    const reqDir = path.join(tmp, 'docs', 'req')

    // Estrutura de roadmap em by_agent para zeus
    fs.mkdirSync(path.join(roadmapDir, 'zeus', 'backlog'), { recursive: true })
    fs.mkdirSync(path.join(roadmapDir, 'zeus', 'wip'), { recursive: true })
    for (const s of ['analyzing', 'blocked', 'done', 'abandoned']) {
      fs.mkdirSync(path.join(roadmapDir, 'zeus', s), { recursive: true })
    }

    // REQs: apolo/done/REQ-zzz.md e zeus/backlog/REQ-aaa.md
    // agents: [zeus, apolo] — zeus vem primeiro, mas REQ-aaa tem basename menor
    fs.mkdirSync(path.join(reqDir, 'apolo', 'done'), { recursive: true })
    fs.mkdirSync(path.join(reqDir, 'zeus', 'backlog'), { recursive: true })

    fs.writeFileSync(
      path.join(tmp, 'trackfw.yaml'),
      'roadmap_dir: docs/roadmaps\nreq_dir: docs/req\nroadmap_namespacing: by_agent\nagents:\n- zeus\n- apolo\n',
      'utf8'
    )
    config.reset()
    process.chdir(tmp)

    const roadmapFile = 'ROADMAP-2026-07-30-sort-discrim.md'
    const oldPath = `docs/roadmaps/zeus/backlog/${roadmapFile}`
    const newPath = `docs/roadmaps/zeus/wip/${roadmapFile}`

    fs.writeFileSync(
      path.join(roadmapDir, 'zeus', 'backlog', roadmapFile),
      canonicalRoadmap('Sort Discrim Test'),
      'utf8'
    )

    // REQ-zzz em apolo/done (caminho apolo/... vem antes de zeus/... alfabeticamente)
    const reqZzzPath = path.join(reqDir, 'apolo', 'done', 'REQ-zzz.md')
    // REQ-aaa em zeus/backlog (basename aaa < zzz — deve aparecer primeiro na saída)
    const reqAaaPath = path.join(reqDir, 'zeus', 'backlog', 'REQ-aaa.md')

    fs.writeFileSync(reqZzzPath, makeReqContent(oldPath), 'utf8')
    fs.writeFileSync(reqAaaPath, makeReqContent(oldPath), 'utf8')

    const { stdout } = captureOutput(() => moveRoadmap('sort-discrim', 'wip'))

    // Ambas devem ser sincronizadas
    assert.ok(stdout.includes('✓ synced REQ-aaa.md'), `REQ-aaa deve ser sincronizada; stdout:\n${stdout}`)
    assert.ok(stdout.includes('✓ synced REQ-zzz.md'), `REQ-zzz deve ser sincronizada; stdout:\n${stdout}`)

    // Sequência: REQ-aaa (basename menor) deve aparecer ANTES de REQ-zzz
    const posAaa = stdout.indexOf('✓ synced REQ-aaa.md')
    const posZzz = stdout.indexOf('✓ synced REQ-zzz.md')
    assert.ok(
      posAaa < posZzz,
      `REQ-aaa deve ser emitida antes de REQ-zzz (ordenação por basename, não por caminho); stdout:\n${stdout}`
    )

    // Conteúdo correto após sync
    const cZzz = fs.readFileSync(reqZzzPath, 'utf8')
    const cAaa = fs.readFileSync(reqAaaPath, 'utf8')
    assert.ok(cZzz.includes(`roadmap: "${newPath}"`), `REQ-zzz deve ter novo path; got:\n${cZzz}`)
    assert.ok(cAaa.includes(`roadmap: "${newPath}"`), `REQ-aaa deve ter novo path; got:\n${cAaa}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ─── Corpo com backticks preservados ─────────────────────────────────────────

test('syncReqReferences — backticks no corpo são preservados após reescrita', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-backtick.md'
    const oldPath = `docs/roadmaps/backlog/${roadmapFile}`
    const newPath = `docs/roadmaps/wip/${roadmapFile}`
    const reqPath = path.join(reqDir, 'REQ-bt.md')

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('Backtick Test'), 'utf8')
    // REQ com backticks no corpo
    const reqContent =
      `---\nstatus: Open\ndate: 2026-07-30\nroadmap: "${oldPath}"\n---\n\n` +
      `# REQ: backtick\n\n## Linked Roadmap\n` +
      `Roadmap: \`${oldPath}\`\n`
    fs.writeFileSync(reqPath, reqContent, 'utf8')

    captureOutput(() => moveRoadmap('backtick', 'wip'))

    const after = fs.readFileSync(reqPath, 'utf8')
    // Corpo deve ter backticks ao redor do novo caminho
    assert.ok(after.includes(`Roadmap: \`${newPath}\``), `backticks devem ser preservados no corpo; got:\n${after}`)
    // Frontmatter não deve ter backticks (só aspas, como estava antes)
    assert.ok(after.includes(`roadmap: "${newPath}"`), `frontmatter deve ter aspas, não backticks; got:\n${after}`)
  })
})

// ─── Erro ao gravar REQ: diagnóstico em stderr, exit não-zero, move não desfeito ───

test('syncReqReferences — erro ao gravar REQ: stderr nomeia a REQ, exit não-zero, move não desfeito', () => {
  // Pula em root (chmod não bloqueia root)
  if (process.getuid && process.getuid() === 0) return

  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-write-err.md'
    const oldPath = `docs/roadmaps/backlog/${roadmapFile}`
    const reqPath = path.join(reqDir, 'REQ-werr.md')

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('Write Error Test'), 'utf8')
    fs.writeFileSync(reqPath, makeReqContent(oldPath), 'utf8')
    // Torna a REQ não-gravável
    fs.chmodSync(reqPath, 0o444)

    const savedExit = process.exitCode
    try {
      process.exitCode = undefined
      const { stderr } = captureOutput(() => moveRoadmap('write-err', 'wip'))

      // Diagnóstico em stderr nomeando a REQ
      assert.ok(
        stderr.includes('trackfw roadmap move: failed to sync REQ-werr.md:'),
        `stderr deve nomear a REQ; stderr: ${stderr}`
      )
      // Exit não-zero
      assert.strictEqual(process.exitCode, 1, 'exitCode deve ser 1 em erro de escrita')
      // Move não desfeito
      const wipFiles = fs.readdirSync(path.join(roadmapDir, 'wip')).filter(f => f.endsWith('.md'))
      assert.strictEqual(wipFiles.length, 1, 'roadmap deve ter sido movido para wip mesmo com erro na REQ')
    } finally {
      process.exitCode = savedExit
      try { fs.chmodSync(reqPath, 0o644) } catch (_) {}
    }
  })
})

// ─── validateRefTargetsExist: zero violações após o move ─────────────────────

test('syncReqReferences — validateRefTargetsExist: zero violações ref_targets_exist após move', () => {
  withReqAndRoadmapDir((tmp, roadmapDir, reqDir) => {
    mkAllStateDirs(roadmapDir)
    const roadmapFile = 'ROADMAP-2026-07-30-validate-sync.md'
    const oldPath = `docs/roadmaps/backlog/${roadmapFile}`
    const reqPath = path.join(reqDir, 'REQ-vsync.md')

    // Cria o ADR referenciado pela REQ (req precisa de ADR para não disparar outra violação)
    const adrDir = path.join(tmp, 'docs', 'adr')
    fs.mkdirSync(adrDir, { recursive: true })
    const adrPath = 'docs/adr/ADR-001-test.md'
    fs.writeFileSync(path.join(tmp, adrPath), '---\nstatus: accepted\n---\n# ADR-001\n', 'utf8')

    fs.writeFileSync(path.join(roadmapDir, 'backlog', roadmapFile), canonicalRoadmap('Validate Sync Test'), 'utf8')
    // REQ com roadmap: apontando para backlog (estado pré-move)
    const reqContent =
      `---\nstatus: Open\ndate: 2026-07-30\nadr: "${adrPath}"\nroadmap: "${oldPath}"\n---\n\n` +
      `# REQ: validate sync\n\n## Linked Roadmap\nRoadmap: \`${oldPath}\`\n`
    fs.writeFileSync(reqPath, reqContent, 'utf8')

    // Executa o move
    captureOutput(() => moveRoadmap('validate-sync', 'wip'))

    // Verifica que o validador não gera violação ref_targets_exist para esta REQ
    const violations = validateRefTargetsExist()
    const refViolations = violations.filter(v => v.includes('REQ-vsync.md') && v.includes('ref_targets_exist') || (v.includes('REQ-vsync') && v.includes('does not exist')))
    assert.strictEqual(refViolations.length, 0,
      `não deve haver violação ref_targets_exist para REQ-vsync.md; violations: ${JSON.stringify(violations)}`)
  })
})

// ─── Relatório final ─────────────────────────────────────────────────────────

console.log(`\n${passed + failed} testes — ${passed} passaram, ${failed} falharam`)
if (failed > 0) process.exitCode = 1
