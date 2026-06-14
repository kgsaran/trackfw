package serve

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// chdir muda para dir e restaura ao fim do teste.
func chdir(t *testing.T, dir string) {
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

// buildCfgForDir retorna um ProjectConfig cujo ADRDir aponta para o dir informado.
func buildCfgForDir(adrDir, reqDir, roadmapDir string) config.ProjectConfig {
	return config.ProjectConfig{
		ADRDirs:    []string{adrDir},
		REQDir:     reqDir,
		RoadmapDir: roadmapDir,
	}
}

// TestFileHandler_Valid — path dentro de um dir permitido retorna 200 com conteúdo correto.
func TestFileHandler_Valid(t *testing.T) {
	base := t.TempDir()
	adrDir := filepath.Join(base, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	wantContent := "# ADR-001\nConteúdo de teste\n"
	filePath := filepath.Join(adrDir, "ADR-001.md")
	if err := os.WriteFile(filePath, []byte(wantContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Muda o cwd para base para que os caminhos relativos funcionem.
	chdir(t, base)

	cfg := buildCfgForDir("docs/adr", "docs/req", "docs/roadmaps")

	// path relativo ao cwd
	relPath := filepath.Join("docs", "adr", "ADR-001.md")
	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+relPath, nil)
	rec := httptest.NewRecorder()

	fileHandler(rec, req, cfg)

	if rec.Code != http.StatusOK {
		t.Errorf("esperado status 200, obteve %d; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != wantContent {
		t.Errorf("conteúdo inesperado: got %q, want %q", got, wantContent)
	}
}

// TestFileHandler_PathTraversal — path com sequência '..' é bloqueado com 403.
func TestFileHandler_PathTraversal(t *testing.T) {
	base := t.TempDir()
	adrDir := filepath.Join(base, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	chdir(t, base)

	cfg := buildCfgForDir("docs/adr", "docs/req", "docs/roadmaps")

	req := httptest.NewRequest(http.MethodGet, "/api/file?path=../../../../etc/passwd", nil)
	rec := httptest.NewRecorder()

	fileHandler(rec, req, cfg)

	if rec.Code != http.StatusForbidden {
		t.Errorf("esperado 403, obteve %d", rec.Code)
	}
}

// TestFileHandler_OutsideAllowedDir — path absoluto fora dos dirs permitidos retorna 403.
func TestFileHandler_OutsideAllowedDir(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)

	cfg := buildCfgForDir("docs/adr", "docs/req", "docs/roadmaps")

	// Tenta acessar /tmp/secret.md — fora de qualquer dir permitido.
	req := httptest.NewRequest(http.MethodGet, "/api/file?path=/tmp/secret.md", nil)
	rec := httptest.NewRecorder()

	fileHandler(rec, req, cfg)

	if rec.Code != http.StatusForbidden {
		t.Errorf("esperado 403, obteve %d", rec.Code)
	}
}

// TestFileHandler_NotFound — path válido mas arquivo inexistente retorna 404.
func TestFileHandler_NotFound(t *testing.T) {
	base := t.TempDir()
	adrDir := filepath.Join(base, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	chdir(t, base)

	cfg := buildCfgForDir("docs/adr", "docs/req", "docs/roadmaps")

	relPath := filepath.Join("docs", "adr", "nao-existe.md")
	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+relPath, nil)
	rec := httptest.NewRecorder()

	fileHandler(rec, req, cfg)

	if rec.Code != http.StatusNotFound {
		t.Errorf("esperado 404, obteve %d", rec.Code)
	}
}
