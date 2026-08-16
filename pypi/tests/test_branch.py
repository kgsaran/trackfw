"""
test_branch.py — Testes para trackfw.commands.branch (`trackfw branch new`).

Espelha internal/commands/branch_test.go — mesmos 8 cenários (match em wip/,
match em done/, sem match bloqueia sem chamar git, --dry-run nos dois casos,
tipo inválido, slug vazio, branch já existente delega ao erro nativo do git).
Usa injeção de dependência (mesmo padrão de test_ship.py / MockGit) — nenhum
teste toca um repositório git real.
"""

from __future__ import annotations

import io

from trackfw.commands.branch import parse_branch_spec, run_branch_new
from trackfw import validator as _validator


def _make_kwargs(matched: bool, candidates: list | None = None):
    """Builds run_branch_new kwargs wired to injectable fakes, plus references to the
    checkout_calls list and out/err buffers, so tests never touch a real git repository or the
    real project filesystem layout."""
    checkout_calls: list[str] = []
    out = io.StringIO()
    err_out = io.StringIO()

    def fake_checkout(branch_name):
        checkout_calls.append(branch_name)
        return 0

    kwargs = dict(
        load_config=lambda: {},
        resolve_wip_dirs=lambda cfg: ["docs/roadmaps/wip"],
        resolve_done_dirs=lambda cfg: ["docs/roadmaps/done"],
        match_slug=lambda slug, wip_dirs, done_dirs: (matched, candidates or []),
        exec_git_checkout=fake_checkout,
        out=out,
        err_out=err_out,
    )
    return kwargs, out, err_out, checkout_calls


# ────────────────────────────────────────────────────────────────────────────
# parse_branch_spec
# ────────────────────────────────────────────────────────────────────────────

def test_parse_branch_spec_valid_types():
    for typ in ("feat", "fix", "refactor", "chore", "docs"):
        branch_type, slug, err = parse_branch_spec(f"{typ}/my-slug")
        assert err is None
        assert branch_type == typ
        assert slug == "my-slug"


def test_parse_branch_spec_invalid_type():
    branch_type, slug, err = parse_branch_spec("banana/my-slug")
    assert branch_type is None and slug is None
    assert "invalid branch type" in err
    assert err == 'invalid branch type "banana" — must be one of feat, fix, refactor, chore, docs'


def test_parse_branch_spec_empty_slug():
    branch_type, slug, err = parse_branch_spec("feat/")
    assert branch_type is None and slug is None
    assert "slug is required" in err


