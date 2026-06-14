'use strict'

const fs = require('fs')
const path = require('path')
const { execSync } = require('child_process')
const config = require('../config')
const { checkTraceIds } = require('./traceid')

const STALE_WIP_DAYS = 7

// listDir retorna array de nomes de arquivo (não-diretórios) em dir.
// Retorna [] se o diretório não existir.
function listDir(dir) {
  try {
    return fs.readdirSync(dir).filter(name => {
      try {
        return !fs.statSync(path.join(dir, name)).isDirectory()
      } catch (_) {
        return false
      }
    })
  } catch (_) {
    return []
  }
}

// walkDirMd retorna basenames de todos .md recursivamente dentro de dir.
function walkDirMd(dir) {
  const results = []
  function walk(d) {
    let entries
    try { entries = fs.readdirSync(d) } catch (_) { return }
    for (const name of entries) {
      const full = path.join(d, name)
      try {
        if (fs.statSync(full).isDirectory()) { walk(full) }
        else if (name.endsWith('.md')) { results.push(name) }
      } catch (_) {}
    }
  }
  walk(dir)
  return results
}

// findAdrFile busca o basename recursivamente em todos os adrDirs configurados.
// Retorna o caminho completo se encontrado, ou null.
function findAdrFile(basename) {
  const cfg = config.load()
  for (const adrDir of cfg.adrDirs) {
    function search(d) {
      let entries
      try { entries = fs.readdirSync(d) } catch (_) { return null }
      for (const name of entries) {
        const full = path.join(d, name)
        try {
          if (fs.statSync(full).isDirectory()) {
            const r = search(full)
            if (r) return r
          } else if (name === basename) {
            return full
          }
        } catch (_) {}
      }
      return null
    }
    const found = search(adrDir)
    if (found) return found
  }
  return null
}

// gitLastModifiedTime retorna o timestamp (ms) do último commit que tocou o arquivo via git log.
// Retorna null em caso de erro ou se não houver commits.
function gitLastModifiedTime(filePath) {
  try {
    const out = execSync(`git log -1 --format=%ct -- "${filePath}"`, {
      encoding: 'utf8',
      stdio: ['pipe', 'pipe', 'pipe']
    }).trim()
    if (out) return parseInt(out, 10) * 1000  // converter para ms
  } catch (_) {}
  return null
}

