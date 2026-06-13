'use strict'
const { Command } = require('commander')
const { listRoadmaps, showRoadmap, moveRoadmap, newRoadmap, newRoadmapFromReq } = require('../generators/roadmap')
const { t } = require('../i18n')

const cmd = new Command('roadmap')
cmd.description(t('roadmap.description'))

cmd.command('new')
  .description(t('roadmap.new.description'))
  .option('-t, --title <title>', 'Roadmap title')
  .option('-r, --req <path>', 'Path to the linked REQ')
  .option('--from-req <path>', 'Generate roadmap with ML stubs from REQ acceptance criteria')
  .action(async (opts) => {
    if (opts.fromReq) {
      newRoadmapFromReq(opts.fromReq)
      return
    }
    const title = opts.title || 'New Roadmap'
    const reqPath = opts.req || ''
    newRoadmap(title, reqPath)
  })

cmd.command('list')
  .description(t('roadmap.list.description'))
  .action(async () => {
    listRoadmaps()
  })

cmd.command('show <name>')
  .description(t('roadmap.show.description'))
  .action(async (name) => {
    showRoadmap(name)
  })

cmd.command('move <name> <state>')
  .description(t('roadmap.move.description'))
  .action(async (name, state) => {
    moveRoadmap(name, state)
  })

module.exports = cmd
