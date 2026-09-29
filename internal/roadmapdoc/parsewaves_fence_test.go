package roadmapdoc

import (
	"strings"
	"testing"
)

// Fence awareness of ParseWaves (issue #470).
//
// RECONCILIAÇÃO (Regra Dura — todo teste novo declara o que afirma):
//
//   - TestParseWaves_CercaNaoFechaAOnda afirma a conclusão de que o sintoma relatado
//     na #470 ("no acceptance block" num ML que TEM o bloco) vinha da varredura de fim
//     de bloco da ParseWaves, que usava strings.HasPrefix cru: o bloco de aceite ficava
//     fora do intervalo da wave, e não do ML.
//   - TestParseWaves_CercaNaoInventaOnda afirma a conclusão de que a mesma linha tem
//     uma segunda direção de falha — uma onda inteira nascia de dentro da cerca —, que
//     é a que o comentário do HasWave0 declarava "theoretical edge case".
//   - TestParseWaves_HeadingRealForaDeCercaAindaFecha é o CONTROLE: afirma que a
//     correção não cegou o parser. Sem ele, um ParseWaves que ignorasse todo "## "
//     passaria nos dois primeiros.
//   - TestParseWaves_ConcordaComDuplicateWaveOrMLLabels afirma a conclusão de que os
//     dois leitores do mesmo literal, no mesmo arquivo, discordavam antes da correção:
//     DuplicateWaveOrMLLabels já pulava heading cercado, ParseWaves não.
//
// 🔴 ARMADILHA DE FIXTURE: WaveHeadingRe é `^## Wave (\S+) ` — com espaço FINAL. Uma
// linha "## Wave 1" terminada ali NÃO casa, e o teste mede zero ondas sem dizer por quê.
// Todo heading abaixo tem texto depois do rótulo de propósito.

// docComCercaQueFechaOnda reproduz o roadmap da #470: o ML cola a saída do
// `trackfw context`, que emite linhas começando com "## ".
const docComCercaQueFechaOnda = "# ROADMAP\n" +
	"\n" +
	"## Wave 1 — a onda\n" +
	"\n" +
	"### ML-1A — cola a saída do context\n" +
	"\n" +
	"**Status:** ✅ Concluído\n" +
	"\n" +
	"```\n" +
	"## ADRs (3)\n" +
	"## Warnings (6)\n" +
	"```\n" +
	"\n" +
	"**Critérios de aceite:**\n" +
	"- [x] o bloco existe\n"

func TestParseWaves_CercaNaoFechaAOnda(t *testing.T) {
	lines := SplitRoadmapLines(docComCercaQueFechaOnda)
	fenced := FenceMask(lines)

	// Guarda de vacuidade: se a máscara não marcar as duas linhas "## " do interior,
	// este teste passa a medir outra coisa — e passaria mesmo com a ParseVWaves antiga.
	var marcadas int
	for i, l := range lines {
		if strings.HasPrefix(l, "## ADRs") || strings.HasPrefix(l, "## Warnings") {
			if fenced[i] {
				marcadas++
			}
		}
	}
	if marcadas != 2 {
		t.Fatalf("guarda: esperava as 2 linhas do interior da cerca mascaradas, obtive %d", marcadas)
	}

	waves, malformed := ParseWaves(lines, fenced)
	if len(malformed) != 0 {
		t.Fatalf("nenhuma wave malformada esperada, obtive %v", malformed)
	}
	if len(waves) != 1 {
		t.Fatalf("esperava 1 wave, obtive %d: %+v", len(waves), waves)
	}

	// O que a #470 mede: o bloco de aceite tem de cair DENTRO do intervalo da wave.
	mls := ParseMLs(lines, fenced, waves[0].Start, waves[0].End)
	if len(mls) != 1 {
		t.Fatalf("esperava 1 ML, obtive %d", len(mls))
	}
	met, unmet, hasBlock := AcceptanceEvaluate(lines, fenced, mls[0])
	if !hasBlock {
		t.Fatalf("o ML TEM bloco de aceite; AcceptanceEvaluate disse que não (met=%d unmet=%d) — "+
			"é exatamente o \"no acceptance block\" da #470", met, unmet)
	}
	if met != 1 || unmet != 0 {
		t.Fatalf("esperava met=1 unmet=0, obtive met=%d unmet=%d", met, unmet)
	}
}

