'use strict'

const fs = require('fs')
const os = require('os')
const path = require('path')
const identityStore = require('../identity')
const { catalog, buildPlans, IntegrationManager, globalGroupPath } = require('../integrations')
const { tildeify, validateTargets, buildDocument, humanReport, silenceConsole } = require('../lib/update-engine')

// `trackfw update harness` is the global counterpart to `trackfw update` —
// see docs/cli-parity.md, "`trackfw update` vs `trackfw update harness`".
// It never requires trackfw.yaml or a project cwd, and its only job is to
// update rules/agents/skills ALREADY INSTALLED under the user's home
// directory (~/.claude and equivalents). It never installs a target that is
// not already present unless --install-missing is passed.
//
// Target universe (fixed declared order, mirrors internal/commands/
// update_harness.go and pypi/trackfw/commands/update_harness.py):
//   1. claude-skill      — ~/.claude/skills/trackfw/SKILL.md (legacy
//                          governance meta-skill, previously written by the
//                          removed `update` step "skill global"; this is the
//                          exact target named in the frozen contract
//                          example in docs/cli-parity.md).
//   2..N. <tool>-agents / <tool>-skills — one target per (catalog target,
//                          kind) pair, in catalog.targets declared order,
//                          covering the project-independent, home-rooted
//                          agents/skills catalog (npm/src/integrations).
//
// Ambiguity (reported, not resolved unilaterally — see ML-6C handoff
// report): each <tool>-<kind> target aggregates potentially many catalog
// items (e.g. claude-agents bundles all 12 agent personas) into a single
// state. The contract does not specify item-level granularity for harness
// targets, so this implementation reports the bundle-level outcome:
//   - every item not-installed             -> missing
//   - any item unmanaged/modified          -> skipped (never overwritten)
//   - otherwise, at least one item written -> updated
//   - otherwise (all current)              -> skipped

// claudeSkillContent — mirrors generators/init.js:installSkillsForce
// (npm/src/generators/init.js:1242) byte-for-byte. Duplicated here, not
// imported, because installSkillsForce() bakes os.homedir() internally with
// an unconditional overwrite and no dry-run mode — it cannot be reused
// without either mutating on --dry-run or forking its signature. Reported
// as a follow-up cleanup opportunity in the ML-6C handoff report.
function claudeSkillContent() {
  return `---
name: trackfw
description: "trackfw — Governed Software Delivery: ADR → REQ → ROADMAP → kanban"
signature: "📦 trackfw - Governed Delivery"
---

# trackfw — Modo de Operação

Este projeto usa **trackfw** para governança de entrega de software.
Cadeia: **ADR → REQ → ROADMAP** · Estados: \`backlog / wip / blocked / done / abandoned\`

## Comandos principais

- \`trackfw context\` — contexto de trabalho atual (sempre execute primeiro)
- \`trackfw status\` — todos os artefatos e estados
- \`trackfw validate\` — valida consistência de governança
- \`trackfw roadmap move <nome> <estado>\` — transição de estado
- \`trackfw serve\` — board Kanban em http://localhost:4080

## Protocolo de agente

1. Antes de iniciar: \`trackfw context\` + ler \`docs/agents-working-context.md\`
2. Após concluir: atualizar \`docs/agents-working-context.md\`
3. Antes de PR: \`trackfw validate\` deve passar com zero violations
`
}

function claudeSkillTarget(homeRoot, { dryRun, installMissing }) {
  const id = 'claude-skill'
  const filePath = path.join(homeRoot, '.claude', 'skills', 'trackfw', 'SKILL.md')
  const displayPath = tildeify(homeRoot, filePath)
  const desired = claudeSkillContent()

  try {
    const exists = fs.existsSync(filePath)
    const actual = exists ? fs.readFileSync(filePath, 'utf8') : null

    if (!exists && !installMissing) return { id, state: 'missing', path: displayPath }
    if (exists && actual === desired) return { id, state: 'skipped', path: displayPath }

    if (dryRun) return { id, state: 'updated', path: displayPath }

    fs.mkdirSync(path.dirname(filePath), { recursive: true })
    fs.writeFileSync(filePath, desired, 'utf8')
    return { id, state: 'updated', path: displayPath }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }
}

