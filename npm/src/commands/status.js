'use strict'
const { Command } = require('commander')
const { getStatus } = require('../validator')
const { t } = require('../i18n')

const cmd = new Command('status')
cmd.description(t('status.description'))
cmd.action(async () => {
  console.log(await getStatus())
})

module.exports = cmd
