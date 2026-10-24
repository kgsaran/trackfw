package generators

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D15 (ADR-2026-10-04 adendo 2026-10-24, REQ-2026-10-23 #613) —
// InjectXHooks (project scope) ALWAYS emits the git-branch-guard entry
// regardless of whether the global-scope wiring is installed. The old D14
// dedup behavior is removed. Tests below assert presence (not absence) when
// a global file is present, and the "message once" test now expects 2 blocks
// when both project and global scopes are wired.
// ---------------------------------------------------------------------------

func gbgDedupScriptPath(home string) string {
	return filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
}

// Reconciliação (D15): afirma que InjectClaudeHooks SEMPRE emite git-branch-guard
// mesmo com o harness global instalado. A semente existe para provar que o código
// não a lê para pular a fiação de projeto.
func TestProjectGitBranchGuard_Claude_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
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
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardGitBranchCmdPSPOSIX) {
		t.Error("project-scope git-branch-guard entry must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX) {
		t.Error("credential-guard entry must still be added")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", attentionSignalCmd) {
		t.Error("attention-signal entry must still be added")
	}
}

// Reconciliação (D15): Codex — idem Claude.
func TestProjectGitBranchGuard_Codex_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
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
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", guardGitBranchCmdPSPOSIX) {
		t.Error("project-scope git-branch-guard entry must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX) {
		t.Error("credential-guard entry must still be added")
	}
}

// Reconciliação (D15): Gemini — idem Claude.
func TestProjectGitBranchGuard_Gemini_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
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
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", guardGitBranchCmdPSPOSIX) {
		t.Error("project-scope git-branch-guard entry must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", guardCredentialCmdPSPOSIX) {
		t.Error("credential-guard entry must still be added")
	}
}

// Reconciliação (D15): Cursor — both beforeShellExecution entries must be present.
func TestProjectGitBranchGuard_Cursor_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
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
	// D15: both credential-guard and git-branch-guard must be present.
	foundCred, foundGit := false, false
	for _, item := range before {
		obj, _ := item.(map[string]interface{})
		switch obj["command"] {
		case guardCredentialCmdPSPOSIX:
			foundCred = true
		case guardGitBranchCmdPSPOSIX:
			foundGit = true
		}
	}
	if !foundCred {
		t.Errorf("credential-guard must be present in beforeShellExecution (D15), got %v", before)
	}
	if !foundGit {
		t.Errorf("git-branch-guard must be present in beforeShellExecution even when global is installed (D15), got %v", before)
	}
}

// Reconciliação (D15): afirma que, com ambos os guards globais instalados para Cursor,
// beforeShellExecution contém tanto credential-guard quanto git-branch-guard (D15: sempre emitidos).
func TestProjectGitBranchGuard_Cursor_BothGloballyInstalled_BothKeysPresent(t *testing.T) {
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
	// D15: beforeShellExecution key must be PRESENT with both entries.
	before, _ := hooks["beforeShellExecution"].([]interface{})
	if _, present := hooks["beforeShellExecution"]; !present {
		t.Error("beforeShellExecution key must be present (D15: both guards always emitted)")
	}
	foundCred, foundGit := false, false
	for _, item := range before {
		obj, _ := item.(map[string]interface{})
		switch obj["command"] {
		case guardCredentialCmdPSPOSIX:
			foundCred = true
		case guardGitBranchCmdPSPOSIX:
			foundGit = true
		}
	}
	if !foundCred {
		t.Errorf("credential-guard must be present (D15), got %v", before)
	}
	if !foundGit {
		t.Errorf("git-branch-guard must be present (D15), got %v", before)
	}
}

// Reconciliação (D15): Copilot — git-branch-guard must be present even with global.
func TestProjectGitBranchGuard_Copilot_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
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
	foundGit := false
	for _, item := range pre {
		if item.(map[string]interface{})["command"] == guardGitBranchCmdPSPOSIX {
			foundGit = true
		}
	}
	if !foundGit {
		t.Errorf("project-scope git-branch-guard must be present even when global is installed (D15), got %v", pre)
	}
	foundCG := false
	for _, item := range pre {
		if item.(map[string]interface{})["command"] == guardCredentialCmdPSPOSIX {
			foundCG = true
		}
	}
	if !foundCG {
		t.Errorf("credential-guard entry must still be added, got %v", pre)
	}
}

// TestGBGDedup_Claude_EmitsProjectEntry_WithDoubleSlashInGlobalCommand (D15):
// the global command with // in the path is a seed that proves the code no
// longer reads the global file to skip project wiring. The guard must be
// present in the project file regardless.
func TestProjectGitBranchGuard_Claude_EmitsProjectEntry_WithDoubleSlashInGlobalCommand(t *testing.T) {
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
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardGitBranchCmdPSPOSIX) {
		t.Error("project-scope git-branch-guard must be present regardless of global state (D15)")
	}
}

