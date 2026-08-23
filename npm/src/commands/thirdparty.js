'use strict'

// Node port of internal/commands/integrations_thirdparty.go — the
// `third-party fetch`/`third-party install` two-phase quarantine gate,
// reachable from both `trackfw agents third-party` and
// `trackfw skills third-party` (D1). See:
// docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md

const os = require('os')

const { Command } = require('commander')

const configModule = require('../config')
const identityStore = require('../identity')
const { items: catalogItems } = require('../integrations/catalog')
const { buildPlans, IntegrationManager } = require('../integrations')
const thirdPartyFetchModule = require('../thirdparty/fetch')
const { checkMarkers, checksum: sha256hex } = require('../thirdparty/markers')
const { quarantinePath, newQuarantineEntry, writeQuarantine, readQuarantine, decodeContent } = require('../thirdparty/quarantine')
const { loadProvenance, verifyApproval, upsertProvenanceEntry } = require('../thirdparty/provenance')
const { upsertThirdPartyReference, normalizeThirdPartyContent, resolveThirdPartySkillDestination } = require('../thirdparty/references')

// api is the module.exports object itself, referenced internally via
// `api.thirdPartyFetch(...)` so tests can substitute the network fetch by
// reassigning the exported property in place — mirrors the
// identityWizard.runIdentityWizard monkey-patch pattern already used by
// npm/tests/identity-wizard.test.js, and the package-var substitution used
// by internal/commands/integrations_thirdparty.go:thirdPartyFetch on the Go
// side (tests substitute it instead of hitting real network, keeping
// fetch-command tests compatible with TRACKFW_DISABLE_EXTERNAL_COMMANDS=1).
const api = {}

// THIRD_PARTY_PROVENANCE_RULE is the name of the trackfw validate rule that
// is the real (git-anchored) enforcement behind the orchestrator-session
// guardrail below. Named as a constant so the guardrail message and any
// future test asserting its wording never drift apart silently. Mirrors
// internal/commands/integrations_thirdparty.go:thirdPartyProvenanceRule.
const THIRD_PARTY_PROVENANCE_RULE = 'thirdparty_artifact_has_provenance'

// THIRD_PARTY_GLOBAL_SCOPE_WARNING/REFUSAL are the D4-bis literal strings
// for --scope global: printed/returned VERBATIM by all 3 CLIs (the
// roadmap's AC requires identical wording). Mirrors
// internal/commands/integrations_thirdparty.go:thirdPartyGlobalScopeWarning/thirdPartyGlobalScopeRefusal
// — keep these two strings byte-identical to the Go constants.
const THIRD_PARTY_GLOBAL_SCOPE_WARNING =
  'warning: --scope global installs outside the project tree; this artifact will NEVER be verified by ' +
  '`trackfw validate` (the "' + THIRD_PARTY_PROVENANCE_RULE + '" rule only scans the project\'s own manifest — ' +
  'an artifact under a home directory is invisible to it, per ADR-2026-08-12).'
const THIRD_PARTY_GLOBAL_SCOPE_REFUSAL =
  'install to --scope global requires --yes-global-scope-unverified as its own explicit confirmation (D4-bis), ' +
  'distinct from --yes-i-trust-this-source: it confirms you understand `trackfw validate` will never verify this installation'

// SLUG_PATTERN mirrors internal/commands/integrations_thirdparty.go:thirdPartySlugPattern.
const SLUG_PATTERN = /^[a-z0-9][a-z0-9._-]{0,63}$/

// CHECKSUM_PATTERN mirrors internal/commands/integrations_thirdparty.go:thirdPartyChecksumPattern.
const CHECKSUM_PATTERN = /^[a-f0-9]{64}$/

function csv(value) {
  return String(value).split(',').map(entry => entry.trim()).filter(Boolean)
}

