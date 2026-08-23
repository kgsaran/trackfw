"""
roadmap.py — Gerador e movimentador de roadmaps.
Espelha npm/src/generators/roadmap.js em Python puro.
"""

import os
import re
import sys
import datetime
import unicodedata

from trackfw import config as cfg_module

VALID_STATES = ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]
STATE_ORDER = ["analyzing", "wip", "backlog", "blocked", "done", "abandoned"]

# WAVE0_BLOCK — seção "## Wave 0 — Threat Model" pré-anexada a todo roadmap gerado, antes da
# primeira wave de implementação (AC1, AC12). Byte-idêntica a internal/generators/roadmap.go
# (wave0Block/wave0GateFence) e a npm/src/generators/roadmap.js (WAVE0_BLOCK) — gate:
# scripts/check-artifact-parity.sh.
#
# O comando do gate (`exit 1`) é fixo, literal, e nunca interpolado com título de REQ, slug ou
# qualquer string controlada pelo usuário — ver o comentário do gerador Go para a justificativa
# completa (AC13, docs/cli-parity.md § "trackfw barrier"). Ele falha fechado (fails closed) de
# propósito, até que ML-0A o substitua por um check real e específico do projeto.
#
# O ML é sempre ML-0A, nunca ML-1A — generate_roadmap_from_req rotula os MLs derivados dos
# critérios de aceite da REQ como "ML-1A", "ML-1B", ... a partir do primeiro critério.
WAVE0_BLOCK = """## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** pending
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

"""


# ---------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------

def slugify(title: str) -> str:
    """
    Converte string para slug kebab-case portável.
    NFKD + remoção de diacríticos + lowercase + [^a-z0-9]+ → hífen.
    Ex: "Autenticação e Sessão" → "autenticacao-e-sessao"
    """
    normalized = unicodedata.normalize("NFKD", title)
    ascii_str = normalized.encode("ascii", "ignore").decode("ascii")
    slug = ascii_str.lower()
    slug = re.sub(r"[^a-z0-9]+", "-", slug)
    slug = re.sub(r"-+", "-", slug)
    return slug.strip("-")


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


def _roadmap_template(title: str, slug: str, date: str, req_path: str = "") -> str:
    """
    Retorna conteúdo do roadmap no formato canônico Go/Node (inglês).
    Frontmatter: status: backlog · date · req: "<req_path>" (vazio se não informado) · squad: "" (minúsculo).
    Header: > Created: <data> | Status: backlog.
    Seções e labels de ML em inglês.
    REQ-2026-07-27-convergencia-templates-python.
    """
    return f"""---
status: backlog
date: {date}
req: "{req_path}"
squad: ""
---

# Roadmap: {title}

> Created: {date} | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: {req_path}

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

{WAVE0_BLOCK}## Wave 1 — <name> (parallel MLs)
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


def _parse_req_for_roadmap(content: str) -> tuple[str, list[str], str]:
    """Extrai título, critérios de aceite e ADR linkada de um arquivo REQ."""
    title = ""
    criteria = []
    linked_adr = ""
    in_criteria = False

    for line in content.splitlines():
        if line.startswith("# REQ: "):
            title = line.removeprefix("# REQ: ").strip()
            continue
        if line.startswith("# REQ — "):
            title = line.removeprefix("# REQ — ").strip()
            continue
        if line.startswith("# REQ - "):
            title = line.removeprefix("# REQ - ").strip()
            continue
        if line.startswith("**ADR:**"):
            linked_adr = line.removeprefix("**ADR:**").strip()
            continue

        lower = line.strip().lower()
        if lower in ("## critérios de aceite", "## acceptance criteria"):
            in_criteria = True
            continue
        if in_criteria and line.startswith("## "):
            in_criteria = False
            continue
        if not in_criteria:
            continue

        trimmed = line.strip()
        for prefix in ("- [ ]", "- [x]", "- [X]"):
            if trimmed.startswith(prefix):
                item = trimmed[len(prefix):].strip().replace("`", "")
                if item:
                    criteria.append(item)
                break

    return title, criteria, linked_adr


# ---------------------------------------------------------------------------
# API pública
# ---------------------------------------------------------------------------

def _backlog_dir(cfg: dict, agent: str = None) -> str:
    if cfg.get("roadmap_namespacing") == cfg_module.NAMESPACING_BY_AGENT:
        return _agent_state_dir(agent, "backlog", cfg)
    return os.path.join(cfg["roadmap_dir"], "backlog")


def generate_roadmap(title: str, cfg: dict, agent: str = None, req_path: str = "") -> str:
    """
    Cria roadmap em backlog/.
    - Modo flat:     cfg["roadmap_dir"]/backlog/<slug>.md
    - Modo by_agent: cfg["roadmap_dir"]/<agent>/backlog/<slug>.md
    Retorna o path do arquivo criado.
    """
    # AC1/AC2: o título é dado de uma linha — newline e CR são entrada malformada.
    # Mensagem byte-idêntica nos 3 CLIs (docs/cli-parity.md).
    if '\n' in title or '\r' in title:
        print("Error: roadmap title must be a single line: newline and carriage return are not allowed", file=sys.stderr)
        sys.exit(1)

    today = datetime.date.today().isoformat()
    slug = slugify(title)
    filename = f"ROADMAP-{today}-{slug}.md"

    backlog_dir = _backlog_dir(cfg, agent)

    os.makedirs(backlog_dir, exist_ok=True)
    filepath = os.path.join(backlog_dir, filename)

    body = _roadmap_template(title, slug, today, req_path=req_path)
    with open(filepath, "w", encoding="utf-8") as f:
        f.write(body)

    return filepath


def generate_roadmap_from_req(req_path: str, cfg: dict, agent: str = None) -> str:
    """Cria roadmap pré-preenchido com MLs extraídos dos critérios de aceite da REQ."""
    with open(req_path, "r", encoding="utf-8") as f:
        data = f.read()

    parsed_title, criteria, linked_adr = _parse_req_for_roadmap(data)
    basename = os.path.basename(req_path)
    title = parsed_title or os.path.splitext(basename)[0].removeprefix("REQ-")

    # AC1: o título lido da REQ também pode conter newline forjado — rejeitar cedo.
    if '\n' in title or '\r' in title:
        print("Error: roadmap title must be a single line: newline and carriage return are not allowed", file=sys.stderr)
        sys.exit(1)

    today = datetime.date.today().isoformat()
    slug = slugify(title)
    filename = f"ROADMAP-{today}-{slug}.md"
    backlog_dir = _backlog_dir(cfg, agent)

    os.makedirs(backlog_dir, exist_ok=True)
    filepath = os.path.join(backlog_dir, filename)

    ml_lines = [
        "## Wave 1 — Implementation (derived from REQ criteria)",
        "> Dependencies: none",
    ]
    for index, criterion in enumerate(criteria):
        ml_label = f"ML-1{chr(ord('A') + index)}"
        ml_lines.extend([
            "",
            f"### {ml_label} — {criterion}",
            "**Status:** pending",
            "**Files affected:**",
            "**Actions:**",
            "**Acceptance criteria:**",
            f"- [ ] {criterion}",
            "- [ ] build passes",
            "- [ ] tests green",
        ])

    adr_ref = f"\nADR: {linked_adr}" if linked_adr else ""
    ml_section = WAVE0_BLOCK + "\n".join(ml_lines)
    body = f"""---
