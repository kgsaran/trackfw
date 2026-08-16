"""
commands/branch.py — Subcomando `trackfw branch new <type>/<slug>`.

Espelha o comportamento de internal/commands/branch.go — Go é a referência
comportamental (docs/cli-parity.md: "Go is the behavioral reference"). Move o
gate de governança branch_has_wip_roadmap (já aplicado por `trackfw validate`
e `trackfw ship`) para antes da criação da branch:

  1. Valida <type> in {feat, fix, refactor} e <slug> não-vazio.
  2. Verifica se algum roadmap em wip/ ou done/ casa com o slug — a mesma
     lógica de matching que `trackfw validate` já usa
     (validator.branch_slug_matches_roadmap).
  3. Sem match: bloqueia — nunca executa `git checkout -b` — e imprime a
     mesma mensagem de orientação que `trackfw validate` já imprime para essa
     regra (validator.branch_governance_orientation /
     branch_no_matching_roadmap_message).
  4. Com match: executa `git checkout -b <type>/<slug>`, propagando a saída e
     o exit code do próprio Git literalmente (nunca reformatar a saída do
     Git).

run_branch_new é testável por injeção de dependência (mesmo padrão de
trackfw.ship.runner.run_ship) — nenhum teste unitário toca um repositório git
real.
"""

from __future__ import annotations

import subprocess
import sys

from .. import config as _config
from .. import validator as _validator


# BRANCH_VALID_TYPES é o vocabulário completo aceito por `trackfw branch new`. feat/fix/refactor
# são gated numa REQ + roadmap correspondente já em wip/ ou done/ (BRANCH_GATED_TYPES abaixo);
# chore/docs são tipos de housekeeping — já tratados como isentos de roadmap por `trackfw ship` e
# `trackfw commit` — e criam a branch sem esse gate.
BRANCH_VALID_TYPES = {"feat", "fix", "refactor", "chore", "docs"}

# BRANCH_GATED_TYPES é o subconjunto de BRANCH_VALID_TYPES que exige uma REQ + roadmap
# correspondente já em wip/ ou done/ antes de criar a branch. Manter sincronizado com o padrão
# que `trackfw ship`/`trackfw commit` usam para decidir quando o gate branch_has_wip_roadmap se
# aplica.
BRANCH_GATED_TYPES = {"feat", "fix", "refactor"}


# ────────────────────────────────────────────────────────────────────────────
# Registro do subcomando
# ────────────────────────────────────────────────────────────────────────────

def register(subparsers):
    """Registra o comando 'branch' e seu subcomando 'new' no argparse."""
    branch_parser = subparsers.add_parser(
        "branch",
        help="Manage governed feature branches",
    )
    sub = branch_parser.add_subparsers(dest="branch_cmd", metavar="SUBCOMMAND")

    new_p = sub.add_parser(
        "new",
        help=(
            "Create a feat/fix/refactor/chore/docs branch; feat/fix/refactor gated on a "
            "matching REQ + roadmap already in wip/ or done/"
        ),
        description=(
            "trackfw branch new moves the branch_has_wip_roadmap governance gate (already "
            "enforced by 'trackfw validate' and 'trackfw ship') to before branch creation, "
            "instead of after:\n\n"
            "  1. Validates <type> is one of feat, fix, refactor, chore, docs and <slug> is "
            "non-empty.\n"
            "  2. For feat, fix, refactor: checks whether a roadmap in wip/ or done/ matches the "
            "given slug — the exact matching logic 'trackfw validate' already uses (normalized "
            "slug, filename contains match). Without a match: blocks — 'git checkout -b' is "
            "never executed — and prints the same governance orientation message "
            "'trackfw validate' already prints for this rule.\n"
            "  3. For chore, docs: housekeeping types already treated as roadmap-exempt by "
            "'trackfw ship' and 'trackfw commit' — the branch is created without the roadmap "
            "gate.\n"
            "  4. With a match (or for chore/docs): runs 'git checkout -b <type>/<slug>', "
            "propagating Git's own output and exit status literally.\n\n"
            "Create the governance artifacts first if this blocks you:\n"
            "  trackfw req new \"title\"\n"
            "  trackfw roadmap new \"title\"\n"
            "  trackfw roadmap move <name> wip"
        ),
    )
    new_p.add_argument(
        "spec",
        metavar="<type>/<slug>",
        help="Branch type and slug, e.g. feat/my-feature",
    )
    new_p.add_argument(
        "--dry-run",
        action="store_true",
        default=False,
        help="Report whether the branch would be created or blocked, without executing git",
    )
    new_p.set_defaults(func=_dispatch_new)

    def _branch_default(args):
        branch_parser.print_help()

    branch_parser.set_defaults(func=_branch_default)
    return branch_parser


