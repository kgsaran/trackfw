'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const config = require('../src/config/index.js')

// Este arquivo cobre o AC3 (fidelidade textual da normalização de escalares para string na
// fronteira, ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-string-
// na-fronteira.md), teste por chave das ~20 chaves suportadas, as formas antes não suportadas
// (mapa inline, lista aninhada inline, âncora) e o comportamento com config ausente/vazio ou
// malformado.

function withTmpDir(yaml, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-config-fidelity-'))
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

// --- AC3: fidelidade textual ---

test('AC3: lenient_until data nua permanece string YYYY-MM-DD', () => {
  withTmpDir('lenient_until: 2026-08-02\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.lenientUntil, '2026-08-02')
  })
})

test('AC3: wip_limit octal 010 vira 10, nao 8', () => {
  withTmpDir('wip_limit: 010\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.wipLimit, 10)
  })
})

test('AC3: wip_limit decimal simples', () => {
  withTmpDir('wip_limit: 3\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.wipLimit, 3)
  })
})

test('AC3: wip_by_squad true', () => {
  withTmpDir('wip_by_squad: true\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.wipBySquad, true)
  })
})

test('AC3: governance_mode yes permanece string "yes" (nao bool)', () => {
  withTmpDir('governance_mode: yes\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.governanceMode, 'yes')
  })
})

test('AC3: governance_mode 1.0 preserva ponto decimal', () => {
  withTmpDir('governance_mode: 1.0\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.governanceMode, '1.0')
  })
})

test('AC3: governance_mode null', () => {
  withTmpDir('governance_mode: null\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.governanceMode, 'null')
  })
})

test('AC3: governance_mode ~ (til)', () => {
  withTmpDir('governance_mode: ~\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.governanceMode, '~')
  })
})

// --- Âncoras ---

test('AC3: ancora — b: *x chega com o VALOR do anchor, nao o nome', () => {
  withTmpDir('governance_mode: &gm strict\nforge: *gm\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.governanceMode, 'strict')
    assert.strictEqual(cfg.forge, 'strict')
  })
})

test('AC3: ancora dentro de lista resolve', () => {
  withTmpDir('agents: [&a zeus, apolo, *a]\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.agents, ['zeus', 'apolo', 'zeus'])
  })
})

// --- Formas antes não suportadas ---

test('mapa inline — rules: {stale_wip: error, adr_orphan: warning}', () => {
  withTmpDir('rules: {stale_wip: error, adr_orphan: warning}\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.rules.stale_wip, 'error')
    assert.strictEqual(cfg.rules.adr_orphan, 'warning')
    assert.strictEqual(cfg.rules.wip_has_req, 'error') // default preservado
  })
})

test('lista aninhada inline — link_fields.req: ["REQ:", "req_id"]', () => {
  withTmpDir('link_fields:\n  req: ["REQ:", "req_id"]\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.linkFields.req, ['REQ:', 'req_id'])
  })
})

// --- Config ausente/vazio/malformado ---

test('config vazio cai nos defaults, sem erro', () => {
  withTmpDir('', (tmp) => {
    const cfg = config.load(tmp)
    const want = config.defaults()
    assert.deepStrictEqual(cfg, want)
  })
})

test('config só com comentário cai nos defaults, sem erro', () => {
  withTmpDir('# apenas um comentário\n\n', (tmp) => {
    const cfg = config.load(tmp)
    const want = config.defaults()
    assert.deepStrictEqual(cfg, want)
  })
})

// assertLoadFailsLoud roda config.load(tmp) num subprocesso Node isolado (necessário porque o
// caminho fatal chama process.exit(1), que mataria o processo do próprio test file) e confirma
// stderr == MALFORMED_CONFIG_MESSAGE + exit 1.
function assertLoadFailsLoud(tmp) {
  const script = `
    const config = require(${JSON.stringify(path.join(__dirname, '..', 'src', 'config', 'index.js'))})
    config.load(${JSON.stringify(tmp)})
  `
  const result = spawnSync(process.execPath, ['-e', script], { encoding: 'utf8' })
  assert.strictEqual(result.status, 1, `exit code: got ${result.status}, want 1 (stderr=${result.stderr})`)
  assert.strictEqual(result.stderr, config.MALFORMED_CONFIG_MESSAGE + '\n')
}