// resolveReqFiles retorna array de paths completos de arquivos .md de REQs.
// Em modo by_agent percorre reqDir/<agente>/<estado>/; em modo flat varre reqDir/ diretamente.
function resolveReqFiles(cfg) {
  const reqDir = cfg.reqDir || cfg.req_dir || ''
  if (!reqDir) return []
  const namespacing = cfg.roadmapNamespacing || cfg.roadmap_namespacing || ''
  if (namespacing === 'by_agent') {
    const STATES = ['backlog', 'wip', 'blocked', 'done', 'abandoned']
    let agents = cfg.agents || []
    if (!agents.length) {
      try {
        agents = fs.readdirSync(reqDir).filter(e => {
          try { return fs.statSync(path.join(reqDir, e)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { return [] }
    }
    const files = []
    for (const agent of agents) {
      for (const state of STATES) {
        const dir = path.join(reqDir, agent, state)
        try {
          for (const name of fs.readdirSync(dir)) {
            if (name.endsWith('.md')) files.push(path.join(dir, name))
          }
        } catch (_) {}
      }
    }
    return files
  }
  // flat (comportamento anterior) — retorna paths completos
  try {
    return fs.readdirSync(reqDir)
      .filter(n => n.endsWith('.md') && !fs.statSync(path.join(reqDir, n)).isDirectory())
      .map(n => path.join(reqDir, n))
  } catch (_) { return [] }
}

// resolveWIPDirs retorna todos os diretórios wip/ conforme o modo de namespacing.
function resolveWIPDirs(cfg) {
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try {
        agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
          try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = [] }
    }
    return agents.map(agent => cfg.roadmapDir + '/' + agent + '/wip')
  }
  return [cfg.roadmapDir + '/wip']
}

// parseBlockedADRs extrai basenames de ADRs da seção "## Blocked by ADRs" de um arquivo REQ.
function parseBlockedADRs(filePath) {
  let content
  try {
    content = fs.readFileSync(filePath, 'utf8')
  } catch (_) {
    return []
  }
  const lines = content.split('\n')
  const adrs = []
  let inSection = false
  for (const line of lines) {
    if (line === '## Blocked by ADRs') {
      inSection = true
      continue
    }
    if (inSection) {
      if (line.startsWith('## ')) break
      if (line.startsWith('- ')) {
        const item = line.slice(2).trim()
        const parts = item.split(/\s+/)
        if (parts.length > 0 && parts[0].endsWith('.md')) {
          adrs.push(parts[0])
        }
      }
    }
  }
  return adrs
}

// contentHasMarker retorna true se o conteúdo contém algum dos markers sem espaço em branco após.
function contentHasMarker(content, markers) {
  for (const marker of markers) {
    if (content.includes(marker) && !content.includes(marker + ' \n')) {
      return true
    }
  }
  return false
}

// adrIsDraft verifica se <adrBasename> contém "Status: Draft" buscando recursivamente nas adrDirs.
function adrIsDraft(basename) {
  const p = findAdrFile(basename)
  if (!p) return false
  try {
    return fs.readFileSync(p, 'utf8').includes('Status: Draft')
  } catch (_) { return false }
}

// validateWIPHasREQ — roadmaps em wip/ sem marker REQ no conteúdo → violation
// Suporta modo by_agent via resolveWIPDirs.
function validateWIPHasREQ() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const violations = []
  for (const wipDir of wipDirs) {
    const entries = listDir(wipDir)
    for (const name of entries) {
      try {
        const content = fs.readFileSync(path.join(wipDir, name), 'utf8')
        if (!contentHasMarker(content, cfg.linkFields.req)) {
          violations.push(`roadmap "${name}" is in wip but has no linked REQ`)
        }
      } catch (_) {
        // ignorar erro de leitura
      }
    }
  }
  return violations
}

// validateREQsHaveADR — REQs em <reqDir>/ sem marker ADR no conteúdo → violation
function validateREQsHaveADR() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!contentHasMarker(content, cfg.linkFields.adr)) {
        violations.push(`req "${path.basename(filePath)}" has no linked ADR`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateBlockedHasREQ — roadmaps em <roadmapDir>/blocked/ sem marker REQ → violation
function validateBlockedHasREQ() {
  const cfg = config.load()
  const entries = listDir(cfg.roadmapDir + '/blocked')
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join(cfg.roadmapDir + '/blocked', name), 'utf8')
      if (!contentHasMarker(content, cfg.linkFields.req)) {
        violations.push(`roadmap "${name}" is in blocked but has no linked REQ`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateREQsHaveRoadmap — REQs sem marker Roadmap → violation
function validateREQsHaveRoadmap() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!contentHasMarker(content, cfg.linkFields.roadmap)) {
        violations.push(`req "${path.basename(filePath)}" has no linked Roadmap`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateADRsAreReferenced — ADRs em adrDirs não referenciados em nenhuma REQ → violation
function validateADRsAreReferenced() {
  const cfg = config.load()
  let adrs = []
  for (const adrDir of cfg.adrDirs) {
    adrs = adrs.concat(walkDirMd(adrDir))
  }

  const reqFiles = resolveReqFiles(cfg)
  let combined = ''
  for (const filePath of reqFiles) {
    try {
      combined += fs.readFileSync(filePath, 'utf8')
    } catch (_) {
      // ignorar
    }
  }

  const violations = []
  for (const adr of adrs) {
    if (!combined.includes(adr)) {
      violations.push(`adr "${adr}" is not referenced by any REQ`)
    }
  }
  return violations
}

// validateWIPHasAcceptanceCriteria — roadmaps wip sem bloco de critérios de aceite → violation
// Suporta modo by_agent via resolveWIPDirs.
function validateWIPHasAcceptanceCriteria() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const violations = []
  for (const wipDir of wipDirs) {
    const entries = listDir(wipDir)
    for (const name of entries) {
      try {
        const content = fs.readFileSync(path.join(wipDir, name), 'utf8')
        const hasBlock = contentHasMarker(content, cfg.acceptanceMarkers)
        if (!hasBlock) {
          violations.push(`roadmap "${name}" is in wip but has no acceptance criteria block`)
        }
      } catch (_) {
        // ignorar
      }
    }
  }
  return violations
}

// readWIPConfig lê wip_limit e wip_by_squad do trackfw.yaml no CWD.
// Retorna { limit: 1, bySquad: false } se o arquivo não existe ou campos ausentes.
function readWIPConfig() {
  const cfg = { limit: 1, bySquad: false }
  let content
  try {
    content = fs.readFileSync('trackfw.yaml', 'utf8')
  } catch (_) {
    return cfg
  }
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith('wip_limit:')) {
      const val = trimmed.slice('wip_limit:'.length).trim().split(/\s+/)[0]
      const n = parseInt(val, 10)
      if (!isNaN(n) && n > 0) cfg.limit = n
    }
    if (trimmed.startsWith('wip_by_squad:')) {
      const val = trimmed.slice('wip_by_squad:'.length).trim().split(/\s+/)[0]
      if (val === 'true') cfg.bySquad = true
    }
  }
  return cfg
}

// parseSquadFromFrontmatter extrai o valor do campo "squad:" de um arquivo markdown.
// Retorna string vazia se ausente ou vazio.
function parseSquadFromFrontmatter(filePath) {
  let content
  try {
    content = fs.readFileSync(filePath, 'utf8')
  } catch (_) {
    return ''
  }
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith('squad:')) {
      return trimmed.slice('squad:'.length).trim()
    }
  }
  return ''
}

// validateWIPLimit — verifica o WIP limit por agente, por squad ou global conforme trackfw.yaml.
// Retorna { violations: [], warnings: [] }.
function validateWIPLimit() {
  const cfg = config.load()
  const violations = []
  const warnings = []

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try {
        agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
          try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = [] }
    }
    const limit = cfg.wipLimit > 0 ? cfg.wipLimit : 1
    for (const agent of agents) {
      const entries = listDir(cfg.roadmapDir + '/' + agent + '/wip')
      if (entries.length > limit) {
        warnings.push(`${entries.length} roadmaps in wip/ for agent "${agent}" (limit: ${limit}) — consider focusing`)
      }
    }
    return { violations, warnings }
  }

  // modo flat (global ou por squad)
  let files = []
  try {
    files = fs.readdirSync(path.join(cfg.roadmapDir, 'wip'))
      .filter(f => { try { return !fs.statSync(path.join(cfg.roadmapDir, 'wip', f)).isDirectory() } catch (_) { return false } })
      .map(f => path.join(cfg.roadmapDir, 'wip', f))
  } catch (_) {
    return { violations, warnings }
  }

  const wipCfg = readWIPConfig()

  if (!wipCfg.bySquad) {
    if (files.length > wipCfg.limit) {
      warnings.push(`${files.length} roadmaps in wip/ (limit: ${wipCfg.limit}) — consider focusing`)
    }
    return { violations, warnings }
  }

  const bySquad = {}
  for (const f of files) {
    let squad = parseSquadFromFrontmatter(f)
    if (!squad) squad = '(no squad)'
    if (!bySquad[squad]) bySquad[squad] = []
    bySquad[squad].push(path.basename(f))
  }
  for (const [squad, items] of Object.entries(bySquad)) {
    if (items.length > wipCfg.limit) {
      warnings.push(`squad "${squad}" has ${items.length} roadmaps in wip/ (limit: ${wipCfg.limit})`)
    }
  }
  return { violations, warnings }
}

