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
    """Parse linha a linha do conteúdo YAML, espelhando a lógica do config/index.js."""
    lines = content.split("\n")
    in_adr_dirs = False
    in_agents = False
    adr_dirs = []
    agents = []

    for raw_line in lines:
        line = raw_line.strip()

        if in_adr_dirs:
            if line.startswith("- "):
                adr_dirs.append(line[2:].strip())
                continue
            in_adr_dirs = False
            if adr_dirs:
                cfg["adr_dirs"] = adr_dirs

        if in_agents:
            if line.startswith("- "):
                agents.append(line[2:].strip())
                continue
            in_agents = False
            if agents:
                cfg["agents"] = agents

        colon_idx = line.find(":")
        if colon_idx < 0:
            continue
        key = line[:colon_idx].strip()
        val = line[colon_idx + 1:].strip()
        if not key:
            continue

        if key == "adr_dirs":
            in_adr_dirs = True
            adr_dirs = []
        elif key == "req_dir":
            cfg["req_dir"] = val
        elif key == "roadmap_dir":
            cfg["roadmap_dir"] = val
        elif key == "roadmap_namespacing":
            cfg["roadmap_namespacing"] = val
        elif key == "agents":
            in_agents = True
            agents = []
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

    # flush pending lists at EOF
    if in_adr_dirs and adr_dirs:
        cfg["adr_dirs"] = adr_dirs
    if in_agents and agents:
        cfg["agents"] = agents
