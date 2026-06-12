'use strict'

const { Command } = require('commander')
const { input } = require('@inquirer/prompts')
const generators = require('../generators/adr')
const { t } = require('../i18n')

const cmd = new Command('adr')
cmd.description(t('adr.description'))

cmd.command('new <title>')
  .description(t('adr.new.description'))
  .action(async (title) => {
    const content = { title }
    // wizard interativo se TTY
    if (process.stdin.isTTY) {
      content.context = await input({ message: t('adr.new.prompt.context'), default: '' })
      content.decision = await input({ message: t('adr.new.prompt.decision'), default: '' })
      content.consequences = await input({ message: t('adr.new.prompt.consequences'), default: '' })
      content.alternatives = await input({ message: t('adr.new.prompt.alternatives'), default: '' })
    }
    await generators.newADR(content)
  })

cmd.command('list')
  .description(t('adr.list.description'))
  .action(async () => {
    await generators.listADRs('docs/adr')
  })

module.exports = cmd
