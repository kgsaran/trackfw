package integrations

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteThirdPartyReferenceRegistry_SymlinkTrackfwDirRefused asserts that
// writeThirdPartyReferenceRegistry rejects a write when .trackfw/ is a symlink
// pointing outside the project root — braço (a) for type (ii) wrapper sites.
// AC11: writeThirdPartyReferenceRegistry returns a non-nil error and writes nothing
// outside the project when .trackfw/ is a symlink, confirming the guard fires before atomicWrite.
func TestWriteThirdPartyReferenceRegistry_SymlinkTrackfwDirRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()

	// .trackfw/ → symlink to outside
	if !symlinkOrSkip(t, outside, filepath.Join(dir, ".trackfw")) {
		return
	}

	reg := thirdPartyReferenceRegistry{}
	err := writeThirdPartyReferenceRegistry(dir, reg)
	if err == nil {
		t.Fatal("writeThirdPartyReferenceRegistry should refuse when .trackfw/ is a symlink, got nil")
	}
	// Nothing should be written outside
	if _, statErr := os.Stat(filepath.Join(outside, "thirdparty-references.json")); statErr == nil {
		t.Error("containment violated: thirdparty-references.json was written outside the project root")
	}
}

// TestWriteThirdPartyReferenceRegistry_LegitimateWritePasses asserts that
// writeThirdPartyReferenceRegistry succeeds in a clean project directory — braço (b):
// legitimate third-party reference persistence is not blocked.
// AC11: writeThirdPartyReferenceRegistry returns nil and creates the registry file
// under .trackfw/ when no symlinks are present, confirming the guard does not break
// normal third-party install or UpsertThirdPartyReference calls.
func TestWriteThirdPartyReferenceRegistry_LegitimateWritePasses(t *testing.T) {
	dir := t.TempDir()
	trackfwDir := filepath.Join(dir, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o700); err != nil {
		t.Fatal(err)
	}

	reg := thirdPartyReferenceRegistry{}
	err := writeThirdPartyReferenceRegistry(dir, reg)
	if err != nil {
		t.Fatalf("writeThirdPartyReferenceRegistry should succeed with clean path, got: %v", err)
	}
	if _, statErr := os.Stat(thirdPartyReferencesPath(dir)); statErr != nil {
		t.Errorf("thirdparty-references.json should exist after legitimate write: %v", statErr)
	}
}
