"""
test_req_list_move_subfolders.py — ML-1C (Python): req list/move recursivos + move físico
condicional, espelhando os testes ML-1A (Go) e ML-1B (Node) do mesmo roadmap.

Cobre:
  - list_reqs / list_req_files: layout por-estado e by_agent
  - find_req: descoberta recursiva nos 3 layouts
  - move_req: move físico em layout por-estado e by_agent
  - _append_req_transition_log: linha registrada em <req_dir>/.trackfw-log
  - CLI: `trackfw req list` nos layouts por-estado e by_agent
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config as _config
from trackfw.generators.req import find_req, list_req_files, list_reqs, move_req


def _write(path: str, content: str = "") -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _req_body(status: str = "Open") -> str:
    return (
        f"---\nstatus: {status}\ndate: 2026-08-04\n---\n\n"
        f"# REQ: Título\n\n> Date: 2026-08-04 | Status: {status}\n"
    )


# ---------------------------------------------------------------------------
# list_reqs / list_req_files
# ---------------------------------------------------------------------------

def test_list_reqs_by_state(tmp_path, capsys):
    """Layout por-estado: req_dir/<estado>/*.md é listado por list_reqs."""
    req_dir = tmp_path / "req"
    _write(str(req_dir / "backlog" / "REQ-1.md"), _req_body())

    cfg = {"req_dir": str(req_dir)}
    list_reqs(cfg)

    out = capsys.readouterr().out
    assert "REQ-1.md" in out
    assert "Open" in out


def test_list_reqs_by_agent(tmp_path, capsys):
    """Layout by_agent: req_dir/<agente>/<estado>/*.md é listado por list_reqs."""
    req_dir = tmp_path / "req"
    _write(str(req_dir / "claude" / "wip" / "REQ-2.md"), _req_body("wip"))

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_namespacing": "by_agent",
        "agents": ["claude"],
    }
    list_reqs(cfg)

    out = capsys.readouterr().out
    assert "REQ-2.md" in out
    assert "wip" in out


def test_list_reqs_empty_dir_message(tmp_path, capsys):
    req_dir = tmp_path / "req"
    os.makedirs(str(req_dir))
    cfg = {"req_dir": str(req_dir)}

    list_reqs(cfg)

    out = capsys.readouterr().out
    assert f"No REQs found in {req_dir}" in out


# ---------------------------------------------------------------------------
# find_req — descoberta recursiva
# ---------------------------------------------------------------------------

def test_find_req_recurses_subfolders(tmp_path):
    req_dir = tmp_path / "req"
    _write(str(req_dir / "analyzing" / "REQ-nested.md"), _req_body())

    cfg = {"req_dir": str(req_dir)}
    found = find_req("nested", cfg)

    assert found == str(req_dir / "analyzing" / "REQ-nested.md")


def test_list_req_files_concatenates_all_layouts(tmp_path):
    """flat + por-estado + by_agent não são mutuamente exclusivos."""
    req_dir = tmp_path / "req"
    _write(str(req_dir / "REQ-flat.md"), _req_body())
    _write(str(req_dir / "wip" / "REQ-state.md"), _req_body("wip"))
    _write(str(req_dir / "claude" / "done" / "REQ-agent.md"), _req_body("done"))

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_namespacing": "by_agent",
        "agents": ["claude"],
    }
    files = list_req_files(cfg)
    basenames = {os.path.basename(f) for f in files}

    assert basenames == {"REQ-flat.md", "REQ-state.md", "REQ-agent.md"}


# ---------------------------------------------------------------------------
# move_req — move físico condicional
# ---------------------------------------------------------------------------

def test_move_req_physically_moves_in_state_layout(tmp_path):
    req_dir = tmp_path / "req"
    src = req_dir / "backlog" / "REQ-mover.md"
    _write(str(src), _req_body("backlog"))

    cfg = {"req_dir": str(req_dir)}
    new_path = move_req("mover", "done", cfg=cfg)

    assert new_path == str(req_dir / "done" / "REQ-mover.md")
    assert not os.path.exists(str(src))
    with open(new_path, encoding="utf-8") as f:
        content = f.read()
    assert "status: done" in content


def test_move_req_physically_moves_in_by_agent_layout(tmp_path):
    req_dir = tmp_path / "req"
    src = req_dir / "claude" / "wip" / "REQ-agente.md"
    _write(str(src), _req_body("wip"))

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_namespacing": "by_agent",
        "agents": ["claude"],
    }
    new_path = move_req("agente", "done", cfg=cfg)

    assert new_path == str(req_dir / "claude" / "done" / "REQ-agente.md")
    assert not os.path.exists(str(src))
    with open(new_path, encoding="utf-8") as f:
        content = f.read()
    assert "status: done" in content


def test_move_req_rejects_invalid_state_in_by_agent_layout(tmp_path):
    """AC5 — o defeito mais grave era aqui: _req_agent_state_dir retornava None para status
    inválido e o código caía silenciosamente no fallback in-place, sem avisar o usuário."""
    req_dir = tmp_path / "req"
    src = req_dir / "claude" / "wip" / "REQ-invalido-agente.md"
    _write(str(src), _req_body("wip"))

    cfg = {
        "req_dir": str(req_dir),
        "roadmap_namespacing": "by_agent",
        "agents": ["claude"],
    }

    try:
        move_req("invalido-agente", "status-invalido-xyz", cfg=cfg)
        assert False, "move_req deveria lançar RuntimeError para status inválido"
    except RuntimeError as exc:
        assert "invalid state" in str(exc)

    assert os.path.exists(str(src))
    assert not os.path.exists(str(req_dir / "claude" / "status-invalido-xyz"))


def test_move_req_rejects_invalid_state_in_state_layout(tmp_path):
    """AC5 — paridade com Go: status inválido em layout por-estado deve lançar erro,
    sem criar pasta arbitrária e sem mover o arquivo."""
    req_dir = tmp_path / "req"
    src = req_dir / "wip" / "REQ-invalido.md"
    _write(str(src), _req_body("wip"))

    cfg = {"req_dir": str(req_dir)}

    try:
        move_req("invalido", "status-invalido-xyz", cfg=cfg)
        assert False, "move_req deveria lançar RuntimeError para status inválido"
    except RuntimeError as exc:
        assert "invalid state" in str(exc)

    assert os.path.exists(str(src))
    assert not os.path.exists(str(req_dir / "status-invalido-xyz"))


def test_move_req_logs_transition(tmp_path):
    req_dir = tmp_path / "req"
    src = req_dir / "backlog" / "REQ-logado.md"
    _write(str(src), _req_body("backlog"))

    cfg = {"req_dir": str(req_dir)}
    move_req("logado", "done", cfg=cfg)

    log_path = req_dir / ".trackfw-log"
    assert log_path.exists()
    log_content = log_path.read_text(encoding="utf-8")
    assert "REQ-logado.md" in log_content
    assert "backlog → done" in log_content


# ---------------------------------------------------------------------------
# CLI — `trackfw req list`
# ---------------------------------------------------------------------------

def _run_cli(monkeypatch, argv):
    from trackfw import cli

    monkeypatch.setattr(sys, "argv", ["trackfw"] + argv)
    cli.main()


def test_cli_req_list_by_state(tmp_path, monkeypatch, capsys):
    _write(str(tmp_path / "docs" / "req" / "backlog" / "REQ-cli.md"), _req_body())
    monkeypatch.chdir(tmp_path)
    _config.reset()

    _run_cli(monkeypatch, ["req", "list"])

    out = capsys.readouterr().out
    assert "REQ-cli.md" in out
    _config.reset()


def test_cli_req_list_by_agent(tmp_path, monkeypatch, capsys):
    _write(
        str(tmp_path / "docs" / "req" / "claude" / "wip" / "REQ-cli-agent.md"),
        _req_body("wip"),
    )
    _write(
        str(tmp_path / "trackfw.yaml"),
        "roadmap_namespacing: by_agent\nagents:\n  - claude\n",
    )
    monkeypatch.chdir(tmp_path)
    _config.reset()

    _run_cli(monkeypatch, ["req", "list"])

    out = capsys.readouterr().out
    assert "REQ-cli-agent.md" in out
    _config.reset()
