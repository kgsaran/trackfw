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

// setEuPrefix é o prefixo de um segmento da forma ML-1A (REQ-2026-11-13 ML-1A, substituída por
// ML-1C e depois ML-1C2). Mantida aqui APENAS para derivar a linha ML-1A no contra-braço de
// EAP=Stop (TestGuardLineShellMatrix_D18NovaContraArm_Stop):
//
//	ml1aLine = setEuPrefix + guardforms.LegacyD18*PSPOSIX
//
// NÃO use setEuPrefix para verificar o prefixo da forma atual — a forma atual começa com
// ml1dPrefix (seis segmentos — forma L, ML-2I).
const setEuPrefix = "set +eu 2>${null-/dev/null}; "

// ml1dConfirmSeg é o segmento de ConfirmPreference adicionado pela forma L (ML-2I).
// Colocado entre o segmento EAP e o set +eu para que Set-Variable "+eu" (ConfirmImpact=Medium)
// não abra um prompt de confirmação quando $ConfirmPreference='Low'/'Medium' está herdado
// e o PowerShell não está rodando com -NonInteractive. Sem ele, o prompt drenaria o stdin
// e o guard receberia 0 bytes do payload JSON (fail-open).
const ml1dConfirmSeg = "set ConfirmPreference None -Force -WhatIf:0 -Confirm:0; "

// ml1dPrefix é o preamble de seis segmentos da forma L, adicionada pela REQ-2026-11-13
// ML-2I. Coloca o reset de EAP com -Force/-WhatIf:0/-Confirm:0 ANTES do ConfirmPreference,
// que por sua vez vem ANTES do set +eu, para que nem WhatIf nem ConfirmPreference possam
// suprimir o próprio set +eu. Define null=/dev/null antes de qualquer 2>$null,
// garantindo zero linhas de preamble no stderr POSIX. Em PowerShell, $null é variável
// automática (sempre definida).
//
// Uso: verificar que a forma atual começa com este prefixo; âncora de sanidade nos
// contra-braços (os oldLine/legacyLine vêm de guardforms, não derivados de guard.line).
const ml1dPrefix = "set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0; " + ml1dConfirmSeg + "set +eu; null=/dev/null; Set-StrictMode -Off 2>$null; "

// ml1dEmulateSegment é o segmento emulate da forma H (ML-1C2). Usa 2>$null para suprimir o
// command-not-found de emulate em POSIX (onde $null já foi definido como /dev/null).
const ml1dEmulateSegment = "emulate -R sh 2>$null; "

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

