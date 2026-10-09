package generators

// roadmap_idempotent_ac8_test.go — testes do AC8 (REQ-2026-09-09):
// `roadmap new` deixa de sobrescrever roadmap existente criada pelo `req new`.
//
// Também cobre os corretivos do red-team ML-6C (ML-6D, A2/A3/A4):
// - A2: repara req: vazio no roadmap quando exatamente 1 REQ aponta para ele.
// - A4: não vincula REQ diferente a roadmap que já pertence a outra REQ.
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
	"bytes"
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

// reqFixtureWithRoadmapLink cria uma REQ com `roadmap:` já apontando para roadmapBasename.
// Usado para cenários A2/A4 onde uma REQ já está vinculada a um roadmap.
func reqFixtureWithRoadmapLink(dir, reqName, roadmapBasename string) string {
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		panic("mkdir docs/req: " + err.Error())
	}
	rel := filepath.Join("docs", "req", reqName)
	body := `---
status: Open
date: 2026-10-09
adr: ""
roadmap: "` + roadmapBasename + `"
---

# REQ: Titulo AC8

## Acceptance Criteria
- [ ] AC1

## Linked Roadmap
Roadmap: ` + roadmapBasename + `
`
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
		panic("WriteFile REQ: " + err.Error())
	}
	return rel
}

// roadmapOrphanFixture cria um roadmap com req: "" (estado de versão antiga).
func roadmapOrphanFixture(dir, basename string) string {
	rel := filepath.Join("docs", "roadmaps", "backlog", basename)
	body := `---
status: backlog
date: 2026-10-09
req: ""
squad: ""
---

# Roadmap: Titulo AC8

> Created: 2026-10-09 | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ:

## Acceptance Criteria
- [ ]
`
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
		panic("WriteFile roadmap orphan: " + err.Error())
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

// ─── ML-6D (A2/A4) tests ─────────────────────────────────────────────────────

// TestRoadmapNew_A2_RepairsOrphanWithOneREQ — afirma que, no caminho "já existe"
// sem --req, se o roadmap tem req: "" e exatamente uma REQ aponta para ele, o
// campo req: do roadmap (e a linha REQ: do corpo) é reparado com o caminho dessa REQ.
//
// Reconciliação: afirma o mecanismo central do A2 (ML-6D) — reconcileExistingRoadmapLink
// detecta req: vazio, encontra 1 REQ via findREQsPointingToRoadmap, e chama
// rewriteREQRoadmapRefWith para reparar o roadmap.
// Falsificação: comente o bloco A2 em reconcileExistingRoadmapLink → o req: permanece
// vazio após `roadmap new` e o teste reprova na asserção do campo req:.
func TestRoadmapNew_A2_RepairsOrphanWithOneREQ(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)

	// Criar o roadmap órfão (req: "") diretamente, sem passar por NewRoadmapFromREQ.
	roadmapBasename := "ROADMAP-2026-10-09-titulo-ac8.md"
	roadmapOrphanFixture(dir, roadmapBasename)

	// Criar a REQ que aponta para ele.
	reqFixtureWithRoadmapLink(dir, "REQ-2026-10-09-titulo-ac8.md", roadmapBasename)

	// `roadmap new` sem --req deve detectar o req: vazio e reparar.
	if err := NewRoadmap("Titulo AC8"); err != nil {
		t.Fatalf("NewRoadmap() retornou erro inesperado: %v", err)
	}

	roadmapPath := filepath.Join("docs", "roadmaps", "backlog", roadmapBasename)
	afterBytes, err := os.ReadFile(roadmapPath)
	if err != nil {
		t.Fatalf("ReadFile após repair: %v", err)
	}

	wantReq := `req: "docs/req/REQ-2026-10-09-titulo-ac8.md"`
	if !strings.Contains(string(afterBytes), wantReq) {
		t.Errorf("A2: req: não foi reparado — esperado %q em:\n%s", wantReq, afterBytes)
	}
}

