"""
cli.py — Entry point principal do trackfw Python CLI.
Usa argparse (stdlib) e delega para subcomandos em trackfw/commands/.
"""

import argparse
import re
import sys
from trackfw import __version__
from trackfw.fatal_error import report_fatal_error
from trackfw.unknown_command import format_unknown_command_error

# Matches argparse's own message for an unrecognized COMMAND positional
# (add_subparsers(dest="command", metavar="COMMAND")):
#   argument COMMAND: invalid choice: 'x' (choose from 'a', 'b', ...)
_UNKNOWN_COMMAND_RE = re.compile(r"^argument COMMAND: invalid choice: '(.*)' \(choose from")


class TrackfwArgumentParser(argparse.ArgumentParser):
    """Overrides error() ONLY for the top-level "unknown command" case, to emit
    the canonical cross-CLI message (unknown_command.py) with exit code 1
    instead of argparse's default "invalid choice" text and exit code 2.

    Deliberately narrow: every other error on this parser (missing/invalid
    top-level flags, "unrecognized arguments: ...", etc.) falls straight
    through to the stdlib ArgumentParser.error() unchanged, so this cannot
    alter the exit code of unrelated errors — required by
    ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-
    de-terceiro.md D3. Only used for the root parser; subparsers (one
    ArgumentParser instance per subcommand) are unaffected, so an invalid
    argument inside e.g. `trackfw roadmap move` keeps its own default exit
    code and message.
    """

    def error(self, message):
        match = _UNKNOWN_COMMAND_RE.match(message)
        if match is None:
            super().error(message)
            return
        typed = match.group(1)
        candidates = getattr(self, "trackfw_command_names", [])
        formatted = format_unknown_command_error(typed, candidates, self.prog)
        sys.stderr.write(formatted + "\n")
        sys.exit(1)


def _force_utf8_output():
    """Reconfigura stdout e stderr para UTF-8 antes de qualquer escrita.

    Console Windows entrega cp1252, que nao representa nada do vocabulario visual
    da ferramenta — setas, marcas, caixas, acentos. Sem isto, `trackfw --help`,
    `status` e `validate` morrem com UnicodeEncodeError num Windows padrao, antes
    de imprimir a primeira linha. A `description=` do parser raiz sozinha ja
    basta: ela contem →.

    UTF-8 e nao a codificacao do console porque e o que os outros dois runtimes
    fazem — Go e Node.js escrevem bytes UTF-8 direto, sem consultar codepage. Sem
    isto o Python e o unico dos tres que depende de `PYTHONUTF8=1` na frente para
    rodar, o que nao e paridade.

    O `newline` explicito desliga a traducao de quebra de linha: sem ele o Python
    emite CRLF no Windows enquanto os outros dois emitem LF, e as tres saidas
    ficam diferentes byte a byte para o mesmo comando.

    `errors="replace"` degrada em vez de abortar. Silencioso quando o stream nao
    suporta `reconfigure`: testes e pipelines substituem sys.stdout por objetos
    que nao tem o metodo.
    """
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure is None:
            continue
        try:
            reconfigure(encoding="utf-8", errors="replace", newline="\n")
        except (ValueError, OSError, TypeError):
            pass


def main():
    _force_utf8_output()

    parser = TrackfwArgumentParser(
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

    # --- serve ---
    from trackfw.commands import serve as serve_cmd
    serve_cmd.register(subparsers)

    # --- note ---
    from trackfw.commands import note as note_cmd
    note_cmd.register(subparsers)

    # --- ship ---
    from trackfw.commands import ship as ship_cmd
    ship_cmd.register(subparsers)

    # --- release ---
    from trackfw.commands import release as release_cmd
    release_cmd.register(subparsers)

    # --- agents / skills ---
    from trackfw.commands import agents as agents_cmd
    from trackfw.commands import skills as skills_cmd
    agents_cmd.register(subparsers)
    skills_cmd.register(subparsers)

    # --- barrier ---
    from trackfw.commands import barrier as barrier_cmd
    barrier_cmd.register(subparsers)

    # --- branch ---
    from trackfw.commands import branch as branch_cmd
    branch_cmd.register(subparsers)

    # --- commit ---
    from trackfw.commands import commit as commit_cmd
    commit_cmd.register(subparsers)

    # --- push ---
    from trackfw.commands import push as push_cmd
    push_cmd.register(subparsers)

    # --- changelog ---
    from trackfw.commands import changelog as changelog_cmd
    changelog_cmd.register(subparsers)

    # --- doctor ---
    from trackfw.commands import doctor as doctor_cmd
    doctor_cmd.register(subparsers)

    # --- audit-surface ---
    from trackfw.commands import audit_surface as audit_surface_cmd
    audit_surface_cmd.register(subparsers)

    # Snapshot the final command set for the "unknown command" error path
    # above — must run after every .register(subparsers) call, before parsing.
    parser.trackfw_command_names = list(subparsers.choices.keys())

    args = parser.parse_args()

    if args.command is None:
        parser.print_help()
        sys.exit(0)

    if hasattr(args, "func"):
        # Global backstop (defense in depth, see fatal_error.py): commands that
        # already catch their own domain errors (e.g. agents/skills install —
        # IntegrationError/OSError/ValueError in integrations/command.py:run())
        # print their own clean message and raise SystemExit, which is a
        # BaseException and therefore NOT caught here — it propagates
        # unchanged. This except only catches what nothing else caught.
        try:
            args.func(args)
        except Exception as error:  # noqa: BLE001 — intentional catch-all backstop
            report_fatal_error(error, command=args.command)
            sys.exit(1)
    else:
        parser.print_help()
        sys.exit(0)


if __name__ == "__main__":
    main()
