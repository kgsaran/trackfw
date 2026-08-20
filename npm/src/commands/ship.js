'use strict'

const { Command } = require('commander')
const { runShip } = require('../ship/runner')
const config = require('../config')

function createShipCommand() {
  const cmd = new Command('ship')
  cmd
    .description(
      'trackfw ship runs a governed delivery sequence:\n\n' +
      '  1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>\n' +
      '  2. Validates governance — REQ + roadmap in wip/ must exist for feat/fix/refactor branches\n' +
      '     (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip\n' +
      "     this check, mirroring 'trackfw commit'\n" +
      '  3. Detects pending squash-merges in other branches (advisory only)\n' +
      '  4. Reviews what is staged (git status --short + git diff --cached --stat)\n' +
      '  5. Commits with Conventional Commits format (-m is required, unless nothing is staged\n' +
      '     and --force-with-lease is set — see below)\n' +
      '  6. Pushes to origin (adds -u if no upstream is configured yet)\n' +
      '  7. Opens PR/MR via the resolved forge CLI (or prints URL if CLI is absent)\n\n' +
      "Stage your files explicitly before running ship.\n" +
      "This command never executes 'git add .' or 'git add -A'.\n\n" +
      "--force-with-lease pushes with 'git push --force-with-lease' instead of a plain push —\n" +
      "for the post-rebase case, where a plain push is rejected. It only runs when the branch\n" +
      "already has an open PR/MR (verified via the resolved forge CLI): the safe path is\n" +
      "always to open the PR first. When nothing is staged, it pushes existing commits\n" +
      "without requiring -m."
    )
    .option('-m, --message <msg>', 'Commit message (Conventional Commits format required)')
    .option('--dry-run', 'Print what would be done without executing write commands', false)
    .option('--no-pr', 'Skip PR/MR creation after push')
    .option('--forge <forge>', 'Override forge detection (github, gitlab, bitbucket, azure)', '')
    .option('--force-with-lease', 'Governed force-push (git push --force-with-lease); requires an open PR/MR on this branch', false)
    .action((options) => {
      const cfg = config.load()
      const exitCode = runShip(
        {
          message: options.message || '',
          dryRun: options.dryRun || false,
          noPR: options.pr === false,
          forge: options.forge || '',
          forceWithLease: options.forceWithLease || false,
        },
        {
          configForge: cfg.forge || '',
          repoDir: '.',
        }
      )
      if (exitCode !== 0) {
        process.exit(exitCode)
      }
    })

  return cmd
}

module.exports = createShipCommand()
