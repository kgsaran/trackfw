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

	return dir, "scripts/" + fname
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

