package commands

// branch_prune_remote_test.go — tests for ROADMAP-2026-10-10 ML-1B (`trackfw branch prune --remote`).
//
// Reconciliation Rule (CLAUDE.md §"Regra Dura de Reconciliação"): each test carries a
// "Conclusion:" comment stating what it asserts, and fails without the corresponding production code.
//
// Bite outcomes are shown at the end of the file in TestRemoteDeleteForceLease and
// TestRemotePrune_ContentHeuristicWithoutPR_Keep.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────────────────────────

// makeRemoteTestGhExec returns a ghExecFn that handles three kinds of calls:
//   - "api", "repos/<owner>/<repo>" (forgeDefaultBranch) → {"default_branch":"main"}
//   - "pr", "list", ... (queryForgePRs) → prJSON
//   - "api", "--paginate", "repos/.../branches?protected=true..." (queryProtectedBranches) → protJSON
func makeRemoteTestGhExec(prJSON, protJSON []byte) ghExecFn {
	return func(args ...string) ([]byte, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("empty args")
		}
		switch args[0] {
		case "pr":
			return prJSON, nil
		case "api":
			if len(args) >= 2 && args[1] == "--paginate" {
				return protJSON, nil
			}
			// forgeDefaultBranch
			return []byte(`{"default_branch":"main"}`), nil
		}
		return nil, fmt.Errorf("unexpected gh call: %v", args)
	}
}

// makeRemoteGitExec returns a gitExec stub for the remote section of runRemotePrune.
// responses maps joined-arg keys to (out, error). "remote get-url origin" is handled
// automatically to return "git@github.com:o/r.git" if not overridden.
func makeRemoteGitExec(t *testing.T, responses map[string]string, errorKeys map[string]error, pushCalls *[]string) func(...string) (string, error) {
	return func(args ...string) (string, error) {
		t.Helper()
		key := strings.Join(args, " ")
		if pushCalls != nil && len(args) > 0 && args[0] == "push" {
			*pushCalls = append(*pushCalls, key)
		}
		if err, ok := errorKeys[key]; ok {
			return "", err
		}
		if out, ok := responses[key]; ok {
			return out, nil
		}
		// Default responses for common calls.
		switch key {
		case "remote get-url origin":
			return "git@github.com:o/r.git", nil
		case "remote get-url --push origin":
			return "git@github.com:o/r.git", nil
		default:
			t.Fatalf("makeRemoteGitExec: unexpected call: git %s", key)
			return "", nil
		}
	}
}

// ── (a) No --remote flag: no push ────────────────────────────────────────────────────────────────

// TestRemotePrune_NoRemoteFlag_NoPushArgs asserts: without deps.remote=true, no "push" argument
// ever reaches gitExec, even when there is a merged_pr branch that would otherwise be deleted.
// Conclusion: the flag deps.remote=false is the sole gate; the remote section never executes.
// Sabotage: set deps.remote=true in runBranchPrune without setting the flag → test fails because
// the remote section is entered and push is called.
func TestRemotePrune_NoRemoteFlag_NoPushArgs(t *testing.T) {
	const tipSHA = "aaa00000bbb11111"
	const headOID = "ccc22222ddd33333"

	var pushCalls []string
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		if len(args) > 0 && args[0] == "push" {
			pushCalls = append(pushCalls, key)
			return "", nil
		}
		switch key {
		case "fetch origin --prune":
			return "", nil
		case "remote get-url origin":
			return "git@github.com:o/r.git", nil
		case "rev-parse --verify -q origin/main":
			return "abc123", nil
		case "branch --format=%(refname:short)":
			return "feat/merged-pr", nil
		case "symbolic-ref --quiet --short HEAD":
			return "main", nil
		case "worktree list --porcelain":
			return "", nil
		case "for-each-ref --format=%(upstream:short) refs/heads/feat/merged-pr":
			return "origin/feat/merged-pr", nil
		case "rev-parse feat/merged-pr":
			return tipSHA, nil
		case "cat-file -e " + headOID + "^{commit}":
			return "", nil // object exists
		case "merge-base --is-ancestor " + tipSHA + " " + headOID:
			return "", nil // tip is ancestor of PR head → merged_pr
		case "branch -d feat/merged-pr":
			return "", nil // delete succeeds
		}
		t.Fatalf("unexpected gitExec call: git %s", key)
		return "", nil
	}

	ghExec := makeRemoteTestGhExec(
		jsonPRs([]forgePR{
			{Number: 10, State: "MERGED", HeadRefName: "feat/merged-pr", HeadRefOid: headOID, BaseRefName: "main"},
		}),
		[]byte(`[]`),
	)

	var out strings.Builder
	deps := branchPruneDeps{
		gitExec:           gitExec,
		ghExec:            ghExec,
		listLocalBranches: defaultListLocalBranches,
		currentBranch:     defaultCurrentBranchForPrune,
		worktreeBranches:  defaultWorktreeBranches,
		deleteBranch: func(ge func(...string) (string, error), name string) error {
			_, err := ge("branch", "-d", name)
			return err
		},
		out:    &writerFromBuilder{&out},
		remote: false, // key: remote=false
	}

	if err := runBranchPrune(true, deps); err != nil {
		t.Fatalf("runBranchPrune returned error: %v", err)
	}
	if len(pushCalls) > 0 {
		t.Errorf("(a) push must not be called without --remote; got push calls: %v", pushCalls)
	}
}