// validateSingleWIP — alias retrocompatível de validateWIPLimit (modo flat)
function validateSingleWIP() {
  return validateWIPLimit()
}

// validateStaleWIP — roadmaps wip com mtime >= 7 dias → warning
// Suporta modo by_agent via resolveWIPDirs.
function validateStaleWIP() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const warnings = []
  const now = Date.now()

  for (const wipDir of wipDirs) {
    let files = []
    try {
      files = fs.readdirSync(wipDir)
        .filter(f => f.endsWith('.md'))
        .map(f => path.join(wipDir, f))
    } catch (_) {
      continue
    }

    for (const filePath of files) {
      try {
        const stat = fs.statSync(filePath)
        const gitTime = gitLastModifiedTime(filePath)
        const ageMs = now - (gitTime !== null ? gitTime : stat.mtimeMs)
        const days = Math.floor(ageMs / (1000 * 60 * 60 * 24))
        if (days >= STALE_WIP_DAYS) {
          const refTime = gitTime !== null ? gitTime : stat.mtimeMs
          const lastModified = new Date(refTime).toISOString().slice(0, 10)
          const basename = path.basename(filePath)
          warnings.push(
            `roadmap/wip/${basename} has been in WIP for ${days} days (last modified ${lastModified})`
          )
        }
      } catch (_) {
        // ignorar
      }
    }
  }
  return warnings
}

