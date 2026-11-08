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
//     Affirms: a "**Gates da wave:**" marker that falls inside a 4-backtick example fence
//     and is the ONLY gate header in the wave causes ParseGatesLines to return an error
//     (E3 rule, ML-2F) — a masked-only header is a named failure, not silent zero gates.
//
//   TestParseGatesLines_F1_RealMarkerAfterMasked
//     Affirms: when a masked marker (inside a 4-backtick fence) precedes a real marker in
//     the same wave, the real marker's commands are returned with correct line numbers —
//     confirming that the scan continues past masked markers rather than stopping, and that
//     the E3 rule does NOT fire when a valid unmasked header exists.
//
//   TestParseGatesLines_RealFile_Wave2MaskedOnlyReturnsError
//     Affirms: ParseGates on Wave 2 of docs/roadmaps/done/ROADMAP-2026-08-22-wave-0-…
//     now returns a non-nil error (E3 rule, ML-2F) because the marker at line 226 is
//     inside a 4-backtick fence and no unmasked header exists in that wave.
//
// ── New tests (ML-3C, REQ-2026-10-31) — AC11 ──────────────────────────────────
//
//   TestParseGatesLines_HTMLComment_MarkerNotGate
//     Affirms: a "**Gates da wave:**" marker whose column 0 is inside an HTML comment block
//     (<!-- … -->) and is the ONLY gate header in the wave causes ParseGatesLines to return
//     an error (E3 rule, ML-2F) — a masked-only header is a named failure, not silent zero gates.
//
//   TestParseGatesLines_HTMLComment_RealMarkerAfterCommented
//     Affirms: when a commented-out marker precedes a real marker outside the comment in
//     the same wave, only the real marker's commands are returned — the scan continues past
//     the commented marker rather than stopping, and the E3 rule does NOT fire when a valid
//     unmasked header exists.
//
// ── New tests (ML-3C ação 3, REQ-2026-10-31) — AC11b inline ──────────────────
//
//   TestStripInlineHTMLComments
//     Affirms: StripInlineHTMLComments removes each complete "<!-- … -->" span from a
//     line and truncates at an unclosed "<!--", returning (stripped, endsOpen) — so
//     backtick tokens inside comments are invisible to extractTokens while tokens outside
//     are preserved.
//
// ── New tests (D1/D3, ADR-2026-11-08 / ML-1A — bilingual gate header) ─────────
//
//   TestGatesHeaderRe_ENAlias_Positive
//     Affirms: GatesHeaderRe matches "**Wave gates:**" (the English alias), so the
//     parser recognises the EN form of the gates header without code changes in callers.
//
//   TestGatesHeaderRe_ENAlias_NegativeCases
//     Affirms: "**Wave gate:**" (singular), "**Wave  gates:**" (double space) and
//     "**wave gates:**" (lower-case) do NOT match GatesHeaderRe — the accepted set is
//     exactly {"**Gates da wave:**", "**Wave gates:**"} and nothing else.
//
//   TestParseGatesLines_ENAlias_ReturnsCommands
//     Affirms: ParseGatesLines recognises "**Wave gates:**" as a valid gate header and
//     returns the commands inside the following ```bash fence with correct line numbers.
//
//   TestParseGatesLines_D3_TwoHeadersPTandPT
//     Affirms: a wave with two "**Gates da wave:**" headers outside fences and comments
//     returns an error naming both 1-based line numbers (ADR D3).
//
//   TestParseGatesLines_D3_TwoHeadersPTandEN
//     Affirms: a wave with one PT header and one EN header outside fences and comments
//     returns a D3 error naming both line numbers — the error fires for any combination
//     of PT/EN headers, not only same-language pairs.
//
//   TestParseGatesLines_D3_TwoHeadersENandEN
//     Affirms: a wave with two "**Wave gates:**" headers outside fences and comments
//     returns a D3 error naming both line numbers.
//
//   TestParseGatesLines_D3_FencedPlusRealNoError
//     Affirms: a wave with one header inside a fence (skipped by F1) and one real header
//     outside the fence does NOT trigger the D3 duplicate-header error — only unmasked
//     headers count toward the duplicate check.

