package integrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mute_guard_speaks_test.go — ML-7B / AC4 of REQ-2026-08-31.
//
// Reconciliation (one sentence): TestRejectSymlinksDelegateIsAudible affirms the
// ML's conclusion that manager.go's one-line delegate was the THIRD mute site —
// neither it nor its two callers in Manager.resolve printed anything, so the
// refusal reached the user as a bare error; the assertion is therefore on stderr,
// which is the only observable that distinguishes the fix from the defect.
//
// 🔴 No t.Parallel: the test swaps os.Stderr, a package variable read at call time.

func captureStderrIntegrations(t *testing.T, body func()) string {
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

func TestRejectSymlinksDelegateIsAudible(t *testing.T) {
	root := t.TempDir()
	victim := t.TempDir()
	if err := os.Symlink(victim, filepath.Join(root, ".trackfw")); err != nil {
		t.Fatalf("planting the bait symlink: %v", err)
	}
	target := filepath.Join(root, ".trackfw", "integrations-manifest.json")

	// Control arm: a clean path must refuse nothing and print nothing. Without
	// it, an empty stderr in the loud arm could mean "guard never ran".
	cleanRoot := t.TempDir()
	cleanTarget := filepath.Join(cleanRoot, ".trackfw", "integrations-manifest.json")
	var cleanErr error
	cleanOutput := captureStderrIntegrations(t, func() {
		cleanErr = rejectSymlinks(cleanRoot, cleanTarget)
	})
	if cleanErr != nil {
		t.Fatalf("clean path must be accepted, got: %v", cleanErr)
	}
	if cleanOutput != "" {
		t.Fatalf("clean path must print nothing, got: %q", cleanOutput)
	}

	var guardErr error
	output := captureStderrIntegrations(t, func() {
		guardErr = rejectSymlinks(root, target)
	})

	if guardErr == nil {
		t.Fatalf("containment not enforced — this site already refused before ML-7B, so a nil check here would be the wrong assertion anyway")
	}
	if output == "" {
		t.Fatalf("rejectSymlinks refused SILENTLY — the refusal existed before ML-7B; only the OUTPUT proves the fix")
	}
	if !strings.Contains(output, "trackfw: refusing write to ") {
		t.Errorf("refusal is audible but not in the single grammar, got: %q", output)
	}
	if !strings.Contains(guardErr.Error(), "refusing symlink path") {
		t.Errorf("refused for the WRONG reason — want the symlink refusal, got: %v", guardErr)
	}
}