// ── (b) --remote without --apply: no push ────────────────────────────────────────────────────────

// TestRemotePrune_NoApplyFlag_NoPush asserts: with --remote but without --apply, no "push"
// reaches gitExec and the output contains "[dry-run] would delete 1 remote branch(es)".
// Conclusion: apply=false suppresses all remote deletions; dry-run message is truthful.
// Sabotage: call deleteRemoteBranch unconditionally (skip the `!apply` guard) → test fails because
// push is called and output does not say "[dry-run]".
func TestRemotePrune_NoApplyFlag_NoPush(t *testing.T) {
	const tipSHA = "rem00000sha11111"
	const headOID = "mer22222ged33333"

	var pushCalls []string
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		if len(args) > 0 && args[0] == "push" {
			pushCalls = append(pushCalls, key)
			return "", nil
		}
		switch key {
		case "fetch origin --prune":
			return "", nil
		case "remote get-url origin":
			return "git@github.com:o/r.git", nil
		case "rev-parse --verify -q origin/main":
			return "abc123", nil
		case "branch --format=%(refname:short)":
			return "", nil // no local branches
		case "symbolic-ref --quiet --short HEAD":
			return "main", nil
		case "worktree list --porcelain":
			return "", nil
		// remote section:
		case "remote get-url --push origin":
			return "git@github.com:o/r.git", nil
		case "for-each-ref --format=%(refname) refs/remotes/origin/":
			return "refs/remotes/origin/feat/remote-merged\n", nil
		case "rev-parse --verify refs/remotes/origin/feat/remote-merged":
			return tipSHA, nil
		case "rev-parse " + tipSHA:
			return tipSHA, nil // evaluateBranchWithForge: rev-parse ref
		case "cat-file -e " + headOID + "^{commit}":
			return "", nil
		case "merge-base --is-ancestor " + tipSHA + " " + headOID:
			return "", nil // contained → merged_pr
		}
		t.Fatalf("unexpected gitExec call: git %s", key)
		return "", nil
	}

	ghExec := makeRemoteTestGhExec(
		jsonPRs([]forgePR{
			{Number: 20, State: "MERGED", HeadRefName: "feat/remote-merged", HeadRefOid: headOID, BaseRefName: "main"},
		}),
		[]byte(`[]`), // no protected branches
	)

	var out strings.Builder
	deps := branchPruneDeps{
		gitExec:           gitExec,
		ghExec:            ghExec,
		listLocalBranches: defaultListLocalBranches,
		currentBranch:     defaultCurrentBranchForPrune,
		worktreeBranches:  defaultWorktreeBranches,
		deleteBranch: func(_ func(...string) (string, error), _ string) error {
			t.Error("deleteBranch must not be called")
			return nil
		},
		out:    &writerFromBuilder{&out},
		remote: true,
	}

	if err := runBranchPrune(false, deps); err != nil {
		t.Fatalf("runBranchPrune returned error: %v", err)
	}
	if len(pushCalls) > 0 {
		t.Errorf("(b) push must not be called without --apply; got: %v", pushCalls)
	}
	got := out.String()
	if !strings.Contains(got, "would delete 1 remote branch") {
		t.Errorf("(b) expected dry-run message with 1 remote branch; got:\n%s", got)
	}
}

// ── helper: minimal remote deps for direct runRemotePrune tests ──────────────────────────────────

// makeMinimalRemoteDeps builds a branchPruneDeps sufficient for calling runRemotePrune directly.
// responses maps "joined git args" → stdout; protJSON is the protected-branches API response.
// If pushCalls is non-nil, push calls are appended to it.
func makeMinimalRemoteDeps(t *testing.T, responses map[string]string, errors_ map[string]error, protJSON []byte, pushCalls *[]string) branchPruneDeps {
	t.Helper()
	if responses == nil {
		responses = map[string]string{}
	}
	if errors_ == nil {
		errors_ = map[string]error{}
	}
	gitExec := makeRemoteGitExec(t, responses, errors_, pushCalls)
	ghExec := func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[1] == "--paginate" {
			return protJSON, nil
		}
		// forgeDefaultBranch (should not be called in runRemotePrune directly, but guard)
		return []byte(`{"default_branch":"main"}`), nil
	}
	return branchPruneDeps{
		gitExec: gitExec,
		ghExec:  ghExec,
		out:     &writerFromBuilder{new(strings.Builder)},
		remote:  true,
	}
}

// ── (c) merged_pr with contained tip → deletes ───────────────────────────────────────────────────

