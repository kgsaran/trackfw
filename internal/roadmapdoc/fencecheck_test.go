package roadmapdoc

// fencecheck_test.go — tests for FenceMaskCheck, the fail-closed predicates,
// and the acervo gate TestAcervoSemCercaAberta.
//
// RECONCILIAÇÃO (Regra Dura — todo teste novo declara o que afirma):
//
//   TestFenceMaskCheck_FechadaRetornaZeroNil afirma que um documento bem-formado (cerca
//   fechada) produz (0, nil) — o sinal de "tudo ok" para FenceMaskCheck.
//
//   TestFenceMaskCheck_AbertaComCraseRetornaLinha afirma que uma cerca de 3 crases nunca
//   fechada produz o número de linha do abridor, não 0.
//
//   TestFenceMaskCheck_AbertaComTilRetornaLinha afirma que o mesmo vale para uma cerca
//   de 3 tils — a gramática cobre ambos os caracteres.
//
//   TestFenceMaskCheck_FechadorMaisCurtoNaoFecha afirma que um fechador com MENOS crases
//   que o abridor NÃO fecha a cerca (o padrão CommonMark exige length >= fenceLen).
//
//   TestFenceMaskCheck_FechadorDeOutroCharNaoFecha afirma que um fechador com caractere
//   diferente do abridor NÃO fecha a cerca.
//
//   TestFenceMaskCheck_FixtureCongeladaArquivo1Corpus afirma que a fixture congelada do
//   corpus interno (cópia 1 do ROADMAP-2026-08-22) retorna linha 460 — o sinal de
//   regressão: se o scanner mudar, este número muda primeiro.
//
//   TestFenceMaskCheck_FixtureCongeladaArquivoSnapshot afirma o mesmo para a cópia
//   congelada em scripts/testdata (mesmos bytes, verificação redundante intencional).
//
//   TestFenceMaskCheck_FixtureCongeladaArquivo2 afirma que a fixture congelada do
//   ROADMAP-2026-08-29 retorna linha 1044.
//
//   TestWave0GateDiagnosis_FailClosedComCercaAberta afirma que Wave0GateDiagnosis retorna
//   valor não-OK para um documento com cerca aberta — o documento poderia ter Wave 0
//   completa logo antes da cerca, e sem o guard o diagnóstico emitiria Wave0GateOK.
//
//   TestWave0GateDiagnosis_OKComDocFechado afirma que o comportamento do diagnóstico NÃO
//   muda para um documento bem-formado (o guard não cria falso positivo).
//
//   TestHasAnyNonPendingML_FailClosedComCercaAberta afirma que hasAnyNonPendingML retorna
//   true para um documento com cerca aberta que mascararia MLs pendentes.
//
//   TestHasAnyNonPendingML_OKComDocFechado afirma que hasAnyNonPendingML retorna false
//   para um documento bem-formado com todos MLs pendentes.
//
//   TestHasWave0_FailClosedComCercaAberta afirma que HasWave0 retorna false para um
//   documento com cerca aberta — mesmo que Wave 0 esteja presente antes da cerca.
//
//   TestHasWave0_OKComDocFechado afirma que HasWave0 retorna true para o mesmo documento
//   com a cerca fechada.
//
//   TestHasUnfinishedMLs_FailClosedComCercaAberta afirma que HasUnfinishedMLs retorna true
//   para um documento com cerca aberta — MLs pendentes na cauda mascarada seriam invisíveis.
//
//   TestHasUnfinishedMLs_OKComDocFechado afirma que HasUnfinishedMLs retorna false para
//   o mesmo documento com a cerca fechada e todos MLs concluídos.
//
//   TestAcervoSemCercaAberta afirma que nenhum arquivo em docs/roadmaps/**/*.md tem cerca
//   não terminada — é o gate de acervo das Waves 1 e 4; o nome é contrato.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// FenceMaskCheck — unit tests
// ────────────────────────────────────────────────────────────────────────────

func TestFenceMaskCheck_FechadaRetornaZeroNil(t *testing.T) {
	doc := "# ROADMAP\n\n```\ncódigo aqui\n```\n\nprosa depois\n"
	lines := SplitRoadmapLines(doc)
	n, err := FenceMaskCheck(lines)
	if n != 0 || err != nil {
		t.Fatalf("FenceMaskCheck(fechada) = (%d, %v), quer (0, nil)", n, err)
	}
}

