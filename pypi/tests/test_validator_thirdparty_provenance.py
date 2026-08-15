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


def _write_provenance(root: Path, destination: str, checksum: str) -> None:
    # Keyed by destination MADE RELATIVE TO root — provenance is keyed by
    # the project-root-relative path, never by the manifest's absolute
    # destination (verified empirically against the real install command;
    # see internal/validator/validator_thirdparty_provenance_test.go's
    # comment for the full explanation).
    rel_dest = os.path.relpath(destination, root)
    _write_json(
        root / ".trackfw" / "thirdparty-provenance.json",
        {
            "schema_version": 1,
            "entries": {
                rel_dest: {
                    "url": "https://example.com/skill.md",
                    "checksum_sha256": checksum,
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
    """Load-bearing regression test: raw fetched content that is NOT already
    canonical must still validate clean when the destination holds exactly
    normalize_third_party_content(raw) — the real output of a correct
    install. Do not weaken this fixture to already-canonical content."""
    root = _chdir(tmp_path, monkeypatch)
    raw = b"\n# hello\n\nsome content\n\n\n"
    normalized = raw.strip() + b"\n"
    assert raw != normalized, "fixture is not actually testing the raw/normalized divergence"

    checksum = _write_quarantine(root, raw)
    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_bytes(normalized)
    _write_manifest(root, destination, "thirdparty")
    _write_provenance(root, destination, checksum)

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert msgs == []


def test_branch_ii_tampered_after_approval_is_caught(tmp_path, monkeypatch):
    root = _chdir(tmp_path, monkeypatch)
    raw = b"# hello\n\nsome content\n"
    checksum = _write_quarantine(root, raw)
    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_bytes(b"# hello\n\nTAMPERED CONTENT\n")
    _write_manifest(root, destination, "thirdparty")
    _write_provenance(root, destination, checksum)

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert len(msgs) == 1
    assert "D2 branch ii" in msgs[0]["message"]


def test_branch_ii_missing_quarantine_fails_closed(tmp_path, monkeypatch):
    root = _chdir(tmp_path, monkeypatch)
    destination = str(root / "skills" / "thirdparty" / "example.md")
    Path(destination).parent.mkdir(parents=True, exist_ok=True)
    Path(destination).write_text("content\n", encoding="utf-8")
    _write_manifest(root, destination, "thirdparty")
    _write_provenance(root, destination, "a" * 64)

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert len(msgs) == 1
    assert "D8f" in msgs[0]["message"]