// TestRemotePrune_MergedPRContainedTip_Deletes asserts: a remote branch whose forge decision is
// merged_pr (tip SHA contained in PR head) produces a push call when --apply is set.
// Conclusion: merged_pr + apply → deleteRemoteBranch is called with the evaluated SHA and the push
// uses --force-with-lease=refs/heads/<b>:<sha>.
// Sabotage: check `eval.Decision.deletable()` instead of `== merged_pr` → test (j) fails;
// this test still passes, so (j) is the mordida target for A6.
func TestRemotePrune_MergedPRContainedTip_Deletes(t *testing.T) {
	const b = "feat/to-delete"
	const sha = "deadbeef00001111"
	const headOID = "cafecafe22223333"

	snapshot := makeSnapshotWith(
		forgePR{Number: 100, State: "MERGED", HeadRefName: b, HeadRefOid: headOID, BaseRefName: "main"},
	)

	var pushCalls []string
	deps := makeMinimalRemoteDeps(t, map[string]string{
		"for-each-ref --format=%(refname) refs/remotes/origin/": "refs/remotes/origin/" + b + "\n",
		"rev-parse --verify refs/remotes/origin/" + b:          sha,
		"rev-parse " + sha:                                       sha,
		"cat-file -e " + headOID + "^{commit}":                  "",
		"merge-base --is-ancestor " + sha + " " + headOID:       "",
		"push --force-with-lease=refs/heads/" + b + ":" + sha + " origin :refs/heads/" + b: "",
	}, nil, []byte(`[]`), &pushCalls)

	var out strings.Builder
	deps.out = &writerFromBuilder{&out}

	if err := runRemotePrune(true, false, false, "main", true, snapshot, deps); err != nil {
		t.Fatalf("runRemotePrune error: %v", err)
	}
	if len(pushCalls) == 0 {
		t.Errorf("(c) expected push call for merged_pr branch; got none. Output:\n%s", out.String())
	}
	want := "push --force-with-lease=refs/heads/" + b + ":" + sha + " origin :refs/heads/" + b
	if len(pushCalls) == 0 || pushCalls[0] != want {
		t.Errorf("(c) push args:\n  want: %s\n  got:  %v", want, pushCalls)
	}
}

// ── (d) commits after merged PR → keep ───────────────────────────────────────────────────────────

