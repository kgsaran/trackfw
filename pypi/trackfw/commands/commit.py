"""
commands/commit.py — Subcomando `trackfw commit -m "<mensagem>"`.

Espelha o comportamento de internal/commands/commit.go — Go é a referência
comportamental (docs/cli-parity.md: "Go is the behavioral reference"). É o
passo intermediário entre `git commit` cru e `trackfw ship`: comita as
mudanças staged diretamente, mas bloqueia o commit antes de acontecer quando
a governança está ausente, em vez de deixar acontecer e só detectar depois:

  1. Em 'main'/'master': sempre bloqueado — commit direto na branch padrão
     nunca é permitido.
  2. Em uma branch feat/fix/refactor: exige um roadmap correspondente ao
     slug da branch já em wip/ ou done/ — a mesma lógica de matching que
     `trackfw branch new` e `trackfw validate` já usam
     (validator.branch_slug_matches_roadmap). Sem match, bloqueia com a
     mesma mensagem de orientação de governança.
  3. Em qualquer outra branch (ex: branches de doc/housekeeping): permitido
     sem exigir roadmap — um aviso é logado, mas o commit prossegue.
  4. Quando permitido: executa `git commit -m <message>`, propagando a
     saída e o exit code do próprio Git literalmente.

run_commit é testável por injeção de dependência (mesmo padrão de
trackfw.commands.branch.run_branch_new) — nenhum teste unitário toca um
repositório git real.
"""

from __future__ import annotations

import subprocess
import sys

from .. import config as _config
from .. import validator as _validator

# Branches onde `trackfw commit` nunca é permitido, independente do estado de
# governança — espelha a mesma regra dura que `trackfw ship` já aplica.
COMMIT_PROTECTED_BRANCHES = {"main", "master"}

# Prefixos de tipo de branch que exigem um roadmap correspondente em wip/ ou
# done/ antes do commit — mesmo vocabulário de `trackfw branch new` e da
# regra de governança branch_has_wip_roadmap.
COMMIT_GOVERNED_PREFIXES = ("feat/", "fix/", "refactor/")


# ────────────────────────────────────────────────────────────────────────────
# Registro do subcomando
# ────────────────────────────────────────────────────────────────────────────

def register(subparsers):
    """Registra o comando 'commit' no argparse."""
    commit_parser = subparsers.add_parser(
        "commit",
        help="Commit staged changes, gated on governance for feat/fix/refactor branches",
        description=(
            "trackfw commit is the missing intermediate step between raw 'git commit' and "
            "'trackfw ship': it commits staged changes directly, but blocks the commit before "
            "it happens when governance is missing, instead of letting it land and only "
            "catching it later:\n\n"
            "  1. On 'main'/'master': always blocked — commit directly on the default branch "
            "is never permitted.\n"
            "  2. On a feat/fix/refactor branch: requires a roadmap matching the branch slug "
            "already in wip/ or done/ — the exact matching logic 'trackfw branch new' and "
            "'trackfw validate' already use. Without a match, blocks with the same governance "
            "orientation message.\n"
            "  3. On any other branch (e.g. doc/housekeeping branches): allowed without "
            "requiring a roadmap — a warning is logged, but the commit proceeds.\n"
            "  4. When allowed: runs 'git commit -m <message>', propagating Git's own output "
            "and exit status literally.\n\n"
            "Create the governance artifacts first if this blocks you:\n"
            "  trackfw req new \"title\"\n"
            "  trackfw roadmap new \"title\"\n"
            "  trackfw roadmap move <name> wip"
        ),
    )
    commit_parser.add_argument(
        "-m",
        "--message",
        default="",
        help="Commit message (required)",
    )
    commit_parser.set_defaults(func=_dispatch)
    return commit_parser


# ────────────────────────────────────────────────────────────────────────────
# git commit -m <message> (produção)
# ────────────────────────────────────────────────────────────────────────────

def _default_current_branch() -> tuple[str, str | None]:
    """Runs `git rev-parse --abbrev-ref HEAD` and returns (branch, error_or_None)."""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "HEAD"],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            return ("", result.stderr.strip() or "git rev-parse --abbrev-ref HEAD failed")
        return (result.stdout.strip(), None)
    except FileNotFoundError:
        return ("", "git not found in PATH")
    except Exception as e:  # noqa: BLE001
        return ("", str(e))


