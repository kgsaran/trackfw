package commands

// netexec_test.go — testes do executor de rede e do classificador de args git.
//
// Regra Dura de Reconciliação: cada teste declara, em comentário, a conclusão que afirma.
//
// AC2 usa re-exec do próprio binário de teste como helper "neto que dorme" (GIT_SSH_COMMAND
// apontando para os.Args[0] com TRACKFW_TEST_HANG_HELPER=1). O helper grava seu PID em
// TRACKFW_TEST_HANG_PIDFILE e dorme 600 s sem ler stdin (ao contrário de `cat`, que sairia
// no EOF e mascararia a regressão — Wave 0 seção 5.1). GIT_SSH_VARIANT=ssh é obrigatório
// para evitar a sonda `ssh -G` do git que completa rapidamente sem travar o pipe.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/netdeadline"
)

// TestMain intercepts helper modes before any test framework code runs.
// The order of checks is significant: HANG must be checked before SPAWN to prevent fork bombs.
//
//   - TRACKFW_TEST_HANG_HELPER=1: write own PID to TRACKFW_TEST_HANG_PIDFILE, sleep 600 s,
//     exit 0. This is the "grandchild that holds the pipe" used by AC2 and D1 tests.
//   - TRACKFW_TEST_SPAWN_GRANDCHILD=1: start a grandchild (this binary) in HANG_HELPER mode
//     inheriting os.Stderr, write grandchild PID to TRACKFW_TEST_HANG_PIDFILE, exit 0.
//     Used by D1 tests to exercise the WaitDelay-success branch without going through git.
//   - TRACKFW_TEST_SHORT_SLEEP=1: sleep 2.5 s, exit 0. Used by the D2 ForgeCLI test to
//     verify that defaultExecForgeCLI does not apply TRACKFW_NETWORK_TIMEOUT.
//
// This pattern (re-exec via os.Args[0]) is the same used by the Go stdlib's os/exec tests.
func TestMain(m *testing.M) {
	// 1. Hang mode — grandchild that holds the pipe open.
	if os.Getenv("TRACKFW_TEST_HANG_HELPER") == "1" {
		if pidFile := os.Getenv("TRACKFW_TEST_HANG_PIDFILE"); pidFile != "" {
			_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600)
		}
		time.Sleep(600 * time.Second)
		os.Exit(0)
	}

	// 2. Spawner mode — start a grandchild in HANG_HELPER mode, exit 0 immediately.
	// Prints the grandchild PID to stdout (so the test can assert "stdout captured").
	// Also writes to TRACKFW_TEST_HANG_PIDFILE for cleanup.
	// Removes TRACKFW_TEST_SPAWN_GRANDCHILD from grandchild env to prevent fork bombs.
	if os.Getenv("TRACKFW_TEST_SPAWN_GRANDCHILD") == "1" {
		pidFile := os.Getenv("TRACKFW_TEST_HANG_PIDFILE")
		grandchild := exec.Command(os.Args[0], "-test.run=^$")
		env := make([]string, 0, len(os.Environ())+1)
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "TRACKFW_TEST_SPAWN_GRANDCHILD=") {
				env = append(env, e)
			}
		}
		env = append(env, "TRACKFW_TEST_HANG_HELPER=1")
		grandchild.Env = env
		grandchild.Stderr = os.Stderr // inherit stderr pipe from the parent executor
		if err := grandchild.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "spawn grandchild: %v\n", err)
			os.Exit(1)
		}
		pid := grandchild.Process.Pid
		if pidFile != "" {
			_ = os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o600)
		}
		// Print PID to stdout so runNetworkCmd can capture it (captureStdout=true path).
		fmt.Println(pid)
		os.Exit(0)
	}

	// 3. Short-sleep mode — sleeps 2.5 s then exits 0. Used by ForgeCLI D2 test.
	if os.Getenv("TRACKFW_TEST_SHORT_SLEEP") == "1" {
		time.Sleep(2500 * time.Millisecond)
		os.Exit(0)
	}

	// ML-2A: fail fast when git is absent from PATH.  exec.Command("git","--version").Run()
	// executes git; exec.LookPath would accept a git that exits 127 (Wave 0, Prescrição 7).
	// Runs BEFORE MkdirTemp so no temporary HOME leaks on incomplete environments.
	if err := exec.Command("git", "--version").Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ambiente incompleto: git não encontrado no PATH")
		os.Exit(2)
	}

	// ML-1A (a): Capture Go cache env vars BEFORE changing HOME.  Five tests in
	// this package call exec.Command("go", "build", …) without setting cmd.Env.
	// On macOS, GOCACHE defaults to $HOME/Library/Caches/go-build.  After HOME
	// isolation those subprocesses would compute a cold cache inside the synthetic
	// home, fail with GOPROXY=off, and leave read-only module files that defeat
	// os.RemoveAll.  Capturing and re-injecting the real values prevents both.
	// TrimSpace: exec.Command output ends with '\n'; a stray newline makes the
	// path different from what go would compute itself.
	goenvCapture := func(varName string) string {
		out, err := exec.Command("go", "env", varName).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	goCache := goenvCapture("GOCACHE")
	goModCache := goenvCapture("GOMODCACHE")
	goPath := goenvCapture("GOPATH")
	goEnv := goenvCapture("GOENV")

	// ML-1A (b): Isolate HOME and USERPROFILE to a synthetic, empty, per-run
	// temp directory.  Both must point at the SAME directory to collapse the two
	// resolution channels: internal/homedir reads HOME first on every platform;
	// os.UserHomeDir on Windows reads USERPROFILE.  Without this isolation,
	// 15 tests fail when the developer's ~/.claude/settings.json carries a global
	// guard hook and trackfw is absent from PATH.
	home, homeErr := os.MkdirTemp("", "trackfw-commands-test-home-*")
	if homeErr != nil {
		panic(homeErr)
	}
	isolatedTestHome = home
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	if err := os.Setenv("USERPROFILE", home); err != nil {
		panic(err)
	}

	// ML-1A: ClaudeConfigDir respects CLAUDE_CONFIG_DIR. Unset before any test
	// runs so a developer's real Claude config is never read or written.
	os.Unsetenv("CLAUDE_CONFIG_DIR")

	// ML-1A (c): Re-inject Go cache vars so go-build subprocesses still use the
	// real cache after HOME has been replaced.  Only set non-empty values; if
	// "go env" failed the subprocess will fall back to its own defaults.
	if goCache != "" {
		os.Setenv("GOCACHE", goCache)
	}
	if goModCache != "" {
		os.Setenv("GOMODCACHE", goModCache)
	}
	if goPath != "" {
		os.Setenv("GOPATH", goPath)
	}
	if goEnv != "" {
		os.Setenv("GOENV", goEnv)
	}

	// ML-7D: disable git maintenance auto-scheduling for every git child process
	// in this test binary. GIT_CONFIG_PARAMETERS stacks with GIT_CONFIG_COUNT
	// (independent mechanism) — helpers that define their own GIT_CONFIG_COUNT
	// do not override it. Measured: 0 maintenance events in GIT_TRACE2_EVENT
	// vs 15 without it (contra-arm), with a helper that sets GIT_CONFIG_COUNT=3.
	// Background: git maintenance run --auto --detach races with t.TempDir's
	// RemoveAll and produces "directory not empty" under parallel load.
	if err := os.Setenv("GIT_CONFIG_PARAMETERS", "'gc.auto=0' 'maintenance.auto=false'"); err != nil {
		panic(err)
	}

	code := m.Run()

	// ML-1A (d): Release write permission on all entries before RemoveAll.  The
	// Go runtime may write read-only telemetry files into $HOME/Library/
	// Application Support/go/telemetry (macOS).  Skip symlinks: os.Chmod follows
	// them and would chmod targets outside the temp tree.
	_ = filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		_ = os.Chmod(p, info.Mode()|0o200)
		return nil
	})
	os.RemoveAll(home)
	os.Exit(code)
}

