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

// ────────────────────────────────────────────────────────────────────────────
// ML-2D (REQ-2026-11-02) — campo gates por wave
// ────────────────────────────────────────────────────────────────────────────

// AFIRMA (ML-2D): wave com cabeçalho EN "**Wave gates:**" expõe os comandos no
// campo gates.commands, e wave com cabeçalho PT "**Gates da wave:**" também.
// Os dois aliases devem ser reconhecidos (ADR D1, ADR-2026-11-08).
func TestRoadmapShowJSON_GatesField_ENAndPTHeaders(t *testing.T) {
	body := "# ROADMAP\n\n## Wave 1 — en\n\n" +
		"**Wave gates:**\n" +
		"```bash\necho en-gate\n```\n\n" +
		"### ML-1A — x\n**Status:** done\n\n" +
		"## Wave 2 — pt\n\n" +
		"**Gates da wave:**\n" +
		"```bash\necho pt-gate\n```\n\n" +
		"### ML-2A — y\n**Status:** done\n"

	got := doc(t, body)
	if len(got.Waves) != 2 {
		t.Fatalf("esperava 2 waves, veio %d", len(got.Waves))
	}
	w1 := got.Waves[0]
	if len(w1.Gates.Commands) != 1 || w1.Gates.Commands[0] != "echo en-gate" {
		t.Errorf("wave EN: gates.commands = %v, queria [\"echo en-gate\"]", w1.Gates.Commands)
	}
	if w1.Gates.Error != "" {
		t.Errorf("wave EN: gates.error inesperado: %q", w1.Gates.Error)
	}
	w2 := got.Waves[1]
	if len(w2.Gates.Commands) != 1 || w2.Gates.Commands[0] != "echo pt-gate" {
		t.Errorf("wave PT: gates.commands = %v, queria [\"echo pt-gate\"]", w2.Gates.Commands)
	}
	if w2.Gates.Error != "" {
		t.Errorf("wave PT: gates.error inesperado: %q", w2.Gates.Error)
	}
}

// AFIRMA (ML-2D): wave sem cabeçalho de gate algum (nem EN nem PT) expõe lista
// vazia em gates.commands e nenhum erro. Zero gates é legal — a barrier nunca
// inventa um gate.
func TestRoadmapShowJSON_GatesField_WaveWithNoGatesHeader(t *testing.T) {
	body := "# ROADMAP\n\n## Wave 1 — sem gate\n\n" +
		"### ML-1A — x\n**Status:** done\n"

	got := doc(t, body)
	if len(got.Waves) != 1 {
		t.Fatalf("esperava 1 wave, veio %d", len(got.Waves))
	}
	g := got.Waves[0].Gates
	if g.Commands == nil {
		t.Error("gates.commands não deve ser nil — deve ser []string{}")
	}
	if len(g.Commands) != 0 {
		t.Errorf("wave sem gate: gates.commands = %v, queria []", g.Commands)
	}
	if g.Error != "" {
		t.Errorf("wave sem gate: gates.error inesperado: %q", g.Error)
	}
}

// AFIRMA (ML-2D): wave com dois cabeçalhos de gate válidos (D3,
// ADR-2026-11-08) resulta em gates.error não-vazio e gates.commands vazio.
// Um segundo leitor que ignorasse o campo error e confiasse só em commands
// veria "zero gates" e concluiria que a wave passa — o campo error é o
// mecanismo que expõe o defeito.
func TestRoadmapShowJSON_GatesField_DuplicateHeaderReturnsError(t *testing.T) {
	body := "# ROADMAP\n\n## Wave 1 — duplicado\n\n" +
		"**Wave gates:**\n```bash\necho first\n```\n\n" +
		"**Gates da wave:**\n```bash\necho second\n```\n\n" +
		"### ML-1A — x\n**Status:** done\n"

	got := doc(t, body)
	if len(got.Waves) != 1 {
		t.Fatalf("esperava 1 wave, veio %d", len(got.Waves))
	}
	g := got.Waves[0].Gates
	if g.Error == "" {
		t.Error("gates.error deve ser não-vazio para D3 (duplicate header), mas está vazio")
	}
	if g.Commands == nil {
		t.Error("gates.commands não deve ser nil mesmo com erro")
	}
	if len(g.Commands) != 0 {
		t.Errorf("gates.commands deve ser [] quando há erro D3, veio %v", g.Commands)
	}
}

// AFIRMA (ML-1C, D3): o campo `lapsed` do JSON conta separadamente os critérios
// "- [ ]" com Caducou: na linha seguinte. Com 1 met, 1 lapsed e 1 unmet, os três
// campos saem distintos (met=1, unmet=1, lapsed=1) — provando que AcceptanceEvaluateFull
// é chamado (não o antigo AcceptanceEvaluate, que colapsaria lapsed em unmet).
//
// Sabotagem S6: se Lapsed não for preenchido no JSON, lapsed sairia 0 enquanto
// unmet sairia 2 — e este teste falharia.
func TestRoadmapShowJSON_LapsedFieldIsPopulated(t *testing.T) {
	// Fixture: 1 met [x], 1 lapsed [ ] com Caducou:, 1 unmet [ ] sem Caducou:
	body := "# R\n\n## Wave 1 — w\n\n### ML-1A — ac\n" +
		"**Acceptance criteria:**\n" +
		"- [x] critério atendido\n" +
		"- [ ] critério caducado\n" +
		"  Caducou: substituído por nova abordagem\n" +
		"- [ ] critério aberto sem Caducou:\n" +
		"**Status:** ⬜ Pendente\n"

	got := doc(t, body)
	if len(got.Waves) != 1 || len(got.Waves[0].MLs) != 1 {
		t.Fatalf("estrutura inesperada: %+v", got)
	}
	ac := got.Waves[0].MLs[0].Acceptance
	if ac.Met != 1 {
		t.Errorf("met esperado 1, obteve %d", ac.Met)
	}
	if ac.Unmet != 1 {
		t.Errorf("unmet esperado 1 (só o genuinamente aberto), obteve %d — se fosse 2, AcceptanceEvaluate (sem Lapsed) está sendo chamado", ac.Unmet)
	}
	if ac.Lapsed != 1 {
		t.Errorf("lapsed esperado 1, obteve %d — campo não preenchido ou AcceptanceEvaluate antigo usado", ac.Lapsed)
	}
	if !ac.HasBlock {
		t.Error("has_block esperado true (há critérios no ML), obteve false")
	}
}
