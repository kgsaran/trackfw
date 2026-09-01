'use strict'
const fs = require('fs')
const path = require('path')
const config = require('../config')
const { localDateISO } = require('./date')
const { resolveReqFiles, resolveAgentNamespaces } = require('../validator/index.js')

// STATUS_LEGEND teaches the vocabulary the `barrier` parser accepts for "**Status:**"
// (AC11, ADR decision 5): the canonical form the template now writes (⬜ Pendente) plus the
// other three states. Placed once, right before the first wave, so it is close to the first
// place a "**Status:**" line appears — not repeated per-ML (would clutter) and not left for
// the end (nobody reads that far). Byte-identical across the 3 CLIs
// (gate: scripts/check-artifact-parity.sh).
const STATUS_LEGEND = `## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

`

// wave0Block — "## Wave 0 — Threat Model" section prepended to every generated roadmap, before
// the first implementation wave (AC1, AC12). Byte-identical to internal/generators/roadmap.go's
// wave0Block/wave0GateFence and to pypi/trackfw/generators/roadmap.py's WAVE0_BLOCK (gate:
// scripts/check-artifact-parity.sh).
//
// The gate command (`exit 1`) is fixed, literal and never interpolated with any REQ title, slug
// or user-controlled string — see the Go generator's comment for the full rationale (AC13,
// docs/cli-parity.md § "trackfw barrier"). It intentionally fails closed until ML-0A replaces it
// with a real, project-specific check.
//
// The ML is always ML-0A, never ML-1A — newRoadmapFromReq labels MLs derived from REQ acceptance
// criteria "ML-1A", "ML-1B", ... starting at the first criterion.
const WAVE0_BLOCK = STATUS_LEGEND + `## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
\`\`\`bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
\`\`\`

`

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
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
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

  // Sincroniza o roadmap: das REQs que apontem para este roadmap. O valor escrito no
  // frontmatter da REQ pareada é dado portável, nunca separador nativo — normaliza antes de
  // sincronizar (dst continua nativo acima, para rename/read/write no filesystem).
  syncReqReferences(basename, normalizeRefSeparator(dst), cfg)
}

/**
 * normalizeRefSeparator — normaliza um valor já extraído (não o buffer inteiro do arquivo)
 * para o separador portável (/) antes de ele ser escrito ou comparado como referência dentro
 * de conteúdo versionado. Substituição incondicional: em POSIX, path.join nunca produz "\",
 * então isto só atua sobre um valor herdado de um commit feito no Windows — exatamente o
 * defeito que esta função existe para curar
 * (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md).
 * @param {string} p
 * @returns {string}
 */
