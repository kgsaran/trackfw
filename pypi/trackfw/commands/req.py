"""
commands/req.py — Subcomando `trackfw req`.
Registra o grupo de subcomandos REQ no argparse principal.
"""

import sys


def register(subparsers):
    """Adiciona subcomando `req` com sub-subcomando `new` ao parser principal."""
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

    req_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    """Despacha para o sub-subcomando correto."""
    if args.req_command == "new":
        _cmd_new(args)
    else:
        print("Usage: trackfw req <command>")
        print("Commands: new")
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
    req_dir = cfg.get("req_dir", "docs/req")

    filepath = generate_req(title=title, req_dir=req_dir)
    print(f"created {filepath}")
