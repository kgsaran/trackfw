// Package agentconfig_test exercises the resolver contract for ML-1A (REQ-2026-09-01).
//
// Tests here must NOT write to the real HOME or USERPROFILE — all directories
// are created via t.TempDir(), which is cleaned up automatically.
package agentconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// TestOSCLIHomeResolver_WindowsBranch_Claude exercises OSCLIHomeResolver's Windows
// branch via SetPlatformForTest on macOS/Linux.
//
// Conclusion (ML-1A): on Windows (simulated), HomeFor(CLIClaude) returns
// os.UserHomeDir() — not homedir.Dir(); this test FAILS without the Windows
// branch in OSCLIHomeResolver.HomeFor.
func TestOSCLIHomeResolver_WindowsBranch_Claude(t *testing.T) {
	userprofileDir := t.TempDir()
	homeDir := t.TempDir()

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	// Simulate HOME pointing elsewhere (the homedir.Dir() path).
	t.Setenv("HOME", homeDir)

	r := agentconfig.OSCLIHomeResolver{}
	got, err := r.HomeFor(agentconfig.CLIClaude)
	if err != nil {
		t.Fatalf("HomeFor(CLIClaude): unexpected error: %v", err)
	}
	if got != userprofileDir {
		t.Errorf("HomeFor(CLIClaude) on simulated Windows: got %q, want %q (USERPROFILE)",
			got, userprofileDir)
	}
}

// TestOSCLIHomeResolver_NonClaudeCLIs_Unchanged confirms that Gemini and other
// CLIs (excluding Claude and Codex, which are measured) return homedir.Dir()
// (HOME), not USERPROFILE, even when the platform is simulated as Windows.
//
// Conclusion (ML-1A/ML-4A4): unmeasured CLIs are unaffected by the Windows
// branch; this test would FAIL if HomeFor were changed to return USERPROFILE
// for all CLIs regardless of measurement status.
func TestOSCLIHomeResolver_NonClaudeCLIs_Unchanged(t *testing.T) {
	userprofileDir := t.TempDir()
	homeDir := t.TempDir()

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	t.Setenv("HOME", homeDir)

	r := agentconfig.OSCLIHomeResolver{}
	// CLICodex is now measured (ML-4A4) and uses USERPROFILE on Windows — see
	// TestOSCLIHomeResolver_WindowsBranch_Codex below. Only unmeasured CLIs
	// remain in this test.
	for _, cli := range []string{
		agentconfig.CLIGemini,
		agentconfig.CLICursor,
		agentconfig.CLICopilot,
		agentconfig.CLIKiro,
	} {
		got, err := r.HomeFor(cli)
		if err != nil {
			t.Errorf("HomeFor(%q): unexpected error: %v", cli, err)
			continue
		}
		if got == userprofileDir {
			t.Errorf("HomeFor(%q) on simulated Windows: got USERPROFILE %q, want HOME — unmeasured CLIs must not change (D2)", cli, got)
		}
	}
}

// TestOSCLIHomeResolver_WindowsBranch_Codex exercises OSCLIHomeResolver's
// Windows branch for Codex CLI via SetPlatformForTest on macOS/Linux.
//
// Conclusion (ML-4A4): on Windows (simulated), HomeFor(CLICodex) returns
// os.UserHomeDir() — not homedir.Dir(); this test FAILS without the Codex
// case in the Windows branch of OSCLIHomeResolver.HomeFor.
func TestOSCLIHomeResolver_WindowsBranch_Codex(t *testing.T) {
	userprofileDir := t.TempDir()
	homeDir := t.TempDir()

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	t.Setenv("HOME", homeDir)

	r := agentconfig.OSCLIHomeResolver{}
	got, err := r.HomeFor(agentconfig.CLICodex)
	if err != nil {
		t.Fatalf("HomeFor(CLICodex): unexpected error: %v", err)
	}
	if got != userprofileDir {
		t.Errorf("HomeFor(CLICodex) on simulated Windows: got %q, want %q (USERPROFILE)",
			got, userprofileDir)
	}
}

