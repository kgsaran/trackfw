package validator

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1C.
//
// hades-tf's ML-1B barrier found that a FIFO in place of any guard config/script file makes
// os.ReadFile block INDEFINITELY, in all 3 runtimes. These tests exercise readRegularFile (the
// primitive both hook_resolvable and *_script_integrity now call through) directly, before the
// higher-level tests in validator_credential_guard_integrity_test.go and
// validator_git_branch_guard_test.go exercise it through the actual rules.
//
// FIFO and unix-socket cases live in regularfile_fifo_unix_test.go, tagged `!windows`: they import
// "syscall" and call syscall.Mkfifo, which does not exist in the windows syscall package —
// `runtime.GOOS == "windows"` runtime skips do NOT stop that file from failing to COMPILE on
// windows (go vet/go build still processes every _test.go file regardless of which tests it will
// skip at run time), so the split has to be a build tag, not a runtime branch. This file holds
// only the tests that compile identically on every platform.

// readWithDeadline runs fn in a goroutine and fails the test if fn has not returned within d. If
// fn is genuinely hung (the defect this ML closes), the goroutine leaks for the remainder of the
// test process's life — acceptable here because os.Exit at process end tears down the OS thread
// regardless of what syscall it is blocked in; the point of this helper is to make a HANG a fast,
// deterministic test FAILURE instead of a `go test` timeout with no attribution to which case hung.
func readWithDeadline(t *testing.T, d time.Duration, path string) (content []byte, err error, hung bool) {
	t.Helper()
	type result struct {
		content []byte
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		c, e := readRegularFile(path)
		ch <- result{c, e}
	}()
	select {
	case r := <-ch:
		return r.content, r.err, false
	case <-time.After(d):
		return nil, nil, true
	}
}

// TestReadRegularFile_ArquivoRegular_LeNormalmente é o controle de falso positivo: um arquivo
// regular comum não deve, por causa desta mudança, passar a ser rejeitado ou ter seu conteúdo
// alterado.
func TestReadRegularFile_ArquivoRegular_LeNormalmente(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	want := []byte(`{"hooks":{}}`)
	if err := os.WriteFile(p, want, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err, hung := readWithDeadline(t, 3*time.Second, p)
	if hung {
		t.Fatal("readRegularFile não deveria travar num arquivo regular comum")
	}
	if err != nil {
		t.Fatalf("erro inesperado lendo arquivo regular: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("conteúdo alterado — esperado %q, obteve %q", want, got)
	}
}

// TestReadRegularFile_Ausente_RetornaENOENT é o segundo controle: ausência continua sendo
// classificada como os.IsNotExist(err) — o estado legítimo "nenhum guard aqui" não pode virar uma
// acusação por causa desta mudança.
func TestReadRegularFile_Ausente_RetornaENOENT(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "does-not-exist.json")

	_, err, hung := readWithDeadline(t, 3*time.Second, p)
	if hung {
		t.Fatal("readRegularFile não deveria travar num caminho ausente")
	}
	if !os.IsNotExist(err) {
		t.Errorf("esperado os.IsNotExist(err) == true para caminho ausente, obteve: %v", err)
	}
}

// TestReadRegularFile_SymlinkParaArquivoRegularExterno_LeNormalmente é o terceiro controle,
// reproduzindo o corpus adversarial da barreira anterior (symlink válido para fora do diretório):
// a mudança para abrir-depois-classificar não pode quebrar o caso legítimo de symlink.
func TestReadRegularFile_SymlinkParaArquivoRegularExterno_LeNormalmente(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Symlink pode exigir privilégio elevado no Windows")
	}
	dir := t.TempDir()
	external := filepath.Join(dir, "external.json")
	want := []byte(`{"hooks":{}}`)
	if err := os.WriteFile(external, want, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(external, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	got, err, hung := readWithDeadline(t, 3*time.Second, link)
	if hung {
		t.Fatal("readRegularFile não deveria travar seguindo um symlink válido para um arquivo regular")
	}
	if err != nil {
		t.Fatalf("erro inesperado lendo através de symlink válido: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("conteúdo alterado através de symlink — esperado %q, obteve %q", want, got)
	}
}
