package commands

// barrier_fragment_test.go — ML-2A (REQ #491, Wave 2)
//
// Tests for the gate fragment check: sh -n and odd-\ detection that runs
// before any gate executes in trusted paths. Uses the real compiled binary
// (barrierBinary(t) from barrier_contract_test.go) and cmd.Env without
// TRACKFW_BARRIER_STACK.
//
// Each test name starts with "Fragment" to match the Wave 2 gate:
//   go test ./internal/commands/ -run 'Fragment' -count=1

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// fragmentFixtureDir sets up a minimal trackfw project with a Wave 1 roadmap
// containing the given gate commands. It uses the buildBarrierRoadmap fixture
// builder from barrier_contract_test.go. Returns (dir, roadmap basename).
func fragmentFixtureDir(t *testing.T, gateCommands []string) (string, string) {
	t.Helper()
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     false,
		mlStatus:      "✅ Concluído",
		criteriaLines: []string{"- [x] criterion met"},
		gateCommands:  gateCommands,
	})
	return dir, "ROADMAP-barrier-fixture"
}

// runFragmentBarrier calls the real barrier binary with --trust-local-gates
// (the dir is not a git repo, so trust check would always refuse without it).
// Returns the parsed JSON result document.
func runFragmentBarrier(t *testing.T, dir, roadmapBasename string, extraArgs ...string) barrierResultDoc {
	t.Helper()
	args := []string{roadmapBasename, "--wave", "1", "--json", "--trust-local-gates"}
	args = append(args, extraArgs...)
	stdout, stderr, code := runBarrierCLI(t, dir, args...)
	if code != 0 && code != 1 {
		t.Fatalf("barrier exited %d (want 0 or 1)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	return doc
}

// gatesCheck returns the gates check from a barrier result document.
func gatesCheckFrom(t *testing.T, doc barrierResultDoc) barrierCheckDoc {
	t.Helper()
	for _, c := range doc.Checks {
		if c.Name == "gates" {
			return c
		}
	}
	t.Fatal("gates check not found in result document")
	return barrierCheckDoc{}
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestBarrierFragment_IncompleteCommand asserts that a gate block whose first
// line is a syntactic fragment (`n=$(python3 -c "`) causes gates to report
// "blocked" with the pinned failure message naming the exact source line, and
// that a sentinel touch command on the second line is never executed.
func TestBarrierFragment_IncompleteCommand(t *testing.T) {
	t.Parallel()
	sentinel := filepath.Join(t.TempDir(), "gate-fragment-sentinel")
	gates := []string{
		`n=$(python3 -c "`,
		"touch " + sentinel,
	}
	dir, roadmap := fragmentFixtureDir(t, gates)
	doc := runFragmentBarrier(t, dir, roadmap)
	gc := gatesCheckFrom(t, doc)

	// The gates block must be blocked.
	if gc.Status != "blocked" {
		t.Errorf("gates.status = %q, want \"blocked\"", gc.Status)
	}

	// Exactly one failure, naming line 22 (the fragment), with the exact pinned message.
	wantFailure := fmt.Sprintf(
		"line 22: incomplete command — each line of the gates block runs as a separate sh -c (rule 5): %s",
		`n=$(python3 -c "`)
	if len(gc.Failures) == 0 {
		t.Errorf("gates.failures is empty, want one entry containing %q", wantFailure)
	} else if gc.Failures[0] != wantFailure {
		t.Errorf("gates.failures[0] = %q\nwant                      %q", gc.Failures[0], wantFailure)
	}

	// The sentinel must not exist: the touch on line 23 must not have executed.
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("sentinel file exists — the touch gate executed despite a fragment on an earlier line")
	}
}

// TestBarrierFragment_OddTrailingBackslash asserts that a gate ending with a
// single trailing backslash (an odd count — shell line continuation) is detected
// by the odd-\ rule and reported as an incomplete command. This proves the
// odd-\ supplement matters: sh -n alone passes trailing-\ lines (measured FN).
func TestBarrierFragment_OddTrailingBackslash(t *testing.T) {
	t.Parallel()
	dir, roadmap := fragmentFixtureDir(t, []string{`echo hello \`})
	doc := runFragmentBarrier(t, dir, roadmap)
	gc := gatesCheckFrom(t, doc)

	if gc.Status != "blocked" {
		t.Errorf("gates.status = %q, want \"blocked\"", gc.Status)
	}
	wantFailure := `line 22: incomplete command — each line of the gates block runs as a separate sh -c (rule 5): echo hello \`
	found := false
	for _, f := range gc.Failures {
		if f == wantFailure {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected failure %q not found in gates.failures: %v", wantFailure, gc.Failures)
	}
}

// TestBarrierFragment_EvenTrailingBackslash asserts that a gate ending with an
// even number of backslashes (an escaped backslash — `\\`) is NOT treated as a
// fragment. The gate executes normally and evidence is recorded as
// `<cmd>: exit 0`.
func TestBarrierFragment_EvenTrailingBackslash(t *testing.T) {
	t.Parallel()
	gate := `echo hello \\`
	dir, roadmap := fragmentFixtureDir(t, []string{gate})
	doc := runFragmentBarrier(t, dir, roadmap)
	gc := gatesCheckFrom(t, doc)

	// No fragment failure may be present.
	for _, f := range gc.Failures {
		if strings.Contains(f, "incomplete command") {
			t.Errorf("unexpected fragment failure for even-\\ gate: %q", f)
		}
	}
	// The gate executes and produces evidence (AC5 preserved).
	wantEvidence := gate + ": exit 0"
	found := false
	for _, e := range gc.Evidence {
		if e == wantEvidence {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("evidence %q not found in gates.evidence: %v", wantEvidence, gc.Evidence)
	}
}

// TestBarrierFragment_ValidBlockEvidenceUnchanged asserts that a valid block
// of three gate commands is not affected by the fragment check: each command
// still produces its "<cmd>: exit 0" evidence entry, preserving AC5.
func TestBarrierFragment_ValidBlockEvidenceUnchanged(t *testing.T) {
	t.Parallel()
	gates := []string{"true", "echo gate2", "true"}
	dir, roadmap := fragmentFixtureDir(t, gates)
	doc := runFragmentBarrier(t, dir, roadmap)
	gc := gatesCheckFrom(t, doc)

	if gc.Status != "passed" {
		t.Errorf("gates.status = %q, want \"passed\" (failures: %v)", gc.Status, gc.Failures)
	}
	wantEvidence := []string{"true: exit 0", "echo gate2: exit 0", "true: exit 0"}
	if len(gc.Evidence) != len(wantEvidence) {
		t.Errorf("gates.evidence has %d entries, want %d: %v", len(gc.Evidence), len(wantEvidence), gc.Evidence)
	}
	for i, want := range wantEvidence {
		if i >= len(gc.Evidence) {
			break
		}
		if gc.Evidence[i] != want {
			t.Errorf("gates.evidence[%d] = %q, want %q", i, gc.Evidence[i], want)
		}
	}
}

// TestBarrierFragment_UntrustedRoadmap_ShNotCalled asserts that the fragment
// check (and therefore sh -n) is NOT invoked when the roadmap is untrusted.
// Proof by effect: a fake sh that records -n calls to a marker file is placed
// on PATH; the marker is absent after an untrusted run.
// Contra-arm: the same fixture with --trust-local-gates creates the marker,
// confirming the fake sh intercepts correctly.
func TestBarrierFragment_UntrustedRoadmap_ShNotCalled(t *testing.T) {
	t.Parallel()

	// Build the binary once (cached by barrierBinaryOnce).
	bin := barrierBinary(t)

	// Create a fake sh that records calls with -n to a marker file,
	// then delegates to the real /bin/sh.
	fakeShDir := t.TempDir()
	markerPath := filepath.Join(t.TempDir(), "fake-sh-n-called")
	fakeShContent := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "-n" ]; then
  touch %s
fi
exec /bin/sh "$@"
`, markerPath)
	fakeShPath := filepath.Join(fakeShDir, "sh")
	if err := os.WriteFile(fakeShPath, []byte(fakeShContent), 0755); err != nil {
		t.Fatalf("write fake sh: %v", err)
	}

	// Build env with fake sh first on PATH.
	origPATH := os.Getenv("PATH")
	fakePATH := fakeShDir + ":" + origPATH
	baseEnv := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PATH=") {
			baseEnv = append(baseEnv, kv)
		}
	}
	baseEnv = append(baseEnv, "PATH="+fakePATH)

	// Fixture: a fragment gate that would trigger sh -n if evaluated.
	gate := `n=$(python3 -c "`
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     false,
		mlStatus:      "✅ Concluído",
		criteriaLines: []string{"- [x] criterion met"},
		gateCommands:  []string{gate},
	})

	// ── untrusted run (no --trust-local-gates, not a git repo) ──
	cmdUntrusted := exec.Command(bin, "barrier", "ROADMAP-barrier-fixture", "--wave", "1", "--json")
	cmdUntrusted.Dir = dir
	cmdUntrusted.Env = append([]string(nil), baseEnv...)
	var outUntrusted, errUntrusted strings.Builder
	cmdUntrusted.Stdout = &outUntrusted
	cmdUntrusted.Stderr = &errUntrusted
	_ = cmdUntrusted.Run() // exit 1 expected (not_evaluated → blocked)

	// Parse result.
	var docUntrusted barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(outUntrusted.String())), &docUntrusted); err != nil {
		t.Fatalf("untrusted stdout is not valid JSON: %v\nstdout: %s", err, outUntrusted.String())
	}
	gcUntrusted := gatesCheckFrom(t, docUntrusted)
	if gcUntrusted.Status != "not_evaluated" {
		t.Errorf("untrusted: gates.status = %q, want \"not_evaluated\"", gcUntrusted.Status)
	}
	// The marker must be absent: fake sh was never called with -n.
	if _, err := os.Stat(markerPath); err == nil {
		t.Error("untrusted run: fake-sh marker exists — sh -n was called despite untrusted roadmap")
	}

	// ── contra-arm: trusted run (--trust-local-gates) ──
	// Remove the marker in case the untrusted run left it (it should not).
	_ = os.Remove(markerPath)

	cmdTrusted := exec.Command(bin, "barrier", "ROADMAP-barrier-fixture", "--wave", "1", "--json", "--trust-local-gates")
	cmdTrusted.Dir = dir
	cmdTrusted.Env = append([]string(nil), baseEnv...)
	var outTrusted, errTrusted strings.Builder
	cmdTrusted.Stdout = &outTrusted
	cmdTrusted.Stderr = &errTrusted
	_ = cmdTrusted.Run()

	// The marker must exist: sh -n was called for the fragment gate.
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("trusted contra-arm: fake-sh marker absent — sh -n was NOT called with --trust-local-gates\n"+
			"stdout: %s\nstderr: %s", outTrusted.String(), errTrusted.String())
	}
	// The trusted run must also report blocked (the gate is a fragment).
	var docTrusted barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(outTrusted.String())), &docTrusted); err != nil {
		t.Fatalf("trusted contra-arm stdout is not valid JSON: %v\nstdout: %s", err, outTrusted.String())
	}
	gcTrusted := gatesCheckFrom(t, docTrusted)
	if gcTrusted.Status != "blocked" {
		t.Errorf("trusted contra-arm: gates.status = %q, want \"blocked\"", gcTrusted.Status)
	}
}
