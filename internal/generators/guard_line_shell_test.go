package generators

// guard_line_shell_test.go — REQ-2026-10-20 (ML-1A / ML-1B).
//
// TestGuardLineShellMatrix afirma, por subprocess real, que as três linhas de guard geradas pelo
// trackfw (guardGitBranchCmdPSPOSIX, guardCredentialCmdPSPOSIX,
// guardCredentialGlobalCmdPSPOSIX) comportam-se corretamente sob sh, bash, zsh, zsh -f,
// dash, zsh -o errexit, zsh -o nounset e zsh -o cshnullglob:
//
//   - FAKE_RC=0 (allow) → exit 0
//   - FAKE_RC=2 (deny)  → exit 2 e stderr contém "motivo-deny-fake"
//   - FAKE_RC=1 (deny não-canônico) → exit 2
//   - trackfw ausente do PATH → exit 2
//   - quando o stub roda, stdin.out é idêntico byte a byte ao JSON enviado
//
// Braço do defeito (ML-1A): a forma ANTIGA (sem o prefixo "emulate -R sh 2>…; ") sai com
// exit ≠ 2 sob zsh quando FAKE_RC=2, provando que o teste distingue defeito de correção.
//
// Braço de regressão (ML-1B): TestGuardLineShellMatrix_UnsetoptInsuficiente demonstra que a
// forma alternativa "unsetopt EQUALS NOMATCH" é insuficiente sob zsh -o nounset (fail-open).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

// stdinPayload é o JSON que Claude Code entregaria ao hook de PreToolUse.
const stdinPayload = `{"tool_name":"Bash","tool_input":{"command":"git add -A"}}`

// setEuPrefix é o prefixo que o REQ-2026-11-13 (ML-1A) adicionou para corrigir a saída 127 sob
// errexit/nounset herdado. `set +eu` é um builtin especial POSIX — desativa errexit (+e) e nounset (+u)
// antes que qualquer comando não encontrado possa encerrar o shell. O redirecionamento swallowa eventuais
// mensagens de erro (nenhuma na prática; incluso por simetria com o emulate).
const setEuPrefix = "set +eu 2>${null-/dev/null}; "

// emulatePrefix é o prefixo que o REQ-2026-10-20 (ML-1B) adicionou para corrigir os dois pontos de aborto
// do zsh. Usa `emulate -R sh` para desativar todas as opções zsh extras (EQUALS, NOMATCH, nounset, errexit,
// cshnullglob…) antes de rodar a linha polyglota. Em sh/bash/dash é command-not-found com stderr swallowed.
const emulatePrefix = "emulate -R sh 2>${null-/dev/null}; "

// unsetoptOldPrefix é a forma alternativa que o ML-1A usou brevemente e que provou ser insuficiente.
// Desliga EQUALS e NOMATCH, mas não protege contra zsh -o nounset nem zsh -o errexit nem cshnullglob.
// Nunca chegou a ser publicada; mantida aqui só como referência para TestGuardLineShellMatrix_UnsetoptInsuficiente.
const unsetoptOldPrefix = "unsetopt EQUALS NOMATCH 2>${null-/dev/null}; "

// prazos e delays das chamadas PS (REQ-2026-11-01, REQ-2026-11-06, AC1, AC2, AC3, D3, P-5).
const (
	// psCallDeadlineFactor é o multiplicador do aquecimento para o prazo de cada chamada PS.
	// Fator 4 é palpite — ADR-2026-11-06 D3.
	psCallDeadlineFactor = 4
	// psCallDeadlineFloor é o prazo mínimo de cada chamada PS; fator 4 é palpite — ADR-2026-11-06 D3.
	psCallDeadlineFloor = 15 * time.Second
	// psCallDeadlineCeil é o prazo máximo de cada chamada PS; fator 4 é palpite — ADR-2026-11-06 D3.
	psCallDeadlineCeil = 60 * time.Second
	psWarmupTimeout    = 60 * time.Second // prazo do aquecimento por binário (AC1)
	psWaitDelay        = 5 * time.Second  // WaitDelay para drenar pipes após kill de grandchild (P-5)
	// psStallThreshold é o limiar de duração de sonda acima do qual a máquina é considerada
	// parada (e não o guard). Palpite — ADR-2026-11-06 D3, segunda emenda.
	// psCallDeadlineFloor / 2 = 7.5 s.
	psStallThreshold = psCallDeadlineFloor / 2
)

// errPSTimeout é o sentinela devolvido por psWarmup e psProbe quando o prazo é excedido.
// Permite distinguir timeout de outros erros via errors.Is nos pontos de decisão
// (psGuardedCall, psWarmupRetryLogic). ADR-2026-11-06 D3, segunda emenda.
var errPSTimeout = errors.New("ps timeout")

// psCallDeadline calcula o prazo de cada chamada PS a partir do tempo de aquecimento medido.
// Retorna clamp(psCallDeadlineFactor × warmup, psCallDeadlineFloor, psCallDeadlineCeil).
func psCallDeadline(warmup time.Duration) time.Duration {
	d := time.Duration(psCallDeadlineFactor) * warmup
	if d < psCallDeadlineFloor {
		return psCallDeadlineFloor
	}
	if d > psCallDeadlineCeil {
		return psCallDeadlineCeil
	}
	return d
}

// psTimeoutRegistry guarda, por binário, o nome do primeiro teste que estourou o prazo
// E cuja nova tentativa também estourou (ou que teve sonda rápida, indicando guard travado).
// Também rastreia o orçamento de nova tentativa por binário (ADR D3, segunda emenda): cada
// binário pode fazer no máximo uma nova tentativa por processo (ML-2E).
// Usada pelo mecanismo fail-fast por binário (AC3, emenda D3).
type psTimeoutRegistry struct {
	mu         sync.Mutex
	m          map[string]string // psBin → nome do teste que registrou o estouro definitivo
	retryUsed  map[string]string // psBin → nome do teste que consumiu o orçamento de nova tentativa
}

// register marca psBin como tendo estourado o prazo em testName.
// Idempotente: a segunda chamada não sobrescreve o culpado original.
func (r *psTimeoutRegistry) register(psBin, testName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[psBin]; !ok {
		r.m[psBin] = testName
	}
}

// query retorna (culprit, true) se psBin foi marcado; ("", false) caso contrário.
func (r *psTimeoutRegistry) query(psBin string) (culprit string, timed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	culprit, timed = r.m[psBin]
	return
}

// consumeRetry marca o orçamento de nova tentativa de psBin como gasto em testName.
// Retorna true se o orçamento estava disponível (esta chamada o consome); false se já foi gasto.
// Lazy-init de retryUsed para compatibilidade com testes que constroem o registro sem o campo.
func (r *psTimeoutRegistry) consumeRetry(psBin, testName string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.retryUsed == nil {
		r.retryUsed = map[string]string{}
	}
	if _, ok := r.retryUsed[psBin]; ok {
		return false
	}
	r.retryUsed[psBin] = testName
	return true
}

// retryBudget retorna (culprit, true) se o orçamento de nova tentativa de psBin foi gasto;
// ("", false) caso contrário. Leitura de mapa nil é segura em Go (retorna zero value).
func (r *psTimeoutRegistry) retryBudget(psBin string) (culprit string, spent bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	culprit, spent = r.retryUsed[psBin]
	return
}

// psTimeouts é o registro global de binários PS que estouraram o prazo nesta execução.
var psTimeouts = &psTimeoutRegistry{m: map[string]string{}, retryUsed: map[string]string{}}

// psProbe executa <psBin> -NoProfile -NonInteractive -Command "exit 0" uma vez, sem cache.
// Retorna (duração, erro). Destinada a verificar se a máquina está responsiva após um estouro.
// Nunca reutiliza o sync.OnceValues do psWarmup — cada chamada é nova.
// Em timeout, devolve um erro que envolve errPSTimeout para detecção via errors.Is.
// ADR-2026-11-06 D3, segunda emenda.
func psProbe(psBin string, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, psBin, "-NoProfile", "-NonInteractive", "-Command", "exit 0")
	runErr := cmd.Run()
	d := time.Since(start)
	if ctx.Err() == context.DeadlineExceeded {
		return d, fmt.Errorf("timeout após %.1fs: %w", d.Seconds(), errPSTimeout)
	}
	if runErr != nil {
		return d, runErr
	}
	return d, nil
}

