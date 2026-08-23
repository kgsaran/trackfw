package commands

// barrier_test.go — unit tests for the string-level parser and per-check evaluation
// logic in barrier.go, additional to the cross-runtime contract fixed in
// barrier_contract_test.go (which drives the real compiled binary end-to-end).
// These tests call the unexported parsing helpers directly and do not build a binary.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseWaves_SingleWave(t *testing.T) {
	content := "# Roadmap\n\n## Wave 1 — Foo\nbody line\n\n### ML-1A — x\n**Status:** ✅\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if waves[0].label != "1" {
		t.Fatalf("expected wave label \"1\", got %q", waves[0].label)
	}
}

func TestParseWaves_MultipleWavesEndAtNextH2(t *testing.T) {
	content := strings.Join([]string{
		"# Roadmap",
		"",
		"## Wave 1 — Foo",
		"content of wave 1",
		"",
		"## Wave 2 — Bar",
		"content of wave 2",
	}, "\n")
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if waves[0].label != "1" || waves[1].label != "2" {
		t.Fatalf("unexpected wave labels: %+v", waves)
	}
	// Wave 1 block must end exactly where the "## Wave 2" heading starts.
	if lines[waves[0].end] != "## Wave 2 — Bar" {
		t.Fatalf("expected wave 1 to end at 'Wave 2' heading, got %q", lines[waves[0].end])
	}
}

func TestParseWaves_MalformedLabelIsUsageError(t *testing.T) {
	content := "## Wave x — Foo\nbody\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr == nil {
		t.Fatalf("expected usage error for malformed wave label, got waves=%+v", waves)
	}
	if !strings.Contains(uerr.Error(), "line 1") {
		t.Fatalf("expected error to name line 1, got: %s", uerr.Error())
	}
	if !strings.Contains(uerr.Error(), "wave label") {
		t.Fatalf("expected error to say \"wave label\", got: %s", uerr.Error())
	}
}

// TestParseWaves_BisSuffix verifies that a valid wave label with a suffix is accepted.
func TestParseWaves_BisSuffix(t *testing.T) {
	content := "## Wave 2-bis — Corrective\nbody\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error for valid label 2-bis: %v", uerr)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if waves[0].label != "2-bis" {
		t.Fatalf("expected label \"2-bis\", got %q", waves[0].label)
	}
}

// TestParseWaves_LabelIdentityDistinct verifies that "2" and "2-bis" are distinct identities:
// a document with both headings must produce two separate waveBlocks.
func TestParseWaves_LabelIdentityDistinct(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 2 — Base Wave",
		"body of wave 2",
		"",
		"## Wave 2-bis — Corrective Wave",
		"body of wave 2-bis",
	}, "\n")
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves (distinct labels), got %d: %+v", len(waves), waves)
	}
	if waves[0].label != "2" {
		t.Fatalf("expected first label \"2\", got %q", waves[0].label)
	}
	if waves[1].label != "2-bis" {
		t.Fatalf("expected second label \"2-bis\", got %q", waves[1].label)
	}
	// Wave 2 block must end exactly where "## Wave 2-bis" starts.
	if lines[waves[0].end] != "## Wave 2-bis — Corrective Wave" {
		t.Fatalf("expected wave 2 to end at 'Wave 2-bis' heading, got %q", lines[waves[0].end])
	}
}

func TestParseMLs_MultipleMLsInWave(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"### ML-1A — First",
		"**Status:** ✅",
		"### ML-1B — Second",
		"**Status:** ⬜ Pendente",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	if len(mls) != 2 {
		t.Fatalf("expected 2 MLs, got %d", len(mls))
	}
	if mls[0].id != "ML-1A" || mls[1].id != "ML-1B" {
		t.Fatalf("unexpected ML ids: %+v", mls)
	}
}

func TestMLStatusMarker_MissingLine(t *testing.T) {
	content := "### ML-1A — Foo\nno status line here\n"
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML, got %d", len(mls))
	}
	_, found := mlStatusMarker(lines, mls[0])
	if found {
		t.Fatal("expected found=false when no **Status:** line is present")
	}
}

func TestAcceptanceEvaluate_AllMet(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] build passes",
		"- [x] tests pass",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	met, unmet, hasBlock := acceptanceEvaluate(lines, mls[0])
	if !hasBlock {
		t.Fatal("expected hasBlock=true")
	}
	if unmet != 0 {
		t.Fatalf("expected 0 unmet, got %d", unmet)
	}
	if met != 2 {
		t.Fatalf("expected 2 met, got %d", met)
	}
}

func TestAcceptanceEvaluate_EmptyBlockIsNotVacuouslyPassed(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"**Files affected:**",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	_, _, hasBlock := acceptanceEvaluate(lines, mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false for an empty acceptance block (anti-vacuity)")
	}
}

