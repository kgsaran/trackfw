'use strict'

const { Command } = require('commander')
const { version } = require('../../package.json')

module.exports = new Command('version')
  .description('Print version')
  .action(() => {
    console.log(`trackfw ${version}`)
  })
