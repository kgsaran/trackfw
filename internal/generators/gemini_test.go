package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var expectedGeminiSkills = []string{
	"trackfw-architect", "trackfw-backend", "trackfw-frontend",
	"trackfw-qa", "trackfw-infra", "trackfw-security",
	"trackfw-code-quality", "trackfw-dba", "trackfw-ux",
	"trackfw-data",
}

var expectedGeminiCommands = []string{
	"trackfw-adr.toml", "trackfw-req.toml", "trackfw-roadmap.toml",
}

func TestInstallGemini_CriaArquivos(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallGemini(); err != nil {
		t.Fatalf("InstallGemini() erro: %v", err)
	}

	// Global GEMINI.md
	p := filepath.Join(home, ".gemini", "GEMINI.md")
	if info, err := os.Stat(p); err != nil {
		t.Errorf("~/.gemini/GEMINI.md não encontrado: %v", err)
	} else if info.Size() == 0 {
		t.Errorf("~/.gemini/GEMINI.md vazio")
	}

	// Project GEMINI.md
	pProj := filepath.Join(dir, "GEMINI.md")
	if info, err := os.Stat(pProj); err != nil {
		t.Errorf("GEMINI.md (projeto) não encontrado: %v", err)
	} else if info.Size() == 0 {
		t.Errorf("GEMINI.md (projeto) vazio")
	}

	// Skills
	for _, role := range expectedGeminiSkills {
		skillPath := filepath.Join(home, ".gemini", "skills", role, "SKILL.md")
		if info, err := os.Stat(skillPath); err != nil {
			t.Errorf("skill %s/SKILL.md não encontrado: %v", role, err)
		} else if info.Size() == 0 {
			t.Errorf("skill %s/SKILL.md vazio", role)
		}
	}

	// Commands
	for _, cmd := range expectedGeminiCommands {
		cmdPath := filepath.Join(home, ".gemini", "commands", cmd)
		if info, err := os.Stat(cmdPath); err != nil {
			t.Errorf("command %s não encontrado: %v", cmd, err)
		} else if info.Size() == 0 {
			t.Errorf("command %s vazio", cmd)
		}
	}
}

func TestInstallGemini_Idempotente(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallGemini(); err != nil {
		t.Fatalf("primeiro InstallGemini() erro: %v", err)
	}

	customPath := filepath.Join(home, ".gemini", "GEMINI.md")
	customContent := "# customizado pelo usuário"
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile customização: %v", err)
	}

	if err := InstallGemini(); err != nil {
		t.Fatalf("segundo InstallGemini() erro: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo install: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("arquivo customizado foi sobrescrito — esperado %q, obteve %q", customContent, string(got))
	}
}

func TestInstallGemini_SkillsTemFrontmatter(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallGemini(); err != nil {
		t.Fatalf("InstallGemini() erro: %v", err)
	}

	for _, role := range expectedGeminiSkills {
		p := filepath.Join(home, ".gemini", "skills", role, "SKILL.md")
		content, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("erro lendo %s/SKILL.md: %v", role, err)
			continue
		}
		if len(content) < 3 || string(content[:3]) != "---" {
			t.Errorf("%s/SKILL.md não começa com frontmatter YAML (---)", role)
		}
		if !strings.Contains(string(content), "name:") {
			t.Errorf("%s/SKILL.md não contém campo 'name:' no frontmatter", role)
		}
	}
}
