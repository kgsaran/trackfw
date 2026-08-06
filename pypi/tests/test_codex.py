import json

from trackfw.generators.codex import install_codex


def test_install_codex_creates_idempotent_native_artifacts(tmp_path):
    install_codex(str(tmp_path))
    install_codex(str(tmp_path))

    required = [
        "AGENTS.md",
        ".codex/config.toml",
        ".codex/hooks.json",
        ".codex/agents/trackfw-architect.toml",
        ".codex/agents/trackfw-reviewer.toml",
        ".agents/skills/trackfw-governance/SKILL.md",
        ".agents/skills/trackfw-release/SKILL.md",
    ]
    for name in required:
        assert (tmp_path / name).exists(), name

    config = (tmp_path / ".codex/config.toml").read_text(encoding="utf-8")
    assert config.count("[agents]") == 1
    assert "max_threads = 6" in config
    assert "max_depth = 1" in config

    hooks = json.loads(
        (tmp_path / ".codex/hooks.json").read_text(encoding="utf-8")
    )["hooks"]
    assert len(hooks["PermissionRequest"]) == 1
    assert (
        hooks["PermissionRequest"][0]["hooks"][0]["command"]
        == "scripts/trackfw-attention-signal.sh"
    )
    assert len(hooks["PreToolUse"]) == 1
    assert hooks["PreToolUse"][0]["matcher"] == "Bash"
    assert (
        hooks["PreToolUse"][0]["hooks"][0]["command"]
        == "scripts/trackfw-credential-guard.sh"
    )
    assert len(hooks["PostToolUse"]) == 2
    assert (
        hooks["PostToolUse"][0]["hooks"][0]["command"]
        == "scripts/trackfw-attention-cleanup.sh"
    )
    assert hooks["PostToolUse"][1]["matcher"] == "Bash"
    assert (
        hooks["PostToolUse"][1]["hooks"][0]["command"]
        == "scripts/trackfw-credential-guard.sh"
    )
