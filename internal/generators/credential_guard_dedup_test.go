package generators

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// D15 (ADR-2026-10-04 adendo 2026-10-24, REQ-2026-10-23 #613) —
// InjectXHooks (project scope) ALWAYS emits the credential-guard entry
// regardless of whether the global-scope wiring is installed. The old D14
// dedup behavior is removed; the tests below assert presence (not absence)
// when a global file is present.
// ---------------------------------------------------------------------------

// dedupFixtureHome creates an isolated $HOME and points HOME, USERPROFILE and
// CLAUDE_CONFIG_DIR at it for the duration of the test.
// ML-2E: on Windows, agentconfig.ClaudeConfigDir reads USERPROFILE (not HOME);
// setting both ensures global-Claude reads/writes land in the temp dir.
func dedupFixtureHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home
}

func dedupScriptPath(home string) string {
	return filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
}

// Reconciliação (D15): afirma que InjectClaudeHooks SEMPRE emite as entradas de
// credential guard mesmo com o harness global instalado. A semente global existe para
// provar que o código não a lê para pular a fiação de projeto.
func TestProjectCredentialGuard_Claude_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := dedupScriptPath(home)
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
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard PreToolUse must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "PostToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard PostToolUse must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", attentionSignalCmd) {
		t.Error("attention-signal entry must still be added")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", attentionCleanupCmd) {
		t.Error("attention-cleanup entry must still be added")
	}
}

// Reconciliação (D15): Codex — idem Claude.
func TestProjectCredentialGuard_Codex_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := dedupScriptPath(home)
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
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard PreToolUse must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard PostToolUse must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "PermissionRequest", ".*", codexSignalCmd) {
		t.Error("attention-signal entry must still be added")
	}
	if !helperHasClaudeHook(data, "PostToolUse", ".*", codexCleanupCmd) {
		t.Error("attention-cleanup entry must still be added")
	}
}

// Reconciliação (D15): Gemini — idem Claude.
func TestProjectCredentialGuard_Gemini_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := dedupScriptPath(home)
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
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard BeforeTool must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "AfterTool", "run_shell_command", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard AfterTool must be present even when global is installed (D15)")
	}
	if !helperHasClaudeHook(data, "Notification", "ToolPermission", geminiSignalCmd) {
		t.Error("attention-signal entry must still be added")
	}
	if !helperHasClaudeHook(data, "AfterTool", "*", geminiCleanupCmd) {
		t.Error("attention-cleanup entry must still be added")
	}
}

// Reconciliação (D15): Cursor — both beforeShellExecution entries (credential + git-branch)
// must be present when global is installed.
func TestProjectCredentialGuard_Cursor_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	scriptPath := dedupScriptPath(home)
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
	after, _ := hooks["afterShellExecution"].([]interface{})

	// D15: both credential-guard and git-branch-guard must be present in beforeShellExecution.
	foundCred, foundGitBranch := false, false
	for _, item := range before {
		obj, _ := item.(map[string]interface{})
		switch obj["command"] {
		case guardCredentialCmdPSPOSIX:
			foundCred = true
		case guardGitBranchCmdPSPOSIX:
			foundGitBranch = true
		}
	}
	if !foundCred {
		t.Errorf("credential-guard must be present in beforeShellExecution even when global is installed (D15), got %v", before)
	}
	if !foundGitBranch {
		t.Errorf("git-branch-guard must be present in beforeShellExecution, got %v", before)
	}
	// afterShellExecution: credential-guard must be present.
	foundCredAfter := false
	for _, item := range after {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == guardCredentialCmdPSPOSIX {
			foundCredAfter = true
		}
	}
	if !foundCredAfter {
		t.Errorf("credential-guard must be present in afterShellExecution (D15), got %v", after)
	}

	pre, _ := hooks["preToolUse"].([]interface{})
	post, _ := hooks["postToolUse"].([]interface{})
	// D15: pre/post each have 3 entries: attention + Read guard + Write guard.
	if len(pre) != 3 {
		t.Errorf("expected 3 preToolUse entries, got %d: %v", len(pre), pre)
	}
	if len(post) != 3 {
		t.Errorf("expected 3 postToolUse entries, got %d: %v", len(post), post)
	}
}

