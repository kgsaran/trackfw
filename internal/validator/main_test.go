package validator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// isolatedTestHome is the synthetic home TestMain binds this test binary to.
// Exposed at package scope so TestIsolatedHomeCoversBothChannels can assert on it.
var isolatedTestHome string

// TestMain isolates the home directory ($HOME AND %USERPROFILE%, see below)
// to a synthetic, empty, per-run temp directory for
// the ENTIRE validator test binary (this covers both `package validator`
// internal tests and the `package validator_test` external tests compiled
// into the same binary — Go allows only one TestMain per binary).
//
// ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
// integridade-independente-de-fiacao, ML-3A: validateGuardGlobalScriptIntegrity
// now triggers on the on-disk EXISTENCE of ~/.trackfw/scripts/<guard>.sh,
// not on config wiring — so any test that calls Validate()/ValidateTagged()
// without controlling $HOME picks up whatever this rule finds under the
// REAL developer's $HOME. Measured: TestValidate_Clean failed on a machine
// with a stale global git-branch-guard script (the exact drift the REQ that
// motivated this ML is about), because os.UserHomeDir() resolved the real
// $HOME and found a real divergence — nothing to do with the fixture under
// test. Same failure mode as the Cenário 46 precedent
// (scripts/check-gates-falsify.sh), generalized here to every test in this
// package instead of patched test-by-test.
//
// A test that specifically exercises global-scope guard behavior (see
// globalGuardHome in validator_git_branch_guard_test.go) still calls
// t.Setenv("HOME", ...) itself — t.Setenv scopes the override to that one
// test and restores the TestMain value set here afterward, so this does not
// conflict with or weaken those tests ON POSIX.
//
// 🔴 On Windows that per-test override is only half an override, and fixing it
// belongs in those tests, not here: it moves HOME (which production reads via
// internal/homedir) but leaves %USERPROFILE% pointing at THIS session home, so
// anything resolving through os.UserHomeDir() during that test sees the session
// home instead of the test's own. Strictly better than before (the leak lands in
// a disposable directory rather than the runner's real profile), but not correct
// per-test isolation.
//
// On Windows, isolating HOME alone is not enough — and the reason is NOT
// "production ignores HOME there". Since 2026-09-01 production resolves the
// home through internal/homedir (homedir.go:32), which reads os.Getenv("HOME")
// FIRST on every platform. The defect is CHANNEL DIVERGENCE inside one process:
// production reads HOME through the shim, while anything computing its
// expectation from the platform primitive (os.UserHomeDir(), which on Windows
// returns %USERPROFILE% and never consults HOME) sees a DIFFERENT home. Two
// homes, one process — measured on the Windows job (run 33742756936, ITEM 2:
// with HOME and USERPROFILE deliberately set to different directories, Go, Node
// and Python primitives all resolved to %USERPROFILE%). Pointing BOTH variables
// at the SAME synthetic directory collapses the two channels; it cannot
// over-isolate, because there is no third channel production legitimately
// reads. On POSIX %USERPROFILE% is inert (neither the stdlib nor trackfw reads
// it), so the extra variable is a no-op there.
func TestMain(m *testing.M) {
	// ML-2A: fail fast when git is absent from PATH.  exec.Command("git","--version").Run()
	// executes git; exec.LookPath would accept a git that exits 127 (Wave 0, Prescrição 7).
	// The guard runs BEFORE MkdirTemp so no temporary HOME leaks on incomplete environments.
	if err := exec.Command("git", "--version").Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ambiente incompleto: git não encontrado no PATH")
		os.Exit(2)
	}

	home, err := os.MkdirTemp("", "trackfw-validator-test-home-*")
	if err != nil {
		panic(err)
	}
	isolatedTestHome = home

	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}

	if err := os.Setenv("USERPROFILE", home); err != nil {
		panic(err)
	}

	// ML-1A (REQ-2026-09-01): ensure CLAUDE_CONFIG_DIR does not leak from the developer's
	// environment into the test binary. agentconfig.ClaudeConfigDir honours this env var (A1);
	// without the unset, a set CLAUDE_CONFIG_DIR would make global-guard tests read/write a real
	// user directory instead of the isolated temp home.
	if err := os.Unsetenv("CLAUDE_CONFIG_DIR"); err != nil {
		panic(err)
	}

	// ML-7D: disable git maintenance auto-scheduling for every git child process
	// in this test binary. GIT_CONFIG_PARAMETERS stacks with GIT_CONFIG_COUNT
	// (independent mechanism) and is not stripped by cleanGitEnv() (which strips
	// GIT_* prefix — but this applies to the production gitCommand/gitCommandCtx
	// helpers, not to test helpers that inherit the process env directly).
	// Measured: 0 maintenance events in GIT_TRACE2_EVENT vs 15 without it, with
	// a helper that sets GIT_CONFIG_COUNT=3.
	if err := os.Setenv("GIT_CONFIG_PARAMETERS", "'gc.auto=0' 'maintenance.auto=false'"); err != nil {
		panic(err)
	}

	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// TestGitGuardFailsFastWhenGitAbsent re-executes this test binary with a PATH
