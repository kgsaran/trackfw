package generators

import (
	"os"
	"path/filepath"
	"testing"
)

var expectedCursorRules = []string{
	"trackfw-architect.mdc", "trackfw-backend.mdc", "trackfw-frontend.mdc",
	"trackfw-qa.mdc", "trackfw-infra.mdc", "trackfw-security.mdc",
	"trackfw-code-quality.mdc", "trackfw-dba.mdc", "trackfw-ux.mdc",
	"trackfw-data.mdc",
}

func TestInstallCursor_CriaArquivos(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallCursor(); err != nil {
		t.Fatalf("InstallCursor() erro: %v", err)
	}

	for _, name := range expectedCursorRules {
		p := filepath.Join(dir, ".cursor", "rules", name)
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

func TestInstallCursor_Idempotente(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallCursor(); err != nil {
		t.Fatalf("primeiro InstallCursor() erro: %v", err)
	}

	customPath := filepath.Join(dir, ".cursor", "rules", "trackfw-architect.mdc")
	customContent := "# customizado pelo usuário"
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile customização: %v", err)
	}

	if err := InstallCursor(); err != nil {
		t.Fatalf("segundo InstallCursor() erro: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo install: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("arquivo customizado foi sobrescrito — esperado %q, obteve %q", customContent, string(got))
	}
}

func TestInstallCursor_ConteudoComFrontmatter(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := InstallCursor(); err != nil {
		t.Fatalf("InstallCursor() erro: %v", err)
	}

	for _, name := range expectedCursorRules {
		p := filepath.Join(dir, ".cursor", "rules", name)
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
