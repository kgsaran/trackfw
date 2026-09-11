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

// isValidHost reports whether host is acceptable as a --host argument.
// Accepts: 'localhost', valid IPv4, valid IPv6 literal, RFC-1123 hostname.
// Rejects anything else — in particular strings with shell metacharacters.
// Espelha internal/serve/serve.go IsValidHost e _is_valid_host do Python.
function isValidHost(host) {
  if (host === 'localhost') return true
  const net = require('net')
  if (net.isIPv4(host) || net.isIPv6(host)) return true
  // RFC 1123 hostname: labels separated by dots, each [a-zA-Z0-9] or hyphens,
  // starting and ending with alphanum. Max label length 63, total max 253.
  if (host.length > 253) return false
  return /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/.test(host)
}

// browserArgv returns [cmd, args] for opening url in the default browser
// without shell interpolation — argv, never a shell string. This is the only
// safe form: exec(`open "${url}"`) interpolates url into a shell command and
// allows injection when url contains shell metacharacters.
//
// Darwin / Linux: 'open' / 'xdg-open' are invoked directly; no shell involved.
// Windows: spawn('cmd', ['/c', 'start', '', url]) passes url as a distinct
// argv element to CreateProcess — safer than exec(), but cmd.exe still
// re-parses its own metacharacters (& | ^ >) in the url portion. Defense for
// that residual is AC4 (isValidHost) rejecting metacharacter-bearing hosts
// before they reach this function.
//
// Exported for testing.
function browserArgv(platform, url) {
  if (platform === 'darwin') return ['open', [url]]
  if (platform === 'win32') return ['cmd', ['/c', 'start', '', url]]
  return ['xdg-open', [url]]
}

// openBrowser opens url in the default browser using spawn (argv, no shell).
// Exported for testing with a PATH shim.
//
// Preserves the original UX: warn when the opener exits non-zero (e.g. when
// xdg-open finds no handler) as well as when it cannot be launched (ENOENT).
// The old exec() warned via the callback err; spawn uses 'close' + 'error'.
function openBrowser(platform, url) {
  const { spawn } = require('child_process')
  const [cmd, args] = browserArgv(platform, url)
  const proc = spawn(cmd, args)
  proc.on('error', (err) => {
    console.warn(`Não foi possível abrir o browser: ${err.message}`)
  })
  proc.on('close', (code) => {
    if (code !== 0 && code !== null) {
      console.warn(`Não foi possível abrir o browser: processo encerrou com código ${code}`)
    }
  })
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

      // AC4 — validate --host before bind. Rejects metacharacter-bearing
      // strings that would reach the browser-open path or the socket.
      if (!isValidHost(host)) {
        console.error(`trackfw serve: invalid --host value: ${host}`)
        console.error('--host must be "localhost", a valid IPv4/IPv6 address, or an RFC-1123 hostname.')
        process.exit(1)
      }

      const server = createServer(cfg, port)

      if (!isLoopbackHost(host)) {
        console.error(exposureWarning(host, port))
      }

      server.listen(port, host, () => {
        const url = displayUrl(host, port)
        console.log(`trackfw serve: ${url}`)

        if (opts.open !== false) {
          // AC1 — use argv (spawn), never shell string interpolation (exec).
          openBrowser(process.platform, url)
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

module.exports = { createServeCommand, createServer, isLoopbackHost, displayUrl, isValidHost, browserArgv, openBrowser }
