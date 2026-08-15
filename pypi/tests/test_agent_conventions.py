"""
test_agent_conventions.py — Testes de unidade para o campo `agent_conventions` (ML-2B, port
Python de ROADMAP-2026-08-15-agentes-especialistas-aceitam-contexto-de-convencoes-especifico-
do-projeto.md, Wave 2).

Cobre:
  - config.defaults()/config._parse(): agent_conventions ausente/presente/multi-linha.
  - config.read_agent_conventions(cwd): arquivo ausente/presente/malformado.
  - init_gen._trackfw_rules_block(): seção "### Project Conventions" ausente quando vazio
    (byte-idêntico ao comportamento pré-ML) e presente quando há conteúdo.
  - init_gen.inject_rules_for_tool(): injeta o texto declarado em trackfw.yaml no arquivo do
    agente.
  - commands.discover.detect_test_framework(): cada arquivo-gatilho (jest, vitest, pytest via
    pytest.ini/pyproject.toml/setup.cfg, go test) e o caso "nenhum match".
  - commands.discover._cmd_discover(): linha de sugestão impressa apenas quando há match, e
    `--init` nunca escreve `agent_conventions` automaticamente em trackfw.yaml.
"""

import argparse
import os
import sys
import tempfile
import shutil
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config
from trackfw.generators import init_gen
from trackfw.commands import discover as discover_cmd


# ---------------------------------------------------------------------------
# config.py — defaults / _parse / read_agent_conventions
# ---------------------------------------------------------------------------

class TestConfigAgentConventions(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_default_is_empty_string(self):
        d = config.defaults()
        self.assertEqual(d["update"]["agent_conventions"], "")

    def test_parse_absent_key_keeps_default(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("hooks: husky\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["update"]["agent_conventions"], "")

    def test_parse_present_single_line(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("agent_conventions: Use pytest, not unittest.\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["update"]["agent_conventions"], "Use pytest, not unittest.")

    def test_parse_present_multiline_block_scalar(self):
        yaml_content = (
            "agent_conventions: |\n"
            "  Use pytest, not unittest.\n"
            "  API REST, no GraphQL.\n"
        )
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write(yaml_content)
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(
            cfg["update"]["agent_conventions"],
            "Use pytest, not unittest.\nAPI REST, no GraphQL.\n",
        )

    def test_read_agent_conventions_file_absent(self):
        self.assertEqual(config.read_agent_conventions(self.tmpdir), "")

    def test_read_agent_conventions_key_absent(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("hooks: husky\n")
        self.assertEqual(config.read_agent_conventions(self.tmpdir), "")

    def test_read_agent_conventions_present(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("agent_conventions: Use pytest, not unittest.\n")
        self.assertEqual(
            config.read_agent_conventions(self.tmpdir),
            "Use pytest, not unittest.",
        )

    def test_read_agent_conventions_malformed_yaml_never_raises(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("agent_conventions: [unterminated\n")
        # Nunca deve lançar exceção — best-effort, retorna "" em qualquer falha.
        self.assertEqual(config.read_agent_conventions(self.tmpdir), "")

    def test_read_agent_conventions_does_not_use_singleton(self):
        # bypassa o singleton load() — não deve ser afetado por uma chamada anterior a load()
        # com outro cwd.
        other_dir = tempfile.mkdtemp()
        try:
            with open(os.path.join(other_dir, "trackfw.yaml"), "w", encoding="utf-8") as f:
                f.write("agent_conventions: from-other-dir\n")
            config.load(cwd=other_dir)  # popula o singleton com other_dir

            with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
                f.write("agent_conventions: from-tmpdir\n")

            self.assertEqual(config.read_agent_conventions(self.tmpdir), "from-tmpdir")
        finally:
            shutil.rmtree(other_dir, ignore_errors=True)


# ---------------------------------------------------------------------------
# init_gen.py — _trackfw_rules_block / inject_rules_for_tool
# ---------------------------------------------------------------------------

class TestRulesBlockAgentConventions(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_block_empty_has_no_conventions_section(self):
        block = init_gen._trackfw_rules_block("")
        self.assertNotIn("### Project Conventions", block)

    def test_block_empty_is_byte_identical_to_pre_ml_behavior(self):
        # Antes deste ML, _trackfw_rules_block() não tinha o parâmetro — chamando com "" (o
        # default) deve reproduzir exatamente o texto anterior: "...first REQ\n\n### Key
        # Commands\n...".
        block = init_gen._trackfw_rules_block()
        self.assertIn(
            "- Use `/trackfw:architect` to define stack before the first REQ\n\n### Key Commands\n",
            block,
        )

    def test_block_with_content_has_conventions_section(self):
        block = init_gen._trackfw_rules_block("Use pytest, not unittest.\nAPI REST, no GraphQL.")
        self.assertIn("### Project Conventions", block)
        self.assertIn(
            "> Declared by the team in `trackfw.yaml`'s `agent_conventions` field — NOT",
            block,
        )
        self.assertIn("Use pytest, not unittest.\nAPI REST, no GraphQL.", block)

    def test_block_whitespace_only_treated_as_empty(self):
        block = init_gen._trackfw_rules_block("   \n  \n")
        self.assertNotIn("### Project Conventions", block)

    def test_inject_rules_for_tool_reads_from_cwd_trackfw_yaml(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("agent_conventions: |\n  Use pytest, not unittest.\n  API REST, no GraphQL.\n")

        init_gen.inject_rules_for_tool("claude", self.tmpdir)

        claude_md = os.path.join(self.tmpdir, "CLAUDE.md")
        self.assertTrue(os.path.exists(claude_md))
        content = open(claude_md, encoding="utf-8").read()
        self.assertIn("### Project Conventions", content)
        self.assertIn("Use pytest, not unittest.", content)
        self.assertIn("API REST, no GraphQL.", content)

    def test_inject_rules_for_tool_no_trackfw_yaml_no_regression(self):
        # Sem trackfw.yaml no cwd: comportamento idêntico ao pré-ML — sem a seção.
        init_gen.inject_rules_for_tool("claude", self.tmpdir)

        claude_md = os.path.join(self.tmpdir, "CLAUDE.md")
        content = open(claude_md, encoding="utf-8").read()
        self.assertNotIn("### Project Conventions", content)


# ---------------------------------------------------------------------------
# commands/discover.py — detect_test_framework
# ---------------------------------------------------------------------------

class TestDetectTestFramework(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _touch(self, rel_path, content=""):
        full = os.path.join(self.tmpdir, rel_path)
        os.makedirs(os.path.dirname(full) or self.tmpdir, exist_ok=True)
        with open(full, "w", encoding="utf-8") as f:
            f.write(content)

    def test_no_trigger_file_returns_empty(self):
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "")

    def test_jest_config_js(self):
        self._touch("jest.config.js")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "jest")

    def test_jest_config_ts(self):
        self._touch("jest.config.ts")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "jest")

    def test_vitest_config_js(self):
        self._touch("vitest.config.js")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "vitest")

    def test_vitest_config_ts(self):
        self._touch("vitest.config.ts")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "vitest")

    def test_pytest_ini(self):
        self._touch("pytest.ini")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "pytest")

    def test_pyproject_toml_with_pytest_section(self):
        self._touch("pyproject.toml", "[tool.pytest.ini_options]\naddopts = '-ra'\n")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "pytest")

    def test_pyproject_toml_without_pytest_section_is_no_match(self):
        self._touch("pyproject.toml", "[tool.black]\nline-length = 88\n")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "")

    def test_setup_cfg_with_pytest_section(self):
        self._touch("setup.cfg", "[tool:pytest]\ntestpaths = tests\n")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "pytest")

    def test_go_mod_and_test_file(self):
        self._touch("go.mod", "module example.com/x\n")
        self._touch(os.path.join("internal", "foo_test.go"), "package internal\n")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "go test")

    def test_go_mod_without_test_file_is_no_match(self):
        self._touch("go.mod", "module example.com/x\n")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "")

    def test_precedence_jest_over_pytest(self):
        self._touch("jest.config.js")
        self._touch("pytest.ini")
        self.assertEqual(discover_cmd.detect_test_framework(self.tmpdir), "jest")


