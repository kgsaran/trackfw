'use strict'

// barrier.test.js — testes unitários próprios do parser e dos checks de
// `trackfw barrier`, adicionais aos testes de contrato universal em
// barrier-contract.test.js (que fixam o contrato de docs/cli-parity.md).

const test = require('node:test')
const assert = require('node:assert/strict')

const barrier = require('../src/commands/barrier')

// ────────────────────────────────────────────────────────────────────────────
// findWave — rule 1 (wave heading) + malformed detection (rule 6)
// ────────────────────────────────────────────────────────────────────────────

test('findWave: locates the matching wave and its line range', () => {
  const lines = [
    '# Roadmap: X',
    '',
    '## Wave 1 — First',
    'content A',
    '## Wave 2 — Second',
    'content B',
  ]
  const wave1 = barrier.findWave(lines, 1)
  assert.equal(wave1.startLine, 2)
  assert.equal(wave1.endLine, 4)

  const wave2 = barrier.findWave(lines, 2)
  assert.equal(wave2.startLine, 4)
  assert.equal(wave2.endLine, 6)
})

test('findWave: throws UsageError naming the wave number when not found', () => {
  const lines = ['## Wave 1 — Only']
  assert.throws(() => barrier.findWave(lines, 7), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /wave 7/)
    return true
  })
})

test('findWave: throws UsageError naming the line number for a malformed wave number', () => {
  const lines = ['# Title', '## Wave abc — Broken']
  assert.throws(() => barrier.findWave(lines, 1), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /line 2/)
    return true
  })
})

// ────────────────────────────────────────────────────────────────────────────
// findMLs — rule 2 (ML heading + boundaries)
// ────────────────────────────────────────────────────────────────────────────

test('findMLs: splits MLs at the next ### or ## boundary', () => {
  const lines = [
    '## Wave 1 — X',
    '### ML-1A — First',
    'body 1a line 1',
    'body 1a line 2',
    '### ML-1B — Second',
    'body 1b',
    '## Wave 2 — Y',
  ]
  const mls = barrier.findMLs(lines, 0, 7)
  assert.equal(mls.length, 2)
  assert.equal(mls[0].id, 'ML-1A')
  assert.deepEqual(mls[0].lines, ['body 1a line 1', 'body 1a line 2'])
  assert.equal(mls[1].id, 'ML-1B')
  assert.deepEqual(mls[1].lines, ['body 1b'])
})

// ────────────────────────────────────────────────────────────────────────────
// mlCompletionStatus — rule 3
// ────────────────────────────────────────────────────────────────────────────

test('mlCompletionStatus: ✅ marker is complete', () => {
  const result = barrier.mlCompletionStatus(['**Status:** ✅ Concluído'])
  assert.equal(result.complete, true)
})

test('mlCompletionStatus: any other marker is incomplete', () => {
  for (const marker of ['⬜ Pendente', '🔄 Em andamento', '❌ Bloqueado']) {
    const result = barrier.mlCompletionStatus([`**Status:** ${marker}`])
    assert.equal(result.complete, false, `expected ${marker} to be incomplete`)
    assert.equal(result.marker, marker)
  }
})

test('mlCompletionStatus: absence of a **Status:** line is incomplete with marker "missing"', () => {
  const result = barrier.mlCompletionStatus(['no status line here'])
  assert.equal(result.complete, false)
  assert.equal(result.marker, 'missing')
})

// ────────────────────────────────────────────────────────────────────────────
// mlAcceptanceEvidence — rule 4 (including the anti-vacuity case)
// ────────────────────────────────────────────────────────────────────────────

test('mlAcceptanceEvidence: all criteria met', () => {
  const result = barrier.mlAcceptanceEvidence([
    '**Critérios de aceite:**',
    '- [x] build passes',
    '- [x] tests pass',
  ])
  assert.equal(result.hasBlock, true)
  assert.equal(result.total, 2)
  assert.equal(result.unmet, 0)
})

test('mlAcceptanceEvidence: unmet criteria counted', () => {
  const result = barrier.mlAcceptanceEvidence([
    '**Critérios de aceite:**',
    '- [x] build passes',
    '- [ ] tests pass',
    '- [ ] gate passes',
  ])
  assert.equal(result.hasBlock, true)
  assert.equal(result.unmet, 2)
})

test('mlAcceptanceEvidence: absent block is not vacuously satisfied', () => {
  const result = barrier.mlAcceptanceEvidence(['no acceptance block at all'])
  assert.equal(result.hasBlock, false)
})

test('mlAcceptanceEvidence: block header with zero criteria lines is treated as absent', () => {
  const result = barrier.mlAcceptanceEvidence([
    '**Critérios de aceite:**',
    '**Next section:**',
  ])
  assert.equal(result.hasBlock, false)
})

test('mlAcceptanceEvidence: block ends at the next ** line', () => {
  const result = barrier.mlAcceptanceEvidence([
    '**Critérios de aceite:**',
    '- [x] build passes',
    '**Files affected:**',
    '- [ ] this line is outside the block and must not count',
  ])
  assert.equal(result.hasBlock, true)
  assert.equal(result.total, 1)
  assert.equal(result.unmet, 0)
})

// ────────────────────────────────────────────────────────────────────────────
// parseGates — rule 5 (zero gates legal, commands in declaration order, comments/blank ignored)
// ────────────────────────────────────────────────────────────────────────────

test('parseGates: no **Gates da wave:** block declares zero gates', () => {
  const lines = ['## Wave 1 — X', 'no gates block here', '## Wave 2 — Y']
  const result = barrier.parseGates(lines, 0, 2)
  assert.deepEqual(result.commands, [])
})

