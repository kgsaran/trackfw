'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

// Isolate HOME for every test in this file -- --scope global writes under
// $HOME/.trackfw/adr, and this must never touch the developer's real $HOME.
// config.load() caches a module-level singleton keyed off the cwd at first
// call, so it must be reset() around every test that switches cwd (same
// pattern as npm/tests/config.test.js) -- otherwise a stale adrDirs from an
// earlier test/file leaks in.
const config = require('../src/config')
let __origHome
let __origCwd
test.beforeEach(() => {
  __origHome = process.env.HOME
  __origCwd = process.cwd()
  process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-adr-home-'))
  config.reset()
})
test.afterEach(() => {
  process.env.HOME = __origHome
  process.chdir(__origCwd)
  config.reset()
})

// commander's Command instance carries option state across repeated
// parseAsync() calls on the same object (default not consistently
// re-applied when a later invocation omits a flag set by an earlier one)
// -- require a fresh module instance per test to avoid cross-test leakage.
function freshAdrCmd() {
  const resolved = require.resolve('../src/commands/adr')
  delete require.cache[resolved]
  return require('../src/commands/adr')
}

test('adr new --scope global writes into $HOME/.trackfw/adr without requiring trackfw.yaml', async () => {
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-adr-proj-'))
  process.chdir(projectDir)
  assert.ok(!fs.existsSync(path.join(projectDir, 'trackfw.yaml')), 'precondition: no trackfw.yaml in cwd')

  const adrCmd = freshAdrCmd()
  await adrCmd.parseAsync(['node', 'adr', 'new', 'teste global', '--scope', 'global'])

  const globalAdrDir = path.join(process.env.HOME, '.trackfw', 'adr')
  assert.ok(fs.existsSync(globalAdrDir), 'global adr dir should have been created')
  const files = fs.readdirSync(globalAdrDir).filter((f) => f.endsWith('.md'))
  assert.equal(files.length, 1, 'exactly one ADR file should exist in the global dir')
  assert.match(files[0], /^ADR-\d{4}-\d{2}-\d{2}-teste-global\.md$/)

  const content = fs.readFileSync(path.join(globalAdrDir, files[0]), 'utf8')
  assert.ok(content.includes('# ADR: teste global'), 'ADR content should include the title')

  // Must not have created anything under docs/adr in the (project-less) cwd.
  assert.ok(!fs.existsSync(path.join(projectDir, 'docs', 'adr')), 'scope=global must not touch project-scoped docs/adr')
})

test('adr new --scope project (default) preserves existing behavior, writing under adrDirs[0]', async () => {
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-adr-proj-default-'))
  process.chdir(projectDir)

  const adrCmd = freshAdrCmd()
  await adrCmd.parseAsync(['node', 'adr', 'new', 'teste project'])

  const projectAdrDir = path.join(projectDir, 'docs', 'adr')
  assert.ok(fs.existsSync(projectAdrDir), 'default docs/adr should have been created')
  const files = fs.readdirSync(projectAdrDir).filter((f) => f.endsWith('.md'))
  assert.equal(files.length, 1)
  assert.match(files[0], /^ADR-\d{4}-\d{2}-\d{2}-teste-project\.md$/)

  // The global dir must remain untouched.
  const globalAdrDir = path.join(process.env.HOME, '.trackfw', 'adr')
  assert.ok(!fs.existsSync(globalAdrDir), 'scope=project must not touch the global adr dir')
})

test('adr list --scope global lists ADRs from $HOME/.trackfw/adr', async () => {
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-adr-list-global-'))
  process.chdir(projectDir)

  const adrCmd = freshAdrCmd()
  await adrCmd.parseAsync(['node', 'adr', 'new', 'lista global', '--scope', 'global'])

  const logs = []
  const origLog = console.log
  console.log = (...args) => logs.push(args.join(' '))
  try {
    await adrCmd.parseAsync(['node', 'adr', 'list', '--scope', 'global'])
  } finally {
    console.log = origLog
  }

  const output = logs.join('\n')
  assert.match(output, /ADR-\d{4}-\d{2}-\d{2}-lista-global\.md/)
})

test('adr new rejects an invalid --scope value', async () => {
  const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-adr-invalid-scope-'))
  process.chdir(projectDir)

  const adrCmd = freshAdrCmd()

  const origExit = process.exit
  const origError = console.error
  let exitCode = null
  const errors = []
  process.exit = (code) => { exitCode = code; throw new Error('__exit__') }
  console.error = (...args) => errors.push(args.join(' '))
  try {
    await assert.rejects(
      adrCmd.parseAsync(['node', 'adr', 'new', 'teste', '--scope', 'bogus']),
      /__exit__/
    )
  } finally {
    process.exit = origExit
    console.error = origError
  }
  assert.notEqual(exitCode, 0, 'invalid --scope should exit with a non-zero code')
  assert.ok(errors.some((e) => e.includes('bogus')), 'error message should mention the invalid scope value')
})

test('generators.newADR writes to the explicit adrDir argument (no internal config.load() dependency for scope)', async () => {
  const { newADR } = require('../src/generators/adr')
  const explicitDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-adr-explicit-'))

  await newADR({ title: 'explicit dir test' }, explicitDir)

  const files = fs.readdirSync(explicitDir).filter((f) => f.endsWith('.md'))
  assert.equal(files.length, 1)
  assert.match(files[0], /^ADR-\d{4}-\d{2}-\d{2}-explicit-dir-test\.md$/)
})
