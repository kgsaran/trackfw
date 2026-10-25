package translog

import (
	"os"
	"testing"
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
func TestMain(m *testing.M) {
	if err := os.Setenv("GIT_CONFIG_PARAMETERS", "'gc.auto=0' 'maintenance.auto=false'"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
