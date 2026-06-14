"""
commands/serve.py — Subcomando `trackfw serve`.
Sobe um servidor HTTP local com dashboard web (stdlib apenas, zero dependências externas).
Espelho Python de internal/commands/serve.go e npm/src/commands/serve.js.
"""

import functools
import mimetypes
import os
import sys
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse

STATIC_DIR = os.path.join(os.path.dirname(__file__), "..", "serve", "static")

# Mapeamento explícito de extensões para garantir Content-Type correto
MIME_TYPES = {
    ".html": "text/html; charset=utf-8",
    ".js": "application/javascript; charset=utf-8",
    ".css": "text/css; charset=utf-8",
    ".json": "application/json",
    ".png": "image/png",
    ".ico": "image/x-icon",
    ".svg": "image/svg+xml",
}


class TrackfwHandler(BaseHTTPRequestHandler):
    """Handler HTTP que serve o dashboard e as APIs REST."""

    def __init__(self, cfg, *args, **kwargs):
        self.cfg = cfg
        super().__init__(*args, **kwargs)

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/" or path == "":
            self._serve_static_file("index.html")
        elif path.startswith("/static/"):
            # Remove o prefixo /static/ para obter o nome do arquivo
            filename = path[len("/static/"):]
            # Impedir path traversal em arquivos estáticos
            filename = os.path.basename(filename)
            self._serve_static_file(filename)
        elif path == "/api/board":
            from trackfw.serve.api_board import get_board
            self._json(get_board(self.cfg))
        elif path == "/api/chain":
            from trackfw.serve.api_chain import get_chain
            self._json(get_chain(self.cfg))
        elif path == "/api/metrics":
            from trackfw.serve.api_metrics import get_metrics
            self._json(get_metrics(self.cfg))
        elif path == "/api/file":
            from trackfw.serve.api_file import get_file
            get_file(self.cfg, parsed, self)
        else:
            self.send_error(404, "Not found")

    def _json(self, data):
        """Envia resposta JSON."""
        body = json.dumps(data).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(body)

    def _serve_static_file(self, filename):
        """Serve um arquivo do diretório static/."""
        # Normalizar path para evitar traversal
        safe_name = os.path.basename(filename)
        file_path = os.path.join(STATIC_DIR, safe_name)
        file_path = os.path.realpath(file_path)
        static_real = os.path.realpath(STATIC_DIR)

        # Segurança: garantir que o arquivo está dentro de STATIC_DIR
        if not file_path.startswith(static_real + os.sep) and file_path != static_real:
            self.send_error(403, "Access denied")
            return

        if not os.path.isfile(file_path):
            self.send_error(404, f"File not found: {safe_name}")
            return

        ext = os.path.splitext(safe_name)[1].lower()
        content_type = MIME_TYPES.get(ext)
        if not content_type:
            content_type, _ = mimetypes.guess_type(safe_name)
            if not content_type:
                content_type = "application/octet-stream"

        try:
            with open(file_path, "rb") as f:
                body = f.read()
        except OSError as e:
            self.send_error(500, f"Cannot read file: {e}")
            return

        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        """Silencia os logs padrão do BaseHTTPRequestHandler."""
        pass


def _open_browser(url):
    """Abre o navegador padrão na URL indicada."""
    import subprocess
    import platform

    system = platform.system()
    try:
        if system == "Darwin":
            subprocess.Popen(["open", url])
        elif system == "Windows":
            subprocess.Popen(["start", url], shell=True)
        else:
            subprocess.Popen(["xdg-open", url])
    except OSError:
        pass  # falha silenciosa se não conseguir abrir


def cmd_serve(args):
    """Inicializa e roda o servidor HTTP local."""
    from trackfw import config as _config

    port = getattr(args, "port", 8080)
    no_open = getattr(args, "no_open", False)

    cfg = _config.load()

    url = f"http://localhost:{port}"

    # Criar handler com cfg injetado via functools.partial
    handler_class = functools.partial(TrackfwHandler, cfg)

    try:
        server = HTTPServer(("", port), handler_class)
    except OSError as e:
        print(f"trackfw serve: cannot bind to port {port}: {e}", file=sys.stderr)
        sys.exit(1)

    print(f"trackfw dashboard: {url}")
    print("Press Ctrl+C to stop.")

    if not no_open:
        _open_browser(url)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopped.")
        server.server_close()
        sys.exit(0)


def register(subparsers):
    """Adiciona subcomando `serve` ao parser principal."""
    parser = subparsers.add_parser(
        "serve",
        help="Start local web dashboard",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8080,
        metavar="PORT",
        help="Port to listen on (default: 8080)",
    )
    parser.add_argument(
        "--no-open",
        action="store_true",
        dest="no_open",
        help="Do not open browser automatically",
    )
    parser.set_defaults(func=cmd_serve)
