'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { execBitRepresentavelPara, execBitNaoExercitado } = require('./exec-bit')
const {
  trackfwRulesBlock,
  generateClaudeMD,
  scaffold,
  generateClaudeCommands,
  generateClaudeCommandsForce,
  GLOBAL_ADRS_DIRECTIVE,
} = require('../src/generators/init')
const {
  injectClaudeHooks,
  injectCodexHooks,
  injectGeminiHooks,
  injectKiroHooks,
  injectCopilotHooks,
  injectCursorHooks,
  injectWindsurfHooks,
  injectHooksDetected,
} = require('../src/generators/hooks')

const EXPECTED_DIRECTIVE = GLOBAL_ADRS_DIRECTIVE

// Isolate the global credential-guard dedup check (ML-3A, npm/src/generators/hooks.js
// globalCredentialGuardInstalled*) from the real $HOME for every test in this file --
// none of them should depend on (or accidentally read) the developer's actual home dir.
let __origHome
test.beforeEach(() => {
  __origHome = process.env.HOME
  process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-home-iso-'))
})
test.afterEach(() => {
  process.env.HOME = __origHome
})

test('trackfwRulesBlock includes mandatory global ADRs directive', () => {
  const block = trackfwRulesBlock()
  assert.ok(block.includes(EXPECTED_DIRECTIVE), `trackfwRulesBlock should contain global ADRs directive.\nGot:\n${block}`)
})

test('trackfwRulesBlock includes mandatory 8 architecture directives', () => {
  const block = trackfwRulesBlock()
  assert.ok(block.includes('### Architecture Directives (mandatory)'), 'should contain Architecture Directives header')
  assert.ok(block.includes('- **3-layer separation:**'), 'should contain 3-layer separation')
  assert.ok(block.includes('- **No in-memory data:**'), 'should contain no in-memory data')
  assert.ok(block.includes('- **Auth from day 1:**'), 'should contain auth from day 1')
  assert.ok(block.includes('- **Docker + .env from day 1:**'), 'should contain docker + .env from day 1')
  assert.ok(block.includes('- **2-layer validation:**'), 'should contain 2-layer validation')
  assert.ok(block.includes('- **API-first:**'), 'should contain api-first')
  assert.ok(block.includes('- **Threat model waves:**'), 'should contain threat model waves')
  assert.ok(block.includes('- **Test coverage:**'), 'should contain test coverage')
  assert.ok(block.includes("Use `/trackfw:architect` to define stack"), 'should mention /trackfw:architect')
})

test('generateClaudeCommands and generateClaudeCommandsForce create architect.md command file', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-architect-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)

    // Test generateClaudeCommands
    generateClaudeCommands()
    const archPath = path.join(tmpDir, '.claude', 'commands', 'trackfw', 'architect.md')
    assert.ok(fs.existsSync(archPath), 'architect.md should exist after generateClaudeCommands()')

    const content = fs.readFileSync(archPath, 'utf8')
    assert.ok(content.includes('guia de arquitetura do trackfw'), 'architect.md should contain role description')
    assert.ok(content.includes('Passo 1 — Descoberta de Negócio'), 'architect.md should contain step 1')
    assert.ok(content.includes('Combo A — Protótipo Rápido'), 'architect.md should contain combo A')

    // Test generateClaudeCommandsForce
    generateClaudeCommandsForce(tmpDir)
    assert.ok(fs.existsSync(archPath), 'architect.md should exist after generateClaudeCommandsForce()')
    const forceContent = fs.readFileSync(archPath, 'utf8')
    assert.ok(forceContent.includes('guia de arquitetura do trackfw'), 'architect.md force content valid')
  } finally {
    process.chdir(origCwd)
  }
})


test('generateClaudeCommands and generateClaudeCommandsForce install the exact same set of slash commands', () => {
  const EXPECTED_COMMANDS = [
    'adr.md',
    'req.md',
    'validate.md',
    'status.md',
    'move.md',
    'roadmap.md',
    'implement.md',
    'barrier.md',
    'architect.md',
  ].sort()

  // Normal path (idempotent, cwd-relative)
  const normalDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cmds-normal-'))
  const origCwd = process.cwd()
  try {
    process.chdir(normalDir)
    generateClaudeCommands()
  } finally {
    process.chdir(origCwd)
  }
  const normalFiles = fs.readdirSync(path.join(normalDir, '.claude', 'commands', 'trackfw')).sort()

  // Forced path (overwrite, rootDir-relative)
  const forceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cmds-force-'))
  generateClaudeCommandsForce(forceDir)
  const forceFiles = fs.readdirSync(path.join(forceDir, '.claude', 'commands', 'trackfw')).sort()

  assert.deepEqual(normalFiles, EXPECTED_COMMANDS, 'normal path should install the canonical 9-command set')
  assert.deepEqual(forceFiles, EXPECTED_COMMANDS, 'force path should install the exact same 9-command set as the normal path')
  assert.deepEqual(normalFiles, forceFiles, 'normal and force paths must install identical command sets')

  // Content must be identical too, not just filenames (both draw from CLAUDE_COMMANDS).
  for (const filename of EXPECTED_COMMANDS) {
    const normalContent = fs.readFileSync(path.join(normalDir, '.claude', 'commands', 'trackfw', filename), 'utf8')
    const forceContent = fs.readFileSync(path.join(forceDir, '.claude', 'commands', 'trackfw', filename), 'utf8')
    assert.equal(normalContent, forceContent, `${filename} content should be identical between normal and force paths`)
  }
})

test('generateClaudeCommands honors an explicit rootDir argument (ML-5D)', () => {
  // Regression test: generateClaudeCommands(root) used to accept `root` but
  // silently ignore it, always writing relative to process.cwd() — the
  // forced twin (generateClaudeCommandsForce) already honored rootDir
  // correctly, and scaffold() calls generateClaudeCommands(root) expecting
  // the argument to be respected.
  const rootDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cmds-root-'))
  const cwdDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cmds-cwd-'))
  const origCwd = process.cwd()
  try {
    process.chdir(cwdDir)
    generateClaudeCommands(rootDir)

    const archInRoot = path.join(rootDir, '.claude', 'commands', 'trackfw', 'architect.md')
    assert.ok(fs.existsSync(archInRoot), 'slash commands should be written under the passed rootDir')

    const archInCwd = path.join(cwdDir, '.claude', 'commands', 'trackfw', 'architect.md')
    assert.ok(!fs.existsSync(archInCwd), 'slash commands must not also be written relative to process.cwd() when rootDir is given')
  } finally {
    process.chdir(origCwd)
  }
})

test('generateClaudeCommands does not overwrite an existing file; generateClaudeCommandsForce does', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cmds-overwrite-'))
  const origCwd = process.cwd()
  const cmdDir = path.join(tmpDir, '.claude', 'commands', 'trackfw')
  fs.mkdirSync(cmdDir, { recursive: true })
  const adrPath = path.join(cmdDir, 'adr.md')
  fs.writeFileSync(adrPath, 'custom user content', 'utf8')

  try {
    process.chdir(tmpDir)
    generateClaudeCommands()
    assert.equal(fs.readFileSync(adrPath, 'utf8'), 'custom user content', 'normal path must not overwrite an existing file')
  } finally {
    process.chdir(origCwd)
  }

  generateClaudeCommandsForce(tmpDir)
  assert.notEqual(fs.readFileSync(adrPath, 'utf8'), 'custom user content', 'force path must overwrite an existing file')
})

