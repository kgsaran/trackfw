'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
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
  assert.ok(block.includes('- **Security wave:**'), 'should contain security wave')
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

    assert.ok(fs.existsSync(signalPath), 'signal script should exist')
    assert.ok(fs.existsSync(cleanupPath), 'cleanup script should exist')

    const signalStat = fs.statSync(signalPath)
    const cleanupStat = fs.statSync(cleanupPath)

    if (process.platform !== 'win32') {
      assert.ok((signalStat.mode & 0o111) !== 0, 'signal script should be executable')
      assert.ok((cleanupStat.mode & 0o111) !== 0, 'cleanup script should be executable')
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
  assert.equal(data.hooks.PreToolUse.length, 2)
  assert.equal(data.hooks.PreToolUse[0].matcher, 'UserTool')
  assert.equal(data.hooks.PreToolUse[1].matcher, 'AskUserQuestion')
  assert.equal(data.hooks.PreToolUse[1].hooks[0].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.PostToolUse[0].matcher, 'AskUserQuestion')
  assert.equal(data.hooks.PostToolUse[0].hooks[0].command, 'scripts/trackfw-attention-cleanup.sh')

  // 3. Segunda injeção (idempotência)
  injectClaudeHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(data.hooks.PreToolUse.length, 2)
  assert.equal(data.hooks.PreToolUse[1].hooks.length, 1)
})

test('injectCodexHooks creates and merges .codex/hooks.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-codex-hooks-'))
  const hooksPath = path.join(tmpDir, '.codex', 'hooks.json')

  injectCodexHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.hooks.PermissionRequest[0].matcher, '.*')
  assert.equal(data.hooks.PermissionRequest[0].hooks[0].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.PostToolUse[0].matcher, '.*')
  assert.equal(data.hooks.PostToolUse[0].hooks[0].command, 'scripts/trackfw-attention-cleanup.sh')

  // Idempotência
  injectCodexHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.hooks.PermissionRequest.length, 1)
  assert.equal(data.hooks.PermissionRequest[0].hooks.length, 1)
})

test('injectGeminiHooks creates and merges .gemini/settings.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gemini-hooks-'))
  const settingsPath = path.join(tmpDir, '.gemini', 'settings.json')

  injectGeminiHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(data.hooks.Notification[0].matcher, 'ToolPermission')
  assert.equal(data.hooks.Notification[0].hooks[0].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks.AfterTool[0].matcher, '*')
  assert.equal(data.hooks.AfterTool[0].hooks[0].command, 'scripts/trackfw-attention-cleanup.sh')

  // Idempotência
  injectGeminiHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
  assert.equal(data.hooks.Notification.length, 1)
  assert.equal(data.hooks.Notification[0].hooks.length, 1)
})

test('injectKiroHooks creates .kiro/hooks/trackfw-attention.json', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-kiro-hooks-'))
  const hookPath = path.join(tmpDir, '.kiro', 'hooks', 'trackfw-attention.json')

  injectKiroHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(hookPath, 'utf8'))
  assert.equal(data.hooks.length, 2)
  assert.equal(data.hooks[0].event, 'PreToolUse')
  assert.equal(data.hooks[0].action.command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks[1].event, 'PostToolUse')
  assert.equal(data.hooks[1].action.command, 'scripts/trackfw-attention-cleanup.sh')
})

test('injectCopilotHooks creates .github/hooks/trackfw-attention.json', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-copilot-hooks-'))
  const hookPath = path.join(tmpDir, '.github', 'hooks', 'trackfw-attention.json')

  injectCopilotHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(hookPath, 'utf8'))
  assert.equal(data.hooks.length, 2)
  assert.equal(data.hooks[0].event, 'preToolUse')
  assert.equal(data.hooks[0].run, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.hooks[1].event, 'postToolUse')
  assert.equal(data.hooks[1].run, 'scripts/trackfw-attention-cleanup.sh')
})

test('injectCursorHooks creates and merges .cursor/hooks.json idempotently', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-cursor-hooks-'))
  const hooksPath = path.join(tmpDir, '.cursor', 'hooks.json')

  // Pré-existente
  fs.mkdirSync(path.dirname(hooksPath), { recursive: true })
  fs.writeFileSync(hooksPath, JSON.stringify({
    preToolUse: [{ command: 'user-pre.sh' }]
  }, null, 2))

  injectCursorHooks(tmpDir)
  let data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.preToolUse.length, 2)
  assert.equal(data.preToolUse[0].command, 'user-pre.sh')
  assert.equal(data.preToolUse[1].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(data.postToolUse[0].command, 'scripts/trackfw-attention-cleanup.sh')

  // Idempotência
  injectCursorHooks(tmpDir)
  data = JSON.parse(fs.readFileSync(hooksPath, 'utf8'))
  assert.equal(data.preToolUse.length, 2)
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
    assert.ok(fs.existsSync(signalPath), 'signal script should be generated by update')
    assert.ok(fs.existsSync(cleanupPath), 'cleanup script should be generated by update')

    // Validar injeção preservando custom tool
    const claudeData = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf8'))
    assert.equal(claudeData.hooks.PreToolUse[0].matcher, 'CustomTool')
    assert.equal(claudeData.hooks.PreToolUse[1].matcher, 'AskUserQuestion')
    assert.equal(claudeData.hooks.PostToolUse[0].matcher, 'AskUserQuestion')

    // Validar Cursor
    const cursorData = JSON.parse(fs.readFileSync(path.join(cursorDir, 'hooks.json'), 'utf8'))
    assert.equal(cursorData.preToolUse[0].command, 'scripts/trackfw-attention-signal.sh')

    // Validar Windsurf
    const windsurfRules = fs.readFileSync(path.join(tmpDir, '.windsurfrules'), 'utf8')
    assert.ok(windsurfRules.includes('Windsurf users:'))

    // Re-executar para testar idempotência
    await updateCmd.parseAsync(['node', 'update'])

    const claudeDataSecond = JSON.parse(fs.readFileSync(path.join(claudeDir, 'settings.json'), 'utf8'))
    assert.equal(claudeDataSecond.hooks.PreToolUse.length, 2)
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


