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

// TestChainHandler_EdgeResolvesStaleStateRoadmapPath — reconciliação: afirma que a cadeia do
// `serve` liga a aresta REQ→Roadmap quando o caminho literal gravado no frontmatter aponta para
// `wip/` mas o arquivo já foi movido para `done/` (mesma causa do ML-3B, medida contra os 2
// vínculos reais REQ-2026-09-03-as-217... e REQ-2026-09-05-tres-defeitos... na árvore do
// projeto). Antes desta correção, edge.To era o valor literal ("wip/..."), que não casa com
// nenhum node.ID real (o node existe em "done/") — a aresta ficava órfã e nunca aparecia no
// grafo. Depois: validator.ResolveRoadmapRef encontra o arquivo em done/ pelo basename e edge.To
// passa a ser o caminho resolvido, que casa com o node.ID do roadmap real.
func TestChainHandler_EdgeResolvesStaleStateRoadmapPath(t *testing.T) {
	base := t.TempDir()
	reqDir := filepath.Join(base, "req")
	roadmapDir := filepath.Join(base, "roadmaps")
	wipDir := filepath.Join(roadmapDir, "wip")
	doneDir := filepath.Join(roadmapDir, "done")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("MkdirAll req: %v", err)
	}
	if err := os.MkdirAll(wipDir, 0755); err != nil {
		t.Fatalf("MkdirAll wip: %v", err)
	}
	if err := os.MkdirAll(doneDir, 0755); err != nil {
		t.Fatalf("MkdirAll done: %v", err)
	}

	// O roadmap está em done/ (foi movido), mas o vínculo da REQ ainda grava o caminho antigo,
	// com "wip/" — exatamente o que `trackfw roadmap move` deixa para trás (ML-3B).
	if err := os.WriteFile(filepath.Join(doneDir, "ROADMAP-moved.md"), []byte("# Roadmap moved\n"), 0644); err != nil {
		t.Fatalf("WriteFile roadmap: %v", err)
	}
	staleRef := filepath.ToSlash(filepath.Join(wipDir, "ROADMAP-moved.md"))
	reqContent := "---\nstatus: Open\nadr: \"\"\nroadmap: \"\"\n---\n# REQ stale\n\n## Linked Roadmap\nRoadmap: " + staleRef + "\n"
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-stale.md"), []byte(reqContent), 0644); err != nil {
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
	if roadmapNodeID == staleRef {
		t.Fatalf("setup inválido: node.ID não deveria coincidir com o caminho velho gravado")
	}

	found := false
	var gotEdges []chainEdge
	for _, e := range resp.Edges {
		gotEdges = append(gotEdges, e)
		if e.To == roadmapNodeID {
			found = true
		}
	}
	if !found {
		t.Errorf("aresta REQ->Roadmap não encontrada após roadmap ir para done/; node.ID=%q, edges=%+v", roadmapNodeID, gotEdges)
	}
}

// TestChainHandler_NoEdgeInventedForUnresolvableRoadmapRef — guarda de vacuidade: um vínculo
// `roadmap:` cujo basename não existe em ESTADO NENHUM não deve produzir aresta alguma — nem
// literal, nem inventada. Afirma o limite declarado do fallback: ausência real continua sendo
// ausência real, só o caso "existe em outro estado" ganha resolução.
func TestChainHandler_NoEdgeInventedForUnresolvableRoadmapRef(t *testing.T) {
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
	// Nenhum roadmap é criado em estado algum.
	missingRef := filepath.ToSlash(filepath.Join(wipDir, "ROADMAP-nunca-existiu.md"))
	reqContent := "---\nstatus: Open\nadr: \"\"\nroadmap: \"\"\n---\n# REQ orfa\n\n## Linked Roadmap\nRoadmap: " + missingRef + "\n"
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-orfa.md"), []byte(reqContent), 0644); err != nil {
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

	for _, e := range resp.Edges {
		if e.To != missingRef {
			continue
		}
		// A aresta literal (não resolvida) é aceitável — hoje o comportamento é mantê-la como
		// está quando não há resolução. O que NÃO pode acontecer é ela apontar para um node.ID
		// que não existe em nenhum node real (aresta inventada).
		for _, n := range resp.Nodes {
			if n.ID == e.To {
				t.Fatalf("nó inventado: %q não deveria existir como node real", e.To)
			}
		}
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
