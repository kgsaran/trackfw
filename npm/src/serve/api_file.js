'use strict'

const fs = require('fs')
const path = require('path')

/**
 * isPathAllowed verifica se o filePath resolvido está dentro de um dos diretórios permitidos.
 * @param {string} resolved - path.resolve(filePath)
 * @param {string[]} allowedDirs - lista de diretórios permitidos (já resolvidos)
 * @returns {boolean}
 */
function isPathAllowed(resolved, allowedDirs) {
  for (const dir of allowedDirs) {
    // garantir que o path começa com dir + separador
    if (resolved === dir || resolved.startsWith(dir + path.sep)) {
      return true
    }
  }
  return false
}

/**
 * handleFile responde ao GET /api/file?path=... com o conteúdo do arquivo.
 * Retorna 400 se path ausente, 403 se fora dos diretórios permitidos, 404 se não existe.
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

  // Resolver o path absoluto para verificar segurança
  const resolved = path.resolve(filePath)

  // Montar lista de diretórios permitidos a partir da config
  const adrDirs = (cfg.adrDirs || ['docs/adr']).map(d => path.resolve(d))
  const reqDir = path.resolve(cfg.reqDir || 'docs/req')
  const roadmapDir = path.resolve(cfg.roadmapDir || 'docs/roadmaps')
  const allowedDirs = [...adrDirs, reqDir, roadmapDir]

  if (!isPathAllowed(resolved, allowedDirs)) {
    res.writeHead(403, { 'Content-Type': 'text/plain' })
    res.end('Forbidden')
    return
  }

  if (!fs.existsSync(resolved)) {
    res.writeHead(404, { 'Content-Type': 'text/plain' })
    res.end('Not Found')
    return
  }

  let content
  try {
    content = fs.readFileSync(resolved, 'utf8')
  } catch (err) {
    res.writeHead(500, { 'Content-Type': 'text/plain' })
    res.end('Internal Server Error')
    return
  }

  res.writeHead(200, { 'Content-Type': 'text/plain; charset=utf-8' })
  res.end(content)
}

module.exports = { handleFile }
