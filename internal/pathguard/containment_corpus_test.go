package pathguard

// containment_corpus_test.go — the T1 arm of the threat model
// (docs/seguranca/2026-09-25-ponto-unico-de-contencao-e-o-instrumento-que-o-prova.md §3.1).
//
// # Why a corpus exists at all
//
// The 34 defects of #400 are already fixed. An analyser run over today's tree is
// therefore green BECAUSE THERE IS NOTHING TO FIND, which is indistinguishable
// from an analyser that works. The only arm that tells the two apart is the
// pre-fix tree.
//
// # Why it is a file in testdata and not a commit-ish
//
// The pre-fix state lives at 87fe4915. That object was reachable from ONE local
// ref on one machine: the PR was squash-merged, the branch was deleted on the
// remote, and `git branch -vv` reports it as gone — which both CLAUDE.md's
// protocol and `trackfw branch prune --apply` classify as safe to delete. This
// project already lost work that way on 2026-09-12. A gate whose evidence lives
// outside the versioned tree is not a gate, and `git show` is not executable in a
// fresh CI clone.
//
// 🔴 So: the corpus is checked in, under testdata/corpus-pre-fix/, as .go.txt
// (the extension keeps it out of the build AND out of the `find internal -name
// '*.go'` of scripts/check-write-containment.sh, which would otherwise see 157
// unjustified write sites). No arm in this file names a commit.
//
// Absence is FAIL, never skip: a skip is indistinguishable from green, which is
// the exact failure mode this file exists to prevent.
//
// One sentence per test (Regra Dura de Reconciliação):
//
//   - TestCorpusIsPresentAndIntact affirms the ML's conclusion that the falsification
//     evidence is versioned and self-verifying: the 16 files are present, their
//     sha256 match the manifest, and every failure mode of loading is FAIL.
//   - TestAnalyserReprovesThePreFixCorpus affirms the ML's conclusion that the
//     analyser discriminates — it reports an exactly pinned number of contained
//     violations over the pre-fix tree, including BY NAME the two flagship cases of
//     #400 (syncREQReferences and the lefthook branch of generateCommitMsgHook),
//     while the same analyser reports none of them over today's tree.
//   - TestCorpusArmIsIndependentOfTheExceptionList affirms the ML's conclusion that
//     T3 is closed: applying every live-tree exception to the corpus does not reduce
//     the corpus verdict, so no one can silence the falsification arm by growing the
//     exception list.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const (
	corpusDir      = "testdata/corpus-pre-fix"
	corpusManifest = "MANIFEST.sha256"
)

// Pins measured on 2026-09-25 by this analyser over the versioned corpus. 🔴 By
// COUNT, never by date: a calendar expiry approves in silence the day after the
// merge (threat model §3.2).
//
// ⚠️ These numbers are NOT the 22 gaps + 7 false markers of #400, and that is not
// a discrepancy to paper over — it is a difference of rulers, reported in the
// ML's report: #400 counted MARKERS audited by hand; this analyser counts WRITE
// SITES that its taint population reaches. Two instruments, two populations, one
// overlapping conclusion.
const (
	corpusFileCount       = 16
	corpusWriteSites      = 146
	corpusInPopulation    = 139
	corpusUnguardedWrites = 35
	corpusRawPredicates   = 35
	corpusRogueEmitters   = 34
	corpusTotalFindings   = 104
)

// corpusFlagshipSites are the two cases #400 names explicitly. They are asserted
// BY NAME, not by count: a count survives losing exactly these two and gaining
// two others, and they are the ones the issue was written about.
var corpusFlagshipSites = []struct {
	file, function, path, why string
}{
	{
		file: "internal/generators/roadmap.go", function: "syncREQReferences", path: "reqPath",
		why: "the marker is present and there is not one pathguard call in the whole function — the false marker in its purest form",
	},
	{
		file: "internal/generators/scaffold.go", function: "generateCommitMsgHook", path: "lefthookPath",
		why: "the two guards of the block cover .husky and .lefthook/commit-msg; lefthook.yml is written at the root, outside both — the leaf gap in its purest form",
	},
}

// loadCorpus reads the versioned corpus and verifies it against the manifest.
// Every failure path is an error, so the caller can Fatal — there is no code path
// here that returns "no corpus, carry on".
func loadCorpus(root string) ([]unit, error) {
	manifestPath := filepath.Join(root, corpusManifest)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("corpus manifest %s is missing or unreadable (%w) — the falsification arm has no evidence to run against; this is FAIL, never skip", manifestPath, err)
	}
	expected := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(manifestBytes)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed manifest line %q", line)
		}
		expected[fields[1]] = fields[0]
	}
	if len(expected) == 0 {
		return nil, fmt.Errorf("corpus manifest %s lists no file", manifestPath)
	}

	var units []unit
	for name, want := range expected {
		path := filepath.Join(root, filepath.FromSlash(name))
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("corpus file %s is missing (%w) — FAIL, never skip", path, readErr)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			return nil, fmt.Errorf("corpus file %s has sha256 %s, the manifest pins %s — the frozen pre-fix evidence was modified", name, got, want)
		}
		units = append(units, unit{
			name: "CORPUS/" + strings.TrimSuffix(name, ".txt"),
			src:  string(data),
		})
	}
	sort.Slice(units, func(i, j int) bool { return units[i].name < units[j].name })
	return units, nil
}

