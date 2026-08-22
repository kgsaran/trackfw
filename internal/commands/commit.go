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
	// stagedNameStatus returns the raw output of `git diff --cached --name-status`
	// (production: defaultStagedNameStatus). Used only by buildSuggestedMessage — never touched
	// by the normal `-m` commit flow.
	stagedNameStatus func() (string, error)
	out              io.Writer
}

func newCommitCmd() *cobra.Command {
	var message string
	var suggest bool

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit staged changes, gated on governance for feat/fix/refactor branches",
		Long: `trackfw commit commits staged changes directly, but blocks the commit before it
happens when governance is missing, instead of letting it land and only catching it later.

Compositional vocabulary:
  trackfw commit -m "..."   commits
  trackfw push              pushes
  trackfw ship -m "..."     commit + push + PR (composition)

Behavioral steps:

  1. On 'main'/'master': always blocked — commit directly on the default branch is never
     permitted.
  2. On a feat/fix/refactor branch: requires a roadmap matching the branch slug already in
     wip/ or done/ — the exact matching logic 'trackfw branch new' and 'trackfw validate'
     already use. Without a match, blocks with the same governance orientation message.
  3. On any other branch (e.g. doc/housekeeping branches): allowed without requiring a
     roadmap — a warning is logged, but the commit proceeds.
  4. When allowed: runs 'git commit -m <message>', propagating Git's own output and exit
     status literally.

'--suggest' takes a completely separate path: it prints a heuristic Conventional Commits
skeleton built from 'git diff --cached --name-status' (type + staged file list) and exits
without ever committing — no LLM call, just a structural heuristic. It is not a ready-to-use
message; review and edit before using it with -m. When '--suggest' is set, '-m' (if also
passed) is ignored and no commit ever happens.

Create the governance artifacts first if this blocks you:
  trackfw req new "title"
  trackfw roadmap new "title"
  trackfw roadmap move <name> wip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Silence cobra's own error/usage printing — runCommit already writes a
			// complete, non-duplicated message to deps.out (or lets Git's own stderr through).
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			deps := commitDeps{
				loadConfig:       config.Load,
				currentBranch:    defaultCurrentBranch,
				resolveWIPDirs:   validator.ResolveWIPDirs,
				resolveDoneDirs:  validator.ResolveDoneDirs,
				matchSlug:        validator.BranchSlugMatchesRoadmap,
				execGitCommit:    defaultGitCommit,
				stagedNameStatus: defaultStagedNameStatus,
				out:              cmd.OutOrStdout(),
			}

			if suggest {
				suggestion, err := buildSuggestedMessage(deps)
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, suggestion)
				return nil
			}

			if strings.TrimSpace(message) == "" {
				return fmt.Errorf("commit message is required — use -m:\n  trackfw commit -m \"feat(<scope>): <description>\"")
			}

			return runCommit(message, deps)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (required)")
	cmd.Flags().BoolVar(&suggest, "suggest", false, "Print a heuristic Conventional Commits message skeleton from staged files and exit without committing (ignores -m)")

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

// defaultStagedNameStatus runs `git diff --cached --name-status` and returns its raw output.
// Reuses defaultGitExec (ship.go) instead of duplicating exec.Command wiring.
func defaultStagedNameStatus() (string, error) {
	return defaultGitExec("diff", "--cached", "--name-status")
}

// stagedFile is one line of `git diff --cached --name-status` output: a status letter
// (A/M/D/...) and the file path it refers to.
type stagedFile struct {
	status string
	path   string
}

// parseStagedNameStatus parses raw `git diff --cached --name-status` output (tab-separated
// "<status>\t<path>" lines) into stagedFile entries, skipping blank lines.
func parseStagedNameStatus(raw string) []stagedFile {
	var files []stagedFile
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		files = append(files, stagedFile{status: strings.TrimSpace(parts[0]), path: parts[1]})
	}
	return files
}

// commitCommandDirs lists the directories (across the 3 supported CLIs) where a new (status
// "A") file signals a new CLI command was added — used by the "feat" heuristic rule below.
var commitCommandDirs = []string{
	"internal/commands/",
	"npm/src/commands/",
	"pypi/trackfw/commands/",
}

// suggestedCommitType returns the Conventional Commits type suggested for a set of staged
// files, following the fixed-priority heuristic documented in ML-1A of
// docs/roadmaps/wip/ROADMAP-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md
// (first matching rule wins — this is a deliberately simple heuristic, not an attempt at
// perfect classification):
//  1. every staged file matches a test-file pattern (*_test.go, *.test.js, test_*.py,
//     *_test.py) -> "test"
//  2. every staged file is under docs/ or vault/, or has a .md extension -> "docs"
//  3. at least one new ("A") file lives under one of commitCommandDirs -> "feat"
//  4. otherwise -> "fix"
func suggestedCommitType(files []stagedFile) string {
	allTests := true
	allDocs := true
	hasNewCommandFile := false

	for _, f := range files {
		if !isTestFile(f.path) {
			allTests = false
		}
		if !isDocsFile(f.path) {
			allDocs = false
		}
		if f.status == "A" && isUnderAnyDir(f.path, commitCommandDirs) {
			hasNewCommandFile = true
		}
	}

	switch {
	case allTests:
		return "test"
	case allDocs:
		return "docs"
	case hasNewCommandFile:
		return "feat"
	default:
		return "fix"
	}
}

// isTestFile reports whether path matches one of the recognized test-file naming conventions
// across the 3 supported stacks: *_test.go, *.test.js, test_*.py, *_test.py.
func isTestFile(path string) bool {
	base := path
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		base = path[idx+1:]
	}
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return true
	case strings.HasSuffix(base, ".test.js"):
		return true
	case strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"):
		return true
	case strings.HasSuffix(base, "_test.py"):
		return true
	default:
		return false
	}
}

// isDocsFile reports whether path lives under docs/ or vault/, or has a .md extension.
func isDocsFile(path string) bool {
	if strings.HasPrefix(path, "docs/") || strings.HasPrefix(path, "vault/") {
		return true
	}
	return strings.HasSuffix(path, ".md")
}

// isUnderAnyDir reports whether path starts with any of the given directory prefixes.
func isUnderAnyDir(path string, dirs []string) bool {
	for _, d := range dirs {
		if strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

// buildSuggestedMessage implements `trackfw commit --suggest`: it reads the staged diff via
// deps.stagedNameStatus, classifies it with suggestedCommitType, and renders the heuristic
// Conventional Commits skeleton described in ML-1A. It never calls deps.execGitCommit — no
// commit ever happens as a side effect of this function.
func buildSuggestedMessage(deps commitDeps) (string, error) {
	raw, err := deps.stagedNameStatus()
	if err != nil {
		return "", fmt.Errorf("could not read staged changes (are you in a git repo?): %w", err)
	}

	files := parseStagedNameStatus(raw)
	if len(files) == 0 {
		return "", fmt.Errorf("nothing staged — `git add` files first")
	}

	commitType := suggestedCommitType(files)

	var b strings.Builder
	fmt.Fprintln(&b, "# Sugestão heurística — NÃO é uma mensagem pronta, revise antes de usar.")
	fmt.Fprintf(&b, "# Tipo sugerido: %s\n", commitType)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%s(<escopo>): <descrição>\n", commitType)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Arquivos staged")
	for _, f := range files {
		fmt.Fprintf(&b, "%s  %s\n", f.status, f.path)
	}

	return strings.TrimRight(b.String(), "\n"), nil
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
