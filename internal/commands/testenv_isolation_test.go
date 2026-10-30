package commands

// testenv_isolation_test.go — sentinels for:
//   (a) HOME isolation installed by TestMain (netexec_test.go); and
//   (b) git guard (ML-2A): the package exits 2 when git is absent.
//
// Regra Dura de Reconciliação: each test declares, in its doc comment, the
// conclusion it asserts.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// isolatedTestHome is the synthetic home TestMain (netexec_test.go) binds this
// test binary to before m.Run().  Exposed at package scope so
// TestIsolatedHomeCoversBothChannels can assert on it.
var isolatedTestHome string

// TestIsolatedHomeCoversBothChannels asserts that after TestMain's isolation,
// HOME, USERPROFILE, and agentconfig.ClaudeConfigDir all resolve inside the
// synthetic home — never the developer's real home directory.
//
// Conclusion: the HOME/USERPROFILE isolation installed by TestMain (ML-1A,
// netexec_test.go) blocks both the environment-variable channels (HOME and
// USERPROFILE) and the production config-dir channel
// (agentconfig.ClaudeConfigDir via OSCLIHomeResolver → homedir.Dir()),
// ensuring no test in this package reads or writes the developer's real
// ~/.claude/settings.json.  Removing either os.Setenv call in TestMain makes
// this test fail with the real path.
func TestIsolatedHomeCoversBothChannels(t *testing.T) {
	if isolatedTestHome == "" {
		t.Fatal("TestMain did not record the synthetic home")
	}

	// HOME: the channel internal/homedir reads first, on every platform.
	if got := os.Getenv("HOME"); got != isolatedTestHome {
		t.Errorf("HOME = %q, want the synthetic home %q", got, isolatedTestHome)
	}

	// USERPROFILE: the channel os.UserHomeDir() reads on Windows.  Asserted on
	// every platform so a regression is caught by the POSIX CI, not only by the
	// Windows job.
	if got := os.Getenv("USERPROFILE"); got != isolatedTestHome {
		t.Errorf("USERPROFILE = %q, want the synthetic home %q", got, isolatedTestHome)
	}

	// agentconfig.ClaudeConfigDir: the production channel exercised by commands
	// production code (e.g. validator_git_branch_guard.go:210, :434, :494).
	// Must resolve inside the synthetic home — not the developer's ~/.claude.
	// A migration from homedir.Dir() to ClaudeConfigDir in the production code
	// would not be caught by the two checks above; this check closes that gap
	// (Wave 0, Prescrição 3).
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

// TestGitGuardFailsFastWhenGitAbsent re-executes this test binary with a PATH
// that has no git and asserts exit code 2, the diagnostic line in stderr, and
// no PASS in stdout.  TRACKFW_TEST_* env vars are stripped from the child so
// the re-exec does not fall into HANG/SPAWN/SHORT_SLEEP mode before the guard.
// The second arm (POSIX only) uses a fake git that exits 127, proving the guard
// executes git rather than just locating it (Wave 0, Prescrição 10).
//
// Conclusion (ML-2A): with git absent from PATH, the commands package exits 2
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
		env := gitGuardChildEnv(emptyDir, []string{"TRACKFW_TEST_"})
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
			cmd.Env = gitGuardChildEnv(fakeDir, []string{"TRACKFW_TEST_"})
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			_ = cmd.Run()
			assertGitGuardExit(t, cmd, stdout.String(), stderr.String())
		})
	}
}

// gitGuardChildEnv returns the current environment with PATH replaced by pathDir.
// extraDropPrefixes lists additional env-key prefixes to exclude (e.g. "TRACKFW_TEST_"
// in the commands package to prevent the child from entering a helper mode before
// the git guard).
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
