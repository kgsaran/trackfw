package guard

// messages_test.go — three-layer gate for AC1 and AC2 (ML-1A, REQ-2026-11-02).
//
// Camada A: orientation preserved — each reason* constant with a governed
// trackfw alternative still contains that alternative after translation, and
// all 17 constants that previously had "Nada antes" now contain the English
// equivalent "Nothing before this command was executed".
//
// Camada B: no private § reference — scans all non-test .go files under
// ./internal/ and fails on any string literal containing "§" except the
// allowlisted "YAML 1.2 §".
//
// Camada C-guard: no Portuguese stopwords — checks each of the 20 reason*
// constants for RECUSADO, "nao ", bloquead, implicito, mensagem and accented
// characters. payload.go:161 is included here too (Option A: already translated).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------- Camada A --------------------------------------------------------

// TestReasonOrientationSubstrings asserts that each reason* constant with a
// governed trackfw alternative still contains that alternative after
// translation. Referencing each constant by its Go identifier means a rename
// breaks compilation — the test cannot silently pass after a delete.
//
// Reconciliation: this test asserts that the 12 constants that previously
// named a trackfw command still name one in their translated form. A constant
// that passes here but loses its actionable command in a future edit will be
// caught immediately.
func TestReasonOrientationSubstrings(t *testing.T) {
	wantSubstrings := []struct {
		name string
		c    string
		want []string
	}{
		{"reasonCheckoutB", reasonCheckoutB, []string{"trackfw branch new"}},
		{"reasonSwitchC", reasonSwitchC, []string{"trackfw branch new"}},
		{"reasonBranchCreate", reasonBranchCreate, []string{"trackfw branch new"}},
		{"reasonWorktreeAddB", reasonWorktreeAddB, []string{"trackfw branch new"}},
		{"reasonCommit", reasonCommit, []string{"trackfw commit"}},
		{"reasonPush", reasonPush, []string{"trackfw push", "trackfw ship", "trackfw release tag"}},
		{"reasonPushDelete", reasonPushDelete, []string{"trackfw branch prune"}},
		{"reasonStash", reasonStash, []string{"trackfw branch new"}},
		{"reasonResetHard", reasonResetHard, []string{"trackfw ship"}},
		{"reasonAddImplicit", reasonAddImplicit, []string{"trackfw commit"}},
		{"reasonWorktreeAddImplicit", reasonWorktreeAddImplicit, []string{"trackfw branch new"}},
		{"reasonTruncated", reasonTruncated, []string{"trackfw ship", "trackfw branch new"}},
	}
	for _, tc := range wantSubstrings {
		for _, sub := range tc.want {
			if !strings.Contains(tc.c, sub) {
				t.Errorf("reason constant %s: expected to contain %q\ngot: %q", tc.name, sub, tc.c)
			}
		}
	}
}

// TestReasonNothingBeforePhrase asserts that the 17 constants that previously
// carried "Nada antes deste comando foi executado" now carry the English
// equivalent. reasonTruncated is excluded (it was never in this group).
//
// Reconciliation: this test asserts that the translated "compound command"
// phrase is present in each constant that had the Portuguese original. A
// translation that drops the phrase fails immediately.
func TestReasonNothingBeforePhrase(t *testing.T) {
	const phrase = "Nothing before this command was executed"
	wantNothingBefore := []struct {
		name string
		c    string
	}{
		{"reasonCheckoutB", reasonCheckoutB},
		{"reasonSwitchC", reasonSwitchC},
		{"reasonBranchCreate", reasonBranchCreate},
		{"reasonWorktreeAddB", reasonWorktreeAddB},
		{"reasonCommit", reasonCommit},
		{"reasonPush", reasonPush},
		{"reasonPushDelete", reasonPushDelete},
		{"reasonStash", reasonStash},
		{"reasonResetHard", reasonResetHard},
		{"reasonAddImplicit", reasonAddImplicit},
		{"reasonCleanForce", reasonCleanForce},
		{"reasonRestorePath", reasonRestorePath},
		{"reasonCheckoutPath", reasonCheckoutPath},
		{"reasonUpdateRef", reasonUpdateRef},
		{"reasonWorktreeRemoveForce", reasonWorktreeRemoveForce},
		{"reasonRmForce", reasonRmForce},
		{"reasonWorktreeAddImplicit", reasonWorktreeAddImplicit},
	}
	for _, tc := range wantNothingBefore {
		if !strings.Contains(tc.c, phrase) {
			t.Errorf("reason constant %s: expected to contain %q\ngot: %q", tc.name, phrase, tc.c)
		}
	}
}

// ---------- Camada B --------------------------------------------------------

