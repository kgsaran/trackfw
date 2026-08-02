'use strict'
const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')
const config = require('../src/config/index.js')

function withTmpDir(yaml, fn) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-config-inline-'))
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

// Contrato (ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-dos-tres-clis):
//
//  # | Entrada                                                | Resultado
//  1 | [a, b]                                                 | [a, b]
//  2 | [a,b]                                                  | [a, b]
//  3 | [ a , b ]                                               | [a, b]
//  4 | ["a", "b"]                                             | [a, b]
//  5 | ['a', 'b']                                             | [a, b]
//  6 | [a]                                                     | [a]
//  7 | []                                                      | lista vazia, não default
//  8 | ["a, b", "c"]                                          | dois itens: "a, b" e "c"
//  9 | ["## Acceptance Criteria", "## Critérios de Aceite"]   | os dois marcadores

const CASES = [
  ['1_espaco_simples', '[a, b]', ['a', 'b']],
  ['2_sem_espaco', '[a,b]', ['a', 'b']],
  ['3_espacos_extras', '[ a , b ]', ['a', 'b']],
  ['4_aspas_duplas', '["a", "b"]', ['a', 'b']],
  ['5_aspas_simples', "['a', 'b']", ['a', 'b']],
  ['6_item_unico', '[a]', ['a']],
  ['7_lista_vazia', '[]', []],
  ['8_virgula_dentro_de_aspas', '["a, b", "c"]', ['a, b', 'c']],
  ['9_marcadores_reais', '["## Acceptance Criteria", "## Critérios de Aceite"]', ['## Acceptance Criteria', '## Critérios de Aceite']],
]

for (const [name, input, want] of CASES) {
  test(`adr_dirs inline — ${name}`, () => {
    const yaml = `adr_dirs: ${input}\n`
    withTmpDir(yaml, (tmp) => {
      const cfg = config.load(tmp)
      assert.deepStrictEqual(cfg.adrDirs, want)
    })
  })

  test(`agents inline — ${name}`, () => {
    const yaml = `agents: ${input}\n`
    withTmpDir(yaml, (tmp) => {
      const cfg = config.load(tmp)
      assert.deepStrictEqual(cfg.agents, want)
    })
  })

  test(`acceptance_markers inline — ${name}`, () => {
    const yaml = `acceptance_markers: ${input}\n`
    withTmpDir(yaml, (tmp) => {
      const cfg = config.load(tmp)
      assert.deepStrictEqual(cfg.acceptanceMarkers, want)
    })
  })

  test(`link_fields.req inline — ${name}`, () => {
    const yaml = `link_fields:\n  req: ${input}\n`
    withTmpDir(yaml, (tmp) => {
      const cfg = config.load(tmp)
      assert.deepStrictEqual(cfg.linkFields.req, want)
    })
  })

  test(`link_fields.adr inline — ${name}`, () => {
    const yaml = `link_fields:\n  adr: ${input}\n`
    withTmpDir(yaml, (tmp) => {
      const cfg = config.load(tmp)
      assert.deepStrictEqual(cfg.linkFields.adr, want)
    })
  })

  test(`link_fields.roadmap inline — ${name}`, () => {
    const yaml = `link_fields:\n  roadmap: ${input}\n`
    withTmpDir(yaml, (tmp) => {
      const cfg = config.load(tmp)
      assert.deepStrictEqual(cfg.linkFields.roadmap, want)
    })
  })
}

test('adr_dirs inline — expande ~ por item', () => {
  const yaml = `adr_dirs: [~/adr, docs/adr]\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.strictEqual(cfg.adrDirs.length, 2)
    assert.strictEqual(cfg.adrDirs[1], 'docs/adr')
    assert.notStrictEqual(cfg.adrDirs[0], '~/adr')
  })
})

test('forma inline não abre contexto de bloco para as linhas seguintes', () => {
  const yaml = `agents: [zeus, apolo]\nreq_dir: docs/req\n`
  withTmpDir(yaml, (tmp) => {
    const cfg = config.load(tmp)
    assert.deepStrictEqual(cfg.agents, ['zeus', 'apolo'])
    assert.strictEqual(cfg.reqDir, 'docs/req')
  })
})

console.log(`\n${passed} passed, ${failed} failed`)
if (failed > 0) process.exit(1)
