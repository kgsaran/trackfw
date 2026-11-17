package generators

// Tests for the conditional D5 notice (R-3) and harness TargetFailed on relative
// CLAUDE_CONFIG_DIR (R-1/c), ML-2D, REQ-2026-09-01, ADR-2026-10-14.
//
// Each test asserts a specific conclusion (Regra de Reconciliação, CLAUDE.md):
//
//   (a) macOS/Linux with CLAUDE_CONFIG_DIR set and HOME/.claude/settings.json
//       containing guard wiring: the D5 Message is the CONDITIONAL form —
//       it mentions "CLAUDE_CONFIG_DIR" and does NOT contain "remove it by hand".
//
//   (b) Simulated Windows with HOME≠USERPROFILE and no CLAUDE_CONFIG_DIR:
//       the D5 Message is the UNCONDITIONAL form — it contains "remove it by hand".
//
//   (c) CLAUDE_CONFIG_DIR=../evil or rel: the harness returns TargetFailed for
//       the Claude credential guard target.
//
// Mordida 1 (R-3): revert withD5Notice to always emit the old unconditional message →
//   (a) fails because "remove it by hand" appears in the message.
//
// Mordida 2 (R-1): remove the filepath.IsAbs check from ClaudeConfigDirWithSource →
//   (c) fails because the harness returns TargetUpdated/TargetSkipped instead of
//   TargetFailed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// minimal Claude settings.json with guard wiring (same constant as ml1b test)
const trackfwGuardWiringD5 = `{"hooks":{"PreToolUse":[{"matcher":"Bash|PowerShell","hooks":[{"type":"command","command":"trackfw guard credential"}]}]}}`

// TestHarnessD5Notice_ClaudeConfigDir_ConditionalMessage asserts conclusion (a):
// on macOS/Linux (no Windows HOME≠USERPROFILE divergence), when CLAUDE_CONFIG_DIR
// is set to a directory outside HOME and HOME/.claude/settings.json contains guard
// wiring, the harness TargetResult.Message is the CONDITIONAL form:
//   - contains "CLAUDE_CONFIG_DIR"
//   - does NOT contain "remove it by hand"
//
// Mordida 1: reverting withD5Notice to always use the unconditional message makes
// this test FAIL because "remove it by hand" appears in the old message.
func TestHarnessD5Notice_ClaudeConfigDir_ConditionalMessage(t *testing.T) {
	home := t.TempDir()
	claudeConfigDir := t.TempDir() // outside home — CLAUDE_CONFIG_DIR points here

	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeConfigDir)

	// Pin platform to linux so HomeDivergencePaths() returns divergent=false
	// regardless of the real OS (prevents Windows CI from triggering divergence
	// between t.TempDir() and the real USERPROFILE, which would flip to the
	// unconditional branch and cause this test to fail).
	restorePlatform := agentconfig.SetPlatformForTest("linux", func() (string, error) {
		return home, nil
	})
	defer restorePlatform()

	// Create the OLD guard file at HOME/.claude/settings.json so D5 fires.
	oldClaudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(oldClaudeDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldClaudeDir, "settings.json"),
		[]byte(trackfwGuardWiringD5), 0o644); err != nil {
		t.Fatalf("write old settings.json: %v", err)
	}

	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"claude-credential-guard"},
		InstallMissing: true,
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

	// (a): Message must be the conditional form — mentions CLAUDE_CONFIG_DIR.
	if !strings.Contains(result.Message, "CLAUDE_CONFIG_DIR") {
		t.Errorf("(a): Message missing %q — not the conditional form\nfull message: %q",
			"CLAUDE_CONFIG_DIR", result.Message)
	}
	// (a): Must NOT instruct the user to delete the file.
	if strings.Contains(result.Message, "remove it by hand") {
		t.Errorf("(a): Message contains %q — conditional form must not instruct deletion\nfull message: %q",
			"remove it by hand", result.Message)
	}
}