def test_parse_branch_spec_no_slash():
    branch_type, slug, err = parse_branch_spec("feat-my-slug")
    assert branch_type is None and slug is None
    assert "invalid branch spec" in err


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — match found (wip/ or done/, no distinction at this layer
# since match_slug is injected — the real matching logic is covered by
# validator.branch_slug_matches_roadmap tests below).
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_match_found_checks_out_branch():
    kwargs, out, err_out, calls = _make_kwargs(True, None)
    exit_code = run_branch_new("feat/my-slug", False, **kwargs)
    assert exit_code == 0
    assert calls == ["feat/my-slug"]
    assert out.getvalue() == ""
    assert err_out.getvalue() == ""


def test_run_branch_new_match_found_wip_roadmap():
    # Simulates a match found via a roadmap in wip/.
    kwargs, _, _, calls = _make_kwargs(True, ["ROADMAP-my-slug.md"])
    exit_code = run_branch_new("fix/my-slug", False, **kwargs)
    assert exit_code == 0
    assert calls == ["fix/my-slug"]


def test_run_branch_new_match_found_done_roadmap():
    # Simulates a match found via a roadmap in done/ — match_slug does not distinguish the
    # source directory in its return value, mirroring validator.branch_slug_matches_roadmap.
    kwargs, _, _, calls = _make_kwargs(True, ["ROADMAP-my-slug.md"])
    exit_code = run_branch_new("refactor/my-slug", False, **kwargs)
    assert exit_code == 0
    assert calls == ["refactor/my-slug"]


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — no match: blocks, never calls git
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_no_match_no_candidates_blocks():
    kwargs, out, err_out, calls = _make_kwargs(False, None)
    exit_code = run_branch_new("feat/orphan-slug", False, **kwargs)
    assert exit_code != 0
    assert calls == []
    want = _validator.branch_governance_orientation("feat/orphan-slug")
    assert want in out.getvalue()


def test_run_branch_new_no_match_with_candidates_blocks():
    candidates = ["ROADMAP-other-thing.md"]
    kwargs, out, err_out, calls = _make_kwargs(False, candidates)
    exit_code = run_branch_new("fix/orphan-slug", False, **kwargs)
    assert exit_code != 0
    assert calls == []
    want = _validator.branch_no_matching_roadmap_message("fix/orphan-slug", candidates)
    assert want in out.getvalue()


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — --dry-run
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_dry_run_match_never_calls_git():
    kwargs, out, err_out, calls = _make_kwargs(True, None)
    exit_code = run_branch_new("feat/my-slug", True, **kwargs)
    assert exit_code == 0
    assert calls == []
    assert "dry-run" in out.getvalue() and "would create" in out.getvalue()


def test_run_branch_new_dry_run_no_match_never_calls_git():
    kwargs, out, err_out, calls = _make_kwargs(False, None)
    exit_code = run_branch_new("feat/orphan-slug", True, **kwargs)
    assert exit_code != 0
    assert calls == []
    assert "dry-run" in out.getvalue() and "would block" in out.getvalue()


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — invalid type / empty slug never reach match_slug or git
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_invalid_type_never_calls_match_or_git():
    match_called = {"value": False}
    kwargs, out, err_out, calls = _make_kwargs(True, None)

    def fake_match(slug, wip_dirs, done_dirs):
        match_called["value"] = True
        return True, []

    kwargs["match_slug"] = fake_match
    exit_code = run_branch_new("banana/my-slug", False, **kwargs)
    assert exit_code != 0
    assert match_called["value"] is False
    assert calls == []


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — chore/docs are housekeeping types: they create the branch without
# the branch_has_wip_roadmap gate, and never call match_slug.
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_chore_type_skips_gate_checks_out_branch():
    match_called = {"value": False}
    # matched=False: gate would block if consulted
    kwargs, out, err_out, calls = _make_kwargs(False, None)

    def fake_match(slug, wip_dirs, done_dirs):
        match_called["value"] = True
        return False, []

    kwargs["match_slug"] = fake_match
    exit_code = run_branch_new("chore/release-7.0.0", False, **kwargs)
    assert exit_code == 0
    assert match_called["value"] is False
    assert calls == ["chore/release-7.0.0"]
    assert out.getvalue() == ""


def test_run_branch_new_docs_type_skips_gate_checks_out_branch():
    match_called = {"value": False}
    kwargs, out, err_out, calls = _make_kwargs(False, None)

    def fake_match(slug, wip_dirs, done_dirs):
        match_called["value"] = True
        return False, []

    kwargs["match_slug"] = fake_match
    exit_code = run_branch_new("docs/atualiza-readme", False, **kwargs)
    assert exit_code == 0
    assert match_called["value"] is False
    assert calls == ["docs/atualiza-readme"]


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — non-regression: feat/fix/refactor without a matching roadmap must
# keep blocking with the same governance orientation message.
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_feat_without_roadmap_still_blocks_non_regression():
    kwargs, out, err_out, calls = _make_kwargs(False, None)
    exit_code = run_branch_new("feat/no-roadmap-for-this", False, **kwargs)
    assert exit_code != 0
    assert calls == []
    want = _validator.branch_governance_orientation("feat/no-roadmap-for-this")
    assert want in out.getvalue()


def test_run_branch_new_empty_slug_never_calls_match_or_git():
    match_called = {"value": False}
    kwargs, out, err_out, calls = _make_kwargs(True, None)

    def fake_match(slug, wip_dirs, done_dirs):
        match_called["value"] = True
        return True, []

    kwargs["match_slug"] = fake_match
    exit_code = run_branch_new("feat/", False, **kwargs)
    assert exit_code != 0
    assert match_called["value"] is False
    assert calls == []


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — branch already exists: delegate to Git's native error, no
# special handling
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_branch_already_exists_propagates_git_exit_code():
    kwargs, out, err_out, calls = _make_kwargs(True, None)

    def failing_checkout(branch_name):
        calls.append(branch_name)
        return 128

    kwargs["exec_git_checkout"] = failing_checkout
    exit_code = run_branch_new("feat/my-slug", False, **kwargs)
    assert exit_code == 128
    assert calls == ["feat/my-slug"]


# ────────────────────────────────────────────────────────────────────────────
# run_branch_new — uses the normalized slug for matching
# ────────────────────────────────────────────────────────────────────────────

def test_run_branch_new_uses_normalized_slug_for_matching():
    received = {}
    kwargs, out, err_out, calls = _make_kwargs(True, None)

    def fake_match(slug, wip_dirs, done_dirs):
        received["slug"] = slug
        return True, []

    kwargs["match_slug"] = fake_match
    exit_code = run_branch_new("feat/My_Weird--Slug", False, **kwargs)
    assert exit_code == 0
    assert received["slug"] == _validator.normalize_branch_slug("My_Weird--Slug")


# ────────────────────────────────────────────────────────────────────────────
# validator.branch_slug_matches_roadmap — matching real, contra o filesystem
# ────────────────────────────────────────────────────────────────────────────

def test_branch_slug_matches_roadmap_matches_in_wip(tmp_path):
    wip = tmp_path / "wip"
    wip.mkdir()
    (wip / "ROADMAP-2026-08-04-my-cool-feature.md").write_text("x", encoding="utf-8")
    matched, candidates = _validator.branch_slug_matches_roadmap(
        "my-cool-feature", [str(wip)], []
    )
    assert matched is True
    assert candidates == ["ROADMAP-2026-08-04-my-cool-feature.md"]


def test_branch_slug_matches_roadmap_matches_in_done(tmp_path):
    done = tmp_path / "done"
    done.mkdir()
    (done / "ROADMAP-2026-08-04-my-cool-feature.md").write_text("x", encoding="utf-8")
    matched, candidates = _validator.branch_slug_matches_roadmap(
        "my-cool-feature", [], [str(done)]
    )
    assert matched is True
    assert candidates == ["ROADMAP-2026-08-04-my-cool-feature.md"]


def test_branch_slug_matches_roadmap_no_match(tmp_path):
    wip = tmp_path / "wip"
    wip.mkdir()
    (wip / "ROADMAP-2026-08-04-something-else.md").write_text("x", encoding="utf-8")
    matched, candidates = _validator.branch_slug_matches_roadmap(
        "my-cool-feature", [str(wip)], []
    )
    assert matched is False
    assert candidates == ["ROADMAP-2026-08-04-something-else.md"]
