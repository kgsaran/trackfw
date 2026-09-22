package validator

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveBaseline_SymlinkLeafRefused asserts that SaveBaseline rejects a write
// when .trackfw-baseline.json is a symlink pointing outside the project root —
// braço (a) for type (i) relative-path sites.
// AC11: SaveBaseline returns a non-nil error and writes nothing outside the project
// when .trackfw-baseline.json is a symlink, confirming the guard fires before os.WriteFile.
func TestSaveBaseline_SymlinkLeafRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// .trackfw-baseline.json → symlink pointing outside project
	outsideBaseline := filepath.Join(outside, ".trackfw-baseline.json")
	if !symlinkOrSkip(t, outsideBaseline, filepath.Join(dir, baselineFileName)) {
		return
	}

	err = SaveBaseline([]string{"test-violation"}, nil)
	if err == nil {
		t.Fatal("SaveBaseline should refuse when .trackfw-baseline.json is a symlink, got nil")
	}
	// Nothing should be written to the target outside the project
	if _, statErr := os.Stat(outsideBaseline); statErr == nil {
		t.Error("containment violated: .trackfw-baseline.json was written outside the project root")
	}
}

// TestSaveBaseline_LegitimateWritePasses asserts that SaveBaseline succeeds in a
// clean project directory with no symlinks — braço (b): legitimate validate --baseline
// is not blocked.
// AC11: SaveBaseline returns nil and creates .trackfw-baseline.json in CWD when no
// symlinks are present, confirming the guard does not break normal validate execution.
func TestSaveBaseline_LegitimateWritePasses(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := SaveBaseline([]string{"existing-violation"}, []string{"a-warning"}); err != nil {
		t.Fatalf("SaveBaseline should succeed with clean CWD, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, baselineFileName)); statErr != nil {
		t.Errorf(".trackfw-baseline.json should exist after legitimate write: %v", statErr)
	}
}