func TestFenceMaskCheck_AbertaComCraseRetornaLinha(t *testing.T) {
	// Cerca de 3 crases que nunca fecha.
	doc := "linha 1\nlinha 2\n```\ncódigo sem fechar\n"
	lines := SplitRoadmapLines(doc)
	n, err := FenceMaskCheck(lines)
	if n != 3 {
		t.Fatalf("FenceMaskCheck(crase aberta) linha = %d, quer 3", n)
	}
	if err == nil {
		t.Fatal("FenceMaskCheck(crase aberta) err = nil, quer erro")
	}
	want := "unterminated code fence starting at line 3"
	if err.Error() != want {
		t.Fatalf("FenceMaskCheck err = %q, quer %q", err.Error(), want)
	}
}

func TestFenceMaskCheck_AbertaComTilRetornaLinha(t *testing.T) {
	// Cerca de 3 tils que nunca fecha.
	doc := "prosa\n~~~\ncódigo sem fechar\n"
	lines := SplitRoadmapLines(doc)
	n, err := FenceMaskCheck(lines)
	if n != 2 {
		t.Fatalf("FenceMaskCheck(til aberto) linha = %d, quer 2", n)
	}
	if err == nil {
		t.Fatal("FenceMaskCheck(til aberto) err = nil, quer erro")
	}
}

func TestFenceMaskCheck_FechadorMaisCurtoNaoFecha(t *testing.T) {
	// Abridor com 4 crases, tentativa de fechar com 3 — não deve fechar.
	doc := "prosa\n````\ncódigo\n```\nstill inside\n"
	lines := SplitRoadmapLines(doc)
	n, err := FenceMaskCheck(lines)
	if n == 0 || err == nil {
		t.Fatal("FenceMaskCheck(fechador mais curto) retornou (0,nil), mas a cerca não fecha")
	}
	if n != 2 {
		t.Fatalf("FenceMaskCheck(fechador mais curto) linha = %d, quer 2", n)
	}
}

func TestFenceMaskCheck_FechadorDeOutroCharNaoFecha(t *testing.T) {
	// Abridor com crases, tentativa de fechar com tils — não deve fechar.
	doc := "prosa\n```\ncódigo\n~~~\nstill inside\n"
	lines := SplitRoadmapLines(doc)
	n, err := FenceMaskCheck(lines)
	if n == 0 || err == nil {
		t.Fatal("FenceMaskCheck(fechador outro char) retornou (0,nil), mas a cerca não fecha")
	}
	if n != 2 {
		t.Fatalf("FenceMaskCheck(fechador outro char) linha = %d, quer 2", n)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// FenceMaskCheck — frozen fixture tests (regressão)
// ────────────────────────────────────────────────────────────────────────────

func TestFenceMaskCheck_FixtureCongeladaArquivo1Corpus(t *testing.T) {
	// A cópia congelada do ROADMAP-2026-08-22 no corpus interno tem cerca aberta
	// na linha 460. Este teste é o pino de regressão: se o scanner ou a fixture
	// mudarem, o número muda antes de qualquer outra coisa.
	path := filepath.Join("testdata", "corpus", "docs", "roadmaps", "done",
		"ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture 1 corpus: %v", err)
	}
	lines := SplitRoadmapLines(string(data))
	n, fErr := FenceMaskCheck(lines)
	if n != 460 {
		t.Errorf("FenceMaskCheck(fixture1-corpus) linha = %d, quer 460", n)
	}
	if fErr == nil {
		t.Error("FenceMaskCheck(fixture1-corpus) err = nil, quer erro")
	}
	if fErr != nil && !strings.Contains(fErr.Error(), "460") {
		t.Errorf("FenceMaskCheck(fixture1-corpus) err = %q, deve conter '460'", fErr.Error())
	}
}

func TestFenceMaskCheck_FixtureCongeladaArquivoSnapshot(t *testing.T) {
	// A cópia congelada do ROADMAP-2026-08-22 no snapshot de scripts/testdata.
	// Mesmos bytes; verificação redundante intencional — confirma que as duas
	// cópias não divergiram.
	wd, _ := os.Getwd()
	path := filepath.Join(wd, "..", "..", "scripts", "testdata",
		"roadmap-barrier-corpus-snapshot",
		"ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture snapshot: %v", err)
	}
	lines := SplitRoadmapLines(string(data))
	n, fErr := FenceMaskCheck(lines)
	if n != 460 {
		t.Errorf("FenceMaskCheck(fixture-snapshot) linha = %d, quer 460", n)
	}
	if fErr == nil {
		t.Error("FenceMaskCheck(fixture-snapshot) err = nil, quer erro")
	}
}