// checkOrchestratorGuardrail implements D2: TRACKFW_ORCHESTRATOR_SESSION is
// a guardrail against accidental invocation from a plain terminal, never a
// security control — it is trivially set by anyone with shell access. The
// real enforcement is the THIRD_PARTY_PROVENANCE_RULE check in `trackfw
// validate` (ML-3A), which is git-anchored per ADR-2026-08-12. This message
// must never present the env var as prevention. Mirrors
// internal/commands/integrations_thirdparty.go:checkOrchestratorGuardrail.
function checkOrchestratorGuardrail() {
  if (process.env.TRACKFW_ORCHESTRATOR_SESSION) return
  throw new Error(
    'refused: TRACKFW_ORCHESTRATOR_SESSION is not set. This is a guardrail against accidental ' +
    'invocation from a plain terminal, not a security control — it does not resist anyone who ' +
    `already has shell access. The real enforcement is the "${THIRD_PARTY_PROVENANCE_RULE}" rule checked by ` +
    '`trackfw validate`, which detects any third-party artifact committed without a matching, checksum-linked ' +
    'provenance entry. If this is an orchestrated agent session, set TRACKFW_ORCHESTRATOR_SESSION=1',
  )
}

function thirdPartyEntryKind(kind) {
  return kind === 'agents' ? 'agent' : 'skill'
}

