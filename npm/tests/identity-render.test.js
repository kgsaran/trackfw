'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { buildPlans, IntegrationManager } = require('../src/integrations')

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
  assert.match(plan.content, /Você é Zeus\.\n\n/)
  assert.doesNotMatch(plan.content, /trackfw-architect/)
})

test('Rota B com apelido — saudação menciona o apelido configurado', () => {
  const plan = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project', identity: zeusConfigWithNickname })[0]
  assert.match(plan.content, /Você é Zeus\. Trate o usuário como Comandante\.\n\n/)
})

test('model: do frontmatter é preservado intacto na Rota B', () => {
  const architect = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project', identity: zeusConfig })[0]
  const backend = buildPlans('agents', { targets: ['claude'], items: ['backend'], scope: 'project', identity: { agents: { backend: { display_name: 'Apolo', slug: 'apolo' } } } })[0]
  assert.match(architect.content, /\nmodel: opus\n/)
  assert.match(backend.content, /\nmodel: sonnet\n/)
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
  assert.doesNotMatch(plan.content, /Você é Zeus/)
  assert.doesNotMatch(plan.content, /zeus-tf/)
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
