package generators

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ROADMAP-2026-08-17 Wave 2/ML-2B — InjectXHooks (project scope) must skip
// the git-branch-guard entry when the corresponding global-scope wiring
// (installed via `trackfw update harness --targets <tool>-git-branch-guard`,
// ML-2A) is already present, so the guard doesn't fire (and print its block
// message) twice per Bash call — and must fail-open (fall back to the
// pre-ML-2B behavior of always adding the project-scope entry) if the
// global file is missing, unreadable, or unparseable. Mirrors
// credential_guard_dedup_test.go exactly, pointed at the git-branch-guard
// scriptPath/scriptdispatch instead.
// ---------------------------------------------------------------------------

func gbgDedupScriptPath(home string) string {
	return filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
}

func TestGBGDedup_Claude_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": scriptPath}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if helperHasClaudeHook(data, "PreToolUse", "Bash", claudeGitGuardCmd) {
		t.Error("project-scope git-branch-guard entry should have been skipped (global already installed)")
	}
	// credential-guard (global not installed in this fixture) and
	// attention-signal must still be added — dedup is per-guard, not global.
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("credential-guard entry must still be added (its own global dedup was not triggered by this fixture)")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh") {
		t.Error("attention-signal entry must still be added regardless of global git-branch-guard state")
	}
}

func TestGBGDedup_Codex_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".codex", "hooks.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": scriptPath}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	if helperHasClaudeHook(data, "PreToolUse", "Bash", codexGitGuardCmd) {
		t.Error("project-scope git-branch-guard entry should have been skipped (global already installed)")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", codexGuardCmd) {
		t.Error("credential-guard entry must still be added (its own global dedup was not triggered by this fixture)")
	}
}

func TestGBGDedup_Gemini_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".gemini", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"BeforeTool": []interface{}{
				map[string]interface{}{
					"matcher": "run_shell_command",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": scriptPath}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	if helperHasClaudeHook(data, "BeforeTool", "run_shell_command", geminiGitGuardCmd) {
		t.Error("project-scope git-branch-guard entry should have been skipped (global already installed)")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", geminiGuardCmd) {
		t.Error("credential-guard entry must still be added (its own global dedup was not triggered by this fixture)")
	}
}

func TestGBGDedup_Cursor_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".cursor", "hooks.json"), map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"beforeShellExecution": []interface{}{
				map[string]interface{}{"command": scriptPath},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	before, _ := hooks["beforeShellExecution"].([]interface{})
	// credential-guard's global dedup was NOT triggered by this fixture (only
	// the git-branch-guard global entry was planted), so beforeShellExecution
	// must contain exactly the credential-guard entry — git-branch-guard is
	// skipped, credential-guard is not.
	if len(before) != 1 || before[0].(map[string]interface{})["command"] != "scripts/trackfw-credential-guard.sh" {
		t.Errorf("expected only the credential-guard beforeShellExecution entry (git-branch-guard skipped, global already installed), got %v", before)
	}
}

func TestGBGDedup_Cursor_BothGloballyInstalled_KeyAbsentNotEmpty(t *testing.T) {
	home := dedupFixtureHome(t)
	credScriptPath := dedupScriptPath(home)
	gbgScriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".cursor", "hooks.json"), map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"beforeShellExecution": []interface{}{
				map[string]interface{}{"command": credScriptPath},
				map[string]interface{}{"command": gbgScriptPath},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	// Both dedups skip: beforeShellExecution must be ABSENT, not an empty
	// array — check-agent-hooks-parity.sh's structural comparator treats a
	// present-but-empty array as drift against the other two stacks unless
	// all three independently choose to always emit an empty array (they
	// don't — Go never assigns the key when nothing is added to it).
	if _, present := hooks["beforeShellExecution"]; present {
		t.Errorf("expected beforeShellExecution key to be absent when both dedups skip, got %v", hooks["beforeShellExecution"])
	}
}

func TestGBGDedup_Copilot_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".copilot", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{"type": "command", "matcher": "bash", "bash": scriptPath, "cwd": ".", "timeoutSec": 10},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["preToolUse"].([]interface{})
	for _, item := range pre {
		if item.(map[string]interface{})["bash"] == "scripts/trackfw-git-branch-guard.sh" {
			t.Errorf("project-scope git-branch-guard entry should have been skipped (global already installed), got %v", pre)
		}
	}
	foundCG := false
	for _, item := range pre {
		if item.(map[string]interface{})["bash"] == "scripts/trackfw-credential-guard.sh" {
			foundCG = true
		}
	}
	if !foundCG {
		t.Errorf("credential-guard entry must still be added (its own global dedup was not triggered by this fixture), got %v", pre)
	}
}

// TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand
// reproduces ROADMAP-2026-08-17 ML-2C's root cause directly at the dedup
// level (not just the comparator unit test in guard_path_normalize_test.go):
// the "command" value stored in ~/.claude/settings.json is built with raw
// string concatenation (as a hand-edited config, or a $HOME captured with a
// trailing slash before normalization, would produce) instead of
// filepath.Join, so it textually differs from what
// globalGitBranchGuardScriptPath() computes today even though it names the
// SAME file. Before ML-2C this made the dedup silently fail to fire.
func TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand(t *testing.T) {
	home := dedupFixtureHome(t)
	rawStoredCommand := home + "//" + ".trackfw/scripts/trackfw-git-branch-guard.sh"
	helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": rawStoredCommand}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if helperHasClaudeHook(data, "PreToolUse", "Bash", claudeGitGuardCmd) {
		t.Error("project-scope git-branch-guard entry should have been skipped despite the // formatting in the stored global command")
	}
}

// ---------------------------------------------------------------------------
// ROADMAP-2026-08-17 ML-4B — hades-tf ML-4A barrier finding: a global entry
// with the CORRECT command but MISSING "type":"command" (hand-edited config,
// older trackfw version, another tool's merge) is silently never executed by
// Claude Code/Codex/Gemini/GitHub Copilot CLI/Kiro. Before this ML,
// hookArrayHasCommand/simpleArrayHasValue still read such an entry as
// "installed", so the project-scope entry was skipped in favor of a global
// entry that never fires — "nenhum dos dois escopos protege, e tudo fica
// verde". Cursor is the one exception: its schema never carries a "type"
// field at all, so a missing "type" there is normal, not malformed — see
// TestGBGDedup_Cursor_SkipsProjectEntryWhenGlobalInstalled above, whose
// fixture already has no "type" field and must keep skipping.
// ---------------------------------------------------------------------------

func TestGBGDedup_Claude_ReWiresProjectEntryWhenGlobalEntryMissingType(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					// Deliberately missing "type":"command" -- the ML-4A barrier
					// finding's exact malformed shape.
					"hooks": []interface{}{map[string]interface{}{"command": scriptPath}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", claudeGitGuardCmd) {
		t.Error("project-scope git-branch-guard entry should have been RE-WIRED: the global entry is missing \"type\":\"command\" and Claude Code will never execute it, so treating it as \"installed\" would leave both scopes unprotected (hades-tf ML-4A barrier finding)")
	}
}

func TestGBGDedup_Copilot_ReWiresProjectEntryWhenGlobalEntryMissingType(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".copilot", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				// Deliberately missing "type":"command" -- same ML-4A finding,
				// Copilot's own schema.
				map[string]interface{}{"matcher": "bash", "bash": scriptPath, "cwd": ".", "timeoutSec": 10},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["preToolUse"].([]interface{})
	found := false
	for _, item := range pre {
		if item.(map[string]interface{})["bash"] == "scripts/trackfw-git-branch-guard.sh" {
			found = true
		}
	}
	if !found {
		t.Errorf("project-scope git-branch-guard entry should have been RE-WIRED when the global Copilot entry is missing \"type\":\"command\", got %v", pre)
	}
}

// TestGBGDedup_MalformedGlobalEntry_ProjectStillProtects proves the ML-4B fix
// end-to-end via EXECUTION (not just JSON presence), mirroring
// TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled's methodology. The
// malformed global entry (missing "type") is real and present in the
// combined project+global hook set, but a real Claude Code runtime would
// never execute it -- so the "what actually fires" set (built via
// helperClaudeHookCommandsWithType, which filters on type=="command", unlike
// helperClaudeHookCommands above) must exclude it. Before this ML: the
// malformed entry made the dedup skip the project entry AND the malformed
// entry itself never fires -- 0 blocks, "nenhum dos dois escopos protege".
// After this ML: the project entry is re-wired and, being structurally
// valid, executes -- 1 block, protection restored.
func TestGBGDedup_MalformedGlobalEntry_ProjectStillProtects(t *testing.T) {
	home := dedupFixtureHome(t)
	if err := GenerateGlobalGitBranchGuardScript(home); err != nil {
		t.Fatalf("GenerateGlobalGitBranchGuardScript failed: %v", err)
	}
	globalScriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"command": globalScriptPath}},
				},
			},
		},
	})

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := GenerateGitBranchGuardScript(projectDir); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript failed: %v", err)
	}
	if err := InjectClaudeHooks(projectDir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	projectData := helperReadJSON(t, filepath.Join(projectDir, ".claude", "settings.json"))
	globalData := helperReadJSON(t, filepath.Join(home, ".claude", "settings.json"))

	executable := append(
		helperClaudeHookCommandsWithType(t, projectData, "PreToolUse", "Bash"),
		helperClaudeHookCommandsWithType(t, globalData, "PreToolUse", "Bash")...,
	)
	resolved := make([]string, 0, len(executable))
	for _, p := range executable {
		if !strings.Contains(p, "trackfw-git-branch-guard.sh") {
			continue
		}
		resolved = append(resolved, strings.ReplaceAll(p, "$CLAUDE_PROJECT_DIR", projectDir))
	}

	got := runGitBranchGuardEntries(t, projectDir, resolved)
	if got != 1 {
		t.Fatalf("expected exactly 1 block (the re-wired, structurally-valid project entry) -- the malformed global entry must be excluded from what actually executes; got %d (executable entries: %v)", got, resolved)
	}
}

