package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// Testes do ML-1C (AC8 da REQ-2026-09-28): FindRoadmapLinkingREQ não cruza namespace de agente
// em modo by_agent — uma REQ de `apolo` não deve ser vinculada ao roadmap de `hades` só porque
// os dois têm o mesmo basename.
//
// Cada teste declara, no próprio comentário, qual conclusão do ML ele afirma
// (Regra Dura de Reconciliação — CLAUDE.md).

// setupByAgentProject prepara um diretório temporário com:
//   - trackfw.yaml: roadmap_namespacing=by_agent, agents=[hades, apolo]
//   - docs/req/hades/ e docs/req/apolo/
//   - docs/roadmaps/hades/<estado>/ e docs/roadmaps/apolo/<estado>/  (todos os estados)
//
// É o fixture mínimo para que roadmapCandidateFiles e FindRoadmapLinkingREQ funcionem
// corretamente no modo by_agent. Sem os diretórios de estado criados no disco,
// roadmapCandidateFiles retorna lista vazia — e "apolo não encontra hades" passaria
// vacuamente sem que nenhum roadmap fosse varrido.
func setupByAgentProject(t *testing.T) {
	t.Helper()
	yaml := "roadmap_namespacing: by_agent\nagents:\n- hades\n- apolo\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("trackfw.yaml: %v", err)
	}
	for _, agent := range []string{"hades", "apolo"} {
		for _, state := range []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"} {
			if err := os.MkdirAll(filepath.Join("docs", "roadmaps", agent, state), 0755); err != nil {
				t.Fatalf("mkdir roadmaps/%s/%s: %v", agent, state, err)
			}
		}
		if err := os.MkdirAll(filepath.Join("docs", "req", agent), 0755); err != nil {
			t.Fatalf("mkdir req/%s: %v", agent, err)
		}
	}
}

// writeRoadmapWithReq grava um roadmap mínimo com o campo req: apontando para reqRef.
func writeRoadmapWithReq(t *testing.T, roadmapRel, reqRef string) {
	t.Helper()
	content := "---\nstatus: backlog\nreq: \"" + reqRef + "\"\n---\n\n# Roadmap\n"
	if err := os.WriteFile(roadmapRel, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile roadmap %s: %v", roadmapRel, err)
	}
}

// Reconciliação: afirma a conclusão central do ML-1C — em modo by_agent, FindRoadmapLinkingREQ
// NÃO retorna o roadmap de `hades` quando a REQ de entrada pertence a `apolo`, mesmo que os
// dois tenham o mesmo basename.
//
// Sem a correção: a busca por basename retornava o roadmap de `hades` → vínculo cruzado.
// Com a correção: o guard de namespace descarta candidatos de agente diferente → retorna "".
func TestFindRoadmapLinkingREQ_ByAgent_DoesNotCrossNamespace(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	setupByAgentProject(t)

	// Roadmap de hades aponta para REQ de hades com o mesmo basename.
	hadesRoadmapRel := filepath.Join("docs", "roadmaps", "hades", "backlog", "ROADMAP-2026-09-28-same-title-test.md")
	hadesReqRef := "docs/req/hades/REQ-2026-09-28-same-title-test.md"
	writeRoadmapWithReq(t, hadesRoadmapRel, hadesReqRef)

	// REQ de apolo com mesmo basename mas namespace diferente.
	apoloReqPath := "docs/req/apolo/REQ-2026-09-28-same-title-test.md"

	got := FindRoadmapLinkingREQ(apoloReqPath)

	// Esta guarda reprovou legitimamente no Windows (PR #464) e expôs que
	// roadmapCandidateFiles devolvia separador nativo. Foi corrigido no PRODUTO
	// (roadmap.go: ToSlash no retorno), não aqui. NÃO relaxe esta guarda: sem ela
	// o teste passa vacuamente no Windows, porque "não retorna o roadmap errado"
	// é trivialmente verdadeiro quando a busca não encontra nada.
	// Anti-vacuidade: verificar que o roadmap de hades está de fato na árvore e seria
	// encontrado por uma busca ingênua (basename-only). Sem esta verificação, um
	// roadmapCandidateFiles vazio passaria no teste por vácuo.
	allCandidates := roadmapCandidateFiles(config.Load())
	foundHadesInCandidates := false
	for _, c := range allCandidates {
		if c == filepath.ToSlash(hadesRoadmapRel) {
			foundHadesInCandidates = true
			break
		}
	}
	if !foundHadesInCandidates {
		t.Fatalf("anti-vacuidade: roadmap de hades não está em roadmapCandidateFiles — "+
			"a busca baseline estava vazia e o teste seria trivialmente satisfeito: %v", allCandidates)
	}

	if got != "" {
		t.Errorf("REQ de apolo não deve vincular ao roadmap de hades: got %q, want %q", got, "")
	}
}

