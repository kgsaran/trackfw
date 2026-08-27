'use strict'

// update-engine.js — shared state machine for `trackfw update` and
// `trackfw update harness` (ML-6C of ROADMAP-2026-07-29-barrier-governanca-
// e-autoridade-do-orquestrador). Both commands report one of exactly four
// states per target (`updated`, `skipped`, `missing`, `failed`) and emit the
// same JSON document shape — see docs/cli-parity.md, section
// "`trackfw update` vs `trackfw update harness`" for the frozen contract.
//
// Key invariant enforced here: a `missing` target is NEVER installed unless
// `installMissing` is explicitly true. This is the single most
// safety-critical rule in the contract, so it is centralized in
// `runFileTarget` rather than re-implemented per target.

const fs = require('fs')
const os = require('os')
const path = require('path')
const crypto = require('crypto')

function sha256(buf) {
  return crypto.createHash('sha256').update(buf).digest('hex')
}

function collectDirEntries(dir) {
  const out = []
  function walk(current, rel) {
    let entries
    try {
      entries = fs.readdirSync(current, { withFileTypes: true })
    } catch (_) {
      return
    }
    entries.sort((a, b) => a.name.localeCompare(b.name))
    for (const entry of entries) {
      const childRel = rel ? path.join(rel, entry.name) : entry.name
      const childAbs = path.join(current, entry.name)
      if (entry.isDirectory()) walk(childAbs, childRel)
      else out.push([childRel, sha256(fs.readFileSync(childAbs))])
    }
  }
  walk(dir, '')
  return out
}

// hashPath — returns null when the path does not exist, a content hash for
// a file, or a hash of the recursive (relative-path, content-hash) listing
// for a directory. Used to detect "nothing changed" without caring whether
// the target is a single file or a directory of generated artifacts.
function hashPath(target) {
  if (!fs.existsSync(target)) return null
  const stat = fs.lstatSync(target)
  if (stat.isDirectory()) return JSON.stringify(collectDirEntries(target))
  return sha256(fs.readFileSync(target))
}

function copyPath(src, dst) {
  if (!fs.existsSync(src)) return
  const stat = fs.lstatSync(src)
  if (stat.isDirectory()) {
    fs.mkdirSync(dst, { recursive: true })
    for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
      copyPath(path.join(src, entry.name), path.join(dst, entry.name))
    }
  } else {
    fs.mkdirSync(path.dirname(dst), { recursive: true })
    fs.copyFileSync(src, dst)
  }
}

// silenceConsole — apply()s reused from generators/init.js and friends log
// their own progress via console.log; those messages are only meaningful
// against the real root, not the scratch copy used to predict --dry-run
// state. Suppressed during simulation only.
function silenceConsole(fn) {
  const original = console.log
  console.log = () => {}
  try {
    return fn()
  } finally {
    console.log = original
  }
}

/**
 * runFileTarget — computes updated/skipped/missing/failed for a target whose
 * only observable effect is writing under a fixed set of paths (files or
 * directories) relative to `root`, by diffing content hashes before/after
 * invoking `apply(root)`.
 *
 * - `--dry-run`: `apply` runs against a scratch temp directory seeded only
 *   with copies of the paths that already exist under `root` — the real
 *   `root` is never touched.
 * - real run: `apply` runs directly against `root`.
 * - `missing` (nothing present under any of `relPaths`) never triggers
 *   `apply` unless `installMissing` is true.
 */
// runFileTarget — computes updated/skipped/missing/failed for a target whose
// only observable effect is writing under a fixed set of paths (files or
// directories) relative to `root`, by diffing content hashes before/after
// invoking `apply(root)`.
//
// `seeds` (optional) — extra paths copied into the dry-run sandbox but NOT
// included in the hash comparison. Used for:
//   - trackfw.yaml: agent-rules reads it for agent_conventions (Gap E)
//   - detection signals for agent-hooks: .windsurfrules, .github/copilot-
//     instructions.md, etc. — InjectHooksDetected checks these to decide
//     which hooks to write; absent → hooks silently omitted (Gap C)
function runFileTarget({ id, path: displayPath, root, relPaths, seeds = [], apply, dryRun, installMissing }) {
  const before = relPaths.map(rel => hashPath(path.join(root, rel)))
  const allMissingBefore = before.every(h => h === null)

  if (allMissingBefore && !installMissing) {
    return { id, state: 'missing', path: displayPath }
  }

  try {
    let after
    if (dryRun) {
      const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-update-'))
      try {
        for (const rel of relPaths) copyPath(path.join(root, rel), path.join(tmp, rel))
        for (const rel of seeds) copyPath(path.join(root, rel), path.join(tmp, rel))
        silenceConsole(() => apply(tmp))
        after = relPaths.map(rel => hashPath(path.join(tmp, rel)))
      } finally {
        fs.rmSync(tmp, { recursive: true, force: true })
      }
    } else {
      apply(root)
      after = relPaths.map(rel => hashPath(path.join(root, rel)))
    }

    const allMissingAfter = after.every(h => h === null)
    if (allMissingBefore && allMissingAfter) return { id, state: 'missing', path: displayPath }
    const unchanged = before.length === after.length && before.every((h, i) => h === after[i])
    return { id, state: unchanged ? 'skipped' : 'updated', path: displayPath }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }
}

