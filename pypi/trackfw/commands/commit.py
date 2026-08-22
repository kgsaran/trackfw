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

# Diretórios (nos 3 CLIs suportados) onde um arquivo novo (status "A") sinaliza
# que um novo comando de CLI foi adicionado — usado pela regra heurística
# "feat" abaixo. Espelha commitCommandDirs em internal/commands/commit.go.
COMMIT_COMMAND_DIRS = (
    "internal/commands/",
    "npm/src/commands/",
    "pypi/trackfw/commands/",
)


# ────────────────────────────────────────────────────────────────────────────
# Registro do subcomando
# ────────────────────────────────────────────────────────────────────────────

def register(subparsers):
    """Registra o comando 'commit' no argparse."""
    commit_parser = subparsers.add_parser(
        "commit",
        help="Commit staged changes, gated on governance for feat/fix/refactor branches",
        description=(
            "trackfw commit commits staged changes directly, but blocks the commit before it "
            "happens when governance is missing, instead of letting it land and only "
            "catching it later.\n\n"
            "Compositional vocabulary:\n"
            "  trackfw commit -m \"...\"   commits\n"
            "  trackfw push              pushes\n"
            "  trackfw ship -m \"...\"     commit + push + PR (composition)\n\n"
            "Behavioral steps:\n\n"
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
            "'--suggest' takes a completely separate path: it prints a heuristic Conventional "
            "Commits skeleton built from 'git diff --cached --name-status' (type + staged file "
            "list) and exits without ever committing — no LLM call, just a structural "
            "heuristic. It is not a ready-to-use message; review and edit before using it with "
            "-m. When '--suggest' is set, '-m' (if also passed) is ignored and no commit ever "
            "happens.\n\n"
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
    commit_parser.add_argument(
        "--suggest",
        action="store_true",
        default=False,
        help=(
            "Print a heuristic Conventional Commits message skeleton from staged files and "
            "exit without committing (ignores -m)"
        ),
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


def _default_staged_name_status() -> tuple[str, str | None]:
    """Runs `git diff --cached --name-status` and returns (raw_output, error_or_None)."""
    try:
        result = subprocess.run(
            ["git", "diff", "--cached", "--name-status"],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            return ("", result.stderr.strip() or "git diff --cached --name-status failed")
        return (result.stdout, None)
    except FileNotFoundError:
        return ("", "git not found in PATH")
    except Exception as e:  # noqa: BLE001
        return ("", str(e))


# ────────────────────────────────────────────────────────────────────────────
# --suggest: heurística de mensagem de commit a partir do diff staged
# ────────────────────────────────────────────────────────────────────────────

def parse_staged_name_status(raw: str) -> list[tuple[str, str]]:
    """Parses raw `git diff --cached --name-status` output (linhas
    "<status>\\t<path>" separadas por tab) em pares (status, path), pulando linhas
    em branco. Espelha parseStagedNameStatus em internal/commands/commit.go."""
    files: list[tuple[str, str]] = []
    for line in raw.split("\n"):
        line = line.rstrip("\r")
        if not line.strip():
            continue
        parts = line.split("\t", 1)
        if len(parts) != 2:
            continue
        files.append((parts[0].strip(), parts[1]))
    return files


def is_test_file(path: str) -> bool:
    """Reports whether path matches one of the recognized test-file naming conventions
    across the 3 supported stacks: *_test.go, *.test.js, test_*.py, *_test.py."""
    base = path.rsplit("/", 1)[-1]
    if base.endswith("_test.go"):
        return True
    if base.endswith(".test.js"):
        return True
    if base.startswith("test_") and base.endswith(".py"):
        return True
    if base.endswith("_test.py"):
        return True
    return False


def is_docs_file(path: str) -> bool:
    """Reports whether path lives under docs/ or vault/, or has a .md extension."""
    if path.startswith("docs/") or path.startswith("vault/"):
        return True
    return path.endswith(".md")


def is_under_any_dir(path: str, dirs) -> bool:
    """Reports whether path starts with any of the given directory prefixes."""
    return any(path.startswith(d) for d in dirs)


def suggested_commit_type(files: list[tuple[str, str]]) -> str:
    """Returns the Conventional Commits type suggested for a set of staged files, following
    the fixed-priority heuristic documented in ML-1A of
    docs/roadmaps/wip/ROADMAP-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md
    (first matching rule wins — this is a deliberately simple heuristic, not an attempt at
    perfect classification). Espelha suggestedCommitType em internal/commands/commit.go:
      1. every staged file matches a test-file pattern -> "test"
      2. every staged file is under docs/ or vault/, or has a .md extension -> "docs"
      3. at least one new ("A") file lives under one of COMMIT_COMMAND_DIRS -> "feat"
      4. otherwise -> "fix"
    """
    all_tests = True
    all_docs = True
    has_new_command_file = False

    for status, path in files:
        if not is_test_file(path):
            all_tests = False
        if not is_docs_file(path):
            all_docs = False
        if status == "A" and is_under_any_dir(path, COMMIT_COMMAND_DIRS):
            has_new_command_file = True

    if all_tests:
        return "test"
    if all_docs:
        return "docs"
    if has_new_command_file:
        return "feat"
    return "fix"


def build_suggested_message(staged_name_status=None) -> tuple[str, str | None]:
    """Implements `trackfw commit --suggest`: reads the staged diff via staged_name_status,
    classifies it with suggested_commit_type, and renders the heuristic Conventional Commits
    skeleton described in ML-1A. It never calls exec_git_commit — no commit ever happens as a
    side effect of this function. Espelha buildSuggestedMessage em
    internal/commands/commit.go. Returns (message, error_or_None)."""
    staged_name_status = staged_name_status or _default_staged_name_status

    raw, err = staged_name_status()
    if err is not None:
        return ("", f"could not read staged changes (are you in a git repo?): {err}")

    files = parse_staged_name_status(raw)
    if not files:
        return ("", "nothing staged — `git add` files first")

    commit_type = suggested_commit_type(files)

    lines = [
        "# Sugestão heurística — NÃO é uma mensagem pronta, revise antes de usar.",
        f"# Tipo sugerido: {commit_type}",
        "",
        f"{commit_type}(<escopo>): <descrição>",
        "",
        "## Arquivos staged",
    ]
    for status, path in files:
        lines.append(f"{status}  {path}")

    return ("\n".join(lines), None)


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
    # --suggest vence mesmo se -m também foi passado: toma um caminho completamente
    # separado e nunca chega em run_commit/git commit real.
    if getattr(args, "suggest", False):
        message, err = build_suggested_message()
        if err is not None:
            sys.stderr.write(err + "\n")
            sys.exit(1)
        sys.stdout.write(message + "\n")
        sys.exit(0)

    if not args.message.strip():
        sys.stderr.write(
            'commit message is required — use -m:\n'
            '  trackfw commit -m "feat(<scope>): <description>"\n'
        )
        sys.exit(1)
    exit_code = run_commit(args.message)
    sys.exit(exit_code)
