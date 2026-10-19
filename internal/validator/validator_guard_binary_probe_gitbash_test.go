package validator

// validator_guard_binary_probe_gitbash_test.go — ML-5D (REQ-2026-09-05)
//
// Tests for guardGitBashBinaryProbe and its seams.
//
// Background: Claude Code on Windows runs hook commands via Git Bash (/usr/bin/bash).
// The Git Bash login profile prepends ~/bin to PATH, which can place an old trackfw binary
// (without guard) ahead of the binary that exec.LookPath sees from the Windows PATH.
// guardGitBashBinaryProbe detects this divergence and emits a violation.
//
// Regra de reconciliação (CLAUDE.md — Regra Dura de Reconciliação):
// each test has, in its opening comment, the conclusion of this ML that the test asserts.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// stubGitBashSeams replaces the three Git Bash probe seams (guardFindGitBashExe,
// guardRunBashLoginProbe, guardRunProbe) and restores them via t.Cleanup.
// The bashPath parameter is what guardFindGitBashExe returns.
// The loginResult/loginRan parameters are what guardRunBashLoginProbe returns.
// The probeResult function is called by guardRunProbe keyed on bin path.
func stubGitBashSeams(
	t *testing.T,
	bashPath string,
	loginResult string, loginRan bool,
	probeResult func(bin string) error,
) {
	t.Helper()
	origFind := guardFindGitBashExe
	origLogin := guardRunBashLoginProbe
	origRun := guardRunProbe
	guardFindGitBashExe = func() string { return bashPath }
	guardRunBashLoginProbe = func(_ string) (string, bool) { return loginResult, loginRan }
	guardRunProbe = probeResult
	t.Cleanup(func() {
		guardFindGitBashExe = origFind
		guardRunBashLoginProbe = origLogin
		guardRunProbe = origRun
	})
}

// --------------------------------------------------------------------------
// (1) Git Bash resolves different binary without guard → violation
// --------------------------------------------------------------------------

