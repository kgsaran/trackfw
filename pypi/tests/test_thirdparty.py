"""Tests for the third-party artifact two-phase quarantine gate (ML-2B,
Python port of internal/commands/integrations_thirdparty_test.go +
internal/thirdparty's own package tests). See
docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md.

Fetch/install are exercised via the argparse Namespace + module functions
(trackfw.commands.thirdparty.execute_fetch/execute_install) directly, in
the same process — not via a subprocess CLI invocation — because these
tests need to monkeypatch thirdparty_fetch (never touching the real
network, compatible with TRACKFW_DISABLE_EXTERNAL_COMMANDS=1) and the
confirm/scope prompts. Mirrors the in-process style already used by
test_scope_resolution.py's test_targets_flag_with_tty_and_no_scope_...
"""

from __future__ import annotations

import argparse
import json
import os
import urllib.error
import urllib.request
from email.message import Message
from pathlib import Path

import pytest

from trackfw import identity
from trackfw import validator as v
from trackfw.commands import thirdparty as tp
from trackfw.integrations.catalog import plan_deployments
from trackfw.integrations.command import run as integrations_run
from trackfw.integrations.manager import IntegrationError, IntegrationManager
from importlib import import_module

from trackfw import thirdparty as thirdparty_pkg

# Note: `import trackfw.thirdparty.fetch as fetch_mod` would NOT yield the
# submodule here — `import a.b.c as x` binds via attribute traversal from
# the top package (`sys.modules['a'].b.c`), and trackfw.thirdparty's
# __init__.py re-exports a *function* named `fetch` that shadows the
# `fetch` submodule attribute on the package object. import_module() goes
# straight to sys.modules by dotted name and sidesteps that shadowing.
fetch_mod = import_module("trackfw.thirdparty.fetch")

BENIGN_CONTENT = b"# Example Third-Party Skill\n\nSome helpful, benign content for the agent to consume.\n"


def _fixture(tmp_path, monkeypatch):
    """project/home directories + guardrail env + cwd/HOME wiring, mirroring
    Go's integrationCommandFixture(t)."""
    project = tmp_path / "project"
    home = tmp_path / "home"
    project.mkdir()
    home.mkdir()
    monkeypatch.setenv("TRACKFW_ORCHESTRATOR_SESSION", "1")
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.chdir(project)
    return project, home


def _install_backend_agent(project: Path, home: Path) -> None:
    args = argparse.Namespace(
        targets="claude", items="backend", scope="project", surface=None, json=True,
        force=False, identity=False, identity_preset=None, action="install",
    )
    integrations_run(args, "agents")


def _stub_fetch(monkeypatch, content: bytes = BENIGN_CONTENT):
    monkeypatch.setattr(tp, "thirdparty_fetch", lambda url: content)


def _fetch_args(url: str, targets: str | None = None, force: bool = False) -> argparse.Namespace:
    return argparse.Namespace(url=url, targets=targets, force_thirdparty_markers=force)


def _install_args(
    checksum: str,
    targets: str = "claude",
    slug: str | None = None,
    apply_to: str | None = None,
    scope: str | None = None,
    yes: bool = True,
) -> argparse.Namespace:
    return argparse.Namespace(
        checksum=checksum, slug=slug, targets=targets, apply_to=apply_to,
        scope=scope, yes_i_trust_this_source=yes,
    )


def _upsert_provenance(project: Path, dest: str, checksum: str, marker_override: bool = False) -> None:
    thirdparty_pkg.upsert_provenance_entry(
        str(project),
        dest,
        {
            "url": "https://example.com/skills/my-skill.md",
            "checksum_sha256": checksum,
            "installed_at": "2026-08-15T00:00:00Z",
            "approved_by": "hades-tf",
            "review_reference": "docs/seguranca/test.md",
            "scope": "project",
            "marker_override": marker_override,
        },
    )


