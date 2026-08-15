'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const changelog = require('../src/changelog')

const fixtureChangelog = `# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

## [Unreleased]

### Added
- feature em desenvolvimento

## [6.10.0] - 2026-08-14

### Added
- comando trackfw changelog

### Fixed
- bug no parser

## [6.9.1] - 2026-08-01

### Fixed
- correção pontual
`

// --- parseSections -----------------------------------------------------

test('parseSections separa 3 seções (Unreleased + 2 versões)', () => {
  const sections = changelog.parseSections(fixtureChangelog)
  assert.equal(sections.length, 3)

  const unreleased = sections[0]
  assert.equal(unreleased.version, 'Unreleased')
  assert.equal(unreleased.date, '')
  assert.ok(unreleased.body.includes('feature em desenvolvimento'))
  assert.ok(!unreleased.body.includes('## ['))

  const v6100 = sections[1]
  assert.equal(v6100.version, '6.10.0')
  assert.equal(v6100.date, '2026-08-14')
  assert.ok(v6100.body.includes('comando trackfw changelog'))
  assert.ok(v6100.body.includes('bug no parser'))

  const v691 = sections[2]
  assert.equal(v691.version, '6.9.1')
  assert.equal(v691.date, '2026-08-01')
  assert.ok(v691.body.includes('correção pontual'))
})

// --- firstSection --------------------------------------------------------

test('firstSection lança erro para lista vazia', () => {
  assert.throws(() => changelog.firstSection([]), /CHANGELOG\.md has no version sections/)
})

test('firstSection retorna a primeira seção', () => {
  const sections = changelog.parseSections(fixtureChangelog)
  const first = changelog.firstSection(sections)
  assert.equal(first.version, 'Unreleased')
})

// --- findVersion -----------------------------------------------------------

test('findVersion encontra versão existente', () => {
  const sections = changelog.parseSections(fixtureChangelog)
  const got = changelog.findVersion(sections, '6.10.0')
  assert.equal(got.version, '6.10.0')
})

test('findVersion lança erro para versão ausente', () => {
  const sections = changelog.parseSections(fixtureChangelog)
  assert.throws(
    () => changelog.findVersion(sections, '999.0.0'),
    /version "999\.0\.0" not found in CHANGELOG\.md/
  )
})

test('findVersion normaliza prefixo "v"', () => {
  const sections = changelog.parseSections(fixtureChangelog)
  const withV = changelog.findVersion(sections, 'v6.10.0')
  const withoutV = changelog.findVersion(sections, '6.10.0')
  assert.equal(withV.version, withoutV.version)
  assert.equal(withV.body, withoutV.body)
})

// --- formatSection -----------------------------------------------------

test('formatSection com data', () => {
  const s = { version: '6.10.0', date: '2026-08-14', body: '### Added\n- x\n' }
  const got = changelog.formatSection(s)
  assert.equal(got, '## [6.10.0] - 2026-08-14\n\n### Added\n- x\n')
})

test('formatSection sem data', () => {
  const s = { version: 'Unreleased', date: '', body: '### Added\n- x\n' }
  const got = changelog.formatSection(s)
  assert.equal(got, '## [Unreleased]\n\n### Added\n- x\n')
})

test('formatSection não duplica linha em branco quando body começa com \\n', () => {
  // Body como vem de parseSections para uma seção cujo cabeçalho é seguido
  // de linha em branco no CHANGELOG.md original (caso real deste repo): a
  // primeira linha do body já é vazia.
  const s = { version: 'Unreleased', date: '', body: '\n### Added\n- x\n' }
  const got = changelog.formatSection(s)
  assert.equal(got, '## [Unreleased]\n\n### Added\n- x\n')
})

// --- read ----------------------------------------------------------------

test('read lança erro quando CHANGELOG.md não existe', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-changelog-'))
  assert.throws(() => changelog.read(dir), /CHANGELOG\.md not found — nothing to show/)
})

test('read retorna conteúdo do arquivo existente', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-changelog-'))
  fs.writeFileSync(path.join(dir, 'CHANGELOG.md'), fixtureChangelog)
  const got = changelog.read(dir)
  assert.equal(got, fixtureChangelog)
})

// --- comando `trackfw changelog` ------------------------------------------

// commander's Command instance carries option state across repeated
// parseAsync() calls on the same object -- require a fresh module instance
// per test to avoid cross-test leakage (same pattern as npm/tests/adr.test.js).
function freshChangelogCmd() {
  const resolved = require.resolve('../src/commands/changelog')
  delete require.cache[resolved]
  return require('../src/commands/changelog')
}

async function captureStdoutAsync(fn) {
  const orig = process.stdout.write.bind(process.stdout)
  let out = ''
  process.stdout.write = (chunk) => {
    out += chunk
    return true
  }
  try {
    await fn()
  } finally {
    process.stdout.write = orig
  }
  return out
}

let __origCwd
test.beforeEach(() => {
  __origCwd = process.cwd()
})
test.afterEach(() => {
  process.chdir(__origCwd)
  process.exitCode = 0
})

function chdirToProjectWithChangelog(content) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-changelog-cmd-'))
  if (content !== '') {
    fs.writeFileSync(path.join(dir, 'CHANGELOG.md'), content)
  }
  process.chdir(dir)
  return dir
}

test('trackfw changelog sem flags imprime a primeira seção', async () => {
  chdirToProjectWithChangelog(fixtureChangelog)
  const cmd = freshChangelogCmd()
  const out = await captureStdoutAsync(() => cmd.parseAsync(['node', 'changelog']))
  assert.equal(out, '## [Unreleased]\n\n### Added\n- feature em desenvolvimento\n')
})

test('trackfw changelog --version 6.10.0 imprime a seção da versão', async () => {
  chdirToProjectWithChangelog(fixtureChangelog)
  const cmd = freshChangelogCmd()
  const out = await captureStdoutAsync(() => cmd.parseAsync(['node', 'changelog', '--version', '6.10.0']))
  assert.equal(
    out,
    '## [6.10.0] - 2026-08-14\n\n### Added\n- comando trackfw changelog\n\n### Fixed\n- bug no parser\n'
  )
})

async function captureStderrAsync(fn) {
  const origError = console.error
  let out = ''
  console.error = (msg) => {
    out += msg + '\n'
  }
  try {
    await fn()
  } finally {
    console.error = origError
  }
  return out
}

test('trackfw changelog --version inexistente reporta erro e exitCode 1', async () => {
  chdirToProjectWithChangelog(fixtureChangelog)
  const cmd = freshChangelogCmd()
  const errOut = await captureStderrAsync(() => cmd.parseAsync(['node', 'changelog', '--version', '999.0.0']))
  assert.match(errOut, /Error: version "999\.0\.0" not found in CHANGELOG\.md/)
  assert.equal(process.exitCode, 1)
})

test('trackfw changelog --all imprime o arquivo inteiro', async () => {
  chdirToProjectWithChangelog(fixtureChangelog)
  const cmd = freshChangelogCmd()
  const out = await captureStdoutAsync(() => cmd.parseAsync(['node', 'changelog', '--all']))
  assert.equal(out, fixtureChangelog)
})

test('trackfw changelog sem CHANGELOG.md reporta erro claro e exitCode 1', async () => {
  chdirToProjectWithChangelog('')
  const cmd = freshChangelogCmd()
  const errOut = await captureStderrAsync(() => cmd.parseAsync(['node', 'changelog']))
  assert.match(errOut, /Error: CHANGELOG\.md not found — nothing to show/)
  assert.equal(process.exitCode, 1)
})
