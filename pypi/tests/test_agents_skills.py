"""Parity, lifecycle, renderer and package tests for Python integrations."""

from __future__ import annotations

import errno
import hashlib
import json
import os
import subprocess
import sys
from importlib.resources import files
from pathlib import Path

import pytest


def _symlink_or_skip(link: Path, target: Path) -> None:
    """Guarda de capacidade: cria link.symlink_to(target); pytest.skip se o
    processo não tem privilégio (WinError 1314 / EPERM / EACCES). Qualquer
    outro OSError é re-lançado — a guarda discrimina "sem privilégio" de
    "falhou por outro motivo".

    A detecção é pela CONDIÇÃO (falha de privilégio), não por sys.platform:
    num Windows com Developer Mode habilitado, symlink_to tem sucesso e o
    teste executa de verdade.
    """
    try:
        link.symlink_to(str(target))
    except OSError as err:
        winerror = getattr(err, 'winerror', None)
        if winerror == 1314 or err.errno in (errno.EPERM, errno.EACCES):
            pytest.skip(
                'guarda de symlink não exercitada: criação de symlink exige '
                f'Developer Mode (ou processo elevado) neste Windows: {err}'
            )
        raise

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
    assert len(catalog["agents"]) == 12
    assert len(catalog["skills"]) == 17
    assert [target["id"] for target in catalog["targets"]] == [
        "claude", "codex", "gemini", "antigravity", "cursor", "copilot", "windsurf", "amazonq", "opencode", "kiro"
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
    assert len(payload["items"]) == 12
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
@pytest.mark.parametrize("action", ["install", "update"])
def test_non_tty_mutation_requires_targets(kind, action, tmp_path):
    result = cli(kind, action, "--json", cwd=tmp_path)
    assert result.returncode == 2
    assert "--targets is required" in result.stderr


@pytest.mark.parametrize("kind", ["agents", "skills"])
def test_non_tty_uninstall_without_scope_requires_scope_before_targets(kind, tmp_path):
    # ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills,
    # D8: uninstall's scope gate runs before the --targets check, so a
    # non-interactive uninstall with neither flag fails on --scope first —
    # matching resolve_scope() running ahead of the --targets branch in
    # run() (mirrors internal/commands/integrations_flags.go's ordering).
    result = cli(kind, "uninstall", "--json", cwd=tmp_path)
    assert result.returncode == 2
    assert "uninstall requires --scope in non-interactive mode" in result.stderr


@pytest.mark.parametrize("kind", ["agents", "skills"])
def test_non_tty_uninstall_with_scope_still_requires_targets(kind, tmp_path):
    result = cli(kind, "uninstall", "--scope", "global", "--json", cwd=tmp_path)
    assert result.returncode == 2
    assert "--targets is required" in result.stderr


def test_cli_install_list_update_and_uninstall_modified(tmp_path):
    # --scope project is explicit and required here (ADR-2026-07-25-escopo-
    # de-instalacao-selecionavel-para-agents-e-skills, D1): without it, a
    # non-TTY subprocess now defaults to `global` (~/.claude/...) instead of
    # writing under `tmp_path`, and this test asserts a project-scope path.
    arguments = ("agents", "install", "--targets", "claude", "--items", "backend", "--scope", "project", "--json")
    installed = cli(*arguments, cwd=tmp_path)
    assert installed.returncode == 0, installed.stderr
    destination = tmp_path / ".claude/agents/trackfw-backend.md"
    assert destination.is_file()
    assert json.loads(installed.stdout)["deployments"][0]["state"] == "current"
    destination.write_text("custom", encoding="utf-8")
    listed = cli(
        "agents", "list", "--targets", "claude", "--items", "backend", "--scope", "project", "--json", cwd=tmp_path
    )
    assert json.loads(listed.stdout)["deployments"][0]["state"] == "modified"
    protected = cli(
        "agents", "update", "--targets", "claude", "--items", "backend", "--scope", "project", "--json", cwd=tmp_path
    )
    assert protected.returncode == 2
    assert destination.read_text(encoding="utf-8") == "custom"
    forced = cli(
        "agents", "update", "--targets", "claude", "--items", "backend",
        "--scope", "project", "--force", "--json", cwd=tmp_path,
    )
    assert forced.returncode == 0, forced.stderr
    destination.write_text("custom again", encoding="utf-8")
    protected = cli(
        "agents", "uninstall", "--targets", "claude", "--items", "backend",
        "--scope", "project", "--json", cwd=tmp_path,
    )
    assert protected.returncode == 2
    removed = cli(
        "agents", "uninstall", "--targets", "claude", "--items", "backend",
        "--scope", "project", "--force", "--json", cwd=tmp_path,
    )
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


def test_install_skips_owned_outdated_artifact(tmp_path):
    """ML-2C: outdated+owned+sem-force → skip; bytes preservados; lote continua; on_skip=None seguro."""
    home = tmp_path / "home"
    home.mkdir()

    # Plan A — escopo global (gemini/backend): será adotado e depois pulado
    _, plans_a = plan_deployments("agents", ["gemini"], ["backend"], "global")
    plan_a = {**plans_a[0]}
    legacy = b"old template bytes for skip test"
    plan_a["legacy_hashes"] = [hashlib.sha256(legacy).hexdigest()]

    destination_a = home / plan_a["destination"][2:]  # strip "~/"
    destination_a.parent.mkdir(parents=True, exist_ok=True)
    destination_a.write_bytes(legacy)

    # Primeira instalação: adota o artefato legado (owned=True, state=outdated)
    IntegrationManager(tmp_path, home).install([plan_a])
    assert IntegrationManager(tmp_path, home).inspect(plan_a)["state"] == "outdated"
    assert IntegrationManager(tmp_path, home).inspect(plan_a)["managed"] is True

    # Plan B — escopo de projeto (claude/backend): deve ser aplicado normalmente no mesmo lote
    _, plans_b = plan_deployments("agents", ["claude"], ["backend"], "project")
    plan_b = plans_b[0]

    # (c) on_skip chamado exatamente uma vez com caminho tilde-abreviado
    skipped: list[tuple[str, str]] = []
    IntegrationManager(tmp_path, home, on_skip=lambda d, r: skipped.append((d, r))).install(
        [plan_a, plan_b]
    )  # (a) sem exceção

    # (b) bytes de plan_a preservados
    assert destination_a.read_bytes() == legacy
    # (c) on_skip chamado exatamente uma vez, caminho tilde-abreviado (global → "~/...")
    assert len(skipped) == 1
    assert skipped[0][0] == plan_a["destination"], f"esperado {plan_a['destination']!r}, obtido {skipped[0][0]!r}"
    assert skipped[0][0].startswith("~/"), f"caminho global deve ter tilde, obtido {skipped[0][0]!r}"
    # (c2) reason é a linha completa (não etiqueta) — escopo global → "trackfw update harness"
    expected_reason_a = (
        f"warning: skipping outdated artifact {skipped[0][0]};"
        " run 'trackfw update harness' to refresh it"
    )
    assert skipped[0][1] == expected_reason_a, (
        f"reason incorreta.\nEsperado: {expected_reason_a!r}\nObtido:   {skipped[0][1]!r}"
    )
    # (d) plan_b aplicado normalmente
    assert (tmp_path / plan_b["destination"]).is_file()

    # (e) on_skip=None não causa erro
    IntegrationManager(tmp_path, home).install([plan_a])
    assert destination_a.read_bytes() == legacy  # bytes ainda preservados (ainda pulado)


def test_install_skip_mixed_scope_batch(tmp_path):
    """ML-2E: lote de escopo misto — cada artefato pulado recebe remediação correta por artefato.

    Prova que a derivação ocorre por plan["claim"]["scope"], não via closure sobre escopo de comando.
    Um lote de escopo uniforme é correto por acidente (closure sobre scope); apenas o lote misto
    discrimina a implementação por artefato da implementação por closure.
    """
    home = tmp_path / "home"
    home.mkdir()

    # Plan A — global (gemini/backend): será adotado via legacy_hashes + depois pulado
    _, plans_a = plan_deployments("agents", ["gemini"], ["backend"], "global")
    plan_a = {**plans_a[0]}
    legacy_a = b"old template bytes for mixed-scope skip test"
    plan_a["legacy_hashes"] = [hashlib.sha256(legacy_a).hexdigest()]

    destination_a = home / plan_a["destination"][2:]  # strip "~/"
    destination_a.parent.mkdir(parents=True, exist_ok=True)
    destination_a.write_bytes(legacy_a)

    # Adota plan_a: owned=True, state=outdated (legacy bytes no disco)
    IntegrationManager(tmp_path, home).install([plan_a])
    assert IntegrationManager(tmp_path, home).inspect(plan_a)["state"] == "outdated"
    assert IntegrationManager(tmp_path, home).inspect(plan_a)["managed"] is True

    # Plan B — project (claude/backend): instala normalmente, depois simulamos outdated
    _, plans_b = plan_deployments("agents", ["claude"], ["backend"], "project")
    plan_b = plans_b[0]
    IntegrationManager(tmp_path, home).install([plan_b])
    assert IntegrationManager(tmp_path, home).inspect(plan_b)["state"] == "current"

    # Simular outdated para plan_b: alterar catalog_version no manifesto de projeto
    project_manifest = tmp_path / ".trackfw" / "integrations-manifest.json"
    manifest_data = json.loads(project_manifest.read_text(encoding="utf-8"))
    dest_key_b = str(tmp_path / plan_b["destination"])
    manifest_data["artifacts"][dest_key_b]["catalog_version"] = "simulated-old"
    project_manifest.write_text(json.dumps(manifest_data, indent=2), encoding="utf-8")
    assert IntegrationManager(tmp_path, home).inspect(plan_b)["state"] == "outdated"
    assert IntegrationManager(tmp_path, home).inspect(plan_b)["managed"] is True

    # Lote misto: plan_a (global) e plan_b (project) — ambos outdated+owned → ambos pulados
    skipped: list[tuple[str, str]] = []
    IntegrationManager(tmp_path, home, on_skip=lambda d, r: skipped.append((d, r))).install(
        [plan_a, plan_b]
    )

    # Ambos devem ter sido pulados
    assert len(skipped) == 2, f"esperado 2 skips, obtido {len(skipped)}: {skipped}"

    # Indexar por destination para ordem-independente
    skipped_by_dest = {d: r for d, r in skipped}

    dest_a = plan_a["destination"]  # "~/.gemini/agents/trackfw-backend.md"
    dest_b = plan_b["destination"]  # ".claude/agents/trackfw-backend.md"
    assert dest_a in skipped_by_dest, f"plan_a não pulado; skips: {list(skipped_by_dest)}"
    assert dest_b in skipped_by_dest, f"plan_b não pulado; skips: {list(skipped_by_dest)}"

    # Remediação por artefato (diferente do que uma closure sobre escopo uniforme produziria)
    assert "trackfw update harness" in skipped_by_dest[dest_a], (
        f"plan_a (global) deve usar 'trackfw update harness';\n"
        f"reason obtida: {skipped_by_dest[dest_a]!r}"
    )
    assert "trackfw update harness" not in skipped_by_dest[dest_b], (
        f"plan_b (project) NÃO deve usar 'trackfw update harness';\n"
        f"reason obtida: {skipped_by_dest[dest_b]!r}"
    )
    assert "'trackfw update'" in skipped_by_dest[dest_b] or "run 'trackfw update'" in skipped_by_dest[dest_b], (
        f"plan_b (project) deve usar 'trackfw update';\n"
        f"reason obtida: {skipped_by_dest[dest_b]!r}"
    )


def test_install_skip_warning_string_project_scope(tmp_path):
    """Warning stderr byte-idêntico ao contrato — escopo de projeto (docs/cli-parity.md §install sobre artefato gerenciado desatualizado)."""
    home = tmp_path / "home"
    home.mkdir()
    (tmp_path / "trackfw.yaml").write_text("hooks: none\nci: none\n", encoding="utf-8")

    _, plans = plan_deployments("agents", ["claude"], ["backend"], "project")
    plan = plans[0]

    # Instalar normalmente para obter owned+current
    IntegrationManager(tmp_path, home).install([plan])

    # Simular outdated: alterar catalog_version no manifesto sem tocar os bytes
    manifest_path = tmp_path / ".trackfw/integrations-manifest.json"
    manifest_data = json.loads(manifest_path.read_text(encoding="utf-8"))
    dest_key = str(tmp_path / plan["destination"])
    manifest_data["artifacts"][dest_key]["catalog_version"] = "simulated-old"
    manifest_path.write_text(json.dumps(manifest_data, indent=2), encoding="utf-8")

    assert IntegrationManager(tmp_path, home).inspect(plan)["state"] == "outdated"
    assert IntegrationManager(tmp_path, home).inspect(plan)["managed"] is True

    result = cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "backend",
        "--scope", "project",
        cwd=tmp_path,
        home=home,
    )
    assert result.returncode == 0, result.stderr

    # String pinada no contrato (docs/cli-parity.md §install sobre artefato gerenciado desatualizado)
    expected = (
        "warning: skipping outdated artifact .claude/agents/trackfw-backend.md;"
        " run 'trackfw update' to refresh it"
    )
    assert expected in result.stderr, (
        f"esperado aviso pinado no contrato em stderr.\n"
        f"Esperado: {expected!r}\n"
        f"Stderr obtido:\n{result.stderr}"
    )


