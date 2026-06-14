"""
traceid.py — Verificação bidirecional de req_id entre REQs e Roadmaps.
Parte do validador trackfw (ML-5C — v2.5).

Quando cfg["trace_id_field"] está definido, indexa o campo de frontmatter
configurado em REQs e Roadmaps e emite 5 tipos de violations:

  traceid_orphan_roadmap   — Roadmap com req_id sem REQ correspondente
  traceid_orphan_req       — REQ com req_id sem Roadmap correspondente
  traceid_state_mismatch   — mesmo req_id em estados diferentes (pasta diferente)
  traceid_duplicate_req    — mesmo req_id em mais de uma REQ
  traceid_duplicate_roadmap— mesmo req_id em mais de um Roadmap

Sem trace_id_field configurado → retorna lista vazia (comportamento inalterado).
"""

import os

# Estados canônicos de pastas de roadmap
_ROADMAP_STATES = ["backlog", "wip", "blocked", "done", "abandoned"]


def _parse_frontmatter(content: str) -> dict:
    """
    Extrai campos entre --- e --- do início do arquivo.
    Duplicado aqui para evitar importação circular com validator.py.
    """
    result = {}
    if not content.startswith("---"):
        return result
    lines = content.split("\n")
    in_block = False
    for i, line in enumerate(lines):
        stripped = line.strip()
        if i == 0 and stripped == "---":
            in_block = True
            continue
        if in_block:
            if stripped == "---":
                break
            colon_idx = stripped.find(":")
            if colon_idx >= 0:
                key = stripped[:colon_idx].strip().replace("-", "_")
                val = stripped[colon_idx + 1:].strip()
                result[key] = val
    return result


def _extract_trace_id(content: str, field: str) -> str:
    """
    Extrai o valor do campo `field:` do frontmatter.
    Retorna string vazia se não encontrado ou vazio.
    """
    fm = _parse_frontmatter(content)
    return fm.get(field, "").strip()


