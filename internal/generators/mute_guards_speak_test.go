package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// mute_guards_speak_test.go — ML-7B / AC4 + AC5 of REQ-2026-08-31.
//
// What each test affirms (Regra Dura de Reconciliação, one sentence each):
//
//   - TestAppendTransitionLogRefusalIsAudible affirms the ML's conclusion that
//     roadmap.go's transition-log guard was one of the 3 MUTE sites: it already
//     refused, so only the OUTPUT can tell the fix from the defect — the assertion
//     is on stderr, never on an error value (there is none to inspect).
//   - TestAppendREQTransitionLogRefusalIsAudible affirms the same conclusion for
//     the second mute site, req.go's REQ transition log.
//   - TestRejectHarnessSymlinkSpeaksTheSingleGrammar affirms that the
//     4-parameter variant (rejectHarnessSymlink, $HOME scope) became a thin
//     adapter over the single emitter: its TargetResult contract is unchanged and
//     its message is now the one grammar.
//
// 🔴 Why each test carries a no-symlink control arm: in ML-7A a mutant
// degenerated into the pre-fix state because the fixture directory did not
// exist, and the arm passed while measuring nothing. Here the degeneration mode
// is different but as quiet: both log helpers wrap the guard in
// `if root, err := projectRoot(); err == nil`, so a fixture that does not make
// the guard reachable produces empty stderr BEFORE and AFTER the fix. The
// control arm proves the guard was reachable and silent when the path is clean,
// which is the only way the loud arm means what it says.
//
// 🔴 No t.Parallel in this file: every test swaps os.Stderr, which is a package
// variable read at call time.