test('generateClaudeCommands creates barrier.md with the operational checklist', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-barrier-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    generateClaudeCommands()
    const barrierPath = path.join(tmpDir, '.claude', 'commands', 'trackfw', 'barrier.md')
    assert.ok(fs.existsSync(barrierPath), 'barrier.md should exist after generateClaudeCommands()')

    const content = fs.readFileSync(barrierPath, 'utf8')
    assert.ok(content.includes('trackfw_architect'), 'barrier.md should name trackfw_architect as the Git authority')
    assert.ok(content.includes('trackfw barrier <roadmap> --wave <n> --trust-local-gates --json'), 'barrier.md should invoke the deterministic core command with --trust-local-gates')
    assert.ok(content.includes('Todos os MLs da wave concluídos e marcados'), 'barrier.md should contain checklist item 1')
    assert.ok(content.includes('Agente code-quality reportou'), 'barrier.md should require the code-quality agent review')
    assert.ok(content.includes('Agente security reportou'), 'barrier.md should require the security agent review')
    assert.ok(content.includes('Resultado registrado antes de liberar a próxima wave'), 'barrier.md should contain checklist item 10')
  } finally {
    process.chdir(origCwd)
  }
})

test('generateClaudeMD includes mandatory global ADRs directive in CLAUDE.md', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gen-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    generateClaudeMD({ projectName: 'test-node-project' })
    const content = fs.readFileSync(path.join(tmpDir, 'CLAUDE.md'), 'utf8')
    assert.ok(content.includes(EXPECTED_DIRECTIVE), `CLAUDE.md should contain global ADRs directive.\nGot:\n${content}`)
  } finally {
    process.chdir(origCwd)
  }
})

test('generateClaudeMD includes all 9 harness sections', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-harness-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    generateClaudeMD({ projectName: 'test-harness-project' })
    const content = fs.readFileSync(path.join(tmpDir, 'CLAUDE.md'), 'utf8')

    const harnessSections = [
      '## Branch strategy',
      '## Definition of done',
      '## Requirement scope',
      '## State requirements',
      '## Roadmap format',
      '## When governance is not required',
      '## Production incidents',
      '## Iterative prototyping',
      '## Autopilot',
    ]
    for (const section of harnessSections) {
      assert.ok(content.includes(section), `CLAUDE.md should contain harness section: "${section}"`)
    }

    const harnessSnippets = [
      'One active branch at a time',
      'squash-merged',
      'Green build and tests do not close a microbatch',
      'explicit negative scope',
      '`blocked` requires a reason and an owner',
      'waves of microbatches',
      'closed list of exemptions',
      'This section takes precedence',
      'Inspect the live environment before proposing a fix',
      'disposable, isolated prototype',
      'Ask everything you need before starting',
    ]
    for (const snippet of harnessSnippets) {
      assert.ok(content.includes(snippet), `CLAUDE.md should contain harness snippet: "${snippet}"`)
    }

    assert.ok(
      content.includes('| `/trackfw:barrier` | Run the wave-release checklist before liberating the next wave |'),
      'CLAUDE.md should announce the /trackfw:barrier slash command in the table'
    )

    // Pre-existing sections must still be present
    const preExisting = [
      '## Governance chain',
      '## Agent rules (mandatory)',
      '## Slash commands (Claude Code)',
      '## CLI commands (terminal / CI)',
      '## Architecture Directives (mandatory)',
      '## Pre-commit checklist',
      '## Git hooks',
      '## CI gate',
    ]
    for (const section of preExisting) {
      assert.ok(content.includes(section), `CLAUDE.md should still contain pre-existing section: "${section}"`)
    }
  } finally {
    process.chdir(origCwd)
  }
})

test('scaffold generates CLAUDE.md with mandatory global ADRs directive', async () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-scaffold-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    await scaffold({ projectName: 'test-scaffold-project', frontend: 'none', backend: 'none' })
    const content = fs.readFileSync(path.join(tmpDir, 'CLAUDE.md'), 'utf8')
    assert.ok(content.includes(EXPECTED_DIRECTIVE), `Scaffolded CLAUDE.md should contain global ADRs directive.\nGot:\n${content}`)
  } finally {
    process.chdir(origCwd)
  }
})

test('scaffold generates attention scripts with execution permissions and expected headers', async () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-attention-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    await scaffold({ projectName: 'test-attention-project', frontend: 'none', backend: 'none' })
    const signalPath = path.join(tmpDir, 'scripts', 'trackfw-attention-signal.sh')
    const cleanupPath = path.join(tmpDir, 'scripts', 'trackfw-attention-cleanup.sh')
    // trackfw init (via scaffold()) must generate the credential guard script in the
    // same lifecycle as the attention scripts -- regression: the generator existed
    // but was never called by any real flow.
    const guardPath = path.join(tmpDir, 'scripts', 'trackfw-credential-guard.sh')

    assert.ok(fs.existsSync(signalPath), 'signal script should exist')
    assert.ok(fs.existsSync(cleanupPath), 'cleanup script should exist')
    assert.ok(fs.existsSync(guardPath), 'credential guard script should exist')

    const signalStat = fs.statSync(signalPath)
    const cleanupStat = fs.statSync(cleanupPath)
    const guardStat = fs.statSync(guardPath)

    // Guarda MEDIDA (npm/tests/exec-bit.js). Substitui `process.platform !== 'win32'`,
    // que suprimia em SILENCIO: agora a supressao NOMEIA o artefato nao verificado.
    for (const [artefato, st, rotulo] of [
      [signalPath, signalStat, 'signal script'],
      [cleanupPath, cleanupStat, 'cleanup script'],
      [guardPath, guardStat, 'credential guard script'],
    ]) {
      if (execBitRepresentavelPara(artefato)) {
        assert.ok((st.mode & 0o111) !== 0, `${rotulo} should be executable`)
      } else {
        execBitNaoExercitado(artefato)
      }
    }

    const signalContent = fs.readFileSync(signalPath, 'utf8')
    assert.ok(signalContent.includes('# trackfw attention signal — PreToolUse/BeforeTool hook'), 'signal header correct')

    const cleanupContent = fs.readFileSync(cleanupPath, 'utf8')
    assert.ok(cleanupContent.includes('# trackfw attention cleanup — PostToolUse/AfterTool hook'), 'cleanup header correct')
  } finally {
    process.chdir(origCwd)
  }
})