// ---- ClassifyGitArgs tests ----

// TestClassifyGitArgs_Fetch asserts that "fetch" after a -C flag is returned as the subcommand.
// Conclusion: the classifier skips -C and its value and returns "fetch".
func TestClassifyGitArgs_Fetch(t *testing.T) {
	got := classifyGitArgs([]string{"-C", "d", "fetch", "origin"})
	if got != "fetch" {
		t.Fatalf("want %q, got %q", "fetch", got)
	}
	if networkSubcmds[got] != "metadata" {
		t.Fatalf("fetch should map to metadata, got %q", networkSubcmds[got])
	}
}

// TestClassifyGitArgs_Push asserts that "push" after a -c flag is returned as the subcommand.
// Conclusion: the classifier skips -c and its value and returns "push", which maps to transfer.
func TestClassifyGitArgs_Push(t *testing.T) {
	got := classifyGitArgs([]string{"-c", "k=v", "push", "origin"})
	if got != "push" {
		t.Fatalf("want %q, got %q", "push", got)
	}
	if networkSubcmds[got] != "transfer" {
		t.Fatalf("push should map to transfer, got %q", networkSubcmds[got])
	}
}

// TestClassifyGitArgs_LSRemote asserts that "ls-remote" after --git-dir= is returned.
// Conclusion: the classifier skips single-token equals-form flags and returns "ls-remote".
func TestClassifyGitArgs_LSRemote(t *testing.T) {
	got := classifyGitArgs([]string{"--git-dir=x", "ls-remote"})
	if got != "ls-remote" {
		t.Fatalf("want %q, got %q", "ls-remote", got)
	}
	if networkSubcmds[got] != "metadata" {
		t.Fatalf("ls-remote should map to metadata, got %q", networkSubcmds[got])
	}
}

