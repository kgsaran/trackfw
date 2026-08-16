"""
test_fatal_error.py — Handler global de erro em trackfw.cli:main() (defesa em
profundidade). Espelha npm/tests/*fatal*: prova que um erro não tratado que
escapa de args.func(args) produz mensagem limpa em stderr + exit != 0 — sem
traceback, sem caminho absoluto de instalação, sem versão do runtime — e que
TRACKFW_DEBUG=1 restaura o traceback completo.

REQ: docs/req/REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-stack-trace-
caminhos-absolutos-e-versao-do-runtime.md
"""

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import trackfw.cli as cli
import trackfw.config as trackfw_config
from trackfw.fatal_error import report_fatal_error


# ---------------------------------------------------------------------------
# report_fatal_error — unidade
# ---------------------------------------------------------------------------

def test_report_fatal_error_prints_clean_message_without_debug(capsys, monkeypatch):
    monkeypatch.delenv("TRACKFW_DEBUG", raising=False)
    report_fatal_error(RuntimeError("boom: /Users/someone/project/lib.py:42"), command="roadmap")
    captured = capsys.readouterr()
    assert captured.err == "trackfw roadmap: boom: /Users/someone/project/lib.py:42\n"
    assert "Traceback" not in captured.err


def test_report_fatal_error_without_command_uses_bare_prefix(capsys, monkeypatch):
    monkeypatch.delenv("TRACKFW_DEBUG", raising=False)
    report_fatal_error(RuntimeError("boom"))
    captured = capsys.readouterr()
    assert captured.err == "trackfw: boom\n"


def test_report_fatal_error_preserves_multiline_message_intact(capsys, monkeypatch):
    # Not just today's one-line domain message — a future multi-line message
    # (e.g. "Adopt it with: ...") must survive intact, never truncated at the
    # first newline. Mirrors the equivalent Node test in fatal-error.test.js.
    monkeypatch.delenv("TRACKFW_DEBUG", raising=False)
    report_fatal_error(
        RuntimeError("line one\nAdopt it with: trackfw agents install --force"),
        command="agents",
    )
    captured = capsys.readouterr()
    assert captured.err == "trackfw agents: line one\nAdopt it with: trackfw agents install --force\n"


def test_report_fatal_error_debug_restores_traceback(capsys, monkeypatch):
    monkeypatch.setenv("TRACKFW_DEBUG", "1")
    try:
        raise RuntimeError("boom with traceback")
    except RuntimeError as error:
        report_fatal_error(error, command="roadmap")
    captured = capsys.readouterr()
    assert "Traceback (most recent call last)" in captured.err
    assert "boom with traceback" in captured.err
    assert __file__ in captured.err


# ---------------------------------------------------------------------------
# trackfw.cli.main() — integração: erro não tratado que escapa de args.func
# ---------------------------------------------------------------------------

def _raise_uncaught(*_args, **_kwargs):
    raise RuntimeError(f"synthetic failure touching {os.path.abspath(__file__)}")


def test_main_uncaught_exception_prints_clean_message_and_exits_nonzero(capsys, monkeypatch, tmp_path):
    # roadmap list's own handler (_cmd_list) has NO try/except around
    # cfg_module.load() — the real gap this ML closes: nothing today stops
    # an exception raised there from reaching the top of main() uncaught.
    monkeypatch.setattr(trackfw_config, "load", _raise_uncaught)
    monkeypatch.setattr(sys, "argv", ["trackfw", "roadmap", "list"])
    monkeypatch.chdir(tmp_path)

    with pytest.raises(SystemExit) as excinfo:
        cli.main()

    assert excinfo.value.code == 1
    captured = capsys.readouterr()
    assert captured.err == f"trackfw roadmap: synthetic failure touching {os.path.abspath(__file__)}\n"
    # AC1 — no stack trace, no source line/file marker beyond the message
    # itself, no interpreter version.
    assert "Traceback" not in captured.err
    assert "line " not in captured.err
    assert "Python" not in captured.err
    assert sys.version not in captured.err


def test_main_uncaught_exception_with_debug_restores_traceback(capsys, monkeypatch, tmp_path):
    monkeypatch.setattr(trackfw_config, "load", _raise_uncaught)
    monkeypatch.setattr(sys, "argv", ["trackfw", "roadmap", "list"])
    monkeypatch.setenv("TRACKFW_DEBUG", "1")
    monkeypatch.chdir(tmp_path)

    with pytest.raises(SystemExit) as excinfo:
        cli.main()

    assert excinfo.value.code == 1
    captured = capsys.readouterr()
    assert "Traceback (most recent call last)" in captured.err
    assert "synthetic failure touching" in captured.err


# ---------------------------------------------------------------------------
# Não-regressão — caminho já limpo (IntegrationError → SystemExit) continua
# byte a byte, o novo `except Exception` não intercepta SystemExit.
# ---------------------------------------------------------------------------

def test_main_preserves_existing_clean_systemexit_path(capsys, monkeypatch, tmp_path):
    monkeypatch.setattr(sys, "argv", ["trackfw", "roadmap", "move", "nao-existe", "wip"])
    monkeypatch.chdir(tmp_path)

    with pytest.raises(SystemExit) as excinfo:
        cli.main()

    assert excinfo.value.code == 1
    captured = capsys.readouterr()
    assert captured.err == 'Erro: Roadmap "nao-existe" não encontrado em nenhum estado.\n'
    assert "Traceback" not in captured.err