import (
	"os"
	"path/filepath"
	"strings"
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

// TestParseGatesLines_F1_MaskedMarkerNotGate asserts that a "**Gates da wave:**"
// marker that falls inside a 4-backtick example fence and has NO real (unmasked) header
// anywhere else in the wave causes ParseGatesLines to return an error (E3 rule, D8,
// ADR-2026-11-08 / ML-2F). The masked marker is not a gate declaration, but its
// presence without a companion real header means the author likely made a mistake —
// failing with an error (rather than silently returning zero gates) prevents the
// barrier from passing with no gate executed.
//
// Reconciliation (Regra Dura): when the ONLY gate header in a wave is inside a
// 4-tick example fence, ParseGatesLines returns a non-nil error naming that line —
// the E3 rule makes the previously fail-open "zero gates" into a named failure.
func TestParseGatesLines_F1_MaskedMarkerNotGate(t *testing.T) {
	// Wave with only one "**Gates da wave:**", but inside a 4-backtick example fence.
	// Under E3 (ML-2F): the wave returns an error because the only gate header is masked.
	//
	//  line 1: ## Wave 1 — F1 Masked
	//  line 2: (empty)
	//  line 3: ````
	//  line 4: Example output:
	//  line 5: **Gates da wave:**   ← inside 4-backtick fence, only header
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

	// Sanity: the marker at line 5 (0-indexed: 4) must be masked by FenceMask.
	if !fenced[4] {
		t.Errorf("expected line 5 (0-indexed 4) to be masked by FenceMask, but it is not")
	}
	if !GatesHeaderRe.MatchString(lines[4]) {
		t.Errorf("sanity: GatesHeaderRe should match line 5, but does not")
	}

	// E3 rule (ML-2F): only masked header → error naming line 5.
	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatalf("expected E3 error for masked-only header (no real header), got nil (cmds=%v)", gcmds)
	}
	if !strings.Contains(err.Error(), "gates header at line 5") {
		t.Errorf("E3: error %q does not name line 5", err.Error())
	}
	if gcmds != nil {
		t.Errorf("expected nil cmds on error path, got %v", gcmds)
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

// ── TestParseGatesLines_RealFile_Wave2MaskedOnlyReturnsError ─────────────────

// TestParseGatesLines_RealFile_Wave2MaskedOnlyReturnsError reads the real acervo file
// docs/roadmaps/done/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md
// and asserts that ParseGates on Wave 2 now returns an error — the marker at line 226
// is inside a 4-backtick fence and there is no real (unmasked) gate header in that wave,
// so the E3 rule (ML-2F) fires. Previously (ML-1A) this returned [] (zero gates, no
// error) after the F1 fix; now the masked-only condition is a named error.
//
// Reconciliation (Regra Dura): ParseGates on a wave whose only gate header is masked
// (fenced) now returns a non-nil error naming that line — the E3 rule applied to the
// real corpus file confirms the fix covers concrete in-repo content (ML-2F).
func TestParseGatesLines_RealFile_Wave2MaskedOnlyReturnsError(t *testing.T) {
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

	// E3 rule (ML-2F): the masked-only gate header at line 226 returns an error.
	cmds, parseErr := ParseGates(lines, wave2.Start, wave2.End)
	if parseErr == nil {
		t.Fatalf("expected E3 error (masked-only header at line 226), got nil (cmds=%v)", cmds)
	}
	if !strings.Contains(parseErr.Error(), "gates header at line 226") {
		t.Errorf("E3: error %q does not name line 226", parseErr.Error())
	}
}

// ── TestParseGatesLines_HTMLComment_MarkerNotGate ─────────────────────────────

// TestParseGatesLines_HTMLComment_MarkerNotGate asserts that a "**Gates da wave:**"
// marker whose column 0 is inside an HTML comment block (<!-- … -->) and has NO real
// (unmasked) header anywhere else in the wave causes ParseGatesLines to return an error
// (E3 rule, D8, ADR-2026-11-08 / ML-2F). The masked marker is not a gate declaration,
// but its presence without a companion real header means the author likely made a
// mistake — failing with an error prevents the barrier from passing with no gate executed.
//
// Contra-braço: without the E3 guard, the masked marker would be silently skipped and
// ParseGatesLines would return zero gates — the barrier would pass with no gate run.
//
// Reconciliation (Regra Dura): when the ONLY gate header in a wave is inside an HTML
// comment, ParseGatesLines returns a non-nil error naming that line (E3 rule, ML-2F).
func TestParseGatesLines_HTMLComment_MarkerNotGate(t *testing.T) {
	// Wave with only one "**Gates da wave:**", but inside an HTML comment block.
	// Under E3 (ML-2F): the wave returns an error because the only gate header is masked.
	//
	//  line 1: ## Wave 3 — HTML Comment
	//  line 2: (empty)
	//  line 3: <!--
	//  line 4: **Gates da wave:**   ← column 0 inside comment, only header
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
	// Sanity: the HTML comment IS closed (-->), so D8's unclosed-comment check will not fire.
	// The only gate header is masked by htmlMask, not by an unclosed comment.
	if htmlOpenLine, _ := HTMLCommentMaskCheck(lines, fenced, w.End); htmlOpenLine != 0 {
		t.Errorf("sanity: unexpected unclosed HTML comment at line %d (should be closed)", htmlOpenLine)
	}

	// E3 rule (ML-2F): only masked header → error naming line 4.
	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatalf("expected E3 error for HTML-comment-only header, got nil (cmds=%v)", gcmds)
	}
	if !strings.Contains(err.Error(), "gates header at line 4") {
		t.Errorf("E3: error %q does not name line 4", err.Error())
	}
	if gcmds != nil {
		t.Errorf("expected nil cmds on error path, got %v", gcmds)
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

// ── TestGatesHeaderRe_ENAlias_Positive ───────────────────────────────────────

// TestGatesHeaderRe_ENAlias_Positive asserts that GatesHeaderRe matches the English
// alias "**Wave gates:**" in addition to the Portuguese "**Gates da wave:**".
//
// Reconciliation (Regra Dura): GatesHeaderRe accepts "**Wave gates:**" as a valid gate
// header, closing the fail-open described in ADR D2 for binary versions after the first
// bilingual-reader release tag (D1, ADR-2026-11-08 / ML-1A).
func TestGatesHeaderRe_ENAlias_Positive(t *testing.T) {
	cases := []string{
		"**Gates da wave:**",           // PT — must continue to match
		"**Wave gates:**",              // EN alias — must now match
		"**Wave gates:** (required)",   // EN with suffix — prefix match, must work
		"**Gates da wave:** (obrigatórios)", // PT with suffix — still matches
	}
	for _, line := range cases {
		if !GatesHeaderRe.MatchString(line) {
			t.Errorf("GatesHeaderRe should match %q, but does not", line)
		}
	}
}

// ── TestGatesHeaderRe_ENAlias_NegativeCases ──────────────────────────────────

// TestGatesHeaderRe_ENAlias_NegativeCases asserts that the forms outside the closed
// accepted set do NOT match GatesHeaderRe.
//
// Reconciliation (Regra Dura): these forms are explicitly rejected — they are
// outside the bilingual closed set (D1 §threat model, wave 0 §5.1 via 2). A regex
// too broad would cause ordinary prose to be treated as a gate header.
func TestGatesHeaderRe_ENAlias_NegativeCases(t *testing.T) {
	cases := []struct {
		line   string
		reason string
	}{
		{"**Wave gate:**", "singular 'gate' — not in the accepted set"},
		{"**Wave  gates:**", "double space — not in the accepted set"},
		{"**wave gates:**", "lower-case first letter — headers are case-sensitive"},
		{"**Wave Gates:**", "wrong capitalisation ('Gates' capitalised)"},
		{"Wave gates:", "missing bold markers"},
		{" **Wave gates:**", "leading space — header must be at column 0"},
	}
	for _, tc := range cases {
		if GatesHeaderRe.MatchString(tc.line) {
			t.Errorf("GatesHeaderRe must NOT match %q (%s), but it does", tc.line, tc.reason)
		}
	}
}

// ── TestParseGatesLines_ENAlias_ReturnsCommands ───────────────────────────────

// TestParseGatesLines_ENAlias_ReturnsCommands asserts that ParseGatesLines recognises
// "**Wave gates:**" as a valid gate header and returns the commands from the following
// ```bash fence with correct 1-based line numbers.
//
// Reconciliation (Regra Dura): ParseGatesLines returns the gate commands when the EN
// alias is used, identical behaviour to the PT form — the fail-open of D2 is closed
// once a binary with this change is installed (D1, ADR-2026-11-08 / ML-1A).
func TestParseGatesLines_ENAlias_ReturnsCommands(t *testing.T) {
	// line 1: ## Wave 5 — EN Alias
	// line 2: (empty)
	// line 3: **Wave gates:**
	// line 4: ```bash
	// line 5: go build ./...
	// line 6: ```
	doc := "## Wave 5 — EN Alias\n" +
		"\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"go build ./...\n" +
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
		t.Fatalf("expected 1 GateCmd, got %d: %v", len(gcmds), gcmds)
	}
	// "go build ./..." is at 0-indexed line 4, so 1-based line 5.
	if gcmds[0].Line != 5 {
		t.Errorf("gcmds[0].Line = %d, want 5", gcmds[0].Line)
	}
	if gcmds[0].Text != "go build ./..." {
		t.Errorf("gcmds[0].Text = %q, want %q", gcmds[0].Text, "go build ./...")
	}
}

// ── TestParseGatesLines_D3_TwoHeadersPTandPT ─────────────────────────────────

// TestParseGatesLines_D3_TwoHeadersPTandPT asserts that a wave with two
// "**Gates da wave:**" headers outside fences and comments returns an error
// naming both 1-based line numbers (ADR D3).
//
// Reconciliation (Regra Dura): ParseGatesLines returns an error when two valid gate
// headers are present in the same wave (PT+PT), preventing silent first-wins behavior
// that would discard the second block's gates.
func TestParseGatesLines_D3_TwoHeadersPTandPT(t *testing.T) {
	// line 1: ## Wave 6 — D3 PT+PT
	// line 2: (empty)
	// line 3: **Gates da wave:**   ← first real header
	// line 4: ```bash
	// line 5: echo "first"
	// line 6: ```
	// line 7: (empty)
	// line 8: **Gates da wave:**   ← second real header
	// line 9: ```bash
	// line 10: echo "second"
	// line 11: ```
	doc := "## Wave 6 — D3 PT+PT\n" +
		"\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"echo \"first\"\n" +
		"```\n" +
		"\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"echo \"second\"\n" +
		"```\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	_, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatal("expected D3 error for two PT+PT headers, got nil")
	}
	// Error must name both line numbers as a pair: "lines 3 and 8".
	// A match on this exact substring proves both the error type and the line numbers.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "lines 3 and 8") {
		t.Errorf("D3 error message %q does not contain %q", errMsg, "lines 3 and 8")
	}
}

