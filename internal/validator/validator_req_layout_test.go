package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// Contrato do resolvedor de REQ (ADR-2026-09-03): leitura é UNIÃO dos 4 layouts, escrita é ÚNICA, e
// o diretório de escrita está contido na união por construção (D2/D3/D4).

func writeREQ(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body := "---\nstatus: Open\ndate: 2026-09-03\n---\n\n# REQ: fixture\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func basenames(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, filepath.Base(p))
	}
	return out
}

// TestResolveREQFilesUniaoDos4Layouts — em by_agent, os 4 layouts são lidos JUNTOS, nunca em
// exclusão mútua. Era o defeito da REQ-2026-08-30: o leitor escolhia só req_dir/<agente>/<estado>/.
func TestResolveREQFilesUniaoDos4Layouts(t *testing.T) {
	dir := t.TempDir()
	reqDir := filepath.Join(dir, "docs/req")
	writeREQ(t, filepath.Join(reqDir, "REQ-flat.md"))                    // (1) flat legado
	writeREQ(t, filepath.Join(reqDir, "backlog", "REQ-estado.md"))       // (2) por-estado legado
	writeREQ(t, filepath.Join(reqDir, "claude", "REQ-canonico.md"))      // (3) CANÔNICO
	writeREQ(t, filepath.Join(reqDir, "claude", "wip", "REQ-legado.md")) // (4) legado

	cfg := config.ProjectConfig{
		REQDir:             reqDir,
		RoadmapNamespacing: config.NamespacingByAgent,
		Agents:             []string{"claude"},
	}

	got := basenames(ResolveREQFiles(cfg))
	want := map[string]bool{
		"REQ-flat.md": false, "REQ-estado.md": false,
		"REQ-canonico.md": false, "REQ-legado.md": false,
	}
	for _, name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("arquivo inesperado na união: %q", name)
			continue
		}
		want[name] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("layout não coberto pela união: %q ausente em %v", name, got)
		}
	}
	if len(got) != 4 {
		t.Fatalf("esperado 4 arquivos (um por layout), obteve %d: %v", len(got), got)
	}
}

// TestResolveREQFilesDeduplicaEstadoEAgente — resolveAgentNamespaces devolve agents: ∪ disco, então
// req_dir/backlog/ entra na lista de agentes e os casos <estado>/*.md e <agente>/*.md colidem. Sem
// deduplicação, a mesma REQ apareceria duas vezes e cada violação sairia em dobro.
func TestResolveREQFilesDeduplicaEstadoEAgente(t *testing.T) {
	dir := t.TempDir()
	reqDir := filepath.Join(dir, "docs/req")
	writeREQ(t, filepath.Join(reqDir, "backlog", "REQ-uma-so.md"))

	cfg := config.ProjectConfig{
		REQDir:             reqDir,
		RoadmapNamespacing: config.NamespacingByAgent,
		Agents:             []string{"claude"},
	}

	got := ResolveREQFiles(cfg)
	if len(got) != 1 {
		t.Fatalf("esperado 1 arquivo (deduplicado), obteve %d: %v", len(got), got)
	}
}

// TestREQWriteDirEstaContidoNaUniao — o invariante D4: escritor e leitor consomem o mesmo ponto de
// decisão de caminho. Um arquivo criado onde REQWriteDir manda TEM de ser encontrado por
// ResolveREQFiles, nos dois modos de namespacing. É a asserção que teria pego o defeito original.
func TestREQWriteDirEstaContidoNaUniao(t *testing.T) {
	for _, tc := range []struct {
		name        string
		namespacing string
		wantSuffix  string
	}{
		{"flat", "", "docs/req"},
		{"by_agent", config.NamespacingByAgent, filepath.Join("docs/req", "claude")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := config.ProjectConfig{
				REQDir:             filepath.Join(dir, "docs/req"),
				RoadmapNamespacing: tc.namespacing,
				Agents:             []string{"claude"},
			}
			writeDir := REQWriteDir(cfg)
			if want := filepath.Join(dir, tc.wantSuffix); writeDir != want {
				t.Fatalf("REQWriteDir = %q, esperado %q", writeDir, want)
			}
			writeREQ(t, filepath.Join(writeDir, "REQ-nova.md"))

			found := false
			for _, p := range ResolveREQFiles(cfg) {
				if filepath.Base(p) == "REQ-nova.md" {
					found = true
				}
			}
			if !found {
				t.Fatalf("REQ criada em REQWriteDir (%s) não foi encontrada por ResolveREQFiles", writeDir)
			}
		})
	}
}

// TestREQWriteDirDefaultEstaContidoNaUniao — caminho de fallback: projeto by_agent SEM agents:
// declarada (o estado de todo repositório adotante antes de alguém declarar a lista). A escrita vai
// para req_dir/default/ e o namespace precisa voltar pelo DISCO, não por config — código diferente
// do caso com agents: declarada, e por isso testado separadamente.
func TestREQWriteDirDefaultEstaContidoNaUniao(t *testing.T) {
	dir := t.TempDir()
	cfg := config.ProjectConfig{
		REQDir:             filepath.Join(dir, "docs/req"),
		RoadmapNamespacing: config.NamespacingByAgent,
	}
	writeDir := REQWriteDir(cfg)
	writeREQ(t, filepath.Join(writeDir, "REQ-nova.md"))

	got := basenames(ResolveREQFiles(cfg))
	if len(got) != 1 || got[0] != "REQ-nova.md" {
		t.Fatalf("REQ criada em %s não foi encontrada pelo resolvedor: %v", writeDir, got)
	}
}

// TestREQWriteDirSemAgentsDeclaradosUsaDefault — mesma convenção de agentStateDir em
// internal/generators/roadmap.go: sem agents:, o namespace de escrita é "default".
func TestREQWriteDirSemAgentsDeclaradosUsaDefault(t *testing.T) {
	cfg := config.ProjectConfig{REQDir: "docs/req", RoadmapNamespacing: config.NamespacingByAgent}
	if got, want := REQWriteDir(cfg), filepath.Join("docs/req", "default"); got != want {
		t.Fatalf("REQWriteDir = %q, esperado %q", got, want)
	}
}
