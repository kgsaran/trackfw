"""
Testes de unidade para o script trackfw-credential-guard.sh (gerador) e o campo de config
credential_guard.mode — ML-1A de
ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md.
"""

import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config
from trackfw.generators.init_gen import _generate_credential_guard_script

SYNTHETIC_JWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.abc123def456ghi789"


class TestCredentialGuardGenerator(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_gera_script_executavel(self):
        _generate_credential_guard_script(self.tmpdir)
        script_path = os.path.join(self.tmpdir, "scripts", "trackfw-credential-guard.sh")
        self.assertTrue(os.path.exists(script_path))
        mode = os.stat(script_path).st_mode
        self.assertTrue(mode & stat.S_IXUSR, "script deveria ser executável")
        with open(script_path, "r", encoding="utf-8") as f:
            content = f.read()
        self.assertTrue(content.startswith("#!/usr/bin/env bash"))

    def test_nao_cria_nenhum_hooks_json_de_cli(self):
        # ML-1A não injeta o script em nenhum hooks.json/settings.json de CLI — escopo da Wave 2.
        _generate_credential_guard_script(self.tmpdir)
        for p in [
            ".claude/settings.json",
            ".codex/hooks.json",
            ".gemini/settings.json",
            ".github/hooks/hooks.json",
            ".cursor/hooks.json",
            ".kiro/hooks/trackfw-attention.json",
        ]:
            self.assertFalse(
                os.path.exists(os.path.join(self.tmpdir, p)),
                f"ML-1A não deveria criar {p} (escopo da Wave 2)",
            )


class TestCredentialGuardConfig(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_default_e_warn_quando_ausente(self):
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["credential_guard"]["mode"], "warn")

    def test_mode_block_e_lido_corretamente(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("credential_guard:\n  mode: block\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["credential_guard"]["mode"], "block")

    def test_valor_invalido_cai_para_warn(self):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("credential_guard:\n  mode: nonsense\n")
        cfg = config.load(cwd=self.tmpdir)
        self.assertEqual(cfg["credential_guard"]["mode"], "warn")


class TestCredentialGuardScriptBehavior(unittest.TestCase):
    """Invoca o script real como subprocesso — não reimplementa a regex em paralelo."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        _generate_credential_guard_script(self.tmpdir)
        self.script_path = os.path.join(self.tmpdir, "scripts", "trackfw-credential-guard.sh")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _write_yaml(self, content="roadmap_dir: docs/roadmaps\n"):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write(content)

    def _run(self, payload):
        proc = subprocess.run(
            ["bash", self.script_path],
            cwd=self.tmpdir,
            input=json.dumps(payload),
            capture_output=True,
            text=True,
        )
        return proc.returncode, proc.stdout, proc.stderr

    def _attention_exists(self):
        return os.path.exists(os.path.join(self.tmpdir, "docs", "roadmaps", ".trackfw-attention.json"))

    def test_sem_match_e_no_op_silencioso(self):
        self._write_yaml()
        code, _out, _err = self._run({"tool_name": "Bash", "tool_input": {"command": "echo hello"}})
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())

    def test_jwt_no_stdout_avisa_por_padrao(self):
        self._write_yaml()
        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 0)
        self.assertIn("JWT", err)
        self.assertTrue(self._attention_exists())

    def test_aws_key_detectada(self):
        self._write_yaml()
        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": "echo AKIAABCDEFGHIJKLMNOP"}})
        self.assertEqual(code, 0)
        self.assertIn("AWS", err)
        self.assertTrue(self._attention_exists())

    def test_redirecionado_para_dev_null_e_efemero_sem_alerta(self):
        self._write_yaml()
        code, _out, _err = self._run(
            {"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT} > /dev/null"}}
        )
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())

    def test_redirecionado_para_mktemp_direto_e_efemero_sem_alerta(self):
        self._write_yaml()
        code, _out, _err = self._run(
            {"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT} > $(mktemp)"}}
        )
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())

    def test_redirecionado_para_variavel_mktemp_e_efemero_sem_alerta(self):
        self._write_yaml()
        cmd = f'TMPFILE=$(mktemp); echo {SYNTHETIC_JWT} > "$TMPFILE"'
        code, _out, _err = self._run({"tool_name": "Bash", "tool_input": {"command": cmd}})
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())

    def test_redirecionado_para_arquivo_comum_nao_e_efemero_alerta(self):
        # Caso do incidente real da REQ: token gravado em arquivo solto, não efêmero.
        self._write_yaml()
        code, _out, _err = self._run(
            {"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT} > /tmp/token.txt"}}
        )
        self.assertEqual(code, 0)
        self.assertTrue(self._attention_exists())

    def test_modo_block_sai_com_codigo_2(self):
        self._write_yaml("credential_guard:\n  mode: block\n")
        code, _out, _err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 2)
        self.assertFalse(self._attention_exists())

    def test_valor_de_mode_invalido_cai_para_warn(self):
        self._write_yaml("credential_guard:\n  mode: nonsense\n")
        code, _out, _err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 0)
        self.assertTrue(self._attention_exists())

    def test_no_op_fora_da_raiz_do_projeto(self):
        os.remove(os.path.join(self.tmpdir, "trackfw.yaml")) if os.path.exists(
            os.path.join(self.tmpdir, "trackfw.yaml")
        ) else None
        code, _out, _err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())


if __name__ == "__main__":
    unittest.main()
