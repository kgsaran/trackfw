'use strict'

// Verifies that IntegrationManager.resolve() uses POSIX semantics so
// forward-slash paths from the catalog (e.g. ".claude/agents/x.md") are
// accepted on all platforms, including Windows where path.normalize would
// convert "/" → "\" and make the safety check fail.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { IntegrationManager } = require('../src/integrations/manager')

function roots() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-resolve-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot)
  fs.mkdirSync(homeRoot)
  return { projectRoot, homeRoot }
}

const ACCEPT = [
  '.claude/agents/trackfw-architect.md',
  '.amazonq/cli-agents/trackfw-architect.json',
]

const REJECT = [
  '..',
  '../x',
  'a/../../x',
  '.',
  './x',
  '',
  'bad\x00name',
]

for (const dest of ACCEPT) {
  test(`resolve accepts valid POSIX path: ${JSON.stringify(dest)}`, () => {
    const dirs = roots()
    const manager = new IntegrationManager(dirs)
    // resolve() is called internally by install(); a valid path must not throw
    // "Unsafe destination".  It may throw for other reasons (e.g. permissions),
    // but we only care that it does NOT throw the path-validation error.
    assert.doesNotThrow(
      () => manager.resolve('project', dest),
      /Unsafe destination/,
      `resolve('project', ${JSON.stringify(dest)}) threw Unsafe destination`
    )
  })
}

for (const dest of REJECT) {
  test(`resolve rejects unsafe destination: ${JSON.stringify(dest)}`, () => {
    const dirs = roots()
    const manager = new IntegrationManager(dirs)
    assert.throws(
      () => manager.resolve('project', dest),
      Error,
      `resolve('project', ${JSON.stringify(dest)}) did not throw`
    )
  })
}
