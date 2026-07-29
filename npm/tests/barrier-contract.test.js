'use strict'

// barrier-contract.test.js — Contrato universal de `trackfw barrier`, congelado em
// docs/cli-parity.md (seção `## trackfw barrier`). Estes testes NÃO implementam produção:
// eles fixam, nos três runtimes, os oito cenários obrigatórios definidos pelo
// ML-1A do roadmap ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.
//
// Mecanismo de pendência: cada test() recebe `{ skip: '...' }` do node:test. O corpo real
// (fixture + invocação do binário real + asserções do contrato) permanece escrito — o
// ML-2B deve REMOVER a opção `skip`, não reescrever o teste.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const CLI = path.resolve(__dirname, '../bin/trackfw')
const SKIP_REASON = 'pendente até ML-2B: trackfw barrier ainda não implementado'

// ────────────────────────────────────────────────────────────────────────────
// Fixture builder — reproduz as regras de parsing string-level da seção
// "Roadmap parsing rules" de docs/cli-parity.md.
// ────────────────────────────────────────────────────────────────────────────

/**
 * @param {{
 *   linkedREQ?: boolean,
 *   mlStatus?: string,
 *   criteriaLines?: string[],
 *   omitCriteriaBlock?: boolean,
 *   gateCommands?: string[] | null,
 * }} cfg
 */
function buildBarrierRoadmap(cfg) {
  const {
    linkedREQ = true,
    mlStatus = '✅',
    criteriaLines = [],
    omitCriteriaBlock = false,
    gateCommands = null,
  } = cfg

  let out = '# Roadmap: Barrier Contract Fixture\n\n'
  if (linkedREQ) {
    out += 'REQ: REQ-2026-07-29-barrier-fixture\n\n'
  }
  // Bloco de aceite em nível de roadmap — satisfaz wip_acceptance (governança),
  // distinto do bloco por-ML usado pela barrier (rule 4).
  out += '## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n'

  out += '## Wave 1 — Fixture Wave\n> Dependências: nenhuma\n\n'
  if (gateCommands !== null) {
    out += '**Gates da wave:**\n```bash\n'
    for (const c of gateCommands) out += c + '\n'
    out += '```\n\n'
  }

  out += '### ML-1A — Fixture ML\n'
  out += '**Status:** ' + mlStatus + '\n'
  if (!omitCriteriaBlock) {
    out += '**Critérios de aceite:**\n'
    for (const line of criteriaLines) out += line + '\n'
  }
  out += '\n'
  return out
}

/**
 * Escreve a árvore de governança + o roadmap de fixture em um diretório temporário
 * e devolve o caminho do diretório.
 */
function setupBarrierFixture(cfg) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-barrier-'))
  for (const d of [
    'docs/roadmaps/wip', 'docs/roadmaps/backlog', 'docs/roadmaps/blocked',
    'docs/roadmaps/done', 'docs/roadmaps/abandoned', 'docs/req', 'docs/adr',
  ]) {
    fs.mkdirSync(path.join(dir, d), { recursive: true })
  }
  fs.writeFileSync(
    path.join(dir, 'docs/roadmaps/wip/ROADMAP-barrier-fixture.md'),
    buildBarrierRoadmap(cfg)
  )
  return dir
}

/** Invoca `trackfw barrier <args>` em `cwd` e devolve { stdout, stderr, status }. */
function runBarrierCLI(cwd, ...args) {
  const result = spawnSync(process.execPath, [CLI, 'barrier', ...args], {
    encoding: 'utf8',
    cwd,
  })
  return {
    stdout: result.stdout || '',
    stderr: result.stderr || '',
    status: result.status,
  }
}

function cleanupFixture(dir) {
  fs.rmSync(dir, { recursive: true, force: true })
}

