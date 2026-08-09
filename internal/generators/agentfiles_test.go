package generators

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func helperWriteJSON(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	if err := os.WriteFile(path, append(b, '\n'), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func helperReadJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile %s: %v", path, err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}

func helperHasClaudeHook(data map[string]interface{}, event, matcher, command string) bool {
	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		return false
	}
	arr, _ := hooks[event].([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})
		for _, h := range innerHooks {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == command {
				return true
			}
		}
	}
	return false
}

// --- Claude ---

func TestInjectClaudeHooks_Create(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse[AskUserQuestion] → signal.sh missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse[AskUserQuestion] → cleanup.sh missing")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse[Bash] → credential-guard.sh missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("PostToolUse[Bash] → credential-guard.sh missing")
	}
}

func TestInjectClaudeHooks_MergeAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	existing := map[string]interface{}{
		"permissions": map[string]interface{}{"defaultMode": "default"},
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/other.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), existing)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("first InjectClaudeHooks failed: %v", err)
	}
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("second InjectClaudeHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/other.sh") {
		t.Error("existing Bash hook lost during merge")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse signal hook missing")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse credential-guard hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse cleanup hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("PostToolUse credential-guard hook missing")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	pr, _ := hooks["PreToolUse"].([]interface{})
	// A pre-existing "Bash" matcher entry (third-party hook) must be merged with
	// (not duplicated by) the new credential-guard "Bash" entry: 4 entries total
	// -- {Bash: [other.sh, credential-guard.sh]}, {AskUserQuestion: [signal.sh]},
	// {Read: [credential-guard.sh]}, {Write|Edit: [credential-guard.sh]}
	// (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(pr) != 4 {
		t.Errorf("expected 4 PreToolUse entries, got %d", len(pr))
	}
	post, _ := hooks["PostToolUse"].([]interface{})
	if len(post) != 4 {
		t.Errorf("expected 4 PostToolUse entries, got %d", len(post))
	}
}

// --- Codex ---

func TestInjectCodexHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks failed: %v", err)
	}
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("second InjectCodexHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	if !helperHasClaudeHook(data, "PermissionRequest", ".*", "scripts/trackfw-attention-signal.sh") {
		t.Error("Codex PermissionRequest hook missing")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("Codex PreToolUse[Bash] credential-guard hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", ".*", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("Codex PostToolUse hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("Codex PostToolUse[Bash] credential-guard hook missing")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["PreToolUse"].([]interface{})
	// 2 entries: {matcher:"Bash", hooks:[credential-guard.sh]}, {matcher:"apply_patch",
	// hooks:[credential-guard.sh]} (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2;
	// Codex has no read-tool matcher — see InjectCodexHooks doc comment).
	if len(pre) != 2 {
		t.Errorf("expected 2 PreToolUse entries (Bash + apply_patch, no idempotency dup), got %d", len(pre))
	}
	post, _ := hooks["PostToolUse"].([]interface{})
	// 3 entries: {matcher:".*", hooks:[cleanup.sh]}, {matcher:"Bash", hooks:[credential-guard.sh]},
	// {matcher:"apply_patch", hooks:[credential-guard.sh]}
	if len(post) != 3 {
		t.Errorf("expected 3 PostToolUse entries, got %d", len(post))
	}
}

func TestInjectCodexHooks_PreservesExistingBashEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/other.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".codex", "hooks.json"), existing)

	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks failed: %v", err)
	}
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("second InjectCodexHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/other.sh") {
		t.Error("existing Bash hook lost during merge")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "scripts/trackfw-credential-guard.sh") {
		t.Error("PreToolUse[Bash] credential-guard hook missing after merge")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["PreToolUse"].([]interface{})
	// 2 entries: {matcher:"Bash", hooks:[other.sh, credential-guard.sh]} (merged),
	// {matcher:"apply_patch", hooks:[credential-guard.sh]}.
	if len(pre) != 2 {
		t.Errorf("expected 2 PreToolUse entries (Bash merged + apply_patch), got %d", len(pre))
	}
}

// --- Gemini ---

func TestInjectGeminiHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks failed: %v", err)
	}
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("second InjectGeminiHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	if !helperHasClaudeHook(data, "Notification", "ToolPermission", "scripts/trackfw-attention-signal.sh") {
		t.Error("Gemini Notification hook missing")
	}
	if !helperHasClaudeHook(data, "AfterTool", "*", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("Gemini AfterTool[*] cleanup hook missing")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", "scripts/trackfw-credential-guard.sh") {
		t.Error("Gemini BeforeTool[run_shell_command] credential-guard hook missing")
	}
	if !helperHasClaudeHook(data, "AfterTool", "run_shell_command", "scripts/trackfw-credential-guard.sh") {
		t.Error("Gemini AfterTool[run_shell_command] credential-guard hook missing")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	before, _ := hooks["BeforeTool"].([]interface{})
	// 3 entries: {matcher:"run_shell_command", ...}, {matcher:"read_file|read_many_files", ...},
	// {matcher:"write_file|replace", ...} (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(before) != 3 {
		t.Errorf("expected 3 BeforeTool entries (run_shell_command + read + write, no idempotency dup), got %d", len(before))
	}
	after, _ := hooks["AfterTool"].([]interface{})
	// 4 entries: {matcher:"*", hooks:[cleanup.sh]}, {matcher:"run_shell_command", ...},
	// {matcher:"read_file|read_many_files", ...}, {matcher:"write_file|replace", ...}
	if len(after) != 4 {
		t.Errorf("expected 4 AfterTool entries, got %d", len(after))
	}
}

func TestInjectGeminiHooks_PreservesExistingBeforeToolEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME

	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"BeforeTool": []interface{}{
				map[string]interface{}{
					"matcher": "run_shell_command",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": "scripts/other.sh"}},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".gemini", "settings.json"), existing)

	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks failed: %v", err)
	}
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("second InjectGeminiHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", "scripts/other.sh") {
		t.Error("existing BeforeTool[run_shell_command] hook lost during merge")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", "scripts/trackfw-credential-guard.sh") {
		t.Error("BeforeTool[run_shell_command] credential-guard hook missing after merge")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	before, _ := hooks["BeforeTool"].([]interface{})
	// 3 entries: {matcher:"run_shell_command", hooks:[other.sh, credential-guard.sh]} (merged),
	// {matcher:"read_file|read_many_files", ...}, {matcher:"write_file|replace", ...}.
	if len(before) != 3 {
		t.Errorf("expected 3 BeforeTool entries (run_shell_command merged + read + write), got %d", len(before))
	}
}

// --- Kiro ---

func TestInjectKiroHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks failed: %v", err)
	}
	file := filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json")
	content1, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("second InjectKiroHooks failed: %v", err)
	}
	content2, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("second ReadFile failed: %v", err)
	}

	if !bytes.Equal(content1, content2) {
		t.Fatalf("expected Kiro config content to be identical after 2nd injection")
	}

	data := helperReadJSON(t, file)
	if v, _ := data["version"].(string); v != "v1" {
		t.Fatalf("expected version \"v1\", got %v", data["version"])
	}
	hooks, _ := data["hooks"].([]interface{})
	// 8 entries: signal, cleanup, credential-guard shell pre/post, read pre/post,
	// write pre/post (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(hooks) != 8 {
		t.Fatalf("expected 8 hooks in Kiro config (signal, cleanup, credential-guard shell/read/write pre/post), got %d", len(hooks))
	}

	sawGuardPre, sawGuardPost := false, false
	for _, h := range hooks {
		entry, _ := h.(map[string]interface{})
		if entry == nil {
			continue
		}
		if _, hasEvent := entry["event"]; hasEvent {
			t.Fatalf("hook entry uses legacy \"event\" field, expected \"trigger\": %v", entry)
		}
		trigger, _ := entry["trigger"].(string)
		if trigger == "" {
			t.Fatalf("hook entry missing \"trigger\": %v", entry)
		}
		if _, isObject := entry["matcher"].(map[string]interface{}); isObject {
			t.Fatalf("hook entry uses object matcher, expected plain regex string: %v", entry)
		}
		name, _ := entry["name"].(string)
		switch name {
		case "trackfw-credential-guard-pre":
			sawGuardPre = true
			if trigger != "PreToolUse" {
				t.Fatalf("expected credential-guard-pre trigger PreToolUse, got %q", trigger)
			}
			if m, _ := entry["matcher"].(string); m != "shell" {
				t.Fatalf("expected credential-guard-pre matcher \"shell\", got %q", m)
			}
		case "trackfw-credential-guard-post":
			sawGuardPost = true
			if trigger != "PostToolUse" {
				t.Fatalf("expected credential-guard-post trigger PostToolUse, got %q", trigger)
			}
		}
	}
	if !sawGuardPre || !sawGuardPost {
		t.Fatalf("expected both credential-guard pre and post hooks, got pre=%v post=%v", sawGuardPre, sawGuardPost)
	}
}

