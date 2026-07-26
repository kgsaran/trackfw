"""
commands/note.py — Subcomando `trackfw note`.
Registra o grupo de subcomandos de vault notes no argparse principal.
"""

import sys


def register(subparsers):
    """Adiciona subcomando `note` com sub-subcomando `new` ao parser principal."""
    note_parser = subparsers.add_parser(
        "note",
        help="Manage vault knowledge notes",
    )
    note_sub = note_parser.add_subparsers(dest="note_command", metavar="COMMAND")

    # note new <title>
    new_parser = note_sub.add_parser(
        "new",
        help="Create a new knowledge note in vault/notes/",
    )
    new_parser.add_argument(
        "title",
        help="Note title",
    )

    note_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    """Despacha para o sub-subcomando correto."""
    if args.note_command == "new":
        _cmd_new(args)
    else:
        print("Usage: trackfw note <command>")
        print("Commands: new")
        sys.exit(0)


def _cmd_new(args):
    from trackfw.generators.note import new_note

    try:
        new_note(args.title)
    except FileExistsError as e:
        print(f"error: {e}", file=sys.stderr)
        sys.exit(1)
