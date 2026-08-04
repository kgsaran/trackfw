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

// branchValidTypes mirrors the vocabulary already enforced by `trackfw ship` (feat|fix|refactor).
var branchValidTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"refactor": true,
}

// branchNewDeps holds injectable dependencies so runBranchNew can be tested without touching a
// real git repository or the real project layout on disk.
type branchNewDeps struct {
	// loadConfig returns the project configuration (production: config.Load).
	loadConfig func() config.ProjectConfig
	// resolveWIPDirs / resolveDoneDirs resolve state directories from cfg (production:
	// validator.ResolveWIPDirs / validator.ResolveDoneDirs).
	resolveWIPDirs  func(config.ProjectConfig) []string
	resolveDoneDirs func(config.ProjectConfig) []string
	// matchSlug checks whether the normalized slug matches any roadmap found in wipDirs/doneDirs
	// (production: validator.BranchSlugMatchesRoadmap — the same logic `trackfw validate` uses).
	matchSlug func(slug string, wipDirs, doneDirs []string) (matched bool, candidates []string)
	// execGitCheckout runs `git checkout -b <branchName>` with inherited stdio, propagating Git's
	// own output and exit code literally (production: defaultGitCheckout).
	execGitCheckout func(branchName string) error
	out             io.Writer
}

func newBranchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Manage governed feature branches",
	}
	cmd.AddCommand(newBranchNewCmd())
	return cmd
}

func newBranchNewCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "new <type>/<slug>",
		Short: "Create a feat/fix/refactor branch, gated on a matching REQ + roadmap already in wip/ or done/",
		Long: `trackfw branch new moves the branch_has_wip_roadmap governance gate (already enforced
by 'trackfw validate' and 'trackfw ship') to before branch creation, instead of after:

  1. Validates <type> is one of feat, fix, refactor and <slug> is non-empty.
  2. Checks whether a roadmap in wip/ or done/ matches the given slug — the exact matching logic
     'trackfw validate' already uses (normalized slug, filename contains match).
  3. Without a match: blocks — 'git checkout -b' is never executed — and prints the same governance
     orientation message 'trackfw validate' already prints for this rule.
  4. With a match: runs 'git checkout -b <type>/<slug>', propagating Git's own output and exit
     status literally.

Create the governance artifacts first if this blocks you:
  trackfw req new "title"
  trackfw roadmap new "title"
  trackfw roadmap move <name> wip`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Silence cobra's own error/usage printing — runBranchNew already writes a
			// complete, non-duplicated message to deps.out (or lets Git's own stderr through).
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			deps := branchNewDeps{
				loadConfig:      config.Load,
				resolveWIPDirs:  validator.ResolveWIPDirs,
				resolveDoneDirs: validator.ResolveDoneDirs,
				matchSlug:       validator.BranchSlugMatchesRoadmap,
				execGitCheckout: defaultGitCheckout,
				out:             cmd.OutOrStdout(),
			}
			return runBranchNew(args[0], dryRun, deps)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report whether the branch would be created or blocked, without executing git")

	return cmd
}

// defaultGitCheckout runs `git checkout -b <branchName>` with inherited stdio, so Git's own
// output (including branch-already-exists errors) reaches the user unmodified.
func defaultGitCheckout(branchName string) error {
	c := exec.Command("git", "checkout", "-b", branchName)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// runBranchNew implements the `trackfw branch new <type>/<slug>` flow described in
// docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md.
func runBranchNew(spec string, dryRun bool, deps branchNewDeps) error {
	branchType, slug, err := parseBranchSpec(spec)
	if err != nil {
		return err
	}

	branchName := branchType + "/" + slug

	cfg := deps.loadConfig()
	wipDirs := deps.resolveWIPDirs(cfg)
	doneDirs := deps.resolveDoneDirs(cfg)

	normalizedSlug := validator.NormalizeBranchSlug(slug)
	matched, candidates := deps.matchSlug(normalizedSlug, wipDirs, doneDirs)

	if !matched {
		var msg string
		if len(candidates) == 0 {
			msg = validator.BranchGovernanceOrientation(branchName)
		} else {
			msg = validator.BranchNoMatchingRoadmapMessage(branchName, candidates)
		}
		if dryRun {
			fmt.Fprintf(deps.out, "[dry-run] would block: %s\n", msg)
		} else {
			fmt.Fprintln(deps.out, msg)
		}
		return fmt.Errorf("blocked: no matching roadmap in wip/ nor done/ for %q", branchName)
	}

	if dryRun {
		fmt.Fprintf(deps.out, "[dry-run] would create branch %q (git checkout -b %s)\n", branchName, branchName)
		return nil
	}

	return deps.execGitCheckout(branchName)
}

// parseBranchSpec splits "<type>/<slug>" and validates both parts. type must be one of
// feat, fix, refactor (branchValidTypes); slug must be non-empty.
func parseBranchSpec(spec string) (branchType, slug string, err error) {
	parts := strings.SplitN(spec, "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", fmt.Errorf(
			"invalid branch spec %q — expected <type>/<slug> with type in feat, fix, refactor",
			spec,
		)
	}
	branchType, slug = parts[0], parts[1]
	if !branchValidTypes[branchType] {
		return "", "", fmt.Errorf(
			"invalid branch type %q — must be one of feat, fix, refactor",
			branchType,
		)
	}
	if strings.TrimSpace(slug) == "" {
		return "", "", fmt.Errorf("branch slug is required — expected <type>/<slug>, got %q", spec)
	}
	return branchType, slug, nil
}
