'use strict'

const fs = require('fs')
const os = require('os')
const path = require('path')
const crypto = require('crypto')
const { gitOutput } = require('./git-exec')
const config = require('../config')
const { checkTraceIds } = require('./traceid')
const { loadProvenance } = require('../thirdparty/provenance')
const { readQuarantine, decodeContent } = require('../thirdparty/quarantine')

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
    const out = gitOutput('.', ['log', '-1', '--format=%ct', '--', filePath]).trim()
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

// extractAdrHeaderStatus extrai o valor declarado na linha de cabeçalho
// ("> Date: ... | Status: X") de um ADR. Ancorado à linha — não é substring livre sobre o
// documento inteiro — para não confundir texto de prosa que mencione "Status: Draft"/"Proposed"
// (ex: citações de código, exemplos) com o status real do artefato. Retorna '' se a linha não
// existir.
function extractAdrHeaderStatus(content) {
  const marker = '| Status: '
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const idx = trimmed.indexOf(marker)
    if (idx >= 0) {
      let rest = trimmed.slice(idx + marker.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) rest = rest.slice(0, pipeIdx)
      return rest.trim()
    }
  }
  return ''
}

// extractFrontmatterField extrai o valor de um campo do bloco frontmatter YAML
// (delimitado por "---" ... "---" no início do arquivo). Retorna '' se o frontmatter
// estiver ausente ou o campo não existir/estiver vazio.
function extractFrontmatterField(content, field) {
  if (!content.startsWith('---')) return ''
  const rest = content.slice(3)
  const end = rest.indexOf('\n---')
  if (end < 0) return ''
  const block = rest.slice(0, end)
  for (const line of block.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith(field + ':')) {
      let val = trimmed.slice(field.length + 1).trim()
      val = val.replace(/^["']|["']$/g, '')
      return val
    }
  }
  return ''
}

// resolveAdrStatus extrai o valor bruto do status de um ADR: frontmatter `status:`
// primeiro — é o campo machine-readable canônico, o mesmo que os geradores (`adr new`,
// NewADRDraft) escrevem e que a regra folder_status já usa como fonte de verdade — com
// fallback para a linha de cabeçalho ("> Date: ... | Status: X") somente se o
// frontmatter estiver ausente ou sem o campo (cobre ADRs legados sem frontmatter). Em um
// ADR bem formado os dois concordam. ML-1D (2026-08-01): alinha o Node ao Go e ao
// Python, que já liam frontmatter-first (ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito).
function resolveAdrStatus(content) {
  const fm = extractFrontmatterField(content, 'status')
  if (fm) return fm
  return extractAdrHeaderStatus(content)
}

// adrNotAcceptedStatusForRule é o helper canônico de "ADR não aceito": único lugar que conhece o
// vocabulário Draft/Proposed. Verdadeiro para ADR cujo status seja "Draft" ou "Proposed"; qualquer
// outro status (Accepted, Superseded, Deprecated, Rejected, ...) conta como aceito por exclusão —
// não há allowlist fechada de status aceitos.
//
// A fonte do status é resolveAdrStatus (frontmatter-first, fallback de cabeçalho — ver acima).
// Crucial: o fallback de cabeçalho extrai o valor de UMA linha específica
// (extractAdrHeaderStatus), não faz content.includes('Status: Draft') sobre o documento
// inteiro — esse era o defeito do código herdado (adrDraftStatusForRule original): um ADR
// com status real "Accepted" mas cuja prosa cita literalmente a string "Status: Draft"
// (ex: este próprio ADR, que documenta o bug) seria classificado como não aceito. Ver
// vault/notes para o caso concreto que expôs isso.
function adrNotAcceptedStatusForRule(rule, basename, messages) {
  const p = findAdrFile(basename)
  if (!p) return { notAccepted: false, status: '', inspected: true }
  try {
    const content = fs.readFileSync(p, 'utf8')
    const status = resolveAdrStatus(content)
    const notAccepted = status.toLowerCase() === 'draft' || status.toLowerCase() === 'proposed'
    return { notAccepted, status, inspected: true }
  } catch (err) {
    if (messages) messages.push(inspectionDiagnostic(rule, p, err))
    return { notAccepted: false, status: '', inspected: false }
  }
}

// adrIsDraft verifica se <adrBasename> está em status não aceito (Draft ou Proposed) buscando
// recursivamente nas adrDirs. Nome mantido por compatibilidade (chamadores existentes); delega no
// helper canônico adrNotAcceptedStatusForRule.
function adrIsDraft(basename) {
  return adrDraftStatusForRule(basename, null).draft
}

function adrDraftStatusForRule(basename, messages) {
  const result = adrNotAcceptedStatusForRule('blocked_by_draft_adr', basename, messages)
  return { draft: result.notAccepted, inspected: result.inspected }
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

// wipConfigFrom deriva { limit, bySquad } a partir do ProjectConfig já normalizado por
// config.load() — nenhuma releitura de trackfw.yaml acontece aqui.
function wipConfigFrom(cfg) {
  return { limit: cfg.wipLimit > 0 ? cfg.wipLimit : 1, bySquad: !!cfg.wipBySquad }
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

  const wipCfg = wipConfigFrom(cfg)

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
        violations.push(`REQ ${path.basename(filePath)} is blocked by not-accepted ADR: ${adrBasename}`)
      }
    }
  }
  return violations
}