// TestRoadmapNew_A2_NoRepairWithZeroREQ — afirma que, no caminho "já existe"
// sem --req e com req: vazio, quando nenhuma REQ aponta para o roadmap, o
// arquivo não é alterado (req: permanece vazio).
//
// Reconciliação: afirma o ramo "len(pointing) == 0" do A2 — o roadmap fica
// intacto (byte-idêntico) e um aviso é emitido em stderr.
// Falsificação: altere `case 0:` para também reparar → o req: seria preenchido
// com um caminho inválido, e o teste reprova na asserção byte-idêntica.
func TestRoadmapNew_A2_NoRepairWithZeroREQ(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)

	roadmapBasename := "ROADMAP-2026-10-09-titulo-ac8.md"
	roadmapOrphanFixture(dir, roadmapBasename)
	// Sem nenhuma REQ apontando para o roadmap.

	roadmapPath := filepath.Join("docs", "roadmaps", "backlog", roadmapBasename)
	origBytes, err := os.ReadFile(roadmapPath)
	if err != nil {
		t.Fatalf("ReadFile original: %v", err)
	}

	if err := NewRoadmap("Titulo AC8"); err != nil {
		t.Fatalf("NewRoadmap() retornou erro inesperado: %v", err)
	}

	afterBytes, err := os.ReadFile(roadmapPath)
	if err != nil {
		t.Fatalf("ReadFile após skip: %v", err)
	}

	if !bytes.Equal(origBytes, afterBytes) {
		t.Errorf("A2 zero-REQ: roadmap foi alterado — deve ser byte-idêntico ao original\nantes:\n%s\ndepois:\n%s",
			origBytes, afterBytes)
	}
	// O req: ainda deve estar vazio.
	if strings.Contains(string(afterBytes), `req: "docs/req/`) {
		t.Errorf("A2 zero-REQ: req: foi preenchido inesperadamente:\n%s", afterBytes)
	}
}

// TestRoadmapNew_A2_NoRepairWithTwoREQs — afirma que, no caminho "já existe"
// sem --req e com req: vazio, quando mais de uma REQ aponta para o roadmap, o
// arquivo não é alterado (ambiguidade → não toca).
//
// Reconciliação: afirma o ramo "len(pointing) > 1" do A2 — o roadmap permanece
// byte-idêntico e um aviso de ambiguidade é emitido.
// Falsificação: altere `default:` para reparar com pointing[0] → o req: seria
// preenchido arbitrariamente, e o teste reprova na asserção byte-idêntica.
func TestRoadmapNew_A2_NoRepairWithTwoREQs(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)

	roadmapBasename := "ROADMAP-2026-10-09-titulo-ac8.md"
	roadmapOrphanFixture(dir, roadmapBasename)

	// Duas REQs apontando para o mesmo roadmap.
	reqFixtureWithRoadmapLink(dir, "REQ-2026-10-09-titulo-ac8.md", roadmapBasename)
	reqFixtureWithRoadmapLink(dir, "REQ-2026-10-09-titulo-ac8-v2.md", roadmapBasename)

	roadmapPath := filepath.Join("docs", "roadmaps", "backlog", roadmapBasename)
	origBytes, err := os.ReadFile(roadmapPath)
	if err != nil {
		t.Fatalf("ReadFile original: %v", err)
	}

	if err := NewRoadmap("Titulo AC8"); err != nil {
		t.Fatalf("NewRoadmap() retornou erro inesperado: %v", err)
	}

	afterBytes, err := os.ReadFile(roadmapPath)
	if err != nil {
		t.Fatalf("ReadFile após skip: %v", err)
	}

	if !bytes.Equal(origBytes, afterBytes) {
		t.Errorf("A2 dois-REQs: roadmap foi alterado — deve ser byte-idêntico ao original\nantes:\n%s\ndepois:\n%s",
			origBytes, afterBytes)
	}
}

