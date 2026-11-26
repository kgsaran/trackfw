package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
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

	// Inject execGit: stub only ls-remote --heads to return empty (simulate the TOCTOU
	// window — the check saw an empty remote, but main was already there by push time).
	// ls-remote --get-url must go to real git so the D3-bypass guard sees no insteadOf
	// rewrite. All other calls use real git.
	stubExec := func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "ls-remote" && args[1] == "--heads" {
			return "", nil // simulate: remote appeared empty during check
		}
		return defaultGitExec(args...)
	}

	deps := pushDeps{execGit: stubExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("expected absence lease to block push when main already exists on remote; got nil error")
	}

	// A1 (ML-2C): discriminate the absence-lease rejection from non-fast-forward.
	// git emits "stale info" for lease rejection; "fetch first" / "non-fast-forward"
	// for ordinary push rejections. Only the former proves the lease is active.
	if !strings.Contains(err.Error(), "stale info") {
		t.Errorf("absence-lease rejection must contain 'stale info'; got: %v", err)
	}
	if strings.Contains(err.Error(), "fetch first") {
		t.Errorf("absence-lease rejection must not contain 'fetch first'; got: %v", err)
	}
	if strings.Contains(err.Error(), "non-fast-forward") {
		t.Errorf("absence-lease rejection must not contain 'non-fast-forward'; got: %v", err)
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

	// Counter-arm: a plain push (no absence-lease) from the same local also fails,
	// but with "non-fast-forward" (not "stale info") — a different rejection path.
	// This proves the two rejection kinds are distinguishable, validating the assertions above.
	t.Run("counter_arm_no_lease", func(t *testing.T) {
		t.Chdir(local)
		_, noLeaseErr := defaultGitExec("push", "-u", "origin", "main")
		if noLeaseErr == nil {
			t.Fatal("plain push (no lease) should also fail — local is not a descendant of remote main; got nil error")
		}
		if strings.Contains(noLeaseErr.Error(), "stale info") {
			t.Errorf("plain push error must NOT say 'stale info'; got: %v", noLeaseErr)
		}
	})
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

// ─── D3-bypass tests (ML-2C, ADR-2026-11-26) ──────────────────────────────
//
// makeBareWithDevelop creates a bare repo that has a "develop" branch (not main).
// Used as the insteadOf-redirect target to confirm the push was blocked (main absent).
func makeBareWithDevelop(t *testing.T) string {
	t.Helper()
	bare := makeBareRepo(t)
	seed := t.TempDir()
	gitRepoIn(t, seed,
		[]string{"init", "-q", "-b", "develop", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
		[]string{"remote", "add", "origin", bare},
	)
	if err := os.WriteFile(filepath.Join(seed, "s.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	gitRepoIn(t, seed,
		[]string{"add", "s.txt"},
		[]string{"commit", "-q", "-m", "seed"},
		[]string{"push", "origin", "develop"},
	)
	return bare
}

// bareHasRef reports whether the bare repo at bareDir contains refName via
// git --git-dir. Uses defaultGitExec to bypass any local insteadOf config
// that might redirect ls-remote from the calling directory.
func bareHasRef(t *testing.T, bareDir, refName string) bool {
	t.Helper()
	out, _ := defaultGitExec("--git-dir="+bareDir, "for-each-ref", refName)
	return strings.Contains(out, refName)
}

// TestD3Bypass_Forma1_PushInsteadOfPlusInsteadOf_Blocked (ML-2C) affirms
// D3-bypass: Forma 1 — pushInsteadOf + insteadOf in cascade.
//
// Setup:
//   origin.url = bareA (empty)
//   url.bareB.pushInsteadOf = bareA  → push goes to bareB (has develop)
//   url.bareC.insteadOf = bareB      → ls-remote of bareB goes to bareC (empty)
//
// The old checkRemoteEmpty called ls-remote --heads bareB, which git silently
// redirected to bareC (empty) — returning "empty remote" while the push reached
// bareB. The D3-bypass guard detects that ls-remote --get-url bareB ≠ bareB
// and fails closed.
//
// Preconditions verified before any assertion:
//   P1: get-url --push origin = bareB (pushInsteadOf applied)
//   P2: ls-remote --get-url bareB = bareC (insteadOf applied)
//
// Rule reconciliation: affirms D3-bypass guard (ML-2C) — without the guard
// the push would succeed (measured by Hades, measure_gaps3.sh). With the guard
// the push is refused and bareB receives no main branch.
func TestD3Bypass_Forma1_PushInsteadOfPlusInsteadOf_Blocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file:// URLs require Unix-style absolute paths — skipped on Windows")
	}

	bareA := makeBareRepo(t)      // origin url (empty)
	bareB := makeBareWithDevelop(t) // push destination (has develop)
	bareC := makeBareRepo(t)      // ls-remote redirect target (empty)

	urlA := "file://" + bareA
	urlB := "file://" + bareB
	urlBSlash := "file://" + bareB + "/"
	urlC := "file://" + bareC
	urlCSlash := "file://" + bareC + "/"

	local := t.TempDir()
	// Isolate HOME and XDG_CONFIG_HOME so global ~/.gitconfig cannot interfere.
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", homeDir)

	gitRepoIn(t, local,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
		[]string{"remote", "add", "origin", urlA},
		// pushInsteadOf: pushes to urlA go to urlB
		[]string{"config", "url." + urlBSlash + ".pushInsteadOf", urlA},
		// insteadOf: ls-remote of urlB goes to urlC
		[]string{"config", "url." + urlCSlash + ".insteadOf", urlBSlash},
	)
	if err := os.WriteFile(filepath.Join(local, "b.txt"), []byte("bootstrap\n"), 0o644); err != nil {
		t.Fatalf("write bootstrap.txt: %v", err)
	}
	gitRepoIn(t, local,
		[]string{"add", "b.txt"},
		[]string{"commit", "-q", "-m", "bootstrap: initial"},
	)
	t.Chdir(local)

	// Precondition P1: get-url --push origin = urlB (pushInsteadOf applied).
	p1Out, p1Err := defaultGitExec("remote", "get-url", "--push", "origin")
	if p1Err != nil {
		t.Fatalf("precondition P1 (get-url --push origin): %v", p1Err)
	}
	p1URL := strings.TrimSpace(p1Out)
	if !strings.HasPrefix(p1URL, urlB) {
		t.Fatalf("precondition P1 failed: expected push URL to start with %q, got %q — test setup broken", urlB, p1URL)
	}

	// Precondition P2: ls-remote --get-url p1URL resolves to urlC (insteadOf applied).
	p2Out, p2Err := defaultGitExec("ls-remote", "--get-url", p1URL)
	if p2Err != nil {
		t.Fatalf("precondition P2 (ls-remote --get-url %s): %v", p1URL, p2Err)
	}
	p2URL := strings.TrimSpace(p2Out)
	if !strings.HasPrefix(p2URL, urlC) {
		t.Fatalf("precondition P2 failed: expected resolved URL to start with %q, got %q — test setup broken", urlC, p2URL)
	}

	deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("D3-bypass Forma 1: expected push to be refused; got nil error")
	}
	if !strings.Contains(err.Error(), "insteadOf") {
		t.Errorf("D3-bypass Forma 1: error must cite 'insteadOf'; got: %v", err)
	}

	// bareB must NOT have refs/heads/main — the push was blocked.
	if bareHasRef(t, bareB, "refs/heads/main") {
		t.Errorf("D3-bypass Forma 1: bareB should NOT have refs/heads/main after blocked push (bare: %s)", bareB)
	}

	_ = urlBSlash // used above
	_ = urlCSlash // used above
}