# ---------------------------------------------------------------------------
# commands/discover.py — _cmd_discover: suggestion line + --init never writes
# agent_conventions automatically
# ---------------------------------------------------------------------------

class TestDiscoverSuggestionLine(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_suggestion_line_present_with_jest_config(self, ):
        with open(os.path.join(self.tmpdir, "jest.config.js"), "w", encoding="utf-8") as f:
            f.write("")

        cwd = os.getcwd()
        os.chdir(self.tmpdir)
        try:
            import io
            import contextlib
            buf = io.StringIO()
            with contextlib.redirect_stdout(buf):
                discover_cmd._cmd_discover(argparse.Namespace(init=False, bootstrap_log=False))
            out = buf.getvalue()
        finally:
            os.chdir(cwd)

        self.assertIn(
            "Suggested test framework: jest (add to trackfw.yaml as agent_conventions: if correct)",
            out,
        )

    def test_suggestion_line_absent_without_trigger_file(self):
        cwd = os.getcwd()
        os.chdir(self.tmpdir)
        try:
            import io
            import contextlib
            buf = io.StringIO()
            with contextlib.redirect_stdout(buf):
                discover_cmd._cmd_discover(argparse.Namespace(init=False, bootstrap_log=False))
            out = buf.getvalue()
        finally:
            os.chdir(cwd)

        self.assertNotIn("Suggested test framework", out)

    def test_init_never_writes_agent_conventions_even_with_suggestion(self):
        with open(os.path.join(self.tmpdir, "jest.config.js"), "w", encoding="utf-8") as f:
            f.write("")

        cwd = os.getcwd()
        os.chdir(self.tmpdir)
        try:
            discover_cmd._cmd_discover(argparse.Namespace(init=True, bootstrap_log=False))
        finally:
            os.chdir(cwd)

        yaml_path = os.path.join(self.tmpdir, "trackfw.yaml")
        self.assertTrue(os.path.exists(yaml_path))
        content = open(yaml_path, encoding="utf-8").read()
        self.assertNotIn("agent_conventions", content)


if __name__ == "__main__":
    unittest.main()
