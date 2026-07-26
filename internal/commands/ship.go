package commands

import (
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
}

// shipOpts holds the parsed CLI flags for the ship command.
type shipOpts struct {
	message string
	dryRun  bool
	noPR    bool   // --no-pr: skip PR/MR creation after push
	forge   string // --forge: override forge detection
}

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

	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Governed git commit + push for feat/fix/refactor branches",
		Long: `trackfw ship runs a governed delivery sequence:

  1. Validates branch name — must match feat|fix|refactor/<slug>
  2. Validates governance — REQ + roadmap in wip/ must exist
     (hard gate: not affected by lenient mode or per-rule severity)
  3. Detects pending squash-merges in other branches (advisory only)
  4. Reviews what is staged (git status --short + git diff --cached --stat)
  5. Commits with Conventional Commits format (-m is required)
  6. Pushes to origin (adds -u if no upstream is configured yet)
  7. Opens PR/MR via the resolved forge CLI (or prints URL if CLI is absent)

Stage your files explicitly before running ship.
This command never executes 'git add .' or 'git add -A'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			deps := shipDeps{
				execGit:         defaultGitExec,
				checkGovernance: defaultCheckGovernance,
				out:             cmd.OutOrStdout(),
				configForge:     config.Load().Forge,
				repoDir:         ".",
				availFn:         nil, // forge.NewAdapter uses its own default when nil
				execForgeCLI:    defaultExecForgeCLI,
			}
			return runShip(shipOpts{
				message: message,
				dryRun:  dryRun,
				noPR:    noPR,
				forge:   forgeFlag,
			}, deps)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (Conventional Commits format required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be done without executing write commands")
	cmd.Flags().BoolVar(&noPR, "no-pr", false, "Skip PR/MR creation after push")
	cmd.Flags().StringVar(&forgeFlag, "forge", "", "Override forge detection (github, gitlab, bitbucket, azure)")

	return cmd
}

// defaultGitExec is the production git executor.
// It runs "git <args...>" and returns trimmed stdout.
func defaultGitExec(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// defaultExecForgeCLI runs the forge CLI (gh, glab, az) with inherited stdin/stdout/stderr.
func defaultExecForgeCLI(name string, args []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

	// ─── Step 1: Branch validation ───────────────────────────────────────────
	branch, err := deps.execGit("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("could not determine current branch (are you in a git repo?): %w", err)
	}
	branch = strings.TrimSpace(branch)

	if branch == "main" || branch == "master" {
		return fmt.Errorf(
			"trackfw ship cannot run on %q — use a feature branch:\n  git checkout -b feat/<slug>",
			branch,
		)
	}

	if !isShipBranch(branch) {
		return fmt.Errorf(
			"branch %q does not match the required pattern feat|fix|refactor/<slug>\n"+
				"Rename your branch or create a new one:\n  git checkout -b feat/<slug>",
			branch,
		)
	}

	fmt.Fprintf(deps.out, "Branch: %s\n", branch)

	// ─── Step 2: Governance ──────────────────────────────────────────────────
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

	cachedFiles, _ := deps.execGit("diff", "--cached", "--name-only")
	if strings.TrimSpace(cachedFiles) == "" {
		return fmt.Errorf(
			"nothing is staged — stage your files explicitly before running ship:\n" +
				"  git add <file1> <file2> ...\n" +
				"Never use 'git add .' or 'git add -A'",
		)
	}

	// ─── Step 5: Commit ──────────────────────────────────────────────────────
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

	// ─── Step 6: Push ────────────────────────────────────────────────────────
	pushArgs := buildPushArgs(branch, deps.execGit)
	if _, err := git(pushArgs...); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	if !opts.dryRun {
		fmt.Fprintf(deps.out, "Pushed:    %s → origin/%s\n", branch, branch)
	}

	// ─── Step 7: Open PR/MR ──────────────────────────────────────────────────
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

	if opts.dryRun {
		fmt.Fprintf(deps.out, "[dry-run] would open %s via %s\n", adapter.Noun, resolution.Forge)
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
	title := firstLine(opts.message)
	body := buildPRBody(branch)
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

// buildPRBody constructs a minimal PR/MR body referencing the branch.
func buildPRBody(branch string) string {
	return fmt.Sprintf("Branch: %s\n\nCreated by trackfw ship.", branch)
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

// isShipBranch returns true when branch matches feat|fix|refactor/<slug>.
func isShipBranch(branch string) bool {
	for _, prefix := range []string{"feat/", "fix/", "refactor/"} {
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

// detectPendingSquashMerges warns about branches that have non-empty diffs
// against origin/main. Non-blocking — prints only.
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
		diff, derr := gitExec("diff", "origin/main", candidate, "--stat")
		if derr != nil {
			continue
		}
		if strings.TrimSpace(diff) != "" {
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
