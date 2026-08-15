package generators

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ROADMAP-2026-08-06 Wave 3/ML-3A — InjectXHooks (project scope) must skip
// the credential-guard entry when the corresponding global-scope wiring
// (installed via `trackfw update harness --targets <tool>-credential-guard`,
// internal/generators/update.go) is already present, and must fail-open
// (fall back to the pre-ML-3A behavior of always adding the project-scope
// entry) if the global file is missing, unreadable, or unparseable.
// ---------------------------------------------------------------------------

// dedupFixtureHome creates an isolated $HOME and points HOME at it for the
// duration of the test.
func dedupFixtureHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func dedupScriptPath(home string) string {
	return filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
}

func TestDedup_Claude_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
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
	if helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("project-scope credential-guard entry should have been skipped (global already installed)")
	}
	if helperHasClaudeHook(data, "PostToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("project-scope PostToolUse credential-guard entry should have been skipped")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh") {
		t.Error("attention-signal entry must still be added regardless of global credential-guard state")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh") {
		t.Error("attention-cleanup entry must still be added regardless of global credential-guard state")
	}
}

func TestDedup_Codex_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
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
	if helperHasClaudeHook(data, "PreToolUse", "Bash", codexGuardCmd) {
		t.Error("project-scope credential-guard entry should have been skipped (global already installed)")
	}
	if helperHasClaudeHook(data, "PostToolUse", "Bash", codexGuardCmd) {
		t.Error("project-scope PostToolUse credential-guard entry should have been skipped")
	}
	if !helperHasClaudeHook(data, "PermissionRequest", ".*", codexSignalCmd) {
		t.Error("attention-signal entry must still be added regardless of global credential-guard state")
	}
	if !helperHasClaudeHook(data, "PostToolUse", ".*", codexCleanupCmd) {
		t.Error("attention-cleanup entry must still be added regardless of global credential-guard state")
	}
}

func TestDedup_Gemini_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
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
	if helperHasClaudeHook(data, "BeforeTool", "run_shell_command", geminiGuardCmd) {
		t.Error("project-scope credential-guard entry should have been skipped (global already installed)")
	}
	if helperHasClaudeHook(data, "AfterTool", "run_shell_command", geminiGuardCmd) {
		t.Error("project-scope AfterTool credential-guard entry should have been skipped")
	}
	if !helperHasClaudeHook(data, "Notification", "ToolPermission", geminiSignalCmd) {
		t.Error("attention-signal entry must still be added regardless of global credential-guard state")
	}
	if !helperHasClaudeHook(data, "AfterTool", "*", geminiCleanupCmd) {
		t.Error("attention-cleanup entry must still be added regardless of global credential-guard state")
	}
}

func TestDedup_Cursor_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
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
	// The git-branch-guard entry (ROADMAP-2026-08-14 ML-3A) has no
	// global-scope dedup target yet, so it is always added regardless of
	// the global credential-guard dedup state exercised by this test —
	// exactly 1 beforeShellExecution entry (git-branch-guard), 0 for
	// credential-guard (skipped, global installed).
	if len(before) != 1 || before[0].(map[string]interface{})["command"] != "scripts/trackfw-git-branch-guard.sh" {
		t.Errorf("expected only the git-branch-guard beforeShellExecution entry (credential-guard skipped, global already installed), got %v", before)
	}
	if len(after) != 0 {
		t.Errorf("expected no project-scope afterShellExecution entries (global already installed), got %v", after)
	}
	pre, _ := hooks["preToolUse"].([]interface{})
	post, _ := hooks["postToolUse"].([]interface{})
	if len(pre) != 1 || pre[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-signal.sh" {
		t.Errorf("attention-signal entry must still be added regardless of global credential-guard state, got %v", pre)
	}
	if len(post) != 1 || post[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-cleanup.sh" {
		t.Errorf("attention-cleanup entry must still be added regardless of global credential-guard state, got %v", post)
	}
}

func TestDedup_Copilot_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
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
	// preToolUse: attention-signal + git-branch-guard (ROADMAP-2026-08-14
	// ML-3A, no global-scope dedup target yet, always added) — credential-guard
	// is skipped (global installed), so exactly 2 entries.
	if len(pre) != 2 {
		t.Errorf("expected attention-signal + git-branch-guard entries in preToolUse (global credential-guard installed), got %v", pre)
	}
	if pre[0].(map[string]interface{})["bash"] != "scripts/trackfw-attention-signal.sh" {
		t.Errorf("expected preToolUse[0] to be the attention-signal entry, got %v", pre[0])
	}
	foundGitGuard := false
	for _, item := range pre {
		if item.(map[string]interface{})["bash"] == "scripts/trackfw-git-branch-guard.sh" {
			foundGitGuard = true
		}
	}
	if !foundGitGuard {
		t.Errorf("expected preToolUse to contain the git-branch-guard entry, got %v", pre)
	}
	if len(post) != 1 || post[0].(map[string]interface{})["bash"] != "scripts/trackfw-attention-cleanup.sh" {
		t.Errorf("expected only the attention-cleanup entry in postToolUse (global credential-guard installed), got %v", post)
	}
}

func TestDedup_Kiro_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
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
	if len(hooks) != 2 {
		t.Fatalf("expected only 2 hooks (signal, cleanup) when global credential-guard is installed, got %d: %v", len(hooks), hooks)
	}
	for _, h := range hooks {
		entry, _ := h.(map[string]interface{})
		name, _ := entry["name"].(string)
		if name == "trackfw-credential-guard-pre" || name == "trackfw-credential-guard-post" {
			t.Errorf("project-scope credential-guard hook %q should have been skipped (global already installed)", name)
		}
	}
}

// --- Fail-open: missing/corrupted global file must not disable the
// project-scope credential-guard entry. ---

func TestDedup_FailOpen_NoGlobalFile(t *testing.T) {
	dedupFixtureHome(t) // empty $HOME, no global files at all

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("expected project-scope credential-guard entry to be added when no global file exists (fail-open)")
	}
}

func TestDedup_FailOpen_CorruptedGlobalFile(t *testing.T) {
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
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("expected project-scope credential-guard entry to be added when global file is corrupted (fail-open)")
	}
}

func TestDedup_FailOpen_UnreadableGlobalFile(t *testing.T) {
	home := dedupFixtureHome(t)
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"hooks":{}}`), 0000); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) }) // allow t.TempDir cleanup

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Error("expected project-scope credential-guard entry to be added when global file is unreadable (fail-open)")
	}
}
