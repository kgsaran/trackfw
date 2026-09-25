package pathguard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// single_emitter_test.go — ML-7B / AC5 of REQ-2026-08-31.
//
// What this file affirms, in one sentence per test (Regra Dura de Reconciliação):
//
//   - TestNoContainmentEmitterOutsidePathguard affirms the ML's conclusion that the
//     53 implementations of the "predicate + audible refusal" pair collapsed into a
//     single emission point: no production file outside package pathguard calls the
//     predicate directly or prints a containment refusal of its own.
//   - TestScannerFlagsEveryHistoricShape affirms that the scanner above is not
//     vacuous: rebuilt from the measurement, each of the 8 shapes that existed
//     before this ML (5 message grammars + the named helper + the mute guard + the
//     leaf-only advisory) is flagged BY NAME, so reverting any single call site is
//     caught rather than averaged away by a count.
//
// 🔴 Why the scanner is a predicate over the source and not a count: a test that
// asserted "there are N call sites" passes with the collapse done halfway. The
// assertion here is "the set of offending sites is empty", and every element of
// that set is printed with file:line — which is what makes reverting ONE site
// reprove, per the AC.
//
// 🔴 Declared limits, stated so they read as scoping and not omission:
//
//  1. The scanner proves each site DELEGATES the refusal. It does not prove the
//     site still ACTS on the returned error (`if err != nil { }` with an empty
//     body would pass here). "Every write is preceded by the guard in the same
//     flow" is the guard-before-write property, and it is ML-7C's analyser.
//  2. Detection keys on the literal `pathguard.RejectSymlinks(`, so an import
//     alias (`pg "…/pathguard"`) evades it. That is a ruler-by-identifier, which
//     is precisely what this REQ condemns — closing it needs the AST, i.e. ML-7C.
//     Measured on 2026-09-25: zero aliased imports of this package in the tree.
//  3. The corpus is the whole repository (every non-test .go outside testdata),
//     not just internal/ — the AC says "in the binary", and cmd/ is in the binary.

// scanFinding is one offending source line. It always names the artifact
// (file:line) — a violation that only names the rule gives a false green.
type scanFinding struct {
	File string
	Line int
	Kind string
	Text string
}

func (f scanFinding) String() string {
	return fmt.Sprintf("%s:%d [%s] %s", f.File, f.Line, f.Kind, f.Text)
}

// refusalVocabulary is the set of substrings that mark a stderr write as a
// containment refusal. It is deliberately broader than the current grammar: it
// covers the four "refusing …" grammars AND the two Portuguese advisories that
// existed before this ML ("não escreve através de symlinks", "é um symlink"),
// because a revert would restore one of those literals, not the current one.
var refusalVocabulary = []string{
	"refusing",
	"refus",
	"não escreve através de symlinks",
	"é um symlink",
}

// scanContainmentEmitters walks scanRoot for production Go files and reports
// every line that either (a) calls the raw predicate pathguard.RejectSymlinks,
// or (b) writes a containment refusal to stderr on its own. Files belonging to
// package pathguard itself are exempt — that is where the single emitter lives.
//
// It also returns the number of files scanned and the number of delegating call
// sites found, so the caller can refuse to report a silent approval over an
// empty or shrunken corpus.
func scanContainmentEmitters(scanRoot string) (findings []scanFinding, filesScanned int, delegations int, err error) {
	walkErr := filepath.Walk(scanRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			base := info.Name()
			if base == "testdata" || base == "vendor" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		source := string(data)
		// Package pathguard is the single emitter: exempt by construction, the
		// same self-exemption pattern the bash gates use for their own helpers.
		if strings.HasPrefix(source, "package pathguard\n") ||
			strings.Contains(source, "\npackage pathguard\n") {
			return nil
		}
		relative, relErr := filepath.Rel(scanRoot, path)
		if relErr != nil {
			relative = path
		}
		filesScanned++
		for index, line := range strings.Split(source, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(line, "pathguard.RejectAndReport(") ||
				strings.Contains(line, "pathguard.RefuseUnverifiableRoot(") {
				delegations++
			}
			if strings.Contains(line, "pathguard.RejectSymlinks(") {
				findings = append(findings, scanFinding{relative, index + 1, "raw-predicate", trimmed})
			}
			if strings.Contains(line, "Fprintf(os.Stderr") && lineIsRefusal(line) {
				findings = append(findings, scanFinding{relative, index + 1, "rogue-emitter", trimmed})
			}
		}
		return nil
	})
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	return findings, filesScanned, delegations, walkErr
}

func lineIsRefusal(line string) bool {
	for _, token := range refusalVocabulary {
		if strings.Contains(line, token) {
			return true
		}
	}
	return false
}

// repositoryRoot walks up from the test's working directory to the directory
// holding go.mod. The whole repository is the corpus: cmd/ ships in the same
// binary as internal/, and a refusal printed from there would be just as
// divergent.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(directory, "go.mod")); statErr == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatalf("could not find go.mod above %q — cannot locate the corpus", directory)
		}
		directory = parent
	}
}

// Floors. Measured on 2026-09-25 after the collapse, over the whole repository:
// 108 production Go files outside package pathguard (internal/ + cmd/), and 56
// delegating call sites (52 RejectAndReport + 4 RefuseUnverifiableRoot). The
// floors sit below the measurement so that legitimate refactors do not trip
// them, but far enough above zero that an empty or half-walked corpus cannot
// report a silent approval.
const (
	fileFloor       = 90
	delegationFloor = 50
)

