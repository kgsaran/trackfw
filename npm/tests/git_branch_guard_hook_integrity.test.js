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
  validateCredentialGuardHookResolvable,
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
          {"command": ${JSON.stringify(scriptAbsPath)}, "type": "command"}
        ]
      }
    ]
  }
}
`
}

// globalClaudeSettingsWithCommandNoType is globalClaudeSettingsWithCommand's ROADMAP-2026-08-17
// ML-4B counterpart: "type":"command" is deliberately OMITTED -- the exact malformed shape
// hades-tf's ML-4A barrier finding reproduced.
function globalClaudeSettingsWithCommandNoType(scriptAbsPath) {
  return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": ${JSON.stringify(scriptAbsPath)}}
        ]
      }
    ]
  }
}
`
}

// globalCursorHooksWithCommand: Cursor's schema never carries a "type" field at all -- this
// fixture is the non-regression control proving requiresCommandType=false for Cursor is not
// over-tightened by ML-4B.
function globalCursorHooksWithCommand(scriptAbsPath) {
  return `{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {"command": ${JSON.stringify(scriptAbsPath)}}
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

  // O bit de execução não é representável em NTFS: no Windows (stat.mode & 0o111) é 0
  // para todo arquivo, inclusive após fs.chmodSync(0o755). Este teste afirma que a
  // regra DISPARA quando o bit falta, o que só tem sentido onde o bit existe — fixamos
  // a plataforma em vez de pular, para a garantia continuar verificada em qualquer host.
  const restorePlatform = validator._setPlatformForTest('linux')
  try {
    const msgs = validateGitBranchGuardHookResolvable(dir)
    assert.equal(msgs.some(m => m.includes('not executable')), true)
  } finally {
    restorePlatform()
  }
})

// No Windows o bit de execução não é representável em NTFS, então "o script não é
// executável" é SEMPRE verdadeiro lá, e nenhuma ação do usuário torna a mensagem falsa —
// `trackfw update`, o remédio que ela prescreve, regenera o script com o mesmo modo.
// Mesmo precedente de _platform em src/integrations/scaffold_doctor.js (AC5).
test('git_branch_guard_hook_resolvable: no Windows não dispara pelo bit de execução', () => {
  const dir = tmpDir()
  writeFile(dir, '.claude/settings.json', gitBranchGuardEntryClaudeSettings('$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh'))
  const scriptPath = path.join(dir, 'scripts', 'trackfw-git-branch-guard.sh')
  fs.mkdirSync(path.dirname(scriptPath), { recursive: true })
  fs.writeFileSync(scriptPath, '#!/bin/sh\nexit 0\n', { mode: 0o644 }) // sem bit +x

  const restorePlatform = validator._setPlatformForTest('win32')
  try {
    const msgs = validateGitBranchGuardHookResolvable(dir)
    assert.equal(msgs.some(m => m.includes('not executable')), false)
    // O script EXISTE: a checagem de existência continua valendo no Windows.
    assert.equal(msgs.some(m => m.includes('does not exist')), false)
  } finally {
    restorePlatform()
  }
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

// ---------------------------------------------------------------------------
// ROADMAP-2026-08-17 ML-4B -- hades-tf ML-4A barrier finding reproduced: a
// config entry with the CORRECT command but MISSING "type":"command"
// (script present and íntegro) makes neither the dedup NOR this rule notice
// anything wrong -- "nenhum dos dois escopos protege, e tudo fica verde".
// ---------------------------------------------------------------------------

test('git_branch_guard_hook_resolvable: dispara com entrada de projeto sem "type":"command"', () => {
  const dir = tmpDir()
  writeFile(dir, '.claude/settings.json', `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"}
        ]
      }
    ]
  }
}
`)
  const scriptPath = path.join(dir, 'scripts', 'trackfw-git-branch-guard.sh')
  fs.mkdirSync(path.dirname(scriptPath), { recursive: true })
  fs.writeFileSync(scriptPath, GIT_BRANCH_GUARD_SCRIPT_REFERENCE, { mode: 0o755 })

  const msgs = validateGitBranchGuardHookResolvable(dir)
  assert.equal(msgs.some(m => m.includes('missing "type":"command"')), true)
  assert.equal(msgs.some(m => m.includes('.claude/settings.json')), true)
  assert.equal(msgs.some(m => m.includes('Claude Code')), true)
  assert.equal(msgs.some(m => m.includes('but the script does not exist')), false)
  assert.equal(msgs.some(m => m.includes('but the script is not executable')), false)
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

// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1H:
// espelha o teste equivalente de credential_guard_script_integrity (mesma ML) -- a comparação
// byte-a-byte nunca folds CRLF->LF, então um script CRLF-corrompido continua reportado como
// divergente.
test('git_branch_guard_script_integrity: script CRLF-corrompido dispara divergência', () => {
  const dir = tmpDir()
  writeFile(dir, 'scripts/trackfw-git-branch-guard.sh', GIT_BRANCH_GUARD_SCRIPT_REFERENCE.replace(/\n/g, '\r\n'))
  const msgs = validateGitBranchGuardScriptIntegrity(dir)
  assert.equal(msgs.length, 1, `esperava divergência para script CRLF-corrompido, obteve: ${JSON.stringify(msgs)}`)
  assert.match(msgs[0], /diverges from the template/)
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

// ROADMAP-2026-08-17 ML-4B -- hades-tf ML-4A barrier finding: script global presente e íntegro,
// mas a entrada de config está sem "type":"command" -- Claude Code nunca a executa em silêncio.
// Antes desta ML: silêncio (mesmo com script e caminho corretos). Depois: violação.
test('escopo global: registrado sem "type":"command" → dispara (não "does not exist")', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    fs.mkdirSync(path.dirname(globalScriptPath), { recursive: true })
    fs.writeFileSync(globalScriptPath, GIT_BRANCH_GUARD_SCRIPT_REFERENCE, { mode: 0o755 })
    fs.mkdirSync(path.join(home, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(home, '.claude', 'settings.json'), globalClaudeSettingsWithCommandNoType(globalScriptPath), 'utf8')

    const msgs = validateGitBranchGuardGlobalHookResolvable()
    assert.equal(msgs.some(m => m.includes('missing "type":"command"')), true)
    assert.equal(msgs.some(m => m.includes('global scope')), true)
    assert.equal(msgs.some(m => m.includes('Claude Code')), true)
    assert.equal(msgs.some(m => m.includes('trackfw update harness')), true)
    assert.equal(msgs.some(m => m.includes('but the script does not exist')), false)
    assert.equal(msgs.some(m => m.includes('but the script is not executable')), false)
  })
})

// Non-regression control: Cursor's schema never carries a "type" field, so its absence is
// normal, not malformed.
test('escopo global (Cursor): ausência de "type" é normal, não malformada → silêncio', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    fs.mkdirSync(path.dirname(globalScriptPath), { recursive: true })
    fs.writeFileSync(globalScriptPath, GIT_BRANCH_GUARD_SCRIPT_REFERENCE, { mode: 0o755 })
    fs.mkdirSync(path.join(home, '.cursor'), { recursive: true })
    fs.writeFileSync(path.join(home, '.cursor', 'hooks.json'), globalCursorHooksWithCommand(globalScriptPath), 'utf8')

    const msgs = validateGitBranchGuardGlobalHookResolvable()
    assert.deepEqual(msgs, [])
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

// Atualizado no ML-3B: a fiação global do git-branch-guard EXISTE desde a Wave 2 (ML-2A), mas
// este teste não a exercita — nenhum dos arquivos globais é escrito no fixture. Prova o caso
// "script global presente, nenhum config o referencia": deve permanecer em silêncio —
// hook_resolvable é condicionado à fiação por desenho. Mesma nota de ML-3B (Go).
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

// ---- escopo global: integridade dispara por EXISTÊNCIA do artefato, não por fiação
// (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-
// de-fiacao, ML-3A). Mirrors internal/validator/validator_git_branch_guard_test.go's
// TestGuardGlobalScriptIntegrity_* (Go). ----

test('escopo global git-branch-guard: dispara sem NENHUMA fiação — discriminante central do ML', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    fs.mkdirSync(path.dirname(globalScriptPath), { recursive: true })
    fs.writeFileSync(globalScriptPath, '#!/usr/bin/env bash\nexit 0\n', { mode: 0o755 })
    // Nenhum arquivo de config é escrito neste $HOME — nenhuma fiação existe.

    const msgs = validateGitBranchGuardGlobalScriptIntegrity()
    assert.equal(msgs.some(m => m.includes('diverges from the template')), true)
    assert.equal(msgs.some(m => m.includes(globalScriptPath)), true)
  })
})

test('escopo global: script nunca instalado (nem arquivo, nem fiação) → silêncio, credential e git-branch-guard', () => {
  withEnv({ HOME: tmpDir() }, () => {
    const gmsgs = validateGitBranchGuardGlobalScriptIntegrity()
    assert.deepEqual(gmsgs, [])
    const cmsgs = validateCredentialGuardGlobalScriptIntegrity()
    assert.deepEqual(cmsgs, [])
  })
})

test('escopo global: script referenciado por 2 configs (Claude + Codex) não duplica a mensagem', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const globalScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    fs.mkdirSync(path.dirname(globalScriptPath), { recursive: true })
    fs.writeFileSync(globalScriptPath, '#!/usr/bin/env bash\nexit 0\n', { mode: 0o755 })
    fs.mkdirSync(path.join(home, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(home, '.claude', 'settings.json'), globalClaudeSettingsWithCommand(globalScriptPath), 'utf8')
    fs.mkdirSync(path.join(home, '.codex'), { recursive: true })
    fs.writeFileSync(path.join(home, '.codex', 'hooks.json'), globalClaudeSettingsWithCommand(globalScriptPath), 'utf8')

    const msgs = validateGitBranchGuardGlobalScriptIntegrity()
    assert.equal(msgs.length, 1, `esperado exatamente 1 mensagem, obteve ${msgs.length}: ${JSON.stringify(msgs)}`)
  })
})

// ---- git_branch_guard_hook_resolvable / credential_guard_hook_resolvable (escopo GLOBAL,
// arquivo DEDICADO do Kiro — ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
// integridade-independente-de-fiacao, ML-3B). Mirrors internal/validator/
// validator_git_branch_guard_test.go's Test*Kiro* (Go). ----

// kiroGlobalGuardFixture monta o documento que os writers reais (harnessCredentialGuardTargetKiro/
// harnessGitBranchGuardTargetKiro no Go, e seus espelhos Node) escrevem —
// {"version":"v1","hooks":[{"name","description","trigger","matcher","action":{"type":"command",
// "command":scriptAbsPath}}, ...]} com dois hooks pre/post.
function kiroGlobalGuardFixture(hookNamePrefix, scriptAbsPath) {
  return `{
  "version": "v1",
  "hooks": [
    {
      "name": "${hookNamePrefix}-global-pre",
      "description": "global pre hook",
      "trigger": "PreToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": ${JSON.stringify(scriptAbsPath)}}
    },
    {
      "name": "${hookNamePrefix}-global-post",
      "description": "global post hook",
      "trigger": "PostToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": ${JSON.stringify(scriptAbsPath)}}
    }
  ]
}
`
}

// Discriminante central do ML-3B: antes dele, o arquivo dedicado do Kiro para git-branch-guard
// (~/.kiro/hooks/trackfw-git-branch-guard.json, escrito desde a Wave 2) nunca era lido por
// validateGitBranchGuardGlobalHookResolvable — um hook Kiro apontando para script ausente passava
// limpo. Agora deve violar.
test('escopo global git-branch-guard (Kiro, arquivo dedicado): script ausente → dispara', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const scriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    // scriptPath NÃO é criado — ausência proposital.
    writeFile(home, '.kiro/hooks/trackfw-git-branch-guard.json', kiroGlobalGuardFixture('trackfw-git-branch-guard', scriptPath))

    const msgs = validateGitBranchGuardGlobalHookResolvable()
    assert.equal(msgs.some(m => m.includes('does not exist')), true)
    assert.equal(msgs.some(m => m.includes('trackfw-git-branch-guard.json')), true)
    assert.equal(msgs.some(m => m.includes('Kiro')), true)
  })
})

test('escopo global git-branch-guard (Kiro, arquivo dedicado): script presente e executável → silêncio', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const scriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    fs.mkdirSync(path.dirname(scriptPath), { recursive: true })
    fs.writeFileSync(scriptPath, GIT_BRANCH_GUARD_SCRIPT_REFERENCE, { mode: 0o755 })
    writeFile(home, '.kiro/hooks/trackfw-git-branch-guard.json', kiroGlobalGuardFixture('trackfw-git-branch-guard', scriptPath))

    const msgs = validateGitBranchGuardGlobalHookResolvable()
    assert.deepEqual(msgs, [])
  })
})

// Não-regressão: com OS DOIS arquivos dedicados do Kiro presentes simultaneamente
// (trackfw-credential-guard.json E trackfw-git-branch-guard.json), cada um referenciando um
// script ausente distinto, cada regra deve reportar exatamente 1 violation — nunca 0 (regressão)
// nem 2+ (dupla contagem entre os dois arquivos/guards).
test('escopo global (Kiro, dois arquivos dedicados): não regride, não duplica', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    const credScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')
    const gbgScriptPath = path.join(home, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
    // Nenhum dos dois scripts é criado — ambos ausentes propositalmente.
    writeFile(home, '.kiro/hooks/trackfw-credential-guard.json', kiroGlobalGuardFixture('trackfw-credential-guard', credScriptPath))
    writeFile(home, '.kiro/hooks/trackfw-git-branch-guard.json', kiroGlobalGuardFixture('trackfw-git-branch-guard', gbgScriptPath))

    const credMsgs = validateCredentialGuardGlobalHookResolvable()
    assert.equal(credMsgs.length, 1, `esperado exatamente 1 violation (credential-guard do Kiro), obteve ${credMsgs.length}: ${JSON.stringify(credMsgs)}`)

    const gbgMsgs = validateGitBranchGuardGlobalHookResolvable()
    assert.equal(gbgMsgs.length, 1, `esperado exatamente 1 violation (git-branch-guard do Kiro), obteve ${gbgMsgs.length}: ${JSON.stringify(gbgMsgs)}`)
  })
})

test('escopo global git-branch-guard (Kiro): sem arquivo dedicado → silêncio', () => {
  withEnv({ HOME: tmpDir() }, () => {
    const msgs = validateGitBranchGuardGlobalHookResolvable()
    assert.deepEqual(msgs, [])
  })
})

// ---------------------------------------------------------------------------
// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1A:
// JSON inválido em um arquivo de guard deixa de ser um `continue` mudo (fail-open) e passa a ser
// violation em AMBAS as regras que compartilham a leitura desse arquivo (credential_guard_hook_
// resolvable e git_branch_guard_hook_resolvable) — o arquivo corrompido cega os dois controles ao
// mesmo tempo, não só um. Port de validator_git_branch_guard_test.go (Go).
// ---------------------------------------------------------------------------

// Afirma: .claude/settings.json com JSON inválido faz as DUAS regras de escopo de projeto
// reportarem violação — antes desta ML, as duas silenciavam (mesma função genérica
// validateGuardHookResolvable, mesmo `continue`).
test('JSON inválido (projeto): dispara em credential_guard_hook_resolvable e git_branch_guard_hook_resolvable', () => {
  const tmp = tmpDir()
  const origDir = process.cwd()
  writeFile(tmp, '.claude/settings.json', '{"hooks": {,}}')
  process.chdir(tmp)
  config.reset()
  try {
    const cgMsgs = validateCredentialGuardHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('is not valid JSON') && m.includes('.claude/settings.json')), true,
      'esperava violation de JSON inválido em credential_guard_hook_resolvable: ' + JSON.stringify(cgMsgs))

    const gbgMsgs = validateGitBranchGuardHookResolvable()
    assert.equal(gbgMsgs.some(m => m.includes('is not valid JSON') && m.includes('.claude/settings.json')), true,
      'esperava violation de JSON inválido em git_branch_guard_hook_resolvable: ' + JSON.stringify(gbgMsgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// Direção oposta da falsificação: JSON sintaticamente VÁLIDO, mesmo sem entrada de guard, nunca
// produz a mensagem "is not valid JSON".
test('JSON válido (projeto): não dispara a mensagem de JSON inválido', () => {
  const tmp = tmpDir()
  const origDir = process.cwd()
  writeFile(tmp, '.claude/settings.json', JSON.stringify({
    hooks: {
      PostToolUse: [{ matcher: 'AskUserQuestion', hooks: [{ command: 'scripts/trackfw-attention-cleanup.sh', type: 'command' }] }],
    },
  }))
  process.chdir(tmp)
  config.reset()
  try {
    const cgMsgs = validateCredentialGuardHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('is not valid JSON')), false, JSON.stringify(cgMsgs))

    const gbgMsgs = validateGitBranchGuardHookResolvable()
    assert.equal(gbgMsgs.some(m => m.includes('is not valid JSON')), false, JSON.stringify(gbgMsgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// Contraparte de escopo GLOBAL: ~/.claude/settings.json com JSON inválido dispara as duas regras
// (escopo global), com "global scope" e o remédio `trackfw update harness` (distinto do remédio de
// projeto, `trackfw update`).
test('JSON inválido (global): dispara em credential_guard_hook_resolvable e git_branch_guard_hook_resolvable', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    fs.mkdirSync(path.join(home, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(home, '.claude', 'settings.json'), '{"hooks": {,}}', 'utf8')

    const cgMsgs = validateCredentialGuardGlobalHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('is not valid JSON')), true, JSON.stringify(cgMsgs))
    assert.equal(cgMsgs.some(m => m.includes('global scope')), true, JSON.stringify(cgMsgs))
    assert.equal(cgMsgs.some(m => m.includes('trackfw update harness')), true, JSON.stringify(cgMsgs))

    const gbgMsgs = validateGitBranchGuardGlobalHookResolvable()
    assert.equal(gbgMsgs.some(m => m.includes('is not valid JSON')), true, JSON.stringify(gbgMsgs))
    assert.equal(gbgMsgs.some(m => m.includes('global scope')), true, JSON.stringify(gbgMsgs))
    assert.equal(gbgMsgs.some(m => m.includes('trackfw update harness')), true, JSON.stringify(gbgMsgs))
  })
})

// Direção oposta em escopo global: ~/.claude/settings.json válido, sem entrada de guard, não
// produz a mensagem "is not valid JSON".
test('JSON válido (global), sem entrada: não dispara a mensagem de JSON inválido', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    fs.mkdirSync(path.join(home, '.claude'), { recursive: true })
    fs.writeFileSync(path.join(home, '.claude', 'settings.json'), '{"hooks": {}}', 'utf8')

    const cgMsgs = validateCredentialGuardGlobalHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('is not valid JSON')), false, JSON.stringify(cgMsgs))

    const gbgMsgs = validateGitBranchGuardGlobalHookResolvable()
    assert.equal(gbgMsgs.some(m => m.includes('is not valid JSON')), false, JSON.stringify(gbgMsgs))
  })
})

// ---------------------------------------------------------------------------
// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1B:
// hades-tf's ML-1A barrier reproduced a SECOND silent `continue` in the same loop — a failure to
// READ the guard file (not just parse it: permission denied, a directory in place of the file).
// Before this ML, `if (e.code === 'ENOENT') continue \n continue` collapsed every non-ENOENT
// error into the same silent skip — `trackfw validate --json` exited 0 with no violation and no
// mention of the file. Port de validator_git_branch_guard_test.go (Go), mesmos nomes de mensagem.
// ---------------------------------------------------------------------------

// Afirma: .claude/settings.json que é na verdade um DIRETÓRIO (não arquivo) dispara "could not be
// read" nas duas regras de escopo de projeto. Fixture determinística (independe de uid/root) —
// hades-tf sinalizou que chmod 000 é um no-op quando o gate roda como root.
test('leitura falha (projeto, diretório no lugar do arquivo): dispara "could not be read" sem crash', () => {
  const tmp = tmpDir()
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude', 'settings.json'), { recursive: true })
  process.chdir(tmp)
  config.reset()
  try {
    const cgMsgs = validateCredentialGuardHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('could not be read') && m.includes('.claude/settings.json')), true,
      'esperava violation de leitura em credential_guard_hook_resolvable: ' + JSON.stringify(cgMsgs))

    const gbgMsgs = validateGitBranchGuardHookResolvable()
    assert.equal(gbgMsgs.some(m => m.includes('could not be read') && m.includes('.claude/settings.json')), true,
      'esperava violation de leitura em git_branch_guard_hook_resolvable: ' + JSON.stringify(gbgMsgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// Variante chmod 000 — pulada no Windows (bits POSIX não se aplicam) e quando o processo roda
// como root (chmod 000 não bloqueia leitura para root, geteuid() === 0).
test('leitura falha (projeto, permissão negada): dispara "could not be read" sem crash', { skip: process.platform === 'win32' || (typeof process.geteuid === 'function' && process.geteuid() === 0) }, () => {
  const tmp = tmpDir()
  const origDir = process.cwd()
  writeFile(tmp, '.claude/settings.json', '{"hooks": {}}')
  const settingsPath = path.join(tmp, '.claude', 'settings.json')
  fs.chmodSync(settingsPath, 0o000)
  process.chdir(tmp)
  config.reset()
  try {
    const cgMsgs = validateCredentialGuardHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('could not be read')), true, JSON.stringify(cgMsgs))
  } finally {
    fs.chmodSync(settingsPath, 0o644)
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1C.
// Mirrors internal/validator/regularfile_test.go's TestReadRegularFile_FIFO_NaoTravaEAcusaTipoErrado
// at the rule level: hades-tf's ML-1B barrier found `mkfifo .claude/settings.json` made
// fs.readFileSync block INDEFINITELY, in this exact rule pair. Node's own `timeout` test option
// turns a real hang into a fast, attributed test failure instead of the whole suite stalling.
test('leitura falha (projeto, FIFO no lugar do arquivo): dispara "could not be read" sem travar', { skip: process.platform === 'win32', timeout: 5000 }, () => {
  const tmp = tmpDir()
  const origDir = process.cwd()
  fs.mkdirSync(path.join(tmp, '.claude'), { recursive: true })
  const { execFileSync: run } = require('node:child_process')
  run('mkfifo', [path.join(tmp, '.claude', 'settings.json')])
  process.chdir(tmp)
  config.reset()
  try {
    const cgMsgs = validateCredentialGuardHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('could not be read') && m.includes('.claude/settings.json')), true,
      'esperava violation de leitura em credential_guard_hook_resolvable: ' + JSON.stringify(cgMsgs))

    const gbgMsgs = validateGitBranchGuardHookResolvable()
    assert.equal(gbgMsgs.some(m => m.includes('could not be read') && m.includes('.claude/settings.json')), true,
      'esperava violation de leitura em git_branch_guard_hook_resolvable: ' + JSON.stringify(gbgMsgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// Contraparte de escopo GLOBAL do teste de diretório acima.
test('leitura falha (global, diretório no lugar do arquivo): dispara "could not be read" sem crash', () => {
  const home = tmpDir()
  withEnv({ HOME: home }, () => {
    fs.mkdirSync(path.join(home, '.claude', 'settings.json'), { recursive: true })

    const cgMsgs = validateCredentialGuardGlobalHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('could not be read')), true, JSON.stringify(cgMsgs))
    assert.equal(cgMsgs.some(m => m.includes('global scope')), true, JSON.stringify(cgMsgs))
    assert.equal(cgMsgs.some(m => m.includes('trackfw update harness')), true, JSON.stringify(cgMsgs))

    const gbgMsgs = validateGitBranchGuardGlobalHookResolvable()
    assert.equal(gbgMsgs.some(m => m.includes('could not be read')), true, JSON.stringify(gbgMsgs))
    assert.equal(gbgMsgs.some(m => m.includes('global scope')), true, JSON.stringify(gbgMsgs))
    assert.equal(gbgMsgs.some(m => m.includes('trackfw update harness')), true, JSON.stringify(gbgMsgs))
  })
})

// ---------------------------------------------------------------------------
// ROADMAP-2026-09-06-...-config-ilegivel-deixa-de-ser-silencio, ML-1B: classe de decodificação,
// distinta de "invalid JSON" e de "could not be read" — hades-tf's ML-1A barrier found Python's
// strict `encoding="utf-8"` read crashing with an unstructured traceback on both fixtures below,
// while Node never crashed (fs.readFileSync(path, 'utf8') is a lossy decode). Port de
// validator_git_branch_guard_test.go (Go), mesmos nomes de mensagem — usados como o anchor para o
// gate de paridade cross-CLI verificar Python contra.
// ---------------------------------------------------------------------------

// Afirma: .claude/settings.json salvo inteiro como UTF-16 dispara "is not valid UTF-8" — DISTINTA
// de "is not valid JSON" — porque o motivo real é a codificação do arquivo, não a sintaxe JSON.
test('UTF-16 (projeto): dispara "is not valid UTF-8", não "is not valid JSON"', () => {
  const tmp = tmpDir()
  const origDir = process.cwd()
  const settingsPath = path.join(tmp, '.claude', 'settings.json')
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  // UTF-16LE com BOM, conteúdo `{"hooks":{}}`.
  fs.writeFileSync(settingsPath, Buffer.from('﻿{"hooks":{}}', 'utf16le'))
  process.chdir(tmp)
  config.reset()
  try {
    const cgMsgs = validateCredentialGuardHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('is not valid UTF-8')), true, JSON.stringify(cgMsgs))
    assert.equal(cgMsgs.some(m => m.includes('is not valid JSON')), false,
      'UTF-16 não deveria produzir a mensagem de JSON inválido: ' + JSON.stringify(cgMsgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

// Afirma o caso "ambíguo" que a barreira mediu: UM byte UTF-8 inválido dentro de um JSON por lo
// mais válido é silenciosamente coagido (Node's lossy decode) e não produz nenhuma violation.
test('1 byte UTF-8 inválido dentro de JSON válido (projeto): não dispara nenhuma violation', () => {
  const tmp = tmpDir()
  const origDir = process.cwd()
  const settingsPath = path.join(tmp, '.claude', 'settings.json')
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, Buffer.concat([
    Buffer.from('{"hooks":{"x":"', 'utf8'),
    Buffer.from([0xff]),
    Buffer.from('"}}', 'utf8'),
  ]))
  process.chdir(tmp)
  config.reset()
  try {
    const cgMsgs = validateCredentialGuardHookResolvable()
    assert.equal(cgMsgs.some(m => m.includes('is not valid UTF-8') || m.includes('is not valid JSON')), false,
      JSON.stringify(cgMsgs))
  } finally {
    process.chdir(origDir)
    config.reset()
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})
