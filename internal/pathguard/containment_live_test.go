package pathguard

// containment_live_test.go — the analyser of ML-7C applied to the tree it
// governs, with the two pinned per-site lists that keep T3 from dissolving it.
//
// # The two lists, and why they are two
//
// An exception that says "this is fine" and a record that says "this is a defect
// we measured and have not fixed" are different claims. Merging them is how a
// gate turns tautological: everything the analyser points at becomes "fine".
//
//	liveAnalyserBlindSpots — the guard IS there and IS correct; the analyser
//	    cannot join the two expressions (a value from a two-result call, a
//	    cross-function queue of pending writes, two sibling switch statements).
//	    Each entry names WHY the analyser cannot see it, and that reason is
//	    checkable by reading the site.
//
//	liveKnownFailOpen — measured DEFECTS. The guard is nested in
//	    `if root, err := resolver(); err == nil { … }` and the write sits outside
//	    it, so a resolver failure means the write proceeds unguarded. Five sites.
//	    🔴 These are NOT approved by being listed. They are counted, named, and
//	    handed to the architect: same cause, same REQ (CLAUDE.md's Regra Dura de
//	    Causa Raiz), so they belong to a new ML of REQ-2026-08-31, not to a new REQ.
//
// Both lists are keyed BY SITE (file + function + kind + path expression), never
// by file and never by pattern, and both carry an exact expected count. A new
// site cannot hide inside an existing entry, and an entry that stops matching is
// an error too — a stale exception reads as "the rule now passes".
//
// One sentence per test (Regra Dura de Reconciliação):
//
//   - TestContainmentAnalyserOverTheLiveTree affirms the ML's conclusion that every
//     P1 violation the analyser finds in today's tree is accounted for by exactly
//     one pinned entry — no unexplained finding, and no stale entry.
//   - TestP1PopulationIsNotVacuous affirms the ML's conclusion that the green above
//     is not "nothing to find": the corpus of files, write sites, sites in
//     population and guard calls are each pinned above zero, and the three
//     obsolescence modes fail.
//   - TestP2PointsAtTheUnresolvedRootsWithoutFixingThem affirms the ML's conclusion
//     that P2 sees the argument defect Wave 8 exists to close: the 16 sites whose
//     guard root is filepath.Clean are reported BY NAME, and the analyser leaves
//     them exactly as they are.
//   - TestAnalyserVocabularyIsPinned affirms the ML's conclusion that the analyser's
//     own reach is a counted list and not an assumption — shrinking the primitive,
//     guard or resolver vocabulary is the cheapest way to make it vacuous.

