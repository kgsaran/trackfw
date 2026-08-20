package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

// shipDeps holds injectable dependencies so that runShip can be tested without
// executing real git write commands against the repository.
type shipDeps struct {
	// execGit runs a git command and returns (trimmed-stdout, error).
	// The caller is responsible for never passing "add ." or "add -A".
	execGit func(args ...string) (string, error)

	// checkGovernance returns violation messages (nil or empty slice = pass).
	// Injected so that tests do not depend on a real trackfw project layout.
	checkGovernance func() []string

	out io.Writer

	// Step 7 — forge resolution and PR/MR opening.
	// configForge is the forge: value from trackfw.yaml (injected; production: config.Load().Forge).
	configForge string
	// repoDir is the repo root for CI file detection ("" skips CI detection — safe for tests).
	repoDir string
	// availFn injects CLI availability check for forge.NewAdapter. nil uses the production default.
	availFn func(string) bool
	// execForgeCLI runs the forge CLI (gh, glab, az). nil uses defaultExecForgeCLI.
	execForgeCLI func(name string, args []string) error

	// checkPROpen queries the resolved forge for an open PR/MR whose source branch is
	// `branch`. Only called when --force-with-lease is set. nil uses defaultCheckPROpen.
	// Returns (open, error); error is non-nil ONLY when the forge CLI call itself failed
	// or its output could not be parsed — callers must treat that as "cannot verify",
	// never silently as "no PR".
	checkPROpen func(adapter forge.Adapter, branch string) (bool, error)
}

// shipOpts holds the parsed CLI flags for the ship command.
type shipOpts struct {
	message        string
	dryRun         bool
	noPR           bool   // --no-pr: skip PR/MR creation after push
	forge          string // --forge: override forge detection
	forceWithLease bool   // --force-with-lease: governed force-push, restricted to branches with an open PR/MR
}

// forceWithLease refusal messages. Named constants so the ML-1B parity gate has a single
// place to compare byte-for-byte across the 3 CLIs. Never expose "--force" (raw) as a flag —
// see ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.
const (
	forceLeaseNoForgeCLIMsg = "trackfw ship --force-with-lease requires a forge CLI (gh, glab, or az) to confirm an open pull/merge request before rewriting remote history. No forge CLI is available for this repository — install and authenticate it, or push without --force-with-lease."

	forceLeaseNoPROpenFmt = "trackfw ship --force-with-lease refuses to run: branch %q has no open pull/merge request. Open the PR/MR first (trackfw ship without --force-with-lease, or your forge's web UI), then retry."

	forceLeaseCannotVerifyFmt = "trackfw ship --force-with-lease could not verify whether branch %q has an open pull/merge request (%s CLI error: %s). Refusing rather than risking a force push without a verified PR — check your %s CLI authentication and retry."
)

// gitWriteCommands lists git subcommands that modify local or remote state.
// In --dry-run mode these are printed but not executed.
var gitWriteCommands = map[string]bool{
	"commit": true,
	"push":   true,
	"fetch":  true,
}

