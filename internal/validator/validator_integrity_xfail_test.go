package validator

// validator_integrity_xfail_test.go — ML-1A: testes negativos que expõem os 3 escapes
// e o Defeito 2 do validator (REQ-2026-07-27-integridade-referencias).
//
// Semântica strict: xfailExpect executa o corpo e reporta XPASS se o defeito for corrigido.
// NÃO usa t.Skip — que não executa o corpo e ficaria cego para sempre.
// Cada teste cita qual ML o reativa ao converter de xfail para teste normal.

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// xfailExpect executa defectStillPresent() e espera true (defeito ainda presente).
// Se retornar false (defeito corrigido), reporta XPASS via t.Errorf — marcador strict.
// Setup: use t.Fatalf para erros de infra (não altera a semântica do xfail).
// REQ: REQ-2026-07-27-integridade-referencias
func xfailExpect(t *testing.T, reactivateML, reason string, defectStillPresent func() bool) {
	t.Helper()
	if defectStillPresent() {
		t.Logf("[xfail esperado] %s", reason)
	} else {
		t.Errorf("[XPASS inesperado — reativar em %s] defeito corrigido mas marcador não removido. %s",
			reactivateML, reason)
	}
}

// TestValidateRefTargetsExist_FrontmatterRoadmap
// REQ: REQ-2026-07-27-integridade-referencias | Reativado: ML-2A
// Escape 1 corrigido: roadmap: em frontmatter minúsculo e aspeado é validado.
func TestValidateRefTargetsExist_FrontmatterRoadmap(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/roadmaps/done",
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/adr",
	)
	// REQ: frontmatter aponta para inexistente; corpo não tem Roadmap:
	writeFile(t, dir, "docs/req/REQ-XFAIL-ESCAPE1.md",
		"---\nstatus: Open\nroadmap: \"docs/roadmaps/wip/NAO-EXISTE-ESCAPE-1.md\"\n---\n\n"+
			"# REQ: Escape 1\n\n> Date: 2026-07-27 | Status: Open\n\n"+
			"## Linked Roadmap\n")
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateRefTargetsExist()
	if err != nil {
		t.Fatalf("validateRefTargetsExist erro: %v", err)
	}
	if !hasWarning(warnings, "NAO-EXISTE-ESCAPE-1") {
		t.Fatalf("esperava warning para roadmap: inexistente no frontmatter; warnings=%v", warnings)
	}
}

// TestValidateRefTargetsExist_RejectsWrongStatePath
// REQ: REQ-2026-07-27-integridade-referencias | Reativado: ML-2A
// Escape 2 corrigido: referenceExists valida o caminho literal, sem fallback por basename.
func TestValidateRefTargetsExist_RejectsWrongStatePath(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/roadmaps/done",
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/adr",
	)
	// Arquivo real em done/ — basename fallback o encontra mesmo com path errado em wip/
	writeFile(t, dir, "docs/roadmaps/done/ESCAPE2-ROADMAP.md",
		"# Roadmap\n## Acceptance Criteria\n- [x] done\n")
	// REQ: corpo aponta para wip/ (errado) — arquivo está em done/
	writeFile(t, dir, "docs/req/REQ-XFAIL-ESCAPE2.md",
		"---\nstatus: Open\n---\n\n# REQ: Escape 2\n\n> Date: 2026-07-27 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: docs/roadmaps/wip/ESCAPE2-ROADMAP.md\n")
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateRefTargetsExist()
	if err != nil {
		t.Fatalf("validateRefTargetsExist erro: %v", err)
	}
	if !hasWarning(warnings, "ESCAPE2-ROADMAP") {
		t.Fatalf("esperava warning para caminho errado wip/ vs done/; warnings=%v", warnings)
	}
}