// validateREQsNotBlockedByDraftADRs — REQs Open com ADRs Draft na seção "## Blocked by ADRs" → violation
function validateREQsNotBlockedByDraftADRs() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    let content
    try {
      content = fs.readFileSync(filePath, 'utf8')
    } catch (_) {
      continue
    }
    if (!content.includes('Status: Open')) continue

    const blockedADRs = parseBlockedADRs(filePath)
    for (const adrBasename of blockedADRs) {
      if (adrIsDraft(adrBasename)) {
        violations.push(`REQ ${path.basename(filePath)} is blocked by Draft ADR: ${adrBasename}`)
      }
    }
  }
  return violations
}

// blockedREQs retorna mapa de reqBasename → [adrBasenames Draft] para uso em getStatus()
function blockedREQs() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const result = {}
  for (const filePath of files) {
    let content
    try {
      content = fs.readFileSync(filePath, 'utf8')
    } catch (_) {
      continue
    }
    if (!content.includes('Status: Open')) continue

    const adrNames = parseBlockedADRs(filePath)
    const draftADRs = adrNames.filter(a => adrIsDraft(a))
    if (draftADRs.length > 0) {
      result[path.basename(filePath)] = draftADRs
    }
  }
  return result
}

// readGovernanceMode lê governance_mode e lenient_until do trackfw.yaml no CWD.
// Retorna { mode: 'strict', lenientUntil: null } se o arquivo não existe ou campos ausentes.
function readGovernanceMode() {
  let content
  try {
    content = fs.readFileSync('trackfw.yaml', 'utf8')
  } catch (_) {
    return { mode: 'strict', lenientUntil: null }
  }
  let mode = 'strict'
  let lenientUntil = null
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith('governance_mode:')) {
      const val = trimmed.slice('governance_mode:'.length).trim().split(/\s+/)[0]
      if (val) mode = val
    }
    if (trimmed.startsWith('lenient_until:')) {
      const val = trimmed.slice('lenient_until:'.length).trim().split(/\s+/)[0]
      if (val) {
        const d = new Date(val)
        if (!isNaN(d.getTime())) lenientUntil = d
      }
    }
  }
  return { mode, lenientUntil }
}

// isLenient retorna true se o projeto está em modo lenient e o prazo não expirou.
function isLenient() {
  const gm = readGovernanceMode()
  if (gm.mode !== 'lenient') return false
  if (!gm.lenientUntil) return true
  return new Date() < gm.lenientUntil
}

