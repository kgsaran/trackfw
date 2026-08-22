'use strict'

// trackfw commit -m "<message>"
//
// CLI wiring for the `trackfw commit` command. The testable logic lives in
// ../commit/runner.js (mirrors the branch.js / branch/runner.js split already used in this CLI).

const { Command } = require('commander')
const { runCommit, buildSuggestedMessage } = require('../commit/runner')

function createCommitCommand() {
  const cmd = new Command('commit')
  cmd
    .description(
      "trackfw commit commits staged changes directly, but blocks the commit before it\n" +
      'happens when governance is missing, instead of letting it land and only catching it later.\n\n' +
      'Compositional vocabulary:\n' +
      '  trackfw commit -m "..."   commits\n' +
      '  trackfw push              pushes\n' +
      '  trackfw ship -m "..."     commit + push + PR (composition)\n\n' +
      'Behavioral steps:\n\n' +
      "  1. On 'main'/'master': always blocked — commit directly on the default branch is never\n" +
      '     permitted.\n' +
      '  2. On a feat/fix/refactor branch: requires a roadmap matching the branch slug already in\n' +
      "     wip/ or done/ — the exact matching logic 'trackfw branch new' and 'trackfw validate'\n" +
      '     already use. Without a match, blocks with the same governance orientation message.\n' +
      '  3. On any other branch (e.g. doc/housekeeping branches): allowed without requiring a\n' +
      '     roadmap — a warning is logged, but the commit proceeds.\n' +
      "  4. When allowed: runs 'git commit -m <message>', propagating Git's own output and exit\n" +
      '     status literally.\n\n' +
      "'--suggest' takes a completely separate path: it prints a heuristic Conventional Commits\n" +
      "skeleton built from 'git diff --cached --name-status' (type + staged file list) and exits\n" +
      'without ever committing — no LLM call, just a structural heuristic. It is not a\n' +
      'ready-to-use message; review and edit before using it with -m. When \'--suggest\' is set,\n' +
      "'-m' (if also passed) is ignored and no commit ever happens.\n\n" +
      'Create the governance artifacts first if this blocks you:\n' +
      '  trackfw req new "title"\n' +
      '  trackfw roadmap new "title"\n' +
      '  trackfw roadmap move <name> wip'
    )
    .option('-m, --message <msg>', 'Commit message (required)')
    .option('--suggest', 'Print a heuristic Conventional Commits message skeleton from staged files and exit without committing (ignores -m)')
    .action((options) => {
      if (options.suggest) {
        try {
          const suggestion = buildSuggestedMessage({})
          process.stdout.write(suggestion + '\n')
          return
        } catch (err) {
          process.stderr.write(err.message + '\n')
          process.exit(1)
        }
      }

      const exitCode = runCommit(options.message || '', {})
      if (exitCode !== 0) {
        process.exit(exitCode)
      }
    })

  return cmd
}

module.exports = createCommitCommand()