# ---------------------------------------------------------------------------
# markers.py — normalization pipeline, fence handling, homoglyph boundary
# ---------------------------------------------------------------------------


def test_check_markers_matches_literal_heading():
    matched = thirdparty_pkg.check_markers(b"# Git authority\n\nsome content redefining boundaries.\n")
    assert matched == ["git authority"]


def test_check_markers_matches_h6_heading():
    # headingLinePattern matches any level 1-6, not just H1 — a marker
    # buried at H6 must be caught the same as at H1.
    matched = thirdparty_pkg.check_markers(b"###### Mode lock\n\nsome content.\n")
    assert matched == ["mode lock"]


def test_check_markers_ignores_marker_inside_fenced_block():
    content = (
        b"Some doc that quotes the marker list:\n\n"
        b"```\n# Git authority\n```\n\nNo heading here.\n"
    )
    assert thirdparty_pkg.check_markers(content) == []


def test_check_markers_unclosed_fence_still_drops_content():
    # An unclosed fence swallows the rest of the document as fenced content
    # (matches the Go scanner: no closer found means everything after the
    # opener stays "inside the fence").
    content = b"```\n# Git authority\nstill inside\n"
    assert thirdparty_pkg.check_markers(content) == []


def test_check_markers_closer_shorter_than_opener_does_not_close():
    # CommonMark rule: the closer needs AT LEAST as many repeats as the
    # opener. A 3-backtick opener closed by a 2-backtick line never closes.
    content = b"````\n# Git authority\n```\nstill fenced\n"
    assert thirdparty_pkg.check_markers(content) == []


def test_check_markers_indented_fence_still_recognized():
    # fencePrefixPattern allows leading whitespace — a fence opener/closer
    # indented under a list item or blockquote is still recognized as a
    # fence delimiter, not read as prose.
    content = b"   ```\n   # Git authority\n   ```\n\nRegular text.\n"
    assert thirdparty_pkg.check_markers(content) == []


def test_check_markers_heading_after_closed_fence_still_matches():
    # Converse of the fence-acceptance tests: content AFTER a properly
    # closed fence must still be read as ordinary text, so a real heading
    # following a fence is not accidentally swallowed.
    content = b"```\nsome code, not a marker\n```\n\n# Git authority\n\nRegular text.\n"
    assert thirdparty_pkg.check_markers(content) == ["git authority"]


def test_check_markers_tilde_fence_also_removed():
    content = b"~~~\n# Mode lock\n~~~\n\nRegular text.\n"
    assert thirdparty_pkg.check_markers(content) == []


def test_check_markers_full_width_heading_is_refused():
    # NFKC folds full-width characters to their ASCII equivalents — a
    # deliberate boundary the marker check DOES cover.
    content = "＃ Git authority\n".encode("utf-8")
    assert thirdparty_pkg.check_markers(content) == ["git authority"]


def test_check_markers_cyrillic_homoglyph_passes():
    # Documented gap (D3, "o que este critério NÃO cobre"): Cyrillic "а"
    # (U+0430) visually resembles Latin "a" but NFKC does not fold it, so
    # this heading passes the check. This is asserted as a boundary, not a
    # bug — do not "fix" it by adding homoglyph folding here.
    content = "# Git аuthority\n".encode("utf-8")
    assert thirdparty_pkg.check_markers(content) == []


def test_check_markers_multiple_distinct_markers_all_reported_once():
    content = b"# Git authority\n\n# Mode Lock\n\n# git authority\n"
    assert thirdparty_pkg.check_markers(content) == ["git authority", "mode lock"]


def test_checksum_is_sha256_hex_of_raw_bytes():
    import hashlib

    raw = b"hello world"
    assert thirdparty_pkg.checksum(raw) == hashlib.sha256(raw).hexdigest()


# ---------------------------------------------------------------------------
# quarantine.py / provenance.py — fail-closed asymmetry (D8f)
# ---------------------------------------------------------------------------