// captureStderr runs body with os.Stderr replaced by a pipe and returns
// everything written to it.
func captureStderr(t *testing.T, body func()) string {
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

// firstSegment returns the first path element of a relative path, or "" when the
// path is absolute or has no usable first element.
func firstSegment(relative string) string {
	if filepath.IsAbs(relative) {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for _, part := range parts {
		if part != "" && part != "." {
			return part
		}
	}
	return ""
}

// plantAncestorSymlink makes <root>/<segment> a symlink pointing at victim and
// returns the victim path. Every write under <root>/<segment>/… must now be
// refused by the containment guard.
func plantAncestorSymlink(t *testing.T, root, segment string) string {
	t.Helper()
	victim := t.TempDir()
	link := filepath.Join(root, segment)
	if err := os.Symlink(victim, link); err != nil {
		t.Fatalf("planting the bait symlink %s → %s: %v", link, victim, err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("bait %s is not a symlink (lstat err=%v) — this arm would pass for the wrong reason", link, err)
	}
	return victim
}

func assertVictimEmpty(t *testing.T, victim string) {
	t.Helper()
	entries, err := os.ReadDir(victim)
	if err != nil {
		t.Fatalf("reading the victim dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("containment violated: %d file(s) written into the victim dir %s", len(entries), victim)
	}
}

// singleGrammar is the one message the binary is allowed to print for a
// containment refusal, as emitted by pathguard.RejectAndReport.
const singleGrammar = "trackfw: refusing write to "

func TestAppendTransitionLogRefusalIsAudible(t *testing.T) {
	logRelative := logPath()
	segment := firstSegment(logRelative)
	if segment == "" {
		t.Skipf("roadmap log path %q has no relative first segment — the ancestor-symlink bait cannot be planted, so this arm was NOT exercised", logRelative)
	}

	// ── control arm: clean tree, guard reachable, nothing printed ──────────
	clean := t.TempDir()
	t.Chdir(clean)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(clean, logRelative)), 0o755); err != nil {
		t.Fatalf("building the clean fixture: %v", err)
	}
	controlOutput := captureStderr(t, func() {
		appendTransitionLog("ROADMAP-control.md", "backlog", "wip")
	})
	if controlOutput != "" {
		t.Fatalf("clean tree must print nothing, got: %q", controlOutput)
	}
	if _, err := os.Stat(filepath.Join(clean, logRelative)); err != nil {
		t.Fatalf("control arm did not reach the write (%v) — the guard was never exercised, so the loud arm below would prove nothing", err)
	}

	// ── loud arm: ancestor symlink, guard fires, refusal must be AUDIBLE ───
	baited := t.TempDir()
	victim := plantAncestorSymlink(t, baited, segment)
	t.Chdir(baited)

	output := captureStderr(t, func() {
		appendTransitionLog("ROADMAP-bait.md", "backlog", "wip")
	})

	if output == "" {
		t.Fatalf("appendTransitionLog refused SILENTLY — it already refused before ML-7B, so an err != nil assertion would have passed without the fix; the only observable is stderr")
	}
	if !strings.Contains(output, singleGrammar) {
		t.Errorf("refusal is audible but not in the single grammar %q, got: %q", singleGrammar, output)
	}
	if !strings.Contains(output, "refusing symlink path") {
		t.Errorf("refused for the WRONG reason — want the symlink refusal, got: %q", output)
	}
	assertVictimEmpty(t, victim)
}

func TestAppendREQTransitionLogRefusalIsAudible(t *testing.T) {
	logRelative := filepath.Join(config.Load().REQDir, ".trackfw-log")
	segment := firstSegment(logRelative)
	if segment == "" {
		t.Skipf("REQ log path %q has no relative first segment — the ancestor-symlink bait cannot be planted, so this arm was NOT exercised", logRelative)
	}

	clean := t.TempDir()
	t.Chdir(clean)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(clean, logRelative)), 0o755); err != nil {
		t.Fatalf("building the clean fixture: %v", err)
	}
	controlOutput := captureStderr(t, func() {
		appendREQTransitionLog("REQ-control.md", "backlog", "wip")
	})
	if controlOutput != "" {
		t.Fatalf("clean tree must print nothing, got: %q", controlOutput)
	}
	if _, err := os.Stat(filepath.Join(clean, logRelative)); err != nil {
		t.Fatalf("control arm did not reach the write (%v) — the guard was never exercised, so the loud arm below would prove nothing", err)
	}

	baited := t.TempDir()
	victim := plantAncestorSymlink(t, baited, segment)
	t.Chdir(baited)

	output := captureStderr(t, func() {
		appendREQTransitionLog("REQ-bait.md", "backlog", "wip")
	})

	if output == "" {
		t.Fatalf("appendREQTransitionLog refused SILENTLY — the refusal already happened before ML-7B; only the OUTPUT distinguishes the fix from the defect")
	}
	if !strings.Contains(output, singleGrammar) {
		t.Errorf("refusal is audible but not in the single grammar %q, got: %q", singleGrammar, output)
	}
	if !strings.Contains(output, "refusing symlink path") {
		t.Errorf("refused for the WRONG reason — want the symlink refusal, got: %q", output)
	}
	assertVictimEmpty(t, victim)
}

func TestRejectHarnessSymlinkSpeaksTheSingleGrammar(t *testing.T) {
	home := t.TempDir()
	victim := plantAncestorSymlink(t, home, ".trackfw")
	target := filepath.Join(home, ".trackfw", "scripts", "probe.sh")

	var result TargetResult
	var fired bool
	output := captureStderr(t, func() {
		result, fired = rejectHarnessSymlink(home, target, "probe-target", "~/.trackfw/scripts/probe.sh")
	})

	if !fired {
		t.Fatalf("guard did not fire on a symlinked $HOME ancestor — the fixture is wrong, not the product")
	}
	if result.State != TargetFailed {
		t.Errorf("TargetResult contract changed: want state %v, got %v", TargetFailed, result.State)
	}
	if !strings.Contains(output, singleGrammar) {
		t.Errorf("rejectHarnessSymlink must print the single grammar %q, got: %q", singleGrammar, output)
	}
	if !strings.Contains(result.Message, "refusing symlink path") {
		t.Errorf("refused for the WRONG reason — want the symlink refusal in the message, got: %q", result.Message)
	}
	assertVictimEmpty(t, victim)
}
