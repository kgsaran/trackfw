package validator

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// ---------------------------------------------------------------------------
// ML-4B (AC2 + AC3) — corte por data de req_has_roadmap e grandfathering visível.
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara, em uma frase, qual
// conclusão do PRÓPRIO ML-4B ele afirma.
// ---------------------------------------------------------------------------

// writeOrphanREQ grava uma REQ SEM vínculo de roadmap (frontmatter e corpo vazios),
// com controle explícito sobre as duas réguas de data: `fmDate` vai no `date:` do
// frontmatter (vazio = campo ausente) e `name` é o basename (que pode ou não
// carregar data no padrão REQ-YYYY-MM-DD).
// ML-1B: usa status: Done para que req_has_roadmap avalie a REQ — a regra só dispara
// para Done; Open, Superseded e Closed são silenciados.
func writeOrphanREQ(t *testing.T, dir, name, fmDate string) {
	t.Helper()
	fm := "---\nstatus: Done\n"
	if fmDate != "" {
		fm += "date: " + fmDate + "\n"
	}
	fm += "roadmap: \"\"\n---\n"
	body := "\n# REQ: Fixture ML-4B\n\n## Linked Roadmap\nRoadmap: <!-- none -->\n"
	writeFile(t, dir, "docs/req/"+name, fm+body)
}

