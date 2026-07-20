package generators

import (
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

	if !helperHasClaudeHook(data, "PermissionRequest", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("PermissionRequest[AskUserQuestion] → signal.sh missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "AskUserQuestion", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse[AskUserQuestion] → cleanup.sh missing")
	}
}

func TestInjectClaudeHooks_MergeAndIdempotent(t *testing.T) {
	dir := t.TempDir()

	existing := map[string]interface{}{
		"permissions": map[string]interface{}{"defaultMode": "default"},
		"hooks": map[string]interface{}{
			"PermissionRequest": []interface{}{
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

	if !helperHasClaudeHook(data, "PermissionRequest", "Bash", "scripts/other.sh") {
		t.Error("existing Bash hook lost during merge")
	}
	if !helperHasClaudeHook(data, "PermissionRequest", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("PermissionRequest signal hook missing")
	}

	hooks, _ := data["hooks"].(map[string]interface{})
	pr, _ := hooks["PermissionRequest"].([]interface{})
	if len(pr) != 2 {
		t.Errorf("expected 2 PermissionRequest entries, got %d", len(pr))
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
	if !helperHasClaudeHook(data, "PreToolUse", ".*", "scripts/trackfw-attention-signal.sh") {
		t.Error("Codex PreToolUse hook missing")
	}
	if !helperHasClaudeHook(data, "PostToolUse", ".*", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("Codex PostToolUse hook missing")
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
		t.Error("Gemini AfterTool hook missing")
	}
}

// --- Kiro ---

func TestInjectKiroHooks(t *testing.T) {
	dir := t.TempDir()
	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks failed: %v", err)
	}
	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("second InjectKiroHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json"))
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
	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("second InjectCopilotHooks failed: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"))
	hooks, ok := data["hooks"].([]interface{})
	if !ok || len(hooks) != 2 {
		t.Fatalf("expected hooks array of size 2, got %v", data["hooks"])
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