// Fixture confirmada como erro real de parse nos 3 CLIs (não vacua sob nenhum schema): Go
// yaml.v3 devolve "did not find expected ',' or ']'"; PyYAML levanta YAMLError equivalente;
// yaml (Node) popula doc.errors (não lança, mas o array não fica vazio).
//
// ML-1B: config.load() agora falha alto (stderr + exit 1) em YAML malformado, em vez do
// fallback silencioso anterior — o fallback silencioso era regressão frente ao parser
// artesanal (que nunca via a config inteira ser descartada por um erro de sintaxe local).
test('YAML malformado falha alto: stderr com MALFORMED_CONFIG_MESSAGE e exit 1', () => {
  withTmpDir('agents: [zeus, apolo\nwip_limit: 3\n', (tmp) => {
    assertLoadFailsLoud(tmp)
  })
})

// Divergência encontrada na auditoria cruzada do ML-1B: sem MULTIPLE_DOCS forçando o caminho
// fatal, o Node aceitaria isso normalmente (single-doc por default), mas a lib `yaml` já marca
// doc.errors com code MULTIPLE_DOCS para stream com mais de um documento — o mesmo shape falha
// em Go (yaml.Unmarshal decodifica só o 1º doc e SEM hasMultipleDocuments não erraria) e em
// PyYAML (yaml.compose: "expected a single document in the stream").
test('multiplos documentos ("---" duas vezes) falha alto, igual aos outros 2 CLIs', () => {
  withTmpDir('wip_limit: 3\n---\nwip_limit: 5\n', (tmp) => {
    assertLoadFailsLoud(tmp)
  })
})

// Segunda divergência da mesma auditoria: referência de âncora antes da definição (b: *x /
// a: &x 3) é inválida pela spec YAML — yaml.v3 e PyYAML rejeitam, mas Alias#resolve(doc) do
// Node simplesmente devolve undefined sem popular doc.errors. resolveAlias(state) detecta isso
// e devolve malformado=true — sem essa detecção, o valor viraria string vazia em silêncio.
test('referencia de ancora antes da definicao falha alto, igual aos outros 2 CLIs', () => {
  withTmpDir('b: *x\na: &x 3\n', (tmp) => {
    assertLoadFailsLoud(tmp)
  })
})

// Contraprova: referência de âncora DEPOIS da definição (ordem normal) continua válida — não
// pode virar falso-positivo pela checagem de alias não-resolvido acima.
test('referencia de ancora depois da definicao continua valida (nao regride)', () => {
  withTmpDir('a: &x 3\nb: *x\n', (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.wipLimit, 1) // nao eh chave conhecida, so confirma que load() nao lancou
  })
})

// Divergência (na direção oposta) da mesma auditoria: chaves top-level duplicadas SÃO aceitas
// (last-wins) por gopkg.in/yaml.v3 (decodificando em *yaml.Node, não struct) e por PyYAML
// yaml.compose() — só a lib `yaml` do Node marca isso como erro (DUPLICATE_KEY). Sem o filtro
// NON_FATAL_ERROR_CODES, o Node seria o único a falhar aqui.
test('chaves top-level duplicadas nao sao malformado (last-wins, sem exit fatal)', () => {
  withTmpDir('wip_limit: 3\nwip_limit: 4\n', (tmp) => {
    assert.strictEqual(config.load(tmp).wipLimit, 4)
  })
})

// --- Teste por chave (~20 chaves) ---

test('chave: adr_dirs', () => {
  withTmpDir('adr_dirs:\n  - docs/adr/x\n', (tmp) => {
    assert.deepStrictEqual(config.load(tmp).adrDirs, ['docs/adr/x'])
  })
})

test('chave: agents', () => {
  withTmpDir('agents:\n  - zeus\n', (tmp) => {
    assert.deepStrictEqual(config.load(tmp).agents, ['zeus'])
  })
})

