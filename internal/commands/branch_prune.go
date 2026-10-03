package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// branchPruneDefaultRemoteRef is the primary source of truth this command consults: the local
// tracking ref for the default branch. When the forge (GitHub) is reachable, a single `gh pr list`
// query provides the PR signal used in the D1 order (ADR-2026-10-03); when it is not, the command
// degrades to the content heuristic below and prints one line naming the cause (D2, same ADR —
// revokes the "no forge lookup" clause of REQ-2026-08-18 decision 2).
// If this ref cannot be resolved (no remote configured, or never fetched), the whole command
// refuses and deletes nothing.
const branchPruneDefaultRemoteRef = "origin/main"

// branchPruneDefaultLocalName is the local branch name matching branchPruneDefaultRemoteRef. It is
// always excluded as a prune candidate — evaluating it against itself would report "no own work"
// and offer to delete the branch the user is meant to keep. This is the highest-severity bug the
// naive heuristic contains (see docs/req/REQ-2026-08-18-...): merge-base origin/main main == the
// tip of main, so "touched" is trivially empty.
const branchPruneDefaultLocalName = "main"

// branchPruneDecision classifies why a branch was (or was not) offered for deletion. Kept as a
// distinct enum, not a bool, so the report is auditable and ML-2A (detectPendingSquashMerges) can
// map only branchPruneDecisionPendingWork to its existing warning.
type branchPruneDecision string

const (
	branchPruneDecisionDefaultBranch branchPruneDecision = "default_branch"
	branchPruneDecisionCurrentBranch branchPruneDecision = "current_branch"
	branchPruneDecisionWorktree      branchPruneDecision = "worktree_branch"
	branchPruneDecisionNoOwnWork     branchPruneDecision = "no_own_work"
	branchPruneDecisionIdentical     branchPruneDecision = "content_identical"
	branchPruneDecisionPendingWork   branchPruneDecision = "pending_work"
	branchPruneDecisionNoMergeBase   branchPruneDecision = "no_merge_base"
	branchPruneDecisionEvalError     branchPruneDecision = "eval_error"
	// branchPruneDecisionReviewDocConfig is a NON-deletable category, distinct from
	// pending_work: every diverging file is doc/config (isDocOrConfigPath), so it is probably
	// housekeeping residue from a squash-merge rather than genuine pending work — but it is
	// never auto-deleted. CLAUDE.md §1's own procedure treats this case as "housekeeping,
	// apagar" but with a human already in the loop reading the diff; a destructive command has
	// no human in the loop by construction, so REQ-2026-08-18 (ML-1B) deliberately narrows that
	// step to "flag for confirmation" instead of "delete automatically". See deletable() below.
	branchPruneDecisionReviewDocConfig branchPruneDecision = "review_doc_config"

	// PR-signal decisions (D1, ADR-2026-10-03). These are produced only when a forge snapshot
	// is available (queryForgePRs returned non-nil). On degradation (D2) or truncation (A7),
	// evaluateBranchWithForge falls back to evaluateBranchIntegration and no PR-signal decision
	// is produced.

	// branchPruneDecisionMergedPR is the forge-signal delete: a non-fork PR for this branch was
	// merged into the default branch and its head commit contains the local tip (D1 case 1).
	// This is the only new deletable category: see deletable() below.
	branchPruneDecisionMergedPR branchPruneDecision = "merged_pr"

	// branchPruneDecisionOpenPR means an OPEN, non-fork PR exists for this branch (D1 case 0).
	// It takes priority over any old MERGED PR with the same head name — defence against reused
	// branch names.
	branchPruneDecisionOpenPR branchPruneDecision = "open_pr"

	// branchPruneDecisionCommitsAfterMerged means a PR was merged but the local tip has commits
	// beyond the PR head (D1 case 2): pending work added after the merge.
	branchPruneDecisionCommitsAfterMerged branchPruneDecision = "commits_after_merged_pr"

	// branchPruneDecisionDivergedFromMerged means a PR was merged but the local tip and the PR
	// head are mutually diverged (D1 case 2b): e.g. rebase or amend after merge.
	branchPruneDecisionDivergedFromMerged branchPruneDecision = "diverged_from_merged_pr"

	// branchPruneDecisionMergedHeadAbsent means a PR was merged but the head commit is not
	// available in the local object store (A3): review, never auto-deleted.
	branchPruneDecisionMergedHeadAbsent branchPruneDecision = "merged_head_absent"

	// branchPruneDecisionClosedPR means the only PR found was closed without merge (D1 case 3):
	// review, never auto-deleted.
	branchPruneDecisionClosedPR branchPruneDecision = "closed_pr"

	// branchPruneDecisionNoPRNeverPushed means no PR was found and the branch has no upstream
	// tracking ref — it was never pushed (D1 case 4). 🔴 NEVER auto-deleted: it is the only
	// class where deleting loses work that exists in no other location.
	branchPruneDecisionNoPRNeverPushed branchPruneDecision = "no_pr_never_pushed"
)