// TestValidateRefTargetsExist_DefaultError
// REQ: REQ-2026-07-27-integridade-referencias | Reativado: ML-3A
// Escape 3 corrigido: a severidade padrão de ref_targets_exist é "error", então
// referência quebrada reprova o gate sem configuração explícita.
func TestValidateRefTargetsExist_DefaultError(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/roadmaps/done",
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/adr",
	)
	// REQ com corpo Roadmap: apontando para arquivo verdadeiramente inexistente
	writeFile(t, dir, "docs/req/REQ-XFAIL-ESCAPE3.md",
		"---\nstatus: Open\n---\n\n# REQ: Escape 3\n\n> Date: 2026-07-27 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: docs/roadmaps/wip/ESCAPE3-TRULY-MISSING.md\n")
	// Config padrão — ref_targets_exist deve reprovar como "error"
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered erro: %v", err)
	}
	if !hasViolation(violations, "ESCAPE3-TRULY-MISSING") {
		t.Fatalf("esperava violation para ref quebrada com severidade default error; violations=%v warnings=%v", violations, warnings)
	}
}

// TestValidateRefTargetsExist_FrontmatterReqBasenameStillFails
// REQ: REQ-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-parametro-roots-morto
// ADR-2026-08-01: o contrato do campo req: é caminho relativo completo; um roadmap cujo
// frontmatter grava apenas o basename (mesmo que a REQ exista de verdade em docs/req/ e o
// corpo aponte para o caminho correto) DEVE continuar reprovando ref_targets_exist. A
// precedência frontmatter-sobre-corpo em extractRefPath não muda, e referenceExists NÃO
// resolve contra raízes (roots foi removido, não implementado) — este teste é o que impede
// alguém de "consertar" o falso positivo afrouxando o validador em vez de corrigir o gerador.
func TestValidateRefTargetsExist_FrontmatterReqBasenameStillFails(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/roadmaps/done",
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/adr",
	)
	// REQ real existe em docs/req/ — não é o caso de referência quebrada de verdade.
	writeFile(t, dir, "docs/req/REQ-BASENAME-TEST.md",
		"---\nstatus: Open\n---\n\n# REQ: Basename Test\n\n> Date: 2026-08-01 | Status: Open\n")
	// Roadmap: frontmatter grava só o basename; corpo grava o caminho completo correto.
	// extractRefPath lê o frontmatter primeiro (precedência), então o basename é o que conta.
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-BASENAME-TEST.md",
		"---\nstatus: wip\nreq: \"REQ-BASENAME-TEST.md\"\n---\n\n"+
			"# Roadmap: Basename Test\n\n## Context\nREQ: docs/req/REQ-BASENAME-TEST.md\n")
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateRefTargetsExist()
	if err != nil {
		t.Fatalf("validateRefTargetsExist erro: %v", err)
	}
	if !hasWarning(warnings, `roadmap "ROADMAP-BASENAME-TEST.md" links to REQ "REQ-BASENAME-TEST.md" which does not exist`) {
		t.Fatalf("esperava reprovação para req: com basename apenas no frontmatter, mesmo com REQ existente em docs/req/ e corpo correto; warnings=%v", warnings)
	}
}

// TestValidateREQRoadmapLifecycle_REQAbertaRoadmapConcluido
// REQ: REQ-2026-07-27-integridade-referencias | Reativado: ML-2B
// Defeito 2 corrigido: REQ com Status: Open cujo roadmap referenciado está em done/
// é sinalizada como inconsistência de ciclo de vida.
func TestValidateREQRoadmapLifecycle_REQAbertaRoadmapConcluido(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/roadmaps/done",
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/adr",
	)
	// Roadmap real em done/ — entregue
	writeFile(t, dir, "docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md",
		"---\nstatus: Done\ndate: 2026-07-01\n---\n# Roadmap concluído\n## Acceptance Criteria\n- [x] done\n")
	// REQ ainda Open mas roadmap está em done/
	writeFile(t, dir, "docs/req/REQ-XFAIL-DEFEITO2.md",
		"---\nstatus: Open\ndate: 2026-07-01\nroadmap: \"docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md\"\n---\n\n"+
			"# REQ: Defeito 2\n\n> Date: 2026-07-01 | Status: Open\n\n"+
			"## Linked Roadmap\nRoadmap: docs/roadmaps/done/DONE-ROADMAP-DEFEITO2.md\n")
	writeFile(t, dir, "trackfw.yaml",
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered erro: %v", err)
	}
	if !hasViolation(violations, "DONE-ROADMAP-DEFEITO2") && !hasWarning(warnings, "DONE-ROADMAP-DEFEITO2") {
		t.Fatalf("esperava mensagem sobre REQ Open com roadmap done; violations=%v warnings=%v", violations, warnings)
	}
}
