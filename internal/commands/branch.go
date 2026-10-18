package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/netdeadline"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/spf13/cobra"
)

// branchValidTypes is the full vocabulary accepted by `trackfw branch new`. feat/fix/refactor are
// gated on a matching REQ + roadmap already in wip/ (branchGatedTypes below); chore/docs
// are housekeeping types — already treated as roadmap-exempt by `trackfw ship` and `trackfw
// commit` — and create the branch without that gate.
var branchValidTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"refactor": true,
	"chore":    true,
	"docs":     true,
}

// branchGatedTypes is the subset of branchValidTypes that requires a matching REQ + roadmap
// already in wip/ before the branch is created. Keep this in sync with the pattern
// `trackfw ship`/`trackfw commit` use to decide when the branch_has_wip_roadmap gate applies.
var branchGatedTypes = map[string]bool{
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
	// matchSlug checks whether the normalized slug matches any roadmap found in wipDirs only
	// (production: validator.BranchSlugMatchesRoadmap — the same logic `trackfw validate` uses).
	// D1 of ADR-2026-10-01: branch creation is gated on wip/ only; doneDirs must be nil/empty.
	matchSlug func(slug string, wipDirs, doneDirs []string) (matched bool, candidates []string)
	// matchDone returns roadmap filenames in doneDirs that match the slug. Used only for the
	// orientation message hint (D1: done/ roadmaps need `roadmap move` before `branch new`).
	// nil skips the hint (pre-existing tests that don't exercise this path).
	matchDone func(slug string, doneDirs []string) []string
	// execGitCheckout runs `git checkout --no-track -b <branchName> [startPoint]` with inherited
	// stdio, propagating Git's own output and exit code literally. startPoint is "" when
	// reproducing the old (HEAD-based) behaviour, e.g. for --from HEAD.
	// (production: defaultGitCheckout).
	execGitCheckout func(branchName, startPoint string) error
	// recordLink writes the branch↔roadmap link after the branch is created (production:
	// validator.RecordBranchLink). 🔴 This is D1 of ADR-2026-09-26: `branch new` KNOWS which roadmap
	// governs the branch at this exact instant, so it writes it down instead of leaving every later
	// `commit`/`ship`/`validate` to re-infer it from the two names. nil skips the recording, which
	// is what the pre-existing unit tests do — the link is an accelerator, never a gate.
	recordLink func(cfg config.ProjectConfig, branch string) error
	// fromRef holds the value of the --from flag: "" means resolve from origin/<default>,
	// "HEAD" means use the current HEAD (old behaviour — no start point passed to git),
	// any other value is used as start point directly.
	fromRef string
	// gitExec runs `git <args...>` and returns (trimmed-stdout, error). Production: defaultGitExec.
	// Used for fetch, remote get-url, symbolic-ref and rev-parse. nil when fromRef != "" (no
	// remote resolution needed) so that pre-existing unit tests don't have to provide it.
	gitExec func(args ...string) (string, error)
	// ghExec runs `gh <args...>` and returns raw stdout bytes. Production: defaultGhExec, nil in
	// tests. Used only when fromRef == "" to resolve the forge default branch name.
	ghExec ghExecFn
	out    io.Writer
}

func newBranchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Manage governed feature branches",
	}
	cmd.AddCommand(newBranchNewCmd())
	cmd.AddCommand(newBranchPruneCmd())
	return cmd
}

