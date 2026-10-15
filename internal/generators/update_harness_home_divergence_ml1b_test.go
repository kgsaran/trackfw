package generators

// Tests for D5 inert-remainder notice and displayPath fix (ML-1B, REQ-2026-09-01,
// ADR-2026-10-14).
//
// Each test asserts a specific conclusion (Regra de Reconciliação, CLAUDE.md):
//
//   (e) D5 inert remainder present → TargetResult.Message mentions "inerte" and
//       the old path; HOME/.claude/settings.json is untouched (hash before == after).
//
//   (f) displayPath with CLAUDE_CONFIG_DIR outside HOME → TargetResult.Path shows
//       the real absolute path, not "~/.claude/settings.json".
//
// Mordida 3: harness without D5 report → (e) fails.
//   Removed the withD5Notice wrappers from the Updated/Skipped returns in
//   harnessCredentialGuardTargetClaude and verified that (e) received a
//   TargetResult with empty Message when HOME/.claude/settings.json existed.

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trackfwGuardWiring is a minimal Claude settings.json containing trackfw guard
// wiring — enough for containsTrackfwGuardMarker to return true.
const trackfwGuardWiring = `{"hooks":{"PreToolUse":[{"matcher":"Bash|PowerShell","hooks":[{"type":"command","command":"trackfw guard credential"}]}]}}`

// TestHarnessD5InertRemainderReported asserts conclusion (e): when
// CLAUDE_CONFIG_DIR points to a directory different from HOME/.claude and
// HOME/.claude/settings.json contains trackfw guard wiring, the harness
// TargetResult for "claude-credential-guard" has a non-empty Message mentioning
// "inerte" and the old path; the old file is not deleted or modified.
//
// Mordida 3 falsifies this: without withD5Notice on the Updated/Skipped returns,
// the TargetResult.Message is empty even when the inert file exists.
func TestHarnessD5InertRemainderReported(t *testing.T) {
	home := t.TempDir()
	claudeConfigDir := t.TempDir() // outside home — CLAUDE_CONFIG_DIR points here

	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeConfigDir)

	// Create the OLD guard file at HOME/.claude/settings.json.
	oldClaudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(oldClaudeDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.claude: %v", err)
	}
	oldPath := filepath.Join(oldClaudeDir, "settings.json")
	if err := os.WriteFile(oldPath, []byte(trackfwGuardWiring), 0o644); err != nil {
		t.Fatalf("write old settings.json: %v", err)
	}

	// Hash the old file BEFORE running the harness.
	beforeData, _ := os.ReadFile(oldPath)
	beforeHash := fmt.Sprintf("%x", sha256.Sum256(beforeData))

	// Run the harness targeting only the credential guard; InstallMissing=true
	// so it writes the new file at CLAUDE_CONFIG_DIR/settings.json.
	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"claude-credential-guard"},
		InstallMissing: true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	// Find the claude-credential-guard result.
	var result *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "claude-credential-guard" {
			result = &report.Targets[i]
			break
		}
	}
	if result == nil {
		t.Fatal("no TargetResult for claude-credential-guard")
	}

	// (e): Message must mention "inert" and the old path.
	if !strings.Contains(result.Message, "inert") {
		t.Errorf("TargetResult.Message missing %q\nfull message: %q", "inert", result.Message)
	}
	if !strings.Contains(result.Message, oldPath) {
		t.Errorf("TargetResult.Message missing old path %q\nfull message: %q", oldPath, result.Message)
	}

	// (e): Old file must be untouched — hash must be identical.
	afterData, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("reading old settings.json after harness: %v", err)
	}
	afterHash := fmt.Sprintf("%x", sha256.Sum256(afterData))
	if beforeHash != afterHash {
		t.Errorf("HOME/.claude/settings.json was modified (D5 must be report-only)\nbefore: %s\nafter:  %s",
			beforeHash, afterHash)
	}
}

// TestHarnessDisplayPathWithClaudeConfigDirOutsideHome asserts conclusion (f):
// when CLAUDE_CONFIG_DIR points to a directory NOT under HOME, the TargetResult.Path
// for "claude-credential-guard" shows the real absolute path (not "~/.claude/settings.json").
func TestHarnessDisplayPathWithClaudeConfigDirOutsideHome(t *testing.T) {
	home := t.TempDir()
	claudeConfigDir := t.TempDir() // outside home

	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeConfigDir)

	// Pre-populate CLAUDE_CONFIG_DIR/settings.json with current guard wiring
	// so the harness returns TargetSkipped (idempotent), making Path the display path.
	if err := os.MkdirAll(claudeConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir claudeConfigDir: %v", err)
	}
	// Write a minimal valid settings.json without guard wiring first; the harness
	// will update it and return Updated. Either way, Path must be the real path.
	// Run with DryRun so no filesystem writes happen.
	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"claude-credential-guard"},
		InstallMissing: true,
		DryRun:         true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	var result *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "claude-credential-guard" {
			result = &report.Targets[i]
			break
		}
	}
	if result == nil {
		t.Fatal("no TargetResult for claude-credential-guard")
	}

	// (f): Path must be the real absolute path (not "~/.claude/settings.json").
	expectedPath := filepath.Join(claudeConfigDir, "settings.json")
	if result.Path != expectedPath {
		t.Errorf("Path mismatch:\n  want: %q\n  got:  %q", expectedPath, result.Path)
	}
	if strings.HasPrefix(result.Path, "~") {
		t.Errorf("Path starts with '~' but should be absolute: %q", result.Path)
	}
}
