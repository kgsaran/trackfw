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
    generate_gitattributes(cwd)
    _write_trackfw_yaml(cwd, opts)
    _write_example_adr(cwd, opts)
    generate_claude_md(cwd, opts)
    generate_claude_commands(cwd)
    generate_validate_script(cwd)
    # ML-2C (REQ-2026-08-28): same flow point as Go's Init (scaffold.go) and
    # Node's scaffold() — generateCIWorkflow(cfg) right after the validate
    # script. opts["ci"] is never set today because this runtime's `init`
    # has no --ci flag (see docs/cli-parity.md and the ML-2C "assimetria que
    # PERMANECE" note) — this call is a no-op in every current invocation,
    # wired only so a future --ci flag needs no new call site. It does NOT
    # honor a `ci:` hand-written into trackfw.yaml before init: the line
    # right above this one (_write_trackfw_yaml) rewrites trackfw.yaml from
    # opts unconditionally, so a pre-existing `ci:` key is not read back.
    generate_ci_workflow(cwd, opts)
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
    with open(dest, 'w', encoding='utf-8', newline="\n") as f:
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

    with open(filepath, 'w', encoding='utf-8', newline="\n") as f:
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


def _trackfw_rules_block(agent_conventions: str = "") -> str:
    conventions_section = ""
    if agent_conventions.strip():
        conventions_section = (
            '\n\n### Project Conventions\n'
            '> Declared by the team in `trackfw.yaml`\'s `agent_conventions` field — NOT\n'
            '> inferred automatically. trackfw does not impose an architectural standard; it only\n'
            '> propagates what the project has already decided.\n\n'
            f'{agent_conventions.strip()}\n'
        )

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
        '- **Threat model waves:** every feature roadmap opens with a Wave 0 threat model (before implementation) and closes with a red-team review wave (before release)\n'
        '- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)\n'
        '- Use `/trackfw:architect` to define stack before the first REQ\n'
        + conventions_section +
        '\n### Key Commands\n'
        '- `trackfw context` — current governance state (always run first)\n'
        '- `trackfw status` — all artifacts and states\n'
        '- `trackfw validate` — governance consistency check\n'
        '- `trackfw roadmap move <name> <state>` — transition roadmap state\n'
        '- `trackfw serve` — live Kanban board at http://localhost:4080\n'
        + RULES_END
    )


def _inject_or_update_rules(file_path: str, header_if_new: str, cwd: str = None) -> None:
    os.makedirs(os.path.dirname(os.path.abspath(file_path)), exist_ok=True)

    from trackfw.config import read_agent_conventions
    block = _trackfw_rules_block(read_agent_conventions(cwd))

    if not os.path.exists(file_path):
        content = header_if_new or ''
        if content and not content.endswith('\n'):
            content += '\n'
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8', newline="\n") as f:
            f.write(content)
        return

    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    start = content.find(RULES_START)
    if start == -1:
        if not content.endswith('\n'):
            content += '\n'
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8', newline="\n") as f:
            f.write(content)
        return

    end = content.find(RULES_END, start)
    if end == -1:
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8', newline="\n") as f:
            f.write(content)
        return

    new_content = content[:start] + block + content[end + len(RULES_END):]
    with open(file_path, 'w', encoding='utf-8', newline="\n") as f:
        f.write(new_content)


def inject_rules_for_tool(tool: str, cwd: str) -> None:
    rel_path = AGENT_FILES.get(tool)
    if not rel_path:
        return
    header = AGENT_HEADERS.get(tool, '')
    _inject_or_update_rules(os.path.join(cwd, rel_path), header, cwd)


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
    lines.append('7. **Threat model waves** — every feature roadmap opens with a Wave 0 threat model (before implementation) and closes with a red-team review wave (before release).\n')
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

    lines.append('\n## Architect responses\n')
    lines.append('\nDefault: what changed · what was decided · what is needed from you. Three to five lines.\n')
    lines.append('\nScale up only on these three triggers, and only on them: a **blocker** that stops the next wave; ')
    lines.append('a **pending user decision** that cannot be inferred from context; ')
    lines.append('an **error the architect made** that cannot be self-corrected.\n')
    lines.append('\nNever cut, even when short: measured evidence (command and result), barrier verdict, decision taken and why. ')
    lines.append('A response that buries a blocker in paragraph seven produced the same effect as not reporting it.\n')
    lines.append('\nCut: restating what an executor already reported, re-explaining reasoning already given, ')
    lines.append('recapping state that has not changed, closing praise. ')
    lines.append('Tables and code blocks only when they replace prose, never when they add to it.\n')
    lines.append('\nDepth is on demand from the user.\n')

    header = ''.join(lines)
    _inject_or_update_rules(os.path.join(cwd, 'CLAUDE.md'), header, cwd)
    print('  checkmark CLAUDE.md')


