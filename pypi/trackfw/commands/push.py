"""
commands/push.py — Subcomando `trackfw push`.

Registra o comando `push` no parser principal e delega para
trackfw.push.runner.run_push (testável por injeção de dependência).
"""

import sys


def register(subparsers):
    """Adiciona o subcomando `push` ao parser principal."""
    push_parser = subparsers.add_parser(
        "push",
        help="Governed git push for commits already created",
        description=(
            "trackfw push pushes already-created commits without committing and without opening a PR/MR.\n\n"
            "  1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>\n"
            "  2. Validates governance — REQ + roadmap in wip/ must exist for feat/fix/refactor branches\n"
            "     (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip\n"
            "     this check, mirroring 'trackfw commit'\n"
            "  3. Detects pending squash-merges in other branches (advisory only)\n"
            "  4. Pushes to origin (adds -u if no upstream is configured yet)\n\n"
            "push never commits and never opens a PR/MR.\n"
            "Does not accept -m. If you have not committed yet, run 'trackfw commit -m \"...\"' first.\n\n"
            "Compositional vocabulary:\n"
            "  trackfw commit -m \"...\"   commits\n"
            "  trackfw push              pushes\n"
            "  trackfw ship -m \"...\"     commit + push + PR (composition)\n\n"
            "--force-with-lease pushes with 'git push --force-with-lease' instead of a plain push — for\n"
            "the post-rebase case, where a plain push is rejected. It only runs when the branch already\n"
            "has an open PR/MR (verified via the resolved forge CLI): the safe path is always to open the\n"
            "PR first."
        ),
        # allow_abbrev=False: without it, argparse accepts any unambiguous prefix of a long
        # flag. cobra/pflag (Go) and commander (Node) never abbreviate, so this keeps the
        # 3 CLIs aligned. See
        # ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.
        allow_abbrev=False,
    )
    push_parser.add_argument(
        "--dry-run",
        action="store_true",
        default=False,
        help="Print what would be done without executing write commands",
    )
    push_parser.add_argument(
        "--force-with-lease",
        action="store_true",
        default=False,
        help="Governed force-push (git push --force-with-lease); requires an open PR/MR on this branch",
    )
    push_parser.set_defaults(func=_dispatch)


def _dispatch(args):
    from trackfw.push.runner import run_push
    from trackfw.config import load as load_config

    cfg = load_config()
    exit_code = run_push(
        dry_run=args.dry_run,
        force_with_lease=args.force_with_lease,
        config_forge=cfg.get("forge", ""),
        repo_dir=".",
    )
    if exit_code != 0:
        sys.exit(exit_code)
