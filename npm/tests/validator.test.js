'use strict'
const assert = require('assert')
const fs = require('fs')
const path = require('path')
const os = require('os')
const config = require('../src/config')

// Reset config singleton antes de cada teste que muda cwd
const validator = require('../src/validator')

let passed = 0, failed = 0, skipped = 0
function test(name, fn) {
  try { fn(); console.log('✓', name); passed++ }
  catch (e) { console.error('✗', name, e.message); failed++ }
}
async function testAsync(name, fn) {
  try { await fn(); console.log('✓', name); passed++ }
  catch (e) { console.error('✗', name, e.message); failed++ }
}
// testSkip registra testes esperando falha (defeito P2 exposto pelo ML-1A).
// Substitui xfail/skip de frameworks externos — sem nova dependência.
// Semântica strict: se o teste PASSAR, emite erro e incrementa failed,
// forçando a reativação após a Wave 2 convergir os templates.
function testSkip(name, fn) {
  try {
    fn()
    // Se chegou aqui o teste passou — defeito foi corrigido mas marcador não foi removido
    console.error('✗ [XPASS inesperado — remover testSkip após ML-2A]', name)
    failed++
  } catch (_e) {
    // Falha esperada — defeito ainda presente
    console.log('↷ [xfail esperado]', name)
    skipped++
  }
}

// walkDirMd
test('walkDirMd finds .md in subdirectories', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-'))
  fs.mkdirSync(path.join(tmp, 'done'))
  fs.writeFileSync(path.join(tmp, 'done', 'ADR-001.md'), '---\nstatus: Accepted\n---\n# ADR\n')
  fs.mkdirSync(path.join(tmp, 'wip'))
  fs.writeFileSync(path.join(tmp, 'wip', 'ADR-002.md'), '---\nstatus: Draft\n---\n# ADR\n')
  const results = validator.walkDirMd(tmp)
  assert(results.includes('ADR-001.md'), 'should find ADR-001.md in done/')
  assert(results.includes('ADR-002.md'), 'should find ADR-002.md in wip/')
  fs.rmSync(tmp, { recursive: true })
})

test('walkDirMd returns empty for non-existent dir', () => {
  const results = validator.walkDirMd('/tmp/tw-nonexistent-xyz-123')
  assert(Array.isArray(results))
  assert.strictEqual(results.length, 0)
})

// extractRefPath
test('extractRefPath extracts .md path', () => {
  const content = 'REQ: docs/req/foo.md\n'
  const result = validator.extractRefPath(content, 'REQ')
  assert.strictEqual(result, 'docs/req/foo.md')
})

test('extractRefPath returns null for em-dash', () => {
  const content = 'REQ: —\n'
  const result = validator.extractRefPath(content, 'REQ')
  assert.strictEqual(result, null)
})

test('extractRefPath returns null for hyphen placeholder', () => {
  const content = 'ADR: -\n'
  const result = validator.extractRefPath(content, 'ADR')
  assert.strictEqual(result, null)
})

test('extractRefPath returns null for non-.md value', () => {
  const content = 'Roadmap: somevalue\n'
  const result = validator.extractRefPath(content, 'Roadmap')
  assert.strictEqual(result, null)
})

test('extractRefPath returns null for empty field', () => {
  const content = 'REQ: \n'
  const result = validator.extractRefPath(content, 'REQ')
  assert.strictEqual(result, null)
})

// ML-1B — backtick não pode tornar a referência invisível (ADR-2026-08-02)
test('extractRefPath strips backticks and resolves the path', () => {
  const content = 'ADR: `docs/adr/X.md`\n'
  const result = validator.extractRefPath(content, 'ADR')
  assert.strictEqual(result, 'docs/adr/X.md')
})

test('extractRefPath strips backticks with trailing prose', () => {
  const content = 'ADR: `docs/adr/X.md` (P1–P4; esta REQ é derivada)\n'
  const result = validator.extractRefPath(content, 'ADR')
  assert.strictEqual(result, 'docs/adr/X.md')
})

// Tabela do AC5 — mede a saída literal para as 8 entradas do ADR-2026-08-02.
// Não força igualdade entre CLIs; apenas documenta o comportamento do Node.
test('extractRefPath AC5 table — measured outputs (Node)', () => {
  const cases = [
    ['ADR: `docs/adr/X.md`', 'docs/adr/X.md'],
    ['ADR: "docs/adr/X.md"', 'docs/adr/X.md'],
    ["ADR: 'docs/adr/X.md'", 'docs/adr/X.md'],
    ['ADR: docs/adr/X.md', 'docs/adr/X.md'],
    ['ADR: `docs/adr/X.md` (P1–P4; prosa após)', 'docs/adr/X.md'],
    // caso 6 — delimitador não pareado. O regex de uma ocorrência por ponta remove
    // o `"` inicial e o `'` final independentemente (não exige par casado), então
    // o Node RESOLVE este caso — divergência esperada frente a mecanismos de par casado.
    ['ADR: "docs/adr/X.md\'', 'docs/adr/X.md'],
    ['ADR:', null],
    ['ADR: —', null],
  ]
  for (const [line, expected] of cases) {
    const result = validator.extractRefPath(line + '\n', 'ADR')
    console.log(`  [AC5/Node] ${JSON.stringify(line)} -> ${JSON.stringify(result)}`)
    assert.strictEqual(result, expected,
      `AC5 caso ${JSON.stringify(line)}: esperado ${JSON.stringify(expected)}, obtido ${JSON.stringify(result)}`)
  }
})

// validateFolderStatusCoherence
test('validateFolderStatusCoherence returns array', () => {
  const result = validator.validateFolderStatusCoherence()
  assert(Array.isArray(result))
})

