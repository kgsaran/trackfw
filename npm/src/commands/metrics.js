'use strict'

const { Command } = require('commander')
const fs = require('fs')
const path = require('path')
const { t } = require('../i18n')

// lineRe: faz match do formato do .trackfw-log
// 2026-06-12 14:30  ROADMAP-2026-06-12-auth.md                  backlog → wip
const LINE_RE = /^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s{2,}(\S+)\s{2,}(\S+)\s+→\s+(\S+)/

/**
 * parseLog lê o arquivo .trackfw-log e retorna array de transições.
 * Retorna [] se o arquivo não existe.
 * @param {string} filePath
 * @returns {{ timestamp: Date, basename: string, from: string, to: string }[]}
 */
function parseLog(filePath) {
  if (!fs.existsSync(filePath)) return []
  const content = fs.readFileSync(filePath, 'utf8')
  const lines = content.split('\n')
  const transitions = []
  for (const line of lines) {
    if (!line.trim()) continue
    const m = LINE_RE.exec(line)
    if (!m) continue
    const [, tsStr, basename, from, to] = m
    const timestamp = new Date(tsStr.replace(' ', 'T') + ':00')
    if (isNaN(timestamp.getTime())) continue
    transitions.push({ timestamp, basename: basename.trim(), from: from.trim(), to: to.trim() })
  }
  return transitions
}

/**
 * filter retorna transições com timestamp >= sinceMs.
 * @param {Array} transitions
 * @param {number} sinceMs - timestamp em milissegundos
 * @returns {Array}
 */
function filter(transitions, sinceMs) {
  return transitions.filter(t => t.timestamp.getTime() >= sinceMs)
}

/**
 * calculate computa cycle time médio, throughput e WIP age.
 * @param {Array} transitions
 * @returns {{ cycleTimeMeanMs: number, throughput: number, wipEntries: Array }}
 */
function calculate(transitions) {
  // Agrupar por basename
  const byName = new Map()
  for (const tr of transitions) {
    if (!byName.has(tr.basename)) byName.set(tr.basename, [])
    byName.get(tr.basename).push(tr)
  }

  // Cycle time: da entrada em backlog ou wip até done
  const cycleTimes = []
  for (const [, entries] of byName) {
    let startTs = null
    let doneTs = null
    for (const e of entries) {
      if ((e.to === 'backlog' || e.to === 'wip') && startTs === null) {
        startTs = e.timestamp.getTime()
      }
      if (e.to === 'done') {
        doneTs = e.timestamp.getTime()
      }
    }
    if (startTs !== null && doneTs !== null) {
      cycleTimes.push(doneTs - startTs)
    }
  }

  let cycleTimeMeanMs = 0
  if (cycleTimes.length > 0) {
    cycleTimeMeanMs = cycleTimes.reduce((a, b) => a + b, 0) / cycleTimes.length
  }

  // Throughput: roadmaps done por semana
  let doneCount = 0
  let minTs = Infinity
  let maxTs = -Infinity
  for (const tr of transitions) {
    const ms = tr.timestamp.getTime()
    if (tr.to === 'done') doneCount++
    if (ms < minTs) minTs = ms
    if (ms > maxTs) maxTs = ms
  }

  let throughput = 0
  if (doneCount > 0) {
    const msPerWeek = 7 * 24 * 60 * 60 * 1000
    let weeks = (maxTs - minTs) / msPerWeek
    if (weeks < 1) weeks = 1
    throughput = doneCount / weeks
  }

  // WIP age: basenames em wip sem done ou abandoned posterior
  const wipEntries = []
  const now = Date.now()
  for (const [basename, entries] of byName) {
    let wipTs = null
    let concluded = false
    for (const e of entries) {
      if (e.to === 'wip') wipTs = e.timestamp.getTime()
      if (e.to === 'done' || e.to === 'abandoned') concluded = true
    }
    if (wipTs !== null && !concluded) {
      wipEntries.push({ basename, ageMs: now - wipTs })
    }
  }

  return { cycleTimeMeanMs, throughput, wipEntries }
}