// ── TestParseGatesLines_D3_TwoHeadersPTandEN ─────────────────────────────────

// TestParseGatesLines_D3_TwoHeadersPTandEN asserts that a wave with one PT and
// one EN header outside fences and comments returns a D3 error naming both lines.
//
// Reconciliation (Regra Dura): ParseGatesLines returns an error for the PT+EN
// combination — D3 fires for any combination of recognised gate headers, not only
// same-language pairs (ADR D3, ADR-2026-11-08 / ML-1A).
func TestParseGatesLines_D3_TwoHeadersPTandEN(t *testing.T) {
	// line 1: ## Wave 7 — D3 PT+EN
	// line 2: (empty)
	// line 3: **Gates da wave:**   ← first real header (PT)
	// line 4: ```bash
	// line 5: echo "first"
	// line 6: ```
	// line 7: (empty)
	// line 8: **Wave gates:**      ← second real header (EN)
	// line 9: ```bash
	// line 10: echo "second"
	// line 11: ```
	doc := "## Wave 7 — D3 PT+EN\n" +
		"\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"echo \"first\"\n" +
		"```\n" +
		"\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"echo \"second\"\n" +
		"```\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	_, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatal("expected D3 error for PT+EN headers, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "lines 3 and 8") {
		t.Errorf("D3 error message %q does not contain %q", errMsg, "lines 3 and 8")
	}
}

