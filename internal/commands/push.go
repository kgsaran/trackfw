package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/kgsaran/trackfw/internal/netdeadline"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

// pushDeps holds injectable dependencies so that runPush can be tested without
// executing real git write commands against the repository.
type pushDeps struct {
	// execGit runs a git command and returns (trimmed-stdout, error).
	execGit func(args ...string) (string, error)

	// checkGovernance returns governance state. nil = pass. Non-nil with empty Missing = degraded
	// (warnings printed, execution continues). Non-nil with non-empty Missing = hard failure.
	// Injected so that tests do not depend on a real trackfw project layout.
	checkGovernance func() *validator.GovernanceViolation

	// cfg is the project config used to compute governance remediation hints.
	// Zero value (flat) is the correct default for tests that do not specify a project layout.
	cfg config.ProjectConfig

	out io.Writer

	// force-with-lease gate — forge resolution.
	// configForge is the forge: value from trackfw.yaml.
	configForge string
	// repoDir is the repo root for CI file detection ("" skips CI detection — safe for tests).
	repoDir string
	// availFn injects CLI availability check for forge.NewAdapter. nil uses the production default.
	availFn func(string) bool
	// checkPROpen queries the resolved forge for an open PR/MR whose source branch is `branch`.
	// Only called when --force-with-lease is set. nil uses defaultCheckPROpen.
	checkPROpen func(adapter forge.Adapter, branch string) (bool, error)

	// ghExec runs `gh <args...>` for the detectPendingSquashMerges forge PR lookup (D4,
	// ADR-2026-10-03). nil means "no gh available" → D2 degradation for that step only.
	// Production: defaultGhExec. Tests pass nil or a stub returning pre-baked JSON.
	ghExec ghExecFn
}

// pushOpts holds the parsed CLI flags for the push command.
type pushOpts struct {
	dryRun         bool
	forceWithLease bool
}

// push --force-with-lease refusal messages — "trackfw push" (not "ship") because this command
// closes the commit→push cycle without opening a PR. See
// ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md.
// Defined here (not imported from ship.go) to keep push's user-visible messages independent of
// ship's contract string — a future rename/rephrase of ship's messages must not silently change
// what push tells the user.
const (
	pushForceLeaseNoForgeCLIMsg = "trackfw push --force-with-lease requires a forge CLI (gh, glab, or az) to confirm an open pull/merge request before rewriting remote history. No forge CLI is available for this repository — install and authenticate it, or push without --force-with-lease."

	pushForceLeaseNoPROpenFmt = "trackfw push --force-with-lease refuses to run: branch %q has no open pull/merge request. Open the PR/MR first (trackfw ship without --force-with-lease, or your forge's web UI), then retry."

	pushForceLeaseCannotVerifyFmt = "trackfw push --force-with-lease could not verify whether branch %q has an open pull/merge request (%s CLI error: %s). Refusing rather than risking a force push without a verified PR — check your %s CLI authentication and retry."
)

func newPushCmd() *cobra.Command {
	var dryRun bool
	var forceWithLease bool

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Governed git push for commits already created",
		Long: `trackfw push pushes already-created commits without committing and without opening a PR/MR.

  1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>
  2. Validates governance — REQ + roadmap in wip/ must exist for feat/fix/refactor branches
     (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip
     this check, mirroring 'trackfw commit'
  3. Detects pending squash-merges in other branches (advisory only)
  4. Pushes to origin (adds -u if no upstream is configured yet)

push never commits and never opens a PR/MR.
Does not accept -m. If you have not committed yet, run 'trackfw commit -m "..."' first.

Compositional vocabulary:
  trackfw commit -m "..."   commits
  trackfw push              pushes
  trackfw ship -m "..."     commit + push + PR (composition)

--force-with-lease pushes with 'git push --force-with-lease' instead of a plain push — for
the post-rebase case, where a plain push is rejected. It only runs when the branch already
has an open PR/MR (verified via the resolved forge CLI): the safe path is always to open the
PR first.`,
		// allow_abbrev=False (argparse) equivalent: cobra/pflag never abbreviates by default.
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cfg := config.Load()
			deps := pushDeps{
				execGit:         defaultGitExec,
				checkGovernance: defaultCheckGovernance,
				cfg:             cfg,
				out:             cmd.OutOrStdout(),
				configForge:     cfg.Forge,
				repoDir:         ".",
				availFn:         nil,
				checkPROpen:     nil,
				ghExec:          defaultGhExec,
			}
			return runPush(pushOpts{dryRun: dryRun, forceWithLease: forceWithLease}, deps)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be done without executing write commands")
	cmd.Flags().BoolVar(&forceWithLease, "force-with-lease", false, "Governed force-push (git push --force-with-lease); requires an open PR/MR on this branch")

	return cmd
}

