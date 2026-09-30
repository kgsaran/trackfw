package roadmapdoc

import (
	"strings"
	"testing"
)

// Deteccao de cerca nao terminada (issue #476).
//
// RECONCILIACAO (Regra Dura — todo teste novo declara o que afirma):
//
//   - TestUnterminatedFence_DetectaAAbertura afirma a conclusao de que a regra 6 do
//     docs/cli-parity.md ("an unterminated fence is a usage error … naming the offending
//     line number") tinha implementacao apenas para a cerca de GATES; no nivel do
//     documento nao havia deteccao nenhuma. O numero devolvido e a linha 1-based da
//     abertura, que e o que a mensagem precisa nomear.
//   - TestUnterminatedFence_CercaFechadaDevolveZero e o CONTROLE. Sem ele, uma funcao
//     que devolvesse sempre um numero positivo passaria no primeiro teste.
//   - TestUnterminatedFence_SaiDaMesmaVarreduraQueOFenceMask afirma a conclusao de que
//     manter dois classificadores de cerca independentes e o risco real: o teste
//     confronta os dois leitores no MESMO documento e exige que concordem sobre onde a
//     regiao mascarada comeca.
//   - TestUnterminatedFence_InfoStringNaoFecha afirma a conclusao de que a assimetria do
//     CommonMark (abertura aceita info string, fechamento nao) e o que produz o caso real
//     do corpus: ```bash no meio de um bloco aberto NAO fecha nada.
//
// 🔴 ARMADILHA DE FIXTURE, medida: um documento cujo texto termina em "```" sem quebra
// de linha final ainda tem esse "```" como LINHA, e ele fecha. Os casos abaixo terminam
// com quebra de linha explicita para que "aberta" signifique aberta.

func TestUnterminatedFence_DetectaAAbertura(t *testing.T) {
	doc := "# ROADMAP\n" +
		"\n" +
		"## Wave 1 — a onda\n" +
		"\n" +
		"```\n" + // linha 5: abre e nunca fecha
		"$ trackfw context\n" +
		"algum texto depois\n"

	lines := SplitRoadmapLines(doc)
	got := UnterminatedFenceLine(lines)
	if got != 5 {
		t.Fatalf("esperava a linha 5 (onde a cerca abre), obtive %d", got)
	}

	// E tudo depois dela tem de estar mascarado — e a consequencia que torna o
	// documento ilegivel, e o motivo de isto ser erro de uso e nao aviso.
	fenced := FenceMask(lines)
	for i := 5; i < len(lines); i++ {
		if !fenced[i] {
			t.Fatalf("linha %d deveria estar mascarada (depois da cerca aberta): %q", i+1, lines[i])
		}
	}
}

func TestUnterminatedFence_CercaFechadaDevolveZero(t *testing.T) {
	doc := "# ROADMAP\n" +
		"\n" +
		"```\n" +
		"$ trackfw context\n" +
		"```\n" +
		"\n" +
		"### ML-1A — depois da cerca, e visivel\n"

	lines := SplitRoadmapLines(doc)
	if got := UnterminatedFenceLine(lines); got != 0 {
		t.Fatalf("a cerca FECHA; esperava 0, obtive %d", got)
	}
	// Controle do controle: a linha depois do fechamento nao pode estar mascarada.
	fenced := FenceMask(lines)
	idx := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "### ML-1A") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("guarda: a fixture perdeu o heading do ML")
	}
	if fenced[idx] {
		t.Fatal("o ML esta DEPOIS do fechamento e nao pode estar mascarado")
	}
}

func TestUnterminatedFence_SaiDaMesmaVarreduraQueOFenceMask(t *testing.T) {
	// Tres documentos com aberturas em posicoes diferentes. Em cada um, a primeira
	// linha mascarada tem de ser exatamente a seguinte a abertura reportada.
	for _, doc := range []string{
		"a\n```\nb\nc\n",
		"a\nb\n~~~\nc\n",
		"```\na\n```\nb\n````\nc\n",
	} {
		lines := SplitRoadmapLines(doc)
		aberta := UnterminatedFenceLine(lines)
		fenced := FenceMask(lines)
		if aberta == 0 {
			t.Fatalf("fixture %q deveria ter cerca aberta", doc)
		}
		// aberta e 1-based; o indice da linha de abertura e aberta-1, e ela
		// NUNCA e mascarada (a mascara marca so o interior).
		if fenced[aberta-1] {
			t.Fatalf("doc %q: a propria linha de abertura (%d) nao deve ser mascarada", doc, aberta)
		}
		if aberta < len(lines) && !fenced[aberta] {
			t.Fatalf("doc %q: a linha seguinte a abertura (%d) devia estar mascarada — "+
				"os dois leitores discordam sobre onde a regiao comeca", doc, aberta+1)
		}
	}
}

func TestUnterminatedFence_InfoStringNaoFecha(t *testing.T) {
	// O caso real do corpus: o autor abre ```, escreve ```bash achando que abre outro
	// bloco, e fecha uma vez so. Em CommonMark a linha com info string NAO fecha nada,
	// entao o documento termina com uma cerca aberta.
	doc := "# ROADMAP\n" +
		"\n" +
		"```\n" + // linha 3: abre
		"$ comando\n" +
		"```bash\n" + // linha 5: info string — NAO fecha
		"outro comando\n"

	lines := SplitRoadmapLines(doc)
	if got := UnterminatedFenceLine(lines); got != 3 {
		t.Fatalf("a cerca aberta e a da linha 3 (a da 5 tem info string e nao fecha); obtive %d", got)
	}
}
