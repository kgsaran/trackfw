'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { moveREQ, rewriteREQStatus } = require('../src/generators/req')
const config = require('../src/config')

let passed = 0, failed = 0
function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

test('moveREQ rewrites frontmatter and header status without moving file', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-req-move-'))
  const orig = process.cwd()
  try {
    process.chdir(tmp)
    config.reset()
    const reqDir = path.join(tmp, 'docs', 'req')
    fs.mkdirSync(reqDir, { recursive: true })
    const reqPath = path.join(reqDir, 'REQ-2026-07-27-fechar.md')
    fs.writeFileSync(reqPath,
      '---\nstatus: Open\ndate: 2026-07-27\nroadmap: "docs/roadmaps/done/RM.md"\n---\n\n' +
      '# REQ: Fechar\n\n> Date: 2026-07-27 | Status: Open | Linear Issue: X\n\n' +
      '## Notes\nstatus: Open\n| Status: Open\n', 'utf8')

    moveREQ('fechar', 'done')
    const updated = fs.readFileSync(reqPath, 'utf8')
    assert(updated.includes('status: done\n'), 'frontmatter status should be updated')
    assert(updated.includes('> Date: 2026-07-27 | Status: done | Linear Issue: X'), 'header Status should be updated')
    assert(updated.includes('## Notes\nstatus: Open\n| Status: Open\n'), 'body status occurrences must be preserved')
    assert(fs.existsSync(reqPath), 'REQ must remain in the flat req dir')
  } finally {
    process.chdir(orig)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ML-5B: falsificação nas duas direções (ADR-2026-09-04-parser-de-frontmatter-tolera-crlf-na-
// fronteira-de-entrada, D1/D3) — o mesmo defeito do ML-5A, agora no 2º sítio deste CLI:
// rewriteREQStatus.
test('rewriteREQStatus — CRLF source renderiza byte-idêntico ao source LF (ADR CRLF)', () => {
  const lfSrc = '---\nstatus: Open\ndate: 2026-07-27\n---\n\n# REQ: X\n\n> Date: 2026-07-27 | Status: Open\n'
  const crlfSrc = lfSrc.replace(/\n/g, '\r\n')

  const lfResult = rewriteREQStatus(lfSrc, 'done')
  const crlfResult = rewriteREQStatus(crlfSrc, 'done')

  assert.strictEqual(crlfResult.changed, true, 'CRLF source deveria ser reconhecida como frontmatter (D3 site não deveria ficar cego)')
  assert.strictEqual(lfResult.content, crlfResult.content, `CRLF e LF deveriam produzir o mesmo resultado.\nLF:   ${JSON.stringify(lfResult.content)}\nCRLF: ${JSON.stringify(crlfResult.content)}`)
  assert.ok(!crlfResult.content.includes('\r'), `saída não deveria conter CR (D2); obteve:\n${JSON.stringify(crlfResult.content)}`)
})

test('moveREQ — controle POSIX: REQ em LF move através da mesma sequência de bytes de hoje', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-req-move-lf-'))
  const orig = process.cwd()
  try {
    process.chdir(tmp)
    config.reset()
    const reqDir = path.join(tmp, 'docs', 'req')
    fs.mkdirSync(reqDir, { recursive: true })
    const reqPath = path.join(reqDir, 'REQ-2026-07-27-controle.md')
    fs.writeFileSync(reqPath,
      '---\nstatus: Open\n---\n\n# REQ: Controle\n\n> Date: 2026-07-27 | Status: Open\n\ncorpo\n', 'utf8')

    moveREQ('controle', 'done')
    const updated = fs.readFileSync(reqPath, 'utf8')
    assert.strictEqual(updated, '---\nstatus: done\n---\n\n# REQ: Controle\n\n> Date: 2026-07-27 | Status: done\n\ncorpo\n')
  } finally {
    process.chdir(orig)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
