"""
roadmap.py — Gerador e movimentador de roadmaps.
Espelha npm/src/generators/roadmap.js em Python puro.
"""

import os
import re
import datetime

from trackfw import config as cfg_module

VALID_STATES = ["backlog", "wip", "blocked", "done", "abandoned"]
STATE_ORDER = ["wip", "backlog", "blocked", "done", "abandoned"]


# ---------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------

def slugify(title: str) -> str:
    """Converte string para slug lowercase com hífens."""
    slug = title.lower()
    slug = re.sub(r"[^a-z0-9]+", "-", slug)
    slug = slug.strip("-")
    return slug


def _rewrite_roadmap_status(source: str, state: str) -> tuple[str, bool]:
    """Reescreve o campo "status:" no bloco de frontmatter e a linha
    "| Status: <valor>" do cabeçalho no corpo.

    Espelha a semântica de _rewrite_frontmatter_fields
    (pypi/trackfw/integrations/renderers.py):
    - Escopo estrito ao bloco de frontmatter (entre "---\\n" de abertura e
      "\\n---" de fechamento).
    - Demais linhas preservadas byte a byte (ordem, espaçamento, estilo de aspas).
    - A chave NÃO é inventada se ausente; source é devolvida inalterada.
    - Sem frontmatter reconhecível → source é devolvida inalterada, sem erro.

    A sincronização de "| Status: " no corpo é escopada: apenas a primeira
    ocorrência antes do primeiro "## " heading é atualizada.

    Retorna (conteúdo_possivelmente_modificado, changed: bool).
    """
    if not source.startswith("---\n"):
        return source, False
    end = source.find("\n---", 4)
    if end < 0:
        return source, False

    frontmatter = source[4:end]
    rest = source[end:]  # starts with "\n---"

    changed = False
    lines = frontmatter.split("\n")
    for index, line in enumerate(lines):
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        if key.strip() != "status":
            continue
        trimmed_value = value.strip()
        quoted = len(trimmed_value) >= 2 and trimmed_value.startswith('"') and trimmed_value.endswith('"')
        if quoted:
            new_line = f'{key}: "{state}"'
        else:
            new_line = f"{key}: {state}"
        if lines[index] != new_line:
            lines[index] = new_line
            changed = True
        break  # only the first status: in frontmatter

    # Sync "| Status: <valor>" in the header line of the body (after closing ---).
    # Only the first occurrence before the first "## " heading is updated.
    new_rest = rest
    marker = "| Status: "
    if len(rest) > 4:
        body = rest[4:]  # skip "\n---"
        body_lines = body.split("\n")
        for i, bline in enumerate(body_lines):
            if bline.lstrip().startswith("## "):
                break
            idx = bline.find(marker)
            if idx < 0:
                continue
            prefix = bline[:idx + len(marker)]
            after = bline[idx + len(marker):]
            pipe_idx = after.find(" |")
            suffix = after[pipe_idx:] if pipe_idx >= 0 else ""
            new_line = prefix + state + suffix
            if body_lines[i] != new_line:
                body_lines[i] = new_line
                changed = True
                new_rest = "\n---" + "\n".join(body_lines)
            break  # only the first | Status: before ##

    if not changed:
        return source, False
    return "---\n" + "\n".join(lines) + new_rest, True


def _state_dir(state: str, cfg: dict) -> str | None:
    """Retorna diretório do estado em modo flat, ou None se estado inválido."""
    if state not in VALID_STATES:
        return None
    return os.path.join(cfg["roadmap_dir"], state)


def _agent_state_dir(agent: str | None, state: str, cfg: dict) -> str | None:
    """Retorna diretório agente/estado em modo by_agent."""
    if state not in VALID_STATES:
        return None
    if not agent:
        agents = cfg.get("agents") or []
        agent = agents[0] if agents else "default"
    return os.path.join(cfg["roadmap_dir"], agent, state)


def _find_roadmap_matches(name: str, cfg: dict) -> list[str]:
    """
    Retorna lista de paths que contêm `name` (case-insensitive) em qualquer estado.
    Suporta modo flat e by_agent.
    """
    matches = []
    name_lower = name.lower()

    if cfg.get("roadmap_namespacing") == cfg_module.NAMESPACING_BY_AGENT:
        agents = list(cfg.get("agents") or [])
        if not agents:
            roadmap_dir = cfg["roadmap_dir"]
            try:
                for entry in os.listdir(roadmap_dir):
                    full = os.path.join(roadmap_dir, entry)
                    if os.path.isdir(full):
                        agents.append(entry)
            except OSError:
                agents = ["default"]
        for agent in agents:
            for state in STATE_ORDER:
                d = os.path.join(cfg["roadmap_dir"], agent, state)
                try:
                    for f in os.listdir(d):
                        if name_lower in f.lower() and f.endswith(".md"):
                            matches.append(os.path.join(d, f))
                except OSError:
                    continue
    else:
        for state in STATE_ORDER:
            d = os.path.join(cfg["roadmap_dir"], state)
            try:
                for f in os.listdir(d):
                    if name_lower in f.lower() and f.endswith(".md"):
                        matches.append(os.path.join(d, f))
            except OSError:
                continue

    return matches


