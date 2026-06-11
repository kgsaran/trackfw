package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadConfig_Empty(t *testing.T) {
	dir := t.TempDir()
	provider, model, apiKey, err := ReadConfig(filepath.Join(dir, "trackfw.yaml"))
	if err != nil {
		t.Fatalf("ReadConfig sem arquivo: erro inesperado: %v", err)
	}
	if provider != "" || model != "" || apiKey != "" {
		t.Errorf("esperado ('','',''), obteve (%q, %q, %q)", provider, model, apiKey)
	}
}

func TestReadConfig_WithValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trackfw.yaml")
	content := "ai_provider: anthropic\nai_model: claude-haiku-4-5-20251001\nai_api_key: sk-test\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	provider, model, apiKey, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if provider != "anthropic" {
		t.Errorf("provider: esperado 'anthropic', obteve %q", provider)
	}
	if model != "claude-haiku-4-5-20251001" {
		t.Errorf("model: esperado 'claude-haiku-4-5-20251001', obteve %q", model)
	}
	if apiKey != "sk-test" {
		t.Errorf("apiKey: esperado 'sk-test', obteve %q", apiKey)
	}
}

func TestFakeClient_Generate(t *testing.T) {
	client := &FakeClient{Response: "hello"}
	result, err := client.Generate(context.Background(), "qualquer prompt")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result != "hello" {
		t.Errorf("esperado 'hello', obteve %q", result)
	}
}