# _VALIDATE_SCRIPT_CONTENT is the byte-exact content that Python's `trackfw init` and
# `trackfw update` (validate-script target) write to scripts/trackfw-validate.sh.
# Exported so scaffold_doctor.py can compare on-disk content against this template without
# calling the generator (which also does makedirs + chmod, which are side-effects the
# doctor must not produce). This form is intentionally simpler than Go's/Node's
# cfg-dependent form — Python's `init` has no --backend/--frontend flags (see
# docs/cli-parity.md, "Template reference per runtime").
_VALIDATE_SCRIPT_CONTENT = "#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n"


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
    docs/cli-parity.md, "Template reference per runtime").
    """
    scripts_dir = os.path.join(cwd, 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)
    content = _VALIDATE_SCRIPT_CONTENT
    dest = os.path.join(scripts_dir, 'trackfw-validate.sh')
    with open(dest, "w", encoding="utf-8", newline="\n") as f:
        f.write(content)
    os.chmod(dest, 0o755)
    print('  checkmark scripts/trackfw-validate.sh')


# ---------------------------------------------------------------------------
# CI workflow (ML-2C, REQ-2026-08-28) — byte-identical to Go's
# buildGitHubActionsWorkflowContent/buildGitLabCIWorkflowContent
# (internal/generators/scaffold.go) and Node's mirror
# (npm/src/generators/init.js) for the same trackfw.__version__. The Python
# CLI never generated a CI workflow before this ML — see ADR-2026-08-28 and
# the ML-2C rewrite note in the roadmap: there was no template to pin, so
# this closes the gap rather than narrowing an existing exclusion.
#
# GitHubActionsWorkflowPath / GitLabCIWorkflowPath mirror Go's exported path
# constants — used by scaffold_doctor.py to identify these artifacts by path.
# ---------------------------------------------------------------------------

GITHUB_ACTIONS_WORKFLOW_PATH = os.path.join('.github', 'workflows', 'trackfw-gate.yml')
GITLAB_CI_WORKFLOW_PATH = '.gitlab-ci-trackfw.yml'


def build_github_actions_workflow_content(_cfg: dict | None = None) -> str:
    """Returns the content trackfw writes to GITHUB_ACTIONS_WORKFLOW_PATH.

    NOT version-independent: pins TRACKFW_VERSION to trackfw.__version__, the
    version of the binary that generated/updated the project (ADR-2026-08-28,
    REQ-2026-08-28 AC6/AC7). Scaffold doctor compares disk content against
    this template, so a project generated by a different binary version is
    correctly reported as scaffold-divergent (AC10) while a project just
    generated by the current binary is not (AC11). `_cfg` is accepted for API
    symmetry with Go's/Node's builder (cfg-independent for now; the `ci:` gate
    at the call site decides whether this is invoked at all).

    Job id is `governance-install-script` (ML-1A, ROADMAP-2026-09-01): was
    `governance`, colliding with the discover-installed sibling workflow's job
    id (trackfw-validate.yml, build_discover_github_actions_workflow_content
    in commands/discover.py) — both previously `governance`, producing two
    identically-named check-runs on any PR of a project with both workflows
    installed, which makes required_status_checks ambiguous (GitHub matches
    by name). Named after the install mechanism (install.sh here, `go
    install` there) rather than the workflow file, since that's what a
    required_status_checks reader needs to tell the two apart without
    opening the YAML.
    """
    from trackfw import __version__
    return (
        "name: trackfw-gate\n"
        "on:\n"
        "  pull_request:\n"
        "    branches: [main]\n"
        "\n"
        "jobs:\n"
        "  governance-install-script:\n"
        "    runs-on: ubuntu-latest\n"
        "    timeout-minutes: 10\n"
        "    env:\n"
        f'      TRACKFW_VERSION: "{__version__}"\n'
        "    steps:\n"
        "      - uses: actions/checkout@v4\n"
        "\n"
        "      - name: Install trackfw\n"
        "        run: |\n"
        "          curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh\n"
        "\n"
        "      - name: Governance gate\n"
        "        run: trackfw validate\n"
    )


def build_gitlab_ci_workflow_content(_cfg: dict | None = None) -> str:
    """Returns the content trackfw writes to GITLAB_CI_WORKFLOW_PATH.

    NOT version-independent — see build_github_actions_workflow_content above
    for the rationale (AC6/AC7, AC10, AC11, AC12).
    """
    from trackfw import __version__
    return (
        "# trackfw governance gate\n"
        "trackfw-gate:\n"
        "  stage: test\n"
        "  image: alpine:latest\n"
        "  timeout: 10 minutes\n"
        "  variables:\n"
        f'    TRACKFW_VERSION: "{__version__}"\n'
        "  before_script:\n"
        "    - apk add --no-cache curl\n"
        "    - curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh\n"
        "  script:\n"
        "    - trackfw validate\n"
        "  only:\n"
        "    - merge_requests\n"
    )


def generate_ci_workflow(cwd: str, cfg: dict) -> None:
    """Writes the CI workflow declared by cfg["ci"] (github-actions | gitlab-ci).

    Mirrors Go's generateCIWorkflow and Node's generateCIWorkflow: no-op when
    cfg["ci"] is anything else (absent, "none", or unrecognized).
    """
    ci = cfg.get('ci')
    if ci == 'github-actions':
        dest = os.path.join(cwd, GITHUB_ACTIONS_WORKFLOW_PATH)
        os.makedirs(os.path.dirname(dest), exist_ok=True)
        with open(dest, 'w', encoding='utf-8', newline="\n") as f:
            f.write(build_github_actions_workflow_content(cfg))
        print(f'  checkmark {GITHUB_ACTIONS_WORKFLOW_PATH}')
    elif ci == 'gitlab-ci':
        dest = os.path.join(cwd, GITLAB_CI_WORKFLOW_PATH)
        with open(dest, 'w', encoding='utf-8', newline="\n") as f:
            f.write(build_gitlab_ci_workflow_content(cfg))
        print(f'  checkmark {GITLAB_CI_WORKFLOW_PATH}')


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
            '   ````markdown\n'
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
            '   ## Wave 0 — Threat Model\n'
            '   > Dependencies: none. Blocks all implementation.\n\n'
            '   ### ML-0A — Threat model for this roadmap\n'
            '   **Status:** pending\n'
            '   **Files affected:**\n'
            '   **Actions:**\n'
            '   1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).\n'
            '   2. Threat model — who empties this Wave 0 without breaking any written rule, and how?\n'
            '   3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?\n'
            '   4. Declared residual — what this design accepts not covering.\n'
            '   **Acceptance criteria:**\n'
            '   - [ ] The four sections above answered with evidence, not a one-line assertion\n'
            '   - [ ] No implementation line written for this ML\n\n'
            '   **Gates da wave:**\n'
            '   ```bash\n'
            '   # Wave 0 gate — replace this placeholder with a project-specific check before\n'
            '   # marking ML-0A done. Do not remove the gate; replace its command (AC13).\n'
            '   exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md\n'
            '   ```\n\n'
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
            '   ````\n\n'
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
            'trackfw barrier <roadmap> --wave <n> --trust-local-gates --json\n'
            '```\n\n'
            '`--trust-local-gates` é obrigatório aqui: roadmaps WIP (modificados localmente, ainda não commitados em\n'
            '`origin/main`) são marcados como não confiáveis pela CLI direta por padrão, como proteção contra a\n'
            'execução de gates de roadmaps chegados por PR de terceiro. O slash command aplica esse flag porque\n'
            'ele representa o fluxo legítimo do arquiteto operando no próprio repositório — não porque os gates\n'
            'são inspecionados previamente (o diff ainda é responsabilidade do checklist abaixo).\n\n'
            '⚠️ **Não use `--trust-local-gates` ao revisar um roadmap chegado por PR de terceiro** — use a CLI\n'
            'direta sem o flag (`trackfw barrier <roadmap> --wave <n> --json`) para que os gates sejam marcados\n'
            'como `not_evaluated` e não executados.\n\n'
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
        with open(file_path, 'w', encoding='utf-8', newline="\n") as f:
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
  TOOL=$(echo "$INPUT" | PYTHONIOENCODING=utf-8 python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | PYTHONIOENCODING=utf-8 python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
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
# trackfw git branch guard — bloqueia git commit/push/checkout -b/branch/worktree add -b
# brutos por subagente
#
# TRIPWIRE, NÃO FRONTEIRA DE SEGURANÇA: detecta o caso óbvio — comando git literal, sem
# indireção de shell — não é defesa contra um agente adversário competente. Evasões que
# exigem tokenizar como o bash (ex.: git${IFS}push, {git,push}, g""it push) permanecem
# abertas por decisão: ver docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-
# com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md. O stripping de
# env/command abaixo reconhece as formas SEM argumentos antes de git (env git ...,
# command git ...) e o env seguido de uma sequência de atribuições CHAVE=valor
# (env FOO=bar git ..., env FOO=bar BAZ=qux git ...) — env com FLAGS (env -i git ...,
# env --ignore-environment git ...) e command com flags (command -p git ...) continuam
# evadindo; declarado, não fechado (ver AC5 do ML que adicionou esse stripping). A
# segmentação abaixo
# (quote_aware_split) evita falso-positivo em texto citado — não deve ser lida como imune a
# evasão por citação/tokenização do shell.
set -euo pipefail
set -f

# --- 0. Drena o stdin ANTES de qualquer saída antecipada (ML-1B, ROADMAP-2026-08-17-guard-
# global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md): sem isso,
# quem escreve o payload JSON no pipe recebe EPIPE quando o no-op abaixo sai com 0 antes de ler
# — reprodutível em 100% das chamadas fora de projeto trackfw, não é corrida de timing. Só drena
# se stdin não for um terminal interativo (-t 0): em invocação manual sem pipe, "cat" bloquearia
# esperando EOF (Ctrl-D). O valor lido é reaproveitado no passo 1 abaixo — nunca há uma segunda
# leitura.
_TRACKFW_STDIN=""
[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)

# --- 0b. No-op fora de projeto trackfw (ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
# projeto-trackfw.md): sobe diretórios a partir do cwd FÍSICO (pwd -P, resolve symlink) até
# achar trackfw.yaml na raiz do projeto. Sem trackfw.yaml em nenhum ancestral, o guard não se
# aplica — fora de projeto trackfw não há trackfw ship como alternativa, e bloquear ali é custo
# sem contrapartida. Custo medido: só parameter expansion e test -f por nível, nenhum fork de
# processo; limitado pela profundidade do caminho.
_TRACKFW_ROOT_DIR=$(pwd -P)
_TRACKFW_FOUND=0
while :; do
  if [ -f "$_TRACKFW_ROOT_DIR/trackfw.yaml" ]; then
    _TRACKFW_FOUND=1
    break
  fi
  if [ "$_TRACKFW_ROOT_DIR" = "/" ]; then
    break
  fi
  _TRACKFW_ROOT_DIR="${_TRACKFW_ROOT_DIR%/*}"
  if [ -z "$_TRACKFW_ROOT_DIR" ]; then
    _TRACKFW_ROOT_DIR="/"
  fi
done
[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT="$_TRACKFW_STDIN"
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .tool_info.command_line // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_info"[[:space:]]*:[[:space:]]*{[^}]*"command_line"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
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

# --- 2. Pré-processamento anti-falso-positivo: neutraliza separadores reais (';', '&&',
# '||', '|', quebra de linha) que estão DENTRO de aspas ou de corpo de heredoc, para que
# conteúdo de mensagem (ex.: `-m "linha 1\nlinha 2"`) nunca seja fatiado em pseudo-segmentos
# e lido como comando -------------------------------------------------------------------
#
# strip_heredoc_bodies: remove o CORPO de blocos heredoc (<<DELIM ... DELIM), preservando a
# linha de abertura e a linha terminadora — cobre o padrão `git commit -F- <<'EOF' ... EOF`
# (heredoc não citado, fora do escopo de quote_aware_split abaixo). Heurística por linha, não
# sintaxe completa de shell: só remove o corpo quando encontra a linha terminadora
# correspondente. Se o heredoc nunca fecha (terminador ausente ou não localizado), devolve o
# texto ORIGINAL sem qualquer alteração — lado seguro: mais restritivo é preferível a esconder
# um comando real atrás de um heredoc mal-formado.
strip_heredoc_bodies() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      in_heredoc = 0
      delim = ""
      ok = 1
    }
    {
      raw = raw $0 "\n"
      if (in_heredoc) {
        trimmed = $0
        sub(/^[ \t]+/, "", trimmed)
        sub(/[ \t]+$/, "", trimmed)
        if (trimmed == delim) {
          in_heredoc = 0
          out = out $0 "\n"
        }
        next
      }
      if (match($0, /<<-?[ \t]*[^ \t]+/)) {
        d = substr($0, RSTART, RLENGTH)
        sub(/^<<-?[ \t]*/, "", d)
        gsub(dq, "", d)
        gsub(sq, "", d)
        if (d != "") {
          delim = d
          in_heredoc = 1
        }
      }
      out = out $0 "\n"
    }
    END {
      if (in_heredoc) ok = 0
      if (ok) { printf "%s", out } else { printf "%s", raw }
    }
  '
}

# quote_aware_split: emite o texto com ';' isolado, '&&', '||' e '|' isolado convertidos em
# quebra de linha — EXCETO quando ocorrem dentro de uma string entre aspas simples ou duplas,
# caso em que são preservados como texto e uma quebra de linha real dentro das aspas vira
# espaço (nunca gera um novo pseudo-segmento). Substitui o antigo `sed` cego, que não
# distinguia texto citado de sintaxe de comando — a causa raiz do falso-positivo de linha de
# mensagem de commit iniciada por "git ...". Aspas não fechadas até o fim da entrada
# permanecem "abertas" até o fim — mesma semântica do shell real: uma aspa não fechada nunca
# deixa o texto seguinte executar como comando novo, só torna o restante parte da mesma
# string.
quote_aware_split() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      bs = sprintf("%c", 92)
      nl = sprintf("%c", 10)
    }
    { s = (NR == 1) ? $0 : s nl $0 }
    END {
      n = length(s)
      q = ""
      out = ""
      i = 1
      while (i <= n) {
        c = substr(s, i, 1)
        if (q != "") {
          if (q == dq && c == bs && i < n) {
            nx = substr(s, i + 1, 1)
            out = out c (nx == nl ? " " : nx)
            i += 2
            continue
          }
          if (c == q) {
            q = ""
            out = out c
            i++
            continue
          }
          out = out (c == nl ? " " : c)
          i++
          continue
        }
        if (c == dq || c == sq) {
          q = c
          out = out c
          i++
          continue
        }
        if (substr(s, i, 2) == "&&" || substr(s, i, 2) == "||") {
          out = out nl
          i += 2
          continue
        }
        if (c == ";" || c == "|") {
          out = out nl
          i++
          continue
        }
        out = out c
        i++
      }
      printf "%s", out
    }
  '
}

# match_subcommand — casa contra "git (commit|push|checkout -b|switch -c)", segmento por
# segmento. Cada segmento é um comando real, obtido depois do pré-processamento acima
# (strip_heredoc_bodies + quote_aware_split), que converte ';', '&&', '||', '|' fora de aspas
# em quebra de linha e neutraliza os mesmos separadores quando aparecem dentro de
# aspas/heredoc. "git" só conta se for o PRIMEIRO token do segmento (por basename, então
# /usr/bin/git também casa) — nunca uma ocorrência solta em qualquer posição da string
# inteira. Isso evita: (a) o segundo comando de uma cadeia escapar da checagem, (b) um path
# absoluto para o git escapar por comparação de igualdade exata, e (c) texto de prosa —
# inclusive linha de mensagem de commit que COMEÇA com "git <sub>" (ex.: uma tabela
# documentando comandos bloqueados) — ser tratado como comando, porque esse texto agora nunca
# produz um novo segmento. `git switch -c/-C/--create` (forma alternativa a `checkout -b`
# para criar branch) é reconhecido varrendo TODOS os tokens após o subcomando, não só o
# primeiro — cobre `git switch --track -c feat/x` (flag antes de -c).
# checkout -b é reconhecido do mesmo jeito: varre TODOS os tokens até achar -b/-B/--orphan,
# não só o primeiro. Prefixos env e command antes de git são descartados antes da checagem do
# basename — cobre env git push/command git push sem exigir tokenizar como o bash.
match_subcommand() {
  normalized=$(strip_heredoc_bodies "$1")
  normalized=$(quote_aware_split "$normalized")
  while IFS= read -r seg; do
    seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
    [ -n "$seg_trimmed" ] || continue

    set -- $seg_trimmed
    first="$1"
    base="${first##*/}"
    while [ "$base" = "env" ] || [ "$base" = "command" ]; do
      is_env="$base"
      shift
      [ "$#" -gt 0 ] || break
      if [ "$is_env" = "env" ]; then
        while [ "$#" -gt 0 ]; do
          case "$1" in
            -*)
              break
              ;;
            *=*)
              shift
              ;;
            *)
              break
              ;;
          esac
        done
        [ "$#" -gt 0 ] || break
      fi
      first="$1"
      base="${first##*/}"
    done
    [ "$base" = "git" ] || continue
    shift

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
        for tok2 in "$@"; do
          case "$tok2" in
            -b|-B|--orphan|--orphan=*)
              echo "checkout-b"
              return 0
              ;;
          esac
        done
        # git checkout -- <path> | git checkout . descarta alterações não commitadas do
        # caminho indicado, de forma irreversível, no worktree compartilhado — bloqueia
        # quando '--' aparece em qualquer posição (forma explícita de pathspec) ou quando
        # '.' aparece como token isolado. 'git checkout <branch>' sem nenhum dos dois
        # segue liberado por decisão (distinguir branch de caminho sem '--' é ambíguo, e
        # adivinhar produziria falso-positivo).
        checkout_path=0
        for tok2 in "$@"; do
          case "$tok2" in
            --|.)
              checkout_path=1
              ;;
          esac
        done
        if [ "$checkout_path" = "1" ]; then
          echo "checkout-path"
          return 0
        fi
        ;;
      switch)
        for tok2 in "$@"; do
          case "$tok2" in
            -c|-C|--create|--create=*|--force-create|--force-create=*)
              echo "switch-c"
              return 0
              ;;
          esac
        done
        ;;
      stash)
        # git stash: liberado só para leitura (list/show) — bloqueia a forma bare
        # (equivale a "push"), push, save, clear e drop. Decisão de KG: bloquear a
        # classe inteira, não só os literais medidos (ver REQ). Repositório com um único
        # worktree compartilhado entre subagentes paralelos — um stash de um agente
        # remove as alterações não commitadas de todos os outros.
        stash_sub="${1:-}"
        case "$stash_sub" in
          list|show)
            ;;
          *)
            echo "stash"
            return 0
            ;;
        esac
        ;;
      reset)
        # Só --hard bloqueia, em qualquer posição de token — --soft/--mixed (inclusive
        # sem flag, que é --mixed implícito) seguem liberados: --soft é o contorno
        # padrão para reempurrar trabalho staged via `trackfw ship -m "..."` (ainda falta commitar após --soft).
        for tok2 in "$@"; do
          case "$tok2" in
            --hard)
              echo "reset-hard"
              return 0
              ;;
          esac
        done
        ;;
      clean)
        # Bloqueia qualquer forma com force (-f, -fd, -fx, --force) ou -x/-X, EXCETO
        # quando -n/--dry-run também está presente (dry-run nunca apaga nada).
        clean_dry=0
        clean_force=0
        for tok2 in "$@"; do
          case "$tok2" in
            -n|--dry-run)
              clean_dry=1
              ;;
            -f*|--force|--force=*|-x|-X)
              clean_force=1
              ;;
          esac
        done
        if [ "$clean_dry" != "1" ] && [ "$clean_force" = "1" ]; then
          echo "clean-force"
          return 0
        fi
        ;;
      restore)
        # git restore --staged SOZINHO nunca toca o working tree (mexe só no
        # index), então segue liberado mesmo com path. Mas --worktree/-W (com ou
        # sem --staged junto) SEMPRE afeta o working tree — inclusive
        # "--staged --worktree", que restaura os dois — então bloqueia sempre que
        # --worktree/-W aparecer, e também no caso padrão (sem --staged em
        # nenhuma forma) com um argumento posicional (o path).
        restore_staged=0
        restore_worktree=0
        restore_positional=0
        for tok2 in "$@"; do
          case "$tok2" in
            --staged)
              restore_staged=1
              ;;
            --worktree|-W)
              restore_worktree=1
              ;;
            -*)
              ;;
            *)
              restore_positional=1
              ;;
          esac
        done
        if [ "$restore_positional" = "1" ]; then
          if [ "$restore_worktree" = "1" ] || [ "$restore_staged" != "1" ]; then
            echo "restore-path"
            return 0
          fi
        fi
        ;;
      branch)
        # git branch é majoritariamente leitura (sem args, -a, -r, -l, --list, -v/-vv,
        # --show-current, --contains, --no-contains, --merged, --no-merged, --sort=,
        # --format=, --points-at, -d/-D/--delete) — bloquear leitura seria pior que a
        # brecha. Só bloqueia: (a) -c/-C/-m/-M/--copy/--move (cria/renomeia branch,
        # qualquer posição de token) ou (b) um argumento posicional puro (nome da branch a
        # criar), a menos que -d/-D/--delete também esteja presente (delete tem
        # posicional legítimo — o nome a apagar). Flags de valor conhecidas (--contains,
        # --no-contains, --sort, --format, --points-at, --merged, --no-merged) têm seu
        # valor seguinte pulado quando vem em token separado, para não ser lido como
        # posicional de criação.
        branch_action=0
        has_delete=0
        saw_positional=0
        skip_next=0
        for tok2 in "$@"; do
          if [ "$skip_next" = "1" ]; then
            skip_next=0
            continue
          fi
          case "$tok2" in
            -c|-C|-m|-M|--copy|--copy=*|--move|--move=*)
              branch_action=1
              ;;
            -d|-D|--delete|--delete=*)
              has_delete=1
              ;;
            --contains|--no-contains|--sort|--format|--points-at|--merged|--no-merged)
              skip_next=1
              ;;
            -*)
              ;;
            *)
              saw_positional=1
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then
          if [ "$branch_action" = "1" ] || [ "$saw_positional" = "1" ]; then
            echo "branch-create"
            return 0
          fi
        fi
        ;;
      worktree)
        if [ "${1:-}" = "add" ]; then
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -b|-B)
                echo "worktree-add-b"
                return 0
                ;;
            esac
          done
        elif [ "${1:-}" = "remove" ]; then
          # git worktree remove SEM -f/--force já recusa sozinho quando há alteração não
          # commitada no worktree indicado — só a forma com force é irreversível o bastante
          # para bloquear aqui.
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -f|--force)
                echo "worktree-remove-force"
                return 0
                ;;
            esac
          done
        fi
        ;;
      update-ref)
        # git update-ref reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o
        # objeto apontado nem exigir push — foi o mecanismo que tornou alcançável o exploit
        # descrito no ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md
        # (Emenda 1): forjar origin/<base> localmente para desviar o commit-alvo de trackfw
        # release tag. Sem forma de leitura equivalente a bloquear seletivamente — a
        # subcommand inteira é escrita — bloqueia sempre, sem exceção de token.
        echo "update-ref"
        return 0
        ;;
      rm)
        # git rm -f/--force apaga do working tree e do index de forma irreversível, mesma
        # classe de git clean -f/git reset --hard já bloqueados acima — sem exceção para
        # --cached (destrancar do index sem -f já segue liberado por não precisar de force).
        for tok2 in "$@"; do
          case "$tok2" in
            -f*|--force|--force=*)
              echo "rm-force"
              return 0
              ;;
          esac
        done
        ;;
    esac
  done <<EOF
$normalized
EOF
  return 1
}

SUBCOMMAND=$(match_subcommand "$CMD_RAW") || exit 0

case "$SUBCOMMAND" in
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  switch-c)
    REASON="trackfw: git switch -c bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  branch-create)
    REASON="trackfw: git branch bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-add-b)
    REASON="trackfw: git worktree add -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use \`trackfw commit -m '<mensagem>'\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use \`trackfw push\` (para empurrar commits já criados), \`trackfw ship\` (para commit+push+PR em uma etapa) ou \`trackfw release tag\` (para publicar uma tag de release). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  stash)
    REASON="trackfw: git stash bruto bloqueado — worktree compartilhado entre subagentes, um stash remove as alterações não commitadas de todos os outros. \`git stash list\`/\`git stash show\` seguem liberados; para guardar trabalho em progresso, use uma branch própria via \`trackfw branch new\` e commit nela. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  reset-hard)
    REASON="trackfw: git reset --hard bruto bloqueado — descarta de forma irreversível as alterações não commitadas de todo o worktree compartilhado. \`git reset --soft\`/\`--mixed\` seguem liberados (ex.: \`git reset --soft HEAD~1\` é o caminho padrão; use \`trackfw ship -m "..."\` para commitar e empurrar). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  clean-force)
    REASON="trackfw: git clean -f/-x bruto bloqueado — apaga arquivos não rastreados do worktree compartilhado, de forma irreversível. \`git clean -n\`/\`--dry-run\` segue liberado para revisar antes o que seria apagado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  restore-path)
    REASON="trackfw: git restore <path> bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. \`git restore --staged\` (não toca o working tree) segue liberado; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  checkout-path)
    REASON="trackfw: git checkout -- <path>/git checkout . bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. \`git checkout <branch>\`/\`git switch <branch>\` seguem liberados; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  update-ref)
    REASON="trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que \`trackfw release tag\` publicaria. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-remove-force)
    REASON="trackfw: git worktree remove -f/--force bruto bloqueado — remove um worktree e descarta de forma irreversível qualquer alteração não commitada nele. \`git worktree remove\` sem force segue liberado (recusa sozinho quando há algo não commitado). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  rm-force)
    REASON="trackfw: git rm -f/--force bruto bloqueado — apaga arquivos do working tree e do index de forma irreversível, mesma classe de \`git clean -f\`/\`git reset --hard\` já bloqueados. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"decision":"block","reason":"%s"}\n' "$REASON"
echo "$REASON" >&2
exit 2
"""


