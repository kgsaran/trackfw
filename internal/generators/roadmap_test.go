package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// testStateDirs retorna os diretórios de estado padrão para uso em testes.
var testStateDirs = []string{
	"docs/roadmaps/backlog",
	"docs/roadmaps/analyzing",
	"docs/roadmaps/wip",
	"docs/roadmaps/blocked",
	"docs/roadmaps/done",
	"docs/roadmaps/abandoned",
}

// chdir muda para dir e restaura ao fim do teste
func chdirRoadmap(t *testing.T, dir string) {
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

// TestNewRoadmap_CreatesFile — arquivo criado em docs/roadmaps/backlog/ com conteúdo correto
func TestNewRoadmap_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	if err := NewRoadmap("My Feature"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	if !strings.Contains(body, "My Feature") {
		t.Errorf("arquivo deveria conter 'My Feature', obteve: %q", body)
	}
	if !strings.Contains(body, "REQ:") {
		t.Errorf("arquivo deveria conter 'REQ:', obteve: %q", body)
	}
}

// mkRoadmapDirs cria a estrutura padrão de diretórios de roadmap no diretório corrente.
func mkRoadmapDirs(t *testing.T) {
	t.Helper()
	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}
}

// TestMoveRoadmap_Valid — cria roadmap em backlog, move para wip e verifica frontmatter sincronizado.
func TestMoveRoadmap_Valid(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	if err := NewRoadmap("Move Test"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	if err := MoveRoadmap("move-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap() erro: %v", err)
	}

	// Deve existir em wip
	wipMatches, err := filepath.Glob("docs/roadmaps/wip/*.md")
	if err != nil {
		t.Fatalf("Glob wip: %v", err)
	}
	if len(wipMatches) != 1 {
		t.Errorf("esperado 1 arquivo em wip, obteve %d: %v", len(wipMatches), wipMatches)
	}

	// Não deve existir mais em backlog
	backlogMatches, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(backlogMatches) != 0 {
		t.Errorf("esperado 0 arquivos em backlog após move, obteve %d: %v", len(backlogMatches), backlogMatches)
	}

	// Frontmatter deve ter status: wip (minúsculo, igual ao nome do estado)
	content, err := os.ReadFile(wipMatches[0])
	if err != nil {
		t.Fatalf("ReadFile após move: %v", err)
	}
	if !strings.Contains(string(content), "status: wip") {
		t.Errorf("frontmatter deveria conter 'status: wip', obteve:\n%s", string(content))
	}
	// Cabeçalho também deve ter | Status: wip
	if !strings.Contains(string(content), "| Status: wip") {
		t.Errorf("cabeçalho deveria conter '| Status: wip', obteve:\n%s", string(content))
	}
}

// TestMoveRoadmap_FrontmatterSync_ValidateAfterMove — prova P4: nenhum warning folder_status após move.
// Controle positivo garante que o validador está de fato inspecionando os arquivos.
func TestMoveRoadmap_FrontmatterSync_ValidateAfterMove(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	// Criar e mover um roadmap real: backlog → wip → done
	if err := NewRoadmap("Validate Test"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}
	if err := MoveRoadmap("validate-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap wip: %v", err)
	}
	if err := MoveRoadmap("validate-test", "done"); err != nil {
		t.Fatalf("MoveRoadmap done: %v", err)
	}

	// Controle positivo: escrever manualmente um arquivo em wip com status: backlog → DEVE gerar warning
	controlContent := "---\nstatus: backlog\ndate: 2026-01-01\n---\n# Roadmap: Control\n\n> Created: 2026-01-01 | Status: backlog\n"
	controlPath := "docs/roadmaps/wip/ROADMAP-control.md"
	if err := os.WriteFile(controlPath, []byte(controlContent), 0644); err != nil {
		t.Fatalf("WriteFile controle: %v", err)
	}

	_, warnings, err := validator.ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered(): %v", err)
	}

	// O arquivo movido NÃO deve gerar warning de folder_status
	for _, w := range warnings {
		if strings.Contains(w, "folder_status") && strings.Contains(w, "validate-test") {
			t.Errorf("roadmap movido gerou warning folder_status inesperado: %s", w)
		}
	}

	// O controle positivo DEVE gerar warning de folder_status
	hasControlWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "ROADMAP-control.md") && strings.Contains(w, "folder") {
			hasControlWarning = true
			break
		}
	}
	if !hasControlWarning {
		t.Errorf("controle positivo não gerou warning folder_status — o validador pode não estar inspecionando os arquivos; warnings: %v", warnings)
	}
}