// TestClaudeConfigDir_CLAUDE_CONFIG_DIR_Honoured confirms that when
// CLAUDE_CONFIG_DIR is set, ClaudeConfigDir returns its clean value (root,
// no .claude/ subdir) regardless of platform or HOME.
//
// Conclusion (ML-1A): CLAUDE_CONFIG_DIR is honoured as the root directory (A1);
// this test FAILS if ClaudeConfigDir ignores CLAUDE_CONFIG_DIR.
func TestClaudeConfigDir_CLAUDE_CONFIG_DIR_Honoured(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", customDir)

	got, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		t.Fatalf("ClaudeConfigDir: unexpected error: %v", err)
	}
	if got != filepath.Clean(customDir) {
		t.Errorf("ClaudeConfigDir with CLAUDE_CONFIG_DIR=%q: got %q, want %q", customDir, got, filepath.Clean(customDir))
	}
}

// TestClaudeConfigDir_Default_JoinsHomeDotClaude confirms the default path is
// HOME/.claude when no CLAUDE_CONFIG_DIR is set and platform is not Windows.
//
// Conclusion (ML-1A): default resolution produces home+".claude"; this test
// FAILS if ClaudeConfigDir stops appending ".claude" on non-Windows.
func TestClaudeConfigDir_Default_JoinsHomeDotClaude(t *testing.T) {
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		t.Fatal(err)
	}

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	got, err := agentconfig.ClaudeConfigDir(agentconfig.FixedCLIHomeResolver{Home: homeDir})
	if err != nil {
		t.Fatalf("ClaudeConfigDir: unexpected error: %v", err)
	}
	want := filepath.Join(homeDir, ".claude")
	if got != want {
		t.Errorf("ClaudeConfigDir default: got %q, want %q", got, want)
	}
}

// TestClaudeConfigDir_Windows_UsesUserprofile confirms that on simulated
// Windows without CLAUDE_CONFIG_DIR, ClaudeConfigDir returns USERPROFILE/.claude.
//
// Conclusion (ML-1A): ClaudeConfigDir uses USERPROFILE on Windows when
// CLAUDE_CONFIG_DIR is absent; this test FAILS if the Windows branch of
// OSCLIHomeResolver is removed.
func TestClaudeConfigDir_Windows_UsesUserprofile(t *testing.T) {
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		t.Fatal(err)
	}

	userprofileDir := t.TempDir()
	homeDir := t.TempDir()

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	t.Setenv("HOME", homeDir)

	got, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		t.Fatalf("ClaudeConfigDir on simulated Windows: unexpected error: %v", err)
	}
	want := filepath.Join(userprofileDir, ".claude")
	if got != want {
		t.Errorf("ClaudeConfigDir on simulated Windows: got %q, want %q (USERPROFILE/.claude)", got, want)
	}
}

// TestClaudeConfigDir_EmptyCLAUDE_CONFIG_DIR_FallsThrough confirms that
// CLAUDE_CONFIG_DIR="" (empty string, as set by gate scripts with
// `CLAUDE_CONFIG_DIR=`) is treated identically to the variable being unset —
// it falls through to the HOME/.claude resolution.
//
// Conclusion (A2, ML-2B REQ-2026-09-01): ClaudeConfigDir ignores an empty
// CLAUDE_CONFIG_DIR and proceeds to the HOME/.claude path; this test FAILS
// if ClaudeConfigDir were changed to use os.LookupEnv (honouring the empty
// string), which would return filepath.Clean("") = "." as the resolved dir.
func TestClaudeConfigDir_EmptyCLAUDE_CONFIG_DIR_FallsThrough(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	homeDir := t.TempDir()

	got, err := agentconfig.ClaudeConfigDir(agentconfig.FixedCLIHomeResolver{Home: homeDir})
	if err != nil {
		t.Fatalf("ClaudeConfigDir with CLAUDE_CONFIG_DIR=\"\": unexpected error: %v", err)
	}
	want := filepath.Join(homeDir, ".claude")
	if got != want {
		t.Errorf("ClaudeConfigDir with CLAUDE_CONFIG_DIR=\"\": got %q, want %q (must fall through to HOME/.claude)", got, want)
	}
}

