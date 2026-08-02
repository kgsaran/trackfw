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


class TestValidateOkMessageUsaI18n(unittest.TestCase):
    """ML-1C — o texto de sucesso do comando `validate` deve vir da chave de
    i18n 'validate.ok' (mesmo mecanismo do Go/Node), não de um literal
    hardcoded no comando."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
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

    def test_validate_ok_imprime_chave_i18n(self):
        import io
        import types
        from trackfw.i18n import t as i18n_t

        old_cwd = os.getcwd()
        os.chdir(self.tmp)
        captured = io.StringIO()
        old_stdout = sys.stdout
        sys.stdout = captured
        try:
            _validate_cmd.run(types.SimpleNamespace(json=False))
        finally:
            sys.stdout = old_stdout
            os.chdir(old_cwd)
            _config.reset()
        output = captured.getvalue()
        # io.StringIO().isatty() é False -> _supports_color() é False -> sem ANSI.
        # Único print no cenário sem violations/warnings em modo strict: a linha de i18n.
        self.assertEqual(output.strip(), i18n_t("validate.ok"))


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
        self.assertIn("ADRs        3", out)

    def test_status_flat_conta_reqs(self):
        """Conta 3 REQs (2 Open, 1 Closed) — discriminação Open/Done/Closed."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("REQs        3", out)
        self.assertIn("2 Open · 0 Done · 1 Closed", out)

    def test_status_flat_conta_roadmaps(self):
        """Conta roadmaps por estado, incluindo analyzing (0 neste fixture)."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("backlog 5 · analyzing 0 · wip 1", out)
        self.assertIn("blocked 0 · done 23 · abandoned 2", out)
        self.assertIn("Roadmaps    31", out)

    def test_status_flat_tem_moldura_e_secoes(self):
        """Formato consolidado: moldura, Inventory, WIP, Blocked, Done."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("── trackfw status ──", out)
        self.assertIn("────────────────────────────────────────", out)
        self.assertIn("📊 Inventory", out)
        self.assertIn("🔄 WIP (1)", out)
        self.assertIn("❌ Blocked (0)", out)
        self.assertIn("✅ Done (last 5)", out)


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
        """Modo by_agent exibe a seção '⚙ WIP by Agent' — espelha GetStatus() em
        internal/validator/validator.go e getStatus() em npm/src/validator/index.js.
        Reescrito no ML-2B: a seção antiga '⚙ Roadmaps by Agent' misturava nomes de
        estado (backlog/done/...) com nomes de agente e mantinha as seções flat
        zeradas — divergência corrigida alinhando Python a Go/Node."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("⚙ WIP by Agent", out)
        self.assertIn("zeus", out)
        # As seções flat não se aplicam no modo by_agent e devem ser omitidas,
        # tal como em Go/Node.
        self.assertNotIn("🔄 WIP (", out)
        self.assertNotIn("❌ Blocked (", out)
        self.assertNotIn("✅ Done (last 5)", out)

    def test_status_by_agent_totais(self):
        """Totais agregados no bloco Inventory: wip=1, done=15."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertIn("wip 1", out)
        self.assertIn("done 15", out)

    def test_status_by_agent_zeus_wip(self):
        """zeus tem wip=1 e deve aparecer em '[zeus] WIP (1)' listando o arquivo."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertRegex(out, r"\[zeus\] WIP \(1\)")
        self.assertIn("rm-1.md", out)

    def test_status_by_agent_apolo_sem_wip_nao_listado(self):
        """apolo tem wip=0 — não deve aparecer na seção '⚙ WIP by Agent', pois
        Go/Node só listam agentes com wip > 0 (GetStatus, validator.go linhas
        774-782). apolo só tem roadmaps em done/, que não é exibido por agente."""
        out = _status_cmd.get_status(cwd=self.tmp)
        self.assertNotIn("apolo", out)


class TestListDirsOrdena(unittest.TestCase):
    """ML-1A — _list_dirs (status.py) deve ordenar como a irmã _list_files já
    faz, alinhado a Go (sort.Strings) e Node (.sort())."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_list_dirs_ordena_subdirs_criados_fora_de_ordem(self):
        # Criados fora de ordem alfabética de propósito.
        _make_dirs(
            os.path.join(self.tmp, "zeus"),
            os.path.join(self.tmp, "apolo"),
        )
        result = _status_cmd._list_dirs(self.tmp)
        self.assertEqual(result, ["apolo", "zeus"])


