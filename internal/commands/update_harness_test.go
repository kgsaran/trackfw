package commands

// update_harness_test.go — every test redirects HOME to a t.TempDir() and
// never runs a harness-mutating command against the real user home
// directory (see docs/req/ handoff restriction for ML-6B).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
)

func TestUpdateHarnessCmd_RunsOutsideProjectWithoutTrackfwYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness failed outside a project: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected a text report on stdout")
	}
}

func TestUpdateHarnessCmd_EmptyHarnessExitsZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	cmd.SetOut(&strings.Builder{})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected exit 0 (nil error) for an empty harness, got: %v", err)
	}
}

func TestUpdateHarnessCmd_UnknownTargetIsUsageError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	cmd.SetOut(&strings.Builder{})
	cmd.SetArgs([]string{"--targets", "not-a-real-target"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for an unknown --targets id")
	}
}

// TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract asserts the
// literal key order of the serialized document — not just presence of keys.
// docs/cli-parity.md pins scope, dry_run, targets, summary at the root, and
// id, state, path (message only when present, last) inside each target. This
// mirrors the barrier command's existing key-order regression coverage
// (see docs/cli-parity.md's own warning about the ML-2E gates divergence).
func TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "claude-skill"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	line := strings.TrimSpace(out.String())
	wantRootOrder := []string{`"scope"`, `"dry_run"`, `"targets"`, `"summary"`}
	assertKeyOrder(t, line, wantRootOrder)

	wantTargetOrder := []string{`"id"`, `"state"`, `"path"`}
	assertKeyOrder(t, line, wantTargetOrder)

	// Decode and check shape/values too, not only key order.
	var doc struct {
		Scope   string `json:"scope"`
		DryRun  bool   `json:"dry_run"`
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
		Summary struct {
			Updated int `json:"updated"`
			Skipped int `json:"skipped"`
			Missing int `json:"missing"`
			Failed  int `json:"failed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if doc.Scope != "harness" {
		t.Fatalf("scope = %q, want harness", doc.Scope)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "claude-skill" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "missing" {
		t.Fatalf("state = %q, want missing on an empty harness", doc.Targets[0].State)
	}
	// Summary must always carry all four counters, including zeros.
	if !strings.Contains(line, `"updated":0`) || !strings.Contains(line, `"skipped":0`) ||
		!strings.Contains(line, `"missing":1`) || !strings.Contains(line, `"failed":0`) {
		t.Fatalf("summary must always emit all four counters including zeros: %s", line)
	}
	// A target with no failure must omit "message" entirely — never emit it as "".
	if strings.Contains(line, `"message"`) {
		t.Fatalf("target without a failure must omit \"message\" entirely: %s", line)
	}
}

func TestUpdateHarnessCmd_FailedTargetIncludesMessageAndExitsNonZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Force a failure: make the claude-skill destination a directory instead
	// of a writable file location, so os.ReadFile succeeds reading a dir
	// (fails) and any write attempt fails too.
	skillPath := generators.GlobalClaudeSkillPath(home)
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "claude-skill"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a non-nil error when a target fails")
	}

	line := strings.TrimSpace(out.String())
	if !strings.Contains(line, `"state":"failed"`) {
		t.Fatalf("expected state failed in JSON output: %s", line)
	}
	if !strings.Contains(line, `"message"`) {
		t.Fatalf("failed target must include a message: %s", line)
	}
}

// TestUpdateHarnessCmd_CredentialGuardClaudeInstallsViaCLI exercises the
// claude-credential-guard target through the full `trackfw update harness`
// CLI surface (not just the generators.UpdateHarness API), confirming the
// --targets/--install-missing/--json flags all thread through correctly for
// this new target.
func TestUpdateHarnessCmd_CredentialGuardClaudeInstallsViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "claude-credential-guard", "--install-missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness --targets claude-credential-guard --install-missing failed: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var doc struct {
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "claude-credential-guard" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "updated" {
		t.Fatalf("state = %q, want updated", doc.Targets[0].State)
	}
	if doc.Targets[0].Path != "~/.claude/settings.json" {
		t.Fatalf("path = %q, want ~/.claude/settings.json", doc.Targets[0].Path)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("~/.claude/settings.json was not written: %v", err)
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !strings.Contains(string(data), wantScript) {
		t.Fatalf("settings.json does not reference the absolute global script path %s:\n%s", wantScript, data)
	}
}

// TestUpdateHarnessCmd_CredentialGuardCodexInstallsViaCLI mirrors
// TestUpdateHarnessCmd_CredentialGuardClaudeInstallsViaCLI for the
// codex-credential-guard target (ROADMAP-2026-08-06 Wave 2/ML-2B).
func TestUpdateHarnessCmd_CredentialGuardCodexInstallsViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "codex-credential-guard", "--install-missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness --targets codex-credential-guard --install-missing failed: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var doc struct {
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "codex-credential-guard" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "updated" {
		t.Fatalf("state = %q, want updated", doc.Targets[0].State)
	}
	if doc.Targets[0].Path != "~/.codex/hooks.json" {
		t.Fatalf("path = %q, want ~/.codex/hooks.json", doc.Targets[0].Path)
	}

	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("~/.codex/hooks.json was not written: %v", err)
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !strings.Contains(string(data), wantScript) {
		t.Fatalf("hooks.json does not reference the absolute global script path %s:\n%s", wantScript, data)
	}
}

// TestUpdateHarnessCmd_CredentialGuardGeminiInstallsViaCLI mirrors
// TestUpdateHarnessCmd_CredentialGuardCodexInstallsViaCLI for the
// gemini-credential-guard target (ROADMAP-2026-08-06 Wave 2/ML-2C).
func TestUpdateHarnessCmd_CredentialGuardGeminiInstallsViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "gemini-credential-guard", "--install-missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness --targets gemini-credential-guard --install-missing failed: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var doc struct {
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "gemini-credential-guard" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "updated" {
		t.Fatalf("state = %q, want updated", doc.Targets[0].State)
	}
	if doc.Targets[0].Path != "~/.gemini/settings.json" {
		t.Fatalf("path = %q, want ~/.gemini/settings.json", doc.Targets[0].Path)
	}

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("~/.gemini/settings.json was not written: %v", err)
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !strings.Contains(string(data), wantScript) {
		t.Fatalf("settings.json does not reference the absolute global script path %s:\n%s", wantScript, data)
	}
}

// TestUpdateHarnessCmd_CredentialGuardCursorInstallsViaCLI mirrors
// TestUpdateHarnessCmd_CredentialGuardGeminiInstallsViaCLI for the
// cursor-credential-guard target (ROADMAP-2026-08-06 Wave 2/ML-2D).
func TestUpdateHarnessCmd_CredentialGuardCursorInstallsViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "cursor-credential-guard", "--install-missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness --targets cursor-credential-guard --install-missing failed: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var doc struct {
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "cursor-credential-guard" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "updated" {
		t.Fatalf("state = %q, want updated", doc.Targets[0].State)
	}
	if doc.Targets[0].Path != "~/.cursor/hooks.json" {
		t.Fatalf("path = %q, want ~/.cursor/hooks.json", doc.Targets[0].Path)
	}

	hooksPath := filepath.Join(home, ".cursor", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("~/.cursor/hooks.json was not written: %v", err)
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !strings.Contains(string(data), wantScript) {
		t.Fatalf("hooks.json does not reference the absolute global script path %s:\n%s", wantScript, data)
	}
}

// TestUpdateHarnessCmd_CredentialGuardCopilotInstallsViaCLI mirrors
// TestUpdateHarnessCmd_CredentialGuardCursorInstallsViaCLI for the
// copilot-credential-guard target (ROADMAP-2026-08-06 Wave 2/ML-2E).
func TestUpdateHarnessCmd_CredentialGuardCopilotInstallsViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "copilot-credential-guard", "--install-missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness --targets copilot-credential-guard --install-missing failed: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var doc struct {
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "copilot-credential-guard" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "updated" {
		t.Fatalf("state = %q, want updated", doc.Targets[0].State)
	}
	if doc.Targets[0].Path != "~/.copilot/settings.json" {
		t.Fatalf("path = %q, want ~/.copilot/settings.json", doc.Targets[0].Path)
	}

	settingsPath := filepath.Join(home, ".copilot", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("~/.copilot/settings.json was not written: %v", err)
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !strings.Contains(string(data), wantScript) {
		t.Fatalf("settings.json does not reference the absolute global script path %s:\n%s", wantScript, data)
	}
}

// TestUpdateHarnessCmd_CredentialGuardKiroInstallsViaCLI mirrors
// TestUpdateHarnessCmd_CredentialGuardCopilotInstallsViaCLI for the
// kiro-credential-guard target (ROADMAP-2026-08-06 Wave 2/ML-2F) — a
// dedicated file (~/.kiro/hooks/trackfw-credential-guard.json), not a merge
// into a shared settings file.
func TestUpdateHarnessCmd_CredentialGuardKiroInstallsViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := newUpdateHarnessCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--targets", "kiro-credential-guard", "--install-missing"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trackfw update harness --targets kiro-credential-guard --install-missing failed: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var doc struct {
		Targets []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			Path  string `json:"path"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(line), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if len(doc.Targets) != 1 || doc.Targets[0].ID != "kiro-credential-guard" {
		t.Fatalf("unexpected targets: %+v", doc.Targets)
	}
	if doc.Targets[0].State != "updated" {
		t.Fatalf("state = %q, want updated", doc.Targets[0].State)
	}
	if doc.Targets[0].Path != "~/.kiro/hooks/trackfw-credential-guard.json" {
		t.Fatalf("path = %q, want ~/.kiro/hooks/trackfw-credential-guard.json", doc.Targets[0].Path)
	}

	hookPath := filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("~/.kiro/hooks/trackfw-credential-guard.json was not written: %v", err)
	}
	wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if !strings.Contains(string(data), wantScript) {
		t.Fatalf("trackfw-credential-guard.json does not reference the absolute global script path %s:\n%s", wantScript, data)
	}
}

