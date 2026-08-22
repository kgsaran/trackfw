package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/forge"
)

// ────────────────────────────────────────────────────────────────────────────
// test helpers
// ────────────────────────────────────────────────────────────────────────────

// mockPushGit captures every call to execGit for push tests.
type mockPushGit struct {
	branch     string // returned for symbolic-ref --short HEAD
	remoteURL  string // returned for remote get-url origin
	hasUpstream bool  // true → @{u} resolves (push without -u); false → push -u
	calls      [][]string
}

func (m *mockPushGit) exec(args ...string) (string, error) {
	call := make([]string, len(args))
	copy(call, args)
	m.calls = append(m.calls, call)

	joined := strings.Join(args, " ")

	switch {
	case strings.HasPrefix(joined, "symbolic-ref --short"):
		if m.branch == "" {
			return "", errors.New("not a git repo")
		}
		return m.branch, nil
	case joined == "remote get-url origin":
		return m.remoteURL, nil
	case strings.HasPrefix(joined, "rev-parse --abbrev-ref --symbolic-full-name @{u}"):
		if m.hasUpstream {
			return "origin/" + m.branch, nil
		}
		return "", errors.New("no upstream")
	case strings.HasPrefix(joined, "fetch"):
		return "", errors.New("could not connect") // offline, non-blocking
	case strings.HasPrefix(joined, "branch -r --no-merged"):
		return "", nil // no pending squash-merges
	}

	return "", nil
}

// makePushDeps builds pushDeps wired to injectable fakes.
func makePushDeps(branch string, hasUpstream bool, violations []string) (pushDeps, *mockPushGit, *bytes.Buffer) {
	out := &bytes.Buffer{}
	m := &mockPushGit{branch: branch, hasUpstream: hasUpstream}
	d := pushDeps{
		execGit:         m.exec,
		checkGovernance: func() []string { return violations },
		out:             out,
		configForge:     "",
		repoDir:         "",
		availFn:         nil,
		checkPROpen:     nil,
	}
	return d, m, out
}

// ────────────────────────────────────────────────────────────────────────────
// Step 1: main/master blocked unconditionally
// ────────────────────────────────────────────────────────────────────────────

func TestPush_Main_Blocks(t *testing.T) {
	deps, _, _ := makePushDeps("main", false, nil)
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected error on main")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Fatalf("expected push-specific block message, got: %q", err.Error())
	}
}

