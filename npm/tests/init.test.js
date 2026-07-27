'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { generateClaudeCommands } = require('../src/generators/init')

test('SlashRoadmap command requires canonical frontmatter with selected REQ path', () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-slash-roadmap-'))
  const origCwd = process.cwd()
  try {
    process.chdir(tmpDir)
    generateClaudeCommands()
    const roadmapCommand = fs.readFileSync(path.join(tmpDir, '.claude', 'commands', 'trackfw', 'roadmap.md'), 'utf8')
    const required = [
      '```markdown\n   ---',
      'status: backlog',
      'date: <YYYY-MM-DD>',
      'req: "docs/req/<arquivo-selecionado>.md"',
      'squad: ""',
      '---\n\n   # Roadmap:',
      '> Created: <YYYY-MM-DD> | Status: backlog',
      'docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md',
      'Preencha `req:` com o caminho relativo completo da REQ selecionada',
      '### ML-1B — <título> (se independente de ML-1A)',
      '## Wave 2 — <nome> (depende de Wave 1)',
      '> Dependências: Wave 1 completa',
    ]
    for (const snippet of required) {
      assert.ok(roadmapCommand.includes(snippet), `roadmap.md should contain canonical snippet: ${snippet}`)
    }

    const versioned = fs.readFileSync(path.join(__dirname, '..', '..', '.claude', 'commands', 'trackfw', 'roadmap.md'), 'utf8')
    assert.equal(roadmapCommand, versioned, 'generated roadmap.md should match the versioned slash-command')
  } finally {
    process.chdir(origCwd)
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
})
