'use strict'

const fs = require('fs')
const path = require('path')
const { resolveAgentNamespaces, extractRefPath } = require('../validator')
const { normalizeRefSeparator } = require('../lib/pathfmt')

const ROADMAP_STATES = ['wip', 'backlog', 'blocked', 'done', 'abandoned']

/**
 * parseFrontmatter extrai todos os campos de um bloco frontmatter YAML (entre --- e ---).
 * Retorna objeto com chaves/valores string.
 * @param {string} content
 * @returns {Object<string, string>}
 */
function parseFrontmatter(content) {
  const fields = {}
  const lines = content.split('\n')
  let started = false
  let inFm = false
  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed === '---') {
      if (!started) { started = true; inFm = true; continue }
      break // fecha frontmatter
    }
    if (!inFm) break
    const colonIdx = trimmed.indexOf(':')
    if (colonIdx < 0) continue
    const key = trimmed.slice(0, colonIdx).trim()
    let val = trimmed.slice(colonIdx + 1).trim()
    val = val.replace(/^["']|["']$/g, '')
    if (key) fields[key] = val
  }
  return fields
}

/**
 * extractTitle retorna a primeira linha '# ...' ou o nome do arquivo sem extensão.
 * @param {string} content
 * @param {string} filename
 * @returns {string}
 */
function extractTitle(content, filename) {
  for (const line of content.split('\n')) {
    const t = line.trim()
    if (t.startsWith('# ')) return t.slice(2).trim()
  }
  return filename.replace(/\.md$/, '')
}

/**
 * collectNodes lê arquivos .md de um diretório e retorna nós do grafo.
 * @param {string} dir
 * @param {'adr'|'req'|'roadmap'} type
 * @param {string} state
 * @returns {Array<{id: string, type: string, title: string, state: string, fm: Object}>}
 */
function collectNodes(dir, type, state) {
  const nodes = []
  let files = []
  try {
    files = fs.readdirSync(dir).filter(f => {
      if (!f.endsWith('.md')) return false
      try { return !fs.statSync(path.join(dir, f)).isDirectory() } catch (_) { return false }
    })
  } catch (_) {
    return nodes
  }
  for (const file of files) {
    let content = ''
    try { content = fs.readFileSync(path.join(dir, file), 'utf8') } catch (_) {}
    const fm = parseFrontmatter(content)
    const title = extractTitle(content, file)
    // node ID do grafo — IDENTIFICADOR emitido em JSON (ADR-2026-09-04, D1 categoria 2),
    // e tambem o alvo de toda aresta: resolveRef() abaixo so devolve valores vindos de
    // fileIndex/titleIndex, isto e, sempre um node id — nunca o valor cru do frontmatter.
    // Normalizar o id aqui, portanto, normaliza `edge.to` por construcao.
    //
    // 🔴 O path.join da linha 71 (fs.readFileSync) e uma EXPRESSAO INDEPENDENTE sobre os
    // mesmos operandos: normalizar esta nao pode afeta-la. O caminho de travessia
    // permanece nativo (ADR D2). Espelha internal/serve/api_chain.go:111.
    const id = normalizeRefSeparator(path.join(dir, file))
    nodes.push({ id, type, title, state, fm, content })
  }
  return nodes
}

