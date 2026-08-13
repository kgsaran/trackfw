'use strict'

// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.
// Mirrors internal/validator/validator_credential_guard_integrity_test.go (Go).

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const config = require('../src/config')
const validator = require('../src/validator')
const {
  validateCredentialGuardScriptIntegrity,
  validateCredentialGuardModeDowngrade,
  CREDENTIAL_GUARD_SCRIPT_REFERENCE,
} = validator

function tmpDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cg-integrity-'))
}

function writeFile(base, rel, content) {
  const full = path.join(base, rel)
  fs.mkdirSync(path.dirname(full), { recursive: true })
  fs.writeFileSync(full, content, 'utf8')
}

function git(dir, ...args) {
  execFileSync('git', args, { cwd: dir, stdio: ['ignore', 'pipe', 'pipe'] })
}

function initGitRepo(dir) {
  git(dir, 'init')
  git(dir, 'config', 'user.email', 'test@test.com')
  git(dir, 'config', 'user.name', 'test')
  git(dir, 'commit', '--allow-empty', '-m', 'init')
}

function commitTrackfwYAML(dir, content) {
  writeFile(dir, 'trackfw.yaml', content)
  git(dir, 'add', 'trackfw.yaml')
  git(dir, 'commit', '-m', 'trackfw.yaml')
}

// ---- credential_guard_script_integrity ----

test('credential_guard_script_integrity: silêncio quando o script não existe', () => {
  const dir = tmpDir()
  const msgs = validateCredentialGuardScriptIntegrity(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_script_integrity: silêncio quando o script é idêntico ao template', () => {
  const dir = tmpDir()
  writeFile(dir, 'scripts/trackfw-credential-guard.sh', CREDENTIAL_GUARD_SCRIPT_REFERENCE)
  const msgs = validateCredentialGuardScriptIntegrity(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_script_integrity: dispara em sobrescrita (mensagem causalmente neutra)', () => {
  const dir = tmpDir()
  writeFile(dir, 'scripts/trackfw-credential-guard.sh', '#!/usr/bin/env bash\nexit 0\n')
  const msgs = validateCredentialGuardScriptIntegrity(dir)
  assert.equal(msgs.length, 1)
  assert.match(msgs[0], /scripts\/trackfw-credential-guard\.sh/)
  assert.match(msgs[0], /diverges from the template/)
  const lower = msgs[0].toLowerCase()
  for (const forbidden of ['adulterad', 'modified by', 'tampered']) {
    assert.equal(lower.includes(forbidden), false, `mensagem não deve conter "${forbidden}"`)
  }
})

// ---- credential_guard_mode_downgrade ----

test('credential_guard_mode_downgrade: silêncio sem repositório git', () => {
  const dir = tmpDir()
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_mode_downgrade: silêncio sem nenhum commit', () => {
  const dir = tmpDir()
  git(dir, 'init')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_mode_downgrade: silêncio com trackfw.yaml não versionado no HEAD', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_mode_downgrade: silêncio quando HEAD não tem credential_guard.mode', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'roadmap_dir: docs/roadmaps\n')
  writeFile(dir, 'trackfw.yaml', 'roadmap_dir: docs/roadmaps\ncredential_guard:\n  mode: warn\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_mode_downgrade: regra é direcional — HEAD warn nunca dispara', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: warn\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: block\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_mode_downgrade: silêncio sem mudança (HEAD e disco ambos block)', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.deepEqual(msgs, [])
})

test('credential_guard_mode_downgrade: dispara em downgrade block -> warn', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.equal(msgs.length, 1)
  assert.match(msgs[0], /credential_guard\.mode: block/)
})

test('credential_guard_mode_downgrade: dispara quando a chave some do disco (bloco removido)', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'roadmap_dir: docs/roadmaps\ncredential_guard:\n  mode: block\n')
  writeFile(dir, 'trackfw.yaml', 'roadmap_dir: docs/roadmaps\n')
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.equal(msgs.length, 1)
})

test('credential_guard_mode_downgrade: dispara quando trackfw.yaml é deletado do disco', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  fs.rmSync(path.join(dir, 'trackfw.yaml'))
  const msgs = validateCredentialGuardModeDowngrade(dir)
  assert.equal(msgs.length, 1)
})

// ---- Configurável via rules: (pipeline completo, chdir) ----

