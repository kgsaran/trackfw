package roadmapdoc

// roadmapdoc_test.go — unit tests for the roadmapdoc leaf package (ML-1A + ML-1B, REQ #392).
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
//     Affirms: documents the current corpus count without asserting a specific number,
//     because the corpus changes with every ML execution. This test NEVER fails; it exists
//     to surface the count in the CI log.
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
// CORPUS MEASUREMENT NOTE (updated by ML-1B):
//   ML-1A measured three counts (pre-fix):
//     byStatusIsComplete (ignoring ParseWaves errors): 27
//     HasUnfinishedMLs (fail-safe: ParseWaves error → unfinished): 30
//     Architect's baseline: 32 (corrected in REQ to 27)
//
//   ML-1B fixes WaveLabelRe to be case-insensitive. After this fix:
//   - "3-Py" is a VALID label; ParseWaves no longer errors on trackfw-update-command-2026-06-18.md.
//   - "## Wave reaberta" (no digit prefix) still fails WaveLabelRe and will cause ParseWaves
//     to error for ROADMAP-2026-09-01-caminho-dentro-de-artefato-versionado-usa-sempre-barra.md.
//     That heading was added after the baseline was captured and is not corrected in this ML.
//   - The 30→27 gap from ML-1A was caused by roadmaps with uppercase suffixes; after this fix,
//     HasUnfinishedMLs no longer erroneously returns true for those roadmaps.
//   - The gap between HasUnfinishedMLs and byStatusIsComplete should narrow after this fix.
//
//   No specific-number assertion is written; the count is logged only. The count will decrease
//   further when ML-4A cleans the 6 sítios measured in done/.
//
//   ML-1A note about "32–30 = 2 gap": the REQ corrected the architect's baseline to 27.
//   The "2-unit difference attributed to corpus drift" in the original note was incorrect —
//   the REQ explicitly states the correct number is 27 and that done/ had not changed.

import (
	"os"
	"path/filepath"
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
func TestCorpusMeasurement_ReportOnly(t *testing.T) {
	doneDir := filepath.Join(repoRoot(t), "docs", "roadmaps", "done")
	entries, err := os.ReadDir(doneDir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", doneDir, err)
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

	t.Logf("done/ corpus: total=%d, unfinished(StatusIsComplete)=%d, unfinished(HasUnfinishedMLs)=%d",
		total, byStatusIsComplete, byThreeCategory)
	t.Logf("Post-ML-1B note: count may remain 30 — trackfw-update-command has genuine pending MLs (⬜), so fixing WaveLabelRe changes the reason (fail-safe→genuine) but not the result. ROADMAP-2026-09-01 still has '## Wave reaberta' (letter-only label, not fixed by AC3-ter). See CORPUS MEASUREMENT NOTE in this file.")
}

// ── ML-1B: AC3-ter — WaveLabelRe case-insensitive suffix ──────────────────────

// TestWaveLabelRe_CaseInsensitiveSuffix verifies that the case-insensitive suffix
// fix accepts real labels like "3-Py" while still rejecting genuinely invalid ones.
//
// AC11 (ML-1B): WaveLabelRe accepts "3-Py" after the AC3-ter fix, and continues
// to reject "abc" (no digit prefix) — the counter-test that proves the regex
// extension does not accidentally admit all-letter labels.
func TestWaveLabelRe_CaseInsensitiveSuffix(t *testing.T) {
	valid := []string{"0", "1", "2-bis", "3", "3-Py", "3-py", "3-PY", "10-Hotfix", "0-A"}
	invalid := []string{"abc", "reaberta", "-1", "Py", "3-", ""}

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
// AC11 (ML-1B): ParseWaves returns an error for "## Wave abc" (letter-only label);
// HasUnfinishedMLs propagates this as true — a roadmap with a malformed heading
// cannot be released to done because its completeness cannot be proven.
func TestParseWaves_FailSafeIsUnfinished(t *testing.T) {
	// "## Wave abc" is still invalid after the AC3-ter fix (no digit prefix).
	content := "# Roadmap\n\n## Wave abc — Invalid heading\n\n### ML-1A — Work\n**Status:** ✅ Concluído\n**Acceptance criteria:**\n- [x] done\n"

	lines := SplitRoadmapLines(content)
	_, err := ParseWaves(lines)
	if err == nil {
		t.Fatal("ParseWaves returned nil error for '## Wave abc', want an error — counter-test: invalid label must stay rejected")
	}

	if !HasUnfinishedMLs(content) {
		t.Fatal("HasUnfinishedMLs = false for roadmap with malformed wave heading, want true — fail-safe must treat parse error as unfinished")
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
