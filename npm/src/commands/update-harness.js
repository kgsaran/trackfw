'use strict'

const fs = require('fs')
const os = require('os')
const path = require('path')
const identityStore = require('../identity')
const { catalog, buildPlans, IntegrationManager, globalGroupPath } = require('../integrations')
const { tildeify, validateTargets, buildDocument, humanReport, silenceConsole } = require('../lib/update-engine')
const { mergeClaudeHookArray } = require('../generators/hooks')

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

// credentialGuardTargetClaude — evaluates (and, unless --dry-run, applies) the
// global-scope credential-guard hook wiring for Claude Code:
// PreToolUse[matcher:"Bash"]/PostToolUse[matcher:"Bash"] entries in
// ~/.claude/settings.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh (a global hook can fire from
// any project's cwd, unlike the project-scope wiring's relative
// "scripts/trackfw-credential-guard.sh"). Mirrors
// internal/generators/update.go:harnessCredentialGuardTargetClaude —
// including deliberately NOT calling generateGlobalCredentialGuardScript here
// (writing the referenced script file itself is out of this target's scope,
// same as the Go implementation). Reuses mergeClaudeHookArray (generators/
// hooks.js) so any pre-existing content in ~/.claude/settings.json (other
// hooks, user config) is preserved — only the credential-guard entry is
// added/merged.
function credentialGuardTargetClaude(homeRoot, { dryRun, installMissing }) {
  const id = 'claude-credential-guard'
  const filePath = path.join(homeRoot, '.claude', 'settings.json')
  const displayPath = tildeify(homeRoot, filePath)
  const scriptPath = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')

  let root
  let raw = null
  try {
    if (fs.existsSync(filePath)) {
      raw = fs.readFileSync(filePath, 'utf8')
      root = raw.length ? JSON.parse(raw) : {}
    } else {
      if (!installMissing) return { id, state: 'missing', path: displayPath }
      if (dryRun) return { id, state: 'updated', path: displayPath }
      root = {}
      if (!root.hooks) root.hooks = {}
      root.hooks.PreToolUse = mergeClaudeHookArray(root.hooks.PreToolUse, 'Bash', scriptPath)
      root.hooks.PostToolUse = mergeClaudeHookArray(root.hooks.PostToolUse, 'Bash', scriptPath)
      const desired = JSON.stringify(root, null, 2) + '\n'
      fs.mkdirSync(path.dirname(filePath), { recursive: true })
      fs.writeFileSync(filePath, desired, 'utf8')
      return { id, state: 'updated', path: displayPath }
    }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }

  try {
    if (!root.hooks) root.hooks = {}
    root.hooks.PreToolUse = mergeClaudeHookArray(root.hooks.PreToolUse, 'Bash', scriptPath)
    root.hooks.PostToolUse = mergeClaudeHookArray(root.hooks.PostToolUse, 'Bash', scriptPath)
    const desired = JSON.stringify(root, null, 2) + '\n'
    if (desired === raw) return { id, state: 'skipped', path: displayPath }
    if (dryRun) return { id, state: 'updated', path: displayPath }
    fs.writeFileSync(filePath, desired, 'utf8')
    return { id, state: 'updated', path: displayPath }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }
}

// credentialGuardTargetCodex — evaluates (and, unless --dry-run, applies) the
// global-scope credential-guard hook wiring for Codex CLI:
// PreToolUse[matcher:"Bash"]/PostToolUse[matcher:"Bash"] entries in
// ~/.codex/hooks.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh — mirrors
// credentialGuardTargetClaude exactly (same 4-state contract, same idempotent
// merge via mergeClaudeHookArray, same reason for an absolute path). Mirrors
// internal/generators/update.go:harnessCredentialGuardTargetCodex.
//
// Investigation (ROADMAP-2026-08-06 Wave 2/ML-2B, confirmed 2026-08-06
// against https://developers.openai.com/codex/hooks): "Hooks are enabled by
// default. To turn them off in config.toml, set: [features] hooks = false.
// Use hooks as the canonical feature key. codex_hooks still works as a
// deprecated alias." No `[features] codex_hooks = true` opt-in is required
// — the flag exists only to turn hooks OFF and is a deprecated alias for the
// canonical `hooks` key. https://developers.openai.com/codex/config-advanced
// (also fetched 2026-08-06) has no conflicting requirement.
function credentialGuardTargetCodex(homeRoot, { dryRun, installMissing }) {
  const id = 'codex-credential-guard'
  const filePath = path.join(homeRoot, '.codex', 'hooks.json')
  const displayPath = tildeify(homeRoot, filePath)
  const scriptPath = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')

  let root
  let raw = null
  try {
    if (fs.existsSync(filePath)) {
      raw = fs.readFileSync(filePath, 'utf8')
      root = raw.length ? JSON.parse(raw) : {}
    } else {
      if (!installMissing) return { id, state: 'missing', path: displayPath }
      if (dryRun) return { id, state: 'updated', path: displayPath }
      root = {}
      if (!root.hooks) root.hooks = {}
      root.hooks.PreToolUse = mergeClaudeHookArray(root.hooks.PreToolUse, 'Bash', scriptPath)
      root.hooks.PostToolUse = mergeClaudeHookArray(root.hooks.PostToolUse, 'Bash', scriptPath)
      const desired = JSON.stringify(root, null, 2) + '\n'
      fs.mkdirSync(path.dirname(filePath), { recursive: true })
      fs.writeFileSync(filePath, desired, 'utf8')
      return { id, state: 'updated', path: displayPath }
    }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }

  try {
    if (!root.hooks) root.hooks = {}
    root.hooks.PreToolUse = mergeClaudeHookArray(root.hooks.PreToolUse, 'Bash', scriptPath)
    root.hooks.PostToolUse = mergeClaudeHookArray(root.hooks.PostToolUse, 'Bash', scriptPath)
    const desired = JSON.stringify(root, null, 2) + '\n'
    if (desired === raw) return { id, state: 'skipped', path: displayPath }
    if (dryRun) return { id, state: 'updated', path: displayPath }
    fs.writeFileSync(filePath, desired, 'utf8')
    return { id, state: 'updated', path: displayPath }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }
}