// Reconciliação: afirma que, quando o Git Bash login resolve um binário diferente do PATH
// e esse binário não tem o subcomando guard, guardGitBashBinaryProbe retorna violation
// nomeando o caminho resolvido e mencionando "~/bin".
// Este é o caso medido em ML-5B Rodada A: C:\Users\Lab\bin\trackfw 8.0.0-rc2 sem guard.
func TestGuardGitBashBinaryProbe_DifferentBinarioSemGuard_Violation(t *testing.T) {
	// PATH binary (exec.LookPath): C:\tools\trackfw.exe (has guard)
	// Git Bash login: /c/Users/Lab/bin/trackfw  (no .exe — measured form)
	// → C:\Users\Lab\bin\trackfw ≠ C:\tools\trackfw (after strip .exe)
	// → probe(C:\Users\Lab\bin\trackfw) fails → violation
	stubGitBashSeams(t,
		`C:\Program Files\Git\bin\bash.exe`,
		"/c/Users/Lab/bin/trackfw", true, // loginResult
		func(bin string) error {
			if bin == `C:\Users\Lab\bin\trackfw` {
				return errors.New("exit status 2") // no guard
			}
			return nil // PATH binary is fine
		},
	)

	msgs := guardGitBashBinaryProbe(`C:\tools\trackfw.exe`)
	if !hasViolation(msgs, "Git Bash") {
		t.Errorf("esperado violation com 'Git Bash', obteve: %v", msgs)
	}
	if !hasViolation(msgs, `C:\Users\Lab\bin\trackfw`) {
		t.Errorf("esperado violation nomeando o caminho resolvido, obteve: %v", msgs)
	}
	if !hasViolation(msgs, "~/bin") {
		t.Errorf("esperado violation mencionando '~/bin', obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// (2a) Git Bash resolves same binary (extension-only difference) → no violation
// --------------------------------------------------------------------------

// Reconciliação: afirma que, quando o Git Bash login resolve o mesmo binário que exec.LookPath
// mas sem a extensão .exe (/c/tools/trackfw vs C:\tools\trackfw.exe), guardGitBashBinaryProbe
// não emite violation — o guardStripExe normaliza a diferença de extensão.
func TestGuardGitBashBinaryProbe_SameBinarioExtensionDiff_Ok(t *testing.T) {
	// Git Bash: /c/tools/trackfw → C:\tools\trackfw
	// PATH:     C:\tools\trackfw.exe → C:\tools\trackfw (after strip)
	// → same → skip
	probeCallCount := 0
	stubGitBashSeams(t,
		`C:\Program Files\Git\bin\bash.exe`,
		"/c/tools/trackfw", true,
		func(bin string) error {
			probeCallCount++
			return nil
		},
	)

	msgs := guardGitBashBinaryProbe(`C:\tools\trackfw.exe`)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs para mesmo binário (só extensão difere), obteve: %v", msgs)
	}
	// guardRunProbe is called once by guardBinaryProbeOnce for the PATH binary, but
	// guardGitBashBinaryProbe itself must NOT call guardRunProbe when paths match.
	// Since this test calls guardGitBashBinaryProbe directly (not guardBinaryProbeOnce),
	// probeCallCount must be 0.
	if probeCallCount != 0 {
		t.Errorf("guardRunProbe não deve ser chamado quando o binário é o mesmo; callCount=%d", probeCallCount)
	}
}

// --------------------------------------------------------------------------
// (2b) Git Bash resolves different binary WITH guard → no violation
// --------------------------------------------------------------------------

// Reconciliação: afirma que, quando o Git Bash login resolve um binário diferente mas esse
// binário TEM o subcomando guard, guardGitBashBinaryProbe não emite violation — o usuário
// tem dois binários diferentes mas ambos funcionam.
func TestGuardGitBashBinaryProbe_DifferentBinarioComGuard_Ok(t *testing.T) {
	stubGitBashSeams(t,
		`C:\Program Files\Git\bin\bash.exe`,
		"/c/Users/Lab/guard-ml3c/bin/trackfw", true,
		func(bin string) error {
			return nil // both binaries have guard
		},
	)

	msgs := guardGitBashBinaryProbe(`C:\tools\trackfw.exe`)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs quando binário do Git Bash tem guard, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// (3) No Git Bash → guardRunBashLoginProbe not called
// --------------------------------------------------------------------------

// Reconciliação: afirma que, quando guardFindGitBashExe retorna "", guardRunBashLoginProbe
// não é chamada (contador = 0) — a sonda do Git Bash não roda se não há Git Bash instalado.
func TestGuardGitBashBinaryProbe_SemGitBash_SondaNaoRoda(t *testing.T) {
	origFind := guardFindGitBashExe
	origLogin := guardRunBashLoginProbe
	loginCallCount := 0
	guardFindGitBashExe = func() string { return "" }
	guardRunBashLoginProbe = func(_ string) (string, bool) {
		loginCallCount++
		return "", true
	}
	t.Cleanup(func() {
		guardFindGitBashExe = origFind
		guardRunBashLoginProbe = origLogin
	})

	msgs := guardGitBashBinaryProbe(`C:\tools\trackfw.exe`)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs sem Git Bash, obteve: %v", msgs)
	}
	if loginCallCount != 0 {
		t.Errorf("guardRunBashLoginProbe não deve ser chamada sem Git Bash; callCount=%d", loginCallCount)
	}
}

// --------------------------------------------------------------------------
// (4) Non-Windows → guardFindGitBashExe not called
// --------------------------------------------------------------------------

// Reconciliação: afirma que, em não-Windows (CurrentGOOS != "windows"), guardFindGitBashExe
// não é chamada — a sonda inteira do Git Bash não roda fora do Windows.
// Exercita o guard via guardBinaryProbeOnce para testar o ramo condicional do chamador.
func TestGuardBinaryProbeOnce_NaoWindows_GitBashFindNaoEhChamada(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindGitBashExe
	findCallCount := 0
	guardLookupBinary = func(_ string) (string, error) { return "/usr/local/bin/trackfw", nil }
	guardRunProbe = func(_ string) error { return nil }
	guardFindGitBashExe = func() string {
		findCallCount++
		return `C:\Program Files\Git\bin\bash.exe`
	}
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindGitBashExe = origFind
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs em não-Windows, obteve: %v", msgs)
	}
	if findCallCount != 0 {
		t.Errorf("guardFindGitBashExe não deve ser chamada fora do Windows; callCount=%d", findCallCount)
	}
}

// --------------------------------------------------------------------------
// (5) trackfw not found in Git Bash login PATH → violation
// --------------------------------------------------------------------------

// Reconciliação: afirma que, quando o Git Bash login executa com sucesso mas não encontra
// trackfw (command -v retorna não-zero, loginPath=""), guardGitBashBinaryProbe emite violation
// informando que o hook falhará com exit 127 (fail-open).
func TestGuardGitBashBinaryProbe_TrackfwNaoEncontradoNoGitBash_Violation(t *testing.T) {
	stubGitBashSeams(t,
		`C:\Program Files\Git\bin\bash.exe`,
		"", true, // ran=true, path="" → trackfw not found
		func(bin string) error { return nil },
	)

	msgs := guardGitBashBinaryProbe(`C:\tools\trackfw.exe`)
	if !hasViolation(msgs, "Git Bash") {
		t.Errorf("esperado violation com 'Git Bash', obteve: %v", msgs)
	}
	if !hasViolation(msgs, "exit 127") {
		t.Errorf("esperado violation mencionando 'exit 127', obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// (6) Bash startup failure → skip (no false-positive)
// --------------------------------------------------------------------------

// Reconciliação: afirma que, quando guardRunBashLoginProbe retorna ran=false (bash não
// iniciou ou timeout), guardGitBashBinaryProbe não emite violation — falha de startup não
// implica fail-open do hook.
func TestGuardGitBashBinaryProbe_BashNaoInicia_Skip(t *testing.T) {
	stubGitBashSeams(t,
		`C:\Program Files\Git\bin\bash.exe`,
		"", false, // ran=false → couldn't start
		func(bin string) error { return errors.New("should not be called") },
	)

	msgs := guardGitBashBinaryProbe(`C:\tools\trackfw.exe`)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs quando bash não inicia, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// guardPosixToWindowsPath unit tests
// --------------------------------------------------------------------------

// Reconciliação: afirma que guardPosixToWindowsPath converte a forma medida (/c/Users/Lab/…)
// para o caminho Windows esperado (C:\Users\Lab\…).
func TestGuardPosixToWindowsPath_FormatMedido(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/c/Users/Lab/bin/trackfw", `C:\Users\Lab\bin\trackfw`},
		{"/c/tools/trackfw", `C:\tools\trackfw`},
		{"/C/tools/Trackfw", `C:\tools\Trackfw`},
		{"/usr/bin/trackfw", "/usr/bin/trackfw"},         // not a drive letter pattern
		{"/mingw64/bin/trackfw", "/mingw64/bin/trackfw"}, // more than one char after /
	}
	for _, c := range cases {
		got := guardPosixToWindowsPath(c.in)
		if got != c.want {
			t.Errorf("guardPosixToWindowsPath(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// --------------------------------------------------------------------------
// guardStripExe unit test
// --------------------------------------------------------------------------

// Reconciliação: afirma que guardStripExe normaliza trackfw.exe e trackfw ao mesmo valor,
// permitindo a comparação case-insensitive entre o caminho do LookPath e o do Git Bash.
func TestGuardStripExe_Normalizacao(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`C:\tools\trackfw.exe`, `C:\tools\trackfw`},
		{`C:\tools\trackfw.EXE`, `C:\tools\trackfw`},
		{`C:\tools\trackfw`, `C:\tools\trackfw`},
		{"/c/tools/trackfw", "/c/tools/trackfw"},
	}
	for _, c := range cases {
		got := guardStripExe(c.in)
		if got != c.want {
			t.Errorf("guardStripExe(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// --------------------------------------------------------------------------
// ML-1A (REQ-2026-10-19) — o prazo do contexto nao e veredito da sonda
// --------------------------------------------------------------------------

// TestGuardProbeEndedByDeadline_Discriminante afirma a conclusao do ML-1A de que a
// pergunta "foi o prazo?" e respondida pelo contexto, e nao pelo tipo do erro do exec.
// Converge com resolveNetworkErr (internal/commands/netexec.go), que faz esta mesma
// pergunta antes de classificar.
func TestGuardProbeEndedByDeadline_Discriminante(t *testing.T) {
	vivo, cancelVivo := context.WithTimeout(context.Background(), time.Hour)
	defer cancelVivo()
	if guardProbeEndedByDeadline(vivo) {
		t.Fatal("contexto vivo nao deveria ser classificado como prazo estourado")
	}

	expirado, cancelExp := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelExp()
	<-expirado.Done()
	if !guardProbeEndedByDeadline(expirado) {
		t.Fatal("contexto expirado deveria ser classificado como prazo estourado")
	}
}

// guardBashParaSonda devolve um executavel que a sonda do login shell possa rodar.
//
// No Windows usa o Git Bash REAL, achado pela propria guardFindGitBashExe — e o unico
// jeito honesto de exercitar o corpo da sonda na plataforma para a qual a regra existe.
// No POSIX escreve um script stub que dorme e imprime, porque ali o shebang e confiavel.
// Devolve "" quando nao ha bash utilizavel, e o teste pula.
func guardBashParaSonda(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return guardFindGitBashExe()
	}
	stub := filepath.Join(t.TempDir(), "bash")
	corpo := "#!/bin/sh\nsleep 2\necho /stub/bin/trackfw\n"
	if err := os.WriteFile(stub, []byte(corpo), 0o700); err != nil {
		t.Fatalf("escrevendo o stub: %v", err)
	}
	return stub
}

// TestGuardRunBashLoginProbe_PrazoEstourado_NaoAfirmaNada afirma a conclusao do ML-1A de
// que o TERCEIRO caso do doc de guardRunBashLoginProbe passou a existir: prazo estourado
// devolve ("", false) — skip —, e nao ("", true), que a regra le como "trackfw ausente do
// PATH do login shell".
//
// Exercita o CORPO REAL da funcao, nao a seam: as seams sao substituidas em todos os
// outros testes deste arquivo, e e por isso que este ramo sobreviveu desde o ML-5D.
func TestGuardRunBashLoginProbe_PrazoEstourado_NaoAfirmaNada(t *testing.T) {
	bash := guardBashParaSonda(t)
	if bash == "" {
		t.Skip("nenhum bash utilizavel nesta maquina")
	}

	origTimeout := guardProbeTimeout
	t.Cleanup(func() { guardProbeTimeout = origTimeout })

	// Controle positivo: com prazo folgado o bash RODA. Sem este braco, um stub quebrado
	// ou um bash ausente daria ("", false) e o teste passaria pelo motivo errado — o
	// caminho que o ML-0A nomeou como o risco mais provavel aqui.
	//
	// Afirma `ran`, nao o caminho: se o trackfw nao estiver no PATH do login shell desta
	// maquina, `command -v` sai 1 e a funcao devolve ("", true) — que tambem prova que o
	// bash rodou, que e o discriminante.
	guardProbeTimeout = 30 * time.Second
	if _, ran := guardRunBashLoginProbe(bash); !ran {
		t.Fatalf("controle positivo: com prazo folgado o bash deveria ter rodado (ran=true)")
	}

	// O caso: prazo curto mata o processo antes de ele responder.
	guardProbeTimeout = 50 * time.Millisecond
	got, ran := guardRunBashLoginProbe(bash)
	if ran {
		t.Errorf("prazo estourado devolveu ran=true (o balde de %q); esperava ran=false, resolved=%q",
			"trackfw ausente do PATH", got)
	}
	if got != "" {
		t.Errorf("prazo estourado devolveu resolved=%q; esperava vazio", got)
	}
}

// TestGuardBinaryProbeOnce_PrazoDoGuardHelp_NaoAcusaOBinario afirma a conclusao do ML-1A
// de que o segundo sitio — a sonda de `trackfw guard --help` — tambem deixou de ler prazo
// estourado como veredito: o chamador pula em vez de dizer que o binario nao tem o
// subcomando guard.
func TestGuardBinaryProbeOnce_PrazoDoGuardHelp_NaoAcusaOBinario(t *testing.T) {
	origLookup := guardLookupBinary
	origRun := guardRunProbe
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
	})
	guardLookupBinary = func(string) (string, error) { return "/usr/local/bin/trackfw", nil }

	// Prazo estourado: o sentinela viaja no erro, e nada e afirmado.
	guardRunProbe = func(string) error {
		return errors.Join(errGuardProbeDeadline, errors.New("signal: killed"))
	}
	if msgs := guardBinaryProbeOnce(false); len(msgs) != 0 {
		t.Errorf("prazo estourado produziu %d violacao(oes): %v", len(msgs), msgs)
	}

	// Controle positivo, e e ele que impede o conserto de ser "silenciar tudo":
	// erro comum, sem o sentinela, continua acusando.
	guardRunProbe = func(string) error { return errors.New("exit status 1") }
	msgs := guardBinaryProbeOnce(false)
	if len(msgs) != 1 {
		t.Fatalf("erro sem prazo deveria acusar 1 violacao, obtive %d: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], "does not have the guard subcommand") {
		t.Errorf("mensagem inesperada: %q", msgs[0])
	}
}
