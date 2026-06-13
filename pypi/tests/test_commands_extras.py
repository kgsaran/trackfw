"""
Testes de unidade para commands/metrics.py, commands/context.py, commands/plugins.py.
Usa unittest (stdlib) — sem dependências externas.
"""

import io
import os
import sys
import tempfile
import shutil
import unittest
from unittest.mock import patch

# Garante que o pacote pypi/trackfw seja importável
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))


class TestMetrics(unittest.TestCase):
    """Testes para commands/metrics.py."""

    def setUp(self):
        self.orig_dir = os.getcwd()
        self.tmpdir = tempfile.mkdtemp()
        os.chdir(self.tmpdir)

    def tearDown(self):
        os.chdir(self.orig_dir)
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_metrics_sem_log(self):
        """Sem .trackfw-log → imprime 'No log found' sem erro."""
        # Importa o módulo de métricas
        from trackfw.commands import metrics as metrics_mod

        # Garante que o arquivo de log não existe
        log_path = os.path.join("docs", "roadmaps", ".trackfw-log")
        self.assertFalse(os.path.exists(log_path))

        captured = io.StringIO()
        with patch("sys.stdout", captured):
            # Simula args sem --since, --days, --export
            class FakeArgs:
                days = None
                since = None
                export = None

            metrics_mod._cmd_metrics(FakeArgs())

        output = captured.getvalue()
        self.assertIn("No log found", output)

    def test_metrics_com_log(self):
        """Com .trackfw-log válido → imprime métricas."""
        from trackfw.commands import metrics as metrics_mod

        # Cria estrutura de diretórios e arquivo de log
        log_dir = os.path.join("docs", "roadmaps")
        os.makedirs(log_dir, exist_ok=True)
        log_path = os.path.join(log_dir, ".trackfw-log")

        log_content = (
            "2026-06-01 10:00  ROADMAP-2026-06-01-auth.md              backlog → wip\n"
            "2026-06-05 12:00  ROADMAP-2026-06-01-auth.md              wip → done\n"
        )
        with open(log_path, "w", encoding="utf-8") as f:
            f.write(log_content)

        captured = io.StringIO()
        with patch("sys.stdout", captured):
            class FakeArgs:
                days = None
                since = None
                export = None

            metrics_mod._cmd_metrics(FakeArgs())

        output = captured.getvalue()
        self.assertIn("trackfw metrics", output)
        # Deve ter cycle time calculado (não n/a)
        self.assertIn("Throughput", output)

    def test_parse_log_arquivo_inexistente(self):
        """_parse_log retorna [] se arquivo não existe."""
        from trackfw.commands import metrics as metrics_mod

        result = metrics_mod._parse_log("/tmp/nao-existe-trackfw-log.txt")
        self.assertEqual(result, [])

    def test_parse_log_linhas_validas(self):
        """_parse_log extrai transições corretamente."""
        from trackfw.commands import metrics as metrics_mod

        log_content = (
            "2026-06-01 10:00  ROADMAP-test.md              backlog → wip\n"
            "2026-06-03 14:00  ROADMAP-test.md              wip → done\n"
            "\n"
            "linha invalida sem formato correto\n"
        )
        log_path = os.path.join(self.tmpdir, "test.log")
        with open(log_path, "w") as f:
            f.write(log_content)

        transitions = metrics_mod._parse_log(log_path)
        self.assertEqual(len(transitions), 2)
        self.assertEqual(transitions[0]["from"], "backlog")
        self.assertEqual(transitions[0]["to"], "wip")
        self.assertEqual(transitions[1]["to"], "done")

    def test_format_duration(self):
        """_format_duration formata horas e dias corretamente."""
        from trackfw.commands import metrics as metrics_mod

        # 2 horas
        self.assertEqual(metrics_mod._format_duration(7200), "2 hours")
        # 1 dia e 3 horas = 27 horas
        self.assertIn("days", metrics_mod._format_duration(27 * 3600))

    def test_calculate_sem_done(self):
        """_calculate com transições sem done → cycle_time_mean_s=0."""
        from trackfw.commands import metrics as metrics_mod

        from datetime import datetime
        transitions = [
            {"timestamp": datetime(2026, 6, 1, 10, 0), "basename": "R.md", "from": "backlog", "to": "wip"},
        ]
        result = metrics_mod._calculate(transitions)
        self.assertEqual(result["cycle_time_mean_s"], 0.0)
        self.assertEqual(result["throughput"], 0.0)
        self.assertEqual(len(result["wip_entries"]), 1)


