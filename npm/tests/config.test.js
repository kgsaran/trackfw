'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const config = require('../src/config/index.js')

function withTmpDir(yaml, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-config-'))
  try {
    if (yaml) fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), yaml, 'utf8')
    config.reset()
    fn(tmp)
  } finally {
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
}

let passed = 0, failed = 0

function test(name, fn) {
  try { fn(); console.log(`✓ ${name}`); passed++ }
  catch (e) { console.error(`✗ ${name}: ${e.message}`); failed++ }
}

test('defaults — linkFields, acceptanceMarkers, rules', () => {
  withTmpDir(null, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:'])
    assert.deepStrictEqual(cfg.linkFields.adr, ['ADR:'])
    assert.deepStrictEqual(cfg.linkFields.roadmap, ['Roadmap:'])
    assert.deepStrictEqual(cfg.acceptanceMarkers, ['## Acceptance Criteria', '## Critérios de Aceite'])
    assert.strictEqual(cfg.rules.wip_has_req, 'error')
    assert.strictEqual(cfg.rules.stale_wip, 'warning')
  })
})

test('link_fields customizado', () => {
  const yaml = `link_fields:\n  req:\n    - "REQ:"\n    - "req_id"\n  adr:\n    - "ADR:"\n  roadmap:\n    - "Roadmap:"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:', 'req_id'])
    assert.deepStrictEqual(cfg.linkFields.adr, ['ADR:'])
  })
})

test('acceptance_markers customizado', () => {
  const yaml = `acceptance_markers:\n  - "## Done"\n  - "## Concluído"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.acceptanceMarkers, ['## Done', '## Concluído'])
  })
})

test('rules parcial — merge com defaults', () => {
  const yaml = `rules:\n  stale_wip: error\n  adr_orphan: off\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.rules.stale_wip, 'error')
    assert.strictEqual(cfg.rules.adr_orphan, 'off')
    assert.strictEqual(cfg.rules.wip_has_req, 'error') // default mantido
  })
})

test('sparse — só wip_limit, novos campos usam defaults', () => {
  withTmpDir('wip_limit: 3\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.wipLimit, 3)
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:'])
    assert.strictEqual(cfg.rules.wip_has_req, 'error')
  })
})

test('retrocompat — yaml v2.3 sem novos campos', () => {
  const yaml = `adr_dirs:\n  - docs/adr\nwip_limit: 2\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.adrDirs, ['docs/adr'])
    assert.strictEqual(cfg.wipLimit, 2)
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:']) // default
  })
})

test('rules com aspas duplas são reconhecidas', () => {
  const yaml = `rules:\n  adr_orphan: "off"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.rules.adr_orphan, 'off')
  })
})

test('rules com aspas simples são reconhecidas', () => {
  const yaml = `rules:\n  stale_wip: 'warning'\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.rules.stale_wip, 'warning')
  })
})

// ML-2B — paths configuráveis adr_dirs/req_dir/roadmap_dir
test('adr_dirs com dois itens → adrDirs é array com dois valores', () => {
  const yaml = `adr_dirs:\n  - docs/adr\n  - docs/decisoes\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.adrDirs, ['docs/adr', 'docs/decisoes'])
  })
})

test('req_dir customizado → cfg.reqDir correto', () => {
  const yaml = `req_dir: "docs/requisições"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.reqDir, 'docs/requisições')
  })
})

test('roadmap_dir customizado → cfg.roadmapDir correto', () => {
  const yaml = `roadmap_dir: "docs/roadmaps/claude"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.roadmapDir, 'docs/roadmaps/claude')
  })
})

test('sem adr_dirs/req_dir/roadmap_dir → defaults corretos', () => {
  withTmpDir(null, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.adrDirs, ['docs/adr'])
    assert.strictEqual(cfg.reqDir, 'docs/req')
    assert.strictEqual(cfg.roadmapDir, 'docs/roadmaps')
  })
})

// ML-1B — Expansão de ~ (tilde) em adr_dirs, req_dir, roadmap_dir
test('expandPath — expande ~ e ~/ para o diretório Home', () => {
  const home = os.homedir()
  assert.strictEqual(config.expandPath('~'), home)
  assert.strictEqual(config.expandPath('~/global-adrs'), path.join(home, 'global-adrs'))
  assert.strictEqual(config.expandPath('~\\global-adrs'), path.join(home, 'global-adrs'))
  assert.strictEqual(config.expandPath('docs/adr'), 'docs/adr')
  assert.strictEqual(config.expandPath(null), null)
})

test('adr_dirs com ~ em trackfw.yaml → expandido para homedir', () => {
  const home = os.homedir()
  const yaml = `adr_dirs:\n  - ~/company-adrs\n  - docs/adr\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.adrDirs, [path.join(home, 'company-adrs'), 'docs/adr'])
  })
})

// ML-2B — strict_ci_paths
test('strict_ci_paths — default é false, aceita true via yaml', () => {
  withTmpDir(null, (tmp) => {
    const cfgDefault = config.load(tmp)
    assert.strictEqual(cfgDefault.strictCiPaths, false)
  })
  withTmpDir('strict_ci_paths: true\n', (tmp) => {
    const cfgTrue = config.load(tmp)
    assert.strictEqual(cfgTrue.strictCiPaths, true)
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)