test('parseGates: commands parsed in declaration order, comments and blank lines skipped', () => {
  const lines = [
    '## Wave 1 — X',
    '**Gates da wave:**',
    '```bash',
    '# a comment line',
    '',
    'make build',
    'make test',
    '```',
    '## Wave 2 — Y',
  ]
  const result = barrier.parseGates(lines, 0, 8)
  assert.deepEqual(result.commands, ['make build', 'make test'])
})

test('parseGates: unterminated fence is a usage error naming the line number', () => {
  const lines = [
    '## Wave 1 — X',
    '**Gates da wave:**',
    '```bash',
    'make build',
  ]
  assert.throws(() => barrier.parseGates(lines, 0, 4), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /line 3/)
    return true
  })
})

// ────────────────────────────────────────────────────────────────────────────
// evalMlsComplete / evalAcceptanceEvidence — evidence/failures formats (pinned strings)
// ────────────────────────────────────────────────────────────────────────────

test('evalMlsComplete: pinned evidence/failures string formats', () => {
  const mls = [
    { id: 'ML-1A', lines: ['**Status:** ✅'] },
    { id: 'ML-1B', lines: ['**Status:** ⬜ Pendente'] },
  ]
  const check = barrier.evalMlsComplete(mls)
  assert.equal(check.name, 'mls_complete')
  assert.equal(check.status, 'blocked')
  assert.deepEqual(check.evidence, ['ML-1A: ✅'])
  assert.deepEqual(check.failures, ['ML-1B: not complete (status: ⬜ Pendente)'])
})

test('evalMlsComplete: wave with zero MLs is blocked', () => {
  const check = barrier.evalMlsComplete([])
  assert.equal(check.status, 'blocked')
})

test('evalAcceptanceEvidence: pinned evidence/failures string formats', () => {
  const mls = [
    { id: 'ML-1A', lines: ['**Critérios de aceite:**', '- [x] a', '- [x] b'] },
    { id: 'ML-1B', lines: ['**Critérios de aceite:**', '- [x] a', '- [ ] b'] },
    { id: 'ML-1C', lines: ['no block'] },
  ]
  const check = barrier.evalAcceptanceEvidence(mls)
  assert.equal(check.name, 'acceptance_evidence')
  assert.equal(check.status, 'blocked')
  assert.deepEqual(check.evidence, ['ML-1A: 2 criteria met'])
  assert.deepEqual(check.failures, [
    'ML-1B: 1 unmet acceptance criteria',
    'ML-1C: no acceptance block',
  ])
})

// ────────────────────────────────────────────────────────────────────────────
// evalGates — command execution, pinned formats, commands array present
// ────────────────────────────────────────────────────────────────────────────

test('evalGates: zero commands passes with empty arrays', () => {
  const check = barrier.evalGates([], process.cwd())
  assert.equal(check.status, 'passed')
  assert.deepEqual(check.commands, [])
  assert.deepEqual(check.evidence, [])
  assert.deepEqual(check.failures, [])
})

test('evalGates: passing and failing commands are recorded with pinned formats', () => {
  const check = barrier.evalGates(['true', 'false'], process.cwd())
  assert.equal(check.status, 'blocked')
  assert.deepEqual(check.evidence, ['true: exit 0'])
  assert.deepEqual(check.failures, ['false: exit 1'])
})

// ────────────────────────────────────────────────────────────────────────────
// buildDoc — determinism contract (key order, arrays never null, commands only on gates)
// ────────────────────────────────────────────────────────────────────────────

test('buildDoc: checks appear in fixed order and top-level failures are prefixed', () => {
  const checks = [
    { name: 'mls_complete', status: 'passed', evidence: ['ML-1A: ✅'], failures: [] },
    { name: 'acceptance_evidence', status: 'blocked', evidence: [], failures: ['ML-1A: 1 unmet acceptance criteria'] },
    { name: 'gates', status: 'passed', commands: [], evidence: [], failures: [] },
    { name: 'validate', status: 'passed', evidence: ['0 violations, 0 warnings'], failures: [] },
  ]
  const started = new Date('2026-07-29T10:30:00.000Z')
  const finished = new Date('2026-07-29T10:30:04.000Z')
  const doc = barrier.buildDoc('ROADMAP-x.md', 2, checks, started, finished)

  assert.equal(doc.status, 'blocked')
  assert.equal(doc.started_at, '2026-07-29T10:30:00Z')
  assert.equal(doc.finished_at, '2026-07-29T10:30:04Z')
  assert.deepEqual(doc.checks.map(c => c.name), ['mls_complete', 'acceptance_evidence', 'gates', 'validate'])
  assert.deepEqual(doc.failures, ['acceptance_evidence: ML-1A: 1 unmet acceptance criteria'])
  assert.ok('commands' in doc.checks[2])
  assert.ok(!('commands' in doc.checks[0]))
  assert.ok(!('commands' in doc.checks[1]))
  assert.ok(!('commands' in doc.checks[3]))
})

// ────────────────────────────────────────────────────────────────────────────
// resolveRoadmapFile — basename with/without .md, wip then done
// ────────────────────────────────────────────────────────────────────────────

test('resolveRoadmapFile: throws UsageError naming the roadmap basename when unresolved', () => {
  const cfg = { roadmapNamespacing: 'flat', roadmapDir: '/nonexistent/dir/for/barrier/tests', agents: [] }
  assert.throws(() => barrier.resolveRoadmapFile(cfg, 'ROADMAP-does-not-exist'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /ROADMAP-does-not-exist\.md/)
    return true
  })
})