func TestParseWaves_CercaNaoInventaOnda(t *testing.T) {
	doc := "# ROADMAP\n" +
		"\n" +
		"## Wave 1 — a única onda real\n" +
		"\n" +
		"### ML-1A — documenta o formato de onda\n" +
		"\n" +
		"**Status:** ✅ Concluído\n" +
		"\n" +
		"```\n" +
		"## Wave 9 — este é um EXEMPLO dentro da cerca\n" +
		"```\n" +
		"\n" +
		"**Critérios de aceite:**\n" +
		"- [x] existe\n"

	lines := SplitRoadmapLines(doc)
	waves, malformed := ParseWaves(lines, FenceMask(lines))
	if len(malformed) != 0 {
		t.Fatalf("nenhuma wave malformada esperada, obtive %v", malformed)
	}
	if len(waves) != 1 {
		var rotulos []string
		for _, w := range waves {
			rotulos = append(rotulos, w.Label)
		}
		t.Fatalf("esperava só a Wave 1; obtive %d ondas %v — a de dentro da cerca é exemplo, não estrutura",
			len(waves), rotulos)
	}
	if waves[0].Label != "1" {
		t.Fatalf("esperava a wave de rótulo \"1\", obtive %q", waves[0].Label)
	}

	// HasWave0 é o sítio cujo comentário declarava este caso "theoretical". Com a
	// máscara, um "## Wave 0 " cercado deixa de satisfazer o predicado.
	docComWave0Cercada := strings.Replace(doc,
		"## Wave 9 — este é um EXEMPLO dentro da cerca",
		"## Wave 0 — threat model, citado como EXEMPLO dentro da cerca", 1)
	if strings.Count(docComWave0Cercada, "## Wave 0 ") != 1 {
		t.Fatal("guarda: a substituição da fixture não pegou")
	}
	if HasWave0(docComWave0Cercada) {
		t.Fatal("HasWave0 achou a Wave 0 que está DENTRO da cerca — é exemplo, não threat model")
	}
}

// Controle: um "## " REAL, fora de qualquer cerca, continua fechando a wave.
// Sem este caso, uma ParseWaves que simplesmente ignorasse todo "## " passaria
// nos dois testes acima — o controle raso que a memória deste projeto cobra.
func TestParseWaves_HeadingRealForaDeCercaAindaFecha(t *testing.T) {
	doc := "# ROADMAP\n" +
		"\n" +
		"## Wave 1 — a onda\n" +
		"\n" +
		"### ML-1A — um microlote\n" +
		"\n" +
		"**Status:** ✅ Concluído\n" +
		"\n" +
		"## Notas — esta seção NÃO é wave e fecha a de cima\n" +
		"\n" +
		"### ML-9Z — não pertence a wave nenhuma\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("esperava 1 wave, obtive %d", len(waves))
	}

	idxNotas := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "## Notas") {
			idxNotas = i
			break
		}
	}
	if idxNotas < 0 {
		t.Fatal("guarda: a fixture perdeu a linha \"## Notas\"")
	}
	if waves[0].End != idxNotas {
		t.Fatalf("a wave devia terminar na linha %d (\"## Notas\"), terminou em %d", idxNotas, waves[0].End)
	}

	// E o ML de depois do corte não entra na wave.
	mls := ParseMLs(lines, fenced, waves[0].Start, waves[0].End)
	if len(mls) != 1 || mls[0].ID != "ML-1A" {
		t.Fatalf("esperava só o ML-1A dentro da wave, obtive %+v", mls)
	}
}

// Os dois leitores do mesmo literal têm de concordar. DuplicateWaveOrMLLabels já
// consultava a máscara; ParseWaves não — então um "## Wave 1 " cercado era onda
// para um e não era para o outro, no MESMO arquivo.
func TestParseWaves_ConcordaComDuplicateWaveOrMLLabels(t *testing.T) {
	doc := "# ROADMAP\n" +
		"\n" +
		"## Wave 1 — a onda real\n" +
		"\n" +
		"### ML-1A — cita o próprio formato\n" +
		"\n" +
		"**Status:** ✅ Concluído\n" +
		"\n" +
		"```\n" +
		"## Wave 1 — a citação usa o MESMO rótulo de propósito\n" +
		"```\n" +
		"\n" +
		"**Critérios de aceite:**\n" +
		"- [x] existe\n"

	if msgs := DuplicateWaveOrMLLabels(doc); len(msgs) != 0 {
		t.Fatalf("DuplicateWaveOrMLLabels não devia ver duplicata (a segunda está cercada): %v", msgs)
	}
	lines := SplitRoadmapLines(doc)
	waves, _ := ParseWaves(lines, FenceMask(lines))
	if len(waves) != 1 {
		t.Fatalf("ParseWaves devia concordar e ver 1 wave, viu %d — os dois leitores divergem", len(waves))
	}
}
