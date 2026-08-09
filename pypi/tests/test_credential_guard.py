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
from trackfw.generators.init_gen import (
    _generate_credential_guard_script,
    _generate_attention_scripts,
    generate_global_credential_guard_script,
)

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
        return os.path.exists(os.path.join(self.tmpdir, "docs", "roadmaps", ".trackfw-credential-guard.json"))

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

    def test_payload_write_com_jwt_no_content_e_capturado(self):
        # ML-3C cenário (b): payload de tool call Write/Edit com JWT no campo de conteúdo
        # (materialização em disco), não numa string "command" -- o script escaneia o INPUT
        # bruto inteiro antes de procurar por "command", então já captura aqui; este teste prova
        # isso a nível de comportamento do script, complementando os testes de wiring de matcher
        # em test_generators_init.py (inject_claude_hooks etc.), que só verificam que a entrada
        # foi registrada no hooks.json, sem invocar o script.
        self._write_yaml()
        payload = {
            "tool_name": "Write",
            "tool_input": {"file_path": "/tmp/out.txt", "content": f"token={SYNTHETIC_JWT}"},
        }
        code, _out, err = self._run(payload)
        self.assertEqual(code, 0)
        self.assertIn("JWT", err)
        self.assertTrue(self._attention_exists())

    def test_payload_read_post_com_aws_key_no_tool_response_e_capturado(self):
        # Mesmo cenário (b), variante Read/PostToolUse: o segredo aparece no tool_response (saída
        # da leitura), não no tool_input -- também capturado pelo scan do INPUT bruto.
        self._write_yaml()
        payload = {
            "tool_name": "Read",
            "tool_input": {"file_path": "/tmp/secret.txt"},
            "tool_response": {"content": "AKIAABCDEFGHIJKLMNOP"},
        }
        code, _out, err = self._run(payload)
        self.assertEqual(code, 0)
        self.assertIn("AWS", err)
        self.assertTrue(self._attention_exists())

    def test_segunda_camada_detecta_jwt_via_cat_de_arquivo_referenciado(self):
        # ML-3C cenário (c): comando Bash que referencia um arquivo com segredo por caminho, sem
        # o segredo literal no comando -- padrão do incidente real da REQ (`cat arquivo`).
        self._write_yaml()
        fixture_path = os.path.join(self.tmpdir, "fixture-com-jwt.txt")
        with open(fixture_path, "w", encoding="utf-8") as f:
            f.write(f"Authorization: Bearer {SYNTHETIC_JWT}\n")
        code, _out, err = self._run(
            {"tool_name": "Bash", "tool_input": {"command": f"cat {fixture_path}"}}
        )
        self.assertEqual(code, 0)
        self.assertIn("JWT", err)
        self.assertTrue(self._attention_exists())

    def test_segunda_camada_detecta_aws_key_via_head_de_arquivo_referenciado(self):
        # Mesmo cenário (c), variante `head -c 50 arquivo` (com flag antes do caminho) e AWS key.
        self._write_yaml()
        fixture_path = os.path.join(self.tmpdir, "arquivo-com-aws-key.txt")
        with open(fixture_path, "w", encoding="utf-8") as f:
            f.write("AWS_ACCESS_KEY_ID=AKIAABCDEFGHIJKLMNOP\n")
        code, _out, err = self._run(
            {"tool_name": "Bash", "tool_input": {"command": f"head -c 50 {fixture_path}"}}
        )
        self.assertEqual(code, 0)
        self.assertIn("AWS", err)
        self.assertTrue(self._attention_exists())

    def test_segunda_camada_nao_dispara_para_comando_fora_da_allowlist(self):
        # Controle negativo: `ls` não está na allowlist (cat/head/tail/jq/grep) da segunda
        # camada -- referenciar o mesmo arquivo com segredo não deveria disparar o scan.
        self._write_yaml()
        fixture_path = os.path.join(self.tmpdir, "fixture-nao-escaneada.txt")
        with open(fixture_path, "w", encoding="utf-8") as f:
            f.write(f"{SYNTHETIC_JWT}\n")
        code, _out, _err = self._run(
            {"tool_name": "Bash", "tool_input": {"command": f"ls {fixture_path}"}}
        )
        self.assertEqual(code, 0)
        self.assertFalse(self._attention_exists())

    def test_cleanup_de_attention_signal_nao_apaga_arquivo_dedicado(self):
        # trackfw-attention-cleanup.sh apaga incondicionalmente $ROADMAP_DIR/.trackfw-attention.json --
        # em harnesses que rodam hooks do mesmo evento concorrentemente (ex.: Codex CLI, PostToolUse
        # com matchers ".*" e "Bash" ambos batendo em uma chamada Bash), isso podia apagar o aviso do
        # credential-guard antes de este ser lido. O credential-guard agora usa um arquivo dedicado
        # (.trackfw-credential-guard.json), então o cleanup não deve mais afetá-lo.
        self._write_yaml()
        _generate_attention_scripts(self.tmpdir)

        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 0, err)
        self.assertTrue(self._attention_exists())

        cleanup_path = os.path.join(self.tmpdir, "scripts", "trackfw-attention-cleanup.sh")
        proc = subprocess.run(["bash", cleanup_path], cwd=self.tmpdir, capture_output=True, text=True)
        self.assertEqual(proc.returncode, 0, proc.stderr)

        self.assertTrue(
            self._attention_exists(),
            ".trackfw-credential-guard.json não deveria ter sido apagado pelo cleanup",
        )


