'use strict'

// knownAgentIds retorna a lista canônica de ids de agentes conhecidos pelo
// trackfw, em ordem estável e determinística. Espelha
// internal/identity/preset.go:KnownAgentIDs.
function knownAgentIds() {
  return [
    'architect',
    'backend',
    'frontend',
    'qa',
    'infra',
    'security',
    'dba',
    'ux',
    'code-quality',
    'data',
    'iac',
    'tooling',
  ]
}

// presets contém o catálogo de presets temáticos de identidade. display_name
// e slug são HARDCODED aqui — não derivados chamando slugify em tempo de
// execução — porque essa é uma decisão explícita do ADR: hardcoding evita
// depender do comportamento de normalização Unicode sendo idêntico entre os
// CLIs Go, Node.js e Python. Espelha internal/identity/preset.go:presets.
const PRESETS = {
  greek: {
    architect: { display_name: 'Zeus', slug: 'zeus' },
    backend: { display_name: 'Apolo', slug: 'apolo' },
    frontend: { display_name: 'Afrodite', slug: 'afrodite' },
    qa: { display_name: 'Ártemis', slug: 'artemis' },
    infra: { display_name: 'Ares', slug: 'ares' },
    security: { display_name: 'Hades', slug: 'hades' },
    dba: { display_name: 'Poseidon', slug: 'poseidon' },
    ux: { display_name: 'Atena', slug: 'atena' },
    'code-quality': { display_name: 'Hefesto', slug: 'hefesto' },
    data: { display_name: 'Métis', slug: 'metis' },
    iac: { display_name: 'Dédalo', slug: 'dedalo' },
    tooling: { display_name: 'Prometeu', slug: 'prometeu' },
  },
  norse: {
    architect: { display_name: 'Odin', slug: 'odin' },
    backend: { display_name: 'Thor', slug: 'thor' },
    frontend: { display_name: 'Freya', slug: 'freya' },
    qa: { display_name: 'Heimdall', slug: 'heimdall' },
    infra: { display_name: 'Tyr', slug: 'tyr' },
    security: { display_name: 'Vidar', slug: 'vidar' },
    dba: { display_name: 'Njord', slug: 'njord' },
    ux: { display_name: 'Idun', slug: 'idun' },
    'code-quality': { display_name: 'Bragi', slug: 'bragi' },
    data: { display_name: 'Mimir', slug: 'mimir' },
    iac: { display_name: 'Ivaldi', slug: 'ivaldi' },
    tooling: { display_name: 'Loki', slug: 'loki' },
  },
  potter: {
    architect: { display_name: 'Dumbledore', slug: 'dumbledore' },
    backend: { display_name: 'Snape', slug: 'snape' },
    frontend: { display_name: 'Luna', slug: 'luna' },
    qa: { display_name: 'Moody', slug: 'moody' },
    infra: { display_name: 'Hagrid', slug: 'hagrid' },
    security: { display_name: 'Kingsley', slug: 'kingsley' },
    dba: { display_name: 'Flitwick', slug: 'flitwick' },
    ux: { display_name: 'Tonks', slug: 'tonks' },
    'code-quality': { display_name: 'Hermione', slug: 'hermione' },
    data: { display_name: 'Trelawney', slug: 'trelawney' },
    iac: { display_name: 'Rowena', slug: 'rowena' },
    tooling: { display_name: 'Ollivander', slug: 'ollivander' },
  },
  thrones: {
    architect: { display_name: 'Tyrion', slug: 'tyrion' },
    backend: { display_name: 'Jon', slug: 'jon' },
    frontend: { display_name: 'Sansa', slug: 'sansa' },
    qa: { display_name: 'Arya', slug: 'arya' },
    infra: { display_name: 'Brienne', slug: 'brienne' },
    security: { display_name: 'Varys', slug: 'varys' },
    dba: { display_name: 'Samwell', slug: 'samwell' },
    ux: { display_name: 'Margaery', slug: 'margaery' },
    'code-quality': { display_name: 'Stannis', slug: 'stannis' },
    data: { display_name: 'Bran', slug: 'bran' },
    iac: { display_name: 'Gendry', slug: 'gendry' },
    tooling: { display_name: 'Qyburn', slug: 'qyburn' },
  },
  chaves: {
    architect: { display_name: 'Girafales', slug: 'girafales' },
    backend: { display_name: 'Madruga', slug: 'madruga' },
    frontend: { display_name: 'Chiquinha', slug: 'chiquinha' },
    qa: { display_name: 'Florinda', slug: 'florinda' },
    infra: { display_name: 'Barriga', slug: 'barriga' },
    security: { display_name: 'Quico', slug: 'quico' },
    dba: { display_name: 'Clotilde', slug: 'clotilde' },
    ux: { display_name: 'Popis', slug: 'popis' },
    'code-quality': { display_name: 'Nhonho', slug: 'nhonho' },
    data: { display_name: 'Godinez', slug: 'godinez' },
    iac: { display_name: 'Chaves', slug: 'chaves' },
    tooling: { display_name: 'Chapolin', slug: 'chapolin' },
  },
  pioneers: {
    architect: { display_name: 'Turing', slug: 'turing' },
    backend: { display_name: 'Ritchie', slug: 'ritchie' },
    frontend: { display_name: 'Berners-Lee', slug: 'berners-lee' },
    qa: { display_name: 'Hamilton', slug: 'hamilton' },
    infra: { display_name: 'Torvalds', slug: 'torvalds' },
    security: { display_name: 'Diffie', slug: 'diffie' },
    dba: { display_name: 'Codd', slug: 'codd' },
    ux: { display_name: 'Norman', slug: 'norman' },
    'code-quality': { display_name: 'Knuth', slug: 'knuth' },
    data: { display_name: 'Hopper', slug: 'hopper' },
    iac: { display_name: 'Hashimoto', slug: 'hashimoto' },
    tooling: { display_name: 'McCarthy', slug: 'mccarthy' },
  },
  starwars: {
    architect: { display_name: 'Yoda', slug: 'yoda' },
    backend: { display_name: 'Han', slug: 'han' },
    frontend: { display_name: 'Leia', slug: 'leia' },
    qa: { display_name: 'Ahsoka', slug: 'ahsoka' },
    infra: { display_name: 'Chewbacca', slug: 'chewbacca' },
    security: { display_name: 'Vader', slug: 'vader' },
    dba: { display_name: 'R2-D2', slug: 'r2-d2' },
    ux: { display_name: 'Padmé', slug: 'padme' },
    'code-quality': { display_name: 'Obi-Wan', slug: 'obi-wan' },
    data: { display_name: 'C-3PO', slug: 'c-3po' },
    iac: { display_name: 'Rey', slug: 'rey' },
    tooling: { display_name: 'Babu Frik', slug: 'babu-frik' },
  },
  tolkien: {
    architect: { display_name: 'Gandalf', slug: 'gandalf' },
    backend: { display_name: 'Aragorn', slug: 'aragorn' },
    frontend: { display_name: 'Arwen', slug: 'arwen' },
    qa: { display_name: 'Legolas', slug: 'legolas' },
    infra: { display_name: 'Gimli', slug: 'gimli' },
    security: { display_name: 'Boromir', slug: 'boromir' },
    dba: { display_name: 'Elrond', slug: 'elrond' },
    ux: { display_name: 'Galadriel', slug: 'galadriel' },
    'code-quality': { display_name: 'Faramir', slug: 'faramir' },
    data: { display_name: 'Bilbo', slug: 'bilbo' },
    iac: { display_name: 'Aulë', slug: 'aule' },
    tooling: { display_name: 'Celebrimbor', slug: 'celebrimbor' },
  },
  turma: {
    architect: { display_name: 'Franjinha', slug: 'franjinha' },
    backend: { display_name: 'Cebolinha', slug: 'cebolinha' },
    frontend: { display_name: 'Magali', slug: 'magali' },
    qa: { display_name: 'Mônica', slug: 'monica' },
    infra: { display_name: 'Cascão', slug: 'cascao' },
    security: { display_name: 'Bidu', slug: 'bidu' },
    dba: { display_name: 'Marocas', slug: 'marocas' },
    ux: { display_name: 'Anjinho', slug: 'anjinho' },
    'code-quality': { display_name: 'Titi', slug: 'titi' },
    data: { display_name: 'Chico', slug: 'chico' },
    iac: { display_name: 'Piteco', slug: 'piteco' },
    tooling: { display_name: 'Nimbus', slug: 'nimbus' },
  },
  egyptian: {
    architect: { display_name: 'Thoth', slug: 'thoth' },
    backend: { display_name: 'Rá', slug: 'ra' },
    frontend: { display_name: 'Ísis', slug: 'isis' },
    qa: { display_name: 'Hórus', slug: 'horus' },
    infra: { display_name: 'Ptah', slug: 'ptah' },
    security: { display_name: 'Anúbis', slug: 'anubis' },
    dba: { display_name: 'Seshat', slug: 'seshat' },
    ux: { display_name: 'Bastet', slug: 'bastet' },
    'code-quality': { display_name: 'Maat', slug: 'maat' },
    data: { display_name: 'Osíris', slug: 'osiris' },
    iac: { display_name: 'Imhotep', slug: 'imhotep' },
    tooling: { display_name: 'Khnum', slug: 'khnum' },
  },
}