// lenientUntilDate retorna a data de expiração formatada (YYYY-MM-DD) ou ''.
function lenientUntilDate() {
  const gm = readGovernanceMode()
  if (gm.mode !== 'lenient' || !gm.lenientUntil) return ''
  return gm.lenientUntil.toISOString().slice(0, 10)
}

// validateFrontmatterPresence — verifica presença de frontmatter em ADRs e REQs
function validateFrontmatterPresence() {
  const cfg = config.load()
  const violations = []

  for (const adrDir of cfg.adrDirs) {
    for (const f of walkDirMd(adrDir)) {
      const fullPath = findAdrFile(f)
      if (!fullPath) continue
      try {
        const content = fs.readFileSync(fullPath, 'utf8')
        if (!content.startsWith('---')) {
          violations.push(`adr "${f}" has no frontmatter block`)
        }
      } catch (_) {}
    }
  }

  const reqFilePaths = resolveReqFiles(cfg)
  for (const filePath of reqFilePaths) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!content.startsWith('---')) {
        violations.push(`req "${path.basename(filePath)}" has no frontmatter block`)
      }
    } catch (_) {}
  }

  return violations
}

// extractRefPath extrai o valor de um campo (ex: "REQ", "ADR", "Roadmap") que aponta para .md
function extractRefPath(content, field) {
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const prefix = field + ':'
    if (trimmed.startsWith(prefix)) {
      let val = trimmed.slice(prefix.length).trim()
      if (!val || val === '—' || val === '-' || val === '–') return null
      val = val.split(/\s+/)[0]
      if (val.endsWith('.md')) return val
    }
  }
  return null
}

// validateRefTargetsExist — verifica se os arquivos referenciados em REQ:, ADR: e Roadmap: existem
function validateRefTargetsExist() {
  const cfg = config.load()
  const warnings = []

  // Roadmaps em wip e blocked: verificar REQ:
  const dirs = [...resolveWIPDirs(cfg), cfg.roadmapDir + '/blocked']
  for (const dir of dirs) {
    for (const name of listDir(dir)) {
      try {
        const content = fs.readFileSync(path.join(dir, name), 'utf8')
        const ref = extractRefPath(content, 'REQ')
        if (ref && !fs.existsSync(ref)) {
          warnings.push(`roadmap "${name}" links to REQ "${ref}" which does not exist`)
        }
      } catch (_) {}
    }
  }

  // REQs: verificar ADR: e Roadmap:
  for (const filePath of resolveReqFiles(cfg)) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      const name = path.basename(filePath)
      const adrRef = extractRefPath(content, 'ADR')
      if (adrRef && !fs.existsSync(adrRef)) {
        warnings.push(`req "${name}" links to ADR "${adrRef}" which does not exist`)
      }
      const roadmapRef = extractRefPath(content, 'Roadmap')
      if (roadmapRef && !fs.existsSync(roadmapRef)) {
        warnings.push(`req "${name}" links to Roadmap "${roadmapRef}" which does not exist`)
      }
    } catch (_) {}
  }

  return warnings
}

// FOLDER_TO_STATUS mapeia pasta de estado para os valores válidos de status no frontmatter
const FOLDER_TO_STATUS = {
  wip:       ['WIP', 'wip', 'In Progress'],
  backlog:   ['Backlog', 'backlog'],
  blocked:   ['Blocked', 'blocked'],
  done:      ['Done', 'done'],
  abandoned: ['Abandoned', 'abandoned'],
}

