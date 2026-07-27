'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { moveREQ } = require('../src/generators/req')
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

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