// that has no git and asserts exit code 2, the diagnostic line in stderr, and
// no PASS in stdout.  The second arm (POSIX only) uses a fake git that exits 127,
// proving the guard executes git rather than just locating it (Wave 0, Prescrição 10).
//
// Conclusion (ML-2A): with git absent from PATH, the validator package exits 2
// with exactly the diagnostic line "ambiente incompleto: git não encontrado no PATH"
// and runs no test — which is why exec.Command("git","--version").Run() is used
// instead of exec.LookPath (a fake git that exits 127 passes LookPath but fails Run).
//
// Conclusion (D1 / path_vazio): with the child's TMPDIR/TMP/TEMP redirected to a
// dedicated t.TempDir(), finding that directory empty after exit proves the git guard
// ran before MkdirTemp — no temporary HOME leaked in an incomplete environment.
func TestGitGuardFailsFastWhenGitAbsent(t *testing.T) {
	t.Run("path_vazio", func(t *testing.T) {
		emptyDir := t.TempDir()
		childTmp := t.TempDir()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
		env := gitGuardChildEnv(emptyDir, nil)
		env = append(env, "TMPDIR="+childTmp, "TMP="+childTmp, "TEMP="+childTmp)
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		_ = cmd.Run()
		assertGitGuardExit(t, cmd, stdout.String(), stderr.String())
		// D1: prove the git guard ran before MkdirTemp — no temporary HOME leaked.
		entries, err := os.ReadDir(childTmp)
		if err != nil {
			t.Fatalf("ReadDir childTmp: %v", err)
		}
		if len(entries) > 0 {
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}
			t.Errorf("guard after MkdirTemp leak: %v dirs in childTmp", names)
		}
	})

	if runtime.GOOS != "windows" {
		t.Run("git_falso_127", func(t *testing.T) {
			fakeDir := t.TempDir()
			fakeGit := filepath.Join(fakeDir, "git")
			if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\nexit 127\n"), 0o755); err != nil {
				t.Fatalf("write fake git: %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
			cmd.Env = gitGuardChildEnv(fakeDir, nil)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()
			assertGitGuardExit(t, cmd, stdout.String(), stderr.String())
		})
	}
}

// gitGuardChildEnv returns the current environment with PATH replaced by pathDir.
// extraDropPrefixes lists additional env-key prefixes to exclude.
func gitGuardChildEnv(pathDir string, extraDropPrefixes []string) []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PATH=") {
			continue
		}
		drop := false
		for _, prefix := range extraDropPrefixes {
			if strings.HasPrefix(e, prefix) {
				drop = true
				break
			}
		}
		if !drop {
			env = append(env, e)
		}
	}
	env = append(env, "PATH="+pathDir)
	return env
}