def test_read_quarantine_missing_file_is_always_an_error(tmp_path):
    with pytest.raises(RuntimeError):
        thirdparty_pkg.read_quarantine(str(tmp_path), "a" * 64)


def test_load_provenance_missing_file_returns_empty(tmp_path):
    prov = thirdparty_pkg.load_provenance(str(tmp_path))
    assert prov == {"schema_version": 2, "entries": {}}


def test_verify_approval_missing_entry_fails_closed(tmp_path):
    with pytest.raises(RuntimeError, match="not approved"):
        thirdparty_pkg.verify_approval(str(tmp_path), "a" * 64, "dest")


def test_write_then_read_quarantine_roundtrip(tmp_path):
    entry = thirdparty_pkg.new_quarantine_entry(
        "https://example.com/skills/x.md", BENIGN_CONTENT, [], "skill", ["claude"]
    )
    thirdparty_pkg.write_quarantine(str(tmp_path), entry)
    reread = thirdparty_pkg.read_quarantine(str(tmp_path), entry["checksum_sha256"])
    assert reread["checksum_sha256"] == entry["checksum_sha256"]
    assert thirdparty_pkg.decode_content(reread) == BENIGN_CONTENT
    assert reread["marker_check"] == {"result": "pass", "matched_markers": None}


# ---------------------------------------------------------------------------
# fetch/install command flow (in-process, network stubbed)
# ---------------------------------------------------------------------------


def test_fetch_never_writes_outside_quarantine(tmp_path, monkeypatch):
    project, _ = _fixture(tmp_path, monkeypatch)
    _stub_fetch(monkeypatch)

    tp.execute_fetch("skills", _fetch_args("https://example.com/skills/my-skill.md"))

    quarantine_dir = project / ".trackfw" / "thirdparty-quarantine"
    files = list(quarantine_dir.glob("*.json"))
    assert len(files) == 1

    unexpected = []
    for path in project.rglob("*"):
        if path.is_dir():
            continue
        rel = path.relative_to(project)
        if not str(rel).startswith(str(Path(".trackfw") / "thirdparty-quarantine")):
            unexpected.append(str(rel))
    assert unexpected == []


def test_fetch_refuses_marker_by_default(tmp_path, monkeypatch):
    _fixture(tmp_path, monkeypatch)
    _stub_fetch(monkeypatch, b"# Git authority\n\nsome content redefining boundaries.\n")

    with pytest.raises(IntegrationError, match="(?i)git authority"):
        tp.execute_fetch("skills", _fetch_args("https://example.com/skills/evil.md"))


def test_fetch_force_flag_records_marker_override_requirement(tmp_path, monkeypatch):
    project, _ = _fixture(tmp_path, monkeypatch)
    _stub_fetch(monkeypatch, b"# Git authority\n\nsome content redefining boundaries.\n")

    tp.execute_fetch("skills", _fetch_args("https://example.com/skills/evil.md", force=True))

    files = list((project / ".trackfw" / "thirdparty-quarantine").glob("*.json"))
    assert len(files) == 1
    record = json.loads(files[0].read_text())
    assert record["marker_check"]["result"] == "fail"
    assert record["marker_check"]["matched_markers"] == ["git authority"]


def test_guardrail_refuses_without_orchestrator_session(tmp_path, monkeypatch):
    project = tmp_path / "project"
    project.mkdir()
    monkeypatch.chdir(project)
    monkeypatch.delenv("TRACKFW_ORCHESTRATOR_SESSION", raising=False)
    _stub_fetch(monkeypatch)

    with pytest.raises(IntegrationError) as excinfo:
        tp.execute_fetch("skills", _fetch_args("https://example.com/skills/my-skill.md"))
    message = str(excinfo.value)
    assert "guardrail" in message
    assert tp.THIRD_PARTY_PROVENANCE_RULE in message
    assert "not a security control" in message


