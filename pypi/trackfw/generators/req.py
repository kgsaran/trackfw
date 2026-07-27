"""
generators/req.py — Gerador de REQs para trackfw.
Espelha npm/src/generators/req.js (funções newREQ, listREQs, parseREQStatus).
Formato canônico Go/Node, em inglês — REQ-2026-07-27-convergencia-templates-python.
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
    Cria docs/req/REQ-YYYY-MM-DD-<slug>.md no formato canônico Go/Node.

    Frontmatter: status: Open · date · author: "" · adr: "" · roadmap: ""
    Header: > Date: <data> | Status: Open
    Seções: ## Motivation, ## Acceptance Criteria, ## Linked ADR,
            ## Blocked by ADRs, ## Linked Roadmap

    Args:
        title: Título da REQ.
        req_dir: Diretório destino (default: docs/req relativo a cwd).
        cwd: Diretório de trabalho base (default: os.getcwd()).

    Returns:
        Path absoluto do arquivo criado.
    """
    base = cwd or os.getcwd()

    if req_dir is None:
        req_dir = os.path.join(base, "docs", "req")
    elif not os.path.isabs(req_dir):
        req_dir = os.path.join(base, req_dir)

    os.makedirs(req_dir, exist_ok=True)

    slug = slugify(title)
    today = date.today().isoformat()
    filename = f"REQ-{today}-{slug}.md"
    filepath = os.path.join(req_dir, filename)

    motivation_section = "<!-- Why is this requirement needed? What problem does it solve? -->"
    criteria_section = "- [ ]\n- [ ]"
    linked_adr_section = ""
    linked_roadmap_section = ""
    blocked_section = "<!-- none -->"
    status_line = f"> Date: {today} | Status: Open"

    content = f"""---
status: Open
date: {today}
author: ""
adr: ""
roadmap: ""
---

# REQ: {title}

{status_line}

## Motivation
{motivation_section}

## Acceptance Criteria
{criteria_section}

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: {linked_adr_section}

## Blocked by ADRs
{blocked_section}

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: {linked_roadmap_section}
"""

    with open(filepath, "w", encoding="utf-8") as f:
        f.write(content)

    return filepath
