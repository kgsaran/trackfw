package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/kgsaran/trackfw/internal/validator"
)

// ────────────────────────────────────────────────────────────────────────────
// test helpers
// ────────────────────────────────────────────────────────────────────────────

// mockPushGit captures every call to execGit for push tests.
type mockPushGit struct {
	branch      string // returned for symbolic-ref --short HEAD
	remoteURL   string // returned for remote get-url origin
	hasUpstream bool   // true → @{u} resolves (push without -u); false → push -u
	calls       [][]string

	// D3/D4 (ADR-2026-11-26) fields.
	// pushURLs is returned for "remote get-url --push --all origin".
	// If empty, the mock returns "" (no URLs → checkRemoteEmpty returns error → fail-closed for D3,
	// fail-open for D4).
	pushURLs string
	// lsRemoteOut is returned for "ls-remote --heads <url>".
	// If empty, mock returns "" (no branches).
	lsRemoteOut string
	// revListCount is returned for "rev-list --count refs/heads/<branch>".
	// If empty, mock returns "" which triggers an error in D3 commit-count gate.
	revListCount string
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
	case strings.HasPrefix(joined, "remote get-url --push --all"):
		return m.pushURLs, nil
	case strings.HasPrefix(joined, "ls-remote --get-url"):
		// No insteadOf rewrite in mock — return the URL unchanged (args[2] is the URL).
		if len(args) >= 3 {
			return args[2], nil
		}
		return "", nil
	case strings.HasPrefix(joined, "ls-remote --heads"):
		return m.lsRemoteOut, nil
	case strings.HasPrefix(joined, "rev-list --count refs/heads/"):
		return m.revListCount, nil
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
		checkGovernance: func() *validator.GovernanceViolation {
			if violations == nil {
				return nil
			}
			return &validator.GovernanceViolation{Missing: violations}
		},
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
	deps, _, out := makePushDeps("feat/my-feature", false, []string{"no roadmap found in wip/, blocked/ nor done/"})
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
	deps, _, out := makePushDeps("feat/orphan", false, []string{"no roadmap found in wip/, blocked/ nor done/"})
	_ = runPush(pushOpts{dryRun: true}, deps)
	stdout := out.String()
	if !strings.Contains(stdout, "trackfw push") {
		t.Fatalf("governance message must say 'trackfw push', got: %q", stdout)
	}
	if strings.Contains(stdout, "trackfw ship") {
		t.Fatalf("governance message must NOT say 'trackfw ship', got: %q", stdout)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Degraded governance (Warnings present, Missing empty) — D3 of ADR-2026-10-01
// ────────────────────────────────────────────────────────────────────────────

// TestPush_GovernanceDegraded_PrintsDegradedNotOK asserts that when CheckShipGovernance
// returns a GovernanceViolation with non-empty Warnings and empty Missing (degraded — D3 of
// ADR-2026-10-01: unresolvable base), push prints "Governance: degraded:" and does NOT print
// "Governance: OK", and execution continues (push proceeds).
//
// Reconciliation: this test affirms that D3 of ADR-2026-10-01 is honoured — degraded governance
// (e.g. branch_done_scope_unverifiable) never blocks push; it warns and continues.
func TestPush_GovernanceDegraded_PrintsDegradedNotOK(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	m := &mockPushGit{branch: "feat/my-feature", hasUpstream: true}
	deps := pushDeps{
		execGit: m.exec,
		checkGovernance: func() *validator.GovernanceViolation {
			return &validator.GovernanceViolation{
				Missing:  nil,
				Warnings: []string{"branch_done_scope_unverifiable: cannot determine base commit"},
			}
		},
		out: out,
	}
	err := runPush(pushOpts{}, deps)
	if err != nil {
		t.Fatalf("degraded governance must not block push, got error: %v", err)
	}
	stdout := out.String()
	if !strings.Contains(stdout, "Governance: degraded:") {
		t.Errorf("degraded governance must print 'Governance: degraded:', got: %q", stdout)
	}
	if strings.Contains(stdout, "Governance: OK") {
		t.Errorf("degraded governance must NOT print 'Governance: OK', got: %q", stdout)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D3 (ADR-2026-11-26): bootstrap push for default branch
// ────────────────────────────────────────────────────────────────────────────

// TestPush_D3_Main_RemoteEmpty_OneCommit_Pushes affirms that when main has an
// empty remote and exactly one commit, D3 pushes with an absence lease and returns nil.
// Rule reconciliation: affirms the D3 happy path — the absence lease is the
// mechanism that lets a single bootstrap push through while blocking a second.
func TestPush_D3_Main_RemoteEmpty_OneCommit_Pushes(t *testing.T) {
	out := &bytes.Buffer{}
	m := &mockPushGit{
		branch:       "main",
		pushURLs:     "https://github.com/user/repo.git",
		lsRemoteOut:  "", // no branches
		revListCount: "1",
	}
	deps := pushDeps{execGit: m.exec, out: out}
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("expected D3 bootstrap push to succeed, got: %v", err)
	}
	// In dry-run mode, the push write command is not sent to the mock — it is
	// printed to stdout as "[dry-run] git push ...". Verify the absence lease
	// appears in the dry-run output.
	stdout := out.String()
	if !strings.Contains(stdout, "--force-with-lease=refs/heads/main:") {
		t.Errorf("D3 dry-run output must contain absence lease '--force-with-lease=refs/heads/main:'\ngot: %q", stdout)
	}
}

// TestPush_D3_Main_RemoteHasBranches_Blocked affirms D3 blocks when the remote
// already has branches (the normal post-bootstrap state).
// Rule reconciliation: affirms D3 refuses after the bootstrap push has been made;
// the second push is what the existing TestPush_Main_Blocks tested before D3.
func TestPush_D3_Main_RemoteHasBranches_Blocked(t *testing.T) {
	m := &mockPushGit{
		branch:      "main",
		pushURLs:    "https://github.com/user/repo.git",
		lsRemoteOut: "abc123def456abc123def456abc123def456abc1\trefs/heads/main\n",
	}
	deps := pushDeps{execGit: m.exec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D3 to block when remote already has branches")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}
}

// TestPush_D3_Main_TwoCommits_Blocked is the counter-arm for the single-commit gate.
// Rule reconciliation: affirms that D3 refuses if the branch has 2 commits — only the
// bootstrap commit (count==1) authorized by D2 may be pushed on the default branch.
func TestPush_D3_Main_TwoCommits_Blocked(t *testing.T) {
	m := &mockPushGit{
		branch:       "main",
		pushURLs:     "https://github.com/user/repo.git",
		lsRemoteOut:  "", // remote is empty
		revListCount: "2",
	}
	deps := pushDeps{execGit: m.exec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D3 to block when branch has 2 commits")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}
}

// TestPush_D3_Main_ForceWithLease_Refused affirms D3 refuses the user --force-with-lease
// flag before any remote query.
// Rule reconciliation: affirms that the absence lease (internal) is never confused with
// the user flag; the user flag is refused unconditionally on the default branch.
func TestPush_D3_Main_ForceWithLease_Refused(t *testing.T) {
	m := &mockPushGit{branch: "main"}
	deps := pushDeps{execGit: m.exec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{forceWithLease: true}, deps)
	if err == nil {
		t.Fatal("expected D3 to refuse --force-with-lease on main")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}
	// No push-URL query must have been made — refusal is before any query.
	for _, call := range m.calls {
		if strings.Join(call, " ") == "remote get-url --push --all origin" {
			t.Error("D3 must not query remote when refusing --force-with-lease flag")
		}
	}
}

// TestPush_D3_Main_RemoteUnreachable_FailClosed affirms D3 refuses when the remote
// cannot be queried (fail-closed).
// Rule reconciliation: affirms that "uncertain = refuse" for D3 — from ADR-2026-11-26 D3:
// "remoto ausente, sem resposta, autenticação recusada ou saída não reconhecida recusa".
func TestPush_D3_Main_RemoteUnreachable_FailClosed(t *testing.T) {
	// pushURLs is empty → checkRemoteEmpty returns error → D3 refuses.
	m := &mockPushGit{branch: "main", pushURLs: ""}
	deps := pushDeps{execGit: m.exec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D3 fail-closed when remote cannot be verified")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D4 (ADR-2026-11-26): block feature push to empty remote
// ────────────────────────────────────────────────────────────────────────────

// TestPush_D4_FeatureBranch_EmptyRemote_Blocked affirms that a feature branch push
// is blocked when the remote has no branches at all.
// Rule reconciliation: affirms the D4 precondition from ADR-2026-11-26 — "empty remote",
// not "no main/master", so that existing repos with a non-standard default branch are unaffected.
func TestPush_D4_FeatureBranch_EmptyRemote_Blocked(t *testing.T) {
	out := &bytes.Buffer{}
	m := &mockPushGit{
		branch:   "feat/my-feature",
		pushURLs: "https://github.com/user/repo.git",
		// lsRemoteOut is "" (no branches) — remote is empty.
	}
	deps := pushDeps{
		execGit:         m.exec,
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             out,
	}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D4 to block feature push to empty remote")
	}
	if !strings.Contains(err.Error(), "remote has no branches yet") {
		t.Errorf("expected 'remote has no branches yet' in error, got: %q", err.Error())
	}
}

// TestPush_D4_FeatureBranch_RemoteHasBranches_Passes is the counter-arm for D4.
// Rule reconciliation: affirms that D4 does NOT block when the remote already has
// at least one branch — normal feature branch push after bootstrap.
func TestPush_D4_FeatureBranch_RemoteHasBranches_Passes(t *testing.T) {
	out := &bytes.Buffer{}
	m := &mockPushGit{
		branch:      "feat/my-feature",
		pushURLs:    "https://github.com/user/repo.git",
		lsRemoteOut: "abc123def456abc123def456abc123def456abc1\trefs/heads/main\n",
	}
	deps := pushDeps{
		execGit:         m.exec,
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             out,
	}
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("expected D4 to pass when remote has branches, got: %v", err)
	}
}

// TestPush_D4_FeatureBranch_RemoteQueryFails_FailOpen affirms D4 is fail-open:
// if the remote cannot be queried, the push proceeds (network errors surface at push time).
// Rule reconciliation: affirms ADR-2026-11-26 D4 "Se a consulta falhar, o push da feature
// segue como hoje" — D4 failure must not become a new blocking gate.
func TestPush_D4_FeatureBranch_RemoteQueryFails_FailOpen(t *testing.T) {
	out := &bytes.Buffer{}
	// pushURLs is "" → checkRemoteEmpty returns error → D4 fail-open → push proceeds.
	m := &mockPushGit{
		branch:   "feat/my-feature",
		pushURLs: "",
	}
	deps := pushDeps{
		execGit:         m.exec,
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             out,
	}
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("expected D4 fail-open when query fails, got: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// parseLSRemoteHeads unit tests
// ────────────────────────────────────────────────────────────────────────────

// TestParseLSRemoteHeads_Empty affirms that empty output is clean (no branches, no unrecognized).
// Rule reconciliation: affirms the "empty remote" happy path that D3 and D4 depend on.
func TestParseLSRemoteHeads_Empty(t *testing.T) {
	hasBranches, unrecognized := parseLSRemoteHeads("")
	if hasBranches || unrecognized {
		t.Errorf("empty output: hasBranches=%v unrecognized=%v, expected both false", hasBranches, unrecognized)
	}
}

// TestParseLSRemoteHeads_OneBranch affirms a single valid branch line is detected.
// Rule reconciliation: affirms D3/D4 refuse when the remote has at least one branch.
func TestParseLSRemoteHeads_OneBranch(t *testing.T) {
	out := "abc123def456abc123def456abc123def456abc1\trefs/heads/main\n"
	hasBranches, unrecognized := parseLSRemoteHeads(out)
	if !hasBranches {
		t.Error("expected hasBranches=true for a line with refs/heads/")
	}
	if unrecognized {
		t.Error("expected unrecognized=false for a valid ls-remote line")
	}
}

// TestParseLSRemoteHeads_NonHeadRef affirms that a non-heads ref (e.g., refs/tags/)
// is not counted as a branch but is not unrecognized.
// Rule reconciliation: affirms that existing repos with tags on a fresh clone are
// not misclassified as "has branches" by D3/D4.
func TestParseLSRemoteHeads_NonHeadRef(t *testing.T) {
	out := "abc123def456abc123def456abc123def456abc1\trefs/tags/v1.0.0\n"
	hasBranches, unrecognized := parseLSRemoteHeads(out)
	if hasBranches {
		t.Error("expected hasBranches=false for refs/tags/ line")
	}
	if unrecognized {
		t.Error("expected unrecognized=false for a valid (non-head) ls-remote line")
	}
}

// TestParseLSRemoteHeads_MalformedLine affirms that a line without a tab is unrecognized.
// Rule reconciliation: affirms D3 fail-closed on malformed output — "saída não reconhecida recusa".
func TestParseLSRemoteHeads_MalformedLine(t *testing.T) {
	out := "not-a-valid-line\n"
	_, unrecognized := parseLSRemoteHeads(out)
	if !unrecognized {
		t.Error("expected unrecognized=true for a line without a tab")
	}
}
