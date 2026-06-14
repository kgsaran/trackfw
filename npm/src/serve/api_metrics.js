'use strict'

const fs = require('fs')
const path = require('path')
const { parseLog, calculate } = require('../commands/metrics')

const STATES = ['wip', 'backlog', 'blocked', 'done', 'abandoned']
const MS_PER_DAY = 24 * 60 * 60 * 1000

/**
 * countFilesInState conta arquivos .md em um diretório (sem recursão).
 * @param {string} dir
 * @returns {number}
 */
function countFilesInState(dir) {
  try {
    return fs.readdirSync(dir).filter(f => {
      if (!f.endsWith('.md')) return false
      try { return !fs.statSync(path.join(dir, f)).isDirectory() } catch (_) { return false }
    }).length
  } catch (_) {
    return 0
  }
}

/**
 * buildStateDistribution conta roadmaps por estado.
 * @param {object} cfg
 * @returns {Object<string, number>}
 */
function buildStateDistribution(cfg) {
  const dist = { wip: 0, backlog: 0, blocked: 0, done: 0, abandoned: 0 }
  const roadmapDir = cfg.roadmapDir || 'docs/roadmaps'
  const namespacing = cfg.roadmapNamespacing || 'flat'

  if (namespacing === 'by_agent') {
    let agents = cfg.agents || []
    if (!agents.length) {
      try {
        agents = fs.readdirSync(roadmapDir).filter(f => {
          try { return fs.statSync(path.join(roadmapDir, f)).isDirectory() } catch (_) { return false }
        })
      } catch (_) { agents = [] }
    }
    for (const agent of agents) {
      for (const state of STATES) {
        dist[state] = (dist[state] || 0) + countFilesInState(path.join(roadmapDir, agent, state))
      }
    }
  } else {
    for (const state of STATES) {
      dist[state] = (dist[state] || 0) + countFilesInState(path.join(roadmapDir, state))
    }
  }

  return dist
}

/**
 * buildBurndown calcula pontos semanais a partir do log de transições.
 * @param {Array} transitions
 * @returns {Array<{date: string, open: number, closed: number}>}
 */
function buildBurndown(transitions) {
  if (!transitions.length) return []

  let minTs = Infinity
  let maxTs = -Infinity
  for (const tr of transitions) {
    const ms = tr.timestamp.getTime()
    if (ms < minTs) minTs = ms
    if (ms > maxTs) maxTs = ms
  }

  // Agrupar por semana (domingo como início)
  const weekMap = new Map()

  function weekKey(ms) {
    const d = new Date(ms)
    // arredondar para o domingo anterior
    const day = d.getDay() // 0=domingo
    const sunday = new Date(ms - day * MS_PER_DAY)
    return sunday.toISOString().slice(0, 10)
  }

  for (const tr of transitions) {
    const wk = weekKey(tr.timestamp.getTime())
    if (!weekMap.has(wk)) weekMap.set(wk, { open: 0, closed: 0 })
    const entry = weekMap.get(wk)
    if (tr.to === 'done' || tr.to === 'abandoned') {
      entry.closed++
    } else if (tr.to === 'wip' || tr.to === 'backlog') {
      entry.open++
    }
  }

  const sorted = Array.from(weekMap.entries()).sort(([a], [b]) => a.localeCompare(b))
  return sorted.map(([date, v]) => ({ date, open: v.open, closed: v.closed }))
}

/**
 * handleMetrics responde ao GET /api/metrics com JSON de métricas.
 * @param {object} cfg
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 */
function handleMetrics(cfg, req, res) {
  const roadmapDir = cfg.roadmapDir || 'docs/roadmaps'
  const logPath = path.join(roadmapDir, '.trackfw-log')

  const transitions = parseLog(logPath)
  const metrics = calculate(transitions)

  // Converter cycle time de ms para dias
  const cycleTimeAvgDays = metrics.cycleTimeMeanMs > 0
    ? parseFloat((metrics.cycleTimeMeanMs / MS_PER_DAY).toFixed(2))
    : 0

  // Lead time: da criação (primeira entrada em qualquer estado) até done
  // Usando as mesmas transições — aproximar como cycle time se não houver campo de criação
  // (o campo creation_date não está no log; usamos start → done como lead time também)
  const leadTimeAvgDays = cycleTimeAvgDays

  // Abandonment rate: abandonados / (done + abandonados)
  const dist = buildStateDistribution(cfg)
  const totalCompleted = (dist.done || 0) + (dist.abandoned || 0)
  const abandonmentRate = totalCompleted > 0
    ? parseFloat(((dist.abandoned || 0) / totalCompleted).toFixed(4))
    : 0

  const burndown = buildBurndown(transitions)

  const result = {
    lead_time_avg_days: leadTimeAvgDays,
    cycle_time_avg_days: cycleTimeAvgDays,
    abandonment_rate: abandonmentRate,
    state_distribution: dist,
    burndown,
  }

  res.writeHead(200, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify(result))
}

module.exports = { handleMetrics }
