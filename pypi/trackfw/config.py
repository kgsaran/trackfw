"""
config.py — Leitura de trackfw.yaml, espelhando npm/src/config/index.js.
Parse linha a linha, sem dependências externas de YAML.
"""

import os

NAMESPACING_FLAT = "flat"
NAMESPACING_BY_AGENT = "by_agent"

_instance = None


def defaults():
    """Retorna dict com valores padrão de configuração."""
    return {
        # campos existentes
        "adr_dirs": ["docs/adr"],
        "req_dir": "docs/req",
        "roadmap_dir": "docs/roadmaps",
        "roadmap_namespacing": "flat",
        "agents": [],
        "governance_mode": "",
        "lenient_until": "",
        "wip_limit": 1,
        "wip_by_squad": False,
        "require_req_in_commit": False,
        # novos campos
        "link_fields": {
            "req":     ["REQ:"],
            "adr":     ["ADR:"],
            "roadmap": ["Roadmap:"],
        },
        "acceptance_markers": ["## Acceptance Criteria", "## Critérios de Aceite"],
        "rules": {
            "wip_has_req":          "error",
            "wip_acceptance":       "error",
            "wip_limit":            "error",
            "stale_wip":            "warning",
            "adr_orphan":           "warning",
            "ref_targets_exist":    "warning",
            "folder_status":        "warning",
            "filename_uniqueness":  "error",
            "blocked_by_draft_adr": "error",
        },
    }


def load(cwd=None):
    """
    Carrega trackfw.yaml do diretório cwd (default: os.getcwd()).
    Singleton: segunda chamada retorna o mesmo objeto.
    """
    global _instance
    if _instance is not None:
        return _instance

    _instance = defaults()
    yaml_path = os.path.join(cwd or os.getcwd(), "trackfw.yaml")
    if not os.path.exists(yaml_path):
        return _instance

    with open(yaml_path, "r", encoding="utf-8") as f:
        content = f.read()

    _parse(content, _instance)
    return _instance


def reset():
    """Zera o singleton (útil em testes)."""
    global _instance
    _instance = None