// validateFolderStatusCoherence — verifica se o status declarado no frontmatter condiz com a pasta
function validateFolderStatusCoherence() {
  const cfg = config.load()
  const warnings = []
  const states = ['wip', 'backlog', 'blocked', 'done', 'abandoned']

  let dirs = []
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try { agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
        try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
      }) } catch (_) { agents = [] }
    }
    for (const agent of agents) {
      for (const state of states) {
        dirs.push({ dir: path.join(cfg.roadmapDir, agent, state), state })
      }
    }
  } else {
    for (const state of states) {
      dirs.push({ dir: path.join(cfg.roadmapDir, state), state })
    }
  }

  for (const { dir, state } of dirs) {
    for (const name of listDir(dir).filter(f => f.endsWith('.md'))) {
      try {
        const content = fs.readFileSync(path.join(dir, name), 'utf8')
        // Extrair status do frontmatter
        let declared = ''
        if (content.startsWith('---')) {
          const end = content.indexOf('\n---', 3)
          if (end > 0) {
            for (const line of content.slice(3, end).split('\n')) {
              const t = line.trim()
              if (t.startsWith('status:')) {
                declared = t.slice('status:'.length).trim().replace(/['"]/g, '')
                break
              }
            }
          }
        }
        if (!declared) continue
        const expected = FOLDER_TO_STATUS[state] || []
        if (!expected.some(e => e.toLowerCase() === declared.toLowerCase())) {
          warnings.push(`roadmap "${name}": folder is "${state}" but status declares "${declared}"`)
        }
      } catch (_) {}
    }
  }
  return warnings
}

// validateFilenameUniqueness — verifica que o mesmo filename não aparece em múltiplos estados
function validateFilenameUniqueness() {
  const cfg = config.load()
  const states = ['wip', 'backlog', 'blocked', 'done', 'abandoned']
  const seen = {}  // filename → [states]

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try { agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
        try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
      }) } catch (_) { agents = [] }
    }
    for (const agent of agents) {
      for (const state of states) {
        for (const name of listDir(path.join(cfg.roadmapDir, agent, state))) {
          const key = agent + '/' + name
          if (!seen[key]) seen[key] = []
          seen[key].push(state)
        }
      }
    }
  } else {
    for (const state of states) {
      for (const name of listDir(path.join(cfg.roadmapDir, state))) {
        if (!seen[name]) seen[name] = []
        seen[name].push(state)
      }
    }
  }

  const violations = []
  for (const [name, stateList] of Object.entries(seen)) {
    if (stateList.length > 1) {
      violations.push(`roadmap "${name}" appears in multiple states: [${stateList.join(', ')}]`)
    }
  }
  return violations
}

// _itemMeta: mapa de message → { rule, file } para enriquecer saída JSON.
// Populado em applyRule e nos pushs diretos do validateUnfiltered.
// Permanece em memória apenas durante a execução de uma chamada validate*.
const _itemMeta = new Map()

// _setMeta registra metadados de rule/file para uma mensagem.
function _setMeta(msg, ruleName) {
  const m = /"([^"]+)"/.exec(msg)
  _itemMeta.set(msg, { rule: ruleName, file: m ? m[1] : '' })
}

// getItemMeta retorna { rule, file } para uma mensagem, ou { rule: '', file: '' } se ausente.
function getItemMeta(msg) {
  return _itemMeta.get(msg) || { rule: '', file: '' }
}

// resetMeta limpa o mapa entre execuções (usado internamente).
function resetMeta() {
  _itemMeta.clear()
}

// ruleSeverity retorna a severidade configurada para uma regra ('error'|'warning'|'off').
function ruleSeverity(name) {
  const cfg = config.load()
  return cfg.rules[name] || 'error'
}

// applyRule distribui msgs para violations ou warnings conforme a severidade configurada.
// Se severidade for 'off', descarta silenciosamente.
// Também popula _itemMeta com rule/file para cada mensagem aceita.
function applyRule(ruleName, msgs, violations, warnings) {
  if (!msgs || msgs.length === 0) return
  const severity = ruleSeverity(ruleName)
  if (severity === 'off') return
  if (severity === 'warning') {
    for (const msg of msgs) { _setMeta(msg, ruleName); warnings.push(msg) }
  } else {
    for (const msg of msgs) { _setMeta(msg, ruleName); violations.push(msg) }
  }
}

const BASELINE_FILE = '.trackfw-baseline.json'

