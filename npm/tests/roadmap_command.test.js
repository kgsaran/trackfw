'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')

const roadmapCommand = require('../src/commands/roadmap')

test('roadmap new exposes parity flags', () => {
  const newCommand = roadmapCommand.commands.find(command => command.name() === 'new')
  assert.ok(newCommand, 'roadmap new subcommand should be registered')

  const flags = new Set(newCommand.options.map(option => option.long))
  for (const flag of ['--title', '--req', '--from-req']) {
    assert.ok(flags.has(flag), `roadmap new should expose ${flag}`)
  }
})
