'use strict'

const crypto = require('node:crypto')
const fs = require('node:fs')
const path = require('node:path')

// SCHEMA_VERSION é a versão de schema atual do arquivo de configuração de
// identidade. Espelha internal/identity/identity.go:schemaVersion.
const SCHEMA_VERSION = 1

// slugPattern casa um slug válido: segmentos alfanuméricos em minúsculas
// unidos por hífens simples, ex.: "zeus", "meu-agente".
const SLUG_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/

function identityPath(homeDir) {
  return path.join(homeDir, '.trackfw', 'identity.json')
}

// load lê a configuração de identidade de <homeDir>/.trackfw/identity.json.
//
// Se o arquivo não existir, load retorna uma Config vazia sem erro — este é
// o caminho de não-regressão e nunca deve surgir como falha para quem ainda
// não customizou sua identidade. Espelha internal/identity/identity.go:Load.
function load(homeDir) {
  const filename = identityPath(homeDir)
  let data
  try {
    data = fs.readFileSync(filename, 'utf8')
  } catch (err) {
    if (err.code === 'ENOENT') return { schema_version: SCHEMA_VERSION, agents: {} }
    throw new Error(`identity: falha ao ler ${filename}: ${err.message}`)
  }

  let parsed
  try {
    parsed = JSON.parse(data)
  } catch (err) {
    throw new Error(`identity: falha ao decodificar ${filename}: ${err.message}`)
  }
  if (parsed.schema_version !== SCHEMA_VERSION) {
    throw new Error(`identity: versao de schema nao suportada em ${filename}: ${parsed.schema_version} (esperado ${SCHEMA_VERSION})`)
  }
  if (!parsed.agents || typeof parsed.agents !== 'object') parsed.agents = {}
  return parsed
}

// save persiste cfg em <homeDir>/.trackfw/identity.json de forma atômica.
// Espelha internal/identity/identity.go:Save — inclusive a omissão de
// user_nickname vazio e agents vazio, que resulta do comportamento
// omitempty do encoding/json do Go, e a ordenação alfabética das chaves do
// mapa agents, que resulta da ordenação de mapas do encoding/json do Go.
function save(homeDir, cfg) {
  const out = { schema_version: SCHEMA_VERSION }
  if (cfg.user_nickname) out.user_nickname = cfg.user_nickname
  const agents = cfg.agents || {}
  const ids = Object.keys(agents).sort()
  if (ids.length > 0) {
    const sortedAgents = {}
    for (const id of ids) {
      sortedAgents[id] = { display_name: agents[id].display_name, slug: agents[id].slug }
    }
    out.agents = sortedAgents
  }

  const data = `${JSON.stringify(out, null, 2)}\n`
  const filename = identityPath(homeDir)
  atomicWrite(filename, data, 0o600)
}

// atomicWrite grava data em filename de forma atômica: cria um arquivo
// temporário no mesmo diretório, escreve, sincroniza e então renomeia para o
// lugar final. Espelha internal/identity/identity.go:atomicWrite.
function atomicWrite(filename, data, mode) {
  const directory = path.dirname(filename)
  fs.mkdirSync(directory, { recursive: true, mode: 0o700 })
  const temporaryName = path.join(directory, `.trackfw-tmp-${process.pid}-${crypto.randomBytes(6).toString('hex')}`)
  const fd = fs.openSync(temporaryName, 'w', mode)
  try {
    fs.writeSync(fd, data)
    fs.fsyncSync(fd)
  } finally {
    fs.closeSync(fd)
  }
  fs.renameSync(temporaryName, filename)
}

// agentName retorna o display name customizado sufixado com "-tf". Este é o
// único lugar do código onde o sufixo "-tf" é aplicado a um slug. Espelha
// internal/identity/identity.go:AgentName.
function agentName(slug) {
  return `${slug}-tf`
}

// validate verifica cfg quanto à integridade estrutural e referencial:
//   - todo id de agente em cfg.agents deve estar presente em knownIds
//   - display_name não pode ser vazio
//   - slug deve casar com ^[a-z0-9]+(-[a-z0-9]+)*$
//   - slugs devem ser únicos entre os agentes
//   - slug não pode terminar com o sufixo "-tf" (ele é acrescentado
//     automaticamente por agentName)
// Espelha internal/identity/identity.go:Validate.
function validate(cfg, knownIds) {
  const known = new Set(knownIds)
  const seenSlugs = new Map()
  const agents = (cfg && cfg.agents) || {}

  for (const [id, agent] of Object.entries(agents)) {
    if (!known.has(id)) {
      throw new Error(`identity: agente desconhecido "${id}" nao esta na lista de agentes conhecidos`)
    }
    if (!agent.display_name) {
      throw new Error(`identity: display_name vazio para o agente "${id}"`)
    }
    if (!SLUG_PATTERN.test(agent.slug)) {
      throw new Error(`identity: slug invalido "${agent.slug}" para o agente "${id}" (esperado padrao ${SLUG_PATTERN.source})`)
    }
    if (seenSlugs.has(agent.slug)) {
      throw new Error(`identity: slug duplicado "${agent.slug}" entre os agentes "${seenSlugs.get(agent.slug)}" e "${id}"`)
    }
    if (agent.slug.endsWith('-tf')) {
      throw new Error(`identity: slug "${agent.slug}" do agente "${id}" nao deve incluir o sufixo "-tf"; ele e acrescentado automaticamente (use "${agent.slug.slice(0, -3)}" em vez de "${agent.slug}")`)
    }
    seenSlugs.set(agent.slug, id)
  }
}

// lookup retorna a identidade configurada para id, se houver. Espelha
// internal/identity/identity.go:Lookup.
function lookup(cfg, id) {
  const agents = (cfg && cfg.agents) || {}
  return Object.prototype.hasOwnProperty.call(agents, id) ? agents[id] : undefined
}

module.exports = { SCHEMA_VERSION, load, save, agentName, validate, lookup, identityPath }
