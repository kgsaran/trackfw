"""D8a/b quarantine record for one fetched third-party artifact. Native
port of internal/thirdparty/quarantine.go.

The on-disk record is keyed by its own SHA-256 checksum (the filename IS
the checksum), which makes the record self-verifying and idempotent:
re-fetching identical content overwrites the same file with
byte-identical data.
"""

from __future__ import annotations

import base64
import json
import os
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .markers import checksum as compute_checksum
from .markers import redact_url

# Bump only alongside a migration path — see read_quarantine, which
# refuses any other value rather than guessing at a compatible shape.
QUARANTINE_SCHEMA_VERSION = 1


def _atomic_write(filename: Path, data: bytes, mode: int) -> None:
    """Writes data to filename via a temp file in the same directory
    followed by os.replace, so a reader never observes a partially
    written file. Shared by quarantine.py, provenance.py and
    references.py — mirrors internal/integrations/manager.go's
    atomicWrite / pypi/trackfw/integrations/manager.py's
    IntegrationManager._atomic_write (replicated here rather than
    imported, to keep the thirdparty package independent of
    trackfw.integrations, same rationale as check_markers/checksum being
    replicas of internal/integrations/manager.go's contentHash)."""
    filename = Path(filename)
    filename.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    descriptor, temporary = tempfile.mkstemp(prefix=".trackfw-tmp-", dir=filename.parent)
    try:
        fchmod = getattr(os, "fchmod", None)
        if fchmod is not None:
            fchmod(descriptor, mode)
        else:
            # os.fchmod is Unix-only (CPython docs: "Availability: Unix").
            # On platforms without it (Windows), fall back to chmod on the
            # temp file's own path. This reopens a narrow TOCTOU window
            # that fchmod(fd) does not have, but only on platforms where
            # fchmod never existed to begin with — os.fchmod continues to
            # be used unconditionally wherever it is available.
            os.chmod(temporary, mode)
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, filename)
    except BaseException:
        try:
            os.close(descriptor)
        except OSError:
            pass
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def new_quarantine_entry(
    raw_url: str,
    raw: bytes,
    matched_markers: list[str] | None,
    kind: str,
    requested_targets: list[str] | None,
) -> dict[str, Any]:
    """Builds a quarantine entry from freshly fetched content.
    matched_markers is check_markers()'s return value for raw; an empty/
    None value yields marker_check.result == "pass". The content is
    embedded whole, base64-encoded, in content_base64 — never a path to
    another file (D8b): an indirection through a second file would reopen
    the TOCTOU window the quarantine record exists to close.

    matched_markers/requested_targets are stored as None (not []) when
    empty/falsy, mirroring Go's nil-slice-marshals-to-null behavior — the
    Go QuarantineEntry/MarkerCheck fields have no `omitempty` json tag and
    a nil []string marshals to JSON `null`, not `[]`.

    raw_url is stored via redact_url, not verbatim (D6-bis) — see
    markers.redact_url's docstring."""
    result = "fail" if matched_markers else "pass"
    return {
        "schema_version": QUARANTINE_SCHEMA_VERSION,
        "url": redact_url(raw_url),
        "checksum_sha256": compute_checksum(raw),
        "fetched_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "content_base64": base64.b64encode(raw).decode("ascii"),
        "marker_check": {
            "result": result,
            "matched_markers": list(matched_markers) if matched_markers else None,
        },
        "kind": kind,
        "requested_targets": list(requested_targets) if requested_targets else None,
    }


def quarantine_path(root, checksum: str) -> Path:
    """The on-disk path of the quarantine record for checksum, rooted at
    root — the project or home directory the caller is operating on."""
    return Path(root) / ".trackfw" / "thirdparty-quarantine" / f"{checksum}.json"


def write_quarantine(root, entry: dict[str, Any]) -> None:
    """Persists entry atomically at
    quarantine_path(root, entry['checksum_sha256'])."""
    entry = dict(entry)
    entry["schema_version"] = QUARANTINE_SCHEMA_VERSION
    data = (json.dumps(entry, indent=2, ensure_ascii=False) + "\n").encode("utf-8")
    try:
        _atomic_write(quarantine_path(root, entry["checksum_sha256"]), data, 0o600)
    except OSError as error:
        raise RuntimeError(f"write quarantine entry: {error}") from error


def read_quarantine(root, checksum: str) -> dict[str, Any]:
    """Reads and validates the quarantine record for checksum,
    fail-closed (D8f): a missing file, invalid JSON, or an unsupported
    schema_version are all raised as errors, never degraded to a default
    value. Unlike load_provenance, a missing quarantine record is ALWAYS
    an error here — the caller already holds a checksum obtained from a
    prior fetch and is asking for that specific record, so its absence is
    itself the failure being guarded against. Do not "fix" this asymmetry
    with provenance.py's missing-file handling; it is deliberate."""
    filename = quarantine_path(root, checksum)
    try:
        raw = filename.read_bytes()
    except OSError as error:
        raise RuntimeError(f"read quarantine entry {checksum!r}: {error}") from error
    try:
        entry = json.loads(raw)
    except json.JSONDecodeError as error:
        raise RuntimeError(f"decode quarantine entry {checksum!r}: {error}") from error
    if entry.get("schema_version") != QUARANTINE_SCHEMA_VERSION:
        raise RuntimeError(
            f"unsupported quarantine schema {entry.get('schema_version')} for {checksum!r}"
        )
    return entry


def decode_content(entry: dict[str, Any]) -> bytes:
    """Decodes entry['content_base64'] back to the original raw bytes."""
    try:
        return base64.b64decode(entry["content_base64"], validate=True)
    except Exception as error:  # noqa: BLE001 - mirrors Go's single fmt.Errorf wrap
        raise RuntimeError(f"decode quarantine content: {error}") from error
