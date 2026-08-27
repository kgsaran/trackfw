'use strict'

const { Command } = require('commander')
const { version } = require('../../package.json')
const { formatUnknownCommandError } = require('../lib/unknown-command')

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

  // Comando desconhecido — mensagem canônica compartilhada pelos 3 CLIs
  // (ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-
  // de-terceiro.md D3). Registrar um listener 'command:*' intercepta ANTES do
  // commander chamar seu próprio Command.unknownCommand() — evita reformatar
  // uma mensagem já impressa por ele (o padrão que causava dupla-impressão no
  // Go, ver internal/commands/root.go) e evita depender do texto/threshold
  // próprio de commander.suggestSimilar, que diverge de cobra/argparse.
  program.on('command:*', (operands) => {
    const typed = operands[0]
    const candidates = program.commands.map((cmd) => cmd.name())
    process.stderr.write(formatUnknownCommandError(typed, candidates, program.name()) + '\n')
    process.exit(1)
  })

  program.addCommand(require('./init'))
  program.addCommand(require('./adr'))
  program.addCommand(require('./req'))
  program.addCommand(require('./roadmap'))
  program.addCommand(require('./validate'))
  program.addCommand(require('./status'))
  program.addCommand(require('./changelog'))
  program.addCommand(require('./log'))
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
  program.addCommand(require('./release'))
  program.addCommand(require('./barrier'))
  program.addCommand(require('./branch'))
  program.addCommand(require('./commit'))
  program.addCommand(require('./push'))
  program.addCommand(require('./doctor'))
  program.addCommand(require('./audit-surface'))

  const { createServeCommand } = require('./serve')
  program.addCommand(createServeCommand())

  return program
}

module.exports = { createProgram }
