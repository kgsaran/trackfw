package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRoadmapNewCmdExposesParityFlags afirma que roadmap new expõe os flags de paridade
// com os outros CLIs — title, req, from-req e agent (AC12: flag existe nos 3 CLIs).
func TestRoadmapNewCmdExposesParityFlags(t *testing.T) {
	cmd := newRoadmapNewCmd()

	for _, flag := range []string{"title", "req", "from-req", "agent"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("roadmap new should expose --%s", flag)
		}
	}
}

// TestReqNewCmdExposesAgentFlag afirma que req new expõe --agent (AC12: flag existe nos 3 CLIs).
func TestReqNewCmdExposesAgentFlag(t *testing.T) {
	cmd := newReqNewCmd()

	if cmd.Flags().Lookup("agent") == nil {
		t.Fatal("req new should expose --agent")
	}
}

// setupRoadmapNewDir cria um diretório temporário com trackfw.yaml e uma REQ de exemplo,
// muda o cwd para ele e retorna o caminho relativo da REQ criada.
func setupRoadmapNewDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	_ = os.MkdirAll(filepath.Join("docs", "req"), 0o755)
	_ = os.MkdirAll(filepath.Join("docs", "roadmaps"), 0o755)
	_ = os.WriteFile("trackfw.yaml", []byte("req_dir: docs/req\nroadmap_dir: docs/roadmaps\n"), 0o644)

	reqPath := filepath.Join("docs", "req", "REQ-2026-01-01-pagamentos.md")
	_ = os.WriteFile(reqPath, []byte("---\nstatus: Open\ndate: 2026-01-01\n---\n# REQ: t\n\n## Acceptance Criteria\n- [ ] AC1\n"), 0o644)
	return reqPath
}

// TestRoadmapNew_PositionalTitleUsedWithReqFlag afirma que quando um argumento posicional é
// fornecido junto com --req, o nome do roadmap usa o título posicional, não o nome da REQ.
// Afirma a conclusão do ML-1D: args[0] é atribuído a title antes do fallback que deriva do
// nome da REQ, corrigindo o bug onde o Go ignorava o título posicional.
func TestRoadmapNew_PositionalTitleUsedWithReqFlag(t *testing.T) {
	reqPath := setupRoadmapNewDir(t)

	cmd := newRoadmapNewCmd()
	cmd.SetArgs([]string{"titulo escolhido", "--req", reqPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("roadmap new: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "*.md"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("nenhum roadmap criado em backlog/: %v", err)
	}
	name := filepath.Base(matches[0])
	if !strings.Contains(name, "titulo-escolhido") {
		t.Errorf("esperava nome com 'titulo-escolhido', obteve %q", name)
	}
	if strings.Contains(name, "pagamentos") {
		t.Errorf("nome do roadmap não deve derivar do nome da REQ quando título posicional fornecido, obteve %q", name)
	}
}

// TestRoadmapNew_FallbackDerivesFromREQNameWhenNoPositionalTitle afirma que sem título
// posicional, --req continua derivando o nome do roadmap a partir do nome da REQ.
// Afirma que o fallback existente não foi quebrado pela correção do ML-1D.
func TestRoadmapNew_FallbackDerivesFromREQNameWhenNoPositionalTitle(t *testing.T) {
	reqPath := setupRoadmapNewDir(t)

	cmd := newRoadmapNewCmd()
	cmd.SetArgs([]string{"--req", reqPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("roadmap new: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "*.md"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("nenhum roadmap criado em backlog/: %v", err)
	}
	name := filepath.Base(matches[0])
	if !strings.Contains(name, "pagamentos") {
		t.Errorf("esperava nome derivado da REQ com 'pagamentos', obteve %q", name)
	}
}
