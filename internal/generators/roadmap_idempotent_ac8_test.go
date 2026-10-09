package generators

// roadmap_idempotent_ac8_test.go — testes do AC8 (REQ-2026-09-09):
// `roadmap new` deixa de sobrescrever roadmap existente criada pelo `req new`.
//
// Cada teste declara, no próprio comentário, qual conclusão do ML ele afirma
// (Regra Dura de Reconciliação — CLAUDE.md).
//
// Falsificação obrigatória: remova a chamada findRoadmapByBasename (ou remova
// a checagem de existência) e o primeiro teste reprova imediatamente porque
// o roadmap sobrescrito perde o campo `req:`. Remova também o O_EXCL e
// substitua por os.WriteFile → o segundo teste reprova porque a escrita
// concorre e sobrescreve. Ambas são documentadas no relatório do ML-6B.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// setupAC8Dir cria um projeto temporário com trackfw.yaml flat e muda o cwd.
func setupAC8Dir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Reset de singleton de config (necessário: cada teste muda o cwd).
	config.Reset()
	t.Cleanup(func() { config.Reset() })
	return dir
}

// reqFixtureAC8 cria uma REQ mínima no formato gerado por `req new`.
func reqFixtureAC8(dir, name string) string {
	t := &testing.T{}
	_ = t
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		panic("mkdir docs/req: " + err.Error())
	}
	rel := filepath.Join("docs", "req", name)
	body := `---
status: Open
date: 2026-10-09
adr: ""
roadmap: ""
---

# REQ: Titulo AC8

## Acceptance Criteria
- [ ] AC1

## Linked Roadmap
Roadmap:
`
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
		panic("WriteFile REQ: " + err.Error())
	}
	return rel
}

// writeStateDir cria os subdiretórios de estado padrão do roadmap.
func writeStateDirs(dir string) {
	for _, state := range []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"} {
		_ = os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", state), 0o755)
	}
}

// TestRoadmapNew_SkipsWhenExistsInBacklog — afirma que `roadmap new T` após
// `req new T` (mesmo dia, mesmo slug) não sobrescreve o arquivo existente:
// o campo `req:` do roadmap criado pelo `req new` deve permanecer preenchido.
//
// Reconciliação: afirma a conclusão central do ML-6B (AC8) — a checagem
// findRoadmapByBasename detecta o arquivo existente e retorna nil sem escrever.
// Falsificação: remova findRoadmapByBasename e o teste reprova porque o
// roadmap sobrescrito volta a ter req:"".
func TestRoadmapNew_SkipsWhenExistsInBacklog(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)
	reqRel := reqFixtureAC8(dir, "REQ-2026-10-09-titulo-ac8.md")

	// Passo 1: criar o roadmap via from-req (simula `req new`).
	if err := NewRoadmapFromREQ(reqRel, "", false); err != nil {
		t.Fatalf("NewRoadmapFromREQ(): %v", err)
	}

	// Capturar conteúdo original do roadmap (deve ter req: preenchido).
	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d: %v", len(matches), err)
	}
	origBytes, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile roadmap original: %v", err)
	}
	if !strings.Contains(string(origBytes), `req: "`+reqRel+`"`) {
		t.Fatalf("roadmap original não tem req: preenchido:\n%s", origBytes)
	}

	// Passo 2: `roadmap new` com mesmo título — deve pular e NÃO sobrescrever.
	if err := NewRoadmap("Titulo AC8"); err != nil {
		t.Fatalf("NewRoadmap() deveria retornar nil (skip), obteve: %v", err)
	}

	// Verificar: arquivo deve ser byte-idêntico ao original.
	afterBytes, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile roadmap após skip: %v", err)
	}
	if string(origBytes) != string(afterBytes) {
		t.Errorf("AC8: roadmap foi sobrescrito — deve ser byte-idêntico ao original\noriginal:\n%s\ndepois:\n%s",
			origBytes, afterBytes)
	}

	// Anti-vacuidade: o campo req: ainda está preenchido.
	if !strings.Contains(string(afterBytes), `req: "`+reqRel+`"`) {
		t.Errorf("AC8: req: foi destruído pelo roadmap new — deve permanecer %q:\n%s", reqRel, afterBytes)
	}

	// Verificar que ainda existe apenas 1 arquivo em backlog (nenhum duplicado criado).
	matches2, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(matches2) != 1 {
		t.Errorf("AC8: esperado 1 roadmap em backlog após skip, obteve %d: %v", len(matches2), matches2)
	}
}

// TestRoadmapNew_CreatesWhenNoExistingFile — afirma que `roadmap new T` cria
// normalmente o arquivo quando não há roadmap existente com o mesmo nome-base.
//
// Reconciliação: afirma o caminho normal (sem colisão) — findRoadmapByBasename
// retorna "" e a criação prossegue. Este é o contra-braço que prova que o fix
// do AC8 não quebra o uso legítimo de `roadmap new` sem `req new` anterior.
func TestRoadmapNew_CreatesWhenNoExistingFile(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)

	if err := NewRoadmap("Titulo Novo Sem REQ"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d: %v", len(matches), err)
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "Titulo Novo Sem REQ") {
		t.Errorf("roadmap criado não contém o título:\n%s", body)
	}
}

