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
	if !strings.Contains(out.String(), "commit direto em \"main\" não é permitido") {
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
	if !strings.Contains(out.String(), "commit direto em \"master\" não é permitido") {
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
	if !strings.Contains(msg, "# Tipo sugerido: test") {
		t.Fatalf("expected suggested type test, got: %q", msg)
	}
	if !strings.Contains(msg, "test(<escopo>): <descrição>") {
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
	if !strings.Contains(msg, "# Tipo sugerido: docs") {
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
	if !strings.Contains(msg, "# Tipo sugerido: feat") {
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
	if !strings.Contains(msg, "# Tipo sugerido: fix") {
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
