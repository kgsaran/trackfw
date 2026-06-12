'use strict'
const fs = require('fs')
const path = require('path')

const VALID_STATES = {
  backlog:   'docs/roadmaps/backlog',
  wip:       'docs/roadmaps/wip',
  blocked:   'docs/roadmaps/blocked',
  done:      'docs/roadmaps/done',
  abandoned: 'docs/roadmaps/abandoned',
}

const STATE_ORDER = ['wip', 'backlog', 'blocked', 'done', 'abandoned']

const TRANSITION_LOG_PATH = 'docs/roadmaps/.trackfw-log'

/**
 * listRoadmaps — lista roadmaps agrupados por estado (wip, backlog, blocked, done, abandoned).
 * Se nenhum encontrado imprime mensagem orientando o usuário.
 */
function listRoadmaps() {
  let found = false

  for (const state of STATE_ORDER) {
    const dir = VALID_STATES[state]
    let files = []
    try {
      files = fs.readdirSync(dir).filter(f => !fs.statSync(path.join(dir, f)).isDirectory() && f.endsWith('.md'))
    } catch (_) {
      continue
    }
    if (files.length === 0) continue

    found = true
    console.log(`[${state}]`)
    for (const f of files) {
      console.log(`  ${f}`)
    }
  }

  if (!found) {
    console.log("Nenhum roadmap encontrado. Crie um com 'trackfw roadmap new'.")
  }
}

/**
 * showRoadmap — busca docs/roadmaps/ESTADO/NOME*.md (partial match), imprime cabeçalho + conteúdo.
 * 0 matches: erro. múltiplos: lista + erro. 1 match: imprime cabeçalho e conteúdo.
 */
function showRoadmap(name) {
  const matches = findRoadmapMatches(name)

  if (matches.length === 0) {
    console.error(`no roadmap found matching "${name}"`)
    process.exitCode = 1
    return
  }

  if (matches.length > 1) {
    console.log('Multiple roadmaps found — be more specific:')
    for (const m of matches) {
      console.log(`  ${m}`)
    }
    console.error(`ambiguous match for "${name}"`)
    process.exitCode = 1
    return
  }

  const filepath = matches[0]
  const basename = path.basename(filepath)
  const state = path.basename(path.dirname(filepath)).toUpperCase()
  const content = fs.readFileSync(filepath, 'utf8')

  console.log(`── ${basename} ── [${state}] ──────────────────────\n`)
  console.log(content)
  console.log(`Location: ${filepath}`)
}

/**
 * moveRoadmap — move arquivo para diretório do estado alvo.
 * Valida estado, procura arquivo em qualquer estado (case-insensitive partial match),
 * move com fs.renameSync, chama appendTransitionLog, imprime confirmação.
 */
function moveRoadmap(name, state) {
  const targetDir = VALID_STATES[state]
  if (!targetDir) {
    console.error(`invalid state "${state}" — valid states: backlog, wip, blocked, done, abandoned`)
    process.exitCode = 1
    return
  }

  const matches = findRoadmapMatches(name)
  if (matches.length === 0) {
    console.error(`roadmap "${name}" not found in any state directory`)
    process.exitCode = 1
    return
  }
  if (matches.length > 1) {
    console.log('Multiple roadmaps found — be more specific:')
    for (const m of matches) {
      console.log(`  ${m}`)
    }
    console.error(`ambiguous match for "${name}"`)
    process.exitCode = 1
    return
  }

  const src = matches[0]
  const basename = path.basename(src)
  const fromState = path.basename(path.dirname(src))

  try {
    fs.mkdirSync(targetDir, { recursive: true })
  } catch (_) {}

  const dst = path.join(targetDir, basename)
  fs.renameSync(src, dst)

  appendTransitionLog(basename, fromState, state)
  console.log(`✓ moved ${basename} → ${targetDir}`)
}

/**
 * appendTransitionLog — append em docs/roadmaps/.trackfw-log.
 * Formato: `YYYY-MM-DD HH:mm  <basename padded to 50>  <fromState> → <toState>\n`
 */
function appendTransitionLog(basename, fromState, toState) {
  const now = new Date()
  const yyyy = now.getFullYear()
  const mm = String(now.getMonth() + 1).padStart(2, '0')
  const dd = String(now.getDate()).padStart(2, '0')
  const hh = String(now.getHours()).padStart(2, '0')
  const min = String(now.getMinutes()).padStart(2, '0')
  const timestamp = `${yyyy}-${mm}-${dd} ${hh}:${min}`
  const line = `${timestamp}  ${basename.padEnd(50)}  ${fromState} → ${toState}\n`

  try {
    fs.mkdirSync(path.dirname(TRANSITION_LOG_PATH), { recursive: true })
    fs.appendFileSync(TRANSITION_LOG_PATH, line, 'utf8')
  } catch (_) {}
}

/**
 * newRoadmap — cria roadmap em docs/roadmaps/backlog/ROADMAP-YYYY-MM-DD-<slug>.md.
 */
function newRoadmap(title, reqPath) {
  const now = new Date()
  const yyyy = now.getFullYear()
  const mm = String(now.getMonth() + 1).padStart(2, '0')
  const dd = String(now.getDate()).padStart(2, '0')
  const date = `${yyyy}-${mm}-${dd}`
  const slug = toSlug(title)
  const filename = `docs/roadmaps/backlog/ROADMAP-${date}-${slug}.md`

  fs.mkdirSync('docs/roadmaps/backlog', { recursive: true })

  const body = `# Roadmap: ${title}

> Created: ${date} | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: ${reqPath || ''}

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — ${title}
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
`

  fs.writeFileSync(filename, body, 'utf8')
  console.log(`✓ created ${filename}`)
}

// --- helpers ---

/**
 * findRoadmapMatches — retorna array de paths que contêm `name` (case-insensitive) em qualquer estado.
 */
function findRoadmapMatches(name) {
  const matches = []
  const nameLower = name.toLowerCase()
  for (const state of STATE_ORDER) {
    const dir = VALID_STATES[state]
    let files = []
    try {
      files = fs.readdirSync(dir)
    } catch (_) {
      continue
    }
    for (const f of files) {
      if (f.toLowerCase().includes(nameLower) && f.endsWith('.md')) {
        matches.push(path.join(dir, f))
      }
    }
  }
  return matches
}

/**
 * toSlug — converte string para slug lowercase com hífens.
 */
function toSlug(s) {
  return s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

module.exports = {
  VALID_STATES,
  listRoadmaps,
  showRoadmap,
  moveRoadmap,
  appendTransitionLog,
  newRoadmap,
}
