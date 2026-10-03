package commands

// branch_prune_forge_test.go — tests for D1 cases (ADR-2026-10-03) and the A1-A8 adjustments.
//
// Every test in this file exercises evaluateBranchWithForge or queryForgePRs directly and is
// named after the D1 case or A-adjustment it asserts.  The reconciliation sentence per test
// follows as a doc-comment on the function, per the Reconciliation Rule (CLAUDE.md §"Regra Dura
// de Reconciliação").
//
// Bite-proof structure: each test reaches the call site of the decision under test, not the
// declaration of the constant.  The sabotage target is stated in the test comment.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// jsonPRs serialises a slice of forgePR objects into the compact JSON format that `gh pr list
// --json ...` returns.
func jsonPRs(prs []forgePR) []byte {
	b, err := json.Marshal(prs)
	if err != nil {
		panic(err)
	}
	return b
}

// makeSnapshotWith builds a *prSnapshot containing the given PRs.
func makeSnapshotWith(prs ...forgePR) *prSnapshot {
	return &prSnapshot{prs: prs}
}

// stubGhExec returns a ghExecFn that always returns the given bytes with no error.
func stubGhExec(data []byte) ghExecFn {
	return func(args ...string) ([]byte, error) {
		return data, nil
	}
}

// ─── D1 Case 0: OPEN PR → keep ───────────────────────────────────────────────────────────────