// deriveSlug produces a filesystem-safe slug from a quarantined artifact's
// source URL when --slug is not given. Mirrors
// internal/commands/integrations_thirdparty.go:deriveSlug.
function deriveSlug(rawURL) {
  let parsed
  try {
    parsed = new URL(rawURL)
  } catch (err) {
    throw new Error(`cannot derive slug from URL "${rawURL}": ${err.message}`)
  }
  let base = parsed.pathname.split('/').filter(Boolean).pop() || ''
  const dot = base.lastIndexOf('.')
  if (dot > 0) base = base.slice(0, dot)
  base = base.toLowerCase()
  let out = ''
  for (const ch of base) {
    if ((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch === '-' || ch === '_' || ch === '.') out += ch
    else out += '-'
  }
  const slug = out.replace(/^[-._]+/, '').replace(/[-._]+$/, '')
  if (!slug || !SLUG_PATTERN.test(slug)) throw new Error(`cannot derive a safe slug from URL "${rawURL}"; pass --slug explicitly`)
  return slug
}

// resolveThirdPartyScope is deliberately separate from resolveScope
// (../commands/integrations.js): third-party's default is "project" (D4),
// the opposite of the catalog's "global" default. Explicit --scope is
// detected via `options.scope !== undefined`, never by comparing the
// value against "project" (that comparison cannot distinguish an explicit
// `--scope project` from the flag's unset default). Mirrors
// internal/commands/integrations_thirdparty.go:resolveThirdPartyScope.
function resolveThirdPartyScope(options) {
  if (options.scope !== undefined) {
    if (options.scope !== 'project' && options.scope !== 'global') throw new Error(`invalid --scope "${options.scope}": use project or global`)
    return options.scope
  }
  return 'project'
}

async function executeThirdPartyFetch(kind, url, options) {
  checkOrchestratorGuardrail()
  const raw = await api.thirdPartyFetch(url)
  const matched = checkMarkers(raw)
  if (matched.length && !options.forceThirdpartyMarkers) {
    throw new Error(
      `refused: content matches boundary-redefinition marker(s) [${matched.join(' ')}] (D3); pass --force-thirdparty-markers ` +
      'to quarantine it anyway (recorded in marker_check, never installed without approval)',
    )
  }

  const manager = new IntegrationManager()
  const entry = newQuarantineEntry(url, raw, matched, thirdPartyEntryKind(kind), options.targets || [])
  writeQuarantine(manager.roots.project, entry)

  console.log(`quarantined: ${quarantinePath(manager.roots.project, entry.checksum_sha256)}`)
  console.log(`checksum: ${entry.checksum_sha256}`)
  if (matched.length) console.log(`warning: marker check failed (matched=[${matched.join(' ')}]); --force-thirdparty-markers was used`)
  console.log(
    'next: obtain a favorable hades-tf review, record its provenance entry keyed by the resolved destination(s), then run ' +
    `\`${kind} third-party install --checksum ${entry.checksum_sha256} --targets <t1,t2> [--apply-to <agent-id,...>]\``,
  )
}

async function executeThirdPartyInstall(kind, options) {
  checkOrchestratorGuardrail()
  if (!options.checksum) throw new Error('install requires --checksum')
  if (!CHECKSUM_PATTERN.test(options.checksum)) {
    throw new Error(`invalid --checksum "${options.checksum}": expected a 64-character lowercase SHA-256 hex digest`)
  }
  if (!options.targets || !options.targets.length) throw new Error('install requires --targets')
  const scope = resolveThirdPartyScope(options)
  // D4-bis — print the warning as early as possible, before any other
  // output, so it is visible even if a later step aborts the command.
  if (scope === 'global') console.log(THIRD_PARTY_GLOBAL_SCOPE_WARNING)

  const manager = new IntegrationManager()
  const projectRoot = manager.roots.project

  const entry = readQuarantine(projectRoot, options.checksum)
  const content = decodeContent(entry)

  // D8c / TOCTOU close: the quarantine record's filename IS its checksum,
  // but that alone does not prove content_base64 hasn't been edited in
  // place since approval. Recompute over the decoded bytes and require
  // both the record's own field and the caller-supplied --checksum to
  // agree with it.
  const recomputed = sha256hex(content)
  if (recomputed !== options.checksum || entry.checksum_sha256 !== options.checksum) {
    throw new Error(`refused: quarantined content for "${entry.url}" no longer matches checksum ${options.checksum} (TOCTOU guard, D8c)`)
  }

  let slug = options.slug
  if (!slug) {
    slug = deriveSlug(entry.url)
  } else if (!SLUG_PATTERN.test(slug)) {
    throw new Error(`invalid --slug "${slug}": use lowercase alphanumerics, '.', '_' or '-'`)
  }

  const resolvedTargets = options.targets.map(targetID => {
    const { destination, surfaceID } = resolveThirdPartySkillDestination(targetID, scope, slug)
    return { targetID, surfaceID, destination }
  })

  // D5/AC3 preconditions for --apply-to are validated here, BEFORE any
  // write happens (including the skill file below) — failing everything up
  // front avoids leaving a partial state (skill file written, no reference
  // injected) on a precondition failure.
  const { models: resolvedAgentModels, warning: agentModelsWarning } = configModule.resolveAgentModels(scope, os.homedir(), projectRoot)
  if (agentModelsWarning) process.stderr.write(agentModelsWarning + '\n')
  let identityConfig
  if (options.applyTo && options.applyTo.length) {
    identityConfig = identityStore.load(manager.roots.global)
    for (const agentID of options.applyTo) {
      if (!catalogItems('agents').some(item => item.id === agentID)) throw new Error(`unknown agent item "${agentID}"`)
      for (const rt of resolvedTargets) {
        const agentPlans = buildPlans('agents', { targets: [rt.targetID], items: [agentID], scope, identity: identityConfig, projectRoot, agentModels: resolvedAgentModels })
        if (!agentPlans.length) throw new Error(`target ${rt.targetID} has no supported agents surface for item "${agentID}"`)
        const inspection = manager.inspect([agentPlans[0]])[0]
        // ADR imprecision found and resolved here (reported, not silently
        // worked around — mirrors the Go reference's own documented
        // resolution): D5/D8 never say which scope the referencing agent
        // artifact must be at, and D4 gives third-party a DIFFERENT default
        // scope (project) than the catalog default (global) — so the common
        // case has no project-scoped agent artifact to inject into.
        // Resolution: require the agent to already be installed, owned, and
        // NOT hand-modified at the SAME scope as the skill; fail loudly with
        // the exact remediation instead of silently skipping (AC3 forbids
        // silent decisions).
        if (!inspection.managed || inspection.state === 'not-installed') {
          throw new Error(
            `cannot attach reference: agent "${agentID}" is not installed at --scope ${scope} for target ${rt.targetID}; run ` +
            `\`trackfw agents install --scope ${scope} --targets ${rt.targetID} --items ${agentID}\` first`,
          )
        }
        if (inspection.state === 'modified') {
          throw new Error(
            `cannot attach reference: agent "${agentID}" at --scope ${scope} for target ${rt.targetID} was modified outside trackfw; run ` +
            `\`trackfw agents update --scope ${scope} --targets ${rt.targetID} --items ${agentID} --force\` first`,
          )
        }
      }
    }
  }

  // AC1 — always show content and every resolved destination before
  // writing anything, in both interactive and non-interactive mode.
  console.log(`URL: ${entry.url}\nChecksum: ${options.checksum}\n\n--- content ---\n${content.toString('utf8')}\n--- end content ---\n`)
  console.log(`Resolved destination(s) (scope=${scope}):`)
  for (const rt of resolvedTargets) console.log(`  ${rt.targetID}: ${rt.destination}`)

  if (!process.stdin.isTTY) {
    if (!options.yesITrustThisSource) throw new Error('install requires --yes-i-trust-this-source in non-interactive mode (AC1)')
  } else if (!options.yesITrustThisSource) {
    const { confirm } = require('@inquirer/prompts')
    const confirmed = await confirm({ message: 'Install this third-party content at the destination(s) shown above?' })
    if (!confirmed) throw new Error('install cancelled')
  }

  // D4-bis — global scope requires ITS OWN explicit confirmation, beyond
  // --yes-i-trust-this-source (decision by KG, 2026-08-15, superseding the
  // ML-3A choice to collapse both into --yes-i-trust-this-source).
  if (scope === 'global' && !options.yesGlobalScopeUnverified) {
    throw new Error(THIRD_PARTY_GLOBAL_SCOPE_REFUSAL)
  }

  // D8c — the TOCTOU-closing approval check, verified per resolved
  // destination (provenance is keyed by destination, not by checksum alone
  // — a checksum approved for one destination is not automatically
  // approved for another).
  for (const rt of resolvedTargets) {
    try {
      verifyApproval(projectRoot, options.checksum, rt.destination)
    } catch (err) {
      throw new Error(`not approved for ${rt.destination}: ${err.message}`)
    }
  }

  // D3 — a failed marker check is not fatal to fetch
  // (--force-thirdparty-markers already overrode that refusal), but install
  // requires the approver to have knowingly recorded marker_override in the
  // provenance entry for each destination.
  if (entry.marker_check.result === 'fail') {
    const prov = loadProvenance(projectRoot)
    for (const rt of resolvedTargets) {
      const provEntry = prov.entries[rt.destination]
      if (!provEntry || !provEntry.marker_override) {
        throw new Error(
          `refused: ${rt.destination} failed the D3 boundary marker check (matched=[${entry.marker_check.matched_markers.join(' ')}]) ` +
          'and its provenance entry lacks marker_override=true',
        )
      }
    }
  }

  // Write the skill file(s) — always through IntegrationManager, never a
  // raw fs write. The claim kind is always "skills" regardless of which
  // lifecycle ("skills"/"agents") invoked this command: the artifact always
  // lives under "skills/thirdparty/" per D5.
  const normalized = normalizeThirdPartyContent(content)
  const plans = resolvedTargets.map(rt => ({
    claim: { target: rt.targetID, surface: rt.surfaceID, scope, kind: 'skills', item: `thirdparty-${slug}`, origin: 'thirdparty' },
    destination: rt.destination,
    content: normalized,
    catalogVersion: `thirdparty:${options.checksum.slice(0, 12)}`,
    supportLevel: 'native',
  }))
  manager.install(plans, {})

  // D2-bis — record installed_sha256 (SHA-256 of the NORMALIZED bytes just
  // installed) on each destination's existing provenance entry, now that
  // the install actually succeeded. checksum_sha256 (the raw-bytes D8c
  // approval anchor, written externally by the approver) is left
  // untouched — only installed_sha256 is added/overwritten. rt.destination
  // is already the provenance key: the project-root-relative (pre-resolve)
  // string resolveThirdPartySkillDestination returns, the same value
  // verifyApproval was just called with above.
  // Field order matters here for cross-CLI byte parity
  // (scripts/check-thirdparty-parity.sh diffs thirdparty-provenance.json
  // verbatim): url, checksum_sha256, installed_sha256, installed_at,
  // approved_by, review_reference, scope, marker_override — mirrors Go's
  // ProvenanceEntry struct field order and Python's _ENTRY_FIELD_ORDER
  // (provenance.py), NOT plain object-spread insertion order (which would
  // append installed_sha256 at the end instead of right after
  // checksum_sha256).
  const installedSHA256 = sha256hex(normalized)
  for (const rt of resolvedTargets) {
    const prov = loadProvenance(projectRoot)
    const existing = prov.entries[rt.destination] || {}
    const provEntry = {
      url: existing.url,
      checksum_sha256: existing.checksum_sha256,
      installed_sha256: installedSHA256,
      installed_at: existing.installed_at,
      approved_by: existing.approved_by,
      review_reference: existing.review_reference,
      scope: existing.scope,
      marker_override: existing.marker_override,
    }
    for (const key of Object.keys(provEntry)) {
      if (provEntry[key] === undefined) delete provEntry[key]
    }
    upsertProvenanceEntry(projectRoot, rt.destination, provEntry)
  }

  // D5 — attach references to the requested catalog agent artifacts, at
  // the SAME scope as the skill file just installed. Preconditions were
  // already validated above, before the skill file was written — this loop
  // only performs writes.
  if (options.applyTo && options.applyTo.length) {
    for (const agentID of options.applyTo) {
      for (const rt of resolvedTargets) {
        upsertThirdPartyReference(projectRoot, rt.targetID, agentID, { slug, destination: rt.destination, url: entry.url })
        // Re-render now so the on-disk artifact reflects the new reference
        // immediately — not only on the next `agents update`. buildPlans
        // picks up the registry entry just written via
        // applyThirdPartyReferences (../integrations/index.js).
        const agentPlans = buildPlans('agents', { targets: [rt.targetID], items: [agentID], scope, identity: identityConfig, projectRoot, agentModels: resolvedAgentModels })
        manager.update(agentPlans, {})
      }
    }
  }

  console.log(`installed: ${plans.length} destination(s)`)
}

function createFetchCommand(kind) {
  return new Command('fetch')
    .argument('<url>', 'URL of the third-party content to fetch')
    .description('Download third-party content into quarantine for review (never installs)')
    .option('--targets <targets>', 'target CLIs this artifact is intended for (recorded for review only; confirmed again at install)', csv)
    .option('--force-thirdparty-markers', 'override refusal on boundary-redefinition markers (D3); recorded, never silent', false)
    .action(async (url, options) => executeThirdPartyFetch(kind, url, options))
}

function createInstallCommand(kind) {
  return new Command('install')
    .description('Consume a quarantined artifact into its resolved destination(s), requiring a prior checksum-linked approval')
    .option('--checksum <checksum>', 'SHA-256 checksum of the quarantined artifact (required)')
    .option('--slug <slug>', 'destination file slug (default: derived from the quarantined URL)')
    .option('--targets <targets>', 'target CLIs to install the skill file into (required)', csv)
    .option('--apply-to <ids>', 'catalog agent item IDs whose rendered file gets a reference to this artifact (optional; never inferred silently — AC3)', csv)
    .option('--scope <scope>', 'installation scope: project or global (default: project — D4)')
    .option('--yes-i-trust-this-source', 'required in non-interactive mode (AC1)', false)
    .option('--yes-global-scope-unverified', 'required, in addition to --yes-i-trust-this-source, for --scope global: confirms this installation will never be verified by `trackfw validate` (D4-bis)', false)
    .action(async options => executeThirdPartyInstall(kind, options))
}

// createThirdPartyCommand builds the "third-party" subcommand (fetch +
// install), attached under both `trackfw agents` and `trackfw skills` (D1).
// Mirrors internal/commands/integrations_thirdparty.go:newIntegrationsThirdPartyCmd.
function createThirdPartyCommand(kind) {
  const root = new Command('third-party').description(`Fetch and install third-party ${kind} content under a two-phase quarantine gate`)
  root.addCommand(createFetchCommand(kind))
  root.addCommand(createInstallCommand(kind))
  return root
}

api.createThirdPartyCommand = createThirdPartyCommand
api.checkOrchestratorGuardrail = checkOrchestratorGuardrail
api.deriveSlug = deriveSlug
api.resolveThirdPartyScope = resolveThirdPartyScope
api.THIRD_PARTY_PROVENANCE_RULE = THIRD_PARTY_PROVENANCE_RULE
api.thirdPartyFetch = thirdPartyFetchModule.fetch

module.exports = api
