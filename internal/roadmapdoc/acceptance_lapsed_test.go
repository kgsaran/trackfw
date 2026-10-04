package roadmapdoc

// acceptance_lapsed_test.go — unit tests for AcceptanceEvaluateFull introduced
// by D2/D3/D6 (REQ #514 / ML-1A).
//
// Each test asserts one rule from the specification. The rule is stated in the
// comment immediately above the test function.

import (
	"strings"
	"testing"
)

// helper: parse lines from joined string and return FenceMask.
func mustParseMLs(t *testing.T, content string) ([]string, []bool, []MLBlock) {
	t.Helper()
	lines := strings.Split(content, "\n")
	fenced := FenceMask(lines)
	mls := ParseMLs(lines, fenced, 0, len(lines))
	if len(mls) == 0 {
		t.Fatal("mustParseMLs: no ML blocks found in content")
	}
	return lines, fenced, mls
}

// ─────────────────────────────────────────────────────────────────────────────
// D2: lapsed classification — canonical and invalid forms
// ─────────────────────────────────────────────────────────────────────────────

// TestAcceptanceEvaluateFull_LapsedCanonicalForm asserts that AcceptanceEvaluateFull
// classifies a "- [ ]" item immediately followed by "  Caducou: <text>" (≥2 spaces,
// non-empty text) as Lapsed=1, Unmet=0, HasBlock=true.
func TestAcceptanceEvaluateFull_LapsedCanonicalForm(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] suíte pypi sem regressão",
		"      Caducou: a v8 removeu o CLI Python (#365)",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if !d.HasBlock {
		t.Fatal("expected HasBlock=true")
	}
	if d.Lapsed != 1 {
		t.Fatalf("expected Lapsed=1, got %d", d.Lapsed)
	}
	if d.Unmet != 0 {
		t.Fatalf("expected Unmet=0, got %d", d.Unmet)
	}
	if d.Met != 0 {
		t.Fatalf("expected Met=0, got %d", d.Met)
	}
}

