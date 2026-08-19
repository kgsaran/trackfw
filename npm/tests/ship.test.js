'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const os = require('node:os')
const fs = require('node:fs')
const path = require('node:path')
const { spawnSync } = require('node:child_process')
const {
  runShip, isShipBranch, isGatedShipBranch, isGitWriteCmd, normalizeBranchSlug, resolveRoadmapDir, resetConfig,
  GIT_WRITE_COMMANDS, buildForgeCreateArgs, firstLine, allDocOnly, defaultBaseBranch,
  gitCommitsSince, buildPRBody, COMMIT_MESSAGE_SEP, detectPendingSquashMerges,
} = require('../src/ship/runner')
const shipCommand = require('../src/commands/ship')

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

/**
 * mockExecGit builds an execGit mock that captures calls and responds
 * based on the configured branch, staged state, and optional remote URL.
 */
function makeMockGit({ branch = '', stagedFiles = '', remoteURL = '', baseRef = '', commitLog = '' } = {}) {
  const calls = []
  function execGit(args) {
    calls.push(args.slice())
    const joined = args.join(' ')

    if (joined.startsWith('symbolic-ref --short')) {
      if (!branch) return { stdout: '', error: new Error('not a git repo') }
      return { stdout: branch, error: null }
    }
    if (joined.startsWith('symbolic-ref refs/remotes/origin/HEAD')) {
      if (!baseRef) return { stdout: '', error: new Error('no remote-tracking HEAD') }
      return { stdout: baseRef, error: null }
    }
    if (joined.startsWith('diff --cached --name-only')) {
      return { stdout: stagedFiles, error: null }
    }
    if (joined.startsWith('log ')) {
      return { stdout: commitLog, error: null }
    }
    if (joined.startsWith('remote get-url')) {
      return { stdout: remoteURL, error: remoteURL ? null : new Error('no remote') }
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

// captureOutput splits writeln (stdout) and writeErr (stderr) into separate buffers, but
// output() concatenates both (stdout lines then stderr lines) so the many pre-existing
// substring assertions below stay meaningful regardless of which stream a given message lands
// on — they only prove the text is present, not which stream it's on. Tests that specifically
// prove the ML-1B stream-routing fix (stderr + "Error: " prefix) use outLines/errLines directly.
function captureOutput() {
  const outLines = []
  const errLines = []
  return {
    writeln: (s) => outLines.push(s),
    writeErr: (s) => errLines.push(`Error: ${s}`),
    outLines,
    errLines,
    lines: outLines,
    output: () => [...outLines, ...errLines].join('\n'),
  }
}

function makeDeps({ branch, staged, violations = [], configForge = '', repoDir = '', availFn = null, execForgeCLI = null, remoteURL = '', baseRef = '', commitLog = '', checkPROpen = null } = {}) {
  const git = makeMockGit({ branch, stagedFiles: staged, remoteURL, baseRef, commitLog })
  const cap = captureOutput()
  const deps = {
    execGit: git,
    checkGovernance: () => violations,
    writeln: cap.writeln,
    writeErr: cap.writeErr,
    configForge,
    repoDir,
    availFn: availFn || (() => false),
    execForgeCLI: execForgeCLI || (() => null),
    checkPROpen: checkPROpen || undefined,
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
  for (const branch of ['feature/foo', 'hotfix/bar', 'chores/typo', 'mybranch']) {
    const { deps, cap } = makeDeps({ branch, staged: 'file.js' })
    const code = runShip(makeOpts(), deps)
    assert.equal(code, 1, `${branch} should abort`)
    assert.ok(cap.output().includes('does not match the required pattern'), `${branch}: wrong error`)
  }
})

test('ship: valid branch patterns not rejected at step 1', () => {
  for (const branch of ['feat/my-feature', 'fix/bug-123', 'refactor/clean-up', 'chore/release-x.y.z', 'docs/update-readme']) {
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
// ML-1B — error stream/prefix parity: the final one-line summary of every abort path goes to
// stderr with the "Error: " prefix (mirrors Go's cobra/root.go behavior exactly); everything
// printed before it (violation lists, remediation hints) stays on stdout with no lowercase
// "error: " leaking in. Byte-level cross-runtime parity for these scenarios is proven by
// scripts/check-ship-parity.sh.
// ────────────────────────────────────────────────────────────────────────────

test('ship: governance violation — final summary lands on stderr with "Error: " prefix, not stdout', () => {
  const violations = ['branch "feat/foo" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/']
  const { deps, cap } = makeDeps({ branch: 'feat/foo', staged: 'file.js', violations })
  const code = runShip(makeOpts(), deps)
  assert.equal(code, 1)
  assert.equal(cap.errLines.length, 1, 'exactly one line must be written to stderr')
  assert.equal(cap.errLines[0], 'Error: governance check failed: 1 violation(s)')
  assert.ok(!cap.outLines.join('\n').includes('governance check failed'), 'the summary must NOT also appear on stdout')
  for (const l of cap.outLines) {
    assert.ok(!/(^|\n)error: /.test(l), `stdout line must not start with lowercase "error: ": ${l}`)
  }
})

test('ship: branch-pattern mismatch — Step 1 error lands on stderr with "Error: " prefix, not stdout', () => {
  const { deps, cap } = makeDeps({ branch: 'hotfix/whatever', staged: 'file.js' })
  const code = runShip(makeOpts(), deps)
  assert.equal(code, 1)
  assert.equal(cap.errLines.length, 1, 'exactly one line must be written to stderr')
  assert.ok(cap.errLines[0].startsWith('Error: branch "hotfix/whatever" does not match the required pattern'))
  assert.equal(cap.outLines.length, 0, 'nothing should have been written to stdout before this abort')
})

// ────────────────────────────────────────────────────────────────────────────
// Doc-only exception — Steps 1 & 2 skip branch-pattern and governance checks
// ────────────────────────────────────────────────────────────────────────────

test('ship: doc-only change on non-conforming branch name is allowed', () => {
  // "docs/foo" does not match feat|fix|refactor/<slug> — normally rejected by isShipBranch,
  // but every staged file is doc-only, so Step 1's branch-pattern check must be skipped.
  const violations = ['should never be called']
  const { deps } = makeDeps({ branch: 'docs/foo', staged: 'docs/some-note.md', violations })
  const code = runShip(makeOpts({ message: 'docs: update note', dryRun: true }), deps)
  assert.equal(code, 0, 'doc-only change on non-conforming branch name should not be blocked')
})

test('ship: doc-only change with missing wip roadmap skips governance entirely', () => {
  // feat/<slug> is a correctly named branch, but checkGovernance would fail — doc-only
  // staged content must skip governance entirely, never calling checkGovernance.
  let called = false
  const git = makeMockGit({ branch: 'feat/doc-fix', stagedFiles: 'docs/req/REQ-x.md\nvault/notes/note.md' })
  const cap = captureOutput()
  const deps = {
    execGit: git,
    checkGovernance: () => { called = true; return ['no matching roadmap in wip/ nor done/'] },
    writeln: cap.writeln,
    availFn: () => false,
    execForgeCLI: () => null,
  }
  const code = runShip(makeOpts({ message: 'docs: fix req', dryRun: true }), deps)
  assert.equal(code, 0, 'doc-only change must not be blocked by governance')
  assert.equal(called, false, 'checkGovernance must not be called at all for a doc-only change')
  assert.ok(cap.output().includes('Governance: skipped (doc-only change)'), `expected doc-only skip message, got:\n${cap.output()}`)
})

test('ship: mixed doc+code on feat branch is still blocked by governance', () => {
  const violations = ['branch "feat/mixed" is a feat/fix/refactor branch but no roadmap is in wip/']
  const { deps, cap } = makeDeps({ branch: 'feat/mixed', staged: 'docs/note.md\ninternal/commands/ship.go', violations })
  const code = runShip(makeOpts({ message: 'feat: mixed change' }), deps)
  assert.equal(code, 1, 'expected governance error for a mixed doc+code change')
  const out = cap.output()
  assert.ok(out.includes('governance check failed'), 'must mention governance check failed')
  assert.ok(!out.includes('skipped (doc-only change)'), 'mixed doc+code change must not be treated as doc-only')
})

test('ship: mixed doc+code on non-conforming branch name is still blocked', () => {
  // Branch name outside the ship vocabulary entirely (feat/fix/refactor/chore/docs) — must still
  // fail Step 1's branch-pattern check.
  const { deps, cap } = makeDeps({ branch: 'hotfix/mixed', staged: 'docs/note.md\ninternal/commands/ship.go' })
  const code = runShip(makeOpts({ message: 'fix: mixed change' }), deps)
  assert.equal(code, 1, 'expected branch-pattern error for a mixed doc+code change on a non-conforming branch')
  assert.ok(cap.output().includes('does not match the required pattern'))
})

// ────────────────────────────────────────────────────────────────────────────
// chore/docs branch-type exception — Step 2 skips governance regardless of staged content
// ────────────────────────────────────────────────────────────────────────────

test('ship: chore branch with mixed content skips governance', () => {
  // "chore/release-x.y.z" carries a non-doc file staged (not doc-only) — proves the skip is
  // keyed on branch type, not on the pre-existing doc-only staged-content exception.
  let called = false
  const git = makeMockGit({ branch: 'chore/release-x.y.z', stagedFiles: 'internal/commands/ship.go' })
  const cap = captureOutput()
  const deps = {
    execGit: git,
    checkGovernance: () => { called = true; return ['should never be called'] },
    writeln: cap.writeln,
    availFn: () => false,
    execForgeCLI: () => null,
  }
  const code = runShip(makeOpts({ message: 'chore: release x.y.z', dryRun: true }), deps)
  assert.equal(code, 0, 'chore branch must not be blocked by governance')
  assert.equal(called, false, 'checkGovernance must not be called at all for a chore/docs branch')
  assert.ok(cap.output().includes('Governance: skipped (chore/docs branch)'), `expected chore/docs branch-type skip message, got:\n${cap.output()}`)
})

test('ship: docs branch with mixed content skips governance', () => {
  let called = false
  const git = makeMockGit({ branch: 'docs/update-readme', stagedFiles: 'docs/note.md\ninternal/commands/ship.go' })
  const cap = captureOutput()
  const deps = {
    execGit: git,
    checkGovernance: () => { called = true; return ['should never be called'] },
    writeln: cap.writeln,
    availFn: () => false,
    execForgeCLI: () => null,
  }
  const code = runShip(makeOpts({ message: 'docs: update readme', dryRun: true }), deps)
  assert.equal(code, 0, 'docs branch must not be blocked by governance')
  assert.equal(called, false, 'checkGovernance must not be called at all for a chore/docs branch')
  assert.ok(cap.output().includes('Governance: skipped (chore/docs branch)'), `expected chore/docs branch-type skip message, got:\n${cap.output()}`)
})

test('ship: feat branch without roadmap is still hard-gated (non-regression)', () => {
  const violations = ['branch "feat/no-roadmap" is a feat/fix/refactor branch but no roadmap is in wip/']
  const { deps, cap } = makeDeps({ branch: 'feat/no-roadmap', staged: 'file.js', violations })
  const code = runShip(makeOpts({ dryRun: true }), deps)
  assert.equal(code, 1, 'expected governance error — feat/fix/refactor must still be gated')
  const out = cap.output()
  assert.ok(out.includes('governance check failed'), 'must mention governance check failed')
  assert.ok(!out.includes('Governance: skipped'), 'feat branch must never print a governance-skipped message')
})

// ────────────────────────────────────────────────────────────────────────────
// allDocOnly unit tests
// ────────────────────────────────────────────────────────────────────────────

test('allDocOnly: doc-only file sets return true', () => {
  const docOnlyCases = [
    ['docs/req/REQ-x.md'],
    ['vault/notes/note.md'],
    ['README.md'],
    ['docs/req/REQ-x.md', 'vault/notes/note.md', 'CHANGELOG.md'],
  ]
  for (const files of docOnlyCases) {
    assert.ok(allDocOnly(files), `allDocOnly(${JSON.stringify(files)}) should be true`)
  }
})

test('allDocOnly: mixed or empty file sets return false', () => {
  const notDocOnlyCases = [
    undefined,
    [],
    ['internal/commands/ship.go'],
    ['docs/req/REQ-x.md', 'internal/commands/ship.go'],
    ['go.mod'],
  ]
  for (const files of notDocOnlyCases) {
    assert.ok(!allDocOnly(files), `allDocOnly(${JSON.stringify(files)}) should be false`)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// defaultBaseBranch unit tests
// ────────────────────────────────────────────────────────────────────────────

test('defaultBaseBranch: symbolic-ref succeeds', () => {
  const execGit = () => ({ stdout: 'refs/remotes/origin/develop', error: null })
  assert.equal(defaultBaseBranch(execGit), 'develop')
})

test('defaultBaseBranch: symbolic-ref fails falls back to main', () => {
  const execGit = () => ({ stdout: '', error: new Error('no remote-tracking HEAD') })
  assert.equal(defaultBaseBranch(execGit), 'main')
})

test('defaultBaseBranch: empty output falls back to main', () => {
  const execGit = () => ({ stdout: '', error: null })
  assert.equal(defaultBaseBranch(execGit), 'main')
})

// ────────────────────────────────────────────────────────────────────────────
// buildPRBody unit tests
// ────────────────────────────────────────────────────────────────────────────

test('buildPRBody: zero or one commit keeps minimal body', () => {
  for (const commits of [[], ['feat: single commit']]) {
    const body = buildPRBody('feat/my-feature', commits)
    assert.equal(body, 'Branch: feat/my-feature\n\nCreated by trackfw ship.')
  }
})

test('buildPRBody: multiple commits aggregates history', () => {
  const commits = [
    'feat(ship): add doc-only exception\n\nSkips governance for docs/vault/md-only staged files.',
    'fix(ship): correct base branch fallback',
    'docs: update roadmap status',
  ]
  const body = buildPRBody('feat/my-feature', commits)

  assert.ok(body.includes('## Commits'), `expected '## Commits' heading, got:\n${body}`)
  for (const subject of [
    '- feat(ship): add doc-only exception',
    '- fix(ship): correct base branch fallback',
    '- docs: update roadmap status',
  ]) {
    assert.ok(body.includes(subject), `expected subject line ${subject}, got:\n${body}`)
  }
  assert.ok(body.includes('## Detalhes'), `expected '## Detalhes' heading, got:\n${body}`)
  assert.ok(body.includes('Skips governance for docs/vault/md-only staged files.'), `expected full commit body, got:\n${body}`)
  assert.ok(body.includes('---\nBranch: feat/my-feature'), `expected trailing footer, got:\n${body}`)
})

// ────────────────────────────────────────────────────────────────────────────
// gitCommitsSince unit tests
// ────────────────────────────────────────────────────────────────────────────

test('gitCommitsSince: parses separated commits', () => {
  const commitLog = 'feat: first' + COMMIT_MESSAGE_SEP + 'fix: second\n\nwith a body' + COMMIT_MESSAGE_SEP
  const execGit = () => ({ stdout: commitLog, error: null })
  const commits = gitCommitsSince('main', execGit)
  assert.equal(commits.length, 2)
  assert.equal(commits[0], 'feat: first')
  assert.equal(commits[1], 'fix: second\n\nwith a body')
})

test('gitCommitsSince: empty range returns empty array', () => {
  const execGit = () => ({ stdout: '', error: null })
  const commits = gitCommitsSince('main', execGit)
  assert.deepEqual(commits, [])
})

// ────────────────────────────────────────────────────────────────────────────
// End-to-end: --dry-run PR body reflects real branch commit history
// ────────────────────────────────────────────────────────────────────────────

test('ship: dry-run PR body aggregates commit history', () => {
  const commitLog = 'feat(x): third commit' + COMMIT_MESSAGE_SEP + 'feat(x): second commit' + COMMIT_MESSAGE_SEP
  const { deps, cap } = makeDeps({
    branch: 'feat/my-feature',
    staged: 'file.go',
    remoteURL: 'https://github.com/org/repo.git',
    baseRef: 'refs/remotes/origin/main',
    commitLog,
    configForge: 'github',
  })
  const code = runShip(makeOpts({ message: 'feat(x): first commit (this ship call)', dryRun: true }), deps)
  assert.equal(code, 0)
  const out = cap.output()
  assert.ok(out.includes('[dry-run] Title: feat(x): first commit (this ship call)'), `expected dry-run title line, got:\n${out}`)
  assert.ok(out.includes('## Commits') && out.includes('feat(x): third commit'), `expected aggregated commit history in dry-run body, got:\n${out}`)
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
  for (const b of ['feat/foo', 'feat/a-very-long-slug', 'fix/123', 'refactor/clean-up', 'chore/x', 'docs/x']) {
    assert.ok(isShipBranch(b), `${b} should be valid`)
  }
})

test('isShipBranch: invalid branches', () => {
  for (const b of ['main', 'master', 'feature/foo', 'hotfix/bar', 'feat/', 'refactor/', 'chore/', 'docs/']) {
    assert.ok(!isShipBranch(b), `${b} should be invalid`)
  }
})

test('isGatedShipBranch: gated branches', () => {
  for (const b of ['feat/foo', 'fix/123', 'refactor/clean-up']) {
    assert.ok(isGatedShipBranch(b), `${b} should be gated`)
  }
})

test('isGatedShipBranch: not gated', () => {
  for (const b of ['chore/x', 'docs/x', 'main', 'feature/foo', 'chore/', 'docs/']) {
    assert.ok(!isGatedShipBranch(b), `${b} should not be gated`)
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

// ────────────────────────────────────────────────────────────────────────────
// Parity test — resolveRoadmapDir default must be docs/roadmaps (not docs/roadmaps/claude)
// Locks the default across all runtimes: Go, Node.js, Python all use docs/roadmaps.
// ────────────────────────────────────────────────────────────────────────────

test('resolveRoadmapDir: default is docs/roadmaps when no trackfw.yaml present', () => {
  const os = require('os')
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-parity-npm-'))
  try {
    resetConfig()
    const result = resolveRoadmapDir(tmpDir)
    assert.equal(result, 'docs/roadmaps',
      `default roadmap_dir must be "docs/roadmaps", got "${result}" — parity lock violated`)
  } finally {
    resetConfig() // clean singleton so subsequent tests are unaffected
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
})

// ────────────────────────────────────────────────────────────────────────────
// Forge matrix — 4 forges × 2 avail states × 2 host types (16 cells)
// All cells run with --dry-run to skip real push.
// ────────────────────────────────────────────────────────────────────────────

const KNOWN_URLS = {
  github:    'https://github.com/org/repo.git',
  gitlab:    'https://gitlab.com/org/repo.git',
  bitbucket: 'https://bitbucket.org/org/repo.git',
  azure:     'https://dev.azure.com/org/proj/_git/repo',
}
const SELF_HOSTED_URL = 'https://git.mycompany.com/org/repo.git'

const forgeMatrix = [
  // github × known host
  { forge: 'github',    cliPresent: true,  remoteURL: KNOWN_URLS.github,    noun: 'Pull Request',  checkOutput: out => out.includes('[dry-run] would open Pull Request via github CLI') },
  { forge: 'github',    cliPresent: false, remoteURL: KNOWN_URLS.github,    noun: 'Pull Request',  checkOutput: out => out.includes('github.com') },
  // github × self-hosted
  { forge: 'github',    cliPresent: true,  remoteURL: SELF_HOSTED_URL,      noun: 'Pull Request',  checkOutput: out => out.includes('[dry-run] would open Pull Request via github CLI') },
  { forge: 'github',    cliPresent: false, remoteURL: SELF_HOSTED_URL,      noun: 'Pull Request',  checkOutput: out => out.includes('mycompany.com') },
  // gitlab × known host
  { forge: 'gitlab',    cliPresent: true,  remoteURL: KNOWN_URLS.gitlab,    noun: 'Merge Request', checkOutput: out => out.includes('[dry-run] would open Merge Request via gitlab CLI') },
  { forge: 'gitlab',    cliPresent: false, remoteURL: KNOWN_URLS.gitlab,    noun: 'Merge Request', checkOutput: out => out.includes('gitlab.com') },
  // gitlab × self-hosted
  { forge: 'gitlab',    cliPresent: true,  remoteURL: SELF_HOSTED_URL,      noun: 'Merge Request', checkOutput: out => out.includes('[dry-run] would open Merge Request via gitlab CLI') },
  { forge: 'gitlab',    cliPresent: false, remoteURL: SELF_HOSTED_URL,      noun: 'Merge Request', checkOutput: out => out.includes('mycompany.com') },
  // bitbucket × known host (bitbucket has no CLI — cliPresent is irrelevant, always absent)
  { forge: 'bitbucket', cliPresent: true,  remoteURL: KNOWN_URLS.bitbucket, noun: 'Pull Request',  checkOutput: out => out.includes('bitbucket.org') },
  { forge: 'bitbucket', cliPresent: false, remoteURL: KNOWN_URLS.bitbucket, noun: 'Pull Request',  checkOutput: out => out.includes('bitbucket.org') },
  // bitbucket × self-hosted
  { forge: 'bitbucket', cliPresent: true,  remoteURL: SELF_HOSTED_URL,      noun: 'Pull Request',  checkOutput: out => out.includes('mycompany.com') },
  { forge: 'bitbucket', cliPresent: false, remoteURL: SELF_HOSTED_URL,      noun: 'Pull Request',  checkOutput: out => out.includes('mycompany.com') },
  // azure × known host
  { forge: 'azure',     cliPresent: true,  remoteURL: KNOWN_URLS.azure,     noun: 'Pull Request',  checkOutput: out => out.includes('[dry-run] would open Pull Request via azure CLI') },
  { forge: 'azure',     cliPresent: false, remoteURL: KNOWN_URLS.azure,     noun: 'Pull Request',  checkOutput: out => out.includes('dev.azure.com') },
  // azure × self-hosted
  { forge: 'azure',     cliPresent: true,  remoteURL: SELF_HOSTED_URL,      noun: 'Pull Request',  checkOutput: out => out.includes('[dry-run] would open Pull Request via azure CLI') },
  { forge: 'azure',     cliPresent: false, remoteURL: SELF_HOSTED_URL,      noun: 'Pull Request',  checkOutput: out => out.includes('mycompany.com') },
]

for (const tc of forgeMatrix) {
  const label = `forge matrix: ${tc.forge} × ${tc.cliPresent ? 'cli-present' : 'cli-absent'} × ${tc.remoteURL.includes('mycompany') ? 'self-hosted' : 'known-host'}`
  test(label, () => {
    const cliCalls = []
    const { deps, cap } = makeDeps({
      branch: 'feat/my-feature',
      staged: 'file.js',
      configForge: tc.forge,
      remoteURL: tc.remoteURL,
      availFn: () => tc.cliPresent,
      execForgeCLI: (name, args) => { cliCalls.push({ name, args }); return null },
    })
    const opts = { message: 'feat(x): matrix test', dryRun: true, noPR: false, forge: '' }
    const code = runShip(opts, deps)
    const out = cap.output()

    assert.equal(code, 0, `expected exit 0, got ${code}\noutput: ${out}`)
    assert.ok(
      out.includes(`Forge:     ${tc.forge} (source: config)`),
      `expected forge line "Forge:     ${tc.forge} (source: config)", got: ${out}`
    )
    assert.ok(out.includes(tc.noun), `expected noun "${tc.noun}" in output, got: ${out}`)
    assert.ok(tc.checkOutput(out), `expected condition not met for ${label}\noutput: ${out}`)
    assert.equal(cliCalls.length, 0, `dry-run must not invoke execForgeCLI, got ${cliCalls.length} calls`)
  })
}

// ────────────────────────────────────────────────────────────────────────────
// Silence usage — runtime errors must NOT show usage; parse errors must show it
// ────────────────────────────────────────────────────────────────────────────

test('silence-usage: runtime error (branch validation) does not show usage text', () => {
  const { deps, cap } = makeDeps({ branch: 'main', staged: 'file.js' })
  const code = runShip(makeOpts(), deps)
  assert.equal(code, 1)
  const out = cap.output()
  // Runtime errors from runShip never reach commander's usage printer — no "Usage:" line
  assert.ok(!out.includes('Usage:'), `runtime error must not print usage, got: ${out}`)
})

test('silence-usage: commander shows usage on unknown flag', () => {
  const { spawnSync } = require('child_process')
  const binPath = path.join(__dirname, '../bin/trackfw')
  const result = spawnSync(process.execPath, [binPath, 'ship', '--unknown-flag-xyz'], { encoding: 'utf8' })
  const out = (result.stdout || '') + (result.stderr || '')
  assert.ok(out.toLowerCase().includes('usage') || out.toLowerCase().includes('error'), `expected usage or error on unknown flag, got: ${out}`)
})

// ────────────────────────────────────────────────────────────────────────────
// --no-pr wiring: commander's negatable option stores .pr (not .noPr)
// ────────────────────────────────────────────────────────────────────────────

test('--no-pr wiring: commander negatable option sets options.pr=false', () => {
  const { spawnSync } = require('child_process')
  // Just verify the flag is accepted by the CLI without an "unknown option" error
  // (we can't test full ship flow without a real git repo, but commander parse must succeed)
  const binPath = path.join(__dirname, '../bin/trackfw')
  const result = spawnSync(process.execPath, [binPath, 'ship', '--no-pr', '--help'], { encoding: 'utf8' })
  const out = (result.stdout || '') + (result.stderr || '')
  // --help with --no-pr must not produce "unknown option" — confirms commander parsed it
  assert.ok(!out.includes('unknown option'), `--no-pr should be a known option, got: ${out}`)
})

// ────────────────────────────────────────────────────────────────────────────
// Integration test — --no-pr wiring at command layer (real subprocess)
// Discriminates against the bug where `options.noPr || false` made noPR always
// false, silently ignoring the --no-pr flag.
// ────────────────────────────────────────────────────────────────────────────

test('ship integration: --no-pr wiring reaches runner (command layer)', async () => {
  const { spawnSync } = require('child_process')
  const os = require('os')

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ship-nopr-'))
  try {
    const tmpBin = path.join(tmpDir, 'bin')
    fs.mkdirSync(tmpBin)
    const gitWhich = spawnSync('which', ['git'], { encoding: 'utf8' }).stdout.trim()
    if (!gitWhich) throw new Error('git not found in PATH')
    fs.symlinkSync(gitWhich, path.join(tmpBin, 'git'))

    const repoDir = path.join(tmpDir, 'repo')
    fs.mkdirSync(repoDir)

    const gitRun = (args) => spawnSync('git', args, { cwd: repoDir, encoding: 'utf8' })
    gitRun(['init'])
    gitRun(['config', 'user.email', 'test@example.com'])
    gitRun(['config', 'user.name', 'Test'])
    spawnSync('git', ['symbolic-ref', 'HEAD', 'refs/heads/feat/nopr-test'], { cwd: repoDir, encoding: 'utf8' })
    gitRun(['remote', 'add', 'origin', 'https://github.com/org/repo.git'])

    fs.writeFileSync(path.join(repoDir, 'staged.txt'), 'content\n')
    gitRun(['add', 'staged.txt'])

    const wipDir = path.join(repoDir, 'docs', 'roadmaps', 'wip')
    fs.mkdirSync(wipDir, { recursive: true })
    fs.writeFileSync(
      path.join(wipDir, 'ROADMAP-nopr-test.md'),
      'REQ: REQ-ship-nopr-test\n\n# Roadmap: --no-pr wiring test\n'
    )

    const binPath = path.resolve(__dirname, '../bin/trackfw')
    // --dry-run skips commit+push; --no-pr must fire in step 7 before dry-run fallback
    const result = spawnSync(
      process.execPath,
      [binPath, 'ship', '--dry-run', '--no-pr', '--forge', 'github', '-m', 'feat: nopr test'],
      {
        cwd: repoDir,
        encoding: 'utf8',
        env: {
          PATH: tmpBin,
          HOME: tmpDir,
          GIT_AUTHOR_NAME: 'Test',
          GIT_AUTHOR_EMAIL: 'test@example.com',
          GIT_COMMITTER_NAME: 'Test',
          GIT_COMMITTER_EMAIL: 'test@example.com',
        },
      }
    )

    const out = (result.stdout || '') + (result.stderr || '')
    assert.equal(result.status, 0, `expected exit 0, got ${result.status}\noutput: ${out}`)
    assert.ok(
      out.includes('--no-pr: skipping'),
      `expected "--no-pr: skipping" in output (wiring bug if absent), got: ${out}`
    )
    // Ensure no github.com URL was printed (noPR fired before dry-run URL block)
    assert.ok(
      !out.includes('github.com/compare'),
      `should not print fallback URL when --no-pr is set, got: ${out}`
    )
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
})

// ────────────────────────────────────────────────────────────────────────────
// Integration test — real binary (node) with clean PATH (only git)
// ────────────────────────────────────────────────────────────────────────────

test('ship integration: graceful degradation with clean PATH (no gh/glab/az)', async () => {
  const { spawnSync } = require('child_process')
  const os = require('os')

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ship-npm-'))
  try {
    // Build tmpBin with only git
    const tmpBin = path.join(tmpDir, 'bin')
    fs.mkdirSync(tmpBin)
    const gitWhich = spawnSync('which', ['git'], { encoding: 'utf8' }).stdout.trim()
    if (!gitWhich) throw new Error('git not found in PATH')
    fs.symlinkSync(gitWhich, path.join(tmpBin, 'git'))

    // Create git repo
    const repoDir = path.join(tmpDir, 'repo')
    fs.mkdirSync(repoDir)

    const gitRun = (args) => spawnSync('git', args, { cwd: repoDir, encoding: 'utf8' })

    gitRun(['init'])
    gitRun(['config', 'user.email', 'test@example.com'])
    gitRun(['config', 'user.name', 'Test'])
    // Set HEAD to feat/my-feature without committing
    spawnSync('git', ['symbolic-ref', 'HEAD', 'refs/heads/feat/my-feature'], { cwd: repoDir, encoding: 'utf8' })
    gitRun(['remote', 'add', 'origin', 'https://github.com/org/repo.git'])

    // Stage a file
    fs.writeFileSync(path.join(repoDir, 'staged.txt'), 'content\n')
    gitRun(['add', 'staged.txt'])

    // Create governance: wip roadmap with branch slug and REQ
    const wipDir = path.join(repoDir, 'docs', 'roadmaps', 'wip')
    fs.mkdirSync(wipDir, { recursive: true })
    fs.writeFileSync(
      path.join(wipDir, 'ROADMAP-my-feature-integration-test.md'),
      'REQ: REQ-ship-integration-test\n\n# Roadmap: Integration Test\n\nTest roadmap for graceful degradation proof.\n'
    )

    // Run npm CLI with explicit node path and clean PATH (no gh/glab/az)
    const binPath = path.resolve(__dirname, '../bin/trackfw')
    const result = spawnSync(
      process.execPath,
      [binPath, 'ship', '--dry-run', '--forge', 'github', '-m', 'feat: integration test'],
      {
        cwd: repoDir,
        encoding: 'utf8',
        env: {
          PATH: tmpBin,
          HOME: tmpDir,
          GIT_AUTHOR_NAME: 'Test',
          GIT_AUTHOR_EMAIL: 'test@example.com',
          GIT_COMMITTER_NAME: 'Test',
          GIT_COMMITTER_EMAIL: 'test@example.com',
        },
      }
    )

    const out = (result.stdout || '') + (result.stderr || '')
    assert.equal(result.status, 0, `expected exit 0, got ${result.status}\noutput: ${out}`)
    assert.ok(out.includes('github.com'), `expected github.com URL in output, got: ${out}`)
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
})

// ────────────────────────────────────────────────────────────────────────────
// ML-2A — detectPendingSquashMerges reuses evaluateBranchIntegration
// ────────────────────────────────────────────────────────────────────────────

test(
  'detectPendingSquashMerges: real git repo — stale-but-integrated vs genuinely pending (P4, AC7)',
  { timeout: 30000 },
  () => {
    const which = spawnSync('git', ['--version'])
    if (which.error) {
      return // git not available — skip like Go's t.Skip
    }

    const work = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ship-squash-node-'))
    const bareDir = path.join(work, 'origin.git')
    const cloneDir = path.join(work, 'clone')
    const emptyGitConfig = path.join(work, 'empty-gitconfig')
    fs.writeFileSync(emptyGitConfig, '')

    const env = () => ({
      ...process.env,
      GIT_CONFIG_GLOBAL: emptyGitConfig,
      GIT_CONFIG_SYSTEM: '/dev/null',
      GIT_TERMINAL_PROMPT: '0',
      HOME: work,
    })

    function run(dir, args) {
      const result = spawnSync('git', args, { cwd: dir, env: env(), encoding: 'utf8' })
      if (result.status !== 0) {
        throw new Error(`git ${args.join(' ')} (dir=${dir}) failed: ${result.stderr}\n${result.stdout}`)
      }
      return result.stdout
    }

    fs.mkdirSync(bareDir, { recursive: true })
    run(bareDir, ['init', '-q', '--bare', '-b', 'main'])

    fs.mkdirSync(cloneDir, { recursive: true })
    run(work, ['clone', '-q', bareDir, cloneDir])
    run(cloneDir, ['config', 'user.email', 'falsify@trackfw.test'])
    run(cloneDir, ['config', 'user.name', 'trackfw falsify'])
    run(cloneDir, ['config', 'commit.gpgsign', 'false'])
    run(cloneDir, ['config', 'core.hooksPath', '/dev/null'])

    function writeFile(name, content) {
      fs.writeFileSync(path.join(cloneDir, name), content)
    }

    writeFile('base.txt', 'base\n')
    run(cloneDir, ['add', 'base.txt'])
    run(cloneDir, ['commit', '-q', '-m', 'base commit'])
    run(cloneDir, ['push', '-q', 'origin', 'main'])

    // feat/a — pushed to origin, squash-merged into main (ancestry never records the merge).
    run(cloneDir, ['checkout', '-q', '-b', 'feat/a'])
    writeFile('a.txt', 'a\n')
    run(cloneDir, ['add', 'a.txt'])
    run(cloneDir, ['commit', '-q', '-m', 'feat/a work'])
    run(cloneDir, ['push', '-q', 'origin', 'feat/a'])
    run(cloneDir, ['checkout', '-q', 'main'])
    run(cloneDir, ['merge', '-q', '--squash', 'feat/a'])
    run(cloneDir, ['commit', '-q', '-m', 'squash-merge feat/a (PR #181)'])

    // main advances further (PR #182) — makes the naive bidirectional diff non-empty for the
    // already-integrated feat/a.
    writeFile('b.txt', 'b\n')
    run(cloneDir, ['add', 'b.txt'])
    run(cloneDir, ['commit', '-q', '-m', 'unrelated follow-up (PR #182)'])
    run(cloneDir, ['push', '-q', 'origin', 'main'])

    // feat/pending — pushed to origin, genuinely never merged anywhere.
    run(cloneDir, ['checkout', '-q', '-b', 'feat/pending'])
    writeFile('c.txt', 'c\n')
    run(cloneDir, ['add', 'c.txt'])
    run(cloneDir, ['commit', '-q', '-m', 'feat/pending work, never merged'])
    run(cloneDir, ['push', '-q', 'origin', 'feat/pending'])

    run(cloneDir, ['checkout', '-q', 'main'])
    run(cloneDir, ['fetch', '-q', 'origin'])

    function execGit(args) {
      const result = spawnSync('git', args, { cwd: cloneDir, env: env(), encoding: 'utf8' })
      if (result.status !== 0) {
        return { stdout: '', error: new Error((result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`) }
      }
      return { stdout: (result.stdout || '').trim(), error: null }
    }

    // P4 baseline: the naive bidirectional check IS non-empty for origin/feat/a — reproduces the
    // exact PR #181/#182 false positive.
    const naive = execGit(['diff', 'origin/main', 'origin/feat/a', '--stat'])
    assert.equal(naive.error, null)
    assert.notEqual(naive.stdout.trim(), '', 'test setup invalid: naive diff must be non-empty to reproduce the #181/#182 false positive')

    // P4 detection: the fixed detectPendingSquashMerges must not warn about feat/a and must still
    // warn about feat/pending.
    const lines = []
    detectPendingSquashMerges('main', execGit, (s) => lines.push(s))
    const got = lines.join('\n')

    assert.ok(!got.includes('"feat/a"'), `must NOT warn about feat/a (stale-but-integrated). Output:\n${got}`)
    assert.ok(got.includes('"feat/pending"'), `must still warn about feat/pending (genuinely unmerged). Output:\n${got}`)
  }
)

test('detectPendingSquashMerges: delegates to evaluateBranchIntegration, no own bidirectional diff', () => {
  const calls = {}
  function execGit(args) {
    const key = args.join(' ')
    calls[key] = (calls[key] || 0) + 1
    if (key === 'branch -r --no-merged origin/main') {
      return { stdout: '  origin/feat/unrelated-history\n', error: null }
    }
    if (key.startsWith('merge-base origin/main')) {
      return { stdout: '', error: new Error('fatal: no merge base') }
    }
    if (key.startsWith('diff origin/main')) {
      // A raw bidirectional-diff implementation would call this directly; the shared
      // evaluateBranchIntegration only reaches its own -z diffs after a successful merge-base,
      // which never happens here.
      return { stdout: 'some.file | 1 +\n', error: null }
    }
    return { stdout: '', error: null }
  }

  const lines = []
  detectPendingSquashMerges('main', execGit, (s) => lines.push(s))

  assert.equal(lines.length, 0, `expected no warning when merge-base fails, got:\n${lines.join('\n')}`)
  assert.equal(calls['diff origin/main origin/feat/unrelated-history --stat'], undefined,
    'detectPendingSquashMerges must not run its own bidirectional diff --stat anymore')
})

// ────────────────────────────────────────────────────────────────────────────
// --force-with-lease — ML-1A
// ────────────────────────────────────────────────────────────────────────────

function makeForceLeaseDeps({ staged = 'file.js', checkPROpen } = {}) {
  const { deps, git, cap } = makeDeps({
    branch: 'fix/rebase-test',
    staged,
    configForge: 'github',
    availFn: () => true,
    remoteURL: 'https://github.com/org/repo.git',
    checkPROpen,
  })
  const opts = { message: 'fix: rebase', forceWithLease: true }
  return { opts, deps, git, cap }
}

test('ship force-with-lease: open PR → succeeds, skips creation, uses correct push args', () => {
  const { opts, deps, git, cap } = makeForceLeaseDeps({
    checkPROpen: (adapter, branch) => {
      assert.equal(branch, 'fix/rebase-test')
      return true
    },
  })
  const code = runShip(opts, deps)
  assert.equal(code, 0, cap.output())

  const pushCall = git.calls.find((c) => c[0] === 'push')
  assert.deepEqual(pushCall, ['push', '--force-with-lease', '-u', 'origin', 'fix/rebase-test'])
  assert.ok(cap.output().includes('already open — skipping creation (--force-with-lease)'), cap.output())
})

test('ship force-with-lease: nothing staged → push-only, no commit required', () => {
  const { opts, deps, git, cap } = makeForceLeaseDeps({ staged: '', checkPROpen: () => true })
  opts.message = ''
  const code = runShip(opts, deps)
  assert.equal(code, 0, cap.output())
  assert.ok(!git.calls.some((c) => c[0] === 'commit'), 'commit must not be called when nothing staged')
  assert.ok(cap.output().includes('pushes existing commits only'), cap.output())
})

test('ship: nothing staged without --force-with-lease still aborts (non-regression)', () => {
  const { deps, cap } = makeDeps({ branch: 'fix/rebase-test', staged: '' })
  const code = runShip({ message: 'fix: x', forceWithLease: false }, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('nothing is staged'), cap.output())
})

test('ship force-with-lease: no open PR → refuses before any write', () => {
  const { opts, deps, git, cap } = makeForceLeaseDeps({ checkPROpen: () => false })
  const code = runShip(opts, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('no open pull/merge request'), cap.output())
  assert.ok(!git.calls.some((c) => c[0] === 'commit' || c[0] === 'push'), 'must not write before refusal')
})

test('ship force-with-lease: no forge CLI available → refuses without degrading', () => {
  const { deps, git, cap } = makeDeps({
    branch: 'fix/rebase-test',
    staged: 'file.js',
    configForge: 'github',
    availFn: () => false,
    remoteURL: 'https://github.com/org/repo.git',
  })
  const code = runShip({ message: 'fix: rebase', forceWithLease: true }, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('requires a forge CLI'), cap.output())
  assert.ok(!git.calls.some((c) => c[0] === 'commit' || c[0] === 'push'), 'must not write before refusal')
})

test('ship force-with-lease: manual forge → refuses', () => {
  const { deps, cap } = makeDeps({
    branch: 'fix/rebase-test',
    staged: 'file.js',
    availFn: () => true,
  })
  const code = runShip({ message: 'fix: rebase', forceWithLease: true }, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('requires a forge CLI'), cap.output())
})

test('ship force-with-lease: checkPROpen throws → cannot verify, refuses', () => {
  const { opts, deps, git, cap } = makeForceLeaseDeps({
    checkPROpen: () => { throw new Error('gh: authentication required') },
  })
  const code = runShip(opts, deps)
  assert.equal(code, 1)
  assert.ok(cap.output().includes('could not verify'), cap.output())
  assert.ok(!git.calls.some((c) => c[0] === 'commit' || c[0] === 'push'), 'must not write before refusal')
})

test('ship force-with-lease: dry-run still runs the gate', () => {
  let called = false
  const { opts, deps, cap } = makeForceLeaseDeps({
    checkPROpen: () => { called = true; return false },
  })
  opts.dryRun = true
  const code = runShip(opts, deps)
  assert.equal(code, 1)
  assert.ok(called, 'checkPROpen must run in dry-run mode too')
  assert.ok(cap.output().includes('no open pull/merge request'), cap.output())
})

test('ship: normal push unaffected by force-with-lease code path (non-regression)', () => {
  const { deps, git } = makeDeps({ branch: 'fix/normal', staged: 'file.js' })
  const code = runShip({ message: 'fix: x', forceWithLease: false }, deps)
  assert.equal(code, 0)
  const pushCall = git.calls.find((c) => c[0] === 'push')
  assert.deepEqual(pushCall, ['push', '-u', 'origin', 'fix/normal'])
})

test('ship CLI: raw --force flag does not exist; --force-with-lease does', () => {
  const forceOpt = shipCommand.options.find((o) => o.long === '--force')
  assert.equal(forceOpt, undefined, 'raw --force must never be registered on ship')
  const leaseOpt = shipCommand.options.find((o) => o.long === '--force-with-lease')
  assert.ok(leaseOpt, 'expected --force-with-lease option to be registered')
})