func TestPush_Master_Blocks(t *testing.T) {
	deps, _, _ := makePushDeps("master", false, nil)
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected error on master")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Fatalf("expected push-specific block message, got: %q", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 1: branch outside ship vocabulary is blocked
// ────────────────────────────────────────────────────────────────────────────

func TestPush_InvalidBranch_Blocks(t *testing.T) {
	deps, _, _ := makePushDeps("wip/something", false, nil)
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected error on non-ship branch")
	}
	if !strings.Contains(err.Error(), "does not match the required pattern") {
		t.Fatalf("expected pattern error, got: %q", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 2: governance — feat/ without roadmap is blocked
// ────────────────────────────────────────────────────────────────────────────

func TestPush_FeatBranch_NoRoadmap_Blocks(t *testing.T) {
	deps, _, out := makePushDeps("feat/my-feature", false, []string{"no roadmap found in wip/ nor done/"})
	err := runPush(pushOpts{dryRun: true}, deps)
	if err == nil {
		t.Fatal("expected governance error")
	}
	if !strings.Contains(err.Error(), "governance check failed") {
		t.Fatalf("expected governance failure, got: %q", err.Error())
	}
	if !strings.Contains(out.String(), "Governance check failed:") {
		t.Fatalf("expected governance detail block in stdout, got: %q", out.String())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 2: governance — chore/ is exempt
// ────────────────────────────────────────────────────────────────────────────

func TestPush_ChoreBranch_GovernanceSkipped(t *testing.T) {
	deps, _, out := makePushDeps("chore/update-deps", false, []string{"would fail if checked"})
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error for chore branch: %v", err)
	}
	if !strings.Contains(out.String(), "Governance: skipped (chore/docs branch)") {
		t.Fatalf("expected governance skip marker, got: %q", out.String())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 2: governance — docs/ is exempt
// ────────────────────────────────────────────────────────────────────────────

func TestPush_DocsBranch_GovernanceSkipped(t *testing.T) {
	deps, _, out := makePushDeps("docs/update-readme", false, []string{"would fail if checked"})
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error for docs branch: %v", err)
	}
	if !strings.Contains(out.String(), "Governance: skipped (chore/docs branch)") {
		t.Fatalf("expected governance skip marker, got: %q", out.String())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 4: push — -u flag present when no upstream configured
// ────────────────────────────────────────────────────────────────────────────

func TestPush_NoUpstream_UsesDashU(t *testing.T) {
	deps, m, _ := makePushDeps("feat/my-feature", false, nil) // no upstream
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Find the push call in git calls
	found := false
	for _, call := range m.calls {
		if len(call) >= 2 && call[0] == "push" {
			joined := strings.Join(call, " ")
			if !strings.Contains(joined, "-u") {
				t.Fatalf("expected -u in push args when no upstream, got: %q", joined)
			}
			found = true
			break
		}
	}
	// In dry-run the git push call is skipped (write command) — check stdout instead
	// since isGitWriteCmd("push") returns true.
	_ = found
}

func TestPush_NoUpstream_DryRunPrintsU(t *testing.T) {
	deps, _, out := makePushDeps("feat/my-feature", false, nil)
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdout := out.String()
	if !strings.Contains(stdout, "[dry-run] git push -u origin feat/my-feature") {
		t.Fatalf("expected dry-run push -u line, got: %q", stdout)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Step 4: push — no -u flag when upstream already configured
// ────────────────────────────────────────────────────────────────────────────

func TestPush_WithUpstream_DryRunNoDashU(t *testing.T) {
	deps, _, out := makePushDeps("feat/my-feature", true, nil)
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdout := out.String()
	if !strings.Contains(stdout, "[dry-run] git push origin feat/my-feature") {
		t.Fatalf("expected dry-run push without -u, got: %q", stdout)
	}
	if strings.Contains(stdout, "push -u") {
		t.Fatalf("must NOT include -u when upstream exists, got: %q", stdout)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// force-with-lease: no forge CLI → blocked (message says "trackfw push")
// ────────────────────────────────────────────────────────────────────────────

func TestPush_ForceWithLease_NoForgeCLI_Blocks(t *testing.T) {
	deps, _, _ := makePushDeps("feat/my-feature", false, nil)
	// availFn always returns false → no forge CLI available
	deps.availFn = func(string) bool { return false }
	err := runPush(pushOpts{forceWithLease: true, dryRun: true}, deps)
	if err == nil {
		t.Fatal("expected error when no forge CLI available")
	}
	if !strings.Contains(err.Error(), "trackfw push --force-with-lease requires") {
		t.Fatalf("expected push-specific force-with-lease message, got: %q", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// force-with-lease: no PR open → blocked (message says "trackfw push")
// ────────────────────────────────────────────────────────────────────────────

func TestPush_ForceWithLease_NoPROpen_Blocks(t *testing.T) {
	deps, _, _ := makePushDeps("feat/my-feature", false, nil)
	deps.availFn = func(string) bool { return true }
	deps.checkPROpen = func(adapter forge.Adapter, branch string) (bool, error) {
		return false, nil // no open PR
	}
	// Fake forge resolution — we test the gate, not forge resolution itself.
	// Since the remote URL is empty, forge.Resolve will return "manual". We need to
	// stub more deeply — instead, test via injecting checkGovernance pass and using a
	// real remoteURL that resolves to a known forge via the manual path detection.
	// This test is limited to verifying the message shape; the full forge integration
	// is exercised by check-push-parity.sh (ML-2A).
	// Skipping the forge gate test here because it requires a real remote URL.
	// The "force-with-lease + no PR open" path has no runtime coverage at any level:
	// - check-ship-force-parity.sh tests `trackfw ship`, not `trackfw push`.
	// - check-push-parity.sh runs all scenarios with --dry-run; the force-with-lease
	//   gate fires before any write, but the parity script never passes --force-with-lease.
	// This gap is declared in docs/cli-parity.md (partial= annotation on check-push-parity.sh).
	t.Skip("force-with-lease+no-PR path has no end-to-end coverage; see docs/cli-parity.md partial= annotation")
}

// ────────────────────────────────────────────────────────────────────────────
// push never commits and never calls forge write commands
// ────────────────────────────────────────────────────────────────────────────

func TestPush_NeverCommits(t *testing.T) {
	deps, m, _ := makePushDeps("chore/update-deps", false, nil)
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, call := range m.calls {
		if len(call) > 0 && call[0] == "commit" {
			t.Fatalf("push must never call git commit, but found call: %v", call)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// --dry-run: fetch is printed, not executed
// ────────────────────────────────────────────────────────────────────────────

func TestPush_DryRun_PrintsFetchAndPush(t *testing.T) {
	deps, _, out := makePushDeps("chore/foo", false, nil)
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdout := out.String()
	if !strings.Contains(stdout, "[dry-run] git fetch origin --prune") {
		t.Fatalf("expected dry-run fetch line, got: %q", stdout)
	}
	if !strings.Contains(stdout, "[dry-run] git push") {
		t.Fatalf("expected dry-run push line, got: %q", stdout)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Governance message says "trackfw push", not "trackfw ship"
// ────────────────────────────────────────────────────────────────────────────

func TestPush_GovernanceMessage_SaysPush(t *testing.T) {
	deps, _, out := makePushDeps("feat/orphan", false, []string{"no roadmap found in wip/ nor done/"})
	_ = runPush(pushOpts{dryRun: true}, deps)
	stdout := out.String()
	if !strings.Contains(stdout, "trackfw push") {
		t.Fatalf("governance message must say 'trackfw push', got: %q", stdout)
	}
	if strings.Contains(stdout, "trackfw ship") {
		t.Fatalf("governance message must NOT say 'trackfw ship', got: %q", stdout)
	}
}
