const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const {
  installAgents,
  installGemini,
  installCursor,
  installCopilot,
  installWindsurf,
  installAmazonQ,
} = require('../src/generators/init')

async function assertIdempotentToolInstall(toolName, installer, expectedFiles) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), `trackfw-${toolName}-`))
  await installer(root)
  const snapshots = expectedFiles.map(file => fs.readFileSync(path.join(root, file), 'utf8'))
  await installer(root)
  expectedFiles.forEach((file, index) => {
    assert.equal(fs.existsSync(path.join(root, file)), true, file)
    assert.equal(fs.readFileSync(path.join(root, file), 'utf8'), snapshots[index], `${toolName}:${file}`)
  })
}

for (const fixture of [
  ['claude', installAgents, ['.claude/agents/trackfw-architect.md', '.claude/skills/trackfw-governance/SKILL.md']],
  ['gemini', installGemini, ['.gemini/agents/trackfw-architect.md', '.gemini/skills/trackfw-governance/SKILL.md', 'GEMINI.md']],
  ['cursor', installCursor, ['.cursor/agents/trackfw-architect.md', '.cursor/skills/trackfw-governance/SKILL.md', '.cursor/rules/trackfw.mdc']],
  ['copilot', installCopilot, ['.github/agents/trackfw-architect.agent.md', '.github/skills/trackfw-governance/SKILL.md', '.github/copilot-instructions.md']],
  ['windsurf', installWindsurf, ['.windsurf/skills/trackfw-agent-architect/SKILL.md', '.windsurf/skills/trackfw-governance/SKILL.md', '.windsurfrules']],
  ['amazonq', installAmazonQ, ['.amazonq/cli-agents/trackfw-architect.json', '.amazonq/rules/trackfw-governance.md', '.amazonq/developer/guidelines.md']],
]) {
  test(`install${fixture[0]} delegates idempotently to canonical integrations`, async () => {
    await assertIdempotentToolInstall(fixture[0], fixture[1], fixture[2])
  })
}