// TestRemotePrune_CommitsAfterMergedPR_Keep asserts: a remote branch with commits after the
// merged PR head is not deleted (commits_after_merged_pr decision → keep).
// Conclusion: tip not contained in PR head + PR head is ancestor of tip → keep, no push.
// Sabotage: invert the is-ancestor check → test fails because branch gets merged_pr.
func TestRemotePrune_CommitsAfterMergedPR_Keep(t *testing.T) {
	const b = "feat/after-merge"
	const sha = "tip0000000001111"    // remote tip (newer than PR head)
	const headOID = "head222222223333" // PR head (older than tip)

	snapshot := makeSnapshotWith(
		forgePR{Number: 200, State: "MERGED", HeadRefName: b, HeadRefOid: headOID, BaseRefName: "main"},
	)

	// Case 2: tip is NOT an ancestor of headOID (headOID < tip in history), but headOID IS
	// an ancestor of tip (PR head is before the tip). Exit 1 for "is sha ancestor of head?",
	// exit 0 for "is head ancestor of sha?".
	deps := makeMinimalRemoteDeps(t, map[string]string{
		"for-each-ref --format=%(refname) refs/remotes/origin/": "refs/remotes/origin/" + b + "\n",
		"rev-parse --verify refs/remotes/origin/" + b:          sha,
		"rev-parse " + sha:                                       sha,
		"cat-file -e " + headOID + "^{commit}":                  "",
		// Case 2: headOID is ancestor of sha (but not vice-versa)
		"merge-base --is-ancestor " + headOID + " " + sha: "",
	}, map[string]error{
		// tip is NOT ancestor of head (case 1 fails)
		"merge-base --is-ancestor " + sha + " " + headOID: fmt.Errorf("git merge-base --is-ancestor exited with 1"),
	}, []byte(`[]`), nil)

	var out strings.Builder
	deps.out = &writerFromBuilder{&out}

	if err := runRemotePrune(true, false, false, "main", true, snapshot, deps); err != nil {
		t.Fatalf("runRemotePrune error: %v", err)
	}
	// "deleted N" or "[dry-run] would delete" indicate unwanted deletion; "nothing to delete"
	// and "keep" are correct. Do NOT use a plain strings.Contains("delete") because the footer
	// "nothing to delete (remote)." also contains the word.
	if strings.Contains(out.String(), "deleted ") || strings.Contains(out.String(), "would delete") {
		t.Errorf("(d) branch with commits after merged PR must be kept, not deleted:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "keep") {
		t.Errorf("(d) expected keep in output; got:\n%s", out.String())
	}
}

// ── (e) open PR of same head → keep ──────────────────────────────────────────────────────────────

// TestRemotePrune_OpenPRSameHead_Keep asserts: a remote branch with an OPEN PR is not deleted,
// even when a MERGED PR for the same head also exists.
// Conclusion: open_pr decision (case 0) takes priority over merged_pr; branch is kept.
// Sabotage: remove the OPEN-PR case from evaluateBranchWithForge → test fails with merged_pr.
func TestRemotePrune_OpenPRSameHead_Keep(t *testing.T) {
	const b = "feat/open-pr"
	const sha = "openbranch000001"
	const headOID = "openprhead000002"

	snapshot := makeSnapshotWith(
		// MERGED: would give merged_pr if evaluated alone.
		forgePR{Number: 300, State: "MERGED", HeadRefName: b, HeadRefOid: headOID, BaseRefName: "main"},
		// OPEN: must take priority (case 0).
		forgePR{Number: 301, State: "OPEN", HeadRefName: b, HeadRefOid: headOID, BaseRefName: "main"},
	)

	deps := makeMinimalRemoteDeps(t, map[string]string{
		"for-each-ref --format=%(refname) refs/remotes/origin/": "refs/remotes/origin/" + b + "\n",
		"rev-parse --verify refs/remotes/origin/" + b:          sha,
		"rev-parse " + sha:                                       sha,
		"cat-file -e " + headOID + "^{commit}":                  "",
		"merge-base --is-ancestor " + sha + " " + headOID:       "",
	}, nil, []byte(`[]`), nil)

	var out strings.Builder
	deps.out = &writerFromBuilder{&out}

	if err := runRemotePrune(true, false, false, "main", true, snapshot, deps); err != nil {
		t.Fatalf("runRemotePrune error: %v", err)
	}
	if strings.Contains(out.String(), "deleted ") || strings.Contains(out.String(), "would delete") {
		t.Errorf("(e) branch with OPEN PR must be kept; output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "keep") {
		t.Errorf("(e) expected keep; output:\n%s", out.String())
	}
}

// ── (f) base of open PR → keep ───────────────────────────────────────────────────────────────────

// TestRemotePrune_OpenPRBase_Keep asserts: a remote branch that is the base of an OPEN PR is
// not deleted (A3, ADR-2026-10-12 — deleting it would close the dependent PR).
// Conclusion: branch is base of open PR → keep before any forge evaluation.
// Sabotage: remove the openPRBases check → test fails when the base branch has no other
// protection and gets evaluated (may get a merged_pr or heuristic decision).
func TestRemotePrune_OpenPRBase_Keep(t *testing.T) {
	const base = "feat/base-branch" // this is the base of an open stacked PR
	const stack = "feat/stacked"    // open PR targeting feat/base-branch

	snapshot := makeSnapshotWith(
		// Stacked PR: open, targeting feat/base-branch.
		forgePR{Number: 400, State: "OPEN", HeadRefName: stack, BaseRefName: base},
	)

	deps := makeMinimalRemoteDeps(t, map[string]string{
		"for-each-ref --format=%(refname) refs/remotes/origin/": "refs/remotes/origin/" + base + "\n",
		// No rev-parse needed — the base branch is excluded before evaluation.
	}, nil, []byte(`[]`), nil)

	var out strings.Builder
	deps.out = &writerFromBuilder{&out}

	if err := runRemotePrune(true, false, false, "main", true, snapshot, deps); err != nil {
		t.Fatalf("runRemotePrune error: %v", err)
	}
	if strings.Contains(out.String(), "deleted ") || strings.Contains(out.String(), "would delete") {
		t.Errorf("(f) base of open PR must be kept; output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "keep") && !strings.Contains(out.String(), "base of open PR") {
		t.Errorf("(f) expected keep (base of open PR); output:\n%s", out.String())
	}
}

// ── (g) protected branch → keep ──────────────────────────────────────────────────────────────────

// TestRemotePrune_ProtectedBranch_Keep asserts: a branch listed as protected via
// `gh api branches?protected=true` is not deleted (D6/A4, ADR-2026-10-12).
// Conclusion: protected branch → keep before forge evaluation; no rev-parse, no push.
// Sabotage: skip the protectedBranches check → test fails when the protected branch has a
// merged_pr signal and gets pushed.
func TestRemotePrune_ProtectedBranch_Keep(t *testing.T) {
	const b = "feat/protected"
	const sha = "prot0000000a1111"
	const headOID = "prot111111222222"

	snapshot := makeSnapshotWith(
		forgePR{Number: 500, State: "MERGED", HeadRefName: b, HeadRefOid: headOID, BaseRefName: "main"},
	)

	protectedJSON, _ := json.Marshal([]struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
	}{{Name: b, Protected: true}})

	var pushCalls []string
	// Note: rev-parse and git eval calls are deliberately absent from the response map
	// because the protected branch must be excluded before evaluation.
	deps := makeMinimalRemoteDeps(t, map[string]string{
		"for-each-ref --format=%(refname) refs/remotes/origin/": "refs/remotes/origin/" + b + "\n",
	}, nil, protectedJSON, &pushCalls)

	var out strings.Builder
	deps.out = &writerFromBuilder{&out}

	if err := runRemotePrune(true, false, false, "main", true, snapshot, deps); err != nil {
		t.Fatalf("runRemotePrune error: %v", err)
	}
	if len(pushCalls) > 0 {
		t.Errorf("(g) protected branch must not be pushed; push calls: %v", pushCalls)
	}
	if strings.Contains(out.String(), "deleted ") || strings.Contains(out.String(), "would delete") {
		t.Errorf("(g) protected branch must be kept, not deleted; output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "keep") && !strings.Contains(out.String(), "protected") {
		t.Errorf("(g) expected keep (protected branch); output:\n%s", out.String())
	}
}

// ── (h) default branch and origin/HEAD are never evaluated ───────────────────────────────────────

// TestRemotePrune_DefaultAndHEAD_NeverEvaluated asserts: refs/remotes/origin/HEAD (A8) and the
// resolved default branch (D2) are excluded from evaluation and never receive a push call.
// Uses "master" as the default branch name (different from "main") to confirm the exclusion
// applies to the forge-resolved name, not a hardcoded literal.
// Conclusion: HEAD excluded by full refname; default branch excluded by name comparison; only
// the non-default, non-HEAD branch is evaluated.
// Sabotage: use %(refname:short) instead of %(refname) for HEAD → HEAD short name is "origin"
// which does not match the filter "refs/remotes/origin/HEAD" → HEAD is evaluated (A8 violated).
func TestRemotePrune_DefaultAndHEAD_NeverEvaluated(t *testing.T) {
	const defaultBranch = "master"
	const otherBranch = "feat/should-be-evaluated"
	const otherSHA = "eval00000000aaaa"
	const headOID = "evalprhead000001"

	snapshot := makeSnapshotWith(
		forgePR{Number: 600, State: "MERGED", HeadRefName: otherBranch, HeadRefOid: headOID, BaseRefName: defaultBranch},
	)

	var evaluatedBranches []string
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch key {
		case "remote get-url origin":
			return "git@github.com:o/r.git", nil
		case "remote get-url --push origin":
			return "git@github.com:o/r.git", nil
		case "for-each-ref --format=%(refname) refs/remotes/origin/":
			// Includes HEAD, default branch (master), and the other branch.
			return strings.Join([]string{
				"refs/remotes/origin/HEAD",
				"refs/remotes/origin/" + defaultBranch,
				"refs/remotes/origin/" + otherBranch,
			}, "\n"), nil
		}
		// Track which branch gets a rev-parse (= evaluated).
		if len(args) == 3 && args[0] == "rev-parse" && args[1] == "--verify" {
			b := strings.TrimPrefix(args[2], "refs/remotes/origin/")
			evaluatedBranches = append(evaluatedBranches, b)
			return otherSHA, nil
		}
		switch key {
		case "rev-parse " + otherSHA:
			return otherSHA, nil
		case "cat-file -e " + headOID + "^{commit}":
			return "", nil
		case "merge-base --is-ancestor " + otherSHA + " " + headOID:
			return "", nil
		}
		t.Fatalf("unexpected gitExec call: git %s", key)
		return "", nil
	}

	ghExec := func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[1] == "--paginate" {
			return []byte(`[]`), nil
		}
		return []byte(`{"default_branch":"master"}`), nil
	}

	var out strings.Builder
	deps := branchPruneDeps{
		gitExec: gitExec,
		ghExec:  ghExec,
		out:     &writerFromBuilder{&out},
		remote:  true,
	}

	if err := runRemotePrune(false, false, false, defaultBranch, true, snapshot, deps); err != nil {
		t.Fatalf("runRemotePrune error: %v", err)
	}

	// HEAD and master must not appear in evaluated branches.
	for _, b := range evaluatedBranches {
		if b == "HEAD" || b == defaultBranch {
			t.Errorf("(h) %q was evaluated but should be excluded (HEAD/default branch)", b)
		}
	}
	// Only the other branch should have been evaluated.
	found := false
	for _, b := range evaluatedBranches {
		if b == otherBranch {
			found = true
		}
	}
	if !found {
		t.Errorf("(h) %q was not evaluated; evaluatedBranches=%v", otherBranch, evaluatedBranches)
	}
}

// ── (i) preconditions block all remote deletions ─────────────────────────────────────────────────

// TestRemotePrune_Preconditions_BlockAll asserts: each precondition of D4/A5 individually
// blocks all remote deletions, and the output names the cause. Each sub-test includes a
// control branch (merged_pr) that would be deleted without the precondition, proving the
// block is the reason — not the absence of candidates.
// Conclusions per sub-test are listed inline.
func TestRemotePrune_Preconditions_BlockAll(t *testing.T) {
	const b = "feat/would-be-deleted"
	const sha = "block0000000a1111"
	const headOID = "blockprhead00001"

	snapshot := makeSnapshotWith(
		forgePR{Number: 700, State: "MERGED", HeadRefName: b, HeadRefOid: headOID, BaseRefName: "main"},
	)

	gitResps := map[string]string{
		"for-each-ref --format=%(refname) refs/remotes/origin/": "refs/remotes/origin/" + b + "\n",
		"rev-parse --verify refs/remotes/origin/" + b:          sha,
		"rev-parse " + sha:                                       sha,
		"cat-file -e " + headOID + "^{commit}":                  "",
		"merge-base --is-ancestor " + sha + " " + headOID:       "",
	}

	cases := []struct {
		name         string
		fetchFailed  bool
		fetchTimedOut bool
		nilSnapshot  bool
		fromForge    bool
		ghExecFail  bool
		pushURLDiff  bool
		wantCause   string
	}{
		{
			name: "fetch_failed",
			fetchFailed: true,
			fromForge:   true,
			wantCause:  "fetch failed",
			// Conclusion: fetch failure blocks remote deletion.
		},
		{
			name: "fetch_timed_out",
			fetchTimedOut: true,
			fromForge:     true,
			wantCause:    "fetch timed out",
			// Conclusion: fetch timeout blocks remote deletion.
		},
		{
			name: "forge_snapshot_nil",
			nilSnapshot: true,
			fromForge:   true,
			wantCause:  "snapshot unavailable",
			// Conclusion: nil snapshot (forge degraded/truncated) blocks remote deletion.
		},
		{
			name: "default_branch_not_from_forge",
			fromForge:  false,
			wantCause: "default branch could not be read from forge",
			// Conclusion: default branch not resolvable from forge blocks remote deletion.
			// ADR D4 third bullet. Note: this test intentionally passes fromForge=false
			// together with a non-nil snapshot (the two conditions are checked independently).
		},
		{
			name: "protected_branches_failed",
			fromForge:  true,
			ghExecFail: true,
			wantCause: "could not read protected branches",
			// Conclusion: failure to query protected branches blocks remote deletion (A4 fail-closed).
		},
		{
			name: "push_url_differs",
			fromForge:  true,
			pushURLDiff: true,
			wantCause:  "fetch URL and push URL for origin differ",
			// Conclusion: push URL ≠ fetch URL blocks remote deletion (A5).
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			snap := snapshot
			if tc.nilSnapshot {
				snap = nil
			}

			gitExec := func(args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "remote get-url origin":
					return "git@github.com:o/r.git", nil
				case "remote get-url --push origin":
					if tc.pushURLDiff {
						return "git@github.com:o/r-other.git", nil // different push URL
					}
					return "git@github.com:o/r.git", nil
				}
				if v, ok := gitResps[key]; ok {
					return v, nil
				}
				// Block push explicitly.
				if len(args) > 0 && args[0] == "push" {
					t.Errorf("push must not be called when precondition blocks; args: %v", args)
					return "", nil
				}
				t.Fatalf("unexpected gitExec call: git %s", key)
				return "", nil
			}

			ghExec := func(args ...string) ([]byte, error) {
				if tc.ghExecFail {
					return nil, fmt.Errorf("simulated gh api failure")
				}
				if len(args) >= 2 && args[1] == "--paginate" {
					return []byte(`[]`), nil
				}
				return []byte(`{"default_branch":"main"}`), nil
			}

			var out strings.Builder
			deps := branchPruneDeps{
				gitExec: gitExec,
				ghExec:  ghExec,
				out:     &writerFromBuilder{&out},
				remote:  true,
			}

			if err := runRemotePrune(true, tc.fetchFailed, tc.fetchTimedOut, "main", tc.fromForge, snap, deps); err != nil {
				t.Fatalf("runRemotePrune error: %v", err)
			}
			if !strings.Contains(out.String(), tc.wantCause) {
				t.Errorf("(%s) output must contain %q; got:\n%s", tc.name, tc.wantCause, out.String())
			}
			if strings.Contains(out.String(), "delete  ") {
				t.Errorf("(%s) must not report delete when precondition fires; got:\n%s", tc.name, out.String())
			}
		})
	}
}

