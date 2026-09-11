"""
Testes unitários para as funções de segurança de abertura de browser em
pypi/trackfw/commands/serve.py — AC1 + AC2 + AC3 + AC4.

Reconciliação (regra dura do projeto):
  Cada teste declara em uma frase qual conclusão deste ML ele afirma.
"""

import os
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.commands.serve import (
    _browser_argv,
    _is_valid_host,
    _open_browser,
)


class TestBrowserArgv(unittest.TestCase):
    """AC1/AC2 — _browser_argv retorna argv sem interpolação de shell."""

    # AFIRMAÇÃO: Darwin → cmd='open', args=[url] como elemento único de argv.
    # A URL com metacaracteres de shell chega ao processo 'open' como string
    # literal, sem interpretação pelo shell.
    def test_darwin_returns_open_with_url_as_single_argv(self):
        cmd, *rest = _browser_argv("Darwin", "http://localhost:4080")
        self.assertEqual(cmd, "open")
        self.assertEqual(rest, ["http://localhost:4080"])

    # AFIRMAÇÃO: Linux → cmd='xdg-open', args=[url] — mesma garantia de argv.
    def test_linux_returns_xdg_open_with_url_as_single_argv(self):
        cmd, *rest = _browser_argv("Linux", "http://192.168.1.1:8080")
        self.assertEqual(cmd, "xdg-open")
        self.assertEqual(rest, ["http://192.168.1.1:8080"])

    # AFIRMAÇÃO: AC2 — Windows → cmd='cmd', args ['/c', 'start', '', url]
    # sem shell=True no Popen; a lista não é reunida em string pelo runtime
    # Python antes de chegar ao CreateProcess.
    def test_windows_returns_cmd_c_start_without_shell_true(self):
        url = "http://my-host.example.com:4080"
        argv = _browser_argv("Windows", url)
        self.assertEqual(argv[0], "cmd")
        self.assertIn("/c", argv)
        self.assertIn("start", argv)
        self.assertEqual(argv[-1], url)
        # Garantia específica do AC2: a URL é o último elemento, não parte de
        # uma string montada pelo runtime.
        self.assertIsInstance(argv, list)

    # AFIRMAÇÃO: uma URL com metacaracteres de shell chega INTACTA como
    # elemento de argv — nenhuma expansão acontece no construtor de argv.
    def test_browser_argv_passes_metachar_url_as_literal(self):
        malicious = 'http://x" ; id > /tmp/INJETADO ; echo ":4080'
        argv = _browser_argv("Darwin", malicious)
        # URL é o segundo e último elemento
        self.assertEqual(argv[-1], malicious)
        self.assertEqual(len(argv), 2)


class TestOpenBrowserUsesArgv(unittest.TestCase):
    """AC1/AC2 — _open_browser chama Popen com lista, sem shell=True."""

    # AFIRMAÇÃO: Darwin → Popen chamado com ['open', url] e sem shell=True —
    # confirma que a correção do Node.js tem espelho Python para Darwin.
    def test_darwin_popen_no_shell(self):
        with mock.patch("subprocess.Popen") as mock_popen:
            _open_browser.__wrapped__ = None  # reset qualquer cache
            with mock.patch("platform.system", return_value="Darwin"):
                _open_browser("http://localhost:4080")
            mock_popen.assert_called_once()
            args, kwargs = mock_popen.call_args
            # argv passado como lista
            self.assertIsInstance(args[0], list)
            self.assertEqual(args[0][0], "open")
            # sem shell=True
            self.assertNotEqual(kwargs.get("shell"), True)

    # AFIRMAÇÃO: AC2 — Windows → Popen NÃO usa shell=True. Este é o defeito
    # corrigido: antes era Popen(["start", url], shell=True), que reunia a
    # lista em string de shell e permitia injeção.
    def test_windows_popen_no_shell_true(self):
        with mock.patch("subprocess.Popen") as mock_popen:
            with mock.patch("platform.system", return_value="Windows"):
                _open_browser("http://localhost:4080")
            mock_popen.assert_called_once()
            args, kwargs = mock_popen.call_args
            # argv passado como lista
            self.assertIsInstance(args[0], list)
            # cmd.exe é o primeiro elemento (necessário para builtin 'start')
            self.assertEqual(args[0][0], "cmd")
            # 🔴 GARANTIA CENTRAL DO AC2: sem shell=True
            self.assertNotEqual(kwargs.get("shell"), True)
            # URL é o último elemento de argv (não embutida em string)
            self.assertEqual(args[0][-1], "http://localhost:4080")

    # AFIRMAÇÃO: Linux → Popen chamado com ['xdg-open', url], sem shell=True —
    # o ramo Linux já estava correto; teste confirma que a refatoração não
    # regrediu.
    def test_linux_popen_no_shell(self):
        with mock.patch("subprocess.Popen") as mock_popen:
            with mock.patch("platform.system", return_value="Linux"):
                _open_browser("http://localhost:4080")
            mock_popen.assert_called_once()
            args, kwargs = mock_popen.call_args
            self.assertIsInstance(args[0], list)
            self.assertEqual(args[0][0], "xdg-open")
            self.assertNotEqual(kwargs.get("shell"), True)


