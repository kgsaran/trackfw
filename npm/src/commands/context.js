'use strict'

const { Command } = require('commander')
const fs = require('fs')
const path = require('path')
const config = require('../config')
const { validate } = require('../validator')

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
 * getContext — coleta governança e imprime em md ou json.
 * @param {string} format - 'md' | 'json'
 */
function getContext(format) {
  const cfg = config.load()

  // ADRs
  const adrs = []
  for (const adrDir of (cfg.adrDirs || ['docs/adr'])) {
    adrs.push(...collectEntries(adrDir, 'ADR'))
  }

  // REQs
  const reqs = collectEntries(cfg.reqDir || 'docs/req', 'REQ')

  // Roadmaps
  const roadmaps = []
  const states = ['wip', 'backlog', 'blocked', 'done', 'abandoned']
  if (cfg.roadmapNamespacing === 'by_agent') {
    let agents = cfg.agents || []
    if (agents.length === 0) {
      try {
        agents = fs.readdirSync(cfg.roadmapDir).filter(f => {
          try { return fs.statSync(path.join(cfg.roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = [] }
    }
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
  const { violations, warnings } = validate()

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
    .action((opts) => {
      getContext(opts.format)
    })
  return cmd
})()