// psConcurrencySem limita os processos pwsh simultâneos durante os testes.
// Sem este limitador, dezenas de testes paralelos lançam pwsh ao mesmo tempo; a
// latência de startup sobe acima de psCallDeadlineCeil, o fail-fast de psGuardedCall
// dispara e todos os testes subsequentes saem com -1.
// Tamanho 4: paralelo real (4 pwsh simultâneos), bem abaixo do limite de memória/CPU
// de um M-series com 8 vCPUs.
var psConcurrencySem = make(chan struct{}, 4)

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
	// Isolar cache JIT (ML-1D): psProbe pode correr com invocações paralelas de pwsh.
	probeEnv, probeCleanup := newPwshCacheEnv(os.Environ())
	defer probeCleanup()
	cmd.Env = probeEnv
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
		"printf '%s' 'motivo-deny-fake' >&2\n" +
		"exit ${FAKE_RC:-0}\n"
	if err := writeExecutableForTest(stub, []byte(content), 0755); err != nil {
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

	// Montar PATH (portável: separador e dirs do sistema variam por SO).
	pathVal := buildShellPath(shellBin, fakeDir)

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
		} else {
			t.Fatalf("runGuardLine: cmd.Run() falhou com erro que não é *exec.ExitError: %v", runErr)
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
	if err := writeExecutableForTest(stub, []byte(content), 0644); err != nil {
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

// buildShellPath monta o PATH para shells POSIX (sh, bash, zsh, dash…).
// shellBin é o caminho completo do binário do shell (usado no Windows para incluir seu diretório).
// fakeDir != "" → inclui o diretório do stub trackfw; fakeDir == "" → apenas dirs do sistema.
// Windows: separador ";" e diretório do shell binary (Git Bash instala em Program Files/Git/usr/bin).
// Outros SO: separador ":" e /usr/bin:/bin.
func buildShellPath(shellBin, fakeDir string) string {
	if runtime.GOOS == "windows" {
		shellDir := filepath.Dir(shellBin)
		if fakeDir != "" {
			return fakeDir + ";" + shellDir
		}
		return shellDir
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
	// Isolar cache JIT por invocação (ML-1D): cada pwsh recebe XDG_CACHE_HOME próprio para
	// evitar corrida na escrita de StartupProfileData-* (FileLoadException com nomes embaralhados).
	filtered, cacheCleanup := newPwshCacheEnv(filtered)
	defer cacheCleanup()
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
			// Isolar cache JIT (ML-1D): aquecimento também corre com invocações paralelas;
			// cache isolado garante que warmup e chamadas reais usem o mesmo regime (cold start).
			warmupEnv, warmupCleanup := newPwshCacheEnv(os.Environ())
			defer warmupCleanup()
			cmd.Env = warmupEnv
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
			psConcurrencySem <- struct{}{}
			defer func() { <-psConcurrencySem }()
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
					// GOOS skip removido: sh/bash disponíveis no windows-latest via Git for Windows.
					// zsh/dash ausentes pulam via LookPath abaixo. Classificação (b) [FALSO].

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
					// t.Parallel() é seguro: cada caso tem seu próprio t.TempDir() e não usa
					// t.Setenv (extraEnvFn retorna []string, não chama os.Setenv direto).
					t.Run("RC0-allow", func(t *testing.T) {
						t.Parallel()
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
						t.Parallel()
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
						t.Parallel()
						dir := t.TempDir()
						makeFakeTrackfw(t, dir)
						code, _, _ := runGuardLine(t, shellPath, sh.flags, xenv, guard.line, dir, 1)
						if code != 2 {
							t.Errorf("FAKE_RC=1: esperado exit 2 (normalizado), obteve %d", code)
						}
					})

					// Caso 4: trackfw ausente do PATH → fail-closed, exit 2.
					// t.Parallel() é seguro: usa dir="" sem modificar PATH via t.Setenv.
					t.Run("absent", func(t *testing.T) {
						t.Parallel()
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
						t.Parallel()
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
						t.Parallel()
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
						t.Parallel()
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
					// instalado no PATH do sistema). t.Setenv é seguro aqui: ps.name não é paralelo e
					// os irmãos paralelos (RC0/RC2/RC1) são liberados só depois que o corpo do ps.name
					// retorna — após o absent ter completado e restaurado PATH via Cleanup.
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
// Atualizado (REQ-2026-11-13 ML-1C2): oldLine vem de guardforms.LegacyD11Rev* (constante estável)
// em vez de ser derivada de guard.line, pois a forma H usa 2>$null que requer null=/dev/null.
// TestGuardLineShellMatrix_DefeitoAntigo: GOOS skip removido; zsh LookPath abaixo skip por probe.
// zsh é improvável no windows-latest mas o probe é o mecanismo correto. Classificação (b).
func TestGuardLineShellMatrix_DefeitoAntigo(t *testing.T) {
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh não encontrado — braço do defeito não aplicável neste ambiente")
	}

	// Mapeamento guard-name → D11Rev legacy constant (guardforms).
	// write-scope não tem D11RevWriteScope próprio (guard adicionado após D11Rev); usa
	// strings.TrimPrefix(LegacyD18WriteScopePSPOSIX, emulatePrefix) como equivalente.
	type d11revEntry struct {
		name    string
		oldLine string
	}
	d11revMatrix := []d11revEntry{
		{"git-branch", guardforms.LegacyD11RevGitBranchPSPOSIX},
		{"credential", guardforms.LegacyD11RevCredentialPSPOSIX},
		{"write-scope", strings.TrimPrefix(guardforms.LegacyD18WriteScopePSPOSIX, emulatePrefix)},
		{"credential-global", guardforms.LegacyD11RevCredentialGlobalPSPOSIX},
	}
	// Indexar por nome para fácil acesso no loop de guardShellMatrix.
	d11revByName := make(map[string]string, len(d11revMatrix))
	for _, e := range d11revMatrix {
		d11revByName[e.name] = e.oldLine
	}

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			// Sanidade: confirmar que a forma atual começa com ml1dPrefix seguido de ml1dEmulateSegment.
			// Se qualquer um estiver ausente, a correção ML-2I (forma L) foi removida.
			if !strings.HasPrefix(guard.line, ml1dPrefix) {
				t.Fatalf("constante %q não começa com ml1dPrefix %q — fix D18 ML-2I (forma L) foi removido?",
					guard.name, ml1dPrefix)
			}
			afterML1D := strings.TrimPrefix(guard.line, ml1dPrefix)
			if !strings.HasPrefix(afterML1D, ml1dEmulateSegment) {
				t.Fatalf("constante %q após remoção de ml1dPrefix não começa com ml1dEmulateSegment %q — fix D11Rev foi removido?",
					guard.name, ml1dEmulateSegment)
			}

			// Forma antiga D11Rev-era: vem de guardforms (constante estável, sem $null).
			// Essa é a forma que falha sob zsh por causa de EQUALS e NOMATCH.
			oldLine, ok := d11revByName[guard.name]
			if !ok {
				t.Fatalf("d11revByName não tem entrada para %q — atualizar mapeamento", guard.name)
			}

			// Exercitar sob zsh com FAKE_RC=2.
			dir := t.TempDir()
			makeFakeTrackfw(t, dir)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			args := []string{"-c", oldLine}
			cmd := exec.CommandContext(ctx, zshPath, args...)

			pathVal := buildShellPath(zshPath, dir)
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
				} else {
					t.Fatalf("TestGuardLineShellMatrix_DefeitoAntigo: cmd.Run() falhou com erro que não é *exec.ExitError: %v", runErr)
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
// TestGuardLineShellMatrix_UnsetoptInsuficiente: GOOS skip removido; zsh LookPath abaixo skip por probe.
// Classificação (b) — zsh improvável no windows-latest mas probe é o mecanismo correto.
func TestGuardLineShellMatrix_UnsetoptInsuficiente(t *testing.T) {
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skipf("zsh não encontrado em PATH (%v) — braço de regressão não aplicável; skip (b): probe de capacidade", err)
	}

	// Mapeamento guard-name → D11Rev legacy body (mesma lógica de DefeitoAntigo).
	// write-scope: TrimPrefix(LegacyD18WriteScopePSPOSIX, emulatePrefix).
	type unsetoptEntry struct {
		name string
		body string
	}
	unsetoptBodies := []unsetoptEntry{
		{"git-branch", guardforms.LegacyD11RevGitBranchPSPOSIX},
		{"credential", guardforms.LegacyD11RevCredentialPSPOSIX},
		{"write-scope", strings.TrimPrefix(guardforms.LegacyD18WriteScopePSPOSIX, emulatePrefix)},
		{"credential-global", guardforms.LegacyD11RevCredentialGlobalPSPOSIX},
	}
	unsetoptBodyByName := make(map[string]string, len(unsetoptBodies))
	for _, e := range unsetoptBodies {
		unsetoptBodyByName[e.name] = e.body
	}

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			// Sanidade: verificar que a constante começa com ml1dPrefix + ml1dEmulateSegment.
			// A regressão precisa de ambos para confirmar que apenas emulate não era suficiente.
			if !strings.HasPrefix(guard.line, ml1dPrefix) {
				t.Fatalf("constante %q não começa com ml1dPrefix — fix D18 ML-2I (forma L) foi removido?",
					guard.name)
			}
			afterML1D := strings.TrimPrefix(guard.line, ml1dPrefix)
			if !strings.HasPrefix(afterML1D, ml1dEmulateSegment) {
				t.Fatalf("constante %q após remoção de ml1dPrefix não começa com ml1dEmulateSegment — fix D11Rev foi removido?",
					guard.name)
			}

			// Construir a forma unsetopt: unsetoptOldPrefix + body D11Rev (de guardforms, com ${null-/dev/null}).
			body, ok := unsetoptBodyByName[guard.name]
			if !ok {
				t.Fatalf("unsetoptBodyByName não tem entrada para %q — atualizar mapeamento", guard.name)
			}
			unsetoptLine := unsetoptOldPrefix + body

			// Exercitar sob zsh -o nounset com FAKE_RC=2.
			dir := t.TempDir()
			makeFakeTrackfw(t, dir)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			args := []string{"-o", "nounset", "-c", unsetoptLine}
			cmd := exec.CommandContext(ctx, zshPath, args...)

			pathVal := buildShellPath(zshPath, dir)
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
				} else {
					t.Fatalf("TestGuardLineShellMatrix_UnsetoptInsuficiente: cmd.Run() falhou com erro que não é *exec.ExitError: %v", runErr)
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

// d18ShellKind classifica o shell em "dash" ou "bash" seguindo os symlinks do shellPath.
// Usado pelo contra-braço D18 para determinar o código de saída esperado sob -u:
//   - "dash": sai com 2 (código de erro nounset do dash) antes de executar o guard.
//   - "bash": sai com 127 (comando não encontrado) — qualquer shell que não seja dash.
//
// No Linux, /bin/sh geralmente é um symlink para /usr/bin/dash (ou /bin/dash) → "dash".
// No macOS, /bin/sh é um binário Mach-O real que implementa bash → "bash" (EvalSymlinks
// não avança além do próprio binário; o base name é "sh", não "dash").
// Se EvalSymlinks falhar, usa o base name do shellPath como aproximação.
func d18ShellKind(shellPath string) string {
	resolved, err := filepath.EvalSymlinks(shellPath)
	if err != nil {
		resolved = shellPath
	}
	base := strings.ToLower(filepath.Base(resolved))
	if base == "dash" || strings.HasPrefix(base, "dash") {
		return "dash"
	}
	return "bash"
}

// TestGuardLineShellMatrix_D18ContraArm demonstra que a forma D18 legacy (emulate -R sh sem o
// prefixo set +eu) falha com o guard nunca rodando sob shells POSIX com errexit ou nounset
// herdados. Essa é a "prova de mordida" do D18: se a forma legada funcionasse nesses ambientes,
// o fix D18 seria desnecessário e este braço seria falso-verde.
//
// Comportamento por shell (medido):
//   - bash-e, bash-u, sh-e, sh-u-em-macOS (sh=binário próprio), dash-e: exit ≠ 2 (tipicamente 127)
//     — shell aborta por "emulate" ou comando não encontrado; guard não roda.
//   - dash-u, sh-u-em-linux (sh→dash via symlink): exit == 2 (fail-closed)
//     — dash aborta com código 2 ao expandir $LASTEXITCODE indefinido sob nounset.
//     stdin.out ausente é a única prova de que o guard não rodou, pois exit 2 coincide
//     com o código de deny do stub (FAKE_RC=2).
//
// Reconciliação (REQ-2026-11-13 ML-2E): afirma que cada forma legada D18 sai com o código
// esperado por shell (exit ≠ 2 ou exit == 2 dependendo de dash+nounset) E guard não rodou
// (stdin.out ausente). A classificação por d18ShellKind(shellPath) determina qual expectativa
// se aplica em tempo de execução, sem depender de GOOS.
// TestGuardLineShellMatrix_D18ContraArm: GOOS skip removido; each shell has LookPath probe inside.
// Classificação (b) — sh/bash disponíveis no windows-latest; zsh/dash pulam via probe.
func TestGuardLineShellMatrix_D18ContraArm(t *testing.T) {
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

	// Shells POSIX com opções herdadas.
	// dash-u é o caso fail-closed (exit 2 por nounset); demais saem com 127.
	// dash-e e dash-u: pulados se dash não estiver disponível.
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
		{"dash-u", "dash", []string{"-u"}},
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

					// Classificar o shell pelo caminho real (resolve symlinks).
					// d18ShellKind e exec.CommandContext usam o mesmo shellPath — sem divergência.
					kind := d18ShellKind(shellPath)
					hasDashU := false
					for _, f := range sh.flags {
						if f == "-u" {
							hasDashU = true
							break
						}
					}
					// dash com -u aborta com exit 2 antes do guard (fail-closed por nounset).
					// Demais combinações saem com 127 (emulate não encontrado).
					wantExit2 := kind == "dash" && hasDashU

					dir := t.TempDir()
					makeFakeTrackfw(t, dir)

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					args := append(sh.flags, "-c", guard.legLine)
					cmd := exec.CommandContext(ctx, shellPath, args...)

					pathVal := buildShellPath(shellPath, dir)
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
						} else {
							t.Fatalf("D18ContraArm(%s/%s): cmd.Run() falhou com erro que não é *exec.ExitError: %v", guard.name, sh.name, runErr)
						}
					}

					if wantExit2 {
						// dash -u: fail-closed — sai com 2 (código nounset do dash) antes do guard.
						if code != 2 {
							t.Errorf(
								"D18 contra-braço (%s/%s, shell=%s, kind=%s): exit %d — "+
									"esperado 2 (dash -u aborta com 2 em variável indefinida antes do guard)",
								guard.name, sh.name, shellPath, kind, code,
							)
						} else {
							t.Logf("D18 contra-braço (%s/%s, shell=%s): exit 2 — fail-closed dash-u confirmado",
								guard.name, sh.name, shellPath)
						}
					} else {
						// Demais: fail-open por 127 — guard nunca roda, mas sai ≠ 2.
						if code == 2 {
							t.Errorf(
								"D18 contra-braço (%s/%s, shell=%s, kind=%s): forma legada saiu com exit 2 — "+
									"esperado exit ≠ 2; verificar se a forma legada foi alterada em guardforms",
								guard.name, sh.name, shellPath, kind,
							)
						} else {
							t.Logf("D18 contra-braço (%s/%s, shell=%s): exit %d (≠ 2) — defeito D18 confirmado",
								guard.name, sh.name, shellPath, code)
						}
					}

					// Confirmar que o guard nunca rodou (stdin.out não deve existir).
					// Para dash-u: distingue "shell abortou antes do guard" (exit 2, sem stdin.out)
					// de "guard rodou e negou" (exit 2, com stdin.out) — FAKE_RC=2 coincide com o
					// código nounset do dash, então stdin.out é a única prova invariante.
					stdinOutPath := filepath.Join(dir, "stdin.out")
					if _, statErr := os.Stat(stdinOutPath); statErr == nil {
						t.Errorf(
							"D18 contra-braço (%s/%s, shell=%s): stdin.out existe — guard rodou quando não deveria; "+
								"a forma legada pode estar passando por outro mecanismo",
							guard.name, sh.name, shellPath,
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

// execPSCommandOpts é a versão configurável de execPSCommand.
// noProfile=true insere "-NoProfile" nos argumentos (comportamento padrão).
// noProfile=false omite "-NoProfile", permitindo que o perfil do usuário seja carregado.
// extraEnv: variáveis adicionais de ambiente a injetar (ex: XDG_CONFIG_HOME=…).
func execPSCommandOpts(psBin, command, fakeDir string, fakeRC int, timeout time.Duration, noProfile bool, extraEnv []string) psExecResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var args []string
	if noProfile {
		args = []string{"-NoProfile", "-NonInteractive", "-Command", command}
	} else {
		args = []string{"-NonInteractive", "-Command", command}
	}
	cmd := exec.CommandContext(ctx, psBin, args...)
	cmd.WaitDelay = psWaitDelay

	pathVal := buildPSPath(fakeDir)

	// Se extraEnv já fornece PATH=..., não adicionamos o pathVal computado — evita
	// dois PATH= no env. Em exec+Unix o comportamento com chaves duplicadas é indefinido;
	// no macOS libc prevalece o primeiro, então dois PATH= levaria o filho ao PATH
	// errado em vez do fornecido por extraEnv.
	extraHasPath := false
	for _, e := range extraEnv {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			extraHasPath = true
			break
		}
	}

	base := os.Environ()
	filtered := make([]string, 0, len(base)+2+len(extraEnv))
	for _, e := range base {
		upper := strings.ToUpper(e)
		if strings.HasPrefix(upper, "PATH=") || strings.HasPrefix(upper, "FAKE_RC=") {
			continue
		}
		filtered = append(filtered, e)
	}
	if !extraHasPath {
		filtered = append(filtered, "PATH="+pathVal)
	}
	if fakeDir != "" {
		filtered = append(filtered, fmt.Sprintf("FAKE_RC=%d", fakeRC))
	}
	filtered = append(filtered, extraEnv...)
	// Isolar cache JIT por invocação (ML-1D): cada pwsh recebe XDG_CACHE_HOME próprio para
	// evitar corrida na escrita de StartupProfileData-* (FileLoadException com nomes embaralhados).
	// Aplica-se após extraEnv para também filtrar XDG_CACHE_HOME de extraEnv, se presente.
	filtered, cacheCleanup := newPwshCacheEnv(filtered)
	defer cacheCleanup()
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

// runGuardLinePSNoProfile executa a linha de guard via PowerShell SEM -NoProfile,
// injetando as variáveis de ambiente extras (ex: XDG_CONFIG_HOME para controlar o perfil).
// A assinatura e semântica de timeout seguem runGuardLinePS.
func runGuardLinePSNoProfile(
	t *testing.T,
	psBin string,
	line string,
	fakeDir string,
	fakeRC int,
	extraEnv []string,
) (exitCode int, stderr string, stdinCaptured string) {
	t.Helper()

	firstDur, retryDur, warmupErr := psWarmupOrRetry(psBin)
	if warmupErr != nil {
		t.Fatalf("aquecimento de %s falhou: %v", psBin, warmupErr)
	}
	warmup := firstDur
	if retryDur > 0 {
		warmup = retryDur
	}

	r, probeDur, fatal := psGuardedCall(psTimeouts, psBin, t.Name(), warmup,
		func() (time.Duration, error) {
			return psProbe(psBin, psCallDeadlineCeil)
		},
		func(d time.Duration) psExecResult {
			psConcurrencySem <- struct{}{}
			defer func() { <-psConcurrencySem }()
			return execPSCommandOpts(psBin, line, fakeDir, fakeRC, d, false, extraEnv)
		})

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
			captured = strings.ReplaceAll(string(data), "\r\n", "\n")
			captured = strings.TrimRight(captured, "\n")
		}
	}

	return r.exitCode, r.stderr, captured
}

// TestGuardLineShellMatrix_PSConstantsStructural afirma que as 4 constantes PS/POSIX
// partilham preamble e sufixo idênticos (derivados de git-branch como referência),
// diferindo apenas no segmento de subcomando. Não requer pwsh — é um teste puramente
// estático sobre o valor das constantes.
//
// Reconciliação (REQ-2026-11-13 ML-2D): afirma que todos os subcomandos partilham o
// mesmo preamble e sufixo que git-branch, tornando desnecessário repetir a matriz
// completa de estados PS para cada subcomando.
func TestGuardLineShellMatrix_PSConstantsStructural(t *testing.T) {
	// Mapeamento canônico guard.name → subcomando esperado na linha.
	expectedSubcmds := map[string]string{
		"git-branch":        "git-branch",
		"credential":        "credential",
		"write-scope":       "write-scope",
		"credential-global": "credential --global",
	}

	const sepFw = "trackfw guard "

	// Derivar preamble e sufixo de referência a partir da constante git-branch.
	refLine := ""
	for _, g := range guardShellMatrix {
		if g.name == "git-branch" {
			refLine = g.line
			break
		}
	}
	if refLine == "" {
		t.Fatal("git-branch not found in guardShellMatrix")
	}

	// Preamble: tudo antes de "trackfw guard git-branch".
	refSubcmd := expectedSubcmds["git-branch"]
	refFull := sepFw + refSubcmd
	idx := strings.Index(refLine, refFull)
	if idx < 0 {
		t.Fatalf("git-branch line does not contain %q:\n%q", refFull, refLine)
	}
	refPre := refLine[:idx]
	refSuf := refLine[idx+len(refFull):]

	// Verificar cada constante contra o template derivado de git-branch.
	for _, g := range guardShellMatrix {
		g := g
		t.Run(g.name, func(t *testing.T) {
			sub, ok := expectedSubcmds[g.name]
			if !ok {
				t.Fatalf("expectedSubcmds does not have entry for %q — update the map", g.name)
			}
			wantLine := refPre + sepFw + sub + refSuf
			if g.line != wantLine {
				t.Errorf("constant %q line mismatch:\n got: %q\nwant: %q", g.name, g.line, wantLine)
			}
		})
	}
}

// TestGuardLineShellMatrix_PSInheritedState testa a forma L (ML-2I) sob estados PS
// herdados: Continue, Stop, StrictMode Latest, ReadOnly-Stop, WhatIf+Stop e ConfirmLow+Stop,
// sozinhos e combinados, via duas formas de herança:
//
//   - via -Command: o estado é injetado como prefixo na linha de comando.
//   - via perfil: o estado é definido num perfil temporário em XDG_CONFIG_HOME.
//
// Estratégia de cobertura (ML-2D): a matriz completa roda apenas para git-branch;
// os outros 3 subcomandos recebem apenas um smoke test (EAP=Stop via -Command,
// allow e deny) pois TestGuardLineShellMatrix_PSConstantsStructural garante que
// todos partilham o mesmo preamble e sufixo.
//
// Reconciliação (REQ-2026-11-13 ML-2C/ML-2D/ML-2I):
//   - Braço -Command: afirma que a forma L sai com RC correto (0 = allow, 2 = deny ou
//     ausente) mesmo quando EAP=Continue/Stop, ReadOnly-EAP, WhatIf=$true ou
//     ConfirmPreference=Low estão ativos antes da linha — o preamble L neutraliza sem
//     emitir ruído no stderr POSIX. Nota: estes braços rodam com -NonInteractive;
//     o caso sem -NonInteractive sob Confirm Low/Medium é coberto por
//     TestGuardLineShellMatrix_LContraArm_ConfirmLow.
//   - Braço perfil: afirma que a forma L sobrevive ao mesmo estado imposto pelo perfil
//     do usuário.
//   - Smoke test: afirma que os outros 3 subcomandos sobrevivem a EAP=Stop via -Command.
func TestGuardLineShellMatrix_PSInheritedState(t *testing.T) {
	psBin, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh não encontrado — braço PS de estado herdado não aplicável neste ambiente")
	}
	ensurePSWarmup(t, psBin)

	// Condições via -Command: prefix prepended à linha de guard.
	// Continue é a baseline; os outros estados são os que a forma K deve neutralizar.
	type cmdCond struct {
		name   string
		prefix string
	}
	cmdConditions := []cmdCond{
		{"continue-via-command", "$ErrorActionPreference='Continue'; "},
		{"stop-via-command", "$ErrorActionPreference='Stop'; "},
		{"strictmode-via-command", "Set-StrictMode -Version Latest; "},
		{"stop-and-strictmode-via-command", "Set-StrictMode -Version Latest; $ErrorActionPreference='Stop'; "},
		{"readonly-stop-via-command", "Set-Variable -Name ErrorActionPreference -Value Stop -Option ReadOnly; "},
		{"whatif-stop-via-command", "$WhatIfPreference = $true; $ErrorActionPreference = 'Stop'; "},
		{"confirm-low-stop-via-command", "$ConfirmPreference = 'Low'; $ErrorActionPreference = 'Stop'; "},
	}

	// Condições via perfil: conteúdo PS escrito em Microsoft.PowerShell_profile.ps1.
	type profCond struct {
		name    string
		content string
	}
	profConditions := []profCond{
		{"continue-via-profile", "$ErrorActionPreference = 'Continue'\n"},
		{"stop-via-profile", "$ErrorActionPreference = 'Stop'\n"},
		{"strictmode-via-profile", "Set-StrictMode -Version Latest\n"},
		{"stop-and-strictmode-via-profile", "Set-StrictMode -Version Latest\n$ErrorActionPreference = 'Stop'\n"},
		{"readonly-stop-via-profile", "Set-Variable -Name ErrorActionPreference -Value Stop -Option ReadOnly\n"},
		{"whatif-stop-via-profile", "$WhatIfPreference = $true\n$ErrorActionPreference = 'Stop'\n"},
		{"confirm-low-stop-via-profile", "$ConfirmPreference = 'Low'\n$ErrorActionPreference = 'Stop'\n"},
	}

	// Encontrar a constante git-branch para a matriz completa.
	gitBranchLine := ""
	for _, g := range guardShellMatrix {
		if g.name == "git-branch" {
			gitBranchLine = g.line
			break
		}
	}
	if gitBranchLine == "" {
		t.Fatal("git-branch not found in guardShellMatrix")
	}

	// ── Matriz completa: somente git-branch ──────────────────────────────────────────
	// Exercita todas as 7 condições × 2 meios (comando e perfil) × 4 casos.
	// Todos os 4 casos (RC0, RC2, RC1, absent) rodam em paralelo dentro de cada
	// condição: absent usa extraEnv PATH em vez de t.Setenv, o que habilita t.Parallel()
	// nele e, por consequência, t.Parallel() em cond.name e pcond.name também.
	t.Run("git-branch", func(t *testing.T) {

		// ── Braço -Command ──────────────────────────────────────────────────────
		for _, cond := range cmdConditions {
			cond := cond
			t.Run(cond.name, func(t *testing.T) {
				// t.Parallel() é seguro: o corpo apenas define 'line' e lança 4 sub-testes
				// paralelos. Nenhum dos sub-testes chama t.Setenv; absent usa PATH via
				// extraEnv do filho — não modifica o ambiente do processo.
				t.Parallel()
				line := cond.prefix + gitBranchLine

				t.Run("RC0-allow", func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					code, _, _ := runGuardLinePS(t, psBin, line, dir, 0)
					if code != 0 {
						t.Errorf("git-branch/%s: RC0-allow: esperado exit 0, obteve %d", cond.name, code)
					}
					if _, sErr := os.Stat(filepath.Join(dir, "stdin.out")); os.IsNotExist(sErr) {
						t.Errorf("git-branch/%s: RC0-allow: guard não rodou (stdin.out ausente)", cond.name)
					}
				})

				t.Run("RC2-deny", func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					code, _, _ := runGuardLinePS(t, psBin, line, dir, 2)
					if code != 2 {
						t.Errorf("git-branch/%s: RC2-deny: esperado exit 2, obteve %d", cond.name, code)
					}
					if _, sErr := os.Stat(filepath.Join(dir, "stdin.out")); os.IsNotExist(sErr) {
						t.Errorf("git-branch/%s: RC2-deny: guard não rodou", cond.name)
					}
				})

				t.Run("RC1-deny-normalizado", func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					code, _, _ := runGuardLinePS(t, psBin, line, dir, 1)
					if code != 2 {
						t.Errorf("git-branch/%s: RC1-deny-normalizado: esperado exit 2, obteve %d", cond.name, code)
					}
				})

				t.Run("absent", func(t *testing.T) {
					// t.Parallel() é seguro: PATH é passado via extraEnv ao processo filho;
					// não chamamos t.Setenv nem os.Setenv — o PATH do processo pai não é alterado.
					// Guarda de vacuidade: escanear absentPath diretamente (não exec.LookPath, que
					// veria o PATH do processo pai, não o PATH que o filho receberá).
					t.Parallel()
					absentPath := buildPSPath("")
					for _, d := range filepath.SplitList(absentPath) {
						if _, statErr := os.Stat(filepath.Join(d, "trackfw")); statErr == nil {
							t.Fatalf("trackfw encontrado em %s — caso absent é vacuidade", filepath.Join(d, "trackfw"))
						}
						if runtime.GOOS == "windows" {
							for _, ext := range []string{".exe", ".cmd"} {
								if _, statErr := os.Stat(filepath.Join(d, "trackfw"+ext)); statErr == nil {
									t.Fatalf("trackfw%s em absentPath — caso absent é vacuidade", ext)
								}
							}
						}
					}
					code, _, _ := runGuardLinePSNoProfile(t, psBin, line, "", 0, []string{"PATH=" + absentPath})
					if code != 2 {
						t.Errorf("git-branch/%s: absent: esperado exit 2, obteve %d", cond.name, code)
					}
				})
			})
		}

		// ── Braço perfil ────────────────────────────────────────────────────────────
		// No Windows o perfil PS está em %USERPROFILE%\Documents\PowerShell\ (pwsh 7)
		// ou %USERPROFILE%\Documents\WindowsPowerShell\ (powershell.exe 5.1); isolar
		// sobrescrevendo USERPROFILE+HOME para t.TempDir().
		// No macOS/Linux usa XDG_CONFIG_HOME. Um marcador escrito pelo próprio perfil
		// confirma carregamento; ausência do marcador → t.Skipf com prova (c).
		if runtime.GOOS == "windows" {
			type winPSEntry struct {
				name, bin, profileSubdir string
			}
			winEntries := []winPSEntry{
				{"pwsh", "pwsh", filepath.Join("Documents", "PowerShell")},
				{"powershell", "powershell", filepath.Join("Documents", "WindowsPowerShell")},
			}
			for _, wps := range winEntries {
				wps := wps
				t.Run("profile-windows/"+wps.name, func(t *testing.T) {
					winBin, lookErr := exec.LookPath(wps.bin)
					if lookErr != nil {
						t.Skipf("%s não encontrado em PATH (%v) — skip (b): probe de capacidade", wps.bin, lookErr)
					}
					ensurePSWarmup(t, winBin)
					for _, pcond := range profConditions {
						pcond := pcond
						t.Run(pcond.name, func(t *testing.T) {
							t.Parallel()
							userprofileDir := t.TempDir()
							profileDir := filepath.Join(userprofileDir, wps.profileSubdir)
							if mkErr := os.MkdirAll(profileDir, 0755); mkErr != nil {
								t.Fatalf("criando diretório de perfil Windows: %v", mkErr)
							}
							markerPath := filepath.Join(userprofileDir, "profile_loaded")
							profileContent := `New-Item -Path "` + markerPath + `" -ItemType File -Force | Out-Null` + "\n" +
								pcond.content
							profileFile := filepath.Join(profileDir, "Microsoft.PowerShell_profile.ps1")
							if wErr := os.WriteFile(profileFile, []byte(profileContent), 0644); wErr != nil {
								t.Fatalf("escrevendo perfil Windows: %v", wErr)
							}
							upEnv := []string{
								"USERPROFILE=" + userprofileDir,
								"HOME=" + userprofileDir,
							}
							// Sentinel: also capture the resolved $PROFILE path for concrete (c) proof.
							resolvedProfileFile := filepath.Join(userprofileDir, "resolved_profile.txt")
							// Use [IO.File]::WriteAllText — not Set-Content, which is suppressed by WhatIf.
							sentinelCmd := `[IO.File]::WriteAllText('` + resolvedProfileFile + `', $PROFILE); exit 0`
							psConcurrencySem <- struct{}{}
							checkR := execPSCommandOpts(winBin, sentinelCmd, "", 0, psCallDeadlineCeil, false, upEnv)
							<-psConcurrencySem
							if checkR.timedOut || checkR.runErr != nil {
								t.Skipf("verificação de carregamento de perfil falhou: timedOut=%v runErr=%v — pulando braço de perfil Windows", checkR.timedOut, checkR.runErr)
							}
							if _, mErr := os.Stat(markerPath); os.IsNotExist(mErr) {
								// (c): USERPROFILE override not honoured; marker absent is the measurement.
								// $PROFILE resolved to whatever resolvedProfileFile contains.
								resolvedProfile := "(unreadable)"
								if rp, rpErr := os.ReadFile(resolvedProfileFile); rpErr == nil {
									resolvedProfile = string(rp)
								}
								t.Skipf("perfil não carregado via USERPROFILE override (marcador ausente) — (c): "+
									"USERPROFILE=%s mas $PROFILE resolveu para %q em vez de %q; "+
									"PS carrega perfil do USERPROFILE original do sistema",
									userprofileDir, resolvedProfile, profileFile)
							}
							if rmErr := os.Remove(markerPath); rmErr != nil {
								t.Fatalf("removendo marcador antes de sub-teste: %v", rmErr)
							}

							t.Run("RC0-allow", func(t *testing.T) {
								t.Parallel()
								dir := t.TempDir()
								makeFakeTrackfwForOS(t, dir)
								code, _, stdinCaptured := runGuardLinePSNoProfile(t, winBin, gitBranchLine, dir, 0, upEnv)
								if code != 0 {
									t.Errorf("git-branch/%s: RC0-allow (perfil Windows %s): esperado exit 0, obteve %d", pcond.name, wps.name, code)
								}
								if _, sErr := os.Stat(filepath.Join(dir, "stdin.out")); os.IsNotExist(sErr) {
									t.Errorf("git-branch/%s: RC0-allow (perfil Windows %s): guard não rodou", pcond.name, wps.name)
								}
								// REQ-IMP-4: bytes recebidos pelo stub == bytes enviados (normalizado CRLF).
								want := strings.TrimRight(stdinPayload, "\r\n")
								if stdinCaptured != want {
									t.Errorf("REQ-IMP-4 RC0-allow: stdin não preservado: got %q, want %q", stdinCaptured, want)
								}
							})

							t.Run("RC2-deny", func(t *testing.T) {
								t.Parallel()
								dir := t.TempDir()
								makeFakeTrackfwForOS(t, dir)
								code, _, stdinCaptured := runGuardLinePSNoProfile(t, winBin, gitBranchLine, dir, 2, upEnv)
								if code != 2 {
									t.Errorf("git-branch/%s: RC2-deny (perfil Windows %s): esperado exit 2, obteve %d", pcond.name, wps.name, code)
								}
								// REQ-IMP-4: bytes recebidos pelo stub == bytes enviados (normalizado CRLF).
								want := strings.TrimRight(stdinPayload, "\r\n")
								if stdinCaptured != want {
									t.Errorf("REQ-IMP-4 RC2-deny: stdin não preservado: got %q, want %q", stdinCaptured, want)
								}
							})

							t.Run("RC1-deny-normalizado", func(t *testing.T) {
								t.Parallel()
								dir := t.TempDir()
								makeFakeTrackfwForOS(t, dir)
								code, _, _ := runGuardLinePSNoProfile(t, winBin, gitBranchLine, dir, 1, upEnv)
								if code != 2 {
									t.Errorf("git-branch/%s: RC1-deny-normalizado (perfil Windows %s): esperado exit 2, obteve %d", pcond.name, wps.name, code)
								}
							})
						})
					}
				})
			}
			return
		}

		for _, pcond := range profConditions {
			pcond := pcond
			t.Run(pcond.name, func(t *testing.T) {
				// t.Parallel() é seguro: o corpo cria seu próprio xdgDir (t.TempDir()),
				// não usa t.Setenv; absent passa PATH via extraEnv ao processo filho.
				// Os 7 pcond.name correm em paralelo — incluindo as verificações de
				// carregamento de perfil (execPSCommandOpts), que antes eram serializadas.
				t.Parallel()
				xdgDir := t.TempDir()
				profileDir := filepath.Join(xdgDir, "powershell")
				if mkErr := os.MkdirAll(profileDir, 0755); mkErr != nil {
					t.Fatalf("criando diretório de perfil: %v", mkErr)
				}
				markerPath := filepath.Join(xdgDir, "profile_loaded")
				// Perfil: escreve marcador ANTES de definir o estado — WhatIf=$true suprimiria
				// New-Item se o estado fosse definido primeiro; Confirm-Low+Stop e ReadOnly-Stop
				// também exigem esta ordem fixa.
				profileContent := `New-Item -Path "` + markerPath + `" -ItemType File -Force | Out-Null` + "\n" +
					pcond.content
				profileFile := filepath.Join(profileDir, "Microsoft.PowerShell_profile.ps1")
				if wErr := os.WriteFile(profileFile, []byte(profileContent), 0644); wErr != nil {
					t.Fatalf("escrevendo perfil: %v", wErr)
				}
				xdgEnv := []string{"XDG_CONFIG_HOME=" + xdgDir}

				// Verificar se o perfil carrega (exit 0, marcador criado).
				psConcurrencySem <- struct{}{}
				checkR := execPSCommandOpts(psBin, "exit 0", "", 0, psCallDeadlineCeil, false, xdgEnv)
				<-psConcurrencySem
				if checkR.timedOut || checkR.runErr != nil {
					t.Skipf("verificação de carregamento de perfil falhou: timedOut=%v runErr=%v — pulando braço de perfil", checkR.timedOut, checkR.runErr)
				}
				if _, mErr := os.Stat(markerPath); os.IsNotExist(mErr) {
					t.Skipf("perfil não carregado (marcador ausente em %s) — XDG_CONFIG_HOME pode não ser suportado nesta versão do pwsh; pulando braço de perfil", markerPath)
				}

				// Remover marcador da verificação de carregamento. O perfil o cria
				// antes do estado (primeira linha do perfil — ver profileContent acima),
				// garantindo que New-Item execute antes que WhatIf, Confirm ou Stop
				// possam suprimi-lo.
				if rmErr := os.Remove(markerPath); rmErr != nil {
					t.Fatalf("removendo marcador antes de sub-teste: %v", rmErr)
				}

				t.Run("RC0-allow", func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					code, _, stdinCaptured := runGuardLinePSNoProfile(t, psBin, gitBranchLine, dir, 0, xdgEnv)
					if code != 0 {
						t.Errorf("git-branch/%s: RC0-allow (perfil): esperado exit 0, obteve %d", pcond.name, code)
					}
					if _, sErr := os.Stat(filepath.Join(dir, "stdin.out")); os.IsNotExist(sErr) {
						t.Errorf("git-branch/%s: RC0-allow (perfil): guard não rodou", pcond.name)
					}
					// REQ-IMP-4: bytes recebidos pelo stub == bytes enviados (normalizado CRLF).
					want := strings.TrimRight(stdinPayload, "\r\n")
					if stdinCaptured != want {
						t.Errorf("REQ-IMP-4 RC0-allow (perfil): stdin não preservado: got %q, want %q", stdinCaptured, want)
					}
				})

				t.Run("RC2-deny", func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					code, _, stdinCaptured := runGuardLinePSNoProfile(t, psBin, gitBranchLine, dir, 2, xdgEnv)
					if code != 2 {
						t.Errorf("git-branch/%s: RC2-deny (perfil): esperado exit 2, obteve %d", pcond.name, code)
					}
					if _, sErr := os.Stat(filepath.Join(dir, "stdin.out")); os.IsNotExist(sErr) {
						t.Errorf("git-branch/%s: RC2-deny (perfil): guard não rodou", pcond.name)
					}
					// REQ-IMP-4: bytes recebidos pelo stub == bytes enviados (normalizado CRLF).
					want := strings.TrimRight(stdinPayload, "\r\n")
					if stdinCaptured != want {
						t.Errorf("REQ-IMP-4 RC2-deny (perfil): stdin não preservado: got %q, want %q", stdinCaptured, want)
					}
				})

				t.Run("RC1-deny-normalizado", func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					code, _, _ := runGuardLinePSNoProfile(t, psBin, gitBranchLine, dir, 1, xdgEnv)
					if code != 2 {
						t.Errorf("git-branch/%s: RC1-deny-normalizado (perfil): esperado exit 2, obteve %d", pcond.name, code)
					}
				})

				t.Run("absent", func(t *testing.T) {
					// t.Parallel() é seguro (ver raciocínio do braço -Command): PATH
					// é passado via extraEnv; não usamos t.Setenv.
					// append(xdgEnv, ...) preserva XDG_CONFIG_HOME para que o perfil
					// ainda carregue — a ausência de trackfw não depende do EAP do perfil.
					t.Parallel()
					absentPath := buildPSPath("")
					for _, d := range filepath.SplitList(absentPath) {
						if _, statErr := os.Stat(filepath.Join(d, "trackfw")); statErr == nil {
							t.Fatalf("trackfw em %s — caso absent é vacuidade", filepath.Join(d, "trackfw"))
						}
						if runtime.GOOS == "windows" {
							for _, ext := range []string{".exe", ".cmd"} {
								if _, statErr := os.Stat(filepath.Join(d, "trackfw"+ext)); statErr == nil {
									t.Fatalf("trackfw%s em absentPath — caso absent é vacuidade", ext)
								}
							}
						}
					}
					code, _, _ := runGuardLinePSNoProfile(t, psBin, gitBranchLine, "", 0,
						append(xdgEnv, "PATH="+absentPath))
					if code != 2 {
						t.Errorf("git-branch/%s: absent (perfil): esperado exit 2, obteve %d", pcond.name, code)
					}
				})
			})
		}
	})

	// ── Smoke test: outros 3 subcomandos (EAP=Stop via -Command: allow e deny) ──────
	// TestGuardLineShellMatrix_PSConstantsStructural garante que credential, write-scope
	// e credential-global têm o mesmo preamble e sufixo que git-branch. Portanto, basta
	// exercitar um estado representativo (Stop via -Command) com RC0-allow e RC2-deny
	// para confirmar que o guard roda e propaga o exit code corretamente.
	// Os 3 smoke tests rodam em paralelo entre si (nenhum usa t.Setenv).
	const stopPrefixSmoke = "$ErrorActionPreference='Stop'; "
	for _, guard := range guardShellMatrix {
		if guard.name == "git-branch" {
			continue
		}
		guard := guard
		t.Run(guard.name+"-smoke-stop", func(t *testing.T) {
			t.Parallel()
			smokeLine := stopPrefixSmoke + guard.line

			t.Run("RC0-allow", func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				makeFakeTrackfwForOS(t, dir)
				code, _, _ := runGuardLinePS(t, psBin, smokeLine, dir, 0)
				if code != 0 {
					t.Errorf("%s smoke-stop: RC0-allow: esperado exit 0, obteve %d", guard.name, code)
				}
				if _, sErr := os.Stat(filepath.Join(dir, "stdin.out")); os.IsNotExist(sErr) {
					t.Errorf("%s smoke-stop: RC0-allow: guard não rodou (stdin.out ausente)", guard.name)
				}
			})

			t.Run("RC2-deny", func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				makeFakeTrackfwForOS(t, dir)
				code, _, _ := runGuardLinePS(t, psBin, smokeLine, dir, 2)
				if code != 2 {
					t.Errorf("%s smoke-stop: RC2-deny: esperado exit 2, obteve %d", guard.name, code)
				}
				if _, sErr := os.Stat(filepath.Join(dir, "stdin.out")); os.IsNotExist(sErr) {
					t.Errorf("%s smoke-stop: RC2-deny: guard não rodou (stdin.out ausente)", guard.name)
				}
			})
		})
	}
}

