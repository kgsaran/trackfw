"""
tests/test_commands_basic.py — Testes de integração básica dos comandos CLI Python.
Usa subprocess.run para chamar o módulo trackfw diretamente.
"""

import os
import subprocess
import sys
import tempfile
import unittest

# Diretório raiz do pypi (onde o pacote trackfw está instalado em modo editable)
PYPI_DIR = os.path.join(os.path.dirname(__file__), "..")
PYPI_DIR = os.path.abspath(PYPI_DIR)


def run_trackfw(*args, cwd=None, env=None):
    """Executa `python3 -m trackfw <args>` e retorna o resultado."""
    cmd = [sys.executable, "-m", "trackfw"] + list(args)

    # Garante que o módulo trackfw seja encontrado mesmo quando cwd é um tmpdir
    run_env = dict(os.environ)
    existing = run_env.get("PYTHONPATH", "")
    run_env["PYTHONPATH"] = PYPI_DIR + (os.pathsep + existing if existing else "")
    if env:
        run_env.update(env)

    result = subprocess.run(
        cmd,
        cwd=cwd or PYPI_DIR,
        capture_output=True,
        text=True,
        env=run_env,
    )
    return result


class TestVersion(unittest.TestCase):
    def test_version(self):
        """trackfw --version retorna código 0 e imprime a versão."""
        result = run_trackfw("--version")
        self.assertEqual(result.returncode, 0)
        # argparse imprime versão em stdout (Python 3.9+) ou stderr (versões anteriores)
        combined = result.stdout + result.stderr
        self.assertIn("trackfw", combined)
        # Verifica que há uma versão no formato X.Y.Z
        import re
        self.assertRegex(combined, r"\d+\.\d+\.\d+")


class TestAdrNew(unittest.TestCase):
    def test_adr_new_cria_arquivo(self):
        """trackfw adr new 'Minha Decisão' cria arquivo ADR em dir temporário."""
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("adr", "new", "Minha Decisao", cwd=tmpdir)
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            # Deve imprimir o path do arquivo criado
            self.assertIn("created", result.stdout)
            # Arquivo deve existir
            adr_dir = os.path.join(tmpdir, "docs", "adr")
            self.assertTrue(os.path.isdir(adr_dir), f"docs/adr não criado em {tmpdir}")
            files = os.listdir(adr_dir)
            self.assertEqual(len(files), 1, f"Esperava 1 arquivo, encontrei: {files}")
            self.assertTrue(files[0].endswith(".md"))
            # Nome canônico: ADR-YYYY-MM-DD-<slug>.md
            import re
            self.assertRegex(files[0], r'^ADR-\d{4}-\d{2}-\d{2}-.*\.md$')

    def test_adr_new_com_status(self):
        """trackfw adr new com --status Accepted cria arquivo com status correto."""
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw(
                "adr", "new", "Status Test", "--status", "Accepted", cwd=tmpdir
            )
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            adr_dir = os.path.join(tmpdir, "docs", "adr")
            files = os.listdir(adr_dir)
            filepath = os.path.join(adr_dir, files[0])
            with open(filepath, encoding="utf-8") as f:
                content = f.read()
            self.assertIn("Accepted", content)

    def test_adr_new_com_dir(self):
        """trackfw adr new --dir caminho-customizado cria no diretório especificado."""
        with tempfile.TemporaryDirectory() as tmpdir:
            custom_dir = os.path.join(tmpdir, "custom-adrs")
            result = run_trackfw(
                "adr", "new", "Custom Dir ADR", "--dir", custom_dir, cwd=tmpdir
            )
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            self.assertTrue(os.path.isdir(custom_dir))
            files = os.listdir(custom_dir)
            self.assertEqual(len(files), 1)


class TestLog(unittest.TestCase):
    def test_log_le_roadmap_dir_configurado(self):
        """trackfw log lê .trackfw-log em roadmap_dir."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_dir = os.path.join(tmpdir, "custom", "roadmaps")
            os.makedirs(log_dir)
            with open(os.path.join(tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
                f.write("roadmap_dir: custom/roadmaps\n")
            with open(os.path.join(log_dir, ".trackfw-log"), "w", encoding="utf-8") as f:
                f.write("2026-07-27 10:00  RM.md  wip -> done\n")

            result = run_trackfw("log", "--tail", "1", cwd=tmpdir)
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            self.assertIn("RM.md", result.stdout)

    def test_log_tail_limita_saida(self):
        """trackfw log --tail mostra apenas as últimas linhas."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_dir = os.path.join(tmpdir, "docs", "roadmaps")
            os.makedirs(log_dir)
            with open(os.path.join(log_dir, ".trackfw-log"), "w", encoding="utf-8") as f:
                f.write("2026-07-27 10:00  RM-1.md  backlog -> wip\n")
                f.write("2026-07-27 11:00  RM-2.md  wip -> done\n")

            result = run_trackfw("log", "--tail", "1", cwd=tmpdir)
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            self.assertNotIn("RM-1.md", result.stdout)
            self.assertIn("RM-2.md", result.stdout)

    def test_log_vazio_quando_arquivo_ausente(self):
        """Sem .trackfw-log em roadmap_dir, comando retorna sucesso com mensagem vazia."""
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("log", cwd=tmpdir)
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            self.assertIn("No transition log found", result.stdout)


class TestRealCommands(unittest.TestCase):
    def test_validate_uses_real_handler(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("validate", "--json", cwd=tmpdir)
        self.assertEqual(result.returncode, 0)
        self.assertIn('"summary"', result.stdout)
        self.assertNotIn("Not implemented yet", result.stdout)

    def test_status_uses_real_handler(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("status", cwd=tmpdir)
        self.assertEqual(result.returncode, 0)
        self.assertIn("Governance Status", result.stdout)

    def test_metrics_uses_real_handler(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("metrics", cwd=tmpdir)
        self.assertEqual(result.returncode, 0)
        self.assertIn("No log found", result.stdout)

    def test_context_uses_real_handler(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("context", "--format", "json", cwd=tmpdir)
        self.assertEqual(result.returncode, 0)
        self.assertIn('"score"', result.stdout)

    def test_roadmap_help_uses_real_handler(self):
        result = run_trackfw("roadmap", "--help")
        self.assertEqual(result.returncode, 0)
        self.assertIn("new", result.stdout)
        self.assertIn("move", result.stdout)

    def test_init_scaffolds_project(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw(
                "init",
                "--project-name",
                "example",
                "--namespacing",
                "flat",
                cwd=tmpdir,
            )
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            self.assertTrue(os.path.isfile(os.path.join(tmpdir, "trackfw.yaml")))
            self.assertTrue(os.path.isdir(os.path.join(tmpdir, "docs", "roadmaps", "wip")))


if __name__ == "__main__":
    unittest.main()