def _default_git_commit(message: str) -> int:
    """Runs `git commit -m <message>` with inherited stdio, so Git's own output reaches the
    user unmodified. Returns Git's exit code, propagated literally."""
    result = subprocess.run(["git", "commit", "-m", message])
    return result.returncode


# ────────────────────────────────────────────────────────────────────────────
# Núcleo testável por injeção de dependência
# ────────────────────────────────────────────────────────────────────────────

def commit_governed_branch_prefix(branch: str):
    """Returns (prefix, matched) — matched is True when branch starts with one of
    COMMIT_GOVERNED_PREFIXES. Espelha internal/commands/commit.go
    commitGovernedBranchPrefix."""
    for prefix in COMMIT_GOVERNED_PREFIXES:
        if branch.startswith(prefix):
            return prefix, True
    return "", False


def run_commit(
    message: str,
    load_config=None,
    current_branch=None,
    resolve_wip_dirs=None,
    resolve_done_dirs=None,
    match_slug=None,
    exec_git_commit=None,
    out=None,
) -> int:
    """Implements the `trackfw commit -m "<message>"` flow described in ML-2C of
    docs/roadmaps/wip/ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md.

    Returns the process exit code (0 success, non-zero blocked/error). Every dependency is
    injectable and defaults to the real implementation in production; tests inject fakes so no
    real git repository or project filesystem layout is touched — mirrors
    trackfw.commands.branch.run_branch_new's DI style.
    """
    load_config = load_config or _config.load
    current_branch = current_branch or _default_current_branch
    resolve_wip_dirs = resolve_wip_dirs or _validator.resolve_wip_dirs
    resolve_done_dirs = resolve_done_dirs or _validator.resolve_done_dirs
    match_slug = match_slug or _validator.branch_slug_matches_roadmap
    exec_git_commit = exec_git_commit or _default_git_commit
    out = out or sys.stdout

    branch, branch_err = current_branch()
    if branch_err is not None:
        sys.stderr.write(
            f"could not determine current branch (are you in a git repo?): {branch_err}\n"
        )
        return 1
    branch = branch.strip()

    # (a) main/master: sempre bloqueado.
    if branch in COMMIT_PROTECTED_BRANCHES:
        msg = (
            f'trackfw commit: commit direto em "{branch}" não é permitido. '
            "Use 'trackfw branch new <type>/<slug>' primeiro. Ver CLAUDE.md §1."
        )
        out.write(msg + "\n")
        sys.stderr.write(f'blocked: commit directly on "{branch}" is not permitted\n')
        return 1

    # (b) feat/fix/refactor: exige roadmap correspondente em wip/ ou done/.
    governed_prefix, is_governed = commit_governed_branch_prefix(branch)
    if is_governed:
        slug = branch[len(governed_prefix):]
        cfg = load_config()
        wip_dirs = resolve_wip_dirs(cfg)
        done_dirs = resolve_done_dirs(cfg)

        normalized_slug = _validator.normalize_branch_slug(slug)
        matched, candidates = match_slug(normalized_slug, wip_dirs, done_dirs)

        if not matched:
            if not candidates:
                msg = _validator.branch_governance_orientation(branch)
            else:
                msg = _validator.branch_no_matching_roadmap_message(branch, candidates)
            out.write(msg + "\n")
            sys.stderr.write(
                f'blocked: no matching roadmap in wip/ nor done/ for "{branch}"\n'
            )
            return 1
    else:
        # (c) branches fora do padrão feat/fix/refactor (ex: branches de
        # doc/housekeeping): permite sem exigir roadmap, mas avisa.
        out.write(
            f'trackfw commit: branch "{branch}" does not follow feat/fix/refactor — '
            "committing without a roadmap check.\n"
        )
        # Flush before exec_git_commit below, which inherits stdio for a real `git commit`
        # subprocess: without this, Python's buffered sys.stdout can interleave after git's own
        # unbuffered output when stdout is redirected to a file/pipe (not a TTY), reordering the
        # warning after git's diagnostic — a divergence from Go/Node, which write unbuffered.
        out.flush()

    # (d) passou em todas as checagens: comita.
    return exec_git_commit(message)


def _dispatch(args):
    if not args.message.strip():
        sys.stderr.write(
            'commit message is required — use -m:\n'
            '  trackfw commit -m "feat(<scope>): <description>"\n'
        )
        sys.exit(1)
    exit_code = run_commit(args.message)
    sys.exit(exit_code)
