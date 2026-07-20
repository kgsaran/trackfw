'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { trackfwRulesBlock, generateClaudeMD, scaffold } = require('../src/generators/init')

const EXPECTED_DIRECTIVE = 'Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura.'

test('trackfwRulesBlock includes mandatory global ADRs directive', () => {
  const block = trackfwRulesBlock()
  assert.ok(block.includes(EXPECTED_DIRECTIVE), `trackfwRulesBlock should contain global ADRs directive.\nGot:\n${block}`)
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

