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
var guardShellMatrix = []struct {
	name string
	line string
}{
	{"git-branch", guardGitBranchCmdPSPOSIX},
	{"credential", guardCredentialCmdPSPOSIX},
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

// TestGuardLineShellMatrix é o teste principal exigido pelo AC da REQ-2026-10-20.
// Executa o produto cartesiano (3 linhas de guard) × (4 shells) × (4 cenários de RC/ausência).
func TestGuardLineShellMatrix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shells POSIX não disponíveis em Windows — matriz Windows medida separadamente na VM")
	}

	for _, guard := range guardShellMatrix {
		guard := guard
		t.Run(guard.name, func(t *testing.T) {
			for _, sh := range shellEntries {
				sh := sh
				t.Run(sh.name, func(t *testing.T) {
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
