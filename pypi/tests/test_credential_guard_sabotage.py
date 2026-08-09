"""
ML-4A -- teste de sabotagem end-to-end de
ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md.

Diferente de test_credential_guard.py (que já invoca o script real como subprocesso, mas com um
payload JSON genérico escrito à mão), este arquivo:

  1. Gera o wiring REAL de cada CLI via inject_*_hooks (o mesmo gerador exercitado por
     test_generators_init.py), confirmando que o hooks.json/settings.json resultante de fato
     referencia "scripts/trackfw-credential-guard.sh".
  2. Constrói o payload JSON EXATO que aquele CLI envia via stdin ao hook, conforme o schema
     documentado em docs/cli-parity.md por CLI.
  3. Materializa um JWT sintético (nunca hardcoded como token real plausível) dentro do payload.
  4. Invoca o script gerado (não uma cópia, não uma reimplementação da regex) como subprocesso,
     passando o payload por stdin.
  5. Confirma detecção nos dois modos (warn/block) e prova negativa.

Cobertura por CLI -- ver a nota equivalente em internal/generators/credential_guard_sabotage_test.go
(Go) para o detalhe completo do motivo de Codex/Gemini/Copilot ficarem de fora: schema de payload de
stdin não confirmado com confiança suficiente em docs/cli-parity.md (só o formato do arquivo de
configuração hooks.json é confirmado para esses três, não o payload de runtime).
  - Claude Code: COBERTO (obrigatório pelo AC da REQ).
  - Cursor: COBERTO.
  - Kiro: COBERTO.
  - Codex, Gemini CLI, GitHub Copilot: SEM teste de sabotagem end-to-end.
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.generators.init_gen import _generate_credential_guard_script
from trackfw.generators.hooks import inject_claude_hooks, inject_cursor_hooks, inject_kiro_hooks

SYNTHETIC_JWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.abc123def456ghi789"


class SabotageFixtureMixin:

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        # Isolate the global credential-guard dedup check (ML-3A) from the real $HOME.
        self._orig_home = os.environ.get("HOME")
        os.environ["HOME"] = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        if self._orig_home is None:
            os.environ.pop("HOME", None)
        else:
            os.environ["HOME"] = self._orig_home

    def _setup(self, inject_hooks, yaml_content="roadmap_dir: docs/roadmaps\n"):
        _generate_credential_guard_script(self.tmpdir)
        inject_hooks(self.tmpdir)
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write(yaml_content)
        return os.path.join(self.tmpdir, "scripts", "trackfw-credential-guard.sh")

    def _run(self, script_path, payload):
        proc = subprocess.run(
            ["bash", script_path],
            cwd=self.tmpdir,
            input=json.dumps(payload),
            capture_output=True,
            text=True,
        )
        return proc.returncode, proc.stdout, proc.stderr

    def _attention_exists(self):
        return os.path.exists(os.path.join(self.tmpdir, "docs", "roadmaps", ".trackfw-credential-guard.json"))

    def _read_json(self, *parts):
        with open(os.path.join(self.tmpdir, *parts), "r", encoding="utf-8") as f:
            return json.load(f)


# ---------------------------------------------------------------------------
# Claude Code -- PreToolUse/PostToolUse, matcher "Bash".
# Schema confirmado: {"tool_name":"Bash","tool_input":{"command":"..."}}
# ---------------------------------------------------------------------------

class TestSabotageClaudeCode(SabotageFixtureMixin, unittest.TestCase):

    def test_wiring_referencia_script_real(self):
        self._setup(inject_claude_hooks)
        data = self._read_json(".claude", "settings.json")
        pre_entries = data["hooks"]["PreToolUse"]
        bash_entry = next((e for e in pre_entries if e.get("matcher") == "Bash"), None)
        self.assertIsNotNone(bash_entry, "PreToolUse[Bash] ausente")
        commands = [h["command"] for h in bash_entry["hooks"]]
        self.assertIn("$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh", commands)

    def test_jwt_no_comando_bash_modo_warn(self):
        script_path = self._setup(inject_claude_hooks)
        payload = {"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}}
        code, _out, err = self._run(script_path, payload)
        self.assertEqual(code, 0, err)
        self.assertTrue(self._attention_exists())

    def test_jwt_no_comando_bash_modo_block(self):
        script_path = self._setup(inject_claude_hooks, "credential_guard:\n  mode: block\n")
        payload = {"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}}
        code, _out, _err = self._run(script_path, payload)
        self.assertEqual(code, 2)

    def test_prova_negativa_sem_jwt(self):
        script_path = self._setup(inject_claude_hooks)
        payload = {"tool_name": "Bash", "tool_input": {"command": "git status"}}
        code, _out, _err = self._run(script_path, payload)
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists(), "prova negativa falhou: teste não pode ser vácuo/sempre-verde")


# ---------------------------------------------------------------------------
# Cursor -- beforeShellExecution/afterShellExecution.
# Schema confirmado (docs/cli-parity.md, "Cursor wiring (ML-2E)"):
# {"command":"...","cwd":"...","sandbox":false}
# ---------------------------------------------------------------------------

class TestSabotageCursor(SabotageFixtureMixin, unittest.TestCase):

    def test_wiring_referencia_script_real(self):
        self._setup(inject_cursor_hooks)
        data = self._read_json(".cursor", "hooks.json")
        commands = [e["command"] for e in data["hooks"]["beforeShellExecution"]]
        self.assertIn("scripts/trackfw-credential-guard.sh", commands)

    def test_jwt_no_comando_de_shell_modo_warn(self):
        script_path = self._setup(inject_cursor_hooks)
        payload = {"command": f"echo {SYNTHETIC_JWT}", "cwd": "/tmp/fixture", "sandbox": False}
        code, _out, err = self._run(script_path, payload)
        self.assertEqual(code, 0, err)
        self.assertTrue(self._attention_exists())

    def test_jwt_no_comando_de_shell_modo_block(self):
        script_path = self._setup(inject_cursor_hooks, "credential_guard:\n  mode: block\n")
        payload = {"command": f"echo {SYNTHETIC_JWT}", "cwd": "/tmp/fixture", "sandbox": False}
        code, _out, _err = self._run(script_path, payload)
        self.assertEqual(code, 2)

    def test_prova_negativa_sem_jwt(self):
        script_path = self._setup(inject_cursor_hooks)
        payload = {"command": "git status", "cwd": "/tmp/fixture", "sandbox": False}
        code, _out, _err = self._run(script_path, payload)
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())

    def test_jwt_apenas_na_saida_capturada_after_shell_execution(self):
        script_path = self._setup(inject_cursor_hooks)
        payload = {
            "command": "curl https://internal.example/token",
            "cwd": "/tmp/fixture",
            "sandbox": False,
            "output": f"token={SYNTHETIC_JWT}",
            "duration": 123,
        }
        code, _out, err = self._run(script_path, payload)
        self.assertEqual(code, 0, err)
        self.assertTrue(self._attention_exists(), "JWT no campo output do afterShellExecution deveria ser detectado")


# ---------------------------------------------------------------------------
# Kiro -- PreToolUse/PostToolUse, matcher "shell".
# Schema confirmado (docs/cli-parity.md, "Kiro wiring (ML-2F)"):
# {"hook_event_name","cwd","session_id","tool_name","tool_input"}
# ---------------------------------------------------------------------------

class TestSabotageKiro(SabotageFixtureMixin, unittest.TestCase):

    def test_wiring_referencia_script_real(self):
        self._setup(inject_kiro_hooks)
        data = self._read_json(".kiro", "hooks", "trackfw-attention.json")
        guard_entries = [
            h for h in data["hooks"]
            if h.get("action", {}).get("command") == "scripts/trackfw-credential-guard.sh"
        ]
        triggers = {h["trigger"] for h in guard_entries}
        self.assertIn("PreToolUse", triggers)
        self.assertIn("PostToolUse", triggers)

    def _payload(self, command):
        return {
            "hook_event_name": "PreToolUse",
            "cwd": self.tmpdir,
            "session_id": "sess-sabotage-test",
            "tool_name": "execute_bash",
            "tool_input": {"command": command},
        }

    def test_jwt_em_tool_input_modo_warn(self):
        script_path = self._setup(inject_kiro_hooks)
        code, _out, err = self._run(script_path, self._payload(f"echo {SYNTHETIC_JWT}"))
        self.assertEqual(code, 0, err)
        self.assertTrue(self._attention_exists())

    def test_jwt_em_tool_input_modo_block(self):
        script_path = self._setup(inject_kiro_hooks, "credential_guard:\n  mode: block\n")
        code, _out, _err = self._run(script_path, self._payload(f"echo {SYNTHETIC_JWT}"))
        self.assertEqual(code, 2)

    def test_prova_negativa_sem_jwt(self):
        script_path = self._setup(inject_kiro_hooks)
        code, _out, _err = self._run(script_path, self._payload("git status"))
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())


if __name__ == "__main__":
    unittest.main()