test('injectClaudeHooks creates and merges .claude/settings.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-claude-hooks-'))
  const settingsPath = path.join(tmpDir, '.claude', 'settings.json')

  // 1. Pre-existente com hooks customizados do usuário
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, JSON.stringify({
    hooks: {
      PreToolUse: [{ matcher: 'UserTool', hooks: [{ type: 'command', command: 'user-script.sh' }] }]
    }
  }, null, 2))

  // 2. Primeira injeção
  injectClaudeHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  // ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): PreToolUse/PostToolUse each gain two
  // new credential-guard entries (Read, Write|Edit) alongside the pre-existing Bash one.
  // ROADMAP-2026-08-14 ML-3B: PreToolUse's "Bash" matcher entry additionally gains a second
  // hook (git branch guard) alongside the credential-guard one -- same matcher entry, two
  // different commands in its `hooks` array.
  assert.equal(data.hooks.PreToolUse.length, 5)
  assert.equal(data.hooks.PreToolUse[0].matcher, 'UserTool')
  assert.equal(data.hooks.PreToolUse[1].matcher, 'AskUserQuestion')
  assert.equal(data.hooks.PreToolUse[1].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.PreToolUse[2].matcher, 'Bash')
  assert.equal(data.hooks.PreToolUse[2].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
  assert.equal(data.hooks.PreToolUse[2].hooks[1].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh')
  assert.equal(data.hooks.PreToolUse[3].matcher, 'Read')
  assert.equal(data.hooks.PreToolUse[3].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
  assert.equal(data.hooks.PreToolUse[4].matcher, 'Write|Edit')
  assert.equal(data.hooks.PreToolUse[4].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
  assert.equal(data.hooks.PostToolUse[0].matcher, 'AskUserQuestion')
  assert.equal(data.hooks.PostToolUse[0].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh')
  assert.equal(data.hooks.PostToolUse[1].matcher, 'Bash')
  assert.equal(data.hooks.PostToolUse[1].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
  assert.equal(data.hooks.PostToolUse[2].matcher, 'Read')
  assert.equal(data.hooks.PostToolUse[3].matcher, 'Write|Edit')

  // 3. Segunda injeção (idempotência)
  injectClaudeHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(data.hooks.PreToolUse.length, 5)
  assert.equal(data.hooks.PreToolUse[1].hooks.length, 1)
  assert.equal(data.hooks.PreToolUse[2].hooks.length, 2)
  assert.equal(data.hooks.PreToolUse[3].hooks.length, 1)
  assert.equal(data.hooks.PreToolUse[4].hooks.length, 1)
  assert.equal(data.hooks.PostToolUse.length, 4)
  assert.equal(data.hooks.PostToolUse[1].hooks.length, 1)
})

// Regression test for the bug reported in production (2026-08-09, CMDB project): the
// credential-guard command was a bare relative path, which Claude Code resolves against the
// hook's *dynamic* cwd (tracks `cd`s the agent runs), not the project root -- any Bash/Read/
// Write/Edit call after a `cd` into a subdirectory made the hook fail with "No such file or
// directory". Confirms re-injecting over a settings.json written by an older trackfw rewrites the
// legacy command in place instead of appending a second, still-broken entry alongside the fixed one.
test('injectClaudeHooks migrates a legacy relative-path credential-guard command in place', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-claude-hooks-migrate-'))
  const settingsPath = path.join(tmpDir, '.claude', 'settings.json')

  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, JSON.stringify({
    hooks: {
      PreToolUse: [
        { matcher: 'Bash', hooks: [{ type: 'command', command: 'scripts/trackfw-credential-guard.sh' }] },
        { matcher: 'Read', hooks: [{ type: 'command', command: 'scripts/trackfw-credential-guard.sh' }] }
      ],
      PostToolUse: [
        { matcher: 'Write|Edit', hooks: [{ type: 'command', command: 'scripts/trackfw-credential-guard.sh' }] }
      ]
    }
  }, null, 2))

  injectClaudeHooks(tmpDir)
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))

  const bashEntry = data.hooks.PreToolUse.find(e => e.matcher === 'Bash')
  const readEntry = data.hooks.PreToolUse.find(e => e.matcher === 'Read')
  const writeEditEntry = data.hooks.PostToolUse.find(e => e.matcher === 'Write|Edit')

  // ROADMAP-2026-08-14 ML-3B: the "Bash" matcher entry now also carries the git branch guard
  // hook (a different command, same matcher) alongside the migrated credential-guard one --
  // migration only rewrites the credential-guard command in place, it does not affect the
  // separate git-branch-guard hook merged afterward.
  assert.equal(bashEntry.hooks.length, 2, 'expected exactly 1 migrated credential-guard hook + 1 git-branch-guard hook, not old+new side by side')
  assert.equal(bashEntry.hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
  assert.equal(bashEntry.hooks[1].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh')
  assert.equal(readEntry.hooks.length, 1)
  assert.equal(readEntry.hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
  assert.equal(writeEditEntry.hooks.length, 1)
  assert.equal(writeEditEntry.hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
})

// ROADMAP-2026-08-11 ML-2A: same cwd-resolution bug class as the credential-guard fix above,
// applied to attention-signal/cleanup -- confirms re-injecting over a settings.json written by an
// older trackfw rewrites the legacy relative-path command in place instead of appending a second,
// still-cwd-fragile entry alongside the fixed one.
test('injectClaudeHooks migrates legacy relative-path attention-signal/cleanup commands in place', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-claude-hooks-migrate-attention-'))
  const settingsPath = path.join(tmpDir, '.claude', 'settings.json')

  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, JSON.stringify({
    hooks: {
      PreToolUse: [
        { matcher: 'AskUserQuestion', hooks: [{ type: 'command', command: 'scripts/trackfw-attention-signal.sh' }] }
      ],
      PostToolUse: [
        { matcher: 'AskUserQuestion', hooks: [{ type: 'command', command: 'scripts/trackfw-attention-cleanup.sh' }] }
      ]
    }
  }, null, 2))

  injectClaudeHooks(tmpDir)
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))

  const signalEntry = data.hooks.PreToolUse.find(e => e.matcher === 'AskUserQuestion')
  const cleanupEntry = data.hooks.PostToolUse.find(e => e.matcher === 'AskUserQuestion')

  assert.equal(signalEntry.hooks.length, 1, 'expected exactly 1 hook after migration, not old+new side by side')
  assert.equal(signalEntry.hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh')
  assert.equal(cleanupEntry.hooks.length, 1)
  assert.equal(cleanupEntry.hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh')
})

// ROADMAP-2026-08-11 ML-3A: Codex has no project-root env var, so the command is wrapped in
// literal double quotes around `$(git rev-parse --show-toplevel)` per ADR-2026-08-11 -- matches
// CODEX_ROOT/codexSignalCmd/codexGuardCmd/codexCleanupCmd in src/generators/hooks.js and
// internal/generators/agentfiles.go.
const CODEX_SIGNAL_CMD = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-signal.sh"'
const CODEX_CLEANUP_CMD = '"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-cleanup.sh"'
const CODEX_GUARD_CMD = '"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"'

// ROADMAP-2026-08-11 ML-4A: Gemini documents and uses $GEMINI_PROJECT_DIR in 100% of its
// official hook command examples (ADR-2026-08-11, "Gemini CLI — alterar, por argumento de
// assimetria") -- matches SIGNAL_CMD_GEMINI/CLEANUP_CMD_GEMINI/GUARD_CMD_GEMINI in
// src/generators/hooks.js and geminiSignalCmd/geminiCleanupCmd/geminiGuardCmd in
// internal/generators/agentfiles.go.
const GEMINI_SIGNAL_CMD = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-signal.sh'
const GEMINI_CLEANUP_CMD = '$GEMINI_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh'
const GEMINI_GUARD_CMD = '$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh'

test('injectCodexHooks creates and merges .codex/hooks.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-codex-hooks-'))
  const hooksPath = path.join(tmpDir, '.codex', 'hooks.json')

  injectCodexHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.hooks.PermissionRequest[0].matcher, '.*')
  assert.equal(data.hooks.PermissionRequest[0].hooks[0].command, CODEX_SIGNAL_CMD)
  assert.equal(data.hooks.PreToolUse[0].matcher, 'Bash')
  assert.equal(data.hooks.PreToolUse[0].hooks[0].command, CODEX_GUARD_CMD)
  // ROADMAP-2026-08-14 ML-3B: the "Bash" matcher entry also gains the git branch guard hook
  // (a different command, same matcher), appended after the credential-guard one.
  assert.equal(data.hooks.PreToolUse[0].hooks[1].command, '"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"')
  // ADR-2026-08-06 emenda 7: Codex has no dedicated read matcher (documented limitation) --
  // only apply_patch (write/edit) is added alongside Bash.
  assert.equal(data.hooks.PreToolUse[1].matcher, 'apply_patch')
  assert.equal(data.hooks.PreToolUse[1].hooks[0].command, CODEX_GUARD_CMD)
  assert.equal(data.hooks.PostToolUse[0].matcher, '.*')
  assert.equal(data.hooks.PostToolUse[0].hooks[0].command, CODEX_CLEANUP_CMD)
  assert.equal(data.hooks.PostToolUse[1].matcher, 'Bash')
  assert.equal(data.hooks.PostToolUse[1].hooks[0].command, CODEX_GUARD_CMD)
  assert.equal(data.hooks.PostToolUse[2].matcher, 'apply_patch')

  // Idempotência
  injectCodexHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.hooks.PermissionRequest.length, 1)
  assert.equal(data.hooks.PermissionRequest[0].hooks.length, 1)
  assert.equal(data.hooks.PreToolUse.length, 2)
  assert.equal(data.hooks.PreToolUse[0].hooks.length, 2)
  assert.equal(data.hooks.PreToolUse[1].hooks.length, 1)
  assert.equal(data.hooks.PostToolUse.length, 3)
  assert.equal(data.hooks.PostToolUse[1].hooks.length, 1)
})