function normalizeRefSeparator(p) {
  return p.replace(/\\/g, '/')
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
 * extractFrontmatterRoadmap — extrai o valor do campo `roadmap:` SOMENTE do frontmatter.
 * Retorna null se não encontrado ou se não termina em .md.
 * Não usa extractRefPath do validator porque este percorre todo o conteúdo (inclusive corpo)
 * sem distinguir frontmatter de body — geraria falsos positivos em REQs sem roadmap: no FM.
 */
function extractFrontmatterRoadmap(content) {
  const lines = content.split('\n')
  let inFm = false
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (i === 0 && line.trim() === '---') { inFm = true; continue }
    if (inFm && line.trim() === '---') return null // fechou frontmatter sem encontrar
    if (!inFm) return null // nunca entrou no frontmatter
    const idx = line.indexOf(':')
    if (idx === -1) continue
    if (line.slice(0, idx).trim() !== 'roadmap') continue
    let val = line.slice(idx + 1).trim()
    val = val.replace(/^["']|["']$/g, '').trim()
    if (val.endsWith('.md')) return val
    return null // roadmap: existe mas sem .md (ex: "" ou slug)
  }
  return null
}

/**
 * rewriteReqRoadmapRef — reescreve o campo `roadmap:` no frontmatter e a linha `Roadmap:` no
 * corpo de uma REQ, substituindo oldRef por newRef.
 * Preserva toda formatação existente (aspas, backticks) via substituição literal de string.
 */
function rewriteReqRoadmapRef(content, oldRef, newRef) {
  const lines = content.split('\n')
  let inFm = false
  let fmDone = false
  const result = []

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (i === 0 && line.trim() === '---') { inFm = true; result.push(line); continue }
    if (inFm && line.trim() === '---') { inFm = false; fmDone = true; result.push(line); continue }
    if (inFm) {
      const idx = line.indexOf(':')
      if (idx !== -1 && line.slice(0, idx).trim() === 'roadmap' && line.includes(oldRef)) {
        result.push(line.replace(oldRef, newRef))
        continue
      }
      result.push(line)
      continue
    }
    if (fmDone) {
      // No corpo: reescreve linhas `Roadmap:` que contenham oldRef
      if (line.trim().toLowerCase().startsWith('roadmap:') && line.includes(oldRef)) {
        result.push(line.replace(oldRef, newRef))
        continue
      }
    }
    result.push(line)
  }

  return result.join('\n')
}

/**
 * syncReqReferences — após rename bem-sucedido, varre req_dir (flat + by_agent) e reescreve o
 * frontmatter `roadmap:` e a linha `Roadmap:` do corpo em toda REQ que aponte para o roadmap
 * movido. Implementa as cinco cardinalidades do contrato:
 *   zero → no-op silencioso; uma → reescreve; várias → reescreve todas; aponta para outro →
 *   não toca; já correta → nenhuma escrita (idempotente byte-a-byte).
 * Erros: diagnóstico em stderr nomeando a REQ; tenta as restantes; exit não-zero ao fim.
 */
function syncReqReferences(movedBasename, newRoadmapPath, cfg) {
  // Ordenação explícita por basename (independente de locale) para garantir saída determinística
  // independente do filesystem. Desempate por caminho completo quando dois agentes têm REQs de
  // mesmo basename. Não confiar na ordem do readdirSync — varia entre filesystems.
  const files = resolveReqFiles(cfg).sort((a, b) => {
    const ba = path.basename(a)
    const bb = path.basename(b)
    if (ba < bb) return -1
    if (ba > bb) return 1
    if (a < b) return -1
    if (a > b) return 1
    return 0
  })
  let anyError = false

  for (const filePath of files) {
    let content
    try {
      content = fs.readFileSync(filePath, 'utf8')
    } catch (e) {
      console.error(`trackfw roadmap move: failed to sync ${path.basename(filePath)}: ${e.message}`)
      anyError = true
      continue
    }

    const currentRef = extractFrontmatterRoadmap(content)
    if (!currentRef) continue
    // path.basename sobre um valor sujo com "\" (gravado no Windows antes do fix de escrita)
    // não separa nada em POSIX — normaliza antes de comparar, senão uma REQ já suja nunca é
    // curada por um roadmap move subsequente.
    if (path.basename(normalizeRefSeparator(currentRef)) !== movedBasename) continue // aponta para outro roadmap
    if (currentRef === newRoadmapPath) {
      // Guarda rápida: frontmatter já correto — confirma idempotência com reescrita estrutural
      const updated = rewriteReqRoadmapRef(content, currentRef, newRoadmapPath)
      if (updated === content) continue // sem escrita, sem output
    }

    try {
      const updated = rewriteReqRoadmapRef(content, currentRef, newRoadmapPath)
      if (updated === content) continue // nenhuma escrita, nenhum output (idempotência byte-a-byte)
      fs.writeFileSync(filePath, updated, 'utf8')
      console.log(`✓ synced ${path.basename(filePath)} → ${newRoadmapPath}`)
    } catch (e) {
      console.error(`trackfw roadmap move: failed to sync ${path.basename(filePath)}: ${e.message}`)
      anyError = true
    }
  }

  if (anyError) process.exitCode = 1
}

/**
 * newRoadmap — cria roadmap em <roadmapDir>/backlog/ROADMAP-YYYY-MM-DD-<slug>.md.
 * Em modo by_agent, usa o primeiro agente configurado.
 */
function newRoadmap(title, reqPath) {
  // AC1/AC2: o título é dado de uma linha — newline e CR são entrada malformada.
  // Mensagem byte-idêntica nos 3 CLIs (docs/cli-parity.md).
  if (/[\n\r]/.test(title)) {
    console.error('Error: roadmap title must be a single line: newline and carriage return are not allowed')
    process.exitCode = 1
    return
  }

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

  const reqField = reqPath ? `"${reqPath}"` : '""'

  const body = `---
status: backlog
date: ${date}
req: ${reqField}
squad: ""
---

# Roadmap: ${title}

> Created: ${date} | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: ${reqPath || ''}

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

${WAVE0_BLOCK}## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — ${title}
**Status:** ⬜ Pendente
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

  // AC1: o título lido da REQ também pode conter newline forjado — rejeitar cedo.
  if (/[\n\r]/.test(title)) {
    console.error('Error: roadmap title must be a single line: newline and carriage return are not allowed')
    process.exitCode = 1
    return
  }

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
    mlLines.push('**Status:** ⬜ Pendente')
    mlLines.push('**Files affected:**')
    mlLines.push('**Actions:**')
    mlLines.push('**Acceptance criteria:**')
    mlLines.push(`- [ ] ${crit}`)
    mlLines.push('- [ ] build passes')
    mlLines.push('- [ ] tests green')
  }
  const mlSection = WAVE0_BLOCK + mlLines.join('\n')

  const adrRef = linkedADR ? `\nADR: ${linkedADR}` : ''

  const body = `---
status: backlog
date: ${date}
req: "${reqPath}"
squad: ""
---

# Roadmap: ${title}

> Created: ${date} | Status: backlog

## Context
<!-- Derived from REQ: ${basename} -->
REQ: ${reqPath}${adrRef}

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

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
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
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
  extractFrontmatterRoadmap,
  rewriteReqRoadmapRef,
  syncReqReferences,
  normalizeRefSeparator,
  newRoadmap,
  newRoadmapFromReq,
  stateDir,
  agentStateDir,
  VALID_STATES,
  STATE_ORDER,
  toSlug,
}
