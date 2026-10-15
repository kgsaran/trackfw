package validator_test

// validator_guard_global_concordance_external_test.go — ML-1A (REQ-2026-09-01)
//
// Global-scope concordance tests: the update harness and the validator must
// agree on the Claude Code config path (ADR-2026-10-14 D3). Each test plants
// known state, runs the harness and/or validator, and asserts the expected
// result on BOTH sides — writer (harness) and auditor (validator).
//
// Tests (a)–(d) are the mordida battery: they would FAIL if the fix in
// internal/agentconfig were removed (i.e., if the validator reverted to
// homedir.Dir() for Claude Code instead of agentconfig.ClaudeConfigDir).
//
// Tests here must NOT write to the real HOME or USERPROFILE — all directories
// are created via t.TempDir(), which is cleaned up automatically.
//
// Regra de reconciliação (CLAUDE.md): each test carries a one-sentence
// "Conclusion" comment declaring what assertion it makes.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// invalidJSON is planted at a path to ensure the old code (which would read
// that path) produces a "not valid JSON" violation instead of silently skipping
// (file-absent → continue → no violation). This is the mordida anchor.
const invalidJSON = "not-valid-json"

// plantInvalidJSON writes invalidJSON at dir/.claude/settings.json.
// Used to make the old homedir.Dir()-based validator path trip when
// HOME points at dir but CLAUDE_CONFIG_DIR / USERPROFILE point elsewhere.
func plantInvalidJSON(t *testing.T, dir string) {
	t.Helper()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", claudeDir, err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("WriteFile invalid JSON: %v", err)
	}
}

// setupGlobalConcordanceDir creates an isolated environment for global tests:
// sets HOME and USERPROFILE to the same temp dir (control; individual tests
// override USERPROFILE for divergence scenarios). Clears CLAUDE_CONFIG_DIR.
// Does NOT call ChdirForTest — global harness does not require a project cwd.
// Stubs the binary probe so validateGuardGlobal* can parse hook entries.
func setupGlobalConcordanceDir(t *testing.T) (homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // default: HOME == USERPROFILE (control)
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		t.Fatalf("Unsetenv CLAUDE_CONFIG_DIR: %v", err)
	}
	validator.StubProbeOKForTest(t)
	return homeDir
}

// runGlobalHarnessForClaude runs UpdateHarness with InstallMissing:true and targets
// limited to the two Claude global guard targets. Returns the report.
func runGlobalHarnessForClaude(t *testing.T) generators.UpdateReport {
	t.Helper()
	opts := generators.UpdateOptions{
		InstallMissing: true,
		Targets:        []string{"claude-credential-guard", "claude-git-branch-guard"},
	}
	report, err := generators.UpdateHarness(opts)
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}
	return report
}

// assertHarnessTargetUpdated verifies that the named target in the report has
// State == TargetUpdated or TargetSkipped (both mean the harness acted).
func assertHarnessTargetUpdated(t *testing.T, report generators.UpdateReport, targetID string) {
	t.Helper()
	for _, r := range report.Targets {
		if r.ID == targetID {
			if r.State == generators.TargetFailed {
				t.Errorf("harness target %q failed: %s", targetID, r.Message)
			}
			return
		}
	}
	t.Errorf("harness target %q not found in report", targetID)
}

// assertSettingsJSONContainsGuard verifies that dir/settings.json is valid JSON
// containing the "hooks" key and at least one entry with "trackfw guard".
func assertSettingsJSONContainsGuard(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	if _, ok := m["hooks"]; !ok {
		t.Errorf("%s: expected 'hooks' key in JSON", path)
	}
	if !strings.Contains(string(data), "trackfw guard") {
		t.Errorf("%s: expected 'trackfw guard' marker in hooks JSON", path)
	}
}

// assertInvalidJSONUntouched verifies that dir/.claude/settings.json still
// holds invalidJSON (the mordida anchor — proving the harness did NOT write
// to the old path).
func assertInvalidJSONUntouched(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("mordida anchor missing at %s: %v", path, err)
	}
	if string(data) != invalidJSON {
		t.Errorf("mordida anchor at %s was modified by harness: got %q, want %q",
			path, string(data), invalidJSON)
	}
}

// assertZeroGlobalGitBranchViolations fails if the global git-branch-guard
// resolvable rule returns any violation.
func assertZeroGlobalGitBranchViolations(t *testing.T) {
	t.Helper()
	msgs, err := validator.ValidateGitBranchGuardGlobalHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateGitBranchGuardGlobalHookResolvable: unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected zero global git_branch_guard_hook_resolvable violations; got %d:\n%v",
			len(msgs), msgs)
	}
}

// assertZeroGlobalCredentialViolations fails if the global credential-guard
// resolvable rule returns any violation.
func assertZeroGlobalCredentialViolations(t *testing.T) {
	t.Helper()
	msgs, err := validator.ValidateCredentialGuardGlobalHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateCredentialGuardGlobalHookResolvable: unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected zero global credential_guard_hook_resolvable violations; got %d:\n%v",
			len(msgs), msgs)
	}
}