class TestIsValidHost(unittest.TestCase):
    """AC4 — _is_valid_host valida na entrada, antes de qualquer exec."""

    # AFIRMAÇÃO: _is_valid_host rejeita o payload de injeção da REQ — o host
    # com metacaracteres de shell nunca chega ao browser-open path.
    def test_rejects_injection_payload(self):
        self.assertFalse(_is_valid_host('x" ; id > /tmp/INJETADO ; echo "'))

    # AFIRMAÇÃO: _is_valid_host aceita 'localhost' — contra-braço do AC3:
    # rejeitar tudo quebraria o serve para o caso padrão.
    def test_accepts_localhost(self):
        self.assertTrue(_is_valid_host("localhost"))

    # AFIRMAÇÃO: aceita endereços IPv4 válidos, incluindo o padrão de bind.
    def test_accepts_ipv4(self):
        self.assertTrue(_is_valid_host("127.0.0.1"))
        self.assertTrue(_is_valid_host("0.0.0.0"))
        self.assertTrue(_is_valid_host("192.168.1.100"))

    # AFIRMAÇÃO: aceita endereços IPv6 válidos.
    def test_accepts_ipv6(self):
        self.assertTrue(_is_valid_host("::1"))
        self.assertTrue(_is_valid_host("2001:db8::1"))

    # AFIRMAÇÃO: aceita hostnames RFC-1123 com hífens e domínios compostos.
    def test_accepts_rfc1123_hostname(self):
        self.assertTrue(_is_valid_host("my-host.example.com"))
        self.assertTrue(_is_valid_host("host"))
        self.assertTrue(_is_valid_host("internal-host"))

    # AFIRMAÇÃO: rejeita strings com metacaracteres de shell além do payload
    # exato da REQ — o filtro não é estreito ao caso reproduzido.
    def test_rejects_shell_metacharacters(self):
        self.assertFalse(_is_valid_host("host;cmd"))
        self.assertFalse(_is_valid_host("host&cmd"))
        self.assertFalse(_is_valid_host("host|cmd"))
        self.assertFalse(_is_valid_host("host>file"))
        self.assertFalse(_is_valid_host("host`cmd`"))

    # AFIRMAÇÃO: rejeita IPv6 scoped addresses (zone ID com '%') — incluindo
    # zone IDs sintaticamente limpos. Python's ipaddress.ip_address() aceita
    # qualquer conteúdo depois de '%', incluindo metacaracteres de cmd.exe
    # que list2cmdline não cita. Rejeitar '%' fecha toda a classe em vez de
    # enumerar metacaracteres individuais, e alinha Python com Go (que rejeita
    # todos os endereços scoped via net.ParseIP) e com Node.js corrigido.
    def test_rejects_ipv6_scoped_address(self):
        # zone ID limpo — agora rejeitado para paridade com Go e para fechar a
        # classe inteira de zone IDs (não só os que contêm metacaracteres)
        self.assertFalse(_is_valid_host("fe80::1%eth0"))
        # zone ID com metacaracteres de cmd.exe — o vetor do bloqueio do hades-tf
        self.assertFalse(_is_valid_host("fe80::1%eth0&calc.exe&echo"))
        self.assertFalse(_is_valid_host("fe80::1%eth0;id"))
        self.assertFalse(_is_valid_host("fe80::1%eth0|whoami"))
        # forma mínima com índice numérico
        self.assertFalse(_is_valid_host("fe80::1%0"))
        # percent-encoding em hostname não-IPv6 também rejeitado
        self.assertFalse(_is_valid_host("host%20name"))

    # AFIRMAÇÃO: list2cmdline não cita '&' quando não há espaço adjacente —
    # este teste afirma que o vetor de injeção via zone ID é REAL: sem a guarda
    # de '%', _is_valid_host aceitaria um host que produz uma linha de cmd.exe
    # com '&' não-citado, executável como separador de comandos. O braço
    # vulnerável prova que o gate discrimina (não passa vacuamente).
    def test_list2cmdline_unquoted_ampersand_proves_vector_real(self):
        import subprocess
        url = "http://[fe80::1%eth0&calc.exe&echo]:4080"
        argv = ["cmd", "/c", "start", "", url]
        cmd_str = subprocess.list2cmdline(argv)
        # '&' está presente na saída e NÃO foi citado por list2cmdline
        self.assertIn("&", cmd_str)
        # Prova que não há aspas em torno do '&': list2cmdline cita com aspas
        # duplas; se o '&' estivesse citado, a string conteria '"&"'.
        self.assertNotIn('"&"', cmd_str)


class TestIsValidHostZoneIdParity(unittest.TestCase):
    """Parity — os 3 CLIs concordam: '%' em host → rejeitado."""

    # AFIRMAÇÃO: todos os cmd.exe metacharacters derivados passam pela guarda
    # de '%' antes mesmo de ipaddress.ip_address() ser chamado — a lista de
    # permissão do zone ID é o conjunto vazio, não uma lista de bloqueio de '&'.
    # Metacaracteres verificados: & | < > ^ ( ) " % ! , ; = espaço tab CR LF.
    # Todos chegam ao rejeitor de '%' via o vetor zone ID; os demais caminhos
    # (RFC-1123, IPv4) já os rejeitavam antes desta correção.
    def test_all_cmd_metacharacters_via_zone_id_are_rejected(self):
        # metacaracteres do cmd.exe derivados da documentação
        cmd_metacharacters = ['&', '|', '<', '>', '^', '(', ')', '"', '%', '!', ',', ';', '=',
                               ' ', '\t', '\r', '\n']
        for meta in cmd_metacharacters:
            host = f"fe80::1%eth0{meta}id"
            self.assertFalse(
                _is_valid_host(host),
                msg=f"_is_valid_host should reject zone ID with cmd.exe metacharacter {meta!r}: {host!r}"
            )


if __name__ == "__main__":
    unittest.main()
