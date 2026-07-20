package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateClaudeMD_GlobalADRsDirective(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{
		ProjectName: "test-project",
	}

	if err := generateClaudeMD(cfg); err != nil {
		t.Fatalf("generateClaudeMD() erro: %v", err)
	}

	content, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("os.ReadFile(CLAUDE.md) erro: %v", err)
	}

	expectedDirective := "Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura."
	if !strings.Contains(string(content), expectedDirective) {
		t.Errorf("CLAUDE.md não contém a diretiva obrigatória de ADRs globais.\nEsperado conter: %q\nConteúdo obtido:\n%s", expectedDirective, string(content))
	}
}

func TestTrackfwRulesBlock_GlobalADRsDirective(t *testing.T) {
	block := trackfwRulesBlock()
	expectedDirective := "Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura."
	if !strings.Contains(block, expectedDirective) {
		t.Errorf("trackfwRulesBlock() não contém a diretiva obrigatória de ADRs globais.\nEsperado conter: %q\nConteúdo obtido:\n%s", expectedDirective, block)
	}
}

func TestInstallGlobalSkill_GlobalADRsDirective(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	orig, _ := os.Getwd()
	origHome := os.Getenv("HOME")
	_ = os.Chdir(dir)
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		_ = os.Setenv("HOME", origHome)
	})

	if err := installGlobalSkill(); err != nil {
		t.Fatalf("installGlobalSkill() erro: %v", err)
	}

	skillPath := filepath.Join(home, ".claude", "skills", "trackfw", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) erro: %v", skillPath, err)
	}

	expectedDirective := "Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura."
	if !strings.Contains(string(content), expectedDirective) {
		t.Errorf("SKILL.md não contém a diretiva obrigatória de ADRs globais.\nEsperado conter: %q\nConteúdo obtido:\n%s", expectedDirective, string(content))
	}
}
