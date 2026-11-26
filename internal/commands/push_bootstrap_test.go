package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/validator"
)

// ────────────────────────────────────────────────────────────────────────────
// D3/D4 real-git integration tests (ADR-2026-11-26)
//
// These tests use real git repos (via gitRepoIn from commit_unborn_branch_test.go)
// to confirm that the predicates in checkRemoteEmpty and parseLSRemoteHeads
// measure what git actually reports, not just what the injected stubs return.
// ────────────────────────────────────────────────────────────────────────────

// makeBareRepo creates a bare git repository in bareDir and returns its path.
// Uses gitRepoIn to initialize it cleanly.
func makeBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRepoIn(t, dir, []string{"init", "--bare", "-q", "."})
	return dir
}

// makeRepoWithRemote creates a local repo in localDir, adds a commit if
// addCommit is true, and sets the bare repo at bareDir as origin.
func makeRepoWithRemote(t *testing.T, localDir, bareDir string, addCommit bool) {
	t.Helper()
	gitRepoIn(t, localDir,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
		[]string{"remote", "add", "origin", bareDir},
	)
	if addCommit {
		if err := os.WriteFile(filepath.Join(localDir, "a.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		gitRepoIn(t, localDir,
			[]string{"add", "a.txt"},
			[]string{"commit", "-q", "-m", "init"},
		)
	}
}

// TestCheckRemoteEmpty_RealGit_FreshBareRepo affirms that a fresh bare repo
// returns (true, nil) from checkRemoteEmpty.
// Rule reconciliation: affirms the D3/D4 "empty remote" detection path — the
// mock-based TestPush_D3_Main_RemoteEmpty_OneCommit_Pushes is the injection-based counterpart.
func TestCheckRemoteEmpty_RealGit_FreshBareRepo(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	makeRepoWithRemote(t, local, bare, false)
	t.Chdir(local)

	empty, err := checkRemoteEmpty(defaultGitExec)
	if err != nil {
		t.Fatalf("unexpected error for fresh bare repo: %v", err)
	}
	if !empty {
		t.Error("expected empty=true for a bare repo with no branches, got false")
	}
}

// TestCheckRemoteEmpty_RealGit_BareRepoWithBranch affirms that after pushing a
// commit to a bare repo, checkRemoteEmpty returns (false, nil).
// Rule reconciliation: affirms D3/D4 refuse after the bootstrap push — the second
// push on main would reach this state and be blocked.
func TestCheckRemoteEmpty_RealGit_BareRepoWithBranch(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	makeRepoWithRemote(t, local, bare, true)
	// Push the commit to the bare repo so it has a branch.
	gitRepoIn(t, local, []string{"push", "origin", "main"})
	t.Chdir(local)

	empty, err := checkRemoteEmpty(defaultGitExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if empty {
		t.Error("expected empty=false after pushing a branch, got true")
	}
}

// TestD3_RealGit_FreshRemote_OneCommit_D3Pushes affirms that D3 successfully
// pushes the default branch when the remote is empty and the branch has exactly
// one commit. The push is done without dryRun; we verify by checking the bare
// repo afterwards.
// Rule reconciliation: affirms the full D3 happy path — after the push, the
// bare repo has main, and a second push with the same setup would be blocked
// by TestD3_RealGit_RemoteAlreadyHasMain_Blocked.
func TestD3_RealGit_FreshRemote_OneCommit_D3Pushes(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	makeRepoWithRemote(t, local, bare, true)
	t.Chdir(local)

	deps := pushDeps{
		execGit: defaultGitExec,
		out:     &bytes.Buffer{},
	}
	err := runPush(pushOpts{}, deps)
	if err != nil {
		t.Fatalf("expected D3 bootstrap push to succeed, got: %v", err)
	}

	// Verify the push landed: ls-remote should now show refs/heads/main.
	lsOut, lsErr := defaultGitExec("ls-remote", "--heads", bare)
	if lsErr != nil {
		t.Fatalf("ls-remote after D3 push: %v", lsErr)
	}
	if !strings.Contains(lsOut, "refs/heads/main") {
		t.Errorf("expected refs/heads/main in bare repo after D3 push, got: %q", lsOut)
	}
}

// TestD3_RealGit_TwoCommits_Blocked is the counter-arm for the D3 commit gate.
// Rule reconciliation: affirms that D3 refuses when the branch has 2 commits — only
// the single bootstrap commit is authorized by D2.
func TestD3_RealGit_TwoCommits_Blocked(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	makeRepoWithRemote(t, local, bare, true)
	// Add a second commit.
	if err := os.WriteFile(filepath.Join(local, "b.txt"), []byte("y\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRepoIn(t, local,
		[]string{"add", "b.txt"},
		[]string{"commit", "-q", "-m", "second"},
	)
	t.Chdir(local)

	deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D3 to block when branch has 2 commits")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}
}

// TestD3_RealGit_RemoteAlreadyHasMain_Blocked affirms that after the bootstrap
// push, a second attempt on main is blocked by D3 (remote already has branches).
// Rule reconciliation: affirms the D3 "exactly once" guarantee — the absence
// lease ensures the server rejects, but the local D3 guard refuses first.
func TestD3_RealGit_RemoteAlreadyHasMain_Blocked(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	makeRepoWithRemote(t, local, bare, true)
	// Pre-push to simulate a repo that already has main.
	gitRepoIn(t, local, []string{"push", "origin", "main"})
	t.Chdir(local)

	deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D3 to block after remote already has main")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}
}

// TestD4_RealGit_FeatureBranch_EmptyRemote_Blocked affirms D4 blocks a real
// feature branch push when the remote is empty.
// Rule reconciliation: affirms D4 closes the path where a feature branch occupies
// the remote before main — the scenario described in ADR-2026-11-26 §Decision.
func TestD4_RealGit_FeatureBranch_EmptyRemote_Blocked(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	makeRepoWithRemote(t, local, bare, true)
	// Switch to a feature branch.
	gitRepoIn(t, local, []string{"checkout", "-q", "-b", "feat/my-feature"})
	t.Chdir(local)

	deps := pushDeps{
		execGit:         defaultGitExec,
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             &bytes.Buffer{},
	}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D4 to block feature push to empty remote")
	}
	if !strings.Contains(err.Error(), "remote has no branches yet") {
		t.Errorf("expected 'remote has no branches yet' in error, got: %q", err.Error())
	}
}

// TestD4_RealGit_FeatureBranch_RemoteHasMain_Passes is the counter-arm for D4.
// Rule reconciliation: affirms D4 does NOT block after main is already on the
// remote — normal feature push post-bootstrap proceeds.
func TestD4_RealGit_FeatureBranch_RemoteHasMain_Passes(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	makeRepoWithRemote(t, local, bare, true)
	// Bootstrap: push main first.
	gitRepoIn(t, local, []string{"push", "origin", "main"})
	// Switch to a feature branch.
	gitRepoIn(t, local, []string{"checkout", "-q", "-b", "feat/my-feature"})
	t.Chdir(local)

	deps := pushDeps{
		execGit:         defaultGitExec,
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             &bytes.Buffer{},
	}
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("expected D4 to pass after main is on remote, got: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// D3 — pushurl-related and absence-lease real-git tests (ML-1F, ADR-2026-11-26)
// ────────────────────────────────────────────────────────────────────────────

// makeBareWithMain creates a bare repo in a fresh TempDir and pushes one commit
// from a seed repo so that refs/heads/main exists in the bare.
func makeBareWithMain(t *testing.T) (bareDir, seedSHA string) {
	t.Helper()
	bare := makeBareRepo(t)
	seed := t.TempDir()
	makeRepoWithRemote(t, seed, bare, true)
	gitRepoIn(t, seed, []string{"push", "origin", "main"})

	lsOut, lsErr := defaultGitExec("ls-remote", "--heads", bare)
	if lsErr != nil {
		t.Fatalf("ls-remote on seed bare: %v", lsErr)
	}
	for _, line := range strings.Split(lsOut, "\n") {
		if strings.Contains(line, "refs/heads/main") {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				return bare, fields[0]
			}
		}
	}
	t.Fatal("seed bare has no refs/heads/main after push")
	return "", ""
}

// TestD3_RealGit_PushURL_ExistingMain_Blocked affirms that when origin's url is an
// empty bare but the pushurl points to a bare that already has main, D3 is blocked.
// checkRemoteEmpty queries push URLs (not the fetch URL), so it sees main and returns
// false — the governed push is refused before any write occurs.
//
// Rule reconciliation: affirms D3 (ADR-2026-11-26) closes the vector where pushurl ≠
// url: checking only the fetch URL would give a false "empty" result.
// Counter-arm: TestD3_RealGit_FreshRemote_OneCommit_D3Pushes (pushurl=url=empty → pushes).
func TestD3_RealGit_PushURL_ExistingMain_Blocked(t *testing.T) {
	bareWithMain, _ := makeBareWithMain(t)
	bareEmpty := makeBareRepo(t)

	local := t.TempDir()
	// url = empty bare; pushurl = bare with main
	makeRepoWithRemote(t, local, bareEmpty, true)
	gitRepoIn(t, local, []string{"remote", "set-url", "--push", "origin", bareWithMain})
	t.Chdir(local)

	deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D3 to block when pushurl already has main, got nil error")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}

	// The empty bare (url) must remain empty — no writes occurred.
	lsOut, _ := defaultGitExec("ls-remote", "--heads", bareEmpty)
	if strings.Contains(lsOut, "refs/heads/main") {
		t.Error("url (empty bare) must remain empty after D3 blocked push via pushurl divergence")
	}
}

// TestD3_RealGit_TwoPushURLs_SecondHasMain_Blocked affirms that with two pushurls
// where the first is empty and the second already has main, D3 is blocked.
// checkRemoteEmpty iterates all push URLs; finding main in any of them returns false.
//
// Rule reconciliation: affirms D3 covers the multi-pushurl case — a partial empty
// is not sufficient to allow the governed bootstrap push.
// Counter-arm: TestD3_RealGit_FreshRemote_OneCommit_D3Pushes (single empty pushurl → pushes).
func TestD3_RealGit_TwoPushURLs_SecondHasMain_Blocked(t *testing.T) {
	bareSecond, _ := makeBareWithMain(t)
	bareFirst := makeBareRepo(t) // empty

	local := t.TempDir()
	makeRepoWithRemote(t, local, bareFirst, true) // initial url = bareFirst
	// Set first pushurl explicitly to bareFirst (same as url, empty)
	gitRepoIn(t, local, []string{"remote", "set-url", "--push", "origin", bareFirst})
	// Add second pushurl that already has main
	gitRepoIn(t, local, []string{"remote", "set-url", "--add", "--push", "origin", bareSecond})
	t.Chdir(local)

	deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected D3 to block when second pushurl already has main, got nil error")
	}
	if !strings.Contains(err.Error(), "trackfw push cannot run on") {
		t.Errorf("expected 'trackfw push cannot run on' in error, got: %q", err.Error())
	}

	// The first (empty) bare must remain empty — the blocked push did not write to it.
	lsOut, _ := defaultGitExec("ls-remote", "--heads", bareFirst)
	if strings.Contains(lsOut, "refs/heads/main") {
		t.Error("first pushurl (empty bare) must remain empty after D3 blocked push")
	}
}

// TestD3_RealGit_AbsenceLease_BlocksPushWhenMainExists tests the TOCTOU window:
// the local check is stubbed to report an empty remote, but the actual push runs
// against a bare that already has main. The absence lease
// (--force-with-lease=refs/heads/main:) must reject the push, leaving the remote's
// main SHA unchanged.
//
// Rule reconciliation: affirms the destination's main is never overwritten —
// even when the local race-window check saw empty, the absence lease on the server
// side prevents the write. The arm fails if absenceLease is changed to --force.
// Counter-arm: TestD3_RealGit_FreshRemote_OneCommit_D3Pushes (empty remote → pushes).
func TestD3_RealGit_AbsenceLease_BlocksPushWhenMainExists(t *testing.T) {
	bareWithMain, seedSHA := makeBareWithMain(t)

	// Create a local repo with DISTINCT content so its commit SHA differs from the
	// seed's. If both repos produce the same SHA (same content, author, message,
	// second), git sees a no-op push and skips the absence-lease check entirely.
	local := t.TempDir()
	gitRepoIn(t, local,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
		[]string{"remote", "add", "origin", bareWithMain},
	)
	if err := os.WriteFile(filepath.Join(local, "bootstrap.txt"), []byte("bootstrap-local\n"), 0o644); err != nil {
		t.Fatalf("write bootstrap.txt: %v", err)
	}
	gitRepoIn(t, local,
		[]string{"add", "bootstrap.txt"},
		[]string{"commit", "-q", "-m", "bootstrap: initial scaffold"},
	)
	t.Chdir(local)

	// Inject execGit: stub ls-remote to return empty (simulate the TOCTOU window —
	// the check saw an empty remote, but main was already there by push time).
	// All other calls (symbolic-ref, remote get-url, rev-list, push) use real git.
	stubExec := func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "ls-remote" {
			return "", nil // simulate: remote appeared empty during check
		}
		return defaultGitExec(args...)
	}

	deps := pushDeps{execGit: stubExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected absence lease to block push when main already exists on remote; got nil error")
	}

	// The remote's main SHA must be unchanged — the push did not write to the remote.
	lsOut2, lsErr := defaultGitExec("ls-remote", "--heads", bareWithMain)
	if lsErr != nil {
		t.Fatalf("ls-remote after blocked push: %v", lsErr)
	}
	for _, line := range strings.Split(lsOut2, "\n") {
		if strings.Contains(line, "refs/heads/main") {
			fields := strings.Fields(line)
			if len(fields) >= 1 && fields[0] != seedSHA {
				t.Errorf("remote main SHA changed after blocked push: seed=%q got=%q — absence lease did not protect the remote", seedSHA, fields[0])
			}
		}
	}
}

// TestD4_RealGit_RemoteHasDevelopOnly_Passes affirms D4 uses "empty remote" (no
// branches at all) and NOT "remote without main/master". A repo whose default
// branch is "develop" is not blocked by D4.
// Rule reconciliation: affirms ADR-2026-11-26 D4 "A condição é 'remoto vazio',
// e não 'remoto sem main/master', para não afetar repositórios existentes cuja
// branch padrão tem outro nome".
func TestD4_RealGit_RemoteHasDevelopOnly_Passes(t *testing.T) {
	bare := makeBareRepo(t)
	local := t.TempDir()
	// Initialize with "develop" as default branch instead of main.
	gitRepoIn(t, local,
		[]string{"init", "-q", "-b", "develop", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
		[]string{"remote", "add", "origin", bare},
	)
	if err := os.WriteFile(filepath.Join(local, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRepoIn(t, local,
		[]string{"add", "a.txt"},
		[]string{"commit", "-q", "-m", "init"},
		[]string{"push", "origin", "develop"},
		[]string{"checkout", "-q", "-b", "feat/my-feature"},
	)
	t.Chdir(local)

	deps := pushDeps{
		execGit:         defaultGitExec,
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             &bytes.Buffer{},
	}
	err := runPush(pushOpts{dryRun: true}, deps)
	if err != nil {
		t.Fatalf("expected D4 to pass when remote has 'develop' (not main/master), got: %v", err)
	}
}