def _index_reqs(req_dir: str, field: str) -> list:
    """
    Lê todos os .md em req_dir (não recursivo).
    Retorna lista de dicts: {"file": basename, "path": caminho_completo,
                              "trace_id": valor, "state": "req"}.
    Inclui apenas arquivos onde trace_id não está vazio.
    """
    entries = []
    try:
        names = [n for n in os.listdir(req_dir)
                 if n.endswith(".md") and not os.path.isdir(os.path.join(req_dir, n))]
    except OSError:
        return entries

    for name in names:
        path = os.path.join(req_dir, name)
        try:
            with open(path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            continue
        trace_id = _extract_trace_id(content, field)
        if trace_id:
            entries.append({
                "file": name,
                "path": path,
                "trace_id": trace_id,
                "state": "req",
            })
    return entries


def _index_reqs_by_agent(req_dir: str, field: str, agents: list) -> list:
    """Indexa REQs em layout by_agent: req_dir/<agente>/<estado>/"""
    if not agents:
        try:
            agents = [e for e in os.listdir(req_dir)
                      if os.path.isdir(os.path.join(req_dir, e))]
        except OSError:
            return []
    entries = []
    for agent in agents:
        agent_dir = os.path.join(req_dir, agent)
        for state in _ROADMAP_STATES:
            state_dir = os.path.join(agent_dir, state)
            try:
                names = [n for n in os.listdir(state_dir)
                         if n.endswith(".md") and not os.path.isdir(os.path.join(state_dir, n))]
            except OSError:
                continue
            for name in names:
                path = os.path.join(state_dir, name)
                try:
                    with open(path, "r", encoding="utf-8") as f:
                        content = f.read()
                except OSError:
                    continue
                trace_id = _extract_trace_id(content, field)
                if trace_id:
                    entries.append({"file": name, "path": path, "trace_id": trace_id, "state": state})
    return entries


def _index_roadmaps(roadmap_dir: str, field: str) -> list:
    """
    Lê todos os .md em roadmap_dir/<state>/ para os estados canônicos.
    Retorna lista de dicts: {"file": basename, "path": caminho_completo,
                              "trace_id": valor, "state": estado_da_pasta}.
    Inclui apenas arquivos onde trace_id não está vazio.
    """
    entries = []
    for state in _ROADMAP_STATES:
        state_dir = os.path.join(roadmap_dir, state)
        try:
            names = [n for n in os.listdir(state_dir)
                     if n.endswith(".md") and not os.path.isdir(os.path.join(state_dir, n))]
        except OSError:
            continue
        for name in names:
            path = os.path.join(state_dir, name)
            try:
                with open(path, "r", encoding="utf-8") as f:
                    content = f.read()
            except OSError:
                continue
            trace_id = _extract_trace_id(content, field)
            if trace_id:
                entries.append({
                    "file": name,
                    "path": path,
                    "trace_id": trace_id,
                    "state": state,
                })
    return entries


def _index_roadmaps_by_agent(roadmap_dir: str, field: str, agents: list) -> list:
    """Indexa roadmaps em layout by_agent: roadmap_dir/<agente>/<estado>/"""
    if not agents:
        try:
            agents = [e for e in os.listdir(roadmap_dir)
                      if os.path.isdir(os.path.join(roadmap_dir, e))]
        except OSError:
            return []
    entries = []
    for agent in agents:
        agent_dir = os.path.join(roadmap_dir, agent)
        for state in _ROADMAP_STATES:
            state_dir = os.path.join(agent_dir, state)
            try:
                names = [n for n in os.listdir(state_dir)
                         if n.endswith(".md") and not os.path.isdir(os.path.join(state_dir, n))]
            except OSError:
                continue
            for name in names:
                path = os.path.join(state_dir, name)
                try:
                    with open(path, "r", encoding="utf-8") as f:
                        content = f.read()
                except OSError:
                    continue
                trace_id = _extract_trace_id(content, field)
                if trace_id:
                    entries.append({"file": name, "path": path, "trace_id": trace_id, "state": state})
    return entries


def _violation(rule: str, file: str, message: str) -> dict:
    return {"type": "violation", "rule": rule, "file": file, "message": message}


def check_traceid(cfg: dict) -> list:
    """
    Ponto de entrada público. Retorna lista de violations de rastreabilidade
    req_id ou [] quando trace_id_field não está configurado.
    """
    field = cfg.get("trace_id_field", "")
    if not field:
        return []

    req_dir = cfg.get("req_dir", "docs/req")
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")

    namespacing = cfg.get("roadmap_namespacing", "")
    agents = cfg.get("agents", [])
    if namespacing == "by_agent":
        req_entries = _index_reqs_by_agent(req_dir, field, agents)
        roadmap_entries = _index_roadmaps_by_agent(roadmap_dir, field, agents)
    else:
        req_entries = _index_reqs(req_dir, field)
        roadmap_entries = _index_roadmaps(roadmap_dir, field)

    if not req_entries and not roadmap_entries:
        return [_violation(
            "traceid_config_warning",
            "",
            "trace_id_field is set but no REQ/Roadmap entries were indexed"
            " — check req_dir, roadmap_dir and roadmap_namespacing"
        )]

    violations = []
    if not req_entries and roadmap_entries:
        violations.append(_violation(
            "traceid_config_warning", "",
            f"trace_id_field is set but REQs (0) were indexed while Roadmaps ({len(roadmap_entries)}) were"
            " — check req_dir and roadmap_namespacing"
        ))
    if not roadmap_entries and req_entries:
        violations.append(_violation(
            "traceid_config_warning", "",
            f"trace_id_field is set but Roadmaps (0) were indexed while REQs ({len(req_entries)}) were"
            " — check roadmap_dir and roadmap_namespacing"
        ))

    # --- traceid_duplicate_req: mesmo req_id em >1 REQ ---
    req_by_id: dict[str, list] = {}
    for e in req_entries:
        req_by_id.setdefault(e["trace_id"], []).append(e)
    for tid, group in req_by_id.items():
        if len(group) > 1:
            files = ", ".join(g["file"] for g in group)
            for e in group:
                violations.append(_violation(
                    "traceid_duplicate_req",
                    e["file"],
                    f'req_id "{tid}" is declared in multiple REQs: {files}'
                ))

    # --- traceid_duplicate_roadmap: mesmo req_id em >1 Roadmap ---
    roadmap_by_id: dict[str, list] = {}
    for e in roadmap_entries:
        roadmap_by_id.setdefault(e["trace_id"], []).append(e)
    for tid, group in roadmap_by_id.items():
        if len(group) > 1:
            files = ", ".join(g["file"] for g in group)
            for e in group:
                violations.append(_violation(
                    "traceid_duplicate_roadmap",
                    e["file"],
                    f'req_id "{tid}" is declared in multiple Roadmaps: {files}'
                ))

    # Índices de IDs únicos (para verificações cruzadas)
    req_ids = {e["trace_id"]: e for e in req_entries}
    roadmap_ids = {e["trace_id"]: e for e in roadmap_entries}

    # --- traceid_orphan_roadmap: Roadmap com req_id sem REQ correspondente ---
    for tid, e in roadmap_ids.items():
        if tid not in req_ids:
            violations.append(_violation(
                "traceid_orphan_roadmap",
                e["file"],
                f'roadmap "{e["file"]}" has req_id "{tid}" but no matching REQ was found'
            ))

    # --- traceid_orphan_req: REQ com req_id sem Roadmap correspondente ---
    for tid, e in req_ids.items():
        if tid not in roadmap_ids:
            violations.append(_violation(
                "traceid_orphan_req",
                e["file"],
                f'req "{e["file"]}" has req_id "{tid}" but no matching Roadmap was found'
            ))

    # --- traceid_state_mismatch: mesmo req_id em estados diferentes ---
    for tid in set(req_ids) & set(roadmap_ids):
        req_state = req_ids[tid]["state"]      # sempre "req"
        roadmap_state = roadmap_ids[tid]["state"]
        # REQ "state" é sempre "req"; estado canônico da REQ é a pasta (req_dir não tem sub-pastas).
        # A comparação é: pasta do roadmap deve casar com o estado da REQ
        # (como REQs ficam em uma única pasta sem subdivisão de estado,
        # usamos um mapeamento fixo: req_dir → "done" quando o arquivo tem
        # Status: Done, etc. Porém, como o campo "state" para REQs é sempre
        # "req" (não há sub-pasta), comparamos pelo campo status do frontmatter
        # vs a pasta do roadmap).
        #
        # Estratégia simples e robusta: ler o campo "status" do frontmatter da
        # REQ e comparar com a pasta do roadmap, case-insensitive.
        try:
            with open(req_ids[tid]["path"], "r", encoding="utf-8") as f:
                req_content = f.read()
        except OSError:
            continue
        fm = _parse_frontmatter(req_content)
        req_status = fm.get("status", "").strip().lower()
        # Mapear status do frontmatter para pasta canônica
        _status_to_folder = {
            "wip": "wip", "in progress": "wip",
            "backlog": "backlog",
            "blocked": "blocked",
            "done": "done",
            "abandoned": "abandoned",
        }
        req_folder = _status_to_folder.get(req_status, req_status)
        if req_folder and req_folder != roadmap_state:
            req_e = req_ids[tid]
            rm_e = roadmap_ids[tid]
            violations.append(_violation(
                "traceid_state_mismatch",
                rm_e["file"],
                f'req_id "{tid}": REQ "{req_e["file"]}" has status "{req_status}" '
                f'but Roadmap "{rm_e["file"]}" is in folder "{roadmap_state}"'
            ))

    return violations