test('credential_guard_script_integrity é configurável via rules: (default warning)', async () => {
  const dir = tmpDir()
  writeFile(dir, 'scripts/trackfw-credential-guard.sh', '#!/usr/bin/env bash\nexit 0\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations, warnings } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('scripts/trackfw-credential-guard.sh')), false)
    assert.equal(warnings.some(w => w.includes('scripts/trackfw-credential-guard.sh')), true)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

test('credential_guard_script_integrity: rules: error promove a violation', async () => {
  const dir = tmpDir()
  writeFile(dir, 'scripts/trackfw-credential-guard.sh', '#!/usr/bin/env bash\nexit 0\n')
  writeFile(dir, 'trackfw.yaml', 'rules:\n  credential_guard_script_integrity: error\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('scripts/trackfw-credential-guard.sh')), true)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

test('credential_guard_mode_downgrade é configurável via rules: (default error)', async () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('credential_guard.mode: block')), true)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard,
// ADR-2026-08-12-severidade-das-regras-de-credential-guard-...: as duas subtests abaixo COMMITAM
// a mudança de rules: junto com mode: block (a âncora do HEAD, que não pode ser removida, senão a
// regra silencia por falta de âncora — outro teste). Antes deste ADR, este teste escrevia
// "rules: <nome>: warning|off" só em disco, SEM commit — exatamente o auto-silenciamento sem
// rastro que o ADR fecha; ver "*_nao_commitado_ainda_dispara" abaixo para o canal fechado.

test('credential_guard_mode_downgrade: rules: warning commitado rebaixa para warning', async () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\nrules:\n  credential_guard_mode_downgrade: warning\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: warning\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations, warnings } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('credential_guard.mode: block')), false)
    assert.equal(warnings.some(w => w.includes('credential_guard.mode: block')), true)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

test('credential_guard_mode_downgrade: rules: off commitado silencia totalmente', async () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\nrules:\n  credential_guard_mode_downgrade: off\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: off\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations, warnings } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('credential_guard.mode: block')), false)
    assert.equal(warnings.some(w => w.includes('credential_guard.mode: block')), false)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

test('credential_guard_mode_downgrade: rules: warning NÃO commitado ainda dispara (violation)', async () => {
  const dir = tmpDir()
  initGitRepo(dir)
  // HEAD só tem mode: block — SEM rules:. Disco rebaixa mode E desliga a regra na MESMA edição,
  // nunca commitada. Ataque combinado que o ADR fecha.
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: warning\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('credential_guard.mode: block')), true)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

test('credential_guard_mode_downgrade: rules: off NÃO commitado ainda dispara (violation)', async () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: off\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('credential_guard.mode: block')), true)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

