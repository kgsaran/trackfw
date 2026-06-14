"""
Testes unitários para pypi/trackfw/serve/api_board.py,
api_file.py e api_metrics.py (ML-4C).

Usa pytest + tmp_path fixture para criar estruturas temporárias.
"""

import os
import sys
from datetime import datetime
from unittest.mock import MagicMock

import pytest

# Garante importabilidade do pacote pypi/trackfw
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.serve.api_board import get_board
from trackfw.serve.api_file import get_file, _is_safe_path
from trackfw.serve.api_metrics import get_metrics, _calc_cycle_time


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_md(path, title=None):
    """Cria arquivo .md com conteúdo mínimo no caminho indicado."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    content = f"# {title}\n\nConteúdo de teste.\n" if title else "Conteúdo de teste.\n"
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)
    return content


# ---------------------------------------------------------------------------
# api_board — modo flat
# ---------------------------------------------------------------------------

class TestBoardFlat:

    def test_board_flat_mode(self, tmp_path):
        """Modo flat: 2 roadmaps em estados diferentes → cards nas columns corretas."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        _make_md(str(roadmap_dir / "wip" / "ROADMAP-2026-06-01-auth.md"), "Auth")
        _make_md(str(roadmap_dir / "done" / "ROADMAP-2026-05-01-ci.md"), "CI")

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "flat",
        }

        result = get_board(cfg)

        assert "columns" in result
        assert len(result["columns"]["wip"]) == 1
        assert len(result["columns"]["done"]) == 1
        assert result["columns"]["backlog"] == []

        wip_card = result["columns"]["wip"][0]
        assert wip_card["file"] == "ROADMAP-2026-06-01-auth.md"
        assert wip_card["title"] == "Auth"
        assert wip_card["state"] == "wip"
        assert wip_card["agent"] == ""

    def test_board_empty(self, tmp_path):
        """Dir existe mas vazio → columns vazias sem erro."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        roadmap_dir.mkdir(parents=True)

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "flat",
        }

        result = get_board(cfg)

        assert "columns" in result
        for state in ("wip", "backlog", "blocked", "done", "abandoned"):
            assert result["columns"][state] == []
        assert result["agents"] == []


# ---------------------------------------------------------------------------
# api_board — modo by_agent
# ---------------------------------------------------------------------------

class TestBoardByAgent:

    def test_board_by_agent(self, tmp_path):
        """Modo by_agent: estrutura rootDir/agente/estado/arquivo.md → agent correto."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        _make_md(str(roadmap_dir / "zeus" / "wip" / "ROADMAP-zeus-001.md"), "Zeus WIP")
        _make_md(str(roadmap_dir / "apolo" / "done" / "ROADMAP-apolo-001.md"), "Apolo Done")

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "by_agent",
            "agents": ["zeus", "apolo"],
        }

        result = get_board(cfg)

        assert "columns" in result

        # Zeus deve aparecer no WIP
        wip_cards = result["columns"]["wip"]
        assert len(wip_cards) == 1
        assert wip_cards[0]["agent"] == "zeus"
        assert wip_cards[0]["title"] == "Zeus WIP"

        # Apolo deve aparecer no done
        done_cards = result["columns"]["done"]
        assert len(done_cards) == 1
        assert done_cards[0]["agent"] == "apolo"
        assert done_cards[0]["title"] == "Apolo Done"

        # Ambos os agents detectados
        assert sorted(result["agents"]) == ["apolo", "zeus"]

    def test_board_by_agent_autodetect(self, tmp_path):
        """Modo by_agent sem lista agents → detecta agents automaticamente pelo filesystem."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        _make_md(str(roadmap_dir / "artemis" / "backlog" / "ROADMAP-qa-001.md"), "QA Backlog")

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "by_agent",
            # agents não informado — deve auto-detectar
        }

        result = get_board(cfg)

        backlog_cards = result["columns"]["backlog"]
        assert len(backlog_cards) == 1
        assert backlog_cards[0]["agent"] == "artemis"
        assert "artemis" in result["agents"]


# ---------------------------------------------------------------------------
# api_file — validacao de path
# ---------------------------------------------------------------------------

class TestFileAPI:

    def _make_handler_mock(self):
        """Cria mock mínimo de BaseHTTPRequestHandler."""
        handler = MagicMock()
        handler.wfile = MagicMock()
        handler.wfile.write = MagicMock()
        return handler

    def test_file_valid_path(self, tmp_path, monkeypatch):
        """Path dentro de dir autorizado → conteúdo retornado (200)."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        test_file = roadmap_dir / "wip" / "ROADMAP-test.md"
        content = _make_md(str(test_file), "Teste")

        # Simular cwd = tmp_path
        monkeypatch.chdir(tmp_path)

        # path relativo ao cwd
        rel_path = os.path.relpath(str(test_file), str(tmp_path))

        from urllib.parse import urlparse, urlencode
        from urllib.parse import ParseResult
        parsed_url = urlparse(f"/?path={rel_path}")

        handler = self._make_handler_mock()
        cfg = {
            "adr_dirs": ["docs/adr"],
            "req_dir": "docs/req",
            "roadmap_dir": "docs/roadmaps",
        }

        get_file(cfg, parsed_url, handler)

        handler.send_response.assert_called_once_with(200)
        handler.wfile.write.assert_called_once()
        written = handler.wfile.write.call_args[0][0]
        assert b"Teste" in written

    def test_file_path_traversal(self, tmp_path, monkeypatch):
        """path=../../../etc/passwd → 403 (path traversal bloqueado)."""
        monkeypatch.chdir(tmp_path)

        from urllib.parse import urlparse
        parsed_url = urlparse("/?path=../../../etc/passwd")

        handler = self._make_handler_mock()
        cfg = {
            "adr_dirs": ["docs/adr"],
            "req_dir": "docs/req",
            "roadmap_dir": "docs/roadmaps",
        }

        get_file(cfg, parsed_url, handler)

        handler.send_error.assert_called_once()
        error_code = handler.send_error.call_args[0][0]
        assert error_code == 403

    def test_file_outside_allowed(self, tmp_path, monkeypatch):
        """Path fora dos dirs autorizados → 403."""
        monkeypatch.chdir(tmp_path)

        # Criar arquivo em dir não autorizado
        secret = tmp_path / "secret.md"
        secret.write_text("segredo", encoding="utf-8")

        from urllib.parse import urlparse
        # path absoluto apontando para fora dos dirs autorizados
        parsed_url = urlparse(f"/?path={secret}")

        handler = self._make_handler_mock()
        cfg = {
            "adr_dirs": ["docs/adr"],
            "req_dir": "docs/req",
            "roadmap_dir": "docs/roadmaps",
        }

        get_file(cfg, parsed_url, handler)

        handler.send_error.assert_called_once()
        error_code = handler.send_error.call_args[0][0]
        assert error_code == 403

    def test_is_safe_path_dentro(self, tmp_path):
        """_is_safe_path: path dentro do base_dir → True."""
        base = str(tmp_path / "docs")
        child = str(tmp_path / "docs" / "adr" / "ADR-001.md")
        assert _is_safe_path(base, child) is True

    def test_is_safe_path_fora(self, tmp_path):
        """_is_safe_path: path fora do base_dir → False."""
        base = str(tmp_path / "docs")
        outside = str(tmp_path / "secrets" / "token.txt")
        assert _is_safe_path(base, outside) is False

    def test_is_safe_path_traversal(self, tmp_path):
        """_is_safe_path: path com '..' que sai do base → False."""
        base = str(tmp_path / "docs" / "roadmaps")
        traversal = str(tmp_path / "docs" / "roadmaps" / ".." / ".." / "etc" / "passwd")
        assert _is_safe_path(base, traversal) is False


