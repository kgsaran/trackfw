package validator

import (
	"os"
	"testing"
)

// TestMain isolates $HOME to a synthetic, empty, per-run temp directory for
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
// conflict with or weaken those tests.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "trackfw-validator-test-home-*")
	if err != nil {
		panic(err)
	}

	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}

	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
