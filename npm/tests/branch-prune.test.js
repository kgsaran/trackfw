'use strict'

// branch-prune.test.js — mirrors internal/commands/branch_prune_test.go scenario-for-scenario, so
// Node.js stays behaviorally identical to Go (the behavioral reference, docs/cli-parity.md).

const test = require('node:test')
const assert = require('node:assert/strict')
const os = require('node:os')
const fs = require('node:fs')
const path = require('node:path')
const { spawnSync } = require('node:child_process')
const {
  DECISION,
  isDeletable,
  splitNulPaths,
  evaluateBranchIntegration,
  defaultListLocalBranches,
  runBranchPrune,
} = require('../src/branch/prune')

// ────────────────────────────────────────────────────────────────────────────
// splitNulPaths
// ────────────────────────────────────────────────────────────────────────────

test('splitNulPaths: empty string yields no paths', () => {
  assert.deepEqual(splitNulPaths(''), [])
})

test('splitNulPaths: single path with trailing NUL', () => {
  assert.deepEqual(splitNulPaths('foo.md\x00'), ['foo.md'])
})

test('splitNulPaths: multiple paths sorted', () => {
  assert.deepEqual(splitNulPaths('z.md\x00a.md\x00'), ['a.md', 'z.md'])
})

test('splitNulPaths: filename with a space survives -z splitting', () => {
  assert.deepEqual(splitNulPaths('foo bar.md\x00'), ['foo bar.md'])
})

test('splitNulPaths: no trailing NUL still splits correctly', () => {
  assert.deepEqual(splitNulPaths('a.md\x00b.md'), ['a.md', 'b.md'])
})

// ────────────────────────────────────────────────────────────────────────────
// evaluateBranchIntegration — unit tests with a fake execGit (no real git repo)
// ────────────────────────────────────────────────────────────────────────────

function fakeExecGit(responses) {
  return (args) => {
    const key = args.join(' ')
    if (!(key in responses)) {
      throw new Error(`fakeExecGit: unexpected call: git ${key}`)
    }
    return responses[key]
  }
}

test('evaluateBranchIntegration: no own work -> deletable', () => {
  const execGit = fakeExecGit({
    'merge-base origin/main feat/foo': { stdout: 'abc123', error: null },
    'diff --name-only -z abc123 feat/foo': { stdout: '', error: null },
  })
  const evalResult = evaluateBranchIntegration('feat/foo', execGit)
  assert.equal(evalResult.decision, DECISION.NO_OWN_WORK)
  assert.equal(isDeletable(evalResult.decision), true)
})

test('evaluateBranchIntegration: content identical (stale but integrated) -> deletable — AC2 discriminant', () => {
  const execGit = fakeExecGit({
    'merge-base origin/main feat/stale': { stdout: 'abc123', error: null },
    'diff --name-only -z abc123 feat/stale': { stdout: 'f1.md\x00', error: null },
    'diff --name-only -z origin/main feat/stale -- f1.md': { stdout: '', error: null },
  })
  const evalResult = evaluateBranchIntegration('feat/stale', execGit)
  assert.equal(evalResult.decision, DECISION.IDENTICAL)
  assert.equal(isDeletable(evalResult.decision), true)
})

test('evaluateBranchIntegration: pending work -> never deletable', () => {
  const execGit = fakeExecGit({
    'merge-base origin/main feat/pending': { stdout: 'abc123', error: null },
    'diff --name-only -z abc123 feat/pending': { stdout: 'f1.md\x00', error: null },
    'diff --name-only -z origin/main feat/pending -- f1.md': { stdout: 'f1.md\x00', error: null },
  })
  const evalResult = evaluateBranchIntegration('feat/pending', execGit)
  assert.equal(evalResult.decision, DECISION.PENDING_WORK)
  assert.equal(isDeletable(evalResult.decision), false)
  assert.ok(evalResult.reason.includes('f1.md'))
})

test('evaluateBranchIntegration: no merge-base -> refuses, never deletable', () => {
  const execGit = fakeExecGit({
    'merge-base origin/main feat/orphan': { stdout: '', error: new Error('fatal: no merge base') },
  })
  const evalResult = evaluateBranchIntegration('feat/orphan', execGit)
  assert.equal(evalResult.decision, DECISION.NO_MERGE_BASE)
  assert.equal(isDeletable(evalResult.decision), false)
})

// ────────────────────────────────────────────────────────────────────────────
// runBranchPrune — orchestration with fully injected deps (no real git repo)
// ────────────────────────────────────────────────────────────────────────────