// credentialGuardTargetGemini — evaluates (and, unless --dry-run, applies)
// the global-scope credential-guard hook wiring for Gemini CLI:
// BeforeTool[matcher:"run_shell_command"]/AfterTool[matcher:"run_shell_command"]
// entries in ~/.gemini/settings.json pointing at the ABSOLUTE path of
// ~/.trackfw/scripts/trackfw-credential-guard.sh — mirrors
// credentialGuardTargetClaude/credentialGuardTargetCodex exactly (same
// 4-state contract, same idempotent merge via mergeClaudeHookArray), only the
// top-level event key names differ (BeforeTool/AfterTool instead of
// PreToolUse/PostToolUse, and matcher "run_shell_command" instead of "Bash")
// since Gemini CLI uses a different hook vocabulary than Claude/Codex (see
// generators/hooks.js:injectGeminiHooks, confirmed against
// https://geminicli.com/docs/hooks/reference). Mirrors
// internal/generators/update.go:harnessCredentialGuardTargetGemini.
function credentialGuardTargetGemini(homeRoot, { dryRun, installMissing }) {
  const id = 'gemini-credential-guard'
  const filePath = path.join(homeRoot, '.gemini', 'settings.json')
  const displayPath = tildeify(homeRoot, filePath)
  const scriptPath = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-credential-guard.sh')

  let root
  let raw = null
  try {
    if (fs.existsSync(filePath)) {
      raw = fs.readFileSync(filePath, 'utf8')
      root = raw.length ? JSON.parse(raw) : {}
    } else {
      if (!installMissing) return { id, state: 'missing', path: displayPath }
      if (dryRun) return { id, state: 'updated', path: displayPath }
      root = {}
      if (!root.hooks) root.hooks = {}
      root.hooks.BeforeTool = mergeClaudeHookArray(root.hooks.BeforeTool, 'run_shell_command', scriptPath)
      root.hooks.AfterTool = mergeClaudeHookArray(root.hooks.AfterTool, 'run_shell_command', scriptPath)
      const desired = JSON.stringify(root, null, 2) + '\n'
      fs.mkdirSync(path.dirname(filePath), { recursive: true })
      fs.writeFileSync(filePath, desired, 'utf8')
      return { id, state: 'updated', path: displayPath }
    }
  } catch (e) {
    return { id, state: 'failed', path: displayPath, message: e.message }
  }

  try {
    if (!root.hooks) root.hooks = {}
    root.hooks.BeforeTool = mergeClaudeHookArray(root.hooks.BeforeTool, 'run_shell_command', scriptPath)
    root.hooks.AfterTool = mergeClaudeHookArray(root.hooks.AfterTool, 'run_shell_command', scriptPath)
    const desired = JSON.stringify(root, null, 2) + '\n'
    if (desired === raw) return { id, state: 'skipped', path: displayPath }
    if (dryRun) return { id, state: 'updated', path: displayPath }
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

// HARNESS_TARGET_IDS — mirrors internal/generators/update.go:HarnessTargetIDs.
// "codex-credential-guard"/"gemini-credential-guard" are each inserted
// immediately BEFORE their tool's "-agents"/"-skills" pair (same relative
// position as claude-credential-guard, which precedes claude-agents/
// claude-skills) — see buildHarnessTargetIDs's comment in update.go for the
// full rationale.
const HARNESS_TARGET_IDS = ['claude-skill', 'claude-credential-guard']
for (const target of catalog.targets) {
  if (target.id === 'codex') HARNESS_TARGET_IDS.push('codex-credential-guard')
  if (target.id === 'gemini') HARNESS_TARGET_IDS.push('gemini-credential-guard')
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
  if (include('claude-credential-guard')) targets.push(credentialGuardTargetClaude(homeRoot, { dryRun, installMissing }))
  for (const target of catalog.targets) {
    if (target.id === 'codex' && include('codex-credential-guard')) {
      targets.push(credentialGuardTargetCodex(homeRoot, { dryRun, installMissing }))
    }
    if (target.id === 'gemini' && include('gemini-credential-guard')) {
      targets.push(credentialGuardTargetGemini(homeRoot, { dryRun, installMissing }))
    }
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

module.exports = {
  run,
  HARNESS_TARGET_IDS,
  buildHarnessTargets,
  claudeSkillContent,
  credentialGuardTargetClaude,
  credentialGuardTargetCodex,
  credentialGuardTargetGemini,
}