// psGuardedCall executa run(deadline) para psBin, aplicando fail-fast e registro de estouro.
// probe é injetável para testes; em produção, chama psProbe(psBin, psCallDeadlineCeil).
//
// Retorna (result, probeDur, fatalMsg):
//   - probeDur > 0 se a sonda foi executada (para log do chamador com t.Logf)
//   - fatalMsg != "" → o chamador deve invocar t.Fatalf(fatalMsg)
//
// Lógica após estouro de run (ADR D3, segunda emenda):
//   - sonda lenta (duração >= psStallThreshold) ou sonda com timeout (errors.Is errPSTimeout):
//     verifica orçamento de nova tentativa por binário (ML-2E):
//     - orçamento disponível: consome, repete run uma vez com prazo = psCallDeadline(probeDur).
//       Se passar → sem fatal, retorna probeDur para log.
//       Se estourar → registra psBin e fatal citando chamada, sonda e nova tentativa.
//     - orçamento esgotado: registra psBin e fatal imediato citando a chamada que gastou o orçamento
//       e a duração da sonda. run chamado 1×.
//   - sonda rápida (duração < psStallThreshold, sem timeout): quem travou foi o guard, não a máquina.
//     Registra psBin e fatal imediato com "o guard travou, não a máquina". run chamado 1×.
//   - sonda com erro não-timeout (crash rápido): tratado como sonda rápida (probe não foi lenta).
//
// Fail-fast por binário: se psBin já foi registrado, run NÃO é chamado.
//
// NOTA DE PIOR CASO (ADR D3, segunda emenda): com orçamento de uma nova tentativa por binário,
// o pior caso é: aquecimento (60+60) + chamada (60) + sonda (60) + nova tentativa (60) +
// mais uma chamada (60) + sonda (60) até o fail-fast = cerca de 420 s por binário.
// Com dois binários, cerca de 840 s, abaixo do -timeout 20m do CI.
func psGuardedCall(
	reg *psTimeoutRegistry,
	psBin, testName string,
	warmup time.Duration,
	probe func() (time.Duration, error),
	run func(time.Duration) psExecResult,
) (psExecResult, time.Duration, string) {
	// Fail-fast: binário já registrado — não executar.
	if culprit, timed := reg.query(psBin); timed {
		return psExecResult{}, 0, fmt.Sprintf("não executado: %s já estourou o prazo em %s", psBin, culprit)
	}

	deadline := psCallDeadline(warmup)
	r := run(deadline)

	if !r.timedOut {
		return r, 0, ""
	}

	// Primeira chamada estourou. Executar sonda para decidir.
	probeDur, probeErr := probe()

	isSlow := errors.Is(probeErr, errPSTimeout) || (probeErr == nil && probeDur >= psStallThreshold)

	if isSlow {
		probeDurMsg := fmt.Sprintf("%.2fs", probeDur.Seconds())
		if errors.Is(probeErr, errPSTimeout) {
			probeDurMsg = fmt.Sprintf("timeout(%.1fs)", probeDur.Seconds())
		}
		// Verificar orçamento: cada binário pode fazer no máximo uma nova tentativa por processo (ML-2E).
		if culprit, spent := reg.retryBudget(psBin); spent {
			// Orçamento esgotado: registrar e reprovar sem nova tentativa.
			reg.register(psBin, testName)
			return r, probeDur, fmt.Sprintf(
				"timeout: orçamento de nova tentativa de %s esgotado (gasto em %s), sonda=%s — marcado sem nova tentativa",
				filepath.Base(psBin), culprit, probeDurMsg,
			)
		}
		// Máquina possivelmente parada: consumir orçamento e nova tentativa com prazo derivado da sonda.
		reg.consumeRetry(psBin, testName)
		retryDeadline := psCallDeadline(probeDur)
		r2 := run(retryDeadline)
		if !r2.timedOut {
			// Nova tentativa passou; retornar probeDur para log do chamador.
			// O binário NÃO é registrado: um episódio de parada não condena o binário.
			return r2, probeDur, ""
		}
		// Nova tentativa também estourou: registrar e fatal.
		reg.register(psBin, testName)
		return r2, probeDur, fmt.Sprintf(
			"timeout: %s esgotou 2×: chamada=%.0fs, sonda=%s, nova tentativa=%.0fs — máquina parada",
			filepath.Base(psBin), deadline.Seconds(), probeDurMsg, retryDeadline.Seconds(),
		)
	}

	// Sonda rápida (ou erro não-timeout): quem travou foi o guard.
	reg.register(psBin, testName)
	probeDesc := fmt.Sprintf("%.2fs", probeDur.Seconds())
	if probeErr != nil {
		probeDesc = fmt.Sprintf("erro(%v)", probeErr)
	}
	return r, probeDur, fmt.Sprintf(
		"timeout: %s após %.0fs, sonda exit 0 rápida (%s): o guard travou, não a máquina",
		filepath.Base(psBin), deadline.Seconds(), probeDesc,
	)
}

// guardShellMatrix mapeia nome legível → constante gerada.
// Deve conter exactamente as 4 formas PS/POSIX do registry (credential, git-branch,
// write-scope, credential-global). A guarda de vacuidade em TestGuardLineShellMatrix
// falha se este número divergir — forçando atualização quando o registry crescer.
var guardShellMatrix = []struct {
	name string
	line string
}{
	{"git-branch", guardGitBranchCmdPSPOSIX},
	{"credential", guardCredentialCmdPSPOSIX},
	{"write-scope", guardWriteScopeCmdPSPOSIX},
	{"credential-global", guardCredentialGlobalCmdPSPOSIX},
}

// shellEntries descreve os shells a exercitar. zshF usa "-f" para suprimir o .zshrc do usuário,
// isolando o comportamento puro do zsh sem customizações.
// ML-1B (REQ-2026-10-20): acrescentados dash, zsh -o errexit, zsh -o nounset, zsh -o cshnullglob
// para cobrir as variantes de opção que a forma `unsetopt` não protegia.
// ML-1A (REQ-2026-11-13): acrescentados bash/sh/dash com -e/-u e BASH_ENV/SHELLOPTS herdados
// para cobrir os casos onde errexit/nounset é herdado pelo processo pai (D18).
var shellEntries = []struct {
	name       string
	bin        string
	flags      []string
	extraEnvFn func(t *testing.T) []string // nil = sem env extra; chamado dentro do sub-test
}{
	{"sh", "sh", nil, nil},
	{"bash", "bash", nil, nil},
	{"zsh", "zsh", nil, nil},
	{"zsh-f", "zsh", []string{"-f"}, nil},
	{"dash", "dash", nil, nil},
	{"zsh-errexit", "zsh", []string{"-o", "errexit"}, nil},
	{"zsh-nounset", "zsh", []string{"-o", "nounset"}, nil},
	{"zsh-cshnullglob", "zsh", []string{"-o", "cshnullglob"}, nil},
	// D18: shells POSIX com opções herdadas que causavam saída 127 na forma anterior.
	{"bash-e", "bash", []string{"-e"}, nil},
	{"bash-u", "bash", []string{"-u"}, nil},
	{"sh-e", "sh", []string{"-e"}, nil},
	{"sh-u", "sh", []string{"-u"}, nil},
	{"dash-e", "dash", []string{"-e"}, nil},
	// SHELLOPTS=errexit herdado via env — bash torna-o readonly dentro do processo.
	{"bash-shellopts-errexit", "bash", nil, func(t *testing.T) []string {
		return []string{"SHELLOPTS=errexit"}
	}},
	// BASH_ENV sourced com set -e / set -u.
	{"bash-BASH_ENV-sete", "bash", nil, func(t *testing.T) []string {
		f := filepath.Join(t.TempDir(), "bash_env_e")
		if err := os.WriteFile(f, []byte("set -e\n"), 0600); err != nil {
			t.Fatalf("criando BASH_ENV-sete: %v", err)
		}
		return []string{"BASH_ENV=" + f}
	}},
	{"bash-BASH_ENV-setu", "bash", nil, func(t *testing.T) []string {
		f := filepath.Join(t.TempDir(), "bash_env_u")
		if err := os.WriteFile(f, []byte("set -u\n"), 0600); err != nil {
			t.Fatalf("criando BASH_ENV-setu: %v", err)
		}
		return []string{"BASH_ENV=" + f}
	}},
}

// makeFakeTrackfw cria um stub "trackfw" em dir que:
//   - copia stdin para stdin.out no mesmo diretório
//   - escreve "motivo-deny-fake" em stderr
//   - sai com $FAKE_RC (lido do env)
//
// O stub é um script #!/bin/sh simples — legível por qualquer shell POSIX via shebang.
func makeFakeTrackfw(t *testing.T, dir string) {
	t.Helper()
	stub := filepath.Join(dir, "trackfw")
	content := "#!/bin/sh\n" +
		"cat > \"$(dirname \"$0\")/stdin.out\"\n" +
		"echo -n 'motivo-deny-fake' >&2\n" +
		"exit ${FAKE_RC:-0}\n"
	if err := os.WriteFile(stub, []byte(content), 0755); err != nil {
		t.Fatalf("criando fake trackfw: %v", err)
	}
}

// runGuardLine executa a linha de guard via o shell indicado.
// Retorna (exitCode, stderr, stdinCaptured, erro de execução).
// stdinCaptured é o conteúdo de stdin.out gravado pelo stub, ou "" se o stub não rodou.
//
// extraEnv: variáveis adicionais a injetar (ex: SHELLOPTS=errexit, BASH_ENV=/path/to/file).
// BASH_ENV e SHELLOPTS são sempre removidos do env herdado antes de aplicar extraEnv, para que o
// test controle exatamente quais dessas variáveis estão presentes.
func runGuardLine(
	t *testing.T,
	shellBin string,
	shellFlags []string,
	extraEnv []string,
	line string,
	fakeDir string, // diretório com o stub; "" = trackfw ausente do PATH
	fakeRC int,
) (exitCode int, stderr string, stdinCaptured string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := append(shellFlags, "-c", line)
	cmd := exec.CommandContext(ctx, shellBin, args...)

	// Montar PATH.
	var pathVal string
	if fakeDir != "" {
		pathVal = fakeDir + ":/usr/bin:/bin"
	} else {
		pathVal = "/usr/bin:/bin"
	}

	// Propagar env mínimo, sobrescrevendo PATH e adicionando FAKE_RC.
	// BASH_ENV e SHELLOPTS são removidos do env herdado — o test os controla via extraEnv.
	base := os.Environ()
	filtered := make([]string, 0, len(base)+2+len(extraEnv))
	for _, e := range base {
		if strings.HasPrefix(e, "PATH=") ||
			strings.HasPrefix(e, "FAKE_RC=") ||
			strings.HasPrefix(e, "BASH_ENV=") ||
			strings.HasPrefix(e, "SHELLOPTS=") {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, "PATH="+pathVal)
	if fakeDir != "" {
		filtered = append(filtered, fmt.Sprintf("FAKE_RC=%d", fakeRC))
	}
	filtered = append(filtered, extraEnv...)
	cmd.Env = filtered

	cmd.Stdin = strings.NewReader(stdinPayload)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
	}

	var captured string
	if fakeDir != "" {
		data, readErr := os.ReadFile(filepath.Join(fakeDir, "stdin.out"))
		if readErr == nil {
			captured = string(data)
		}
	}

	return code, errBuf.String(), captured
}

// psEntries descreve os intérpretes PowerShell a exercitar.
// pwsh (7.x) existe no macOS (/opt/homebrew/bin/pwsh) e no runner windows-latest.
// powershell (5.1) existe apenas no Windows; o braço é pulado no macOS com motivo explícito.
var psEntries = []struct {
	name string
	bin  string
}{
	{"pwsh", "pwsh"},
	{"powershell", "powershell"},
}

// makeFakeTrackfwCmd cria um stub "trackfw.cmd" em dir para uso no Windows.
// Preferido sobre trackfw.ps1 porque: (1) processo nativo via cmd.exe — $LASTEXITCODE
// e stdin fluem corretamente; (2) não sofre restrição de execution policy.
// stdin é capturado com "more", que lê stdin até EOF; o comparador normaliza CRLF no Go.
func makeFakeTrackfwCmd(t *testing.T, dir string) {
	t.Helper()
	stub := filepath.Join(dir, "trackfw.cmd")
	content := "@echo off\r\n" +
		"if \"%FAKE_RC%\"==\"\" set FAKE_RC=0\r\n" +
		"more > \"%~dp0stdin.out\"\r\n" +
		"echo motivo-deny-fake>&2\r\n" +
		"exit /b %FAKE_RC%\r\n"
	if err := os.WriteFile(stub, []byte(content), 0644); err != nil {
		t.Fatalf("criando fake trackfw.cmd: %v", err)
	}
}