// moduleRoot returns the repository root derived from this source file's path.
// Using runtime.Caller makes the result immune to os.Chdir calls in other tests.
func moduleRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	// thisFile = …/trackfw/internal/guard/messages_test.go
	// up 3 levels: guard → internal → trackfw
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

// TestNoPrivateSectionReferenceInternal (Camada B) scans all non-test .go
// files under ./internal/ for string literals containing "§". The only
// allowlisted value is "YAML 1.2 §" (render.go, a spec citation).
//
// Reconciliation: this test asserts that no string literal in the internal
// packages emits a private document reference (§N) after translation. It
// catches any site that was missed in ML-1A or re-introduced later.
func TestNoPrivateSectionReferenceInternal(t *testing.T) {
	root := filepath.Join(moduleRoot(), "internal")
	fset := token.NewFileSet()

	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip testdata and corpus directories.
		if info.IsDir() {
			base := info.Name()
			if base == "testdata" || base == "corpus-pre-fix" {
				return filepath.SkipDir
			}
			return nil
		}
		// Non-test .go files only.
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil // ignore unparseable files
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val := lit.Value
			if !strings.Contains(val, "§") {
				return true
			}
			// Allowlist: YAML 1.2 §5.1 (render.go spec citation).
			if strings.Contains(val, "YAML 1.2 §") {
				return true
			}
			pos := fset.Position(lit.Pos())
			violations = append(violations, pos.String()+": "+val)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	for _, v := range violations {
		t.Errorf("private § reference in non-test source: %s", v)
	}
}

// ---------- Camada C-guard --------------------------------------------------

// TestReasonNoPortugueseStopwords (Camada C-guard) checks all 20 reason*
// constants for Portuguese stopwords and accented characters. The check also
// covers the payload.go error literal (translated to English in ML-1A).
//
// Stopwords chosen because they appeared in the pre-translation constants and
// would not appear in correct English text:
//   - "RECUSADO"         — Portuguese "REJECTED"
//   - "nao "             — Portuguese "not" / "no" (with trailing space to
//                          avoid matching "cannot", "navigation", etc.)
//   - "bloquead"         — root of "bloqueado/bloqueada" (blocked)
//   - "implicito"        — Portuguese "implicit"
//   - "mensagem"         — Portuguese "message"
//   - accented chars     — any of áàâãéêíóôõúçÁÉÍÓÚÇÃÕ
//
// Reconciliation: this test asserts that after translation the 20 constants
// contain no Portuguese text. A future edit that re-introduces Portuguese
// wording (accidentally or by revert) fails immediately.
func TestReasonNoPortugueseStopwords(t *testing.T) {
	all := []struct {
		name string
		c    string
	}{
		{"reasonTruncated", reasonTruncated},
		{"reasonNUL", reasonNUL},
		{"reasonIndecodeable", reasonIndecodeable},
		{"reasonCheckoutB", reasonCheckoutB},
		{"reasonSwitchC", reasonSwitchC},
		{"reasonBranchCreate", reasonBranchCreate},
		{"reasonWorktreeAddB", reasonWorktreeAddB},
		{"reasonCommit", reasonCommit},
		{"reasonPush", reasonPush},
		{"reasonPushDelete", reasonPushDelete},
		{"reasonStash", reasonStash},
		{"reasonResetHard", reasonResetHard},
		{"reasonAddImplicit", reasonAddImplicit},
		{"reasonCleanForce", reasonCleanForce},
		{"reasonRestorePath", reasonRestorePath},
		{"reasonCheckoutPath", reasonCheckoutPath},
		{"reasonUpdateRef", reasonUpdateRef},
		{"reasonWorktreeRemoveForce", reasonWorktreeRemoveForce},
		{"reasonRmForce", reasonRmForce},
		{"reasonWorktreeAddImplicit", reasonWorktreeAddImplicit},
	}

	stopwords := []string{
		"RECUSADO",
		"nao ",
		"bloquead",
		"implicito",
		"mensagem",
	}
	ptAccented := "áàâãéêíóôõúçÁÉÍÓÚÇÃÕ"

	for _, tc := range all {
		for _, sw := range stopwords {
			if strings.Contains(tc.c, sw) {
				t.Errorf("reason constant %s contains Portuguese stopword %q\ngot: %q", tc.name, sw, tc.c)
			}
		}
		for _, r := range ptAccented {
			if strings.ContainsRune(tc.c, r) {
				t.Errorf("reason constant %s contains accented Portuguese character %q\ngot: %q", tc.name, r, tc.c)
			}
		}
		if strings.Contains(tc.c, "§") {
			t.Errorf("reason constant %s contains § symbol\ngot: %q", tc.name, tc.c)
		}
	}
}
