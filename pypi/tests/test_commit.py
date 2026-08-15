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

import argparse
import io

import pytest

from trackfw.commands import commit as _commit
from trackfw.commands.commit import (
    build_suggested_message,
    commit_governed_branch_prefix,
    is_docs_file,
    is_test_file,
    parse_staged_name_status,
    run_commit,
    suggested_commit_type,
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


# ────────────────────────────────────────────────────────────────────────────
# build_suggested_message — --suggest nunca comita, classifica por heurística
# ────────────────────────────────────────────────────────────────────────────

def _make_staged(raw: str, err: str | None = None):
    """Builds a fake staged_name_status callable, mirroring makeSuggestDeps in
    commit_test.go (Go)."""
    def fake():
        return raw, err
    return fake


def test_build_suggested_message_nothing_staged_errors():
    message, err = build_suggested_message(staged_name_status=_make_staged(""))
    assert err is not None
    assert "nothing staged" in err
    assert message == ""


def test_build_suggested_message_staged_read_error_propagates():
    message, err = build_suggested_message(
        staged_name_status=_make_staged("", "not a git repository")
    )
    assert err is not None
    assert message == ""


def test_build_suggested_message_only_test_files_suggests_test():
    raw = "M\tinternal/commands/commit_test.go\nA\tnpm/src/commit/runner.test.js\n"
    message, err = build_suggested_message(staged_name_status=_make_staged(raw))
    assert err is None
    assert "# Tipo sugerido: test" in message
    assert "test(<escopo>): <descrição>" in message
    assert "M  internal/commands/commit_test.go" in message


def test_build_suggested_message_only_docs_files_suggests_docs():
    raw = "M\tdocs/roadmaps/wip/ROADMAP-x.md\nA\tvault/notes/example.md\nM\tREADME.md\n"
    message, err = build_suggested_message(staged_name_status=_make_staged(raw))
    assert err is None
    assert "# Tipo sugerido: docs" in message


def test_build_suggested_message_new_command_file_suggests_feat():
    raw = "A\tpypi/trackfw/commands/newthing.py\nM\tpypi/trackfw/commands/commit.py\n"
    message, err = build_suggested_message(staged_name_status=_make_staged(raw))
    assert err is None
    assert "# Tipo sugerido: feat" in message


def test_build_suggested_message_generic_change_suggests_fix():
    raw = "M\tpypi/trackfw/commands/commit.py\nM\tpypi/trackfw/config.py\n"
    message, err = build_suggested_message(staged_name_status=_make_staged(raw))
    assert err is None
    assert "# Tipo sugerido: fix" in message


def test_build_suggested_message_exact_template():
    raw = "A\tpypi/trackfw/commands/newthing.py\n"
    message, err = build_suggested_message(staged_name_status=_make_staged(raw))
    assert err is None
    expected = (
        "# Sugestão heurística — NÃO é uma mensagem pronta, revise antes de usar.\n"
        "# Tipo sugerido: feat\n"
        "\n"
        "feat(<escopo>): <descrição>\n"
        "\n"
        "## Arquivos staged\n"
        "A  pypi/trackfw/commands/newthing.py"
    )
    assert message == expected


# ────────────────────────────────────────────────────────────────────────────
# suggested_commit_type / parse_staged_name_status / is_test_file / is_docs_file
# — cobertura direta das funções auxiliares
# ────────────────────────────────────────────────────────────────────────────

def test_parse_staged_name_status_skips_blank_lines():
    raw = "A\tfoo.py\n\nM\tbar.py\n"
    files = parse_staged_name_status(raw)
    assert files == [("A", "foo.py"), ("M", "bar.py")]


def test_is_test_file_recognizes_all_patterns():
    assert is_test_file("internal/commands/commit_test.go") is True
    assert is_test_file("npm/src/commit/runner.test.js") is True
    assert is_test_file("pypi/tests/test_commit.py") is True
    assert is_test_file("pypi/trackfw/commands/commit_test.py") is True
    assert is_test_file("pypi/trackfw/commands/commit.py") is False


def test_is_docs_file_recognizes_docs_vault_and_md():
    assert is_docs_file("docs/roadmaps/wip/x.md") is True
    assert is_docs_file("vault/notes/example.md") is True
    assert is_docs_file("README.md") is True
    assert is_docs_file("pypi/trackfw/commands/commit.py") is False


def test_suggested_commit_type_priority_test_over_others():
    files = [("A", "pypi/tests/test_commit.py")]
    assert suggested_commit_type(files) == "test"


# ────────────────────────────────────────────────────────────────────────────
# --suggest via _dispatch: vence sobre -m, nunca chama run_commit
# ────────────────────────────────────────────────────────────────────────────

def test_dispatch_suggest_never_calls_run_commit_even_with_message(monkeypatch, capsys):
    raw = "A\tpypi/trackfw/commands/newthing.py\n"
    monkeypatch.setattr(_commit, "_default_staged_name_status", _make_staged(raw))

    def fail_if_called(*args, **kwargs):
        raise AssertionError("run_commit must never be called by --suggest")

    monkeypatch.setattr(_commit, "run_commit", fail_if_called)

    args = argparse.Namespace(message="should be ignored", suggest=True)
    with pytest.raises(SystemExit) as excinfo:
        _commit._dispatch(args)
    assert excinfo.value.code == 0

    captured = capsys.readouterr()
    assert "# Tipo sugerido: feat" in captured.out


def test_dispatch_suggest_nothing_staged_exits_nonzero(monkeypatch, capsys):
    monkeypatch.setattr(_commit, "_default_staged_name_status", _make_staged(""))

    args = argparse.Namespace(message="", suggest=True)
    with pytest.raises(SystemExit) as excinfo:
        _commit._dispatch(args)
    assert excinfo.value.code != 0

    captured = capsys.readouterr()
    assert "nothing staged" in captured.err


def test_dispatch_without_suggest_still_requires_message(monkeypatch):
    args = argparse.Namespace(message="", suggest=False)
    with pytest.raises(SystemExit) as excinfo:
        _commit._dispatch(args)
    assert excinfo.value.code != 0
