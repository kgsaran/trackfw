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


# ────────────────────────────────────────────────────────────────────────────
# `trackfw branch prune` — espelha internal/commands/branch_prune.go byte a byte em
# comportamento e texto de mensagem (Go é a referência comportamental,
# docs/cli-parity.md). Decide se uma branch local é segura para apagar pela
# heurística de arquivos-tocados documentada em CLAUDE.md §1 e
# REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-
# correta-de-squash-merge.md — NÃO pela ancestralidade do próprio git
# (`git branch -d`, que sempre recusa branch squash-mergeada) e NÃO por um diff
# bidirecional ingênuo contra origin/main (que dá falso-positivo numa branch
# integrada porém defasada, quando a main avançou por outros PRs).
#
# evaluate_branch_integration é a implementação única e compartilhada — o ML-2A
# (pypi/trackfw/ship/runner.py:_detect_pending_squash_merges) deve chamá-la em vez
# de reimplementar o diff bidirecional.
# ────────────────────────────────────────────────────────────────────────────

# Única fonte de verdade que este comando consulta: o ref de rastreamento local da
# branch default. Por decisão 2 da REQ-2026-08-18, não há consulta a forge nem
# chamada de rede — offline e determinístico por desenho. Se este ref não puder
# ser resolvido (sem remoto configurado, ou nunca dado fetch), o comando inteiro
# se recusa e não apaga nada.
BRANCH_PRUNE_DEFAULT_REMOTE_REF = "origin/main"

# Nome da branch local que corresponde a BRANCH_PRUNE_DEFAULT_REMOTE_REF. Sempre
# excluída como candidata a apagar — avaliá-la contra si mesma reportaria "sem
# trabalho próprio" e ofereceria para apagar a branch que o usuário deve manter
# (merge-base origin/main main == a própria ponta de main, então "touched" fica
# trivialmente vazio). É o bug de maior severidade que a heurística ingênua
# contém.
BRANCH_PRUNE_DEFAULT_LOCAL_NAME = "main"

BRANCH_PRUNE_DECISION_DEFAULT_BRANCH = "default_branch"
BRANCH_PRUNE_DECISION_CURRENT_BRANCH = "current_branch"
BRANCH_PRUNE_DECISION_WORKTREE = "worktree_branch"
BRANCH_PRUNE_DECISION_NO_OWN_WORK = "no_own_work"
BRANCH_PRUNE_DECISION_IDENTICAL = "content_identical"
BRANCH_PRUNE_DECISION_PENDING_WORK = "pending_work"
BRANCH_PRUNE_DECISION_NO_MERGE_BASE = "no_merge_base"
BRANCH_PRUNE_DECISION_EVAL_ERROR = "eval_error"

# review_doc_config é uma categoria NÃO apagável, distinta de pending_work: todo arquivo
# divergente é doc/config (is_doc_or_config_path), então é provavelmente resíduo de housekeeping
# de um squash-merge em vez de trabalho pendente genuíno — mas nunca é apagado automaticamente.
# O próprio CLAUDE.md §1 trata esse caso como "housekeeping, apagar", mas com um humano já no
# laço lendo o diff; um comando destrutivo não tem humano no laço por construção, então a
# REQ-2026-08-18 (ML-1B) restringe deliberadamente esse passo para "sinalizar para confirmação"
# em vez de "apagar automaticamente". Ver branch_prune_is_deletable abaixo.
BRANCH_PRUNE_DECISION_REVIEW_DOC_CONFIG = "review_doc_config"

_BRANCH_PRUNE_DELETABLE_DECISIONS = {
    BRANCH_PRUNE_DECISION_NO_OWN_WORK,
    BRANCH_PRUNE_DECISION_IDENTICAL,
}

# _DOC_CONFIG_EXTENSIONS / _DOC_CONFIG_BASENAMES espelham
# internal/commands/branch_prune.go's branchPruneDocConfigExtensions/branchPruneDocConfigBasenames
# — deliberadamente conservador e best-effort. Classificar mal um arquivo aqui nunca causa uma
# deleção: só muda em qual categoria não-apagável (review_doc_config vs pending_work) uma branch
# mantida é reportada.
_DOC_CONFIG_EXTENSIONS = (".yaml", ".yml", ".json", ".toml", ".ini", ".cfg")
_DOC_CONFIG_BASENAMES = {".gitignore", ".gitattributes", ".editorconfig", "trackfw.yaml", "LICENSE"}