// TestUpdateHarnessCmd_GitBranchGuardInstallsViaCLI is a table-driven mirror
// of the six TestUpdateHarnessCmd_CredentialGuard<Tool>InstallsViaCLI tests
// above, for the sibling <tool>-git-branch-guard targets (ROADMAP-2026-08-17
// Wave 2/ML-2A). Same 4-state contract, same displayPath per tool, only the
// referenced script differs (trackfw-git-branch-guard.sh instead of
// trackfw-credential-guard.sh) and Kiro gets its OWN dedicated file
// (trackfw-git-branch-guard.json, not a shared trackfw-credential-guard.json
// — see harnessGitBranchGuardTargetKiro's doc comment for why sharing would
// break idempotency).
func TestUpdateHarnessCmd_GitBranchGuardInstallsViaCLI(t *testing.T) {
	cases := []struct {
		tool        string
		relPath     string
		displayPath string
	}{
		{"claude", filepath.Join(".claude", "settings.json"), "~/.claude/settings.json"},
		{"codex", filepath.Join(".codex", "hooks.json"), "~/.codex/hooks.json"},
		{"gemini", filepath.Join(".gemini", "settings.json"), "~/.gemini/settings.json"},
		{"cursor", filepath.Join(".cursor", "hooks.json"), "~/.cursor/hooks.json"},
		{"copilot", filepath.Join(".copilot", "settings.json"), "~/.copilot/settings.json"},
		{"kiro", filepath.Join(".kiro", "hooks", "trackfw-git-branch-guard.json"), "~/.kiro/hooks/trackfw-git-branch-guard.json"},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			targetID := tc.tool + "-git-branch-guard"
			cmd := newUpdateHarnessCmd()
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--json", "--targets", targetID, "--install-missing"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("trackfw update harness --targets %s --install-missing failed: %v", targetID, err)
			}

			line := strings.TrimSpace(out.String())
			var doc struct {
				Targets []struct {
					ID    string `json:"id"`
					State string `json:"state"`
					Path  string `json:"path"`
				} `json:"targets"`
			}
			if err := json.Unmarshal([]byte(line), &doc); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, line)
			}
			if len(doc.Targets) != 1 || doc.Targets[0].ID != targetID {
				t.Fatalf("unexpected targets: %+v", doc.Targets)
			}
			if doc.Targets[0].State != "updated" {
				t.Fatalf("state = %q, want updated", doc.Targets[0].State)
			}
			if doc.Targets[0].Path != tc.displayPath {
				t.Fatalf("path = %q, want %s", doc.Targets[0].Path, tc.displayPath)
			}

			written := filepath.Join(home, tc.relPath)
			data, err := os.ReadFile(written)
			if err != nil {
				t.Fatalf("%s was not written: %v", tc.relPath, err)
			}
			wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
			if !strings.Contains(string(data), wantScript) {
				t.Fatalf("%s does not reference the absolute global script path %s:\n%s", tc.relPath, wantScript, data)
			}

			// Non-regression: credential-guard's own file (shared for the
			// merge-based tools; Kiro's is a separate dedicated file with a
			// different name and is never even touched here) never got its
			// own script reference injected by the git-branch-guard target.
			if tc.tool != "kiro" {
				credScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
				if strings.Contains(string(data), credScript) {
					t.Fatalf("%s unexpectedly references trackfw-credential-guard.sh — git-branch-guard target should not install credential-guard wiring", tc.relPath)
				}
			}
		})
	}
}

