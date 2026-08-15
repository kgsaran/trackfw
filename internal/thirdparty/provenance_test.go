package thirdparty

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestUpsertProvenanceEntry_RedactsQueryStringOnDisk is the D6-bis
// falsification test named by the ML-4C AC, provenance side: even though
// no command in this codebase writes ProvenanceEntry.URL today (D10.2 —
// written externally by the approver), WriteProvenance/UpsertProvenanceEntry
// must redact it as a defense-in-depth boundary. Grep the raw bytes on
// disk, not just the in-memory struct.
func TestUpsertProvenanceEntry_RedactsQueryStringOnDisk(t *testing.T) {
	root := t.TempDir()
	entry := ProvenanceEntry{
		URL:            "https://example.com/skills/my-skill.md?token=super-secret-value",
		ChecksumSHA256: "abc123",
		ApprovedBy:     "hades-tf",
	}
	if err := UpsertProvenanceEntry(root, "dest/my-skill.md", entry); err != nil {
		t.Fatalf("UpsertProvenanceEntry: %v", err)
	}
	raw, err := os.ReadFile(ProvenancePath(root))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "super-secret-value") {
		t.Fatalf("provenance file on disk leaked the query-string token:\n%s", raw)
	}
	if !strings.Contains(string(raw), "[redacted]") {
		t.Fatalf("expected the redacted marker in the provenance file on disk:\n%s", raw)
	}
}

func TestProvenanceRoundTripPreservesAllFields(t *testing.T) {
	root := t.TempDir()

	entry := ProvenanceEntry{
		URL:             "https://example.com/skill.md",
		ChecksumSHA256:  "abc123",
		InstalledSHA256: "def456",
		InstalledAt:     "2026-08-15T14:32:00Z",
		ApprovedBy:      "hades-tf",
		ReviewReference: "docs/seguranca/2026-08-15-skill-de-terceiro.md",
		Scope:           "project",
		MarkerOverride:  true,
	}

	if err := UpsertProvenanceEntry(root, "apolo-tf/skills/thirdparty/x.md", entry); err != nil {
		t.Fatalf("UpsertProvenanceEntry: %v", err)
	}

	prov, err := LoadProvenance(root)
	if err != nil {
		t.Fatalf("LoadProvenance: %v", err)
	}
	if prov.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2", prov.SchemaVersion)
	}
	got, ok := prov.Entries["apolo-tf/skills/thirdparty/x.md"]
	if !ok {
		t.Fatal("expected entry for destination, got none")
	}
	if got != entry {
		t.Errorf("round-tripped entry = %+v, want %+v", got, entry)
	}
}

func TestProvenanceUpsertPreservesOtherEntries(t *testing.T) {
	root := t.TempDir()

	first := ProvenanceEntry{URL: "https://example.com/a.md", ChecksumSHA256: "aaa", ApprovedBy: "hades-tf"}
	second := ProvenanceEntry{URL: "https://example.com/b.md", ChecksumSHA256: "bbb", ApprovedBy: "hades-tf"}

	if err := UpsertProvenanceEntry(root, "dest/a.md", first); err != nil {
		t.Fatalf("UpsertProvenanceEntry(a): %v", err)
	}
	if err := UpsertProvenanceEntry(root, "dest/b.md", second); err != nil {
		t.Fatalf("UpsertProvenanceEntry(b): %v", err)
	}

	prov, err := LoadProvenance(root)
	if err != nil {
		t.Fatalf("LoadProvenance: %v", err)
	}
	if len(prov.Entries) != 2 {
		t.Fatalf("Entries = %v, want 2 entries", prov.Entries)
	}
	if prov.Entries["dest/a.md"] != first {
		t.Errorf("dest/a.md entry = %+v, want %+v", prov.Entries["dest/a.md"], first)
	}
	if prov.Entries["dest/b.md"] != second {
		t.Errorf("dest/b.md entry = %+v, want %+v", prov.Entries["dest/b.md"], second)
	}
}

