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
	"runtime"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/roadmapdoc"
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
	// Use forward-slash path so the gate `touch <path>` works on Windows
	// under MSYS sh (MSYS accepts C:/... paths; native \ would be mangled).
	sentinelSlash := filepath.ToSlash(sentinel)
	gates := []string{
		`n=$(python3 -c "`,
		"touch " + sentinelSlash,
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

// buildFakeSh compiles a minimal Go program that acts as a fake "sh" binary.
// When called with -n it writes a marker file (path from FAKE_SH_MARKER env var)
// and exits 2 (simulating sh -n detecting a syntax error). Without -n it exits 0.
// The binary is placed in dir as "sh" (or "sh.exe" on Windows) so tests can
// prepend dir to PATH. This replaces the earlier #!/bin/sh script which Windows
// cannot execute.
func buildFakeSh(t *testing.T, dir string) {
	t.Helper()
	// The fake sh records whether -n was passed (proof-by-effect) and always
	// exits 0. Exiting 0 under -n means the product would NOT see a syntax
	// error from the fragment check — so the test is measuring a strictly
	// weaker property than the real sh: the marker proves sh -n was called,
	// not that it did anything useful. That is all this test is for: the
	// "was sh -n called?" proof for the untrusted/trusted split. Fragment
	// detection correctness is covered by the other Fragment_* tests that
	// use the real sh from PATH.
	src := `package main

import (
	"os"
)

func main() {
	marker := os.Getenv("FAKE_SH_MARKER")
	for _, arg := range os.Args[1:] {
		if arg == "-n" {
			if marker != "" {
				_ = os.WriteFile(marker, []byte("called"), 0644)
			}
			// Always exit 0: the fake does not simulate syntax errors.
			// Fragment detection correctness is tested by the other
			// TestBarrierFragment_* tests that use the real sh.
			os.Exit(0)
		}
	}
	// No -n flag: exit 0.
	os.Exit(0)
}
`
	srcPath := filepath.Join(dir, "fakesh_main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatalf("write fake sh source: %v", err)
	}
	binName := "sh"
	if runtime.GOOS == "windows" {
		binName = "sh.exe"
	}
	binPath := filepath.Join(dir, binName)
	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake sh: %v\n%s", err, out)
	}
}

// TestBarrierFragment_UntrustedRoadmap_ShNotCalled asserts that the fragment
// check (and therefore sh -n) is NOT invoked when the roadmap is untrusted.
// Proof by effect: a fake sh binary (Go, portable to Windows) that records -n
// calls to a marker file is placed on PATH; the marker is absent after an
// untrusted run.
// Contra-arm: the same fixture with --trust-local-gates creates the marker,
// confirming the fake sh intercepts correctly.
func TestBarrierFragment_UntrustedRoadmap_ShNotCalled(t *testing.T) {
	t.Parallel()

	// Build the binary once (cached by barrierBinaryOnce).
	bin := barrierBinary(t)

	// Build a fake sh binary (Go, runs on all OSes including Windows).
	fakeShDir := t.TempDir()
	buildFakeSh(t, fakeShDir)
	markerPath := filepath.Join(t.TempDir(), "fake-sh-n-called")

	// Build env with fake sh first on PATH, and FAKE_SH_MARKER pointing to the
	// marker file so the fake sh binary knows where to write it.
	origPATH := os.Getenv("PATH")
	fakePATH := fakeShDir + string(os.PathListSeparator) + origPATH
	baseEnv := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PATH=") && !strings.HasPrefix(kv, "FAKE_SH_MARKER=") {
			baseEnv = append(baseEnv, kv)
		}
	}
	baseEnv = append(baseEnv, "PATH="+fakePATH)
	baseEnv = append(baseEnv, "FAKE_SH_MARKER="+markerPath)

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
	// Note: the fake sh always exits 0, so the trusted run may report
	// "passed" here — that is intentional. The fake sh's job is only to
	// record whether sh -n was invoked; fragment detection correctness is
	// proven by the other TestBarrierFragment_* tests that use the real sh.
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("trusted contra-arm: fake-sh marker absent — sh -n was NOT called with --trust-local-gates\n"+
			"stdout: %s\nstderr: %s", outTrusted.String(), errTrusted.String())
	}
}

