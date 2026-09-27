package generators

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// #407 — a linha de transição ganha fuso e autor, NO FIM.
//
// A restrição que decide a forma não é estética: a regex de
// internal/serve/metrics_log.go não é ancorada no fim, então um sufixo é invisível
// para ela — mas um offset inserido DEPOIS da hora quebra o \s{2,} que segue o grupo
// do timestamp, e a linha deixaria de ser lida em silêncio.
//
// Reconciliação (Regra Dura): a frase de cada teste está no seu comentário.
// ────────────────────────────────────────────────────────────────────────────

// parseLogRe é a regex de internal/serve/metrics_log.go, copiada LITERALMENTE.
//
// 🔴 É uma segunda cópia, e isso é declarado em vez de escondido: o pacote serve não
// exporta a regex, e importá-lo daqui inverteria a direção de dependência. Se a regex
// de lá mudar, este teste continua passando sobre a régua antiga — o que ele garante é
// a compatibilidade com a régua de HOJE, que é exatamente a afirmação do #407.
var parseLogRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s{2,}(\S+)\s{2,}(\S+)\s+→\s+(\S+)`)

// AFIRMA: a linha com fuso e autor no fim continua sendo lida pela régua de hoje — e
// a forma que o #407 mediu como quebrada (offset logo após a hora) continua quebrada,
// que é o controle que impede "passou" de significar "a régua aceita qualquer coisa".
func TestTransitionLine_StaysReadableByTodaysParser(t *testing.T) {
	const alvo = "claude/ROADMAP-2026-09-18-x.md"

	casos := []struct {
		nome  string
		linha string
		casa  bool
	}{
		{
			"hoje, sem sufixo",
			"2026-09-18 08:29  " + alvo + "  backlog → wip",
			true,
		},
		{
			"com fuso no fim",
			"2026-09-18 08:29  " + alvo + "  backlog → wip  -0300",
			true,
		},
		{
			"com fuso e autor no fim",
			"2026-09-18 08:29  " + alvo + "  backlog → wip  -0300  (ana@exemplo.com)",
			true,
		},
		{
			// O CONTROLE: a forma recusada. Se esta casasse, o teste acima não diria
			// nada — provaria só que a regex aceita tudo.
			"fuso logo depois da hora — a forma que quebra",
			"2026-09-18 08:29 -0300  " + alvo + "  backlog → wip",
			false,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := parseLogRe.MatchString(c.linha)
			if got != c.casa {
				t.Fatalf("MatchString = %v, queria %v\n  linha: %q", got, c.casa, c.linha)
			}
		})
	}
}

// AFIRMA: o escritor produz exatamente a forma que o teste acima aprova — sem isto,
// os dois poderiam divergir e o formato "compatível" existiria só no teste.
func TestTransitionLogSuffix_ProducesAParsableLine(t *testing.T) {
	quando := time.Date(2026, 9, 18, 8, 29, 0, 0, time.FixedZone("BRT", -3*60*60))

	sufixo := transitionLogSuffix(quando)
	if !strings.HasPrefix(sufixo, "  -0300") {
		t.Fatalf("o offset não abre o sufixo: %q", sufixo)
	}

	linha := "2026-09-18 08:29  claude/ROADMAP-x.md  backlog → wip" + sufixo
	if !parseLogRe.MatchString(linha) {
		t.Fatalf("a linha que o escritor produz não é lida pela régua de hoje: %q", linha)
	}

	// O destino continua sendo o quarto grupo — sem isto, "casou" poderia significar
	// que a regex casou o sufixo no lugar do estado.
	m := parseLogRe.FindStringSubmatch(linha)
	if m[4] != "wip" {
		t.Fatalf("o estado de destino saiu %q, e não %q — o sufixo entrou no grupo errado", m[4], "wip")
	}
}

// AFIRMA: o campo de autor é fail-soft e nunca quebra a linha. E-mail com quebra de
// linha ou com parêntese é descartado inteiro, porque a linha é a unidade de que
// dependem tanto o merge=union quanto o parser — um campo sujo partiria os dois.
func TestGitUserEmail_NeverBreaksTheLine(t *testing.T) {
	// gitUserEmail lê o git da máquina, então o que dá para afirmar aqui sem
	// depender do ambiente é a propriedade: o que ele devolve nunca contém os
	// caracteres que partiriam a linha.
	email := gitUserEmail()
	if strings.ContainsAny(email, "\r\n()") {
		t.Fatalf("gitUserEmail devolveu um valor que parte a linha: %q", email)
	}

	sufixo := transitionLogSuffix(time.Now())
	if strings.ContainsAny(sufixo, "\r\n") {
		t.Fatalf("o sufixo contém quebra de linha: %q", sufixo)
	}
}