// TestRuleSeverity_ZeroDeltaParaRegrasNaoGuard (Go) — mesma prova em Node: ruleSeverity() para
// qualquer regra fora de CREDENTIAL_GUARD_ANCHORED_RULES continua resolvendo só pelo disco.
test('ruleSeverity: zero delta para regras não-guard (continuam só-disco)', async () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, '')
  writeFile(dir, 'trackfw.yaml', 'rules:\n  wip_limit: warning\n  adr_orphan: off\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    assert.equal(validator.ruleSeverity('wip_limit'), 'warning')
    assert.equal(validator.ruleSeverity('adr_orphan'), 'off')
    assert.equal(validator.ruleSeverity('filename_uniqueness'), 'error')
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

// TestCredentialGuardRuleSeverity_SemHead_CaiNoDisco (Go) — mesma prova em Node.
test('ruleSeverity: sem HEAD utilizável, credential-guard cai no disco puro', async () => {
  const dir = tmpDir() // nem sequer é git worktree
  writeFile(dir, 'trackfw.yaml', 'rules:\n  credential_guard_mode_downgrade: warning\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    assert.equal(validator.ruleSeverity('credential_guard_mode_downgrade'), 'warning')
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

// ---- ML-1B: invocações de git isoladas de redirecionamento por ambiente ----
// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1B.
// Mirrors internal/validator/validator_git_exec_test.go (Go).

const { cleanGitEnv } = require('../src/validator/git-exec')

function withEnv(overrides, fn) {
  const saved = {}
  for (const key of Object.keys(overrides)) {
    saved[key] = process.env[key]
    if (overrides[key] === undefined) {
      delete process.env[key]
    } else {
      process.env[key] = overrides[key]
    }
  }
  try {
    return fn()
  } finally {
    for (const key of Object.keys(saved)) {
      if (saved[key] === undefined) {
        delete process.env[key]
      } else {
        process.env[key] = saved[key]
      }
    }
  }
}

test('cleanGitEnv: remove apenas variáveis com prefixo GIT_', () => {
  withEnv({ GIT_DIR: '/tmp/whatever', GIT_CONFIG_COUNT: 'abc', MY_GIT_DIR_LOOKALIKE: 'kept' }, () => {
    const cleaned = cleanGitEnv()
    for (const key of Object.keys(cleaned)) {
      assert.equal(key.startsWith('GIT_'), false, `cleanGitEnv() não deveria manter ${key}`)
    }
    assert.equal(cleaned.MY_GIT_DIR_LOOKALIKE, 'kept')
  })
})

test('credential_guard_mode_downgrade: GIT_DIR/GIT_WORK_TREE redirecionados continuam detectando', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')

  const other = tmpDir()
  git(other, 'init')
  git(other, 'config', 'user.email', 'test@test.com')
  git(other, 'config', 'user.name', 'test')
  git(other, 'commit', '--allow-empty', '-m', 'init')

  withEnv({ GIT_DIR: path.join(other, '.git'), GIT_WORK_TREE: other }, () => {
    const msgs = validateCredentialGuardModeDowngrade(dir)
    assert.equal(msgs.length, 1, `GIT_DIR/GIT_WORK_TREE redirecionados NÃO deveriam silenciar a detecção, obteve: ${JSON.stringify(msgs)}`)
    assert.match(msgs[0], /credential_guard\.mode: block/)
  })
})

test('credential_guard_mode_downgrade: GIT_CONFIG_COUNT malformado continua detectando', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')
  writeFile(dir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')

  withEnv({ GIT_CONFIG_COUNT: 'abc' }, () => {
    const msgs = validateCredentialGuardModeDowngrade(dir)
    assert.equal(msgs.length, 1, `GIT_CONFIG_COUNT malformado NÃO deveria silenciar a detecção, obteve: ${JSON.stringify(msgs)}`)
    assert.match(msgs[0], /credential_guard\.mode: block/)
  })
})

test('credential_guard_mode_downgrade: GIT_CONFIG_COUNT malformado prova não-vacuidade (sem limpeza, git falha de verdade)', () => {
  const dir = tmpDir()
  initGitRepo(dir)
  commitTrackfwYAML(dir, 'credential_guard:\n  mode: block\n')

  assert.throws(() => {
    execFileSync('git', ['-C', dir, 'rev-parse', '--verify', 'HEAD'], {
      env: Object.assign({}, process.env, { GIT_CONFIG_COUNT: 'abc' }),
      stdio: ['ignore', 'pipe', 'pipe'],
    })
  }, /./, 'esperava que git falhasse com GIT_CONFIG_COUNT=abc herdado sem limpeza — não falhou, o fixture não prova nada')
})

test('isGitWorktree: linked worktree legítima continua funcionando (git worktree add real)', () => {
  const mainDir = tmpDir()
  initGitRepo(mainDir)
  commitTrackfwYAML(mainDir, 'credential_guard:\n  mode: block\n')

  const linkedDir = path.join(tmpDir(), 'linked')
  git(mainDir, 'worktree', 'add', '-b', 'feat/linked-worktree-test-node', linkedDir)

  // Sem downgrade — disco na worktree ainda resolve para block, idêntico ao HEAD.
  assert.deepEqual(validateCredentialGuardModeDowngrade(linkedDir), [])

  // Downgrade introduzido dentro da worktree — deve disparar normalmente.
  writeFile(linkedDir, 'trackfw.yaml', 'credential_guard:\n  mode: warn\n')
  const msgs = validateCredentialGuardModeDowngrade(linkedDir)
  assert.equal(msgs.length, 1)
  assert.match(msgs[0], /credential_guard\.mode: block/)
})

// ---- Paridade: CREDENTIAL_GUARD_SCRIPT_REFERENCE deve bater com o gerador real ----

test('CREDENTIAL_GUARD_SCRIPT_REFERENCE é byte-idêntico ao que generateCredentialGuardScript emite', () => {
  const { generateCredentialGuardScript } = require('../src/generators/hooks')
  const dir = tmpDir()
  generateCredentialGuardScript(dir)
  const emitted = fs.readFileSync(path.join(dir, 'scripts', 'trackfw-credential-guard.sh'), 'utf8')
  assert.equal(emitted, CREDENTIAL_GUARD_SCRIPT_REFERENCE)
})
