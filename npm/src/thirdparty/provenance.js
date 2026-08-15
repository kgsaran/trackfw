'use strict'

// Node port of internal/thirdparty/provenance.go — the D6/D8c provenance
// record and TOCTOU-closing approval check. See:
// docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md
// (D6, D8c, D8f).

const fs = require('node:fs')
const path = require('node:path')

const { atomicWrite } = require('./quarantine')
const { redactURL } = require('./markers')

// PROVENANCE_SCHEMA_VERSION mirrors internal/thirdparty/provenance.go:provenanceSchemaVersion.
//
// Bumped 1 -> 2 (ADR-2026-08-15 D2-bis, ML-3B) to add the entry field
// installed_sha256 (SHA-256 of the NORMALIZED bytes, computed at install
// time). checksum_sha256 (SHA-256 of the RAW bytes, the D8c approval
// anchor) is unchanged. No migration path exists or is needed: at the time
// of the bump no provenance file existed anywhere with schema_version 1.
const PROVENANCE_SCHEMA_VERSION = 2

function emptyProvenance() {
  return { schema_version: PROVENANCE_SCHEMA_VERSION, entries: {} }
}

// provenancePath returns the on-disk path of the provenance file, rooted at
// root. Mirrors internal/thirdparty/provenance.go:ProvenancePath.
function provenancePath(root) {
  return path.join(root, '.trackfw', 'thirdparty-provenance.json')
}

// loadProvenance reads and validates the provenance file: a missing file is
// a legitimate "nothing installed from a third party yet" state and returns
// an empty, schema-valid Provenance; invalid JSON or an unsupported
// schema_version are fatal and thrown as errors — never silently degraded
// to empty (D8f). Deliberately asymmetric with readQuarantine (see
// quarantine.js) — do NOT "fix" this asymmetry, it is intentional. Mirrors
// internal/thirdparty/provenance.go:LoadProvenance.
function loadProvenance(root) {
  const filename = provenancePath(root)
  let data
  try {
    data = fs.readFileSync(filename, 'utf8')
  } catch (err) {
    if (err.code === 'ENOENT') return emptyProvenance()
    throw new Error(`read thirdparty provenance: ${err.message}`)
  }
  let prov
  try {
    prov = JSON.parse(data)
  } catch (err) {
    throw new Error(`decode thirdparty provenance: ${err.message}`)
  }
  if (prov.schema_version !== PROVENANCE_SCHEMA_VERSION) {
    throw new Error(`unsupported thirdparty provenance schema ${prov.schema_version}`)
  }
  if (!prov.entries || Array.isArray(prov.entries)) prov.entries = {}
  return prov
}

// writeProvenance persists prov atomically.
//
// Failure here MUST propagate to the caller and abort the installation
// (D6) — the deliberate opposite of a best-effort/log-and-continue write.
// Provenance is the only record of who approved a third-party artifact, so
// losing a write silently would leave an unapproved artifact on disk
// indistinguishable from an approved one.
//
// Every entry's url is passed through redactURL before serialization
// (D6-bis) — defense-in-depth, mirroring
// internal/thirdparty/provenance.go:WriteProvenance's doc comment: no
// command in this codebase writes a provenance entry's url today (D10.2),
// but this call site guarantees the query string is never persisted here
// regardless. Idempotent: redacting an already-redacted url is a no-op.
function writeProvenance(root, prov) {
  const entries = prov.entries || {}
  const redactedEntries = {}
  for (const [dest, entry] of Object.entries(entries)) {
    redactedEntries[dest] = { ...entry, url: redactURL(entry.url) }
  }
  const toWrite = { schema_version: PROVENANCE_SCHEMA_VERSION, entries: redactedEntries }
  const data = `${JSON.stringify(toWrite, null, 2)}\n`
  atomicWrite(provenancePath(root), data, 0o600)
}

// upsertProvenanceEntry loads the current provenance, sets
// entries[dest] = entry, and writes it back. The write is fatal-on-failure
// (see writeProvenance) — callers MUST treat a thrown error as
// "installation aborted", never log-and-continue. Mirrors
// internal/thirdparty/provenance.go:UpsertProvenanceEntry.
function upsertProvenanceEntry(root, dest, entry) {
  const prov = loadProvenance(root)
  prov.entries[dest] = entry
  writeProvenance(root, prov)
}

// verifyApproval is the D8c TOCTOU close: it only succeeds if dest has a
// provenance entry whose checksum_sha256 matches checksum exactly and whose
// approved_by is non-empty. A destination with no entry, an entry for a
// different checksum, or an entry with an empty approved_by all fail
// identically: not approved. Mirrors
// internal/thirdparty/provenance.go:VerifyApproval.
function verifyApproval(root, checksum, dest) {
  const prov = loadProvenance(root)
  const entry = prov.entries[dest]
  if (!entry) throw new Error(`no provenance entry for destination "${dest}": not approved`)
  if (entry.checksum_sha256 !== checksum) {
    throw new Error(`provenance checksum mismatch for "${dest}": approved ${entry.checksum_sha256}, got ${checksum}`)
  }
  if (!entry.approved_by) throw new Error(`provenance entry for "${dest}" has no approved_by`)
}

module.exports = { provenancePath, loadProvenance, writeProvenance, upsertProvenanceEntry, verifyApproval, PROVENANCE_SCHEMA_VERSION }