test('injectCodexHooks preserves pre-existing PreToolUse Bash entry (merge, not overwrite)', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-codex-hooks-merge-'))
  const hooksPath = path.join(tmpDir, '.codex', 'hooks.json')

  fs.mkdirSync(path.dirname(hooksPath), { recursive: true })
  fs.writeFileSync(hooksPath, JSON.stringify({
    hooks: {
      PreToolUse: [{ matcher: 'Bash', hooks: [{ type: 'command', command: 'scripts/other.sh' }] }]
    }
  }, null, 2))

  injectCodexHooks(tmpDir)
  injectCodexHooks(tmpDir)

  const data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  // ADR-2026-08-06 emenda 7: apply_patch is now added alongside Bash.
  assert.equal(data.hooks.PreToolUse.length, 2)
  assert.equal(data.hooks.PreToolUse[0].matcher, 'Bash')
  const commands = data.hooks.PreToolUse[0].hooks.map(h => h.command)
  assert.ok(commands.includes('scripts/other.sh'), 'existing Bash hook lost during merge')
  assert.ok(commands.includes(CODEX_GUARD_CMD), 'credential-guard hook missing after merge')
  assert.equal(data.hooks.PreToolUse[1].matcher, 'apply_patch')
})

// ML-1A migration wiring, now exercised as a genuine migration (ROADMAP-2026-08-11 ML-3A):
// migrateHookCommand is called before mergeClaudeHookArray for every trackfw-owned matcher in
// injectCodexHooks. This fixture pre-populates every trackfw-owned matcher with the pre-ML-3A
// relative-path command, exactly as an older trackfw run would have left it, and asserts the
// injector rewrites each entry to the new $(git rev-parse --show-toplevel)-pinned command in place
// instead of appending a second, still-cwd-fragile entry alongside it.
test('injectCodexHooks migration wiring rewrites in place, does not duplicate', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-codex-hooks-migrate-'))
  const hooksPath = path.join(tmpDir, '.codex', 'hooks.json')

  const mk = (matcher, command) => ({ matcher, hooks: [{ type: 'command', command }] })
  fs.mkdirSync(path.dirname(hooksPath), { recursive: true })
  fs.writeFileSync(hooksPath, JSON.stringify({
    hooks: {
      PermissionRequest: [mk('.*', 'scripts/trackfw-attention-signal.sh')],
      PreToolUse: [
        mk('Bash', 'scripts/trackfw-credential-guard.sh'),
        mk('apply_patch', 'scripts/trackfw-credential-guard.sh'),
      ],
      PostToolUse: [
        mk('.*', 'scripts/trackfw-attention-cleanup.sh'),
        mk('Bash', 'scripts/trackfw-credential-guard.sh'),
        mk('apply_patch', 'scripts/trackfw-credential-guard.sh'),
      ],
    },
  }, null, 2))

  injectCodexHooks(tmpDir)

  const data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  const checkOne = (event, matcher, command) => {
    const entries = data.hooks[event].filter(e => e.matcher === matcher)
    assert.equal(entries.length, 1, `${event}[${matcher}]: expected exactly 1 matcher entry (no duplicate)`)
    assert.equal(entries[0].hooks.length, 1, `${event}[${matcher}]: expected exactly 1 hook`)
    assert.equal(entries[0].hooks[0].command, command, `${event}[${matcher}]: unexpected command`)
  }
  checkOne('PermissionRequest', '.*', CODEX_SIGNAL_CMD)
  // "Bash" now also carries the (non-migrated, freshly-added) git branch guard hook alongside
  // the migrated credential-guard one (ROADMAP-2026-08-14 ML-3B) -- checked separately below
  // instead of via checkOne, which asserts exactly 1 hook.
  const preBash = data.hooks.PreToolUse.find(e => e.matcher === 'Bash')
  assert.equal(preBash.hooks.length, 2, 'PreToolUse[Bash]: expected migrated credential-guard hook + git-branch-guard hook')
  assert.equal(preBash.hooks[0].command, CODEX_GUARD_CMD)
  assert.equal(preBash.hooks[1].command, '"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"')
  checkOne('PreToolUse', 'apply_patch', CODEX_GUARD_CMD)
  checkOne('PostToolUse', '.*', CODEX_CLEANUP_CMD)
  checkOne('PostToolUse', 'Bash', CODEX_GUARD_CMD)
  checkOne('PostToolUse', 'apply_patch', CODEX_GUARD_CMD)
})

test('injectGeminiHooks creates and merges .gemini/settings.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gemini-hooks-'))
  const settingsPath = path.join(tmpDir, '.gemini', 'settings.json')

  injectGeminiHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(data.hooks.Notification[0].matcher, 'ToolPermission')
  assert.equal(data.hooks.Notification[0].hooks[0].command, GEMINI_SIGNAL_CMD)
  assert.equal(data.hooks.AfterTool[0].matcher, '*')
  assert.equal(data.hooks.AfterTool[0].hooks[0].command, GEMINI_CLEANUP_CMD)
  assert.equal(data.hooks.BeforeTool[0].matcher, 'run_shell_command')
  assert.equal(data.hooks.BeforeTool[0].hooks[0].command, GEMINI_GUARD_CMD)
  const afterToolGuard = data.hooks.AfterTool.find(e => e.matcher === 'run_shell_command')
  assert.ok(afterToolGuard, 'AfterTool[run_shell_command] credential-guard entry missing')
  assert.equal(afterToolGuard.hooks[0].command, GEMINI_GUARD_CMD)

  // ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): read_file|read_many_files and
  // write_file|replace credential-guard entries alongside run_shell_command.
  const beforeToolRead = data.hooks.BeforeTool.find(e => e.matcher === 'read_file|read_many_files')
  assert.ok(beforeToolRead, 'BeforeTool[read_file|read_many_files] credential-guard entry missing')
  assert.equal(beforeToolRead.hooks[0].command, GEMINI_GUARD_CMD)
  const beforeToolWrite = data.hooks.BeforeTool.find(e => e.matcher === 'write_file|replace')
  assert.ok(beforeToolWrite, 'BeforeTool[write_file|replace] credential-guard entry missing')
  const afterToolRead = data.hooks.AfterTool.find(e => e.matcher === 'read_file|read_many_files')
  assert.ok(afterToolRead, 'AfterTool[read_file|read_many_files] credential-guard entry missing')
  const afterToolWrite = data.hooks.AfterTool.find(e => e.matcher === 'write_file|replace')
  assert.ok(afterToolWrite, 'AfterTool[write_file|replace] credential-guard entry missing')

  // Idempotência
  injectGeminiHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(data.hooks.Notification.length, 1)
  assert.equal(data.hooks.Notification[0].hooks.length, 1)
  assert.equal(data.hooks.BeforeTool.length, 3)
  assert.equal(data.hooks.AfterTool.length, 4)
})

