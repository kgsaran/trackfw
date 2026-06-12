'use strict'
const { Command } = require('commander')
const { validate } = require('../validator')
const { t } = require('../i18n')

const cmd = new Command('validate')
cmd.description(t('validate.description'))
cmd.action(async () => {
  const { violations, warnings } = await validate()

  if (violations.length === 0 && warnings.length === 0) {
    console.log(t('validate.ok'))
    return
  }

  if (violations.length > 0) {
    console.log(`\n${t('validate.violations', { count: violations.length })}`)
    violations.forEach(v => console.log(`  • ${v}`))
  }

  if (warnings.length > 0) {
    console.log(`\n${t('validate.warnings', { count: warnings.length })}`)
    warnings.forEach(w => console.log(`  • ${w}`))
  }

  if (violations.length > 0) process.exit(1)
})

module.exports = cmd