function makePruneDeps(writelnSink) {
  return {
    execGit: (args) => {
      const key = args.join(' ')
      switch (key) {
        case 'rev-parse --verify -q origin/main':
          return { stdout: 'abc123', error: null }
        case 'merge-base origin/main feat/integrated':
          return { stdout: 'abc123', error: null }
        case 'diff --name-only -z abc123 feat/integrated':
          return { stdout: '', error: null }
        case 'merge-base origin/main feat/pending':
          return { stdout: 'abc123', error: null }
        case 'diff --name-only -z abc123 feat/pending':
          return { stdout: 'f1.md\x00', error: null }
        case 'diff --name-only -z origin/main feat/pending -- f1.md':
          return { stdout: 'f1.md\x00', error: null }
        default:
          throw new Error(`unexpected execGit call: ${key}`)
      }
    },
    listLocalBranches: () => ({
      branches: ['main', 'feat/integrated', 'feat/pending', 'fix/current', 'chore/wt'],
      error: null,
    }),
    currentBranch: () => 'fix/current',
    worktreeBranches: () => new Set(['chore/wt']),
    deleteBranch: () => {
      throw new Error('deleteBranch must not be called in dry-run tests')
    },
    writeln: (s) => writelnSink.push(s),
  }
}

test('runBranchPrune: dry-run never deletes; main is never a delete candidate', () => {
  const lines = []
  const deps = makePruneDeps(lines)
  let deleteCalled = false
  deps.deleteBranch = () => {
    deleteCalled = true
    return null
  }

  const exitCode = runBranchPrune(false, deps)
  assert.equal(exitCode, 0)
  assert.equal(deleteCalled, false)

  const got = lines.join('\n')
  assert.ok(got.includes('would delete'), `expected 'would delete' in output: ${got}`)
  for (const line of got.split('\n')) {
    if (line.trim().startsWith('main ') && line.includes('delete')) {
      assert.fail(`main must never be offered for deletion, got line: ${line}`)
    }
  }
  assert.ok(got.includes('default branch'))
  assert.ok(got.includes('current branch'))
  assert.ok(got.includes('worktree'))
})

test('runBranchPrune: --apply deletes only integrated, keeps pending', () => {
  const lines = []
  const deps = makePruneDeps(lines)
  const deletedNames = []
  deps.deleteBranch = (execGit, name) => {
    deletedNames.push(name)
    return null
  }

  const exitCode = runBranchPrune(true, deps)
  assert.equal(exitCode, 0)
  assert.deepEqual(deletedNames, ['feat/integrated'])
  const got = lines.join('\n')
  assert.ok(got.includes('deleted 1 branch(es): feat/integrated'), got)
})

test('runBranchPrune: origin/main unresolvable refuses everything, even with --apply', () => {
  const lines = []
  const deps = {
    execGit: () => ({ stdout: '', error: new Error('fatal: needed a single revision') }),
    listLocalBranches: () => {
      throw new Error('listLocalBranches must not be called when origin/main is unresolvable')
    },
    currentBranch: () => '',
    worktreeBranches: () => new Set(),
    deleteBranch: () => {
      throw new Error('deleteBranch must not be called when origin/main is unresolvable')
    },
    writeln: (s) => lines.push(s),
  }

  const exitCode = runBranchPrune(true, deps)
  assert.equal(exitCode, 1)
  const got = lines.join('\n')
  assert.ok(got.includes('origin/main'), got)
})

// ────────────────────────────────────────────────────────────────────────────
// Real git repository integration test — the AC2 discriminant, mirroring
// internal/commands/branch_prune_test.go's TestEvaluateBranchIntegration_RealGitRepo_*.
// A mock of `git` would only prove the mock agrees with the code; this exercises real git
// plumbing via a local bare repo as "origin" (offline, no network) + a clone.
// ────────────────────────────────────────────────────────────────────────────

