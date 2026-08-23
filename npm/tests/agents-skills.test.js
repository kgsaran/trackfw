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
  assert.equal(plans.length, 12)
  for (const plan of plans) {
    const historicalPath = path.join(historicalRoot, `trackfw-${plan.claim.item}.md`)
    // New agents (e.g. iac, tooling) have no historical fixture — skip legacy hash check.
    if (!fs.existsSync(historicalPath)) continue
    const historical = fs.readFileSync(historicalPath)
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

test('unmanaged desired is current, legacy is outdated, and owned outdated skips install', () => {
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
  // owned + outdated → skip em vez de erro (contrato: cli-parity.md, seção
  // "install sobre artefato gerenciado desatualizado — skip, não erro fatal").
  // onSkip recebe (destination=tilde-abreviado, reason=linha completa pronta
  // para impressão) — contrato "Valor de cada parâmetro — pinado".
  const skips = []
  const managerWithSkip = new IntegrationManager(dirs, { onSkip: (dest, reason) => skips.push({ dest, reason }) })
  assert.doesNotThrow(() => managerWithSkip.install([plan]))
  assert.equal(fs.readFileSync(file, 'utf8'), 'recognized old template')
  assert.equal(skips.length, 1)
  // destination é o caminho relativo ao projectRoot (escopo project, sem './')
  assert.equal(skips[0].dest, plan.destination)
  // reason é a linha de aviso completa e pronta para impressão
  assert.equal(skips[0].reason, `warning: skipping outdated artifact ${plan.destination}; run 'trackfw update' to refresh it`)
})

test('mixed-scope batch: each artifact receives remediation derived from claim.scope', () => {
  // Caso que distingue derivação por artefato (plan.claim.scope) de derivação
  // por closure ou por inferência sobre o caminho. Um lote com artefatos de
  // projeto e globais deve emitir remediação correta para cada um.
  const dirs = roots()
  const [projectPlan] = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project' })
  const [globalPlan] = buildPlans('agents', { targets: ['gemini'], items: ['architect'], scope: 'global' })

  const legacyContent = 'recognized old template for mixed-scope test'
  const legacyHash = sha256(legacyContent)
  const projectLegacy = { ...projectPlan, legacyHashes: [legacyHash] }
  const globalLegacy = { ...globalPlan, legacyHashes: [legacyHash] }

  // Escrever conteúdo legado em ambos os arquivos para permitir adoção
  const projectFile = path.join(dirs.projectRoot, projectPlan.destination)
  const globalFile = path.join(dirs.homeRoot, '.gemini', 'agents', 'trackfw-architect.md')
  fs.mkdirSync(path.dirname(projectFile), { recursive: true })
  fs.writeFileSync(projectFile, legacyContent)
  fs.mkdirSync(path.dirname(globalFile), { recursive: true })
  fs.writeFileSync(globalFile, legacyContent)

  const manager = new IntegrationManager(dirs)
  // Adoção: instala ambos os legacy plans → owned com state outdated
  manager.install([projectLegacy, globalLegacy])

  // Instala os planos desejados → ambos devem ser pulados (outdated + owned)
  const skips = []
  const managerWithSkip = new IntegrationManager(dirs, { onSkip: (dest, reason) => skips.push({ dest, reason }) })
  assert.doesNotThrow(() => managerWithSkip.install([projectPlan, globalPlan]))
  assert.equal(skips.length, 2)

  // Artefato de projeto: caminho relativo, remediação 'trackfw update'
  const projectSkip = skips.find(s => s.dest === projectPlan.destination)
  assert.ok(projectSkip, 'project artifact should be skipped with relative dest')
  assert.equal(
    projectSkip.reason,
    `warning: skipping outdated artifact ${projectPlan.destination}; run 'trackfw update' to refresh it`
  )

  // Artefato global: caminho tilde-abreviado, remediação 'trackfw update harness'
  const globalSkip = skips.find(s => s.dest === '~/.gemini/agents/trackfw-architect.md')
  assert.ok(globalSkip, 'global artifact should be skipped with tilde dest')
  assert.equal(
    globalSkip.reason,
    `warning: skipping outdated artifact ~/.gemini/agents/trackfw-architect.md; run 'trackfw update harness' to refresh it`
  )
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
  // Re-congelado em 2026-07-26 (Wave 2): adendo do implementador (Governance prerequisite,
  // Git boundary, Microbatch completion protocol, Definition of done, Mission) adicionado ao backend.
  // Re-congelado em 2026-08-14 (Wave 2 do ROADMAP-2026-08-14-roteamento-de-model-tier):
  // "model = ..." passa a ser emitido entre description e developer_instructions, mapeado
  // a partir do tier canônico do frontmatter ("model: sonnet" → "gpt-5.4-mini") — ADR
  // ADR-2026-08-14-roteamento-de-model-tier-por-alvo-no-render-de-agentes-para-codex-e-cursor.
  const expected = 'name = "trackfw_backend"\n' +
    'description = "Senior backend specialist for APIs, domain logic, integrations and data access."\n' +
    'model = "gpt-5.4-mini"\n' +
    'developer_instructions = "# Backend\\n\\n## Mode lock\\nYou are pinned as Backend. Until the user explicitly hands off: do not switch persona; do not load or cite instructions from other agents; this file is your only authority. On violation, stop and reply \\"MODE LOCK VIOLATED. Remaining as Backend.\\"\\n\\n## Before you act\\nRead the existing code before proposing or editing anything. Never invent file paths, symbols, commands or contracts: verify them first. If the information needed to act is missing, stop and say what is missing instead of guessing.\\n\\n## Scope boundary\\nWork only within this role\'s domain. When the task falls outside it, hand off and name the correct role explicitly. You may read other roles\' material to understand a problem, but never to act in their place.\\n\\n## Working context\\nAppend an entry to `docs/agents-working-context.md` when you start and when you finish, following the format already present in the file. Do this automatically, without asking.\\n\\n## Knowledge vault\\nBefore investigating a bug or unexpected behavior, read `vault/notes/index.md` when it exists and open the related notes. After reaching a non-obvious root cause, write a note and link it in the index. Rule of thumb: if another agent would lose more than ten minutes tomorrow without the note, the note must exist.\\n\\n## Governance prerequisite\\nDo not edit code without a requirement and a roadmap already in the `wip` state. Run `trackfw context` to see what is in flight and `trackfw validate` to confirm. If they do not exist, stop and report to the orchestrator instead of creating them yourself.\\n\\n## Git authority\\nThis role never executes Git operations — no `branch`, `commit`, `push`, `checkout`, `merge`, `rebase` or `stash`. `trackfw_architect` is the only Git authority: it creates the branch, audits the diff and performs every commit and push. Act only on a self-contained handoff from `trackfw_architect`; refuse to implement anything without one.\\n\\n## Microbatch completion protocol\\nIn order: build, tests, project gate, `trackfw validate`, then report the exact command output as evidence and hand the microbatch back to `trackfw_architect` for audit and commit. Update the microbatch status in the roadmap only after the orchestrator\'s audit passes.\\n\\n## Definition of done\\nGreen build and tests do not close a microbatch. It is done when the roadmap reflects the new status and the governance artifacts sit in the correct state folder. Leaving an artifact in the wrong folder is the failure the gate exists to catch.\\n\\n## Mission\\nImplement only the assigned backend scope. Preserve public contracts, Clean Architecture boundaries, observability and trackfw traceability. Run focused build and tests and report evidence and remaining risks.\\n\\n— Backend, Senior Backend Specialist"\n'
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

// ML-6C (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador)
// reworked `trackfw update` into the missing/updated/skipped/failed target
// contract (docs/cli-parity.md). The codex-project-agents target now
// classifies items via IntegrationManager.inspect() first and only ever
// calls manager.update() on the subset that is actually 'outdated' (or
// 'not-installed' with --install-missing) — an unmanaged/modified file is
// excluded from that call entirely, so it is preserved without ever
// throwing, and the target simply reports 'skipped' (nothing about it
// needed writing). This replaces the old behavior of calling
// manager.update() on every present item and letting it throw + warn on
// conflict — same outcome for the file on disk, quieter and cheaper.
test('legacy trackfw update alias preserves unknown Codex bytes without a warning', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  fs.writeFileSync(path.join(dirs.projectRoot, 'trackfw.yaml'), 'hooks: none\nci: none\n')
  const unknown = path.join(dirs.projectRoot, '.codex/agents/trackfw-backend.toml')
  fs.mkdirSync(path.dirname(unknown), { recursive: true })
  fs.writeFileSync(unknown, 'user-owned unknown bytes\n')
  const run = spawnSync(process.execPath, [bin, 'update', '--json'], {
    cwd: dirs.projectRoot,
    env: { ...process.env, HOME: dirs.homeRoot },
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  assert.equal(fs.readFileSync(unknown, 'utf8'), 'user-owned unknown bytes\n')
  const doc = JSON.parse(run.stdout)
  const target = doc.targets.find(t => t.id === 'codex-project-agents')
  assert.equal(target.state, 'skipped')
})

test('legacy trackfw update alias converts only present Codex artifacts', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  fs.writeFileSync(path.join(dirs.projectRoot, 'trackfw.yaml'), 'hooks: none\nci: none\n')
  const backend = path.join(dirs.projectRoot, '.codex/agents/trackfw-backend.toml')
  fs.mkdirSync(path.dirname(backend), { recursive: true })
  fs.writeFileSync(backend, `${legacyCodexFixtures.agents['trackfw-backend.toml'].trim()}\n`)
  const run = spawnSync(process.execPath, [bin, 'update'], {
    cwd: dirs.projectRoot,
    env: { ...process.env, HOME: dirs.homeRoot },
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  const desired = buildPlans('agents', options(['codex'], ['backend']))[0]
  assert.equal(fs.readFileSync(backend, 'utf8'), desired.content)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.codex/agents/trackfw-qa.toml')), false)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.agents/skills/trackfw-governance/SKILL.md')), false)
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

// Sem TTY e sem --scope, o escopo default passa a ser `global` (ADR-2026-07-
// 25-escopo-de-instalacao-selecionavel-para-agents-e-skills, D1 — breaking
// change documentado no CHANGELOG). `init` grava, portanto, em
// ~/.gemini/config (HOME redirecionado para o diretório de teste), não mais
// em .agents/ do projeto.
test('init uses the canonical integration engine', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const run = spawnSync(process.execPath, [bin, 'init', '--ai-tools', 'antigravity'], {
    cwd: dirs.projectRoot,
    env: { ...process.env, HOME: dirs.homeRoot },
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  assert.equal(fs.existsSync(path.join(dirs.homeRoot, '.gemini/config/agents/trackfw-architect/agent.md')), true)
  assert.equal(fs.existsSync(path.join(dirs.homeRoot, '.gemini/config/skills/trackfw-governance/SKILL.md')), true)
})

// Escopo explícito `--scope project` continua respeitado sem prompt e sem
// depender do default global acima.
test('agents install --scope project explícito instala no projeto', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const run = spawnSync(process.execPath, [bin, 'agents', 'install', '--targets', 'claude', '--items', 'architect', '--scope', 'project'], {
    cwd: dirs.projectRoot,
    env: { ...process.env, HOME: dirs.homeRoot },
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.claude/agents/trackfw-architect.md')), true)
  assert.equal(fs.existsSync(path.join(dirs.homeRoot, '.claude/agents/trackfw-architect.md')), false)
})

// Sem TTY e sem --scope, `agents install` grava em ~/.claude (default global).
test('agents install sem TTY e sem --scope grava em ~/.claude', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const run = spawnSync(process.execPath, [bin, 'agents', 'install', '--targets', 'claude', '--items', 'architect'], {
    cwd: dirs.projectRoot,
    env: { ...process.env, HOME: dirs.homeRoot },
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  assert.equal(fs.existsSync(path.join(dirs.homeRoot, '.claude/agents/trackfw-architect.md')), true)
  assert.equal(fs.existsSync(path.join(dirs.projectRoot, '.claude/agents/trackfw-architect.md')), false)
})

// `agents list` nunca pergunta (comando de leitura, D6), mas adota o mesmo
// default `global` do `install` — caso contrário reportaria deployments
// divergentes dos que o `install` de fato gravou.
test('agents list sem --scope reporta destinos globais', () => {
  const dirs = roots()
  const bin = path.resolve(__dirname, '../bin/trackfw')
  const run = spawnSync(process.execPath, [bin, 'agents', 'list', '--targets', 'claude', '--items', 'architect', '--json'], {
    cwd: dirs.projectRoot,
    env: { ...process.env, HOME: dirs.homeRoot },
    encoding: 'utf8',
  })
  assert.equal(run.status, 0, run.stderr)
  const output = JSON.parse(run.stdout)
  assert.equal(output.deployments[0].scope, 'global')
  assert.match(output.deployments[0].destination, /^~\/\.claude\//)
})

// `--targets claude` sem `--scope`, com TTY simulado, aciona o resolvedor de
// escopo (o prompt é um gate independente da seleção de targets — dispara
// mesmo quando --targets já foi informado).
test('--targets claude sem --scope, com TTY simulado, aciona o resolvedor de escopo', async () => {
  const { resolveScope } = require('../src/commands/integrations')
  const originalIsTTY = Object.getOwnPropertyDescriptor(process.stdin, 'isTTY')
  Object.defineProperty(process.stdin, 'isTTY', { value: true, configurable: true })
  let called = false
  try {
    const scope = await resolveScope(
      { targets: ['claude'] },
      { interactive: true, prompts: { select: async () => { called = true; return 'global' } } },
    )
    assert.equal(called, true, 'o resolvedor de escopo deve perguntar quando --scope não foi informado e stdin é TTY')
    assert.equal(scope, 'global')
  } finally {
    if (originalIsTTY) Object.defineProperty(process.stdin, 'isTTY', originalIsTTY)
    else delete process.stdin.isTTY
  }
})

// Divergência #5 do roadmap (reconciliação ML-2A): o teste acima estuba
// `select` para retornar 'global' incondicionalmente, então nunca prova que a
// pré-seleção (`global`) do prompt real é a que de fato chega ao usuário. Este
// teste captura a configuração passada ao `select()` real de resolveScope e
// verifica que `default: 'global'` (integrations.js:69) é exatamente o campo
// que a implementação de produção envia — regressão aqui falha se alguém
// remover ou trocar o default sem tocar em nenhum outro teste.
test('resolveScope: TTY sem --scope monta o select real com "global" pré-selecionado', async () => {
  const { resolveScope } = require('../src/commands/integrations')
  const originalIsTTY = Object.getOwnPropertyDescriptor(process.stdin, 'isTTY')
  Object.defineProperty(process.stdin, 'isTTY', { value: true, configurable: true })
  let capturedConfig = null
  try {
    await resolveScope(
      {},
      { interactive: true, prompts: { select: async (config) => { capturedConfig = config; return config.default } } },
    )
  } finally {
    if (originalIsTTY) Object.defineProperty(process.stdin, 'isTTY', originalIsTTY)
    else delete process.stdin.isTTY
  }
  assert.ok(capturedConfig, 'select() deveria ter sido chamado')
  assert.equal(capturedConfig.default, 'global')
  assert.deepEqual(capturedConfig.choices.map(choice => choice.value), ['global', 'project'])
})

test('Antigravity agent-directory renderer é byte-equivalente ao contrato Go/Python', () => {
  const architect = buildPlans('agents', options(['antigravity'], ['architect']))[0]
  const backend = buildPlans('agents', options(['antigravity'], ['backend']))[0]

  // IDs proibidos — nunca devem aparecer no output
  const forbidden = ['edit_file', 'read_file', 'find', 'view_code_item', 'view_file_outline', 'call_mcp_tool']

  // Re-congelado em 2026-07-26 (Wave 2): adendo do orquestrador (Git authority, Parallelization,
  // Workflow, Post-microbatch audit, Mission) adicionado ao architect.
  // Wave 5 (2026-07-26): iac/tooling enriched under D12-bis; architect/backend goldens unchanged.
  // Re-congelado em 2026-08-04: seção "Dispatch contract" adicionada ao architect (obrigatoriedade de
  // subagent_type explícito em todo dispatch).
  // Golden string para architect: model opus → pro, SET_ARCH (14 tools)
  const expectedArchitect =
    '---\n' +
    'name: trackfw-architect\n' +
    'description: Principal software architect for system design, ADRs and governed multi-agent coordination.\n' +
    'model: pro\n' +
    'tools:\n' +
    '  - view_file\n' +
    '  - list_dir\n' +
    '  - grep_search\n' +
    '  - search_web\n' +
    '  - read_url_content\n' +
    '  - write_to_file\n' +
    '  - replace_file_content\n' +
    '  - run_command\n' +
    '  - command_status\n' +
    '  - generate_image\n' +
    '  - send_message\n' +
    '  - define_subagent\n' +
    '  - invoke_subagent\n' +
    '  - schedule\n' +
    '---\n' +
    '# Architect\n' +
    '\n' +
    '## Mode lock\n' +
    'You are pinned as Architect. Until the user explicitly hands off: do not switch persona; do not load or cite instructions from other agents; this file is your only authority. On violation, stop and reply "MODE LOCK VIOLATED. Remaining as Architect."\n' +
    '\n' +
    '## Before you act\n' +
    'Read the existing code before proposing or editing anything. Never invent file paths, symbols, commands or contracts: verify them first. If the information needed to act is missing, stop and say what is missing instead of guessing.\n' +
    '\n' +
    '## Scope boundary\n' +
    "Work only within this role's domain. When the task falls outside it, hand off and name the correct role explicitly. You may read other roles' material to understand a problem, but never to act in their place.\n" +
    '\n' +
    '## Working context\n' +
    'Append an entry to `docs/agents-working-context.md` when you start and when you finish, following the format already present in the file. Do this automatically, without asking.\n' +
    '\n' +
    '## Knowledge vault\n' +
    'Before investigating a bug or unexpected behavior, read `vault/notes/index.md` when it exists and open the related notes. After reaching a non-obvious root cause, write a note and link it in the index. Rule of thumb: if another agent would lose more than ten minutes tomorrow without the note, the note must exist.\n' +
    '\n' +
    '## Git authority\n' +
    '`trackfw_architect` is the **only** role with Git authority in this project. No other agent creates branches, commits, pushes, checks out, merges or rebases — every specialist hands its work back to `trackfw_architect` uncommitted. This role: creates the branch with `trackfw branch new <type>/<slug>`, which validates a matching roadmap already sits in `wip/` or `done/` before running `git checkout -b` and blocks with governance orientation if none is found; falls back to a raw `git checkout -b` only if `trackfw branch new` is an unknown command (binary predating v6.4.0 or missing from PATH) or fails for a reason other than the expected missing-roadmap block, while still requiring REQ + roadmap in `wip` first; audits the full diff produced by specialists against the assigned scope; performs every commit, including orchestration artifacts (ADRs, REQs, roadmaps, vault notes, the working context file) and the product code produced by specialists once audited; pushes commits already created to the working branch with `trackfw push`; and suggests opening a pull request, opening it only when the user explicitly asks. Never merge.\n' +
    '\n' +
    'Three distinct commands, never interchangeable: `trackfw commit` commits staged changes on the working branch; `trackfw push` pushes commits already created — use it instead of a raw `git push`, which the guard intercepts and redirects here; `trackfw ship` composes commit + push + PR into one governed step. Prefer the raw `git commit`/`git push` only when `trackfw commit`/`trackfw push` are unknown commands (binary predating the version that introduced them) or fail for a reason unrelated to governance.\n' +
    '\n' +
    '## Barrier protocol\n' +
    'Before releasing the next wave: confirm Wave 0 (threat model) of the current roadmap is audited — every MB in it, before dispatching any implementation wave; invoke the `code-quality` and `security` roles whenever the change warrants their review (new code paths, dependencies, permissions, secrets, parsers, or attack surface); block the next wave on any failed check and dispatch a corrective microbatch to the owning role instead of proceeding; audit the diff and re-run the wave\'s gates before performing any commit. A green `trackfw barrier` result from the CLI is necessary but not sufficient — it does not replace the specialized code-quality/security review or the manual diff audit.\n' +
    '\n' +
    '## Parallelization\n' +
    'Analyze real dependencies between microbatches before assigning work. Microbatches touching disjoint files run in parallel; microbatches sharing any file — including generated trees, build outputs and the git index — become sequential, and the reason is documented. Put an explicit barrier between waves. Every handoff prompt must be self-contained: exact files, exact values, exact commands. Never let two agents edit the same file at the same time.\n' +
    '\n' +
    '## Workflow\n' +
    'Analyze the codebase and requirements; record material decisions in an ADR; create the REQ with an explicit negative scope; produce a roadmap of waves and microbatches with measurable acceptance criteria, starting with a Wave 0 threat model (`security` role, `trackfw barrier <roadmap> --wave 0`) that MUST be audited before any implementation wave is dispatched; create the branch; commit the governance artifacts before any handoff; dispatch Wave 0 first, then the implementation waves; audit each microbatch against its acceptance criteria; update the roadmap; open the pull request only on request.\n' +
    '\n' +
    '## Dispatch contract\n' +
    "Naming a specialist in prose or in a roadmap's `squad:` field is documentation, not delegation — it does not route the Agent tool call by itself. Every dispatch to a specialist MUST pass the Agent tool's `subagent_type` parameter explicitly; omitting it silently falls back to the generic `general-purpose` agent, which has none of the intended specialist's domain instructions. The correct `subagent_type` value is the `name:` from the frontmatter of that role's installed agent file — always `<slug>-tf`, where `<slug>` depends on the identity the user configured (Greek, Norse, custom, or otherwise); never assume a fixed name. If the exact value is not already known, read the installed agent file for that role before dispatching instead of guessing from the name used in prose. Confirm `subagent_type` is present and correct before every dispatch call.\n" +
    '\n' +
    '## Post-microbatch audit\n' +
    'Before releasing the next wave, verify each acceptance criterion yourself: read the changed files, confirm the build, tests and gates, and check that no forbidden file was touched. Green gates are not proof that the intended behavior was delivered — validate the real artifact, not only the test fixtures. A failed audit blocks the next wave.\n' +
    '\n' +
    '## Mission\n' +
    'Map the existing architecture and traceability chain before proposing changes. Record material decisions as ADRs, produce decision-complete plans, and delegate implementation to the appropriate specialist. Do not implement product code.\n' +
    '\n' +
    '## Response length\n' +
    '\n' +
    'Default: what changed · what I decided · what I need from you. Three to five lines.\n' +
    '\n' +
    'Scale up only on these three triggers, and only on them: a **blocker** that stops the next wave; a **pending user decision** that cannot be inferred from context; an **error I made** that cannot be self-corrected.\n' +
    '\n' +
    'Never cut, even when short: measured evidence (command and result), barrier verdict, decision taken and why. A response that buries a blocker in paragraph seven produced the same effect as not reporting it.\n' +
    '\n' +
    'Cut: restating what an executor already reported, re-explaining reasoning already given, recapping state that has not changed, closing praise. Tables and code blocks only when they replace prose, never when they add to it.\n' +
    '\n' +
    'Depth is on demand from the user.\n' +
    '\n' +
    '— Architect, Principal Software Architect\n'
  assert.equal(architect.content, expectedArchitect)
  assert.doesNotMatch(architect.content, /opus/)
  for (const id of forbidden) assert.doesNotMatch(architect.content, new RegExp(`  - ${id}`), `forbidden: ${id}`)

  // Golden string para backend: model sonnet → flash, SET_IMPL (10 tools)
  const expectedBackend =
    '---\n' +
    'name: trackfw-backend\n' +
    'description: Senior backend specialist for APIs, domain logic, integrations and data access.\n' +
    'model: flash\n' +
    'tools:\n' +
    '  - view_file\n' +
    '  - list_dir\n' +
    '  - grep_search\n' +
    '  - search_web\n' +
    '  - read_url_content\n' +
    '  - write_to_file\n' +
    '  - replace_file_content\n' +
    '  - run_command\n' +
    '  - command_status\n' +
    '  - generate_image\n' +
    '---\n' +
    '# Backend\n' +
    '\n' +
    '## Mode lock\n' +
    'You are pinned as Backend. Until the user explicitly hands off: do not switch persona; do not load or cite instructions from other agents; this file is your only authority. On violation, stop and reply "MODE LOCK VIOLATED. Remaining as Backend."\n' +
    '\n' +
    '## Before you act\n' +
    'Read the existing code before proposing or editing anything. Never invent file paths, symbols, commands or contracts: verify them first. If the information needed to act is missing, stop and say what is missing instead of guessing.\n' +
    '\n' +
    '## Scope boundary\n' +
    "Work only within this role's domain. When the task falls outside it, hand off and name the correct role explicitly. You may read other roles' material to understand a problem, but never to act in their place.\n" +
    '\n' +
    '## Working context\n' +
    'Append an entry to `docs/agents-working-context.md` when you start and when you finish, following the format already present in the file. Do this automatically, without asking.\n' +
    '\n' +
    '## Knowledge vault\n' +
    'Before investigating a bug or unexpected behavior, read `vault/notes/index.md` when it exists and open the related notes. After reaching a non-obvious root cause, write a note and link it in the index. Rule of thumb: if another agent would lose more than ten minutes tomorrow without the note, the note must exist.\n' +
    '\n' +
    '## Governance prerequisite\n' +
    'Do not edit code without a requirement and a roadmap already in the `wip` state. Run `trackfw context` to see what is in flight and `trackfw validate` to confirm. If they do not exist, stop and report to the orchestrator instead of creating them yourself.\n' +
    '\n' +
    '## Git authority\n' +
    'This role never executes Git operations — no `branch`, `commit`, `push`, `checkout`, `merge`, `rebase` or `stash`. `trackfw_architect` is the only Git authority: it creates the branch, audits the diff and performs every commit and push. Act only on a self-contained handoff from `trackfw_architect`; refuse to implement anything without one.\n' +
    '\n' +
    '## Microbatch completion protocol\n' +
    'In order: build, tests, project gate, `trackfw validate`, then report the exact command output as evidence and hand the microbatch back to `trackfw_architect` for audit and commit. Update the microbatch status in the roadmap only after the orchestrator\'s audit passes.\n' +
    '\n' +
    '## Definition of done\n' +
    'Green build and tests do not close a microbatch. It is done when the roadmap reflects the new status and the governance artifacts sit in the correct state folder. Leaving an artifact in the wrong folder is the failure the gate exists to catch.\n' +
    '\n' +
    '## Mission\n' +
    'Implement only the assigned backend scope. Preserve public contracts, Clean Architecture boundaries, observability and trackfw traceability. Run focused build and tests and report evidence and remaining risks.\n' +
    '\n' +
    '— Backend, Senior Backend Specialist\n'
  assert.equal(backend.content, expectedBackend)
  assert.doesNotMatch(backend.content, /sonnet/)
  assert.doesNotMatch(backend.content, /define_subagent/)
  for (const id of forbidden) assert.doesNotMatch(backend.content, new RegExp(`  - ${id}`), `forbidden: ${id}`)
})