/**
 * exportCSV grava transições e métricas em um arquivo CSV.
 * @param {{ cycleTimeMeanMs: number, throughput: number, wipEntries: Array }} metrics
 * @param {Array} transitions
 * @param {string} filePath
 */
function exportCSV(metrics, transitions, filePath) {
  const rows = []
  rows.push('basename,from,to,timestamp')
  for (const tr of transitions) {
    const ts = tr.timestamp.toISOString().slice(0, 16).replace('T', ' ')
    rows.push(`${tr.basename},${tr.from},${tr.to},${ts}`)
  }
  rows.push('')
  rows.push('metric,value')
  const cycleHours = (metrics.cycleTimeMeanMs / (1000 * 3600)).toFixed(2)
  rows.push(`cycle_time_mean_hours,${cycleHours}`)
  rows.push(`throughput_per_week,${metrics.throughput.toFixed(2)}`)
  rows.push(`wip_count,${metrics.wipEntries.length}`)
  fs.writeFileSync(filePath, rows.join('\n') + '\n', 'utf8')
}

/**
 * parseSinceDuration converte "7d", "30d", "90d" em milissegundos.
 * @param {string} s
 * @returns {number}
 */
function parseSinceDuration(s) {
  if (!s || s.length < 2) throw new Error(`formato inválido: "${s}"`)
  const unit = s[s.length - 1]
  if (unit !== 'd') throw new Error(`unidade não suportada "${unit}" (use 'd' para dias)`)
  const n = parseInt(s.slice(0, -1), 10)
  if (isNaN(n) || n <= 0) throw new Error(`número inválido em "${s}"`)
  return n * 24 * 60 * 60 * 1000
}

/**
 * formatDuration formata milissegundos em string legível.
 * @param {number} ms
 * @returns {string}
 */
function formatDuration(ms) {
  const totalHours = Math.floor(ms / (1000 * 3600))
  const days = Math.floor(totalHours / 24)
  const hours = totalHours % 24
  if (days > 0) return `${days} days ${hours} hours`
  return `${hours} hours`
}

/**
 * printMetrics imprime as métricas em formato tabela ASCII.
 * @param {{ cycleTimeMeanMs: number, throughput: number, wipEntries: Array }} metrics
 */
function printMetrics(metrics) {
  console.log('── trackfw metrics ──────────────────────')

  if (metrics.cycleTimeMeanMs > 0) {
    console.log(`  Cycle Time Mean   : ${formatDuration(metrics.cycleTimeMeanMs)}`)
  } else {
    console.log('  Cycle Time Mean   : n/a (no completed cycles)')
  }

  if (metrics.throughput > 0) {
    console.log(`  Throughput        : ${metrics.throughput.toFixed(2)} roadmaps/week`)
  } else {
    console.log('  Throughput        : n/a (no completed roadmaps)')
  }

  if (metrics.wipEntries.length === 0) {
    console.log('  WIP Age           : no items in progress')
  } else {
    console.log(`  WIP Age (${metrics.wipEntries.length} items) :`)
    for (const w of metrics.wipEntries) {
      console.log(`    - ${w.basename}: ${formatDuration(w.ageMs)}`)
    }
  }

  console.log('─────────────────────────────────────────')
}

const cmd = new Command('metrics')
cmd.description(t('metrics.description'))
cmd.option('--since <period>', t('metrics.since'))
cmd.option('--export <file>', t('metrics.export'))
cmd.action((opts) => {
  const logPath = path.join('docs', 'roadmaps', '.trackfw-log')
  let transitions = parseLog(logPath)

  if (transitions.length === 0) {
    console.log(t('metrics.no_data'))
    return
  }

  if (opts.since) {
    let sinceMs
    try {
      sinceMs = parseSinceDuration(opts.since)
    } catch (err) {
      console.error(`invalid --since format (use: 7d, 30d, 90d): ${err.message}`)
      process.exit(1)
    }
    transitions = filter(transitions, Date.now() - sinceMs)
    if (transitions.length === 0) {
      console.log(t('metrics.no_data'))
      return
    }
  }

  const m = calculate(transitions)
  printMetrics(m)

  if (opts.export) {
    exportCSV(m, transitions, opts.export)
    console.log(`exported to ${opts.export}`)
  }
})

module.exports = cmd
