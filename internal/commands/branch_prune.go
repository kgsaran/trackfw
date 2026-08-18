package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// branchPruneDefaultRemoteRef is the only source of truth this command consults: the local
// tracking ref for the default branch. Per REQ-2026-08-18 decision 2, there is no forge lookup and
// no network call — offline and deterministic by construction. If this ref cannot be resolved
// (no remote configured, or never fetched), the whole command refuses and deletes nothing.
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
)

// branchPruneDeletable reports whether decision, on its own, makes a branch a deletion candidate.
// Both no_own_work (squash-merge with no ancestry — the git branch -d false negative) and
// content_identical (defasada porém integrada — the naive git diff false positive) are safe to
// delete; every other decision keeps the branch.
func (d branchPruneDecision) deletable() bool {
	return d == branchPruneDecisionNoOwnWork || d == branchPruneDecisionIdentical
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

	return branchPruneEvaluation{
		Name:     branch,
		Decision: branchPruneDecisionPendingWork,
		Reason:   fmt.Sprintf("pending work vs %s: %s", branchPruneDefaultRemoteRef, strings.Join(diverg, ", ")),
		Touched:  touched,
		Diverged: diverg,
	}
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
// real git repository.
type branchPruneDeps struct {
	// gitExec runs `git <args...>` and returns (trimmed-stdout, error). Production: defaultGitExec.
	gitExec func(args ...string) (string, error)
	// listLocalBranches returns every local branch name. Production: git branch --format=%(refname:short).
	listLocalBranches func(gitExec func(args ...string) (string, error)) ([]string, error)
	// currentBranch returns the checked-out branch name, or "" on detached HEAD.
	currentBranch func(gitExec func(args ...string) (string, error)) string
	// worktreeBranches returns the set of branch names checked out in ANY worktree (including the
	// current one — current is also excluded separately/redundantly by design, belt-and-suspenders).
	worktreeBranches func(gitExec func(args ...string) (string, error)) map[string]bool
	// deleteBranch runs `git branch -D <name>` (production: real deletion). -D, not -d: -d refuses
	// squash-merged branches by ancestry, which is exactly the false negative this command exists
	// to fix — all safety lives in evaluateBranchIntegration + the re-check immediately before
	// calling this, not in git's own ancestry check.
	deleteBranch func(gitExec func(args ...string) (string, error), name string) error
	out          io.Writer
}

func newBranchPruneCmd() *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Report (and, with --apply, delete) local branches already integrated into origin/main",
		Long: `trackfw branch prune replaces the 6-step manual procedure documented in CLAUDE.md §1
("Uma branch ativa por vez") with a deterministic, offline command.

Decides integration with the touched-files heuristic, NOT git's own ancestry check (which always
refuses squash-merged branches) and NOT a naive bidirectional diff against origin/main (which
false-positives on a branch that is merged but stale, once main has advanced further):

  mb      = git merge-base origin/main <branch>
  touched = git diff --name-only mb <branch>                 (what the branch touched)
  diverg  = git diff --name-only origin/main <branch> -- touched  (what still differs there)

touched empty -> integrated (safe to delete)
diverg empty  -> integrated (safe to delete) -- squash-merged, stale, main advanced since
otherwise     -> kept, with the diverging files named

Every local branch is reported, always, with its decision and reason. The current branch, any
branch checked out in another worktree, and the default branch (main) are always kept and never
evaluated for deletion. Without origin/main resolvable (offline, no remote, never fetched), the
whole command refuses and deletes nothing.

--dry-run is the default: without --apply, nothing is ever deleted, even the clearly integrated.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			deps := branchPruneDeps{
				gitExec:           defaultGitExec,
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

	fmt.Fprintf(deps.out, "trackfw branch prune — evaluating %d local branch(es) against %s\n\n", len(branches), branchPruneDefaultRemoteRef)

	var toDelete []string
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
			eval = evaluateBranchIntegration(b, deps.gitExec)
		}

		action := "keep"
		if eval.Decision.deletable() {
			action = "delete"
			toDelete = append(toDelete, b)
		}
		fmt.Fprintf(deps.out, "  %-30s %-7s %s\n", eval.Name, action, eval.Reason)
	}

	fmt.Fprintln(deps.out)
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

// defaultDeleteBranch runs `git branch -D <name>`. -D is intentional: -d refuses by ancestry,
// which the whole command exists to route around safely via evaluateBranchIntegration.
func defaultDeleteBranch(gitExec func(args ...string) (string, error), name string) error {
	_, err := gitExec("branch", "-D", name)
	return err
}