test('validateFolderStatusCoherence detects mismatch', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-'))
  const wipDir = path.join(tmp, 'roadmaps', 'wip')
  fs.mkdirSync(wipDir, { recursive: true })
  // Arquivo em wip/ mas status: Done no frontmatter
  fs.writeFileSync(path.join(wipDir, 'ROADMAP-test.md'), '---\nstatus: Done\ndate: 2026-01-01\n---\n# Test\n')
  // trackfw.yaml apontando para tmp
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${path.join(tmp, 'roadmaps')}\n`)

  const origCwd = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateFolderStatusCoherence()
    assert(result.some(w => w.includes('ROADMAP-test.md') && w.includes('Done')), `Expected mismatch warning, got: ${JSON.stringify(result)}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// validateFilenameUniqueness
test('validateFilenameUniqueness no-op when no duplicates', () => {
  const result = validator.validateFilenameUniqueness()
  assert(Array.isArray(result))
})

test('validateFilenameUniqueness detects duplicate', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-'))
  const roadmapDir = path.join(tmp, 'roadmaps')
  for (const state of ['wip', 'backlog', 'done']) {
    fs.mkdirSync(path.join(roadmapDir, state), { recursive: true })
  }
  // Mesmo nome em wip e backlog
  const fname = 'ROADMAP-2026-06-13-duplicate.md'
  fs.writeFileSync(path.join(roadmapDir, 'wip', fname), '# wip\n')
  fs.writeFileSync(path.join(roadmapDir, 'backlog', fname), '# backlog\n')
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${roadmapDir}\n`)

  const origCwd = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateFilenameUniqueness()
    assert(result.some(v => v.includes(fname) && v.includes('wip') && v.includes('backlog')), `Expected uniqueness violation, got: ${JSON.stringify(result)}`)
  } finally {
    process.chdir(origCwd)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// validateRefTargetsExist
test('validateRefTargetsExist returns array', () => {
  const result = validator.validateRefTargetsExist()
  assert(Array.isArray(result))
})

test('validateRefTargetsExist rejects generated basename references', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-ref-'))
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/req/REQ-001.md'), '# REQ\nRoadmap: ROADMAP-001.md\n')
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/ROADMAP-001.md'), '# Roadmap\nREQ: REQ-001.md\n')
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'req_dir: docs/req\nroadmap_dir: docs/roadmaps\n')

  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const warnings = validator.validateRefTargetsExist()
    assert(warnings.some(w => w.includes('REQ-001.md')), `Expected REQ basename warning, got ${JSON.stringify(warnings)}`)
    assert(warnings.some(w => w.includes('ROADMAP-001.md')), `Expected roadmap basename warning, got ${JSON.stringify(warnings)}`)
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// ML-2B: field mapping + severity per rule

test('field mapping: req_id satisfies wip_has_req', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'link_fields:\n  req:\n    - req_id\n')
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-001.md'),
    '---\nstatus: WIP\nreq_id: docs/req/REQ-001.md\n---\n## Acceptance Criteria\n- [ ] done\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateWIPHasREQ()
    assert(!result.some(v => v.includes('no linked REQ')),
      'req_id marker should satisfy wip_has_req: ' + JSON.stringify(result))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('severity off: adr_orphan suppressed', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'rules:\n  adr_orphan: off\n')
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/adr/ADR-001.md'),
    '---\nstatus: Accepted\n---\n# ADR-001\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const violations = []
    const warnings = []
    validator.applyRule('adr_orphan', validator.validateADRsAreReferenced(), violations, warnings)
    assert(!violations.some(v => v.includes('not referenced')),
      'adr_orphan: off should suppress violations')
    assert(!warnings.some(w => w.includes('not referenced')),
      'adr_orphan: off should suppress warnings too')
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('severity warning: wip_has_req appears in warnings not violations', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'rules:\n  wip_has_req: warning\n')
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-001.md'),
    '---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const violations = []
    const warnings = []
    validator.applyRule('wip_has_req', validator.validateWIPHasREQ(), violations, warnings)
    assert(!violations.some(v => v.includes('no linked REQ')),
      'wip_has_req: warning should not be in violations')
    assert(warnings.some(w => w.includes('no linked REQ')),
      'wip_has_req: warning should appear in warnings')
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

test('acceptance_markers custom: custom marker satisfies check', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-vm-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
    'acceptance_markers:\n  - "## Done When"\n  - "## Critérios"\n')
  fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
  fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip/RM-001.md'),
    '---\nstatus: WIP\nREQ: docs/req/REQ-001.md\n---\n## Done When\n- [ ] done\n')
  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const result = validator.validateWIPHasAcceptanceCriteria()
    assert(!result.some(v => v.includes('no acceptance criteria')),
      'custom marker ## Done When should satisfy acceptance criteria check')
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true })
  }
})

// ML-1B — Validação de adr_dirs com ~/
test('adr_dirs com ~/ no validador resolve diretório no home do usuário', () => {
  const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-home-'))
  const testSubdir = '.trackfw-test-adrs-' + Date.now()
  const fullHomeSubdir = path.join(fakeHome, testSubdir)
  const oldHome = process.env.HOME
  process.env.HOME = fakeHome
  fs.mkdirSync(fullHomeSubdir, { recursive: true })
  fs.writeFileSync(path.join(fullHomeSubdir, 'ADR-GLOBAL-001.md'), '---\nstatus: Accepted\n---\n# Global ADR\n')

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-tilde-val-'))
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `adr_dirs:\n  - "~/${testSubdir}"\n`)

  const origDir = process.cwd()
  process.chdir(tmp)
  config.reset()
  try {
    const found = validator.findAdrFile('ADR-GLOBAL-001.md')
    assert.strictEqual(found, path.join(fullHomeSubdir, 'ADR-GLOBAL-001.md'))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
    fs.rmSync(fullHomeSubdir, { recursive: true, force: true })
    fs.rmSync(fakeHome, { recursive: true, force: true })
    if (oldHome === undefined) delete process.env.HOME
    else process.env.HOME = oldHome
  }
})

// ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A —
// regra credential_guard_hook_resolvable
function guardEntryClaudeSettings(scriptCmd) {
  return JSON.stringify({
    hooks: {
      PreToolUse: [
        { matcher: 'Bash', hooks: [{ command: scriptCmd, type: 'command' }] },
      ],
    },
  })
}

test('credential_guard_hook_resolvable: dispara quando o script referenciado não existe', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('does not exist') && m.includes('.claude/settings.json')),
      'esperava violation de script ausente: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: dispara quando o script não é executável', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))
  fs.mkdirSync(path.join(tmp, 'scripts'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), '#!/bin/sh\nexit 0\n', { mode: 0o644 })
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('not executable')),
      'esperava violation de script não executável: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: não dispara sem entrada de guard (estado legítimo)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'), JSON.stringify({
    hooks: {
      PostToolUse: [{ matcher: 'AskUserQuestion', hooks: [{ command: 'scripts/trackfw-attention-cleanup.sh', type: 'command' }] }],
      PreToolUse: [{ matcher: 'AskUserQuestion', hooks: [{ command: 'scripts/trackfw-attention-signal.sh', type: 'command' }] }],
    },
  }))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0, 'sem entrada de guard não deve haver violations: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: não dispara para formato de prefixo desconhecido', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('$SOME_OTHER_VAR/scripts/trackfw-credential-guard.sh'))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0, 'formato desconhecido não deve violar: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: resolve a forma do Codex (aspas literais)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.codex'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.codex', 'hooks.json'), JSON.stringify({
    hooks: {
      PreToolUse: [{ matcher: '.*', hooks: [{ command: '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"', type: 'command' }] }],
    },
  }))
  process.chdir(tmp)
  config.reset()
  try {
    let msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('does not exist') && m.includes('.codex/hooks.json')),
      'esperava violation resolvendo a forma do Codex: ' + JSON.stringify(msgs))

    fs.mkdirSync(path.join(tmp, 'scripts'), { recursive: true })
    fs.writeFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), '#!/bin/sh\nexit 0\n', { mode: 0o755 })
    msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0, 'com script existente e executável não deve haver violations: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: resolve caminho relativo puro (Cursor)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.cursor'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.cursor', 'hooks.json'), JSON.stringify({
    version: 1,
    hooks: { beforeShellExecution: [{ command: 'scripts/trackfw-credential-guard.sh' }] },
  }))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('does not exist') && m.includes('.cursor/hooks.json')),
      'esperava violation resolvendo caminho relativo puro: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: configurável via rules (warning/off), default error', () => {
  const build = () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-'))
    fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
      guardEntryClaudeSettings('$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh'))
    return tmp
  }
  const origDir = process.cwd()

  // default error
  let tmp = build()
  process.chdir(tmp)
  config.reset()
  try {
    const violations = []
    const warnings = []
    validator.applyRule('credential_guard_hook_resolvable', validator.validateCredentialGuardHookResolvable(), violations, warnings)
    assert(violations.some(v => v.includes('trackfw-credential-guard.sh')), 'default deve ser error: ' + JSON.stringify(violations))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }

  // warning
  tmp = build()
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'rules:\n  credential_guard_hook_resolvable: warning\n')
  process.chdir(tmp)
  config.reset()
  try {
    const violations = []
    const warnings = []
    validator.applyRule('credential_guard_hook_resolvable', validator.validateCredentialGuardHookResolvable(), violations, warnings)
    assert(!violations.some(v => v.includes('trackfw-credential-guard.sh')), 'rules:warning não deve gerar violation: ' + JSON.stringify(violations))
    assert(warnings.some(w => w.includes('trackfw-credential-guard.sh')), 'rules:warning deve gerar warning: ' + JSON.stringify(warnings))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }

  // off
  tmp = build()
  fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'rules:\n  credential_guard_hook_resolvable: off\n')
  process.chdir(tmp)
  config.reset()
  try {
    const violations = []
    const warnings = []
    validator.applyRule('credential_guard_hook_resolvable', validator.validateCredentialGuardHookResolvable(), violations, warnings)
    assert.strictEqual(violations.length, 0, 'rules:off não deve gerar violation: ' + JSON.stringify(violations))
    assert.strictEqual(warnings.length, 0, 'rules:off não deve gerar warning: ' + JSON.stringify(warnings))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ROADMAP-2026-08-21 ML-1B — requiresVarOrShellPrefix: forma relativa antiga em Claude acusada,
// Cursor com relativo limpo (AC1 + AC3 não-vácuo).

test('credential_guard_hook_resolvable: dispara forma relativa antiga em Claude (AC1, script presente)', () => {
  // AC1: Claude settings com "scripts/trackfw-credential-guard.sh" (sem prefixo), script
  // presente e executável. A violação vem da forma do comando, não da ausência do script.
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-legrel-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('scripts/trackfw-credential-guard.sh'))
  fs.mkdirSync(path.join(tmp, 'scripts'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), '#!/bin/sh\nexit 0\n', { mode: 0o755 })
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('bare relative path') && m.includes('.claude/settings.json')),
      'AC1: esperava violation de forma relativa antiga em Claude: ' + JSON.stringify(msgs))
    assert(msgs.some(m => m.includes('trackfw update')),
      'AC4: mensagem deve nomear trackfw update: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: Cursor com relativo puro e script presente continua limpo (AC3, não-vácuo)', () => {
  // AC3 não-vácuo: Cursor com "scripts/trackfw-credential-guard.sh" e script PRESENTE e
  // executável não deve violar — requiresVarOrShellPrefix=false para Cursor por construção.
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-cursor-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.cursor'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.cursor', 'hooks.json'), JSON.stringify({
    version: 1,
    hooks: { beforeShellExecution: [{ command: 'scripts/trackfw-credential-guard.sh' }] },
  }))
  fs.mkdirSync(path.join(tmp, 'scripts'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), '#!/bin/sh\nexit 0\n', { mode: 0o755 })
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0,
      'AC3: Cursor com relativo deve estar limpo (falso-positivo eliminado por construção): ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ADR-2026-08-22 ML-1A — classifyHookAnchorage + stripOuterQuotesForClassify: testes unitários
// e testes de integração com validateCredentialGuardHookResolvable.

test('classifyHookAnchorage: classe 1 — formas ancoradas', () => {
  const cases = [
    { raw: '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '/opt/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '/absolute/path/guard.sh', wasQuoted: false },
    // ~/… sem aspas: tilde expande para $HOME em qualquer shell POSIX — ancorado.
    { raw: '~/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '~/.trackfw/scripts/trackfw-credential-guard.sh', wasQuoted: false },
  ]
  for (const { raw, wasQuoted } of cases) {
    assert.strictEqual(validator.classifyHookAnchorage(raw, wasQuoted), validator.HOOK_ANCHORAGE_CLASS_ANCHORED,
      `esperava classe 1 para: ${raw} (wasQuoted=${wasQuoted})`)
  }
})

test('classifyHookAnchorage: classe 2 — dependente do cwd', () => {
  const cases = [
    { raw: '$PWD/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '${PWD}/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: './scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '../scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: 'scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: 'sh scripts/trackfw-credential-guard.sh', wasQuoted: false },
    // "~/…" com aspas: tilde NÃO expande dentro de aspas duplas — classe 2.
    { raw: '~/scripts/trackfw-credential-guard.sh', wasQuoted: true },
    { raw: '~/.trackfw/scripts/trackfw-credential-guard.sh', wasQuoted: true },
  ]
  for (const { raw, wasQuoted } of cases) {
    assert.strictEqual(validator.classifyHookAnchorage(raw, wasQuoted), validator.HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT,
      `esperava classe 2 para: ${raw} (wasQuoted=${wasQuoted})`)
  }
})

test('classifyHookAnchorage: classe 3 — indecidível', () => {
  const cases = [
    { raw: '$SOME_OTHER_VAR/scripts/trackfw-credential-guard.sh', wasQuoted: false },
    { raw: '$MY_CUSTOM_DIR/guard.sh', wasQuoted: false },
    { raw: '$UNDEFINED/trackfw-credential-guard.sh', wasQuoted: false },
  ]
  for (const { raw, wasQuoted } of cases) {
    assert.strictEqual(validator.classifyHookAnchorage(raw, wasQuoted), validator.HOOK_ANCHORAGE_CLASS_UNDECIDABLE,
      `esperava classe 3 para: ${raw} (wasQuoted=${wasQuoted})`)
  }
})

test('stripOuterQuotesForClassify: remove aspas duplas envolventes', () => {
  const cases = [
    { raw: '"$PWD/scripts/guard.sh"', want: '$PWD/scripts/guard.sh' },
    { raw: '"$(git rev-parse --show-toplevel)/scripts/guard.sh"', want: '$(git rev-parse --show-toplevel)/scripts/guard.sh' },
    { raw: '$CLAUDE_PROJECT_DIR/scripts/guard.sh', want: '$CLAUDE_PROJECT_DIR/scripts/guard.sh' },
    { raw: 'scripts/guard.sh', want: 'scripts/guard.sh' },
    { raw: '"', want: '"' },
    { raw: '""', want: '' },
    { raw: '"abc', want: '"abc' },
  ]
  for (const { raw, want } of cases) {
    assert.strictEqual(validator.stripOuterQuotesForClassify(raw), want,
      `stripOuterQuotesForClassify(${JSON.stringify(raw)})`)
  }
})

test('credential_guard_hook_resolvable: $PWD/… acusado em Claude (AC2)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-pwd-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('$PWD/scripts/trackfw-credential-guard.sh'))
  fs.mkdirSync(path.join(tmp, 'scripts'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), '#!/bin/sh\nexit 0\n', { mode: 0o755 })
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('$PWD path') && m.includes('.claude/settings.json')),
      'AC2: esperava violation de $PWD em Claude: ' + JSON.stringify(msgs))
    assert(msgs.some(m => m.includes('current working directory')),
      'AC2: mensagem deve explicar que $PWD não ancora: ' + JSON.stringify(msgs))
    assert(msgs.some(m => m.includes('trackfw update')),
      'AC2: mensagem deve citar trackfw update: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: "$PWD/…" entre aspas também acusado (D.3)', () => {
  // Achado D.3: valor entre aspas deve ser acusado após strip de aspas externas.
  // O JSON field value é: "$PWD/scripts/..." (com as aspas fazendo parte do valor JSON).
  // guardEntryClaudeSettings insere o scriptCmd diretamente no template JSON.
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-pwdq-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  // Para que o valor JSON do campo "command" seja "$PWD/scripts/..." (com aspas literais),
  // passamos a string JS com os caracteres de aspas duplas — guardEntryClaudeSettings usa
  // JSON.stringify, que irá escapar as aspas corretamente para \"...\".
  // Passamos '"$PWD/..."' (JS string com aspas duplas no conteúdo).
  const cmdValue = '"$PWD/scripts/trackfw-credential-guard.sh"'
  const content = guardEntryClaudeSettings(cmdValue)
  // Verifica que o JSON é válido (fixture malformado → falso negativo silencioso).
  const parseCheck = JSON.parse(content)
  assert(parseCheck, 'fixture JSON deve ser válido')
  // Verifica que o valor do campo "command" foi serializado com aspas (achado D.3).
  const cmdInJSON = parseCheck.hooks.PreToolUse[0].hooks[0].command
  assert(cmdInJSON.startsWith('"') && cmdInJSON.endsWith('"'),
    'valor command deve ter aspas duplas como primeiro e último char: ' + JSON.stringify(cmdInJSON))
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'), content)
  fs.mkdirSync(path.join(tmp, 'scripts'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), '#!/bin/sh\nexit 0\n', { mode: 0o755 })
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('$PWD path') && m.includes('.claude/settings.json')),
      'D.3: esperava violation de $PWD entre aspas em Claude: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: caminho absoluto silencioso (classe 1, falso-positivo dominante)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-abs-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('/opt/scripts/trackfw-credential-guard.sh'))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0,
      'classe 1 (absoluto) deve ser silenciosa: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: $OUTRA_VAR/… silenciosa (classe 3)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-cls3-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('$MY_CUSTOM_DIR/scripts/trackfw-credential-guard.sh'))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0,
      'classe 3 ($OUTRA_VAR) deve ser silenciosa: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: forma Codex com aspas continua silenciosa (classe 1)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-codex-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.codex'), { recursive: true })
  // O valor emitido pelo Codex já inclui aspas literais como parte do JSON value.
  const content = JSON.stringify({
    hooks: {
      PreToolUse: [{
        matcher: '.*',
        hooks: [{ command: '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"', type: 'command' }],
      }],
    },
  }, null, 2)
  fs.writeFileSync(path.join(tmp, '.codex', 'hooks.json'), content)
  fs.mkdirSync(path.join(tmp, 'scripts'), { recursive: true })
  fs.writeFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), '#!/bin/sh\nexit 0\n', { mode: 0o755 })
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0,
      'forma Codex (classe 1 com aspas) deve ser silenciosa: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: $PWD/… acusado em Codex (AC2)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-pwdcodex-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.codex'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.codex', 'hooks.json'), JSON.stringify({
    hooks: {
      PreToolUse: [{ matcher: '.*', hooks: [{ command: '$PWD/scripts/trackfw-credential-guard.sh', type: 'command' }] }],
    },
  }))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('$PWD path') && m.includes('.codex/hooks.json')),
      'AC2 Codex: esperava violation de $PWD: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: $PWD/… acusado em Gemini (AC2)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-pwdgemini-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.gemini'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.gemini', 'settings.json'), JSON.stringify({
    hooks: {
      PreToolUse: [{ matcher: '.*', hooks: [{ command: '$PWD/scripts/trackfw-credential-guard.sh', type: 'command' }] }],
    },
  }))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('$PWD path') && m.includes('.gemini/settings.json')),
      'AC2 Gemini: esperava violation de $PWD: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ML-4A (ROADMAP-2026-08-22) — ~/…, ${PWD}/…, mensagem certa para sh -c "$PWD/…"

test('hookValueWasQuoted: detecta aspas externas', () => {
  assert.strictEqual(validator.hookValueWasQuoted('"$PWD/scripts/guard.sh"'), true)
  assert.strictEqual(validator.hookValueWasQuoted('"~/scripts/guard.sh"'), true)
  assert.strictEqual(validator.hookValueWasQuoted('~/scripts/guard.sh'), false)
  assert.strictEqual(validator.hookValueWasQuoted('$PWD/scripts/guard.sh'), false)
  assert.strictEqual(validator.hookValueWasQuoted('"'), false)
  assert.strictEqual(validator.hookValueWasQuoted('""'), true)
})

test('cwdDependentReason: $PWD em qualquer posição usa mensagem do $PWD', () => {
  const pwdCases = [
    '$PWD/scripts/guard.sh',
    '${PWD}/scripts/guard.sh',
    'sh -c "$PWD/scripts/guard.sh"',
    'env FOO=x $PWD/scripts/guard.sh',
  ]
  for (const raw of pwdCases) {
    const reason = validator.cwdDependentReason(raw)
    assert(reason.includes('$PWD path'), `esperava '$PWD path' para: ${raw}, obteve: ${reason}`)
  }
  const bareCases = ['./scripts/guard.sh', '../scripts/guard.sh', 'scripts/guard.sh', '~/scripts/guard.sh']
  for (const raw of bareCases) {
    const reason = validator.cwdDependentReason(raw)
    assert(reason.includes('bare relative path'), `esperava 'bare relative path' para: ${raw}, obteve: ${reason}`)
  }
})

test('credential_guard_hook_resolvable: ~/… sem aspas é silencioso (ML-4A — classe 1)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-tilde-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('~/scripts/trackfw-credential-guard.sh'))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert.strictEqual(msgs.length, 0,
      'ML-4A: ~/… sem aspas (classe 1) deve ser silencioso: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: "~/…" aspeado é acusado (ML-4A — classe 2)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-tildequot-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  // Valor JSON do campo command: "~/scripts/..." (com aspas literais no valor)
  const cmdValue = '"~/scripts/trackfw-credential-guard.sh"'
  const content = guardEntryClaudeSettings(cmdValue)
  const parseCheck = JSON.parse(content)
  assert(parseCheck, 'fixture JSON deve ser válido')
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'), content)
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('bare relative path')),
      'ML-4A: "~/…" aspeado (classe 2) deve ser acusado com bare relative path: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: ${PWD}/… é acusado (ML-4A — classe 2)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-pwdbraced-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'),
    guardEntryClaudeSettings('${PWD}/scripts/trackfw-credential-guard.sh'))
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('$PWD path')),
      'ML-4A: ${PWD}/… deve ser acusado com mensagem do $PWD: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('credential_guard_hook_resolvable: sh -c "$PWD/…" usa mensagem do $PWD (ML-4A)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-cg-shcpwd-'))
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  const cmdValue = 'sh -c "$PWD/scripts/trackfw-credential-guard.sh"'
  const content = guardEntryClaudeSettings(cmdValue)
  const parseCheck = JSON.parse(content)
  assert(parseCheck, 'fixture JSON deve ser válido')
  fs.writeFileSync(path.join(tmp, '.claude', 'settings.json'), content)
  process.chdir(tmp)
  config.reset()
  try {
    const msgs = validator.validateCredentialGuardHookResolvable()
    assert(msgs.some(m => m.includes('$PWD path')),
      'ML-4A: sh -c "$PWD/…" deve usar mensagem do $PWD: ' + JSON.stringify(msgs))
    assert(!msgs.some(m => m.includes('bare relative path')),
      'ML-4A: sh -c "$PWD/…" não deve dizer bare relative path: ' + JSON.stringify(msgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas, ML-3A —
// thirdparty_artifact_has_provenance (ADR-2026-08-15 D2). Port of
// internal/validator/validator_thirdparty_provenance_test.go — same fixtures, same assertions.
;(() => {
  const crypto = require('crypto')

  function sha256Hex(buf) {
    return crypto.createHash('sha256').update(buf).digest('hex')
  }

  function writeJSON(filePath, value) {
    fs.mkdirSync(path.dirname(filePath), { recursive: true })
    fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`)
  }

  function writeManifest(root, destination, origin) {
    const claim = { target: 'claude', surface: 'code', scope: 'project', kind: 'skills', item: 'thirdparty-example' }
    if (origin) claim.origin = origin
    writeJSON(path.join(root, '.trackfw', 'integrations-manifest.json'), {
      schema_version: 1,
      artifacts: {
        [destination]: {
          destination,
          sha256: 'irrelevant-for-this-rule',
          catalog_version: 'thirdparty:abcdef123456',
          claims: [claim],
        },
      },
    })
  }

  // Keyed by destination MADE RELATIVE TO root — provenance is keyed by the
  // project-root-relative path, never by the manifest's absolute
  // destination (verified empirically against the real install command;
  // see the Go sibling test's comment for the full explanation).
  //
  // checksum is the D6 raw-bytes approval anchor (checksum_sha256);
  // installedSHA256 is the D2-bis field branch (ii) actually checks against
  // the installed file's own hash. Independent parameters, never derived
  // from one another — mirrors production, where one is written by the
  // external approver and the other by the install command.
  function writeProvenance(root, destination, checksum, installedSHA256) {
    const relDest = path.relative(root, destination)
    writeJSON(path.join(root, '.trackfw', 'thirdparty-provenance.json'), {
      schema_version: 2,
      entries: {
        [relDest]: {
          url: 'https://example.com/skill.md',
          checksum_sha256: checksum,
          installed_sha256: installedSHA256,
          installed_at: '2026-08-15T00:00:00Z',
          approved_by: 'hades-tf',
          review_reference: 'docs/seguranca/example.md',
          scope: 'project',
          marker_override: false,
        },
      },
    })
  }

  function writeQuarantine(root, rawBuf) {
    const checksum = sha256Hex(rawBuf)
    writeJSON(path.join(root, '.trackfw', 'thirdparty-quarantine', `${checksum}.json`), {
      schema_version: 1,
      url: 'https://example.com/skill.md',
      checksum_sha256: checksum,
      fetched_at: '2026-08-15T00:00:00Z',
      content_base64: rawBuf.toString('base64'),
      marker_check: { result: 'pass', matched_markers: [] },
      kind: 'skill',
      requested_targets: ['claude'],
    })
    return checksum
  }

  function withTmpCwd(fn) {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-tp-'))
    // Resolve symlinks (macOS: os.tmpdir() returns "/var/folders/..." but
    // process.cwd() after chdir returns the physical "/private/var/folders/...").
    // Passing the resolved root to fn keeps destination paths built by the
    // caller consistent with what process.cwd() reports inside the rule.
    const resolved = fs.realpathSync(tmp)
    const origDir = process.cwd()
    process.chdir(resolved)
    config.reset()
    try {
      fn(resolved)
    } finally {
      process.chdir(origDir)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  }

  test('thirdparty_artifact_has_provenance: sem manifest -> sem violations', () => {
    withTmpCwd(() => {
      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 0, JSON.stringify(msgs))
    })
  })

  test('thirdparty_artifact_has_provenance: claim de catálogo (origin ausente) nunca é sinalizado', () => {
    withTmpCwd(root => {
      const destination = path.join(root, 'skill.md')
      fs.writeFileSync(destination, 'catalog content\n')
      writeManifest(root, destination, undefined)
      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 0, JSON.stringify(msgs))
    })
  })

  test('thirdparty_artifact_has_provenance: manifest legado sem campo origin lê como catálogo (retrocompat)', () => {
    withTmpCwd(root => {
      const destination = path.join(root, 'agent.md')
      fs.writeFileSync(destination, 'legacy agent content\n')
      const manifestPath = path.join(root, '.trackfw', 'integrations-manifest.json')
      fs.mkdirSync(path.dirname(manifestPath), { recursive: true })
      fs.writeFileSync(manifestPath, `{
  "schema_version": 1,
  "artifacts": {
    ${JSON.stringify(destination)}: {
      "destination": ${JSON.stringify(destination)},
      "sha256": "irrelevant",
      "catalog_version": "v1",
      "claims": [
        {"target": "claude", "surface": "code", "scope": "project", "kind": "agents", "item": "backend"}
      ]
    }
  }
}
`)
      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 0, JSON.stringify(msgs))
    })
  })

  test('thirdparty_artifact_has_provenance: branch i — sem entrada de proveniência', () => {
    withTmpCwd(root => {
      const destination = path.join(root, 'skills', 'thirdparty', 'example.md')
      fs.mkdirSync(path.dirname(destination), { recursive: true })
      fs.writeFileSync(destination, 'some content\n')
      writeManifest(root, destination, 'thirdparty')
      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 1, JSON.stringify(msgs))
      assert(msgs[0].includes('D2 branch i'), msgs[0])
      assert(msgs[0].includes(destination), msgs[0])
    })
  })

  // Carrega-prova a mesma regressão coberta no Go (ver o comentário no arquivo Go irmão): o
  // conteúdo bruto NÃO é canônico (linha em branco à frente/atrás) — checksum_sha256 é
  // sha256(bruto), instalado_sha256 é sha256(normalize(bruto)). Uma implementação ingênua que
  // compara sha256(arquivo instalado) contra checksum_sha256 FALHARIA aqui. Nenhum registro de
  // quarentena é escrito neste teste — D2-bis existe exatamente para que a ramificação (ii) não
  // dependa mais dele.
  test('thirdparty_artifact_has_provenance: branch ii — install legítimo com conteúdo não-canônico não é falso-positivo', () => {
    withTmpCwd(root => {
      const raw = Buffer.from('\n# hello\n\nsome content\n\n\n', 'utf8')
      const normalized = Buffer.from(`${raw.toString('utf8').trim()}\n`, 'utf8')
      assert(!raw.equals(normalized), 'fixture não testa a divergência bruto/normalizado')
      const checksumOfRaw = sha256Hex(raw)
      const installedSHA256 = sha256Hex(normalized)
      assert.notStrictEqual(checksumOfRaw, installedSHA256, 'fixture deve manter checksum_sha256 e installed_sha256 distintos')

      const destination = path.join(root, 'skills', 'thirdparty', 'example.md')
      fs.mkdirSync(path.dirname(destination), { recursive: true })
      fs.writeFileSync(destination, normalized)
      writeManifest(root, destination, 'thirdparty')
      writeProvenance(root, destination, checksumOfRaw, installedSHA256)

      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 0, JSON.stringify(msgs))
    })
  })

  test('thirdparty_artifact_has_provenance: branch ii — adulteração pós-aprovação é detectada', () => {
    withTmpCwd(root => {
      const raw = Buffer.from('# hello\n\nsome content\n', 'utf8')
      const normalized = Buffer.from(`${raw.toString('utf8').trim()}\n`, 'utf8')
      const destination = path.join(root, 'skills', 'thirdparty', 'example.md')
      fs.mkdirSync(path.dirname(destination), { recursive: true })
      fs.writeFileSync(destination, Buffer.from('# hello\n\nTAMPERED CONTENT\n', 'utf8'))
      writeManifest(root, destination, 'thirdparty')
      writeProvenance(root, destination, sha256Hex(raw), sha256Hex(normalized))

      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 1, JSON.stringify(msgs))
      assert(msgs[0].includes('D2 branch ii'), msgs[0])
    })
  })

  // Âncora de paridade: uma entrada de proveniência escrita só pelo aprovador (D10.2), nunca
  // tendo passado por `install`, tem installed_sha256 AUSENTE (não vazio — ausente). Node ingênuo
  // interpolaria `entry.installed_sha256 === undefined` como o literal "undefined" na mensagem,
  // divergindo de Go/Python (string vazia). Este teste fixa o texto da mensagem para essa
  // regressão não passar despercebida.
  test('thirdparty_artifact_has_provenance: branch ii — installed_sha256 ausente é capturado sem "undefined"', () => {
    withTmpCwd(root => {
      const destination = path.join(root, 'skills', 'thirdparty', 'example.md')
      fs.mkdirSync(path.dirname(destination), { recursive: true })
      fs.writeFileSync(destination, 'content\n')
      writeManifest(root, destination, 'thirdparty')
      // Hand-authored: sem a chave installed_sha256, exatamente como um aprovador (nunca tendo
      // rodado `install`) escreveria.
      const relDest = path.relative(root, destination)
      writeJSON(path.join(root, '.trackfw', 'thirdparty-provenance.json'), {
        schema_version: 2,
        entries: {
          [relDest]: {
            url: 'https://example.com/skill.md',
            checksum_sha256: 'a'.repeat(64),
            installed_at: '2026-08-15T00:00:00Z',
            approved_by: 'hades-tf',
            review_reference: 'docs/seguranca/example.md',
            scope: 'project',
            marker_override: false,
          },
        },
      })

      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 1, JSON.stringify(msgs))
      assert(!msgs[0].includes('undefined'), msgs[0])
      assert(msgs[0].includes('installed_sha256  recorded'), msgs[0])
    })
  })

  // Testes carrega-prova do ML-3B (ADR-2026-08-15 D2-bis): reproduzem o cenário exato que o
  // desenho do ML-3A errava — apagar .trackfw/thirdparty-quarantine/ INTEIRO e confirmar que (a)
  // instalação íntegra não vira violação e (b) adulteração continua detectada. Substituem o teste
  // "quarentena ausente falha fechado (D8f)" do ML-3A: essa premissa (quarentena ausente = erro)
  // era exatamente o footgun que D2-bis remove — não há mais dependência da quarentena na
  // ramificação (ii), então sua ausência não é mais um caso de fail-closed.
  test('thirdparty_artifact_has_provenance: branch ii — apagar quarentena não quebra install íntegro', () => {
    withTmpCwd(root => {
      const raw = Buffer.from('\n# hello\n\nsome content\n\n\n', 'utf8')
      const normalized = Buffer.from(`${raw.toString('utf8').trim()}\n`, 'utf8')
      const checksumOfRaw = writeQuarantine(root, raw) // registro real, como `fetch` escreveria

      const destination = path.join(root, 'skills', 'thirdparty', 'example.md')
      fs.mkdirSync(path.dirname(destination), { recursive: true })
      fs.writeFileSync(destination, normalized)
      writeManifest(root, destination, 'thirdparty')
      writeProvenance(root, destination, checksumOfRaw, sha256Hex(normalized))

      fs.rmSync(path.join(root, '.trackfw', 'thirdparty-quarantine'), { recursive: true, force: true })

      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 0, JSON.stringify(msgs))
    })
  })

  test('thirdparty_artifact_has_provenance: branch ii — apagar quarentena não impede detecção de adulteração', () => {
    withTmpCwd(root => {
      const raw = Buffer.from('# hello\n\nsome content\n', 'utf8')
      const normalized = Buffer.from(`${raw.toString('utf8').trim()}\n`, 'utf8')
      const checksumOfRaw = writeQuarantine(root, raw)

      const destination = path.join(root, 'skills', 'thirdparty', 'example.md')
      fs.mkdirSync(path.dirname(destination), { recursive: true })
      fs.writeFileSync(destination, Buffer.from('# hello\n\nTAMPERED CONTENT\n', 'utf8'))
      writeManifest(root, destination, 'thirdparty')
      writeProvenance(root, destination, checksumOfRaw, sha256Hex(normalized))

      fs.rmSync(path.join(root, '.trackfw', 'thirdparty-quarantine'), { recursive: true, force: true })

      const msgs = validator.validateThirdPartyArtifactHasProvenance()
      assert.strictEqual(msgs.length, 1, JSON.stringify(msgs))
      assert(msgs[0].includes('D2 branch ii'), msgs[0])
    })
  })
})()

