package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// makeBranchDeps builds branchNewDeps wired to injectable fakes, so tests never touch a real
// git repository or the real project filesystem layout.
//
// matched/candidates control what matchSlug returns. checkoutErr/checkoutCalls let tests assert
// whether git checkout -b was (or was not) invoked.
func makeBranchDeps(matched bool, candidates []string) (branchNewDeps, *bytes.Buffer, *[]string) {
	out := &bytes.Buffer{}
	checkoutCalls := []string{}
	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug: func(slug string, wipDirs, doneDirs []string) (bool, []string) {
			return matched, candidates
		},
		execGitCheckout: func(branchName string) error {
			checkoutCalls = append(checkoutCalls, branchName)
			return nil
		},
		out: out,
	}
	return d, out, &checkoutCalls
}

// ────────────────────────────────────────────────────────────────────────────
// parseBranchSpec
// ────────────────────────────────────────────────────────────────────────────

func TestParseBranchSpec_ValidTypes(t *testing.T) {
	for _, typ := range []string{"feat", "fix", "refactor", "chore", "docs"} {
		branchType, slug, err := parseBranchSpec(typ + "/my-slug")
		if err != nil {
			t.Fatalf("type %q: unexpected error: %v", typ, err)
		}
		if branchType != typ || slug != "my-slug" {
			t.Fatalf("type %q: got (%q, %q)", typ, branchType, slug)
		}
	}
}

func TestParseBranchSpec_InvalidType(t *testing.T) {
	_, _, err := parseBranchSpec("banana/my-slug")
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid branch type") {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `invalid branch type "banana" — must be one of feat, fix, refactor, chore, docs`
	if err.Error() != want {
		t.Fatalf("unexpected error message.\ngot:  %q\nwant: %q", err.Error(), want)
	}
}

