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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// runGuardLinePS executa a linha de guard via PowerShell com
// -NoProfile -NonInteractive -Command <linha>.
// Retorna (exitCode, stderr, stdinCaptured).
//
// PATH construído via buildPSPath:
//   - Windows: stub-dir + %SystemRoot%\System32 (necessário para cmd.exe/powershell.exe).
//   - Outros SO: stub-dir + /usr/bin:/bin (mesmo padrão do runGuardLine POSIX).
//
// Env: herda o processo pai, substituindo PATH (case-insensitive no Windows) e FAKE_RC.
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := []string{"-NoProfile", "-NonInteractive", "-Command", line}
	cmd := exec.CommandContext(ctx, psBin, args...)

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
			// Normalizar CRLF: trackfw.cmd no Windows pode adicionar \r\n via "more";
			// o stub POSIX não adiciona. TrimRight remove trailing \n residual.
			captured = strings.ReplaceAll(string(data), "\r\n", "\n")
			captured = strings.TrimRight(captured, "\n")
		}
	}

	return code, errBuf.String(), captured
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
