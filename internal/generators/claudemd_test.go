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
	block := trackfwRulesBlock("")
	expectedDirective := "Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura."
	if !strings.Contains(block, expectedDirective) {
		t.Errorf("trackfwRulesBlock() não contém a diretiva obrigatória de ADRs globais.\nEsperado conter: %q\nConteúdo obtido:\n%s", expectedDirective, block)
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
		"| `/trackfw:barrier` | Run the wave-release checklist before liberating the next wave |",
		"1. **3-layer separation** — frontend / backend / database. Never mix concerns.",
	}

	for _, expected := range checks {
		if !strings.Contains(content, expected) {
			t.Errorf("CLAUDE.md não contém o trecho esperado: %q", expected)
		}
	}
}

func TestTrackfwRulesBlock_ArchitectureDirectives(t *testing.T) {
	block := trackfwRulesBlock("")
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

func TestGenerateClaudeMD_HarnessSections(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{
		ProjectName: "test-harness-project",
	}

	if err := generateClaudeMD(cfg); err != nil {
		t.Fatalf("generateClaudeMD() erro: %v", err)
	}

	contentBytes, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("os.ReadFile(CLAUDE.md) erro: %v", err)
	}
	content := string(contentBytes)

	harnessSections := []string{
		"## Branch strategy",
		"## Definition of done",
		"## Requirement scope",
		"## State requirements",
		"## Roadmap format",
		"## When governance is not required",
		"## Production incidents",
		"## Iterative prototyping",
		"## Autopilot",
	}

	for _, section := range harnessSections {
		if !strings.Contains(content, section) {
			t.Errorf("CLAUDE.md não contém a seção de harness: %q", section)
		}
	}

	// Verificar trechos de conteúdo específico de cada seção
	harnessSnippets := []string{
		"One active branch at a time",
		"squash-merged",
		"Green build and tests do not close a microbatch",
		"explicit negative scope",
		"`blocked` requires a reason and an owner",
		"waves of microbatches",
		"closed list of exemptions",
		"This section takes precedence",
		"Inspect the live environment before proposing a fix",
		"disposable, isolated prototype",
		"Ask everything you need before starting",
	}

	for _, snippet := range harnessSnippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("CLAUDE.md não contém o trecho de harness esperado: %q", snippet)
		}
	}

	// Verificar que seções pré-existentes ainda estão presentes
	preExisting := []string{
		"## Governance chain",
		"## Agent rules (mandatory)",
		"## Slash commands (Claude Code)",
		"## CLI commands (terminal / CI)",
		"## Architecture Directives (mandatory)",
		"## Pre-commit checklist",
		"## Git hooks",
		"## CI gate",
	}

	for _, section := range preExisting {
		if !strings.Contains(content, section) {
			t.Errorf("CLAUDE.md perdeu a seção pré-existente: %q", section)
		}
	}
}

func TestGenerateClaudeMD_ArchitectResponsesSection(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{ProjectName: "verbosity-test"}
	if err := generateClaudeMD(cfg); err != nil {
		t.Fatalf("generateClaudeMD() erro: %v", err)
	}

	contentBytes, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("os.ReadFile(CLAUDE.md) erro: %v", err)
	}
	content := string(contentBytes)

	checks := []string{
		"## Architect responses",
		"Default: what changed",
		"what was decided",
		"what is needed from you. Three to five lines.",
		"a **blocker** that stops the next wave",
		"a **pending user decision** that cannot be inferred from context",
		"an **error the architect made** that cannot be self-corrected",
		"Never cut, even when short: measured evidence",
		"Cut: restating what an executor already reported",
		"Depth is on demand from the user.",
	}

	for _, expected := range checks {
		if !strings.Contains(content, expected) {
			t.Errorf("CLAUDE.md ## Architect responses não contém o trecho esperado: %q", expected)
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
