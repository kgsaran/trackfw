package roadmapdoc

// roadmapdoc_test.go — unit tests for the roadmapdoc leaf package (ML-1A + ML-1B + ML-1D, REQ #392).
//
// AC11 reconciliation (one sentence per test asserting which conclusion it affirms):
//
//   TestHasUnfinishedMLs_PositiveBraco
//     Affirms: HasUnfinishedMLs returns true for a roadmap that contains at least one ML
//     with a ⬜ Pendente status — the canonical positive case for the gate that this REQ
//     introduces.
//
//   TestHasUnfinishedMLs_ContraBraco
//     Affirms: HasUnfinishedMLs returns false for a roadmap where every ML carries a
//     status that StatusIsComplete accepts, proving the predicate does not false-alarm on
//     a legitimately concluded roadmap.
//
//   TestHasUnfinishedMLs_TerminatedIsNotPending
//     Affirms: a ML explicitly marked ABANDONADO is classified as Terminated, not Pending,
//     so HasUnfinishedMLs returns false for a roadmap whose only non-complete ML is
//     terminated — encoding the policy decision that explicit abandonment is intentional.
//
//   TestHasUnfinishedMLs_MissingStatusIsPending
//     Affirms: a ML without a **Status:** line is treated as Pending (fail-safe), so
//     HasUnfinishedMLs returns true — a missing marker cannot release a done transition.
//
//   TestStatusCategory_TerminatedVariants
//     Affirms: the three-category classifier correctly identifies every "terminated" marker
//     in the vocabulary (ABANDONADO, 🚫 Abandonado, ❌ Cancelado) as StatusTerminated,
//     and ❌ Bloqueado as StatusPending — the first-token-only shortcut would invert the
//     last two.
//
//   TestStatusCategory_CompleteVariants
//     Affirms: StatusCategory delegates Complete classification to StatusIsComplete, so
//     every marker that StatusIsComplete accepts is also classified as StatusComplete by
//     the three-category function.
//
//   TestCorpusMeasurement_ReportOnly
//     Affirms: when docs/roadmaps/done/ exists (flat layout), the test logs the current
//     corpus count without asserting a specific number; when the directory is absent (by_agent
//     layout or consumer tree), it calls t.Skip instead of t.Fatal — fulfilling "never fails"
//     in both layouts.  The test exists to surface the count in the CI log of the upstream.
//
//   TestWaveLabelRe_CaseInsensitiveSuffix (ML-1B / AC3-ter)
//     Affirms: WaveLabelRe accepts "3-Py" (upper-case P suffix) after the case-insensitive
//     fix, and continues to reject "abc" (no digit prefix) — the counter-test that
//     ensures the fix does not extend acceptance to genuinely invalid labels.
//
//   TestParseWaves_FailSafeIsUnfinished (ML-1B / Ação 3)
//     Affirms: HasUnfinishedMLs returns true when ParseWaves returns an error — a malformed
//     wave heading cannot release a done transition (fail-safe closed).
//
//   TestCompareWaveLabels_CaseNeutral (ML-1B / AC3-ter)
//     Affirms: "3-Py" and "3-py" compare as equal by CompareWaveLabels — the ordering of
//     waves must not depend on the case of the authored suffix.
//
//   TestSplitWaveLabel_NoHyphen (ML-1D)
//     Affirms: SplitWaveLabel("1b") returns (1, "b"), not (0, "") — the no-hyphen suffix
//     form is parsed correctly so that CompareWaveLabels can normalise "1b" and "1-b" to
//     the same (integer, suffix) pair.
//
//   TestSplitWaveLabel_Invariant_PreviouslyValidLabels (ML-1D)
//     Affirms: for every label form valid before ML-1D (digit-only and digit-hyphen-suffix),
//     SplitWaveLabel returns the same (integer, suffix) as the old implementation —
//     no preexisting pin line in the corpus TSV was reclassified by the parser rewrite.
//
//   TestCompareWaveLabels_NoHyphenEquivalence (ML-1D)
//     Affirms: CompareWaveLabels("1b", "1-b") == 0 — the two forms of the same label are
//     considered equal, so --wave 1-b resolves to a document heading "## Wave 1b".
//
//   TestCompareWaveLabels_Ordering_NoHyphenSuffix (ML-1D)
//     Affirms: "1" < "1b" < "2" in CompareWaveLabels ordering — the no-hyphen form still
//     sorts between the bare integer and the next integer, matching the hyphen form.
//
//   TestParseWaves_CascadeIsolated (ML-1D)
//     Affirms: a document with a malformed wave label and a valid wave produces a non-empty
//     []MalformedWave AND a non-empty []WaveBlock — the malformed wave is isolated, not
//     propagated, proving the cascade-abort behaviour of ADR-2026-07-29 decision 16 is reversed.
//
// CORPUS MEASUREMENT NOTE (updated by ML-1A of REQ #396):
//   No specific-number assertion is written; the count is logged only.  The corpus grows with
//   every merged roadmap; pinning a number here would cause false failures on every merge that
//   moves a roadmap to done/.
//
//   Historical baselines (kept for audit continuity, not for assertion):
//     ML-1A pre-fix (REQ #392 era, ~27 items in done/): byStatusIsComplete=27, HasUnfinishedMLs=30
//     After ML-1B case-insensitive fix: HasUnfinishedMLs count aligned closer to byStatusIsComplete
//     As of REQ #396 (2026-09-22), done/ contains ≥194 files — those earlier numbers are stale.
//
//   ML-1A (REQ #396) change: replaced t.Fatalf with t.Skip when done/ is absent (errors.Is
//   fs.ErrNotExist), so the test fulfils its "never fails" contract in consumer trees with
//   by_agent layout.  Non-ENOENT errors (e.g. ENOTDIR) are logged and the function returns
//   without failing.  In the flat upstream layout the directory exists and the measurement
//   runs unchanged.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot returns the repository root from the package directory (internal/roadmapdoc).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// Tests run in internal/roadmapdoc/; repo root is two levels up.
	return filepath.Join(wd, "..", "..")
}