// TestHarnessD5Notice_WindowsDivergence_UnconditionalMessage asserts conclusion (b):
// on simulated Windows with HOME≠USERPROFILE and no CLAUDE_CONFIG_DIR, the harness
// TargetResult.Message is the UNCONDITIONAL form: it contains "remove it by hand".
//
// Conclusion (b, R-3 ML-2D): when Windows HOME≠USERPROFILE divergence drives the
// path, HOME/.claude is unconditionally inert; this test FAILS if the conditional
// branch fires incorrectly for the divergence case.
func TestHarnessD5Notice_WindowsDivergence_UnconditionalMessage(t *testing.T) {
	homeDir := t.TempDir()
	userprofileDir := t.TempDir()

	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Simulate Windows: ClaudeConfigDir returns USERPROFILE/.claude.
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	// Create the OLD guard at HOME/.claude/settings.json so InertClaudeGuardPath fires.
	oldClaudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(oldClaudeDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldClaudeDir, "settings.json"),
		[]byte(trackfwGuardWiringD5), 0o644); err != nil {
		t.Fatalf("write old settings.json: %v", err)
	}

	// Pre-create USERPROFILE/.claude so the harness can write into it.
	userprofileClaude := filepath.Join(userprofileDir, ".claude")
	if err := os.MkdirAll(userprofileClaude, 0o755); err != nil {
		t.Fatalf("mkdir USERPROFILE/.claude: %v", err)
	}

	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"claude-credential-guard"},
		InstallMissing: true,
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

	// (b): unconditional form — must contain "remove it by hand".
	if !strings.Contains(result.Message, "remove it by hand") {
		t.Errorf("(b): Message missing %q — must use unconditional form for Windows divergence\nfull message: %q",
			"remove it by hand", result.Message)
	}
	// (b): must NOT use the conditional form.
	if strings.Contains(result.Message, "CLAUDE_CONFIG_DIR") {
		t.Errorf("(b): Message contains %q — must NOT use conditional form when divergent\nfull message: %q",
			"CLAUDE_CONFIG_DIR", result.Message)
	}
}

// TestHarnessD5Notice_WindowsDivergence_WithClaudeConfigDir_UnconditionalMessage
// asserts that when Windows HOME≠USERPROFILE divergence is present AND
// CLAUDE_CONFIG_DIR is also set, the D5 message is still the UNCONDITIONAL form
// ("remove it by hand") — NOT the conditional CLAUDE_CONFIG_DIR form.
//
// Rationale: on Windows with divergence, HOME/.claude is unconditionally inert
// because Claude Code reads USERPROFILE/.claude (or CLAUDE_CONFIG_DIR) — the
// "if Claude runs without CLAUDE_CONFIG_DIR, that file is the active one" claim
// in the conditional message would be FALSE. The discriminant is
// (envDir != "" && !divergent), so BOTH conditions must hold.
//
// Conclusion (R-3 ML-2D, &&!divergent): with Windows divergence the unconditional
// form applies even when CLAUDE_CONFIG_DIR is set; this test FAILS if the &&!divergent
// guard is removed (e.g. replaced by `_ = divergent; if envDir != ""`).
func TestHarnessD5Notice_WindowsDivergence_WithClaudeConfigDir_UnconditionalMessage(t *testing.T) {
	homeDir := t.TempDir()
	userprofileDir := t.TempDir()
	claudeConfigDir := t.TempDir()

	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeConfigDir)

	// Simulate Windows divergence: USERPROFILE differs from HOME.
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	// Create the OLD guard at HOME/.claude/settings.json so InertClaudeGuardPath fires.
	oldClaudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(oldClaudeDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldClaudeDir, "settings.json"),
		[]byte(trackfwGuardWiringD5), 0o644); err != nil {
		t.Fatalf("write old settings.json: %v", err)
	}

	// Pre-create claudeConfigDir/.claude... wait, CLAUDE_CONFIG_DIR IS the dir.
	// The harness writes into claudeConfigDir directly (no .claude subdir).
	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"claude-credential-guard"},
		InstallMissing: true,
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

	// Mixed case: CLAUDE_CONFIG_DIR set BUT divergent=true → unconditional form.
	if !strings.Contains(result.Message, "remove it by hand") {
		t.Errorf("(mixed): Message missing %q — must use unconditional form when divergent\nfull message: %q",
			"remove it by hand", result.Message)
	}
	if strings.Contains(result.Message, "inert only for") {
		t.Errorf("(mixed): Message contains %q — conditional form must not fire when divergent\nfull message: %q",
			"inert only for", result.Message)
	}
}

