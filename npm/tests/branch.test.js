'use strict'

// branch.test.js — mirrors internal/commands/branch_test.go scenario-for-scenario, so Node.js
// stays behaviorally identical to Go (the behavioral reference, docs/cli-parity.md). Go's
// stdout/stderr split (internal/commands/branch.go + root.go Execute()) is mirrored here via
// separate writeln (stdout) / writeErr (stderr) captures.

const test = require('node:test')
const assert = require('node:assert/strict')
const { parseBranchSpec, runBranchNew } = require('../src/branch/runner')
const validator = require('../src/validator')

// makeBranchDeps builds runBranchNew deps wired to injectable fakes, so tests never touch a real
// git repository or the real project filesystem layout.
function makeBranchDeps(matched, candidates) {
  const out = []
  const err = []
  const checkoutCalls = []
  const deps = {
    loadConfig: () => ({}),
    resolveWIPDirs: () => ['docs/roadmaps/wip'],
    resolveDoneDirs: () => ['docs/roadmaps/done'],
    matchSlug: (slug, wipDirs, doneDirs) => ({ matched, candidates: candidates || [] }),
    execGitCheckout: (branchName) => {
      checkoutCalls.push(branchName)
      return 0
    },
    writeln: (s) => out.push(s),
    writeErr: (s) => err.push(s),
  }
  return { deps, out, err, checkoutCalls }
}

// ────────────────────────────────────────────────────────────────────────────
// parseBranchSpec
// ────────────────────────────────────────────────────────────────────────────

test('parseBranchSpec: valid types', () => {
  for (const typ of ['feat', 'fix', 'refactor', 'chore', 'docs']) {
    const { branchType, slug } = parseBranchSpec(`${typ}/my-slug`)
    assert.equal(branchType, typ)
    assert.equal(slug, 'my-slug')
  }
})

test('parseBranchSpec: invalid type', () => {
  assert.throws(() => parseBranchSpec('banana/my-slug'), /invalid branch type/)
  assert.throws(
    () => parseBranchSpec('banana/my-slug'),
    (err) => err.message === 'invalid branch type "banana" — must be one of feat, fix, refactor, chore, docs'
  )
})

test('parseBranchSpec: empty slug', () => {
  assert.throws(() => parseBranchSpec('feat/'), /slug is required/)
})

