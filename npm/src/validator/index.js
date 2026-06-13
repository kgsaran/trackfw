'use strict'

const fs = require('fs')
const path = require('path')
const config = require('../config')

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

// adrIsDraft verifica se <adrBasename> contém "Status: Draft" em alguma das adrDirs configuradas.
function adrIsDraft(basename) {
  const cfg = config.load()
  for (const adrDir of cfg.adrDirs) {
    const p = path.join(adrDir, basename)
    if (fs.existsSync(p)) {
      try {
        return fs.readFileSync(p, 'utf8').includes('Status: Draft')
      } catch (_) {
        // ignorar erro de leitura
      }
    }
  }
  return false
}

// validateWIPHasREQ — roadmaps em wip/ sem "REQ:" no conteúdo → violation
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
        if (!content.includes('REQ:') || content.includes('REQ: \n')) {
          violations.push(`roadmap "${name}" is in wip but has no linked REQ`)
        }
      } catch (_) {
        // ignorar erro de leitura
      }
    }
  }
  return violations
}

// validateREQsHaveADR — REQs em <reqDir>/ sem "ADR:" no conteúdo → violation
function validateREQsHaveADR() {
  const cfg = config.load()
  const entries = listDir(cfg.reqDir)
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join(cfg.reqDir, name), 'utf8')
      if (!content.includes('ADR:') || content.includes('ADR: \n')) {
        violations.push(`req "${name}" has no linked ADR`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateBlockedHasREQ — roadmaps em <roadmapDir>/blocked/ sem "REQ:" → violation
function validateBlockedHasREQ() {
  const cfg = config.load()
  const entries = listDir(cfg.roadmapDir + '/blocked')
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join(cfg.roadmapDir + '/blocked', name), 'utf8')
      if (!content.includes('REQ:') || content.includes('REQ: \n')) {
        violations.push(`roadmap "${name}" is in blocked but has no linked REQ`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateREQsHaveRoadmap — REQs sem "Roadmap:" → violation
function validateREQsHaveRoadmap() {
  const cfg = config.load()
  const entries = listDir(cfg.reqDir)
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join(cfg.reqDir, name), 'utf8')
      if (!content.includes('Roadmap:') || content.includes('Roadmap: \n')) {
        violations.push(`req "${name}" has no linked Roadmap`)
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
    adrs = adrs.concat(listDir(adrDir))
  }

  const reqEntries = listDir(cfg.reqDir)
  let combined = ''
  for (const name of reqEntries) {
    try {
      combined += fs.readFileSync(path.join(cfg.reqDir, name), 'utf8')
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
        const hasBlock =
          content.includes('## Acceptance Criteria') ||
          content.includes('## Critérios de Aceite') ||
          content.includes('acceptance criteria') ||
          content.includes('Acceptance Criteria:')
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
        const ageMs = now - stat.mtimeMs
        const days = Math.floor(ageMs / (1000 * 60 * 60 * 24))
        if (days >= STALE_WIP_DAYS) {
          const lastModified = stat.mtime.toISOString().slice(0, 10)
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
  const entries = listDir(cfg.reqDir)
  const violations = []
  for (const name of entries) {
    const filePath = path.join(cfg.reqDir, name)
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
        violations.push(`REQ ${name} is blocked by Draft ADR: ${adrBasename}`)
      }
    }
  }
  return violations
}

// blockedREQs retorna mapa de reqBasename → [adrBasenames Draft] para uso em getStatus()
function blockedREQs() {
  const cfg = config.load()
  const entries = listDir(cfg.reqDir)
  const result = {}
  for (const name of entries) {
    const filePath = path.join(cfg.reqDir, name)
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
      result[name] = draftADRs
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

// validate executa todas as validações e retorna { violations, warnings }
async function validate() {
  const wipLimitResult = validateWIPLimit()
  let violations = [
    ...validateWIPHasREQ(),
    ...validateREQsHaveADR(),
    ...validateBlockedHasREQ(),
    ...validateREQsHaveRoadmap(),
    ...validateADRsAreReferenced(),
    ...validateWIPHasAcceptanceCriteria(),
    ...validateREQsNotBlockedByDraftADRs(),
    ...wipLimitResult.violations,
  ]
  let warnings = [
    ...wipLimitResult.warnings,
    ...validateStaleWIP(),
  ]
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
  resolveWIPDirs,
  readGovernanceMode,
  readWIPConfig,
  parseSquadFromFrontmatter,
}
