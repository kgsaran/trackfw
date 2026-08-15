'use strict'

// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
// ML-2A. Mirrors internal/validator/validator_git_branch_guard_test.go (Go).
//
// Distinct filename from npm/tests/git_branch_guard.test.js (which already covers
// generateGitBranchGuardScript/injection and the shell script's own git-blocking behavior, from
// ROADMAP-2026-08-14) — this file covers the NEW `trackfw validate` rules
// (git_branch_guard_hook_resolvable / git_branch_guard_script_integrity) plus the GLOBAL-scope
// checks for both guards.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const config = require('../src/config')
const validator = require('../src/validator')
const {
  validateGitBranchGuardHookResolvable,
  validateGitBranchGuardScriptIntegrity,
  validateCredentialGuardGlobalHookResolvable,
  validateCredentialGuardGlobalScriptIntegrity,
  validateGitBranchGuardGlobalHookResolvable,
  validateGitBranchGuardGlobalScriptIntegrity,
  GIT_BRANCH_GUARD_SCRIPT_REFERENCE,
  CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE,
} = validator

function tmpDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gbg-integrity-'))
}

function writeFile(base, rel, content) {
  const full = path.join(base, rel)
  fs.mkdirSync(path.dirname(full), { recursive: true })
  fs.writeFileSync(full, content, 'utf8')
}

// gitBranchGuardEntryClaudeSettings monta um .claude/settings.json mínimo com uma entrada de
// git-branch-guard apontando para scriptCmd (valor bruto do campo "command") — mesmo padrão do
// equivalente Go.
function gitBranchGuardEntryClaudeSettings(scriptCmd) {
  return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "${scriptCmd}", "type": "command"}
        ]
      }
    ]
  }
}
`
}

// globalClaudeSettingsWithCommand monta ~/.claude/settings.json com uma entrada global
// PreToolUse[Bash] apontando para o caminho absoluto scriptAbsPath.
function globalClaudeSettingsWithCommand(scriptAbsPath) {
  return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "${scriptAbsPath}", "type": "command"}
        ]
      }
    ]
  }
}
`
}

function withEnv(overrides, fn) {
  const saved = {}
  for (const key of Object.keys(overrides)) {
    saved[key] = process.env[key]
    if (overrides[key] === undefined) delete process.env[key]
    else process.env[key] = overrides[key]
  }
  try {
    return fn()
  } finally {
    for (const key of Object.keys(saved)) {
      if (saved[key] === undefined) delete process.env[key]
      else process.env[key] = saved[key]
    }
  }
}

// ---- git_branch_guard_hook_resolvable (projeto) ----

