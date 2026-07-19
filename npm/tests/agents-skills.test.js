'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const { buildPlans, execute, IntegrationManager } = require('../src/integrations')
const { sha256 } = require('../src/integrations/manager')
const { promptSelection, promptAmbiguousSurfaces } = require('../src/commands/integrations')
const { legacyCodexFixtures } = require('../src/generators/codex')

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
  assert.throws(() => manager.update(plans), /--force/)
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
  const artifact = Object.values(manifest.artifacts)[0]
  assert.equal(artifact.claims.length, 1)
  assert.equal(artifact.claims[0].target, 'antigravity')

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
  assert.throws(() => manager2.update([plan], { force: true }), /unmanaged artifact/i)
  assert.equal(fs.readFileSync(unknown, 'utf8'), 'user content')
})

test('all historical Claude agent fixtures are wired to current destinations', () => {
  const historicalRoot = path.resolve(__dirname, '../../internal/generators/templates/agents')
  const plans = buildPlans('agents', { targets: ['claude'], scope: 'global' })
  assert.equal(plans.length, 10)
  for (const plan of plans) {
    const historical = fs.readFileSync(path.join(historicalRoot, `trackfw-${plan.claim.item}.md`))
    assert.equal(plan.legacyHashes.includes(sha256(historical)), true, plan.claim.item)
  }
  assert.equal(buildPlans('agents', { targets: ['claude'], items: ['backend'], scope: 'project' })[0].legacyHashes.length, 0)
  assert.equal(buildPlans('agents', { targets: ['codex'], items: ['backend'], scope: 'global' })[0].legacyHashes.length, 0)
})

test('Codex legacy union recognizes exact Go, npm and Python producer bytes', () => {
  const [plan] = buildPlans('agents', options(['codex'], ['backend']))
  const producerFixtures = {
    go: `name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """
Implement only the assigned backend scope. Preserve public contracts and trackfw traceability.
Run focused tests and report changed files, validation evidence, and remaining risks.
"""
`,
    npm: `${legacyCodexFixtures.agents['trackfw-backend.toml'].trim()}\n`,
    python: `name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """Implement only the assigned backend scope, preserve contracts and traceability, and run focused tests."""
`,
  }
  for (const [producer, content] of Object.entries(producerFixtures)) {
    assert.equal(plan.legacyHashes.includes(sha256(content)), true, producer)
    const dirs = roots()
    const filename = path.join(dirs.projectRoot, plan.destination)
    fs.mkdirSync(path.dirname(filename), { recursive: true })
    fs.writeFileSync(filename, content)
    assert.deepEqual(new IntegrationManager(dirs).inspect([plan]).map(entry => [entry.state, entry.managed]), [['outdated', false]], producer)
  }
})

