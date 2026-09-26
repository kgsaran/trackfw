package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Testes do ML-1B (AC7 da REQ-2026-09-09): `roadmap new` escreve OS DOIS lados do elo, e o bloco
// consolidado de "Acceptance Criteria" do caminho --from-req deixa de sair vazio.
//
// Cada teste declara, no próprio comentário, qual conclusão do ML ele afirma (Regra Dura de
// Reconciliação — CLAUDE.md).

// reqFixtureML1B monta uma REQ no formato que `trackfw req new` gera: frontmatter com
// `roadmap: ""` e corpo com o marcador `Roadmap:` vazio — as duas grafias de "órfã" que o
// validate enxerga.
func reqFixtureML1B(criteria ...string) string {
	acs := ""
	for _, c := range criteria {
		acs += "- [ ] " + c + "\n"
	}
	return strings.Join([]string{
		"---",
		"status: Open",
		"date: 2026-09-26",
		`adr: ""`,
		`roadmap: ""`,
		"---",
		"",
		"# REQ: alvo do ml1b",
		"",
		"## Acceptance Criteria",
		acs,
		"## Linked ADR",
		"ADR: ",
		"",
		"## Linked Roadmap",
		"Roadmap: ",
		"",
	}, "\n")
}

func writeREQML1B(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755); err != nil {
		t.Fatalf("mkdir docs/req: %v", err)
	}
	rel := filepath.Join("docs", "req", name)
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile REQ: %v", err)
	}
	return rel
}

func onlyRoadmapML1B(t *testing.T) string {
	t.Helper()
	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil || len(matches) != 1 {
		t.Fatalf("esperado 1 roadmap em backlog, obteve %d: %v", len(matches), err)
	}
	return matches[0]
}

// Reconciliação: afirma a conclusão central do ML-1B — `roadmap new --from-req` escreve o ponteiro
// REQ→roadmap (frontmatter E corpo) no mesmo ato em que cria o roadmap, e é esse valor que o
// `validate` lê em req_has_roadmap. Antes do ML-1B a REQ ficava com `roadmap: ""`.
func TestNewRoadmapFromREQ_WritesBacklinkIntoREQ(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	reqRel := writeREQML1B(t, dir, "REQ-2026-09-26-alvo.md", reqFixtureML1B("AC1 — o vinculo volta"))

	if err := NewRoadmapFromREQ(reqRel, ""); err != nil {
		t.Fatalf("NewRoadmapFromREQ(): %v", err)
	}
	roadmapRel := onlyRoadmapML1B(t)

	reqAfter, err := os.ReadFile(reqRel)
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}
	got := string(reqAfter)

	// Frontmatter: é o campo normativo (ML-1A) e o que req_has_roadmap consulta.
	wantFM := `roadmap: "` + roadmapRel + `"`
	if !strings.Contains(got, wantFM) {
		t.Errorf("frontmatter da REQ deveria conter %q, obteve:\n%s", wantFM, got)
	}
	// Corpo: o marcador estava vazio, logo é preenchível — um corpo que discorda do frontmatter
	// engana o leitor humano (contrato de docs/cli-parity.md).
	if !strings.Contains(got, "Roadmap: "+roadmapRel) {
		t.Errorf("corpo da REQ deveria conter o marcador Roadmap: %q, obteve:\n%s", roadmapRel, got)
	}
	// O caminho gravado tem de resolver no disco — um valor não resolvível satisfaria
	// req_has_roadmap (que aceita qualquer valor não-vazio) e reprovaria ref_targets_exist.
	if _, err := os.Stat(roadmapRel); err != nil {
		t.Errorf("caminho gravado na REQ não resolve no disco: %v", err)
	}
}

// Reconciliação: afirma que o backlink do ML-1B é idempotente — rodar `--from-req` duas vezes sobre
// a mesma REQ não duplica linha nem reescreve byte, porque o predicado de preenchimento aceita o
// mesmo basename e a reescrita resultante é idêntica.
func TestNewRoadmapFromREQ_BacklinkIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	reqRel := writeREQML1B(t, dir, "REQ-2026-09-26-alvo.md", reqFixtureML1B("AC1 — idempotencia"))

	if err := NewRoadmapFromREQ(reqRel, ""); err != nil {
		t.Fatalf("NewRoadmapFromREQ() 1a rodada: %v", err)
	}
	first, err := os.ReadFile(reqRel)
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}

	if err := NewRoadmapFromREQ(reqRel, ""); err != nil {
		t.Fatalf("NewRoadmapFromREQ() 2a rodada: %v", err)
	}
	second, err := os.ReadFile(reqRel)
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}

	// Anti-vacuidade: idempotência é satisfeita trivialmente por "nunca escreve". Exigir que o
	// vínculo esteja presente separa "idempotente" de "inerte" — sem esta linha, desligar o backlink
	// passaria neste teste.
	if !strings.Contains(string(second), `roadmap: "docs/roadmaps/backlog/`) {
		t.Fatalf("idempotência medida sobre REQ sem vínculo — teste vacuoso:\n%s", second)
	}
	if string(first) != string(second) {
		t.Errorf("REQ deveria ser byte-idêntica entre as duas rodadas\nprimeira:\n%s\nsegunda:\n%s", first, second)
	}
	if n := strings.Count(string(second), "roadmap: "); n != 1 {
		t.Errorf("frontmatter deveria ter 1 campo roadmap:, obteve %d", n)
	}
}