// D15: The "rewire" and "fail-open" concepts no longer apply — project wiring is
// unconditional; the global file is never consulted. Tests that covered those
// pre-D15 paths (ReWiresProjectEntryWhenGlobalEntryMissingType,
// MalformedGlobalEntry_ProjectStillProtects, FailOpen_NoGlobalFile,
// FailOpen_CorruptedGlobalFile) have been removed. The EmitsProjectEntry*
// tests above cover the same protection assertions under D15 semantics.

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
		var cmd *exec.Cmd
		if strings.Contains(script, "trackfw guard git-branch") {
			// inline command form (ML-2A): simulate agent runtime invoking the
			// hook with "git push" as the blocked command. The inline hook does
			// not receive positional args, so we pass via --command.
			cmd = exec.Command("bash", "-c", `trackfw guard git-branch --command "git push"; exit $?`)
		} else {
			cmd = exec.Command("bash", script, "git", "push")
		}
		// Both the inline command form and .sh thin-wrapper scripts invoke
		// `trackfw guard git-branch`. Inject the binary compiled by TestMain so
		// CI (where no system trackfw is installed) can resolve it. Without this,
		// wrapper scripts exit 2 with "trackfw not found in PATH" before ever
		// reaching the blocking logic.
		cmd.Env = injectGuardBinaryPath(t, os.Environ())
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

// TestProjectGitBranchGuard_BothScopesFireIndependently_TwoMessages (D15): under D15,
// InjectClaudeHooks ALWAYS emits the project-scope entry even when a global
// entry exists. Claude Code merges both project and global scope hooks for a
// single Bash tool call, so the combined set has TWO git-branch-guard entries
// (project under claudeShellMatcher + global under "Bash"). Both execute and
// produce the block message — 2 blocks is the expected, accepted cost of D15
// (ADR §D15: ~10.6 ms on a dual-scope machine). The NonVacuous test below
// confirms 2 blocks is achievable.
//
// Reconciliação (D15): afirma que projeto + global emitem 2 entradas independentes de
// git-branch-guard, resultando em 2 bloqueios (custo aceito por D15).
func TestProjectGitBranchGuard_BothScopesFireIndependently_TwoMessages(t *testing.T) {
	home := dedupFixtureHome(t)

	// Seed global scope exactly the way harnessGitBranchGuardTargetClaude would.
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

	// Collect entries from both matchers: project uses claudeShellMatcher
	// ("Bash|PowerShell"), global uses "Bash". Combined must have exactly 2
	// git-branch-guard entries.
	projectData := helperReadJSON(t, filepath.Join(projectDir, ".claude", "settings.json"))
	globalData := helperReadJSON(t, filepath.Join(home, ".claude", "settings.json"))
	scriptPaths := append(
		helperClaudeHookCommands(t, projectData, "PreToolUse", claudeShellMatcher),
		helperClaudeHookCommands(t, globalData, "PreToolUse", "Bash")...,
	)
	gitGuardEntries := 0
	for _, p := range scriptPaths {
		if strings.Contains(p, "trackfw-git-branch-guard.sh") || strings.Contains(p, "trackfw guard git-branch") {
			gitGuardEntries++
		}
	}
	if gitGuardEntries != 2 {
		t.Fatalf("D15: expected exactly 2 git-branch-guard entries across project+global (1 project under %q + 1 global under \"Bash\"), got %d: %v", claudeShellMatcher, gitGuardEntries, scriptPaths)
	}

	resolved := make([]string, 0, len(scriptPaths))
	for _, p := range scriptPaths {
		if !strings.Contains(p, "trackfw-git-branch-guard.sh") && !strings.Contains(p, "trackfw guard git-branch") {
			continue
		}
		resolved = append(resolved, strings.ReplaceAll(p, "$CLAUDE_PROJECT_DIR", projectDir))
	}

	got := runGitBranchGuardEntries(t, projectDir, resolved)
	if got != 2 {
		t.Errorf("D15: expected 2 blocks (both project and global scopes fire independently), got %d (entries executed: %v)", got, resolved)
	}
}

// TestProjectGitBranchGuard_BothScopesFireIndependently_TwoMessages_NonVacuous proves
// the test above is not vacuous: with the SAME fixture but the dedup
// deliberately bypassed (both the project-scope AND a second, synthetic
// global-scope entry executed, simulating the pre-ML-2B double-wiring bug),
// the block message appears twice — the exact production symptom
// (`docs/roadmaps/wip/ROADMAP-2026-08-17-...md` ML-2B "Impacto medido").
func TestProjectGitBranchGuard_BothScopesFireIndependently_TwoMessages_NonVacuous(t *testing.T) {
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