test('injectGeminiHooks preserves an existing BeforeTool[run_shell_command] entry when merging', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gemini-hooks-merge-'))
  const settingsPath = path.join(tmpDir, '.gemini', 'settings.json')

  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, JSON.stringify({
    hooks: {
      BeforeTool: [
        { matcher: 'run_shell_command', hooks: [{ type: 'command', command: 'scripts/other.sh' }] },
      ],
    },
  }, null, 2))

  injectGeminiHooks(tmpDir)
  injectGeminiHooks(tmpDir)

  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  // ADR-2026-08-06 emenda 7: read_file|read_many_files and write_file|replace entries are added
  // alongside run_shell_command.
  assert.equal(data.hooks.BeforeTool.length, 3)
  const commands = data.hooks.BeforeTool[0].hooks.map(h => h.command)
  assert.ok(commands.includes('scripts/other.sh'), 'existing BeforeTool hook lost during merge')
  assert.ok(commands.includes(GEMINI_GUARD_CMD), 'credential-guard hook missing after merge')
})

// ML-1A migration wiring, now exercised as a genuine migration (ROADMAP-2026-08-11 ML-4A):
// this fixture is an old settings.json written by a pre-ML-4A trackfw (relative-path commands),
// and injectGeminiHooks must rewrite each entry in place to $GEMINI_PROJECT_DIR/... form rather
// than duplicating it.
test('injectGeminiHooks migration wiring rewrites in place, does not duplicate', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gemini-hooks-migrate-'))
  const settingsPath = path.join(tmpDir, '.gemini', 'settings.json')

  const mk = (matcher, command) => ({ matcher, hooks: [{ type: 'command', command }] })
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true })
  fs.writeFileSync(settingsPath, JSON.stringify({
    hooks: {
      Notification: [mk('ToolPermission', 'scripts/trackfw-attention-signal.sh')],
      BeforeTool: [
        mk('run_shell_command', 'scripts/trackfw-credential-guard.sh'),
        mk('read_file|read_many_files', 'scripts/trackfw-credential-guard.sh'),
        mk('write_file|replace', 'scripts/trackfw-credential-guard.sh'),
      ],
      AfterTool: [
        mk('*', 'scripts/trackfw-attention-cleanup.sh'),
        mk('run_shell_command', 'scripts/trackfw-credential-guard.sh'),
        mk('read_file|read_many_files', 'scripts/trackfw-credential-guard.sh'),
        mk('write_file|replace', 'scripts/trackfw-credential-guard.sh'),
      ],
    },
  }, null, 2))

  injectGeminiHooks(tmpDir)

  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  const checkOne = (event, matcher, command) => {
    const entries = data.hooks[event].filter(e => e.matcher === matcher)
    assert.equal(entries.length, 1, `${event}[${matcher}]: expected exactly 1 matcher entry (no duplicate)`)
    assert.equal(entries[0].hooks.length, 1, `${event}[${matcher}]: expected exactly 1 hook`)
    assert.equal(entries[0].hooks[0].command, command, `${event}[${matcher}]: unexpected command`)
  }
  checkOne('Notification', 'ToolPermission', GEMINI_SIGNAL_CMD)
  // "BeforeTool"'s "run_shell_command" matcher now also carries the (non-migrated,
  // freshly-added) git branch guard hook alongside the migrated credential-guard one
  // (ROADMAP-2026-08-14 ML-3B) -- checked separately below instead of via checkOne, which
  // asserts exactly 1 hook.
  const beforeShell = data.hooks.BeforeTool.find(e => e.matcher === 'run_shell_command')
  assert.equal(beforeShell.hooks.length, 2, 'BeforeTool[run_shell_command]: expected migrated credential-guard hook + git-branch-guard hook')
  assert.equal(beforeShell.hooks[0].command, GEMINI_GUARD_CMD)
  assert.equal(beforeShell.hooks[1].command, '$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh')
  checkOne('BeforeTool', 'read_file|read_many_files', GEMINI_GUARD_CMD)
  checkOne('BeforeTool', 'write_file|replace', GEMINI_GUARD_CMD)
  checkOne('AfterTool', '*', GEMINI_CLEANUP_CMD)
  checkOne('AfterTool', 'run_shell_command', GEMINI_GUARD_CMD)
  checkOne('AfterTool', 'read_file|read_many_files', GEMINI_GUARD_CMD)
  checkOne('AfterTool', 'write_file|replace', GEMINI_GUARD_CMD)
})

test('injectKiroHooks creates .kiro/hooks/trackfw-attention.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-kiro-hooks-'))
  const hookPath = path.join(tmpDir, '.kiro', 'hooks', 'trackfw-attention.json')

  injectKiroHooks(tmpDir)
  let data1 = JSON.parse(fs.readFileSync(hookPath, 'utf8'))
  assert.equal(data1.version, 'v1')
  // ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): +4 credential-guard entries
  // (read-pre/read-post/write-pre/write-post) alongside the pre-existing shell pre/post.
  assert.equal(data1.hooks.length, 8)
  assert.equal(data1.hooks[0].trigger, 'PreToolUse')
  assert.equal(data1.hooks[0].event, undefined, 'legacy "event" field must not be emitted')
  assert.equal(data1.hooks[0].action.command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data1.hooks[1].trigger, 'PostToolUse')
  assert.equal(data1.hooks[1].action.command, 'scripts/trackfw-attention-cleanup.sh')

  const guardPre = data1.hooks.find(h => h.name === 'trackfw-credential-guard-pre')
  assert.ok(guardPre, 'missing trackfw-credential-guard-pre hook')
  assert.equal(guardPre.trigger, 'PreToolUse')
  assert.equal(guardPre.matcher, 'shell')
  assert.equal(guardPre.action.command, 'scripts/trackfw-credential-guard.sh')

  const guardPost = data1.hooks.find(h => h.name === 'trackfw-credential-guard-post')
  assert.ok(guardPost, 'missing trackfw-credential-guard-post hook')
  assert.equal(guardPost.trigger, 'PostToolUse')
  assert.equal(guardPost.matcher, 'shell')
  assert.equal(guardPost.action.command, 'scripts/trackfw-credential-guard.sh')

  const guardReadPre = data1.hooks.find(h => h.name === 'trackfw-credential-guard-read-pre')
  assert.ok(guardReadPre, 'missing trackfw-credential-guard-read-pre hook')
  assert.equal(guardReadPre.matcher, 'read')
  const guardReadPost = data1.hooks.find(h => h.name === 'trackfw-credential-guard-read-post')
  assert.ok(guardReadPost, 'missing trackfw-credential-guard-read-post hook')
  assert.equal(guardReadPost.matcher, 'read')
  const guardWritePre = data1.hooks.find(h => h.name === 'trackfw-credential-guard-write-pre')
  assert.ok(guardWritePre, 'missing trackfw-credential-guard-write-pre hook')
  assert.equal(guardWritePre.matcher, 'write')
  const guardWritePost = data1.hooks.find(h => h.name === 'trackfw-credential-guard-write-post')
  assert.ok(guardWritePost, 'missing trackfw-credential-guard-write-post hook')
  assert.equal(guardWritePost.matcher, 'write')

  for (const h of data1.hooks) {
    assert.notEqual(typeof h.matcher, 'object', `hook ${h.name} uses object matcher, expected regex string`)
  }

  injectKiroHooks(tmpDir)
  let data2 = JSON.parse(fs.readFileSync(hookPath, 'utf8'))
  assert.deepStrictEqual(data1, data2)
})

