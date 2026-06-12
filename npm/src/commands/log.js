'use strict'
const { Command } = require('commander')
const fs = require('fs')
const path = require('path')

const cmd = new Command('log')
cmd.description('Show roadmap state transition history')
cmd.option('--tail <n>', 'Number of recent transitions to show', '20')
cmd.action(async (opts) => {
  const tail = parseInt(opts.tail, 10)
  const logPath = path.join('docs', 'roadmaps', '.trackfw-log')

  if (!fs.existsSync(logPath)) {
    console.log('No transitions recorded yet.')
    return
  }

  const lines = fs.readFileSync(logPath, 'utf8')
    .split('\n')
    .filter(l => l.trim() !== '')

  const start = Math.max(0, lines.length - tail)
  const visible = lines.slice(start)

  console.log('── trackfw log ─────────────────────────')
  visible.forEach(l => console.log(l))
})

module.exports = cmd
