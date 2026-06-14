'use strict'

const fs = require('fs')
const path = require('path')

const STATES = ['wip', 'backlog', 'blocked', 'done', 'abandoned']

/**
 * extractTitle retorna o título do arquivo: primeira linha '# ...' ou nome do arquivo sem extensão.
 * @param {string} content
 * @param {string} filename
 * @returns {string}
 */
function extractTitle(content, filename) {
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith('# ')) {
      return trimmed.slice(2).trim()
    }
  }
  return filename.replace(/\.md$/, '')
}

/**
 * scanState lê todos os .md de um diretório e retorna os itens do kanban.
 * @param {string} dir
 * @param {string} state
 * @param {string} agent - agente ou '' para flat
 * @param {string} roadmapDir - diretório base (para montar path relativo)
 * @returns {Array<{file: string, title: string, state: string, agent: string, path: string}>}
 */
function scanState(dir, state, agent, roadmapDir) {
  const items = []
  let files = []
  try {
    files = fs.readdirSync(dir).filter(f => {
      if (!f.endsWith('.md')) return false
      try { return !fs.statSync(path.join(dir, f)).isDirectory() } catch (_) { return false }
    })
  } catch (_) {
    return items
  }

  for (const file of files) {
    let content = ''
    try { content = fs.readFileSync(path.join(dir, file), 'utf8') } catch (_) {}
    const title = extractTitle(content, file)
    const relPath = agent
      ? path.join(roadmapDir, agent, state, file)
      : path.join(roadmapDir, state, file)

    items.push({ file, title, state, agent: agent || '', path: relPath })
  }
  return items
}

/**
 * handleBoard responde ao GET /api/board com o kanban em JSON.
 * @param {object} cfg - configuração do trackfw
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 */
function handleBoard(cfg, req, res) {
  const roadmapDir = cfg.roadmapDir || 'docs/roadmaps'
  const namespacing = cfg.roadmapNamespacing || 'flat'

  const columns = { wip: [], backlog: [], blocked: [], done: [], abandoned: [] }
  const agentSet = new Set()

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
      agentSet.add(agent)
      for (const state of STATES) {
        const dir = path.join(roadmapDir, agent, state)
        const items = scanState(dir, state, agent, roadmapDir)
        columns[state].push(...items)
      }
    }
  } else {
    for (const state of STATES) {
      const dir = path.join(roadmapDir, state)
      const items = scanState(dir, state, '', roadmapDir)
      columns[state].push(...items)
    }
  }

  const result = {
    columns,
    agents: Array.from(agentSet).sort(),
  }

  res.writeHead(200, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify(result))
}

module.exports = { handleBoard }
