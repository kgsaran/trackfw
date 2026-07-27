'use strict'
const fs = require('fs')
const path = require('path')
const config = require('../config')
const { localDateISO } = require('./date')

const VALID_STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']
const STATE_ORDER = ['analyzing', 'wip', 'backlog', 'blocked', 'done', 'abandoned']
const VALID_STATES_MESSAGE = VALID_STATES.join(', ')

// stateDir retorna o caminho do diretório para um estado válido no modo flat, ou null se inválido.
function stateDir(state) {
  const cfg = config.load()
  if (!VALID_STATES.includes(state)) return null
  return cfg.roadmapDir + '/' + state
}

// agentStateDir retorna o diretório para um agente+estado em modo by_agent.
// agent=null usa o primeiro agente configurado (ou "default" se lista vazia).
function agentStateDir(agent, state) {
  const cfg = config.load()
  if (!VALID_STATES.includes(state)) return null
  if (!agent) {
    agent = cfg.agents && cfg.agents.length > 0 ? cfg.agents[0] : 'default'
  }
  return cfg.roadmapDir + '/' + agent + '/' + state
}

// logPath retorna o caminho do arquivo de log de transições.
function logPath() {
  return config.load().roadmapDir + '/.trackfw-log'
}

/**
 * listRoadmaps — lista roadmaps agrupados por estado (e por agente em modo by_agent).
 * Se nenhum encontrado imprime mensagem orientando o usuário.
 */
function listRoadmaps() {
  const cfg = config.load()
  let found = false

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try {
        agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
          try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = [] }
    }
    for (const agent of agents) {
      for (const state of STATE_ORDER) {
        const dir = cfg.roadmapDir + '/' + agent + '/' + state
        let files = []
        try {
          files = fs.readdirSync(dir).filter(f => {
            try { return !fs.statSync(path.join(dir, f)).isDirectory() && f.endsWith('.md') } catch (_) { return false }
          })
        } catch (_) { continue }
        if (files.length === 0) continue
        found = true
        console.log(`[${agent}/${state}]`)
        for (const f of files) console.log(`  ${f}`)
      }
    }
  } else {
    for (const state of STATE_ORDER) {
      const dir = cfg.roadmapDir + '/' + state
      let files = []
      try {
        files = fs.readdirSync(dir).filter(f => {
          try { return !fs.statSync(path.join(dir, f)).isDirectory() && f.endsWith('.md') } catch (_) { return false }
        })
      } catch (_) { continue }
      if (files.length === 0) continue
      found = true
      console.log(`[${state}]`)
      for (const f of files) console.log(`  ${f}`)
    }
  }

  if (!found) {
    console.log("Nenhum roadmap encontrado. Crie um com 'trackfw roadmap new'.")
  }
}

/**
 * showRoadmap — busca <roadmapDir>/ESTADO/NOME*.md (partial match, flat) ou
 * <roadmapDir>/AGENTE/ESTADO/NOME*.md (by_agent), imprime cabeçalho + conteúdo.
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
    for (const m of matches) console.log(`  ${m}`)
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
 * rewriteRoadmapStatus — reescreve o campo "status:" no bloco de frontmatter e a
 * linha "| Status: <valor>" do cabeçalho no corpo.
 *
 * Espelha a semântica de rewriteFrontmatterFields (npm/src/integrations/render.js):
 * - Escopo estrito ao bloco de frontmatter (entre "---\n" de abertura e "\n---" de fechamento).
 * - Demais linhas preservadas byte a byte (ordem, espaçamento, estilo de aspas).
 * - A chave NÃO é inventada se ausente; source é devolvida inalterada.
 * - Sem frontmatter reconhecível → source é devolvida inalterada.
 *
 * A sincronização do "| Status: " no corpo é escopada: apenas a primeira ocorrência
 * antes do primeiro "## " heading é atualizada.
 *
 * Retorna { content: string, changed: boolean }.
 */
