//go:build !windows

// Tagged !windows because syscall.Mkfifo does not exist in the Windows syscall
// package. A runtime.GOOS check inside the test body would not stop the file
// from failing to compile on Windows (go vet/go build processes every _test.go
// file regardless of which tests will run), so the split must be at the build-tag
// level. The cross-platform tests (regular file, directory, absent) live in
// fileutil_test.go.

package fileutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/fileutil"
)

// TestReadRegularFile_FIFO_NaoTravaERetornaErro asserts the core property:
// ReadRegularFile on a POSIX named pipe (FIFO) with no writer returns quickly
// with ErrNotRegularFile instead of blocking indefinitely.
//
// Conclusion (ML-FA2): ReadRegularFile does not hang on a FIFO — the O_NONBLOCK
// + fstat pattern closes the attack that makes os.ReadFile block indefinitely on
// a FIFO planted at a guard config path; this test FAILS (hangs for 2 s and
// reports "hung") if the implementation uses os.ReadFile instead of O_NONBLOCK.
func TestReadRegularFile_FIFO_NaoTravaERetornaErro(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hooks.json")
	if err := syscall.Mkfifo(p, 0644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	_, err, hung := readWithDeadline(t, 2*time.Second, p)
	if hung {
		t.Fatal("ReadRegularFile blocked on a FIFO — O_NONBLOCK fix not effective")
	}
	if err == nil {
		t.Fatal("expected error for FIFO, got nil")
	}
	if os.IsNotExist(err) {
		t.Errorf("FIFO must not be reported as absent: %v", err)
	}
	if !errors.Is(err, fileutil.ErrNotRegularFile) {
		t.Errorf("expected ErrNotRegularFile for FIFO, got: %v", err)
	}
}
