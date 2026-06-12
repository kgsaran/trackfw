'use strict'

const fs = require('fs')
const path = require('path')

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

// adrIsDraft verifica se docs/adr/<basename> contém "Status: Draft".
function adrIsDraft(basename) {
  try {
    const content = fs.readFileSync(path.join('docs', 'adr', basename), 'utf8')
    return content.includes('Status: Draft')
  } catch (_) {
    return false
  }
}

// validateWIPHasREQ — roadmaps em docs/roadmaps/wip/ sem "REQ:" no conteúdo → violation
function validateWIPHasREQ() {
  const entries = listDir('docs/roadmaps/wip')
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join('docs/roadmaps/wip', name), 'utf8')
      if (!content.includes('REQ:') || content.includes('REQ: \n')) {
        violations.push(`roadmap "${name}" is in wip but has no linked REQ`)
      }
    } catch (_) {
      // ignorar erro de leitura
    }
  }
  return violations
}

// validateREQsHaveADR — REQs em docs/req/ sem "ADR:" no conteúdo → violation
function validateREQsHaveADR() {
  const entries = listDir('docs/req')
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join('docs/req', name), 'utf8')
      if (!content.includes('ADR:') || content.includes('ADR: \n')) {
        violations.push(`req "${name}" has no linked ADR`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateBlockedHasREQ — roadmaps em docs/roadmaps/blocked/ sem "REQ:" → violation
function validateBlockedHasREQ() {
  const entries = listDir('docs/roadmaps/blocked')
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join('docs/roadmaps/blocked', name), 'utf8')
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
  const entries = listDir('docs/req')
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join('docs/req', name), 'utf8')
      if (!content.includes('Roadmap:') || content.includes('Roadmap: \n')) {
        violations.push(`req "${name}" has no linked Roadmap`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateADRsAreReferenced — ADRs em docs/adr/ não referenciados em nenhuma REQ → violation
function validateADRsAreReferenced() {
  const adrs = listDir('docs/adr')
  const reqEntries = listDir('docs/req')

  let combined = ''
  for (const name of reqEntries) {
    try {
      combined += fs.readFileSync(path.join('docs/req', name), 'utf8')
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
function validateWIPHasAcceptanceCriteria() {
  const entries = listDir('docs/roadmaps/wip')
  const violations = []
  for (const name of entries) {
    try {
      const content = fs.readFileSync(path.join('docs/roadmaps/wip', name), 'utf8')
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
  return violations
}

// validateSingleWIP — mais de 1 roadmap em wip → warning
function validateSingleWIP() {
  const entries = listDir('docs/roadmaps/wip')
  if (entries.length > 1) {
    return [`${entries.length} roadmaps in wip/ (recommended: keep only 1 active at a time)`]
  }
  return []
}

// validateStaleWIP — roadmaps wip com mtime >= 7 dias → warning
function validateStaleWIP() {
  let files = []
  try {
    files = fs.readdirSync('docs/roadmaps/wip')
      .filter(f => f.endsWith('.md'))
      .map(f => path.join('docs/roadmaps/wip', f))
  } catch (_) {
    return []
  }

  const warnings = []
  const now = Date.now()
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
  return warnings
}

// validateREQsNotBlockedByDraftADRs — REQs Open com ADRs Draft na seção "## Blocked by ADRs" → violation
function validateREQsNotBlockedByDraftADRs() {
  const entries = listDir('docs/req')
  const violations = []
  for (const name of entries) {
    const filePath = path.join('docs/req', name)
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
  const entries = listDir('docs/req')
  const result = {}
  for (const name of entries) {
    const filePath = path.join('docs/req', name)
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

// validate executa todas as validações e retorna { violations, warnings }
async function validate() {
  const violations = [
    ...validateWIPHasREQ(),
    ...validateREQsHaveADR(),
    ...validateBlockedHasREQ(),
    ...validateREQsHaveRoadmap(),
    ...validateADRsAreReferenced(),
    ...validateWIPHasAcceptanceCriteria(),
    ...validateREQsNotBlockedByDraftADRs(),
  ]
  const warnings = [
    ...validateSingleWIP(),
    ...validateStaleWIP(),
  ]
  return { violations, warnings }
}

// getStatus retorna string formatada com o status de governança do projeto
async function getStatus() {
  const wip = listDir('docs/roadmaps/wip')
  const blocked = listDir('docs/roadmaps/blocked')
  const done = listDir('docs/roadmaps/done')

  let out = ''
  out += '── trackfw status ──────────────────────\n'

  out += `\n🔄 WIP (${wip.length})\n`
  for (const f of wip) out += `   ${f}\n`

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

  out += '\n────────────────────────────────────────\n'
  return out
}

module.exports = {
  validate,
  getStatus,
  // exportadas para testes unitários
  validateWIPHasREQ,
  validateREQsHaveADR,
  validateBlockedHasREQ,
  validateREQsHaveRoadmap,
  validateADRsAreReferenced,
  validateWIPHasAcceptanceCriteria,
  validateSingleWIP,
  validateStaleWIP,
  validateREQsNotBlockedByDraftADRs,
  parseBlockedADRs,
  adrIsDraft,
  listDir,
}
