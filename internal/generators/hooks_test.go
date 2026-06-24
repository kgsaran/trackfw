package generators

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- helpers ---

func writeJSONFile(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	if err := os.WriteFile(path, append(b, '\n'), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]interface{} {
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

func claudeHooksFromFile(t *testing.T, dir string) map[string]interface{} {
	t.Helper()
	return readJSONFile(t, filepath.Join(dir, ".claude", "settings.json"))
}

func hasClaudeHookEntry(data map[string]interface{}, event, matcher, command string) bool {
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

// --- TestInjectClaudeHooks_Create: settings.json não existe → cria com hooks corretos ---

func TestInjectClaudeHooks_Create(t *testing.T) {
	dir := t.TempDir()
	if err := injectClaudeHooks(dir); err != nil {
		t.Fatalf("injectClaudeHooks() erro: %v", err)
	}

	data := claudeHooksFromFile(t, dir)

	if !hasClaudeHookEntry(data, "PreToolUse", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse[AskUserQuestion] → signal.sh não encontrado")
	}
	if !hasClaudeHookEntry(data, "PostToolUse", "AskUserQuestion", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse[AskUserQuestion] → cleanup.sh não encontrado")
	}
}

// --- TestInjectClaudeHooks_Merge: settings existente com outros hooks → merge sem sobrescrever ---

func TestInjectClaudeHooks_Merge(t *testing.T) {
	dir := t.TempDir()

	existing := map[string]interface{}{
		"permissions": map[string]interface{}{
			"defaultMode": "default",
		},
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "scripts/my-guardrail.sh"},
					},
				},
			},
		},
	}
	writeJSONFile(t, filepath.Join(dir, ".claude", "settings.json"), existing)

	if err := injectClaudeHooks(dir); err != nil {
		t.Fatalf("injectClaudeHooks() erro: %v", err)
	}

	data := claudeHooksFromFile(t, dir)

	// Entry original (Bash guardrail) deve continuar presente
	if !hasClaudeHookEntry(data, "PreToolUse", "Bash", "scripts/my-guardrail.sh") {
		t.Error("entry Bash existente foi removida — merge não preservou hooks anteriores")
	}

	// Novas entries trackfw devem ter sido adicionadas
	if !hasClaudeHookEntry(data, "PreToolUse", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("PreToolUse[AskUserQuestion] → signal.sh não foi adicionado")
	}
	if !hasClaudeHookEntry(data, "PostToolUse", "AskUserQuestion", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("PostToolUse[AskUserQuestion] → cleanup.sh não foi adicionado")
	}

	// Permissions deve continuar intacto
	perms, _ := data["permissions"].(map[string]interface{})
	if perms == nil || perms["defaultMode"] != "default" {
		t.Error("campo permissions foi perdido no merge")
	}
}

// --- TestInjectClaudeHooks_Idempotent: rodar duas vezes não duplica entries ---

func TestInjectClaudeHooks_Idempotent(t *testing.T) {
	dir := t.TempDir()

	if err := injectClaudeHooks(dir); err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	if err := injectClaudeHooks(dir); err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}

	data := claudeHooksFromFile(t, dir)
	hooks, _ := data["hooks"].(map[string]interface{})
	pre, _ := hooks["PreToolUse"].([]interface{})
	post, _ := hooks["PostToolUse"].([]interface{})

	// Deve haver exatamente 1 entry em cada evento (não 2)
	if len(pre) != 1 {
		t.Errorf("PreToolUse tem %d entries após 2 chamadas, esperado 1", len(pre))
	}
	if len(post) != 1 {
		t.Errorf("PostToolUse tem %d entries após 2 chamadas, esperado 1", len(post))
	}
}

// --- TestInjectHooksDetected_SkipsAbsent: não cria arquivos para CLIs ausentes ---

func TestInjectHooksDetected_SkipsAbsent(t *testing.T) {
	dir := t.TempDir()

	// nenhum CLI detectado → InjectHooksDetected não deve criar nada
	if err := InjectHooksDetected(dir); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// Nenhum arquivo de hook deve existir
	absent := []string{
		filepath.Join(dir, ".claude", "settings.json"),
		filepath.Join(dir, ".codex", "hooks.json"),
		filepath.Join(dir, ".gemini", "settings.json"),
		filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json"),
		filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"),
		filepath.Join(dir, ".cursor", "hooks.json"),
	}
	for _, p := range absent {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("arquivo criado sem CLI detectado: %s", p)
		}
	}
}

// --- TestInjectHooksDetected_Claude: detecta CLAUDE.md e injeta ---

func TestInjectHooksDetected_Claude(t *testing.T) {
	dir := t.TempDir()

	// Criar CLAUDE.md para simular projeto Claude
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InjectHooksDetected(dir); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	data := claudeHooksFromFile(t, dir)
	if !hasClaudeHookEntry(data, "PreToolUse", "AskUserQuestion", "scripts/trackfw-attention-signal.sh") {
		t.Error("hook Claude não foi injetado ao detectar CLAUDE.md")
	}
}

func TestInjectGeminiHooks_UsesCurrentSchema(t *testing.T) {
	dir := t.TempDir()
	if err := injectGeminiHooks(dir); err != nil {
		t.Fatalf("injectGeminiHooks() error: %v", err)
	}
	if err := injectGeminiHooks(dir); err != nil {
		t.Fatalf("injectGeminiHooks() must be idempotent: %v", err)
	}

	data := readJSONFile(t, filepath.Join(dir, ".gemini", "settings.json"))
	if !hasClaudeHookEntry(data, "Notification", "ToolPermission", "scripts/trackfw-attention-signal.sh") {
		t.Error("Notification[ToolPermission] attention hook not found")
	}
	if !hasClaudeHookEntry(data, "AfterTool", "*", "scripts/trackfw-attention-cleanup.sh") {
		t.Error("AfterTool cleanup hook not found")
	}
}

func TestInjectCopilotHooks_UsesVersionedSchema(t *testing.T) {
	dir := t.TempDir()
	if err := injectCopilotHooks(dir); err != nil {
		t.Fatalf("injectCopilotHooks() error: %v", err)
	}

	data := readJSONFile(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"))
	if data["version"] != float64(1) {
		t.Fatalf("version = %v, want 1", data["version"])
	}
	hooks, ok := data["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("hooks must be an object, got %T", data["hooks"])
	}
	pre, _ := hooks["preToolUse"].([]interface{})
	if len(pre) != 1 {
		t.Fatalf("preToolUse = %v", pre)
	}
	entry, _ := pre[0].(map[string]interface{})
	if entry["type"] != "command" || entry["bash"] != "scripts/trackfw-attention-signal.sh" {
		t.Errorf("unexpected preToolUse entry: %v", entry)
	}
}
