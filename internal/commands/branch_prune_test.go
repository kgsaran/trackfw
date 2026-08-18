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
	// f1.md — deliberately a doc file, to prove pending_work is decided by "diverg == touched"
	// (nothing from this branch reached main at all), not by file type. ML-1C fixed the earlier
	// bug where a doc-only branch with diverg == touched was misrouted to review_doc_config
	// ("probable housekeeping, confirm and delete manually") even though it had never been
	// integrated at all — see "Auditoria do ML-1B" in the roadmap. TestEvaluateBranchIntegration_
	// ReviewDocConfig below is the contrasting case: same file types, but diverg is a PROPER
	// subset of touched (partial integration), which is what actually makes it review_doc_config.
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

func TestEvaluateBranchIntegration_ReviewDocConfig_NotDeletable(t *testing.T) {
	// AC "Divergência só em doc/config": review_doc_config requires diverg to be a PROPER
	// subset of touched — i.e. genuine partial integration (part of the branch's own work
	// reached main, doc/config residue is left over), not a branch that was never integrated
	// at all. Here the branch touched three files (CLAUDE.md, trackfw.yaml, README-merged.md);
	// README-merged.md made it into main (absent from diverg), but CLAUDE.md and trackfw.yaml —
	// both doc/config, never real code — still diverge. That is the housekeeping-residue case
	// ML-1C's discriminant targets: never auto-deletable (KG's explicit instruction), but
	// distinct from pending_work because SOME of the branch's own work is already in main.
	gitExec := fakeGitExec(t, map[string]struct {
		out string
		err error
	}{
		"merge-base origin/main feat/docs-only":                                                     {"abc123", nil},
		"diff --name-only -z abc123 feat/docs-only":                                                 {"CLAUDE.md\x00README-merged.md\x00trackfw.yaml\x00", nil},
		"diff --name-only -z origin/main feat/docs-only -- CLAUDE.md README-merged.md trackfw.yaml": {"CLAUDE.md\x00trackfw.yaml\x00", nil},
	})
	eval := evaluateBranchIntegration("feat/docs-only", gitExec)
	if eval.Decision != branchPruneDecisionReviewDocConfig {
		t.Fatalf("expected review_doc_config, got %v (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("review_doc_config must never be deletable — KG's explicit instruction, do not auto-delete doc/config-only divergence")
	}
	if !strings.Contains(eval.Reason, "CLAUDE.md") || !strings.Contains(eval.Reason, "trackfw.yaml") {
		t.Fatalf("expected reason to name the diverging files, got %q", eval.Reason)
	}
	if !strings.Contains(eval.Reason, "confirm and delete manually") {
		t.Fatalf("expected reason to orient the user toward manual confirmation, got %q", eval.Reason)
	}
}

func TestEvaluateBranchIntegration_MixedDocAndCode_StaysPendingWork(t *testing.T) {
	// A single non-doc/config file in diverg must keep the branch in pending_work, not
	// review_doc_config — the classification is all-or-nothing, never partial credit.
	gitExec := fakeGitExec(t, map[string]struct {
		out string
		err error
	}{
		"merge-base origin/main feat/mixed":                               {"abc123", nil},
		"diff --name-only -z abc123 feat/mixed":                           {"README.md\x00main.go\x00", nil},
		"diff --name-only -z origin/main feat/mixed -- README.md main.go": {"README.md\x00main.go\x00", nil},
	})
	eval := evaluateBranchIntegration("feat/mixed", gitExec)
	if eval.Decision != branchPruneDecisionPendingWork {
		t.Fatalf("expected pending_work (mixed doc+code), got %v (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("pending_work must never be deletable")
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
		case "fetch origin --prune":
			return "", nil
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
		case "fetch origin --prune":
			return "", nil
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

func TestRunBranchPrune_FetchFails_WarnsButStillEvaluates(t *testing.T) {
	// AC "fetch origin --prune roda antes da avaliação; falha é não-bloqueante e avisada": unlike
	// `git fetch` failing entirely aborting the check (ship.go's posture), branch prune must
	// still evaluate every branch against whatever origin/main ref is already resolvable
	// locally, and must print a warning explaining the data may be stale.
	out := &bytes.Buffer{}
	deps := makePruneDeps(out)
	fetchCalled := false
	deps.gitExec = func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch key {
		case "fetch origin --prune":
			fetchCalled = true
			return "", fmt.Errorf("fatal: unable to access origin (simulated offline)")
		case "rev-parse --verify -q origin/main":
			return "abc123", nil // a previous, successful fetch already resolved this ref
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

	err := runBranchPrune(false, deps)
	if err != nil {
		t.Fatalf("fetch failure must not abort the command: %v", err)
	}
	if !fetchCalled {
		t.Fatal("expected git fetch origin --prune to have been attempted")
	}
	got := out.String()
	if !strings.Contains(strings.ToLower(got), "warning") || !strings.Contains(got, "fetch") {
		t.Fatalf("expected a warning mentioning the fetch failure, got: %q", got)
	}
	// Evaluation must still have proceeded normally — feat/integrated still reported deletable.
	if !strings.Contains(got, "would delete") || !strings.Contains(got, "feat/integrated") {
		t.Fatalf("expected evaluation to proceed despite fetch failure, got: %q", got)
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

// ────────────────────────────────────────────────────────────────────────────
// defaultDeleteBranch — -d tried before -D, real git repository (both codepaths).
// ────────────────────────────────────────────────────────────────────────────

func TestDefaultDeleteBranch_TriesDashDBeforeDashD_BothCodepaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	work := t.TempDir()
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

	repo := filepath.Join(work, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run(repo, "init", "-q", "-b", "main")
	run(repo, "config", "user.email", "falsify@trackfw.test")
	run(repo, "config", "user.name", "trackfw falsify")
	run(repo, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "base.txt")
	run(repo, "commit", "-q", "-m", "base")

	gitExec := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+filepath.Join(work, "empty-gitconfig"),
			"GIT_CONFIG_SYSTEM=/dev/null",
			"HOME="+work,
		)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	// Codepath 1: feat/ff has fast-forward ancestry with main (a plain merge, no squash) — `git
	// branch -d` must succeed on its own, without ever needing -D.
	run(repo, "checkout", "-q", "-b", "feat/ff")
	if err := os.WriteFile(filepath.Join(repo, "ff.txt"), []byte("ff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "ff.txt")
	run(repo, "commit", "-q", "-m", "feat/ff work")
	run(repo, "checkout", "-q", "main")
	run(repo, "merge", "-q", "--no-ff", "feat/ff") // fast-forward-able ancestry preserved

	if err := defaultDeleteBranch(gitExec, "feat/ff"); err != nil {
		t.Fatalf("expected defaultDeleteBranch to succeed via plain -d, got: %v", err)
	}
	branches, err := defaultListLocalBranches(gitExec)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range branches {
		if b == "feat/ff" {
			t.Fatal("feat/ff should have been deleted by defaultDeleteBranch (via -d)")
		}
	}

	// Codepath 2: feat/squash has NO ancestry with main (squash-merge) — plain `git branch -d`
	// refuses; defaultDeleteBranch must fall back to -D and still succeed.
	run(repo, "checkout", "-q", "-b", "feat/squash")
	if err := os.WriteFile(filepath.Join(repo, "squash.txt"), []byte("squash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "squash.txt")
	run(repo, "commit", "-q", "-m", "feat/squash work")
	run(repo, "checkout", "-q", "main")
	run(repo, "merge", "-q", "--squash", "feat/squash")
	run(repo, "commit", "-q", "-m", "squash-merge feat/squash")

	if _, err := exec.Command("git", "-C", repo, "branch", "-d", "feat/squash").CombinedOutput(); err == nil {
		t.Fatal("test setup invalid: git branch -d unexpectedly succeeded on a squash-merged branch")
	}
	if err := defaultDeleteBranch(gitExec, "feat/squash"); err != nil {
		t.Fatalf("expected defaultDeleteBranch to fall back to -D and succeed, got: %v", err)
	}
	branches, err = defaultListLocalBranches(gitExec)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range branches {
		if b == "feat/squash" {
			t.Fatal("feat/squash should have been deleted by defaultDeleteBranch (via fallback -D)")
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Stale origin/main — real git repository. Proves the AC "origin/main defasado leva a mais
// recusas, nunca a deleção indevida": when `git fetch origin --prune` fails (simulated by
// breaking the remote URL), evaluateBranchIntegration keeps using whatever origin/main ref is
// already resolvable locally. A branch that has, in fact, since been integrated upstream (but
// whose integration this stale ref cannot see) is reported KEPT (pending_work), never wrongly
// offered for deletion — the false negative this fixture proves is safe, in contrast to a false
// positive, which would not be.
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchPrune_RealGitRepo_StaleOriginMain_IsConservativeNotWrong(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	work := t.TempDir()
	bareDir := filepath.Join(work, "origin.git")
	cloneDir := filepath.Join(work, "clone")
	otherCloneDir := filepath.Join(work, "other-clone")

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

	// Bare "origin".
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "main")

	// Our clone under test.
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, cloneDir)
	run(cloneDir, "config", "user.email", "falsify@trackfw.test")
	run(cloneDir, "config", "user.name", "trackfw falsify")
	run(cloneDir, "config", "commit.gpgsign", "false")
	run(cloneDir, "config", "core.hooksPath", "/dev/null")

	if err := os.WriteFile(filepath.Join(cloneDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "base.txt")
	run(cloneDir, "commit", "-q", "-m", "base commit")
	run(cloneDir, "push", "-q", "origin", "main")

	// In our clone: branch feat/mine, touches mine.txt, never pushed/merged from here.
	run(cloneDir, "checkout", "-q", "-b", "feat/mine")
	if err := os.WriteFile(filepath.Join(cloneDir, "mine.txt"), []byte("mine v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "mine.txt")
	run(cloneDir, "commit", "-q", "-m", "feat/mine work")
	run(cloneDir, "checkout", "-q", "main")

	// Our clone's origin/main is now frozen at "base commit" — we deliberately never fetch again
	// in this clone, simulating a fetch failure / offline session from this point on.

	// Meanwhile, "someone else" merges the exact same content upstream via a second, independent
	// clone — main advances on the bare remote to include mine.txt, unbeknownst to our clone.
	if err := os.MkdirAll(otherCloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, otherCloneDir)
	run(otherCloneDir, "config", "user.email", "falsify@trackfw.test")
	run(otherCloneDir, "config", "user.name", "trackfw falsify")
	run(otherCloneDir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(otherCloneDir, "mine.txt"), []byte("mine v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(otherCloneDir, "add", "mine.txt")
	run(otherCloneDir, "commit", "-q", "-m", "someone else lands the same content upstream")
	run(otherCloneDir, "push", "-q", "origin", "main")

	// Break the remote URL in our clone so `git fetch origin --prune` fails deterministically —
	// our clone's origin/main stays stale at "base commit", never learning about the push above.
	run(cloneDir, "remote", "set-url", "origin", filepath.Join(work, "does-not-exist.git"))

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

	out := &bytes.Buffer{}
	deps := branchPruneDeps{
		gitExec:           gitExec,
		listLocalBranches: defaultListLocalBranches,
		currentBranch:     defaultCurrentBranchForPrune,
		worktreeBranches:  defaultWorktreeBranches,
		deleteBranch: func(func(args ...string) (string, error), string) error {
			t.Fatal("dry-run must never call deleteBranch")
			return nil
		},
		out: out,
	}

	if err := runBranchPrune(false, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(strings.ToLower(got), "warning") {
		t.Fatalf("expected a fetch-failure warning (remote URL was broken deliberately), got: %q", got)
	}
	// The core of this AC: feat/mine, though truly integrated upstream, must be reported KEPT
	// (pending_work against the stale local origin/main) — never offered for deletion.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "feat/mine ") {
			if strings.Contains(line, "delete") {
				t.Fatalf("stale origin/main must never make feat/mine look deletable, got line: %q", line)
			}
			if !strings.Contains(line, "keep") {
				t.Fatalf("expected feat/mine reported keep (pending, stale data), got line: %q", line)
			}
		}
	}

	// Contrast: with a working fetch, the exact same branch (unchanged locally) becomes
	// deletable — proving the staleness above was the cause of the conservative outcome, not a
	// pre-existing bug.
	run(cloneDir, "remote", "set-url", "origin", bareDir)
	run(cloneDir, "fetch", "-q", "origin")
	eval := evaluateBranchIntegration("feat/mine", gitExec)
	if eval.Decision != branchPruneDecisionIdentical {
		t.Fatalf("after a real fetch, expected feat/mine to become content_identical (deletable), got %v (%s)", eval.Decision, eval.Reason)
	}
	if !eval.Decision.deletable() {
		t.Fatal("after a real fetch, feat/mine must be deletable — confirms staleness (not a bug) caused the earlier conservative result")
	}
}

// TestEvaluateBranchIntegration_RealGitRepo_DocOnlyNeverIntegratedVsPartialResidue is the ML-1C
// discriminant, proved in a real repository with the two contrasting branches side by side (per
// "Auditoria do ML-1B" in the roadmap):
//
//   - feat/doc-real:  brand-new documentation, never merged anywhere. touched == diverg
//     (docs/guia-novo.md). Must be pending_work, kept — NOT review_doc_config, and calling it
//     "probable housekeeping" would be wrong advice about real, unmerged work.
//   - feat/residue:   touches both code (app.go, which lands in main) and doc (docs/notas.md,
//     which does NOT land in main — squash-merge residue). diverg (docs/notas.md) is a PROPER
//     subset of touched (app.go, docs/notas.md). This is the genuine housekeeping-residue case:
//     must be review_doc_config, kept, flagged for manual confirmation.
//
// Neither branch is ever deletable — the failure-closed guarantee never loosens.
func TestEvaluateBranchIntegration_RealGitRepo_DocOnlyNeverIntegratedVsPartialResidue(t *testing.T) {
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
		full := filepath.Join(cloneDir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("base.txt", "base\n")
	run(cloneDir, "add", "base.txt")
	run(cloneDir, "commit", "-q", "-m", "base commit")
	run(cloneDir, "push", "-q", "origin", "main")

	// feat/residue: touches app.go (code) and docs/notas.md (doc). Squash-merged, but only the
	// code file's content is picked up by the squash-merge commit below — docs/notas.md is
	// deliberately left out, simulating a human editing the doc differently during merge.
	run(cloneDir, "checkout", "-q", "-b", "feat/residue")
	writeFile("app.go", "package main\n")
	writeFile("docs/notas.md", "notas da branch\n")
	run(cloneDir, "add", "app.go", "docs/notas.md")
	run(cloneDir, "commit", "-q", "-m", "feat/residue work: code + doc")
	run(cloneDir, "checkout", "-q", "main")
	// Squash-merge only app.go's content into main — docs/notas.md never lands there, the
	// residue this discriminant targets.
	writeFile("app.go", "package main\n")
	run(cloneDir, "add", "app.go")
	run(cloneDir, "commit", "-q", "-m", "squash-merge feat/residue (code only, doc left out)")
	run(cloneDir, "push", "-q", "origin", "main")

	// feat/doc-real: brand-new documentation, branched off current main, never merged anywhere.
	run(cloneDir, "checkout", "-q", "-b", "feat/doc-real")
	writeFile("docs/guia-novo.md", "guia novo, nunca mergeado\n")
	run(cloneDir, "add", "docs/guia-novo.md")
	run(cloneDir, "commit", "-q", "-m", "feat/doc-real: never-merged documentation")
	run(cloneDir, "checkout", "-q", "main")
	run(cloneDir, "fetch", "-q", "origin")

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

	evalDocReal := evaluateBranchIntegration("feat/doc-real", gitExec)
	if evalDocReal.Decision != branchPruneDecisionPendingWork {
		t.Fatalf("feat/doc-real (never merged, doc-only) expected pending_work, got %v (%s)", evalDocReal.Decision, evalDocReal.Reason)
	}
	if evalDocReal.Decision.deletable() {
		t.Fatal("feat/doc-real must never be deletable")
	}
	if strings.Contains(evalDocReal.Reason, "housekeeping") {
		t.Fatalf("feat/doc-real must not be advised as housekeeping — it is real, unmerged work: %q", evalDocReal.Reason)
	}
	if len(evalDocReal.Touched) != len(evalDocReal.Diverged) {
		t.Fatalf("feat/doc-real: expected touched == diverg (nothing integrated), got touched=%v diverg=%v", evalDocReal.Touched, evalDocReal.Diverged)
	}

	evalResidue := evaluateBranchIntegration("feat/residue", gitExec)
	if evalResidue.Decision != branchPruneDecisionReviewDocConfig {
		t.Fatalf("feat/residue (partial integration, doc residue) expected review_doc_config, got %v (%s)", evalResidue.Decision, evalResidue.Reason)
	}
	if evalResidue.Decision.deletable() {
		t.Fatal("feat/residue must never be deletable")
	}
	if !strings.Contains(evalResidue.Reason, "confirm and delete manually") {
		t.Fatalf("feat/residue expected manual-confirmation guidance, got %q", evalResidue.Reason)
	}
	if len(evalResidue.Diverged) >= len(evalResidue.Touched) {
		t.Fatalf("feat/residue: expected diverg to be a PROPER subset of touched, got touched=%v diverg=%v", evalResidue.Touched, evalResidue.Diverged)
	}
}