function rewriteRoadmapStatus(source, state) {
  const s = String(source)
  if (!s.startsWith('---\n')) return { content: s, changed: false }
  const end = s.indexOf('\n---', 4)
  if (end < 0) return { content: s, changed: false }

  const frontmatter = s.slice(4, end)
  const rest = s.slice(end) // starts with "\n---"

  let changed = false
  const fmLines = frontmatter.split('\n')
  for (let i = 0; i < fmLines.length; i++) {
    const sep = fmLines[i].indexOf(':')
    if (sep < 0) continue
    const key = fmLines[i].slice(0, sep).trim()
    if (key !== 'status') continue
    const value = fmLines[i].slice(sep + 1).trim()
    const quoted = value.length >= 2 && value.startsWith('"') && value.endsWith('"')
    const newLine = quoted ? `${fmLines[i].slice(0, sep)}: "${state}"` : `${fmLines[i].slice(0, sep)}: ${state}`
    if (fmLines[i] !== newLine) {
      fmLines[i] = newLine
      changed = true
    }
    break // only the first status: in frontmatter
  }

  // Sync "| Status: <valor>" in the header line (body, after the closing ---).
  // Only the first occurrence before the first "## " heading is updated.
  let newRest = rest
  if (rest.length > 4) {
    const body = rest.slice(4) // skip "\n---"
    const bodyLines = body.split('\n')
    const marker = '| Status: '
    for (let i = 0; i < bodyLines.length; i++) {
      if (bodyLines[i].trimStart().startsWith('## ')) break
      const idx = bodyLines[i].indexOf(marker)
      if (idx < 0) continue
      const prefix = bodyLines[i].slice(0, idx + marker.length)
      const after = bodyLines[i].slice(idx + marker.length)
      const pipeIdx = after.indexOf(' |')
      const suffix = pipeIdx >= 0 ? after.slice(pipeIdx) : ''
      const newLine = prefix + state + suffix
      if (bodyLines[i] !== newLine) {
        bodyLines[i] = newLine
        changed = true
        newRest = '\n---' + bodyLines.join('\n')
      }
      break // only the first | Status: before ##
    }
  }

  if (!changed) return { content: s, changed: false }
  return { content: '---\n' + fmLines.join('\n') + newRest, changed: true }
}

/**
 * moveRoadmap — move arquivo para diretório do estado alvo.
 * Em modo by_agent, mantém o agente na hierarquia.
 */
function moveRoadmap(name, state) {
  const cfg = config.load()
  if (!VALID_STATES.includes(state)) {
    console.error(`invalid state "${state}" — valid states: ${VALID_STATES_MESSAGE}`)
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
    for (const m of matches) console.log(`  ${m}`)
    console.error(`ambiguous match for "${name}"`)
    process.exitCode = 1
    return
  }

  const src = matches[0]
  const basename = path.basename(src)
  let targetDir, fromState, logBasename

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    const agentDir = path.dirname(path.dirname(src))
    const agent = path.basename(agentDir)
    fromState = path.basename(path.dirname(src))
    targetDir = agentStateDir(agent, state)
    if (!targetDir) {
      console.error(`invalid state "${state}" — valid states: ${VALID_STATES_MESSAGE}`)
      process.exitCode = 1
      return
    }
    logBasename = agent + '/' + basename
  } else {
    fromState = path.basename(path.dirname(src))
    targetDir = stateDir(state)
    if (!targetDir) {
      console.error(`invalid state "${state}" — valid states: ${VALID_STATES_MESSAGE}`)
      process.exitCode = 1
      return
    }
    logBasename = basename
  }

  try { fs.mkdirSync(targetDir, { recursive: true }) } catch (_) {}

  const dst = path.join(targetDir, basename)
  fs.renameSync(src, dst)

  // Sincroniza status: no frontmatter (e cabeçalho no corpo) com o novo estado.
  try {
    const rawContent = fs.readFileSync(dst, 'utf8')
    const { content: updated, changed } = rewriteRoadmapStatus(rawContent, state)
    if (changed) fs.writeFileSync(dst, updated, 'utf8')
  } catch (_) {}

  appendTransitionLog(logBasename, fromState, state)
  console.log(`✓ moved ${basename} → ${targetDir}`)
}

/**
 * appendTransitionLog — append em <roadmapDir>/.trackfw-log.
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
    const lp = logPath()
    fs.mkdirSync(path.dirname(lp), { recursive: true })
    fs.appendFileSync(lp, line, 'utf8')
  } catch (_) {}
}

/**
 * newRoadmap — cria roadmap em <roadmapDir>/backlog/ROADMAP-YYYY-MM-DD-<slug>.md.
 * Em modo by_agent, usa o primeiro agente configurado.
 */
