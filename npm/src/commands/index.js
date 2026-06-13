'use strict'

const { Command } = require('commander')
const { version } = require('../../package.json')

function createProgram() {
  const program = new Command()
  program
    .name('trackfw')
    .description('trackfw — governed software delivery framework\nADR → REQ → ROADMAP → kanban')
    .version(version)

  program.addCommand(require('./init'))
  program.addCommand(require('./adr'))
  program.addCommand(require('./req'))
  program.addCommand(require('./roadmap'))
  program.addCommand(require('./validate'))
  program.addCommand(require('./status'))
  program.addCommand(require('./log'))
  program.addCommand(require('./plugins'))
  program.addCommand(require('./discover'))

  // plugin dispatch — comandos desconhecidos tentam executar plugin
  program.hook('preSubcommand', () => {})

  return program
}

module.exports = { createProgram }
