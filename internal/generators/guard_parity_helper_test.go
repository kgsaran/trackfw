// guard_parity_helper_test.go — ML-1C: paridade .sh ↔ Go (AC4).
//
// Responsabilidades deste arquivo:
//
//  1. TestMain — compila o binário do trackfw uma vez e registra o cleanup.
//  2. Funções de conveniência usadas pelos runners modificados:
//     - compiledGuardBinary(t) → caminho absoluto do binário já compilado.
//     - runGuardBinaryGitBranch(…) → roda "trackfw guard git-branch [--command …]".
//     - runGuardBinaryCredential(…) → roda "trackfw guard credential [--global]".
//     - assertGuardParity(…) → compara (rc, stdout, stderr) e reporta divergência.
//  3. Teste das fixtures congeladas (sha256 pinado):
//     - TestGuardShReferenceFixtures_Sha256 — reprova se um ML futuro mudar as fixtures.
//
// ML-2A: os scripts .sh ao vivo viraram invólucros finos (exec trackfw guard …). Os braços
// bash da paridade continuam usando as fixtures CONGELADAS do ML-1C
// (testdata/guard-sh-reference/), por isso a comparação continua significativa — ela afirma
// que o comportamento do novo binário Go é equivalente ao dos scripts originais completos.
// O TestGuardShReferenceFixtures_Sha256 reprova se as fixtures forem alteradas.
package generators

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Variáveis de pacote — binário compilado uma vez para toda a execução.
// ---------------------------------------------------------------------------

var (
	guardBinaryOnce sync.Once
	guardBinaryDir  string // diretório persistente; apagado no TestMain cleanup
	guardBinaryPath string // caminho absoluto do binário compilado
	guardBinaryErr  error  // erro de compilação, se houver
)

// moduleRootForTests devolve a raiz do módulo Go deduzida a partir do caminho
// deste arquivo-fonte.  Usa runtime.Caller para ser imune a os.Chdir() que os
// testes de fixture chamam.
func moduleRootForTests() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller falhou")
	}
	// thisFile = …/trackfw/internal/generators/guard_parity_helper_test.go
	// subir 3 níveis: generators → internal → trackfw
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

// testdataGuardRefDir devolve o caminho absoluto de
// internal/generators/testdata/guard-sh-reference/.
func testdataGuardRefDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller falhou")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata", "guard-sh-reference")
}

// buildGuardBinary compila o binário uma vez. Chamado por TestMain.
func buildGuardBinary() {
	guardBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "trackfw-guard-bin-*")
		if err != nil {
			guardBinaryErr = fmt.Errorf("MkdirTemp: %w", err)
			return
		}
		guardBinaryDir = dir

		binName := "trackfw"
		if runtime.GOOS == "windows" {
			binName = "trackfw.exe"
		}
		guardBinaryPath = filepath.Join(dir, binName)

		modRoot := moduleRootForTests()
		cmd := exec.Command("go", "build", "-o", guardBinaryPath, "./cmd/trackfw")
		cmd.Dir = modRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			guardBinaryErr = fmt.Errorf("go build falhou:\n%s\n%w", out, err)
		}
	})
}

// cleanupGuardBinary apaga o diretório do binário. Chamado por TestMain.
func cleanupGuardBinary() {
	if guardBinaryDir != "" {
		_ = os.RemoveAll(guardBinaryDir)
	}
}

// injectGuardBinaryPath prepends the directory containing the compiled trackfw binary
// to the PATH variable in baseEnv, so thin-wrapper scripts can call `trackfw guard …`
// using the binary compiled by TestMain rather than any system-installed trackfw.
// ML-2A: required after scripts became thin wrappers that invoke the binary.
func injectGuardBinaryPath(t *testing.T, baseEnv []string) []string {
	t.Helper()
	binDir := filepath.Dir(compiledGuardBinary(t))
	result := make([]string, 0, len(baseEnv))
	pathInjected := false
	for _, e := range baseEnv {
		if strings.HasPrefix(e, "PATH=") {
			result = append(result, "PATH="+binDir+string(filepath.ListSeparator)+strings.TrimPrefix(e, "PATH="))
			pathInjected = true
		} else {
			result = append(result, e)
		}
	}
	if !pathInjected {
		result = append(result, "PATH="+binDir)
	}
	return result
}

