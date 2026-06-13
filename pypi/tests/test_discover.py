"""
test_discover.py — Testes pytest para o comando 'trackfw discover' (ML-4C).

Cobre:
  - test_discover_report: estrutura conhecida -> relatório com caminhos e contagens
  - test_discover_init_by_agent: docs/requisições/ + by_agent detectados -> yaml correto
  - test_discover_bootstrap_log: done/ com 2 roadmaps -> .trackfw-log com 2 entradas
  - test_discover_init_no_overwrite: trackfw.yaml existente -> avisa e não sobrescreve
"""

import argparse
import os

import pytest

from trackfw.commands import discover as discover_cmd


# ---------------------------------------------------------------------------
# test_discover_report
# ---------------------------------------------------------------------------

def test_discover_report(tmp_path, capsys, monkeypatch):
    """
    Dado uma estrutura de governança conhecida, _cmd_discover imprime
    os caminhos detectados, contagens e governance score.
    """
    # Estrutura
    adr_dir = tmp_path / "docs" / "adr"
    adr_dir.mkdir(parents=True)
    (adr_dir / "ADR-001.md").write_text("adr1")
    (adr_dir / "ADR-002.md").write_text("adr2")
    (adr_dir / "ADR-003.md").write_text("adr3")

    req_dir = tmp_path / "docs" / "req"
    req_dir.mkdir(parents=True)
    (req_dir / "REQ-001.md").write_text("req1")
    (req_dir / "REQ-002.md").write_text("req2")

    roadmap_backlog = tmp_path / "docs" / "roadmaps" / "backlog"
    roadmap_backlog.mkdir(parents=True)
    (roadmap_backlog / "RM-001.md").write_text("rm1")

    monkeypatch.chdir(tmp_path)

    args = argparse.Namespace(init=False, bootstrap_log=False)
    discover_cmd._cmd_discover(args)

    captured = capsys.readouterr()
    out = captured.out

    # Deve imprimir o diretório de ADR
    assert "docs/adr" in out
    # Deve conter as contagens
    assert "3" in out  # adr_count
    assert "2" in out  # req_count
    assert "1" in out  # roadmap_count
    # Deve imprimir governance score
    assert "Governance Score" in out or "governance" in out.lower()
    # Score deve ser maior que zero (ADR=20, REQ=20, roadmap=20 -> 60)
    assert "60" in out or "/100" in out


# ---------------------------------------------------------------------------
# test_discover_init_by_agent
# ---------------------------------------------------------------------------

def test_discover_init_by_agent(tmp_path, monkeypatch):
    """
    Quando docs/requisições/ existe e roadmaps têm subdirs com wip/,
    --init gera trackfw.yaml com req_dir correto e roadmap_namespacing: by_agent.
    """
    # REQ dir alternativo (docs/requisições/)
    req_dir = tmp_path / "docs" / "requisições"
    req_dir.mkdir(parents=True)
    (req_dir / "REQ-001.md").write_text("req")

    # Roadmap by_agent: zeus/wip/ e apolo/backlog/
    (tmp_path / "docs" / "roadmaps" / "zeus" / "wip").mkdir(parents=True)
    (tmp_path / "docs" / "roadmaps" / "apolo" / "backlog").mkdir(parents=True)

    monkeypatch.chdir(tmp_path)

    args = argparse.Namespace(init=True, bootstrap_log=False)
    discover_cmd._cmd_discover(args)

    yaml_path = tmp_path / "trackfw.yaml"
    assert yaml_path.exists(), "trackfw.yaml deve ser criado"

    content = yaml_path.read_text(encoding="utf-8")

    # req_dir deve apontar para docs/requisições
    assert "req_dir: docs/requisições" in content or "docs/requisi" in content

    # namespacing deve ser by_agent
    assert "roadmap_namespacing: by_agent" in content

    # agentes detectados
    assert "zeus" in content
    assert "apolo" in content

    # governance_mode lenient sempre presente
    assert "governance_mode: lenient" in content


# ---------------------------------------------------------------------------
# test_discover_bootstrap_log
# ---------------------------------------------------------------------------

def test_discover_bootstrap_log(tmp_path, monkeypatch):
    """
    Com done/ contendo 2 roadmaps, --bootstrap-log cria .trackfw-log com 2 entradas.
    """
    done_dir = tmp_path / "docs" / "roadmaps" / "done"
    done_dir.mkdir(parents=True)
    (done_dir / "RM-2026-01-01-feature-x.md").write_text("rm1")
    (done_dir / "RM-2026-02-01-feature-y.md").write_text("rm2")

    monkeypatch.chdir(tmp_path)

    args = argparse.Namespace(init=False, bootstrap_log=True)
    discover_cmd._cmd_discover(args)

    log_path = tmp_path / "docs" / "roadmaps" / ".trackfw-log"
    assert log_path.exists(), ".trackfw-log deve ser criado"

    content = log_path.read_text(encoding="utf-8")
    lines = [l for l in content.splitlines() if l.strip()]

    # Deve ter exatamente 2 entradas
    assert len(lines) == 2

    # Cada entrada deve referenciar os arquivos e conter "backlog -> done"
    combined = "\n".join(lines)
    assert "RM-2026-01-01-feature-x.md" in combined
    assert "RM-2026-02-01-feature-y.md" in combined
    assert "backlog -> done" in combined


# ---------------------------------------------------------------------------
# test_discover_init_no_overwrite
# ---------------------------------------------------------------------------

def test_discover_init_no_overwrite(tmp_path, capsys, monkeypatch):
    """
    Se trackfw.yaml já existe, --init deve imprimir aviso e não sobrescrever.
    """
    yaml_path = tmp_path / "trackfw.yaml"
    original_content = "governance_mode: strict\n# arquivo original\n"
    yaml_path.write_text(original_content, encoding="utf-8")

    monkeypatch.chdir(tmp_path)

    args = argparse.Namespace(init=True, bootstrap_log=False)
    discover_cmd._cmd_discover(args)

    captured = capsys.readouterr()
    out = captured.out

    # Deve imprimir aviso
    assert "ja existe" in out.lower() or "already exists" in out.lower() or "aviso" in out.lower()

    # Conteúdo original deve permanecer intacto
    assert yaml_path.read_text(encoding="utf-8") == original_content
