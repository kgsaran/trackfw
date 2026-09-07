"""
serve/api_chain.py — Chain API: retorna grafo ADR -> REQ -> ROADMAP.
Espelho Python de internal/serve/api_chain.go e npm/src/serve/api_chain.js.
"""

import os
import re

from trackfw import config as _config
from trackfw.pathfmt import normalize_ref_separator
from trackfw.validator import extract_ref_path

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
        # node ID do grafo — IDENTIFICADOR emitido em JSON (ADR-2026-09-04, D1
        # categoria 2). Antes do ML-2A a normalização era um .replace() inline aqui;
        # passou a chamar o ponto único do runtime (D3: "não espalhar ReplaceAll pelos
        # chamadores"). Comportamento idêntico, uma noção de formato a menos.
        #
        # 🔴 full_path (acima) permanece nativo e é o que vai a open() — a normalização
        # é de saída, não de travessia (ADR D2).
        rel_path = normalize_ref_separator(os.path.relpath(full_path, os.getcwd()))

        node = {
            "id": rel_path,
            "type": node_type,
            "title": title,
            "state": state,
            "frontmatter": fm,
            "content": content,
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
            agents = _config.resolve_agent_namespaces(cfg, adr_dir)
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
        agents = _config.resolve_agent_namespaces(cfg, req_dir)
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
        agents = _config.resolve_agent_namespaces(cfg, roadmap_dir)
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
        """Tenta encontrar node pelo id exato ou pelo basename.

        ML-3D: basename(ref), não ref cru — o formato canônico gravado por
        `trackfw req new`/`roadmap new` (ADR-2026-08-01) é o CAMINHO COMPLETO
        (ex.: "docs/roadmaps/wip/ROADMAP-x.md"), não um basename isolado. Sem
        aplicar basename() aqui, um vínculo real nunca batia contra as chaves
        de by_basename (que são sempre basenames) — a mesma classe de defeito
        corrigida em npm/src/serve/api_chain.js:resolveRef.
        """
        ref = ref.strip()
        if ref in by_id:
            return by_id[ref]
        base = os.path.basename(ref)
        candidates = by_basename.get(base, []) or by_basename.get(base + ".md", [])
        if candidates:
            return candidates[0]
        return None

    edge_seen = set()

    def _add_edge(from_id, to_id):
        if from_id == to_id:
            return
        key = (from_id, to_id)
        if key in edge_seen:
            return
        edge_seen.add(key)
        edges.append({"from": from_id, "to": to_id})

    def _resolve_field_edges(node, field, fm_ref):
        """Resolve um campo de vínculo (req/adr/roadmap) para o node, tentando o
        frontmatter primeiro (fm_ref) e, se vazio, o valor extraído do CORPO do
        arquivo via extract_ref_path.

        ML-3D — achado: `trackfw req new` (trackfw/generators/req.py) grava
        `adr: ""` e `roadmap: ""` SEMPRE vazios no frontmatter; o valor real
        vive em "## Linked ADR / ADR: <path>" e "## Linked Roadmap /
        Roadmap: <path>", no corpo. fm_ref sozinho NUNCA resolvia o vínculo
        de uma REQ gerada pelo próprio CLI — não era caso de borda, era o
        formato canônico inteiro nunca resolvendo.
        """
        refs = []
        if isinstance(fm_ref, str) and fm_ref:
            refs.append(fm_ref)
        elif isinstance(fm_ref, list):
            refs.extend(r for r in fm_ref if r)
        if not refs:
            body_ref = extract_ref_path(node.get("content", ""), field)
            if body_ref:
                refs.append(body_ref)
        for ref in refs:
            target = _find_node_by_ref(ref)
            if target:
                _add_edge(node["id"], target["id"])

    # --- Construir arestas ---
    for node in nodes:
        fm = node.get("frontmatter", {})

        # REQ → ADR
        if node["type"] == "req":
            _resolve_field_edges(node, "ADR", fm.get("adr", ""))
            # REQ → ROADMAP (achado do ML-3D: faltava por completo — nem o campo de
            # frontmatter nem o do corpo eram lidos para REQ→Roadmap antes desta correção)
            _resolve_field_edges(node, "Roadmap", fm.get("roadmap", ""))

        # ROADMAP → REQ / ROADMAP → ADR
        if node["type"] == "roadmap":
            _resolve_field_edges(node, "REQ", fm.get("req", ""))
            _resolve_field_edges(node, "ADR", fm.get("adr", ""))

    # Remover frontmatter e conteúdo do output (não deve ir para o cliente)
    output_nodes = [
        {k: v for k, v in n.items() if k not in ("frontmatter", "content")}
        for n in nodes
    ]

    return {"nodes": output_nodes, "edges": edges}
