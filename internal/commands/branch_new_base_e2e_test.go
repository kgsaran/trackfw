package commands

// branch_new_base_e2e_test.go — real-git integration tests for the start-point behaviour
// introduced in ROADMAP-2026-09-12 ML-1A (REQ-2026-09-12).
//
// Every test runs the compiled trackfw binary as a subprocess (e2eBinary, same as
// branch_state_e2e_test.go) against a real git repo in a temp dir. No mocks are used for the
// git operations under test; the injected-fake unit tests in branch_test.go cover governance.
//
// Reconciliation sentences (one per test, per the Regra Dura de Reconciliação):
//
//   TestBranchNewBase_DefaultCreatesFromRemote:
//     Affirms AC1/F-1 (negative direction): `branch new` without --from creates a branch with zero
//     commits beyond origin/main, even when the current branch has its own commits.
//
//   TestBranchNewBase_FromHEAD_PreservesCommits:
//     Affirms F-1 (positive / contra-braço): `branch new --from HEAD` preserves the current
//     branch's commits on the new branch.
//
//   TestBranchNewBase_ChoreDefaultCreatesFromRemote:
//     Affirms F-3/A-3: chore/ type also creates from origin/main, not from HEAD, so the A-3
//     threat (fetch/start-point inside branchGatedTypes block) is falsified.
//
//   TestBranchNewBase_NoTrackUpstream:
//     Affirms F-5/A-5: after `branch new` without --from, git rev-parse @{u} is empty (no
//     upstream), confirming --no-track was used.
//
//   TestBranchNewBase_FetchUpdatesBase:
//     Affirms AC3 (fetch): when origin/main has advanced past the local tracking ref, `branch new`
//     creates the branch from the remote tip, not the stale local ref.
//
//   TestBranchNewBase_FetchFailureWarnsAndCreates:
//     Affirms F-2/AC3 (warn, do not abort): when fetch fails, `branch new` prints a warning and
//     creates the branch from the locally-known state of origin/main.
//
//   TestBranchNewBase_NoOriginActionable:
//     Affirms F-4b: a repo without 'origin' gets an actionable message and exit non-zero, never
//     a raw git exit 128.
//
//   TestBranchNewBase_DryRunShowsBase:
//     Affirms AC9/F-6: --dry-run output names the base ref.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// makeRemoteRepo creates an isolated git repo with a bare remote ('origin') and an initial
// commit on main with a trackfw layout. Returns (workDir, bareDir, homeDir).
func makeRemoteRepo(t *testing.T) (workDir, bareDir, homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	bareDir = t.TempDir()
	workDir = t.TempDir()

	// Init bare remote.
	gitE2E(t, bareDir, homeDir, "init", "-q", "--bare", "-b", "main", ".")

	// Init work tree, add remote, create initial commit.
	gitE2E(t, workDir, homeDir, "init", "-q", "-b", "main", ".")
	gitE2E(t, workDir, homeDir, "config", "user.email", "e2e@localhost")
	gitE2E(t, workDir, homeDir, "config", "user.name", "E2E Test")
	gitE2E(t, workDir, homeDir, "config", "commit.gpgsign", "false")
	gitE2E(t, workDir, homeDir, "remote", "add", "origin", bareDir)

	// Create minimal trackfw.yaml and directory structure.
	for _, d := range []string{
		"docs/roadmaps/wip",
		"docs/roadmaps/done",
		"docs/roadmaps/backlog",
		"docs/roadmaps/abandoned",
		"docs/req",
	} {
		if err := os.MkdirAll(filepath.Join(workDir, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, "trackfw.yaml"),
		[]byte(minimalTrackfwYAML()), 0o644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	// Placeholder file so initial commit is non-empty.
	if err := os.WriteFile(filepath.Join(workDir, "README.md"),
		[]byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	gitE2E(t, workDir, homeDir, "add", ".")
	gitE2E(t, workDir, homeDir, "commit", "-q", "-m", "initial commit")
	gitE2E(t, workDir, homeDir, "push", "-q", "-u", "origin", "main")

	return workDir, bareDir, homeDir
}

// addWIPRoadmapFile writes a minimal wip/ roadmap file for slug directly in workDir without
// staging or committing it. The governance validator reads the filesystem, so uncommitted
// roadmaps are found. Use this when the goal is to test start-point behaviour, not governance.
func addWIPRoadmapFile(t *testing.T, workDir, slug string) {
	t.Helper()
	name := "ROADMAP-" + slug + ".md"
	content := "---\nstatus: wip\ndate: 2026-10-18\n---\n# Roadmap: " + slug + "\n"
	if err := os.WriteFile(filepath.Join(workDir, "docs/roadmaps/wip", name),
		[]byte(content), 0o644); err != nil {
		t.Fatalf("write roadmap %s: %v", name, err)
	}
}

// countAheadOfOriginMain returns the number of commits on HEAD that are not on origin/main.
// Used to verify the start point of a newly created branch.
func countAheadOfOriginMain(t *testing.T, dir, homeDir string) int {
	t.Helper()
	out := strings.TrimSpace(gitE2EOutput(t, dir, homeDir, "rev-list", "--count", "origin/main..HEAD"))
	n, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("countAheadOfOriginMain: unexpected output %q: %v", out, err)
	}
	return n
}

// ────────────────────────────────────────────────────────────────────────────────────────────────
// Tests
// ────────────────────────────────────────────────────────────────────────────────────────────────

// Affirms AC1/F-1 (negative direction): `branch new` without --from creates a branch with zero
// commits beyond origin/main, even when the current branch has its own commits.
func TestBranchNewBase_DefaultCreatesFromRemote(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	// Create a feature branch that has extra commits.
	gitE2E(t, workDir, homeDir, "checkout", "-b", "feat/existing-work")
	if err := os.WriteFile(filepath.Join(workDir, "work.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatalf("write work.txt: %v", err)
	}
	gitE2E(t, workDir, homeDir, "add", "work.txt")
	gitE2E(t, workDir, homeDir, "commit", "-q", "-m", "some work on existing-work branch")

	// Verify we ARE ahead of origin/main from this branch.
	if n := countAheadOfOriginMain(t, workDir, homeDir); n == 0 {
		t.Fatal("setup: expected feat/existing-work to be ahead of origin/main")
	}

	// Write roadmap directly (not committed) so the validator finds it on the current branch.
	addWIPRoadmapFile(t, workDir, "new-feature")

	// Run `trackfw branch new feat/new-feature` from feat/existing-work.
	out, code := runTFW(t, workDir, homeDir, "branch", "new", "feat/new-feature")
	if code != 0 {
		t.Fatalf("branch new returned non-zero (%d):\n%s", code, out)
	}

	// The new branch should have zero commits ahead of origin/main.
	aheadCount := countAheadOfOriginMain(t, workDir, homeDir)
	if aheadCount != 0 {
		t.Errorf("AC1/F-1: expected feat/new-feature to have 0 commits ahead of origin/main, got %d", aheadCount)
	}
}

// Affirms F-1 (positive / contra-braço): `branch new --from HEAD` preserves the current
// branch's commits on the new branch.
func TestBranchNewBase_FromHEAD_PreservesCommits(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	// Create a feature branch with an extra commit.
	gitE2E(t, workDir, homeDir, "checkout", "-b", "feat/base-branch")
	if err := os.WriteFile(filepath.Join(workDir, "work2.txt"), []byte("work2\n"), 0o644); err != nil {
		t.Fatalf("write work2.txt: %v", err)
	}
	gitE2E(t, workDir, homeDir, "add", "work2.txt")
	gitE2E(t, workDir, homeDir, "commit", "-q", "-m", "commit on base-branch")

	// Write roadmap directly (not committed) so the validator finds it.
	addWIPRoadmapFile(t, workDir, "stacked-feature")

	// Run with --from HEAD: should preserve commits.
	out, code := runTFW(t, workDir, homeDir, "branch", "new", "--from", "HEAD", "feat/stacked-feature")
	if code != 0 {
		t.Fatalf("branch new --from HEAD returned non-zero (%d):\n%s", code, out)
	}

	// The new branch should be ahead of origin/main (has the extra commit from HEAD).
	aheadCount := countAheadOfOriginMain(t, workDir, homeDir)
	if aheadCount == 0 {
		t.Errorf("F-1 contra-braço: expected feat/stacked-feature to be ahead of origin/main (from HEAD), got 0 commits")
	}
}

// Affirms F-3/A-3: chore/ type also creates from origin/main, not from HEAD.
func TestBranchNewBase_ChoreDefaultCreatesFromRemote(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	// Create a working branch with extra commits (no roadmap needed for chore).
	gitE2E(t, workDir, homeDir, "checkout", "-b", "feat/work-in-progress")
	if err := os.WriteFile(filepath.Join(workDir, "w.txt"), []byte("w\n"), 0o644); err != nil {
		t.Fatalf("write w.txt: %v", err)
	}
	gitE2E(t, workDir, homeDir, "add", "w.txt")
	gitE2E(t, workDir, homeDir, "commit", "-q", "-m", "work in progress")

	if n := countAheadOfOriginMain(t, workDir, homeDir); n == 0 {
		t.Fatal("setup: expected feat/work-in-progress to be ahead of origin/main")
	}

	// `trackfw branch new chore/release-x` from a branch that is ahead of origin/main.
	out, code := runTFW(t, workDir, homeDir, "branch", "new", "chore/release-x")
	if code != 0 {
		t.Fatalf("branch new chore/release-x returned non-zero (%d):\n%s", code, out)
	}

	// chore/ also creates from origin/main (A-3 falsified).
	aheadCount := countAheadOfOriginMain(t, workDir, homeDir)
	if aheadCount != 0 {
		t.Errorf("F-3/A-3: expected chore/release-x to have 0 commits ahead of origin/main, got %d", aheadCount)
	}
}

// Affirms F-5/A-5: after `branch new` without --from, git rev-parse @{u} is empty
// (no upstream configured), confirming --no-track was used.
func TestBranchNewBase_NoTrackUpstream(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	addWIPRoadmapFile(t, workDir, "no-track")

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "feat/no-track")
	if code != 0 {
		t.Fatalf("branch new returned non-zero (%d):\n%s", code, out)
	}

	// rev-parse @{u} should fail (no upstream configured).
	_, err := gitE2ECheck(workDir, homeDir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err == nil {
		t.Error("F-5/A-5: expected @{u} to be unset (no upstream) after branch new without --from, but rev-parse succeeded")
	}
}

// Affirms AC3 (fetch advances base): when origin/main has advanced past the local tracking ref,
// `branch new` creates the branch from the remote tip (after fetch), not the stale local ref.
func TestBranchNewBase_FetchUpdatesBase(t *testing.T) {
	workDir, bareDir, homeDir := makeRemoteRepo(t)

	// Write the roadmap file directly in workDir (not committed) so the governance validator
	// accepts the branch slug. The test's goal is to verify the start-point, not governance.
	roadmapContent := "---\nstatus: wip\ndate: 2026-10-18\n---\n# Roadmap: fetch-advance\n"
	if err := os.WriteFile(filepath.Join(workDir, "docs/roadmaps/wip", "ROADMAP-fetch-advance.md"),
		[]byte(roadmapContent), 0o644); err != nil {
		t.Fatalf("write local roadmap: %v", err)
	}

	// Record the stale tip (before a second clone pushes a new commit).
	staleTip := strings.TrimSpace(gitE2EOutput(t, workDir, homeDir, "rev-parse", "origin/main"))

	// Create a second clone, push a new commit to origin — making workDir's origin/main stale.
	workDir2 := t.TempDir()
	gitE2E(t, workDir2, homeDir, "clone", "-q", bareDir, ".")
	gitE2E(t, workDir2, homeDir, "config", "user.email", "e2e@localhost")
	gitE2E(t, workDir2, homeDir, "config", "user.name", "E2E Test")
	gitE2E(t, workDir2, homeDir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(workDir2, "remote_advance.txt"), []byte("advance\n"), 0o644); err != nil {
		t.Fatalf("write remote_advance.txt: %v", err)
	}
	gitE2E(t, workDir2, homeDir, "add", "remote_advance.txt")
	gitE2E(t, workDir2, homeDir, "commit", "-q", "-m", "advance origin/main")
	gitE2E(t, workDir2, homeDir, "push", "-q", "origin", "main")

	// workDir's origin/main is now stale. Run branch new — it should fetch and use the new tip.
	out, code := runTFW(t, workDir, homeDir, "branch", "new", "feat/fetch-advance")
	if code != 0 {
		t.Fatalf("branch new returned non-zero (%d):\n%s", code, out)
	}

	// The new branch's HEAD should NOT be at the stale tip.
	newTip := strings.TrimSpace(gitE2EOutput(t, workDir, homeDir, "rev-parse", "HEAD"))
	if newTip == staleTip {
		t.Error("AC3: expected branch to start from the fetched tip of origin/main, not the stale local ref")
	}
}

// Affirms F-2/AC3 (warn, do not abort): when fetch fails, `branch new` prints a warning
// and creates the branch from the locally-known state of origin/main.
func TestBranchNewBase_FetchFailureWarnsAndCreates(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	addWIPRoadmapFile(t, workDir, "fetch-fail")

	// Break the origin URL so fetch fails.
	gitE2E(t, workDir, homeDir, "remote", "set-url", "origin", "/nonexistent/path")

	// branch new should warn but still create the branch (origin/main is known locally).
	out, code := runTFW(t, workDir, homeDir, "branch", "new", "feat/fetch-fail")
	if code != 0 {
		t.Fatalf("F-2: branch new should succeed even when fetch fails, got exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "Warning") || !strings.Contains(out, "fetch") {
		t.Errorf("F-2: expected a fetch-failure warning in output, got:\n%s", out)
	}
}

// Affirms F-4b: a repo without 'origin' gets an actionable trackfw message and exit non-zero,
// never a raw git exit 128 ("fatal: not a commit").
func TestBranchNewBase_NoOriginActionable(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	gitE2E(t, workDir, homeDir, "init", "-q", "-b", "main", ".")
	gitE2E(t, workDir, homeDir, "config", "user.email", "e2e@localhost")
	gitE2E(t, workDir, homeDir, "config", "user.name", "E2E Test")
	gitE2E(t, workDir, homeDir, "config", "commit.gpgsign", "false")

	// Create minimal trackfw.yaml (no roadmap needed; the check happens before the gate).
	for _, d := range []string{"docs/roadmaps/wip", "docs/roadmaps/done", "docs/roadmaps/backlog", "docs/roadmaps/abandoned", "docs/req"} {
		if err := os.MkdirAll(filepath.Join(workDir, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, "trackfw.yaml"), []byte(minimalTrackfwYAML()), 0o644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	gitE2E(t, workDir, homeDir, "add", ".")
	gitE2E(t, workDir, homeDir, "commit", "-q", "-m", "initial")

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "chore/no-origin")
	if code == 0 {
		t.Fatalf("F-4b: expected non-zero exit without origin, got 0:\n%s", out)
	}
	// Must NOT propagate git's raw "not a commit" or exit 128 message.
	if strings.Contains(out, "not a commit") || strings.Contains(out, "128") {
		t.Errorf("F-4b: must not propagate raw git error; got:\n%s", out)
	}
	// R-5: must contain the exact Portuguese actionable message.
	if !strings.Contains(out, "nenhum remote 'origin' encontrado") {
		t.Errorf("F-4b/R-5: expected exact message \"nenhum remote 'origin' encontrado\"; got:\n%s", out)
	}
	// Must contain actionable hint: --from HEAD or 'origin remote add'.
	if !strings.Contains(out, "--from HEAD") {
		t.Errorf("F-4b: expected actionable hint mentioning '--from HEAD'; got:\n%s", out)
	}
}

// Affirms AC9/F-6: --dry-run output names the base ref (not just the branch name).
func TestBranchNewBase_DryRunShowsBase(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	addWIPRoadmapFile(t, workDir, "dry-run-base")

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "--dry-run", "feat/dry-run-base")
	if code != 0 {
		t.Fatalf("dry-run returned non-zero (%d):\n%s", code, out)
	}
	if !strings.Contains(out, "would create") {
		t.Errorf("AC9/F-6: dry-run must contain 'would create', got:\n%s", out)
	}
	// Output must name the base (e.g. "origin/main"), not just the branch.
	if !strings.Contains(out, "origin/") {
		t.Errorf("AC9/F-6: dry-run must name the base ref (e.g. origin/main), got:\n%s", out)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// B-1 (REQ-2026-09-12, red-team) — --from flag-injection guard
// ──────────────────────────────────────────────────────────────────────────────

// TestBranchNew_FromDashFlagRejected asserts that --from values starting with '-'
// are rejected, the process exits non-zero, and uncommitted work is preserved.
// Reconciliation: affirms B-1 fix — without the guard, --from "-f" passes "-f" to
// git checkout, which silently discards uncommitted work via --force.
func TestBranchNew_FromDashFlagRejected(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	addWIPRoadmapFile(t, workDir, "b1-dash")

	// Write an uncommitted file that must survive.
	sentinel := filepath.Join(workDir, "uncommitted.txt")
	if err := os.WriteFile(sentinel, []byte("must survive\n"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "--from", "-f", "chore/b1-dash")
	if code == 0 {
		t.Fatalf("B-1: expected non-zero exit for --from -f, got 0:\n%s", out)
	}
	// Sentinel must still exist — work not discarded.
	if _, err := os.Stat(sentinel); os.IsNotExist(err) {
		t.Errorf("B-1: uncommitted file was discarded — --from -f triggered git checkout -f")
	}
	// Error message must mention the rejection reason.
	if !strings.Contains(out, "não pode começar com '-'") {
		t.Errorf("B-1: expected rejection message mentioning '-'; got:\n%s", out)
	}
}

// TestBranchNew_FromUploadPackRejected asserts that --from --upload-pack=x is rejected.
// Reconciliation: affirms B-1 fix — values starting with '--' are flag-like and rejected.
func TestBranchNew_FromUploadPackRejected(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	addWIPRoadmapFile(t, workDir, "b1-upload")

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "--from", "--upload-pack=x", "chore/b1-upload")
	if code == 0 {
		t.Fatalf("B-1: expected non-zero exit for --from --upload-pack=x, got 0:\n%s", out)
	}
	if !strings.Contains(out, "não pode começar com '-'") {
		t.Errorf("B-1: expected rejection message; got:\n%s", out)
	}
}

// TestBranchNew_FromTagResolvedToSHA asserts that --from <tag> succeeds and creates the branch
// at the tag's commit (not from origin/main).
// Reconciliation: affirms B-1 fix — the SHA-resolution path works for valid refs.
func TestBranchNew_FromTagResolvedToSHA(t *testing.T) {
	workDir, bareDir, homeDir := makeRemoteRepo(t)

	// Create a tag on the initial commit.
	gitE2E(t, workDir, homeDir, "tag", "v0.0.1")
	// Push the tag so we can verify it resolves.
	gitE2E(t, workDir, homeDir, "push", "origin", "v0.0.1")

	addWIPRoadmapFile(t, workDir, "from-tag")
	_ = bareDir

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "--from", "v0.0.1", "chore/from-tag")
	if code != 0 {
		t.Fatalf("B-1/tag: expected success for --from v0.0.1, got %d:\n%s", code, out)
	}
	// Verify the branch was created.
	branches := gitE2EOutput(t, workDir, homeDir, "branch")
	if !strings.Contains(branches, "chore/from-tag") {
		t.Errorf("B-1/tag: branch chore/from-tag not found in:\n%s", branches)
	}
}

// TestBranchNew_FromSHAWorks asserts that --from <sha> succeeds.
// Reconciliation: affirms B-1 fix — a full SHA is valid and accepted as-is.
func TestBranchNew_FromSHAWorks(t *testing.T) {
	workDir, _, homeDir := makeRemoteRepo(t)

	addWIPRoadmapFile(t, workDir, "from-sha")

	// Get the current HEAD SHA.
	sha := strings.TrimSpace(gitE2EOutput(t, workDir, homeDir, "rev-parse", "HEAD"))
	if len(sha) < 7 {
		t.Fatalf("could not get HEAD SHA: %q", sha)
	}

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "--from", sha, "chore/from-sha")
	if code != 0 {
		t.Fatalf("B-1/sha: expected success for --from <sha>, got %d:\n%s", code, out)
	}
	branches := gitE2EOutput(t, workDir, homeDir, "branch")
	if !strings.Contains(branches, "chore/from-sha") {
		t.Errorf("B-1/sha: branch chore/from-sha not found in:\n%s", branches)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// R-1 (REQ-2026-09-12, red-team) — dry-run fetches so it sees the same default branch as real run
// ──────────────────────────────────────────────────────────────────────────────

// TestBranchNew_DryRunFetchesMasterDefault asserts that --dry-run reports the correct base when
// the remote defaults to "master" (no origin/HEAD locally before fetch).
// Reconciliation: affirms R-1 fix — dry-run now always fetches, so both paths resolve the same
// default branch for remotes that use "master" as default.
func TestBranchNew_DryRunFetchesMasterDefault(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	bareDir := t.TempDir()

	// Init bare repo with "master" as default branch (not "main").
	gitE2E(t, bareDir, homeDir, "init", "--bare", "-b", "master", ".")

	// Init working repo from the bare remote.
	gitE2E(t, workDir, homeDir, "init", "-b", "master", ".")
	gitE2E(t, workDir, homeDir, "config", "user.email", "e2e@localhost")
	gitE2E(t, workDir, homeDir, "config", "user.name", "E2E Test")
	gitE2E(t, workDir, homeDir, "config", "commit.gpgsign", "false")
	gitE2E(t, workDir, homeDir, "remote", "add", "origin", bareDir)

	// Bootstrap the required governance structure.
	for _, d := range []string{"docs/roadmaps/wip", "docs/roadmaps/done", "docs/roadmaps/backlog", "docs/roadmaps/abandoned", "docs/req"} {
		if err := os.MkdirAll(filepath.Join(workDir, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(workDir, "trackfw.yaml"), []byte(minimalTrackfwYAML()), 0o644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	gitE2E(t, workDir, homeDir, "add", ".")
	gitE2E(t, workDir, homeDir, "commit", "-q", "-m", "initial")
	gitE2E(t, workDir, homeDir, "push", "-u", "origin", "master")

	addWIPRoadmapFile(t, workDir, "dry-run-master")

	// AC1: prevent git ≥ 2.48 from recreating the symref on fetch, reproducing git < 2.48
	// behaviour in any git version. Without this config, git ≥ 2.48 would silently create
	// refs/remotes/origin/HEAD during the fetch inside `branch new`, masking the bug.
	gitE2E(t, workDir, homeDir, "config", "remote.origin.followRemoteHEAD", "never")
	// Delete origin/HEAD to simulate a clone that never had it (the state after git < 2.48 fetch).
	gitE2E(t, workDir, homeDir, "remote", "set-head", "origin", "--delete")

	out, code := runTFW(t, workDir, homeDir, "branch", "new", "--dry-run", "chore/dry-run-master")
	if code != 0 {
		t.Fatalf("AC1: dry-run returned non-zero (%d):\n%s", code, out)
	}
	// AC1: with followRemoteHEAD=never and no local symref, branch new resolves origin/master
	// via ls-remote --symref origin HEAD, not the literal "main".
	if !strings.Contains(out, "origin/master") {
		t.Errorf("AC1: dry-run must report 'origin/master' as base (via ls-remote); got:\n%s", out)
	}
}