// Reconciliação: afirma a segunda conclusão do ML-1B — o bloco consolidado "## Acceptance Criteria"
// do roadmap gerado por --from-req reflete os ACs da REQ, em vez dos dois `- [ ]` vazios que
// obrigaram a reescrever à mão o roadmap desta própria REQ.
func TestNewRoadmapFromREQ_ConsolidatedACsComeFromREQ(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	reqRel := writeREQML1B(t, dir, "REQ-2026-09-26-alvo.md",
		reqFixtureML1B("AC1 — o vinculo volta para a REQ", "AC2 — o bloco de ACs deixa de sair vazio"))

	if err := NewRoadmapFromREQ(reqRel, ""); err != nil {
		t.Fatalf("NewRoadmapFromREQ(): %v", err)
	}
	content, err := os.ReadFile(onlyRoadmapML1B(t))
	if err != nil {
		t.Fatalf("ReadFile roadmap: %v", err)
	}
	body := string(content)

	start := strings.Index(body, "## Acceptance Criteria")
	if start < 0 {
		t.Fatalf("roadmap sem heading consolidado:\n%s", body)
	}
	end := strings.Index(body[start:], "## Status Legend")
	if end < 0 {
		t.Fatalf("roadmap sem Status Legend após o heading:\n%s", body)
	}
	block := body[start : start+end]

	for _, want := range []string{
		"- [ ] AC1 — o vinculo volta para a REQ",
		"- [ ] AC2 — o bloco de ACs deixa de sair vazio",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("bloco consolidado deveria conter %q, obteve:\n%s", want, block)
		}
	}
	// O placeholder vazio não sobrevive ao lado dos critérios reais — se sobrevivesse, o bloco teria
	// itens em branco misturados aos ACs.
	if strings.Contains(block, "- [ ]\n- [ ]") {
		t.Errorf("bloco consolidado ainda contém o placeholder vazio:\n%s", block)
	}
}

// Reconciliação (braço negativo): afirma que a segunda conclusão do ML-1B NÃO inventa conteúdo —
// REQ sem AC nenhum mantém o placeholder anterior e não produz ML derivado algum. É o contra-braço
// que impede "o bloco nunca mais sai vazio" de ser satisfeito fabricando critérios.
func TestNewRoadmapFromREQ_NoCriteriaKeepsPlaceholderAndInventsNoML(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	reqRel := writeREQML1B(t, dir, "REQ-2026-09-26-sem-acs.md", reqFixtureML1B())

	if err := NewRoadmapFromREQ(reqRel, ""); err != nil {
		t.Fatalf("NewRoadmapFromREQ(): %v", err)
	}
	content, err := os.ReadFile(onlyRoadmapML1B(t))
	if err != nil {
		t.Fatalf("ReadFile roadmap: %v", err)
	}
	body := string(content)

	if !strings.Contains(body, "## Acceptance Criteria\n<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->\n- [ ]\n- [ ]\n") {
		t.Errorf("REQ sem ACs deveria manter o bloco placeholder intacto, obteve:\n%s", body)
	}
	if strings.Contains(body, "### ML-1A") {
		t.Errorf("REQ sem ACs não deve produzir ML derivado, obteve:\n%s", body)
	}
}

// Reconciliação: afirma que a correção do ML-1B fecha a causa, não só o sintoma do --from-req — o
// caminho `--req` (NewRoadmapFromContent, template simples) também escreve o backlink, porque o
// elo de mão única era do ponto de criação, não da variante de template.
func TestNewRoadmapFromContent_REQPathAlsoGetsBacklink(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	reqRel := writeREQML1B(t, dir, "REQ-2026-09-26-via-req-flag.md", reqFixtureML1B("AC1 — mesma causa"))

	if err := NewRoadmapFromContent(RoadmapContent{Title: "via req flag", REQPath: reqRel}); err != nil {
		t.Fatalf("NewRoadmapFromContent(): %v", err)
	}
	roadmapRel := onlyRoadmapML1B(t)

	reqAfter, err := os.ReadFile(reqRel)
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}
	if !strings.Contains(string(reqAfter), `roadmap: "`+roadmapRel+`"`) {
		t.Errorf("REQ do caminho --req deveria receber o backlink %q, obteve:\n%s", roadmapRel, reqAfter)
	}
}