// TestMoveRoadmap_BodyStatusIntact — status: no corpo e | Status: em bloco de código NÃO são tocados.
// Reprova a implementação Python original (re.sub não escopado).
func TestMoveRoadmap_BodyStatusIntact(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	// Roadmap cujo corpo contém 'status: backlog' (em tabela) e '| Status: backlog' (em seção)
	bodyWithStatusLines := "---\nstatus: backlog\ndate: 2026-01-01\n---\n" +
		"# Roadmap: Body Status Test\n\n" +
		"> Created: 2026-01-01 | Status: backlog\n\n" +
		"## Context\n\n" +
		"A tabela abaixo documenta os estados:\n\n" +
		"| Estado | status: backlog |\n" +
		"|--------|----------------|\n" +
		"| Inicial | backlog |\n\n" +
		"Código de exemplo com header:\n\n" +
		"```\n" +
		"> Created: 2026-01-01 | Status: backlog\n" +
		"```\n"

	roadmapPath := "docs/roadmaps/backlog/ROADMAP-body-status-test.md"
	if err := os.WriteFile(roadmapPath, []byte(bodyWithStatusLines), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := MoveRoadmap("body-status-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap(): %v", err)
	}

	wipMatches, _ := filepath.Glob("docs/roadmaps/wip/*.md")
	if len(wipMatches) != 1 {
		t.Fatalf("esperado 1 arquivo em wip, obteve %d", len(wipMatches))
	}
	content, err := os.ReadFile(wipMatches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	// Frontmatter deve ter status: wip
	if !strings.Contains(body, "status: wip") {
		t.Errorf("frontmatter deveria conter 'status: wip'")
	}
	// Cabeçalho deve ter | Status: wip
	if !strings.Contains(body, "| Status: wip") {
		t.Errorf("cabeçalho deveria conter '| Status: wip'")
	}
	// A linha do corpo '| Estado | status: backlog |' NÃO deve ter sido tocada
	if !strings.Contains(body, "| Estado | status: backlog |") {
		t.Errorf("linha do corpo 'status: backlog' foi modificada incorretamente; corpo:\n%s", body)
	}
	// O '| Status: backlog' dentro do bloco de código (após ## ) NÃO deve ter sido tocado
	if !strings.Contains(body, "```\n> Created: 2026-01-01 | Status: backlog\n```") {
		t.Errorf("'| Status: backlog' no bloco de código foi modificado incorretamente; corpo:\n%s", body)
	}
}

// TestMoveRoadmap_NoFrontmatter — arquivo sem frontmatter é movido sem corrupção.
func TestMoveRoadmap_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	mkRoadmapDirs(t)

	// Arquivo sem frontmatter reconhecível
	plainContent := "# Roadmap sem frontmatter\n\nConteúdo simples sem bloco ---.\n"
	roadmapPath := "docs/roadmaps/backlog/ROADMAP-no-frontmatter.md"
	if err := os.WriteFile(roadmapPath, []byte(plainContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := MoveRoadmap("no-frontmatter", "wip"); err != nil {
		t.Fatalf("MoveRoadmap() erro: %v", err)
	}

	wipMatches, _ := filepath.Glob("docs/roadmaps/wip/*.md")
	if len(wipMatches) != 1 {
		t.Fatalf("esperado 1 arquivo em wip, obteve %d", len(wipMatches))
	}
	content, err := os.ReadFile(wipMatches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Conteúdo deve ser idêntico ao original (sem chave inventada, sem corrupção)
	if string(content) != plainContent {
		t.Errorf("conteúdo do arquivo sem frontmatter foi alterado;\noriginal: %q\nobteve: %q", plainContent, string(content))
	}
}

func assertMoveRoadmapAnalyzingContract(t *testing.T, byAgent bool) error {
	t.Helper()
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if byAgent {
		yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n"
		if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
			return err
		}
		if err := os.MkdirAll("docs/roadmaps/zeus/backlog", 0755); err != nil {
			return err
		}
		content := "---\nstatus: backlog\ndate: 2026-07-27\nreq: \"docs/req/REQ-demo.md\"\nsquad: \"\"\n---\n\n# Roadmap: Analyze By Agent\n\n> Created: 2026-07-27 | Status: backlog\n"
		if err := os.WriteFile("docs/roadmaps/zeus/backlog/ROADMAP-analyze-by-agent.md", []byte(content), 0644); err != nil {
			return err
		}
		if err := MoveRoadmap("analyze-by-agent", "analyzing"); err != nil {
			return err
		}
		dst := "docs/roadmaps/zeus/analyzing/ROADMAP-analyze-by-agent.md"
		raw, err := os.ReadFile(dst)
		if err != nil {
			return err
		}
		body := string(raw)
		for _, want := range []string{"status: analyzing", "| Status: analyzing"} {
			if !strings.Contains(body, want) {
				return &testExpectationError{message: "roadmap by_agent não sincronizou " + want}
			}
		}
		log, err := os.ReadFile("docs/roadmaps/.trackfw-log")
		if err != nil {
			return err
		}
		if !strings.Contains(string(log), "zeus/ROADMAP-analyze-by-agent.md") || !strings.Contains(string(log), "backlog → analyzing") {
			return &testExpectationError{message: "log by_agent não registrou backlog → analyzing preservando agente"}
		}
		found, err := findRoadmap("analyze-by-agent")
		if err != nil {
			return err
		}
		if found != dst {
			return &testExpectationError{message: "findRoadmap by_agent não encontrou o arquivo em analyzing"}
		}
		if err := ShowRoadmap("analyze-by-agent"); err != nil {
			return err
		}
		if err := ListRoadmaps(); err != nil {
			return err
		}
		return nil
	}

	for _, d := range []string{
		"docs/roadmaps/backlog",
		"docs/roadmaps/analyzing",
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	content := "---\nstatus: backlog\ndate: 2026-07-27\nreq: \"docs/req/REQ-demo.md\"\nsquad: \"\"\n---\n\n# Roadmap: Analyze Flat\n\n> Created: 2026-07-27 | Status: backlog\n"
	if err := os.WriteFile("docs/roadmaps/backlog/ROADMAP-analyze-flat.md", []byte(content), 0644); err != nil {
		return err
	}
	if err := MoveRoadmap("analyze-flat", "analyzing"); err != nil {
		return err
	}
	raw, err := os.ReadFile("docs/roadmaps/analyzing/ROADMAP-analyze-flat.md")
	if err != nil {
		return err
	}
	body := string(raw)
	for _, want := range []string{"status: analyzing", "| Status: analyzing"} {
		if !strings.Contains(body, want) {
			return &testExpectationError{message: "roadmap flat não sincronizou " + want}
		}
	}
	log, err := os.ReadFile("docs/roadmaps/.trackfw-log")
	if err != nil {
		return err
	}
	if !strings.Contains(string(log), "ROADMAP-analyze-flat.md") || !strings.Contains(string(log), "backlog → analyzing") {
		return &testExpectationError{message: "log flat não registrou backlog → analyzing"}
	}
	found, err := findRoadmap("analyze-flat")
	if err != nil {
		return err
	}
	if found != "docs/roadmaps/analyzing/ROADMAP-analyze-flat.md" {
		return &testExpectationError{message: "findRoadmap flat não encontrou o arquivo em analyzing"}
	}
	if err := ShowRoadmap("analyze-flat"); err != nil {
		return err
	}
	if err := ListRoadmaps(); err != nil {
		return err
	}
	return nil
}

func TestMoveRoadmap_AnalyzingFlat(t *testing.T) {
	if err := assertMoveRoadmapAnalyzingContract(t, false); err != nil {
		t.Fatalf("contrato analyzing flat falhou: %v", err)
	}
}

func TestMoveRoadmap_AnalyzingByAgent(t *testing.T) {
	if err := assertMoveRoadmapAnalyzingContract(t, true); err != nil {
		t.Fatalf("contrato analyzing by_agent falhou: %v", err)
	}
}

// TestMoveRoadmap_InvalidState — estado inválido → erro descritivo
func TestMoveRoadmap_InvalidState(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := MoveRoadmap("qualquer-coisa", "inexistente")
	if err == nil {
		t.Fatal("esperado erro para estado inválido, obteve nil")
	}
	if !strings.Contains(err.Error(), "invalid state") {
		t.Errorf("erro deveria mencionar 'invalid state', obteve: %v", err)
	}
}

// TestMoveRoadmap_NotFound — roadmap inexistente → erro descritivo
func TestMoveRoadmap_NotFound(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	// Criar todos os diretórios válidos (vazios)
	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	err := MoveRoadmap("nao-existe", "wip")
	if err == nil {
		t.Fatal("esperado erro para roadmap não encontrado, obteve nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("erro deveria mencionar 'not found', obteve: %v", err)
	}
}

// TestNewRoadmapFromContent_CreatesFile — verifica que arquivo é criado quando Body é preenchido
func TestNewRoadmapFromContent_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "AI Feature",
		REQPath: "docs/req/REQ-2026-01-01-ai-feature.md",
		Body:    "# Roadmap gerado por IA\nConteúdo customizado aqui.",
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "Conteúdo customizado aqui") {
		t.Errorf("arquivo deveria conter o body fornecido, obteve: %q", body)
	}
}

// TestNewRoadmapFromContent_EmptyBody — verifica que template padrão é gerado quando Body == ""
func TestNewRoadmapFromContent_EmptyBody(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "Template Feature",
		REQPath: "docs/req/REQ-2026-01-01-template-feature.md",
		Body:    "",
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "Template Feature") {
		t.Errorf("template deveria conter o título, obteve: %q", body)
	}
	if !strings.Contains(body, "REQ:") {
		t.Errorf("template deveria conter 'REQ:', obteve: %q", body)
	}
	if !strings.Contains(body, "ML-1A") {
		t.Errorf("template deveria conter 'ML-1A', obteve: %q", body)
	}
}

// TestListRoadmaps_GroupedByState — verifica agrupamento correto por estado
func TestListRoadmaps_GroupedByState(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}

	// Criar um arquivo em backlog e um em done
	if err := os.WriteFile("docs/roadmaps/backlog/ROADMAP-2026-01-01-feature-a.md", []byte("# A"), 0644); err != nil {
		t.Fatalf("WriteFile backlog: %v", err)
	}
	if err := os.WriteFile("docs/roadmaps/done/ROADMAP-2026-01-01-feature-b.md", []byte("# B"), 0644); err != nil {
		t.Fatalf("WriteFile done: %v", err)
	}

	// ListRoadmaps não deve retornar erro
	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro: %v", err)
	}
}

