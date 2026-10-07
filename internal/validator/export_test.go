package validator

import (
	"os"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// IsLenientForTest exposes the unexported isLenientFor function to the external test package
// (package validator_test) — standard Go "export_test.go" pattern.
// Used by AC2 / AC8(a) table tests to assert the three-arm falsification without clock flake.
func IsLenientForTest(mode string, until time.Time, now time.Time) bool {
	return isLenientFor(mode, until, now)
}

// LenientCarveoutRulesForTest exposes the lenientCarveoutRules map for test verification
// of the closed set membership — allows tests to confirm the named rules are in the carve-out
// without depending on private symbol visibility.
func LenientCarveoutRulesForTest() map[string]bool {
	return lenientCarveoutRules
}

// CredentialGuardScriptReferenceForTest exposes the unexported credentialGuardScriptReference
// constant to the external test package (package validator_test) — standard Go "export_test.go"
// pattern. This file is *_test.go, so it is excluded from the production build; it does not widen
// this package's public API. See
// validator_credential_guard_integrity_external_test.go for the consumer.
func CredentialGuardScriptReferenceForTest() string {
	return credentialGuardScriptReference
}

// GitBranchGuardScriptReferenceForTest exposes the unexported gitBranchGuardScriptReference
// constant to the external test package (package validator_test) — same "export_test.go" pattern
// as CredentialGuardScriptReferenceForTest above. See
// validator_git_branch_guard_integrity_external_test.go for the consumer.
func GitBranchGuardScriptReferenceForTest() string {
	return gitBranchGuardScriptReference
}

// CredentialGuardGlobalScriptReferenceForTest exposes the unexported
// credentialGuardGlobalScriptReference constant to the external test package (package
// validator_test) — same "export_test.go" pattern as the two functions above. See
// validator_credential_guard_global_integrity_external_test.go for the consumer.
func CredentialGuardGlobalScriptReferenceForTest() string {
	return credentialGuardGlobalScriptReference
}

// ValidateGitBranchGuardHookResolvableForTest exposes validateGitBranchGuardHookResolvable to the
// external test package (package validator_test). Used by the concordance test in
// validator_guard_hook_concordance_external_test.go to exercise the rule after the generator has
// written hook files into a temp dir — without importing the production-side unexported symbol.
func ValidateGitBranchGuardHookResolvableForTest() ([]string, error) {
	return validateGitBranchGuardHookResolvable()
}

// ValidateCredentialGuardHookResolvableForTest exposes validateCredentialGuardHookResolvable to the
// external test package — same pattern as ValidateGitBranchGuardHookResolvableForTest.
func ValidateCredentialGuardHookResolvableForTest() ([]string, error) {
	return validateCredentialGuardHookResolvable()
}

// StubProbeOKForTest replaces the binary-probe seam vars so any call to guardBinaryProbeOnce
// succeeds (binary found, guard subcommand present, no Git Bash divergence). Restores the
// originals via t.Cleanup. Used by the concordance test so it does not depend on a real
// trackfw binary in PATH.
// guardFindGitBashExe is stubbed to "" to prevent the Git Bash login-probe from running on
// Windows CI runners where Git for Windows may be installed.
func StubProbeOKForTest(t testing.TB) {
	t.Helper()
	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFindGitBash := guardFindGitBashExe
	guardLookupBinary = func(_ string) (string, error) { return "/usr/local/bin/trackfw", nil }
	guardRunProbe = func(_ string) error { return nil }
	guardFindGitBashExe = func() string { return "" }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindGitBashExe = origFindGitBash
	})
}

// ChdirForTest changes the working directory to dir and resets the config singleton (required
// because Validate* functions call os.Getwd() which is cached by config.Load). Restores both
// on t.Cleanup. Safe to use from external test package (package validator_test).
func ChdirForTest(t testing.TB, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("ChdirForTest: getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("ChdirForTest: chdir(%q): %v", dir, err)
	}
	config.Reset()
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		config.Reset()
	})
}
