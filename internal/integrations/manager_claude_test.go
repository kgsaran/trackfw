// Package integrations_test (white-box) — ML-1D tests for ClaudeDir on Manager.
//
// Each test carries a single-sentence conclusion comment (Regra Dura de
// Reconciliação, CLAUDE.md): the sentence states what the test asserts and
// explains why it FAILS without the fix described in ML-1D (ADR-2026-10-14 D1/A1).
//
// Tests must NOT write to the real HOME, USERPROFILE, or CLAUDE_CONFIG_DIR.
// All directories are isolated via t.TempDir().
//
// SetPlatformForTest is a global, non-goroutine-safe override: no t.Parallel().
package integrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// testClaudePlan builds a PlannedArtifact for a Claude global agent using the
// catalog-canonical path "~/.claude/agents/trackfw-<item>.md".
func testClaudePlan(item, version, content string) PlannedArtifact {
	return PlannedArtifact{
		Claim: Claim{
			Target:  agentconfig.CLIClaude,
			Surface: "cli",
			Scope:   "global",
			Kind:    KindAgents,
			Item:    item,
		},
		Destination:    "~/.claude/agents/trackfw-" + item + ".md",
		Content:        []byte(content),
		CatalogVersion: version,
		SupportLevel:   "native",
	}
}

// TestManagerClaudeDir_WindowsDivergence_AgentLandsUnderCLAUDE_DIR simulates
// Windows divergence (USERPROFILE ≠ HOME) and verifies that Install writes
// the Claude agent file under ClaudeDir (USERPROFILE/.claude/agents/…) and
// NOT under HOME/.claude/agents/….
//
// Conclusion (ML-1D): With ClaudeDir pointing to USERPROFILE/.claude, Install
// writes to USERPROFILE/.claude/agents/… and never to HOME/.claude/agents/…;
// this test FAILS without ClaudeDir set on Manager (manager without ClaudeDir
// writes to HomeDir/.claude/agents/…, which is HOME/.claude on Windows —
// not where Claude Code reads).
func TestManagerClaudeDir_WindowsDivergence_AgentLandsUnderCLAUDE_DIR(t *testing.T) {
	homeDir := t.TempDir()
	userprofileDir := t.TempDir()
	claudeDir := filepath.Join(userprofileDir, ".claude")

	// Simulate Windows: USERPROFILE ≠ HOME.
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", userprofileDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	manager := Manager{
		ProjectRoot: homeDir,
		HomeDir:     homeDir,
		ClaudeDir:   func() (string, error) { return claudeDir, nil },
	}

	plan := testClaudePlan("backend", "v1", "# backend agent")
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Assert: file exists under USERPROFILE/.claude/agents/…
	want := filepath.Join(claudeDir, "agents", "trackfw-backend.md")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("agent file not found at USERPROFILE/.claude path %q: %v", want, err)
	}

	// Assert: file does NOT exist under HOME/.claude/agents/…
	bad := filepath.Join(homeDir, ".claude", "agents", "trackfw-backend.md")
	if _, err := os.Stat(bad); err == nil {
		t.Errorf("agent file unexpectedly written to HOME/.claude path %q (ghost installation)", bad)
	}
}

// TestManagerClaudeDir_CLAUDE_CONFIG_DIR_AgentLandsAtRoot verifies that when
// CLAUDE_CONFIG_DIR is set, Install writes the agent to
// <CLAUDE_CONFIG_DIR>/agents/… (no .claude/ subdir) and not to HomeDir.
//
// Conclusion (ML-1D): With CLAUDE_CONFIG_DIR=/custom, Install writes to
// <custom>/agents/… directly; this test FAILS without ClaudeDir set on Manager
// (the manager without ClaudeDir would write to HomeDir/.claude/agents/…
// regardless of CLAUDE_CONFIG_DIR).
func TestManagerClaudeDir_CLAUDE_CONFIG_DIR_AgentLandsAtRoot(t *testing.T) {
	homeDir := t.TempDir()
	customDir := t.TempDir() // plays the role of CLAUDE_CONFIG_DIR

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	manager := Manager{
		ProjectRoot: homeDir,
		HomeDir:     homeDir,
		ClaudeDir:   func() (string, error) { return customDir, nil }, // simulates CLAUDE_CONFIG_DIR
	}

	plan := testClaudePlan("frontend", "v1", "# frontend agent")
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Assert: file exists under <customDir>/agents/… (no .claude/ subdir)
	want := filepath.Join(customDir, "agents", "trackfw-frontend.md")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("agent file not found at CLAUDE_CONFIG_DIR path %q: %v", want, err)
	}

	// Assert: file does NOT exist under HomeDir/.claude/agents/…
	bad := filepath.Join(homeDir, ".claude", "agents", "trackfw-frontend.md")
	if _, err := os.Stat(bad); err == nil {
		t.Errorf("agent file unexpectedly written to HomeDir/.claude path %q", bad)
	}
}