// runPush implements the push sequence: branch validation, governance, force-with-lease gate,
// squash-merge detection, and push. Never commits and never opens a PR/MR.
func runPush(opts pushOpts, deps pushDeps) error {
	// Inner wrapper: skips write commands in --dry-run mode.
	git := func(args ...string) (string, error) {
		if opts.dryRun && isGitWriteCmd(args) {
			fmt.Fprintf(deps.out, "[dry-run] git %s\n", strings.Join(args, " "))
			return "", nil
		}
		return deps.execGit(args...)
	}

	// ─── Step 1: Branch validation ───────────────────────────────────────────
	branch, err := deps.execGit("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("could not determine current branch (are you in a git repo?): %w", err)
	}
	branch = strings.TrimSpace(branch)

	// ─── D3 (ADR-2026-11-26): bootstrap push for default branch ─────────────────
	// Self-contained early return — never falls through to isShipBranch, governance,
	// or squash-merge detection. Accepts main/master only when the remote has no
	// branches yet and the local branch has exactly one commit.
	if branch == "main" || branch == "master" {
		// D3: refuse the user --force-with-lease flag before any query.
		if opts.forceWithLease {
			return fmt.Errorf(
				"trackfw push cannot run on %q: --force-with-lease is not valid for the bootstrap push",
				branch,
			)
		}
		// D3: verify remote state (fail-closed — any error aborts).
		empty, remErr := checkRemoteEmpty(deps.execGit)
		if remErr != nil {
			return fmt.Errorf(
				"trackfw push cannot run on %q: cannot verify remote state (%v).\n"+
					"Complete the bootstrap first:\n"+
					"  1. Create the repository on your forge\n"+
					"  2. git remote add origin <url>\n"+
					"  3. git status --short\n"+
					"  4. git add <each path listed by git status --short>\n"+
					"  5. trackfw commit -m \"chore(scaffold): initial\"\n"+
					"  6. trackfw push  (from main or master)",
				branch, remErr,
			)
		}
		if !empty {
			return fmt.Errorf(
				"trackfw push cannot run on %q: the remote already has branches.\n"+
					"Use a feature branch: trackfw branch new feat/<slug>",
				branch,
			)
		}
		// D3: single-commit gate — only the bootstrap commit is allowed.
		countOut, countErr := deps.execGit("rev-list", "--count", "refs/heads/"+branch)
		if countErr != nil {
			return fmt.Errorf(
				"trackfw push cannot run on %q: cannot count commits (%v)",
				branch, countErr,
			)
		}
		if strings.TrimSpace(countOut) != "1" {
			return fmt.Errorf(
				"trackfw push cannot run on %q: bootstrap push requires exactly 1 commit, found %s.\n"+
					"Use a feature branch for subsequent commits: trackfw branch new feat/<slug>",
				branch, strings.TrimSpace(countOut),
			)
		}
		// D3: absence-lease push — publishes default branch exactly once.
		// The lease is internal; the user --force-with-lease flag is refused above.
		absenceLease := "--force-with-lease=refs/heads/" + branch + ":"
		if pushOut, pushErr := git("push", "-u", "origin", branch, absenceLease); pushErr != nil {
			return pushFailure(pushOut, pushErr)
		}
		if !opts.dryRun {
			fmt.Fprintf(deps.out, "Bootstrap push: %s → origin/%s\n", branch, branch)
		}
		return nil
	}

	if !isShipBranch(branch) {
		return fmt.Errorf(
			"branch %q does not match the required pattern feat|fix|refactor|chore|docs/<slug>\n"+
				"Rename your branch or create a new one:\n  trackfw branch new feat/<slug>",
			branch,
		)
	}

	fmt.Fprintf(deps.out, "Branch: %s\n", branch)

	// ─── Step 2: Governance ──────────────────────────────────────────────────
	// push never reads the index — no doc-only exception. Governance is either
	// skipped (chore/docs) or enforced (feat/fix/refactor), nothing in between.
	if !isGatedShipBranch(branch) {
		// chore/docs: housekeeping types already exempted from this gate by
		// `trackfw branch new` and `trackfw commit` — push without it too.
		fmt.Fprintf(deps.out, "Governance: skipped (chore/docs branch)\n")
	} else {
		gv := deps.checkGovernance()
		if gv != nil && len(gv.Missing) > 0 {
			// Hard failure: print warnings first, then violations, then remediation.
			for _, w := range gv.Warnings {
				fmt.Fprintf(deps.out, "Governance: degraded: %s\n", w)
			}
			fmt.Fprintf(deps.out, "\nGovernance check failed:\n")
			for _, v := range gv.Missing {
				fmt.Fprintf(deps.out, "  %s\n", v)
			}
			fmt.Fprintf(deps.out, "\nCreate the required artifacts before running push:\n")
			fmt.Fprintf(deps.out, "  %s\n", validator.ReqNewLine(deps.cfg))
			fmt.Fprintf(deps.out, "  %s\n", validator.RoadmapNewLine(deps.cfg))
			fmt.Fprintf(deps.out, "  trackfw roadmap move <name> wip\n")
			fmt.Fprintf(deps.out, "\nNote: this governance check is a hard gate — it is not affected by lenient\n")
			fmt.Fprintf(deps.out, "mode or per-rule severity configured in trackfw.yaml. If 'trackfw validate'\n")
			fmt.Fprintf(deps.out, "passes but 'trackfw push' aborts here, you likely have lenient mode\n")
			fmt.Fprintf(deps.out, "configured — push always requires REQ + roadmap in wip/.\n")
			return fmt.Errorf("governance check failed: %d violation(s)", len(gv.Missing))
		}
		if gv != nil && len(gv.Warnings) > 0 {
			// Degraded: warnings present, no hard violations — execution continues.
			for _, w := range gv.Warnings {
				fmt.Fprintf(deps.out, "Governance: degraded: %s\n", w)
			}
		} else {
			fmt.Fprintf(deps.out, "Governance: OK\n")
		}
	}

	// ─── Step 2.5: force-with-lease gate ──────────────────────────────────────
	// Runs before any write (push) — a refusal here never leaves the caller unable to push.
	// Read-only, so it runs in --dry-run too.
	if opts.forceWithLease {
		gateRemoteURL, _ := deps.execGit("remote", "get-url", "origin")
		gateRemoteURL = strings.TrimSpace(gateRemoteURL)

		resolution, resErr := forge.Resolve(forge.Input{
			FlagForge:   "",
			ConfigForge: deps.configForge,
			RemoteURL:   gateRemoteURL,
			RepoDir:     deps.repoDir,
		})
		if resErr != nil {
			return resErr
		}

		adapter := forge.NewAdapter(resolution.Forge, deps.availFn)
		if resolution.Forge == "manual" || !adapter.Available {
			return fmt.Errorf("%s", pushForceLeaseNoForgeCLIMsg)
		}

		checkPROpen := deps.checkPROpen
		if checkPROpen == nil {
			checkPROpen = defaultCheckPROpen
		}
		open, prErr := checkPROpen(adapter, branch)
		if prErr != nil {
			return fmt.Errorf(pushForceLeaseCannotVerifyFmt, branch, adapter.CLIName, prErr, adapter.CLIName)
		}
		if !open {
			return fmt.Errorf(pushForceLeaseNoPROpenFmt, branch)
		}

		fmt.Fprintf(deps.out, "force-with-lease: open %s confirmed for %q (%s).\n", adapter.Noun, branch, resolution.Forge)
	}

	// ─── Step 3: Squash-merge detection ──────────────────────────────────────
	// fetch origin --prune; any failure (offline, no remote) is non-blocking.
	if opts.dryRun {
		fmt.Fprintf(deps.out, "[dry-run] git fetch origin --prune\n")
	} else {
		if _, ferr := deps.execGit("fetch", "origin", "--prune"); ferr != nil {
			// ADR D1/D5: name the timeout so the user can distinguish it from "offline".
			if errors.Is(ferr, netdeadline.ErrTimeout) {
				fmt.Fprintf(deps.out, "Warning: could not fetch origin (%s); skipping squash-merge check.\n", ferr.Error())
			} else {
				fmt.Fprintf(deps.out, "Warning: could not fetch origin (offline or no remote); skipping squash-merge check.\n")
			}
		} else {
			detectPendingSquashMerges(branch, deps.execGit, deps.ghExec, deps.out)
		}
	}

	// ─── Step 3.5: D4 (ADR-2026-11-26) — block feature push to empty remote ────
	// Fail-open: if the remote cannot be queried, the push proceeds as today (the
	// network failure will surface at the actual push). Blocks only when we can
	// confirm the remote has no branches at all — "empty remote", not "no main/master".
	if d4Empty, d4Err := checkRemoteEmpty(deps.execGit); d4Err == nil && d4Empty {
		return errors.New("trackfw push: the remote has no branches yet.\n" +
			bootstrapStepsHint("push your feature branch"))
	}

	// ─── Step 4: Push ────────────────────────────────────────────────────────
	pushArgs := buildPushArgs(branch, deps.execGit)
	if opts.forceWithLease {
		// Fixed position: push --force-with-lease [-u] origin <branch> — identical
		// across the 3 CLIs (the parity gate compares this literally).
		pushArgs = append([]string{pushArgs[0], "--force-with-lease"}, pushArgs[1:]...)
	}
	if pushOut, err := git(pushArgs...); err != nil {
		return pushFailure(pushOut, err)
	}

	if !opts.dryRun {
		fmt.Fprintf(deps.out, "Pushed: %s → origin/%s\n", branch, branch)
	}

	return nil
}

