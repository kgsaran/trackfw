"""
Testes unitários para pypi/trackfw/commands/serve.py — resolução de endereço
(bind loopback/IPv6) e formatação da URL impressa (ML-1B).

Espelha internal/serve/serve_test.go e npm/tests/serve_address.test.js.
"""

import http.client
import os
import socket
import sys
import threading

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw.commands.serve import (
    _display_url,
    _is_loopback_host,
    _server_class_for_host,
)


# ---------------------------------------------------------------------------
# _display_url
# ---------------------------------------------------------------------------

@pytest.mark.parametrize(
    "host,port,expected",
    [
        ("localhost", 4080, "http://localhost:4080"),
        ("127.0.0.1", 4080, "http://localhost:4080"),
        ("127.0.0.5", 4080, "http://localhost:4080"),
        ("::1", 4080, "http://[::1]:4080"),
        ("2001:db8::1", 4080, "http://[2001:db8::1]:4080"),
        ("::ffff:127.0.0.1", 4080, "http://[::ffff:127.0.0.1]:4080"),  # paridade com Go/Node
        ("192.168.3.137", 4080, "http://192.168.3.137:4080"),
        ("0.0.0.0", 4080, "http://0.0.0.0:4080"),
    ],
)
def test_display_url(host, port, expected):
    assert _display_url(host, port) == expected


# ---------------------------------------------------------------------------
# _is_loopback_host (não alterado neste ML — conferido, não-regressão)
# ---------------------------------------------------------------------------

def test_is_loopback_host_accepts_ipv6_loopback():
    assert _is_loopback_host("::1") is True


def test_is_loopback_host_rejects_lan():
    assert _is_loopback_host("192.168.3.137") is False


# ---------------------------------------------------------------------------
# _server_class_for_host + bind real — prova por escuta, não por leitura
# ---------------------------------------------------------------------------

def test_server_class_for_host_ipv6():
    cls = _server_class_for_host("::1")
    assert cls.address_family == socket.AF_INET6


def test_server_class_for_host_ipv4():
    cls = _server_class_for_host("127.0.0.1")
    assert cls.address_family == socket.AF_INET


def _serve_one_request(server):
    thread = threading.Thread(target=server.handle_request, daemon=True)
    thread.start()
    return thread


def test_bind_and_respond_on_ipv6_loopback():
    from functools import partial
    from trackfw.commands.serve import TrackfwHandler
    from trackfw import config as _config

    cfg = _config.load()
    cls = _server_class_for_host("::1")
    handler_class = partial(TrackfwHandler, cfg)
    try:
        server = cls(("::1", 0), handler_class)
    except OSError as e:
        pytest.skip(f"IPv6 loopback unavailable in this environment: {e}")

    try:
        port = server.server_address[1]
        thread = _serve_one_request(server)
        conn = http.client.HTTPConnection("::1", port, timeout=5)
        conn.request("GET", "/")
        resp = conn.getresponse()
        assert resp.status == 200
        conn.close()
        thread.join(timeout=5)
    finally:
        server.server_close()


def test_bind_and_respond_on_ipv4_loopback_default():
    """Não-regressão: o padrão sem --host continua escutando em 127.0.0.1."""
    from functools import partial
    from trackfw.commands.serve import TrackfwHandler
    from trackfw import config as _config

    cfg = _config.load()
    cls = _server_class_for_host("127.0.0.1")
    handler_class = partial(TrackfwHandler, cfg)
    server = cls(("127.0.0.1", 0), handler_class)

    try:
        assert server.server_address[0] == "127.0.0.1"
        port = server.server_address[1]
        thread = _serve_one_request(server)
        conn = http.client.HTTPConnection("localhost", port, timeout=5)
        conn.request("GET", "/")
        resp = conn.getresponse()
        assert resp.status == 200
        conn.close()
        thread.join(timeout=5)
    finally:
        server.server_close()
