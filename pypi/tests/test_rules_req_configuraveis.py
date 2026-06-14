"""
test_rules_req_configuraveis.py — Testes de configurabilidade via _apply_rule para
req_has_adr, blocked_has_req e req_has_roadmap (ML-1C — v2.6.0).

9 testes: 3 regras × 3 cenários (warning / off / default-error).
"""

import os
import sys
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import trackfw.config as _config_mod
from trackfw.validator import validate_unfiltered


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _write(path: str, content: str = "") -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _req_sem_adr(tmp_path):
    """Cria REQ sem campo `adr:` no frontmatter."""
    req_dir = tmp_path / "req"
    roadmap_dir = tmp_path / "rm"
    req_dir.mkdir()
    roadmap_dir.mkdir()
    _write(str(req_dir / "REQ-001.md"), "---\nreq_id: REQ-001\n---\n# Req sem ADR\n")
    return str(req_dir), str(roadmap_dir)


def _blocked_sem_req(tmp_path):
    """Cria roadmap em blocked/ sem campo `req_id:` no frontmatter."""
    roadmap_dir = tmp_path / "rm"
    req_dir = tmp_path / "req"
    req_dir.mkdir()
    blocked_dir = roadmap_dir / "blocked"
    blocked_dir.mkdir(parents=True)
    _write(str(blocked_dir / "RM-blocked.md"), "---\nstatus: blocked\n---\n# Roadmap bloqueado\n")
    return str(req_dir), str(roadmap_dir)


def _req_sem_roadmap(tmp_path):
    """Cria REQ sem campo `roadmap:` no frontmatter."""
    req_dir = tmp_path / "req"
    roadmap_dir = tmp_path / "rm"
    req_dir.mkdir()
    roadmap_dir.mkdir()
    _write(str(req_dir / "REQ-002.md"), "---\nreq_id: REQ-002\n---\n# Req sem roadmap\n")
    return str(req_dir), str(roadmap_dir)


def _base_cfg(req_dir: str, roadmap_dir: str, rules: dict = None) -> dict:
    """Monta dict de config mínimo com regras opcionais."""
    cfg = _config_mod.defaults()
    cfg["req_dir"] = req_dir
    cfg["roadmap_dir"] = roadmap_dir
    if rules:
        cfg["rules"].update(rules)
    return cfg


def _run_with_cfg(monkeypatch, cfg: dict) -> dict:
    """Executa validate_unfiltered com cfg injetado via monkeypatch."""
    _config_mod.reset()

    def _patched_load(cwd=None):
        return cfg

    monkeypatch.setattr(_config_mod, "load", _patched_load)
    return validate_unfiltered()


# ---------------------------------------------------------------------------
# req_has_adr
# ---------------------------------------------------------------------------

def test_req_has_adr_warning(tmp_path, monkeypatch):
    """Severidade 'warning': violação de req_has_adr vai para warnings, não violations."""
    req_dir, roadmap_dir = _req_sem_adr(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir, rules={"req_has_adr": "warning"})
    result = _run_with_cfg(monkeypatch, cfg)

    w_rules = [w["rule"] for w in result["warnings"] if w.get("rule") == "req_has_adr"]
    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "req_has_adr"]

    assert w_rules, f"Esperado req_has_adr em warnings, obteve warnings={result['warnings']}"
    assert not v_rules, f"Nao esperado req_has_adr em violations, obteve={result['violations']}"


def test_req_has_adr_off(tmp_path, monkeypatch):
    """Severidade 'off': violação de req_has_adr silenciada."""
    req_dir, roadmap_dir = _req_sem_adr(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir, rules={"req_has_adr": "off"})
    result = _run_with_cfg(monkeypatch, cfg)

    w_rules = [w["rule"] for w in result["warnings"] if w.get("rule") == "req_has_adr"]
    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "req_has_adr"]

    assert not w_rules, f"Esperado silencio, obteve warnings={w_rules}"
    assert not v_rules, f"Esperado silencio, obteve violations={v_rules}"