// ── (j) content heuristic says "integrated" but no PR → keep ─────────────────────────────────────

// TestRemotePrune_ContentHeuristicWithoutPR_Keep asserts: a remote branch where the content
// heuristic would classify the branch as integreated (no_own_work or content_identical) but
// where no merged PR exists is kept, not deleted.
// Conclusion: A6 — only decision == merged_pr authorises remote deletion; deletable() is never
// used for the remote path.
//
// Bite (M4): replace `eval.Decision != branchPruneDecisionMergedPR` with `!eval.Decision.deletable()`
// in runRemotePrune → no_own_work.deletable()==true → push is called → this test fails.
// Test (c) (merged_pr) still passes because merged_pr.deletable() is also true.
//
// Bite output (measured on isolated copy):
// FAIL: (j) branch with heuristic=no_own_work but no PR must be kept; push was called:
//   [push --force-with-lease=refs/heads/feat/heuristic-only:heur000000001111 origin :refs/heads/feat/heuristic-only]
//   Output: "feat/heuristic-only   delete  no own work relative to origin/main — safe to delete"
//           "deleted 1 remote branch(es): feat/heuristic-only"
// EXIT=1
func TestRemotePrune_ContentHeuristicWithoutPR_Keep(t *testing.T) {
	const b = "feat/heuristic-only"
	const sha = "heur000000001111"
	// No PR for this branch in the snapshot.
	snapshot := makeSnapshotWith() // empty snapshot

	// Content heuristic: touched=[] (no own work) → no_own_work (deletable by deletable()).
	// The merge-base is sha itself (same commit), touched files is empty.
	deps := makeMinimalRemoteDeps(t, map[string]string{
		"for-each-ref --format=%(refname) refs/remotes/origin/": "refs/remotes/origin/" + b + "\n",
		"rev-parse --verify refs/remotes/origin/" + b:          sha,
		"rev-parse " + sha:                                       sha,
		// evaluateBranchWithForge: no PRs → falls to content heuristic (case 5)
		// evaluateBranchIntegration: merge-base returns sha (no own work → no_own_work)
		"merge-base origin/main " + sha:      sha,
		"diff --name-only -z " + sha + " " + sha: "",  // empty touched → no_own_work
	}, nil, []byte(`[]`), nil)

	var out strings.Builder
	deps.out = &writerFromBuilder{&out}

	var pushCalls []string
	origGit := deps.gitExec
	deps.gitExec = func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "push" {
			pushCalls = append(pushCalls, strings.Join(args, " "))
			return "", nil
		}
		return origGit(args...)
	}

	if err := runRemotePrune(true, false, false, "main", true, snapshot, deps); err != nil {
		t.Fatalf("runRemotePrune error: %v", err)
	}
	if len(pushCalls) > 0 {
		t.Errorf("(j) branch with heuristic=no_own_work but no PR must be kept; push was called: %v\nOutput:\n%s", pushCalls, out.String())
	}
	if strings.Contains(out.String(), "delete  ") {
		t.Errorf("(j) branch with heuristic only must be kept, not deleted:\n%s", out.String())
	}
	// Positive assertion: the output must name the heuristic reason. This confirms the branch
	// was CLASSIFIED as no_own_work (deletable by deletable()) and kept only because of A6
	// (not because it fell off some earlier path). Under M4 (deletable() replaces == merged_pr),
	// this assertion still passes — but push is called and the assertion above fails, proving M4
	// is the correct sabotage target for A6.
	if !strings.Contains(out.String(), "no own work") {
		t.Errorf("(j) expected heuristic reason 'no own work' in output; got:\n%s", out.String())
	}
}

