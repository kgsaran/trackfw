// Package generators — ML-2B: execute guard cmd.exe lines under real cmd.exe /d /c.
//
// Reconciliação por teste (Regra Dura de Reconciliação, REQ-2026-11-15 ML-2B):
//
//   TestGuardCmdExe_Deny
//     Afirma que a linha cmd.exe propaga rc=2 do guard e entrega bytes de stdin íntegros
//     quando trackfw nega (fake rc=2) sob cmd.exe real.
//
//   TestGuardCmdExe_Allow
//     Afirma que a linha cmd.exe propaga rc=0 do guard e entrega bytes de stdin íntegros
//     quando trackfw permite (fake rc=0) sob cmd.exe real.
//
//   TestGuardCmdExe_Absent
//     Afirma que `|| exit 2` garante rc=2 (fail-closed per contrato — critério de deny de
//     Kiro e Amazon Q; vault: guard-fallback-old-binary-exit1-fail-open-6-clis-2026-10-06.md)
//     quando trackfw não está no PATH.
//
//   TestGuardCmdExe_ContraArm
//     Afirma que a forma sem `|| exit 2` NÃO sai com rc=2 quando trackfw está ausente,
//     provando que o sufixo é load-bearing e que a forma real reprova o contra-braço.
//     O rc efectivo da forma podada é registado em t.Logf para evidência no CI.
//
// Fora do Windows: cada teste emite t.Skip("cmd.exe existe apenas no Windows").
// O que só o CI de Windows confirma: rc efectivo nos 4 casos acima; rc podado (provável 9009
// — ERRORLEVEL do "not recognized" cmd.exe) no contra-braço; stdin íntegro via "more".
package generators

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// cmdExeStdinPayload é o payload enviado como stdin para o stub trackfw.cmd.
// Declarado neste ficheiro para que a compilação não dependa de guard_line_shell_test.go,
// cujo estado é gerido pelo ML-2A em paralelo.
const cmdExeStdinPayload = `{"tool_name":"Bash","tool_input":{"command":"git add -A"}}`

// cmdExeSuffix é o sufixo fail-closed de todas as formas cmd.exe.
// Assert de sufixo em TestGuardCmdExe_ContraArm verifica que cada constante ainda termina com ele.
const cmdExeSuffix = " || exit 2"

// cmdExeGuardForms lista as três formas cmd.exe a exercitar, usando os aliases do pacote
// (agentfiles.go:518-520) que referenciam as constantes canônicas em internal/guardforms.
var cmdExeGuardForms = []struct {
	name string
	line string
}{
	{"credential", guardCredentialCmdCmdExe},
	{"git-branch", guardGitBranchCmdCmdExe},
	{"credential-global", guardCredentialGlobalCmdCmdExe},
}

// makeCmdExeStub cria um stub "trackfw.cmd" em dir.
//   - captura stdin com "more" (lê stdin até EOF) e escreve em stdin.out no mesmo dir
//   - sai com FAKE_RC (default 0 se não definido no env)
//   - emite "motivo-deny-fake" em stderr para diagnóstico
//
// Preferido sobre trackfw.ps1 porque: (1) processo nativo via cmd.exe — ERRORLEVEL e stdin
// fluem correctamente; (2) não sofre restrição de execution policy.
func makeCmdExeStub(t *testing.T, dir string) {
	t.Helper()
	stub := filepath.Join(dir, "trackfw.cmd")
	content := "@echo off\r\n" +
		"if \"%FAKE_RC%\"==\"\" set FAKE_RC=0\r\n" +
		"more > \"%~dp0stdin.out\"\r\n" +
		"echo motivo-deny-fake>&2\r\n" +
		"exit /b %FAKE_RC%\r\n"
	if err := os.WriteFile(stub, []byte(content), 0644); err != nil {
		t.Fatalf("makeCmdExeStub: escrever trackfw.cmd: %v", err)
	}
}

