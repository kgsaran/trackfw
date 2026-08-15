'use strict'

const { Command } = require('commander')
const { version } = require('../../package.json')

function createProgram() {
  const program = new Command()
  program
    .name('trackfw')
    .description('trackfw — governed software delivery framework\nADR → REQ → ROADMAP → kanban')
    .version(`trackfw ${version}`)
  // enablePositionalOptions: garante que uma flag de subcomando (ex.: o
  // "changelog --version <x.y.z>") não seja capturada pela flag global
  // "--version" do root — sem isto, commander casa "--version" contra o
  // primeiro registro conhecido em toda a árvore de comandos.
  program.enablePositionalOptions()

  program.addCommand(require('./init'))
  program.addCommand(require('./adr'))
  program.addCommand(require('./req'))
  program.addCommand(require('./roadmap'))
  program.addCommand(require('./validate'))
  program.addCommand(require('./status'))
  program.addCommand(require('./changelog'))
  program.addCommand(require('./log'))
  program.addCommand(require('./plugins'))
  program.addCommand(require('./discover'))
  program.addCommand(require('./update'))
  program.addCommand(require('./metrics'))
  program.addCommand(require('./sync'))
  program.addCommand(require('./context'))
  program.addCommand(require('./baseline'))
  program.addCommand(require('./help'))
  program.addCommand(require('./configure'))
  program.addCommand(require('./version'))
  program.addCommand(require('./agents'))
  program.addCommand(require('./skills'))
  program.addCommand(require('./note'))
  program.addCommand(require('./ship'))
  program.addCommand(require('./barrier'))
  program.addCommand(require('./branch'))
  program.addCommand(require('./commit'))

  const { createServeCommand } = require('./serve')
  program.addCommand(createServeCommand())

  // plugin dispatch — comandos desconhecidos tentam executar plugin
  program.hook('preSubcommand', () => {})

  return program
}

module.exports = { createProgram }
