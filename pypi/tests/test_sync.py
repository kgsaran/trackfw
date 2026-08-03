"""ML-2A (REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis)
— `trackfw sync` (Python CLI) now resolves linear_*/jira_* fields through the single config
loader (trackfw.config, cfg["sync"]) instead of the removed artisanal _read_config_field scanner.

Every scenario runs against a project with no docs/req/, so _sync_to_provider() returns []
before ever making a real network call — these tests exercise config resolution + error text
only, never the Linear/Jira HTTP clients.
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

PYPI_ROOT = Path(__file__).parents[1]


def cli(*arguments: str, cwd: Path, env: dict):
    environment = dict(os.environ)
    environment["PYTHONPATH"] = str(PYPI_ROOT)
    environment.update(env)
    return subprocess.run(
        [sys.executable, "-m", "trackfw", *arguments],
        cwd=cwd,
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )


# AC5 — trackfw.yaml value wins over the env var fallback.
def test_sync_linear_file_takes_precedence_over_env(tmp_path):
    (tmp_path / "trackfw.yaml").write_text(
        "linear_api_key: file-key\nlinear_team_id: file-team\n", encoding="utf-8"
    )
    result = cli(
        "sync", "--to=linear", cwd=tmp_path,
        env={"LINEAR_API_KEY": "env-key", "LINEAR_TEAM_ID": "env-team"},
    )
    assert result.returncode == 0, result.stderr + result.stdout
    assert "No REQs found" in result.stdout


# AC5 — env var fallback used only when trackfw.yaml has no value.
def test_sync_linear_env_fallback_when_file_absent(tmp_path):
    result = cli(
        "sync", "--to=linear", cwd=tmp_path,
        env={"LINEAR_API_KEY": "env-key", "LINEAR_TEAM_ID": "env-team"},
    )
    assert result.returncode == 0, result.stderr + result.stdout
    assert "No REQs found" in result.stdout


# AC5 — error text byte-identical to the pre-refactor scanner's messages.
def test_sync_linear_error_message_unchanged(tmp_path):
    result = cli(
        "sync", "--to=linear", cwd=tmp_path,
        env={"LINEAR_API_KEY": "", "LINEAR_TEAM_ID": ""},
    )
    assert result.returncode != 0
    assert (
        "Linear API key not found. Set LINEAR_API_KEY env var or linear_api_key in trackfw.yaml"
        in result.stderr
    )


# AC4 — quoted value, trailing comment and a nested homonym key resolve to the root-level value.
# The artisanal scanner matched the first line with the "field:" prefix at ANY indentation, so
# the nested homonym below would have silently hijacked the value; the single YAML-library
# loader must not.
def test_sync_linear_ac4_tricky_yaml(tmp_path):
    (tmp_path / "trackfw.yaml").write_text(
        "some_unrelated_map:\n"
        "  linear_api_key: hijacked-nested-value\n"
        'linear_api_key: "quoted-root-key"  # trailing comment\n'
        "linear_team_id: root-team\n",
        encoding="utf-8",
    )
    result = cli("sync", "--to=linear", cwd=tmp_path, env={})
    assert result.returncode == 0, result.stderr + result.stdout
    assert "No REQs found" in result.stdout


# AC5 — trackfw.yaml value wins over env vars for Jira.
def test_sync_jira_file_takes_precedence_over_env(tmp_path):
    (tmp_path / "trackfw.yaml").write_text(
        'jira_base_url: "https://file.atlassian.net:443"\n'
        "jira_email: file@example.com\n"
        "jira_token: file-token\n"
        "jira_project: FILEPROJ\n",
        encoding="utf-8",
    )
    result = cli(
        "sync", "--to=jira", cwd=tmp_path,
        env={
            "JIRA_BASE_URL": "https://env.atlassian.net",
            "JIRA_EMAIL": "env@example.com",
            "JIRA_TOKEN": "env-token",
            "JIRA_PROJECT": "ENVPROJ",
        },
    )
    assert result.returncode == 0, result.stderr + result.stdout
    assert "No REQs found" in result.stdout


# AC5 — env var fallback for Jira when trackfw.yaml is absent.
def test_sync_jira_env_fallback_when_file_absent(tmp_path):
    result = cli(
        "sync", "--to=jira", cwd=tmp_path,
        env={
            "JIRA_BASE_URL": "https://env.atlassian.net",
            "JIRA_EMAIL": "env@example.com",
            "JIRA_TOKEN": "env-token",
            "JIRA_PROJECT": "ENVPROJ",
        },
    )
    assert result.returncode == 0, result.stderr + result.stdout
    assert "No REQs found" in result.stdout


# AC5 — error text byte-identical to the pre-refactor scanner's messages.
def test_sync_jira_error_message_unchanged(tmp_path):
    result = cli(
        "sync", "--to=jira", cwd=tmp_path,
        env={"JIRA_BASE_URL": "", "JIRA_EMAIL": "", "JIRA_TOKEN": "", "JIRA_PROJECT": ""},
    )
    assert result.returncode != 0
    assert (
        "Jira base URL not found. Set JIRA_BASE_URL env var or jira_base_url in trackfw.yaml"
        in result.stderr
    )


# AC4 — colon-embedded scalar (jira_base_url with an explicit port) resolves whole.
def test_sync_jira_ac4_colon_embedded_scalar(tmp_path):
    (tmp_path / "trackfw.yaml").write_text(
        'jira_base_url: "https://x.atlassian.net:443"\n'
        "jira_email: bot@example.com\n"
        "jira_token: tok\n"
        "jira_project: PROJ\n",
        encoding="utf-8",
    )
    result = cli("sync", "--to=jira", cwd=tmp_path, env={})
    assert result.returncode == 0, result.stderr + result.stdout
    assert "No REQs found" in result.stdout