// TestListRoadmaps_Empty — nenhum roadmap → mensagem amigável, sem erro
func TestListRoadmaps_Empty(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	for _, d := range testStateDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro esperando nil: %v", err)
	}
}

// TestListRoadmaps_ByAgent — modo by_agent lista roadmaps agrupados por agente/estado
func TestListRoadmaps_ByAgent(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Criar trackfw.yaml com by_agent + agentes zeus e apolo
	yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n- apolo\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	// Criar estrutura de diretórios e arquivos
	if err := os.MkdirAll("docs/roadmaps/zeus/wip", 0755); err != nil {
		t.Fatalf("mkdir zeus/wip: %v", err)
	}
	if err := os.MkdirAll("docs/roadmaps/apolo/backlog", 0755); err != nil {
		t.Fatalf("mkdir apolo/backlog: %v", err)
	}
	if err := os.WriteFile("docs/roadmaps/zeus/wip/ROADMAP-2026-01-01-zeus-test.md", []byte("# Zeus"), 0644); err != nil {
		t.Fatalf("escrever arquivo zeus: %v", err)
	}
	if err := os.WriteFile("docs/roadmaps/apolo/backlog/ROADMAP-2026-01-01-apolo-test.md", []byte("# Apolo"), 0644); err != nil {
		t.Fatalf("escrever arquivo apolo: %v", err)
	}

	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro: %v", err)
	}
}

