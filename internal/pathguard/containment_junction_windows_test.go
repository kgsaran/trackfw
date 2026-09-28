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
// A assimetria de privilégio que torna a junção o vetor de ataque:
//
//	mklink /D  (symlink de diretório)  →  exige privilégio; falha numa conta comum
//	mklink /J  (junção)                →  NÃO exige privilégio; sempre funciona
//
// Corrigido em ML-1A (2026-09-27): o predicado passou de `ModeSymlink` para
// `ModeSymlink|ModeIrregular`. Sob go.mod >= 1.23 (winsymlink=1, trackfw usa
// go 1.25.2), juncões reportam ModeIrregular — o bit antigo não as via.
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

// AFIRMA: com a junção no caminho, RejectSymlinks devolve erro — a guarda recusa a
// escrita. É o comportamento corrigido pelo ML-1A (#444): o predicado
// ModeSymlink|ModeIrregular captura a junção que ModeSymlink sozinho não via.
func TestRejectSymlinks_RefusesAJunction(t *testing.T) {
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

	// Braço C2 — controle na direção oposta: um caminho que escapa DE FORMA VISÍVEL
	// continua sendo recusado. Sem ele, "RejectSymlinks devolveu erro" poderia
	// significar que a guarda está recusando tudo, e não que a junção foi detectada.
	visible := filepath.Join(base, "victim", "adr", "ADR-probe.md")
	if visibleErr := RejectSymlinks(root, visible); visibleErr == nil {
		t.Fatal("the guard accepted a path that escapes root in plain sight — it is not armed, " +
			"so this file proves nothing about junctions (braço C2 falhou)")
	}

	// Braço principal — a guarda DEVE recusar a junção.
	err := RejectSymlinks(root, target)
	if err == nil {
		t.Fatalf("RejectSymlinks(%q, %q) = nil — the guard accepted a path that traverses a "+
			"junction; the reparse-point containment (ML-1A, #444) is broken. "+
			"The write would silently reach %q, outside the root.", root, target, victim)
	}
}