// locateCmdExe devolve o caminho absoluto de cmd.exe.
// Consulta ComSpec primeiro; recorre a %SystemRoot%\System32\cmd.exe.
// Chama t.Fatalf se não encontrar — no Windows, ausência de cmd.exe é condição fatal, não skip.
func locateCmdExe(t *testing.T) string {
	t.Helper()
	if cs := os.Getenv("ComSpec"); cs != "" {
		if _, err := os.Stat(cs); err == nil {
			return cs
		}
	}
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	p := filepath.Join(sysRoot, "System32", "cmd.exe")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("locateCmdExe: cmd.exe não encontrado em %s (ComSpec vazio)", p)
	}
	return p
}

// buildCmdExePath monta o PATH para execução via cmd.exe.
// Inclui sempre System32 (necessário para comandos internos); antepõe fakeDir quando não vazio.
// Um PATH explícito impede que cmd.exe pesquise no directório corrente antes do PATH.
func buildCmdExePath(fakeDir string) string {
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

// runCmdExeLine executa cmdExe /d /c <line> e devolve (exitCode, stdinCaptured).
//   - fakeDir: directório com trackfw.cmd; "" significa trackfw ausente do PATH.
//   - fakeRC: valor de FAKE_RC enviado ao stub.
//   - stubMustRun: se true, t.Fatalf quando stdin.out não for criado (stub não executou).
//
// Erros que não sejam *exec.ExitError causam t.Fatalf imediato (não são engolidos).
// cmd.Dir é definido como um TempDir vazio para prevenir que cmd.exe resolva trackfw
// a partir do directório corrente antes de pesquisar o PATH.
func runCmdExeLine(t *testing.T, cmdExe, line, fakeDir string, fakeRC int, stubMustRun bool) (int, string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdExe, "/d", "/c", line)
	cmd.Dir = t.TempDir() // directório corrente vazio; impede resolução local de trackfw

	// Montar env: herdar do processo pai, substituindo PATH e FAKE_RC.
	// No Windows PATH pode aparecer como "Path" — filtrar pelo prefixo em maiúsculas.
	pathVal := buildCmdExePath(fakeDir)
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

	cmd.Stdin = strings.NewReader(cmdExeStdinPayload)

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("runCmdExeLine: erro inesperado (não ExitError): %v", runErr)
		}
	}

	var captured string
	if fakeDir != "" {
		stdinOutPath := filepath.Join(fakeDir, "stdin.out")
		data, readErr := os.ReadFile(stdinOutPath)
		if readErr != nil {
			if stubMustRun {
				t.Fatalf("runCmdExeLine: stub não executou (stdin.out ausente em %s): %v",
					stdinOutPath, readErr)
			}
		} else {
			// Normalizar CRLF adicionados por "more"; remover newline final residual.
			captured = strings.ReplaceAll(string(data), "\r\n", "\n")
			captured = strings.TrimRight(captured, "\n")
		}
	}

	return exitCode, captured
}

// TestGuardCmdExe_Deny afirma que a linha cmd.exe propaga rc=2 e entrega bytes de stdin
// íntegros quando trackfw nega (fake rc=2) sob cmd.exe real.
// Reconciliação ML-2B: rc=2 + bytes preservados prova que o stub correu sob cmd.exe real
// (distingue "stub negou" de "|| exit 2 disparou por ausência de trackfw").
func TestGuardCmdExe_Deny(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd.exe existe apenas no Windows")
	}
	cmdExe := locateCmdExe(t)
	want := strings.TrimRight(cmdExeStdinPayload, "\r\n")

	for _, g := range cmdExeGuardForms {
		g := g
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			makeCmdExeStub(t, dir)
			rc, captured := runCmdExeLine(t, cmdExe, g.line, dir, 2, true)
			if rc != 2 {
				t.Errorf("deny/%s: want rc=2, got %d", g.name, rc)
			}
			if len(captured) != len(want) {
				t.Errorf("deny/%s: stdin byte count: got %d bytes, want %d bytes",
					g.name, len(captured), len(want))
			}
			if captured != want {
				t.Errorf("deny/%s: stdin mismatch: got %q, want %q", g.name, captured, want)
			}
		})
	}
}