func corpusUnits(t *testing.T) []unit {
	t.Helper()
	units, err := loadCorpus(corpusDir)
	if err != nil {
		t.Fatalf("loading the pre-fix corpus: %v", err)
	}
	return units
}

func TestCorpusIsPresentAndIntact(t *testing.T) {
	units := corpusUnits(t)
	if len(units) != corpusFileCount {
		t.Fatalf("the corpus holds %d file(s), the pin is %d (delta %+d) — update the pin in the same commit that changes the corpus, or the falsification arm is measuring something else",
			len(units), corpusFileCount, len(units)-corpusFileCount)
	}

	// Mode (a) of the obsolescence table: corpus absent must be FAIL, and the
	// only way to prove the loader does not degrade to a skip is to ask it for a
	// corpus that is not there.
	if _, err := loadCorpus(filepath.Join(corpusDir, "there-is-no-such-directory")); err == nil {
		t.Fatalf("loadCorpus returned no error for an absent corpus — an absent corpus MUST fail; a skip is indistinguishable from green")
	}
	t.Logf("corpus: %d file(s), sha256 verified against %s", len(units), corpusManifest)
}

func TestAnalyserReprovesThePreFixCorpus(t *testing.T) {
	report, err := analyzeUnits(corpusUnits(t))
	if err != nil {
		t.Fatalf("analysing the corpus: %v", err)
	}

	// Mode (b): population zero is FAIL. Mode (c): population divergence is FAIL
	// WITH THE DELTA PRINTED, so the next reader knows how far it moved.
	if report.InPopulation == 0 {
		t.Fatalf("the analyser found ZERO write sites in population over the corpus — it is measuring nothing; refusing to report a verdict")
	}
	assertPin(t, "corpus write sites", report.WriteSites, corpusWriteSites)
	assertPin(t, "corpus sites in population", report.InPopulation, corpusInPopulation)
	assertPin(t, "corpus total findings", len(report.Findings), corpusTotalFindings)

	byKind := map[string]int{}
	for _, finding := range report.Findings {
		byKind[finding.Kind]++
	}
	assertPin(t, "corpus unguarded writes (P1)", byKind[kindUnguardedWrite], corpusUnguardedWrites)
	assertPin(t, "corpus raw predicate calls", byKind[kindRawPredicate], corpusRawPredicates)
	assertPin(t, "corpus rogue emitters", byKind[kindRogueEmitter], corpusRogueEmitters)

	// 🔴 The count is not the claim. These two sites are.
	for _, flagship := range corpusFlagshipSites {
		found := false
		for _, finding := range report.Findings {
			if finding.Kind == kindUnguardedWrite &&
				strings.HasSuffix(finding.File, flagship.file) &&
				finding.Func == flagship.function &&
				finding.Path == flagship.path {
				found = true
				t.Logf("flagship case reproved: %s — %s", finding, flagship.why)
			}
		}
		if !found {
			t.Errorf("the analyser did NOT reprove %s() writing %q in %s — this is one of the two cases #400 is written about; %s",
				flagship.function, flagship.path, flagship.file, flagship.why)
		}
	}
}

// TestCorpusArmIsIndependentOfTheExceptionList is the T3 arm. The threat model
// calls the exception list "the most likely way to empty this gate, above the
// malicious ones": the analyser points at a legitimate site, someone relaxes the
// rule or excepts the file, and the gate is green forever.
//
// 🔴 The countermeasure is mechanical, not a promise: the exception keys are
// stripped of their file component (so a live exception matches the corpus copy
// of the same function, which it would NOT do otherwise) and applied to the
// corpus. The corpus verdict must not move.
func TestCorpusArmIsIndependentOfTheExceptionList(t *testing.T) {
	report, err := analyzeUnits(corpusUnits(t))
	if err != nil {
		t.Fatalf("analysing the corpus: %v", err)
	}

	loose := map[string]bool{}
	for key := range liveAnalyserBlindSpots {
		loose[stripFileFromKey(key)] = true
	}
	for key := range liveKnownFailOpen {
		loose[stripFileFromKey(key)] = true
	}
	if len(loose) == 0 {
		t.Fatalf("no exception key to apply — this arm would pass vacuously")
	}

	survivors := 0
	for _, finding := range report.Findings {
		if !loose[stripFileFromKey(finding.siteKey())] {
			survivors++
		}
	}
	if survivors == 0 {
		t.Fatalf("applying the live exception list silenced the corpus entirely — the list has become the back door T3 describes")
	}

	// And the two flagship cases must survive by name, not merely "some findings
	// remain": a list that silenced exactly those two while leaving 80 others
	// would pass a count-only assertion.
	for _, flagship := range corpusFlagshipSites {
		silenced := true
		for _, finding := range report.Findings {
			if finding.Func == flagship.function && finding.Path == flagship.path &&
				!loose[stripFileFromKey(finding.siteKey())] {
				silenced = false
			}
		}
		if silenced {
			t.Errorf("the exception list silences the corpus finding for %s() — the falsification arm must be independent of the list", flagship.function)
		}
	}
	t.Logf("%d of %d corpus finding(s) survive every live exception applied file-blind", survivors, len(report.Findings))
}

func stripFileFromKey(key string) string {
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		return key
	}
	return parts[1]
}

func assertPin(t *testing.T, label string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: measured %d, pinned %d (delta %+d) — if the change is legitimate, move the pin in the same commit; a pin that drifts silently is how a gate becomes vacuous",
			label, got, want, got-want)
		return
	}
	t.Logf("%s: %d (pinned)", label, got)
}
