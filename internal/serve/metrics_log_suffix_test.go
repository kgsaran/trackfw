package serve

import (
	"os"
	"path/filepath"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// #407 — a linha de transição ganhou fuso e autor NO FIM, e a afirmação inteira
// depende de o ParseLog de hoje continuar lendo sem mudança nenhuma.
//
// Este teste roda contra o ParseLog REAL, e não contra uma cópia da regex. É a
// diferença entre "a forma combina com a régua que eu transcrevi" e "a forma é lida
// pelo leitor que existe".
//
// Reconciliação (Regra Dura): a frase de cada teste está no seu comentário.
// ────────────────────────────────────────────────────────────────────────────

// AFIRMA: o ParseLog de hoje lê a linha nova sem alteração, e continua extraindo os
// MESMOS quatro campos — o sufixo não vaza para o estado de destino. O braço de
// controle é a forma com o offset logo depois da hora: ela tem de ser DESCARTADA,
// porque foi medindo isso que o #407 decidiu pôr o acréscimo no fim.
func TestParseLog_ReadsTheSuffixedLineAndDropsTheBrokenOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".trackfw-log")

	const conteudo = "" +
		"2026-09-18 08:29  claude/ROADMAP-a.md  backlog → wip\n" +
		"2026-09-18 08:30  claude/ROADMAP-b.md  backlog → wip  -0300\n" +
		"2026-09-18 08:31  claude/ROADMAP-c.md  wip → done  -0300  (ana@exemplo.com)\n" +
		"2026-09-18 08:32 -0300  claude/ROADMAP-d.md  backlog → wip\n" // controle: quebrada

	if err := os.WriteFile(path, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ParseLog(path)

	if len(got) != 3 {
		t.Fatalf("esperava 3 transições lidas (a quarta é a forma quebrada, de propósito), veio %d: %+v", len(got), got)
	}

	// A linha de hoje continua igual.
	if got[0].Basename != "claude/ROADMAP-a.md" || got[0].To != "wip" {
		t.Fatalf("a linha sem sufixo mudou de leitura: %+v", got[0])
	}

	// Com fuso no fim: o destino é "wip", e não "wip  -0300" nem "-0300".
	if got[1].To != "wip" {
		t.Fatalf("o offset vazou para o destino: %q", got[1].To)
	}

	// Com fuso e autor: idem, e o basename não foi comido.
	if got[2].To != "done" || got[2].Basename != "claude/ROADMAP-c.md" {
		t.Fatalf("a linha com autor foi lida errado: %+v", got[2])
	}

	// E o controle: a forma que o #407 mediu como quebrada continua fora. Sem esta
	// asserção, "3 linhas lidas" poderia significar que a quarta entrou no lugar de
	// outra.
	for _, tr := range got {
		if tr.Basename == "claude/ROADMAP-d.md" {
			t.Fatal("a forma com o offset depois da hora foi lida — o discriminante do #407 não vale mais, e a decisão de pôr o sufixo no fim precisa ser remedida")
		}
	}
}
