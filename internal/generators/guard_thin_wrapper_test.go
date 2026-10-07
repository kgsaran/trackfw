package generators

// guard_thin_wrapper_test.go — ML-2A corretivo (Defeito 1): testes de fail-closed para os
// invólucros finos scripts/trackfw-git-branch-guard.sh e scripts/trackfw-credential-guard.sh.
//
// O que este arquivo afirma (ver comentários AFIRMA em cada teste):
//   - Sem trackfw no PATH → rc=2 + stderr "not found in PATH"
//   - trackfw no PATH mas sem subcomando guard → rc=2 + stderr "subcommand unavailable"
//   - trackfw real no PATH → delega normalmente (rc≠2 para ls, rc=2 para git push)
//   - Credential guard fora de projeto (sem trackfw.yaml) → rc=0 (no-op intencional),
//     independentemente da disponibilidade do trackfw.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers locais
// ---------------------------------------------------------------------------

// liveScriptPath devolve o caminho absoluto de um script em scripts/ na raiz do módulo.
func liveScriptPath(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller falhou")
	}
	// thisFile = …/trackfw/internal/generators/guard_thin_wrapper_test.go
	// subir 3 níveis para chegar na raiz do módulo
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	p := filepath.Join(root, "scripts", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("script não encontrado: %s (%v)", p, err)
	}
	return p
}

// runThinWrapper executa o script bash com o env informado e devolve (rc, stdout, stderr).
func runThinWrapper(t *testing.T, dir, scriptPath string, env []string, stdinPayload string) (exitCode int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdinPayload)
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
	t.Fatalf("exec falhou: %v (stderr: %s)", err, errBuf.String())
	return -1, "", ""
}

// envWithPath devolve os.Environ() com PATH substituído pelo valor informado.
func envWithPath(newPath string) []string {
	base := os.Environ()
	result := make([]string, 0, len(base))
	for _, e := range base {
		if !strings.HasPrefix(e, "PATH=") {
			result = append(result, e)
		}
	}
	return append(result, "PATH="+newPath)
}

// makeFakeTrackfwWithoutGuard cria um binário fake de trackfw num diretório temporário.
// O fake responde a qualquer subcomando com rc=0, EXCETO `guard --help` que retorna rc=1
// (simula uma versão antiga sem o subcomando guard).
func makeFakeTrackfwWithoutGuard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "trackfw")
	content := "#!/usr/bin/env bash\n" +
		"if [ \"$1\" = \"guard\" ]; then exit 1; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("criando fake trackfw: %v", err)
	}
	return dir
}

// makeDirWithTrackfwYAML cria um diretório temporário com um trackfw.yaml mínimo.
func makeDirWithTrackfwYAML(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("criando trackfw.yaml: %v", err)
	}
	return dir
}

// ---------------------------------------------------------------------------
// Testes — scripts/trackfw-git-branch-guard.sh
// ---------------------------------------------------------------------------

// AFIRMA: que o invólucro fino do git-branch-guard falha FECHADO (rc=2, mensagem em stderr)
// quando trackfw não está no PATH — garantindo que a ausência do binário não aprova silenciosamente
// um git push (fail-open).
func TestGitBranchGuardWrapper_NoTrackfwInPath_ExitsTwo(t *testing.T) {
	script := liveScriptPath(t, "trackfw-git-branch-guard.sh")
	dir := t.TempDir()
	// PATH contém apenas diretórios que têm bash/echo/etc., mas não trackfw.
	safePathDirs := findSafePathDirs(t)
	env := envWithPath(strings.Join(safePathDirs, string(filepath.ListSeparator)))

	payload := `{"tool_input":{"command":"git push origin main"}}`
	rc, _, stderr := runThinWrapper(t, dir, script, env, payload)

	if rc != 2 {
		t.Errorf("rc want 2 (fail-closed sem trackfw), got %d (stderr: %s)", rc, stderr)
	}
	if !strings.Contains(stderr, "not found in PATH") {
		t.Errorf("stderr deveria conter 'not found in PATH'; got: %s", stderr)
	}
}

// AFIRMA: que o invólucro fino do git-branch-guard falha FECHADO (rc=2) quando trackfw
// está no PATH mas não tem o subcomando guard — garantindo que uma versão desatualizada do
// binário não aprova silenciosamente comandos bloqueados.
func TestGitBranchGuardWrapper_TrackfwWithoutGuardSubcommand_ExitsTwo(t *testing.T) {
	script := liveScriptPath(t, "trackfw-git-branch-guard.sh")
	dir := t.TempDir()
	fakeDir := makeFakeTrackfwWithoutGuard(t)
	// Insere o fake no início do PATH, garantindo que seja encontrado antes do real.
	origPath := os.Getenv("PATH")
	env := envWithPath(fakeDir + string(filepath.ListSeparator) + origPath)

	payload := `{"tool_input":{"command":"git push origin main"}}`
	rc, _, stderr := runThinWrapper(t, dir, script, env, payload)

	if rc != 2 {
		t.Errorf("rc want 2 (fail-closed: guard indisponível), got %d (stderr: %s)", rc, stderr)
	}
	if !strings.Contains(stderr, "subcommand unavailable") {
		t.Errorf("stderr deveria conter 'subcommand unavailable'; got: %s", stderr)
	}
}