def _run_fetch(project: Path, monkeypatch, url: str, content: bytes = BENIGN_CONTENT) -> str:
    _stub_fetch(monkeypatch, content)
    tp.execute_fetch("skills", _fetch_args(url))
    files = list((project / ".trackfw" / "thirdparty-quarantine").glob("*.json"))
    assert len(files) == 1
    return files[0].stem


def test_install_fails_without_approval(tmp_path, monkeypatch):
    project, _ = _fixture(tmp_path, monkeypatch)
    checksum = _run_fetch(project, monkeypatch, "https://example.com/skills/my-skill.md")

    with pytest.raises(IntegrationError, match="not approved"):
        tp.execute_install("skills", _install_args(checksum))


def test_install_fails_on_toctou_checksum_mismatch(tmp_path, monkeypatch):
    project, _ = _fixture(tmp_path, monkeypatch)
    checksum = _run_fetch(project, monkeypatch, "https://example.com/skills/my-skill.md")
    dest = ".claude/skills/thirdparty/my-skill.md"
    _upsert_provenance(project, dest, checksum)

    # Tamper the quarantine record in place: same filename (still named by
    # the ORIGINAL checksum), different content_base64 — the exact TOCTOU
    # scenario D8c exists to close.
    quarantine_path = project / ".trackfw" / "thirdparty-quarantine" / f"{checksum}.json"
    record = json.loads(quarantine_path.read_text())
    record["content_base64"] = "dGFtcGVyZWQtY29udGVudA=="  # base64("tampered-content")
    quarantine_path.write_text(json.dumps(record))

    with pytest.raises(IntegrationError, match="(?i)(toctou|checksum)"):
        tp.execute_install("skills", _install_args(checksum))


def test_install_default_scope_is_project(tmp_path, monkeypatch):
    project, home = _fixture(tmp_path, monkeypatch)
    checksum = _run_fetch(project, monkeypatch, "https://example.com/skills/my-skill.md")
    dest = ".claude/skills/thirdparty/my-skill.md"
    _upsert_provenance(project, dest, checksum)

    # Deliberately no --scope: must default to project (D4), unlike
    # `agents install`/`skills install`, which default to global.
    tp.execute_install("skills", _install_args(checksum, scope=None))

    assert (project / ".claude" / "skills" / "thirdparty" / "my-skill.md").is_file()
    assert not (home / ".claude" / "skills" / "thirdparty" / "my-skill.md").exists()


def test_install_via_agents_cmd_records_agent_kind_and_skill_destination(tmp_path, monkeypatch):
    project, _ = _fixture(tmp_path, monkeypatch)
    checksum = None
    _stub_fetch(monkeypatch)
    tp.execute_fetch("agents", _fetch_args("https://example.com/skills/my-skill.md"))
    files = list((project / ".trackfw" / "thirdparty-quarantine").glob("*.json"))
    assert len(files) == 1
    record = json.loads(files[0].read_text())
    assert record["kind"] == "agent"
    checksum = files[0].stem

    dest = ".claude/skills/thirdparty/my-skill.md"
    _upsert_provenance(project, dest, checksum)

    tp.execute_install("agents", _install_args(checksum))

    assert (project / ".claude" / "skills" / "thirdparty" / "my-skill.md").is_file()
    assert not (project / ".claude" / "agents" / "thirdparty").exists()


def test_install_catalog_agent_byte_identical_except_markers(tmp_path, monkeypatch):
    project, home = _fixture(tmp_path, monkeypatch)
    _install_backend_agent(project, home)
    agent_path = project / ".claude" / "agents" / "trackfw-backend.md"
    before = agent_path.read_bytes()

    checksum = _run_fetch(project, monkeypatch, "https://example.com/skills/my-skill.md")
    dest = ".claude/skills/thirdparty/my-skill.md"
    _upsert_provenance(project, dest, checksum)

    tp.execute_install("skills", _install_args(checksum, apply_to="backend"))

    after = agent_path.read_bytes()
    after_str = after.decode("utf-8")
    start = after_str.find(thirdparty_pkg.THIRDPARTY_REF_START)
    end = after_str.find(thirdparty_pkg.THIRDPARTY_REF_END)
    assert start != -1 and end != -1

    excised = after_str[:start].rstrip("\n") + "\n"
    assert excised == before.decode("utf-8")
    assert dest in after_str

    skill_path = project / ".claude" / "skills" / "thirdparty" / "my-skill.md"
    assert "Example Third-Party Skill" in skill_path.read_text()