// TestRoadmapNew_A4_DifferentREQ_NoLinkCreated — afirma que, no caminho "já existe"
// com --req REQ-B, quando o roadmap existente já tem req: apontando para REQ-A
// (diferente), nenhuma das duas REQs nem o roadmap são alterados (byte-idênticos).
//
// Reconciliação: afirma o guarda do A4 (ML-6D) — reconcileExistingRoadmapLink detecta
// a divergência de req: e retorna sem chamar linkREQToRoadmap, preservando todos
// os três arquivos intactos.
// Falsificação: remova o bloco A4 em reconcileExistingRoadmapLink (ou remova a
// guarda de divergência) → REQ-B ganha um ponteiro falso para o roadmap e o teste
// reprova na asserção byte-idêntica de REQ-B.
func TestRoadmapNew_A4_DifferentREQ_NoLinkCreated(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)

	roadmapBasename := "ROADMAP-2026-10-09-titulo-ac8.md"

	// Criar roadmap apontando para REQ-A.
	reqARel := filepath.Join("docs", "req", "REQ-2026-10-09-titulo-ac8-a.md")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("mkdir docs/req: %v", err)
	}
	roadmapWithReqA := `---
status: backlog
date: 2026-10-09
req: "` + reqARel + `"
squad: ""
---

# Roadmap: Titulo AC8

> Created: 2026-10-09 | Status: backlog

## Context
REQ: ` + reqARel + `
`
	roadmapPath := filepath.Join("docs", "roadmaps", "backlog", roadmapBasename)
	if err := os.WriteFile(filepath.Join(dir, roadmapPath), []byte(roadmapWithReqA), 0o644); err != nil {
		t.Fatalf("WriteFile roadmap: %v", err)
	}

	// REQ-A com roadmap: já vinculado.
	reqABody := `---
status: Open
date: 2026-10-09
adr: ""
roadmap: "` + roadmapBasename + `"
---

# REQ: Titulo AC8 A

## Linked Roadmap
Roadmap: ` + roadmapBasename + `
`
	if err := os.WriteFile(filepath.Join(dir, reqARel), []byte(reqABody), 0o644); err != nil {
		t.Fatalf("WriteFile REQ-A: %v", err)
	}

	// REQ-B sem vínculo.
	reqBRel := filepath.Join("docs", "req", "REQ-2026-10-09-titulo-ac8-b.md")
	reqBBody := `---
status: Open
date: 2026-10-09
adr: ""
roadmap: ""
---

# REQ: Titulo AC8 B

## Linked Roadmap
Roadmap:
`
	if err := os.WriteFile(filepath.Join(dir, reqBRel), []byte(reqBBody), 0o644); err != nil {
		t.Fatalf("WriteFile REQ-B: %v", err)
	}

	// Snapshots antes.
	snapRoadmap, _ := os.ReadFile(filepath.Join(dir, roadmapPath))
	snapReqA, _ := os.ReadFile(filepath.Join(dir, reqARel))
	snapReqB, _ := os.ReadFile(filepath.Join(dir, reqBRel))

	// `roadmap new --req REQ-B` sobre roadmap que já pertence a REQ-A.
	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "Titulo AC8",
		REQPath: reqBRel,
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() retornou erro inesperado: %v", err)
	}

	// Todos os três arquivos devem ser byte-idênticos aos snapshots.
	afterRoadmap, _ := os.ReadFile(filepath.Join(dir, roadmapPath))
	afterReqA, _ := os.ReadFile(filepath.Join(dir, reqARel))
	afterReqB, _ := os.ReadFile(filepath.Join(dir, reqBRel))

	if !bytes.Equal(snapRoadmap, afterRoadmap) {
		t.Errorf("A4: roadmap foi alterado — deve ser byte-idêntico ao snapshot\nantes:\n%s\ndepois:\n%s",
			snapRoadmap, afterRoadmap)
	}
	if !bytes.Equal(snapReqA, afterReqA) {
		t.Errorf("A4: REQ-A foi alterada — deve ser byte-idêntica ao snapshot\nantes:\n%s\ndepois:\n%s",
			snapReqA, afterReqA)
	}
	if !bytes.Equal(snapReqB, afterReqB) {
		t.Errorf("A4: REQ-B foi alterada — deve ser byte-idêntica ao snapshot (link não deve ser criado)\nantes:\n%s\ndepois:\n%s",
			snapReqB, afterReqB)
	}
}

