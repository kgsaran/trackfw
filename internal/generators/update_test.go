package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// ────────────────────────────────────────────────────────────────────────────
// ensureGlobalADRDirRegistered (via Update) — ROADMAP-2026-08-08 ML-1A.
// ────────────────────────────────────────────────────────────────────────────

func writeGlobalADR(t *testing.T, home string) {
	t.Helper()
	dir := GlobalADRDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ADR-2026-08-08-example.md"), []byte("# Example ADR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRegistersGlobalADRDirWhenAbsentFromYAML(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	writeGlobalADR(t, home)

	yamlPath := filepath.Join(root, "trackfw.yaml")
	if err := os.WriteFile(yamlPath, []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "adr_dirs:\n  - docs/adr\n  - ~/.trackfw/adr\n") {
		t.Fatalf("trackfw.yaml did not gain the expected adr_dirs block:\n%s", content)
	}
	if !strings.Contains(content, "hooks: none\nci: none\n") {
		t.Fatalf("original trackfw.yaml content was not preserved:\n%s", content)
	}
}

func TestUpdateDoesNotTouchYAMLWhenGlobalADRDirMissing(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	// Deliberately do NOT create ~/.trackfw/adr.

	yamlPath := filepath.Join(root, "trackfw.yaml")
	original := []byte("hooks: none\nci: none\n")
	if err := os.WriteFile(yamlPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("trackfw.yaml was modified even though ~/.trackfw/adr does not exist:\ngot:  %q\nwant: %q", data, original)
	}
}

func TestUpdateDoesNotTouchYAMLWhenGlobalADRDirEmpty(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(GlobalADRDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	// Directory exists but has no ADR-*.md files.

	yamlPath := filepath.Join(root, "trackfw.yaml")
	original := []byte("hooks: none\nci: none\n")
	if err := os.WriteFile(yamlPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("trackfw.yaml was modified even though ~/.trackfw/adr has no ADR-*.md files:\ngot:  %q\nwant: %q", data, original)
	}
}

func TestUpdateIsIdempotentWhenGlobalADRDirAlreadyRegistered(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	writeGlobalADR(t, home)

	yamlPath := filepath.Join(root, "trackfw.yaml")
	original := []byte("hooks: none\nci: none\nadr_dirs:\n  - docs/adr\n  - ~/.trackfw/adr\n")
	if err := os.WriteFile(yamlPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("trackfw.yaml was modified even though ~/.trackfw/adr was already registered:\ngot:  %q\nwant: %q", data, original)
	}
}

func TestUpdateIsIdempotentWhenGlobalADRDirAlreadyRegisteredAsAbsolutePath(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	writeGlobalADR(t, home)

	absGlobalDir := GlobalADRDir(home)
	yamlPath := filepath.Join(root, "trackfw.yaml")
	original := []byte(fmt.Sprintf("hooks: none\nci: none\nadr_dirs:\n  - docs/adr\n  - %s\n", absGlobalDir))
	if err := os.WriteFile(yamlPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("trackfw.yaml was modified even though ~/.trackfw/adr was already registered as an absolute path:\ngot:  %q\nwant: %q", data, original)
	}
}

func TestUpdateRegistersGlobalADRDirPreservingCommentsAndOtherKeys(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	writeGlobalADR(t, home)

	yamlPath := filepath.Join(root, "trackfw.yaml")
	original := "# user comment at the top\nhooks: none\nci: none\n# another comment\nadr_dirs:\n  - docs/adr\n  - docs/adr/zeus\n"
	if err := os.WriteFile(yamlPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	want := "# user comment at the top\nhooks: none\nci: none\n# another comment\nadr_dirs:\n  - docs/adr\n  - docs/adr/zeus\n  - ~/.trackfw/adr\n"
	if content != want {
		t.Fatalf("trackfw.yaml surgical insert did not preserve comments/other keys as expected:\ngot:  %q\nwant: %q", content, want)
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

func TestUpdateHarnessCredentialGuardClaudeMissingWithoutInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-credential-guard"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("claude-credential-guard was installed without --install-missing: %v", err)
	}
}

func TestUpdateHarnessCredentialGuardClaudeInstallsAbsolutePathWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("~/.claude/settings.json was not written: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}

	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !filepath.IsAbs(wantScript) {
		t.Fatalf("test setup error: expected script path to be absolute: %s", wantScript)
	}
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		hooks, _ := doc["hooks"].(map[string]interface{})
		if hooks == nil {
			t.Fatalf("no hooks object written: %v", doc)
		}
		arr, _ := hooks[event].([]interface{})
		found := false
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["matcher"] != "Bash" {
				continue
			}
			innerHooks, _ := obj["hooks"].([]interface{})
			for _, h := range innerHooks {
				hObj, _ := h.(map[string]interface{})
				if hObj["command"] == wantScript {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("%s[matcher=Bash] does not point at the absolute global script path %s: %v", event, wantScript, doc)
		}
	}
}

func TestUpdateHarnessCredentialGuardClaudeIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-credential-guard"}, InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	firstRun, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already installed, idempotent)", report.Targets[0].State)
	}
	secondRun, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRun) != string(secondRun) {
		t.Fatalf("second run mutated ~/.claude/settings.json:\nfirst:  %s\nsecond: %s", firstRun, secondRun)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(secondRun, &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	arr, _ := hooks["PreToolUse"].([]interface{})
	bashEntries := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] == "Bash" {
			bashEntries++
		}
	}
	if bashEntries != 1 {
		t.Fatalf("expected exactly one PreToolUse[matcher=Bash] entry, got %d: %v", bashEntries, doc)
	}
}

