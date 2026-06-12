'use strict'
const { Command } = require('commander')
const { validate } = require('../validator')

const cmd = new Command('validate')
cmd.description('Validate governance rules')
cmd.action(async () => {
  const { violations, warnings } = await validate()

  if (violations.length === 0 && warnings.length === 0) {
    console.log('✓ No violations found.')
    return
  }

  if (violations.length > 0) {
    console.log(`\n✗ Violations (${violations.length}):`)
    violations.forEach(v => console.log(`  • ${v}`))
  }

  if (warnings.length > 0) {
    console.log(`\n⚠ Warnings (${warnings.length}):`)
    warnings.forEach(w => console.log(`  • ${w}`))
  }

  if (violations.length > 0) process.exit(1)
})

module.exports = cmd
