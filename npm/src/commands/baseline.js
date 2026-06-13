'use strict'
const { Command } = require('commander')
const { validateUnfiltered, saveBaseline } = require('../validator')

const cmd = new Command('baseline')
cmd.description('Grava snapshot das violations atuais em .trackfw-baseline.json')
cmd.action(async () => {
  const { violations, warnings } = await validateUnfiltered()
  saveBaseline(violations, warnings)
  console.log(`Baseline gravado: ${violations.length} violations, ${warnings.length} warnings`)
})

module.exports = cmd