// TestRoadmapNew_SkipsWhenExistsInWip — afirma que `roadmap new T` quando o
// roadmap já está em wip/ não cria um segundo arquivo em backlog/.
//
// Reconciliação: afirma que findRoadmapByBasename detecta o arquivo em wip/
// (diferente de backlog/) e retorna nil sem criar duplicata. O validate não
// dispararia `multiple states` porque nenhum arquivo novo foi criado.
// Falsificação: remova findRoadmapByBasename e haverá 2 roadmaps com o mesmo
// basename (backlog/ e wip/), fazendo `validate` disparar `multiple states`.
func TestRoadmapNew_SkipsWhenExistsInWip(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)
	reqRel := reqFixtureAC8(dir, "REQ-2026-10-09-titulo-ac8.md")

	// Passo 1: criar via from-req.
	if err := NewRoadmapFromREQ(reqRel, "", false); err != nil {
		t.Fatalf("NewRoadmapFromREQ(): %v", err)
	}

	// Passo 2: mover para wip/.
	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d: %v", len(matches), err)
	}
	if err := MoveRoadmap(filepath.Base(matches[0]), "wip"); err != nil {
		t.Fatalf("MoveRoadmap wip: %v", err)
	}

	// Passo 3: `roadmap new` com mesmo título — deve pular, não criar em backlog/.
	if err := NewRoadmap("Titulo AC8"); err != nil {
		t.Fatalf("NewRoadmap() deveria retornar nil (skip wip), obteve: %v", err)
	}

	// Verificar: backlog/ deve estar vazio.
	backlogMatches, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(backlogMatches) != 0 {
		t.Errorf("AC8: roadmap criado em backlog mesmo com existente em wip — %v", backlogMatches)
	}

	// Verificar: wip/ ainda tem exatamente 1 arquivo.
	wipMatches, _ := filepath.Glob("docs/roadmaps/wip/*.md")
	if len(wipMatches) != 1 {
		t.Errorf("AC8: esperado 1 roadmap em wip, obteve %d: %v", len(wipMatches), wipMatches)
	}
}

// TestRoadmapNew_ForceOverwritesSamePath — afirma que `roadmap new --force T`
// sobrescreve o roadmap existente em backlog/ (caso opt-in explícito).
//
// Reconciliação: afirma que Force:true bypassa a checagem e usa os.WriteFile
// (O_TRUNC), produzindo um arquivo novo com req:"" no lugar do original.
func TestRoadmapNew_ForceOverwritesSamePath(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)
	reqRel := reqFixtureAC8(dir, "REQ-2026-10-09-titulo-ac8.md")

	// Passo 1: criar via from-req (com req: preenchido).
	if err := NewRoadmapFromREQ(reqRel, "", false); err != nil {
		t.Fatalf("NewRoadmapFromREQ(): %v", err)
	}
	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d: %v", len(matches), err)
	}

	// Passo 2: `roadmap new --force T` deve sobrescrever.
	if err := NewRoadmapFromContent(RoadmapContent{Title: "Titulo AC8", Force: true}); err != nil {
		t.Fatalf("NewRoadmapFromContent(Force:true): %v", err)
	}

	afterBytes, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile após force: %v", err)
	}

	// O roadmap sobrescrito deve ter req:"" (template simples, sem REQ linkada).
	if strings.Contains(string(afterBytes), reqRel) {
		t.Errorf("AC8 --force: esperado roadmap sobrescrito com req vazio, mas req:%q ainda presente:\n%s",
			reqRel, afterBytes)
	}
}

// TestRoadmapNew_ForceRefusedWhenExistingInWip — afirma que `roadmap new --force T`
// quando o roadmap existente está em wip/ retorna erro (não cria duplicata).
//
// Reconciliação: afirma que --force com roadmap em estado diferente de backlog
// é recusado com error — a checagem `normExisting != normFilename` detecta a
// divergência de pasta antes de tentar escrever.
func TestRoadmapNew_ForceRefusedWhenExistingInWip(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)
	reqRel := reqFixtureAC8(dir, "REQ-2026-10-09-titulo-ac8.md")

	// Passo 1: criar e mover para wip/.
	if err := NewRoadmapFromREQ(reqRel, "", false); err != nil {
		t.Fatalf("NewRoadmapFromREQ(): %v", err)
	}
	matches, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d", len(matches))
	}
	if err := MoveRoadmap(filepath.Base(matches[0]), "wip"); err != nil {
		t.Fatalf("MoveRoadmap wip: %v", err)
	}

	// Passo 2: `roadmap new --force T` deve retornar erro.
	err := NewRoadmapFromContent(RoadmapContent{Title: "Titulo AC8", Force: true})
	if err == nil {
		t.Error("AC8 --force wip: esperado erro, obteve nil — --force não deve criar duplicata em estado diferente")
	}

	// Verificar que nenhum arquivo foi criado em backlog/.
	backlogMatches, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(backlogMatches) != 0 {
		t.Errorf("AC8 --force wip: arquivo criado em backlog mesmo com existente em wip — %v", backlogMatches)
	}
}
