"""Parity, lifecycle, renderer and package tests for Python integrations."""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
from importlib.resources import files
from pathlib import Path

import pytest

from trackfw.integrations.catalog import _surfaces, load_catalog, plan_deployments
from trackfw.integrations.command import _prompt_ambiguous_surfaces
from trackfw.integrations.manager import IntegrationError, IntegrationManager
from trackfw.generators.codex import AGENTS as LEGACY_PYTHON_AGENTS


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
    canonical = PYPI_ROOT.parent / "internal/integrations/assets"
    packaged = PYPI_ROOT / "trackfw/integrations/assets"
    canonical_files = sorted(path.relative_to(canonical) for path in canonical.rglob("*") if path.is_file())
    packaged_files = sorted(path.relative_to(packaged) for path in packaged.rglob("*") if path.is_file())
    assert packaged_files == canonical_files
    for relative in canonical_files:
        assert (packaged / relative).read_bytes() == (canonical / relative).read_bytes()


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


def test_list_without_surface_includes_current_and_legacy_surfaces(tmp_path):
    result = cli("agents", "list", "--targets", "antigravity", "--items", "backend", "--json", cwd=tmp_path)
    assert result.returncode == 0, result.stderr
    deployments = json.loads(result.stdout)["deployments"]
    assert [deployment["surface"] for deployment in deployments] == ["current", "legacy-cli"]


def test_human_list_includes_available_catalog_and_deployments(tmp_path):
    result = cli("skills", "list", "--targets", "claude", "--items", "implement", cwd=tmp_path)
    assert result.returncode == 0, result.stderr
    assert "Available skills (catalog 1.1.0):" in result.stdout
    assert "governance" in result.stdout
    assert "Deployments:" in result.stdout
    assert "claude/cli" in result.stdout


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


def test_reads_canonical_go_manifest_fixture(tmp_path):
    manager = IntegrationManager(tmp_path)
    _, plans = plan_deployments("agents", ["claude"], ["backend"], "project")
    plan = plans[0]
    destination = tmp_path / plan["destination"]
    destination.parent.mkdir(parents=True)
    destination.write_bytes(plan["content"])
    manifest_path = tmp_path / ".trackfw/integrations-manifest.json"
    manifest_path.parent.mkdir(parents=True)
    manifest_path.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "artifacts": {
                    str(destination): {
                        "destination": str(destination),
                        "sha256": hashlib.sha256(plan["content"]).hexdigest(),
                        "catalog_version": plan["catalog_version"],
                        "claims": [plan["claim"]],
                    }
                },
            }
        ),
        encoding="utf-8",
    )
    assert manager.inspect(plan)["state"] == "current"
    assert manager.inspect(plan)["managed"] is True


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


def test_released_claude_hashes_are_global_only():
    historical_root = PYPI_ROOT.parent / "internal/generators/templates/agents"
    _, global_plans = plan_deployments("agents", ["claude"], scope="global")
    assert len(global_plans) == 10
    for plan in global_plans:
        historical = (historical_root / f"trackfw-{plan['claim']['item']}.md").read_bytes()
        assert hashlib.sha256(historical).hexdigest() in plan["legacy_hashes"]
    _, project = plan_deployments("agents", ["claude"], ["backend"], "project")
    _, codex_global = plan_deployments("agents", ["codex"], ["backend"], "global")
    assert project[0]["legacy_hashes"] == []
    assert codex_global[0]["legacy_hashes"] == []


def test_codex_legacy_union_recognizes_go_npm_and_python_bytes(tmp_path):
    _, plans = plan_deployments("agents", ["codex"], ["backend"], "project")
    plan = plans[0]
    fixtures = {
        "go": b'''name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """
Implement only the assigned backend scope. Preserve public contracts and trackfw traceability.
Run focused tests and report changed files, validation evidence, and remaining risks.
"""
''',
        "npm": b'''name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """
Implement only the assigned backend scope, preserve contracts and traceability, run focused tests, and report changed files and evidence.
"""
''',
        "python": (LEGACY_PYTHON_AGENTS["trackfw-backend.toml"].strip() + "\n").encode(),
    }
    for producer, content in fixtures.items():
        assert hashlib.sha256(content).hexdigest() in plan["legacy_hashes"], producer
        root = tmp_path / producer
        destination = root / plan["destination"]
        destination.parent.mkdir(parents=True)
        destination.write_bytes(content)
        inspection = IntegrationManager(root).inspect(plan)
        assert (inspection["state"], inspection["managed"]) == ("outdated", False)


