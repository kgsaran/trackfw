package commands

import (
	"bytes"
	"errors"
	"os/exec"
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
			// D1: gate is wip-only; tests pass nil for doneDirs when calling the gate.
			// matchDone below handles done/ hint separately.
			return matched, candidates
		},
		matchDone: func(slug string, doneDirs []string) []string {
			return nil // no done matches by default in fake helpers
		},
		execGitCheckout: func(branchName, _ string) error {
			checkoutCalls = append(checkoutCalls, branchName)
			return nil
		},
		// fromRef: "HEAD" reproduces the pre-v10 behaviour in unit tests: no fetch, no origin
		// resolution, no start point. Tests that exercise the remote-resolution path build
		// their own deps instead of using makeBranchDeps.
		fromRef: "HEAD",
		out:     out,
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
// runBranchNew — match found (wip/ only at creation; done/ distinction is in the orientation
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

func TestRunBranchNew_DoneOnlyMatch_Blocks(t *testing.T) {
	// D1 of ADR-2026-10-01: branch creation is gated on wip/ only.
	// A match ONLY in done/ must block the branch and name the roadmap + suggest roadmap move.
	// This test was formerly TestRunBranchNew_MatchFound_DoneRoadmap which asserted the
	// OPPOSITE (done/ match creates the branch) — inverted in ML-1B (REQ-2026-10-01).
	out := &bytes.Buffer{}
	calls := []string{}
	deps := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug: func(slug string, wipDirs, doneDirs []string) (bool, []string) {
			// D1 gate: only wip — no match in wip.
			return false, nil
		},
		matchDone: func(slug string, doneDirs []string) []string {
			// There IS a matching roadmap in done/.
			return []string{"ROADMAP-my-slug.md"}
		},
		execGitCheckout: func(branchName, _ string) error {
			calls = append(calls, branchName)
			return nil
		},
		fromRef: "HEAD",
		out:     out,
	}
	err := runBranchNew("refactor/my-slug", false, deps)
	if err == nil {
		t.Fatal("expected error: done/-only match must block branch creation (D1)")
	}
	if len(calls) != 0 {
		t.Fatalf("git checkout must not run when blocked, got calls: %v", calls)
	}
	got := out.String()
	if !strings.Contains(got, "ROADMAP-my-slug.md") {
		t.Fatalf("expected message to name the done/ roadmap, got:\n%s", got)
	}
	if !strings.Contains(got, "roadmap move") {
		t.Fatalf("expected message to suggest roadmap move, got:\n%s", got)
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
	want := validator.BranchGovernanceOrientationForCreation("feat/orphan-slug", config.ProjectConfig{}, nil)
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
	want := validator.BranchNoMatchingRoadmapMessageForCreation("fix/orphan-slug", candidates, nil)
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
	want := validator.BranchGovernanceOrientationForCreation("feat/no-roadmap-for-this", config.ProjectConfig{}, nil)
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
	deps.execGitCheckout = func(branchName, _ string) error { return gitErr }

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

// ────────────────────────────────────────────────────────────────────────────
// resolveBranchStartPoint — B-1 flag-injection guard
// ────────────────────────────────────────────────────────────────────────────

// TestResolveBranchStartPoint_DashFlagRejected asserts that --from values starting with '-'
// are rejected before any git call is made.
// Reconciliation: affirms B-1 fix — `--from "-f"` would otherwise execute `git checkout -f`
// and silently discard uncommitted work.
func TestResolveBranchStartPoint_DashFlagRejected(t *testing.T) {
	cases := []string{
		"-f",
		"--force",
		"--upload-pack=x",
		"-b",
	}
	for _, ref := range cases {
		deps := branchNewDeps{
			fromRef: ref,
			gitExec: func(args ...string) (string, error) {
				t.Errorf("B-1: git must not be called for --from %q, but was called with %v", ref, args)
				return "", nil
			},
		}
		startPoint, displayRef, err := resolveBranchStartPoint(deps, "chore/x")
		if err == nil {
			t.Errorf("B-1: expected error for --from %q, got nil (startPoint=%q displayRef=%q)", ref, startPoint, displayRef)
		}
		if startPoint != "" || displayRef != "" {
			t.Errorf("B-1: expected empty startPoint/displayRef for rejected --from %q, got %q/%q", ref, startPoint, displayRef)
		}
	}
}

// TestResolveBranchStartPoint_ExplicitRefResolvesToSHA asserts that a valid explicit ref
// calls git rev-parse and returns the resolved SHA (not the raw ref text).
// Reconciliation: affirms B-1 fix — checkout receives a flag-safe SHA, not user-supplied text.
func TestResolveBranchStartPoint_ExplicitRefResolvesToSHA(t *testing.T) {
	const fakeRef = "origin/altra"
	const fakeSHA = "abc1234def5678901234567890123456789012ab"

	deps := branchNewDeps{
		fromRef: fakeRef,
		gitExec: func(args ...string) (string, error) {
			// Expect: rev-parse --verify --quiet --end-of-options <ref>^{commit}
			if len(args) >= 2 && args[0] == "rev-parse" {
				return fakeSHA + "\n", nil
			}
			t.Errorf("B-1: unexpected git call: %v", args)
			return "", nil
		},
	}
	startPoint, displayRef, err := resolveBranchStartPoint(deps, "chore/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startPoint != fakeSHA {
		t.Errorf("B-1: expected SHA %q as startPoint, got %q", fakeSHA, startPoint)
	}
	if displayRef != fakeRef {
		t.Errorf("B-1: expected displayRef %q, got %q", fakeRef, displayRef)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D2 (ADR-2026-11-26): branch new rejects unborn HEAD before type check
// ────────────────────────────────────────────────────────────────────────────

// TestRunBranchNew_UnbornHEAD_BlocksBeforeTypeCheck affirms D2 for branch new:
// when headExitCode returns 1 (rev-parse --verify -q HEAD exits 1 = unborn HEAD),
// runBranchNew must return an error with the bootstrap instructions — without
// reaching the roadmap/type gate.
// Rule reconciliation: affirms ADR-2026-11-26 D2 conclusion that rc==1 is blocked
// at the entry of runBranchNew; the counter-arm (TestRunBranchNew_BornHEAD_AllowsTypeCheck)
// confirms the type gate fires when HEAD has a commit (rc==0).
func TestRunBranchNew_UnbornHEAD_BlocksBeforeTypeCheck(t *testing.T) {
	out := &bytes.Buffer{}
	typeGateCalled := false

	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug: func(slug string, wip, done []string) (bool, []string) {
			typeGateCalled = true
			return true, nil
		},
		matchDone:       func(slug string, done []string) []string { return nil },
		execGitCheckout: func(branch, _ string) error { return nil },
		fromRef:         "HEAD",
		out:             out,
		headExitCode:    func() int { return 1 }, // simulate unborn HEAD (rc==1)
	}

	err := runBranchNew("feat/bootstrap-test", false, d)
	if err == nil {
		t.Fatal("expected error for unborn HEAD, got nil")
	}
	if !strings.Contains(err.Error(), "unborn") && !strings.Contains(err.Error(), "bootstrap") {
		t.Errorf("error must mention unborn/bootstrap, got: %v", err)
	}
	// The type/roadmap gate must NOT have fired — block happens before it.
	if typeGateCalled {
		t.Error("type gate must not be called when HEAD has no commit (D2 blocks first)")
	}
}

// TestRunBranchNew_BornHEAD_AllowsTypeCheck is the counter-arm for
// TestRunBranchNew_UnbornHEAD_BlocksBeforeTypeCheck: when headExitCode returns 0
// (HEAD has a commit), runBranchNew proceeds past the D2 check to the type gate.
// Rule reconciliation: affirms the counter-arm — if D2 blocked all branches
// unconditionally, this test would fail because typeGateCalled would be false.
func TestRunBranchNew_BornHEAD_AllowsTypeCheck(t *testing.T) {
	out := &bytes.Buffer{}
	typeGateCalled := false

	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug: func(slug string, wip, done []string) (bool, []string) {
			typeGateCalled = true
			return true, nil // matched: allow checkout
		},
		matchDone:       func(slug string, done []string) []string { return nil },
		execGitCheckout: func(branch, _ string) error { return nil },
		fromRef:         "HEAD",
		out:             out,
		headExitCode:    func() int { return 0 }, // simulate born HEAD (rc==0)
	}

	err := runBranchNew("feat/bootstrap-test", false, d)
	if err != nil {
		t.Fatalf("unexpected error for born HEAD: %v", err)
	}
	if !typeGateCalled {
		t.Error("type gate must be called when HEAD has a commit (D2 passes through)")
	}
}

// TestRunBranchNew_UnbornHEAD_IncludesGitSwitchHint (ML-2C M-X1) affirms that
// when branch new is blocked on an unborn HEAD (D2, headExitCode returns 1), the
// output printed to the user includes a "git switch <branch>" hint so the user can
// recover by switching to an existing branch rather than being stuck with only the
// bootstrap path. The hint lives in the printed message (deps.out), not the returned
// error; the error is a short machine-parseable "blocked: HEAD has no commit" string.
//
// Rule reconciliation: affirms M-X1 (ML-2C) — the printed block already contained
// the hint after the Wave 2 implementation; this test asserts it is present and
// will catch any accidental removal.
func TestRunBranchNew_UnbornHEAD_IncludesGitSwitchHint(t *testing.T) {
	out := &bytes.Buffer{}

	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug:       func(slug string, wip, done []string) (bool, []string) { return true, nil },
		matchDone:       func(slug string, done []string) []string { return nil },
		execGitCheckout: func(branch, _ string) error { return nil },
		fromRef:         "HEAD",
		out:             out,
		headExitCode:    func() int { return 1 }, // simulate unborn HEAD (rc==1)
	}

	err := runBranchNew("feat/bootstrap-test", false, d)
	if err == nil {
		t.Fatal("expected error for unborn HEAD, got nil")
	}
	// The hint is in the printed output, not the returned error.
	if !strings.Contains(out.String(), "git switch") {
		t.Errorf("M-X1: branch new unborn HEAD output must include 'git switch <branch>' hint; got output: %q, err: %v", out.String(), err)
	}
}

