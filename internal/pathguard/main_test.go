package pathguard

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
)

// TestMain sets GIT_CONFIG_PARAMETERS once for the entire test binary so that
// every git child process runs with gc.auto=0 and maintenance.auto=false.
//
// GIT_CONFIG_PARAMETERS is independent of GIT_CONFIG_COUNT: helpers that
// define their own GIT_CONFIG_COUNT do not override it. Measured: with this
// variable set and a helper with GIT_CONFIG_COUNT=3 (no gc.auto), maintenance
// events in GIT_TRACE2_EVENT drop from 15 to 0.
//
// Background: git maintenance run --auto --detach races with t.TempDir's
// RemoveAll and produces "directory not empty" failures under parallel load.
//
// ML-2A: the git guard is the first instruction.  exec.Command("git","--version").Run()
// actually executes git; exec.LookPath would accept a git that exits 127 (Wave 0,
// Prescrição 7).  With git absent the package prints a single diagnostic line to
// stderr and exits with code 2, producing zero "--- FAIL" lines.
func TestMain(m *testing.M) {
	// ML-2A: fail fast when git is absent from PATH.
	if err := exec.Command("git", "--version").Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ambiente incompleto: git não encontrado no PATH")
		os.Exit(2)
	}

	if err := os.Setenv("GIT_CONFIG_PARAMETERS", "'gc.auto=0' 'maintenance.auto=false'"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// TestGitGuardFailsFastWhenGitAbsent re-executes this test binary with a PATH
// that has no git and asserts exit code 2, the diagnostic line in stderr, and
// no PASS in stdout.  The second arm (POSIX only) uses a fake git that exits 127,
// proving the guard executes git rather than just locating it (Wave 0, Prescrição 10).
//
// Conclusion (ML-2A): with git absent from PATH, the pathguard package exits 2
// with exactly the diagnostic line "ambiente incompleto: git não encontrado no PATH"
// and runs no test — which is why exec.Command("git","--version").Run() is used
// instead of exec.LookPath (a fake git that exits 127 passes LookPath but fails Run).
func TestGitGuardFailsFastWhenGitAbsent(t *testing.T) {
	t.Run("path_vazio", func(t *testing.T) {
		emptyDir := t.TempDir()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
		cmd.Env = gitGuardChildEnv(emptyDir, nil)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		_ = cmd.Run()
		assertGitGuardExit(t, cmd, stdout.String(), stderr.String())
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
// extraDropPrefixes lists additional env-key prefixes to exclude from the child env.
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
