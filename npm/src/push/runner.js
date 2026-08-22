'use strict'

/**
 * push/runner.js — Core implementation of `trackfw push`.
 *
 * Empurra commits já criados, sem commitar e sem abrir PR/MR.
 * Reusa os helpers de npm/src/ship/runner.js; nunca reimplementa a lógica de governança,
 * detecção de squash-merge ou construção de push args.
 *
 * See ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md.
 */

const {
  isShipBranch,
  isGatedShipBranch,
  isGitWriteCmd,
  checkShipGovernance,
  detectPendingSquashMerges,
  defaultCheckPROpen,
  buildPushArgs,
  defaultExecGit,
} = require('../ship/runner')

/**
 * defaultCheckGovernance delegates to checkShipGovernance — the same hard gate used by
 * `trackfw ship` and `trackfw commit`.
 * @returns {string[]}
 */
function defaultCheckGovernance() {
  return checkShipGovernance()
}

const { resolve: forgeResolve } = require('../forge/resolve')
const { forgeAdapter } = require('../forge/adapter')

// push --force-with-lease refusal messages — "trackfw push" (not "ship") because this command
// closes the commit→push cycle without opening a PR. See
// ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md.
// Defined here (not imported from ship/runner.js) to keep push's user-visible messages
// independent of ship's contract string.
const PUSH_FORCE_LEASE_NO_FORGE_CLI_MSG =
  'trackfw push --force-with-lease requires a forge CLI (gh, glab, or az) to confirm an open pull/merge request before rewriting remote history. No forge CLI is available for this repository — install and authenticate it, or push without --force-with-lease.'

function pushForceLeaseNoPROpenMsg(branch) {
  return `trackfw push --force-with-lease refuses to run: branch "${branch}" has no open pull/merge request. Open the PR/MR first (trackfw ship without --force-with-lease, or your forge's web UI), then retry.`
}

function pushForceLeaseCannotVerifyMsg(branch, cliName, errMessage) {
  return `trackfw push --force-with-lease could not verify whether branch "${branch}" has an open pull/merge request (${cliName} CLI error: ${errMessage}). Refusing rather than risking a force push without a verified PR — check your ${cliName} CLI authentication and retry.`
}

/**
 * runPush implements the push sequence: branch validation, governance, force-with-lease gate,
 * squash-merge detection, and push. Never commits and never opens a PR/MR.
 *
 * @param {{ dryRun?: boolean, forceWithLease?: boolean }} opts
 * @param {{
 *   execGit?: function,
 *   checkGovernance?: function,
 *   writeErr?: function,
 *   configForge?: string,
 *   repoDir?: string,
 *   availFn?: function,
 *   checkPROpen?: function,
 * }} deps
 * @returns {number} exit code (0 = success)
 */
