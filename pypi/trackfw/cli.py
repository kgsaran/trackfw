"""
cli.py — Entry point principal do trackfw Python CLI.
Usa argparse (stdlib) e delega para subcomandos em trackfw/commands/.
"""

import argparse
import sys
from trackfw import __version__


def main():
    parser = argparse.ArgumentParser(
        prog="trackfw",
        description="trackfw — governed software delivery framework\nADR → REQ → ROADMAP → kanban",
    )
    parser.add_argument(
        "--version",
        action="version",
        version=f"trackfw {__version__}",
    )

    subparsers = parser.add_subparsers(dest="command", metavar="COMMAND")

    version_parser = subparsers.add_parser("version", help="Print version")
    version_parser.set_defaults(func=lambda _args: print(f"trackfw {__version__}"))

    # --- init ---
    from trackfw.commands import init as init_cmd
    init_cmd.register(subparsers)

    # --- adr ---
    from trackfw.commands import adr as adr_cmd
    adr_cmd.register(subparsers)

    # --- req ---
    from trackfw.commands import req as req_cmd
    req_cmd.register(subparsers)

    # --- roadmap ---
    from trackfw.commands import roadmap as roadmap_cmd
    roadmap_cmd.register(subparsers)

    # --- validate ---
    from trackfw.commands import validate as validate_cmd
    validate_cmd.register(subparsers)

    # --- status ---
    from trackfw.commands import status as status_cmd
    status_cmd.register(subparsers)

    # --- log ---
    from trackfw.commands import log as log_cmd
    log_cmd.register(subparsers)

    # --- baseline ---
    from trackfw.commands import baseline as baseline_cmd
    baseline_cmd.register(subparsers)

    # --- help ---
    from trackfw.commands import help_cmd
    help_cmd.register(subparsers)

    # --- configure ---
    from trackfw.commands import configure as configure_cmd
    configure_cmd.register(subparsers)

    # --- discover ---
    from trackfw.commands import discover as discover_cmd
    discover_cmd.register(subparsers)

    # --- update ---
    from trackfw.commands import update as update_cmd
    update_cmd.register(subparsers)

    # --- metrics ---
    from trackfw.commands import metrics as metrics_cmd
    metrics_cmd.register(subparsers)

    # --- context ---
    from trackfw.commands import context as context_cmd
    context_cmd.register(subparsers)

    # --- sync ---
    from trackfw.commands import sync as sync_cmd
    sync_cmd.register(subparsers)

    # --- plugins ---
    from trackfw.commands import plugins as plugins_cmd
    plugins_cmd.register(subparsers)

    # --- serve ---
    from trackfw.commands import serve as serve_cmd
    serve_cmd.register(subparsers)

    # --- agents / skills ---
    from trackfw.commands import agents as agents_cmd
    from trackfw.commands import skills as skills_cmd
    agents_cmd.register(subparsers)
    skills_cmd.register(subparsers)

    args = parser.parse_args()

    if args.command is None:
        parser.print_help()
        sys.exit(0)

    if hasattr(args, "func"):
        args.func(args)
    else:
        parser.print_help()
        sys.exit(0)


if __name__ == "__main__":
    main()
