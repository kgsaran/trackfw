package commands

// branch_default_ls_remote_test.go — unit and integration tests for the ls-remote fallback
// introduced in ADR-2026-10-30 (ML-1A, REQ-2026-10-30). Covers AC3, AC4, AC5 (REQ) and R-5.
//
// Reconciliation sentences (one per test, per the Regra Dura de Reconciliação):
//
//   TestIsValidBranchLocalName:
//     Affirms P-2 — isValidBranchLocalName accepts only [a-zA-Z0-9._/-], rejects empty,
//     leading dash, ".." anywhere, and characters outside the permit list (including unicode).
//
//   TestParseLsRemoteSymref:
//     Affirms P-1/P-2 — parseLsRemoteSymref accepts only "ref: refs/heads/<name>\tHEAD" with a
//     name that passes isValidBranchLocalName; rejects refs/tags, wrong second field, missing tab,
//     and strips \r from CRLF lines.
//
//   TestResolveDefaultBranch_LsRemoteReturnsMaster:
//     Affirms P-1 — when symbolic-ref fails and ls-remote returns a valid master symref,
//     resolveDefaultBranchWithSource returns ("master","origin/master",false).
//
//   TestResolveDefaultBranch_LsRemoteReturnsError_FallsBackToMain:
//     Affirms P-1 — when symbolic-ref fails and ls-remote also returns an error, the chain
//     falls back to the literal ("main","origin/main",false).
//
//   TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled:
//     Affirms P-1(b/c) — when symbolic-ref succeeds but lacks the "origin/" prefix, the chain
//     falls to the literal WITHOUT calling ls-remote (only err!=nil triggers ls-remote).
//
//   TestResolveDefaultBranch_LsRemote_FromForge_IsFalse:
//     Affirms P-1 — the ls-remote source returns fromForge=false (not counted as forge signal).
//
//   TestBranchPrune_MasterRepo_NoSymref_LsRemoteResolvesMaster:
//     Affirms AC3 — on a real git master repo with followRemoteHEAD=never (no local symref),
//     runBranchPrune resolves origin/master via ls-remote and does not refuse with "not found".
//
//   TestDefaultBaseBranch_MasterRepoNoSymref_LsRemoteResolves_Unit:
//     Affirms AC5/P-3 — defaultBaseBranch resolves "master" via ls-remote when symbolic-ref
//     returns an error (fake-driven unit test).
//
//   TestDefaultBaseBranch_MasterRepoNoSymref_RealGit:
//     Affirms AC5/P-3 — on a real git master repo with followRemoteHEAD=never, defaultBaseBranch
//     returns "master" (ls-remote path), not "main" (literal fallback).
//
//   TestGitCommitsSince_FallsBackToOriginWhenLocalRefAbsent:
//     Affirms R-5 — when refs/heads/<base> is absent but origin/<base> exists, gitCommitsSince
//     returns commits from origin/<base>..HEAD instead of returning nil.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── AC4: unit tests for isValidBranchLocalName ──────────────────────────────────────────────

func TestIsValidBranchLocalName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		wantOK   bool
	}{
		// All reject vectors from P-2.
		{"empty", "", false},
		{"leading dash", "-bad", false},
		{"double dot", "a..b", false},
		{"tilde (revision)", "main~1", false},
		{"caret (revision)", "main^", false},
		{"at-brace (upstream)", "main@{0}", false},
		{"colon (subtree)", "a:b", false},
		{"asterisk (glob)", "a*b", false},
		// R-2: unicode falls to literal — permit list rejects it.
		{"unicode char", "branchä", false},
		{"percent", "branch%2F", false},
		// All accept vectors from P-2.
		{"simple main", "main", true},
		{"simple master", "master", true},
		{"versioned release", "release/7.2", true},
		{"hyphen feature", "feat/my-feature", true},
		// Extra accept cases for robustness.
		{"single char", "x", true},
		{"numbers", "v2", true},
		{"underscore", "fix_foo", true},
		{"dot in name", "release.2026", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isValidBranchLocalName(tc.input)
			if got != tc.wantOK {
				t.Errorf("isValidBranchLocalName(%q) = %v, want %v", tc.input, got, tc.wantOK)
			}
		})
	}
}

// ─── AC4: unit tests for parseLsRemoteSymref ─────────────────────────────────────────────────

