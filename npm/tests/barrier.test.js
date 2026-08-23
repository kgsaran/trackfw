'use strict'

// barrier.test.js — testes unitários próprios do parser e dos checks de
// `trackfw barrier`, adicionais aos testes de contrato universal em
// barrier-contract.test.js (que fixam o contrato de docs/cli-parity.md).

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const barrier = require('../src/commands/barrier')

const CLI = path.resolve(__dirname, '../bin/trackfw')

// ────────────────────────────────────────────────────────────────────────────
// CLI-level regression fixtures — the two defects found while cross-checking
// the three runtimes over the same fixture (ML-2D). These are NOT part of the
// frozen contract in barrier-contract.test.js; they cover concrete literal
// messages that had zero coverage before ML-2D.
// ────────────────────────────────────────────────────────────────────────────

function setupRegressionFixture(roadmapContent) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-barrier-regression-'))
  for (const d of [
    'docs/roadmaps/wip', 'docs/roadmaps/backlog', 'docs/roadmaps/blocked',
    'docs/roadmaps/done', 'docs/roadmaps/abandoned', 'docs/req', 'docs/adr',
  ]) {
    fs.mkdirSync(path.join(dir, d), { recursive: true })
  }
  fs.writeFileSync(path.join(dir, 'docs/roadmaps/wip/ROADMAP-regression.md'), roadmapContent)
  return dir
}

function runBarrierCLI(cwd, ...args) {
  const result = spawnSync(process.execPath, [CLI, 'barrier', ...args], { encoding: 'utf8', cwd })
  return { stdout: result.stdout || '', stderr: result.stderr || '', status: result.status }
}

