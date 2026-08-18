'use strict'

// trackfw branch new <type>/<slug>
//
// CLI wiring for the `trackfw branch` command group. The testable logic lives in
// ../branch/runner.js (mirrors the ship.js / ship/runner.js split already used in this CLI).

const { Command } = require('commander')
const { runBranchNew } = require('../branch/runner')
const { runBranchPrune } = require('../branch/prune')

function createBranchCommand() {
  const cmd = new Command('branch')
  cmd.description('Manage governed feature branches')

  const newCmd = new Command('new')
  newCmd
    .description(
      'Create a feat/fix/refactor/chore/docs branch; feat/fix/refactor gated on a matching REQ + roadmap already in wip/ or done/\n\n' +
      'trackfw branch new moves the branch_has_wip_roadmap governance gate (already enforced\n' +
      "by 'trackfw validate' and 'trackfw ship') to before branch creation, instead of after:\n\n" +
      '  1. Validates <type> is one of feat, fix, refactor, chore, docs and <slug> is non-empty.\n' +
      '  2. For feat, fix, refactor: checks whether a roadmap in wip/ or done/ matches the given\n' +
      "     slug — the exact matching logic 'trackfw validate' already uses (normalized slug,\n" +
      "     filename contains match). Without a match: blocks — 'git checkout -b' is never\n" +
      "     executed — and prints the same governance orientation message 'trackfw validate'\n" +
      "     already prints for this rule.\n" +
      "  3. For chore, docs: housekeeping types already treated as roadmap-exempt by 'trackfw ship'\n" +
      "     and 'trackfw commit' — the branch is created without the roadmap gate.\n" +
      "  4. With a match (or for chore/docs): runs 'git checkout -b <type>/<slug>', propagating\n" +
      "     Git's own output and exit status literally.\n\n" +
      'Create the governance artifacts first if this blocks you:\n' +
      '  trackfw req new "title"\n' +
      '  trackfw roadmap new "title"\n' +
      '  trackfw roadmap move <name> wip'
    )
    .argument('<spec>', '<type>/<slug>, type in feat, fix, refactor, chore, docs')
    .option('--dry-run', 'Report whether the branch would be created or blocked, without executing git', false)
    .action((spec, options) => {
      const exitCode = runBranchNew(spec, !!options.dryRun, {})
      if (exitCode !== 0) {
        process.exit(exitCode)
      }
    })

  cmd.addCommand(newCmd)

  const pruneCmd = new Command('prune')
  pruneCmd
    .description('Report (and, with --apply, delete) local branches already integrated into origin/main')
    .addHelpText(
      'after',
      '\n' +
      'trackfw branch prune replaces the 6-step manual procedure documented in CLAUDE.md §1\n' +
      '("Uma branch ativa por vez") with a deterministic, offline command.\n\n' +
      "Decides integration with the touched-files heuristic, NOT git's own ancestry check (which\n" +
      'always refuses squash-merged branches) and NOT a naive bidirectional diff against origin/main\n' +
      '(which false-positives on a branch that is merged but stale, once main has advanced further):\n\n' +
      '  mb      = git merge-base origin/main <branch>\n' +
      '  touched = git diff --name-only mb <branch>                 (what the branch touched)\n' +
      '  diverg  = git diff --name-only origin/main <branch> -- touched  (what still differs there)\n\n' +
      'touched empty -> integrated (safe to delete)\n' +
      'diverg empty  -> integrated (safe to delete) -- squash-merged, stale, main advanced since\n' +
      'otherwise     -> kept, with the diverging files named\n\n' +
      'Every local branch is reported, always, with its decision and reason. The current branch, any\n' +
      'branch checked out in another worktree, and the default branch (main) are always kept and\n' +
      'never evaluated for deletion. Without origin/main resolvable (offline, no remote, never\n' +
      'fetched), the whole command refuses and deletes nothing.\n\n' +
      '--dry-run is the default: without --apply, nothing is ever deleted, even the clearly integrated.'
    )
    .option('--apply', 'Actually delete branches decided as integrated (default: report only, delete nothing)', false)
    .action((options) => {
      const exitCode = runBranchPrune(!!options.apply, {})
      if (exitCode !== 0) {
        process.exit(exitCode)
      }
    })

  cmd.addCommand(pruneCmd)
  return cmd
}

module.exports = createBranchCommand()
