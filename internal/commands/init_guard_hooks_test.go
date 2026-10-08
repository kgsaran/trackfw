package commands

// init_guard_hooks_test.go — ML-1A + ML-1B / REQ-2026-10-07
//
// These tests assert that `trackfw init --ai-tools <cli>` installs the
// git-branch-guard hook for the requested CLI in a single execution (first
// run, HOME isolated). Seven CLIs were affected by the ordering bug; claude
// was not (CLAUDE.md is created by Scaffold before InjectHooksDetected runs).
//
// Falsification contract:
//   - Remove the InjectHooksForTools call from installAITools → 7 subtests
//     fail; the claude subtest still passes (not a meaningful gate).
//   - Remove InjectHooksForTools from the agents-install path → ML-1B test
//     fails.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
)

// hookFileForCLI returns the path to the hook file for a given CLI target ID.
func hookFileForCLI(project, cli string) string {
	switch cli {
	case "claude":
		return filepath.Join(project, ".claude", "settings.json")
	case "codex":
		return filepath.Join(project, ".codex", "hooks.json")
	case "gemini":
		return filepath.Join(project, ".gemini", "settings.json")
	case "kiro":
		return filepath.Join(project, ".kiro", "hooks", "trackfw-attention.json")
	case "copilot":
		return filepath.Join(project, ".github", "hooks", "trackfw-attention.json")
	case "cursor":
		return filepath.Join(project, ".cursor", "hooks.json")
	case "windsurf":
		return filepath.Join(project, ".windsurf", "hooks.json")
	case "amazonq":
		return filepath.Join(project, ".amazonq", "cli-agents", "q_cli_default.json")
	default:
		return ""
	}
}

// assertGuardInHookFile fails the test if the hook file at path is absent or
// does not contain the D11 git-branch-guard marker.
func assertGuardInHookFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("hook file %s missing: %v", path, err)
	}
	if !strings.Contains(string(data), "trackfw guard git-branch") {
		t.Fatalf("hook file %s does not contain 'trackfw guard git-branch':\n%s", path, string(data))
	}
}

// TestInitAIToolsInstallsGuardHooksFirstRun verifies that a single non-TTY
// execution of `trackfw init --ai-tools <cli>` (first run, no pre-existing
// CLI files, HOME isolated) installs the git-branch-guard hook for each of
// the 8 supported CLIs.
//
// Reconciliation (Regra Dura de Reconciliação): this test asserts the fix for
// ML-1A — InjectHooksForTools is called inside installAITools, after the
// CLI's own files are created, so detection-based injection is no longer
// needed for the first-run path.
func TestInitAIToolsInstallsGuardHooksFirstRun(t *testing.T) {
	clis := []string{
		"claude", "codex", "gemini", "kiro",
		"copilot", "cursor", "windsurf", "amazonq",
	}
	for _, cli := range clis {
		cli := cli
		t.Run(cli, func(t *testing.T) {
			project, _ := initFixture(t)
			// git init so Scaffold can derive ProjectName and pathguard is happy.
			if err := exec.Command("git", "init", "-q").Run(); err != nil {
				t.Fatalf("git init: %v", err)
			}

			cmd := newInitCmd()
			cmd.SetArgs([]string{"--ai-tools", cli})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("init --ai-tools %s: %v", cli, err)
			}

			hookPath := hookFileForCLI(project, cli)
			assertGuardInHookFile(t, hookPath)
		})
	}
}

// TestInitAIToolsDoesNotInstallHooksForUnrequestedCLI verifies that
// `init --ai-tools gemini` does not create hook files for other CLIs (cursor,
// kiro). Claude is excluded from this assertion because CLAUDE.md is always
// created by Scaffold (residual R1, Wave 0 §4).
//
// Reconciliation: asserts the fix does NOT introduce false positives —
// InjectHooksForTools only dispatches for tools in the requested list.
func TestInitAIToolsDoesNotInstallHooksForUnrequestedCLI(t *testing.T) {
	project, _ := initFixture(t)
	if err := exec.Command("git", "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "gemini"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --ai-tools gemini: %v", err)
	}

	// Cursor and kiro hook files must NOT exist.
	for _, absent := range []string{
		filepath.Join(project, ".cursor", "hooks.json"),
		filepath.Join(project, ".kiro", "hooks", "trackfw-attention.json"),
	} {
		if _, err := os.Stat(absent); err == nil {
			t.Errorf("hook file should not exist for unrequested CLI: %s", absent)
		}
	}
}

// TestInitKiroFirstRunCreatesHookDir verifies that kiro's hook directory and
// JSON file are created on the first run even without a pre-existing .kiro/
// directory (cause (b) from Wave 0: global scope never creates .kiro/ in the
// project). InjectKiroHooks calls MkdirAll(.kiro/hooks/) itself.
//
// Reconciliation: asserts that the kiro cause (b) fix works — InjectKiroHooks
// is called by name, not by detection, so the absence of .kiro/ before
// install is not a blocker.
func TestInitKiroFirstRunCreatesHookDir(t *testing.T) {
	project, _ := initFixture(t)
	if err := exec.Command("git", "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Confirm .kiro/ does NOT pre-exist.
	if _, err := os.Stat(filepath.Join(project, ".kiro")); err == nil {
		t.Fatal(".kiro/ must not pre-exist for this test to be meaningful")
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--ai-tools", "kiro"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --ai-tools kiro: %v", err)
	}

	hookPath := filepath.Join(project, ".kiro", "hooks", "trackfw-attention.json")
	assertGuardInHookFile(t, hookPath)
}

