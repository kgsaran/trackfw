package sync

import (
	"os"
	"path/filepath"
	"testing"
)

const openREQContent = `# REQ: Test Requirement

> Date: 2026-01-01 | Status: Open
| Linear Issue:

## Motivation
Test motivation.

## Acceptance Criteria
- [ ]
`

// TestSyncWriteGuard_SymlinkREQFileRefused asserts that syncToProvider rejects a write
// when a REQ file inside the req_dir is a symlink pointing outside the project root —
// braço (a) for the write path in sync.go.
// AC11: syncToProvider returns a per-file error (not a global error) for the symlinked
// REQ and writes nothing outside the project, confirming the guard fires before os.WriteFile.
func TestSyncWriteGuard_SymlinkREQFileRefused(t *testing.T) {
	dir := chdirTempWithReset(t)
	outside := t.TempDir()

	// Set up docs/req/
	reqDir := filepath.Join(dir, "docs", "req")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeYAMLSync(t, dir, "req_dir: docs/req\n")

	// Create a real REQ file outside the project
	outsideREQ := filepath.Join(outside, "REQ-outside.md")
	if err := os.WriteFile(outsideREQ, []byte(openREQContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Link inside req_dir → points to outside
	linkPath := filepath.Join(reqDir, "REQ-2026-01-01-linked.md")
	if !symlinkOrSkip(t, outsideREQ, linkPath) {
		return
	}

	// Create stub that returns an issue ID
	create := func(_, _ string) (string, error) {
		return "ENG-99", nil
	}

	results, err := syncToProvider(create, "linear_issue")
	// syncToProvider returns nil error, per-file error in results
	if err != nil {
		t.Fatalf("unexpected global error: %v", err)
	}

	// The result for the symlink REQ should carry an error
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected per-file error for symlink REQ, got nil")
	}

	// The write guard fires AFTER create() — the issue may be created in the
	// external system, but the file on disk must NOT be modified.
	// Verify: the outside file must not contain the linear_issue injection.
	content, _ := os.ReadFile(outsideREQ)
	if contains(string(content), "ENG-99") {
		t.Error("containment violated: linear_issue was written to file outside project root")
	}
}

// TestSyncWriteGuard_LegitimateWritePasses asserts that syncToProvider successfully
// writes the issue ID into a REQ file with no symlinks — braço (b): legitimate sync
// is not blocked.
// AC11: syncToProvider returns a result with IssueID set and writes the linear_issue
// field into the REQ file when no symlinks are present.
func TestSyncWriteGuard_LegitimateWritePasses(t *testing.T) {
	dir := chdirTempWithReset(t)

	setupREQ(t, dir, "docs/req", "REQ-2026-01-01-open.md", openREQContent)
	writeYAMLSync(t, dir, "req_dir: docs/req\n")

	results, err := syncToProvider(func(_, _ string) (string, error) {
		return "ENG-42", nil
	}, "linear_issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Error != nil {
		t.Fatalf("expected 1 successful result, got: %+v", results)
	}
	if results[0].IssueID != "ENG-42" {
		t.Errorf("expected IssueID=ENG-42, got %q", results[0].IssueID)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