// makeFakeTrackfwForOS seleciona o stub adequado para o SO corrente.
// No Windows cria trackfw.cmd (resolvido via PATHEXT por pwsh/powershell).
// Em outros SOs cria trackfw com shebang #!/bin/sh (resolvido diretamente via PATH).
// Medido no macOS 2026-10-29: pwsh 7 encaminha stdin para subprocessos nativos — a
// sonda confirma que o stub POSIX funciona sob pwsh neste SO.
func makeFakeTrackfwForOS(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		makeFakeTrackfwCmd(t, dir)
	} else {
		makeFakeTrackfw(t, dir)
	}
}

// buildPSPath monta o valor de PATH para runGuardLinePS e para a sonda de LookPath no caso absent.
// fakeDir != "" → inclui o diretório do stub; fakeDir == "" → apenas dirs do sistema.
// Windows: separador ";" e %SystemRoot%\System32 (necessário para cmd.exe/powershell.exe).
// Outros SO: separador ":" e /usr/bin:/bin.
func buildPSPath(fakeDir string) string {
	if runtime.GOOS == "windows" {
		sysRoot := os.Getenv("SystemRoot")
		if sysRoot == "" {
			sysRoot = `C:\Windows`
		}
		sys32 := filepath.Join(sysRoot, "System32")
		if fakeDir != "" {
			return fakeDir + ";" + sys32
		}
		return sys32
	}
	if fakeDir != "" {
		return fakeDir + ":/usr/bin:/bin"
	}
	return "/usr/bin:/bin"
}

// psExecResult contém o resultado de execPSCommand.
// runErr é não-nil apenas para erros que não são nil nem *exec.ExitError
// (ex: exec.ErrWaitDelay ou falha de cmd.Start).
type psExecResult struct {
	exitCode int
	stderr   string
	timedOut bool // ctx.Err() == context.DeadlineExceeded — discriminante do #649 (AC2)
	started  bool // cmd.ProcessState != nil: o processo chegou a iniciar (P-3)
	runErr   error
}

// execPSCommand executa psBin com -NoProfile -NonInteractive -Command <command>.
// Não chama funções de testing; todos os resultados são devolvidos em psExecResult.
//
// fakeDir != "" → inclui o stub no PATH e define FAKE_RC=fakeRC no env.
// cmd.WaitDelay = psWaitDelay (P-5): evita bloqueio por grandchild que segura pipe após kill.
//
// Classificação do erro em três vias (P-5):
//   - err == nil          → exitCode = 0
//   - ErrWaitDelay        → runErr não-nil (grandchild travou pipe)
//   - *exec.ExitError     → exitCode = ExitCode() (inclui kill por deadline: -1/macOS, 1/Windows)
//   - outro               → runErr não-nil
//
// timedOut é definido como ctx.Err() == context.DeadlineExceeded, não pelo exit code (AC2).
func execPSCommand(psBin, command, fakeDir string, fakeRC int, timeout time.Duration) psExecResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"-NoProfile", "-NonInteractive", "-Command", command}
	cmd := exec.CommandContext(ctx, psBin, args...)
	cmd.WaitDelay = psWaitDelay

	// Montar PATH adequado ao SO via helper compartilhado com a sonda de absent.
	pathVal := buildPSPath(fakeDir)

	// Propagar env: herdar o do processo pai, substituindo PATH e FAKE_RC.
	// No Windows PATH pode aparecer como "Path" — filtrar pelo prefixo normalizado.
	base := os.Environ()
	filtered := make([]string, 0, len(base)+2)
	for _, e := range base {
		upper := strings.ToUpper(e)
		if strings.HasPrefix(upper, "PATH=") || strings.HasPrefix(upper, "FAKE_RC=") {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, "PATH="+pathVal)
	if fakeDir != "" {
		filtered = append(filtered, fmt.Sprintf("FAKE_RC=%d", fakeRC))
	}
	cmd.Env = filtered

	cmd.Stdin = strings.NewReader(stdinPayload)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	err := cmd.Run()

	var r psExecResult
	r.stderr = errBuf.String()
	r.started = cmd.ProcessState != nil

	switch {
	case err == nil:
		r.exitCode = 0
	case errors.Is(err, exec.ErrWaitDelay):
		r.runErr = err
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.exitCode = exitErr.ExitCode()
		} else {
			r.runErr = err
		}
	}
	r.timedOut = ctx.Err() == context.DeadlineExceeded
	return r
}

// psWarmupMu e psWarmupFns guardam, por binário, a função sync.OnceValues que executa
// o aquecimento exatamente uma vez por processo (AC1, P-2).
var (
	psWarmupMu  sync.Mutex
	psWarmupFns = make(map[string]func() (time.Duration, error))
)

// psWarmup retorna (dur, err, fresh) do aquecimento de psBin.
// fresh=true indica que o aquecimento foi executado agora (primeira chamada para este binário).
// O aquecimento executa: <psBin> -NoProfile -NonInteractive -Command "exit 0" com psWarmupTimeout.
// Em timeout, devolve err que envolve errPSTimeout.
func psWarmup(psBin string) (dur time.Duration, err error, fresh bool) {
	psWarmupMu.Lock()
	fn, exists := psWarmupFns[psBin]
	if !exists {
		fn = sync.OnceValues(func() (time.Duration, error) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), psWarmupTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, psBin, "-NoProfile", "-NonInteractive", "-Command", "exit 0")
			runErr := cmd.Run()
			d := time.Since(start)
			if ctx.Err() == context.DeadlineExceeded {
				return d, fmt.Errorf("timeout após %.1fs: %w", d.Seconds(), errPSTimeout)
			}
			if runErr != nil {
				return d, runErr
			}
			return d, nil
		})
		psWarmupFns[psBin] = fn
		fresh = true
	}
	psWarmupMu.Unlock()
	dur, err = fn()
	return
}

// psWarmupRetryFns guarda, por binário, a função sync.OnceValues que executa a nova tentativa
// do aquecimento via psProbe, caso psWarmup tenha estourado o prazo (ADR D3, segunda emenda).
var (
	psWarmupRetryMu  sync.Mutex
	psWarmupRetryFns = make(map[string]func() (time.Duration, error))
)

// psWarmupRetryLogic implementa a lógica de nova tentativa do aquecimento sem acoplamento
// a PowerShell, tornando-a testável com doubles (ML-2E, Regra Dura de Reconciliação).
//
// Semântica:
//   - firstErr == nil → não há necessidade de nova tentativa; retorna (firstDur, 0, nil).
//   - firstErr != nil, mas não é errPSTimeout → não mascarar com retry; retorna (firstDur, 0, firstErr).
//   - firstErr wraps errPSTimeout → chama retryFn uma vez:
//     - retryFn passa → (firstDur, retryDur, nil).
//     - retryFn falha → (firstDur, retryDur, erro com as duas durações).
func psWarmupRetryLogic(firstDur time.Duration, firstErr error, retryFn func() (time.Duration, error)) (time.Duration, time.Duration, error) {
	if firstErr == nil {
		return firstDur, 0, nil
	}
	if !errors.Is(firstErr, errPSTimeout) {
		// Erro não-timeout: não tentar mascarar com retry.
		return firstDur, 0, firstErr
	}
	rDur, rErr := retryFn()
	if rErr == nil {
		return firstDur, rDur, nil
	}
	return firstDur, rDur, fmt.Errorf("dois estouros: %.1fs, %.1fs: %w", firstDur.Seconds(), rDur.Seconds(), errPSTimeout)
}

// psWarmupOrRetry retorna a duração efetiva do aquecimento para psBin.
// Se psWarmup passou, usa aquela duração. Se estourou, tenta uma vez via psProbe(psBin, psWarmupTimeout).
// A nova tentativa é cacheada (sync.OnceValues): chamadas subsequentes reutilizam o resultado.
//
// Retorna (firstDur, retryDur, err):
//   - retryDur > 0 indica que a nova tentativa foi executada (e a duração efetiva é retryDur).
//   - err != nil indica falha em ambas as tentativas ou falha não-timeout na original.
func psWarmupOrRetry(psBin string) (firstDur, retryDur time.Duration, err error) {
	firstDur, firstErr, _ := psWarmup(psBin)

	psWarmupRetryMu.Lock()
	fn, exists := psWarmupRetryFns[psBin]
	if !exists {
		fn = sync.OnceValues(func() (time.Duration, error) {
			return psProbe(psBin, psWarmupTimeout)
		})
		psWarmupRetryFns[psBin] = fn
	}
	psWarmupRetryMu.Unlock()

	return psWarmupRetryLogic(firstDur, firstErr, fn)
}

// ensurePSWarmup aquece psBin uma vez por processo e registra o resultado com t.Logf.
// Se o aquecimento original estourou, tenta uma nova tentativa via psProbe (cacheada).
// Chama t.Fatalf com mensagem própria se ambas as tentativas falharem (AC1, D3 segunda emenda).
// Todo braço PS chama ensurePSWarmup logo após LookPath (P-2, P-8).
func ensurePSWarmup(t *testing.T, psBin string) {
	t.Helper()
	firstDur, retryDur, err := psWarmupOrRetry(psBin)
	if err != nil {
		if retryDur > 0 {
			// Ambas as tentativas estouraram.
			t.Fatalf("aquecimento de %s estourou duas vezes (%.1fs, %.1fs): máquina parada, nada foi verificado",
				filepath.Base(psBin), firstDur.Seconds(), retryDur.Seconds())
		}
		// Falha não-timeout na tentativa original.
		t.Fatalf("aquecimento de %s falhou: %v (%.2fs)", filepath.Base(psBin), err, firstDur.Seconds())
	}
	if retryDur > 0 {
		t.Logf("aquecimento de %s: 1ª tentativa estourou (%.2fs), nova tentativa passou (%.2fs)",
			filepath.Base(psBin), firstDur.Seconds(), retryDur.Seconds())
	} else {
		t.Logf("aquecimento de %s: %.2fs", filepath.Base(psBin), firstDur.Seconds())
	}
}

