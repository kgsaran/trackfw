'use strict'

const { Command } = require('commander')
const { runReleaseTag } = require('../release/runner')
const config = require('../config')

function createReleaseCommand() {
  const cmd = new Command('release')
  cmd.description('Governed release operations')

  const tagCmd = new Command('tag')
  tagCmd
    .description(
      'Create and publish an annotated release tag.\n\n' +
      "It exists because 'trackfw ship' only pushes branches — tag is not a branch operation,\n" +
      "and ship's governance gate (\"REQ + roadmap in wip/\") does not apply to release. See\n" +
      'ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.\n\n' +
      'Every precondition below refuses with a message naming what to fix — this command\n' +
      'never guesses:\n' +
      '  1. Working tree must be clean.\n' +
      '  2. The default branch (main/master), if checked out locally, must be up to date with origin.\n' +
      '  3. The 4 version files must all match <version> exactly.\n' +
      '  4. CHANGELOG.md must have a "## [<version>] - YYYY-MM-DD" section.\n' +
      '  5. The tag must not already exist, locally or on origin.\n' +
      '  6. The GitHub CLI (gh) must be available and authenticated — release tag currently\n' +
      '     only supports GitHub; other forges are refused with instructions to push the tag\n' +
      '     manually.\n\n' +
      'On success, it publishes the tag via two GitHub API calls (POST git/tags then POST\n' +
      'git/refs), preserving the annotation.'
    )
    .argument('<version>', 'Version to tag (with or without a leading "v")')
    .action((version) => {
      const cfg = config.load()
      const exitCode = runReleaseTag(version, {
        configForge: cfg.forge || '',
        repoDir: '.',
      })
      if (exitCode !== 0) {
        process.exit(exitCode)
      }
    })

  cmd.addCommand(tagCmd)
  return cmd
}

module.exports = createReleaseCommand()
