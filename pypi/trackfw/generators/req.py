"""
generators/req.py — Gerador de REQs para trackfw.
Espelha npm/src/generators/req.js (funções newREQ, listREQs, parseREQStatus).
Formato canônico Go/Node, em inglês — REQ-2026-07-27-convergencia-templates-python.
Stdlib apenas — sem dependências externas.
"""

import datetime
import os
import re
import unicodedata
from datetime import date

from trackfw import config as cfg_module
from trackfw.generators.roadmap import STATE_ORDER, VALID_STATES


def slugify(title: str) -> str:
    """
    Converte título em slug kebab-case portável.
    NFKD + remoção de diacríticos + lowercase + [^a-z0-9]+ → hífen.
    Ex: "Autenticação e Sessão" → "autenticacao-e-sessao"
    """
    normalized = unicodedata.normalize("NFKD", title)
    ascii_str = normalized.encode("ascii", "ignore").decode("ascii")
    slug = ascii_str.lower()
    slug = re.sub(r"[^a-z0-9]+", "-", slug)
    slug = re.sub(r"-+", "-", slug)
    return slug.strip("-")


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
    status_line = f"> Date: {today} | Status: Open\n| Linear Issue: \n| Jira Issue: "

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


def rewrite_req_status(source: str, status: str) -> tuple[str, bool]:
    """Reescreve status no frontmatter e no header, preservando o restante."""
    if not source.startswith("---\n"):
        return source, False
    end = source[4:].find("\n---")
    if end < 0:
        return source, False

    frontmatter = source[4:4 + end]
    rest = source[4 + end:]
    changed = False
    lines = frontmatter.split("\n")

    for i, line in enumerate(lines):
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        if key.strip() != "status":
            continue
        trimmed = value.strip()
        quoted = len(trimmed) >= 2 and trimmed.startswith('"') and trimmed.endswith('"')
        new_line = f'{key}: "{status}"' if quoted else f"{key}: {status}"
        if lines[i] != new_line:
            lines[i] = new_line
            changed = True
        break

    if len(rest) > 4:
        body_lines = rest[4:].split("\n")
        marker = "| Status: "
        for i, line in enumerate(body_lines):
            if line.strip().startswith("## "):
                break
            idx = line.find(marker)
            if idx < 0:
                continue
            prefix = line[:idx + len(marker)]
            after = line[idx + len(marker):]
            pipe_idx = after.find(" |")
            suffix = after[pipe_idx:] if pipe_idx >= 0 else ""
            new_line = f"{prefix}{status}{suffix}"
            if body_lines[i] != new_line:
                body_lines[i] = new_line
                changed = True
                rest = "\n---" + "\n".join(body_lines)
            break

    if not changed:
        return source, False
    return "---\n" + "\n".join(lines) + rest, True


