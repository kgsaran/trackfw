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
  defaultDeleteBranch,
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
  // f1.md — deliberately a doc file, to prove pending_work is decided by "diverg == touched"
  // (nothing from this branch reached main at all), not by file type. ML-1C fixed the earlier
  // bug where a doc-only branch with diverg == touched was misrouted to REVIEW_DOC_CONFIG
  // ("probable housekeeping, confirm and delete manually") even though it had never been
  // integrated at all. The test below is the contrasting case: same file types, but diverg is a
  // PROPER subset of touched (partial integration), which is what actually makes it
  // REVIEW_DOC_CONFIG.
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

test('evaluateBranchIntegration: doc/config-only divergence -> review, never deletable', () => {
  // review_doc_config requires diverg to be a PROPER subset of touched — genuine partial
  // integration (README-merged.md made it into main, CLAUDE.md/trackfw.yaml are residue), not a
  // branch that was never integrated at all.
  const execGit = fakeExecGit({
    'merge-base origin/main feat/docs-only': { stdout: 'abc123', error: null },
    'diff --name-only -z abc123 feat/docs-only': {
      stdout: 'CLAUDE.md\x00README-merged.md\x00trackfw.yaml\x00',
      error: null,
    },
    'diff --name-only -z origin/main feat/docs-only -- CLAUDE.md README-merged.md trackfw.yaml': {
      stdout: 'CLAUDE.md\x00trackfw.yaml\x00',
      error: null,
    },
  })
  const evalResult = evaluateBranchIntegration('feat/docs-only', execGit)
  assert.equal(evalResult.decision, DECISION.REVIEW_DOC_CONFIG)
  assert.equal(isDeletable(evalResult.decision), false, 'review_doc_config must never be deletable — KG explicit instruction')
  assert.ok(evalResult.reason.includes('CLAUDE.md'))
  assert.ok(evalResult.reason.includes('trackfw.yaml'))
  assert.ok(evalResult.reason.includes('confirm and delete manually'))
})

test('evaluateBranchIntegration: doc-only, never integrated (diverg == touched) -> pending_work, not review — ML-1C discriminant', () => {
  // KG's exact repro: a branch with brand-new, never-merged documentation. diverg == touched
  // (nothing reached main), so this must be pending_work even though every file is doc/config.
  const execGit = fakeExecGit({
    'merge-base origin/main feat/doc-real': { stdout: 'abc123', error: null },
    'diff --name-only -z abc123 feat/doc-real': { stdout: 'docs/guia-novo.md\x00', error: null },
    'diff --name-only -z origin/main feat/doc-real -- docs/guia-novo.md': {
      stdout: 'docs/guia-novo.md\x00',
      error: null,
    },
  })
  const evalResult = evaluateBranchIntegration('feat/doc-real', execGit)
  assert.equal(evalResult.decision, DECISION.PENDING_WORK)
  assert.equal(isDeletable(evalResult.decision), false)
  assert.ok(!evalResult.reason.includes('housekeeping'), `must not suggest housekeeping for never-merged work: ${evalResult.reason}`)
})

