package integrations

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteManifest_SymlinkTrackfwDirRefused asserts that writeManifest rejects a
// write when the .trackfw/ directory is a symlink pointing outside the project root —
// braço (a) for type (ii) wrapper sites.
// AC11: writeManifest returns a non-nil error and writes nothing outside the project
// when .trackfw/ is a symlink, confirming the guard fires before atomicWrite.
func TestWriteManifest_SymlinkTrackfwDirRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()

	// .trackfw/ → symlink to outside
	if !symlinkOrSkip(t, outside, filepath.Join(dir, ".trackfw")) {
		return
	}

	filename := manifestPath(dir)
	manifest := emptyManifest()
	err := writeManifest(filename, manifest)
	if err == nil {
		t.Fatal("writeManifest should refuse when .trackfw/ is a symlink, got nil")
	}
	// Nothing should be created outside
	if _, statErr := os.Stat(filepath.Join(outside, "integrations-manifest.json")); statErr == nil {
		t.Error("containment violated: integrations-manifest.json was written outside the project root")
	}
}

// TestWriteManifest_LegitimateWritePasses asserts that writeManifest succeeds in a
// clean project directory with no symlinks — braço (b): legitimate manifest persistence
// is not blocked.
// AC11: writeManifest returns nil and creates the manifest file under .trackfw/ when
// no symlinks are present, confirming the guard does not break normal install/update.
func TestWriteManifest_LegitimateWritePasses(t *testing.T) {
	dir := t.TempDir()
	filename := manifestPath(dir)

	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	err := writeManifest(filename, emptyManifest())
	if err != nil {
		t.Fatalf("writeManifest should succeed with clean path, got: %v", err)
	}
	if _, statErr := os.Stat(filename); statErr != nil {
		t.Errorf("manifest should exist after legitimate write: %v", statErr)
	}
}
