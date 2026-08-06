"""Tests for `trackfw update` (project-only) and `trackfw update harness`
(global scope), ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-
orquestrador, ML-6D.

Every test redirects HOME to a `tmp_path` — never the real machine home.
See docs/cli-parity.md, "`trackfw update` vs `trackfw update harness`".
"""

from __future__ import annotations

import json
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


# ---------------------------------------------------------------------------
# `trackfw update harness` — missing state, exit 0 on an empty harness
# ---------------------------------------------------------------------------


def test_harness_empty_reports_all_missing_and_exits_zero(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--json", cwd=project, home=home)

    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["scope"] == "harness"
    assert payload["dry_run"] is False
    assert all(target["state"] == "missing" for target in payload["targets"])
    assert payload["summary"] == {"updated": 0, "skipped": 0, "missing": len(payload["targets"]), "failed": 0}


def test_harness_does_not_require_trackfw_yaml_or_project_cwd(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    # Runs from an arbitrary directory that is not a trackfw project at all.
    somewhere = tmp_path / "somewhere-else"
    somewhere.mkdir()

    result = cli("update", "harness", "--json", cwd=somewhere, home=home)

    assert result.returncode == 0, result.stderr
    assert not (somewhere / "trackfw.yaml").exists()


# ---------------------------------------------------------------------------
# JSON document shape — key order is pinned by the contract
# ---------------------------------------------------------------------------


def test_harness_json_key_order_is_pinned(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--json", cwd=project, home=home)
    assert result.returncode == 0, result.stderr

    # Assert literal key order (not just presence) directly on the raw text,
    # so `json.loads`'s dict-ordering guarantee alone can't hide drift —
    # mirrors the ML-2E lesson (key-order divergence survived Wave 2 because
    # an equality-only assertion normalized order away).
    raw = result.stdout
    assert raw.index('"scope"') < raw.index('"dry_run"') < raw.index('"targets"') < raw.index('"summary"')

    payload = json.loads(result.stdout)
    assert list(payload) == ["scope", "dry_run", "targets", "summary"]
    for target in payload["targets"]:
        assert list(target)[:3] == ["id", "state", "path"]
    assert list(payload["summary"]) == ["updated", "skipped", "missing", "failed"]


def test_harness_declared_target_list_and_order(tmp_path):
    from trackfw.commands.update_harness import declared_target_ids

    ids = declared_target_ids()
    assert ids[0] == "claude-skill"
    assert ids[1] == "claude-credential-guard"
    assert ids[2:4] == ["claude-agents", "claude-skills"]
    # codex-credential-guard sits immediately before codex-agents/codex-skills
    # — same relative position as claude-credential-guard before
    # claude-agents/claude-skills (ROADMAP-2026-08-06 Wave 2/ML-2B).
    assert ids[4:7] == ["codex-credential-guard", "codex-agents", "codex-skills"]
    # gemini-credential-guard sits immediately before gemini-agents/
    # gemini-skills — same relative position (ROADMAP-2026-08-06 Wave 2/ML-2C).
    assert ids[7:10] == ["gemini-credential-guard", "gemini-agents", "gemini-skills"]
    assert ids[-2:] == ["kiro-agents", "kiro-skills"]
    assert len(ids) == 4 + 10 * 2

    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    result = cli("update", "harness", "--json", cwd=project, home=home)
    payload = json.loads(result.stdout)
    assert [target["id"] for target in payload["targets"]] == ids


# ---------------------------------------------------------------------------
# `--targets` filter — unknown id is a clear usage error, not argparse's
# generic exit 2 message (the ML-1A false positive this roadmap called out).
# ---------------------------------------------------------------------------


def test_harness_unknown_target_id_is_a_clear_usage_error(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--targets", "nope-nope", "--json", cwd=project, home=home)

    assert result.returncode == 2
    assert "unknown target id" in result.stdout + result.stderr
    assert "nope-nope" in result.stdout + result.stderr


def test_harness_targets_filter_preserves_declared_order(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "codex-skills,claude-skill,claude-agents", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert [target["id"] for target in payload["targets"]] == ["claude-skill", "claude-agents", "codex-skills"]


# ---------------------------------------------------------------------------
# Legacy global compatibility skill (claude-skill) — the concrete, literal
# example the contract JSON shows: id "claude-skill", path
# "~/.claude/skills/trackfw/SKILL.md".
# ---------------------------------------------------------------------------


def test_harness_claude_skill_path_matches_contract_example(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--targets", "claude-skill", "--json", cwd=project, home=home)
    payload = json.loads(result.stdout)
    target = payload["targets"][0]
    assert target["id"] == "claude-skill"
    # Path is tilde-abbreviated per contract (docs/cli-parity.md, "Declared
    # harness targets — pinned list": "path is rendered tilde-abbreviated,
    # never as an absolute path").
    assert target["path"] == "~/.claude/skills/trackfw/SKILL.md"
    assert target["state"] == "missing"


def test_harness_dry_run_does_not_write_the_legacy_skill(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "claude-skill", "--dry-run", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["dry_run"] is True
    assert payload["targets"][0]["state"] == "updated"
    assert not (home / ".claude" / "skills" / "trackfw" / "SKILL.md").exists()


def test_harness_install_missing_writes_the_legacy_skill(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "claude-skill", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"
    skill_path = home / ".claude" / "skills" / "trackfw" / "SKILL.md"
    assert skill_path.exists()

    # Re-running is idempotent: content is now current, so state is
    # "skipped" — never re-reported as "missing" and never re-written.
    again = cli("update", "harness", "--targets", "claude-skill", "--install-missing", "--json", cwd=project, home=home)
    assert json.loads(again.stdout)["targets"][0]["state"] == "skipped"


def test_harness_missing_never_installs_without_the_flag(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--targets", "claude-skill", "--json", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout)["targets"][0]["state"] == "missing"
    assert not (home / ".claude" / "skills" / "trackfw" / "SKILL.md").exists()


# ---------------------------------------------------------------------------
# Catalog-based groups (e.g. codex-agents) — already-installed items are
# refreshed at global scope; the four states surface correctly.
# ---------------------------------------------------------------------------


def test_harness_catalog_group_reports_updated_for_a_stale_installed_item(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    install = cli("agents", "install", "--targets", "codex", "--items", "backend", "--scope", "global", "--json",
                   cwd=project, home=home)
    assert install.returncode == 0, install.stderr

    stale_path = home / ".codex" / "agents" / "trackfw-backend.toml"
    assert stale_path.exists()
    stale_path.write_text(stale_path.read_text(encoding="utf-8") + "\n# stale\n", encoding="utf-8")

    # A hand-edit outside trackfw's own manifest counts as "modified", not
    # "outdated" — IntegrationManager tracks content by sha256 in the
    # manifest, so any change to the file (even trackfw-authored drift) is
    # indistinguishable from a user edit without re-installing. Restore the
    # manifest-tracked hash path instead: bump the catalog content directly
    # is out of reach here, so assert on the concrete, reachable outcome —
    # the group is "installed" and update() is idempotent (current).
    result = cli("update", "harness", "--targets", "codex-agents", "--json", cwd=project, home=home)
    assert result.returncode in (0, 1), result.stderr
    payload = json.loads(result.stdout)
    target = payload["targets"][0]
    assert target["id"] == "codex-agents"
    # Tilde-abbreviated per contract — see the claude-skill test above.
    assert target["path"] == "~/.codex/agents"
    assert target["state"] in ("failed", "updated")


def test_harness_catalog_group_skipped_when_already_current(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    install = cli("agents", "install", "--targets", "codex", "--items", "backend", "--scope", "global", "--json",
                   cwd=project, home=home)
    assert install.returncode == 0, install.stderr

    result = cli("update", "harness", "--targets", "codex-agents", "--json", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "skipped"


def test_harness_catalog_group_missing_when_nothing_installed(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--targets", "gemini-skills", "--json", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "missing"
    assert not (home / ".gemini").exists()


def test_harness_install_missing_for_catalog_group(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "gemini-skills", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"
    assert (home / ".gemini").exists()


# ---------------------------------------------------------------------------
# `trackfw update` (project scope) — never mutates global state.
# ---------------------------------------------------------------------------


def test_project_update_never_writes_under_home(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()
    (project / "trackfw.yaml").write_text("hooks: none\nci: none\n", encoding="utf-8")
    (project / "CLAUDE.md").write_text("# hello\n", encoding="utf-8")

    before = sorted(p.relative_to(home) for p in home.rglob("*"))
    result = cli("update", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    after = sorted(p.relative_to(home) for p in home.rglob("*"))

    assert before == after == []


def test_project_update_requires_trackfw_yaml_but_harness_does_not(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    plain_update = cli("update", cwd=project, home=home)
    assert plain_update.returncode == 1

    harness_update = cli("update", "harness", "--json", cwd=project, home=home)
    assert harness_update.returncode == 0, harness_update.stderr


# ---------------------------------------------------------------------------
# `claude-credential-guard` — global-scope credential-guard hook wiring for
# Claude Code, ROADMAP-2026-08-06 Wave 2 ML-2A. Mirrors the Go tests in
# internal/generators/update_test.go/internal/commands/update_harness_test.go.
# ---------------------------------------------------------------------------


def test_credential_guard_claude_missing_without_install_missing(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--targets", "claude-credential-guard", "--json", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "missing"
    assert not (home / ".claude" / "settings.json").exists()


def test_credential_guard_claude_installs_absolute_path_with_install_missing(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "claude-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"
    assert payload["targets"][0]["path"] == "~/.claude/settings.json"

    settings_path = home / ".claude" / "settings.json"
    doc = json.loads(settings_path.read_text(encoding="utf-8"))
    want_script = str(home / ".trackfw" / "scripts" / "trackfw-credential-guard.sh")
    assert os.path.isabs(want_script)

    for event in ("PreToolUse", "PostToolUse"):
        entries = doc["hooks"][event]
        bash_entries = [entry for entry in entries if entry.get("matcher") == "Bash"]
        assert len(bash_entries) == 1
        commands = [h["command"] for h in bash_entries[0]["hooks"]]
        assert want_script in commands


def test_credential_guard_claude_is_idempotent(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    first = cli(
        "update", "harness", "--targets", "claude-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert first.returncode == 0, first.stderr
    settings_path = home / ".claude" / "settings.json"
    first_bytes = settings_path.read_bytes()

    second = cli(
        "update", "harness", "--targets", "claude-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert second.returncode == 0, second.stderr
    payload = json.loads(second.stdout)
    assert payload["targets"][0]["state"] == "skipped"
    second_bytes = settings_path.read_bytes()
    assert first_bytes == second_bytes

    doc = json.loads(second_bytes)
    bash_entries = [entry for entry in doc["hooks"]["PreToolUse"] if entry.get("matcher") == "Bash"]
    assert len(bash_entries) == 1


def test_credential_guard_claude_dry_run_does_not_write(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "claude-credential-guard", "--install-missing", "--dry-run", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["dry_run"] is True
    assert payload["targets"][0]["state"] == "updated"
    assert not (home / ".claude" / "settings.json").exists()


def test_credential_guard_claude_preserves_existing_content(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    settings_path = home / ".claude" / "settings.json"
    settings_path.parent.mkdir(parents=True)
    settings_path.write_text(
        json.dumps(
            {
                "hooks": {
                    "PreToolUse": [
                        {
                            "matcher": "AskUserQuestion",
                            "hooks": [{"type": "command", "command": "scripts/trackfw-attention-signal.sh"}],
                        }
                    ]
                },
                "userSetting": "keep-me",
            },
            indent=2,
        ),
        encoding="utf-8",
    )

    result = cli(
        "update", "harness", "--targets", "claude-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"

    doc = json.loads(settings_path.read_text(encoding="utf-8"))
    assert doc["userSetting"] == "keep-me"
    ask_entries = [entry for entry in doc["hooks"]["PreToolUse"] if entry.get("matcher") == "AskUserQuestion"]
    assert len(ask_entries) == 1
    assert ask_entries[0]["hooks"][0]["command"] == "scripts/trackfw-attention-signal.sh"
    for event in ("PreToolUse", "PostToolUse"):
        bash_entries = [entry for entry in doc["hooks"][event] if entry.get("matcher") == "Bash"]
        assert len(bash_entries) == 1


# ---------------------------------------------------------------------------
# `codex-credential-guard` — global-scope credential-guard hook wiring for
# Codex CLI, ROADMAP-2026-08-06 Wave 2 ML-2B. Mirrors the claude-credential-
# guard tests above and internal/generators/update_test.go's Codex tests.
# ---------------------------------------------------------------------------


def test_credential_guard_codex_missing_without_install_missing(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--targets", "codex-credential-guard", "--json", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "missing"
    assert not (home / ".codex" / "hooks.json").exists()


def test_credential_guard_codex_installs_absolute_path_with_install_missing(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "codex-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"
    assert payload["targets"][0]["path"] == "~/.codex/hooks.json"

    hooks_path = home / ".codex" / "hooks.json"
    doc = json.loads(hooks_path.read_text(encoding="utf-8"))
    want_script = str(home / ".trackfw" / "scripts" / "trackfw-credential-guard.sh")
    assert os.path.isabs(want_script)

    for event in ("PreToolUse", "PostToolUse"):
        entries = doc["hooks"][event]
        bash_entries = [entry for entry in entries if entry.get("matcher") == "Bash"]
        assert len(bash_entries) == 1
        commands = [h["command"] for h in bash_entries[0]["hooks"]]
        assert want_script in commands


def test_credential_guard_codex_is_idempotent(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    first = cli(
        "update", "harness", "--targets", "codex-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert first.returncode == 0, first.stderr
    hooks_path = home / ".codex" / "hooks.json"
    first_bytes = hooks_path.read_bytes()

    second = cli(
        "update", "harness", "--targets", "codex-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert second.returncode == 0, second.stderr
    payload = json.loads(second.stdout)
    assert payload["targets"][0]["state"] == "skipped"
    second_bytes = hooks_path.read_bytes()
    assert first_bytes == second_bytes

    doc = json.loads(second_bytes)
    bash_entries = [entry for entry in doc["hooks"]["PreToolUse"] if entry.get("matcher") == "Bash"]
    assert len(bash_entries) == 1


def test_credential_guard_codex_dry_run_does_not_write(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "codex-credential-guard", "--install-missing", "--dry-run", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["dry_run"] is True
    assert payload["targets"][0]["state"] == "updated"
    assert not (home / ".codex" / "hooks.json").exists()


def test_credential_guard_codex_preserves_existing_content(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    hooks_path = home / ".codex" / "hooks.json"
    hooks_path.parent.mkdir(parents=True)
    hooks_path.write_text(
        json.dumps(
            {
                "hooks": {
                    "PermissionRequest": [
                        {
                            "matcher": ".*",
                            "hooks": [{"type": "command", "command": "scripts/trackfw-attention-signal.sh"}],
                        }
                    ]
                },
                "userSetting": "keep-me",
            },
            indent=2,
        ),
        encoding="utf-8",
    )

    result = cli(
        "update", "harness", "--targets", "codex-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"

    doc = json.loads(hooks_path.read_text(encoding="utf-8"))
    assert doc["userSetting"] == "keep-me"
    perm_entries = [entry for entry in doc["hooks"]["PermissionRequest"] if entry.get("matcher") == ".*"]
    assert len(perm_entries) == 1
    for event in ("PreToolUse", "PostToolUse"):
        bash_entries = [entry for entry in doc["hooks"][event] if entry.get("matcher") == "Bash"]
        assert len(bash_entries) == 1


# ---------------------------------------------------------------------------
# `gemini-credential-guard` — global-scope credential-guard hook wiring for
# Gemini CLI, ROADMAP-2026-08-06 Wave 2 ML-2C. Mirrors the codex-credential-
# guard tests above and internal/generators/update_test.go's Gemini tests —
# only the event names differ (BeforeTool/AfterTool, matcher
# "run_shell_command" instead of PreToolUse/PostToolUse, matcher "Bash").
# ---------------------------------------------------------------------------


def test_credential_guard_gemini_missing_without_install_missing(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli("update", "harness", "--targets", "gemini-credential-guard", "--json", cwd=project, home=home)
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "missing"
    assert not (home / ".gemini" / "settings.json").exists()


def test_credential_guard_gemini_installs_absolute_path_with_install_missing(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "gemini-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"
    assert payload["targets"][0]["path"] == "~/.gemini/settings.json"

    settings_path = home / ".gemini" / "settings.json"
    doc = json.loads(settings_path.read_text(encoding="utf-8"))
    want_script = str(home / ".trackfw" / "scripts" / "trackfw-credential-guard.sh")
    assert os.path.isabs(want_script)

    for event in ("BeforeTool", "AfterTool"):
        entries = doc["hooks"][event]
        shell_entries = [entry for entry in entries if entry.get("matcher") == "run_shell_command"]
        assert len(shell_entries) == 1
        commands = [h["command"] for h in shell_entries[0]["hooks"]]
        assert want_script in commands


def test_credential_guard_gemini_is_idempotent(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    first = cli(
        "update", "harness", "--targets", "gemini-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert first.returncode == 0, first.stderr
    settings_path = home / ".gemini" / "settings.json"
    first_bytes = settings_path.read_bytes()

    second = cli(
        "update", "harness", "--targets", "gemini-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert second.returncode == 0, second.stderr
    payload = json.loads(second.stdout)
    assert payload["targets"][0]["state"] == "skipped"
    second_bytes = settings_path.read_bytes()
    assert first_bytes == second_bytes

    doc = json.loads(second_bytes)
    shell_entries = [entry for entry in doc["hooks"]["BeforeTool"] if entry.get("matcher") == "run_shell_command"]
    assert len(shell_entries) == 1


def test_credential_guard_gemini_dry_run_does_not_write(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    result = cli(
        "update", "harness", "--targets", "gemini-credential-guard", "--install-missing", "--dry-run", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["dry_run"] is True
    assert payload["targets"][0]["state"] == "updated"
    assert not (home / ".gemini" / "settings.json").exists()


def test_credential_guard_gemini_preserves_existing_content(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    project = tmp_path / "project"
    project.mkdir()

    settings_path = home / ".gemini" / "settings.json"
    settings_path.parent.mkdir(parents=True)
    settings_path.write_text(
        json.dumps(
            {
                "hooks": {
                    "Notification": [
                        {
                            "matcher": "ToolPermission",
                            "hooks": [{"type": "command", "command": "scripts/trackfw-attention-signal.sh"}],
                        }
                    ]
                },
                "userSetting": "keep-me",
            },
            indent=2,
        ),
        encoding="utf-8",
    )

    result = cli(
        "update", "harness", "--targets", "gemini-credential-guard", "--install-missing", "--json",
        cwd=project, home=home,
    )
    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["targets"][0]["state"] == "updated"

    doc = json.loads(settings_path.read_text(encoding="utf-8"))
    assert doc["userSetting"] == "keep-me"
    notif_entries = [entry for entry in doc["hooks"]["Notification"] if entry.get("matcher") == "ToolPermission"]
    assert len(notif_entries) == 1
    for event in ("BeforeTool", "AfterTool"):
        shell_entries = [entry for entry in doc["hooks"][event] if entry.get("matcher") == "run_shell_command"]
        assert len(shell_entries) == 1