// TestRoadmapNew_A4_OrphanRoadmap_GetsReqFilled — afirma que, no caminho "já existe"
// com --req R, quando o roadmap tem req: "" (órfão), o campo req: do roadmap (e a
// linha REQ: do corpo) é preenchido com R, e a REQ R recebe o ponteiro roadmap:.
//
// Reconciliação: afirma o complemento do ML-6D — reconcileExistingRoadmapLink, no
// caminho A4 com existingREQ fillable, agora chama rewriteREQRoadmapRefWith sobre o
// roadmap para gravar req: R além de chamar linkREQToRoadmap para gravar roadmap: em R.
// Falsificação: remova o bloco `if reqRoadmapFMIsFillable(existingREQ) { ... }` em
// reconcileExistingRoadmapLink → o roadmap permanece com req: "" e o teste reprova
// na asserção `strings.Contains(afterRoadmap, wantReq)`.
func TestRoadmapNew_A4_OrphanRoadmap_GetsReqFilled(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)

	roadmapBasename := "ROADMAP-2026-10-09-titulo-ac8.md"
	roadmapOrphanFixture(dir, roadmapBasename)

	// REQ R com roadmap: "" (vínculo ainda não estabelecido).
	reqRRel := reqFixtureAC8(dir, "REQ-2026-10-09-titulo-ac8.md")

	// `roadmap new --req R` sobre roadmap órfão.
	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "Titulo AC8",
		REQPath: reqRRel,
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() retornou erro inesperado: %v", err)
	}

	roadmapPath := filepath.Join("docs", "roadmaps", "backlog", roadmapBasename)
	afterRoadmap, err := os.ReadFile(filepath.Join(dir, roadmapPath))
	if err != nil {
		t.Fatalf("ReadFile roadmap após reconcile: %v", err)
	}
	afterREQ, err := os.ReadFile(filepath.Join(dir, reqRRel))
	if err != nil {
		t.Fatalf("ReadFile REQ após reconcile: %v", err)
	}

	// O roadmap deve ter req: R.
	wantReq := `req: "` + reqRRel + `"`
	if !strings.Contains(string(afterRoadmap), wantReq) {
		t.Errorf("A4 orphan: req: não foi preenchido no roadmap — esperado %q em:\n%s", wantReq, afterRoadmap)
	}

	// A REQ R deve apontar para o roadmap.
	if !strings.Contains(string(afterREQ), roadmapBasename) {
		t.Errorf("A4 orphan: roadmap: não foi preenchido na REQ — esperado %q em:\n%s", roadmapBasename, afterREQ)
	}
}

// TestRoadmapNew_A4_SameREQ_LinkOk — afirma que, no caminho "já existe" com
// --req REQ-A, quando o roadmap existente já tem req: apontando para REQ-A
// (mesma REQ, mesmo basename), a chamada não falha e o link pode ser completado.
//
// Reconciliação: afirma que a guarda do A4 deixa passar quando os basenames
// coincidem — "mesma causa, idempotente" é o caminho correto para re-execução.
// Falsificação: torne a guarda A4 mais estrita (sem comparação por basename,
// só string exata) → uma path ligeiramente diferente como "./docs/req/REQ-A.md"
// seria bloqueada mesmo sendo a mesma REQ, e o link nunca seria criado.
func TestRoadmapNew_A4_SameREQ_LinkOk(t *testing.T) {
	dir := setupAC8Dir(t)
	writeStateDirs(dir)

	roadmapBasename := "ROADMAP-2026-10-09-titulo-ac8.md"
	reqARel := filepath.Join("docs", "req", "REQ-2026-10-09-titulo-ac8.md")

	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("mkdir docs/req: %v", err)
	}

	// Roadmap com req: REQ-A.
	roadmapWithReqA := `---
status: backlog
date: 2026-10-09
req: "` + reqARel + `"
squad: ""
---

# Roadmap: Titulo AC8

> Created: 2026-10-09 | Status: backlog

## Context
REQ: ` + reqARel + `
`
	roadmapPath := filepath.Join("docs", "roadmaps", "backlog", roadmapBasename)
	if err := os.WriteFile(filepath.Join(dir, roadmapPath), []byte(roadmapWithReqA), 0o644); err != nil {
		t.Fatalf("WriteFile roadmap: %v", err)
	}

	// REQ-A com roadmap: vazio (simula re-execução onde req new sobrescreveu a REQ).
	reqABody := `---
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
	if err := os.WriteFile(filepath.Join(dir, reqARel), []byte(reqABody), 0o644); err != nil {
		t.Fatalf("WriteFile REQ-A: %v", err)
	}

	// `roadmap new --req REQ-A` — mesma REQ, deve completar o link sem erro.
	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "Titulo AC8",
		REQPath: reqARel,
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() retornou erro inesperado: %v", err)
	}

	// REQ-A deve agora ter roadmap: preenchido.
	reqAAfter, err := os.ReadFile(filepath.Join(dir, reqARel))
	if err != nil {
		t.Fatalf("ReadFile REQ-A após link: %v", err)
	}
	if !strings.Contains(string(reqAAfter), roadmapBasename) {
		t.Errorf("A4 same-REQ: REQ-A deveria ter roadmap: %q mas obteve:\n%s", roadmapBasename, reqAAfter)
	}
}
