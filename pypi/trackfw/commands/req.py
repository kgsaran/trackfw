"""
commands/req.py — Subcomando `trackfw req`.
Registra o grupo de subcomandos REQ no argparse principal.
"""

import sys


def register(subparsers):
    """Adiciona subcomando `req` ao parser principal."""
    req_parser = subparsers.add_parser(
        "req",
        help="Manage Requirements",
    )
    req_sub = req_parser.add_subparsers(dest="req_command", metavar="COMMAND")

    # req new [<title>]
    new_parser = req_sub.add_parser(
        "new",
        help="Create a new REQ",
    )
    new_parser.add_argument(
        "title",
        nargs="?",
        default=None,
        help="REQ title (prompted if omitted)",
    )

    move_parser = req_sub.add_parser(
        "move",
        help="Update a REQ status in place",
    )
    move_parser.add_argument("name", help="REQ filename fragment")
    move_parser.add_argument("status", help="New status")

    req_sub.add_parser(
        "list",
        help="List REQs",
    )

    req_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    """Despacha para o sub-subcomando correto."""
    if args.req_command == "new":
        _cmd_new(args)
    elif args.req_command == "move":
        _cmd_move(args)
    elif args.req_command == "list":
        _cmd_list(args)
    else:
        print("Usage: trackfw req <command>")
        print("Commands: new, move, list")
        sys.exit(0)


def _cmd_new(args):
    from trackfw.config import load as load_config
    from trackfw.generators.req import generate_req

    title = args.title
    if not title:
        try:
            title = input("REQ title: ").strip()
        except (EOFError, KeyboardInterrupt):
            print("")
            sys.exit(0)

    if not title:
        print("Error: title is required", file=sys.stderr)
        sys.exit(1)

    cfg = load_config()
    # Ponto único de decisão de caminho de ESCRITA (ADR-2026-09-03, D2/D4): by_agent grava no
    # canônico req_dir/<agente>/; flat grava em req_dir/ — o mesmo ponto que alimenta a união de
    # leitura de resolve_req_files.
    from trackfw.validator import req_write_dir  # noqa: PLC0415

    req_dir = req_write_dir(cfg) or cfg.get("req_dir", "docs/req")

    filepath = generate_req(title=title, req_dir=req_dir)
    print(f"created {filepath}")


def _cmd_move(args):
    from trackfw.config import load as load_config
    from trackfw.generators.req import move_req

    cfg = load_config()
    try:
        filepath = move_req(args.name, args.status, cfg=cfg)
    except RuntimeError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        sys.exit(1)
    print(f"updated {filepath} status -> {args.status}")


def _cmd_list(args):
    from trackfw.config import load as load_config
    from trackfw.generators.req import list_reqs

    cfg = load_config()
    list_reqs(cfg)