// TestGuardLineShellMatrix_HContraArm_ReadOnlyStop demonstra que a forma H (nunca released,
// substituída por ML-2C/forma K, e depois por ML-2I/forma L) falha sob PowerShell com EAP
// configurado como ReadOnly=Stop — saindo com exit ≠ 0 e sem rodar o guard. Esta é a "prova
// de mordida" do ML-2C: se a forma H passasse sob ReadOnly-Stop, o -Force em K/L seria
// desnecessário.
//
// A forma H é construída a partir de peças da forma L (nunca como literal contíguo) para evitar
// que o teste em si seja um veículo de propagação da forma defeituosa:
//
//	hForm = hPreamble + L[após os três primeiros segmentos de L (kSeg1+kSeg1b+kSeg2)]
//
// Reconciliação (REQ-2026-11-13 ML-2C/ML-2I): afirma que a forma H sai com exit ≠ 0 sob
// ReadOnly-Stop (Set-Variable -Option ReadOnly com Value Stop), confirmando que -Force
// em L é necessário para sobrescrever a variável somente-leitura.
func TestGuardLineShellMatrix_HContraArm_ReadOnlyStop(t *testing.T) {
	psBin, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh não encontrado — contra-braço H ReadOnly-Stop não aplicável neste ambiente")
	}
	ensurePSWarmup(t, psBin)

	// Prefixo ReadOnly-Stop: define EAP como Stop com opção ReadOnly (simula perfil restritivo).
	const readOnlyStopPrefix = "Set-Variable -Name ErrorActionPreference -Value Stop -Option ReadOnly; "

	// Segmentos do L que constituem o preamble (devem ser mantidos sincronizados com ml1dPrefix).
	// kSeg1b é o segmento de ConfirmPreference adicionado pela forma L (ML-2I); coincide com
	// ml1dConfirmSeg (definido no escopo do pacote). Duplicado aqui para explicitar a estrutura.
	const kSeg1 = "set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0; "
	const kSeg1b = ml1dConfirmSeg // "set ConfirmPreference None -Force -WhatIf:0 -Confirm:0; "
	const kSeg2 = "set +eu; "
	// Preamble da forma H: ordena set +eu primeiro, sem -Force/-WhatIf:0/-Confirm:0.
	const hPreamble = "set +eu; set ErrorActionPreference Continue; "

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			// Sanidade: confirmar que a forma L começa com os três segmentos esperados (kSeg1+kSeg1b+kSeg2).
			if !strings.HasPrefix(guard.line, kSeg1+kSeg1b+kSeg2) {
				t.Fatalf("forma L de %q não começa com kSeg1+kSeg1b+kSeg2 — forma L foi alterada?", guard.name)
			}

			// Construir forma H a partir das peças de L (H nunca foi released).
			// H = hPreamble + resto após kSeg1+kSeg1b+kSeg2 (sem os três segmentos de cabeçalho).
			hForm := hPreamble + guard.line[len(kSeg1)+len(kSeg1b)+len(kSeg2):]

			// Verificar sanidade: H não deve conter -Force.
			if strings.Contains(hForm, "-Force") {
				t.Fatalf("forma H construída para %q ainda contém -Force — verificar derivação", guard.name)
			}

			// Executar a forma H sob ReadOnly-Stop via -Command.
			lineUnderReadOnly := readOnlyStopPrefix + hForm

			dir := t.TempDir()
			makeFakeTrackfwForOS(t, dir)

			r := execPSCommand(psBin, lineUnderReadOnly, dir, 2, psCallDeadlineCeil)
			if r.timedOut {
				t.Skipf("contra-braço H ReadOnly-Stop: timeout ao executar forma H sob ReadOnly-Stop — pulando")
			}
			if r.runErr != nil {
				t.Fatalf("contra-braço H ReadOnly-Stop: erro de execução: %v", r.runErr)
			}

			// A forma H DEVE falhar sob ReadOnly-Stop (exit ≠ 0).
			if r.exitCode == 0 {
				t.Errorf(
					"contra-braço H ReadOnly-Stop (%s): forma H saiu com exit 0 sob ReadOnly-Stop — "+
						"esperado exit ≠ 0; o -Force em K pode ser desnecessário",
					guard.name,
				)
			} else {
				t.Logf("contra-braço H ReadOnly-Stop (%s): forma H saiu com exit %d (esperado ≠ 0) — defeito H confirmado", guard.name, r.exitCode)
			}

			// Confirmar que o guard nunca rodou (stdin.out não deve existir).
			if _, statErr := os.Stat(filepath.Join(dir, "stdin.out")); statErr == nil {
				t.Errorf(
					"contra-braço H ReadOnly-Stop (%s): stdin.out existe — guard rodou quando não deveria",
					guard.name,
				)
			}
		})
	}
}

