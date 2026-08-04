package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// chdirREQ muda para dir e restaura ao fim do teste
func chdirREQ(t *testing.T, dir string) {
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

// TestNewREQ_CreatesFile — arquivo criado em docs/req/ com título e seção Motivation
func TestNewREQ_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	if err := NewREQ(REQContent{Title: "My Req"}); err != nil {
		t.Fatalf("NewREQ() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/req/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/req, obteve %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	if !strings.Contains(body, "My Req") {
		t.Errorf("arquivo deveria conter título 'My Req', obteve: %q", body)
	}
	if !strings.Contains(body, "## Motivation") {
		t.Errorf("arquivo deveria conter '## Motivation', obteve: %q", body)
	}
}

// TestNewREQ_SlugInFilename — título com espaços → filename usa hífens
func TestNewREQ_SlugInFilename(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	if err := NewREQ(REQContent{Title: "Suporte a Multi Tenant"}); err != nil {
		t.Fatalf("NewREQ() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/req/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/req, obteve %d: %v", len(matches), matches)
	}

	filename := filepath.Base(matches[0])

	if strings.Contains(filename, " ") {
		t.Errorf("filename não deveria conter espaços: %q", filename)
	}
	if !strings.Contains(filename, "suporte-a-multi-tenant") {
		t.Errorf("filename deveria conter 'suporte-a-multi-tenant', obteve: %q", filename)
	}
}

// TestNewREQ_WithContent — campos preenchidos aparecem no arquivo gerado
func TestNewREQ_WithContent(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	content := REQContent{
		Title:         "Autenticação OAuth2",
		Motivation:    "Usuários precisam de login social.",
		Criteria:      "- [ ] Login com Google\n- [ ] Login com GitHub",
		LinkedADR:     "ADR-2026-01-01-oauth2.md",
		LinkedRoadmap: "roadmap-oauth2-2026-01-01.md",
	}
	if err := NewREQ(content); err != nil {
		t.Fatalf("NewREQ() erro: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/*.md")
	body, _ := os.ReadFile(matches[0])
	s := string(body)

	if !strings.Contains(s, "Usuários precisam de login social.") {
		t.Errorf("Motivation não encontrado no arquivo")
	}
	if !strings.Contains(s, "Login com Google") {
		t.Errorf("Criteria não encontrado no arquivo")
	}
	if !strings.Contains(s, "ADR-2026-01-01-oauth2.md") {
		t.Errorf("LinkedADR não encontrado no arquivo")
	}
	if !strings.Contains(s, "roadmap-oauth2-2026-01-01.md") {
		t.Errorf("LinkedRoadmap não encontrado no arquivo")
	}
}

// TestNewREQ_EmptyFields — campos vazios geram placeholders HTML
func TestNewREQ_EmptyFields(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	if err := NewREQ(REQContent{Title: "Sem Detalhes"}); err != nil {
		t.Fatalf("NewREQ() erro: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/*.md")
	body, _ := os.ReadFile(matches[0])
	s := string(body)

	if !strings.Contains(s, "<!-- Why is this requirement needed?") {
		t.Errorf("placeholder HTML de Motivation deveria aparecer quando campo vazio")
	}
	if !strings.Contains(s, "- [ ]") {
		t.Errorf("placeholder de Criteria deveria aparecer quando campo vazio")
	}
}

// TestListREQs_Empty — sem docs/req/ → ListREQs retorna nil, sem pânico
func TestListREQs_Empty(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	// docs/req/ não existe — ListREQs deve retornar nil sem erro
	if err := ListREQs(); err != nil {
		t.Fatalf("ListREQs() com diretório ausente deveria retornar nil, obteve: %v", err)
	}
}

// TestListREQs_WithFiles — 2 REQs criados → ListREQs executa sem erro
func TestListREQs_WithFiles(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	if err := NewREQ(REQContent{Title: "Req Alpha", Motivation: "motivo A"}); err != nil {
		t.Fatalf("NewREQ alpha: %v", err)
	}
	if err := NewREQ(REQContent{Title: "Req Beta", Motivation: "motivo B"}); err != nil {
		t.Fatalf("NewREQ beta: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/*.md")
	if len(matches) != 2 {
		t.Fatalf("esperado 2 REQs, obteve %d", len(matches))
	}

	if err := ListREQs(); err != nil {
		t.Fatalf("ListREQs() com 2 arquivos deveria retornar nil, obteve: %v", err)
	}
}

// TestNewREQ_ComADRsVinculados — seção Blocked by ADRs listada quando DependsOnADRs preenchido
func TestNewREQ_ComADRsVinculados(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	content := REQContent{
		Title:         "Login Screen",
		DependsOnADRs: []string{"ADR-2026-06-12-authentication-strategy.md", "ADR-2026-06-12-ui-framework.md"},
	}
	if err := NewREQ(content); err != nil {
		t.Fatalf("NewREQ erro: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join("docs", "req", "*.md"))
	if len(matches) == 0 {
		t.Fatal("nenhum arquivo REQ criado")
	}
	data, _ := os.ReadFile(matches[0])
	body := string(data)

	if !strings.Contains(body, "## Blocked by ADRs") {
		t.Error("seção '## Blocked by ADRs' ausente")
	}
	if !strings.Contains(body, "ADR-2026-06-12-authentication-strategy.md (Draft)") {
		t.Error("ADR authentication-strategy não listado")
	}
	if !strings.Contains(body, "ADR-2026-06-12-ui-framework.md (Draft)") {
		t.Error("ADR ui-framework não listado")
	}
}

// TestNewREQ_SemADRsVinculados — seção Blocked by ADRs com placeholder quando DependsOnADRs vazio
func TestNewREQ_SemADRsVinculados(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	content := REQContent{Title: "Simple Feature"}
	if err := NewREQ(content); err != nil {
		t.Fatalf("NewREQ erro: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join("docs", "req", "*.md"))
	data, _ := os.ReadFile(matches[0])
	body := string(data)

	if !strings.Contains(body, "## Blocked by ADRs") {
		t.Error("seção '## Blocked by ADRs' deve existir mesmo sem ADRs")
	}
	if !strings.Contains(body, "<!-- none -->") {
		t.Error("placeholder '<!-- none -->' ausente")
	}
}

// TestNewREQ_ContadorNoStatus — cabeçalho exibe contador de ADRs bloqueantes
func TestNewREQ_ContadorNoStatus(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	content := REQContent{
		Title:         "Auth Feature",
		DependsOnADRs: []string{"ADR-x.md", "ADR-y.md"},
	}
	_ = NewREQ(content)

	matches, _ := filepath.Glob(filepath.Join("docs", "req", "*.md"))
	data, _ := os.ReadFile(matches[0])
	body := string(data)

	if !strings.Contains(body, "Blocked by ADRs: 2") {
		t.Errorf("esperava 'Blocked by ADRs: 2' no header, obteve:\n%s", body)
	}
}

// TestListREQs_ParsesMeta — parseREQMeta extrai título e status corretamente
func TestListREQs_ParsesMeta(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	if err := NewREQ(REQContent{Title: "Exportar CSV"}); err != nil {
		t.Fatalf("NewREQ: %v", err)
	}

	matches, _ := filepath.Glob("docs/req/*.md")
	title, status := parseREQMeta(matches[0])

	if title != "Exportar CSV" {
		t.Errorf("título esperado 'Exportar CSV', obteve: %q", title)
	}
	if status != "Open" {
		t.Errorf("status esperado 'Open', obteve: %q", status)
	}
}

func TestMoveREQ_RewritesStatusInPlace(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)

	reqPath := filepath.Join("docs", "req", "REQ-2026-07-27-fechar.md")
	if err := os.MkdirAll(filepath.Dir(reqPath), 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nstatus: Open\ndate: 2026-07-27\nroadmap: \"docs/roadmaps/done/RM.md\"\n---\n\n" +
		"# REQ: Fechar\n\n> Date: 2026-07-27 | Status: Open | Linear Issue: X\n\n" +
		"## Notes\nstatus: Open\n| Status: Open\n"
	if err := os.WriteFile(reqPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := MoveREQ("fechar", "done"); err != nil {
		t.Fatalf("MoveREQ: %v", err)
	}

	updated, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(updated)
	if !strings.Contains(body, "status: done\n") {
		t.Fatalf("frontmatter status nao atualizado:\n%s", body)
	}
	if !strings.Contains(body, "> Date: 2026-07-27 | Status: done | Linear Issue: X") {
		t.Fatalf("header Status nao atualizado:\n%s", body)
	}
	if !strings.Contains(body, "## Notes\nstatus: Open\n| Status: Open\n") {
		t.Fatalf("status no corpo deveria ser preservado:\n%s", body)
	}
	if _, err := os.Stat(reqPath); err != nil {
		t.Fatalf("REQ deveria permanecer no mesmo caminho flat: %v", err)
	}
}

// TestListREQs_ByState — REQ em docs/req/backlog/ deve aparecer em ListREQs (layout por-estado).
func TestListREQs_ByState(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.MkdirAll("docs/req/backlog", 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nstatus: backlog\ndate: 2026-08-04\n---\n\n# REQ: X\n\n> Date: 2026-08-04 | Status: backlog\n"
	if err := os.WriteFile("docs/req/backlog/REQ-x.md", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Load()
	files := listREQFiles(cfg)
	found := false
	for _, f := range files {
		if filepath.Base(f) == "REQ-x.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("listREQFiles deveria encontrar REQ-x.md em docs/req/backlog/, obteve: %v", files)
	}

	if err := ListREQs(); err != nil {
		t.Fatalf("ListREQs() erro: %v", err)
	}
}

// TestListREQs_ByAgent — REQ em docs/req/claude/wip/ (roadmap_namespacing: by_agent) deve aparecer.
func TestListREQs_ByAgent(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yamlContent := "roadmap_namespacing: by_agent\nagents:\n- claude\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("docs/req/claude/wip", 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nstatus: wip\ndate: 2026-08-04\n---\n\n# REQ: Y\n\n> Date: 2026-08-04 | Status: wip\n"
	if err := os.WriteFile("docs/req/claude/wip/REQ-y.md", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Load()
	files := listREQFiles(cfg)
	found := false
	for _, f := range files {
		if filepath.Base(f) == "REQ-y.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("listREQFiles deveria encontrar REQ-y.md em docs/req/claude/wip/, obteve: %v", files)
	}

	if err := ListREQs(); err != nil {
		t.Fatalf("ListREQs() erro: %v", err)
	}
}

// TestFindREQ_RecursesSubfolders — findREQ deve localizar uma REQ dentro de docs/req/wip/.
func TestFindREQ_RecursesSubfolders(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.MkdirAll("docs/req/wip", 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nstatus: wip\ndate: 2026-08-04\n---\n\n# REQ: Sub\n\n> Date: 2026-08-04 | Status: wip\n"
	reqPath := filepath.Join("docs", "req", "wip", "REQ-sub-pasta.md")
	if err := os.WriteFile(reqPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Load()
	found, err := findREQ("sub-pasta", cfg)
	if err != nil {
		t.Fatalf("findREQ erro: %v", err)
	}
	if found != reqPath {
		t.Fatalf("findREQ esperava %q, obteve %q", reqPath, found)
	}
}

// TestMoveREQ_PhysicallyMovesInStateLayout — REQ em docs/req/backlog/ é movida fisicamente para docs/req/done/.
func TestMoveREQ_PhysicallyMovesInStateLayout(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.MkdirAll("docs/req/backlog", 0755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join("docs", "req", "backlog", "REQ-2026-08-04-mover.md")
	content := "---\nstatus: backlog\ndate: 2026-08-04\n---\n\n# REQ: Mover\n\n> Date: 2026-08-04 | Status: backlog\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := MoveREQ("mover", "done"); err != nil {
		t.Fatalf("MoveREQ erro: %v", err)
	}

	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("REQ deveria ter sido removida do caminho original, err: %v", err)
	}

	dstPath := filepath.Join("docs", "req", "done", "REQ-2026-08-04-mover.md")
	updated, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("REQ deveria estar em docs/req/done/: %v", err)
	}
	body := string(updated)
	if !strings.Contains(body, "status: done") {
		t.Fatalf("status não atualizado no destino:\n%s", body)
	}
}

// TestMoveREQ_RejectsInvalidStateInStateLayout — REQ em subpasta de estado reconhecida
// (docs/req/wip/) rejeita status inválido no move físico, sem criar pasta arbitrária e
// sem mover o arquivo (AC5 — divergência de tratamento corrigida entre os 3 CLIs).
func TestMoveREQ_RejectsInvalidStateInStateLayout(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.MkdirAll("docs/req/wip", 0755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join("docs", "req", "wip", "REQ-2026-08-04-invalido.md")
	content := "---\nstatus: wip\ndate: 2026-08-04\n---\n\n# REQ: Invalido\n\n> Date: 2026-08-04 | Status: wip\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := MoveREQ("invalido", "status-invalido-xyz")
	if err == nil {
		t.Fatal("MoveREQ deveria retornar erro para status inválido")
	}
	if !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("erro esperado deveria mencionar 'invalid state', obteve: %v", err)
	}

	if _, statErr := os.Stat(srcPath); statErr != nil {
		t.Fatalf("REQ deveria permanecer no caminho original: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join("docs", "req", "status-invalido-xyz")); !os.IsNotExist(statErr) {
		t.Fatal("não deveria ter criado pasta arbitrária docs/req/status-invalido-xyz")
	}
}

// TestMoveREQ_RejectsInvalidStateInByAgentLayout — REQ em docs/req/claude/wip/ rejeita status
// inválido no move físico, sem criar pasta arbitrária e sem mover o arquivo (AC5).
func TestMoveREQ_RejectsInvalidStateInByAgentLayout(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yamlContent := "roadmap_namespacing: by_agent\nagents:\n- claude\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("docs/req/claude/wip", 0755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join("docs", "req", "claude", "wip", "REQ-2026-08-04-invalido-agente.md")
	content := "---\nstatus: wip\ndate: 2026-08-04\n---\n\n# REQ: InvalidoAgente\n\n> Date: 2026-08-04 | Status: wip\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := MoveREQ("invalido-agente", "status-invalido-xyz")
	if err == nil {
		t.Fatal("MoveREQ deveria retornar erro para status inválido")
	}
	if !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("erro esperado deveria mencionar 'invalid state', obteve: %v", err)
	}

	if _, statErr := os.Stat(srcPath); statErr != nil {
		t.Fatalf("REQ deveria permanecer no caminho original: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join("docs", "req", "claude", "status-invalido-xyz")); !os.IsNotExist(statErr) {
		t.Fatal("não deveria ter criado pasta arbitrária docs/req/claude/status-invalido-xyz")
	}
}

// TestMoveREQ_PhysicallyMovesInByAgentLayout — REQ em docs/req/claude/backlog/ é movida para docs/req/claude/wip/.
func TestMoveREQ_PhysicallyMovesInByAgentLayout(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yamlContent := "roadmap_namespacing: by_agent\nagents:\n- claude\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("docs/req/claude/backlog", 0755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join("docs", "req", "claude", "backlog", "REQ-2026-08-04-agente.md")
	content := "---\nstatus: backlog\ndate: 2026-08-04\n---\n\n# REQ: Agente\n\n> Date: 2026-08-04 | Status: backlog\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := MoveREQ("agente", "wip"); err != nil {
		t.Fatalf("MoveREQ erro: %v", err)
	}

	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("REQ deveria ter sido removida do caminho original, err: %v", err)
	}

	dstPath := filepath.Join("docs", "req", "claude", "wip", "REQ-2026-08-04-agente.md")
	updated, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("REQ deveria estar em docs/req/claude/wip/: %v", err)
	}
	body := string(updated)
	if !strings.Contains(body, "status: wip") {
		t.Fatalf("status não atualizado no destino:\n%s", body)
	}
}

// TestMoveREQ_LogsTransition — transição de estado registrada em docs/req/.trackfw-log.
func TestMoveREQ_LogsTransition(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.MkdirAll("docs/req/backlog", 0755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join("docs", "req", "backlog", "REQ-2026-08-04-log.md")
	content := "---\nstatus: backlog\ndate: 2026-08-04\n---\n\n# REQ: Log\n\n> Date: 2026-08-04 | Status: backlog\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := MoveREQ("log", "wip"); err != nil {
		t.Fatalf("MoveREQ erro: %v", err)
	}

	log, err := os.ReadFile(filepath.Join("docs", "req", ".trackfw-log"))
	if err != nil {
		t.Fatalf("log de transição não foi criado: %v", err)
	}
	logBody := string(log)
	if !strings.Contains(logBody, "REQ-2026-08-04-log.md") || !strings.Contains(logBody, "backlog → wip") {
		t.Fatalf("log não registrou a transição esperada, obteve:\n%s", logBody)
	}
}
