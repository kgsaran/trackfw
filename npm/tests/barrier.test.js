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

// unfenced(n) builds a "nothing is inside a fence" mask for tests that
// exercise mlCompletionStatus/mlAcceptanceEvidence directly with hand-built
// line arrays. Required explicitly since hades-tf achado #3 (2026-08-29)
// made the `fenced` default fail CLOSED (mask everything) instead of open —
// see the comment on mlCompletionStatus/mlAcceptanceEvidence in
// src/commands/barrier.js.
const unfenced = (n) => new Array(n).fill(false)

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
  const result = barrier.mlCompletionStatus(['**Status:** ✅ Concluído'], unfenced(1))
  assert.equal(result.complete, true)
})

test('mlCompletionStatus: any other marker is incomplete', () => {
  for (const marker of ['⬜ Pendente', '🔄 Em andamento', '❌ Bloqueado']) {
    const result = barrier.mlCompletionStatus([`**Status:** ${marker}`], unfenced(1))
    assert.equal(result.complete, false, `expected ${marker} to be incomplete`)
    assert.equal(result.marker, marker)
  }
})

test('mlCompletionStatus: absence of a **Status:** line is incomplete with marker "missing"', () => {
  const result = barrier.mlCompletionStatus(['no status line here'], unfenced(1))
  assert.equal(result.complete, false)
  assert.equal(result.marker, 'missing')
})

// ────────────────────────────────────────────────────────────────────────────
// mlAcceptanceEvidence — rule 4 (including the anti-vacuity case)
// ────────────────────────────────────────────────────────────────────────────

test('mlAcceptanceEvidence: all criteria met', () => {
  const lines = [
    '**Critérios de aceite:**',
    '- [x] build passes',
    '- [x] tests pass',
  ]
  const result = barrier.mlAcceptanceEvidence(lines, unfenced(lines.length))
  assert.equal(result.hasBlock, true)
  assert.equal(result.total, 2)
  assert.equal(result.unmet, 0)
})

test('mlAcceptanceEvidence: unmet criteria counted', () => {
  const lines = [
    '**Critérios de aceite:**',
    '- [x] build passes',
    '- [ ] tests pass',
    '- [ ] gate passes',
  ]
  const result = barrier.mlAcceptanceEvidence(lines, unfenced(lines.length))
  assert.equal(result.hasBlock, true)
  assert.equal(result.unmet, 2)
})

test('mlAcceptanceEvidence: absent block is not vacuously satisfied', () => {
  const result = barrier.mlAcceptanceEvidence(['no acceptance block at all'], unfenced(1))
  assert.equal(result.hasBlock, false)
})

test('mlAcceptanceEvidence: block header with zero criteria lines is treated as absent', () => {
  const lines = [
    '**Critérios de aceite:**',
    '**Next section:**',
  ]
  const result = barrier.mlAcceptanceEvidence(lines, unfenced(lines.length))
  assert.equal(result.hasBlock, false)
})

