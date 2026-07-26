'use strict'

const { Command } = require('commander')
const { runShip } = require('../ship/runner')

function createShipCommand() {
  const cmd = new Command('ship')
  cmd
    .description(
      'trackfw ship runs a governed delivery sequence:\n\n' +
      '  1. Validates branch name — must match feat|fix|refactor/<slug>\n' +
      '  2. Validates governance — REQ + roadmap in wip/ must exist\n' +
      '  3. Detects pending squash-merges in other branches (advisory only)\n' +
      '  4. Reviews what is staged (git status --short + git diff --cached --stat)\n' +
      '  5. Commits with Conventional Commits format (-m is required)\n' +
      '  6. Pushes to origin (adds -u if no upstream is configured yet)\n\n' +
      "Stage your files explicitly before running ship.\n" +
      "This command never executes 'git add .' or 'git add -A'."
    )
    .option('-m, --message <msg>', 'Commit message (Conventional Commits format required)')
    .option('--dry-run', 'Print what would be done without executing write commands', false)
    .action((options) => {
      const exitCode = runShip({
        message: options.message || '',
        dryRun: options.dryRun || false,
      })
      if (exitCode !== 0) {
        process.exit(exitCode)
      }
    })

  return cmd
}

module.exports = createShipCommand()
