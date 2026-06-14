"""
serve/api_file.py — File API: retorna conteúdo raw de um artefato Markdown.
Aplica validação de segurança (path traversal) antes de servir.
Espelho Python de internal/serve/api_file.go e npm/src/serve/api_file.js.
"""

import os
from urllib.parse import parse_qs


def _is_safe_path(base_dir, requested_path):
    """
    Verifica se requested_path resolve para dentro de base_dir.
    Usa os.path.realpath para resolver symlinks e '..' antes da comparação.
    """
    real_base = os.path.realpath(base_dir)
    real_path = os.path.realpath(requested_path)
    # Garantir que real_base termina com separador para evitar falsos positivos
    # (ex: /docs/adr vs /docs/adr2)
    if not real_base.endswith(os.sep):
        real_base = real_base + os.sep
    return real_path.startswith(real_base)


def get_file(cfg, parsed_url, handler):
    """
    Serve o conteúdo de um arquivo .md como text/plain.

    Parâmetro de query: ?path=<rel_path>

    Segurança: o path resolvido deve estar dentro de um dos diretórios
    autorizados (adr_dirs, req_dir, roadmap_dir). Se não, retorna 403.
    """
    qs = parse_qs(parsed_url.query)
    path_list = qs.get("path", [])
    if not path_list:
        handler.send_error(400, "Missing 'path' query parameter")
        return

    rel_path = path_list[0]

    # Construir path absoluto a partir do cwd
    cwd = os.getcwd()
    requested_abs = os.path.join(cwd, rel_path)

    # Diretórios autorizados
    adr_dirs = cfg.get("adr_dirs", ["docs/adr"])
    req_dir = cfg.get("req_dir", "docs/req")
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")

    authorized_dirs = list(adr_dirs) + [req_dir, roadmap_dir]
    authorized_abs = [os.path.join(cwd, d) for d in authorized_dirs]

    # Verificar se o path está dentro de algum diretório autorizado
    allowed = any(_is_safe_path(auth_dir, requested_abs) for auth_dir in authorized_abs)
    if not allowed:
        handler.send_error(403, "Access denied")
        return

    # Verificar existência
    if not os.path.isfile(requested_abs):
        handler.send_error(404, "File not found")
        return

    # Ler e retornar conteúdo
    try:
        with open(requested_abs, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError as e:
        handler.send_error(500, f"Cannot read file: {e}")
        return

    body = content.encode("utf-8")
    handler.send_response(200)
    handler.send_header("Content-Type", "text/plain; charset=utf-8")
    handler.send_header("Content-Length", str(len(body)))
    handler.end_headers()
    handler.wfile.write(body)
