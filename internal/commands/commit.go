package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

// commitProtectedBranches lists branches where a direct `trackfw commit` is never allowed,
// regardless of governance state — mirrors the same hard rule `trackfw ship` already enforces.
var commitProtectedBranches = map[string]bool{
	"main":   true,
	"master": true,
}

// commitGovernedPrefixes lists the branch-type prefixes that require a matching roadmap in
// wip/ or done/ before a commit is allowed — the same vocabulary `trackfw branch new` and the
// branch_has_wip_roadmap governance rule already enforce.
var commitGovernedPrefixes = []string{"feat/", "fix/", "refactor/"}

// commitDeps holds injectable dependencies so runCommit can be tested without touching a real
// git repository or the real project layout on disk — mirrors branchNewDeps in branch.go.
type commitDeps struct {
	// loadConfig returns the project configuration (production: config.Load).
	loadConfig func() config.ProjectConfig
	// currentBranch returns the current branch name (production: defaultCurrentBranch, which
	// runs `git rev-parse --abbrev-ref HEAD`).
	currentBranch func() (string, error)
	// resolveWIPDirs / resolveDoneDirs resolve state directories from cfg (production:
	// validator.ResolveWIPDirs / validator.ResolveDoneDirs).
	resolveWIPDirs  func(config.ProjectConfig) []string
	resolveDoneDirs func(config.ProjectConfig) []string
	// matchSlug checks whether the normalized slug matches any roadmap found in wipDirs/doneDirs
	// (production: validator.BranchSlugMatchesRoadmap — the same logic `trackfw branch new` and
	// `trackfw validate` use).
	matchSlug func(slug string, wipDirs, doneDirs []string) (matched bool, candidates []string)
	// execGitCommit runs `git commit -m <message>` with inherited stdio, propagating Git's own
	// output and exit code literally (production: defaultGitCommit).
	execGitCommit func(message string) error
	out           io.Writer
}

func newCommitCmd() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit staged changes, gated on governance for feat/fix/refactor branches",
		Long: `trackfw commit is the missing intermediate step between raw 'git commit' and
'trackfw ship': it commits staged changes directly, but blocks the commit before it happens
when governance is missing, instead of letting it land and only catching it later:

  1. On 'main'/'master': always blocked — commit directly on the default branch is never
     permitted.
  2. On a feat/fix/refactor branch: requires a roadmap matching the branch slug already in
     wip/ or done/ — the exact matching logic 'trackfw branch new' and 'trackfw validate'
     already use. Without a match, blocks with the same governance orientation message.
  3. On any other branch (e.g. doc/housekeeping branches): allowed without requiring a
     roadmap — a warning is logged, but the commit proceeds.
  4. When allowed: runs 'git commit -m <message>', propagating Git's own output and exit
     status literally.

Create the governance artifacts first if this blocks you:
  trackfw req new "title"
  trackfw roadmap new "title"
  trackfw roadmap move <name> wip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Silence cobra's own error/usage printing — runCommit already writes a
			// complete, non-duplicated message to deps.out (or lets Git's own stderr through).
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			if strings.TrimSpace(message) == "" {
				return fmt.Errorf("commit message is required — use -m:\n  trackfw commit -m \"feat(<scope>): <description>\"")
			}

			deps := commitDeps{
				loadConfig:      config.Load,
				currentBranch:   defaultCurrentBranch,
				resolveWIPDirs:  validator.ResolveWIPDirs,
				resolveDoneDirs: validator.ResolveDoneDirs,
				matchSlug:       validator.BranchSlugMatchesRoadmap,
				execGitCommit:   defaultGitCommit,
				out:             cmd.OutOrStdout(),
			}
			return runCommit(message, deps)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (required)")

	return cmd
}

// defaultCurrentBranch runs `git rev-parse --abbrev-ref HEAD` and returns the trimmed branch
// name. Reuses defaultGitExec (ship.go) instead of duplicating exec.Command wiring.
func defaultCurrentBranch() (string, error) {
	return defaultGitExec("rev-parse", "--abbrev-ref", "HEAD")
}

// defaultGitCommit runs `git commit -m <message>` with inherited stdio, so Git's own output
// reaches the user unmodified. Mirrors defaultGitCheckout in branch.go: on failure with a
// process exit, it exits the process directly with Git's own exit code instead of returning the
// error, so cobra never prints a redundant "exit status N" line on top of Git's own diagnostic.
func defaultGitCommit(message string) error {
	c := exec.Command("git", "commit", "-m", message)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	return err
}

// runCommit implements the `trackfw commit -m "<message>"` flow described in ML-2A of
// docs/roadmaps/wip/ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md.
func runCommit(message string, deps commitDeps) error {
	branch, err := deps.currentBranch()
	if err != nil {
		return fmt.Errorf("could not determine current branch (are you in a git repo?): %w", err)
	}
	branch = strings.TrimSpace(branch)

	// (a) main/master: always blocked.
	if commitProtectedBranches[branch] {
		msg := fmt.Sprintf(
			"trackfw commit: commit direto em %q não é permitido. Use 'trackfw branch new <type>/<slug>' primeiro. Ver CLAUDE.md §1.",
			branch,
		)
		fmt.Fprintln(deps.out, msg)
		return fmt.Errorf("blocked: commit directly on %q is not permitted", branch)
	}

	// (b) feat/fix/refactor: require a matching roadmap in wip/ or done/.
	governedPrefix, isGoverned := commitGovernedBranchPrefix(branch)
	if isGoverned {
		slug := strings.TrimPrefix(branch, governedPrefix)
		cfg := deps.loadConfig()
		wipDirs := deps.resolveWIPDirs(cfg)
		doneDirs := deps.resolveDoneDirs(cfg)

		normalizedSlug := validator.NormalizeBranchSlug(slug)
		matched, candidates := deps.matchSlug(normalizedSlug, wipDirs, doneDirs)

		if !matched {
			var msg string
			if len(candidates) == 0 {
				msg = validator.BranchGovernanceOrientation(branch)
			} else {
				msg = validator.BranchNoMatchingRoadmapMessage(branch, candidates)
			}
			fmt.Fprintln(deps.out, msg)
			return fmt.Errorf("blocked: no matching roadmap in wip/ nor done/ for %q", branch)
		}
	} else {
		// (c) branches outside the feat/fix/refactor pattern (e.g. doc/housekeeping branches):
		// allow without requiring a roadmap, but warn.
		fmt.Fprintf(deps.out, "trackfw commit: branch %q does not follow feat/fix/refactor — committing without a roadmap check.\n", branch)
	}

	// (d) passed all checks: commit.
	return deps.execGitCommit(message)
}

// commitGovernedBranchPrefix returns the matched prefix (e.g. "feat/") and whether branch starts
// with one of commitGovernedPrefixes.
func commitGovernedBranchPrefix(branch string) (prefix string, matched bool) {
	for _, p := range commitGovernedPrefixes {
		if strings.HasPrefix(branch, p) {
			return p, true
		}
	}
	return "", false
}