def parse_req_status(filepath: str) -> str:
    """
    Extrai o status da linha `> Date: ... | Status: ...` de um arquivo REQ.
    Espelha parseREQStatus (Node) / parseREQMeta (Go): o status termina no
    próximo " |" ou no fim da linha. 'unknown' se não encontrado ou arquivo ilegível.
    """
    try:
        with open(filepath, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return "unknown"

    marker = "| Status: "
    for line in content.split("\n"):
        idx = line.find(marker)
        if idx < 0:
            continue
        rest = line[idx + len(marker):]
        pipe_idx = rest.find(" |")
        if pipe_idx >= 0:
            rest = rest[:pipe_idx]
        rest = rest.rstrip(" >|")
        return rest.strip() or "unknown"
    return "unknown"


def _req_state_dir(state: str, cfg: dict) -> str | None:
    """Retorna req_dir/<estado> em modo flat, ou None se estado inválido."""
    if state not in VALID_STATES:
        return None
    return os.path.join(cfg.get("req_dir", "docs/req"), state)


def _req_agent_state_dir(agent: str | None, state: str, cfg: dict) -> str | None:
    """Retorna req_dir/<agente>/<estado> em modo by_agent."""
    if state not in VALID_STATES:
        return None
    if not agent:
        agents = cfg.get("agents") or []
        agent = agents[0] if agents else "default"
    return os.path.join(cfg.get("req_dir", "docs/req"), agent, state)


def list_req_files(cfg: dict) -> list[str]:
    """
    Descoberta recursiva de arquivos .md em req_dir, nos 3 layouts suportados
    (não mutuamente exclusivos — concatena todos):
    1. req_dir/*.md (flat legado)
    2. req_dir/<estado>/*.md, para cada estado em STATE_ORDER
    3. Se roadmap_namespacing == "by_agent": req_dir/<agente>/<estado>/*.md,
       para cada agente (cfg["agents"], ou subpastas de 1o nível de req_dir se vazio)
       x cada estado.
    """
    req_dir = cfg.get("req_dir", "docs/req")
    files: list[str] = []

    # 1. Flat legado.
    try:
        for f in sorted(os.listdir(req_dir)):
            if f.endswith(".md"):
                files.append(os.path.join(req_dir, f))
    except OSError:
        pass

    # 2. Por-estado, sem agente.
    for state in STATE_ORDER:
        d = os.path.join(req_dir, state)
        try:
            for f in sorted(os.listdir(d)):
                if f.endswith(".md"):
                    files.append(os.path.join(d, f))
        except OSError:
            continue

    # 3. by_agent.
    if cfg.get("roadmap_namespacing") == cfg_module.NAMESPACING_BY_AGENT:
        agents = list(cfg.get("agents") or [])
        if not agents:
            try:
                for entry in os.listdir(req_dir):
                    full = os.path.join(req_dir, entry)
                    if os.path.isdir(full):
                        agents.append(entry)
            except OSError:
                agents = []
        for agent in agents:
            for state in STATE_ORDER:
                d = os.path.join(req_dir, agent, state)
                try:
                    for f in sorted(os.listdir(d)):
                        if f.endswith(".md"):
                            files.append(os.path.join(d, f))
                except OSError:
                    continue

    return files


def list_reqs(cfg: dict) -> None:
    """
    Lista todos os REQs encontrados em cfg["req_dir"] (nos 3 layouts suportados),
    imprimindo filename e status. Formato idêntico ao Go/Node: "%-60s %s".
    """
    req_dir = cfg.get("req_dir", "docs/req")
    matches = list_req_files(cfg)

    if not matches:
        print(f"No REQs found in {req_dir}")
        return

    for path in matches:
        filename = os.path.basename(path)
        status = parse_req_status(path)
        print(f"{filename:<60} {status}")


def find_req(name: str, cfg: dict) -> str:
    lowered = name.lower()
    for path in list_req_files(cfg):
        if lowered in os.path.basename(path).lower():
            return path
    raise RuntimeError(f'REQ "{name}" not found in {cfg.get("req_dir", "docs/req")}')


def _append_req_transition_log(basename: str, from_state: str, to_state: str, cfg: dict) -> None:
    """Grava linha no .trackfw-log dentro de req_dir. Mesmo formato de
    _append_transition_log (roadmap.py), em arquivo de log separado."""
    now = datetime.datetime.now()
    timestamp = now.strftime("%Y-%m-%d %H:%M")
    line = f"{timestamp}  {basename:<50}  {from_state} → {to_state}\n"
    req_dir = cfg.get("req_dir", "docs/req")
    log_path = os.path.join(req_dir, ".trackfw-log")
    try:
        os.makedirs(req_dir, exist_ok=True)
        with open(log_path, "a", encoding="utf-8") as f:
            f.write(line)
    except OSError:
        pass


def move_req(name: str, status: str, cfg: dict = None, req_dir: str = None, cwd: str = None) -> str:
    if not status or not status.strip():
        raise RuntimeError("status is required")

    if cfg is None:
        base = cwd or os.getcwd()
        if req_dir is None:
            req_dir = os.path.join(base, "docs", "req")
        elif not os.path.isabs(req_dir):
            req_dir = os.path.join(base, req_dir)
        cfg = {"req_dir": req_dir}
    else:
        cfg = dict(cfg)
        resolved_req_dir = cfg.get("req_dir", "docs/req")
        if not os.path.isabs(resolved_req_dir):
            base = cwd or os.getcwd()
            resolved_req_dir = os.path.join(base, resolved_req_dir)
        cfg["req_dir"] = resolved_req_dir

    req_dir = cfg["req_dir"]

    filepath = find_req(name, cfg)
    with open(filepath, "r", encoding="utf-8") as f:
        source = f.read()
    updated, changed = rewrite_req_status(source, status)
    if not changed:
        raise RuntimeError(f'REQ "{os.path.basename(filepath)}" has no frontmatter status/header Status to update')

    req_dir_clean = os.path.normpath(req_dir)
    parent_dir = os.path.dirname(filepath)
    grandparent_dir = os.path.dirname(parent_dir)

    # Modo in-place: REQ solta em req_dir.
    if os.path.normpath(parent_dir) == req_dir_clean:
        with open(filepath, "w", encoding="utf-8") as f:
            f.write(updated)
        return filepath

    if status not in VALID_STATES:
        raise RuntimeError(f'invalid state "{status}" — valid states: {", ".join(VALID_STATES)}')

    target_dir = None
    from_state = None
    log_basename = None

    if os.path.normpath(grandparent_dir) == req_dir_clean and os.path.basename(parent_dir) in VALID_STATES:
        # Layout por-estado.
        from_state = os.path.basename(parent_dir)
        target_dir = _req_state_dir(status, cfg)
        log_basename = os.path.basename(filepath)
    elif (
        os.path.basename(parent_dir) in VALID_STATES
        and os.path.normpath(os.path.dirname(grandparent_dir)) == req_dir_clean
    ):
        # Layout by_agent.
        from_state = os.path.basename(parent_dir)
        agent = os.path.basename(grandparent_dir)
        target_dir = _req_agent_state_dir(agent, status, cfg)
        log_basename = f"{agent}/{os.path.basename(filepath)}"

    if target_dir is None:
        # Layout não reconhecido — fallback in-place, sem mover.
        with open(filepath, "w", encoding="utf-8") as f:
            f.write(updated)
        return filepath

    os.makedirs(target_dir, exist_ok=True)
    dst = os.path.join(target_dir, os.path.basename(filepath))
    with open(dst, "w", encoding="utf-8") as f:
        f.write(updated)
    if os.path.normpath(dst) != os.path.normpath(filepath):
        os.remove(filepath)

    _append_req_transition_log(log_basename, from_state, status, cfg)

    return dst