// runGuardLinePS executa a linha de guard via PowerShell com
// -NoProfile -NonInteractive -Command <linha>.
// Retorna (exitCode, stderr, stdinCaptured).
//
// Delega a execução para psGuardedCall, que aplica o mecanismo de sonda (D3, segunda emenda):
//   - Se timedOut na 1ª chamada: sonda com psProbe decide entre nova tentativa (sonda lenta)
//     e fatal imediato (sonda rápida — guard travou).
//   - Se a nova tentativa também estoura: registra o binário e t.Fatalf.
//   - Se o binário já estourou: t.Fatalf "não executado", sem chamar execPSCommand.
//   - Se runErr: t.Fatalf com o erro (ex: ErrWaitDelay de grandchild).
//
// O aquecimento efetivo é obtido de psWarmupOrRetry (pode ser a nova tentativa do aquecimento).
func runGuardLinePS(
	t *testing.T,
	psBin string,
	line string,
	fakeDir string, // diretório com o stub; "" = trackfw ausente do PATH
	fakeRC int,
) (exitCode int, stderr string, stdinCaptured string) {
	t.Helper()

	// Obter aquecimento efetivo do cache.
	firstDur, retryDur, warmupErr := psWarmupOrRetry(psBin)
	if warmupErr != nil {
		t.Fatalf("aquecimento de %s falhou; prazo não pode ser derivado: %v", psBin, warmupErr)
	}
	warmup := firstDur
	if retryDur > 0 {
		warmup = retryDur // usar duração da nova tentativa como referência do prazo
	}

	r, probeDur, fatal := psGuardedCall(psTimeouts, psBin, t.Name(), warmup,
		func() (time.Duration, error) {
			return psProbe(psBin, psCallDeadlineCeil)
		},
		func(d time.Duration) psExecResult {
			return execPSCommand(psBin, line, fakeDir, fakeRC, d)
		})

	// Registrar sonda antes dos checks de fatal (preserva a informação mesmo em erro).
	if probeDur > 0 {
		t.Logf("sonda de %s: %.2fs", filepath.Base(psBin), probeDur.Seconds())
	}

	if fatal != "" {
		t.Fatalf("%s", fatal)
	}
	if r.runErr != nil {
		t.Fatalf("erro de execução em %s: %v", psBin, r.runErr)
	}

	var captured string
	if fakeDir != "" {
		data, readErr := os.ReadFile(filepath.Join(fakeDir, "stdin.out"))
		if readErr == nil {
			// Normalizar CRLF: trackfw.cmd no Windows pode adicionar \r\n via "more";
			// o stub POSIX não adiciona. TrimRight remove trailing \n residual.
			captured = strings.ReplaceAll(string(data), "\r\n", "\n")
			captured = strings.TrimRight(captured, "\n")
		}
	}

	return r.exitCode, r.stderr, captured
}

// TestGuardLineShellMatrix é o teste principal exigido pelo AC da REQ-2026-10-20.
// Executa o produto cartesiano (4 linhas de guard) × (shells POSIX + PS) × (4 cenários).
//
// Braços POSIX: sh, bash, zsh, dash e variantes — pulam no Windows (shells ausentes).
// Braço pwsh: roda no macOS (/opt/homebrew/bin/pwsh) e no CI windows-latest.
// Braço powershell 5.1: exclusivo do Windows — pulado no macOS com motivo explícito.
func TestGuardLineShellMatrix(t *testing.T) {
	// Guarda de vacuidade: falha se guardShellMatrix tiver mais ou menos de 4 entradas.
	// Detecta adição ou remoção de entradas nesta lista — não verifica o CONTEÚDO de cada
	// entrada, portanto uma troca de constante não dispara esta guarda. A guarda de
	// reconciliação independente (Regra Dura) é responsabilidade da auditoria do arquiteto.
	const wantGuardForms = 4
	if len(guardShellMatrix) != wantGuardForms {
		t.Fatalf("guardShellMatrix tem %d formas; esperado %d "+
			"(credential, git-branch, write-scope, credential-global) — "+
			"entrada adicionada ou removida desta lista exige atualização do contador",
			len(guardShellMatrix), wantGuardForms)
	}

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {

			// ── Braços POSIX ────────────────────────────────────────────────────
			for _, sh := range shellEntries {
				sh := sh
				t.Run(sh.name, func(t *testing.T) {
					if runtime.GOOS == "windows" {
						t.Skip("shells POSIX não disponíveis em Windows — matriz Windows medida separadamente na VM")
					}

					// Verificar se o binário do shell existe.
					shellPath, err := exec.LookPath(sh.bin)
					if err != nil {
						t.Skipf("%s não encontrado em PATH — shell ausente neste ambiente", sh.bin)
					}

					// Construir env extra (BASH_ENV, SHELLOPTS, etc.) para este shell.
					var xenv []string
					if sh.extraEnvFn != nil {
						xenv = sh.extraEnvFn(t)
					}

					// Caso 1: FAKE_RC=0 (allow) → exit 0, stdin preservado.
					t.Run("RC0-allow", func(t *testing.T) {
						dir := t.TempDir()
						makeFakeTrackfw(t, dir)
						code, _, captured := runGuardLine(t, shellPath, sh.flags, xenv, guard.line, dir, 0)
						if code != 0 {
							t.Errorf("FAKE_RC=0: esperado exit 0, obteve %d", code)
						}
						if captured != stdinPayload {
							t.Errorf("stdin não preservado: got %q, want %q", captured, stdinPayload)
						}
					})

					// Caso 2: FAKE_RC=2 (deny canônico) → exit 2, stderr com motivo, stdin preservado.
					t.Run("RC2-deny", func(t *testing.T) {
						dir := t.TempDir()
						makeFakeTrackfw(t, dir)
						code, stderr, captured := runGuardLine(t, shellPath, sh.flags, xenv, guard.line, dir, 2)
						if code != 2 {
							t.Errorf("FAKE_RC=2: esperado exit 2, obteve %d", code)
						}
						if !strings.Contains(stderr, "motivo-deny-fake") {
							t.Errorf("FAKE_RC=2: stderr não contém 'motivo-deny-fake'; got %q", stderr)
						}
						if captured != stdinPayload {
							t.Errorf("stdin não preservado: got %q, want %q", captured, stdinPayload)
						}
					})

					// Caso 3: FAKE_RC=1 (deny não-canônico) → normalizado para exit 2.
					t.Run("RC1-deny-normalizado", func(t *testing.T) {
						dir := t.TempDir()
						makeFakeTrackfw(t, dir)
						code, _, _ := runGuardLine(t, shellPath, sh.flags, xenv, guard.line, dir, 1)
						if code != 2 {
							t.Errorf("FAKE_RC=1: esperado exit 2 (normalizado), obteve %d", code)
						}
					})

					// Caso 4: trackfw ausente do PATH → fail-closed, exit 2.
					t.Run("absent", func(t *testing.T) {
						code, _, _ := runGuardLine(t, shellPath, sh.flags, xenv, guard.line, "", 0)
						if code != 2 {
							t.Errorf("trackfw ausente: esperado exit 2, obteve %d", code)
						}
					})
				})
			}

			// ── Braços PS (pwsh 7.x e powershell 5.1) ──────────────────────────
			// A linha é executada como: <psBin> -NoProfile -NonInteractive -Command <linha>.
			// Stub no macOS: #!/bin/sh (resolvido via shebang, medido 2026-10-29).
			// Stub no Windows: trackfw.cmd (resolvido via PATHEXT, preferido sobre .ps1 —
			//   sem execution policy, $LASTEXITCODE e stdin fluem como processo nativo cmd.exe).
			// stderr PS inclui CommandNotFound para `emulate` e para LASTEXITCODE=$((2*!!$?)) —
			//   custo aceito, ADR-2026-10-04 adendo 2026-10-21. A asserção do deny verifica
			//   apenas a presença de "motivo-deny-fake" (substring), não ausência de ruído.
			for _, ps := range psEntries {
				ps := ps
				t.Run(ps.name, func(t *testing.T) {
					// powershell 5.1 só existe no Windows.
					if ps.bin == "powershell" && runtime.GOOS != "windows" {
						t.Skip("powershell 5.1 existe apenas no Windows; braço exercitado no CI windows-latest")
					}

					// Localizar o binário PS; pular com motivo se ausente.
					psBin, err := exec.LookPath(ps.bin)
					if err != nil {
						t.Skipf("%s não encontrado em PATH — PowerShell ausente neste ambiente", ps.bin)
					}

					// Aquecimento por binário, uma vez por processo (AC1, P-2).
					// ensurePSWarmup usa psWarmupOrRetry: se o aquecimento original estourou,
					// tenta uma nova tentativa via psProbe antes de reprovar (D3, segunda emenda).
					ensurePSWarmup(t, psBin)

					// Caso 1: FAKE_RC=0 (allow) → exit 0, stdin preservado.
					// Afirma: a linha PS/POSIX não altera o exit code quando trackfw sai 0.
					t.Run("RC0-allow", func(t *testing.T) {
						dir := t.TempDir()
						makeFakeTrackfwForOS(t, dir)
						code, _, captured := runGuardLinePS(t, psBin, guard.line, dir, 0)
						if code != 0 {
							t.Errorf("FAKE_RC=0: esperado exit 0, obteve %d", code)
						}
						want := strings.TrimRight(stdinPayload, "\r\n")
						if captured != want {
							t.Errorf("stdin não preservado: got %q, want %q", captured, want)
						}
					})

					// Caso 2: FAKE_RC=2 (deny canônico) → exit 2, motivo no stderr.
					// Afirma: a linha PS/POSIX propaga exit 2 e o motivo do guard sob PowerShell.
					t.Run("RC2-deny", func(t *testing.T) {
						dir := t.TempDir()
						makeFakeTrackfwForOS(t, dir)
						code, stderr, captured := runGuardLinePS(t, psBin, guard.line, dir, 2)
						if code != 2 {
							t.Errorf("FAKE_RC=2: esperado exit 2, obteve %d", code)
						}
						if !strings.Contains(stderr, "motivo-deny-fake") {
							t.Errorf("FAKE_RC=2: stderr não contém 'motivo-deny-fake'; got %q", stderr)
						}
						want := strings.TrimRight(stdinPayload, "\r\n")
						if captured != want {
							t.Errorf("stdin não preservado: got %q, want %q", captured, want)
						}
					})

					// Caso 3: FAKE_RC=1 (deny não-canônico) → normalizado para exit 2.
					// Afirma: a linha PS/POSIX normaliza exit 1 para exit 2 sob PowerShell.
					t.Run("RC1-deny-normalizado", func(t *testing.T) {
						dir := t.TempDir()
						makeFakeTrackfwForOS(t, dir)
						code, _, _ := runGuardLinePS(t, psBin, guard.line, dir, 1)
						if code != 2 {
							t.Errorf("FAKE_RC=1: esperado exit 2 (normalizado), obteve %d", code)
						}
					})

					// Caso 4: trackfw ausente do PATH → fail-closed, exit 2.
					// Afirma: a linha PS/POSIX falha fechada quando trackfw não está no PATH sob PowerShell.
					// Guarda de vacuidade: LookPath sobre o PATH montado confirma que trackfw não resolve
					// antes de afirmar que a ausência foi realmente exercitada (não falso-verde por stub
					// instalado no PATH do sistema). t.Setenv é seguro aqui: nenhum t.Parallel no arquivo.
					t.Run("absent", func(t *testing.T) {
						absentPath := buildPSPath("")
						t.Setenv("PATH", absentPath)
						if _, err := exec.LookPath("trackfw"); err == nil {
							t.Fatal("trackfw resolve no PATH sem stub — " +
								"o caso 'absent' é vacuidade; verificar se trackfw está em " + absentPath)
						}
						code, _, _ := runGuardLinePS(t, psBin, guard.line, "", 0)
						if code != 2 {
							t.Errorf("trackfw ausente: esperado exit 2, obteve %d", code)
						}
					})
				})
			}
		})
	}
}