// TestD1_Case0_OpenPR_Keep asserts: an OPEN, non-fork PR for a branch produces open_pr (keep),
// even when a MERGED PR with the same head name also exists.
// Sabotage: remove the "OPEN" case from the evaluateBranchWithForge switch → test fails with
// merged_pr (delete) instead of open_pr.
func TestD1_Case0_OpenPR_Keep(t *testing.T) {
	tip := "aaaa0000"
	snapshot := makeSnapshotWith(
		// MERGED PR: tip is ancestor of head → would normally give delete (case 1).
		forgePR{Number: 10, State: "MERGED", HeadRefName: "feat/x", HeadRefOid: "aaaa0000", BaseRefName: "main"},
		// OPEN PR: must take priority (case 0).
		forgePR{Number: 11, State: "OPEN", HeadRefName: "feat/x", HeadRefOid: "aaaa0000", BaseRefName: "main"},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "rev-parse feat/x":
			return tip, nil
		case key == "cat-file -e aaaa0000^{commit}":
			return "", nil // object exists
		case key == "merge-base --is-ancestor aaaa0000 aaaa0000":
			return "", nil // tip is ancestor of (equal to) head → case 1 if OPEN wasn't checked first
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}
	eval := evaluateBranchWithForge("feat/x", "feat/x", "origin/feat/x", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionOpenPR {
		t.Fatalf("case 0: expected open_pr, got %q (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("case 0: open_pr must not be deletable")
	}
}

// ─── D1 Case 1: MERGED PR contains tip → delete ──────────────────────────────────────────────

// TestD1_Case1_MergedPR_Delete asserts: a MERGED, non-fork PR whose head commit contains the
// local tip (is-ancestor exit 0) produces merged_pr (delete).
// Sabotage: change is-ancestor call arguments so tip is not tested against headRefOid → test
// fails with pending_work or content-heuristic result instead.
func TestD1_Case1_MergedPR_Delete(t *testing.T) {
	tip := "bbbb1111"
	prHead := "bbbb1111" // tip == head (common case)
	snapshot := makeSnapshotWith(
		forgePR{Number: 20, State: "MERGED", HeadRefName: "feat/y", HeadRefOid: prHead, BaseRefName: "main"},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "rev-parse feat/y":
			return tip, nil
		case key == "cat-file -e "+prHead+"^{commit}":
			return "", nil
		case key == "merge-base --is-ancestor "+tip+" "+prHead:
			return "", nil // tip ≤ prHead → merged
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}
	eval := evaluateBranchWithForge("feat/y", "feat/y", "origin/feat/y", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionMergedPR {
		t.Fatalf("case 1: expected merged_pr, got %q (%s)", eval.Decision, eval.Reason)
	}
	if !eval.Decision.deletable() {
		t.Fatal("case 1: merged_pr must be deletable")
	}
}

// ─── D1 Case 2: tip has commits after merged PR head → keep ──────────────────────────────────

// TestD1_Case2_CommitsAfterMerged_Keep asserts: when the PR head is an ancestor of the local
// tip (new commits after merge), the decision is commits_after_merged_pr (keep).
// Sabotage: swap the is-ancestor argument order in the case-2 check so prHead is tested as
// descendant → test fails, decision falls to content heuristic or diverged.
func TestD1_Case2_CommitsAfterMerged_Keep(t *testing.T) {
	tip := "cccc2222"
	prHead := "cccc1111" // prHead is an ancestor of tip
	snapshot := makeSnapshotWith(
		forgePR{Number: 30, State: "MERGED", HeadRefName: "feat/z", HeadRefOid: prHead, BaseRefName: "main"},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "rev-parse feat/z":
			return tip, nil
		case key == "cat-file -e "+prHead+"^{commit}":
			return "", nil
		case key == "merge-base --is-ancestor "+tip+" "+prHead:
			// tip is NOT ≤ prHead → not a case-1
			return "", fmt.Errorf("git merge-base --is-ancestor %s %s exited with 1", tip, prHead)
		case key == "merge-base --is-ancestor "+prHead+" "+tip:
			// prHead IS ≤ tip → new commits after the merged PR
			return "", nil
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}
	eval := evaluateBranchWithForge("feat/z", "feat/z", "origin/feat/z", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionCommitsAfterMerged {
		t.Fatalf("case 2: expected commits_after_merged_pr, got %q (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("case 2: commits_after_merged_pr must not be deletable")
	}
}

// ─── D1 Case 2b: tip and PR head mutually diverged → keep ────────────────────────────────────

// TestD1_Case2b_DivergedFromMerged_Keep asserts: when neither tip nor prHead is an ancestor of
// the other (e.g. rebase after merge), the decision is diverged_from_merged_pr (keep).
// Sabotage: make the case-2 is-ancestor call succeed unconditionally → test fails, decision
// becomes commits_after_merged_pr instead of diverged_from_merged_pr.
func TestD1_Case2b_DivergedFromMerged_Keep(t *testing.T) {
	tip := "dddd2b22"
	prHead := "dddd2b11"
	snapshot := makeSnapshotWith(
		forgePR{Number: 40, State: "MERGED", HeadRefName: "feat/w", HeadRefOid: prHead, BaseRefName: "main"},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "rev-parse feat/w":
			return tip, nil
		case key == "cat-file -e "+prHead+"^{commit}":
			return "", nil
		case key == "merge-base --is-ancestor "+tip+" "+prHead:
			return "", fmt.Errorf("git merge-base --is-ancestor %s %s exited with 1", tip, prHead)
		case key == "merge-base --is-ancestor "+prHead+" "+tip:
			return "", fmt.Errorf("git merge-base --is-ancestor %s %s exited with 1", prHead, tip)
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}
	eval := evaluateBranchWithForge("feat/w", "feat/w", "origin/feat/w", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionDivergedFromMerged {
		t.Fatalf("case 2b: expected diverged_from_merged_pr, got %q (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("case 2b: diverged_from_merged_pr must not be deletable")
	}
}

// ─── D1 Case 3: CLOSED PR → review ──────────────────────────────────────────────────────────

// TestD1_Case3_ClosedPR_Review asserts: a CLOSED (without merge) PR produces closed_pr (review,
// never delete), and isReviewDecision reports it as requiring review.
// Sabotage: remove the "CLOSED" case from the evaluateBranchWithForge switch → test fails with
// content-heuristic decision (pending_work or no_own_work) depending on gitExec.
func TestD1_Case3_ClosedPR_Review(t *testing.T) {
	snapshot := makeSnapshotWith(
		forgePR{Number: 50, State: "CLOSED", HeadRefName: "feat/v", HeadRefOid: "eeee3333", BaseRefName: "main"},
	)
	// gitExec should not be called for PR-detected cases — strict stub.
	gitExec := func(args ...string) (string, error) {
		return "", fmt.Errorf("unexpected gitExec in case-3 test: %v", args)
	}
	eval := evaluateBranchWithForge("feat/v", "feat/v", "origin/feat/v", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionClosedPR {
		t.Fatalf("case 3: expected closed_pr, got %q (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("case 3: closed_pr must not be deletable")
	}
	if !isReviewDecision(eval.Decision) {
		t.Fatal("case 3: closed_pr must be a review decision")
	}
}

// ─── D1 Case 4: no PR, no upstream → keep (never pushed) ────────────────────────────────────

// TestD1_Case4_NoPR_NeverPushed_Keep asserts: when the snapshot has no PR for this branch and
// the upstream is empty (never pushed), the decision is no_pr_never_pushed (keep, not deletable).
// Sabotage: change the upstream check so an empty upstream does not trigger case 4 → test falls
// through to the content heuristic and may return no_own_work (deletable) for an empty diff.
func TestD1_Case4_NoPR_NeverPushed_Keep(t *testing.T) {
	snapshot := makeSnapshotWith() // no PRs at all
	// gitExec would be called for the content heuristic fallback IF case 4 doesn't fire.
	// Make it return "no own work" so the test can detect if case 4 was skipped.
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "merge-base origin/main feat/unpushed":
			return "fff04444", nil
		case strings.HasPrefix(key, "diff --name-only -z fff04444"):
			return "", nil // no own work → would be deletable if case 4 skipped
		}
		return "", fmt.Errorf("unexpected gitExec in case-4 test: %v", args)
	}
	eval := evaluateBranchWithForge("feat/unpushed", "feat/unpushed", "", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionNoPRNeverPushed {
		t.Fatalf("case 4: expected no_pr_never_pushed, got %q (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("case 4: no_pr_never_pushed must not be deletable")
	}
}

// ─── D1 Case 5: no PR, with upstream → content heuristic ─────────────────────────────────────

// TestD1_Case5_NoPR_WithUpstream_ContentHeuristic asserts: when no PR is found but an upstream
// exists (branch was pushed), evaluateBranchWithForge falls back to evaluateBranchIntegration
// and the content-heuristic decision is used.
// Sabotage: route all "no PR with upstream" cases to no_pr_never_pushed (ignoring upstream) →
// test fails because the branch returns no_pr_never_pushed instead of the heuristic result.
func TestD1_Case5_NoPR_WithUpstream_ContentHeuristic(t *testing.T) {
	snapshot := makeSnapshotWith() // no PRs
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "merge-base origin/main feat/pushed-noPR":
			return "fff05555", nil
		case key == "diff --name-only -z fff05555 feat/pushed-noPR":
			return "", nil // no own work → content_identical / no_own_work → deletable
		}
		return "", fmt.Errorf("unexpected gitExec in case-5 test: %v", args)
	}
	eval := evaluateBranchWithForge("feat/pushed-noPR", "feat/pushed-noPR", "origin/feat/pushed-noPR", snapshot, gitExec)
	// Content heuristic should fire and return a deletable result.
	if eval.Decision == branchPruneDecisionNoPRNeverPushed {
		t.Fatal("case 5: with a non-empty upstream, must NOT produce no_pr_never_pushed — must use content heuristic")
	}
	// no_own_work is the content-heuristic deletable result for an empty diff.
	if eval.Decision != branchPruneDecisionNoOwnWork {
		t.Fatalf("case 5: expected no_own_work (content heuristic), got %q (%s)", eval.Decision, eval.Reason)
	}
}

// ─── AC3: fork MERGED PR with same head never produces delete ────────────────────────────────

// TestAC3_ForkPR_MERGED_NeverDelete asserts: a MERGED PR marked isCrossRepository=true is
// filtered out and does NOT produce merged_pr (delete), even when the is-ancestor check would
// otherwise succeed if the fork were treated as a non-fork PR.
// Sabotage: remove the isCrossRepository filter → rev-parse returns prHead, cat-file succeeds,
// is-ancestor returns exit 0, result becomes merged_pr (delete). Test fails.
// Content heuristic stubs return pending_work (not deletable) so that the test ONLY passes
// when the fork filter is active AND short-circuits the is-ancestor path.
func TestAC3_ForkPR_MERGED_NeverDelete(t *testing.T) {
	prHead := "fork1111"
	snapshot := makeSnapshotWith(
		// Fork PR — must be excluded.
		forgePR{Number: 60, State: "MERGED", HeadRefName: "feat/fork", HeadRefOid: prHead, BaseRefName: "main", IsCrossRepository: true},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		// Stubs for the is-ancestor path (only reached if fork filter is removed).
		case key == "rev-parse feat/fork":
			return prHead, nil // tip == prHead → case 1 delete if fork filter absent
		case key == "cat-file -e " + prHead + "^{commit}":
			return "", nil
		case key == "merge-base --is-ancestor " + prHead + " " + prHead:
			return "", nil // exit 0 → case 1 if fork filter absent

		// Stubs for the content heuristic (reached when fork filter fires correctly).
		case key == "merge-base origin/main feat/fork":
			return "base0000", nil
		case key == "diff --name-only -z base0000 feat/fork":
			return "real.go\x00", nil
		case key == "diff --name-only -z origin/main feat/fork -- real.go":
			return "real.go\x00", nil // pending_work — not deletable
		}
		return "", fmt.Errorf("unexpected gitExec in AC3 test: %v", args)
	}
	eval := evaluateBranchWithForge("feat/fork", "feat/fork", "origin/feat/fork", snapshot, gitExec)
	if eval.Decision == branchPruneDecisionMergedPR {
		t.Fatal("AC3: fork PR must not produce merged_pr (delete) — isCrossRepository filter must exclude it")
	}
	if eval.Decision.deletable() {
		t.Fatalf("AC3: result must not be deletable, got %q (%s)", eval.Decision, eval.Reason)
	}
}

// ─── A3: cat-file before is-ancestor ─────────────────────────────────────────────────────────

// TestA3_HeadAbsent_NoCatFile_ReviewNotDelete asserts: when the PR head commit is absent from
// the local object store (cat-file -e returns an error), the decision is merged_head_absent
// (review), NOT merged_pr (delete). This is the A3 adjustment (ADR-2026-10-03).
// Sabotage: skip the cat-file check and call is-ancestor directly for missing objects → if
// is-ancestor returns exit 128 (fatal), the code may misparse it as "not ancestor" → case 2b
// (diverged, keep) instead of merged_head_absent (review).
func TestA3_HeadAbsent_NoCatFile_ReviewNotDelete(t *testing.T) {
	const a3Tip = "a3tip111"
	prHead := "a3prXXXX" // not available locally
	snapshot := makeSnapshotWith(
		forgePR{Number: 70, State: "MERGED", HeadRefName: "feat/a3", HeadRefOid: prHead, BaseRefName: "main"},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "rev-parse feat/a3":
			return a3Tip, nil
		case key == "cat-file -e "+prHead+"^{commit}":
			// Object not in local store.
			return "", fmt.Errorf("git cat-file -e %s^{commit} exited with 1", prHead)
		// is-ancestor must NOT be called for an absent object.
		case strings.HasPrefix(key, "merge-base --is-ancestor"):
			t.Errorf("A3: is-ancestor called for a head object that failed cat-file — must check object first")
			return "", fmt.Errorf("unexpected call")
		}
		return "", fmt.Errorf("unexpected gitExec in A3 test: %v", args)
	}
	eval := evaluateBranchWithForge("feat/a3", "feat/a3", "origin/feat/a3", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionMergedHeadAbsent {
		t.Fatalf("A3: expected merged_head_absent, got %q (%s)", eval.Decision, eval.Reason)
	}
	if eval.Decision.deletable() {
		t.Fatal("A3: merged_head_absent must not be deletable")
	}
	if !isReviewDecision(eval.Decision) {
		t.Fatal("A3: merged_head_absent must be a review decision")
	}
}

// ─── A2: base ≠ main → not treated as case-1 delete ─────────────────────────────────────────

// TestA2_MergedPR_WrongBase_NotDelete asserts: a MERGED PR with baseRefName != main (e.g. a
// stacked PR into another branch) does NOT produce merged_pr (delete).
// Sabotage: remove the BaseRefName == branchPruneDefaultLocalName check → rev-parse succeeds,
// cat-file succeeds, is-ancestor returns exit 0, result becomes merged_pr (delete). Test fails.
// Content heuristic stubs return pending_work (not deletable) so the test only passes when the
// base filter routes around the is-ancestor path.
func TestA2_MergedPR_WrongBase_NotDelete(t *testing.T) {
	prHead := "a2tip222" // same as tip — would be a case-1 delete without the filter
	snapshot := makeSnapshotWith(
		forgePR{Number: 80, State: "MERGED", HeadRefName: "feat/stacked", HeadRefOid: prHead, BaseRefName: "feat/base"},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		// Stubs for is-ancestor path (reached only if base filter is removed).
		case key == "rev-parse feat/stacked":
			return prHead, nil
		case key == "cat-file -e " + prHead + "^{commit}":
			return "", nil
		case key == "merge-base --is-ancestor " + prHead + " " + prHead:
			return "", nil // exit 0 → case 1 delete if base filter absent

		// Stubs for the content heuristic (reached when base filter fires correctly).
		case key == "merge-base origin/main feat/stacked":
			return "base2222", nil
		case key == "diff --name-only -z base2222 feat/stacked":
			return "f1.go\x00", nil
		case key == "diff --name-only -z origin/main feat/stacked -- f1.go":
			return "f1.go\x00", nil // pending_work — not deletable
		}
		return "", fmt.Errorf("unexpected gitExec in A2 test: %v", args)
	}
	eval := evaluateBranchWithForge("feat/stacked", "feat/stacked", "origin/feat/stacked", snapshot, gitExec)
	if eval.Decision == branchPruneDecisionMergedPR {
		t.Fatal("A2: stacked PR (base != main) must not produce merged_pr (delete) — baseRefName filter must exclude it")
	}
	if eval.Decision.deletable() {
		t.Fatalf("A2: result must not be deletable, got %q (%s)", eval.Decision, eval.Reason)
	}
}

// ─── AC5: ghExec called exactly once for N branches ──────────────────────────────────────────

// TestAC5_ForgeQueriedExactlyOnce asserts: for a prune run over N branches, the ghExecFn is
// invoked exactly once (D3, single query, ADR-2026-10-03). Invoking it more than once would
// multiply network calls by the branch count, violating D3.
// Sabotage: move the queryForgePRs call inside the per-branch loop in runBranchPrune → test
// fails with callCount > 1.
func TestAC5_ForgeQueriedExactlyOnce(t *testing.T) {
	callCount := 0

	ghExec := func(args ...string) ([]byte, error) {
		callCount++
		return jsonPRs([]forgePR{}), nil // empty — no PRs
	}

	// gitExec handles: fetch, rev-parse, for-each-ref (upstream), and content heuristic for
	// two non-excluded branches.
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "fetch origin --prune":
			return "", nil
		case key == "rev-parse --verify -q origin/main":
			return "ac5main1", nil
		case key == "remote get-url origin":
			return "git@github.com:owner/repo.git", nil
		case strings.HasPrefix(key, "for-each-ref --format=%(upstream:short)"):
			return "origin/feat/a", nil // non-empty → never case 4
		case key == "merge-base origin/main feat/a":
			return "ac5mba1", nil
		case key == "diff --name-only -z ac5mba1 feat/a":
			return "", nil // no own work
		case key == "merge-base origin/main feat/b":
			return "ac5mbb1", nil
		case key == "diff --name-only -z ac5mbb1 feat/b":
			return "file.go\x00", nil
		case key == "diff --name-only -z origin/main feat/b -- file.go":
			return "file.go\x00", nil // pending
		}
		return "", fmt.Errorf("unexpected gitExec in AC5 test: %v", args)
	}

	out := &strings.Builder{}
	deps := branchPruneDeps{
		gitExec: gitExec,
		ghExec:  ghExec,
		listLocalBranches: func(func(args ...string) (string, error)) ([]string, error) {
			// Two non-excluded branches.
			return []string{"main", "feat/a", "feat/b"}, nil
		},
		currentBranch:    func(func(args ...string) (string, error)) string { return "main" },
		worktreeBranches: func(func(args ...string) (string, error)) map[string]bool { return nil },
		deleteBranch:     func(func(args ...string) (string, error), string) error { return nil },
		out:              &writerFromBuilder{out},
	}

	_ = runBranchPrune(false, deps)

	if callCount != 1 {
		t.Fatalf("AC5: ghExecFn must be called exactly once for any number of branches; called %d times", callCount)
	}
}

// writerFromBuilder satisfies io.Writer using a strings.Builder.
type writerFromBuilder struct{ b *strings.Builder }

func (w *writerFromBuilder) Write(p []byte) (int, error) { return w.b.Write(p) }

// ─── AC5 corollary: truncated response does not produce "no PR" deletes ──────────────────────

// TestAC5_TruncatedResponse_BlocksForgeSignalDeletes asserts: when ghExec returns exactly
// forgeQueryLimit PRs, queryForgePRs treats the response as truncated (A7) and returns nil,
// preventing any forge-signal delete for that run.
// Sabotage: change the truncation check from `== forgeQueryLimit` to `> forgeQueryLimit` →
// test fails because the truncated response is treated as complete and may produce deletes.
func TestAC5_TruncatedResponse_BlocksForgeSignalDeletes(t *testing.T) {
	// Build exactly forgeQueryLimit PRs.
	prs := make([]forgePR, forgeQueryLimit)
	for i := range prs {
		prs[i] = forgePR{Number: i + 1, State: "MERGED", HeadRefName: fmt.Sprintf("feat/branch-%d", i), BaseRefName: "main"}
	}

	ghExec := stubGhExec(jsonPRs(prs))
	gitExec := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "remote get-url origin" {
			return "git@github.com:owner/repo.git", nil
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}

	snapshot, reason := queryForgePRs(gitExec, ghExec)
	if snapshot != nil {
		t.Fatal("AC5 truncation: snapshot must be nil when response length == forgeQueryLimit")
	}
	if !strings.Contains(reason, "truncated") && !strings.Contains(reason, "limit") {
		t.Fatalf("AC5 truncation: reason must mention truncation/limit, got %q", reason)
	}
}

// ─── A1: --repo flag prevents GH_REPO misdirection ──────────────────────────────────────────

// TestA1_RepoFlagUsed asserts: queryForgePRs passes "--repo HOST/OWNER/REPO" derived from the
// git remote URL (not from GH_REPO or other environment), so a hostile GH_REPO cannot redirect
// the query to a different repository.
// Sabotage: remove the "--repo" argument from the ghExec call in queryForgePRs → test fails
// because the args received by the stub do not contain "--repo".
func TestA1_RepoFlagUsed(t *testing.T) {
	var receivedArgs []string
	ghExec := func(args ...string) ([]byte, error) {
		receivedArgs = append(receivedArgs, args...)
		return jsonPRs([]forgePR{}), nil
	}
	gitExec := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "remote get-url origin" {
			return "git@github.com:testowner/testrepo.git", nil
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}

	snapshot, reason := queryForgePRs(gitExec, ghExec)
	if snapshot == nil {
		t.Fatalf("A1: expected non-nil snapshot, got degraded: %q", reason)
	}

	// "--repo" must appear immediately before "testowner/testrepo" (with "github.com/" prefix).
	foundRepo := false
	for i, arg := range receivedArgs {
		if arg == "--repo" && i+1 < len(receivedArgs) && strings.HasSuffix(receivedArgs[i+1], "testowner/testrepo") {
			foundRepo = true
			break
		}
	}
	if !foundRepo {
		t.Fatalf("A1: --repo github.com/testowner/testrepo not found in gh args: %v", receivedArgs)
	}

	// A8: --limit 3000 must be in the args.
	foundLimit := false
	for i, arg := range receivedArgs {
		if arg == "--limit" && i+1 < len(receivedArgs) && receivedArgs[i+1] == fmt.Sprintf("%d", forgeQueryLimit) {
			foundLimit = true
			break
		}
	}
	if !foundLimit {
		t.Fatalf("A8: --limit %d not found in gh args: %v", forgeQueryLimit, receivedArgs)
	}
}

// ─── A1 corollary: non-GitHub host → degrade ─────────────────────────────────────────────────

// TestA1_NonGitHubHost_Degrades asserts: when the git remote URL is a non-GitHub host (e.g.
// gitlab.example.com), queryForgePRs returns nil (D2 degradation) without calling ghExec.
// Sabotage: remove the host == "github.com" check → test fails because ghExec is called.
func TestA1_NonGitHubHost_Degrades(t *testing.T) {
	called := false
	ghExec := func(args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	gitExec := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "remote get-url origin" {
			return "git@gitlab.example.com:owner/repo.git", nil
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}

	snapshot, reason := queryForgePRs(gitExec, ghExec)
	if snapshot != nil {
		t.Fatal("A1 non-GitHub: snapshot must be nil for non-GitHub host")
	}
	if called {
		t.Fatal("A1 non-GitHub: ghExec must not be called when host != github.com")
	}
	if !strings.Contains(reason, "github.com") {
		t.Fatalf("A1 non-GitHub: reason should mention github.com, got %q", reason)
	}
}

// ─── D2: nil ghExec → content heuristic ──────────────────────────────────────────────────────

// TestD2_NilGhExec_ContentHeuristic asserts: when ghExec is nil, evaluateBranchWithForge falls
// back to evaluateBranchIntegration and produces the same result as calling the heuristic
// directly.
// Sabotage: make nil ghExec call a panic stub → test panics (would fail at the call site).
func TestD2_NilGhExec_ContentHeuristic(t *testing.T) {
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "merge-base origin/main feat/degraded":
			return "d2base11", nil
		case key == "diff --name-only -z d2base11 feat/degraded":
			return "g.go\x00", nil
		case key == "diff --name-only -z origin/main feat/degraded -- g.go":
			return "g.go\x00", nil
		}
		return "", fmt.Errorf("unexpected gitExec in D2 test: %v", args)
	}
	// nil snapshot = degraded.
	eval := evaluateBranchWithForge("feat/degraded", "feat/degraded", "origin/feat/degraded", nil, gitExec)
	// Must fall back to evaluateBranchIntegration — same result as calling it directly.
	direct := evaluateBranchIntegration("feat/degraded", gitExec)
	if eval.Decision != direct.Decision {
		t.Fatalf("D2: with nil snapshot, evaluateBranchWithForge must return same decision as evaluateBranchIntegration; got %q vs %q", eval.Decision, direct.Decision)
	}
}

// ─── A5: gh exit ≠ 0 → degrade ───────────────────────────────────────────────────────────────

// TestA5_GhExitNonZero_Degrades asserts: when the ghExecFn returns an error (simulating
// gh exit ≠ 0), queryForgePRs returns nil (D2) instead of treating the output as an empty list.
// Sabotage: ignore the error return from ghExec and try to parse the (empty/nil) output as JSON
// → test may fail because nil snapshot is expected but a non-nil (empty) snapshot is returned.
func TestA5_GhExitNonZero_Degrades(t *testing.T) {
	ghExec := func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh: authentication required")
	}
	gitExec := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "remote get-url origin" {
			return "git@github.com:owner/repo.git", nil
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}

	snapshot, reason := queryForgePRs(gitExec, ghExec)
	if snapshot != nil {
		t.Fatal("A5: gh exit ≠ 0 must produce nil snapshot (D2 degradation), not empty snapshot")
	}
	if !strings.Contains(reason, "failed") && !strings.Contains(reason, "authentication") {
		t.Fatalf("A5: reason must describe the failure, got %q", reason)
	}
}

// ─── parseHostOwnerRepo tests ─────────────────────────────────────────────────────────────────

// TestParseHostOwnerRepo asserts: parseHostOwnerRepo correctly extracts (host, owner/repo) from
// SSH git@, SSH ssh://, and HTTPS URL formats, and returns ("","") for unrecognised inputs.
// Sabotage: swap the owner and repo parts in the output → test fails with wrong ownerRepo value.
func TestParseHostOwnerRepo(t *testing.T) {
	cases := []struct {
		rawURL    string
		wantHost  string
		wantOwner string
	}{
		{"git@github.com:owner/repo.git", "github.com", "owner/repo"},
		{"git@github.com:owner/repo", "github.com", "owner/repo"},
		{"https://github.com/owner/repo.git", "github.com", "owner/repo"},
		{"https://github.com/owner/repo", "github.com", "owner/repo"},
		{"http://github.com/owner/repo.git", "github.com", "owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "github.com", "owner/repo"},
		{"git@gitlab.example.com:org/project.git", "gitlab.example.com", "org/project"},
		{"", "", ""},
		{"not-a-url", "", ""},
		{"git@github.com:", "", ""},      // no path
		{"https://github.com/onlyone", "", ""}, // only one path component
	}
	for _, c := range cases {
		host, owner := parseHostOwnerRepo(c.rawURL)
		if host != c.wantHost || owner != c.wantOwner {
			t.Errorf("parseHostOwnerRepo(%q): got (%q, %q), want (%q, %q)", c.rawURL, host, owner, c.wantHost, c.wantOwner)
		}
	}
}

// ─── isNotAncestorError tests ─────────────────────────────────────────────────────────────────

// TestIsNotAncestorError asserts: isNotAncestorError returns true ONLY for errors whose message
// ends with "exited with 1" (exit-1 sentinel). It must return false for "exited with 128" — the
// message "git ... exited with 128" contains "exited with 1" as a substring, so Contains would
// give a false positive. HasSuffix is the correct check. Sabotage: change HasSuffix to Contains
// → test fails because exitWith128 is now (incorrectly) classified as "not ancestor" (A3).
func TestIsNotAncestorError(t *testing.T) {
	exitWith1 := fmt.Errorf("git merge-base --is-ancestor abc def exited with 1")
	// "exited with 128" contains "exited with 1" as a substring — must NOT match.
	exitWith128 := fmt.Errorf("git merge-base --is-ancestor abc def exited with 128")
	exitFatal := fmt.Errorf("fatal: not a valid object name: 'abc'")

	if !isNotAncestorError(exitWith1) {
		t.Error("isNotAncestorError: must return true for 'exited with 1'")
	}
	if isNotAncestorError(exitWith128) {
		t.Error("isNotAncestorError: must return false for 'exited with 128' — '128' contains '1' as substring; HasSuffix must be used")
	}
	if isNotAncestorError(exitFatal) {
		t.Error("isNotAncestorError: must return false for fatal errors without exit code")
	}
	if isNotAncestorError(nil) {
		t.Error("isNotAncestorError: must return false for nil")
	}
}

// ─── Real git repo integration: D1 cases with real objects ───────────────────────────────────

// TestD1_RealGit_Case1_And_Case2 creates a real temporary git repository and exercises
// evaluateBranchWithForge with a fake ghExec that returns pre-baked JSON, so no network call is
// made but the is-ancestor logic runs against real git objects.
//
// This test asserts two things:
//  1. A branch whose tip is contained in a "MERGED" PR head is classified as merged_pr (delete).
//  2. A branch with commits beyond the "MERGED" PR head is classified as
//     commits_after_merged_pr (keep).
//
// Sabotage: invert the is-ancestor argument order in the case-1 check → both branches get
// commits_after_merged_pr instead of one getting merged_pr.
func TestD1_RealGit_Case1_And_Case2(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	runSilent := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	// Bootstrap: init, configure, initial commit on main.
	runSilent("init", "-b", "main")
	runSilent("config", "user.email", "test@test.com")
	runSilent("config", "user.name", "Test")
	// Create a remote named origin pointing to itself (needed for merge-base origin/main).
	runSilent("remote", "add", "origin", dir)
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base"), 0644); err != nil {
		t.Fatal(err)
	}
	runSilent("add", ".")
	runSilent("commit", "-m", "initial")
	mainOid := run("rev-parse", "HEAD")

	// Branch feat/integrated: one commit, which will be the "PR head".
	runSilent("checkout", "-b", "feat/integrated")
	if err := os.WriteFile(filepath.Join(dir, "integrated.txt"), []byte("integrated"), 0644); err != nil {
		t.Fatal(err)
	}
	runSilent("add", ".")
	runSilent("commit", "-m", "feat: integrated")
	integratedOid := run("rev-parse", "HEAD")

	// Branch feat/with-extra: same starting point, plus extra commit beyond PR head.
	runSilent("checkout", "-b", "feat/with-extra", mainOid)
	if err := os.WriteFile(filepath.Join(dir, "pr-commit.txt"), []byte("pr"), 0644); err != nil {
		t.Fatal(err)
	}
	runSilent("add", ".")
	runSilent("commit", "-m", "feat: PR commit")
	prHeadOid := run("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("extra"), 0644); err != nil {
		t.Fatal(err)
	}
	runSilent("add", ".")
	runSilent("commit", "-m", "feat: extra commit after PR head")
	extraTipOid := run("rev-parse", "HEAD")

	// We also need origin/main to exist as a ref; simulate by creating a fake remote ref.
	runSilent("update-ref", "refs/remotes/origin/main", mainOid)

	gitExec := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
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

	snapshot := makeSnapshotWith(
		// PR for feat/integrated: head == tip → case 1 (delete).
		forgePR{Number: 100, State: "MERGED", HeadRefName: "feat/integrated", HeadRefOid: integratedOid, BaseRefName: "main"},
		// PR for feat/with-extra: head is behind the tip → case 2 (keep).
		forgePR{Number: 101, State: "MERGED", HeadRefName: "feat/with-extra", HeadRefOid: prHeadOid, BaseRefName: "main"},
	)
	_ = extraTipOid // used implicitly as "tip" of feat/with-extra branch

	// Evaluate feat/integrated: expect merged_pr (delete).
	evalInt := evaluateBranchWithForge("feat/integrated", "feat/integrated", "origin/feat/integrated", snapshot, gitExec)
	if evalInt.Decision != branchPruneDecisionMergedPR {
		t.Fatalf("real git case 1: expected merged_pr, got %q (%s)", evalInt.Decision, evalInt.Reason)
	}
	if !evalInt.Decision.deletable() {
		t.Fatal("real git case 1: merged_pr must be deletable")
	}

	// Evaluate feat/with-extra: expect commits_after_merged_pr (keep).
	evalExtra := evaluateBranchWithForge("feat/with-extra", "feat/with-extra", "origin/feat/with-extra", snapshot, gitExec)
	if evalExtra.Decision != branchPruneDecisionCommitsAfterMerged {
		t.Fatalf("real git case 2: expected commits_after_merged_pr, got %q (%s)", evalExtra.Decision, evalExtra.Reason)
	}
	if evalExtra.Decision.deletable() {
		t.Fatal("real git case 2: commits_after_merged_pr must not be deletable")
	}
}

// ─── A4: upstream via for-each-ref, not @{u} ─────────────────────────────────────────────────

// TestA4_UpstreamFor_ForEachRef asserts: upstreamFor uses `for-each-ref --format=%(upstream:short)`
// and returns "" for a never-pushed branch (empty field) vs non-empty for a branch with upstream.
// Sabotage: replace `for-each-ref` with `rev-parse @{u}` → the [gone] branch case returns an
// error and is treated as "never pushed", but an actually-gone upstream would also fail, masking
// the distinction. Test fails because upstreamFor("gone-branch") returns "" even though the
// branch was pushed (its remote was just deleted).
//
// Note: this test uses the stub convention; the real `@{u}` behaviour is documented in A4.
func TestA4_UpstreamFor_ForEachRef(t *testing.T) {
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "for-each-ref --format=%(upstream:short) refs/heads/feat/pushed":
			return "origin/feat/pushed", nil // has upstream
		case key == "for-each-ref --format=%(upstream:short) refs/heads/feat/never-pushed":
			return "", nil // empty → never pushed
		case key == "for-each-ref --format=%(upstream:short) refs/heads/feat/gone":
			// [gone] upstream: for-each-ref returns empty string (not an error, A4).
			return "", nil
		// Reject @{u} calls — would indicate wrong implementation.
		case strings.Contains(key, "@{u}"):
			return "", fmt.Errorf("A4: rev-parse @{u} must not be used — use for-each-ref")
		}
		return "", fmt.Errorf("unexpected gitExec in A4 test: %v", args)
	}

	if upstream := upstreamFor("feat/pushed", gitExec); upstream == "" {
		t.Error("A4: upstreamFor must return non-empty for a branch with upstream")
	}
	if upstream := upstreamFor("feat/never-pushed", gitExec); upstream != "" {
		t.Errorf("A4: upstreamFor must return empty for a never-pushed branch, got %q", upstream)
	}
	if upstream := upstreamFor("feat/gone", gitExec); upstream != "" {
		t.Errorf("A4: upstreamFor must return empty for a [gone] branch (for-each-ref returns empty, not error), got %q", upstream)
	}
}

