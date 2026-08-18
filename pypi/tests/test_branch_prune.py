"""
test_branch_prune.py — Testes para trackfw.commands.branch (`trackfw branch prune`).

Espelha internal/commands/branch_prune_test.go e npm/tests/branch-prune.test.js
cenário-a-cenário, para que os 3 CLIs fiquem comportamentalmente idênticos (Go é
a referência comportamental, docs/cli-parity.md).
"""

from __future__ import annotations

import io
import os
import subprocess
import sys
import tempfile

import pytest

from trackfw.commands.branch import (
    BRANCH_PRUNE_DECISION_DEFAULT_BRANCH,
    BRANCH_PRUNE_DECISION_CURRENT_BRANCH,
    BRANCH_PRUNE_DECISION_WORKTREE,
    BRANCH_PRUNE_DECISION_NO_OWN_WORK,
    BRANCH_PRUNE_DECISION_IDENTICAL,
    BRANCH_PRUNE_DECISION_PENDING_WORK,
    BRANCH_PRUNE_DECISION_NO_MERGE_BASE,
    branch_prune_is_deletable,
    split_nul_paths,
    evaluate_branch_integration,
    _default_list_local_branches,
    run_branch_prune,
)


# ────────────────────────────────────────────────────────────────────────────
# split_nul_paths
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.parametrize(
    "raw,want",
    [
        ("", []),
        ("foo.md\x00", ["foo.md"]),
        ("z.md\x00a.md\x00", ["a.md", "z.md"]),
        ("foo bar.md\x00", ["foo bar.md"]),
        ("a.md\x00b.md", ["a.md", "b.md"]),
    ],
)
def test_split_nul_paths(raw, want):
    assert split_nul_paths(raw) == want


# ────────────────────────────────────────────────────────────────────────────
# evaluate_branch_integration — testes unitários com exec_git fake (sem repo git real)
# ────────────────────────────────────────────────────────────────────────────

def _fake_exec_git(responses: dict):
    def exec_git(args):
        key = " ".join(args)
        if key not in responses:
            raise AssertionError(f"fake_exec_git: unexpected call: git {key}")
        return responses[key]
    return exec_git


