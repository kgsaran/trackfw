'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const { buildPlans, execute, IntegrationManager } = require('../src/integrations')

function roots() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-integrations-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot)
  fs.mkdirSync(homeRoot)
  return { base, projectRoot, homeRoot }
}

function options(targets, items = ['governance'], scope = 'project') {
  return { targets, items, scope }
}

test('manager reports lifecycle states and honors force semantics', () => {
  const dirs = roots()
  const plans = buildPlans('agents', options(['codex'], ['architect']))
  const manager = new IntegrationManager(dirs)
  assert.equal(manager.inspect(plans)[0].state, 'not-installed')
  assert.equal(manager.install(plans)[0].state, 'current')

  const file = path.join(dirs.projectRoot, '.codex/agents/trackfw-architect.toml')
  fs.appendFileSync(file, '\ncustom=true\n')
  assert.equal(manager.inspect(plans)[0].state, 'modified')
  assert.throws(() => manager.update(plans), /without --force/)
  assert.equal(manager.update(plans, { force: true })[0].state, 'current')

  const newer = plans.map(plan => ({ ...plan, catalogVersion: '99.0.0', content: `${plan.content}\n# newer\n` }))
  assert.equal(manager.inspect(newer)[0].state, 'outdated')
})

test('shared claims preserve a physical skill until its final consumer is removed', () => {
  const dirs = roots()
  const plans = buildPlans('skills', options(['codex', 'antigravity'], ['governance']))
  const manager = new IntegrationManager(dirs)
  manager.install(plans)
  const file = path.join(dirs.projectRoot, '.agents/skills/trackfw-governance/SKILL.md')
  assert.equal(fs.existsSync(file), true)

  manager.uninstall(plans.filter(plan => plan.claim.target === 'codex'))
  assert.equal(fs.existsSync(file), true)
  const manifest = JSON.parse(fs.readFileSync(path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')))
  assert.equal(manifest.artifacts[0].claims.length, 1)
  assert.equal(manifest.artifacts[0].claims[0].target, 'antigravity')

  manager.uninstall(plans.filter(plan => plan.claim.target === 'antigravity'))
  assert.equal(fs.existsSync(file), false)
})

test('recognized legacy content is adopted but unknown files are never overwritten', () => {
  const dirs = roots()
  const [plan] = buildPlans('agents', options(['claude'], ['architect']))
  const manager = new IntegrationManager(dirs)
  const file = path.join(dirs.projectRoot, '.claude/agents/trackfw-architect.md')
  fs.mkdirSync(path.dirname(file), { recursive: true })
  fs.writeFileSync(file, plan.content)
  manager.install([plan])
  assert.equal(manager.inspect([plan])[0].managed, true)

  const dirs2 = roots()
  const manager2 = new IntegrationManager(dirs2)
  const unknown = path.join(dirs2.projectRoot, '.claude/agents/trackfw-architect.md')
  fs.mkdirSync(path.dirname(unknown), { recursive: true })
  fs.writeFileSync(unknown, 'user content')
  assert.throws(() => manager2.update([plan], { force: true }), /unmanaged file/)
  assert.equal(fs.readFileSync(unknown, 'utf8'), 'user content')
})

test('failed atomic mutation rolls files and manifest back', () => {
  const dirs = roots()
  const plans = buildPlans('agents', options(['claude'], ['architect', 'backend']))
  const manager = new IntegrationManager(dirs)
  const realWrite = manager.atomicWrite.bind(manager)
  let writes = 0
  manager.atomicWrite = (file, content) => {
    writes++
    if (writes === 2) throw new Error('injected write failure')
    realWrite(file, content)
  }
  assert.throws(() => manager.install(plans), /injected write failure/)
  for (const plan of plans) assert.equal(fs.existsSync(path.join(dirs.projectRoot, plan.destination)), false)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')), false)
})

test('manager rejects traversal, absolute destinations and symlinks', () => {
  const dirs = roots()
  const manager = new IntegrationManager(dirs)
  const [base] = buildPlans('agents', options(['claude'], ['architect']))
  assert.throws(() => manager.install([{ ...base, destination: '../escape.md' }]), /Unsafe|escapes/)
  assert.throws(() => manager.install([{ ...base, destination: '/tmp/escape.md' }]), /relative/)

  fs.mkdirSync(path.join(dirs.projectRoot, '.claude'))
  fs.symlinkSync(dirs.homeRoot, path.join(dirs.projectRoot, '.claude', 'agents'))
  assert.throws(() => manager.install([base]), /Symlink/)
})

test('project and global scopes use separate manifests', () => {
  const dirs = roots()
  const manager = new IntegrationManager(dirs)
  manager.install(buildPlans('skills', options(['claude'], ['plan'], 'project')))
  manager.install(buildPlans('skills', options(['claude'], ['plan'], 'global')))
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')), true)
  assert.equal(fs.existsSync(path.join(dirs.homeRoot, '.trackfw/integrations-manifest.json')), true)
})

test('renderers produce native deterministic formats', () => {
  const codex = buildPlans('agents', options(['codex'], ['architect']))[0]
  const amazonq = buildPlans('agents', options(['amazonq'], ['architect']))[0]
  const claude = buildPlans('agents', options(['claude'], ['architect']))[0]
  assert.match(codex.content, /^name = "architect"/)
  assert.equal(JSON.parse(amazonq.content).name, 'trackfw-architect')
  assert.match(claude.content, /^---\nname:/)
  assert.equal(codex.content, buildPlans('agents', options(['codex'], ['architect']))[0].content)
})

test('CLI emits the exact deterministic JSON envelope and supports lifecycle', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const args = ['agents', 'install', '--targets', 'codex', '--items', 'architect', '--scope', 'project', '--json']
  const installed = spawnSync(process.execPath, [bin, ...args], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.equal(installed.status, 0, installed.stderr)
  const output = JSON.parse(installed.stdout)
  assert.deepEqual(Object.keys(output), ['kind', 'catalog_version', 'items', 'deployments'])
  assert.deepEqual(Object.keys(output.deployments[0]), ['target', 'surface', 'scope', 'item', 'support_level', 'representation', 'destination', 'state', 'managed'])
  assert.equal(output.deployments[0].state, 'current')
  assert.equal(output.deployments[0].managed, true)

  const missing = spawnSync(process.execPath, [bin, 'skills', 'install'], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.notEqual(missing.status, 0)
  assert.match(missing.stderr, /--targets is required/)
})

test('init uses the canonical integration engine', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const run = spawnSync(process.execPath, [bin, 'init', '--ai-tools', 'antigravity'], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.equal(run.status, 0, run.stderr)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.agents/agents/trackfw-architect/agent.md')), true)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.agents/skills/trackfw-governance/SKILL.md')), true)
})
