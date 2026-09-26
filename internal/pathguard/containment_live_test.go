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
//	    it, so a resolver failure means the write proceeds unguarded. 🔴 EMPTY since
//	    ML-9A: the five sites it held were fixed, not approved — same cause, same
//	    REQ (CLAUDE.md's Regra Dura de Causa Raiz). The list stays declared because
//	    an empty exception list is what makes a reintroduction land in the
//	    `unexplained` arm and fail by name.
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
//   - TestP2FindsOnlyTheRootsThatCannotMove affirms ML-8A's conclusion that the
//     Wave 8 population is CLOSED: zero guard roots are a filepath.Clean, and the
//     three that remain without resolver provenance are named one by one with the
//     measurement that says why each cannot move — while a synthetic unit carrying
//     a Clean root, analysed in the same run as the live tree, is still reported,
//     so "P2 found nothing" cannot mean "P2 stopped running".
//   - TestRootAliasedResidualIsNamed affirms ML-8A's conclusion that the writes
//     guarded under a different root spelling fell from 16 to 3, and that the 3
//     survivors are the two sites whose OTHER operand cannot move with the root.
//   - TestAnalyserVocabularyIsPinned affirms the ML's conclusion that the analyser's
//     own reach is a counted list and not an assumption — shrinking the primitive,
//     guard or resolver vocabulary is the cheapest way to make it vacuous.
//   - TestML9ASitesStayClosed affirms ML-9A's conclusion that the five fail-open
//     sites it fixed are closed IN THE TREE: each of the five functions still holds
//     the write primitive it always held (so "no finding" is not "nothing left to
//     analyse"), and not one finding of any kind names them.

import (
	"go/ast"
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
	liveP2UnresolvedPin   = 3   // was 22 until ML-8A; see liveP2IrreducibleRoots
	liveP2CleanSitesPin   = 0   // was 16 until ML-8A — the whole Wave 8 population
	liveRootAliasedPin    = 3   // was 16 until ML-8A; see liveRootAliasedResidual
	liveBlindSpotSitesPin = 15
	liveFailOpenSitesPin  = 0 // was 5 until ML-9A; see liveKnownFailOpen
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
	// 🔴 EMPTY since ML-9A (2026-09-25), and that is the point of the list, not the
	// end of it. The five entries that lived here — java.go|GeneratePomXML,
	// note.go|appendNoteToIndex (2 writes), req.go|appendREQTransitionLog and
	// roadmap.go|appendTransitionLog — were measured DEFECTS, not exceptions: the
	// guard sat inside `if …, err := projectRoot(); err == nil` and the write sat
	// outside it. ML-9A replaced the fail-open with pathguard.RefuseUnverifiableRoot
	// and the analyser stopped reporting them (20 findings → 15).
	//
	// Keeping the list declared and empty is deliberate: a finding that lands in
	// NEITHER pinned list is an error (the `unexplained` arm above), so the empty
	// map is what makes a reintroduced fail-open fail this test by NAME instead of
	// being absorbed by an entry someone widens. TestML9ASitesStayClosed below
	// names the five sites, so their silence is asserted and not merely observed.
}