// ── TestRemoteDeleteForceLease (A9) ──────────────────────────────────────────────────────────────

// TestRemoteDeleteForceLease exercises the force-with-lease contract of deleteRemoteBranch
// against a real bare git repository (A9, ADR-2026-10-12 D5, Wave 0 Seção 3).
//
// Three arms:
//
//	A: evaluated SHA == remote SHA → push succeeds, ref is gone from bare
//	B: remote was advanced by another clone after evaluation → push rejected (stale info)
//	C: another clone advanced remote AND our clone re-fetched between evaluation and removal;
//	   deleteRemoteBranch uses the EVALUATED SHA (not the re-fetched one) → push rejected
//
// Verification uses `git rev-parse --verify --quiet refs/heads/<b>` on the bare repo.
// git ls-remote is NOT used (it exits 0 with empty output when the ref is absent — false positive).
//
// Mutants verified during development (edit branch_prune_remote.go, run, restore, cmp):
//
//	Mutant 1 — implicit lease (drop `:<sha>`):
//	  lease := "refs/heads/" + b  // no :<sha>
//	  → Arm A passes. Arm B passes (tracking ref not re-fetched, SHA_OLD still there → rejected).
//	    Arm C FAILS: after clone1 re-fetches, tracking ref = SHA_NEW; implicit lease uses
//	    SHA_NEW as the expected value, which matches the remote → push succeeds → TOCTOU.
//	  Measured: FAIL arm_C_refetch_preserves_evaluated_sha
//	    "arm C: deleteRemoteBranch must be rejected when using pre-fetch SHA after a re-fetch;
//	     got nil (TOCTOU protection failed)" EXIT=1
//
//	Mutant 2 — lease on wrong ref (refs/remotes/origin/<b> instead of refs/heads/<b>):
//	  lease := "refs/remotes/origin/" + b + ":" + sha
//	  → The server never has a ref named refs/remotes/origin/<b>; the lease is vacuously
//	    satisfied and git pushes unconditionally.
//	  → Arm B FAILS: remote was advanced but the push succeeds → branch deleted.
//	  → Arm C FAILS: same reason (lease vacuously passes after re-fetch).
//	  Measured: FAIL arm_B_remote_advanced_rejects, arm_C_refetch_preserves_evaluated_sha
//	    "arm B: deleteRemoteBranch must be rejected when remote advanced; got nil error"
//	    "arm C: deleteRemoteBranch must be rejected when using pre-fetch SHA after a re-fetch;
//	     got nil (TOCTOU protection failed)" EXIT=1
func TestRemoteDeleteForceLease(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	work := t.TempDir()

	cfgPath := filepath.Join(work, "empty-gitconfig")
	if err := os.WriteFile(cfgPath, []byte("[gc]\n\tauto = 0\n[maintenance]\n\tauto = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_CONFIG_GLOBAL", cfgPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")

	gitEnv := os.Environ() // includes the setenv values

	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	runSilent := func(dir string, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = gitEnv
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

	// Build the fixture: bare repo + initial clone + base commit.
	bareDir := filepath.Join(work, "bare.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "main")

	clone1Dir := filepath.Join(work, "clone1")
	run(work, "clone", "-q", bareDir, clone1Dir)

	// Initial commit on main.
	if err := os.WriteFile(filepath.Join(clone1Dir, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(clone1Dir, "add", "base.txt")
	run(clone1Dir, "commit", "-q", "-m", "base")
	run(clone1Dir, "push", "-q", "origin", "main")

	// gitExec scoped to clone1 — this is the executor passed to deleteRemoteBranch.
	gitExec := func(args ...string) (string, error) {
		return runSilent(clone1Dir, args...)
	}

	// refExistsInBare reports whether refs/heads/<name> exists in the bare repo.
	// Uses rev-parse --verify --quiet, NOT ls-remote (ls-remote exits 0 with empty output
	// when the ref is absent — that is a false positive for "ref exists").
	refExistsInBare := func(branchName string) bool {
		ref := "refs/heads/" + branchName
		cmd := exec.Command("git", "-c", "commit.gpgsign=false", "rev-parse", "--verify", "--quiet", ref)
		cmd.Dir = bareDir
		cmd.Env = gitEnv
		return cmd.Run() == nil
	}

	// pushFileToNewBranch creates a branch in clone1 with a new commit and pushes it to bare.
	// Returns the pushed SHA.
	pushFileToNewBranch := func(branchName, filename string) string {
		t.Helper()
		run(clone1Dir, "checkout", "-q", "-b", branchName, "main")
		if err := os.WriteFile(filepath.Join(clone1Dir, filename), []byte(filename), 0o644); err != nil {
			t.Fatal(err)
		}
		run(clone1Dir, "add", filename)
		run(clone1Dir, "commit", "-q", "-m", "add "+filename)
		run(clone1Dir, "push", "-q", "origin", branchName)
		sha := run(clone1Dir, "rev-parse", "refs/remotes/origin/"+branchName)
		run(clone1Dir, "checkout", "-q", "main")
		return sha
	}

	// ── Arm A: SHA equal → deletion succeeds ─────────────────────────────────────
	t.Run("arm_A_sha_equal_deletes", func(t *testing.T) {
		bName := "feat-A"
		sha := pushFileToNewBranch(bName, "file-A.txt")

		// Verify fixture: ref exists before deletion.
		if !refExistsInBare(bName) {
			t.Fatalf("arm A fixture: refs/heads/%s not found in bare before deletion", bName)
		}

		// Call the production function.
		if err := deleteRemoteBranch(gitExec, bName, sha); err != nil {
			t.Fatalf("arm A: deleteRemoteBranch failed: %v", err)
		}
		// Verify: ref must no longer exist.
		if refExistsInBare(bName) {
			t.Errorf("arm A: refs/heads/%s still exists in bare after deletion", bName)
		}
	})

	// ── Arm B: remote advanced after evaluation → rejection ──────────────────────
	t.Run("arm_B_remote_advanced_rejects", func(t *testing.T) {
		bName := "feat-B"
		oldSHA := pushFileToNewBranch(bName, "file-B.txt")

		// clone2 advances the branch, pushing a new commit to bare.
		clone2Dir := filepath.Join(work, "clone2-B")
		run(work, "clone", "-q", bareDir, clone2Dir)
		run(clone2Dir, "checkout", "-q", "-b", bName, "origin/"+bName)
		if err := os.WriteFile(filepath.Join(clone2Dir, "file-B2.txt"), []byte("B2"), 0o644); err != nil {
			t.Fatal(err)
		}
		run(clone2Dir, "add", "file-B2.txt")
		run(clone2Dir, "commit", "-q", "-m", "advance B")
		run(clone2Dir, "push", "-q", "origin", bName)

		// clone1 has NOT re-fetched: its refs/remotes/origin/feat-B is at oldSHA.
		// deleteRemoteBranch uses oldSHA as the lease expectation.
		err := deleteRemoteBranch(gitExec, bName, oldSHA)
		if err == nil {
			t.Errorf("arm B: deleteRemoteBranch must be rejected when remote advanced; got nil error")
		}
		// Verify: ref must still exist (with the new SHA from clone2).
		if !refExistsInBare(bName) {
			t.Errorf("arm B: refs/heads/%s was deleted but should have been preserved", bName)
		}
	})

	// ── Arm C: refetch between evaluation and removal → rejection ────────────────
	t.Run("arm_C_refetch_preserves_evaluated_sha", func(t *testing.T) {
		bName := "feat-C"
		evaluatedSHA := pushFileToNewBranch(bName, "file-C.txt")

		// clone3 advances the branch and pushes.
		clone3Dir := filepath.Join(work, "clone3-C")
		run(work, "clone", "-q", bareDir, clone3Dir)
		run(clone3Dir, "checkout", "-q", "-b", bName, "origin/"+bName)
		if err := os.WriteFile(filepath.Join(clone3Dir, "file-C2.txt"), []byte("C2"), 0o644); err != nil {
			t.Fatal(err)
		}
		run(clone3Dir, "add", "file-C2.txt")
		run(clone3Dir, "commit", "-q", "-m", "advance C")
		run(clone3Dir, "push", "-q", "origin", bName)

		// clone1 re-fetches: refs/remotes/origin/feat-C is now at the NEW SHA.
		// If we were to use the current tracking ref (implicit lease or re-read),
		// the push would succeed (TOCTOU). We must use evaluatedSHA (the old SHA).
		run(clone1Dir, "fetch", "-q", "origin")

		// deleteRemoteBranch must use evaluatedSHA (the pre-fetch SHA), not the current one.
		err := deleteRemoteBranch(gitExec, bName, evaluatedSHA)
		if err == nil {
			t.Errorf("arm C: deleteRemoteBranch must be rejected when using pre-fetch SHA " +
				"after a re-fetch; got nil (TOCTOU protection failed)")
		}
		if !refExistsInBare(bName) {
			t.Errorf("arm C: refs/heads/%s was deleted but should have been preserved", bName)
		}
	})
}
