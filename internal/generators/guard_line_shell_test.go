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
)

// stdinPayload é o JSON que Claude Code entregaria ao hook de PreToolUse.
const stdinPayload = `{"tool_name":"Bash","tool_input":{"command":"git add -A"}}`

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
)

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

// psTimeoutRegistry guarda, por binário, o nome do primeiro teste que estourou o prazo.
// Usada pelo mecanismo fail-fast por binário (AC3, emenda D3).
type psTimeoutRegistry struct {
	mu sync.Mutex
	m  map[string]string // psBin → nome do teste que estourou
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

// psTimeouts é o registro global de binários PS que estouraram o prazo nesta execução.
var psTimeouts = &psTimeoutRegistry{m: map[string]string{}}

// psGuardedCall executa run(deadline) para psBin, aplicando fail-fast e registro de estouro.
// Retorna (result, fatalMsg); se fatalMsg != "" o chamador deve invocar t.Fatalf(fatalMsg).
// O prazo é derivado de warmup via psCallDeadline.
//
// Fail-fast por binário (AC3, emenda D3): se psBin já estourou o prazo nesta execução,
// run NÃO é chamado e fatalMsg cita o teste que causou o primeiro estouro.
// Isso limita o pior caso a um único timeout por binário (~60 s), mantendo a execução
// total dentro do -timeout 20m do CI.
func psGuardedCall(
	reg *psTimeoutRegistry,
	psBin, testName string,
	warmup time.Duration,
	run func(time.Duration) psExecResult,
) (psExecResult, string) {
	// Fail-fast: binário já estourou o prazo — não executar.
	if culprit, timed := reg.query(psBin); timed {
		return psExecResult{}, fmt.Sprintf("não executado: %s já estourou o prazo em %s", psBin, culprit)
	}

	deadline := psCallDeadline(warmup)
	r := run(deadline)

	if r.timedOut {
		reg.register(psBin, testName)
		return r, fmt.Sprintf(
			"timeout: %s após %.0fs — prazo derivado do aquecimento (%.2fs × %d, limitado a [%.0fs, %.0fs]) esgotado",
			filepath.Base(psBin), deadline.Seconds(), warmup.Seconds(), psCallDeadlineFactor,
			psCallDeadlineFloor.Seconds(), psCallDeadlineCeil.Seconds(),
		)
	}
	return r, ""
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
var shellEntries = []struct {
	name  string
	bin   string
	flags []string
}{
	{"sh", "sh", nil},
	{"bash", "bash", nil},
	{"zsh", "zsh", nil},
	{"zsh-f", "zsh", []string{"-f"}},
	{"dash", "dash", nil},
	{"zsh-errexit", "zsh", []string{"-o", "errexit"}},
	{"zsh-nounset", "zsh", []string{"-o", "nounset"}},
	{"zsh-cshnullglob", "zsh", []string{"-o", "cshnullglob"}},
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
func runGuardLine(
	t *testing.T,
	shellBin string,
	shellFlags []string,
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
	base := os.Environ()
	filtered := make([]string, 0, len(base)+2)
	for _, e := range base {
		if strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "FAKE_RC=") {
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
				return d, fmt.Errorf("timeout após %.1fs", d.Seconds())
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

// ensurePSWarmup aquece psBin uma vez por processo e registra o resultado com t.Logf.
// Chama t.Fatalf com mensagem própria se o aquecimento falhar (AC1).
// Todo braço PS chama ensurePSWarmup logo após LookPath (P-2, P-8).
func ensurePSWarmup(t *testing.T, psBin string) {
	t.Helper()
	dur, err, fresh := psWarmup(psBin)
	if err != nil {
		t.Fatalf("aquecimento de %s falhou: %v (%.2fs)", psBin, err, dur.Seconds())
	}
	if fresh {
		t.Logf("aquecimento de %s: %.2fs", psBin, dur.Seconds())
	} else {
		t.Logf("aquecimento de %s: resultado em cache (%.2fs)", psBin, dur.Seconds())
	}
}

// runGuardLinePS executa a linha de guard via PowerShell com
// -NoProfile -NonInteractive -Command <linha>.
// Retorna (exitCode, stderr, stdinCaptured).
//
// Delega a execução para psGuardedCall, que aplica fail-fast por binário (AC3, emenda D3)
// e deriva o prazo via psCallDeadline(aquecimento) (AC3, D3).
//   - Se timedOut: registra o binário no psTimeouts e t.Fatalf com binário, aquecimento e prazo.
//   - Se o binário já estourou: t.Fatalf "não executado", sem chamar execPSCommand.
//   - Se runErr: t.Fatalf com o erro (ex: ErrWaitDelay de grandchild).
//
// O aquecimento é obtido do cache de psWarmup (não executa de novo).
// Se o aquecimento falhou, não inventa prazo: t.Fatalf antes de derivar deadline.
//
// PATH construído via buildPSPath:
//   - Windows: stub-dir + %SystemRoot%\System32 (necessário para cmd.exe/powershell.exe).
//   - Outros SO: stub-dir + /usr/bin:/bin (mesmo padrão do runGuardLine POSIX).
//
// stdinCaptured tem CRLF normalizado e trailing \n removido — torna a comparação portável
// entre "cat > file" (POSIX, sem trailing newline) e "more > file" (Windows, pode adicionar).
func runGuardLinePS(
	t *testing.T,
	psBin string,
	line string,
	fakeDir string, // diretório com o stub; "" = trackfw ausente do PATH
	fakeRC int,
) (exitCode int, stderr string, stdinCaptured string) {
	t.Helper()

	// Obter aquecimento do cache; ensurePSWarmup já reprovou se falhou.
	// Verificação defensiva: não derivar prazo de aquecimento fracassado.
	warmup, warmupErr, _ := psWarmup(psBin)
	if warmupErr != nil {
		t.Fatalf("aquecimento de %s falhou; prazo não pode ser derivado: %v", psBin, warmupErr)
	}

	r, fatal := psGuardedCall(psTimeouts, psBin, t.Name(), warmup, func(d time.Duration) psExecResult {
		return execPSCommand(psBin, line, fakeDir, fakeRC, d)
	})
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

					// Caso 1: FAKE_RC=0 (allow) → exit 0, stdin preservado.
					t.Run("RC0-allow", func(t *testing.T) {
						dir := t.TempDir()
						makeFakeTrackfw(t, dir)
						code, _, captured := runGuardLine(t, shellPath, sh.flags, guard.line, dir, 0)
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
						code, stderr, captured := runGuardLine(t, shellPath, sh.flags, guard.line, dir, 2)
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
						code, _, _ := runGuardLine(t, shellPath, sh.flags, guard.line, dir, 1)
						if code != 2 {
							t.Errorf("FAKE_RC=1: esperado exit 2 (normalizado), obteve %d", code)
						}
					})

					// Caso 4: trackfw ausente do PATH → fail-closed, exit 2.
					t.Run("absent", func(t *testing.T) {
						code, _, _ := runGuardLine(t, shellPath, sh.flags, guard.line, "", 0)
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
					// ensurePSWarmup usa sync.OnceValues: o warmup roda uma única vez;
					// arms subsequentes verificam o resultado em cache e emitem log próprio (P-8).
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

// TestGuardLineShellMatrix_DefeitoAntigo demonstra que a forma ANTIGA (sem o prefixo emulate -R sh)
// falha sob zsh com FAKE_RC=2 — saindo com exit ≠ 2.
// Esse braço é a "prova de mordida": se o teste passar com a forma antiga, ele não distingue
// defeito de correção e não tem valor como regressão.
//
// Reconciliação (REQ-2026-10-20): afirma que a forma D11 revised sem emulate -R sh sai ≠ 2
// sob zsh, confirmando que o prefixo emulate é necessário.
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
			// Confirmar que o prefixo emulate existe nas constantes (falha de compilação mascararia o defeito).
			if !strings.HasPrefix(guard.line, emulatePrefix) {
				t.Fatalf("constante %q não começa com o prefixo emulate esperado %q — a correção foi removida?",
					guard.name, emulatePrefix)
			}

			// Forma antiga: remover o prefixo emulate da linha gerada (D11 revised, sem nenhum prefixo).
			oldLine := strings.TrimPrefix(guard.line, emulatePrefix)

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
				if strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "FAKE_RC=") {
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
					"braço do defeito (%s): forma antiga saiu com exit 2 sob zsh — "+
						"o prefixo emulate pode não ser necessário para este guard ou o zsh está configurado de forma não-padrão; "+
						"verificar ambiente zsh",
					guard.name,
				)
			} else {
				t.Logf("braço do defeito (%s): forma antiga saiu com exit %d (esperado ≠ 2) — defeito confirmado", guard.name, code)
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
			// Verificar que a constante começa com emulate (não unsetopt) — se começar com
			// unsetopt, a regressão ficou sem cobertura.
			if !strings.HasPrefix(guard.line, emulatePrefix) {
				t.Fatalf("constante %q não começa com emulatePrefix — forma errada aplicada",
					guard.name)
			}

			// Construir a forma unsetopt: unsetoptOldPrefix + corpo sem o emulate.
			body := strings.TrimPrefix(guard.line, emulatePrefix)
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
				if strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "FAKE_RC=") {
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
// fail-fast por binário: após o primeiro estouro, chamadas subsequentes para o mesmo binário
// não invocam run; para binário diferente, run é invocado normalmente (REQ-2026-11-06, AC3, emenda D3).
//
// Usa um registro isolado (não o psTimeouts global) e uma função run fake, sem PowerShell instalado.
//
// Reconciliação — AC3 (emenda D3): afirma que o fail-fast impede execuções desnecessárias após
// o primeiro estouro, limitando o pior caso a ~60 s por binário dentro do -timeout 20m do CI.
func TestRunGuardLinePS_FailFastPorBinario(t *testing.T) {
	reg := &psTimeoutRegistry{m: map[string]string{}}
	fakeBinA := "/nonexistent/fake-ps-a-" + t.Name()
	fakeBinB := "/nonexistent/fake-ps-b-" + t.Name()

	var callCount int

	// Chamada 1: bin A com aquecimento de 8 s → prazo derivado 32 s, estoura.
	callCount = 0
	_, msg1 := psGuardedCall(reg, fakeBinA, "TestX/case1", 8*time.Second, func(d time.Duration) psExecResult {
		callCount++
		if d != 32*time.Second {
			t.Errorf("chamada 1: prazo esperado 32s, obteve %v", d)
		}
		return psExecResult{timedOut: true}
	})
	if callCount != 1 {
		t.Errorf("chamada 1: run deve ser chamada uma vez; chamada %d vezes", callCount)
	}
	if !strings.Contains(msg1, "timeout") {
		t.Errorf("chamada 1: msg deve conter 'timeout'; got %q", msg1)
	}
	if !strings.Contains(msg1, "32s") {
		t.Errorf("chamada 1: msg deve citar prazo '32s'; got %q", msg1)
	}

	// Chamada 2: mesmo bin A — fail-fast, run NÃO deve ser chamada.
	// Afirma: após estouro, execução é bloqueada e a mensagem cita o culpado original.
	callCount = 0
	_, msg2 := psGuardedCall(reg, fakeBinA, "TestX/case2", 8*time.Second, func(d time.Duration) psExecResult {
		callCount++
		return psExecResult{}
	})
	if callCount != 0 {
		t.Errorf("chamada 2 (fail-fast): run não deve ser chamada; chamada %d vezes", callCount)
	}
	if !strings.Contains(msg2, "não executado") {
		t.Errorf("chamada 2: msg deve conter 'não executado'; got %q", msg2)
	}
	if !strings.Contains(msg2, "TestX/case1") {
		t.Errorf("chamada 2: msg deve citar culpado 'TestX/case1'; got %q", msg2)
	}

	// Chamada 3: bin B diferente — fail-fast NÃO se aplica; run deve ser chamada.
	// Afirma: o fail-fast é por binário, não global.
	callCount = 0
	_, msg3 := psGuardedCall(reg, fakeBinB, "TestX/case3", 4*time.Second, func(d time.Duration) psExecResult {
		callCount++
		if d != 16*time.Second {
			t.Errorf("chamada 3: prazo esperado 16s, obteve %v", d)
		}
		return psExecResult{exitCode: 0}
	})
	if callCount != 1 {
		t.Errorf("chamada 3 (binário diferente): run deve ser chamada uma vez; chamada %d vezes", callCount)
	}
	if msg3 != "" {
		t.Errorf("chamada 3: mensagem fatal inesperada: %q", msg3)
	}
}