// TestBarrierFragment_TransportNoArgvMangling asserts that the gate text is
// delivered to sh via stdin, not argv, so that a gate containing an unquoted `"`
// (e.g. `esperado="scaffold.go`) is detected as a fragment on every OS — including
// Windows, where Go's EscapeArg + MSYS reparse would silently convert
// `esperado="scaffold.go` to `esperado=\scaffold.go` (a valid assignment, exit 0)
// if the text were passed as an argv argument.
//
// What this test asserts: `esperado="scaffold.go` is blocked with "incomplete command"
// and a sentinel touch on the second line does not execute.
//
// On macOS/Linux the test passes because the stdin transport is used.
// On Windows (CI job windows-full-suites) it proves that the argv mangling path is
// closed — the gate text reaches sh byte-identical via stdin.
func TestBarrierFragment_TransportNoArgvMangling(t *testing.T) {
	t.Parallel()
	sentinel := filepath.Join(t.TempDir(), "transport-sentinel")
	// Use forward-slash path so the gate `touch <path>` works on Windows
	// under MSYS sh (MSYS accepts C:/... paths; native \ would be mangled).
	sentinelSlash := filepath.ToSlash(sentinel)
	gates := []string{
		`esperado="scaffold.go`,
		"touch " + sentinelSlash,
	}
	dir, roadmap := fragmentFixtureDir(t, gates)
	doc := runFragmentBarrier(t, dir, roadmap)
	gc := gatesCheckFrom(t, doc)

	// The gates block must be blocked: the fragment was detected.
	if gc.Status != "blocked" {
		t.Errorf("gates.status = %q, want \"blocked\"", gc.Status)
	}

	// At least one failure must name the fragment with "incomplete command".
	found := false
	for _, f := range gc.Failures {
		if strings.Contains(f, "incomplete command") && strings.Contains(f, `esperado="scaffold.go`) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no failure naming the fragment found in gates.failures: %v", gc.Failures)
	}

	// The sentinel must not exist: the touch on the second line must not have executed.
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("sentinel exists — the touch gate executed despite a fragment on line 1")
	}
}

// TestBarrierFragment_TransportMultiLineRunGate asserts that runGateCommand
// rejects a gate text containing an embedded newline, returning exit code 2
// without spawning sh. The guard fires before any sh invocation; the invariant
// is that ParseGatesLines delivers one line per gate (rule 5). Measured by
// Lourival in PR #495: with two lines, a gate that reads stdin (`read x`)
// consumes the next script line, producing `lido=[SEGUNDA_LINHA…]`.
func TestBarrierFragment_TransportMultiLineRunGate(t *testing.T) {
	t.Parallel()
	sentinel := filepath.Join(t.TempDir(), "multiline-rungatecommand-sentinel")
	sentinelSlash := filepath.ToSlash(sentinel)
	// Second line is a touch; it must not execute.
	cmd := "read x; echo \"lido=[$x]\"\ntouch " + sentinelSlash
	code, spawnFailed := runGateCommand(cmd, nil)
	if code == 0 {
		t.Errorf("runGateCommand: exit code = 0, want non-zero for multi-line gate")
	}
	if spawnFailed {
		t.Errorf("runGateCommand: spawnFailed = true, want false — guard must fire before sh spawn")
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("sentinel exists — sh was spawned and executed the second line despite the multi-line guard")
	}
}

// TestBarrierFragment_TransportMultiLineCheckFragments asserts that
// checkGateFragments rejects a GateCmd whose Text contains an embedded newline,
// reporting "blocked" with the pinned message "gate text spans multiple lines".
// The second line would be a sentinel touch; the sentinel being absent proves
// sh was never invoked. Same invariant as TransportMultiLineRunGate.
func TestBarrierFragment_TransportMultiLineCheckFragments(t *testing.T) {
	t.Parallel()
	sentinel := filepath.Join(t.TempDir(), "multiline-checkfragments-sentinel")
	sentinelSlash := filepath.ToSlash(sentinel)
	gcmds := []roadmapdoc.GateCmd{
		{Text: "read x; echo \"lido=[$x]\"\ntouch " + sentinelSlash, Line: 42},
	}
	status, failures := checkGateFragments(gcmds)
	if status != "blocked" {
		t.Errorf("checkGateFragments: status = %q, want \"blocked\"", status)
	}
	want := "line 42: gate text spans multiple lines — the transport reads one line per gate (rule 5)"
	found := false
	for _, f := range failures {
		if f == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected failure %q not found in failures: %v", want, failures)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("sentinel exists — sh was invoked despite the multi-line guard in checkGateFragments")
	}
}

// TestBarrierFragment_TransportSingleLineReadContra asserts that a single-line
// gate containing `read x; test -z "$x"` passes through runGateCommand with
// exit 0, proving that the single-line invariant (rule 5) still permits
// stdin-reading gates. sh reads the script from its own stdin; `read x` then
// reads from the same (now-exhausted) stdin, x gets the empty string, and
// `test -z ""` exits 0.
func TestBarrierFragment_TransportSingleLineReadContra(t *testing.T) {
	t.Parallel()
	code, spawnFailed := runGateCommand(`read x; test -z "$x"`, nil)
	if spawnFailed {
		t.Skip("sh not found in PATH — skipping single-line read contra-arm")
	}
	if code != 0 {
		t.Errorf("runGateCommand: exit code = %d, want 0 for single-line read gate", code)
	}
}