// catalogBundleTarget — one target per (tool, kind) pair at global scope.
// Uses IntegrationManager.inspect (read-only) to classify every catalog
// item under that pair, then only calls manager.update() for the subset
// that needs a write — and never for --dry-run. displayPath is derived from
// the catalog itself (globalGroupPath), not from any individual plan's
// destination, so it never depends on catalog item iteration order (this is
// what previously caused the claude-skills path to diverge from the Python
// CLI — see docs/cli-parity.md, "Declared harness targets — pinned list").
function catalogBundleTarget(toolId, kind, homeRoot, identityConfig, { dryRun, installMissing }) {
  const id = `${toolId}-${kind}`
  let displayPath = `~/.${toolId}`
  try {
    displayPath = globalGroupPath(toolId, kind)
    const plans = buildPlans(kind, { targets: [toolId], scope: 'global', identity: identityConfig })
    if (!plans.length) return { id, state: 'missing', path: displayPath }

    const manager = new IntegrationManager({ homeRoot })
    const statuses = manager.inspect(plans)

    const allNotInstalled = statuses.every((s) => s.state === 'not-installed')
    const anyModified = statuses.some((s) => s.state === 'modified')

    if (allNotInstalled && !installMissing) return { id, state: 'missing', path: displayPath }

    const toWrite = plans.filter((_, index) => {
      const state = statuses[index].state
      if (state === 'outdated') return true
      if (state === 'not-installed') return installMissing
      return false
    })

    if (!toWrite.length) {
      if (allNotInstalled) return { id, state: 'missing', path: displayPath }
      return { id, state: 'skipped', path: displayPath }
    }

    if (!dryRun) manager.update(toWrite)
    return { id, state: 'updated', path: displayPath }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }
}

const HARNESS_TARGET_IDS = ['claude-skill']
for (const target of catalog.targets) {
  HARNESS_TARGET_IDS.push(`${target.id}-agents`, `${target.id}-skills`)
}

// buildHarnessTargets — `wanted` (nullable) restricts which targets are
// even computed/applied. This must happen HERE, not as a post-hoc filter
// on the returned array: catalogBundleTarget/claudeSkillTarget call
// manager.update() as a side effect, so filtering after construction would
// still have mutated every unrequested target's files on disk.
function buildHarnessTargets(homeRoot, identityConfig, { dryRun, installMissing }, wanted) {
  const include = (id) => !wanted || wanted.includes(id)
  const targets = []
  if (include('claude-skill')) targets.push(claudeSkillTarget(homeRoot, { dryRun, installMissing }))
  for (const target of catalog.targets) {
    const agentsId = `${target.id}-agents`
    const skillsId = `${target.id}-skills`
    if (include(agentsId)) targets.push(catalogBundleTarget(target.id, 'agents', homeRoot, identityConfig, { dryRun, installMissing }))
    if (include(skillsId)) targets.push(catalogBundleTarget(target.id, 'skills', homeRoot, identityConfig, { dryRun, installMissing }))
  }
  return targets
}

// run — entry point invoked by `trackfw update harness`. Deliberately a
// plain function, not its own commander.Command: nesting a Command that
// redeclares the SAME flag names (--json, --dry-run, --targets,
// --install-missing) as its parent ('update') triggers a commander@12
// parsing quirk where the flag binds to the ANCESTOR command's opts()
// instead of the child's, silently producing `{}` in the child action no
// matter what was passed on the command line (reproduced and confirmed in
// isolation while building this ML — see the vault note this ML links).
// `update.js` instead parses `update harness --json` as ITS OWN single
// command with an optional positional `[mode]` argument, so there is only
// ever one Option object per flag name; it calls `run(options)` here when
// `mode === 'harness'`.
function run(options) {
  let wanted
  try {
    const requested = options.targets ? String(options.targets).split(',').map((s) => s.trim()).filter(Boolean) : []
    wanted = validateTargets(HARNESS_TARGET_IDS, requested)
  } catch (e) {
    console.error(`✗ ${e.message}`)
    process.exit(1)
  }

  const homeRoot = os.homedir()
  const dryRun = Boolean(options.dryRun)
  const installMissing = Boolean(options.installMissing)

  // Identidade resolvida do disco antes de buildPlans — pular esta etapa
  // reverteria silenciosamente os nomes customizados para os defaults
  // neutros (mesma justificativa de npm/src/integrations/index.js:execute).
  const identityConfig = identityStore.load(homeRoot)

  const targets = options.json
    ? silenceConsole(() => buildHarnessTargets(homeRoot, identityConfig, { dryRun, installMissing }, wanted))
    : buildHarnessTargets(homeRoot, identityConfig, { dryRun, installMissing }, wanted)

  const doc = buildDocument('harness', dryRun, targets)

  if (options.json) {
    console.log(JSON.stringify(doc, null, 2))
  } else {
    console.log(humanReport('harness', dryRun, targets))
  }

  if (doc.summary.failed > 0) process.exitCode = 1
}

module.exports = { run, HARNESS_TARGET_IDS, buildHarnessTargets, claudeSkillContent }
