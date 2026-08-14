"""
generators/init_gen.py — scaffold de governança trackfw em Python puro.
Espelha npm/src/generators/init.js com suporte a namespacing flat e by_agent.
Depende apenas de stdlib.
"""

import os
import stat
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
    'vault/notes',
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

    generate_vault_index(cwd)
    _write_trackfw_yaml(cwd, opts)
    _write_example_adr(cwd, opts)
    generate_claude_md(cwd, opts)
    generate_claude_commands(cwd)
    generate_validate_script(cwd)
    _generate_attention_scripts(cwd)
    _generate_credential_guard_script(cwd)
    _generate_git_branch_guard_script(cwd)
    try:
        from trackfw.generators.hooks import inject_hooks_detected
        inject_hooks_detected(cwd)
    except Exception as e:
        print(f'  ⚠ agent hooks: {e}')
    print_architect_next_steps(cwd)


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

    forge = opts.get('forge', '')
    if forge:
        lines.append(f'forge: {forge}')

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

GLOBAL_ADR_DIRECTIVE = (
    'Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs '
    '(inclusive caminhos ~/...) antes de propor alterações de arquitetura.'
)


def _trackfw_rules_block() -> str:
    return (
        RULES_START + '\n'
        '## trackfw — Governance Rules\n\n'
        'This project uses **trackfw** for AI-native delivery governance.\n'
        'Chain: `ADR → REQ → ROADMAP` · States: `backlog / analyzing / wip / blocked / done / abandoned`\n\n'
        '### Agent Protocol\n'
        '1. **Before any implementation (mandatory):** create governance artifacts FIRST, then branch:\n'
        '   `trackfw req new "title"` → `trackfw roadmap new "title"` → `trackfw roadmap move <name> wip` → `git checkout -b feat/<branch>`\n'
        '   ❌ Never create a branch before REQ + ROADMAP are in wip/\n'
        '   ❌ Never defer REQ/ROADMAP creation to a future task — they are prerequisites, not deliverables\n'
        '   ✓ `trackfw validate` enforces this via `branch_has_wip_roadmap` rule (v2.7.0+)\n'
        '2. **Before starting:** run `trackfw context` · read `docs/agents-working-context.md`\n'
        '3. **After finishing:** update `docs/agents-working-context.md` with what changed\n'
        '4. **Before PR:** `trackfw validate` must pass\n'
        '5. **ML lifecycle — mandatory:**\n'
        '   - Starting a ML: edit roadmap `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` + commit.\n'
        '   - Completing a ML: edit roadmap → `**Status:** ✅ Concluído` + include in ML commit.\n'
        '   - Analyzing a roadmap: move from `backlog/` to `analyzing/`; to `wip/` only when coding starts.\n'
        f'6. **{GLOBAL_ADR_DIRECTIVE}**\n\n'
        '### Attention Signal (when you need user input during a task)\n'
        'Write `docs/roadmaps/.trackfw-attention.json`:\n'
        '```json\n'
        '{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}\n'
        '```\n'
        'Delete the file when resolved. Visible as a live banner in `trackfw serve`.\n\n'
        '> **Windsurf users:** before asking the user a question or requesting approval, write\n'
        '> `<roadmap_dir>/.trackfw-attention.json` manually — there is no automatic hook for this.\n'
        '> Delete the file after the user responds.\n'
        '\n### Architecture Directives (mandatory)\n'
        '- **3-layer separation:** frontend / backend / database — never mix concerns\n'
        '- **No in-memory data:** always database + ORM (never arrays/globals for persistence)\n'
        '- **Auth from day 1:** never defer — refactoring auth later is very costly\n'
        '- **Docker + .env from day 1:** containerize early; all config via env vars\n'
        '- **2-layer validation:** frontend (UX) + backend (security) — never only one\n'
        '- **API-first:** define OpenAPI contract before coding frontend/backend integration\n'
        '- **Security wave:** include a red-team review wave in every feature roadmap\n'
        '- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)\n'
        '- Use `/trackfw:architect` to define stack before the first REQ\n'
        '\n### Key Commands\n'
        '- `trackfw context` — current governance state (always run first)\n'
        '- `trackfw status` — all artifacts and states\n'
        '- `trackfw validate` — governance consistency check\n'
        '- `trackfw roadmap move <name> <state>` — transition roadmap state\n'
        '- `trackfw serve` — live Kanban board at http://localhost:4080\n'
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


def generate_claude_md(cwd: str, opts: dict) -> None:
    """
    Gera CLAUDE.md com harness completo de governança + 9 seções de harness pessoal.
    Espelha generateClaudeMD de Go e Node.js.
    """
    today = date.today().isoformat()
    project_name = opts.get('project_name') or 'my-project'

    lines = []
    lines.append(f'# {project_name} — Claude Code Instructions\n')
    lines.append(f'\n> Generated by trackfw on {today}. Update this file as the project evolves.\n')
    lines.append('\n## Project overview\n')
    lines.append('\n<!-- Describe what this project does in 2-3 sentences. -->\n')
    lines.append('\n## Governance chain\n')
    lines.append('\n```\nADR → REQ → ROADMAP → backlog / analyzing / wip / blocked / done / abandoned\n```\n')
    lines.append('\n## Agent rules (mandatory)\n')
    lines.append('\nThese rules apply to every agent or AI assistant working in this project:\n')
    lines.append('\n1. **Never start coding without a REQ and a ROADMAP.** If none exists, create them first.\n')
    lines.append('2. **Use `/trackfw:implement <req-slug>` to start any implementation.** This skill orchestrates the full flow automatically: finds or generates the roadmap, moves it to `wip/`, executes each ML, updates the roadmap, and moves to `done/`.\n')
    lines.append('3. **Only one roadmap in `wip/` at a time.** Before starting a new one, complete or move to `blocked/` the current one.\n')
    lines.append('4. **ML lifecycle — mandatory:** When starting a ML, change `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` and commit the roadmap. When completing, change to `**Status:** ✅ Concluído` and include in the ML commit. When analyzing a roadmap before starting, move it from `backlog/` to `analyzing/`; only move to `wip/` when actually coding.\n')
    lines.append('5. **Run `trackfw validate` before every commit.** Zero violations required.\n')
    lines.append('6. **ADRs before decisions.** Any architectural or technical decision must have an ADR (`/trackfw:adr`).\n')
    lines.append('6a. **Usar `/trackfw:architect` para definir stack e arquitetura antes da primeira REQ.**\n')
    lines.append(f'7. **{GLOBAL_ADR_DIRECTIVE}**\n')
    lines.append('\n## Slash commands (Claude Code)\n')
    lines.append('\n| Command | When to use |\n')
    lines.append('|---|---|\n')
    lines.append('| `/trackfw:implement <req>` | **Start here** — orchestrates full implementation flow |\n')
    lines.append('| `/trackfw:adr <title>` | Before any architectural decision |\n')
    lines.append('| `/trackfw:req <title>` | Before any implementation work |\n')
    lines.append('| `/trackfw:roadmap <req>` | Generate AI roadmap from a REQ |\n')
    lines.append('| `/trackfw:move <name> <state>` | Move roadmap between states manually |\n')
    lines.append('| `/trackfw:validate` | Run governance validation |\n')
    lines.append('| `/trackfw:status` | Check what is in flight |\n')
    lines.append('| `/trackfw:architect` | Guide stack and architecture decisions |\n')
    lines.append('| `/trackfw:barrier` | Run the wave-release checklist before liberating the next wave |\n')
    lines.append('\n## CLI commands (terminal / CI)\n')
    lines.append('\n| Command | When to use |\n')
    lines.append('|---|---|\n')
    lines.append('| `trackfw adr new "title"` | Create ADR |\n')
    lines.append('| `trackfw req new "title"` | Create REQ |\n')
    lines.append('| `trackfw roadmap new` | Create empty roadmap linked to a REQ |\n')
    lines.append('| `trackfw roadmap move <name> <state>` | Move roadmap state |\n')
    lines.append('| `trackfw validate` | Governance validation gate |\n')
    lines.append('| `trackfw status` | Show governance status |\n')
    lines.append('\n## Architecture Directives (mandatory)\n')
    lines.append('\n1. **3-layer separation** — frontend / backend / database. Never mix concerns.\n')
    lines.append('2. **No in-memory data** — always database + ORM. Never arrays/globals for persistence.\n')
    lines.append('3. **Auth from day 1** — never defer; refactoring auth later is very costly.\n')
    lines.append('4. **Docker + .env from day 1** — containerize early; all config via env vars, never hardcoded.\n')
    lines.append('5. **2-layer validation** — frontend (UX feedback) + backend (security guard). Never only one.\n')
    lines.append('6. **API-first** — define OpenAPI contract before coding frontend/backend integration.\n')
    lines.append('7. **Security wave** — include a red-team review wave at the end of every feature roadmap.\n')
    lines.append('8. **Test coverage** — TDD for critical business logic; min 60% (prototype) / 80% (production).\n')
    lines.append('\n## Pre-commit checklist\n')
    lines.append('\nBefore every commit:\n')
    lines.append('- [ ] `trackfw validate` passes with zero violations\n')
    lines.append('\n## Git hooks\n')
    lines.append('\nNo git hook configured. Run `trackfw validate` manually before every commit.\n')
    lines.append('\n## CI gate\n')
    lines.append('\nNo CI gate configured.\n')

    # Harness sections — derived from project governance conventions
    lines.append('\n## Branch strategy\n')
    lines.append('\nOne active branch at a time. Name it `feat/<slug>`, `fix/<slug>` or `refactor/<slug>`. ')
    lines.append('Before creating a new branch, verify no other is genuinely open: run `git fetch origin --prune`, ')
    lines.append('then `git branch -r --no-merged origin/main`, then for each candidate `git diff origin/main <branch> --stat`. ')
    lines.append('An empty diff means it was squash-merged — ignore it. ')
    lines.append('Squash merges do not mark a branch as merged, so `--no-merged` alone is not evidence. ')
    lines.append('If the branch is stale and the diff looks inflated by main\'s own evolution, ')
    lines.append('compare only the files the branch itself touched since the merge base.\n')
    lines.append('\n## Definition of done\n')
    lines.append('\nGreen build and tests do not close a microbatch. ')
    lines.append('It is done when the requirement and the roadmap sit in the correct state folder, ')
    lines.append('their declared status matches that folder, the final validation is recorded with evidence, ')
    lines.append('no duplicate copy remains in another state, and `trackfw validate` reports no violations.\n')
    lines.append('\n## Requirement scope\n')
    lines.append('\nEvery requirement must declare an explicit negative scope: what must not be implemented. ')
    lines.append('Boundaries prevent an implementing agent from inventing work.\n')
    lines.append('\n## State requirements\n')
    lines.append('\n`blocked` requires a reason and an owner. ')
    lines.append('`abandoned` requires a reason and a successor. ')
    lines.append('`wip` must reflect work that is genuinely active; ')
    lines.append('anything stalled moves to `blocked` or `abandoned` instead of rotting in `wip`.\n')
    lines.append('\n## Roadmap format\n')
    lines.append('\nOrganize work as waves of microbatches. ')
    lines.append('A wave groups microbatches that can run in parallel; a barrier separates waves. ')
    lines.append('Microbatches sharing any file — including generated trees and build outputs — must be sequential, ')
    lines.append('and the reason is documented. ')
    lines.append('Each microbatch declares exact files, exact actions, measurable acceptance criteria and exact validation commands, ')
    lines.append('so that a small model can execute it without guessing.\n')
    lines.append('\n## When governance is not required\n')
    lines.append('\nA closed list of exemptions: a typo or local variable rename; a documentation-only change; ')
    lines.append('a configuration tweak with no runtime effect; a direct revert; ')
    lines.append('answering a question or reviewing without changes. ')
    lines.append('Additionally, when the user reports a concrete bug, fix it directly and do not open an architectural analysis for it. ')
    lines.append('**This section takes precedence over the general rule that requires a requirement and a roadmap.** ')
    lines.append('Anything touching business logic, an API contract, a data schema, authentication or authorization, ')
    lines.append('localization, or user-facing behavior always requires governance, regardless of how few files it touches.\n')
    lines.append('\n## Production incidents\n')
    lines.append('\nInspect the live environment before proposing a fix: real variables, active credentials, ')
    lines.append('granted permissions, running processes. ')
    lines.append('Confirm the root cause against real evidence, then implement the smallest fix. ')
    lines.append('Never edit static configuration files as a response to a root cause that has not been confirmed in the running environment.\n')
    lines.append('\n## Iterative prototyping\n')
    lines.append('\nFor complex or uncertain user-facing work, validate the concept with a disposable, isolated prototype ')
    lines.append('that the user reviews visually, and only then write the decision record and the production roadmap. ')
    lines.append('Build and test success is not evidence that an interface is right.\n')
    lines.append('\n## Autopilot\n')
    lines.append('\nAsk everything you need before starting. ')
    lines.append('Once started, do not interrupt for confirmations that could have been anticipated. ')
    lines.append('Decide low-risk details autonomously following existing project conventions, ')
    lines.append('and record autonomous decisions in the commit message.\n')

    header = ''.join(lines)
    _inject_or_update_rules(os.path.join(cwd, 'CLAUDE.md'), header)
    print('  checkmark CLAUDE.md')


def generate_validate_script(cwd: str) -> None:
    """Escreve scripts/trackfw-validate.sh.

    This is the SINGLE canonical generator for this file in the Python
    runtime — both `trackfw init` (via scaffold(), above) and `trackfw
    update`'s `validate-script` target (pypi/trackfw/commands/update.py)
    call this same function. Previously this file was written only by the
    `discover` command's own private `_write_validate_script` (never by
    `init`), so a freshly-`init`-ed Python project had no
    scripts/trackfw-validate.sh at all and `trackfw update` always reported
    it `missing` — diverging in target-count AND state from the Go and
    Node.js CLIs, which both write this file at init time (ML-6H,
    docs/cli-parity.md, "Declared project targets — pinned list").

    Unlike Go's/Node's per-backend script (buildValidateScript), this
    runtime's `init` has no --backend/--frontend/--pkg-manager flags (a
    pre-existing, intentionally reduced Python `init` CLI surface — see
    trackfw/commands/init.py), so the generated script is intentionally the
    simpler, backend-agnostic form. Only the update-state contract (missing/
    skipped/updated/failed) and the JSON document shape are pinned across
    runtimes for this target — the script's own bytes are not (see
    docs/cli-parity.md's declared-targets note on Python's reduced surface).
    """
    scripts_dir = os.path.join(cwd, 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)
    content = "#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n"
    dest = os.path.join(scripts_dir, 'trackfw-validate.sh')
    with open(dest, "w", encoding="utf-8") as f:
        f.write(content)
    os.chmod(dest, 0o755)
    print('  checkmark scripts/trackfw-validate.sh')


def generate_claude_commands(cwd: str) -> None:
    """Instala os slash commands do trackfw em .claude/commands/trackfw/."""
    cmd_dir = os.path.join(cwd, '.claude', 'commands', 'trackfw')
    os.makedirs(cmd_dir, exist_ok=True)

    _install_not_found = (
        '\n\nSe o comando falhar com `trackfw: command not found` ou similar, informe ao usuário:\n\n'
        '```\n'
        'trackfw não está instalado. Instale com uma das opções:\n\n'
        '  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh\n'
        '  npm install -g trackfw\n'
        '  pip install trackfw\n'
        '```'
    )

    commands = {
        'adr.md': (
            'Execute o seguinte comando bash: `trackfw adr new "$ARGUMENTS"`'
            + _install_not_found
        ),
        'req.md': (
            'Execute o seguinte comando bash: `trackfw req new "$ARGUMENTS"`'
            + _install_not_found
        ),
        'validate.md': (
            'Execute o seguinte comando bash: `trackfw validate`'
            + _install_not_found
        ),
        'status.md': (
            'Execute o seguinte comando bash: `trackfw status`'
            + _install_not_found
        ),
        'move.md': (
            'Execute o seguinte comando bash: `trackfw roadmap move $ARGUMENTS`\n\n'
            'O formato esperado é: `<nome-do-roadmap> <estado>`\n\n'
            'Estados válidos: `backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`\n\n'
            'Exemplo: `/trackfw:move meu-roadmap analyzing`\n\n'
            'Se o comando falhar com `trackfw: command not found` ou similar, informe ao usuário:\n'
            'trackfw não está instalado. Instale com:\n'
            '  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh\n'
            '  npm install -g trackfw\n'
            '  pip install trackfw'
        ),
        'roadmap.md': (
            'Gere um roadmap de implementação em microlotes para uma REQ do projeto.\n\n'
            '## Passos\n\n'
            '1. **Listar REQs disponíveis**\n'
            '   Use Glob para listar `docs/req/*.md`. Se nenhum arquivo encontrado, informe:\n'
            '   > Nenhuma REQ encontrada em `docs/req/`. Crie uma primeiro com `/trackfw:req`.\n\n'
            '2. **Selecionar a REQ**\n'
            '   - Se `$ARGUMENTS` foi fornecido: use como filtro (substring case-insensitive) para encontrar o arquivo\n'
            '   - Se não foi fornecido ou o filtro não encontrar exatamente um: liste os arquivos disponíveis e pergunte ao usuário qual usar\n'
            '   - Leia o conteúdo completo do arquivo REQ selecionado\n\n'
            '3. **Gerar o roadmap**\n'
            '   Com base no conteúdo da REQ, gere um roadmap seguindo **estritamente** este formato:\n\n'
            '   ```markdown\n'
            '   ---\n'
            '   status: backlog\n'
            '   date: <YYYY-MM-DD>\n'
            '   req: "docs/req/<arquivo-selecionado>.md"\n'
            '   squad: ""\n'
            '   ---\n\n'
            '   # Roadmap: <título derivado da REQ>\n\n'
            '   > Created: <YYYY-MM-DD> | Status: backlog\n\n'
            '   ## Diagnóstico / Contexto\n'
            '   <resumo do problema, motivação e escopo extraídos da REQ>\n\n'
            '   ## Wave 1 — <nome descritivo> (<N> MLs em paralelo)\n'
            '   > Dependências: Independente\n\n'
            '   ### ML-1A — <título>\n'
            '   **Status:** ⬜ Pendente\n'
            '   **Arquivos afetados:**\n'
            '   - `caminho/exato/do/arquivo`\n'
            '   **Ações:**\n'
            '   - Descrição detalhada da ação com valores, chaves e comandos exatos\n'
            '   **Critérios de aceite:**\n'
            '   - [ ] build sem erros\n'
            '   - [ ] testes verdes\n'
            '   **Comandos de validação:** `<comando de build e teste do projeto>`\n'
            '\n'
            '   ### ML-1B — <título> (se independente de ML-1A)\n'
            '   ...\n\n'
            '   ## Wave 2 — <nome> (depende de Wave 1)\n'
            '   > Dependências: Wave 1 completa\n'
            '   ...\n'
            '   ```\n\n'
            '   **Princípios obrigatórios:**\n'
            '   - MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)\n'
            '   - Cada ML deve ser detalhado o suficiente para execução por um agente sem contexto extra\n'
            '   - Maximizar paralelismo: agrupe em paralelo tudo que não compartilhar arquivos\n'
            '   - Waves sequenciais apenas quando há dependência real de resultado\n'
            '   - Critérios de aceite mensuráveis em cada ML\n\n'
            '4. **Salvar o arquivo**\n'
            '   - Calcule o slug: título em lowercase, espaços → hifens, remova caracteres especiais\n'
            '   - Crie o arquivo em `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`\n'
            '   - Preencha `req:` com o caminho relativo completo da REQ selecionada\n'
            '   - Use a data de hoje\n\n'
            '5. **Confirmar**\n'
            '   Informe o caminho do arquivo criado e um resumo das Waves e total de MLs gerados.\n'
        ),
        'implement.md': (
            'Você é o orquestrador de implementação do trackfw. Siga o fluxo abaixo **sem pular etapas**.\n\n'
            '## Argumento\n\n'
            '`$ARGUMENTS` é opcional. Se fornecido, é usado como filtro (substring case-insensitive) sobre os nomes de arquivo das REQs.\n\n'
            '---\n\n'
            '## Passo 1 — Selecionar a REQ\n\n'
            'Use Glob para listar `docs/req/*.md`.\n\n'
            '- Se **nenhum arquivo encontrado**: informe que não há REQs disponíveis e sugira criar com `/trackfw:req`.\n'
            '- Se **`$ARGUMENTS` foi fornecido** e filtra para exatamente uma REQ: use-a diretamente.\n'
            '- Em **todos os outros casos** (sem argumento, ou argumento ambíguo): apresente a lista de REQs disponíveis e pergunte ao usuário qual deseja implementar.\n\n'
            'Leia o conteúdo completo da REQ selecionada.\n\n'
            '---\n\n'
            '## Passo 2 — Encontrar ou gerar o Roadmap\n\n'
            'Verifique se existe um roadmap vinculado à REQ buscando em `docs/roadmaps/` (backlog, wip, blocked, done, abandoned) por arquivo cujo nome contenha o slug da REQ.\n\n'
            '**Se o roadmap ainda não existe:**\n'
            '- Informe o usuário: "Nenhum roadmap encontrado para esta REQ. Gerando agora..."\n'
            '- Execute o fluxo completo de geração do `/trackfw:roadmap` (leia o arquivo `.claude/commands/trackfw/roadmap.md` para seguir as instruções exatas), passando a REQ já selecionada — não pergunte novamente.\n'
            '- Salve o roadmap gerado em `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`.\n\n'
            '**Se o roadmap existe e já está em `done/` ou `abandoned/`:**\n'
            '- Informe o usuário e pergunte se deseja criar um novo roadmap ou encerrar.\n\n'
            '**Se o roadmap existe em `backlog/` ou `blocked/`:**\n'
            '- Prossiga para o Passo 3.\n\n'
            '**Se já está em `wip/`:**\n'
            '- Prossiga diretamente para o Passo 4 (já está em execução).\n\n'
            '---\n\n'
            '## Passo 3 — Mover roadmap para WIP\n\n'
            'Execute:\n'
            '```bash\n'
            'trackfw roadmap move <nome-do-roadmap> wip\n'
            '```\n\n'
            'Confirme que o arquivo foi movido para `docs/roadmaps/wip/`.\n\n'
            '---\n\n'
            '## Passo 4 — Ler e apresentar o plano\n\n'
            'Leia o roadmap (agora em `wip/`). Apresente ao usuário:\n'
            '- Título do roadmap\n'
            '- Total de Waves e MLs\n'
            '- Lista resumida dos MLs por Wave\n\n'
            'Confirme: "Iniciando implementação. Vou executar cada ML em ordem e atualizar o roadmap a cada conclusão."\n\n'
            '---\n\n'
            '## Passo 5 — Executar cada ML em ordem\n\n'
            'Para cada Wave (em sequência), execute os MLs da Wave:\n\n'
            '### Para cada ML:\n\n'
            '**5a. Anunciar:** informe qual ML está sendo executado (ex: "Executando ML-1A — Criar client.go").\n\n'
            '**5b. Implementar:** execute as ações descritas no ML usando suas ferramentas (Read, Write, Edit, Bash). Siga exatamente os arquivos afetados, ações e critérios de aceite listados no roadmap.\n\n'
            '**5c. Validar:** execute os comandos de validação do ML. Se falhar, corrija antes de avançar.\n\n'
            '**5d. Atualizar o roadmap:** edite o arquivo de roadmap em `docs/roadmaps/wip/` substituindo o status do ML:\n'
            '- `**Status:** ⬜ Pendente` → `**Status:** ✅ Concluído`\n\n'
            '**5e. Commitar:**\n'
            '```bash\n'
            'git add -A\n'
            'git commit -m "feat(<escopo>): <descrição do ML>"\n'
            '```\n\n'
            'Só avance para a próxima Wave após todos os MLs da Wave atual estarem ✅.\n\n'
            '---\n\n'
            '## Passo 6 — Finalizar\n\n'
            'Quando todos os MLs estiverem ✅:\n\n'
            '**6a.** Execute `trackfw validate` — deve passar com zero violations.\n\n'
            '**6b.** Mova o roadmap para done:\n'
            '```bash\n'
            'trackfw roadmap move <nome-do-roadmap> done\n'
            '```\n\n'
            '**6c.** Faça o commit final:\n'
            '```bash\n'
            'git add docs/roadmaps/\n'
            'git commit -m "docs(trackfw): roadmap <nome> → done"\n'
            '```\n\n'
            '**6d.** Informe o usuário:\n'
            '```\n'
            '✅ Implementação concluída.\n'
            'Roadmap: docs/roadmaps/done/<nome>.md\n'
            'Próximo passo: abrir PR com gh pr create\n'
            '```'
        ),
        'barrier.md': (
            'Você é o `trackfw_architect`, a única autoridade Git deste projeto. Este comando executa o checklist operacional de liberação de uma wave — nenhum outro agente commita, faz push ou libera a próxima wave.\n\n'
            '## Argumento\n\n'
            '`$ARGUMENTS` no formato `<roadmap> <wave>`. Se ausente ou incompleto, pergunte ao usuário qual roadmap (em `docs/roadmaps/wip/`) e qual número de wave validar.\n\n'
            '---\n\n'
            '## Núcleo determinístico\n\n'
            'Execute primeiro:\n'
            '```bash\n'
            'trackfw barrier <roadmap> --wave <n> --json\n'
            '```\n\n'
            'Este comando é **necessário mas não suficiente**. Ele verifica MLs concluídos, evidências e `trackfw validate`, mas não substitui as inspeções especializadas nem a auditoria de diff abaixo — nenhuma delas é avaliada pelo binário. Consulte a seção `trackfw barrier` em `docs/cli-parity.md` para o contrato completo (estados, exit codes, saída JSON).\n\n'
            'Se o comando retornar exit code não-zero (`blocked` ou erro de resolução): pare, reporte a falha ao usuário e não prossiga no checklist até que a wave passe.\n\n'
            '---\n\n'
            '## Definição de pronto da barrier — checklist completo\n\n'
            'Antes de liberar a próxima wave, confirme cada item com evidência concreta — não presuma:\n\n'
            '1. **Todos os MLs da wave concluídos e marcados** — cada ML da wave está com `**Status:** ✅ Concluído` no roadmap.\n'
            '2. **Testes unitários e E2E aplicáveis executados** — rode os comandos de validação declarados em cada ML.\n'
            '3. **Build aplicável sem erros** — rode o comando de build do(s) workspace(s) afetado(s).\n'
            '4. **Cada critério de aceite inspecionado com evidência** — leia os arquivos modificados e confirme contra os critérios listados, não apenas contra os testes.\n'
            '5. **Agente code-quality reportou conformidade, performance, robustez e clareza** — invoque o agente `code-quality` quando a mudança introduzir lógica nova, duplicação relevante ou risco de manutenibilidade.\n'
            '6. **Agente security reportou SAST, privilégios, controle de acesso e camadas aplicáveis** — invoque o agente `security` quando a mudança tocar autenticação, segredos, entrada externa ou permissões.\n'
            '7. **Gates pré-commit declarados pelo projeto executados** — rode os hooks/gates configurados (lint, format, testes de contrato).\n'
            '8. **`trackfw validate --json` aprovado** — execute e confirme zero violações.\n'
            '9. **Diff auditado contra o escopo** — revise o diff completo; confirme que não há alterações de agentes concorrentes nem arquivos fora do escopo do ML (ex: `docs/adr/`, `docs/req/`, `docs/roadmaps/` quando não autorizado ao especialista).\n'
            '10. **Resultado registrado antes de liberar a próxima wave** — anote no roadmap ou na resposta ao usuário que a wave passou, com a evidência de cada item acima.\n\n'
            'Se qualquer item falhar: bloqueie a próxima wave, identifique o item e o agente responsável, e despache um microlote corretivo. Só repita o checklist depois que o corretivo for concluído.\n\n'
            '---\n\n'
            '## Autoridade Git\n\n'
            'Somente o `trackfw_architect` cria branch, audita diff, commita e faz push. Especialistas entregam trabalho sem commit — cabe a este papel revisar, commitar e sugerir a abertura de PR/MR (sem abrir automaticamente sem autorização do usuário).\n'
        ),
        'architect.md': (
            'Você é o guia de arquitetura do trackfw. Ajude o usuário a escolher a stack correta e arquitetar a aplicação em linguagem simples, acessível para times não técnicos.\n\n'
            '## Passo 1 — Descoberta de Negócio\n\n'
            'Faça ao usuário as seguintes perguntas em linguagem simples, uma por vez:\n\n'
            '1. "O que sua aplicação vai fazer? Descreva em 2-3 frases como se fosse explicar para alguém de fora da TI."\n'
            '2. "Quantas pessoas vão usar esse sistema simultaneamente? (< 10 pessoas / 10-100 pessoas / > 100 pessoas)"\n'
            '3. "Esse sistema vai para produção de verdade ou é um protótipo para validar uma ideia?"\n'
            '4. "Você precisa de login/autenticação de usuários? (Sim / Não / Não sei)"\n'
            '5. "Tem alguma restrição de tecnologia ou preferência da empresa? (ex: só Java, só Microsoft, etc.)"\n\n'
            '---\n\n'
            '## Passo 2 — Recomendação de Stack\n\n'
            'Com base nas respostas, escolha **UM** dos combos pré-validados:\n\n'
            '### Combo A — Protótipo Rápido\n'
            '**Quando usar:** prototipagem, validação de ideia, até ~10 usuários, sem pressão de produção.\n'
            '- **Frontend:** React + Vite\n'
            '- **Backend:** FastAPI (Python) ou Express (Node.js)\n'
            '- **Banco:** SQLite + SQLAlchemy / Prisma\n'
            '- **Auth:** JWT simples quando necessário\n'
            '- **Docker:** Dockerfile básico para o backend\n\n'
            '### Combo B — Sistema Pequeno/Médio em Produção\n'
            '**Quando usar:** sistema real, 10-100 usuários, robustez e manutenibilidade.\n'
            '- **Frontend:** Next.js (SSR + rotas prontas)\n'
            '- **Backend:** FastAPI (Python) ou NestJS (Node.js)\n'
            '- **Banco:** PostgreSQL + ORM (SQLAlchemy / Prisma / TypeORM)\n'
            '- **Auth:** OAuth2 com JWT (Supabase Auth ou Auth0)\n'
            '- **Docker:** docker-compose com frontend + backend + banco\n\n'
            '### Combo C — Enterprise / Java\n'
            '**Quando usar:** integração com sistemas corporativos, > 100 usuários, exigência de Java.\n'
            '- **Frontend:** Angular\n'
            '- **Backend:** Spring Boot\n'
            '- **Banco:** PostgreSQL + Hibernate\n'
            '- **Auth:** Spring Security + OAuth2 (Keycloak ou Azure AD)\n'
            '- **Docker:** docker-compose com todos os serviços\n\n'
            'Apresente o combo recomendado com explicação simples do motivo.\n\n'
            '---\n\n'
            '## Passo 3 — Arquitetura em Camadas (explicação simples)\n\n'
            'Explique a arquitetura com uma metáfora de negócio:\n\n'
            '"Pense na aplicação como um restaurante:\n'
            '- **Frontend** = o salão: o que o cliente vê e interage\n'
            '- **Backend** = a cozinha: onde as regras de negócio acontecem, nunca exposta diretamente\n'
            '- **Banco de dados** = a despensa: onde os dados ficam guardados, acessada só pela cozinha"\n\n'
            'Reforce as **Architecture Directives** já injetadas no CLAUDE.md deste projeto: separação em 3 camadas sem dados em memória (sempre DB + ORM), auth + Docker + .env desde o dia 1, validação em 2 camadas, contrato OpenAPI antes de codar, wave de segurança em todo roadmap e cobertura mínima de testes (60% protótipo / 80% produção).\n\n'
            '---\n\n'
            '## Passo 4 — Gerar o ADR de Stack\n\n'
            'Execute `/trackfw:adr` com o título: `"Stack e arquitetura em camadas — [nome do projeto]"`\n\n'
            'O ADR deve registrar a stack escolhida (combo e componentes), motivação baseada nas respostas, alternativas descartadas e princípios de arquitetura adotados.\n\n'
            '---\n\n'
            '## Passo 5 — Próximos Passos\n\n'
            'Oriente o usuário:\n\n'
            '```\n'
            '✅ Stack definida. Próximos passos:\n\n'
            '1. Crie a REQ da primeira feature com /trackfw:req\n'
            '2. Gere o roadmap em microlotes com /trackfw:roadmap\n'
            '3. Inicie a implementação com /trackfw:implement\n'
            '```'
        ),
    }

    created = 0
    skipped = 0
    for filename, content in commands.items():
        file_path = os.path.join(cmd_dir, filename)
        if os.path.exists(file_path):
            skipped += 1
            continue
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        created += 1

    if skipped > 0:
        print(f'  ✓ .claude/commands/trackfw/ ({created} slash commands criados, {skipped} já existiam)')
    else:
        print(f'  ✓ .claude/commands/trackfw/ ({created} slash commands)')


_ATTENTION_SIGNAL_SH = r"""#!/usr/bin/env bash
# trackfw attention signal — PreToolUse/BeforeTool hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // ""')
  MSG=$(echo "$INPUT" | jq -r '(.tool_input.question // .tool_input.command // "Agent is executing: \(.tool_name // "unknown")") | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

TOOL_ESC=$(echo "$TOOL" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$TOOL_ESC" \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
"""

_ATTENTION_CLEANUP_SH = r"""#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
set -euo pipefail

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
"""


# _CG_HEADER/_CG_PROJECT_GUARD/_CG_DETECTION_CORE/_CG_PROJECT_TAIL/_CG_GLOBAL_TAIL compõem
# _CREDENTIAL_GUARD_SH (escopo de projeto) e _GLOBAL_CREDENTIAL_GUARD_SH (escopo global,
# ~/.trackfw/scripts/, instalado via `trackfw update harness`) sem duplicar a lógica de
# detecção JWT/AWS-key em dois lugares — espelha a mesma decomposição em
# internal/generators/scaffold.go (credentialGuardHeader/credentialGuardDetectionCore/...).

_CG_HEADER = r"""#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

"""

_CG_PROJECT_GUARD = r"""# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

"""

_CG_DETECTION_CORE = r"""JWT_PATTERN='eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\$\{?[A-Za-z_][A-Za-z0-9_]*\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$/\1/')
    pattern="*${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

"""

# Resolução de MODE (grep de `credential_guard.mode` em trackfw.yaml + fallback) é replicada como
# texto literal idêntico em _CG_PROJECT_TAIL (fallback "warn") e _CG_GLOBAL_TAIL (fallback
# "block") -- não extraída para uma constante Python compartilhada e concatenada, ao contrário do
# Go (credentialGuardModeResolution): o gate de paridade Go/Node/Python
# (internal/generators/credential_guard_test.go, getPythonSourceBlock) extrai cada constante via
# regex de um único literal `NAME = r"""..."""` sem suportar concatenação de string -- concatenar
# quebraria a extração estática (mesma restrição documentada para Node, ver vault
# credential-guard-parity-test-extractor-rejects-string-concatenation-2026-08-08). Nunca editar a
# lógica de resolução em só um dos dois blocos sem replicar no outro.
_CG_PROJECT_TAIL = r"""DEFAULT_MODE="warn"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
"""

# _CG_GLOBAL_TAIL é a contraparte de _CG_PROJECT_TAIL para o escopo global
# (~/.trackfw/scripts/trackfw-credential-guard.sh, instalado via `trackfw update harness`).
#
# Decisão (ML-1C, ver ADR-2026-08-06 emenda 6 de 2026-08-08 e ROADMAP-2026-08-08, Wave 1): o modo
# em escopo global reusa a MESMA leitura de `credential_guard.mode` de trackfw.yaml que
# _CG_PROJECT_TAIL já faz (mesma resolução, replicada aqui -- ver o comentário de
# _CG_PROJECT_TAIL sobre por que não é extraída para uma constante compartilhada em Python) -- sem
# exigir trackfw.yaml existir (não há o guard `[ -f trackfw.yaml ] || exit 0` da variante de
# projeto: o objetivo do escopo global é proteger qualquer projeto, com ou sem trackfw.yaml).
# Quando o hook global roda a partir do cwd de um projeto com trackfw.yaml e
# credential_guard.mode explícito, esse valor é respeitado (warn ou block) -- nenhuma mudança de
# comportamento para quem já definiu mode: warn explicitamente. Em qualquer outro caso (sem
# trackfw.yaml, ou trackfw.yaml sem essa chave), o fallback deixa de ser "warn" e passa a ser
# "block": um guard opt-in que nunca bloqueia por padrão é uma falsa sensação de proteção -- o
# usuário que rodou `trackfw update harness` já demonstrou intenção explícita de ter o mecanismo
# ativo. Supersede a decisão original ("modo global sempre warn", opção "b" avaliada na ADR
# original) -- não cria ~/.trackfw/config.yaml nem nenhuma outra segunda fonte de configuração só
# para isto.
#
# ROADMAP_DIR em escopo global: como não há garantia de trackfw.yaml para ler `roadmap_dir:`, o
# script usa o caminho padrão fixo "docs/roadmaps" relativo ao cwd de onde o hook foi disparado, e
# só grava o attention signal se esse diretório já existir (e só em modo warn -- modo block nunca
# grava o attention signal, mesma decisão da variante de projeto). Não cria "docs/roadmaps" em um
# projeto aleatório só para sinalizar isso -- isso pareceria ao usuário que o trackfw foi
# "instalado" nesse projeto, o que não é verdade. O texto de warning/block em stderr acontece
# sempre (visível no output do CLI/hook), independente de o diretório de attention existir.
_CG_GLOBAL_TAIL = r"""DEFAULT_MODE="block"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR="docs/roadmaps"
if [ ! -d "$ROADMAP_DIR" ]; then
  exit 0
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
"""

_CREDENTIAL_GUARD_SH = _CG_HEADER + _CG_PROJECT_GUARD + _CG_DETECTION_CORE + _CG_PROJECT_TAIL

_GLOBAL_CREDENTIAL_GUARD_SH = _CG_HEADER + _CG_DETECTION_CORE + _CG_GLOBAL_TAIL


# _GIT_BRANCH_GUARD_SH — bloqueia `git commit`/`git push`/`git checkout -b` brutos por
# subagente (ML-3C, ROADMAP-2026-08-14, port de internal/generators/scaffold.go:gitBranchGuardScript).
#
# Ao contrário do credential-guard, o conteúdo é idêntico entre escopo de projeto e escopo
# global — não depende de trackfw.yaml (nenhuma leitura de credential_guard.mode/roadmap_dir):
# a detecção de `git commit`/`git push`/`git checkout -b` bruto e a mensagem de bloqueio são
# as mesmas em qualquer diretório. Por isso, ao contrário de _CREDENTIAL_GUARD_SH/
# _GLOBAL_CREDENTIAL_GUARD_SH (montados de blocos Header/ProjectGuard/DetectionCore/Tail
# distintos por escopo), aqui um único literal serve os dois pontos de geração — mesma
# decisão de design do Go (ver doc comment de GenerateGlobalGitBranchGuardScript em
# internal/generators/scaffold.go: "as duas funções existem separadamente só para espelhar
# o par Generate*/GenerateGlobal* já estabelecido pelo credential-guard").
#
# Raw string (r"""...""") é obrigatório aqui: o corpo contém `\`` (backtick escapado dentro
# de string bash entre aspas duplas, usado nas três mensagens REASON) que uma string Python
# não-raw interpretaria como sequência de escape inválida.
_GIT_BRANCH_GUARD_SH = r"""#!/usr/bin/env bash
# trackfw git branch guard — bloqueia git commit/push/checkout -b brutos por subagente
set -euo pipefail
set -f

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT=$(cat 2>/dev/null || true)
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"hook_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      ;;
    *)
      CMD_RAW="$INPUT"
      ;;
  esac
fi

if [ -z "$CMD_RAW" ]; then
  CMD_RAW="${TRACKFW_GIT_COMMAND:-}"
fi

[ -n "$CMD_RAW" ] || exit 0

# --- 2. Casar contra "git (commit|push|checkout -b)", aceitando flags antes ------------------
match_subcommand() {
  set -- $1
  found=0
  args=""
  for tok in "$@"; do
    if [ "$found" -eq 0 ]; then
      if [ "$tok" = "git" ]; then
        found=1
      fi
      continue
    fi
    args="$args $tok"
  done
  [ "$found" -eq 1 ] || return 1

  set -- $args
  sub=""
  while [ "$#" -gt 0 ]; do
    tok="$1"
    case "$tok" in
      -C|-c|--work-tree|--git-dir|--namespace)
        if [ "$#" -ge 2 ]; then shift 2; else shift; fi
        continue
        ;;
      -*)
        shift
        continue
        ;;
      *)
        sub="$tok"
        shift
        break
        ;;
    esac
  done

  case "$sub" in
    commit)
      echo "commit"
      return 0
      ;;
    push)
      echo "push"
      return 0
      ;;
    checkout)
      if [ "${1:-}" = "-b" ]; then
        echo "checkout-b"
        return 0
      fi
      ;;
  esac
  return 1
}

SUBCOMMAND=$(match_subcommand "$CMD_RAW") || exit 0

case "$SUBCOMMAND" in
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use \`trackfw commit -m '<mensagem>'\`. Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use \`trackfw ship\`. Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"decision":"block","reason":"%s"}\n' "$REASON"
echo "$REASON" >&2
exit 2
"""


def generate_vault_index(cwd: str) -> None:
    """Cria vault/notes/ e vault/notes/index.md se ainda não existirem."""
    vault_dir = os.path.join(cwd, 'vault', 'notes')
    os.makedirs(vault_dir, exist_ok=True)
    index_path = os.path.join(vault_dir, 'index.md')
    if os.path.exists(index_path):
        return
    content = (
        "# Vault de Conhecimento\n\n"
        "> Ponto de entrada de conhecimento do projeto para agentes e pessoas.\n"
        "> Cada nota documenta uma causa-raiz, decisão técnica ou restrição não óbvia.\n"
        "> Crie notas com: trackfw note new \"<título>\"\n\n"
        "## Índice\n\n"
        "<!-- As notas serão listadas abaixo. Exemplo:\n"
        "- [nome-da-nota-YYYY-MM-DD](nome-da-nota-YYYY-MM-DD.md)\n"
        "-->\n"
    )
    with open(index_path, 'w', encoding='utf-8') as f:
        f.write(content)
    print('  ✓ vault/notes/index.md')


def _generate_attention_scripts(cwd: str) -> None:
    """Gera scripts shell de attention signal/cleanup em scripts/."""
    scripts_dir = os.path.join(cwd, 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)

    signal_path = os.path.join(scripts_dir, 'trackfw-attention-signal.sh')
    with open(signal_path, 'w', encoding='utf-8') as f:
        f.write(_ATTENTION_SIGNAL_SH.lstrip('\n'))
    os.chmod(signal_path, 0o755)

    cleanup_path = os.path.join(scripts_dir, 'trackfw-attention-cleanup.sh')
    with open(cleanup_path, 'w', encoding='utf-8') as f:
        f.write(_ATTENTION_CLEANUP_SH.lstrip('\n'))
    os.chmod(cleanup_path, 0o755)


def _generate_credential_guard_script(cwd: str) -> None:
    """Gera o script shell trackfw-credential-guard.sh em scripts/.

    ML-1A apenas: cria o script. Nao o injeta em nenhum hooks.json/settings.json de CLI --
    isso e escopo da Wave 2 (ver ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-
    credenciais-reais-por-subagentes.md).
    """
    scripts_dir = os.path.join(cwd, 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)

    script_path = os.path.join(scripts_dir, 'trackfw-credential-guard.sh')
    with open(script_path, 'w', encoding='utf-8') as f:
        f.write(_CREDENTIAL_GUARD_SH.lstrip('\n'))
    os.chmod(script_path, 0o755)


def _generate_git_branch_guard_script(cwd: str) -> None:
    """Gera o script shell trackfw-git-branch-guard.sh em scripts/.

    Mesmo padrão de _generate_credential_guard_script: este ML (3C) só cria o script --
    não o injeta em nenhum hooks.json/settings.json de CLI sozinho (isso é feito por
    generators/hooks.py:inject_hooks_detected, que chama esta função e depois os
    injetores por runtime).
    """
    scripts_dir = os.path.join(cwd, 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)

    script_path = os.path.join(scripts_dir, 'trackfw-git-branch-guard.sh')
    with open(script_path, 'w', encoding='utf-8') as f:
        f.write(_GIT_BRANCH_GUARD_SH.lstrip('\n'))
    os.chmod(script_path, 0o755)


def generate_global_git_branch_guard_script(home: str) -> None:
    """Gera o script shell trackfw-git-branch-guard.sh em escopo global, em
    <home>/.trackfw/scripts/trackfw-git-branch-guard.sh.

    Destinado a ser referenciado por hooks globais de CLI (~/.claude/settings.json,
    ~/.gemini/settings.json etc.), instalados via `trackfw update harness` -- não é
    chamado por `trackfw init`/`trackfw update` (escopo de projeto), que continuam
    usando _generate_git_branch_guard_script. Mesmo conteúdo do escopo de projeto (ver
    doc comment de _GIT_BRANCH_GUARD_SH acima).
    """
    if not home:
        raise ValueError('home directory vazio')

    scripts_dir = os.path.join(home, '.trackfw', 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)

    script_path = os.path.join(scripts_dir, 'trackfw-git-branch-guard.sh')
    with open(script_path, 'w', encoding='utf-8') as f:
        f.write(_GIT_BRANCH_GUARD_SH.lstrip('\n'))
    os.chmod(script_path, 0o755)


def generate_global_credential_guard_script(home: str) -> None:
    """Gera o script shell trackfw-credential-guard.sh em escopo global, em
    <home>/.trackfw/scripts/trackfw-credential-guard.sh.

    Destinado a ser referenciado por hooks globais de CLI, instalados via
    `trackfw update harness` (ver ROADMAP-2026-08-06, Wave 2) -- nao e chamado por
    `trackfw init`/`trackfw update` (escopo de projeto), que continuam usando
    _generate_credential_guard_script.
    """
    if not home:
        raise ValueError('home directory vazio')

    scripts_dir = os.path.join(home, '.trackfw', 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)

    script_path = os.path.join(scripts_dir, 'trackfw-credential-guard.sh')
    with open(script_path, 'w', encoding='utf-8') as f:
        f.write(_GLOBAL_CREDENTIAL_GUARD_SH.lstrip('\n'))
    os.chmod(script_path, 0o755)


def print_architect_next_steps(cwd: str) -> None:
    """Exibe instruções de próximo passo após init/update."""
    candidates = [
        ('CLAUDE.md',                              'claude'),
        ('.cursor/rules/trackfw.mdc',              'cursor .'),
        ('.windsurfrules',                         'windsurf .'),
        ('.github/copilot-instructions.md',        'code . (Copilot)'),
        ('.amazonq/developer/guidelines.md',       'code . (Amazon Q)'),
        ('GEMINI.md',                              'gemini'),
        ('AGENTS.md',                              'codex'),
    ]

    detected = [cmd for f, cmd in candidates if os.path.exists(os.path.join(cwd, f))]
    if not detected:
        detected = ['claude']

    print()
    print('Próximo passo — inicie com o guia de arquitetura:')
    print()
    for cmd in detected:
        print(f'  {cmd}')
    print()
    print('  Execute /trackfw:architect no chat do seu assistente de IA.')
    print()