test('evaluateBranchIntegration: real git repo — squash-merge + stale discriminant (AC1/AC2)', { timeout: 30000 }, () => {
  const which = spawnSync('git', ['--version'])
  if (which.error) {
    return // git not available — skip like Go's t.Skip
  }

  const work = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-branch-prune-node-'))
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

  // Branch A: touches a.txt, squash-merged into main first.
  run(cloneDir, ['checkout', '-q', '-b', 'feat/a'])
  writeFile('a.txt', 'a\n')
  run(cloneDir, ['add', 'a.txt'])
  run(cloneDir, ['commit', '-q', '-m', 'feat/a work'])
  run(cloneDir, ['checkout', '-q', 'main'])
  run(cloneDir, ['merge', '-q', '--squash', 'feat/a'])
  run(cloneDir, ['commit', '-q', '-m', 'squash-merge feat/a'])

  // Branch B: touches b.txt, branched off main AFTER feat/a's squash-merge landed, then
  // squash-merged too — main advances further, leaving feat/a behind but still integrated.
  run(cloneDir, ['checkout', '-q', '-b', 'feat/b'])
  writeFile('b.txt', 'b\n')
  run(cloneDir, ['add', 'b.txt'])
  run(cloneDir, ['commit', '-q', '-m', 'feat/b work'])
  run(cloneDir, ['checkout', '-q', 'main'])
  run(cloneDir, ['merge', '-q', '--squash', 'feat/b'])
  run(cloneDir, ['commit', '-q', '-m', 'squash-merge feat/b'])

  run(cloneDir, ['push', '-q', 'origin', 'main'])
  run(cloneDir, ['fetch', '-q', 'origin'])

  // A genuinely pending branch: touches c.txt, never merged anywhere.
  run(cloneDir, ['checkout', '-q', '-b', 'feat/pending'])
  writeFile('c.txt', 'c\n')
  run(cloneDir, ['add', 'c.txt'])
  run(cloneDir, ['commit', '-q', '-m', 'feat/pending work, never merged'])

  function execGit(args) {
    const result = spawnSync('git', args, { cwd: cloneDir, env: env(), encoding: 'utf8' })
    if (result.status !== 0) {
      return { stdout: '', error: new Error((result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`) }
    }
    return { stdout: (result.stdout || '').trim(), error: null }
  }

  // Sanity: the naive bidirectional check IS non-empty for feat/a — proving this test actually
  // discriminates between the naive check and the heuristic, not vacuously passing.
  const naive = execGit(['diff', 'origin/main', 'feat/a', '--stat'])
  assert.equal(naive.error, null)
  assert.notEqual(naive.stdout.trim(), '', 'test setup invalid: naive diff must be non-empty to discriminate (AC2)')

  const evalA = evaluateBranchIntegration('feat/a', execGit)
  assert.equal(evalA.decision, DECISION.IDENTICAL, `feat/a expected content_identical, got ${evalA.decision} (${evalA.reason})`)
  assert.equal(isDeletable(evalA.decision), true)

  const evalPending = evaluateBranchIntegration('feat/pending', execGit)
  assert.equal(evalPending.decision, DECISION.PENDING_WORK)
  assert.equal(isDeletable(evalPending.decision), false)

  // AC1 — squash-merge without ancestry: `git branch -d` would refuse feat/a.
  const dResult = spawnSync('git', ['-C', cloneDir, 'branch', '-d', 'feat/a'], { env: env() })
  assert.notEqual(dResult.status, 0, 'test setup invalid: git branch -d unexpectedly succeeded on a squash-merged branch')

  // Full runBranchPrune end-to-end against the real repo.
  const deletedViaDeleteBranch = []
  const outLines = []
  const exitCode = runBranchPrune(true, {
    execGit,
    listLocalBranches: defaultListLocalBranches,
    currentBranch: (g) => {
      const r = g(['symbolic-ref', '--quiet', '--short', 'HEAD'])
      return r.error ? '' : r.stdout.trim()
    },
    worktreeBranches: (g) => {
      const r = g(['worktree', 'list', '--porcelain'])
      const set = new Set()
      if (r.error) return set
      const prefix = 'branch refs/heads/'
      for (const line of r.stdout.split('\n')) {
        const t = line.trim()
        if (t.startsWith(prefix)) set.add(t.slice(prefix.length))
      }
      return set
    },
    deleteBranch: (g, name) => {
      deletedViaDeleteBranch.push(name)
      return g(['branch', '-D', name]).error
    },
    writeln: (s) => outLines.push(s),
  })

  assert.equal(exitCode, 0)
  const sortedDeleted = [...deletedViaDeleteBranch].sort()
  assert.deepEqual(sortedDeleted, ['feat/a', 'feat/b'])

  const remaining = defaultListLocalBranches(execGit).branches.sort()
  assert.ok(!remaining.includes('feat/a'), 'feat/a should have been deleted by --apply')
  assert.ok(remaining.includes('feat/pending'), 'feat/pending must still exist')
})
