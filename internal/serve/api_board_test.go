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

// TestParseMLProgress_FenceDoesNotCountAsComplete — falsificação do defeito: um ✅ dentro de um
// bloco de código não pode ser contado como ML concluído.
//
// AC11: Este teste afirma que a implementação usa a máscara de cerca (FenceMask) ao avaliar
// status, de modo que um ✅ documentado num bloco de código não conta como conclusão de ML.
//
// "Before" evidence (current code, before the fix):
//
//	--- FAIL: TestParseMLProgress_FenceDoesNotCountAsComplete (0.00s)
//	    api_board_test.go:NNN: done: esperado 0, obteve 1 (✅ dentro de cerca contou indevidamente)
func TestParseMLProgress_FenceDoesNotCountAsComplete(t *testing.T) {
	content := "# Roadmap Fence Test\n\n" +
		"## Wave 1 — Backend\n\n" +
		"### ML-1A — Verificar gates\n" +
		"**Status:** ⬜ Pendente\n\n" +
		"Exemplo de status concluído (dentro de cerca de código — NÃO é status real):\n\n" +
		"```\n" +
		"**Status:** ✅ Concluído\n" +
		"```\n"

	f, err := os.CreateTemp("", "roadmap-fence-*.md")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	total, done, _, nextML := parseMLProgress(f.Name())

	if total != 1 {
		t.Errorf("total: esperado 1, obteve %d", total)
	}
	// O defeito atual: strings.Contains(marker, "✅") dentro da cerca → done=1 (incorreto).
	// Após o fix (máscara de cerca + primeiro token): done=0.
	if done != 0 {
		t.Errorf("done: esperado 0, obteve %d (✅ dentro de cerca contou indevidamente)", done)
	}
	if nextML == "" {
		t.Errorf("nextML: esperado preenchido (ML pendente), obteve vazio")
	}
}

// TestParseMLProgress_CheckmarkNotFirstToken — falsificação: ✅ no meio da linha (não como
// primeiro token do marcador) não conta como ML concluído.
//
// AC11: Este teste afirma que a avaliação usa primeiro token, não substring.
func TestParseMLProgress_CheckmarkNotFirstToken(t *testing.T) {
	content := "# Roadmap Token Test\n\n" +
		"## Wave 1 — Backend\n\n" +
		"### ML-1A — Status ambíguo\n" +
		"**Status:** ⬜ Pendente ✅\n"

	f, err := os.CreateTemp("", "roadmap-token-*.md")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	total, done, _, _ := parseMLProgress(f.Name())

	if total != 1 {
		t.Errorf("total: esperado 1, obteve %d", total)
	}
	// O defeito atual: strings.Contains(marker, "✅") → done=1 (incorreto).
	// Após o fix (primeiro token): done=0, pois o primeiro token é ⬜.
	if done != 0 {
		t.Errorf("done: esperado 0, obteve %d (✅ não-primeiro-token contou indevidamente)", done)
	}
}

// TestParseMLProgress_AllDone — contra-braço: roadmap com todos os MLs concluídos corretamente
// conta done == total.
//
// AC11: Este teste afirma que MLs com marcador ✅ como primeiro token continuam sendo contados
// como concluídos (não-regressão).
func TestParseMLProgress_AllDone(t *testing.T) {
	content := "# Roadmap Concluído\n\n" +
		"## Wave 1 — Backend\n\n" +
		"### ML-1A — Criar endpoint\n" +
		"**Status:** ✅ Concluído\n\n" +
		"### ML-1B — Adicionar testes\n" +
		"**Status:** ✅ Concluído\n"

	f, err := os.CreateTemp("", "roadmap-alldone-*.md")
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
	if done != 2 {
		t.Errorf("done: esperado 2, obteve %d (todos MLs concluídos devem ser contados)", done)
	}
	if activeML != "" {
		t.Errorf("activeML: esperado vazio, obteve %q", activeML)
	}
	if nextML != "" {
		t.Errorf("nextML: esperado vazio, obteve %q", nextML)
	}
}

