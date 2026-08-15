"""commands/changelog.py — Subcomando `trackfw changelog`."""

import os
import sys

from .. import changelog as _changelog


def register(subparsers):
    """Adiciona subcomando `changelog` ao parser principal."""
    parser = subparsers.add_parser(
        "changelog",
        help="Show entries from CHANGELOG.md",
    )
    parser.add_argument("--version", dest="version", default="", help="Show a specific version section")
    parser.add_argument("--all", dest="all", action="store_true", help="Show the entire CHANGELOG.md")
    parser.set_defaults(func=run)
    return parser


def run(args):
    """Executa o comando changelog e imprime o resultado."""
    root = os.getcwd()
    try:
        content = _changelog.read(root)
        if getattr(args, "all", False):
            print(content, end="")
            return 0

        sections = _changelog.parse_sections(content)
        version = getattr(args, "version", "") or ""
        if version:
            section = _changelog.find_version(sections, version)
        else:
            section = _changelog.first_section(sections)

        print(_changelog.format_section(section), end="")
        return 0
    except (OSError, ValueError) as exc:
        print(f"Error: {exc}", file=sys.stderr)
        sys.exit(1)
