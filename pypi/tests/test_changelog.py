import os
import subprocess
import sys

import pytest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from trackfw import changelog as _changelog  # noqa: E402


_SAMPLE = """# Changelog

Preamble text that should be discarded.

## [Unreleased]

### Added
- something in flight

## [6.10.0] - 2026-08-14

### Added
- feature A
- feature B

### Fixed
- bug X

## [6.9.1] - 2026-08-10

### Fixed
- bug Y
"""


def _run_cli(args, cwd):
    env = os.environ.copy()
    env["PYTHONPATH"] = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    return subprocess.run(
        [sys.executable, "-m", "trackfw.cli", "changelog", *args],
        cwd=cwd,
        env=env,
        text=True,
        # encoding explicito: o CLI escreve UTF-8 nos tres runtimes (ver
        # _force_utf8_output em trackfw/cli.py). text=True sozinho decodifica
        # pelo locale — cp1252 no Windows — e o travessao vira mojibake.
        encoding="utf-8",
        capture_output=True,
        check=False,
    )


def test_parse_sections_returns_three_sections():
    sections = _changelog.parse_sections(_SAMPLE)
    assert len(sections) == 3
    assert sections[0].version == "Unreleased"
    assert sections[0].date == ""
    assert sections[1].version == "6.10.0"
    assert sections[1].date == "2026-08-14"
    assert sections[2].version == "6.9.1"
    assert sections[2].date == "2026-08-10"


def test_first_section_returns_first():
    sections = _changelog.parse_sections(_SAMPLE)
    section = _changelog.first_section(sections)
    assert section.version == "Unreleased"


def test_first_section_errors_on_empty_list():
    with pytest.raises(ValueError) as exc_info:
        _changelog.first_section([])
    assert str(exc_info.value) == "CHANGELOG.md has no version sections"


def test_find_version_with_v_prefix():
    sections = _changelog.parse_sections(_SAMPLE)
    section = _changelog.find_version(sections, "v6.10.0")
    assert section.version == "6.10.0"


def test_find_version_without_prefix():
    sections = _changelog.parse_sections(_SAMPLE)
    section = _changelog.find_version(sections, "6.9.1")
    assert section.version == "6.9.1"


def test_find_version_not_found():
    sections = _changelog.parse_sections(_SAMPLE)
    with pytest.raises(ValueError) as exc_info:
        _changelog.find_version(sections, "999.0.0")
    assert str(exc_info.value) == 'version "999.0.0" not found in CHANGELOG.md'


def test_format_section_does_not_duplicate_leading_blank_line():
    # Body começa com "\n" (linha em branco logo após o cabeçalho, caso real
    # do CHANGELOG.md deste projeto) — format_section não pode duplicar essa
    # linha em branco.
    section = _changelog.Section(version="1.0.0", date="2026-01-01", body="\n### Added\n- x\n")
    formatted = _changelog.format_section(section)
    assert formatted == "## [1.0.0] - 2026-01-01\n\n### Added\n- x\n"
    assert "\n\n\n" not in formatted


def test_read_missing_file_raises(tmp_path):
    with pytest.raises(FileNotFoundError) as exc_info:
        _changelog.read(str(tmp_path))
    assert str(exc_info.value) == "CHANGELOG.md not found — nothing to show"


def test_cli_changelog_default_shows_first_section(tmp_path):
    (tmp_path / "CHANGELOG.md").write_text(_SAMPLE, encoding="utf-8")
    result = _run_cli([], tmp_path)
    assert result.returncode == 0, result.stderr
    assert result.stdout.startswith("## [Unreleased]\n\n### Added\n")


def test_cli_changelog_with_version(tmp_path):
    (tmp_path / "CHANGELOG.md").write_text(_SAMPLE, encoding="utf-8")
    result = _run_cli(["--version", "6.9.1"], tmp_path)
    assert result.returncode == 0, result.stderr
    assert result.stdout.startswith("## [6.9.1] - 2026-08-10\n\n")


def test_cli_changelog_version_not_found(tmp_path):
    (tmp_path / "CHANGELOG.md").write_text(_SAMPLE, encoding="utf-8")
    result = _run_cli(["--version", "999.0.0"], tmp_path)
    assert result.returncode == 1
    assert 'version "999.0.0" not found in CHANGELOG.md' in result.stderr


def test_cli_changelog_all_shows_entire_file(tmp_path):
    (tmp_path / "CHANGELOG.md").write_text(_SAMPLE, encoding="utf-8")
    result = _run_cli(["--all"], tmp_path)
    assert result.returncode == 0, result.stderr
    assert result.stdout == _SAMPLE


def test_cli_changelog_missing_file(tmp_path):
    result = _run_cli([], tmp_path)
    assert result.returncode == 1
    assert "CHANGELOG.md not found — nothing to show" in result.stderr