function newRoadmap(title, reqPath) {
  const cfg = config.load()
  const date = localDateISO()
  const slug = toSlug(title)

  let backlogDir
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    backlogDir = agentStateDir(null, 'backlog')
    if (!backlogDir) {
      console.error('cannot resolve backlog dir in by_agent mode')
      process.exitCode = 1
      return
    }
  } else {
    backlogDir = cfg.roadmapDir + '/backlog'
  }

  const filename = `${backlogDir}/ROADMAP-${date}-${slug}.md`
  fs.mkdirSync(backlogDir, { recursive: true })

  const body = `---
status: backlog
date: ${date}
req: ""
squad: ""
---

# Roadmap: ${title}

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

/**
 * newRoadmapFromReq — lê uma REQ e gera roadmap pré-preenchido com MLs extraídos
 * dos critérios de aceite.
 */
function newRoadmapFromReq(reqPath) {
  let data
  try {
    data = fs.readFileSync(reqPath, 'utf8')
  } catch (err) {
    console.error(`reading REQ: ${err.message}`)
    process.exitCode = 1
    return
  }

  const { title: parsedTitle, criteria, linkedADR } = parseReqForRoadmap(data)
  const basename = path.basename(reqPath)
  const title = parsedTitle || basename.replace(/\.md$/, '').replace(/^REQ-/, '')

  const cfg = config.load()
  const date = localDateISO()
  const slug = toSlug(title)

  let backlogDir
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    backlogDir = agentStateDir(null, 'backlog')
    if (!backlogDir) {
      console.error('cannot resolve backlog dir in by_agent mode')
      process.exitCode = 1
      return
    }
  } else {
    backlogDir = cfg.roadmapDir + '/backlog'
  }

  const filename = `${backlogDir}/ROADMAP-${date}-${slug}.md`
  try { fs.mkdirSync(backlogDir, { recursive: true }) } catch (_) {}

  // Gerar seção de MLs a partir dos critérios de aceite
  const mlLines = ['## Wave 1 — Implementation (derived from REQ criteria)', '> Dependencies: none']
  for (let i = 0; i < criteria.length; i++) {
    const mlLabel = `ML-1${String.fromCharCode(65 + i)}`
    const crit = criteria[i]
    mlLines.push(`\n### ${mlLabel} — ${crit}`)
    mlLines.push('**Status:** pending')
    mlLines.push('**Files affected:**')
    mlLines.push('**Actions:**')
    mlLines.push('**Acceptance criteria:**')
    mlLines.push(`- [ ] ${crit}`)
    mlLines.push('- [ ] build passes')
    mlLines.push('- [ ] tests green')
  }
  const mlSection = mlLines.join('\n')

  const adrRef = linkedADR ? `\nADR: ${linkedADR}` : ''

  const body = `---
status: backlog
date: ${date}
req: "${basename}"
squad: ""
---

# Roadmap: ${title}

> Created: ${date} | Status: backlog

## Context
<!-- Derived from REQ: ${basename} -->
REQ: ${reqPath}${adrRef}

${mlSection}
`

  fs.writeFileSync(filename, body, 'utf8')
  console.log(`✓ created ${filename}`)
}

/**
 * parseReqForRoadmap — extrai título, critérios de aceite e ADR linkada de conteúdo REQ.
 */
function parseReqForRoadmap(content) {
  const lines = content.split('\n')
  let title = ''
  let linkedADR = ''
  const criteria = []
  let inCriteria = false

  for (const line of lines) {
    if (line.startsWith('# REQ: ')) {
      title = line.replace('# REQ: ', '').trim()
      continue
    }
    if (line.startsWith('# REQ — ')) {
      title = line.replace('# REQ — ', '').trim()
      continue
    }
    if (line.startsWith('# REQ - ')) {
      title = line.replace('# REQ - ', '').trim()
      continue
    }
    if (line.startsWith('**ADR:**')) {
      linkedADR = line.replace('**ADR:**', '').trim()
      continue
    }

    const lower = line.trim().toLowerCase()
    if (lower === '## critérios de aceite' || lower === '## acceptance criteria') {
      inCriteria = true
      continue
    }
    if (inCriteria && line.startsWith('## ')) {
      inCriteria = false
      continue
    }
    if (inCriteria) {
      const trimmed = line.trim()
      const checkboxPrefixes = ['- [ ]', '- [x]', '- [X]']
      for (const prefix of checkboxPrefixes) {
        if (trimmed.startsWith(prefix)) {
          const item = trimmed.slice(prefix.length).trim().replace(/`/g, '')
          if (item) criteria.push(item)
          break
        }
      }
    }
  }
  return { title, criteria, linkedADR }
}

// --- helpers ---

/**
 * findRoadmapMatches — retorna array de paths que contêm `name` (case-insensitive) em qualquer estado.
 * Suporta modo flat (1 nível) e by_agent (2 níveis).
 */
function findRoadmapMatches(name) {
  const cfg = config.load()
  const matches = []
  const nameLower = name.toLowerCase()

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try {
        agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
          try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = ['default'] }
    }
    for (const agent of agents) {
      for (const state of STATE_ORDER) {
        const dir = cfg.roadmapDir + '/' + agent + '/' + state
        let files = []
        try { files = fs.readdirSync(dir) } catch (_) { continue }
        for (const f of files) {
          if (f.toLowerCase().includes(nameLower) && f.endsWith('.md')) {
            matches.push(path.join(dir, f))
          }
        }
      }
    }
  } else {
    for (const state of STATE_ORDER) {
      const dir = cfg.roadmapDir + '/' + state
      let files = []
      try { files = fs.readdirSync(dir) } catch (_) { continue }
      for (const f of files) {
        if (f.toLowerCase().includes(nameLower) && f.endsWith('.md')) {
          matches.push(path.join(dir, f))
        }
      }
    }
  }
  return matches
}

/**
 * toSlug — converte string para slug lowercase com hífens.
 */
function toSlug(s) {
  // NFKD normalization + remove combining marks (diacríticos) + lowercase + non-alphanumeric → hífen
  return s
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

module.exports = {
  listRoadmaps,
  showRoadmap,
  moveRoadmap,
  rewriteRoadmapStatus,
  appendTransitionLog,
  newRoadmap,
  newRoadmapFromReq,
  stateDir,
  agentStateDir,
  VALID_STATES,
  STATE_ORDER,
  toSlug,
}
