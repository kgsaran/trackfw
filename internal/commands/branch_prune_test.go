package commands

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// splitNulPaths
// ────────────────────────────────────────────────────────────────────────────

func TestSplitNulPaths(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"single", "foo.md\x00", []string{"foo.md"}},
		{"multi_sorted", "z.md\x00a.md\x00", []string{"a.md", "z.md"}},
		{"space_in_name", "foo bar.md\x00", []string{"foo bar.md"}},
		{"no_trailing_nul", "a.md\x00b.md", []string{"a.md", "b.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitNulPaths(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// evaluateBranchIntegration — unit tests with a fake gitExec (no real git repo)
// ────────────────────────────────────────────────────────────────────────────

// fakeGitExec returns a gitExec closure driven by a map keyed on the joined args.
func fakeGitExec(t *testing.T, responses map[string]struct {
	out string
	err error
}) func(...string) (string, error) {
	return func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		r, ok := responses[key]
		if !ok {
			t.Fatalf("fakeGitExec: unexpected call: git %s", key)
		}
		return r.out, r.err
	}
}

func TestEvaluateBranchIntegration_NoOwnWork_Deletable(t *testing.T) {
	gitExec := fakeGitExec(t, map[string]struct {
		out string
		err error
	}{
		"merge-base origin/main feat/foo":     {"abc123", nil},
		"diff --name-only -z abc123 feat/foo": {"", nil}, // touched empty
	})
	eval := evaluateBranchIntegration("feat/foo", gitExec)
	if eval.Decision != branchPruneDecisionNoOwnWork {
		t.Fatalf("expected no_own_work, got %v (%s)", eval.Decision, eval.Reason)
	}
	if !eval.Decision.deletable() {
		t.Fatal("expected no_own_work to be deletable")
	}
}

func TestEvaluateBranchIntegration_ContentIdentical_Deletable(t *testing.T) {
	// The AC2 discriminant, at the unit level: touched is non-empty (branch DID touch files) but
	// diverg comes back empty (main has since converged on the same content in those files —
	// stale-but-integrated).
	gitExec := fakeGitExec(t, map[string]struct {
		out string
		err error
	}{
		"merge-base origin/main feat/stale":                   {"abc123", nil},
		"diff --name-only -z abc123 feat/stale":               {"f1.md\x00", nil},
		"diff --name-only -z origin/main feat/stale -- f1.md": {"", nil},
	})
	eval := evaluateBranchIntegration("feat/stale", gitExec)
	if eval.Decision != branchPruneDecisionIdentical {
		t.Fatalf("expected content_identical, got %v (%s)", eval.Decision, eval.Reason)
	}
	if !eval.Decision.deletable() {
		t.Fatal("expected content_identical to be deletable")
	}
}

func TestEvaluateBranchIntegration_PendingWork_NotDeletable(t *testing.T) {
	gitExec := fakeGitExec(t, map[string]struct {
		out string
		err error
	}{
		"merge-base origin/main feat/pending":                   {"abc123", nil},
		"diff --name-only -z abc123 feat/pending":               {"f1.md\x00", nil},
		"diff --name-only -z origin/main feat/pending -- f1.md": {"f1.md\x00", nil},
	})
	eval := evaluateBranchIntegration("feat/pending", gitExec)
	if eval.Decision != branchPruneDecisionPendingWork {
		t.Fatalf("expected pending_work, got %v (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("expected pending_work to never be deletable")
	}
	if !strings.Contains(eval.Reason, "f1.md") {
		t.Fatalf("expected reason to name the diverging file, got %q", eval.Reason)
	}
}

func TestEvaluateBranchIntegration_NoMergeBase_Refuses(t *testing.T) {
	gitExec := fakeGitExec(t, map[string]struct {
		out string
		err error
	}{
		"merge-base origin/main feat/orphan": {"", fmt.Errorf("fatal: no merge base")},
	})
	eval := evaluateBranchIntegration("feat/orphan", gitExec)
	if eval.Decision != branchPruneDecisionNoMergeBase {
		t.Fatalf("expected no_merge_base, got %v", eval.Decision)
	}
	if eval.Decision.deletable() {
		t.Fatal("no_merge_base must never be deletable")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchPrune — orchestration with fully injected deps (no real git repo)
// ────────────────────────────────────────────────────────────────────────────

func makePruneDeps(out *bytes.Buffer) branchPruneDeps {
	return branchPruneDeps{
		gitExec: func(args ...string) (string, error) {
			if len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify" {
				return "abc123", nil // origin/main resolvable
			}
			return "", fmt.Errorf("unexpected gitExec call in this test: %v", args)
		},
		listLocalBranches: func(func(args ...string) (string, error)) ([]string, error) {
			return []string{"main", "feat/integrated", "feat/pending", "fix/current", "chore/wt"}, nil
		},
		currentBranch: func(func(args ...string) (string, error)) string {
			return "fix/current"
		},
		worktreeBranches: func(func(args ...string) (string, error)) map[string]bool {
			return map[string]bool{"chore/wt": true}
		},
		deleteBranch: func(func(args ...string) (string, error), string) error {
			t := new(testing.T)
			t.Fatal("deleteBranch must not be called in dry-run tests")
			return nil
		},
		out: out,
	}
}

func TestRunBranchPrune_DryRun_NeverDeletes_MainNeverCandidate(t *testing.T) {
	out := &bytes.Buffer{}
	deps := makePruneDeps(out)
	// Wire a real-ish evaluator via gitExec dispatch table for the two non-excluded branches.
	deps.gitExec = func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch key {
		case "rev-parse --verify -q origin/main":
			return "abc123", nil
		case "merge-base origin/main feat/integrated":
			return "abc123", nil
		case "diff --name-only -z abc123 feat/integrated":
			return "", nil // no own work -> deletable
		case "merge-base origin/main feat/pending":
			return "abc123", nil
		case "diff --name-only -z abc123 feat/pending":
			return "f1.md\x00", nil
		case "diff --name-only -z origin/main feat/pending -- f1.md":
			return "f1.md\x00", nil // pending
		}
		return "", fmt.Errorf("unexpected gitExec call: %v", args)
	}
	deleteCalled := false
	deps.deleteBranch = func(func(args ...string) (string, error), string) error {
		deleteCalled = true
		return nil
	}

	err := runBranchPrune(false, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Fatal("dry-run (default) must never call deleteBranch")
	}
	got := out.String()
	if !strings.Contains(got, "would delete") {
		t.Fatalf("expected dry-run summary mentioning 'would delete', got: %q", got)
	}
	// main must never appear as a delete candidate, even though evaluateBranchIntegration would
	// trivially report "no own work" for it (merge-base origin/main main == main's own tip).
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "main ") && strings.Contains(line, "delete") {
			t.Fatalf("main must never be offered for deletion, got line: %q", line)
		}
	}
	if !strings.Contains(got, "default branch") {
		t.Fatalf("expected main to be reported with 'default branch' reason, got: %q", got)
	}
	if !strings.Contains(got, "current branch") {
		t.Fatalf("expected fix/current to be reported with 'current branch' reason, got: %q", got)
	}
	if !strings.Contains(got, "worktree") {
		t.Fatalf("expected chore/wt to be reported with worktree reason, got: %q", got)
	}
}

