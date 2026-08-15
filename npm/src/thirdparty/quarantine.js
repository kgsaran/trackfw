'use strict'

// Node port of internal/thirdparty/quarantine.go — the D8a/b on-disk
// quarantine record for one fetched third-party artifact. See:
// docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md
// (D8a, D8b, D8f).

const crypto = require('node:crypto')
const fs = require('node:fs')
const path = require('node:path')

const { checksum: sha256hex, redactURL } = require('./markers')

// QUARANTINE_SCHEMA_VERSION mirrors internal/thirdparty/quarantine.go:quarantineSchemaVersion.
const QUARANTINE_SCHEMA_VERSION = 1

// atomicWrite writes data to filename via a temp file in the same
// directory followed by fs.renameSync, so a reader never observes a
// partially written file. Mirrors internal/thirdparty/quarantine.go's
// atomicWrite (shared there by quarantine.go and provenance.go; replicated
// here and in references.js, same rationale as the Go reference's own
// per-package duplication of Checksum in markers.go's doc comment).
function atomicWrite(filename, data, mode) {
  const directory = path.dirname(filename)
  fs.mkdirSync(directory, { recursive: true, mode: 0o700 })
  const tmp = path.join(directory, `.trackfw-tmp-${process.pid}-${crypto.randomBytes(6).toString('hex')}`)
  try {
    fs.writeFileSync(tmp, data, { mode })
    fs.chmodSync(tmp, mode)
    fs.renameSync(tmp, filename)
  } finally {
    if (fs.existsSync(tmp)) fs.unlinkSync(tmp)
  }
}

// quarantinePath returns the on-disk path of the quarantine record for
// checksum, rooted at root — the project or home directory the caller is
// operating on. Mirrors internal/thirdparty/quarantine.go:QuarantinePath.
function quarantinePath(root, checksum) {
  return path.join(root, '.trackfw', 'thirdparty-quarantine', `${checksum}.json`)
}

// newQuarantineEntry builds a quarantine entry from freshly fetched content.
// matchedMarkers is checkMarkers()'s return value for raw; an empty array
// yields marker_check.result === "pass". The content is embedded whole,
// base64-encoded, in content_base64 — never a path to another file (D8b):
// an indirection through a second file would reopen the TOCTOU window the
// quarantine record exists to close.
//
// rawURL is stored via redactURL, not verbatim (D6-bis) — see
// internal/thirdparty/quarantine.go:NewQuarantineEntry's doc comment.
function newQuarantineEntry(rawURL, raw, matchedMarkers, kind, requestedTargets) {
  const buf = Buffer.isBuffer(raw) ? raw : Buffer.from(String(raw), 'utf8')
  return {
    schema_version: QUARANTINE_SCHEMA_VERSION,
    url: redactURL(rawURL),
    checksum_sha256: sha256hex(buf),
    fetched_at: new Date().toISOString().replace(/\.\d{3}Z$/, 'Z'),
    content_base64: buf.toString('base64'),
    marker_check: {
      result: matchedMarkers.length > 0 ? 'fail' : 'pass',
      matched_markers: matchedMarkers,
    },
    kind,
    requested_targets: requestedTargets || [],
  }
}

// writeQuarantine persists entry atomically at
// quarantinePath(root, entry.checksum_sha256). Mirrors
// internal/thirdparty/quarantine.go:WriteQuarantine.
function writeQuarantine(root, entry) {
  const toWrite = { ...entry, schema_version: QUARANTINE_SCHEMA_VERSION }
  const data = `${JSON.stringify(toWrite, null, 2)}\n`
  atomicWrite(quarantinePath(root, entry.checksum_sha256), data, 0o600)
}

// readQuarantine reads and validates the quarantine record for checksum,
// fail-closed (D8f): a missing file, invalid JSON, or an unsupported
// schema_version are all thrown as errors, never degraded to an empty
// value. Deliberately asymmetric with loadProvenance (see provenance.js):
// here, the caller already holds a checksum obtained from a prior fetch and
// is asking for that specific record, so its absence is itself the failure
// being guarded against. Do NOT "fix" this asymmetry — it is intentional,
// verified against the Go reference by audit. Mirrors
// internal/thirdparty/quarantine.go:ReadQuarantine.
function readQuarantine(root, checksum) {
  const filename = quarantinePath(root, checksum)
  let data
  try {
    data = fs.readFileSync(filename, 'utf8')
  } catch (err) {
    throw new Error(`read quarantine entry "${checksum}": ${err.message}`)
  }
  let entry
  try {
    entry = JSON.parse(data)
  } catch (err) {
    throw new Error(`decode quarantine entry "${checksum}": ${err.message}`)
  }
  if (entry.schema_version !== QUARANTINE_SCHEMA_VERSION) {
    throw new Error(`unsupported quarantine schema ${entry.schema_version} for "${checksum}"`)
  }
  return entry
}

// decodeContent decodes entry.content_base64 back to the original raw
// bytes (a Buffer). Mirrors internal/thirdparty/quarantine.go:QuarantineEntry.DecodeContent.
function decodeContent(entry) {
  return Buffer.from(entry.content_base64, 'base64')
}

module.exports = { quarantinePath, newQuarantineEntry, writeQuarantine, readQuarantine, decodeContent, atomicWrite, QUARANTINE_SCHEMA_VERSION }