test('git_branch_guard_hook_resolvable: dispara com script ausente', () => {
  const dir = tmpDir()
  writeFile(dir, '.claude/settings.json', gitBranchGuardEntryClaudeSettings('$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))
  // scripts/trackfw-git-branch-guard.sh NÃO é criado — ausência proposital.

  const msgs = validateGitBranchGuardHookResolvable(dir)
  assert.equal(msgs.some(m => m.includes('does not exist')), true)
  assert.equal(msgs.some(m => m.includes('.claude/settings.json')), true)
  assert.equal(msgs.some(m => m.includes('trackfw-git-branch-guard.sh')), true)
})

test('git_branch_guard_hook_resolvable: dispara com script não executável', () => {
  const dir = tmpDir()
  writeFile(dir, '.claude/settings.json', gitBranchGuardEntryClaudeSettings('$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))
  const scriptPath = path.join(dir, 'scripts', 'trackfw-git-branch-guard.sh')
  fs.mkdirSync(path.dirname(scriptPath), { recursive: true })
  fs.writeFileSync(scriptPath, '#!/bin/sh\nexit 0\n', { mode: 0o644 }) // sem bit +x

  const msgs = validateGitBranchGuardHookResolvable(dir)
  assert.equal(msgs.some(m => m.includes('not executable')), true)
})

test('git_branch_guard_hook_resolvable: não dispara sem entrada', () => {
  const dir = tmpDir()
  writeFile(dir, '.claude/settings.json', `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "AskUserQuestion", "hooks": [{"command": "scripts/trackfw-attention-cleanup.sh", "type": "command"}]}
    ]
  }
}
`)

  const msgs = validateGitBranchGuardHookResolvable(dir)
  assert.deepEqual(msgs, [])
})

test('git_branch_guard_hook_resolvable: não dispara com script presente e executável', () => {
  const dir = tmpDir()
  writeFile(dir, '.claude/settings.json', gitBranchGuardEntryClaudeSettings('$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))
  const scriptPath = path.join(dir, 'scripts', 'trackfw-git-branch-guard.sh')
  fs.mkdirSync(path.dirname(scriptPath), { recursive: true })
  fs.writeFileSync(scriptPath, GIT_BRANCH_GUARD_SCRIPT_REFERENCE, { mode: 0o755 })

  const msgs = validateGitBranchGuardHookResolvable(dir)
  assert.deepEqual(msgs, [])
})

// ---- git_branch_guard_script_integrity (projeto) ----

test('git_branch_guard_script_integrity: silêncio quando o script não existe', () => {
  const dir = tmpDir()
  // scripts/trackfw-git-branch-guard.sh NÃO existe — cobertura de ausência é
  // git_branch_guard_hook_resolvable, não esta regra.
  const msgs = validateGitBranchGuardScriptIntegrity(dir)
  assert.deepEqual(msgs, [])
})

test('git_branch_guard_script_integrity: silêncio quando o script é idêntico ao template', () => {
  const dir = tmpDir()
  writeFile(dir, 'scripts/trackfw-git-branch-guard.sh', GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
  const msgs = validateGitBranchGuardScriptIntegrity(dir)
  assert.deepEqual(msgs, [])
})

test('git_branch_guard_script_integrity: 1 byte alterado dispara violação', () => {
  const dir = tmpDir()
  const tampered = GIT_BRANCH_GUARD_SCRIPT_REFERENCE.slice(0, -1) + 'X'
  writeFile(dir, 'scripts/trackfw-git-branch-guard.sh', tampered)

  const msgs = validateGitBranchGuardScriptIntegrity(dir)
  assert.equal(msgs.length, 1)
  assert.match(msgs[0], /scripts\/trackfw-git-branch-guard\.sh/)
  assert.match(msgs[0], /diverges from the template/)
})

// severidade default "warning" (mesmo raciocínio de credential_guard_script_integrity: o script
// não carrega marcador de versão, não dá para distinguir drift legítimo de adulteração).
test('git_branch_guard_script_integrity: severidade default warning (pipeline completo)', async () => {
  const dir = tmpDir()
  writeFile(dir, 'scripts/trackfw-git-branch-guard.sh', '#!/usr/bin/env bash\nexit 0\n')
  const origCwd = process.cwd()
  process.chdir(dir)
  config.reset()
  try {
    const { violations, warnings } = await validator.validateUnfiltered()
    assert.equal(violations.some(v => v.includes('trackfw-git-branch-guard.sh')), false)
    assert.equal(warnings.some(w => w.includes('trackfw-git-branch-guard.sh')), true)
  } finally {
    process.chdir(origCwd)
    config.reset()
  }
})

// ---- Escopo global (credential-guard e git-branch-guard) ----

test('escopo global: sem entrada global, silêncio (credential-guard e git-branch-guard)', () => {
  withEnv({ HOME: tmpDir() }, () => {
    const msgs = validateCredentialGuardGlobalHookResolvable()
    assert.deepEqual(msgs, [])
    const gmsgs = validateGitBranchGuardGlobalHookResolvable()
    assert.deepEqual(gmsgs, [])
  })
})

// O gap principal que este ML fecha: hook de PROJETO ausente (dedup) + global instalado E
// íntegro → silêncio (dedup preservado).
test('escopo global: instalado e íntegro → silêncio', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
    fs.mkdirSync(path.dirname(globalScriptPath), { recursive: true })
    fs.writeFileSync(globalScriptPath, CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE, { mode: 0o755 })
    fs.mkdirSync(path.join(home, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(home, '.claude', 'settings.json'), globalClaudeSettingsWithCommand(globalScriptPath), 'utf8')

    const hookMsgs = validateCredentialGuardGlobalHookResolvable()
    assert.deepEqual(hookMsgs, [])

    const integrityMsgs = validateCredentialGuardGlobalScriptIntegrity()
    assert.deepEqual(integrityMsgs, [])
  })
})

// O gap principal: hook de PROJETO ausente + global REGISTRADO em ~/.claude/settings.json mas o
// script global não existe no disco → antes deste ML, `trackfw validate` silenciava; agora deve
// violar.
test('escopo global: registrado mas script ausente → dispara', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
    // Script global NÃO é criado — ausência proposital, apesar de estar registrado no settings.json.
    fs.mkdirSync(path.join(home, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(home, '.claude', 'settings.json'), globalClaudeSettingsWithCommand(globalScriptPath), 'utf8')

    const msgs = validateCredentialGuardGlobalHookResolvable()
    assert.equal(msgs.some(m => m.includes('does not exist')), true)
    assert.equal(msgs.some(m => m.includes('global scope')), true)
    assert.equal(msgs.some(m => m.includes('trackfw update harness')), true)
  })
})

// Mesmo gap acima, mas para o script global corrompido/desatualizado.
test('escopo global: registrado mas script corrompido → dispara', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
    fs.mkdirSync(path.dirname(globalScriptPath), { recursive: true })
    fs.writeFileSync(globalScriptPath, '#!/usr/bin/env bash\nexit 0\n', { mode: 0o755 })
    fs.mkdirSync(path.join(home, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(home, '.claude', 'settings.json'), globalClaudeSettingsWithCommand(globalScriptPath), 'utf8')

    const msgs = validateCredentialGuardGlobalScriptIntegrity()
    assert.equal(msgs.some(m => m.includes('diverges from the template')), true)
    assert.equal(msgs.some(m => m.includes('global scope')), true)
    assert.equal(msgs.some(m => m.includes('trackfw update harness')), true)
  })
})

// Divergência de design documentada: hoje nenhum wiring de git-branch-guard existe em nenhum
// config global (~/.claude/settings.json etc — só o script GLOBAL é gerado por
// `trackfw update harness`, nunca referenciado). Então, mesmo com o script global presente,
// nenhum arquivo de config global o referencia — o mecanismo genérico fica corretamente em
// silêncio até essa wiring existir. Mesma nota de ML-1A (Go).
test('escopo global git-branch-guard: sem wiring hoje → silêncio', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    fs.mkdirSync(path.dirname(globalScriptPath), { recursive: true })
    fs.writeFileSync(globalScriptPath, GIT_BRANCH_GUARD_SCRIPT_REFERENCE, { mode: 0o755 })
    // Nenhum ~/.claude/settings.json (ou equivalente) referencia trackfw-git-branch-guard.sh hoje.

    const msgs = validateGitBranchGuardGlobalHookResolvable()
    assert.deepEqual(msgs, [])
  })
})

// ---- Paridade: GIT_BRANCH_GUARD_SCRIPT_REFERENCE deve bater com o gerador real ----

test('GIT_BRANCH_GUARD_SCRIPT_REFERENCE é byte-idêntico ao que generateGitBranchGuardScript emite', () => {
  const { generateGitBranchGuardScript } = require('../src/generators/hooks')
  const dir = tmpDir()
  generateGitBranchGuardScript(dir)
  const emitted = fs.readFileSync(path.join(dir, 'scripts', 'trackfw-git-branch-guard.sh'), 'utf8')
  assert.equal(emitted, GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
})

test('CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE é byte-idêntico ao que generateGlobalCredentialGuardScript emite', () => {
  const { generateGlobalCredentialGuardScript } = require('../src/generators/hooks')
  const home = tmpDir()
  generateGlobalCredentialGuardScript(home)
  const emitted = fs.readFileSync(path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh'), 'utf8')
  assert.equal(emitted, CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE)
})
