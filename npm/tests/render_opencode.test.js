'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')

const { render, normalizeCRLF } = require('../src/integrations/render')
const { items, readAsset } = require('../src/integrations/catalog')

// TestRenderOpenCodeAgent (paridade com internal/integrations/render_test.go)
//
// Prova que a representação "opencode-agent" reconstrói o frontmatter do
// zero (mesmo estilo do case "agent-directory") de um jeito que o OpenCode
// real (1.18.13) aceita: description presente, "mode: subagent" sempre fixo,
// e "model:"/"tools:"/"memory:" AUSENTES — achado #3 da Wave 1 do roadmap
// ROADMAP-2026-08-04-compatibilidade-com-opencode: "tools:" é chave
// reservada no schema do OpenCode (recusa TODO o carregamento do projeto se
// receber a lista estilo Claude Code) e "model:" é omitido por decisão de
// produto (deixar o OpenCode resolver pelo default já configurado pelo
// usuário em opencode.json, alinhado com a motivação de negócio do REQ de
// permitir modelos open-source/locais).
test('opencode-agent renderer produz frontmatter description + mode: subagent, sem model/tools/memory', () => {
  const item = items('agents').find(entry => entry.id === 'backend')
  assert.ok(item, "agente 'backend' não encontrado no catalog")
  const content = readAsset(item)

  const output = render({
    kind: 'agents',
    content,
    capability: { representation: 'opencode-agent' },
    item,
    identity: undefined,
  })

  assert.match(output, /^---\n/, 'esperado frontmatter delimitado por ---')
  assert.match(output, /description:/, "esperado campo 'description:' no frontmatter")
  assert.match(output, /mode: subagent\n/, "esperado 'mode: subagent' fixo no frontmatter")

  for (const forbidden of ['model:', 'tools:', 'memory:']) {
    assert.doesNotMatch(
      output,
      new RegExp(forbidden),
      `campo ${forbidden} não deve aparecer no frontmatter do OpenCode (schema incompatível)`,
    )
  }

  // corpo original preservado
  assert.match(output, /# Backend/, 'corpo original perdido')
})

// Falsificação exigida pela ADR (ADR-2026-09-04-parser-de-frontmatter-tolera-
// crlf-na-fronteira-de-entrada, "Verificação exigida"): o mesmo asset, com
// todo "\n" trocado por "\r\n" ANTES de render() vê-lo, deve renderizar
// byte-idêntico ao asset LF. É esta reprodução exata da divergência medida
// no handoff (Node: LF → name=teste/model=sonnet; CRLF → name=trackfw-agent,
// model vazio, frontmatter inteiro no corpo) que este teste afirma ter
// fechado — CONCLUSÃO 1 do ML-5A.
//
// O asset em disco é LF (git checkout local nunca produz CRLF); o CRLF é
// injetado no source em memória, que é a fronteira que o D1 da ADR
// normaliza.
test('opencode-agent renderer: source CRLF renderiza byte-idêntico ao source LF (ADR CRLF)', () => {
  const item = items('agents').find(entry => entry.id === 'backend')
  const lfContent = readAsset(item)
  const crlfContent = lfContent.replace(/\n/g, '\r\n')

  const lfOutput = render({
    kind: 'agents', content: lfContent, capability: { representation: 'opencode-agent' }, item, identity: undefined,
  })
  const crlfOutput = render({
    kind: 'agents', content: crlfContent, capability: { representation: 'opencode-agent' }, item, identity: undefined,
  })

  assert.equal(crlfOutput, lfOutput, 'source CRLF deveria renderizar igual ao source LF')
  // Controle D2: entrada CRLF não pode virar saída com "\r" — a tolerância na
  // leitura nunca autoriza emitir CRLF no que o trackfw escreve.
  assert.ok(!crlfOutput.includes('\r'), 'saída renderizada não deveria conter "\\r" (D2)')
})

// Falsificação unitária de normalizeCRLF: dobra "\r\n" em "\n" e preserva um
// "\r" solto (sem "\n" em seguida) intocado — não-objetivo declarado da ADR
// (D4: Mac clássico fica fora de escopo).
test('normalizeCRLF dobra somente CRLF, preserva CR solto', () => {
  assert.equal(normalizeCRLF('a\r\nb\r\nc\rd'), 'a\nb\nc\rd')
})