# gitattributes: o primeiro campo da regra mantida por este gerador. Casa o
# BASENAME (padrao sem barra) de proposito: `roadmap_dir` e `req_dir` sao
# configuraveis por projeto (trackfw.yaml) e ambos carregam um `.trackfw-log`,
# entao caminho fixo nasceria quebrado em quem configurou diretorio diferente.
GITATTRIBUTES_RULE_TARGET = ".trackfw-log"

# Byte-identico ao bloco de internal/generators/scaffold.go e
# npm/src/generators/init.js (regra dura de paridade).
GITATTRIBUTES_BLOCK = (
    "# trackfw: .trackfw-log is append-only \u2014 every write lands on the last line, so\n"
    "# two parallel branches conflict on every merge. merge=union keeps the lines\n"
    "# from both sides (chronological order is not guaranteed). The pattern has no\n"
    "# slash, so it matches the file in any directory \u2014 roadmap_dir and req_dir both\n"
    "# carry one, and both are configurable per project.\n"
    ".trackfw-log merge=union\n"
)


def has_gitattributes_rule(content: str) -> bool:
    """True quando ALGUMA linha nao-comentario tem `.trackfw-log` como primeiro
    campo. Predicado sobre o CAMPO, nao sobre a string literal da linha inteira:
    espacamento diferente ou outro atributo ja sao "a regra existe", e reescrever
    por cima sobrescreveria decisao do projeto."""
    for line in content.split("\n"):
        trimmed = line.strip()
        if not trimmed or trimmed.startswith("#"):
            continue
        if trimmed.split()[0] == GITATTRIBUTES_RULE_TARGET:
            return True
    return False