// TestClaudeConfigDir_RelativeCLAUDE_CONFIG_DIR_ReturnsError asserts conclusion (c-R1):
// when CLAUDE_CONFIG_DIR is set to a relative path (e.g. "../evil" or "rel"),
// ClaudeConfigDir returns a named error — it does NOT silently write to a path
// that can escape the expected root.
//
// Conclusion (R-1, ML-2D): non-absolute CLAUDE_CONFIG_DIR is an A7-failure; this test
// FAILS without the filepath.IsAbs check because filepath.Clean("../evil") = "../evil"
// which is not absolute, yet the old code accepted it and passed it straight to callers.
func TestClaudeConfigDir_RelativeCLAUDE_CONFIG_DIR_ReturnsError(t *testing.T) {
	for _, rel := range []string{"../evil", "rel", "relative/path"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", rel)
			_, err := agentconfig.ClaudeConfigDir(agentconfig.OSCLIHomeResolver{})
			if err == nil {
				t.Errorf("ClaudeConfigDir with CLAUDE_CONFIG_DIR=%q: want error for non-absolute path, got nil", rel)
			}
		})
	}
}

// TestClaudeConfigDir_SpacesOnlyCLAUDE_CONFIG_DIR_FallsThrough asserts conclusion (d):
// when CLAUDE_CONFIG_DIR is set to only whitespace it is treated as "not set" — the
// resolution falls through to the HOME-based path without returning an error.
//
// Conclusion (d, ML-2D): spaces-only CLAUDE_CONFIG_DIR is equivalent to unset; this
// test FAILS if TrimSpace is not applied (a spaces-only value would fail filepath.IsAbs
// and return a spurious error, or would clean to a non-absolute string that errors).
func TestClaudeConfigDir_SpacesOnlyCLAUDE_CONFIG_DIR_FallsThrough(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "   ")

	got, err := agentconfig.ClaudeConfigDir(agentconfig.FixedCLIHomeResolver{Home: homeDir})
	if err != nil {
		t.Fatalf("ClaudeConfigDir with CLAUDE_CONFIG_DIR=\"   \": unexpected error: %v", err)
	}
	want := filepath.Join(homeDir, ".claude")
	if got != want {
		t.Errorf("ClaudeConfigDir with CLAUDE_CONFIG_DIR=\"   \": got %q, want %q (must fall through to HOME/.claude)", got, want)
	}
}

// TestSetPlatformForTest_Restore confirms that the restore function returned by
// SetPlatformForTest correctly reverts to the previous platform inputs.
//
// Conclusion (ML-1A): SetPlatformForTest is correctly scoped; this test
// FAILS if the restore function does not revert the override.
func TestSetPlatformForTest_Restore(t *testing.T) {
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		t.Fatal(err)
	}

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Capture the pre-override result.
	before, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		t.Fatalf("before: %v", err)
	}

	userprofileDir := t.TempDir()
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	restore()

	after, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		t.Fatalf("after restore: %v", err)
	}

	if before != after {
		t.Errorf("after restore: got %q, want %q (same as before override)", after, before)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// CodexConfigDir tests (ML-4A4, ADR-2026-10-14 D2)
// ────────────────────────────────────────────────────────────────────────────

// TestCodexConfigDir_CODEX_HOME_Honoured confirms that when CODEX_HOME is set
// to an absolute path, CodexConfigDir returns its clean value (the root, no
// .codex/ subdir) regardless of platform or HOME.
//
// Conclusion (ML-4A4): CODEX_HOME is honoured as the root directory;
// this test FAILS if CodexConfigDir ignores CODEX_HOME.
func TestCodexConfigDir_CODEX_HOME_Honoured(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("CODEX_HOME", customDir)

	got, err := agentconfig.CodexConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		t.Fatalf("CodexConfigDir: unexpected error: %v", err)
	}
	if got != filepath.Clean(customDir) {
		t.Errorf("CodexConfigDir with CODEX_HOME=%q: got %q, want %q", customDir, got, filepath.Clean(customDir))
	}
}