// loadBaseline carrega o baseline do arquivo .trackfw-baseline.json.
// Retorna null se o arquivo não existir.
function loadBaseline() {
  try {
    const data = fs.readFileSync(BASELINE_FILE, 'utf8')
    return JSON.parse(data)
  } catch (e) {
    if (e.code === 'ENOENT') return null
    throw new Error(`Erro ao ler baseline: ${e.message}`)
  }
}

// saveBaseline salva snapshot de violations e warnings em .trackfw-baseline.json.
function saveBaseline(violations, warnings) {
  const bf = {
    created: new Date().toISOString(),
    violations,
    warnings,
  }
  fs.writeFileSync(BASELINE_FILE, JSON.stringify(bf, null, 2), 'utf8')
}

// validateUnfiltered executa todas as validações e retorna { violations, warnings } sem ratchet.
async function validateUnfiltered() {
  resetMeta()
  const wipLimitResult = validateWIPLimit()
  const violations = []
  const warnings = []

  // Regras com severidade configurável via applyRule (popula _itemMeta automaticamente)
  applyRule('wip_has_req',          validateWIPHasREQ(),                   violations, warnings)
  applyRule('wip_acceptance',       validateWIPHasAcceptanceCriteria(),    violations, warnings)
  applyRule('wip_limit',            wipLimitResult.violations,             violations, warnings)
  applyRule('adr_orphan',           validateADRsAreReferenced(),           violations, warnings)
  applyRule('stale_wip',            validateStaleWIP(),                    violations, warnings)
  applyRule('ref_targets_exist',    validateRefTargetsExist(),             violations, warnings)
  applyRule('folder_status',        validateFolderStatusCoherence(),       violations, warnings)
  applyRule('filename_uniqueness',  validateFilenameUniqueness(),          violations, warnings)
  applyRule('blocked_by_draft_adr', validateREQsNotBlockedByDraftADRs(),  violations, warnings)

  // Regras configuráveis via applyRule (popula _itemMeta automaticamente)
  applyRule('req_has_adr',          validateREQsHaveADR(),          violations, warnings)
  applyRule('blocked_has_req',      validateBlockedHasREQ(),        violations, warnings)
  applyRule('req_has_roadmap',      validateREQsHaveRoadmap(),      violations, warnings)

  // Regra direta (sem configuração de severidade): violation sempre
  for (const msg of validateFrontmatterPresence())  { _setMeta(msg, 'frontmatter_presence'); violations.push(msg) }

  // warnings diretos do WIP limit (não configuráveis)
  for (const msg of wipLimitResult.warnings) { _setMeta(msg, 'wip_limit'); warnings.push(msg) }

  // Verificação bidirecional de trace ID (somente se traceIdField configurado)
  const cfg = config.load()
  if (cfg.traceIdField) {
    for (const msg of checkTraceIds(cfg.reqDir, cfg.roadmapDir, cfg.traceIdField)) {
      // O prefixo da mensagem traceid já carrega o nome da regra (ex: "traceid_orphan_roadmap: ...")
      const ruleName = msg.split(':')[0].trim()
      _setMeta(msg, ruleName)
      violations.push(msg)
    }
  }

  return { violations, warnings }
}

// validate executa todas as validações, aplica ratchet (baseline) e modo lenient.
// Retorna { violations, warnings }.
async function validate() {
  const result = await validateUnfiltered()
  let { violations, warnings } = result

  // Ratchet: filtrar violations e warnings que já estavam no baseline
  const baseline = loadBaseline()
  if (baseline) {
    const baselineSet = new Set(baseline.violations || [])
    violations = violations.filter(v => !baselineSet.has(v))
    const baselineWarnSet = new Set(baseline.warnings || [])
    warnings = warnings.filter(w => !baselineWarnSet.has(w))
  }

  // Modo lenient: mover violations para warnings, exit code 0
  if (isLenient()) {
    warnings = [...warnings, ...violations]
    violations = []
  }

  return { violations, warnings }
}

