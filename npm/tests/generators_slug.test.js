'use strict'

/**
 * Testes de slug acentuado para os geradores de artefato Node.js.
 *
 * Invariante: título acentuado deve gerar o mesmo slug ASCII nos 3 runtimes.
 * Cobre: á é í ó ú (agudo), ç (cedilha), ã õ (til), à (crase).
 *
 * REQ-2026-07-27-convergencia-templates-python / ML-3B
 */

const test = require('node:test')
const assert = require('node:assert/strict')

const { toSlug: toSlugADR } = require('../src/generators/adr')
const { toSlug: toSlugREQ } = require('../src/generators/req')
const { toSlug: toSlugNote } = require('../src/generators/note')
const { toSlug: toSlugRoadmap } = require('../src/generators/roadmap')

const cases = [
  { input: 'Autenticação e Sessão',       expected: 'autenticacao-e-sessao' },
  { input: 'Criação de Requisição',        expected: 'criacao-de-requisicao' },
  { input: 'Configuração Avançada',        expected: 'configuracao-avancada' },
  { input: 'Título com À crase e Ã til',  expected: 'titulo-com-a-crase-e-a-til' },
  { input: 'ADR Config (v2)',              expected: 'adr-config-v2' },
  { input: 'á é í ó ú',                   expected: 'a-e-i-o-u' },
  { input: 'ç ã õ à',                     expected: 'c-a-o-a' },
]

for (const { input, expected } of cases) {
  test(`toSlug adr.js: "${input}" → "${expected}"`, () => {
    assert.equal(toSlugADR(input), expected)
  })
  test(`toSlug req.js: "${input}" → "${expected}"`, () => {
    assert.equal(toSlugREQ(input), expected)
  })
  test(`toSlug note.js: "${input}" → "${expected}"`, () => {
    assert.equal(toSlugNote(input), expected)
  })
  test(`toSlug roadmap.js: "${input}" → "${expected}"`, () => {
    assert.equal(toSlugRoadmap(input), expected)
  })
}