// D2 counter-arm tests (ML-2E): exit codes other than 1 must NOT trigger the unborn refusal.
// ────────────────────────────────────────────────────────────────────────────────────────────

// TestRunBranchNew_D2_ExitCode128_AllowsTypeCheck affirms that rc==128
// ("not a git repository") does NOT trigger the D2 unborn-HEAD refusal.
// headExitCode returning 128 simulates the response when git is called outside
// any repository (the common case for scripts that run --dry-run to measure
// slug matching, such as check-roadmap-slug-matching.sh).
// Rule reconciliation: affirms ML-2E requirement that D2 uses rc==1 exclusively;
// rc==128 must pass through to the slug governance check.
func TestRunBranchNew_D2_ExitCode128_AllowsTypeCheck(t *testing.T) {
	out := &bytes.Buffer{}
	typeGateCalled := false

	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug: func(slug string, wip, done []string) (bool, []string) {
			typeGateCalled = true
			return true, nil
		},
		matchDone:       func(slug string, done []string) []string { return nil },
		execGitCheckout: func(branch, _ string) error { return nil },
		fromRef:         "HEAD",
		out:             out,
		headExitCode:    func() int { return 128 }, // not a git repository
	}

	err := runBranchNew("feat/bootstrap-test", false, d)
	if err != nil {
		t.Fatalf("rc==128 must not block: unexpected error: %v", err)
	}
	if !typeGateCalled {
		t.Error("type gate must be called when headExitCode is 128 (not a repository)")
	}
	if strings.Contains(out.String(), "unborn HEAD") {
		t.Errorf("rc==128 must not emit unborn-HEAD message; got: %q", out.String())
	}
}

