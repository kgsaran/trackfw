'use strict'

const { Command } = require('commander')
const { catalog, execute, parseSurfaces } = require('../integrations')

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
    const command = new Command(operation)
      .option('--targets <targets>', 'Comma-separated target CLIs', csv)
      .option('--items <items>', `Comma-separated ${kind} IDs`, csv)
      .option('--scope <scope>', 'Installation scope: project or global', 'project')
      .option('--surface <target=surface>', 'Surface selection (repeatable)', collect, [])
      .option('--json', 'Print deterministic JSON')
      .option('--force', 'Replace or remove modified artifacts')
    command.action(async options => {
      options.surfaces = options.surface || []
      if (options.scope !== 'project' && options.scope !== 'global') throw new Error(`Unsupported scope: ${options.scope}`)
      const mutation = operation !== 'list'
      if (mutation && (!options.targets || !options.targets.length)) {
        if (!process.stdin.isTTY) throw new Error(`${operation} requires --targets in non-interactive mode`)
        await promptSelection(kind, options)
      }
      if (mutation && process.stdin.isTTY) await promptAmbiguousSurfaces(kind, options)
      options.allSurfaces = operation === 'list'
      const output = execute(kind, operation, options)
      console.log(options.json ? JSON.stringify(output) : human(output))
    })
    root.addCommand(command)
  }
  return root
}

module.exports = { createLifecycleCommand, csv, human, promptSelection, promptAmbiguousSurfaces }