// --------------------------------------------------------------------------
// Test (a): HOME ≠ USERPROFILE (Windows-simulated), no CLAUDE_CONFIG_DIR
//
// Conclusion (ML-1A): harness writes to USERPROFILE/.claude/settings.json and
// validator reads from the same path; both sides agree; this test FAILS without
// the fix because the old harness writes to HOME/.claude (skips due to invalid
// JSON anchor) and the old validator reads HOME/.claude (invalid JSON → violation).
// --------------------------------------------------------------------------

func TestGuardGlobalConcordance_a_HomeNotEqualsUserprofile_GitBranch(t *testing.T) {
	homeDir := setupGlobalConcordanceDir(t)

	// USERPROFILE → distinct dir (simulates Windows divergence).
	userprofileDir := t.TempDir()
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()
	t.Setenv("USERPROFILE", userprofileDir)

	// Mordida anchor: plant invalid JSON at HOME/.claude/settings.json.
	// Old harness reads HOME → fails to install → old validator reads HOME → invalid JSON violation.
	// Fixed code: harness writes to USERPROFILE/.claude, validator reads same → zero violations.
	plantInvalidJSON(t, homeDir)

	report := runGlobalHarnessForClaude(t)

	// Writer-side assertions: harness wrote to USERPROFILE/.claude, NOT to HOME/.claude.
	assertHarnessTargetUpdated(t, report, "claude-git-branch-guard")
	assertSettingsJSONContainsGuard(t, filepath.Join(userprofileDir, ".claude"))
	assertInvalidJSONUntouched(t, homeDir) // HOME/.claude must remain untouched

	// Auditor-side assertion.
	assertZeroGlobalGitBranchViolations(t)
}

func TestGuardGlobalConcordance_a_HomeNotEqualsUserprofile_Credential(t *testing.T) {
	homeDir := setupGlobalConcordanceDir(t)

	userprofileDir := t.TempDir()
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()
	t.Setenv("USERPROFILE", userprofileDir)

	plantInvalidJSON(t, homeDir)
	report := runGlobalHarnessForClaude(t)

	assertHarnessTargetUpdated(t, report, "claude-credential-guard")
	assertSettingsJSONContainsGuard(t, filepath.Join(userprofileDir, ".claude"))
	assertInvalidJSONUntouched(t, homeDir)

	assertZeroGlobalCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Test (b): CLAUDE_CONFIG_DIR set to a custom directory
//
// Conclusion (ML-1A): harness writes to CLAUDE_CONFIG_DIR/settings.json (root,
// no .claude/ subdir) and validator reads from the same path; both sides agree;
// this test FAILS without the fix because the old code ignores CLAUDE_CONFIG_DIR.
// --------------------------------------------------------------------------

func TestGuardGlobalConcordance_b_ClaudeConfigDir_GitBranch(t *testing.T) {
	homeDir := setupGlobalConcordanceDir(t)

	customDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", customDir)

	// Mordida anchor: invalid JSON at HOME/.claude/settings.json.
	plantInvalidJSON(t, homeDir)

	// Harness honours CLAUDE_CONFIG_DIR → writes to customDir/settings.json.
	report := runGlobalHarnessForClaude(t)

	// Writer-side: harness wrote to customDir (root), not to HOME/.claude.
	assertHarnessTargetUpdated(t, report, "claude-git-branch-guard")
	assertSettingsJSONContainsGuard(t, customDir)
	assertInvalidJSONUntouched(t, homeDir)

	// Auditor-side.
	assertZeroGlobalGitBranchViolations(t)
}

func TestGuardGlobalConcordance_b_ClaudeConfigDir_Credential(t *testing.T) {
	homeDir := setupGlobalConcordanceDir(t)

	customDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", customDir)

	plantInvalidJSON(t, homeDir)
	report := runGlobalHarnessForClaude(t)

	assertHarnessTargetUpdated(t, report, "claude-credential-guard")
	assertSettingsJSONContainsGuard(t, customDir)
	assertInvalidJSONUntouched(t, homeDir)

	assertZeroGlobalCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Test (c): control — HOME == USERPROFILE, no CLAUDE_CONFIG_DIR
//
// Conclusion (ML-1A): when HOME and USERPROFILE agree and CLAUDE_CONFIG_DIR
// is absent, the fix writes to and reads from homeDir/.claude — identical to
// the pre-fix behaviour — with zero violations.
// --------------------------------------------------------------------------

func TestGuardGlobalConcordance_c_Control_NoDiv_GitBranch(t *testing.T) {
	homeDir := setupGlobalConcordanceDir(t)
	report := runGlobalHarnessForClaude(t)

	// Writer-side: harness wrote to HOME/.claude (same as USERPROFILE/.claude).
	assertHarnessTargetUpdated(t, report, "claude-git-branch-guard")
	assertSettingsJSONContainsGuard(t, filepath.Join(homeDir, ".claude"))

	assertZeroGlobalGitBranchViolations(t)
}

func TestGuardGlobalConcordance_c_Control_NoDiv_Credential(t *testing.T) {
	homeDir := setupGlobalConcordanceDir(t)
	report := runGlobalHarnessForClaude(t)

	assertHarnessTargetUpdated(t, report, "claude-credential-guard")
	assertSettingsJSONContainsGuard(t, filepath.Join(homeDir, ".claude"))

	assertZeroGlobalCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Test (d): resolution error — A7 (validator emits a named finding on error)
//
// Conclusion (ML-1A): when ClaudeConfigDir returns an error (e.g. USERPROFILE
// absent on Windows), the validator emits a finding naming Claude Code rather
// than silently returning nil; this test FAILS without the A7 branch because
// the old code uses homedir.Dir() (which succeeds) and produces no finding.
// --------------------------------------------------------------------------

func TestGuardGlobalConcordance_d_ResolutionError_GitBranch(t *testing.T) {
	setupGlobalConcordanceDir(t)
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		t.Fatalf("Unsetenv CLAUDE_CONFIG_DIR: %v", err)
	}

	// Simulate Windows with a failing os.UserHomeDir() (USERPROFILE absent).
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return "", fmt.Errorf("USERPROFILE not set")
	})
	defer restore()

	msgs, err := validator.ValidateGitBranchGuardGlobalHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateGitBranchGuardGlobalHookResolvable: unexpected error: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected at least one finding for Claude Code resolution error (A7), got zero")
	}
	// At least one message must mention Claude Code.
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "Claude Code") || strings.Contains(m, "claude") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a finding mentioning Claude Code; got: %v", msgs)
	}
}

