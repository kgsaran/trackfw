"""
commands/ship.py — Subcomando `trackfw ship`.

Registra o comando `ship` no parser principal e delega para
trackfw.ship.runner.run_ship (testável por injeção de dependência).
"""

import sys


def register(subparsers):
    """Adiciona o subcomando `ship` ao parser principal."""
    ship_parser = subparsers.add_parser(
        "ship",
        help="Governed git commit + push for feat/fix/refactor branches",
        description=(
            "trackfw ship runs a governed delivery sequence:\n\n"
            "  1. Validates branch name — must match feat|fix|refactor/<slug>\n"
            "  2. Validates governance — REQ + roadmap in wip/ must exist\n"
            "  3. Detects pending squash-merges in other branches (advisory only)\n"
            "  4. Reviews what is staged (git status --short + git diff --cached --stat)\n"
            "  5. Commits with Conventional Commits format (-m is required)\n"
            "  6. Pushes to origin (adds -u if no upstream is configured yet)\n\n"
            "Stage your files explicitly before running ship.\n"
            "This command never executes 'git add .' or 'git add -A'."
        ),
    )
    ship_parser.add_argument(
        "-m",
        "--message",
        default="",
        help="Commit message (Conventional Commits format required)",
    )
    ship_parser.add_argument(
        "--dry-run",
        action="store_true",
        default=False,
        help="Print what would be done without executing write commands",
    )
    ship_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    from trackfw.ship.runner import run_ship

    exit_code = run_ship(
        message=args.message,
        dry_run=args.dry_run,
    )
    if exit_code != 0:
        sys.exit(exit_code)
