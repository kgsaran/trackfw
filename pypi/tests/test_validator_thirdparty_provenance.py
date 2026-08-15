"""Tests for the "thirdparty_artifact_has_provenance" validate rule
(ADR-2026-08-15 D2, ML-3A). Python port of
internal/validator/validator_thirdparty_provenance_test.go — same
fixtures, same assertions, including the branch-ii regression test that
guards against comparing checksum_sha256 (sha256 of RAW bytes, D6)
directly against sha256 of the NORMALIZED installed file, which would
false-positive on any legitimate install whose raw content was not already
canonical.
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path

from trackfw import validator as v


def _sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _write_json(path: Path, value) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def _write_manifest(root: Path, destination: str, origin: str | None) -> None:
    claim = {
        "target": "claude",
        "surface": "code",
        "scope": "project",
        "kind": "skills",
        "item": "thirdparty-example",
    }
    if origin:
        claim["origin"] = origin
    _write_json(
        root / ".trackfw" / "integrations-manifest.json",
        {
            "schema_version": 1,
            "artifacts": {
                destination: {
                    "destination": destination,
                    "sha256": "irrelevant-for-this-rule",
                    "catalog_version": "thirdparty:abcdef123456",
                    "claims": [claim],
                }
            },
        },
    )


def _write_provenance(root: Path, destination: str, checksum: str, installed_sha256: str) -> None:
    # Keyed by destination MADE RELATIVE TO root — provenance is keyed by
    # the project-root-relative path, never by the manifest's absolute
    # destination (verified empirically against the real install command;
    # see internal/validator/validator_thirdparty_provenance_test.go's
    # comment for the full explanation).
    #
    # checksum is the D6 raw-bytes approval anchor (checksum_sha256);
    # installed_sha256 is the D2-bis field branch (ii) actually checks
    # against the installed file's own hash — independent parameters, never
    # derived from one another, mirroring production where one is written
    # by the external approver and the other by the install command.
    rel_dest = os.path.relpath(destination, root)
    _write_json(
        root / ".trackfw" / "thirdparty-provenance.json",
        {
            "schema_version": 2,
            "entries": {
                rel_dest: {
                    "url": "https://example.com/skill.md",
                    "checksum_sha256": checksum,
                    "installed_sha256": installed_sha256,
                    "installed_at": "2026-08-15T00:00:00Z",
                    "approved_by": "hades-tf",
                    "review_reference": "docs/seguranca/example.md",
                    "scope": "project",
                    "marker_override": False,
                }
            },
        },
    )


def _write_quarantine(root: Path, raw: bytes) -> str:
    checksum = _sha256_hex(raw)
    _write_json(
        root / ".trackfw" / "thirdparty-quarantine" / f"{checksum}.json",
        {
            "schema_version": 1,
            "url": "https://example.com/skill.md",
            "checksum_sha256": checksum,
            "fetched_at": "2026-08-15T00:00:00Z",
            "content_base64": base64.b64encode(raw).decode("ascii"),
            "marker_check": {"result": "pass", "matched_markers": []},
            "kind": "skill",
            "requested_targets": ["claude"],
        },
    )
    return checksum


def _chdir(tmp_path: Path, monkeypatch) -> Path:
    monkeypatch.chdir(tmp_path)
    return tmp_path.resolve()


def test_no_manifest_no_violations(tmp_path, monkeypatch):
    _chdir(tmp_path, monkeypatch)
    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert msgs == []


def test_catalog_claim_never_flagged(tmp_path, monkeypatch):
    root = _chdir(tmp_path, monkeypatch)
    destination = str(root / "skill.md")
    Path(destination).write_text("catalog content\n", encoding="utf-8")
    _write_manifest(root, destination, None)
    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert msgs == []


def test_legacy_manifest_no_origin_field_reads_as_catalog(tmp_path, monkeypatch):
    """Explicit retrocompatibility test: a manifest written before Claim.origin
    existed has NO "origin" key at all in its claim JSON."""
    root = _chdir(tmp_path, monkeypatch)
    destination = str(root / "agent.md")
    Path(destination).write_text("legacy agent content\n", encoding="utf-8")
    manifest_path = root / ".trackfw" / "integrations-manifest.json"
    manifest_path.parent.mkdir(parents=True, exist_ok=True)
    manifest_path.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "artifacts": {
                    destination: {
                        "destination": destination,
                        "sha256": "irrelevant",
                        "catalog_version": "v1",
                        "claims": [
                            {
                                "target": "claude",
                                "surface": "code",
                                "scope": "project",
                                "kind": "agents",
                                "item": "backend",
                            }
                        ],
                    }
                },
            }
        ),
        encoding="utf-8",
    )
    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert msgs == []


def test_branch_i_missing_provenance_entry(tmp_path, monkeypatch):
    root = _chdir(tmp_path, monkeypatch)
    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_text("some content\n", encoding="utf-8")
    _write_manifest(root, destination, "thirdparty")

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert len(msgs) == 1
    assert "D2 branch i" in msgs[0]["message"]
    assert destination in msgs[0]["message"]


def test_branch_ii_legitimate_install_does_not_false_positive(tmp_path, monkeypatch):
    """Load-bearing regression test (ML-3A, preserved by D2-bis/ML-3B): raw
    fetched content that is NOT already canonical must still validate clean
    when the destination holds exactly normalize_third_party_content(raw)
    and the provenance entry's installed_sha256 is sha256 of THAT
    normalized content. checksum_sha256 and installed_sha256 are
    deliberately kept at DIFFERENT values here to prove branch (ii)
    compares against installed_sha256, not checksum_sha256. No quarantine
    record is written in this test — D2-bis's whole point is that branch
    (ii) no longer touches the quarantine directory. Do not weaken this
    fixture to already-canonical content."""
    root = _chdir(tmp_path, monkeypatch)
    raw = b"\n# hello\n\nsome content\n\n\n"
    normalized = raw.strip() + b"\n"
    assert raw != normalized, "fixture is not actually testing the raw/normalized divergence"
    checksum_of_raw = _sha256_hex(raw)
    installed_sha256 = _sha256_hex(normalized)
    assert checksum_of_raw != installed_sha256, "fixture must keep checksum_sha256 and installed_sha256 distinct"

    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_bytes(normalized)
    _write_manifest(root, destination, "thirdparty")
    _write_provenance(root, destination, checksum_of_raw, installed_sha256)

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert msgs == []


def test_branch_ii_tampered_after_approval_is_caught(tmp_path, monkeypatch):
    root = _chdir(tmp_path, monkeypatch)
    raw = b"# hello\n\nsome content\n"
    normalized = raw.strip() + b"\n"
    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_bytes(b"# hello\n\nTAMPERED CONTENT\n")
    _write_manifest(root, destination, "thirdparty")
    _write_provenance(root, destination, _sha256_hex(raw), _sha256_hex(normalized))

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert len(msgs) == 1
    assert "D2 branch ii" in msgs[0]["message"]


def test_branch_ii_missing_installed_sha256_is_caught(tmp_path, monkeypatch):
    """Parity anchor: a provenance entry written only by the approver
    (D10.2), never having gone through `install`, has installed_sha256
    ABSENT (not empty — absent). Go's zero value and Python's
    entry.get(..., "") both render an empty string; a naive Node
    implementation would interpolate the JS-only literal "undefined"
    instead. This mirrors the Go/Node sibling test and pins the exact
    message text."""
    root = _chdir(tmp_path, monkeypatch)
    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_text("content\n", encoding="utf-8")
    _write_manifest(root, destination, "thirdparty")

    # Hand-authored: no "installed_sha256" key at all, exactly what an
    # approver (never having run `install`) would write.
    rel_dest = os.path.relpath(destination, root)
    _write_json(
        root / ".trackfw" / "thirdparty-provenance.json",
        {
            "schema_version": 2,
            "entries": {
                rel_dest: {
                    "url": "https://example.com/skill.md",
                    "checksum_sha256": "a" * 64,
                    "installed_at": "2026-08-15T00:00:00Z",
                    "approved_by": "hades-tf",
                    "review_reference": "docs/seguranca/example.md",
                    "scope": "project",
                    "marker_override": False,
                }
            },
        },
    )

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert len(msgs) == 1
    assert "undefined" not in msgs[0]["message"]
    assert "installed_sha256  recorded" in msgs[0]["message"]


def test_branch_ii_quarantine_deletion_does_not_break_clean_install(tmp_path, monkeypatch):
    """Load-bearing test ML-3B exists for (ADR-2026-08-15 D2-bis): reproduces
    the exact scenario the ML-3A design got wrong — build a REAL
    end-to-end state including a quarantine record (as `fetch` would leave
    it), then delete .trackfw/thirdparty-quarantine/ ENTIRELY, and confirm
    branch (ii) still works correctly. Replaces
    test_branch_ii_missing_quarantine_fails_closed (ML-3A), whose entire
    premise — that a missing quarantine record must fail validate closed —
    was the footgun D2-bis removes: branch (ii) no longer reads the
    quarantine directory at all."""
    root = _chdir(tmp_path, monkeypatch)
    raw = b"\n# hello\n\nsome content\n\n\n"
    normalized = raw.strip() + b"\n"
    checksum_of_raw = _write_quarantine(root, raw)

    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_bytes(normalized)
    _write_manifest(root, destination, "thirdparty")
    _write_provenance(root, destination, checksum_of_raw, _sha256_hex(normalized))

    import shutil

    shutil.rmtree(root / ".trackfw" / "thirdparty-quarantine")

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert msgs == []


def test_branch_ii_quarantine_deletion_still_detects_tamper(tmp_path, monkeypatch):
    root = _chdir(tmp_path, monkeypatch)
    raw = b"# hello\n\nsome content\n"
    normalized = raw.strip() + b"\n"
    checksum_of_raw = _write_quarantine(root, raw)

    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_bytes(b"# hello\n\nTAMPERED CONTENT\n")
    _write_manifest(root, destination, "thirdparty")
    _write_provenance(root, destination, checksum_of_raw, _sha256_hex(normalized))

    import shutil

    shutil.rmtree(root / ".trackfw" / "thirdparty-quarantine")

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert len(msgs) == 1
    assert "D2 branch ii" in msgs[0]["message"]
