"""Parity, lifecycle, renderer and package tests for Python integrations."""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
import tomllib
from importlib.resources import files
from pathlib import Path

import pytest

from trackfw.integrations.catalog import load_catalog, plan_deployments
from trackfw.integrations.manager import IntegrationError, IntegrationManager


PYPI_ROOT = Path(__file__).parents[1]


def cli(*arguments: str, cwd: Path, home: Path | None = None):
    environment = dict(os.environ)
    environment["PYTHONPATH"] = str(PYPI_ROOT)
    if home:
        environment["HOME"] = str(home)
    return subprocess.run(
        [sys.executable, "-m", "trackfw", *arguments],
        cwd=cwd,
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )


def test_packaged_catalog_and_assets_are_complete():
    catalog = load_catalog()
    assert catalog["version"] == "1.1.0"
    assert len(catalog["agents"]) == 10
    assert len(catalog["skills"]) == 5
    assert [target["id"] for target in catalog["targets"]] == [
        "claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "kiro"
    ]
    root = files("trackfw.integrations")
    for item in catalog["agents"] + catalog["skills"]:
        assert root.joinpath(item["asset"]).read_bytes()
    pyproject = (PYPI_ROOT / "pyproject.toml").read_text(encoding="utf-8")
    assert "integrations/assets/catalog.json" in pyproject
    assert "integrations/assets/agents/*.md" in pyproject
    assert "integrations/assets/skills/*.md" in pyproject


def test_list_json_has_exact_contract_and_deterministic_order(tmp_path):
    first = cli("agents", "list", "--targets", "codex,claude", "--items", "backend", "--json", cwd=tmp_path)
    second = cli("agents", "list", "--targets", "codex,claude", "--items", "backend", "--json", cwd=tmp_path)
    assert first.returncode == 0, first.stderr
    assert first.stdout == second.stdout
    payload = json.loads(first.stdout)
    assert list(payload) == ["kind", "catalog_version", "items", "deployments"]
    assert payload["kind"] == "agents"
    assert len(payload["items"]) == 10
    assert [deployment["target"] for deployment in payload["deployments"]] == ["claude", "codex"]
    assert list(payload["deployments"][0]) == [
        "target", "surface", "scope", "item", "support_level", "representation", "destination", "state", "managed"
    ]


@pytest.mark.parametrize("kind", ["agents", "skills"])
@pytest.mark.parametrize("action", ["install", "update", "uninstall"])
def test_non_tty_mutation_requires_targets(kind, action, tmp_path):
    result = cli(kind, action, "--json", cwd=tmp_path)
    assert result.returncode == 2
    assert "--targets is required" in result.stderr


def test_cli_install_list_update_and_uninstall_modified(tmp_path):
    arguments = ("agents", "install", "--targets", "claude", "--items", "backend", "--json")
    installed = cli(*arguments, cwd=tmp_path)
    assert installed.returncode == 0, installed.stderr
    destination = tmp_path / ".claude/agents/trackfw-backend.md"
    assert destination.is_file()
    assert json.loads(installed.stdout)["deployments"][0]["state"] == "current"
    destination.write_text("custom", encoding="utf-8")
    listed = cli("agents", "list", "--targets", "claude", "--items", "backend", "--json", cwd=tmp_path)
    assert json.loads(listed.stdout)["deployments"][0]["state"] == "modified"
    protected = cli("agents", "update", "--targets", "claude", "--items", "backend", "--json", cwd=tmp_path)
    assert protected.returncode == 2
    assert destination.read_text(encoding="utf-8") == "custom"
    forced = cli("agents", "update", "--targets", "claude", "--items", "backend", "--force", "--json", cwd=tmp_path)
    assert forced.returncode == 0, forced.stderr
    destination.write_text("custom again", encoding="utf-8")
    protected = cli("agents", "uninstall", "--targets", "claude", "--items", "backend", "--json", cwd=tmp_path)
    assert protected.returncode == 2
    removed = cli("agents", "uninstall", "--targets", "claude", "--items", "backend", "--force", "--json", cwd=tmp_path)
    assert removed.returncode == 0, removed.stderr
    assert not destination.exists()


def test_shared_skill_claim_preserves_physical_artifact(tmp_path):
    manager = IntegrationManager(tmp_path)
    _, codex = plan_deployments("skills", ["codex"], ["implement"], "project")
    _, antigravity = plan_deployments("skills", ["antigravity"], ["implement"], "project")
    assert codex[0]["destination"] == antigravity[0]["destination"]
    manager.install(codex + antigravity)
    destination = tmp_path / codex[0]["destination"]
    manager.uninstall(codex)
    assert destination.exists()
    assert manager.inspect(antigravity[0])["managed"] is True
    manager.uninstall(antigravity)
    assert not destination.exists()


