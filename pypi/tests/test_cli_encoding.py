"""
Testes do forçamento de UTF-8 na saída do CLI.
Usa unittest (stdlib) — sem dependências externas.

"""

import io
import os
import subprocess
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.cli import _force_utf8_output

PYPI_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))


class _StreamSemReconfigure:
    """Dublê do que testes e pipelines colocam no lugar de sys.stdout."""

    def __init__(self):
        self.buf = io.StringIO()

    def write(self, s):
        return self.buf.write(s)

    def flush(self):
        pass


class _StreamQueRegistra:
    def __init__(self):
        self.chamadas = []

    def reconfigure(self, **kwargs):
        self.chamadas.append(kwargs)


class TestForceUtf8Output(unittest.TestCase):

    def setUp(self):
        self._out, self._err = sys.stdout, sys.stderr

    def tearDown(self):
        sys.stdout, sys.stderr = self._out, self._err

    def test_stream_sem_reconfigure_nao_levanta(self):
        """StringIO e afins não têm reconfigure — o helper tem que ignorar."""
        sys.stdout = _StreamSemReconfigure()
        sys.stderr = _StreamSemReconfigure()
        _force_utf8_output()  # não pode levantar

    def test_reconfigura_com_utf8_e_replace(self):
        out, err = _StreamQueRegistra(), _StreamQueRegistra()
        sys.stdout, sys.stderr = out, err

        _force_utf8_output()

        # newline: sem ele o Python emite CRLF no Windows enquanto Go e
        # Node.js emitem LF, e as três saídas divergem byte a byte para o
        # mesmo comando.
        esperado = {"encoding": "utf-8", "errors": "replace", "newline": "\n"}
        self.assertEqual(out.chamadas, [esperado])
        self.assertEqual(err.chamadas, [esperado])


class TestCliEmConsoleCp1252(unittest.TestCase):
    """Reproduz o console Windows de forma determinística, em qualquer sistema:
    PYTHONIOENCODING=cp1252 faz o Python abrir stdout com a mesma codificação
    que quebrava --help, status e validate."""

    def _run(self, *args):
        env = dict(os.environ)
        env["PYTHONIOENCODING"] = "cp1252"
        env["PYTHONPATH"] = PYPI_ROOT
        env.pop("PYTHONUTF8", None)
        return subprocess.run(
            [sys.executable, "-m", "trackfw", *args],
            cwd=PYPI_ROOT,
            env=env,
            capture_output=True,
        )

    def test_help_nao_quebra(self):
        r = self._run("--help")
        self.assertEqual(r.returncode, 0, r.stderr.decode("utf-8", "replace"))
        self.assertNotIn(b"UnicodeEncodeError", r.stderr)

    def test_status_nao_quebra(self):
        r = self._run("status")
        self.assertEqual(r.returncode, 0, r.stderr.decode("utf-8", "replace"))
        self.assertNotIn(b"UnicodeEncodeError", r.stderr)

    def test_validate_nao_quebra(self):
        r = self._run("validate")
        self.assertNotIn(b"UnicodeEncodeError", r.stderr)


if __name__ == "__main__":
    unittest.main()
