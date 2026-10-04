package validator

// validator_roadmap_wave0_cutoff_test.go — D5 (ADR-2026-10-04, REQ #514 ML-1C).
//
// Testa o corte por data da exigência de Wave 0 (2026-09-18) em:
//   1. A regra roadmap_wave0_required no validator (validateRoadmapGatesCoverage).
//   2. O gate do `move … done` em generators/roadmap.go (coberto em generators/).
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara, em uma frase, qual
// conclusão do próprio ML-1B/ML-1C ele afirma.
//
// Sabotagens cobertas:
//   S3 (isenção ignorada no validator): derruba TestRoadmapWave0Cutoff_PreCutoffIsExempt
//      e TestRoadmapWave0Cutoff_FrontmatterDateWins.
//   S5 (frontmatter ignorado em RoadmapCreationDate): derruba
//      TestRoadmapWave0Cutoff_FrontmatterDateWins, TestRoadmapWave0Cutoff_RoadmapCreationDateFrontmatterFirst
//      e TestMoveDoneWave0Cutoff_FrontmatterDateWins (generators/).

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// writeWIPRoadmapWithoutWave0 grava um roadmap em wip/ SEM heading ## Wave 0,
// com data controlada no frontmatter (fmDate) ou no nome (que deve ser ROADMAP-AAAA-MM-DD).
func writeWIPRoadmapWithoutWave0(t *testing.T, dir, name, fmDate string) {
	t.Helper()
	fm := "---\nstatus: wip\n"
	if fmDate != "" {
		fm += "date: " + fmDate + "\n"
	}
	fm += "---\n"
	// Sem ## Wave 0 — só Wave 1 para ser um roadmap reconhecível
	body := "\n# Roadmap: Fixture D5\n\n## Wave 1 — Implementação\n\n### ML-1A — Única tarefa\n\n**Status:** ⬜ Pendente\n"
	writeFile(t, dir, "docs/roadmaps/wip/"+name, fm+body)
}

// writeWIPRoadmapWithWave0 grava um roadmap em wip/ COM heading ## Wave 0.
func writeWIPRoadmapWithWave0(t *testing.T, dir, name, fmDate string) {
	t.Helper()
	fm := "---\nstatus: wip\ndate: " + fmDate + "\n---\n"
	body := "\n# Roadmap: Fixture D5\n\n## Wave 0 — Threat Model\n\n> Sem ameaças identificadas.\n\n## Wave 1 — Implementação\n\n### ML-1A — Única tarefa\n\n**Status:** ⬜ Pendente\n"
	writeFile(t, dir, "docs/roadmaps/wip/"+name, fm+body)
}

// validateWave0Fixture executa ValidateUnfiltered num diretório de fixture D5.
func validateWave0Fixture(t *testing.T, dir string) (violations, warnings []string) {
	t.Helper()
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)
	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	return violations, warnings
}

// ---------------------------------------------------------------------------
// Testes D5 — roadmap_wave0_required no validator
// ---------------------------------------------------------------------------

// TestRoadmapWave0Cutoff_PreCutoffIsExempt — AFIRMA a conclusão do ML-1B de que
// um roadmap em wip/ sem Wave 0 datado ANTES de 2026-09-18 fica isento da regra
// (a exigência ainda não existia), e a isenção é contada no aviso agregado.
func TestRoadmapWave0Cutoff_PreCutoffIsExempt(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeWIPRoadmapWithoutWave0(t, dir, "ROADMAP-2026-09-12-pre-cutoff.md", "2026-09-12")
	violations, warnings := validateWave0Fixture(t, dir)

	for _, v := range violations {
		if strings.Contains(v, "Wave 0") {
			t.Errorf("roadmap pré-cutoff SEM Wave 0 NÃO deve ser violation, obteve %q", v)
		}
	}
	// A isenção deve ser visível no aviso agregado
	exemptFound := false
	for _, w := range warnings {
		if strings.Contains(w, "roadmap_wave0_required:") && strings.Contains(w, "exempt") {
			exemptFound = true
		}
	}
	if !exemptFound {
		t.Errorf("isenção de Wave 0 pré-cutoff deve produzir aviso agregado visível, warnings=%v", warnings)
	}
}

// TestRoadmapWave0Cutoff_OnCutoffIsNotExempt — AFIRMA a conclusão do ML-1B de que
// um roadmap datado exatamente em 2026-09-18 (o cutoff, inclusive) NÃO é isento:
// o corte é estritamente menor que (<), então a data de entrada não dispensa.
func TestRoadmapWave0Cutoff_OnCutoffIsNotExempt(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeWIPRoadmapWithoutWave0(t, dir, "ROADMAP-2026-09-18-on-cutoff.md", "2026-09-18")
	violations, _ := validateWave0Fixture(t, dir)

	found := false
	for _, v := range violations {
		if strings.Contains(v, "Wave 0") {
			found = true
		}
	}
	if !found {
		t.Errorf("roadmap datado 2026-09-18 (on-cutoff) sem Wave 0 DEVE ser violation, obteve violations=%v", violations)
	}
}

// TestRoadmapWave0Cutoff_WithWave0NeverFires — AFIRMA a escolha de design do ML-1B:
// um roadmap com ## Wave 0 nunca dispara a regra roadmap_wave0_required,
// independentemente da data.
func TestRoadmapWave0Cutoff_WithWave0NeverFires(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeWIPRoadmapWithWave0(t, dir, "ROADMAP-2026-09-20-with-wave0.md", "2026-09-20")
	violations, _ := validateWave0Fixture(t, dir)

	for _, v := range violations {
		if strings.Contains(v, "Wave 0") {
			t.Errorf("roadmap com Wave 0 NÃO deve disparar, obteve %q", v)
		}
	}
}

