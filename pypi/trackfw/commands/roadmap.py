"""
roadmap.py — Subcomandos CLI para roadmap.
Espelha npm/src/commands/roadmap.js em Python puro.
"""

import os
import sys

from trackfw import config as cfg_module
from trackfw.generators.roadmap import (
    generate_roadmap,
    move_roadmap,
    VALID_STATES,
)


# ---------------------------------------------------------------------------
# helpers de listagem
# ---------------------------------------------------------------------------

def _list_flat(roadmap_dir: str, filter_state: str = None) -> list[tuple[str, str, str]]:
    """Retorna lista de (state, '', filename) em modo flat."""
    results = []
    states = [filter_state] if filter_state else VALID_STATES
    for state in states:
        d = os.path.join(roadmap_dir, state)
        if not os.path.isdir(d):
            continue
        for f in sorted(os.listdir(d)):
            if f.endswith(".md"):
                results.append((state, "", f))
    return results


def _list_by_agent(roadmap_dir: str, filter_state: str = None, agents=None) -> list[tuple[str, str, str]]:
    """Retorna lista de (state, agent, filename) em modo by_agent."""
    results = []
    if not agents:
        try:
            agents = [
                e for e in os.listdir(roadmap_dir)
                if os.path.isdir(os.path.join(roadmap_dir, e))
            ]
        except OSError:
            agents = []

    states = [filter_state] if filter_state else VALID_STATES
    for agent in sorted(agents):
        for state in states:
            d = os.path.join(roadmap_dir, agent, state)
            if not os.path.isdir(d):
                continue
            for f in sorted(os.listdir(d)):
                if f.endswith(".md"):
                    results.append((state, agent, f))
    return results


def _find_file(name: str, roadmap_dir: str, namespacing: str, agents=None) -> str | None:
    """
    Encontra o path completo de um arquivo de roadmap pelo nome (ou parte dele).
    Retorna None se não encontrado.
    """
    name_lower = name.lower()
    if namespacing == "by_agent":
        entries = _list_by_agent(roadmap_dir, agents=agents)
        for state, agent, fname in entries:
            if name_lower in fname.lower():
                return os.path.join(roadmap_dir, agent, state, fname)
    else:
        entries = _list_flat(roadmap_dir)
        for state, _, fname in entries:
            if name_lower in fname.lower():
                return os.path.join(roadmap_dir, state, fname)
    return None


# ---------------------------------------------------------------------------
# handlers de subcomandos
# ---------------------------------------------------------------------------

def _cmd_new(args):
    cfg = cfg_module.load()
    title = " ".join(args.title) if isinstance(args.title, list) else args.title
    agent = getattr(args, "agent", None)
    try:
        path = generate_roadmap(title, cfg, agent=agent)
        print(f"Roadmap criado: {path}")
    except Exception as e:
        print(f"Erro ao criar roadmap: {e}", file=sys.stderr)
        sys.exit(1)


def _cmd_move(args):
    cfg = cfg_module.load()
    filename = args.filename
    state = args.state
    try:
        new_path = move_roadmap(filename, state, cfg)
        print(f"Roadmap movido para: {new_path}")
    except (ValueError, FileNotFoundError) as e:
        print(f"Erro: {e}", file=sys.stderr)
        sys.exit(1)


def _cmd_list(args):
    cfg = cfg_module.load()
    roadmap_dir = cfg["roadmap_dir"]
    namespacing = cfg.get("roadmap_namespacing", "flat")
    filter_state = getattr(args, "state", None)

    if not os.path.isdir(roadmap_dir):
        print(f"Diretório de roadmaps nao encontrado: {roadmap_dir}", file=sys.stderr)
        sys.exit(1)

    if namespacing == "by_agent":
        agents = cfg.get("agents") or []
        entries = _list_by_agent(roadmap_dir, filter_state=filter_state, agents=agents or None)
        if not entries:
            print("Nenhum roadmap encontrado.")
            return
        current_agent = None
        for state, agent, fname in entries:
            if agent != current_agent:
                print(f"\n[{agent}]")
                current_agent = agent
            print(f"  [{state}] {fname}")
    else:
        entries = _list_flat(roadmap_dir, filter_state=filter_state)
        if not entries:
            print("Nenhum roadmap encontrado.")
            return
        current_state = None
        for state, _, fname in entries:
            if state != current_state:
                print(f"\n[{state}]")
                current_state = state
            print(f"  {fname}")


def _cmd_show(args):
    cfg = cfg_module.load()
    roadmap_dir = cfg["roadmap_dir"]
    namespacing = cfg.get("roadmap_namespacing", "flat")
    agents = cfg.get("agents") or []

    path = _find_file(args.filename, roadmap_dir, namespacing, agents=agents or None)
    if not path:
        print(f"Roadmap nao encontrado: {args.filename}", file=sys.stderr)
        sys.exit(1)

    try:
        with open(path, encoding="utf-8") as f:
            print(f.read())
    except OSError as e:
        print(f"Erro ao ler arquivo: {e}", file=sys.stderr)
        sys.exit(1)


# ---------------------------------------------------------------------------
# registro no argparse
# ---------------------------------------------------------------------------

def register(subparsers):
    """Registra o comando 'roadmap' e seus subcomandos no argparse."""
    roadmap_parser = subparsers.add_parser(
        "roadmap",
        help="Gerencia roadmaps de governança",
    )
    sub = roadmap_parser.add_subparsers(dest="roadmap_cmd", metavar="SUBCOMMAND")

    # roadmap new <title> [--agent AGENT]
    new_p = sub.add_parser("new", help="Cria um novo roadmap em backlog/")
    new_p.add_argument("title", nargs="+", help="Titulo do roadmap")
    new_p.add_argument("--agent", default=None, help="Agente responsavel (modo by_agent)")
    new_p.set_defaults(func=_cmd_new)

    # roadmap move <filename> <state>
    move_p = sub.add_parser("move", help="Move um roadmap entre estados")
    move_p.add_argument("filename", help="Nome do arquivo do roadmap")
    move_p.add_argument("state", choices=VALID_STATES, help="Estado de destino")
    move_p.set_defaults(func=_cmd_move)

    # roadmap list [--state STATE]
    list_p = sub.add_parser("list", help="Lista roadmaps por estado")
    list_p.add_argument(
        "--state",
        choices=VALID_STATES,
        default=None,
        help="Filtra por estado (omitir = todos)",
    )
    list_p.set_defaults(func=_cmd_list)

    # roadmap show <filename>
    show_p = sub.add_parser("show", help="Exibe o conteudo de um roadmap")
    show_p.add_argument("filename", help="Nome (ou parte do nome) do arquivo")
    show_p.set_defaults(func=_cmd_show)

    def _roadmap_default(args):
        roadmap_parser.print_help()

    roadmap_parser.set_defaults(func=_roadmap_default)
