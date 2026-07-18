'use strict'

const { Command } = require('commander')
const { catalog, execute } = require('../integrations')

function csv(value) {
  return String(value).split(',').map(entry => entry.trim()).filter(Boolean)
}

function human(result) {
  const lines = [`${result.kind} (catalog ${result.catalog_version})`]
  for (const deployment of result.deployments) {
    const managed = deployment.managed ? 'managed' : 'unmanaged'
    lines.push(`${deployment.target}=${deployment.surface} ${deployment.scope} ${deployment.item}: ${deployment.state} (${managed}, ${deployment.support_level})`)
  }
  return lines.join('\n')
}

async function chooseTargets(kind) {
  const { checkbox } = require('@inquirer/prompts')
  return checkbox({
    message: `Select CLIs for ${kind}`,
    choices: catalog.targets.map(target => ({ name: target.name, value: target.id })),
    required: true,
  })
}

function createLifecycleCommand(kind) {
  const root = new Command(kind).description(`Manage trackfw ${kind}`)
  for (const operation of ['list', 'install', 'uninstall', 'update']) {
    const command = new Command(operation)
      .option('--targets <targets>', 'Comma-separated target selectors (target or target=surface)', csv)
      .option('--items <items>', `Comma-separated ${kind} IDs`, csv)
      .option('--scope <scope>', 'Installation scope: project or global', 'project')
      .option('--json', 'Print deterministic JSON')
      .option('--force', 'Overwrite or remove modified managed files')
    command.action(async options => {
      if (options.scope !== 'project' && options.scope !== 'global') throw new Error(`Unsupported scope: ${options.scope}`)
      if (operation !== 'list' && (!options.targets || !options.targets.length)) {
        if (!process.stdin.isTTY) throw new Error(`--targets is required for non-interactive ${operation}`)
        options.targets = await chooseTargets(kind)
      }
      const output = execute(kind, operation, options)
      console.log(options.json ? JSON.stringify(output) : human(output))
    })
    root.addCommand(command)
  }
  return root
}

module.exports = { createLifecycleCommand, csv, human }
