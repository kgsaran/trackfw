'use strict'

// trackfw branch new <type>/<slug>
//
// CLI wiring for the `trackfw branch` command group. The testable logic lives in
// ../branch/runner.js (mirrors the ship.js / ship/runner.js split already used in this CLI).

const { Command } = require('commander')
const { runBranchNew } = require('../branch/runner')

function createBranchCommand() {
  const cmd = new Command('branch')
  cmd.description('Manage governed feature branches')

  const newCmd = new Command('new')
  newCmd
    .description(
      'Create a feat/fix/refactor branch, gated on a matching REQ + roadmap already in wip/ or done/\n\n' +
      'trackfw branch new moves the branch_has_wip_roadmap governance gate (already enforced\n' +
      "by 'trackfw validate' and 'trackfw ship') to before branch creation, instead of after:\n\n" +
      '  1. Validates <type> is one of feat, fix, refactor and <slug> is non-empty.\n' +
      '  2. Checks whether a roadmap in wip/ or done/ matches the given slug — the exact matching\n' +
      "     logic 'trackfw validate' already uses (normalized slug, filename contains match).\n" +
      "  3. Without a match: blocks — 'git checkout -b' is never executed — and prints the same\n" +
      "     governance orientation message 'trackfw validate' already prints for this rule.\n" +
      "  4. With a match: runs 'git checkout -b <type>/<slug>', propagating Git's own output and\n" +
      '     exit status literally.\n\n' +
      'Create the governance artifacts first if this blocks you:\n' +
      '  trackfw req new "title"\n' +
      '  trackfw roadmap new "title"\n' +
      '  trackfw roadmap move <name> wip'
    )
    .argument('<spec>', '<type>/<slug>, type in feat, fix, refactor')
    .option('--dry-run', 'Report whether the branch would be created or blocked, without executing git', false)
    .action((spec, options) => {
      const exitCode = runBranchNew(spec, !!options.dryRun, {})
      if (exitCode !== 0) {
        process.exit(exitCode)
      }
    })

  cmd.addCommand(newCmd)
  return cmd
}

module.exports = createBranchCommand()
