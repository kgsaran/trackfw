//go:build !windows

package validator

import (
	"errors"
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