func TestLoadProvenanceMissingFileIsEmptyNotError(t *testing.T) {
	root := t.TempDir()
	prov, err := LoadProvenance(root)
	if err != nil {
		t.Fatalf("LoadProvenance on missing file: %v", err)
	}
	if prov.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2", prov.SchemaVersion)
	}
	if len(prov.Entries) != 0 {
		t.Errorf("Entries = %v, want empty", prov.Entries)
	}
}

func TestLoadProvenanceMalformedJSONIsError(t *testing.T) {
	root := t.TempDir()
	path := ProvenancePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := LoadProvenance(root); err == nil {
		t.Fatal("expected error for malformed provenance JSON, got nil")
	}
}

// TestLoadProvenanceUnsupportedSchemaVersionIsError uses schema_version 1
// deliberately — the AC for the 1->2 bump (ML-3B, D2-bis) is "version 1
// refused, fail-closed", and 1 is the version a stale working copy could
// plausibly still have on disk (a provenance file written before this
// bump), unlike an arbitrary future number. Still discriminating: 1 !=
// provenanceSchemaVersion (2).
func TestLoadProvenanceUnsupportedSchemaVersionIsError(t *testing.T) {
	root := t.TempDir()
	path := ProvenancePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	body := `{"schema_version":1,"entries":{}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := LoadProvenance(root); err == nil {
		t.Fatal("expected error for schema_version 1, got nil")
	}
}

func TestWriteProvenanceFailureAbortsAndReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits behave differently on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission enforcement is not applicable")
	}

	root := t.TempDir()
	// Pre-create .trackfw as read-only so atomicWrite's os.CreateTemp
	// inside it fails — MkdirAll on an existing directory does not change
	// its mode, so this reliably forces the write to fail.
	trackfwDir := filepath.Join(root, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o500); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(trackfwDir, 0o700)
	})

	err := UpsertProvenanceEntry(root, "dest/x.md", ProvenanceEntry{ChecksumSHA256: "aaa", ApprovedBy: "hades-tf"})
	if err == nil {
		t.Fatal("expected UpsertProvenanceEntry to return an error when the provenance file cannot be written, got nil")
	}
}

func TestVerifyApprovalSucceedsForMatchingApprovedEntry(t *testing.T) {
	root := t.TempDir()
	entry := ProvenanceEntry{ChecksumSHA256: "abc123", ApprovedBy: "hades-tf"}
	if err := UpsertProvenanceEntry(root, "dest/x.md", entry); err != nil {
		t.Fatalf("UpsertProvenanceEntry: %v", err)
	}

	if err := VerifyApproval(root, "abc123", "dest/x.md"); err != nil {
		t.Fatalf("VerifyApproval: unexpected error: %v", err)
	}
}

func TestVerifyApprovalRejectsMissingDestination(t *testing.T) {
	root := t.TempDir()
	if err := VerifyApproval(root, "abc123", "dest/never-installed.md"); err == nil {
		t.Fatal("expected error for destination with no provenance entry, got nil")
	}
}

func TestVerifyApprovalRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	entry := ProvenanceEntry{ChecksumSHA256: "abc123", ApprovedBy: "hades-tf"}
	if err := UpsertProvenanceEntry(root, "dest/x.md", entry); err != nil {
		t.Fatalf("UpsertProvenanceEntry: %v", err)
	}

	if err := VerifyApproval(root, "different-checksum", "dest/x.md"); err == nil {
		t.Fatal("expected error for checksum mismatch, got nil")
	}
}

func TestVerifyApprovalRejectsEmptyApprovedBy(t *testing.T) {
	root := t.TempDir()
	entry := ProvenanceEntry{ChecksumSHA256: "abc123", ApprovedBy: ""}
	if err := UpsertProvenanceEntry(root, "dest/x.md", entry); err != nil {
		t.Fatalf("UpsertProvenanceEntry: %v", err)
	}

	if err := VerifyApproval(root, "abc123", "dest/x.md"); err == nil {
		t.Fatal("expected error for empty approved_by, got nil")
	}
}
