'use strict'

const fs = require('fs')
const path = require('path')
const { execSync } = require('child_process')
const config = require('../config')
const { checkTraceIds } = require('./traceid')

const STALE_WIP_DAYS = 7
let staleWipNowMs = () => Date.now()

// listDir retorna array de nomes de arquivo (não-diretórios) em dir.
// Retorna [] se o diretório não existir.
function listDir(dir) {
  const expanded = config.expandPath ? config.expandPath(dir) : dir
  try {
    return fs.readdirSync(expanded).filter(name => {
      try {
        return !fs.statSync(path.join(expanded, name)).isDirectory()
      } catch (_) {
        return false
      }
    })
  } catch (_) {
    return []
  }
}

// tryListDir tenta listar o diretório distinguindo "não existe" de outros erros.
// Retorna { entries: string[], readError: Error|null }.
// readError é null tanto no caso de sucesso quanto quando o diretório não existe (ENOENT) —
// diretório ausente é esperado para estados que o projeto não usa.
// readError não-null indica que o diretório EXISTE mas não pôde ser lido (ENOTDIR, EPERM…).
function tryListDir(dir) {
  const expanded = config.expandPath ? config.expandPath(dir) : dir
  try {
    const entries = fs.readdirSync(expanded).filter(name => {
      try { return !fs.statSync(path.join(expanded, name)).isDirectory() } catch (_) { return false }
    })
    return { entries, readError: null }
  } catch (err) {
    if (err && err.code === 'ENOENT') return { entries: [], readError: null }
    return { entries: [], readError: err }
  }
}

function inspectionDiagnostic(rule, target, err) {
  const cause = err && err.message ? err.message : String(err)
  return `${rule}: could not inspect "${target}": ${cause}`
}

function listDirForRule(rule, dir, messages) {
  const { entries, readError } = tryListDir(dir)
  if (readError) messages.push(inspectionDiagnostic(rule, dir, readError))
  return entries
}

function readFileForRule(rule, filePath, messages) {
  try {
    return fs.readFileSync(filePath, 'utf8')
  } catch (err) {
    messages.push(inspectionDiagnostic(rule, filePath, err))
    return null
  }
}

// isInsideDir retorna true se childPath estiver contido ou for igual a parentDir.
function isInsideDir(parentDir, childPath) {
  if (!parentDir || !childPath) return false
  const rel = path.relative(path.resolve(parentDir), path.resolve(childPath))
  return rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel))
}

// walkDirMdWithPaths retorna { name, fullPath } de todos .md recursivamente dentro de dir.
function walkDirMdWithPaths(dir) {
  return walkDirMdWithPathsForRule(null, dir, null)
}

function walkDirMdWithPathsForRule(rule, dir, messages) {
  const results = []
  const expandedDir = config.expandPath ? config.expandPath(dir) : dir
  function walk(d) {
    let entries
    try {
      entries = fs.readdirSync(d)
    } catch (err) {
      if (messages && err && err.code !== 'ENOENT') messages.push(inspectionDiagnostic(rule, d, err))
      return
    }
    for (const name of entries) {
      const full = path.join(d, name)
      try {
        if (fs.statSync(full).isDirectory()) { walk(full) }
        else if (name.endsWith('.md')) { results.push({ name, fullPath: full }) }
      } catch (err) {
        if (messages) messages.push(inspectionDiagnostic(rule, full, err))
      }
    }
  }
  walk(expandedDir)
  return results
}

// walkDirMd retorna basenames de todos .md recursivamente dentro de dir.
function walkDirMd(dir) {
  return walkDirMdWithPaths(dir).map(item => item.name)
}

// findAdrFile busca o basename recursivamente em todos os adrDirs configurados.
// Retorna o caminho completo se encontrado, ou null.
function findAdrFile(basename) {
  const cfg = config.load()
  const adrDirs = (cfg.adrDirs || []).map(d => config.expandPath ? config.expandPath(d) : d)
  for (const adrDir of adrDirs) {
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
    const STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']
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

// resolveStateDirs retorna todos os diretórios de um estado (ex: 'wip', 'done') conforme o modo de
// namespacing. É a fonte única de resolução de caminho por estado — resolveWIPDirs e resolveDoneDirs
// são wrappers finos sobre esta função. Duplicar a lógica aqui foi a causa raiz de defeitos
// anteriores (roadmap_dir divergente entre runtimes).
function resolveStateDirs(cfg, state) {
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try {
        agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
          try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = [] }
    }
    return agents.map(agent => cfg.roadmapDir + '/' + agent + '/' + state)
  }
  return [cfg.roadmapDir + '/' + state]
}

