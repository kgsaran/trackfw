'use strict'

const os = require('node:os')
const { Command } = require('commander')
const { catalog, execute, parseSurfaces, buildPlans } = require('../integrations')
const { surfaceFor, readAsset } = require('../integrations/catalog')
const { markdownParts, resolveAgentModel, looksLikeSuspectModelValue } = require('../integrations/render')
const identityStore = require('../identity')
const identityWizard = require('./identity-wizard')
const { t } = require('../i18n')
const { createThirdPartyCommand } = require('./thirdparty')
const configModule = require('../config')
const { homedir } = require('../homedir')

const csv = value => String(value).split(',').map(entry => entry.trim()).filter(Boolean)
const collect = (value, previous) => previous.concat(value)

// forceHelp returns the --force help text for a mutation subcommand. The
// three operations grant --force different powers, and a single shared
// string previously overstated update/uninstall's reach while never
// mentioning install's ability to adopt unmanaged bytes — that ambiguity is
// what sent a user straight into the "Unmanaged artifact does not match a
// trackfw template" error on `update --force` (see unmanagedArtifactError in
// src/integrations/manager.js for the matching remediation). Mirrors
// internal/commands/integrations_flags.go:forceHelp and
// pypi/trackfw/integrations/command.py.
function forceHelp(operation) {
  if (operation === 'install') return 'Replace a modified managed artifact, or adopt/overwrite an unmanaged file already on disk'
  if (operation === 'uninstall') return 'Remove a modified managed artifact'
  return "Replace a modified managed artifact; never adopts unmanaged bytes — use 'install --force' for that"
}

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

// resolveScope decide o escopo de instalação (ADR-2026-07-25-escopo-de-
// instalacao-selecionavel-para-agents-e-skills):
//
//  - `--scope` explícito (`options.scope !== undefined`) é sempre respeitado
//    e nunca dispara prompt — apenas validado contra os valores aceitos. A
//    detecção é por *flag-set* (undefined), nunca por comparação de valor,
//    para não confundir um `--scope project` explícito com o default.
//  - Sem flag e `interactive` desligado (comando `list`, D6): default
//    `global`, nunca lança erro (list nunca é destrutivo).
//  - Sem flag e stdin não é um TTY (D1): default `global` — exceto para
//    `uninstall` (ADR D8), que falha exigindo `--scope` explícito. D1 foi
//    aprovado no enquadramento "onde instalar"; aplicá-lo a uninstall
//    permitiria que um script de CI que hoje limpa `.claude/agents/` do
//    repositório passasse a apagar arquivos do home do usuário.
//  - Sem flag, `interactive` ligado e TTY (D2): pergunta o escopo, com
//    `global` pré-selecionado, para toda operação incluindo uninstall — o
//    usuário vê a escolha antes de destruir. Usa o mesmo mecanismo de prompt
//    (`@inquirer/prompts`) já empregado por `identity-wizard.js`, sem
//    dependência nova.
//
// `interactive` é passado pelo chamador como `false` para `list` — comando de
// leitura que nunca deve bloquear em prompt, mas que ainda assim precisa do
// mesmo default `global` para não reportar deployments divergentes dos que o
// `install` gravou (D6). `operation` identifica install/update/uninstall/list
// para o branch de D8. Espelha
// internal/commands/integrations_flags.go:resolveScope.
async function resolveScope(options, { interactive = true, operation, prompts = require('@inquirer/prompts') } = {}) {
  if (options.scope !== undefined) {
    if (options.scope !== 'project' && options.scope !== 'global') throw new Error(`Unsupported scope: ${options.scope}`)
    return options.scope
  }
  if (!interactive) return 'global'
  if (!process.stdin.isTTY) {
    if (operation === 'uninstall') throw new Error('uninstall requires --scope in non-interactive mode')
    return 'global'
  }
  const { select } = prompts
  return select({
    message: 'Onde instalar os artefatos?',
    default: 'global',
    choices: [
      { name: 'Pasta do usuário (~/.claude) — vale para todos os projetos', value: 'global' },
      { name: 'Este projeto (.claude) — apenas neste repositório', value: 'project' },
    ],
  })
}

