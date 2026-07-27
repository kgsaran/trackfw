'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const config = require('../src/config')

let passed = 0, failed = 0
function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

test('trackfw log reads .trackfw-log from configured roadmap_dir', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-log-'))
  try {
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: custom/roadmaps\n', 'utf8')
    const logDir = path.join(tmp, 'custom', 'roadmaps')
    fs.mkdirSync(logDir, { recursive: true })
    fs.writeFileSync(path.join(logDir, '.trackfw-log'), '2026-07-27 10:00  RM.md  wip → done\n', 'utf8')

    const result = spawnSync(process.execPath, [path.join(__dirname, '..', 'bin', 'trackfw'), 'log', '--tail', '1'], {
      cwd: tmp,
      encoding: 'utf8',
    })
    assert.strictEqual(result.status, 0, result.stderr || result.stdout)
    assert(result.stdout.includes('RM.md'), `expected configured log output, got: ${result.stdout}`)
  } finally {
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
