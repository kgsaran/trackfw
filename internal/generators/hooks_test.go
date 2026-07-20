package generators

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInjectHooksDetected_SkipsAbsent(t *testing.T) {
	dir := t.TempDir()

	if err := InjectHooksDetected(dir); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	absent := []string{
		filepath.Join(dir, ".claude", "settings.json"),
		filepath.Join(dir, ".codex", "hooks.json"),
		filepath.Join(dir, ".gemini", "settings.json"),
		filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json"),
		filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"),
		filepath.Join(dir, ".cursor", "hooks.json"),
	}
	for _, p := range absent {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("arquivo criado sem CLI detectado: %s", p)
		}
	}
}

func TestInjectHooksDetected_Claude(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InjectHooksDetected(dir); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("hook Claude não foi injetado ao detectar CLAUDE.md")
	}
}
