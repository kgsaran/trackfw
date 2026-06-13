"""
generators/req.py — Gerador de REQs para trackfw.
Espelha npm/src/generators/req.js (funções newREQ, listREQs, parseREQStatus).
Stdlib apenas — sem dependências externas.
"""

import os
import unicodedata
from datetime import date


def slugify(title: str) -> str:
    """
    Converte título em slug kebab-case lowercase.
    Remove acentos via NFKD + encode ascii ignore, substitui espaços por hífens.
    """
    normalized = unicodedata.normalize("NFKD", title)
    ascii_str = normalized.encode("ascii", "ignore").decode("ascii")
    return ascii_str.lower().replace(" ", "-")


def generate_req(title: str, req_dir: str = None, cwd: str = None) -> str:
    """
    Cria docs/requisições/<req_dir>/REQ-YYYY-MM-DD-<slug>.md.

    Args:
        title: Título da REQ.
        req_dir: Diretório destino (default: docs/requisicoes/claude).
        cwd: Diretório de trabalho base (default: os.getcwd()).

    Returns:
        Path absoluto do arquivo criado.
    """
    base = cwd or os.getcwd()

    if req_dir is None:
        req_dir = os.path.join(base, "docs", "requisicoes", "claude")
    elif not os.path.isabs(req_dir):
        req_dir = os.path.join(base, req_dir)

    os.makedirs(req_dir, exist_ok=True)

    slug = slugify(title)
    today = date.today().isoformat()
    filename = f"REQ-{today}-{slug}.md"
    filepath = os.path.join(req_dir, filename)

    content = f"""---
name: REQ-{today}-{slug}
title: "{title}"
status: Open
linked_adr: —
created: {today}
author:
---

# REQ: {title}

| Campo | Valor |
|---|---|
| Status | Open |
| Criado | {today} |

---

## Motivação

<!-- Descreva o problema ou oportunidade -->

---

## Critérios de Aceite

- [ ] critério 1

---

## Fora de Escopo

<!-- O que esta REQ NÃO cobre -->
"""

    with open(filepath, "w", encoding="utf-8") as f:
        f.write(content)

    return filepath