// readFixture reads a file from internal/roadmapdoc/testdata/ and returns its content.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	wd, _ := os.Getwd()
	data, err := os.ReadFile(filepath.Join(wd, "testdata", name))
	if err != nil {
		t.Fatalf("readFixture %q: %v", name, err)
	}
	return string(data)
}

// ── HasUnfinishedMLs — frozen-fixture tests ───────────────────────────────────

// TestHasUnfinishedMLs_PositiveBraco uses the frozen sync-enumera fixture (2 MLs ⬜)
// to confirm the positive case of the ML-done predicate.
//
// AC11: HasUnfinishedMLs returns true for a roadmap with at least one ⬜ Pendente ML.
func TestHasUnfinishedMLs_PositiveBraco(t *testing.T) {
	data := readFixture(t, "fixture-sync-enumera.md")
	if !HasUnfinishedMLs(data) {
		t.Fatal("HasUnfinishedMLs(sync-enumera) = false, want true — fixture has pending MLs")
	}
}

// TestHasUnfinishedMLs_ContraBraco uses the frozen leniencia fixture (7 MLs ✅)
// to confirm the predicate does not alarm on a fully-concluded roadmap.
//
// AC11: HasUnfinishedMLs returns false when all MLs satisfy StatusIsComplete.
func TestHasUnfinishedMLs_ContraBraco(t *testing.T) {
	data := readFixture(t, "fixture-leniencia.md")
	if HasUnfinishedMLs(data) {
		t.Fatal("HasUnfinishedMLs(leniencia) = true, want false — all MLs are ✅")
	}
}

// TestHasUnfinishedMLs_TerminatedIsNotPending confirms the three-category policy:
// a ML marked ABANDONADO releases (does not block) the done transition.
//
// AC11: a ML with status ABANDONADO is classified StatusTerminated, not StatusPending,
// so HasUnfinishedMLs returns false even though StatusIsComplete also returns false for it.
func TestHasUnfinishedMLs_TerminatedIsNotPending(t *testing.T) {
	content := "# Roadmap\n\n## Wave 1 — Foo\n\n### ML-1A — Work\n**Status:** ✅ Concluído\n**Acceptance criteria:**\n- [x] done\n\n### ML-1B — Abandoned\n**Status:** ABANDONADO — abordagem revertida\n**Acceptance criteria:**\n- [x] n/a\n"
	if HasUnfinishedMLs(content) {
		t.Fatal("HasUnfinishedMLs with ABANDONADO ML = true, want false — terminated ML must not block done")
	}
}

// TestHasUnfinishedMLs_MissingStatusIsPending confirms that a ML without a **Status:** line
// is treated as pending (fail-safe).
//
// AC11: a ML with no status marker causes HasUnfinishedMLs to return true — a missing
// marker cannot be silently treated as complete.
func TestHasUnfinishedMLs_MissingStatusIsPending(t *testing.T) {
	content := "# Roadmap\n\n## Wave 1 — Foo\n\n### ML-1A — Work\n(no status line)\n"
	if !HasUnfinishedMLs(content) {
		t.Fatal("HasUnfinishedMLs with missing status = false, want true — missing marker must be treated as pending")
	}
}

// ── StatusCategory — three-category classifier ────────────────────────────────

// TestStatusCategory_TerminatedVariants verifies every "terminated" first-token and
// the ❌ disambiguation (❌ Cancelado = Terminated, ❌ Bloqueado = Pending).
//
// AC11: the three-category classifier correctly identifies Terminated markers from
// the vocabulary; without two-token disambiguation for ❌, ❌ Bloqueado would be
// incorrectly classified as Terminated, releasing a transition it should block.
func TestStatusCategory_TerminatedVariants(t *testing.T) {
	terminated := []string{
		"ABANDONADO — abordagem revertida",
		"ABANDONADO",
		"abandonado",
		"🚫 Abandonado",
		"❌ Cancelado",
		"❌ cancelado — descartado",
	}
	pending := []string{
		"❌ Bloqueado — veredito BLOQUEAR",
		"❌ Bloqueado",
		"⬜ Pendente",
		"🔄 Em andamento",
		"pending",
		"",
	}

	for _, marker := range terminated {
		cat := StatusCategory(marker)
		if cat != StatusTerminated {
			t.Errorf("StatusCategory(%q) = %d, want StatusTerminated(%d)", marker, cat, StatusTerminated)
		}
	}
	for _, marker := range pending {
		cat := StatusCategory(marker)
		if cat != StatusPending {
			t.Errorf("StatusCategory(%q) = %d, want StatusPending(%d)", marker, cat, StatusPending)
		}
	}
}

