'use strict'

// unknown-command.test.js — trava o comportamento de comandos desconhecidos após a
// remoção do subsistema de plugins (ADR-2026-08-15-remocao-do-subsistema-de-plugins-
// em-vez-de-gate-de-binario-de-terceiro.md).
//
// O trackfw não baixa, gerencia nem executa código de terceiro. Qualquer subcomando
// não reconhecido deve falhar com erro de comando desconhecido — nunca tentar
// executar um binário externo (ex.: trackfw-<nome>) — usando a mensagem CANÔNICA
// compartilhada pelos 3 CLIs (D3 do ADR, pinada em docs/cli-parity.md e coberta
// byte-a-byte por scripts/check-unknown-command-parity.sh):
//
//   Error: unknown command "x" for "trackfw"
//   Did you mean "validate"?
//   Run 'trackfw --help' for usage.

const test = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const fs = require('node:fs')
const os = require('node:os')
const { spawnSync } = require('node:child_process')

const CLI = path.resolve(__dirname, '../bin/trackfw')

function runCLI(args, options = {}) {
  return spawnSync(process.execPath, [CLI, ...args], { encoding: 'utf8', ...options })
}

test('trackfw sem argumento exibe help em stdout, exit 0 (não é erro — ML-1C)', () => {
  // trackfw sem argumento é uso legítimo (pedir ajuda), não um comando
  // desconhecido — decisão do arquiteto no ML-1C
  // (ROADMAP-2026-08-16-higiene-sete-debitos-...). Antes desta correção,
  // commander tratava a ausência de subcomando como "provavelmente faltou um
  // subcomando" e chamava this.help({error: true}): help em stderr, exit 1 —
  // divergente de Go e Python, que já eram exit 0/stdout.
  const result = runCLI([])
  assert.strictEqual(result.status, 0, `exit code deve ser 0, obteve ${result.status}`)
  assert.strictEqual(result.stderr, '', `stderr deve ser vazio, obteve: "${result.stderr}"`)
  assert.match(result.stdout, /Usage: trackfw/, `esperava help contendo "Usage: trackfw" em stdout, obteve: "${result.stdout}"`)
  assert.match(result.stdout, /Commands:/, `esperava seção "Commands:" no help, obteve: "${result.stdout}"`)
})

test('trackfw comando-inexistente (sem sugestão próxima) produz mensagem canônica, exit 1', () => {
  const result = runCLI(['comando-inexistente-xyz'])
  assert.strictEqual(result.status, 1, `exit code deve ser 1, obteve ${result.status}`)
  const stderr = (result.stderr || '').trim()
  assert.strictEqual(
    stderr,
    'Error: unknown command "comando-inexistente-xyz" for "trackfw"\n' +
      "Run 'trackfw --help' for usage.",
    `mensagem canônica não bate byte-a-byte, obteve stderr: "${result.stderr}"`
  )
})

test('trackfw vaildate (typo próximo de validate) inclui linha "Did you mean"', () => {
  const result = runCLI(['vaildate'])
  assert.strictEqual(result.status, 1, `exit code deve ser 1, obteve ${result.status}`)
  const stderr = (result.stderr || '').trim()
  assert.strictEqual(
    stderr,
    'Error: unknown command "vaildate" for "trackfw"\n' +
      'Did you mean "validate"?\n' +
      "Run 'trackfw --help' for usage.",
    `mensagem canônica com sugestão não bate byte-a-byte, obteve stderr: "${result.stderr}"`
  )
})

test('trackfw plugins não existe mais como comando — mesma mensagem canônica, sem caso especial', () => {
  const result = runCLI(['plugins'])
  assert.strictEqual(result.status, 1, `exit code deve ser 1, obteve ${result.status}`)
  assert.match(
    (result.stderr || '').trim(),
    /^Error: unknown command "plugins" for "trackfw"/,
    `esperava erro de comando desconhecido para "plugins", obteve stderr: "${result.stderr}"`
  )
})

test('trackfw vaildate NUNCA executa um binário externo trackfw-vaildate real do PATH', () => {
  // Falsificação (P4): coloca um executável REAL trackfw-vaildate no PATH que
  // imprime um marcador distintivo, e prova que ele nunca roda — é o vetor exato
  // que o fallback de execução de plugin removido (D3 do ADR) costumava abrir.
  const fakeBinDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-unknown-cmd-'))
  try {
    const fakeBinPath = path.join(fakeBinDir, 'trackfw-vaildate')
    fs.writeFileSync(fakeBinPath, '#!/bin/sh\necho EXECUTOU_PLUGIN_MALICIOSO\n')
    fs.chmodSync(fakeBinPath, 0o755)

    const result = runCLI(['vaildate'], { env: { ...process.env, PATH: `${fakeBinDir}:${process.env.PATH}` } })
    const output = `${result.stdout || ''}${result.stderr || ''}`
    assert.doesNotMatch(output, /EXECUTOU_PLUGIN_MALICIOSO/, `binário externo foi executado — saída: "${output}"`)
    assert.strictEqual(result.status, 1, `exit code deve ser 1, obteve ${result.status}`)
    assert.match((result.stderr || ''), /Did you mean "validate"\?/, `esperava sugestão mesmo com binário no PATH, obteve: "${result.stderr}"`)
  } finally {
    fs.rmSync(fakeBinDir, { recursive: true, force: true })
  }
})
