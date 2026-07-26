package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: chdir + cleanup
func chdirNote(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestNewNote_CriaArquivoELinkaNaIndex verifica que note new gera o arquivo e adiciona link ao index.
func TestNewNote_CriaArquivoELinkaNaIndex(t *testing.T) {
	dir := t.TempDir()
	chdirNote(t, dir)

	if err := NewNote("Minha nota de teste"); err != nil {
		t.Fatalf("NewNote: %v", err)
	}

	// Arquivo gerado
	matches, _ := filepath.Glob(filepath.Join("vault", "notes", "minha-nota-de-teste-*.md"))
	if len(matches) == 0 {
		t.Fatal("nota não foi criada em vault/notes/")
	}

	// Conteúdo da nota
	data, _ := os.ReadFile(matches[0])
	content := string(data)
	if !strings.Contains(content, "## Problem") {
		t.Error("nota deve ter seção ## Problem")
	}
	if !strings.Contains(content, "## Root cause") {
		t.Error("nota deve ter seção ## Root cause")
	}
	if !strings.Contains(content, "## Solution") {
		t.Error("nota deve ter seção ## Solution")
	}
	if !strings.Contains(content, "title: \"Minha nota de teste\"") {
		t.Error("nota deve ter campo title no frontmatter")
	}

	// Index linkado
	idxData, _ := os.ReadFile(filepath.Join("vault", "notes", "index.md"))
	basename := filepath.Base(matches[0])
	if !strings.Contains(string(idxData), "("+basename+")") {
		t.Errorf("index.md não contém link para %s", basename)
	}
}

// TestNewNote_IdempotenteFalhaSegundaVez verifica que segunda chamada com mesmo título falha.
func TestNewNote_IdempotenteFalhaSegundaVez(t *testing.T) {
	dir := t.TempDir()
	chdirNote(t, dir)

	if err := NewNote("Nota duplicada"); err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	if err := NewNote("Nota duplicada"); err == nil {
		t.Fatal("segunda chamada deveria falhar mas retornou nil")
	}
}

// TestNewNote_NaoDuplicaLinkNoIndex verifica que chamar duas vezes não duplica a linha no index.
func TestNewNote_NaoDuplicaLinkNoIndex(t *testing.T) {
	dir := t.TempDir()
	chdirNote(t, dir)

	// Cria a nota manualmente e linka no index para simular nota existente
	if err := os.MkdirAll(filepath.Join("vault", "notes"), 0755); err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join("vault", "notes", "minha-nota-2026-01-01.md")
	_ = os.WriteFile(notePath, []byte("# Minha nota\n"), 0644)
	indexContent := "# Vault\n\n## Índice\n\n- [minha-nota-2026-01-01](minha-nota-2026-01-01.md)\n"
	_ = os.WriteFile(filepath.Join("vault", "notes", "index.md"), []byte(indexContent), 0644)

	// Tenta adicionar novamente — não deve duplicar
	if err := appendNoteToIndex("minha-nota-2026-01-01.md"); err != nil {
		t.Fatalf("appendNoteToIndex: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join("vault", "notes", "index.md"))
	count := strings.Count(string(data), "minha-nota-2026-01-01.md")
	if count != 1 {
		t.Errorf("esperado 1 ocorrência no index, encontradas %d", count)
	}
}