// ── TestParseGatesLines_D3_TwoHeadersENandEN ─────────────────────────────────

// TestParseGatesLines_D3_TwoHeadersENandEN asserts that a wave with two
// "**Wave gates:**" headers outside fences and comments returns a D3 error.
//
// Reconciliation (Regra Dura): ParseGatesLines returns an error for the EN+EN
// combination — D3 applies to all language combinations (ADR D3, ADR-2026-11-08 / ML-1A).
func TestParseGatesLines_D3_TwoHeadersENandEN(t *testing.T) {
	doc := "## Wave 8 — D3 EN+EN\n" +
		"\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"echo \"first\"\n" +
		"```\n" +
		"\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"echo \"second\"\n" +
		"```\n"

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	_, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatal("expected D3 error for EN+EN headers, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "lines 3 and 8") {
		t.Errorf("D3 error message %q does not contain %q", errMsg, "lines 3 and 8")
	}
}

// ── TestParseGatesLines_D3_FencedPlusRealNoError ──────────────────────────────

// TestParseGatesLines_D3_FencedPlusRealNoError asserts that a wave with one header
// inside a fence (skipped by F1) and one real header outside the fence does NOT
// trigger the D3 duplicate-header error.
//
// Reconciliation (Regra Dura): only unmasked (non-fenced, non-commented) headers
// count toward the D3 duplicate check — a header inside a fence is documentation,
// not a declaration, and must not cause a false D3 error (ADR D3).
func TestParseGatesLines_D3_FencedPlusRealNoError(t *testing.T) {
	// line 1: ## Wave 9 — D3 Fenced+Real
	// line 2: (empty)
	// line 3: ````
	// line 4: **Gates da wave:**  ← inside fence, skipped by F1
	// line 5: ```bash
	// line 6: exit 1
	// line 7: ```
	// line 8: ````
	// line 9: (empty)
	// line 10: **Wave gates:**    ← real EN header, outside fence
	// line 11: ```bash
	// line 12: echo "real gate"
	// line 13: ```
	doc := "## Wave 9 — D3 Fenced+Real\n" +
		"\n" +
		"````\n" +
		"**Gates da wave:**\n" +
		"```bash\n" +
		"exit 1\n" +
		"```\n" +
		"````\n" +
		"\n" +
		"**Wave gates:**\n" +
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
		t.Fatalf("unexpected D3 error (fenced+real should not trigger D3): %v", err)
	}
	if len(gcmds) != 1 {
		t.Fatalf("expected 1 GateCmd from real header, got %d: %v", len(gcmds), gcmds)
	}
	if gcmds[0].Text != `echo "real gate"` {
		t.Errorf("gcmds[0].Text = %q, want %q", gcmds[0].Text, `echo "real gate"`)
	}
}