func TestFenceMaskCheck_FixtureCongeladaArquivo2(t *testing.T) {
	// A cópia congelada do ROADMAP-2026-08-29 no corpus interno tem cerca aberta
	// na linha 1044.
	path := filepath.Join("testdata", "corpus", "docs", "roadmaps", "done",
		"ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-do-barrier.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture 2 corpus: %v", err)
	}
	lines := SplitRoadmapLines(string(data))
	n, fErr := FenceMaskCheck(lines)
	if n != 1044 {
		t.Errorf("FenceMaskCheck(fixture2-corpus) linha = %d, quer 1044", n)
	}
	if fErr == nil {
		t.Error("FenceMaskCheck(fixture2-corpus) err = nil, quer erro")
	}
	if fErr != nil && !strings.Contains(fErr.Error(), "1044") {
		t.Errorf("FenceMaskCheck(fixture2-corpus) err = %q, deve conter '1044'", fErr.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Fixtures compartilhadas para testes de predicados
// ────────────────────────────────────────────────────────────────────────────

// docCercaAbertaComMLCompleto: Wave 0 com ML concluído antes da cerca aberta.
// O viés antigo do predicado: tudo está ok ANTES da cerca, então o predicado
// retornaria o valor favorável (Wave0GateOK, hasAnyNonPendingML=true, etc.).
// Com o guard, o predicado deve ser fail-closed independentemente do que vem antes.
const docCercaAbertaComMLCompleto = "# ROADMAP\n\n" +
	"## Wave 0 — threat model\n\n" +
	"**Gates da wave:**\n\n" +
	"```bash\nbash scripts/check.sh\n```\n\n" +
	"### ML-0A — threat model\n" +
	"**Status:** ✅ Concluído\n\n" +
	"```\n" + // cerca aberta sem fechamento — tudo abaixo fica mascarado
	"## Wave 1 — teria sido mascarada\n\n" +
	"### ML-1A — mascarado\n" +
	"**Status:** ⬜ Pendente\n"

// docCercaFechadaComMLCompleto: mesmo documento mas com a cerca fechada.
const docCercaFechadaComMLCompleto = "# ROADMAP\n\n" +
	"## Wave 0 — threat model\n\n" +
	"**Gates da wave:**\n\n" +
	"```bash\nbash scripts/check.sh\n```\n\n" +
	"### ML-0A — threat model\n" +
	"**Status:** ✅ Concluído\n\n" +
	"```\nexemplo de saída\n```\n"

// docTodosMLsPendentesComCercaAberta: Wave 1 com MLs todos pendentes, mas
// cerca aberta mascara um segundo ML que também é pendente.
const docTodosMLsPendentesComCercaAberta = "# ROADMAP\n\n" +
	"## Wave 1 — trabalho\n\n" +
	"### ML-1A — A\n" +
	"**Status:** ⬜ Pendente\n\n" +
	"```\n" + // cerca aberta
	"### ML-1B — B mascarado\n" +
	"**Status:** ⬜ Pendente\n"

// docTodosMLsPendentesComCercaFechada: mesmos MLs, cerca fechada.
const docTodosMLsPendentesComCercaFechada = "# ROADMAP\n\n" +
	"## Wave 1 — trabalho\n\n" +
	"### ML-1A — A\n" +
	"**Status:** ⬜ Pendente\n\n"

// docTodosMLsConcluidosComCercaAberta: todos MLs concluídos antes da cerca aberta.
const docTodosMLsConcluidosComCercaAberta = "# ROADMAP\n\n" +
	"## Wave 1 — trabalho\n\n" +
	"### ML-1A — A\n" +
	"**Status:** ✅ Concluído\n\n" +
	"```\n" // cerca aberta

// docTodosMLsConcluidosComCercaFechada: todos MLs concluídos, cerca fechada.
const docTodosMLsConcluidosComCercaFechada = "# ROADMAP\n\n" +
	"## Wave 1 — trabalho\n\n" +
	"### ML-1A — A\n" +
	"**Status:** ✅ Concluído\n\n"

// ────────────────────────────────────────────────────────────────────────────
// Wave0GateDiagnosis — fail-closed
// ────────────────────────────────────────────────────────────────────────────

func TestWave0GateDiagnosis_FailClosedComCercaAberta(t *testing.T) {
	// O documento tem Wave 0 com gate real e ML concluído ANTES da cerca aberta.
	// Sem o guard, Wave0GateDiagnosis encontraria Wave 0 e retornaria Wave0GateOK.
	// Com o guard, deve retornar valor não-OK (Wave0GateMalformed) imediatamente.
	got := Wave0GateDiagnosis(docCercaAbertaComMLCompleto)
	if got == Wave0GateOK {
		t.Fatalf("Wave0GateDiagnosis(cerca aberta) = Wave0GateOK, quer Wave0GateMalformed (fail-closed)")
	}
	if got != Wave0GateMalformed {
		t.Errorf("Wave0GateDiagnosis(cerca aberta) = %v, quer Wave0GateMalformed", got)
	}
}

func TestWave0GateDiagnosis_OKComDocFechado(t *testing.T) {
	// Documento bem-formado com Wave 0 real: o guard não deve criar falso positivo.
	got := Wave0GateDiagnosis(docCercaFechadaComMLCompleto)
	if got != Wave0GateOK {
		t.Fatalf("Wave0GateDiagnosis(doc fechado) = %v, quer Wave0GateOK", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// hasAnyNonPendingML — fail-closed
// ────────────────────────────────────────────────────────────────────────────

func TestHasAnyNonPendingML_FailClosedComCercaAberta(t *testing.T) {
	// Documento com cerca aberta: hasAnyNonPendingML deve retornar true (fail-closed)
	// mesmo que não haja ML não-pendente visível.
	if !hasAnyNonPendingML(docTodosMLsPendentesComCercaAberta) {
		t.Fatal("hasAnyNonPendingML(cerca aberta, todos pendentes) = false, quer true (fail-closed)")
	}
}

func TestHasAnyNonPendingML_OKComDocFechado(t *testing.T) {
	// Documento bem-formado com todos MLs pendentes: deve retornar false.
	if hasAnyNonPendingML(docTodosMLsPendentesComCercaFechada) {
		t.Fatal("hasAnyNonPendingML(doc fechado, todos pendentes) = true, quer false")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// HasWave0 — fail-closed
// ────────────────────────────────────────────────────────────────────────────

func TestHasWave0_FailClosedComCercaAberta(t *testing.T) {
	// Documento com Wave 0 presente mas cerca aberta logo depois.
	// Sem o guard, HasWave0 retornaria true. Com o guard, deve retornar false.
	if HasWave0(docCercaAbertaComMLCompleto) {
		t.Fatal("HasWave0(cerca aberta) = true, quer false (fail-closed)")
	}
}

func TestHasWave0_OKComDocFechado(t *testing.T) {
	// Mesmo documento com cerca fechada: HasWave0 deve retornar true.
	if !HasWave0(docCercaFechadaComMLCompleto) {
		t.Fatal("HasWave0(doc fechado) = false, quer true")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// HasUnfinishedMLs — fail-closed
// ────────────────────────────────────────────────────────────────────────────

func TestHasUnfinishedMLs_FailClosedComCercaAberta(t *testing.T) {
	// Documento com todos MLs concluídos MAS cerca aberta.
	// Sem o guard, HasUnfinishedMLs retornaria false (MLs estão todos ✅).
	// Com o guard, deve retornar true (a cauda mascarada pode ter MLs pendentes).
	if !HasUnfinishedMLs(docTodosMLsConcluidosComCercaAberta) {
		t.Fatal("HasUnfinishedMLs(cerca aberta, todos concluídos) = false, quer true (fail-closed)")
	}
}

func TestHasUnfinishedMLs_OKComDocFechado(t *testing.T) {
	// Documento bem-formado com todos MLs concluídos: deve retornar false.
	if HasUnfinishedMLs(docTodosMLsConcluidosComCercaFechada) {
		t.Fatal("HasUnfinishedMLs(doc fechado, todos concluídos) = true, quer false")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TestAcervoSemCercaAberta — gate de acervo (nome é contrato)
// ────────────────────────────────────────────────────────────────────────────

// TestAcervoSemCercaAberta percorre docs/roadmaps/**/*.md a partir da raiz do
// módulo e falha nomeando arquivo e linha se FenceMaskCheck acusar cerca aberta.
// É o gate de acervo das Waves 1 e 4 deste roadmap; o nome é contrato.
func TestAcervoSemCercaAberta(t *testing.T) {
	root := repoRoot(t)
	acervoDir := filepath.Join(root, "docs", "roadmaps")

	var count int
	err := filepath.Walk(acervoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		count++
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("ReadFile %s: %v", path, readErr)
			return nil
		}
		lines := SplitRoadmapLines(string(data))
		n, fErr := FenceMaskCheck(lines)
		if fErr != nil {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("cerca aberta em %s: %s (linha %d)", rel, fErr.Error(), n)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk %s: %v", acervoDir, err)
	}
	if count == 0 {
		t.Fatalf("nenhum arquivo .md encontrado em %s — acervo vazio ou caminho errado", acervoDir)
	}
	t.Logf("TestAcervoSemCercaAberta: %d arquivos verificados em %s", count, acervoDir)
}
