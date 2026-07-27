'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { generateClaudeCommands } = require('../src/generators/init')

function testSkipStrict(name, fn) {
  test(name, () => {
    try {
      fn()
    } catch (err) {
      console.log(`↷ [xfail esperado] ${name}: ${err.message}`)
      return
    }
    assert.fail(`XPASS inesperado — remover testSkipStrict após correção: ${name}`)
  })
}

testSkipStrict('SlashRoadmap command requires canonical frontmatter with selected REQ path', () => {
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
      'docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md',
    ]
    for (const snippet of required) {
      assert.ok(roadmapCommand.includes(snippet), `roadmap.md should contain canonical snippet: ${snippet}`)
    }
  } finally {
    process.chdir(origCwd)
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
})
