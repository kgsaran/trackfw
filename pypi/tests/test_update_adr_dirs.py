"""ML-1C (REQ-2026-08-08-conectar-adrs-globais-aos-agentes-...): `trackfw update`
(Python CLI, project scope) registers ~/.trackfw/adr in trackfw.yaml's `adr_dirs`
list — but only when that directory exists on $HOME AND contains at least one
`ADR-*.md` file. The edit is surgical (text-level splice) and idempotent, mirroring
Go's ensureGlobalADRDirRegistered (internal/generators/update.go) and Node's
ensureGlobalAdrDirRegistered (npm/src/commands/update.js).

Every test spawns the real CLI as a subprocess (mirrors test_update_hooks_ac6.py)
with an isolated $HOME (tmp_path fixture) — the real user $HOME is never touched.
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


def _make_project(tmp_path: Path, yaml_content: str) -> tuple[Path, Path]:
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    (project / "trackfw.yaml").write_text(yaml_content, encoding="utf-8")
    return project, home


def _make_global_adr_dir(home: Path, with_adr: bool) -> None:
    global_dir = home / ".trackfw" / "adr"
    global_dir.mkdir(parents=True)
    if with_adr:
        (global_dir / "ADR-2026-08-08-exemplo.md").write_text("# ADR\n", encoding="utf-8")


def test_update_registers_global_adr_dir_when_missing_from_yaml(tmp_path):
    project, home = _make_project(tmp_path, "hooks: none\nci: none\n")
    _make_global_adr_dir(home, with_adr=True)

    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    content = (project / "trackfw.yaml").read_text(encoding="utf-8")
    assert "adr_dirs:" in content
    assert "- docs/adr" in content
    assert "- ~/.trackfw/adr" in content
    assert "hooks: none" in content  # original content preserved


def test_update_is_noop_when_global_adr_dir_does_not_exist(tmp_path):
    project, home = _make_project(tmp_path, "hooks: none\nci: none\n")
    # No ~/.trackfw/adr created at all.

    before = (project / "trackfw.yaml").read_bytes()
    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    after = (project / "trackfw.yaml").read_bytes()
    assert before == after


def test_update_is_noop_when_global_adr_dir_is_empty(tmp_path):
    project, home = _make_project(tmp_path, "hooks: none\nci: none\n")
    _make_global_adr_dir(home, with_adr=False)

    before = (project / "trackfw.yaml").read_bytes()
    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    after = (project / "trackfw.yaml").read_bytes()
    assert before == after


def test_update_is_idempotent_when_already_registered(tmp_path):
    project, home = _make_project(
        tmp_path,
        "hooks: none\nci: none\nadr_dirs:\n  - docs/adr\n  - ~/.trackfw/adr\n",
    )
    _make_global_adr_dir(home, with_adr=True)

    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    content = (project / "trackfw.yaml").read_text(encoding="utf-8")
    assert content.count("~/.trackfw/adr") == 1


def test_update_preserves_comments_and_other_keys(tmp_path):
    yaml_content = (
        "# trackfw config\n"
        "hooks: none  # no git hooks\n"
        "ci: none\n"
        "backend: python\n"
    )
    project, home = _make_project(tmp_path, yaml_content)
    _make_global_adr_dir(home, with_adr=True)

    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    content = (project / "trackfw.yaml").read_text(encoding="utf-8")
    assert "# trackfw config" in content
    assert "hooks: none  # no git hooks" in content
    assert "backend: python" in content
    assert "- ~/.trackfw/adr" in content