// ─── A5 corollary: invalid JSON → degrade ─────────────────────────────────────────────────────

// TestA5_InvalidJSON_Degrades asserts: when the ghExecFn returns exit 0 but invalid/partial JSON,
// queryForgePRs returns nil (D2 degradation) instead of silently treating it as empty list.
// Sabotage: change the json.Unmarshal error branch to return an empty snapshot → test fails
// because snapshot is non-nil when JSON is broken.
func TestA5_InvalidJSON_Degrades(t *testing.T) {
	ghExec := func(args ...string) ([]byte, error) {
		return []byte(`[{"number":1,"state":"MERGED","headRefName":"feat/x"`), nil // truncated JSON
	}
	gitExec := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "remote get-url origin" {
			return "git@github.com:owner/repo.git", nil
		}
		return "", fmt.Errorf("unexpected gitExec: %v", args)
	}

	snapshot, reason := queryForgePRs(gitExec, ghExec)
	if snapshot != nil {
		t.Fatal("A5 invalid JSON: snapshot must be nil — invalid JSON is D2 degradation, not an empty list")
	}
	if !strings.Contains(reason, "parse") && !strings.Contains(reason, "JSON") && !strings.Contains(reason, "json") {
		t.Fatalf("A5 invalid JSON: reason must describe parse failure, got %q", reason)
	}
}