test('chave: req_dir', () => {
  withTmpDir('req_dir: docs/req2\n', (tmp) => {
    assert.strictEqual(config.load(tmp).reqDir, 'docs/req2')
  })
})

test('chave: roadmap_dir', () => {
  withTmpDir('roadmap_dir: docs/rm2\n', (tmp) => {
    assert.strictEqual(config.load(tmp).roadmapDir, 'docs/rm2')
  })
})

test('chave: roadmap_namespacing', () => {
  withTmpDir('roadmap_namespacing: by_agent\n', (tmp) => {
    assert.strictEqual(config.load(tmp).roadmapNamespacing, 'by_agent')
  })
})

test('chave: acceptance_markers', () => {
  withTmpDir('acceptance_markers:\n  - "## Done"\n', (tmp) => {
    assert.deepStrictEqual(config.load(tmp).acceptanceMarkers, ['## Done'])
  })
})

test('chave: link_fields.req', () => {
  withTmpDir('link_fields:\n  req:\n    - "req_id"\n', (tmp) => {
    assert.deepStrictEqual(config.load(tmp).linkFields.req, ['req_id'])
  })
})

test('chave: link_fields.adr', () => {
  withTmpDir('link_fields:\n  adr:\n    - "adr_id"\n', (tmp) => {
    assert.deepStrictEqual(config.load(tmp).linkFields.adr, ['adr_id'])
  })
})

test('chave: link_fields.roadmap', () => {
  withTmpDir('link_fields:\n  roadmap:\n    - "rm_id"\n', (tmp) => {
    assert.deepStrictEqual(config.load(tmp).linkFields.roadmap, ['rm_id'])
  })
})

test('chave: rules', () => {
  withTmpDir('rules:\n  stale_wip: error\n', (tmp) => {
    assert.strictEqual(config.load(tmp).rules.stale_wip, 'error')
  })
})

test('chave: wip_limit', () => {
  withTmpDir('wip_limit: 4\n', (tmp) => {
    assert.strictEqual(config.load(tmp).wipLimit, 4)
  })
})

test('chave: wip_by_squad', () => {
  withTmpDir('wip_by_squad: true\n', (tmp) => {
    assert.strictEqual(config.load(tmp).wipBySquad, true)
  })
})

test('chave: stale_wip_days', () => {
  withTmpDir('stale_wip_days: 14\n', (tmp) => {
    assert.strictEqual(config.load(tmp).staleWipDays, 14)
  })
})

test('chave: lenient_until', () => {
  withTmpDir('lenient_until: 2026-09-01\n', (tmp) => {
    assert.strictEqual(config.load(tmp).lenientUntil, '2026-09-01')
  })
})

test('chave: governance_mode', () => {
  withTmpDir('governance_mode: strict\n', (tmp) => {
    assert.strictEqual(config.load(tmp).governanceMode, 'strict')
  })
})

test('chave: require_req_in_commit', () => {
  withTmpDir('require_req_in_commit: true\n', (tmp) => {
    assert.strictEqual(config.load(tmp).requireReqInCommit, true)
  })
})

test('chave: strict_ci_paths', () => {
  withTmpDir('strict_ci_paths: true\n', (tmp) => {
    assert.strictEqual(config.load(tmp).strictCiPaths, true)
  })
})

test('chave: trace_id_field', () => {
  withTmpDir('trace_id_field: req_id\n', (tmp) => {
    assert.strictEqual(config.load(tmp).traceIdField, 'req_id')
  })
})

test('chave: forge', () => {
  withTmpDir('forge: gitlab\n', (tmp) => {
    assert.strictEqual(config.load(tmp).forge, 'gitlab')
  })
})

test('chave: squad (nao consumida por ProjectConfig, nao deve quebrar leitura das demais)', () => {
  withTmpDir('squad: platform\nreq_dir: docs/req\n', (tmp) => {
    assert.strictEqual(config.load(tmp).reqDir, 'docs/req')
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
