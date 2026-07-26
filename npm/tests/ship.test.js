'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { runShip, isShipBranch, isGitWriteCmd, normalizeBranchSlug, GIT_WRITE_COMMANDS } = require('../src/ship/runner')

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

/**
 * mockExecGit builds an execGit mock that captures calls and responds
 * based on the configured branch and staged state.
 */
function makeMockGit({ branch = '', stagedFiles = '' } = {}) {
  const calls = []
  function execGit(args) {
    calls.push(args.slice())
    const joined = args.join(' ')

    if (joined.startsWith('symbolic-ref --short')) {
      if (!branch) return { stdout: '', error: new Error('not a git repo') }
      return { stdout: branch, error: null }
    }
    if (joined.startsWith('diff --cached --name-only')) {
      return { stdout: stagedFiles, error: null }
    }
    if (joined.includes('@{u}')) {
      return { stdout: '', error: new Error('no upstream') }
    }
    if (joined.startsWith('fetch')) {
      return { stdout: '', error: new Error('offline') }
    }
    return { stdout: '', error: null }
  }
  execGit.calls = calls
  return execGit
}

function makeOpts({ message = 'feat: test', dryRun = false } = {}) {
  return { message, dryRun }
}

function captureOutput() {
  const lines = []
  return {
    writeln: (s) => lines.push(s),
    lines,
    output: () => lines.join('\n'),
  }
}

function makeDeps({ branch, staged, violations = [] } = {}) {
  const git = makeMockGit({ branch, stagedFiles: staged })
  const cap = captureOutput()
  const deps = {
    execGit: git,
    checkGovernance: () => violations,
    writeln: cap.writeln,
  }
  return { deps, git, cap }
}

// ────────────────────────────────────────────────────────────────────────────
// Step 1 — Branch validation
// ────────────────────────────────────────────────────────────────────────────

test('ship: main branch aborts', () => {
  const { deps, cap } = makeDeps({ branch: 'main', staged: 'file.js' })
  const code = runShip(makeOpts(), deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('cannot run on'), 'must mention cannot run on')
})

test('ship: master branch aborts', () => {
  const { deps, cap } = makeDeps({ branch: 'master', staged: 'file.js' })
  const code = runShip(makeOpts(), deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('cannot run on'), 'must mention cannot run on')
})

test('ship: wrong pattern aborts', () => {
  for (const branch of ['feature/foo', 'hotfix/bar', 'docs/update', 'mybranch']) {
    const { deps, cap } = makeDeps({ branch, staged: 'file.js' })
    const code = runShip(makeOpts(), deps)
    assert.equal(code, 1, `${branch} should abort`)
    assert.ok(cap.output().includes('does not match the required pattern'), `${branch}: wrong error`)
  }
})