// Reconciliação (D15): Copilot — bash credential guard must be present even with global.
func TestProjectCredentialGuard_Copilot_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
	// D15: global seed present to prove the code no longer reads it to skip.
	home := dedupFixtureHome(t)
	scriptPath := dedupScriptPath(home)
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
	post, _ := hooks["postToolUse"].([]interface{})

	// D15: preToolUse must have bash credential guard present.
	foundBashCred := false
	for _, item := range pre {
		obj := item.(map[string]interface{})
		if obj["matcher"] == "bash" && obj["command"] == guardCredentialCmdPSPOSIX {
			foundBashCred = true
		}
	}
	if !foundBashCred {
		t.Errorf("bash credential-guard must be present in preToolUse even when global is installed (D15), got %v", pre)
	}

	// postToolUse: bash credential guard must also be present.
	foundBashCredPost := false
	for _, item := range post {
		obj := item.(map[string]interface{})
		if obj["matcher"] == "bash" && obj["command"] == guardCredentialCmdPSPOSIX {
			foundBashCredPost = true
		}
	}
	if !foundBashCredPost {
		t.Errorf("bash credential-guard must be present in postToolUse even when global is installed (D15), got %v", post)
	}

	// view, create|edit and git-branch-guard must all still be present.
	foundView, foundCreateEdit, foundGitGuard := false, false, false
	for _, item := range pre {
		obj := item.(map[string]interface{})
		if obj["command"] == guardCredentialCmdPSPOSIX {
			switch obj["matcher"] {
			case "view":
				foundView = true
			case "create|edit":
				foundCreateEdit = true
			}
		}
		if obj["command"] == guardGitBranchCmdPSPOSIX {
			foundGitGuard = true
		}
	}
	if !foundView {
		t.Errorf("view credential-guard must be present, got %v", pre)
	}
	if !foundCreateEdit {
		t.Errorf("create|edit credential-guard must be present, got %v", pre)
	}
	if !foundGitGuard {
		t.Errorf("git-branch-guard must be present, got %v", pre)
	}
}

// Reconciliação (D15): Kiro — shell credential guard must be present even with global.
func TestProjectCredentialGuard_Kiro_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
	// D15: global kiro seed present.
	home := dedupFixtureHome(t)
	globalKiroPath := filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json")
	if err := os.MkdirAll(filepath.Dir(globalKiroPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalKiroPath, []byte(`{"version":"v1","hooks":[]}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dir := t.TempDir()
	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json"))
	hooks, _ := data["hooks"].([]interface{})
	// D15: signal + cleanup + shell-pre + shell-post + read-pre + read-post + write-pre + write-post + git-branch = 9
	if len(hooks) != 9 {
		t.Fatalf("expected 9 hooks (D15: shell credential-guard present), got %d: %v", len(hooks), hooks)
	}
	// shell pre/post must be present.
	wantNames := []string{
		"trackfw-credential-guard-pre",
		"trackfw-credential-guard-post",
		"trackfw-credential-guard-read-pre",
		"trackfw-credential-guard-read-post",
		"trackfw-credential-guard-write-pre",
		"trackfw-credential-guard-write-post",
	}
	for _, want := range wantNames {
		found := false
		for _, h := range hooks {
			entry, _ := h.(map[string]interface{})
			if entry["name"] == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected hook %q to be present (D15), missing from: %v", want, hooks)
		}
	}
}

// ---------------------------------------------------------------------------
// ML-1B (REQ-2026-10-06) — Windsurf credential guard
// ---------------------------------------------------------------------------

// Reconciliação (D15): afirma que InjectWindsurfHooks SEMPRE adiciona entradas de
// credential guard em pre_run_command e pre_write_code mesmo com a instalação global
// (~/.codeium/windsurf/hooks.json) presente. A semente existe para provar que o código
// não a lê para pular a fiação de projeto.
func TestProjectCredentialGuard_Windsurf_EmitsProjectEntryEvenWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	helperWriteJSON(t, filepath.Join(home, ".codeium", "windsurf", "hooks.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"pre_run_command": []interface{}{
				map[string]interface{}{"command": guardCredentialGlobalCmdPSPOSIX, "show_output": true},
			},
			"pre_write_code": []interface{}{
				map[string]interface{}{"command": guardCredentialGlobalCmdPSPOSIX, "show_output": true},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"))
	hooksMap, _ := data["hooks"].(map[string]interface{})

	// D15: project-scope credential guard must be present in both events.
	for _, event := range []string{"pre_run_command", "pre_write_code"} {
		arr, _ := hooksMap[event].([]interface{})
		found := false
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["command"] == guardCredentialCmdPSPOSIX {
				found = true
			}
		}
		if !found {
			t.Errorf("project-scope credential-guard must be present in %s even when global is installed (D15)", event)
		}
	}

	// git-branch guard must still be present in pre_run_command.
	pre, _ := hooksMap["pre_run_command"].([]interface{})
	found := false
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == guardGitBranchCmdPSPOSIX {
			found = true
		}
	}
	if !found {
		t.Error("git-branch guard in pre_run_command must still be present")
	}
}
