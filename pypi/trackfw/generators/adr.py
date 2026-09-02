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
    Converte string para slug kebab-case portável.
    NFKD + remoção de diacríticos + lowercase + [^a-z0-9]+ → hífen.
    Ex: "Autenticação e Sessão" → "autenticacao-e-sessao"

    Colapso, nunca deleção — ver `## Artifact slug contract` em docs/cli-parity.md.
    Esta função deletava os não-alfanuméricos, que é a regra do slug de *identidade
    de agente*, não a de artefato: "C/C++" virava "cc" aqui e "c-c" em Go, Node e nos
    outros três geradores do próprio Python.
    """
    normalized = unicodedata.normalize("NFKD", title)
    ascii_str = normalized.encode("ascii", "ignore").decode("ascii")
    slug = ascii_str.lower()
    slug = re.sub(r"[^a-z0-9]+", "-", slug)
    slug = re.sub(r"-+", "-", slug)
    return slug.strip("-")


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

    with open(filepath, 'w', encoding='utf-8', newline="\n") as f:
        f.write(body)

    return filepath


def global_adr_dir(home: str) -> str:
    """
    Retorna o diretório global de ADRs cross-project: <home>/.trackfw/adr.
    Espelha GlobalADRDir (Go, internal/generators/scaffold.go) e o path
    literal usado por npm/src/commands/adr.js (resolveAdrDir).
    """
    return os.path.join(home, '.trackfw', 'adr')


def _parse_adr_status(filepath: str) -> str:
    """
    Extrai o status de um ADR a partir da linha "> Date: ... | Status: ...".
    Espelha parseADRMeta (Go) / parseADRStatus (Node): retorna o primeiro
    match de "| Status: " na primeira linha em que ocorrer, aparado de
    espaços e dos caracteres '>' e '|' à direita. 'unknown' se não encontrar
    ou se o arquivo não puder ser lido.
    """
    try:
        with open(filepath, encoding='utf-8') as f:
            for line in f:
                idx = line.find('| Status: ')
                if idx >= 0:
                    rest = line[idx + len('| Status: '):]
                    rest = rest.rstrip(' >|\n\r')
                    return rest.strip()
    except OSError:
        pass
    return 'unknown'


def list_adrs(dir: str) -> None:
    """
    Lista todos os ADRs (*.md) encontrados em dir, imprimindo
    "<filename padded a 60 chars> <status>" por linha, em ordem alfabética.
    Espelha ListADRs (Go, internal/generators/adr.go) e listADRs
    (Node, npm/src/generators/adr.js) byte a byte.

    Se dir não existir ou não tiver arquivos .md, imprime
    "No ADRs found in <dir>".
    """
    if not os.path.isdir(dir):
        print(f'No ADRs found in {dir}')
        return

    files = sorted(f for f in os.listdir(dir) if f.endswith('.md'))

    if not files:
        print(f'No ADRs found in {dir}')
        return

    for filename in files:
        filepath = os.path.join(dir, filename)
        status = _parse_adr_status(filepath)
        print(f'{filename:<60} {status}')