// TestStatusCategory_CompleteVariants verifies that StatusCategory delegates to
// StatusIsComplete for the Complete classification.
//
// AC11: every marker that StatusIsComplete accepts is also StatusComplete in the
// three-category function — the two functions cannot diverge on the complete case.
func TestStatusCategory_CompleteVariants(t *testing.T) {
	complete := []string{
		"✅ Concluído",
		"✅",
		"done",
		"Concluído",
		"CONCLUIDO",
	}
	for _, marker := range complete {
		if !StatusIsComplete(marker) {
			t.Errorf("StatusIsComplete(%q) = false (pre-check; fix StatusIsComplete test)", marker)
			continue
		}
		cat := StatusCategory(marker)
		if cat != StatusComplete {
			t.Errorf("StatusCategory(%q) = %d, want StatusComplete(%d)", marker, cat, StatusComplete)
		}
	}
}

// ── Corpus measurement — report-only, never fails ─────────────────────────────

// TestCorpusMeasurement_ReportOnly logs the current count of done/ roadmaps with
// unfinished MLs. Does NOT assert a specific number (see CORPUS MEASUREMENT NOTE above).
//
// AC11: documents the measurement without asserting, surfacing the count in CI logs
// so divergence from the architect's baseline (32) is visible and can be adjudicated.
//
// Reconciliation (ML-2A): afirma que quando done/ existe mas está vazia (total == 0), o
// teste emite t.Skip declarado em vez de silenciar com "total=0" — guardando contra a
// classe de defeito "medir sem ter medido" (hades-tf R-A, 2026-09-22).
func TestCorpusMeasurement_ReportOnly(t *testing.T) {
	doneDir := filepath.Join(repoRoot(t), "docs", "roadmaps", "done")
	entries, err := os.ReadDir(doneDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// docs/roadmaps/done/ does not exist in this tree — by_agent layout or consumer
			// tree where roadmaps live under docs/roadmaps/<agent>/done/.  There is no flat
			// done/ to measure; the test skips rather than fails, honouring "never fails".
			t.Skipf("docs/roadmaps/done/ not found (%s) — by_agent layout or consumer tree; corpus measurement skipped", doneDir)
		}
		// Any other I/O error: log and return.  "Never fails" means no t.Fatalf regardless
		// of error kind — including ENOTDIR if a path component is a regular file.
		t.Logf("ReadDir %s: %v — corpus measurement skipped", doneDir, err)
		return
	}

	total := 0
	byStatusIsComplete := 0 // any ML where !StatusIsComplete
	byThreeCategory := 0    // HasUnfinishedMLs

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		total++
		path := filepath.Join(doneDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Logf("ReadFile %s: %v", path, err)
			continue
		}

		lines := SplitRoadmapLines(string(data))
		fenced := FenceMask(lines)
		waves, _ := ParseWaves(lines)
		anyNotComplete := false
		for _, wave := range waves {
			mls := ParseMLs(lines, fenced, wave.Start, wave.End)
			for _, ml := range mls {
				marker, found := MLStatusMarker(lines, fenced, ml)
				if !found || !StatusIsComplete(marker) {
					anyNotComplete = true
				}
			}
		}
		if anyNotComplete {
			byStatusIsComplete++
		}
		if HasUnfinishedMLs(string(data)) {
			byThreeCategory++
		}
	}

	// ML-2A (REQ #396): guard de vacuidade. done/ existe mas tem zero arquivos .md (ex.:
	// sparse-checkout que materializa o dir sem os arquivos, ou done/ contém só .gitkeep).
	// Silenciar com total=0 é dizer "medi" sem ter medido — mesma classe que esta campanha
	// corrige. t.Skip em vez de t.Fatal: "never fails" continua válido (hades-tf R-A, 2026-09-22).
	if total == 0 {
		t.Skipf("done/ corpus vazio (%s) — diretório existe mas não contém arquivos .md; "+
			"sparse-checkout ou checkout incompleto? Corpus measurement skipped.", doneDir)
	}

	t.Logf("done/ corpus: total=%d, unfinished(StatusIsComplete)=%d, unfinished(HasUnfinishedMLs)=%d",
		total, byStatusIsComplete, byThreeCategory)
	t.Logf("Corpus measurement (REQ #396): count is logged for CI visibility only; no specific number is asserted. See CORPUS MEASUREMENT NOTE in this file.")
}

// ── ML-1B: AC3-ter — WaveLabelRe case-insensitive suffix ──────────────────────

