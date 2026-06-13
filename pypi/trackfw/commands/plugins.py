"""
commands/plugins.py — Subcomando `trackfw plugins`.
Lista plugins instalados (trackfw-* no PATH) e executa via subprocess.
Espelho Python de npm/src/commands/plugins.js (subset: list e run).
"""

import os
import shutil
import subprocess
import sys


def _find_plugins_in_path():
    """
    Busca executáveis com prefixo `trackfw-` no PATH.
    Retorna lista de nomes (sem o prefixo).
    """
    found = []
    path_dirs = os.environ.get("PATH", "").split(os.pathsep)
    seen = set()
    for directory in path_dirs:
        if not directory:
            continue
        try:
            entries = os.listdir(directory)
        except OSError:
            continue
        for entry in entries:
            if entry.startswith("trackfw-") and entry not in seen:
                full = os.path.join(directory, entry)
                if os.path.isfile(full) and os.access(full, os.X_OK):
                    seen.add(entry)
                    found.append(entry)
    return sorted(found)


def register(subparsers):
    """Adiciona subcomando `plugins` com sub-subcomandos ao parser principal."""
    plugins_parser = subparsers.add_parser(
        "plugins",
        help="Manage trackfw plugins",
    )
    plugins_sub = plugins_parser.add_subparsers(dest="plugins_command", metavar="COMMAND")

    # plugins list
    plugins_sub.add_parser(
        "list",
        help="List installed plugins (trackfw-* executables in PATH)",
    )

    # plugins run <name> [args...]
    run_parser = plugins_sub.add_parser(
        "run",
        help="Run an installed plugin by name",
    )
    run_parser.add_argument("name", help="Plugin name (without trackfw- prefix)")
    run_parser.add_argument(
        "plugin_args",
        nargs="*",
        metavar="ARGS",
        help="Arguments to pass to the plugin",
    )

    plugins_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    """Despacha para o sub-subcomando correto."""
    if args.plugins_command == "list":
        _cmd_list(args)
    elif args.plugins_command == "run":
        _cmd_run(args)
    else:
        print("Usage: trackfw plugins <command>")
        print("Commands: list, run")
        sys.exit(0)


def _cmd_list(args):
    """Lista plugins instalados como executáveis trackfw-* no PATH."""
    plugins = _find_plugins_in_path()
    if not plugins:
        print("No plugins installed")
        return
    for p in plugins:
        # exibe o nome completo e também o nome curto (sem prefixo)
        short = p[len("trackfw-"):]
        print(f"{p}  (trackfw plugins run {short})")


def _cmd_run(args):
    """Executa trackfw-<name> repassando args via subprocess."""
    executable = f"trackfw-{args.name}"
    path = shutil.which(executable)
    if path is None:
        print(
            f'Plugin "{executable}" not found in PATH. '
            f'Install it or check `trackfw plugins list`.',
            file=sys.stderr,
        )
        sys.exit(1)

    cmd = [path] + list(getattr(args, "plugin_args", []))
    try:
        result = subprocess.run(cmd, check=False)
        sys.exit(result.returncode)
    except OSError as e:
        print(f"Failed to run plugin: {e}", file=sys.stderr)
        sys.exit(1)
