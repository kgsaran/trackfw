'use strict'

// version.test.js — Trava o contrato de saída de versão para as duas superfícies:
//   trackfw version     →  "trackfw <semver>"
//   trackfw --version   →  "trackfw <semver>"  (byte-idêntico ao subcomando)
//
// Contrato congelado em docs/cli-parity.md §"Version output".
// Regex canônica: ^trackfw [0-9]+\.[0-9]+\.[0-9]+$

const test = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const CLI = path.resolve(__dirname, '../bin/trackfw')
const VERSION_RE = /^trackfw [0-9]+\.[0-9]+\.[0-9]+$/

/** Roda `node <CLI> ...args` e devolve stdout sem trailing newline. */
function runCLI(...args) {
  const result = spawnSync(process.execPath, [CLI, ...args], { encoding: 'utf8' })
  // commander imprime a versão em stdout
  return (result.stdout || '').trimEnd()
}

test('trackfw version imprime no formato "trackfw <semver>"', () => {
  const out = runCLI('version')
  assert.match(out, VERSION_RE, `formato inesperado: "${out}"`)
})

test('trackfw --version imprime no formato "trackfw <semver>"', () => {
  const out = runCLI('--version')
  assert.match(out, VERSION_RE, `formato inesperado: "${out}"`)
})

test('trackfw version e --version são byte-idênticos', () => {
  const outVersion = runCLI('version')
  const outFlag = runCLI('--version')
  assert.strictEqual(
    outFlag,
    outVersion,
    `saídas divergem:\n  version  : "${outVersion}"\n  --version: "${outFlag}"`
  )
})