def generate_gitattributes(cwd: str) -> None:
    """Garante `merge=union` para o `.trackfw-log` no `.gitattributes` da raiz.

    Tres ramos, todos idempotentes: ausente -> cria; existe sem a regra ->
    APPEND (nunca sobrescreve o arquivo do projeto); existe com a regra -> no-op.
    """
    file_path = os.path.join(cwd, ".gitattributes")
    if not os.path.exists(file_path):
        with open(file_path, "w", encoding="utf-8", newline="\n") as f:
            f.write(GITATTRIBUTES_BLOCK)
        print("  \u2713 .gitattributes")
        return
    with open(file_path, "r", encoding="utf-8") as f:
        existing = f.read()
    if has_gitattributes_rule(existing):
        return
    # Arquivo preexistente sem newline final: emendar direto grudaria a primeira
    # linha do bloco na ultima linha do projeto, corrompendo a config em silencio.
    if existing and not existing.endswith("\n"):
        existing += "\n"
    with open(file_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(existing + GITATTRIBUTES_BLOCK)
    print("  \u2713 .gitattributes")


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
    with open(index_path, 'w', encoding='utf-8', newline="\n") as f:
        f.write(content)
    print('  ✓ vault/notes/index.md')


def _generate_attention_scripts(cwd: str) -> None:
    """Gera scripts shell de attention signal/cleanup em scripts/."""
    scripts_dir = os.path.join(cwd, 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)

    signal_path = os.path.join(scripts_dir, 'trackfw-attention-signal.sh')
    with open(signal_path, 'w', encoding='utf-8', newline="\n") as f:
        f.write(_ATTENTION_SIGNAL_SH.lstrip('\n'))
    os.chmod(signal_path, 0o755)

    cleanup_path = os.path.join(scripts_dir, 'trackfw-attention-cleanup.sh')
    with open(cleanup_path, 'w', encoding='utf-8', newline="\n") as f:
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
    with open(script_path, 'w', encoding='utf-8', newline="\n") as f:
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
    with open(script_path, 'w', encoding='utf-8', newline="\n") as f:
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
    with open(script_path, 'w', encoding='utf-8', newline="\n") as f:
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
    with open(script_path, 'w', encoding='utf-8', newline="\n") as f:
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