// reqStatusEquals compara o status de uma REQ (case-insensitive) contra o valor esperado.
// Casa tanto o frontmatter ("status: X") quanto a linha de cabeçalho ("> Date: ... | Status: X"),
// mesma lógica de detecção usada por reqStatusIsOpen — duplicada aqui (em vez de generalizar
// reqStatusIsOpen) para não alterar o comportamento de req_roadmap_lifecycle, que já a consome.
function reqStatusEquals(content, status) {
  const target = String(status).toLowerCase()
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const idx = trimmed.indexOf(':')
    if (idx >= 0 && trimmed.slice(0, idx).trim().toLowerCase() === 'status') {
      return trimmed.slice(idx + 1).trim().replace(/^["']|["']$/g, '').toLowerCase() === target
    }
    const marker = '| Status: '
    const markerIdx = trimmed.indexOf(marker)
    if (markerIdx >= 0) {
      let rest = trimmed.slice(markerIdx + marker.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) rest = rest.slice(0, pipeIdx)
      return rest.trim().toLowerCase() === target
    }
  }
  return false
}

// validateADRAcceptedWhenREQDone — REQ Done cujo ADR vinculado (campo "ADR:") não está aceito
// (Draft ou Proposed, via o helper canônico adrNotAcceptedStatusForRule) → violation. Fecha a
// lacuna que deixou um ADR Proposed atravessar sete REQs Done sem nenhum gate detectar.
function validateADRAcceptedWhenREQDone() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    const content = readFileForRule('adr_accepted_when_req_done', filePath, violations)
    if (content === null) continue
    if (!reqStatusEquals(content, 'done')) continue

    const adrRef = extractRefPath(content, 'ADR')
    if (!adrRef) continue
    const adrBasename = path.basename(adrRef)
    const status = adrNotAcceptedStatusForRule('adr_accepted_when_req_done', adrBasename, violations)
    if (status.notAccepted) {
      const reqBasename = path.basename(filePath)
      violations.push(
        `REQ "${reqBasename}" is Done but linked ADR "${adrBasename}" is not accepted (status: ${status.status})`
      )
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

// governanceModeFrom deriva { mode, lenientUntil } a partir do ProjectConfig já normalizado por
// config.load() — nenhuma releitura de trackfw.yaml acontece aqui. cfg.governanceMode chega como
// o valor bruto do campo (string vazia se ausente); cfg.lenientUntil chega como a data literal
// (ex.: "2026-08-02"), convertida aqui para Date.
function governanceModeFrom(cfg) {
  const mode = cfg.governanceMode ? cfg.governanceMode : 'strict'
  let lenientUntil = null
  if (cfg.lenientUntil) {
    const d = new Date(cfg.lenientUntil)
    if (!isNaN(d.getTime())) lenientUntil = d
  }
  return { mode, lenientUntil }
}

// isLenient retorna true se o projeto está em modo lenient e o prazo não expirou.
function isLenient() {
  const gm = governanceModeFrom(config.load())
  if (gm.mode !== 'lenient') return false
  if (!gm.lenientUntil) return true
  return new Date() < gm.lenientUntil
}

// lenientUntilDate retorna a data de expiração formatada (YYYY-MM-DD) ou ''.
function lenientUntilDate() {
  const gm = governanceModeFrom(config.load())
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
      val = val.replace(/^["'`]|["'`]$/g, '')
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
      if (ref && !referenceExists(ref)) {
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
    if (adrRef && !referenceExists(adrRef)) {
      warnings.push(`req "${name}" links to ADR "${adrRef}" which does not exist`)
    }
    const roadmapRef = extractRefPath(content, 'Roadmap')
    if (roadmapRef && !referenceExists(roadmapRef)) {
      warnings.push(`req "${name}" links to Roadmap "${roadmapRef}" which does not exist`)
    }
  }

  return warnings
}

function referenceExists(ref) {
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

// branchSlugMatchesRoadmap verifica se branchSlug (já normalizado via normalizeBranchSlug) casa com o
// nome de algum roadmap .md encontrado em wipDirs ou doneDirs. Reutilizada por
// validateBranchHasWIPRoadmap e pelo comando `trackfw branch new` — nunca duplicar esta lógica.
//
// Espelha internal/validator/validator.go:BranchSlugMatchesRoadmap. Retorna { matched, candidates }:
// matched indica se algum candidato casou com o slug; candidates lista todos os roadmaps .md
// encontrados em wipDirs+doneDirs (para diagnóstico/mensagem de orientação quando matched é false).
function branchSlugMatchesRoadmap(branchSlug, wipDirs, doneDirs) {
  const dirs = [...wipDirs, ...doneDirs]
  const candidates = []
  let matched = false
  for (const dir of dirs) {
    const files = listDir(dir).filter(f => f.endsWith('.md'))
    candidates.push(...files)
    if (files.some(file => normalizeBranchSlug(file).includes(branchSlug))) matched = true
  }
  return { matched, candidates }
}

// branchGovernanceOrientation is the guidance message printed when a feat/fix/refactor branch has
// no roadmap in wip/ nor done/ at all (candidates is empty). Shared by validateBranchHasWIPRoadmap
// and `trackfw branch new` — never duplicate this string. Byte-identical to Go's
// BranchGovernanceOrientation.
function branchGovernanceOrientation(branch) {
  return `branch "${branch}" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ — create governance artifacts first:\n  trackfw req new "title"\n  trackfw roadmap new "title"\n  trackfw roadmap move <name> wip`
}

// branchNoMatchingRoadmapMessage is the guidance message printed when roadmaps exist in wip/ or
// done/ but none of them match the branch's slug. Shared by validateBranchHasWIPRoadmap and
// `trackfw branch new` — never duplicate this string. Byte-identical to Go's
// BranchNoMatchingRoadmapMessage. Does not mutate candidates.
function branchNoMatchingRoadmapMessage(branch, candidates) {
  // P3: sort for deterministic output regardless of filesystem ordering.
  const sorted = [...candidates].sort()
  const display = sorted.slice(0, 3)
  const suffix = sorted.length > 3 ? `, e mais ${sorted.length - 3}` : ''
  return `branch "${branch}" has no matching roadmap in wip/ nor done/ (found: ${display.join(', ')}${suffix}) — include the branch slug in the roadmap filename or set TRACKFW_BRANCH explicitly in CI`
}

// validateBranchHasWIPRoadmap — verifica que branch feat/fix/refactor tem ao menos um roadmap em
// wip/ ou done/ cujo slug case com a branch. Aceita done/ para permitir encerramento do roadmap na
// própria branch, conforme a Definition of Done, sem reprovar o gate.
function validateBranchHasWIPRoadmap() {
  let branch = process.env.TRACKFW_BRANCH || ''
  if (!branch && isGitWorktree(process.cwd())) {
    try {
      branch = gitOutput(process.cwd(), ['symbolic-ref', '--short', 'HEAD']).trim()
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
  const { matched, candidates } = branchSlugMatchesRoadmap(branchSlug, wipDirs, doneDirs)
  if (matched) return []

  if (candidates.length === 0) {
    return [branchGovernanceOrientation(branch)]
  }
  return [branchNoMatchingRoadmapMessage(branch, candidates)]
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

// CREDENTIAL_GUARD_SCRIPT_MARKER é o nome do script que a regra credential_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto.
const CREDENTIAL_GUARD_SCRIPT_MARKER = 'trackfw-credential-guard.sh'

// GIT_BRANCH_GUARD_SCRIPT_MARKER é o nome do script que a regra git_branch_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto (ROADMAP-2026-08-15-trackfw-validate-deve-
// detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-2A — port de
// internal/validator/validator_git_branch_guard.go's gitBranchGuardScriptMarker). Mesmo padrão de
// CREDENTIAL_GUARD_SCRIPT_MARKER — só o nome do arquivo muda.
const GIT_BRANCH_GUARD_SCRIPT_MARKER = 'trackfw-git-branch-guard.sh'

// CREDENTIAL_GUARD_HOOK_FILES é a lista fechada dos arquivos de hook de PROJETO que o trackfw
// gera hoje e que podem conter uma entrada de credential-guard (ROADMAP-2026-08-12-mitigacao-do
// -fail-open-do-credential-guard, ML-1A). Hooks de escopo GLOBAL (~/.trackfw/..., trackfw update
// harness) ficam fora — caso distinto, fora do repositório do usuário, e a checagem de dedup
// globalCredentialGuardInstalled*() já os pula de propósito nas entradas de projeto.
const CREDENTIAL_GUARD_HOOK_FILES = [
  { relPath: '.claude/settings.json', cli: 'Claude Code' },
  { relPath: '.codex/hooks.json', cli: 'Codex CLI' },
  { relPath: '.gemini/settings.json', cli: 'Gemini CLI' },
  { relPath: '.cursor/hooks.json', cli: 'Cursor' },
  { relPath: '.github/hooks/trackfw-attention.json', cli: 'GitHub Copilot CLI' },
  { relPath: '.kiro/hooks/trackfw-attention.json', cli: 'Kiro' },
]

// resolveCredentialGuardHookPath resolve o valor bruto de um comando de hook (string extraída do
// JSON) para um caminho de arquivo absoluto, usando exatamente as 3 formas de prefixo que o
// trackfw emite hoje (docs/cli-parity.md, "Mecanismo de resolução de caminho dos hooks de
// projeto, por CLI"):
//   1. "$CLAUDE_PROJECT_DIR/…" / "$GEMINI_PROJECT_DIR/…" — placeholder de env var expandido em
//      runtime pelo próprio CLI, substituído aqui pela raiz do projeto.
//   2. '"$(git rev-parse --show-toplevel)/…"' — substituição de shell entre aspas literais
//      (Codex). As aspas fazem parte do valor emitido e são removidas antes de resolver contra a
//      raiz do projeto.
//   3. Caminho relativo puro, sem prefixo nenhum (Cursor/Copilot/Kiro) — resolvido diretamente
//      contra a raiz do projeto.
// Qualquer valor que não bata em nenhuma das 3 formas retorna null — o chamador NÃO deve tratar
// isso como violação. Não é função desta regra adivinhar wiring próprio do usuário fora dos
// formatos que o trackfw gera.
function resolveCredentialGuardHookPath(raw, root) {
  const claudePrefix = '$CLAUDE_PROJECT_DIR/'
  const geminiPrefix = '$GEMINI_PROJECT_DIR/'
  const codexPrefix = '"$(git rev-parse --show-toplevel)/'

  if (raw.startsWith(claudePrefix)) {
    return path.join(root, raw.slice(claudePrefix.length))
  }
  if (raw.startsWith(geminiPrefix)) {
    return path.join(root, raw.slice(geminiPrefix.length))
  }
  if (raw.startsWith(codexPrefix) && raw.endsWith('"')) {
    const inner = raw.slice(codexPrefix.length, raw.length - 1)
    return path.join(root, inner)
  }
  if (!raw.startsWith('$') && !raw.startsWith('"') && !path.isAbsolute(raw)) {
    // Caminho relativo puro — Cursor (beforeShellExecution/preToolUse), GitHub Copilot CLI
    // (campo "bash"), Kiro (action.command).
    return path.join(root, raw)
  }
  return null
}

// collectCommandsWithMarker percorre recursivamente um valor JSON já decodificado e coleta todo
// valor-string que contém marker, independentemente do nome do campo que o contém.
//
// Os 6 formatos de hook usam campos diferentes para o comando: "command" (Claude/Codex/
// Gemini/Cursor), "bash" (GitHub Copilot CLI), "action.command" (Kiro). Varrer por VALOR em vez
// de por caminho de chave evita acoplar esta regra à forma exata de cada schema.
//
// Generalizado (ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
// desatualizados, ML-2A, port de collectCommandsWithMarker em
// internal/validator/validator_git_branch_guard.go) para aceitar qualquer marker — originalmente
// collectCredentialGuardCommands, hardcoded para CREDENTIAL_GUARD_SCRIPT_MARKER; reusado agora
// também para GIT_BRANCH_GUARD_SCRIPT_MARKER, sem duplicar a travessia recursiva.
function collectCommandsWithMarker(value, marker, out) {
  if (typeof value === 'string') {
    if (value.includes(marker)) out.push(value)
    return
  }
  if (Array.isArray(value)) {
    for (const item of value) collectCommandsWithMarker(item, marker, out)
    return
  }
  if (value && typeof value === 'object') {
    for (const key of Object.keys(value)) collectCommandsWithMarker(value[key], marker, out)
  }
}

// collectCredentialGuardCommands é o wrapper específico de credential-guard sobre
// collectCommandsWithMarker — preservado para compatibilidade de assinatura (exportada
// historicamente com 2 argumentos).
function collectCredentialGuardCommands(value, out) {
  collectCommandsWithMarker(value, CREDENTIAL_GUARD_SCRIPT_MARKER, out)
}

// validateGuardHookResolvable é a implementação genérica compartilhada pelas regras
// "credential_guard_hook_resolvable" e "git_branch_guard_hook_resolvable": para cada arquivo de
// hook de PROJETO que existir, extrai os comandos que referenciam scriptMarker, resolve o caminho
// e verifica que o script existe e é executável.
//
// Generalizado a partir da antiga validateCredentialGuardHookResolvable (ROADMAP-2026-08-15-
// trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-2A, port de
// validateGuardHookResolvable em internal/validator/validator_credential_guard.go) — a lógica de
// resolução de caminho por CLI é idêntica para os 2 scripts, só o marker e o texto da mensagem
// mudam.
//
// Riscos de regressão mapeados no roadmap:
//   - A regra só avalia entradas que EXISTEM. Ausência de entrada de guard é estado legítimo
//     (guard global instalado via `trackfw update harness`) — nunca é violação por si só.
//   - Arquivo de hook ausente é pulado em silêncio.
//   - Arquivo de hook presente mas com JSON inválido é pulado em silêncio — validar a forma do
//     JSON não é escopo desta regra.
//
// process.cwd() no Node já retorna o caminho FÍSICO (resolve symlinks via getcwd(3) diretamente),
// diferente de os.Getwd() do Go — ver o comentário sobre EvalSymlinks em
// internal/validator/validator_git_branch_guard.go's validateGuardHookResolvable equivalente
// (validator_credential_guard.go). Nenhuma resolução extra de symlink é necessária aqui.
function validateGuardHookResolvable(scriptMarker, cwd) {
  const root = cwd || process.cwd()
  const msgs = []

  for (const hf of CREDENTIAL_GUARD_HOOK_FILES) {
    const fullPath = path.join(root, hf.relPath)
    let content
    try {
      content = fs.readFileSync(fullPath, 'utf8')
    } catch (e) {
      if (e.code === 'ENOENT') continue
      continue
    }

    let parsed
    try {
      parsed = JSON.parse(content)
    } catch (_) {
      continue
    }

    const commands = []
    collectCommandsWithMarker(parsed, scriptMarker, commands)

    const seen = new Set()
    for (const raw of commands) {
      if (seen.has(raw)) continue
      seen.add(raw)

      const resolved = resolveCredentialGuardHookPath(raw, root)
      if (resolved === null) continue

      let stat = null
      try {
        stat = fs.statSync(resolved)
      } catch (_) {
        stat = null
      }

      if (!stat) {
        msgs.push(`${hf.relPath} (${hf.cli}) references ${scriptMarker} resolved to "${resolved}", but the script does not exist — run \`trackfw update\` to regenerate it`)
      } else if ((stat.mode & 0o111) === 0) {
        msgs.push(`${hf.relPath} (${hf.cli}) references ${scriptMarker} resolved to "${resolved}", but the script is not executable — run \`trackfw update\` to regenerate it`)
      }
    }
  }

  return msgs
}

// validateCredentialGuardHookResolvable é a regra "credential_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
function validateCredentialGuardHookResolvable(cwd) {
  return validateGuardHookResolvable(CREDENTIAL_GUARD_SCRIPT_MARKER, cwd)
}

// validateGitBranchGuardHookResolvable é a regra "git_branch_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
function validateGitBranchGuardHookResolvable(cwd) {
  return validateGuardHookResolvable(GIT_BRANCH_GUARD_SCRIPT_MARKER, cwd)
}

function isGitWorktree(dir) {
  try {
    const out = gitOutput(dir, ['rev-parse', '--is-inside-work-tree'])
    return String(out).trim() === 'true'
  } catch {
    return false
  }
}

// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.
// ADR: docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-
// resposta-e-deteccao-ancorada-no-git.md (Emenda 1: âncora POR ALVO, decidida na Barreira B0).
// Ver internal/validator/validator_credential_guard_integrity.go para o raciocínio completo
// (âncoras, severidades, e por que "trackfw.yaml sem a chave" é lido como HEAD sem a chave, não
// disco sem a chave) — replicado aqui byte-a-byte na semântica, não na forma.

// CREDENTIAL_GUARD_SCRIPT_REFERENCE is a validator-local copy of the same template composed in
// generators/hooks.js (CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_PROJECT_GUARD + CG_DETECTION_CORE
// + CG_PROJECT_TAIL). Not required directly from generators/hooks.js because
// generateCredentialGuardScript() writes to disk AND console.log()s a success line on every call
// -- calling it from inside `trackfw validate` would leak a stray "  \u2713
// scripts/trackfw-credential-guard.sh" line into validate's output on every run, corrupting the
// exact success message Cenario 29 fixes byte-for-byte. Kept as a literal copy (same choice made
// in internal/validator/validator_credential_guard_integrity_reference.go for Go, for the same
// reason) instead -- drift is caught by test/validator-credential-guard-script-reference.test.js,
// which regenerates the real script via generateCredentialGuardScript() into a temp dir and
// asserts byte-equality against this constant.
const CREDENTIAL_GUARD_SCRIPT_REFERENCE = `#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \\" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\\$\\{?[A-Za-z_][A-Za-z0-9_]*\\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\\$\\{?([A-Za-z_][A-Za-z0-9_]*)\\}?$/\\1/')
    pattern="*\${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=\${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="\${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

DEFAULT_MODE="warn"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$MSG_ESC" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

// validateCredentialGuardScriptIntegrity é a regra "credential_guard_script_integrity": compara
// scripts/trackfw-credential-guard.sh em disco contra o template que esta versão do trackfw
// geraria. Silenciosa quando o script não existe — ausência é escopo de
// credential_guard_hook_resolvable, não desta regra.
function validateCredentialGuardScriptIntegrity(cwd) {
  const root = cwd || process.cwd()
  const relPath = 'scripts/trackfw-credential-guard.sh'
  let content
  try {
    content = fs.readFileSync(path.join(root, relPath), 'utf8')
  } catch (e) {
    if (e.code === 'ENOENT') return []
    return [inspectionDiagnostic('credential_guard_script_integrity', relPath, e)]
  }

  if (content === CREDENTIAL_GUARD_SCRIPT_REFERENCE) return []

  return [
    `${relPath} content diverges from the template this version of trackfw generates — ` +
    'if you did not edit this file by hand, run `trackfw update` to regenerate it',
  ]
}

// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
// ML-2A: port of internal/validator/validator_git_branch_guard.go — adds git-branch-guard
// coverage to the two existing credential-guard checks (existence/executability via
// validateGuardHookResolvable above, and content-drift integrity via
// validateGitBranchGuardScriptIntegrity below), plus the GLOBAL-scope check that was missing for
// BOTH guards before this ML (same gap the Go implementation closed first).

// GBG_REF_BACKTICK — validator-local copy of the same escaped-backtick helper hooks.js's
// GBG_BACKTICK uses (npm/src/generators/hooks.js), to embed REASON's `` `cmd` `` inside the
// script's double-quoted shell string without breaking this JS template literal.
const GBG_REF_BACKTICK = '\\`'

// GIT_BRANCH_GUARD_SCRIPT_REFERENCE is a validator-local copy of the
// scripts/trackfw-git-branch-guard.sh template composed in npm/src/generators/hooks.js
// (GIT_BRANCH_GUARD_SCRIPT const). Not required directly from generators/hooks.js: like
// CREDENTIAL_GUARD_SCRIPT_REFERENCE above, generateGitBranchGuardScript() writes to disk AND
// console.log()s a success line on every call — calling it from inside `trackfw validate` would
// leak a stray "  ✓ scripts/trackfw-git-branch-guard.sh" line into validate's output. There is no
// import-cycle constraint in Node (unlike Go's internal/validator ↔ internal/generators cycle —
// see gitBranchGuardScriptReference's doc comment in
// internal/validator/validator_git_branch_guard_reference.go) but the console.log side effect is
// reason enough on its own to keep a local copy here, matching the choice already made for
// credential-guard in this same file.
//
// Unlike CREDENTIAL_GUARD_SCRIPT_REFERENCE, GIT_BRANCH_GUARD_SCRIPT_REFERENCE is used VERBATIM for
// both the project scope and the global scope (generateGitBranchGuardScript /
// generateGlobalGitBranchGuardScript write the exact same GIT_BRANCH_GUARD_SCRIPT constant) — so
// this single reference constant covers both git_branch_guard_script_integrity (project,
// scripts/trackfw-git-branch-guard.sh) and the global integrity check
// (~/.trackfw/scripts/trackfw-git-branch-guard.sh), no second reference constant needed.
//
// Drift between this copy and the real generator is caught by
// "GIT_BRANCH_GUARD_SCRIPT_REFERENCE é byte-idêntico ao que generateGitBranchGuardScript emite"
// (npm/tests/git_branch_guard.test.js): it regenerates the script via generateGitBranchGuardScript
// into a temp dir and asserts byte-equality against this constant.
const GIT_BRANCH_GUARD_SCRIPT_REFERENCE = `#!/usr/bin/env bash
# trackfw git branch guard — bloqueia git commit/push/checkout -b brutos por subagente
set -euo pipefail
set -f

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT=$(cat 2>/dev/null || true)
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \\{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .tool_info.command_line // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_info"[[:space:]]*:[[:space:]]*{[^}]*"command_line"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"hook_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      ;;
    *)
      CMD_RAW="$INPUT"
      ;;
  esac
fi

if [ -z "$CMD_RAW" ]; then
  CMD_RAW="\${TRACKFW_GIT_COMMAND:-}"
fi

[ -n "$CMD_RAW" ] || exit 0

# --- 2. Casar contra "git (commit|push|checkout -b)", segmento por segmento -----------------
# Cada segmento é um comando real (dividido por ; && || | e por quebra de linha). "git" só
# conta se for o PRIMEIRO token do segmento (por basename, então /usr/bin/git também casa) —
# nunca uma ocorrência solta em qualquer posição da string inteira. Isso evita: (a) o segundo
# comando de uma cadeia escapar da checagem, (b) um path absoluto para o git escapar por
# comparação de igualdade exata, e (c) texto de prosa (ex.: mensagem de commit mencionando
# "git commit" no meio de uma frase) ser tratado como comando.
match_subcommand() {
  normalized=$(printf '%s' "$1" | sed -e 's/&&/\\n/g' -e 's/||/\\n/g' -e 's/[;|]/\\n/g')
  while IFS= read -r seg; do
    seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
    [ -n "$seg_trimmed" ] || continue

    set -- $seg_trimmed
    first="$1"
    base="\${first##*/}"
    [ "$base" = "git" ] || continue
    shift

    sub=""
    while [ "$#" -gt 0 ]; do
      tok="$1"
      case "$tok" in
        -C|-c|--work-tree|--git-dir|--namespace)
          if [ "$#" -ge 2 ]; then shift 2; else shift; fi
          continue
          ;;
        -*)
          shift
          continue
          ;;
        *)
          sub="$tok"
          shift
          break
          ;;
      esac
    done

    case "$sub" in
      commit)
        echo "commit"
        return 0
        ;;
      push)
        echo "push"
        return 0
        ;;
      checkout)
        if [ "\${1:-}" = "-b" ]; then
          echo "checkout-b"
          return 0
        fi
        ;;
    esac
  done <<EOF
$normalized
EOF
  return 1
}

SUBCOMMAND=$(match_subcommand "$CMD_RAW") || exit 0

case "$SUBCOMMAND" in
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_REF_BACKTICK + `. Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw commit -m '<mensagem>'` + GBG_REF_BACKTICK + `. Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw ship` + GBG_REF_BACKTICK + `. Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"decision":"block","reason":"%s"}\\n' "$REASON"
echo "$REASON" >&2
exit 2
`

// validateGitBranchGuardScriptIntegrity é a regra "git_branch_guard_script_integrity": compara
// scripts/trackfw-git-branch-guard.sh em disco contra o template que esta versão do trackfw
// geraria. Espelha validateCredentialGuardScriptIntegrity exatamente — mesmo contrato de silêncio
// na ausência (existência é responsabilidade de git_branch_guard_hook_resolvable, não desta regra).
function validateGitBranchGuardScriptIntegrity(cwd) {
  const root = cwd || process.cwd()
  const relPath = 'scripts/trackfw-git-branch-guard.sh'
  let content
  try {
    content = fs.readFileSync(path.join(root, relPath), 'utf8')
  } catch (e) {
    if (e.code === 'ENOENT') return []
    return [inspectionDiagnostic('git_branch_guard_script_integrity', relPath, e)]
  }

  if (content === GIT_BRANCH_GUARD_SCRIPT_REFERENCE) return []

  return [
    `${relPath} content diverges from the template this version of trackfw generates — ` +
    'if you did not edit this file by hand, run `trackfw update` to regenerate it',
  ]
}

// CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE is a validator-local copy of the GLOBAL-scope
// ~/.trackfw/scripts/trackfw-credential-guard.sh template composed in npm/src/generators/hooks.js
// (GLOBAL_CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_DETECTION_CORE + CG_GLOBAL_TAIL). This is a
// DIFFERENT template than CREDENTIAL_GUARD_SCRIPT_REFERENCE (the project-scope variant): the
// global variant omits the project-guard block ("no-op outside a trackfw.yaml project") and
// defaults credential_guard.mode to "block" instead of "warn" — mirrors Go's
// credentialGuardGlobalScriptReference
// (internal/validator/validator_credential_guard_global_reference.go). Comparing the global
// on-disk script against CREDENTIAL_GUARD_SCRIPT_REFERENCE (the project template) would be a
// guaranteed false positive for every user with the global harness installed.
const CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE = `#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \\" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\\$\\{?[A-Za-z_][A-Za-z0-9_]*\\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\\$\\{?([A-Za-z_][A-Za-z0-9_]*)\\}?$/\\1/')
    pattern="*\${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=\${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="\${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

DEFAULT_MODE="block"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR="docs/roadmaps"
if [ ! -d "$ROADMAP_DIR" ]; then
  exit 0
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$MSG_ESC" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

// globalGuardConfigFile associates a GLOBAL (per-CLI, $HOME-rooted) hook/settings config file with
// the CLI that consumes it, for the global-scope guard checks below. Distinct from
// CREDENTIAL_GUARD_HOOK_FILES, whose relPath is rooted at the PROJECT root, not $HOME.
//
// GLOBAL_GUARD_CONFIG_FILES is the closed list of GLOBAL hook/settings files `trackfw update
// harness` can write a guard entry into — the global-scope counterpart of
// CREDENTIAL_GUARD_HOOK_FILES. Paths and CLI labels ported from Go's globalGuardConfigFiles
// (internal/validator/validator_git_branch_guard.go) — note this list DIFFERS from
// CREDENTIAL_GUARD_HOOK_FILES for Copilot and Kiro (global scope uses .copilot/settings.json and
// .kiro/hooks/trackfw-credential-guard.json, not the project-scope attention-hook paths).
const GLOBAL_GUARD_CONFIG_FILES = [
  { relPath: '.claude/settings.json', cli: 'Claude Code' },
  { relPath: '.codex/hooks.json', cli: 'Codex CLI' },
  { relPath: '.gemini/settings.json', cli: 'Gemini CLI' },
  { relPath: '.cursor/hooks.json', cli: 'Cursor' },
  { relPath: '.copilot/settings.json', cli: 'GitHub Copilot CLI' },
  { relPath: '.kiro/hooks/trackfw-credential-guard.json', cli: 'Kiro' },
]

// validateGuardGlobalHookResolvable is the GLOBAL-scope counterpart of
// validateGuardHookResolvable: for each of the 6 GLOBAL_GUARD_CONFIG_FILES that exists AND
// references scriptMarker, verifies the referenced script exists and is executable. Port of Go's
// validateGuardGlobalHookResolvable (internal/validator/validator_git_branch_guard.go) — see that
// function's doc comment for the full trigger-condition and fail-open rationale.
//
// Global entries are written by npm/src/generators (harnessCredentialGuardTarget*-equivalent code
// in agentfiles-equivalent generators) as fully resolved absolute paths, never a placeholder like
// $CLAUDE_PROJECT_DIR — so, unlike the project-scope resolveCredentialGuardHookPath, no
// prefix-stripping is needed here: any matched command that is NOT already an absolute path is
// skipped (never treated as a violation).
//
// Fail-open: unresolvable $HOME, unreadable file, or invalid JSON all skip that file in silence —
// same contract validateGuardHookResolvable already has for project-scope files.
function validateGuardGlobalHookResolvable(scriptMarker) {
  const home = os.homedir()
  if (!home) return []

  const msgs = []
  for (const gf of GLOBAL_GUARD_CONFIG_FILES) {
    const fullPath = path.join(home, gf.relPath)
    let content
    try {
      content = fs.readFileSync(fullPath, 'utf8')
    } catch (e) {
      if (e.code === 'ENOENT') continue
      continue
    }

    let parsed
    try {
      parsed = JSON.parse(content)
    } catch (_) {
      continue
    }

    const commands = []
    collectCommandsWithMarker(parsed, scriptMarker, commands)

    const seen = new Set()
    for (const raw of commands) {
      if (seen.has(raw)) continue
      seen.add(raw)

      if (!path.isAbsolute(raw)) continue

      let stat = null
      try {
        stat = fs.statSync(raw)
      } catch (_) {
        stat = null
      }

      if (!stat) {
        msgs.push(`~/${gf.relPath} (${gf.cli}, global scope) references ${scriptMarker} resolved to "${raw}", but the script does not exist — run \`trackfw update harness\` to regenerate it`)
      } else if ((stat.mode & 0o111) === 0) {
        msgs.push(`~/${gf.relPath} (${gf.cli}, global scope) references ${scriptMarker} resolved to "${raw}", but the script is not executable — run \`trackfw update harness\` to regenerate it`)
      }
    }
  }

  return msgs
}

// validateGuardGlobalScriptIntegrity is the GLOBAL-scope counterpart of
// validateCredentialGuardScriptIntegrity/validateGitBranchGuardScriptIntegrity: for each of the 6
// GLOBAL_GUARD_CONFIG_FILES that references scriptMarker, verifies the referenced script's content
// matches referenceContent byte-for-byte. Port of Go's validateGuardGlobalScriptIntegrity
// (internal/validator/validator_git_branch_guard.go) — same trigger condition and fail-open
// contract as validateGuardGlobalHookResolvable above; does not re-validate existence/
// executability, only content, matching the project-scope split between *_hook_resolvable and
// *_script_integrity.
function validateGuardGlobalScriptIntegrity(scriptMarker, referenceContent) {
  const home = os.homedir()
  if (!home) return []

  const msgs = []
  for (const gf of GLOBAL_GUARD_CONFIG_FILES) {
    const fullPath = path.join(home, gf.relPath)
    let content
    try {
      content = fs.readFileSync(fullPath, 'utf8')
    } catch (e) {
      if (e.code === 'ENOENT') continue
      continue
    }

    let parsed
    try {
      parsed = JSON.parse(content)
    } catch (_) {
      continue
    }

    const commands = []
    collectCommandsWithMarker(parsed, scriptMarker, commands)

    const seen = new Set()
    for (const raw of commands) {
      if (seen.has(raw)) continue
      seen.add(raw)

      if (!path.isAbsolute(raw)) continue

      let scriptContent
      try {
        scriptContent = fs.readFileSync(raw, 'utf8')
      } catch (_) {
        // Existence/readability is validateGuardGlobalHookResolvable's job — do not double-report
        // the same underlying condition under two rule names.
        continue
      }

      if (scriptContent === referenceContent) continue

      msgs.push(`${raw} (global scope, referenced from ~/${gf.relPath}, ${gf.cli}) content diverges from the template this version of trackfw generates — if you did not edit this file by hand, run \`trackfw update harness\` to regenerate it`)
    }
  }

  return msgs
}

// validateCredentialGuardGlobalHookResolvable / validateCredentialGuardGlobalScriptIntegrity /
// validateGitBranchGuardGlobalHookResolvable / validateGitBranchGuardGlobalScriptIntegrity are the
// 4 thin wrappers wired in validateUnfiltered below — each folds its messages into the SAME rule
// name as its project-scope counterpart (credential_guard_hook_resolvable,
// credential_guard_script_integrity, git_branch_guard_hook_resolvable,
// git_branch_guard_script_integrity respectively), so no new rules: entries in trackfw.yaml are
// needed. Port of the 4 equivalent wrappers in
// internal/validator/validator_git_branch_guard.go.
function validateCredentialGuardGlobalHookResolvable() {
  return validateGuardGlobalHookResolvable(CREDENTIAL_GUARD_SCRIPT_MARKER)
}

function validateCredentialGuardGlobalScriptIntegrity() {
  return validateGuardGlobalScriptIntegrity(CREDENTIAL_GUARD_SCRIPT_MARKER, CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE)
}

function validateGitBranchGuardGlobalHookResolvable() {
  return validateGuardGlobalHookResolvable(GIT_BRANCH_GUARD_SCRIPT_MARKER)
}

function validateGitBranchGuardGlobalScriptIntegrity() {
  return validateGuardGlobalScriptIntegrity(GIT_BRANCH_GUARD_SCRIPT_MARKER, GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
}

// CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES mirrors the shell script's own resolution of
// credential_guard.mode (`grep -A 5 '^credential_guard:'`): the mode key is found on the matched
// line or within the 5 lines following it.
const CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES = 5

// extractCredentialGuardMode replica a leitura leve (grep/sed) que o próprio script faz —
// deliberadamente não um parser YAML completo, para que a noção desta regra de "para o que
// credential_guard.mode resolve" bata com o que roda de fato no hook.
function extractCredentialGuardMode(content) {
  const lines = content.split('\n')
  const start = lines.findIndex(l => l.startsWith('credential_guard:'))
  if (start === -1) return { mode: '', ok: false }

  const end = Math.min(start + 1 + CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES, lines.length)
  for (const line of lines.slice(start, end)) {
    const trimmed = line.trim()
    if (!trimmed.includes('mode:')) continue
    let rest = trimmed.startsWith('mode:') ? trimmed.slice('mode:'.length) : trimmed
    rest = rest.trim()
    const hashIdx = rest.indexOf('#')
    if (hashIdx >= 0) rest = rest.slice(0, hashIdx).trim()
    rest = rest.replace(/^["']+|["']+$/g, '')
    return { mode: rest, ok: true }
  }
  return { mode: '', ok: false }
}

// headTrackfwYAML retorna o conteúdo de trackfw.yaml no HEAD do git, resolvido relativo a cwd
// (não necessariamente a raiz do repo — `trackfw validate` pode rodar de um subdiretório). ok é
// false sempre que não há âncora usável: não é worktree git, sem commits, ou trackfw.yaml não
// versionado no HEAD — todos "sem âncora, silêncio", nunca erro.
function headTrackfwYAML(cwd) {
  const root = cwd || process.cwd()
  if (!isGitWorktree(root)) return { content: '', ok: false }
  try {
    gitOutput(root, ['rev-parse', '--verify', 'HEAD'])
  } catch {
    return { content: '', ok: false }
  }
  try {
    const out = gitOutput(root, ['show', 'HEAD:./trackfw.yaml'])
    return { content: out, ok: true }
  } catch {
    return { content: '', ok: false }
  }
}

// validateCredentialGuardModeDowngrade é a regra "credential_guard_mode_downgrade": dispara apenas
// quando credential_guard.mode era explicitamente "block" no HEAD e o trackfw.yaml atual em disco
// não resolve mais para "block" (warn explícito, valor não reconhecido, ou chave/arquivo
// ausente — todos os quais o próprio script resolveria como "warn", o DEFAULT_MODE da variante de
// projeto).
//
// Silenciosa sempre que HEAD não é "block": isso é "sem âncora para detectar downgrade", não
// "nada errado". A ausência da chave em DISCO nunca é tratada como silêncio — é exatamente a via
// que esta regra existe para cobrir.
function validateCredentialGuardModeDowngrade(cwd) {
  const root = cwd || process.cwd()
  const head = headTrackfwYAML(root)
  if (!head.ok) return []

  const headMode = extractCredentialGuardMode(head.content)
  if (headMode.mode !== 'block') return []

  let diskContent
  try {
    diskContent = fs.readFileSync(path.join(root, 'trackfw.yaml'), 'utf8')
  } catch (e) {
    if (e.code === 'ENOENT') return [credentialGuardModeDowngradeMessage()]
    return [inspectionDiagnostic('credential_guard_mode_downgrade', 'trackfw.yaml', e)]
  }

  const diskMode = extractCredentialGuardMode(diskContent)
  if (diskMode.mode === 'block') return []

  return [credentialGuardModeDowngradeMessage()]
}

function credentialGuardModeDowngradeMessage() {
  return 'trackfw.yaml sets credential_guard.mode: block at the git HEAD commit, but the ' +
    'current file does not resolve to block — if this was intentional, commit the change; ' +
    'otherwise investigate before treating the credential guard as active'
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
  // ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A,
  // ADR-2026-08-12 Emenda 3: the script carries no version marker, so this rule cannot tell
  // legitimate drift (trackfw not updated yet) from tampering — kept a warning, never an error.
  // credential_guard_mode_downgrade is deliberately absent: it falls through to ruleSeverity's
  // 'error' default.
  credential_guard_script_integrity: 'warning',
  // ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
  // ML-2A: same rationale as credential_guard_script_integrity above — the script carries no
  // version marker, so this rule cannot tell legitimate drift from tampering.
  // git_branch_guard_hook_resolvable is deliberately absent from this map (falls through to
  // 'error'), mirroring credential_guard_hook_resolvable.
  git_branch_guard_script_integrity: 'warning',
}

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1A.
// ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
// estrita-entre-head-e-disco.md.
//
// As 3 regras abaixo resolvem severidade de forma DIFERENTE de todas as outras ~38: comparam HEAD
// contra disco e adotam a MAIS ESTRITA das duas, em vez de ler só o disco. Deliberado, não bug —
// sem isso, estas 3 regras podem ser desligadas pela mesma edição NÃO COMMITADA que elas deveriam
// denunciar (`rules: credential_guard_mode_downgrade: off` em trackfw.yaml, nunca commitado). Toda
// outra regra continua passando por diskRuleSeverity, byte-idêntico a antes deste ADR.
const CREDENTIAL_GUARD_ANCHORED_RULES = new Set([
  'credential_guard_hook_resolvable',
  'credential_guard_script_integrity',
  'credential_guard_mode_downgrade',
])

// credentialGuardSeverityRank ordena severidades da menos para a mais estrita, para a comparação
// "mais estrita vence" de credentialGuardRuleSeverity. Qualquer valor fora de 'off'/'warning' só
// significa 'error' na prática — applyRule já trata qualquer valor não reconhecido como violation,
// então este ranking espelha esse mesmo fallback em vez de introduzir um contrato mais rígido.
function credentialGuardSeverityRank(s) {
  if (s === 'off') return 0
  if (s === 'warning') return 1
  return 2
}

// credentialGuardStricterSeverity retorna a mais estrita entre a e b ('error' > 'warning' > 'off').
function credentialGuardStricterSeverity(a, b) {
  return credentialGuardSeverityRank(a) >= credentialGuardSeverityRank(b) ? a : b
}

// credentialGuardDefaultSeverity é o mesmo fallback "RULE_DEFAULTS > error" que diskRuleSeverity
// usa quando trackfw.yaml não tem rules: <name> — extraído para credentialGuardRuleSeverity poder
// aplicá-lo igualmente ao lado HEAD (que não tem equivalente de RULE_DEFAULTS próprio, já que
// config.parseRulesFromContent só devolve o que rules: em si contém).
function credentialGuardDefaultSeverity(name) {
  return RULE_DEFAULTS[name] || 'error'
}

// credentialGuardRuleSeverity resolve a severidade de uma das 3 CREDENTIAL_GUARD_ANCHORED_RULES
// como a MAIS ESTRITA entre HEAD e disco — direcional, não "ignora disco e usa só HEAD" (ver o
// parecer §2 e o ADR — o caso comum, HEAD sem menção à regra, precisa resolver para o default, ou
// seja o valor mais estrito possível, senão o disco venceria de volta silenciosamente sempre).
//
// Sem HEAD (não é git worktree, sem commits, ou trackfw.yaml não versionado no HEAD —
// headTrackfwYAML's 3 casos de "sem âncora"): cai no disco puro, igual a qualquer outra regra.
// ADR ponto de decisão 4: limite aceito, não um bypass acionável por adversário — nenhum desses 3
// casos é alcançável por uma edição não commitada de trackfw.yaml sozinha.
function credentialGuardRuleSeverity(name) {
  const diskSeverity = diskRuleSeverity(name)

  const head = headTrackfwYAML()
  if (!head.ok) return diskSeverity

  const headRules = config.parseRulesFromContent(head.content)
  const headSeverity = headRules[name] || credentialGuardDefaultSeverity(name)

  return credentialGuardStricterSeverity(headSeverity, diskSeverity)
}

// ruleSeverity retorna a severidade configurada para uma regra ('error'|'warning'|'off').
// Prioridade: trackfw.yaml rules: > RULE_DEFAULTS > 'error'.
//
// Para as 3 CREDENTIAL_GUARD_ANCHORED_RULES, delega a credentialGuardRuleSeverity acima — ver o
// comentário logo antes dessa constante para o porquê. Toda outra regra segue para
// diskRuleSeverity, textualmente idêntico ao corpo desta função antes do ADR-2026-08-12.
function ruleSeverity(name) {
  if (CREDENTIAL_GUARD_ANCHORED_RULES.has(name)) return credentialGuardRuleSeverity(name)
  return diskRuleSeverity(name)
}

// diskRuleSeverity é a resolução ordinária, só-disco, usada por toda regra exceto as 3
// CREDENTIAL_GUARD_ANCHORED_RULES: trackfw.yaml rules: (CWD) > RULE_DEFAULTS > 'error'.
function diskRuleSeverity(name) {
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

// normalizeThirdPartyForValidation replica npm/src/integrations/render.js's normalizeMarkdown
// (TrimSpace + uma única quebra de linha final) byte a byte, sem importar toda a superfície de
// render por uma linha de lógica — mesma justificativa do doc comment de checksum() em
// thirdparty/markers.js sobre não reusar contentHash quando a assinatura não se encaixa
// perfeitamente. Espelha internal/validator/validator_thirdparty_provenance.go's
// normalizeThirdPartyForValidation.
function normalizeThirdPartyForValidation(content) {
  return Buffer.from(`${content.toString('utf8').trim()}\n`, 'utf8')
}

function sha256Hex(buf) {
  return crypto.createHash('sha256').update(buf).digest('hex')
}

// THIRDPARTY_ORIGIN é o valor de Claim.origin que marca um artefato do manifest como instalado
// por `third-party install` (ADR-2026-08-15 D11).
const THIRDPARTY_ORIGIN = 'thirdparty'

// validateThirdPartyArtifactHasProvenance implementa a regra "thirdparty_artifact_has_provenance"
// (ADR-2026-08-15 D2) — a detecção real, ancorada em git, por trás do guardrail
// TRACKFW_ORCHESTRATOR_SESSION. NUNCA faz fetch de rede (D6): lê só
// .trackfw/integrations-manifest.json, .trackfw/thirdparty-provenance.json e
// .trackfw/thirdparty-quarantine/<checksum>.json, todos já em disco (e, por convenção deste
// projeto, versionados no repositório).
//
// Duas ramificações, ambas fatais (error — a regra está deliberadamente ausente de RULE_DEFAULTS):
//   1. um artifact do manifest carrega um claim com origin === "thirdparty" mas
//      thirdparty-provenance.json não tem entrada chaveada por aquele destino;
//   2. existe entrada de proveniência, mas seu checksum_sha256 não pode ser reconciliado com o que
//      de fato está em disco no destino declarado.
//
// Sobre a ramificação 2 — mesma imprecisão de D2 encontrada e resolvida na implementação Go
// (ver internal/validator/validator_thirdparty_provenance.go's doc comment para a análise
// completa): checksum_sha256 é sha256 dos bytes BRUTOS (D6), mas o arquivo instalado é
// NormalizeThirdPartyContent(raw) — não é a função identidade em geral. A comparação literal
// "sha256(arquivo instalado) === checksum_sha256" produziria falso-positivo em toda instalação
// legítima cujo conteúdo bruto não fosse já exatamente TrimSpace+newline único. Resolução:
// usar o registro de quarentena (que persiste após o install e não é git-ignored) como ponte
// auditável entre os dois domínios — mesma leitura aplicada no Go, portada aqui 1:1.
function validateThirdPartyArtifactHasProvenance(cwd) {
  const root = cwd || process.cwd()

  const manifestPath = path.join(root, '.trackfw', 'integrations-manifest.json')
  let manifest
  try {
    const raw = fs.readFileSync(manifestPath, 'utf8')
    manifest = JSON.parse(raw)
  } catch (err) {
    if (err.code === 'ENOENT') return []
    throw new Error(`thirdparty_artifact_has_provenance: read ${manifestPath}: ${err.message}`)
  }

  const destinations = []
  for (const [destination, artifact] of Object.entries(manifest.artifacts || {})) {
    const claims = artifact.claims || []
    if (claims.some(claim => claim.origin === THIRDPARTY_ORIGIN)) destinations.push(destination)
  }
  if (destinations.length === 0) return []
  destinations.sort()

  let prov
  try {
    prov = loadProvenance(root)
  } catch (err) {
    throw new Error(`thirdparty_artifact_has_provenance: ${err.message}`)
  }

  const msgs = []
  for (const destination of destinations) {
    // Provenance keys are NOT the manifest's absolute destination —
    // verified empirically against the real install command
    // (npm/src/commands/thirdparty.js): verifyApproval/upsertProvenanceEntry
    // are called with the project-root-relative (or "~/"-prefixed,
    // global-scope) destination string BEFORE IntegrationManager.resolve()
    // joins it against root to produce the absolute manifest key. Every
    // claim reached here came from the PROJECT manifest, so its scope is
    // always "project" (a global-scope claim lives in the home manifest
    // instead, which this rule intentionally never reads). path.relative
    // inverts resolve()'s path.resolve(root, relative) exactly. Mirrors
    // internal/validator/validator_thirdparty_provenance.go.
    const provenanceKey = path.relative(root, destination)
    const entry = (prov.entries || {})[provenanceKey]
    if (!entry) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" is claimed as a third-party artifact but has no ` +
        'entry in .trackfw/thirdparty-provenance.json — obtain a favorable hades-tf review and record an ' +
        'approved provenance entry for this destination before this can pass validate (D2 branch i)'
      )
      continue
    }

    let quarantineEntry
    try {
      quarantineEntry = readQuarantine(root, entry.checksum_sha256)
    } catch (err) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" has a provenance entry for checksum ` +
        `${entry.checksum_sha256}, but .trackfw/thirdparty-quarantine/${entry.checksum_sha256}.json could not ` +
        `be read (${err.message}) — the quarantine record is required to verify the approval against the ` +
        'installed content (D2 branch ii, fail-closed per D8f)'
      )
      continue
    }

    const rawContent = decodeContent(quarantineEntry)
    if (sha256Hex(rawContent) !== entry.checksum_sha256) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" — quarantine record for checksum ` +
        `${entry.checksum_sha256} is not self-consistent (recomputed checksum does not match its own ` +
        'filename); the record may have been hand-edited'
      )
      continue
    }

    let installed
    try {
      installed = fs.readFileSync(destination)
    } catch (err) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" is claimed as a third-party artifact with an ` +
        `approved provenance entry, but the destination file could not be read (${err.message})`
      )
      continue
    }

    const expected = normalizeThirdPartyForValidation(rawContent)
    if (!installed.equals(expected)) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" — installed content does not match the checksum ` +
        `${entry.checksum_sha256} approved in .trackfw/thirdparty-provenance.json (verified via its quarantine ` +
        'record) — the artifact was modified after approval or installed outside the fetch/install flow ' +
        '(D2 branch ii)'
      )
    }
  }

  return msgs
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
  applyRule('adr_accepted_when_req_done', validateADRAcceptedWhenREQDone(), violations, warnings)
  applyRule('note_orphan',           validateNoteOrphan(),                 violations, warnings)
  applyRule('credential_guard_hook_resolvable', validateCredentialGuardHookResolvable().concat(validateCredentialGuardGlobalHookResolvable()), violations, warnings)
  applyRule('credential_guard_script_integrity', validateCredentialGuardScriptIntegrity().concat(validateCredentialGuardGlobalScriptIntegrity()), violations, warnings)
  applyRule('credential_guard_mode_downgrade', validateCredentialGuardModeDowngrade(), violations, warnings)
  // ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
  // ML-2A — port de internal/validator/validator.go's wiring das 4 regras/wrappers em
  // validator_git_branch_guard.go.
  applyRule('git_branch_guard_hook_resolvable', validateGitBranchGuardHookResolvable().concat(validateGitBranchGuardGlobalHookResolvable()), violations, warnings)
  applyRule('git_branch_guard_script_integrity', validateGitBranchGuardScriptIntegrity().concat(validateGitBranchGuardGlobalScriptIntegrity()), violations, warnings)

  // ADR-2026-08-15-gate-de-duas-fases-..., ML-3A (D2): detecção ancorada em git por trás do
  // guardrail TRACKFW_ORCHESTRATOR_SESSION.
  applyRule('thirdparty_artifact_has_provenance', validateThirdPartyArtifactHasProvenance(), violations, warnings)

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
//
// ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-
// e-disco: carve-out do baseline — violations/warnings de uma das 3
// CREDENTIAL_GUARD_ANCHORED_RULES NUNCA são toleradas via .trackfw-baseline.json, não importa o
// que o arquivo contenha para elas. Mecanismo DIFERENTE do HEAD-vs-disco em
// credentialGuardRuleSeverity: .trackfw-baseline.json é .gitignore'd DE PROPÓSITO ("baseline local
// de violations toleradas (nao versionado)"), então não há HEAD desse arquivo para comparar —
// "exigir commit" simplesmente não se aplica a um arquivo que o projeto decidiu nunca versionar. A
// única forma de fechar esse canal é excluir estas 3 regras da elegibilidade de ratchet, por nome,
// independente do conteúdo da mensagem — daí a checagem via getItemMeta(msg).rule abaixo.
async function validate() {
  const result = await validateUnfiltered()
  let { violations, warnings } = result

  // Ratchet: filtrar violations e warnings que já estavam no baseline
  const baseline = loadBaseline()
  if (baseline) {
    const baselineSet = new Set(baseline.violations || [])
    violations = violations.filter(v => !baselineSet.has(v) || CREDENTIAL_GUARD_ANCHORED_RULES.has(getItemMeta(v).rule))
    const baselineWarnSet = new Set(baseline.warnings || [])
    warnings = warnings.filter(w => !baselineWarnSet.has(w) || CREDENTIAL_GUARD_ANCHORED_RULES.has(getItemMeta(w).rule))
  }

  // Modo lenient: mover violations para warnings, exit code 0
  if (isLenient()) {
    warnings = [...warnings, ...violations]
    violations = []
  }

  return { violations, warnings }
}

// ROADMAP_STATES enumera os 6 estados de roadmap na ordem exibida pelo bloco Inventory.
const ROADMAP_STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']

// buildInventorySection monta o bloco "📊 Inventory" — contagem agregada de ADRs, REQs
// (discriminadas por status real via reqStatusEquals) e roadmaps (pelos 6 estados, incluindo
// "analyzing", historicamente omitido). Namespacing-agnóstico: agrega através de
// resolveStateDirs()/resolveReqFiles(), que já resolvem flat vs by_agent.
function buildInventorySection(cfg) {
  let adrCount = 0
  for (const adrDir of cfg.adrDirs || []) {
    adrCount += walkDirMdWithPathsForRule('status_inventory', adrDir, []).length
  }

  const reqFiles = resolveReqFiles(cfg)
  let reqOpen = 0
  let reqDone = 0
  let reqClosed = 0
  for (const filePath of reqFiles) {
    let content
    try {
      content = fs.readFileSync(filePath, 'utf8')
    } catch (_) {
      continue
    }
    if (reqStatusEquals(content, 'open')) reqOpen++
    else if (reqStatusEquals(content, 'done')) reqDone++
    else if (reqStatusEquals(content, 'closed')) reqClosed++
  }

  const roadmapCounts = {}
  let roadmapTotal = 0
  for (const state of ROADMAP_STATES) {
    let count = 0
    for (const dir of resolveStateDirs(cfg, state)) {
      count += listDir(dir).length
    }
    roadmapCounts[state] = count
    roadmapTotal += count
  }

  let section = '\n📊 Inventory\n'
  section += `   ${'ADRs'.padEnd(12)}${adrCount}\n`
  section += `   ${'REQs'.padEnd(12)}${reqFiles.length}  (${reqOpen} Open · ${reqDone} Done · ${reqClosed} Closed)\n`
  section += `   ${'Roadmaps'.padEnd(12)}${roadmapTotal}\n`
  section += `     backlog ${roadmapCounts.backlog} · analyzing ${roadmapCounts.analyzing} · wip ${roadmapCounts.wip}\n`
  section += `     blocked ${roadmapCounts.blocked} · done ${roadmapCounts.done} · abandoned ${roadmapCounts.abandoned}\n`
  return section
}

// getStatus retorna string formatada com o status de governança do projeto
async function getStatus() {
  const cfg = config.load()
  let out = '── trackfw status ──────────────────────\n'
  out += buildInventorySection(cfg)

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

    const wipCfg = wipConfigFrom(cfg)
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

    // Seção: REQs bloqueadas por ADRs não aceitos (Draft ou Proposed). O status exibido por
    // ADR é resolvido via adrNotAcceptedStatusForRule (helper canônico) em vez de hardcodar
    // "Draft" — blockedREQs() cobre ambos os status desde que delega em adrIsDraft/
    // adrDraftStatusForRule, e um rótulo fixo "(Draft)" mentiria para um ADR Proposed.
    const blockedByDraft = blockedREQs()
    const blockedKeys = Object.keys(blockedByDraft)
    if (blockedKeys.length > 0) {
      out += `\n⏳ REQs blocked by not-accepted ADRs (${blockedKeys.length})\n`
      for (const reqFile of blockedKeys) {
        out += `   ${reqFile}\n`
        for (const adr of blockedByDraft[reqFile]) {
          const { status } = adrNotAcceptedStatusForRule('blocked_by_draft_adr', adr, null)
          out += `     → ${adr} (${status})\n`
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
  resolveStateDirs,
  resolveWIPDirs,
  resolveDoneDirs,
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
  // novas funções — trackfw branch new (extraídas do gate branch_has_wip_roadmap)
  branchSlugMatchesRoadmap,
  branchGovernanceOrientation,
  branchNoMatchingRoadmapMessage,
  normalizeBranchSlug,
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
  // novas funções ML-1B (2026-08-01 — adr_accepted_when_req_done)
  extractAdrHeaderStatus,
  adrNotAcceptedStatusForRule,
  reqStatusEquals,
  validateADRAcceptedWhenREQDone,
  // ML-1D (2026-08-01 — reconciliação de paridade: frontmatter-first)
  extractFrontmatterField,
  resolveAdrStatus,
  // ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A
  resolveCredentialGuardHookPath,
  collectCredentialGuardCommands,
  validateCredentialGuardHookResolvable,
  // ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A
  validateCredentialGuardScriptIntegrity,
  validateCredentialGuardModeDowngrade,
  extractCredentialGuardMode,
  CREDENTIAL_GUARD_SCRIPT_REFERENCE,
  // ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
  // ML-2A
  collectCommandsWithMarker,
  validateGuardHookResolvable,
  validateGitBranchGuardHookResolvable,
  validateGitBranchGuardScriptIntegrity,
  GIT_BRANCH_GUARD_SCRIPT_REFERENCE,
  CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE,
  GLOBAL_GUARD_CONFIG_FILES,
  validateGuardGlobalHookResolvable,
  validateGuardGlobalScriptIntegrity,
  // ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas, ML-3A
  validateThirdPartyArtifactHasProvenance,
  normalizeThirdPartyForValidation,
  validateCredentialGuardGlobalHookResolvable,
  validateCredentialGuardGlobalScriptIntegrity,
  validateGitBranchGuardGlobalHookResolvable,
  validateGitBranchGuardGlobalScriptIntegrity,
}
