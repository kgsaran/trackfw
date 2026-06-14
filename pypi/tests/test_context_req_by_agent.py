"""
test_context_req_by_agent.py — Testes para `trackfw context` com REQs em by_agent (ML-1C — v2.5.4).
"""

import os
import sys
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.commands.context import _get_context


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _write(path: str, content: str) -> None:
    """Cria arquivo e diretórios intermediários."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


# ---------------------------------------------------------------------------
# Testes
# ---------------------------------------------------------------------------

def test_context_req_by_agent(tmp_path, capsys, monkeypatch):
    """
    Modo by_agent: REQ em req_dir/<agent>/<state>/req.md deve ser contada.
    Verifica que a saída exibe 'REQs (1)'.
    """
    req_dir = tmp_path / "req"
    rm_dir = tmp_path / "rm"

    # Cria REQ em subdiretório by_agent
    _write(str(req_dir / "claude" / "wip" / "req.md"),
           "---\nstatus: Open\n---\n# REQ\n")

    # Cria Roadmap dummy (necessário para evitar erros de OSError no scan de roadmaps)
    _write(str(rm_dir / "claude" / "wip" / "rm.md"),
           "---\nstatus: wip\n---\n# Roadmap\n")

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_dir": str(rm_dir),
        "roadmap_namespacing": "by_agent",
        "adr_dirs": [],
    }

    # Monkeypatcha config.load() para retornar cfg de teste
    import trackfw.config as config_mod
    monkeypatch.setattr(config_mod, "load", lambda: cfg)

    # Monkeypatcha validate() para retornar resultado vazio (evita acesso a disco real)
    import trackfw.validator as validator_mod
    monkeypatch.setattr(validator_mod, "validate", lambda: {"violations": [], "warnings": []})

    _get_context("md")

    captured = capsys.readouterr()
    assert "REQs (1)" in captured.out, (
        f"Esperado 'REQs (1)' na saída, obteve:\n{captured.out}"
    )


def test_context_req_flat_no_regression(tmp_path, capsys, monkeypatch):
    """
    Modo flat (sem roadmap_namespacing): REQ diretamente em req_dir deve ser contada.
    Verifica que a saída exibe 'REQs (1)' — sem regressão.
    """
    req_dir = tmp_path / "req"
    rm_dir = tmp_path / "rm"

    # Cria REQ em modo flat
    _write(str(req_dir / "req.md"),
           "---\nstatus: Open\n---\n# REQ\n")

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_dir": str(rm_dir),
        "adr_dirs": [],
    }

    import trackfw.config as config_mod
    monkeypatch.setattr(config_mod, "load", lambda: cfg)

    import trackfw.validator as validator_mod
    monkeypatch.setattr(validator_mod, "validate", lambda: {"violations": [], "warnings": []})

    _get_context("md")

    captured = capsys.readouterr()
    assert "REQs (1)" in captured.out, (
        f"Esperado 'REQs (1)' na saída (modo flat), obteve:\n{captured.out}"
    )