// Reconciliação (braço positivo / anti-vacuidade): afirma que o guard de namespace ML-1C NÃO
// quebra a idempotência de mesmo agente — uma REQ de `apolo` AINDA encontra o roadmap de
// `apolo` que a aponta. Sem este teste, um guard que sempre retorna "" satisfaria o teste
// anterior enquanto destruiria a funcionalidade de idempotência.
func TestFindRoadmapLinkingREQ_ByAgent_FindsOwnNamespace(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	setupByAgentProject(t)

	// Roadmap de apolo aponta para REQ de apolo.
	apoloRoadmapRel := filepath.Join("docs", "roadmaps", "apolo", "backlog", "ROADMAP-2026-09-28-same-title-test.md")
	apoloReqRef := "docs/req/apolo/REQ-2026-09-28-same-title-test.md"
	writeRoadmapWithReq(t, apoloRoadmapRel, apoloReqRef)

	got := FindRoadmapLinkingREQ(apoloReqRef)
	want := filepath.ToSlash(apoloRoadmapRel)

	if got != want {
		t.Errorf("REQ de apolo deveria encontrar roadmap de apolo: got %q, want %q", got, want)
	}
}

// Reconciliação (contra-braço flat): afirma que o ML-1C NÃO quebra o modo flat — o vínculo
// por basename continua funcionando em projetos sem by_agent. Este é o caminho comum: qualquer
// alteração que corrija o raro (by_agent) e quebre o frequente (flat) é uma regressão.
//
// O teste exerce FindRoadmapLinkingREQ diretamente com um projeto flat (sem trackfw.yaml de
// by_agent), garantindo que o guard de namespace nunca é ativado e o resultado é idêntico ao
// comportamento pré-ML-1C.
func TestFindRoadmapLinkingREQ_Flat_BasenameStillWorks(t *testing.T) {
	dir := t.TempDir()
	chdirREQ(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Projeto flat: sem by_agent, sem agents.
	for _, state := range []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"} {
		if err := os.MkdirAll(filepath.Join("docs", "roadmaps", state), 0755); err != nil {
			t.Fatalf("mkdir roadmaps/%s: %v", state, err)
		}
	}
	if err := os.MkdirAll(filepath.Join("docs", "req"), 0755); err != nil {
		t.Fatalf("mkdir req: %v", err)
	}

	// Roadmap flat aponta para REQ flat.
	flatRoadmapRel := filepath.Join("docs", "roadmaps", "backlog", "ROADMAP-2026-09-28-flat-title.md")
	flatReqRef := "docs/req/REQ-2026-09-28-flat-title.md"
	writeRoadmapWithReq(t, flatRoadmapRel, flatReqRef)

	got := FindRoadmapLinkingREQ(flatReqRef)
	want := filepath.ToSlash(flatRoadmapRel)

	if got != want {
		t.Errorf("modo flat: vínculo por basename deveria retornar %q, got %q", want, got)
	}
}
