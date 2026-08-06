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
	// (not duplicated by) the new credential-guard "Bash" entry: 2 entries total
	// -- {Bash: [other.sh, credential-guard.sh]}, {AskUserQuestion: [signal.sh]}.
	if len(pr) != 2 {
		t.Errorf("expected 2 PreToolUse entries, got %d", len(pr))
	}
	post, _ := hooks["PostToolUse"].([]interface{})
	if len(post) != 2 {
		t.Errorf("expected 2 PostToolUse entries, got %d", len(post))
	}
}

// --- Codex ---

func TestInjectCodexHooks(t *testing.T) {
	dir := t.TempDir()
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
	if len(pre) != 1 {
		t.Errorf("expected 1 PreToolUse entry (Bash only, no idempotency dup), got %d", len(pre))
	}
	post, _ := hooks["PostToolUse"].([]interface{})
	// 2 entries: {matcher:".*", hooks:[cleanup.sh]}, {matcher:"Bash", hooks:[credential-guard.sh]}
	if len(post) != 2 {
		t.Errorf("expected 2 PostToolUse entries, got %d", len(post))
	}
}

func TestInjectCodexHooks_PreservesExistingBashEntry(t *testing.T) {
	dir := t.TempDir()

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
	if len(pre) != 1 {
		t.Errorf("expected 1 PreToolUse entry (merged into existing Bash matcher, not duplicated), got %d", len(pre))
	}
}

// --- Gemini ---

func TestInjectGeminiHooks(t *testing.T) {
	dir := t.TempDir()
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
	if len(before) != 1 {
		t.Errorf("expected 1 BeforeTool entry (run_shell_command only, no idempotency dup), got %d", len(before))
	}
	after, _ := hooks["AfterTool"].([]interface{})
	// 2 entries: {matcher:"*", hooks:[cleanup.sh]}, {matcher:"run_shell_command", hooks:[credential-guard.sh]}
	if len(after) != 2 {
		t.Errorf("expected 2 AfterTool entries, got %d", len(after))
	}
}

func TestInjectGeminiHooks_PreservesExistingBeforeToolEntry(t *testing.T) {
	dir := t.TempDir()

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
	if len(before) != 1 {
		t.Errorf("expected 1 BeforeTool entry (merged into existing run_shell_command matcher, not duplicated), got %d", len(before))
	}
}

// --- Kiro ---

func TestInjectKiroHooks(t *testing.T) {
	dir := t.TempDir()
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
	hooks, _ := data["hooks"].([]interface{})
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks in Kiro config, got %d", len(hooks))
	}
}

// --- Copilot ---

func TestInjectCopilotHooks(t *testing.T) {
	dir := t.TempDir()
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

	pre, ok := hooks["preToolUse"].([]interface{})
	if !ok || len(pre) != 2 {
		t.Fatalf("expected preToolUse array of size 2, got %v", hooks["preToolUse"])
	}
	post, ok := hooks["postToolUse"].([]interface{})
	if !ok || len(post) != 2 {
		t.Fatalf("expected postToolUse array of size 2, got %v", hooks["postToolUse"])
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
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks failed: %v", err)
	}
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("second InjectCursorHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	pre, _ := data["preToolUse"].([]interface{})
	post, _ := data["postToolUse"].([]interface{})
	if len(pre) != 1 || len(post) != 1 {
		t.Fatalf("expected 1 pre and 1 post entry, got %d pre, %d post", len(pre), len(post))
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
