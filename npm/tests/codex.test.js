'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { installCodex } = require('../src/generators/codex')

test('installCodex creates idempotent native Codex artifacts', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-codex-'))
  installCodex(root)
  installCodex(root)

  const required = [
    'AGENTS.md',
    '.codex/config.toml',
    '.codex/hooks.json',
    '.codex/agents/trackfw-architect.toml',
    '.codex/agents/trackfw-infra.toml',
    '.agents/skills/trackfw-governance/SKILL.md',
    '.agents/skills/trackfw-release/SKILL.md',
  ]
  for (const name of required) assert.equal(fs.existsSync(path.join(root, name)), true, name)

  const config = fs.readFileSync(path.join(root, '.codex/config.toml'), 'utf8')
  assert.equal((config.match(/^\[agents\]$/gm) || []).length, 1)
  assert.match(config, /max_threads = 6/)
  assert.match(config, /max_depth = 1/)

  const hooks = JSON.parse(fs.readFileSync(path.join(root, '.codex/hooks.json'), 'utf8')).hooks
  assert.equal(hooks.PermissionRequest.length, 1)
  assert.equal(hooks.PermissionRequest[0].hooks[0].command, 'scripts/trackfw-attention-signal.sh')
  assert.equal(hooks.PostToolUse.length, 1)
  assert.equal(hooks.PostToolUse[0].hooks[0].command, 'scripts/trackfw-attention-cleanup.sh')
  assert.equal(hooks.PreToolUse, undefined)
})