def _is_docs_file(path: str) -> bool:
    """Reporta se path vive sob docs/ ou vault/, ou tem extensão .md. Espelha
    internal/commands/commit.go's isDocsFile (mesmo pacote lá; standalone aqui)."""
    if path.startswith("docs/") or path.startswith("vault/"):
        return True
    return path.endswith(".md")


def is_doc_or_config_path(path: str) -> bool:
    """Reporta se path é um arquivo de doc (_is_docs_file) ou um arquivo/extensão de config
    não-runtime bem conhecido. Usado só para rotear uma branch que seria pending_work para
    review_doc_config — nunca para tornar algo apagável."""
    if _is_docs_file(path):
        return True
    base = path.rsplit("/", 1)[-1]
    if base in _DOC_CONFIG_BASENAMES:
        return True
    return any(path.endswith(ext) for ext in _DOC_CONFIG_EXTENSIONS)


def all_doc_or_config(paths) -> bool:
    """Reporta se todo path em paths é doc/config (is_doc_or_config_path). Lista vazia retorna
    False."""
    if not paths:
        return False
    return all(is_doc_or_config_path(p) for p in paths)


def branch_prune_is_deletable(decision: str) -> bool:
    """Reporta se decision, por si só, torna a branch candidata a apagar. Tanto
    no_own_work (squash-merge sem ancestralidade — o falso negativo do
    `git branch -d`) quanto content_identical (defasada porém integrada — o
    falso positivo do diff ingênuo) são seguras para apagar; qualquer outra
    decisão mantém a branch."""
    return decision in _BRANCH_PRUNE_DELETABLE_DECISIONS


def split_nul_paths(raw: str):
    """Divide uma saída NUL-separada de `git diff --name-only -z` numa lista de
    caminhos ordenada e sem vazios. Um NUL final (git sempre emite um depois da
    última entrada) produz um elemento vazio final, que é descartado."""
    parts = raw.split("\x00")
    return sorted(p for p in parts if p != "")


