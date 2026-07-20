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

func TestGenerateClaudeMD_ArchitectCommandAndDirectives(t *testing.T) {
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

	contentBytes, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("os.ReadFile(CLAUDE.md) erro: %v", err)
	}
	content := string(contentBytes)

	checks := []string{
		"6a. **Usar `/trackfw:architect` para definir stack e arquitetura antes da primeira REQ.**",
		"## Architecture Directives (mandatory)",
		"| `/trackfw:architect` | Guide stack and architecture decisions |",
		"1. **3-layer separation** — frontend / backend / database. Never mix concerns.",
	}

	for _, expected := range checks {
		if !strings.Contains(content, expected) {
			t.Errorf("CLAUDE.md não contém o trecho esperado: %q", expected)
		}
	}
}

func TestTrackfwRulesBlock_ArchitectureDirectives(t *testing.T) {
	block := trackfwRulesBlock()
	checks := []string{
		"### Architecture Directives (mandatory)",
		"- **3-layer separation:** frontend / backend / database — never mix concerns",
		"Use `/trackfw:architect` to define stack before the first REQ",
	}

	for _, expected := range checks {
		if !strings.Contains(block, expected) {
			t.Errorf("trackfwRulesBlock() não contém o trecho esperado: %q", expected)
		}
	}
}

func TestGenerateClaudeCommands_Architect(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := generateClaudeCommands(); err != nil {
		t.Fatalf("generateClaudeCommands() erro: %v", err)
	}

	architectPath := filepath.Join(".claude", "commands", "trackfw", "architect.md")
	contentBytes, err := os.ReadFile(architectPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) erro: %v", architectPath, err)
	}
	content := string(contentBytes)

	checks := []string{
		"Você é o guia de arquitetura do trackfw.",
		"Passo 1 — Descoberta de Negócio",
		"Combo A — Protótipo Rápido",
		"Combo B — Sistema Pequeno/Médio em Produção",
		"Combo C — Enterprise / Java",
		"Passo 4 — Gerar o ADR de Stack",
	}

	for _, expected := range checks {
		if !strings.Contains(content, expected) {
			t.Errorf("architect.md não contém o trecho esperado: %q", expected)
		}
	}
}