// compiledGuardBinary devolve o caminho do binário compilado.
// Chama t.Fatalf se a compilação falhou.
func compiledGuardBinary(t *testing.T) string {
	t.Helper()
	if guardBinaryErr != nil {
		t.Fatalf("binário do trackfw não compilou: %v", guardBinaryErr)
	}
	return guardBinaryPath
}

// TestMain — compila o binário uma vez antes de todos os testes do pacote.
func TestMain(m *testing.M) {
	buildGuardBinary()
	code := m.Run()
	cleanupGuardBinary()
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Auxiliares de execução do binário.
// ---------------------------------------------------------------------------

// runGuardBinaryGitBranch executa "trackfw guard git-branch [--command …]".
//   - dir: cwd do processo (mesmo do braço bash).
//   - args: se não vazio, passado como --command "<args joined by space>".
//   - stdin: conteúdo injetado no stdin do binário.
//   - env: slice de env para o processo (nil = herdar do pai).
func runGuardBinaryGitBranch(t *testing.T, dir string, args []string, stdin string, env []string) (exitCode int, stdout, stderr string) {
	t.Helper()

	binPath := compiledGuardBinary(t)
	cmdArgs := []string{"guard", "git-branch"}
	if len(args) > 0 {
		// Bash usa "$*" para juntar args separados por IFS (espaço por padrão).
		cmdArgs = append(cmdArgs, "--command", strings.Join(args, " "))
	}
	cmd := exec.Command(binPath, cmdArgs...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	if env != nil {
		cmd.Env = env
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err == nil {
		return 0, outBuf.String(), errBuf.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), outBuf.String(), errBuf.String()
	}
	t.Fatalf("erro executando binário guard git-branch: %v (stderr: %s)", err, errBuf.String())
	return -1, "", ""
}

// runGuardBinaryCredential executa "trackfw guard credential [--global]".
//   - dir: cwd do processo.
//   - stdin: conteúdo injetado no stdin.
//   - global: se true, passa --global.
func runGuardBinaryCredential(t *testing.T, dir, stdin string, global bool) (exitCode int, stdout, stderr string) {
	t.Helper()

	binPath := compiledGuardBinary(t)
	cmdArgs := []string{"guard", "credential"}
	if global {
		cmdArgs = append(cmdArgs, "--global")
	}
	cmd := exec.Command(binPath, cmdArgs...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err == nil {
		return 0, outBuf.String(), errBuf.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), outBuf.String(), errBuf.String()
	}
	t.Fatalf("erro executando binário guard credential: %v (stderr: %s)", err, errBuf.String())
	return -1, "", ""
}

// runGuardBinaryGitBranchWithPipe executa "trackfw guard git-branch [--command …]"
// com um pipe real no stdin, espelhando o padrão de runGuardWithPipe.
// Retorna (exitCode, stdout, stderr, writeErr).
func runGuardBinaryGitBranchWithPipe(t *testing.T, dir string, args []string, writeFn func(w *os.File) error) (exitCode int, stdout, stderr string, writeErr error) {
	t.Helper()

	binPath := compiledGuardBinary(t)
	cmdArgs := []string{"guard", "git-branch"}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, "--command", strings.Join(args, " "))
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (Go arm): %v", err)
	}

	cmd := exec.Command(binPath, cmdArgs...)
	cmd.Dir = dir
	cmd.Stdin = pr
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("erro iniciando binário guard git-branch: %v", err)
	}
	_ = pr.Close()

	done := make(chan error, 1)
	go func() { done <- writeFn(pw) }()

	waitErr := cmd.Wait()
	_ = pw.Close()

	select {
	case writeErr = <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("escritor (braço Go) não terminou em 15s")
	}

	exitCode = 0
	if waitErr != nil {
		exitErr, ok := waitErr.(*exec.ExitError)
		if !ok {
			t.Fatalf("erro executando binário guard git-branch: %v (stderr: %s)", waitErr, errBuf.String())
		}
		exitCode = exitErr.ExitCode()
	}
	return exitCode, outBuf.String(), errBuf.String(), writeErr
}

// ---------------------------------------------------------------------------
// Comparação de paridade.
// ---------------------------------------------------------------------------

