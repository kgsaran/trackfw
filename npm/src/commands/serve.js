'use strict'

const http = require('http')
const path = require('path')
const fs = require('fs')
const { Command } = require('commander')
const config = require('../config')
const { handleBoard } = require('../serve/api_board')
const { handleChain } = require('../serve/api_chain')
const { handleMetrics } = require('../serve/api_metrics')
const { handleFile } = require('../serve/api_file')
const { handleAttention } = require('../serve/api_attention')

const STATIC_DIR = path.join(__dirname, '..', 'serve', 'static')

// Aviso pinado, byte-idêntico entre os 3 runtimes (Go, Node.js, Python) — ver
// docs/cli-parity.md "`trackfw serve` — endereço de escuta, `--host` e
// aviso de exposição". Emitido quando --host resolve para uma interface
// diferente de loopback.
function exposureWarning(host, port) {
  return `WARNING: trackfw serve is binding to ${host}:${port} — the governance chain (ADRs, REQs, roadmaps) will be readable without authentication by any device that can reach it.`
}

// Texto de --help pinado, byte-idêntico entre os 3 runtimes.
const SERVE_HOST_FLAG_HELP = 'Host to bind to (loopback only by default; use 0.0.0.0 to expose on the network)'

// Espelha internal/serve/serve.go IsLoopbackHost — 'localhost' ou IP loopback
// (IPv4 127.0.0.0/8 inteiro, ou IPv6 ::1), igual a net.IP.IsLoopback() do Go
// e ipaddress.ip_address(...).is_loopback do Python.
function isLoopbackHost(host) {
  const net = require('net')
  if (host === 'localhost') return true
  if (net.isIPv4(host)) {
    return host.split('.')[0] === '127'
  }
  if (net.isIPv6(host)) {
    return host === '::1' || host === '0:0:0:0:0:0:0:1'
  }
  return false
}

// Espelha internal/serve/serve.go DisplayURL — URL a imprimir e a abrir no
// browser. 'localhost' é mantido só para 'localhost' ou IPv4 loopback
// (127.0.0.0/8), para não mudar a saída do caso comum; hosts IPv6 usam
// colchetes; qualquer outro host é impresso como está.
function displayUrl(host, port) {
  const net = require('net')
  if (host === 'localhost') return `http://localhost:${port}`
  if (net.isIPv4(host)) {
    return host.split('.')[0] === '127' ? `http://localhost:${port}` : `http://${host}:${port}`
  }
  if (net.isIPv6(host)) {
    return `http://[${host}]:${port}`
  }
  return `http://${host}:${port}`
}

// Mapa de extensão → Content-Type
const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js':   'application/javascript; charset=utf-8',
  '.css':  'text/css; charset=utf-8',
  '.json': 'application/json',
  '.svg':  'image/svg+xml',
  '.ico':  'image/x-icon',
}

/**
 * serveStatic serve arquivos do STATIC_DIR.
 * Retorna 404 se o arquivo não existe ou se o path tenta escape (path traversal).
 * @param {string} urlPath - pathname da URL (ex: '/static/app.js')
 * @param {http.ServerResponse} res
 */
function serveStatic(urlPath, res) {
  // Remove o prefixo '/static'
  const relative = urlPath.replace(/^\/static/, '') || '/index.html'
  const resolved = path.resolve(path.join(STATIC_DIR, relative))

  // Segurança: path traversal
  if (!resolved.startsWith(path.resolve(STATIC_DIR) + path.sep) && resolved !== path.resolve(STATIC_DIR)) {
    res.writeHead(403, { 'Content-Type': 'text/plain' })
    res.end('Forbidden')
    return
  }

  if (!fs.existsSync(resolved)) {
    res.writeHead(404, { 'Content-Type': 'text/plain' })
    res.end('Not Found')
    return
  }

  const ext = path.extname(resolved).toLowerCase()
  const contentType = MIME[ext] || 'application/octet-stream'

  let content
  try {
    content = fs.readFileSync(resolved)
  } catch (_) {
    res.writeHead(500, { 'Content-Type': 'text/plain' })
    res.end('Internal Server Error')
    return
  }

  res.writeHead(200, { 'Content-Type': contentType })
  res.end(content)
}