func TestParseLsRemoteSymref(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantName   string
		wantOK     bool
	}{
		// Accept vectors from P-2.
		{
			name:     "simple main",
			input:    "ref: refs/heads/main\tHEAD\nabc123\tHEAD",
			wantName: "main",
			wantOK:   true,
		},
		{
			name:     "master",
			input:    "ref: refs/heads/master\tHEAD\ndef456\tHEAD",
			wantName: "master",
			wantOK:   true,
		},
		{
			name:     "CRLF line ending stripped",
			input:    "ref: refs/heads/main\tHEAD\r\nabc123\tHEAD",
			wantName: "main",
			wantOK:   true,
		},
		{
			name:     "release branch with slash",
			input:    "ref: refs/heads/release/7.2\tHEAD\nabc\tHEAD",
			wantName: "release/7.2",
			wantOK:   true,
		},
		// Reject vectors from P-2.
		{
			name:   "refs/tags not refs/heads",
			input:  "ref: refs/tags/v1\tHEAD\nabc\tHEAD",
			wantOK: false,
		},
		{
			name:   "second field is not HEAD (HEADX)",
			input:  "ref: refs/heads/main\tHEADX",
			wantOK: false,
		},
		{
			name:   "no tab separator (missing second field)",
			input:  "ref: refs/heads/main",
			wantOK: false,
		},
		{
			name:   "empty output",
			input:  "",
			wantOK: false,
		},
		{
			name:   "only SHA line",
			input:  "abc123def456\tHEAD",
			wantOK: false,
		},
		{
			name:   "invalid branch name (leading dash) rejected by isValidBranchLocalName",
			input:  "ref: refs/heads/-bad\tHEAD",
			wantOK: false,
		},
		{
			name:   "invalid branch name (tilde) rejected by isValidBranchLocalName",
			input:  "ref: refs/heads/main~1\tHEAD",
			wantOK: false,
		},
		{
			name:   "second field is refs/heads/HEAD not HEAD",
			input:  "ref: refs/heads/HEAD\tHEAD",
			// "HEAD" as a branch name passes isValidBranchLocalName — but the HEAD ref
			// itself is not a regular branch. The second field IS "HEAD" so this parses.
			// Documenting: if the remote reports "ref: refs/heads/HEAD\tHEAD", parseLsRemoteSymref
			// would return ("HEAD", true). This is an edge case — git servers do not normally
			// name a branch "HEAD". R-2 note: this path is technically accepted by the parser;
			// it would be used as the base branch name.
			wantName: "HEAD",
			wantOK:   true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotName, gotOK := parseLsRemoteSymref(tc.input)
			if gotOK != tc.wantOK {
				t.Errorf("parseLsRemoteSymref: wantOK=%v gotOK=%v (name=%q) for input %q",
					tc.wantOK, gotOK, gotName, tc.input)
			}
			if gotOK && gotName != tc.wantName {
				t.Errorf("parseLsRemoteSymref: wantName=%q gotName=%q for input %q",
					tc.wantName, gotName, tc.input)
			}
		})
	}
}

// ─── AC4: resolveDefaultBranchWithSource + ls-remote ─────────────────────────────────────────

// TestResolveDefaultBranch_LsRemoteReturnsMaster asserts that when symbolic-ref fails (err!=nil)
// and ls-remote returns a valid master symref, the chain resolves to ("master","origin/master",false).
func TestResolveDefaultBranch_LsRemoteReturnsMaster(t *testing.T) {
	t.Parallel()

	lsRemoteCalled := false
	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "symbolic-ref --short refs/remotes/origin/HEAD":
			return "", fmt.Errorf("ref refs/remotes/origin/HEAD is not a symbolic ref")
		case joined == "ls-remote --symref origin HEAD":
			lsRemoteCalled = true
			return "ref: refs/heads/master\tHEAD\nf12f83133990aea3cf2b\tHEAD", nil
		default:
			t.Fatalf("unexpected gitExec call: %q", joined)
			return "", nil
		}
	}

	localName, remoteRef, fromForge := resolveDefaultBranchWithSource(gitExec, nil)
	if !lsRemoteCalled {
		t.Error("ls-remote was not called when symbolic-ref failed")
	}
	if localName != "master" || remoteRef != "origin/master" {
		t.Errorf("want (master, origin/master), got (%q, %q)", localName, remoteRef)
	}
	if fromForge {
		t.Error("ls-remote source must return fromForge=false")
	}
}

