package validator

import (
	"os"
	"testing"
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

	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// TestIsolatedHomeCoversBothChannels is the loop-breaker for the isolation
// above: it fails if either channel stops pointing at the synthetic home.
// Removing either os.Setenv call in TestMain makes it fail — verified in both
// directions on macOS (POSIX), where the USERPROFILE half is asserted on the
// environment variable rather than on os.UserHomeDir(), since only Windows
// resolves the home through it.
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
}
