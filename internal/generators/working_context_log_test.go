package generators

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestContextWorkingContextEntries_TenMostRecent affirms that with 12 .md entries
// in docs/agents-working-context/, recentWorkingContextEntries returns exactly
// the 10 lexicographically greatest names in descending order.
func TestContextWorkingContextEntries_TenMostRecent(t *testing.T) {
	dir := t.TempDir()
	wctx := filepath.Join(dir, "docs", "agents-working-context")
	if err := os.MkdirAll(wctx, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 12 entries with sortable ISO-8601-like names; the oldest (index 0) must
	// be excluded from the top-10 result.
	names := []string{
		"20261022T100001Z-apolo-ml1a-inicio.md",
		"20261022T100002Z-apolo-ml1a-fim.md",
		"20261022T100003Z-ares-ml1b-inicio.md",
		"20261022T100004Z-ares-ml1b-fim.md",
		"20261022T100005Z-zeus-ml2a-inicio.md",
		"20261022T100006Z-zeus-ml2a-fim.md",
		"20261022T100007Z-hades-ml0a-inicio.md",
		"20261022T100008Z-hades-ml0a-fim.md",
		"20261022T100009Z-prometeu-ml2a-inicio.md",
		"20261022T100010Z-prometeu-ml2a-fim.md",
		"20261022T100011Z-apolo-ml1c-inicio.md",
		"20261022T100012Z-apolo-ml1c-fim.md",
	}
	for i, name := range names {
		var content string
		if i == 0 {
			content = "no heading here\n" // oldest; no ## heading
		} else {
			content = "## entry-" + name + "\n\n- item\n"
		}
		if err := os.WriteFile(filepath.Join(wctx, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	got := recentWorkingContextEntries(wctx, 10)
	if len(got) != 10 {
		t.Fatalf("expected 10 entries, got %d", len(got))
	}
	// First entry: lexicographically greatest (newest).
	if got[0].Name != names[11] {
		t.Errorf("first entry: got %q, want %q", got[0].Name, names[11])
	}
	// Last entry of the 10: should be names[2] (third oldest, first not excluded).
	if got[9].Name != names[2] {
		t.Errorf("last entry: got %q, want %q", got[9].Name, names[2])
	}
	// Oldest entry (names[0]) must not appear.
	for _, e := range got {
		if e.Name == names[0] {
			t.Errorf("oldest entry %q must be excluded from top-10 result", names[0])
		}
	}
	// Heading of the newest entry must be extracted.
	if got[0].Heading == "" {
		t.Error("most recent entry heading must not be empty")
	}
	// Verify heading-absent case via direct call.
	if h := extractFirstHeading(filepath.Join(wctx, names[0])); h != "" {
		t.Errorf("extractFirstHeading on file without ## heading: got %q, want empty", h)
	}
}

// TestContextWorkingContextEntries_AbsentDir affirms that an absent directory
// returns an empty slice without error (no panic, no non-zero exit).
func TestContextWorkingContextEntries_AbsentDir(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "does-not-exist", "agents-working-context")
	got := recentWorkingContextEntries(absent, 10)
	if len(got) != 0 {
		t.Errorf("absent dir: expected 0 entries, got %d", len(got))
	}
}

// TestContextWorkingContextEntries_SymlinkIgnored affirms that a symlink inside
// docs/agents-working-context/ is silently skipped and does not appear in the
// result.
func TestContextWorkingContextEntries_SymlinkIgnored(t *testing.T) {
	dir := t.TempDir()
	wctx := filepath.Join(dir, "docs", "agents-working-context")
	if err := os.MkdirAll(wctx, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	regular := "20261022T100001Z-apolo-ml1a.md"
	if err := os.WriteFile(filepath.Join(wctx, regular), []byte("## heading\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Point the symlink to a file outside the directory.
	extTarget := filepath.Join(dir, "external.md")
	if err := os.WriteFile(extTarget, []byte("## external\n"), 0644); err != nil {
		t.Fatalf("create external target: %v", err)
	}
	symlinkName := "20261022T200000Z-symlink-entry.md"
	symlinkPath := filepath.Join(wctx, symlinkName)
	if !symlinkOrSkipWorkingCtx(t, extTarget, symlinkPath) {
		return // privilege missing; test skipped
	}

	got := recentWorkingContextEntries(wctx, 10)
	if len(got) != 1 {
		t.Errorf("expected 1 entry (symlink excluded), got %d: %v", len(got), got)
	}
	for _, e := range got {
		if e.Name == symlinkName {
			t.Errorf("symlink %q must be excluded from results", symlinkName)
		}
	}
	if len(got) > 0 && got[0].Name != regular {
		t.Errorf("expected regular entry %q, got %q", regular, got[0].Name)
	}
}

// symlinkOrSkipWorkingCtx creates a symlink at link pointing to target.
// Returns false (and skips) when the OS requires a privilege not present
// (Windows without Developer Mode). Fatal for any other error.
// Follows the approved pattern from internal/translog (check-symlink-privilege-guard.sh).
func symlinkOrSkipWorkingCtx(t *testing.T, target, link string) bool {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if isSymlinkPrivilegeErrorWCtx(err) {
		t.Skipf("symlink privilege not available (Windows without Developer Mode): %v", err)
		return false
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
	return false
}

// isSymlinkPrivilegeErrorWCtx reports whether err is a privilege-missing
// symlink creation failure. Checked by error value, not by GOOS.
func isSymlinkPrivilegeErrorWCtx(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var e syscall.Errno
	if errors.As(err, &e) && e == 1314 { // ERROR_PRIVILEGE_NOT_HELD on Windows
		return true
	}
	return false
}

// TestTrackfwRulesBlockUsesWorkingContextDir affirms that the generated governance
// rules block instructs reading from docs/agents-working-context/ and writing a
// new file there, and does NOT reference docs/agents-working-context.md as a
// read/write target. This directly tests agentfiles.go:92-93.
func TestTrackfwRulesBlockUsesWorkingContextDir(t *testing.T) {
	block := trackfwRulesBlock("", "", nil)

	if !strings.Contains(block, "docs/agents-working-context/") {
		t.Error("trackfwRulesBlock: must contain docs/agents-working-context/")
	}
	if strings.Contains(block, "docs/agents-working-context.md") {
		t.Error("trackfwRulesBlock: must not reference docs/agents-working-context.md (legacy frozen file)")
	}
}

// TestTrackfwRulesBlockAfterFinishingSubagentsNeverWrite affirms ML-1B (D1):
// line 3 of the Agent Protocol in the generated governance rules block states
// that the orchestrating session (trackfw_architect) writes the working-context
// entry and that subagents never write there. This test FAILS on HEAD before
// this ML is applied (the old text says "write a new file" without the subagent
// restriction) and PASSES on the branch after the edit to agentfiles.go:94.
func TestTrackfwRulesBlockAfterFinishingSubagentsNeverWrite(t *testing.T) {
	block := trackfwRulesBlock("", "flat", nil)

	const wantOrchestrator = "the orchestrating session (`trackfw_architect`) writes a new file in"
	const wantSubagentRule = "Subagents never write there"

	if !strings.Contains(block, wantOrchestrator) {
		t.Errorf("trackfwRulesBlock: line 3 must state that the orchestrating session writes the entry.\nwant substring: %q\ngot block:\n%s", wantOrchestrator, block)
	}
	if !strings.Contains(block, wantSubagentRule) {
		t.Errorf("trackfwRulesBlock: line 3 must prohibit subagents from writing to docs/agents-working-context/.\nwant substring: %q\ngot block:\n%s", wantSubagentRule, block)
	}
}
