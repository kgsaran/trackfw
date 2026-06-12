'use strict'
const { Command } = require('commander')
const { getStatus } = require('../validator')

const cmd = new Command('status')
cmd.description('Show project governance status')
cmd.action(async () => {
  console.log(await getStatus())
})

module.exports = cmd
