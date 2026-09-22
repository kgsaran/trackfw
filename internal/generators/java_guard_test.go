package generators

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratePomXML_SymlinkLeafRefused asserts that GeneratePomXML rejects a
// write when pom.xml is a symlink pointing outside the project root — braço (a)
// for type (i) relative-path sites.
// AC11: GeneratePomXML returns a non-nil error and writes nothing outside the
// project when pom.xml is a symlink, confirming the guard fires before os.WriteFile.
func TestGeneratePomXML_SymlinkLeafRefused(t *testing.T) {
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

	// pom.xml → symlink pointing outside project
	symlinkOrSkip(t, filepath.Join(outside, "pom.xml"), filepath.Join(dir, "pom.xml"))

	err = GeneratePomXML(Config{ProjectType: "backend", ProjectName: "test"})
	if err == nil {
		t.Fatal("GeneratePomXML should refuse when pom.xml is a symlink, got nil")
	}
	// Nothing should exist at the symlink target
	if _, statErr := os.Stat(filepath.Join(outside, "pom.xml")); statErr == nil {
		t.Error("containment violated: pom.xml was written outside the project root")
	}
}

// TestGeneratePomXML_LegitimateWritePasses asserts that GeneratePomXML succeeds in a
// clean project directory with no symlinks — braço (b): legitimate operation unblocked.
// AC11: GeneratePomXML returns nil and creates pom.xml in the project root when no
// symlinks are present, confirming the guard does not break normal scaffold execution.
func TestGeneratePomXML_LegitimateWritePasses(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	err = GeneratePomXML(Config{ProjectType: "backend", ProjectName: "my-app"})
	if err != nil {
		t.Fatalf("GeneratePomXML should succeed with clean path, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "pom.xml")); statErr != nil {
		t.Errorf("pom.xml should exist after legitimate write: %v", statErr)
	}
}