test('injectCopilotHooks creates .github/hooks/trackfw-attention.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-copilot-hooks-'))
  const hookPath = path.join(tmpDir, '.github', 'hooks', 'trackfw-attention.json')

  injectCopilotHooks(tmpDir)
  let data1 = JSON.parse(fs.readFileSync(hookPath, 'utf8'))
  assert.equal(data1.version, 1)
  // ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): +2 credential-guard entries (view,
  // create|edit) alongside the pre-existing bash one, in each of preToolUse/postToolUse.
  // ROADMAP-2026-08-14 ML-3B: preToolUse also gains +1 git-branch-guard entry (matcher
  // "bash"), unconditional -- postToolUse is untouched (no PostToolUse wiring for this
  // guard, see GBG_CMD_CLAUDE comment in hooks.js).
  assert.equal(data1.hooks.preToolUse.length, 5)
  assert.equal(data1.hooks.postToolUse.length, 4)

  const findByBash = (arr, bash) => arr.find(e => e.bash === bash)
  const findByMatcher = (arr, bash, matcher) => arr.find(e => e.bash === bash && e.matcher === matcher)

  const signal = findByBash(data1.hooks.preToolUse, 'scripts/trackfw-attention-signal.sh')
  assert.ok(signal, 'preToolUse missing attention-signal entry')
  assert.equal(signal.matcher, undefined)

  const guardPre = findByMatcher(data1.hooks.preToolUse, 'scripts/trackfw-credential-guard.sh', 'bash')
  assert.ok(guardPre, 'preToolUse missing credential-guard bash entry')

  const guardPreView = findByMatcher(data1.hooks.preToolUse, 'scripts/trackfw-credential-guard.sh', 'view')
  assert.ok(guardPreView, 'preToolUse missing credential-guard view entry')

  const guardPreEdit = findByMatcher(data1.hooks.preToolUse, 'scripts/trackfw-credential-guard.sh', 'create|edit')
  assert.ok(guardPreEdit, 'preToolUse missing credential-guard create|edit entry')

  const gitGuardPre = findByMatcher(data1.hooks.preToolUse, 'scripts/trackfw-git-branch-guard.sh', 'bash')
  assert.ok(gitGuardPre, 'preToolUse missing git-branch-guard bash entry')

  const cleanup = findByBash(data1.hooks.postToolUse, 'scripts/trackfw-attention-cleanup.sh')
  assert.ok(cleanup, 'postToolUse missing attention-cleanup entry')

  const guardPost = findByMatcher(data1.hooks.postToolUse, 'scripts/trackfw-credential-guard.sh', 'bash')
  assert.ok(guardPost, 'postToolUse missing credential-guard bash entry')

  const guardPostView = findByMatcher(data1.hooks.postToolUse, 'scripts/trackfw-credential-guard.sh', 'view')
  assert.ok(guardPostView, 'postToolUse missing credential-guard view entry')

  const guardPostEdit = findByMatcher(data1.hooks.postToolUse, 'scripts/trackfw-credential-guard.sh', 'create|edit')
  assert.ok(guardPostEdit, 'postToolUse missing credential-guard create|edit entry')

  injectCopilotHooks(tmpDir)
  let data2 = JSON.parse(fs.readFileSync(hookPath, 'utf8'))
  assert.deepStrictEqual(data1, data2)
})

test('injectCursorHooks creates and merges .cursor/hooks.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cursor-hooks-'))
  const hooksPath = path.join(tmpDir, '.cursor', 'hooks.json')

  injectCursorHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.preToolUse, undefined, 'legacy top-level preToolUse must not be written')
  assert.equal(data.postToolUse, undefined, 'legacy top-level postToolUse must not be written')

  assert.equal(data.version, 1)
  // ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): +2 credential-guard entries (matcher
  // Read, matcher Write) added to the generic preToolUse/postToolUse events alongside the
  // unfiltered attention-signal/cleanup entry already there.
  assert.equal(data.hooks.preToolUse.length, 3)
  assert.equal(data.hooks.preToolUse[0].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.postToolUse.length, 3)
  assert.equal(data.hooks.postToolUse[0].command, 'scripts/trackfw-attention-cleanup.sh')
  // ROADMAP-2026-08-14 ML-3B: beforeShellExecution also gains the git branch guard entry
  // (a separate plain-command entry, no matcher) alongside the credential-guard one.
  assert.equal(data.hooks.beforeShellExecution.length, 2)
  assert.equal(data.hooks.beforeShellExecution[0].command, 'scripts/trackfw-credential-guard.sh')
  assert.equal(data.hooks.beforeShellExecution[1].command, 'scripts/trackfw-git-branch-guard.sh')
  assert.equal(data.hooks.afterShellExecution.length, 1)
  assert.equal(data.hooks.afterShellExecution[0].command, 'scripts/trackfw-credential-guard.sh')

  const preGuardRead = data.hooks.preToolUse.find(e => e.command === 'scripts/trackfw-credential-guard.sh' && e.matcher === 'Read')
  assert.ok(preGuardRead, 'preToolUse missing credential-guard Read entry')
  const preGuardWrite = data.hooks.preToolUse.find(e => e.command === 'scripts/trackfw-credential-guard.sh' && e.matcher === 'Write')
  assert.ok(preGuardWrite, 'preToolUse missing credential-guard Write entry')
  const postGuardRead = data.hooks.postToolUse.find(e => e.command === 'scripts/trackfw-credential-guard.sh' && e.matcher === 'Read')
  assert.ok(postGuardRead, 'postToolUse missing credential-guard Read entry')
  const postGuardWrite = data.hooks.postToolUse.find(e => e.command === 'scripts/trackfw-credential-guard.sh' && e.matcher === 'Write')
  assert.ok(postGuardWrite, 'postToolUse missing credential-guard Write entry')

  // Idempotência
  injectCursorHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.hooks.preToolUse.length, 3)
  assert.equal(data.hooks.postToolUse.length, 3)
  assert.equal(data.hooks.beforeShellExecution.length, 2)
  assert.equal(data.hooks.afterShellExecution.length, 1)
})

test('injectCursorHooks migrates legacy top-level preToolUse/postToolUse, preserving unrelated entries', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cursor-hooks-legacy-'))
  const hooksPath = path.join(tmpDir, '.cursor', 'hooks.json')

  // Pré-existente: schema legado (top-level), inclui uma entrada não-trackfw
  fs.mkdirSync(path.dirname(hooksPath), { recursive: true })
  fs.writeFileSync(hooksPath, JSON.stringify({
    preToolUse: [{ command: 'scripts/trackfw-attention-signal.sh' }, { command: 'user-pre.sh' }],
    postToolUse: [{ command: 'scripts/trackfw-attention-cleanup.sh' }]
  }, null, 2))

  injectCursorHooks(tmpDir)
  const data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))

  // Entrada trackfw removida do nível raiz; entrada não relacionada permanece intacta.
  assert.equal(data.preToolUse.length, 1)
  assert.equal(data.preToolUse[0].command, 'user-pre.sh')
  // postToolUse só tinha a entrada trackfw -> chave inteira removida.
  assert.equal(data.postToolUse, undefined)

  // Entradas migradas para o local real (aninhado sob hooks).
  assert.equal(data.hooks.preToolUse[0].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.postToolUse[0].command, 'scripts/trackfw-attention-cleanup.sh')
})

