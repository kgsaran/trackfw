'use strict'
// Tests for ML-2B (Node): `trackfw agents install` registers the installed
// agent in trackfw.yaml's `agents:` key (AC1, AC2, AC3, AC8).
//
// Each test carries a // Reconciliation sentence per CLAUDE.md "Regra Dura
// de Reconciliação": "if you cannot write the sentence, the test should not
// exist."

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { execute } = require('../src/integrations')
const config = require('../src/config/index.js')

// ── helpers ──────────────────────────────────────────────────────────────────

/**
 * Create a temp dir with an optional trackfw.yaml.
 * Returns { projectRoot, homeRoot } with realpathSync applied (macOS
 * /var → /private/var fix, per handoff caveat).
 */
function makeProject(yaml) {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-ml2b-'))
  // mkdirSync before realpathSync — /var → /private/var requires the path to exist
  const projectRaw = path.join(base, 'project')
  const homeRaw = path.join(base, 'home')
  fs.mkdirSync(projectRaw)
  fs.mkdirSync(homeRaw)
  const projectRoot = fs.realpathSync(projectRaw)
  const homeRoot = fs.realpathSync(homeRaw)
  if (yaml !== undefined) {
    fs.writeFileSync(path.join(projectRoot, 'trackfw.yaml'), yaml, 'utf8')
  }
  return { projectRoot, homeRoot }
}

/**
 * Run `execute('agents', 'install', ...)` scoped to a single target and item
 * so we install exactly one known catalog agent.
 */
function installAgent(roots, agentId, scope = 'project') {
  config.reset()
  execute(
    'agents',
    'install',
    { targets: ['claude'], items: [agentId], scope, projectRoot: roots.projectRoot },
    roots
  )
  config.reset()
}

/**
 * Read agents: from the file (returns [] if key absent or file missing).
 */
function readAgents(projectRoot) {
  const cfgPath = path.join(projectRoot, 'trackfw.yaml')
  if (!fs.existsSync(cfgPath)) return []
  const { parseDocument } = require('yaml')
  const doc = parseDocument(fs.readFileSync(cfgPath, 'utf8'))
  const node = doc.getIn(['agents'], true)
  if (!node) return []
  return node.items.map(i => i.value)
}

/**
 * Return true if the raw file text contains the `agents:` key at top level.
 */
function hasAgentsKey(projectRoot) {
  const cfgPath = path.join(projectRoot, 'trackfw.yaml')
  if (!fs.existsSync(cfgPath)) return false
  return /^agents:/m.test(fs.readFileSync(cfgPath, 'utf8'))
}

// ── AC1 — idempotency ────────────────────────────────────────────────────────

