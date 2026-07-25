'use strict'

const fs = require('node:fs')
const path = require('node:path')

const identityStore = require('../identity')
const { t } = require('../i18n')

// identityFileExists reporta se ~/.trackfw/identity.json já existe, sem
// depender de identityStore.load (que retorna config vazia para "ausente").
// Espelha internal/commands/init.go:identityFileExists.
function identityFileExists(home) {
  return fs.existsSync(path.join(home, '.trackfw', 'identity.json'))
}

// resolveIdentityPreset traduz o valor da flag --identity-preset para uma
// Config a persistir. "none" e "neutral" significam "não gravar nada" — o
// chamador não deve criar ~/.trackfw/identity.json para esses valores. Um
// valor desconhecido é sempre um erro, listando os valores aceitos. Espelha
// internal/commands/init.go:resolveIdentityPreset.
function resolveIdentityPreset(value) {
  if (value === 'none' || value === 'neutral') return { cfg: null, shouldSave: false }
  let cfg
  try {
    cfg = identityStore.preset(value)
  } catch {
    const valid = ['none', 'neutral', ...identityStore.presetNames()]
    throw new Error(`identity-preset invalido "${value}" (validos: ${valid.join(', ')})`)
  }
  return { cfg, shouldSave: true }
}

// applyIdentityPresetFlag resolve e persiste --identity-preset para
// `agents install|update|uninstall`, reusando resolveIdentityPreset para que
// ambos os comandos aceitem exatamente os mesmos nomes de preset e rejeitem
// valores inválidos com o mesmo formato de erro. Espelha
// internal/commands/integrations_flags.go:applyIdentityPresetFlag.
function applyIdentityPresetFlag(home, presetValue, operation) {
  const { cfg, shouldSave } = resolveIdentityPreset(presetValue)
  if (!shouldSave) return
  try {
    identityStore.validate(cfg, identityStore.knownAgentIds())
  } catch (err) {
    throw new Error(`${operation}: identidade invalida: ${err.message}`)
  }
  try {
    identityStore.save(home, cfg)
  } catch (err) {
    throw new Error(`${operation}: falha ao gravar identidade: ${err.message}`)
  }
}

// shouldPromptIdentity decide se `trackfw agents install` (e as operações
// update/uninstall, que compartilham o mesmo fluxo) deve mostrar o wizard
// interativo de identidade. Todas as condições devem valer simultaneamente
// (ADR D2):
//
//  1. kind é "agents" — skills não têm identidade e nunca devem perguntar.
//  2. stdin é um TTY — nunca bloquear uma execução não-interativa.
//  3. ou não há identidade configurada ainda, ou o usuário pediu
//     explicitamente para reconfigurá-la via --identity.
//
// Com uma identidade já configurada e sem a flag --identity, o wizard NÃO
// deve reaparecer — um wizard que aparece a cada install vira incentivo a
// automatizar o "pular", o que esvazia a feature. Espelha
// internal/commands/identity_wizard.go:shouldPromptIdentity.
function shouldPromptIdentity(kind, isTTY, identityExists, forceFlag) {
  if (kind !== 'agents') return false
  if (!isTTY) return false
  return !identityExists || Boolean(forceFlag)
}

// identityPresetOptions é a lista fixa de presets temáticos de identidade
// oferecidos pelo wizard, compartilhada literalmente entre init e agents
// install para que nenhum consumidor divirja do outro. Espelha
// internal/commands/identity_wizard.go:identityPresetOptions.
function identityPresetOptions() {
  return [
    { name: 'Panteão grego (Zeus, Apolo, Afrodite...)', value: 'greek' },
    { name: 'Mitologia nórdica (Odin, Thor, Freya...)', value: 'norse' },
    { name: 'Pioneiros da computação (Turing, Codd, Knuth...)', value: 'pioneers' },
    { name: 'Harry Potter (Dumbledore, Snape, Luna...)', value: 'potter' },
    { name: 'Game of Thrones (Tyrion, Jon, Arya...)', value: 'thrones' },
    { name: 'Senhor dos Anéis (Gandalf, Aragorn, Arwen...)', value: 'tolkien' },
    { name: 'Star Wars (Yoda, Leia, Vader...)', value: 'starwars' },
    { name: 'Chaves (Girafales, Madruga, Chiquinha...)', value: 'chaves' },
    { name: 'Turma da Mônica (Franjinha, Cebolinha, Mônica...)', value: 'turma' },
    { name: 'Panteão egípcio (Thoth, Ísis, Anúbis...)', value: 'egyptian' },
    { name: 'Personalizar um a um', value: 'custom' },
    { name: 'Nomes neutros (padrão)', value: 'neutral' },
  ]
}

// catalogAgentItem busca o item "agents" do catálogo pelo id. Todo id vindo
// de identity.knownAgentIds() deve existir no catálogo embedado — se não
// existir, é erro de programação: falha alto em vez de rotular
// silenciosamente errado. Espelha o panic em
// internal/commands/identity_wizard.go:buildCustomIdentityGroup.
function catalogAgentItem(catalog, id) {
  const found = (catalog.agents || []).find(entry => entry.id === id)
  if (!found) {
    throw new Error(`identity wizard: agent id "${id}" from identity.knownAgentIds() has no entry in the agents catalog`)
  }
  return found
}