func TestAcceptanceEvaluate_NoHeaderAtAll(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	_, _, hasBlock := acceptanceEvaluate(lines, mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false when no **Critérios de aceite:** header exists")
	}
}

func TestParseGates_NoBlockYieldsEmptyNonNilSlice(t *testing.T) {
	content := "## Wave 1 — Foo\nno gates here\n"
	lines := strings.Split(content, "\n")
	cmds, uerr := parseGates(lines, 0, len(lines))
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if cmds == nil {
		t.Fatal("expected non-nil empty slice for a wave with no gates block")
	}
	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands, got %v", cmds)
	}
}

func TestParseGates_ParsesCommandsIgnoringBlankAndComment(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"```bash",
		"go build ./...",
		"",
		"# a comment",
		"go test ./...",
		"```",
	}, "\n")
	lines := strings.Split(content, "\n")
	cmds, uerr := parseGates(lines, 0, len(lines))
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	want := []string{"go build ./...", "go test ./..."}
	if len(cmds) != len(want) {
		t.Fatalf("expected %v, got %v", want, cmds)
	}
	for i := range want {
		if cmds[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, cmds)
		}
	}
}

func TestParseGates_UnterminatedFenceIsUsageError(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"```bash",
		"go build ./...",
	}, "\n")
	lines := strings.Split(content, "\n")
	_, uerr := parseGates(lines, 0, len(lines))
	if uerr == nil {
		t.Fatal("expected usage error for unterminated fence")
	}
}

func TestParseGates_MissingFenceRightAfterHeaderIsUsageError(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"go build ./...",
	}, "\n")
	lines := strings.Split(content, "\n")
	_, uerr := parseGates(lines, 0, len(lines))
	if uerr == nil {
		t.Fatal("expected usage error when no ```bash fence immediately follows the gates header")
	}
}

// TestBarrierCheck_JSONKeyOrderMatchesCliParityContract asserts the literal key
// order of a serialized barrierCheck — not just presence of keys. This is the
// regression coverage for the Go-vs-Node/Python "gates" check key-order
// divergence described in vault/notes/barrier-gates-check-key-order-divergence-go-2026-07-29.md:
// encoding/json serializes struct fields in declaration order, so reordering the
// barrierCheck struct silently changes the emitted JSON. docs/cli-parity.md pins
// "commands" as the third key (right after "status") for the gates check.
func TestBarrierCheck_JSONKeyOrderMatchesCliParityContract(t *testing.T) {
	cmds := []string{"go build ./..."}
	check := barrierCheck{
		Name:     "gates",
		Status:   "passed",
		Commands: &cmds,
		Evidence: []string{"go build ./...: exit 0"},
		Failures: []string{},
	}
	out, err := json.Marshal(check)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	wantOrder := []string{`"name"`, `"status"`, `"commands"`, `"evidence"`, `"failures"`}
	var positions []int
	for _, key := range wantOrder {
		pos := strings.Index(string(out), key)
		if pos < 0 {
			t.Fatalf("expected key %s to be present in %s", key, out)
		}
		positions = append(positions, pos)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] >= positions[i] {
			t.Fatalf("expected key order %v, got JSON with wrong order: %s", wantOrder, out)
		}
	}

	// A check without gates (Commands == nil) must omit "commands" entirely —
	// never emit it as null.
	nonGatesCheck := barrierCheck{
		Name:     "mls_complete",
		Status:   "passed",
		Evidence: []string{},
		Failures: []string{},
	}
	nonGatesOut, err := json.Marshal(nonGatesCheck)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(nonGatesOut), "commands") {
		t.Fatalf("expected non-gates check to omit \"commands\" entirely, got: %s", nonGatesOut)
	}
}

