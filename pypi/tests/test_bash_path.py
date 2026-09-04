"""Guarda a resolução de `bash` dos sítios de teste — ML-0C do
ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz.

Existe para que ninguém "simplifique" `bash_cmd(...)` de volta para `["bash", ...]`: no Windows o
nome nu resolve para `System32\\bash.exe`, o stub do WSL, que sai 1 falando UTF-16 pelo stdout e
faz o guard nunca ser invocado. Ver pypi/tests/bash_path.py para a medição.

🔴 Estes testes NÃO pulam em nenhuma plataforma. Se não houver GNU bash, eles FALHAM nomeando isso —
pular trocaria um bloqueio visível por um invisível.
"""

import os
import subprocess
import unittest

from .bash_path import bash_cmd, bash_executable


class TestBashPathResolution(unittest.TestCase):

    def test_resolve_para_caminho_absoluto_e_nao_para_o_nome_nu(self):
        # A falsificação local do ML-0C: o valor entregue ao subprocess deixou de ser "bash".
        path = bash_executable()
        self.assertTrue(os.path.isabs(path), "esperado caminho absoluto, veio %r" % path)
        self.assertNotEqual(path, "bash")
        self.assertNotEqual(path, os.path.basename(path))

    def test_identidade_provada_por_gnu_bash_no_version(self):
        # O discriminante entre "não achou" e "achou o errado" NÃO é o exit code: é a identidade.
        proc = subprocess.run([bash_executable(), "--version"], capture_output=True)
        self.assertEqual(proc.returncode, 0, proc.stderr[:200])
        self.assertIn(b"GNU bash", proc.stdout + proc.stderr)

    def test_nunca_resolve_para_o_stub_do_wsl(self):
        path = os.path.normcase(bash_executable())
        self.assertNotIn(os.path.normcase(os.path.join("system32", "bash.exe")), path)

    def test_bash_cmd_prefixa_o_caminho_absoluto_e_preserva_os_argumentos(self):
        argv = bash_cmd("script.sh", "git", "push")
        self.assertEqual(argv[0], bash_executable())
        self.assertEqual(argv[1:], ["script.sh", "git", "push"])

    def test_o_bash_resolvido_executa_o_script_de_fato(self):
        # Não-vacuidade: prova que o caminho resolvido roda um script, em vez de só responder
        # --version. É a execução que os 10 sítios corrigidos dependem.
        proc = subprocess.run(
            bash_cmd("-c", 'echo trackfw-ml0c; exit 3'),
            capture_output=True,
            text=True,
        )
        self.assertEqual(proc.returncode, 3, proc.stderr)
        self.assertIn("trackfw-ml0c", proc.stdout)


if __name__ == "__main__":
    unittest.main()
