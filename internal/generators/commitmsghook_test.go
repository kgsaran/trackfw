package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCommitMsgHook_Husky(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{
		Hooks:              "husky",
		RequireReqInCommit: true,
	}

	if err := generateCommitMsgHook(cfg); err != nil {
		t.Fatalf("generateCommitMsgHook() erro: %v", err)
	}

	hookPath := filepath.Join(".husky", "commit-msg")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook não encontrado em %s: %v", hookPath, err)
	}

	// Verificar permissão executável
	if info.Mode()&0111 == 0 {
		t.Errorf("hook não tem permissão executável: mode=%v", info.Mode())
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("erro ao ler hook: %v", err)
	}

	if !strings.Contains(string(content), `grep -qE "^(REQ|req): "`) {
		t.Errorf("hook não contém grep para REQ reference; conteúdo:\n%s", content)
	}

	if !strings.Contains(string(content), "feat/*|fix/*") {
		t.Errorf("hook não contém padrão feat/*|fix/*; conteúdo:\n%s", content)
	}
}

func TestGenerateCommitMsgHook_Disabled(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{
		Hooks:              "husky",
		RequireReqInCommit: false,
	}

	if err := generateCommitMsgHook(cfg); err != nil {
		t.Fatalf("generateCommitMsgHook() erro: %v", err)
	}

	hookPath := filepath.Join(".husky", "commit-msg")
	if _, err := os.Stat(hookPath); err == nil {
		t.Errorf("hook não deveria ser criado quando RequireReqInCommit=false, mas existe em %s", hookPath)
	}
}

func TestGenerateCommitMsgHook_Lefthook(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Pré-criar lefthook.yml (como faria generateLefthookHook)
	lefthookContent := "pre-commit:\n  commands:\n    trackfw-validate:\n      run: trackfw validate\n"
	if err := os.WriteFile("lefthook.yml", []byte(lefthookContent), 0644); err != nil {
		t.Fatalf("erro ao criar lefthook.yml: %v", err)
	}

	cfg := Config{
		Hooks:              "lefthook",
		RequireReqInCommit: true,
	}

	if err := generateCommitMsgHook(cfg); err != nil {
		t.Fatalf("generateCommitMsgHook() erro: %v", err)
	}

	// lefthook.yml deve conter commit-msg:
	yml, err := os.ReadFile("lefthook.yml")
	if err != nil {
		t.Fatalf("erro ao ler lefthook.yml: %v", err)
	}
	if !strings.Contains(string(yml), "commit-msg:") {
		t.Errorf("lefthook.yml não contém seção commit-msg:; conteúdo:\n%s", yml)
	}

	// Script deve existir em .lefthook/commit-msg/
	scriptPath := filepath.Join(".lefthook", "commit-msg", "trackfw-req-check.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("script não encontrado em %s: %v", scriptPath, err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("script não tem permissão executável: mode=%v", info.Mode())
	}

	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("erro ao ler script: %v", err)
	}
	if !strings.Contains(string(scriptContent), `grep -qE "^(REQ|req): "`) {
		t.Errorf("script não contém grep para REQ reference; conteúdo:\n%s", scriptContent)
	}
}
