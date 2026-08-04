'use strict'

// req_move_cli_errors.test.js — regressão para o bug documentado em
// docs/req/REQ-2026-08-04-req-move-no-cli-node-nao-trata-erros-stack-trace-nao-capturado-em-vez-de-mensagem-limpa.md
//
// `trackfw req move <name> <status>` deve reportar erros de domínio (ex.: REQ
// não encontrada) como `Error: <mensagem>` limpa em stderr, com exit code
// não-zero — nunca um stack trace de Promise rejeitada sem handler.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const CLI = path.resolve(__dirname, '../bin/trackfw')

function setupFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-req-move-cli-'))
  fs.mkdirSync(path.join(dir, 'docs/req'), { recursive: true })
  return dir
}

function runReqMoveCLI(cwd, ...args) {
  const result = spawnSync(process.execPath, [CLI, 'req', 'move', ...args], { encoding: 'utf8', cwd })
  return { stdout: result.stdout || '', stderr: result.stderr || '', status: result.status }
}

test('req move CLI: REQ inexistente imprime "Error: ..." limpo em stderr, sem stack trace, e sai com código != 0', () => {
  const dir = setupFixture()
  try {
    const { stdout, stderr, status } = runReqMoveCLI(dir, 'nome-que-nao-existe', 'done')

    assert.notEqual(status, 0, 'exit code deve ser diferente de zero')
    assert.match(stderr, /Error: REQ "nome-que-nao-existe" not found/, 'stderr deve conter mensagem limpa de erro')
    assert.doesNotMatch(stderr, /at moveREQ|at findREQ|node:internal|\.js:\d+:\d+/, 'stderr não deve conter stack trace')
    assert.doesNotMatch(stdout, /at moveREQ|at findREQ|node:internal/, 'stdout não deve conter stack trace')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('req move CLI: REQ sem frontmatter/header status imprime "Error: ..." limpo em stderr, sem stack trace', () => {
  const dir = setupFixture()
  try {
    const reqPath = path.join(dir, 'docs/req/REQ-2026-08-04-fixture.md')
    fs.writeFileSync(reqPath, '# REQ: Fixture\n\nSem frontmatter e sem header de status.\n', 'utf8')

    const { stderr, status } = runReqMoveCLI(dir, 'fixture', 'done')

    assert.notEqual(status, 0, 'exit code deve ser diferente de zero')
    assert.match(stderr, /Error: REQ ".*" has no frontmatter status\/header Status to update/, 'stderr deve conter mensagem limpa de erro')
    assert.doesNotMatch(stderr, /at moveREQ|at findREQ|node:internal|\.js:\d+:\d+/, 'stderr não deve conter stack trace')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})