// presetOrder é a ordem canônica e estável em que os nomes de presets são
// listados (ex.: por presetNames e em mensagens de erro).
const PRESET_ORDER = ['greek', 'norse', 'potter', 'thrones', 'chaves', 'pioneers', 'starwars', 'tolkien', 'turma', 'egyptian']

// preset retorna a Config de identidade para o nome de preset temático dado.
// Se name não for um preset conhecido, lança erro listando os nomes válidos.
//
// A Config retornada é sempre uma cópia: mutá-la não afeta a tabela de
// presets no nível do módulo nem qualquer outra Config retornada por uma
// chamada anterior a preset().
function preset(name) {
  const agents = PRESETS[name]
  if (!agents) {
    throw new Error(`identity: preset desconhecido "${name}" (validos: ${presetNames().join(', ')})`)
  }
  const copied = {}
  for (const [id, agent] of Object.entries(agents)) {
    copied[id] = { display_name: agent.display_name, slug: agent.slug }
  }
  return { schema_version: 1, agents: copied }
}

// presetNames retorna os nomes de todos os presets conhecidos, na ordem
// estável: greek, norse, potter, thrones, chaves, pioneers, starwars,
// tolkien, turma, egyptian.
function presetNames() {
  return [...PRESET_ORDER]
}

// Verificação em tempo de carga do módulo: presetOrder e PRESETS devem estar
// em sincronia — guarda defensiva contra drift silencioso entre o mapa e a
// ordem documentada (espelha o init() de internal/identity/preset.go).
;(function assertPresetOrderInSync() {
  if (PRESET_ORDER.length !== Object.keys(PRESETS).length) {
    throw new Error('identity: presetOrder e PRESETS fora de sincronia')
  }
  const seen = new Set()
  for (const name of PRESET_ORDER) {
    if (!PRESETS[name]) {
      throw new Error(`identity: preset "${name}" listado em presetOrder mas ausente de PRESETS`)
    }
    seen.add(name)
  }
  if (seen.size !== PRESET_ORDER.length) {
    throw new Error('identity: presetOrder contem nomes duplicados')
  }
})()

module.exports = { knownAgentIds, preset, presetNames }
