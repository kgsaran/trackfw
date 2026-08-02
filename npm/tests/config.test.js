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

test('forge e trace_id_field com aspas sao normalizados', () => {
  const yaml = `forge: "github"\ntrace_id_field: 'req_id'\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.forge, 'github')
    assert.strictEqual(cfg.traceIdField, 'req_id')
  })
})

// ML-1B — sequência em bloco no mesmo nível de indentação da chave é YAML válido
// (confirmado com yaml.safe_load real: "agents:\n- zeus\n- apolo" resolve para
// {'agents': ['zeus', 'apolo']}). Antes do fix, qualquer linha sem indentação — mesmo "- item"
// — era tratada como top-level e fechava a lista aberta, descartando-a silenciosamente.

test('adr_dirs — sequência não indentada é lida corretamente', () => {
  const yaml = `adr_dirs:\n- docs/adr/zeus\n- docs/adr/apolo\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.adrDirs, ['docs/adr/zeus', 'docs/adr/apolo'])
  })
})

test('agents — sequência não indentada é lida corretamente', () => {
  const yaml = `agents:\n- zeus\n- apolo\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.agents, ['zeus', 'apolo'])
  })
})

test('acceptance_markers — sequência não indentada é lida corretamente', () => {
  const yaml = `acceptance_markers:\n- "## Done"\n- "## Concluído"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.acceptanceMarkers, ['## Done', '## Concluído'])
  })
})

// link_fields: a chave e as sub-chaves (req/adr/roadmap) precisam permanecer indentadas —
// exigência da própria especificação YAML para mapeamentos aninhados. O que pode variar é a
// indentação dos ITENS da sequência em relação à sub-chave que os abre.
test('link_fields — itens no mesmo nível da sub-chave (req/adr/roadmap)', () => {
  const yaml = `link_fields:\n  req:\n  - "REQ:"\n  - "req_id"\n  adr:\n  - "ADR:"\n  roadmap:\n  - "Roadmap:"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:', 'req_id'])
    assert.deepStrictEqual(cfg.linkFields.adr, ['ADR:'])
    assert.deepStrictEqual(cfg.linkFields.roadmap, ['Roadmap:'])
  })
})

test('link_fields — itens mais indentados que a sub-chave (forma original, sem regressão)', () => {
  const yaml = `link_fields:\n  req:\n    - "REQ:"\n  adr:\n    - "ADR:"\n  roadmap:\n    - "Roadmap:"\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:'])
    assert.deepStrictEqual(cfg.linkFields.adr, ['ADR:'])
    assert.deepStrictEqual(cfg.linkFields.roadmap, ['Roadmap:'])
  })
})

// rules: mapeamento (chave: valor), não sequência. Sub-chaves não indentadas NÃO são YAML
// válido de forma aninhada (viram chaves top-level soltas — confirmado com yaml.safe_load).
// Documenta que o comportamento é intencional, não uma regressão do fix de listas.
test('rules — sub-chave não indentada não é aninhada (comportamento esperado)', () => {
  const yaml = `rules:\nstale_wip: error\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.rules.stale_wip, 'warning') // default preservado
  })
})

test('todas as cinco chaves — forma indentada continua funcionando (sem regressão)', () => {
  const yaml = `adr_dirs:\n  - docs/adr/zeus\n  - docs/adr/apolo\nagents:\n  - zeus\n  - apolo\nacceptance_markers:\n  - "## Done"\nlink_fields:\n  req:\n    - "REQ:"\n  adr:\n    - "ADR:"\n  roadmap:\n    - "Roadmap:"\nrules:\n  stale_wip: error\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.adrDirs, ['docs/adr/zeus', 'docs/adr/apolo'])
    assert.deepStrictEqual(cfg.agents, ['zeus', 'apolo'])
    assert.deepStrictEqual(cfg.acceptanceMarkers, ['## Done'])
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:'])
    assert.strictEqual(cfg.rules.stale_wip, 'error')
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
