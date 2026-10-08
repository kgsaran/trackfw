package validator

// validator_req_done_criteria_test.go — D4 (ADR-2026-10-04, REQ #514 ML-1B/ML-1C/ML-1A).
//
// Reconciliação obrigatória (CLAUDE.md): cada teste declara, em uma frase, qual
// conclusão do próprio ML-1B/ML-1C/ML-1A ele afirma.
//
// Sabotagens cobertas (ML-1B/1C):
//   S1 (cutoff < → <=): derruba TestReqDoneOpenCriteria_PostCutoffIsWarning e
//      TestReqDoneOpenCriteria_EnforcedCountInNotice.
//   S2 (Unmet+Lapsed em vez de Unmet): derruba TestReqDoneOpenCriteria_LapsedDoesNotFire
//      e TestReqDoneOpenCriteria_SectionScanCountsOnlyUnmet.
//
// Sabotagens cobertas (ML-1A — upstream inheritance):
//   S3 (remover T1): derruba TestReqDoneOpenCriteria_AC4b_T1_UpstreamEqualsOrigin.
//   S4 (path match em vez de basename): derruba TestReqDoneOpenCriteria_AC4a_InheritedByBasename.
//
// Sabotagem coberta (ML-2B — sanitização de shortRef):
//   S5 (remover allowlist shortRefSafe): derruba TestReqDoneOpenCriteria_DoubleQuoteBranchName
//      (a parentética exibiria `"evil"` e File no JSON viria "evil" em vez de "").

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// ---------------------------------------------------------------------------
// Helpers para testes de herança upstream (ML-1A, issue #542, REQ-2026-10-08)
// ---------------------------------------------------------------------------

// validateTaggedFixture executa ValidateTagged no diretório dir (site B — caminho do CLI).
// Usado pelos novos testes de herança: sítio A (ValidateUnfiltered) prova que baseline
// propaga o recorte; sítio B prova que `trackfw validate` (e --json) o faz.
func validateTaggedFixture(t *testing.T, dir string) (violations, warnings []TaggedMsg) {
	t.Helper()
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)
	v, w, err := ValidateTagged()
	if err != nil {
		t.Fatalf("ValidateTagged() erro: %v", err)
	}
	return v, w
}

// findD4NoticeFromTagged localiza a linha de aviso req_done_open_criteria: nos warnings tagged.
func findD4NoticeFromTagged(t *testing.T, warnings []TaggedMsg) string {
	t.Helper()
	for _, w := range warnings {
		if strings.Contains(w.Msg, reqDoneOpenCriteriaNoticeSubstr) {
			return w.Msg
		}
	}
	t.Fatalf("aviso req_done_open_criteria ausente dos warnings tagged: %v", warnings)
	return ""
}

// writeDoneREQWithOpenCriteriaIn é como writeDoneREQWithOpenCriteria mas escreve em reqSubdir
// em vez de docs/req/. Usado para forks com req_dir personalizado (ex: docs/requisições/).
func writeDoneREQWithOpenCriteriaIn(t *testing.T, dir, reqSubdir, name, fmDate string) {
	t.Helper()
	fm := "---\nstatus: Done\n"
	if fmDate != "" {
		fm += "date: " + fmDate + "\n"
	}
	fm += "roadmap: \"docs/roadmaps/done/ROADMAP-x.md\"\n---\n"
	body := "\n# REQ: Fixture D4 upstream\n\n## Acceptance Criteria\n\n- [x] Critério atendido\n- [ ] Critério aberto sem Caducou:\n\n## Linked Roadmap\nRoadmap: docs/roadmaps/done/ROADMAP-x.md\n"
	writeFile(t, dir, reqSubdir+"/"+name, fm+body)
}

// initUpstreamForInheritance cria um repositório git em upstreamDir com os arquivos
// indicados commitados no ramo main, e adiciona-o como remote "upstream" de forkDir com fetch.
// Requer git >= 2.28 (git init -b main); confirmado na versão 2.54.0 do ambiente.
func initUpstreamForInheritance(t *testing.T, forkDir, upstreamDir string, files map[string]string) {
	t.Helper()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %s", args, dir, out)
		}
	}

	runIn(upstreamDir, "init", "-b", "main")
	runIn(upstreamDir, "config", "user.email", "test@test.com")
	runIn(upstreamDir, "config", "user.name", "test")
	runIn(upstreamDir, "config", "commit.gpgsign", "false")

	for rel, content := range files {
		path := filepath.Join(upstreamDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writeFile %s: %v", path, err)
		}
		runIn(upstreamDir, "add", rel)
	}
	runIn(upstreamDir, "commit", "--allow-empty", "-m", "init")

	runIn(forkDir, "remote", "add", "upstream", upstreamDir)
	runIn(forkDir, "fetch", "upstream")
}

