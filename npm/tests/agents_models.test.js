'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const { resolveAgentModel, looksLikeSuspectModelValue } = require('../src/integrations/render')

// ---------------------------------------------------------------------------
// LooksLikeSuspectModelValue — criterion for the "4.6-beta" warning (ML-2A)
// ---------------------------------------------------------------------------

test('looksLikeSuspectModelValue warns on 4.6-beta', () => {
  assert.equal(looksLikeSuspectModelValue('4.6-beta'), true)
})

test('looksLikeSuspectModelValue does not warn on bare version strings', () => {
  assert.equal(looksLikeSuspectModelValue('4.6'), false)
  assert.equal(looksLikeSuspectModelValue('5'), false)
  assert.equal(looksLikeSuspectModelValue('1.0.2'), false)
})

test('looksLikeSuspectModelValue does not warn on claude- prefixed IDs', () => {
  assert.equal(looksLikeSuspectModelValue('claude-sonnet-4-5-20250929'), false)
  assert.equal(looksLikeSuspectModelValue('claude-opus-5'), false)
})

test('looksLikeSuspectModelValue warns on wrong-namespace values', () => {
  assert.equal(looksLikeSuspectModelValue('gpt-5'), true)
  assert.equal(looksLikeSuspectModelValue('latest'), true)
})

// ---------------------------------------------------------------------------
// resolveAgentModel — resolution table (ML-2A)
// ---------------------------------------------------------------------------

const pinned = { sonnet: '4.6', opus: '5' }
const empty = {}

test('resolveAgentModel: claude target no pin → tier alias', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'subagent', 'claude', empty), { resolved: 'sonnet', present: true })
  assert.deepEqual(resolveAgentModel('opus', 'subagent', 'claude', empty), { resolved: 'opus', present: true })
})

test('resolveAgentModel: claude target with pin → composed model ID', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'subagent', 'claude', pinned), { resolved: 'claude-sonnet-4-6', present: true })
  assert.deepEqual(resolveAgentModel('opus', 'subagent', 'claude', pinned), { resolved: 'claude-opus-5', present: true })
})

test('resolveAgentModel: claude target escape hatch (4.6-beta) → literal', () => {
  assert.deepEqual(
    resolveAgentModel('sonnet', 'subagent', 'claude', { sonnet: '4.6-beta' }),
    { resolved: '4.6-beta', present: true }
  )
})

test('resolveAgentModel: claude target escape hatch (claude- prefix) → literal no warn', () => {
  assert.deepEqual(
    resolveAgentModel('sonnet', 'subagent', 'claude', { sonnet: 'claude-sonnet-4-5-20250929' }),
    { resolved: 'claude-sonnet-4-5-20250929', present: true }
  )
})

test('resolveAgentModel: codex → mapModelCodex (agentModels ignored)', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'custom-agent-toml', 'codex', pinned), { resolved: 'gpt-5.4-mini', present: true })
  assert.deepEqual(resolveAgentModel('opus', 'custom-agent-toml', 'codex', pinned), { resolved: 'gpt-5.4', present: true })
})

test('resolveAgentModel: cursor → mapModelCursor (agentModels ignored)', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'agent-markdown', 'cursor', pinned), { resolved: 'composer-2.5[fast=true]', present: true })
  assert.deepEqual(resolveAgentModel('opus', 'agent-markdown', 'cursor', pinned), { resolved: 'claude-opus-5[effort=high]', present: true })
})

test('resolveAgentModel: antigravity → resolveModel (agentModels ignored)', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'agent-directory', 'antigravity', pinned), { resolved: 'flash', present: true })
  assert.deepEqual(resolveAgentModel('opus', 'agent-directory', 'antigravity', pinned), { resolved: 'pro', present: true })
})

test('resolveAgentModel: amazonq cli-agent-json → no model field', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'cli-agent-json', 'amazonq', pinned), { resolved: '', present: false })
})

test('resolveAgentModel: opencode → no model field', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'opencode-agent', 'opencode', pinned), { resolved: '', present: false })
})

test('resolveAgentModel: agent-json → no model field', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'agent-json', 'antigravity', pinned), { resolved: '', present: false })
})

test('resolveAgentModel: gemini agent-markdown → tier alias (agentModels ignored)', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'agent-markdown', 'gemini', pinned), { resolved: 'sonnet', present: true })
  assert.deepEqual(resolveAgentModel('opus', 'agent-markdown', 'gemini', pinned), { resolved: 'opus', present: true })
})

test('resolveAgentModel: copilot custom-agent → tier alias', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'custom-agent', 'copilot', pinned), { resolved: 'sonnet', present: true })
})

test('resolveAgentModel: windsurf skill → tier alias', () => {
  assert.deepEqual(resolveAgentModel('sonnet', 'skill', 'windsurf', pinned), { resolved: 'sonnet', present: true })
})
