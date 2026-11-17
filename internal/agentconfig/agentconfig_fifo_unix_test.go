//go:build !windows

// Tagged !windows because syscall.Mkfifo does not exist in the Windows syscall
// package. A runtime.GOOS check inside the test body would not stop the file
// from failing to compile on Windows (go vet/go build processes every _test.go
// regardless of which tests run), so the split must be at the build-tag level.
// The counter-arms (regular file → present=true) are cross-platform and live in
// agentconfig_test.go.

package agentconfig_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// inertGuardReadWithDeadline runs fn in a goroutine and reports whether it hung
// past deadline d.
func inertGuardReadWithDeadline(t *testing.T, d time.Duration, fn func() (string, bool)) (path string, present bool, hung bool) {
	t.Helper()
	type result struct {
		path    string
		present bool
	}
	ch := make(chan result, 1)
	go func() {
		p, ok := fn()
		ch <- result{p, ok}
	}()
	select {
	case r := <-ch:
		return r.path, r.present, false
	case <-time.After(d):
		return "", false, true
	}
}

// TestInertGuardPath_FIFODoesNotBlock asserts the ML-FA2 conclusion for the
// FIFO-attack surface: when HOME/.codex/hooks.json or HOME/.claude/settings.json
// is replaced by a POSIX named pipe (FIFO) with no writer, the two Inert*GuardPath
// functions return in under 2 s with present=false instead of blocking indefinitely.
//
// Conclusion (ML-FA2): fileutil.ReadRegularFile's O_NONBLOCK + fstat prevents the
// hang that os.ReadFile would cause; this test FAILS (each subtest reports "hung")
// if either function uses os.ReadFile instead of fileutil.ReadRegularFile.
//
// Counter-arm (regular file → present=true): see
// TestInertCodexGuardPath_DifferentDirs_WithGuardMarker (agentconfig_test.go) for
// Codex and TestInertClaudeGuardPath_DifferentDirs_WithGuardMarker (agentconfig_test.go)
// for Claude.
func TestInertGuardPath_FIFODoesNotBlock(t *testing.T) {
	t.Run("codex", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		t.Setenv("CODEX_HOME", "")

		codexSubdir := filepath.Join(homeDir, ".codex")
		if err := os.MkdirAll(codexSubdir, 0755); err != nil {
			t.Fatalf("mkdir .codex: %v", err)
		}
		fifo := filepath.Join(codexSubdir, "hooks.json")
		if err := syscall.Mkfifo(fifo, 0644); err != nil {
			t.Fatalf("mkfifo %s: %v", fifo, err)
		}

		// codexDir points elsewhere so InertCodexGuardPath attempts to read
		// HOME/.codex/hooks.json (the FIFO).
		otherDir := t.TempDir()
		_, present, hung := inertGuardReadWithDeadline(t, 2*time.Second, func() (string, bool) {
			return agentconfig.InertCodexGuardPath(otherDir)
		})
		if hung {
			t.Fatal("InertCodexGuardPath blocked on a FIFO in HOME/.codex/hooks.json — O_NONBLOCK fix not effective")
		}
		if present {
			t.Error("InertCodexGuardPath: expected present=false for FIFO (not a readable guard), got true")
		}
	})

	t.Run("claude", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		claudeSubdir := filepath.Join(homeDir, ".claude")
		if err := os.MkdirAll(claudeSubdir, 0755); err != nil {
			t.Fatalf("mkdir .claude: %v", err)
		}
		fifo := filepath.Join(claudeSubdir, "settings.json")
		if err := syscall.Mkfifo(fifo, 0644); err != nil {
			t.Fatalf("mkfifo %s: %v", fifo, err)
		}

		// claudeDir points elsewhere so InertClaudeGuardPath attempts to read
		// HOME/.claude/settings.json (the FIFO).
		otherDir := t.TempDir()
		_, present, hung := inertGuardReadWithDeadline(t, 2*time.Second, func() (string, bool) {
			return agentconfig.InertClaudeGuardPath(otherDir)
		})
		if hung {
			t.Fatal("InertClaudeGuardPath blocked on a FIFO in HOME/.claude/settings.json — O_NONBLOCK fix not effective")
		}
		if present {
			t.Error("InertClaudeGuardPath: expected present=false for FIFO (not a readable guard), got true")
		}
	})
}
