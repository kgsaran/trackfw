"""
cli.py — Entry point principal do trackfw Python CLI.
Usa argparse (stdlib) e delega para subcomandos em trackfw/commands/.
"""

import argparse
import sys

from trackfw import __version__


def _stub(name):
    """Retorna uma função handler que imprime 'Not implemented yet' para um stub."""
    def _handler(args):
        print("Not implemented yet")
        sys.exit(0)
    _handler.__name__ = name
    return _handler


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

    # --- init (stub) ---
    init_parser = subparsers.add_parser("init", help="Initialize trackfw in this project")
    init_parser.set_defaults(func=_stub("init"))

    # --- adr ---
    from trackfw.commands import adr as adr_cmd
    adr_cmd.register(subparsers)

    # --- req ---
    from trackfw.commands import req as req_cmd
    req_cmd.register(subparsers)

    # --- roadmap (stub) ---
    roadmap_parser = subparsers.add_parser("roadmap", help="Manage roadmaps")
    roadmap_parser.set_defaults(func=_stub("roadmap"))

    # --- validate (stub) ---
    validate_parser = subparsers.add_parser("validate", help="Validate governance artifacts")
    validate_parser.set_defaults(func=_stub("validate"))

    # --- status (stub) ---
    status_parser = subparsers.add_parser("status", help="Show governance status")
    status_parser.set_defaults(func=_stub("status"))

    # --- log ---
    from trackfw.commands import log as log_cmd
    log_cmd.register(subparsers)

    # --- baseline ---
    from trackfw.commands import baseline as baseline_cmd
    baseline_cmd.register(subparsers)

    # --- discover (stub) ---
    discover_parser = subparsers.add_parser("discover", help="Discover governance context")
    discover_parser.set_defaults(func=_stub("discover"))

    # --- metrics (stub) ---
    metrics_parser = subparsers.add_parser("metrics", help="Show delivery metrics")
    metrics_parser.set_defaults(func=_stub("metrics"))

    # --- context (stub) ---
    context_parser = subparsers.add_parser("context", help="Show AI agent context")
    context_parser.set_defaults(func=_stub("context"))

    # --- sync (stub) ---
    sync_parser = subparsers.add_parser("sync", help="Sync REQs to external trackers")
    sync_parser.set_defaults(func=_stub("sync"))

    # --- plugins (stub) ---
    plugins_parser = subparsers.add_parser("plugins", help="Manage trackfw plugins")
    plugins_parser.set_defaults(func=_stub("plugins"))

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