test('barrier regression: wave with zero MLs fails mls_complete with the pinned message', () => {
  const content =
    '# Roadmap: No ML\n\nREQ: REQ-2026-07-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Sem MLs\n> Dependências: nenhuma\n\nSome prose, no ML heading at all.\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.deepEqual(mlsCheck.failures, ['wave 1: no ML found'])
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier regression: --wave 2-bis resolves ## Wave 2-bis heading at CLI level', () => {
  const content =
    '# Roadmap: Suffix\n\nREQ: REQ-2026-07-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 2-bis — Corrective\n> Dependências: nenhuma\n\n' +
    '### ML-2A — Fixture ML\n**Status:** ✅\n**Critérios de aceite:**\n- [x] build passes\n\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '2-bis', '--json')
    assert.equal(status, 0, `expected exit 0 (passed), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'passed')
    assert.equal(doc.wave, '2-bis', `expected doc.wave to be string "2-bis", got ${JSON.stringify(doc.wave)}`)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier regression: --wave 2 does NOT match ## Wave 2-bis at CLI level', () => {
  const content =
    '# Roadmap: Suffix\n\nREQ: REQ-2026-07-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 2-bis — Corrective\n> Dependências: nenhuma\n\n' +
    '### ML-2A — Fixture ML\n**Status:** ✅\n**Critérios de aceite:**\n- [x] build passes\n\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '2', '--json')
    assert.equal(status, 2, `expected exit 2 (wave not found), got ${status}`)
    assert.equal(
      stderr,
      'trackfw barrier: wave 2 not found in roadmap "ROADMAP-regression.md"\n'
    )
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier regression: ABORT — malformed wave heading aborts entire document for every --wave value', () => {
  // Document has a malformed heading (## Wave X) plus valid waves 1 and 2.
  // Every --wave request must exit 2 with the pinned malformed message, proving
  // decision 16 (abort is intentional, not a skip).
  const content =
    '# Roadmap: Malformed\n\nREQ: REQ-2026-07-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave X — Malformed heading intentionally\n> this causes the full document abort\n\n' +
    '### ML-XA — ML under malformed wave\n**Status:** ✅\n**Critérios de aceite:**\n- [x] x\n\n' +
    '## Wave 1 — First\n> Dependências: nenhuma\n\n' +
    '### ML-1A — First ML\n**Status:** ✅\n**Critérios de aceite:**\n- [x] build passes\n\n' +
    '## Wave 2 — Second\n> Dependências: nenhuma\n\n' +
    '### ML-2A — Second ML\n**Status:** ✅\n**Critérios de aceite:**\n- [x] tests pass\n\n'
  const dir = setupRegressionFixture(content)
  try {
    for (const waveArg of ['1', '2', 'X']) {
      const { stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', waveArg)
      // --wave X is itself an invalid label → exit 2 from CLI validation, not from parser
      // --wave 1 and --wave 2 must abort due to ## Wave X heading in the document
      assert.equal(status, 2, `--wave ${waveArg}: expected exit 2, got ${status}\nstderr: ${stderr}`)
      if (waveArg !== 'X') {
        assert.match(
          stderr, /malformed wave heading/,
          `--wave ${waveArg}: expected "malformed wave heading" in stderr, got: ${stderr}`
        )
        assert.match(
          stderr, /"X" is not a valid wave label/,
          `--wave ${waveArg}: expected token "X" named in stderr, got: ${stderr}`
        )
      }
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier regression: exit-2 messages are pinned literally and byte-identical to the contract', () => {
  const content =
    '# Roadmap: Fixture\n\nREQ: REQ-2026-07-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Fixture Wave\n> Dependências: nenhuma\n\n' +
    '### ML-1A — Fixture ML\n**Status:** ✅\n**Critérios de aceite:**\n- [x] build passes\n\n'
  const dir = setupRegressionFixture(content)
  try {
    const wave = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '99', '--json')
    assert.equal(wave.status, 2, `expected exit 2, got ${wave.status}\nstderr: ${wave.stderr}`)
    assert.equal(
      wave.stderr,
      'trackfw barrier: wave 99 not found in roadmap "ROADMAP-regression.md"\n'
    )

    const missing = runBarrierCLI(dir, 'ROADMAP-nao-existe', '--wave', '1', '--json')
    assert.equal(missing.status, 2, `expected exit 2, got ${missing.status}\nstderr: ${missing.stderr}`)
    assert.equal(
      missing.stderr,
      'trackfw barrier: roadmap "ROADMAP-nao-existe" not found in wip/ nor done/ under docs/roadmaps\n'
    )
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier regression: invalid --wave message is pinned literally (fourth exit-2 message)', () => {
  // The fourth pinned exit-2 message is the invalid --wave argument.
  // Canonical text (from Go): trackfw barrier: invalid --wave "<value>" — not a valid wave label
  // The separator is an em dash U+2014, not a hyphen.
  // This test verifies byte-identity against the Go canonical form.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-barrier-invalid-wave-'))
  try {
    const result = runBarrierCLI(dir, 'any-roadmap', '--wave', '2-BIS')
    assert.equal(result.status, 2, `expected exit 2, got ${result.status}\nstderr: ${result.stderr}`)
    assert.equal(
      result.stderr,
      'trackfw barrier: invalid --wave "2-BIS" — not a valid wave label\n'
    )
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

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
  const wave1 = barrier.findWave(lines, '1')
  assert.equal(wave1.startLine, 2)
  assert.equal(wave1.endLine, 4)

  const wave2 = barrier.findWave(lines, '2')
  assert.equal(wave2.startLine, 4)
  assert.equal(wave2.endLine, 6)
})

test('findWave: throws UsageError naming the wave label when not found', () => {
  const lines = ['## Wave 1 — Only']
  assert.throws(() => barrier.findWave(lines, '7'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /wave 7/)
    return true
  })
})

test('findWave: usage error message is pinned literally (docs/cli-parity.md)', () => {
  const lines = ['## Wave 1 — Only']
  assert.throws(() => barrier.findWave(lines, '7', 'ROADMAP-example.md'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.equal(err.message, 'wave 7 not found in roadmap "ROADMAP-example.md"')
    return true
  })
})

test('findWave: throws UsageError naming the line number for a malformed wave label', () => {
  const lines = ['# Title', '## Wave abc — Broken']
  assert.throws(() => barrier.findWave(lines, '1'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /line 2/)
    return true
  })
})

// ────────────────────────────────────────────────────────────────────────────
// findWave — wave label with suffix (ML-2B: barrier aceita wave com sufixo bis)
// ────────────────────────────────────────────────────────────────────────────

test('isValidWaveLabel: valid labels (contract table)', () => {
  // "0" is the Wave 0 threat-model convention (docs/cli-parity.md § "Wave label
  // grammar"; ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness ML-1A).
  for (const label of ['0', '1', '2', '2-bis', '2-hotfix', '10-a2']) {
    assert.ok(barrier.isValidWaveLabel(label), `expected "${label}" to be valid`)
  }
})

test('isValidWaveLabel: invalid labels (contract table)', () => {
  for (const label of ['X', '2-BIS', '-bis', '2-', '2-bis-ter']) {
    assert.ok(!barrier.isValidWaveLabel(label), `expected "${label}" to be invalid`)
  }
})

test('findWave: resolves wave by label including suffix (2-bis)', () => {
  const lines = [
    '## Wave 2 — Second',
    'content 2',
    '## Wave 2-bis — Corrective',
    'content 2-bis',
    '## Wave 3 — Third',
    'content 3',
  ]
  const wave = barrier.findWave(lines, '2-bis')
  assert.equal(wave.startLine, 2)
  assert.equal(wave.endLine, 4)
})

test('findWave: --wave 2 does not match ## Wave 2-bis (labels are distinct identities)', () => {
  const lines = [
    '## Wave 2-bis — Corrective',
    'content 2-bis',
  ]
  assert.throws(() => barrier.findWave(lines, '2', 'ROADMAP-example.md'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.equal(err.message, 'wave 2 not found in roadmap "ROADMAP-example.md"')
    return true
  })
})

test('findWave: malformed error message contains the token, not the entire heading line', () => {
  const lines = ['## Wave 2-BIS — Uppercase suffix should be rejected']
  assert.throws(() => barrier.findWave(lines, '1'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    // Token is '2-BIS'; line starts with '## Wave 2-BIS ...'
    assert.match(err.message, /"2-BIS" is not a valid wave label/)
    assert.ok(
      !err.message.includes('## Wave 2-BIS — Uppercase suffix should be rejected'),
      'error message must not contain the full heading line'
    )
    return true
  })
})

test('findWave: REGRESSION — malformed heading aborts entire document regardless of which wave is requested (ADR decision 16)', () => {
  // ## Wave X appears before ## Wave 1. Even requesting '1' (which comes after)
  // must abort because all headings are pre-validated before any search.
  const lines = [
    '## Wave X — Malformed heading',
    'content X',
    '## Wave 1 — First',
    'content 1',
    '## Wave 2 — Second',
    'content 2',
  ]
  // Requesting wave '1': scanner hits '## Wave X' first → must abort
  assert.throws(() => barrier.findWave(lines, '1'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /malformed wave heading at line 1/)
    assert.match(err.message, /"X" is not a valid wave label/)
    return true
  })
  // Requesting wave '2': same abort
  assert.throws(() => barrier.findWave(lines, '2'), (err) => {
    assert.ok(err instanceof barrier.UsageError)
    assert.match(err.message, /malformed wave heading/)
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
  const check = barrier.evalMlsComplete([], '1')
  assert.equal(check.status, 'blocked')
})

test('evalMlsComplete: wave with zero MLs pins the failure message literally (docs/cli-parity.md)', () => {
  const check = barrier.evalMlsComplete([], '3')
  assert.deepEqual(check.failures, ['wave 3: no ML found'])
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
    assert.match(err.message, /ROADMAP-does-not-exist/)
    return true
  })
})

test('resolveRoadmapFile: usage error message is pinned literally, using the arg as typed (no .md normalization)', () => {
  const cfg = { roadmapNamespacing: 'flat', roadmapDir: '/nonexistent/dir/for/barrier/tests', agents: [] }
  assert.throws(() => barrier.resolveRoadmapFile(cfg, 'ROADMAP-does-not-exist'), (err) => {
    assert.equal(
      err.message,
      'roadmap "ROADMAP-does-not-exist" not found in wip/ nor done/ under /nonexistent/dir/for/barrier/tests'
    )
    return true
  })
})
