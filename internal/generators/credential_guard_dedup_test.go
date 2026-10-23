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
	if helperHasClaudeHook(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard entry should have been skipped (global already installed)")
	}
	if helperHasClaudeHook(data, "PostToolUse", "Bash", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope PostToolUse credential-guard entry should have been skipped")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", attentionSignalCmd) {
		t.Error("attention-signal entry must still be added regardless of global credential-guard state")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", attentionCleanupCmd) {
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
	if helperHasClaudeHook(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard entry should have been skipped (global already installed)")
	}
	if helperHasClaudeHook(data, "PostToolUse", "Bash", guardCredentialCmdPSPOSIX) {
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
	if helperHasClaudeHook(data, "BeforeTool", "run_shell_command", guardCredentialCmdPSPOSIX) {
		t.Error("project-scope credential-guard entry should have been skipped (global already installed)")
	}
	if helperHasClaudeHook(data, "AfterTool", "run_shell_command", guardCredentialCmdPSPOSIX) {
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
	if len(before) != 1 || before[0].(map[string]interface{})["command"] != guardGitBranchCmdPSPOSIX {
		t.Errorf("expected only the git-branch-guard beforeShellExecution entry (credential-guard skipped, global already installed), got %v", before)
	}
	if len(after) != 0 {
		t.Errorf("expected no project-scope afterShellExecution entries (global already installed), got %v", after)
	}
	pre, _ := hooks["preToolUse"].([]interface{})
	post, _ := hooks["postToolUse"].([]interface{})
	// D14 revised (ADR-2026-10-04 adendo 2026-10-23): the global harness covers only shell
	// events; Read/Write pairs are ALWAYS emitted. So pre/post each have 3 entries:
	// attention-signal/cleanup + {Read, credential} + {Write, credential}.
	if len(pre) != 3 {
		t.Errorf("expected 3 preToolUse entries (attention-signal + Read guard + Write guard), got %d: %v", len(pre), pre)
	} else {
		if pre[0].(map[string]interface{})["command"] != attentionSignalCmd {
			t.Errorf("preToolUse[0] must be attention-signal, got %v", pre[0])
		}
		// Read and Write guard entries can be in any order after the signal.
		hasRead, hasWrite := false, false
		for _, item := range pre[1:] {
			obj := item.(map[string]interface{})
			if obj["command"] == guardCredentialCmdPSPOSIX && obj["matcher"] == "Read" {
				hasRead = true
			}
			if obj["command"] == guardCredentialCmdPSPOSIX && obj["matcher"] == "Write" {
				hasWrite = true
			}
		}
		if !hasRead {
			t.Errorf("preToolUse must contain a Read credential-guard entry, got %v", pre)
		}
		if !hasWrite {
			t.Errorf("preToolUse must contain a Write credential-guard entry, got %v", pre)
		}
	}
	if len(post) != 3 {
		t.Errorf("expected 3 postToolUse entries (attention-cleanup + Read guard + Write guard), got %d: %v", len(post), post)
	} else {
		if post[0].(map[string]interface{})["command"] != attentionCleanupCmd {
			t.Errorf("postToolUse[0] must be attention-cleanup, got %v", post[0])
		}
	}
}

func TestDedup_Copilot_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	// D14 revised (REQ-2026-10-23 ML-2C): global covers only "bash". With global
	// installed, "bash" credential-guard is absent; "view" and "create|edit" are
	// always emitted (not covered by the global).
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

	// preToolUse: attention-signal + view guard + create|edit guard + git-branch-guard = 4
	// (bash credential-guard absent — covered by global).
	if len(pre) != 4 {
		t.Errorf("expected 4 preToolUse entries (attention-signal + view + create|edit + git-branch), got %d: %v", len(pre), pre)
	}
	if pre[0].(map[string]interface{})["command"] != attentionSignalCmd {
		t.Errorf("expected preToolUse[0] to be the attention-signal entry, got %v", pre[0])
	}
	// "bash" credential guard must be absent (global covers it).
	for _, item := range pre {
		obj := item.(map[string]interface{})
		if obj["matcher"] == "bash" && obj["command"] == guardCredentialCmdPSPOSIX {
			t.Errorf("bash credential-guard must be absent when global is installed, found: %v", obj)
		}
	}
	// "view" and "create|edit" credential guards must be present.
	foundView, foundCreateEdit := false, false
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
	}
	if !foundView {
		t.Errorf("expected view credential-guard in preToolUse (not covered by global), got %v", pre)
	}
	if !foundCreateEdit {
		t.Errorf("expected create|edit credential-guard in preToolUse (not covered by global), got %v", pre)
	}
	// git-branch-guard must still be present.
	foundGitGuard := false
	for _, item := range pre {
		if item.(map[string]interface{})["command"] == guardGitBranchCmdPSPOSIX {
			foundGitGuard = true
		}
	}
	if !foundGitGuard {
		t.Errorf("expected preToolUse to contain the git-branch-guard entry, got %v", pre)
	}

	// postToolUse: attention-cleanup + view guard + create|edit guard = 3
	if len(post) != 3 {
		t.Errorf("expected 3 postToolUse entries (attention-cleanup + view + create|edit), got %d: %v", len(post), post)
	}
	if post[0].(map[string]interface{})["command"] != attentionCleanupCmd {
		t.Errorf("expected postToolUse[0] to be attention-cleanup, got %v", post[0])
	}
	// "bash" credential guard must be absent in postToolUse too.
	for _, item := range post {
		obj := item.(map[string]interface{})
		if obj["matcher"] == "bash" && obj["command"] == guardCredentialCmdPSPOSIX {
			t.Errorf("bash credential-guard must be absent in postToolUse when global is installed, found: %v", obj)
		}
	}
}

