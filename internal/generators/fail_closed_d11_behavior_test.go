// Package generators — ML-6C behavior test for the D11 fail-closed hook line.
//
// Reconciliação: este teste afirma que a nova linha D11 (ML-6C)
// ("$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard <nome>; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE")
// produz exit 2 para qualquer código de saída não-zero do guard (RC=1, 2, 3) e para ausência do
// binário (absent), e exit 0 quando o guard permite (RC=0).
// Cobre sh, bash (tabela completa RC=0,1,2,3,absent) e PowerShell via pwsh (t.Skip se ausente).
// O mecanismo testado é o poliglota PS/POSIX da ADR-2026-10-04 D11 (4º segmento introduzido no ML-6C).
package generators

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// d11LineFor retorna a linha D11 PS/POSIX para o subcomando dado, usando as constantes canônicas
// do gerador — não um Sprintf autônomo. Divergência entre gerador e teste detectada em compilação.
// Reconciliação ML-6C: usar constante do gerador garante que o teste reprova quando a constante
// for alterada (ex.: remoção do 4º segmento PS dispara falha no concordance externo E aqui).
func d11LineFor(subcmd string) string {
	switch subcmd {
	case "git-branch":
		return guardGitBranchCmdPSPOSIX
	case "credential":
		return guardCredentialCmdPSPOSIX
	case "credential --global":
		return guardCredentialGlobalCmdPSPOSIX
	default:
		panic("d11LineFor: subcomando desconhecido: " + subcmd)
	}
}

// runWithShell executa a linha via o shell dado, com PATH sobrescrito se pathOverride != "".
// Retorna o exit code da execução.
func runWithShell(t *testing.T, shell, line, pathOverride string) int {
	t.Helper()
	cmd := exec.Command(shell, "-c", line)
	if pathOverride != "" {
		env := os.Environ()
		filtered := make([]string, 0, len(env))
		for _, e := range env {
			if len(e) >= 5 && e[:5] == "PATH=" {
				filtered = append(filtered, "PATH="+pathOverride)
			} else {
				filtered = append(filtered, e)
			}
		}
		cmd.Env = filtered
	}
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
	}
	return 0
}

// writeTrackfwStub cria um executável stub "trackfw" no diretório dado que sai com exitCode.
func writeTrackfwStub(t *testing.T, dir string, exitCode int) {
	t.Helper()
	stub := filepath.Join(dir, "trackfw")
	content := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	if err := os.WriteFile(stub, []byte(content), 0755); err != nil {
		t.Fatalf("escrever stub trackfw (exitCode=%d): %v", exitCode, err)
	}
}

// d11BehaviorCase descreve um caso de tabela para o teste de comportamento D11.
type d11BehaviorCase struct {
	name     string
	stubExit int  // código de saída do stub; -1 = ausente (não no PATH)
	want     int  // código de saída esperado da linha D11
}

var d11BehaviorCases = []d11BehaviorCase{
	{name: "RC=0 (allow)",  stubExit: 0, want: 0},
	{name: "RC=1 (deny)",   stubExit: 1, want: 2},
	{name: "RC=2 (deny)",   stubExit: 2, want: 2},
	{name: "RC=3 (deny)",   stubExit: 3, want: 2},
	{name: "absent",        stubExit: -1, want: 2},
}

// TestD11FailClosed_POSIX_TabelaRC afirma que a linha D11 produz o código de saída esperado
// para cada RC (0,1,2,3,absent) quando executada via sh e bash.
// Reconciliação ML-6C: cobre os 5 cenários da tabela da ADR (seção D11 PS/POSIX revisado).
func TestD11FailClosed_POSIX_TabelaRC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linha D11 PS/POSIX não se aplica diretamente ao cmd.exe — skip em Windows")
	}

	line := d11LineFor("git-branch")

	for _, shellName := range []string{"sh", "bash"} {
		shellPath, err := exec.LookPath(shellName)
		if err != nil {
			t.Logf("%s não encontrado — pulando subtestes deste shell", shellName)
			continue
		}

		for _, tc := range d11BehaviorCases {
			tc := tc
			t.Run(shellName+"/"+tc.name, func(t *testing.T) {
				var pathOverride string
				if tc.stubExit == -1 {
					// Ausente: PATH aponta para diretório sem trackfw.
					pathOverride = "/dev/null/nonexistent"
				} else {
					dir := t.TempDir()
					writeTrackfwStub(t, dir, tc.stubExit)
					pathOverride = dir
				}

				got := runWithShell(t, shellPath, line, pathOverride)
				if got != tc.want {
					t.Errorf("%s/%s: esperado exit %d, obteve %d\nlinha: %s", shellName, tc.name, tc.want, got, line)
				}
			})
		}
	}
}

// TestD11FailClosed_PowerShell_TabelaRC afirma que a linha D11 produz o código de saída esperado
// quando executada via pwsh (PowerShell Core), cobrindo RC=0,1,2,3,absent.
// O stub usa shebang #!/bin/sh — o macOS entrega a invocação ao kernel que respeita o shebang.
// Reconciliação ML-6C: o 4º segmento "$LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}" é
// precisamente o trecho que normaliza RC não-zero para 2 em PS; sem ele RC=1 → exit 1, não 2.
func TestD11FailClosed_PowerShell_TabelaRC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("teste usa stub #!/bin/sh — não aplicável em cmd.exe/PS no Windows desta forma")
	}

	pwshPath, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh não encontrado — cenário PowerShell não aplicável neste ambiente")
	}

	line := d11LineFor("git-branch")

	for _, tc := range d11BehaviorCases {
		tc := tc
		t.Run("pwsh/"+tc.name, func(t *testing.T) {
			var pathOverride string
			if tc.stubExit == -1 {
				pathOverride = "/dev/null/nonexistent"
			} else {
				dir := t.TempDir()
				writeTrackfwStub(t, dir, tc.stubExit)
				pathOverride = dir
			}

			cmd := exec.Command(pwshPath, "-NoProfile", "-Command", line)
			if pathOverride != "" {
				env := os.Environ()
				filtered := make([]string, 0, len(env))
				for _, e := range env {
					if len(e) >= 5 && e[:5] == "PATH=" {
						filtered = append(filtered, "PATH="+pathOverride)
					} else {
						filtered = append(filtered, e)
					}
				}
				cmd.Env = filtered
			}

			got := 0
			if runErr := cmd.Run(); runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					got = exitErr.ExitCode()
				}
			}

			if got != tc.want {
				t.Errorf("pwsh/%s: esperado exit %d, obteve %d\nlinha: %s", tc.name, tc.want, got, line)
			}
		})
	}
}