func newBranchNewCmd() *cobra.Command {
	var dryRun bool
	var from string

	cmd := &cobra.Command{
		Use:   "new <type>/<slug>",
		Short: "Create a feat/fix/refactor/chore/docs branch; feat/fix/refactor gated on a matching REQ + roadmap already in wip/",
		Long: `trackfw branch new moves the branch_has_wip_roadmap governance gate (already enforced
by 'trackfw validate' and 'trackfw ship') to before branch creation, instead of after:

  1. Validates <type> is one of feat, fix, refactor, chore, docs and <slug> is non-empty.
  2. For feat, fix, refactor: checks whether a roadmap in wip/ matches the given slug —
     the exact matching logic 'trackfw validate' already uses (normalized slug, filename contains
     match). Without a wip/ match: blocks — 'git checkout -b' is never executed — and prints the
     governance orientation message. If done/ has matching roadmaps, the message names them and
     suggests 'trackfw roadmap move <name> wip' (D1 of ADR-2026-10-01).
  3. For chore, docs: housekeeping types already treated as roadmap-exempt by 'trackfw ship' and
     'trackfw commit' — the branch is created without the roadmap gate.
  4. Fetches origin and resolves the default remote branch (e.g. origin/main) as the start
     point. If fetch fails, warns and uses the last locally-known state. Use --from HEAD to
     reproduce the old behaviour (start from current HEAD) or --from <ref> to specify any ref.
  5. Runs 'git checkout --no-track -b <type>/<slug> [<start-point>]', propagating Git's own
     output and exit status literally.

Create the governance artifacts first if this blocks you:
  trackfw req new "title"               # flat project
  trackfw req new --agent <agent> "title"  # by_agent project with 2+ agents
  trackfw roadmap new "title"               # flat project
  trackfw roadmap new --agent <agent> "title"  # by_agent project with 2+ agents
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
				matchDone: func(slug string, doneDirs []string) []string {
					matches, _ := validator.MatchRoadmapsForBranchSlug(slug, nil, doneDirs)
					return matches
				},
				execGitCheckout: defaultGitCheckout,
				recordLink:      validator.RecordBranchLink,
				fromRef:         from,
				gitExec:         defaultGitExec,
				ghExec:          defaultGhExec,
				out:             cmd.OutOrStdout(),
			}
			return runBranchNew(args[0], dryRun, deps)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report whether the branch would be created or blocked, without executing git")
	cmd.Flags().StringVar(&from, "from", "", "Start point for the branch (default: origin/<default-branch>; use HEAD to create from current HEAD)")

	return cmd
}

// defaultGitCheckout runs `git checkout --no-track -b <branchName> [startPoint]` with inherited
// stdio, so Git's own output (including branch-already-exists errors) reaches the user unmodified.
//
// --no-track is mandatory (A-5/F-5 of ROADMAP-2026-09-12): without it, when a start point is
// given git configures @{u}=origin/<default>, which causes `trackfw push` to skip -u and leave
// the upstream permanently pointing at origin/<default> instead of origin/<branch>. The Passo 0
// protocol in CLAUDE.md §1 uses [origin/X: gone] as the signal for merged branches; without -u
// the signal never appears after the PR is merged and the remote branch deleted.
//
// startPoint is "" when the caller wants to create from the current HEAD (--from HEAD, or the
// pre-v10 behaviour in repos without an origin). Passing HEAD explicitly fails on unborn repos.
//
// When git exits with code 1 and its stderr contains "would be overwritten by checkout", an
// additional actionable hint is printed before the process exits (dirty-tree conflict with the
// start point — only possible when startPoint != "").
//
// On failure with a process exit (the common case — e.g. branch already exists), it exits the
// process directly with Git's own exit code instead of returning the error: Git has already
// written its diagnostic to the inherited stderr, and letting the error propagate back through
// cobra would make Execute() print Go's own exec.ExitError string ("exit status 128") as an
// extra, redundant line that Git itself never produced — breaking the "propagate literally"
// contract this command promises. A non-ExitError failure (e.g. the git binary is missing) has
// no Git-produced diagnostic to rely on, so it still returns normally to be reported the usual way.
func defaultGitCheckout(branchName, startPoint string) error {
	args := []string{"checkout", "--no-track", "-b", branchName}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	c := exec.Command("git", args...) //nolint:gosec
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	// Capture stderr via MultiWriter so Git's output still reaches the user
	// AND we can inspect it to emit the dirty-tree hint.
	var stderrBuf bytes.Buffer
	c.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err := c.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		// Dirty-tree hint: git exits 1 with an "overwritten" message when the start
		// point contains a different version of a file with uncommitted changes.
		if exitErr.ExitCode() == 1 && startPoint != "" &&
			strings.Contains(stderrBuf.String(), "would be overwritten") {
			fmt.Fprintf(os.Stderr, "\nhint: commite ou faça stash das suas alterações antes de trocar a base, ou use --from HEAD para criar a branch a partir do HEAD atual sem mover arquivos.\n")
		}
		os.Exit(exitErr.ExitCode())
	}
	return err
}

// runBranchNew implements the `trackfw branch new <type>/<slug>` flow described in
// docs/req/REQ-2026-09-12-*.md and REQ-2026-08-04-*.md.
func runBranchNew(spec string, dryRun bool, deps branchNewDeps) error {
	branchType, slug, err := parseBranchSpec(spec)
	if err != nil {
		return err
	}

	branchName := branchType + "/" + slug

	// chore/docs are housekeeping types — already treated as roadmap-exempt by `trackfw ship`
	// and `trackfw commit` — so the branch_has_wip_roadmap gate below does not apply to them.
	var cfg config.ProjectConfig
	if branchGatedTypes[branchType] {
		cfg = deps.loadConfig()
		wipDirs := deps.resolveWIPDirs(cfg)
		doneDirs := deps.resolveDoneDirs(cfg)

		normalizedSlug := validator.NormalizeBranchSlug(slug)
		// D1: gate checks wip/ only. done/ roadmaps need `trackfw roadmap move` first.
		matched, candidates := deps.matchSlug(normalizedSlug, wipDirs, nil)

		if !matched {
			// Compute done matches for the hint in the orientation message.
			var doneMatches []string
			if deps.matchDone != nil {
				doneMatches = deps.matchDone(normalizedSlug, doneDirs)
			}
			var msg string
			if len(candidates) == 0 {
				msg = validator.BranchGovernanceOrientationForCreation(branchName, cfg, doneMatches)
			} else {
				msg = validator.BranchNoMatchingRoadmapMessageForCreation(branchName, candidates, doneMatches)
			}
			if dryRun {
				fmt.Fprintf(deps.out, "[dry-run] would block: %s\n", msg)
			} else {
				fmt.Fprintln(deps.out, msg)
			}
			return fmt.Errorf("blocked: no matching roadmap in wip/ for %q", branchName)
		}
	}

	// Resolve the start point for the branch.
	// - fromRef == "HEAD": reproduce old behaviour (no start point, creates from current HEAD).
	// - fromRef != "" and != "HEAD": validate, resolve to SHA, return SHA + original ref as display.
	// - fromRef == "" (default): fetch origin, resolve the default remote branch, return SHA + name.
	startPoint, displayRef, err := resolveBranchStartPoint(deps, branchName)
	if err != nil {
		// Dry-run: governance passed, so the branch WOULD be created — but the base could not be
		// resolved (e.g. no origin). Report "would create" (governance result) and append the base
		// warning so the user knows the real execution would need --from HEAD or an origin remote.
		// Exit 0: the gate measures governance, not base availability; a branch that passes
		// governance must print "would create" regardless of base state.
		if dryRun {
			fmt.Fprintf(deps.out, "[dry-run] would create branch %q (base não resolvida: %s; a execução real requer --from HEAD ou um remote 'origin')\n", branchName, err.Error())
			return nil
		}
		return err
	}

	if dryRun {
		fmt.Fprintf(deps.out, "[dry-run] would create branch %q from %s\n", branchName, displayRef)
		return nil
	}

	if err := deps.execGitCheckout(branchName, startPoint); err != nil {
		return err
	}

	// The link is recorded ONLY for gated types and ONLY after Git created the branch: recording a
	// link for a branch that does not exist would be state about nothing. A failure here is
	// reported and swallowed — the branch exists, the gate passed, and losing the accelerator must
	// not turn a successful creation into an error the user cannot act on.
	if branchGatedTypes[branchType] && deps.recordLink != nil {
		if err := deps.recordLink(cfg, branchName); err != nil {
			fmt.Fprintf(deps.out, "trackfw branch new: branch created, but the branch↔roadmap link could not be recorded: %v\n", err)
		}
	}
	return nil
}

// resolveBranchStartPoint resolves the git start point for the new branch based on
// deps.fromRef, performing fetch + resolveDefaultBranch when fromRef == "".
//
// Returns (startPoint, displayRef, error):
//   - startPoint: value to pass to git checkout (canonical SHA, or "" for HEAD-based creation).
//   - displayRef: human-readable name for user messages ("HEAD", "origin/main", user's --from value).
//   - error: non-nil when resolution fails and branch creation must be blocked.
func resolveBranchStartPoint(deps branchNewDeps, branchName string) (startPoint, displayRef string, err error) {
	switch {
	case deps.fromRef == "HEAD":
		// Explicit: create from current HEAD (no start point). Reproduces the pre-v10.0.0 behaviour.
		return "", "HEAD", nil
	case deps.fromRef != "":
		// Explicit ref — validate before touching git (B-1: reject flag-like values that git
		// would interpret as options, silently discarding uncommitted work via e.g. -f/--force).
		if strings.HasPrefix(deps.fromRef, "-") {
			return "", "", fmt.Errorf(
				"trackfw branch new: --from %q: a ref não pode começar com '-' — use o nome de uma branch, tag ou SHA",
				deps.fromRef,
			)
		}
		if deps.gitExec == nil {
			// Test mode: no git available; return the ref as-is (unit tests inject the mock).
			return deps.fromRef, deps.fromRef, nil
		}
		// Resolve to a canonical SHA so the checkout receives a stable, flag-safe value.
		// --end-of-options separates our ref argument from any flags git might interpret.
		sha, resolveErr := deps.gitExec("rev-parse", "--verify", "--quiet", "--end-of-options", deps.fromRef+"^{commit}")
		if resolveErr != nil {
			return "", "", fmt.Errorf(
				"trackfw branch new: --from %q: a ref não foi encontrada ou não é um commit válido",
				deps.fromRef,
			)
		}
		return strings.TrimSpace(sha), deps.fromRef, nil
	}

	// Default path: fetch origin, resolve default branch, verify ref, return SHA.
	// Requires deps.gitExec to be set (production wiring; tests that set fromRef="HEAD" skip this).
	if deps.gitExec == nil {
		// Safety: if gitExec was not provided and fromRef is empty, fall back to HEAD behaviour.
		// This should not happen in production (RunE always wires defaultGitExec).
		return "", "HEAD", nil
	}

	// Check that origin exists before attempting fetch or symref resolution.
	if _, err := deps.gitExec("remote", "get-url", "origin"); err != nil {
		// Single message, in Portuguese. Execute() in root.go prints err.Error() to stderr.
		return "", "", fmt.Errorf(
			"trackfw branch new: nenhum remote 'origin' encontrado — use --from HEAD para criar"+
				" a partir do HEAD atual, ou adicione um remote com 'git remote add origin <url>'.",
		)
	}

	// Best-effort fetch (R-1: always fetch, including --dry-run, so both paths resolve the same
	// default branch for remotes whose origin/HEAD is not locally set yet). Only ref objects
	// are updated; working tree is not touched.
	if _, err := deps.gitExec("fetch", "origin"); err != nil {
		if errors.Is(err, netdeadline.ErrTimeout) {
			fmt.Fprintf(deps.out, "Warning: could not fetch origin (%s) — creating branch from possibly stale remote ref.\n", err.Error())
		} else {
			fmt.Fprintf(deps.out, "Warning: could not fetch origin (offline, no remote, or fetch failed) — creating branch from possibly stale remote ref.\n")
		}
	}

	// Resolve the default branch (forge API → symref → fallback "main").
	_, remoteRef := resolveDefaultBranch(deps.gitExec, deps.ghExec)

	// Resolve to a canonical SHA (B-1: checkout receives a stable, flag-safe value).
	sha, resolveErr := deps.gitExec("rev-parse", "--verify", "-q", "--end-of-options", remoteRef+"^{commit}")
	if resolveErr != nil {
		return "", "", fmt.Errorf(
			"trackfw branch new: a ref base '%s' não foi encontrada — verifique se o remote foi"+
				" configurado corretamente e tente 'git fetch origin', ou use --from HEAD para criar"+
				" a partir do HEAD atual.",
			remoteRef,
		)
	}

	return strings.TrimSpace(sha), remoteRef, nil
}

// parseBranchSpec splits "<type>/<slug>" and validates both parts. type must be one of
// feat, fix, refactor, chore, docs (branchValidTypes); slug must be non-empty.
func parseBranchSpec(spec string) (branchType, slug string, err error) {
	parts := strings.SplitN(spec, "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", fmt.Errorf(
			"invalid branch spec %q — expected <type>/<slug> with type in feat, fix, refactor, chore, docs",
			spec,
		)
	}
	branchType, slug = parts[0], parts[1]
	if !branchValidTypes[branchType] {
		return "", "", fmt.Errorf(
			"invalid branch type %q — must be one of feat, fix, refactor, chore, docs",
			branchType,
		)
	}
	if strings.TrimSpace(slug) == "" {
		return "", "", fmt.Errorf("branch slug is required — expected <type>/<slug>, got %q", spec)
	}
	return branchType, slug, nil
}