def evaluate_branch_integration(branch: str, exec_git) -> dict:
    """Decide se branch é seguro apagar em relação a
    BRANCH_PRUNE_DEFAULT_REMOTE_REF, usando a heurística de arquivos-tocados:

        mb      = git merge-base origin/main <branch>
        touched = git diff --name-only mb <branch>                       (o que a branch tocou)
        diverg  = git diff --name-only origin/main <branch> -- touched   (o que ainda difere lá)

    touched vazio    -> no_own_work (apagável)       -- o falso negativo squash-merge/-d
    diverg vazio     -> content_identical (apagável) -- o falso positivo do diff ingênuo
    diverg não-vazio -> pending_work (mantida, explicada)

    Ambas as chamadas de diff usam -z (separadas por NUL, caminhos sem quoting)
    para que nomes de arquivo com espaço ou bytes não-ASCII nunca sejam
    mal-divididos — exatamente a classe de bug que faria uma branch com trabalho
    pendente em "foo bar.md" ler diverg como vazio e ser apagada.

    exec_git(args: list[str]) -> (stdout_str, error_str_or_None) — mesmo
    contrato de trackfw.ship.runner.default_exec_git.
    """
    mb, mb_err = exec_git(["merge-base", BRANCH_PRUNE_DEFAULT_REMOTE_REF, branch])
    mb = (mb or "").strip()
    if mb_err is not None or mb == "":
        return {
            "name": branch,
            "decision": BRANCH_PRUNE_DECISION_NO_MERGE_BASE,
            "reason": (
                f"no merge-base with {BRANCH_PRUNE_DEFAULT_REMOTE_REF} — refusing "
                "(unrelated history or bad ref)"
            ),
            "touched": [],
            "diverged": [],
        }

    touched_raw, touched_err = exec_git(["diff", "--name-only", "-z", mb, branch])
    if touched_err is not None:
        return {
            "name": branch,
            "decision": BRANCH_PRUNE_DECISION_EVAL_ERROR,
            "reason": f"git diff --name-only -z {mb} {branch} failed: {touched_err}",
            "touched": [],
            "diverged": [],
        }
    touched = split_nul_paths(touched_raw)

    if not touched:
        return {
            "name": branch,
            "decision": BRANCH_PRUNE_DECISION_NO_OWN_WORK,
            "reason": f"no own work relative to {BRANCH_PRUNE_DEFAULT_REMOTE_REF} — safe to delete",
            "touched": [],
            "diverged": [],
        }

    diverg_raw, diverg_err = exec_git(
        ["diff", "--name-only", "-z", BRANCH_PRUNE_DEFAULT_REMOTE_REF, branch, "--", *touched]
    )
    if diverg_err is not None:
        return {
            "name": branch,
            "decision": BRANCH_PRUNE_DECISION_EVAL_ERROR,
            "reason": (
                f"git diff --name-only -z {BRANCH_PRUNE_DEFAULT_REMOTE_REF} {branch} "
                f"-- <touched> failed: {diverg_err}"
            ),
            "touched": touched,
            "diverged": [],
        }
    diverg = split_nul_paths(diverg_raw)

    if not diverg:
        return {
            "name": branch,
            "decision": BRANCH_PRUNE_DECISION_IDENTICAL,
            "reason": (
                f"squash-merged into {BRANCH_PRUNE_DEFAULT_REMOTE_REF} — content identical "
                "in touched files, safe to delete"
            ),
            "touched": touched,
            "diverged": [],
        }

    # review_doc_config exige que diverg seja um subconjunto PRÓPRIO de touched
    # (len(diverg) < len(touched)) — não basta ser "tudo doc/config". diverg já é
    # subconjunto de touched por construção (o segundo git diff é escopado `-- touched`),
    # então isso equivale a: pelo menos um arquivo tocado entrou na main. Isso distingue
    # resíduo genuíno de housekeeping de squash-merge de uma branch cujo conjunto tocado
    # inteiro — doc/config ou não — nunca chegou na main (diverg == touched): isso é
    # trabalho pendente, qualquer que seja o tipo de arquivo.
    if len(diverg) < len(touched) and all_doc_or_config(diverg):
        return {
            "name": branch,
            "decision": BRANCH_PRUNE_DECISION_REVIEW_DOC_CONFIG,
            "reason": (
                f"only doc/config files diverge from {BRANCH_PRUNE_DEFAULT_REMOTE_REF} "
                f"({', '.join(diverg)}) — probable housekeeping, confirm and delete manually"
            ),
            "touched": touched,
            "diverged": diverg,
        }

    return {
        "name": branch,
        "decision": BRANCH_PRUNE_DECISION_PENDING_WORK,
        "reason": f"pending work vs {BRANCH_PRUNE_DEFAULT_REMOTE_REF}: {', '.join(diverg)}",
        "touched": touched,
        "diverged": diverg,
    }


def _default_list_local_branches(exec_git):
    """Roda `git branch --format=%(refname:short)` e retorna uma lista com uma
    branch por linha não-vazia. Retorna (branches, error_str_or_None)."""
    raw, err = exec_git(["branch", "--format=%(refname:short)"])
    if err is not None:
        return [], err
    branches = [line.strip() for line in raw.split("\n") if line.strip() != ""]
    return branches, None


def _default_current_branch_for_prune(exec_git):
    """Retorna o nome curto da branch atual, ou "" em HEAD destacado."""
    name, err = exec_git(["symbolic-ref", "--quiet", "--short", "HEAD"])
    if err is not None:
        return ""
    return (name or "").strip()


def _default_worktree_branches(exec_git):
    """Faz parsing de `git worktree list --porcelain` e retorna o conjunto de
    nomes curtos de branch em uso em qualquer worktree. Usa a linha porcelain
    "branch refs/heads/<name>", não o formato legível por humano."""
    raw, err = exec_git(["worktree", "list", "--porcelain"])
    result = set()
    if err is not None:
        return result
    prefix = "branch refs/heads/"
    for raw_line in raw.split("\n"):
        line = raw_line.strip()
        if line.startswith(prefix):
            result.add(line[len(prefix):])
    return result


def _default_delete_branch(exec_git, name):
    """Tenta `git branch -d <name>` primeiro. Quando a branch também tem
    ancestralidade fast-forward com main (um merge simples, não squash), -d tem
    sucesso sozinho e confirma a integração pelo próprio check independente do
    git — sem nunca precisar de -D. Cai para `git branch -D <name>` só quando
    -d recusa, o resultado esperado para branches squash-mergeadas (sem
    ancestralidade por construção): toda a segurança já vive em
    evaluate_branch_integration e na reconferência imediatamente antes desta
    chamada, não em qual flag efetivamente apaga."""
    _, err = exec_git(["branch", "-d", name])
    if err is None:
        return None
    _, err = exec_git(["branch", "-D", name])
    return err