// TestGuardLineShellMatrix_D18NovaContraArm_Stop demonstra que a forma ML-1A (REQ-2026-11-13 ML-1A,
// substituída por ML-1C, ML-1C2 e depois ML-2C/forma K) falha sob PowerShell com EAP=Stop herdado —
// saindo com exit ≠ 0 e sem rodar o guard. Esta é a "prova de mordida" do ML-2C: se a forma ML-1A
// passasse sob Stop, o fix seria desnecessário e este braço seria falso-verde.
//
// A forma ML-1A é construída a partir de guardforms.LegacyD18*PSPOSIX (constante estável):
//
//	ml1aLine = setEuPrefix + guardforms.LegacyD18*PSPOSIX
//
// Reconciliação (REQ-2026-11-13 ML-1C/ML-2C): afirma que a forma ML-1A (`set +eu 2>${null-/dev/null}; …`)
// sai com exit ≠ 0 sob EAP=Stop — `emulate -R sh 2>${null-/dev/null}` em ML-1A usa ${null-/dev/null}
// como alvo de redirecionamento; sob Stop, a CommandNotFound do `emulate` vira exceção terminante que
// propaga antes que `2>${null-/dev/null}` possa suprimi-la, saindo exit 1. ML-2C/forma K mantém
// null=/dev/null e adiciona -Force/-WhatIf:0/-Confirm:0 no preamble — zero stderr em POSIX.
func TestGuardLineShellMatrix_D18NovaContraArm_Stop(t *testing.T) {
	psBin, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh não encontrado — contra-braço ML-1A Stop não aplicável neste ambiente")
	}
	ensurePSWarmup(t, psBin)

	// Mapeamento guard-name → LegacyD18 form (guardforms).
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
	d18ByName := make(map[string]string, len(d18Matrix))
	for _, e := range d18Matrix {
		d18ByName[e.name] = e.legLine
	}

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			// Sanidade: verificar que ml1dPrefix está presente — forma atual deve ser L.
			if !strings.HasPrefix(guard.line, ml1dPrefix) {
				t.Fatalf("constante %q não começa com ml1dPrefix %q — forma L (ML-2I) foi removida?",
					guard.name, ml1dPrefix)
			}

			// Construir ML-1A a partir de LegacyD18 (constante estável com 2>${null-/dev/null}).
			legD18, ok := d18ByName[guard.name]
			if !ok {
				t.Fatalf("d18ByName não tem entrada para %q — atualizar mapeamento", guard.name)
			}
			ml1aLine := setEuPrefix + legD18

			// Verificar que a derivação produziu o prefixo esperado (guard de sanidade).
			if !strings.HasPrefix(ml1aLine, setEuPrefix) {
				t.Fatalf("derivação ML-1A inconsistente: ml1aLine não começa com setEuPrefix; got %q", ml1aLine[:min(len(ml1aLine), 60)])
			}

			// Executar a forma ML-1A sob EAP=Stop via -Command.
			stopPrefix := "$ErrorActionPreference='Stop'; "
			lineUnderStop := stopPrefix + ml1aLine

			dir := t.TempDir()
			makeFakeTrackfwForOS(t, dir)

			r := execPSCommand(psBin, lineUnderStop, dir, 2, psCallDeadlineCeil)
			if r.timedOut {
				t.Skipf("contra-braço ML-1A Stop: timeout ao executar forma ML-1A sob Stop — pulando")
			}
			if r.runErr != nil {
				t.Fatalf("contra-braço ML-1A Stop: erro de execução: %v", r.runErr)
			}

			// A forma ML-1A DEVE falhar sob EAP=Stop (exit ≠ 0).
			if r.exitCode == 0 {
				t.Errorf(
					"contra-braço ML-1A Stop (%s): forma ML-1A saiu com exit 0 sob EAP=Stop — "+
						"esperado exit ≠ 0; verificar se a forma ML-1A foi alterada ou se o Stop não está ativo",
					guard.name,
				)
			} else {
				t.Logf("contra-braço ML-1A Stop (%s): forma ML-1A saiu com exit %d sob EAP=Stop (esperado ≠ 0) — defeito ML-1A confirmado", guard.name, r.exitCode)
			}

			// Confirmar que o guard nunca rodou (stdin.out não deve existir).
			stdinOutPath := filepath.Join(dir, "stdin.out")
			if _, statErr := os.Stat(stdinOutPath); statErr == nil {
				t.Errorf(
					"contra-braço ML-1A Stop (%s): stdin.out existe — guard rodou quando não deveria; "+
						"a forma ML-1A pode estar passando sob Stop por outro mecanismo",
					guard.name,
				)
			}
		})
	}
}