// buildForkDirWithReqDir cria um diretório de fixture com req_dir personalizado e git init.
// Usado pelos testes de herança upstream onde o fork tem req_dir diferente do upstream.
func buildForkDirWithReqDir(t *testing.T, reqDir string) string {
	t.Helper()
	dir := t.TempDir()
	mkdirs(t, dir,
		reqDir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/adr",
	)
	writeFile(t, dir, "trackfw.yaml", "req_dir: "+reqDir+"\n")
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	initGitRepo(t, dir, "main")
	return dir
}

// ---------------------------------------------------------------------------
// Testes de herança upstream (ML-1A, issue #542, REQ-2026-10-08)
// ---------------------------------------------------------------------------

// wantD4NoticeAC3 é o texto literal esperado quando não há upstream — byte-idêntico
// ao formato anterior ao ML-1A. Hard-coded para não ser tautológico após o refactor.
// Sabotagem: inserir "inherited" aqui quebraria o controle de AC3.
const wantD4NoticeAC3Prefix = "req_done_open_criteria: 2 Done REQ(s) with open criteria exempt as created before cutoff 2026-10-04, 0 enforced, 2 Done REQ(s) scanned (cutoff declared in internal/validator/validator_req_done_criteria.go)"

// TestReqDoneOpenCriteria_AC3_NoUpstreamByteIdentical — AFIRMA que sem remote upstream,
// o aviso agregado é byte-idêntico ao formato anterior ao ML-1A (sem parentética).
// Sítio B (ValidateTagged) — caminho do CLI.
func TestReqDoneOpenCriteria_AC3_NoUpstreamByteIdentical(t *testing.T) {
	// buildReqRoadmapDir não inicializa git — simula repositório sem upstream.
	dir := buildReqRoadmapDir(t)
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-03-a.md", "2026-10-03")
	writeDoneREQWithOpenCriteria(t, dir, "REQ-2026-10-03-b.md", "2026-10-03")

	_, warnings := validateTaggedFixture(t, dir)
	notice := findD4NoticeFromTagged(t, warnings)

	if notice != wantD4NoticeAC3Prefix {
		t.Errorf("AC3: aviso deve ser byte-idêntico ao formato pré-ML-1A.\nquer: %q\nobteve: %q", wantD4NoticeAC3Prefix, notice)
	}
	if strings.Contains(notice, "inherited") || strings.Contains(notice, "(upstream") {
		t.Errorf("AC3: aviso NÃO deve conter parentética de herança, obteve: %q", notice)
	}
}

// TestReqDoneOpenCriteria_AC4a_InheritedByBasename — AFIRMA que o discriminante de herança
// usa basename (não caminho completo): fork com req_dir diferente do upstream ainda conta
// K=2 para REQs com mesmo basename. Falsificação: trocar basename por path → K=0, teste reprova.
// Sítio B (ValidateTagged) — caminho do CLI.
func TestReqDoneOpenCriteria_AC4a_InheritedByBasename(t *testing.T) {
	// Fork com req_dir: docs/requisições (diferente do upstream que usa docs/req).
	forkDir := buildForkDirWithReqDir(t, "docs/requisições")

	// Upstream tem 2 REQs em docs/req/ — req_dir padrão.
	upstreamDir := t.TempDir()
	initUpstreamForInheritance(t, forkDir, upstreamDir, map[string]string{
		"docs/req/REQ-2026-10-03-up1.md": "# upstream req 1\n",
		"docs/req/REQ-2026-10-03-up2.md": "# upstream req 2\n",
	})

	// Fork tem 3 REQs em docs/requisições/:
	// - up1 e up2 com mesmo basename do upstream (herdadas)
	// - local com basename único (não herdada)
	writeDoneREQWithOpenCriteriaIn(t, forkDir, "docs/requisições", "REQ-2026-10-03-up1.md", "2026-10-03")
	writeDoneREQWithOpenCriteriaIn(t, forkDir, "docs/requisições", "REQ-2026-10-03-up2.md", "2026-10-03")
	writeDoneREQWithOpenCriteriaIn(t, forkDir, "docs/requisições", "REQ-2026-10-03-local.md", "2026-10-03")

	_, warnings := validateTaggedFixture(t, forkDir)
	notice := findD4NoticeFromTagged(t, warnings)

	for _, want := range []string{
		"3 Done REQ(s) with open criteria exempt",
		"(2 inherited from upstream/main)",
		"0 enforced",
		"3 Done REQ(s) scanned",
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("AC4a: notice deve conter %q, obteve: %q", want, notice)
		}
	}
	// A REQ local (basename único) NÃO deve ser contada em K.
	if strings.Contains(notice, "(3 inherited") {
		t.Errorf("AC4a: REQ local NÃO deve ser contada como herdada, obteve: %q", notice)
	}
}