// TestWaveLabelRe_CaseInsensitiveSuffix verifies that the case-insensitive suffix
// fix accepts real labels like "3-Py" while still rejecting genuinely invalid ones.
// ML-1D (REQ #392) extends the valid set to include no-hyphen suffixes ("1b", "3b").
//
// AC11 (ML-1B + ML-1D): WaveLabelRe accepts "3-Py" and "1b" after the grammar fixes,
// and continues to reject "abc", "reaberta", "X" (no digit prefix) — the counter-tests
// that prove the extensions do not accidentally admit all-letter labels.
func TestWaveLabelRe_CaseInsensitiveSuffix(t *testing.T) {
	valid := []string{
		"0", "1", "2-bis", "3", "3-Py", "3-py", "3-PY", "10-Hotfix", "0-A",
		// ML-1D additions: no-hyphen suffix forms from real roadmaps
		"1b", "3b", "10a2", "2bis",
	}
	invalid := []string{
		"abc", "reaberta", "-1", "Py", "3-", "",
		"X", // ML-1D counter-test: single upper-case letter with no digit is invalid
	}

	for _, label := range valid {
		if !WaveLabelRe.MatchString(label) {
			t.Errorf("WaveLabelRe.MatchString(%q) = false, want true", label)
		}
	}
	for _, label := range invalid {
		if WaveLabelRe.MatchString(label) {
			t.Errorf("WaveLabelRe.MatchString(%q) = true, want false (counter-test: invalid label must stay rejected)", label)
		}
	}
}

// TestParseWaves_FailSafeIsUnfinished confirms that a malformed wave heading causes
// HasUnfinishedMLs to return true (fail-safe closed, Ação 3).
//
// AC11 (ML-1B, updated for ML-1D): ParseWaves reports "## Wave abc" (letter-only label)
// as a MalformedWave; HasUnfinishedMLs propagates this as true — a roadmap with a
// malformed heading cannot be released to done because completeness cannot be proven.
// ML-1D changed the return type from ([]WaveBlock, error) to ([]WaveBlock, []MalformedWave)
// so parsing continues for the rest of the document; the fail-safe is preserved.
func TestParseWaves_FailSafeIsUnfinished(t *testing.T) {
	// "## Wave abc" is still invalid after the AC3-ter and ML-1D fixes (no digit prefix).
	content := "# Roadmap\n\n## Wave abc — Invalid heading\n\n### ML-1A — Work\n**Status:** ✅ Concluído\n**Acceptance criteria:**\n- [x] done\n"

	lines := SplitRoadmapLines(content)
	_, malformed := ParseWaves(lines)
	if len(malformed) == 0 {
		t.Fatal("ParseWaves returned no MalformedWave for '## Wave abc', want one — counter-test: invalid label must stay reported")
	}
	// The malformed entry must name the line (line 3, 1-based) and the token.
	if malformed[0].Token != "abc" {
		t.Errorf("MalformedWave.Token = %q, want %q", malformed[0].Token, "abc")
	}
	if malformed[0].Line != 3 {
		t.Errorf("MalformedWave.Line = %d, want 3", malformed[0].Line)
	}

	if !HasUnfinishedMLs(content) {
		t.Fatal("HasUnfinishedMLs = false for roadmap with malformed wave heading, want true — fail-safe must treat malformed wave as unfinished")
	}
}

// TestCompareWaveLabels_CaseNeutral verifies that "3-Py" and "3-py" sort as equal,
// and that the overall ordering is unaffected by suffix casing.
//
// AC11 (ML-1B): CompareWaveLabels normalizes the suffix to lower-case before
// comparison, so "3-Py" == "3-py" in ordering — authors who use mixed-case
// suffixes get the same sort position as those who use lower-case.
func TestCompareWaveLabels_CaseNeutral(t *testing.T) {
	// "3-Py" and "3-py" must compare as equal.
	if got := CompareWaveLabels("3-Py", "3-py"); got != 0 {
		t.Errorf("CompareWaveLabels(\"3-Py\", \"3-py\") = %d, want 0 (case-neutral)", got)
	}
	if got := CompareWaveLabels("3-py", "3-Py"); got != 0 {
		t.Errorf("CompareWaveLabels(\"3-py\", \"3-Py\") = %d, want 0 (case-neutral)", got)
	}

	// Overall ordering must be preserved: "3" < "3-py" < "3-Py" is WRONG after fix;
	// both "3-py" and "3-Py" must sort the same relative to "3" and "4".
	if got := CompareWaveLabels("3", "3-Py"); got >= 0 {
		t.Errorf("CompareWaveLabels(\"3\", \"3-Py\") = %d, want < 0 (no-suffix before with-suffix)", got)
	}
	if got := CompareWaveLabels("3-Py", "4"); got >= 0 {
		t.Errorf("CompareWaveLabels(\"3-Py\", \"4\") = %d, want < 0 (integer part dominates)", got)
	}
}

// ── ML-1D: no-hyphen suffix grammar and cascade isolation ────────────────────

