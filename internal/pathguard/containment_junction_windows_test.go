//go:build windows

package pathguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// #444 — a contenção recusa symlink e não vê JUNÇÃO do Windows.
//
// Este arquivo NÃO escolhe o remédio. Ele fixa em teste a medição, porque a
// assimetria que a torna alcançável é de privilégio e não de forma:
//
//	mklink /D  (symlink de diretório)  →  exige privilégio; falha numa conta comum
//	mklink /J  (junção)                →  NÃO exige privilégio; sempre funciona
//
// Ou seja: a isca que a guarda recusa é a que um usuário comum não consegue criar,
// e a que ele consegue criar passa. Medido em 2026-09-27, Windows 11 sem Developer
// Mode, Go 1.26.1/amd64.
//
// 🔴 Por que não entra aqui um teste que EXIGE a recusa: recusar `ModeIrregular` em
// bloco alcança outros reparse points, e o AC13 da REQ-2026-09-09 escreve que
// falso-positivo neste portão "paralisa, não irrita" — há máquina Windows com junção
// legítima no caminho (redirecionamento de pasta). A direção é decisão de produto.
// Estes testes tornam o estado ATUAL visível e reproduzível; quando a decisão sair,
// é aqui que ela vira expectativa.
//
// Reconciliação (Regra Dura): cada teste declara, em uma frase, o que afirma.
// ────────────────────────────────────────────────────────────────────────────

// makeJunction creates dir\link pointing at dir\target using `mklink /J`.
//
// The relative form with a backslash is deliberate: an absolute path handed to
// cmd through a POSIX-ish toolchain arrives mangled ("A sintaxe do nome do
// arquivo … está incorreta") and mklink still exits 0 — a silent failure. Running
// from the parent with relative arguments avoids it.
func makeJunction(t *testing.T, dir, link, target string) {
	t.Helper()
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("mklink /J unavailable here (%v): %s", err, strings.TrimSpace(string(out)))
	}
	if _, err := os.Stat(filepath.Join(dir, link)); err != nil {
		t.Skipf("junction did not resolve: %v", err)
	}
}

// AFIRMA: o bit que a guarda testa (os.ModeSymlink) NÃO acende numa junção, e o
// EvalSymlinks também não a desfaz — logo nenhum dos dois instrumentos que a guarda
// tem enxerga a travessia. O diretório comum é o controle que separa "a sonda não
// funciona" de "a junção é invisível".
func TestJunction_IsInvisibleToBothInstruments(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, base, "link", "target")

	junction := filepath.Join(base, "link")
	info, err := os.Lstat(junction)
	if err != nil {
		t.Fatalf("lstat junction: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("junction reported ModeSymlink — the guard would already see it; "+
			"this test and #444 need re-measuring on this toolchain (mode=%v)", info.Mode())
	}

	// Controle: um diretório comum também não acende o bit. Sem ele, "ModeSymlink
	// ausente" não distinguiria junção de qualquer outra coisa.
	plainInfo, err := os.Lstat(filepath.Join(base, "plain"))
	if err != nil {
		t.Fatalf("lstat plain dir: %v", err)
	}
	if plainInfo.Mode()&os.ModeIrregular != 0 {
		t.Fatalf("a plain directory reported ModeIrregular — the discriminant is broken (mode=%v)", plainInfo.Mode())
	}
	if info.Mode()&os.ModeIrregular == 0 {
		t.Fatalf("junction did not report ModeIrregular; the mode is %v", info.Mode())
	}

	// E a segunda porta: resolver antes da guarda também não desmascara.
	resolved, err := filepath.EvalSymlinks(junction)
	if err != nil {
		t.Fatalf("EvalSymlinks on junction: %v", err)
	}
	if !strings.EqualFold(resolved, junction) {
		t.Logf("NOTE: EvalSymlinks resolved the junction to %q — it did NOT when #444 "+
			"was measured. If this holds, resolving before the guard becomes a viable "+
			"remedy and #444 should be re-read.", resolved)
	}
}

// AFIRMA: com a junção no caminho, RejectSymlinks devolve nil — a escrita atravessa
// para fora da raiz sem que a guarda diga nada. É o comportamento de hoje, não o
// desejado, e está aqui para ser reproduzível em vez de anedótico.
func TestRejectSymlinks_WalksThroughAJunction(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "project")
	victim := filepath.Join(base, "victim")
	if err := os.MkdirAll(filepath.Join(victim, "adr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, root, "docs", filepath.Join("..", "victim"))

	// O alvo está, COMO STRING, sob a raiz — por isso Beneath concorda. A travessia
	// acontece no sistema de arquivos, não no caminho.
	target := filepath.Join(root, "docs", "adr", "ADR-probe.md")
	if !Beneath(root, target) {
		t.Fatalf("Beneath said %q is outside %q — the scenario is not the one #444 describes", target, root)
	}

	err := RejectSymlinks(root, target)

	// Controle na direção oposta: um caminho que escapa DE FORMA VISÍVEL continua
	// sendo recusado. Sem ele, "RejectSymlinks devolveu nil" poderia significar que a
	// guarda está desligada, e não que a junção é invisível para ela.
	visible := filepath.Join(base, "victim", "adr", "ADR-probe.md")
	if visibleErr := RejectSymlinks(root, visible); visibleErr == nil {
		t.Fatal("the guard accepted a path that escapes root in plain sight — it is not armed, " +
			"so this file proves nothing about junctions")
	}

	if err != nil {
		t.Logf("RejectSymlinks refused the junction (%v). If this is intentional, #444 is "+
			"addressed and this test should become an expectation instead of a record.", err)
		return
	}
	t.Logf("measured state of #444: RejectSymlinks(%q, %q) = nil — the write reaches %q, "+
		"outside the root, with the guard silent.", root, target, victim)
}
