"""
test_traceid.py — Testes para verificação bidirecional de req_id (ML-5C — v2.5).
Usa pytest com fixture tmp_path para isolamento completo.
"""

import os
import sys
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config as _config
from trackfw.traceid import check_traceid


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _write(path: str, content: str = "") -> None:
    """Cria arquivo, criando diretórios intermediários se necessário."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _make_cfg(tmp_path, trace_id_field: str = "req_id") -> dict:
    """Retorna dict de config mínimo apontando para tmp_path."""
    cfg = _config.defaults()
    cfg["req_dir"] = str(tmp_path / "docs/req")
    cfg["roadmap_dir"] = str(tmp_path / "docs/roadmaps")
    cfg["trace_id_field"] = trace_id_field
    return cfg


def _req_content(req_id: str, status: str = "open") -> str:
    return f"---\nreq_id: {req_id}\nstatus: {status}\n---\n\n# REQ\n"


def _roadmap_content(req_id: str) -> str:
    return f"---\nreq_id: {req_id}\nstatus: wip\n---\n\n# Roadmap\n"


# ---------------------------------------------------------------------------
# Testes
# ---------------------------------------------------------------------------

def test_traceid_orphan_roadmap(tmp_path):
    """Roadmap com req_id sem REQ correspondente → violation traceid_orphan_roadmap."""
    cfg = _make_cfg(tmp_path)

    # Cria roadmap em wip com req_id
    _write(str(tmp_path / "docs/roadmaps/wip/RM-001.md"), _roadmap_content("REQ-001"))
    # Não cria REQ correspondente

    violations = check_traceid(cfg)
    rules = [v["rule"] for v in violations]
    assert "traceid_orphan_roadmap" in rules, f"Esperado traceid_orphan_roadmap, obteve: {violations}"


def test_traceid_orphan_req(tmp_path):
    """REQ com req_id sem Roadmap correspondente → violation traceid_orphan_req."""
    cfg = _make_cfg(tmp_path)

    # Cria REQ com req_id
    _write(str(tmp_path / "docs/req/REQ-002.md"), _req_content("REQ-002"))
    # Não cria Roadmap correspondente

    violations = check_traceid(cfg)
    rules = [v["rule"] for v in violations]
    assert "traceid_orphan_req" in rules, f"Esperado traceid_orphan_req, obteve: {violations}"


def test_traceid_state_mismatch(tmp_path):
    """REQ em done/ e Roadmap em wip/ com mesmo req_id → violation traceid_state_mismatch."""
    cfg = _make_cfg(tmp_path)

    # REQ com status done
    _write(str(tmp_path / "docs/req/REQ-003.md"), _req_content("REQ-003", status="done"))
    # Roadmap em wip (estado diferente de done)
    _write(str(tmp_path / "docs/roadmaps/wip/RM-003.md"), _roadmap_content("REQ-003"))

    violations = check_traceid(cfg)
    rules = [v["rule"] for v in violations]
    assert "traceid_state_mismatch" in rules, f"Esperado traceid_state_mismatch, obteve: {violations}"


def test_traceid_duplicate_req(tmp_path):
    """2 REQs com mesmo req_id → violation traceid_duplicate_req."""
    cfg = _make_cfg(tmp_path)

    _write(str(tmp_path / "docs/req/REQ-004a.md"), _req_content("REQ-DUP"))
    _write(str(tmp_path / "docs/req/REQ-004b.md"), _req_content("REQ-DUP"))
    # Roadmap correspondente para evitar orphan mascarar o teste
    _write(str(tmp_path / "docs/roadmaps/wip/RM-004.md"), _roadmap_content("REQ-DUP"))

    violations = check_traceid(cfg)
    rules = [v["rule"] for v in violations]
    assert "traceid_duplicate_req" in rules, f"Esperado traceid_duplicate_req, obteve: {violations}"


def test_traceid_valid_pair(tmp_path):
    """Par válido REQ + Roadmap no mesmo estado → sem violations traceid."""
    cfg = _make_cfg(tmp_path)

    # REQ com status wip
    _write(str(tmp_path / "docs/req/REQ-005.md"), _req_content("REQ-005", status="wip"))
    # Roadmap em wip (estado consistente)
    _write(str(tmp_path / "docs/roadmaps/wip/RM-005.md"), _roadmap_content("REQ-005"))

    violations = check_traceid(cfg)
    traceid_rules = [v["rule"] for v in violations if v["rule"].startswith("traceid_")]
    assert traceid_rules == [], f"Par valido nao deveria gerar violations traceid, obteve: {violations}"


def test_traceid_normaliza_aspas_duplas_em_ids_e_status(tmp_path):
    """Valores de frontmatter com aspas duplas devem casar com valores sem aspas."""
    cfg = _make_cfg(tmp_path)

    _write(
        str(tmp_path / "docs/req/REQ-008.md"),
        '---\nreq_id: "REQ-008"\nstatus: "wip"\n---\n\n# REQ\n',
    )
    _write(str(tmp_path / "docs/roadmaps/wip/RM-008.md"), _roadmap_content("REQ-008"))

    violations = check_traceid(cfg)
    traceid_rules = [v["rule"] for v in violations if v["rule"].startswith("traceid_")]
    assert traceid_rules == [], f"Aspas duplas nao deveriam gerar violations traceid, obteve: {violations}"


def test_traceid_normaliza_aspas_simples_em_ids_e_status(tmp_path):
    """Valores de frontmatter com aspas simples devem casar com valores sem aspas."""
    cfg = _make_cfg(tmp_path)

    _write(
        str(tmp_path / "docs/req/REQ-009.md"),
        "---\nreq_id: 'REQ-009'\nstatus: 'wip'\n---\n\n# REQ\n",
    )
    _write(str(tmp_path / "docs/roadmaps/wip/RM-009.md"), _roadmap_content("REQ-009"))

    violations = check_traceid(cfg)
    traceid_rules = [v["rule"] for v in violations if v["rule"].startswith("traceid_")]
    assert traceid_rules == [], f"Aspas simples nao deveriam gerar violations traceid, obteve: {violations}"


def test_traceid_preserva_conteudo_interno_do_id(tmp_path):
    """A normalização remove só aspas externas e preserva aspas internas no trace ID."""
    cfg = _make_cfg(tmp_path)

    _write(
        str(tmp_path / "docs/req/REQ-010.md"),
        "---\nreq_id: \"REQ-'010'\"\nstatus: wip\n---\n\n# REQ\n",
    )
    _write(
        str(tmp_path / "docs/roadmaps/wip/RM-010.md"),
        "---\nreq_id: REQ-'010'\nstatus: wip\n---\n\n# Roadmap\n",
    )

    violations = check_traceid(cfg)
    traceid_rules = [v["rule"] for v in violations if v["rule"].startswith("traceid_")]
    assert traceid_rules == [], f"Conteudo interno deveria ser preservado e casar, obteve: {violations}"


def test_traceid_ignora_id_vazio_aspeado(tmp_path):
    """req_id: "" normaliza para vazio e não deve ser indexado como ID real."""
    cfg = _make_cfg(tmp_path)

    _write(str(tmp_path / "docs/req/REQ-011.md"), '---\nreq_id: ""\nstatus: wip\n---\n\n# REQ\n')
    _write(str(tmp_path / "docs/roadmaps/wip/RM-011.md"), '---\nreq_id: ""\nstatus: wip\n---\n\n# Roadmap\n')

    violations = check_traceid(cfg)
    assert any(
        v["rule"] == "traceid_config_warning"
        and "no REQ/Roadmap entries were indexed" in v.get("message", "")
        for v in violations
    ), f"IDs vazios aspeados nao deveriam ser indexados, obteve: {violations}"


def test_traceid_disabled(tmp_path):
    """Sem trace_id_field configurado → nenhuma verificação traceid é feita."""
    cfg = _make_cfg(tmp_path, trace_id_field="")

    # Mesmo com arquivos com req_id, não deve gerar violations
    _write(str(tmp_path / "docs/roadmaps/wip/RM-006.md"), _roadmap_content("REQ-006"))

    violations = check_traceid(cfg)
    assert violations == [], f"Sem trace_id_field, violations deve ser vazio, obteve: {violations}"


def test_traceid_by_agent(tmp_path):
    """Layout by_agent: req_dir/<agente>/<estado>/ e roadmap_dir/<agente>/<estado>/ — orphans devem ser detectados."""
    cfg = _make_cfg(tmp_path)
    cfg["roadmap_namespacing"] = "by_agent"

    # REQ em by_agent com req_id sem roadmap correspondente
    _write(str(tmp_path / "docs/req/claude/wip/REQ-007.md"), _req_content("orphan-req-007"))
    # Roadmap em by_agent sem REQ correspondente
    _write(
        str(tmp_path / "docs/roadmaps/claude/wip/RM-007.md"),
        _roadmap_content("orphan-roadmap-007"),
    )

    violations = check_traceid(cfg)
    rules = [v["rule"] for v in violations]
    assert "traceid_orphan_req" in rules, f"Esperado traceid_orphan_req em {rules}"
    assert "traceid_orphan_roadmap" in rules, f"Esperado traceid_orphan_roadmap em {rules}"


def test_traceid_zero_entries_warning(tmp_path):
    """Diretórios vazios com trace_id_field → violation traceid_config_warning."""
    cfg = _make_cfg(tmp_path)
    # Cria os diretórios mas sem arquivos .md
    os.makedirs(str(tmp_path / "docs/req"), exist_ok=True)
    os.makedirs(str(tmp_path / "docs/roadmaps"), exist_ok=True)

    result = check_traceid(cfg)
    assert any(
        "no REQ/Roadmap entries were indexed" in v.get("message", "")
        for v in result
    ), f"Esperado traceid_config_warning com mensagem de zero entradas, obteve: {result}"
