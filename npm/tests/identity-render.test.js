'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { buildPlans, IntegrationManager } = require('../src/integrations')
const { rewriteSignatureLine } = require('../src/integrations/render')

function roots() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-identity-render-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot)
  fs.mkdirSync(homeRoot)
  return { base, projectRoot, homeRoot }
}

const zeusConfig = {
  agents: { architect: { display_name: 'Zeus', slug: 'zeus' } },
}

const zeusConfigWithNickname = {
  agents: { architect: { display_name: 'Zeus', slug: 'zeus' } },
  user_nickname: 'Comandante',
}

test('sem identidade — saída idêntica ao comportamento pré-existente (não-regressão)', () => {
  const withoutIdentity = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project' })[0]
  const withEmptyIdentity = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project', identity: { agents: {} } })[0]
  assert.equal(withoutIdentity.content, withEmptyIdentity.content)
  assert.match(withoutIdentity.content, /^---\nname: trackfw-architect\n/)
})

test('Rota B (subagent/claude) com identidade — name, description e saudação no corpo', () => {
  const plan = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project', identity: zeusConfig })[0]
  assert.match(plan.content, /^---\nname: zeus-tf\n/)
  assert.match(plan.content, /^description: Zeus — /m)
  assert.match(plan.content, /model: opus/)
  assert.match(plan.content, /You are Zeus\.\n\n/)
  assert.doesNotMatch(plan.content, /trackfw-architect/)
})

test('Rota B com apelido — saudação menciona o apelido configurado', () => {
  const plan = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project', identity: zeusConfigWithNickname })[0]
  assert.match(plan.content, /You are Zeus\. Address the user as Comandante\.\n\n/)
})

test('model: do frontmatter é preservado intacto na Rota B', () => {
  const architect = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project', identity: zeusConfig })[0]
  const backend = buildPlans('agents', { targets: ['claude'], items: ['backend'], scope: 'project', identity: { agents: { backend: { display_name: 'Apolo', slug: 'apolo' } } } })[0]
  assert.match(architect.content, /\nmodel: opus\n/)
  assert.match(backend.content, /\nmodel: sonnet\n/)
})

// custom-agent-toml (target codex) emite "model = ..." mapeado a partir do
// tier canônico declarado no frontmatter do asset ("model: opus"/
// "model: sonnet"), posicionado entre "description" e
// "developer_instructions" — paridade com
// internal/integrations/render_test.go:TestRenderCustomAgentTomlEmitsCodexModel
// (ADR ADR-2026-08-14-roteamento-de-model-tier-por-alvo-no-render-de-agentes-
// para-codex-e-cursor).
test('custom-agent-toml (codex) emite model mapeado entre description e developer_instructions', () => {
  const cases = [
    { itemId: 'architect', wantModel: 'model = "gpt-5.4"' },
    { itemId: 'backend', wantModel: 'model = "gpt-5.4-mini"' },
  ]
  for (const { itemId, wantModel } of cases) {
    const plan = buildPlans('agents', { targets: ['codex'], items: [itemId], scope: 'project' })[0]
    const toml = plan.content
    const descIdx = toml.indexOf('description =')
    const modelIdx = toml.indexOf(wantModel)
    const instrIdx = toml.indexOf('developer_instructions =')
    assert.notEqual(descIdx, -1, `description ausente para ${itemId}`)
    assert.notEqual(modelIdx, -1, `${wantModel} ausente para ${itemId}:\n${toml}`)
    assert.notEqual(instrIdx, -1, `developer_instructions ausente para ${itemId}`)
    assert.ok(descIdx < modelIdx && modelIdx < instrIdx, `linha ${wantModel} fora da posição esperada para ${itemId}:\n${toml}`)
  }
})

