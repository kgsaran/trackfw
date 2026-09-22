package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAppendAgentToConfig_EmptyRoot_FailClosed asserts that AppendAgentToConfig
// returns an error and writes nothing when root is empty — fail-closed behaviour (ML-5A).
//
// Reconciliation sentence (Regra Dura de Reconciliação): this test affirms the
// ML-5A conclusion that "precondition failure must be fail-closed" — AppendAgentToConfig
// must return a non-nil error (containing "cannot verify containment") and leave
// the config file unmodified when the project root is unknown (root == "").
//
// Old-code proof: before the fix, `if root != ""` was false when root == "", so the
// guard block was skipped entirely and os.WriteFile ran unconditionally. This test
// would have failed against the old code:
//   - want: err != nil (refusing write)
//   - got:  err == nil (file written despite empty root)
func TestAppendAgentToConfig_EmptyRoot_FailClosed(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "trackfw.yaml")

	// Write an initial file with roadmap_namespacing: by_agent and an agents: section.
	// The by_agent mode is required for appendAgentTextual to be called; flat mode
	// returns early before reaching the guard.
	initial := "roadmap_namespacing: by_agent\nagents:\n  - alpha\n"
	if err := os.WriteFile(p, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Pass root == "" to trigger the fail-closed path.
	err := AppendAgentToConfig("", p, "beta")
	if err == nil {
		t.Fatal("AppendAgentToConfig must return an error when root is empty (fail-closed), got nil")
	}
	if !containsAgentReg(err.Error(), "cannot verify containment") {
		t.Fatalf("error must mention 'cannot verify containment', got: %v", err)
	}

	// The file must remain unmodified.
	content, readErr := os.ReadFile(p)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != initial {
		t.Errorf("file was modified despite empty root:\n got: %q\nwant: %q", string(content), initial)
	}
}

// containsAgentReg reports whether sub is a substring of s.
func containsAgentReg(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
