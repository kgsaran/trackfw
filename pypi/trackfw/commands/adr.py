"""
commands/adr.py — Subcomando `trackfw adr`.
Registra o grupo de subcomandos ADR no argparse principal.
"""

import os
import sys


def register(subparsers):
    """Adiciona subcomando `adr` com sub-subcomandos `new`/`list` ao parser principal."""
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
        default="Proposed",
        choices=["Draft", "Proposed", "Accepted", "Deprecated", "Superseded"],
        help="Initial ADR status (default: Proposed)",
    )
    new_parser.add_argument(
        "--dir",
        default=None,
        metavar="PATH",
        help="Target directory (overrides trackfw.yaml adr_dirs)",
    )
    new_parser.add_argument(
        "--scope",
        choices=["project", "global"],
        default="project",
        help="ADR scope: project (docs/adr, default) or global (~/.trackfw/adr, cross-project)",
    )

    # adr list
    list_parser = adr_sub.add_parser(
        "list",
        help="List all ADRs",
    )
    list_parser.add_argument(
        "--scope",
        choices=["project", "global"],
        default="project",
        help="ADR scope: project (docs/adr, default) or global (~/.trackfw/adr, cross-project)",
    )

    adr_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    """Despacha para o sub-subcomando correto."""
    if args.adr_command == "new":
        _cmd_new(args)
    elif args.adr_command == "list":
        _cmd_list(args)
    else:
        print("Usage: trackfw adr <command>")
        print("Commands: new, list")
        sys.exit(0)


def _resolve_adr_dirs(args):
    """
    Resolve a lista de diretórios de ADR conforme --scope.
    'project' (default) preserva o comportamento atual: --dir (se passado)
    ou adr_dirs do trackfw.yaml. 'global' escreve/lista em ~/.trackfw/adr,
    sem exigir trackfw.yaml/raiz de projeto no cwd — mesmo padrão de Go
    (resolveADRDir) e Node.js (resolveAdrDir).

    --scope global e --dir são mutuamente exclusivos: passar os dois é erro.
    """
    if args.scope == "global":
        if getattr(args, "dir", None):
            print(
                'trackfw: --scope global e --dir são mutuamente exclusivos. '
                'Use --scope global (escreve em ~/.trackfw/adr) ou --dir '
                '<path> (escopo project, ignora trackfw.yaml), não os dois.',
                file=sys.stderr,
            )
            sys.exit(1)
        from trackfw.generators.adr import global_adr_dir

        home = os.path.expanduser("~")
        return [global_adr_dir(home)]

    from trackfw.config import load as load_config

    cfg = load_config()
    if getattr(args, "dir", None):
        return [args.dir]
    return cfg.get("adr_dirs", ["docs/adr"])


def _cmd_new(args):
    from trackfw.generators.adr import generate_adr

    adr_dirs = _resolve_adr_dirs(args)

    filepath = generate_adr(
        title=args.title,
        status=args.status,
        adr_dirs=adr_dirs,
    )
    print(f"created {filepath}")


def _cmd_list(args):
    from trackfw.generators.adr import list_adrs

    adr_dirs = _resolve_adr_dirs(args)
    list_adrs(adr_dirs[0])
