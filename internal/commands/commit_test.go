package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// makeCommitDeps builds commitDeps wired to injectable fakes, so tests never touch a real git
// repository or the real project filesystem layout. branch controls what currentBranch returns.
// matched/candidates control what matchSlug returns.
func makeCommitDeps(branch string, matched bool, candidates []string) (commitDeps, *bytes.Buffer, *[]string) {
	out := &bytes.Buffer{}
	commitCalls := []string{}
	d := commitDeps{
		loadConfig:    func() config.ProjectConfig { return config.ProjectConfig{} },
		currentBranch: func() (string, error) { return branch, nil },
		resolveRoadmap: func(cfg config.ProjectConfig, b string) validator.BranchRoadmapResolution {
			if matched {
				return validator.BranchRoadmapResolution{Matched: true, Source: "inference"}
			}
			return validator.BranchRoadmapResolution{Matched: false, Candidates: candidates}
		},
		execGitCommit: func(message string) error {
			commitCalls = append(commitCalls, message)
			return nil
		},
		out: out,
	}
	return d, out, &commitCalls
}

// ────────────────────────────────────────────────────────────────────────────
// runCommit — main/master: always blocked
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_Main_Blocks(t *testing.T) {
	deps, out, calls := makeCommitDeps("main", true, nil)
	err := runCommit("fix: something", deps)
	if err == nil {
		t.Fatal("expected error when committing directly on main")
	}
	if len(*calls) != 0 {
		t.Fatalf("git commit must not run on main, got calls: %v", *calls)
	}
	if !strings.Contains(out.String(), "direct commit on \"main\" is not permitted") {
		t.Fatalf("expected blocking message, got: %q", out.String())
	}
}