// liveP2IrreducibleRoots — the guard roots ML-8A did NOT move, BY SITE, each with
// the measurement that says why. 🔴 These are not "fine": they are the three
// places where moving the root alone would recreate armadilha 3 in its other
// direction, because the OTHER operand of the comparison cannot move with it.
// That is the test ML-8A applied at every site: a root may be resolved only when
// the target derived from it moves into the same namespace.
var liveP2IrreducibleRoots = map[string]blindSpot{
	"internal/generators/update.go|rejectHarnessSymlink|root-unresolved|home": {1,
		"`home` is not only the guard root of UpdateHarness: it is embedded VERBATIM in the hook command paths written into .claude/settings.json, .codex/hooks.json, .gemini/settings.json and .cursor/hooks.json. Resolving it changes the CONTENT of every generated artifact (/var/… → /private/var/… on macOS) — measured: 15 tests pin the logical form — and that is a product decision about what trackfw writes into the user's config, not an argument fix. Containment is not degraded: every harness target is filepath.Join'ed from this same `home`, so root and target share a namespace, and armadilha 3 is the MISMATCH between namespaces, not the choice of one"},
	"internal/integrations/manager.go|rejectSymlinks|root-unresolved|root": {1,
		"ML-8A made this root pathguard.ResolveRoot(root) and REVERTED after measuring: in Manager.resolve's `IsAnchored || IsAbs` branch the destination is a user-supplied ABSOLUTE path accepted verbatim, not derived from root, so a resolved root refuses it — 7 tests failed, e.g. TestClaimOrigin_LegacyManifestReadsAsCatalog reported `destination \"/var/folders/….claude/agents/trackfw-backend.md\" is outside project root`. The other operand cannot move either: resolving the DESTINATION is the ML-6A self-refutation, since EvalSymlinks erases the symlink the guard exists to reject"},
	"internal/pathguard/pathguard.go|RejectAndReport|root-unresolved|resolvedRoot": {1,
		"this is the single emitter itself, and `resolvedRoot` is its PARAMETER — the contract boundary where the caller's obligation is discharged. There is nothing upstream of it inside this package to derive provenance from, and resolving it here is precisely what the function's own doc forbids: it would silently repair every caller's argument and erase the measurement P2 exists to produce. Irreducible by construction, not by omission"},
}