// TestHarnessD5Notice_RelativeCLAUDE_CONFIG_DIR_TargetFailed asserts conclusion (c):
// when CLAUDE_CONFIG_DIR is set to a relative path, the harness returns TargetFailed
// for the Claude credential guard target (R-1 A7).
//
// Conclusion (c, R-1 ML-2D): non-absolute CLAUDE_CONFIG_DIR produces TargetFailed,
// not a silent write; this test FAILS without the filepath.IsAbs check in
// ClaudeConfigDirWithSource.
func TestHarnessD5Notice_RelativeCLAUDE_CONFIG_DIR_TargetFailed(t *testing.T) {
	for _, rel := range []string{"../evil", "rel"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", rel)

			report, err := UpdateHarness(UpdateOptions{
				Targets:        []string{"claude-credential-guard"},
				InstallMissing: true,
				DryRun:         true, // avoid any filesystem write
			})
			if err != nil {
				t.Fatalf("UpdateHarness(%q): unexpected error: %v", rel, err)
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

			// (c): must be TargetFailed — not Updated or Skipped.
			if result.State != TargetFailed {
				t.Errorf("(c) CLAUDE_CONFIG_DIR=%q: want TargetFailed, got %v (message: %q)",
					rel, result.State, result.Message)
			}
			// (c): message must mention the bad path.
			if !strings.Contains(result.Message, rel) {
				t.Errorf("(c): TargetFailed message does not mention %q\nfull message: %q", rel, result.Message)
			}
		})
	}
}

// ---- ML-4A4 R2: Codex D5 inert-copy notice ----

// minimal Codex hooks.json with guard wiring (same schema as Claude settings.json — hookCmdSchemaNested)
const trackfwGuardWiringCodexD5 = `{"hooks":{"PreToolUse":[{"matcher":"Bash|PowerShell","hooks":[{"type":"command","command":"trackfw guard credential --global"}]}]}}`

// TestHarnessD5NoticeCodex_CODEX_HOME_ConditionalMessage asserts ML-4A4 R2 (arm):
// when CODEX_HOME is set to a directory outside HOME and HOME/.codex/hooks.json
// contains guard wiring, the harness TargetResult.Message is the conditional form —
// it mentions "CODEX_HOME" and does NOT contain "remove it by hand".
//
// Conclusion (ML-4A4 R2): withD5NoticeCodex fires for the arm case and sets the
// conditional message; this test FAILS without InertCodexGuardPath or withD5NoticeCodex.
//
// Counter-arm (caminhos iguais → sem aviso): see TestHarnessD5NoticeCodex_SamePaths_NoMessage.
func TestHarnessD5NoticeCodex_CODEX_HOME_ConditionalMessage(t *testing.T) {
	homeDir := t.TempDir()
	codexHomeDir := t.TempDir() // outside HOME — CODEX_HOME points here

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("CODEX_HOME", codexHomeDir)
	defer os.Unsetenv("CODEX_HOME")

	// Pin platform to linux so HomeDivergencePaths() returns divergent=false.
	restorePlatform := agentconfig.SetPlatformForTest("linux", func() (string, error) {
		return homeDir, nil
	})
	defer restorePlatform()

	// Create the OLD guard file at HOME/.codex/hooks.json so InertCodexGuardPath fires.
	oldCodexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(oldCodexDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldCodexDir, "hooks.json"),
		[]byte(trackfwGuardWiringCodexD5), 0o644); err != nil {
		t.Fatalf("write old hooks.json: %v", err)
	}

	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"codex-credential-guard"},
		InstallMissing: true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	var result *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "codex-credential-guard" {
			result = &report.Targets[i]
			break
		}
	}
	if result == nil {
		t.Fatal("no TargetResult for codex-credential-guard")
	}

	// R2 arm: Message must be the conditional form — mentions "CODEX_HOME".
	if !strings.Contains(result.Message, "CODEX_HOME") {
		t.Errorf("R2 arm: Message missing %q — not the conditional form\nfull message: %q",
			"CODEX_HOME", result.Message)
	}
	// R2 arm: Must NOT instruct the user to delete the file.
	if strings.Contains(result.Message, "remove it by hand") {
		t.Errorf("R2 arm: Message contains %q — conditional form must not instruct deletion\nfull message: %q",
			"remove it by hand", result.Message)
	}
}