def test_released_claude_hashes_are_global_only():
    historical_root = PYPI_ROOT.parent / "internal/generators/templates/agents"
    _, global_plans = plan_deployments("agents", ["claude"], scope="global")
    assert len(global_plans) == 12
    for plan in global_plans:
        historical_path = historical_root / f"trackfw-{plan['claim']['item']}.md"
        # New agents (e.g. iac, tooling) have no historical fixture — skip legacy hash check.
        if not historical_path.exists():
            continue
        historical = historical_path.read_bytes()
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
    # Guarda de capacidade: sem privilégio → pytest.skip; outro erro → re-raise.
    # O symlink precede um pytest.raises — sem guarda, um PermissionError do
    # symlink_to substituiria o IntegrationError esperado e o teste passaria
    # pelo motivo errado.
    _symlink_or_skip(tmp_path / "linked", outside)
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
    # `init` has no --scope flag and this subprocess has no controlling TTY,
    # so resolve_scope(None) takes the "global" default (ADR-2026-07-25-
    # escopo-de-instalacao-selecionavel-para-agents-e-skills, D1/D4) —
    # artifacts land under HOME, not under the project (tmp_path). `home` is
    # pinned to an isolated tmp directory so this never touches the real
    # user's ~/.cursor.
    home = tmp_path / "home"
    home.mkdir()
    result = cli("init", "--project-name", "example", "--ai-tools", "cursor", cwd=tmp_path, home=home)
    assert result.returncode == 0, result.stderr
    assert (home / ".cursor/agents/trackfw-backend.md").is_file()
    assert (home / ".cursor/skills/trackfw-implement/SKILL.md").is_file()


