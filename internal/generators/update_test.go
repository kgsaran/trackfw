package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/integrations"
)

func TestUpdateDoesNotImplicitlyInstallAgentIntegrations(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Existing instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Update(root); err != nil {
		t.Fatal(err)
	}
	for _, unexpected := range []string{
		filepath.Join(root, ".codex", "agents"),
		filepath.Join(root, ".agents", "skills"),
	} {
		if _, err := os.Stat(unexpected); !os.IsNotExist(err) {
			t.Fatalf("governance update implicitly installed integration %s: %v", unexpected, err)
		}
	}
	for _, expected := range []string{
		filepath.Join(root, ".claude", "commands", "trackfw", "adr.md"),
		filepath.Join(home, ".claude", "skills", "trackfw", "SKILL.md"),
	} {
		if _, err := os.Stat(expected); err != nil {
			t.Fatalf("historical update auxiliary was not preserved: %s: %v", expected, err)
		}
	}
}

func TestUpdateMigratesKnownCodexAndPreservesUnknown(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Existing instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agentsDir := filepath.Join(root, ".codex", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyBackend := []byte(`name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = """
Implement only the assigned backend scope. Preserve public contracts and trackfw traceability.
Run focused tests and report changed files, validation evidence, and remaining risks.
"""
`)
	backendPath := filepath.Join(agentsDir, "trackfw-backend.toml")
	frontendPath := filepath.Join(agentsDir, "trackfw-frontend.toml")
	unknown := []byte("user-owned unknown Codex agent\n")
	if err := os.WriteFile(backendPath, legacyBackend, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(frontendPath, unknown, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update(root); err != nil {
		t.Fatal(err)
	}
	catalog, _ := integrations.LoadCatalog()
	plans, _ := integrations.BuildPlans(catalog, integrations.PlanRequest{Kind: integrations.KindAgents, Targets: []string{"codex"}, Items: []string{"backend"}, Scope: "project"})
	backend, _ := os.ReadFile(backendPath)
	if string(backend) != string(plans[0].Content) {
		t.Fatal("known legacy Codex agent was not converted to canonical content")
	}
	frontend, _ := os.ReadFile(frontendPath)
	if string(frontend) != string(unknown) {
		t.Fatal("unknown Codex agent was modified")
	}
	if _, err := os.Stat(filepath.Join(agentsDir, "trackfw-qa.toml")); !os.IsNotExist(err) {
		t.Fatalf("governance update installed missing Codex item: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(root, ".trackfw", "integrations-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), backendPath) || strings.Contains(string(manifest), frontendPath) {
		t.Fatalf("unexpected Codex ownership manifest:\n%s", manifest)
	}
}

func TestUpdateInjectsAndUpdatesAttentionHooksIdempotently(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Criar marcadores para Claude, Cursor e Windsurf com hook customizado pré-existente no Claude
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	customClaudeSettings := []byte(`{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "CustomTool",
        "hooks": [{"type": "command", "command": "custom-script.sh"}]
      }
    ]
  }
}`)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), customClaudeSettings, 0o644); err != nil {
		t.Fatal(err)
	}

	cursorDir := filepath.Join(root, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	windsurfRules := filepath.Join(root, ".windsurfrules")
	if err := os.WriteFile(windsurfRules, []byte("# Existing windsurf rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Primeiramente executar Update
	if err := Update(root); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Validar que os scripts de atenção foram gerados em scripts/
	signalPath := filepath.Join(root, "scripts", "trackfw-attention-signal.sh")
	cleanupPath := filepath.Join(root, "scripts", "trackfw-attention-cleanup.sh")
	if _, err := os.Stat(signalPath); err != nil {
		t.Fatalf("attention signal script not created by update: %v", err)
	}
	if _, err := os.Stat(cleanupPath); err != nil {
		t.Fatalf("attention cleanup script not created by update: %v", err)
	}

	// Validar injeção do Claude preservando hook customizado
	claudeContent, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("failed to read claude settings: %v", err)
	}
	if !strings.Contains(string(claudeContent), "CustomTool") {
		t.Fatalf("custom claude hook was overwritten by update:\n%s", claudeContent)
	}
	if !strings.Contains(string(claudeContent), "AskUserQuestion") {
		t.Fatalf("claude attention hook missing after update:\n%s", claudeContent)
	}

	// Validar injeção do Cursor
	cursorContent, err := os.ReadFile(filepath.Join(cursorDir, "hooks.json"))
	if err != nil {
		t.Fatalf("failed to read cursor hooks: %v", err)
	}
	if !strings.Contains(string(cursorContent), "scripts/trackfw-attention-signal.sh") {
		t.Fatalf("cursor attention hook missing after update:\n%s", cursorContent)
	}

	// Validar injeção do Windsurf
	windsurfContent, err := os.ReadFile(windsurfRules)
	if err != nil {
		t.Fatalf("failed to read windsurfrules: %v", err)
	}
	if !strings.Contains(string(windsurfContent), "Windsurf users:") {
		t.Fatalf("windsurf instruction missing after update:\n%s", windsurfContent)
	}

	// Executar Update uma segunda vez para garantir idempotência
	if err := Update(root); err != nil {
		t.Fatalf("Second Update failed: %v", err)
	}

	claudeContentSecond, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	count := strings.Count(string(claudeContentSecond), "AskUserQuestion")
	if count != 2 {
		t.Fatalf("claude attention hooks duplicated on re-update. Expected 2 occurrences of AskUserQuestion, got %d:\n%s", count, claudeContentSecond)
	}
}