test('AC1: two consecutive installs of the same agent yield exactly one entry', () => {
  // Reconciliation: affirms that the idempotency guard in registerAgentInConfig
  // prevents duplicate entries in agents: when the same agent is installed twice.
  const { projectRoot, homeRoot } = makeProject(
    'roadmap_namespacing: by_agent\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n'
  )
  try {
    installAgent({ projectRoot, homeRoot }, 'backend')
    installAgent({ projectRoot, homeRoot }, 'backend')
    const agents = readAgents(projectRoot)
    assert.equal(agents.filter(a => a === 'backend').length, 1,
      `expected exactly one "backend" in agents:, got: ${JSON.stringify(agents)}`)
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})

// ── AC2 — flat mode must not create the key ───────────────────────────────────

test('AC2: install in a flat project does not create agents: key', () => {
  // Reconciliation: affirms that registerAgentInConfig is a no-op when
  // roadmap_namespacing is "flat", ensuring the key is absent after install.
  const yaml = 'roadmap_namespacing: flat\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n'
  const { projectRoot, homeRoot } = makeProject(yaml)
  try {
    installAgent({ projectRoot, homeRoot }, 'backend')
    assert.equal(hasAgentsKey(projectRoot), false,
      'agents: key must not exist after install in flat project')
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})

// ── AC3 — format preservation ─────────────────────────────────────────────────

test('AC3: only the agents: block changes after install (comments and key order preserved)', () => {
  // Reconciliation: affirms that parseDocument/toString round-trip writes only
  // the added agents: line(s), leaving all other lines (comments, non-alphabetical
  // key order, quoted values) byte-for-byte identical.
  //
  // Fixture has: a top comment, req_dir before roadmap_namespacing (non-alphabetical),
  // a quoted string value, no agents: block yet.
  const before = [
    '# project config',
    "req_dir: 'docs/req'",       // quoted value, non-alphabetical position
    'roadmap_namespacing: by_agent',
    'roadmap_dir: docs/roadmaps',
    '',
  ].join('\n')

  const { projectRoot, homeRoot } = makeProject(before)
  try {
    installAgent({ projectRoot, homeRoot }, 'architect')

    const after = fs.readFileSync(path.join(projectRoot, 'trackfw.yaml'), 'utf8')
    const beforeLines = before.split('\n')
    const afterLines = after.split('\n')

    // Derive which lines changed
    const changedLines = afterLines.filter(line => !beforeLines.includes(line))

    // Every changed line must belong to the agents: block
    // (either the "agents:" header itself or "  - <name>" entries)
    for (const line of changedLines) {
      const isAgentsBlock = line === 'agents:' || /^\s+-\s+\S+/.test(line)
      assert.ok(isAgentsBlock,
        `Unexpected change outside agents: block: ${JSON.stringify(line)}`)
    }

    // The agent actually appears in the list
    const agents = readAgents(projectRoot)
    assert.ok(agents.includes('architect'),
      `"architect" must appear in agents:, got: ${JSON.stringify(agents)}`)

    // Comments, quoted values and non-alphabetical key order survive intact
    assert.ok(after.includes('# project config'), 'top comment must be preserved')
    assert.ok(after.includes("req_dir: 'docs/req'"), 'quoted value must be preserved')
    assert.ok(after.indexOf('req_dir') < after.indexOf('roadmap_namespacing'),
      'non-alphabetical key order must be preserved (req_dir before roadmap_namespacing)')
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})

// ── AC8 — falsification: both directions ──────────────────────────────────────

test('AC8-a (by_agent): installed agent appears in agents:', () => {
  // Reconciliation: affirms that execute('agents','install',...) with a
  // by_agent project causes registerAgentInConfig to write the agent name
  // into the agents: key — the positive falsification arm.
  const { projectRoot, homeRoot } = makeProject(
    'roadmap_namespacing: by_agent\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n'
  )
  try {
    installAgent({ projectRoot, homeRoot }, 'qa')
    const agents = readAgents(projectRoot)
    assert.ok(agents.includes('qa'),
      `"qa" must appear in agents: after install in by_agent project; got: ${JSON.stringify(agents)}`)
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})

test('AC8-b (flat): agents: key is absent after install', () => {
  // Reconciliation: affirms that execute('agents','install',...) with a flat
  // project does NOT create the agents: key — the negative falsification arm.
  const yaml = 'roadmap_namespacing: flat\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n'
  const { projectRoot, homeRoot } = makeProject(yaml)
  try {
    installAgent({ projectRoot, homeRoot }, 'qa')
    assert.equal(hasAgentsKey(projectRoot), false,
      'agents: key must be absent after install in flat project')
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})

// ── additional coverage: by_agent with existing agents: list ─────────────────

test('registers new agent alongside existing entries without disturbing them', () => {
  // Reconciliation: affirms that when agents: already has entries, the new
  // agent is appended and the existing entries are untouched.
  const { projectRoot, homeRoot } = makeProject(
    'roadmap_namespacing: by_agent\nagents:\n  - alpha\n  - beta\nroadmap_dir: docs/roadmaps\n'
  )
  try {
    installAgent({ projectRoot, homeRoot }, 'infra')
    const agents = readAgents(projectRoot)
    assert.ok(agents.includes('alpha'), '"alpha" must still be present')
    assert.ok(agents.includes('beta'), '"beta" must still be present')
    assert.ok(agents.includes('infra'), '"infra" must be added')
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})

test('does not write agents: when trackfw.yaml is absent', () => {
  // Reconciliation: affirms that registerAgentInConfig is a safe no-op when
  // there is no trackfw.yaml, so installs in un-configured projects don't fail.
  const { projectRoot, homeRoot } = makeProject() // no yaml
  try {
    // Must not throw
    installAgent({ projectRoot, homeRoot }, 'backend')
    assert.equal(hasAgentsKey(projectRoot), false,
      'no trackfw.yaml created when it did not previously exist')
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})

test('global-scope install does not write project trackfw.yaml', () => {
  // Reconciliation: affirms that when scope is "global", registerAgentInConfig
  // is not called for the project trackfw.yaml — global installs don't own
  // the project config.
  const yaml = 'roadmap_namespacing: by_agent\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n'
  const { projectRoot, homeRoot } = makeProject(yaml)
  try {
    installAgent({ projectRoot, homeRoot }, 'security', 'global')
    const agents = readAgents(projectRoot)
    assert.equal(agents.includes('security'), false,
      'global-scope install must not register agent in project trackfw.yaml')
  } finally {
    fs.rmSync(path.dirname(projectRoot), { recursive: true, force: true })
  }
})