// TestParseMLProgress_TerminatedCountsAsDone — falsificação do defeito ML-2C:
// um ML com status ABANDONADO bloqueava o progresso (total=2, done=1) porque StatusTerminated não
// incrementava done, mas o ML continuava contando em total.
//
// AC11: este teste afirma que StatusTerminated incrementa done (opção b: "resolved" = complete OR
// terminated), de forma que 1✅ + 1ABANDONADO → total=2, done=2, pct=100%.
//
// Before evidence (code before this fix):
//
//	--- FAIL: TestParseMLProgress_TerminatedCountsAsDone (0.00s)
//	    api_board_test.go:NNN: done: esperado 2, obteve 1 (ABANDONADO não incrementava done)
//	    api_board_test.go:NNN: total: esperado 2, obteve 2
//	(progresso: 1/2 = 50%, barra nunca ficava verde)
func TestParseMLProgress_TerminatedCountsAsDone(t *testing.T) {
	content := "# Roadmap Terminated Test\n\n" +
		"## Wave 1 — Backend\n\n" +
		"### ML-1A — Criar endpoint\n" +
		"**Status:** ✅ Concluído\n\n" +
		"### ML-1B — Abordagem descartada\n" +
		"**Status:** ABANDONADO — decidido em 2026-09-01\n"

	f, err := os.CreateTemp("", "roadmap-terminated-*.md")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	total, done, activeML, nextML := parseMLProgress(f.Name())

	// Before the fix: done=1 (ABANDONADO não incrementava done), total=2 → 1/2, barra presa em 50%.
	// After the fix: done=2 (terminated = resolved), total=2 → 2/2, pct=100%, barra verde.
	if total != 2 {
		t.Errorf("total: esperado 2, obteve %d", total)
	}
	if done != 2 {
		t.Errorf("done: esperado 2, obteve %d (ABANDONADO deve contar como resolvido)", done)
	}
	if activeML != "" {
		t.Errorf("activeML: esperado vazio (ML terminado não é trabalho em andamento), obteve %q", activeML)
	}
	if nextML != "" {
		t.Errorf("nextML: esperado vazio (ML terminado não é trabalho pendente), obteve %q", nextML)
	}
}

// TestParseMLProgress_BlockedIsNotResolved — contra-braço do ML-2C:
// um ML com status ❌ Bloqueado é pendência, não encerramento — o roadmap NÃO pode aparecer
// completo enquanto houver MLs bloqueados.
//
// AC11: este teste afirma que StatusPending (incluindo ❌ Bloqueado) NÃO incrementa done, de forma
// que 1✅ + 1❌ Bloqueado → total=2, done=1, pct=50% (não 100%), garantindo que o defeito não foi
// resolvido afrouxando demais a condição de "resolvido".
func TestParseMLProgress_BlockedIsNotResolved(t *testing.T) {
	content := "# Roadmap Blocked Test\n\n" +
		"## Wave 1 — Backend\n\n" +
		"### ML-1A — Criar endpoint\n" +
		"**Status:** ✅ Concluído\n\n" +
		"### ML-1B — Aguardando dependência\n" +
		"**Status:** ❌ Bloqueado — aguardando aprovação\n"

	f, err := os.CreateTemp("", "roadmap-blocked-*.md")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	total, done, _, _ := parseMLProgress(f.Name())

	// ❌ Bloqueado é pendência (StatusPending), não encerramento (StatusTerminated).
	// O roadmap NÃO pode aparecer completo enquanto houver ML bloqueado.
	if total != 2 {
		t.Errorf("total: esperado 2, obteve %d", total)
	}
	if done != 1 {
		t.Errorf("done: esperado 1, obteve %d (❌ Bloqueado não pode contar como resolvido)", done)
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