def test_antigravity_current_surface_renders_agent_directory():
    """Surface 'current' do antigravity deve reconstruir o frontmatter no formato agent-directory.

    - architect (model opus): mapeia para pro, recebe SET_ARCH (14 tools).
    - backend (model sonnet): mapeia para flash, recebe SET_IMPL (10 tools).
    - Modelos originais (opus, sonnet) e IDs proibidos nunca devem aparecer.
    """
    # IDs proibidos — nunca devem aparecer no output
    forbidden_ids = [
        "edit_file", "read_file", "find",
        "view_code_item", "view_file_outline", "call_mcp_tool",
    ]

    # --- architect: model opus → pro, SET_ARCH (14 tools) ---
    _, plans = plan_deployments("agents", ["antigravity"], ["architect"], "project")
    assert len(plans) == 1
    assert plans[0]["claim"]["surface"] == "current"
    content = plans[0]["content"].decode("utf-8")

    assert "model: pro" in content, f"esperado 'model: pro', output:\n{content}"
    assert "opus" not in content, f"'opus' não deve aparecer no output:\n{content}"

    arch_tools = [
        "view_file", "list_dir", "grep_search", "search_web",
        "read_url_content", "write_to_file", "replace_file_content",
        "run_command", "command_status", "generate_image",
        "send_message", "define_subagent", "invoke_subagent", "schedule",
    ]
    for tool in arch_tools:
        assert f"  - {tool}" in content, f"tool '{tool}' ausente no output do architect:\n{content}"

    for forbidden in forbidden_ids:
        assert forbidden not in content, f"ID proibido '{forbidden}' presente no output do architect:\n{content}"

    # --- backend: model sonnet → flash, SET_IMPL (10 tools, sem define_subagent) ---
    _, plans = plan_deployments("agents", ["antigravity"], ["backend"], "project")
    assert len(plans) == 1
    assert plans[0]["claim"]["surface"] == "current"
    content = plans[0]["content"].decode("utf-8")

    assert "model: flash" in content, f"esperado 'model: flash', output:\n{content}"
    assert "sonnet" not in content, f"'sonnet' não deve aparecer no output:\n{content}"

    impl_tools = [
        "view_file", "list_dir", "grep_search", "search_web",
        "read_url_content", "write_to_file", "replace_file_content",
        "run_command", "command_status", "generate_image",
    ]
    for tool in impl_tools:
        assert f"  - {tool}" in content, f"tool '{tool}' ausente no output do backend:\n{content}"

    assert "define_subagent" not in content, f"'define_subagent' não deve aparecer no SET_IMPL:\n{content}"

    for forbidden in forbidden_ids:
        assert forbidden not in content, f"ID proibido '{forbidden}' presente no output do backend:\n{content}"


