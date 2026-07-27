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

// ─── Relatório final ─────────────────────────────────────────────────────────

console.log(`\n${passed + failed} testes — ${passed} passaram, ${failed} falharam`)
if (failed > 0) process.exitCode = 1
