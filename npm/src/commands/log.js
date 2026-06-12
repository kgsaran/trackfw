'use strict'
const { Command } = require('commander')
const fs = require('fs')
const path = require('path')
const { t } = require('../i18n')

const cmd = new Command('log')
cmd.description(t('log.description'))
cmd.option('--tail <n>', t('log.tail'), '20')
cmd.action(async (opts) => {
  const tail = parseInt(opts.tail, 10)
  const logPath = path.join('docs', 'roadmaps', '.trackfw-log')

  if (!fs.existsSync(logPath)) {
    console.log(t('log.empty'))
    return
  }

  const lines = fs.readFileSync(logPath, 'utf8')
    .split('\n')
    .filter(l => l.trim() !== '')

  const start = Math.max(0, lines.length - tail)
  const visible = lines.slice(start)

  console.log(t('log.header'))
  visible.forEach(l => console.log(l))
})

module.exports = cmd
