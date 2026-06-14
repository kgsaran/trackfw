"""
test_req_by_agent.py — Testes para REQ indexing by_agent (ML-1C — v2.5.3).
Cobre resolve_req_files, _index_reqs_by_agent e salvaguarda one-sided.
"""

import os
import sys
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.traceid import check_traceid
from trackfw.validator import resolve_req_files


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _write(path: str, content: str = "") -> None:
    """Cria arquivo, criando diretórios intermediários se necessário."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _frontmatter(field: str, value: str) -> str:
    return f"---\n{field}: {value}\n---\n# Título\n"


# ---------------------------------------------------------------------------
# Testes de resolve_req_files
# ---------------------------------------------------------------------------

def test_resolve_req_files_flat(tmp_path):
    """Modo flat: retorna .md diretamente em req_dir."""
    req_dir = tmp_path / "req"
    req_dir.mkdir()
    (req_dir / "file.md").write_text("# REQ\n")

    files = resolve_req_files({"req_dir": str(req_dir)})
    assert str(req_dir / "file.md") in files, f"Esperado path em resultado, obteve: {files}"


def test_resolve_req_files_by_agent(tmp_path):
    """Modo by_agent: percorre req_dir/<agente>/<estado>/ e retorna path completo."""
    req_dir = tmp_path / "req"
    target = req_dir / "claude" / "wip" / "file.md"
    os.makedirs(str(target.parent), exist_ok=True)
    target.write_text("# REQ\n")

    files = resolve_req_files({
        "req_dir": str(req_dir),
        "roadmap_namespacing": "by_agent",
    })
    assert str(target) in files, f"Esperado path completo em resultado, obteve: {files}"


# ---------------------------------------------------------------------------
# Testes de check_traceid com by_agent para REQs
# ---------------------------------------------------------------------------

def test_traceid_req_by_agent_no_orphan(tmp_path):
    """Par REQ + Roadmap em by_agent com mesmo req_id → nenhuma violation traceid_orphan_roadmap."""
    req_dir = tmp_path / "req"
    rm_dir = tmp_path / "rm"

    _write(str(req_dir / "claude" / "wip" / "req.md"), _frontmatter("req_id", "RID-1"))
    _write(str(rm_dir / "claude" / "wip" / "rm.md"), _frontmatter("req_id", "RID-1"))

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_dir": str(rm_dir),
        "trace_id_field": "req_id",
        "roadmap_namespacing": "by_agent",
    }

    violations = check_traceid(cfg)
    orphan_rules = [v["rule"] for v in violations if v["rule"] == "traceid_orphan_roadmap"]
    assert orphan_rules == [], f"Nao deveria haver traceid_orphan_roadmap, obteve: {violations}"


def test_salvaguarda_one_sided(tmp_path):
    """Roadmap indexado mas REQ dir vazio → violation traceid_config_warning com 'REQs (0)'."""
    rm_dir = tmp_path / "rm"
    req_dir = tmp_path / "req_empty"

    _write(str(rm_dir / "claude" / "wip" / "rm.md"), _frontmatter("req_id", "RID-1"))

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_dir": str(rm_dir),
        "trace_id_field": "req_id",
        "roadmap_namespacing": "by_agent",
    }

    violations = check_traceid(cfg)
    warnings = [v for v in violations if v.get("rule") == "traceid_config_warning"]
    assert warnings, f"Esperado traceid_config_warning, obteve: {violations}"
    assert any("REQs (0)" in v.get("message", "") for v in warnings), (
        f"Esperado 'REQs (0)' na mensagem, obteve: {[v.get('message') for v in warnings]}"
    )
