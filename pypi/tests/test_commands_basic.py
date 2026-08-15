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
    # Regex que pina o formato canônico do contrato de paridade:
    #   ^trackfw [0-9]+\.[0-9]+\.[0-9]+$
    # (sem prefixo 'v', sem sufixo, exatamente uma linha)
    _CANONICAL_RE = r"^trackfw [0-9]+\.[0-9]+\.[0-9]+$"

    def test_version_flag_format_exact(self):
        """--version imprime exatamente 'trackfw <semver>' em stdout, sem prefixo v."""
        result = run_trackfw("--version")
        self.assertEqual(result.returncode, 0)
        # O contrato exige stdout (não stderr) em Python 3.9+.
        output = result.stdout.strip()
        import re
        self.assertRegex(
            output,
            self._CANONICAL_RE,
            msg=(
                f"--version deve imprimir 'trackfw X.Y.Z' (sem prefixo v) em stdout; "
                f"obtido: {result.stdout!r}"
            ),
        )

    def test_version_subcommand_format_exact(self):
        """O subcomando 'version' imprime exatamente 'trackfw <semver>' em stdout, sem prefixo v."""
        result = run_trackfw("version")
        self.assertEqual(result.returncode, 0)
        output = result.stdout.strip()
        import re
        self.assertRegex(
            output,
            self._CANONICAL_RE,
            msg=(
                f"'version' deve imprimir 'trackfw X.Y.Z' (sem prefixo v) em stdout; "
                f"obtido: {result.stdout!r}"
            ),
        )

    def test_version_surfaces_byte_identical(self):
        """As duas superfícies ('version' e '--version') produzem saída idêntica byte a byte."""
        flag_result = run_trackfw("--version")
        sub_result = run_trackfw("version")
        self.assertEqual(flag_result.returncode, 0)
        self.assertEqual(sub_result.returncode, 0)
        # Comparação byte-a-byte: os bytes de stdout devem ser iguais.
        self.assertEqual(
            flag_result.stdout,
            sub_result.stdout,
            msg=(
                f"'--version' e 'version' devem produzir saída byte-a-byte idêntica; "
                f"--version: {flag_result.stdout!r}, version: {sub_result.stdout!r}"
            ),
        )

    def test_version(self):
        """trackfw --version retorna código 0 e imprime a versão (legado — mantido para compatibilidade)."""
        result = run_trackfw("--version")
        self.assertEqual(result.returncode, 0)
        combined = result.stdout + result.stderr
        self.assertIn("trackfw", combined)


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


class TestUnknownCommand(unittest.TestCase):
    """ADR-2026-08-15 (remocao do subsistema de plugins): comando desconhecido
    nunca deve tentar executar um binario trackfw-* do PATH — deve falhar com
    erro de comando desconhecido e exit code != 0."""

    def test_comando_inexistente_retorna_erro_sem_executar_binario(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("comando-inexistente", cwd=tmpdir)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("invalid choice: 'comando-inexistente'", result.stderr)


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
        self.assertIn("📊 Inventory", result.stdout)

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