/**
 * createServer cria o servidor HTTP do trackfw serve.
 * @param {object} cfg - configuração do trackfw (resultado de config.load())
 * @param {number} port
 * @returns {http.Server}
 */
function createServer(cfg, port) {
  return http.createServer((req, res) => {
    // CORS permissivo para desenvolvimento local
    res.setHeader('Access-Control-Allow-Origin', '*')

    let urlObj
    try {
      urlObj = new URL(req.url, `http://localhost:${port}`)
    } catch (_) {
      res.writeHead(400)
      res.end('Bad Request')
      return
    }

    const pathname = urlObj.pathname

    if (pathname === '/' || pathname === '/index.html') {
      const indexPath = path.join(STATIC_DIR, 'index.html')
      if (!fs.existsSync(indexPath)) {
        res.writeHead(404, { 'Content-Type': 'text/plain' })
        res.end('index.html not found')
        return
      }
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
      res.end(fs.readFileSync(indexPath))
      return
    }

    if (pathname.startsWith('/static/')) {
      serveStatic(pathname, res)
      return
    }

    if (pathname === '/api/board') {
      handleBoard(cfg, req, res)
      return
    }

    if (pathname === '/api/chain') {
      handleChain(cfg, req, res)
      return
    }

    if (pathname === '/api/metrics') {
      handleMetrics(cfg, req, res)
      return
    }

    if (pathname === '/api/file') {
      handleFile(cfg, req, res)
      return
    }

    if (pathname === '/api/attention') {
      handleAttention(cfg, req, res)
      return
    }

    res.writeHead(404, { 'Content-Type': 'text/plain' })
    res.end('Not found')
  })
}

/**
 * createServeCommand retorna o comando commander 'serve'.
 * @returns {Command}
 */
function createServeCommand() {
  const cmd = new Command('serve')
  cmd
    .description('Inicia o servidor HTTP do trackfw dashboard (kanban + chain + metrics)')
    .option('--port <port>', 'Porta do servidor', '8080')
    .option('--host <host>', SERVE_HOST_FLAG_HELP, '127.0.0.1')
    .option('--no-open', 'Não abrir o browser automaticamente')
    .action((opts) => {
      const cfg = config.load()
      const port = parseInt(opts.port, 10) || 8080
      const host = opts.host

      const server = createServer(cfg, port)

      if (!isLoopbackHost(host)) {
        console.error(exposureWarning(host, port))
      }

      server.listen(port, host, () => {
        const url = displayUrl(host, port)
        console.log(`trackfw serve: ${url}`)

        if (opts.open !== false) {
          // Tentar abrir o browser
          const { exec } = require('child_process')
          const platform = process.platform
          let openCmd
          if (platform === 'darwin') openCmd = `open "${url}"`
          else if (platform === 'win32') openCmd = `start "" "${url}"`
          else openCmd = `xdg-open "${url}"`
          exec(openCmd, (err) => {
            if (err) console.warn(`Não foi possível abrir o browser: ${err.message}`)
          })
        }
      })

      server.on('error', (err) => {
        if (err.code === 'EADDRINUSE') {
          console.error(`Porta ${port} já está em uso. Use --port para especificar outra.`)
        } else {
          console.error(`Erro no servidor: ${err.message}`)
        }
        process.exit(1)
      })

      // Graceful shutdown
      process.on('SIGINT', () => {
        server.close(() => {
          console.log('\ntrackfw serve: encerrado.')
          process.exit(0)
        })
      })
    })

  return cmd
}

module.exports = { createServeCommand, createServer, isLoopbackHost, displayUrl }
