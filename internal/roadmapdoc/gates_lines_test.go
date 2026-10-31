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
//
// ── New tests (ML-3C, REQ-2026-10-31) — AC11 ──────────────────────────────────
//
//   TestParseGatesLines_HTMLComment_MarkerNotGate
//     Affirms: a "**Gates da wave:**" marker whose column 0 is inside an HTML comment block
//     (<!-- … -->) is silently skipped (AC11), so no gate is generated from that block.
//
//   TestParseGatesLines_HTMLComment_RealMarkerAfterCommented
//     Affirms: when a commented-out marker precedes a real marker outside the comment in
//     the same wave, only the real marker's commands are returned — the scan continues past
//     the commented marker rather than stopping.
//
// ── New tests (ML-3C ação 3, REQ-2026-10-31) — AC11b inline ──────────────────
//
//   TestStripInlineHTMLComments
//     Affirms: StripInlineHTMLComments removes each complete "<!-- … -->" span from a
//     line and truncates at an unclosed "<!--", returning (stripped, endsOpen) — so
//     backtick tokens inside comments are invisible to extractTokens while tokens outside
//     are preserved.

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

// ── TestParseGatesLines_HTMLComment_MarkerNotGate ─────────────────────────────

// TestParseGatesLines_HTMLComment_MarkerNotGate asserts that a "**Gates da wave:**"
// marker whose column 0 is inside an HTML comment block (<!-- … -->) is silently
// skipped (AC11, ML-3C, REQ-2026-10-31), producing zero gates.
//
// Contra-braço: without htmlCommentMask in ParseGatesLines the marker inside the
// comment would be treated as a real gate declaration; the exit 1 command would be
// collected and the test would fail because len(gcmds) != 0.
//
// Reconciliation (Regra Dura): a "**Gates da wave:**" marker inside <!-- --> does NOT
// produce a gate — the block is invisible to ParseGatesLines (AC11).
func TestParseGatesLines_HTMLComment_MarkerNotGate(t *testing.T) {
	// Wave with only one "**Gates da wave:**", but inside an HTML comment block.
	// The wave should return zero gates.
	//
	//  line 1: ## Wave 3 — HTML Comment
	//  line 2: (empty)
	//  line 3: <!--
	//  line 4: **Gates da wave:**   ← column 0 inside comment, must be skipped
	//  line 5: ```bash
	//  line 6: exit 1
	//  line 7: ```
	//  line 8: -->
	doc := "## Wave 3 — HTML Comment\n" +
		"\n" +
		"<!--\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"exit 1\n" +
		"```\n" +
		"-->\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	// Sanity: GatesHeaderRe matches line 4 (0-indexed: 3).
	if !GatesHeaderRe.MatchString(lines[3]) {
		t.Errorf("sanity: GatesHeaderRe should match line 4 (0-indexed 3), but does not")
	}
	// Sanity: the header line is NOT fenced (it's inside a comment, not a code fence).
	if fenced[3] {
		t.Errorf("sanity: line 4 (0-indexed 3) should NOT be inside FenceMask (it is in HTML comment, not fence)")
	}
	// Sanity: the header line IS masked by htmlCommentMask.
	htmlMask := HTMLCommentMask(lines, fenced)
	if !htmlMask[3] {
		t.Errorf("sanity: line 4 (0-indexed 3) should be masked by HTMLCommentMask (inside <!-- -->)")
	}

	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gcmds) != 0 {
		t.Errorf("expected 0 GateCmds (marker inside HTML comment), got %d: %v", len(gcmds), gcmds)
	}
}

// ── TestParseGatesLines_HTMLComment_RealMarkerAfterCommented ──────────────────

