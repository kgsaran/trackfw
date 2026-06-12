package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var expectedWindsurfRules = []string{
	"trackfw-architect.md", "trackfw-backend.md", "trackfw-frontend.md",
	"trackfw-qa.md", "trackfw-infra.md", "trackfw-security.md",
	"trackfw-code-quality.md", "trackfw-dba.md", "trackfw-ux.md",
	"trackfw-data.md",
}

var expectedWindsurfWorkflows = []string{
	"trackfw-adr.md", "trackfw-req.md", "trackfw-implement.md",
}

func TestInstallWindsurf_CriaArquivos(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Use a temp home so we don't touch real ~/.codeium
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	if err := InstallWindsurf(); err != nil {
		t.Fatalf("InstallWindsurf() erro: %v", err)
	}

	for _, name := range expectedWindsurfRules {
		p := filepath.Join(dir, ".windsurf", "rules", name)
		if info, err := os.Stat(p); err != nil {
			t.Errorf("rules/%s não encontrado: %v", name, err)
		} else if info.Size() == 0 {
			t.Errorf("rules/%s vazio", name)
		}
	}

	for _, name := range expectedWindsurfWorkflows {
		p := filepath.Join(dir, ".windsurf", "workflows", name)
		if info, err := os.Stat(p); err != nil {
			t.Errorf("workflows/%s não encontrado: %v", name, err)
		} else if info.Size() == 0 {
			t.Errorf("workflows/%s vazio", name)
		}
	}
}

func TestInstallWindsurf_Idempotente(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	if err := InstallWindsurf(); err != nil {
		t.Fatalf("primeiro InstallWindsurf() erro: %v", err)
	}

	customPath := filepath.Join(dir, ".windsurf", "rules", "trackfw-architect.md")
	customContent := "# customizado pelo usuário"
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile customização: %v", err)
	}

	if err := InstallWindsurf(); err != nil {
		t.Fatalf("segundo InstallWindsurf() erro: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo install: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("arquivo customizado foi sobrescrito — esperado %q, obteve %q", customContent, string(got))
	}
}

func TestInstallWindsurf_GlobalRulesNaoDuplica(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	// First install
	if err := InstallWindsurf(); err != nil {
		t.Fatalf("primeiro InstallWindsurf() erro: %v", err)
	}

	globalPath := filepath.Join(home, ".codeium", "windsurf", "memories", "global_rules.md")
	contentAfterFirst, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("global_rules.md não criado: %v", err)
	}

	// Second install — must not duplicate
	if err := InstallWindsurf(); err != nil {
		t.Fatalf("segundo InstallWindsurf() erro: %v", err)
	}

	contentAfterSecond, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo install: %v", err)
	}

	if len(contentAfterSecond) != len(contentAfterFirst) {
		t.Errorf("global_rules.md foi modificado na segunda execução — conteúdo duplicado")
	}
}

func TestInstallWindsurf_RulesTemFrontmatter(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	if err := InstallWindsurf(); err != nil {
		t.Fatalf("InstallWindsurf() erro: %v", err)
	}

	for _, name := range expectedWindsurfRules {
		p := filepath.Join(dir, ".windsurf", "rules", name)
		content, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("erro lendo %s: %v", name, err)
			continue
		}
		if !strings.Contains(string(content), "trigger:") {
			t.Errorf("rules/%s não contém 'trigger:' no frontmatter", name)
		}
	}
}