// AFIRMA: que o invólucro fino do git-branch-guard delega ao binário real e bloqueia git push
// quando o binário trackfw está no PATH — comprovando que a cadeia invólucro→binário funciona.
func TestGitBranchGuardWrapper_WithRealBinary_BlocksGitPush(t *testing.T) {
	script := liveScriptPath(t, "trackfw-git-branch-guard.sh")
	dir := makeDirWithTrackfwYAML(t)
	env := injectGuardBinaryPath(t, os.Environ())

	payload := `{"tool_input":{"command":"git push origin main"}}`
	rc, _, _ := runThinWrapper(t, dir, script, env, payload)

	if rc != 2 {
		t.Errorf("rc want 2 (git push bloqueado pelo binário real), got %d", rc)
	}
}

// AFIRMA: que o invólucro fino do git-branch-guard delega ao binário real e permite ls
// (comando não bloqueado) — comprovando que o invólucro não bloqueia tudo indiscriminadamente.
func TestGitBranchGuardWrapper_WithRealBinary_AllowsNonBlockedCommand(t *testing.T) {
	script := liveScriptPath(t, "trackfw-git-branch-guard.sh")
	dir := makeDirWithTrackfwYAML(t)
	env := injectGuardBinaryPath(t, os.Environ())

	payload := `{"tool_input":{"command":"ls -la"}}`
	rc, _, stderr := runThinWrapper(t, dir, script, env, payload)

	if rc != 0 {
		t.Errorf("rc want 0 (ls não bloqueado), got %d (stderr: %s)", rc, stderr)
	}
}

// ---------------------------------------------------------------------------
// Testes — scripts/trackfw-credential-guard.sh
// ---------------------------------------------------------------------------

// AFIRMA: que o invólucro fino do credential-guard é um no-op (rc=0) quando não há
// trackfw.yaml no cwd — independentemente da disponibilidade do binário trackfw.
// Esta garantia é intencional e deve ser preservada: o credential-guard não deve falhar
// fora de projetos trackfw.
func TestCredentialGuardWrapper_NoTrackfwYAML_IsNoOp(t *testing.T) {
	script := liveScriptPath(t, "trackfw-credential-guard.sh")
	// dir sem trackfw.yaml — simula execução fora de projeto.
	dir := t.TempDir()
	// PATH sem trackfw, para garantir que o no-op é pela ausência do trackfw.yaml,
	// não pela ausência do binário.
	safePathDirs := findSafePathDirs(t)
	env := envWithPath(strings.Join(safePathDirs, string(filepath.ListSeparator)))

	rc, _, _ := runThinWrapper(t, dir, script, env, "")

	if rc != 0 {
		t.Errorf("rc want 0 (no-op fora de projeto), got %d", rc)
	}
}

// AFIRMA: que o invólucro fino do credential-guard falha FECHADO (rc=2, mensagem em stderr)
// quando há trackfw.yaml no cwd mas trackfw não está no PATH.
func TestCredentialGuardWrapper_NoTrackfwInPath_ExitsTwo(t *testing.T) {
	script := liveScriptPath(t, "trackfw-credential-guard.sh")
	dir := makeDirWithTrackfwYAML(t)
	safePathDirs := findSafePathDirs(t)
	env := envWithPath(strings.Join(safePathDirs, string(filepath.ListSeparator)))

	rc, _, stderr := runThinWrapper(t, dir, script, env, "")

	if rc != 2 {
		t.Errorf("rc want 2 (fail-closed sem trackfw dentro de projeto), got %d (stderr: %s)", rc, stderr)
	}
	if !strings.Contains(stderr, "not found in PATH") {
		t.Errorf("stderr deveria conter 'not found in PATH'; got: %s", stderr)
	}
}

// AFIRMA: que o invólucro fino do credential-guard falha FECHADO (rc=2) quando trackfw
// está no PATH mas não tem o subcomando guard.
func TestCredentialGuardWrapper_TrackfwWithoutGuardSubcommand_ExitsTwo(t *testing.T) {
	script := liveScriptPath(t, "trackfw-credential-guard.sh")
	dir := makeDirWithTrackfwYAML(t)
	fakeDir := makeFakeTrackfwWithoutGuard(t)
	origPath := os.Getenv("PATH")
	env := envWithPath(fakeDir + string(filepath.ListSeparator) + origPath)

	rc, _, stderr := runThinWrapper(t, dir, script, env, "")

	if rc != 2 {
		t.Errorf("rc want 2 (fail-closed: guard indisponível), got %d (stderr: %s)", rc, stderr)
	}
	if !strings.Contains(stderr, "subcommand unavailable") {
		t.Errorf("stderr deveria conter 'subcommand unavailable'; got: %s", stderr)
	}
}

// ---------------------------------------------------------------------------
// findSafePathDirs: devolve os diretórios do PATH que contêm bash mas não trackfw.
// ---------------------------------------------------------------------------

// findSafePathDirs retorna os diretórios do PATH atual que NÃO contêm o binário trackfw
// (nem trackfw.exe no Windows), mas mantêm bash, echo e demais utilitários necessários.
func findSafePathDirs(t *testing.T) []string {
	t.Helper()
	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	var safe []string
	for _, d := range pathDirs {
		hasTrackfw := false
		for _, name := range []string{"trackfw", "trackfw.exe"} {
			if _, err := os.Stat(filepath.Join(d, name)); err == nil {
				hasTrackfw = true
				break
			}
		}
		if !hasTrackfw {
			safe = append(safe, d)
		}
	}
	if len(safe) == 0 {
		t.Fatal("PATH ficou vazio após remover dirs com trackfw — não é possível executar bash")
	}
	return safe
}