// ─── D1§1: multiple MERGED PRs — first one diverged, second contains tip → delete ─────────────

// TestD1_MultiMerged_SecondContainsTip_Delete asserts: when there are multiple MERGED PRs for
// the same branch and the first is diverged (case 2b) but the second contains the tip (case 1),
// the result is merged_pr (delete). One PR containing the tip is enough (D1 §1).
// Sabotage: change the case-1 return to `continue` (accumulate instead of return immediately) →
// the best-outcome logic might pick diverged_from_merged_pr instead. Test fails.
func TestD1_MultiMerged_SecondContainsTip_Delete(t *testing.T) {
	tip := "multi1tip"
	pr1Head := "multipr1hd" // diverged from tip
	pr2Head := "multi1tip"  // == tip → case 1
	snapshot := makeSnapshotWith(
		forgePR{Number: 110, State: "MERGED", HeadRefName: "feat/multi", HeadRefOid: pr1Head, BaseRefName: "main"},
		forgePR{Number: 111, State: "MERGED", HeadRefName: "feat/multi", HeadRefOid: pr2Head, BaseRefName: "main"},
	)
	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "rev-parse feat/multi":
			return tip, nil
		case key == "cat-file -e " + pr1Head + "^{commit}":
			return "", nil
		case key == "cat-file -e " + pr2Head + "^{commit}":
			return "", nil
		// PR1: tip is NOT ≤ pr1Head, AND pr1Head is NOT ≤ tip → diverged (case 2b).
		case key == "merge-base --is-ancestor " + tip + " " + pr1Head:
			return "", fmt.Errorf("git merge-base --is-ancestor %s %s exited with 1", tip, pr1Head)
		case key == "merge-base --is-ancestor " + pr1Head + " " + tip:
			return "", fmt.Errorf("git merge-base --is-ancestor %s %s exited with 1", pr1Head, tip)
		// PR2: tip == pr2Head → is-ancestor returns exit 0 (case 1 → delete immediately).
		case key == "merge-base --is-ancestor " + tip + " " + pr2Head:
			return "", nil // exit 0 → case 1
		}
		return "", fmt.Errorf("unexpected gitExec in multi-merged test: %v", args)
	}
	eval := evaluateBranchWithForge("feat/multi", "feat/multi", "origin/feat/multi", snapshot, gitExec)
	if eval.Decision != branchPruneDecisionMergedPR {
		t.Fatalf("D1§1 multi-merged: expected merged_pr (second PR contains tip), got %q (%s)", eval.Decision, eval.Reason)
	}
	if !eval.Decision.deletable() {
		t.Fatal("D1§1 multi-merged: merged_pr must be deletable")
	}
}

