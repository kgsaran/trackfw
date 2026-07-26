'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const identity = require('../src/identity')

const fixture = JSON.parse(fs.readFileSync(path.join(__dirname, 'fixtures/slug_vectors.json'), 'utf8'))

function tmpHome() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-identity-'))
}

test('slugify — vetores de paridade compartilhados com o Go', () => {
  for (const testCase of fixture.cases) {
    if (testCase.error) {
      assert.throws(() => identity.slugify(testCase.input), undefined, `esperava erro para ${JSON.stringify(testCase.input)}`)
    } else {
      assert.equal(identity.slugify(testCase.input), testCase.expect, `input=${JSON.stringify(testCase.input)}`)
    }
  }
})

test('load — arquivo ausente retorna config vazia sem erro', () => {
  const home = tmpHome()
  const cfg = identity.load(home)
  assert.deepEqual(cfg.agents, {})
  assert.equal(cfg.schema_version, 1)
})

test('load — schema_version diferente de 1 é erro', () => {
  const home = tmpHome()
  fs.mkdirSync(path.join(home, '.trackfw'), { recursive: true })
  fs.writeFileSync(path.join(home, '.trackfw', 'identity.json'), JSON.stringify({ schema_version: 2, agents: {} }))
  assert.throws(() => identity.load(home), /versao de schema nao suportada/)
})

test('save/load — round trip preserva agents e user_nickname', () => {
  const home = tmpHome()
  const cfg = { agents: { architect: { display_name: 'Zeus', slug: 'zeus' } }, user_nickname: 'Chefe' }
  identity.save(home, cfg)
  const reloaded = identity.load(home)
  assert.deepEqual(reloaded.agents, cfg.agents)
  assert.equal(reloaded.user_nickname, 'Chefe')

  const file = identity.identityPath(home)
  assert.equal(fs.statSync(file).mode & 0o777, 0o600)
  assert.equal(fs.readFileSync(file, 'utf8').endsWith('\n'), true)
})

test('save — omite agents vazio e user_nickname vazio, ordena chaves alfabeticamente', () => {
  const home = tmpHome()
  identity.save(home, { agents: {} })
  const raw = fs.readFileSync(identity.identityPath(home), 'utf8')
  assert.equal(raw, '{\n  "schema_version": 1\n}\n')

  const home2 = tmpHome()
  identity.save(home2, {
    agents: {
      qa: { display_name: 'Ártemis', slug: 'artemis' },
      architect: { display_name: 'Zeus', slug: 'zeus' },
    },
  })
  const parsed = JSON.parse(fs.readFileSync(identity.identityPath(home2), 'utf8'))
  assert.deepEqual(Object.keys(parsed.agents), ['architect', 'qa'])
  assert.deepEqual(Object.keys(parsed), ['schema_version', 'agents'])
})

test('agentName — sufixa slug com -tf', () => {
  assert.equal(identity.agentName('zeus'), 'zeus-tf')
})

test('validate — agente desconhecido é erro', () => {
  const cfg = { agents: { unknown: { display_name: 'X', slug: 'x' } } }
  assert.throws(() => identity.validate(cfg, identity.knownAgentIds()), /agente desconhecido/)
})

test('validate — display_name vazio é erro', () => {
  const cfg = { agents: { architect: { display_name: '', slug: 'zeus' } } }
  assert.throws(() => identity.validate(cfg, identity.knownAgentIds()), /display_name vazio/)
})

test('validate — slug fora do padrão é erro', () => {
  const cfg = { agents: { architect: { display_name: 'Zeus', slug: 'Zeus!' } } }
  assert.throws(() => identity.validate(cfg, identity.knownAgentIds()), /slug invalido/)
})

test('validate — slugs duplicados entre agentes é erro', () => {
  const cfg = {
    agents: {
      architect: { display_name: 'Zeus', slug: 'zeus' },
      backend: { display_name: 'Zeus2', slug: 'zeus' },
    },
  }
  assert.throws(() => identity.validate(cfg, identity.knownAgentIds()), /slug duplicado/)
})

test('validate — slug com sufixo -tf é erro e a mensagem cita id e slug', () => {
  const cfg = { agents: { architect: { display_name: 'Zeus', slug: 'zeus-tf' } } }
  assert.throws(
    () => identity.validate(cfg, identity.knownAgentIds()),
    (err) => {
      assert.match(err.message, /sufixo "-tf"/)
      assert.match(err.message, /zeus-tf/)
      assert.match(err.message, /architect/)
      return true
    },
  )
})

test('validate — slugs que não terminam em -tf continuam válidos', () => {
  for (const slug of ['zeus', 'tf', 'meu-tf-agente']) {
    const cfg = { agents: { architect: { display_name: 'Zeus', slug } } }
    assert.doesNotThrow(() => identity.validate(cfg, identity.knownAgentIds()))
  }
})

test('validate — config válida não lança', () => {
  const cfg = { agents: { architect: { display_name: 'Zeus', slug: 'zeus' } } }
  assert.doesNotThrow(() => identity.validate(cfg, identity.knownAgentIds()))
})

test('preset — 10 presets na ordem canônica, cada um com os 12 ids conhecidos', () => {
  const order = ['greek', 'norse', 'potter', 'thrones', 'chaves', 'pioneers', 'starwars', 'tolkien', 'turma', 'egyptian']
  assert.deepEqual(identity.presetNames(), order)
  for (const name of order) {
    const cfg = identity.preset(name)
    assert.deepEqual(Object.keys(cfg.agents).sort(), identity.knownAgentIds().slice().sort())
    assert.doesNotThrow(() => identity.validate(cfg, identity.knownAgentIds()))
  }
})

test('preset — retorna cópia; mutar o retorno não afeta chamadas futuras', () => {
  const first = identity.preset('greek')
  first.agents.architect.display_name = 'Mutado'
  const second = identity.preset('greek')
  assert.equal(second.agents.architect.display_name, 'Zeus')
})

test('preset — nome desconhecido lança erro listando os válidos', () => {
  assert.throws(() => identity.preset('nao-existe'), /preset desconhecido.*greek/)
})

test('knownAgentIds — retorna os 12 ids na ordem estável', () => {
  assert.deepEqual(identity.knownAgentIds(), [
    'architect', 'backend', 'frontend', 'qa', 'infra', 'security', 'dba', 'ux', 'code-quality', 'data', 'iac', 'tooling',
  ])
})
