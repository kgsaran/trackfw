package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// setupGitRepoForGitignoreTest creates a minimal git repo in dir, writes a
// trackfw.yaml with defaults, and chdirs into it. It returns dir.
func setupGitRepoForGitignoreTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Write a minimal trackfw.yaml so config.Load() picks up defaults.
	writeFile(t, dir, "trackfw.yaml", "")

	chdir(t, dir)
	return dir
}

// addGitignorePattern appends a pattern to .gitignore in dir (creating it if absent).
func addGitignorePattern(t *testing.T, dir, pattern string) {
	t.Helper()
	path := filepath.Join(dir, ".gitignore")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("addGitignorePattern: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(pattern + "\n"); err != nil {
		t.Fatalf("addGitignorePattern write: %v", err)
	}
}

// gitAvailable returns false when git is not in PATH — used to skip tests that
// need a real git repo.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// TestValidateLogEntriesDirGitignored_EntryGlobWarns affirms R1: a *.entry pattern
// in .gitignore causes validateLogEntriesDirGitignored to emit a warning naming
// the roadmap log-entries directory.
func TestValidateLogEntriesDirGitignored_EntryGlobWarns(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not in PATH")
	}
	dir := setupGitRepoForGitignoreTest(t)
	config.Reset()
	t.Cleanup(config.Reset)

	// *.entry matches docs/roadmaps/.trackfw-log-entries/x.entry.
	addGitignorePattern(t, dir, "*.entry")

	msgs, err := validateLogEntriesDirGitignored()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ".trackfw-log-entries/"
	found := false
	for _, m := range msgs {
		if strings.Contains(m, want) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning containing %q, got %v", want, msgs)
	}
}

// TestValidateLogEntriesDirGitignored_DotStarWarns affirms R1: a .* pattern (which
// covers hidden directories like .trackfw-log-entries) triggers the warning.
func TestValidateLogEntriesDirGitignored_DotStarWarns(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not in PATH")
	}
	dir := setupGitRepoForGitignoreTest(t)
	config.Reset()
	t.Cleanup(config.Reset)

	// .* matches .trackfw-log-entries/ because the directory name starts with a dot.
	addGitignorePattern(t, dir, ".*")

	msgs, err := validateLogEntriesDirGitignored()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ".trackfw-log-entries/"
	found := false
	for _, m := range msgs {
		if strings.Contains(m, want) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning containing %q for .* pattern, got %v", want, msgs)
	}
}

// TestValidateLogEntriesDirGitignored_EntriesGlobWarns affirms R1: a *-entries pattern
// covers .trackfw-log-entries and triggers the warning.
func TestValidateLogEntriesDirGitignored_EntriesGlobWarns(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not in PATH")
	}
	dir := setupGitRepoForGitignoreTest(t)
	config.Reset()
	t.Cleanup(config.Reset)

	// *-entries matches the directory name .trackfw-log-entries.
	addGitignorePattern(t, dir, "*-entries")

	msgs, err := validateLogEntriesDirGitignored()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := ".trackfw-log-entries/"
	found := false
	for _, m := range msgs {
		if strings.Contains(m, want) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning containing %q for *-entries pattern, got %v", want, msgs)
	}
}

// TestValidateLogEntriesDirGitignored_NoPatternsNoWarn affirms R1 negative arm: when
// no .gitignore patterns cover the log-entries paths, no warning is produced.
func TestValidateLogEntriesDirGitignored_NoPatternsNoWarn(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not in PATH")
	}
	dir := setupGitRepoForGitignoreTest(t)
	config.Reset()
	t.Cleanup(config.Reset)

	// A .gitignore that covers completely unrelated paths must not warn.
	addGitignorePattern(t, dir, "*.log")
	addGitignorePattern(t, dir, "node_modules/")
	_ = dir

	msgs, err := validateLogEntriesDirGitignored()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, "log_entries_dir_gitignored") {
			t.Errorf("expected no warning, got: %q", m)
		}
	}
}

// TestValidateLogEntriesDirGitignored_NoGitNoWarn affirms R1 safety arm: when the
// CWD is not inside a git work-tree, no warning is produced (the rule is a no-op).
func TestValidateLogEntriesDirGitignored_NoGitNoWarn(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)
	config.Reset()

	// No git init → not a work-tree → rule must be silent.
	msgs, err := validateLogEntriesDirGitignored()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected no warnings outside a git repo, got %v", msgs)
	}
}