// TestD3Bypass_Forma2_ChainedInsteadOf_Blocked (ML-2C) affirms
// D3-bypass: Forma 2 — chained insteadOf (no pushInsteadOf).
//
// Setup:
//   origin.url = bareA (empty)
//   url.bareX.insteadOf = bareA  → push + ls-remote of bareA go to bareX (has develop)
//   url.bareY.insteadOf = bareX  → ls-remote of bareX goes to bareY (empty)
//
// The old checkRemoteEmpty called ls-remote --heads bareX, which git redirected
// to bareY (empty). The D3-bypass guard detects ls-remote --get-url bareX ≠ bareX.
//
// Preconditions verified before any assertion:
//   P1: get-url --push origin = bareX (insteadOf applied)
//   P2: ls-remote --get-url bareX = bareY (second insteadOf applied)
//
// Rule reconciliation: affirms D3-bypass guard (ML-2C) closes the chained-
// insteadOf vector confirmed in Hades measure_gaps8.sh Section A.
func TestD3Bypass_Forma2_ChainedInsteadOf_Blocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file:// URLs require Unix-style absolute paths — skipped on Windows")
	}

	bareA := makeBareRepo(t)      // origin url (empty)
	bareX := makeBareWithDevelop(t) // first redirect target (has develop)
	bareY := makeBareRepo(t)      // second redirect target (empty)

	urlA := "file://" + bareA
	urlX := "file://" + bareX
	urlY := "file://" + bareY

	local := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", homeDir)

	gitRepoIn(t, local,
		[]string{"init", "-q", "-b", "main", "."},
		[]string{"config", "user.email", "t@localhost"},
		[]string{"config", "user.name", "T"},
		[]string{"remote", "add", "origin", urlA},
		// insteadOf: bareA → bareX (applies to push and to ls-remote)
		[]string{"config", "url." + urlX + ".insteadOf", urlA},
		// insteadOf: bareX → bareY (second level, applies to ls-remote of bareX)
		[]string{"config", "url." + urlY + ".insteadOf", urlX},
	)
	if err := os.WriteFile(filepath.Join(local, "b.txt"), []byte("bootstrap\n"), 0o644); err != nil {
		t.Fatalf("write bootstrap.txt: %v", err)
	}
	gitRepoIn(t, local,
		[]string{"add", "b.txt"},
		[]string{"commit", "-q", "-m", "bootstrap: initial"},
	)
	t.Chdir(local)

	// Precondition P1: get-url --push origin = bareX (insteadOf A→X).
	p1Out, p1Err := defaultGitExec("remote", "get-url", "--push", "origin")
	if p1Err != nil {
		t.Fatalf("precondition P1 (get-url --push origin): %v", p1Err)
	}
	p1URL := strings.TrimSpace(p1Out)
	if !strings.HasPrefix(p1URL, urlX) {
		t.Fatalf("precondition P1 failed: expected push URL to start with %q, got %q — test setup broken", urlX, p1URL)
	}

	// Precondition P2: ls-remote --get-url p1URL resolves to bareY (insteadOf X→Y).
	p2Out, p2Err := defaultGitExec("ls-remote", "--get-url", p1URL)
	if p2Err != nil {
		t.Fatalf("precondition P2 (ls-remote --get-url %s): %v", p1URL, p2Err)
	}
	p2URL := strings.TrimSpace(p2Out)
	if !strings.HasPrefix(p2URL, urlY) {
		t.Fatalf("precondition P2 failed: expected resolved URL to start with %q, got %q — test setup broken", urlY, p2URL)
	}

	deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
	err := runPush(pushOpts{}, deps)
	if err == nil {
		t.Fatal("D3-bypass Forma 2: expected push to be refused; got nil error")
	}
	if !strings.Contains(err.Error(), "insteadOf") {
		t.Errorf("D3-bypass Forma 2: error must cite 'insteadOf'; got: %v", err)
	}

	// bareX must NOT have refs/heads/main — the push was blocked.
	if bareHasRef(t, bareX, "refs/heads/main") {
		t.Errorf("D3-bypass Forma 2: bareX should NOT have refs/heads/main after blocked push (bare: %s)", bareX)
	}
}

