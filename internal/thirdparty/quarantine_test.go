package thirdparty

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewQuarantineEntry_RedactsQueryStringOnDisk is the D6-bis
// falsification test named by the ML-4C AC: a URL with a query-string
// token must never reach the file written to disk — grep the raw bytes of
// the written quarantine record, not just the in-memory struct.
func TestNewQuarantineEntry_RedactsQueryStringOnDisk(t *testing.T) {
	root := t.TempDir()
	entry := NewQuarantineEntry("https://example.com/skills/my-skill.md?token=super-secret-value", []byte("content"), nil, "skill", nil)
	if err := WriteQuarantine(root, entry); err != nil {
		t.Fatalf("WriteQuarantine: %v", err)
	}
	raw, err := os.ReadFile(QuarantinePath(root, entry.ChecksumSHA256))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "super-secret-value") {
		t.Fatalf("quarantine record on disk leaked the query-string token:\n%s", raw)
	}
	if !strings.Contains(string(raw), "[redacted]") {
		t.Fatalf("expected the redacted marker in the quarantine record on disk:\n%s", raw)
	}
}

func TestQuarantineRoundTripPreservesAllFields(t *testing.T) {
	root := t.TempDir()

	// Content with UTF-8 multibyte runes and newlines, to exercise the
	// base64 round trip on non-ASCII/non-trivial input.
	raw := []byte("# Título de terceiro\n\nConteúdo com acentuação e emojis 🚀\nlinha final\n")

	entry := NewQuarantineEntry(
		"https://example.com/skill.md",
		raw,
		[]string{"git authority"},
		"skill",
		[]string{"hades-tf", "apolo-tf"},
	)

	if err := WriteQuarantine(root, entry); err != nil {
		t.Fatalf("WriteQuarantine: %v", err)
	}

	got, err := ReadQuarantine(root, entry.ChecksumSHA256)
	if err != nil {
		t.Fatalf("ReadQuarantine: %v", err)
	}

	if got.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", got.SchemaVersion)
	}
	if got.URL != entry.URL {
		t.Errorf("URL = %q, want %q", got.URL, entry.URL)
	}
	if got.ChecksumSHA256 != entry.ChecksumSHA256 {
		t.Errorf("ChecksumSHA256 = %q, want %q", got.ChecksumSHA256, entry.ChecksumSHA256)
	}
	if got.FetchedAt != entry.FetchedAt {
		t.Errorf("FetchedAt = %q, want %q", got.FetchedAt, entry.FetchedAt)
	}
	if got.Kind != "skill" {
		t.Errorf("Kind = %q, want %q", got.Kind, "skill")
	}
	if len(got.RequestedTargets) != 2 || got.RequestedTargets[0] != "hades-tf" || got.RequestedTargets[1] != "apolo-tf" {
		t.Errorf("RequestedTargets = %v, want [hades-tf apolo-tf]", got.RequestedTargets)
	}
	if got.MarkerCheck.Result != "fail" {
		t.Errorf("MarkerCheck.Result = %q, want %q", got.MarkerCheck.Result, "fail")
	}
	if len(got.MarkerCheck.MatchedMarkers) != 1 || got.MarkerCheck.MatchedMarkers[0] != "git authority" {
		t.Errorf("MarkerCheck.MatchedMarkers = %v, want [git authority]", got.MarkerCheck.MatchedMarkers)
	}

	decoded, err := got.DecodeContent()
	if err != nil {
		t.Fatalf("DecodeContent: %v", err)
	}
	if string(decoded) != string(raw) {
		t.Fatalf("decoded content mismatch:\n got:  %q\n want: %q", decoded, raw)
	}
}

func TestQuarantineMarkerCheckPassWhenNoMarkersMatched(t *testing.T) {
	root := t.TempDir()
	raw := []byte("# Clean skill\n\nNothing suspicious here.\n")

	entry := NewQuarantineEntry("https://example.com/clean.md", raw, nil, "agent", []string{"apolo-tf"})
	if entry.MarkerCheck.Result != "pass" {
		t.Fatalf("MarkerCheck.Result = %q, want %q", entry.MarkerCheck.Result, "pass")
	}

	if err := WriteQuarantine(root, entry); err != nil {
		t.Fatalf("WriteQuarantine: %v", err)
	}
	got, err := ReadQuarantine(root, entry.ChecksumSHA256)
	if err != nil {
		t.Fatalf("ReadQuarantine: %v", err)
	}
	if got.MarkerCheck.Result != "pass" {
		t.Errorf("MarkerCheck.Result = %q, want %q", got.MarkerCheck.Result, "pass")
	}
	if len(got.MarkerCheck.MatchedMarkers) != 0 {
		t.Errorf("MatchedMarkers = %v, want empty", got.MarkerCheck.MatchedMarkers)
	}
}

func TestQuarantineFilenameIsChecksum(t *testing.T) {
	root := t.TempDir()
	raw := []byte("content")
	entry := NewQuarantineEntry("https://example.com/x.md", raw, nil, "skill", nil)

	if err := WriteQuarantine(root, entry); err != nil {
		t.Fatalf("WriteQuarantine: %v", err)
	}

	wantPath := filepath.Join(root, ".trackfw", "thirdparty-quarantine", entry.ChecksumSHA256+".json")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected quarantine file at %s: %v", wantPath, err)
	}
}

func TestReadQuarantineMissingFileIsError(t *testing.T) {
	root := t.TempDir()
	if _, err := ReadQuarantine(root, "deadbeef"); err == nil {
		t.Fatal("expected error for missing quarantine file, got nil")
	}
}

func TestReadQuarantineMalformedJSONIsError(t *testing.T) {
	root := t.TempDir()
	checksum := "abc123"
	path := QuarantinePath(root, checksum)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ReadQuarantine(root, checksum); err == nil {
		t.Fatal("expected error for malformed quarantine JSON, got nil")
	}
}

func TestReadQuarantineUnsupportedSchemaVersionIsError(t *testing.T) {
	root := t.TempDir()
	checksum := "abc123"
	path := QuarantinePath(root, checksum)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	body := `{"schema_version":2,"url":"https://example.com","checksum_sha256":"abc123","fetched_at":"2026-08-15T14:20:00Z","content_base64":"eA==","marker_check":{"result":"pass","matched_markers":[]},"kind":"skill","requested_targets":[]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ReadQuarantine(root, checksum); err == nil {
		t.Fatal("expected error for schema_version 2, got nil")
	}
}
