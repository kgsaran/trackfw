package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestBoardHandler_FlatMode — modo flat com roadmaps em estados diferentes retorna JSON correto.
func TestBoardHandler_FlatMode(t *testing.T) {
	base := t.TempDir()

	// Criar estrutura flat: base/wip/roadmap1.md, base/backlog/roadmap2.md, base/done/roadmap3.md
	states := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}
	for _, s := range states {
		if err := os.MkdirAll(filepath.Join(base, s), 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", s, err)
		}
	}

	// Arquivo wip
	if err := os.WriteFile(
		filepath.Join(base, "wip", "ROADMAP-auth.md"),
		[]byte("# Roadmap de Autenticação\nConteúdo wip."),
		0644,
	); err != nil {
		t.Fatalf("WriteFile wip: %v", err)
	}

	// Arquivo backlog
	if err := os.WriteFile(
		filepath.Join(base, "backlog", "ROADMAP-search.md"),
		[]byte("# Roadmap de Busca\nConteúdo backlog."),
		0644,
	); err != nil {
		t.Fatalf("WriteFile backlog: %v", err)
	}

	// Arquivo done (sem heading # — fallback para nome sem extensão)
	if err := os.WriteFile(
		filepath.Join(base, "done", "ROADMAP-login.md"),
		[]byte("Sem heading de título\n"),
		0644,
	); err != nil {
		t.Fatalf("WriteFile done: %v", err)
	}

	cfg := config.ProjectConfig{
		RoadmapDir:         base,
		RoadmapNamespacing: config.NamespacingFlat,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/board", nil)
	rec := httptest.NewRecorder()

	boardHandler(rec, req, cfg)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obteve %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp boardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("falha ao decodificar JSON: %v", err)
	}

	// Verificar colunas presentes
	for _, state := range states {
		if _, ok := resp.Columns[state]; !ok {
			t.Errorf("coluna %q ausente no JSON", state)
		}
	}

	// wip deve ter 1 item
	if len(resp.Columns["wip"]) != 1 {
		t.Errorf("esperado 1 item em wip, obteve %d", len(resp.Columns["wip"]))
	} else {
		item := resp.Columns["wip"][0]
		if item.File != "ROADMAP-auth.md" {
			t.Errorf("wip[0].File: esperado %q, obteve %q", "ROADMAP-auth.md", item.File)
		}
		if item.Title != "Roadmap de Autenticação" {
			t.Errorf("wip[0].Title: esperado %q, obteve %q", "Roadmap de Autenticação", item.Title)
		}
		if item.State != "wip" {
			t.Errorf("wip[0].State: esperado %q, obteve %q", "wip", item.State)
		}
	}

	// backlog deve ter 1 item
	if len(resp.Columns["backlog"]) != 1 {
		t.Errorf("esperado 1 item em backlog, obteve %d", len(resp.Columns["backlog"]))
	} else {
		item := resp.Columns["backlog"][0]
		if item.Title != "Roadmap de Busca" {
			t.Errorf("backlog[0].Title: esperado %q, obteve %q", "Roadmap de Busca", item.Title)
		}
	}

	// done deve ter 1 item com título fallback (nome do arquivo sem extensão)
	if len(resp.Columns["done"]) != 1 {
		t.Errorf("esperado 1 item em done, obteve %d", len(resp.Columns["done"]))
	} else {
		item := resp.Columns["done"][0]
		if item.Title != "ROADMAP-login" {
			t.Errorf("done[0].Title: esperado %q (fallback), obteve %q", "ROADMAP-login", item.Title)
		}
	}

	// analyzing, blocked e abandoned devem estar vazios
	if len(resp.Columns["analyzing"]) != 0 {
		t.Errorf("esperado 0 itens em analyzing, obteve %d", len(resp.Columns["analyzing"]))
	}
	if len(resp.Columns["blocked"]) != 0 {
		t.Errorf("esperado 0 itens em blocked, obteve %d", len(resp.Columns["blocked"]))
	}
	if len(resp.Columns["abandoned"]) != 0 {
		t.Errorf("esperado 0 itens em abandoned, obteve %d", len(resp.Columns["abandoned"]))
	}
}

// TestParseMLProgress — verifica contagem de MLs, active_ml e next_ml.
func TestParseMLProgress(t *testing.T) {
	content := `# Roadmap Teste

## Wave 1 — Backend

### ML-1A — Criar endpoint
**Status:** ✅ Concluído

### ML-1B — Adicionar testes
**Status:** 🔄 Em andamento

### ML-1C — Deploy
**Status:** ⬜ Pendente

## Wave 2 — Frontend

### ML-2A — Tela inicial
**Status:** ⬜ Pendente
`
	f, err := os.CreateTemp("", "roadmap-*.md")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	total, done, activeML, nextML := parseMLProgress(f.Name())

	if total != 4 {
		t.Errorf("total: esperado 4, obteve %d", total)
	}
	if done != 1 {
		t.Errorf("done: esperado 1, obteve %d", done)
	}
	if activeML != "Wave 1 — Backend · ML-1B — Adicionar testes" {
		t.Errorf("activeML: obteve %q", activeML)
	}
	if nextML != "Wave 1 — Backend · ML-1C — Deploy" {
		t.Errorf("nextML: obteve %q", nextML)
	}
}

// TestParseMLProgress_AllPending — roadmap em wip com todos MLs pendentes deve ter next_ml preenchido.
func TestParseMLProgress_AllPending(t *testing.T) {
	content := `# Roadmap Novo

## Wave 1 — Implementação

### ML-1A — Passo inicial
**Status:** ⬜ Pendente

### ML-1B — Passo seguinte
**Status:** ⬜ Pendente
`
	f, err := os.CreateTemp("", "roadmap-pending-*.md")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	total, done, activeML, nextML := parseMLProgress(f.Name())

	if total != 2 {
		t.Errorf("total: esperado 2, obteve %d", total)
	}
	if done != 0 {
		t.Errorf("done: esperado 0, obteve %d", done)
	}
	if activeML != "" {
		t.Errorf("activeML: esperado vazio, obteve %q", activeML)
	}
	if nextML != "Wave 1 — Implementação · ML-1A — Passo inicial" {
		t.Errorf("nextML: obteve %q", nextML)
	}
}

// TestBoardHandler_EmptyBoard — dir existe mas está vazio: JSON com colunas vazias, sem erro.
func TestBoardHandler_EmptyBoard(t *testing.T) {
	base := t.TempDir()

	// Criar as pastas de estado mas sem nenhum arquivo .md
	states := []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}
	for _, s := range states {
		if err := os.MkdirAll(filepath.Join(base, s), 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", s, err)
		}
	}

	cfg := config.ProjectConfig{
		RoadmapDir:         base,
		RoadmapNamespacing: config.NamespacingFlat,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/board", nil)
	rec := httptest.NewRecorder()

	boardHandler(rec, req, cfg)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obteve %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp boardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("falha ao decodificar JSON: %v", err)
	}

	// Todas as colunas devem existir e estar vazias
	for _, state := range states {
		col, ok := resp.Columns[state]
		if !ok {
			t.Errorf("coluna %q ausente no JSON", state)
			continue
		}
		if len(col) != 0 {
			t.Errorf("coluna %q deve estar vazia, obteve %d itens", state, len(col))
		}
	}
}
