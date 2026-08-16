'use strict'

const assert = require('assert')
const net = require('net')
const http = require('http')

const { isLoopbackHost, displayUrl, createServer } = require('../src/commands/serve')
const config = require('../src/config')

let passed = 0, failed = 0
const tests = []

function test(name, fn) {
  tests.push({ name, fn })
}

// ─── displayUrl ──────────────────────────────────────────────────────────────

test('displayUrl mantem localhost para "localhost"', () => {
  assert.strictEqual(displayUrl('localhost', 4080), 'http://localhost:4080')
})

test('displayUrl mantem localhost para loopback IPv4 127.0.0.1', () => {
  assert.strictEqual(displayUrl('127.0.0.1', 4080), 'http://localhost:4080')
})

test('displayUrl mantem localhost para outro IPv4 em 127.0.0.0/8', () => {
  assert.strictEqual(displayUrl('127.0.0.5', 4080), 'http://localhost:4080')
})

test('displayUrl usa colchetes para IPv6 loopback ::1 (nao "localhost")', () => {
  assert.strictEqual(displayUrl('::1', 4080), 'http://[::1]:4080')
})

test('displayUrl usa colchetes para outro IPv6', () => {
  assert.strictEqual(displayUrl('2001:db8::1', 4080), 'http://[2001:db8::1]:4080')
})

test('displayUrl usa colchetes para IPv4-mapped IPv6 (paridade com Go/Python)', () => {
  assert.strictEqual(displayUrl('::ffff:127.0.0.1', 4080), 'http://[::ffff:127.0.0.1]:4080')
})

test('displayUrl imprime IP de LAN como esta', () => {
  assert.strictEqual(displayUrl('192.168.3.137', 4080), 'http://192.168.3.137:4080')
})

test('displayUrl imprime wildcard IPv4 como esta', () => {
  assert.strictEqual(displayUrl('0.0.0.0', 4080), 'http://0.0.0.0:4080')
})

// ─── isLoopbackHost (regressao — nao alterado neste ML, so conferido) ────────

test('isLoopbackHost aceita ::1 como loopback', () => {
  assert.strictEqual(isLoopbackHost('::1'), true)
})

test('isLoopbackHost recusa LAN', () => {
  assert.strictEqual(isLoopbackHost('192.168.3.137'), false)
})

// ─── Bind real em IPv6 loopback — prova por escuta, nao por leitura ─────────

test('servidor escuta de fato em [::1] e responde 200', async () => {
  const cfg = config.load()
  const server = createServer(cfg, 0)

  await new Promise((resolve, reject) => {
    server.listen(0, '::1', resolve)
    server.on('error', reject)
  })

  try {
    const address = server.address()
    assert.strictEqual(address.family, 'IPv6')

    const statusCode = await new Promise((resolve, reject) => {
      http.get(
        { host: '::1', port: address.port, path: '/', family: 6 },
        (res) => {
          res.resume()
          resolve(res.statusCode)
        }
      ).on('error', reject)
    })
    assert.strictEqual(statusCode, 200)
  } finally {
    await new Promise((resolve) => server.close(resolve))
  }
})

test('servidor continua escutando em 127.0.0.1 por padrao (nao-regressao)', async () => {
  const cfg = config.load()
  const server = createServer(cfg, 0)

  await new Promise((resolve, reject) => {
    server.listen(0, '127.0.0.1', resolve)
    server.on('error', reject)
  })

  try {
    const address = server.address()
    assert.strictEqual(address.address, '127.0.0.1')

    const statusCode = await new Promise((resolve, reject) => {
      http.get({ host: 'localhost', port: address.port, path: '/' }, (res) => {
        res.resume()
        resolve(res.statusCode)
      }).on('error', reject)
    })
    assert.strictEqual(statusCode, 200)
  } finally {
    await new Promise((resolve) => server.close(resolve))
  }
})

// ─── Runner ──────────────────────────────────────────────────────────────────

;(async () => {
  for (const { name, fn } of tests) {
    try {
      await fn()
      console.log('v', name)
      passed++
    } catch (e) {
      console.error('x', name, e.message)
      failed++
    }
  }
  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
})()