// TestAcceptanceEvaluateFull_LapsedEmptyJustification asserts that AcceptanceEvaluateFull
// rejects "  Caducou:" with no text after the colon as unmet (D2: justificativa obrigatória).
func TestAcceptanceEvaluateFull_LapsedEmptyJustification(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion",
		"  Caducou:",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (empty justification must not count), got %d", d.Lapsed)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedNoIndent asserts that AcceptanceEvaluateFull rejects
// a "Caducou:" line at column 0 (no indentation) as not a continuation (D2).
func TestAcceptanceEvaluateFull_LapsedNoIndent(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion",
		"Caducou: not indented",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (missing indentation), got %d", d.Lapsed)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedAfterBlankLine asserts that AcceptanceEvaluateFull
// does NOT treat a "Caducou:" continuation separated from its "[ ]" by a blank line
// as lapsed — the blank line breaks the continuation (D2).
func TestAcceptanceEvaluateFull_LapsedAfterBlankLine(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion",
		"",
		"  Caducou: texto aqui",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (blank line breaks continuation), got %d", d.Lapsed)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedInFencedBlock asserts that AcceptanceEvaluateFull
// does NOT count a "Caducou:" line that appears inside a fenced code block as a valid
// continuation. NOTE: this test exercises the adjacency requirement, not the
// !fenced[next] guard: the fence opener "```" is the line immediately following "- [ ]"
// and does not match LapsedContinuationRe. The fenced guard is defense-in-depth and
// is currently unreachable by any well-formed markdown input (a fenced line adjacent to
// an unfenced criterion would require the criterion itself to be inside the fence, which
// is already excluded by fenced[i]).
func TestAcceptanceEvaluateFull_LapsedInFencedBlock(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion",
		"```",
		"  Caducou: inside fence",
		"```",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (fence opener '```' is adjacent, not Caducou:), got %d", d.Lapsed)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedInHTMLCommentAdjacent asserts that AcceptanceEvaluateFull
// does NOT count "<!--" (adjacent line) as a Caducou: continuation — it exercises adjacency,
// not the htmlMask. The form "<!--\n  Caducou:..." has an opener (not a Caducou: line) adjacent.
func TestAcceptanceEvaluateFull_LapsedInHTMLCommentAdjacent(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion",
		"<!--",
		"  Caducou: inside HTML comment",
		"-->",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (HTML opener '<!--' is adjacent, not Caducou:), got %d", d.Lapsed)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedInHTMLComment asserts that AcceptanceEvaluateFull
// does NOT count a "Caducou:" line that is masked by the htmlCommentMask because the
// comment opens on the criterion line itself: "- [ ] criterion <!--\n  Caducou: text".
// This test actually exercises !htmlMask[next]: without the mask, the Caducou: line
// would be adjacent and syntactically valid, and the criterion would be lapsed.
func TestAcceptanceEvaluateFull_LapsedInHTMLComment(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion <!--",
		"  Caducou: inside HTML comment",
		"-->",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (Caducou: is inside HTML comment — htmlMask must block it), got %d", d.Lapsed)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedNoSpaceAfterColon asserts that AcceptanceEvaluateFull
// accepts "Caducou:text" (no space after colon) as a valid lapsed continuation (D2:
// "texto não vazio depois dos dois-pontos" — the spec does not mandate a space).
func TestAcceptanceEvaluateFull_LapsedNoSpaceAfterColon(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion",
		"  Caducou:v8-removeu-python",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 1 {
		t.Fatalf("expected Lapsed=1 (Caducou:text with no space is valid), got %d", d.Lapsed)
	}
	if d.Unmet != 0 {
		t.Fatalf("expected Unmet=0, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedSingleLineHTMLComment asserts that AcceptanceEvaluateFull
// does NOT count "  <!-- Caducou: ... -->" (single-line HTML comment) as a valid continuation
// — the line starts with "<!--", not "  Caducou:", so LapsedContinuationRe does not match.
func TestAcceptanceEvaluateFull_LapsedSingleLineHTMLComment(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] criterion",
		"  <!-- Caducou: v8 removed -->",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (single-line HTML comment must not match), got %d", d.Lapsed)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// D6/T3: unrecognized checkbox characters count as pending
// ─────────────────────────────────────────────────────────────────────────────

// TestAcceptanceEvaluateFull_UnrecognizedCheckbox asserts that AcceptanceEvaluateFull
// counts a "- [~]" criterion as Unmet=1 and records its 1-based line number in
// UnrecognizedLines (D6/T3: only [x]/[X] are met; any other char is pending).
func TestAcceptanceEvaluateFull_UnrecognizedCheckbox(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [~] partial work done",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if !d.HasBlock {
		t.Fatal("expected HasBlock=true")
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1, got %d", d.Unmet)
	}
	if d.Met != 0 {
		t.Fatalf("expected Met=0 ([~] must not count as met), got %d", d.Met)
	}
	if len(d.UnrecognizedLines) != 1 {
		t.Fatalf("expected 1 UnrecognizedLine, got %v", d.UnrecognizedLines)
	}
	// The criterion is on the 4th line of the content (0-indexed: 3; 1-based: 4).
	wantLine := 4
	if d.UnrecognizedLines[0] != wantLine {
		t.Fatalf("expected UnrecognizedLines[0]=%d, got %d", wantLine, d.UnrecognizedLines[0])
	}
}

// TestAcceptanceEvaluateFull_MetIsOnlyXorCapitalX asserts that AcceptanceEvaluateFull
// recognizes "[x]" and "[X]" as the only met forms (D6/T3).
func TestAcceptanceEvaluateFull_MetIsOnlyXorCapitalX(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] lowercase x",
		"- [X] uppercase X",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Met != 2 {
		t.Fatalf("expected Met=2, got %d", d.Met)
	}
	if d.Unmet != 0 {
		t.Fatalf("expected Unmet=0, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_UnrecognizedPlusLapsed asserts that AcceptanceEvaluateFull
// correctly handles a mix: "[~]" counts as unmet, "[  ] Caducou:" counts as lapsed,
// and "[x]" counts as met — three classes present simultaneously.
func TestAcceptanceEvaluateFull_UnrecognizedPlusLapsed(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] build passes",
		"- [~] partial work",
		"- [ ] suíte pypi",
		"  Caducou: v8 removeu o CLI Python",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Met != 1 {
		t.Fatalf("expected Met=1, got %d", d.Met)
	}
	if d.Unmet != 1 {
		t.Fatalf("expected Unmet=1 ([~] counts as unmet), got %d", d.Unmet)
	}
	if d.Lapsed != 1 {
		t.Fatalf("expected Lapsed=1, got %d", d.Lapsed)
	}
	if len(d.UnrecognizedLines) != 1 {
		t.Fatalf("expected 1 UnrecognizedLine, got %v", d.UnrecognizedLines)
	}
}

// TestAcceptanceEvaluateFull_AllLapsed asserts that AcceptanceEvaluateFull returns
// Met=0 and Lapsed=2 when all criteria are lapsed — the T8 condition detected by the
// barrier ("all acceptance criteria lapsed").
func TestAcceptanceEvaluateFull_AllLapsed(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [ ] suíte pypi",
		"  Caducou: a v8 removeu o CLI Python",
		"- [ ] versão reportada no help",
		"  Caducou: número exato caducou",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Met != 0 {
		t.Fatalf("expected Met=0, got %d", d.Met)
	}
	if d.Lapsed != 2 {
		t.Fatalf("expected Lapsed=2, got %d", d.Lapsed)
	}
	if d.Unmet != 0 {
		t.Fatalf("expected Unmet=0, got %d", d.Unmet)
	}
}

// TestAcceptanceEvaluateFull_LapsedParentMetNotUnmet asserts that AcceptanceEvaluateFull
// does not lapse a criterion whose parent box is "[x]" (already met) — lapsed applies
// only to "[ ]" parents.
func TestAcceptanceEvaluateFull_LapsedParentMetNotUnmet(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] criterion",
		"  Caducou: this text is ignored — parent is already met",
	}, "\n")
	lines, fenced, mls := mustParseMLs(t, content)
	d := AcceptanceEvaluateFull(lines, fenced, mls[0])
	if d.Met != 1 {
		t.Fatalf("expected Met=1, got %d", d.Met)
	}
	if d.Lapsed != 0 {
		t.Fatalf("expected Lapsed=0 (parent [x] must not be lapsed), got %d", d.Lapsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Backward compat: AcceptanceEvaluate wrapper
// ─────────────────────────────────────────────────────────────────────────────

// TestAcceptanceEvaluate_LapsedCountsAsUnmetInWrapper asserts that the legacy
// AcceptanceEvaluate wrapper returns lapsed items in the unmet count (D2: "toda
// contagem existente continua verdadeira" — the box is genuinely open).
func TestAcceptanceEvaluate_LapsedCountsAsUnmetInWrapper(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] criterion met",
		"- [ ] lapsed criterion",
		"  Caducou: justified",
	}, "\n")
	lines := strings.Split(content, "\n")
	fenced := FenceMask(lines)
	mls := ParseMLs(lines, fenced, 0, len(lines))
	met, unmet, hasBlock := AcceptanceEvaluate(lines, fenced, mls[0])
	if !hasBlock {
		t.Fatal("expected hasBlock=true")
	}
	if met != 1 {
		t.Fatalf("expected met=1, got %d", met)
	}
	// Lapsed counts as unmet in the legacy API.
	if unmet != 1 {
		t.Fatalf("expected unmet=1 (lapsed included in legacy unmet), got %d", unmet)
	}
}