// TestResolveDefaultBranch_LsRemoteReturnsError_FallsBackToMain asserts that when symbolic-ref
// fails and ls-remote also returns an error, the chain falls back to the literal "main".
func TestResolveDefaultBranch_LsRemoteReturnsError_FallsBackToMain(t *testing.T) {
	t.Parallel()

	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "symbolic-ref --short refs/remotes/origin/HEAD":
			return "", fmt.Errorf("not a symbolic ref")
		case joined == "ls-remote --symref origin HEAD":
			return "", fmt.Errorf("ls-remote: connection refused")
		default:
			t.Fatalf("unexpected gitExec call: %q", joined)
			return "", nil
		}
	}

	localName, remoteRef, fromForge := resolveDefaultBranchWithSource(gitExec, nil)
	if localName != branchPruneDefaultLocalName || remoteRef != branchPruneDefaultRemoteRef {
		t.Errorf("want fallback (%q,%q), got (%q,%q)",
			branchPruneDefaultLocalName, branchPruneDefaultRemoteRef, localName, remoteRef)
	}
	if fromForge {
		t.Error("literal fallback must return fromForge=false")
	}
}

// TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled asserts that when symbolic-ref
// returns output but it lacks the "origin/" prefix (path b/c from P-1), the chain falls
// to the literal WITHOUT calling ls-remote. This proves ls-remote is only called on err!=nil.
func TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled(t *testing.T) {
	t.Parallel()

	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "symbolic-ref --short refs/remotes/origin/HEAD":
			// Returns output but without the expected "origin/" prefix — malformed data.
			return "refs/heads/main", nil
		case joined == "ls-remote --symref origin HEAD":
			// If this is reached, the test fails: ls-remote must NOT be called on path b/c.
			t.Errorf("ls-remote must not be called when symbolic-ref returned output (path b/c)")
			return "", nil
		default:
			t.Fatalf("unexpected gitExec call: %q", joined)
			return "", nil
		}
	}

	localName, remoteRef, _ := resolveDefaultBranchWithSource(gitExec, nil)
	if localName != branchPruneDefaultLocalName || remoteRef != branchPruneDefaultRemoteRef {
		t.Errorf("want literal fallback (%q,%q), got (%q,%q)",
			branchPruneDefaultLocalName, branchPruneDefaultRemoteRef, localName, remoteRef)
	}
}

// TestResolveDefaultBranch_LsRemote_FromForge_IsFalse asserts that the ls-remote source
// returns fromForge=false (it is not counted as a forge signal for D4 purposes).
func TestResolveDefaultBranch_LsRemote_FromForge_IsFalse(t *testing.T) {
	t.Parallel()

	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "symbolic-ref --short refs/remotes/origin/HEAD":
			return "", fmt.Errorf("absent")
		case joined == "ls-remote --symref origin HEAD":
			return "ref: refs/heads/release/7.2\tHEAD\nabc\tHEAD", nil
		default:
			t.Fatalf("unexpected gitExec call: %q", joined)
			return "", nil
		}
	}

	_, _, fromForge := resolveDefaultBranchWithSource(gitExec, nil)
	if fromForge {
		t.Error("ls-remote source must not set fromForge=true (ADR-2026-10-30 §3)")
	}
}

// ─── AC5: defaultBaseBranch unit test ────────────────────────────────────────────────────────

// TestDefaultBaseBranch_MasterRepoNoSymref_LsRemoteResolves_Unit asserts that defaultBaseBranch
// returns "master" when symbolic-ref fails and ls-remote returns a master symref (fake-driven).
func TestDefaultBaseBranch_MasterRepoNoSymref_LsRemoteResolves_Unit(t *testing.T) {
	t.Parallel()

	execGit := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "symbolic-ref refs/remotes/origin/HEAD":
			return "", fmt.Errorf("not a symbolic ref")
		case joined == "ls-remote --symref origin HEAD":
			return "ref: refs/heads/master\tHEAD\nabc123\tHEAD", nil
		default:
			t.Fatalf("unexpected execGit call: %q", joined)
			return "", nil
		}
	}

	got := defaultBaseBranch(execGit)
	if got != "master" {
		t.Errorf("defaultBaseBranch: want %q, got %q (AC5/P-3: ls-remote must resolve master when symref is absent)", "master", got)
	}
}

// ─── AC3: branch prune E2E with master repo, no symref ───────────────────────────────────────

