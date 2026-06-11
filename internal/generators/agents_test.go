package generators

import (
	"os"
	"path/filepath"
	"testing"
)

var expectedAgents = []string{
	"trackfw-zeus.md", "trackfw-apolo.md", "trackfw-afrodite.md",
	"trackfw-artemis.md", "trackfw-ares.md", "trackfw-hades.md",
	"trackfw-hephaestus.md", "trackfw-poseidon.md", "trackfw-athena.md",
	"trackfw-metis.md",
}

func TestInstallAgents_CriaArquivosEmHome(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	if err := InstallAgents(); err != nil {
		t.Fatalf("InstallAgents() erro: %v", err)
	}

	for _, name := range expectedAgents {
		p := filepath.Join(home, ".claude", "agents", name)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("agente não encontrado: %s", p)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("agente vazio: %s", name)
		}
	}
}

func TestInstallAgents_Idempotente(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	if err := InstallAgents(); err != nil {
		t.Fatalf("primeiro InstallAgents() erro: %v", err)
	}

	customPath := filepath.Join(home, ".claude", "agents", "trackfw-zeus.md")
	customContent := "# conteúdo customizado pelo usuário"
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile customização: %v", err)
	}

	if err := InstallAgents(); err != nil {
		t.Fatalf("segundo InstallAgents() erro: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo install: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("agente customizado foi sobrescrito — esperado %q, obteve %q", customContent, string(got))
	}
}

func TestInstallAgents_ConteudoComFrontmatter(t *testing.T) {
	home := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	if err := InstallAgents(); err != nil {
		t.Fatalf("InstallAgents() erro: %v", err)
	}

	for _, name := range expectedAgents {
		p := filepath.Join(home, ".claude", "agents", name)
		content, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("erro lendo %s: %v", name, err)
			continue
		}
		if len(content) < 3 || string(content[:3]) != "---" {
			t.Errorf("%s não começa com frontmatter YAML (---)", name)
		}
	}
}
