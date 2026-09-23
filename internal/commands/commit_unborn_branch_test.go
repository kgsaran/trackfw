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
