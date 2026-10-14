package generators

// scaffold_global_skill_resolver_ml2c_test.go — ML-2C (REQ-2026-09-01):
// installGlobalSkillInner must follow agentconfig.ClaudeConfigDir so that on
// Windows (HOME ≠ USERPROFILE) the skill is written where Claude Code reads.
//
// Regra Dura de Reconciliação — one sentence per test:
//
//   (a) TestInstallGlobalSkillInner_WindowsDivergence_LandsAtUserProfile:
//       On Windows with HOME≠USERPROFILE, installGlobalSkillInner writes the
//       skill under USERPROFILE/.claude/skills/trackfw/SKILL.md, and
//       HOME/.claude is never created.
//
//   (b) TestInstallGlobalSkillInner_ClaudeConfigDir_LandsAtCustomDir:
//       When CLAUDE_CONFIG_DIR is set, installGlobalSkillInner writes the skill
//       at <CLAUDE_CONFIG_DIR>/skills/trackfw/SKILL.md (no .claude/ segment).
//
//   (c) TestInstallGlobalSkillInner_NoDiv_PathEqualsGlobalClaudeSkillPath:
//       Without Windows divergence and without CLAUDE_CONFIG_DIR,
//       installGlobalSkillInner writes at HOME/.claude/skills/trackfw/SKILL.md,
//       which equals GlobalClaudeSkillPath(home).
//
//   (d) TestInstallGlobalSkillInner_ContainmentRejectsSymlinkOutsideRoot:
//       installGlobalSkillInner rejects a write through a symlinked .claude
//       directory pointing outside the user home, and the victim outside the
//       root stays empty.
//
// Mordida: all four tests exist; on HEAD (before the fix):
//   - (a), (b) fail because installGlobalSkillInner uses homedir.Dir() (HOME)
//     instead of agentconfig.ClaudeConfigDir.
//   - (d) fails because the containment root is HOME-based; a symlink under
//     USERPROFILE/.claude is not checked.
//   - (c) passes (control).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// TestInstallGlobalSkillInner_WindowsDivergence_LandsAtUserProfile asserts
// conclusion (a): On Windows with HOME≠USERPROFILE, the skill is written under
// USERPROFILE/.claude, not under HOME/.claude.
//
// Fails on HEAD (before ML-2C): installGlobalSkillInner calls homedir.Dir()
// which returns HOME → writes to HOME/.claude/skills/…, not USERPROFILE/.claude.
func TestInstallGlobalSkillInner_WindowsDivergence_LandsAtUserProfile(t *testing.T) {
	homeDir := t.TempDir()
	userProfileDir := t.TempDir()

	// Simulate Windows: os.UserHomeDir() returns USERPROFILE (not HOME).
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userProfileDir, nil
	})
	defer restore()

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", userProfileDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	if err := installGlobalSkillInner(true); err != nil {
		t.Fatalf("installGlobalSkillInner: %v", err)
	}

	// SKILL.md must be under USERPROFILE/.claude, not HOME/.claude.
	wantPath := filepath.Join(userProfileDir, ".claude", "skills", "trackfw", "SKILL.md")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("SKILL.md not found at USERPROFILE/.claude: %v", err)
	}

	// HOME/.claude must not have been created at all.
	homeClaudeDir := filepath.Join(homeDir, ".claude")
	if _, err := os.Stat(homeClaudeDir); !os.IsNotExist(err) {
		t.Errorf("HOME/.claude was created (or stat error %v) — skill must NOT be written under HOME on Windows with HOME≠USERPROFILE", err)
	}
}

// TestInstallGlobalSkillInner_ClaudeConfigDir_LandsAtCustomDir asserts
// conclusion (b): When CLAUDE_CONFIG_DIR is set, the skill is written at
// <CLAUDE_CONFIG_DIR>/skills/trackfw/SKILL.md (no .claude/ subdir).
//
// Fails on HEAD (before ML-2C): installGlobalSkillInner ignores CLAUDE_CONFIG_DIR
// and writes to HOME/.claude/skills/…
func TestInstallGlobalSkillInner_ClaudeConfigDir_LandsAtCustomDir(t *testing.T) {
	homeDir := t.TempDir()
	customClaudeDir := t.TempDir()

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("CLAUDE_CONFIG_DIR", customClaudeDir)

	if err := installGlobalSkillInner(true); err != nil {
		t.Fatalf("installGlobalSkillInner: %v", err)
	}

	// SKILL.md must be at <CLAUDE_CONFIG_DIR>/skills/trackfw/SKILL.md.
	wantPath := filepath.Join(customClaudeDir, "skills", "trackfw", "SKILL.md")
	if data, err := os.ReadFile(wantPath); err != nil {
		t.Errorf("SKILL.md not found at CLAUDE_CONFIG_DIR/skills/trackfw/: %v", err)
	} else if len(data) == 0 {
		t.Error("SKILL.md at CLAUDE_CONFIG_DIR is empty")
	}

	// HOME/.claude must not have been created.
	homeClaudeDir := filepath.Join(homeDir, ".claude")
	if _, err := os.Stat(homeClaudeDir); !os.IsNotExist(err) {
		t.Errorf("HOME/.claude was created — CLAUDE_CONFIG_DIR must be honoured without a .claude/ subdir, stat err: %v", err)
	}
}

// TestInstallGlobalSkillInner_NoDiv_PathEqualsGlobalClaudeSkillPath asserts
// conclusion (c): Without Windows divergence and without CLAUDE_CONFIG_DIR, the
// written path equals GlobalClaudeSkillPath(home).
//
// Must pass on HEAD too — this is the control test.
func TestInstallGlobalSkillInner_NoDiv_PathEqualsGlobalClaudeSkillPath(t *testing.T) {
	homeDir := t.TempDir()

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // no divergence
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	if err := installGlobalSkillInner(true); err != nil {
		t.Fatalf("installGlobalSkillInner: %v", err)
	}

	expectedPath := GlobalClaudeSkillPath(homeDir)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("SKILL.md not found at expected path %q: %v", expectedPath, err)
	}
}

// TestInstallGlobalSkillInner_ContainmentRejectsSymlinkOutsideRoot asserts
// conclusion (d): installGlobalSkillInner rejects a write through a symlinked
// .claude directory pointing outside the guard root.
//
// Fails on HEAD (before ML-2C): with SetPlatformForTest("windows", …) the
// guard root was HOME-based; the symlink under USERPROFILE/.claude was not
// checked and the write went through.
func TestInstallGlobalSkillInner_ContainmentRejectsSymlinkOutsideRoot(t *testing.T) {
	homeDir := t.TempDir()
	userProfileDir := t.TempDir()
	outside := t.TempDir() // victim: must stay empty

	// Simulate Windows.
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return userProfileDir, nil
	})
	defer restore()

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", userProfileDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Make userProfileDir/.claude a symlink pointing at outside.
	symlinkOrSkip(t, outside, filepath.Join(userProfileDir, ".claude"))

	err := installGlobalSkillInner(true)
	if err == nil {
		t.Fatal("installGlobalSkillInner() should have been refused when .claude is a symlink pointing outside the root, got nil")
	}

	// outside must remain empty — nothing should have been written through the symlink.
	entries, readErr := os.ReadDir(outside)
	if readErr != nil {
		t.Fatalf("reading outside dir: %v", readErr)
	}
	if len(entries) > 0 {
		t.Errorf("containment violated: %d entry(ies) written to outside dir via symlink", len(entries))
	}
}