# ---------------------------------------------------------------------------
# ML-2C — agents install registers agent in trackfw.yaml for by_agent projects
# ---------------------------------------------------------------------------

def _by_agent_yaml(extra_keys: str = "") -> str:
    """Minimal trackfw.yaml with roadmap_namespacing: by_agent."""
    return (
        "# trackfw project config\n"
        "req_dir: docs/req\n"
        "roadmap_dir: docs/roadmaps\n"
        "roadmap_namespacing: by_agent\n"
        f"{extra_keys}"
    )


def _flat_yaml() -> str:
    """Minimal trackfw.yaml with roadmap_namespacing: flat (default)."""
    return (
        "req_dir: docs/req\n"
        "roadmap_dir: docs/roadmaps\n"
        "roadmap_namespacing: flat\n"
    )


def test_agents_install_registers_agent_in_by_agent_yaml_idempotent(tmp_path):
    """
    AC1 / AC8 (installed appears) — After ``trackfw agents install`` in a
    by_agent project the installed agent ID appears in ``agents:`` exactly
    once.  Running a second install leaves exactly one entry (idempotent).

    Reconciliation: asserts that ``agents: install`` in a by_agent project
    writes the installed item ID into ``agents:`` and does not duplicate it
    on a second call — the core behavioural contract of ML-2C.
    """
    (tmp_path / "trackfw.yaml").write_text(_by_agent_yaml(), encoding="utf-8")
    home = tmp_path / "home"
    home.mkdir()

    result = cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "backend",
        "--scope", "project",
        "--json",
        cwd=tmp_path,
        home=home,
    )
    assert result.returncode == 0, result.stderr

    content = (tmp_path / "trackfw.yaml").read_text(encoding="utf-8")
    assert "agents:" in content, "agents: key must appear after install in by_agent project"
    # Count occurrences — a substring check would pass with two entries
    assert content.count("- backend") == 1, (
        f"expected exactly one '- backend' entry, got:\n{content}"
    )

    # Second install — must stay idempotent
    cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "backend",
        "--scope", "project",
        "--json",
        cwd=tmp_path,
        home=home,
    )
    content_after = (tmp_path / "trackfw.yaml").read_text(encoding="utf-8")
    assert content_after.count("- backend") == 1, (
        f"idempotency violated — duplicate entry after second install:\n{content_after}"
    )


