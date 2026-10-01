package roadmapdoc

// gates_lines_test.go — unit tests for ParseGatesLines and the F1 fix (REQ #491, ML-1A).
//
// AC11 reconciliation (one sentence per test asserting which conclusion it affirms):
//
//   TestParseGatesLines_LineNumbers
//     Affirms: ParseGatesLines returns the correct 1-based document line number for each
//     gate command, while ignoring empty lines and comment lines that begin with '#'.
//
//   TestParseGatesLines_F1_MaskedMarkerNotGate
//     Affirms: a "**Gates da wave:**" marker that falls inside a 4-backtick example fence is
//     silently skipped (F1), so no gate is generated from that marker.
//
//   TestParseGatesLines_F1_RealMarkerAfterMasked
//     Affirms: when a masked marker (inside a 4-backtick fence) precedes a real marker in
//     the same wave, the real marker's commands are returned with correct line numbers —
//     confirming that the scan continues past masked markers rather than stopping.
//
//   TestParseGatesLines_RealFile_Wave2Returns_Empty
//     Affirms: ParseGates on Wave 2 of docs/roadmaps/done/ROADMAP-2026-08-22-wave-0-…
//     now returns [] (the marker at line 226 is inside a 4-backtick fence and is masked),
//     matching the measured before/after delta reported in the ML-1A handoff.

import (
	"os"
	"path/filepath"
	"testing"
)

// ── TestParseGatesLines_LineNumbers ──────────────────────────────────────────

func TestParseGatesLines_LineNumbers(t *testing.T) {
	// Inline document: Wave 0 spanning lines 1-9 (1-based).
	//
	//  line 1: ## Wave 0 — Test Gate Lines
	//  line 2: (empty)
	//  line 3: **Gates da wave:**
	//  line 4: ```bash
	//  line 5: # this is a comment — must be excluded
	//  line 6: echo "cmd1"
	//  line 7: (empty line — must be excluded)
	//  line 8: echo "cmd2"
	//  line 9: ```
	doc := "## Wave 0 — Test Gate Lines\n" +
		"\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"# this is a comment — must be excluded\n" +
		"echo \"cmd1\"\n" +
		"\n" +
		"echo \"cmd2\"\n" +
		"```\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gcmds) != 2 {
		t.Fatalf("expected 2 GateCmds, got %d: %v", len(gcmds), gcmds)
	}

	// cmd1 is at 0-indexed line 5, so 1-based line 6
	if gcmds[0].Line != 6 {
		t.Errorf("gcmds[0].Line = %d, want 6", gcmds[0].Line)
	}
	if gcmds[0].Text != `echo "cmd1"` {
		t.Errorf("gcmds[0].Text = %q, want %q", gcmds[0].Text, `echo "cmd1"`)
	}

	// cmd2 is at 0-indexed line 7, so 1-based line 8
	if gcmds[1].Line != 8 {
		t.Errorf("gcmds[1].Line = %d, want 8", gcmds[1].Line)
	}
	if gcmds[1].Text != `echo "cmd2"` {
		t.Errorf("gcmds[1].Text = %q, want %q", gcmds[1].Text, `echo "cmd2"`)
	}
}

// ── TestParseGatesLines_F1_MaskedMarkerNotGate ───────────────────────────────

func TestParseGatesLines_F1_MaskedMarkerNotGate(t *testing.T) {
	// Wave with only one "**Gates da wave:**", but inside a 4-backtick example fence.
	// The wave should return zero gates (F1 fix).
	//
	//  line 1: ## Wave 1 — F1 Masked
	//  line 2: (empty)
	//  line 3: ````
	//  line 4: Example output:
	//  line 5: **Gates da wave:**   ← inside 4-backtick fence, must be skipped
	//  line 6: ```bash
	//  line 7: exit 1  # inside the example
	//  line 8: ```
	//  line 9: ````
	doc := "## Wave 1 — F1 Masked\n" +
		"\n" +
		"````\n" +
		"Example output:\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"exit 1  # inside the example\n" +
		"```\n" +
		"````\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	// The marker at line 5 (0-indexed: 4) must be masked.
	if !fenced[4] {
		t.Errorf("expected line 5 (0-indexed 4) to be masked by FenceMask, but it is not")
	}
	if !GatesHeaderRe.MatchString(lines[4]) {
		t.Errorf("sanity: GatesHeaderRe should match line 5, but does not")
	}

	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gcmds) != 0 {
		t.Errorf("expected 0 GateCmds (masked marker), got %d: %v", len(gcmds), gcmds)
	}
}