// awkDiagnosticNormalizations is a fixed base list that strips the awk-extractor
// diagnostic lines emitted by the bash script to stderr when jq is unavailable.
// The bash awk fallback writes "trackfw git-branch-guard: extrator JSON (sem jq): <code>"
// for each parse error; Go's encoding/json does not emit these.
// Stripping them before comparison makes the two outputs comparable for the
// decision/rc fields without loosening those comparisons.
var awkDiagnosticNormalizations = []normalization{
	{"trackfw git-branch-guard: extrator JSON (sem jq): truncated_escape\n", ""},
	{"trackfw git-branch-guard: extrator JSON (sem jq): unterminated_string\n", ""},
	{"trackfw git-branch-guard: extrator JSON (sem jq): incomplete_unicode\n", ""},
	{"trackfw git-branch-guard: extrator JSON (sem jq): invalid_unicode_hex\n", ""},
	{"trackfw git-branch-guard: extrator JSON (sem jq): nul_in_value\n", ""},
	{"trackfw git-branch-guard: extrator JSON (sem jq): invalid_escape\n", ""},
}

// c22AwkNulSentinel is the raw awk diagnostic line written to stderr when the
// awk extractor encounters a NUL byte in the command value.  It is written to
// the raw bash stderr BEFORE awkDiagnosticNormalizations strips it, so checking
// shStderr for this sentinel (before normalization) is reliable.
//
// Roadmap ML-1C, grupo C, C22: o Go segue o braço com jq; o awk do .sh diverge
// só no texto (reasonIndecodeable vs reasonNUL), não na decisão (rc=2).
const c22AwkNulSentinel = "trackfw git-branch-guard: extrator JSON (sem jq): nul_in_value\n"

// c22IndecodeableReason is the REASON text the bash awk path emits for C22.
// Mirrors internal/guard/gitbranch.go:reasonIndecodeable (unexported).
const c22IndecodeableReason = "trackfw git-branch-guard: RECUSADO — o extrator JSON (sem jq) encontrou erro ao decodificar o campo de comando: string nao terminada, escape invalido ou NUL. O guard recusa em vez de aprovar um payload que nao pode ser lido com seguranca."

// c22NULReason is the REASON text the Go path (and the bash jq path) emits for C22.
// Mirrors internal/guard/gitbranch.go:reasonNUL (unexported).
// Note: \\u0000 in this Go source is the 6-byte literal sequence \u0000 in the output.
const c22NULReason = "trackfw git-branch-guard: RECUSADO — o comando contem NUL (\\u0000) e nao pode ser interpretado com seguranca pelo shell. O guard recusa em vez de executar um comando corrompido."


// assertGuardParity compara os resultados bash e Go e reporta divergência.
// Se divergirem, o teste REPROVA — não é normalizado.
// normalizations lista as substituições aplicadas às strings de saída antes
// da comparação (ex: trocar caminhos de temp dir específicos por <TMPDIR>).
// O conjunto base awkDiagnosticNormalizations é sempre aplicado automaticamente
// (sem precisar ser passado pelo chamador) — ele remove linhas diagnósticas do
// extrator awk do stderr do bash que não têm equivalente no Go.
func assertGuardParity(t *testing.T, testName string, shRC int, shStdout, shStderr string, goRC int, goStdout, goStderr string, normalizations []normalization) {
	t.Helper()

	// Merge base awk normalizations with caller-provided ones.
	allNorms := make([]normalization, 0, len(awkDiagnosticNormalizations)+len(normalizations)+1)
	allNorms = append(allNorms, awkDiagnosticNormalizations...)
	allNorms = append(allNorms, normalizations...)

	// Roadmap ML-1C, grupo C, C22: o Go segue o braço com jq; o awk do .sh diverge
	// só no texto (reasonIndecodeable vs reasonNUL), não na decisão (rc=2).
	// Quando o stderr bash RAW contém a linha diagnóstica nul_in_value do awk,
	// normaliza o REASON indecodeable para o REASON NUL antes da comparação.
	// Estritamente limitado: somente C22 no modo sem jq aciona esta normalização
	// (esperado: 1 acionamento por subteste C22/WithoutJQ e 1 por WithJQ internamente).
	if strings.Contains(shStderr, c22AwkNulSentinel) {
		allNorms = append(allNorms, normalization{
			Old: c22IndecodeableReason,
			New: c22NULReason,
		})
	}

	shOut := applyNormalizations(shStdout, allNorms)
	shErr := applyNormalizations(shStderr, allNorms)
	goOut := applyNormalizations(goStdout, allNorms)
	goErr := applyNormalizations(goStderr, allNorms)

	if shRC != goRC || shOut != goOut || shErr != goErr {
		t.Errorf("[paridade .sh↔Go] %s\n"+
			"  bash  rc=%d stdout=%q stderr=%q\n"+
			"  go    rc=%d stdout=%q stderr=%q\n"+
			"  normalizações: %v",
			testName, shRC, shOut, shErr, goRC, goOut, goErr, allNorms)
	}
}

