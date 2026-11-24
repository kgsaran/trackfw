//go:build !windows

// Tagged !windows because syscall.Mkfifo does not exist in the Windows syscall
// package. A runtime.GOOS check inside the test body would not stop the file
// from failing to compile on Windows (go vet/go build processes every _test.go
// file regardless of which tests will run), so the split must be at the build-tag
// level. The cross-platform D1/D2/D3 tests live in writescope_nested_root_test.go.
//
// ML-1A, REQ-2026-11-23, ADR-2026-11-24 D3.
package guard

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestD3_FIFONamedTrackfwYAML_DenyWithoutHanging asserts that a FIFO named trackfw.yaml triggers deny without blocking.
// Asserts: D3 — fileutil.ReadRegularFile opens with O_NONBLOCK so a writer-less FIFO does not hang; rc=2 and no timeout.
func TestD3_FIFONamedTrackfwYAML_DenyWithoutHanging(t *testing.T) {
	dir := t.TempDir()
	fifoPath := filepath.Join(dir, "trackfw.yaml")
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	target := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	p := payload("Write", target, "agent-123")
	rc, _ := runWS(t, p)
	if rc != 2 {
		t.Fatalf("want rc=2, got %d", rc)
	}
}