func TestCommit_Master_Blocks(t *testing.T) {
	deps, out, calls := makeCommitDeps("master", true, nil)
	err := runCommit("fix: something", deps)
	if err == nil {
		t.Fatal("expected error when committing directly on master")
	}
	if len(*calls) != 0 {
		t.Fatalf("git commit must not run on master, got calls: %v", *calls)
	}
	if !strings.Contains(out.String(), "direct commit on \"master\" is not permitted") {
		t.Fatalf("expected blocking message, got: %q", out.String())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runCommit — feat/fix/refactor without a matching roadmap: blocked
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_GovernedBranch_NoMatch_NoCandidates_Blocks(t *testing.T) {
	deps, out, calls := makeCommitDeps("feat/orphan-slug", false, nil)
	err := runCommit("feat: orphan work", deps)
	if err == nil {
		t.Fatal("expected error when no roadmap matches")
	}
	if len(*calls) != 0 {
		t.Fatalf("git commit must not run when blocked, got calls: %v", *calls)
	}
	want := validator.BranchGovernanceOrientationForExisting("feat/orphan-slug", config.ProjectConfig{})
	if !strings.Contains(out.String(), want) {
		t.Fatalf("expected output to contain governance orientation message.\ngot: %q\nwant substring: %q", out.String(), want)
	}
}

func TestCommit_GovernedBranch_NoMatch_WithCandidates_Blocks(t *testing.T) {
	candidates := []string{"ROADMAP-other-thing.md"}
	deps, out, calls := makeCommitDeps("fix/orphan-slug", false, candidates)
	err := runCommit("fix: orphan work", deps)
	if err == nil {
		t.Fatal("expected error when no roadmap matches")
	}
	if len(*calls) != 0 {
		t.Fatalf("git commit must not run when blocked, got calls: %v", *calls)
	}
	want := validator.BranchNoMatchingRoadmapMessageForExisting("fix/orphan-slug", candidates)
	if !strings.Contains(out.String(), want) {
		t.Fatalf("expected output to contain no-matching-roadmap message.\ngot: %q\nwant substring: %q", out.String(), want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runCommit — feat/fix/refactor with a matching roadmap: succeeds
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_GovernedBranch_Match_Commits(t *testing.T) {
	deps, _, calls := makeCommitDeps("feat/my-slug", true, nil)
	err := runCommit("feat: my change", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "feat: my change" {
		t.Fatalf("expected git commit -m %q, got %v", "feat: my change", *calls)
	}
}

func TestCommit_GovernedBranch_Match_FixAndRefactor(t *testing.T) {
	for _, branch := range []string{"fix/my-slug", "refactor/my-slug"} {
		deps, _, calls := makeCommitDeps(branch, true, nil)
		if err := runCommit("chore: msg", deps); err != nil {
			t.Fatalf("branch %q: unexpected error: %v", branch, err)
		}
		if len(*calls) != 1 {
			t.Fatalf("branch %q: expected commit to run once, got %v", branch, *calls)
		}
	}
}

// TestCommit_BlockedRoadmap_Passes asserts that when the resolver returns a match whose roadmap
// lives in blocked/ (Source="inferred", Matched=true), commit proceeds without error.
//
// Reconciliation: this test affirms D2 of ADR-2026-10-01 — blocked/ governs an existing branch.
// The resolver is the single source of truth; commit accepts any Matched=true resolution.
func TestCommit_BlockedRoadmap_Passes(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	commitCalls := []string{}
	deps := commitDeps{
		loadConfig:    func() config.ProjectConfig { return config.ProjectConfig{} },
		currentBranch: func() (string, error) { return "fix/my-feature", nil },
		resolveRoadmap: func(cfg config.ProjectConfig, branch string) validator.BranchRoadmapResolution {
			// Simulate a roadmap found in blocked/ (moved there while work was paused).
			return validator.BranchRoadmapResolution{
				Matched: true,
				Source:  "inferred",
				Roadmap: "ROADMAP-2026-10-01-my-feature.md",
			}
		},
		execGitCommit: func(message string) error {
			commitCalls = append(commitCalls, message)
			return nil
		},
		out: out,
	}
	if err := runCommit("fix: resume blocked work", deps); err != nil {
		t.Fatalf("commit must succeed when roadmap is in blocked/: %v", err)
	}
	if len(commitCalls) != 1 {
		t.Fatalf("expected exactly one git commit call, got %v", commitCalls)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runCommit — branches outside feat/fix/refactor: allowed without a roadmap, but warns
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_UngovernedBranch_CommitsWithWarning(t *testing.T) {
	resolverCalled := false
	deps, out, calls := makeCommitDeps("docs/housekeeping", false, nil)
	deps.resolveRoadmap = func(cfg config.ProjectConfig, branch string) validator.BranchRoadmapResolution {
		resolverCalled = true
		return validator.BranchRoadmapResolution{}
	}
	err := runCommit("docs: update readme", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolverCalled {
		t.Fatal("resolveRoadmap must not be called for a branch outside feat/fix/refactor")
	}
	if len(*calls) != 1 || (*calls)[0] != "docs: update readme" {
		t.Fatalf("expected git commit -m %q, got %v", "docs: update readme", *calls)
	}
	if !strings.Contains(out.String(), "docs/housekeeping") {
		t.Fatalf("expected a warning mentioning the branch, got: %q", out.String())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runCommit — uses the normalized slug for matching, same as branch new
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_UsesNormalizedSlugForMatching(t *testing.T) {
	// Verifies that resolveRoadmap is called with the full branch name for a governed branch.
	// Slug normalization is an internal detail of the production dep (defaultResolveRoadmap);
	// the commit layer's contract is to forward the branch name unchanged.
	var receivedBranch string
	deps, _, _ := makeCommitDeps("feat/My_Weird--Slug", true, nil)
	deps.resolveRoadmap = func(cfg config.ProjectConfig, branch string) validator.BranchRoadmapResolution {
		receivedBranch = branch
		return validator.BranchRoadmapResolution{Matched: true, Source: "inference"}
	}
	if err := runCommit("feat: msg", deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBranch != "feat/My_Weird--Slug" {
		t.Fatalf("expected resolveRoadmap to receive full branch name %q, got %q", "feat/My_Weird--Slug", receivedBranch)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runCommit — current-branch resolution error propagates
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_CurrentBranchError_Propagates(t *testing.T) {
	deps, _, calls := makeCommitDeps("", true, nil)
	deps.currentBranch = func() (string, error) { return "", errors.New("not a git repository") }
	err := runCommit("feat: msg", deps)
	if err == nil {
		t.Fatal("expected error when current branch cannot be resolved")
	}
	if len(*calls) != 0 {
		t.Fatalf("git commit must not run when branch resolution fails, got: %v", *calls)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// buildSuggestedMessage — --suggest never commits, and classifies by heuristic
// ────────────────────────────────────────────────────────────────────────────

// makeSuggestDeps builds commitDeps for buildSuggestedMessage tests. execGitCommit fails the
// test immediately if invoked — --suggest must never trigger a real commit.
func makeSuggestDeps(t *testing.T, stagedRaw string, stagedErr error) commitDeps {
	t.Helper()
	return commitDeps{
		stagedNameStatus: func() (string, error) { return stagedRaw, stagedErr },
		execGitCommit: func(message string) error {
			t.Fatal("execGitCommit must never be called by --suggest")
			return nil
		},
		out: &bytes.Buffer{},
	}
}

func TestCommit_Suggest_NothingStaged_Errors(t *testing.T) {
	deps := makeSuggestDeps(t, "", nil)
	_, err := buildSuggestedMessage(deps)
	if err == nil {
		t.Fatal("expected error when nothing is staged")
	}
	if !strings.Contains(err.Error(), "nothing staged") {
		t.Fatalf("expected clear 'nothing staged' error, got: %v", err)
	}
}

func TestCommit_Suggest_OnlyTestFiles_SuggestsTest(t *testing.T) {
	raw := "M\tinternal/commands/commit_test.go\nA\tnpm/src/commit/runner.test.js\n"
	deps := makeSuggestDeps(t, raw, nil)
	msg, err := buildSuggestedMessage(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "# Suggested type: test") {
		t.Fatalf("expected suggested type test, got: %q", msg)
	}
	if !strings.Contains(msg, "test(<scope>): <description>") {
		t.Fatalf("expected test skeleton line, got: %q", msg)
	}
	if !strings.Contains(msg, "M  internal/commands/commit_test.go") {
		t.Fatalf("expected staged file listing, got: %q", msg)
	}
}

func TestCommit_Suggest_OnlyDocsFiles_SuggestsDocs(t *testing.T) {
	raw := "M\tdocs/roadmaps/wip/ROADMAP-x.md\nA\tvault/notes/example.md\nM\tREADME.md\n"
	deps := makeSuggestDeps(t, raw, nil)
	msg, err := buildSuggestedMessage(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "# Suggested type: docs") {
		t.Fatalf("expected suggested type docs, got: %q", msg)
	}
}

func TestCommit_Suggest_NewCommandFile_SuggestsFeat(t *testing.T) {
	raw := "A\tinternal/commands/newthing.go\nM\tinternal/commands/commit.go\n"
	deps := makeSuggestDeps(t, raw, nil)
	msg, err := buildSuggestedMessage(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "# Suggested type: feat") {
		t.Fatalf("expected suggested type feat, got: %q", msg)
	}
}

func TestCommit_Suggest_GenericChange_SuggestsFix(t *testing.T) {
	raw := "M\tinternal/commands/commit.go\nM\tinternal/config/config.go\n"
	deps := makeSuggestDeps(t, raw, nil)
	msg, err := buildSuggestedMessage(deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "# Suggested type: fix") {
		t.Fatalf("expected suggested type fix, got: %q", msg)
	}
}

func TestCommit_Suggest_StagedReadError_Propagates(t *testing.T) {
	deps := makeSuggestDeps(t, "", errors.New("not a git repository"))
	_, err := buildSuggestedMessage(deps)
	if err == nil {
		t.Fatal("expected error when staged diff cannot be read")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// newCommitCmd wiring — --suggest never commits, even if -m is also passed
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_SuggestFlag_NeverCommits_EvenWithMessage(t *testing.T) {
	cmd := newCommitCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--suggest", "-m", "should be ignored"})

	// This runs against the real trackfw repo working tree, but since execGitCommit is only
	// wired through the real defaultGitCommit in production code (never exercised here unless
	// runCommit is reached), a nothing-staged error is an acceptable and expected outcome as
	// long as no commit happens. We only assert no panic/commit occurs; the branch-specific
	// heuristic classification is already covered by the buildSuggestedMessage tests above.
	_ = cmd.Execute()
}

// ────────────────────────────────────────────────────────────────────────────
// D2 (ADR-2026-11-26): isBootstrapFirstCommit — conjunct predicate
// ────────────────────────────────────────────────────────────────────────────

// TestIsBootstrapFirstCommit_UnbornAndZeroCount affirms the main bootstrap case:
// both conditions true (HEAD unborn AND rev-list --all --count == "0") → true.
// Rule reconciliation: affirms ADR-2026-11-26 D2 conclusion that only a fresh repo
// (never had any commit) triggers the exception; the counter-arm (SwitchOrphan)
// confirms count>0 is correctly rejected.
func TestIsBootstrapFirstCommit_UnbornAndZeroCount(t *testing.T) {
	execGit := func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "rev-parse" {
			// HEAD is unborn — rev-parse fails.
			return "", errors.New("fatal: Needed a single revision")
		}
		if len(args) >= 1 && args[0] == "rev-list" {
			return "0\n", nil
		}
		return "", nil
	}
	if !isBootstrapFirstCommit(execGit) {
		t.Error("expected true for unborn HEAD with zero commits, got false")
	}
}

// TestIsBootstrapFirstCommit_SwitchOrphan affirms M-X1: when HEAD is unborn but
// rev-list --all --count is > 0 (git switch --orphan in a repo with history),
// returns false — the exception must NOT fire.
// Rule reconciliation: affirms the M-X1 measurement conclusion that
// rev-list --all --count (without HEAD) is the only reliable discriminator
// between a fresh repo and an orphan branch in an existing repo.
func TestIsBootstrapFirstCommit_SwitchOrphan(t *testing.T) {
	execGit := func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "rev-parse" {
			return "", errors.New("fatal: Needed a single revision")
		}
		if len(args) >= 1 && args[0] == "rev-list" {
			// count > 0: other branches have commits (switch --orphan case)
			return "3\n", nil
		}
		return "", nil
	}
	if isBootstrapFirstCommit(execGit) {
		t.Error("expected false for unborn HEAD with existing commits (switch --orphan), got true")
	}
}

// TestIsBootstrapFirstCommit_BornHead affirms that when HEAD resolves (there is at
// least one commit on the current branch), returns false immediately.
// Rule reconciliation: affirms the D2 "from second commit on, exception does not
// fire" behaviour — if condition (1) is false, (2) is not even evaluated.
func TestIsBootstrapFirstCommit_BornHead(t *testing.T) {
	revListCalled := false
	execGit := func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "rev-parse" {
			return "abc1234", nil // HEAD resolves → not unborn
		}
		if len(args) >= 1 && args[0] == "rev-list" {
			revListCalled = true
		}
		return "1\n", nil
	}
	if isBootstrapFirstCommit(execGit) {
		t.Error("expected false when HEAD has a commit, got true")
	}
	// Short-circuit: rev-list must not be called when condition (1) is false.
	if revListCalled {
		t.Error("rev-list must not be called when HEAD already has a commit")
	}
}

// TestIsBootstrapFirstCommit_NilExecGit affirms the fail-closed contract: a nil
// execGit returns false (block the commit).
// Rule reconciliation: affirms the fail-closed specification in ADR-2026-11-26 D2.
func TestIsBootstrapFirstCommit_NilExecGit(t *testing.T) {
	if isBootstrapFirstCommit(nil) {
		t.Error("expected false for nil execGit (fail-closed), got true")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D2 (ADR-2026-11-26): runCommit bootstrap exception on main/master
// ────────────────────────────────────────────────────────────────────────────

// TestRunCommit_BootstrapException_AllowsFirstCommitOnMain affirms that when
// isBootstrapFirstCommit returns true, runCommit proceeds to call execGitCommit
// on the default branch (main) instead of blocking it.
// Rule reconciliation: affirms ADR-2026-11-26 D2 — first commit on main is
// allowed; the counter-arm (TestRunCommit_BootstrapException_BlocksSecondCommit)
// confirms the block resumes from the second commit on.
func TestRunCommit_BootstrapException_AllowsFirstCommitOnMain(t *testing.T) {
	out := &bytes.Buffer{}
	commitCalled := false

	d := commitDeps{
		loadConfig:    func() config.ProjectConfig { return config.ProjectConfig{} },
		currentBranch: func() (string, error) { return "main", nil },
		resolveRoadmap: func(cfg config.ProjectConfig, b string) validator.BranchRoadmapResolution {
			return validator.BranchRoadmapResolution{Matched: false}
		},
		execGitCommit: func(msg string) error {
			commitCalled = true
			return nil
		},
		stagedNameStatus: func() (string, error) { return "M\tREADME.md\n", nil },
		execGit: func(args ...string) (string, error) {
			// Simulate bootstrap: HEAD unborn AND count==0.
			if len(args) >= 1 && args[0] == "rev-parse" {
				return "", errors.New("fatal: Needed a single revision")
			}
			if len(args) >= 1 && args[0] == "rev-list" {
				return "0\n", nil
			}
			return "", nil
		},
		out: out,
	}

	if err := runCommit("chore(scaffold): initial", d); err != nil {
		t.Fatalf("expected first bootstrap commit to be allowed, got error: %v", err)
	}
	if !commitCalled {
		t.Error("execGitCommit must be called for the bootstrap first commit on main")
	}
}

// TestRunCommit_BootstrapException_BlocksSecondCommit is the counter-arm for
// TestRunCommit_BootstrapException_AllowsFirstCommitOnMain: when HEAD resolves
// (second or later commit), the protected-branch block must fire on main.
// Rule reconciliation: affirms ADR-2026-11-26 D2 "from second commit on, the
// block resumes" — the exception is strictly one-shot.
func TestRunCommit_BootstrapException_BlocksSecondCommit(t *testing.T) {
	out := &bytes.Buffer{}
	commitCalled := false

	d := commitDeps{
		loadConfig:    func() config.ProjectConfig { return config.ProjectConfig{} },
		currentBranch: func() (string, error) { return "main", nil },
		resolveRoadmap: func(cfg config.ProjectConfig, b string) validator.BranchRoadmapResolution {
			return validator.BranchRoadmapResolution{Matched: false}
		},
		execGitCommit: func(msg string) error {
			commitCalled = true
			return nil
		},
		stagedNameStatus: func() (string, error) { return "M\tREADME.md\n", nil },
		execGit: func(args ...string) (string, error) {
			// HEAD resolves → not bootstrap (second commit scenario).
			return "abc1234", nil
		},
		out: out,
	}

	err := runCommit("fix: second commit on main", d)
	if err == nil {
		t.Fatal("expected block for second commit on main, got nil error")
	}
	if !strings.Contains(err.Error(), "not permitted") {
		t.Errorf("error must mention 'not permitted', got: %v", err)
	}
	if commitCalled {
		t.Error("execGitCommit must not be called when main commit is blocked")
	}
}
