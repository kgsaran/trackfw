//go:build !windows

package commands

// netexec_groupkill_unix_test.go — AC2 test for the Unix process-group kill path.
//
// TestGitExec_AC2_GroupKill verifies that when there is no controlling terminal
// (hasControllingTTY forced false), defaultGitExec returns within the deadline and
// leaves zero orphans. The bite (removing cmd.Cancel) is demonstrated externally by
// temporarily removing the group kill from netexec_unix.go and running this test.

import (
	"errors"
	"os"
	"path/filepath"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/netdeadline"
)

// TestGitExec_AC2_GroupKill runs defaultGitExec against a hanging grandchild with the
// GroupKill path (hasControllingTTY forced false, Setpgid+Cancel active).
//
// Conclusion: with GroupKill, defaultGitExec returns within timeout + WaitDelay, the error
// is a TimeoutError, and the helper process does not survive as an orphan (0 orphans).
func TestGitExec_AC2_GroupKill(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	// Force GroupKill path (no controlling terminal → Setpgid+Cancel active).
	origTTY := hasControllingTTY
	hasControllingTTY = func() bool { return false }
	defer func() { hasControllingTTY = origTTY }()

	repoDir := initFakeRepoWithSSHRemote(t)
	pidFile := filepath.Join(t.TempDir(), "gkill-helper.pid")

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

	const protectionTimeout = 10 * time.Second // 3s deadline + 5s WaitDelay + 2s margin

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := defaultGitExec("-C", repoDir, "fetch", "origin")
		done <- err
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)

		// Wait for PID file so we can check orphan status.
		if pid := waitForPIDFile(t, pidFile, 3*time.Second); pid > 0 {
			helperPID = pid
		}

		// Lower bound: must have waited at least the deadline.
		if elapsed < 3*time.Second {
			t.Errorf("elapsed %s < 3s deadline; deadline was not honoured", elapsed)
		}

		// Upper bound: must have returned within deadline + WaitDelay + 1s margin.
		maxExpected := 3*time.Second + cmdWaitDelay + 1*time.Second
		if elapsed >= maxExpected {
			t.Errorf("elapsed %s >= %s (deadline+WaitDelay+margin)", elapsed, maxExpected)
		}

		// Error must be a network timeout.
		if err == nil {
			t.Error("expected error, got nil")
		} else if !errors.Is(err, netdeadline.ErrTimeout) {
			t.Errorf("error should be ErrTimeout; got: %v", err)
		}

		// Orphan check: helper should be dead (group was killed).
		// Poll for up to 2 s after the group kill.
		if helperPID > 0 {
			orphanDeadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(orphanDeadline) {
				if !isProcessAlive(helperPID) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if isProcessAlive(helperPID) {
				t.Errorf("helper pid=%d survived GroupKill (expected 0 orphans)", helperPID)
			} else {
				t.Logf("helper pid=%d terminated by group kill (0 orphans)", helperPID)
			}
		}

		t.Logf("AC2 GroupKill: returned in %s with err=%v", elapsed, err)

	case <-time.After(protectionTimeout):
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil {
				helperPID = pid
			}
		}
		t.Fatalf("PROTECTION TIMEOUT (%s): defaultGitExec did not return", protectionTimeout)
	}
}