def test_req_has_adr_default_error(tmp_path, monkeypatch):
    """Sem override de severidade (default 'error'): violação vai para violations."""
    req_dir, roadmap_dir = _req_sem_adr(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir)
    result = _run_with_cfg(monkeypatch, cfg)

    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "req_has_adr"]
    assert v_rules, f"Esperado req_has_adr em violations, obteve={result['violations']}"


# ---------------------------------------------------------------------------
# blocked_has_req
# ---------------------------------------------------------------------------

def test_blocked_has_req_warning(tmp_path, monkeypatch):
    """Severidade 'warning': violação de blocked_has_req vai para warnings, não violations."""
    req_dir, roadmap_dir = _blocked_sem_req(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir, rules={"blocked_has_req": "warning"})
    result = _run_with_cfg(monkeypatch, cfg)

    w_rules = [w["rule"] for w in result["warnings"] if w.get("rule") == "blocked_has_req"]
    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "blocked_has_req"]

    assert w_rules, f"Esperado blocked_has_req em warnings, obteve warnings={result['warnings']}"
    assert not v_rules, f"Nao esperado blocked_has_req em violations, obteve={result['violations']}"


def test_blocked_has_req_off(tmp_path, monkeypatch):
    """Severidade 'off': violação de blocked_has_req silenciada."""
    req_dir, roadmap_dir = _blocked_sem_req(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir, rules={"blocked_has_req": "off"})
    result = _run_with_cfg(monkeypatch, cfg)

    w_rules = [w["rule"] for w in result["warnings"] if w.get("rule") == "blocked_has_req"]
    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "blocked_has_req"]

    assert not w_rules, f"Esperado silencio, obteve warnings={w_rules}"
    assert not v_rules, f"Esperado silencio, obteve violations={v_rules}"


def test_blocked_has_req_default_error(tmp_path, monkeypatch):
    """Sem override de severidade (default 'error'): violação vai para violations."""
    req_dir, roadmap_dir = _blocked_sem_req(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir)
    result = _run_with_cfg(monkeypatch, cfg)

    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "blocked_has_req"]
    assert v_rules, f"Esperado blocked_has_req em violations, obteve={result['violations']}"


# ---------------------------------------------------------------------------
# req_has_roadmap
# ---------------------------------------------------------------------------

def test_req_has_roadmap_warning(tmp_path, monkeypatch):
    """Severidade 'warning': violação de req_has_roadmap vai para warnings, não violations."""
    req_dir, roadmap_dir = _req_sem_roadmap(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir, rules={"req_has_roadmap": "warning"})
    result = _run_with_cfg(monkeypatch, cfg)

    w_rules = [w["rule"] for w in result["warnings"] if w.get("rule") == "req_has_roadmap"]
    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "req_has_roadmap"]

    assert w_rules, f"Esperado req_has_roadmap em warnings, obteve warnings={result['warnings']}"
    assert not v_rules, f"Nao esperado req_has_roadmap em violations, obteve={result['violations']}"


def test_req_has_roadmap_off(tmp_path, monkeypatch):
    """Severidade 'off': violação de req_has_roadmap silenciada."""
    req_dir, roadmap_dir = _req_sem_roadmap(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir, rules={"req_has_roadmap": "off"})
    result = _run_with_cfg(monkeypatch, cfg)

    w_rules = [w["rule"] for w in result["warnings"] if w.get("rule") == "req_has_roadmap"]
    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "req_has_roadmap"]

    assert not w_rules, f"Esperado silencio, obteve warnings={w_rules}"
    assert not v_rules, f"Esperado silencio, obteve violations={v_rules}"


def test_req_has_roadmap_default_error(tmp_path, monkeypatch):
    """Sem override de severidade (default 'error'): violação vai para violations."""
    req_dir, roadmap_dir = _req_sem_roadmap(tmp_path)
    cfg = _base_cfg(req_dir, roadmap_dir)
    result = _run_with_cfg(monkeypatch, cfg)

    v_rules = [v["rule"] for v in result["violations"] if v.get("rule") == "req_has_roadmap"]
    assert v_rules, f"Esperado req_has_roadmap em violations, obteve={result['violations']}"
