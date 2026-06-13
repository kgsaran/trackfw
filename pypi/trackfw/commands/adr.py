"""
commands/adr.py — Subcomando `trackfw adr`.
Registra o grupo de subcomandos ADR no argparse principal.
"""

import sys


def register(subparsers):
    """Adiciona subcomando `adr` com sub-subcomando `new` ao parser principal."""
    adr_parser = subparsers.add_parser(
        "adr",
        help="Manage Architecture Decision Records",
    )
    adr_sub = adr_parser.add_subparsers(dest="adr_command", metavar="COMMAND")

    # adr new <title>
    new_parser = adr_sub.add_parser(
        "new",
        help="Create a new ADR",
    )
    new_parser.add_argument("title", help="ADR title")
    new_parser.add_argument(
        "--status",
        default="Draft",
        choices=["Draft", "Proposed", "Accepted", "Deprecated", "Superseded"],
        help="Initial ADR status (default: Draft)",
    )
    new_parser.add_argument(
        "--dir",
        default=None,
        metavar="PATH",
        help="Target directory (overrides trackfw.yaml adr_dirs)",
    )

    adr_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    """Despacha para o sub-subcomando correto."""
    if args.adr_command == "new":
        _cmd_new(args)
    else:
        print("Usage: trackfw adr <command>")
        print("Commands: new")
        sys.exit(0)


def _cmd_new(args):
    from trackfw.config import load as load_config
    from trackfw.generators.adr import generate_adr

    cfg = load_config()

    if args.dir:
        adr_dirs = [args.dir]
    else:
        adr_dirs = cfg.get("adr_dirs", ["docs/adr"])

    filepath = generate_adr(
        title=args.title,
        status=args.status,
        adr_dirs=adr_dirs,
    )
    print(f"created {filepath}")
