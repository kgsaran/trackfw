package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestChainHandler_EdgeToleratesDirtyBackslashReference reproduz o PoC B do parecer de
// ameaça (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md): uma REQ cujo
// frontmatter roadmap: foi gravado com separador nativo do Windows ("\") deve, mesmo assim,
// desenhar a aresta REQ→Roadmap no grafo — sem a normalização, node.ID (via filepath.WalkDir,
// sempre "/" nesta máquina) e edge.To (valor cru do frontmatter, "\") nunca batem por
// igualdade de string e a ligação some silenciosamente.
func TestChainHandler_EdgeToleratesDirtyBackslashReference(t *testing.T) {
	base := t.TempDir()
	reqDir := filepath.Join(base, "req")
	roadmapDir := filepath.Join(base, "roadmaps")
	wipDir := filepath.Join(roadmapDir, "wip")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("MkdirAll req: %v", err)
	}
	if err := os.MkdirAll(wipDir, 0755); err != nil {
		t.Fatalf("MkdirAll wip: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wipDir, "ROADMAP-dirty.md"), []byte("# Roadmap dirty\n"), 0644); err != nil {
		t.Fatalf("WriteFile roadmap: %v", err)
	}
	// Simula o valor que um `roadmap move` rodado no Windows, antes do fix de escrita, teria
	// gravado no frontmatter — separador nativo "\", montado à mão a partir do caminho real
	// (ML-0A: não dá para produzir isto rodando o comando nesta máquina, filepath.Join sempre
	// usa "/" aqui).
	cleanRefForDirty := filepath.ToSlash(filepath.Join(wipDir, "ROADMAP-dirty.md"))
	dirtyRef := strings.ReplaceAll(cleanRefForDirty, "/", `\`)
	reqContent := "---\nstatus: Open\nroadmap: \"" + dirtyRef + "\"\n---\n# REQ dirty\n"
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-dirty.md"), []byte(reqContent), 0644); err != nil {
		t.Fatalf("WriteFile req: %v", err)
	}

	cfg := config.ProjectConfig{
		ADRDirs:            []string{filepath.Join(base, "adr")},
		REQDir:             reqDir,
		RoadmapDir:         roadmapDir,
		RoadmapNamespacing: config.NamespacingFlat,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/chain", nil)
	rec := httptest.NewRecorder()
	chainHandler(rec, req, cfg)

	var resp chainResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var roadmapNodeID string
	for _, n := range resp.Nodes {
		if n.Type == "roadmap" {
			roadmapNodeID = n.ID
		}
	}
	if roadmapNodeID == "" {
		t.Fatalf("nó do roadmap não encontrado; nodes: %+v", resp.Nodes)
	}

	found := false
	for _, e := range resp.Edges {
		if e.To == roadmapNodeID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("aresta REQ→Roadmap não encontrada — a referência suja com \"\\\\\" não resolveu contra o node.ID %q; edges: %+v", roadmapNodeID, resp.Edges)
	}
}

// TestChainHandler_EdgeStillResolvesWithPortableReference — controle: referência já gravada
// com "/" (comportamento normal pós-fix) continua resolvendo.
func TestChainHandler_EdgeStillResolvesWithPortableReference(t *testing.T) {
	base := t.TempDir()
	reqDir := filepath.Join(base, "req")
	roadmapDir := filepath.Join(base, "roadmaps")
	wipDir := filepath.Join(roadmapDir, "wip")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("MkdirAll req: %v", err)
	}
	if err := os.MkdirAll(wipDir, 0755); err != nil {
		t.Fatalf("MkdirAll wip: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wipDir, "ROADMAP-clean.md"), []byte("# Roadmap clean\n"), 0644); err != nil {
		t.Fatalf("WriteFile roadmap: %v", err)
	}
	cleanRef := filepath.ToSlash(filepath.Join(wipDir, "ROADMAP-clean.md"))
	reqContent := "---\nstatus: Open\nroadmap: \"" + cleanRef + "\"\n---\n# REQ clean\n"
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-clean.md"), []byte(reqContent), 0644); err != nil {
		t.Fatalf("WriteFile req: %v", err)
	}

	cfg := config.ProjectConfig{
		ADRDirs:            []string{filepath.Join(base, "adr")},
		REQDir:             reqDir,
		RoadmapDir:         roadmapDir,
		RoadmapNamespacing: config.NamespacingFlat,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/chain", nil)
	rec := httptest.NewRecorder()
	chainHandler(rec, req, cfg)

	var resp chainResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	found := false
	for _, e := range resp.Edges {
		if e.To == cleanRef {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("aresta com referência já portável deveria continuar resolvendo; edges: %+v", resp.Edges)
	}
}

// TestNormalizeRefSeparator_ControlDoesNotTouchUnrelatedValue — limite duro: a função só
// converte "\" para "/"; não deve alterar nada além disso (não trunca, não mexe em outros
// caracteres).
func TestNormalizeRefSeparator_ControlDoesNotTouchUnrelatedValue(t *testing.T) {
	in := "docs/roadmaps/wip/ROADMAP-x.md"
	if got := normalizeRefSeparator(in); got != in {
		t.Errorf("valor já portável não deveria mudar; queria %q, obteve %q", in, got)
	}
	dirty := `docs\roadmaps\wip\ROADMAP-x.md`
	want := "docs/roadmaps/wip/ROADMAP-x.md"
	if got := normalizeRefSeparator(dirty); got != want {
		t.Errorf("normalização incorreta; queria %q, obteve %q", want, got)
	}
}