func newShipCmd() *cobra.Command {
	var message string
	var dryRun bool
	var noPR bool
	var forgeFlag string
	var forceWithLease bool

	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Governed git commit + push for feat/fix/refactor/chore/docs branches",
		Long: `trackfw ship runs a governed delivery sequence:

  1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>
  2. Validates governance — REQ + roadmap in wip/ must exist for feat/fix/refactor branches
     (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip
     this check, mirroring 'trackfw commit'
  3. Detects pending squash-merges in other branches (advisory only)
  4. Reviews what is staged (git status --short + git diff --cached --stat)
  5. Commits with Conventional Commits format (-m is required, unless nothing is staged and
     --force-with-lease is set — see below)
  6. Pushes to origin (adds -u if no upstream is configured yet)
  7. Opens PR/MR via the resolved forge CLI (or prints URL if CLI is absent)

Stage your files explicitly before running ship.
This command never executes 'git add .' or 'git add -A'.

--force-with-lease pushes with 'git push --force-with-lease' instead of a plain push — for
the post-rebase case, where a plain push is rejected. It only runs when the branch already
has an open PR/MR (verified via the resolved forge CLI): the safe path is always to open the
PR first. When nothing is staged, it pushes existing commits without requiring -m.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Silence usage for runtime errors (governance failures, missing files, etc.).
			// Flag-parse errors happen before RunE is reached; cobra still shows usage for those.
			cmd.SilenceUsage = true
			deps := shipDeps{
				execGit:         defaultGitExec,
				checkGovernance: defaultCheckGovernance,
				out:             cmd.OutOrStdout(),
				configForge:     config.Load().Forge,
				repoDir:         ".",
				availFn:         nil, // forge.NewAdapter uses its own default when nil
				execForgeCLI:    defaultExecForgeCLI,
				checkPROpen:     nil, // defaultCheckPROpen
			}
			return runShip(shipOpts{
				message:        message,
				dryRun:         dryRun,
				noPR:           noPR,
				forge:          forgeFlag,
				forceWithLease: forceWithLease,
			}, deps)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (Conventional Commits format required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be done without executing write commands")
	cmd.Flags().BoolVar(&noPR, "no-pr", false, "Skip PR/MR creation after push")
	cmd.Flags().StringVar(&forgeFlag, "forge", "", "Override forge detection (github, gitlab, bitbucket, azure)")
	cmd.Flags().BoolVar(&forceWithLease, "force-with-lease", false, "Governed force-push (git push --force-with-lease); requires an open PR/MR on this branch")

	return cmd
}

// defaultGitExec is the production git executor.
// It runs "git <args...>" and returns trimmed stdout.
//
// On failure, surfaces git's actual stderr text (trimmed) instead of exec.Command().Output()'s
// own error, which formats as the generic "exit status N" and discards git's real diagnostic
// (e.g. "! [rejected] ... (stale info)" from a refused --force-with-lease push). Node's
// defaultExecGit and Python's default_exec_git already captured stderr this way — without this,
// every git-failure message this command ever prints (git commit failed: ..., git push failed:
// ...) diverges byte-for-byte from Node/Python. Caught by check-ship-force-parity.sh's
// "remote-advanced-lease-mismatch" scenario (ML-1B,
// ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md), which is the
// first parity gate to exercise a real git push rejection end to end.
func defaultGitExec(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
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
		return strings.TrimSpace(string(out)), errors.New(msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// defaultExecForgeCLI runs the forge CLI (gh, glab, az) with inherited stdin/stdout/stderr.
func defaultExecForgeCLI(name string, args []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// defaultCheckPROpen queries the resolved forge CLI for an open PR/MR whose source branch is
// `branch`, using the same list-based query shape for every forge: empty result means "no PR"
// (exit 0), any non-zero exit or unparseable output means "cannot verify" (returned as error —
// never conflated with "no PR"). bitbucket and "manual" never reach here: runShip only calls
// checkPROpen when adapter.Available is true, and bitbucket's adapter is always Available=false.
func defaultCheckPROpen(adapter forge.Adapter, branch string) (bool, error) {
	var args []string
	switch adapter.Forge {
	case "github":
		args = []string{"pr", "list", "--head", branch, "--state", "open", "--json", "number"}
	case "gitlab":
		// glab mr list: --source-branch filters by source branch, --state opened matches
		// gh's "open" state, -F json requests machine-readable output (glab's own flag,
		// not an external jq/GNU dependency).
		args = []string{"mr", "list", "--source-branch", branch, "--state", "opened", "-F", "json"}
	case "azure":
		// az defaults to --output json; passed explicitly here for clarity, not reliance
		// on the ambient default.
		args = []string{"repos", "pr", "list", "--source-branch", branch, "--status", "active", "--output", "json"}
	default:
		return false, fmt.Errorf("no PR/MR query defined for forge %q", adapter.Forge)
	}

	// Capture stderr explicitly and surface its trimmed text in the "cannot verify" error —
	// exec.Command().Output()'s own error (an *exec.ExitError) formats as the generic
	// "exit status N", discarding the CLI's actual diagnostic (e.g. an auth failure). Node's
	// spawnSync and Python's subprocess.run both surface the real stderr text by default, so
	// without this capture Go's forceLeaseCannotVerifyFmt message diverges byte-for-byte from
	// Node/Python's — caught by check-ship-force-parity.sh's "forge-nao-verificavel" scenario
	// (ML-1B, ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md).
	cmd := exec.Command(adapter.CLIName, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			if exitErr, ok := err.(*exec.ExitError); ok {
				msg = fmt.Sprintf("%s exited with %d", adapter.CLIName, exitErr.ExitCode())
			} else {
				msg = err.Error()
			}
		}
		return false, errors.New(msg)
	}

	var results []json.RawMessage
	if err := json.Unmarshal(out, &results); err != nil {
		return false, fmt.Errorf("could not parse %s output: %w", adapter.CLIName, err)
	}
	return len(results) > 0, nil
}

// defaultCheckGovernance calls CheckShipGovernance as a hard gate.
// Bypasses baseline, lenient mode and per-rule severity configuration.
func defaultCheckGovernance() []string {
	gv := validator.CheckShipGovernance()
	if gv == nil {
		return nil
	}
	return gv.Missing
}

// runShip implements the six-step ship sequence.
// All git write operations are guarded by dryRun via the inner `git` wrapper.
func runShip(opts shipOpts, deps shipDeps) error {
	// Inner wrapper: skips write commands in --dry-run mode.
	git := func(args ...string) (string, error) {
		if opts.dryRun && isGitWriteCmd(args) {
			fmt.Fprintf(deps.out, "[dry-run] git %s\n", strings.Join(args, " "))
			return "", nil
		}
		return deps.execGit(args...)
	}

	// ─── Step 0: staged files ─────────────────────────────────────────────────
	// Read once, up front, so Steps 1 and 2 can grant a doc-only exception before
	// they run — and so Step 4 below reuses the same read instead of querying git twice.
	stagedOut, _ := deps.execGit("diff", "--cached", "--name-only")
	stagedFiles := splitNonEmptyLines(stagedOut)
	docOnly := allDocOnly(stagedFiles)

	// ─── Step 1: Branch validation ───────────────────────────────────────────
	branch, err := deps.execGit("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("could not determine current branch (are you in a git repo?): %w", err)
	}
	branch = strings.TrimSpace(branch)

	// main/master is blocked unconditionally — the doc-only exception never applies here.
	if branch == "main" || branch == "master" {
		return fmt.Errorf(
			"trackfw ship cannot run on %q — use a feature branch:\n  git checkout -b feat/<slug>",
			branch,
		)
	}

	if !docOnly && !isShipBranch(branch) {
		return fmt.Errorf(
			"branch %q does not match the required pattern feat|fix|refactor|chore|docs/<slug>\n"+
				"Rename your branch or create a new one:\n  git checkout -b feat/<slug>",
			branch,
		)
	}

	fmt.Fprintf(deps.out, "Branch: %s\n", branch)

	// ─── Step 2: Governance ──────────────────────────────────────────────────
	// Doc-only changes (all staged files under docs/, vault/, or *.md) are exempt from
	// REQ+roadmap governance — mirrors the CLAUDE.md §7 exception for doc-only changes.
	if docOnly {
		fmt.Fprintf(deps.out, "Governance: skipped (doc-only change)\n")
	} else if isShipBranch(branch) && !isGatedShipBranch(branch) {
		// chore/docs: housekeeping types already exempted from this gate by
		// `trackfw branch new` and `trackfw commit` — ship without it too.
		fmt.Fprintf(deps.out, "Governance: skipped (chore/docs branch)\n")
	} else {
		violations := deps.checkGovernance()
		if len(violations) > 0 {
			fmt.Fprintf(deps.out, "\nGovernance check failed:\n")
			for _, v := range violations {
				fmt.Fprintf(deps.out, "  %s\n", v)
			}
			fmt.Fprintf(deps.out, "\nCreate the required artifacts before running ship:\n")
			fmt.Fprintf(deps.out, "  trackfw req new \"<title>\"\n")
			fmt.Fprintf(deps.out, "  trackfw roadmap new \"<title>\"\n")
			fmt.Fprintf(deps.out, "  trackfw roadmap move <name> wip\n")
			fmt.Fprintf(deps.out, "\nNote: this governance check is a hard gate — it is not affected by lenient\n")
			fmt.Fprintf(deps.out, "mode or per-rule severity configured in trackfw.yaml. If 'trackfw validate'\n")
			fmt.Fprintf(deps.out, "passes but 'trackfw ship' aborts here, you likely have lenient mode\n")
			fmt.Fprintf(deps.out, "configured — ship always requires REQ + roadmap in wip/.\n")
			return fmt.Errorf("governance check failed: %d violation(s)", len(violations))
		}

		fmt.Fprintf(deps.out, "Governance: OK\n")
	}

	// ─── Step 2.5: force-with-lease gate ──────────────────────────────────────
	// Runs before any write (commit/push) — a refusal here must never leave a local
	// commit the caller cannot push. Read-only, so it runs in --dry-run too, same
	// posture as the read-only calls in Step 0 / Step 7.
	//
	// forceLeaseAdapter/forceLeaseResolution are reused by Step 7 below to avoid a
	// second forge resolution and a duplicate "Forge: ..." line, and because Step 7
	// must skip PR/MR creation entirely once this gate has confirmed one is already
	// open — creating a second one would be a spurious failure on every successful
	// force push.
	var forceLeaseAdapter forge.Adapter
	var forceLeaseResolution forge.Resolution
	if opts.forceWithLease {
		gateRemoteURL, _ := deps.execGit("remote", "get-url", "origin")
		gateRemoteURL = strings.TrimSpace(gateRemoteURL)

		resolution, resErr := forge.Resolve(forge.Input{
			FlagForge:   opts.forge,
			ConfigForge: deps.configForge,
			RemoteURL:   gateRemoteURL,
			RepoDir:     deps.repoDir,
		})
		if resErr != nil {
			return resErr
		}

		adapter := forge.NewAdapter(resolution.Forge, deps.availFn)
		if resolution.Forge == "manual" || !adapter.Available {
			return fmt.Errorf("%s", forceLeaseNoForgeCLIMsg)
		}

		checkPROpen := deps.checkPROpen
		if checkPROpen == nil {
			checkPROpen = defaultCheckPROpen
		}
		open, prErr := checkPROpen(adapter, branch)
		if prErr != nil {
			return fmt.Errorf(forceLeaseCannotVerifyFmt, branch, adapter.CLIName, prErr, adapter.CLIName)
		}
		if !open {
			return fmt.Errorf(forceLeaseNoPROpenFmt, branch)
		}

		fmt.Fprintf(deps.out, "force-with-lease: open %s confirmed for %q (%s).\n", adapter.Noun, branch, resolution.Forge)
		forceLeaseAdapter = adapter
		forceLeaseResolution = resolution
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

	// ─── Step 4: Review staged ───────────────────────────────────────────────
	statusOut, _ := deps.execGit("status", "--short")
	diffStatOut, _ := deps.execGit("diff", "--cached", "--stat")

	fmt.Fprintf(deps.out, "\n── Staged changes ──────────────────────────────────────\n")
	if statusOut != "" {
		fmt.Fprintf(deps.out, "%s\n", statusOut)
	}
	if diffStatOut != "" {
		fmt.Fprintf(deps.out, "%s\n", diffStatOut)
	}
	fmt.Fprintf(deps.out, "────────────────────────────────────────────────────────\n\n")

	// Reuses stagedFiles read at the top of the function (Step 0) — never re-query git here.
	//
	// --force-with-lease push-only mode: a rebase that resolved conflicts already
	// committed the result (the index is clean afterwards) — there is nothing left to
	// stage or commit, only to push. Only --force-with-lease grants this exception;
	// without it, "nothing staged" still aborts exactly as before (non-regression).
	pushOnly := opts.forceWithLease && len(stagedFiles) == 0

	if len(stagedFiles) == 0 && !opts.forceWithLease {
		return fmt.Errorf(
			"nothing is staged — stage your files explicitly before running ship:\n" +
				"  git add <file1> <file2> ...\n" +
				"Never use 'git add .' or 'git add -A'",
		)
	}

	// ─── Step 5: Commit ──────────────────────────────────────────────────────
	if pushOnly {
		fmt.Fprintf(deps.out, "Nothing staged — --force-with-lease pushes existing commits only, no new commit.\n")
	} else {
		if opts.message == "" {
			return fmt.Errorf(
				"commit message is required — use -m:\n" +
					"  trackfw ship -m \"feat(<scope>): <description>\"",
			)
		}

		if _, err := git("commit", "-m", opts.message); err != nil {
			return fmt.Errorf("git commit failed: %w", err)
		}

		if !opts.dryRun {
			fmt.Fprintf(deps.out, "Committed: %s\n", opts.message)
		}
	}

	// ─── Step 6: Push ────────────────────────────────────────────────────────
	pushArgs := buildPushArgs(branch, deps.execGit)
	if opts.forceWithLease {
		// Fixed position: push --force-with-lease [-u] origin <branch> — identical
		// across the 3 CLIs (ML-1B's parity gate compares this literally).
		pushArgs = append([]string{pushArgs[0], "--force-with-lease"}, pushArgs[1:]...)
	}
	if _, err := git(pushArgs...); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	if !opts.dryRun {
		fmt.Fprintf(deps.out, "Pushed:    %s → origin/%s\n", branch, branch)
	}

	// ─── Step 7: Open PR/MR ──────────────────────────────────────────────────
	// --force-with-lease only ever reaches here after Step 2.5 confirmed a PR/MR is
	// already open on this branch — creating another one would be a spurious failure
	// on every successful force push. Reuses the adapter/resolution Step 2.5 already
	// computed instead of resolving the forge a second time.
	if opts.forceWithLease {
		fmt.Fprintf(deps.out, "Forge:     %s (source: %s)\n", forceLeaseResolution.Forge, forceLeaseResolution.Source)
		fmt.Fprintf(deps.out, "%s already open — skipping creation (--force-with-lease).\n", forceLeaseAdapter.Noun)
		fmt.Fprintf(deps.out, "\nship complete.\n")
		return nil
	}

	// Resolve forge from: flag → config → remote URL → CI files → manual.
	remoteURL, _ := deps.execGit("remote", "get-url", "origin")
	remoteURL = strings.TrimSpace(remoteURL)

	resolution, resErr := forge.Resolve(forge.Input{
		FlagForge:   opts.forge,
		ConfigForge: deps.configForge,
		RemoteURL:   remoteURL,
		RepoDir:     deps.repoDir,
	})
	if resErr != nil {
		fmt.Fprintf(deps.out, "Warning: forge resolution error: %s — open PR/MR manually.\n", resErr)
		fmt.Fprintf(deps.out, "\nship complete.\n")
		return nil
	}

	adapter := forge.NewAdapter(resolution.Forge, deps.availFn)
	fmt.Fprintf(deps.out, "Forge:     %s (source: %s)\n", resolution.Forge, resolution.Source)

	if opts.noPR {
		fmt.Fprintf(deps.out, "--no-pr: skipping %s creation.\n", adapter.Noun)
		fmt.Fprintf(deps.out, "\nship complete.\n")
		return nil
	}

	// Title/body computed once for every remaining branch below (dry-run and real CLI
	// invocation alike). git log/diff are read-only — they run in --dry-run mode too,
	// same as the staged-files read in Step 0.
	//
	// Design decision (documented per roadmap ML-1A): the title is always
	// firstLine(opts.message), the -m message passed to this very `ship` call, even
	// when the branch carries multiple prior commits. Deriving a distinct "PR title"
	// from N unrelated commit subjects would need a heuristic with no unambiguous
	// answer — the simplest, least surprising rule is that -m is the PR's summary.
	base := defaultBaseBranch(deps.execGit)
	commits := gitCommitsSince(base, deps.execGit)
	title := firstLine(opts.message)
	body := buildPRBody(branch, commits)

	if opts.dryRun {
		fmt.Fprintf(deps.out, "[dry-run] Title: %s\n", title)
		fmt.Fprintf(deps.out, "[dry-run] Body:\n%s\n", body)
		if !adapter.Available && resolution.Forge != "manual" {
			url := adapter.FallbackURL(remoteURL, branch)
			if url != "" {
				fmt.Fprintf(deps.out, "[dry-run] %s CLI (%s) not available — would open in browser:\n  %s\n", adapter.Noun, adapter.CLIName, url)
			} else {
				fmt.Fprintf(deps.out, "[dry-run] %s CLI (%s) not available — would open %s manually\n", adapter.Noun, adapter.CLIName, adapter.Noun)
			}
		} else {
			fmt.Fprintf(deps.out, "[dry-run] would open %s via %s CLI\n", adapter.Noun, resolution.Forge)
		}
		return nil
	}

	if resolution.Forge == "manual" {
		fmt.Fprintf(deps.out, "\nOpen your %s manually at:\n  %s\n", adapter.Noun, remoteURL)
		fmt.Fprintf(deps.out, "\nship complete.\n")
		return nil
	}

	if !adapter.Available {
		url := adapter.FallbackURL(remoteURL, branch)
		if url != "" {
			fmt.Fprintf(deps.out, "%s CLI (%s) not available — open in browser:\n  %s\n", adapter.Noun, adapter.CLIName, url)
		} else {
			fmt.Fprintf(deps.out, "%s CLI (%s) not available — open %s manually.\n", adapter.Noun, adapter.CLIName, adapter.Noun)
		}
		fmt.Fprintf(deps.out, "\nship complete.\n")
		return nil
	}

	// CLI is available — invoke it to create the PR/MR.
	cliArgs := buildForgeCreateArgs(adapter, title, body)

	execForgeCLI := deps.execForgeCLI
	if execForgeCLI == nil {
		execForgeCLI = defaultExecForgeCLI
	}

	if err := execForgeCLI(adapter.CLIName, cliArgs); err != nil {
		url := adapter.FallbackURL(remoteURL, branch)
		fmt.Fprintf(deps.out, "Warning: %s CLI failed (%s).\n", adapter.Noun, err)
		if url != "" {
			fmt.Fprintf(deps.out, "Open in browser:\n  %s\n", url)
		}
	} else {
		fmt.Fprintf(deps.out, "%s created.\n", adapter.Noun)
	}

	fmt.Fprintf(deps.out, "\nship complete.\n")
	return nil
}

// firstLine returns only the first line of s (before any newline).
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

// commitMessageSep delimits full commit messages (%B) in the output of gitCommitsSince's
// `git log --format=%B<sep>`. Chosen because it is a non-printable control character that
// cannot appear in a real commit message and is unaffected by strings.TrimSpace, which
// defaultGitExec applies only to the start/end of the whole output.
const commitMessageSep = "\x1e"

// gitCommitsSince returns the full message (subject + body) of every non-merge commit in
// base..HEAD, most-recent-first (git log's natural order). Returns nil on any git error or
// when the range is empty.
func gitCommitsSince(base string, execGit func(args ...string) (string, error)) []string {
	out, err := execGit("log", base+"..HEAD", "--no-merges", "--format=%B"+commitMessageSep)
	if err != nil {
		return nil
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	parts := strings.Split(out, commitMessageSep)
	commits := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "\n")
		if strings.TrimSpace(p) == "" {
			continue
		}
		commits = append(commits, p)
	}
	return commits
}

// gitSymbolicRefOriginHeadPrefix is the fixed prefix `git symbolic-ref refs/remotes/origin/HEAD`
// always returns before the branch name, because "origin" is the literal ref namespace we
// queried — never derived from the output itself. Stripping this exact prefix (instead of
// cutting at the last '/') is what makes defaultBaseBranch correct for a default branch that
// itself contains a slash (e.g. "release/7.2"): LastIndexByte("refs/remotes/origin/release/7.2",
// '/') used to cut at "7.2", discarding "release/". Two consumers depend on this: release.go's
// commit-target resolution and this file's buildPRBody (via gitCommitsSince).
const gitSymbolicRefOriginHeadPrefix = "refs/remotes/origin/"

// defaultBaseBranch resolves the repository's default branch for `git log <base>..HEAD`.
// It tries `git symbolic-ref refs/remotes/origin/HEAD` and falls back to "main" when that fails
// or yields nothing (e.g. shallow clone without a remote-tracking HEAD). Same resolution
// pattern already used for branch/governance checks in internal/validator/validator.go.
//
// This is a LOCAL, gravable ref — trackfw release tag treats its result as a cross-check
// candidate only, never as the source of truth for the tag's commit target (the forge is the
// source; see ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md, Emenda 1).
func defaultBaseBranch(execGit func(args ...string) (string, error)) string {
	out, err := execGit("symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "main"
	}
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, gitSymbolicRefOriginHeadPrefix) {
		return "main"
	}
	name := out[len(gitSymbolicRefOriginHeadPrefix):]
	if name == "" {
		return "main"
	}
	return name
}

// buildPRBody constructs the PR/MR body. With 0 or 1 non-merge commit on the branch (the
// trivial case — just the commit `ship` itself made), it keeps the original minimal body,
// not a regression. With 2+ commits, it aggregates the branch's commit history:
//
//	## Commits
//	- <subject of commit 1>
//	- <subject of commit 2>
//
//	## Detalhes
//	<full body of each commit that has one, in blocks>
//
//	---
//	Branch: <branch>
func buildPRBody(branch string, commits []string) string {
	if len(commits) <= 1 {
		return fmt.Sprintf("Branch: %s\n\nCreated by trackfw ship.", branch)
	}

	var subjects []string
	var details []string
	for _, c := range commits {
		lines := strings.SplitN(c, "\n", 2)
		subject := strings.TrimSpace(lines[0])
		if subject == "" {
			continue
		}
		subjects = append(subjects, subject)
		if len(lines) > 1 {
			if bodyText := strings.TrimSpace(lines[1]); bodyText != "" {
				details = append(details, fmt.Sprintf("**%s**\n\n%s", subject, bodyText))
			}
		}
	}

	var b strings.Builder
	b.WriteString("## Commits\n\n")
	for _, s := range subjects {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	if len(details) > 0 {
		b.WriteString("\n## Detalhes\n\n")
		b.WriteString(strings.Join(details, "\n\n"))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n---\nBranch: %s\n", branch)
	return b.String()
}

// splitNonEmptyLines splits git output (e.g. `diff --cached --name-only`) into a slice of
// trimmed, non-empty lines. Returns nil for empty input.
func splitNonEmptyLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// allDocOnly returns true when there is at least one staged file and every staged file is
// doc-only: under docs/ or vault/ (path prefix), or has a .md extension. A single file
// outside that criterion makes it return false. Mirrors the doc-only exception documented
// in CLAUDE.md §7 ("Alteração doc-only (markdown, comentários)").
func allDocOnly(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if strings.HasPrefix(f, "docs/") || strings.HasPrefix(f, "vault/") || strings.HasSuffix(f, ".md") {
			continue
		}
		return false
	}
	return true
}

// buildForgeCreateArgs appends --title and --body (or --description for azure)
// to a copy of adapter.CLIArgs. Never mutates the adapter slice.
func buildForgeCreateArgs(adapter forge.Adapter, title, body string) []string {
	args := make([]string, len(adapter.CLIArgs))
	copy(args, adapter.CLIArgs)
	args = append(args, "--title", title)
	if adapter.Forge == "azure" {
		args = append(args, "--description", body)
	} else {
		args = append(args, "--body", body)
	}
	return args
}

// shipBranchPrefixes is the full vocabulary `trackfw ship` accepts on the branch name.
// feat/fix/refactor are gated on Step 2's branch_has_wip_roadmap governance check (a hard gate
// not affected by lenient mode); chore/docs are housekeeping types — already exempted from that
// gate by `trackfw branch new` and `trackfw commit` — and ship without it too. Mirrors
// branchValidTypes/branchGatedTypes in branch.go and commitGovernedPrefixes in commit.go.
var shipBranchPrefixes = []string{"feat/", "fix/", "refactor/", "chore/", "docs/"}

// shipGatedBranchPrefixes is the subset of shipBranchPrefixes that requires Step 2's
// branch_has_wip_roadmap governance check. Keep in sync with branchGatedTypes (branch.go) and
// commitGovernedPrefixes (commit.go).
var shipGatedBranchPrefixes = []string{"feat/", "fix/", "refactor/"}

// isShipBranch returns true when branch matches feat|fix|refactor|chore|docs/<slug>.
func isShipBranch(branch string) bool {
	return matchesBranchPrefix(branch, shipBranchPrefixes)
}

// isGatedShipBranch returns true when branch matches feat|fix|refactor/<slug> — the subset of
// isShipBranch's vocabulary that requires Step 2's branch_has_wip_roadmap governance check.
// chore/docs branches satisfy isShipBranch but return false here.
func isGatedShipBranch(branch string) bool {
	return matchesBranchPrefix(branch, shipGatedBranchPrefixes)
}

// matchesBranchPrefix returns true when branch starts with one of prefixes and has a non-empty
// slug after the prefix.
func matchesBranchPrefix(branch string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(branch, prefix) && len(branch) > len(prefix) {
			return true
		}
	}
	return false
}

// isGitWriteCmd returns true when the first arg is a git subcommand that
// modifies local or remote state (commit, push, fetch).
func isGitWriteCmd(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return gitWriteCommands[args[0]]
}

// detectPendingSquashMerges warns about branches that have genuinely pending work against
// origin/main. Non-blocking — prints only.
//
// Reuses evaluateBranchIntegration (branch_prune.go) — the same touched-files heuristic
// `trackfw branch prune` uses — instead of a naive bidirectional `git diff origin/main <branch>
// --stat`. The naive check false-positives on a branch that was squash-merged and is now merely
// stale (main advanced further afterwards): it always shows a non-empty diff even though nothing
// from the branch is actually missing from main. Only branchPruneDecisionPendingWork — genuine,
// unintegrated work — surfaces this warning; every other decision (no_own_work,
// content_identical, review_doc_config, no_merge_base, eval_error) is silently kept quiet, same
// posture the naive check had on error (skip, no warning).
func detectPendingSquashMerges(currentBranch string, gitExec func(...string) (string, error), out io.Writer) {
	remoteBranches, err := gitExec("branch", "-r", "--no-merged", "origin/main")
	if err != nil || strings.TrimSpace(remoteBranches) == "" {
		return
	}
	for _, raw := range strings.Split(remoteBranches, "\n") {
		candidate := strings.TrimSpace(raw)
		if candidate == "" || strings.Contains(candidate, "HEAD") {
			continue
		}
		// The short name is candidate with "origin/" stripped.
		shortName := strings.TrimPrefix(candidate, "origin/")
		if shortName == currentBranch {
			continue
		}
		eval := evaluateBranchIntegration(candidate, gitExec)
		if eval.Decision == branchPruneDecisionPendingWork {
			fmt.Fprintf(out, "Warning: branch %q appears to have unmerged changes vs origin/main.\n", shortName)
		}
	}
}

// buildPushArgs determines whether -u is needed and returns the full push args.
// Uses git rev-parse @{u} to detect upstream; exit ≠ 0 means no upstream.
func buildPushArgs(branch string, gitExec func(...string) (string, error)) []string {
	_, err := gitExec("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		// No upstream configured — push with -u to set it.
		return []string{"push", "-u", "origin", branch}
	}
	return []string{"push", "origin", branch}
}