def test_install_agents_update_stays_current_after_attach(tmp_path, monkeypatch):
    project, home = _fixture(tmp_path, monkeypatch)
    _install_backend_agent(project, home)

    checksum = _run_fetch(project, monkeypatch, "https://example.com/skills/my-skill.md")
    dest = ".claude/skills/thirdparty/my-skill.md"
    _upsert_provenance(project, dest, checksum)

    tp.execute_install("skills", _install_args(checksum, apply_to="backend"))

    agent_path = project / ".claude" / "agents" / "trackfw-backend.md"
    attached = agent_path.read_bytes()

    ident = identity.load(str(home))
    catalog, plans = plan_deployments(
        "agents", target_ids=["claude"], item_ids=["backend"], scope="project",
        identity_cfg=ident, project_root=str(project),
    )
    manager = IntegrationManager(str(project), home_dir=str(home))
    manager.update(plans, force=False)
    deployment = manager.inspect(plans[0])
    assert deployment["state"] != "modified"
    assert deployment["state"] == "current"

    after_update = agent_path.read_bytes()
    assert after_update == attached


def test_install_apply_to_rejects_hand_modified_agent_before_any_write(tmp_path, monkeypatch):
    project, home = _fixture(tmp_path, monkeypatch)
    _install_backend_agent(project, home)
    agent_path = project / ".claude" / "agents" / "trackfw-backend.md"
    agent_path.write_text("hand-edited content, not trackfw-managed anymore\n")

    checksum = _run_fetch(project, monkeypatch, "https://example.com/skills/my-skill.md")
    dest = ".claude/skills/thirdparty/my-skill.md"
    _upsert_provenance(project, dest, checksum)

    with pytest.raises(IntegrationError, match="modified"):
        tp.execute_install("skills", _install_args(checksum, apply_to="backend"))

    assert not (project / ".claude" / "skills" / "thirdparty" / "my-skill.md").exists()
    assert not (project / ".trackfw" / "thirdparty-references.json").exists()


def test_thirdparty_reachable_from_both_agents_and_skills():
    import argparse as _argparse

    from trackfw.commands import agents as agents_cmd
    from trackfw.commands import skills as skills_cmd

    for register in (agents_cmd.register, skills_cmd.register):
        parser = _argparse.ArgumentParser()
        subparsers = parser.add_subparsers(dest="command")
        register(subparsers)
        args = parser.parse_args(
            ["agents" if register is agents_cmd.register else "skills",
             "third-party", "fetch", "https://example.com/x.md"]
        )
        assert hasattr(args, "func")
        args_install = parser.parse_args(
            ["agents" if register is agents_cmd.register else "skills",
             "third-party", "install", "--checksum", "a" * 64, "--targets", "claude"]
        )
        assert hasattr(args_install, "func")


def test_install_marker_override_required_when_marker_check_failed(tmp_path, monkeypatch):
    project, _ = _fixture(tmp_path, monkeypatch)
    _stub_fetch(monkeypatch, b"# Git authority\n\nboundary redefinition.\n")
    # force-fetch it into quarantine despite the failed marker check.
    tp.execute_fetch("skills", _fetch_args("https://example.com/skills/evil.md", force=True))
    files = sorted((project / ".trackfw" / "thirdparty-quarantine").glob("*.json"))
    checksum = files[-1].stem

    dest = ".claude/skills/thirdparty/evil.md"
    _upsert_provenance(project, dest, checksum, marker_override=False)

    with pytest.raises(IntegrationError, match="marker_override"):
        tp.execute_install("skills", _install_args(checksum, slug="evil"))

    _upsert_provenance(project, dest, checksum, marker_override=True)
    tp.execute_install("skills", _install_args(checksum, slug="evil"))
    assert (project / ".claude" / "skills" / "thirdparty" / "evil.md").is_file()


