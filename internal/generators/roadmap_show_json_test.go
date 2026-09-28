package generators

import (
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// Parte 2 da #407 — o status de ML deixa de existir só como texto.
//
// A afirmação que esta saída faz é uma só: quem consome não precisa mais
// reimplementar o dialeto. Os testes abaixo medem exatamente isso, e cada um
// declara, no seu comentário, qual conclusão afirma (Regra Dura).
//
// 🔴 Nota de fixture, aprendida por reprovação: `WaveHeadingRe` é
// `^## Wave (\S+) ` — com espaço no fim. Um heading `## Wave 1` sem texto depois
// NÃO casa, e a primeira versão destes testes media zero wave por isso. É uma
// regra do dialeto que um reimplementador perderia do mesmo jeito.
// ────────────────────────────────────────────────────────────────────────────

func doc(t *testing.T, body string) roadmapShowDoc {
	t.Helper()
	return buildRoadmapShowDoc("docs/roadmaps/wip/ROADMAP-x.md", body)
}

// AFIRMA: as grafias que o acervo realmente tem — medidas na #407: ✅ Concluído,
// ⬜ Pendente, pending, done, 🔄 Em andamento, ❌ Cancelado, ❌ Bloqueado e
// 🚫 **Abandonado** — colapsam nas TRÊS categorias do produto, e não numa
// classificação nova. É este campo que dispensa o segundo leitor.
//
// 🔴 Os dois braços de ❌ são o discriminante: "❌ Cancelado" é terminated e
// "❌ Bloqueado" é pending. Um leitor que olhasse só o primeiro token daria a
// mesma resposta para os dois — e é precisamente o erro que um reimplementador
// comete.
func TestRoadmapShowJSON_NormalizesTheMeasuredVocabulary(t *testing.T) {
	casos := []struct {
		marcador string
		quer     string
	}{
		{"✅ Concluído", "complete"},
		{"done", "complete"},
		{"⬜ Pendente", "pending"},
		{"pending", "pending"},
		{"🔄 Em andamento", "pending"},
		{"❌ Bloqueado", "pending"},
		{"❌ Cancelado", "terminated"},
		{"🚫 **Abandonado**", "terminated"},
	}

	var sb strings.Builder
	sb.WriteString("# ROADMAP\n\n## Wave 1 — vocabulário\n\n")
	for i, c := range casos {
		sb.WriteString("### ML-1" + string(rune('A'+i)) + " — caso\n")
		sb.WriteString("**Status:** " + c.marcador + "\n\n")
	}

	got := doc(t, sb.String())
	if len(got.Waves) != 1 {
		t.Fatalf("esperava uma wave, veio %d", len(got.Waves))
	}
	mls := got.Waves[0].MLs
	if len(mls) != len(casos) {
		t.Fatalf("esperava %d MLs, vieram %d", len(casos), len(mls))
	}
	for i, c := range casos {
		if mls[i].Status != c.quer {
			t.Errorf("marcador %q → status %q, queria %q", c.marcador, mls[i].Status, c.quer)
		}
		// E o cru continua lá, byte a byte.
		if mls[i].StatusMarker != c.marcador {
			t.Errorf("o marcador cru foi alterado: %q → %q", c.marcador, mls[i].StatusMarker)
		}
	}
}

// AFIRMA: `**Status:**` dentro de cerca de código NÃO é lido — é a regra do
// dialeto que um segundo leitor mais facilmente perde, porque o texto está lá e
// parece um marcador. O contra-braço, fora da cerca, prova que a sonda enxerga
// quando deve.
func TestRoadmapShowJSON_IgnoresStatusInsideACodeFence(t *testing.T) {
	dentro := doc(t, "# R\n\n## Wave 1 — w\n\n### ML-1A — x\n```\n**Status:** ✅ Concluído\n```\n")
	if len(dentro.Waves) != 1 || len(dentro.Waves[0].MLs) != 1 {
		t.Fatalf("estrutura inesperada: %+v", dentro)
	}
	if dentro.Waves[0].MLs[0].StatusFound {
		t.Fatalf("leu um Status que está dentro de cerca de código: %q", dentro.Waves[0].MLs[0].StatusMarker)
	}

	fora := doc(t, "# R\n\n## Wave 1 — w\n\n### ML-1A — x\n**Status:** ✅ Concluído\n")
	if !fora.Waves[0].MLs[0].StatusFound || fora.Waves[0].MLs[0].Status != "complete" {
		t.Fatalf("o contra-braço falhou — fora da cerca o Status tem de ser lido: %+v", fora.Waves[0].MLs[0])
	}
}

// AFIRMA: wave com heading malformado NÃO some do documento. O ParseWaves a isola
// de propósito, então um consumidor que só olhasse `waves` concluiria que o
// arquivo está íntegro — o mesmo "verde por não ter olhado" que este projeto já
// pagou. Aqui ela aparece nomeada, com a linha.
func TestRoadmapShowJSON_SurfacesMalformedWaves(t *testing.T) {
	got := doc(t, "# R\n\n## Wave abc — w\n\n### ML-1A — x\n**Status:** ⬜ Pendente\n\n## Wave 2 — w\n\n### ML-2A — y\n**Status:** done\n")
	if len(got.MalformedWaves) == 0 {
		t.Fatal("a wave malformada não apareceu no documento — some em silêncio")
	}
	if got.MalformedWaves[0].Token != "abc" {
		t.Fatalf("token da wave malformada saiu %q", got.MalformedWaves[0].Token)
	}
	if got.MalformedWaves[0].Line <= 0 {
		t.Fatalf("linha da wave malformada saiu %d — tooling não consegue apontar", got.MalformedWaves[0].Line)
	}
	// E a wave válida continua sendo lida: isolar não é abortar.
	if len(got.Waves) != 1 || got.Waves[0].Label != "2" {
		t.Fatalf("a wave válida foi perdida junto: %+v", got.Waves)
	}
}

// AFIRMA: ML sem linha de Status é distinguível pelo campo status_found, e cai em
// pending — fail-safe. Sem esse campo, "sem linha" e "linha vazia" seriam a mesma
// coisa para quem audita, e são situações diferentes.
func TestRoadmapShowJSON_DistinguishesAbsentStatus(t *testing.T) {
	ausente := doc(t, "# R\n\n## Wave 1 — w\n\n### ML-1A — sem linha de status\nTexto qualquer.\n")
	if len(ausente.Waves) != 1 || len(ausente.Waves[0].MLs) != 1 {
		t.Fatalf("estrutura inesperada: %+v", ausente)
	}
	if ausente.Waves[0].MLs[0].StatusFound {
		t.Fatal("ML sem linha de Status foi reportado como tendo uma")
	}
	if ausente.Waves[0].MLs[0].Status != "pending" {
		t.Fatalf("ML sem Status deve cair em pending (fail-safe), veio %q", ausente.Waves[0].MLs[0].Status)
	}
}

// AFIRMA: o heading sai CRU, sem tentativa de separar id de título. É decisão
// escrita no topo do arquivo de produção: adivinhar o separador seria inventar um
// dialeto para descrever um dialeto.
func TestRoadmapShowJSON_HeadingIsRaw(t *testing.T) {
	got := doc(t, "# R\n\n## Wave 1 — w\n\n### ML-1A — título: com dois pontos — e travessão\n**Status:** done\n")
	want := "ML-1A — título: com dois pontos — e travessão"
	if got.Waves[0].MLs[0].Heading != want {
		t.Fatalf("heading saiu %q, queria %q", got.Waves[0].MLs[0].Heading, want)
	}
}
