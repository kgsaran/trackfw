'use strict'
const { Command } = require('commander')
const { validate, isLenient, lenientUntilDate, getItemMeta } = require('../validator')
const { t } = require('../i18n')

const cmd = new Command('validate')
cmd.description(t('validate.description'))
cmd.option('--json', 'output result as JSON')
cmd.action(async (options) => {
  const { violations, warnings } = await validate()
  const lenient = isLenient()
  const mode = lenient ? 'lenient' : 'strict'
  const exitCode = violations.length > 0 ? 1 : 0

  if (options.json) {
    const output = {
      summary: {
        violations: violations.length,
        warnings: warnings.length,
        mode,
        exit_code: exitCode,
      },
      violations: violations.map(v => { const m = getItemMeta(v); return { message: v, rule: m.rule, file: m.file } }),
      warnings: warnings.map(w => { const m = getItemMeta(w); return { message: w, rule: m.rule, file: m.file } }),
    }
    console.log(JSON.stringify(output, null, 2))
    process.exit(exitCode)
    return
  }

  // Informar usuário sobre modo lenient
  if (lenient) {
    const until = lenientUntilDate()
    if (until) {
      console.log(`[LENIENT MODE] ${t('validate.lenient_mode', { date: until })}`)
    }
  }

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
