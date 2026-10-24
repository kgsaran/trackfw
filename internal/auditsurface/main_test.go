package auditsurface

import (
	"os"
	"testing"
)

// TestMain sets GIT_CONFIG_PARAMETERS once for the entire test binary so that
// every git child process — direct or via a helper — runs with gc.auto=0 and
// maintenance.auto=false.
//
// GIT_CONFIG_PARAMETERS is the mechanism chosen because it stacks with
// GIT_CONFIG_COUNT (independent mechanism): helpers that define their own
// GIT_CONFIG_COUNT do not override it, and helpers that set a custom
// GIT_CONFIG_GLOBAL to a file or /dev/null also do not override it.
// Measured via GIT_TRACE2_EVENT: with this variable set and a helper that
// defines GIT_CONFIG_COUNT=3 (no gc.auto keys), maintenance events drop from
// 15 to 0 (contra-arm: 15; arm: 0).
//
// Background: git maintenance run --auto --detach (triggered by git commit and
// git merge) spawns a background process that writes to .git/objects. When the
// process outlives the test, t.TempDir's RemoveAll races with it and fails with
// "directory not empty". Disabling auto-maintenance prevents the race.
func TestMain(m *testing.M) {
	if err := os.Setenv("GIT_CONFIG_PARAMETERS", "'gc.auto=0' 'maintenance.auto=false'"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