test('evaluateBranchIntegration: mixed doc+code divergence stays pending_work', () => {
  const execGit = fakeExecGit({
    'merge-base origin/main feat/mixed': { stdout: 'abc123', error: null },
    'diff --name-only -z abc123 feat/mixed': { stdout: 'README.md\x00main.js\x00', error: null },
    'diff --name-only -z origin/main feat/mixed -- README.md main.js': {
      stdout: 'README.md\x00main.js\x00',
      error: null,
    },
  })
  const evalResult = evaluateBranchIntegration('feat/mixed', execGit)
  assert.equal(evalResult.decision, DECISION.PENDING_WORK)
  assert.equal(isDeletable(evalResult.decision), false)
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
        case 'fetch origin --prune':
          return { stdout: '', error: null }
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

test('runBranchPrune: fetch failure warns but still evaluates (non-blocking)', () => {
  const lines = []
  let fetchCalled = false
  const deps = {
    execGit: (args) => {
      const key = args.join(' ')
      switch (key) {
        case 'fetch origin --prune':
          fetchCalled = true
          return { stdout: '', error: new Error('fatal: unable to access origin (simulated offline)') }
        case 'rev-parse --verify -q origin/main':
          return { stdout: 'abc123', error: null } // a previous successful fetch already resolved this
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
    writeln: (s) => lines.push(s),
  }

  const exitCode = runBranchPrune(false, deps)
  assert.equal(exitCode, 0, 'fetch failure must not abort the command')
  assert.equal(fetchCalled, true)
  const got = lines.join('\n')
  assert.ok(got.toLowerCase().includes('warning') && got.includes('fetch'), `expected a fetch-failure warning: ${got}`)
  assert.ok(got.includes('would delete') && got.includes('feat/integrated'), `evaluation must proceed despite fetch failure: ${got}`)
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

// ────────────────────────────────────────────────────────────────────────────
// defaultDeleteBranch — -d tried before -D, real git repository (both codepaths).
// ────────────────────────────────────────────────────────────────────────────

test('defaultDeleteBranch: tries -d before -D, both codepaths', { timeout: 30000 }, () => {
  const which = spawnSync('git', ['--version'])
  if (which.error) return

  const work = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-branch-prune-dd-node-'))
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

  const repo = path.join(work, 'repo')
  fs.mkdirSync(repo, { recursive: true })
  run(repo, ['init', '-q', '-b', 'main'])
  run(repo, ['config', 'user.email', 'falsify@trackfw.test'])
  run(repo, ['config', 'user.name', 'trackfw falsify'])
  run(repo, ['config', 'commit.gpgsign', 'false'])
  fs.writeFileSync(path.join(repo, 'base.txt'), 'base\n')
  run(repo, ['add', 'base.txt'])
  run(repo, ['commit', '-q', '-m', 'base'])

  function execGit(args) {
    const result = spawnSync('git', args, { cwd: repo, env: env(), encoding: 'utf8' })
    if (result.status !== 0) {
      return { stdout: '', error: new Error((result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`) }
    }
    return { stdout: (result.stdout || '').trim(), error: null }
  }

  // Codepath 1: feat/ff has fast-forward ancestry with main (plain merge, no squash) — `-d` alone
  // must succeed.
  run(repo, ['checkout', '-q', '-b', 'feat/ff'])
  fs.writeFileSync(path.join(repo, 'ff.txt'), 'ff\n')
  run(repo, ['add', 'ff.txt'])
  run(repo, ['commit', '-q', '-m', 'feat/ff work'])
  run(repo, ['checkout', '-q', 'main'])
  run(repo, ['merge', '-q', '--no-ff', 'feat/ff'])

  const err1 = defaultDeleteBranch(execGit, 'feat/ff')
  assert.equal(err1, null, `expected defaultDeleteBranch to succeed via plain -d, got: ${err1}`)
  assert.ok(!defaultListLocalBranches(execGit).branches.includes('feat/ff'))

  // Codepath 2: feat/squash has NO ancestry with main (squash-merge) — plain -d refuses;
  // defaultDeleteBranch must fall back to -D and still succeed.
  run(repo, ['checkout', '-q', '-b', 'feat/squash'])
  fs.writeFileSync(path.join(repo, 'squash.txt'), 'squash\n')
  run(repo, ['add', 'squash.txt'])
  run(repo, ['commit', '-q', '-m', 'feat/squash work'])
  run(repo, ['checkout', '-q', 'main'])
  run(repo, ['merge', '-q', '--squash', 'feat/squash'])
  run(repo, ['commit', '-q', '-m', 'squash-merge feat/squash'])

  const dCheck = spawnSync('git', ['-C', repo, 'branch', '-d', 'feat/squash'], { env: env() })
  assert.notEqual(dCheck.status, 0, 'test setup invalid: git branch -d unexpectedly succeeded on a squash-merged branch')

  const err2 = defaultDeleteBranch(execGit, 'feat/squash')
  assert.equal(err2, null, `expected defaultDeleteBranch to fall back to -D and succeed, got: ${err2}`)
  assert.ok(!defaultListLocalBranches(execGit).branches.includes('feat/squash'))
})

// ────────────────────────────────────────────────────────────────────────────
// Stale origin/main — real git repository. Proves "origin/main defasado leva a mais recusas,
// nunca a deleção indevida": when `git fetch origin --prune` fails (simulated by breaking the
// remote URL), evaluation keeps using whatever origin/main ref is already resolvable locally. A
// branch truly integrated upstream, but invisible to this stale ref, is reported KEPT
// (pending_work) — never wrongly offered for deletion.
// ────────────────────────────────────────────────────────────────────────────

test('runBranchPrune: stale origin/main is conservative, not wrong', { timeout: 30000 }, () => {
  const which = spawnSync('git', ['--version'])
  if (which.error) return

  const work = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-branch-prune-stale-node-'))
  const bareDir = path.join(work, 'origin.git')
  const cloneDir = path.join(work, 'clone')
  const otherCloneDir = path.join(work, 'other-clone')
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

  fs.writeFileSync(path.join(cloneDir, 'base.txt'), 'base\n')
  run(cloneDir, ['add', 'base.txt'])
  run(cloneDir, ['commit', '-q', '-m', 'base commit'])
  run(cloneDir, ['push', '-q', 'origin', 'main'])

  run(cloneDir, ['checkout', '-q', '-b', 'feat/mine'])
  fs.writeFileSync(path.join(cloneDir, 'mine.txt'), 'mine v1\n')
  run(cloneDir, ['add', 'mine.txt'])
  run(cloneDir, ['commit', '-q', '-m', 'feat/mine work'])
  run(cloneDir, ['checkout', '-q', 'main'])
  // cloneDir's origin/main is now frozen at "base commit" — never fetched again from here.

  fs.mkdirSync(otherCloneDir, { recursive: true })
  run(work, ['clone', '-q', bareDir, otherCloneDir])
  run(otherCloneDir, ['config', 'user.email', 'falsify@trackfw.test'])
  run(otherCloneDir, ['config', 'user.name', 'trackfw falsify'])
  run(otherCloneDir, ['config', 'commit.gpgsign', 'false'])
  fs.writeFileSync(path.join(otherCloneDir, 'mine.txt'), 'mine v1\n')
  run(otherCloneDir, ['add', 'mine.txt'])
  run(otherCloneDir, ['commit', '-q', '-m', 'someone else lands the same content upstream'])
  run(otherCloneDir, ['push', '-q', 'origin', 'main'])

  // Break the remote URL so `git fetch origin --prune` fails deterministically.
  run(cloneDir, ['remote', 'set-url', 'origin', path.join(work, 'does-not-exist.git')])

  function execGit(args) {
    const result = spawnSync('git', args, { cwd: cloneDir, env: env(), encoding: 'utf8' })
    if (result.status !== 0) {
      return { stdout: '', error: new Error((result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`) }
    }
    return { stdout: (result.stdout || '').trim(), error: null }
  }

  const outLines = []
  const exitCode = runBranchPrune(false, {
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
    deleteBranch: () => {
      throw new Error('dry-run must never call deleteBranch')
    },
    writeln: (s) => outLines.push(s),
  })

  assert.equal(exitCode, 0)
  const got = outLines.join('\n')
  assert.ok(got.toLowerCase().includes('warning'), `expected fetch-failure warning: ${got}`)
  for (const line of got.split('\n')) {
    if (line.trim().startsWith('feat/mine ')) {
      assert.ok(!line.includes('delete'), `stale origin/main must never make feat/mine look deletable, got line: ${line}`)
      assert.ok(line.includes('keep'), `expected feat/mine reported keep, got line: ${line}`)
    }
  }

  // Contrast: with a working fetch, the exact same branch becomes deletable.
  run(cloneDir, ['remote', 'set-url', 'origin', bareDir])
  run(cloneDir, ['fetch', '-q', 'origin'])
  const evalAfterFetch = evaluateBranchIntegration('feat/mine', execGit)
  assert.equal(
    evalAfterFetch.decision,
    DECISION.IDENTICAL,
    `after a real fetch, expected feat/mine to become content_identical, got ${evalAfterFetch.decision} (${evalAfterFetch.reason})`
  )
  assert.equal(isDeletable(evalAfterFetch.decision), true)
})

// ────────────────────────────────────────────────────────────────────────────
// ML-1C discriminant, real git repo: doc-only never-integrated vs partial doc residue,
// side by side (mirrors Go's TestEvaluateBranchIntegration_RealGitRepo_DocOnlyNeverIntegrated
// VsPartialResidue).
// ────────────────────────────────────────────────────────────────────────────

test('evaluateBranchIntegration: real git repo — doc-only never-integrated vs partial doc residue (ML-1C)', { timeout: 30000 }, () => {
  const which = spawnSync('git', ['--version'])
  if (which.error) {
    return
  }

  const work = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-branch-prune-node-ml1c-'))
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
    const full = path.join(cloneDir, name)
    fs.mkdirSync(path.dirname(full), { recursive: true })
    fs.writeFileSync(full, content)
  }

  writeFile('base.txt', 'base\n')
  run(cloneDir, ['add', 'base.txt'])
  run(cloneDir, ['commit', '-q', '-m', 'base commit'])
  run(cloneDir, ['push', '-q', 'origin', 'main'])

  // feat/residue: touches app.js (code) and docs/notas.md (doc). Squash-merged, but only the
  // code file's content is picked up by the squash-merge commit — docs/notas.md never lands in
  // main, the residue this discriminant targets.
  run(cloneDir, ['checkout', '-q', '-b', 'feat/residue'])
  writeFile('app.js', 'module.exports = {}\n')
  writeFile('docs/notas.md', 'notas da branch\n')
  run(cloneDir, ['add', 'app.js', 'docs/notas.md'])
  run(cloneDir, ['commit', '-q', '-m', 'feat/residue work: code + doc'])
  run(cloneDir, ['checkout', '-q', 'main'])
  writeFile('app.js', 'module.exports = {}\n')
  run(cloneDir, ['add', 'app.js'])
  run(cloneDir, ['commit', '-q', '-m', 'squash-merge feat/residue (code only, doc left out)'])
  run(cloneDir, ['push', '-q', 'origin', 'main'])

  // feat/doc-real: brand-new documentation, branched off current main, never merged anywhere.
  run(cloneDir, ['checkout', '-q', '-b', 'feat/doc-real'])
  writeFile('docs/guia-novo.md', 'guia novo, nunca mergeado\n')
  run(cloneDir, ['add', 'docs/guia-novo.md'])
  run(cloneDir, ['commit', '-q', '-m', 'feat/doc-real: never-merged documentation'])
  run(cloneDir, ['checkout', '-q', 'main'])
  run(cloneDir, ['fetch', '-q', 'origin'])

  function execGit(args) {
    const result = spawnSync('git', args, { cwd: cloneDir, env: env(), encoding: 'utf8' })
    if (result.status !== 0) {
      return { stdout: '', error: new Error((result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`) }
    }
    return { stdout: (result.stdout || '').trim(), error: null }
  }

  const evalDocReal = evaluateBranchIntegration('feat/doc-real', execGit)
  assert.equal(evalDocReal.decision, DECISION.PENDING_WORK, `feat/doc-real expected pending_work, got ${evalDocReal.decision} (${evalDocReal.reason})`)
  assert.equal(isDeletable(evalDocReal.decision), false)
  assert.ok(!evalDocReal.reason.includes('housekeeping'), `feat/doc-real must not be advised as housekeeping: ${evalDocReal.reason}`)
  assert.equal(evalDocReal.touched.length, evalDocReal.diverged.length, 'feat/doc-real: expected touched == diverg')

  const evalResidue = evaluateBranchIntegration('feat/residue', execGit)
  assert.equal(evalResidue.decision, DECISION.REVIEW_DOC_CONFIG, `feat/residue expected review_doc_config, got ${evalResidue.decision} (${evalResidue.reason})`)
  assert.equal(isDeletable(evalResidue.decision), false)
  assert.ok(evalResidue.reason.includes('confirm and delete manually'), `expected manual-confirmation guidance, got ${evalResidue.reason}`)
  assert.ok(evalResidue.diverged.length < evalResidue.touched.length, `feat/residue: expected diverg to be a PROPER subset of touched, got touched=${evalResidue.touched} diverg=${evalResidue.diverged}`)
})
