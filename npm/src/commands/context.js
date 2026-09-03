'use strict'

const { Command } = require('commander')
const fs = require('fs')
const path = require('path')
const config = require('../config')
const { validate, resolveAgentNamespaces, resolveReqFiles } = require('../validator')

/**
 * extractFrontmatterField — extrai valor de campo YAML dentro de bloco --- ... ---.
 * Retorna string vazia se não encontrado ou valor '""'.
 * @param {string} content
 * @param {string} field
 * @returns {string}
 */
function extractFrontmatterField(content, field) {
  const lines = content.split('\n')
  let started = false
  let inFrontmatter = false
  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed === '---') {
      if (!started) {
        started = true
        inFrontmatter = true
        continue
      }
      break // segundo --- fecha o bloco
    }
    if (!inFrontmatter) break
    const key = field + ':'
    if (trimmed.startsWith(key)) {
      let val = trimmed.slice(key.length).trim()
      val = val.replace(/^["']|["']$/g, '') // remover aspas
      return val
    }
  }
  return ''
}

/**
 * extractInlineStatus — extrai status da linha "| Status: ..." do markdown.
 * @param {string} content
 * @returns {string}
 */
function extractInlineStatus(content) {
  for (const line of content.split('\n')) {
    const idx = line.indexOf('| Status: ')
    if (idx >= 0) {
      let rest = line.slice(idx + '| Status: '.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) rest = rest.slice(0, pipeIdx)
      rest = rest.replace(/[\s>|]+$/, '').trim()
      return rest || 'unknown'
    }
  }
  return 'unknown'
}

/**
 * collectEntries — lê diretório e retorna lista de entradas com type, file, status, state.
 * @param {string} dir
 * @param {string} type - 'ADR' | 'REQ' | 'ROADMAP'
 * @param {string} [state] - estado kanban (somente ROADMAP)
 * @returns {Array<{type: string, file: string, status: string, state?: string}>}
 */
function collectEntries(dir, type, state) {
  const entries = []
  let files = []
  try {
    files = fs.readdirSync(dir).filter(f => f.endsWith('.md') && !fs.statSync(path.join(dir, f)).isDirectory())
  } catch (_) {
    return entries
  }
  for (const file of files) {
    let content = ''
    try { content = fs.readFileSync(path.join(dir, file), 'utf8') } catch (_) {}
    let status = extractFrontmatterField(content, 'status')
    if (!status) status = extractInlineStatus(content)
    if (!status) status = state || 'unknown'
    const entry = { type, file, status }
    if (state) entry.state = state
    entries.push(entry)
  }
  return entries
}

/**
 * collectReqEntries — entradas de REQ para o `context`, derivadas do ponto único de leitura
 * (resolveReqFiles). O `state` sai do nome da pasta-pai quando ela é um estado legado; em layout
 * flat ou no canônico req_dir/<agente>/*.md não há estado (invariante D1: REQ não tem estado).
 * @param {object} cfg
 * @returns {Array<{type:string,file:string,status:string,state?:string}>}
 */
function collectReqEntries(cfg) {
  const STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']
  const entries = []
  for (const full of resolveReqFiles(cfg)) {
    let content = ''
    try { content = fs.readFileSync(full, 'utf8') } catch (_) {}
    const parent = path.basename(path.dirname(full))
    const state = STATES.includes(parent) ? parent : ''
    let status = extractFrontmatterField(content, 'status')
    if (!status) status = extractInlineStatus(content)
    if (!status) status = state || 'unknown'
    const entry = { type: 'REQ', file: path.basename(full), status }
    if (state) entry.state = state
    entries.push(entry)
  }
  return entries
}

/**
 * getContext — coleta governança e imprime em md ou json.
 * @param {string} format - 'md' | 'json'
 * @returns {Promise<void>}
 */
async function getContext(format) {
  const cfg = config.load()

  // ADRs
  const adrs = []
  for (const adrDir of (cfg.adrDirs || ['docs/adr'])) {
    adrs.push(...collectEntries(adrDir, 'ADR'))
  }

  // REQs — pelo PONTO ÚNICO de leitura (ADR-2026-09-03, D3/D4). Antes, `context` montava a própria
  // árvore (flat, ou <agente>/<estado>/ em by_agent) e não enxergava o layout canônico <agente>/*.md.
  const reqs = collectReqEntries(cfg)

  // Roadmaps
  const roadmaps = []
  const states = ['wip', 'backlog', 'blocked', 'done', 'abandoned']
  if (cfg.roadmapNamespacing === 'by_agent') {
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
    for (const agent of agents) {
      for (const state of states) {
        const dir = path.join(cfg.roadmapDir, agent, state)
        roadmaps.push(...collectEntries(dir, 'ROADMAP', state))
      }
    }
  } else {
    for (const state of states) {
      const dir = path.join(cfg.roadmapDir, state)
      roadmaps.push(...collectEntries(dir, 'ROADMAP', state))
    }
  }

  // Validate
  const { violations, warnings } = await validate()

  // Score
  let score = 0
  if (adrs.length > 0) score += 20
  if (reqs.length > 0) score += 20
  if (roadmaps.length > 0) score += 20
  if (violations.length === 0) score += 40

  if (format === 'json') {
    console.log(JSON.stringify({ score, violations, warnings, adrs, reqs, roadmaps }, null, 2))
    return
  }

  // Markdown
  console.log('# trackfw governance context\n')
  console.log(`**Governance score:** ${score}/100\n`)

  console.log(`## ADRs (${adrs.length})`)
  if (adrs.length === 0) {
    console.log('- (none)')
  } else {
    for (const a of adrs) console.log(`- ${a.file} [${a.status}]`)
  }

  console.log(`\n## REQs (${reqs.length})`)
  if (reqs.length === 0) {
    console.log('- (none)')
  } else {
    for (const r of reqs) console.log(`- ${r.file} [${r.status}]`)
  }

  console.log(`\n## Roadmaps (${roadmaps.length})`)
  if (roadmaps.length === 0) {
    console.log('- (none)')
  } else {
    for (const r of roadmaps) console.log(`- ${r.file} [${r.state}]`)
  }

  if (violations.length > 0) {
    console.log(`\n## Violations (${violations.length})`)
    for (const v of violations) console.log(`- ${v}`)
  }

  if (warnings.length > 0) {
    console.log(`\n## Warnings (${warnings.length})`)
    for (const w of warnings) console.log(`- ${w}`)
  }
}

module.exports = (function () {
  const cmd = new Command('context')
  cmd
    .description('Print governance context for LLM consumption')
    .option('--format <fmt>', 'Output format: md or json', 'md')
    // Retorna a Promise: o bin roda parseAsync() e só assim uma rejeição chega
    // ao .catch(reportFatalError) com exit code 1 em vez de virar unhandled
    // rejection (ML-1A, ROADMAP-2026-09-02-context-do-cli-node-aguarda-validate).
    .action((opts) => getContext(opts.format))
  return cmd
})()

// Exposto para teste: o teste de context em by_agent duplicava a lógica de coleta e, por isso,
// continuava verde mesmo quando a produção mudava (asserção sobre uma CÓPIA). Anexar a função ao
// Command exportado é o menor caminho para o teste exercitar o código de produção de fato.
module.exports.collectReqEntries = collectReqEntries
