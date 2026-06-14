"""
serve/api_chain.py — Chain API: retorna grafo ADR -> REQ -> ROADMAP.
Espelho Python de internal/serve/api_chain.go e npm/src/serve/api_chain.js.
"""

import os
import re

STATES = ["wip", "backlog", "blocked", "done", "abandoned"]


def _extract_frontmatter(content):
    """
    Extrai campos do bloco frontmatter (entre --- e ---).
    Retorna dict com os campos encontrados.
    Suporta valores simples e listas (- item).
    """
    fields = {}
    lines = content.split("\n")
    if not lines or lines[0].strip() != "---":
        return fields

    in_frontmatter = False
    current_key = None
    current_list = []

    for i, raw_line in enumerate(lines):
        line = raw_line.strip()
        if i == 0:
            in_frontmatter = True
            continue
        if line == "---":
            # Flush lista pendente
            if current_key and current_list:
                fields[current_key] = current_list
            break
        if not in_frontmatter:
            break

        if line.startswith("- "):
            # item de lista
            if current_key:
                current_list.append(line[2:].strip().strip("\"'"))
            continue

        colon_idx = line.find(":")
        if colon_idx > 0:
            # Flush lista anterior
            if current_key and current_list:
                fields[current_key] = current_list

            key = line[:colon_idx].strip()
            val = line[colon_idx + 1:].strip().strip("\"'")
            current_key = key
            current_list = []
            if val:
                fields[key] = val
                current_key = None  # valor inline, não lista

    return fields


def _extract_title(content, filename):
    """Extrai título da primeira linha '# ...' ou usa o nome do arquivo."""
    for line in content.split("\n"):
        stripped = line.strip()
        if stripped.startswith("# "):
            return stripped[2:].strip()
    return os.path.splitext(filename)[0]


