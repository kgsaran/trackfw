//go:build !windows

package validator

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1C.
//
// Tagged `!windows` because syscall.Mkfifo does not exist in the windows syscall package — a
// runtime.GOOS=="windows" skip inside the test body would not stop this file from failing to
// COMPILE on windows (go vet/go build process every _test.go file regardless of which tests run),
// so the split has to be at the build-tag level. See regularfile_test.go for the cross-platform
// tests (regular file, absence, symlink).

// TestReadRegularFile_FIFO_NaoTravaEAcusaTipoErrado afirma a conclusão central deste ML: um FIFO
// no lugar do arquivo não trava mais a leitura — readRegularFile retorna rápido com um erro
// diferente de ENOENT (o mesmo braço "could not be read" que toda a família de regras já usa).
func TestReadRegularFile_FIFO_NaoTravaEAcusaTipoErrado(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	if err := syscall.Mkfifo(p, 0644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	_, err, hung := readWithDeadline(t, 3*time.Second, p)
	if hung {
		t.Fatal("readRegularFile travou indefinidamente num FIFO sem escritor — exatamente o hang que hades-tf's ML-1B barrier reportou; o remédio não fechou")
	}
	if err == nil {
		t.Fatal("esperado erro (FIFO não é arquivo regular), obteve nil")
	}
	if os.IsNotExist(err) {
		t.Errorf("erro de FIFO não deve ser classificado como ENOENT (o arquivo EXISTE, só não é regular) — obteve: %v", err)
	}
	if !errors.Is(err, errNotRegularFile) {
		t.Errorf("esperado errNotRegularFile, obteve: %v", err)
	}
}

// TestReadRegularFile_Socket_NaoTravaEAcusaTipoErrado afirma a conclusão de enumeração: a checagem
// "deve ser arquivo regular" é um ALLOWLIST positivo — cobre socket sem precisar de um branch
// dedicado a socket, exatamente como cobre FIFO sem um branch dedicado a FIFO.
func TestReadRegularFile_Socket_NaoTravaEAcusaTipoErrado(t *testing.T) {
	// Uses /tmp directly (not t.TempDir()) — unix domain socket paths are limited to ~104 bytes
	// (sun_path) on macOS/BSD, and t.TempDir()'s nested per-test path routinely exceeds that.
	dir, err := os.MkdirTemp("", "tfsock")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	p := filepath.Join(dir, "s.json")

	l, err := net.Listen("unix", p)
	if err != nil {
		t.Fatalf("net.Listen(unix): %v", err)
	}
	defer l.Close()

	_, readErr, hung := readWithDeadline(t, 3*time.Second, p)
	if hung {
		t.Fatal("readRegularFile travou num socket — a checagem de tipo deveria ter classificado e retornado antes de qualquer read")
	}
	if readErr == nil {
		t.Fatal("esperado erro (socket não é arquivo regular), obteve nil")
	}
	// On macOS/BSD, open(2) itself refuses a socket path with ENOTSUP/ENXIO before this function's
	// own fstat/IsRegular check ever runs — so the error here is open()'s raw error, not always the
	// errNotRegularFile sentinel (that sentinel only fires when open() SUCCEEDS on something that
	// turns out not to be regular, e.g. a FIFO). Both paths converge on the property that matters
	// for every caller: non-ENOENT, so the rule accuses instead of silently skipping.
	if os.IsNotExist(readErr) {
		t.Errorf("erro de socket não deve ser classificado como ENOENT (o arquivo EXISTE, só não é regular) — obteve: %v", readErr)
	}
}
