"""
commands/release.py — Subcomando `trackfw release` (e `release tag`).

Registra o comando 'release' e o subcomando 'tag' no argparse, delegando para
trackfw.release.runner.run_release_tag (testavel por injecao de dependencia).
"""

import sys

_TAG_DESCRIPTION = (
    "Create and publish an annotated release tag.\n\n"
    "It exists because 'trackfw ship' only pushes branches — tag is not a branch operation, "
    "and ship's governance gate (\"REQ + roadmap in wip/\") does not apply to release. See "
    "ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.\n\n"
    "Every precondition below refuses with a message naming what to fix — this command never "
    "guesses:\n"
    "  1. Working tree must be clean.\n"
    "  2. The default branch (main/master), if checked out locally, must be up to date with origin.\n"
    "  3. The 4 version files must all match <version> exactly.\n"
    "  4. CHANGELOG.md must have a \"## [<version>] - YYYY-MM-DD\" section.\n"
    "  5. The tag must not already exist, locally or on origin.\n"
    "  6. The GitHub CLI (gh) must be available and authenticated — release tag currently only "
    "supports GitHub; other forges are refused with instructions to push the tag manually.\n\n"
    "On success, it publishes the tag via two GitHub API calls (POST git/tags then POST "
    "git/refs), preserving the annotation."
)


def register(subparsers):
    """Registra o comando 'release' e seus subcomandos no argparse."""
    release_parser = subparsers.add_parser(
        "release",
        help="Governed release operations",
    )
    sub = release_parser.add_subparsers(dest="release_cmd", metavar="SUBCOMMAND")

    tag_p = sub.add_parser(
        "tag",
        help="Create and publish an annotated release tag",
        description=_TAG_DESCRIPTION,
    )
    tag_p.add_argument("version", help='Version to tag (with or without a leading "v")')
    tag_p.set_defaults(func=_dispatch_tag)

    def _release_default(args):
        release_parser.print_help()

    release_parser.set_defaults(func=_release_default)


def _dispatch_tag(args):
    from trackfw.release.runner import run_release_tag
    from trackfw.config import load as load_config

    cfg = load_config()
    exit_code = run_release_tag(
        args.version,
        config_forge=cfg.get("forge", ""),
        repo_dir=".",
    )
    if exit_code != 0:
        sys.exit(exit_code)