func TestParseBranchSpec_EmptySlug(t *testing.T) {
	_, _, err := parseBranchSpec("feat/")
	if err == nil {
		t.Fatal("expected error for empty slug")
	}
	if !strings.Contains(err.Error(), "slug is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseBranchSpec_NoSlash(t *testing.T) {
	_, _, err := parseBranchSpec("feat-my-slug")
	if err == nil {
		t.Fatal("expected error for missing slash")
	}
	if !strings.Contains(err.Error(), "invalid branch spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — match found (wip/ or done/, no distinction at this layer since
// matchSlug is injected — the real matching logic is covered by
// internal/validator TestBranchSlugMatchesRoadmap-style tests).
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_MatchFound_ChecksOutBranch(t *testing.T) {
	deps, out, calls := makeBranchDeps(true, nil)
	err := runBranchNew("feat/my-slug", false, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "feat/my-slug" {
		t.Fatalf("expected git checkout -b feat/my-slug, got %v", *calls)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no extra output on successful checkout, got %q", out.String())
	}
}

func TestRunBranchNew_MatchFound_WipRoadmap(t *testing.T) {
	// Simulates a match found via a roadmap in wip/.
	deps, _, calls := makeBranchDeps(true, []string{"ROADMAP-my-slug.md"})
	if err := runBranchNew("fix/my-slug", false, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected checkout to run once, got %v", *calls)
	}
}

func TestRunBranchNew_MatchFound_DoneRoadmap(t *testing.T) {
	// Simulates a match found via a roadmap in done/ — matchSlug does not distinguish the
	// source directory in its return value, mirroring validator.BranchSlugMatchesRoadmap.
	deps, _, calls := makeBranchDeps(true, []string{"ROADMAP-my-slug.md"})
	if err := runBranchNew("refactor/my-slug", false, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected checkout to run once, got %v", *calls)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — no match: blocks, never calls git
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_NoMatch_NoCandidates_Blocks(t *testing.T) {
	deps, out, calls := makeBranchDeps(false, nil)
	err := runBranchNew("feat/orphan-slug", false, deps)
	if err == nil {
		t.Fatal("expected error when no roadmap matches")
	}
	if len(*calls) != 0 {
		t.Fatalf("git checkout must not run when blocked, got calls: %v", *calls)
	}
	got := out.String()
	want := validator.BranchGovernanceOrientation("feat/orphan-slug")
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain governance orientation message.\ngot: %q\nwant substring: %q", got, want)
	}
}

func TestRunBranchNew_NoMatch_WithCandidates_Blocks(t *testing.T) {
	candidates := []string{"ROADMAP-other-thing.md"}
	deps, out, calls := makeBranchDeps(false, candidates)
	err := runBranchNew("fix/orphan-slug", false, deps)
	if err == nil {
		t.Fatal("expected error when no roadmap matches")
	}
	if len(*calls) != 0 {
		t.Fatalf("git checkout must not run when blocked, got calls: %v", *calls)
	}
	got := out.String()
	want := validator.BranchNoMatchingRoadmapMessage("fix/orphan-slug", candidates)
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain no-matching-roadmap message.\ngot: %q\nwant substring: %q", got, want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — --dry-run
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_DryRun_Match_NeverCallsGit(t *testing.T) {
	deps, out, calls := makeBranchDeps(true, nil)
	err := runBranchNew("feat/my-slug", true, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("dry-run must never call git checkout, got: %v", *calls)
	}
	if !strings.Contains(out.String(), "dry-run") || !strings.Contains(out.String(), "would create") {
		t.Fatalf("expected dry-run 'would create' message, got: %q", out.String())
	}
}

func TestRunBranchNew_DryRun_NoMatch_NeverCallsGit(t *testing.T) {
	deps, out, calls := makeBranchDeps(false, nil)
	err := runBranchNew("feat/orphan-slug", true, deps)
	if err == nil {
		t.Fatal("expected error when dry-run would block")
	}
	if len(*calls) != 0 {
		t.Fatalf("dry-run must never call git checkout, got: %v", *calls)
	}
	if !strings.Contains(out.String(), "dry-run") || !strings.Contains(out.String(), "would block") {
		t.Fatalf("expected dry-run 'would block' message, got: %q", out.String())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — invalid type / empty slug never reach matchSlug or git
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_InvalidType_NeverCallsMatchOrGit(t *testing.T) {
	matchCalled := false
	deps, _, calls := makeBranchDeps(true, nil)
	deps.matchSlug = func(slug string, wipDirs, doneDirs []string) (bool, []string) {
		matchCalled = true
		return true, nil
	}
	err := runBranchNew("banana/my-slug", false, deps)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if matchCalled {
		t.Fatal("matchSlug must not be called for an invalid type")
	}
	if len(*calls) != 0 {
		t.Fatalf("git checkout must not run for an invalid type, got: %v", *calls)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — chore/docs are housekeeping types: they create the branch without
// the branch_has_wip_roadmap gate, and never call matchSlug.
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_ChoreType_SkipsGate_ChecksOutBranch(t *testing.T) {
	matchCalled := false
	deps, out, calls := makeBranchDeps(false, nil) // matched=false: gate would block if consulted
	deps.matchSlug = func(slug string, wipDirs, doneDirs []string) (bool, []string) {
		matchCalled = true
		return false, nil
	}
	err := runBranchNew("chore/release-7.0.0", false, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matchCalled {
		t.Fatal("matchSlug must not be called for chore — no roadmap gate applies")
	}
	if len(*calls) != 1 || (*calls)[0] != "chore/release-7.0.0" {
		t.Fatalf("expected git checkout -b chore/release-7.0.0, got %v", *calls)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no extra output on successful checkout, got %q", out.String())
	}
}

func TestRunBranchNew_DocsType_SkipsGate_ChecksOutBranch(t *testing.T) {
	matchCalled := false
	deps, _, calls := makeBranchDeps(false, nil) // matched=false: gate would block if consulted
	deps.matchSlug = func(slug string, wipDirs, doneDirs []string) (bool, []string) {
		matchCalled = true
		return false, nil
	}
	err := runBranchNew("docs/atualiza-readme", false, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matchCalled {
		t.Fatal("matchSlug must not be called for docs — no roadmap gate applies")
	}
	if len(*calls) != 1 || (*calls)[0] != "docs/atualiza-readme" {
		t.Fatalf("expected git checkout -b docs/atualiza-readme, got %v", *calls)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — non-regression: feat/fix/refactor without a matching roadmap must
// keep blocking with the same governance orientation message.
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_FeatWithoutRoadmap_StillBlocks_NonRegression(t *testing.T) {
	deps, out, calls := makeBranchDeps(false, nil)
	err := runBranchNew("feat/no-roadmap-for-this", false, deps)
	if err == nil {
		t.Fatal("expected error: feat without a matching roadmap must still block")
	}
	if len(*calls) != 0 {
		t.Fatalf("git checkout must not run when the gate blocks, got calls: %v", *calls)
	}
	got := out.String()
	want := validator.BranchGovernanceOrientation("feat/no-roadmap-for-this")
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to still contain the governance orientation message.\ngot: %q\nwant substring: %q", got, want)
	}
}

func TestRunBranchNew_EmptySlug_NeverCallsMatchOrGit(t *testing.T) {
	matchCalled := false
	deps, _, calls := makeBranchDeps(true, nil)
	deps.matchSlug = func(slug string, wipDirs, doneDirs []string) (bool, []string) {
		matchCalled = true
		return true, nil
	}
	err := runBranchNew("feat/", false, deps)
	if err == nil {
		t.Fatal("expected error for empty slug")
	}
	if matchCalled {
		t.Fatal("matchSlug must not be called for an empty slug")
	}
	if len(*calls) != 0 {
		t.Fatalf("git checkout must not run for an empty slug, got: %v", *calls)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runBranchNew — branch already exists: delegate to Git's native error, no special handling
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_BranchAlreadyExists_PropagatesGitError(t *testing.T) {
	deps, _, _ := makeBranchDeps(true, nil)
	gitErr := errors.New("fatal: a branch named 'feat/my-slug' already exists")
	deps.execGitCheckout = func(branchName string) error { return gitErr }

	err := runBranchNew("feat/my-slug", false, deps)
	if err == nil {
		t.Fatal("expected error propagated from git checkout")
	}
	if err != gitErr {
		t.Fatalf("expected the exact git error to propagate unmodified, got: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// newBranchNewCmd — matchSlug wired to the real validator function end-to-end via NormalizeBranchSlug
// ────────────────────────────────────────────────────────────────────────────

func TestRunBranchNew_UsesNormalizedSlugForMatching(t *testing.T) {
	var receivedSlug string
	deps, _, _ := makeBranchDeps(true, nil)
	deps.matchSlug = func(slug string, wipDirs, doneDirs []string) (bool, []string) {
		receivedSlug = slug
		return true, nil
	}
	if err := runBranchNew("feat/My_Weird--Slug", false, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := validator.NormalizeBranchSlug("My_Weird--Slug")
	if receivedSlug != want {
		t.Fatalf("expected normalized slug %q, got %q", want, receivedSlug)
	}
}
