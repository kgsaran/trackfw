"""
test_commands_validate_status.py — Testes para commands/validate.py e commands/status.py.

Usa tempfile.mkdtemp() e chama as funções diretamente (sem subprocess).
"""

import os
import sys
import tempfile
import shutil
import unittest

# Garante que o pacote pypi/trackfw é importável mesmo sem instalação
_HERE = os.path.dirname(os.path.abspath(__file__))
_PYPI = os.path.dirname(_HERE)
if _PYPI not in sys.path:
    sys.path.insert(0, _PYPI)

from trackfw import config as _config
from trackfw import validator as _validator
from trackfw.commands import validate as _validate_cmd
from trackfw.commands import status as _status_cmd


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_file(path: str, content: str = ""):
    """Cria um arquivo (e diretórios pai) com o conteúdo dado."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)


def _make_dirs(*paths):
    for p in paths:
        os.makedirs(p, exist_ok=True)


# ---------------------------------------------------------------------------
# Testes de validate
# ---------------------------------------------------------------------------

class TestValidateSemViolations(unittest.TestCase):
    """Projeto com dirs vazios → validate() retorna violations=[]."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        # Criar dirs padrão sem conteúdo
        _make_dirs(
            os.path.join(self.tmp, "docs", "adr"),
            os.path.join(self.tmp, "docs", "req"),
            os.path.join(self.tmp, "docs", "roadmaps", "wip"),
            os.path.join(self.tmp, "docs", "roadmaps", "blocked"),
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_validate_sem_violations(self):
        """Dirs vazios não geram violations nem warnings relevantes."""
        old_cwd = os.getcwd()
        os.chdir(self.tmp)
        try:
            result = _validator.validate(cwd=self.tmp)
            self.assertEqual(result["violations"], [],
                             "Projeto vazio não deve ter violations")
        finally:
            os.chdir(old_cwd)
            _config.reset()


class TestValidateComViolation(unittest.TestCase):
    """wip com 2 arquivos e wip_limit=1 → warning de WIP limit."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        _make_dirs(wip_dir)
        # 2 roadmaps em wip com REQ e critérios (para não gerar outras violations)
        for i in range(1, 3):
            _make_file(
                os.path.join(wip_dir, f"roadmap-{i}.md"),
                f"# Roadmap {i}\n\nREQ: REQ-2026-01-0{i}-exemplo.md\n\n## Acceptance Criteria\n- [ ] item\n",
            )
        # trackfw.yaml com wip_limit: 1
        _make_file(
            os.path.join(self.tmp, "trackfw.yaml"),
            "roadmap_dir: docs/roadmaps\nwip_limit: 1\n",
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_validate_com_violation_wip_limit(self):
        """2 arquivos em wip com wip_limit=1 deve gerar warning de WIP limit."""
        old_cwd = os.getcwd()
        os.chdir(self.tmp)
        try:
            result = _validator.validate(cwd=self.tmp)
            warnings = result.get("warnings", [])
            msgs = [w["message"] if isinstance(w, dict) else str(w) for w in warnings]
            found = any("wip" in m.lower() and "limit" in m.lower() for m in msgs)
            self.assertTrue(found,
                            f"Esperava warning de WIP limit, obteve: {msgs}")
        finally:
            os.chdir(old_cwd)
            _config.reset()


class TestValidateLenientExitZero(unittest.TestCase):
    """Modo lenient: violations existem mas são convertidas em warnings (exit 0)."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        _make_dirs(wip_dir)
        # Roadmap sem REQ → violation em modo strict
        _make_file(
            os.path.join(wip_dir, "roadmap-sem-req.md"),
            "# Roadmap sem REQ\n\nConteúdo sem link de REQ.\n",
        )
        # trackfw.yaml com governance_mode: lenient
        _make_file(
            os.path.join(self.tmp, "trackfw.yaml"),
            "roadmap_dir: docs/roadmaps\ngovernance_mode: lenient\n",
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_validate_lenient_violations_viram_warnings(self):
        """Em modo lenient, violations são promovidas a warnings; violations=[]."""
        old_cwd = os.getcwd()
        os.chdir(self.tmp)
        try:
            result = _validator.validate(cwd=self.tmp)
            self.assertEqual(result["violations"], [],
                             "Modo lenient deve zerar violations")
            self.assertGreater(len(result["warnings"]), 0,
                               "Modo lenient deve ter pelo menos 1 warning")
        finally:
            os.chdir(old_cwd)
            _config.reset()


# ---------------------------------------------------------------------------
# Testes de status — modo flat
# ---------------------------------------------------------------------------

class TestStatusFlat(unittest.TestCase):
    """get_status() no modo flat conta corretamente ADRs, REQs e Roadmaps."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        # ADRs
        adr_dir = os.path.join(self.tmp, "docs", "adr")
        _make_dirs(adr_dir)
        for i in range(1, 4):
            _make_file(os.path.join(adr_dir, f"ADR-00{i}-exemplo.md"), f"# ADR {i}\n")

        # REQs (2 Open, 1 Closed)
        req_dir = os.path.join(self.tmp, "docs", "req")
        _make_dirs(req_dir)
        _make_file(
            os.path.join(req_dir, "REQ-2026-01-01-a.md"),
            "---\nstatus: Open\n---\n# REQ A\n",
        )
        _make_file(
            os.path.join(req_dir, "REQ-2026-01-02-b.md"),
            "---\nstatus: Open\n---\n# REQ B\n",
        )
        _make_file(
            os.path.join(req_dir, "REQ-2026-01-03-c.md"),
            "---\nstatus: Closed\n---\n# REQ C\n",
        )

        # Roadmaps
        roadmap_dir = os.path.join(self.tmp, "docs", "roadmaps")
        for state, count in [("backlog", 5), ("wip", 1), ("blocked", 0), ("done", 23), ("abandoned", 2)]:
            d = os.path.join(roadmap_dir, state)
            _make_dirs(d)
            for i in range(count):
                _make_file(os.path.join(d, f"rm-{i+1}.md"), f"# Roadmap {i+1}\n")

        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_status_flat_conta_adrs(self):
        """Conta 3 ADRs corretamente."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("ADRs:      3", out)

    def test_status_flat_conta_reqs(self):
        """Conta 3 REQs (2 Open, 1 Closed)."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("REQs:      3", out)
        self.assertIn("2 Open", out)
        self.assertIn("1 Closed", out)

    def test_status_flat_conta_roadmaps(self):
        """Conta roadmaps por estado."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("backlog:  5", out)
        self.assertIn("wip:      1", out)
        self.assertIn("blocked:  0", out)
        self.assertIn("done:     23", out)
        self.assertIn("abandoned: 2", out)


# ---------------------------------------------------------------------------
# Testes de status — modo by_agent
# ---------------------------------------------------------------------------

class TestStatusByAgent(unittest.TestCase):
    """get_status() em modo by_agent exibe breakdown por agente."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

        # trackfw.yaml com roadmap_namespacing: by_agent
        _make_file(
            os.path.join(self.tmp, "trackfw.yaml"),
            "roadmap_dir: docs/roadmaps\nroadmap_namespacing: by_agent\nagents:\n- zeus\n- apolo\n",
        )

        # Dirs de agentes
        roadmap_dir = os.path.join(self.tmp, "docs", "roadmaps")
        for agent, wip_count, done_count in [("zeus", 1, 10), ("apolo", 0, 5)]:
            for state, count in [("wip", wip_count), ("done", done_count), ("backlog", 0), ("blocked", 0), ("abandoned", 0)]:
                d = os.path.join(roadmap_dir, agent, state)
                _make_dirs(d)
                for i in range(count):
                    _make_file(os.path.join(d, f"rm-{i+1}.md"), f"# Roadmap\n")

        # Dirs de ADR e REQ (vazios)
        _make_dirs(
            os.path.join(self.tmp, "docs", "adr"),
            os.path.join(self.tmp, "docs", "req"),
        )

        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_status_by_agent_breakdown(self):
        """Modo by_agent exibe seção 'Roadmaps (by agent):' com dados por agente."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("by agent", out.lower(),
                      "Deve conter seção by agent")
        self.assertIn("zeus", out)
        self.assertIn("apolo", out)

    def test_status_by_agent_totais(self):
        """Totais agregados: wip=1, done=15."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("wip:      1", out)
        self.assertIn("done:     15", out)

    def test_status_by_agent_zeus_wip(self):
        """zeus deve aparecer com wip=1."""
        out = _status_cmd.get_status(cwd=self.tmp)
        # Zeus tem wip=1 e done=10
        self.assertRegex(out, r"zeus.*wip=1")

    def test_status_by_agent_apolo_done(self):
        """apolo deve aparecer com done=5."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertRegex(out, r"apolo.*done=5")


if __name__ == "__main__":
    unittest.main()