// TestD3Bypass_CounterArm_NoInsteadOf_Passes (ML-2C) affirms the counter-arms
// for the D3-bypass tests: a fresh empty remote without insteadOf rules continues
// to pass, and an explicit pushurl without insteadOf also passes.
//
// Rule reconciliation: affirms D3-bypass guard does not regress the normal
// bootstrap path (D3-bypass guard only blocks when ls-remote --get-url returns
// a different URL, never when the URL is returned unchanged).
func TestD3Bypass_CounterArm_NoInsteadOf_Passes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file:// URLs require Unix-style absolute paths — skipped on Windows")
	}

	t.Run("plain_empty_remote", func(t *testing.T) {
		bare := makeBareRepo(t)
		local := t.TempDir()
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		t.Setenv("XDG_CONFIG_HOME", homeDir)

		gitRepoIn(t, local,
			[]string{"init", "-q", "-b", "main", "."},
			[]string{"config", "user.email", "t@localhost"},
			[]string{"config", "user.name", "T"},
			[]string{"remote", "add", "origin", "file://" + bare},
		)
		if err := os.WriteFile(filepath.Join(local, "b.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		gitRepoIn(t, local,
			[]string{"add", "b.txt"},
			[]string{"commit", "-q", "-m", "bootstrap"},
		)
		t.Chdir(local)

		deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
		if err := runPush(pushOpts{}, deps); err != nil {
			t.Errorf("D3-bypass counter-arm: plain empty remote should pass, got: %v", err)
		}
	})

	t.Run("explicit_pushurl_no_insteadof", func(t *testing.T) {
		bareFetch := makeBareRepo(t) // fetch URL (not used for push)
		barePush := makeBareRepo(t)  // explicit pushurl (empty, no insteadOf)
		local := t.TempDir()
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		t.Setenv("XDG_CONFIG_HOME", homeDir)

		gitRepoIn(t, local,
			[]string{"init", "-q", "-b", "main", "."},
			[]string{"config", "user.email", "t@localhost"},
			[]string{"config", "user.name", "T"},
			[]string{"remote", "add", "origin", "file://" + bareFetch},
			// Explicit pushurl pointing to a different empty bare (no insteadOf).
			[]string{"remote", "set-url", "--push", "origin", "file://" + barePush},
		)
		if err := os.WriteFile(filepath.Join(local, "b.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		gitRepoIn(t, local,
			[]string{"add", "b.txt"},
			[]string{"commit", "-q", "-m", "bootstrap"},
		)
		t.Chdir(local)

		deps := pushDeps{execGit: defaultGitExec, out: &bytes.Buffer{}}
		if err := runPush(pushOpts{}, deps); err != nil {
			t.Errorf("D3-bypass counter-arm: explicit pushurl without insteadOf should pass, got: %v", err)
		}
	})
}
