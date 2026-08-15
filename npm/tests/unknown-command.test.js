'use strict'

// unknown-command.test.js — trava o comportamento de comandos desconhecidos após a
// remoção do subsistema de plugins (ADR-2026-08-15-remocao-do-subsistema-de-plugins-
// em-vez-de-gate-de-binario-de-terceiro.md).
//
// O trackfw não baixa, gerencia nem executa código de terceiro. Qualquer subcomando
// não reconhecido deve falhar com erro de comando desconhecido — nunca tentar
// executar um binário externo (ex.: trackfw-<nome>).

const test = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const CLI = path.resolve(__dirname, '../bin/trackfw')

function runCLI(...args) {
  return spawnSync(process.execPath, [CLI, ...args], { encoding: 'utf8' })
}

test('trackfw sem argumento exibe help (comportamento preservado)', () => {
  const result = runCLI()
  const output = `${result.stdout || ''}${result.stderr || ''}`
  assert.match(output, /Usage: trackfw/, `esperava help contendo "Usage: trackfw", obteve: "${output}"`)
  assert.match(output, /Commands:/, `esperava seção "Commands:" no help, obteve: "${output}"`)
})

test('trackfw comando-inexistente produz erro de comando desconhecido e exit != 0', () => {
  const result = runCLI('comando-inexistente-xyz')
  assert.notStrictEqual(result.status, 0, 'exit code deve ser diferente de 0')
  assert.match(
    (result.stderr || '').trim(),
    /^error: unknown command 'comando-inexistente-xyz'/,
    `esperava mensagem "error: unknown command '...'", obteve stderr: "${result.stderr}"`
  )
})

test('trackfw comando-inexistente nunca tenta executar binário externo trackfw-<nome>', () => {
  // Regressão do subsistema de plugins removido: nenhum spawn de "trackfw-comando-inexistente-xyz"
  // deve ocorrer. Validamos indiretamente — stdout não deve conter qualquer rastro de execução
  // de plugin/binário externo, e o processo deve falhar rápido via commander.
  const result = runCLI('comando-inexistente-xyz')
  const output = `${result.stdout || ''}${result.stderr || ''}`
  assert.doesNotMatch(output, /plugin/i, `saída não deve mencionar plugin, obteve: "${output}"`)
})

test('trackfw plugins não existe mais como comando', () => {
  const result = runCLI('plugins')
  assert.notStrictEqual(result.status, 0, 'exit code deve ser diferente de 0')
  assert.match(
    (result.stderr || '').trim(),
    /^error: unknown command 'plugins'/,
    `esperava erro de comando desconhecido para "plugins", obteve stderr: "${result.stderr}"`
  )
})