// TestSplitWaveLabel_NoHyphen verifies that SplitWaveLabel correctly parses the
// no-hyphen suffix form introduced by ML-1D.
//
// AC11 (ML-1D): SplitWaveLabel("1b") returns (1, "b"), not (0, "") — the no-hyphen
// suffix form is split correctly so that CompareWaveLabels can normalise "1b" == "1-b".
func TestSplitWaveLabel_NoHyphen(t *testing.T) {
	cases := []struct {
		label   string
		wantInt int
		wantSuf string
	}{
		{"1b", 1, "b"},
		{"3b", 3, "b"},
		{"10a2", 10, "a2"},
		{"2bis", 2, "bis"},
	}
	for _, c := range cases {
		gotInt, gotSuf := SplitWaveLabel(c.label)
		if gotInt != c.wantInt || gotSuf != c.wantSuf {
			t.Errorf("SplitWaveLabel(%q) = (%d, %q), want (%d, %q)",
				c.label, gotInt, gotSuf, c.wantInt, c.wantSuf)
		}
	}
}

// TestSplitWaveLabel_Invariant_PreviouslyValidLabels proves that the ML-1D rewrite of
// SplitWaveLabel does not change the output for any label form valid before ML-1D.
//
// AC11 (ML-1D): for digit-only and digit-hyphen-suffix labels, SplitWaveLabel returns
// the same (integer, suffix) as the original implementation — no preexisting corpus pin
// line can be reclassified by the parser rewrite.
func TestSplitWaveLabel_Invariant_PreviouslyValidLabels(t *testing.T) {
	cases := []struct {
		label   string
		wantInt int
		wantSuf string
	}{
		// digit-only
		{"0", 0, ""},
		{"1", 1, ""},
		{"2", 2, ""},
		{"10", 10, ""},
		// digit-hyphen-suffix (the form valid before ML-1D)
		{"2-bis", 2, "bis"},
		{"3-py", 3, "py"},
		{"3-Py", 3, "Py"},
		{"10-a2", 10, "a2"},
		{"0-A", 0, "A"},
	}
	for _, c := range cases {
		gotInt, gotSuf := SplitWaveLabel(c.label)
		if gotInt != c.wantInt || gotSuf != c.wantSuf {
			t.Errorf("SplitWaveLabel(%q) = (%d, %q), want (%d, %q) — invariant: ML-1D must not change pre-existing label parsing",
				c.label, gotInt, gotSuf, c.wantInt, c.wantSuf)
		}
	}
}

// TestCompareWaveLabels_NoHyphenEquivalence verifies that "1b" and "1-b" compare as equal.
//
// AC11 (ML-1D): CompareWaveLabels("1b", "1-b") == 0 — both forms normalise to (1, "b")
// via SplitWaveLabel, so --wave 1-b resolves a document heading "## Wave 1b" and vice versa.
func TestCompareWaveLabels_NoHyphenEquivalence(t *testing.T) {
	if got := CompareWaveLabels("1b", "1-b"); got != 0 {
		t.Errorf("CompareWaveLabels(\"1b\", \"1-b\") = %d, want 0 (no-hyphen == hyphen form)", got)
	}
	if got := CompareWaveLabels("1-b", "1b"); got != 0 {
		t.Errorf("CompareWaveLabels(\"1-b\", \"1b\") = %d, want 0 (symmetric)", got)
	}
	// Additional forms
	if got := CompareWaveLabels("2bis", "2-bis"); got != 0 {
		t.Errorf("CompareWaveLabels(\"2bis\", \"2-bis\") = %d, want 0", got)
	}
}

// TestCompareWaveLabels_Ordering_NoHyphenSuffix proves that no-hyphen suffix labels sort
// between the bare integer and the next integer, matching the hyphen-suffix ordering.
//
// AC11 (ML-1D): "1" < "1b" < "2" in CompareWaveLabels — the no-hyphen form does not
// sort before the bare integer (which would break wave ordering), and it sorts before
// the next integer (consistent with "1" < "1-b" < "2" from the hyphen form).
func TestCompareWaveLabels_Ordering_NoHyphenSuffix(t *testing.T) {
	if got := CompareWaveLabels("1", "1b"); got >= 0 {
		t.Errorf("CompareWaveLabels(\"1\", \"1b\") = %d, want < 0 (bare integer before suffixed)", got)
	}
	if got := CompareWaveLabels("1b", "2"); got >= 0 {
		t.Errorf("CompareWaveLabels(\"1b\", \"2\") = %d, want < 0 (suffixed before next integer)", got)
	}
	// Verify "1b" and "1-b" occupy the same position relative to neighbours.
	if got := CompareWaveLabels("1b", "1-b"); got != 0 {
		t.Errorf("CompareWaveLabels(\"1b\", \"1-b\") = %d, want 0 (same position)", got)
	}
}

