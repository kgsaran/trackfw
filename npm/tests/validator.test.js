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
async function testAsync(name, fn) {
  try { await fn(); console.log('✓', name); passed++ }
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

test('validateRefTargetsExist accepts generated basename references', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-ref-'))
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/req/REQ-001.md'), '# REQ\nRoadmap: ROADMAP-001.md\n')
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/ROADMAP-001.md'), '# Roadmap\nREQ: REQ-001.md\n')
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'req_dir: docs/req\nroadmap_dir: docs/roadmaps\n')

  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    assert.deepStrictEqual(validator.validateRefTargetsExist(), [])
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// ML-2B: field mapping + severity per rule

test('field mapping: req_id satisfies wip_has_req', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'link_fields:\n  req:\n    - req_id\n')
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-001.md'),
    '---\nstatus: WIP\nreq_id: docs/req/REQ-001.md\n---\n## Acceptance Criteria\n- [ ] done\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateWIPHasREQ()
    assert(!result.some(v => v.includes('no linked REQ')),
      'req_id marker should satisfy wip_has_req: ' + JSON.stringify(result))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('severity off: adr_orphan suppressed', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'rules:\n  adr_orphan: off\n')
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/adr/ADR-001.md'),
    '---\nstatus: Accepted\n---\n# ADR-001\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const violations = []
    const warnings = []
    validator.applyRule('adr_orphan', validator.validateADRsAreReferenced(), violations, warnings)
    assert(!violations.some(v => v.includes('not referenced')),
      'adr_orphan: off should suppress violations')
    assert(!warnings.some(w => w.includes('not referenced')),
      'adr_orphan: off should suppress warnings too')
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('severity warning: wip_has_req appears in warnings not violations', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'rules:\n  wip_has_req: warning\n')
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-001.md'),
    '---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const violations = []
    const warnings = []
    validator.applyRule('wip_has_req', validator.validateWIPHasREQ(), violations, warnings)
    assert(!violations.some(v => v.includes('no linked REQ')),
      'wip_has_req: warning should not be in violations')
    assert(warnings.some(w => w.includes('no linked REQ')),
      'wip_has_req: warning should appear in warnings')
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('acceptance_markers custom: custom marker satisfies check', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'acceptance_markers:\n  - "## Done When"\n  - "## Critérios"\n')
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-001.md'),
    '---\nstatus: WIP\nREQ: docs/req/REQ-001.md\n---\n## Done When\n- [ ] done\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateWIPHasAcceptanceCriteria()
    assert(!result.some(v => v.includes('no acceptance criteria')),
      'custom marker ## Done When should satisfy acceptance criteria check')
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// ML-1B — Validação de adr_dirs com ~/
test('adr_dirs com ~/ no validador resolve diretório no home do usuário', () => {
  const testSubdir = '.trackfw-test-adrs-' + Date.now()
  const fullHomeSubdir = path.join(os.homedir(), testSubdir)
  fs.mkdirSync(fullHomeSubdir, { recursive: true })
  fs.writeFileSync(path.join(fullHomeSubdir, 'ADR-GLOBAL-001.md'), '---\nstatus: Accepted\n---\n# Global ADR\n')

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-tilde-val-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `adr_dirs:\n  - "~/${testSubdir}"\n`)

  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const found = validator.findAdrFile('ADR-GLOBAL-001.md')
    assert.strictEqual(found, path.join(fullHomeSubdir, 'ADR-GLOBAL-001.md'))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
    fs.rmSync(fullHomeSubdir, { recursive: true, force: true })
  }
})

// ML-2B — Resiliência CI/CD para adr_dirs inexistentes e isenção de adr_orphan em ADRs externos
;(async () => {
  await testAsync('adr_dirs inexistente com strict_ci_paths false (default) gera warning', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-nonexistent-warning-'))
    const nonexistent = path.join(tmp, 'nonexistent-adrs-dir')
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `adr_dirs:\n  - "${nonexistent}"\nstrict_ci_paths: false\n`)

    const origDir = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const res = validator.validateADRDirsExist()
      assert.strictEqual(res.violations.length, 0)
      assert(res.warnings.some(w => w.includes('does not exist') && w.includes('nonexistent-adrs-dir')))
      
      const unfilt = await validator.validateUnfiltered()
      assert(unfilt.warnings.some(w => w.includes('does not exist')))
      assert(!unfilt.violations.some(v => v.includes('does not exist')))
    } finally {
      process.chdir(origDir)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  await testAsync('adr_dirs inexistente com strict_ci_paths true gera violation', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-nonexistent-violation-'))
    const nonexistent = path.join(tmp, 'nonexistent-adrs-dir')
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `adr_dirs:\n  - "${nonexistent}"\nstrict_ci_paths: true\n`)

    const origDir = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const res = validator.validateADRDirsExist()
      assert.strictEqual(res.warnings.length, 0)
      assert(res.violations.some(v => v.includes('does not exist') && v.includes('nonexistent-adrs-dir')))

      const unfilt = await validator.validateUnfiltered()
      assert(unfilt.violations.some(v => v.includes('does not exist')))
    } finally {
      process.chdir(origDir)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  test('adr_orphan isenta arquivos de ADR externos à raiz do projeto (cwd)', () => {
    const externalDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-external-adrs-'))
    fs.writeFileSync(path.join(externalDir, 'ADR-EXTERNAL-999.md'), '---\nstatus: Accepted\n---\n# External ADR\n')

    const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-project-dir-'))
    fs.mkdirSync(path.join(projectDir, 'docs/req'), { recursive: true })
    fs.mkdirSync(path.join(projectDir, 'docs/adr'), { recursive: true })
    fs.writeFileSync(path.join(projectDir, 'docs/adr/ADR-LOCAL-001.md'), '---\nstatus: Accepted\n---\n# Local ADR\n')
    fs.writeFileSync(path.join(projectDir, 'trackfw.yaml'), `adr_dirs:\n  - docs/adr\n  - "${externalDir}"\n`)

    const origDir = process.cwd()
    process.chdir(projectDir)
    config.reset()
    try {
      const violations = validator.validateADRsAreReferenced()
      // ADR-LOCAL-001.md não está em nenhuma REQ -> deve ser marcado como violation adr_orphan
      assert(violations.some(v => v.includes('ADR-LOCAL-001.md')), 'ADR local não referenciado deve ser órfão')
      // ADR-EXTERNAL-999.md está fora do cwd -> DEVE SER ISENTO de adr_orphan
      assert(!violations.some(v => v.includes('ADR-EXTERNAL-999.md')), 'ADR externo deve ser ISENTO de adr_orphan')
    } finally {
      process.chdir(origDir)
      config.reset()
      fs.rmSync(externalDir, { recursive: true, force: true })
      fs.rmSync(projectDir, { recursive: true, force: true })
    }
  })

  console.log(`\n${passed} passed, ${failed} failed`)
  if (failed > 0) process.exit(1)
})()