class TestContext(unittest.TestCase):
    """Testes para commands/context.py."""

    def setUp(self):
        self.orig_dir = os.getcwd()
        self.tmpdir = tempfile.mkdtemp()
        os.chdir(self.tmpdir)
        # Reset config singleton
        import trackfw.config as cfg_mod
        cfg_mod.reset()

    def tearDown(self):
        import trackfw.config as cfg_mod
        cfg_mod.reset()
        os.chdir(self.orig_dir)
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_context_markdown(self):
        """Sem arquivos de governança → gera saída markdown com ADRs e REQs (vazios)."""
        from trackfw.commands import context as ctx_mod

        captured = io.StringIO()
        with patch("sys.stdout", captured):
            class FakeArgs:
                format = "markdown"
                output = None

            ctx_mod._cmd_context(FakeArgs())

        output = captured.getvalue()
        self.assertIn("# trackfw governance context", output)
        self.assertIn("## ADRs", output)
        self.assertIn("## REQs", output)
        self.assertIn("## Roadmaps", output)
        self.assertIn("Governance score:", output)

    def test_context_json(self):
        """Formato json → saída JSON válida com chaves esperadas."""
        from trackfw.commands import context as ctx_mod
        import json

        captured = io.StringIO()
        with patch("sys.stdout", captured):
            class FakeArgs:
                format = "json"
                output = None

            ctx_mod._cmd_context(FakeArgs())

        output = captured.getvalue()
        data = json.loads(output)
        self.assertIn("score", data)
        self.assertIn("adrs", data)
        self.assertIn("reqs", data)
        self.assertIn("roadmaps", data)
        self.assertIn("violations", data)

    def test_context_markdown_com_adrs(self):
        """Com ADRs criados → eles aparecem na saída markdown."""
        from trackfw.commands import context as ctx_mod

        adr_dir = os.path.join(self.tmpdir, "docs", "adr")
        os.makedirs(adr_dir, exist_ok=True)

        # Cria um ADR de exemplo com frontmatter
        adr_content = "---\nstatus: Accepted\ndate: 2026-06-01\n---\n# ADR-001 Auth\n"
        with open(os.path.join(adr_dir, "ADR-001-auth.md"), "w") as f:
            f.write(adr_content)

        captured = io.StringIO()
        with patch("sys.stdout", captured):
            class FakeArgs:
                format = "markdown"
                output = None

            ctx_mod._cmd_context(FakeArgs())

        output = captured.getvalue()
        self.assertIn("ADR-001-auth.md", output)

    def test_context_output_para_arquivo(self):
        """--output FILE → grava contexto em arquivo."""
        from trackfw.commands import context as ctx_mod

        output_file = os.path.join(self.tmpdir, "context-output.md")

        class FakeArgs:
            format = "markdown"
            output = output_file

        ctx_mod._cmd_context(FakeArgs())

        self.assertTrue(os.path.exists(output_file))
        with open(output_file, "r") as f:
            content = f.read()
        self.assertIn("# trackfw governance context", content)

    def test_extract_frontmatter_field(self):
        """_extract_frontmatter_field extrai campo do bloco YAML."""
        from trackfw.commands import context as ctx_mod

        content = "---\nstatus: Accepted\ndate: 2026-06-01\n---\n# Titulo\n"
        self.assertEqual(ctx_mod._extract_frontmatter_field(content, "status"), "Accepted")
        self.assertEqual(ctx_mod._extract_frontmatter_field(content, "date"), "2026-06-01")
        self.assertEqual(ctx_mod._extract_frontmatter_field(content, "author"), "")

    def test_extract_inline_status(self):
        """_extract_inline_status extrai status da linha | Status: ..."""
        from trackfw.commands import context as ctx_mod

        content = "# REQ\n| Status: Open |\n| ADR: foo.md |\n"
        self.assertEqual(ctx_mod._extract_inline_status(content), "Open")

        content_sem = "# REQ\nSem status\n"
        self.assertEqual(ctx_mod._extract_inline_status(content_sem), "unknown")