test('parseBranchSpec: no slash', () => {
  assert.throws(() => parseBranchSpec('feat-my-slug'), /invalid branch spec/)
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — match found (wip/ or done/, no distinction at this layer since matchSlug is
// injected — the real matching logic is covered by validator.test.js branch_has_wip_roadmap
// scenarios).
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: match found checks out branch, no extra output', () => {
  const { deps, out, err, checkoutCalls } = makeBranchDeps(true, null)
  const code = runBranchNew('feat/my-slug', false, deps)
  assert.equal(code, 0)
  assert.deepEqual(checkoutCalls, ['feat/my-slug'])
  assert.equal(out.length, 0)
  assert.equal(err.length, 0)
})

test('runBranchNew: match found via wip roadmap', () => {
  const { deps, checkoutCalls } = makeBranchDeps(true, ['ROADMAP-my-slug.md'])
  const code = runBranchNew('fix/my-slug', false, deps)
  assert.equal(code, 0)
  assert.equal(checkoutCalls.length, 1)
})

test('runBranchNew: match found via done roadmap', () => {
  const { deps, checkoutCalls } = makeBranchDeps(true, ['ROADMAP-my-slug.md'])
  const code = runBranchNew('refactor/my-slug', false, deps)
  assert.equal(code, 0)
  assert.equal(checkoutCalls.length, 1)
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — no match: blocks, never calls git
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: no match, no candidates — blocks with governance orientation on stdout', () => {
  const { deps, out, err, checkoutCalls } = makeBranchDeps(false, null)
  const code = runBranchNew('feat/orphan-slug', false, deps)
  assert.notEqual(code, 0)
  assert.equal(checkoutCalls.length, 0)
  const want = validator.branchGovernanceOrientation('feat/orphan-slug')
  assert.ok(out.join('\n').includes(want), `expected stdout to include:\n${want}\ngot:\n${out.join('\n')}`)
  assert.ok(err.join('\n').includes('blocked: no matching roadmap in wip/ nor done/ for "feat/orphan-slug"'))
})

test('runBranchNew: no match, with candidates — blocks with no-matching-roadmap message on stdout', () => {
  const candidates = ['ROADMAP-other-thing.md']
  const { deps, out, err, checkoutCalls } = makeBranchDeps(false, candidates)
  const code = runBranchNew('fix/orphan-slug', false, deps)
  assert.notEqual(code, 0)
  assert.equal(checkoutCalls.length, 0)
  const want = validator.branchNoMatchingRoadmapMessage('fix/orphan-slug', candidates)
  assert.ok(out.join('\n').includes(want), `expected stdout to include:\n${want}\ngot:\n${out.join('\n')}`)
  assert.ok(err.join('\n').includes('blocked: no matching roadmap in wip/ nor done/ for "fix/orphan-slug"'))
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — --dry-run
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: dry-run + match never calls git', () => {
  const { deps, out, checkoutCalls } = makeBranchDeps(true, null)
  const code = runBranchNew('feat/my-slug', true, deps)
  assert.equal(code, 0)
  assert.equal(checkoutCalls.length, 0)
  const joined = out.join('\n')
  assert.ok(joined.includes('dry-run'))
  assert.ok(joined.includes('would create'))
})

test('runBranchNew: dry-run + no match never calls git', () => {
  const { deps, out, checkoutCalls } = makeBranchDeps(false, null)
  const code = runBranchNew('feat/orphan-slug', true, deps)
  assert.notEqual(code, 0)
  assert.equal(checkoutCalls.length, 0)
  const joined = out.join('\n')
  assert.ok(joined.includes('dry-run'))
  assert.ok(joined.includes('would block'))
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — invalid type / empty slug never reach matchSlug or git
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: invalid type never calls matchSlug or git', () => {
  let matchCalled = false
  const { deps, err, checkoutCalls } = makeBranchDeps(true, null)
  deps.matchSlug = () => { matchCalled = true; return { matched: true, candidates: [] } }
  const code = runBranchNew('banana/my-slug', false, deps)
  assert.notEqual(code, 0)
  assert.equal(matchCalled, false)
  assert.equal(checkoutCalls.length, 0)
  assert.ok(err.join('\n').includes('invalid branch type'))
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — chore/docs are housekeeping types: they create the branch without the
// branch_has_wip_roadmap gate, and never call matchSlug.
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: chore type skips gate and checks out branch', () => {
  let matchCalled = false
  // matched=false: gate would block if consulted
  const { deps, out, checkoutCalls } = makeBranchDeps(false, null)
  deps.matchSlug = () => { matchCalled = true; return { matched: false, candidates: [] } }
  const code = runBranchNew('chore/release-7.0.0', false, deps)
  assert.equal(code, 0)
  assert.equal(matchCalled, false, 'matchSlug must not be called for chore — no roadmap gate applies')
  assert.deepEqual(checkoutCalls, ['chore/release-7.0.0'])
  assert.equal(out.length, 0)
})

test('runBranchNew: docs type skips gate and checks out branch', () => {
  let matchCalled = false
  const { deps, checkoutCalls } = makeBranchDeps(false, null)
  deps.matchSlug = () => { matchCalled = true; return { matched: false, candidates: [] } }
  const code = runBranchNew('docs/atualiza-readme', false, deps)
  assert.equal(code, 0)
  assert.equal(matchCalled, false, 'matchSlug must not be called for docs — no roadmap gate applies')
  assert.deepEqual(checkoutCalls, ['docs/atualiza-readme'])
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — non-regression: feat/fix/refactor without a matching roadmap must keep
// blocking with the same governance orientation message.
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: feat without matching roadmap still blocks (non-regression)', () => {
  const { deps, out, checkoutCalls } = makeBranchDeps(false, null)
  const code = runBranchNew('feat/no-roadmap-for-this', false, deps)
  assert.notEqual(code, 0)
  assert.equal(checkoutCalls.length, 0)
  const want = validator.branchGovernanceOrientation('feat/no-roadmap-for-this')
  assert.ok(out.join('\n').includes(want), `expected stdout to still include:\n${want}\ngot:\n${out.join('\n')}`)
})

test('runBranchNew: empty slug never calls matchSlug or git', () => {
  let matchCalled = false
  const { deps, err, checkoutCalls } = makeBranchDeps(true, null)
  deps.matchSlug = () => { matchCalled = true; return { matched: true, candidates: [] } }
  const code = runBranchNew('feat/', false, deps)
  assert.notEqual(code, 0)
  assert.equal(matchCalled, false)
  assert.equal(checkoutCalls.length, 0)
  assert.ok(err.join('\n').includes('slug is required'))
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — branch already exists: delegate to Git's native error, no special handling
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: propagates git\'s own exit code literally, without reformatting output', () => {
  const { deps, out, err } = makeBranchDeps(true, null)
  // 128 mirrors Git's real exit code for "branch already exists" (internal/commands/branch.go's
  // defaultGitCheckout exits with exitErr.ExitCode() directly for this exact scenario) — the
  // contract is to propagate Git's exit status literally, never hardcode a generic 1.
  deps.execGitCheckout = () => 128
  const code = runBranchNew('feat/my-slug', false, deps)
  assert.equal(code, 128)
  // Git's own stderr is inherited directly by the child process — runBranchNew must not
  // reformat or duplicate it via writeln/writeErr.
  assert.equal(out.length, 0)
  assert.equal(err.length, 0)
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — normalized slug is what reaches matchSlug
// ────────────────────────────────────────────────────────────────────────────

test('runBranchNew: uses normalized slug for matching', () => {
  let receivedSlug = null
  const { deps } = makeBranchDeps(true, null)
  deps.matchSlug = (slug) => { receivedSlug = slug; return { matched: true, candidates: [] } }
  runBranchNew('feat/My_Weird--Slug', false, deps)
  assert.equal(receivedSlug, validator.normalizeBranchSlug('My_Weird--Slug'))
})
