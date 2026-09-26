package pathguard

// containment_falsify_test.go — the arms that prove the analyser of ML-7C is not
// vacuous, each built from a MUTANT the analyser must reject, plus a control arm
// built from the shape it must accept.
//
// 🔴 Why a control arm is mandatory here: in ML-7A a mutant "failed" because a
// fixture directory did not exist, so the arm was green while measuring nothing.
// Every arm below therefore asserts the KIND and the SITE of the finding, never
// "there was some finding", and the control arm asserts ZERO findings over the
// correct shape — without it, an analyser that flags everything would pass all
// the negative arms.
//
// One sentence per test (Regra Dura de Reconciliação):
//
//   - TestAnalyserRejectsEachMutantShape affirms the ML's conclusion that the two
//     textual residuals of ML-7B (import alias and empty failure branch) and the
//     two structural predicates (dominance, leaf gap) are decided by the AST and
//     not by text: each mutant is flagged by name, and the control arm — the same
//     code written correctly — produces no finding at all.
//   - TestCleanIsNotTransparentForProvenance affirms the ML's conclusion that P2
//     refuses filepath.Clean as a resolver whatever is inside it, which is the
//     discriminant that separates this analyser from the reachability analyser
//     the threat model says would approve the live escape of ML-7A.

import (
	"strings"
	"testing"
)

type mutantArm struct {
	name string
	// src is one synthetic production file. The package name is never "pathguard"
	// so that the self-exemptions of the analyser cannot make an arm pass.
	src string
	// wantKind is the finding the arm must produce; empty means "no finding at all".
	wantKind string
	// wantPath is the artifact the finding must name — a violation that names only
	// the rule gives a false green.
	wantPath string
	// why is the sentence the arm affirms.
	why string
}

func mutantArms() []mutantArm {
	return []mutantArm{
		{
			name: "control/correct-shape-produces-no-finding",
			src: `package mutant

import (
	"os"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

func writeIt(name string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	target := filepath.Join(root, "docs", name)
	if guardErr := pathguard.RejectAndReport(root, target); guardErr != nil {
		return guardErr
	}
	return os.WriteFile(target, nil, 0644)
}

func projectRoot() (string, error) { return filepath.EvalSymlinks(".") }
`,
			wantKind: "",
			why:      "the correct shape — resolved root, guard dominating the write, same path — is accepted, so the negative arms below fail for their stated reason and not because the analyser flags everything",
		},
		{
			name: "alias-import/raw-predicate-hidden-behind-pg",
			src: `package mutant

import (
	"os"
	"path/filepath"

	pg "github.com/kgsaran/trackfw/internal/pathguard"
)

func writeIt(root, name string) error {
	target := filepath.Join(root, name)
	if guardErr := pg.RejectSymlinks(root, target); guardErr != nil {
		return guardErr
	}
	return os.WriteFile(target, nil, 0644)
}
`,
			wantKind: kindRawPredicate,
			wantPath: "target",
			why:      "ML-7B residual 2 is closed: the raw predicate reached through an import alias is flagged, where the literal scanner of single_emitter_test.go sees nothing because it matches the text \"pathguard.RejectSymlinks(\"",
		},
		{
			name: "empty-failure-branch/guard-refuses-and-the-flow-writes-anyway",
			src: `package mutant

import (
	"os"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

func writeIt(root, name string) error {
	target := filepath.Join(root, name)
	if guardErr := pathguard.RejectAndReport(root, target); guardErr != nil {
	}
	return os.WriteFile(target, nil, 0644)
}
`,
			wantKind: kindGuardNotActed,
			wantPath: "target",
			why:      "ML-7B residual 3 is closed: delegating to pathguard is not acting on it — an empty failure branch lets the refused write proceed, and the analyser says so",
		},
		{
			name: "dominance/guard-nested-in-a-branch-the-write-does-not-take",
			src: `package mutant

import (
	"os"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

func writeIt(root, name string, check bool) error {
	target := filepath.Join(root, name)
	if check {
		if guardErr := pathguard.RejectAndReport(root, target); guardErr != nil {
			return guardErr
		}
	}
	return os.WriteFile(target, nil, 0644)
}
`,
			wantKind: kindUnguardedWrite,
			wantPath: "target",
			why:      "P1 is dominance and not source order: a guard nested in a branch the write does not go through leaves the write unguarded, and an analyser that only asked \"does a guard appear earlier in the file\" would approve it",
		},
		{
			name: "leaf-gap/guard-on-the-directory-write-on-the-child",
			src: `package mutant

import (
	"os"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

func writeIt(root string) error {
	directory := filepath.Join(root, ".husky")
	if guardErr := pathguard.RejectAndReport(root, directory); guardErr != nil {
		return guardErr
	}
	return os.WriteFile(filepath.Join(root, ".husky", "pre-commit"), nil, 0644)
}
`,
			wantKind: kindUnguardedWrite,
			wantPath: `filepath.Join(root, ".husky", "pre-commit")`,
			why:      "the leaf gap of #400 is refused by construction: RejectSymlinks walks ancestors and never descends, so guarding a directory does not contain a write to a file inside it — this is the exact shape of the 22 gaps in the corpus",
		},
	}
}

