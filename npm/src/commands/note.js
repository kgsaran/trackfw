'use strict'

const { Command } = require('commander')
const { newNote } = require('../generators/note')

function createNoteCommand() {
  const note = new Command('note').description('Manage vault knowledge notes')

  note
    .command('new <title>')
    .description('Create a new knowledge note in vault/notes/')
    .action((title) => {
      try {
        newNote(title)
      } catch (err) {
        console.error(`error: ${err.message}`)
        process.exit(1)
      }
    })

  return note
}

module.exports = createNoteCommand()