# ---------------------------------------------------------------------------
# fetch.py — D7 network policy (opener mocked at the urllib.request.build_opener
# boundary; no real sockets/TLS). The redirect-hop counting in particular
# mirrors Go's net/http.Client.CheckRedirect off-by-one (see
# vault/notes/node-https-redirect-checkredirect-off-by-one-2026-08-15.md):
# only 2 redirects are ever followed before the 3rd attempt is refused.
# ---------------------------------------------------------------------------


class _FakeResponse:
    def __init__(self, status: int, headers: dict[str, str], body: bytes):
        self.status = status
        self.headers = Message()
        for key, value in headers.items():
            self.headers[key] = value
        self._body = body

    def read(self, size: int = -1) -> bytes:
        if size is None or size < 0:
            return self._body
        return self._body[:size]

    def getcode(self) -> int:
        return self.status

    def __enter__(self):
        return self

    def __exit__(self, *exc_info):
        return False


def _redirect_error(url: str, code: int, location: str) -> urllib.error.HTTPError:
    headers = Message()
    headers["Location"] = location
    return urllib.error.HTTPError(url, code, "redirect", headers, None)


class _ScriptedOpener:
    """Fake opener standing in for urllib.request.build_opener()'s return
    value: .open() pops the next scripted outcome (an exception instance
    to raise, or a response to return) off a queue, one per hop."""

    def __init__(self, outcomes: list):
        self._outcomes = list(outcomes)

    def open(self, request, timeout=None):
        outcome = self._outcomes.pop(0)
        if isinstance(outcome, BaseException):
            raise outcome
        return outcome


def _patch_opener(monkeypatch, outcomes: list) -> None:
    monkeypatch.setattr(
        urllib.request, "build_opener", lambda *handlers: _ScriptedOpener(outcomes)
    )


def test_fetch_refuses_non_https_scheme():
    with pytest.raises(fetch_mod.ThirdPartyFetchError, match="https"):
        fetch_mod.fetch("http://example.com/skills/x.md")


def test_fetch_refuses_ftp_scheme():
    with pytest.raises(fetch_mod.ThirdPartyFetchError, match="https"):
        fetch_mod.fetch("ftp://example.com/skills/x.md")


def test_fetch_refuses_redirect_downgrade_to_http(monkeypatch):
    outcomes = [_redirect_error("https://example.com/a", 302, "http://example.com/b")]
    _patch_opener(monkeypatch, outcomes)
    with pytest.raises(fetch_mod.ThirdPartyFetchError, match="non-https"):
        fetch_mod.fetch("https://example.com/a")


def test_fetch_follows_two_redirects_then_succeeds(monkeypatch):
    # Off-by-one accounting (Go parity): exactly 2 redirect hops followed
    # is within the allowed budget — must succeed, not refuse.
    outcomes = [
        _redirect_error("https://example.com/hop0", 302, "https://example.com/hop1"),
        _redirect_error("https://example.com/hop1", 302, "https://example.com/hop2"),
        _FakeResponse(200, {"Content-Type": "text/plain"}, b"done"),
    ]
    _patch_opener(monkeypatch, outcomes)
    assert fetch_mod.fetch("https://example.com/hop0") == b"done"


