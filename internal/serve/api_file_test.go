package serve

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

// TestFileHandler_SymlinkEscape — symlink dentro de docs/req apontando para fora
// deve retornar 403 e NÃO deve vazar o conteúdo do arquivo destino (AC1, AC3, AC5).
//
// Reconciliação: afirma que a contenção física (EvalSymlinks) bloqueia um symlink
// cujo destino físico está fora das raízes autorizadas — conclusão direta do H-01.
func TestFileHandler_SymlinkEscape(t *testing.T) {
	// Diretório do projeto simulado
	base := t.TempDir()
	reqDir := filepath.Join(base, "docs", "req")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("MkdirAll reqDir: %v", err)
	}

	// Arquivo secreto FORA da raiz autorizada
	outsideDir := t.TempDir()
	secretFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("HADES_SECRET_TOKEN_ABC123"), 0644); err != nil {
		t.Fatalf("WriteFile secret: %v", err)
	}

	// Symlink dentro de docs/req apontando para o arquivo secreto externo
	linkPath := filepath.Join(reqDir, "link.md")
	// symlinkOrSkip: guarda de capacidade — distingue "sem privilégio" (skip)
	// de "falhou por outro motivo" (fail). Issue #315, padrão do projeto.
	if !symlinkOrSkip(t, secretFile, linkPath) {
		return
	}

	chdir(t, base)
	cfg := buildCfgForDir("docs/adr", "docs/req", "docs/roadmaps")

	relPath := filepath.Join("docs", "req", "link.md")
	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+relPath, nil)
	rec := httptest.NewRecorder()

	fileHandler(rec, req, cfg)

	// AC3: deve retornar 403
	if rec.Code != http.StatusForbidden {
		t.Errorf("esperado 403, obteve %d; body: %s", rec.Code, rec.Body.String())
	}
	// AC3: corpo NÃO deve conter o segredo
	if body := rec.Body.String(); strings.Contains(body, "HADES_SECRET") {
		t.Errorf("corpo vazou segredo: %s", body)
	}
}

// TestFileHandler_SymlinkInsideRoot — symlink dentro de docs/req apontando para
// outro arquivo dentro de docs/req deve continuar funcionando (AC4).
//
// Reconciliação: afirma que um symlink legítimo (destino dentro da raiz autorizada)
// continua retornando 200 com conteúdo — o contra-braço de AC1.
func TestFileHandler_SymlinkInsideRoot(t *testing.T) {
	base := t.TempDir()
	reqDir := filepath.Join(base, "docs", "req")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("MkdirAll reqDir: %v", err)
	}

	// Arquivo real dentro da raiz
	realFile := filepath.Join(reqDir, "REQ-real.md")
	wantContent := "# REQ real\nConteúdo legítimo.\n"
	if err := os.WriteFile(realFile, []byte(wantContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Symlink também dentro da raiz, apontando para o arquivo real
	linkPath := filepath.Join(reqDir, "REQ-link.md")
	// symlinkOrSkip: guarda de capacidade — distingue "sem privilégio" (skip)
	// de "falhou por outro motivo" (fail). Issue #315, padrão do projeto.
	if !symlinkOrSkip(t, realFile, linkPath) {
		return
	}

	chdir(t, base)
	cfg := buildCfgForDir("docs/adr", "docs/req", "docs/roadmaps")

	relPath := filepath.Join("docs", "req", "REQ-link.md")
	req := httptest.NewRequest(http.MethodGet, "/api/file?path="+relPath, nil)
	rec := httptest.NewRecorder()

	fileHandler(rec, req, cfg)

	if rec.Code != http.StatusOK {
		t.Errorf("esperado 200, obteve %d; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != wantContent {
		t.Errorf("conteúdo inesperado: got %q, want %q", got, wantContent)
	}
}

