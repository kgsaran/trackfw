"""
test_commit.py — Testes para trackfw.commands.commit (`trackfw commit -m`).

Espelha internal/commands/commit_test.go — mesmos 4 cenários mínimos exigidos
pelo roadmap (bloqueio em main, bloqueio em feat/x sem roadmap em wip, sucesso
em feat/x com roadmap em wip, sucesso em branch fora do padrão
feat/fix/refactor), mais cobertura equivalente aos casos extra do Go (master,
fix/refactor, candidatos sem match, erro ao resolver a branch atual, slug
normalizado). Usa injeção de dependência (mesmo padrão de test_branch.py) —
nenhum teste toca um repositório git real.
"""

from __future__ import annotations

import io

from trackfw.commands.commit import (
    commit_governed_branch_prefix,
    run_commit,
)
from trackfw import validator as _validator


def _make_kwargs(branch: str, matched: bool = True, candidates: list | None = None):
    """Builds run_commit kwargs wired to injectable fakes, plus references to the commit_calls
    list and out buffer, so tests never touch a real git repository or the real project
    filesystem layout."""
    commit_calls: list[str] = []
    match_calls: list[str] = []
    out = io.StringIO()

    def fake_current_branch():
        return branch, None

    def fake_match(slug, wip_dirs, done_dirs):
        match_calls.append(slug)
        return matched, candidates or []

    def fake_commit(message):
        commit_calls.append(message)
        return 0

    kwargs = dict(
        load_config=lambda: {},
        current_branch=fake_current_branch,
        resolve_wip_dirs=lambda cfg: ["docs/roadmaps/wip"],
        resolve_done_dirs=lambda cfg: ["docs/roadmaps/done"],
        match_slug=fake_match,
        exec_git_commit=fake_commit,
        out=out,
    )
    return kwargs, out, commit_calls, match_calls


# ────────────────────────────────────────────────────────────────────────────
# (a) main/master: sempre bloqueado
# ────────────────────────────────────────────────────────────────────────────

def test_run_commit_blocks_on_main():
    kwargs, out, commit_calls, match_calls = _make_kwargs("main")
    exit_code = run_commit("feat: something", **kwargs)
    assert exit_code != 0
    assert commit_calls == []
    assert match_calls == []
    assert (
        'trackfw commit: commit direto em "main" não é permitido. '
        "Use 'trackfw branch new <type>/<slug>' primeiro. Ver CLAUDE.md §1."
    ) in out.getvalue()


def test_run_commit_blocks_on_master():
    kwargs, out, commit_calls, match_calls = _make_kwargs("master")
    exit_code = run_commit("feat: something", **kwargs)
    assert exit_code != 0
    assert commit_calls == []
    assert match_calls == []
    assert (
        'trackfw commit: commit direto em "master" não é permitido. '
        "Use 'trackfw branch new <type>/<slug>' primeiro. Ver CLAUDE.md §1."
    ) in out.getvalue()


# ────────────────────────────────────────────────────────────────────────────
# (b) feat/fix/refactor sem roadmap em wip/done: bloqueia, nunca comita
# ────────────────────────────────────────────────────────────────────────────

def test_run_commit_blocks_governed_branch_no_match_no_candidates():
    kwargs, out, commit_calls, match_calls = _make_kwargs(
        "feat/orphan-slug", matched=False, candidates=None
    )
    exit_code = run_commit("feat: something", **kwargs)
    assert exit_code != 0
    assert commit_calls == []
    want = _validator.branch_governance_orientation("feat/orphan-slug")
    assert want in out.getvalue()


def test_run_commit_blocks_governed_branch_no_match_with_candidates():
    candidates = ["ROADMAP-other-thing.md"]
    kwargs, out, commit_calls, match_calls = _make_kwargs(
        "fix/orphan-slug", matched=False, candidates=candidates
    )
    exit_code = run_commit("fix: something", **kwargs)
    assert exit_code != 0
    assert commit_calls == []
    want = _validator.branch_no_matching_roadmap_message("fix/orphan-slug", candidates)
    assert want in out.getvalue()


# ────────────────────────────────────────────────────────────────────────────
# (b) feat/fix/refactor com roadmap correspondente: comita
# ────────────────────────────────────────────────────────────────────────────

def test_run_commit_governed_branch_match_commits():
    kwargs, out, commit_calls, match_calls = _make_kwargs("feat/my-slug", matched=True)
    exit_code = run_commit("feat: something", **kwargs)
    assert exit_code == 0
    assert commit_calls == ["feat: something"]


def test_run_commit_governed_branch_match_fix_and_refactor():
    for branch in ("fix/my-slug", "refactor/my-slug"):
        kwargs, out, commit_calls, match_calls = _make_kwargs(branch, matched=True)
        exit_code = run_commit("chore: something", **kwargs)
        assert exit_code == 0
        assert commit_calls == ["chore: something"]


def test_run_commit_uses_normalized_slug_for_matching():
    kwargs, out, commit_calls, match_calls = _make_kwargs("feat/My_Weird--Slug", matched=True)
    exit_code = run_commit("feat: something", **kwargs)
    assert exit_code == 0
    assert match_calls == [_validator.normalize_branch_slug("My_Weird--Slug")]


# ────────────────────────────────────────────────────────────────────────────
# (c) branches fora do padrão feat/fix/refactor: comita sem exigir roadmap
# ────────────────────────────────────────────────────────────────────────────

def test_run_commit_ungoverned_branch_commits_with_warning():
    kwargs, out, commit_calls, match_calls = _make_kwargs("docs/housekeeping")
    exit_code = run_commit("docs: something", **kwargs)
    assert exit_code == 0
    assert commit_calls == ["docs: something"]
    assert match_calls == []
    assert (
        'trackfw commit: branch "docs/housekeeping" does not follow feat/fix/refactor — '
        "committing without a roadmap check."
    ) in out.getvalue()


# ────────────────────────────────────────────────────────────────────────────
# Propagação de erro ao resolver a branch atual
# ────────────────────────────────────────────────────────────────────────────

def test_run_commit_current_branch_error_propagates():
    commit_calls: list[str] = []
    out = io.StringIO()

    def failing_current_branch():
        return "", "not a git repository"

    exit_code = run_commit(
        "feat: something",
        load_config=lambda: {},
        current_branch=failing_current_branch,
        resolve_wip_dirs=lambda cfg: [],
        resolve_done_dirs=lambda cfg: [],
        match_slug=lambda slug, wip_dirs, done_dirs: (True, []),
        exec_git_commit=lambda message: commit_calls.append(message) or 0,
        out=out,
    )
    assert exit_code != 0
    assert commit_calls == []


# ────────────────────────────────────────────────────────────────────────────
# commit_governed_branch_prefix
# ────────────────────────────────────────────────────────────────────────────

def test_commit_governed_branch_prefix():
    for branch, expected_prefix in (
        ("feat/x", "feat/"),
        ("fix/x", "fix/"),
        ("refactor/x", "refactor/"),
    ):
        prefix, matched = commit_governed_branch_prefix(branch)
        assert matched is True
        assert prefix == expected_prefix


def test_commit_governed_branch_prefix_no_match():
    prefix, matched = commit_governed_branch_prefix("docs/housekeeping")
    assert matched is False
    assert prefix == ""