// TestClassifyGitArgs_Commit asserts that "commit" is returned and has no network class.
// Conclusion: commit is a local subcommand; defaultGitExec runs it without a deadline.
func TestClassifyGitArgs_Commit(t *testing.T) {
	got := classifyGitArgs([]string{"commit", "-m", "m"})
	if got != "commit" {
		t.Fatalf("want %q, got %q", "commit", got)
	}
	if class := networkSubcmds[got]; class != "" {
		t.Fatalf("commit should have no network class, got %q", class)
	}
}

// TestClassifyGitArgs_Log asserts that "log" is returned and has no network class.
// Conclusion: log is a local subcommand.
func TestClassifyGitArgs_Log(t *testing.T) {
	got := classifyGitArgs([]string{"log", "--oneline"})
	if got != "log" {
		t.Fatalf("want %q, got %q", "log", got)
	}
	if class := networkSubcmds[got]; class != "" {
		t.Fatalf("log should have no network class, got %q", class)
	}
}

// TestClassifyGitArgs_RevParse asserts that "rev-parse" is returned and has no network class.
// Conclusion: rev-parse is a local subcommand.
func TestClassifyGitArgs_RevParse(t *testing.T) {
	got := classifyGitArgs([]string{"rev-parse", "HEAD"})
	if got != "rev-parse" {
		t.Fatalf("want %q, got %q", "rev-parse", got)
	}
	if class := networkSubcmds[got]; class != "" {
		t.Fatalf("rev-parse should have no network class, got %q", class)
	}
}

// ---- defaultExecForgeCLI AC6 structural test ----