// ─── AC4: degradation one-line cause printed in runBranchPrune ────────────────────────────────

// TestAC4_Degradation_CauseLineAndContentHeuristic asserts: when ghExec is non-nil but errors
// (D2 degradation), runBranchPrune prints one line naming the cause and uses the content
// heuristic for all branches — the results must match what nil-ghExec produces.
// Sabotage: remove the "Note: forge PR signal not available" print in runBranchPrune → test
// fails because the cause line is absent.
func TestAC4_Degradation_CauseLineAndContentHeuristic(t *testing.T) {
	// ghExec that always errors — simulates auth failure.
	ghExecErr := func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh: authentication required")
	}

	makeGit := func() func(args ...string) (string, error) {
		return func(args ...string) (string, error) {
			key := strings.Join(args, " ")
			switch {
			case key == "fetch origin --prune":
				return "", nil
			case key == "rev-parse --verify -q origin/main":
				return "ac4main00", nil
			case key == "remote get-url origin":
				return "git@github.com:owner/repo.git", nil
			case strings.HasPrefix(key, "for-each-ref --format=%(upstream:short)"):
				return "origin/feat/ac4", nil
			case key == "merge-base origin/main feat/ac4":
				return "ac4base00", nil
			case key == "diff --name-only -z ac4base00 feat/ac4":
				return "x.go\x00", nil
			case key == "diff --name-only -z origin/main feat/ac4 -- x.go":
				return "x.go\x00", nil // pending_work
			}
			return "", fmt.Errorf("unexpected gitExec in AC4 test: %v", args)
		}
	}

	makeDeps := func(ghExec ghExecFn, out *strings.Builder) branchPruneDeps {
		return branchPruneDeps{
			gitExec: makeGit(),
			ghExec:  ghExec,
			listLocalBranches: func(func(args ...string) (string, error)) ([]string, error) {
				return []string{"main", "feat/ac4"}, nil
			},
			currentBranch:    func(func(args ...string) (string, error)) string { return "main" },
			worktreeBranches: func(func(args ...string) (string, error)) map[string]bool { return nil },
			deleteBranch:     func(func(args ...string) (string, error), string) error { return nil },
			out:              &writerFromBuilder{out},
		}
	}

	// Run with erroring ghExec.
	outErr := &strings.Builder{}
	_ = runBranchPrune(false, makeDeps(ghExecErr, outErr))
	gotErr := outErr.String()

	// Run with nil ghExec (classic degraded mode).
	outNil := &strings.Builder{}
	_ = runBranchPrune(false, makeDeps(nil, outNil))
	gotNil := outNil.String()

	// AC4 assertion 1: degradation cause line must appear.
	if !strings.Contains(gotErr, "forge PR signal not available") {
		t.Fatalf("AC4: degradation cause line missing. Output:\n%s", gotErr)
	}
	// AC4 assertion 2: verdicts must match (content heuristic in both cases).
	// Both cases print a "Note:" line (different cause text). Strip Note: from BOTH before comparing
	// so only the branch verdict lines are compared.
	var errLines, nilLines []string
	for _, l := range strings.Split(gotErr, "\n") {
		if !strings.HasPrefix(l, "Note:") {
			errLines = append(errLines, l)
		}
	}
	for _, l := range strings.Split(gotNil, "\n") {
		if !strings.HasPrefix(l, "Note:") {
			nilLines = append(nilLines, l)
		}
	}
	if strings.Join(errLines, "\n") != strings.Join(nilLines, "\n") {
		t.Fatalf("AC4: verdicts differ between erroring-ghExec and nil-ghExec.\nWith error:\n%s\nWith nil:\n%s", gotErr, gotNil)
	}
}