// TestHarnessD5NoticeCodex_SamePaths_NoMessage asserts ML-4A4 R2 (counter-arm):
// when codexDir == HOME/.codex (same path — no CODEX_HOME), the D5 notice is
// NOT set because the file IS the active guard, not an inert copy.
//
// Conclusion (ML-4A4 R2): equal paths suppress the D5 notice; this test FAILS
// if InertCodexGuardPath omits the same-path check (always fires D5 notice).
func TestHarnessD5NoticeCodex_SamePaths_NoMessage(t *testing.T) {
	homeDir := t.TempDir()

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("CODEX_HOME", "") // no CODEX_HOME — codexDir resolves to HOME/.codex

	// Write the guard at HOME/.codex/hooks.json — this IS the active path, so no D5.
	oldCodexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(oldCodexDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldCodexDir, "hooks.json"),
		[]byte(trackfwGuardWiringCodexD5), 0o644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}

	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"codex-credential-guard"},
		InstallMissing: true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	var result *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "codex-credential-guard" {
			result = &report.Targets[i]
			break
		}
	}
	if result == nil {
		t.Fatal("no TargetResult for codex-credential-guard")
	}

	// R2 counter-arm: Message must NOT contain the D5 notice — paths are identical.
	if strings.Contains(result.Message, "inert") {
		t.Errorf("R2 counter-arm: Message contains D5 notice %q — should be empty when paths are equal\nfull message: %q",
			"inert", result.Message)
	}
}

// ---- ML-4A5: missing D5 Codex coverage (NOTA-1 from hefesto in ML-4A4) ----

// TestHarnessD5NoticeCodex_WindowsDivergence_RemoveItByHand asserts ML-4A5 (arm):
// on simulated Windows with HOME≠USERPROFILE and no CODEX_HOME, the harness
// TargetResult.Message for codex-credential-guard is the UNCONDITIONAL form —
// it contains "remove it by hand" and names the inert old path
// (HOME/.codex/hooks.json).
//
// Conclusion (ML-4A5): when Windows HOME≠USERPROFILE divergence is present and
// CODEX_HOME is not set, withD5NoticeCodex fires the else-branch ("remove it
// by hand"); this test FAILS if the else-branch text is replaced by the
// conditional form, because "remove it by hand" would be absent from the message.
//
// Counter-arm: mutate the else-branch Sprintf in withD5NoticeCodex to drop
// "remove it by hand" → test FAILS.
func TestHarnessD5NoticeCodex_WindowsDivergence_RemoveItByHand(t *testing.T) {
	homeDir := t.TempDir()
	userprofileDir := t.TempDir() // different from homeDir → Windows divergence

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", userprofileDir)
	t.Setenv("CODEX_HOME", "")
	defer os.Unsetenv("CODEX_HOME")

	// Simulate Windows: HomeFor(CLICodex) returns USERPROFILE (via effectiveUserHomeDir),
	// and HomeDivergencePaths() returns divergent=true because HOME≠USERPROFILE.
	restorePlatform := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restorePlatform()

	// Create the OLD guard file at HOME/.codex/hooks.json so InertCodexGuardPath fires.
	// codexDir resolves to USERPROFILE/.codex — different from HOME/.codex — so the
	// old file is inert and the else-branch ("remove it by hand") applies.
	oldCodexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(oldCodexDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldCodexDir, "hooks.json"),
		[]byte(trackfwGuardWiringCodexD5), 0o644); err != nil {
		t.Fatalf("write old hooks.json: %v", err)
	}

	// Pre-create USERPROFILE/.codex so the harness can write the new guard there.
	userprofileCodexDir := filepath.Join(userprofileDir, ".codex")
	if err := os.MkdirAll(userprofileCodexDir, 0o755); err != nil {
		t.Fatalf("mkdir USERPROFILE/.codex: %v", err)
	}

	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"codex-credential-guard"},
		InstallMissing: true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	var result *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "codex-credential-guard" {
			result = &report.Targets[i]
			break
		}
	}
	if result == nil {
		t.Fatal("no TargetResult for codex-credential-guard")
	}

	oldGuardPath := filepath.Join(homeDir, ".codex", "hooks.json")

	// Unconditional form: message must contain "remove it by hand".
	if !strings.Contains(result.Message, "remove it by hand") {
		t.Errorf("Windows divergence: Message missing %q — must use unconditional form\nfull message: %q",
			"remove it by hand", result.Message)
	}
	// Unconditional form: message must name the old inert path.
	if !strings.Contains(result.Message, oldGuardPath) {
		t.Errorf("Windows divergence: Message missing old inert path %q\nfull message: %q",
			oldGuardPath, result.Message)
	}
	// Unconditional form: message must NOT use the conditional form.
	if strings.Contains(result.Message, "CODEX_HOME") {
		t.Errorf("Windows divergence: Message contains %q — conditional form must not fire when divergent\nfull message: %q",
			"CODEX_HOME", result.Message)
	}
}

