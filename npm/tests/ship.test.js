'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { runShip, isShipBranch, isGitWriteCmd, normalizeBranchSlug, GIT_WRITE_COMMANDS, buildForgeCreateArgs, firstLine } = require('../src/ship/runner')

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

function makeDeps({ branch, staged, violations = [], configForge = '', repoDir = '', availFn = null, execForgeCLI = null } = {}) {
  const git = makeMockGit({ branch, stagedFiles: staged })
  const cap = captureOutput()
  const deps = {
    execGit: git,
    checkGovernance: () => violations,
    writeln: cap.writeln,
    configForge,
    repoDir,
    availFn: availFn || (() => false),
    execForgeCLI: execForgeCLI || (() => null),
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

// ────────────────────────────────────────────────────────────────────────────
// Step 7 — forge resolution and PR/MR opening
// ────────────────────────────────────────────────────────────────────────────

function makeStep7Deps({ configForge = '', forgeFlag = '', availFn = null } = {}) {
  const cliCalls = []
  const mockExecForgeCLI = (name, args) => { cliCalls.push({ name, args }); return null }
  const { deps, cap } = makeDeps({
    branch: 'feat/my-feature',
    staged: 'file.js',
    configForge,
    repoDir: '',
    availFn: availFn || (() => false),
    execForgeCLI: mockExecForgeCLI,
  })
  const opts = { message: 'feat(x): test step7', dryRun: false, noPR: false, forge: forgeFlag }
  return { deps, cap, opts, cliCalls }
}

test('ship step7: gitlab outputs Merge Request', () => {
  const { deps, cap, opts } = makeStep7Deps({ configForge: 'gitlab' })
  const code = runShip(opts, deps)
  assert.equal(code, 0)
  assert.ok(cap.output().includes('Merge Request'), `expected Merge Request, got: ${cap.output()}`)
})

test('ship step7: github outputs Pull Request', () => {
  const { deps, cap, opts } = makeStep7Deps({ configForge: 'github' })
  const code = runShip(opts, deps)
  assert.equal(code, 0)
  assert.ok(cap.output().includes('Pull Request'), `expected Pull Request, got: ${cap.output()}`)
})

test('ship step7: CLI unavailable → exit 0, print URL', () => {
  const cliCalls = []
  const git = makeMockGit({ branch: 'feat/my-feature', stagedFiles: 'file.js' })
  const origGit = git
  // Override to return a fake remote URL
  function execGitWithRemote(args) {
    if (args.join(' ').startsWith('remote get-url')) {
      return { stdout: 'https://github.com/org/repo.git', error: null }
    }
    return origGit(args)
  }
  const cap = captureOutput()
  const deps = {
    execGit: execGitWithRemote,
    checkGovernance: () => [],
    writeln: cap.writeln,
    configForge: 'github',
    repoDir: '',
    availFn: () => false,
    execForgeCLI: (name, args) => { cliCalls.push({ name, args }); return null },
  }
  const code = runShip({ message: 'feat: x', dryRun: false, noPR: false, forge: '' }, deps)
  assert.equal(code, 0)
  assert.equal(cliCalls.length, 0, 'execForgeCLI must not be called when CLI unavailable')
  assert.ok(cap.output().includes('github.com'), `expected fallback URL, got: ${cap.output()}`)
})

test('ship step7: manual forge → exit 0, no CLI called', () => {
  const { deps, cap, opts, cliCalls } = makeStep7Deps({ configForge: '', forgeFlag: '' })
  const code = runShip(opts, deps)
  assert.equal(code, 0)
  assert.equal(cliCalls.length, 0, 'execForgeCLI must not be called for manual forge')
  assert.ok(cap.output().includes('ship complete'), `expected ship complete, got: ${cap.output()}`)
})

test('ship step7: --no-pr skips PR creation', () => {
  const { deps, cap, opts, cliCalls } = makeStep7Deps({ configForge: 'github', availFn: () => true })
  opts.noPR = true
  const code = runShip(opts, deps)
  assert.equal(code, 0)
  assert.equal(cliCalls.length, 0, 'execForgeCLI must not be called with --no-pr')
  assert.ok(cap.output().includes('--no-pr'), `expected --no-pr message, got: ${cap.output()}`)
  assert.ok(cap.output().includes('ship complete'), `expected ship complete, got: ${cap.output()}`)
})

test('ship step7: --forge overrides config', () => {
  const { deps, cap, opts } = makeStep7Deps({ configForge: '', forgeFlag: 'github' })
  const code = runShip(opts, deps)
  assert.equal(code, 0)
  assert.ok(cap.output().includes('github (source: flag)'), `expected source: flag, got: ${cap.output()}`)
})

test('ship step7: dry-run does not call execForgeCLI', () => {
  const { deps, cap, opts, cliCalls } = makeStep7Deps({ configForge: 'github', availFn: () => true })
  opts.dryRun = true
  const code = runShip(opts, deps)
  assert.equal(code, 0)
  assert.equal(cliCalls.length, 0, 'execForgeCLI must not be called in dry-run mode')
  const out = cap.output()
  assert.ok(out.includes('[dry-run]') || out.includes('would open'), `expected dry-run marker, got: ${out}`)
})

test('ship step7: resolution source in output', () => {
  const { deps, cap, opts } = makeStep7Deps({ configForge: 'gitlab' })
  runShip(opts, deps)
  assert.ok(cap.output().includes('source: config'), `expected source: config, got: ${cap.output()}`)
})

test('ship step7: CLI available → execForgeCLI invoked', () => {
  const { deps, cap, opts, cliCalls } = makeStep7Deps({ configForge: 'github', availFn: () => true })
  const code = runShip(opts, deps)
  assert.equal(code, 0)
  assert.equal(cliCalls.length, 1, 'execForgeCLI must be called exactly once')
  assert.equal(cliCalls[0].name, 'gh')
  assert.ok(cliCalls[0].args.includes('--title'), `expected --title in args, got: ${JSON.stringify(cliCalls[0].args)}`)
})

// ────────────────────────────────────────────────────────────────────────────
// buildForgeCreateArgs unit tests
// ────────────────────────────────────────────────────────────────────────────

test('buildForgeCreateArgs: github uses --body', () => {
  const { forgeAdapter } = require('../src/forge/adapter')
  const adapter = forgeAdapter('github', () => false)
  const args = buildForgeCreateArgs(adapter, 'my title', 'my body')
  assert.deepEqual(args, ['pr', 'create', '--title', 'my title', '--body', 'my body'])
})

test('buildForgeCreateArgs: azure uses --description', () => {
  const { forgeAdapter } = require('../src/forge/adapter')
  const adapter = forgeAdapter('azure', () => false)
  const args = buildForgeCreateArgs(adapter, 'my title', 'my body')
  assert.ok(!args.includes('--body'), 'azure must not use --body')
  assert.ok(args.includes('--description'), 'azure must use --description')
})

test('buildForgeCreateArgs: does not mutate adapter.cliArgs', () => {
  const { forgeAdapter } = require('../src/forge/adapter')
  const adapter = forgeAdapter('gitlab', () => false)
  const original = adapter.cliArgs.slice()
  buildForgeCreateArgs(adapter, 't1', 'b1')
  buildForgeCreateArgs(adapter, 't2', 'b2')
  assert.deepEqual(adapter.cliArgs, original, 'adapter.cliArgs must not be mutated')
})

test('firstLine: returns only first line', () => {
  assert.equal(firstLine('feat(x): title\n\nmore body'), 'feat(x): title')
  assert.equal(firstLine('no newline'), 'no newline')
  assert.equal(firstLine(''), '')
})