// printResolvedDestinations imprime, antes da gravação, o escopo escolhido e
// os caminhos de destino que serão afetados (ADR D5) — nunca em modo --json,
// que já é o canal determinístico consumido por scripts. Recalcula os planos
// via buildPlans (sem side effect) só para enumerar destinos; a gravação em
// si acontece depois, em execute().
function printResolvedDestinations(kind, options) {
  if (options.json) return
  const plans = buildPlans(kind, options)
  const destinations = [...new Set(plans.map(plan => plan.destination))].sort()
  console.log(`Destino (${options.scope}):`)
  for (const destination of destinations) console.log(`  ${destination}`)
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
  // third-party fetch/install (D1): reachable from both `trackfw agents
  // third-party` and `trackfw skills third-party`, under the same
  // two-phase quarantine gate. Mirrors
  // internal/commands/integrations_flags.go registering `third-party` in
  // newIntegrationsLifecycleCmd.
  root.addCommand(createThirdPartyCommand(kind))
  // "models" is agents-only: skills have no model field, and surfacing this
  // under `trackfw skills models` would mislead users. Mirrors the identity
  // flag gate and the Go/Python implementations.
  if (kind === 'agents') root.addCommand(createAgentModelsCommand())
  for (const operation of ['list', 'install', 'uninstall', 'update']) {
    const mutation = operation !== 'list'
    const command = new Command(operation)
      .option('--targets <targets>', 'Comma-separated target CLIs', csv)
      .option('--items <items>', `Comma-separated ${kind} IDs`, csv)
      .option('--scope <scope>', 'Installation scope: project or global (default: global; asks interactively)')
      .option('--surface <target=surface>', 'Surface selection (repeatable)', collect, [])
      .option('--json', 'Print deterministic JSON')
      .option('--force', forceHelp(operation))

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

      // Gate de escopo (ADR D1-D3, D6): resolvido incondicionalmente aqui,
      // antes de qualquer seleção de targets/surfaces ou construção de
      // planos, e independentemente de --targets já ter sido informado —
      // caso contrário o caso mais comum (`agents install --targets claude`)
      // nunca passaria por prompt algum. `list` (mutation === false) nunca
      // pergunta (comando de leitura), apenas adota o default `global`.
      options.scope = await resolveScope(options, { interactive: mutation, operation })

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
        if (presetChanged) identityWizard.applyIdentityPresetFlag(homedir(), options.identityPreset, operation)
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
        const homeRoot = homedir()
        const identityExists = identityWizard.identityFileExists(homeRoot)
        const isTTY = Boolean(process.stdin.isTTY)
        if (identityWizard.shouldPromptIdentity(kind, isTTY, identityExists, forceIdentity)) {
          await identityWizard.runIdentityWizard(catalog, homeRoot)
        } else if (identityExists && !options.json) {
          const existing = identityStore.load(homeRoot)
          console.log(t('identity.inUse', { count: String(Object.keys(existing.agents || {}).length) }))
        }
      }

      // D5: caminhos de destino impressos antes da gravação, apenas para
      // operações de mutação (install/update/uninstall) e fora de --json.
      if (mutation) printResolvedDestinations(kind, options)

      // Ligar onSkip: aviso em stderr por artefato outdated+owned pulado pelo
      // preflight de install (contrato: docs/cli-parity.md, seção "install
      // sobre artefato gerenciado desatualizado — skip, não erro fatal").
      if (mutation) {
        options.onSkip = (_destination, reason) => {
          process.stderr.write(`${reason}\n`)
        }
      }

      const { models: resolvedModels, warning: modelsWarning } = configModule.resolveAgentModels(options.scope, homedir(), process.cwd())
      if (modelsWarning) process.stderr.write(modelsWarning + '\n')
      options.agentModels = resolvedModels
      const output = execute(kind, operation, options)
      console.log(options.json ? JSON.stringify(output) : human(output))
    })
    root.addCommand(command)
  }
  return root
}