// assertGitGuardExit checks that a re-executed binary hit the git guard:
// exit code 2, diagnostic line in stderr, no PASS in stdout.
func assertGitGuardExit(t *testing.T, cmd *exec.Cmd, stdout, stderr string) {
	t.Helper()
	if cmd.ProcessState == nil {
		t.Fatal("process did not start or state unavailable")
	}
	if got := cmd.ProcessState.ExitCode(); got != 2 {
		t.Errorf("exit code = %d, want 2\nstdout: %q\nstderr: %q", got, stdout, stderr)
	}
	const diagLine = "ambiente incompleto: git não encontrado no PATH"
	if !strings.Contains(stderr, diagLine) {
		t.Errorf("stderr does not contain %q\nstderr: %q", diagLine, stderr)
	}
	if strings.Contains(stdout, "PASS") {
		t.Errorf("stdout contains PASS (tests ran when they should not)\nstdout: %q", stdout)
	}
}

// TestIsolatedHomeCoversBothChannels is the loop-breaker for the isolation
// above: it fails if any of the three resolution channels stops pointing at
// the synthetic home.  Removing either os.Setenv call in TestMain makes it
// fail — verified in both directions on macOS (POSIX).
//
// Conclusion (D3): HOME, USERPROFILE, and agentconfig.ClaudeConfigDir all
// resolve inside the synthetic home — ensuring no test in this package reads
// or writes the developer's real ~/.claude/settings.json, including via the
// production config-dir channel (validator_git_branch_guard.go:210, :434,
// :494, :561, :637).  Removing os.Setenv("HOME", ...) from TestMain makes
// this test fail on all three checks; removing only os.Unsetenv("CLAUDE_CONFIG_DIR")
// while CLAUDE_CONFIG_DIR is set makes only the third check fail.
func TestIsolatedHomeCoversBothChannels(t *testing.T) {
	if isolatedTestHome == "" {
		t.Fatal("TestMain did not record the synthetic home")
	}

	// HOME: the channel internal/homedir reads first, on every platform.
	if got := os.Getenv("HOME"); got != isolatedTestHome {
		t.Errorf("HOME = %q, want the synthetic home %q", got, isolatedTestHome)
	}

	// USERPROFILE: the channel os.UserHomeDir() reads on Windows. Asserted on
	// every platform so a regression is caught by the POSIX CI too, not only by
	// the Windows job.
	if got := os.Getenv("USERPROFILE"); got != isolatedTestHome {
		t.Errorf("USERPROFILE = %q, want the synthetic home %q", got, isolatedTestHome)
	}

	// The invariant that actually matters: both channels agree, so production
	// (via the shim) and any expectation computed from the platform primitive
	// resolve to the same directory.
	if os.Getenv("HOME") != os.Getenv("USERPROFILE") {
		t.Errorf("channel divergence: HOME = %q, USERPROFILE = %q",
			os.Getenv("HOME"), os.Getenv("USERPROFILE"))
	}

	// agentconfig.ClaudeConfigDir: the production channel used by validator
	// production code (validator_git_branch_guard.go:210, :434, :494, :561, :637).
	// Must resolve inside the synthetic home — not the developer's ~/.claude.
	// A migration from homedir.Dir() to ClaudeConfigDir in production code
	// would not be caught by the two checks above; this check closes that gap
	// (D3, Wave 0 Prescrição 3 applied to the validator package).
	dir, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		t.Fatalf("agentconfig.ClaudeConfigDir returned error: %v", err)
	}
	sep := string(filepath.Separator)
	if !strings.HasPrefix(dir, isolatedTestHome+sep) {
		t.Errorf("agentconfig.ClaudeConfigDir = %q, want path rooted under synthetic home %q",
			dir, isolatedTestHome)
	}
}