// TestGuardLineShellMatrix_DefeitoAntigo demonstra que a forma ANTIGA (sem emulate -R sh e sem set +eu)
// falha sob zsh com FAKE_RC=2 — saindo com exit ≠ 2.
// Esse braço é a "prova de mordida": se o teste passar com a forma antiga, ele não distingue
// defeito de correção e não tem valor como regressão.
//
// Reconciliação (REQ-2026-10-20): afirma que a forma D11 revised sem emulate -R sh sai ≠ 2
// sob zsh, confirmando que o prefixo emulate é necessário.
// Atualizado (REQ-2026-11-13): a forma atual começa com setEuPrefix + emulatePrefix; ambos são
// removidos para obter o corpo D11Rev-era, que é o "defeito original" a demonstrar.
func TestGuardLineShellMatrix_DefeitoAntigo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shells POSIX não disponíveis em Windows")
	}

	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh não encontrado — braço do defeito não aplicável neste ambiente")
	}

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			// Confirmar que a forma atual começa com setEuPrefix (D18) seguido de emulatePrefix (D11Rev).
			// Se qualquer um estiver ausente, a correção foi removida — falha de compilação mascararia o defeito.
			if !strings.HasPrefix(guard.line, setEuPrefix) {
				t.Fatalf("constante %q não começa com setEuPrefix %q — fix D18 foi removido?",
					guard.name, setEuPrefix)
			}
			afterSetEu := strings.TrimPrefix(guard.line, setEuPrefix)
			if !strings.HasPrefix(afterSetEu, emulatePrefix) {
				t.Fatalf("constante %q após remoção de setEuPrefix não começa com emulatePrefix %q — fix D11Rev foi removido?",
					guard.name, emulatePrefix)
			}

			// Forma antiga D11Rev-era: o corpo sem nenhum prefixo (nem set+eu nem emulate).
			// Essa é a forma que falha sob zsh por causa de EQUALS e NOMATCH.
			oldLine := strings.TrimPrefix(afterSetEu, emulatePrefix)

			// Exercitar sob zsh com FAKE_RC=2.
			dir := t.TempDir()
			makeFakeTrackfw(t, dir)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			args := []string{"-c", oldLine}
			cmd := exec.CommandContext(ctx, zshPath, args...)

			pathVal := dir + ":/usr/bin:/bin"
			base := os.Environ()
			filtered := make([]string, 0, len(base)+2)
			for _, e := range base {
				if strings.HasPrefix(e, "PATH=") ||
					strings.HasPrefix(e, "FAKE_RC=") ||
					strings.HasPrefix(e, "BASH_ENV=") ||
					strings.HasPrefix(e, "SHELLOPTS=") {
					continue
				}
				filtered = append(filtered, e)
			}
			filtered = append(filtered, "PATH="+pathVal, "FAKE_RC=2")
			cmd.Env = filtered
			cmd.Stdin = strings.NewReader(stdinPayload)

			runErr := cmd.Run()
			code := 0
			if runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				}
			}

			// A forma antiga DEVE falhar (sair com exit ≠ 2 sob zsh).
			// Se sair com exit 2, o braço não detecta o defeito original — é falso-verde.
			if code == 2 {
				t.Errorf(
					"braço do defeito (%s): forma D11Rev-era saiu com exit 2 sob zsh — "+
						"o prefixo emulate pode não ser necessário para este guard ou o zsh está configurado de forma não-padrão; "+
						"verificar ambiente zsh",
					guard.name,
				)
			} else {
				t.Logf("braço do defeito (%s): forma D11Rev-era saiu com exit %d (esperado ≠ 2) — defeito D11Rev confirmado", guard.name, code)
			}
		})
	}
}

// TestGuardLineShellMatrix_UnsetoptInsuficiente demonstra que a forma alternativa com
// "unsetopt EQUALS NOMATCH" como prefixo é insuficiente: sob zsh -o nounset com FAKE_RC=2,
// a linha ainda sai com exit ≠ 2 (fail-open). Essa é a razão pela qual o ML-1B trocou
// o prefixo `unsetopt` pelo `emulate -R sh`.
//
// Reconciliação (REQ-2026-10-20 ML-1B): afirma que a forma unsetopt falha sob zsh -o nounset
// — confirmando que apenas `emulate -R sh` é suficiente para todas as opções zsh.
func TestGuardLineShellMatrix_UnsetoptInsuficiente(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shells POSIX não disponíveis em Windows")
	}

	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh não encontrado — braço de regressão não aplicável neste ambiente")
	}

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			// Verificar que a constante começa com setEuPrefix + emulatePrefix — a regressão
			// precisa de ambos para confirmar que apenas emulate não era suficiente.
			if !strings.HasPrefix(guard.line, setEuPrefix) {
				t.Fatalf("constante %q não começa com setEuPrefix — fix D18 foi removido?",
					guard.name)
			}
			afterSetEu := strings.TrimPrefix(guard.line, setEuPrefix)
			if !strings.HasPrefix(afterSetEu, emulatePrefix) {
				t.Fatalf("constante %q após remoção de setEuPrefix não começa com emulatePrefix — fix D11Rev foi removido?",
					guard.name)
			}

			// Construir a forma unsetopt: unsetoptOldPrefix + corpo sem os dois prefixos.
			body := strings.TrimPrefix(afterSetEu, emulatePrefix)
			unsetoptLine := unsetoptOldPrefix + body

			// Exercitar sob zsh -o nounset com FAKE_RC=2.
			dir := t.TempDir()
			makeFakeTrackfw(t, dir)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			args := []string{"-o", "nounset", "-c", unsetoptLine}
			cmd := exec.CommandContext(ctx, zshPath, args...)

			pathVal := dir + ":/usr/bin:/bin"
			base := os.Environ()
			filtered := make([]string, 0, len(base)+2)
			for _, e := range base {
				if strings.HasPrefix(e, "PATH=") ||
					strings.HasPrefix(e, "FAKE_RC=") ||
					strings.HasPrefix(e, "BASH_ENV=") ||
					strings.HasPrefix(e, "SHELLOPTS=") {
					continue
				}
				filtered = append(filtered, e)
			}
			filtered = append(filtered, "PATH="+pathVal, "FAKE_RC=2")
			cmd.Env = filtered
			cmd.Stdin = strings.NewReader(stdinPayload)

			runErr := cmd.Run()
			code := 0
			if runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				}
			}

			// A forma unsetopt DEVE falhar sob zsh -o nounset (sair com exit ≠ 2).
			// Se sair com exit 2 significa que a forma funcionou — contraditório com a
			// afirmação de que unsetopt é insuficiente — então o teste deve reprovar.
			if code == 2 {
				t.Errorf("forma unsetopt saiu com exit 2 sob zsh -o nounset (%s) — "+
					"esperado exit ≠ 2 (insuficiência); o prefixo emulate -R sh pode ter "+
					"sido removido ou o zsh local não aplica nounset a expansões não-interativas",
					guard.name)
			} else {
				t.Logf("forma unsetopt saiu com exit %d (≠ 2) sob zsh -o nounset (%s) — "+
					"insuficiência confirmada; emulate -R sh é necessário", code, guard.name)
			}
		})
	}
}

// TestGuardLineShellMatrix_D18ContraArm demonstra que a forma D18 legacy (emulate -R sh sem o
// prefixo set +eu) falha sob shells POSIX com errexit ou nounset herdado — saindo com exit ≠ 2.
// Essa é a "prova de mordida" do D18: se a forma legada funcionasse nesses ambientes, o fix D18
// seria desnecessário e este braço seria falso-verde.
//
// Reconciliação (REQ-2026-11-13 ML-1A): afirma que cada uma das quatro formas legadas D18
// (guardforms.LegacyD18*PSPOSIX) sai ≠ 2 sob bash -e, bash -u, sh -e, sh -u e dash -e,
// confirmando que o prefixo `set +eu` é necessário para esses ambientes.
func TestGuardLineShellMatrix_D18ContraArm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shells POSIX não disponíveis em Windows")
	}

	// Mapeamento guard-name → D18 legacy constant (guardforms).
	type d18Entry struct {
		name    string
		legLine string
	}
	d18Matrix := []d18Entry{
		{"git-branch", guardforms.LegacyD18GitBranchPSPOSIX},
		{"credential", guardforms.LegacyD18CredentialPSPOSIX},
		{"write-scope", guardforms.LegacyD18WriteScopePSPOSIX},
		{"credential-global", guardforms.LegacyD18CredentialGlobalPSPOSIX},
	}

	// Shells POSIX com opções herdadas que causavam saída 127 na forma legada.
	// dash -e: pulado se dash não estiver disponível.
	type d18Shell struct {
		name  string
		bin   string
		flags []string
	}
	d18Shells := []d18Shell{
		{"bash-e", "bash", []string{"-e"}},
		{"bash-u", "bash", []string{"-u"}},
		{"sh-e", "sh", []string{"-e"}},
		{"sh-u", "sh", []string{"-u"}},
		{"dash-e", "dash", []string{"-e"}},
	}

	for _, guard := range d18Matrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			for _, sh := range d18Shells {
				sh := sh
				t.Run(sh.name, func(t *testing.T) {
					shellPath, err := exec.LookPath(sh.bin)
					if err != nil {
						t.Skipf("%s não encontrado em PATH — braço D18 não aplicável neste ambiente", sh.bin)
					}

					dir := t.TempDir()
					makeFakeTrackfw(t, dir)

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					args := append(sh.flags, "-c", guard.legLine)
					cmd := exec.CommandContext(ctx, shellPath, args...)

					pathVal := dir + ":/usr/bin:/bin"
					base := os.Environ()
					filtered := make([]string, 0, len(base)+2)
					for _, e := range base {
						if strings.HasPrefix(e, "PATH=") ||
							strings.HasPrefix(e, "FAKE_RC=") ||
							strings.HasPrefix(e, "BASH_ENV=") ||
							strings.HasPrefix(e, "SHELLOPTS=") {
							continue
						}
						filtered = append(filtered, e)
					}
					filtered = append(filtered, "PATH="+pathVal, "FAKE_RC=2")
					cmd.Env = filtered
					cmd.Stdin = strings.NewReader(stdinPayload)

					runErr := cmd.Run()
					code := 0
					if runErr != nil {
						if exitErr, ok := runErr.(*exec.ExitError); ok {
							code = exitErr.ExitCode()
						}
					}

					// A forma D18 legacy DEVE falhar (sair ≠ 2) sob o shell com opção herdada.
					// Se sair com exit 2, o braço não detecta o defeito — é falso-verde.
					if code == 2 {
						t.Errorf(
							"D18 contra-braço (%s/%s): forma legada saiu com exit 2 — "+
								"esperado exit ≠ 2; verificar se a forma legada foi alterada em guardforms",
							guard.name, sh.name,
						)
					} else {
						t.Logf("D18 contra-braço (%s/%s): forma legada saiu com exit %d (≠ 2) — defeito D18 confirmado",
							guard.name, sh.name, code)
					}

					// Confirmar que o guard nunca rodou (stdin.out não deve existir).
					stdinOutPath := filepath.Join(dir, "stdin.out")
					if _, statErr := os.Stat(stdinOutPath); statErr == nil {
						t.Errorf(
							"D18 contra-braço (%s/%s): stdin.out existe — guard rodou quando não deveria; "+
								"a forma legada pode estar passando por outro mecanismo",
							guard.name, sh.name,
						)
					}
				})
			}
		})
	}
}