// TestParseWaves_CascadeIsolated proves that a document with a malformed wave label
// and a valid wave returns both a non-empty []MalformedWave and a non-empty []WaveBlock.
//
// AC11 (ML-1D): a malformed wave heading is isolated — ParseWaves reports it in
// []MalformedWave and continues to parse the rest of the document — reversing the
// cascade-abort behaviour of ADR-2026-07-29 decision 16. The malformed wave is named
// with its line number; the valid wave is fully available for evaluation.
func TestParseWaves_CascadeIsolated(t *testing.T) {
	// Document: Wave 1 is valid and green; Wave reaberta is malformed (no digit prefix).
	content := strings.Join([]string{
		"# Roadmap: Cascade Test",
		"",
		"## Wave 1 — Valid Wave",
		"",
		"### ML-1A — Work",
		"**Status:** ✅ Concluído",
		"**Acceptance criteria:**",
		"- [x] done",
		"",
		"## Wave reaberta — Malformed Label",   // line 10
		"",
		"### ML-R — Unreachable",
		"**Status:** ✅",
	}, "\n")

	lines := SplitRoadmapLines(content)
	waves, malformed := ParseWaves(lines)

	// At least one valid wave must be returned.
	if len(waves) == 0 {
		t.Fatal("ParseWaves returned no valid waves — cascade isolation failed: malformed wave must not abort valid ones")
	}
	if waves[0].Label != "1" {
		t.Errorf("expected first valid wave label \"1\", got %q", waves[0].Label)
	}

	// Exactly one malformed wave must be reported.
	if len(malformed) == 0 {
		t.Fatal("ParseWaves returned no MalformedWave — malformed heading not reported")
	}
	if malformed[0].Token != "reaberta" {
		t.Errorf("MalformedWave.Token = %q, want \"reaberta\"", malformed[0].Token)
	}
	// Verify the error message names the line (line 10, 1-based).
	msg := malformed[0].Error()
	if !strings.Contains(msg, "line 10") {
		t.Errorf("MalformedWave.Error() = %q, want it to contain \"line 10\"", msg)
	}
	if !strings.Contains(msg, "not a valid wave label") {
		t.Errorf("MalformedWave.Error() = %q, want it to contain \"not a valid wave label\"", msg)
	}

	// HasUnfinishedMLs must return true (fail-safe: malformed wave means completeness is unproven).
	if !HasUnfinishedMLs(content) {
		t.Fatal("HasUnfinishedMLs = false for roadmap with malformed wave, want true (fail-safe closed)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// AC7, AC7-bis, AC8 tests (ML-3B, REQ #392)
// ─────────────────────────────────────────────────────────────────────────────

// AC11 reconciliation for ML-3B/ML-4D tests (one sentence per test):
//
//   TestWave0HasPlaceholderOrMissingGate_ArmA_ExitOneIntact
//     Affirms: Wave0HasPlaceholderOrMissingGate returns true when Wave 0's gate block
//     contains only "exit 1  # placeholder gate" AND at least one ML is non-pending —
//     the canonical template placeholder with started work prevents the transition (AC7 arm a).
//
//   TestWave0HasPlaceholderOrMissingGate_ArmB_BlockDeleted
//     Affirms: Wave0HasPlaceholderOrMissingGate returns true when Wave 0 has a
//     ## Wave 0 heading but no **Gates da wave:** block at all AND work has started —
//     the naïve discriminant ("is exit 1 present?") would pass here; the correct
//     discriminant ("lost the gate the template gave") catches it (AC7 arm b).
//
//   TestWave0HasPlaceholderOrMissingGate_ArmC_RealGate
//     Affirms: Wave0HasPlaceholderOrMissingGate returns false when Wave 0's gate block
//     contains at least one command that does not start with "exit 1" — a real gate
//     does not trigger the violation (AC7 arm c, counter-arm).
//
//   TestHasWave0_Present
//     Affirms: HasWave0 returns true for a document that contains "## Wave 0 — ..." —
//     the AC7-bis predicate correctly identifies the Wave 0 heading.
//
//   TestHasWave0_Absent
//     Affirms: HasWave0 returns false for a document that has no ## Wave 0 heading —
//     the AC7-bis predicate requires a Wave 0 heading to return true.
//
//   TestHasWave0_RenamedHeading_MissesDetection (AC7-bis falsification)
//     Affirms: renaming "## Wave 0 — X" to "## X" (removing the Wave heading entirely)
//     makes HasWave0 return false — this is the AC7-bis violation the rule catches, and
//     it is distinct from the gate-coverage check (AC7), as designed.
//
//   TestDuplicateWaveOrMLLabels_WaveDuplicate
//     Affirms: DuplicateWaveOrMLLabels returns a non-empty slice when two "## Wave 0"
//     headings appear in the same document — the violation that barrier.go:877-882
//     (break-on-first-match) previously silenced is now named.
//
//   TestDuplicateWaveOrMLLabels_MLDuplicate
//     Affirms: DuplicateWaveOrMLLabels returns a non-empty slice when two "### ML-1A"
//     headings appear in the document — the same silent-duplicate scenario for ML labels.
//
//   TestDuplicateWaveOrMLLabels_NoDuplicate
//     Affirms: DuplicateWaveOrMLLabels returns an empty slice for a well-formed roadmap
//     with unique Wave and ML labels — the predicate does not false-alarm on clean input.
//
//   TestDuplicateWaveOrMLLabels_FencedExampleIgnored
//     Affirms: a "## Wave 0" line inside a fenced code block is not counted as a real
//     Wave heading, so a document with one real ## Wave 0 and one fenced example does
//     not trigger a duplicate violation (fence-awareness, ADR-2026-08-22).

func TestWave0HasPlaceholderOrMissingGate_ArmA_ExitOneIntact(t *testing.T) {
	// AC7 arm (a): exit 1 placeholder intact → violation.
	content := strings.Join([]string{
		"# Roadmap: Gate Test",
		"",
		"## Wave 0 — Threat Model",
		"",
		"### ML-0A — Threat model",
		"**Status:** ✅ Concluído",
		"",
		"**Gates da wave:**",
		"```bash",
		"exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md",
		"```",
	}, "\n")

	if !Wave0HasPlaceholderOrMissingGate(content) {
		t.Fatal("Wave0HasPlaceholderOrMissingGate = false for intact exit 1 placeholder, want true (AC7 arm a)")
	}
}

func TestWave0HasPlaceholderOrMissingGate_ArmB_BlockDeleted(t *testing.T) {
	// AC7 arm (b): **Gates da wave:** block entirely deleted → violation.
	// This is the gap that kills the naïve discriminant ("is exit 1 present?"):
	// the naïve check would pass here, but the correct one must reprove.
	content := strings.Join([]string{
		"# Roadmap: Gate Test",
		"",
		"## Wave 0 — Threat Model",
		"",
		"### ML-0A — Threat model",
		"**Status:** ✅ Concluído",
		// No **Gates da wave:** block at all.
	}, "\n")

	if !Wave0HasPlaceholderOrMissingGate(content) {
		t.Fatal("Wave0HasPlaceholderOrMissingGate = false when **Gates da wave:** block deleted, want true (AC7 arm b)")
	}
}

func TestWave0HasPlaceholderOrMissingGate_ArmC_RealGate(t *testing.T) {
	// AC7 arm (c): gate replaced with a real command → no violation.
	content := strings.Join([]string{
		"# Roadmap: Gate Test",
		"",
		"## Wave 0 — Threat Model",
		"",
		"### ML-0A — Threat model",
		"**Status:** ✅ Concluído",
		"",
		"**Gates da wave:**",
		"```bash",
		"test -f docs/portabilidade/threat-model.md || { echo 'missing'; exit 1; }",
		"```",
	}, "\n")

	if Wave0HasPlaceholderOrMissingGate(content) {
		t.Fatal("Wave0HasPlaceholderOrMissingGate = true for real gate command, want false (AC7 arm c counter-arm)")
	}
}

// TestWave0HasPlaceholderOrMissingGate_ArmD_AllMLsPending affirms: when Wave 0
// has an exit 1 placeholder gate but ALL MLs in the document are ⬜ Pendente,
// Wave0HasPlaceholderOrMissingGate returns false — the placeholder is legitimate
// for a freshly scaffolded roadmap where ML-0A has not yet been worked (ML-4D
// discriminant, REQ #392: "ciclo limpo" case: roadmap new → move to wip → validate).
func TestWave0HasPlaceholderOrMissingGate_ArmD_AllMLsPending(t *testing.T) {
	content := strings.Join([]string{
		"# Roadmap: Fresh Scaffold",
		"",
		"## Wave 0 — Threat Model",
		"",
		"### ML-0A — Threat model",
		"**Status:** ⬜ Pendente",
		"",
		"**Gates da wave:**",
		"```bash",
		"exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md",
		"```",
		"",
		"## Wave 1 — Implementation",
		"",
		"### ML-1A — First task",
		"**Status:** ⬜ Pendente",
	}, "\n")

	if Wave0HasPlaceholderOrMissingGate(content) {
		t.Fatal("Wave0HasPlaceholderOrMissingGate = true for fresh scaffold with all MLs pending, want false " +
			"(ML-4D discriminant: placeholder gate is legitimate while no ML has moved past pending)")
	}
}

// TestWave0HasPlaceholderOrMissingGate_ArmE_OneNonPendingML affirms: when Wave 0
// has an exit 1 placeholder gate AND at least one ML is non-pending (✅ Concluído),
// Wave0HasPlaceholderOrMissingGate returns true — work has started and ML-0A should
// have replaced the gate by now (ML-4D discriminant, REQ #392: counter-arm proving
// the early-return did not disable the rule for roadmaps with started work).
func TestWave0HasPlaceholderOrMissingGate_ArmE_OneNonPendingML(t *testing.T) {
	content := strings.Join([]string{
		"# Roadmap: Work Started",
		"",
		"## Wave 0 — Threat Model",
		"",
		"### ML-0A — Threat model",
		"**Status:** ⬜ Pendente",
		"",
		"**Gates da wave:**",
		"```bash",
		"exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md",
		"```",
		"",
		"## Wave 1 — Implementation",
		"",
		"### ML-1A — First task",
		"**Status:** ✅ Concluído",
	}, "\n")

	if !Wave0HasPlaceholderOrMissingGate(content) {
		t.Fatal("Wave0HasPlaceholderOrMissingGate = false when ML-1A is complete and Wave 0 gate is placeholder, want true " +
			"(ML-4D contra-braço: the rule still fires when work has started)")
	}
}

func TestHasWave0_Present(t *testing.T) {
	content := strings.Join([]string{
		"# Roadmap: Test",
		"## Wave 0 — Threat Model",
		"Some content.",
		"## Wave 1 — Implementation",
	}, "\n")

	if !HasWave0(content) {
		t.Fatal("HasWave0 = false for document with ## Wave 0 heading, want true")
	}
}

func TestHasWave0_Absent(t *testing.T) {
	content := strings.Join([]string{
		"# Roadmap: Test",
		"## Wave 1 — Implementation",
		"No Wave 0 here.",
	}, "\n")

	if HasWave0(content) {
		t.Fatal("HasWave0 = true for document without ## Wave 0 heading, want false")
	}
}

func TestHasWave0_RenamedHeading_MissesDetection(t *testing.T) {
	// AC7-bis falsification: renaming "## Wave 0 — X" to "## X" removes the Wave
	// heading from the parser's view — HasWave0 returns false, which is the violation
	// the AC7-bis rule is designed to catch.
	contentWithWave0 := "## Wave 0 — Threat Model\nSome ML."
	contentRenamed := "## Threat Model\nSame content, heading renamed."

	if !HasWave0(contentWithWave0) {
		t.Fatal("HasWave0 = false for ## Wave 0 heading, want true (test setup)")
	}
	if HasWave0(contentRenamed) {
		t.Fatal("HasWave0 = true after Wave 0 heading renamed away, want false (AC7-bis violation)")
	}
}

func TestDuplicateWaveOrMLLabels_WaveDuplicate(t *testing.T) {
	// AC8: two ## Wave 0 headings → duplicate violation.
	// Before this change barrier.go:877-882 (break-on-first-match) silently ignored
	// the second copy; now the duplicate is named.
	content := strings.Join([]string{
		"# Roadmap: Duplicate Test",
		"## Wave 0 — First copy",
		"### ML-0A — First ML-0A",
		"**Status:** ✅ Concluído",
		"## Wave 0 — Second copy (scaffold residual)",
		"### ML-0A — Second ML-0A",
		"**Status:** ⬜ Pendente",
	}, "\n")

	msgs := DuplicateWaveOrMLLabels(content)
	if len(msgs) == 0 {
		t.Fatal("DuplicateWaveOrMLLabels = empty for document with two ## Wave 0 headings, want violation")
	}
	found := false
	for _, m := range msgs {
		if strings.Contains(m, `"0"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("DuplicateWaveOrMLLabels msgs %v do not mention Wave label \"0\"", msgs)
	}
}

func TestDuplicateWaveOrMLLabels_MLDuplicate(t *testing.T) {
	// AC8: two ### ML-1A headings → duplicate violation.
	content := strings.Join([]string{
		"# Roadmap: Duplicate ML Test",
		"## Wave 1 — Wave",
		"### ML-1A — First",
		"**Status:** ✅ Concluído",
		"### ML-1A — Second (scaffold residual)",
		"**Status:** ⬜ Pendente",
	}, "\n")

	msgs := DuplicateWaveOrMLLabels(content)
	if len(msgs) == 0 {
		t.Fatal("DuplicateWaveOrMLLabels = empty for document with two ### ML-1A headings, want violation")
	}
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "ML-1A") {
			found = true
		}
	}
	if !found {
		t.Fatalf("DuplicateWaveOrMLLabels msgs %v do not mention ML-1A", msgs)
	}
}

func TestDuplicateWaveOrMLLabels_NoDuplicate(t *testing.T) {
	// AC8 counter-arm: unique labels → no violation.
	content := strings.Join([]string{
		"# Roadmap: Clean",
		"## Wave 0 — Threat Model",
		"### ML-0A — Threat model",
		"**Status:** ✅ Concluído",
		"## Wave 1 — Implementation",
		"### ML-1A — Implementation",
		"**Status:** ✅ Concluído",
	}, "\n")

	msgs := DuplicateWaveOrMLLabels(content)
	if len(msgs) != 0 {
		t.Fatalf("DuplicateWaveOrMLLabels = %v for clean roadmap, want empty", msgs)
	}
}

func TestDuplicateWaveOrMLLabels_FencedExampleIgnored(t *testing.T) {
	// Fence-awareness: a "## Wave 0" line inside a fenced code block must not be
	// counted as a real Wave heading, so the document has only one real Wave 0.
	content := strings.Join([]string{
		"# Roadmap: Fence Test",
		"## Wave 0 — Threat Model",
		"### ML-0A — Threat model",
		"**Status:** ✅ Concluído",
		"",
		"Here is an example:",
		"```bash",
		"## Wave 0 — this is inside a fence and must be ignored",
		"echo 'not a heading'",
		"```",
		"",
		"No duplicate here.",
	}, "\n")

	msgs := DuplicateWaveOrMLLabels(content)
	if len(msgs) != 0 {
		t.Fatalf("DuplicateWaveOrMLLabels = %v for fenced example, want empty (fence-aware)", msgs)
	}
}