test('historical Codex agents and skills are recognized, adopted, then converted on update', () => {
  const dirs = roots()
  for (const [name, content] of Object.entries(legacyCodexFixtures.agents)) {
    const filename = path.join(dirs.projectRoot, '.codex/agents', name)
    fs.mkdirSync(path.dirname(filename), { recursive: true })
    fs.writeFileSync(filename, `${content.trim()}\n`)
  }
  for (const [name, content] of Object.entries(legacyCodexFixtures.skills)) {
    const filename = path.join(dirs.projectRoot, '.agents/skills', name, 'SKILL.md')
    fs.mkdirSync(path.dirname(filename), { recursive: true })
    fs.writeFileSync(filename, `${content.trim()}\n`)
  }
  const manager = new IntegrationManager(dirs)
  const agentItems = ['architect', 'backend', 'frontend', 'qa', 'security']
  const skillItems = ['governance', 'plan', 'implement', 'review', 'release']
  const agentPlans = buildPlans('agents', options(['codex'], agentItems))
  const skillPlans = buildPlans('skills', options(['codex'], skillItems))

  for (const plan of [...agentPlans, ...skillPlans]) {
    const filename = path.join(dirs.projectRoot, plan.destination)
    const historical = fs.readFileSync(filename)
    assert.equal(plan.legacyHashes.includes(sha256(historical)), true, `${plan.claim.kind}:${plan.claim.item}`)
    assert.deepEqual(manager.inspect([plan]).map(entry => [entry.state, entry.managed]), [['outdated', false]])
  }

  const plan = agentPlans.find(entry => entry.claim.item === 'architect')
  const filename = path.join(dirs.projectRoot, plan.destination)
  const historical = fs.readFileSync(filename)
  manager.install([plan])
  assert.deepEqual(fs.readFileSync(filename), historical, 'install adoption must not overwrite legacy bytes')
  assert.deepEqual(manager.inspect([plan]).map(entry => [entry.state, entry.managed]), [['outdated', true]])
  const manifest = JSON.parse(fs.readFileSync(path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')))
  assert.equal(manifest.artifacts[filename].catalog_version, 'legacy')

  manager.update([plan])
  assert.equal(fs.readFileSync(filename, 'utf8'), plan.content)
  assert.deepEqual(manager.inspect([plan]).map(entry => [entry.state, entry.managed]), [['current', true]])
})

test('install force replaces unknown unmanaged content while update force never does', () => {
  const dirs = roots()
  const [plan] = buildPlans('agents', options(['claude'], ['architect']))
  const manager = new IntegrationManager(dirs)
  const file = path.join(dirs.projectRoot, plan.destination)
  fs.mkdirSync(path.dirname(file), { recursive: true })
  fs.writeFileSync(file, 'unknown user bytes')
  assert.throws(() => manager.install([plan]), /modified|force/i)
  manager.install([plan], { force: true })
  assert.equal(fs.readFileSync(file, 'utf8'), plan.content)

  const dirs2 = roots()
  const file2 = path.join(dirs2.projectRoot, plan.destination)
  fs.mkdirSync(path.dirname(file2), { recursive: true })
  fs.writeFileSync(file2, 'unknown user bytes')
  assert.throws(() => new IntegrationManager(dirs2).update([plan], { force: true }), /unmanaged/i)
})

test('unmanaged desired is current, legacy is outdated, and owned outdated requires update', () => {
  const dirs = roots()
  const [plan] = buildPlans('agents', options(['claude'], ['architect']))
  const manager = new IntegrationManager(dirs)
  const file = path.join(dirs.projectRoot, plan.destination)
  fs.mkdirSync(path.dirname(file), { recursive: true })
  fs.writeFileSync(file, plan.content)
  assert.deepEqual(manager.inspect([plan]).map(x => [x.state, x.managed]), [['current', false]])
  fs.writeFileSync(file, 'recognized old template')
  const legacy = { ...plan, legacyHashes: [sha256('recognized old template')] }
  assert.deepEqual(manager.inspect([legacy]).map(x => [x.state, x.managed]), [['outdated', false]])
  manager.install([legacy])
  assert.throws(() => manager.install([plan]), /outdated.*update/i)
})

test('Go manifest fixture is interoperable for inspect, update and uninstall', () => {
  const dirs = roots()
  const [plan] = buildPlans('agents', options(['claude'], ['architect']))
  const destination = path.join(dirs.projectRoot, plan.destination)
  fs.mkdirSync(path.dirname(destination), { recursive: true })
  fs.writeFileSync(destination, plan.content, { mode: 0o644 })
  const manifestFile = path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')
  fs.mkdirSync(path.dirname(manifestFile), { recursive: true })
  fs.writeFileSync(manifestFile, `${JSON.stringify({ schema_version: 1, artifacts: {
    [destination]: { destination, sha256: sha256(plan.content), catalog_version: plan.catalogVersion, claims: [plan.claim] },
  } }, null, 2)}\n`, { mode: 0o600 })

  const manager = new IntegrationManager(dirs)
  assert.deepEqual(manager.inspect([plan]).map(x => [x.state, x.managed]), [['current', true]])
  const updated = { ...plan, content: `${plan.content}updated\n`, catalogVersion: '1.2.0' }
  manager.update([updated])
  const nodeManifest = JSON.parse(fs.readFileSync(manifestFile, 'utf8'))
  assert.deepEqual(Object.keys(nodeManifest), ['schema_version', 'artifacts'])
  assert.deepEqual(Object.keys(nodeManifest.artifacts[destination]), ['destination', 'sha256', 'catalog_version', 'claims'])
  assert.deepEqual(nodeManifest.artifacts[destination].claims[0], plan.claim)
  assert.equal(fs.statSync(destination).mode & 0o777, 0o644)
  assert.equal(fs.statSync(manifestFile).mode & 0o777, 0o600)
  manager.uninstall([updated])
  assert.equal(fs.existsSync(destination), false)
})

test('failed atomic mutation rolls files and manifest back', () => {
  const dirs = roots()
  const plans = buildPlans('agents', options(['claude'], ['architect', 'backend']))
  const manager = new IntegrationManager(dirs)
  const realWrite = manager.atomicWrite.bind(manager)
  let writes = 0
  manager.atomicWrite = (file, content, mode) => {
    writes++
    if (writes === 2) throw new Error('injected write failure')
    realWrite(file, content, mode)
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
  assert.throws(() => manager.install([{ ...base, destination: '/tmp/escape.md' }]), /outside/)

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
  assert.match(codex.content, /^name = "trackfw_architect"/)
  assert.equal(JSON.parse(amazonq.content).name, 'trackfw-architect')
  assert.match(claude.content, /^---\nname:/)
  assert.equal(codex.content, buildPlans('agents', options(['codex'], ['architect']))[0].content)
})

test('Codex TOML renderer is byte-equivalent to the Go golden contract', () => {
  const backend = buildPlans('agents', options(['codex'], ['backend']))[0]
  const expected = 'name = "trackfw_backend"\n' +
    'description = "Senior backend specialist for APIs, domain logic, integrations and data access."\n' +
    'developer_instructions = "# Backend\\n\\nImplement only the assigned backend scope. Preserve public contracts, Clean Architecture boundaries, observability and trackfw traceability. Run focused build and tests and report evidence and remaining risks."\n'
  assert.equal(backend.content, expected)

  const codeQuality = buildPlans('agents', options(['codex'], ['code-quality']))[0]
  assert.match(codeQuality.content, /^name = "trackfw_code_quality"\n/)
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
  assert.match(missing.stderr, /install requires --targets/)
})

test('legacy trackfw update alias preserves unknown Codex bytes and warns', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  fs.writeFileSync(path.join(dirs.projectRoot, 'trackfw.yaml'), 'hooks: none\nci: none\n')
  const unknown = path.join(dirs.projectRoot, '.codex/agents/trackfw-backend.toml')
  fs.mkdirSync(path.dirname(unknown), { recursive: true })
  fs.writeFileSync(unknown, 'user-owned unknown bytes\n')
  const run = spawnSync(process.execPath, [bin, 'update'], {
    cwd: dirs.projectRoot,
    env: { ...process.env, HOME: dirs.homeRoot },
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  assert.equal(fs.readFileSync(unknown, 'utf8'), 'user-owned unknown bytes\n')
  assert.match(run.stderr, /Codex integration:.*Unmanaged artifact/i)
})

test('CLI uses repeatable --surface and unfiltered list includes legacy surfaces', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const selected = spawnSync(process.execPath, [bin, 'agents', 'list', '--targets', 'kiro', '--surface', 'kiro=cli', '--items', 'architect', '--json'], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.equal(selected.status, 0, selected.stderr)
  assert.equal(JSON.parse(selected.stdout).deployments[0].surface, 'cli')

  const all = spawnSync(process.execPath, [bin, 'agents', 'list', '--items', 'architect', '--json'], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.equal(all.status, 0, all.stderr)
  const deployments = JSON.parse(all.stdout).deployments
  assert.equal(deployments.some(entry => entry.target === 'antigravity' && entry.surface === 'legacy-cli'), true)
  assert.equal(deployments.some(entry => entry.target === 'kiro' && entry.surface === 'cli'), true)

  const filtered = spawnSync(process.execPath, [bin, 'agents', 'list', '--targets', 'antigravity', '--items', 'architect', '--json'], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.equal(filtered.status, 0, filtered.stderr)
  assert.deepEqual(JSON.parse(filtered.stdout).deployments.map(entry => entry.surface), ['current', 'legacy-cli'])

  const human = spawnSync(process.execPath, [bin, 'skills', 'list', '--targets', 'claude', '--items', 'plan'], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.match(human.stdout, /Available skills/)
  assert.match(human.stdout, /Governance/)
  assert.match(human.stdout, /Deployments:/)
})

test('TTY prompts select targets and items and disambiguate non-legacy surfaces', async () => {
  const selections = [['kiro'], ['architect']]
  const selected = { targets: [], items: [], surfaces: [] }
  await promptSelection('agents', selected, { checkbox: async () => selections.shift() })
  assert.deepEqual(selected.targets, ['kiro'])
  assert.deepEqual(selected.items, ['architect'])
  await promptAmbiguousSurfaces('agents', selected, { select: async () => 'cli' })
  assert.deepEqual(selected.surfaces, ['kiro=cli'])
})

test('init uses the canonical integration engine', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const run = spawnSync(process.execPath, [bin, 'init', '--ai-tools', 'antigravity'], { cwd: dirs.projectRoot, encoding: 'utf8' })
  assert.equal(run.status, 0, run.stderr)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.agents/agents/trackfw-architect/agent.md')), true)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.agents/skills/trackfw-governance/SKILL.md')), true)
})