// TestRunGuardLinePS_TimeoutDiscriminado verifica que execPSCommand discrimina
// corretamente timeout (kill por deadline) de saída natural, usando ctx.Err() (AC3, P-3, P-4).
//
// Usa apenas pwsh (P-9): evita cold start de powershell.exe no CI de Windows, que recriaria o #649.
//
// Reconciliação — AC3, dois braços:
//
//   - Braço "curto": afirma que ctx.Err()==DeadlineExceeded (não o exit code) é o discriminante
//     de timeout — porque kill sai com exit ≠ 0, mas timedOut deve ser true pelo contexto.
//
//   - Braço "normal": afirma que timedOut=false quando pwsh sai naturalmente com exit 1
//     (P-4: exit 1 falsifica mutação timedOut=(exitCode!=0), que produziria true incorretamente).
func TestRunGuardLinePS_TimeoutDiscriminado(t *testing.T) {
	// P-9: usa só pwsh para não causar cold start do powershell.exe no CI de Windows.
	psBin, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh não encontrado — teste do discriminante não aplicável neste ambiente")
	}

	// Braço curto: Start-Sleep 60 com prazo de 2 s (P-3).
	// Afirma: timedOut=true quando o processo é morto pelo deadline antes de terminar.
	// started=true confirma que o processo chegou a iniciar (ctx não expirou antes de cmd.Start).
	t.Run("curto", func(t *testing.T) {
		r := execPSCommand(psBin, "Start-Sleep 60", "", 0, 2*time.Second)
		if !r.timedOut {
			t.Errorf("braço curto: esperado timedOut=true (kill por deadline de 2s), obteve false; exitCode=%d runErr=%v", r.exitCode, r.runErr)
		}
		if !r.started {
			t.Errorf("braço curto: processo não chegou a iniciar (ProcessState==nil) — contexto pré-cancelado antes de cmd.Start?")
		}
	})

	// Braço normal: exit 1 com prazo de 15 s (P-4: exit 1 em vez de exit 0).
	// Afirma: timedOut=false quando a saída é natural — distingue kill de saída legítima.
	// P-4: exit 1 falsifica mutação timedOut=(exitCode!=0), que daria true incorretamente.
	t.Run("normal", func(t *testing.T) {
		r := execPSCommand(psBin, "exit 1", "", 0, 15*time.Second)
		if r.timedOut {
			t.Errorf("braço normal: esperado timedOut=false (saída natural exit 1), obteve true")
		}
		if r.exitCode != 1 {
			t.Errorf("braço normal: esperado exitCode=1, obteve %d", r.exitCode)
		}
	})
}

// TestPSCallDeadline verifica que psCallDeadline implementa clamp(4×warmup, 15s, 60s)
// nos três regimes e nas fronteiras exactas dos regimes (REQ-2026-11-06, AC3, D3).
//
// Reconciliação — AC3: afirma que a função de prazo deriva corretamente o deadline a
// partir do aquecimento, garantindo que o piso e o teto são respeitados.
func TestPSCallDeadline(t *testing.T) {
	cases := []struct {
		name   string
		warmup time.Duration
		want   time.Duration
	}{
		{"zero->piso", 0, 15 * time.Second},
		{"1s->piso", 1 * time.Second, 15 * time.Second},
		{"3.75s->piso(fronteira)", 3750 * time.Millisecond, 15 * time.Second},
		{"8s->32s(meio)", 8 * time.Second, 32 * time.Second},
		{"15s->teto(fronteira)", 15 * time.Second, 60 * time.Second},
		{"20s->teto", 20 * time.Second, 60 * time.Second},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := psCallDeadline(tc.warmup)
			if got != tc.want {
				t.Errorf("psCallDeadline(%v) = %v; want %v", tc.warmup, got, tc.want)
			}
		})
	}
}

// TestRunGuardLinePS_FailFastPorBinario verifica que psGuardedCall implementa o mecanismo
// fail-fast por binário: após o primeiro estouro definitivo (sonda rápida → guard travou),
// chamadas subsequentes para o mesmo binário não invocam run; para binário diferente, run é
// invocado normalmente (REQ-2026-11-06, AC3, emenda D3).
//
// Usa um registro isolado (não o psTimeouts global) e funções run/probe fake, sem PowerShell.
//
// Reconciliação — AC3 (emenda D3): afirma que o fail-fast impede execuções desnecessárias após
// o primeiro estouro definitivo (sonda rápida), limitando o pior caso por binário dentro do
// -timeout 20m do CI. A sonda rápida falsifica a mutação "sempre nova tentativa" (contra-braço).
func TestRunGuardLinePS_FailFastPorBinario(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBinA := "/nonexistent/fake-ps-a-" + t.Name()
	fakeBinB := "/nonexistent/fake-ps-b-" + t.Name()

	var runCallDeadlines []time.Duration

	// probe rápida: simula sonda exit 0 rápida (< psStallThreshold) → guard travou, registra imediato.
	fastProbe := func() (time.Duration, error) { return psStallThreshold / 2, nil }
	// probe que nunca deve ser chamada: reporta erro se for.
	neverProbe := func() (time.Duration, error) {
		t.Errorf("probe não deveria ser chamada neste ponto")
		return 0, nil
	}

	// Chamada 1: bin A com aquecimento de 5 s → prazo derivado 20 s, estoura.
	// sonda rápida (3.75 s < 7.5 s) → registra imediatamente, run chamado 1×.
	runCallDeadlines = nil
	_, _, msg1 := psGuardedCall(reg, fakeBinA, "TestX/case1", 5*time.Second,
		fastProbe,
		func(d time.Duration) psExecResult {
			runCallDeadlines = append(runCallDeadlines, d)
			if d != 20*time.Second {
				t.Errorf("chamada 1: prazo esperado 20s, obteve %v", d)
			}
			return psExecResult{timedOut: true}
		})
	if len(runCallDeadlines) != 1 {
		t.Errorf("chamada 1: run deve ser chamada uma vez; chamada %d vezes", len(runCallDeadlines))
	}
	if !strings.Contains(msg1, "timeout") {
		t.Errorf("chamada 1: msg deve conter 'timeout'; got %q", msg1)
	}
	if !strings.Contains(msg1, "20s") {
		t.Errorf("chamada 1: msg deve citar prazo '20s'; got %q", msg1)
	}
	if !strings.Contains(msg1, "guard travou") {
		t.Errorf("chamada 1: msg deve conter 'guard travou' (sonda rápida); got %q", msg1)
	}

	// Chamada 2: mesmo bin A — fail-fast, run NÃO deve ser chamada.
	// Afirma: após estouro com sonda rápida, execução é bloqueada e a mensagem cita o culpado original.
	runCallDeadlines = nil
	_, _, msg2 := psGuardedCall(reg, fakeBinA, "TestX/case2", 5*time.Second,
		neverProbe,
		func(d time.Duration) psExecResult {
			runCallDeadlines = append(runCallDeadlines, d)
			return psExecResult{}
		})
	if len(runCallDeadlines) != 0 {
		t.Errorf("chamada 2 (fail-fast): run não deve ser chamada; chamada %d vezes", len(runCallDeadlines))
	}
	if !strings.Contains(msg2, "não executado") {
		t.Errorf("chamada 2: msg deve conter 'não executado'; got %q", msg2)
	}
	if !strings.Contains(msg2, "TestX/case1") {
		t.Errorf("chamada 2: msg deve citar culpado 'TestX/case1'; got %q", msg2)
	}

	// Chamada 3: bin B diferente — fail-fast NÃO se aplica; run deve ser chamada.
	// Afirma: o fail-fast é por binário, não global.
	runCallDeadlines = nil
	_, _, msg3 := psGuardedCall(reg, fakeBinB, "TestX/case3", 4*time.Second,
		neverProbe,
		func(d time.Duration) psExecResult {
			runCallDeadlines = append(runCallDeadlines, d)
			if d != 16*time.Second {
				t.Errorf("chamada 3: prazo esperado 16s, obteve %v", d)
			}
			return psExecResult{exitCode: 0}
		})
	if len(runCallDeadlines) != 1 {
		t.Errorf("chamada 3 (binário diferente): run deve ser chamada uma vez; chamada %d vezes", len(runCallDeadlines))
	}
	if msg3 != "" {
		t.Errorf("chamada 3: mensagem fatal inesperada: %q", msg3)
	}
}