func TestAnalyserRejectsEachMutantShape(t *testing.T) {
	arms := mutantArms()
	if len(arms) < 5 {
		t.Fatalf("mutant corpus has %d arm(s), the design calls for at least 5 — refusing to report a silent approval", len(arms))
	}
	for _, arm := range arms {
		t.Run(arm.name, func(t *testing.T) {
			if len(strings.TrimSpace(arm.why)) < 40 {
				t.Fatalf("arm %q carries no reconciliation sentence — a test that cannot say which conclusion of the ML it affirms should not exist", arm.name)
			}
			if !strings.Contains(arm.src, "package mutant") {
				t.Fatalf("fixture is not a mutant package — this arm would pass for the wrong reason")
			}
			report, err := analyzeUnits([]unit{{name: "mutant/mutant.go", src: arm.src}})
			if err != nil {
				t.Fatalf("analysing the mutant: %v", err)
			}
			if report.FilesParsed != 1 {
				t.Fatalf("the mutant corpus must hold exactly 1 file, the analyser parsed %d", report.FilesParsed)
			}

			if arm.wantKind == "" {
				if len(report.Findings) != 0 {
					for _, finding := range report.Findings {
						t.Errorf("control arm produced a finding: %s", finding)
					}
					t.Fatalf("the control arm must produce NO finding — every negative arm below is otherwise meaningless")
				}
				t.Logf("control: 0 finding(s) over %d write site(s), %d in population", report.WriteSites, report.InPopulation)
				return
			}

			var matched []Finding
			for _, finding := range report.Findings {
				if finding.Kind == arm.wantKind && finding.Path == arm.wantPath {
					matched = append(matched, finding)
				}
			}
			if len(matched) == 0 {
				t.Errorf("analyser did NOT flag %q as %s on path %q — this mutation would pass the gate unnoticed", arm.name, arm.wantKind, arm.wantPath)
				for _, finding := range report.Findings {
					t.Logf("  what it did report: %s", finding)
				}
				t.FailNow()
			}
			for _, finding := range matched {
				if !strings.HasSuffix(finding.File, "mutant.go") {
					t.Errorf("finding does not name the offending artifact: %s", finding)
				}
			}
			t.Logf("%s → %v", arm.name, matched)
		})
	}
}

// TestCleanIsNotTransparentForProvenance is the T4 arm of the threat model: an
// analyser that only asked "does the flow pass through pathguard" approves the
// escape, because it does pass.
func TestCleanIsNotTransparentForProvenance(t *testing.T) {
	cases := []struct {
		name string
		root string
		want string // "" = resolved, otherwise the kind expected
	}{
		{"clean-of-cwd", "filepath.Clean(cwd)", kindRootUnresolved},
		{"clean-wrapping-a-resolver", "filepath.Clean(resolvedRoot)", kindRootUnresolved},
		{"abs-is-not-a-resolver", "absRoot", kindRootUnresolved},
		{"evalsymlinks-is-the-resolver", "resolvedRoot", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			src := `package mutant

import (
	"os"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

func writeIt(name string) error {
	cwd, _ := os.Getwd()
	absRoot, _ := filepath.Abs(cwd)
	resolvedRoot, _ := filepath.EvalSymlinks(cwd)
	target := filepath.Join(` + testCase.root + `, name)
	if guardErr := pathguard.RejectAndReport(` + testCase.root + `, target); guardErr != nil {
		return guardErr
	}
	return os.WriteFile(target, nil, 0644)
}
`
			report, err := analyzeUnits([]unit{{name: "mutant/provenance.go", src: src}})
			if err != nil {
				t.Fatalf("analysing the mutant: %v", err)
			}
			flagged := len(report.P2Unresolved) > 0
			if testCase.want == kindRootUnresolved && !flagged {
				t.Fatalf("P2 did NOT flag root %q — a guard rooted in the logical namespace checks a different path than the one being written, which is the live escape ML-7A had to close", testCase.root)
			}
			if testCase.want == "" && flagged {
				t.Fatalf("P2 flagged root %q, which IS EvalSymlinks-resolved — a P2 that refuses the correct shape would force the tree to work around it, and that is how an exception list is born", testCase.root)
			}
			t.Logf("root=%s unresolved=%v", testCase.root, flagged)
		})
	}
}
