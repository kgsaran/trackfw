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

RULES_START = '<!-- trackfw:rules:start -->'
RULES_END = '<!-- trackfw:rules:end -->'

AGENT_FILES = {
    'claude':   'CLAUDE.md',
    'codex':    'AGENTS.md',
    'gemini':   'GEMINI.md',
    'copilot':  '.github/copilot-instructions.md',
    'windsurf': '.windsurfrules',
    'amazonq':  '.amazonq/developer/guidelines.md',
    'cursor':   '.cursor/rules/trackfw.mdc',
}

AGENT_HEADERS = {
    'claude':   '# Project Instructions\n',
    'codex':    '# Project Instructions\n',
    'gemini':   '# Project Instructions\n',
    'copilot':  '# GitHub Copilot Instructions\n',
    'windsurf': '# Windsurf Rules\n',
    'amazonq':  '# Amazon Q Developer Guidelines\n',
    'cursor':   '---\ndescription: trackfw governance rules\nglob: "**/*"\nalwaysApply: true\n---\n',
}

GOV_DIRS_FLAT = [
    'docs/adr',
    'docs/req',
    'docs/roadmaps/backlog',
    'docs/roadmaps/analyzing',
    'docs/roadmaps/wip',
    'docs/roadmaps/blocked',
    'docs/roadmaps/done',
    'docs/roadmaps/abandoned',
]

ROADMAP_STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']


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


# ---------------------------------------------------------------------------
# trackfw rules inject-or-update
# ---------------------------------------------------------------------------

def _trackfw_rules_block() -> str:
    return (
        RULES_START + '\n'
        '## trackfw — Governance Rules\n\n'
        'This project uses **trackfw** for AI-native delivery governance.\n'
        'Chain: `ADR → REQ → ROADMAP` · States: `backlog / analyzing / wip / blocked / done / abandoned`\n\n'
        '### Roadmap State Lifecycle\n'
        '```\n'
        'backlog     → roadmaps awaiting execution\n'
        'analyzing   → roadmap under analysis/validation before wip\n'
        'wip         → roadmap in active execution (max 1)\n'
        'blocked     → blocked by dependency or decision\n'
        'done        → completed and validated\n'
        'abandoned   → discontinued (requires reason + successor)\n'
        '```\n\n'
        '### Agent Protocol\n'
        '1. **Before starting:** run `trackfw context` · read `docs/agents-working-context.md`\n'
        '2. **After finishing:** update `docs/agents-working-context.md` with what changed\n'
        '3. **Before PR:** `trackfw validate` must pass\n'
        '4. **ML lifecycle — mandatory:**\n'
        '   - When **starting** a ML: edit the roadmap changing `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` and commit the roadmap.\n'
        '   - When **completing** a ML: edit the roadmap changing `**Status:** 🔄 Em andamento` → `**Status:** ✅ Concluído` and include this change in the ML commit.\n'
        '   - When **analyzing** a roadmap before starting: move the file from `backlog/` to `analyzing/`; only move to `wip/` when actually starting to code.\n\n'
        '### Key Commands\n'
        '- `trackfw context` — current governance state (always run first)\n'
        '- `trackfw status` — all artifacts and states\n'
        '- `trackfw validate` — governance consistency check\n'
        '- `trackfw roadmap move <name> <state>` — transition roadmap state (valid: backlog, analyzing, wip, blocked, done, abandoned)\n'
        '- `trackfw serve` — live Kanban board at http://localhost:4080\n\n'
        '### Attention Signal (when you need user input during a task)\n'
        'Write `docs/roadmaps/.trackfw-attention.json`:\n'
        '```json\n'
        '{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}\n'
        '```\n'
        'Delete the file when resolved. Visible as a live banner in `trackfw serve`.\n'
        + RULES_END
    )


def _inject_or_update_rules(file_path: str, header_if_new: str) -> None:
    os.makedirs(os.path.dirname(os.path.abspath(file_path)), exist_ok=True)

    block = _trackfw_rules_block()

    if not os.path.exists(file_path):
        content = header_if_new or ''
        if content and not content.endswith('\n'):
            content += '\n'
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return

    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    start = content.find(RULES_START)
    if start == -1:
        if not content.endswith('\n'):
            content += '\n'
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return

    end = content.find(RULES_END, start)
    if end == -1:
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return

    new_content = content[:start] + block + content[end + len(RULES_END):]
    with open(file_path, 'w', encoding='utf-8') as f:
        f.write(new_content)


def inject_rules_for_tool(tool: str, cwd: str) -> None:
    rel_path = AGENT_FILES.get(tool)
    if not rel_path:
        return
    header = AGENT_HEADERS.get(tool, '')
    _inject_or_update_rules(os.path.join(cwd, rel_path), header)


def inject_rules_detected(cwd: str) -> None:
    for tool, rel_path in AGENT_FILES.items():
        if tool == 'cursor':
            if os.path.isdir(os.path.join(cwd, '.cursor')):
                try:
                    inject_rules_for_tool('cursor', cwd)
                except Exception:
                    pass
            continue
        if os.path.exists(os.path.join(cwd, rel_path)):
            try:
                inject_rules_for_tool(tool, cwd)
            except Exception:
                pass
