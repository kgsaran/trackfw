'use strict'

// commit.test.js — mirrors internal/commands/commit_test.go scenario-for-scenario, so Node.js
// stays behaviorally identical to Go (the behavioral reference, docs/cli-parity.md). Follows the
// same injectable-deps pattern already established by branch.test.js, and the same stdout/stderr
// split convention (writeln = stdout, writeErr = stderr, mirroring Go's cmd.OutOrStdout() vs
// root.go Execute()'s os.Stderr).

const test = require('node:test')
const assert = require('node:assert/strict')
const { runCommit, buildSuggestedMessage, isCommitGatedBranch, commitGovernedBranchPrefix } = require('../src/commit/runner')
const validator = require('../src/validator')

// makeCommitDeps builds runCommit deps wired to injectable fakes, so tests never touch a real
// git repository or the real project filesystem layout.
function makeCommitDeps({ branch, matched, candidates } = {}) {
  const out = []
  const err = []
  const commitCalls = []
  const deps = {
    loadConfig: () => ({}),
    resolveWIPDirs: () => ['docs/roadmaps/wip'],
    resolveDoneDirs: () => ['docs/roadmaps/done'],
    matchSlug: () => ({ matched: !!matched, candidates: candidates || [] }),
    currentBranch: () => branch,
    execGitCommit: (message) => {
      commitCalls.push(message)
      return 0
    },
    writeln: (s) => out.push(s),
    writeErr: (s) => err.push(s),
  }
  return { deps, out, err, commitCalls }
}

// ────────────────────────────────────────────────────────────────────────────
// isCommitGatedBranch / commitGovernedBranchPrefix
// ────────────────────────────────────────────────────────────────────────────

test('isCommitGatedBranch: feat/fix/refactor match', () => {
  assert.equal(isCommitGatedBranch('feat/my-slug'), true)
  assert.equal(isCommitGatedBranch('fix/my-slug'), true)
  assert.equal(isCommitGatedBranch('refactor/my-slug'), true)
})

test('isCommitGatedBranch: other branches do not match', () => {
  assert.equal(isCommitGatedBranch('main'), false)
  assert.equal(isCommitGatedBranch('chore/housekeeping'), false)
  assert.equal(isCommitGatedBranch('feat'), false)
})

test('commitGovernedBranchPrefix: returns the matched prefix', () => {
  assert.equal(commitGovernedBranchPrefix('feat/my-slug'), 'feat/')
  assert.equal(commitGovernedBranchPrefix('fix/my-slug'), 'fix/')
  assert.equal(commitGovernedBranchPrefix('refactor/my-slug'), 'refactor/')
  assert.equal(commitGovernedBranchPrefix('main'), null)
})

// ────────────────────────────────────────────────────────────────────────────
// runCommit — case 1: blocked on main/master (literal match only — no dynamic resolution of
// the repo's configured default branch, mirroring Go's commitProtectedBranches).
// ────────────────────────────────────────────────────────────────────────────

test('runCommit: blocks on main, never calls git commit', () => {
  const { deps, out, err, commitCalls } = makeCommitDeps({ branch: 'main' })
  const code = runCommit('fix: something', deps)
  assert.notEqual(code, 0)
  assert.equal(commitCalls.length, 0)
  assert.ok(
    out.join('\n').includes('trackfw commit: commit direto em "main" não é permitido.'),
    `unexpected stdout:\n${out.join('\n')}`
  )
  assert.ok(out.join('\n').includes("Use 'trackfw branch new <type>/<slug>' primeiro."))
  assert.ok(out.join('\n').includes('Ver CLAUDE.md §1.'))
  assert.ok(err.join('\n').includes('blocked: commit directly on "main" is not permitted'))
})

test('runCommit: blocks on master, never calls git commit', () => {
  const { deps, out, err, commitCalls } = makeCommitDeps({ branch: 'master' })
  const code = runCommit('fix: something', deps)
  assert.notEqual(code, 0)
  assert.equal(commitCalls.length, 0)
  assert.ok(out.join('\n').includes('trackfw commit: commit direto em "master" não é permitido.'))
  assert.ok(err.join('\n').includes('blocked: commit directly on "master" is not permitted'))
})