// --- Fail-open: missing/corrupted global file must not disable the
// project-scope git-branch-guard entry. ---

func TestGBGDedup_FailOpen_NoGlobalFile(t *testing.T) {
	dedupFixtureHome(t) // empty $HOME, no global files at all

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", claudeGitGuardCmd) {
		t.Error("expected project-scope git-branch-guard entry to be added when no global file exists (fail-open)")
	}
}

func TestGBGDedup_FailOpen_CorruptedGlobalFile(t *testing.T) {
	home := dedupFixtureHome(t)
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", claudeGitGuardCmd) {
		t.Error("expected project-scope git-branch-guard entry to be added when global file is corrupted (fail-open)")
	}
}

// ---------------------------------------------------------------------------
// "Message once" — proved by EXECUTING the generated hook file's entries
// (not by counting JSON entries), per the architect's explicit AC3 wording
// ("prove executando, não por contagem de entradas no JSON"). Simulates what
// Claude Code actually does: invoke every command registered under the
// PreToolUse/Bash matcher once per Bash tool call.
// ---------------------------------------------------------------------------

// runGitBranchGuardEntries invokes each of the given script paths (as argv,
// "<script> git push") against a project directory containing trackfw.yaml
// (so the ML-1A no-op does not swallow the invocation), and returns the
// number of entries that actually blocked (exit 2 + the block reason on
// stderr) — i.e. how many times a real agent runtime invoking every
// registered PreToolUse/Bash entry would show the user a block message for
// a single `git push`. Counts blocking INVOCATIONS, not substring
// occurrences in combined output — the script emits the same reason on both
// stdout (JSON, machine-consumed) and stderr (human-readable) per
// invocation, so a raw text-occurrence count would double-count a single
// block.
func runGitBranchGuardEntries(t *testing.T, projectDir string, scriptPaths []string) int {
	t.Helper()
	count := 0
	for _, script := range scriptPaths {
		cmd := exec.Command("bash", script, "git", "push")
		cmd.Dir = projectDir
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()
		exitErr, isExitErr := err.(*exec.ExitError)
		blocked := isExitErr && exitErr.ExitCode() == 2 && strings.Contains(stderr.String(), "git push bruto bloqueado")
		if err == nil && !blocked {
			// exited 0: allow, not a failure of the harness.
			continue
		}
		if blocked {
			count++
		} else {
			t.Fatalf("unexpected script outcome for %s: err=%v stderr=%s", script, err, stderr.String())
		}
	}
	return count
}

func TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled(t *testing.T) {
	home := dedupFixtureHome(t)

	// Write the real global script content (byte-identical to project scope,
	// per GenerateGlobalGitBranchGuardScript's own doc comment) and wire the
	// global-scope Claude config exactly the way harnessGitBranchGuardTargetClaude
	// (ML-2A) would.
	if err := GenerateGlobalGitBranchGuardScript(home); err != nil {
		t.Fatalf("GenerateGlobalGitBranchGuardScript failed: %v", err)
	}
	globalScriptPath := gbgDedupScriptPath(home)
	helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": globalScriptPath}},
				},
			},
		},
	})

	// Project scaffold: needs trackfw.yaml so the ML-1A no-op doesn't
	// swallow the invocation, plus its own project-scope script (generated
	// the same way `trackfw init` would).
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := GenerateGitBranchGuardScript(projectDir); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript failed: %v", err)
	}

	if err := InjectClaudeHooks(projectDir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	// Claude Code reads and merges hook entries from BOTH the project-scope
	// (.claude/settings.json) and global-scope (~/.claude/settings.json)
	// files for a single Bash tool call — the dedup fix (ML-2B) is only
	// correct if the COMBINED set of entries across both files has exactly
	// one git-branch-guard command, not just the project file in isolation
	// (the global file's entry is real and expected to be there).
	projectData := helperReadJSON(t, filepath.Join(projectDir, ".claude", "settings.json"))
	globalData := helperReadJSON(t, filepath.Join(home, ".claude", "settings.json"))
	scriptPaths := append(
		helperClaudeHookCommands(t, projectData, "PreToolUse", "Bash"),
		helperClaudeHookCommands(t, globalData, "PreToolUse", "Bash")...,
	)
	// The dedup fix (ML-2B) must leave exactly ONE Bash-matcher entry
	// pointing at a git-branch-guard script across the combined project+
	// global hook set — the global one; the project-scope entry was
	// skipped.
	gitGuardEntries := 0
	for _, p := range scriptPaths {
		if strings.Contains(p, "trackfw-git-branch-guard.sh") {
			gitGuardEntries++
		}
	}
	if gitGuardEntries != 1 {
		t.Fatalf("expected exactly 1 git-branch-guard entry across project+global PreToolUse/Bash after dedup, got %d: %v", gitGuardEntries, scriptPaths)
	}

	// Resolve $CLAUDE_PROJECT_DIR the way Claude Code would before invoking
	// the hook command, then execute every Bash-matcher entry once (as a
	// real agent runtime would per Bash tool call) and count how many times
	// the block message appears.
	resolved := make([]string, 0, len(scriptPaths))
	for _, p := range scriptPaths {
		if !strings.Contains(p, "trackfw-git-branch-guard.sh") {
			continue
		}
		resolved = append(resolved, strings.ReplaceAll(p, "$CLAUDE_PROJECT_DIR", projectDir))
	}

	got := runGitBranchGuardEntries(t, projectDir, resolved)
	if got != 1 {
		t.Errorf("expected the block message to appear exactly once when executing every registered PreToolUse/Bash git-branch-guard entry, got %d (entries executed: %v)", got, resolved)
	}
}

// TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled_NonVacuous proves
// the test above is not vacuous: with the SAME fixture but the dedup
// deliberately bypassed (both the project-scope AND a second, synthetic
// global-scope entry executed, simulating the pre-ML-2B double-wiring bug),
// the block message appears twice — the exact production symptom
// (`docs/roadmaps/wip/ROADMAP-2026-08-17-...md` ML-2B "Impacto medido").
func TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled_NonVacuous(t *testing.T) {
	home := dedupFixtureHome(t)
	if err := GenerateGlobalGitBranchGuardScript(home); err != nil {
		t.Fatalf("GenerateGlobalGitBranchGuardScript failed: %v", err)
	}
	globalScriptPath := gbgDedupScriptPath(home)

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := GenerateGitBranchGuardScript(projectDir); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript failed: %v", err)
	}
	projectScriptPath := filepath.Join(projectDir, "scripts", "trackfw-git-branch-guard.sh")

	// Simulates the pre-ML-2B state directly (both entries present,
	// unconditionally) instead of going through the injector, since the
	// injector is exactly the thing under test in the scenario above.
	got := runGitBranchGuardEntries(t, projectDir, []string{projectScriptPath, globalScriptPath})
	if got != 2 {
		t.Fatalf("non-vacuity check failed: expected the double-wiring symptom (message twice) with both entries present, got %d — the test harness itself is broken, not proving anything about the dedup fix", got)
	}
}

// helperClaudeHookCommands extracts every "command" string registered under
// hooks[event][matcher==matcher].hooks[].command in a parsed Claude/Codex/
// Gemini-shaped settings JSON.
func helperClaudeHookCommands(t *testing.T, data map[string]interface{}, event, matcher string) []string {
	t.Helper()
	var out []string
	hooks, _ := data["hooks"].(map[string]interface{})
	arr, _ := hooks[event].([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		inner, _ := obj["hooks"].([]interface{})
		for _, h := range inner {
			hObj, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, ok := hObj["command"].(string); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}

// helperClaudeHookCommandsWithType is helperClaudeHookCommands' ML-4B
// counterpart: it only returns commands from entries that ALSO carry
// "type":"command" — i.e. it models what a real Claude Code runtime would
// actually execute, not merely what textually references a script. Used by
// TestGBGDedup_MalformedGlobalEntry_ProjectStillProtects to exclude a
// malformed (type-less) entry from the "what fires" set even though it is
// present in the combined project+global hook set.
func helperClaudeHookCommandsWithType(t *testing.T, data map[string]interface{}, event, matcher string) []string {
	t.Helper()
	var out []string
	hooks, _ := data["hooks"].(map[string]interface{})
	arr, _ := hooks[event].([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		inner, _ := obj["hooks"].([]interface{})
		for _, h := range inner {
			hObj, ok := h.(map[string]interface{})
			if !ok || hObj["type"] != "command" {
				continue
			}
			if cmd, ok := hObj["command"].(string); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}