func validateOrphanFixture(t *testing.T, dir string) (violations, warnings []string) {
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

// TestReqHasRoadmapCutoff_PostCutoffIsError — AFIRMA a conclusão do ML-4B de que REQ
// criada A PARTIR do corte (2026-09-03) sem roadmap é cobrada pela severidade normal
// da regra, que é "error" por default (req_has_roadmap ausente de ruleDefaults).
// Este é o braço 1 dos dois exigidos pelo AC2.
func TestReqHasRoadmapCutoff_PostCutoffIsError(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeOrphanREQ(t, dir, "REQ-2026-09-10-pos-corte.md", "2026-09-10")
	violations, warnings := validateOrphanFixture(t, dir)

	if !hasViolation(violations, "no linked Roadmap") {
		t.Errorf("REQ pós-corte sem roadmap DEVE ser violation, obteve violations=%v", violations)
	}
	if hasWarning(warnings, "exempt as pre-cutoff") {
		t.Errorf("REQ pós-corte NÃO pode ser marcada isenta, obteve warnings=%v", warnings)
	}
}

// TestReqHasRoadmapCutoff_PreCutoffIsWarning — AFIRMA a conclusão do ML-4B de que o
// passivo histórico (REQ anterior a 2026-09-03) sai por warning e NUNCA por violation,
// mesmo com a regra em "error" — é isto que impede o `validate` de falhar de uma vez e
// empurrar o projeto para `governance_mode: lenient`. Braço 2 dos dois do AC2.
func TestReqHasRoadmapCutoff_PreCutoffIsWarning(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeOrphanREQ(t, dir, "REQ-2026-08-20-pre-corte.md", "2026-08-20")
	violations, warnings := validateOrphanFixture(t, dir)

	if hasViolation(violations, "no linked Roadmap") {
		t.Errorf("REQ pré-corte NÃO pode ser violation, obteve violations=%v", violations)
	}
	if !hasWarning(warnings, "no linked Roadmap") || !hasWarning(warnings, "exempt as pre-cutoff") {
		t.Errorf("REQ pré-corte deve sair por warning marcado como isento, obteve warnings=%v", warnings)
	}
}

// TestReqHasRoadmapCutoff_NoticeShowsCountAndCutoff — AFIRMA a conclusão do AC3 de que
// *isenção que não se vê vira permanente*: o relatório carrega uma linha com a
// CONTAGEM de isentas e a DATA do corte, e não apenas os artefatos um a um.
func TestReqHasRoadmapCutoff_NoticeShowsCountAndCutoff(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeOrphanREQ(t, dir, "REQ-2026-08-20-pre-um.md", "2026-08-20")
	writeOrphanREQ(t, dir, "REQ-2026-08-21-pre-dois.md", "2026-08-21")
	_, warnings := validateOrphanFixture(t, dir)

	notice := findNotice(t, warnings)
	for _, want := range []string{"2 REQ(s) without a linked Roadmap exempt", reqRoadmapCutoff, "0 enforced", "2 REQ(s) scanned"} {
		if !strings.Contains(notice, want) {
			t.Errorf("aviso de grandfathering deve conter %q, obteve %q", want, notice)
		}
	}
}

// TestReqHasRoadmapCutoff_NoticeIsNotVacuous — AFIRMA a conclusão do ML-4B de que a
// distinção entre *não há passivo* e *não medi* existe: com ZERO isentas o aviso não diz
// só "0 isentas" — ele traz o denominador (quantas foram varridas) e quantas foram
// cobradas, então "0" nunca é indistinguível de silêncio.
func TestReqHasRoadmapCutoff_NoticeIsNotVacuous(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeOrphanREQ(t, dir, "REQ-2026-09-10-pos-corte.md", "2026-09-10")
	writeREQWithFields(t, dir, "REQ-2026-09-11-vinculada.md", "docs/roadmaps/done/ROADMAP-x.md", "")
	_, warnings := validateOrphanFixture(t, dir)

	notice := findNotice(t, warnings)
	for _, want := range []string{"0 REQ(s) without a linked Roadmap exempt", "1 enforced", "2 REQ(s) scanned"} {
		if !strings.Contains(notice, want) {
			t.Errorf("aviso com zero isentas deve conter %q (denominador), obteve %q", want, notice)
		}
	}
}

// TestReqHasRoadmapCutoff_NoOrphanNoNotice — AFIRMA a ESCOLHA declarada do ML-4B: o
// aviso só é emitido quando a regra achou algo. Sem nenhuma REQ órfã não há isenção a
// tornar visível, e emitir sempre quebraria o contrato de "estrutura limpa = zero ruído"
// (TestValidate_Clean). Contrapartida do teste de vacuidade acima, que garante que
// quando o aviso sai ele nunca é ambíguo.
func TestReqHasRoadmapCutoff_NoOrphanNoNotice(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeREQWithFields(t, dir, "REQ-2026-09-11-vinculada.md", "docs/roadmaps/done/ROADMAP-x.md", "")
	_, warnings := validateOrphanFixture(t, dir)

	for _, w := range warnings {
		if strings.Contains(w, "grandfathering") {
			t.Errorf("sem REQ órfã não deve haver aviso de grandfathering, obteve %q", w)
		}
	}
}

// TestReqHasRoadmapCutoff_FrontmatterDateWinsOverFilename — AFIRMA a régua de data
// medida pelo ML-4B (frontmatter-first, nome do arquivo como fallback), a mesma
// precedência que o ML-1A/ML-1D fixou para o vínculo — e mede as DUAS direções da
// divergência, porque o acervo tem 3 REQs em que as duas réguas discordam.
func TestReqHasRoadmapCutoff_FrontmatterDateWinsOverFilename(t *testing.T) {
	t.Run("nome_antigo_frontmatter_novo_e_cobrada", func(t *testing.T) {
		dir := buildReqRoadmapDir(t)
		writeOrphanREQ(t, dir, "REQ-2026-08-01-nome-antigo.md", "2026-09-20")
		violations, _ := validateOrphanFixture(t, dir)
		if !hasViolation(violations, "no linked Roadmap") {
			t.Errorf("frontmatter pós-corte deve vencer nome pré-corte (cobrada), obteve violations=%v", violations)
		}
	})
	t.Run("nome_novo_frontmatter_antigo_e_isenta", func(t *testing.T) {
		dir := buildReqRoadmapDir(t)
		writeOrphanREQ(t, dir, "REQ-2026-09-20-nome-novo.md", "2026-08-01")
		violations, warnings := validateOrphanFixture(t, dir)
		if hasViolation(violations, "no linked Roadmap") {
			t.Errorf("frontmatter pré-corte deve vencer nome pós-corte (isenta), obteve violations=%v", violations)
		}
		if !hasWarning(warnings, "exempt as pre-cutoff") {
			t.Errorf("esperado warning de isenção, obteve warnings=%v", warnings)
		}
	})
	t.Run("sem_frontmatter_cai_no_nome", func(t *testing.T) {
		dir := buildReqRoadmapDir(t)
		writeOrphanREQ(t, dir, "REQ-2026-08-01-so-nome.md", "")
		violations, warnings := validateOrphanFixture(t, dir)
		if hasViolation(violations, "no linked Roadmap") {
			t.Errorf("sem `date:` a data do nome (pré-corte) deve isentar, obteve violations=%v", violations)
		}
		if !hasWarning(warnings, "exempt as pre-cutoff") {
			t.Errorf("esperado warning de isenção pelo nome, obteve warnings=%v", warnings)
		}
	})
}

// TestReqHasRoadmapCutoff_UnknownDateFailsClosed — AFIRMA a conclusão do ML-4B de que
// data ilegível nas duas réguas é tratada como PÓS-corte: aceitar "sem data" como
// anistia abriria o bypass de apagar o `date:` e renomear o arquivo. Medido no acervo:
// zero REQs caem neste ramo hoje, logo fail-closed não cobra ninguém retroativamente.
func TestReqHasRoadmapCutoff_UnknownDateFailsClosed(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeOrphanREQ(t, dir, "REQ-sem-data-nenhuma.md", "")
	violations, _ := validateOrphanFixture(t, dir)
	if !hasViolation(violations, "no linked Roadmap") {
		t.Errorf("REQ sem data legível deve ser cobrada (fail closed), obteve violations=%v", violations)
	}
}

// TestReqHasRoadmapCutoff_OffSilencesBothArms — AFIRMA que o corte NÃO cria um canal
// que sobrevive ao desligamento da regra: com `rules: {req_has_roadmap: off}`, o braço
// isento e o aviso de contagem ficam silenciosos junto com o braço cobrado.
func TestReqHasRoadmapCutoff_OffSilencesBothArms(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "trackfw.yaml", "rules:\n  req_has_roadmap: off\n")
	writeOrphanREQ(t, dir, "REQ-2026-08-20-pre-corte.md", "2026-08-20")
	writeOrphanREQ(t, dir, "REQ-2026-09-10-pos-corte.md", "2026-09-10")
	violations, warnings := validateOrphanFixture(t, dir)

	if hasViolation(violations, "no linked Roadmap") {
		t.Errorf("com off não deve haver violation, obteve %v", violations)
	}
	for _, w := range warnings {
		if strings.Contains(w, "no linked Roadmap") || strings.Contains(w, "grandfathering") {
			t.Errorf("com off não deve haver warning de req_has_roadmap, obteve %q", w)
		}
	}
}