def _append_transition_log(basename: str, from_state: str, to_state: str, cfg: dict) -> None:
    """Grava linha no .trackfw-log dentro do roadmap_dir."""
    now = datetime.datetime.now()
    timestamp = now.strftime("%Y-%m-%d %H:%M")
    line = f"{timestamp}  {basename:<50}  {from_state} → {to_state}\n"
    log_path = os.path.join(cfg["roadmap_dir"], ".trackfw-log")
    try:
        os.makedirs(os.path.dirname(log_path), exist_ok=True)
        with open(log_path, "a", encoding="utf-8") as f:
            f.write(line)
    except OSError:
        pass


def _roadmap_template(title: str, slug: str, date: str) -> str:
    """
    Retorna conteúdo do roadmap no formato canônico Go/Node (inglês).
    Frontmatter: status: backlog · date · req: "" · squad: "" (minúsculo).
    Header: > Created: <data> | Status: backlog.
    Seções e labels de ML em inglês.
    REQ-2026-07-27-convergencia-templates-python.
    """
    req_path = ""
    return f"""---
status: backlog
date: {date}
req: ""
squad: ""
---

# Roadmap: {title}

> Created: {date} | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: {req_path}

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — {title}
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
"""


# ---------------------------------------------------------------------------
# API pública
# ---------------------------------------------------------------------------

def generate_roadmap(title: str, cfg: dict, agent: str = None) -> str:
    """
    Cria roadmap em backlog/.
    - Modo flat:     cfg["roadmap_dir"]/backlog/<slug>.md
    - Modo by_agent: cfg["roadmap_dir"]/<agent>/backlog/<slug>.md
    Retorna o path do arquivo criado.
    """
    today = datetime.date.today().isoformat()
    slug = slugify(title)
    filename = f"ROADMAP-{today}-{slug}.md"

    if cfg.get("roadmap_namespacing") == cfg_module.NAMESPACING_BY_AGENT:
        backlog_dir = _agent_state_dir(agent, "backlog", cfg)
    else:
        backlog_dir = os.path.join(cfg["roadmap_dir"], "backlog")

    os.makedirs(backlog_dir, exist_ok=True)
    filepath = os.path.join(backlog_dir, filename)

    body = _roadmap_template(title, slug, today)
    with open(filepath, "w", encoding="utf-8") as f:
        f.write(body)

    return filepath


def move_roadmap(filename: str, to_state: str, cfg: dict) -> str:
    """
    Move um roadmap de um estado para outro, atualizando status: no frontmatter.
    Busca o arquivo em todos os estados (e em todos os agentes em modo by_agent).
    Retorna o novo path.
    Levanta ValueError em estado inválido ou arquivo não encontrado.
    """
    if to_state not in VALID_STATES:
        raise ValueError(
            f'Estado inválido "{to_state}" — válidos: {", ".join(VALID_STATES)}'
        )

    # Encontra o arquivo em qualquer estado
    matches = _find_roadmap_matches(filename, cfg)

    # Filtra apenas o arquivo com basename exato (sem partial match por default)
    exact = [m for m in matches if os.path.basename(m) == filename]
    if not exact:
        # Tenta partial match se não houver exato
        exact = matches

    if not exact:
        raise FileNotFoundError(f'Roadmap "{filename}" não encontrado em nenhum estado.')

    if len(exact) > 1:
        raise ValueError(
            f'Múltiplos roadmaps encontrados para "{filename}": {exact}'
        )

    src = exact[0]
    basename = os.path.basename(src)
    from_state = os.path.basename(os.path.dirname(src))

    # Determina diretório de destino preservando agente em by_agent
    if cfg.get("roadmap_namespacing") == cfg_module.NAMESPACING_BY_AGENT:
        agent_dir = os.path.dirname(os.path.dirname(src))
        agent = os.path.basename(agent_dir)
        target_dir = _agent_state_dir(agent, to_state, cfg)
    else:
        target_dir = _state_dir(to_state, cfg)

    os.makedirs(target_dir, exist_ok=True)
    dst = os.path.join(target_dir, basename)

    # Lê conteúdo, sincroniza status: no frontmatter (e cabeçalho no corpo) e escreve no destino.
    with open(src, "r", encoding="utf-8") as f:
        content = f.read()

    updated, _ = _rewrite_roadmap_status(content, to_state)

    with open(dst, "w", encoding="utf-8") as f:
        f.write(updated)

    os.remove(src)

    _append_transition_log(basename, from_state, to_state, cfg)

    return dst