# ────────────────────────────────────────────────────────────────────────────
# Parsing de "<type>/<slug>"
# ────────────────────────────────────────────────────────────────────────────

def parse_branch_spec(spec: str):
    """Splits "<type>/<slug>" and validates both parts.

    Returns (branch_type, slug, error) — error is None on success. type must be one of feat,
    fix, refactor, chore, docs (BRANCH_VALID_TYPES); slug must be non-empty. Espelha
    internal/commands/branch.go parseBranchSpec (mesmo comportamento: bloquear sem chamar git;
    a redação é adaptada ao estilo Python já usado neste CLI).
    """
    parts = spec.split("/", 1)
    if len(parts) != 2 or parts[0] == "":
        return None, None, (
            f'invalid branch spec "{spec}" — expected <type>/<slug> with type in feat, fix, refactor, chore, docs'
        )
    branch_type, slug = parts[0], parts[1]
    if branch_type not in BRANCH_VALID_TYPES:
        return None, None, (
            f'invalid branch type "{branch_type}" — must be one of feat, fix, refactor, chore, docs'
        )
    if slug.strip() == "":
        return None, None, f'branch slug is required — expected <type>/<slug>, got "{spec}"'
    return branch_type, slug, None


# ────────────────────────────────────────────────────────────────────────────
# git checkout -b (produção)
# ────────────────────────────────────────────────────────────────────────────

def _default_git_checkout(branch_name: str) -> int:
    """Runs `git checkout -b <branch_name>` with inherited stdio, so Git's own output
    (including branch-already-exists errors) reaches the user unmodified. Returns Git's exit
    code, propagated literally."""
    result = subprocess.run(["git", "checkout", "-b", branch_name])
    return result.returncode


# ────────────────────────────────────────────────────────────────────────────
# Núcleo testável por injeção de dependência
# ────────────────────────────────────────────────────────────────────────────

def run_branch_new(
    spec: str,
    dry_run: bool = False,
    load_config=None,
    resolve_wip_dirs=None,
    resolve_done_dirs=None,
    match_slug=None,
    exec_git_checkout=None,
    out=None,
    err_out=None,
) -> int:
    """Implements the `trackfw branch new <type>/<slug>` flow described in
    docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md.

    Returns the process exit code (0 success, non-zero blocked/error). Every dependency is
    injectable and defaults to the real implementation in production; tests inject fakes so no
    real git repository or project filesystem layout is touched — mirrors
    internal/commands/branch.go's branchNewDeps / trackfw.ship.runner.run_ship's DI style.
    """
    load_config = load_config or _config.load
    resolve_wip_dirs = resolve_wip_dirs or _validator.resolve_wip_dirs
    resolve_done_dirs = resolve_done_dirs or _validator.resolve_done_dirs
    match_slug = match_slug or _validator.branch_slug_matches_roadmap
    exec_git_checkout = exec_git_checkout or _default_git_checkout
    out = out or sys.stdout
    err_out = err_out or sys.stderr

    branch_type, slug, parse_err = parse_branch_spec(spec)
    if parse_err is not None:
        err_out.write(parse_err + "\n")
        return 1

    branch_name = f"{branch_type}/{slug}"

    # chore/docs são tipos de housekeeping — já tratados como isentos de roadmap por
    # `trackfw ship` e `trackfw commit` — então o gate branch_has_wip_roadmap abaixo não se
    # aplica a eles.
    if branch_type in BRANCH_GATED_TYPES:
        cfg = load_config()
        wip_dirs = resolve_wip_dirs(cfg)
        done_dirs = resolve_done_dirs(cfg)

        normalized_slug = _validator.normalize_branch_slug(slug)
        matched, candidates = match_slug(normalized_slug, wip_dirs, done_dirs)

        if not matched:
            if not candidates:
                msg = _validator.branch_governance_orientation(branch_name)
            else:
                msg = _validator.branch_no_matching_roadmap_message(branch_name, candidates)
            if dry_run:
                out.write(f"[dry-run] would block: {msg}\n")
            else:
                out.write(msg + "\n")
            err_out.write(f'blocked: no matching roadmap in wip/ nor done/ for "{branch_name}"\n')
            return 1

    if dry_run:
        out.write(f'[dry-run] would create branch "{branch_name}" (git checkout -b {branch_name})\n')
        return 0

    return exec_git_checkout(branch_name)


def _dispatch_new(args):
    exit_code = run_branch_new(args.spec, dry_run=args.dry_run)
    sys.exit(exit_code)
