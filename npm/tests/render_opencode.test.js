'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')

const { render } = require('../src/integrations/render')
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
