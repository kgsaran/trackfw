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
    BRANCH_PRUNE_DECISION_REVIEW_DOC_CONFIG,
    branch_prune_is_deletable,
    split_nul_paths,
    evaluate_branch_integration,
    _default_list_local_branches,
    _default_delete_branch,
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
    # f1.md — deliberadamente um arquivo doc, para provar que pending_work é decidido por
    # "diverg == touched" (nada da branch chegou na main), não pelo tipo do arquivo. O ML-1C
    # corrigiu o bug em que uma branch só-de-doc com diverg == touched era roteada erradamente
    # para review_doc_config ("probable housekeeping, confirm and delete manually") mesmo nunca
    # tendo sido integrada. O teste abaixo é o caso contrastante: mesmos tipos de arquivo, mas
    # diverg é subconjunto PRÓPRIO de touched (integração parcial) — é isso que de fato torna a
    # branch review_doc_config.
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/pending": ("abc123", None),
        "diff --name-only -z abc123 feat/pending": ("f1.md\x00", None),
        "diff --name-only -z origin/main feat/pending -- f1.md": ("f1.md\x00", None),
    })
    ev = evaluate_branch_integration("feat/pending", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_PENDING_WORK
    assert not branch_prune_is_deletable(ev["decision"])
    assert "f1.md" in ev["reason"]


def test_evaluate_branch_integration_review_doc_config_not_deletable():
    # review_doc_config exige que diverg seja subconjunto PRÓPRIO de touched — integração
    # parcial genuína (README-merged.md chegou na main, CLAUDE.md/trackfw.yaml são resíduo),
    # não uma branch que nunca foi integrada.
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/docs-only": ("abc123", None),
        "diff --name-only -z abc123 feat/docs-only": (
            "CLAUDE.md\x00README-merged.md\x00trackfw.yaml\x00",
            None,
        ),
        "diff --name-only -z origin/main feat/docs-only -- CLAUDE.md README-merged.md trackfw.yaml": (
            "CLAUDE.md\x00trackfw.yaml\x00",
            None,
        ),
    })
    ev = evaluate_branch_integration("feat/docs-only", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_REVIEW_DOC_CONFIG
    assert not branch_prune_is_deletable(ev["decision"]), (
        "review_doc_config nunca deve ser apagável — instrução explícita do KG"
    )
    assert "CLAUDE.md" in ev["reason"]
    assert "trackfw.yaml" in ev["reason"]
    assert "confirm and delete manually" in ev["reason"]


def test_evaluate_branch_integration_doc_only_never_integrated_is_pending_work():
    # Repro exato do KG: branch com documentação nova, nunca mergeada. diverg == touched (nada
    # chegou na main), então isso deve ser pending_work mesmo sendo tudo doc/config.
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/doc-real": ("abc123", None),
        "diff --name-only -z abc123 feat/doc-real": ("docs/guia-novo.md\x00", None),
        "diff --name-only -z origin/main feat/doc-real -- docs/guia-novo.md": (
            "docs/guia-novo.md\x00",
            None,
        ),
    })
    ev = evaluate_branch_integration("feat/doc-real", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_PENDING_WORK
    assert not branch_prune_is_deletable(ev["decision"])
    assert "housekeeping" not in ev["reason"], (
        f"não deve sugerir housekeeping para trabalho nunca integrado: {ev['reason']!r}"
    )


def test_evaluate_branch_integration_mixed_doc_and_code_stays_pending_work():
    exec_git = _fake_exec_git({
        "merge-base origin/main feat/mixed": ("abc123", None),
        "diff --name-only -z abc123 feat/mixed": ("README.md\x00main.py\x00", None),
        "diff --name-only -z origin/main feat/mixed -- README.md main.py": (
            "README.md\x00main.py\x00",
            None,
        ),
    })
    ev = evaluate_branch_integration("feat/mixed", exec_git)
    assert ev["decision"] == BRANCH_PRUNE_DECISION_PENDING_WORK
    assert not branch_prune_is_deletable(ev["decision"])


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
            "fetch origin --prune": ("", None),
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


def test_run_branch_prune_fetch_fails_warns_but_still_evaluates():
    out = io.StringIO()
    fetch_called = {"v": False}

    def exec_git(args):
        key = " ".join(args)
        if key == "fetch origin --prune":
            fetch_called["v"] = True
            return ("", "fatal: unable to access origin (simulated offline)")
        table = {
            "rev-parse --verify -q origin/main": ("abc123", None),  # already resolved before
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

    def delete_branch(_exec_git, _name):
        raise AssertionError("delete_branch must not be called in dry-run tests")

    exit_code = run_branch_prune(
        apply=False,
        exec_git=exec_git,
        list_local_branches=list_local_branches,
        current_branch=lambda _g: "fix/current",
        worktree_branches=lambda _g: {"chore/wt"},
        delete_branch=delete_branch,
        out=out,
    )
    assert exit_code == 0, "fetch failure must not abort the command"
    assert fetch_called["v"]
    got = out.getvalue()
    assert "warning" in got.lower() and "fetch" in got, got
    assert "would delete" in got and "feat/integrated" in got, got


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


# ────────────────────────────────────────────────────────────────────────────
# _default_delete_branch — -d tentado antes de -D, repositório git real (ambos os caminhos).
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.skipif(not _git_available(), reason="git not available in PATH")
def test_default_delete_branch_tries_dash_d_before_dash_capital_d():
    with tempfile.TemporaryDirectory(prefix="trackfw-branch-prune-dd-py-") as work:
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
            result = subprocess.run(["git"] + args, cwd=cwd, env=env, capture_output=True, text=True)
            if result.returncode != 0:
                raise AssertionError(f"git {args} (cwd={cwd}) failed: {result.stderr}\n{result.stdout}")
            return result.stdout

        repo = os.path.join(work, "repo")
        os.makedirs(repo, exist_ok=True)
        run(repo, ["init", "-q", "-b", "main"])
        run(repo, ["config", "user.email", "falsify@trackfw.test"])
        run(repo, ["config", "user.name", "trackfw falsify"])
        run(repo, ["config", "commit.gpgsign", "false"])
        with open(os.path.join(repo, "base.txt"), "w") as f:
            f.write("base\n")
        run(repo, ["add", "base.txt"])
        run(repo, ["commit", "-q", "-m", "base"])

        def exec_git(args):
            result = subprocess.run(["git"] + args, cwd=repo, env=env, capture_output=True, text=True)
            if result.returncode != 0:
                return ("", result.stderr.strip() or f"git {' '.join(args)} failed")
            return (result.stdout.strip(), None)

        # Caminho 1: feat/ff tem ancestralidade fast-forward com main (merge simples, sem
        # squash) — -d sozinho deve ter sucesso.
        run(repo, ["checkout", "-q", "-b", "feat/ff"])
        with open(os.path.join(repo, "ff.txt"), "w") as f:
            f.write("ff\n")
        run(repo, ["add", "ff.txt"])
        run(repo, ["commit", "-q", "-m", "feat/ff work"])
        run(repo, ["checkout", "-q", "main"])
        run(repo, ["merge", "-q", "--no-ff", "feat/ff"])

        err1 = _default_delete_branch(exec_git, "feat/ff")
        assert err1 is None, f"expected _default_delete_branch to succeed via plain -d, got: {err1}"
        remaining, _ = _default_list_local_branches(exec_git)
        assert "feat/ff" not in remaining

        # Caminho 2: feat/squash não tem ancestralidade com main (squash-merge) — -d puro
        # recusa; _default_delete_branch deve cair para -D e ainda ter sucesso.
        run(repo, ["checkout", "-q", "-b", "feat/squash"])
        with open(os.path.join(repo, "squash.txt"), "w") as f:
            f.write("squash\n")
        run(repo, ["add", "squash.txt"])
        run(repo, ["commit", "-q", "-m", "feat/squash work"])
        run(repo, ["checkout", "-q", "main"])
        run(repo, ["merge", "-q", "--squash", "feat/squash"])
        run(repo, ["commit", "-q", "-m", "squash-merge feat/squash"])

        d_check = subprocess.run(
            ["git", "-C", repo, "branch", "-d", "feat/squash"], env=env, capture_output=True
        )
        assert d_check.returncode != 0, (
            "fixture inválida: git branch -d teve sucesso inesperado numa branch squash-mergeada"
        )

        err2 = _default_delete_branch(exec_git, "feat/squash")
        assert err2 is None, f"expected _default_delete_branch to fall back to -D and succeed, got: {err2}"
        remaining, _ = _default_list_local_branches(exec_git)
        assert "feat/squash" not in remaining


# ────────────────────────────────────────────────────────────────────────────
# origin/main defasado — repositório git real. Prova "origin/main defasado leva a mais recusas,
# nunca a deleção indevida": quando `git fetch origin --prune` falha (simulado quebrando a URL
# do remoto), a avaliação continua usando qualquer ref origin/main já resolvível localmente. Uma
# branch de fato integrada upstream, mas invisível para esse ref defasado, é reportada MANTIDA
# (pending_work) — nunca oferecida para deleção indevidamente.
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.skipif(not _git_available(), reason="git not available in PATH")
def test_run_branch_prune_stale_origin_main_is_conservative_not_wrong():
    with tempfile.TemporaryDirectory(prefix="trackfw-branch-prune-stale-py-") as work:
        bare_dir = os.path.join(work, "origin.git")
        clone_dir = os.path.join(work, "clone")
        other_clone_dir = os.path.join(work, "other-clone")
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
            result = subprocess.run(["git"] + args, cwd=cwd, env=env, capture_output=True, text=True)
            if result.returncode != 0:
                raise AssertionError(f"git {args} (cwd={cwd}) failed: {result.stderr}\n{result.stdout}")
            return result.stdout

        os.makedirs(bare_dir, exist_ok=True)
        run(bare_dir, ["init", "-q", "--bare", "-b", "main"])

        os.makedirs(clone_dir, exist_ok=True)
        run(work, ["clone", "-q", bare_dir, clone_dir])
        run(clone_dir, ["config", "user.email", "falsify@trackfw.test"])
        run(clone_dir, ["config", "user.name", "trackfw falsify"])
        run(clone_dir, ["config", "commit.gpgsign", "false"])
        run(clone_dir, ["config", "core.hooksPath", "/dev/null"])

        with open(os.path.join(clone_dir, "base.txt"), "w") as f:
            f.write("base\n")
        run(clone_dir, ["add", "base.txt"])
        run(clone_dir, ["commit", "-q", "-m", "base commit"])
        run(clone_dir, ["push", "-q", "origin", "main"])

        run(clone_dir, ["checkout", "-q", "-b", "feat/mine"])
        with open(os.path.join(clone_dir, "mine.txt"), "w") as f:
            f.write("mine v1\n")
        run(clone_dir, ["add", "mine.txt"])
        run(clone_dir, ["commit", "-q", "-m", "feat/mine work"])
        run(clone_dir, ["checkout", "-q", "main"])
        # origin/main deste clone fica congelado em "base commit" — nunca mais é atualizado.

        os.makedirs(other_clone_dir, exist_ok=True)
        run(work, ["clone", "-q", bare_dir, other_clone_dir])
        run(other_clone_dir, ["config", "user.email", "falsify@trackfw.test"])
        run(other_clone_dir, ["config", "user.name", "trackfw falsify"])
        run(other_clone_dir, ["config", "commit.gpgsign", "false"])
        with open(os.path.join(other_clone_dir, "mine.txt"), "w") as f:
            f.write("mine v1\n")
        run(other_clone_dir, ["add", "mine.txt"])
        run(other_clone_dir, ["commit", "-q", "-m", "someone else lands the same content upstream"])
        run(other_clone_dir, ["push", "-q", "origin", "main"])

        # Quebra a URL do remoto para que `git fetch origin --prune` falhe deterministicamente.
        run(clone_dir, ["remote", "set-url", "origin", os.path.join(work, "does-not-exist.git")])

        def exec_git(args):
            result = subprocess.run(["git"] + args, cwd=clone_dir, env=env, capture_output=True, text=True)
            if result.returncode != 0:
                return ("", result.stderr.strip() or f"git {' '.join(args)} failed")
            return (result.stdout.strip(), None)

        out_buf = io.StringIO()

        def delete_branch(_g, _name):
            raise AssertionError("dry-run must never call delete_branch")

        exit_code = run_branch_prune(
            apply=False,
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
        got = out_buf.getvalue()
        assert "warning" in got.lower(), f"expected fetch-failure warning: {got}"
        for line in got.split("\n"):
            if line.strip().startswith("feat/mine "):
                assert "delete" not in line, (
                    f"stale origin/main must never make feat/mine look deletable, got line: {line!r}"
                )
                assert "keep" in line, f"expected feat/mine reported keep, got line: {line!r}"

        # Contraste: com um fetch funcionando, a mesma branch se torna apagável.
        run(clone_dir, ["remote", "set-url", "origin", bare_dir])
        run(clone_dir, ["fetch", "-q", "origin"])
        eval_after_fetch = evaluate_branch_integration("feat/mine", exec_git)
        assert eval_after_fetch["decision"] == BRANCH_PRUNE_DECISION_IDENTICAL, eval_after_fetch
        assert branch_prune_is_deletable(eval_after_fetch["decision"])


# ────────────────────────────────────────────────────────────────────────────
# ML-1C discriminant, repo git real: doc só-nunca-integrada vs resíduo parcial de doc, lado a
# lado (espelha Go TestEvaluateBranchIntegration_RealGitRepo_DocOnlyNeverIntegratedVsPartialResidue).
# ────────────────────────────────────────────────────────────────────────────

@pytest.mark.skipif(not _git_available(), reason="git not available in PATH")
def test_evaluate_branch_integration_real_git_repo_doc_only_never_integrated_vs_partial_residue():
    with tempfile.TemporaryDirectory(prefix="trackfw-branch-prune-py-ml1c-") as work:
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
            full = os.path.join(clone_dir, name)
            os.makedirs(os.path.dirname(full), exist_ok=True)
            with open(full, "w") as f:
                f.write(content)

        write_file("base.txt", "base\n")
        run(clone_dir, ["add", "base.txt"])
        run(clone_dir, ["commit", "-q", "-m", "base commit"])
        run(clone_dir, ["push", "-q", "origin", "main"])

        # feat/residue: toca app.py (código) e docs/notas.md (doc). Squash-mergeada, mas só o
        # conteúdo de app.py é levado no commit de squash-merge — docs/notas.md nunca chega na
        # main, o resíduo que este discriminante mira.
        run(clone_dir, ["checkout", "-q", "-b", "feat/residue"])
        write_file("app.py", "def main():\n    pass\n")
        write_file("docs/notas.md", "notas da branch\n")
        run(clone_dir, ["add", "app.py", "docs/notas.md"])
        run(clone_dir, ["commit", "-q", "-m", "feat/residue work: code + doc"])
        run(clone_dir, ["checkout", "-q", "main"])
        write_file("app.py", "def main():\n    pass\n")
        run(clone_dir, ["add", "app.py"])
        run(clone_dir, ["commit", "-q", "-m", "squash-merge feat/residue (code only, doc left out)"])
        run(clone_dir, ["push", "-q", "origin", "main"])

        # feat/doc-real: documentação nova, criada a partir da main atual, nunca mergeada.
        run(clone_dir, ["checkout", "-q", "-b", "feat/doc-real"])
        write_file("docs/guia-novo.md", "guia novo, nunca mergeado\n")
        run(clone_dir, ["add", "docs/guia-novo.md"])
        run(clone_dir, ["commit", "-q", "-m", "feat/doc-real: never-merged documentation"])
        run(clone_dir, ["checkout", "-q", "main"])
        run(clone_dir, ["fetch", "-q", "origin"])

        def exec_git(args):
            result = subprocess.run(
                ["git"] + args, cwd=clone_dir, env=env, capture_output=True, text=True
            )
            if result.returncode != 0:
                return ("", result.stderr.strip() or f"git {' '.join(args)} failed")
            return (result.stdout.strip(), None)

        eval_doc_real = evaluate_branch_integration("feat/doc-real", exec_git)
        assert eval_doc_real["decision"] == BRANCH_PRUNE_DECISION_PENDING_WORK, eval_doc_real
        assert not branch_prune_is_deletable(eval_doc_real["decision"])
        assert "housekeeping" not in eval_doc_real["reason"], (
            f"feat/doc-real não deve ser sugerida como housekeeping: {eval_doc_real['reason']!r}"
        )
        assert len(eval_doc_real["touched"]) == len(eval_doc_real["diverged"]), (
            f"feat/doc-real: esperado touched == diverg, got {eval_doc_real}"
        )

        eval_residue = evaluate_branch_integration("feat/residue", exec_git)
        assert eval_residue["decision"] == BRANCH_PRUNE_DECISION_REVIEW_DOC_CONFIG, eval_residue
        assert not branch_prune_is_deletable(eval_residue["decision"])
        assert "confirm and delete manually" in eval_residue["reason"]
        assert len(eval_residue["diverged"]) < len(eval_residue["touched"]), (
            f"feat/residue: esperado diverg subconjunto PRÓPRIO de touched, got {eval_residue}"
        )