// ── TestParseGatesLines_F1_RealMarkerAfterMasked ─────────────────────────────

func TestParseGatesLines_F1_RealMarkerAfterMasked(t *testing.T) {
	// Wave with a masked marker first (inside a 4-backtick fence) followed by
	// a real marker outside the fence. The real marker's commands must be returned.
	//
	//  line  1: ## Wave 2 — F1 Two Markers
	//  line  2: (empty)
	//  line  3: ````
	//  line  4: **Gates da wave:**   ← inside 4-backtick fence, skipped
	//  line  5: ```bash
	//  line  6: exit 1
	//  line  7: ```
	//  line  8: ````
	//  line  9: (empty)
	//  line 10: **Gates da wave:**   ← real marker, outside fence
	//  line 11: ```bash
	//  line 12: echo "real gate"
	//  line 13: ```
	doc := "## Wave 2 — F1 Two Markers\n" +
		"\n" +
		"````\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"exit 1\n" +
		"```\n" +
		"````\n" +
		"\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"echo \"real gate\"\n" +
		"```\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gcmds) != 1 {
		t.Fatalf("expected 1 GateCmd (real marker after masked), got %d: %v", len(gcmds), gcmds)
	}
	// "echo \"real gate\"" is at 0-indexed line 11, so 1-based line 12.
	if gcmds[0].Line != 12 {
		t.Errorf("gcmds[0].Line = %d, want 12", gcmds[0].Line)
	}
	if gcmds[0].Text != `echo "real gate"` {
		t.Errorf("gcmds[0].Text = %q, want %q", gcmds[0].Text, `echo "real gate"`)
	}
}

// ── TestParseGatesLines_RealFile_Wave2Returns_Empty ──────────────────────────

// TestParseGatesLines_RealFile_Wave2Returns_Empty reads the real acervo file
// docs/roadmaps/done/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md
// and asserts that ParseGates on Wave 2 now returns [] (previously it returned
// ["exit 1  # placeholder gate fails closed until ML-0A replaces it"] because
// the marker at line 226 was inside a 4-backtick fence but ParseGates ignored FenceMask).
func TestParseGatesLines_RealFile_Wave2Returns_Empty(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "roadmaps", "done",
		"ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("real-file test skipped: %v", err)
	}

	lines := SplitRoadmapLines(string(data))
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)

	var wave2 *WaveBlock
	for i := range waves {
		if waves[i].Label == "2" {
			wave2 = &waves[i]
			break
		}
	}
	if wave2 == nil {
		t.Fatal("Wave 2 not found in the real file")
	}

	// Sanity: the heading at waveStart must be "## Wave 2 — Gate" so we are
	// sure we have the right wave and not "## Wave 2-bis".
	heading := lines[wave2.Start]
	if heading != "## Wave 2 — Gate" {
		t.Fatalf("unexpected heading at Wave 2 start: %q", heading)
	}

	// Sanity: line 226 (1-based) = index 225 must match GatesHeaderRe and be masked.
	const markerIdx = 225 // 0-indexed
	if markerIdx >= len(lines) {
		t.Fatalf("file has fewer than %d lines", markerIdx+1)
	}
	if !GatesHeaderRe.MatchString(lines[markerIdx]) {
		t.Errorf("sanity: GatesHeaderRe does not match line 226 (%q)", lines[markerIdx])
	}
	if !fenced[markerIdx] {
		t.Errorf("sanity: line 226 (0-indexed %d) should be inside FenceMask, but is not", markerIdx)
	}

	// The key assertion: ParseGates must return [] now (not the placeholder command).
	cmds, parseErr := ParseGates(lines, wave2.Start, wave2.End)
	if parseErr != nil {
		t.Fatalf("ParseGates error: %v", parseErr)
	}
	if len(cmds) != 0 {
		t.Errorf("expected ParseGates to return [] for Wave 2 (marker inside fence), got %v", cmds)
	}
}