// ML-2B — Resiliência CI/CD para adr_dirs inexistentes e isenção de adr_orphan em ADRs externos
;(async () => {
  await testAsync('adr_dirs inexistente com strict_ci_paths false (default) gera warning', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-nonexistent-warning-'))
    const nonexistent = path.join(tmp, 'nonexistent-adrs-dir')
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `adr_dirs:\n  - "${nonexistent}"\nstrict_ci_paths: false\n`)

    const origDir = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const res = validator.validateADRDirsExist()
      assert.strictEqual(res.violations.length, 0)
      assert(res.warnings.some(w => w.includes('does not exist') && w.includes('nonexistent-adrs-dir')))
      
      const unfilt = await validator.validateUnfiltered()
      assert(unfilt.warnings.some(w => w.includes('does not exist')))
      assert(!unfilt.violations.some(v => v.includes('does not exist')))
    } finally {
      process.chdir(origDir)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  await testAsync('adr_dirs inexistente com strict_ci_paths true gera violation', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-nonexistent-violation-'))
    const nonexistent = path.join(tmp, 'nonexistent-adrs-dir')
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `adr_dirs:\n  - "${nonexistent}"\nstrict_ci_paths: true\n`)

    const origDir = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const res = validator.validateADRDirsExist()
      assert.strictEqual(res.warnings.length, 0)
      assert(res.violations.some(v => v.includes('does not exist') && v.includes('nonexistent-adrs-dir')))

      const unfilt = await validator.validateUnfiltered()
      assert(unfilt.violations.some(v => v.includes('does not exist')))
    } finally {
      process.chdir(origDir)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  test('adr_orphan isenta arquivos de ADR externos à raiz do projeto (cwd)', () => {
    const externalDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-external-adrs-'))
    fs.writeFileSync(path.join(externalDir, 'ADR-EXTERNAL-999.md'), '---\nstatus: Accepted\n---\n# External ADR\n')

    const projectDir = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-project-dir-'))
    fs.mkdirSync(path.join(projectDir, 'docs/req'), { recursive: true })
    fs.mkdirSync(path.join(projectDir, 'docs/adr'), { recursive: true })
    fs.writeFileSync(path.join(projectDir, 'docs/adr/ADR-LOCAL-001.md'), '---\nstatus: Accepted\n---\n# Local ADR\n')
    fs.writeFileSync(path.join(projectDir, 'trackfw.yaml'), `adr_dirs:\n  - docs/adr\n  - "${externalDir}"\n`)

    const origDir = process.cwd()
    process.chdir(projectDir)
    config.reset()
    try {
      const violations = validator.validateADRsAreReferenced()
      // ADR-LOCAL-001.md não está em nenhuma REQ -> deve ser marcado como violation adr_orphan
      assert(violations.some(v => v.includes('ADR-LOCAL-001.md')), 'ADR local não referenciado deve ser órfão')
      // ADR-EXTERNAL-999.md está fora do cwd -> DEVE SER ISENTO de adr_orphan
      assert(!violations.some(v => v.includes('ADR-EXTERNAL-999.md')), 'ADR externo deve ser ISENTO de adr_orphan')
    } finally {
      process.chdir(origDir)
      config.reset()
      fs.rmSync(externalDir, { recursive: true, force: true })
      fs.rmSync(projectDir, { recursive: true, force: true })
    }
  })

  // Estado analyzing: não deve gerar folder_status warning
  test('analyzing state: roadmap em analyzing/ com status: analyzing não gera folder_status warning', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-analyzing-'))
    const roadmapDir = path.join(tmp, 'roadmaps')
    fs.mkdirSync(path.join(roadmapDir, 'analyzing'), { recursive: true })
    fs.writeFileSync(
      path.join(roadmapDir, 'analyzing', 'ROADMAP-em-analise.md'),
      '---\nstatus: analyzing\ndate: 2026-07-26\n---\n# Roadmap: Em Análise\n'
    )
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${roadmapDir}\n`)

    const origCwd = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const result = validator.validateFolderStatusCoherence()
      assert(
        !result.some(w => w.includes('ROADMAP-em-analise.md')),
        `Roadmap em analyzing/ NÃO deve gerar folder_status warning, obteve: ${JSON.stringify(result)}`
      )
    } finally {
      process.chdir(origCwd)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  // Estado analyzing: wip_limit NÃO deve contar roadmaps em analyzing/
  test('analyzing state: wip_limit não conta roadmaps em analyzing/', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-analyzing-wip-'))
    const roadmapDir = path.join(tmp, 'roadmaps')
    fs.mkdirSync(path.join(roadmapDir, 'wip'), { recursive: true })
    fs.mkdirSync(path.join(roadmapDir, 'analyzing'), { recursive: true })
    // wip_limit=1, 1 em wip, 1 em analyzing → não deve exceder
    fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `roadmap_dir: ${roadmapDir}\nwip_limit: 1\nwip_by_squad: false\n`)
    fs.writeFileSync(path.join(roadmapDir, 'wip', 'ROADMAP-em-wip.md'), '# Roadmap em WIP\n\nREQ: REQ-001\n')
    fs.writeFileSync(
      path.join(roadmapDir, 'analyzing', 'ROADMAP-em-analise.md'),
      '---\nstatus: analyzing\n---\n# Roadmap em Análise\n'
    )

    const origCwd = process.cwd()
    process.chdir(tmp)
    config.reset()
    try {
      const result = validator.validateWIPLimit()
      assert.strictEqual(
        result.warnings.length, 0,
        `wip_limit NÃO deve contar analyzing/ — esperado 0 warnings, obteve: ${JSON.stringify(result.warnings)}`
      )
    } finally {
      process.chdir(origCwd)
      config.reset()
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  })

  // ── validateBranchHasWIPRoadmap — 4 cenários (P4 do ADR) ──────────────────
  test('branch_has_wip_roadmap: cenário 1 — roadmap em wip/ com slug → sem violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip1-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', 'ROADMAP-my-feature.md'), 'REQ: REQ-001\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert.strictEqual(violations.length, 0, `roadmap em wip/ com slug deve passar, obteve: ${JSON.stringify(violations)}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('branch_has_wip_roadmap: cenário 2 — roadmap em done/ com slug → sem violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip2-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'done', 'ROADMAP-my-feature.md'), 'REQ: REQ-001\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert.strictEqual(violations.length, 0, `roadmap em done/ com slug deve passar, obteve: ${JSON.stringify(violations)}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('branch_has_wip_roadmap: cenário 3 — nenhum roadmap em wip/ nem done/ → violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip3-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert(violations.length > 0, 'sem roadmap em wip/ nem done/ deve reprovar')
        assert(violations[0].includes('no roadmap is in wip/ nor done/'), `mensagem esperada, obteve: ${violations[0]}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('branch_has_wip_roadmap: cenário 4 — roadmap em done/ com slug DIFERENTE → violation', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-bwip4-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'done', 'ROADMAP-outra-coisa.md'), 'REQ: REQ-001\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/my-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert(violations.length > 0, 'slug diferente em done/ deve reprovar')
        assert(violations[0].includes('no matching roadmap in wip/ nor done/'), `mensagem esperada, obteve: ${violations[0]}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // -------------------------------------------------------------------------
  // Testes P2/P3 — adicionados pelo ML-2A (REQ-2026-07-26-robustez-gates)
  // -------------------------------------------------------------------------

  test('contentHasMarker: campo vazio CRLF não deve contar como presente (P3)', () => {
    const content = '# Roadmap\r\nREQ: \r\n## Seção\r\n'
    assert(!validator.contentHasMarker(content, ['REQ:']), 'campo vazio com CRLF não deve ser tratado como presente')
  })

  test('contentHasMarker: campo preenchido CRLF deve contar como presente (P3)', () => {
    const content = '# Roadmap\r\nREQ: REQ-001-titulo.md\r\n## Seção\r\n'
    assert(validator.contentHasMarker(content, ['REQ:']), 'campo preenchido com CRLF deve ser tratado como presente')
  })

  test('validateFolderStatusCoherence: diretório não legível (ENOTDIR) gera warning (P2)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-fsc-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps'), { recursive: true })
      // "analyzing" como arquivo regular — ENOTDIR ao tentar listar
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'analyzing'), 'eu sou um arquivo')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const warnings = validator.validateFolderStatusCoherence()
        assert(warnings.some(w => w.includes('could not read directory')),
          `esperado warning sobre diretório ilegível, obteve: ${JSON.stringify(warnings)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('validateFilenameUniqueness: diretório não legível (ENOTDIR) gera violation (P2)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-fnu-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps'), { recursive: true })
      // "wip" como arquivo regular — ENOTDIR ao tentar listar
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), 'eu sou um arquivo')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const violations = validator.validateFilenameUniqueness()
        assert(violations.some(v => v.includes('could not read directory')),
          `esperado violation sobre diretório ilegível, obteve: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('validateFilenameUniqueness: estados na mensagem em ordem alfabética (P3)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-p3-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', 'ROADMAP-duplicado.md'), '# Dup\n')
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'done', 'ROADMAP-duplicado.md'), '# Dup\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const violations = validator.validateFilenameUniqueness()
        assert(violations.length === 1, `esperado 1 violation, obteve ${violations.length}`)
        assert(violations[0].includes('[done, wip]'),
          `estados devem estar em ordem alfabética (done antes de wip), obteve: ${violations[0]}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('adr_dir_exists: tag correta em Node.js (P3 — paridade com Go/Python)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-adr-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs'), { recursive: true })
      // NÃO criar docs/adr — forçar violação
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'strict_ci_paths: true\nadr_dirs:\n  - docs/adr\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const result = validator.validateADRDirsExist()
        assert(result.violations.length > 0, 'esperado violation quando adr_dir não existe')
        assert(result.violations[0].includes('adr_dir "'), `mensagem deve usar 'adr_dir "', obteve: ${result.violations[0]}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // -------------------------------------------------------------------------
  // Testes P3+P4 adicionados pelo ML-3A (REQ-2026-07-26-robustez-gates)
  // -------------------------------------------------------------------------

  test('branch_has_wip_roadmap: 4 candidatos → mensagem truncada em 3 + "e mais 1" em ordem alfabética (P3+P4)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-trunc-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      // 4 roadmaps sem slug da branch → todos são candidatos, nenhum casa
      for (const name of ['ROADMAP-alpha.md', 'ROADMAP-bravo.md', 'ROADMAP-charlie.md', 'ROADMAP-delta.md']) {
        fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', name), 'REQ: REQ-001\n')
      }
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        process.env.TRACKFW_BRANCH = 'feat/minha-feature'
        const violations = validator.validateBranchHasWIPRoadmap()
        assert(violations.length > 0, 'esperava violation com 4 candidatos sem slug correspondente')
        const want = 'ROADMAP-alpha.md, ROADMAP-bravo.md, ROADMAP-charlie.md, e mais 1'
        assert(violations[0].includes(want),
          `mensagem truncada deve conter "${want}", obteve: ${violations[0]}`)
      } finally {
        delete process.env.TRACKFW_BRANCH
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('wip_has_req: roadmap CRLF com REQ vazio emite violation (P3+P4 — integração)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-crlf-wip-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      // Arquivo CRLF: REQ: seguido de espaço + \r\n — campo vazio
      fs.writeFileSync(path.join(tmp, 'docs', 'roadmaps', 'wip', 'ROADMAP-crlf.md'),
        'REQ: \r\n## Acceptance Criteria\r\n- [ ] ok\r\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const violations = validator.validateWIPHasREQ()
        assert(violations.some(v => v.includes('wip but has no linked REQ')),
          `esperava violation de REQ vazio com CRLF, obteve: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // ---------------------------------------------------------------------------
  // ML-2A — REQ-2026-07-27-convergencia-templates-python (reativado)
  // Após convergência dos templates Python, as regras devem detectar os artefatos
  // no formato canônico Go/Node/Python. Testes convertidos de testSkip para test.
  // ---------------------------------------------------------------------------

  // ML-2A: adrIsDraft detecta ADR no formato canônico (após convergência Python)
  // ADR canônico tem "> Date: … | Status: Draft" — detectado por adrIsDraft().
  // Fixture: ADR canônico + REQ canônica.
  test('ML-2A: adrIsDraft detecta ADR Draft no formato canonico (REQ-2026-07-27-convergencia-templates-python)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-adr-can-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })

      // ADR no formato canônico (produzido pelo gerador Python após ML-2A):
      // header "> Date: … | Status: Draft" — detectado por adrIsDraft()
      const adrCanonico = `---\nstatus: Draft\ndate: 2026-07-27\nauthor: ""\n---\n\n# ADR: auth strategy\n\n> Date: 2026-07-27 | Status: Draft\n\n## Context\n<!-- What is the situation that motivates this decision? -->\n\n## Decision\n<!-- What was decided? -->\n\n## Consequences\n<!-- What are the positive and negative consequences of this decision? -->\n\n## Alternatives Considered\n<!-- What other options were evaluated and why were they rejected? -->\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-07-27-auth-strategy.md'), adrCanonico)

      // REQ no formato canônico Go/Node: tem "> Date: … | Status: Open"
      const reqCanonicalContent = `# REQ: Login\n\n> Date: 2026-07-27 | Status: Open\n\n## Motivation\n\n## Acceptance Criteria\n\n- [ ] criterio\n\n## Linked ADR\nADR:\n\n## Blocked by ADRs\n- ADR-2026-07-27-auth-strategy.md (Draft)\n\n## Linked Roadmap\nRoadmap:\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'req', 'REQ-2026-07-27-login.md'), reqCanonicalContent)
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `req_dir: docs/req\nadr_dirs:\n  - docs/adr\n`)

      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        // Pré-condição: ADR existe
        assert(fs.existsSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-07-27-auth-strategy.md')),
          'pré-condição: ADR não encontrado')

        const violations = validator.validateREQsNotBlockedByDraftADRs()
        // DEVE disparar violation — formato canônico tem "Status: Draft" que adrIsDraft detecta
        assert(violations.length > 0,
          `regressao: blocked_by_draft_adr nao detectou ADR Draft no formato canonico. ` +
          `adrIsDraft deve encontrar '| Status: Draft' inline. violations: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // ML-2A: validator detecta REQ Open no formato canônico (após convergência Python)
  // REQ canônica tem "> Date: … | Status: Open" — detectada pelo guard inicial.
  // Fixture: REQ canônica + ADR canônico Draft.
  test('ML-2A: validator detecta REQ Open no formato canonico (REQ-2026-07-27-convergencia-templates-python)', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-req-can-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })

      // ADR no formato canônico Go/Node: tem "> Date: … | Status: Draft"
      const adrCanonicalContent = `# ADR: Auth\n\n> Date: 2026-07-27 | Status: Draft\n\n## Context\ncontext\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-07-27-auth-draft.md'), adrCanonicalContent)

      // REQ no formato canônico (produzida pelo gerador Python após ML-2A):
      // header "> Date: … | Status: Open" detectado pelo guard inicial.
      const reqCanonico = `---\nstatus: Open\ndate: 2026-07-27\nauthor: ""\nadr: ""\nroadmap: ""\n---\n\n# REQ: login\n\n> Date: 2026-07-27 | Status: Open\n\n## Motivation\n<!-- Why is this requirement needed? What problem does it solve? -->\n\n## Acceptance Criteria\n- [ ]\n- [ ]\n\n## Linked ADR\n<!-- Reference the ADR that governs this requirement -->\nADR: \n\n## Blocked by ADRs\n- ADR-2026-07-27-auth-draft.md (Draft)\n\n## Linked Roadmap\n<!-- Reference the roadmap that implements this requirement -->\nRoadmap: \n`
      fs.writeFileSync(path.join(tmp, 'docs', 'req', 'REQ-2026-07-27-login.md'), reqCanonico)
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), `req_dir: docs/req\nadr_dirs:\n  - docs/adr\n`)

      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        // Pré-condição: ADR canônico deve ser detectado como Draft
        assert(validator.adrIsDraft('ADR-2026-07-27-auth-draft.md'),
          'pré-condição falhou: adrIsDraft deve retornar true para ADR canônico com Status: Draft')

        const violations = validator.validateREQsNotBlockedByDraftADRs()
        // DEVE disparar violation — formato canônico tem "Status: Open" que o guard detecta
        assert(violations.length > 0,
          `regressao: blocked_by_draft_adr nao detectou REQ Open no formato canonico. ` +
          `REQ tem '> Date: ... | Status: Open' (inline) — deve ser detectada. violations: ${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // ---------------------------------------------------------------------------
  // ML-1A — REQ-2026-07-27-integridade-referencias — testes negativos xfail
  // Semântica strict: testSkip executa o corpo; se o defeito for corrigido sem reativação
  // do teste, reporta XPASS e incrementa failed → make quality fica vermelho.
  // ---------------------------------------------------------------------------

  // Escape 1 reativado no ML-2A: frontmatter roadmap: é validado.
  test('ML-2A Escape1 reativado: frontmatter roadmap: inexistente gera aviso', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-e1-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/done'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
      // REQ: frontmatter aponta para inexistente; corpo não tem Roadmap:
      fs.writeFileSync(path.join(tmp, 'docs/req/REQ-XFAIL-ESCAPE1.md'),
        '---\nstatus: Open\nroadmap: "docs/roadmaps/wip/NAO-EXISTE-ESCAPE-1.md"\n---\n\n' +
        '## Linked Roadmap\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
        'req_dir: docs/req\nroadmap_dir: docs/roadmaps\n')
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const warnings = validator.validateRefTargetsExist()
        assert(warnings.some(w => w.includes('NAO-EXISTE-ESCAPE-1')),
          `esperava aviso para frontmatter roadmap: inexistente. warnings=${JSON.stringify(warnings)}`)
      } finally {
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // Escape 2 reativado no ML-2A: fallback por basename foi removido.
  test('ML-2A Escape2 reativado: caminho errado wip/ vs done/ gera aviso', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-e2-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/done'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
      // Arquivo real em done/ — basename fallback o encontra mesmo com path errado em wip/
      fs.writeFileSync(path.join(tmp, 'docs/roadmaps/done/ESCAPE2-ROADMAP.md'),
        '# Roadmap\n## Acceptance Criteria\n- [x] done\n')
      // REQ: corpo aponta para wip/ (errado) — arquivo real está em done/
      fs.writeFileSync(path.join(tmp, 'docs/req/REQ-XFAIL-ESCAPE2.md'),
        '---\nstatus: Open\n---\n\n' +
        '## Linked Roadmap\nRoadmap: docs/roadmaps/wip/ESCAPE2-ROADMAP.md\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
        'req_dir: docs/req\nroadmap_dir: docs/roadmaps\n')
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const warnings = validator.validateRefTargetsExist()
        assert(warnings.some(w => w.includes('ESCAPE2-ROADMAP')),
          `esperava aviso para caminho errado (wip/ vs done/). warnings=${JSON.stringify(warnings)}`)
      } finally {
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // Escape 3 reativado no ML-3A: severidade padrão error reprova ref quebrada.
  await testAsync('ML-3A Escape3 reativado: ref_targets_exist default error reprova gate', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-e3-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/done'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/backlog'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/blocked'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
      // REQ com Roadmap: verdadeiramente inexistente (não há match de basename)
      fs.writeFileSync(path.join(tmp, 'docs/req/REQ-XFAIL-ESCAPE3.md'),
        '---\nstatus: Open\n---\n\n' +
        '## Linked Roadmap\nRoadmap: docs/roadmaps/wip/ESCAPE3-TRULY-MISSING.md\n')
      // Config padrão — ref_targets_exist deve reprovar como "error"
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
        'req_dir: docs/req\nroadmap_dir: docs/roadmaps\n')
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        // Usa validateUnfiltered() que aplica severidade default antes do ratchet.
        const { violations } = await validator.validateUnfiltered()
        assert(violations.some(v => v.includes('ESCAPE3-TRULY-MISSING')),
          `esperava violation para ref quebrada com severidade default error. violations=${JSON.stringify(violations)}`)
      } finally {
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // Defeito 2 reativado no ML-2B: REQ Open com roadmap em done/ é sinalizada.
  await testAsync('ML-2B Defeito2 reativado: REQ Open com roadmap em done/ gera warning', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-d2-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/done'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/blocked'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
      // Roadmap real em done/ — entregue
      fs.writeFileSync(path.join(tmp, 'docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md'),
        '---\nstatus: Done\n---\n# Roadmap concluído\n## Acceptance Criteria\n- [x] done\n')
      // REQ ainda Open mas roadmap está em done/
      fs.writeFileSync(path.join(tmp, 'docs/req/REQ-XFAIL-DEFEITO2.md'),
        '---\nstatus: Open\nroadmap: "docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md"\n---\n\n' +
        '# REQ: Defeito 2\n\n> Date: 2026-07-27 | Status: Open\n\n' +
        '## Linked Roadmap\nRoadmap: docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
        'req_dir: docs/req\nroadmap_dir: docs/roadmaps\n')
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const { violations, warnings } = await validator.validateUnfiltered()
        // Defeito corrigido: a inconsistência de ciclo de vida deve aparecer.
        const allMsgs = [...violations, ...warnings]
        assert(allMsgs.some(m => m.includes('DONE-ROADMAP-DEFEITO2')),
          `esperava mensagem sobre REQ Open com roadmap em done/. allMsgs=${JSON.stringify(allMsgs)}`)
      } finally {
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('ML-2A stale_wip usa entrada .trackfw-log backlog → wip como idade', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-stale-log-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
      const roadmapPath = path.join(tmp, 'docs/roadmaps/wip/ROADMAP-old-wip.md')
      fs.writeFileSync(roadmapPath,
        '---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n')
      fs.writeFileSync(path.join(tmp, 'docs/roadmaps/.trackfw-log'),
        '2026-07-10 10:00  ROADMAP-old-wip.md                                backlog → wip\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
        'roadmap_dir: docs/roadmaps\nreq_dir: docs/req\nadr_dirs:\n  - docs/adr\n')
      const now = new Date()
      fs.utimesSync(roadmapPath, now, now)
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      validator.setStaleWipNowForTests(() => Date.parse('2026-07-27T12:00:00'))
      try {
        const warnings = validator.validateStaleWIP()
        assert(warnings.some(w => w.includes('ROADMAP-old-wip.md')),
          `esperava stale_wip pela entrada antiga do .trackfw-log; warnings=${JSON.stringify(warnings)}`)
      } finally {
        validator.setStaleWipNowForTests(null)
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('ML-2A stale_wip respeita latest transition boundary e stale_wip_days', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-stale-boundary-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
      const roadmapPath = path.join(tmp, 'docs/roadmaps/wip/ROADMAP-boundary.md')
      fs.writeFileSync(roadmapPath, '---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n')
      fs.writeFileSync(path.join(tmp, 'docs/roadmaps/.trackfw-log'),
        '2026-07-01 10:00  ROADMAP-boundary.md                               backlog → wip\n' +
        '2026-07-26 10:01  ROADMAP-boundary.md                               blocked → wip\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\nstale_wip_days: 2\n')
      fs.utimesSync(roadmapPath, new Date('2026-06-01T10:00:00'), new Date('2026-06-01T10:00:00'))
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        validator.setStaleWipNowForTests(() => Date.parse('2026-07-28T10:01:00'))
        let warnings = validator.validateStaleWIP()
        assert(warnings.some(w => w.includes('ROADMAP-boundary.md')),
          `boundary de 2 dias deveria gerar warning; warnings=${JSON.stringify(warnings)}`)
        validator.setStaleWipNowForTests(() => Date.parse('2026-07-28T10:00:59'))
        warnings = validator.validateStaleWIP()
        assert(!warnings.some(w => w.includes('ROADMAP-boundary.md')),
          `abaixo do boundary não deveria gerar warning; warnings=${JSON.stringify(warnings)}`)
      } finally {
        validator.setStaleWipNowForTests(null)
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('ML-2A stale_wip usa mtime quando log está ausente', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-stale-mtime-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps/wip'), { recursive: true })
      const roadmapPath = path.join(tmp, 'docs/roadmaps/wip/ROADMAP-mtime.md')
      fs.writeFileSync(roadmapPath, '---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\nstale_wip_days: 3\n')
      fs.utimesSync(roadmapPath, new Date('2026-07-20T09:00:00'), new Date('2026-07-20T09:00:00'))
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      validator.setStaleWipNowForTests(() => Date.parse('2026-07-24T09:00:00'))
      try {
        const warnings = validator.validateStaleWIP()
        assert(warnings.some(w => w.includes('ROADMAP-mtime.md')),
          `fallback por mtime deveria gerar warning; warnings=${JSON.stringify(warnings)}`)
      } finally {
        validator.setStaleWipNowForTests(null)
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  test('ML-2B stale_wip diagnostica erro de walk em wip/', () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-stale-walk-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs/roadmaps'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs/adr'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'docs/roadmaps/wip'), 'not a directory\n')
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'),
        'roadmap_dir: docs/roadmaps\nreq_dir: docs/req\nadr_dirs:\n  - docs/adr\n')
      const origDir = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const warnings = validator.validateStaleWIP()
        assert(warnings.some(w => w.includes('wip')),
          `esperava diagnostico para erro de walk/ENOTDIR em wip/; warnings=${JSON.stringify(warnings)}`)
      } finally {
        process.chdir(origDir)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // ---------------------------------------------------------------------------
  // ML-1B — REQ-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida
  // Helper canônico adrNotAcceptedStatusForRule (Draft ou Proposed) + regra nova
  // adr_accepted_when_req_done. blocked_by_draft_adr deixa de ser cega a Proposed.
  // ---------------------------------------------------------------------------

  function withTmpProject(fn) {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-adr-accepted-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })
      fs.writeFileSync(path.join(tmp, 'trackfw.yaml'), 'req_dir: docs/req\nadr_dirs:\n  - docs/adr\n')
      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        fn(tmp)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally {
      fs.rmSync(tmp, { recursive: true, force: true })
    }
  }

  function writeAdr(tmp, basename, status) {
    fs.writeFileSync(
      path.join(tmp, 'docs', 'adr', basename),
      `---\nstatus: ${status}\ndate: 2026-08-01\nauthor: ""\n---\n\n# ADR: fixture\n\n> Date: 2026-08-01 | Status: ${status}\n\n## Context\ncontext\n`
    )
  }

  function writeReq(tmp, basename, reqStatus, adrBasename) {
    fs.writeFileSync(
      path.join(tmp, 'docs', 'req', basename),
      `---\nstatus: ${reqStatus}\ndate: 2026-08-01\nauthor: ""\nadr: "docs/adr/${adrBasename}"\n---\n\n# REQ: fixture\n\n> Date: 2026-08-01 | Status: ${reqStatus}\n\n## Motivation\n\n## Acceptance Criteria\n- [ ]\n\n## Linked ADR\nADR: docs/adr/${adrBasename}\n\n## Linked Roadmap\nRoadmap:\n`
    )
  }

  test('adr_accepted_when_req_done: REQ Done + ADR Proposed -> violation citando ambos', () => {
    withTmpProject((tmp) => {
      writeAdr(tmp, 'ADR-2026-08-01-fixture.md', 'Proposed')
      writeReq(tmp, 'REQ-2026-08-01-fixture.md', 'Done', 'ADR-2026-08-01-fixture.md')
      const violations = validator.validateADRAcceptedWhenREQDone()
      assert.strictEqual(violations.length, 1, `esperava 1 violation; got ${JSON.stringify(violations)}`)
      assert(violations[0].includes('REQ-2026-08-01-fixture.md'), 'mensagem deve citar a REQ')
      assert(violations[0].includes('ADR-2026-08-01-fixture.md'), 'mensagem deve citar o ADR')
    })
  })

  test('adr_accepted_when_req_done: REQ Done + ADR Draft -> violation', () => {
    withTmpProject((tmp) => {
      writeAdr(tmp, 'ADR-2026-08-01-fixture.md', 'Draft')
      writeReq(tmp, 'REQ-2026-08-01-fixture.md', 'Done', 'ADR-2026-08-01-fixture.md')
      const violations = validator.validateADRAcceptedWhenREQDone()
      assert.strictEqual(violations.length, 1, `esperava 1 violation; got ${JSON.stringify(violations)}`)
    })
  })

  test('adr_accepted_when_req_done: REQ Done + ADR Superseded -> sem violation (aceito por exclusao)', () => {
    withTmpProject((tmp) => {
      writeAdr(tmp, 'ADR-2026-08-01-fixture.md', 'Superseded')
      writeReq(tmp, 'REQ-2026-08-01-fixture.md', 'Done', 'ADR-2026-08-01-fixture.md')
      const violations = validator.validateADRAcceptedWhenREQDone()
      assert.strictEqual(violations.length, 0, `esperava 0 violations; got ${JSON.stringify(violations)}`)
    })
  })

  test('adr_accepted_when_req_done: REQ Open + ADR Proposed -> sem violation da regra nova', () => {
    withTmpProject((tmp) => {
      writeAdr(tmp, 'ADR-2026-08-01-fixture.md', 'Proposed')
      writeReq(tmp, 'REQ-2026-08-01-fixture.md', 'Open', 'ADR-2026-08-01-fixture.md')
      const violations = validator.validateADRAcceptedWhenREQDone()
      assert.strictEqual(violations.length, 0, `esperava 0 violations; got ${JSON.stringify(violations)}`)
    })
  })

  test('blocked_by_draft_adr: REQ Open bloqueada por ADR Proposed -> violation (correcao da cegueira)', () => {
    withTmpProject((tmp) => {
      writeAdr(tmp, 'ADR-2026-08-01-fixture.md', 'Proposed')
      const reqContent = `---\nstatus: Open\ndate: 2026-08-01\nauthor: ""\nadr: ""\n---\n\n# REQ: fixture\n\n> Date: 2026-08-01 | Status: Open\n\n## Motivation\n\n## Acceptance Criteria\n- [ ]\n\n## Linked ADR\nADR:\n\n## Blocked by ADRs\n- ADR-2026-08-01-fixture.md (Proposed)\n\n## Linked Roadmap\nRoadmap:\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'req', 'REQ-2026-08-01-fixture.md'), reqContent)
      const violations = validator.validateREQsNotBlockedByDraftADRs()
      assert.strictEqual(violations.length, 1,
        `regressao: blocked_by_draft_adr continua cego a Proposed; got ${JSON.stringify(violations)}`)
      assert(violations[0].includes('ADR-2026-08-01-fixture.md'))
    })
  })

  test('adr_accepted_when_req_done: ADR Accepted cuja PROSA cita "Status: Draft"/"Proposed" -> sem violation (anchoring, nao substring livre)', () => {
    withTmpProject((tmp) => {
      // Cabeçalho real é "Accepted"; o corpo do documento (prosa) menciona literalmente as strings
      // "Status: Draft" e "Status: Proposed" ao documentar o próprio defeito que este ADR corrige.
      // Uma detecção por content.includes() classificaria isso erroneamente como "não aceito".
      const adrContent = `---\nstatus: Accepted\ndate: 2026-08-01\nauthor: ""\n---\n\n# ADR: fixture\n\n> Date: 2026-08-01 | Status: Accepted\n\n## Context\n\nO defeito antigo testava \`content.includes("Status: Draft")\`. Um ADR emitido por\n\`adr new\` produz \`Status: Proposed\` no cabeçalho.\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-08-01-fixture.md'), adrContent)
      writeReq(tmp, 'REQ-2026-08-01-fixture.md', 'Done', 'ADR-2026-08-01-fixture.md')

      // Pré-condição: prova que a fixture de fato contém as substrings problemáticas na prosa.
      assert(adrContent.includes('Status: Draft'), 'pré-condição: fixture deve conter "Status: Draft" na prosa')
      assert(adrContent.includes('Status: Proposed'), 'pré-condição: fixture deve conter "Status: Proposed" na prosa')

      const violations = validator.validateADRAcceptedWhenREQDone()
      assert.strictEqual(violations.length, 0,
        `regressao: ADR Accepted classificado como nao aceito por causa de menção na prosa; got ${JSON.stringify(violations)}`)
    })
  })

  // ML-1B (2026-08-02) — teste discriminante do ADR-2026-08-02: REQ Done com `adr: ""` no
  // frontmatter (early-return legítimo, não é a causa) e o campo `ADR:` real só no CORPO,
  // entre backticks — réplica fiel da forma real usada em 13 REQs do repositório. Antes
  // da correção do regex de extractRefPath, este caso não violava (falso negativo, em
  // silêncio); depois, deve violar.
  test('adr_accepted_when_req_done: ADR referenciado entre backticks no corpo (frontmatter adr vazio) -> viola', () => {
    withTmpProject((tmp) => {
      writeAdr(tmp, 'ADR-2026-08-02-fixture.md', 'Proposed')
      const reqContent = `---\nstatus: Done\ndate: 2026-08-02\nauthor: ""\nadr: ""\n---\n\n# REQ: fixture\n\n> Date: 2026-08-02 | Status: Done\n\n## Motivation\n\n## Acceptance Criteria\n- [ ]\n\n## Linked ADR\nADR: \`docs/adr/ADR-2026-08-02-fixture.md\` (P1–P4; esta REQ é derivada)\n\n## Linked Roadmap\nRoadmap:\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'req', 'REQ-2026-08-02-fixture.md'), reqContent)

      // Prova "antes": simula o regex antigo (sem backtick no charset) sobre o mesmo conteúdo —
      // deve extrair um valor que NÃO termina em .md, ou seja, extractRefPath antigo retornaria
      // null e a violação NÃO seria detectada.
      const oldExtract = (content, field) => {
        for (const line of content.split('\n')) {
          const trimmed = line.trim()
          const idx = trimmed.indexOf(':')
          if (idx !== -1 && trimmed.slice(0, idx).trim().toLowerCase() === field.toLowerCase()) {
            let val = trimmed.slice(idx + 1).trim()
            if (!val || val === '—' || val === '-' || val === '–') return null
            val = val.split(/\s+/)[0]
            val = val.replace(/^["']|["']$/g, '') // regex ANTIGO — sem backtick
            if (val.endsWith('.md')) return val
          }
        }
        return null
      }
      const before = oldExtract(reqContent, 'ADR')
      assert.strictEqual(before, null,
        `pré-condição do teste discriminante falhou: mecanismo antigo deveria retornar null (referência invisível), obteve ${JSON.stringify(before)}`)

      // Depois da correção: extractRefPath (atual) resolve o caminho normalmente.
      const after = validator.extractRefPath(reqContent, 'ADR')
      assert.strictEqual(after, 'docs/adr/ADR-2026-08-02-fixture.md',
        `extractRefPath atual deveria resolver a referência entre backticks; obteve ${JSON.stringify(after)}`)

      const violations = validator.validateADRAcceptedWhenREQDone()
      assert.strictEqual(violations.length, 1,
        `esperava 1 violation (ADR Proposed referenciado entre backticks por REQ Done); got ${JSON.stringify(violations)}`)
      assert(violations[0].includes('REQ-2026-08-02-fixture.md'))
      assert(violations[0].includes('ADR-2026-08-02-fixture.md'))
    })
  })

  // Prova direta com as 3 REQs REAIS do repositório citadas no ADR-2026-08-02 — sem `adr:`
  // efetivo no frontmatter, ADR referenciado só no corpo entre backticks. Deve resolver.
  test('extractRefPath resolve o ADR das 3 REQs reais do repositório com backtick no corpo', () => {
    const repoRoot = path.resolve(__dirname, '..', '..')
    const reqFiles = [
      'docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md',
      'docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md',
      'docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md',
    ]
    for (const rel of reqFiles) {
      const full = path.join(repoRoot, rel)
      const content = fs.readFileSync(full, 'utf8')
      const ref = validator.extractRefPath(content, 'ADR')
      assert(ref && ref.endsWith('.md'),
        `REQ real ${rel}: esperava ADR resolvido (.md), obteve ${JSON.stringify(ref)}`)
      assert.strictEqual(path.basename(ref), 'ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md',
        `REQ real ${rel}: ADR resolvido inesperado ${JSON.stringify(ref)}`)
    }
  })

  // ML-1D (2026-08-01) — divergência A da auditoria de paridade: o Node lia só a linha
  // de cabeçalho ("| Status: X"), ignorando o frontmatter. Um ADR com frontmatter
  // `status:` e SEM nenhuma linha de cabeçalho é o caso que discriminava o Node do Go e
  // do Python. Esta prova de teste falha antes da correção (com extractAdrHeaderStatus
  // como única fonte, o status resolvido seria '' -> não-aceito ficaria false).
  test('adr_accepted_when_req_done: ADR com frontmatter status e SEM linha de cabeçalho -> resolve pelo frontmatter', () => {
    withTmpProject((tmp) => {
      const adrContent = `---\nstatus: Proposed\ndate: 2026-08-01\nauthor: ""\n---\n\n# ADR: sem cabeçalho\n\n## Context\ncontext\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'adr', 'ADR-2026-08-01-fixture.md'), adrContent)
      writeReq(tmp, 'REQ-2026-08-01-fixture.md', 'Done', 'ADR-2026-08-01-fixture.md')

      // Pré-condição: prova que a fixture de fato não tem linha "| Status: ".
      assert(!adrContent.includes('| Status: '), 'pré-condição: fixture não deve ter linha de cabeçalho')

      const violations = validator.validateADRAcceptedWhenREQDone()
      assert.strictEqual(violations.length, 1,
        `esperava violation resolvida via frontmatter mesmo sem cabeçalho; got ${JSON.stringify(violations)}`)
    })
  })

  test('resolveAdrStatus: frontmatter presente e sem linha de cabeçalho -> resolve pelo frontmatter', () => {
    const content = '---\nstatus: Accepted\ndate: 2026-08-01\n---\n\n# ADR: sem cabeçalho\n\n## Context\nctx\n'
    assert.strictEqual(validator.resolveAdrStatus(content), 'Accepted')
  })

  test('extractAdrHeaderStatus: ancorado na linha de cabeçalho, ignora ocorrências soltas no corpo', () => {
    const content = 'texto solto contendo Status: Draft em algum lugar\n\n> Date: 2026-08-01 | Status: Accepted\n'
    assert.strictEqual(validator.extractAdrHeaderStatus(content), 'Accepted')
  })

  test('adr_accepted_when_req_done: registrada em config.defaults() com severidade error', () => {
    const defaults = config.defaults()
    assert.strictEqual(defaults.rules.adr_accepted_when_req_done, 'error')
  })

  // ML-1E (2026-08-01) — o bloco de resumo de getStatus() hardcodava "(Draft)" para
  // qualquer ADR não aceito, mesmo quando o status real era "Proposed". Um ADR Proposed
  // bloqueador deve aparecer com o status real "(Proposed)", nunca "(Draft)".
  await testAsync('getStatus: REQ bloqueada por ADR Proposed exibe "(Proposed)" no resumo, não "(Draft)"', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-status-proposed-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'blocked'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(
        path.join(tmp, 'trackfw.yaml'),
        'req_dir: docs/req\nadr_dirs:\n  - docs/adr\nroadmap_dir: docs/roadmaps\n'
      )

      writeAdr(tmp, 'ADR-2026-08-01-fixture.md', 'Proposed')
      const reqContent = `---\nstatus: Open\ndate: 2026-08-01\nauthor: ""\nadr: ""\n---\n\n# REQ: fixture\n\n> Date: 2026-08-01 | Status: Open\n\n## Motivation\n\n## Acceptance Criteria\n- [ ]\n\n## Linked ADR\nADR:\n\n## Blocked by ADRs\n- ADR-2026-08-01-fixture.md (Proposed)\n\n## Linked Roadmap\nRoadmap:\n`
      fs.writeFileSync(path.join(tmp, 'docs', 'req', 'REQ-2026-08-01-fixture.md'), reqContent)

      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const out = await validator.getStatus()
        assert(out.includes('⏳ REQs blocked by not-accepted ADRs'),
          `resumo deve usar o cabeçalho neutro; got: ${out}`)
        assert(out.includes('ADR-2026-08-01-fixture.md (Proposed)'),
          `resumo deve mostrar o status real (Proposed); got: ${out}`)
        assert(!out.includes('ADR-2026-08-01-fixture.md (Draft)'),
          `regressao: resumo rotulou um ADR Proposed como (Draft); got: ${out}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  // ML-1B (2026-08-02) — getStatus() ganhou o bloco "📊 Inventory" no topo, discriminando
  // roadmaps pelos 6 estados (incluindo "analyzing", historicamente omitido) e REQs por status
  // real (Open/Done/Closed). Este teste prova que "analyzing" é contado — não apenas presente na
  // enumeração de estados, mas efetivamente encontrado num fixture com 1 roadmap lá dentro.
  await testAsync('getStatus Inventory: roadmap em analyzing/ é contado (antes não aparecia)', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-status-analyzing-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'backlog'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'analyzing'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'blocked'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'abandoned'), { recursive: true })
      fs.writeFileSync(
        path.join(tmp, 'trackfw.yaml'),
        'req_dir: docs/req\nadr_dirs:\n  - docs/adr\nroadmap_dir: docs/roadmaps\n'
      )
      fs.writeFileSync(
        path.join(tmp, 'docs', 'roadmaps', 'analyzing', 'ROADMAP-fixture.md'),
        '---\nstatus: analyzing\ndate: 2026-08-02\n---\n# Roadmap em análise\n'
      )

      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const out = await validator.getStatus()
        assert(out.includes('backlog 0 · analyzing 1 · wip 0'),
          `esperava "analyzing 1" na linha de contagem; got: ${out}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  await testAsync('getStatus Inventory: REQs discriminadas em Open/Done/Closed', async () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tw-status-reqs-'))
    try {
      fs.mkdirSync(path.join(tmp, 'docs', 'req'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'adr'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'wip'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'blocked'), { recursive: true })
      fs.mkdirSync(path.join(tmp, 'docs', 'roadmaps', 'done'), { recursive: true })
      fs.writeFileSync(
        path.join(tmp, 'trackfw.yaml'),
        'req_dir: docs/req\nadr_dirs:\n  - docs/adr\nroadmap_dir: docs/roadmaps\n'
      )
      fs.writeFileSync(
        path.join(tmp, 'docs', 'req', 'REQ-2026-08-02-open.md'),
        '---\nstatus: Open\ndate: 2026-08-02\n---\n# REQ: open\n'
      )
      fs.writeFileSync(
        path.join(tmp, 'docs', 'req', 'REQ-2026-08-02-done.md'),
        '---\nstatus: Done\ndate: 2026-08-02\n---\n# REQ: done\n'
      )
      fs.writeFileSync(
        path.join(tmp, 'docs', 'req', 'REQ-2026-08-02-closed.md'),
        '---\nstatus: Closed\ndate: 2026-08-02\n---\n# REQ: closed\n'
      )

      const origCwd = process.cwd()
      process.chdir(tmp)
      config.reset()
      try {
        const out = await validator.getStatus()
        assert(out.includes('REQs        3  (1 Open · 1 Done · 1 Closed)'),
          `esperava discriminação "(1 Open · 1 Done · 1 Closed)"; got: ${out}`)
      } finally {
        process.chdir(origCwd)
        config.reset()
      }
    } finally { fs.rmSync(tmp, { recursive: true, force: true }) }
  })

  console.log(`\n${passed} passed, ${failed} failed, ${skipped} xfail`)
  if (failed > 0) process.exit(1)
})()