// branchPruneDeletable reports whether decision, on its own, makes a branch a deletion candidate.
// no_own_work (squash-merge with no ancestry — the git branch -d false negative) and
// content_identical (defasada porém integrada — the naive git diff false positive) are the
// content-heuristic deletable decisions. merged_pr is the new forge-signal deletable decision:
// a non-fork PR was merged into the default branch and the merged head contains the local tip.
// Every other decision keeps the branch.
func (d branchPruneDecision) deletable() bool {
	return d == branchPruneDecisionNoOwnWork || d == branchPruneDecisionIdentical || d == branchPruneDecisionMergedPR
}

// isReviewDecision reports whether a branch should be flagged for manual review (never auto-deleted).
// Covers both the content-heuristic category (review_doc_config) and the new forge-signal
// categories where human review is required before acting (closed_pr, merged_head_absent).
func isReviewDecision(d branchPruneDecision) bool {
	return d == branchPruneDecisionReviewDocConfig ||
		d == branchPruneDecisionClosedPR ||
		d == branchPruneDecisionMergedHeadAbsent
}

// branchPruneEvaluation is the per-branch outcome of evaluateBranchIntegration.
type branchPruneEvaluation struct {
	Name     string
	Decision branchPruneDecision
	Reason   string
	// Touched/Diverged are populated only for no_own_work/content_identical/pending_work — the
	// paths behind the reason string, kept structured for callers that want them (e.g. tests).
	Touched  []string
	Diverged []string
}