func TestGuardGlobalConcordance_d_ResolutionError_Credential(t *testing.T) {
	setupGlobalConcordanceDir(t)
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		t.Fatalf("Unsetenv CLAUDE_CONFIG_DIR: %v", err)
	}

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return "", fmt.Errorf("USERPROFILE not set")
	})
	defer restore()

	msgs, err := validator.ValidateCredentialGuardGlobalHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateCredentialGuardGlobalHookResolvable: unexpected error: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected at least one finding for Claude Code resolution error (A7), got zero")
	}
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "Claude Code") || strings.Contains(m, "claude") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a finding mentioning Claude Code; got: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Test (e): relative CLAUDE_CONFIG_DIR — R-1 A7 finding (ML-2D)
//
// Conclusion (ML-2D R-1): a relative CLAUDE_CONFIG_DIR produces an A7 finding
// naming Claude Code; this test FAILS without the filepath.IsAbs check in
// ClaudeConfigDirWithSource because the old code accepted relative paths and
// the validator would silently skip or write outside the expected root.
// --------------------------------------------------------------------------

func TestGuardGlobalConcordance_e_RelativeCLAUDE_CONFIG_DIR_GitBranch(t *testing.T) {
	for _, rel := range []string{"../evil", "rel"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			setupGlobalConcordanceDir(t)
			t.Setenv("CLAUDE_CONFIG_DIR", rel)

			msgs, err := validator.ValidateGitBranchGuardGlobalHookResolvableForTest()
			if err != nil {
				t.Fatalf("ValidateGitBranchGuardGlobalHookResolvable: unexpected error: %v", err)
			}
			if len(msgs) == 0 {
				t.Errorf("CLAUDE_CONFIG_DIR=%q: expected at least one A7 finding, got zero", rel)
				return
			}
			found := false
			for _, m := range msgs {
				if strings.Contains(m, "Claude Code") || strings.Contains(m, "claude") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("CLAUDE_CONFIG_DIR=%q: expected finding mentioning Claude Code; got: %v", rel, msgs)
			}
		})
	}
}

func TestGuardGlobalConcordance_e_RelativeCLAUDE_CONFIG_DIR_Credential(t *testing.T) {
	for _, rel := range []string{"../evil", "rel"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			setupGlobalConcordanceDir(t)
			t.Setenv("CLAUDE_CONFIG_DIR", rel)

			msgs, err := validator.ValidateCredentialGuardGlobalHookResolvableForTest()
			if err != nil {
				t.Fatalf("ValidateCredentialGuardGlobalHookResolvable: unexpected error: %v", err)
			}
			if len(msgs) == 0 {
				t.Errorf("CLAUDE_CONFIG_DIR=%q: expected at least one A7 finding, got zero", rel)
				return
			}
			found := false
			for _, m := range msgs {
				if strings.Contains(m, "Claude Code") || strings.Contains(m, "claude") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("CLAUDE_CONFIG_DIR=%q: expected finding mentioning Claude Code; got: %v", rel, msgs)
			}
		})
	}
}