// TestGuardLineShellMatrix_PreambleNoStderr afirma que a forma L (ML-2I) não produz ruído de
// preamble no stderr em shells POSIX — o stderr é byte-idêntico ao que o guard fake escreve.
// Isto é: zero linhas extras antes ou depois do conteúdo normal do guard.
// O novo segmento de ConfirmPreference (ml1dConfirmSeg) é POSIX-inerte: `set ConfirmPreference
// None ...` em shells POSIX define argumentos posicionais, exit 0, zero stderr.
//
// Contra-braço: a forma CAND_F (ML-1C, nunca released) falha esta asserção em shells POSIX
// porque `Set-StrictMode -Off` sem redirecionamento emite uma linha de "command not found"
// antes do output do guard. A forma está em candFCredentialPSPOSIX dentro do sub-teste.
//
// Reconciliação (REQ-2026-11-13 ML-2C/ML-2I): afirma que nenhuma das formas atuais emite linhas
// adicionais de stderr por chamada POSIX (zero linhas de preamble, independente de allow/deny).
// TestGuardLineShellMatrix_PreambleNoStderr: GOOS skip removido; each shell has LookPath probe inside.
// Classificação (b) — sh/bash disponíveis no windows-latest; outros shells pulam via probe.
func TestGuardLineShellMatrix_PreambleNoStderr(t *testing.T) {
	// FRCs a testar: allow (0), deny canônico (2), deny não-canônico (1).
	// O stub SEMPRE escreve "motivo-deny-fake" no stderr independente do FAKE_RC.
	type frcCase struct {
		name string
		frc  int
	}
	frcCases := []frcCase{
		{"allow", 0},
		{"deny", 2},
		{"frc1", 1},
	}

	// O stub trackfw escreve exatamente "motivo-deny-fake" (sem newline) no stderr.
	// Stderr esperado = apenas o que o guard escreve; zero ruído de preamble.
	const expectedStderr = "motivo-deny-fake"

	// ── Braço L (forma atual, ML-2I) ─────────────────────────────────────────────────
	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			for _, sh := range shellEntries {
				sh := sh
				t.Run(sh.name, func(t *testing.T) {
					shellPath, lookErr := exec.LookPath(sh.bin)
					if lookErr != nil {
						t.Skipf("%s não encontrado em PATH — shell ausente neste ambiente", sh.bin)
					}

					var xenv []string
					if sh.extraEnvFn != nil {
						xenv = sh.extraEnvFn(t)
					}

					for _, fc := range frcCases {
						fc := fc
						// t.Parallel() é seguro: cada frcCase usa t.TempDir() próprio e não usa t.Setenv.
						t.Run(fc.name, func(t *testing.T) {
							t.Parallel()
							dir := t.TempDir()
							makeFakeTrackfw(t, dir)
							_, stderr, _ := runGuardLine(t, shellPath, sh.flags, xenv, guard.line, dir, fc.frc)
							if stderr != expectedStderr {
								t.Errorf(
									"preamble noise detectado (%s/%s/%s): got %q, want %q",
									guard.name, sh.name, fc.name, stderr, expectedStderr,
								)
							}
						})
					}
				})
			}
		})
	}

	// ── Contra-braço CAND_F ───────────────────────────────────────────────────
	// A forma ML-1C (CAND_F) deve produzir stderr ≠ "motivo-deny-fake" em sh,
	// pois `Set-StrictMode -Off` sem redirecionamento emite "command not found"
	// antes do output do guard.
	//
	// CAND_F foi candidato em REQ-2026-11-13 ML-1C mas NUNCA foi released/commitado:
	// `git log -S'Set-StrictMode -Off;'` vem vazio. A forma está aqui APENAS como
	// contra-braço para provar que o teste distingue ruído de preamble de ausência de
	// ruído; ela não consta no registry de guardforms (remoção em ML-1C3).
	//
	// candFCredentialPSPOSIX é a forma CAND_F (ML-1C, nunca released) para credential guard.
	// Mantida SOMENTE como contra-braço; NÃO usar fora deste bloco.
	const candFCredentialPSPOSIX = `set +eu; Set-StrictMode -Off; set ErrorActionPreference Continue; emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	// Usa o credential guard com deny (frc=2) como caso representativo.
	t.Run("cand_f-preamble-noise", func(t *testing.T) {
		shPath, lookErr := exec.LookPath("sh")
		if lookErr != nil {
			t.Skip("sh não encontrado — contra-braço CAND_F não aplicável")
		}

		dir := t.TempDir()
		makeFakeTrackfw(t, dir)
		_, stderr, _ := runGuardLine(t, shPath, nil, nil, candFCredentialPSPOSIX, dir, 2)
		// O mecanismo deve ser verificado: stderr deve conter a mensagem "command not found" de
		// Set-StrictMode (ruído de preamble) E terminar com a saída do guard (motivo-deny-fake).
		// Verificar só "stderr != motivo-deny-fake" não é suficiente; cobrir os dois componentes
		// garante que o ruído veio do preamble e não de uma falha de execução (stub ausente, etc.).
		if !strings.Contains(stderr, "Set-StrictMode") {
			t.Errorf(
				"contra-braço CAND_F: stderr=%q não contém \"Set-StrictMode\" — ruído de preamble não detectado (stub ausente?)",
				stderr,
			)
		}
		if !strings.HasSuffix(stderr, expectedStderr) {
			t.Errorf(
				"contra-braço CAND_F: stderr=%q não termina com %q — guard não executou ou saída corrompida",
				stderr, expectedStderr,
			)
		}
		if t.Failed() {
			return
		}
		t.Logf("contra-braço CAND_F: stderr=%q — ruído de preamble confirmado, guard executou", stderr)
	})
}

// execPSCommandInteractive executa psBin SEM -NonInteractive, permitindo que prompts de
// confirmação do PowerShell possam consumir stdin. Usado exclusivamente para testar o
// comportamento de forma L vs forma K sob $ConfirmPreference='Low'/'Medium' sem -NonInteractive.
//
// addNoProfile=true adiciona -NoProfile; addNoProfile=false omite ambos -NoProfile e -NonInteractive.
// extraEnv: variáveis adicionais de ambiente a injetar (ex: XDG_CONFIG_HOME=…, PATH=…).
//
// AVISO: Se o PS exibir um prompt de confirmação e stdin for pipes.Reader, o prompt drena os bytes
// disponíveis. O timeout via context.WithTimeout impede que o processo fique preso indefinidamente.
func execPSCommandInteractive(psBin, command, fakeDir string, fakeRC int, timeout time.Duration, addNoProfile bool, extraEnv []string) psExecResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var args []string
	if addNoProfile {
		args = []string{"-NoProfile", "-Command", command} // sem -NonInteractive
	} else {
		args = []string{"-Command", command} // sem -NoProfile nem -NonInteractive
	}
	cmd := exec.CommandContext(ctx, psBin, args...) //nolint:gosec
	cmd.WaitDelay = psWaitDelay

	pathVal := buildPSPath(fakeDir)

	extraHasPath := false
	for _, e := range extraEnv {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			extraHasPath = true
			break
		}
	}

	base := os.Environ()
	filtered := make([]string, 0, len(base)+2+len(extraEnv))
	for _, e := range base {
		upper := strings.ToUpper(e)
		if strings.HasPrefix(upper, "PATH=") || strings.HasPrefix(upper, "FAKE_RC=") {
			continue
		}
		filtered = append(filtered, e)
	}
	if !extraHasPath {
		filtered = append(filtered, "PATH="+pathVal)
	}
	if fakeDir != "" {
		filtered = append(filtered, fmt.Sprintf("FAKE_RC=%d", fakeRC))
	}
	filtered = append(filtered, extraEnv...)
	// Isolar cache JIT por invocação (ML-1D): cada pwsh recebe XDG_CACHE_HOME próprio para
	// evitar corrida na escrita de StartupProfileData-* (FileLoadException com nomes embaralhados).
	// Aplica-se após extraEnv para também filtrar XDG_CACHE_HOME de extraEnv, se presente.
	filtered, cacheCleanup := newPwshCacheEnv(filtered)
	defer cacheCleanup()
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

// TestGuardLineShellMatrix_LContraArm_ConfirmLow testa a forma L (ML-2I) sob
// $ConfirmPreference='Low' e 'Medium' WITHOUT -NonInteractive, via duas formas de herança:
//
//   - via -Command: o estado é injetado como prefixo na linha de comando.
//   - via perfil: o estado é definido num perfil temporário em XDG_CONFIG_HOME
//     (somente em não-Windows — no Windows o perfil via XDG_CONFIG_HOME não funciona).
//
// O teste cobre apenas git-branch (a matriz estrutural garante que os outros 3 partilham
// o mesmo preamble). Os FRCs RC0 e RC2 são exercitados para ambas as condições.
//
// Contra-braço: a forma K (ML-2C, sem o segmento set ConfirmPreference None) sob
// Confirm Low sem -NonInteractive entrega menos bytes do payload ao guard que o payload
// enviado — o prompt de confirmação drena stdin antes do guard rodar.
//
// Reconciliação (REQ-2026-11-13 ML-2I):
//   - Braços form-L/via-command/{low,medium} × RC0/RC2: afirmam que a forma L entrega o payload
//     completo (len(stdinCapturado) == len(stdinPayload)) e sai com RC correto mesmo sem
//     -NonInteractive sob Confirm Low/Medium, confirmando que ml1dConfirmSeg previne o drain
//     de stdin.
//   - Braços form-L/via-profile/{low,medium} × RC0/RC2: idem via herança por perfil.
//   - Contra-braço form-K/confirm-low: afirma que a forma K sem ConfirmPreference None entrega
//     menos bytes que stdinPayload sob Confirm Low sem -NonInteractive — prova que o braço
//     morde e que o segmento adicionado por ML-2I é necessário.
func TestGuardLineShellMatrix_LContraArm_ConfirmLow(t *testing.T) {
	psBin, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh não encontrado — contra-braço ConfirmLow/Medium sem -NonInteractive não aplicável")
	}
	ensurePSWarmup(t, psBin)

	// Encontrar a constante git-branch para o braço principal.
	gitBranchLine := ""
	for _, g := range guardShellMatrix {
		if g.name == "git-branch" {
			gitBranchLine = g.line
			break
		}
	}
	if gitBranchLine == "" {
		t.Fatal("git-branch not found in guardShellMatrix")
	}

	// Deadline por chamada: usar o teto fixo para não depender do warmup
	// (estas chamadas podem ser mais lentas por causa do prompt drenado em K).
	const perCallTimeout = psCallDeadlineCeil

	// stdinPayloadNorm é o payload normalizado como o stub POSIX o grava (sem newline final).
	stdinPayloadNorm := strings.TrimRight(stdinPayload, "\r\n")

	// helper: executa gitBranchLine via PS sem -NonInteractive, com prefixo de estado,
	// e retorna (exitCode, stdinCaptured). Lê stdin.out de fakeDir após a execução.
	runInteractive := func(t *testing.T, line, fakeDir string, fakeRC int, extraEnv []string, addNoProfile bool) (exitCode int, captured string) {
		t.Helper()
		psConcurrencySem <- struct{}{}
		r := execPSCommandInteractive(psBin, line, fakeDir, fakeRC, perCallTimeout, addNoProfile, extraEnv)
		<-psConcurrencySem
		if r.timedOut {
			t.Skipf("timeout ao executar linha PS interativa — ambiente provavelmente sem PS real (pwsh mock ou CI não-interativo)")
		}
		if r.runErr != nil {
			t.Fatalf("erro de execução: %v", r.runErr)
		}
		exitCode = r.exitCode
		if fakeDir != "" {
			data, readErr := os.ReadFile(filepath.Join(fakeDir, "stdin.out"))
			if readErr == nil {
				captured = strings.ReplaceAll(string(data), "\r\n", "\n")
				captured = strings.TrimRight(captured, "\n")
			}
		}
		return
	}

	// Confirm levels to test for form L.
	confirmLevels := []struct {
		name  string
		level string
	}{
		{"low", "Low"},
		{"medium", "Medium"},
	}

	frcCases := []struct {
		name string
		frc  int
	}{
		{"RC0-allow", 0},
		{"RC2-deny", 2},
	}

	// ── Braços forma L via -Command ──────────────────────────────────────────────
	for _, cl := range confirmLevels {
		cl := cl
		t.Run("form-L/via-command/"+cl.name, func(t *testing.T) {
			t.Parallel()
			prefix := "$ConfirmPreference = '" + cl.level + "'; "
			line := prefix + gitBranchLine
			for _, fc := range frcCases {
				fc := fc
				t.Run(fc.name, func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					code, captured := runInteractive(t, line, dir, fc.frc, nil, true)
					// Payload delivery assertion: guard deve ter recebido o payload completo.
					if captured != stdinPayloadNorm {
						t.Errorf(
							"form-L confirm-%s %s: stdin não entregue completamente: got %d bytes (%q), want %d bytes (%q)",
							cl.name, fc.name, len(captured), captured, len(stdinPayloadNorm), stdinPayloadNorm,
						)
					}
					// Exit code assertion.
					if fc.frc == 0 && code != 0 {
						t.Errorf("form-L confirm-%s %s: esperado exit 0, obteve %d", cl.name, fc.name, code)
					}
					if fc.frc == 2 && code != 2 {
						t.Errorf("form-L confirm-%s %s: esperado exit 2, obteve %d", cl.name, fc.name, code)
					}
				})
			}
		})
	}

	// ── Braços forma L via perfil ───────────────────────────────────────────────────
	// No Windows: USERPROFILE+HOME → t.TempDir(); perfil em Documents\PowerShell\ ou
	//             Documents\WindowsPowerShell\. Sentinela verifica carregamento.
	// No macOS/Linux: XDG_CONFIG_HOME → t.TempDir(); perfil em powershell/.
	if runtime.GOOS == "windows" {
		type winPSEntry struct {
			name, bin, profileSubdir string
		}
		winLEntries := []winPSEntry{
			{"pwsh", "pwsh", filepath.Join("Documents", "PowerShell")},
			{"powershell", "powershell", filepath.Join("Documents", "WindowsPowerShell")},
		}
		for _, wps := range winLEntries {
			wps := wps
			t.Run("form-L/via-profile-windows/"+wps.name, func(t *testing.T) {
				winBin, lookErr := exec.LookPath(wps.bin)
				if lookErr != nil {
					t.Skipf("%s não encontrado em PATH (%v) — skip (b): probe de capacidade", wps.bin, lookErr)
				}
				for _, cl := range confirmLevels {
					cl := cl
					t.Run(cl.name, func(t *testing.T) {
						t.Parallel()
						userprofileDir := t.TempDir()
						profileDir := filepath.Join(userprofileDir, wps.profileSubdir)
						if mkErr := os.MkdirAll(profileDir, 0755); mkErr != nil {
							t.Fatalf("criando diretório de perfil Windows: %v", mkErr)
						}
						markerPath := filepath.Join(userprofileDir, "profile_loaded")
						profileContent := `New-Item -Path "` + markerPath + `" -ItemType File -Force | Out-Null` + "\n" +
							"$ConfirmPreference = '" + cl.level + "'\n"
						profileFile := filepath.Join(profileDir, "Microsoft.PowerShell_profile.ps1")
						if wErr := os.WriteFile(profileFile, []byte(profileContent), 0644); wErr != nil {
							t.Fatalf("escrevendo perfil Windows: %v", wErr)
						}
						upEnv := []string{
							"USERPROFILE=" + userprofileDir,
							"HOME=" + userprofileDir,
						}
						// Sentinel: capture resolved $PROFILE for concrete (c) proof if marker absent.
						resolvedProfileFile := filepath.Join(userprofileDir, "resolved_profile.txt")
						sentinelCmd := `[IO.File]::WriteAllText('` + resolvedProfileFile + `', $PROFILE); exit 0`
						psConcurrencySem <- struct{}{}
						checkR := execPSCommandInteractive(winBin, sentinelCmd, "", 0, perCallTimeout, false, upEnv)
						<-psConcurrencySem
						if checkR.timedOut || checkR.runErr != nil {
							t.Skipf("perfil via USERPROFILE não carregou: timedOut=%v, runErr=%v", checkR.timedOut, checkR.runErr)
						}
						if _, statErr := os.Stat(markerPath); os.IsNotExist(statErr) {
							resolvedProfile := "(unreadable)"
							if rp, rpErr := os.ReadFile(resolvedProfileFile); rpErr == nil {
								resolvedProfile = string(rp)
							}
							// (c): USERPROFILE override not honoured; marker absent is the measurement.
							t.Skipf("marcador de perfil ausente: (c) USERPROFILE=%s mas $PROFILE resolveu para %q em vez de %q — PS carrega perfil do USERPROFILE original do sistema",
								userprofileDir, resolvedProfile, profileFile)
						}

						// runInteractiveWith chama execPSCommandInteractive com winBin (não com
						// psBin capturado pelo closure externo). Necessário porque psBin é sempre
						// pwsh, independente do wps.bin iterado (pode ser powershell.exe).
						runInteractiveWith := func(t *testing.T, line, fakeDir string, fakeRC int, extraEnv []string) (exitCode int, captured string) {
							t.Helper()
							psConcurrencySem <- struct{}{}
							r := execPSCommandInteractive(winBin, line, fakeDir, fakeRC, perCallTimeout, false, extraEnv)
							<-psConcurrencySem
							if r.timedOut {
								t.Skipf("timeout ao executar linha PS interativa via %s — ambiente sem PS interativo", wps.bin)
							}
							if r.runErr != nil {
								t.Fatalf("erro de execução em %s: %v", wps.bin, r.runErr)
							}
							exitCode = r.exitCode
							if fakeDir != "" {
								data, readErr := os.ReadFile(filepath.Join(fakeDir, "stdin.out"))
								if readErr == nil {
									captured = strings.ReplaceAll(string(data), "\r\n", "\n")
									captured = strings.TrimRight(captured, "\n")
								}
							}
							return
						}

						// Form L under profile: RC0 and RC2 must pass with full stdin delivery.
						for _, fc := range frcCases {
							fc := fc
							t.Run("form-L/"+fc.name, func(t *testing.T) {
								t.Parallel()
								dir := t.TempDir()
								makeFakeTrackfwForOS(t, dir)
								pathVal := buildPSPath(dir)
								envWithPath := append([]string{"PATH=" + pathVal}, upEnv...)
								code, captured := runInteractiveWith(t, gitBranchLine, dir, fc.frc, envWithPath)
								if captured != stdinPayloadNorm {
									t.Errorf(
										"form-L confirm-%s via-profile-windows/%s %s: stdin não entregue completamente: got %d bytes (%q), want %d bytes (%q)",
										cl.name, wps.name, fc.name, len(captured), captured, len(stdinPayloadNorm), stdinPayloadNorm,
									)
								}
								if fc.frc == 0 && code != 0 {
									t.Errorf("form-L confirm-%s via-profile-windows/%s %s: esperado exit 0, obteve %d", cl.name, wps.name, fc.name, code)
								}
								if fc.frc == 2 && code != 2 {
									t.Errorf("form-L confirm-%s via-profile-windows/%s %s: esperado exit 2, obteve %d", cl.name, wps.name, fc.name, code)
								}
							})
						}

						// Counter-arm K under same USERPROFILE profile (addNoProfile=false, no prefix).
						// K = gitBranchLine sem ml1dConfirmSeg — ConfirmPreference vem só do perfil.
						// Deve drenar stdin / entregar menos bytes que stdinPayloadNorm.
						t.Run("form-K/via-profile-windows", func(t *testing.T) {
							t.Parallel()
							kFormGitBranch := strings.Replace(gitBranchLine, ml1dConfirmSeg, "", 1)
							dir := t.TempDir()
							makeFakeTrackfwForOS(t, dir)
							pathVal := buildPSPath(dir)
							envWithPath := append([]string{"PATH=" + pathVal}, upEnv...)
							// frc=2: K pode não alcançar o guard; assertamos apenas stdin drain.
							_, captured := runInteractiveWith(t, kFormGitBranch, dir, 2, envWithPath)
							// A forma K sob Confirm herdado do perfil DEVE entregar menos bytes.
							if captured == stdinPayloadNorm {
								t.Errorf(
									"counter-arm K/via-profile-windows confirm-%s/%s: forma K entregou o payload completo (%d bytes) sem ConfirmPreference None — braço não mordeu",
									cl.name, wps.name, len(captured),
								)
							}
						})
					})
				}
			})
		}
	} else {
		for _, cl := range confirmLevels {
			cl := cl
			t.Run("form-L/via-profile/"+cl.name, func(t *testing.T) {
				t.Parallel()
				xdgDir := t.TempDir()
				profileDir := filepath.Join(xdgDir, "powershell")
				if mkErr := os.MkdirAll(profileDir, 0755); mkErr != nil {
					t.Fatalf("criando diretório de perfil: %v", mkErr)
				}
				markerPath := filepath.Join(xdgDir, "profile_loaded")
				// Perfil: escreve marcador antes de definir o estado — o marcador usa New-Item
				// que tem ConfirmImpact=Low; com ConfirmPreference ainda não definido como Low,
				// não abre prompt. Depois define ConfirmPreference.
				profileContent := `New-Item -Path "` + markerPath + `" -ItemType File -Force | Out-Null` + "\n" +
					"$ConfirmPreference = '" + cl.level + "'\n"
				profileFile := filepath.Join(profileDir, "Microsoft.PowerShell_profile.ps1")
				if wErr := os.WriteFile(profileFile, []byte(profileContent), 0644); wErr != nil {
					t.Fatalf("escrevendo perfil: %v", wErr)
				}
				xdgEnv := []string{"XDG_CONFIG_HOME=" + xdgDir}

				// Verificar se o perfil carrega (marcador criado).
				psConcurrencySem <- struct{}{}
				checkR := execPSCommandInteractive(psBin, "exit 0", "", 0, perCallTimeout, false, xdgEnv)
				<-psConcurrencySem
				if checkR.timedOut || checkR.runErr != nil {
					t.Skipf("perfil via XDG não carregou: timedOut=%v, runErr=%v", checkR.timedOut, checkR.runErr)
				}
				if _, statErr := os.Stat(markerPath); os.IsNotExist(statErr) {
					t.Skipf("marcador de perfil ausente: perfil não carregou via XDG_CONFIG_HOME neste ambiente")
				}

				for _, fc := range frcCases {
					fc := fc
					t.Run(fc.name, func(t *testing.T) {
						t.Parallel()
						dir := t.TempDir()
						makeFakeTrackfwForOS(t, dir)
						pathVal := buildPSPath(dir)
						envWithPath := append([]string{"PATH=" + pathVal}, xdgEnv...)
						code, captured := runInteractive(t, gitBranchLine, dir, fc.frc, envWithPath, false)
						// Payload delivery assertion.
						if captured != stdinPayloadNorm {
							t.Errorf(
								"form-L confirm-%s via-profile %s: stdin não entregue completamente: got %d bytes (%q), want %d bytes (%q)",
								cl.name, fc.name, len(captured), captured, len(stdinPayloadNorm), stdinPayloadNorm,
							)
						}
						if fc.frc == 0 && code != 0 {
							t.Errorf("form-L confirm-%s via-profile %s: esperado exit 0, obteve %d", cl.name, fc.name, code)
						}
						if fc.frc == 2 && code != 2 {
							t.Errorf("form-L confirm-%s via-profile %s: esperado exit 2, obteve %d", cl.name, fc.name, code)
						}
					})
				}

				// Counter-arm K under same XDG profile (addNoProfile=false, no prefix).
				// Provides local evidence on macOS that K bites; validates Windows arm design.
				// NOT t.Parallel(): K may drain stdin slowly under Confirm Low; running it
				// sequentially (before the parallel frc sub-tests are released) avoids holding a
				// psConcurrencySem slot during the high-contention parallel phase.
				t.Run("form-K/via-profile", func(t *testing.T) {
					kFormGitBranch := strings.Replace(gitBranchLine, ml1dConfirmSeg, "", 1)
					dir := t.TempDir()
					makeFakeTrackfwForOS(t, dir)
					pathVal := buildPSPath(dir)
					envWithPath := append([]string{"PATH=" + pathVal}, xdgEnv...)
					// frc=2: K pode não alcançar o guard; assertamos apenas stdin drain.
					_, captured := runInteractive(t, kFormGitBranch, dir, 2, envWithPath, false)
					// A forma K sob Confirm herdado do perfil DEVE entregar menos bytes.
					if captured == stdinPayloadNorm {
						t.Errorf(
							"counter-arm K/via-profile confirm-%s: forma K entregou o payload completo (%d bytes) sem ConfirmPreference None — braço não mordeu",
							cl.name, len(captured),
						)
					} else {
						t.Logf("counter-arm K/via-profile confirm-%s: forma K entregou %d bytes (payload=%d) — braço morde", cl.name, len(captured), len(stdinPayloadNorm))
					}
				})
			})
		}
	}

	// ── Contra-braço: forma K sob Confirm Low sem -NonInteractive ────────────────────────
	// A forma K é construída removendo ml1dConfirmSeg de gitBranchLine em tempo de execução.
	// Nenhuma ocorrência do literal "...Confirm:0; set +eu..." aparece no código-fonte
	// (os dois segmentos estão separados), de modo que o grep de validação continua vazio.
	t.Run("counter-arm/form-K/confirm-low", func(t *testing.T) {
		// Verificar que ml1dConfirmSeg está em gitBranchLine; sem isso a remoção seria silenciosa.
		if !strings.Contains(gitBranchLine, ml1dConfirmSeg) {
			t.Fatalf("gitBranchLine não contém ml1dConfirmSeg %q — forma L foi alterada?", ml1dConfirmSeg)
		}
		// Remover ml1dConfirmSeg de gitBranchLine → forma K (nunca released após ML-2I).
		kFormGitBranch := strings.Replace(gitBranchLine, ml1dConfirmSeg, "", 1)
		if strings.Contains(kFormGitBranch, ml1dConfirmSeg) {
			t.Fatalf("kFormGitBranch ainda contém ml1dConfirmSeg — múltiplas ocorrências?")
		}

		prefix := "$ConfirmPreference = 'Low'; "
		line := prefix + kFormGitBranch
		dir := t.TempDir()
		makeFakeTrackfwForOS(t, dir)
		// Executar diretamente sem psGuardedCall: se K travar, não queremos registrar pwsh
		// no psTimeouts e fail-fast todos os outros testes PS do pacote.
		psConcurrencySem <- struct{}{}
		r := execPSCommandInteractive(psBin, line, dir, 2, perCallTimeout, true, nil)
		<-psConcurrencySem
		if r.timedOut {
			t.Skipf("counter-arm K/confirm-low: timeout — ambiente pode não ter PS interativo; pulando")
		}
		if r.runErr != nil {
			t.Fatalf("counter-arm K/confirm-low: erro de execução: %v", r.runErr)
		}

		var captured string
		if data, readErr := os.ReadFile(filepath.Join(dir, "stdin.out")); readErr == nil {
			captured = strings.ReplaceAll(string(data), "\r\n", "\n")
			captured = strings.TrimRight(captured, "\n")
		}

		// A forma K sob Confirm Low sem -NonInteractive DEVE entregar menos bytes que stdinPayload
		// ao guard (ou o guard não roda — stdin.out ausente → captured == "").
		// Se captured == stdinPayloadNorm, o braço não mordeu — o segmento em ML-2I seria inútil.
		if captured == stdinPayloadNorm {
			t.Errorf(
				"counter-arm K/confirm-low: forma K entregou o payload completo (%d bytes) sem -NonInteractive "+
					"sob Confirm Low — braço não mordeu; ml1dConfirmSeg em ML-2I pode ser desnecessário",
				len(captured),
			)
		} else {
			t.Logf(
				"counter-arm K/confirm-low: forma K entregou %d bytes (payload=%d, stdinPayload=%d) — braço morde, ML-2I justificado",
				len(captured), len(stdinPayloadNorm), len(stdinPayload),
			)
		}
	})
}

// TestD18ContraArm_ShAsDashSim simula o ambiente Linux onde /bin/sh é um symlink para dash.
// Cria um "sh" como symlink para o dash local (em t.TempDir) e executa as formas legadas D18
// com flags -u e -e, verificando:
//   - sh-as-dash-u: exit == 2 (dash nounset) + guard não rodou (stdin.out ausente).
//   - sh-as-dash-e: exit ≠ 2 (127) + guard não rodou (stdin.out ausente).
//
// Este teste prova que a lógica d18ShellKind + seleção de expectativa do
// TestGuardLineShellMatrix_D18ContraArm funciona no caso Linux, onde o braço sh-u falhou
// com "forma legada saiu com exit 2 — esperado exit ≠ 2" (REQ-2026-11-13 ML-2E).
//
// Reconciliação (REQ-2026-11-13 ML-2E):
//   - sh-as-dash-u afirma: "dash -u aborta com exit 2 antes do guard rodar" (medição do arquiteto).
//   - sh-as-dash-e afirma: "dash -e não causa nounset; sai com ≠ 2 por emulate/comando ausente".
//   - Ambos afirmam: "guard não rodou" (stdin.out ausente — prova invariante).
//
// GOOS skip removido; dash LookPath abaixo skip por probe.
// Classificação (b) — dash disponível via Git for Windows no windows-latest
// (confirmado: job 133335668133, TestGuardLineShellMatrix_D18ContraArm/*/dash-u PASS).
// No Windows o link recebe extensão .exe para que CreateProcess possa executá-lo.
// buildShellPath usa dashPath (não shLink) para incluir o diretório das DLLs do dash.
func TestD18ContraArm_ShAsDashSim(t *testing.T) {
	dashPath, err := exec.LookPath("dash")
	if err != nil {
		t.Skipf("dash não encontrado em PATH (%v) — simulação sh→dash não aplicável; skip (b): probe de capacidade", err)
	}

	// Criar symlink sh[.exe] → dash simulando /bin/sh→/bin/dash (Linux).
	// No Windows: o link precisa de extensão .exe para que CreateProcess o execute.
	// d18ShellKind(shLink) chama EvalSymlinks, que resolve sh.exe→dash.exe → "dash" ✓.
	linkDir := t.TempDir()
	shLinkName := "sh"
	if runtime.GOOS == "windows" {
		shLinkName = "sh.exe"
	}
	shLink := filepath.Join(linkDir, shLinkName)
	symlinkOrSkip(t, dashPath, shLink)

	// d18ShellKind deve classificar shLink como "dash" — sem isso o resto do teste não tem sentido.
	// Mesmo shellPath passado para d18ShellKind e exec.CommandContext: sem divergência.
	kind := d18ShellKind(shLink)
	if kind != "dash" {
		t.Fatalf("d18ShellKind(%q) = %q; esperado \"dash\" — symlink não resolveu para dash", shLink, kind)
	}

	type simCase struct {
		name      string
		flags     []string
		wantExit2 bool
	}
	cases := []simCase{
		// sh-as-dash-u: d18ShellKind="dash" && flag="-u" → wantExit2=true.
		// Afirma: "dash -u aborta com exit 2 (nounset em $LASTEXITCODE) antes do guard".
		{"sh-as-dash-u", []string{"-u"}, true},
		// sh-as-dash-e: d18ShellKind="dash" && flag="-e" → wantExit2=false.
		// Afirma: "dash -e aborta com exit 127 (emulate não encontrado) antes do guard".
		{"sh-as-dash-e", []string{"-e"}, false},
	}

	d18Matrix := []struct {
		name    string
		legLine string
	}{
		{"git-branch", guardforms.LegacyD18GitBranchPSPOSIX},
		{"credential", guardforms.LegacyD18CredentialPSPOSIX},
		{"write-scope", guardforms.LegacyD18WriteScopePSPOSIX},
		{"credential-global", guardforms.LegacyD18CredentialGlobalPSPOSIX},
	}

	for _, sc := range cases {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			for _, guard := range d18Matrix {
				guard := guard
				t.Run(guard.name, func(t *testing.T) {
					fakeDir := t.TempDir()
					makeFakeTrackfw(t, fakeDir)

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					args := append(sc.flags, "-c", guard.legLine)
					cmd := exec.CommandContext(ctx, shLink, args...)

					// buildShellPath usa dashPath (não shLink) para incluir o diretório do
					// executável real do dash (e.g. Git/usr/bin no Windows), onde vivem as
					// DLLs do MSYS. Usar shLink poria apenas o TempDir no PATH, e
					// msys-2.0.dll não seria encontrada (STATUS_DLL_NOT_FOUND 0xC0000135).
					pathVal := buildShellPath(dashPath, fakeDir)
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
					var stderrBuf bytes.Buffer
					cmd.Stderr = &stderrBuf

					runErr := cmd.Run()
					code := 0
					if runErr != nil {
						if exitErr, ok := runErr.(*exec.ExitError); ok {
							code = exitErr.ExitCode()
						} else {
							t.Fatalf("sim(%s/%s): cmd.Run() falhou com erro que não é *exec.ExitError: %v", sc.name, guard.name, runErr)
						}
					}
					// Registrar rc e stderr para diagnóstico no CI: mostra o que o dash do
					// Git for Windows produz com -u e -e (só confirmável no windows-latest).
					t.Logf("sim(%s/%s): rc=%d stderr=%q", sc.name, guard.name, code, stderrBuf.String())
					// Código fora do intervalo 0–255 indica falha do loader (NTSTATUS),
					// não exit code real do shell. O guard nunca rodou nesse caso.
					if code < 0 || code > 255 {
						t.Fatalf("sim(%s/%s): exit %d fora do intervalo 0–255 — loader/NTSTATUS error (shell nunca rodou)", sc.name, guard.name, code)
					}

					// stdin.out ausente: prova invariante — guard nunca rodou.
					// Para sh-as-dash-u: distingue "shell abortou antes do guard" (exit 2, sem stdin.out)
					// de "guard rodou e negou" (exit 2, com stdin.out), pois FAKE_RC=2 coincide.
					stdinOutPath := filepath.Join(fakeDir, "stdin.out")
					if _, statErr := os.Stat(stdinOutPath); statErr == nil {
						t.Errorf("sim(%s/%s): stdin.out existe — guard rodou quando não deveria", sc.name, guard.name)
					}

					if sc.wantExit2 {
						if code != 2 {
							t.Errorf(
								"sim(%s/%s): exit %d — esperado 2 (dash -u aborta com 2 em variável indefinida)",
								sc.name, guard.name, code,
							)
						} else {
							t.Logf("sim(%s/%s): exit 2 — fail-closed dash-u confirmado", sc.name, guard.name)
						}
					} else {
						if code == 2 {
							t.Errorf(
								"sim(%s/%s): exit 2 — esperado ≠ 2 (defeito D18 não detectado nesta combinação)",
								sc.name, guard.name,
							)
						} else {
							t.Logf("sim(%s/%s): exit %d (≠ 2) — defeito D18 confirmado", sc.name, guard.name, code)
						}
					}
				})
			}
		})
	}
}
