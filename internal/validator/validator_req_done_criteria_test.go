package validator

// validator_req_done_criteria_test.go — D4 (ADR-2026-10-04, REQ #514 ML-1C).
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara, em uma frase, qual
// conclusão do próprio ML-1B/ML-1C ele afirma.
//
// Sabotagens cobertas:
//   S1 (cutoff < → <=): derruba TestReqDoneOpenCriteria_PostCutoffIsWarning e
//      TestReqDoneOpenCriteria_EnforcedCountInNotice.
//   S2 (Unmet+Lapsed em vez de Unmet): derruba TestReqDoneOpenCriteria_LapsedDoesNotFire
//      e TestReqDoneOpenCriteria_SectionScanCountsOnlyUnmet.

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// writeDoneREQWithOpenCriteria grava uma REQ Done com um critério aberto (- [ ])
// sem Caducou:, e com data controlada via `fmDate` (frontmatter) ou no nome do arquivo.
// A fixture usa a seção ## Acceptance Criteria com um critério aberto.
func writeDoneREQWithOpenCriteria(t *testing.T, dir, name, fmDate string) {
	t.Helper()
	fm := "---\nstatus: Done\n"
	if fmDate != "" {
		fm += "date: " + fmDate + "\n"
	}
	fm += "roadmap: \"docs/roadmaps/done/ROADMAP-x.md\"\n---\n"
	body := "\n# REQ: Fixture D4\n\n## Acceptance Criteria\n\n- [x] Critério atendido\n- [ ] Critério aberto sem Caducou:\n\n## Linked Roadmap\nRoadmap: docs/roadmaps/done/ROADMAP-x.md\n"
	writeFile(t, dir, "docs/req/"+name, fm+body)
}

// writeDoneREQWithLapsedCriteria grava uma REQ Done em que o único critério aberto
// tem Caducou: com justificativa na LINHA SEGUINTE com ≥2 espaços — portanto o
// parser conta Lapsed, não Unmet (D3: Caducou: é continuation, não inline).
func writeDoneREQWithLapsedCriteria(t *testing.T, dir, name, fmDate string) {
	t.Helper()
	fm := "---\nstatus: Done\ndate: " + fmDate + "\nroadmap: \"docs/roadmaps/done/ROADMAP-x.md\"\n---\n"
	// Caducou: deve estar na linha SEGUINTE com ≥2 espaços de indentação
	body := "\n# REQ: Fixture D4 lapsed\n\n## Acceptance Criteria\n\n- [x] Critério atendido\n- [ ] Critério caducado\n  Caducou: foi substituído por nova abordagem\n\n## Linked Roadmap\nRoadmap: docs/roadmaps/done/ROADMAP-x.md\n"
	writeFile(t, dir, "docs/req/"+name, fm+body)
}

// writeDoneREQMetOnly grava uma REQ Done com critérios só atendidos (- [x]).
func writeDoneREQMetOnly(t *testing.T, dir, name, fmDate string) {
	t.Helper()
	fm := "---\nstatus: Done\ndate: " + fmDate + "\nroadmap: \"docs/roadmaps/done/ROADMAP-x.md\"\n---\n"
	body := "\n# REQ: Fixture D4 met only\n\n## Acceptance Criteria\n\n- [x] Atendido\n\n## Linked Roadmap\nRoadmap: docs/roadmaps/done/ROADMAP-x.md\n"
	writeFile(t, dir, "docs/req/"+name, fm+body)
}

// validateD4Fixture executa ValidateUnfiltered num diretório de fixture D4.
func validateD4Fixture(t *testing.T, dir string) (violations, warnings []string) {
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

// findD4Notice localiza a linha de aviso req_done_open_criteria: nos warnings.
func findD4Notice(t *testing.T, warnings []string) string {
	t.Helper()
	for _, w := range warnings {
		if strings.Contains(w, reqDoneOpenCriteriaNoticeSubstr) {
			return w
		}
	}
	t.Fatalf("aviso req_done_open_criteria ausente dos warnings: %v", warnings)
	return ""
}

// ---------------------------------------------------------------------------
// Testes de D4
// ---------------------------------------------------------------------------

// TestReqDoneOpenCriteria_PostCutoffIsWarning — AFIRMA a conclusão do ML-1B de que
// uma REQ Done criada em 2026-10-04 (o cutoff, inclusive) com critério aberto deve
// sair por WARNING (severidade da regra), e não como violation nem silêncio.
func TestReqDoneOpenCriteria_PostCutoffIsWarning(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	// Criar o roadmap alvo para que req_has_roadmap não dispare ruído
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-04-no-cutoff.md", "2026-10-04")
	_, warnings := validateD4Fixture(t, dir)

	if !hasWarning(warnings, "open acceptance criteria") {
		t.Errorf("REQ pós-cutoff (2026-10-04) com critério aberto DEVE gerar warning, obteve warnings=%v", warnings)
	}
}

// TestReqDoneOpenCriteria_PreCutoffIsExempt — AFIRMA a conclusão do ML-1B de que
// uma REQ Done criada ANTES de 2026-10-04 com critério aberto fica ISENTA (sem warning
// individual), porque a regra é nova e cobrar retroativamente seria retroativo.
func TestReqDoneOpenCriteria_PreCutoffIsExempt(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-03-pre-cutoff.md", "2026-10-03")
	_, warnings := validateD4Fixture(t, dir)

	for _, w := range warnings {
		if strings.Contains(w, "open acceptance criteria") {
			t.Errorf("REQ pré-cutoff NÃO deve gerar warning individual, obteve %q", w)
		}
	}
}

// TestReqDoneOpenCriteria_LapsedDoesNotFire — AFIRMA a conclusão do ML-1B de que
// - [ ] com Caducou: (lapsed) NÃO conta como critério aberto: a regra lê detail.Unmet,
// que exclui os lapsed — logo REQ com apenas critérios atendidos ou caducados não dispara.
func TestReqDoneOpenCriteria_LapsedDoesNotFire(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	// Data pós-cutoff: se a regra fosse disparar, dispararia aqui
	writeDoneREQWithLapsedCriteria(t, dir, "REQ-2026-10-04-lapsed.md", "2026-10-04")
	_, warnings := validateD4Fixture(t, dir)

	for _, w := range warnings {
		if strings.Contains(w, "open acceptance criteria") {
			t.Errorf("REQ com critério caducado (Caducou:) NÃO deve disparar req_done_open_criteria, obteve %q", w)
		}
	}
}

// TestReqDoneOpenCriteria_MetOnlyDoesNotFire — AFIRMA a escolha de design do ML-1B:
// uma REQ Done com todos os critérios marcados [x] não produz nenhum aviso da regra D4.
func TestReqDoneOpenCriteria_MetOnlyDoesNotFire(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	writeDoneREQMetOnly(t, dir, "REQ-2026-10-04-met.md", "2026-10-04")
	_, warnings := validateD4Fixture(t, dir)

	for _, w := range warnings {
		if strings.Contains(w, "open acceptance criteria") {
			t.Errorf("REQ com apenas critérios atendidos NÃO deve disparar req_done_open_criteria, obteve %q", w)
		}
	}
}

// TestReqDoneOpenCriteria_NoticeShowsCountsAndCutoff — AFIRMA a conclusão do ML-1B de
// que a isenção deve ser VISÍVEL: a linha de aviso agrega exempt, enforced e scanned,
// tornando-a distinguível de silêncio (mesmo quando não há enforced).
func TestReqDoneOpenCriteria_NoticeShowsCountsAndCutoff(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-03-pre-A.md", "2026-10-03")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-03-pre-B.md", "2026-10-03")
	_, warnings := validateD4Fixture(t, dir)

	notice := findD4Notice(t, warnings)
	for _, want := range []string{
		"2 Done REQ(s) with open criteria exempt",
		reqDoneOpenCriteriaCutoff,
		"0 enforced",
		"2 Done REQ(s) scanned",
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("aviso D4 deve conter %q, obteve %q", want, notice)
		}
	}
}