// TestMoveRoadmap_ByAgent — move roadmap dentro do namespace do agente em modo by_agent
func TestMoveRoadmap_ByAgent(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Criar trackfw.yaml com by_agent
	yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	// Criar roadmap em zeus/backlog
	if err := os.MkdirAll("docs/roadmaps/zeus/backlog", 0755); err != nil {
		t.Fatalf("mkdir zeus/backlog: %v", err)
	}
	const roadmapFile = "docs/roadmaps/zeus/backlog/ROADMAP-test.md"
	if err := os.WriteFile(roadmapFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("escrever arquivo: %v", err)
	}

	if err := MoveRoadmap("ROADMAP-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap() erro: %v", err)
	}

	// Deve existir em zeus/wip
	if _, err := os.Stat("docs/roadmaps/zeus/wip/ROADMAP-test.md"); err != nil {
		t.Errorf("arquivo não encontrado em zeus/wip: %v", err)
	}

	// Não deve existir mais em zeus/backlog
	if _, err := os.Stat(roadmapFile); err == nil {
		t.Error("arquivo ainda existe em zeus/backlog após move")
	}
}

// TestContainsIgnoreCase — função privada testada diretamente via white-box
func TestContainsIgnoreCase(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"ROADMAP-My-Feature.md", "my-feature", true},
		{"roadmap-my-feature.md", "MY-FEATURE", true},
		{"ROADMAP-Other.md", "my-feature", false},
		{"", "sub", false},
		{"something", "", true}, // strings.Contains("something", "") == true
	}

	for _, c := range cases {
		got := containsIgnoreCase(c.s, c.sub)
		if got != c.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, quer %v", c.s, c.sub, got, c.want)
		}
	}
}