// ── New tests (D8, ADR-2026-11-08 / ML-2C — red team fixes) ──────────────────
//
//   TestParseGatesLines_A2_ExampleFenceMaskedBash
//     Affirms: a "```bash" that appears inside a 4-backtick outer example fence
//     (fenced[j]=true) is NOT treated as the gate fence opener — the j-loop skips
//     it and continues to the real "```bash" that follows the outer fence (A2 fix).
//
//   TestParseGatesLines_A1_HTMLCommentMaskedBash
//     Affirms: a "```bash" that appears on its own line inside a multi-line HTML
//     comment (htmlMask[j]=true) is NOT treated as the gate fence opener — the
//     j-loop skips it and continues to the real "```bash" outside the comment (A1 fix).
//
//   TestParseGatesLines_D1_UnclosedHTMLCommentReturnsError
//     Affirms: a wave containing an unclosed HTML comment (<!-- without -->) causes
//     ParseGatesLines to return a non-nil error instead of silently returning zero
//     gates (D8 fail-closed, ADR-2026-11-08 / ML-2C D1).
//
//   TestParseGatesLines_D8_UnclosedFenceBeforeHeaderReturnsError
//     Affirms: when a code fence opens (and never closes anywhere in the document)
//     before waveEnd, ParseGatesLines returns a non-nil error and nil cmds (D8
//     fail-closed for unclosed fence). Contra-arm: same doc with the fence closed
//     returns no error and finds the gate.
//     Note: in `barrier`, the document-level FenceMaskCheck in runBarrier fires first
//     for this scenario, so this D8 fence check is reachable via validator/Wave0GateDiagnosis
//     but unreachable from `barrier`'s runBarrier codepath.
//
//   TestParseGatesLines_E3_MaskedOnlyHeaderReturnsError
//     Affirms: when ALL gate headers in a wave are masked (fenced or HTML-commented)
//     and no unmasked header exists, ParseGatesLines returns a non-nil error naming
//     the first masked line — the gates block would be silently ignored without this
//     guard (E3 fix, D8, ADR-2026-11-08 / ML-2F). Counter-braços: (a) wave with no
//     gate header at all → zero gates, no error; (b) masked header + real unmasked
//     header → real gate commands returned, no error; (c) header only inside HTML
//     comment → error.

// TestParseGatesLines_A2_ExampleFenceMaskedBash tests the A2 fix.
//
// Contra-arm: the wrong gate text ("echo \"example gate\"") must NOT be returned.
func TestParseGatesLines_A2_ExampleFenceMaskedBash(t *testing.T) {
	// A2 (D8, ADR-2026-11-08 / ML-2C): a "```bash" inside a 4-backtick outer fence
	// (fenced[j]=true) must be skipped — only the real "```bash" after the outer
	// fence closes is the gate fence opener.
	doc := "## Wave 0 — A2 test\n" +
		"**Wave gates:**\n" +
		"````\n" +
		"This is an example fence:\n" +
		"```bash\n" +
		"echo \"example gate\"\n" +
		"```\n" +
		"````\n" +
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
		t.Fatalf("expected 1 GateCmd (real gate), got %d: %v", len(gcmds), gcmds)
	}
	if gcmds[0].Text == `echo "example gate"` {
		t.Error("A2 not fixed: returned the gate from inside the example fence instead of the real gate")
	}
	if gcmds[0].Text != `echo "real gate"` {
		t.Errorf("gcmds[0].Text = %q, want %q", gcmds[0].Text, `echo "real gate"`)
	}
}