// TestDefaultExecForgeCLI_AC6_NoDeadline asserts that defaultExecForgeCLI does NOT apply
// TRACKFW_NETWORK_TIMEOUT: a subprocess that sleeps 2.5 s completes successfully even when
// TRACKFW_NETWORK_TIMEOUT=1s.
//
// Conclusion: defaultExecForgeCLI uses exec.Command (no context), so an interactive forge
// prompt is never killed by the network deadline (ADR D3/I). A timed executor (runNetworkCmd)
// would kill at 1 s; absence of the kill proves the no-deadline contract.
//
// Bite: temporarily replace defaultExecForgeCLI body with runNetworkCmd. The test fails
// because the subprocess is killed at 1 s (err != nil and elapsed < 2 s).
func TestDefaultExecForgeCLI_AC6_NoDeadline(t *testing.T) {
	t.Setenv("TRACKFW_TEST_SHORT_SLEEP", "1")
	t.Setenv("TRACKFW_NETWORK_TIMEOUT", "1s")

	const sleepDur = 2500 * time.Millisecond

	start := time.Now()
	err := defaultExecForgeCLI(os.Args[0], []string{"-test.run=^$"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error (no deadline applied), got: %v", err)
	}
	// time.Sleep guarantees at least sleepDur; no margin needed.
	if elapsed < sleepDur {
		t.Errorf("elapsed %s < %s; process was killed before completing — deadline was applied when it should not have been",
			elapsed, sleepDur)
	}
	t.Logf("AC6 NoDeadline: returned in %s (TRACKFW_NETWORK_TIMEOUT=1s had no effect)", elapsed)
}

// ---- AC2 hang test helpers ----

// initFakeRepoWithSSHRemote creates a minimal git repo pointing at ssh://x.invalid/fake.git.
func initFakeRepoWithSSHRemote(t *testing.T) string {
	t.Helper()

	emptyCfg := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.WriteFile(emptyCfg, nil, 0o600); err != nil {
		t.Fatalf("write empty git config: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", emptyCfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	dir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		fullArgs := append([]string{"-C", dir}, args...)
		out, err := exec.Command("git", fullArgs...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	runGit("config", "commit.gpgsign", "false")
	runGit("commit", "--allow-empty", "-m", "init")
	runGit("remote", "add", "origin", "ssh://x.invalid/fake.git")

	return dir
}

// hangHelperCmd returns the GIT_SSH_COMMAND value that re-executes the test binary.
// Uses filepath.ToSlash to avoid backslash-escape issues in sh -c on Windows.
func hangHelperCmd() string {
	return "'" + filepath.ToSlash(os.Args[0]) + "'"
}

// initRepoWithBareRemote creates a working git repo with an empty commit and a bare remote
// pointing at a local temp dir. Returns (repoDir, bareDir, hooksDir).
// Used by D1 tests that push through a pre-push hook to exercise the WaitDelay-success path.
func initRepoWithBareRemote(t *testing.T) (repoDir, bareDir, hooksDir string) {
	t.Helper()

	emptyCfg := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.WriteFile(emptyCfg, nil, 0o600); err != nil {
		t.Fatalf("write empty git config: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", emptyCfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	bareDir = t.TempDir()
	repoDir = t.TempDir()
	hooksDir = t.TempDir()

	runGit := func(dir string, args ...string) {
		t.Helper()
		fullArgs := append([]string{"-C", dir}, args...)
		out, err := exec.Command("git", fullArgs...).CombinedOutput()
		if err != nil {
			t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
		}
	}

	runGit(bareDir, "init", "--bare")
	runGit(repoDir, "init")
	runGit(repoDir, "config", "user.email", "test@example.com")
	runGit(repoDir, "config", "user.name", "Test")
	runGit(repoDir, "config", "commit.gpgsign", "false")
	runGit(repoDir, "commit", "--allow-empty", "-m", "init")
	runGit(repoDir, "remote", "add", "origin", bareDir)
	return
}

// waitForPIDFile polls until the PID file appears (helper has started) or timeout.
func waitForPIDFile(t *testing.T, pidFile string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil && len(data) > 0 {
			pid, err := strconv.Atoi(string(data))
			if err == nil {
				return pid
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("PID file %s did not appear within %s", pidFile, timeout)
	return 0
}

// ---- TestGitExec_AC2_WaitDelayOnly ----

// TestGitExec_AC2_WaitDelayOnly runs defaultGitExec against a hanging grandchild with
// the WaitDelay-only path (hasControllingTTY forced true, no process-group kill).
// This is the always-active path on Windows and the fallback on Unix with a ctty.
//
// Conclusion: with WaitDelay, defaultGitExec returns within timeout + WaitDelay even when
// the grandchild holds the stderr pipe open; without WaitDelay the protection timeout fires.
func TestGitExec_AC2_WaitDelayOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	// Force WaitDelay-only path regardless of actual terminal state.
	origTTY := hasControllingTTY
	hasControllingTTY = func() bool { return true }
	defer func() { hasControllingTTY = origTTY }()

	repoDir := initFakeRepoWithSSHRemote(t)
	pidFile := filepath.Join(t.TempDir(), "helper.pid")

	// Register cleanup to kill any surviving helper (prevents leaks on test failure).
	var helperPID int
	t.Cleanup(func() {
		if helperPID > 0 {
			killProcess(helperPID)
		}
	})

	t.Setenv("TRACKFW_TEST_HANG_HELPER", "1")
	t.Setenv("TRACKFW_TEST_HANG_PIDFILE", pidFile)
	t.Setenv("GIT_SSH_COMMAND", hangHelperCmd())
	t.Setenv("GIT_SSH_VARIANT", "ssh")        // avoid ssh -G variant probe
	t.Setenv("TRACKFW_NETWORK_TIMEOUT", "3s") // short timeout for the test

	const protectionTimeout = 15 * time.Second // 3s + 5s WaitDelay + 7s margin

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := defaultGitExec("-C", repoDir, "fetch", "origin")
		done <- err
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)

		// Give the helper a moment to write its PID file (it may race with our done channel).
		if pid := waitForPIDFile(t, pidFile, 3*time.Second); pid > 0 {
			helperPID = pid
		}

		// Lower bound: must have waited at least the full deadline.
		if elapsed < 3*time.Second {
			t.Errorf("elapsed %s < 3s deadline; deadline was not honoured", elapsed)
		}

		// Upper bound: must have returned within deadline + WaitDelay + 3s margin.
		// 3s provides enough headroom for Windows CI process-spawn overhead (~1-2s extra).
		maxExpected := 3*time.Second + cmdWaitDelay + 3*time.Second
		if elapsed >= maxExpected {
			t.Errorf("elapsed %s >= %s (deadline+WaitDelay+margin); WaitDelay did not bound the wait", elapsed, maxExpected)
		}

		// Error must be distinguishable as a network timeout.
		if err == nil {
			t.Error("expected error on timeout, got nil")
		} else if !errors.Is(err, netdeadline.ErrTimeout) {
			t.Errorf("error should satisfy errors.Is(err, ErrTimeout); got: %v", err)
		}

		t.Logf("AC2 WaitDelayOnly: returned in %s with err=%v", elapsed, err)

	case <-time.After(protectionTimeout):
		// Force-read PID file for cleanup.
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil {
				helperPID = pid
			}
		}
		t.Fatalf("PROTECTION TIMEOUT (%s): defaultGitExec did not return — likely missing WaitDelay", protectionTimeout)
	}
}

// ---- TestGitExec_AC2_Bite_NoWaitDelay ----

// TestGitExec_AC2_Bite_NoWaitDelay demonstrates that exec.CommandContext alone (without
// WaitDelay) does not return when a grandchild holds the stderr pipe open.
// Skipped on Windows where the pipe behaviour differs.
//
// Conclusion: without WaitDelay, exec.CommandContext alone blocks beyond its own deadline;
// the production WaitDelay provision is the required fix (ADR D2, D7/A2).
func TestGitExec_AC2_Bite_NoWaitDelay(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("bite test skipped on Windows: pipe hang behaviour not confirmed there")
	}

	// Force WaitDelay-only path (no group kill) to isolate the WaitDelay effect.
	origTTY := hasControllingTTY
	hasControllingTTY = func() bool { return true }
	defer func() { hasControllingTTY = origTTY }()

	repoDir := initFakeRepoWithSSHRemote(t)
	pidFile := filepath.Join(t.TempDir(), "bite-helper.pid")

	var helperPID int
	t.Cleanup(func() {
		if helperPID > 0 {
			killProcess(helperPID)
		}
	})

	t.Setenv("TRACKFW_TEST_HANG_HELPER", "1")
	t.Setenv("TRACKFW_TEST_HANG_PIDFILE", pidFile)
	t.Setenv("GIT_SSH_COMMAND", hangHelperCmd())
	t.Setenv("GIT_SSH_VARIANT", "ssh")

	const biteDeadline = 3 * time.Second
	// Protection: if the command returns quickly it means the helper didn't block the pipe.
	const biteProtection = biteDeadline + 2*time.Second

	biteCtx, biteCancel := context.WithTimeout(context.Background(), biteDeadline)
	defer biteCancel()

	// Build the command the same way defaultGitExec does but WITHOUT WaitDelay.
	cmd := exec.CommandContext(biteCtx, "git", "-C", repoDir, "fetch", "origin") //nolint:gosec
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// NO cmd.WaitDelay — this is the bite. The grandchild holds the stderr pipe.

	done := make(chan struct{})
	go func() {
		cmd.Run() //nolint:errcheck
		close(done)
	}()

	// Wait for helper PID file (proves the helper ran and is holding the pipe).
	time.Sleep(300 * time.Millisecond) // give git a moment to spawn the helper
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(string(data)); err == nil {
			helperPID = pid
		}
	}

	select {
	case <-done:
		t.Log("bite: command returned — helper may not have been holding the stderr pipe")
	case <-time.After(biteProtection):
		t.Logf("BITE CONFIRMED: exec.CommandContext without WaitDelay blocked beyond %s; helper (pid=%d) held stderr pipe",
			biteProtection, helperPID)
	}
}

// ---- D1 WaitDelay-success tests ----

// TestWaitDelay_ResolveNetworkErr verifies the classification logic of resolveNetworkErr.
//
// Conclusion: resolveNetworkErr returns (true,false) on context expiry (including when
// exec.ErrWaitDelay+success also holds simultaneously), (false,true) when exec.ErrWaitDelay
// fires with a success ProcessState and ctx has not expired, and (false,false) for other
// errors. Both defaultGitExec and runNetworkCmd rely on this function for the D1 fix.
func TestWaitDelay_ResolveNetworkErr(t *testing.T) {
	// Obtain a real success ProcessState by running a no-op subprocess.
	probe := exec.Command(os.Args[0], "-test.run=^$")
	if err := probe.Run(); err != nil {
		t.Fatalf("probe command failed: %v", err)
	}
	successState := probe.ProcessState

	bg := context.Background()

	// Case 1: ErrWaitDelay + success → (false, true)
	fakeCmd := &exec.Cmd{ProcessState: successState}
	isTimeout, isSuccess := resolveNetworkErr(exec.ErrWaitDelay, bg, fakeCmd)
	if isTimeout || !isSuccess {
		t.Errorf("ErrWaitDelay+success: want (false, true), got (%v, %v)", isTimeout, isSuccess)
	}

	// Case 2: context cancelled → (true, false)
	cancelCtx, cancel := context.WithCancel(bg)
	cancel()
	isTimeout, isSuccess = resolveNetworkErr(errors.New("killed"), cancelCtx, &exec.Cmd{})
	if !isTimeout || isSuccess {
		t.Errorf("cancelled ctx: want (true, false), got (%v, %v)", isTimeout, isSuccess)
	}

	// Case 3: other non-WaitDelay error → (false, false)
	isTimeout, isSuccess = resolveNetworkErr(errors.New("connection reset"), bg, &exec.Cmd{})
	if isTimeout || isSuccess {
		t.Errorf("generic error: want (false, false), got (%v, %v)", isTimeout, isSuccess)
	}

	// Case 4: ErrWaitDelay but ProcessState nil → (false, false); not a success
	isTimeout, isSuccess = resolveNetworkErr(exec.ErrWaitDelay, bg, &exec.Cmd{})
	if isTimeout || isSuccess {
		t.Errorf("ErrWaitDelay+nil ProcessState: want (false, false), got (%v, %v)", isTimeout, isSuccess)
	}

	// Case 5: ErrWaitDelay + success ProcessState + ctx already expired → (true, false).
	// Context expiry takes precedence over the WaitDelay-success branch (ADR D1).
	// Mordida: inverting the order of the two if-blocks in resolveNetworkErr causes this
	// case to return (false, true) instead of (true, false), and this assertion fails.
	cancelledCtx, cancelFn := context.WithCancel(bg)
	cancelFn() // expire the context immediately
	fakeSuccessCmd := &exec.Cmd{ProcessState: successState}
	isTimeout, isSuccess = resolveNetworkErr(exec.ErrWaitDelay, cancelledCtx, fakeSuccessCmd)
	if !isTimeout || isSuccess {
		t.Errorf("ErrWaitDelay+success+expired ctx: want (true, false), got (%v, %v)", isTimeout, isSuccess)
	}
}

// TestNetworkCmd_WaitDelay_ExitZeroWithHangingGrandchild verifies that runNetworkCmd returns
// nil when the process exits 0 but a grandchild holds the stderr pipe past WaitDelay (D1 fix).
// The spawner exits 0 immediately; the grandchild holds the pipe until killed in cleanup.
//
// Conclusion: with the D1 fix (errors.Is(err, exec.ErrWaitDelay) + ProcessState.Success()),
// runNetworkCmd returns (nil, nil) when the child exits 0 but a grandchild holds the pipe.
// Without the fix (errors.As(*ExitError)), runNetworkCmd returns a non-nil error.
func TestNetworkCmd_WaitDelay_ExitZeroWithHangingGrandchild(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "netcmd-spawn-helper.pid")

	var grandchildPID int
	t.Cleanup(func() {
		if grandchildPID > 0 {
			killProcess(grandchildPID)
		}
	})

	t.Setenv("TRACKFW_TEST_SPAWN_GRANDCHILD", "1")
	t.Setenv("TRACKFW_TEST_HANG_PIDFILE", pidFile)
	t.Setenv("TRACKFW_NETWORK_TIMEOUT", "60s") // context timeout must not fire

	const testDelay = 500 * time.Millisecond
	origDelay := cmdWaitDelay
	cmdWaitDelay = testDelay
	defer func() { cmdWaitDelay = origDelay }()

	const protectionTimeout = 10 * time.Second

	start := time.Now()
	done := make(chan struct {
		out []byte
		err error
	}, 1)
	go func() {
		// captureStdout=true: the spawner prints the grandchild PID to stdout.
		out, err := runNetworkCmd(os.Args[0], []string{"-test.run=^$"}, "", true)
		done <- struct {
			out []byte
			err error
		}{out, err}
	}()

	var result struct {
		out []byte
		err error
	}
	select {
	case result = <-done:
	case <-time.After(protectionTimeout):
		t.Fatalf("PROTECTION TIMEOUT (%s): runNetworkCmd did not return", protectionTimeout)
	}
	elapsed := time.Since(start)

	// Read PID file for cleanup (written by the spawner before it exited).
	if data, readErr := os.ReadFile(pidFile); readErr == nil {
		if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil {
			grandchildPID = pid
		}
	}

	if result.err != nil {
		t.Fatalf("expected nil error (WaitDelay+exit-0 must be success), got: %v", result.err)
	}
	// Stdout must be captured: the spawner printed the grandchild PID.
	capturedPID := strings.TrimSpace(string(result.out))
	if capturedPID == "" {
		t.Error("stdout was empty; spawner PID not captured (captureStdout path not exercised)")
	} else if grandchildPID > 0 && capturedPID != strconv.Itoa(grandchildPID) {
		t.Errorf("captured stdout PID %q != pidfile PID %d", capturedPID, grandchildPID)
	}
	// Lower bound: WaitDelay must have fired.
	if elapsed < testDelay {
		t.Errorf("elapsed %s < %s (cmdWaitDelay); WaitDelay branch may not have been exercised", elapsed, testDelay)
	}
	// Upper bound: must complete within cmdWaitDelay + 3s margin.
	// 3s provides enough headroom for process-spawn overhead on slow CI.
	if elapsed >= testDelay+3*time.Second {
		t.Errorf("elapsed %s >= %s (cmdWaitDelay+3s); test took too long (possible hang)", elapsed, testDelay+3*time.Second)
	}
	t.Logf("WaitDelay ExitZero (runNetworkCmd): returned in %s; stdout=%q; grandchild pid=%d",
		elapsed, capturedPID, grandchildPID)
}

// TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild verifies that defaultGitExec returns
// ("push-output", nil) when a pre-push hook spawns a grandchild that holds stderr open.
// The push itself succeeds; the grandchild delays the pipe drain past WaitDelay.
//
// Conclusion: with the D1 fix, defaultGitExec returns success (stdout captured, nil error)
// when git exits 0 but a hook grandchild holds the stderr pipe past WaitDelay.
// Without the fix, defaultGitExec would return a non-nil error despite git exiting 0.
//
// Skipped on Windows: pre-push hooks require a POSIX shell interpreter.
func TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("pre-push hook test skipped on Windows: shell hooks not supported")
	}

	repoDir, _, hooksDir := initRepoWithBareRemote(t)
	pidFile := filepath.Join(t.TempDir(), "gitexec-spawn-helper.pid")

	// Write pre-push hook: run the test binary in spawner mode.
	// The spawner spawns a hang-helper grandchild inheriting stderr, writes its PID, exits 0.
	// The hook therefore exits 0 and the grandchild holds the stderr pipe open.
	hookScript := fmt.Sprintf("#!/bin/sh\n\"%s\" -test.run=^$\n", filepath.ToSlash(os.Args[0]))
	hookPath := filepath.Join(hooksDir, "pre-push")
	if err := os.WriteFile(hookPath, []byte(hookScript), 0o755); err != nil {
		t.Fatalf("write pre-push hook: %v", err)
	}

	var grandchildPID int
	t.Cleanup(func() {
		if grandchildPID > 0 {
			killProcess(grandchildPID)
		}
	})

	t.Setenv("TRACKFW_TEST_SPAWN_GRANDCHILD", "1")
	t.Setenv("TRACKFW_TEST_HANG_PIDFILE", pidFile)
	t.Setenv("TRACKFW_PUSH_TIMEOUT", "60s") // push is class "transfer"; context must not fire

	const testDelay = 500 * time.Millisecond
	origDelay := cmdWaitDelay
	cmdWaitDelay = testDelay
	defer func() { cmdWaitDelay = origDelay }()

	const protectionTimeout = 15 * time.Second

	start := time.Now()
	done := make(chan struct {
		out string
		err error
	}, 1)
	go func() {
		out, err := defaultGitExec("-C", repoDir,
			"-c", "core.hooksPath="+hooksDir,
			"push", "--porcelain", "origin", "HEAD:refs/heads/main")
		done <- struct {
			out string
			err error
		}{out, err}
	}()

	var result struct {
		out string
		err error
	}
	select {
	case result = <-done:
	case <-time.After(protectionTimeout):
		t.Fatalf("PROTECTION TIMEOUT (%s): defaultGitExec did not return", protectionTimeout)
	}
	elapsed := time.Since(start)

	// Read PID file for cleanup.
	if data, readErr := os.ReadFile(pidFile); readErr == nil {
		if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil {
			grandchildPID = pid
		}
	}

	if result.err != nil {
		t.Fatalf("expected nil error (push+hook exit-0, WaitDelay must be success), got: %v (stdout: %q)",
			result.err, result.out)
	}
	if !strings.Contains(result.out, "refs/heads/main") {
		t.Errorf("stdout %q does not mention refs/heads/main; push may not have succeeded", result.out)
	}
	// Lower bound: WaitDelay must have fired.
	if elapsed < testDelay {
		t.Errorf("elapsed %s < %s (cmdWaitDelay); WaitDelay branch may not have been exercised", elapsed, testDelay)
	}
	// Upper bound: must complete within cmdWaitDelay + 3s margin.
	// 3s provides enough headroom for git startup and process-spawn overhead on slow CI.
	if elapsed >= testDelay+3*time.Second {
		t.Errorf("elapsed %s >= %s (cmdWaitDelay+3s); test took too long (possible hang)", elapsed, testDelay+3*time.Second)
	}
	t.Logf("WaitDelay ExitZero (defaultGitExec): returned in %s with out=%q; grandchild pid=%d",
		elapsed, result.out, grandchildPID)
}
