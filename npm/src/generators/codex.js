'use strict'

const fs = require('fs')
const path = require('path')
const { injectCodexHooks } = require('./hooks')
const { injectRulesForTool } = require('./init')

const skills = {
  'trackfw-governance': `---
name: trackfw-governance
description: Inspect or maintain ADR, REQ, ROADMAP, traceability, lifecycle state, and trackfw validation. Use for governance questions and artifact changes.
---
Run \`trackfw context --format=json\`, preserve the ADR → REQ → ROADMAP chain, and finish with \`trackfw validate --json\`.
`,
  'trackfw-plan': `---
name: trackfw-plan
description: Plan a trackfw-governed implementation before code changes. Use for ADRs, REQs, roadmaps, microbatches, acceptance criteria, and validation commands.
---
Inspect existing artifacts and code. Create required decisions and requirements before a decision-complete roadmap. Keep the roadmap in analyzing until coding starts.
`,
  'trackfw-implement': `---
name: trackfw-implement
description: Implement work governed by an existing trackfw roadmap. Use for features and fixes that must follow lifecycle and validation gates.
---
Identify the matching REQ and roadmap, move it to wip, implement one microbatch at a time, run project tests and \`trackfw validate --json\`, then update lifecycle state.
`,
  'trackfw-review': `---
name: trackfw-review
description: Review a change for governance traceability, correctness, security, regressions, missing tests, and roadmap acceptance.
---
Stay read-only unless fixes are requested. Report evidence-backed findings by severity and verify the linked REQ, roadmap, acceptance criteria, tests, and validation output.
`,
  'trackfw-release': `---
name: trackfw-release
description: Prepare and verify a trackfw release. Use for versioning, changelog, packaging, parity, and publication gates.
---
Run all quality and parity gates, derive SemVer from commits, and stop before tags or publication unless explicitly authorized.
`,
}

const agents = {
  'trackfw-architect.toml': `name = "trackfw_architect"
description = "Architecture and implementation-planning specialist for governed changes."
sandbox_mode = "read-only"
developer_instructions = """
Map architecture, contracts, constraints, and traceability. Require ADRs for unresolved material decisions. Produce decision-complete plans and do not edit files unless explicitly assigned.
"""
`,
  'trackfw-backend.toml': `name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """
Implement only the assigned backend scope, preserve contracts and traceability, run focused tests, and report changed files and evidence.
"""
`,
  'trackfw-frontend.toml': `name = "trackfw_frontend"
description = "Frontend implementation specialist focused on accessibility, i18n, UX consistency, and browser behavior."
developer_instructions = """
Implement only the assigned frontend scope, preserve accessibility and localization, and run focused build and UI tests.
"""
`,
  'trackfw-qa.toml': `name = "trackfw_qa"
description = "Read-only QA specialist for regressions, flaky tests, critical flows, and acceptance coverage."
sandbox_mode = "read-only"
developer_instructions = """
Trace critical flows and compare behavior against roadmap acceptance criteria. Report reproducible failures and test gaps.
"""
`,
  'trackfw-security.toml': `name = "trackfw_security"
description = "Read-only security reviewer for trust boundaries, secrets, injection, permissions, and unsafe defaults."
sandbox_mode = "read-only"
developer_instructions = """
Perform evidence-backed threat analysis. Report concrete exploit paths and mitigations by severity.
"""
`,
  'trackfw-reviewer.toml': `name = "trackfw_reviewer"
description = "Owner-level reviewer for correctness, governance, maintainability, security, and missing tests."
sandbox_mode = "read-only"
developer_instructions = """
Review the branch as an owner. Return findings first, ordered by severity, with file references and governance evidence.
"""
`,
}

function installCodex(cwd) {
  const root = cwd || process.cwd()
  const { execute } = require('../integrations')
  execute('agents', 'install', { targets: ['codex'], scope: 'project' }, { projectRoot: root })
  execute('skills', 'install', { targets: ['codex'], scope: 'project' }, { projectRoot: root })
  injectRulesForTool('codex', root)

  const configPath = path.join(root, '.codex', 'config.toml')
  fs.mkdirSync(path.dirname(configPath), { recursive: true })
  let config = fs.existsSync(configPath) ? fs.readFileSync(configPath, 'utf8') : ''
  if (!/^\[agents\]\s*$/m.test(config)) {
    config = config.replace(/\s*$/, '') + '\n\n[agents]\nmax_threads = 6\nmax_depth = 1\n'
    fs.writeFileSync(configPath, config, 'utf8')
  }

  injectCodexHooks(root)
  console.log('  ✓ Codex: AGENTS.md, skills, custom agents and hooks')
}

module.exports = { installCodex, legacyCodexFixtures: { skills, agents } }
