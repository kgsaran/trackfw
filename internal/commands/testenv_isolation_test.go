package commands

// testenv_isolation_test.go — sentinel that the HOME isolation installed by
// TestMain (netexec_test.go) covers both OS-environment channels and the
// production agentconfig.ClaudeConfigDir path.
//
// Regra Dura de Reconciliação: each test declares, in its doc comment, the
// conclusion it asserts.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
