"""
generators/init_gen.py — scaffold de governança trackfw em Python puro.
Espelha npm/src/generators/init.js com suporte a namespacing flat e by_agent.
Depende apenas de stdlib.
"""

import os
from datetime import date, timedelta


# ---------------------------------------------------------------------------
# Constantes
# ---------------------------------------------------------------------------

GOV_DIRS_FLAT = [
    'docs/adr',
    'docs/req',
    'docs/roadmaps/backlog',
    'docs/roadmaps/wip',
    'docs/roadmaps/blocked',
    'docs/roadmaps/done',
    'docs/roadmaps/abandoned',
]

ROADMAP_STATES = ['backlog', 'wip', 'blocked', 'done', 'abandoned']


# ---------------------------------------------------------------------------
# Função principal
# ---------------------------------------------------------------------------

def scaffold(cwd: str, opts: dict) -> None:
    """
    Cria a estrutura de governança trackfw no diretório cwd.

    opts esperado:
    {
        "project_name": str,
        "namespacing": "flat" | "by_agent",
        "agents": list[str],   # usado somente se namespacing == "by_agent"
        "wip_limit": int,
    }
    """
    namespacing = opts.get('namespacing', 'flat')
    agents = opts.get('agents', [])
    wip_limit = opts.get('wip_limit', 1)

    if namespacing == 'by_agent':
        dirs = _gov_dirs_by_agent(agents)
    else:
        dirs = GOV_DIRS_FLAT

    for d in dirs:
        abs_dir = os.path.join(cwd, d)
        os.makedirs(abs_dir, exist_ok=True)
        print(f'  checkmark {d}')

    _write_trackfw_yaml(cwd, opts)
    _write_example_adr(cwd, opts)


# ---------------------------------------------------------------------------
# Helpers de estrutura de diretórios
# ---------------------------------------------------------------------------

def _gov_dirs_by_agent(agents: list) -> list:
    """
    Retorna a lista de diretórios para o modo by_agent.
    docs/req é sempre flat (não por agente).
    """
    dirs = []
    for agent in agents:
        dirs.append(f'docs/adr/{agent}')
    dirs.append('docs/req')
    for agent in agents:
        for state in ROADMAP_STATES:
            dirs.append(f'docs/roadmaps/{agent}/{state}')
    return dirs


# ---------------------------------------------------------------------------
# trackfw.yaml
# ---------------------------------------------------------------------------

def _write_trackfw_yaml(cwd: str, opts: dict) -> None:
    namespacing = opts.get('namespacing', 'flat')
    agents = opts.get('agents', [])
    wip_limit = opts.get('wip_limit', 1)
    today = date.today().isoformat()

    lines = [
        '# trackfw configuration',
        f'# generated: {today}',
        '',
    ]

    if namespacing == 'by_agent':
        lines.append('adr_dirs:')
        for agent in agents:
            lines.append(f'  - docs/adr/{agent}')
    else:
        lines.append('adr_dirs:')
        lines.append('  - docs/adr')

    lines.append('req_dir: docs/req')
    lines.append('roadmap_dir: docs/roadmaps')
    lines.append(f'roadmap_namespacing: {namespacing}')

    if namespacing == 'by_agent' and agents:
        lines.append('agents:')
        for agent in agents:
            lines.append(f'  - {agent}')

    lines.append(f'wip_limit: {wip_limit}')
    lines.append('')  # newline final

    content = '\n'.join(lines)
    dest = os.path.join(cwd, 'trackfw.yaml')
    with open(dest, 'w', encoding='utf-8') as f:
        f.write(content)
    print('  checkmark trackfw.yaml')


# ---------------------------------------------------------------------------
# ADR exemplo
# ---------------------------------------------------------------------------

def _write_example_adr(cwd: str, opts: dict) -> None:
    """
    Cria docs/adr/ADR-001-inicio-do-projeto.md como arquivo exemplo.
    No modo by_agent cria no diretório do primeiro agente (se houver).
    """
    namespacing = opts.get('namespacing', 'flat')
    agents = opts.get('agents', [])

    if namespacing == 'by_agent' and agents:
        adr_dir = os.path.join(cwd, 'docs', 'adr', agents[0])
    else:
        adr_dir = os.path.join(cwd, 'docs', 'adr')

    os.makedirs(adr_dir, exist_ok=True)

    today = date.today().isoformat()
    filename = 'ADR-001-inicio-do-projeto.md'
    filepath = os.path.join(adr_dir, filename)

    # Idempotente: não sobrescreve se já existir
    if os.path.exists(filepath):
        return

    content = f"""---
name: ADR-001-inicio-do-projeto
title: "Início do projeto"
status: Proposed
date: {today}
---

# ADR-001: Início do projeto

## Status
Proposed

## Context
<!-- Descreva o contexto e o problema que motivou esta decisão -->

## Decision
<!-- Descreva a decisão tomada -->

## Consequences
<!-- Descreva as consequências desta decisão -->
"""

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)

    rel = os.path.relpath(filepath, cwd)
    print(f'  checkmark {rel}')
