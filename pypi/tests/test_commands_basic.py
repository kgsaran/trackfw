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
            self.assertIn("ADR-001", files[0])

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
    def test_log_cria_arquivo(self):
        """trackfw log 'mensagem teste' cria .trackfw-log com a mensagem."""
        with tempfile.TemporaryDirectory() as tmpdir:
            result = run_trackfw("log", "mensagem teste", cwd=tmpdir)
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            log_path = os.path.join(tmpdir, ".trackfw-log")
            self.assertTrue(os.path.isfile(log_path), ".trackfw-log não criado")
            with open(log_path, encoding="utf-8") as f:
                content = f.read()
            self.assertIn("mensagem teste", content)

    def test_log_append(self):
        """trackfw log faz append — múltiplas chamadas acumulam linhas."""
        with tempfile.TemporaryDirectory() as tmpdir:
            run_trackfw("log", "primeira mensagem", cwd=tmpdir)
            run_trackfw("log", "segunda mensagem", cwd=tmpdir)
            log_path = os.path.join(tmpdir, ".trackfw-log")
            with open(log_path, encoding="utf-8") as f:
                lines = [l for l in f.read().splitlines() if l.strip()]
            self.assertEqual(len(lines), 2, f"Esperava 2 linhas, encontrei: {lines}")
            self.assertIn("primeira mensagem", lines[0])
            self.assertIn("segunda mensagem", lines[1])

    def test_log_formato_timestamp(self):
        """Linha do log tem timestamp no formato YYYY-MM-DD HH:MM."""
        import re
        with tempfile.TemporaryDirectory() as tmpdir:
            run_trackfw("log", "teste timestamp", cwd=tmpdir)
            log_path = os.path.join(tmpdir, ".trackfw-log")
            with open(log_path, encoding="utf-8") as f:
                content = f.read()
            self.assertRegex(content, r"\d{4}-\d{2}-\d{2} \d{2}:\d{2}")


class TestStubs(unittest.TestCase):
    def test_validate_stub(self):
        """trackfw validate imprime 'Not implemented yet' e sai com 0."""
        result = run_trackfw("validate")
        self.assertEqual(result.returncode, 0)
        self.assertIn("Not implemented yet", result.stdout)

    def test_status_stub(self):
        """trackfw status imprime 'Not implemented yet' e sai com 0."""
        result = run_trackfw("status")
        self.assertEqual(result.returncode, 0)
        self.assertIn("Not implemented yet", result.stdout)

    def test_metrics_stub(self):
        """trackfw metrics imprime 'Not implemented yet' e sai com 0."""
        result = run_trackfw("metrics")
        self.assertEqual(result.returncode, 0)
        self.assertIn("Not implemented yet", result.stdout)

    def test_context_stub(self):
        """trackfw context imprime 'Not implemented yet' e sai com 0."""
        result = run_trackfw("context")
        self.assertEqual(result.returncode, 0)
        self.assertIn("Not implemented yet", result.stdout)


if __name__ == "__main__":
    unittest.main()