def _parse(content, cfg):
    """Parse linha a linha do conteúdo YAML, espelhando a lógica do config/index.js.
    Suporta blocos aninhados de 1 nível: link_fields, acceptance_markers, rules.

    Itens de lista podem ou não ter indentação — ambos os formatos são aceitos:
        agents:          agents:
          - zeus    OU   - zeus
          - apolo        - apolo
    """
    lines = content.split("\n")

    # estados existentes
    in_adr_dirs = False
    in_agents = False
    adr_dirs = []
    agents = []

    # novos estados
    in_link_fields = False
    in_link_fields_req = False
    in_link_fields_adr = False
    in_link_fields_roadmap = False
    link_fields_req = []
    link_fields_adr = []
    link_fields_roadmap = []

    in_acceptance_markers = False
    acceptance_markers = []

    in_rules = False
    rules = {}

    def _flush_link_fields_sub():
        """Flush sub-campos de link_fields ativos para cfg."""
        nonlocal in_link_fields_req, in_link_fields_adr, in_link_fields_roadmap
        if in_link_fields_req and link_fields_req:
            cfg["link_fields"]["req"] = link_fields_req[:]
            link_fields_req.clear()
        if in_link_fields_adr and link_fields_adr:
            cfg["link_fields"]["adr"] = link_fields_adr[:]
            link_fields_adr.clear()
        if in_link_fields_roadmap and link_fields_roadmap:
            cfg["link_fields"]["roadmap"] = link_fields_roadmap[:]
            link_fields_roadmap.clear()
        in_link_fields_req = False
        in_link_fields_adr = False
        in_link_fields_roadmap = False

    def flush_blocks():
        nonlocal in_adr_dirs, adr_dirs, in_agents, agents
        nonlocal in_link_fields
        nonlocal in_acceptance_markers, acceptance_markers, in_rules, rules

        if in_adr_dirs and adr_dirs:
            cfg["adr_dirs"] = adr_dirs[:]
        if in_agents and agents:
            cfg["agents"] = agents[:]
        if in_link_fields:
            _flush_link_fields_sub()
        if in_acceptance_markers and acceptance_markers:
            cfg["acceptance_markers"] = acceptance_markers[:]
        if in_rules and rules:
            cfg["rules"].update(rules)

        in_adr_dirs = False
        adr_dirs.clear()
        in_agents = False
        agents.clear()
        in_link_fields = False
        in_acceptance_markers = False
        acceptance_markers.clear()
        in_rules = False
        rules.clear()

    for raw_line in lines:
        line = raw_line.strip()
        if not line:
            continue
        has_indent = len(raw_line) > 0 and raw_line[0] in (' ', '\t')

        # Itens de lista simples podem aparecer sem indentação (ex: "- zeus")
        # ou com indentação (ex: "  - zeus"). Tratamos ambos os casos.
        # Um item de lista não pode ser uma nova chave top-level: chaves não
        # começam com "- ".
        is_list_item = line.startswith("- ")

        # Se linha top-level e não é item de lista, encerra blocos anteriores.
        if not has_indent and not is_list_item:
            flush_blocks()

        # --- Processamento de itens de lista (indentados ou não) ---
        if is_list_item:
            val = line[2:].strip()
            if in_adr_dirs:
                adr_dirs.append(val)
                continue
            if in_agents:
                agents.append(val)
                continue
            if in_acceptance_markers:
                acceptance_markers.append(val.strip('"\''))
                continue
            if in_link_fields:
                clean_val = val.strip('"\'')
                if in_link_fields_req:
                    link_fields_req.append(clean_val)
                elif in_link_fields_adr:
                    link_fields_adr.append(clean_val)
                elif in_link_fields_roadmap:
                    link_fields_roadmap.append(clean_val)
                continue
            # item de lista sem bloco ativo — ignorar
            continue

        # --- Processamento de linhas indentadas não-lista (sub-chaves) ---
        if has_indent:
            if in_rules:
                colon_idx = line.find(":")
                if colon_idx > 0:
                    k = line[:colon_idx].strip()
                    v = line[colon_idx + 1:].strip()
                    if k:
                        rules[k] = v
                continue
            if in_link_fields:
                # sub-chave dentro de link_fields (ex: "  req:", "  adr:")
                colon_idx = line.find(":")
                sub_key = line[:colon_idx].strip() if colon_idx > 0 else line.replace(":", "").strip()
                # flush sub-campo anterior antes de mudar
                _flush_link_fields_sub()
                if sub_key == "req":
                    in_link_fields_req = True
                elif sub_key == "adr":
                    in_link_fields_adr = True
                elif sub_key == "roadmap":
                    in_link_fields_roadmap = True
                continue
            continue

        # --- Linha top-level (chave: valor) ---
        colon_idx = line.find(":")
        if colon_idx < 0:
            continue
        key = line[:colon_idx].strip()
        val = line[colon_idx + 1:].strip()
        if not key:
            continue

        if key == "adr_dirs":
            in_adr_dirs = True
            adr_dirs.clear()
        elif key == "req_dir":
            cfg["req_dir"] = val
        elif key == "roadmap_dir":
            cfg["roadmap_dir"] = val
        elif key == "roadmap_namespacing":
            cfg["roadmap_namespacing"] = val
        elif key == "agents":
            in_agents = True
            agents.clear()
        elif key == "governance_mode":
            cfg["governance_mode"] = val
        elif key == "lenient_until":
            cfg["lenient_until"] = val
        elif key == "wip_limit":
            try:
                n = int(val)
                if n > 0:
                    cfg["wip_limit"] = n
            except ValueError:
                pass
        elif key == "wip_by_squad":
            cfg["wip_by_squad"] = val == "true"
        elif key == "require_req_in_commit":
            cfg["require_req_in_commit"] = val == "true"
        elif key == "link_fields":
            in_link_fields = True
        elif key == "acceptance_markers":
            in_acceptance_markers = True
            acceptance_markers.clear()
        elif key == "rules":
            in_rules = True
            rules.clear()

    # flush final (EOF)
    flush_blocks()