// TestParseGatesLines_A1_HTMLCommentMaskedBash tests the A1 fix.
//
// Contra-arm: the wrong gate text ("echo \"inside-comment gate\"") must NOT be returned.
func TestParseGatesLines_A1_HTMLCommentMaskedBash(t *testing.T) {
	// A1 (D8, ADR-2026-11-08 / ML-2C): a "```bash" that is inside an HTML comment
	// (htmlMask[j]=true) must be skipped by the j-loop. The real "```bash" after
	// "-->" must be used as the gate fence opener.
	doc := "## Wave 0 — A1 test\n" +
		"**Wave gates:**\n" +
		"<!--\n" +
		"```bash\n" +
		"echo \"inside-comment gate\"\n" +
		"```\n" +
		"-->\n" +
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
		t.Fatalf("expected 1 GateCmd (real gate), got %d: %v", len(gcmds), gcmds)
	}
	if gcmds[0].Text == `echo "inside-comment gate"` {
		t.Error("A1 not fixed: returned the gate from inside the HTML comment instead of the real gate")
	}
	if gcmds[0].Text != `echo "real gate"` {
		t.Errorf("gcmds[0].Text = %q, want %q", gcmds[0].Text, `echo "real gate"`)
	}
}

// TestParseGatesLines_D1_UnclosedHTMLCommentReturnsError tests the D1 / D8 fix.
//
// Contra-arm: the returned slice must be nil (error path, not empty-gates path).
func TestParseGatesLines_D1_UnclosedHTMLCommentReturnsError(t *testing.T) {
	// D1 fail-closed (D8, ADR-2026-11-08 / ML-2C): an unclosed HTML comment that
	// spans the wave boundary forces ParseGatesLines to return an error — the
	// comment could hide the gates header, so "zero gates" is not a safe result.
	doc := "## Wave 0 — D1 test\n" +
		"<!--\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"echo \"hidden gate\"\n" +
		"```\n"
	// No closing "-->" — the HTML comment is left unclosed.

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatalf("expected an error for unclosed HTML comment, got nil (cmds=%v)", gcmds)
	}
	if !strings.Contains(err.Error(), "unclosed HTML comment") {
		t.Errorf("D1: error %q does not contain \"unclosed HTML comment\"", err.Error())
	}
	if gcmds != nil {
		t.Errorf("expected nil cmds on error path, got %v", gcmds)
	}
}

// TestParseGatesLines_D8_UnclosedFenceBeforeHeaderReturnsError tests the D8 fail-closed
// rule for the unclosed-fence branch.
//
// Contra-arm: the same wave with the fence closed (before the gates header) returns
// no error and finds the real gate command.
//
// Note on reachability: in `trackfw barrier`, the document-level FenceMaskCheck in
// runBarrier fires before ParseGatesLines is called, so this D8 branch is unreachable
// from barrier's runBarrier. It is reachable via validator/Wave0GateDiagnosis and
// direct unit callers. The test exercises ParseGatesLines directly.
func TestParseGatesLines_D8_UnclosedFenceBeforeHeaderReturnsError(t *testing.T) {
	// D8 unclosed fence (ADR-2026-11-08 / ML-2C): a 3-tick fence opens at line 2
	// and is never closed anywhere in the document. openFenceLine=2 (1-based), and
	// 2-1=1 < waveEnd=5. ParseGatesLines must return an error (fail-closed).
	docUnclosed := "## Wave 0 — D8 unclosed fence\n" +
		"```\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"echo \"hidden gate\"\n"
	// No closing ``` — fence is never closed.

	lines := SplitRoadmapLines(docUnclosed)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatalf("D8 unclosed fence: expected error, got nil (cmds=%v)", gcmds)
	}
	if !strings.Contains(err.Error(), "unclosed code fence") {
		t.Errorf("D8 unclosed fence: error %q does not contain \"unclosed code fence\"", err.Error())
	}
	if gcmds != nil {
		t.Errorf("D8 unclosed fence: expected nil cmds on error path, got %v", gcmds)
	}

	// Contra-arm: a doc where the fence closes BEFORE the gate header — no error, gate found.
	// (Note: adding a closing ``` to docUnclosed would give the E3 scenario: zero gates, no
	// error, because the fence still masks the header throughout. So the contra-arm uses a
	// structurally different doc where the closed fence precedes the gate header.)
	docClosed := "## Wave 0 — D8 closed fence\n" +
		"```\n" +
		"some prose\n" +
		"```\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"echo \"real gate\"\n" +
		"```\n"
	lines2 := SplitRoadmapLines(docClosed)
	fenced2 := FenceMask(lines2)
	waves2, _ := ParseWaves(lines2, fenced2)
	if len(waves2) != 1 {
		t.Fatalf("contra-arm: expected 1 wave, got %d", len(waves2))
	}
	w2 := waves2[0]
	gcmds2, err2 := ParseGatesLines(lines2, w2.Start, w2.End)
	if err2 != nil {
		t.Fatalf("contra-arm: expected no error (fence closed), got %v", err2)
	}
	if len(gcmds2) != 1 || gcmds2[0].Text != `echo "real gate"` {
		t.Errorf("contra-arm: expected [echo \"real gate\"], got %v", gcmds2)
	}
}