// TestReqDoneOpenCriteria_AC4b_T1_UpstreamEqualsOrigin — AFIRMA que quando upstream.url ==
// origin.url a parentética é suprimida (T1 guard). Falsificação: remover T1 → upstream/main
// existe e K > 0 → notice teria "(2 inherited...)" → teste reprova.
// Sítio B (ValidateTagged) — caminho do CLI.
func TestReqDoneOpenCriteria_AC4b_T1_UpstreamEqualsOrigin(t *testing.T) {
	// Fork usa docs/req (padrão).
	forkDir := buildReqRoadmapDir(t)
	writeFile(t, forkDir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	initGitRepo(t, forkDir, "main")

	// Upstream tem REQs com mesmo basename dos locais.
	upstreamDir := t.TempDir()
	initUpstreamForInheritance(t, forkDir, upstreamDir, map[string]string{
		"docs/req/REQ-2026-10-03-a.md": "# upstream req a\n",
		"docs/req/REQ-2026-10-03-b.md": "# upstream req b\n",
	})

	// Adiciona origin apontando para o mesmo upstream → T1.
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = forkDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	runGit("remote", "add", "origin", upstreamDir)

	// Fork REQs com mesmo basename do upstream (mas T1 deve suprimir K).
	writeDoneREQWithOpenCriteria(t, forkDir, "REQ-2026-10-03-a.md", "2026-10-03")
	writeDoneREQWithOpenCriteria(t, forkDir, "REQ-2026-10-03-b.md", "2026-10-03")

	_, warnings := validateTaggedFixture(t, forkDir)
	notice := findD4NoticeFromTagged(t, warnings)

	if strings.Contains(notice, "inherited") || strings.Contains(notice, "(upstream") {
		t.Errorf("AC4b T1: upstream==origin deve suprimir parentética, obteve: %q", notice)
	}
}

// TestReqDoneOpenCriteria_AC4c_InheritedPostCutoffInEnforced — AFIRMA que REQ herdada
// pós-cutoff fica em enforced (não em exempt) e não é contada em K.
// K é subconjunto de exempt: só REQs isentas podem ser herdadas no recorte.
// Sítio B (ValidateTagged) — caminho do CLI.
func TestReqDoneOpenCriteria_AC4c_InheritedPostCutoffInEnforced(t *testing.T) {
	forkDir := buildReqRoadmapDir(t)
	writeFile(t, forkDir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	initGitRepo(t, forkDir, "main")

	upstreamDir := t.TempDir()
	// Upstream tem 2 REQs: uma pré-cutoff e uma pós-cutoff (2026-11-01).
	initUpstreamForInheritance(t, forkDir, upstreamDir, map[string]string{
		"docs/req/REQ-2026-10-03-pre.md":  "# upstream pre-cutoff\n",
		"docs/req/REQ-2026-11-01-post.md": "# upstream post-cutoff\n",
	})

	// Fork: REQ pré-cutoff (isenta → K conta); REQ pós-cutoff (enforced → K não conta).
	writeDoneREQWithOpenCriteria(t, forkDir, "REQ-2026-10-03-pre.md", "2026-10-03")
	writeDoneREQWithOpenCriteria(t, forkDir, "REQ-2026-11-01-post.md", "2026-11-01")

	_, warnings := validateTaggedFixture(t, forkDir)
	notice := findD4NoticeFromTagged(t, warnings)

	// K=1 (só a pré-cutoff é inherited e isenta); enforced=1 (a pós-cutoff dispara).
	for _, want := range []string{
		"1 Done REQ(s) with open criteria exempt",
		"(1 inherited from upstream/main)",
		"1 enforced",
		"2 Done REQ(s) scanned",
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("AC4c: notice deve conter %q, obteve: %q", want, notice)
		}
	}
}

// TestReqDoneOpenCriteria_UpstreamRefUnresolvable — AFIRMA que quando upstream está
// configurado mas nenhuma ref (main, master, HEAD) resolve, a parentética indica
// explicitamente que as refs foram tentadas mas não resolveram.
// Sítio B (ValidateTagged) — caminho do CLI.
func TestReqDoneOpenCriteria_UpstreamRefUnresolvable(t *testing.T) {
	forkDir := buildReqRoadmapDir(t)
	writeFile(t, forkDir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	initGitRepo(t, forkDir, "main")

	// Adiciona remote "upstream" mas NÃO faz fetch → refs/remotes/upstream/* inexistentes.
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = forkDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	// Usa um path que existe para que "git remote add" não reclame.
	runGit("remote", "add", "upstream", t.TempDir())
	// Adiciona origin diferente do upstream para não disparar T1.
	runGit("remote", "add", "origin", "https://example.invalid/origin.git")

	writeDoneREQWithOpenCriteria(t, forkDir, "REQ-2026-10-03-x.md", "2026-10-03")

	_, warnings := validateTaggedFixture(t, forkDir)
	notice := findD4NoticeFromTagged(t, warnings)

	if !strings.Contains(notice, "(upstream tried main, master: ref unresolvable)") {
		t.Errorf("ref unresolvable: notice deve conter a variante de irresolvível, obteve: %q", notice)
	}
}

// ---------------------------------------------------------------------------
// Testes de ML-2B — sanitização de shortRef (F1 do red-team)
// ---------------------------------------------------------------------------

// TestReqDoneOpenCriteria_DoubleQuoteBranchName — AFIRMA que um nome de ramo com `"`
// (fora da allowlist shortRefSafe) é tratado como irresolvível: a parentética contém
// a variante unresolvable e não contém `"`, e o campo File do RuleItem JSON fica vazio.
// Falsificação: remover shortRefSafe em upstreamInheritedInfo → shortRef cru `upstream/main"evil"`
// é usado na parentética → extractFile extrai "evil" → File = "evil" ≠ "" → teste reprova.
func TestReqDoneOpenCriteria_DoubleQuoteBranchName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("NTFS não permite `\"` em nomes de arquivo; ataque não é montável nessa plataforma")
	}

	forkDir := buildReqRoadmapDir(t)
	writeFile(t, forkDir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	initGitRepo(t, forkDir, "main")

	// Adiciona remote upstream sem fazer fetch (main/master refs ausentes).
	// origin ausente não dispara T1 (guard só quando err == nil).
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = forkDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	runGit("remote", "add", "upstream", "https://example.invalid/upstream.git")

	// Obtém o hash do commit inicial do fork para usar no ref file.
	hashOut, err := exec.Command("git", "-C", forkDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	commitHash := strings.TrimRight(string(hashOut), "\n")

	// Escreve os ref files diretamente (git aceita `"` em nomes de ramo: exit=0).
	refDir := filepath.Join(forkDir, ".git", "refs", "remotes", "upstream")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// refs/remotes/upstream/main"evil" → hash real
	if err := os.WriteFile(filepath.Join(refDir, `main"evil"`), []byte(commitHash+"\n"), 0o644); err != nil {
		t.Fatalf("write ref main\"evil\": %v", err)
	}
	// refs/remotes/upstream/HEAD → symref para main"evil"
	headContent := "ref: refs/remotes/upstream/main\"evil\"\n"
	if err := os.WriteFile(filepath.Join(refDir, "HEAD"), []byte(headContent), 0o644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	// Pré-condição 1: HEAD deve resolver como commit (arma o ataque).
	verifyOut, verifyErr := exec.Command("git", "-C", forkDir, "rev-parse", "--verify", "--quiet",
		"refs/remotes/upstream/HEAD^{commit}").Output()
	if verifyErr != nil || strings.TrimSpace(string(verifyOut)) == "" {
		t.Fatalf("pré-condição: refs/remotes/upstream/HEAD^{commit} não resolveu — ataque não armado: %v", verifyErr)
	}

	// Pré-condição 2: symbolic-ref --short deve retornar exatamente upstream/main"evil".
	symrefOut, symrefErr := exec.Command("git", "-C", forkDir, "symbolic-ref", "--short",
		"refs/remotes/upstream/HEAD").Output()
	if symrefErr != nil {
		t.Fatalf("pré-condição: symbolic-ref --short falhou: %v", symrefErr)
	}
	gotShort := strings.TrimRight(string(symrefOut), "\n")
	wantShort := `upstream/main"evil"`
	if gotShort != wantShort {
		t.Fatalf("pré-condição: symbolic-ref retornou %q, esperado %q — ataque não armado", gotShort, wantShort)
	}

	writeDoneREQWithOpenCriteria(t, forkDir, "REQ-2026-10-03-x.md", "2026-10-03")

	violations, warnings := validateTaggedFixture(t, forkDir)
	result := BuildResultTagged(violations, warnings, false)

	// Localiza o RuleItem da linha D4 nos warnings JSON.
	var d4Item *RuleItem
	for i := range result.Warnings {
		if strings.Contains(result.Warnings[i].Message, reqDoneOpenCriteriaNoticeSubstr) {
			d4Item = &result.Warnings[i]
			break
		}
	}
	if d4Item == nil {
		t.Fatalf("RuleItem req_done_open_criteria ausente dos warnings: %+v", result.Warnings)
	}

	// A parentética deve ser a variante unresolvable (allowlist rejeitou o nome com `"`).
	if !strings.Contains(d4Item.Message, "(upstream tried main, master: ref unresolvable)") {
		t.Errorf("DoubleQuoteBranch: esperava variante unresolvable, obteve: %q", d4Item.Message)
	}
	// A mensagem não deve conter aspas duplas na parentética.
	if strings.Contains(d4Item.Message, `"evil"`) {
		t.Errorf("DoubleQuoteBranch: mensagem contém `\"evil\"` — allowlist não aplicada: %q", d4Item.Message)
	}
	// O campo File JSON deve ser vazio (nenhum caminho falso extraído).
	if d4Item.File != "" {
		t.Errorf("DoubleQuoteBranch: File deve ser \"\", obteve %q", d4Item.File)
	}
}

// TestReqDoneOpenCriteria_SpecialBranchNamePassesAllowlist — AFIRMA que a allowlist
// não rejeita nomes legítimos com `/` e `-`: `feature/special-branch` continua aparecendo
// na parentética via step 4 (HEAD fallback, quando main/master ausentes).
// Falsificação: remover `/` da allowlist → shortRefSafe rejeita `upstream/feature/special-branch`
// → parentética vira unresolvable → teste reprova.
func TestReqDoneOpenCriteria_SpecialBranchNamePassesAllowlist(t *testing.T) {
	// Fork com docs/req (padrão).
	forkDir := buildReqRoadmapDir(t)
	writeFile(t, forkDir, "docs/roadmaps/done/ROADMAP-x.md", "# Fixture\n")
	initGitRepo(t, forkDir, "main")

	// Upstream criado com ramo feature/special-branch (sem main nem master).
	upstreamDir := t.TempDir()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %s", args, dir, out)
		}
	}
	runIn(upstreamDir, "init", "-b", "feature/special-branch")
	runIn(upstreamDir, "config", "user.email", "test@test.com")
	runIn(upstreamDir, "config", "user.name", "test")
	runIn(upstreamDir, "config", "commit.gpgsign", "false")
	// Upstream tem uma REQ com mesmo basename da REQ local do fork.
	reqPath := filepath.Join(upstreamDir, "docs", "req", "REQ-2026-10-03-shared.md")
	if err := os.MkdirAll(filepath.Dir(reqPath), 0o755); err != nil {
		t.Fatalf("mkdir upstream req: %v", err)
	}
	if err := os.WriteFile(reqPath, []byte("# upstream req\n"), 0o644); err != nil {
		t.Fatalf("write upstream req: %v", err)
	}
	runIn(upstreamDir, "add", "docs/req/REQ-2026-10-03-shared.md")
	runIn(upstreamDir, "commit", "--allow-empty", "-m", "init")

	// Adiciona upstream ao fork e faz fetch.
	runIn(forkDir, "remote", "add", "upstream", upstreamDir)
	runIn(forkDir, "fetch", "upstream")

	// Garante que HEAD aponta para feature/special-branch.
	runIn(forkDir, "remote", "set-head", "upstream", "feature/special-branch")

	// Pré-condição: refs/remotes/upstream/main não deve existir (garante step 4).
	if _, err := exec.Command("git", "-C", forkDir, "rev-parse", "--verify", "--quiet",
		"refs/remotes/upstream/main^{commit}").Output(); err == nil {
		t.Fatal("pré-condição: refs/remotes/upstream/main existe — step 3 vai curto-circuitar o teste")
	}

	// REQ local com mesmo basename do upstream (isenta: pré-cutoff).
	writeDoneREQWithOpenCriteria(t, forkDir, "REQ-2026-10-03-shared.md", "2026-10-03")

	_, warnings := validateTaggedFixture(t, forkDir)
	notice := findD4NoticeFromTagged(t, warnings)

	if !strings.Contains(notice, "(1 inherited from upstream/feature/special-branch)") {
		t.Errorf("SpecialBranch: notice deve conter \"(1 inherited from upstream/feature/special-branch)\", obteve: %q", notice)
	}
}