# ---------------------------------------------------------------------------
# api_metrics
# ---------------------------------------------------------------------------

class TestMetricsAPI:

    def test_metrics_no_log(self, tmp_path):
        """Sem .trackfw-log → métricas zeradas sem exceção."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        roadmap_dir.mkdir(parents=True)

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "flat",
        }

        result = get_metrics(cfg)

        assert result["lead_time_avg_days"] == 0.0
        assert result["cycle_time_avg_days"] == 0.0
        assert result["abandonment_rate"] == 0.0
        assert result["burndown"] == []
        assert isinstance(result["state_distribution"], dict)

    def test_metrics_with_log(self, tmp_path):
        """Log com transições wip → done → cycle_time_avg_days calculado corretamente."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        roadmap_dir.mkdir(parents=True)

        log_content = (
            "2026-06-10 08:00  ROADMAP-feature-auth.md  backlog  →  wip\n"
            "2026-06-12 08:00  ROADMAP-feature-auth.md  wip      →  done\n"
        )
        log_path = roadmap_dir / ".trackfw-log"
        log_path.write_text(log_content, encoding="utf-8")

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "flat",
        }

        result = get_metrics(cfg)

        # cycle time: wip (10 Jun 08:00) → done (12 Jun 08:00) = 2 dias
        assert result["cycle_time_avg_days"] == 2.0
        # lead time: backlog (10 Jun 08:00) → done (12 Jun 08:00) = 2 dias
        assert result["lead_time_avg_days"] == 2.0
        assert result["abandonment_rate"] == 0.0
        assert len(result["burndown"]) > 0

    def test_metrics_with_abandonment(self, tmp_path):
        """Log com roadmap abandonado → abandonment_rate > 0."""
        roadmap_dir = tmp_path / "docs" / "roadmaps"
        roadmap_dir.mkdir(parents=True)

        log_content = (
            "2026-06-01 09:00  ROADMAP-done.md       backlog  →  wip\n"
            "2026-06-03 09:00  ROADMAP-done.md       wip      →  done\n"
            "2026-06-01 09:00  ROADMAP-abandoned.md  backlog  →  wip\n"
            "2026-06-04 09:00  ROADMAP-abandoned.md  wip      →  abandoned\n"
        )
        log_path = roadmap_dir / ".trackfw-log"
        log_path.write_text(log_content, encoding="utf-8")

        cfg = {
            "roadmap_dir": str(roadmap_dir),
            "roadmap_namespacing": "flat",
        }

        result = get_metrics(cfg)

        # 1 abandonado, 1 concluido → 50%
        assert result["abandonment_rate"] == 0.5

    def test_calc_cycle_time_direto(self):
        """_calc_cycle_time: transições wip→done de 3 dias → média 3.0."""
        transitions = [
            {
                "timestamp": datetime(2026, 6, 1, 10, 0),
                "basename": "ROADMAP-x.md",
                "from": "backlog",
                "to": "wip",
            },
            {
                "timestamp": datetime(2026, 6, 4, 10, 0),
                "basename": "ROADMAP-x.md",
                "from": "wip",
                "to": "done",
            },
        ]
        result = _calc_cycle_time(transitions)
        assert result == 3.0
