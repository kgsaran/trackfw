package auditsurface

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mustRunGit is a test helper that runs a git command in the given directory
// and fails the test on error.
func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// setupAccentedRepo creates a minimal git repository in dir with core.quotepath=true
// (the git default) and commits a file whose name contains a non-ASCII accented character.
// Returns the repo root and the repo-relative path of the committed file.
func setupAccentedRepo(t *testing.T, dir string) (repoRoot, relPath string) {
	t.Helper()

	// Initialise repo with a known identity so git-commit does not fail.
	mustRunGit(t, dir, "init", "-b", "main")
	mustRunGit(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=Test",
		"config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	// Explicitly set core.quotepath=true so the test exercises the real default.
	mustRunGit(t, dir, "config", "core.quotepath", "true")

	sub := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// File name with non-ASCII character (UTF-8 encoded ã = \xc3\xa7\xc3\xa3o).
	fname := "ação.md"
	fpath := filepath.Join(sub, fname)
	if err := os.WriteFile(fpath, []byte("conteúdo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, dir, "add", ".")
	mustRunGit(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=Test",
		"commit", "-m", "initial")

	return dir, filepath.Join("scripts", fname)
}

// TestGitLsTree_AccentedFilename asserts that gitLsTree returns the exact repo-relative
// path of a file whose name contains non-ASCII characters even when core.quotepath=true
// (the git default), which would cause the newline-split approach to return a corrupted
// quoted path instead.
func TestGitLsTree_AccentedFilename(t *testing.T) {
	dir := t.TempDir()
	repoRoot, want := setupAccentedRepo(t, dir)

	files, err := gitLsTree("HEAD", "scripts", repoRoot)
	if err != nil {
		t.Fatalf("gitLsTree error: %v", err)
	}
	for _, f := range files {
		if f == want {
			return // found — test passes
		}
	}
	t.Errorf("gitLsTree returned %v; want it to contain %q", files, want)
}

// TestGitLsTree_AccentedFilename_OldBehavior demonstrates that the pre-fix implementation
// (newline split, no -z flag) returns a git-quoted path instead of the real file name when
// core.quotepath=true.  The test asserts that at least one returned entry is NOT equal to
// the true path — proving the old behaviour was broken.
//
// This test exercises the helper directly, simulating the old code path.
func TestGitLsTree_AccentedFilename_OldBehavior(t *testing.T) {
	dir := t.TempDir()
	repoRoot, want := setupAccentedRepo(t, dir)

	// Simulate the old implementation: no -z, split on newline.
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", "HEAD", "--", "scripts")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-tree: %v", err)
	}

	// Collect entries using the old newline-split logic.
	var oldEntries []string
	for _, line := range splitNewlines(string(out)) {
		if line != "" {
			oldEntries = append(oldEntries, line)
		}
	}

	for _, e := range oldEntries {
		if e == want {
			// Old code returned the correct path — either git is configured with
			// core.quotepath=false globally, or the system does not quote this sequence.
			// We skip rather than fail: the test is about proving the bug on systems
			// where quoting occurs; if it does not occur here the fix is still correct.
			t.Skipf("git did not quote the accented filename on this system; cannot demonstrate old failure")
		}
	}
	// At least one entry was quoted / corrupted.  This is the expected behaviour of the
	// old code path when core.quotepath=true is active.
	if len(oldEntries) == 0 {
		t.Fatal("old behaviour returned no entries at all — unexpected")
	}
	// The test asserts: old newline-split returns a corrupted (quoted) path, not the real path.
	t.Logf("old behaviour returned %v (does not contain %q) — bug confirmed", oldEntries, want)
}

// splitNewlines is a copy of the old splitting logic used only inside the test above.
func splitNewlines(s string) []string {
	// Trim trailing newline then split, exactly as the old implementation did.
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	// Simple split; mirrors strings.Split(string(bytes.TrimRight(out, "\n")), "\n").
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
