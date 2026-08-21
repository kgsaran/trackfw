'use strict'

const os = require('node:os')

const { catalog, items, target, surfaceFor, readAsset, globalGroupPath } = require('./catalog')
const { render } = require('./render')
const { IntegrationManager } = require('./manager')
const { legacyHashes } = require('./legacy')
const identityStore = require('../identity')
const { injectRulesForTool } = require('../generators/init')
const { applyThirdPartyReferences } = require('../thirdparty/references')

function parseSurfaces(values = []) {
  const result = {}
  for (const value of values) {
    const [targetID, surfaceID, extra] = String(value).split('=')
    if (!targetID || !surfaceID || extra !== undefined) throw new Error(`Invalid --surface ${value}: expected target=surface`)
    if (result[targetID]) throw new Error(`Duplicate --surface for target ${targetID}`)
    result[targetID] = surfaceID
  }
  return result
}

function selections(kind, options = {}) {
  const selectedItems = options.items && options.items.length ? options.items : items(kind).map(item => item.id)
  const itemEntries = selectedItems.map(id => {
    const found = items(kind).find(item => item.id === id)
    if (!found) throw new Error(`Unsupported ${kind} item: ${id}`)
    return found
  })
  const targetValues = options.targets && options.targets.length ? options.targets : catalog.targets.map(entry => entry.id)
  const surfaceSelections = parseSurfaces(options.surfaces)
  const scopes = options.scope ? [options.scope] : ['project']
  const targets = []
  for (const targetID of targetValues) {
    const targetEntry = target(targetID)
    const selected = surfaceSelections[targetID]
    const surfaces = options.allSurfaces && !selected
      ? targetEntry.surfaces.filter(entry => entry.capabilities[kind].support_level !== 'unsupported')
      : [surfaceFor(targetEntry, selected, kind)]
    for (const surface of surfaces) targets.push({ target: targetEntry, surface })
  }
  return { itemEntries, targets, scopes }
}

function buildPlans(kind, options = {}) {
  const selected = selections(kind, options)
  const identityConfig = options.identity || { agents: {} }
  const plans = []
  for (const { target: targetEntry, surface } of selected.targets) {
    const capability = surface.capabilities[kind]
    if (capability.support_level === 'unsupported') continue
    for (const scope of selected.scopes) {
      if (!surface.scopes.includes(scope)) continue
      const paths = surface.paths[kind].filter(entry => entry.scope === scope)
      for (const item of selected.itemEntries) {
        for (const installPath of paths) {
          const destination = installPath.path.replace('{{id}}', item.id)
          let content = render({ target: targetEntry.id, kind, item, content: readAsset(item), capability, destination, identity: identityConfig, agentModels: options.agentModels || {} })
          // D5/D9 extension point: reproduce any persisted third-party
          // reference block so regenerating this exact artifact (e.g. a
          // later `trackfw agents update`) settles at state "current"
          // instead of treating the attachment as drift. See the doc
          // comment on applyThirdPartyReferences (../thirdparty/references.js)
          // for why this cannot live inside render() itself. Mirrors
          // internal/integrations/plan.go:BuildPlans.
          if (kind === 'agents' && options.projectRoot) {
            content = applyThirdPartyReferences(options.projectRoot, content, targetEntry.id, item.id)
          }
          const claim = { target: targetEntry.id, surface: surface.id, scope, kind, item: item.id }
          plans.push({
            claim,
            destination,
            content,
            catalogVersion: catalog.version,
            supportLevel: capability.support_level,
            representation: capability.representation,
            legacyHashes: legacyHashes(claim),
            item,
          })
        }
      }
    }
  }
  return plans.sort((a, b) => [a.claim.target, a.claim.surface, a.claim.scope, a.claim.item, a.destination].join('\0').localeCompare([b.claim.target, b.claim.surface, b.claim.scope, b.claim.item, b.destination].join('\0')))
}

function result(kind, plans, statuses) {
  return {
    kind,
    catalog_version: catalog.version,
    items: items(kind).map(({ id, name, description }) => ({ id, name, description })),
    deployments: statuses.map((status, index) => ({
      target: status.claim.target,
      surface: status.claim.surface,
      scope: status.claim.scope,
      item: status.claim.item,
      support_level: status.supportLevel,
      representation: status.representation,
      destination: plans[index].destination,
      state: status.state,
      managed: status.managed,
    })),
  }
}

function execute(kind, operation, options = {}, roots = {}) {
  // A identidade é resolvida do disco antes de buildPlans — pular esta etapa
  // reverteria silenciosamente os nomes customizados dos agentes para os
  // defaults neutros (espelha internal/commands/integrations_flags.go:
  // executeIntegrationMutation/executeIntegrationList).
  const homeRoot = roots.homeRoot || os.homedir()
  const identityConfig = options.identity !== undefined ? options.identity : identityStore.load(homeRoot)
  const manager = new IntegrationManager(roots, { onSkip: options.onSkip })
  // D5/D9: manager.roots.project lets buildPlans reproduce any persisted
  // third-party reference block (see the projectRoot doc comment above the
  // applyThirdPartyReferences call in buildPlans) so a plain `agents update`
  // settles at state "current" after a third-party attachment instead of
  // reporting drift. Mirrors internal/commands/integrations_flags.go passing
  // manager.ProjectRoot to BuildPlans at both call sites.
  const plans = buildPlans(kind, { ...options, identity: identityConfig, projectRoot: options.projectRoot || manager.roots.project })
  let statuses
  if (operation === 'list') statuses = manager.inspect(plans)
  else if (operation === 'install') statuses = manager.install(plans, { force: options.force })
  else if (operation === 'update') statuses = manager.update(plans, { force: options.force })
  else if (operation === 'uninstall') statuses = manager.uninstall(plans, { force: options.force })
  else throw new Error(`Unsupported integration operation: ${operation}`)

  // Auxiliary rules files (GEMINI.md, .github/copilot-instructions.md,
  // .windsurfrules, .amazonq/developer/guidelines.md, etc.) are outside the
  // agents/skills catalog managed by IntegrationManager above — they are a
  // separate, tool-specific mechanism (injectRulesForTool), and this is the
  // canonical catalog-based install path (`trackfw agents|skills install
  // --targets <tool>`). Mirrors internal/commands/integrations_flags.go:
  // executeIntegrationMutation (ML-5E/ML-5G of ROADMAP-2026-07-29-barrier-
  // governanca-e-autoridade-do-orquestrador). Scoped to "install" only,
  // mirroring the one-shot semantics the removed deprecated CLI aliases had.
  // injectRulesForTool no-ops for targets without a rules surface (e.g.
  // antigravity, kiro) and is idempotent for repeated runs.
  if (operation === 'install') {
    const targetValues = options.targets && options.targets.length ? options.targets : catalog.targets.map(entry => entry.id)
    for (const targetID of targetValues) injectRulesForTool(targetID, manager.roots.project)
  }

  return result(kind, plans, statuses)
}

module.exports = { catalog, buildPlans, execute, IntegrationManager, parseSurfaces, globalGroupPath }