// TestCodexConfigDir_Default_JoinsHomeDotCodex confirms the default path is
// HOME/.codex when no CODEX_HOME is set and platform is not Windows.
//
// Conclusion (ML-4A4): default resolution produces home+".codex"; this test
// FAILS if CodexConfigDir stops appending ".codex" on non-Windows.
func TestCodexConfigDir_Default_JoinsHomeDotCodex(t *testing.T) {
	t.Setenv("CODEX_HOME", "")

	homeDir := t.TempDir()

	// Force platform to linux to avoid the Windows branch on the Windows CI
	// runner (windows-full-suites uses real GOOS=windows, which would take the
	// os.UserHomeDir() path instead of homedir.Dir()).
	restore := agentconfig.SetPlatformForTest("linux", nil)
	defer restore()

	got, err := agentconfig.CodexConfigDir(agentconfig.FixedCLIHomeResolver{Home: homeDir})
	if err != nil {
		t.Fatalf("CodexConfigDir: unexpected error: %v", err)
	}
	want := filepath.Join(homeDir, ".codex")
	if got != want {
		t.Errorf("CodexConfigDir default: got %q, want %q", got, want)
	}
}

// TestCodexConfigDir_Windows_UsesUserprofile confirms that on simulated
// Windows without CODEX_HOME, CodexConfigDir returns USERPROFILE/.codex.
//
// Conclusion (ML-4A4): CodexConfigDir uses USERPROFILE on Windows when
// CODEX_HOME is absent; this test FAILS if the Windows branch of
// OSCLIHomeResolver is not extended to Codex.
func TestCodexConfigDir_Windows_UsesUserprofile(t *testing.T) {
	t.Setenv("CODEX_HOME", "")

	userprofileDir := t.TempDir()
	homeDir := t.TempDir()

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	t.Setenv("HOME", homeDir)

	got, err := agentconfig.CodexConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		t.Fatalf("CodexConfigDir on simulated Windows: unexpected error: %v", err)
	}
	want := filepath.Join(userprofileDir, ".codex")
	if got != want {
		t.Errorf("CodexConfigDir on simulated Windows: got %q, want %q (USERPROFILE/.codex)", got, want)
	}
}

// TestCodexConfigDir_RelativeCODEX_HOME_ReturnsError asserts that when
// CODEX_HOME is set to a relative path, CodexConfigDir returns a named error.
//
// Conclusion (ML-4A4): non-absolute CODEX_HOME is an A7-failure; this test
// FAILS without the filepath.IsAbs check.
func TestCodexConfigDir_RelativeCODEX_HOME_ReturnsError(t *testing.T) {
	for _, rel := range []string{"../evil", "rel", "relative/path"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			t.Setenv("CODEX_HOME", rel)
			_, err := agentconfig.CodexConfigDir(agentconfig.OSCLIHomeResolver{})
			if err == nil {
				t.Errorf("CodexConfigDir with CODEX_HOME=%q: want error for non-absolute path, got nil", rel)
			}
		})
	}
}

// ─── InertCodexGuardPath tests (ML-4A4 R2) ───────────────────────────────────