func TestRunGateCommand_ExitCodes(t *testing.T) {
	if code := runGateCommand("true"); code != 0 {
		t.Fatalf("expected exit 0 for 'true', got %d", code)
	}
	if code := runGateCommand("false"); code != 1 {
		t.Fatalf("expected exit 1 for 'false', got %d", code)
	}
	if code := runGateCommand("exit 7"); code != 7 {
		t.Fatalf("expected exit 7 for 'exit 7', got %d", code)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Wave label ordering tests (compareWaveLabels, splitWaveLabel).
// ────────────────────────────────────────────────────────────────────────────

func TestWaveLabelOrdering(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// Numeric ordering (the discriminating case: 10 > 2, not lexicographic)
		{"10", "2", 1},
		{"2", "10", -1},
		{"3", "3", 0},
		// No-suffix before with-suffix on same integer
		{"2", "2-bis", -1},
		{"2-bis", "2", 1},
		// Lexicographic between suffixes
		{"2-bis", "2-hotfix", -1},
		{"2-hotfix", "2-bis", 1},
		{"2-bis", "2-bis", 0},
		// Full contract example: 2 < 2-bis < 2-hotfix < 3
		{"2", "3", -1},
		{"2-hotfix", "3", -1},
	}
	for _, tc := range cases {
		got := compareWaveLabels(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("compareWaveLabels(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSplitWaveLabel(t *testing.T) {
	cases := []struct {
		label   string
		wantInt int
		wantSuf string
	}{
		{"1", 1, ""},
		{"2", 2, ""},
		{"2-bis", 2, "bis"},
		{"2-hotfix", 2, "hotfix"},
		{"10-a2", 10, "a2"},
	}
	for _, tc := range cases {
		gotInt, gotSuf := splitWaveLabel(tc.label)
		if gotInt != tc.wantInt || gotSuf != tc.wantSuf {
			t.Errorf("splitWaveLabel(%q) = (%d, %q), want (%d, %q)",
				tc.label, gotInt, gotSuf, tc.wantInt, tc.wantSuf)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Regression: malformed heading aborts ENTIRE document — ADR decision 16.
// This end-to-end test is the evidence that prevents anyone from "fixing"
// the abort into a scoped skip for the requested wave.
// ────────────────────────────────────────────────────────────────────────────

// TestParseWaves_MalformedHeadingAbortsEntireDocument_Regression proves that a
// malformed wave heading (`## Wave X — ...`) aborts the whole document even when
// the requested wave (`--wave 1`) is fully green and appears before the bad heading.
// This is the regression guard for ADR decision 16.
func TestParseWaves_MalformedHeadingAbortsEntireDocument_Regression(t *testing.T) {
	// Build a multi-wave roadmap where Wave 1 is fully green and Wave X is malformed.
	// Line positions are fixed so we can assert the exact pinned error message.
	content := strings.Join([]string{
		"# Roadmap: Malformed Wave Regression",             // line 1
		"",                                                  // line 2
		"REQ: REQ-regression-test",                         // line 3
		"",                                                  // line 4
		"## Acceptance Criteria",                            // line 5
		"- [x] fixture criterion",                           // line 6
		"",                                                  // line 7
		"## Wave 1 — Green Wave",                            // line 8
		"> Dependências: nenhuma",                           // line 9
		"",                                                  // line 10
		"### ML-1A — Green ML",                              // line 11
		"**Status:** ✅ Concluído",                          // line 12
		"**Critérios de aceite:**",                          // line 13
		"- [x] criterion met",                               // line 14
		"",                                                  // line 15
		"## Wave X — Malformed Label",                       // line 16 ← bad heading
		"> body of malformed wave",                          // line 17
		"",                                                  // line 18
		"### ML-X1 — Irrelevant ML",                         // line 19
		"**Status:** ✅",                                    // line 20
		"**Critérios de aceite:**",                          // line 21
		"- [x] whatever",                                    // line 22
	}, "\n")

	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr == nil {
		t.Fatalf("expected parse error for malformed heading, got %d waves", len(waves))
	}

	// The error must name the line AND carry the pinned fragment.
	wantFrag := `"X" is not a valid wave label`
	if !strings.Contains(uerr.Error(), "line 16") {
		t.Errorf("expected error to name line 16, got: %s", uerr.Error())
	}
	if !strings.Contains(uerr.Error(), wantFrag) {
		t.Errorf("expected error to contain %q, got: %s", wantFrag, uerr.Error())
	}

	// End-to-end: even requesting --wave 1 (the green wave) must exit 2, not 0.
	// This is the regression guard that prevents "fix" the abort into a scoped skip.
	dir := t.TempDir()
	for _, d := range []string{"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	roadmapPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-malformed-regression.md")
	if err := os.WriteFile(roadmapPath, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-malformed-regression", "--wave", "1")
	if code != 2 {
		t.Fatalf("expected exit 2 (abort on malformed heading), got %d\nstdout: %s\nstderr: %s",
			code, stdout, stderr)
	}
	// Assert pinned stderr byte-for-byte.
	wantStderr := "trackfw barrier: malformed wave heading at line 16: \"X\" is not a valid wave label\n"
	if stderr != wantStderr {
		t.Fatalf("stderr mismatch:\nwant: %q\ngot:  %q", wantStderr, stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Regression tests — ML-2D. These two cases had zero coverage before ML-2D,
// which is exactly why the cross-runtime divergence (Go vs Node.js vs Python)
// slipped through the per-runtime suites in the Wave 2 barrier run.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierRegression_WaveWithNoMLProducesPinnedFailureMessage(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"docs/roadmaps/wip", "docs/req", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	content := "# Roadmap: No ML\n\nREQ: REQ-x\n\n" +
		"## Acceptance Criteria\n- [x] fixture\n\n" +
		"## Wave 1 — Sem MLs\n> Dependências: nenhuma\n\n" +
		"Some prose, no ML heading at all.\n"
	roadmapPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-no-ml.md")
	if err := os.WriteFile(roadmapPath, []byte(content), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-no-ml", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1 (blocked), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	var mlsCheck *barrierCheckDoc
	for i := range doc.Checks {
		if doc.Checks[i].Name == "mls_complete" {
			mlsCheck = &doc.Checks[i]
		}
	}
	if mlsCheck == nil {
		t.Fatalf("mls_complete check not found in: %+v", doc.Checks)
	}
	want := []string{"wave 1: no ML found"}
	if len(mlsCheck.Failures) != 1 || mlsCheck.Failures[0] != want[0] {
		t.Fatalf("expected mls_complete failures %v, got %v", want, mlsCheck.Failures)
	}
}

func TestBarrierRegression_Exit2MessagesArePinnedLiterally(t *testing.T) {
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
	})

	_, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "99", "--json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d, stderr=%s", code, stderr)
	}
	wantWave := "trackfw barrier: wave 99 not found in roadmap \"ROADMAP-barrier-fixture.md\"\n"
	if stderr != wantWave {
		t.Fatalf("wave-not-found message mismatch:\nwant: %q\ngot:  %q", wantWave, stderr)
	}

	_, stderr2, code2 := runBarrierCLI(t, dir, "ROADMAP-nao-existe", "--wave", "1", "--json")
	if code2 != 2 {
		t.Fatalf("expected exit 2, got %d, stderr=%s", code2, stderr2)
	}
	wantRoadmap := "trackfw barrier: roadmap \"ROADMAP-nao-existe\" not found in wip/ nor done/ under docs/roadmaps\n"
	if stderr2 != wantRoadmap {
		t.Fatalf("roadmap-not-found message mismatch:\nwant: %q\ngot:  %q", wantRoadmap, stderr2)
	}
}

// TestWaveLabelGrammar_ValidAndInvalid is a table test of the full contract table
// from docs/cli-parity.md §wave-label-grammar (pinned in ML-3A, extended in
// ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness ML-1A). It exercises
// the composite predicate used in the heading pre-pass: waveLabelRe (regex) +
// int≥0 guard in parseWaves. "0" is now valid (the Wave 0 threat-model
// convention) — it used to be the only label that passed the regex but failed
// the (then int≥1) guard; parseWaves remains the correct surface to test it on.
func TestWaveLabelGrammar_ValidAndInvalid(t *testing.T) {
	valid := []string{"0", "1", "2", "2-bis", "2-hotfix", "10-a2"}
	invalid := []string{"X", "2-BIS", "-bis", "2-", "2-bis-ter"}

	for _, lbl := range valid {
		lbl := lbl
		t.Run("valid/"+lbl, func(t *testing.T) {
			if !waveLabelRe.MatchString(lbl) {
				t.Errorf("waveLabelRe: expected %q to match (valid per contract), but it did not", lbl)
			}
			// Composite check: heading pre-pass must also accept it.
			content := "## Wave " + lbl + " — Test Heading\nbody\n"
			lines := strings.Split(content, "\n")
			_, uerr := parseWaves(lines)
			if uerr != nil {
				t.Errorf("parseWaves: expected no error for valid label %q, got: %v", lbl, uerr)
			}
		})
	}

	for _, lbl := range invalid {
		lbl := lbl
		t.Run("invalid/"+lbl, func(t *testing.T) {
			// Use parseWaves (not waveLabelRe alone) to exercise the full composite
			// predicate.
			content := "## Wave " + lbl + " — Bad Heading\nbody\n"
			lines := strings.Split(content, "\n")
			_, uerr := parseWaves(lines)
			if uerr == nil {
				t.Errorf("parseWaves: expected usage error for invalid label %q, got nil", lbl)
			}
		})
	}
}

// TestBarrierRegression_FourthExitTwoMessage proves the fourth pinned exit-2
// message (cli-parity.md §four-pinned-exit-2-messages) is emitted byte-for-byte
// when --wave receives a syntactically invalid label. This is the message class
// that distinguished from the third message (malformed heading) — same exit code,
// different trigger surface (argument, not document). Node.js had complete coverage
// of this path before ML-3A; this test closes the gap in Go.
func TestBarrierRegression_FourthExitTwoMessage(t *testing.T) {
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
	})

	_, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "2-BIS")
	if code != 2 {
		t.Fatalf("expected exit 2 for invalid --wave label, got %d; stderr=%s", code, stderr)
	}
	want := "trackfw barrier: invalid --wave \"2-BIS\" — not a valid wave label\n"
	if stderr != want {
		t.Fatalf("fourth pinned message mismatch:\nwant: %q\ngot:  %q", want, stderr)
	}
}
