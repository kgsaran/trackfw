'use strict'

const fs = require('fs')
const path = require('path')

/**
 * isPathAllowed verifica se absPath está dentro de um dos diretórios permitidos.
 * Usa separador no sufixo para evitar que /docs/adr case com /docs/adr2.
 * @param {string} absPath
 * @param {string[]} allowedDirs
 * @returns {boolean}
 */
function isPathAllowed(absPath, allowedDirs) {
  for (const dir of allowedDirs) {
    if (absPath === dir || absPath.startsWith(dir + path.sep)) {
      return true
    }
  }
  return false
}

/**
 * handleFile responde ao GET /api/file?path=... com o conteúdo do arquivo.
 *
 * Modelo de segurança — contenção em dois estágios:
 *
 * 1. Contenção léxica (sem acesso ao filesystem): path.resolve() normaliza ".."
 *    e o resultado deve começar com uma das raízes autorizadas.  Caminhos fora
 *    das raízes recebem 403 antes de qualquer acesso ao disco, impedindo que a
 *    distinção 403 vs 404 sirva de oracle de existência para caminhos arbitrários.
 *
 * 2. Contenção física (resolução de symlinks): fs.realpathSync.native() é chamado
 *    no arquivo pedido e em cada raiz autorizada.  Um symlink cujo nome está
 *    dentro de uma raiz autorizada mas cujo destino físico não está → 403.
 *    Qualquer falha de canonicalização (ENOENT, symlink pendente, …) → 404.
 *
 * Um symlink legítimo cujo destino também está dentro de uma raiz autorizada
 * continua sendo servido normalmente.
 *
 * @param {object} cfg
 * @param {http.IncomingMessage} req
 * @param {http.ServerResponse} res
 */
function handleFile(cfg, req, res) {
  let urlObj
  try {
    urlObj = new URL(req.url, 'http://localhost')
  } catch (_) {
    res.writeHead(400, { 'Content-Type': 'text/plain' })
    res.end('Bad Request')
    return
  }

  const filePath = urlObj.searchParams.get('path')
  if (!filePath) {
    res.writeHead(400, { 'Content-Type': 'text/plain' })
    res.end('Missing path parameter')
    return
  }

  // ── Estágio 1: Contenção léxica ────────────────────────────────────────────
  // path.resolve() resolve '..' mas NÃO segue symlinks (operação puramente léxica).
  const resolved = path.resolve(filePath)

  const adrDirs = (cfg.adrDirs || ['docs/adr']).map(d => path.resolve(d))
  const reqDir = path.resolve(cfg.reqDir || 'docs/req')
  const roadmapDir = path.resolve(cfg.roadmapDir || 'docs/roadmaps')
  const allowedDirs = [...adrDirs, reqDir, roadmapDir]

  if (!isPathAllowed(resolved, allowedDirs)) {
    res.writeHead(403, { 'Content-Type': 'text/plain' })
    res.end('Forbidden')
    return
  }

  // ── Estágio 2: Contenção física ────────────────────────────────────────────
  // Canonicalizar o arquivo pedido. Qualquer erro (ENOENT, symlink pendente,
  // loop, permissão) → 404. O arquivo está nominalmente dentro de uma raiz
  // autorizada (estágio 1 passou), portanto 404 é o resultado correto quando
  // ele simplesmente não existe.
  let realResolved
  try {
    realResolved = fs.realpathSync.native(resolved)
  } catch (_) {
    res.writeHead(404, { 'Content-Type': 'text/plain' })
    res.end('Not Found')
    return
  }

  // Canonicalizar as raízes autorizadas. Raízes inexistentes são ignoradas —
  // nenhum arquivo pode residir em um diretório que não existe.
  const realAllowedDirs = allowedDirs.flatMap(d => {
    try { return [fs.realpathSync.native(d)] } catch (_) { return [] }
  })

  // Contenção física: o destino canônico deve estar dentro de uma raiz canônica.
  if (!isPathAllowed(realResolved, realAllowedDirs)) {
    res.writeHead(403, { 'Content-Type': 'text/plain' })
    res.end('Forbidden')
    return
  }

  // O arquivo existe (realpathSync não falhou) — ler da rota canônica.
  let content
  try {
    content = fs.readFileSync(realResolved, 'utf8')
  } catch (err) {
    res.writeHead(500, { 'Content-Type': 'text/plain' })
    res.end('Internal Server Error')
    return
  }

  res.writeHead(200, { 'Content-Type': 'text/plain; charset=utf-8' })
  res.end(content)
}

module.exports = { handleFile }
