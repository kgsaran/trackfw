//go:build !windows

package validator

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1C.
//
// Tagged `!windows` because syscall.Mkfifo does not exist in the windows syscall package — see
// regularfile_fifo_unix_test.go's identical doc comment for why this has to be a build tag, not a
// runtime.GOOS skip.

// TestCredentialGuardScriptIntegrity_FIFO_NaoTrava afirma, no nível da regra (não só do
// primitivo readRegularFile em regularfile_test.go), que um FIFO no lugar do script não trava
// mais `trackfw validate` — reprodução literal do vetor que hades-tf's ML-1B barrier mediu com
// `mkfifo`.
func TestCredentialGuardScriptIntegrity_FIFO_NaoTrava(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	fifoPath := filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")
	if err := syscall.Mkfifo(fifoPath, 0644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	done := make(chan struct {
		msgs []string
		err  error
	}, 1)
	go func() {
		m, e := validateCredentialGuardScriptIntegrity()
		done <- struct {
			msgs []string
			err  error
		}{m, e}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("erro inesperado: %v", r.err)
		}
		if !hasViolation(r.msgs, "could not be read") {
			t.Errorf("esperado violation de leitura para FIFO, obteve: %v", r.msgs)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("validateCredentialGuardScriptIntegrity() travou indefinidamente num FIFO — hang que este ML existe para fechar")
	}
}