func TestRunBranchPrune_Apply_DeletesOnlyIntegrated_KeepsPending(t *testing.T) {
	out := &bytes.Buffer{}
	deps := makePruneDeps(out)
	deps.gitExec = func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch key {
		case "rev-parse --verify -q origin/main":
			return "abc123", nil
		case "merge-base origin/main feat/integrated":
			return "abc123", nil
		case "diff --name-only -z abc123 feat/integrated":
			return "", nil
		case "merge-base origin/main feat/pending":
			return "abc123", nil
		case "diff --name-only -z abc123 feat/pending":
			return "f1.md\x00", nil
		case "diff --name-only -z origin/main feat/pending -- f1.md":
			return "f1.md\x00", nil
		}
		return "", fmt.Errorf("unexpected gitExec call: %v", args)
	}
	var deletedNames []string
	deps.deleteBranch = func(gitExec func(args ...string) (string, error), name string) error {
		deletedNames = append(deletedNames, name)
		return nil
	}

	err := runBranchPrune(true, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deletedNames) != 1 || deletedNames[0] != "feat/integrated" {
		t.Fatalf("expected only feat/integrated to be deleted, got: %v", deletedNames)
	}
	got := out.String()
	if !strings.Contains(got, "deleted 1 branch(es): feat/integrated") {
		t.Fatalf("expected apply summary naming feat/integrated, got: %q", got)
	}
}

