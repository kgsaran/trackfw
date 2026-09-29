package roadmapdoc

import (
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// #460 — a mensagem culpava o CONTEÚDO do gate quando a causa era POSICIONAL,
// e a forma mais natural de escrever um gate real era a única recusada.
//
// Reconciliação (Regra Dura): a frase de cada teste está no seu comentário.
// ────────────────────────────────────────────────────────────────────────────

// roadmapComGate monta um documento mínimo com Wave 0, um ML concluído (para que
// o discriminante de ML-4D considere que o trabalho começou) e o corpo de gates
// que o caso quiser.
func roadmapComGate(corpoGates string) string {
	return "# ROADMAP\n\n## Wave 0 — threat model\n\n" +
		corpoGates +
		"\n### ML-0A — threat model\n**Status:** ✅ Concluído\n"
}

// AFIRMA: prosa entre `**Gates da wave:**` e a cerca deixa de ser recusada, e o
// resultado é IDÊNTICO ao da forma sem prosa. É a bisseção do relato: o conteúdo
// do bloco é o mesmo nos dois braços, e a única variável é a prosa.
func TestParseGates_AcceptsProseBetweenMarkerAndFence(t *testing.T) {
	const gate = "bash scripts/threat-model-check.sh\n"

	semProsa := "**Gates da wave:**\n\n```bash\n" + gate + "```\n"
	comProsa := "**Gates da wave:**\n\n> O gate abaixo roda a checagem do modelo de ameaça\n> e recusa quando a superfície muda sem revisão.\n\n```bash\n" + gate + "```\n"

	for _, tc := range []struct {
		nome  string
		corpo string
	}{
		{"sem prosa", semProsa},
		{"com prosa", comProsa},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			lines := SplitRoadmapLines(roadmapComGate(tc.corpo))
			waves, malformed := ParseWaves(lines, FenceMask(lines))
			if len(malformed) != 0 || len(waves) != 1 {
				t.Fatalf("fixture inválida: waves=%d malformed=%d", len(waves), len(malformed))
			}
			cmds, err := ParseGates(lines, waves[0].Start, waves[0].End)
			if err != nil {
				t.Fatalf("ParseGates recusou: %v", err)
			}
			if len(cmds) != 1 || cmds[0] != strings.TrimSpace(gate) {
				t.Fatalf("comandos saíram %q", cmds)
			}
		})
	}
}

// AFIRMA: 🔴 a varredura PARA no próximo heading. É a guarda do afrouxamento: sem
// ela, uma cerca ```bash que pertence ao corpo de um ML viraria "o gate", e o
// produto passaria a executar um comando que ninguém declarou como gate. Este é o
// teste que impede a correção da #460 de abrir um buraco maior que o defeito.
func TestParseGates_StopsAtTheNextHeading(t *testing.T) {
	// Marcador presente, NENHUMA cerca antes do heading — e uma cerca logo depois,
	// dentro do ML. A cerca de dentro do ML não pode ser adotada.
	doc := "# ROADMAP\n\n## Wave 0 — threat model\n\n" +
		"**Gates da wave:**\n\n> só prosa, sem cerca nenhuma nesta seção\n\n" +
		"### ML-0A — threat model\n**Status:** ✅ Concluído\n\n" +
		"Exemplo de uso:\n\n```bash\nrm -rf /\n```\n"

	lines := SplitRoadmapLines(doc)
	waves, _ := ParseWaves(lines, FenceMask(lines))
	if len(waves) != 1 {
		t.Fatalf("fixture inválida: %d waves", len(waves))
	}
	cmds, err := ParseGates(lines, waves[0].Start, waves[0].End)
	if err == nil {
		t.Fatalf("a cerca de OUTRA seção foi adotada como gate: %q", cmds)
	}
	if !strings.Contains(err.Error(), "before the next heading") {
		t.Fatalf("a mensagem não nomeia a causa posicional: %v", err)
	}
}

// AFIRMA: as três causas são distinguíveis, e cada uma leva a uma ação diferente
// — reescrever o comando, escrever o bloco, mover a cerca. Era essa distinção que
// faltava e que custou seis ciclos no relato.
func TestWave0GateDiagnosis_NamesTheCause(t *testing.T) {
	casos := []struct {
		nome  string
		corpo string
		quer  Wave0GateCause
	}{
		{
			"placeholder intacto",
			"**Gates da wave:**\n\n```bash\nexit 1\n```\n",
			Wave0GatePlaceholder,
		},
		{
			"bloco ausente",
			"",
			Wave0GateAbsent,
		},
		{
			"marcador sem cerca antes do proximo heading",
			"**Gates da wave:**\n\n> prosa e nada mais\n\n",
			Wave0GateMalformed,
		},
		{
			"gate real",
			"**Gates da wave:**\n\n```bash\nbash scripts/check.sh\n```\n",
			Wave0GateOK,
		},
		{
			"gate real DEPOIS de prosa — o caso do relato",
			"**Gates da wave:**\n\n> explicação do gate\n\n```bash\nbash scripts/check.sh\n```\n",
			Wave0GateOK,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := Wave0GateDiagnosis(roadmapComGate(c.corpo))
			if got != c.quer {
				t.Fatalf("causa saiu %v, queria %v", got, c.quer)
			}
		})
	}
}

// AFIRMA: o predicado booleano continua sendo a leitura do diagnóstico, e não uma
// segunda derivação do mesmo veredito. Sem isto, os dois poderiam discordar e a
// mensagem diria uma coisa enquanto a violação dizia outra.
func TestWave0HasPlaceholderOrMissingGate_IsTheBooleanOfTheDiagnosis(t *testing.T) {
	corpos := []string{
		"**Gates da wave:**\n\n```bash\nexit 1\n```\n",
		"",
		"**Gates da wave:**\n\n> prosa e nada mais\n\n",
		"**Gates da wave:**\n\n```bash\nbash scripts/check.sh\n```\n",
		"**Gates da wave:**\n\n> explicação\n\n```bash\nbash scripts/check.sh\n```\n",
	}
	for _, corpo := range corpos {
		doc := roadmapComGate(corpo)
		quer := Wave0GateDiagnosis(doc) != Wave0GateOK
		if got := Wave0HasPlaceholderOrMissingGate(doc); got != quer {
			t.Fatalf("booleano %v discorda do diagnóstico %v, corpo=%q", got, Wave0GateDiagnosis(doc), corpo)
		}
	}
}

// AFIRMA: o discriminante do ML-4D sobrevive ao afrouxamento — scaffold recém-criado,
// com todos os MLs pendentes e placeholder intacto, continua NÃO sendo violação.
// Sem este braço, "a mensagem melhorou" poderia estar acompanhado de "e agora
// reprova quem não reprovava".
func TestWave0GateDiagnosis_FreshScaffoldIsStillLegitimate(t *testing.T) {
	doc := "# ROADMAP\n\n## Wave 0 — threat model\n\n" +
		"**Gates da wave:**\n\n```bash\nexit 1\n```\n" +
		"\n### ML-0A — threat model\n**Status:** ⬜ Pendente\n"
	if got := Wave0GateDiagnosis(doc); got != Wave0GateOK {
		t.Fatalf("scaffold recém-criado passou a ser violação (%v)", got)
	}
}