// TestInertCodexGuardPath_DifferentDirs_WithGuardMarker asserts the arm: when
// HOME/.codex/hooks.json exists and contains a trackfw guard marker, and
// codexDir points elsewhere, InertCodexGuardPath returns the inert path and
// present=true.
//
// Conclusion (ML-4A4 R2): the D5 inert-remainder detection works for Codex;
// this test FAILS without InertCodexGuardPath or if it uses the wrong file.
func TestInertCodexGuardPath_DifferentDirs_WithGuardMarker(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", "")

	// Write guard marker at HOME/.codex/hooks.json.
	codexSubdir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(codexSubdir, 0755); err != nil {
		t.Fatal(err)
	}
	oldHooks := filepath.Join(codexSubdir, "hooks.json")
	if err := os.WriteFile(oldHooks, []byte(`{"hooks":{"PreToolUse":[{"type":"command","command":"trackfw guard credential --global"}]}}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// codexDir points somewhere else (simulates CODEX_HOME=/other/dir).
	otherDir := t.TempDir()
	path, present := agentconfig.InertCodexGuardPath(otherDir)
	if !present {
		t.Fatalf("InertCodexGuardPath: want present=true (HOME/.codex/hooks.json has guard), got false")
	}
	if path != oldHooks {
		t.Errorf("InertCodexGuardPath: got path %q, want %q", path, oldHooks)
	}
}

// TestInertCodexGuardPath_SameDirs_NoNotice is the counter-arm: when codexDir
// equals HOME/.codex, there is no inert remainder — the file IS the active guard.
//
// Conclusion (ML-4A4 R2): equal paths suppress the D5 notice; this test FAILS
// if InertCodexGuardPath omits the same-path check.
func TestInertCodexGuardPath_SameDirs_NoNotice(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", "")

	codexSubdir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(codexSubdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexSubdir, "hooks.json"), []byte(`{"hooks":{"PreToolUse":[{"type":"command","command":"trackfw guard credential --global"}]}}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// codexDir == HOME/.codex — same directory, no inert remainder.
	_, present := agentconfig.InertCodexGuardPath(codexSubdir)
	if present {
		t.Error("InertCodexGuardPath: want present=false when codexDir == HOME/.codex, got true")
	}
}

// ─── InertClaudeGuardPath tests (ML-FA2) ─────────────────────────────────────

// TestInertClaudeGuardPath_DifferentDirs_WithGuardMarker asserts the arm: when
// HOME/.claude/settings.json exists and contains a trackfw guard marker, and
// claudeDir points elsewhere, InertClaudeGuardPath returns the inert path and
// present=true.
//
// Conclusion (ML-FA2): InertClaudeGuardPath detects the inert remainder correctly
// after switching to fileutil.ReadRegularFile; this test FAILS if the function
// returns present=false for a regular file containing the guard marker.
func TestInertClaudeGuardPath_DifferentDirs_WithGuardMarker(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	claudeSubdir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeSubdir, 0755); err != nil {
		t.Fatal(err)
	}
	oldSettings := filepath.Join(claudeSubdir, "settings.json")
	if err := os.WriteFile(oldSettings, []byte(`{"hooks":{"PreToolUse":[{"type":"command","command":"trackfw guard credential --global"}]}}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// claudeDir points somewhere else (simulates CLAUDE_CONFIG_DIR=/other/dir).
	otherDir := t.TempDir()
	path, present := agentconfig.InertClaudeGuardPath(otherDir)
	if !present {
		t.Fatalf("InertClaudeGuardPath: want present=true (HOME/.claude/settings.json has guard), got false")
	}
	if path != oldSettings {
		t.Errorf("InertClaudeGuardPath: got path %q, want %q", path, oldSettings)
	}
}

// ─── CodexHooksDisplayPath tests (ML-4A4 R1) ─────────────────────────────────

// TestCodexHooksDisplayPath_EmptyDir_FallsBack confirms the empty-dir fallback.
//
// Conclusion (ML-4A4 R1): empty codexDir → "~/.codex/hooks.json"; FAILS if
// the guard is removed.
func TestCodexHooksDisplayPath_EmptyDir_FallsBack(t *testing.T) {
	got := agentconfig.CodexHooksDisplayPath("")
	if got != "~/.codex/hooks.json" {
		t.Errorf("CodexHooksDisplayPath(%q): got %q, want %q", "", got, "~/.codex/hooks.json")
	}
}

// TestCodexHooksDisplayPath_InHome_UsesRelative confirms that a codexDir inside
// HOME produces a ~/ abbreviation.
//
// Conclusion (ML-4A4 R1): path inside HOME is abbreviated; FAILS if Rel is not
// applied.
func TestCodexHooksDisplayPath_InHome_UsesRelative(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", "")

	codexDir := filepath.Join(homeDir, ".codex")
	got := agentconfig.CodexHooksDisplayPath(codexDir)
	if got != "~/.codex/hooks.json" {
		t.Errorf("CodexHooksDisplayPath inside HOME: got %q, want %q", got, "~/.codex/hooks.json")
	}
}

// TestCodexHooksDisplayPath_OutsideHome_UsesAbsolute confirms that a codexDir
// outside HOME returns the absolute path unchanged.
//
// Conclusion (ML-4A4 R1): path outside HOME is NOT abbreviated; FAILS if the
// function always returns "~/.codex/hooks.json".
func TestCodexHooksDisplayPath_OutsideHome_UsesAbsolute(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", "")

	outsideDir := t.TempDir() // different tempdir, sibling to homeDir — outside HOME
	got := agentconfig.CodexHooksDisplayPath(outsideDir)
	want := filepath.Join(outsideDir, "hooks.json")
	if got != want {
		t.Errorf("CodexHooksDisplayPath outside HOME: got %q, want %q", got, want)
	}
}