// TestInitAIToolsKiroHookIdempotent verifies that running `init --ai-tools
// kiro` twice produces a byte-identical hook file on the second run.
// Double-call safety is required because Scaffold's InjectHooksDetected may
// run again on the second init (kiro is now detectable after the first run).
//
// Reconciliation: asserts that InjectKiroHooks is idempotent (os.WriteFile
// + deterministic JSON marshal), so repeated init does not corrupt the file.
func TestInitAIToolsKiroHookIdempotent(t *testing.T) {
	project, _ := initFixture(t)
	if err := exec.Command("git", "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	hookPath := filepath.Join(project, ".kiro", "hooks", "trackfw-attention.json")

	first := newInitCmd()
	first.SetArgs([]string{"--ai-tools", "kiro"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first init: %v", err)
	}
	before, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook after first init: %v", err)
	}

	second := newInitCmd()
	second.SetArgs([]string{"--ai-tools", "kiro"})
	if err := second.Execute(); err != nil {
		t.Fatalf("second init: %v", err)
	}
	after, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook after second init: %v", err)
	}

	if string(before) != string(after) {
		t.Fatalf("hook file changed between two init runs (not idempotent):\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestUpdateAgentHooksOnKiroInitProject_RepairPath verifies that after a
// fixed `init --ai-tools kiro` (which creates .kiro/hooks/), removing the
// hook file but leaving .kiro/ intact, `trackfw update --targets agent-hooks`
// detects kiro and reports "updated". Under the old code (no .kiro/ after
// init), the same repair step would leave kiro undetectable → "skipped".
//
// Reconciliation: asserts that InjectHooksForTools closes the kiro cause (b)
// gap — init now creates .kiro/ so update can detect and re-inject kiro hooks.
func TestUpdateAgentHooksOnKiroInitProject_RepairPath(t *testing.T) {
	project, _ := initFixture(t)
	if err := exec.Command("git", "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Step 1: fixed init creates .kiro/hooks/trackfw-attention.json.
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--ai-tools", "kiro"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init --ai-tools kiro: %v", err)
	}

	hookPath := filepath.Join(project, ".kiro", "hooks", "trackfw-attention.json")
	if _, err := os.Stat(hookPath); err != nil {
		t.Fatalf("hook file missing after fixed init: %v", err)
	}

	// Step 2: simulate repair scenario — remove the hook file, keep .kiro/ dir.
	if err := os.Remove(hookPath); err != nil {
		t.Fatalf("removing hook file: %v", err)
	}

	// Step 3: update --targets agent-hooks must detect .kiro/ and recreate the
	// file, reporting "updated" (hashes changed from "" to non-empty).
	updateCmd := newUpdateCmd()
	updateCmd.SetArgs([]string{"--targets", "agent-hooks"})
	var buf strings.Builder
	updateCmd.SetOut(&buf)
	if err := updateCmd.Execute(); err != nil {
		t.Fatalf("update --targets agent-hooks: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "updated=1") {
		t.Fatalf("expected updated=1 in update output; got:\n%s", out)
	}
	// Confirm the file was actually recreated.
	assertGuardInHookFile(t, hookPath)
}

// TestAgentsInstallInjectsGuardHooks verifies ML-1B: `trackfw agents install
// --targets gemini --scope project` installs the gemini git-branch-guard hook
// in a single execution (first run, project dir without GEMINI.md pre-existing).
//
// Reconciliation: asserts that InjectHooksForTools is called inside the agents
// install path (integrations_flags.go), so the guard hook is written after
// InjectRulesForTool creates GEMINI.md — not just on the second run when
// InjectHooksDetected detects it.
func TestAgentsInstallInjectsGuardHooks(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	if err := exec.Command("git", "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// First: init without gemini so there are no pre-existing gemini files.
	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--forge", "github"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Confirm GEMINI.md does not pre-exist.
	if _, err := os.Stat(filepath.Join(project, "GEMINI.md")); err == nil {
		t.Fatal("GEMINI.md should not pre-exist before agents install for this test to be meaningful")
	}

	// Now install gemini agents.
	agentsCmd := newAgentsCmd()
	agentsCmd.SetArgs([]string{"install", "--targets", "gemini", "--scope", "project"})
	if err := agentsCmd.Execute(); err != nil {
		t.Fatalf("agents install --targets gemini: %v", err)
	}

	hookPath := filepath.Join(project, ".gemini", "settings.json")
	assertGuardInHookFile(t, hookPath)
}

// TestInjectHooksForToolsUnknownNameSkipped verifies that InjectHooksForTools
// silently skips unknown CLI names (e.g. "opencode", "antigravity") without
// error and without writing any hook files.
//
// Reconciliation: asserts the silent-skip contract of InjectHooksForTools for
// CLIs that have no Inject*Hooks implementation.
func TestInjectHooksForToolsUnknownNameSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := generators.InjectHooksForTools([]string{"opencode", "antigravity", "unknown-tool"}, dir); err != nil {
		t.Fatalf("expected no error for unknown tool names, got: %v", err)
	}
	// No files should have been created.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("no files should be created for unknown tools; got: %v", entries)
	}
}
