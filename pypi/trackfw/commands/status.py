"""
status.py — Comando `trackfw status`.

Exibe contagens agregadas de ADRs, REQs e Roadmaps.
Suporta modo flat e by_agent (roadmap_namespacing).
"""

import os

from .. import config as _config
from .. import validator as _validator


def _list_files(path: str) -> list:
    """Retorna lista de arquivos (não-diretórios) em path. Retorna [] se não existir."""
    try:
        entries = []
        for name in os.listdir(path):
            full = os.path.join(path, name)
            if not os.path.isdir(full):
                entries.append(name)
        return entries
    except OSError:
        return []


def _list_dirs(path: str) -> list:
    """Retorna lista de subdiretórios em path. Retorna [] se não existir."""
    try:
        return [
            name for name in os.listdir(path)
            if os.path.isdir(os.path.join(path, name))
        ]
    except OSError:
        return []


def _count_reqs_by_status(req_dir: str) -> dict:
    """
    Conta REQs e agrupa por Status (Open/Closed/etc.) lendo o frontmatter.
    Retorna {"total": N, "open": X, "closed": Y, "other": Z}.
    """
    files = _list_files(req_dir)
    counts = {"total": len(files), "open": 0, "closed": 0, "other": 0}
    for name in files:
        path = os.path.join(req_dir, name)
        try:
            with open(path, "r", encoding="utf-8") as f:
                content = f.read()
            fm = _validator.parse_frontmatter(content)
            status = fm.get("status", "").lower()
            if status == "open":
                counts["open"] += 1
            elif status in ("closed", "done"):
                counts["closed"] += 1
            else:
                counts["other"] += 1
        except OSError:
            counts["other"] += 1
    return counts


def _count_adrs(adr_dirs: list) -> int:
    """Conta total de ADRs em todos os adr_dirs."""
    total = 0
    for adr_dir in adr_dirs:
        total += len(_list_files(adr_dir))
    return total


def _roadmap_counts_flat(roadmap_dir: str) -> dict:
    """Conta roadmaps por estado no modo flat."""
    states = ["backlog", "wip", "blocked", "done", "abandoned"]
    return {state: len(_list_files(os.path.join(roadmap_dir, state))) for state in states}


def _roadmap_counts_by_agent(roadmap_dir: str, agents: list) -> dict:
    """
    Retorna dict: agent → {state: count}.
    """
    states = ["backlog", "wip", "blocked", "done", "abandoned"]
    result = {}
    for agent in agents:
        agent_dir = os.path.join(roadmap_dir, agent)
        result[agent] = {
            state: len(_list_files(os.path.join(agent_dir, state)))
            for state in states
        }
    return result


def _get_agents(cfg: dict) -> list:
    """Descobre os agentes: da config ou do filesystem."""
    agents = cfg.get("agents") or []
    if not agents:
        roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
        agents = _list_dirs(roadmap_dir)
    return agents


def _resolve(base: str, path: str) -> str:
    """Resolve path relativo a base se não for absoluto."""
    if os.path.isabs(path):
        return path
    return os.path.join(base, path)


def get_status(cwd: str = None) -> str:
    """
    Retorna string formatada com o status de governança do projeto.
    Espelha o comportamento de getStatus() do npm/src/validator/index.js,
    mas com formato de dashboard agregado conforme especificação Python.
    """
    _config.reset()
    cfg = _config.load(cwd)

    base = cwd or os.getcwd()
    roadmap_dir = _resolve(base, cfg.get("roadmap_dir", "docs/roadmaps"))
    req_dir = _resolve(base, cfg.get("req_dir", "docs/req"))
    adr_dirs = [_resolve(base, d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    namespacing = cfg.get("roadmap_namespacing", _config.NAMESPACING_FLAT)

    adr_count = _count_adrs(adr_dirs)
    req_counts = _count_reqs_by_status(req_dir)

    lines = ["Governance Status", "─────────────────"]
    lines.append(f"ADRs:      {adr_count}")

    req_detail = f"{req_counts['open']} Open, {req_counts['closed']} Closed"
    if req_counts["other"] > 0:
        req_detail += f", {req_counts['other']} Other"
    lines.append(f"REQs:      {req_counts['total']} ({req_detail})")

    lines.append("Roadmaps:")

    if namespacing == _config.NAMESPACING_BY_AGENT:
        agents = _get_agents(cfg)
        by_agent = _roadmap_counts_by_agent(roadmap_dir, agents)

        # Totais agregados por estado
        states = ["backlog", "wip", "blocked", "done", "abandoned"]
        totals = {state: sum(by_agent[a][state] for a in agents) for state in states}

        lines.append(f"  backlog:  {totals['backlog']}")
        lines.append(f"  wip:      {totals['wip']}")
        lines.append(f"  blocked:  {totals['blocked']}")
        lines.append(f"  done:     {totals['done']}")
        lines.append(f"  abandoned: {totals['abandoned']}")

        lines.append("Roadmaps (by agent):")
        for agent, counts in by_agent.items():
            active = [
                f"{state}={counts[state]}"
                for state in states
                if counts[state] > 0
            ]
            if active:
                lines.append(f"  {agent}:   {' '.join(active)}")
            else:
                lines.append(f"  {agent}:   (empty)")
    else:
        counts = _roadmap_counts_flat(roadmap_dir)
        lines.append(f"  backlog:  {counts['backlog']}")
        lines.append(f"  wip:      {counts['wip']}")
        lines.append(f"  blocked:  {counts['blocked']}")
        lines.append(f"  done:     {counts['done']}")
        lines.append(f"  abandoned: {counts['abandoned']}")

    return "\n".join(lines)


def register(subparsers):
    """Registra o subcomando 'status' no parser principal."""
    parser = subparsers.add_parser(
        "status",
        help="Exibe o status de governança do projeto",
    )
    parser.set_defaults(func=run)
    return parser


def run(args):
    """Executa o status e imprime o resultado."""
    print(get_status())
    return 0
