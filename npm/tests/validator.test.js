'use strict'
const assert = require('assert')
const fs = require('fs')
const path = require('path')
const os = require('os')
const config = require('../src/config')

// Reset config singleton antes de cada teste que muda cwd
const validator = require('../src/validator')

let passed = 0, failed = 0
function test(name, fn) {
  try { fn(); console.log('✓', name); passed++ }
  catch (e) { console.error('✗', name, e.message); failed++ }
}

// walkDirMd
test('walkDirMd finds .md in subdirectories', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-'))
  fs.mkdirSync(path.join(tmp, 'done'))
  fs.writeFileSync(path.join(tmp, 'done', 'ADR-001.md'), '---\nstatus: Accepted\n---\n# ADR\n')
  fs.mkdirSync(path.join(tmp, 'wip'))
  fs.writeFileSync(path.join(tmp, 'wip', 'ADR-002.md'), '---\nstatus: Draft\n---\n# ADR\n')
  const results = validator.walkDirMd(tmp)
  assert(results.includes('ADR-001.md'), 'should find ADR-001.md in done/')
  assert(results.includes('ADR-002.md'), 'should find ADR-002.md in wip/')
  fs.rmSync(tmp, { recursive: true })
})

test('walkDirMd returns empty for non-existent dir', () => {
  const results = validator.walkDirMd('/tmp/tw-nonexistent-xyz-123')
  assert(Array.isArray(results))
  assert.strictEqual(results.length, 0)
})

// extractRefPath
test('extractRefPath extracts .md path', () => {
  const content = 'REQ: docs/req/foo.md\n'
  const result = validator.extractRefPath(content, 'REQ')
  assert.strictEqual(result, 'docs/req/foo.md')
})

test('extractRefPath returns null for em-dash', () => {
  const content = 'REQ: —\n'
  const result = validator.extractRefPath(content, 'REQ')
  assert.strictEqual(result, null)
})

test('extractRefPath returns null for hyphen placeholder', () => {
  const content = 'ADR: -\n'
  const result = validator.extractRefPath(content, 'ADR')
  assert.strictEqual(result, null)
})

test('extractRefPath returns null for non-.md value', () => {
  const content = 'Roadmap: somevalue\n'
  const result = validator.extractRefPath(content, 'Roadmap')
  assert.strictEqual(result, null)
})

test('extractRefPath returns null for empty field', () => {
  const content = 'REQ: \n'
  const result = validator.extractRefPath(content, 'REQ')
  assert.strictEqual(result, null)
})

// validateFolderStatusCoherence
test('validateFolderStatusCoherence returns array', () => {
  const result = validator.validateFolderStatusCoherence()
  assert(Array.isArray(result))
})

test('validateFolderStatusCoherence detects mismatch', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-'))
  const wipDir = path.join(tmp, 'roadmaps', 'wip')
  fs.mkdirSync(wipDir, { recursive: true })
  // Arquivo em wip/ mas status: Done no frontmatter
  fs.writeFileSync(path.join(wipDir, 'ROADMAP-test.md'), '---\nstatus: Done\ndate: 2026-01-01\n---\n# Test\n')
  // trackfw.yaml apontando para tmp
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${path.join(tmp, 'roadmaps')}\n`)

  const origCwd = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateFolderStatusCoherence()
    assert(result.some(w => w.includes('ROADMAP-test.md') && w.includes('Done')), `Expected mismatch warning, got: ${JSON.stringify(result)}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// validateFilenameUniqueness
test('validateFilenameUniqueness no-op when no duplicates', () => {
  const result = validator.validateFilenameUniqueness()
  assert(Array.isArray(result))
})

test('validateFilenameUniqueness detects duplicate', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-'))
  const roadmapDir = path.join(tmp, 'roadmaps')
  for (const state of ['wip', 'backlog', 'done']) {
    fs.mkdirSync(path.join(roadmapDir, state), { recursive: true })
  }
  // Mesmo nome em wip e backlog
  const fname = 'ROADMAP-2026-06-13-duplicate.md'
  fs.writeFileSync(path.join(roadmapDir, 'wip', fname), '# wip\n')
  fs.writeFileSync(path.join(roadmapDir, 'backlog', fname), '# backlog\n')
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${roadmapDir}\n`)

  const origCwd = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateFilenameUniqueness()
    assert(result.some(v => v.includes(fname) && v.includes('wip') && v.includes('backlog')), `Expected uniqueness violation, got: ${JSON.stringify(result)}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// validateRefTargetsExist
test('validateRefTargetsExist returns array', () => {
  const result = validator.validateRefTargetsExist()
  assert(Array.isArray(result))
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