// ────────────────────────────────────────────────────────────────────────────
// 1 — wave_verde_passa
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: wave_verde_passa', () => {
  const dir = setupBarrierFixture({
    linkedREQ: true,
    mlStatus: '✅',
    criteriaLines: ['- [x] build passes', '- [x] tests pass'],
    gateCommands: null, // sem bloco de gates — zero gates é legal
  })
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '1', '--json')
    assert.equal(status, 0, `expected exit 0, got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)

    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'passed')

    const gatesCheck = doc.checks.find((c) => c.name === 'gates')
    assert.ok(gatesCheck, 'expected a "gates" check in the result document')
    assert.equal(gatesCheck.status, 'passed')
    assert.deepEqual(gatesCheck.commands, [])
  } finally {
    cleanupFixture(dir)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// 2 — ml_pendente_bloqueia
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: ml_pendente_bloqueia', () => {
  const dir = setupBarrierFixture({
    linkedREQ: true,
    mlStatus: '⬜ Pendente',
    criteriaLines: ['- [x] build passes'],
  })
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1, got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)

    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'blocked')

    const mlsCheck = doc.checks.find((c) => c.name === 'mls_complete')
    assert.ok(mlsCheck, 'expected a "mls_complete" check in the result document')
    assert.equal(mlsCheck.status, 'blocked')
  } finally {
    cleanupFixture(dir)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// 3 — evidencia_ausente_bloqueia
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: evidencia_ausente_bloqueia', () => {
  const dir = setupBarrierFixture({
    linkedREQ: true,
    mlStatus: '✅',
    criteriaLines: ['- [x] build passes', '- [ ] tests pass'], // ao menos um não marcado
  })
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1, got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)

    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'blocked')

    const evidenceCheck = doc.checks.find((c) => c.name === 'acceptance_evidence')
    assert.ok(evidenceCheck, 'expected an "acceptance_evidence" check in the result document')
    assert.equal(evidenceCheck.status, 'blocked')
  } finally {
    cleanupFixture(dir)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// 4 — ml_sem_bloco_de_criterios_bloqueia (caso anti-vacuidade)
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: ml_sem_bloco_de_criterios_bloqueia', () => {
  const dir = setupBarrierFixture({
    linkedREQ: true,
    mlStatus: '✅',
    omitCriteriaBlock: true, // nenhum bloco "**Critérios de aceite:**" — não pode passar vacuamente
  })
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1 (anti-vacuity), got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)

    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'blocked')

    const evidenceCheck = doc.checks.find((c) => c.name === 'acceptance_evidence')
    assert.ok(evidenceCheck, 'expected an "acceptance_evidence" check in the result document')
    assert.equal(evidenceCheck.status, 'blocked')
  } finally {
    cleanupFixture(dir)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// 5 — gate_falho_bloqueia
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: gate_falho_bloqueia', () => {
  const dir = setupBarrierFixture({
    linkedREQ: true,
    mlStatus: '✅',
    criteriaLines: ['- [x] build passes'],
    gateCommands: ['false'],
  })
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1, got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)

    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'blocked')

    const gatesCheck = doc.checks.find((c) => c.name === 'gates')
    assert.ok(gatesCheck, 'expected a "gates" check in the result document')
    assert.equal(gatesCheck.status, 'blocked')
    assert.ok(gatesCheck.commands.includes('false'), `expected commands to record "false", got ${JSON.stringify(gatesCheck.commands)}`)
  } finally {
    cleanupFixture(dir)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// 6 — validate_falho_bloqueia
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: validate_falho_bloqueia', () => {
  // Wave/ML/gates estão inteiramente verdes; a única falha é de governança
  // (roadmap em wip sem REQ vinculada), que só o check "validate" deve capturar.
  const dir = setupBarrierFixture({
    linkedREQ: false,
    mlStatus: '✅',
    criteriaLines: ['- [x] build passes'],
  })
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '1', '--json')
    assert.equal(status, 1, `expected exit 1, got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)

    const doc = JSON.parse(stdout)
    assert.equal(doc.status, 'blocked')

    const validateCheck = doc.checks.find((c) => c.name === 'validate')
    assert.ok(validateCheck, 'expected a "validate" check in the result document')
    assert.equal(validateCheck.status, 'blocked')

    // Os demais checks devem permanecer verdes — prova que o fixture isola a falha.
    for (const c of doc.checks) {
      if (c.name !== 'validate') {
        assert.equal(c.status, 'passed', `expected only "validate" to be blocked, but ${c.name} is ${c.status}`)
      }
    }
  } finally {
    cleanupFixture(dir)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// 7 — roadmap_ou_wave_inexistente_e_erro_de_uso
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: roadmap_ou_wave_inexistente_e_erro_de_uso', () => {
  // Sub-caso 1 — wave inexistente
  const dir = setupBarrierFixture({
    linkedREQ: true,
    mlStatus: '✅',
    criteriaLines: ['- [x] build passes'],
  })
  try {
    const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '99', '--json')
    assert.equal(status, 2, `expected exit 2 (usage error) for nonexistent wave, got ${status}`)
    assert.ok(
      !stdout.includes('"status":"blocked"') && !stdout.includes('"status": "blocked"'),
      `a usage error must never emit a status=blocked result document, got: ${stdout}`
    )
    assert.ok(
      stderr.toLowerCase().includes('wave') || stderr.includes('99'),
      `usage error must explicitly name the unresolved wave, got stderr: ${stderr}`
    )
  } finally {
    cleanupFixture(dir)
  }

  // Sub-caso 2 — roadmap inexistente
  const emptyDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-barrier-empty-'))
  fs.mkdirSync(path.join(emptyDir, 'docs/roadmaps/wip'), { recursive: true })
  try {
    const { stdout, stderr, status } = runBarrierCLI(emptyDir, 'ROADMAP-does-not-exist', '--wave', '1', '--json')
    assert.equal(status, 2, `expected exit 2 (usage error) for nonexistent roadmap, got ${status}`)
    assert.ok(
      !stdout.includes('"status":"blocked"') && !stdout.includes('"status": "blocked"'),
      `a usage error must never emit a status=blocked result document, got: ${stdout}`
    )
    assert.ok(
      stderr.toLowerCase().includes('roadmap') || stderr.toLowerCase().includes('does-not-exist'),
      `usage error must explicitly name the unresolved roadmap, got stderr: ${stderr}`
    )
  } finally {
    cleanupFixture(emptyDir)
  }
})