// TestParseGatesLines_HTMLComment_RealMarkerAfterCommented asserts that when a
// commented-out "**Gates da wave:**" marker precedes a real marker outside the
// comment in the same wave, only the real marker's commands are returned.
//
// Contra-braço: without htmlCommentMask in ParseGatesLines the first (commented)
// marker would be treated as real; ParseGatesLines returns after the first match,
// so the real marker's commands would be missed. The test fails because it expects
// exactly one command from the real marker.
//
// Reconciliation (Regra Dura): the scan continues past a commented marker; the real
// declaration after the comment is found and its commands returned (AC11).
func TestParseGatesLines_HTMLComment_RealMarkerAfterCommented(t *testing.T) {
	// Wave with a commented marker first (inside HTML comment) followed by a real marker.
	//
	//  line 1: ## Wave 4 — HTML Comment Two Markers
	//  line 2: (empty)
	//  line 3: <!--
	//  line 4: **Gates da wave:**   ← inside HTML comment, skipped
	//  line 5: ```bash
	//  line 6: exit 1
	//  line 7: ```
	//  line 8: -->
	//  line 9: (empty)
	//  line 10: **Gates da wave:**   ← real marker, outside comment
	//  line 11: ```bash
	//  line 12: echo "real gate"
	//  line 13: ```
	doc := "## Wave 4 — HTML Comment Two Markers\n" +
		"\n" +
		"<!--\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"exit 1\n" +
		"```\n" +
		"-->\n" +
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
		t.Fatalf("expected 1 GateCmd (real marker after commented), got %d: %v", len(gcmds), gcmds)
	}
	// "echo \"real gate\"" is at 0-indexed line 11, so 1-based line 12.
	if gcmds[0].Line != 12 {
		t.Errorf("gcmds[0].Line = %d, want 12", gcmds[0].Line)
	}
	if gcmds[0].Text != `echo "real gate"` {
		t.Errorf("gcmds[0].Text = %q, want %q", gcmds[0].Text, `echo "real gate"`)
	}
}

// ── TestStripInlineHTMLComments ───────────────────────────────────────────────

// TestStripInlineHTMLComments asserts that StripInlineHTMLComments removes
// complete "<!-- … -->" spans (replacing each with a space) and truncates at an
// unclosed "<!--", returning endsOpen=true in the latter case.
//
// Contra-braço: without the function, backtick-quoted tokens inside inline
// comments would be visible to extractTokens and appear in Files affected scope.
//
// Reconciliation (Regra Dura): StripInlineHTMLComments removes inline comment
// regions from a line so that tokens inside them are not extracted as file paths,
// and signals an unclosed "<!--" so callers can stop collecting continuation lines.
func TestStripInlineHTMLComments(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		want     string
		endsOpen bool
	}{
		{
			name:     "no comment",
			input:    "`a.go` and `b.go`",
			want:     "`a.go` and `b.go`",
			endsOpen: false,
		},
		{
			name:     "single closed span",
			input:    "`real.go` <!-- `evil.go` --> suffix",
			want:     "`real.go`   suffix",
			endsOpen: false,
		},
		{
			name:     "unclosed comment truncates",
			input:    "`real.go` <!-- `evil.go`",
			want:     "`real.go` ",
			endsOpen: true,
		},
		{
			name:     "two closed spans — tokens outside both preserved",
			input:    "<!-- `a.go` --> `x.go` <!-- `b.go` -->",
			want:     "  `x.go`  ",
			endsOpen: false,
		},
		{
			name:     "closed span then unclosed — endsOpen true",
			input:    "<!-- `a.go` --> <!-- `b.go`",
			want:     "  ",
			endsOpen: true,
		},
		{
			name:     "empty string",
			input:    "",
			want:     "",
			endsOpen: false,
		},
		{
			name:     "adjacent tokens don't merge — space inserted",
			input:    "`a`<!--x-->`b`",
			want:     "`a` `b`",
			endsOpen: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotEndsOpen := StripInlineHTMLComments(tc.input)
			if got != tc.want {
				t.Errorf("stripped: got %q, want %q", got, tc.want)
			}
			if gotEndsOpen != tc.endsOpen {
				t.Errorf("endsOpen: got %v, want %v", gotEndsOpen, tc.endsOpen)
			}
		})
	}
}