test('mlAcceptanceEvidence: block ends at the next ** line', () => {
  const lines = [
    '**Critérios de aceite:**',
    '- [x] build passes',
    '**Files affected:**',
    '- [ ] this line is outside the block and must not count',
  ]
  const result = barrier.mlAcceptanceEvidence(lines, unfenced(lines.length))
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

test('parseGates: header is a PREFIX match, matching Go/Python — trailing prose on the header line is still recognised (ML-1B)', () => {
  const lines = [
    '## Wave 1 — X',
    '**Gates da wave:** (obrigatórios)',
    '```bash',
    'make build',
    '```',
    '## Wave 2 — Y',
  ]
  const result = barrier.parseGates(lines, 0, 5)
  assert.deepEqual(result.commands, ['make build'])
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
  // fenced: [false] explicit — hades-tf achado #3 made the "fenced" default
  // fail CLOSED (everything masked) instead of open, so a hand-built ml
  // object exercising this path must say explicitly that its line is not
  // inside a fence.
  const mls = [
    { id: 'ML-1A', lines: ['**Status:** ✅'], fenced: [false] },
    { id: 'ML-1B', lines: ['**Status:** ⬜ Pendente'], fenced: [false] },
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
  // fenced explicit, all-false — see the comment on the equivalent
  // evalMlsComplete test above (hades-tf achado #3, fail-closed default).
  const mls = [
    { id: 'ML-1A', lines: ['**Critérios de aceite:**', '- [x] a', '- [x] b'], fenced: [false, false, false] },
    { id: 'ML-1B', lines: ['**Critérios de aceite:**', '- [x] a', '- [ ] b'], fenced: [false, false, false] },
    { id: 'ML-1C', lines: ['no block'], fenced: [false] },
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

// exit 127 signals "tool not found inside sh" — sh itself started and ran.
// This must NEVER be confused with sh itself being missing (ML-0A measurement).
test('evalGates: a missing tool inside sh is a normal exit 127, not not_evaluated', () => {
  const check = barrier.evalGates(['nosuchtool-xyz'], process.cwd())
  assert.equal(check.status, 'blocked')
  assert.deepEqual(check.failures, ['nosuchtool-xyz: exit 127'])
})

test('evalGates: sh missing from $PATH reports not_evaluated with the pinned message', () => {
  const fs = require('fs')
  const os = require('os')
  const path = require('path')
  const curated = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-no-sh-'))
  const originalPath = process.env.PATH
  try {
    process.env.PATH = curated
    const check = barrier.evalGates(['true', 'false'], process.cwd())
    assert.equal(check.status, 'not_evaluated')
    assert.deepEqual(check.evidence, [])
    assert.deepEqual(check.failures, [
      'gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates',
    ])
  } finally {
    process.env.PATH = originalPath
    fs.rmSync(curated, { recursive: true, force: true })
  }
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

// ────────────────────────────────────────────────────────────────────────────
// statusIsComplete — first-token vocabulary (ADR decision 3/4/8, AC8/AC9/AC14)
// ────────────────────────────────────────────────────────────────────────────

test('statusIsComplete: accepted forms (ADR-pinned, including suffixed ones)', () => {
  const accepted = [
    '✅',
    '✅ Concluído',
    '✅ Concluído · **Agente:** `apolo-tf`',
    '✅ concluído (auditado 2026-08-02)',
    'done',
    'Concluído',
    'DONE',
    'concluido',
    'done\t· extra', // tab after the marker is a valid separator
    'done · extra', // NBSP (U+00A0) after the marker is a valid separator
    '✅️', // VS16 (U+FE0F) text-style emoji presentation — the single Mn exception (ADR decision 9)
  ]
  for (const marker of accepted) {
    assert.equal(barrier.statusIsComplete(marker), true, `expected ${JSON.stringify(marker)} to be complete`)
  }
})

test('statusIsComplete: rejected forms — AC9 falsified in the opposite direction', () => {
  const rejected = [
    'não done',
    'pending (era done)',
    'notdone',
    'done-not-really',
    '⬜ Pendente',
    '🔄 Em andamento',
    '❌ Bloqueado',
    '⬜ Pendente ✅', // AC14 — position matters; today (includes) this passes in prod
    '`done`', // marker inside inline code — backticks glue to the token
    '​done', // zero-width space before the token — not \s, stays glued
    '',
    '   ',
    'd᷀one', // AC15 (ADR decision 9) — combining mark (U+1DC0) on the first token, rejected outright, not folded
    'do᷀ne', // AC15 — same, mark on a different codepoint of the token
    'done᷀', // AC15 — same, mark trailing the token
    '✅᷀', // AC15 — combining mark on the emoji marker itself, still rejected
  ]
  for (const marker of rejected) {
    assert.equal(barrier.statusIsComplete(marker), false, `expected ${JSON.stringify(marker)} to be incomplete`)
  }
})

test('CRITERIA_HEADER_RE: accepts English and Portuguese headers, anchored (AC1/AC2/AC3)', () => {
  const accepted = [
    '**Acceptance criteria:**',
    '**Critérios de aceite:**',
    '**Criterios de aceite:**',
  ]
  for (const line of accepted) {
    assert.match(line, barrier.CRITERIA_HEADER_RE)
  }
  const rejected = [
    'the header is **Acceptance criteria:**',
    '> **Critérios de aceite:**',
    'prose citing **Acceptance criteria:** mid-sentence',
  ]
  for (const line of rejected) {
    assert.doesNotMatch(line, barrier.CRITERIA_HEADER_RE)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// Fence-awareness (ADR decision 7, AC13) — mlStatusMarker/acceptanceEvaluate/
// findMLs must ignore content inside ``` fences. Reproduces forged.md and
// forged3.md from the ML-0A threat-model result, verbatim.
// ────────────────────────────────────────────────────────────────────────────

test('fence-awareness: status inside a fence is ignored (forged.md)', () => {
  const lines = [
    '### ML-1A — probe',
    'Example of the bug we are documenting:',
    '```',
    '**Status:** done',
    '```',
    '**Status:** pending',
  ]
  const fenceMask = barrier.computeFenceMask(lines)
  const mls = barrier.findMLs(lines, 0, lines.length, fenceMask)
  assert.equal(mls.length, 1)
  const { complete, marker } = barrier.mlCompletionStatus(mls[0].lines, mls[0].fenced)
  assert.equal(marker, 'pending', 'expected the real, unfenced status')
  assert.equal(complete, false, 'the fenced "done" must not leak into the real status')
})

test('fence-awareness: acceptance block inside a fence is ignored (forged3.md)', () => {
  const lines = [
    '### ML-1A — probe',
    'Example of the bug we are documenting:',
    '```',
    '**Critérios de aceite:**',
    '- [x] fake evidence, nothing built',
    '```',
    '**Status:** ✅',
  ]
  const fenceMask = barrier.computeFenceMask(lines)
  const mls = barrier.findMLs(lines, 0, lines.length, fenceMask)
  assert.equal(mls.length, 1)
  const ev = barrier.mlAcceptanceEvidence(mls[0].lines, mls[0].fenced)
  assert.equal(ev.hasBlock, false, 'the fenced acceptance block must not count as real evidence')
})

test('fence-awareness: a "### ML-XX" heading inside a fence is not a phantom ML (AC13-b)', () => {
  const lines = [
    '## Wave 1 — Foo',
    '### ML-1A — Real ML',
    '**Status:** ✅',
    '**Critérios de aceite:**',
    '- [x] real criterion',
    '',
    'Example of a malformed heading inside a fence, cited as documentation:',
    '```markdown',
    '### ML-9Z — phantom, must not be detected',
    '**Status:** ⬜ Pendente',
    '```',
  ]
  const fenceMask = barrier.computeFenceMask(lines)
  const mls = barrier.findMLs(lines, 0, lines.length, fenceMask)
  assert.equal(mls.length, 1, `expected 1 ML, got ${mls.length}: ${JSON.stringify(mls.map(m => m.id))}`)
  assert.equal(mls[0].id, 'ML-1A')
})

// ────────────────────────────────────────────────────────────────────────────
// End-to-end CLI regressions — real binary (AC1/AC12, AC13)
// ────────────────────────────────────────────────────────────────────────────

test('barrier CLI: English header + word status passes end-to-end (AC1/AC12)', () => {
  const content =
    '# Roadmap: English dialect fixture\n\n' +
    'REQ: REQ-2026-08-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Fixture Wave\n> Dependencies: none\n\n' +
    '### ML-1A — Fixture ML\n' +
    '**Status:** done\n' +
    '**Acceptance criteria:**\n' +
    '- [x] build passes\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 0, `expected exit 0, got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    for (const name of ['mls_complete', 'acceptance_evidence']) {
      const check = doc.checks.find((c) => c.name === name)
      assert.equal(check.status, 'passed', `expected ${name}=passed, got ${check.status} (failures: ${JSON.stringify(check.failures)})`)
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier CLI: forged fence content does not liberate the wave (AC13, end-to-end)', () => {
  const content =
    '# Roadmap: Forged fence fixture\n\n' +
    'REQ: REQ-2026-08-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Fixture Wave\n> Dependencies: none\n\n' +
    '### ML-1A — Fixture ML\n' +
    'Example of the bug we are documenting:\n' +
    '```\n' +
    '**Status:** done\n' +
    '**Critérios de aceite:**\n' +
    '- [x] fake evidence, nothing built\n' +
    '```\n' +
    '**Status:** pending\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.equal(mlsCheck.status, 'blocked')
    assert.deepEqual(mlsCheck.failures, ['ML-1A: not complete (status: pending)'])
    const accCheck = doc.checks.find((c) => c.name === 'acceptance_evidence')
    assert.equal(accCheck.status, 'blocked')
    assert.deepEqual(accCheck.failures, ['ML-1A: no acceptance block'])
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// ────────────────────────────────────────────────────────────────────────────
// ML-1B achado 1 — computeFenceMask must recognise ~~~ and 4+-backtick fences
// per CommonMark (3+ of the SAME character, closed by a run of the same
// character with length >= the opening run).
// ────────────────────────────────────────────────────────────────────────────

test('computeFenceMask: ~~~ (3+ tildes) masks the same as ``` (ML-1B achado 1)', () => {
  const lines = ['before', '~~~', 'inside', '~~~', 'after']
  assert.deepEqual(barrier.computeFenceMask(lines), [false, false, true, false, false])
})

test('computeFenceMask: a 4-backtick fence masks its entire interior, including a nested 3-backtick block (ML-1B achado 1)', () => {
  const lines = [
    'before',
    '````',
    'outer',
    '```',
    'nested (shorter run, must stay masked as interior)',
    '```',
    'still outer',
    '````',
    'after',
  ]
  assert.deepEqual(
    barrier.computeFenceMask(lines),
    [false, false, true, true, true, true, true, false, false],
  )
})

test('computeFenceMask: closing fence requires the SAME character and length >= opening (ML-1B achado 1)', () => {
  // A ``` line inside a ~~~ fence does not close it (different character).
  const lines = ['~~~', '```', 'still inside', '~~~']
  assert.deepEqual(barrier.computeFenceMask(lines), [false, true, true, false])
})

test('computeFenceMask: a LONGER closing run of the same character closes the fence (ML-1B achado 1)', () => {
  const lines = ['before', '```', 'inside', '`````', 'after']
  assert.deepEqual(barrier.computeFenceMask(lines), [false, false, true, false, false])
})

// ────────────────────────────────────────────────────────────────────────────
// ML-1B achado 2 — Status/acceptance-header/criterion-item/gates-header
// markers must be matched against the RAW line (column 0), never a
// per-line-trimmed line: an indented marker is not recognised, aligning
// Node with Go/Python (which already require column 0 via `^`).
// ────────────────────────────────────────────────────────────────────────────

test('mlCompletionStatus: an indented "**Status:**" line is not recognised (ML-1B achado 2)', () => {
  const result = barrier.mlCompletionStatus(['  **Status:** done'])
  assert.equal(result.complete, false)
  assert.equal(result.marker, 'missing')
})

test('mlAcceptanceEvidence: an indented acceptance header/criteria are not recognised (ML-1B achado 2)', () => {
  const result = barrier.mlAcceptanceEvidence([
    '  **Critérios de aceite:**',
    '  - [x] indented criterion',
  ])
  assert.equal(result.hasBlock, false)
})

test('barrier CLI: forged ~~~ fence does not liberate the wave (ML-1B achado 1, end-to-end)', () => {
  const content =
    '# Roadmap: Tilde fence fixture\n\n' +
    'REQ: REQ-2026-08-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Fixture Wave\n> Dependencies: none\n\n' +
    '### ML-1A — Real ML\n' +
    '**Status:** ⬜ Pendente\n' +
    '**Critérios de aceite:**\n' +
    '- [ ] real unmet criterion\n\n' +
    'Example of a phantom ML hidden inside a tilde fence:\n' +
    '~~~\n' +
    '### ML-9Z — phantom\n' +
    '**Status:** done\n' +
    '**Critérios de aceite:**\n' +
    '- [x] fake\n' +
    '~~~\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.deepEqual(mlsCheck.failures, ['ML-1A: not complete (status: ⬜ Pendente)'], 'the phantom ML-9Z must not exist')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier CLI: forged 4-backtick fence with a nested 3-backtick block does not liberate the wave (ML-1B achado 1, end-to-end)', () => {
  const content =
    '# Roadmap: Nested fence fixture\n\n' +
    'REQ: REQ-2026-08-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Fixture Wave\n> Dependencies: none\n\n' +
    '### ML-1A — Real ML\n' +
    '**Status:** ⬜ Pendente\n' +
    '**Critérios de aceite:**\n' +
    '- [ ] real unmet criterion\n\n' +
    'Example nesting a 3-backtick fence inside a 4-backtick fence:\n' +
    '````\n' +
    'outer fence, then a nested doc block:\n' +
    '```\n' +
    '### ML-9Z — nested phantom\n' +
    '**Status:** done\n' +
    '**Critérios de aceite:**\n' +
    '- [x] fake\n' +
    '```\n' +
    'still inside the outer fence\n' +
    '````\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.deepEqual(mlsCheck.failures, ['ML-1A: not complete (status: ⬜ Pendente)'], 'the phantom ML-9Z must not exist')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier CLI: indented markers are not recognised, matching Go/Python strict behaviour (ML-1B achado 2, end-to-end)', () => {
  const content =
    '# Roadmap: Indented marker fixture\n\n' +
    'REQ: REQ-2026-08-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Fixture Wave\n> Dependencies: none\n\n' +
    '### ML-1A — Real ML\n' +
    '  **Status:** done\n' +
    '  **Critérios de aceite:**\n' +
    '  - [x] indented criterion\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.equal(mlsCheck.status, 'blocked')
    assert.deepEqual(mlsCheck.failures, ['ML-1A: not complete (status: missing)'])
    const accCheck = doc.checks.find((c) => c.name === 'acceptance_evidence')
    assert.equal(accCheck.status, 'blocked')
    assert.deepEqual(accCheck.failures, ['ML-1A: no acceptance block'])
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier CLI: gates header with trailing prose still runs the gate (ML-1B regression, end-to-end)', () => {
  // Regression this ML found while fixing achado 2: the header marker must be
  // a PREFIX match (Go's gatesHeaderRe / Python's _GATES_HEADER_RE), not
  // full-line equality — a full-line-equality regression would silently skip
  // the gate (gates: passed, zero commands) instead of running and blocking on it.
  const content =
    '# Roadmap: Gates header prefix fixture\n\n' +
    'REQ: REQ-2026-08-29-barrier-fixture\n\n' +
    '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n' +
    '## Wave 1 — Fixture Wave\n\n' +
    '**Gates da wave:** (obrigatórios)\n' +
    '```bash\n' +
    'false\n' +
    '```\n\n' +
    '### ML-1A — Real ML\n' +
    '**Status:** done\n' +
    '**Critérios de aceite:**\n' +
    '- [x] build passes\n'
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const gatesCheck = doc.checks.find((c) => c.name === 'gates')
    assert.equal(gatesCheck.status, 'blocked', `expected the prefixed header to be recognised and "false" executed, got commands=${JSON.stringify(gatesCheck.commands)}`)
    assert.deepEqual(gatesCheck.commands, ['false'])
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

// ────────────────────────────────────────────────────────────────────────────
// ML-3C — CRLF roadmaps. Found in audit: a roadmap saved with CRLF line
// endings reported mls_complete: passed in Go and Python but blocked
// ("status: missing") in Node, on a ML whose "**Status:**" line was fully
// filled in. JS regex "." excludes "\r" (a LineTerminator per the ECMAScript
// spec), so `/^\*\*Status:\*\*(.*)$/` never matched
// "**Status:** ✅ Concluído\r". Fixed by normalizing CRLF once, at the
// line-split boundary (splitRoadmapLines), instead of patching every marker
// regex.
// ────────────────────────────────────────────────────────────────────────────

test('splitRoadmapLines: strips exactly a trailing \\r per line, nothing more (ML-3C)', () => {
  assert.deepEqual(
    barrier.splitRoadmapLines('  **Status:** done\r\n**Status:** done\r\nlast\r'),
    ['  **Status:** done', '**Status:** done', 'last'],
  )
})

test('barrier regression: CRLF roadmap with a fully completed ML passes (ML-3C)', () => {
  const content = [
    '# Roadmap: CRLF Fixture',
    '',
    'REQ: REQ-2026-07-29-barrier-fixture',
    '',
    '## Acceptance Criteria',
    '- [x] fixture roadmap-level criterion',
    '',
    '## Wave 1 — Fixture Wave',
    '',
    '### ML-1A — Real ML',
    '**Status:** ✅ Concluído',
    '**Critérios de aceite:**',
    '- [x] real met criterion',
    '',
  ].join('\r\n')
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 0, `expected exit 0 (passed), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'passed')
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.equal(mlsCheck.status, 'passed')
    assert.deepEqual(mlsCheck.failures, [])
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier regression: CRLF roadmap with a pending ML still blocks (ML-3C — not laundered by the fix)', () => {
  const content = [
    '# Roadmap: CRLF Fixture',
    '',
    'REQ: REQ-2026-07-29-barrier-fixture',
    '',
    '## Acceptance Criteria',
    '- [x] fixture roadmap-level criterion',
    '',
    '## Wave 1 — Fixture Wave',
    '',
    '### ML-1A — Real ML',
    '**Status:** ⬜ Pendente',
    '**Critérios de aceite:**',
    '- [ ] real unmet criterion',
    '',
  ].join('\r\n')
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.deepEqual(mlsCheck.failures, ['ML-1A: not complete (status: ⬜ Pendente)'])
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('barrier regression: CRLF roadmap — fence masking and indented-marker strictness (ML-1B) both still hold (ML-3C)', () => {
  const content = [
    '# Roadmap: CRLF Fixture',
    '',
    'REQ: REQ-2026-07-29-barrier-fixture',
    '',
    '## Acceptance Criteria',
    '- [x] fixture roadmap-level criterion',
    '',
    '## Wave 1 — Fixture Wave',
    '',
    '### ML-1A — Real ML',
    '**Status:** ⬜ Pendente',
    '**Critérios de aceite:**',
    '- [ ] real unmet criterion',
    '',
    'Example of a phantom ML hidden inside a fence:',
    '```',
    '### ML-9Z — phantom',
    '**Status:** done',
    '**Critérios de aceite:**',
    '- [x] fake',
    '```',
    '',
    '### ML-1B — indented marker must stay unrecognised',
    '  **Status:** done',
    '  **Critérios de aceite:**',
    '  - [x] indented criterion',
    '',
  ].join('\r\n')
  const dir = setupRegressionFixture(content)
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-regression', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (blocked), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)
    const doc = JSON.parse(stdout)
    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    // No "ML-9Z" anywhere: the fence hid the phantom ML entirely.
    assert.deepEqual(mlsCheck.failures, [
      'ML-1A: not complete (status: ⬜ Pendente)',
      'ML-1B: not complete (status: missing)',
    ])
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})