// TestPSGuardedCall_SondaLentaRetryPassaSemFatal verifica que, após um estouro com sonda lenta
// (duração >= psStallThreshold), uma nova tentativa é feita com prazo derivado da sonda, e se
// ela passa não há fatal e o binário NÃO é registrado.
//
// Doubles: warmup=5s → deadline1=20s; probe=10s (lento, >= 7.5s) → deadline2=40s; run1 estoura,
// run2 passa. Pradros distintos do piso (15s) e teto (60s) verificam que psCallDeadline é chamado
// com a duração real da sonda.
//
// Reconciliação — D3, segunda emenda: afirma que sonda lenta indica máquina parada, levando a
// nova tentativa; se ela passa, o episódio não condena o binário.
func TestPSGuardedCall_SondaLentaRetryPassaSemFatal(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBin := "/nonexistent/fake-ps-lenta-pass-" + t.Name()

	var runCallDeadlines []time.Duration
	probeCallCount := 0

	probe := func() (time.Duration, error) {
		probeCallCount++
		return 10 * time.Second, nil // lento: >= 7.5 s, sem erro
	}
	run := func(d time.Duration) psExecResult {
		runCallDeadlines = append(runCallDeadlines, d)
		if len(runCallDeadlines) == 1 {
			return psExecResult{timedOut: true} // 1ª chamada: estoura
		}
		return psExecResult{exitCode: 0} // 2ª chamada (nova tentativa): passa
	}

	r, probeDur, fatalMsg := psGuardedCall(reg, fakeBin, t.Name(), 5*time.Second, probe, run)

	if fatalMsg != "" {
		t.Errorf("esperado sem fatal; got %q", fatalMsg)
	}
	if r.exitCode != 0 {
		t.Errorf("esperado exitCode=0 após nova tentativa; got %d", r.exitCode)
	}
	if len(runCallDeadlines) != 2 {
		t.Errorf("run deve ser chamada 2×; chamada %d vezes", len(runCallDeadlines))
	}
	if runCallDeadlines[0] != 20*time.Second {
		t.Errorf("1ª deadline: esperado 20s (psCallDeadline(5s)); got %v", runCallDeadlines[0])
	}
	if runCallDeadlines[1] != 40*time.Second {
		t.Errorf("2ª deadline: esperado 40s (psCallDeadline(10s)); got %v", runCallDeadlines[1])
	}
	if probeCallCount != 1 {
		t.Errorf("probe deve ser chamada 1×; chamada %d vezes", probeCallCount)
	}
	if probeDur != 10*time.Second {
		t.Errorf("probeDur retornado: esperado 10s; got %v", probeDur)
	}
	if _, registered := reg.query(fakeBin); registered {
		t.Errorf("binário não deve ser registrado após nova tentativa bem-sucedida")
	}
}

// TestPSGuardedCall_SondaRapidaFatalSemRetry verifica que, após um estouro com sonda rápida
// (duração < psStallThreshold, sem timeout), o guard é considerado travado e há fatal imediato
// sem nova tentativa. Este é o caso discriminante da D3 segunda emenda.
//
// Doubles: warmup=5s → deadline=20s; probe=1s (rápida, < 7.5s); run estoura uma vez.
//
// Reconciliação — D3, segunda emenda (caso discriminante): afirma que sonda rápida elimina a
// hipótese de máquina parada — quem travou foi o guard, não a máquina. Fatal sem retry.
func TestPSGuardedCall_SondaRapidaFatalSemRetry(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBin := "/nonexistent/fake-ps-rapida-" + t.Name()

	var runCallDeadlines []time.Duration
	probeCallCount := 0

	probe := func() (time.Duration, error) {
		probeCallCount++
		return 1 * time.Second, nil // rápida: < 7.5 s
	}
	run := func(d time.Duration) psExecResult {
		runCallDeadlines = append(runCallDeadlines, d)
		return psExecResult{timedOut: true}
	}

	_, probeDur, fatalMsg := psGuardedCall(reg, fakeBin, t.Name(), 5*time.Second, probe, run)

	if fatalMsg == "" {
		t.Error("esperado fatal; got vazio")
	}
	if !strings.Contains(fatalMsg, "guard travou") {
		t.Errorf("fatal deve conter 'guard travou'; got %q", fatalMsg)
	}
	if strings.Contains(fatalMsg, "máquina parada") {
		t.Errorf("fatal não deve conter 'máquina parada' (é sonda rápida); got %q", fatalMsg)
	}
	if len(runCallDeadlines) != 1 {
		t.Errorf("run deve ser chamada 1× (sem retry); chamada %d vezes", len(runCallDeadlines))
	}
	if probeCallCount != 1 {
		t.Errorf("probe deve ser chamada 1×; chamada %d vezes", probeCallCount)
	}
	if probeDur != 1*time.Second {
		t.Errorf("probeDur retornado: esperado 1s; got %v", probeDur)
	}
	if _, registered := reg.query(fakeBin); !registered {
		t.Error("binário deve ser registrado após sonda rápida (guard travou)")
	}
}

// TestPSGuardedCall_SondaLentaRetryTambemEstouraFatal verifica que, após um estouro com sonda
// lenta e nova tentativa que também estoura, o binário é registrado e há fatal citando ambos
// os estouros.
//
// Doubles: warmup=5s → deadline1=20s; probe=10s (lento) → deadline2=40s; ambas as runs estouram.
//
// Reconciliação — D3, segunda emenda: afirma que sonda lenta + nova tentativa que estoura
// confirma máquina parada; binário registrado para fail-fast das chamadas seguintes.
func TestPSGuardedCall_SondaLentaRetryTambemEstouraFatal(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBin := "/nonexistent/fake-ps-lenta-fail-" + t.Name()

	var runCallDeadlines []time.Duration
	probeCallCount := 0

	probe := func() (time.Duration, error) {
		probeCallCount++
		return 10 * time.Second, nil // lento: >= 7.5 s
	}
	run := func(d time.Duration) psExecResult {
		runCallDeadlines = append(runCallDeadlines, d)
		return psExecResult{timedOut: true} // ambas as chamadas estouram
	}

	_, probeDur, fatalMsg := psGuardedCall(reg, fakeBin, t.Name(), 5*time.Second, probe, run)

	if fatalMsg == "" {
		t.Error("esperado fatal; got vazio")
	}
	// Mensagem deve citar a nova tentativa e a sonda, não "guard travou".
	if strings.Contains(fatalMsg, "guard travou") {
		t.Errorf("fatal não deve conter 'guard travou' (é sonda lenta); got %q", fatalMsg)
	}
	if !strings.Contains(fatalMsg, "máquina parada") {
		t.Errorf("fatal deve conter 'máquina parada'; got %q", fatalMsg)
	}
	if len(runCallDeadlines) != 2 {
		t.Errorf("run deve ser chamada 2×; chamada %d vezes", len(runCallDeadlines))
	}
	if runCallDeadlines[0] != 20*time.Second {
		t.Errorf("1ª deadline: esperado 20s; got %v", runCallDeadlines[0])
	}
	if runCallDeadlines[1] != 40*time.Second {
		t.Errorf("2ª deadline: esperado 40s (psCallDeadline(10s)); got %v", runCallDeadlines[1])
	}
	if probeCallCount != 1 {
		t.Errorf("probe deve ser chamada 1×; chamada %d vezes", probeCallCount)
	}
	if probeDur != 10*time.Second {
		t.Errorf("probeDur retornado: esperado 10s; got %v", probeDur)
	}
	if _, registered := reg.query(fakeBin); !registered {
		t.Error("binário deve ser registrado após nova tentativa que também estoura")
	}
}

// TestPSGuardedCall_SondaEstouraComoLenta verifica que uma sonda que estoura (errors.Is errPSTimeout)
// é tratada como lenta, levando a nova tentativa — mesmo que a duração retornada seja menor que
// psStallThreshold (o que não ocorre em produção, mas valida o caminho de código).
//
// Doubles: warmup=5s → deadline1=20s; probe=(psCallDeadlineCeil, errPSTimeout) → lenta por timeout;
// deadline2=psCallDeadline(60s)=60s (teto); run1 estoura, run2 passa.
//
// Reconciliação — D3, segunda emenda: afirma que sonda com timeout é equivalente a sonda lenta
// (errors.Is(probeErr, errPSTimeout) → isSlow=true); nova tentativa acontece e, passando, sem fatal.
func TestPSGuardedCall_SondaEstouraComoLenta(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBin := "/nonexistent/fake-ps-sonda-estoura-" + t.Name()

	var runCallDeadlines []time.Duration
	probeCallCount := 0

	// Sonda que "estoura": retorna errPSTimeout com duração igual a psCallDeadlineCeil.
	// Em produção, psProbe retorna (d, fmt.Errorf("...: %w", errPSTimeout)) ao estourar.
	probe := func() (time.Duration, error) {
		probeCallCount++
		return psCallDeadlineCeil, fmt.Errorf("timeout após 60.0s: %w", errPSTimeout)
	}
	run := func(d time.Duration) psExecResult {
		runCallDeadlines = append(runCallDeadlines, d)
		if len(runCallDeadlines) == 1 {
			return psExecResult{timedOut: true} // 1ª: estoura
		}
		return psExecResult{exitCode: 0} // 2ª (nova tentativa): passa
	}

	r, probeDur, fatalMsg := psGuardedCall(reg, fakeBin, t.Name(), 5*time.Second, probe, run)

	if fatalMsg != "" {
		t.Errorf("esperado sem fatal (nova tentativa passou); got %q", fatalMsg)
	}
	if r.exitCode != 0 {
		t.Errorf("esperado exitCode=0 após nova tentativa; got %d", r.exitCode)
	}
	if len(runCallDeadlines) != 2 {
		t.Errorf("run deve ser chamada 2× (estouro + nova tentativa); chamada %d vezes", len(runCallDeadlines))
	}
	if runCallDeadlines[0] != 20*time.Second {
		t.Errorf("1ª deadline: esperado 20s; got %v", runCallDeadlines[0])
	}
	// psCallDeadline(60s) = 60s (teto).
	if runCallDeadlines[1] != psCallDeadlineCeil {
		t.Errorf("2ª deadline: esperado %v (teto, derivado da sonda que estourou); got %v",
			psCallDeadlineCeil, runCallDeadlines[1])
	}
	if probeCallCount != 1 {
		t.Errorf("probe deve ser chamada 1×; chamada %d vezes", probeCallCount)
	}
	if probeDur != psCallDeadlineCeil {
		t.Errorf("probeDur retornado: esperado %v; got %v", psCallDeadlineCeil, probeDur)
	}
	if _, registered := reg.query(fakeBin); registered {
		t.Error("binário não deve ser registrado: nova tentativa passou")
	}
}