function runPush(opts, deps) {
  opts = opts || {}
  deps = deps || {}

  const execGit = deps.execGit || defaultExecGit
  const checkGovernanceFn = deps.checkGovernance || defaultCheckGovernance
  // writeln is injectable (mirrors ship/runner.js) so tests can capture stdout without spawning
  // a real process. Production callers omit it and fall back to process.stdout.
  const writeln = deps.writeln || ((s) => process.stdout.write(s + '\n'))
  // "Error: " prefix — mirrors Go exactly: push.go returns messages as errors, and
  // root.go's ExecuteC path emits `cmd.ErrPrefix() + " " + err.Error() + "\n"` —
  // cmd.ErrPrefix() is "Error:", so the combined line is "Error: <message>\n".
  const writeErr = deps.writeErr || ((s) => process.stderr.write(`Error: ${s}\n`))

  // Inner wrapper: skips write commands in --dry-run mode.
  function git(args) {
    if (opts.dryRun && isGitWriteCmd(args)) {
      writeln(`[dry-run] git ${args.join(' ')}`)
      return { stdout: '', error: null }
    }
    return execGit(args)
  }

  // ─── Step 1: Branch validation ─────────────────────────────────────────────
  const branchResult = execGit(['symbolic-ref', '--short', 'HEAD'])
  if (branchResult.error) {
    writeErr(`could not determine current branch (are you in a git repo?): ${branchResult.error.message}`)
    return 1
  }
  const branch = branchResult.stdout.trim()

  // main/master is blocked unconditionally.
  if (branch === 'main' || branch === 'master') {
    writeErr(`trackfw push cannot run on "${branch}" — use a feature branch:\n  git checkout -b feat/<slug>`)
    return 1
  }

  if (!isShipBranch(branch)) {
    writeErr(
      `branch "${branch}" does not match the required pattern feat|fix|refactor|chore|docs/<slug>\n` +
      'Rename your branch or create a new one:\n  git checkout -b feat/<slug>'
    )
    return 1
  }

  writeln(`Branch: ${branch}`)

  // ─── Step 2: Governance ────────────────────────────────────────────────────
  // push never reads the index — no doc-only exception. Governance is either
  // skipped (chore/docs) or enforced (feat/fix/refactor), nothing in between.
  if (!isGatedShipBranch(branch)) {
    // chore/docs: housekeeping types already exempted from this gate by
    // `trackfw branch new` and `trackfw commit` — push without it too.
    writeln('Governance: skipped (chore/docs branch)')
  } else {
    const violations = checkGovernanceFn()
    if (violations.length > 0) {
      writeln('\nGovernance check failed:')
      for (const v of violations) {
        writeln(`  ${v}`)
      }
      writeln('\nCreate the required artifacts before running push:')
      writeln('  trackfw req new "<title>"')
      writeln('  trackfw roadmap new "<title>"')
      writeln('  trackfw roadmap move <name> wip')
      writeln("\nNote: this governance check is a hard gate — it is not affected by lenient")
      writeln("mode or per-rule severity configured in trackfw.yaml. If 'trackfw validate'")
      writeln("passes but 'trackfw push' aborts here, you likely have lenient mode")
      writeln("configured — push always requires REQ + roadmap in wip/.")
      writeErr(`governance check failed: ${violations.length} violation(s)`)
      return 1
    }

    writeln('Governance: OK')
  }

  // ─── Step 2.5: force-with-lease gate ───────────────────────────────────────
  // Runs before any write (push) — a refusal here never leaves the caller unable to push.
  // Read-only, so it runs in --dry-run too.
  if (opts.forceWithLease) {
    const { stdout: gateRemoteURLRaw } = execGit(['remote', 'get-url', 'origin'])
    const gateRemoteURL = (gateRemoteURLRaw || '').trim()

    let resolution
    try {
      resolution = forgeResolve({
        flagForge: '',
        configForge: deps.configForge || '',
        remoteURL: gateRemoteURL,
        repoDir: deps.repoDir || '',
      })
    } catch (resErr) {
      writeErr(resErr.message)
      return 1
    }

    const adapter = forgeAdapter(resolution.forge, deps.availFn || undefined)
    if (resolution.forge === 'manual' || !adapter.available) {
      writeErr(PUSH_FORCE_LEASE_NO_FORGE_CLI_MSG)
      return 1
    }

    const checkPROpen = deps.checkPROpen || defaultCheckPROpen
    let open
    try {
      open = checkPROpen(adapter, branch)
    } catch (prErr) {
      writeErr(pushForceLeaseCannotVerifyMsg(branch, adapter.cliName, prErr.message))
      return 1
    }
    if (!open) {
      writeErr(pushForceLeaseNoPROpenMsg(branch))
      return 1
    }

    writeln(`force-with-lease: open ${adapter.noun} confirmed for "${branch}" (${resolution.forge}).`)
  }

  // ─── Step 3: Squash-merge detection ────────────────────────────────────────
  if (opts.dryRun) {
    writeln('[dry-run] git fetch origin --prune')
  } else {
    const { error: fetchErr } = execGit(['fetch', 'origin', '--prune'])
    if (fetchErr) {
      writeln('Warning: could not fetch origin (offline or no remote); skipping squash-merge check.')
    } else {
      detectPendingSquashMerges(branch, execGit, writeln)
    }
  }

  // ─── Step 4: Push ──────────────────────────────────────────────────────────
  let pushArgs = buildPushArgs(branch, execGit)
  if (opts.forceWithLease) {
    // Fixed position: push --force-with-lease [-u] origin <branch> — identical
    // across the 3 CLIs (the parity gate compares this literally).
    pushArgs = [pushArgs[0], '--force-with-lease', ...pushArgs.slice(1)]
  }
  const { error: pushErr } = git(pushArgs)
  if (pushErr) {
    writeErr(`git push failed: ${pushErr.message}`)
    return 1
  }

  if (!opts.dryRun) {
    writeln(`Pushed: ${branch} → origin/${branch}`)
  }

  return 0
}

module.exports = {
  runPush,
  PUSH_FORCE_LEASE_NO_FORGE_CLI_MSG,
  pushForceLeaseNoPROpenMsg,
  pushForceLeaseCannotVerifyMsg,
}