// ────────────────────────────────────────────────────────────────────────────
// runCommit — case 2: feat/fix/refactor branch without matching roadmap in wip/
// ────────────────────────────────────────────────────────────────────────────

test('runCommit: blocks feat/x with no roadmap in wip/, no candidates', () => {
  const { deps, out, err, commitCalls } = makeCommitDeps({ branch: 'feat/orphan-slug', matched: false, candidates: null })
  const code = runCommit('feat: something', deps)
  assert.notEqual(code, 0)
  assert.equal(commitCalls.length, 0)
  const want = validator.branchGovernanceOrientation('feat/orphan-slug')
  assert.ok(out.join('\n').includes(want), `expected stdout to include:\n${want}\ngot:\n${out.join('\n')}`)
  assert.ok(err.join('\n').includes('blocked: no matching roadmap in wip/ nor done/ for "feat/orphan-slug"'))
})

test('runCommit: blocks feat/x with no matching roadmap, but candidates exist', () => {
  const candidates = ['ROADMAP-other-thing.md']
  const { deps, out, err, commitCalls } = makeCommitDeps({ branch: 'fix/orphan-slug', matched: false, candidates })
  const code = runCommit('fix: something', deps)
  assert.notEqual(code, 0)
  assert.equal(commitCalls.length, 0)
  const want = validator.branchNoMatchingRoadmapMessage('fix/orphan-slug', candidates)
  assert.ok(out.join('\n').includes(want), `expected stdout to include:\n${want}\ngot:\n${out.join('\n')}`)
  assert.ok(err.join('\n').includes('blocked: no matching roadmap in wip/ nor done/ for "fix/orphan-slug"'))
})

// ────────────────────────────────────────────────────────────────────────────
// runCommit — case 3: feat/fix/refactor branch WITH matching roadmap in wip/ — succeeds
// ────────────────────────────────────────────────────────────────────────────

test('runCommit: succeeds on feat/x with matching roadmap in wip/', () => {
  const { deps, commitCalls } = makeCommitDeps({ branch: 'feat/my-slug', matched: true, candidates: ['ROADMAP-my-slug.md'] })
  const code = runCommit('feat: my change', deps)
  assert.equal(code, 0)
  assert.deepEqual(commitCalls, ['feat: my change'])
})

test('runCommit: succeeds on refactor/x with matching roadmap in done/', () => {
  const { deps, commitCalls } = makeCommitDeps({ branch: 'refactor/my-slug', matched: true, candidates: ['ROADMAP-my-slug.md'] })
  const code = runCommit('refactor: my change', deps)
  assert.equal(code, 0)
  assert.deepEqual(commitCalls, ['refactor: my change'])
})

// ────────────────────────────────────────────────────────────────────────────
// runCommit — case 4: branch outside feat/fix/refactor — allowed, warns only
// ────────────────────────────────────────────────────────────────────────────

test('runCommit: succeeds on branch outside feat/fix/refactor convention, only warns', () => {
  const { deps, out, commitCalls } = makeCommitDeps({ branch: 'chore/housekeeping' })
  const code = runCommit('chore: housekeeping', deps)
  assert.equal(code, 0)
  assert.deepEqual(commitCalls, ['chore: housekeeping'])
  assert.ok(
    out.join('\n').includes('trackfw commit: branch "chore/housekeeping" does not follow feat/fix/refactor — committing without a roadmap check.'),
    `unexpected stdout:\n${out.join('\n')}`
  )
})

// ────────────────────────────────────────────────────────────────────────────
// runCommit — message is required
// ────────────────────────────────────────────────────────────────────────────

test('runCommit: empty message is rejected before touching git or resolving branch', () => {
  const { deps, err, commitCalls } = makeCommitDeps({ branch: 'feat/my-slug', matched: true })
  deps.currentBranch = () => { throw new Error('should not be called') }
  const code = runCommit('', deps)
  assert.notEqual(code, 0)
  assert.equal(commitCalls.length, 0)
  assert.ok(err.join('\n').includes('commit message is required — use -m:\n  trackfw commit -m "feat(<scope>): <description>"'))
})

// ────────────────────────────────────────────────────────────────────────────
// runCommit — propagates git's own exit code literally
// ────────────────────────────────────────────────────────────────────────────