// ────────────────────────────────────────────────────────────────────────────
// 8 — json_deterministico
// ────────────────────────────────────────────────────────────────────────────

test('barrier contract: json_deterministico', () => {
  const dir = setupBarrierFixture({
    linkedREQ: true,
    mlStatus: '✅',
    criteriaLines: ['- [x] build passes'],
    gateCommands: ['true'],
  })
  const wantOrder = ['mls_complete', 'acceptance_evidence', 'gates', 'validate']

  try {
    for (let run = 0; run < 2; run++) {
      const { stdout, stderr, status } = runBarrierCLI(dir, 'ROADMAP-barrier-fixture', '--wave', '1', '--json')
      assert.equal(status, 0, `run ${run}: expected exit 0, got ${status}\nstdout: ${stdout}\nstderr: ${stderr}`)

      const doc = JSON.parse(stdout)
      assert.equal(doc.checks.length, wantOrder.length, `run ${run}: expected ${wantOrder.length} checks`)

      wantOrder.forEach((name, i) => {
        const check = doc.checks[i]
        assert.equal(check.name, name, `run ${run}: expected checks[${i}].name=${name}, got ${check.name}`)
        assert.ok(Array.isArray(check.evidence), `run ${run}: checks[${i}].evidence must always be an array`)
        assert.ok(Array.isArray(check.failures), `run ${run}: checks[${i}].failures must always be an array`)
        if (name === 'gates') {
          assert.ok(Array.isArray(check.commands), `run ${run}: "commands" must be present on the gates check`)
        } else {
          assert.ok(!('commands' in check), `run ${run}: "commands" must be present only on the gates check, found on ${name}`)
        }
      })

      assert.ok(Array.isArray(doc.failures), `run ${run}: top-level failures must always be an array`)
    }
  } finally {
    cleanupFixture(dir)
  }
})
