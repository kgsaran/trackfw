package validator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestSaveBaseline_GetWdFailure_FailClosed asserts that SaveBaseline returns an error
// and writes nothing when getwdFn returns an error — fail-closed behaviour (ML-5A).
//
// Reconciliation sentence (Regra Dura de Reconciliação): this test affirms the
// ML-5A conclusion that "precondition failure must be fail-closed" — SaveBaseline
// must return a non-nil error (containing "cannot verify containment") and leave
// the baseline file uncreated when the CWD cannot be determined.
//
// Old-code proof: before the fix, the `if cwdErr == nil` guard was skipped and
// os.WriteFile ran unconditionally, so the baseline file was written even when
// Getwd() failed. This test would have failed against the old code:
//   - want: err != nil (refusing write)
//   - got:  err == nil (write succeeded despite Getwd failure)
func TestSaveBaseline_GetWdFailure_FailClosed(t *testing.T) {
	dir := t.TempDir()

	// Temporarily override the Getwd seam to inject a failure.
	orig := getwdFn
	getwdFn = func() (string, error) {
		return "", errors.New("injected: no working directory")
	}
	t.Cleanup(func() { getwdFn = orig })

	// The baseline file must NOT be created.
	baselinePath := filepath.Join(dir, baselineFileName)

	// Change to the temp dir so any accidental write goes there (not the real CWD).
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	saveErr := SaveBaseline([]string{"v"}, nil)
	if saveErr == nil {
		t.Fatal("SaveBaseline must return an error when Getwd() fails (fail-closed), got nil")
	}
	if !contains5A(saveErr.Error(), "cannot verify containment") {
		t.Fatalf("error must mention 'cannot verify containment', got: %v", saveErr)
	}
	if _, statErr := os.Stat(baselinePath); statErr == nil {
		t.Error("containment violated: baseline file was written even though Getwd() failed")
	}
}

// contains5A reports whether sub is a substring of s. Named to avoid collision with
// the identically-named helper in sync's test files.
func contains5A(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