def run_branch_prune(
    apply: bool = False,
    exec_git=None,
    list_local_branches=None,
    current_branch=None,
    worktree_branches=None,
    delete_branch=None,
    out=None,
    err_out=None,
) -> int:
    """Implementa `trackfw branch prune`.

    --dry-run é o padrão (apply=False): sem --apply, nada é apagado, nem o
    claramente integrado. A branch atual, qualquer branch em outro worktree, e a
    branch default (main) são sempre mantidas e nunca avaliadas para apagar.
    Sem origin/main resolvível (offline, sem remoto, nunca deu fetch), o comando
    inteiro se recusa e não apaga nada.

    Retorna o exit code do processo (0 = rodou até o fim, 1 = origin/main
    irresolvível).
    """
    from ..ship import runner as _ship_runner  # import tardio: mantém commands/branch.py sem
    # dependência de import-time em ship/runner.py, usado só para o default de exec_git.

    exec_git = exec_git or _ship_runner.default_exec_git
    list_local_branches = list_local_branches or _default_list_local_branches
    current_branch = current_branch or _default_current_branch_for_prune
    worktree_branches = worktree_branches or _default_worktree_branches
    delete_branch = delete_branch or _default_delete_branch
    out = out or sys.stdout
    err_out = err_out or sys.stderr

    # Best-effort `git fetch origin --prune`, conforme CLAUDE.md §1 passo 1. Falha é
    # não-bloqueante — offline é um caso de uso legítimo, a mesma postura que o check de
    # squash-merge do `trackfw ship` já adota (ship/runner.py) — mas diferente do ship (que pula
    # o check inteiro na falha de fetch), a avaliação abaixo continua contra qualquer ref
    # origin/main já resolvível localmente: um ref defasado só torna o resultado MAIS
    # conservador, nunca menos.
    _, fetch_err = exec_git(["fetch", "origin", "--prune"])
    if fetch_err is not None:
        out.write(
            "Warning: could not fetch origin (offline, no remote, or fetch failed) — "
            "evaluating with possibly stale data; a branch merged upstream since the last "
            "fetch may still be reported as pending.\n"
        )

    _, origin_err = exec_git(["rev-parse", "--verify", "-q", BRANCH_PRUNE_DEFAULT_REMOTE_REF])
    if origin_err is not None:
        out.write(
            f"trackfw branch prune: {BRANCH_PRUNE_DEFAULT_REMOTE_REF} not found — offline, "
            "no remote configured, or never fetched. Refusing to evaluate any branch; nothing "
            "deleted.\n"
        )
        # Espelha internal/commands/branch_prune.go: a mensagem legível vai para stdout acima, e
        # o erro puro (mesmo padrão de `branch new`) vai para stderr.
        err_out.write(f"branch prune: {BRANCH_PRUNE_DEFAULT_REMOTE_REF} not resolvable\n")
        return 1

    branches, list_err = list_local_branches(exec_git)
    if list_err is not None:
        out.write(f"trackfw branch prune: failed to list local branches: {list_err}\n")
        err_out.write(f"{list_err}\n")
        return 1
    branches = sorted(branches)

    current = current_branch(exec_git)
    worktreed = worktree_branches(exec_git)

    out.write(
        f"trackfw branch prune — evaluating {len(branches)} local branch(es) against "
        f"{BRANCH_PRUNE_DEFAULT_REMOTE_REF}\n\n"
    )

    to_delete = []
    to_review = []
    for b in branches:
        if b == BRANCH_PRUNE_DEFAULT_LOCAL_NAME:
            ev = {"name": b, "decision": BRANCH_PRUNE_DECISION_DEFAULT_BRANCH, "reason": "default branch — never pruned"}
        elif b == current:
            ev = {"name": b, "decision": BRANCH_PRUNE_DECISION_CURRENT_BRANCH, "reason": "current branch — never pruned"}
        elif b in worktreed:
            ev = {"name": b, "decision": BRANCH_PRUNE_DECISION_WORKTREE, "reason": "checked out in another worktree — never pruned"}
        else:
            ev = evaluate_branch_integration(b, exec_git)

        action = "keep"
        if branch_prune_is_deletable(ev["decision"]):
            action = "delete"
            to_delete.append(b)
        elif ev["decision"] == BRANCH_PRUNE_DECISION_REVIEW_DOC_CONFIG:
            action = "review"
            to_review.append(b)
        out.write(f"  {ev['name']:<30} {action:<7} {ev['reason']}\n")

    out.write("\n")
    if to_review:
        out.write(
            f"{len(to_review)} branch(es) need manual review (only doc/config diverges, "
            f"never auto-deleted): {', '.join(to_review)}\n"
        )
    if not apply:
        if not to_delete:
            out.write("[dry-run] nothing to delete.\n")
        else:
            out.write(
                f"[dry-run] would delete {len(to_delete)} branch(es): {', '.join(to_delete)}. "
                "Rerun with --apply to delete.\n"
            )
        return 0

    if not to_delete:
        out.write("nothing to delete.\n")
        return 0

    deleted = []
    for b in to_delete:
        # Reconfirma imediatamente antes de cada delete — cinto e suspensório contra a
        # branch mudar de estado entre o relatório acima e este loop.
        if b == current_branch(exec_git):
            out.write(f"skip {b}: became the current branch — refusing to delete\n")
            continue
        if b in worktree_branches(exec_git):
            out.write(f"skip {b}: became checked out in a worktree — refusing to delete\n")
            continue
        del_err = delete_branch(exec_git, b)
        if del_err is not None:
            out.write(f"failed to delete {b}: {del_err}\n")
            continue
        deleted.append(b)

    if not deleted:
        out.write("deleted 0 branch(es).\n")
    else:
        out.write(f"deleted {len(deleted)} branch(es): {', '.join(deleted)}\n")
    return 0


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

    prune_p = sub.add_parser(
        "prune",
        help=(
            "Report (and, with --apply, delete) local branches already integrated into "
            "origin/main"
        ),
        description=(
            "trackfw branch prune automates the \"one active branch at a time\" check "
            "documented in CLAUDE.md §1 — it does not remove human judgment from every case: "
            "a branch whose only remaining divergence is doc/config files is flagged for "
            "manual review, never deleted automatically.\n\n"
            "A best-effort 'git fetch origin --prune' runs first. Failure (offline, no "
            "remote) is non-blocking: a warning is printed and evaluation proceeds against "
            "the local origin/main ref, whatever its state. A stale origin/main only ever "
            "makes the result MORE conservative — it can miss a branch that was in fact "
            "integrated since the last fetch (reporting it kept when a fresh fetch would show "
            "it deletable), but it never reports one as deletable that a fresh fetch would "
            "show as pending.\n\n"
            "Decides integration with the touched-files heuristic, NOT git's own ancestry "
            "check (which always refuses squash-merged branches) and NOT a naive "
            "bidirectional diff against origin/main (which false-positives on a branch that "
            "is merged but stale, once main has advanced further):\n\n"
            "  mb      = git merge-base origin/main <branch>\n"
            "  touched = git diff --name-only mb <branch>                 "
            "(what the branch touched)\n"
            "  diverg  = git diff --name-only origin/main <branch> -- touched  "
            "(what still differs there)\n\n"
            "touched empty          -> integrated (safe to delete)\n"
            "diverg empty           -> integrated (safe to delete) -- squash-merged, stale, "
            "main advanced since\n"
            "diverg doc/config only -> flagged for review (kept; probable housekeeping, "
            "confirm and delete manually)\n"
            "otherwise               -> kept, with the diverging files named\n\n"
            "Every local branch is reported, always, with its decision and reason. The "
            "current branch, any branch checked out in another worktree, and the default "
            "branch (main) are always kept and never evaluated for deletion. Without "
            "origin/main resolvable at all (offline with no prior fetch ever having run, or "
            "no remote configured), the whole command refuses and deletes nothing.\n\n"
            "--dry-run is the default: without --apply, nothing is ever deleted, even the "
            "clearly integrated. Deletion tries 'git branch -d' first — confirming the "
            "integration via git's own ancestry check too, when possible — and falls back to "
            "'git branch -D' only when -d refuses, the expected case for squash-merged "
            "branches, which never have fast-forward ancestry with main."
        ),
    )
    prune_p.add_argument(
        "--apply",
        action="store_true",
        default=False,
        help=(
            "Actually delete branches decided as integrated (default: report only, delete "
            "nothing)"
        ),
    )
    prune_p.set_defaults(func=_dispatch_prune)

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


def _dispatch_prune(args):
    exit_code = run_branch_prune(apply=args.apply)
    sys.exit(exit_code)