// evaluateBranchIntegration decides whether branch is safe to delete relative to
// branchPruneDefaultRemoteRef, using the touched-files heuristic documented in CLAUDE.md §1 and
// REQ-2026-08-18 — NOT the naive bidirectional `git diff origin/main <branch> --stat`, which is
// only correct when the branch is up to date with main (see detectPendingSquashMerges' known
// false positive, ship.go).
//
//	mb      = git merge-base origin/main <branch>
//	touched = git diff --name-only mb <branch>              (what the branch touched)
//	diverg  = git diff --name-only origin/main <branch> -- touched  (what still differs there)
//
// touched empty            -> no_own_work (deletable)      -- the squash-merge / -d false negative
// touched non-empty,
//
//	diverg empty            -> content_identical (deletable) -- the naive-diff false positive (stale
//	                                                             branch, main advanced by other PRs)
//	diverg non-empty        -> pending_work (kept, explained)
//
// This function is the single, shared implementation — ML-2A (detectPendingSquashMerges in
// ship.go) is expected to call it instead of its own bidirectional diff. It never deletes
// anything; it only decides and explains.
//
// gitExec runs `git <args...>` and returns trimmed stdout (production: defaultGitExec). Both
// git diff calls use -z (NUL-separated, unquoted paths) so filenames with spaces or non-ASCII
// bytes are never mis-split — the exact class of bug that would make a branch with pending work in
// "foo bar.md" read as an empty diverg and get deleted.
func evaluateBranchIntegration(branch string, gitExec func(...string) (string, error)) branchPruneEvaluation {
	mb, err := gitExec("merge-base", branchPruneDefaultRemoteRef, branch)
	mb = strings.TrimSpace(mb)
	if err != nil || mb == "" {
		return branchPruneEvaluation{
			Name:     branch,
			Decision: branchPruneDecisionNoMergeBase,
			Reason:   fmt.Sprintf("no merge-base with %s — refusing (unrelated history or bad ref)", branchPruneDefaultRemoteRef),
		}
	}

	touchedRaw, err := gitExec("diff", "--name-only", "-z", mb, branch)
	if err != nil {
		return branchPruneEvaluation{
			Name:     branch,
			Decision: branchPruneDecisionEvalError,
			Reason:   fmt.Sprintf("git diff --name-only -z %s %s failed: %v", mb, branch, err),
		}
	}
	touched := splitNulPaths(touchedRaw)

	if len(touched) == 0 {
		return branchPruneEvaluation{
			Name:     branch,
			Decision: branchPruneDecisionNoOwnWork,
			Reason:   fmt.Sprintf("no own work relative to %s — safe to delete", branchPruneDefaultRemoteRef),
		}
	}

	divergArgs := append([]string{"diff", "--name-only", "-z", branchPruneDefaultRemoteRef, branch, "--"}, touched...)
	divergRaw, err := gitExec(divergArgs...)
	if err != nil {
		return branchPruneEvaluation{
			Name:     branch,
			Decision: branchPruneDecisionEvalError,
			Reason:   fmt.Sprintf("git diff --name-only -z %s %s -- <touched> failed: %v", branchPruneDefaultRemoteRef, branch, err),
			Touched:  touched,
		}
	}
	diverg := splitNulPaths(divergRaw)

	if len(diverg) == 0 {
		return branchPruneEvaluation{
			Name:     branch,
			Decision: branchPruneDecisionIdentical,
			Reason:   fmt.Sprintf("squash-merged into %s — content identical in touched files, safe to delete", branchPruneDefaultRemoteRef),
			Touched:  touched,
		}
	}

	// review_doc_config requires diverg to be a PROPER subset of touched (len(diverg) <
	// len(touched)) — not just "all doc/config". diverg is always a subset of touched by
	// construction (the second git diff is scoped `-- touched`), so this is the same as: at
	// least one touched file is NOT in diverg, i.e. it made it into main. That is what
	// distinguishes genuine housekeeping residue from a squash-merge (some files integrated,
	// doc/config noise left behind) from a branch whose ENTIRE touched set — doc/config or
	// not — never reached main at all (diverg == touched): that is pending work, full stop,
	// regardless of file type. See "Auditoria do ML-1B" in the roadmap: a branch with brand-new,
	// never-merged docs/guia-novo.md previously fell into review_doc_config and was reported as
	// "probable housekeeping, confirm and delete manually" — wrong advice about real work.
	if len(diverg) < len(touched) && allDocOrConfig(diverg) {
		return branchPruneEvaluation{
			Name:     branch,
			Decision: branchPruneDecisionReviewDocConfig,
			Reason:   fmt.Sprintf("only doc/config files diverge from %s (%s) — probable housekeeping, confirm and delete manually", branchPruneDefaultRemoteRef, strings.Join(diverg, ", ")),
			Touched:  touched,
			Diverged: diverg,
		}
	}

	return branchPruneEvaluation{
		Name:     branch,
		Decision: branchPruneDecisionPendingWork,
		Reason:   fmt.Sprintf("pending work vs %s: %s", branchPruneDefaultRemoteRef, strings.Join(diverg, ", ")),
		Touched:  touched,
		Diverged: diverg,
	}
}

// branchPruneDocConfigExtensions are file extensions treated as "doc/config" by
// isDocOrConfigPath, for the review_doc_config classification only. Deliberately conservative and
// best-effort — misclassifying a file here never causes a deletion: it only changes which
// non-deletable category (review_doc_config vs pending_work) a kept branch is reported under. A
// false positive means a branch with real code changes is reported for manual review instead of
// "pending work"; a false negative means it stays "pending work". Both keep the branch either way.
var branchPruneDocConfigExtensions = []string{".yaml", ".yml", ".json", ".toml", ".ini", ".cfg"}

// branchPruneDocConfigBasenames are exact filenames (not paths) treated as "doc/config" by
// isDocOrConfigPath, in addition to branchPruneDocConfigExtensions. CLAUDE.md itself is the
// motivating example from REQ-2026-08-18 — already covered by isDocsFile's ".md" suffix check, not
// listed again here.
var branchPruneDocConfigBasenames = map[string]bool{
	".gitignore":     true,
	".gitattributes": true,
	".editorconfig":  true,
	"trackfw.yaml":   true,
	"LICENSE":        true,
}

