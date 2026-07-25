"""Tests for the shared identity wizard (ML-2B), parity port of
internal/commands/identity_wizard.go
(ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install).

Covers: should_prompt_identity's full truth table, the agents install
trigger rule (D2), skills never invoking the wizard (D5), non-interactive
safety, confirmation decline never persisting, custom labels using
name+description (not the raw id), and --identity-preset error shape.
"""

from __future__ import annotations

import argparse
import json
import os

import pytest

from trackfw.commands import identity_wizard
from trackfw import identity
from trackfw.integrations import command as integrations_command
from trackfw.integrations.catalog import load_catalog


# ---------------------------------------------------------------------------
# should_prompt_identity — pure function, full truth table
# ---------------------------------------------------------------------------


@pytest.mark.parametrize("kind", ["agents", "skills"])
@pytest.mark.parametrize("is_tty", [True, False])
@pytest.mark.parametrize("identity_exists", [True, False])
@pytest.mark.parametrize("force_flag", [True, False])
def test_should_prompt_identity_truth_table(kind, is_tty, identity_exists, force_flag):
    expected = kind == "agents" and is_tty and (not identity_exists or force_flag)
    got = identity_wizard.should_prompt_identity(kind, is_tty, identity_exists, force_flag)
    assert got == expected


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_args(**overrides):
    defaults = dict(
        targets="claude",
        items="backend",
        scope="project",
        surface=None,
        json=False,
        force=False,
        identity=False,
        identity_preset=None,
        action="install",
    )
    defaults.update(overrides)
    return argparse.Namespace(**defaults)


class _Spy:
    """Records whether it was invoked, without running a real prompt."""

    def __init__(self, cfg=None, persisted=False):
        self.called = False
        self.calls = []
        self._cfg = cfg if cfg is not None else identity.Config()
        self._persisted = persisted

    def __call__(self, catalog, home):
        self.called = True
        self.calls.append((catalog, home))
        return self._cfg, self._persisted


# ---------------------------------------------------------------------------
# `agents install` wizard trigger (ADR D2)
# ---------------------------------------------------------------------------


def test_agents_install_with_existing_identity_and_no_flag_does_not_invoke(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    identity.save(home, identity.preset("greek"))

    spy = _Spy()
    monkeypatch.setattr(identity_wizard, "identity_wizard_runner", spy)
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.chdir(project)

    args = _make_args()
    integrations_command.run(args, "agents")

    assert spy.called is False
    destination = project / ".claude" / "agents" / "trackfw-backend.md"
    assert destination.is_file()
    assert "apolo-tf" in destination.read_text(encoding="utf-8")


def test_agents_install_without_identity_with_tty_invokes(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    spy = _Spy()
    monkeypatch.setattr(identity_wizard, "identity_wizard_runner", spy)
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.chdir(project)

    args = _make_args()
    integrations_command.run(args, "agents")

    assert spy.called is True


def test_agents_install_with_identity_flag_and_existing_identity_invokes(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    identity.save(home, identity.preset("greek"))

    spy = _Spy()
    monkeypatch.setattr(identity_wizard, "identity_wizard_runner", spy)
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.chdir(project)

    args = _make_args(identity=True)
    integrations_command.run(args, "agents")

    assert spy.called is True


def test_skills_install_never_invokes_wizard(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    spy = _Spy()
    monkeypatch.setattr(identity_wizard, "identity_wizard_runner", spy)
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.chdir(project)

    args = _make_args(items="implement")
    integrations_command.run(args, "skills")

    assert spy.called is False


def test_non_tty_never_blocks_and_still_requires_targets(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    spy = _Spy()
    monkeypatch.setattr(identity_wizard, "identity_wizard_runner", spy)
    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: False)
    monkeypatch.chdir(project)

    args = _make_args(targets=None)
    with pytest.raises(SystemExit) as excinfo:
        integrations_command.run(args, "agents")
    assert excinfo.value.code == 2
    assert spy.called is False


# ---------------------------------------------------------------------------
# The wizard itself: decline never persists, custom labels use name+description
# ---------------------------------------------------------------------------


def test_declining_confirmation_persists_nothing(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    catalog = load_catalog()

    # 1st loop: choose "greek" (index 1), nickname "", decline confirmation.
    # 2nd loop: choose "neutral" (last option) to exit cleanly.
    answers = iter(["1", "", "n", str(len(identity_wizard.IDENTITY_PRESET_LABELS))])
    monkeypatch.setattr("builtins.input", lambda *_args: next(answers))

    cfg, persisted = identity_wizard.run_identity_wizard(catalog, str(home))

    assert persisted is False
    assert cfg.agents == {}
    assert not (home / ".trackfw" / "identity.json").exists()


def test_custom_labels_use_name_and_description_not_raw_id(tmp_path, monkeypatch, capsys):
    home = tmp_path / "home"
    home.mkdir()
    catalog = load_catalog()
    known_ids = identity.known_agent_ids()

    # Choose "custom" (index 11), then one distinct name per known agent id,
    # then nickname "", then confirm "s".
    custom_choice = next(
        str(index) for index, (key, _) in enumerate(identity_wizard.IDENTITY_PRESET_LABELS, 1) if key == "custom"
    )
    names = [f"name-{agent_id}" for agent_id in known_ids]
    answers = iter([custom_choice, *names, "", "s"])
    monkeypatch.setattr("builtins.input", lambda *_args: next(answers))

    cfg, persisted = identity_wizard.run_identity_wizard(catalog, str(home))

    captured = capsys.readouterr()
    assert persisted is True
    assert "Architecture, ADRs and governed coordination" in captured.err
    assert "Architect —" in captured.err
    for agent_id in known_ids:
        assert cfg.agents[agent_id].display_name == f"name-{agent_id}"


# ---------------------------------------------------------------------------
# --identity-preset invalid value error shape
# ---------------------------------------------------------------------------


def test_invalid_identity_preset_lists_valid_values(tmp_path, monkeypatch):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    monkeypatch.setattr("os.path.expanduser", lambda p: str(home) if p == "~" else os.path.expanduser(p))
    monkeypatch.setattr("sys.stdin.isatty", lambda: True)
    monkeypatch.chdir(project)

    args = _make_args(identity_preset="xpto")
    with pytest.raises(SystemExit) as excinfo:
        integrations_command.run(args, "agents")
    assert excinfo.value.code == 2
    assert not (home / ".trackfw" / "identity.json").exists()