func TestUpdateHarnessCredentialGuardClaudeDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-credential-guard"}, InstallMissing: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("report.DryRun = false, want true")
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (would install)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("--dry-run wrote ~/.claude/settings.json")
	}
}

func TestUpdateHarnessCredentialGuardClaudePreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	preexisting := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [
          {
            "type": "command",
            "command": "scripts/trackfw-attention-signal.sh"
          }
        ]
      }
    ]
  },
  "userSetting": "keep-me"
}
`
	if err := os.WriteFile(settingsPath, []byte(preexisting), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"claude-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (merging into existing file)", report.Targets[0].State)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["userSetting"] != "keep-me" {
		t.Fatalf("pre-existing top-level key was not preserved: %v", doc)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	preArr, _ := hooks["PreToolUse"].([]interface{})
	var matchers []string
	for _, item := range preArr {
		obj, _ := item.(map[string]interface{})
		matchers = append(matchers, fmt.Sprintf("%v", obj["matcher"]))
	}
	hasAskUserQuestion, hasBash := false, false
	for _, m := range matchers {
		if m == "AskUserQuestion" {
			hasAskUserQuestion = true
		}
		if m == "Bash" {
			hasBash = true
		}
	}
	if !hasAskUserQuestion {
		t.Fatalf("pre-existing PreToolUse[matcher=AskUserQuestion] entry was dropped: %v", matchers)
	}
	if !hasBash {
		t.Fatalf("expected PreToolUse[matcher=Bash] entry to be added: %v", matchers)
	}
}

func TestUpdateHarnessCredentialGuardCodexMissingWithoutInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"codex-credential-guard"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("codex-credential-guard was installed without --install-missing: %v", err)
	}
}

func TestUpdateHarnessCredentialGuardCodexInstallsAbsolutePathWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"codex-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}
	if report.Targets[0].Path != "~/.codex/hooks.json" {
		t.Fatalf("path = %q, want ~/.codex/hooks.json", report.Targets[0].Path)
	}

	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("~/.codex/hooks.json was not written: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}

	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !filepath.IsAbs(wantScript) {
		t.Fatalf("test setup error: expected script path to be absolute: %s", wantScript)
	}
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		hooks, _ := doc["hooks"].(map[string]interface{})
		if hooks == nil {
			t.Fatalf("no hooks object written: %v", doc)
		}
		arr, _ := hooks[event].([]interface{})
		found := false
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["matcher"] != "Bash" {
				continue
			}
			innerHooks, _ := obj["hooks"].([]interface{})
			for _, h := range innerHooks {
				hObj, _ := h.(map[string]interface{})
				if hObj["command"] == wantScript {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("%s[matcher=Bash] does not point at the absolute global script path %s: %v", event, wantScript, doc)
		}
	}
}

func TestUpdateHarnessCredentialGuardCodexIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := UpdateHarness(UpdateOptions{Targets: []string{"codex-credential-guard"}, InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	firstRun, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"codex-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already installed, idempotent)", report.Targets[0].State)
	}
	secondRun, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRun) != string(secondRun) {
		t.Fatalf("second run mutated ~/.codex/hooks.json:\nfirst:  %s\nsecond: %s", firstRun, secondRun)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(secondRun, &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	arr, _ := hooks["PreToolUse"].([]interface{})
	bashEntries := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] == "Bash" {
			bashEntries++
		}
	}
	if bashEntries != 1 {
		t.Fatalf("expected exactly one PreToolUse[matcher=Bash] entry, got %d: %v", bashEntries, doc)
	}
}

func TestUpdateHarnessCredentialGuardCodexDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"codex-credential-guard"}, InstallMissing: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("report.DryRun = false, want true")
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (would install)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatal("--dry-run wrote ~/.codex/hooks.json")
	}
}

func TestUpdateHarnessCredentialGuardCodexPreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0755); err != nil {
		t.Fatal(err)
	}
	preexisting := `{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "scripts/trackfw-attention-signal.sh"
          }
        ]
      }
    ]
  },
  "userSetting": "keep-me"
}
`
	if err := os.WriteFile(hooksPath, []byte(preexisting), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"codex-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (merging into existing file)", report.Targets[0].State)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["userSetting"] != "keep-me" {
		t.Fatalf("pre-existing top-level key was not preserved: %v", doc)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	permArr, _ := hooks["PermissionRequest"].([]interface{})
	if len(permArr) != 1 {
		t.Fatalf("pre-existing PermissionRequest entry was dropped: %v", hooks)
	}
	preArr, _ := hooks["PreToolUse"].([]interface{})
	var matchers []string
	for _, item := range preArr {
		obj, _ := item.(map[string]interface{})
		matchers = append(matchers, fmt.Sprintf("%v", obj["matcher"]))
	}
	hasBash := false
	for _, m := range matchers {
		if m == "Bash" {
			hasBash = true
		}
	}
	if !hasBash {
		t.Fatalf("expected PreToolUse[matcher=Bash] entry to be added: %v", matchers)
	}
}

// The following gemini-credential-guard tests mirror the codex-credential-
// guard tests above (ROADMAP-2026-08-06 Wave 2/ML-2C), only the top-level
// hook event names differ (BeforeTool/AfterTool instead of PreToolUse/
// PostToolUse) since Gemini CLI uses a different event vocabulary than
// Claude/Codex (see mergeCredentialGuardGeminiHooks, update.go).

func TestUpdateHarnessCredentialGuardGeminiMissingWithoutInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"gemini-credential-guard"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("gemini-credential-guard was installed without --install-missing: %v", err)
	}
}

func TestUpdateHarnessCredentialGuardGeminiInstallsAbsolutePathWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"gemini-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}
	if report.Targets[0].Path != "~/.gemini/settings.json" {
		t.Fatalf("path = %q, want ~/.gemini/settings.json", report.Targets[0].Path)
	}

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("~/.gemini/settings.json was not written: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}

	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !filepath.IsAbs(wantScript) {
		t.Fatalf("test setup error: expected script path to be absolute: %s", wantScript)
	}
	for _, event := range []string{"BeforeTool", "AfterTool"} {
		hooks, _ := doc["hooks"].(map[string]interface{})
		if hooks == nil {
			t.Fatalf("no hooks object written: %v", doc)
		}
		arr, _ := hooks[event].([]interface{})
		found := false
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["matcher"] != "run_shell_command" {
				continue
			}
			innerHooks, _ := obj["hooks"].([]interface{})
			for _, h := range innerHooks {
				hObj, _ := h.(map[string]interface{})
				if hObj["command"] == wantScript {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("%s[matcher=run_shell_command] does not point at the absolute global script path %s: %v", event, wantScript, doc)
		}
	}
}

func TestUpdateHarnessCredentialGuardGeminiIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := UpdateHarness(UpdateOptions{Targets: []string{"gemini-credential-guard"}, InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	firstRun, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"gemini-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already installed, idempotent)", report.Targets[0].State)
	}
	secondRun, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRun) != string(secondRun) {
		t.Fatalf("second run mutated ~/.gemini/settings.json:\nfirst:  %s\nsecond: %s", firstRun, secondRun)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(secondRun, &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	arr, _ := hooks["BeforeTool"].([]interface{})
	shellEntries := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] == "run_shell_command" {
			shellEntries++
		}
	}
	if shellEntries != 1 {
		t.Fatalf("expected exactly one BeforeTool[matcher=run_shell_command] entry, got %d: %v", shellEntries, doc)
	}
}

func TestUpdateHarnessCredentialGuardGeminiDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"gemini-credential-guard"}, InstallMissing: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("report.DryRun = false, want true")
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (would install)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("--dry-run wrote ~/.gemini/settings.json")
	}
}

func TestUpdateHarnessCredentialGuardGeminiPreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	preexisting := `{
  "hooks": {
    "Notification": [
      {
        "matcher": "ToolPermission",
        "hooks": [
          {
            "type": "command",
            "command": "scripts/trackfw-attention-signal.sh"
          }
        ]
      }
    ]
  },
  "userSetting": "keep-me"
}
`
	if err := os.WriteFile(settingsPath, []byte(preexisting), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"gemini-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (merging into existing file)", report.Targets[0].State)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["userSetting"] != "keep-me" {
		t.Fatalf("pre-existing top-level key was not preserved: %v", doc)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	notifArr, _ := hooks["Notification"].([]interface{})
	if len(notifArr) != 1 {
		t.Fatalf("pre-existing Notification entry was dropped: %v", hooks)
	}
	beforeArr, _ := hooks["BeforeTool"].([]interface{})
	var matchers []string
	for _, item := range beforeArr {
		obj, _ := item.(map[string]interface{})
		matchers = append(matchers, fmt.Sprintf("%v", obj["matcher"]))
	}
	hasShell := false
	for _, m := range matchers {
		if m == "run_shell_command" {
			hasShell = true
		}
	}
	if !hasShell {
		t.Fatalf("expected BeforeTool[matcher=run_shell_command] entry to be added: %v", matchers)
	}
}

// The following cursor-credential-guard tests mirror the gemini-credential-
// guard tests above (ROADMAP-2026-08-06 Wave 2/ML-2D). Cursor's hooks.json
// schema differs structurally from Claude/Codex/Gemini's — each event array
// holds flat {"command":"..."} entries with no per-entry "matcher" — see
// mergeCredentialGuardCursorHooks (update.go), so assertions below read
// hooks[event] directly instead of hooks[event][].hooks[].command.

func TestUpdateHarnessCredentialGuardCursorMissingWithoutInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"cursor-credential-guard"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("cursor-credential-guard was installed without --install-missing: %v", err)
	}
}

func TestUpdateHarnessCredentialGuardCursorInstallsAbsolutePathWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"cursor-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}
	if report.Targets[0].Path != "~/.cursor/hooks.json" {
		t.Fatalf("path = %q, want ~/.cursor/hooks.json", report.Targets[0].Path)
	}

	hooksPath := filepath.Join(home, ".cursor", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("~/.cursor/hooks.json was not written: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}
	if doc["version"] != float64(1) {
		t.Fatalf("version = %v, want 1", doc["version"])
	}

	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !filepath.IsAbs(wantScript) {
		t.Fatalf("test setup error: expected script path to be absolute: %s", wantScript)
	}
	for _, event := range []string{"beforeShellExecution", "afterShellExecution"} {
		hooks, _ := doc["hooks"].(map[string]interface{})
		if hooks == nil {
			t.Fatalf("no hooks object written: %v", doc)
		}
		arr, _ := hooks[event].([]interface{})
		found := false
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["command"] == wantScript {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s does not contain a {command} entry pointing at the absolute global script path %s: %v", event, wantScript, doc)
		}
	}
}

func TestUpdateHarnessCredentialGuardCursorIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := UpdateHarness(UpdateOptions{Targets: []string{"cursor-credential-guard"}, InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(home, ".cursor", "hooks.json")
	firstRun, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"cursor-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already installed, idempotent)", report.Targets[0].State)
	}
	secondRun, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRun) != string(secondRun) {
		t.Fatalf("second run mutated ~/.cursor/hooks.json:\nfirst:  %s\nsecond: %s", firstRun, secondRun)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(secondRun, &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	arr, _ := hooks["beforeShellExecution"].([]interface{})
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	shellEntries := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == wantScript {
			shellEntries++
		}
	}
	if shellEntries != 1 {
		t.Fatalf("expected exactly one beforeShellExecution entry, got %d: %v", shellEntries, doc)
	}
}

func TestUpdateHarnessCredentialGuardCursorDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"cursor-credential-guard"}, InstallMissing: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("report.DryRun = false, want true")
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (would install)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor", "hooks.json")); !os.IsNotExist(err) {
		t.Fatal("--dry-run wrote ~/.cursor/hooks.json")
	}
}

func TestUpdateHarnessCredentialGuardCursorPreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	hooksPath := filepath.Join(home, ".cursor", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0755); err != nil {
		t.Fatal(err)
	}
	preexisting := `{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "command": "scripts/trackfw-attention-signal.sh"
      }
    ]
  },
  "userSetting": "keep-me"
}
`
	if err := os.WriteFile(hooksPath, []byte(preexisting), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"cursor-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (merging into existing file)", report.Targets[0].State)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["userSetting"] != "keep-me" {
		t.Fatalf("pre-existing top-level key was not preserved: %v", doc)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	preArr, _ := hooks["preToolUse"].([]interface{})
	if len(preArr) != 1 {
		t.Fatalf("pre-existing preToolUse entry was dropped: %v", hooks)
	}
	beforeArr, _ := hooks["beforeShellExecution"].([]interface{})
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	hasShell := false
	for _, item := range beforeArr {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == wantScript {
			hasShell = true
		}
	}
	if !hasShell {
		t.Fatalf("expected beforeShellExecution entry to be added: %v", beforeArr)
	}
}

func TestUpdateHarnessCredentialGuardCopilotMissingWithoutInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"copilot-credential-guard"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".copilot", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("copilot-credential-guard was installed without --install-missing: %v", err)
	}
}

func TestUpdateHarnessCredentialGuardCopilotInstallsAbsolutePathWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"copilot-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}
	if report.Targets[0].Path != "~/.copilot/settings.json" {
		t.Fatalf("path = %q, want ~/.copilot/settings.json", report.Targets[0].Path)
	}

	settingsPath := filepath.Join(home, ".copilot", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("~/.copilot/settings.json was not written: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}
	if _, hasVersion := doc["version"]; hasVersion {
		t.Fatalf("~/.copilot/settings.json is a general config file, not a dedicated hooks file — must not gain an unconfirmed top-level \"version\" key: %v", doc)
	}

	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !filepath.IsAbs(wantScript) {
		t.Fatalf("test setup error: expected script path to be absolute: %s", wantScript)
	}
	for _, event := range []string{"preToolUse", "postToolUse"} {
		hooks, _ := doc["hooks"].(map[string]interface{})
		if hooks == nil {
			t.Fatalf("no hooks object written: %v", doc)
		}
		arr, _ := hooks[event].([]interface{})
		found := false
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["bash"] == wantScript {
				if obj["type"] != "command" {
					t.Fatalf("%s entry missing type=command: %v", event, obj)
				}
				if obj["matcher"] != "bash" {
					t.Fatalf("%s entry missing matcher=bash: %v", event, obj)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("%s does not contain a {bash} entry pointing at the absolute global script path %s: %v", event, wantScript, doc)
		}
	}
}

func TestUpdateHarnessCredentialGuardCopilotIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := UpdateHarness(UpdateOptions{Targets: []string{"copilot-credential-guard"}, InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".copilot", "settings.json")
	firstRun, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"copilot-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already installed, idempotent)", report.Targets[0].State)
	}
	secondRun, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRun) != string(secondRun) {
		t.Fatalf("second run mutated ~/.copilot/settings.json:\nfirst:  %s\nsecond: %s", firstRun, secondRun)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(secondRun, &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	arr, _ := hooks["preToolUse"].([]interface{})
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	shellEntries := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["bash"] == wantScript {
			shellEntries++
		}
	}
	if shellEntries != 1 {
		t.Fatalf("expected exactly one preToolUse entry, got %d: %v", shellEntries, doc)
	}
}

func TestUpdateHarnessCredentialGuardCopilotDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"copilot-credential-guard"}, InstallMissing: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("report.DryRun = false, want true")
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (would install)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".copilot", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("--dry-run wrote ~/.copilot/settings.json")
	}
}

func TestUpdateHarnessCredentialGuardCopilotPreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".copilot", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	preexisting := `{
  "model": "gpt-5",
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "matcher": "curl",
        "bash": "echo hi"
      }
    ]
  },
  "userSetting": "keep-me"
}
`
	if err := os.WriteFile(settingsPath, []byte(preexisting), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"copilot-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (merging into existing file)", report.Targets[0].State)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["userSetting"] != "keep-me" {
		t.Fatalf("pre-existing top-level key was not preserved: %v", doc)
	}
	if doc["model"] != "gpt-5" {
		t.Fatalf("pre-existing \"model\" key was not preserved: %v", doc)
	}
	hooks, _ := doc["hooks"].(map[string]interface{})
	preArr, _ := hooks["preToolUse"].([]interface{})
	if len(preArr) != 2 {
		t.Fatalf("pre-existing preToolUse entry was dropped or credential-guard entry not appended: %v", hooks)
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	hasGuard := false
	for _, item := range preArr {
		obj, _ := item.(map[string]interface{})
		if obj["bash"] == wantScript {
			hasGuard = true
		}
	}
	if !hasGuard {
		t.Fatalf("expected preToolUse entry to be added: %v", preArr)
	}
}

// --- kiro-credential-guard (ROADMAP-2026-08-06 Wave 2/ML-2F) ---
//
// Unlike claude/codex/gemini/cursor/copilot-credential-guard (which merge
// into a shared, general settings file), ~/.kiro/hooks/trackfw-credential-
// guard.json is a DEDICATED file — only trackfw ever writes it, mirroring
// harnessClaudeSkillTarget's own wholesale-overwrite contract (missing /
// install-missing / idempotent-skip / dry-run / stale-content-rewrite),
// not the merge-and-preserve-foreign-keys contract of the settings-file
// targets above.

func TestUpdateHarnessCredentialGuardKiroMissingWithoutInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"kiro-credential-guard"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetMissing {
		t.Fatalf("state = %q, want missing (no --install-missing)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")); !os.IsNotExist(err) {
		t.Fatalf("kiro-credential-guard was installed without --install-missing: %v", err)
	}
}

func TestUpdateHarnessCredentialGuardKiroInstallsAbsolutePathWithInstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"kiro-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (--install-missing)", report.Targets[0].State)
	}
	if report.Targets[0].Path != "~/.kiro/hooks/trackfw-credential-guard.json" {
		t.Fatalf("path = %q, want ~/.kiro/hooks/trackfw-credential-guard.json", report.Targets[0].Path)
	}

	hookPath := filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("~/.kiro/hooks/trackfw-credential-guard.json was not written: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}
	if v, _ := doc["version"].(string); v != "v1" {
		t.Fatalf(`expected "version":"v1", got %v`, doc["version"])
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !filepath.IsAbs(wantScript) {
		t.Fatalf("test setup error: expected script path to be absolute: %s", wantScript)
	}
	hooks, _ := doc["hooks"].([]interface{})
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks (pre/post), got %d: %v", len(hooks), doc)
	}
	sawPre, sawPost := false, false
	for _, h := range hooks {
		entry, _ := h.(map[string]interface{})
		if entry == nil {
			t.Fatalf("hook entry is not an object: %v", h)
		}
		if entry["matcher"] != "shell" {
			t.Fatalf("hook entry matcher = %v, want \"shell\": %v", entry["matcher"], entry)
		}
		action, _ := entry["action"].(map[string]interface{})
		if action == nil || action["type"] != "command" || action["command"] != wantScript {
			t.Fatalf("hook entry action does not point at the absolute global script path %s: %v", wantScript, entry)
		}
		switch entry["trigger"] {
		case "PreToolUse":
			sawPre = true
		case "PostToolUse":
			sawPost = true
		}
	}
	if !sawPre || !sawPost {
		t.Fatalf("expected both PreToolUse and PostToolUse hook entries: %v", hooks)
	}
}

func TestUpdateHarnessCredentialGuardKiroIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := UpdateHarness(UpdateOptions{Targets: []string{"kiro-credential-guard"}, InstallMissing: true}); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
	firstRun, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"kiro-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetSkipped {
		t.Fatalf("state = %q, want skipped (already installed, idempotent)", report.Targets[0].State)
	}
	secondRun, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRun) != string(secondRun) {
		t.Fatalf("second run mutated ~/.kiro/hooks/trackfw-credential-guard.json:\nfirst:  %s\nsecond: %s", firstRun, secondRun)
	}
}

func TestUpdateHarnessCredentialGuardKiroDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"kiro-credential-guard"}, InstallMissing: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("report.DryRun = false, want true")
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (would install)", report.Targets[0].State)
	}
	if _, err := os.Stat(filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")); !os.IsNotExist(err) {
		t.Fatal("--dry-run wrote ~/.kiro/hooks/trackfw-credential-guard.json")
	}
}

func TestUpdateHarnessCredentialGuardKiroRewritesStaleContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	hookPath := filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		t.Fatal(err)
	}
	// This is a DEDICATED file (only trackfw ever writes it) — unlike the
	// settings-file targets above, a pre-existing foreign shape here is
	// stale/corrupt state from a previous trackfw version, not user content
	// to merge into. It must be replaced wholesale, never merged.
	if err := os.WriteFile(hookPath, []byte(`{"version":"v1","hooks":[{"name":"stale"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateHarness(UpdateOptions{Targets: []string{"kiro-credential-guard"}, InstallMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets[0].State != TargetUpdated {
		t.Fatalf("state = %q, want updated (stale content rewritten)", report.Targets[0].State)
	}
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"stale"`) {
		t.Fatalf("stale content was not replaced: %s", data)
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
	guardPath := filepath.Join(root, "scripts", "trackfw-credential-guard.sh")
	if _, err := os.Stat(signalPath); err != nil {
		t.Fatalf("attention signal script not created by update: %v", err)
	}
	if _, err := os.Stat(cleanupPath); err != nil {
		t.Fatalf("attention cleanup script not created by update: %v", err)
	}
	if _, err := os.Stat(guardPath); err != nil {
		t.Fatalf("credential guard script not created by update: %v", err)
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

// TestUpdateBackfillsCredentialGuardScriptForPreExistingProject simulates a
// project that already ran `trackfw init`/`update` BEFORE this REQ:
// scripts/trackfw-attention-signal.sh exists but scripts/trackfw-credential-guard.sh
// does not. `trackfw update` must generate the missing script without breaking
// anything already there — the upgrade scenario from the acceptance criteria.
func TestUpdateBackfillsCredentialGuardScriptForPreExistingProject(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	signalPath := filepath.Join(scriptsDir, "trackfw-attention-signal.sh")
	if err := os.WriteFile(signalPath, []byte("#!/usr/bin/env bash\necho \"old signal script\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	guardPath := filepath.Join(scriptsDir, "trackfw-credential-guard.sh")
	if _, err := os.Stat(guardPath); !os.IsNotExist(err) {
		t.Fatalf("test precondition failed: credential guard script should not exist yet, stat err=%v", err)
	}

	if err := Update(root); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	guardInfo, err := os.Stat(guardPath)
	if err != nil {
		t.Fatalf("update did not backfill the missing credential guard script: %v", err)
	}
	if guardInfo.Mode().Perm()&0o111 == 0 {
		t.Errorf("credential guard script should be executable, mode=%v", guardInfo.Mode())
	}

	if _, err := os.Stat(signalPath); err != nil {
		t.Fatalf("pre-existing attention signal script should not be removed: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ci-workflow target manages .github/workflows/trackfw-validate.yml too
// (AC17, REQ-2026-08-28, ML-2G) — the doctor's "trackfw update" remedy for
// that file must stop being inert.
// ────────────────────────────────────────────────────────────────────────────

// TestUpdateCiWorkflowRefreshesDiscoverWorkflowWhenPresent covers AC17(a):
// existence on disk (not cfg.ci) is the criterion — a project with `ci:
// none` but an already-installed, stale trackfw-validate.yml gets it
// refreshed to the current template.
func TestUpdateCiWorkflowRefreshesDiscoverWorkflowWhenPresent(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workflowPath := filepath.Join(root, DiscoverGitHubActionsWorkflowPath)
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "name: trackfw validate\non: [push, pull_request]\njobs:\n  governance:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v0.0.1\n      - run: trackfw validate\n"
	if err := os.WriteFile(workflowPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateProject(root, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	var ciResult *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "ci-workflow" {
			ciResult = &report.Targets[i]
		}
	}
	if ciResult == nil {
		t.Fatal("ci-workflow target not declared for a project with ci:none but an existing trackfw-validate.yml (AC17(c))")
	}
	if ciResult.State != TargetUpdated {
		t.Fatalf("ci-workflow state = %q, want %q", ciResult.State, TargetUpdated)
	}

	got, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != BuildDiscoverGitHubActionsWorkflowContent() {
		t.Fatalf("trackfw-validate.yml was not refreshed to the current template:\n%s", got)
	}

	// AC17(d): running update again against the now-current file must not
	// report "updated" a second time.
	report2, err := UpdateProject(root, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateProject (second run): %v", err)
	}
	var ciResult2 *TargetResult
	for i := range report2.Targets {
		if report2.Targets[i].ID == "ci-workflow" {
			ciResult2 = &report2.Targets[i]
		}
	}
	if ciResult2 == nil {
		t.Fatal("ci-workflow target missing on second run")
	}
	if ciResult2.State != TargetSkipped {
		t.Fatalf("second UpdateProject run: ci-workflow state = %q, want %q (idempotency, AC17(d))", ciResult2.State, TargetSkipped)
	}
}

// TestUpdateCiWorkflowNeverCreatesDiscoverWorkflow covers AC17(b): `update`
// never creates trackfw-validate.yml for a project that doesn't already have
// it — including the trap case where trackfw-gate.yml (managed by ci:
// github-actions) IS present, which must not make runFileTarget's allEmpty
// check treat the target as "not missing" and let apply() create the second
// file.
func TestUpdateCiWorkflowNeverCreatesDiscoverWorkflow(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: github-actions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gatePath := filepath.Join(root, ".github", "workflows", "trackfw-gate.yml")
	if err := os.MkdirAll(filepath.Dir(gatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gatePath, []byte("stale gate content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	validatePath := filepath.Join(root, DiscoverGitHubActionsWorkflowPath)
	if _, err := os.Stat(validatePath); !os.IsNotExist(err) {
		t.Fatalf("test precondition failed: %s should not exist yet", validatePath)
	}

	report, err := UpdateProject(root, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	var ciResult *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "ci-workflow" {
			ciResult = &report.Targets[i]
		}
	}
	if ciResult == nil {
		t.Fatal("ci-workflow target not declared for a project with ci: github-actions")
	}
	// trackfw-gate.yml WAS refreshed (stale -> current), so the target as a
	// whole is "updated" — but that must not have created the sibling file.
	if ciResult.State != TargetUpdated {
		t.Fatalf("ci-workflow state = %q, want %q (trackfw-gate.yml refresh)", ciResult.State, TargetUpdated)
	}
	if _, err := os.Stat(validatePath); !os.IsNotExist(err) {
		t.Fatalf("AC17(b) violated: trackfw update created %s for a project that never had it (stat err=%v)", validatePath, err)
	}
}

// TestUpdateCiWorkflowNotDeclaredWithoutCIOrDiscoverWorkflow is the negative
// control for AC17(c): a project with ci:none and no trackfw-validate.yml on
// disk must not declare "ci-workflow" at all — same behavior as before this
// ML.
func TestUpdateCiWorkflowNotDeclaredWithoutCIOrDiscoverWorkflow(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := UpdateProject(root, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	for _, target := range report.Targets {
		if target.ID == "ci-workflow" {
			t.Fatalf("ci-workflow must not be declared for ci:none with no trackfw-validate.yml on disk, got target: %+v", target)
		}
	}
}

// TestUpdateCiWorkflowClosesDoctorFindingForDiscoverWorkflow is the
// end-to-end proof the remedy stopped being inert: a stale
// trackfw-validate.yml produces a scaffold-divergent doctor finding; after
// `trackfw update` (the exact remedy doctor prints) the SAME project reports
// no mismatches for that path.
func TestUpdateCiWorkflowClosesDoctorFindingForDiscoverWorkflow(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Update(root); err == nil {
		// Update() requires trackfw.yaml — write it first, then scaffold via
		// a real Update() run so every other doctor-checked artifact
		// (scripts, validate-script, etc.) is already current and only the
		// discover workflow under test is stale, isolating the assertion.
		t.Fatal("Update() unexpectedly succeeded before trackfw.yaml existed")
	}
	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update(root); err != nil {
		t.Fatalf("Update (initial scaffold): %v", err)
	}

	workflowPath := filepath.Join(root, DiscoverGitHubActionsWorkflowPath)
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "name: trackfw validate\non: [push, pull_request]\njobs:\n  governance:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v0.0.1\n      - run: trackfw validate\n"
	if err := os.WriteFile(workflowPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := RunScaffoldDoctor(root)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor (before): %v", err)
	}
	foundDivergent := false
	for _, f := range before {
		if f.Destination == DiscoverGitHubActionsWorkflowPath {
			foundDivergent = true
			if f.FindingKind != integrations.DoctorScaffoldDivergent {
				t.Fatalf("expected scaffold-divergent for %s, got %q", DiscoverGitHubActionsWorkflowPath, f.FindingKind)
			}
		}
	}
	if !foundDivergent {
		t.Fatalf("doctor did not flag the stale %s before update — test precondition failed:\n%+v", DiscoverGitHubActionsWorkflowPath, before)
	}

	// The remedy doctor actually prints is bare `trackfw update` — no
	// --targets flag — which the CLI wires to Update(), NOT UpdateProject().
	// This is the exact spot ML-2G's regression escaped review: it proved
	// the fix through UpdateProject (the --targets/--json path) and declared
	// the end-to-end remedy closed without ever exercising the code path the
	// user (and the doctor's own remedy text) actually runs. Asserting
	// through Update() here is what makes this test fail again if that
	// simple path ever stops refreshing the discover workflow.
	if err := Update(root); err != nil {
		t.Fatalf("Update (the remedy doctor prints — bare `trackfw update`): %v", err)
	}

	after, err := RunScaffoldDoctor(root)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor (after): %v", err)
	}
	for _, f := range after {
		if f.Destination == DiscoverGitHubActionsWorkflowPath {
			t.Fatalf("doctor still flags %s after trackfw update — remedy is still inert:\n%+v", DiscoverGitHubActionsWorkflowPath, f)
		}
	}
}

// TestUpdateNeverWritesThroughSymlinkAtDiscoverWorkflowPath is the corrective
// falsifier for the symlink-follow arbitrary-write reported by hades-tf's
// final barrier review (2026-08-28): a project with `ci: none` that has a
// LIVE symlink at .github/workflows/trackfw-validate.yml pointing OUTSIDE
// the project must not have the pointed-to file overwritten by `trackfw
// update`. Before the fix, discoverWorkflowPresent/
// refreshDiscoverGitHubActionsWorkflowIfPresent used os.Stat (follows
// symlinks), so this exact scenario let `update` clobber an arbitrary file
// outside the project tree even though the project opted out of CI
// management entirely.
func TestUpdateNeverWritesThroughSymlinkAtDiscoverWorkflowPath(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root := t.TempDir()
	outside := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "vitima.txt")
	const originalContent = "CONTEUDO ORIGINAL DA VITIMA\n"
	if err := os.WriteFile(victim, []byte(originalContent), 0o644); err != nil {
		t.Fatal(err)
	}
	workflowPath := filepath.Join(root, DiscoverGitHubActionsWorkflowPath)
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, workflowPath); err != nil {
		t.Fatal(err)
	}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	// Exercised through Update(), the bare `trackfw update` command the
	// original report reproduced against — NOT UpdateProject/--targets: for
	// a ci:none project, discoverWorkflowPresent(root) correctly reports the
	// symlink as "not manageable", so UpdateProject never even declares the
	// ci-workflow target here — but Update()'s step 3b calls
	// refreshDiscoverGitHubActionsWorkflowIfPresent(cwd) unconditionally,
	// independent of the declared-targets list, so it must still refuse and
	// warn on its own.
	updateErr := Update(root)

	w.Close()
	os.Stderr = origStderr
	var stderrBuf strings.Builder
	if _, err := io.Copy(&stderrBuf, r); err != nil {
		t.Fatal(err)
	}

	if updateErr != nil {
		t.Fatalf("Update: %v", updateErr)
	}

	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != originalContent {
		t.Fatalf("symlink-follow arbitrary write: victim file outside the project was overwritten.\nwant: %q\ngot:  %q", originalContent, got)
	}

	linkInfo, err := os.Lstat(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to remain a symlink (untouched), got mode %v", workflowPath, linkInfo.Mode())
	}

	if !strings.Contains(stderrBuf.String(), DiscoverGitHubActionsWorkflowPath) || !strings.Contains(stderrBuf.String(), "symlink") {
		t.Fatalf("expected a stderr warning naming %s as a symlink, got: %q", DiscoverGitHubActionsWorkflowPath, stderrBuf.String())
	}

	// A live symlink is not "manageable" by update, so — for a ci:none
	// project — the ci-workflow target must not be declared on its account
	// via the --targets/--json path (UpdateProject), even though the bare
	// `trackfw update` path above (Update()) still ran its own refresh
	// attempt and refused independently.
	report, err := UpdateProject(root, UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	for _, target := range report.Targets {
		if target.ID == "ci-workflow" {
			t.Fatalf("ci-workflow target should not be declared for a ci:none project whose only trackfw-validate.yml is a symlink, got: %+v", target)
		}
	}
}

// TestUpdateNeverWritesThroughDanglingSymlinkAtDiscoverWorkflowPath is the
// same falsifier for the dangling-symlink variant: the link target does not
// exist yet, so a naive os.Stat-based presence check reports "not present"
// and lets os.WriteFile CREATE the file at the attacker-chosen path.
func TestUpdateNeverWritesThroughDanglingSymlinkAtDiscoverWorkflowPath(t *testing.T) {
	config.Reset()
	t.Cleanup(config.Reset)
	root := t.TempDir()
	outside := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(root, "trackfw.yaml"), []byte("hooks: none\nci: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	danglingTarget := filepath.Join(outside, "does-not-exist-yet")
	workflowPath := filepath.Join(root, DiscoverGitHubActionsWorkflowPath)
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(danglingTarget, workflowPath); err != nil {
		t.Fatal(err)
	}

	if err := Update(root); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := UpdateProject(root, UpdateOptions{}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	if _, err := os.Lstat(danglingTarget); !os.IsNotExist(err) {
		t.Fatalf("dangling-symlink arbitrary write: %s was created outside the project (stat err=%v)", danglingTarget, err)
	}
}