// TestRoadmapWave0Cutoff_PostCutoffNoWave0IsViolation — AFIRMA a conclusão do ML-1B
// de que roadmap em wip/ sem Wave 0 datado A PARTIR de 2026-09-18 é cobrado normalmente
// como violation da regra roadmap_wave0_required.
func TestRoadmapWave0Cutoff_PostCutoffNoWave0IsViolation(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeWIPRoadmapWithoutWave0(t, dir, "ROADMAP-2026-09-20-no-wave0.md", "2026-09-20")
	violations, _ := validateWave0Fixture(t, dir)

	found := false
	for _, v := range violations {
		if strings.Contains(v, "Wave 0") {
			found = true
		}
	}
	if !found {
		t.Errorf("roadmap pós-cutoff sem Wave 0 DEVE ser violation, obteve violations=%v", violations)
	}
}

// TestRoadmapWave0Cutoff_FrontmatterDateWins — AFIRMA a régua D6/T7 do ML-1B:
// `date:` do frontmatter tem precedência sobre a data do nome do arquivo.
// Um roadmap com data pós-cutoff no NOME mas pré-cutoff no FRONTMATTER é isento.
func TestRoadmapWave0Cutoff_FrontmatterDateWins(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	// Nome tem 2026-09-20 (pós-cutoff), frontmatter tem 2026-09-12 (pré-cutoff)
	writeWIPRoadmapWithoutWave0(t, dir, "ROADMAP-2026-09-20-nome-novo.md", "2026-09-12")
	violations, warnings := validateWave0Fixture(t, dir)

	for _, v := range violations {
		if strings.Contains(v, "Wave 0") {
			t.Errorf("frontmatter pré-cutoff deve vencer nome pós-cutoff (isento), obteve %q", v)
		}
	}
	exemptFound := false
	for _, w := range warnings {
		if strings.Contains(w, "roadmap_wave0_required:") && strings.Contains(w, "exempt") {
			exemptFound = true
		}
	}
	if !exemptFound {
		t.Errorf("isenção via frontmatter deve aparecer no aviso, warnings=%v", warnings)
	}
}

// TestRoadmapWave0Cutoff_UnknownDateFailsClosed — AFIRMA a semântica fail-closed do
// ML-1B: um roadmap sem data legível (sem frontmatter date: e sem AAAA-MM-DD no nome)
// NÃO é isento — a ausência de data não abre bypass de isenção.
func TestRoadmapWave0Cutoff_UnknownDateFailsClosed(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeWIPRoadmapWithoutWave0(t, dir, "ROADMAP-sem-data-nenhuma.md", "")
	violations, _ := validateWave0Fixture(t, dir)

	found := false
	for _, v := range violations {
		if strings.Contains(v, "Wave 0") {
			found = true
		}
	}
	if !found {
		t.Errorf("roadmap sem data legível deve ser cobrado (fail-closed), obteve violations=%v", violations)
	}
}

// TestRoadmapWave0Cutoff_ConstanteParseavel — AFIRMA que a constante exportada
// RoadmapWave0Cutoff parseia como data válida, e que RoadmapWave0CutoffDate()
// retorna a data correta (sem zero time).
func TestRoadmapWave0Cutoff_ConstanteParseavel(t *testing.T) {
	d := RoadmapWave0CutoffDate()
	if d.IsZero() {
		t.Fatalf("RoadmapWave0Cutoff=%q não parseia como 2006-01-02", RoadmapWave0Cutoff)
	}
	if d.Format("2006-01-02") != RoadmapWave0Cutoff {
		t.Errorf("round-trip divergiu: %q != %q", d.Format("2006-01-02"), RoadmapWave0Cutoff)
	}
}

// TestRoadmapWave0Cutoff_RoadmapCreationDateFrontmatterFirst — AFIRMA a régua D6/T7
// de que RoadmapCreationDate lê `date:` do frontmatter como primeiro candidato, com
// fallback para a primeira AAAA-MM-DD no basename.
func TestRoadmapWave0Cutoff_RoadmapCreationDateFrontmatterFirst(t *testing.T) {
	t.Run("frontmatter_lido", func(t *testing.T) {
		content := "---\ndate: 2026-09-12\n---\n"
		d, ok := RoadmapCreationDate(content, "ROADMAP-2026-09-20-slug.md")
		if !ok {
			t.Fatal("RoadmapCreationDate deve retornar ok=true com date: no frontmatter")
		}
		if d.Format("2006-01-02") != "2026-09-12" {
			t.Errorf("frontmatter deve vencer: esperado 2026-09-12, obteve %s", d.Format("2006-01-02"))
		}
	})
	t.Run("fallback_para_nome", func(t *testing.T) {
		content := "---\n---\n" // sem date:
		d, ok := RoadmapCreationDate(content, "/path/to/ROADMAP-2026-09-08-slug.md")
		if !ok {
			t.Fatal("RoadmapCreationDate deve retornar ok=true com data no nome")
		}
		if d.Format("2006-01-02") != "2026-09-08" {
			t.Errorf("nome deve ser fallback: esperado 2026-09-08, obteve %s", d.Format("2006-01-02"))
		}
	})
	t.Run("sem_data_legivel", func(t *testing.T) {
		content := "---\n---\n"
		_, ok := RoadmapCreationDate(content, "ROADMAP-sem-data.md")
		if ok {
			t.Error("sem data legível em nenhuma régua, ok deve ser false")
		}
	})
}

