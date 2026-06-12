'use strict'
const { Command } = require('commander')
const { listRoadmaps, showRoadmap, moveRoadmap, newRoadmap } = require('../generators/roadmap')

const cmd = new Command('roadmap')
cmd.description('Manage Roadmaps')

cmd.command('new')
  .description('Create a new roadmap from a REQ')
  .option('-t, --title <title>', 'Roadmap title')
  .option('-r, --req <path>', 'Path to the linked REQ')
  .action(async (opts) => {
    const title = opts.title || 'New Roadmap'
    const reqPath = opts.req || ''
    newRoadmap(title, reqPath)
  })

cmd.command('list')
  .description('List all roadmaps grouped by state')
  .action(async () => {
    listRoadmaps()
  })

cmd.command('show <name>')
  .description('Show a roadmap by name (partial match)')
  .action(async (name) => {
    showRoadmap(name)
  })

cmd.command('move <name> <state>')
  .description('Move a roadmap between states (backlog|wip|blocked|done|abandoned)')
  .action(async (name, state) => {
    moveRoadmap(name, state)
  })

module.exports = cmd