// TestRunBranchNew_D2_ExitCodeMinus1_AllowsTypeCheck affirms that rc==-1
// (git binary not found, or other non-ExitError failure) does NOT trigger D2.
// Rule reconciliation: affirms ML-2E requirement that D2 uses rc==1 exclusively;
// any non-zero code other than 1 passes through.
func TestRunBranchNew_D2_ExitCodeMinus1_AllowsTypeCheck(t *testing.T) {
	out := &bytes.Buffer{}
	typeGateCalled := false

	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug: func(slug string, wip, done []string) (bool, []string) {
			typeGateCalled = true
			return true, nil
		},
		matchDone:       func(slug string, done []string) []string { return nil },
		execGitCheckout: func(branch, _ string) error { return nil },
		fromRef:         "HEAD",
		out:             out,
		headExitCode:    func() int { return -1 }, // git not found
	}

	err := runBranchNew("feat/bootstrap-test", false, d)
	if err != nil {
		t.Fatalf("rc==-1 must not block: unexpected error: %v", err)
	}
	if !typeGateCalled {
		t.Error("type gate must be called when headExitCode is -1 (git not found)")
	}
	if strings.Contains(out.String(), "unborn HEAD") {
		t.Errorf("rc==-1 must not emit unborn-HEAD message; got: %q", out.String())
	}
}