// resolveWIPDirs retorna todos os diretórios wip/ conforme o modo de namespacing.
function resolveWIPDirs(cfg) {
  return resolveStateDirs(cfg, 'wip')
}

// resolveDoneDirs retorna todos os diretórios done/ conforme o modo de namespacing.
function resolveDoneDirs(cfg) {
  return resolveStateDirs(cfg, 'done')
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
// P3: verifica tanto "\n" quanto "\r\n" para detectar campos vazios em arquivos CRLF.
function contentHasMarker(content, markers) {
  for (const marker of markers) {
    if (content.includes(marker) && !content.includes(marker + ' \n') && !content.includes(marker + ' \r\n')) {
      return true
    }
  }
  return false
}

// adrIsDraft verifica se <adrBasename> contém "Status: Draft" buscando recursivamente nas adrDirs.
function adrIsDraft(basename) {
  return adrDraftStatusForRule(basename, null).draft
}

function adrDraftStatusForRule(basename, messages) {
  const p = findAdrFile(basename)
  if (!p) return { draft: false, inspected: true }
  try {
    return { draft: fs.readFileSync(p, 'utf8').includes('Status: Draft'), inspected: true }
  } catch (err) {
    if (messages) messages.push(inspectionDiagnostic('blocked_by_draft_adr', p, err))
    return { draft: false, inspected: false }
  }
}

// validateWIPHasREQ — roadmaps em wip/ sem marker REQ no conteúdo → violation
// Suporta modo by_agent via resolveWIPDirs.
function validateWIPHasREQ() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const violations = []
  for (const wipDir of wipDirs) {
    const entries = listDirForRule('wip_has_req', wipDir, violations)
    for (const name of entries) {
      const content = readFileForRule('wip_has_req', path.join(wipDir, name), violations)
      if (content === null) continue
      if (!contentHasMarker(content, cfg.linkFields.req)) {
        violations.push(`roadmap "${name}" is in wip but has no linked REQ`)
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
  const violations = []
  for (const blockedDir of resolveStateDirs(cfg, 'blocked')) {
    const entries = listDirForRule('blocked_has_req', blockedDir, violations)
    for (const name of entries) {
      const content = readFileForRule('blocked_has_req', path.join(blockedDir, name), violations)
      if (content === null) continue
      if (!contentHasMarker(content, cfg.linkFields.req)) {
        violations.push(`roadmap "${name}" is in blocked but has no linked REQ`)
      }
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

// validateADRDirsExist — verifica se todos adrDirs existem.
// Retorna { violations: [], warnings: [] } respeitando strictCiPaths.
function validateADRDirsExist() {
  const cfg = config.load()
  const violations = []
  const warnings = []
  for (const adrDir of cfg.adrDirs || []) {
    const expanded = config.expandPath ? config.expandPath(adrDir) : adrDir
    const absDir = path.resolve(expanded)
    if (!fs.existsSync(absDir)) {
      const msg = `adr_dir "${adrDir}" does not exist`
      if (cfg.strictCiPaths) {
        violations.push(msg)
      } else {
        warnings.push(msg)
      }
    }
  }
  return { violations, warnings }
}

// validateADRsAreReferenced — ADRs em adrDirs não referenciados em nenhuma REQ → violation.
// ADRs localizados fora do projeto local (cwd) são isentos desta validação.
function validateADRsAreReferenced() {
  const cfg = config.load()
  const cwd = process.cwd()
  const violations = []
  let adrs = []
  for (const adrDir of cfg.adrDirs || []) {
    const expanded = config.expandPath ? config.expandPath(adrDir) : adrDir
    const absDir = path.resolve(expanded)
    // Isentar diretórios fora do cwd local
    if (!isInsideDir(cwd, absDir)) {
      continue
    }
    for (const item of walkDirMdWithPathsForRule('adr_orphan', absDir, violations)) {
      if (isInsideDir(cwd, item.fullPath)) {
        adrs.push(item.name)
      }
    }
  }

  const reqFiles = resolveReqFiles(cfg)
  let combined = ''
  for (const filePath of reqFiles) {
    const content = readFileForRule('adr_orphan', filePath, violations)
    if (content !== null) combined += content
  }

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
    const entries = listDirForRule('wip_acceptance', wipDir, violations)
    for (const name of entries) {
      const content = readFileForRule('wip_acceptance', path.join(wipDir, name), violations)
      if (content === null) continue
      const hasBlock = contentHasMarker(content, cfg.acceptanceMarkers)
      if (!hasBlock) {
        violations.push(`roadmap "${name}" is in wip but has no acceptance criteria block`)
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
function roadmapLogIdentity(cfg, filePath) {
  const basename = path.basename(filePath)
  if (cfg.roadmapNamespacing !== config.NAMESPACING_BY_AGENT) return basename
  const agent = path.basename(path.dirname(path.dirname(filePath)))
  return `${agent}/${basename}`
}

function parseTransitionLogLine(line) {
  const fields = String(line).trim().split(/\s+/).filter(Boolean)
  if (fields.length < 5) return null
  const timestamp = Date.parse(`${fields[0]}T${fields[1]}:00`)
  if (Number.isNaN(timestamp)) return null
  const arrow = fields.findIndex((field, index) => index >= 3 && (field === '→' || field === '->'))
  if (arrow < 0 || arrow + 1 >= fields.length) return null
  return { timestamp, name: fields[2], toState: fields[arrow + 1] }
}

function latestWipTransitionTime(cfg, filePath) {
  let content
  const logPath = path.join(cfg.roadmapDir, '.trackfw-log')
  try {
    content = fs.readFileSync(logPath, 'utf8')
  } catch (err) {
    return { time: null, diagnostics: err && err.code === 'ENOENT' ? [] : [inspectionDiagnostic('stale_wip', logPath, err)] }
  }
  const identity = roadmapLogIdentity(cfg, filePath)
  let latest = null
  const diagnostics = []
  for (const line of content.split('\n')) {
    if (!line.trim()) continue
    const parsed = parseTransitionLogLine(line)
    if (!parsed) {
      diagnostics.push(`stale_wip: invalid support line in "${logPath}": "${line}"`)
      continue
    }
    if (parsed.name !== identity || parsed.toState !== 'wip') continue
    if (latest === null || parsed.timestamp > latest) latest = parsed.timestamp
  }
  return { time: latest, diagnostics }
}

function validateStaleWIP() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const warnings = []
  const now = staleWipNowMs()
  const thresholdDays = cfg.staleWipDays > 0 ? cfg.staleWipDays : STALE_WIP_DAYS

  for (const wipDir of wipDirs) {
    const files = listDirForRule('stale_wip', wipDir, warnings)
      .filter(f => f.endsWith('.md'))
      .map(f => path.join(wipDir, f))

    for (const filePath of files) {
      try {
        const stat = fs.statSync(filePath)
        const logResult = latestWipTransitionTime(cfg, filePath)
        warnings.push(...logResult.diagnostics)
        const refTime = logResult.time !== null ? logResult.time : stat.mtimeMs
        const ageMs = now - refTime
        const days = Math.floor(ageMs / (1000 * 60 * 60 * 24))
        if (days >= thresholdDays) {
          const lastModified = new Date(refTime).toISOString().slice(0, 10)
          const basename = path.basename(filePath)
          warnings.push(
            `roadmap/wip/${basename} has been in WIP for ${days} days (last modified ${lastModified})`
          )
        }
      } catch (err) {
        warnings.push(inspectionDiagnostic('stale_wip', filePath, err))
      }
    }
  }
  return warnings
}

function setStaleWipNowForTests(fn) {
  staleWipNowMs = fn || (() => Date.now())
}

// validateREQsNotBlockedByDraftADRs — REQs Open com ADRs Draft na seção "## Blocked by ADRs" → violation
function validateREQsNotBlockedByDraftADRs() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    const content = readFileForRule('blocked_by_draft_adr', filePath, violations)
    if (content === null) continue
    if (!content.includes('Status: Open')) continue

    const blockedADRs = parseBlockedADRs(filePath)
    for (const adrBasename of blockedADRs) {
      if (adrDraftStatusForRule(adrBasename, violations).draft) {
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
    const idx = trimmed.indexOf(':')
    if (idx !== -1 && trimmed.slice(0, idx).trim().toLowerCase() === field.toLowerCase()) {
      let val = trimmed.slice(idx + 1).trim()
      if (!val || val === '—' || val === '-' || val === '–') return null
      val = val.split(/\s+/)[0]
      val = val.replace(/^["']|["']$/g, '')
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
  const dirs = [...resolveWIPDirs(cfg), ...resolveStateDirs(cfg, 'blocked')]
  for (const dir of dirs) {
    for (const name of listDirForRule('ref_targets_exist', dir, warnings)) {
      const content = readFileForRule('ref_targets_exist', path.join(dir, name), warnings)
      if (content === null) continue
      const ref = extractRefPath(content, 'REQ')
      if (ref && !referenceExists(ref, [cfg.reqDir])) {
        warnings.push(`roadmap "${name}" links to REQ "${ref}" which does not exist`)
      }
    }
  }

  // REQs: verificar ADR: e Roadmap:
  for (const filePath of resolveReqFiles(cfg)) {
    const content = readFileForRule('ref_targets_exist', filePath, warnings)
    if (content === null) continue
    const name = path.basename(filePath)
    const adrRef = extractRefPath(content, 'ADR')
    if (adrRef && !referenceExists(adrRef, cfg.adrDirs)) {
      warnings.push(`req "${name}" links to ADR "${adrRef}" which does not exist`)
    }
    const roadmapRef = extractRefPath(content, 'Roadmap')
    if (roadmapRef && !referenceExists(roadmapRef, [cfg.roadmapDir])) {
      warnings.push(`req "${name}" links to Roadmap "${roadmapRef}" which does not exist`)
    }
  }

  return warnings
}

function referenceExists(ref, roots) {
  const expandedRef = config.expandPath ? config.expandPath(ref) : ref
  if (fs.existsSync(expandedRef)) return true
  return false
}

function validateREQRoadmapLifecycle() {
  const cfg = config.load()
  const warnings = []
  for (const filePath of resolveReqFiles(cfg)) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!reqStatusIsOpen(content)) continue
      const ref = extractRefPath(content, 'Roadmap')
      if (!ref) continue
      const expandedRef = config.expandPath ? config.expandPath(ref) : ref
      if (!fs.existsSync(expandedRef)) continue
      if (path.basename(path.dirname(expandedRef)) === 'done') {
        warnings.push(`req "${path.basename(filePath)}" is Open but linked Roadmap "${ref}" is in done/`)
      }
    } catch (_) {}
  }
  return warnings
}

function reqStatusIsOpen(content) {
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const idx = trimmed.indexOf(':')
    if (idx >= 0 && trimmed.slice(0, idx).trim().toLowerCase() === 'status') {
      return trimmed.slice(idx + 1).trim().replace(/^["']|["']$/g, '').toLowerCase() === 'open'
    }
    const marker = '| Status: '
    const markerIdx = trimmed.indexOf(marker)
    if (markerIdx >= 0) {
      let rest = trimmed.slice(markerIdx + marker.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) rest = rest.slice(0, pipeIdx)
      return rest.trim().toLowerCase() === 'open'
    }
  }
  return false
}

// FOLDER_TO_STATUS mapeia pasta de estado para os valores válidos de status no frontmatter
const FOLDER_TO_STATUS = {
  wip:       ['WIP', 'wip', 'In Progress'],
  backlog:   ['Backlog', 'backlog'],
  analyzing: ['Analyzing', 'analyzing'],
  blocked:   ['Blocked', 'blocked'],
  done:      ['Done', 'done'],
  abandoned: ['Abandoned', 'abandoned'],
}

// validateFolderStatusCoherence — verifica se o status declarado no frontmatter condiz com a pasta
function validateFolderStatusCoherence() {
  const cfg = config.load()
  const warnings = []
  const states = ['wip', 'backlog', 'analyzing', 'blocked', 'done', 'abandoned']

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
    // P2: distinguir "diretório ausente" (esperado) de outros erros (reportar).
    const { entries, readError } = tryListDir(dir)
    if (readError) {
      warnings.push(`folder_status: could not read directory "${dir}": ${readError.message}`)
      continue
    }
    for (const name of entries.filter(f => f.endsWith('.md'))) {
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
  const states = ['wip', 'backlog', 'analyzing', 'blocked', 'done', 'abandoned']
  const seen = {}  // filename → [states]

  const listErrors = []
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try { agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
        try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
      }) } catch (_) { agents = [] }
    }
    for (const agent of agents) {
      for (const state of states) {
        const dir = path.join(cfg.roadmapDir, agent, state)
        const { entries, readError } = tryListDir(dir)
        if (readError) {
          listErrors.push(`filename_uniqueness: could not read directory "${dir}": ${readError.message}`)
          continue
        }
        for (const name of entries) {
          const key = agent + '/' + name
          if (!seen[key]) seen[key] = []
          seen[key].push(state)
        }
      }
    }
  } else {
    for (const state of states) {
      const dir = path.join(cfg.roadmapDir, state)
      const { entries, readError } = tryListDir(dir)
      if (readError) {
        listErrors.push(`filename_uniqueness: could not read directory "${dir}": ${readError.message}`)
        continue
      }
      for (const name of entries) {
        if (!seen[name]) seen[name] = []
        seen[name].push(state)
      }
    }
  }

  const violations = [...listErrors]
  // P3: ordenar os estados dentro de cada mensagem e as mensagens pelo nome
  // para garantir saída determinística independente de ordem de inserção.
  const sortedNames = Object.keys(seen).sort()
  for (const name of sortedNames) {
    const stateList = seen[name]
    if (stateList.length > 1) {
      const sortedStates = [...stateList].sort()
      violations.push(`roadmap "${name}" appears in multiple states: [${sortedStates.join(', ')}]`)
    }
  }
  return violations
}

// validateBranchHasWIPRoadmap — verifica que branch feat/fix/refactor tem ao menos um roadmap em
// wip/ ou done/ cujo slug case com a branch. Aceita done/ para permitir encerramento do roadmap na
// própria branch, conforme a Definition of Done, sem reprovar o gate.
function validateBranchHasWIPRoadmap() {
  const { execSync } = require('child_process')
  let branch = process.env.TRACKFW_BRANCH || ''
  if (!branch && isGitWorktree(process.cwd())) {
    try {
      branch = execSync('git symbolic-ref --short HEAD', { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] }).trim()
    } catch {
      branch = ''
    }
    if (!branch) {
      branch = process.env.GITHUB_HEAD_REF || process.env.CI_COMMIT_REF_NAME || process.env.GITHUB_REF_NAME || ''
    }
  }
  if (!branch) return []
  if (!branch.startsWith('feat/') && !branch.startsWith('fix/') && !branch.startsWith('refactor/')) {
    return []
  }

  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const doneDirs = resolveDoneDirs(cfg)
  const branchSlug = normalizeBranchSlug(branch.split('/', 2)[1])
  // candidates reúne todos os roadmaps encontrados em wip/ e done/.
  const candidates = []
  for (const dir of [...wipDirs, ...doneDirs]) {
    const files = listDir(dir).filter(f => f.endsWith('.md'))
    candidates.push(...files)
    if (files.some(file => normalizeBranchSlug(file).includes(branchSlug))) return []
  }

  if (candidates.length === 0) {
    return [`branch "${branch}" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ — create governance artifacts first:\n  trackfw req new "title"\n  trackfw roadmap new "title"\n  trackfw roadmap move <name> wip`]
  }
  // P3: sort for deterministic output regardless of filesystem ordering.
  const sorted = [...candidates].sort()
  const display = sorted.slice(0, 3)
  const suffix = sorted.length > 3 ? `, e mais ${sorted.length - 3}` : ''
  return [`branch "${branch}" has no matching roadmap in wip/ nor done/ (found: ${display.join(', ')}${suffix}) — include the branch slug in the roadmap filename or set TRACKFW_BRANCH explicitly in CI`]
}

function normalizeBranchSlug(value) {
  return String(value || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

/**
 * Detecta notas em vault/notes/ não referenciadas pelo index.md.
 * index.md não conta como nota órfã. Projeto sem vault/ retorna [].
 * Aceita link markdown `[texto](arquivo.md)` ou wikilink `[[nome]]`.
 * @param {string} [cwd]
 * @returns {string[]}
 */
function validateNoteOrphan(cwd) {
  const root = cwd || process.cwd()
  const vaultDir = path.join(root, 'vault', 'notes')
  if (!fs.existsSync(vaultDir)) return []

  const indexPath = path.join(vaultDir, 'index.md')
  let indexContent = ''
  if (fs.existsSync(indexPath)) {
    indexContent = fs.readFileSync(indexPath, 'utf8')
  }

  const notes = fs.readdirSync(vaultDir).filter((f) => f.endsWith('.md') && f !== 'index.md')
  const msgs = []
  for (const filename of notes) {
    const nameWithoutExt = filename.replace(/\.md$/, '')
    const referenced =
      indexContent.includes(`(${filename})`) ||
      indexContent.includes(`[[${nameWithoutExt}]]`) ||
      indexContent.includes(`[[${filename}]]`)
    if (!referenced) {
      msgs.push(`note "${filename}" is not referenced in vault/notes/index.md`)
    }
  }
  return msgs
}

function isGitWorktree(dir) {
  try {
    const out = execSync('git rev-parse --is-inside-work-tree', { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], cwd: dir })
    return String(out).trim() === 'true'
  } catch {
    return false
  }
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

// RULE_DEFAULTS mapeia regras cujo default NÃO é 'error'.
const RULE_DEFAULTS = {
  note_orphan: 'warning',
}

// ruleSeverity retorna a severidade configurada para uma regra ('error'|'warning'|'off').
// Prioridade: trackfw.yaml rules: > RULE_DEFAULTS > 'error'.
function ruleSeverity(name) {
  const cfg = config.load()
  if (cfg.rules[name]) return cfg.rules[name]
  if (RULE_DEFAULTS[name]) return RULE_DEFAULTS[name]
  return 'error'
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
  for (const msg of validateREQRoadmapLifecycle()) { _setMeta(msg, 'req_roadmap_lifecycle'); warnings.push(msg) }
  applyRule('folder_status',        validateFolderStatusCoherence(),       violations, warnings)
  applyRule('filename_uniqueness',  validateFilenameUniqueness(),          violations, warnings)
  applyRule('branch_has_wip_roadmap', validateBranchHasWIPRoadmap(),      violations, warnings)
  applyRule('blocked_by_draft_adr', validateREQsNotBlockedByDraftADRs(),  violations, warnings)
  applyRule('note_orphan',           validateNoteOrphan(),                 violations, warnings)

  // Regras configuráveis via applyRule (popula _itemMeta automaticamente)
  applyRule('req_has_adr',          validateREQsHaveADR(),          violations, warnings)
  applyRule('blocked_has_req',      validateBlockedHasREQ(),        violations, warnings)
  applyRule('req_has_roadmap',      validateREQsHaveRoadmap(),      violations, warnings)

  // Regra direta (sem configuração de severidade): violation sempre
  for (const msg of validateFrontmatterPresence())  { _setMeta(msg, 'frontmatter_presence'); violations.push(msg) }

  // Validação de existência dos adr_dirs (retorna violations se strictCiPaths, senão warnings)
  const adrDirsExistResult = validateADRDirsExist()
  for (const msg of adrDirsExistResult.violations) { _setMeta(msg, 'adr_dir_exists'); violations.push(msg) }
  for (const msg of adrDirsExistResult.warnings) { _setMeta(msg, 'adr_dir_exists'); warnings.push(msg) }

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
  setStaleWipNowForTests,
  validateREQsNotBlockedByDraftADRs,
  parseBlockedADRs,
  adrIsDraft,
  listDir,
  tryListDir,
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
  validateREQRoadmapLifecycle,
  validateFolderStatusCoherence,
  validateFilenameUniqueness,
  validateBranchHasWIPRoadmap,
  // novas funções ML-2B
  contentHasMarker,
  ruleSeverity,
  applyRule,
  // novas funções ML-1B (v2.5.1)
  getItemMeta,
  resetMeta,
  // novas funções ML-2B (governança global)
  isInsideDir,
  walkDirMdWithPaths,
  validateADRDirsExist,
}
