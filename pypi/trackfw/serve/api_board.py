"""
serve/api_board.py — Board API: retorna roadmaps agrupados por estado (kanban).
Espelho Python de internal/serve/api_board.go e npm/src/serve/api_board.js.
"""

import os

STATES = ["wip", "backlog", "blocked", "done", "abandoned"]


def _extract_title(content, filename):
    """Extrai título da primeira linha '# ...' ou usa o nome do arquivo."""
    for line in content.split("\n"):
        stripped = line.strip()
        if stripped.startswith("# "):
            return stripped[2:].strip()
    # fallback: nome sem extensão
    return os.path.splitext(filename)[0]


def _scan_state_dir(dir_path, state, agent=None):
    """Varre um diretório de estado e retorna lista de cards."""
    cards = []
    if not os.path.isdir(dir_path):
        return cards
    try:
        files = sorted(
            f for f in os.listdir(dir_path)
            if f.endswith(".md") and not os.path.isdir(os.path.join(dir_path, f))
        )
    except OSError:
        return cards

    for filename in files:
        full_path = os.path.join(dir_path, filename)
        content = ""
        try:
            with open(full_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            pass

        title = _extract_title(content, filename)
        # path relativo para uso pelo frontend
        rel_path = os.path.relpath(full_path, os.getcwd()).replace("\\", "/")

        card = {
            "file": filename,
            "title": title,
            "state": state,
            "agent": agent or "",
            "path": rel_path,
        }
        cards.append(card)

    return cards


def get_board(cfg):
    """
    Retorna dict com 'columns' (por estado) e 'agents' detectados.
    Suporta namespacing flat e by_agent.
    """
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    namespacing = cfg.get("roadmap_namespacing", "flat")

    columns = {state: [] for state in STATES}
    agents_found = set()

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
                dir_path = os.path.join(roadmap_dir, agent, state)
                cards = _scan_state_dir(dir_path, state, agent)
                if cards:
                    agents_found.add(agent)
                columns[state].extend(cards)
    else:
        for state in STATES:
            dir_path = os.path.join(roadmap_dir, state)
            cards = _scan_state_dir(dir_path, state)
            columns[state].extend(cards)

    return {
        "columns": columns,
        "agents": sorted(agents_found),
    }