// TestHarnessD5NoticeCodex_GitBranch_CODEX_HOME_ConditionalMessage asserts ML-4A5:
// when CODEX_HOME is set to a directory outside HOME and HOME/.codex/hooks.json
// contains guard wiring, the harness TargetResult.Message for codex-git-branch-guard
// is the conditional form — it mentions "CODEX_HOME" and does NOT contain
// "remove it by hand".
//
// Conclusion (ML-4A5): withD5NoticeCodex fires for harnessGitBranchGuardTargetCodex
// (not only harnessCredentialGuardTargetCodex); this test FAILS if the
// withD5NoticeCodex call is removed from the post-write return site in
// harnessGitBranchGuardTargetCodex.
//
// Counter-arm: bypass withD5NoticeCodex at the post-write return in
// harnessGitBranchGuardTargetCodex → test FAILS;
// TestHarnessD5NoticeCodex_CODEX_HOME_ConditionalMessage (credential-guard) PASSES
// — proving the new test closes a gap the old one did not.
func TestHarnessD5NoticeCodex_GitBranch_CODEX_HOME_ConditionalMessage(t *testing.T) {
	homeDir := t.TempDir()
	codexHomeDir := t.TempDir() // outside HOME — CODEX_HOME points here

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // no Windows divergence
	t.Setenv("CODEX_HOME", codexHomeDir)
	defer os.Unsetenv("CODEX_HOME")

	// Pin platform to linux so HomeDivergencePaths() returns divergent=false.
	restorePlatform := agentconfig.SetPlatformForTest("linux", func() (string, error) {
		return homeDir, nil
	})
	defer restorePlatform()

	// Create the OLD guard file at HOME/.codex/hooks.json so InertCodexGuardPath fires.
	oldCodexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(oldCodexDir, 0o755); err != nil {
		t.Fatalf("mkdir HOME/.codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldCodexDir, "hooks.json"),
		[]byte(trackfwGuardWiringCodexD5), 0o644); err != nil {
		t.Fatalf("write old hooks.json: %v", err)
	}

	report, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"codex-git-branch-guard"},
		InstallMissing: true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	var result *TargetResult
	for i := range report.Targets {
		if report.Targets[i].ID == "codex-git-branch-guard" {
			result = &report.Targets[i]
			break
		}
	}
	if result == nil {
		t.Fatal("no TargetResult for codex-git-branch-guard")
	}

	// Conditional form: message must mention "CODEX_HOME".
	if !strings.Contains(result.Message, "CODEX_HOME") {
		t.Errorf("GitBranch R2 arm: Message missing %q — not the conditional form\nfull message: %q",
			"CODEX_HOME", result.Message)
	}
	// Conditional form: must NOT instruct the user to delete the file.
	if strings.Contains(result.Message, "remove it by hand") {
		t.Errorf("GitBranch R2 arm: Message contains %q — conditional form must not instruct deletion\nfull message: %q",
			"remove it by hand", result.Message)
	}
}
