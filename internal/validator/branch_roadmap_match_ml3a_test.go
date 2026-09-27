package validator

// ML-3A (REQ-2026-09-09, AC12+AC13+AC15) — the branch↔roadmap matcher, in ADDITIVE mode.
// Implements ADR-2026-09-26-precisao-do-vinculo-branch-roadmap-escrever-em-vez-de-inferir.
//
// Reconciliation (Regra Dura de Reconciliação) — each test below carries, in its own doc comment,
// the ONE sentence stating which conclusion of the ML-3A report it affirms.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// corpusDir writes the given roadmap filenames into a temp directory and returns it.
func corpusDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("# roadmap\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Affirms: measurement D of the report — an empty branch slug used to report matched=true vacuously
// (strings.Contains(x, "") is always true) and now matches 0 roadmaps while still listing every
// candidate for the diagnostic message.
func TestMatchRoadmaps_EmptySlugMatchesNothing(t *testing.T) {
	dir := corpusDir(t,
		"ROADMAP-2026-09-09-primeiro.md",
		"ROADMAP-2026-09-10-segundo.md",
	)
	matches, candidates := MatchRoadmapsForBranchSlug("", []string{dir}, nil)
	if len(matches) != 0 {
		t.Fatalf("empty slug must match no roadmap, got %v", matches)
	}
	if len(candidates) != 2 {
		t.Fatalf("empty slug must still report every candidate for diagnostics, got %v", candidates)
	}
	if matched, _ := BranchSlugMatchesRoadmap("   ", []string{dir}, nil); matched {
		t.Fatal("blank slug must not match either — whitespace normalizes to empty")
	}
}

// Affirms: measurement C of the report — the #273 pair
// feat/adrs-retroativas-da-divida-do-acervo × ROADMAP-2026-09-05-divida-de-governanca-do-acervo-…
// shares EXACTLY 2 content tokens (divida, acervo), is rejected by the substring arm, and is now
// accepted by the token-overlap arm — the restrito-demais direction of the defect closes.
func TestMatchRoadmaps_Issue273CaseAccepted(t *testing.T) {
	const roadmap = "ROADMAP-2026-09-05-divida-de-governanca-do-acervo-que-nasceu-sem-adr.md"
	dir := corpusDir(t, roadmap)
	slug := NormalizeBranchSlug("adrs-retroativas-da-divida-do-acervo")

	if strings.Contains(normalizeBranchSlug(roadmap), slug) {
		t.Fatal("precondition broken: this pair must NOT be a substring match, or the test proves nothing")
	}
	shared := sharedTokenCount(branchRoadmapTokens(slug), branchRoadmapTokens(roadmapContentSlug(roadmap)))
	if shared != 2 {
		t.Fatalf("the #273 pair must share exactly 2 content tokens (this is what caps the threshold at 2), got %d", shared)
	}
	matched, _ := BranchSlugMatchesRoadmap(slug, []string{dir}, nil)
	if !matched {
		t.Fatal("AC15: a legitimately governed branch whose slug is not a substring of the roadmap must be accepted")
	}
}

// Affirms: the structural half of the zero-regression claim — the substring arm is kept verbatim, so
// a pair that ONLY substring accepts (single generic token: 1 shared token, below the threshold)
// keeps being accepted. This is what makes the change reversible without manual git surgery.
func TestMatchRoadmaps_SubstringArmPreserved(t *testing.T) {
	const roadmap = "ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md"
	dir := corpusDir(t, roadmap)
	slug := NormalizeBranchSlug("gates")

	if n := sharedTokenCount(branchRoadmapTokens(slug), branchRoadmapTokens(roadmapContentSlug(roadmap))); n >= branchRoadmapMinSharedTokens {
		t.Fatalf("precondition broken: this pair must be below the token threshold (got %d) so that only the substring arm can accept it", n)
	}
	matched, _ := BranchSlugMatchesRoadmap(slug, []string{dir}, nil)
	if !matched {
		t.Fatal("AC13: a branch the historical substring relation accepted must keep being accepted")
	}
}

// Affirms: the raw-vs-stripped measurement of the report — tokenizing the filename raw gives every
// roadmap a free "roadmap" token, and the ONE historical branch that buys (feat/roadmap-list, via
// ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md) is a spurious acceptance; stripping
// the structural prefix keeps it rejected.
func TestMatchRoadmaps_StructuralPrefixIsNotAContentToken(t *testing.T) {
	const roadmap = "ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md"
	dir := corpusDir(t, roadmap)
	slug := NormalizeBranchSlug("roadmap-list")

	// Only the STRUCTURAL kind prefix and the ISO date are stripped. The "req" here is part of the
	// TITLE ("req move list subpastas") and survives — stripping is positional, not a blacklist.
	if got := roadmapContentSlug(roadmap); got != "req-move-list-subpastas-e-move-fisico" {
		t.Fatalf("roadmapContentSlug must strip kind prefix + ISO date + .md, got %q", got)
	}
	rawShared := sharedTokenCount(branchRoadmapTokens(slug), branchRoadmapTokens(normalizeBranchSlug(roadmap)))
	if rawShared < branchRoadmapMinSharedTokens {
		t.Fatalf("precondition broken: raw tokenization must reach the threshold here (got %d), otherwise the test does not exercise the stripping", rawShared)
	}
	if matched, _ := BranchSlugMatchesRoadmap(slug, []string{dir}, nil); matched {
		t.Fatal("the structural 'roadmap' prefix must not count as a shared content token")
	}

	// Contra-arm: a roadmap legitimately TITLED "roadmap …" keeps its content token, so stripping is
	// prefix removal and not a word blacklist.
	titled := corpusDir(t, "ROADMAP-2026-09-26-roadmap-move-com-nome-vazio-move-o-primeiro.md")
	if matched, _ := BranchSlugMatchesRoadmap(NormalizeBranchSlug("roadmap-move-nome-vazio"), []string{titled}, nil); !matched {
		t.Fatal("a roadmap whose TITLE contains 'roadmap' must still match on that token")
	}
}

// Affirms: the reporter's own contra-arm in #273 — fix/consistencias-6-a-9 is a badly named branch,
// not a defect of the rule: it shares no token of 3+ characters with any roadmap and stays rejected,
// so the token arm is not a blanket acceptance.
func TestMatchRoadmaps_BadlyNamedBranchStaysRejected(t *testing.T) {
	dir := corpusDir(t,
		"ROADMAP-2026-09-05-divida-de-governanca-do-acervo-que-nasceu-sem-adr.md",
		"ROADMAP-2026-08-30-aspas-nao-pareadas-em-itens-de-lista.md",
	)
	if matched, _ := BranchSlugMatchesRoadmap(NormalizeBranchSlug("consistencias-6-a-9"), []string{dir}, nil); matched {
		t.Fatal("a branch sharing no content token must stay rejected — the token arm is not a blanket accept")
	}
}

// Affirms: the calibration of the report — the threshold is 2 because both bounds are forced: 1
// shared token must not accept (at N=1 the relation accepted 205 of 205 historical branches and
// admitted ~10% of the corpus for a single generic word) and 2 must accept (at N=3 the #273 case
// fails). The constant is the measured value, not a preference.
func TestMatchRoadmaps_ThresholdIsExactlyTwo(t *testing.T) {
	if branchRoadmapMinSharedTokens != 2 {
		t.Fatalf("threshold changed to %d — re-run the ML-3A calibration before changing it: N=1 is verdict-vacuous on the historical corpus and N>=3 reopens the #273 false negative", branchRoadmapMinSharedTokens)
	}
	one := corpusDir(t, "ROADMAP-2026-09-05-divida-de-governanca-do-tempo.md")
	if matched, _ := BranchSlugMatchesRoadmap(NormalizeBranchSlug("retroativas-da-divida-antiga"), []string{one}, nil); matched {
		t.Fatal("ONE shared content token must not be enough (floor of the calibration)")
	}
	two := corpusDir(t, "ROADMAP-2026-09-05-divida-de-governanca-do-acervo.md")
	if matched, _ := BranchSlugMatchesRoadmap(NormalizeBranchSlug("retroativas-da-divida-do-acervo"), []string{two}, nil); !matched {
		t.Fatal("TWO shared content tokens must be enough (ceiling of the calibration, forced by #273)")
	}
}

// Affirms: the contra-arm of AC13 — legitimate RESUMPTION of a roadmap already in done/ keeps
// working, including through the new token-overlap arm, so the matcher did not become wip/-only.
func TestMatchRoadmaps_DoneDirResumptionStillMatches(t *testing.T) {
	done := corpusDir(t, "ROADMAP-2026-09-05-divida-de-governanca-do-acervo.md")
	matched, candidates := BranchSlugMatchesRoadmap(NormalizeBranchSlug("retroativas-da-divida-do-acervo"), nil, []string{done})
	if !matched {
		t.Fatal("a roadmap in done/ must keep governing a resumed branch")
	}
	if len(candidates) != 1 {
		t.Fatalf("done/ candidates must be listed, got %v", candidates)
	}
}

// --- D1: the WRITTEN link ---------------------------------------------------------------------

func linkFixture(t *testing.T, roadmapsInWip []string, links string) config.ProjectConfig {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs/roadmaps/wip"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs/roadmaps/done"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range roadmapsInWip {
		if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip", n), []byte("# r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if links != "" {
		if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps", BranchLinkFileName), []byte(links), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)
	return config.Load()
}

// Affirms: D1 of the ADR — the written link is the SOURCE of truth: a branch whose name shares
// nothing with its roadmap (so inference rejects it) is governed because `branch new` wrote the link
// down at the instant it knew.
func TestResolveBranchRoadmap_WrittenLinkIsTheSource(t *testing.T) {
	cfg := linkFixture(t,
		[]string{"ROADMAP-2026-09-26-tema-totalmente-diferente.md"},
		`{"version":1,"links":{"feat/nada-em-comum":"ROADMAP-2026-09-26-tema-totalmente-diferente.md"}}`)

	wip := ResolveWIPDirs(cfg)
	done := ResolveDoneDirs(cfg)
	if matched, _ := BranchSlugMatchesRoadmap(NormalizeBranchSlug("nada-em-comum"), wip, done); matched {
		t.Fatal("precondition broken: inference must reject this pair, or the test does not prove the link is the source")
	}
	res := ResolveBranchRoadmap(cfg, "feat/nada-em-comum", wip, done)
	if !res.Matched || res.Source != "written-link" {
		t.Fatalf("written link must govern: %+v", res)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("a valid link must be silent, got %v", res.Warnings)
	}
}

// Affirms: the declared negative consequence of the ADR — a link whose target left wip/+done/ is
// STALE, and the answer is fallback to inference plus a warning that NAMES the stale target; the
// answer the ADR forbids is a silent fallback.
func TestResolveBranchRoadmap_StaleLinkWarnsAndFallsBack(t *testing.T) {
	cfg := linkFixture(t,
		[]string{"ROADMAP-2026-09-26-divida-do-acervo.md"},
		`{"version":1,"links":{"feat/retroativas-da-divida-do-acervo":"ROADMAP-2026-01-01-que-saiu-de-wip.md"}}`)

	res := ResolveBranchRoadmap(cfg, "feat/retroativas-da-divida-do-acervo", ResolveWIPDirs(cfg), ResolveDoneDirs(cfg))
	if !res.Matched || res.Source != "inference" {
		t.Fatalf("stale link must fall back to inference: %+v", res)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "ROADMAP-2026-01-01-que-saiu-de-wip.md") {
		t.Fatalf("the fallback must not be silent and must name the stale target, got %v", res.Warnings)
	}
}

// Affirms: D4 (additive order) applied to the new state surface — a stale link produces a WARNING
// and zero violations, so the new written-link surface cannot make a branch that passes today start
// failing.
func TestValidateBranchHasWIPRoadmap_StaleLinkIsWarningNeverViolation(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "feat/divida-do-acervo")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-2026-09-26-divida-do-acervo.md", "REQ: REQ-001\n")
	writeFile(t, dir, "docs/roadmaps/"+BranchLinkFileName,
		`{"version":1,"links":{"feat/divida-do-acervo":"ROADMAP-2026-01-01-sumiu.md"}}`)
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("a stale link must never become a violation, got %v", violations)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "branch_link_stale") {
		t.Fatalf("expected exactly one branch_link_stale warning, got %v", warnings)
	}
}