class TestPlugins(unittest.TestCase):
    """Testes para commands/plugins.py."""

    def setUp(self):
        self.orig_dir = os.getcwd()
        self.tmpdir = tempfile.mkdtemp()
        os.chdir(self.tmpdir)

    def tearDown(self):
        os.chdir(self.orig_dir)
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_plugins_list_sem_plugins(self):
        """Sem plugins no PATH → lista vazia sem erro."""
        from trackfw.commands import plugins as plugins_mod

        # Força PATH a apontar para diretório vazio
        empty_dir = os.path.join(self.tmpdir, "empty_bin")
        os.makedirs(empty_dir, exist_ok=True)

        captured = io.StringIO()
        with patch("sys.stdout", captured), \
             patch.dict(os.environ, {"PATH": empty_dir}):

            class FakeArgs:
                plugins_command = "list"

            plugins_mod._dispatch(FakeArgs())

        output = captured.getvalue()
        self.assertIn("No plugins installed", output)

    def test_plugins_list_com_plugin(self):
        """Com executável trackfw-* no PATH → aparece na lista."""
        from trackfw.commands import plugins as plugins_mod

        # Cria executável fake no tmpdir
        plugin_file = os.path.join(self.tmpdir, "trackfw-myplugin")
        with open(plugin_file, "w") as f:
            f.write("#!/bin/sh\necho hello\n")
        os.chmod(plugin_file, 0o755)

        captured = io.StringIO()
        with patch("sys.stdout", captured), \
             patch.dict(os.environ, {"PATH": self.tmpdir}):

            class FakeArgs:
                plugins_command = "list"

            plugins_mod._dispatch(FakeArgs())

        output = captured.getvalue()
        self.assertIn("trackfw-myplugin", output)

    def test_find_plugins_in_path_sem_executaveis(self):
        """_find_plugins_in_path retorna [] quando PATH não tem trackfw-*."""
        from trackfw.commands import plugins as plugins_mod

        empty_dir = os.path.join(self.tmpdir, "bin")
        os.makedirs(empty_dir, exist_ok=True)

        with patch.dict(os.environ, {"PATH": empty_dir}):
            result = plugins_mod._find_plugins_in_path()

        self.assertEqual(result, [])

    def test_find_plugins_in_path_com_executavel(self):
        """_find_plugins_in_path detecta executável trackfw-* no PATH."""
        from trackfw.commands import plugins as plugins_mod

        plugin = os.path.join(self.tmpdir, "trackfw-demo")
        with open(plugin, "w") as f:
            f.write("#!/bin/sh\n")
        os.chmod(plugin, 0o755)

        with patch.dict(os.environ, {"PATH": self.tmpdir}):
            result = plugins_mod._find_plugins_in_path()

        self.assertIn("trackfw-demo", result)

    def test_plugins_run_nao_encontrado(self):
        """plugins run <inexistente> → sys.exit(1)."""
        from trackfw.commands import plugins as plugins_mod

        empty_dir = os.path.join(self.tmpdir, "empty_bin")
        os.makedirs(empty_dir, exist_ok=True)

        with patch.dict(os.environ, {"PATH": empty_dir}), \
             self.assertRaises(SystemExit) as cm:

            class FakeArgs:
                plugins_command = "run"
                name = "inexistente"
                plugin_args = []

            plugins_mod._dispatch(FakeArgs())

        self.assertEqual(cm.exception.code, 1)


if __name__ == "__main__":
    unittest.main()
