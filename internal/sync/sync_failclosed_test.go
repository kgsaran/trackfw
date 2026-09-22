package sync

import (
	"errors"
	"os"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestSyncToProvider_GetWdFailure_FailClosed asserts that syncToProvider returns a
// non-nil error and writes nothing when syncGetwdFn returns an error —
// fail-closed behaviour (ML-5A).
//
// Reconciliation sentence (Regra Dura de Reconciliação): this test affirms the
// ML-5A conclusion that "precondition failure must be fail-closed" — syncToProvider
// must return a global non-nil error (containing "cannot verify containment") and leave
// all REQ files unmodified when the CWD cannot be determined.
//
// Old-code proof: before the fix, syncRootErr != nil caused the per-write
// `if syncRootErr == nil` guard to be skipped, so os.WriteFile ran for each REQ
// with no containment check. This test would have failed against the old code:
//   - want: global err != nil (fail-closed before loop)
//   - got:  global err == nil (loop ran and wrote files without containment check)
func TestSyncToProvider_GetWdFailure_FailClosed(t *testing.T) {
	dir := chdirTempWithReset(t)
	config.Reset()
	t.Cleanup(config.Reset)

	// Set up a REQ file with status Open so the loop would be entered.
	setupREQ(t, dir, "docs/req", "REQ-2026-01-01-open.md", openREQContent)
	writeYAMLSync(t, dir, "req_dir: docs/req\n")

	// Read the file before the call so we can verify it's unchanged after.
	reqPath := dir + "/docs/req/REQ-2026-01-01-open.md"
	before, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatal(err)
	}

	// Temporarily override the Getwd seam to inject a failure.
	orig := syncGetwdFn
	syncGetwdFn = func() (string, error) {
		return "", errors.New("injected: no working directory")
	}
	t.Cleanup(func() { syncGetwdFn = orig })

	results, syncErr := syncToProvider(func(_, _ string) (string, error) {
		return "ENG-99", nil
	}, "linear_issue")

	// Must return a global error — not per-file results.
	if syncErr == nil {
		t.Fatal("syncToProvider must return a global error when Getwd() fails (fail-closed), got nil")
	}
	if !containsSync(syncErr.Error(), "cannot verify containment") {
		t.Fatalf("error must mention 'cannot verify containment', got: %v", syncErr)
	}
	if results != nil {
		t.Errorf("results must be nil on fail-closed error, got: %v", results)
	}

	// The REQ file must remain unmodified.
	after, readErr := os.ReadFile(reqPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Error("containment violated: REQ file was modified even though Getwd() failed")
	}
}

// containsSync reports whether sub is a substring of s.
func containsSync(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