func TestRunBranchPrune_NoOriginMain_RefusesEverything(t *testing.T) {
	out := &bytes.Buffer{}
	deps := branchPruneDeps{
		gitExec: func(args ...string) (string, error) {
			return "", fmt.Errorf("fatal: needed a single revision")
		},
		listLocalBranches: func(func(args ...string) (string, error)) ([]string, error) {
			t := new(testing.T)
			t.Fatal("listLocalBranches must not be called when origin/main is unresolvable")
			return nil, nil
		},
		currentBranch:    func(func(args ...string) (string, error)) string { return "" },
		worktreeBranches: func(func(args ...string) (string, error)) map[string]bool { return nil },
		deleteBranch: func(func(args ...string) (string, error), string) error {
			t := new(testing.T)
			t.Fatal("deleteBranch must not be called when origin/main is unresolvable")
			return nil
		},
		out: out,
	}

	err := runBranchPrune(true, deps) // even with --apply
	if err == nil {
		t.Fatal("expected an error when origin/main cannot be resolved")
	}
	got := out.String()
	if !strings.Contains(got, "origin/main") {
		t.Fatalf("expected message to name origin/main, got: %q", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Real git repository integration test — the AC2 discriminant. Per REQ-2026-08-18 /
// vault precedent (Cenário 50 in scripts/check-gates-falsify.sh), a mock of `git`
// would only prove the mock agrees with the code; this exercises real git plumbing.
//
// Fixture: local bare repo as "origin" (offline, no network) + a clone.
//   1. On main: commit base.txt
//   2. Branch A off main, commit a.txt, squash-merge A into main (no ancestry)
//   3. Branch B off main, commit b.txt, squash-merge B into main (main advances further)
//   4. Push main to origin; fetch in the clone
//   5. Branch A is now BEHIND origin/main (B's squash-merge came after), but fully integrated —
//      exactly the false positive the naive `git diff origin/main A --stat` reports as pending.
// ────────────────────────────────────────────────────────────────────────────

func TestEvaluateBranchIntegration_RealGitRepo_SquashMergeAndStaleDiscriminant(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	work := t.TempDir()
	bareDir := filepath.Join(work, "origin.git")
	cloneDir := filepath.Join(work, "clone")

	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+filepath.Join(work, "empty-gitconfig"),
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_TERMINAL_PROMPT=0",
			"HOME="+work,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v (dir=%s) failed: %v\n%s", args, dir, err, out)
		}
		return string(out)
	}

	if err := os.WriteFile(filepath.Join(work, "empty-gitconfig"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bare "origin" — offline substitute for a real remote.
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "main")

	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, cloneDir)
	run(cloneDir, "config", "user.email", "falsify@trackfw.test")
	run(cloneDir, "config", "user.name", "trackfw falsify")
	run(cloneDir, "config", "commit.gpgsign", "false")
	run(cloneDir, "config", "core.hooksPath", "/dev/null")

	writeFile := func(name, content string) {
		if err := os.WriteFile(filepath.Join(cloneDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("base.txt", "base\n")
	run(cloneDir, "add", "base.txt")
	run(cloneDir, "commit", "-q", "-m", "base commit")
	run(cloneDir, "push", "-q", "origin", "main")

	// Branch A: touches a.txt, squash-merged into main first.
	run(cloneDir, "checkout", "-q", "-b", "feat/a")
	writeFile("a.txt", "a\n")
	run(cloneDir, "add", "a.txt")
	run(cloneDir, "commit", "-q", "-m", "feat/a work")
	run(cloneDir, "checkout", "-q", "main")
	run(cloneDir, "merge", "-q", "--squash", "feat/a")
	run(cloneDir, "commit", "-q", "-m", "squash-merge feat/a")

	// Branch B: touches b.txt, branched off main AFTER feat/a's squash-merge landed, then
	// squash-merged too — main advances further, leaving feat/a behind but still integrated.
	run(cloneDir, "checkout", "-q", "-b", "feat/b")
	writeFile("b.txt", "b\n")
	run(cloneDir, "add", "b.txt")
	run(cloneDir, "commit", "-q", "-m", "feat/b work")
	run(cloneDir, "checkout", "-q", "main")
	run(cloneDir, "merge", "-q", "--squash", "feat/b")
	run(cloneDir, "commit", "-q", "-m", "squash-merge feat/b")

	run(cloneDir, "push", "-q", "origin", "main")
	run(cloneDir, "fetch", "-q", "origin")

	// A genuinely pending branch: touches c.txt, never merged anywhere.
	run(cloneDir, "checkout", "-q", "-b", "feat/pending")
	writeFile("c.txt", "c\n")
	run(cloneDir, "add", "c.txt")
	run(cloneDir, "commit", "-q", "-m", "feat/pending work, never merged")

	gitExec := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = cloneDir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+filepath.Join(work, "empty-gitconfig"),
			"GIT_CONFIG_SYSTEM=/dev/null",
			"HOME="+work,
		)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	// Sanity: the naive bidirectional check IS non-empty for feat/a — proving this test is
	// actually discriminating between the naive check and the heuristic, not vacuously passing.
	naiveDiff, err := gitExec("diff", "origin/main", "feat/a", "--stat")
	if err != nil {
		t.Fatalf("naive diff failed: %v", err)
	}
	if strings.TrimSpace(naiveDiff) == "" {
		t.Fatal("test setup invalid: naive diff origin/main feat/a --stat must be non-empty to discriminate against the heuristic (AC2)")
	}

	evalA := evaluateBranchIntegration("feat/a", gitExec)
	if evalA.Decision != branchPruneDecisionIdentical {
		t.Fatalf("feat/a (stale but integrated) expected content_identical, got %v (%s)", evalA.Decision, evalA.Reason)
	}
	if !evalA.Decision.deletable() {
		t.Fatal("feat/a must be deletable — this is the AC2 discriminant")
	}

	evalPending := evaluateBranchIntegration("feat/pending", gitExec)
	if evalPending.Decision != branchPruneDecisionPendingWork {
		t.Fatalf("feat/pending expected pending_work, got %v (%s)", evalPending.Decision, evalPending.Reason)
	}
	if evalPending.Decision.deletable() {
		t.Fatal("feat/pending (genuinely unmerged) must never be deletable")
	}

	// AC1 — squash-merge without ancestry: `git branch -d` would refuse feat/a (no fast-forward
	// ancestry), which is exactly the false negative this heuristic exists to route around.
	if _, err := exec.Command("git", "-C", cloneDir, "branch", "-d", "feat/a").CombinedOutput(); err == nil {
		t.Fatal("test setup invalid: git branch -d unexpectedly succeeded on a squash-merged branch — AC1 fixture no longer demonstrates the ancestry false negative")
	}

	// Full runBranchPrune with real deps.deleteBranch — end-to-end proof --apply deletes the
	// integrated one and keeps the pending one, against a real repo.
	var deleted []string
	out := &bytes.Buffer{}
	deps := branchPruneDeps{
		gitExec:           gitExec,
		listLocalBranches: defaultListLocalBranches,
		currentBranch:     defaultCurrentBranchForPrune,
		worktreeBranches:  defaultWorktreeBranches,
		deleteBranch: func(gitExec func(args ...string) (string, error), name string) error {
			deleted = append(deleted, name)
			_, err := gitExec("branch", "-D", name)
			return err
		},
		out: out,
	}

	if err := runBranchPrune(true, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(deleted)
	// feat/a and feat/b are both local branches, both squash-merged into main and now integrated
	// (feat/a stale-but-integrated is the AC2 discriminant; feat/b is a plain up-to-date
	// squash-merge, the AC1 case). feat/pending must never appear here.
	want := []string{"feat/a", "feat/b"}
	if len(deleted) != len(want) || deleted[0] != want[0] || deleted[1] != want[1] {
		t.Fatalf("expected feat/a and feat/b to be deleted, feat/pending kept, got: %v", deleted)
	}

	remaining, err := defaultListLocalBranches(gitExec)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(remaining)
	for _, b := range remaining {
		if b == "feat/a" {
			t.Fatal("feat/a should have been deleted by --apply")
		}
	}
	found := false
	for _, b := range remaining {
		if b == "feat/pending" {
			found = true
		}
	}
	if !found {
		t.Fatal("feat/pending must still exist — it was never a delete candidate")
	}
}