def _scan_dir(dir_path, node_type, state):
    """Varre um diretório e retorna lista de nodes com seus campos frontmatter."""
    nodes = []
    if not os.path.isdir(dir_path):
        return nodes
    try:
        files = sorted(
            f for f in os.listdir(dir_path)
            if f.endswith(".md") and not os.path.isdir(os.path.join(dir_path, f))
        )
    except OSError:
        return nodes

    for filename in files:
        full_path = os.path.join(dir_path, filename)
        content = ""
        try:
            with open(full_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            pass

        fm = _extract_frontmatter(content)
        title = _extract_title(content, filename)
        rel_path = os.path.relpath(full_path, os.getcwd()).replace("\\", "/")

        node = {
            "id": rel_path,
            "type": node_type,
            "title": title,
            "state": state,
            "frontmatter": fm,
        }
        nodes.append(node)

    return nodes


def get_chain(cfg):
    """
    Retorna grafo { nodes: [...], edges: [...] } representando
    a cadeia ADR → REQ → ROADMAP.
    """
    adr_dirs = cfg.get("adr_dirs", ["docs/adr"])
    req_dir = cfg.get("req_dir", "docs/req")
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    namespacing = cfg.get("roadmap_namespacing", "flat")

    nodes = []
    edges = []

    # --- ADRs ---
    for adr_dir in adr_dirs:
        # Suporte a by_agent: verificar subpastas done/wip/...
        if namespacing == "by_agent":
            agents = cfg.get("agents") or []
            if not agents:
                try:
                    agents = sorted(
                        e for e in os.listdir(adr_dir)
                        if os.path.isdir(os.path.join(adr_dir, e))
                    )
                except OSError:
                    agents = []
            for agent in agents:
                for state in STATES:
                    nodes.extend(_scan_dir(os.path.join(adr_dir, agent, state), "adr", state))
        else:
            # flat: pode haver subpastas done/wip ou arquivos direto
            has_state_dirs = any(
                os.path.isdir(os.path.join(adr_dir, s)) for s in STATES
            )
            if has_state_dirs:
                for state in STATES:
                    nodes.extend(_scan_dir(os.path.join(adr_dir, state), "adr", state))
            else:
                nodes.extend(_scan_dir(adr_dir, "adr", "done"))

    # --- REQs ---
    if namespacing == "by_agent":
        agents = cfg.get("agents") or []
        if not agents:
            try:
                agents = sorted(
                    e for e in os.listdir(req_dir)
                    if os.path.isdir(os.path.join(req_dir, e))
                )
            except OSError:
                agents = []
        for agent in agents:
            for state in STATES:
                nodes.extend(_scan_dir(os.path.join(req_dir, agent, state), "req", state))
    else:
        has_state_dirs = any(os.path.isdir(os.path.join(req_dir, s)) for s in STATES)
        if has_state_dirs:
            for state in STATES:
                nodes.extend(_scan_dir(os.path.join(req_dir, state), "req", state))
        else:
            nodes.extend(_scan_dir(req_dir, "req", "unknown"))

    # --- Roadmaps ---
    if namespacing == "by_agent":
        agents = cfg.get("agents") or []
        if not agents:
            try:
                agents = sorted(
                    e for e in os.listdir(roadmap_dir)
                    if os.path.isdir(os.path.join(roadmap_dir, e))
                )
            except OSError:
                agents = []
        for agent in agents:
            for state in STATES:
                nodes.extend(_scan_dir(os.path.join(roadmap_dir, agent, state), "roadmap", state))
    else:
        for state in STATES:
            nodes.extend(_scan_dir(os.path.join(roadmap_dir, state), "roadmap", state))

    # --- Construir índice id → node ---
    by_id = {n["id"]: n for n in nodes}

    # --- Construir índice basename → node (para match por nome de arquivo) ---
    by_basename = {}
    for n in nodes:
        basename = os.path.basename(n["id"])
        by_basename.setdefault(basename, []).append(n)

    def _find_node_by_ref(ref):
        """Tenta encontrar node pelo id exato ou pelo basename."""
        ref = ref.strip()
        if ref in by_id:
            return by_id[ref]
        # tenta basename
        candidates = by_basename.get(ref, []) or by_basename.get(ref + ".md", [])
        if candidates:
            return candidates[0]
        return None

    # --- Construir arestas ---
    for node in nodes:
        fm = node.get("frontmatter", {})

        # REQ → ADR
        if node["type"] == "req":
            adr_ref = fm.get("adr", "")
            if isinstance(adr_ref, str) and adr_ref:
                target = _find_node_by_ref(adr_ref)
                if target:
                    edges.append({"from": node["id"], "to": target["id"]})
            elif isinstance(adr_ref, list):
                for ref in adr_ref:
                    target = _find_node_by_ref(ref)
                    if target:
                        edges.append({"from": node["id"], "to": target["id"]})

        # ROADMAP → REQ
        if node["type"] == "roadmap":
            req_ref = fm.get("req", "")
            if isinstance(req_ref, str) and req_ref:
                target = _find_node_by_ref(req_ref)
                if target:
                    edges.append({"from": node["id"], "to": target["id"]})
            elif isinstance(req_ref, list):
                for ref in req_ref:
                    target = _find_node_by_ref(ref)
                    if target:
                        edges.append({"from": node["id"], "to": target["id"]})

            # ROADMAP → ADR (link direto)
            adr_ref = fm.get("adr", "")
            if isinstance(adr_ref, str) and adr_ref:
                target = _find_node_by_ref(adr_ref)
                if target:
                    edges.append({"from": node["id"], "to": target["id"]})
            elif isinstance(adr_ref, list):
                for ref in adr_ref:
                    target = _find_node_by_ref(ref)
                    if target:
                        edges.append({"from": node["id"], "to": target["id"]})

    # Remover frontmatter do output (não deve ir para o cliente)
    output_nodes = [
        {k: v for k, v in n.items() if k != "frontmatter"}
        for n in nodes
    ]

    return {"nodes": output_nodes, "edges": edges}