func TestNoContainmentEmitterOutsidePathguard(t *testing.T) {
	findings, files, delegations, err := scanContainmentEmitters(repositoryRoot(t))
	if err != nil {
		t.Fatalf("scanning the repository: %v", err)
	}

	// Vacuity guard FIRST: an empty or shrunken corpus must never be reported
	// as an approval.
	if files < fileFloor {
		t.Fatalf("only %d production Go file(s) scanned, floor is %d — refusing to report a silent approval", files, fileFloor)
	}
	if delegations < delegationFloor {
		t.Fatalf("only %d delegating call site(s) found, floor is %d — the collapse cannot have happened over so few sites; refusing to report a silent approval", delegations, delegationFloor)
	}

	if len(findings) != 0 {
		for _, finding := range findings {
			t.Errorf("containment refusal outside package pathguard: %s", finding)
		}
		t.Fatalf("%d site(s) do not delegate to pathguard.RejectAndReport — AC5 requires exactly one emitter in the binary", len(findings))
	}

	t.Logf("corpus: %d production file(s), %d delegating call site(s), 0 rogue emitter(s)", files, delegations)
}

// historicShape is one of the forms that existed in the tree before this ML,
// reconstructed from the ML-6A measurement of 2026-09-25. Each is a separate
// synthetic file so that a failure names exactly which shape the scanner missed.
type historicShape struct {
	name string
	body string
}

func historicShapes() []historicShape {
	return []historicShape{
		{"grammar-1-refusing-write", `package mutant

func guard(root, absTarget string) error {
	if guardErr := pathguard.RejectSymlinks(root, absTarget); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absTarget, guardErr)
		return guardErr
	}
	return nil
}
`},
		{"grammar-2-refusing-symlink-path", `package mutant

func guard(root, absTarget string) error {
	if guardErr := pathguard.RejectSymlinks(root, absTarget); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing symlink path %s: %v\n", absTarget, guardErr)
		return guardErr
	}
	return nil
}
`},
		{"grammar-3-husky-suffix", `package mutant

func guard(root, rootDir string) error {
	if guardErr := pathguard.RejectSymlinks(root, rootDir); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s/.husky: %v\n", rootDir, guardErr)
		return guardErr
	}
	return nil
}
`},
		{"grammar-4-roadmap-move-prefix", `package mutant

func guard(root, reqBase string) error {
	if guardErr := pathguard.RejectSymlinks(root, reqBase); guardErr != nil {
		fmt.Fprintf(os.Stderr, "trackfw roadmap move: refusing symlink path %s: %v\n", reqBase, guardErr)
		return guardErr
	}
	return nil
}
`},
		{"grammar-5-discover-aviso-nonfatal", `package mutant

func guard(root, workflow string) error {
	if guardErr := pathguard.RejectSymlinks(root, workflow); guardErr != nil {
		fmt.Fprintf(os.Stderr, "aviso: %s — trackfw discover não escreve através de symlinks — arquivo não foi tocado\n", workflow)
		return nil
	}
	return nil
}
`},
		{"named-helper-reimplementing-the-pair", `package mutant

func rejectScaffoldPath(root, absTarget string) error {
	if err := pathguard.RejectSymlinks(root, absTarget); err != nil {
		fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absTarget, err)
		return fmt.Errorf("refusing write to %s: %w", absTarget, err)
	}
	return nil
}
`},
		{"mute-guard-refuses-without-a-word", `package mutant

func appendTransitionLog(root, absLog string) {
	if guardErr := pathguard.RejectSymlinks(root, absLog); guardErr != nil {
		return
	}
}
`},
		{"leaf-only-advisory-without-the-predicate", `package mutant

func refreshWorkflow(path string) error {
	if isSymlink(path) {
		fmt.Fprintf(os.Stderr, "aviso: %s é um symlink; trackfw update não escreve através de symlinks — arquivo não foi tocado\n", path)
		return nil
	}
	return nil
}
`},
	}
}

func TestScannerFlagsEveryHistoricShape(t *testing.T) {
	shapes := historicShapes()
	if len(shapes) < 8 {
		t.Fatalf("mutant corpus has %d shape(s); the measurement of 2026-09-25 found 8 — refusing to report a silent approval", len(shapes))
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			corpus := t.TempDir()
			packageDir := filepath.Join(corpus, "mutantpkg")
			if err := os.MkdirAll(packageDir, 0o755); err != nil {
				t.Fatalf("building the mutant corpus: %v", err)
			}
			file := filepath.Join(packageDir, "mutant.go")
			if err := os.WriteFile(file, []byte(shape.body), 0o644); err != nil {
				t.Fatalf("building the mutant corpus: %v", err)
			}

			// The fixture must exist and be readable — otherwise the scanner
			// would report "no findings" for the wrong reason and the arm would
			// look green while measuring nothing (the ML-7A degeneration).
			if info, err := os.Stat(file); err != nil || info.Size() == 0 {
				t.Fatalf("fixture %s is missing or empty — this arm would pass for the wrong reason", file)
			}

			findings, files, _, err := scanContainmentEmitters(corpus)
			if err != nil {
				t.Fatalf("scanning the mutant corpus: %v", err)
			}
			if files != 1 {
				t.Fatalf("mutant corpus should hold exactly 1 production file, scanner saw %d", files)
			}
			if len(findings) == 0 {
				t.Fatalf("scanner did NOT flag the %q shape — a revert to this form would pass the gate unnoticed", shape.name)
			}
			for _, finding := range findings {
				if !strings.HasSuffix(finding.File, "mutant.go") {
					t.Errorf("finding does not name the offending artifact: %s", finding)
				}
			}
			t.Logf("%s → %d finding(s): %v", shape.name, len(findings), findings)
		})
	}
}