/**
 * handleChain responde ao GET /api/chain com nodes e edges em JSON.
 * @param {object} cfg
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 */
function handleChain(cfg, req, res) {
  const adrDirs = cfg.adrDirs || ['docs/adr']
  const reqDir = cfg.reqDir || 'docs/req'
  const roadmapDir = cfg.roadmapDir || 'docs/roadmaps'
  const namespacing = cfg.roadmapNamespacing || 'flat'

  const allNodes = []

  // ADRs — scan recursivo de subpastas (done/, wip/, etc.)
  for (const adrDir of adrDirs) {
    const subfolders = ['done', 'wip', 'backlog', 'blocked', 'abandoned', '']
    for (const sub of subfolders) {
      const dir = sub ? path.join(adrDir, sub) : adrDir
      const nodes = collectNodes(dir, 'adr', sub || 'unknown')
      allNodes.push(...nodes)
    }
  }

  // REQs
  if (namespacing === 'by_agent') {
    const agents = resolveAgentNamespaces(cfg, reqDir)
    for (const agent of agents) {
      for (const state of ROADMAP_STATES) {
        const dir = path.join(reqDir, agent, state)
        allNodes.push(...collectNodes(dir, 'req', state))
      }
    }
  } else {
    allNodes.push(...collectNodes(reqDir, 'req', 'unknown'))
  }

  // Roadmaps
  if (namespacing === 'by_agent') {
    const agents = resolveAgentNamespaces(cfg, roadmapDir)
    for (const agent of agents) {
      for (const state of ROADMAP_STATES) {
        const dir = path.join(roadmapDir, agent, state)
        allNodes.push(...collectNodes(dir, 'roadmap', state))
      }
    }
  } else {
    for (const state of ROADMAP_STATES) {
      const dir = path.join(roadmapDir, state)
      allNodes.push(...collectNodes(dir, 'roadmap', state))
    }
  }

  // Deduplicar por id (path)
  const nodeMap = new Map()
  for (const n of allNodes) {
    if (!nodeMap.has(n.id)) nodeMap.set(n.id, n)
  }

  // Construir índice por título e nome de arquivo para resolver referências
  const titleIndex = new Map()  // titulo normalizado -> id
  const fileIndex = new Map()   // basename -> id
  for (const [id, n] of nodeMap) {
    const norm = n.title.toLowerCase().trim()
    if (!titleIndex.has(norm)) titleIndex.set(norm, id)
    const base = path.basename(id).replace(/\.md$/, '').toLowerCase()
    if (!fileIndex.has(base)) fileIndex.set(base, id)
  }

  // Construir edges a partir dos campos de frontmatter
  const edges = []
  const edgeSet = new Set()

  function addEdge(from, to) {
    const key = `${from}→${to}`
    if (!edgeSet.has(key)) {
      edgeSet.add(key)
      edges.push({ from, to })
    }
  }

  function resolveRef(val) {
    if (!val) return null
    // Tenta pelo nome de arquivo — path.basename(val), não val cru: o formato canônico
    // gravado por `trackfw req new`/`roadmap new` é o CAMINHO COMPLETO (ADR-2026-08-01), não o
    // basename isolado. Sem o basename() aqui, um vínculo real como
    // "docs/roadmaps/wip/ROADMAP-x.md" nunca batia contra a chave "roadmap-x" do fileIndex —
    // achado do ML-3D: nenhuma aresta REQ/ADR/Roadmap gerada pelo CLI jamais resolvia por este
    // caminho, mascarado porque os testes existentes só cobrem valor já-basename.
    //
    // Por que este arquivo NÃO importa validator.resolveRoadmapRef (ao contrário de
    // internal/serve/api_chain.go, ML-3D): fileIndex é indexado por basename ACROSS todos os
    // estados (wip/backlog/blocked/done/abandoned) desde antes desta correção — um vínculo
    // `roadmap:` "wip/X.md" já resolve contra o node real em "done/X.md" sem fallback extra.
    // O mecanismo do ML-3B/3D (caminho literal gravado com pasta de estado velha) não afeta
    // este CLI porque ele nunca comparou por igualdade de caminho completo — só por basename/
    // título. Ver docs/cli-parity.md, seção "serve: /api/chain".
    const base = path.basename(val).replace(/\.md$/, '').toLowerCase().trim()
    if (fileIndex.has(base)) return fileIndex.get(base)
    // Tenta pelo título
    const norm = val.toLowerCase().trim()
    if (titleIndex.has(norm)) return titleIndex.get(norm)
    return null
  }

  // Campos de link padrão
  const reqFields = (cfg.linkFields && cfg.linkFields.req) || ['REQ:', 'req:']
  const adrFields = (cfg.linkFields && cfg.linkFields.adr) || ['ADR:', 'adr:']
  const roadmapFields = (cfg.linkFields && cfg.linkFields.roadmap) || ['Roadmap:', 'roadmap:']

  for (const [id, node] of nodeMap) {
    const fm = node.fm || {}

    // Checar campos de frontmatter para criar arestas
    for (const [fmKey, fmVal] of Object.entries(fm)) {
      const keyLower = fmKey.toLowerCase()
      if (keyLower === 'req' || reqFields.some(f => f.toLowerCase().replace(':', '') === keyLower)) {
        const target = resolveRef(fmVal)
        if (target && target !== id) addEdge(id, target)
      }
      if (keyLower === 'adr' || adrFields.some(f => f.toLowerCase().replace(':', '') === keyLower)) {
        const target = resolveRef(fmVal)
        if (target && target !== id) addEdge(id, target)
      }
      if (keyLower === 'roadmap' || roadmapFields.some(f => f.toLowerCase().replace(':', '') === keyLower)) {
        const target = resolveRef(fmVal)
        if (target && target !== id) addEdge(id, target)
      }
    }

    // ML-3D: `trackfw req new` (npm/src/generators/req.js) grava `adr: ""` e `roadmap: ""`
    // SEMPRE vazios no frontmatter — o valor real vive no corpo, em "## Linked ADR / ADR:
    // <path>" e "## Linked Roadmap / Roadmap: <path>". O laço de frontmatter acima nunca
    // encontra esse vínculo para uma REQ gerada pelo próprio CLI (não é caso de borda: é o
    // formato canônico inteiro). validator.extractRefPath varre o CONTEÚDO inteiro
    // linha-a-linha (mesma função usada por validateRefTargetsExist) — ponto único de
    // extração, não duplicado aqui.
    for (const field of ['req', 'adr', 'roadmap']) {
      const val = extractRefPath(node.content || '', field)
      if (!val) continue
      const target = resolveRef(val)
      if (target && target !== id) addEdge(id, target)
    }
  }

  // Montar nodes de saída sem o campo fm interno
  const outputNodes = Array.from(nodeMap.values()).map(n => ({
    id: n.id,
    type: n.type,
    title: n.title,
    state: n.state,
  }))

  res.writeHead(200, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify({ nodes: outputNodes, edges }))
}

module.exports = { handleChain }