// --- Copilot ---

func TestInjectCopilotHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks failed: %v", err)
	}
	file := filepath.Join(dir, ".github", "hooks", "trackfw-attention.json")
	content1, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("second InjectCopilotHooks failed: %v", err)
	}
	content2, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("second ReadFile failed: %v", err)
	}

	if !bytes.Equal(content1, content2) {
		t.Fatalf("expected Copilot config content to be identical after 2nd injection")
	}

	data := helperReadJSON(t, file)
	if data["version"] != float64(1) {
		t.Fatalf("expected version 1, got %v", data["version"])
	}
	hooks, ok := data["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected hooks to be an object keyed by event, got %v", data["hooks"])
	}

	// 4 entries each: signal/cleanup + credential-guard "bash"/"view"/"create|edit"
	// (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	pre, ok := hooks["preToolUse"].([]interface{})
	if !ok || len(pre) != 4 {
		t.Fatalf("expected preToolUse array of size 4, got %v", hooks["preToolUse"])
	}
	post, ok := hooks["postToolUse"].([]interface{})
	if !ok || len(post) != 4 {
		t.Fatalf("expected postToolUse array of size 4, got %v", hooks["postToolUse"])
	}

	helperFindCopilotEntry := func(arr []interface{}, bash string) map[string]interface{} {
		for _, item := range arr {
			obj, ok := item.(map[string]interface{})
			if ok && obj["bash"] == bash {
				return obj
			}
		}
		return nil
	}

	signal := helperFindCopilotEntry(pre, "scripts/trackfw-attention-signal.sh")
	if signal == nil {
		t.Fatal("preToolUse missing attention-signal entry")
	}
	if signal["matcher"] != nil {
		t.Errorf("attention-signal entry should not have a matcher, got %v", signal["matcher"])
	}

	guardPre := helperFindCopilotEntry(pre, "scripts/trackfw-credential-guard.sh")
	if guardPre == nil {
		t.Fatal("preToolUse missing credential-guard entry")
	}
	if guardPre["matcher"] != "bash" {
		t.Errorf("credential-guard preToolUse entry should have matcher=bash, got %v", guardPre["matcher"])
	}

	cleanup := helperFindCopilotEntry(post, "scripts/trackfw-attention-cleanup.sh")
	if cleanup == nil {
		t.Fatal("postToolUse missing attention-cleanup entry")
	}

	guardPost := helperFindCopilotEntry(post, "scripts/trackfw-credential-guard.sh")
	if guardPost == nil {
		t.Fatal("postToolUse missing credential-guard entry")
	}
	if guardPost["matcher"] != "bash" {
		t.Errorf("credential-guard postToolUse entry should have matcher=bash, got %v", guardPost["matcher"])
	}
}

// --- Cursor ---

func TestInjectCursorHooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("second InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	if _, ok := data["preToolUse"]; ok {
		t.Errorf("expected no top-level preToolUse key (legacy schema), got %v", data["preToolUse"])
	}
	if _, ok := data["postToolUse"]; ok {
		t.Errorf("expected no top-level postToolUse key (legacy schema), got %v", data["postToolUse"])
	}

	if data["version"] != float64(1) {
		t.Errorf("expected version=1, got %v", data["version"])
	}
	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatalf("expected top-level hooks object, got none")
	}

	pre, _ := hooks["preToolUse"].([]interface{})
	post, _ := hooks["postToolUse"].([]interface{})
	// 3 entries each: attention-signal/cleanup (unfiltered) + credential-guard
	// scoped to matcher "Read" + credential-guard scoped to matcher "Write"
	// (ADR-2026-08-06 emenda 7/ROADMAP-2026-08-08 Wave 2).
	if len(pre) != 3 || len(post) != 3 {
		t.Fatalf("expected 3 hooks.preToolUse and 3 hooks.postToolUse entries, got %d pre, %d post", len(pre), len(post))
	}
	if pre[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-signal.sh" {
		t.Errorf("hooks.preToolUse[0] should be the attention-signal script, got %v", pre[0])
	}
	if post[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-cleanup.sh" {
		t.Errorf("hooks.postToolUse[0] should be the attention-cleanup script, got %v", post[0])
	}

	before, _ := hooks["beforeShellExecution"].([]interface{})
	after, _ := hooks["afterShellExecution"].([]interface{})
	if len(before) != 1 || len(after) != 1 {
		t.Fatalf("expected 1 beforeShellExecution and 1 afterShellExecution entry, got %d before, %d after", len(before), len(after))
	}
	if before[0].(map[string]interface{})["command"] != "scripts/trackfw-credential-guard.sh" {
		t.Errorf("beforeShellExecution[0] should be the credential-guard script, got %v", before[0])
	}
	if after[0].(map[string]interface{})["command"] != "scripts/trackfw-credential-guard.sh" {
		t.Errorf("afterShellExecution[0] should be the credential-guard script, got %v", after[0])
	}
}

func TestInjectCursorHooks_MigratesLegacyTopLevelArrays(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{
  "preToolUse": [{"command": "scripts/trackfw-attention-signal.sh"}, {"command": "./my-custom-hook.sh"}],
  "postToolUse": [{"command": "scripts/trackfw-attention-cleanup.sh"}]
}`
	if err := os.WriteFile(filepath.Join(cursorDir, "hooks.json"), []byte(legacy), 0644); err != nil {
		t.Fatalf("seed hooks.json: %v", err)
	}

	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))

	// The known trackfw entry must be migrated out of preToolUse, but the
	// unrelated user entry must survive untouched at the top level.
	pre, _ := data["preToolUse"].([]interface{})
	if len(pre) != 1 {
		t.Fatalf("expected 1 surviving unrelated entry in top-level preToolUse, got %d: %v", len(pre), pre)
	}
	if pre[0].(map[string]interface{})["command"] != "./my-custom-hook.sh" {
		t.Errorf("expected unrelated user entry to survive, got %v", pre[0])
	}

	// postToolUse had only the known trackfw entry, so the key must be gone entirely.
	if _, ok := data["postToolUse"]; ok {
		t.Errorf("expected top-level postToolUse to be removed once empty, got %v", data["postToolUse"])
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	hPre, _ := hooks["preToolUse"].([]interface{})
	hPost, _ := hooks["postToolUse"].([]interface{})
	// 3 entries each after migration: the migrated attention-signal/cleanup entry
	// plus the two matcher-scoped credential-guard entries (Read/Write) added by
	// this ML — see TestInjectCursorHooks.
	if len(hPre) != 3 || hPre[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-signal.sh" {
		t.Errorf("expected hooks.preToolUse to contain the migrated attention-signal entry, got %v", hPre)
	}
	if len(hPost) != 3 || hPost[0].(map[string]interface{})["command"] != "scripts/trackfw-attention-cleanup.sh" {
		t.Errorf("expected hooks.postToolUse to contain the migrated attention-cleanup entry, got %v", hPost)
	}
}

func TestInjectCursorHooks_PreservesUserVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cursorDir, "hooks.json"), []byte(`{"version": 2, "hooks": {}}`), 0644); err != nil {
		t.Fatalf("seed hooks.json: %v", err)
	}

	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	if data["version"] != float64(2) {
		t.Errorf("expected pre-existing version=2 to be preserved, got %v", data["version"])
	}
}

// --- Windsurf ---

func TestInjectWindsurfHooks(t *testing.T) {
	dir := t.TempDir()
	if err := InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".windsurfrules"))
	if err != nil {
		t.Fatalf("readFile .windsurfrules: %v", err)
	}

	str := string(content)
	if !strings.Contains(str, "Windsurf users:") || !strings.Contains(str, "trackfw-attention.json") {
		t.Errorf(".windsurfrules missing attention instructions: %s", str)
	}
}
