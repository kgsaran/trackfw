'use strict'
const assert = require('assert')
const fs = require('fs')
const path = require('path')
const os = require('os')
const config = require('../src/config')

// Reset config singleton antes de cada teste que muda cwd
const validator = require('../src/validator')

let passed = 0, failed = 0, skipped = 0
function test(name, fn) {
  try { fn(); console.log('✓', name); passed++ }
  catch (e) { console.error('✗', name, e.message); failed++ }
}
async function testAsync(name, fn) {
  try { await fn(); console.log('✓', name); passed++ }
  catch (e) { console.error('✗', name, e.message); failed++ }
}
// testSkip registra testes esperando falha (defeito P2 exposto pelo ML-1A).
// Substitui xfail/skip de frameworks externos — sem nova dependência.
// Semântica strict: se o teste PASSAR, emite erro e incrementa failed,
// forçando a reativação após a Wave 2 convergir os templates.
function testSkip(name, fn) {
  try {
    fn()
    // Se chegou aqui o teste passou — defeito foi corrigido mas marcador não foi removido
    console.error('✗ [XPASS inesperado — remover testSkip após ML-2A]', name)
    failed++
  } catch (_e) {
    // Falha esperada — defeito ainda presente
    console.log('↷ [xfail esperado]', name)
    skipped++
  }
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

  // Estado analyzing: não deve gerar folder_status warning
  test('analyzing state: roadmap em analyzing/ com status: analyzing não gera folder_status warning', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-analyzing-'))
    const roadmapDir = path.join(tmp, 'roadmaps')
    fs.mkdirSync(path.join(roadmapDir, 'analyzing'), { recursive: true })
    fs.writeFileSync(
      path.join(roadmapDir, 'analyzing', 'ROADMAP-em-analise.md'),
      '---\nstatus: analyzing\ndate: 2026-07-26\n---\n# Roadmap: Em Análise\n'
    )
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${roadmapDir}\n`)

    const origCwd = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const result = validator.validateFolderStatusCoherence()
      assert(
        !result.some(w => w.includes('ROADMAP-em-analise.md')),
        `Roadmap em analyzing/ NÃO deve gerar folder_status warning, obteve: ${JSON.stringify(result)}`
      )
    } finally {
      process.chdir(origCwd)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  // Estado analyzing: wip_limit NÃO deve contar roadmaps em analyzing/
  test('analyzing state: wip_limit não conta roadmaps em analyzing/', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-analyzing-wip-'))
    const roadmapDir = path.join(tmp, 'roadmaps')
    fs.mkdirSync(path.join(roadmapDir, 'wip'), { recursive: true })
    fs.mkdirSync(path.join(roadmapDir, 'analyzing'), { recursive: true })
    // wip_limit=1, 1 em wip, 1 em analyzing → não deve exceder
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${roadmapDir}\nwip_limit: 1\nwip_by_squad: false\n`)
    fs.writeFileSync(path.join(roadmapDir, 'wip', 'ROADMAP-em-wip.md'), '# Roadmap em WIP\n\nREQ: REQ-001\n')
    fs.writeFileSync(
      path.join(roadmapDir, 'analyzing', 'ROADMAP-em-analise.md'),
      '---\nstatus: analyzing\n---\n# Roadmap em Análise\n'
    )

    const origCwd = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const result = validator.validateWIPLimit()
      assert.strictEqual(
        result.warnings.length, 0,
        `wip_limit NÃO deve contar analyzing/ — esperado 0 warnings, obteve: ${JSON.stringify(result.warnings)}`
      )
    } finally {
      process.chdir(origCwd)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  // ── validateBranchHasWIPRoadmap — 4 cenários (P4 do ADR) ──────────────────
  test('branch_has_wip_roadmap: cenário 1 — roadmap em wip/ com slug → sem violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip1-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', 'ROADMAP-my-feature.md'), 'REQ: REQ-001\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert.strictEqual(violations.length, 0, `roadmap em wip/ com slug deve passar, obteve: ${JSON.stringify(violations)}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('branch_has_wip_roadmap: cenário 2 — roadmap em done/ com slug → sem violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip2-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'done', 'ROADMAP-my-feature.md'), 'REQ: REQ-001\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert.strictEqual(violations.length, 0, `roadmap em done/ com slug deve passar, obteve: ${JSON.stringify(violations)}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('branch_has_wip_roadmap: cenário 3 — nenhum roadmap em wip/ nem done/ → violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip3-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert(violations.length > 0, 'sem roadmap em wip/ nem done/ deve reprovar')
        assert(violations[0].includes('no roadmap is in wip/ nor done/'), `mensagem esperada, obteve: ${violations[0]}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('branch_has_wip_roadmap: cenário 4 — roadmap em done/ com slug DIFERENTE → violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip4-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'done', 'ROADMAP-outra-coisa.md'), 'REQ: REQ-001\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert(violations.length > 0, 'slug diferente em done/ deve reprovar')
        assert(violations[0].includes('no matching roadmap in wip/ nor done/'), `mensagem esperada, obteve: ${violations[0]}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // -------------------------------------------------------------------------
  // Testes P2/P3 — adicionados pelo ML-2A (REQ-2026-07-26-robustez-gates)
  // -------------------------------------------------------------------------

  test('contentHasMarker: campo vazio CRLF não deve contar como presente (P3)', () => {
    const content = '# Roadmap\r\nREQ: \r\n## Seção\r\n'
    assert(!validator.contentHasMarker(content, ['REQ:']), 'campo vazio com CRLF não deve ser tratado como presente')
  })

  test('contentHasMarker: campo preenchido CRLF deve contar como presente (P3)', () => {
    const content = '# Roadmap\r\nREQ: REQ-001-titulo.md\r\n## Seção\r\n'
    assert(validator.contentHasMarker(content, ['REQ:']), 'campo preenchido com CRLF deve ser tratado como presente')
  })

  test('validateFolderStatusCoherence: diretório não legível (ENOTDIR) gera warning (P2)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-fsc-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps'), { recursive: true })
      // "analyzing" como arquivo regular — ENOTDIR ao tentar listar
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'analyzing'), 'eu sou um arquivo')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const warnings = validator.validateFolderStatusCoherence()
        assert(warnings.some(w => w.includes('could not read directory')),
          `esperado warning sobre diretório ilegível, obteve: ${JSON.stringify(warnings)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('validateFilenameUniqueness: diretório não legível (ENOTDIR) gera violation (P2)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-fnu-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps'), { recursive: true })
      // "wip" como arquivo regular — ENOTDIR ao tentar listar
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), 'eu sou um arquivo')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const violations = validator.validateFilenameUniqueness()
        assert(violations.some(v => v.includes('could not read directory')),
          `esperado violation sobre diretório ilegível, obteve: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('validateFilenameUniqueness: estados na mensagem em ordem alfabética (P3)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-p3-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', 'ROADMAP-duplicado.md'), '# Dup\n')
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'done', 'ROADMAP-duplicado.md'), '# Dup\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const violations = validator.validateFilenameUniqueness()
        assert(violations.length === 1, `esperado 1 violation, obteve ${violations.length}`)
        assert(violations[0].includes('[done, wip]'),
          `estados devem estar em ordem alfabética (done antes de wip), obteve: ${violations[0]}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('adr_dir_exists: tag correta em Node.js (P3 — paridade com Go/Python)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-adr-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs'), { recursive: true })
      // NÃO criar docs/adr — forçar violação
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'strict_ci_paths: true\nadr_dirs:\n  - docs/adr\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const result = validator.validateADRDirsExist()
        assert(result.violations.length > 0, 'esperado violation quando adr_dir não existe')
        assert(result.violations[0].includes('adr_dir "'), `mensagem deve usar 'adr_dir "', obteve: ${result.violations[0]}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // -------------------------------------------------------------------------
  // Testes P3+P4 adicionados pelo ML-3A (REQ-2026-07-26-robustez-gates)
  // -------------------------------------------------------------------------

  test('branch_has_wip_roadmap: 4 candidatos → mensagem truncada em 3 + "e mais 1" em ordem alfabética (P3+P4)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-trunc-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      // 4 roadmaps sem slug da branch → todos são candidatos, nenhum casa
      for (const name of ['ROADMAP-alpha.md', 'ROADMAP-bravo.md', 'ROADMAP-charlie.md', 'ROADMAP-delta.md']) {
        fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', name), 'REQ: REQ-001\n')
      }
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/minha-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert(violations.length > 0, 'esperava violation com 4 candidatos sem slug correspondente')
        const want = 'ROADMAP-alpha.md, ROADMAP-bravo.md, ROADMAP-charlie.md, e mais 1'
        assert(violations[0].includes(want),
          `mensagem truncada deve conter "${want}", obteve: ${violations[0]}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('wip_has_req: roadmap CRLF com REQ vazio emite violation (P3+P4 — integração)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-crlf-wip-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      // Arquivo CRLF: REQ: seguido de espaço + \r\n — campo vazio
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', 'ROADMAP-crlf.md'),
        'REQ: \r\n## Acceptance Criteria\r\n- [ ] ok\r\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const violations = validator.validateWIPHasREQ()
        assert(violations.some(v => v.includes('wip but has no linked REQ')),
          `esperava violation de REQ vazio com CRLF, obteve: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // ---------------------------------------------------------------------------
  // ML-2A — REQ-2026-07-27-convergencia-templates-python (reativado)
  // Após convergência dos templates Python, as regras devem detectar os artefatos
  // no formato canônico Go/Node/Python. Testes convertidos de testSkip para test.
  // ---------------------------------------------------------------------------

  // ML-2A: adrIsDraft detecta ADR no formato canônico (após convergência Python)
  // ADR canônico tem "> Date: … | Status: Draft" — detectado por adrIsDraft().
  // Fixture: ADR canônico + REQ canônica.
  test('ML-2A: adrIsDraft detecta ADR Draft no formato canonico (REQ-2026-07-27-convergencia-templates-python)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-adr-can-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })

      // ADR no formato canônico (produzido pelo gerador Python após ML-2A):
      // header "> Date: … | Status: Draft" — detectado por adrIsDraft()
      const adrCanonico = `---\nstatus: Draft\ndate: 2026-07-27\nauthor: ""\n---\n\n# ADR: auth strategy\n\n> Date: 2026-07-27 | Status: Draft\n\n## Context\n<!-- What is the situation that motivates this decision? -->\n\n## Decision\n<!-- What was decided? -->\n\n## Consequences\n<!-- What are the positive and negative consequences of this decision? -->\n\n## Alternatives Considered\n<!-- What other options were evaluated and why were they rejected? -->\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-07-27-auth-strategy.md'), adrCanonico)

      // REQ no formato canônico Go/Node: tem "> Date: … | Status: Open"
      const reqCanonicalContent = `# REQ: Login\n\n> Date: 2026-07-27 | Status: Open\n\n## Motivation\n\n## Acceptance Criteria\n\n- [ ] criterio\n\n## Linked ADR\nADR:\n\n## Blocked by ADRs\n- ADR-2026-07-27-auth-strategy.md (Draft)\n\n## Linked Roadmap\nRoadmap:\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'req', 'REQ-2026-07-27-login.md'), reqCanonicalContent)
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `req_dir: docs/req\nadr_dirs:\n  - docs/adr\n`)

      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        // Pré-condição: ADR existe
        assert(fs.existsSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-07-27-auth-strategy.md')),
          'pré-condição: ADR não encontrado')

        const violations = validator.validateREQsNotBlockedByDraftADRs()
        // DEVE disparar violation — formato canônico tem "Status: Draft" que adrIsDraft detecta
        assert(violations.length > 0,
          `regressao: blocked_by_draft_adr nao detectou ADR Draft no formato canonico. ` +
          `adrIsDraft deve encontrar '| Status: Draft' inline. violations: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // ML-2A: validator detecta REQ Open no formato canônico (após convergência Python)
  // REQ canônica tem "> Date: … | Status: Open" — detectada pelo guard inicial.
  // Fixture: REQ canônica + ADR canônico Draft.
  test('ML-2A: validator detecta REQ Open no formato canonico (REQ-2026-07-27-convergencia-templates-python)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-req-can-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })

      // ADR no formato canônico Go/Node: tem "> Date: … | Status: Draft"
      const adrCanonicalContent = `# ADR: Auth\n\n> Date: 2026-07-27 | Status: Draft\n\n## Context\ncontext\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-07-27-auth-draft.md'), adrCanonicalContent)

      // REQ no formato canônico (produzida pelo gerador Python após ML-2A):
      // header "> Date: … | Status: Open" detectado pelo guard inicial.
      const reqCanonico = `---\nstatus: Open\ndate: 2026-07-27\nauthor: ""\nadr: ""\nroadmap: ""\n---\n\n# REQ: login\n\n> Date: 2026-07-27 | Status: Open\n\n## Motivation\n<!-- Why is this requirement needed? What problem does it solve? -->\n\n## Acceptance Criteria\n- [ ]\n- [ ]\n\n## Linked ADR\n<!-- Reference the ADR that governs this requirement -->\nADR: \n\n## Blocked by ADRs\n- ADR-2026-07-27-auth-draft.md (Draft)\n\n## Linked Roadmap\n<!-- Reference the roadmap that implements this requirement -->\nRoadmap: \n`
      fs.writeFileSync(path.join(tmp, 'docs', 'req', 'REQ-2026-07-27-login.md'), reqCanonico)
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `req_dir: docs/req\nadr_dirs:\n  - docs/adr\n`)

      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        // Pré-condição: ADR canônico deve ser detectado como Draft
        assert(validator.adrIsDraft('ADR-2026-07-27-auth-draft.md'),
          'pré-condição falhou: adrIsDraft deve retornar true para ADR canônico com Status: Draft')

        const violations = validator.validateREQsNotBlockedByDraftADRs()
        // DEVE disparar violation — formato canônico tem "Status: Open" que o guard detecta
        assert(violations.length > 0,
          `regressao: blocked_by_draft_adr nao detectou REQ Open no formato canonico. ` +
          `REQ tem '> Date: ... | Status: Open' (inline) — deve ser detectada. violations: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  console.log(`\n${passed} passed, ${failed} failed, ${skipped} xfail`)
  if (failed > 0) process.exit(1)
})()
