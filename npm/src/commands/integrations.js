'use strict'

const os = require('node:os')
const { Command } = require('commander')
const { catalog, execute, parseSurfaces } = require('../integrations')
const identityStore = require('../identity')
const identityWizard = require('./identity-wizard')
const { t } = require('../i18n')

const csv = value => String(value).split(',').map(entry => entry.trim()).filter(Boolean)
const collect = (value, previous) => previous.concat(value)

function human(result) {
  const lines = [`Available ${result.kind} (catalog ${result.catalog_version}):`]
  for (const item of result.items) lines.push(`  ${item.id.padEnd(14)} ${item.name} — ${item.description}`)
  lines.push('', 'Deployments:')
  for (const deployment of result.deployments) {
    const managed = deployment.managed ? 'managed' : 'unmanaged'
    lines.push(`  ${deployment.target.padEnd(12)} ${deployment.surface.padEnd(12)} ${deployment.item.padEnd(14)} ${deployment.state.padEnd(13)} ${deployment.destination} (${managed})`)
  }
  return lines.join('\n')
}

async function promptSelection(kind, options, prompts = require('@inquirer/prompts')) {
  const { checkbox } = prompts
  options.targets = await checkbox({ message: 'Target CLIs', choices: catalog.targets.map(target => ({ name: target.name, value: target.id })), required: true })
  options.items = await checkbox({ message: `${kind} to manage`, choices: catalog[kind].map(item => ({ name: item.name, value: item.id })), required: true })
}

async function promptAmbiguousSurfaces(kind, options, prompts = require('@inquirer/prompts')) {
  const { select } = prompts
  const selected = parseSurfaces(options.surfaces)
  for (const targetID of options.targets || []) {
    if (selected[targetID]) continue
    const target = catalog.targets.find(entry => entry.id === targetID)
    const eligible = target.surfaces.filter(surface => !['legacy', 'unsupported'].includes(surface.capabilities[kind].support_level))
    if (eligible.length <= 1) continue
    const surface = await select({ message: `Surface for ${target.name}`, choices: eligible.map(entry => ({ name: entry.name, value: entry.id })) })
    options.surfaces.push(`${targetID}=${surface}`)
  }
}

function createLifecycleCommand(kind) {
  const root = new Command(kind).description(`Manage trackfw ${kind}`)
  for (const operation of ['list', 'install', 'uninstall', 'update']) {
    const mutation = operation !== 'list'
    const command = new Command(operation)
      .option('--targets <targets>', 'Comma-separated target CLIs', csv)
      .option('--items <items>', `Comma-separated ${kind} IDs`, csv)
      .option('--scope <scope>', 'Installation scope: project or global', 'project')
      .option('--surface <target=surface>', 'Surface selection (repeatable)', collect, [])
      .option('--json', 'Print deterministic JSON')
      .option('--force', 'Replace or remove modified artifacts')

    // Flags de identidade são exclusivas de agents (ADR D5): skills não têm
    // identidade, e createLifecycleCommand é compartilhado entre `agents` e
    // `skills` — sem este filtro por kind, `trackfw skills install
    // --identity` aceitaria silenciosamente uma flag sem nenhum efeito.
    // Espelha internal/commands/integrations_flags.go:addIntegrationFlags.
    if (mutation && kind === 'agents') {
      command
        .option('--identity', 'Reconfigure agent identity even if ~/.trackfw/identity.json already exists')
        .option('--identity-preset <preset>', `Agent identity preset (non-interactive): none, neutral, ${identityStore.presetNames().join(', ')}`)
    }

    command.action(async (options, cmd) => {
      options.surfaces = options.surface || []
      if (options.scope !== 'project' && options.scope !== 'global') throw new Error(`Unsupported scope: ${options.scope}`)

      // O booleano de --identity nunca deve chegar a execute()/buildPlans()
      // sob a chave "identity" — essa chave ali é reservada para uma Config
      // de identidade já resolvida (ver src/integrations/index.js:execute).
      // Colidir os dois faria "agents install --identity" renderizar uma
      // identidade booleana nos artefatos em vez da Config real.
      const forceIdentity = options.identity === true
      delete options.identity

      // --identity-preset é validado e persistido incondicionalmente, acima
      // de qualquer ramo dependente de TTY abaixo — isso é o que faz um
      // --identity-preset inválido falhar alto em CI em vez de
      // silenciosamente não fazer nada. Espelha init.js e
      // internal/commands/integrations_flags.go:executeIntegrationMutation.
      let presetChanged = false
      if (mutation && kind === 'agents') {
        presetChanged = cmd.getOptionValueSource('identityPreset') === 'cli'
        if (presetChanged) identityWizard.applyIdentityPresetFlag(os.homedir(), options.identityPreset, operation)
      }

      if (mutation && (!options.targets || !options.targets.length)) {
        if (!process.stdin.isTTY) throw new Error(`${operation} requires --targets in non-interactive mode`)
        await promptSelection(kind, options)
      }
      if (mutation && process.stdin.isTTY) await promptAmbiguousSurfaces(kind, options)
      options.allSurfaces = operation === 'list'

      // Disparo do wizard de identidade (ADR D2): mostrado somente quando o
      // caminho da flag acima ainda não resolveu a identidade desta
      // execução, e somente para agents (nunca skills, D5). Roda depois da
      // seleção de alvo/superfície e antes de execute() para que a
      // identidade recém-gravada pelo wizard seja a que é renderizada nos
      // planos abaixo. Espelha
      // internal/commands/integrations_flags.go:executeIntegrationMutation.
      if (mutation && kind === 'agents' && !presetChanged) {
        const homeRoot = os.homedir()
        const identityExists = identityWizard.identityFileExists(homeRoot)
        const isTTY = Boolean(process.stdin.isTTY)
        if (identityWizard.shouldPromptIdentity(kind, isTTY, identityExists, forceIdentity)) {
          await identityWizard.runIdentityWizard(catalog, homeRoot)
        } else if (identityExists && !options.json) {
          const existing = identityStore.load(homeRoot)
          console.log(t('identity.inUse', { count: String(Object.keys(existing.agents || {}).length) }))
        }
      }

      const output = execute(kind, operation, options)
      console.log(options.json ? JSON.stringify(output) : human(output))
    })
    root.addCommand(command)
  }
  return root
}

module.exports = { createLifecycleCommand, csv, human, promptSelection, promptAmbiguousSurfaces }
