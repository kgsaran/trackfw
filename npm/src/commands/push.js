'use strict'

const { Command } = require('commander')
const { runPush } = require('../push/runner')
const config = require('../config')

function createPushCommand() {
  const cmd = new Command('push')
  cmd
    .description(
      'trackfw push pushes already-created commits without committing and without opening a PR/MR.\n\n' +
      '  1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>\n' +
      '  2. Validates governance — REQ + roadmap in wip/ must exist for feat/fix/refactor branches\n' +
      '     (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip\n' +
      '     this check, mirroring \'trackfw commit\'\n' +
      '  3. Detects pending squash-merges in other branches (advisory only)\n' +
      '  4. Pushes to origin (adds -u if no upstream is configured yet)\n\n' +
      'push never commits and never opens a PR/MR.\n' +
      'Does not accept -m. If you have not committed yet, run \'trackfw commit -m "..."\' first.\n\n' +
      'Compositional vocabulary:\n' +
      '  trackfw commit -m "..."   commits\n' +
      '  trackfw push              pushes\n' +
      '  trackfw ship -m "..."     commit + push + PR (composition)\n\n' +
      "--force-with-lease pushes with 'git push --force-with-lease' instead of a plain push — for\n" +
      'the post-rebase case, where a plain push is rejected. It only runs when the branch already\n' +
      "has an open PR/MR (verified via the resolved forge CLI): the safe path is always to open the\n" +
      'PR first.'
    )
    .option('--dry-run', 'Print what would be done without executing write commands', false)
    .option('--force-with-lease', 'Governed force-push (git push --force-with-lease); requires an open PR/MR on this branch', false)
    .action((options) => {
      const cfg = config.load()
      const exitCode = runPush(
        {
          dryRun: options.dryRun || false,
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

module.exports = createPushCommand()