def test_project_and_global_use_separate_manifests(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    manager = IntegrationManager(tmp_path, home)
    _, project_plans = plan_deployments("agents", ["claude"], ["backend"], "project")
    _, global_plans = plan_deployments("agents", ["claude"], ["backend"], "global")
    manager.install(project_plans + global_plans)
    project_manifest = json.loads((tmp_path / ".trackfw/integrations-manifest.json").read_text())
    global_manifest = json.loads((home / ".trackfw/integrations-manifest.json").read_text())
    assert len(project_manifest["artifacts"]) == 1
    assert len(global_manifest["artifacts"]) == 1
    assert all(path.startswith(str(tmp_path)) for path in project_manifest["artifacts"])
    assert all(path.startswith(str(home)) for path in global_manifest["artifacts"])


def test_update_force_never_claims_unknown_unmanaged_file(tmp_path):
    manager = IntegrationManager(tmp_path)
    _, plans = plan_deployments("agents", ["claude"], ["backend"], "project")
    destination = tmp_path / plans[0]["destination"]
    destination.parent.mkdir(parents=True)
    destination.write_bytes(b"user-owned")
    with pytest.raises(IntegrationError):
        manager.update(plans, force=True)
    assert destination.read_bytes() == b"user-owned"
    assert not (tmp_path / ".trackfw/integrations-manifest.json").exists()


def test_legacy_adoption_then_update(tmp_path):
    manager = IntegrationManager(tmp_path)
    _, plans = plan_deployments("agents", ["claude"], ["backend"], "project")
    legacy = b"old canonical bytes"
    plans[0]["legacy_hashes"] = [hashlib.sha256(legacy).hexdigest()]
    destination = tmp_path / plans[0]["destination"]
    destination.parent.mkdir(parents=True)
    destination.write_bytes(legacy)
    manager.install(plans)
    assert destination.read_bytes() == legacy
    assert manager.inspect(plans[0])["state"] == "outdated"
    manager.update(plans)
    assert destination.read_bytes() == plans[0]["content"]


@pytest.mark.parametrize(
    "scope,destination",
    [("project", "../escape.md"), ("global", "/tmp/escape-trackfw.md"), ("project", "bad\x00name.md")],
)
def test_manager_rejects_unsafe_destinations(tmp_path, scope, destination):
    plan = {
        "claim": {"target": "x", "surface": "x", "scope": scope, "kind": "agents", "item": "x"},
        "destination": destination,
        "content": b"x",
        "catalog_version": "1",
        "support_level": "native",
        "representation": "markdown",
        "legacy_hashes": [],
    }
    with pytest.raises(IntegrationError):
        IntegrationManager(tmp_path, tmp_path / "home").install([plan])


def test_manager_rejects_symlink_parent(tmp_path):
    outside = tmp_path / "outside"
    outside.mkdir()
    (tmp_path / "linked").symlink_to(outside, target_is_directory=True)
    plan = {
        "claim": {"target": "x", "surface": "x", "scope": "project", "kind": "agents", "item": "x"},
        "destination": "linked/file.md",
        "content": b"x",
        "catalog_version": "1",
        "support_level": "native",
        "representation": "markdown",
        "legacy_hashes": [],
    }
    with pytest.raises(IntegrationError, match="symlink"):
        IntegrationManager(tmp_path).install([plan])


def test_renderers_emit_native_toml_json_and_markdown():
    _, codex = plan_deployments("agents", ["codex"], ["backend"], "project")
    parsed_toml = tomllib.loads(codex[0]["content"].decode())
    assert parsed_toml["name"] == "backend"
    assert "developer_instructions" in parsed_toml
    _, amazon = plan_deployments("agents", ["amazonq"], ["backend"], "project")
    assert json.loads(amazon[0]["content"])["name"] == "trackfw-backend"
    _, antigravity = plan_deployments("agents", ["antigravity"], ["backend"], "project", {"antigravity": "legacy-cli"})
    assert json.loads(antigravity[0]["content"])["prompt"]
    _, claude = plan_deployments("agents", ["claude"], ["backend"], "project")
    assert claude[0]["content"].startswith(b"---\n")


def test_surface_selection_and_default_skip_legacy():
    _, default = plan_deployments("agents", ["antigravity"], ["backend"], "project")
    _, legacy = plan_deployments("agents", ["antigravity"], ["backend"], "project", {"antigravity": "legacy-cli"})
    assert default[0]["claim"]["surface"] == "current"
    assert default[0]["destination"].endswith("agent.md")
    assert legacy[0]["claim"]["surface"] == "legacy-cli"
    assert legacy[0]["destination"].endswith("agent.json")


def test_init_ai_tools_uses_integration_engine_for_all_targets(tmp_path):
    result = cli("init", "--project-name", "example", "--ai-tools", "cursor", cwd=tmp_path)
    assert result.returncode == 0, result.stderr
    assert (tmp_path / ".cursor/agents/trackfw-backend.md").is_file()
    assert (tmp_path / ".cursor/skills/trackfw-implement/SKILL.md").is_file()
