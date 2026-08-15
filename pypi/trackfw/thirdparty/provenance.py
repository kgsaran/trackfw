"""D6/D8c/D8f provenance record for installed third-party artifacts.
Native port of internal/thirdparty/provenance.go.

Keyed by destination — not appended chronologically — because the
`trackfw validate` rule that consumes it
(thirdparty_artifact_has_provenance, D2) needs an O(1) lookup by
destination to decide whether a managed file has a matching approval, not
a history of installs.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from .markers import redact_url
from .quarantine import _atomic_write

# Bump only alongside a migration path — see load_provenance, which
# refuses any other value.
#
# Bumped 1 -> 2 (ADR-2026-08-15 D2-bis, ML-3B) to add installed_sha256
# (SHA-256 of the NORMALIZED bytes, computed at install time). checksum_sha256
# (SHA-256 of the RAW bytes, the D8c approval anchor) is unchanged. No
# migration path exists or is needed: at the time of the bump no provenance
# file existed anywhere with schema_version 1.
PROVENANCE_SCHEMA_VERSION = 2

# Canonical field order of Go's ProvenanceEntry struct — provenance
# entries, unlike quarantine records and reference entries (which this
# package always builds itself), are written by an external caller
# (hades-tf/the architect, per D10.2 — there is no `install --approve`
# command in this ML). A caller-supplied dict's key insertion order is NOT
# guaranteed to match Go's struct-field-declaration JSON key order, so
# write_provenance() below reorders each entry to this canonical order
# before serializing, to stay byte-identical to the Go/Node output.
_ENTRY_FIELD_ORDER = (
    "url",
    "checksum_sha256",
    "installed_sha256",
    "installed_at",
    "approved_by",
    "review_reference",
    "scope",
    "marker_override",
)


def _canonicalize_entry(entry: dict[str, Any]) -> dict[str, Any]:
    ordered = {key: entry[key] for key in _ENTRY_FIELD_ORDER if key in entry}
    extra = {key: value for key, value in entry.items() if key not in _ENTRY_FIELD_ORDER}
    ordered.update(extra)
    # D6-bis, defense-in-depth: redact the query string before it is ever
    # written to disk, mirroring internal/thirdparty/provenance.go's
    # WriteProvenance. No command in this codebase writes a provenance
    # entry's url today (D10.2 — the external approver writes it
    # directly), but this call site guarantees it never leaks here
    # regardless. Idempotent: redacting an already-redacted url is a no-op.
    if "url" in ordered:
        ordered["url"] = redact_url(ordered["url"])
    return ordered


def provenance_path(root) -> Path:
    """The on-disk path of the provenance file, rooted at root — the
    project or home directory the caller is operating on."""
    return Path(root) / ".trackfw" / "thirdparty-provenance.json"


def _empty_provenance() -> dict[str, Any]:
    return {"schema_version": PROVENANCE_SCHEMA_VERSION, "entries": {}}


def load_provenance(root) -> dict[str, Any]:
    """Reads and validates the provenance file: a missing file is a
    legitimate "nothing installed from a third party yet" state and
    returns an empty, schema-valid provenance dict; invalid JSON or an
    unsupported schema_version are fatal and raised as errors — never
    silently degraded to empty (D8f). Callers that require a specific
    entry to exist (verify_approval) get their own fail-closed behavior
    from the entry lookup, not from this function refusing to run on a
    missing file — this asymmetry with read_quarantine is deliberate, see
    quarantine.py's read_quarantine docstring."""
    filename = provenance_path(root)
    try:
        raw = filename.read_bytes()
    except FileNotFoundError:
        return _empty_provenance()
    except OSError as error:
        raise RuntimeError(f"read thirdparty provenance: {error}") from error
    try:
        prov = json.loads(raw)
    except json.JSONDecodeError as error:
        raise RuntimeError(f"decode thirdparty provenance: {error}") from error
    if prov.get("schema_version") != PROVENANCE_SCHEMA_VERSION:
        raise RuntimeError(f"unsupported thirdparty provenance schema {prov.get('schema_version')}")
    if not isinstance(prov.get("entries"), dict):
        prov["entries"] = {}
    return prov


def write_provenance(root, prov: dict[str, Any]) -> None:
    """Persists prov atomically.

    Failure here MUST propagate to the caller and abort the installation
    (D6). Provenance is the only record of who approved a third-party
    artifact, so losing a write silently would leave an unapproved
    artifact on disk indistinguishable from an approved one — never
    downgrade this to a best-effort/log-and-continue write."""
    prov = dict(prov)
    prov["schema_version"] = PROVENANCE_SCHEMA_VERSION
    entries = prov.get("entries") or {}
    prov["entries"] = {dest: _canonicalize_entry(entry) for dest, entry in entries.items()}
    data = (json.dumps(prov, indent=2, ensure_ascii=False) + "\n").encode("utf-8")
    try:
        _atomic_write(provenance_path(root), data, 0o600)
    except OSError as error:
        raise RuntimeError(f"write thirdparty provenance: {error}") from error


def upsert_provenance_entry(root, dest: str, entry: dict[str, Any]) -> None:
    """Loads the current provenance, sets entries[dest] = entry, and
    writes it back. The write is fatal-on-failure (see write_provenance)
    — callers MUST treat a raised error as "installation aborted", never
    log-and-continue."""
    prov = load_provenance(root)
    prov["entries"][dest] = entry
    write_provenance(root, prov)


def verify_approval(root, checksum: str, dest: str) -> None:
    """The D8c TOCTOU close: only succeeds if dest has a provenance entry
    whose checksum_sha256 matches checksum exactly and whose approved_by
    is non-empty. A loose "approved" boolean is rejected by construction
    — a destination with no entry, an entry for a different checksum, or
    an entry with an empty approved_by all fail identically: not
    approved. Raises RuntimeError on any of those three cases."""
    prov = load_provenance(root)
    entry = prov["entries"].get(dest)
    if entry is None:
        raise RuntimeError(f"no provenance entry for destination {dest!r}: not approved")
    if entry.get("checksum_sha256") != checksum:
        raise RuntimeError(
            f"provenance checksum mismatch for {dest!r}: approved "
            f"{entry.get('checksum_sha256')}, got {checksum}"
        )
    if not entry.get("approved_by"):
        raise RuntimeError(f"provenance entry for {dest!r} has no approved_by")
