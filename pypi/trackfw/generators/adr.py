"""
generators/adr.py — Gerador de ADRs para trackfw.
Espelha npm/src/generators/adr.js (funções newADR, newADRDraft).
Formato canônico Go/Node — REQ-2026-07-27-convergencia-templates-python.
Stdlib apenas — sem dependências externas.
"""

import os
import re
import unicodedata
from datetime import date


def slugify(title: str) -> str:
    """
    Converte título em slug: lowercase, acentos removidos via NFKD,
    espaços → hifens, remove chars não-alfanuméricos exceto hífen.
    """
    normalized = unicodedata.normalize('NFKD', title)
    ascii_str = normalized.encode('ascii', 'ignore').decode('ascii')
    slug = ascii_str.lower().replace(' ', '-')
    slug = re.sub(r'[^a-z0-9-]', '', slug)
    slug = re.sub(r'-+', '-', slug)
    return slug.strip('-')


def _today() -> str:
    return date.today().isoformat()


def generate_adr(
    title: str,
    status: str = 'Proposed',
    adr_dirs: list = None,
    cwd: str = None,
) -> str:
    """
    Cria docs/adr/ADR-YYYY-MM-DD-<slug>.md no formato canônico Go/Node.

    Frontmatter: status · date · author: ""
    Header: > Date: <data> | Status: <status>
    Seções: ## Context, ## Decision, ## Consequences, ## Alternatives Considered
    H1: # ADR: <title>

    Args:
        title: Título do ADR.
        status: Status inicial (default: 'Proposed'). Use 'Draft' para rascunho.
        adr_dirs: Lista de diretórios destino; usa o primeiro. Default: docs/adr.
        cwd: Diretório de trabalho base (default: os.getcwd()).

    Returns:
        Path absoluto do arquivo criado.
    """
    base = cwd or os.getcwd()

    if adr_dirs and len(adr_dirs) > 0:
        adr_dir = adr_dirs[0]
    else:
        adr_dir = 'docs/adr'

    if not os.path.isabs(adr_dir):
        adr_dir = os.path.join(base, adr_dir)

    os.makedirs(adr_dir, exist_ok=True)

    slug = slugify(title)
    today = _today()
    filename = f'ADR-{today}-{slug}.md'
    filepath = os.path.join(adr_dir, filename)

    context_section = '<!-- What is the situation that motivates this decision? -->'
    decision_section = '<!-- What was decided? -->'
    consequences_section = '<!-- What are the positive and negative consequences of this decision? -->'
    alternatives_section = '<!-- What other options were evaluated and why were they rejected? -->'

    body = f"""---
status: {status}
date: {today}
author: ""
---

# ADR: {title}

> Date: {today} | Status: {status}

## Context
{context_section}

## Decision
{decision_section}

## Consequences
{consequences_section}

## Alternatives Considered
{alternatives_section}
"""

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(body)

    return filepath
