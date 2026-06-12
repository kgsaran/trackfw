package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var expectedAmazonQRules = []string{
	"trackfw-architect.md", "trackfw-backend.md", "trackfw-frontend.md",
	"trackfw-qa.md", "trackfw-infra.md", "trackfw-security.md",
	"trackfw-code-quality.md", "trackfw-dba.md", "trackfw-ux.md",
	"trackfw-data.md",
}

func TestInstallAmazonQ_CriaArquivos(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallAmazonQ(); err != nil {
		t.Fatalf("InstallAmazonQ() erro: %v", err)
	}

	for _, name := range expectedAmazonQRules {
		p := filepath.Join(dir, ".amazonq", "rules", name)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("arquivo não encontrado: %s", p)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("arquivo vazio: %s", name)
		}
	}
}

func TestInstallAmazonQ_Idempotente(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallAmazonQ(); err != nil {
		t.Fatalf("primeiro InstallAmazonQ() erro: %v", err)
	}

	customPath := filepath.Join(dir, ".amazonq", "rules", "trackfw-architect.md")
	customContent := "# customizado pelo usuário"
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile customização: %v", err)
	}

	if err := InstallAmazonQ(); err != nil {
		t.Fatalf("segundo InstallAmazonQ() erro: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo install: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("arquivo customizado foi sobrescrito — esperado %q, obteve %q", customContent, string(got))
	}
}

func TestInstallAmazonQ_ConteudoSemFrontmatter(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallAmazonQ(); err != nil {
		t.Fatalf("InstallAmazonQ() erro: %v", err)
	}

	for _, name := range expectedAmazonQRules {
		p := filepath.Join(dir, ".amazonq", "rules", name)
		content, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("erro lendo %s: %v", name, err)
			continue
		}
		if strings.HasPrefix(string(content), "---") {
			t.Errorf("%s não deve ter frontmatter YAML (Amazon Q requer Markdown puro)", name)
		}
	}
}
