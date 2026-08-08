'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

// Same isolation pattern as npm/tests/adr.test.js -- config.load() caches a
// module-level singleton keyed off cwd, so it must be reset() around every
// test that switches cwd/HOME.
const config = require('../src/config')
let __origHome
let __origCwd
test.beforeEach(() => {
  __origHome = process.env.HOME
  __origCwd = process.cwd()
  process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-req-home-'))
  config.reset()
})
test.afterEach(() => {
  process.env.HOME = __origHome
  process.chdir(__origCwd)
  config.reset()
})

function freshReqCmd() {
  const resolved = require.resolve('../src/commands/req')
  delete require.cache[resolved]
  return require('../src/commands/req')
}

test('req new without TTY: no ADR scope prompt, no ADR drafts, behavior unchanged', async () => {
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-req-notty-'))
  process.chdir(projectDir)

  // Confirm precondition asserted by the ML: no TTY in the test runner's stdin.
  assert.ok(!process.stdin.isTTY, 'precondition: process.stdin.isTTY must be falsy in this test run')

  const reqCmd = freshReqCmd()
  await reqCmd.parseAsync(['node', 'req', 'new', 'authentication flow'])

  const reqDir = path.join(projectDir, 'docs', 'req')
  assert.ok(fs.existsSync(reqDir), 'REQ dir should have been created')
  const files = fs.readdirSync(reqDir).filter((f) => f.endsWith('.md'))
  assert.equal(files.length, 1, 'exactly one REQ file should exist')

  // No ADR drafts should have been generated anywhere (no adr_dirs default, no global).
  assert.ok(!fs.existsSync(path.join(projectDir, 'docs', 'adr')), 'no ADR drafts should be created without TTY')
  assert.ok(!fs.existsSync(path.join(process.env.HOME, '.trackfw', 'adr')), 'global ADR dir should not be touched without TTY')
})

test('generators.newADRDraft with explicit adrDir="local" style path writes there, not to config default', async () => {
  const { newADRDraft } = require('../src/generators/adr')
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-req-adr-local-'))
  process.chdir(projectDir)
  config.reset()

  const localAdrDir = config.load().adrDirs[0]
  const basename = await newADRDraft('authentication-strategy', localAdrDir)

  assert.match(basename, /^ADR-\d{4}-\d{2}-\d{2}-authentication-strategy\.md$/)
  assert.ok(fs.existsSync(path.join(localAdrDir, basename)), 'ADR draft should exist in the local (project) adr dir')
})

test('generators.newADRDraft with explicit global adrDir writes under $HOME/.trackfw/adr', async () => {
  const { newADRDraft } = require('../src/generators/adr')
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-req-adr-global-'))
  process.chdir(projectDir)

  const globalAdrDir = path.join(process.env.HOME, '.trackfw', 'adr')
  const basename = await newADRDraft('deploy-target', globalAdrDir)

  assert.match(basename, /^ADR-\d{4}-\d{2}-\d{2}-deploy-target\.md$/)
  assert.ok(fs.existsSync(path.join(globalAdrDir, basename)), 'ADR draft should exist in the global adr dir')
  assert.ok(!fs.existsSync(path.join(projectDir, 'docs', 'adr')), 'project-scoped docs/adr must not be touched for a global draft')
})

test('generators.newADRDraft without adrDir argument preserves prior default behavior (adrDirs[0])', async () => {
  const { newADRDraft } = require('../src/generators/adr')
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-req-adr-default-'))
  process.chdir(projectDir)
  config.reset()

  const basename = await newADRDraft('api-protocol')

  const expectedDir = config.load().adrDirs[0]
  assert.match(basename, /^ADR-\d{4}-\d{2}-\d{2}-api-protocol\.md$/)
  assert.ok(fs.existsSync(path.join(expectedDir, basename)), 'ADR draft should exist in adrDirs[0] when adrDir is omitted')
})
