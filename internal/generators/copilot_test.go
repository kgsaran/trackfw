package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var expectedCopilotInstructions = []string{
	"trackfw-architect.instructions.md", "trackfw-backend.instructions.md",
	"trackfw-frontend.instructions.md", "trackfw-qa.instructions.md",
	"trackfw-infra.instructions.md", "trackfw-security.instructions.md",
	"trackfw-code-quality.instructions.md", "trackfw-dba.instructions.md",
	"trackfw-ux.instructions.md", "trackfw-data.instructions.md",
}

var expectedCopilotPrompts = []string{
	"trackfw-architect.prompt.md", "trackfw-backend.prompt.md",
	"trackfw-frontend.prompt.md", "trackfw-qa.prompt.md",
	"trackfw-infra.prompt.md", "trackfw-security.prompt.md",
	"trackfw-code-quality.prompt.md", "trackfw-dba.prompt.md",
	"trackfw-ux.prompt.md", "trackfw-data.prompt.md",
}

func TestInstallCopilot_CriaArquivos(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallCopilot(); err != nil {
		t.Fatalf("InstallCopilot() erro: %v", err)
	}

	// copilot-instructions.md
	p := filepath.Join(dir, ".github", "copilot-instructions.md")
	if info, err := os.Stat(p); err != nil {
		t.Errorf("copilot-instructions.md não encontrado: %v", err)
	} else if info.Size() == 0 {
		t.Errorf("copilot-instructions.md vazio")
	}

	for _, name := range expectedCopilotInstructions {
		p := filepath.Join(dir, ".github", "instructions", name)
		if info, err := os.Stat(p); err != nil {
			t.Errorf("instructions/%s não encontrado: %v", name, err)
		} else if info.Size() == 0 {
			t.Errorf("instructions/%s vazio", name)
		}
	}

	for _, name := range expectedCopilotPrompts {
		p := filepath.Join(dir, ".github", "prompts", name)
		if info, err := os.Stat(p); err != nil {
			t.Errorf("prompts/%s não encontrado: %v", name, err)
		} else if info.Size() == 0 {
			t.Errorf("prompts/%s vazio", name)
		}
	}
}

func TestInstallCopilot_Idempotente(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallCopilot(); err != nil {
		t.Fatalf("primeiro InstallCopilot() erro: %v", err)
	}

	customPath := filepath.Join(dir, ".github", "copilot-instructions.md")
	customContent := "# customizado pelo usuário"
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile customização: %v", err)
	}

	if err := InstallCopilot(); err != nil {
		t.Fatalf("segundo InstallCopilot() erro: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo install: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("arquivo customizado foi sobrescrito — esperado %q, obteve %q", customContent, string(got))
	}
}

func TestInstallCopilot_FrontmatterInstructions(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallCopilot(); err != nil {
		t.Fatalf("InstallCopilot() erro: %v", err)
	}

	for _, name := range expectedCopilotInstructions {
		p := filepath.Join(dir, ".github", "instructions", name)
		content, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("erro lendo %s: %v", name, err)
			continue
		}
		if !strings.Contains(string(content), "applyTo:") {
			t.Errorf("instructions/%s não contém 'applyTo:' no frontmatter", name)
		}
	}

	for _, name := range expectedCopilotPrompts {
		p := filepath.Join(dir, ".github", "prompts", name)
		content, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("erro lendo %s: %v", name, err)
			continue
		}
		if !strings.Contains(string(content), "agent:") {
			t.Errorf("prompts/%s não contém 'agent:' no frontmatter", name)
		}
	}
}