// TestBranchPrune_MasterRepo_NoSymref_LsRemoteResolvesMaster asserts: on a real git repository
// whose default branch is "master" and where followRemoteHEAD=never prevents the symref from
// being created, runBranchPrune resolves origin/master via ls-remote and does not refuse.
//
// Gate (AC3 false-positive guard from Wave 0): the fixture must have `symbolic-ref` fail and
// `ls-remote --symref origin HEAD` return "ref: refs/heads/master\tHEAD".
func TestBranchPrune_MasterRepo_NoSymref_LsRemoteResolvesMaster(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	work := t.TempDir()
	bareDir := filepath.Join(work, "origin.git")
	cloneDir := filepath.Join(work, "clone")

	cfgPath := filepath.Join(work, "empty-gitconfig")
	if err := os.WriteFile(cfgPath, []byte("[gc]\n\tauto = 0\n[maintenance]\n\tauto = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	baseEnv := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+cfgPath,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@t",
		"HOME="+work,
	)

	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = baseEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v (dir=%s) failed: %v\n%s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "master")

	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, cloneDir)
	run(cloneDir, "config", "commit.gpgsign", "false")

	// Initial commit on master.
	if err := os.WriteFile(filepath.Join(cloneDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "base.txt")
	run(cloneDir, "commit", "-q", "-m", "base commit")
	run(cloneDir, "push", "-q", "origin", "master")

	// AC3: set followRemoteHEAD=never so that the fetch inside runBranchPrune does not
	// recreate refs/remotes/origin/HEAD. This reproduces the git < 2.48 behaviour.
	run(cloneDir, "config", "remote.origin.followRemoteHEAD", "never")
	// Delete origin/HEAD so the symref starts absent.
	run(cloneDir, "remote", "set-head", "origin", "--delete")

	// feat/integrada: no extra commits → no_own_work → delete.
	run(cloneDir, "branch", "feat/integrada")

	// Return to master so it becomes the current branch.
	run(cloneDir, "checkout", "-q", "master")

	gitExec := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = cloneDir
		cmd.Env = baseEnv
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				if exitErr, ok := err.(*exec.ExitError); ok {
					msg = fmt.Sprintf("git %s exited with %d", strings.Join(args, " "), exitErr.ExitCode())
				} else {
					msg = err.Error()
				}
			}
			return strings.TrimSpace(string(out)), fmt.Errorf("%s", msg)
		}
		return strings.TrimSpace(string(out)), nil
	}

	// AC3 false-positive guard: symref must be absent, ls-remote must return master.
	if _, err := gitExec("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		t.Fatal("fixture: symbolic-ref must fail (symref absent); ls-remote path depends on this")
	}
	lsOut, lsErr := gitExec("ls-remote", "--symref", "origin", "HEAD")
	if lsErr != nil {
		t.Fatalf("fixture: ls-remote must succeed, got error: %v", lsErr)
	}
	if !strings.Contains(lsOut, "ref: refs/heads/master") {
		t.Fatalf("fixture: ls-remote must return master symref, got: %q", lsOut)
	}

	var outBuf strings.Builder
	deps := branchPruneDeps{
		gitExec:           gitExec,
		ghExec:            nil,
		listLocalBranches: defaultListLocalBranches,
		currentBranch:     defaultCurrentBranchForPrune,
		worktreeBranches:  defaultWorktreeBranches,
		deleteBranch: func(_ func(args ...string) (string, error), name string) error {
			t.Errorf("dry-run: deleteBranch must not be called, was called with %q", name)
			return nil
		},
		out: &writerFromBuilder{&outBuf},
	}

	if err := runBranchPrune(false, deps); err != nil {
		t.Fatalf("runBranchPrune returned error: %v\nOutput:\n%s", err, outBuf.String())
	}
	got := outBuf.String()

	// AC3 assertions.
	if strings.Contains(got, "Refusing") {
		t.Errorf("AC3: command refused — origin/master was not resolved via ls-remote:\n%s", got)
	}
	if !strings.Contains(got, "origin/master") {
		t.Errorf("AC3: output must mention origin/master as resolved default branch:\n%s", got)
	}
}

// ─── AC5: defaultBaseBranch with real git ────────────────────────────────────────────────────

