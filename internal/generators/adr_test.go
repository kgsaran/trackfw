package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestToSlug_Acentuado — título acentuado gera slug ASCII portável nos 3 CLIs.
// Cobre: á é í ó ú (agudo), ç (cedilha), ã õ (til), à (crase).
func TestToSlug_Acentuado(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Autenticação e Sessão", "autenticacao-e-sessao"},
		{"Criação de Requisição", "criacao-de-requisicao"},
		{"Configuração Avançada", "configuracao-avancada"},
		{"Título com À crase e Ã til", "titulo-com-a-crase-e-a-til"},
		{"ADR Config (v2)", "adr-config-v2"},
		{"á é í ó ú", "a-e-i-o-u"},
		{"ç ã õ à", "c-a-o-a"},
	}
	for _, tc := range cases {
		got := toSlug(tc.input)
		if got != tc.expected {
			t.Errorf("toSlug(%q) = %q, queria %q", tc.input, got, tc.expected)
		}
	}
}

// chdirADR muda para dir e restaura ao fim do teste
func chdirADR(t *testing.T, dir string) {
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

// TestNewADR_CreatesFile — arquivo criado em docs/adr/ com título e seção Context
func TestNewADR_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	if err := NewADR(ADRContent{Title: "Escolha de Banco"}, "docs/adr"); err != nil {
		t.Fatalf("NewADR() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/adr/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/adr, obteve %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	if !strings.Contains(body, "Escolha de Banco") {
		t.Errorf("arquivo deveria conter título 'Escolha de Banco', obteve: %q", body)
	}
	if !strings.Contains(body, "## Context") {
		t.Errorf("arquivo deveria conter '## Context', obteve: %q", body)
	}
}

// TestNewADR_SlugInFilename — título com espaços → filename usa hífens
func TestNewADR_SlugInFilename(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	if err := NewADR(ADRContent{Title: "Uso de Redis Cache"}, "docs/adr"); err != nil {
		t.Fatalf("NewADR() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/adr/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em docs/adr, obteve %d: %v", len(matches), matches)
	}

	filename := filepath.Base(matches[0])

	// Slug: "uso-de-redis-cache" — sem espaços
	if strings.Contains(filename, " ") {
		t.Errorf("filename não deveria conter espaços: %q", filename)
	}
	if !strings.Contains(filename, "uso-de-redis-cache") {
		t.Errorf("filename deveria conter 'uso-de-redis-cache', obteve: %q", filename)
	}
}

// TestNewADR_WithContent — campos preenchidos aparecem no arquivo; sem placeholders HTML
func TestNewADR_WithContent(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	content := ADRContent{
		Title:        "Adotar PostgreSQL",
		Context:      "Precisamos de um banco relacional robusto.",
		Decision:     "Usar PostgreSQL 15.",
		Consequences: "Custo de operação maior; maior confiabilidade.",
		Alternatives: "MySQL foi rejeitado por licença.",
	}
	if err := NewADR(content, "docs/adr"); err != nil {
		t.Fatalf("NewADR() erro: %v", err)
	}

	matches, _ := filepath.Glob("docs/adr/*.md")
	body, _ := os.ReadFile(matches[0])
	s := string(body)

	if !strings.Contains(s, "Precisamos de um banco relacional robusto.") {
		t.Errorf("Context não encontrado no arquivo")
	}
	if !strings.Contains(s, "Usar PostgreSQL 15.") {
		t.Errorf("Decision não encontrado no arquivo")
	}
	if strings.Contains(s, "<!-- What is the situation") {
		t.Errorf("placeholder HTML de Context não deveria aparecer quando campo preenchido")
	}
}

// TestNewADR_EmptyFields — campos vazios mantêm placeholders HTML
func TestNewADR_EmptyFields(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	if err := NewADR(ADRContent{Title: "Sem Detalhes"}, "docs/adr"); err != nil {
		t.Fatalf("NewADR() erro: %v", err)
	}

	matches, _ := filepath.Glob("docs/adr/*.md")
	body, _ := os.ReadFile(matches[0])
	s := string(body)

	if !strings.Contains(s, "<!-- What is the situation") {
		t.Errorf("placeholder HTML de Context deveria aparecer quando campo vazio")
	}
	if !strings.Contains(s, "<!-- What was decided?") {
		t.Errorf("placeholder HTML de Decision deveria aparecer quando campo vazio")
	}
}

// TestListADRs_Empty — diretório ausente → retorna nil, sem pânico
func TestListADRs_Empty(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	// docs/adr/ não existe — ListADRs deve retornar nil sem erro
	if err := ListADRs("docs/adr"); err != nil {
		t.Fatalf("ListADRs() com diretório ausente deveria retornar nil, obteve: %v", err)
	}
}

// TestListADRs_WithFiles — cria 2 ADRs e verifica que ListADRs não retorna erro
func TestListADRs_WithFiles(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	if err := NewADR(ADRContent{Title: "Decisao Alpha", Context: "contexto A", Decision: "decidido A"}, "docs/adr"); err != nil {
		t.Fatalf("NewADR alpha: %v", err)
	}
	if err := NewADR(ADRContent{Title: "Decisao Beta", Context: "contexto B", Decision: "decidido B"}, "docs/adr"); err != nil {
		t.Fatalf("NewADR beta: %v", err)
	}

	// Verifica que existem 2 arquivos antes de listar
	matches, _ := filepath.Glob("docs/adr/*.md")
	if len(matches) != 2 {
		t.Fatalf("esperado 2 ADRs, obteve %d", len(matches))
	}

	if err := ListADRs("docs/adr"); err != nil {
		t.Fatalf("ListADRs() com 2 arquivos deveria retornar nil, obteve: %v", err)
	}
}

func TestNewADRDraft_CriaArquivo(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	basename, err := NewADRDraft("authentication-strategy", filepath.Join("docs", "adr"))
	if err != nil {
		t.Fatalf("NewADRDraft erro: %v", err)
	}
	if basename == "" {
		t.Fatal("basename vazio")
	}
	path := filepath.Join("docs", "adr", basename)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("arquivo não criado: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("arquivo vazio")
	}
}

func TestNewADRDraft_StatusDraft(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	basename, _ := NewADRDraft("ui-framework", filepath.Join("docs", "adr"))
	content, err := os.ReadFile(filepath.Join("docs", "adr", basename))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(content), "Status: Draft") {
		t.Error("arquivo não contém 'Status: Draft'")
	}
}