def test_released_python_codex_is_adopted_without_overwrite_then_updated(tmp_path):
    _, plans = plan_deployments("agents", ["codex"], ["backend"], "project")
    plan = plans[0]
    legacy = (LEGACY_PYTHON_AGENTS["trackfw-backend.toml"].strip() + "\n").encode()
    destination = tmp_path / plan["destination"]
    destination.parent.mkdir(parents=True)
    destination.write_bytes(legacy)
    manager = IntegrationManager(tmp_path)
    assert (manager.inspect(plan)["state"], manager.inspect(plan)["managed"]) == ("outdated", False)
    manager.install(plans)
    assert destination.read_bytes() == legacy
    assert (manager.inspect(plan)["state"], manager.inspect(plan)["managed"]) == ("outdated", True)
    manifest = json.loads((tmp_path / ".trackfw/integrations-manifest.json").read_text())
    assert manifest["artifacts"][str(destination)]["catalog_version"] == "legacy"
    manager.update(plans)
    assert destination.read_bytes() == plan["content"]
    assert (manager.inspect(plan)["state"], manager.inspect(plan)["managed"]) == ("current", True)


def test_update_alias_preserves_unknown_codex_bytes_and_warns(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    (tmp_path / "trackfw.yaml").write_text("hooks: none\nci: none\n", encoding="utf-8")
    unknown = tmp_path / ".codex/agents/trackfw-backend.toml"
    unknown.parent.mkdir(parents=True)
    unknown.write_bytes(b"user-owned unknown bytes\n")
    result = cli("update", cwd=tmp_path, home=home)
    assert result.returncode == 0, result.stderr
    assert unknown.read_bytes() == b"user-owned unknown bytes\n"
    assert "Codex integration" in result.stdout
    assert "unmanaged artifact" in result.stdout.lower()


def test_update_alias_converts_only_present_codex_artifacts(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    (tmp_path / "trackfw.yaml").write_text("hooks: none\nci: none\n", encoding="utf-8")
    backend = tmp_path / ".codex/agents/trackfw-backend.toml"
    backend.parent.mkdir(parents=True)
    backend.write_text(LEGACY_PYTHON_AGENTS["trackfw-backend.toml"].strip() + "\n", encoding="utf-8")
    result = cli("update", cwd=tmp_path, home=home)
    assert result.returncode == 0, result.stderr
    _, plans = plan_deployments("agents", ["codex"], ["backend"], "project")
    assert backend.read_bytes() == plans[0]["content"]
    assert not (tmp_path / ".codex/agents/trackfw-qa.toml").exists()
    assert not (tmp_path / ".agents/skills/trackfw-governance/SKILL.md").exists()


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
    codex_toml = codex[0]["content"]
    assert codex_toml.startswith(b'name = "trackfw_backend"\n')
    assert b'\ndeveloper_instructions = "' in codex_toml
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


def test_default_surface_selection_is_specific_to_kind():
    target = {
        "id": "mixed",
        "surfaces": [
            {
                "id": "skill-current",
                "capabilities": {
                    "agents": {"support_level": "unsupported"},
                    "skills": {"support_level": "native"},
                },
            },
            {
                "id": "agent-current",
                "capabilities": {
                    "agents": {"support_level": "native"},
                    "skills": {"support_level": "legacy"},
                },
            },
        ],
    }
    assert _surfaces(target, "agents", {}, False)[0]["id"] == "agent-current"
    assert _surfaces(target, "skills", {}, False)[0]["id"] == "skill-current"


def test_tty_prompts_for_ambiguous_nonlegacy_surface(monkeypatch):
    catalog = load_catalog()
    selected = {}
    monkeypatch.setattr("builtins.input", lambda _prompt: "2")
    _prompt_ambiguous_surfaces(catalog, "agents", ["kiro"], selected)
    assert selected == {"kiro": "cli"}


def test_init_ai_tools_uses_integration_engine_for_all_targets(tmp_path):
    result = cli("init", "--project-name", "example", "--ai-tools", "cursor", cwd=tmp_path)
    assert result.returncode == 0, result.stderr
    assert (tmp_path / ".cursor/agents/trackfw-backend.md").is_file()
    assert (tmp_path / ".cursor/skills/trackfw-implement/SKILL.md").is_file()
