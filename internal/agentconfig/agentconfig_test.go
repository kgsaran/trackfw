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

// TestOSCLIHomeResolver_NonClaudeCLIs_Unchanged confirms that Codex, Gemini
// and other CLIs return homedir.Dir() (HOME), not USERPROFILE, even when the
// platform is simulated as Windows.
//
// Conclusion (ML-1A): non-Claude CLIs are unaffected by the Windows branch;
// this test would FAIL if HomeFor were changed to return USERPROFILE for all CLIs.
func TestOSCLIHomeResolver_NonClaudeCLIs_Unchanged(t *testing.T) {
	userprofileDir := t.TempDir()
	homeDir := t.TempDir()

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userprofileDir, nil
	})
	defer restore()

	t.Setenv("HOME", homeDir)

	r := agentconfig.OSCLIHomeResolver{}
	for _, cli := range []string{
		agentconfig.CLICodex,
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
			t.Errorf("HomeFor(%q) on simulated Windows: got USERPROFILE %q, want HOME — non-Claude CLIs must not change (D2)", cli, got)
		}
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