test('injectCursorHooks preserves a pre-existing top-level version field', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cursor-hooks-version-'))
  const hooksPath = path.join(tmpDir, '.cursor', 'hooks.json')

  fs.mkdirSync(path.dirname(hooksPath), { recursive: true })
  fs.writeFileSync(hooksPath, JSON.stringify({ version: 2, hooks: {} }, null, 2))

  injectCursorHooks(tmpDir)
  const data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.version, 2)
})

test('injectWindsurfHooks updates .windsurfrules', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-windsurf-hooks-'))
  const rulesPath = path.join(tmpDir, '.windsurfrules')

  injectWindsurfHooks(tmpDir)
  const content = fs.readFileSync(rulesPath, 'utf8')
  assert.ok(content.includes('Windsurf users:'), 'should contain Windsurf instruction')
  assert.ok(content.includes('.trackfw-attention.json'), 'should mention attention JSON')
})

test('injectHooksDetected auto-detects all 7 CLIs and injects hooks', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-all-hooks-'))

  // Criar marcadores dos 7 CLIs
  fs.mkdirSync(path.join(tmpDir, '.claude'), { recursive: true })
  fs.mkdirSync(path.join(tmpDir, '.codex'), { recursive: true })
  fs.mkdirSync(path.join(tmpDir, '.gemini'), { recursive: true })
  fs.mkdirSync(path.join(tmpDir, '.kiro'), { recursive: true })
  fs.mkdirSync(path.join(tmpDir, '.github', 'hooks'), { recursive: true })
  fs.mkdirSync(path.join(tmpDir, '.cursor'), { recursive: true })
  fs.writeFileSync(path.join(tmpDir, '.windsurfrules'), '', 'utf8')

  injectHooksDetected(tmpDir)

  assert.ok(fs.existsSync(path.join(tmpDir, '.claude', 'settings.json')), 'claude hooks generated')
  assert.ok(fs.existsSync(path.join(tmpDir, '.codex', 'hooks.json')), 'codex hooks generated')
  assert.ok(fs.existsSync(path.join(tmpDir, '.gemini', 'settings.json')), 'gemini hooks generated')
  assert.ok(fs.existsSync(path.join(tmpDir, '.kiro', 'hooks', 'trackfw-attention.json')), 'kiro hooks generated')
  assert.ok(fs.existsSync(path.join(tmpDir, '.github', 'hooks', 'trackfw-attention.json')), 'copilot hooks generated')
  assert.ok(fs.existsSync(path.join(tmpDir, '.cursor', 'hooks.json')), 'cursor hooks generated')

  const windsurfContent = fs.readFileSync(path.join(tmpDir, '.windsurfrules'), 'utf8')
  assert.ok(windsurfContent.includes('Windsurf users:'), 'windsurf rules injected')
})

test('trackfw update command injects attention hooks and scripts idempotently preserving user settings', async () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-update-hooks-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'hooks: none\nci: none\n', 'utf8')

    // Marcadores para Claude e Cursor com hook customizado no Claude
    const claudeDir = path.join(tmpDir, '.claude')
    fs.mkdirSync(claudeDir, { recursive: true })
    fs.writeFileSync(path.join(claudeDir, 'settings.json'), JSON.stringify({
      hooks: {
        PreToolUse: [{ matcher: 'CustomTool', hooks: [{ type: 'command', command: 'custom.sh' }] }]
      }
    }, null, 2), 'utf8')

    const cursorDir = path.join(tmpDir, '.cursor')
    fs.mkdirSync(cursorDir, { recursive: true })

    fs.writeFileSync(path.join(tmpDir, '.windsurfrules'), '# Existing rules\n', 'utf8')

    // Invocação do update command
    const updateCmd = require('../src/commands/update')
    await updateCmd.parseAsync(['node', 'update'])

    // Validar criação dos scripts de atenção
    const signalPath = path.join(tmpDir, 'scripts', 'trackfw-attention-signal.sh')
    const cleanupPath = path.join(tmpDir, 'scripts', 'trackfw-attention-cleanup.sh')
    const guardPath = path.join(tmpDir, 'scripts', 'trackfw-credential-guard.sh')
    assert.ok(fs.existsSync(signalPath), 'signal script should be generated by update')
    assert.ok(fs.existsSync(cleanupPath), 'cleanup script should be generated by update')
    assert.ok(fs.existsSync(guardPath), 'credential guard script should be generated by update')

    // Validar injeção preservando custom tool
    const claudeData = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf8'))
    assert.equal(claudeData.hooks.PreToolUse[0].matcher, 'CustomTool')
    assert.equal(claudeData.hooks.PreToolUse[1].matcher, 'AskUserQuestion')
    assert.equal(claudeData.hooks.PreToolUse[2].matcher, 'Bash')
    assert.equal(claudeData.hooks.PreToolUse[2].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
    // ADR-2026-08-06 emenda 7 (ROADMAP-2026-08-08 Wave 2): Read/Write|Edit credential-guard entries.
    assert.equal(claudeData.hooks.PreToolUse[3].matcher, 'Read')
    assert.equal(claudeData.hooks.PreToolUse[4].matcher, 'Write|Edit')
    assert.equal(claudeData.hooks.PostToolUse[0].matcher, 'AskUserQuestion')
    assert.equal(claudeData.hooks.PostToolUse[1].matcher, 'Bash')
    assert.equal(claudeData.hooks.PostToolUse[1].hooks[0].command, '$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh')
    assert.equal(claudeData.hooks.PostToolUse[2].matcher, 'Read')
    assert.equal(claudeData.hooks.PostToolUse[3].matcher, 'Write|Edit')

    // Validar Cursor
    const cursorData = JSON.parse(fs.readFileSync(path.join(cursorDir, 'hooks.json'), 'utf8'))
    assert.equal(cursorData.hooks.preToolUse[0].command, 'scripts/trackfw-attention-signal.sh')

    // Validar Windsurf
    const windsurfRules = fs.readFileSync(path.join(tmpDir, '.windsurfrules'), 'utf8')
    assert.ok(windsurfRules.includes('Windsurf users:'))

    // Re-executar para testar idempotência
    await updateCmd.parseAsync(['node', 'update'])

    const claudeDataSecond = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf8'))
    assert.equal(claudeDataSecond.hooks.PreToolUse.length, 5)
    assert.equal(claudeDataSecond.hooks.PostToolUse.length, 4)
  } finally {
    process.chdir(origCwd)
  }
})

