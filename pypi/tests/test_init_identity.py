"""Tests for `trackfw init --identity-preset` (ML-4B).

Covers: valid/invalid preset persistence, non-interactive/non-TTY safety
(must never block), and idempotency of a second `init` run when
~/.trackfw/identity.json already exists.
"""

from __future__ import annotations

import argparse
import json
import os

import pytest

from trackfw.commands import init as init_cmd
from trackfw import identity


def _parser():
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command")
    init_cmd.register(subparsers)
    return parser


def _run_init(argv, cwd, home, monkeypatch):
    monkeypatch.setattr(init_cmd, "_identity_home", lambda: str(home))
    monkeypatch.setattr(os, "getcwd", lambda: str(cwd))
    monkeypatch.chdir(cwd)
    parser = _parser()
    args = parser.parse_args(["init"] + argv)
    return init_cmd.run(args)


class TestIdentityPresetFlag:
    def test_valid_preset_persists_identity(self, tmp_path, monkeypatch):
        cwd = tmp_path / "project"
        cwd.mkdir()
        home = tmp_path / "home"
        home.mkdir()
        _run_init(["--identity-preset", "greek"], cwd, home, monkeypatch)

        identity_file = home / ".trackfw" / "identity.json"
        assert identity_file.is_file()
        data = json.loads(identity_file.read_text(encoding="utf-8"))
        assert data["agents"]["architect"]["display_name"] == "Zeus"
        assert data["agents"]["architect"]["slug"] == "zeus"

    def test_none_preset_does_not_create_identity_file(self, tmp_path, monkeypatch):
        cwd = tmp_path / "project"
        cwd.mkdir()
        home = tmp_path / "home"
        home.mkdir()
        _run_init(["--identity-preset", "none"], cwd, home, monkeypatch)
        assert not (home / ".trackfw" / "identity.json").exists()

    def test_neutral_preset_does_not_create_identity_file(self, tmp_path, monkeypatch):
        cwd = tmp_path / "project"
        cwd.mkdir()
        home = tmp_path / "home"
        home.mkdir()
        _run_init(["--identity-preset", "neutral"], cwd, home, monkeypatch)
        assert not (home / ".trackfw" / "identity.json").exists()

    def test_invalid_preset_exits_with_error(self, tmp_path, monkeypatch):
        cwd = tmp_path / "project"
        cwd.mkdir()
        home = tmp_path / "home"
        home.mkdir()
        with pytest.raises(SystemExit) as excinfo:
            _run_init(["--identity-preset", "does-not-exist"], cwd, home, monkeypatch)
        assert excinfo.value.code == 2
        assert not (home / ".trackfw" / "identity.json").exists()


class TestNonInteractiveNeverBlocks:
    def test_no_flag_and_no_tty_does_not_prompt(self, tmp_path, monkeypatch):
        cwd = tmp_path / "project"
        cwd.mkdir()
        home = tmp_path / "home"
        home.mkdir()
        monkeypatch.setattr("sys.stdin.isatty", lambda: False)
        # If this blocks on input(), the test suite hangs — proves the guard.
        _run_init([], cwd, home, monkeypatch)
        assert not (home / ".trackfw" / "identity.json").exists()


class TestReinitIsIdempotent:
    def test_rerun_without_flag_preserves_existing_identity(self, tmp_path, monkeypatch):
        cwd = tmp_path / "project"
        cwd.mkdir()
        home = tmp_path / "home"
        home.mkdir()
        cfg = identity.preset("norse")
        identity.save(home, cfg)

        monkeypatch.setattr("sys.stdin.isatty", lambda: True)
        _run_init([], cwd, home, monkeypatch)

        loaded = identity.load(home)
        assert loaded.agents["architect"].display_name == "Odin"
