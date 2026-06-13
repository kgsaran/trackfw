"""
generators/adr.py — geração de ADR sequencial com numeração automática.
Espelha npm/src/generators/adr.js em Python puro (stdlib apenas).
"""

import os
import re
import unicodedata
from datetime import date


def next_adr_number(adr_dir: str) -> int:
    """
    Escaneia adr_dir por arquivos ADR-NNN-*.md e retorna max(NNN)+1.
    Retorna 1 se o diretório estiver vazio ou não existir.
    """
    if not os.path.isdir(adr_dir):
        return 1

    pattern = re.compile(r'^ADR-(\d+)-.*\.md$', re.IGNORECASE)
    max_num = 0

    for entry in os.listdir(adr_dir):
        m = pattern.match(entry)
        if m:
            num = int(m.group(1))
            if num > max_num:
                max_num = num

    return max_num + 1


def slugify(title: str) -> str:
    """
    Converte título em slug: lowercase, acentos removidos via NFKD,
    espaços → hifens, remove chars não-alfanuméricos exceto hífen.
    """
    # Normaliza para NFKD e descarta caracteres não-ASCII
    normalized = unicodedata.normalize('NFKD', title)
    ascii_str = normalized.encode('ascii', 'ignore').decode('ascii')

    # Lowercase e espaços → hifens
    slug = ascii_str.lower().replace(' ', '-')

    # Remove chars não-alfanuméricos exceto hífen
    slug = re.sub(r'[^a-z0-9-]', '', slug)

    # Colapsa hifens múltiplos
    slug = re.sub(r'-+', '-', slug)

    return slug.strip('-')


def _today() -> str:
    return date.today().isoformat()


def generate_adr(
    title: str,
    status: str = 'Draft',
    adr_dirs: list = None,
    cwd: str = None,
) -> str:
    """
    Gera arquivo ADR no primeiro diretório de adr_dirs (ou 'docs/adr' como default).
    Cria o diretório se não existir.
    Retorna o path absoluto do arquivo criado.
    """
    base = cwd or os.getcwd()

    if adr_dirs and len(adr_dirs) > 0:
        adr_dir = adr_dirs[0]
    else:
        adr_dir = 'docs/adr'

    # Tornar absoluto se relativo
    if not os.path.isabs(adr_dir):
        adr_dir = os.path.join(base, adr_dir)

    os.makedirs(adr_dir, exist_ok=True)

    num = next_adr_number(adr_dir)
    slug = slugify(title)
    num_str = str(num).zfill(3)
    name = f'ADR-{num_str}-{slug}'
    filename = f'{name}.md'
    filepath = os.path.join(adr_dir, filename)
    today = _today()

    body = f"""---
name: {name}
title: "{title}"
status: {status}
created: {today}
---

# ADR-{num_str}: {title}

## Status
{status}

## Context
<!-- Descreva o contexto e o problema que motivou esta decisão -->

## Decision
<!-- Descreva a decisão tomada -->

## Consequences
<!-- Descreva as consequências desta decisão -->
"""

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(body)

    return filepath
