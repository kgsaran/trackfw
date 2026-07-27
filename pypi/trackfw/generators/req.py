"""
generators/req.py — Gerador de REQs para trackfw.
Espelha npm/src/generators/req.js (funções newREQ, listREQs, parseREQStatus).
Formato canônico Go/Node, em inglês — REQ-2026-07-27-convergencia-templates-python.
Stdlib apenas — sem dependências externas.
"""

import os
import re
import unicodedata
from datetime import date


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


def find_req(name: str, req_dir: str) -> str:
    try:
        files = [f for f in os.listdir(req_dir) if f.endswith(".md")]
    except OSError as exc:
        raise RuntimeError(f"reading REQ dir: {exc}") from exc

    lowered = name.lower()
    for filename in files:
        if lowered in filename.lower():
            return os.path.join(req_dir, filename)
    raise RuntimeError(f'REQ "{name}" not found in {req_dir}')


def move_req(name: str, status: str, req_dir: str = None, cwd: str = None) -> str:
    if not status or not status.strip():
        raise RuntimeError("status is required")

    base = cwd or os.getcwd()
    if req_dir is None:
        req_dir = os.path.join(base, "docs", "req")
    elif not os.path.isabs(req_dir):
        req_dir = os.path.join(base, req_dir)

    filepath = find_req(name, req_dir)
    with open(filepath, "r", encoding="utf-8") as f:
        source = f.read()
    updated, changed = rewrite_req_status(source, status)
    if not changed:
        raise RuntimeError(f'REQ "{os.path.basename(filepath)}" has no frontmatter status/header Status to update')
    with open(filepath, "w", encoding="utf-8") as f:
        f.write(updated)
    return filepath
