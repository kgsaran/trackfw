package generators

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/integrations"
)

func TestUpdateDoesNotImplicitlyInstallAgentIntegrations(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
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
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "trackfw", "adr.md")); err != nil {
		t.Fatalf("historical update auxiliary was not preserved: %v", err)
	}
	// trackfw update is project-scope only: it must never write the global
	// legacy Claude skill in the user's home directory. That is the job of
	// 'trackfw update harness' (see TestUpdateHarness*).
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "trackfw", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("trackfw update must not write the global harness skill: %v", err)
	}
}

func TestUpdateMigratesKnownCodexAndPreservesUnknown(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
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

// ────────────────────────────────────────────────────────────────────────────
// trackfw update harness — every test below redirects HOME to a t.TempDir()
// and never touches the real user home directory.
// ────────────────────────────────────────────────────────────────────────────

func TestUpdateHarnessRunsWithoutProjectOrTrackfwYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateHarness() erro inesperado: %v", err)
	}
	if report.Scope != "harness" {
		t.Fatalf("scope = %q, want harness", report.Scope)
	}
	if len(report.Targets) != len(HarnessTargetIDs) {
		t.Fatalf("got %d targets, want %d (%v)", len(report.Targets), len(HarnessTargetIDs), HarnessTargetIDs)
	}
}

func TestUpdateHarnessEmptyHomeReportsAllMissingAndDoesNotFail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateHarness() erro inesperado: %v", err)
	}
	summary := report.Summary()
	if summary.Missing != len(HarnessTargetIDs) {
		t.Fatalf("summary.Missing = %d, want %d (every target missing on an empty harness): %+v", summary.Missing, len(HarnessTargetIDs), report.Targets)
	}
	if summary.Updated != 0 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("expected only missing targets on an empty harness, got %+v", summary)
	}
}

func TestUpdateHarnessUnknownTargetIsUsageError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := UpdateHarness(UpdateOptions{Targets: []string{"not-a-real-target"}})
	if err == nil {
		t.Fatal("expected an error for an unknown --targets id")
	}
	var unknown *UnknownHarnessTargetError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected *UnknownHarnessTargetError, got %T: %v", err, err)
	}
}

func TestUpdateHarnessTargetsFilterPreservesDeclaredOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"codex-agents", "claude-skill"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Targets) != 2 {
		t.Fatalf("got %d targets, want 2: %+v", len(report.Targets), report.Targets)
	}
	// HarnessTargetIDs declares claude-skill before codex-agents — the
	// output must follow that declared order, not the --targets argument order.
	if report.Targets[0].ID != "claude-skill" || report.Targets[1].ID != "codex-agents" {
		t.Fatalf("targets not in declared order: %+v", report.Targets)
	}
}

func TestUpdateHarnessNeverTouchesHomeInDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{DryRun: true, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("report.DryRun = false, want true")
	}
	entries, _ := os.ReadDir(home)
	if len(entries) != 0 {
		t.Fatalf("dry-run wrote to HOME: %v", entries)
	}
}

func TestUpdateHarnessClaudeSkillInstallsOnlyWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-skill"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(GlobalClaudeSkillPath(home)); !os.IsNotExist(err) {
		t.Fatalf("claude-skill was installed without --install-missing: %v", err)
	}

	report, err = UpdateHarness(UpdateOptions{Targets: []string{"claude-skill"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}
	data, err := os.ReadFile(GlobalClaudeSkillPath(home))
	if err != nil {
		t.Fatalf("claude-skill was not installed with --install-missing: %v", err)
	}
	if string(data) != string(GlobalClaudeSkillContent()) {
		t.Fatal("installed claude-skill content does not match GlobalClaudeSkillContent()")
	}
}

func TestUpdateHarnessClaudeSkillUpdatesStaleContentAndSkipsCurrent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := GlobalClaudeSkillPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale content from a previous trackfw version\n"), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-skill"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (stale content)", report.Targets[0].State)
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(GlobalClaudeSkillContent()) {
		t.Fatal("stale claude-skill content was not rewritten to the current template")
	}

	// Second run: content is now current — must report skipped, not updated.
	report, err = UpdateHarness(UpdateOptions{Targets: []string{"claude-skill"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already current)", report.Targets[0].State)
	}
}

func TestUpdateHarnessDoesNotWriteAnythingOutsideHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if _, err := UpdateHarness(UpdateOptions{InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("trackfw update harness wrote into the current working directory: %v", entries)
	}
}

func TestUpdateInjectsAndUpdatesAttentionHooksIdempotently(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
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
