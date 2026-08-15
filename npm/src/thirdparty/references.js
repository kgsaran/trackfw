'use strict'

// Node port of the D9 extension point from internal/integrations/render.go
// (ThirdPartyReference, UpsertThirdPartyReference, ApplyThirdPartyReferences,
// NormalizeThirdPartyContent, ResolveThirdPartySkillDestination) — see the
// D9 amendment in
// docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md.
//
// D9 note (see docs/agents-working-context.md, ML-1A entry): this file
// persists .trackfw/thirdparty-references.json, an artifact NOT mentioned
// by the original ADR text (D1-D8) — it exists because a plain rendered
// catalog agent file cannot simply have a reference block appended once:
// the next `trackfw agents update` calls buildPlans -> render again, which
// re-derives content straight from the catalog asset and knows nothing
// about the appended block. Persisting the reference here and having
// buildPlans call applyThirdPartyReferences after every render (see
// ../integrations/index.js) makes the canonical render reproduce the
// block, so repeated `agents update` runs settle at state "current"
// instead of fighting the third-party attachment.

const fs = require('node:fs')
const path = require('node:path')

const { atomicWrite } = require('./quarantine')
const { target: catalogTarget, surfaceFor } = require('../integrations/catalog')

const THIRDPARTY_REFERENCES_SCHEMA_VERSION = 1

// truncateBeforeIdSegment drops the "{{id}}"-bearing path segment and
// everything after it, returning the shared parent directory. Duplicated
// (not imported) from npm/src/integrations/catalog.js — that module does
// not export it, and this ML's file list does not include editing
// catalog.js; this is the same small, pure, low-risk duplication already
// used by the Go reference itself (see markers.go's doc comment on
// Checksum) rather than widening catalog.js's public surface for one
// caller.
function truncateBeforeIdSegment(p) {
  const segments = p.split('/')
  const index = segments.findIndex(segment => segment.includes('{{id}}'))
  if (index === -1) return p
  return segments.slice(0, index).join('/')
}

function thirdPartyReferencesPath(root) {
  return path.join(root, '.trackfw', 'thirdparty-references.json')
}

function emptyRegistry() {
  return { schema_version: THIRDPARTY_REFERENCES_SCHEMA_VERSION, entries: {} }
}

function loadThirdPartyReferenceRegistry(root) {
  const filename = thirdPartyReferencesPath(root)
  let data
  try {
    data = fs.readFileSync(filename, 'utf8')
  } catch (err) {
    if (err.code === 'ENOENT') return emptyRegistry()
    throw new Error(`read thirdparty reference registry: ${err.message}`)
  }
  let reg
  try {
    reg = JSON.parse(data)
  } catch (err) {
    throw new Error(`decode thirdparty reference registry: ${err.message}`)
  }
  if (reg.schema_version !== THIRDPARTY_REFERENCES_SCHEMA_VERSION) {
    throw new Error(`unsupported thirdparty reference registry schema ${reg.schema_version}`)
  }
  if (!reg.entries || Array.isArray(reg.entries)) reg.entries = {}
  return reg
}

function writeThirdPartyReferenceRegistry(root, reg) {
  const toWrite = { schema_version: THIRDPARTY_REFERENCES_SCHEMA_VERSION, entries: reg.entries || {} }
  const data = `${JSON.stringify(toWrite, null, 2)}\n`
  atomicWrite(thirdPartyReferencesPath(root), data, 0o600)
}

// upsertThirdPartyReference records (or replaces, keyed by ref.slug) a
// reference from the rendered catalog agent artifact (targetID,
// agentItemID) to a third-party skill, persisted under root (project root
// — D4). Idempotent: calling it twice with the same slug replaces the
// prior entry instead of duplicating it. Mirrors
// internal/integrations/render.go:UpsertThirdPartyReference.
function upsertThirdPartyReference(root, targetID, agentItemID, ref) {
  const reg = loadThirdPartyReferenceRegistry(root)
  const key = `${targetID}/${agentItemID}`
  const refs = reg.entries[key] || []
  let replaced = false
  for (let i = 0; i < refs.length; i++) {
    if (refs[i].slug === ref.slug) {
      refs[i] = ref
      replaced = true
      break
    }
  }
  if (!replaced) refs.push(ref)
  refs.sort((a, b) => a.slug.localeCompare(b.slug))
  reg.entries[key] = refs
  writeThirdPartyReferenceRegistry(root, reg)
}