// TestParseGatesLines_E3_MaskedOnlyHeaderReturnsError tests the E3 fix (ML-2F).
//
// E3 (D8, ADR-2026-11-08 / ML-2F): a 3-tick fence that opens and closes within the
// wave, masking the gates header due to CommonMark fence semantics. scanFences returns
// openLine=0 (all fences close), so D8's unclosed-fence check cannot fire. With the
// E3 fix, ParseGatesLines detects that the only GatesHeaderRe match in the wave is
// masked and returns an error instead of silently returning zero gates.
//
// Reconciliation (Regra Dura): ParseGatesLines returns a non-nil error naming the first
// masked gate header line when no unmasked gate header exists in the wave — the
// previously fail-open E3 vector is now closed (D8, ADR-2026-11-08 / ML-2F).
//
// Counter-braços:
//   (a) wave with no gate header at all — zero gates, no error (absence is legal)
//   (b) masked header inside 4-tick fence + real unmasked header — real gates returned, no error
//   (c) header only inside HTML comment (closed) — error naming the masked line
func TestParseGatesLines_E3_MaskedOnlyHeaderReturnsError(t *testing.T) {
	// Main arm: E3 scenario — 3-tick fence opens at line 2 (no info string),
	// masks the gates header at line 3. The ```bash at line 4 has an info string so
	// it does NOT close the outer fence per CommonMark. The outer fence closes at
	// line 6 (plain ```). scanFences returns openLine=0. ParseGatesLines now detects
	// the masked-only header and returns an error.
	doc := "## Wave 0 — E3 fix\n" +
		"```\n" +
		"**Wave gates:**\n" +
		"```bash\n" +
		"echo \"gate\"\n" +
		"```\n"
	// The final ``` closes the outer fence (plain ```, no info string).

	lines := SplitRoadmapLines(doc)
	fenced := FenceMask(lines)
	waves, _ := ParseWaves(lines, fenced)
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	w := waves[0]

	// Sanity: scanFences returns openLine=0 — all fences close (the E3 characterisation).
	_, openFenceLine := scanFences(lines)
	if openFenceLine != 0 {
		t.Errorf("E3: scanFences returned openFenceLine=%d, expected 0 — characterisation changed", openFenceLine)
	}
	// Sanity: the gates header at line 3 (0-indexed 2) is masked.
	if !fenced[2] {
		t.Errorf("E3: expected line 3 (0-indexed 2) to be masked by FenceMask, but it is not")
	}

	gcmds, err := ParseGatesLines(lines, w.Start, w.End)
	if err == nil {
		t.Fatalf("E3: expected error for masked-only gate header, got nil (cmds=%v)", gcmds)
	}
	if !strings.Contains(err.Error(), "gates header at line 3") {
		t.Errorf("E3: error %q does not name line 3", err.Error())
	}
	if !strings.Contains(err.Error(), "inside a code fence or HTML comment") {
		t.Errorf("E3: error %q does not mention code fence or HTML comment", err.Error())
	}
	if gcmds != nil {
		t.Errorf("E3: expected nil cmds on error path, got %v", gcmds)
	}

	// Counter-braço (a): wave with no gate header at all — zero gates, no error.
	// Absence of a gate header is legal ("a wave with no valid gate header returns an
	// empty, non-nil slice"); the E3 rule must not fire here.
	docNoHeader := "## Wave 0 — no header\n" +
		"Some prose with no gate block.\n"
	lines2 := SplitRoadmapLines(docNoHeader)
	fenced2 := FenceMask(lines2)
	waves2, _ := ParseWaves(lines2, fenced2)
	if len(waves2) != 1 {
		t.Fatalf("counter-arm (a): expected 1 wave, got %d", len(waves2))
	}
	w2 := waves2[0]
	gcmds2, err2 := ParseGatesLines(lines2, w2.Start, w2.End)
	if err2 != nil {
		t.Fatalf("counter-arm (a): expected no error (no header), got %v", err2)
	}
	if len(gcmds2) != 0 {
		t.Errorf("counter-arm (a): expected 0 gates, got %v", gcmds2)
	}

	// Counter-braço (b): masked header inside 4-tick fence PLUS real unmasked header.
	// This is the "example + real" pattern from TestParseGatesLines_D3_FencedPlusRealNoError
	// but applied here: the E3 rule must NOT fire when a valid unmasked header also exists.
	docMaskedPlusReal := "## Wave 0 — masked and real\n" +
		"````\n" +
		"**Wave gates:**\n" + // masked header (inside 4-tick fence)
		"```bash\n" +
		"exit 1\n" +
		"```\n" +
		"````\n" +
		"\n" +
		"**Wave gates:**\n" + // real unmasked header
		"```bash\n" +
		"echo \"real\"\n" +
		"```\n"
	lines3 := SplitRoadmapLines(docMaskedPlusReal)
	fenced3 := FenceMask(lines3)
	waves3, _ := ParseWaves(lines3, fenced3)
	if len(waves3) != 1 {
		t.Fatalf("counter-arm (b): expected 1 wave, got %d", len(waves3))
	}
	w3 := waves3[0]
	gcmds3, err3 := ParseGatesLines(lines3, w3.Start, w3.End)
	if err3 != nil {
		t.Fatalf("counter-arm (b): expected no error (real header present), got %v", err3)
	}
	if len(gcmds3) != 1 || gcmds3[0].Text != `echo "real"` {
		t.Errorf("counter-arm (b): expected [echo \"real\"], got %v", gcmds3)
	}

	// Counter-braço (c): header only inside a CLOSED HTML comment — error naming the line.
	// This exercises the htmlMask[i] branch of the E3 guard.
	docHTMLOnly := "## Wave 0 — HTML-comment-only header\n" +
		"<!--\n" +
		"**Wave gates:**\n" + // masked by HTML comment (line 3)
		"```bash\n" +
		"echo \"hidden\"\n" +
		"```\n" +
		"-->\n"
	// The comment IS closed (-->), so D8's unclosed-HTML check will not fire.
	// The gates header is masked by htmlMask, not by an unclosed comment.
	lines4 := SplitRoadmapLines(docHTMLOnly)
	fenced4 := FenceMask(lines4)
	waves4, _ := ParseWaves(lines4, fenced4)
	if len(waves4) != 1 {
		t.Fatalf("counter-arm (c): expected 1 wave, got %d", len(waves4))
	}
	w4 := waves4[0]
	// Sanity: header at line 3 (0-indexed 2) is in HTML comment, not fenced.
	htmlMask4 := HTMLCommentMask(lines4, fenced4)
	if !htmlMask4[2] {
		t.Errorf("counter-arm (c): expected line 3 (0-indexed 2) to be in HTML comment mask")
	}
	if fenced4[2] {
		t.Errorf("counter-arm (c): line 3 must NOT be fenced (it is in an HTML comment)")
	}
	gcmds4, err4 := ParseGatesLines(lines4, w4.Start, w4.End)
	if err4 == nil {
		t.Fatalf("counter-arm (c): expected error for HTML-comment-only header, got nil (cmds=%v)", gcmds4)
	}
	if !strings.Contains(err4.Error(), "gates header at line 3") {
		t.Errorf("counter-arm (c): error %q does not name line 3", err4.Error())
	}
	if !strings.Contains(err4.Error(), "inside a code fence or HTML comment") {
		t.Errorf("counter-arm (c): error %q does not mention code fence or HTML comment", err4.Error())
	}
	if gcmds4 != nil {
		t.Errorf("counter-arm (c): expected nil cmds on error path, got %v", gcmds4)
	}
}