// buildIdentityConfig constrói a Config em memória para o caminho escolhido
// no wizard, sem tocar em disco. Espelha
// internal/commands/identity_wizard.go:buildIdentityConfig.
function buildIdentityConfig(identitySelect, knownAgentIds, customDisplayNames, userNickname) {
  let cfg
  if (identitySelect === 'custom') {
    const agents = {}
    knownAgentIds.forEach((id, index) => {
      let slug
      try {
        slug = identityStore.slugify(customDisplayNames[index])
      } catch (err) {
        throw new Error(`identidade customizada invalida para "${id}": ${err.message}`)
      }
      agents[id] = { display_name: customDisplayNames[index], slug }
    })
    cfg = { agents }
  } else {
    cfg = identityStore.preset(identitySelect)
  }
  cfg.user_nickname = userNickname
  return cfg
}

// promptCustomNames pede um nome de exibição para cada agente conhecido,
// rotulado com a especialidade do catálogo (Item.name + Item.description) —
// nunca com o id técnico cru (ADR D4). Espelha
// internal/commands/identity_wizard.go:buildCustomIdentityGroup.
async function promptCustomNames(catalog, knownAgentIds, prompts) {
  const { input } = prompts
  const helperText = t('init.prompt.identityCustomName')
  const customDisplayNames = []
  for (const id of knownAgentIds) {
    const item = catalogAgentItem(catalog, id)
    const label = `${item.name} — ${item.description}`
    // eslint-disable-next-line no-await-in-loop
    const value = await input({
      message: `${label}\n  ${helperText}`,
      validate: candidate => {
        let slug
        try {
          slug = identityStore.slugify(candidate)
        } catch (err) {
          return err.message
        }
        for (let index = 0; index < customDisplayNames.length; index += 1) {
          let otherSlug
          try {
            otherSlug = identityStore.slugify(customDisplayNames[index])
          } catch {
            continue
          }
          if (otherSlug === slug) return `slug "${slug}" duplicado com o agente "${knownAgentIds[index]}"`
        }
        return true
      },
    })
    customDisplayNames.push(value)
  }
  return customDisplayNames
}

// confirmIdentitySelection exibe o mapeamento especialidade → nome de
// exibição (ADR D3) e pede confirmação. O mapeamento é impresso com
// console.log simples (não por um campo de prompt) para que o alinhamento
// de colunas siga exatamente o exemplo do ADR. Espelha
// internal/commands/identity_wizard.go:confirmIdentitySelection.
async function confirmIdentitySelection(catalog, knownAgentIds, cfg, prompts) {
  const { confirm } = prompts
  const nicknameLabel = t('identity.wizard.nicknameRowLabel')
  let labelWidth = nicknameLabel.length
  const labels = knownAgentIds.map(id => {
    const item = catalogAgentItem(catalog, id)
    if (item.description.length > labelWidth) labelWidth = item.description.length
    return item.description
  })

  console.log()
  console.log(t('identity.wizard.confirmHeader'))
  knownAgentIds.forEach((id, index) => {
    const agent = cfg.agents[id]
    console.log(`  ${labels[index].padEnd(labelWidth)}   →  ${agent.display_name}`)
  })
  if (cfg.user_nickname) {
    console.log(`  ${nicknameLabel.padEnd(labelWidth)}   ${cfg.user_nickname}`)
  }
  console.log()

  return confirm({ message: t('identity.wizard.confirmQuestion') })
}

// runIdentityWizard é o único wizard interativo de identidade, consumido
// tanto por `trackfw init` quanto por `trackfw agents install`
// (ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install, D1).
//
// Mostra, em ordem: seleção de preset/custom, nomes livres em texto (só no
// modo custom), o apelido do usuário e uma tela de confirmação (D3, D6). Se
// o usuário recusar a confirmação, o wizard volta à seleção de preset sem
// persistir nada — nunca retorna tendo gravado uma config parcial ou
// recusada em disco.
//
// Retorna [cfg, persisted]. "neutral" (ou deixar o select vazio) é o
// caminho "não gravar nada" e retorna [{}, false].
async function runIdentityWizard(catalog, home, prompts = require('@inquirer/prompts')) {
  const { select, input } = prompts
  const knownAgentIds = identityStore.knownAgentIds()

  const titleIdentityPreset = t('init.prompt.identityPreset')
  const titleIdentityNickname = t('init.prompt.identityNickname')

  for (;;) {
    const identitySelect = await select({
      message: titleIdentityPreset,
      choices: identityPresetOptions(),
    })

    if (identitySelect === '' || identitySelect === 'neutral') {
      return [{}, false]
    }

    let customDisplayNames = []
    if (identitySelect === 'custom') {
      customDisplayNames = await promptCustomNames(catalog, knownAgentIds, prompts)
    }

    const userNickname = await input({ message: titleIdentityNickname, default: '' })

    const cfg = buildIdentityConfig(identitySelect, knownAgentIds, customDisplayNames, userNickname)
    identityStore.validate(cfg, knownAgentIds)

    // eslint-disable-next-line no-await-in-loop
    const confirmed = await confirmIdentitySelection(catalog, knownAgentIds, cfg, prompts)
    if (!confirmed) {
      // D3: recusar volta à seleção de preset sem persistir absolutamente nada.
      continue
    }

    identityStore.save(home, cfg)
    return [cfg, true]
  }
}

module.exports = {
  identityFileExists,
  resolveIdentityPreset,
  applyIdentityPresetFlag,
  shouldPromptIdentity,
  identityPresetOptions,
  buildIdentityConfig,
  confirmIdentitySelection,
  runIdentityWizard,
}