import (
	"sort"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Floors and pins, measured on 2026-09-25 over internal/ + cmd/
// ─────────────────────────────────────────────────────────────────────────────

const (
	liveFileFloor         = 90  // measured 107
	liveWriteSiteFloor    = 120 // measured 154
	livePopulationFloor   = 120 // measured 151
	liveGuardCallFloor    = 45  // measured 59
	liveP2UnresolvedPin   = 22  // measured; 16 of them are a literal filepath.Clean
	liveP2CleanSitesPin   = 16  // the Wave 8 population, by the mechanism's ruler
	liveRootAliasedPin    = 16
	liveBlindSpotSitesPin = 15
	liveFailOpenSitesPin  = 5
)

// liveAnalyserBlindSpots — site → number of write sites it covers → why the
// analyser cannot see the guard that IS there.
var liveAnalyserBlindSpots = map[string]blindSpot{
	"internal/generators/adr.go|NewADR|unguarded-write|adrDir": {1,
		"guarded by the probe path filepath.Join(absAdrDir, \".trackfw-new-adr\"), which walks adrDir and every ancestor; absAdrDir arrives from a two-result call so the analyser cannot prove it is the same chain as adrDir"},
	"internal/generators/adr.go|NewADRDraft|unguarded-write|adrDir": {1,
		"same shape as NewADR: the guard is on draftProbe = filepath.Join(absAdrDirDraft, \".trackfw-new-adr-draft\"), which walks adrDir and every ancestor, and absAdrDirDraft comes from a two-result call"},
	"internal/generators/scaffold.go|installGlobalSkillInner|unguarded-write|skillDir": {1,
		"guarded by filepath.Join(absHome, \".claude\", \"skills\", \"trackfw\"); the written path comes from GlobalClaudeSkillPath(home), a call the analyser cannot expand into components"},
	"internal/generators/scaffold.go|installGlobalSkillInner|unguarded-write|skillPath": {1,
		"same shape, with the leaf guard on filepath.Join(absHome, …, \"SKILL.md\")"},
	"internal/generators/scaffold.go|generateCommitMsgHook|unguarded-write|\".husky\"": {1,
		"guarded by rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, \".husky\")) in the FIRST `switch cfg.Hooks`; the write is in the SECOND switch over the same tag, and block-ancestry dominance does not relate two sibling switch statements (declared limit 4)"},
	"internal/generators/scaffold.go|generateCommitMsgHook|unguarded-write|scriptDir": {1,
		"same two-switch shape, guard on filepath.Join(cmhRoot, \".lefthook\", \"commit-msg\")"},
	"internal/generators/update.go|copyPath|unguarded-write|filepath.Dir(dst)": {2,
		"dst is always an os.MkdirTemp sandbox root (the site says so and the callers confirm it); it enters the population only through the conservative taint transfer, not through a project root"},
	"internal/generators/update.go|copyPath|unguarded-write|dst": {3,
		"same sandbox reason as filepath.Dir(dst) above: copyPath only ever writes into the os.MkdirTemp tree the update dry-run builds, so no user-supplied root reaches it"},
	"internal/integrations/manager.go|atomicWrite|unguarded-write|directory": {2,
		"containment is established by the caller through rejectSymlinks before the write is queued; the queue crosses a function boundary and one call site is inside a deferred closure, which walkWithBlocks does not enter"},
	"internal/integrations/manager.go|atomicWrite|unguarded-write|filename": {1,
		"same call-site containment as the directory writes above: rejectSymlinks runs on the destination in the caller, before the write is appended to pendingWrites"},
	"internal/metrics/metrics.go|ExportCSV|unguarded-write|path": {1,
		"the guard is deliberately conditional on pathguard.Beneath: an external absolute destination is a user-directed named exception of ADR-2026-09-18, not a root-derived path"},
}

// liveKnownFailOpen — MEASURED DEFECTS, not approvals. Same cause (the guard is
// conditional on the resolver succeeding, and the write is not), so by the Regra
// Dura de Causa Raiz they belong to a new ML of this same REQ.
var liveKnownFailOpen = map[string]blindSpot{
	"internal/generators/java.go|GeneratePomXML|unguarded-write|\"pom.xml\"": {1,
		"guard lives inside `if rootErr == nil`; when projectRoot() fails the write happens unguarded — RefuseUnverifiableRoot (built by ML-7B for exactly this) is the fix"},
	"internal/generators/note.go|appendNoteToIndex|unguarded-write|vaultIndexFile": {2,
		"guard lives inside `if indexRoot, err := projectRoot(); err == nil`; both writes below it are outside the branch"},
	"internal/generators/req.go|appendREQTransitionLog|unguarded-write|logFile": {1,
		"guard lives inside `if root, err := projectRoot(); err == nil`; the append happens whether or not the root was established"},
	"internal/generators/roadmap.go|appendTransitionLog|unguarded-write|lp": {1,
		"same shape as appendREQTransitionLog — the two are explicitly written as mirrors of each other"},
}

type blindSpot struct {
	sites  int
	reason string
}

func liveReport(t *testing.T) Report {
	t.Helper()
	units, err := liveUnits(repositoryRoot(t), "internal", "cmd")
	if err != nil {
		t.Fatalf("reading the live tree: %v", err)
	}
	report, err := analyzeUnits(units)
	if err != nil {
		t.Fatalf("analysing the live tree: %v", err)
	}
	return report
}

func TestContainmentAnalyserOverTheLiveTree(t *testing.T) {
	report := liveReport(t)

	// Vacuity guard FIRST — a verdict over an empty or half-walked tree is worth
	// nothing, and reporting it as an approval is the failure this ML exists for.
	if report.InPopulation < livePopulationFloor {
		t.Fatalf("only %d write site(s) in population, floor is %d — refusing to report a silent approval", report.InPopulation, livePopulationFloor)
	}

	accountedBlind, accountedFailOpen := map[string]int{}, map[string]int{}
	var unexplained []Finding
	for _, finding := range report.Findings {
		key := finding.siteKey()
		switch {
		case liveAnalyserBlindSpots[key].sites > 0:
			accountedBlind[key]++
		case liveKnownFailOpen[key].sites > 0:
			accountedFailOpen[key]++
		default:
			unexplained = append(unexplained, finding)
		}
	}

	for _, finding := range unexplained {
		t.Errorf("containment violation with no pinned entry: %s", finding)
	}
	if len(unexplained) > 0 {
		t.Fatalf("%d site(s) violate P1 and are in neither pinned list — either the write is genuinely unguarded, or the analyser needs an entry that NAMES the site and says why", len(unexplained))
	}

	// A stale entry is as bad as a missing one: it reads as "the rule passes here"
	// while matching nothing at all.
	assertListMatches(t, "blind spot", liveAnalyserBlindSpots, accountedBlind)
	assertListMatches(t, "known fail-open", liveKnownFailOpen, accountedFailOpen)

	// 🔴 T3 again: an entry without a written reason is an exception that says
	// nothing, and an exception that says nothing is the one nobody can audit.
	for _, list := range []map[string]blindSpot{liveAnalyserBlindSpots, liveKnownFailOpen} {
		for key, entry := range list {
			if len(strings.TrimSpace(entry.reason)) < 40 {
				t.Errorf("pinned entry %q carries no usable reason — an exception whose justification cannot be checked by reading the site is the back door T3 describes", key)
			}
		}
	}

	assertPin(t, "blind-spot sites", sumSites(liveAnalyserBlindSpots), liveBlindSpotSitesPin)
	assertPin(t, "known fail-open sites", sumSites(liveKnownFailOpen), liveFailOpenSitesPin)

	t.Logf("live tree: %d file(s), %d write site(s), %d in population, %d guard call(s); %d finding(s), all pinned (%d blind spot, %d fail-open)",
		report.FilesParsed, report.WriteSites, report.InPopulation, report.GuardCalls,
		len(report.Findings), sumSites(liveAnalyserBlindSpots), sumSites(liveKnownFailOpen))
}

func TestP1PopulationIsNotVacuous(t *testing.T) {
	report := liveReport(t)
	if report.FilesParsed < liveFileFloor {
		t.Fatalf("only %d production Go file(s) parsed, floor is %d — the walk did not reach the tree", report.FilesParsed, liveFileFloor)
	}
	if report.WriteSites < liveWriteSiteFloor {
		t.Fatalf("only %d write primitive call(s) seen, floor is %d — the analyser's write vocabulary or its walk has shrunk", report.WriteSites, liveWriteSiteFloor)
	}
	if report.InPopulation == 0 {
		t.Fatalf("ZERO write sites in population — the taint fixpoint is dead and every verdict above is vacuous")
	}
	if report.InPopulation < livePopulationFloor {
		t.Fatalf("%d site(s) in population, floor is %d (delta %+d)", report.InPopulation, livePopulationFloor, report.InPopulation-livePopulationFloor)
	}
	if report.GuardCalls < liveGuardCallFloor {
		t.Fatalf("only %d containment guard call(s) found, floor is %d — ML-7B collapsed 53 implementations into one emitter and left ~59 delegating sites; so few means the walk is not seeing them", report.GuardCalls, liveGuardCallFloor)
	}
	t.Logf("non-vacuity: %d files, %d writes, %d in population, %d guards", report.FilesParsed, report.WriteSites, report.InPopulation, report.GuardCalls)
}

// TestP2PointsAtTheUnresolvedRootsWithoutFixingThem is the arm the threat model
// asks for in §3.4 and the AC repeats: P2 must SEE the argument defect. 🔴 It must
// not correct it — the 16 sites are the population Wave 8 exists to measure, and
// resolving them here would erase it.
func TestP2PointsAtTheUnresolvedRootsWithoutFixingThem(t *testing.T) {
	report := liveReport(t)
	if len(report.P2Unresolved) == 0 {
		t.Fatalf("P2 reports ZERO unresolved guard roots — the 16 filepath.Clean sites are still in the tree by design (Wave 8), so zero means P2 is not running")
	}

	cleanSites := []Finding{}
	for _, finding := range report.P2Unresolved {
		if strings.HasPrefix(finding.Path, "filepath.Clean(") {
			cleanSites = append(cleanSites, finding)
		}
	}
	assertPin(t, "guard roots that are literally filepath.Clean(…)", len(cleanSites), liveP2CleanSitesPin)
	assertPin(t, "guard roots without resolver provenance (total)", len(report.P2Unresolved), liveP2UnresolvedPin)
	assertPin(t, "writes guarded under a different root spelling", len(report.RootAliased), liveRootAliasedPin)

	for _, finding := range cleanSites {
		t.Logf("Wave 8 population: %s", finding)
	}
}

func TestAnalyserVocabularyIsPinned(t *testing.T) {
	if len(writePrimitives) != analyzerVocabularyFloors {
		t.Errorf("the write primitive vocabulary holds %d entries, pinned at %d — a primitive removed from this map is a whole class of writes the analyser stops seeing, silently",
			len(writePrimitives), analyzerVocabularyFloors)
	}
	for _, required := range []string{"WriteFile", "Create", "CreateTemp", "OpenFile", "Rename", "MkdirAll"} {
		if _, ok := writePrimitives[required]; !ok {
			t.Errorf("os.%s is not in the write vocabulary — scripts/check-write-containment.sh covers it, so dropping it here is a coverage regression against the gate this analyser is meant to surpass", required)
		}
	}
	for _, required := range []string{"RejectAndReport", "RejectSymlinks", "GuardedWrite"} {
		if _, ok := guardFunctions[required]; !ok {
			t.Errorf("%s is not recognised as a guard — every site that uses it would be read as unguarded, and the resulting noise is what pressures someone into widening the exception list", required)
		}
	}
	if !unresolvedProducers["Clean"] {
		t.Errorf("filepath.Clean was made transparent for provenance — that single change turns this analyser into the reachability analyser the threat model says approves the live escape")
	}
	for _, required := range []string{"projectRoot", "scaffoldRoot", "resolveRoot", "EvalSymlinks"} {
		if !approvedResolvers[required] {
			t.Errorf("%s is no longer an approved resolver — correct sites would be flagged, and pressure to except them follows", required)
		}
	}
}

func assertListMatches(t *testing.T, label string, pinned map[string]blindSpot, observed map[string]int) {
	t.Helper()
	keys := make([]string, 0, len(pinned))
	for key := range pinned {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		want := pinned[key].sites
		got := observed[key]
		switch {
		case got == 0:
			t.Errorf("%s entry %q matches NOTHING in the tree — a stale entry reads as an approval; remove it in the commit that fixed the site", label, key)
		case got != want:
			t.Errorf("%s entry %q covers %d site(s), pinned at %d (delta %+d) — a new site must not hide inside an existing entry", label, key, got, want, got-want)
		}
	}
}

func sumSites(list map[string]blindSpot) int {
	total := 0
	for _, entry := range list {
		total += entry.sites
	}
	return total
}
