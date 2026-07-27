'use strict'

/**
 * Testes de paridade de fuso horário para os geradores de artefato Node.js.
 *
 * Invariante: localDateISO() / today() devem retornar a DATA LOCAL, não UTC.
 * Estratégia determinística (sem mock, sem nova dependência):
 *
 * Pacific/Kiritimati = UTC+14, sem DST → sempre adiantado 14h em relação a UTC.
 * Pacific/Midway     = UTC-11, sem DST → sempre atrasado 11h em relação a UTC.
 * Span total = 25 horas → as duas datas locais NUNCA coincidem.
 *
 * Com implementação correta (hora local):  kiri ≠ midway  →  assert.notEqual → PASS
 * Com implementação quebrada (UTC):        kiri == midway  →  assert.notEqual → FAIL
 *
 * Além da inequação, cada resultado é verificado contra o cálculo explícito
 * de offset UTC (sem depender de `TZ` no processo de teste).
 *
 * REQ: REQ-2026-07-27-convergencia-templates-python
 */

const test = require('node:test')
const assert = require('node:assert/strict')
const { execSync } = require('node:child_process')
const path = require('node:path')

const NPM_ROOT = path.join(__dirname, '..')

/**
 * Calcula a data local para um offset UTC fixo (sem DST), determinístico.
 * @param {number} offsetHours e.g. 14 para UTC+14, -11 para UTC-11
 * @returns {string} "YYYY-MM-DD"
 */
function expectedLocalDate(offsetHours) {
  const utcMs = Date.now()
  const localMs = utcMs + offsetHours * 3600 * 1000
  return new Date(localMs).toISOString().slice(0, 10)
}

/**
 * Executa um snippet Node.js com TZ específico e retorna o stdout trimado.
 */
function runWithTZ(tzName, snippet) {
  return execSync(`node -e "${snippet}"`, {
    env: { ...process.env, TZ: tzName },
    cwd: NPM_ROOT,
  }).toString().trim()
}

// ─── adr.js ───────────────────────────────────────────────────────────────────

test('adr.js today() — UTC+14 e UTC-11 produzem datas diferentes (usa hora local, não UTC)', () => {
  const snippet = "const {today}=require('./src/generators/adr');process.stdout.write(today())"
  const kiritimati = runWithTZ('Pacific/Kiritimati', snippet)
  const midway     = runWithTZ('Pacific/Midway', snippet)

  assert.notEqual(kiritimati, midway,
    `today() retornou a MESMA data em UTC+14 e UTC-11 (${kiritimati}) — uso de toISOString (UTC) detectado`)

  assert.equal(kiritimati, expectedLocalDate(14),
    `adr.js UTC+14: esperado ${expectedLocalDate(14)}, obteve ${kiritimati}`)
  assert.equal(midway, expectedLocalDate(-11),
    `adr.js UTC-11: esperado ${expectedLocalDate(-11)}, obteve ${midway}`)
})

// ─── note.js ──────────────────────────────────────────────────────────────────

test('note.js today() — UTC+14 e UTC-11 produzem datas diferentes (usa hora local, não UTC)', () => {
  const snippet = "const {today}=require('./src/generators/note');process.stdout.write(today())"
  const kiritimati = runWithTZ('Pacific/Kiritimati', snippet)
  const midway     = runWithTZ('Pacific/Midway', snippet)

  assert.notEqual(kiritimati, midway,
    `today() retornou a MESMA data em UTC+14 e UTC-11 (${kiritimati}) — uso de toISOString (UTC) detectado`)

  assert.equal(kiritimati, expectedLocalDate(14),
    `note.js UTC+14: esperado ${expectedLocalDate(14)}, obteve ${kiritimati}`)
  assert.equal(midway, expectedLocalDate(-11),
    `note.js UTC-11: esperado ${expectedLocalDate(-11)}, obteve ${midway}`)
})

// ─── req.js (via localDateISO exportado) ─────────────────────────────────────

test('req.js localDateISO() — UTC+14 e UTC-11 produzem datas diferentes (usa hora local, não UTC)', () => {
  const snippet = "const {localDateISO}=require('./src/generators/req');process.stdout.write(localDateISO())"
  const kiritimati = runWithTZ('Pacific/Kiritimati', snippet)
  const midway     = runWithTZ('Pacific/Midway', snippet)

  assert.notEqual(kiritimati, midway,
    `localDateISO() retornou a MESMA data em UTC+14 e UTC-11 (${kiritimati}) — uso de toISOString (UTC) detectado`)

  assert.equal(kiritimati, expectedLocalDate(14),
    `req.js UTC+14: esperado ${expectedLocalDate(14)}, obteve ${kiritimati}`)
  assert.equal(midway, expectedLocalDate(-11),
    `req.js UTC-11: esperado ${expectedLocalDate(-11)}, obteve ${midway}`)
})

// ─── date.js (helper base) ────────────────────────────────────────────────────

test('date.js localDateISO() — UTC+14 e UTC-11 produzem datas diferentes (usa hora local, não UTC)', () => {
  const snippet = "const {localDateISO}=require('./src/generators/date');process.stdout.write(localDateISO())"
  const kiritimati = runWithTZ('Pacific/Kiritimati', snippet)
  const midway     = runWithTZ('Pacific/Midway', snippet)

  assert.notEqual(kiritimati, midway,
    `localDateISO() base: mesma data em UTC+14 e UTC-11 (${kiritimati}) — regressão no helper`)

  assert.equal(kiritimati, expectedLocalDate(14),
    `date.js UTC+14: esperado ${expectedLocalDate(14)}, obteve ${kiritimati}`)
  assert.equal(midway, expectedLocalDate(-11),
    `date.js UTC-11: esperado ${expectedLocalDate(-11)}, obteve ${midway}`)
})