// ─── D3/D4 helpers (ADR-2026-11-26) ─────────────────────────────────────────

// bootstrapStepsHint returns the shared bootstrap step list used by D4 messages
// in push and ship. The action parameter completes "Then come back and <action>."
// (A3 — ADR-2026-11-26: common D4 steps extracted to avoid verbatim duplication
// between push.go and ship.go.)
func bootstrapStepsHint(action string) string {
	return "Publish the default branch first:\n" +
		"  1. Create the repository on your forge\n" +
		"  2. git remote add origin <url>\n" +
		"  3. git status --short\n" +
		"  4. git add <each path listed by git status --short>\n" +
		"  5. trackfw commit -m \"chore(scaffold): initial\"\n" +
		"  6. trackfw push  (from main or master)\n" +
		"Then come back and " + action + "."
}

// checkRemoteEmpty queries every push URL configured for origin and reports
// whether all of them have no branches.
//
// Returns (true, nil) when every URL returns no refs/heads/* lines from
// git ls-remote --heads. Returns (false, nil) when any URL has branches.
// Returns (false, err) when the remote state cannot be determined — callers
// must treat this as "uncertain": D3 refuses (fail-closed), D4 passes (fail-open).
//
// D3-bypass guard (ADR-2026-11-26, ML-2C): before querying with ls-remote
// --heads, each push URL is passed to ls-remote --get-url. If the result
// differs from the URL, a git insteadOf/pushInsteadOf rule would redirect the
// ls-remote query to a different destination than the actual push — the check
// would see an empty remote while the push reaches a non-empty one. fail-closed.
func checkRemoteEmpty(execGit func(args ...string) (string, error)) (bool, error) {
	urlsOut, err := execGit("remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		return false, fmt.Errorf("remote get-url: %w", err)
	}
	var urls []string
	for _, u := range strings.Split(urlsOut, "\n") {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return false, fmt.Errorf("no push URLs configured for origin")
	}
	for _, u := range urls {
		// D3-bypass guard: verify ls-remote would query the same URL that push
		// will use. git applies insteadOf to the URL passed to ls-remote; if the
		// resolved URL differs from u, the check sees a different destination.
		resolved, resolvedErr := execGit("ls-remote", "--get-url", u)
		resolved = strings.TrimSpace(resolved)
		if resolvedErr != nil || resolved != u {
			resolvedDisplay := resolved
			if resolvedErr != nil {
				resolvedDisplay = resolvedErr.Error()
			}
			return false, fmt.Errorf(
				"push URL %q is rewritten to %q by an insteadOf/pushInsteadOf rule — "+
					"remove the rule temporarily to perform the bootstrap push",
				u, resolvedDisplay,
			)
		}
		lsOut, lsErr := execGit("ls-remote", "--heads", u)
		if lsErr != nil {
			return false, fmt.Errorf("ls-remote %s: %w", u, lsErr)
		}
		hasBranches, unrecognized := parseLSRemoteHeads(lsOut)
		if unrecognized {
			return false, fmt.Errorf("ls-remote returned unrecognized output for %s", u)
		}
		if hasBranches {
			return false, nil
		}
	}
	return true, nil
}

// parseLSRemoteHeads parses the stdout of `git ls-remote --heads <url>`.
// Each non-empty line must have the form:  <hex>\t<ref>
// Returns hasBranches=true when at least one ref starts with refs/heads/.
// Returns unrecognized=true when at least one non-empty line does not match
// the expected format; callers should treat unrecognized output as an error.
func parseLSRemoteHeads(out string) (hasBranches bool, unrecognized bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			unrecognized = true
			continue
		}
		hash, ref := line[:tab], line[tab+1:]
		if !isHexString(hash) || len(hash) == 0 {
			unrecognized = true
			continue
		}
		if strings.HasPrefix(ref, "refs/heads/") {
			hasBranches = true
		}
		// Other refs under refs/heads/ that do not indicate a branch (none
		// expected from --heads, which limits output to refs/heads/*) are
		// skipped without setting unrecognized.
	}
	return
}

// isHexString reports whether every rune in s is a hex digit (0-9, a-f, A-F).
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
