package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirADR muda para dir e restaura ao fim do teste
func chdirADR(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestNewADR_CreatesFile — arquivo criado em docs/adr/ com título e seção Context
func TestNewADR_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	if err := NewADR("Escolha de Banco"); err != nil {
		t.Fatalf("NewADR() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/adr/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/adr, obteve %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	if !strings.Contains(body, "Escolha de Banco") {
		t.Errorf("arquivo deveria conter título 'Escolha de Banco', obteve: %q", body)
	}
	if !strings.Contains(body, "## Context") {
		t.Errorf("arquivo deveria conter '## Context', obteve: %q", body)
	}
}

// TestNewADR_SlugInFilename — título com espaços → filename usa hífens
func TestNewADR_SlugInFilename(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	if err := NewADR("Uso de Redis Cache"); err != nil {
		t.Fatalf("NewADR() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/adr/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/adr, obteve %d: %v", len(matches), matches)
	}

	filename := filepath.Base(matches[0])

	// Slug: "uso-de-redis-cache" — sem espaços
	if strings.Contains(filename, " ") {
		t.Errorf("filename não deveria conter espaços: %q", filename)
	}
	if !strings.Contains(filename, "uso-de-redis-cache") {
		t.Errorf("filename deveria conter 'uso-de-redis-cache', obteve: %q", filename)
	}
}