// Rota B / default (agent-markdown) com target === 'cursor' reescreve a linha
// "model:" do frontmatter mapeando o tier canônico para a sintaxe aceita pela
// Cursor (fonte: cursor.com/docs/subagents, ver ADR ADR-2026-08-14-
// roteamento-de-model-tier-por-alvo-no-render-de-agentes-para-codex-e-cursor)
// — paridade com internal/integrations/render_test.go (ML-3A).
test('cursor reescreve model: opus -> claude-opus-5[effort=high] (architect)', () => {
  const plan = buildPlans('agents', { targets: ['cursor'], items: ['architect'], scope: 'project' })[0]
  assert.match(plan.content, /\nmodel: claude-opus-5\[effort=high\]\n/)
  assert.doesNotMatch(plan.content, /model: opus/)
})

test('cursor reescreve model: sonnet -> composer-2.5[fast=true] (backend)', () => {
  const plan = buildPlans('agents', { targets: ['cursor'], items: ['backend'], scope: 'project' })[0]
  assert.match(plan.content, /\nmodel: composer-2\.5\[fast=true\]\n/)
  assert.doesNotMatch(plan.content, /model: sonnet/)
})

test('gemini e kiro (mesma representação agent-markdown do cursor) permanecem bit-a-bit inalterados', () => {
  const claudePlan = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project' })[0]
  const geminiPlan = buildPlans('agents', { targets: ['gemini'], items: ['architect'], scope: 'project' })[0]
  const kiroPlan = buildPlans('agents', { targets: ['kiro'], items: ['architect'], scope: 'project' })[0]
  assert.match(geminiPlan.content, /\nmodel: opus\n/)
  assert.match(kiroPlan.content, /\nmodel: opus\n/)
  assert.equal(geminiPlan.content, claudePlan.content)
  assert.equal(kiroPlan.content, claudePlan.content)
})

test('cursor com identidade customizada — model reescrito compõe com name/description', () => {
  const plan = buildPlans('agents', { targets: ['cursor'], items: ['architect'], scope: 'project', identity: zeusConfig })[0]
  assert.match(plan.content, /^---\nname: zeus-tf\n/)
  assert.match(plan.content, /^description: Zeus — /m)
  assert.match(plan.content, /\nmodel: claude-opus-5\[effort=high\]\n/)
  assert.doesNotMatch(plan.content, /model: opus/)
})

test('table-driven — name deriva do slug em todas as representações nativas', () => {
  const targets = [
    ['codex', 'custom-agent-toml'],
    ['claude', 'subagent'],
    ['gemini', 'agent-markdown'],
    ['cursor', 'agent-markdown'],
    ['copilot', 'custom-agent'],
    ['windsurf', 'skill'],
    ['amazonq', 'cli-agent-json'],
    ['antigravity', 'agent-directory'],
  ]
  for (const [target, label] of targets) {
    const plan = buildPlans('agents', { targets: [target], items: ['architect'], scope: 'project', identity: zeusConfig })[0]
    if (target === 'codex') {
      assert.match(plan.content, /^name = "zeus_tf"/, label)
    } else if (target === 'amazonq') {
      assert.equal(JSON.parse(plan.content).name, 'zeus-tf', label)
    } else if (target === 'antigravity') {
      assert.match(plan.content, /\nname: zeus-tf\n/, label)
    } else {
      assert.match(plan.content, /\nname: zeus-tf\n/, label)
    }
  }
})

test('SET_ARCH (14 tools) é mantido para architect mesmo com name customizado', () => {
  const plan = buildPlans('agents', { targets: ['antigravity'], items: ['architect'], scope: 'project', identity: zeusConfig })[0]
  assert.match(plan.content, /name: zeus-tf/)
  for (const tool of ['send_message', 'define_subagent', 'invoke_subagent', 'schedule']) {
    assert.match(plan.content, new RegExp(`  - ${tool}`), tool)
  }
  const backendPlan = buildPlans('agents', { targets: ['antigravity'], items: ['backend'], scope: 'project', identity: { agents: { backend: { display_name: 'Apolo', slug: 'apolo' } } } })[0]
  assert.doesNotMatch(backendPlan.content, /define_subagent/)
})