def test_agents_install_does_not_create_agents_key_in_flat_project(tmp_path):
    """
    AC2 / AC8 (flat does not create key) — In a flat project ``trackfw agents
    install`` must NOT write an ``agents:`` key to ``trackfw.yaml``.

    Reconciliation: asserts that the by_agent guard fires correctly for a flat
    project — the complement of AC1 that falsifies AC2 in the opposite
    direction.
    """
    (tmp_path / "trackfw.yaml").write_text(_flat_yaml(), encoding="utf-8")
    before = (tmp_path / "trackfw.yaml").read_bytes()
    home = tmp_path / "home"
    home.mkdir()

    result = cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "backend",
        "--scope", "project",
        "--json",
        cwd=tmp_path,
        home=home,
    )
    assert result.returncode == 0, result.stderr

    content = (tmp_path / "trackfw.yaml").read_text(encoding="utf-8")
    # Assert the string "agents:" is absent — not just that the install
    # returned 0 — so the test actually falsifies the flat-guard.
    assert "agents:" not in content, (
        f"agents: key must NOT appear in flat project, got:\n{content}"
    )


def test_agents_install_yaml_diff_touches_only_agents_block(tmp_path):
    """
    AC3 — With a fixture that has comments, a trailing comment after agents:,
    and non-alphabetical key order, a diff after install shows only the agents:
    block changing.  Every other byte is preserved.

    Reconciliation: asserts that text-level splicing does not rewrite keys it
    did not intend to touch — the formatting-preservation claim of ML-2C.
    """
    # Fixture: comments + non-alphabetical key order + agents: with one entry
    fixture = (
        "# project settings (do not sort keys)\n"
        "roadmap_dir: docs/roadmaps\n"  # before req_dir — intentionally non-alphabetical
        "req_dir: docs/req\n"
        "roadmap_namespacing: by_agent\n"
        "agents:\n"
        "  - architect\n"
        "# end of file\n"
    )
    (tmp_path / "trackfw.yaml").write_text(fixture, encoding="utf-8")
    home = tmp_path / "home"
    home.mkdir()

    result = cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "backend",
        "--scope", "project",
        "--json",
        cwd=tmp_path,
        home=home,
    )
    assert result.returncode == 0, result.stderr

    after = (tmp_path / "trackfw.yaml").read_text(encoding="utf-8")
    assert "- backend" in after, "installed agent must appear in agents: block"

    # Remove the single inserted line and compare byte-for-byte with the
    # original.  This is the AC3 claim: "diff shows only agents: changing."
    reconstructed = after.replace("  - backend\n", "", 1)
    assert reconstructed == fixture, (
        "trackfw.yaml changed beyond the agents: block:\n"
        f"reconstructed:\n{reconstructed}\n"
        f"expected:\n{fixture}"
    )