// TestManagerClaudeDir_NoDiv_PathIdentical verifies that without divergence
// (ClaudeDir == HomeDir/.claude), Install produces the same destination as the
// legacy HomeDir-based expansion — i.e., the fix is a no-op in the common case.
//
// Conclusion (ML-1D): When ClaudeDir is HomeDir/.claude (no divergence), the
// resolved destination is byte-identical to the legacy path; this test FAILS if
// the ClaudeDir logic incorrectly changes the path when HOME == USERPROFILE.
func TestManagerClaudeDir_NoDiv_PathIdentical(t *testing.T) {
	homeDir := t.TempDir()
	claudeDir := filepath.Join(homeDir, ".claude") // same as legacy HOME/.claude

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Manager with ClaudeDir set to HomeDir/.claude — should produce the legacy path.
	newMgr := Manager{
		ProjectRoot: homeDir,
		HomeDir:     homeDir,
		ClaudeDir:   func() (string, error) { return claudeDir, nil },
	}

	// Manager without ClaudeDir — the legacy baseline.
	oldMgr := Manager{
		ProjectRoot: homeDir,
		HomeDir:     homeDir,
	}

	plan := testClaudePlan("backend", "v1", "# backend agent")

	// Resolve via both managers and compare destinations (via Inspect — the plan
	// must be installed first).
	newDir := t.TempDir()
	oldDir := t.TempDir()

	newMgr.ProjectRoot = newDir
	newMgr.HomeDir = newDir
	newMgr.ClaudeDir = func() (string, error) { return filepath.Join(newDir, ".claude"), nil }

	oldMgr.ProjectRoot = oldDir
	oldMgr.HomeDir = oldDir

	if err := newMgr.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("new Install: %v", err)
	}
	if err := oldMgr.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("old Install: %v", err)
	}

	// Both should produce <root>/.claude/agents/trackfw-backend.md.
	wantNew := filepath.Join(newDir, ".claude", "agents", "trackfw-backend.md")
	wantOld := filepath.Join(oldDir, ".claude", "agents", "trackfw-backend.md")

	if _, err := os.Stat(wantNew); err != nil {
		t.Errorf("new manager: file not at expected path %q: %v", wantNew, err)
	}
	if _, err := os.Stat(wantOld); err != nil {
		t.Errorf("old manager: file not at expected path %q: %v", wantOld, err)
	}

	// Control assertion: the manifest stays at root/.trackfw/…, not at ClaudeDir.
	manifestNew := filepath.Join(newDir, ".trackfw", "integrations-manifest.json")
	if _, err := os.Stat(manifestNew); err != nil {
		t.Errorf("manifest not at HomeDir (root) path %q: %v", manifestNew, err)
	}
}

// TestManagerClaudeDir_OtherCLI_Unchanged verifies that a Codex-targeted plan
// resolves using HomeDir (not ClaudeDir) even when ClaudeDir is set.
//
// Conclusion (ML-1D): ClaudeDir only affects claude/global plans; Codex plans
// continue to use HomeDir per ADR-2026-10-14 D2; this test FAILS if ClaudeDir
// were incorrectly applied to non-claude targets.
func TestManagerClaudeDir_OtherCLI_Unchanged(t *testing.T) {
	homeDir := t.TempDir()
	divergedDir := t.TempDir() // simulates a diverged USERPROFILE (only for Claude)

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", divergedDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	manager := Manager{
		ProjectRoot: homeDir,
		HomeDir:     homeDir,
		ClaudeDir:   func() (string, error) { return filepath.Join(divergedDir, ".claude"), nil },
	}

	// Codex global plan — same catalog shape but target=codex.
	codexPlan := PlannedArtifact{
		Claim: Claim{
			Target:  agentconfig.CLICodex,
			Surface: "cli",
			Scope:   "global",
			Kind:    KindAgents,
			Item:    "backend",
		},
		Destination:    "~/.codex/agents/trackfw-backend.md",
		Content:        []byte("# codex backend"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}

	if err := manager.Install([]PlannedArtifact{codexPlan}, false); err != nil {
		t.Fatalf("Install codex plan: %v", err)
	}

	// Assert: Codex file lands under HomeDir (home/.codex/…), not under divergedDir.
	want := filepath.Join(homeDir, ".codex", "agents", "trackfw-backend.md")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("codex agent not at HomeDir path %q: %v", want, err)
	}

	// Assert: Codex file NOT at divergedDir (that would mean ClaudeDir leaked).
	bad := filepath.Join(divergedDir, ".claude", "agents", "trackfw-backend.md")
	if _, err := os.Stat(bad); err == nil {
		t.Errorf("codex agent incorrectly landed at ClaudeDir path %q", bad)
	}
}