func TestNewADRDraft_Idempotente(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	b1, err1 := NewADRDraft("session-management", filepath.Join("docs", "adr"))
	b2, err2 := NewADRDraft("session-management", filepath.Join("docs", "adr"))
	if err1 != nil || err2 != nil {
		t.Fatalf("erros: %v, %v", err1, err2)
	}
	if b1 != b2 {
		t.Errorf("basenames diferentes: %s vs %s", b1, b2)
	}
	// verificar que há apenas um arquivo com esse slug
	matches, _ := filepath.Glob(filepath.Join("docs", "adr", "*session-management*"))
	if len(matches) != 1 {
		t.Errorf("esperava 1 arquivo, encontrou %d", len(matches))
	}
}

func TestNewADRDraft_TituloDerivado(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	basename, _ := NewADRDraft("api-protocol", filepath.Join("docs", "adr"))
	content, _ := os.ReadFile(filepath.Join("docs", "adr", basename))
	if !strings.Contains(string(content), "# ADR: Api Protocol") {
		t.Errorf("título esperado 'Api Protocol' não encontrado em:\n%s", string(content))
	}
}

// TestGlobalADRDir_ResolvesUnderHomeTrackfwAdr — GlobalADRDir(home) segue o mesmo padrão de
// GlobalClaudeSkillPath: <home>/.trackfw/adr.
func TestGlobalADRDir_ResolvesUnderHomeTrackfwAdr(t *testing.T) {
	home := t.TempDir()
	got := GlobalADRDir(home)
	want := filepath.Join(home, ".trackfw", "adr")
	if got != want {
		t.Errorf("GlobalADRDir(%q) = %q, queria %q", home, got, want)
	}
}

// TestNewADR_ScopeGlobal_CreatesUnderFixtureHome — NewADR(content, GlobalADRDir(home)) escreve
// em $HOME/.trackfw/adr mesmo sem cwd em raiz de projeto (nenhum docs/adr criado), provando que
// o generator não chama mais config.Load() internamente.
func TestNewADR_ScopeGlobal_CreatesUnderFixtureHome(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	chdirADR(t, cwd)

	globalDir := GlobalADRDir(home)
	if err := NewADR(ADRContent{Title: "Decisao Global"}, globalDir); err != nil {
		t.Fatalf("NewADR() erro: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(globalDir, "*.md"))
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em %s, obteve %d: %v", globalDir, len(matches), matches)
	}

	// docs/adr não deve ter sido criado no cwd — escopo global não toca o projeto.
	if _, err := os.Stat(filepath.Join(cwd, "docs", "adr")); err == nil {
		t.Errorf("docs/adr não deveria existir no cwd quando adrDir aponta para escopo global")
	}
}

// TestNewADRDraft_ScopeGlobal_CreatesUnderFixtureHome — NewADRDraft(slug, GlobalADRDir(home))
// escreve em $HOME/.trackfw/adr mesmo sem cwd em raiz de projeto, provando que o generator não
// chama mais config.Load() internamente. ROADMAP-2026-08-08 ML-2A.
func TestNewADRDraft_ScopeGlobal_CreatesUnderFixtureHome(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	chdirADR(t, cwd)

	globalDir := GlobalADRDir(home)
	basename, err := NewADRDraft("estrategia-global", globalDir)
	if err != nil {
		t.Fatalf("NewADRDraft() erro: %v", err)
	}
	if basename == "" {
		t.Fatal("basename vazio")
	}

	if _, err := os.Stat(filepath.Join(globalDir, basename)); err != nil {
		t.Fatalf("arquivo não criado em %s: %v", globalDir, err)
	}

	// docs/adr não deve ter sido criado no cwd — escopo global não toca o projeto.
	if _, err := os.Stat(filepath.Join(cwd, "docs", "adr")); err == nil {
		t.Errorf("docs/adr não deveria existir no cwd quando adrDir aponta para escopo global")
	}
}

// TestListADRs_ParsesMeta — verifica que parseADRMeta extrai título e status corretamente
func TestListADRs_ParsesMeta(t *testing.T) {
	dir := t.TempDir()
	chdirADR(t, dir)

	if err := NewADR(ADRContent{Title: "Uso de Kafka"}, "docs/adr"); err != nil {
		t.Fatalf("NewADR: %v", err)
	}

	matches, _ := filepath.Glob("docs/adr/*.md")
	title, status := parseADRMeta(matches[0])

	if title != "Uso de Kafka" {
		t.Errorf("título esperado 'Uso de Kafka', obteve: %q", title)
	}
	if status != "Proposed" {
		t.Errorf("status esperado 'Proposed', obteve: %q", status)
	}
}
