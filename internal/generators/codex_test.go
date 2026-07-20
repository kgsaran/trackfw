package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCodexCreatesNativeArtifacts(t *testing.T) {
	dir := t.TempDir()

	if err := InstallCodex(dir); err != nil {
		t.Fatalf("InstallCodex() error: %v", err)
	}
	if err := InstallCodex(dir); err != nil {
		t.Fatalf("InstallCodex() must be idempotent: %v", err)
	}

	required := []string{
		"AGENTS.md",
		".codex/config.toml",
		".codex/hooks.json",
		".codex/agents/trackfw-architect.toml",
		".codex/agents/trackfw-backend.toml",
		".codex/agents/trackfw-frontend.toml",
		".codex/agents/trackfw-qa.toml",
		".codex/agents/trackfw-security.toml",
		".codex/agents/trackfw-reviewer.toml",
		".agents/skills/trackfw-governance/SKILL.md",
		".agents/skills/trackfw-plan/SKILL.md",
		".agents/skills/trackfw-implement/SKILL.md",
		".agents/skills/trackfw-review/SKILL.md",
		".agents/skills/trackfw-release/SKILL.md",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing Codex artifact %s: %v", name, err)
		}
	}

	config, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(config), "[agents]") != 1 ||
		!strings.Contains(string(config), "max_threads = 6") ||
		!strings.Contains(string(config), "max_depth = 1") {
		t.Errorf("unexpected Codex config:\n%s", config)
	}

	hooks := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	if !helperHasClaudeHook(hooks, "PreToolUse", ".*", "scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse attention hook not found")
	}
	if !helperHasClaudeHook(hooks, "PostToolUse", ".*", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse cleanup hook not found")
	}
}