def test_agents_install_both_falsification_directions(tmp_path):
    """
    AC8 — Two sub-cases in one test, one for each falsification direction:
    (a) by_agent project: installed agent appears in agents:
    (b) flat project:     agents: key is absent after install

    Reconciliation: explicitly exercises both branches of the by_agent guard
    and checks the opposite outcome for each, ensuring neither direction can
    produce a false green.
    """
    home = tmp_path / "home"
    home.mkdir()

    # (a) by_agent — installed agent must appear
    by_agent_dir = tmp_path / "by_agent_proj"
    by_agent_dir.mkdir()
    (by_agent_dir / "trackfw.yaml").write_text(_by_agent_yaml(), encoding="utf-8")
    r = cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "frontend",
        "--scope", "project",
        "--json",
        cwd=by_agent_dir,
        home=home,
    )
    assert r.returncode == 0, r.stderr
    content_a = (by_agent_dir / "trackfw.yaml").read_text(encoding="utf-8")
    assert "agents:" in content_a, "(a) agents: key must appear in by_agent project"
    assert content_a.count("- frontend") == 1, (
        f"(a) expected exactly one '- frontend', got:\n{content_a}"
    )

    # (b) flat — agents: key must be absent
    flat_dir = tmp_path / "flat_proj"
    flat_dir.mkdir()
    (flat_dir / "trackfw.yaml").write_text(_flat_yaml(), encoding="utf-8")
    r = cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "frontend",
        "--scope", "project",
        "--json",
        cwd=flat_dir,
        home=home,
    )
    assert r.returncode == 0, r.stderr
    content_b = (flat_dir / "trackfw.yaml").read_text(encoding="utf-8")
    assert "agents:" not in content_b, (
        f"(b) agents: key must NOT appear in flat project, got:\n{content_b}"
    )


def test_agents_install_global_scope_does_not_write_trackfw_yaml(tmp_path):
    """
    Contrato de paridade ML-2B/2C — instalação de escopo global não toca
    trackfw.yaml do projeto.

    Reconciliation: asserts the scope == 'project' guard fires correctly —
    a global install must leave trackfw.yaml byte-identical after the call.
    """
    (tmp_path / "trackfw.yaml").write_text(_by_agent_yaml(), encoding="utf-8")
    before = (tmp_path / "trackfw.yaml").read_bytes()

    result = cli(
        "agents", "install",
        "--targets", "claude",
        "--items", "backend",
        "--scope", "global",
        "--json",
        cwd=tmp_path,
    )
    assert result.returncode == 0, result.stderr

    after = (tmp_path / "trackfw.yaml").read_bytes()
    assert after == before, (
        "trackfw.yaml must not change for global-scope install:\n"
        f"{(tmp_path / 'trackfw.yaml').read_text(encoding='utf-8')}"
    )