// TestGuardCmdExe_Allow afirma que a linha cmd.exe propaga rc=0 e entrega bytes de stdin
// íntegros quando trackfw permite (fake rc=0) sob cmd.exe real.
// Reconciliação ML-2B: rc=0 + bytes preservados prova propagação de stdin sem drenagem.
func TestGuardCmdExe_Allow(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd.exe existe apenas no Windows")
	}
	cmdExe := locateCmdExe(t)
	want := strings.TrimRight(cmdExeStdinPayload, "\r\n")

	for _, g := range cmdExeGuardForms {
		g := g
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			makeCmdExeStub(t, dir)
			rc, captured := runCmdExeLine(t, cmdExe, g.line, dir, 0, true)
			if rc != 0 {
				t.Errorf("allow/%s: want rc=0, got %d", g.name, rc)
			}
			if len(captured) != len(want) {
				t.Errorf("allow/%s: stdin byte count: got %d bytes, want %d bytes",
					g.name, len(captured), len(want))
			}
			if captured != want {
				t.Errorf("allow/%s: stdin mismatch: got %q, want %q", g.name, captured, want)
			}
		})
	}
}

// TestGuardCmdExe_Absent afirma que `|| exit 2` garante rc=2 (fail-closed per contrato —
// critério de deny de Kiro e Amazon Q) quando trackfw não está no PATH.
// Reconciliação ML-2B: o sufixo `|| exit 2` é o único mecanismo que garante rc=2 neste caso;
// o contra-braço em TestGuardCmdExe_ContraArm prova que sem ele o rc não é 2.
func TestGuardCmdExe_Absent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd.exe existe apenas no Windows")
	}
	cmdExe := locateCmdExe(t)

	for _, g := range cmdExeGuardForms {
		g := g
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()
			// fakeDir="" → trackfw ausente do PATH; stubMustRun=false (stub não corre).
			rc, _ := runCmdExeLine(t, cmdExe, g.line, "", 0, false)
			if rc != 2 {
				t.Errorf("absent/%s: want rc=2 (fail-closed per contrato), got %d", g.name, rc)
			}
		})
	}
}

// TestGuardCmdExe_ContraArm afirma que a forma sem `|| exit 2` NÃO sai com rc=2 quando
// trackfw está ausente do PATH, provando que o sufixo é load-bearing e que a forma real
// reprova o contra-braço.
// Reconciliação ML-2B: o assert de sufixo garante que cada constante ainda carrega ` || exit 2`;
// o rc efectivo da forma podada (provável 9009 no cmd.exe — ERRORLEVEL de "not recognized") é
// registado em t.Logf para evidência permanente no CI.
func TestGuardCmdExe_ContraArm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd.exe existe apenas no Windows")
	}
	cmdExe := locateCmdExe(t)

	for _, g := range cmdExeGuardForms {
		g := g
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()

			// Assert de sufixo: cada constante deve terminar com cmdExeSuffix.
			// Falha de compilação-time se o sufixo for alterado sem actualizar este teste.
			if !strings.HasSuffix(g.line, cmdExeSuffix) {
				t.Fatalf("contra-arm/%s: constante não termina com %q — "+
					"constante alterada sem actualizar guard_cmdexe_exec_test.go",
					g.name, cmdExeSuffix)
			}

			stripped := strings.TrimSuffix(g.line, cmdExeSuffix)

			// Forma podada com trackfw ausente: deve NÃO sair com rc=2.
			// cmd.exe devolve ERRORLEVEL≈9009 quando o comando não é reconhecido;
			// 9009 ≠ 2, logo a garantia fail-closed com rc específico não é satisfeita.
			rcStripped, _ := runCmdExeLine(t, cmdExe, stripped, "", 0, false)
			t.Logf("contra-arm/%s: forma podada absent rc=%d (esperado ≠2, provável 9009)",
				g.name, rcStripped)
			if rcStripped == 2 {
				t.Errorf("contra-arm/%s: forma podada saiu rc=2 sem || exit 2 — "+
					"sufixo não é load-bearing (inesperado; verificar ambiente)",
					g.name)
			}

			// Forma real com trackfw ausente: deve sair rc=2 (re-afirmação do absent case).
			// Esta re-afirmação prova que a forma real reprova o contra-braço.
			rcReal, _ := runCmdExeLine(t, cmdExe, g.line, "", 0, false)
			if rcReal != 2 {
				t.Errorf("contra-arm/%s: forma real absent rc=%d, want 2 — "+
					"|| exit 2 não está a garantir fail-closed",
					g.name, rcReal)
			}
		})
	}
}