// Reconciliação (contra-braço): afirma que o backlink do ML-1B preenche placeholder e NÃO sequestra
// vínculo existente — REQ que já aponta para outro roadmap fica intacta, e a criação do roadmap não
// falha por isso (a não-fatalidade é decisão escrita em linkREQToRoadmap).
func TestNewRoadmapFromREQ_DoesNotOverwriteExistingDifferentLink(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	existing := "docs/roadmaps/wip/ROADMAP-2026-09-01-outro.md"
	fixture := strings.Replace(reqFixtureML1B("AC1 — vinculo preexistente"),
		`roadmap: ""`, `roadmap: "`+existing+`"`, 1)
	reqRel := writeREQML1B(t, dir, "REQ-2026-09-26-ja-vinculada.md", fixture)

	if err := NewRoadmapFromREQ(reqRel, ""); err != nil {
		t.Fatalf("NewRoadmapFromREQ() não deve falhar por vínculo preexistente: %v", err)
	}
	reqAfter, err := os.ReadFile(reqRel)
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}
	if !strings.Contains(string(reqAfter), `roadmap: "`+existing+`"`) {
		t.Errorf("vínculo preexistente deveria permanecer %q, obteve:\n%s", existing, reqAfter)
	}
}

// Reconciliação: afirma a assimetria deliberada dos dois predicados de preenchimento — o corpo da
// REQ é prosa humana, então um valor que não é placeholder reconhecido não é sobrescrito, enquanto o
// frontmatter (escrito por máquina) aceita qualquer valor que não seja uma referência ".md".
func TestReqRoadmapFillablePredicates(t *testing.T) {
	fmCases := map[string]bool{
		"":                                true,
		"none":                            true,
		"-":                               true,
		"<!-- preencher depois -->":       true,
		"docs/roadmaps/wip/ROADMAP-x.md":  false,
		`docs/roadmaps/done/ROADMAP-y.md`: false,
	}
	for val, want := range fmCases {
		if got := reqRoadmapFMIsFillable(val); got != want {
			t.Errorf("reqRoadmapFMIsFillable(%q) = %v, want %v", val, got, want)
		}
	}

	bodyCases := map[string]bool{
		"":                               true,
		"-":                              true,
		"—":                              true,
		"<!-- preencher depois -->":      true,
		"none":                           false, // valor escrito por alguém: não é placeholder reconhecido
		"a decidir depois do ADR":        false, // prosa humana: sobrescrever apagaria conteúdo
		"docs/roadmaps/wip/ROADMAP-x.md": false,
	}
	for val, want := range bodyCases {
		if got := reqRoadmapBodyIsFillable(val); got != want {
			t.Errorf("reqRoadmapBodyIsFillable(%q) = %v, want %v", val, got, want)
		}
	}
}

// Reconciliação: afirma que o backlink do ML-1B sobrevive à forma NÃO-CANÔNICA do caminho da REQ —
// o `--req` absoluto em `/var/folders/...` (que resolve para `/private/var/...` no macOS) é a forma
// que os testes ML-3C exercitam, e sem resolver os symlinks antes do guard de contenção o backlink
// seria abandonado em silêncio enquanto o roadmap continuaria sendo criado.
func TestNewRoadmapFromContent_BacklinkWithNonCanonicalAbsoluteREQPath(t *testing.T) {
	dir := t.TempDir() // em macOS, /var/folders/... — caminho não-canônico (/var → /private/var)
	chdirRoadmap(t, dir)
	reqRel := writeREQML1B(t, dir, "REQ-2026-09-26-abs.md", reqFixtureML1B("AC1 — caminho absoluto"))
	absREQ := filepath.Join(dir, reqRel)

	if err := NewRoadmapFromContent(RoadmapContent{Title: "abs req", REQPath: absREQ}); err != nil {
		t.Fatalf("NewRoadmapFromContent(): %v", err)
	}
	roadmapRel := onlyRoadmapML1B(t)

	reqAfter, err := os.ReadFile(reqRel)
	if err != nil {
		t.Fatalf("ReadFile REQ: %v", err)
	}
	if !strings.Contains(string(reqAfter), `roadmap: "`+roadmapRel+`"`) {
		t.Errorf("REQ apontada por caminho absoluto não-canônico deveria receber o backlink %q, obteve:\n%s",
			roadmapRel, reqAfter)
	}
}