def test_agents_install_appends_agents_block_at_end_of_file(tmp_path):
    """
    Contrato de paridade ML-2B/2C — quando `agents:` ainda não existe, o
    bloco é anexado ao FIM do documento, nunca inserido no meio.

    Reconciliation: asserts the end-of-file append rule: with `wip_limit`
    as the last key, the `agents:` block appears after it — not between
    `roadmap_namespacing` and `wip_limit`.
    """
    fixture = (
        "req_dir: docs/req\n"
        "roadmap_dir: docs/roadmaps\n"
        "roadmap_namespacing: by_agent\n"
        "wip_limit: 3\n"
    )
    (tmp_path / "trackfw.yaml").write_text(fixture, encoding="utf-8")
    home = tmp_path / "home"
    home.mkdir()

    env = dict(__import__("os").environ)
    env.pop("FORCE_COLOR", None)
    env["PYTHONPATH"] = str(__import__("pathlib").Path(__file__).parents[1])
    __import__("subprocess").run(
        [__import__("sys").executable, "-m", "trackfw",
         "agents", "install", "--targets", "claude", "--items", "architect",
         "--scope", "project", "--json"],
        cwd=tmp_path, env=env, capture_output=True, text=True, check=False,
    )

    content = (tmp_path / "trackfw.yaml").read_text(encoding="utf-8")
    lines = content.splitlines()
    wip_idx = next(i for i, l in enumerate(lines) if l.startswith("wip_limit:"))
    agents_idx = next(i for i, l in enumerate(lines) if l == "agents:")
    assert agents_idx > wip_idx, (
        f"agents: must appear AFTER wip_limit (end of file), "
        f"but wip_limit is at line {wip_idx} and agents: at line {agents_idx}:\n{content}"
    )


def test_agents_install_existing_agents_block_stays_in_place(tmp_path):
    """
    Contrato de paridade ML-2B/2C — quando `agents:` já existe no meio do
    arquivo, o novo item é adicionado DENTRO do bloco e o bloco NÃO muda
    de posição.

    Reconciliation: asserts that an already-present agents: block is extended
    in-place — no block relocation, no diff noise on keys surrounding it.
    """
    fixture = (
        "req_dir: docs/req\n"
        "agents:\n"
        "  - architect\n"
        "roadmap_dir: docs/roadmaps\n"
        "roadmap_namespacing: by_agent\n"
        "wip_limit: 3\n"
    )
    (tmp_path / "trackfw.yaml").write_text(fixture, encoding="utf-8")
    home = tmp_path / "home"
    home.mkdir()

    import os, subprocess, sys
    from pathlib import Path
    env = dict(os.environ)
    env.pop("FORCE_COLOR", None)
    env["PYTHONPATH"] = str(Path(__file__).parents[1])
    subprocess.run(
        [sys.executable, "-m", "trackfw",
         "agents", "install", "--targets", "claude", "--items", "backend",
         "--scope", "project", "--json"],
        cwd=tmp_path, env=env, capture_output=True, text=True, check=False,
    )

    content = (tmp_path / "trackfw.yaml").read_text(encoding="utf-8")
    lines = content.splitlines()

    # agents: must still be at line index 1 (second line)
    agents_idx = next(i for i, l in enumerate(lines) if l == "agents:")
    assert agents_idx == 1, (
        f"agents: block must NOT move — expected line 1, got line {agents_idx}:\n{content}"
    )

    # both entries must be present
    assert "  - architect" in lines, f"original entry missing:\n{content}"
    assert "  - backend" in lines, f"new entry missing:\n{content}"

    # wip_limit must still be after roadmap_dir (original trailing order preserved)
    wip_idx = next(i for i, l in enumerate(lines) if l.startswith("wip_limit:"))
    rd_idx = next(i for i, l in enumerate(lines) if l.startswith("roadmap_dir:"))
    assert wip_idx > rd_idx, (
        f"trailing key order must be preserved:\n{content}"
    )