// TestPSWarmupRetryLogic verifica psWarmupRetryLogic com doubles, sem PowerShell.
// Cobre os três caminhos: (a) primeira tentativa passa; (b) timeout + retry passa;
// (c) timeout + retry falha; (d) erro não-timeout → retry nunca chamado.
//
// Reconciliação — D3, segunda emenda: afirma que a lógica de nova tentativa do aquecimento
// só aciona retry em timeout (errPSTimeout), não em qualquer erro — evitando mascarar falhas
// reais do binário com uma mensagem de "máquina parada".
func TestPSWarmupRetryLogic(t *testing.T) {
	neverRetry := func() (time.Duration, error) {
		t.Errorf("retryFn não deveria ser chamada neste caso")
		return 0, nil
	}

	// (a) primeira tentativa passa: retryFn nunca chamada.
	t.Run("primeira-passa", func(t *testing.T) {
		d1, d2, err := psWarmupRetryLogic(2*time.Second, nil, neverRetry)
		if err != nil {
			t.Errorf("esperado nil; got %v", err)
		}
		if d1 != 2*time.Second {
			t.Errorf("firstDur: esperado 2s; got %v", d1)
		}
		if d2 != 0 {
			t.Errorf("retryDur: esperado 0; got %v", d2)
		}
	})

	// (b) timeout + retry passa.
	t.Run("timeout-retry-passa", func(t *testing.T) {
		firstErr := fmt.Errorf("timeout após 60.0s: %w", errPSTimeout)
		d1, d2, err := psWarmupRetryLogic(60*time.Second, firstErr, func() (time.Duration, error) {
			return 5 * time.Second, nil
		})
		if err != nil {
			t.Errorf("esperado nil; got %v", err)
		}
		if d1 != 60*time.Second {
			t.Errorf("firstDur: esperado 60s; got %v", d1)
		}
		if d2 != 5*time.Second {
			t.Errorf("retryDur: esperado 5s; got %v", d2)
		}
	})

	// (c) timeout + retry também falha.
	t.Run("timeout-retry-falha", func(t *testing.T) {
		firstErr := fmt.Errorf("timeout após 60.0s: %w", errPSTimeout)
		retryErr := fmt.Errorf("timeout após 55.0s: %w", errPSTimeout)
		d1, d2, err := psWarmupRetryLogic(60*time.Second, firstErr, func() (time.Duration, error) {
			return 55 * time.Second, retryErr
		})
		if err == nil {
			t.Error("esperado erro; got nil")
		}
		if !strings.Contains(err.Error(), "dois estouros") {
			t.Errorf("erro deve conter 'dois estouros'; got %v", err)
		}
		if d1 != 60*time.Second || d2 != 55*time.Second {
			t.Errorf("durations: esperado (60s, 55s); got (%v, %v)", d1, d2)
		}
	})

	// (d) erro não-timeout: retryFn nunca chamada, erro original preservado.
	t.Run("erro-nao-timeout", func(t *testing.T) {
		otherErr := errors.New("binary crashed")
		d1, d2, err := psWarmupRetryLogic(1*time.Second, otherErr, neverRetry)
		if !errors.Is(err, otherErr) {
			t.Errorf("esperado otherErr; got %v", err)
		}
		if d2 != 0 {
			t.Errorf("retryDur: esperado 0 (retry não chamado); got %v", d2)
		}
		if d1 != 1*time.Second {
			t.Errorf("firstDur: esperado 1s; got %v", d1)
		}
	})
}

// TestPSGuardedCall_OrcamentoPorBinario verifica que o orçamento de nova tentativa é por binário:
// a primeira chamada que estoura com sonda lenta consome o orçamento e tenta de novo (passando);
// a segunda chamada ao mesmo binário que estoura encontra o orçamento esgotado, reprova sem nova
// tentativa (run 1×) e registra o binário.
//
// Doubles: warmup=5s → deadline=20s; probe=10s (lento, >= 7.5s) → retryDeadline=40s.
//   - Chamada 1: run estoura + probe lenta → consome orçamento → nova tentativa (run 2ª) passa → sem fatal.
//   - Chamada 2: run estoura + probe lenta → orçamento esgotado → fatal citando chamada 1, run 1×, bin registrado.
//
// Reconciliação — D3, segunda emenda (orçamento): afirma que uma nova tentativa bem-sucedida
// consome o orçamento do binário; estouro subsequente do mesmo binário reprova sem nova tentativa,
// com run chamado 1× e o binário registrado para fail-fast das chamadas seguintes.
func TestPSGuardedCall_OrcamentoPorBinario(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBinA := "/nonexistent/fake-ps-orcamento-" + t.Name()
	testName1 := t.Name() + "/chamada1"
	testName2 := t.Name() + "/chamada2"

	// ── Chamada 1: estouro + sonda lenta → consome orçamento → nova tentativa passa → sem fatal ──
	var runCallsA1 []time.Duration
	probeCallCount1 := 0
	_, probeDur1, fatal1 := psGuardedCall(reg, fakeBinA, testName1, 5*time.Second,
		func() (time.Duration, error) {
			probeCallCount1++
			return 10 * time.Second, nil // lento: >= 7.5 s
		},
		func(d time.Duration) psExecResult {
			runCallsA1 = append(runCallsA1, d)
			if len(runCallsA1) == 1 {
				return psExecResult{timedOut: true} // 1ª: estoura
			}
			return psExecResult{exitCode: 0} // 2ª (nova tentativa): passa
		})
	if fatal1 != "" {
		t.Errorf("chamada 1: esperado sem fatal; got %q", fatal1)
	}
	if len(runCallsA1) != 2 {
		t.Errorf("chamada 1: run deve ser chamada 2× (estouro + nova tentativa); chamada %d vezes", len(runCallsA1))
	}
	if probeCallCount1 != 1 {
		t.Errorf("chamada 1: probe deve ser chamada 1×; chamada %d vezes", probeCallCount1)
	}
	if probeDur1 != 10*time.Second {
		t.Errorf("chamada 1: probeDur esperado 10s; got %v", probeDur1)
	}
	if _, registered := reg.query(fakeBinA); registered {
		t.Error("chamada 1: bin não deve ser registrado após nova tentativa bem-sucedida")
	}
	if culprit, spent := reg.retryBudget(fakeBinA); !spent {
		t.Error("chamada 1: orçamento deve estar gasto após nova tentativa")
	} else if culprit != testName1 {
		t.Errorf("chamada 1: culprit esperado %q; got %q", testName1, culprit)
	}

	// ── Chamada 2: estouro + sonda lenta + orçamento esgotado → fatal, run 1×, bin registrado ──
	var runCallsA2 []time.Duration
	probeCallCount2 := 0
	_, probeDur2, fatal2 := psGuardedCall(reg, fakeBinA, testName2, 5*time.Second,
		func() (time.Duration, error) {
			probeCallCount2++
			return 10 * time.Second, nil // lento: >= 7.5 s
		},
		func(d time.Duration) psExecResult {
			runCallsA2 = append(runCallsA2, d)
			return psExecResult{timedOut: true}
		})
	if fatal2 == "" {
		t.Error("chamada 2: esperado fatal (orçamento esgotado); got vazio")
	}
	if !strings.Contains(fatal2, "orçamento") {
		t.Errorf("chamada 2: fatal deve citar 'orçamento'; got %q", fatal2)
	}
	if !strings.Contains(fatal2, testName1) {
		t.Errorf("chamada 2: fatal deve citar a chamada que gastou o orçamento %q; got %q", testName1, fatal2)
	}
	if len(runCallsA2) != 1 {
		t.Errorf("chamada 2: run deve ser chamada 1× (sem nova tentativa); chamada %d vezes", len(runCallsA2))
	}
	if probeCallCount2 != 1 {
		t.Errorf("chamada 2: probe deve ser chamada 1×; chamada %d vezes", probeCallCount2)
	}
	if probeDur2 != 10*time.Second {
		t.Errorf("chamada 2: probeDur esperado 10s; got %v", probeDur2)
	}
	if _, registered := reg.query(fakeBinA); !registered {
		t.Error("chamada 2: bin deve ser registrado após orçamento esgotado")
	}
}

// TestPSGuardedCall_OrcamentoBinarioDiferente verifica que o orçamento de nova tentativa é
// isolado por binário: um binário cujo orçamento foi gasto não impede a nova tentativa de um
// binário diferente no mesmo registro.
//
// Setup: bin A tem orçamento pré-gasto (simula estado após TestPSGuardedCall_OrcamentoPorBinario).
// Bin B: estouro + sonda lenta → consome seu próprio orçamento → nova tentativa passa → sem fatal.
//
// Reconciliação — D3, segunda emenda (isolamento): afirma que o orçamento é por binário; o
// orçamento gasto de bin A não bloqueia a nova tentativa de bin B.
func TestPSGuardedCall_OrcamentoBinarioDiferente(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBinA := "/nonexistent/fake-ps-orcamento-a-sep-" + t.Name()
	fakeBinB := "/nonexistent/fake-ps-orcamento-b-sep-" + t.Name()

	// Pré-gastar o orçamento de bin A (simula nova tentativa bem-sucedida anterior).
	reg.consumeRetry(fakeBinA, t.Name()+"/setup-a")

	// Bin B: tem orçamento próprio → estouro + sonda lenta → nova tentativa passa → sem fatal.
	var runCallsB []time.Duration
	probeCallCount := 0
	_, probeDurB, fatalB := psGuardedCall(reg, fakeBinB, t.Name()+"/chamadaB", 4*time.Second,
		func() (time.Duration, error) {
			probeCallCount++
			return 8 * time.Second, nil // lento: >= 7.5 s
		},
		func(d time.Duration) psExecResult {
			runCallsB = append(runCallsB, d)
			if len(runCallsB) == 1 {
				return psExecResult{timedOut: true} // 1ª: estoura
			}
			return psExecResult{exitCode: 0} // 2ª (nova tentativa): passa
		})
	if fatalB != "" {
		t.Errorf("bin B: esperado sem fatal; got %q", fatalB)
	}
	if len(runCallsB) != 2 {
		t.Errorf("bin B: run deve ser chamada 2× (estouro + nova tentativa); chamada %d vezes", len(runCallsB))
	}
	if probeCallCount != 1 {
		t.Errorf("bin B: probe deve ser chamada 1×; chamada %d vezes", probeCallCount)
	}
	if probeDurB != 8*time.Second {
		t.Errorf("bin B: probeDur esperado 8s; got %v", probeDurB)
	}
	if _, registered := reg.query(fakeBinB); registered {
		t.Error("bin B: não deve ser registrado após nova tentativa bem-sucedida")
	}
	if culprit, spent := reg.retryBudget(fakeBinB); !spent {
		t.Error("bin B: orçamento deve estar gasto após nova tentativa")
	} else if culprit != t.Name()+"/chamadaB" {
		t.Errorf("bin B: culprit esperado %q; got %q", t.Name()+"/chamadaB", culprit)
	}
}
