package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err := ListREQs("docs/req"); err != nil {
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

	if err := ListREQs("docs/req"); err != nil {
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
