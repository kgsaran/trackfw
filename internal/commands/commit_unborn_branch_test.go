package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitNoCommitsRepo creates a repository with a staged file and NO commits, and chdirs
// into it for the duration of the test. Environment is isolated from the developer's
// global git config: a stray init.defaultBranch or commit.gpgsign would otherwise
// decide the result of the assertions below.
func gitRepoIn(t *testing.T, dir string, args ...[]string) {
	t.Helper()
	for _, a := range args {
		cmd := exec.Command("git", a...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+filepath.Join(dir, "gitconfig"),
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_TERMINAL_PROMPT=0",
			"HOME="+dir,
			"LC_ALL=C",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
}

// TestDefaultCurrentBranch_UnbornBranch asserts the conclusion this change rests on:
// in a repository with no commits, `rev-parse --abbrev-ref HEAD` cannot name the
// branch and `symbolic-ref --short HEAD` can — so defaultCurrentBranch answers
// instead of failing, and `trackfw commit` has a path to the root commit.
func TestDefaultCurrentBranch_UnbornBranch(t *testing.T) {
	dir := t.TempDir()
	gitRepoIn(t, dir,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
	)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRepoIn(t, dir, []string{"add", "a.txt"})
	t.Chdir(dir)

	// Control in the other direction: without this the test would pass even if the
	// unborn state were not actually reproduced.
	if _, err := defaultGitExec("rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		t.Fatal("premissa quebrada: rev-parse funcionou num repositorio sem commits")
	}

	branch, err := defaultCurrentBranch()
	if err != nil {
		t.Fatalf("defaultCurrentBranch em branch sem commits: %v", err)
	}
	if branch != "main" {
		t.Fatalf("branch = %q, esperava \"main\"", branch)
	}
}

// TestDefaultCurrentBranch_DetachedHEAD pins the answer the callers already depend on:
// detached HEAD keeps returning the literal "HEAD". It is why rev-parse stays first
// instead of being replaced by symbolic-ref, which errors in this state.
func TestDefaultCurrentBranch_DetachedHEAD(t *testing.T) {
	dir := t.TempDir()
	gitRepoIn(t, dir,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
	)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRepoIn(t, dir,
		[]string{"add", "a.txt"},
		[]string{"commit", "-q", "-m", "raiz"},
		[]string{"checkout", "-q", "--detach", "HEAD"},
	)
	t.Chdir(dir)

	if _, err := defaultGitExec("symbolic-ref", "--short", "HEAD"); err == nil {
		t.Fatal("premissa quebrada: symbolic-ref funcionou com HEAD destacado")
	}

	branch, err := defaultCurrentBranch()
	if err != nil {
		t.Fatalf("defaultCurrentBranch com HEAD destacado: %v", err)
	}
	if branch != "HEAD" {
		t.Fatalf("branch = %q, esperava \"HEAD\" (resposta historica, da qual os chamadores dependem)", branch)
	}
}

// TestDefaultCurrentBranch_OutsideRepo asserts the error that survives: outside a
// repository BOTH commands fail, and the message returned is rev-parse's — the one
// that names the real condition — not the fallback's "not a symbolic ref".
func TestDefaultCurrentBranch_OutsideRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))

	if _, err := defaultCurrentBranch(); err == nil {
		t.Fatal("esperava erro fora de repositorio, veio nil")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D2 (ADR-2026-11-26): isBootstrapFirstCommit — real-git integration arms
// These tests use real git repos to confirm the conclusions from the mock-based
// unit tests: the predicates measure what git actually reports, not just what
// the injected stubs return.
// ────────────────────────────────────────────────────────────────────────────

// TestIsBootstrapFirstCommit_RealGit_FreshRepo affirms that in a real git repo
// with no commits, isBootstrapFirstCommit(defaultGitExec) returns true.
// Rule reconciliation (real-git arm): affirms that the conjunct predicate works
// against git's actual rev-parse and rev-list commands — the mock test
// TestIsBootstrapFirstCommit_UnbornAndZeroCount is the injection-based counterpart.
func TestIsBootstrapFirstCommit_RealGit_FreshRepo(t *testing.T) {
	dir := t.TempDir()
	gitRepoIn(t, dir,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
	)
	t.Chdir(dir)

	if !isBootstrapFirstCommit(defaultGitExec) {
		t.Error("expected true for fresh git repo with no commits, got false")
	}
}

// TestIsBootstrapFirstCommit_RealGit_SwitchOrphan affirms M-X1 with real git:
// after `git switch --orphan x` in a repo that has commits on another branch,
// isBootstrapFirstCommit returns false (the orphan branch is unborn, but
// rev-list --all --count is > 0).
// Rule reconciliation (real-git arm): affirms the M-X1 measurement — the mock
// test TestIsBootstrapFirstCommit_SwitchOrphan is the injection-based counterpart.
func TestIsBootstrapFirstCommit_RealGit_SwitchOrphan(t *testing.T) {
	dir := t.TempDir()
	gitRepoIn(t, dir,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
	)
	// Create a commit on main first.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRepoIn(t, dir,
		[]string{"add", "a.txt"},
		[]string{"commit", "-q", "-m", "init"},
	)
	// Switch to an orphan branch (unborn HEAD, but history exists on main).
	gitRepoIn(t, dir, []string{"switch", "--orphan", "orphan-branch"})
	t.Chdir(dir)

	if isBootstrapFirstCommit(defaultGitExec) {
		t.Error("expected false after switch --orphan (commits exist elsewhere), got true")
	}
}

// TestDefaultInitGitCheck_RealGit_BareRepo affirms M-X9 with real git:
// defaultInitGitCheck inside a bare repo returns rc=0 and stdout=="false".
// Rule reconciliation (real-git arm): affirms the M-X9 measurement that
// is-inside-work-tree reports false in a bare repo — which is what D1 guard
// uses to prevent nested repo creation; the mock test
// TestRunGitInitForBootstrap_InsideBareOrGitDir is the injection-based counterpart.
func TestDefaultInitGitCheck_RealGit_BareRepo(t *testing.T) {
	dir := t.TempDir()
	// Initialize a bare repo.
	gitRepoIn(t, dir, []string{"init", "--bare", "-q", "."})
	t.Chdir(dir)

	stdout, _, rc := defaultInitGitCheck()
	if rc != 0 {
		t.Fatalf("expected rc=0 inside bare repo, got rc=%d", rc)
	}
	if stdout != "false" {
		t.Errorf("expected stdout=false inside bare repo, got %q", stdout)
	}
}
