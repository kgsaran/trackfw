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
        help="Governed git commit + push for feat/fix/refactor/chore/docs branches",
        description=(
            "trackfw ship runs a governed delivery sequence:\n\n"
            "  1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>\n"
            "  2. Validates governance — REQ + roadmap in wip/ must exist for feat/fix/refactor branches\n"
            "     (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip\n"
            "     this check, mirroring 'trackfw commit'\n"
            "  3. Detects pending squash-merges in other branches (advisory only)\n"
            "  4. Reviews what is staged (git status --short + git diff --cached --stat)\n"
            "  5. Commits with Conventional Commits format (-m is required, unless nothing is staged\n"
            "     and --force-with-lease is set — see below)\n"
            "  6. Pushes to origin (adds -u if no upstream is configured yet)\n"
            "  7. Opens PR/MR via the resolved forge CLI (or prints URL if CLI is absent)\n\n"
            "Stage your files explicitly before running ship.\n"
            "This command never executes 'git add .' or 'git add -A'.\n\n"
            "--force-with-lease pushes with 'git push --force-with-lease' instead of a plain\n"
            "push — for the post-rebase case, where a plain push is rejected. It only runs when\n"
            "the branch already has an open PR/MR (verified via the resolved forge CLI): the\n"
            "safe path is always to open the PR first. When nothing is staged, it pushes\n"
            "existing commits without requiring -m."
        ),
        # allow_abbrev=False: without it, argparse accepts any unambiguous prefix of a long
        # flag — with --force-with-lease as the only --f... flag on this subparser, plain
        # "--force" would silently resolve to --force-with-lease. cobra/pflag (Go) and
        # commander (Node) never abbreviate, so this keeps the 3 CLIs aligned: "--force"
        # (raw) must not work anywhere. See
        # ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.
        allow_abbrev=False,
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
    ship_parser.add_argument(
        "--no-pr",
        action="store_true",
        default=False,
        help="Skip PR/MR creation after push",
    )
    ship_parser.add_argument(
        "--forge",
        default="",
        metavar="FORGE",
        help="Override forge detection (github, gitlab, bitbucket, azure)",
    )
    ship_parser.add_argument(
        "--force-with-lease",
        action="store_true",
        default=False,
        help="Governed force-push (git push --force-with-lease); requires an open PR/MR on this branch",
    )
    ship_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    from trackfw.ship.runner import run_ship
    from trackfw.config import load as load_config

    cfg = load_config()
    exit_code = run_ship(
        message=args.message,
        dry_run=args.dry_run,
        no_pr=args.no_pr,
        forge_flag=args.forge,
        config_forge=cfg.get("forge", ""),
        repo_dir=".",
        force_with_lease=args.force_with_lease,
    )
    if exit_code != 0:
        sys.exit(exit_code)
