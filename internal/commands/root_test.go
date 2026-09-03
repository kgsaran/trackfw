package commands

// root_test.go — trava o comportamento de comandos desconhecidos após a remoção
// do subsistema de plugins (ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-
// vez-de-gate-de-binario-de-terceiro.md, D3). O trackfw não baixa, gerencia nem
// executa código de terceiro. Qualquer comando não reconhecido deve falhar com a
// mensagem CANÔNICA compartilhada pelos 3 CLIs (pinada em docs/cli-parity.md e
// coberta byte-a-byte por scripts/check-unknown-command-parity.sh):
//
//	Error: unknown command "x" for "trackfw"
//	Did you mean "validate"?
//	Run 'trackfw --help' for usage.
//
// A parte unitária (suggestCommand/levenshteinDistance) roda in-process contra
// o *cobra.Command construído por newRootCmd(). A falsificação com um binário
// real trackfw-vaildate no PATH precisa do processo real (os.Exit dentro de
// Execute() mataria o `go test`), então usa exec.Command contra o binário
// compilado — mesmo padrão de internal/commands/barrier_contract_test.go.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestSuggestCommand_ClosTypo_SuggestsSingleClosestMatch(t *testing.T) {
	root := newRootCmd()
	candidates := unknownCommandCandidates(root)

	suggestion, found := suggestCommand("vaildate", candidates)
	if !found {
		t.Fatalf("esperava sugestão para 'vaildate', obteve nenhuma")
	}
	if suggestion != "validate" {
		t.Errorf("esperava sugestão 'validate', obteve %q", suggestion)
	}
}

func TestSuggestCommand_NoCloseMatch_NoSuggestion(t *testing.T) {
	root := newRootCmd()
	candidates := unknownCommandCandidates(root)

	_, found := suggestCommand("zzzzzzzzzz-nao-existe", candidates)
	if found {
		t.Errorf("não esperava sugestão para um comando sem vizinho próximo")
	}
}

// TestUnknownCommandCandidates_ExcludesCompletion — "completion" é auto-
// registrado pelo cobra e não tem equivalente em npm/pypi; incluí-lo quebraria
// a paridade do critério de sugestão entre os 3 CLIs (docs/cli-parity.md
// "Candidate set — completion is Go-only").
func TestUnknownCommandCandidates_ExcludesCompletion(t *testing.T) {
	root := newRootCmd()
	// Força o cobra a registrar o comando "completion" auto-gerado, exatamente
	// como acontece dentro de ExecuteC antes do erro de comando desconhecido
	// ser produzido.
	root.InitDefaultCompletionCmd()

	for _, name := range unknownCommandCandidates(root) {
		if name == "completion" {
			t.Fatalf("unknownCommandCandidates não deve incluir 'completion' (Go-only, sem equivalente em npm/pypi)")
		}
	}
}

func TestFormatUnknownCommandError_CanonicalMessage_WithSuggestion(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"vaildate"})
	root.SilenceErrors = true
	root.SilenceUsage = true

	_, err := root.ExecuteC()
	if err == nil {
		t.Fatal("esperava erro para comando 'vaildate' desconhecido")
	}

	msg, ok := formatUnknownCommandError(root, err)
	if !ok {
		t.Fatalf("formatUnknownCommandError não reconheceu o erro de comando desconhecido: %v", err)
	}

	want := "Error: unknown command \"vaildate\" for \"trackfw\"\n" +
		"Did you mean \"validate\"?\n" +
		"Run 'trackfw --help' for usage."
	if msg != want {
		t.Errorf("mensagem canônica não bate byte-a-byte.\nesperado: %q\nobteve:   %q", want, msg)
	}
}

func TestFormatUnknownCommandError_CanonicalMessage_NoSuggestion(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"zzzzzzzzzz-nao-existe"})
	root.SilenceErrors = true
	root.SilenceUsage = true

	_, err := root.ExecuteC()
	if err == nil {
		t.Fatal("esperava erro para comando desconhecido")
	}

	msg, ok := formatUnknownCommandError(root, err)
	if !ok {
		t.Fatalf("formatUnknownCommandError não reconheceu o erro de comando desconhecido: %v", err)
	}

	want := "Error: unknown command \"zzzzzzzzzz-nao-existe\" for \"trackfw\"\n" +
		"Run 'trackfw --help' for usage."
	if msg != want {
		t.Errorf("mensagem canônica não bate byte-a-byte.\nesperado: %q\nobteve:   %q", want, msg)
	}
}