// THIRD_PARTY_REF_START/END are the composition markers (D5), dedicated and
// distinct from the auxiliary-rules-file markers (a different subsystem).
// Mirrors internal/integrations/render.go:thirdPartyRefStart/thirdPartyRefEnd.
const THIRD_PARTY_REF_START = '<!-- trackfw:thirdparty-skills:start -->'
const THIRD_PARTY_REF_END = '<!-- trackfw:thirdparty-skills:end -->'

// applyThirdPartyReferences injects or updates the third-party reference
// block in content for (targetID, agentItemID), based on entries persisted
// by upsertThirdPartyReference. When root is empty or there are no entries
// for this key, content is returned byte-for-byte unchanged — this is the
// guarantee that every agent artifact which never received a third-party
// attachment renders exactly as it did before D5/D9 were introduced.
// Mirrors internal/integrations/render.go:ApplyThirdPartyReferences.
function applyThirdPartyReferences(root, content, targetID, agentItemID) {
  if (!root) return content
  const reg = loadThirdPartyReferenceRegistry(root)
  const refs = reg.entries[`${targetID}/${agentItemID}`] || []
  if (!refs.length) return content

  let block = `${THIRD_PARTY_REF_START}\n`
  for (const ref of refs) {
    block += `- Third-party skill "${ref.slug}": ${ref.destination} (source: ${ref.url})\n`
  }
  block += THIRD_PARTY_REF_END

  let text = content
  const start = text.indexOf(THIRD_PARTY_REF_START)
  if (start === -1) {
    if (!text.endsWith('\n')) text += '\n'
    text += `\n${block}\n`
    return text
  }
  // Search for the end marker starting at start, not from the beginning of
  // text (hefesto-tf finding, ML-4C): searching the whole text could find
  // an END marker that appears BEFORE start, producing end < start and
  // silently corrupting the composed output. Anchoring the search at start
  // makes that impossible: end is either -1 or >= start.
  const relEnd = text.slice(start).indexOf(THIRD_PARTY_REF_END)
  if (relEnd === -1) {
    // Malformed (start without end): append a fresh block rather than
    // guess at repair.
    text += `\n${block}\n`
    return text
  }
  const end = start + relEnd
  const newText = text.slice(0, start) + block + text.slice(end + THIRD_PARTY_REF_END.length)
  return `${newText.replace(/\n+$/, '')}\n`
}

// normalizeThirdPartyContent applies the same normalization used for
// managed catalog skill content (trim + single trailing newline) to raw
// third-party bytes before they are written through IntegrationManager, so
// third-party artifacts are stored with the same on-disk convention as
// catalog artifacts. Mirrors
// internal/integrations/render.go:NormalizeThirdPartyContent.
function normalizeThirdPartyContent(content) {
  const text = Buffer.isBuffer(content) ? content.toString('utf8') : String(content)
  return `${text.trim()}\n`
}

// resolveThirdPartySkillDestination computes where a third-party artifact's
// content should live for (targetID, scope), per D5: the shared parent
// directory of the target's own canonical project/global Skills install
// path template, followed by "/thirdparty/<slug>.md". The directory always
// comes from the catalog's own path template — never a hardcoded
// ".claude" — so every target the catalog declares a Skills capability for
// is supported without per-target special-casing. Mirrors
// internal/integrations/render.go:ResolveThirdPartySkillDestination.
function resolveThirdPartySkillDestination(targetID, scope, slug) {
  let targetEntry
  try {
    targetEntry = catalogTarget(targetID)
  } catch {
    throw new Error(`unknown target "${targetID}"`)
  }
  const surface = surfaceFor(targetEntry, undefined, 'skills')
  const installPath = (surface.paths.skills || []).find(entry => entry.scope === scope)
  if (!installPath) throw new Error(`target ${targetID} surface ${surface.id} has no ${scope} skills path`)
  const baseDir = truncateBeforeIdSegment(installPath.path)
  return { destination: `${baseDir}/thirdparty/${slug}.md`, surfaceID: surface.id }
}

module.exports = {
  thirdPartyReferencesPath,
  upsertThirdPartyReference,
  applyThirdPartyReferences,
  normalizeThirdPartyContent,
  resolveThirdPartySkillDestination,
  THIRD_PARTY_REF_START,
  THIRD_PARTY_REF_END,
}