test('skills não recebem identidade', () => {
  const plan = buildPlans('skills', { targets: ['claude'], items: ['governance'], scope: 'project', identity: { agents: { governance: { display_name: 'Zeus', slug: 'zeus' } } } })[0]
  assert.doesNotMatch(plan.content, /You are Zeus/)
  assert.doesNotMatch(plan.content, /zeus-tf/)
})

// --- Testes unitários de rewriteSignatureLine ---

test('rewriteSignatureLine — substitui o nome na última linha de assinatura', () => {
  const source = '---\nname: trackfw-architect\n---\n\n# Corpo\n\nAlgum texto.\n\n— Architect, Principal Software Architect\n'
  const got = rewriteSignatureLine(source, 'Zeus')
  assert.match(got, /— Zeus, Principal Software Architect/)
  assert.doesNotMatch(got, /— Architect, Principal Software Architect/)
})

test('rewriteSignatureLine — sem assinatura: source retornado inalterado', () => {
  const source = '---\nname: trackfw-architect\n---\n\n# Corpo\n\nSem assinatura.\n'
  const got = rewriteSignatureLine(source, 'Zeus')
  assert.equal(got, source)
})

test('rewriteSignatureLine — assinatura dentro do frontmatter não é tocada', () => {
  const source = '---\nname: trackfw-architect\ndescription: — Architect, Principal Software Architect\n---\n\n# Corpo sem assinatura.\n'
  const got = rewriteSignatureLine(source, 'Zeus')
  assert.equal(got, source)
})

test('rewriteSignatureLine — múltiplas linhas candidatas: apenas a última é reescrita', () => {
  const source = '---\nname: trackfw-architect\n---\n\n— Architect, Senior Role\n\nTexto.\n\n— Architect, Principal Software Architect\n'
  const got = rewriteSignatureLine(source, 'Zeus')
  assert.match(got, /— Zeus, Principal Software Architect/)
  assert.match(got, /— Architect, Senior Role/)
  assert.doesNotMatch(got, /— Zeus, Senior Role/)
})

test('rewriteSignatureLine — displayName vazio: source retornado inalterado', () => {
  const source = '---\nname: trackfw-architect\n---\n\n# Corpo\n\n— Architect, Principal Software Architect\n'
  const got = rewriteSignatureLine(source, '')
  assert.equal(got, source)
})

test('Rota B (subagent) com identidade e assinatura inline: nome reescrito', () => {
  const { render } = require('../src/integrations/render')
  const source = '---\nname: trackfw-architect\ndescription: Principal software architect.\nmodel: opus\n---\n\n# Architect\n\nCorpo do agente.\n\n— Architect, Principal Software Architect\n'
  const item = { id: 'architect' }
  const identity = { agents: { architect: { display_name: 'Zeus', slug: 'zeus' } }, user_nickname: 'chefe' }
  const got = render({ kind: 'agents', content: source, capability: { representation: 'subagent' }, item, identity })
  assert.match(got, /— Zeus, Principal Software Architect/)
  assert.doesNotMatch(got, /— Architect, Principal Software Architect/)
})

test('colisão de name gera erro; force contorna', () => {
  const dirs = roots()
  const manager = new IntegrationManager(dirs)
  // architect e backend, ambos mapeados para o mesmo slug "zeus", colidem
  // no mesmo diretório de destino (.claude/agents/).
  const collidingIdentity = {
    agents: {
      architect: { display_name: 'Zeus', slug: 'zeus' },
      backend: { display_name: 'Zeus2', slug: 'zeus' },
    },
  }
  const plans = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project', identity: collidingIdentity })
  const conflicting = buildPlans('agents', { targets: ['claude'], items: ['backend'], scope: 'project', identity: collidingIdentity })

  manager.install(plans)
  assert.throws(() => manager.install(conflicting), /collides with existing file/)
  assert.doesNotThrow(() => manager.install(conflicting, { force: true }))
})