// liveRootAliasedResidual — writes still guarded under a DIFFERENT spelling of
// the root than the one they are written through. ML-8A closed 13 of the 16 (all
// of internal/discover) by deriving the written path from the resolved root; the
// three below fail the same "does the other operand move" test.
var liveRootAliasedResidual = map[string]blindSpot{
	"internal/generators/adr.go|NewADRDraft|root-aliased|path": {1,
		"the guard is deliberately on the PRE-EvalSymlinks path (absLeafDraft = Join(absAdrDirDraftRaw, filename)) — the ML-4B leaf-gap fix — so that a symlink leaf is still seen as a symlink. Aligning the write to the guard's spelling means changing which path adr new creates relative to a possibly-relative adrDir, which is exactly the flow ML-7A's adrGuardPaths owns and whose two $PWD arms it pins. A spelling decision inside another ML's invariant, not an argument fix"},
	"internal/generators/roadmap.go|MoveRoadmap|root-aliased|dst": {2,
		"dst is RELATIVE by design and is used that way by os.Rename, os.ReadFile, the status-sync write, agentFromPath and every error message in the function; the guard is on absDst = Join(root, dst). Moving the write to absDst means changing MoveRoadmap's path convention wholesale — and the function already carries a measured reason not to re-derive paths after the rename (src no longer exists, and EvalSymlinks then yields the divergent /var vs /private/var prefix). A convention change with its own decision, not an argument fix"},
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

// TestP2FindsOnlyTheRootsThatCannotMove is the ML-8A inversion of the arm the
// threat model asks for in §3.4. Until ML-8A, P2 had to SEE the argument defect
// and leave it alone — the 16 filepath.Clean roots were the population Wave 8
// existed to measure. ML-8A closed that population, so the assertion flips: zero
// Clean roots, and every surviving finding named with the measurement that says
// why it could not move.
//
// 🔴 A test that only asserts "P2 found nothing" is satisfied by a P2 that stopped
// running. The synthetic unit below is analysed in the SAME call as the live tree
// and MUST be reported, so the green above is "nothing left to find" and not
// "nothing is looking".
func TestP2FindsOnlyTheRootsThatCannotMove(t *testing.T) {
	report := liveReport(t)

	cleanSites := []Finding{}
	for _, finding := range report.P2Unresolved {
		if strings.HasPrefix(finding.Path, "filepath.Clean(") {
			cleanSites = append(cleanSites, finding)
		}
	}
	for _, finding := range cleanSites {
		t.Errorf("a guard root is still a literal filepath.Clean(…): %s — filepath.Clean normalises text and never resolves a symlink; this is the whole of #402", finding)
	}
	assertPin(t, "guard roots that are literally filepath.Clean(…)", len(cleanSites), liveP2CleanSitesPin)

	accounted := map[string]int{}
	for _, finding := range report.P2Unresolved {
		key := finding.siteKey()
		if liveP2IrreducibleRoots[key].sites > 0 {
			accounted[key]++
			continue
		}
		t.Errorf("guard root without resolver provenance and with NO written reason: %s — either derive it from a resolver, or add an entry that NAMES this site and says what measurement stops it", finding)
	}
	assertListMatches(t, "irreducible root", liveP2IrreducibleRoots, accounted)
	assertPin(t, "guard roots without resolver provenance (total)", len(report.P2Unresolved), liveP2UnresolvedPin)

	for key, entry := range liveP2IrreducibleRoots {
		if len(strings.TrimSpace(entry.reason)) < 40 {
			t.Errorf("irreducible root %q carries no usable reason — \"it cannot move\" without the measurement is an exception nobody can audit", key)
		}
	}

	// Non-vacuity, in the same walk: a synthetic unit whose guard root is
	// filepath.Clean(cwd) must still be reported by name.
	const probe = `package waveeightprobe

import (
	"os"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

func writeProbe(name string) error {
	cwd, _ := os.Getwd()
	target := filepath.Join(cwd, name)
	if guardErr := pathguard.RejectAndReport(filepath.Clean(cwd), target); guardErr != nil {
		return guardErr
	}
	return os.WriteFile(target, nil, 0644)
}
`
	units, err := liveUnits(repositoryRoot(t), "internal", "cmd")
	if err != nil {
		t.Fatalf("reading the live tree: %v", err)
	}
	withProbe, err := analyzeUnits(append(units, unit{name: "waveeightprobe/probe.go", src: probe}))
	if err != nil {
		t.Fatalf("analysing the live tree plus the probe: %v", err)
	}
	seen := false
	for _, finding := range withProbe.P2Unresolved {
		if finding.File == "waveeightprobe/probe.go" && finding.Func == "writeProbe" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("P2 did NOT report the synthetic filepath.Clean root planted in the same walk as the live tree — the zero above would then mean the instrument is off, not that the tree is clean")
	}

	t.Logf("P2 over the live tree: 0 Clean root(s), %d unresolved root(s), all named; probe reported", len(report.P2Unresolved))
}

// TestRootAliasedResidualIsNamed keeps the OTHER half of Wave 8 honest: a write
// whose path is spelled from a different root than the guard's is a write the
// guard does not govern, even when the flow passes through pathguard.
func TestRootAliasedResidualIsNamed(t *testing.T) {
	report := liveReport(t)

	accounted := map[string]int{}
	for _, finding := range report.RootAliased {
		key := finding.siteKey()
		if liveRootAliasedResidual[key].sites > 0 {
			accounted[key]++
			continue
		}
		t.Errorf("write guarded under a different root spelling, with no written reason: %s — derive the written path from the same root the guard uses, or name the site here", finding)
	}
	assertListMatches(t, "root-aliased residual", liveRootAliasedResidual, accounted)
	assertPin(t, "writes guarded under a different root spelling", len(report.RootAliased), liveRootAliasedPin)

	for key, entry := range liveRootAliasedResidual {
		if len(strings.TrimSpace(entry.reason)) < 40 {
			t.Errorf("root-aliased entry %q carries no usable reason", key)
		}
	}
	t.Logf("root-aliased: %d finding(s), all named (13 of the original 16 closed by ML-8A)", len(report.RootAliased))
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

// ─────────────────────────────────────────────────────────────────────────────
// ML-9A — the five fail-open sites, by name
// ─────────────────────────────────────────────────────────────────────────────

// ml9ASites are the five write sites ML-7C measured as fail-open and ML-9A
// closed. They are asserted BY NAME because a count survives losing exactly these
// and gaining others, and because `liveKnownFailOpen` is now empty: an empty map
// asserts nothing on its own, and "the list is empty" must not be the whole proof.
//
// `function` is the function the write lives in TODAY. `formerFunction` is where
// it lived before ML-9A — roadmap.go and req.go extracted the guarded body into a
// mirror pair (…Entry) so that the refusal of an unverifiable root is a plain
// `return pathguard.RefuseUnverifiableRoot(…)` instead of a discarded call whose
// error nothing consumes. Both names are asserted clean, so neither the new shape
// nor a revert to the old one can pass silently.
//
// 🔴 The behavioural proof that each site REFUSES when projectRoot() fails is not
// here — it is in internal/generators (…_fail_closed_test.go), per site, with a
// control arm. This arm proves the instrument no longer sees the defect; that one
// proves the binary no longer commits it.
var ml9ASites = []struct {
	file, function, formerFunction string
	writes                         int
}{
	{"internal/generators/java.go", "GeneratePomXML", "GeneratePomXML", 1},
	{"internal/generators/note.go", "appendNoteToIndex", "appendNoteToIndex", 2},
	{"internal/generators/req.go", "appendREQTransitionLogEntry", "appendREQTransitionLog", 1},
	{"internal/generators/roadmap.go", "appendTransitionLogEntry", "appendTransitionLog", 1},
}

func TestML9ASitesStayClosed(t *testing.T) {
	if len(liveKnownFailOpen) != 0 {
		t.Errorf("liveKnownFailOpen holds %d entry/entries — ML-9A emptied it; a new entry here is a measured defect being recorded instead of fixed, which CLAUDE.md's Regra Dura de Causa Raiz forbids inside the REQ that owns the cause", len(liveKnownFailOpen))
	}

	units, err := liveUnits(repositoryRoot(t), "internal", "cmd")
	if err != nil {
		t.Fatalf("reading the live tree: %v", err)
	}
	analyser, err := newAnalyzer(units)
	if err != nil {
		t.Fatalf("building the analyser: %v", err)
	}
	report := analyser.analyze()

	for _, site := range ml9ASites {
		fn := analyser.funcs[site.function]
		if fn == nil {
			t.Errorf("%s: function %s() is gone from the tree — the silence of this site below would then mean nothing", site.file, site.function)
			continue
		}
		if fn.file.name != site.file {
			t.Errorf("%s() now lives in %s, not %s — the site moved and this pin no longer names what it claims to", site.function, fn.file.name, site.file)
		}
		// Non-vacuity, per site: the write primitive is still there. Without this,
		// deleting the write would make the site "clean" and this test green — the
		// degeneration ML-7A already paid for once in this REQ.
		writes := 0
		walkWithBlocks(fn.decl.Body, func(call *ast.CallExpr, _ located, _ ast.Stmt) {
			if _, _, ok := analyser.writeCallAt(call); ok {
				writes++
			}
		})
		if writes != site.writes {
			t.Errorf("%s:%s() holds %d write primitive call(s), pinned at %d (delta %+d) — a site whose write disappeared is not a site that was fixed", site.file, site.function, writes, site.writes, writes-site.writes)
		}
	}

	for _, finding := range report.Findings {
		for _, site := range ml9ASites {
			if finding.File != site.file {
				continue
			}
			if finding.Func == site.function || finding.Func == site.formerFunction {
				t.Errorf("ML-9A site is violating P1 again: %s", finding)
			}
		}
	}

	t.Logf("ML-9A: %d site(s) named, liveKnownFailOpen empty, %d finding(s) in the tree — none of them in these functions", len(ml9ASites), len(report.Findings))
}