// createAgentModelsCommand returns the "models" subcommand of "trackfw agents".
// Lists, for each catalog agent × target pair, the model identifier that
// install/update would write to the generated artifact. Mirrors
// internal/commands/agents_models.go:newAgentModelsCmd and
// pypi/trackfw/integrations/command.py:_run_models.
//
// Column widths must match Go and Python for byte-identical output.
const MODELS_AGENT_WIDTH = 14
const MODELS_TIER_WIDTH = 8
const MODELS_TARGET_WIDTH = 12
const MODELS_NA = '—'

function pad(s, width) {
  return String(s).padEnd(width)
}

function createAgentModelsCommand() {
  const cmd = new Command('models')
    .description('Show the resolved model each agent uses per target')
  cmd.action(() => {
    // AC5 + AC11: read agent_models from the global config (~/.trackfw/trackfw.yaml),
    // not from the cwd singleton. Show origin before the table.
    const { models: agentModels, source: modelsSource } = configModule.loadGlobalAgentModels(homedir(), process.cwd())

    // Source line (AC5): show origin before the table; advisory to stderr when not resolved.
    const sourceLines = {
      global: 'source: ~/.trackfw/trackfw.yaml',
      none: 'source: não configurado',
      project_only: 'source: trackfw.yaml do projeto (não vale para escopo global)',
      global_malformed: 'source: arquivo global malformado',
    }
    const sourceLine = sourceLines[modelsSource] || sourceLines['none']
    process.stdout.write(sourceLine + '\n')
    if (modelsSource === 'none') process.stderr.write(configModule.GLOBAL_AGENT_MODELS_NONE_MESSAGE + '\n')
    else if (modelsSource === 'project_only') process.stderr.write(configModule.GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE + '\n')
    else if (modelsSource === 'global_malformed') process.stderr.write(configModule.MALFORMED_GLOBAL_CONFIG_MESSAGE + '\n')

    // Warnings: emit once per suspect tier, sorted, to stderr.
    const suspectTiers = Object.keys(agentModels)
      .filter(tier => looksLikeSuspectModelValue(agentModels[tier]))
      .sort()
    for (const tier of suspectTiers) {
      process.stderr.write(
        `WARN: agent_models.${tier} = ${JSON.stringify(agentModels[tier])} — not a version string and not a claude- model ID; will be written literally and may produce an invalid model identifier\n`
      )
    }

    const lines = []
    lines.push(`${pad('AGENT', MODELS_AGENT_WIDTH)} ${pad('TIER', MODELS_TIER_WIDTH)} ${pad('TARGET', MODELS_TARGET_WIDTH)} RESOLVED`)

    for (const agent of catalog.agents) {
      const source = readAsset(agent)
      const parts = markdownParts(source)
      const tier = parts.model || 'sonnet'

      for (const target of catalog.targets) {
        const surface = surfaceFor(target, null, 'agents')
        const representation = surface.capabilities.agents.representation
        const { resolved, present } = resolveAgentModel(tier, representation, target.id, agentModels)
        const display = present ? resolved : MODELS_NA
        lines.push(`${pad(agent.id, MODELS_AGENT_WIDTH)} ${pad(tier, MODELS_TIER_WIDTH)} ${pad(target.id, MODELS_TARGET_WIDTH)} ${display}`)
      }
    }

    console.log(lines.join('\n'))
  })
  return cmd
}

module.exports = { createLifecycleCommand, createAgentModelsCommand, csv, human, promptSelection, promptAmbiguousSurfaces, resolveScope }