def test_evaluate_branch_integration_no_own_work_deletable():
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/foo": ("abc123", None),
        "diff --name-only -z abc123 feat/foo": ("", None),
    })
    ev = evaluate_branch_integration("feat/foo", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_NO_OWN_WORK
    assert branch_prune_is_deletable(ev["decision"])


def test_evaluate_branch_integration_content_identical_deletable_ac2():
    # O discriminante do AC2: touched não-vazio (a branch TOCOU arquivos) mas
    # diverg volta vazio (a main convergiu para o mesmo conteúdo nesses
    # arquivos — defasada porém integrada).
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/stale": ("abc123", None),
        "diff --name-only -z abc123 feat/stale": ("f1.md\x00", None),
        "diff --name-only -z origin/main feat/stale -- f1.md": ("", None),
    })
    ev = evaluate_branch_integration("feat/stale", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_IDENTICAL
    assert branch_prune_is_deletable(ev["decision"])


def test_evaluate_branch_integration_pending_work_not_deletable():
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/pending": ("abc123", None),
        "diff --name-only -z abc123 feat/pending": ("f1.md\x00", None),
        "diff --name-only -z origin/main feat/pending -- f1.md": ("f1.md\x00", None),
    })
    ev = evaluate_branch_integration("feat/pending", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_PENDING_WORK
    assert not branch_prune_is_deletable(ev["decision"])
    assert "f1.md" in ev["reason"]


def test_evaluate_branch_integration_no_merge_base_refuses():
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/orphan": ("", "fatal: no merge base"),
    })
    ev = evaluate_branch_integration("feat/orphan", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_NO_MERGE_BASE
    assert not branch_prune_is_deletable(ev["decision"])


# ────────────────────────────────────────────────────────────────────────────
# run_branch_prune — orquestração com deps totalmente injetadas (sem repo git real)
# ────────────────────────────────────────────────────────────────────────────

def _make_prune_kwargs(out):
    def exec_git(args):
        key = " ".join(args)
        table = {
            "rev-parse --verify -q origin/main": ("abc123", None),
            "merge-base origin/main feat/integrated": ("abc123", None),
            "diff --name-only -z abc123 feat/integrated": ("", None),
            "merge-base origin/main feat/pending": ("abc123", None),
            "diff --name-only -z abc123 feat/pending": ("f1.md\x00", None),
            "diff --name-only -z origin/main feat/pending -- f1.md": ("f1.md\x00", None),
        }
        if key not in table:
            raise AssertionError(f"unexpected exec_git call: {key}")
        return table[key]

    def list_local_branches(_exec_git):
        return (["main", "feat/integrated", "feat/pending", "fix/current", "chore/wt"], None)

    def current_branch(_exec_git):
        return "fix/current"

    def worktree_branches(_exec_git):
        return {"chore/wt"}

    def delete_branch(_exec_git, _name):
        raise AssertionError("delete_branch must not be called in dry-run tests")

    return dict(
        exec_git=exec_git,
        list_local_branches=list_local_branches,
        current_branch=current_branch,
        worktree_branches=worktree_branches,
        delete_branch=delete_branch,
        out=out,
    )


def test_run_branch_prune_dry_run_never_deletes_main_never_candidate():
    out = io.StringIO()
    kwargs = _make_prune_kwargs(out)
    delete_called = {"v": False}

    def delete_branch(_exec_git, _name):
        delete_called["v"] = True
        return None

    kwargs["delete_branch"] = delete_branch

    exit_code = run_branch_prune(apply=False, **kwargs)
    assert exit_code == 0
    assert not delete_called["v"]

    got = out.getvalue()
    assert "would delete" in got
    for line in got.split("\n"):
        if line.strip().startswith("main ") and "delete" in line:
            pytest.fail(f"main must never be offered for deletion, got line: {line!r}")
    assert "default branch" in got
    assert "current branch" in got
    assert "worktree" in got


def test_run_branch_prune_apply_deletes_only_integrated_keeps_pending():
    out = io.StringIO()
    kwargs = _make_prune_kwargs(out)
    deleted_names = []

    def delete_branch(_exec_git, name):
        deleted_names.append(name)
        return None

    kwargs["delete_branch"] = delete_branch

    exit_code = run_branch_prune(apply=True, **kwargs)
    assert exit_code == 0
    assert deleted_names == ["feat/integrated"]
    got = out.getvalue()
    assert "deleted 1 branch(es): feat/integrated" in got


def test_run_branch_prune_no_origin_main_refuses_everything():
    out = io.StringIO()

    def exec_git(_args):
        return ("", "fatal: needed a single revision")

    def list_local_branches(_exec_git):
        raise AssertionError("list_local_branches must not be called when origin/main is unresolvable")

    def delete_branch(_exec_git, _name):
        raise AssertionError("delete_branch must not be called when origin/main is unresolvable")

    exit_code = run_branch_prune(
        apply=True,  # mesmo com --apply
        exec_git=exec_git,
        list_local_branches=list_local_branches,
        current_branch=lambda _g: "",
        worktree_branches=lambda _g: set(),
        delete_branch=delete_branch,
        out=out,
    )
    assert exit_code == 1
    assert "origin/main" in out.getvalue()


# ────────────────────────────────────────────────────────────────────────────
# Repositório git real — o discriminante do AC2, espelhando
# internal/commands/branch_prune_test.go /
# npm/tests/branch-prune.test.js. Um mock de `git` só provaria que o mock
# concorda com o código; este teste exercita o git de verdade via um repo bare
# local como "origin" (offline, sem rede) + um clone.
# ────────────────────────────────────────────────────────────────────────────

def _git_available():
    try:
        subprocess.run(["git", "--version"], capture_output=True, check=True)
        return True
    except Exception:
        return False


@pytest.mark.skipif(not _git_available(), reason="git not available in PATH")
def test_evaluate_branch_integration_real_git_repo_squash_merge_and_stale_discriminant():
    with tempfile.TemporaryDirectory(prefix="trackfw-branch-prune-py-") as work:
        bare_dir = os.path.join(work, "origin.git")
        clone_dir = os.path.join(work, "clone")
        empty_gitconfig = os.path.join(work, "empty-gitconfig")
        with open(empty_gitconfig, "w") as f:
            f.write("")

        env = dict(os.environ)
        env.update({
            "GIT_CONFIG_GLOBAL": empty_gitconfig,
            "GIT_CONFIG_SYSTEM": "/dev/null",
            "GIT_TERMINAL_PROMPT": "0",
            "HOME": work,
        })

        def run(cwd, args):
            result = subprocess.run(
                ["git"] + args, cwd=cwd, env=env, capture_output=True, text=True
            )
            if result.returncode != 0:
                raise AssertionError(
                    f"git {args} (cwd={cwd}) failed: {result.stderr}\n{result.stdout}"
                )
            return result.stdout

        os.makedirs(bare_dir, exist_ok=True)
        run(bare_dir, ["init", "-q", "--bare", "-b", "main"])

        os.makedirs(clone_dir, exist_ok=True)
        run(work, ["clone", "-q", bare_dir, clone_dir])
        run(clone_dir, ["config", "user.email", "falsify@trackfw.test"])
        run(clone_dir, ["config", "user.name", "trackfw falsify"])
        run(clone_dir, ["config", "commit.gpgsign", "false"])
        run(clone_dir, ["config", "core.hooksPath", "/dev/null"])

        def write_file(name, content):
            with open(os.path.join(clone_dir, name), "w") as f:
                f.write(content)

        write_file("base.txt", "base\n")
        run(clone_dir, ["add", "base.txt"])
        run(clone_dir, ["commit", "-q", "-m", "base commit"])
        run(clone_dir, ["push", "-q", "origin", "main"])

        # Branch A: toca a.txt, squash-mergeada na main primeiro.
        run(clone_dir, ["checkout", "-q", "-b", "feat/a"])
        write_file("a.txt", "a\n")
        run(clone_dir, ["add", "a.txt"])
        run(clone_dir, ["commit", "-q", "-m", "feat/a work"])
        run(clone_dir, ["checkout", "-q", "main"])
        run(clone_dir, ["merge", "-q", "--squash", "feat/a"])
        run(clone_dir, ["commit", "-q", "-m", "squash-merge feat/a"])

        # Branch B: toca b.txt, criada depois do squash-merge de feat/a, também
        # squash-mergeada — a main avança mais, deixando feat/a para trás mas
        # ainda integrada.
        run(clone_dir, ["checkout", "-q", "-b", "feat/b"])
        write_file("b.txt", "b\n")
        run(clone_dir, ["add", "b.txt"])
        run(clone_dir, ["commit", "-q", "-m", "feat/b work"])
        run(clone_dir, ["checkout", "-q", "main"])
        run(clone_dir, ["merge", "-q", "--squash", "feat/b"])
        run(clone_dir, ["commit", "-q", "-m", "squash-merge feat/b"])

        run(clone_dir, ["push", "-q", "origin", "main"])
        run(clone_dir, ["fetch", "-q", "origin"])

        # Branch genuinamente pendente: toca c.txt, nunca mergeada.
        run(clone_dir, ["checkout", "-q", "-b", "feat/pending"])
        write_file("c.txt", "c\n")
        run(clone_dir, ["add", "c.txt"])
        run(clone_dir, ["commit", "-q", "-m", "feat/pending work, never merged"])

        def exec_git(args):
            result = subprocess.run(
                ["git"] + args, cwd=clone_dir, env=env, capture_output=True, text=True
            )
            if result.returncode != 0:
                return ("", result.stderr.strip() or f"git {' '.join(args)} failed")
            return (result.stdout.strip(), None)

        # Sanidade: o diff bidirecional ingênuo é NÃO-vazio para feat/a — prova
        # que este teste realmente discrimina entre o check ingênuo e a
        # heurística (AC2), não passa vacuamente.
        naive_out, naive_err = exec_git(["diff", "origin/main", "feat/a", "--stat"])
        assert naive_err is None
        assert naive_out.strip() != "", (
            "fixture inválida: diff ingênuo deve ser não-vazio para discriminar (AC2)"
        )

        eval_a = evaluate_branch_integration("feat/a", exec_git)
        assert eval_a["decision"] == BRANCH_PRUNE_DECISION_IDENTICAL, eval_a
        assert branch_prune_is_deletable(eval_a["decision"])

        eval_pending = evaluate_branch_integration("feat/pending", exec_git)
        assert eval_pending["decision"] == BRANCH_PRUNE_DECISION_PENDING_WORK
        assert not branch_prune_is_deletable(eval_pending["decision"])

        # AC1 — squash-merge sem ancestralidade: `git branch -d` recusaria feat/a.
        d_result = subprocess.run(
            ["git", "-C", clone_dir, "branch", "-d", "feat/a"], env=env, capture_output=True
        )
        assert d_result.returncode != 0, (
            "fixture inválida: git branch -d teve sucesso inesperado numa branch squash-mergeada"
        )

        # run_branch_prune completo, ponta a ponta, contra o repo real.
        deleted_via_delete_branch = []
        out_buf = io.StringIO()

        def delete_branch(g, name):
            deleted_via_delete_branch.append(name)
            _, err = g(["branch", "-D", name])
            return err

        exit_code = run_branch_prune(
            apply=True,
            exec_git=exec_git,
            list_local_branches=_default_list_local_branches,
            current_branch=lambda g: (lambda r: r[0].strip() if r[1] is None else "")(
                g(["symbolic-ref", "--quiet", "--short", "HEAD"])
            ),
            worktree_branches=lambda g: _worktree_branches_from(g),
            delete_branch=delete_branch,
            out=out_buf,
        )

        assert exit_code == 0
        assert sorted(deleted_via_delete_branch) == ["feat/a", "feat/b"]

        remaining, _ = _default_list_local_branches(exec_git)
        remaining = sorted(remaining)
        assert "feat/a" not in remaining
        assert "feat/pending" in remaining


def _worktree_branches_from(exec_git):
    raw, err = exec_git(["worktree", "list", "--porcelain"])
    result = set()
    if err is not None:
        return result
    prefix = "branch refs/heads/"
    for line in raw.split("\n"):
        t = line.strip()
        if t.startswith(prefix):
            result.add(t[len(prefix):])
    return result
