'use strict'

/**
 * push.test.js — Unit tests for trackfw push (Node.js)
 *
 * Covers the same 5 cases as Go (internal/commands/push_test.go):
 *   1. No upstream → -u is present in push args
 *   2. With upstream → -u is absent from push args
 *   3. Branch `main` blocked
 *   4. Governance absent in feat/
 *   5. Exemption in chore/
 *
 * Follows the style and dependency-injection mechanisms of ship.test.js.
 */

const test = require('node:test')
const assert = require('node:assert/strict')
const { runPush } = require('../src/push/runner')

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

/**
 * makeMockPushGit builds an execGit mock for push tests.
 * Captures all calls and responds based on branch and hasUpstream.
 */
function makeMockPushGit({ branch = 'feat/my-feature', hasUpstream = false } = {}) {
  const calls = []
  function execGit(args) {
    calls.push(args.slice())
    const joined = args.join(' ')

    if (joined.startsWith('symbolic-ref --short')) {
      if (!branch) return { stdout: '', error: new Error('not a git repo') }
      return { stdout: branch, error: null }
    }
    if (joined.includes('@{u}')) {
      if (hasUpstream) return { stdout: `origin/${branch}`, error: null }
      return { stdout: '', error: new Error('no upstream') }
    }
    if (joined.startsWith('remote get-url')) {
      return { stdout: '', error: new Error('no remote') }
    }
    if (joined.startsWith('fetch')) {
      // Non-blocking — squash-merge step skips on error.
      return { stdout: '', error: new Error('offline') }
    }
    if (joined.startsWith('branch -r')) {
      return { stdout: '', error: null }
    }
    if (joined.startsWith('push')) {
      return { stdout: '', error: null }
    }

    return { stdout: '', error: null }
  }
  execGit.calls = calls
  return execGit
}

/**
 * captureOutput splits writeln (stdout) and writeErr (stderr) into separate buffers.
 * output() concatenates both for substring assertions.
 */
function captureOutput() {
  const outLines = []
  const errLines = []
  return {
    writeln: (s) => outLines.push(s),
    writeErr: (s) => errLines.push(`Error: ${s}`),
    outLines,
    errLines,
    output: () => [...outLines, ...errLines].join('\n'),
  }
}

/**
 * makeDeps builds a deps object with injectable fakes.
 * writeErr is injectable (runner.js hardcodes writeln to stdout, but writeErr is injectable).
 */
function makeDeps({ branch = 'feat/my-feature', hasUpstream = false, violations = [] } = {}) {
  const execGit = makeMockPushGit({ branch, hasUpstream })
  const cap = captureOutput()
  const deps = {
    execGit,
    checkGovernance: () => violations,
    writeln: cap.writeln,
    writeErr: cap.writeErr,
  }
  return { deps, execGit, cap }
}

// ────────────────────────────────────────────────────────────────────────────
// Case 3: main branch blocked unconditionally
// ────────────────────────────────────────────────────────────────────────────

test('push: main branch aborts', () => {
  const { deps, cap } = makeDeps({ branch: 'main' })
  const code = runPush({}, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('cannot run on'), 'must mention cannot run on')
})

test('push: master branch aborts', () => {
  const { deps, cap } = makeDeps({ branch: 'master' })
  const code = runPush({}, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('cannot run on'), 'must mention cannot run on')
})

// ────────────────────────────────────────────────────────────────────────────
// Case 4: governance absent in feat/ → blocked
// ────────────────────────────────────────────────────────────────────────────

test('push: feat branch with no roadmap in wip/ aborts with governance error', () => {
  const violations = ['branch "feat/my-feature" is a feat/fix/refactor branch but no roadmap is in wip/']
  const { deps, cap } = makeDeps({ branch: 'feat/my-feature', violations })
  const code = runPush({}, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('governance check failed'), 'must mention governance check failed')
  assert.ok(cap.output().includes('trackfw req new'), 'must mention remediation')
  assert.ok(cap.output().includes('lenient'), 'must mention lenient mode so users understand why validate passes but push aborts')
})

// ────────────────────────────────────────────────────────────────────────────
// Case 5: chore/ branch — governance exempted
// ────────────────────────────────────────────────────────────────────────────