// isDocOrConfigPath reports whether path is a doc file (isDocsFile: docs/, vault/, or .md — see
// commit.go) or a well-known non-runtime config file/extension. Used only to route an otherwise
// pending_work branch into the review_doc_config category — never to make anything deletable.
func isDocOrConfigPath(path string) bool {
	if isDocsFile(path) {
		return true
	}
	base := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		base = path[idx+1:]
	}
	if branchPruneDocConfigBasenames[base] {
		return true
	}
	for _, ext := range branchPruneDocConfigExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// allDocOrConfig reports whether every path in paths is doc/config (isDocOrConfigPath). Empty
// input returns false — there is nothing to classify as doc/config-only, and this function is only
// ever called with a non-empty diverg slice in practice.
func allDocOrConfig(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		if !isDocOrConfigPath(p) {
			return false
		}
	}
	return true
}

// splitNulPaths splits a NUL-separated `git diff --name-only -z` output into a sorted, non-empty
// path list. A trailing NUL (git always emits one after the last entry) produces one empty
// trailing element, which is dropped.
func splitNulPaths(raw string) []string {
	parts := strings.Split(raw, "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// branchPruneDeps holds injectable dependencies so runBranchPrune can be tested without touching a
// real git repository or a live forge.
type branchPruneDeps struct {
	// gitExec runs `git <args...>` and returns (trimmed-stdout, error). Production: defaultGitExec.
	gitExec func(args ...string) (string, error)
	// ghExec runs `gh <args...>` and returns raw stdout bytes. Production: defaultGhExec.
	// nil means "no forge CLI available" and forces D2 (content heuristic + one-line cause).
	// Tests pass nil or a stub that returns pre-baked JSON; the forge is never hit in tests.
	ghExec ghExecFn
	// listLocalBranches returns every local branch name. Production: git branch --format=%(refname:short).
	listLocalBranches func(gitExec func(args ...string) (string, error)) ([]string, error)
	// currentBranch returns the checked-out branch name, or "" on detached HEAD.
	currentBranch func(gitExec func(args ...string) (string, error)) string
	// worktreeBranches returns the set of branch names checked out in ANY worktree (including the
	// current one — current is also excluded separately/redundantly by design, belt-and-suspenders).
	worktreeBranches func(gitExec func(args ...string) (string, error)) map[string]bool
	// deleteBranch tries `git branch -d <name>` first, falling back to `git branch -D <name>`
	// only when -d refuses (production: real deletion; defaultDeleteBranch). -d refuses by
	// ancestry, which squash-merged branches never have — the expected case this command exists
	// to route around. Trying -d first costs nothing and, when it succeeds, confirms the
	// integration via git's own independent check too; all safety already lives in
	// evaluateBranchIntegration + the re-check immediately before calling this, not in which
	// flag ultimately performs the deletion.
	deleteBranch func(gitExec func(args ...string) (string, error), name string) error
	out          io.Writer
}

func newBranchPruneCmd() *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Report (and, with --apply, delete) local branches already integrated into origin/main",
		Long: `trackfw branch prune automates the "one active branch at a time" check documented in
CLAUDE.md §1 — it does not remove human judgment from every case: a branch whose only remaining
divergence is doc/config files is flagged for manual review, never deleted automatically.

A best-effort 'git fetch origin --prune' runs first. Failure (offline, no remote) is non-blocking:
a warning is printed and evaluation proceeds against the local origin/main ref, whatever its
state. A stale origin/main only ever makes the result MORE conservative — it can miss a branch
that was in fact integrated since the last fetch (reporting it kept when a fresh fetch would show
it deletable), but it never reports one as deletable that a fresh fetch would show as pending.

Decides integration with the touched-files heuristic, NOT git's own ancestry check (which always
refuses squash-merged branches) and NOT a naive bidirectional diff against origin/main (which
false-positives on a branch that is merged but stale, once main has advanced further):

  mb      = git merge-base origin/main <branch>
  touched = git diff --name-only mb <branch>                 (what the branch touched)
  diverg  = git diff --name-only origin/main <branch> -- touched  (what still differs there)

touched empty          -> integrated (safe to delete)
diverg empty           -> integrated (safe to delete) -- squash-merged, stale, main advanced since
diverg doc/config only -> flagged for review (kept; probable housekeeping, confirm and delete manually)
otherwise              -> kept, with the diverging files named

Every local branch is reported, always, with its decision and reason. The current branch, any
branch checked out in another worktree, and the default branch (main) are always kept and never
evaluated for deletion. Without origin/main resolvable at all (offline with no prior fetch ever
having run, or no remote configured), the whole command refuses and deletes nothing.

--dry-run is the default: without --apply, nothing is ever deleted, even the clearly integrated.
Deletion tries 'git branch -d' first — confirming the integration via git's own ancestry check
too, when possible — and falls back to 'git branch -D' only when -d refuses, the expected case for
squash-merged branches, which never have fast-forward ancestry with main.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			deps := branchPruneDeps{
				gitExec:           defaultGitExec,
				ghExec:            defaultGhExec,
				listLocalBranches: defaultListLocalBranches,
				currentBranch:     defaultCurrentBranchForPrune,
				worktreeBranches:  defaultWorktreeBranches,
				deleteBranch:      defaultDeleteBranch,
				out:               cmd.OutOrStdout(),
			}
			return runBranchPrune(apply, deps)
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "Actually delete branches decided as integrated (default: report only, delete nothing)")

	return cmd
}

// runBranchPrune implements `trackfw branch prune`. See newBranchPruneCmd's Long text for the
// full contract.
func runBranchPrune(apply bool, deps branchPruneDeps) error {
	// Best-effort `git fetch origin --prune`, per CLAUDE.md §1 step 1. Failure is non-blocking —
	// offline is a legitimate use case, the same posture `trackfw ship`'s squash-merge check
	// already takes (ship.go) — but unlike ship (which skips its check entirely on fetch
	// failure), evaluation below still proceeds against whatever origin/main ref is already
	// resolvable locally: a stale ref only ever makes the result MORE conservative (see the Long
	// help text), never less, so there is no safety reason to abort here.
	if _, err := deps.gitExec("fetch", "origin", "--prune"); err != nil {
		fmt.Fprintf(deps.out, "Warning: could not fetch origin (offline, no remote, or fetch failed) — evaluating with possibly stale data; a branch merged upstream since the last fetch may still be reported as pending.\n")
	}

	if _, err := deps.gitExec("rev-parse", "--verify", "-q", branchPruneDefaultRemoteRef); err != nil {
		fmt.Fprintf(deps.out, "trackfw branch prune: %s not found — offline, no remote configured, or never fetched. Refusing to evaluate any branch; nothing deleted.\n", branchPruneDefaultRemoteRef)
		return fmt.Errorf("branch prune: %s not resolvable", branchPruneDefaultRemoteRef)
	}

	branches, err := deps.listLocalBranches(deps.gitExec)
	if err != nil {
		fmt.Fprintf(deps.out, "trackfw branch prune: failed to list local branches: %v\n", err)
		return err
	}
	sort.Strings(branches)

	current := deps.currentBranch(deps.gitExec)
	worktreed := deps.worktreeBranches(deps.gitExec)

	// D3 (ADR-2026-10-03): one forge query for all branches. queryForgePRs returns nil on
	// degradation (D2) or truncation (A7) and prints one line explaining why; every branch then
	// falls back to the content heuristic. The nil check is enforced inside evaluateBranchWithForge.
	snapshot, degradeReason := queryForgePRs(deps.gitExec, deps.ghExec)
	if snapshot == nil {
		fmt.Fprintf(deps.out, "Note: forge PR signal not available — using content heuristic only. Cause: %s\n\n", degradeReason)
	}

	fmt.Fprintf(deps.out, "trackfw branch prune — evaluating %d local branch(es) against %s\n\n", len(branches), branchPruneDefaultRemoteRef)

	var toDelete []string
	var toReview []string
	for _, b := range branches {
		var eval branchPruneEvaluation
		switch {
		case b == branchPruneDefaultLocalName:
			eval = branchPruneEvaluation{Name: b, Decision: branchPruneDecisionDefaultBranch, Reason: "default branch — never pruned"}
		case b == current:
			eval = branchPruneEvaluation{Name: b, Decision: branchPruneDecisionCurrentBranch, Reason: "current branch — never pruned"}
		case worktreed[b]:
			eval = branchPruneEvaluation{Name: b, Decision: branchPruneDecisionWorktree, Reason: "checked out in another worktree — never pruned"}
		default:
			// A4 (ADR-2026-10-03): upstream via for-each-ref, not @{u}.
			// D1 case 4 needs the upstream to distinguish "never pushed" from "pushed, no PR".
			upstream := upstreamFor(b, deps.gitExec)
			eval = evaluateBranchWithForge(b, b, upstream, snapshot, deps.gitExec)
		}

		action := "keep"
		switch {
		case eval.Decision.deletable():
			action = "delete"
			toDelete = append(toDelete, b)
		case isReviewDecision(eval.Decision):
			action = "review"
			toReview = append(toReview, b)
		}
		fmt.Fprintf(deps.out, "  %-30s %-7s %s\n", eval.Name, action, eval.Reason)
	}

	fmt.Fprintln(deps.out)
	if len(toReview) > 0 {
		fmt.Fprintf(deps.out, "%d branch(es) need manual review (never auto-deleted): %s\n", len(toReview), strings.Join(toReview, ", "))
	}
	if !apply {
		if len(toDelete) == 0 {
			fmt.Fprintln(deps.out, "[dry-run] nothing to delete.")
		} else {
			fmt.Fprintf(deps.out, "[dry-run] would delete %d branch(es): %s. Rerun with --apply to delete.\n", len(toDelete), strings.Join(toDelete, ", "))
		}
		return nil
	}

	if len(toDelete) == 0 {
		fmt.Fprintln(deps.out, "nothing to delete.")
		return nil
	}

	var deleted []string
	for _, b := range toDelete {
		// Re-check current/worktree status immediately before each delete — belt-and-suspenders
		// against the branch changing state between the report above and this loop (e.g. another
		// process checking it out into a new worktree mid-run).
		if b == deps.currentBranch(deps.gitExec) {
			fmt.Fprintf(deps.out, "skip %s: became the current branch — refusing to delete\n", b)
			continue
		}
		if deps.worktreeBranches(deps.gitExec)[b] {
			fmt.Fprintf(deps.out, "skip %s: became checked out in a worktree — refusing to delete\n", b)
			continue
		}
		if err := deps.deleteBranch(deps.gitExec, b); err != nil {
			fmt.Fprintf(deps.out, "failed to delete %s: %v\n", b, err)
			continue
		}
		deleted = append(deleted, b)
	}

	if len(deleted) == 0 {
		fmt.Fprintln(deps.out, "deleted 0 branch(es).")
	} else {
		fmt.Fprintf(deps.out, "deleted %d branch(es): %s\n", len(deleted), strings.Join(deleted, ", "))
	}
	return nil
}

// defaultListLocalBranches runs `git branch --format=%(refname:short)` and returns one name per
// non-empty line.
func defaultListLocalBranches(gitExec func(args ...string) (string, error)) ([]string, error) {
	raw, err := gitExec("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// defaultCurrentBranchForPrune returns the checked-out branch's short name, or "" on detached
// HEAD (symbolic-ref fails there — that is not an error for this command's purposes, it just means
// there is no current branch to exclude by name).
func defaultCurrentBranchForPrune(gitExec func(args ...string) (string, error)) string {
	name, err := gitExec("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

// defaultWorktreeBranches parses `git worktree list --porcelain` and returns the set of branch
// short names checked out in any worktree. Uses the porcelain "branch refs/heads/<name>" line,
// not the human-readable format, per REQ-2026-08-18's explicit instruction.
func defaultWorktreeBranches(gitExec func(args ...string) (string, error)) map[string]bool {
	raw, err := gitExec("worktree", "list", "--porcelain")
	result := map[string]bool{}
	if err != nil {
		return result
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "branch refs/heads/"
		if strings.HasPrefix(line, prefix) {
			result[strings.TrimPrefix(line, prefix)] = true
		}
	}
	return result
}

// defaultDeleteBranch tries `git branch -d <name>` first. When the branch happens to have
// fast-forward ancestry with main too (a plain merge, not a squash), -d succeeds and confirms the
// integration via git's own independent check — no need for -D at all. It falls back to `git
// branch -D <name>` only when -d refuses, which is the expected outcome for squash-merged
// branches (no ancestry by construction): all safety already lives in evaluateBranchIntegration
// and the re-check immediately before this call, not in which flag performs the deletion.
func defaultDeleteBranch(gitExec func(args ...string) (string, error), name string) error {
	if _, err := gitExec("branch", "-d", name); err == nil {
		return nil
	}
	_, err := gitExec("branch", "-D", name)
	return err
}
