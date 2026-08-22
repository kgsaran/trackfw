package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/spf13/cobra"
)

// pushDeps holds injectable dependencies so that runPush can be tested without
// executing real git write commands against the repository.
type pushDeps struct {
	// execGit runs a git command and returns (trimmed-stdout, error).
	execGit func(args ...string) (string, error)

	// checkGovernance returns violation messages (nil or empty slice = pass).
	// Injected so that tests do not depend on a real trackfw project layout.
	checkGovernance func() []string

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
			deps := pushDeps{
				execGit:         defaultGitExec,
				checkGovernance: defaultCheckGovernance,
				out:             cmd.OutOrStdout(),
				configForge:     config.Load().Forge,
				repoDir:         ".",
				availFn:         nil,
				checkPROpen:     nil,
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

	// main/master is blocked unconditionally.
	if branch == "main" || branch == "master" {
		return fmt.Errorf(
			"trackfw push cannot run on %q — use a feature branch:\n  git checkout -b feat/<slug>",
			branch,
		)
	}

	if !isShipBranch(branch) {
		return fmt.Errorf(
			"branch %q does not match the required pattern feat|fix|refactor|chore|docs/<slug>\n"+
				"Rename your branch or create a new one:\n  git checkout -b feat/<slug>",
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
		violations := deps.checkGovernance()
		if len(violations) > 0 {
			fmt.Fprintf(deps.out, "\nGovernance check failed:\n")
			for _, v := range violations {
				fmt.Fprintf(deps.out, "  %s\n", v)
			}
			fmt.Fprintf(deps.out, "\nCreate the required artifacts before running push:\n")
			fmt.Fprintf(deps.out, "  trackfw req new \"<title>\"\n")
			fmt.Fprintf(deps.out, "  trackfw roadmap new \"<title>\"\n")
			fmt.Fprintf(deps.out, "  trackfw roadmap move <name> wip\n")
			fmt.Fprintf(deps.out, "\nNote: this governance check is a hard gate — it is not affected by lenient\n")
			fmt.Fprintf(deps.out, "mode or per-rule severity configured in trackfw.yaml. If 'trackfw validate'\n")
			fmt.Fprintf(deps.out, "passes but 'trackfw push' aborts here, you likely have lenient mode\n")
			fmt.Fprintf(deps.out, "configured — push always requires REQ + roadmap in wip/.\n")
			return fmt.Errorf("governance check failed: %d violation(s)", len(violations))
		}

		fmt.Fprintf(deps.out, "Governance: OK\n")
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
			fmt.Fprintf(deps.out, "Warning: could not fetch origin (offline or no remote); skipping squash-merge check.\n")
		} else {
			detectPendingSquashMerges(branch, deps.execGit, deps.out)
		}
	}

	// ─── Step 4: Push ────────────────────────────────────────────────────────
	pushArgs := buildPushArgs(branch, deps.execGit)
	if opts.forceWithLease {
		// Fixed position: push --force-with-lease [-u] origin <branch> — identical
		// across the 3 CLIs (the parity gate compares this literally).
		pushArgs = append([]string{pushArgs[0], "--force-with-lease"}, pushArgs[1:]...)
	}
	if _, err := git(pushArgs...); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	if !opts.dryRun {
		fmt.Fprintf(deps.out, "Pushed: %s → origin/%s\n", branch, branch)
	}

	return nil
}