// ─── runBranchPrune forge integration ────────────────────────────────────────────────────────

// TestRunBranchPrune_ForgeSignal_DeletesWithMergedPR asserts: when runBranchPrune has a non-nil
// forge snapshot and a branch has a MERGED PR containing its tip, it appears in the "delete" line.
// A branch with no PR and no upstream ("never pushed") must appear as keep (case 4), even though
// the content heuristic would return no_own_work (deletable).
// Sabotage: revert the inner branch evaluation call from evaluateBranchWithForge back to
// evaluateBranchIntegration → the MERGED-PR branch gets no_own_work (same result, "delete"),
// but the never-pushed branch gets no_own_work (also deletable!). Test fails: never-pushed branch
// appears in delete candidates.
func TestRunBranchPrune_ForgeSignal_DeletesWithMergedPR_KeepsNeverPushed(t *testing.T) {
	const mergedTip = "run1merged"
	const neverPushed = "feat/orphan"

	ghExec := func(args ...string) ([]byte, error) {
		prs := []forgePR{
			{Number: 300, State: "MERGED", HeadRefName: "feat/was-merged", HeadRefOid: mergedTip, BaseRefName: "main"},
		}
		b, _ := json.Marshal(prs)
		return b, nil
	}

	gitExec := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case key == "fetch origin --prune":
			return "", nil
		case key == "rev-parse --verify -q origin/main":
			return "runmain00", nil
		case key == "remote get-url origin":
			return "git@github.com:owner/repo.git", nil

		// feat/was-merged: case 1 (MERGED PR containing tip).
		case strings.HasPrefix(key, "for-each-ref --format=%(upstream:short) refs/heads/feat/was-merged"):
			return "origin/feat/was-merged", nil
		case key == "rev-parse feat/was-merged":
			return mergedTip, nil
		case key == "cat-file -e " + mergedTip + "^{commit}":
			return "", nil
		case key == "merge-base --is-ancestor " + mergedTip + " " + mergedTip:
			return "", nil // exit 0 → case 1

		// feat/orphan: no upstream (never pushed), no PR → case 4 (keep).
		// Content heuristic would return no_own_work (deletable) — must be prevented by case 4.
		case key == "for-each-ref --format=%(upstream:short) refs/heads/" + neverPushed:
			return "", nil // empty → never pushed
		// heuristic stubs for feat/orphan (reached only if case 4 is bypassed):
		case key == "merge-base origin/main " + neverPushed:
			return "orphbase0", nil
		case key == "diff --name-only -z orphbase0 " + neverPushed:
			return "", nil // no own work → no_own_work (deletable!) — must not reach here
		}
		return "", fmt.Errorf("unexpected gitExec in runBranchPrune forge test: %v", args)
	}

	out := &strings.Builder{}
	deletedBranches := []string{}
	deps := branchPruneDeps{
		gitExec: gitExec,
		ghExec:  ghExec,
		listLocalBranches: func(func(args ...string) (string, error)) ([]string, error) {
			return []string{"main", "feat/was-merged", neverPushed}, nil
		},
		currentBranch:    func(func(args ...string) (string, error)) string { return "main" },
		worktreeBranches: func(func(args ...string) (string, error)) map[string]bool { return nil },
		deleteBranch: func(_ func(args ...string) (string, error), name string) error {
			deletedBranches = append(deletedBranches, name)
			return nil
		},
		out: &writerFromBuilder{out},
	}

	// Run with --apply to actually trigger deletes.
	if err := runBranchPrune(true, deps); err != nil {
		t.Fatalf("runBranchPrune: unexpected error: %v", err)
	}
	got := out.String()

	// feat/was-merged must be in delete candidates (forge signal: merged_pr).
	if !strings.Contains(got, "delete") || !strings.Contains(got, "feat/was-merged") {
		t.Fatalf("runBranchPrune forge: feat/was-merged must be in delete. Output:\n%s", got)
	}
	// feat/orphan must NOT be deleted (case 4: never pushed).
	for _, d := range deletedBranches {
		if d == neverPushed {
			t.Fatalf("runBranchPrune forge: %q must never be deleted (never pushed, no PR)", neverPushed)
		}
	}
	if strings.Contains(got, "delete") && strings.Contains(got, neverPushed) {
		// "keep" or "no_pr_never_pushed" should appear for it, not "delete".
		for _, line := range strings.Split(got, "\n") {
			if strings.Contains(line, neverPushed) && strings.Contains(line, "delete") {
				t.Fatalf("runBranchPrune forge: %q appeared as delete: %q", neverPushed, line)
			}
		}
	}
}