// TestReqDoneOpenCriteria_EnforcedCountInNotice — AFIRMA que quando uma REQ pós-cutoff
// dispara, o campo "enforced" no aviso reflete a contagem correta.
func TestReqDoneOpenCriteria_EnforcedCountInNotice(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-03-pre.md", "2026-10-03")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-04-enforced.md", "2026-10-04")
	_, warnings := validateD4Fixture(t, dir)

	notice := findD4Notice(t, warnings)
	for _, want := range []string{"1 Done REQ(s) with open criteria exempt", "1 enforced", "2 Done REQ(s) scanned"} {
		if !strings.Contains(notice, want) {
			t.Errorf("aviso D4 (enforced) deve conter %q, obteve %q", want, notice)
		}
	}
}

// TestReqDoneOpenCriteria_ConstanteParseavel — AFIRMA que a constante de cutoff do D4
// parseia como data válida, e o fallback da função (zero de time.Time em corrupção de
// literal) não isentaria nenhum artefato silenciosamente.
func TestReqDoneOpenCriteria_ConstanteParseavel(t *testing.T) {
	d := reqDoneOpenCriteriaCutoffDate()
	if d.IsZero() {
		t.Fatalf("constante reqDoneOpenCriteriaCutoff=%q não parseia como 2006-01-02", reqDoneOpenCriteriaCutoff)
	}
	if d.Format("2006-01-02") != reqDoneOpenCriteriaCutoff {
		t.Errorf("round-trip divergiu: %q != %q", d.Format("2006-01-02"), reqDoneOpenCriteriaCutoff)
	}
}

// TestReqDoneOpenCriteria_SectionScanDoesNotLeakBodyCheckboxes — AFIRMA que
// countREQOpenCriteria lê SOMENTE a seção ## Acceptance Criteria: checkboxes fora
// dessa seção (ex.: em ## Notas) não são contados como critérios abertos.
func TestReqDoneOpenCriteria_SectionScanDoesNotLeakBodyCheckboxes(t *testing.T) {
	// REQ com critério aberto FORA da seção AC — deve retornar 0
	content := "---\nstatus: Done\ndate: 2026-10-04\n---\n\n# REQ\n\n## Notas\n\n- [ ] tarefa fora do AC\n\n## Acceptance Criteria\n\n- [x] único critério\n"
	got := countREQOpenCriteria(content)
	if got != 0 {
		t.Errorf("checkbox fora da seção AC NÃO deve contar: countREQOpenCriteria()=%d, esperado 0", got)
	}
}

// TestReqDoneOpenCriteria_SectionScanCountsOnlyUnmet — AFIRMA que a função
// countREQOpenCriteria conta exatamente os critérios Unmet (sem Caducou:), separando
// met ([x]) e lapsed ([ ] com Caducou: na linha seguinte) dos genuinamente abertos.
func TestReqDoneOpenCriteria_SectionScanCountsOnlyUnmet(t *testing.T) {
	// Caducou: deve estar na linha seguinte com ≥2 espaços (D3)
	content := "---\nstatus: Done\ndate: 2026-10-04\n---\n\n# REQ\n\n## Acceptance Criteria\n\n- [x] atendido\n- [ ] caducado\n  Caducou: superado por nova abordagem\n- [ ] aberto sem justificativa\n- [ ] outro aberto\n"
	got := countREQOpenCriteria(content)
	if got != 2 {
		t.Errorf("countREQOpenCriteria deve retornar 2 (unmet), obteve %d", got)
	}
}

