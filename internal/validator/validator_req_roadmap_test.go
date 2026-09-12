package validator

// Tests for ML-1A (AC9): uma noção de "vinculada" — frontmatter como fonte de verdade.
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara em comentário qual conclusão
// do ML-1A ele afirma.

import (
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// buildReqRoadmapDir cria um diretório mínimo com os dirs necessários para o validate.
func buildReqRoadmapDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/req",
		"docs/adr",
	)
	return dir
}

// writeREQWithFields grava um arquivo REQ com frontmatter e/ou marcador de corpo configurável.
func writeREQWithFields(t *testing.T, dir, name, fmRoadmap, bodyRoadmap string) {
	t.Helper()
	var fm string
	if fmRoadmap != "" {
		fm = "---\nstatus: Open\ndate: 2026-09-12\nroadmap: \"" + fmRoadmap + "\"\n---\n"
	} else {
		fm = "---\nstatus: Open\ndate: 2026-09-12\nroadmap: \"\"\n---\n"
	}
	body := "\n# REQ: Fixture\n\n> Date: 2026-09-12 | Status: Open\n\n## Linked Roadmap\n"
	if bodyRoadmap != "" {
		body += "Roadmap: " + bodyRoadmap + "\n"
	} else {
		body += "Roadmap: <!-- none -->\n"
	}
	writeFile(t, dir, filepath.Join("docs/req", name), fm+body)
}

// TestValidateREQsHaveRoadmap_FrontmatterOnly — afirma que frontmatter `roadmap:` preenchido
// com corpo vazio é suficiente para req_has_roadmap passar (ML-1A: frontmatter é fonte de verdade).
func TestValidateREQsHaveRoadmap_FrontmatterOnly(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-fm-only.md",
		"docs/roadmaps/done/ROADMAP-x.md", // fmRoadmap preenchido
		"",                                  // bodyRoadmap vazio
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			t.Errorf("frontmatter `roadmap:` preenchido NÃO deve disparar req_has_roadmap, obteve violation: %q", v)
		}
	}
}

// TestValidateREQsHaveRoadmap_BodyOnly — afirma que corpo `Roadmap:` preenchido com frontmatter
// vazio ainda é aceito como fallback (ML-1A: body é fallback quando frontmatter está ausente).
func TestValidateREQsHaveRoadmap_BodyOnly(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-body-only.md",
		"",                                  // fmRoadmap vazio
		"docs/roadmaps/done/ROADMAP-x.md",  // bodyRoadmap preenchido
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			t.Errorf("corpo `Roadmap:` preenchido (frontmatter vazio) NÃO deve disparar req_has_roadmap, obteve violation: %q", v)
		}
	}
}

// TestValidateREQsHaveRoadmap_BothEqual — afirma que ambos os campos preenchidos e iguais passa
// sem violation nem divergence warning (ML-1A: contra-braço — os dois iguais → passa).
func TestValidateREQsHaveRoadmap_BothEqual(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-both-equal.md",
		"docs/roadmaps/done/ROADMAP-x.md",
		"docs/roadmaps/done/ROADMAP-x.md",
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			t.Errorf("ambos iguais NÃO deve disparar req_has_roadmap, obteve: %q", v)
		}
	}
	for _, w := range warnings {
		if hasWarning([]string{w}, "divergent roadmap") {
			t.Errorf("ambos iguais NÃO deve disparar req_roadmap_sync, obteve: %q", w)
		}
	}
}

// TestValidateREQsHaveRoadmap_Neither — afirma que ausência de ambos os campos produz violation
// (ML-1A: ausência → violation preserva o cheque de órfã).
func TestValidateREQsHaveRoadmap_Neither(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-neither.md", "", "") // ambos vazios
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	found := false
	for _, v := range violations {
		if hasViolation([]string{v}, "no linked Roadmap") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ausência de ambos os campos deve disparar req_has_roadmap violation, obteve violations=%v", violations)
	}
}

// TestValidateREQRoadmapSync_BasenameDiff — afirma que basename diferente entre frontmatter e
// corpo dispara req_roadmap_sync warning (ML-1A decisão 2: warning para divergência de basename).
func TestValidateREQRoadmapSync_BasenameDiff(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	// frontmatter aponta para ROADMAP-a.md, corpo aponta para ROADMAP-b.md — basenames diferentes
	writeREQWithFields(t, dir, "REQ-divergent.md",
		"docs/roadmaps/done/ROADMAP-a.md",
		"docs/roadmaps/done/ROADMAP-b.md",
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	found := false
	for _, w := range warnings {
		if hasWarning([]string{w}, "divergent roadmap") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("basename diferente entre frontmatter e corpo deve disparar req_roadmap_sync warning, obteve warnings=%v", warnings)
	}
}

// TestValidateREQRoadmapSync_StateDiffOnly — afirma que diferença apenas de pasta de estado
// (wip vs done, mesmo basename) NÃO dispara req_roadmap_sync (ML-1A decisão 2: estado diferente
// é esperado após roadmap move, não é divergência real).
func TestValidateREQRoadmapSync_StateDiffOnly(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	// frontmatter aponta para done/ROADMAP-x.md, corpo aponta para wip/ROADMAP-x.md (mesmo basename)
	writeREQWithFields(t, dir, "REQ-state-diff.md",
		"docs/roadmaps/done/ROADMAP-x.md",
		"docs/roadmaps/wip/ROADMAP-x.md",
	)
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	_, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	for _, w := range warnings {
		if hasWarning([]string{w}, "divergent roadmap") {
			t.Errorf("diferença de estado (wip vs done, mesmo basename) NÃO deve disparar req_roadmap_sync, obteve: %q", w)
		}
	}
}