// Real-git D2 tests using gitRevParseHeadExitCode directly (ML-2E).
// ────────────────────────────────────────────────────────────────────────────────────────────

// TestGitRevParseHeadExitCode_NonGitDir affirms that gitRevParseHeadExitCode returns 128
// (not 1) for a directory that is not a git repository. This is the exit code that was
// previously misclassified as "unborn HEAD" by the old D2 implementation, causing
// check-roadmap-slug-matching.sh --self-test to report 205/205 false rejections.
// Rule reconciliation: affirms ML-2E — the discriminant between "unborn HEAD" and
// "not a repository" is the exit code (1 vs 128), and this test asserts the value
// that arrives at D2 for the non-repository case.
func TestGitRevParseHeadExitCode_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	// Set GIT_CEILING_DIRECTORIES so git cannot walk up into an enclosing repository.
	t.Setenv("GIT_CEILING_DIRECTORIES", dir)

	got := gitRevParseHeadExitCode(dir)
	if got != 128 {
		t.Errorf("expected exit code 128 for non-git dir, got %d", got)
	}
}

// TestGitRevParseHeadExitCode_UnbornRepo affirms that gitRevParseHeadExitCode returns 1
// for a git-initialized repository with no commits (unborn HEAD).
// Rule reconciliation: affirms ML-2E — rc==1 is the signal D2 must act on; this test
// asserts that the real git subprocess produces rc==1 in the unborn-HEAD scenario.
func TestGitRevParseHeadExitCode_UnbornRepo(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Skipf("git init failed (git not available?): %v", err)
	}

	got := gitRevParseHeadExitCode(dir)
	if got != 1 {
		t.Errorf("expected exit code 1 for unborn HEAD, got %d", got)
	}
}

