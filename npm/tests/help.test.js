'use strict'
const assert = require('assert')
const helpCmd = require('../src/commands/help')

const { listKeys, describeKey } = helpCmd

let passed = 0, failed = 0
const tests = []

function test(name, fn) {
  tests.push({ name, fn })
}

// listKeys
test('help sem argumento lista adr_dirs', () => {
  const output = listKeys()
  assert(typeof output === 'string', 'listKeys deve retornar string')
  assert(output.includes('adr_dirs'), 'output deve conter "adr_dirs"')
})

test('help sem argumento lista wip_limit', () => {
  const output = listKeys()
  assert(output.includes('wip_limit'), 'output deve conter "wip_limit"')
})

test('help sem argumento exibe header KEY', () => {
  const output = listKeys()
  assert(output.includes('KEY'), 'output deve conter header "KEY"')
  assert(output.includes('DEFAULT'), 'output deve conter header "DEFAULT"')
  assert(output.includes('DESCRIÇÃO'), 'output deve conter header "DESCRIÇÃO"')
})

test('help sem argumento lista todas as rules.*', () => {
  const output = listKeys()
  assert(output.includes('rules.wip_has_req'), 'deve listar rules.wip_has_req')
  assert(output.includes('rules.stale_wip'), 'deve listar rules.stale_wip')
  assert(output.includes('rules.filename_uniqueness'), 'deve listar rules.filename_uniqueness')
})

// describeKey
test('help com argumento wip_limit exibe Default e valor 1', () => {
  const output = describeKey('wip_limit')
  assert(output !== null, 'describeKey não deve retornar null para wip_limit')
  assert(output.includes('Default'), 'output deve conter "Default"')
  assert(output.includes('1'), 'output deve conter o valor default "1"')
})

test('help com argumento wip_limit exibe Type integer', () => {
  const output = describeKey('wip_limit')
  assert(output.includes('integer'), 'output deve conter "integer"')
})

test('help com argumento adr_dirs exibe informações completas', () => {
  const output = describeKey('adr_dirs')
  assert(output !== null, 'describeKey não deve retornar null para adr_dirs')
  assert(output.includes('adr_dirs'), 'output deve conter o nome da key')
  assert(output.includes('Type'), 'output deve conter Type')
  assert(output.includes('Example'), 'output deve conter Example')
  assert(output.includes('Impact'), 'output deve conter Impact')
})

test('help com argumento rules.stale_wip exibe severidade', () => {
  const output = describeKey('rules.stale_wip')
  assert(output !== null, 'describeKey não deve retornar null para rules.stale_wip')
  assert(output.includes('warning'), 'output deve conter o default "warning"')
})

// trace_id_field e rules.traceid_*
test('help lista trace_id_field', () => {
  const output = listKeys()
  assert(output.includes('trace_id_field'), 'listKeys deve conter "trace_id_field"')
})

test('help lista rules.traceid_orphan_roadmap', () => {
  const output = listKeys()
  assert(output.includes('rules.traceid_orphan_roadmap'), 'listKeys deve conter "rules.traceid_orphan_roadmap"')
})

test('help lista rules.traceid_orphan_req', () => {
  const output = listKeys()
  assert(output.includes('rules.traceid_orphan_req'), 'listKeys deve conter "rules.traceid_orphan_req"')
})

test('help lista rules.traceid_state_mismatch', () => {
  const output = listKeys()
  assert(output.includes('rules.traceid_state_mismatch'), 'listKeys deve conter "rules.traceid_state_mismatch"')
})

test('help lista rules.traceid_duplicate_req', () => {
  const output = listKeys()
  assert(output.includes('rules.traceid_duplicate_req'), 'listKeys deve conter "rules.traceid_duplicate_req"')
})

test('help lista rules.traceid_duplicate_roadmap', () => {
  const output = listKeys()
  assert(output.includes('rules.traceid_duplicate_roadmap'), 'listKeys deve conter "rules.traceid_duplicate_roadmap"')
})

test('describeKey trace_id_field retorna dados válidos', () => {
  const output = describeKey('trace_id_field')
  assert(output !== null, 'describeKey não deve retornar null para trace_id_field')
  assert(output.includes('trace_id_field'), 'output deve conter o nome da key')
  assert(output.includes('Type'), 'output deve conter Type')
  assert(output.includes('Default'), 'output deve conter Default')
  assert(output.includes('Example'), 'output deve conter Example')
  assert(output.includes('Impact'), 'output deve conter Impact')
})

test('describeKey rules.traceid_orphan_roadmap exibe severidade error', () => {
  const output = describeKey('rules.traceid_orphan_roadmap')
  assert(output !== null, 'describeKey não deve retornar null')
  assert(output.includes('error'), 'deve conter o default "error"')
})

test('describeKey rules.traceid_state_mismatch exibe severidade error', () => {
  const output = describeKey('rules.traceid_state_mismatch')
  assert(output !== null, 'describeKey não deve retornar null')
  assert(output.includes('error'), 'deve conter o default "error"')
})

// key inválida
test('help key inválida retorna null', () => {
  const output = describeKey('nao_existe')
  assert.strictEqual(output, null, 'describeKey deve retornar null para key inexistente')
})

test('help key inválida — chave vazia retorna null', () => {
  const output = describeKey('')
  assert.strictEqual(output, null, 'describeKey deve retornar null para string vazia')
})

test('help key inválida — chave com espaço retorna null', () => {
  const output = describeKey('wip limit')
  assert.strictEqual(output, null, 'describeKey deve retornar null para key com espaço')
})

;(async () => {
  for (const { name, fn } of tests) {
    try {
      await fn()
      console.log('✓', name)
      passed++
    } catch (e) {
      console.error('✗', name, e.message)
      failed++
    }
  }
  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
})()