// TestDefaultBaseBranch_MasterRepoNoSymref_RealGit asserts that defaultBaseBranch returns
// "master" on a real git master repo where followRemoteHEAD=never prevents symref creation.
func TestDefaultBaseBranch_MasterRepoNoSymref_RealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	work := t.TempDir()
	bareDir := filepath.Join(work, "origin.git")
	cloneDir := filepath.Join(work, "clone")

	cfgPath := filepath.Join(work, "empty-gitconfig")
	if err := os.WriteFile(cfgPath, []byte("[gc]\n\tauto = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	baseEnv := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+cfgPath,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@t",
		"HOME="+work,
	)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = baseEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}

	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "master")

	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, cloneDir)
	run(cloneDir, "config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(cloneDir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "f.txt")
	run(cloneDir, "commit", "-q", "-m", "init")
	run(cloneDir, "push", "-q", "origin", "master")

	// No symref: followRemoteHEAD=never + delete origin/HEAD.
	run(cloneDir, "config", "remote.origin.followRemoteHEAD", "never")
	run(cloneDir, "remote", "set-head", "origin", "--delete")

	execGit := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = cloneDir
		cmd.Env = baseEnv
		var se strings.Builder
		cmd.Stderr = &se
		out, err := cmd.Output()
		if err != nil {
			msg := se.String()
			if msg == "" {
				msg = err.Error()
			}
			return "", fmt.Errorf("%s", strings.TrimSpace(msg))
		}
		return strings.TrimSpace(string(out)), nil
	}

	got := defaultBaseBranch(execGit)
	if got != "master" {
		t.Errorf("AC5/P-3: defaultBaseBranch on master repo with no symref = %q, want %q (should resolve via ls-remote)", got, "master")
	}
}

// ─── R-5: gitCommitsSince fallback to origin/<base> ──────────────────────────────────────────

// TestGitCommitsSince_FallsBackToOriginWhenLocalRefAbsent asserts: when refs/heads/<base> does
// not exist locally but origin/<base> does, gitCommitsSince returns the commits from
// origin/<base>..HEAD instead of returning nil (exit 128).
func TestGitCommitsSince_FallsBackToOriginWhenLocalRefAbsent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	work := t.TempDir()
	bareDir := filepath.Join(work, "origin.git")
	cloneDir := filepath.Join(work, "clone")

	cfgPath := filepath.Join(work, "empty-gitconfig")
	if err := os.WriteFile(cfgPath, []byte("[gc]\n\tauto = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	baseEnv := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+cfgPath,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@t",
		"HOME="+work,
	)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = baseEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}

	// Init bare remote with master.
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "master")

	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, cloneDir)
	run(cloneDir, "config", "commit.gpgsign", "false")

	// Initial commit on master, push.
	if err := os.WriteFile(filepath.Join(cloneDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "base.txt")
	run(cloneDir, "commit", "-q", "-m", "base commit")
	run(cloneDir, "push", "-q", "origin", "master")

	// Create and checkout feat/x with one commit.
	run(cloneDir, "checkout", "-q", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(cloneDir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "feature.txt")
	run(cloneDir, "commit", "-q", "-m", "feat: add feature")

	// R-5 setup: delete the local master branch so refs/heads/master is absent.
	// origin/master (the remote tracking ref) remains and resolves correctly.
	run(cloneDir, "branch", "-D", "master")

	// Verify the setup: refs/heads/master must be absent, origin/master must exist.
	verifyCmd := exec.Command("git", "rev-parse", "-q", "--verify", "refs/heads/master")
	verifyCmd.Dir = cloneDir
	verifyCmd.Env = baseEnv
	if err := verifyCmd.Run(); err == nil {
		t.Fatal("setup: refs/heads/master must be absent for R-5 test")
	}
	originCmd := exec.Command("git", "rev-parse", "-q", "--verify", "origin/master")
	originCmd.Dir = cloneDir
	originCmd.Env = baseEnv
	if err := originCmd.Run(); err != nil {
		t.Fatal("setup: origin/master must exist for R-5 test")
	}

	execGit := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = cloneDir
		cmd.Env = baseEnv
		var se strings.Builder
		cmd.Stderr = &se
		out, err := cmd.Output()
		if err != nil {
			msg := se.String()
			if msg == "" {
				msg = err.Error()
			}
			return "", fmt.Errorf("%s", strings.TrimSpace(msg))
		}
		return strings.TrimSpace(string(out)), nil
	}

	commits := gitCommitsSince("master", execGit)
	if len(commits) == 0 {
		t.Fatalf("R-5: gitCommitsSince(\"master\") returned nil/empty; expected 1 commit from feat/x. "+
			"Without the R-5 fix, git log master..HEAD fails (exit 128) because refs/heads/master is absent.")
	}
	if !strings.Contains(commits[0], "feat: add feature") {
		t.Errorf("R-5: expected commit message to contain %q, got %q", "feat: add feature", commits[0])
	}
}