class TestStatusByAgentFallbackSemAgentsConfigurados(unittest.TestCase):
    """ML-1A — fixture by_agent SEM `agents:` no trackfw.yaml, portanto via
    fallback de _get_agents/_list_dirs, com subdiretórios criados fora de
    ordem alfabética. As 3 CLIs devem produzir a mesma ordem de agentes."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()

        # trackfw.yaml SEM a chave "agents" — força o fallback de varredura
        # de subdiretórios.
        _make_file(
            os.path.join(self.tmp, "trackfw.yaml"),
            "roadmap_dir: docs/roadmaps\nroadmap_namespacing: by_agent\n",
        )

        # Subdiretórios de agente criados fora de ordem alfabética
        # (zeus antes de apolo) para exercitar a ordenação do fallback.
        roadmap_dir = os.path.join(self.tmp, "docs", "roadmaps")
        for agent in ["zeus", "apolo"]:
            for state in ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]:
                d = os.path.join(roadmap_dir, agent, state)
                _make_dirs(d)
                if state == "wip":
                    _make_file(os.path.join(d, "rm-1.md"), "# Roadmap\n")

        _make_dirs(
            os.path.join(self.tmp, "docs", "adr"),
            os.path.join(self.tmp, "docs", "req"),
        )

        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_fallback_lista_agentes_em_ordem_alfabetica(self):
        """_get_agents() via fallback (_list_dirs) devolve ['apolo', 'zeus'],
        não a ordem de criação no filesystem ('zeus', 'apolo'). roadmap_dir é
        resolvido contra self.tmp, espelhando cfg_local em get_status()."""
        cfg = _config.load(self.tmp)
        cfg["roadmap_dir"] = os.path.join(self.tmp, cfg.get("roadmap_dir", "docs/roadmaps"))
        agents = _status_cmd._get_agents(cfg)
        self.assertEqual(agents, ["apolo", "zeus"])

    def test_fallback_ordena_saida_de_status(self):
        """A seção '⚙ WIP by Agent' lista apolo antes de zeus, refletindo a
        ordenação alfabética do fallback."""
        out = _status_cmd.get_status(cwd=self.tmp)
        pos_apolo = out.find("[apolo]")
        pos_zeus = out.find("[zeus]")
        self.assertGreater(pos_apolo, -1)
        self.assertGreater(pos_zeus, -1)
        self.assertLess(pos_apolo, pos_zeus)


class TestAnalyzingStateNoFolderStatusViolation(unittest.TestCase):
    """Roadmap em analyzing/ com status: analyzing não deve gerar folder_status warning."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        analyzing_dir = os.path.join(self.tmp, "docs", "roadmaps", "analyzing")
        _make_dirs(analyzing_dir)
        _make_file(
            os.path.join(analyzing_dir, "ROADMAP-em-analise.md"),
            "---\nstatus: analyzing\ndate: 2026-07-26\n---\n# Roadmap: Em Análise\n\n## Objetivo\nPlanejamento.\n",
        )
        _make_file(
            os.path.join(self.tmp, "trackfw.yaml"),
            "roadmap_dir: docs/roadmaps\n",
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_analyzing_no_folder_status_warning(self):
        """Roadmap em pasta analyzing/ com status: analyzing não gera folder_status."""
        old_cwd = os.getcwd()
        os.chdir(self.tmp)
        try:
            cfg = _config.load(self.tmp)
            warnings = _validator.validate_folder_status_coherence(cfg)
            for w in warnings:
                self.assertNotIn(
                    "ROADMAP-em-analise.md", w,
                    f"Roadmap em analyzing/ NÃO deve gerar folder_status warning, obteve: {w}",
                )
        finally:
            os.chdir(old_cwd)
            _config.reset()


class TestAnalyzingStateWipLimitDoesNotCount(unittest.TestCase):
    """Roadmap em analyzing/ NÃO deve ser contado pelo wip_limit."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        wip_dir = os.path.join(self.tmp, "docs", "roadmaps", "wip")
        analyzing_dir = os.path.join(self.tmp, "docs", "roadmaps", "analyzing")
        _make_dirs(wip_dir, analyzing_dir)
        _make_file(
            os.path.join(wip_dir, "ROADMAP-em-wip.md"),
            "# Roadmap em WIP\n\nREQ: REQ-001\n",
        )
        _make_file(
            os.path.join(analyzing_dir, "ROADMAP-em-analise.md"),
            "---\nstatus: analyzing\n---\n# Roadmap em Análise\n",
        )
        _make_file(
            os.path.join(self.tmp, "trackfw.yaml"),
            "roadmap_dir: docs/roadmaps\nwip_limit: 1\n",
        )
        _config.reset()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)
        _config.reset()

    def test_wip_limit_ignores_analyzing(self):
        """wip_limit=1 com 1 wip + 1 analyzing não deve gerar warning."""
        old_cwd = os.getcwd()
        os.chdir(self.tmp)
        try:
            cfg = _config.load(self.tmp)
            result = _validator.validate_wip_limit(cfg)
            warnings = result.get("warnings", [])
            self.assertEqual(
                warnings, [],
                f"wip_limit NÃO deve contar roadmaps em analyzing/ — esperado [], obteve: {warnings}",
            )
        finally:
            os.chdir(old_cwd)
            _config.reset()


if __name__ == "__main__":
    unittest.main()
