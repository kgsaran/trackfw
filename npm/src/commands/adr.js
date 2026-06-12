'use strict'

const { Command } = require('commander')
const { input } = require('@inquirer/prompts')
const generators = require('../generators/adr')

const cmd = new Command('adr')
cmd.description('Manage Architecture Decision Records')

cmd.command('new <title>')
  .description('Create a new ADR')
  .action(async (title) => {
    const content = { title }
    // wizard interativo se TTY
    if (process.stdin.isTTY) {
      content.context = await input({ message: 'Context (what motivates this decision)?', default: '' })
      content.decision = await input({ message: 'Decision (what was decided)?', default: '' })
      content.consequences = await input({ message: 'Consequences (positive and negative)?', default: '' })
      content.alternatives = await input({ message: 'Alternatives considered?', default: '' })
    }
    await generators.newADR(content)
  })

cmd.command('list')
  .description('List all ADRs in docs/adr/')
  .action(async () => {
    await generators.listADRs('docs/adr')
  })

module.exports = cmd