test('push: chore branch skips governance', () => {
  // violations would fail if governance were checked
  const violations = ['would fail if checked']
  const { deps, cap, execGit } = makeDeps({ branch: 'chore/update-deps', violations })
  const code = runPush({}, deps)
  assert.equal(code, 0)
  // Governance skip message goes to stdout (writeln) which is not injectable in push/runner.js;
  // confirm by absence of governance failure and exit 0.
  assert.equal(code, 0, 'chore branch must succeed despite governance violations being present')
  // The checkGovernance function must not have been called.
  // Confirm indirectly: governance violation text must not appear in captured output.
  assert.ok(!cap.output().includes('governance check failed'), 'governance must not be checked for chore/ branch')
})

test('push: docs branch skips governance', () => {
  const violations = ['would fail if checked']
  const { deps, cap } = makeDeps({ branch: 'docs/update-readme', violations })
  const code = runPush({}, deps)
  assert.equal(code, 0, 'docs branch must succeed despite governance violations being present')
  assert.ok(!cap.output().includes('governance check failed'), 'governance must not be checked for docs/ branch')
})

// ────────────────────────────────────────────────────────────────────────────
// Case 1: no upstream → push args include -u
// ────────────────────────────────────────────────────────────────────────────

test('push: no upstream configured → push args include -u', () => {
  // hasUpstream: false → execGit returns error for @{u} → buildPushArgs adds -u
  const { deps, execGit } = makeDeps({ branch: 'feat/my-feature', hasUpstream: false })
  const code = runPush({}, deps)
  assert.equal(code, 0)
  const pushCall = execGit.calls.find(c => c[0] === 'push')
  assert.ok(pushCall, 'a push call must have been made')
  assert.ok(pushCall.includes('-u'), `push args must include -u when no upstream is set; got: ${pushCall.join(' ')}`)
  assert.ok(pushCall.includes('origin'), 'push args must include origin')
  assert.ok(pushCall.includes('feat/my-feature'), 'push args must include branch name')
})

// ────────────────────────────────────────────────────────────────────────────
// Case 2: with upstream → push args do NOT include -u
// ────────────────────────────────────────────────────────────────────────────

test('push: upstream already configured → push args do not include -u', () => {
  // hasUpstream: true → execGit returns success for @{u} → buildPushArgs omits -u
  const { deps, execGit } = makeDeps({ branch: 'feat/my-feature', hasUpstream: true })
  const code = runPush({}, deps)
  assert.equal(code, 0)
  const pushCall = execGit.calls.find(c => c[0] === 'push')
  assert.ok(pushCall, 'a push call must have been made')
  assert.ok(!pushCall.includes('-u'), `push args must NOT include -u when upstream exists; got: ${pushCall.join(' ')}`)
  assert.ok(pushCall.includes('origin'), 'push args must include origin')
  assert.ok(pushCall.includes('feat/my-feature'), 'push args must include branch name')
})

// ────────────────────────────────────────────────────────────────────────────
// NeverCommits — push must never call git commit (ML-4A)
// ────────────────────────────────────────────────────────────────────────────

test('push: never calls git commit', () => {
  // chore/ branch: no governance check, no upstream (--dry-run keeps it safe)
  const { deps, execGit } = makeDeps({ branch: 'chore/update-deps' })
  const code = runPush({ dryRun: true }, deps)
  assert.equal(code, 0)
  const commitCall = execGit.calls.find(c => c[0] === 'commit')
  assert.equal(commitCall, undefined, `push must never call git commit, but found call: ${commitCall}`)
})

// ────────────────────────────────────────────────────────────────────────────
// GovernanceMessage — says "trackfw push", not "trackfw ship" (ML-4A)
// ────────────────────────────────────────────────────────────────────────────

test('push: governance message says "trackfw push", not "trackfw ship"', () => {
  const violations = ['no roadmap found in wip/ nor done/']
  const { deps, cap } = makeDeps({ branch: 'feat/orphan', violations })
  runPush({ dryRun: true }, deps)
  const output = cap.output()
  assert.ok(output.includes('trackfw push'), `governance message must contain "trackfw push"; got: ${output}`)
  assert.ok(!output.includes('trackfw ship'), `governance message must NOT contain "trackfw ship"; got: ${output}`)
})
