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
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		currentBranch:   func() (string, error) { return branch, nil },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug: func(slug string, wipDirs, doneDirs []string) (bool, []string) {
			return matched, candidates
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
	want := validator.BranchGovernanceOrientation("feat/orphan-slug")
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
	want := validator.BranchNoMatchingRoadmapMessage("fix/orphan-slug", candidates)
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

// ────────────────────────────────────────────────────────────────────────────
// runCommit — branches outside feat/fix/refactor: allowed without a roadmap, but warns
// ────────────────────────────────────────────────────────────────────────────

func TestCommit_UngovernedBranch_CommitsWithWarning(t *testing.T) {
	matchCalled := false
	deps, out, calls := makeCommitDeps("docs/housekeeping", false, nil)
	deps.matchSlug = func(slug string, wipDirs, doneDirs []string) (bool, []string) {
		matchCalled = true
		return false, nil
	}
	err := runCommit("docs: update readme", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matchCalled {
		t.Fatal("matchSlug must not be called for a branch outside feat/fix/refactor")
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
	var receivedSlug string
	deps, _, _ := makeCommitDeps("feat/My_Weird--Slug", true, nil)
	deps.matchSlug = func(slug string, wipDirs, doneDirs []string) (bool, []string) {
		receivedSlug = slug
		return true, nil
	}
	if err := runCommit("feat: msg", deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := validator.NormalizeBranchSlug("My_Weird--Slug")
	if receivedSlug != want {
		t.Fatalf("expected normalized slug %q, got %q", want, receivedSlug)
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