test('runCommit: propagates git commit\'s own exit code literally', () => {
  const { deps, out, err } = makeCommitDeps({ branch: 'feat/my-slug', matched: true })
  deps.execGitCommit = () => 1
  const code = runCommit('feat: x', deps)
  assert.equal(code, 1)
  assert.equal(out.length, 0)
  assert.equal(err.length, 0)
})

// ────────────────────────────────────────────────────────────────────────────
// runCommit — current branch resolution failure
// ────────────────────────────────────────────────────────────────────────────

test('runCommit: current branch resolution failure blocks with a clear message', () => {
  const { deps, err, commitCalls } = makeCommitDeps({ branch: 'feat/my-slug', matched: true })
  deps.currentBranch = () => { throw new Error('not a git repository') }
  const code = runCommit('feat: x', deps)
  assert.notEqual(code, 0)
  assert.equal(commitCalls.length, 0)
  assert.ok(err.join('\n').includes('could not determine current branch'))
})

// ────────────────────────────────────────────────────────────────────────────
// buildSuggestedMessage — mirrors internal/commands/commit_test.go's TestBuildSuggestedMessage
// scenario-for-scenario.
// ────────────────────────────────────────────────────────────────────────────

test('buildSuggestedMessage: nothing staged raises a clear error', () => {
  assert.throws(
    () => buildSuggestedMessage({ stagedNameStatus: () => '' }),
    /nothing staged — `git add` files first/
  )
})

test('buildSuggestedMessage: only test files staged -> type test', () => {
  const raw = 'M\tnpm/src/commit/runner.test.js\nA\tinternal/commands/commit_test.go\n'
  const out = buildSuggestedMessage({ stagedNameStatus: () => raw })
  assert.match(out, /# Tipo sugerido: test/)
  assert.match(out, /^test\(<escopo>\): <descrição>$/m)
})

test('buildSuggestedMessage: only docs/.md staged -> type docs', () => {
  const raw = 'M\tdocs/adr/ADR-001-example.md\nA\tvault/notes/example.md\nM\treadme.md\n'
  const out = buildSuggestedMessage({ stagedNameStatus: () => raw })
  assert.match(out, /# Tipo sugerido: docs/)
  assert.match(out, /^docs\(<escopo>\): <descrição>$/m)
})

test('buildSuggestedMessage: new file under a commands dir -> type feat', () => {
  const raw = 'A\tnpm/src/commands/newthing.js\nM\tnpm/src/commit/runner.js\n'
  const out = buildSuggestedMessage({ stagedNameStatus: () => raw })
  assert.match(out, /# Tipo sugerido: feat/)
  assert.match(out, /^feat\(<escopo>\): <descrição>$/m)
})

test('buildSuggestedMessage: generic staged set -> type fix', () => {
  const raw = 'M\tnpm/src/commit/runner.js\nM\tnpm/src/validator/index.js\n'
  const out = buildSuggestedMessage({ stagedNameStatus: () => raw })
  assert.match(out, /# Tipo sugerido: fix/)
  assert.match(out, /^fix\(<escopo>\): <descrição>$/m)
})

test('buildSuggestedMessage: lists staged files by status under the expected heading', () => {
  const raw = 'A\tnpm/src/commands/newthing.js\nM\tnpm/src/commit/runner.js\nD\tnpm/src/old.js\n'
  const out = buildSuggestedMessage({ stagedNameStatus: () => raw })
  assert.match(out, /## Arquivos staged/)
  assert.match(out, /^A {2}npm\/src\/commands\/newthing\.js$/m)
  assert.match(out, /^M {2}npm\/src\/commit\/runner\.js$/m)
  assert.match(out, /^D {2}npm\/src\/old\.js$/m)
})

test('buildSuggestedMessage: starts with the heuristic disclaimer header', () => {
  const raw = 'M\tnpm/src/commit/runner.js\n'
  const out = buildSuggestedMessage({ stagedNameStatus: () => raw })
  assert.ok(out.startsWith('# Sugestão heurística — NÃO é uma mensagem pronta, revise antes de usar.\n'))
})

test('buildSuggestedMessage: propagates a read failure as a clear error', () => {
  assert.throws(
    () => buildSuggestedMessage({ stagedNameStatus: () => { throw new Error('not a git repository') } }),
    /could not read staged changes \(are you in a git repo\?\)/
  )
})
