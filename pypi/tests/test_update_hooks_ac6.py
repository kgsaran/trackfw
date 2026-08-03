"""AC6 (REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis):
`trackfw update` (Python CLI) now reads hooks/ci/backend/frontend/pkg_manager via the single
config loader and acts on `hooks` — closing a functional gap that existed before this change
(`grep -rn pkg_manager pypi/trackfw` returned nothing prior to ML-2A; this runtime's `update`
never read any of the 5 fields).

Every test spawns the real CLI as a subprocess (mirrors test_update_harness.py) so each case
gets a fresh config singleton — no in-process config.reset() needed.
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

PYPI_ROOT = Path(__file__).parents[1]


def cli(*arguments: str, cwd: Path, home: Path):
    environment = dict(os.environ)
    environment["PYTHONPATH"] = str(PYPI_ROOT)
    environment["HOME"] = str(home)
    return subprocess.run(
        [sys.executable, "-m", "trackfw", *arguments],
        cwd=cwd,
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )


def test_update_injects_trackfw_validate_into_existing_husky_hook(tmp_path):
    """Proof of the AC6 gap being closed: before this change, `_update_hooks_surgical` did not
    exist and `hooks` was never read by update.py's `_run` — this exact scenario would have left
    .husky/pre-commit completely untouched. With hooks: husky and an existing hook missing the
    trackfw validate line, the same effect Go's updateHooksSurgical and Node's
    updateHooksSurgical produce (surgical injection, no overwrite of user content) is now
    reproduced here.
    """
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    (project / "trackfw.yaml").write_text("hooks: husky\nci: none\n", encoding="utf-8")

    husky_dir = project / ".husky"
    husky_dir.mkdir()
    hook_path = husky_dir / "pre-commit"
    hook_path.write_text("#!/usr/bin/env sh\necho existing user hook\n", encoding="utf-8")

    assert "trackfw validate" not in hook_path.read_text(encoding="utf-8")

    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    content = hook_path.read_text(encoding="utf-8")
    assert content.count("trackfw validate") == 1
    assert "echo existing user hook" in content  # user content preserved, not overwritten

    # Idempotent re-run: must not duplicate the injected line (matches Go/Node behavior).
    result2 = cli("update", cwd=project, home=home)
    assert result2.returncode == 0, result2.stderr
    content2 = hook_path.read_text(encoding="utf-8")
    assert content2.count("trackfw validate") == 1


def test_update_injects_trackfw_validate_into_existing_lefthook_yml(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    (project / "trackfw.yaml").write_text("hooks: lefthook\nci: none\n", encoding="utf-8")
    lefthook_path = project / "lefthook.yml"
    lefthook_path.write_text("pre-commit:\n  commands:\n    lint:\n      run: eslint .\n", encoding="utf-8")

    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    content = lefthook_path.read_text(encoding="utf-8")
    assert "trackfw-validate:" in content
    assert "run: eslint ." in content  # user content preserved

    result2 = cli("update", cwd=project, home=home)
    assert result2.returncode == 0, result2.stderr
    content2 = lefthook_path.read_text(encoding="utf-8")
    assert content2.count("trackfw-validate:") == 1


def test_update_does_not_touch_hooks_when_hooks_is_none(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    (project / "trackfw.yaml").write_text("hooks: none\nci: none\n", encoding="utf-8")

    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    assert not (project / ".husky").exists()
    assert not (project / "lefthook.yml").exists()