class TestGlobalCredentialGuardGenerator(unittest.TestCase):
    """Escopo global (~/.trackfw/scripts/), ML-1A do roadmap ROADMAP-2026-08-06-hooks-de-
    credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md. Usa SEMPRE
    um HOME de fixture (tempfile.mkdtemp) -- nunca o HOME real de quem roda a suite.
    """

    def setUp(self):
        self.fake_home = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.fake_home, ignore_errors=True)

    def test_gera_script_executavel_sem_guarda_de_projeto(self):
        generate_global_credential_guard_script(self.fake_home)
        script_path = os.path.join(self.fake_home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
        self.assertTrue(os.path.exists(script_path))
        mode = os.stat(script_path).st_mode
        self.assertTrue(mode & stat.S_IXUSR, "script global deveria ser executável")
        with open(script_path, "r", encoding="utf-8") as f:
            content = f.read()
        self.assertTrue(content.startswith("#!/usr/bin/env bash"))
        self.assertNotIn(
            '[ -f "trackfw.yaml" ] || exit 0',
            content,
            "script global não deve conter a guarda de projeto",
        )

    def test_home_vazio_levanta_erro(self):
        with self.assertRaises(ValueError):
            generate_global_credential_guard_script("")


class TestGlobalCredentialGuardScriptBehavior(unittest.TestCase):
    """Invoca o script global real como subprocesso, a partir de um cwd sem trackfw.yaml."""

    def setUp(self):
        self.fake_home = tempfile.mkdtemp()
        generate_global_credential_guard_script(self.fake_home)
        self.script_path = os.path.join(self.fake_home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
        self.cwd = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.fake_home, ignore_errors=True)
        shutil.rmtree(self.cwd, ignore_errors=True)

    def _run(self, payload):
        proc = subprocess.run(
            ["bash", self.script_path],
            cwd=self.cwd,
            input=json.dumps(payload),
            capture_output=True,
            text=True,
        )
        return proc.returncode, proc.stdout, proc.stderr

    def _attention_exists(self):
        return os.path.exists(os.path.join(self.cwd, "docs", "roadmaps", ".trackfw-credential-guard.json"))

    def test_detecta_jwt_fora_de_qualquer_projeto_trackfw(self):
        # Ao contrário da variante de projeto, o script global NÃO é no-op sem trackfw.yaml -- esse
        # é o propósito da mudança (proteção cross-project). Sem trackfw.yaml no cwd, o fallback de
        # modo é "block" (ADR-2026-08-06 emenda 6, ROADMAP-2026-08-08 ML-1C).
        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 2, "modo block (fallback sem trackfw.yaml)")
        self.assertIn("JWT", err)

    def test_trackfw_yaml_presente_sem_credential_guard_mode_bloqueia_tambem(self):
        # ML-3C cenário (a), variante complementar a test_detecta_jwt_fora_de_qualquer_projeto_
        # trackfw: não é só a AUSÊNCIA do arquivo trackfw.yaml que cai no fallback block -- um
        # trackfw.yaml presente mas sem a chave credential_guard.mode também deve cair no mesmo
        # fallback "block" (não em "warn").
        with open(os.path.join(self.cwd, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("roadmap_dir: docs/roadmaps\n")
        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 2, "trackfw.yaml presente mas sem credential_guard.mode -> fallback block")
        self.assertIn("blocked", err)

    def test_respeita_mode_block_explicito_no_trackfw_yaml_do_cwd(self):
        # O script global reusa a mesma leitura de credential_guard.mode de trackfw.yaml que a
        # variante de projeto já faz -- quando o cwd tem trackfw.yaml com mode explícito, esse
        # valor é respeitado.
        with open(os.path.join(self.cwd, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("credential_guard:\n  mode: block\n")
        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 2, "trackfw.yaml com mode: block explícito")
        self.assertIn("blocked", err)

    def test_respeita_mode_warn_explicito_no_trackfw_yaml_do_cwd(self):
        # Sentido contrário: mode: warn explícito continua produzindo warn (exit 0) -- não é o
        # fallback que muda de warn para block, é só o default sem chave/arquivo.
        with open(os.path.join(self.cwd, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("credential_guard:\n  mode: warn\n")
        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 0, "trackfw.yaml com mode: warn explícito")
        self.assertIn("warning", err)

    def test_attention_so_escrita_quando_docs_roadmaps_ja_existe(self):
        # attention signal só é gravado em modo warn -- usa mode: warn explícito no cwd para
        # exercitar essa checagem independente do fallback default de modo global (block).
        with open(os.path.join(self.cwd, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write("credential_guard:\n  mode: warn\n")
        code, _out, err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 0)
        self.assertIn("JWT", err)
        self.assertFalse(self._attention_exists(), "não deveria criar docs/roadmaps num projeto qualquer")

        os.makedirs(os.path.join(self.cwd, "docs", "roadmaps"), exist_ok=True)
        code, _out, _err = self._run({"tool_name": "Bash", "tool_input": {"command": f"echo {SYNTHETIC_JWT}"}})
        self.assertEqual(code, 0)
        self.assertTrue(self._attention_exists())

    def test_deteccao_identica_a_variante_de_projeto_para_aws_key(self):
        # Prova que a detecção (mesmo payload sintético) é idêntica entre projeto e global -- os
        # modos default divergem por design (projeto: warn; global sem trackfw.yaml: block), então
        # os exit codes divergem, mas ambos devem mencionar AWS.
        project_dir = tempfile.mkdtemp()
        try:
            _generate_credential_guard_script(project_dir)
            with open(os.path.join(project_dir, "trackfw.yaml"), "w", encoding="utf-8") as f:
                f.write("roadmap_dir: docs/roadmaps\n")
            project_proc = subprocess.run(
                ["bash", os.path.join(project_dir, "scripts", "trackfw-credential-guard.sh")],
                cwd=project_dir,
                input=json.dumps({"tool_name": "Bash", "tool_input": {"command": "echo AKIAABCDEFGHIJKLMNOP"}}),
                capture_output=True,
                text=True,
            )
            global_code, _out, global_err = self._run(
                {"tool_name": "Bash", "tool_input": {"command": "echo AKIAABCDEFGHIJKLMNOP"}}
            )
            self.assertEqual(project_proc.returncode, 0, "projeto (fallback warn)")
            self.assertEqual(global_code, 2, "global sem trackfw.yaml (fallback block)")
            self.assertIn("AWS", project_proc.stderr)
            self.assertIn("AWS", global_err)
        finally:
            shutil.rmtree(project_dir, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