// TestRunBranchNew_D2_RealGit_NonGitDir_DryRun_ShowsWouldCreate affirms the end-to-end
// fix for ML-2E: running --dry-run in a directory that is not a git repository must NOT
// print the unborn-HEAD refusal. The slug governance check runs normally and prints
// "would create" (or "would block") — not the bootstrap instructions.
//
// This test exercises the production headExitCode path (gitRevParseHeadExitCode) so
// that any regression in the discriminant is caught before the slug-matching script.
// Rule reconciliation: affirms ML-2E — this is the exact scenario that caused
// check-roadmap-slug-matching.sh --self-test to report 205/205 blocked branches.
func TestRunBranchNew_D2_RealGit_NonGitDir_DryRun_ShowsWouldCreate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", dir)

	out := &bytes.Buffer{}

	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug:       func(slug string, wip, done []string) (bool, []string) { return true, nil },
		matchDone:       func(slug string, done []string) []string { return nil },
		execGitCheckout: func(branch, _ string) error { return nil },
		fromRef:         "HEAD",
		out:             out,
		headExitCode:    func() int { return gitRevParseHeadExitCode(dir) },
	}

	err := runBranchNew("chore/x", true /*dryRun*/, d)
	if err != nil {
		t.Fatalf("non-git dir must not block in dry-run: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "unborn HEAD") || strings.Contains(got, "bootstrap") {
		t.Errorf("non-git dir must not produce unborn-HEAD message; got: %q", got)
	}
	if !strings.Contains(got, "would create") && !strings.Contains(got, "would block") {
		t.Errorf("non-git dir dry-run must print 'would create' or 'would block'; got: %q", got)
	}
}

// TestRunBranchNew_D2_RealGit_UnbornRepo_Blocks affirms that an unborn HEAD repository
// (git-initialized, no commits) is blocked by D2 even with real git.
// Rule reconciliation: affirms ML-2E counter-arm — the fix must not accidentally
// disable D2 for the actual unborn-HEAD case.
func TestRunBranchNew_D2_RealGit_UnbornRepo_Blocks(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Skipf("git init failed (git not available?): %v", err)
	}

	out := &bytes.Buffer{}

	d := branchNewDeps{
		loadConfig:      func() config.ProjectConfig { return config.ProjectConfig{} },
		resolveWIPDirs:  func(config.ProjectConfig) []string { return []string{"docs/roadmaps/wip"} },
		resolveDoneDirs: func(config.ProjectConfig) []string { return []string{"docs/roadmaps/done"} },
		matchSlug:       func(slug string, wip, done []string) (bool, []string) { return true, nil },
		matchDone:       func(slug string, done []string) []string { return nil },
		execGitCheckout: func(branch, _ string) error { return nil },
		fromRef:         "HEAD",
		out:             out,
		headExitCode:    func() int { return gitRevParseHeadExitCode(dir) },
	}

	err := runBranchNew("chore/x", false /*dryRun*/, d)
	if err == nil {
		t.Fatal("unborn HEAD repo must be blocked by D2, got nil error")
	}
	if !strings.Contains(err.Error(), "unborn") && !strings.Contains(err.Error(), "bootstrap") {
		t.Errorf("error must mention unborn/bootstrap, got: %v", err)
	}
}

// Cobra wiring tests (ML-2E): run through newBranchNewCmd to validate that
// headExitCode is set in the production deps.
// ────────────────────────────────────────────────────────────────────────────────────────────

// TestBranchNewCmd_D2_WiringSet_UnbornRepo affirms that the production command
// (newBranchNewCmd) sets headExitCode in deps so D2 fires for a real unborn repository.
// Rule reconciliation: validates the production wiring — if headExitCode is not set in
// newBranchNewCmd, D2 is nil and silently dead; this test catches that regression.
func TestBranchNewCmd_D2_WiringSet_UnbornRepo(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Skipf("git init failed (git not available?): %v", err)
	}
	t.Chdir(dir)

	cmd := newBranchNewCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"chore/wiring-check", "--dry-run", "--from", "HEAD"})

	execErr := cmd.Execute()
	if execErr == nil {
		t.Fatal("unborn HEAD repo must be blocked (D2); got nil error — headExitCode may not be wired in newBranchNewCmd")
	}
	combined := out.String() + errBuf.String()
	if !strings.Contains(combined, "unborn") && !strings.Contains(combined, "bootstrap") {
		t.Errorf("expected unborn-HEAD message; got out=%q err=%q execErr=%v", out.String(), errBuf.String(), execErr)
	}
}
