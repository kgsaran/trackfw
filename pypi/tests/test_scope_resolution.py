"""Tests for selectable install scope (ML-1C), parity port of the Go/Node
CLIs' `resolveScope`/`resolve_scope` gate
(ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills).

Covers the 4 mandatory scenarios from the roadmap ML-1C brief:
  1. `--scope project` explicit does not trigger the prompt and writes
     under `.claude/` (project scope).
  2. No TTY and no `--scope` writes under `~/.claude/` (global default,
     D1 — the breaking change from the old silent "project" default).
  3. `--targets claude` with no `--scope`, simulated TTY, triggers the
     scope resolver — verified with a spy standing in for the real prompt,
     never invoking the real blocking `input()`.
  4. `list` with no `--scope` reports global destinations without
     prompting (D6 — read-only command never blocks on a prompt).

Also covers `resolve_scope` as a pure function (flag-set detection,
D3's "Armadilha crítica": comparing against a sentinel value instead of
`is not None` would make an explicit `--scope project` indistinguishable
from the default) and `trackfw init`'s scope resolution (D4).
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

from trackfw import identity
from trackfw.integrations import command as integrations_command

PYPI_ROOT = Path(__file__).parents[1]


def cli(*arguments: str, cwd: Path, home: Path | None = None):
    environment = dict(os.environ)
    environment["PYTHONPATH"] = str(PYPI_ROOT)
    if home:
        environment["HOME"] = str(home)
    return subprocess.run(
        [sys.executable, "-m", "trackfw", *arguments],
        cwd=cwd,
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )


# ---------------------------------------------------------------------------
# resolve_scope — pure function, flag-set detection (D3)
# ---------------------------------------------------------------------------


def test_resolve_scope_explicit_project_is_returned_as_is(monkeypatch):
    # An explicit --scope must win even if stdin happens to be a TTY — the
    # detection is by flag-set (`scope is not None`), never by comparing
    # against a sentinel value.
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    called = {"prompted": False}

    def _spy() -> str:
        called["prompted"] = True
        return "global"

    monkeypatch.setattr(integrations_command, "scope_prompt_runner", _spy)
    assert integrations_command.resolve_scope("project") == "project"
    assert called["prompted"] is False


def test_resolve_scope_explicit_global_is_returned_as_is(monkeypatch):
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    assert integrations_command.resolve_scope("global") == "global"


def test_resolve_scope_no_tty_and_no_flag_defaults_to_global(monkeypatch):
    monkeypatch.setattr("sys.stdin.isatty", lambda: False)
    assert integrations_command.resolve_scope(None) == "global"


def test_resolve_scope_tty_and_no_flag_invokes_the_prompt_runner(monkeypatch):
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.setattr(integrations_command, "scope_prompt_runner", lambda: "project")
    assert integrations_command.resolve_scope(None) == "project"


def test_resolve_scope_no_tty_uninstall_and_no_flag_raises(monkeypatch):
    # ADR D8: uninstall never inherits install/update's "global" default in
    # non-interactive mode — errs loudly instead of risking deletion of
    # files outside the scope the caller intended.
    monkeypatch.setattr("sys.stdin.isatty", lambda: False)
    try:
        integrations_command.resolve_scope(None, operation="uninstall")
    except ValueError as error:
        assert "uninstall requires --scope in non-interactive mode" in str(error)
    else:
        raise AssertionError("expected resolve_scope to raise for non-interactive uninstall")


def test_resolve_scope_no_tty_install_and_no_flag_still_defaults_to_global(monkeypatch):
    # install/update are unaffected by D8 — only uninstall's operation
    # value triggers the raise above.
    monkeypatch.setattr("sys.stdin.isatty", lambda: False)
    assert integrations_command.resolve_scope(None, operation="install") == "global"
    assert integrations_command.resolve_scope(None, operation="update") == "global"


def test_resolve_scope_tty_uninstall_and_no_flag_invokes_the_prompt_runner(monkeypatch):
    # In TTY, uninstall prompts exactly like install/update (ADR D8's
    # non-interactive guard does not apply once the user can see the
    # choice before anything destructive happens).
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.setattr(integrations_command, "scope_prompt_runner", lambda: "project")
    assert integrations_command.resolve_scope(None, operation="uninstall") == "project"


def test_prompt_scope_defaults_to_global_on_bare_enter(monkeypatch):
    # Real prompt behavior (D2): global is index [1] and pre-selected, so a
    # bare Enter (empty input) must resolve to "global" without needing to
    # type "1".
    monkeypatch.setattr("builtins.input", lambda _prompt="": "")
    assert integrations_command._prompt_scope() == "global"


def test_prompt_scope_selects_project_by_index(monkeypatch):
    monkeypatch.setattr("builtins.input", lambda _prompt="": "2")
    assert integrations_command._prompt_scope() == "project"


# ---------------------------------------------------------------------------
# Case 1 — `--scope project` explicit does not prompt, writes to `.claude/`
# ---------------------------------------------------------------------------


def test_scope_project_explicit_does_not_prompt_and_writes_project_scope(tmp_path):
    result = cli(
        "agents", "install", "--targets", "claude", "--items", "backend",
        "--scope", "project", "--json", cwd=tmp_path,
    )
    assert result.returncode == 0, result.stderr
    assert (tmp_path / ".claude/agents/trackfw-backend.md").is_file()
    payload = json.loads(result.stdout)
    assert payload["deployments"][0]["scope"] == "project"


# ---------------------------------------------------------------------------
# Case 2 — no TTY, no --scope → writes to ~/.claude/ (global default, D1)
# ---------------------------------------------------------------------------


def test_no_tty_and_no_scope_writes_global_scope(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    # subprocess.run's captured pipes are never a TTY, so this exercises the
    # non-interactive branch of resolve_scope without any extra stubbing.
    result = cli(
        "agents", "install", "--targets", "claude", "--items", "backend",
        "--json", cwd=tmp_path, home=home,
    )
    assert result.returncode == 0, result.stderr
    assert (home / ".claude/agents/trackfw-backend.md").is_file()
    assert not (tmp_path / ".claude").exists()
    payload = json.loads(result.stdout)
    assert payload["deployments"][0]["scope"] == "global"


# ---------------------------------------------------------------------------
# Case 3 — `--targets claude` without --scope, simulated TTY, triggers the
# scope resolver (gate independent of --targets). A spy replaces the real
# prompt so the test never blocks on the real input().
# ---------------------------------------------------------------------------


def _make_args(**overrides):
    defaults = dict(
        targets="claude",
        items="backend",
        scope=None,
        surface=None,
        json=True,
        force=False,
        identity=False,
        identity_preset=None,
        action="install",
    )
    defaults.update(overrides)
    return argparse.Namespace(**defaults)


def test_targets_flag_with_tty_and_no_scope_still_triggers_scope_resolver(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    # An identity already on disk means the (unrelated) agents-install
    # identity wizard does not also try to prompt on this TTY — otherwise
    # it would block on a second, real input() and this test would hang
    # under pytest's captured stdin.
    identity.save(home, identity.preset("greek"))

    prompt_calls = {"count": 0}

    def _spy() -> str:
        prompt_calls["count"] += 1
        return "project"

    monkeypatch.setattr(integrations_command, "scope_prompt_runner", _spy)
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.chdir(project)

    args = _make_args()
    integrations_command.run(args, "agents")

    # The gate must fire even though --targets was already provided — it is
    # independent of target/item selection (ADR D2).
    assert prompt_calls["count"] == 1
    assert (project / ".claude/agents/trackfw-backend.md").is_file()


# ---------------------------------------------------------------------------
# Case 4 — `list` without --scope reports global destinations, never prompts
# (D6)
# ---------------------------------------------------------------------------


def test_list_without_scope_reports_global_destinations_without_prompting(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    def _fail_if_called() -> str:
        raise AssertionError("list must never invoke the interactive scope prompt (D6)")

    monkeypatch.setattr(integrations_command, "scope_prompt_runner", _fail_if_called)
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.chdir(project)

    args = _make_args(action="list", targets="claude", items="backend")
    integrations_command.run(args, "agents")


def test_list_without_scope_reports_global_destinations_via_cli(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    result = cli(
        "agents", "list", "--targets", "claude", "--items", "backend",
        "--json", cwd=tmp_path, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["deployments"][0]["scope"] == "global"
    # The catalog's raw destination template uses `~/` (resolved against
    # home_dir only at write time by IntegrationManager); asserting the
    # scope is "global" here is what proves list adopted the D6 default
    # without prompting.
    assert payload["deployments"][0]["destination"].startswith("~/.claude")


# ---------------------------------------------------------------------------
# `trackfw init` (D4): no --scope flag exists; resolves via the same
# resolve_scope(None) path — no TTY -> "global".
# ---------------------------------------------------------------------------


def test_init_ai_tools_no_tty_defaults_to_global_scope(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    result = cli(
        "init", "--project-name", "example", "--ai-tools", "claude",
        cwd=tmp_path, home=home,
    )
    assert result.returncode == 0, result.stderr
    assert (home / ".claude/agents/trackfw-backend.md").is_file()
    assert not (tmp_path / ".claude/agents").exists()
