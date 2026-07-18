'use strict'

const { catalog, items, target, surfaceFor, readAsset } = require('./catalog')
const { render } = require('./render')
const { IntegrationManager } = require('./manager')

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
          const content = render({ target: targetEntry.id, kind, item, content: readAsset(item), capability, destination })
          plans.push({
            claim: { target: targetEntry.id, surface: surface.id, scope, kind, item: item.id },
            destination,
            content,
            catalogVersion: catalog.version,
            supportLevel: capability.support_level,
            representation: capability.representation,
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
  const plans = buildPlans(kind, options)
  const manager = new IntegrationManager(roots)
  let statuses
  if (operation === 'list') statuses = manager.inspect(plans)
  else if (operation === 'install') statuses = manager.install(plans, { force: options.force })
  else if (operation === 'update') statuses = manager.update(plans, { force: options.force })
  else if (operation === 'uninstall') statuses = manager.uninstall(plans, { force: options.force })
  else throw new Error(`Unsupported integration operation: ${operation}`)
  return result(kind, plans, statuses)
}

module.exports = { catalog, buildPlans, execute, IntegrationManager, parseSurfaces }