// Affirms: ML-1C's decision carried into the new surface — with more than one matching roadmap there
// is no single truth to write, so RecordBranchLink records NOTHING instead of picking one by scan
// order, which is the defect ML-1C removed from findRoadmap.
func TestRecordBranchLink_AmbiguousInferenceRecordsNothing(t *testing.T) {
	cfg := linkFixture(t, []string{
		"ROADMAP-2026-09-26-divida-do-acervo-parte-um.md",
		"ROADMAP-2026-09-27-divida-do-acervo-parte-dois.md",
	}, "")
	if err := RecordBranchLink(cfg, "feat/divida-do-acervo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(BranchLinkPath(cfg)); !os.IsNotExist(err) {
		t.Fatalf("no link file should exist for an ambiguous inference (err=%v)", err)
	}

	// Contra-arm: exactly one match → the link IS recorded, and it names that roadmap.
	cfg2 := linkFixture(t, []string{"ROADMAP-2026-09-26-divida-do-acervo-parte-um.md"}, "")
	if err := RecordBranchLink(cfg2, "feat/divida-do-acervo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := BranchLinkFor(cfg2, "feat/divida-do-acervo", ResolveWIPDirs(cfg2), ResolveDoneDirs(cfg2))
	if !status.InScope || status.Roadmap != "ROADMAP-2026-09-26-divida-do-acervo-parte-um.md" {
		t.Fatalf("unique inference must be recorded verbatim, got %+v", status)
	}
}

// Affirms: the link degrades to inference instead of blocking — a corrupt link file is an
// accelerator failure, and the gate it feeds is on the critical path of every commit and ship.
func TestBranchLinkFor_CorruptFileDegradesToNoLink(t *testing.T) {
	cfg := linkFixture(t, []string{"ROADMAP-2026-09-26-divida-do-acervo.md"}, "{not json at all")
	status := BranchLinkFor(cfg, "feat/qualquer", ResolveWIPDirs(cfg), ResolveDoneDirs(cfg))
	if status.Present {
		t.Fatalf("a corrupt link file must read as 'no link', got %+v", status)
	}
}