// TestUpdateHarnessCmd_GitBranchGuardAndCredentialGuardCoexistIdempotently
// installs both guard targets for Claude (merge-based, shares one file) and
// for Kiro (Kiro's credential-guard writer is wholesale, so it gets its own
// file) twice in a row, proving: (1) both entries land in the expected
// file(s), (2) a second run reports "skipped" for all four targets — the
// idempotency AC this ML is required to paste evidence for.
func TestUpdateHarnessCmd_GitBranchGuardAndCredentialGuardCoexistIdempotently(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	targets := "claude-credential-guard,claude-git-branch-guard,kiro-credential-guard,kiro-git-branch-guard"

	run := func() map[string]string {
		cmd := newUpdateHarnessCmd()
		var out strings.Builder
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--json", "--targets", targets, "--install-missing"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("trackfw update harness --targets %s --install-missing failed: %v", targets, err)
		}
		var doc struct {
			Targets []struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"targets"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &doc); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out.String())
		}
		states := make(map[string]string, len(doc.Targets))
		for _, tgt := range doc.Targets {
			states[tgt.ID] = tgt.State
		}
		return states
	}

	first := run()
	for _, id := range strings.Split(targets, ",") {
		if first[id] != "updated" {
			t.Fatalf("first run: %s state = %q, want updated (%v)", id, first[id], first)
		}
	}

	claudeSettings := filepath.Join(home, ".claude", "settings.json")
	claudeData, err := os.ReadFile(claudeSettings)
	if err != nil {
		t.Fatalf("~/.claude/settings.json not written: %v", err)
	}
	credScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	branchScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	if !strings.Contains(string(claudeData), credScript) || !strings.Contains(string(claudeData), branchScript) {
		t.Fatalf("~/.claude/settings.json missing one of the two guard script references:\n%s", claudeData)
	}
	if strings.Count(string(claudeData), credScript) != 2 { // PreToolUse + PostToolUse
		t.Fatalf("~/.claude/settings.json expected exactly 2 references to %s (Pre+Post), got %d:\n%s", credScript, strings.Count(string(claudeData), credScript), claudeData)
	}
	if strings.Count(string(claudeData), branchScript) != 2 {
		t.Fatalf("~/.claude/settings.json expected exactly 2 references to %s (Pre+Post), got %d:\n%s", branchScript, strings.Count(string(claudeData), branchScript), claudeData)
	}

	kiroCredFile := filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
	kiroBranchFile := filepath.Join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json")
	if _, err := os.Stat(kiroCredFile); err != nil {
		t.Fatalf("kiro credential-guard file not written: %v", err)
	}
	if _, err := os.Stat(kiroBranchFile); err != nil {
		t.Fatalf("kiro git-branch-guard dedicated file not written: %v", err)
	}

	second := run()
	for _, id := range strings.Split(targets, ",") {
		if second[id] != "skipped" {
			t.Fatalf("second run: %s state = %q, want skipped — idempotency broken (%v)", id, second[id], second)
		}
	}

	claudeDataAfter, err := os.ReadFile(claudeSettings)
	if err != nil {
		t.Fatalf("~/.claude/settings.json unreadable after second run: %v", err)
	}
	if string(claudeDataAfter) != string(claudeData) {
		t.Fatalf("~/.claude/settings.json content changed on second (idempotent) run:\nbefore:\n%s\nafter:\n%s", claudeData, claudeDataAfter)
	}
}

func assertKeyOrder(t *testing.T, doc string, keys []string) {
	t.Helper()
	var positions []int
	for _, key := range keys {
		pos := strings.Index(doc, key)
		if pos < 0 {
			t.Fatalf("expected key %s to be present in %s", key, doc)
		}
		positions = append(positions, pos)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] >= positions[i] {
			t.Fatalf("expected key order %v, got JSON with wrong order: %s", keys, doc)
		}
	}
}