test('trackfw update backfills the credential guard script for a pre-existing project (upgrade scenario)', async () => {
  // Simulates a project that already ran `trackfw init`/`update` BEFORE this REQ:
  // scripts/trackfw-attention-signal.sh exists, scripts/trackfw-credential-guard.sh
  // does not yet. `trackfw update` must generate the missing script without
  // breaking anything already there.
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-update-upgrade-test-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'hooks: none\nci: none\n', 'utf8')

    const scriptsDir = path.join(tmpDir, 'scripts')
    fs.mkdirSync(scriptsDir, { recursive: true })
    const signalPath = path.join(scriptsDir, 'trackfw-attention-signal.sh')
    fs.writeFileSync(signalPath, '#!/usr/bin/env bash\necho "old signal script"\n', { encoding: 'utf8', mode: 0o755 })

    const guardPath = path.join(scriptsDir, 'trackfw-credential-guard.sh')
    assert.ok(!fs.existsSync(guardPath), 'test precondition: credential guard should not exist yet')

    const updateCmd = require('../src/commands/update')
    await updateCmd.parseAsync(['node', 'update'])

    assert.ok(fs.existsSync(guardPath), 'update should have generated the missing credential guard script')
    if (execBitRepresentavelPara(guardPath)) {
      assert.ok((fs.statSync(guardPath).mode & 0o111) !== 0, 'credential guard script should be executable')
    } else {
      execBitNaoExercitado(guardPath)
    }
    assert.ok(fs.existsSync(signalPath), 'pre-existing attention signal script should not be removed')
  } finally {
    process.chdir(origCwd)
  }
})

test('attention scripts are resilient to missing roadmap_dir in YAML and execute successfully', async () => {
  const { generateAttentionScripts } = require('../src/generators/hooks')
  const { execSync } = require('node:child_process')
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-att-resilient-'))

  // Criar trackfw.yaml SEM roadmap_dir
  fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'frontend: react\nbackend: go\n', 'utf8')
  generateAttentionScripts({}, tmpDir)

  const signalScript = path.join(tmpDir, 'scripts', 'trackfw-attention-signal.sh')
  const cleanupScript = path.join(tmpDir, 'scripts', 'trackfw-attention-cleanup.sh')

  // Executar signalScript com input JSON contendo aspas, backslash e newlines (C1 + C5)
  const payload = JSON.stringify({
    tool_name: 'test_tool',
    tool_input: { question: 'Need help with path\\file.txt and "quotes"\nSecond line' }
  })

  execSync(`"${signalScript}"`, {
    cwd: tmpDir,
    input: payload,
    stdio: ['pipe', 'pipe', 'pipe']
  })

  const attFile = path.join(tmpDir, 'docs', 'roadmaps', '.trackfw-attention.json')
  assert.ok(fs.existsSync(attFile), 'attention json file should be created in default docs/roadmaps')

  const writtenContent = fs.readFileSync(attFile, 'utf8')
  const parsed = JSON.parse(writtenContent)
  assert.equal(parsed.tool, 'test_tool')
  assert.ok(parsed.message.includes('Need help with path\\file.txt and "quotes"'), 'message escapes quotes and backslashes properly')
  assert.ok(!writtenContent.includes('\nSecond line'), 'newlines stripped from message body in JSON')

  // Executar cleanupScript
  execSync(`"${cleanupScript}"`, { cwd: tmpDir, stdio: ['ignore', 'pipe', 'pipe'] })
  assert.ok(!fs.existsSync(attFile), 'attention json file should be removed after cleanup script')
})

test('attention scripts normalize/contain ROADMAP_DIR against path traversal', async () => {
  const { generateAttentionScripts } = require('../src/generators/hooks')
  const { execSync } = require('node:child_process')
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-att-traversal-'))

  // Criar trackfw.yaml COM path traversal em roadmap_dir
  fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'roadmap_dir: ../../../tmp/traversal\n', 'utf8')
  generateAttentionScripts({}, tmpDir)

  const signalScript = path.join(tmpDir, 'scripts', 'trackfw-attention-signal.sh')
  const payload = JSON.stringify({ tool_name: 'test_tool', tool_input: { question: 'Hello' } })

  execSync(`"${signalScript}"`, { cwd: tmpDir, input: payload, stdio: ['pipe', 'pipe', 'pipe'] })

  // Garantir que não escreveu fora do CWD
  const defaultAttFile = path.join(tmpDir, 'docs', 'roadmaps', '.trackfw-attention.json')
  assert.ok(fs.existsSync(defaultAttFile), 'traversal attempt should fallback to docs/roadmaps')
})

test('attention scripts tolerate roadmap_dir without spaces and inline comments, and sanitize U+0000-U+001F control chars', async () => {
  const { generateAttentionScripts } = require('../src/generators/hooks')
  const { execSync } = require('node:child_process')
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-att-tolerant-'))

  // Criar trackfw.yaml sem espaço após ':' e com comentário inline
  fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'roadmap_dir:docs/custom_roadmaps # inline comment\n', 'utf8')
  generateAttentionScripts({}, tmpDir)

  const signalScript = path.join(tmpDir, 'scripts', 'trackfw-attention-signal.sh')

  // Payload com caracteres de controle U+0000..U+001F (ex: bell \x07, tab \x09, newline \x0A, carriage return \x0D)
  const payload = JSON.stringify({
    tool_name: 'test_tool\x07\x09',
    tool_input: { question: 'Question with control chars: \x07\x1b[31mRed\x1b[0m and \r\nnewlines' }
  })

  execSync(`"${signalScript}"`, { cwd: tmpDir, input: payload, stdio: ['pipe', 'pipe', 'pipe'] })

  const customAttFile = path.join(tmpDir, 'docs', 'custom_roadmaps', '.trackfw-attention.json')
  assert.ok(fs.existsSync(customAttFile), 'attention json file should be created in parsed custom roadmap_dir')

  const writtenContent = fs.readFileSync(customAttFile, 'utf8')
  const parsed = JSON.parse(writtenContent)
  assert.equal(parsed.tool, 'test_tool')
  assert.equal(parsed.message, 'Question with control chars: [31mRed[0m and newlines')
})

test('attention signal script falls back to python3 when jq is not in PATH', async () => {
  const { generateAttentionScripts } = require('../src/generators/hooks')
  const { execSync } = require('node:child_process')
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-att-nojq-'))

  fs.writeFileSync(path.join(tmpDir, 'trackfw.yaml'), 'roadmap_dir: docs/roadmaps\n', 'utf8')
  generateAttentionScripts({}, tmpDir)

  const signalScript = path.join(tmpDir, 'scripts', 'trackfw-attention-signal.sh')

  const payload = JSON.stringify({
    tool_name: 'no_jq_tool',
    tool_input: { question: 'Fallback python3 question' }
  })

  const fakeBinDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-fakebin-'))
  const pathDirs = (process.env.PATH || '').split(path.delimiter)
  for (const dir of pathDirs) {
    try {
      if (!fs.existsSync(dir)) continue
      const files = fs.readdirSync(dir)
      for (const file of files) {
        if (file === 'jq' || file === 'jq.exe') continue
        const src = path.join(dir, file)
        const dst = path.join(fakeBinDir, file)
        if (!fs.existsSync(dst)) {
          try { fs.symlinkSync(src, dst) } catch (_) {}
        }
      }
    } catch (_) {}
  }

  execSync(`"${signalScript}"`, {
    cwd: tmpDir,
    input: payload,
    env: { ...process.env, PATH: fakeBinDir },
    stdio: ['pipe', 'pipe', 'pipe']
  })

  const attFile = path.join(tmpDir, 'docs', 'roadmaps', '.trackfw-attention.json')
  assert.ok(fs.existsSync(attFile), 'attention json file should be created using python3 fallback')

  const writtenContent = fs.readFileSync(attFile, 'utf8')
  const parsed = JSON.parse(writtenContent)
  assert.equal(parsed.tool, 'no_jq_tool')
  assert.equal(parsed.message, 'Fallback python3 question')
})