// TestManagerClaudeDir_ClaudeConfigDir_EqualsHome verifies that when
// CLAUDE_CONFIG_DIR equals HomeDir (no .claude/ subdir), Install writes the
// agent to <CLAUDE_CONFIG_DIR>/agents/… and NOT to <CLAUDE_CONFIG_DIR>/.claude/agents/….
//
// Conclusion (A1, ML-2B REQ-2026-09-01): when CLAUDE_CONFIG_DIR=HOME the old
// `destRoot != root` condition was false (both equal filepath.Abs(HOME)),
// causing the .claude/ prefix not to be stripped and agents landing at
// HOME/.claude/agents/…; the claudeDirSet boolean fix always strips the prefix
// when ClaudeDir was resolved; this test FAILS without the claudeDirSet fix.
func TestManagerClaudeDir_ClaudeConfigDir_EqualsHome(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("CLAUDE_CONFIG_DIR", homeDir) // CLAUDE_CONFIG_DIR=HOME (no .claude/ subdir)

	manager := Manager{
		ProjectRoot: homeDir,
		HomeDir:     homeDir,
		ClaudeDir:   func() (string, error) { return agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver) },
	}

	plan := testClaudePlan("backend", "v1", "# backend agent")
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Assert: file exists under HOME/agents/… (CLAUDE_CONFIG_DIR=HOME means root=HOME, no .claude/)
	want := filepath.Join(homeDir, "agents", "trackfw-backend.md")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("agent file not found at CLAUDE_CONFIG_DIR(=HOME)/agents path %q: %v", want, err)
	}

	// Assert: file does NOT exist under HOME/.claude/agents/…
	bad := filepath.Join(homeDir, ".claude", "agents", "trackfw-backend.md")
	if _, err := os.Stat(bad); err == nil {
		t.Errorf("agent file incorrectly written to HOME/.claude path %q (old destRoot!=root bug)", bad)
	}
}

// TestManagerClaudeDir_OutsideRoot_Rejected verifies that a Claude global plan
// with an absolute destination outside ClaudeDir is rejected by the beneath guard.
//
// Conclusion (ML-1D): The beneath guard is evaluated against ClaudeDir (not
// HomeDir) for claude/global plans; a destination outside ClaudeDir is refused;
// this test FAILS without the ClaudeDir fix (with HomeDir as root, a destination
// outside HomeDir is also rejected, but this test specifically proves the guard
// uses ClaudeDir by using a destination that is under HomeDir but outside ClaudeDir).
func TestManagerClaudeDir_OutsideRoot_Rejected(t *testing.T) {
	homeDir := t.TempDir()
	claudeDir := t.TempDir() // ClaudeDir is a completely separate dir (simulates CLAUDE_CONFIG_DIR)

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	manager := Manager{
		ProjectRoot: homeDir,
		HomeDir:     homeDir,
		ClaudeDir:   func() (string, error) { return claudeDir, nil },
	}

	// Absolute destination under HomeDir but outside ClaudeDir.
	// With fix: beneath(claudeDir, dest) = false → rejected.
	// Without fix: beneath(homeDir, dest) = true → accepted (regression).
	dest := filepath.Join(homeDir, ".claude", "agents", "trackfw-backend.md")
	plan := PlannedArtifact{
		Claim: Claim{
			Target:  agentconfig.CLIClaude,
			Surface: "cli",
			Scope:   "global",
			Kind:    KindAgents,
			Item:    "backend",
		},
		Destination:    dest, // absolute path
		Content:        []byte("# backend"),
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}

	if err := manager.Install([]PlannedArtifact{plan}, false); err == nil {
		t.Errorf("Install with destination outside ClaudeDir should have been rejected, but succeeded")
	}
}
