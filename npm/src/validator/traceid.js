'use strict'

const fs = require('fs')
const path = require('path')

// ESTADOS reconhecidos para REQs e Roadmaps (baseado na pasta onde o arquivo reside)
const KNOWN_STATES = ['wip', 'backlog', 'blocked', 'done', 'abandoned']

// extractFrontmatterField extrai o valor de um campo do frontmatter YAML simples de um arquivo .md.
// Retorna string vazia se o campo não for encontrado ou o arquivo não tiver frontmatter.
function extractFrontmatterField(filePath, fieldName) {
  let content
  try { content = fs.readFileSync(filePath, 'utf8') } catch (_) { return '' }
  if (!content.startsWith('---')) return ''
  const end = content.indexOf('\n---', 3)
  const block = end > 0 ? content.slice(3, end) : content.slice(3)
  for (const line of block.split('\n')) {
    const trimmed = line.trim()
    const prefix = fieldName + ':'
    if (trimmed.startsWith(prefix)) {
      return trimmed.slice(prefix.length).trim().replace(/^["']|["']$/g, '')
    }
  }
  return ''
}

// stateFromPath extrai o estado (wip/backlog/blocked/done/abandoned) a partir do caminho do arquivo.
// Percorre os segmentos de path de trás para frente e retorna o primeiro que for um estado reconhecido.
// Retorna '' se nenhum segmento for reconhecido.
function stateFromPath(filePath) {
  const segments = filePath.split(path.sep)
  for (let i = segments.length - 2; i >= 0; i--) {
    if (KNOWN_STATES.includes(segments[i])) return segments[i]
  }
  return ''
}

// walkMd retorna array de caminhos absolutos de todos .md recursivamente dentro de dir.
function walkMd(dir) {
  const results = []
  function walk(d) {
    let entries
    try { entries = fs.readdirSync(d) } catch (_) { return }
    for (const name of entries) {
      const full = path.join(d, name)
      try {
        if (fs.statSync(full).isDirectory()) { walk(full) }
        else if (name.endsWith('.md')) { results.push(full) }
      } catch (_) {}
    }
  }
  walk(dir)
  return results
}

// checkTraceIds verifica a consistência bidirecional de req_id entre REQs e Roadmaps.
// Parâmetros:
//   reqDir     — caminho absoluto ou relativo do diretório de REQs
//   roadmapDir — caminho absoluto ou relativo do diretório de Roadmaps
//   fieldName  — nome do campo de frontmatter que contém o trace ID (ex: 'req_id')
// Retorna array de strings de violation.
function checkTraceIds(reqDir, roadmapDir, fieldName) {
  if (!fieldName) return []

  // --- Indexar REQs ---
  // reqIndex: Map<traceId, [{file, state}]>
  const reqIndex = new Map()
  for (const filePath of walkMd(reqDir)) {
    const traceId = extractFrontmatterField(filePath, fieldName)
    if (!traceId) continue
    const state = stateFromPath(filePath)
    if (!reqIndex.has(traceId)) reqIndex.set(traceId, [])
    reqIndex.get(traceId).push({ file: path.basename(filePath), state })
  }

  // --- Indexar Roadmaps ---
  // roadmapIndex: Map<traceId, [{file, state}]>
  const roadmapIndex = new Map()
  for (const filePath of walkMd(roadmapDir)) {
    const traceId = extractFrontmatterField(filePath, fieldName)
    if (!traceId) continue
    const state = stateFromPath(filePath)
    if (!roadmapIndex.has(traceId)) roadmapIndex.set(traceId, [])
    roadmapIndex.get(traceId).push({ file: path.basename(filePath), state })
  }

  // Salvaguarda: trace_id_field configurado mas nenhum arquivo indexado
  if (reqIndex.size === 0 && roadmapIndex.size === 0) {
    return ['traceid_config_warning: trace_id_field is set but no REQ/Roadmap entries were indexed — check reqDir, roadmapDir and roadmap_namespacing']
  }

  const violations = []

  // traceid_duplicate_req: mesmo req_id em >1 REQ
  for (const [traceId, entries] of reqIndex.entries()) {
    if (entries.length > 1) {
      const files = entries.map(e => e.file).join(', ')
      violations.push(`traceid_duplicate_req: req_id "${traceId}" appears in ${entries.length} REQs: ${files}`)
    }
  }

  // traceid_duplicate_roadmap: mesmo req_id em >1 Roadmap
  for (const [traceId, entries] of roadmapIndex.entries()) {
    if (entries.length > 1) {
      const files = entries.map(e => e.file).join(', ')
      violations.push(`traceid_duplicate_roadmap: req_id "${traceId}" appears in ${entries.length} Roadmaps: ${files}`)
    }
  }

  // traceid_orphan_roadmap: Roadmap com req_id sem REQ correspondente
  for (const [traceId, entries] of roadmapIndex.entries()) {
    if (!reqIndex.has(traceId)) {
      for (const e of entries) {
        violations.push(`traceid_orphan_roadmap: roadmap "${e.file}" has req_id "${traceId}" but no matching REQ`)
      }
    }
  }

  // traceid_orphan_req: REQ com req_id sem Roadmap correspondente
  for (const [traceId, entries] of reqIndex.entries()) {
    if (!roadmapIndex.has(traceId)) {
      for (const e of entries) {
        violations.push(`traceid_orphan_req: req "${e.file}" has req_id "${traceId}" but no matching Roadmap`)
      }
    }
  }

  // traceid_state_mismatch: REQ e Roadmap com mesmo req_id em estados diferentes
  for (const [traceId, reqEntries] of reqIndex.entries()) {
    if (!roadmapIndex.has(traceId)) continue
    const roadmapEntries = roadmapIndex.get(traceId)
    // Comparar todos os pares (normalmente 1x1, mas suporta duplicados já reportados acima)
    for (const req of reqEntries) {
      for (const rm of roadmapEntries) {
        if (req.state && rm.state && req.state !== rm.state) {
          violations.push(
            `traceid_state_mismatch: req_id "${traceId}" — REQ "${req.file}" is in "${req.state}" but Roadmap "${rm.file}" is in "${rm.state}"`
          )
        }
      }
    }
  }

  return violations
}

module.exports = { checkTraceIds, extractFrontmatterField, stateFromPath }