status: backlog
date: {today}
req: "{req_path}"
squad: ""
---

# Roadmap: {title}

> Created: {today} | Status: backlog

## Context
<!-- Derived from REQ: {basename} -->
REQ: {req_path}{adr_ref}

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

{ml_section}
"""

    with open(filepath, "w", encoding="utf-8") as f:
        f.write(body)

    return filepath


def _get_frontmatter_roadmap_value(content: str) -> str:
    """
    Extrai o valor do campo 'roadmap:' do bloco de frontmatter de uma REQ.
    Remove aspas externas (não backticks) — mesma semântica de normalize_yaml_flat_value
    do validator (trim apenas ' e ", não `).
    Retorna '' se ausente ou se o valor não terminar em '.md'.
    """
    if not content.startswith("---\n"):
        return ""
    end = content.find("\n---", 4)
    if end < 0:
        return ""
    for line in content[4:end].split("\n"):
        if ":" not in line:
            continue
        key, val = line.split(":", 1)
        if key.strip().lower() != "roadmap":
            continue
        val = val.strip()
        if len(val) >= 2 and val[0] == val[-1] and val[0] in ('"', "'"):
            val = val[1:-1]
        return val if val.endswith(".md") else ""
    return ""


def _rewrite_req_roadmap_ref(content: str, new_path: str) -> tuple[str, bool]:
    """
    Reescreve o campo 'roadmap:' no frontmatter e a linha 'Roadmap:' no corpo de uma REQ.

    - Frontmatter: preserva estilo de aspas existente.
    - Corpo: preserva formatação existente, inclusive backticks.
    - Idempotente: se o valor já for new_path, retorna (content, False).

    Espelha a semântica de _rewrite_roadmap_status: escopo estrito ao frontmatter,
    sem inventar chave ausente, sem alterar outras linhas.
    """
    if not content.startswith("---\n"):
        return content, False
    end = content.find("\n---", 4)
    if end < 0:
        return content, False

    frontmatter = content[4:end]
    rest = content[end:]  # começa com "\n---"

    changed = False
    fm_lines = frontmatter.split("\n")
    for i, line in enumerate(fm_lines):
        if ":" not in line:
            continue
        key, val = line.split(":", 1)
        if key.strip().lower() != "roadmap":
            continue
        val_stripped = val.strip()
        # Preserva estilo de aspas
        if len(val_stripped) >= 2 and val_stripped[0] == val_stripped[-1] and val_stripped[0] in ('"', "'"):
            q = val_stripped[0]
            new_line = f'{key}: {q}{new_path}{q}'
        else:
            new_line = f'{key}: {new_path}'
        if fm_lines[i] != new_line:
            fm_lines[i] = new_line
            changed = True
        break  # apenas o primeiro 'roadmap:' no frontmatter

    # Atualiza 'Roadmap:' no corpo (após o fechamento ---), preservando formatação
    new_rest = rest
    if len(rest) > 4:
        body = rest[4:]  # pula "\n---"
        body_lines = body.split("\n")
        for i, bline in enumerate(body_lines):
            m = re.match(r'^(\s*[Rr]oadmap:\s*)(.+)$', bline)
            if not m:
                continue
            prefix = m.group(1)   # "Roadmap: " — preserva caixa original
            value = m.group(2)    # "`path`", "path", '"path"', etc.
            # Preserva delimitadores: backtick, aspas ou bare
            if value.startswith("`") and value.endswith("`"):
                new_value = f"`{new_path}`"
            elif len(value) >= 2 and value[0] == value[-1] and value[0] in ('"', "'"):
                q = value[0]
                new_value = f'{q}{new_path}{q}'
            else:
                new_value = new_path
            new_bline = f"{prefix}{new_value}"
            if body_lines[i] != new_bline:
                body_lines[i] = new_bline
                changed = True
                new_rest = "\n---" + "\n".join(body_lines)
            break  # apenas a primeira linha 'Roadmap:' no corpo

    if not changed:
        return content, False
    return "---\n" + "\n".join(fm_lines) + new_rest, True


def sync_paired_req_references(
    new_roadmap_path: str, cfg: dict
) -> tuple[list[str], list[tuple[str, str]]]:
    """
    Após mover um roadmap, varre req_dir procurando REQs cujo frontmatter 'roadmap:'
    aponte para o basename do roadmap movido, e atualiza essas referências.

    Cobre layout flat (req_dir/*.md) e by_agent (req_dir/<agente>/<estado>/*.md),
    espelhando exatamente resolve_req_files do validator.

    Cardinalidades (todas pinadas no contrato cli-parity.md):
    - Zero REQs → no-op silencioso.
    - Uma      → reescreve.
    - Várias   → reescreve todas, na ordem de varredura.
    - Aponta para outro roadmap → não toca.
    - Já correta → nenhuma escrita (idempotente byte-a-byte).

    Retorna (synced, failures):
    - synced:   lista de basenames de REQs efetivamente reescritas.
    - failures: lista de (basename, causa) para REQs que falharam.
    """
    # Import escopado: validator não importa generators → sem ciclo.
    # (Inline seria necessário apenas se houvesse ciclo; não há, mas o import
    # dentro da função evita qualquer dependência circular futura.)
    from trackfw.validator import resolve_req_files  # noqa: PLC0415

    roadmap_basename = os.path.basename(new_roadmap_path)
    req_files = sorted(resolve_req_files(cfg), key=lambda p: (os.path.basename(p), p))

    synced: list[str] = []
    failures: list[tuple[str, str]] = []

    for req_path in req_files:
        try:
            with open(req_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            # Não consegue ler → não sabemos se aponta para nós → skip
            continue

        current_ref = _get_frontmatter_roadmap_value(content)
        if not current_ref:
            continue
        if os.path.basename(current_ref) != roadmap_basename:
            continue  # aponta para outro roadmap → não toca
        if current_ref == new_roadmap_path:
            continue  # já correta → idempotente, nenhuma escrita

        new_content, changed = _rewrite_req_roadmap_ref(content, new_roadmap_path)
        if not changed:
            continue  # sem campo a reescrever (frontmatter ausente)

        req_basename = os.path.basename(req_path)
        try:
            with open(req_path, "w", encoding="utf-8") as f:
                f.write(new_content)
            synced.append(req_basename)
        except OSError as e:
            failures.append((req_basename, str(e)))

    return synced, failures


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
        log_basename = os.path.join(agent, basename)
    else:
        target_dir = _state_dir(to_state, cfg)
        log_basename = basename

    os.makedirs(target_dir, exist_ok=True)
    dst = os.path.join(target_dir, basename)

    # Lê conteúdo, sincroniza status: no frontmatter (e cabeçalho no corpo) e escreve no destino.
    with open(src, "r", encoding="utf-8") as f:
        content = f.read()

    updated, _ = _rewrite_roadmap_status(content, to_state)

    with open(dst, "w", encoding="utf-8") as f:
        f.write(updated)

    os.remove(src)

    _append_transition_log(log_basename, from_state, to_state, cfg)

    return dst