// TestReqHasRoadmapCutoff_ML1DReaderPreserved — AFIRMA que o ML-4B NÃO desfez o ML-1D:
// o veredito continua vindo de contentHasStructuredRefValue (exige caminho ".md",
// frontmatter-first), então placeholder segue acusado — o corte muda o CANAL da
// mensagem, nunca o critério de "está vinculada".
func TestReqHasRoadmapCutoff_ML1DReaderPreserved(t *testing.T) {
	dir := buildReqRoadmapDir(t)
	// ML-1B: status: Done para que a regra avalie a REQ; date pré-corte para testar o braço de isenção.
	writeFile(t, dir, "docs/req/REQ-2026-08-20-placeholder.md",
		"---\nstatus: Done\ndate: 2026-08-20\nroadmap: \"none\"\n---\n\n# REQ\n\n## Linked Roadmap\nRoadmap: none\n")
	violations, warnings := validateOrphanFixture(t, dir)

	if hasViolation(violations, "no linked Roadmap") {
		t.Errorf("placeholder pré-corte deve sair por warning, não violation: %v", violations)
	}
	if !hasWarning(warnings, "no linked Roadmap") {
		t.Errorf("placeholder DEVE continuar acusado pelo leitor do ML-1D, obteve warnings=%v", warnings)
	}
}

// TestReqHasRoadmapCutoff_ConstanteParseavel — AFIRMA que o corte é DECLARADO no
// artefato e legível: o literal da constante parseia como data, e o fallback de
// reqRoadmapCutoffDate (zero de time.Time em caso de literal corrompido) não é
// silencioso — ele torna tudo pós-corte, que é o lado estrito.
func TestReqHasRoadmapCutoff_ConstanteParseavel(t *testing.T) {
	d := reqRoadmapCutoffDate()
	if d.IsZero() {
		t.Fatalf("constante reqRoadmapCutoff=%q não parseia como 2006-01-02", reqRoadmapCutoff)
	}
	if d.Format("2006-01-02") != reqRoadmapCutoff {
		t.Errorf("round-trip da constante divergiu: %q != %q", d.Format("2006-01-02"), reqRoadmapCutoff)
	}
}

func findNotice(t *testing.T, warnings []string) string {
	t.Helper()
	for _, w := range warnings {
		if strings.Contains(w, "req_has_roadmap grandfathering:") {
			return w
		}
	}
	t.Fatalf("aviso de grandfathering ausente dos warnings: %v", warnings)
	return ""
}
