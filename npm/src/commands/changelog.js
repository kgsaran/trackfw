'use strict'
const { Command } = require('commander')
const { read, parseSections, firstSection, findVersion, formatSection } = require('../changelog')

const cmd = new Command('changelog')
cmd.description('Show entries from CHANGELOG.md')
// enablePositionalOptions escopa a flag local "--version" a este subcomando,
// evitando que o root a intercepte como a flag global "trackfw --version".
cmd.enablePositionalOptions()
cmd.option('--version <x.y.z>', 'Show a specific version section')
cmd.option('--all', 'Show the entire CHANGELOG.md')
cmd.action(async (opts) => {
  try {
    const root = process.cwd()
    const content = read(root)
    if (opts.all) {
      process.stdout.write(content)
      return
    }
    const sections = parseSections(content)
    const section = opts.version ? findVersion(sections, opts.version) : firstSection(sections)
    process.stdout.write(formatSection(section))
  } catch (err) {
    console.error(`Error: ${err.message}`)
    process.exitCode = 1
  }
})

module.exports = cmd