test('ship: valid branch patterns not rejected at step 1', () => {
  for (const branch of ['feat/my-feature', 'fix/bug-123', 'refactor/clean-up']) {
    const { deps, cap } = makeDeps({ branch, staged: 'file.js' })
    const code = runShip(makeOpts(), deps)
    // May fail at later steps, but must NOT fail with branch-pattern or main/master errors.
    const out = cap.output()
    assert.ok(!out.includes('does not match the required pattern'), `${branch}: should be valid`)
    assert.ok(!out.includes('cannot run on'), `${branch}: should not trigger main/master check`)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// Step 2 — Governance
// ────────────────────────────────────────────────────────────────────────────

test('ship: no wip roadmap aborts with remediation commands', () => {
  const violations = ['branch "feat/foo" is a feat/fix/refactor branch but no roadmap is in wip/']
  const { deps, cap } = makeDeps({ branch: 'feat/foo', staged: 'file.js', violations })
  const code = runShip(makeOpts(), deps)
  assert.equal(code, 1)
  const out = cap.output()
  assert.ok(out.includes('governance check failed'), 'must mention governance check failed')
  assert.ok(out.includes('trackfw req new'), 'must mention trackfw req new')
  assert.ok(out.includes('trackfw roadmap new'), 'must mention trackfw roadmap new')
  assert.ok(out.includes('trackfw roadmap move'), 'must mention trackfw roadmap move')
  assert.ok(out.includes('lenient'), 'must mention lenient mode so users understand why validate passes but ship aborts')
})

// ────────────────────────────────────────────────────────────────────────────
// Step 4 — Nothing staged
// ────────────────────────────────────────────────────────────────────────────

test('ship: nothing staged aborts', () => {
  const { deps, cap } = makeDeps({ branch: 'feat/my-feature', staged: '' })
  const code = runShip(makeOpts(), deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('nothing is staged'), 'must mention nothing is staged')
})

// ────────────────────────────────────────────────────────────────────────────
// Step 5 — Missing commit message
// ────────────────────────────────────────────────────────────────────────────

test('ship: no -m aborts', () => {
  const { deps, cap } = makeDeps({ branch: 'feat/my-feature', staged: 'file.js' })
  const code = runShip(makeOpts({ message: '' }), deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('commit message is required'), 'must mention commit message required')
})

// ────────────────────────────────────────────────────────────────────────────
// --dry-run: no write commands sent to execGit
// ────────────────────────────────────────────────────────────────────────────

test('ship: dry-run does not call execGit with write commands', () => {
  const git = makeMockGit({ branch: 'feat/dry-feature', stagedFiles: 'file.js' })
  const cap = captureOutput()
  const deps = {
    execGit: git,
    checkGovernance: () => [],
    writeln: cap.writeln,
  }

  const code = runShip(makeOpts({ dryRun: true }), deps)
  assert.equal(code, 0, 'dry-run should succeed')

  for (const call of git.calls) {
    if (call.length > 0 && GIT_WRITE_COMMANDS.has(call[0])) {
      assert.fail(`dry-run must not execute write command via execGit: git ${call.join(' ')}`)
    }
  }

  assert.ok(cap.output().includes('[dry-run]'), 'dry-run output must contain [dry-run] markers')
})

// ────────────────────────────────────────────────────────────────────────────
// Source-level guarantee: git add . / git add -A must not appear in runner.js
// ────────────────────────────────────────────────────────────────────────────

test('ship: runner.js source has no git add . or git add -A', () => {
  const runnerPath = path.join(__dirname, '../src/ship/runner.js')
  const src = fs.readFileSync(runnerPath, 'utf8')

  // Check for argument patterns that would indicate a real git add call.
  // Quoted doc-string occurrences use single quotes and won't match these patterns.
  const forbidden = ["'add', '.'", "'add', '-A'", '"add", "."', '"add", "-A"']
  for (const bad of forbidden) {
    assert.ok(!src.includes(bad), `runner.js must not contain ${bad}`)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// Runtime guarantee: execGit never receives add . or add -A
// ────────────────────────────────────────────────────────────────────────────

test('ship: execGit never receives git add . or git add -A', () => {
  const git = makeMockGit({ branch: 'feat/safe-check', stagedFiles: 'file.js' })
  const deps = {
    execGit: git,
    checkGovernance: () => [],
    writeln: () => {},
  }

  runShip(makeOpts({ dryRun: true }), deps)

  for (const call of git.calls) {
    if (call.length >= 2 && call[0] === 'add' && (call[1] === '.' || call[1] === '-A')) {
      assert.fail(`execGit received forbidden call: git ${call.join(' ')}`)
    }
  }
})

// ────────────────────────────────────────────────────────────────────────────
// isShipBranch unit tests
// ────────────────────────────────────────────────────────────────────────────

test('isShipBranch: valid branches', () => {
  for (const b of ['feat/foo', 'feat/a-very-long-slug', 'fix/123', 'refactor/clean-up']) {
    assert.ok(isShipBranch(b), `${b} should be valid`)
  }
})

test('isShipBranch: invalid branches', () => {
  for (const b of ['main', 'master', 'feature/foo', 'hotfix/bar', 'feat/', 'refactor/']) {
    assert.ok(!isShipBranch(b), `${b} should be invalid`)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// isGitWriteCmd unit tests
// ────────────────────────────────────────────────────────────────────────────

test('isGitWriteCmd: write commands', () => {
  for (const args of [
    ['commit', '-m', 'msg'],
    ['push', 'origin', 'feat/foo'],
    ['push', '-u', 'origin', 'feat/foo'],
    ['fetch', 'origin', '--prune'],
  ]) {
    assert.ok(isGitWriteCmd(args), `${args.join(' ')} should be a write command`)
  }
})

test('isGitWriteCmd: read-only commands', () => {
  for (const args of [
    ['status', '--short'],
    ['diff', '--cached', '--stat'],
    ['branch', '-r', '--no-merged'],
    ['symbolic-ref', '--short', 'HEAD'],
    ['log', '-1'],
  ]) {
    assert.ok(!isGitWriteCmd(args), `${args.join(' ')} should be read-only`)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// normalizeBranchSlug unit tests
// ────────────────────────────────────────────────────────────────────────────

test('normalizeBranchSlug: converts slug correctly', () => {
  assert.equal(normalizeBranchSlug('my-feature'), 'my-feature')
  assert.equal(normalizeBranchSlug('My Feature'), 'my-feature')
  assert.equal(normalizeBranchSlug('foo_bar.baz'), 'foo-bar-baz')
  assert.equal(normalizeBranchSlug('ABC123'), 'abc123')
})