// TestFormatUnknownCommandError_PluginsIsGone — "plugins" não existe mais como
// comando (AC1/D1 do ADR); deve produzir a MESMA mensagem canônica de qualquer
// outro comando desconhecido, sem caso especial.
func TestFormatUnknownCommandError_PluginsIsGone(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"plugins"})
	root.SilenceErrors = true
	root.SilenceUsage = true

	_, err := root.ExecuteC()
	if err == nil {
		t.Fatal("esperava erro: 'plugins' não deve mais existir como comando")
	}

	msg, ok := formatUnknownCommandError(root, err)
	if !ok {
		t.Fatalf("formatUnknownCommandError não reconheceu o erro de comando desconhecido: %v", err)
	}
	want := "Error: unknown command \"plugins\" for \"trackfw\"\n" +
		"Run 'trackfw --help' for usage."
	if msg != want {
		t.Errorf("mensagem canônica não bate byte-a-byte.\nesperado: %q\nobteve:   %q", want, msg)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Falsificação (P4): binário real no PATH — requer o binário compilado e
// exec.Command, mesmo padrão de barrier_contract_test.go.
// ────────────────────────────────────────────────────────────────────────────

var (
	rootBinaryOnce sync.Once
	rootBinaryPath string
	rootBinaryErr  error
)

func rootFindProjectRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller unavailable")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found: could not determine project root")
		}
		dir = parent
	}
}

func rootBinary(t *testing.T) string {
	t.Helper()
	rootBinaryOnce.Do(func() {
		projRoot := rootFindProjectRoot(t)
		dir, err := os.MkdirTemp("", "trackfw-root-bin-")
		if err != nil {
			rootBinaryErr = err
			return
		}
		bin := filepath.Join(dir, testBinaryName("trackfw"))
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/trackfw")
		cmd.Dir = projRoot
		if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
			rootBinaryErr = fmt.Errorf("go build ./cmd/trackfw failed: %v\n%s", buildErr, out)
			return
		}
		rootBinaryPath = bin
	})
	if rootBinaryErr != nil {
		t.Fatalf("could not build trackfw binary: %v", rootBinaryErr)
	}
	return rootBinaryPath
}

// TestUnknownCommand_NeverExecutesExternalBinary — coloca um executável REAL
// trackfw-vaildate no PATH, que imprime um marcador distintivo, e prova que o
// trackfw nunca o executa. É o vetor exato que o fallback de execução de
// plugin removido (D3 do ADR) costumava abrir: `trackfw vaildate` rodava
// `trackfw-vaildate` se existisse no PATH.
func TestUnknownCommand_NeverExecutesExternalBinary(t *testing.T) {
	bin := rootBinary(t)

	fakeBinDir := t.TempDir()
	// No Windows um shebang não é executável e um arquivo sem extensão do
	// PATHEXT não é sequer encontrado pelo LookPath: o plugin falso não
	// poderia rodar nem que o trackfw tentasse, e a asserção passaria por
	// vacuidade. `.bat` está no PATHEXT default, então a prova continua real.
	fakeName := "trackfw-vaildate"
	script := "#!/bin/sh\necho EXECUTOU_PLUGIN_MALICIOSO\n"
	if runtime.GOOS == "windows" {
		fakeName = "trackfw-vaildate.bat"
		script = "@echo EXECUTOU_PLUGIN_MALICIOSO\r\n"
	}
	fakeBinPath := filepath.Join(fakeBinDir, fakeName)
	if err := os.WriteFile(fakeBinPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	cmd := exec.Command(bin, "vaildate")
	cmd.Env = append(os.Environ(), "PATH="+fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, runErr := cmd.CombinedOutput()

	if strings.Contains(string(out), "EXECUTOU_PLUGIN_MALICIOSO") {
		t.Fatalf("binário externo trackfw-vaildate foi executado — vetor de plugin reintroduzido; saída: %s", out)
	}

	if runErr == nil {
		t.Fatal("esperava exit != 0 para comando desconhecido 'vaildate'")
	}
	if exitErr, ok := runErr.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		t.Errorf("esperava exit code 1, obteve erro: %v", runErr)
	}

	if !strings.Contains(string(out), `Did you mean "validate"?`) {
		t.Errorf("esperava sugestão 'validate' mesmo com o binário externo presente no PATH; saída: %s", out)
	}
}

// TestBareInvocation_ExitZero_HelpOnStdout — trackfw sem argumento é uso
// legítimo (pedir ajuda), não um comando desconhecido: exit 0, help em
// stdout, stderr vazio. Decisão do arquiteto no ML-1C
// (ROADMAP-2026-08-16-higiene-sete-debitos-...), que unificou o Node.js
// (antes exit 1/stderr, default do commander) para este comportamento — Go e
// Python já eram assim. Roda o binário real (não newRootCmd() in-process)
// para exercitar exatamente o mesmo caminho de Execute() que os testes de
// comando desconhecido acima, e para poder observar exit code e stream
// separadamente como um processo externo o faria.
func TestBareInvocation_ExitZero_HelpOnStdout(t *testing.T) {
	bin := rootBinary(t)

	cmd := exec.Command(bin)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	if runErr != nil {
		t.Fatalf("esperava exit 0 para 'trackfw' sem argumento, obteve erro: %v (stderr: %s)", runErr, stderr.String())
	}
	if stderr.String() != "" {
		t.Errorf("esperava stderr vazio, obteve: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("esperava help contendo \"Usage:\" em stdout, obteve: %q", stdout.String())
	}
}