func TestDedup_Kiro_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	// D14 revised (REQ-2026-10-23 ML-2C): global covers only "shell". With global
	// installed, "shell" credential-guard is absent; "read" and "write" are always
	// emitted (not covered by the global).
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
	// With global installed: signal + cleanup + read-pre + read-post + write-pre + write-post + git-branch = 7
	// (shell pre/post absent — covered by global).
	if len(hooks) != 7 {
		t.Fatalf("expected 7 hooks when global credential-guard is installed, got %d: %v", len(hooks), hooks)
	}
	// "shell" pre/post must be absent.
	for _, h := range hooks {
		entry, _ := h.(map[string]interface{})
		name, _ := entry["name"].(string)
		if name == "trackfw-credential-guard-pre" || name == "trackfw-credential-guard-post" {
			t.Errorf("project-scope shell credential-guard hook %q must be absent when global is installed", name)
		}
	}
	// "read" and "write" guards must be present.
	wantNames := []string{
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
			t.Errorf("expected hook %q to be present (not covered by global), missing from: %v", want, hooks)
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
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX) {
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
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX) {
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
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX) {
		t.Error("expected project-scope credential-guard entry to be added when global file is unreadable (fail-open)")
	}
}

// ---------------------------------------------------------------------------
// ML-1B (REQ-2026-10-06) — Windsurf global dedup
// ---------------------------------------------------------------------------

// Reconciliação: afirma que InjectWindsurfHooks NÃO adiciona entradas de credential
// guard em pre_run_command nem pre_write_code quando a instalação global
// (~/.codeium/windsurf/hooks.json) já contém o global form (guardCredentialGlobalCmdPSPOSIX).
// Isso evita disparo duplo quando o harness windsurf-credential-guard está ativo.
func TestDedup_Windsurf_SkipsProjectEntryWhenGlobalInstalled(t *testing.T) {
	home := dedupFixtureHome(t)
	// Simula a instalação global via harnessCredentialGuardTargetWindsurf.
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

	// project-scope credential guard must NOT be added
	for _, event := range []string{"pre_run_command", "pre_write_code"} {
		arr, _ := hooksMap[event].([]interface{})
		for _, item := range arr {
			obj, _ := item.(map[string]interface{})
			if obj["command"] == guardCredentialCmdPSPOSIX {
				t.Errorf("project-scope credential-guard entry should have been skipped in %s (global already installed)", event)
			}
		}
	}

	// git-branch guard must still be present (dedup only applies to credential)
	pre, _ := hooksMap["pre_run_command"].([]interface{})
	found := false
	for _, item := range pre {
		obj, _ := item.(map[string]interface{})
		if obj["command"] == guardGitBranchCmdPSPOSIX {
			found = true
		}
	}
	if !found {
		t.Error("git-branch guard in pre_run_command must still be added even when global credential-guard is installed")
	}
}

// Reconciliação: afirma que InjectWindsurfHooks adiciona o credential guard
// em pre_run_command e pre_write_code quando a global está ausente (fail-open).
func TestDedup_Windsurf_FailOpenWhenGlobalMissing(t *testing.T) {
	dedupFixtureHome(t) // HOME → temp dir, ~/.codeium/windsurf/hooks.json not present
	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".windsurf", "hooks.json"))
	hooksMap, _ := data["hooks"].(map[string]interface{})

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
			t.Errorf("expected project-scope credential-guard in %s when global is absent (fail-open)", event)
		}
	}
}