// normalization é um par (old, new) aplicado como strings.ReplaceAll às saídas.
type normalization struct {
	Old, New string
}

func applyNormalizations(s string, ns []normalization) string {
	for _, n := range ns {
		s = strings.ReplaceAll(s, n.Old, n.New)
	}
	return s
}

// isCurrentGuardScript verifica se scriptPath aponta para um dos scripts de guarda
// canônicos (nome de arquivo esperado).  Retorna false para scripts históricos como
// old-guard.sh, que são exercitados por TestGitBranchGuardAwk_ProvaDeMordida.
func isCurrentGuardScript(scriptPath string) bool {
	base := filepath.Base(scriptPath)
	return base == "trackfw-git-branch-guard.sh" || base == "trackfw-credential-guard.sh"
}

// isGlobalCredentialScript detecta se scriptPath é a variante global do credential
// guard (vive em /.trackfw/scripts/ dentro do HOME de fixture).
func isGlobalCredentialScript(scriptPath string) bool {
	// A variante global é gravada em <fakeHome>/.trackfw/scripts/trackfw-credential-guard.sh
	return strings.Contains(scriptPath, string(filepath.Separator)+".trackfw"+string(filepath.Separator)+"scripts"+string(filepath.Separator))
}

// ---------------------------------------------------------------------------
// Teste das fixtures congeladas — sha256 pinado.
// ---------------------------------------------------------------------------

// TestGuardShReferenceFixtures_Sha256 afirma que as três fixtures congeladas em
// testdata/guard-sh-reference/ correspondem byte-a-byte aos scripts de guarda
// vigentes no momento do ML-1C (2026-10-04).
//
// Após o ML-2A, quando os .sh virarem `exec trackfw guard …`, estes hashes vão
// divergir — e é CORRETO que este teste reprove: o ML-2A deve então atualizar
// as fixtures e os hashes abaixo (parte da sua entrega).
//
// AFIRMA: que as fixtures congeladas não foram alteradas desde o ML-1C, garantindo
// que o braço bash da paridade continua rodando o script original (completo) e não
// um invólucro que chama Go.
func TestGuardShReferenceFixtures_Sha256(t *testing.T) {
	// ML-1C: hashes of the original full .sh scripts, frozen at the ML-1C commit.
	// These must NOT change — the frozen fixture is the bash parity arm's reference.
	// The live scripts/trackfw-*.sh are now thin wrappers (ML-2A); the fixtures remain
	// the original full scripts so the parity comparison stays meaningful.
	expected := map[string]string{
		"git-branch-guard.sh":         "f51ee1f93a168a19a5e03090f38543409702c03044f7807f3ff5bb3a1f3232a3",
		"credential-guard-project.sh": "e73d6502f851fee4ca304c1e8e8a60485f1339714734fb09d89a514dea65a4f7",
		"credential-guard-global.sh":  "af21a5772a2edeba738db454771e8347b6300b4272a17c447f68a921b9c3cc43",
	}

	dir := testdataGuardRefDir()
	for name, want := range expected {
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("fixture %s: não abrível: %v", name, err)
			continue
		}
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			t.Errorf("fixture %s: leitura falhou: %v", name, err)
			continue
		}
		_ = f.Close()
		got := hex.EncodeToString(h.Sum(nil))
		if got != want {
			t.Errorf("fixture %s: sha256 divergiu\n  want %s\n  got  %s\n"+
				"  → o script mudou. Se o ML-2A foi aplicado (script virou invólucro),\n"+
				"    regenere as fixtures e atualize os hashes aqui.", name, want, got)
		}
	}
}
