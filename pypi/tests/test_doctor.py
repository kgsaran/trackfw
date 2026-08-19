"""Mirrors internal/integrations/doctor_test.go and
internal/commands/doctor_test.go. classify_doctor is a pure function;
run_doctor exercises the real IntegrationManager/plan_deployments path so
_inspect_core is proven reused, not reimplemented.
"""

from __future__ import annotations

import json

from trackfw.integrations.catalog import plan_deployments
from trackfw.integrations.doctor import (
    HAND_MODIFIED,
    UNKNOWN_CONTENT,
    UNREGISTERED_WRITE,
    classify_doctor,
    run_doctor,
)
from trackfw.integrations.manager import IntegrationManager

_CLAIM = {"target": "claude", "surface": "cli", "scope": "project", "kind": "agents", "item": "backend"}


def _status(state, managed, registered):
    return {"claim": _CLAIM, "destination": "/proj/x.md", "state": state, "managed": managed, "registered": registered}


def test_classify_doctor_template_match_no_manifest_entry_is_unregistered_write():
    findings = classify_doctor([_status("current", False, False)])
    assert len(findings) == 1
    assert findings[0]["finding"] == UNREGISTERED_WRITE
    assert findings[0]["remedy"]


def test_classify_doctor_manifest_owned_hash_differs_is_hand_modified():
    findings = classify_doctor([_status("modified", True, True)])
    assert len(findings) == 1
    assert findings[0]["finding"] == HAND_MODIFIED


def test_classify_doctor_content_matches_neither_template_nor_manifest_entry_is_unknown_content():
    # Was "not our problem" / [] before ML-2C — this is exactly the state
    # ML-3A's audit found silently falling outside classify_doctor's
    # branches, which is what makes `agents install`'s preflight refuse with
    # "unmanaged artifact". See UNKNOWN_CONTENT's doc comment.
    findings = classify_doctor([_status("modified", False, False)])
    assert len(findings) == 1
    assert findings[0]["finding"] == UNKNOWN_CONTENT
    assert "unmanaged artifact" in findings[0]["remedy"]


def test_classify_doctor_template_match_already_registered_and_owned_is_nothing_to_report():
    assert classify_doctor([_status("current", True, True)]) == []


def test_classify_doctor_registered_under_different_claim_current_must_not_be_unregistered():
    # registered=True (some claim owns the manifest entry), managed=False
    # (this specific claim does NOT) — must not collapse into unregistered-write.
    assert classify_doctor([_status("current", False, True)]) == []


def test_classify_doctor_registered_under_different_claim_modified_must_not_be_unknown_content():
    # The unknown-content analogue of the case above: a destination
    # registered under a DIFFERENT claim whose content also mismatches must
    # stay silent too — it is that other claim's concern, not this one's,
    # regardless of state. This is the discriminant Cenário 72
    # (check-gates-falsify.sh) falsifies for UNKNOWN_CONTENT, mirroring
    # Cenário 71 for UNREGISTERED_WRITE.
    assert classify_doctor([_status("modified", False, True)]) == []


def test_classify_doctor_sort_is_total_across_shared_destination():
    base = {"destination": "/proj/shared.md", "state": "current", "managed": False, "registered": False}
    a = {**base, "claim": {**_CLAIM, "target": "zzz"}}
    b = {**base, "claim": {**_CLAIM, "target": "aaa"}}
    findings = classify_doctor([a, b])
    assert len(findings) == 2
    assert findings[0]["claim"]["target"] == "aaa"
    assert findings[1]["claim"]["target"] == "zzz"


def test_run_doctor_empty_project_reports_zero_findings(tmp_path):
    project = tmp_path / "project"
    home = tmp_path / "home"
    project.mkdir()
    home.mkdir()
    findings = run_doctor(project_root=str(project), home_dir=str(home))
    assert findings == []


def test_run_doctor_finds_unregistered_write_then_distinguishes_hand_edit(tmp_path):
    project = tmp_path / "project"
    home = tmp_path / "home"
    project.mkdir()
    home.mkdir()

    _catalog, plans = plan_deployments(
        "agents", target_ids=["claude"], item_ids=["backend"], scope="project", 
    )
    manager = IntegrationManager(project, home)
    manager.install(plans)

    manifest_file = project / ".trackfw" / "integrations-manifest.json"
    manifest = json.loads(manifest_file.read_text())
    assert len(manifest["artifacts"]) == 1
    manifest["artifacts"] = {}
    manifest_file.write_text(json.dumps(manifest, indent=2))

    findings = run_doctor(project_root=str(project), home_dir=str(home))
    assert len(findings) == 1
    assert findings[0]["finding"] == UNREGISTERED_WRITE

    # Re-register normally, then hand-edit — must classify differently.
    manager.install(plans, force=True)
    destination, _manifest_file, _root = manager._resolve(plans[0])
    destination.write_text("edited by hand")

    findings = run_doctor(project_root=str(project), home_dir=str(home))
    assert len(findings) == 1
    assert findings[0]["finding"] == HAND_MODIFIED


def test_run_doctor_finds_unknown_content_for_a_destination_never_installed(tmp_path):
    project = tmp_path / "project"
    home = tmp_path / "home"
    project.mkdir()
    home.mkdir()

    _catalog, plans = plan_deployments(
        "agents", target_ids=["claude"], item_ids=["backend"], scope="project",
    )
    manager = IntegrationManager(project, home)
    destination, _manifest_file, _root = manager._resolve(plans[0])
    destination.parent.mkdir(parents=True, exist_ok=True)
    # Never installed (zero manifest entry), content matches neither the
    # catalog template nor any LegacyHashes entry.
    destination.write_text("nobody installed this through trackfw")

    findings = run_doctor(project_root=str(project), home_dir=str(home))
    assert len(findings) == 1
    assert findings[0]["finding"] == UNKNOWN_CONTENT
    assert "unmanaged artifact" in findings[0]["remedy"]
