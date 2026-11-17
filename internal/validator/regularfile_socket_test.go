package validator

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1C /
// REQ-2026-11-15 ML-3A2.
//
// No build tag — runs on all platforms. The original test was tagged !windows because a
// PENDING note in the evidence claimed that on Windows "os.Open on the socket path does not
// return ModeSocket and the behavior diverges from the test assertion." That claim was falsified
// by direct measurement on Windows ARM64 go1.25.2:
//
//	net.Listen("unix", %TEMP%\tfsockNNN\s.json): <nil>
//	lstat mode=Srw-rw-rw- regular=false socket=true
//	RESULT: errNotRegularFile before open (no read, no hang)
//
// The Windows production code path (regularfile_windows.go) uses Stat-then-Open: os.Stat
// reports ModeSocket (regular=false), so openRegularFileNonblock returns errNotRegularFile
// before any open — the socket is rejected by the positive ALLOWLIST, exactly as on Unix.

// TestReadRegularFile_Socket_NaoTravaEAcusaTipoErrado asserts the enumeration conclusion: the
// "must be a regular file" check is a positive ALLOWLIST — covers socket without a dedicated
// socket branch, exactly as it covers FIFO without a dedicated FIFO branch.
//
// Error polarity per platform:
//   - Windows: errNotRegularFile (Stat path, measured). errors.Is asserted.
//   - macOS/BSD: open(2) itself may refuse with ENOTSUP/ENXIO before the fstat check runs,
//     so errNotRegularFile is not guaranteed; only the weaker "non-nil, non-ENOENT" holds.
//   - Linux: O_NONBLOCK open on a socket usually returns ENXIO, also non-nil non-ENOENT.
func TestReadRegularFile_Socket_NaoTravaEAcusaTipoErrado(t *testing.T) {
	// Uses os.MkdirTemp (not t.TempDir) — unix domain socket paths are limited to ~104 bytes
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
	// On macOS/BSD, open(2) itself refuses a socket path with ENOTSUP/ENXIO before this
	// function's own fstat/IsRegular check ever runs — so the error here is open()'s raw
	// error, not always the errNotRegularFile sentinel (that sentinel only fires when open()
	// SUCCEEDS on something that turns out not to be regular, e.g. a FIFO). Both paths
	// converge on the property that matters for every caller: non-ENOENT, so the rule accuses
	// instead of silently skipping.
	if os.IsNotExist(readErr) {
		t.Errorf("erro de socket não deve ser classificado como ENOENT (o arquivo EXISTE, só não é regular) — obteve: %v", readErr)
	}
	// On Windows, openRegularFileNonblock (regularfile_windows.go) uses Stat-then-Open:
	// os.Stat returns ModeSocket (regular=false), so errNotRegularFile is always the
	// sentinel — measured directly on Windows ARM64 go1.25.2 (see package-level comment).
	if runtime.GOOS == "windows" {
		if !errors.Is(readErr, errNotRegularFile) {
			t.Errorf("Windows: esperado errNotRegularFile (Stat detecta ModeSocket antes do open), obteve: %v", readErr)
		}
	}
}
