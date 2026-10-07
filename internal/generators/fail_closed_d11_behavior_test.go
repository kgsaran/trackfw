// Package generators — ML-6B behavior test for the D11 fail-closed hook line.
//
// Reconciliação: este teste afirma que a linha D11
// ("$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard <nome>; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE")
// produz exit 2 quando o binário está ausente do PATH (cenário A), exit 2 quando o guard
// nega (cenário B) e exit 0 quando o guard permite (cenário C).
// O mecanismo testado é o poliglota PS/POSIX da ADR-2026-10-04 D11.
package generators

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// d11LineFor retorna a linha D11 PS/POSIX para o subcomando dado.
func d11LineFor(subcmd string) string {
	return fmt.Sprintf(
		`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard %s; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`,
		subcmd,
	)
}

// findShell retorna o caminho do shell sh (ou bash como fallback) disponível no sistema.
func findShell(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"sh", "bash"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	t.Skip("nenhum shell POSIX (sh/bash) encontrado — cenário não aplicável neste ambiente")
	return ""
}

// runD11Line executa a linha D11 via o shell fornecido, com PATH sobrescrito se pathOverride != "".
// Retorna o exit code da execução.
func runD11Line(t *testing.T, shell, line, pathOverride string) int {
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

// TestD11FailClosed_CenarioA_AusenteRetorna2 afirma que a linha D11 retorna exit 2
// quando o binário trackfw está ausente do PATH (shell falha fechado, não aberto).
// Reconciliação ML-6B: cenário A — ausente → 2.
func TestD11FailClosed_CenarioA_AusenteRetorna2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linha D11 PS/POSIX não se aplica diretamente ao cmd.exe — skip em Windows")
	}
	shell := findShell(t)
	line := d11LineFor("git-branch")

	// PATH vazio: trackfw não existe em nenhum diretório acessível.
	got := runD11Line(t, shell, line, "/dev/null/nonexistent")
	if got != 2 {
		t.Errorf("cenário A (ausente): esperado exit 2, obteve %d", got)
	}
}

// TestD11FailClosed_CenarioB_DenyRetorna2 afirma que a linha D11 retorna exit 2
// quando o trackfw existe mas retorna exit não-zero (guard nega a operação).
// Reconciliação ML-6B: cenário B — deny → 2.
func TestD11FailClosed_CenarioB_DenyRetorna2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linha D11 PS/POSIX não se aplica diretamente ao cmd.exe — skip em Windows")
	}
	shell := findShell(t)

	// Stub: um executável "trackfw" que sai com código 1 (deny).
	dir := t.TempDir()
	stub := filepath.Join(dir, "trackfw")
	stubContent := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(stub, []byte(stubContent), 0755); err != nil {
		t.Fatalf("escrever stub deny: %v", err)
	}

	line := d11LineFor("git-branch")
	got := runD11Line(t, shell, line, dir)
	if got != 2 {
		t.Errorf("cenário B (deny): esperado exit 2, obteve %d", got)
	}
}

// TestD11FailClosed_CenarioC_AllowRetorna0 afirma que a linha D11 retorna exit 0
// quando o trackfw existe e retorna exit 0 (guard permite a operação).
// Reconciliação ML-6B: cenário C — allow → 0.
func TestD11FailClosed_CenarioC_AllowRetorna0(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linha D11 PS/POSIX não se aplica diretamente ao cmd.exe — skip em Windows")
	}
	shell := findShell(t)

	// Stub: um executável "trackfw" que sai com código 0 (allow).
	dir := t.TempDir()
	stub := filepath.Join(dir, "trackfw")
	stubContent := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(stub, []byte(stubContent), 0755); err != nil {
		t.Fatalf("escrever stub allow: %v", err)
	}

	line := d11LineFor("git-branch")
	got := runD11Line(t, shell, line, dir)
	if got != 0 {
		t.Errorf("cenário C (allow): esperado exit 0, obteve %d", got)
	}
}
