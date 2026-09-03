"""
test_req_layout_union.py — contrato do resolvedor de REQ (ADR-2026-09-03):
leitura é UNIÃO dos 4 layouts, escrita é ÚNICA, e o diretório de escrita está contido na união por
construção (D2/D3/D4). Espelha internal/validator/validator_req_layout_test.go e
npm/tests/req_layout_union.test.js.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.validator import req_write_dir, resolve_req_files


def _write_req(path: str) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write("---\nstatus: Open\ndate: 2026-09-03\n---\n\n# REQ: fixture\n")


def test_uniao_dos_4_layouts(tmp_path):
    """Em by_agent os 4 layouts são lidos JUNTOS, nunca em exclusão mútua."""
    req_dir = str(tmp_path / "docs" / "req")
    _write_req(os.path.join(req_dir, "REQ-flat.md"))                      # (1) flat legado
    _write_req(os.path.join(req_dir, "backlog", "REQ-estado.md"))         # (2) por-estado legado
    _write_req(os.path.join(req_dir, "claude", "REQ-canonico.md"))        # (3) CANÔNICO
    _write_req(os.path.join(req_dir, "claude", "wip", "REQ-legado.md"))   # (4) legado

    cfg = {"req_dir": req_dir, "roadmap_namespacing": "by_agent", "agents": ["claude"]}
    got = sorted(os.path.basename(p) for p in resolve_req_files(cfg))
    assert got == ["REQ-canonico.md", "REQ-estado.md", "REQ-flat.md", "REQ-legado.md"]


def test_dedup_estado_e_agente(tmp_path):
    """<estado>/ e <agente>/ colidem (agents: união disco) — a REQ é contada UMA vez."""
    req_dir = str(tmp_path / "docs" / "req")
    _write_req(os.path.join(req_dir, "backlog", "REQ-uma-so.md"))

    cfg = {"req_dir": req_dir, "roadmap_namespacing": "by_agent", "agents": ["claude"]}
    got = resolve_req_files(cfg)
    assert len(got) == 1, got


def test_write_dir_esta_contido_na_uniao(tmp_path):
    """D4: REQ criada onde req_write_dir manda é encontrada por resolve_req_files, nos dois modos."""
    for namespacing, suffix in (("", ("docs", "req")), ("by_agent", ("docs", "req", "claude"))):
        req_dir = str(tmp_path / (namespacing or "flat") / "docs" / "req")
        cfg = {"req_dir": req_dir, "roadmap_namespacing": namespacing, "agents": ["claude"]}

        write_dir = req_write_dir(cfg)
        assert write_dir == os.path.join(str(tmp_path / (namespacing or "flat")), *suffix)

        _write_req(os.path.join(write_dir, "REQ-nova.md"))
        assert any(os.path.basename(p) == "REQ-nova.md" for p in resolve_req_files(cfg)), (
            f"REQ criada em {write_dir} não foi encontrada pelo resolvedor"
        )


def test_write_dir_default_esta_contido_na_uniao(tmp_path):
    """by_agent SEM agents: declarada — a REQ em default/ tem de voltar pelo DISCO, não por config."""
    cfg = {"req_dir": str(tmp_path / "docs" / "req"), "roadmap_namespacing": "by_agent"}
    write_dir = req_write_dir(cfg)
    _write_req(os.path.join(write_dir, "REQ-nova.md"))

    got = [os.path.basename(p) for p in resolve_req_files(cfg)]
    assert got == ["REQ-nova.md"], f"REQ criada em {write_dir} não foi encontrada: {got}"


def test_write_dir_sem_agents_usa_default():
    cfg = {"req_dir": "docs/req", "roadmap_namespacing": "by_agent"}
    assert req_write_dir(cfg) == os.path.join("docs/req", "default")