function summarize(targets) {
  const summary = { updated: 0, skipped: 0, missing: 0, failed: 0 }
  for (const t of targets) summary[t.state] += 1
  return summary
}

// tildeify — the contract's JSON example shows global paths abbreviated as
// `~/...`; emitting the absolute path would both break byte-parity with the
// other runtimes and leak the local username into committed/shared output.
//
// Both operands are run through path.normalize before comparison: a raw
// process.env.HOME can contain a redundant separator (e.g. macOS's default
// $TMPDIR ends in "/", so a test harness building HOME as `"$TMPDIR/foo"`
// yields ".../T//foo"), while `absPath` here is always produced via
// path.join, which normalizes internally — comparing the two without
// normalizing homeRoot first made the prefix check silently fail and fall
// through to the absolute-path branch. Mirrors Python's _tildeify
// (pypi/trackfw/commands/update_harness.py), which normalizes both sides
// via os.path.normpath for the same reason.
//
// A second, subtler case (ML-6H): path.normalize PRESERVES a trailing
// separator when the input already has one (macOS's default $TMPDIR itself
// ends in "/", so a HOME built as "$TMPDIR/foo" collapses the internal
// "//" but still ends in a single trailing "/"). The prefix check below
// then appended ANOTHER path.sep, comparing against ".../foo//" — which
// normalizedPath (which never carries a trailing separator, since it comes
// from path.join) never starts with — so the check silently failed and
// fell through to the absolute-path branch even though homeRoot had
// already been normalized. Stripping a trailing separator from
// normalizedHome (root "/" excepted) before the prefix check closes this.
function tildeify(homeRoot, absPath) {
  let normalizedHome = path.normalize(homeRoot)
  if (normalizedHome.length > path.sep.length && normalizedHome.endsWith(path.sep)) {
    normalizedHome = normalizedHome.slice(0, -path.sep.length)
  }
  const normalizedPath = path.normalize(absPath)
  if (normalizedPath === normalizedHome) return '~'
  if (normalizedPath.startsWith(normalizedHome + path.sep)) return '~' + normalizedPath.slice(normalizedHome.length)
  return normalizedPath
}

// validateTargets — throws (usage error) when the caller asked for an id
// that does not exist in the declared universe for this command. Returns
// the requested id list unchanged (or null when no filter was requested).
function validateTargets(allIds, wanted) {
  if (!wanted || !wanted.length) return null
  for (const id of wanted) {
    if (!allIds.includes(id)) throw new Error(`Unknown update target: ${id}`)
  }
  return wanted
}

function buildDocument(scope, dryRun, targets) {
  return {
    scope,
    dry_run: dryRun,
    targets: targets.map(t => {
      const entry = { id: t.id, state: t.state, path: t.path }
      if (t.state === 'failed' && t.message) entry.message = t.message
      return entry
    }),
    summary: summarize(targets),
  }
}

function humanReport(scope, dryRun, targets) {
  const lines = [`trackfw update — scope: ${scope}${dryRun ? ' (dry-run)' : ''}`, '']
  for (const t of targets) {
    const mark = { updated: '✓', skipped: '·', missing: '—', failed: '✗' }[t.state] || '?'
    const suffix = t.state === 'failed' && t.message ? ` (${t.message})` : ''
    lines.push(`  ${mark} ${t.id.padEnd(24)} ${t.state.padEnd(8)} ${t.path}${suffix}`)
  }
  const summary = summarize(targets)
  lines.push('', `updated=${summary.updated} skipped=${summary.skipped} missing=${summary.missing} failed=${summary.failed}`)
  return lines.join('\n')
}

module.exports = {
  hashPath,
  copyPath,
  runFileTarget,
  summarize,
  tildeify,
  validateTargets,
  buildDocument,
  humanReport,
  silenceConsole,
}