def test_fetch_refuses_third_redirect_off_by_one(monkeypatch):
    # The critical Go-parity boundary: a THIRD redirect attempt (after 2
    # already followed) must be refused, even though MAX_REDIRECTS == 3 —
    # this is the off-by-one documented in the vault note. A naive "count
    # redirects followed, refuse at >= 3" implementation would follow this
    # 3rd hop and only refuse the 4th, diverging from Go here.
    outcomes = [
        _redirect_error("https://example.com/hop0", 302, "https://example.com/hop1"),
        _redirect_error("https://example.com/hop1", 302, "https://example.com/hop2"),
        _redirect_error("https://example.com/hop2", 302, "https://example.com/hop3"),
        _FakeResponse(200, {"Content-Type": "text/plain"}, b"done"),
    ]
    _patch_opener(monkeypatch, outcomes)
    with pytest.raises(fetch_mod.ThirdPartyFetchError, match="redirect"):
        fetch_mod.fetch("https://example.com/hop0")


def test_fetch_refuses_disallowed_content_type(monkeypatch):
    outcomes = [_FakeResponse(200, {"Content-Type": "application/json"}, b"{}")]
    _patch_opener(monkeypatch, outcomes)
    with pytest.raises(fetch_mod.ThirdPartyFetchError, match="Content-Type"):
        fetch_mod.fetch("https://example.com/a.json")


def test_fetch_accepts_content_type_with_charset_suffix(monkeypatch):
    outcomes = [_FakeResponse(200, {"Content-Type": "text/markdown; charset=utf-8"}, b"# hi\n")]
    _patch_opener(monkeypatch, outcomes)
    assert fetch_mod.fetch("https://example.com/a.md") == b"# hi\n"


def test_fetch_refuses_content_over_size_limit(monkeypatch):
    oversized = b"a" * (fetch_mod.MAX_CONTENT_SIZE + 1)
    outcomes = [_FakeResponse(200, {"Content-Type": "text/plain"}, oversized)]
    _patch_opener(monkeypatch, outcomes)
    with pytest.raises(fetch_mod.ThirdPartyFetchError, match="exceeds"):
        fetch_mod.fetch("https://example.com/big.txt")


def test_fetch_refuses_non_200_status(monkeypatch):
    outcomes = [urllib.error.HTTPError("https://example.com/a", 404, "not found", Message(), None)]
    _patch_opener(monkeypatch, outcomes)
    with pytest.raises(fetch_mod.ThirdPartyFetchError, match="404"):
        fetch_mod.fetch("https://example.com/a")


# ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas, ML-3A —
# thirdparty_artifact_has_provenance end-to-end, real command path (not hand-authored fixtures).
# Mirrors internal/commands/integrations_thirdparty_validate_test.go — exists because the rule's
# own unit tests (test_validator_thirdparty_provenance.py) hand-author manifest/provenance JSON,
# and an incorrect key-domain assumption baked into BOTH the rule and its fixtures would pass there
# while still being wrong against the real command (exactly what happened in Go during this ML: the
# rule initially looked up provenance by the manifest's ABSOLUTE destination, but
# verify_approval/upsert_provenance_entry are actually called with the project-relative
# destination).
def test_install_passes_thirdparty_artifact_has_provenance_end_to_end(tmp_path, monkeypatch):
    project, home = _fixture(tmp_path, monkeypatch)
    _install_backend_agent(project, home)

    checksum = _run_fetch(project, monkeypatch, "https://example.com/skills/my-skill.md")
    dest = ".claude/skills/thirdparty/my-skill.md"
    _upsert_provenance(project, dest, checksum)

    tp.execute_install("skills", _install_args(checksum, apply_to="backend"))

    msgs = v.validate_thirdparty_artifact_has_provenance()
    assert msgs == [], "a correctly approved+installed third-party artifact must not trip the rule"

    # Negative counterpart: tamper the installed file, expect the rule to catch it.
    (project / dest).write_text("# Example Third-Party Skill\n\nTAMPERED.\n", encoding="utf-8")
    tampered_msgs = v.validate_thirdparty_artifact_has_provenance()
    assert len(tampered_msgs) == 1, tampered_msgs
    assert "D2 branch ii" in tampered_msgs[0]["message"]