// getStatus retorna string formatada com o status de governança do projeto
async function getStatus() {
  const cfg = config.load()
  let out = '── trackfw status ──────────────────────\n'

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try {
        agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
          try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = [] }
    }
    out += '\n⚙ WIP by Agent\n'
    for (const agent of agents) {
      const wip = listDir(cfg.roadmapDir + '/' + agent + '/wip')
      if (wip.length > 0) {
        out += `  [${agent}] WIP (${wip.length})\n`
        wip.forEach(f => { out += `    ${f}\n` })
      }
    }
  } else {
    const wip = listDir(cfg.roadmapDir + '/wip')
    const blocked = listDir(cfg.roadmapDir + '/blocked')
    const done = listDir(cfg.roadmapDir + '/done')

    out += `\n🔄 WIP (${wip.length})\n`
    for (const f of wip) out += `   ${f}\n`

    const wipCfg = readWIPConfig()
    if (wipCfg.bySquad && wip.length > 0) {
      const bySquad = {}
      for (const f of wip) {
        let squad = parseSquadFromFrontmatter(path.join(cfg.roadmapDir, 'wip', f))
        if (!squad) squad = '(no squad)'
        bySquad[squad] = (bySquad[squad] || 0) + 1
      }
      out += `\n⚙ WIP by Squad (limit: ${wipCfg.limit} per squad)\n`
      for (const [squad, count] of Object.entries(bySquad)) {
        const status = count > wipCfg.limit ? '⚠' : '✓'
        const noun = count === 1 ? 'roadmap' : 'roadmaps'
        out += `   ${(squad + ':').padEnd(20)} ${count} ${noun}  ${status}\n`
      }
    }

    out += `\n❌ Blocked (${blocked.length})\n`
    for (const f of blocked) out += `   ${f}\n`

    const staleWIPs = validateStaleWIP()
    if (staleWIPs.length > 0) {
      out += `\n⚠  Stale WIP (${staleWIPs.length})\n`
      for (const w of staleWIPs) out += `   ${w}\n`
    }

    const blockedByDraft = blockedREQs()
    const blockedKeys = Object.keys(blockedByDraft)
    if (blockedKeys.length > 0) {
      out += `\n⏳ REQs blocked by Draft ADRs (${blockedKeys.length})\n`
      for (const reqFile of blockedKeys) {
        out += `   ${reqFile}\n`
        for (const adr of blockedByDraft[reqFile]) {
          out += `     → ${adr} (Draft)\n`
        }
      }
    }

    out += `\n✅ Done (last 5)\n`
    const last5 = done.length > 5 ? done.slice(done.length - 5) : done
    for (const f of last5) out += `   ${f}\n`
  }

  out += '\n────────────────────────────────────────\n'
  return out
}

module.exports = {
  validate,
  validateUnfiltered,
  loadBaseline,
  saveBaseline,
  getStatus,
  isLenient,
  lenientUntilDate,
  // exportadas para testes unitários
  validateWIPHasREQ,
  validateREQsHaveADR,
  validateBlockedHasREQ,
  validateREQsHaveRoadmap,
  validateADRsAreReferenced,
  validateWIPHasAcceptanceCriteria,
  validateWIPLimit,
  validateSingleWIP,
  validateStaleWIP,
  validateREQsNotBlockedByDraftADRs,
  parseBlockedADRs,
  adrIsDraft,
  listDir,
  resolveReqFiles,
  resolveWIPDirs,
  readGovernanceMode,
  readWIPConfig,
  parseSquadFromFrontmatter,
  validateFrontmatterPresence,
  // novas funções ML-1B
  walkDirMd,
  findAdrFile,
  gitLastModifiedTime,
  extractRefPath,
  validateRefTargetsExist,
  validateFolderStatusCoherence,
  validateFilenameUniqueness,
  // novas funções ML-2B
  contentHasMarker,
  ruleSeverity,
  applyRule,
  // novas funções ML-1B (v2.5.1)
  getItemMeta,
  resetMeta,
}
