'use strict'

const path = require('path')
const os = require('os')
const { Command } = require('commander')
const { input } = require('@inquirer/prompts')
const generators = require('../generators/adr')
const { t } = require('../i18n')
const { homedir } = require('../homedir')

const cmd = new Command('adr')
cmd.description(t('adr.description'))

/**
 * Resolve o diretório de ADRs conforme --scope.
 * 'project' (default) preserva o comportamento atual (adrDirs[0] do trackfw.yaml).
 * 'global' escreve/lista em ~/.trackfw/adr, sem exigir trackfw.yaml no cwd.
 * @param {string} scope
 * @returns {string}
 */
function resolveAdrDir(scope) {
  if (scope === 'global') {
    return path.join(homedir(), '.trackfw', 'adr')
  }
  if (scope === 'project') {
    return require('../config').load().adrDirs[0]
  }
  console.error(`Invalid --scope value: "${scope}". Expected "project" or "global".`)
  process.exit(1)
}

cmd.command('new <title>')
  .description(t('adr.new.description'))
  .option('--scope <scope>', 'ADR scope: project (default) or global (~/.trackfw/adr)', 'project')
  .action(async (title, options) => {
    const adrDir = resolveAdrDir(options.scope)
    const content = { title }
    // wizard interativo se TTY
    if (process.stdin.isTTY) {
      content.context = await input({ message: t('adr.new.prompt.context'), default: '' })
      content.decision = await input({ message: t('adr.new.prompt.decision'), default: '' })
      content.consequences = await input({ message: t('adr.new.prompt.consequences'), default: '' })
      content.alternatives = await input({ message: t('adr.new.prompt.alternatives'), default: '' })
    }
    await generators.newADR(content, adrDir)
  })

cmd.command('list')
  .description(t('adr.list.description'))
  .option('--scope <scope>', 'ADR scope: project (default) or global (~/.trackfw/adr)', 'project')
  .action(async (options) => {
    const adrDir = resolveAdrDir(options.scope)
    await generators.listADRs(adrDir)
  })

module.exports = cmd
