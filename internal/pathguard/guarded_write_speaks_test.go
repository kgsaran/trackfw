package pathguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// guarded_write_speaks_test.go — ML-7B / AC4 of REQ-2026-08-31.
//
// Reconciliation (one sentence): TestGuardedWriteRefusalIsAudible affirms the
// ML's conclusion that the census of mute sites was drawn by the wrong ruler —
// GuardedWrite refused silently too, and all three of its callers
// (internal/identity, internal/thirdparty/quarantine, internal/thirdparty/
// provenance) only wrapped the error, so it is the same mechanism as the 3
// named mute sites and belongs in the same fix, per the Regra Dura de Causa Raiz.
//
// 🔴 No t.Parallel: the test swaps os.Stderr, a package variable read at call time.

func captureStderrPathguard(t *testing.T, body func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	original := os.Stderr
	os.Stderr = writer
	done := make(chan string, 1)
	go func() {
		var builder strings.Builder
		buffer := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buffer)
			if n > 0 {
				builder.Write(buffer[:n])
			}
			if readErr != nil {
				break
			}
		}
		done <- builder.String()
	}()
	body()
	os.Stderr = original
	writer.Close() //nolint:errcheck
	captured := <-done
	reader.Close() //nolint:errcheck
	return captured
}

func TestGuardedWriteRefusalIsAudible(t *testing.T) {
	// Control arm: a clean destination is written and nothing is printed.
	clean := t.TempDir()
	var cleanErr error
	cleanOutput := captureStderrPathguard(t, func() {
		cleanErr = GuardedWrite(clean, filepath.Join(clean, "sub", "file.json"), []byte("{}\n"), 0o600)
	})
	if cleanErr != nil {
		t.Fatalf("clean destination must be written, got: %v", cleanErr)
	}
	if cleanOutput != "" {
		t.Fatalf("clean destination must print nothing, got: %q", cleanOutput)
	}
	if _, err := os.Stat(filepath.Join(clean, "sub", "file.json")); err != nil {
		t.Fatalf("control arm did not reach the write (%v) — the loud arm would prove nothing", err)
	}

	// Loud arm: an ancestor of the destination is a symlink pointing outside.
	root := t.TempDir()
	victim := t.TempDir()
	if err := os.Symlink(victim, filepath.Join(root, "sub")); err != nil {
		t.Fatalf("planting the bait symlink: %v", err)
	}

	var err error
	output := captureStderrPathguard(t, func() {
		err = GuardedWrite(root, filepath.Join(root, "sub", "file.json"), []byte("{}\n"), 0o600)
	})

	if err == nil {
		t.Fatalf("containment not enforced by GuardedWrite")
	}
	if output == "" {
		t.Fatalf("GuardedWrite refused SILENTLY — it already refused before ML-7B; only the OUTPUT proves the fix")
	}
	if !strings.Contains(output, "trackfw: refusing write to ") {
		t.Errorf("refusal is audible but not in the single grammar, got: %q", output)
	}
	entries, readErr := os.ReadDir(victim)
	if readErr != nil {
		t.Fatalf("reading the victim dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("containment violated: %d file(s) written into the victim dir", len(entries))
	}
}

// TestRefuseUnverifiableRootSpeaksAndWraps — reconciliation (one sentence): it
// affirms the ML's conclusion that the 4 fail-closed sites (metrics, validator,
// configure, config_agents_register), which emitted "refusing write to …: cannot
// verify containment" from four separate literals, now share one emitter, and
// that the substring their existing fail-closed tests assert on is preserved.
func TestRefuseUnverifiableRootSpeaksAndWraps(t *testing.T) {
	cause := os.ErrPermission
	var err error
	output := captureStderrPathguard(t, func() {
		err = RefuseUnverifiableRoot("trackfw.yaml", cause)
	})
	if err == nil {
		t.Fatalf("RefuseUnverifiableRoot must always return an error — it is the fail-closed path")
	}
	if !strings.Contains(output, "trackfw: refusing write to trackfw.yaml: cannot verify containment: ") {
		t.Errorf("stderr must carry the single grammar plus the reason clause, got: %q", output)
	}
	if !strings.Contains(err.Error(), "cannot verify containment") {
		t.Errorf("returned error must keep the substring the fail-closed tests assert on, got: %v", err)
	}
	if !strings.Contains(err.Error(), cause.Error()) {
		t.Errorf("returned error must wrap the cause, got: %v", err)
	}
}
