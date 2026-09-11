'use strict'
// serve_browser_security.test.js — AC1 + AC3 + AC4 (Node.js)
//
// Reconciliação (regra dura do projeto):
//   Cada teste abaixo declara em uma frase qual conclusão deste ML ele afirma.

const assert = require('assert')
const { browserArgv, isValidHost } = require('../src/commands/serve')

let passed = 0, failed = 0
const tests = []

function test(name, fn) {
  tests.push({ name, fn })
}

// ---------------------------------------------------------------------------
// browserArgv — AC1: argv, nunca string de shell
// ---------------------------------------------------------------------------

// AFIRMAÇÃO: Darwin → cmd='open', args=[url] como elemento único de argv (sem
// shell). Uma URL com metacaracteres de shell chega como string literal ao
// processo 'open', sem interpretação pelo shell.
test('browserArgv darwin retorna [open, [url]] — url como elemento único de argv', () => {
  const url = 'http://localhost:4080'
  const [cmd, args] = browserArgv('darwin', url)
  assert.strictEqual(cmd, 'open')
  assert.deepStrictEqual(args, [url])
})

// AFIRMAÇÃO: Linux → cmd='xdg-open', args=[url] — mesma garantia de argv.
test('browserArgv linux retorna [xdg-open, [url]] — url como elemento único de argv', () => {
  const url = 'http://192.168.1.100:8080'
  const [cmd, args] = browserArgv('linux', url)
  assert.strictEqual(cmd, 'xdg-open')
  assert.deepStrictEqual(args, [url])
})

// AFIRMAÇÃO: Windows → cmd='cmd', args inclui '/c', 'start', '', url — sem
// shell=true no spawn, url é elemento distinto de argv (CreateProcess nível
// de argumento), não parte de uma string de shell que o Python ou Node montam.
test('browserArgv win32 retorna [cmd, [/c, start, , url]] — sem shell interpolation pelo runtime', () => {
  const url = 'http://my-host.example.com:4080'
  const [cmd, args] = browserArgv('win32', url)
  assert.strictEqual(cmd, 'cmd')
  assert.deepStrictEqual(args, ['/c', 'start', '', url])
})

// AFIRMAÇÃO: uma URL com metacaracteres de shell chega INTACTA como elemento
// de argv em browserArgv — nenhuma expansão shell acontece no nível do
// construtor de argv; a execução real do processo é o que garante a não-
// injeção (e o PATH shim do gate externo prova isso em tempo de execução).
test('browserArgv — URL com metacaracteres de shell chega como argv literal (sem expansão)', () => {
  const malicious = 'http://x" ; id > /tmp/INJETADO ; echo ":4080'
  const [, args] = browserArgv('darwin', malicious)
  // A URL chega intacta como elemento único — o shim de PATH vai recebê-la
  // como $1, não como expansão de shell.
  assert.strictEqual(args[0], malicious)
  assert.strictEqual(args.length, 1)
})

// ---------------------------------------------------------------------------
// isValidHost — AC4: validação na entrada
// ---------------------------------------------------------------------------

// AFIRMAÇÃO: isValidHost rejeita o payload de injeção da REQ — o host com
// metacaracteres de shell nunca chega ao browser-open path.
test('isValidHost rejeita host com metacaracteres de shell (payload da REQ)', () => {
  assert.strictEqual(isValidHost('x" ; id > /tmp/INJETADO ; echo "'), false)
})

// AFIRMAÇÃO: isValidHost aceita 'localhost' — caso padrão, nunca deve ser
// rejeitado (contra-braço de AC3: rejeitar tudo quebraria o serve).
test('isValidHost aceita localhost', () => {
  assert.strictEqual(isValidHost('localhost'), true)
})

// AFIRMAÇÃO: isValidHost aceita IPv4 válido.
test('isValidHost aceita IPv4 válido', () => {
  assert.strictEqual(isValidHost('127.0.0.1'), true)
  assert.strictEqual(isValidHost('192.168.1.100'), true)
  assert.strictEqual(isValidHost('0.0.0.0'), true)
})

// AFIRMAÇÃO: isValidHost aceita IPv6 válido.
test('isValidHost aceita IPv6 válido', () => {
  assert.strictEqual(isValidHost('::1'), true)
  assert.strictEqual(isValidHost('2001:db8::1'), true)
})

// AFIRMAÇÃO: isValidHost aceita hostname RFC-1123 com hífens e pontos.
test('isValidHost aceita hostname RFC-1123', () => {
  assert.strictEqual(isValidHost('my-host.example.com'), true)
  assert.strictEqual(isValidHost('host'), true)
})

// AFIRMAÇÃO: isValidHost rejeita strings com outros caracteres especiais de
// shell que não sejam espaço/aspas — garante que o filtro não é estreito ao
// payload exato da REQ.
test('isValidHost rejeita strings com semicolon, ampersand, pipe', () => {
  assert.strictEqual(isValidHost('host;cmd'), false)
  assert.strictEqual(isValidHost('host&cmd'), false)
  assert.strictEqual(isValidHost('host|cmd'), false)
  assert.strictEqual(isValidHost('host>file'), false)
  assert.strictEqual(isValidHost('host`cmd`'), false)
})

// ---------------------------------------------------------------------------
// IPv6 zone ID — parity with Go and Python (fix for hades-tf BLOQUEIA)
// ---------------------------------------------------------------------------

// AFIRMAÇÃO: isValidHost rejeita IPv6 scoped addresses (zone ID com '%') —
// incluindo zone IDs sintaticamente limpos como fe80::1%eth0. Node.js
// net.isIPv6() aceita zone IDs limpos mas rejeita os que têm metacaracteres;
// esse comportamento dependente de versão do runtime não é um contrato
// confiável. Rejeitando '%' antes de net.isIPv6() alinhamos Node.js com Go
// (que rejeita todos os scoped via net.ParseIP) e com Python corrigido, e
// fechamos a classe inteira de zone IDs em vez de enumerar metacaracteres.
test('isValidHost rejeita IPv6 scoped address (zone ID com %)', () => {
  // zone ID limpo — rejeitado para paridade com Go e Python
  assert.strictEqual(isValidHost('fe80::1%eth0'), false)
  // zone ID com metacaracteres de cmd.exe — o vetor do bloqueio do hades-tf
  assert.strictEqual(isValidHost('fe80::1%eth0&calc.exe&echo'), false)
  assert.strictEqual(isValidHost('fe80::1%eth0;id'), false)
  assert.strictEqual(isValidHost('fe80::1%0'), false)
  // percent em hostname RFC-1123
  assert.strictEqual(isValidHost('host%20name'), false)
})

// ---------------------------------------------------------------------------
// Runner
// ---------------------------------------------------------------------------

;(async () => {
  for (const { name, fn } of tests) {
    try {
      await fn()
      console.log(`OK   ${name}`)
      passed++
    } catch (err) {
      console.error(`FAIL ${name}: ${err.message}`)
      failed++
    }
  }
  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
})()
