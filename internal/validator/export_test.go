package validator

import (
	"fmt"
	"os"
	"path/filepath"
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

// ValidateClaudeGuardHookMatcherWarningsForTest exposes validateClaudeGuardHookMatcherWarnings
// to the external test package (package validator_test) — standard "export_test.go" pattern.
// The caller must have chdired into a directory that contains .claude/settings.json before
// calling; ChdirForTest handles this in concordance tests.
// Used by ML-5F concordance test (validator_guard_hook_concordance_external_test.go).
func ValidateClaudeGuardHookMatcherWarningsForTest() ([]string, error) {
	return validateClaudeGuardHookMatcherWarnings()
}

// ValidateCredentialGuardGlobalHookResolvableForTest exposes
// validateCredentialGuardGlobalHookResolvable to the external test package.
// ML-1A (REQ-2026-09-01): used by the global concordance tests that exercise
// the HOME≠USERPROFILE and CLAUDE_CONFIG_DIR code paths.
func ValidateCredentialGuardGlobalHookResolvableForTest() ([]string, error) {
	return validateCredentialGuardGlobalHookResolvable()
}

// ValidateGitBranchGuardGlobalHookResolvableForTest exposes
// validateGitBranchGuardGlobalHookResolvable to the external test package.
// ML-1A (REQ-2026-09-01): same contract as ValidateCredentialGuardGlobalHookResolvableForTest.
func ValidateGitBranchGuardGlobalHookResolvableForTest() ([]string, error) {
	return validateGitBranchGuardGlobalHookResolvable()
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

// CredentialGuardGlobalExpectedCmdWindsurfForTest exposes the exact D11 revised global
// credential guard command for Windsurf to the external test package (package validator_test).
// Used by the ML-2C concordance test to verify that the validator and generator agree on the
// same command string without importing generators (which would create an import cycle).
func CredentialGuardGlobalExpectedCmdWindsurfForTest() string {
	return credentialGuardGlobalExpectedCmdWindsurf()
}

// GuardWiringTypeFromCmdPSPosixForTest exposes guardWiringTypeFromCmd for the PS/POSIX shell family
// to the external test package (package validator_test) — standard "export_test.go" pattern.
// Used by validator_guard_wiring_concordance_external_test.go to verify that every guard command
// form emitted by the generator is recognised by the validator (Q1 ML-2B).
func GuardWiringTypeFromCmdPSPosixForTest(cmd string) string {
	return guardWiringTypeFromCmd(cmd, guardShellFamilyPSPosix)
}

// GuardWiringTypeFromCmdCmdExeForTest exposes guardWiringTypeFromCmd for the cmd.exe shell family
// to the external test package — same pattern as GuardWiringTypeFromCmdPSPosixForTest.
func GuardWiringTypeFromCmdCmdExeForTest(cmd string) string {
	return guardWiringTypeFromCmd(cmd, guardShellFamilyCmdExe)
}

// GuardExpectedLinePSPosixForTest exposes guardExpectedLine for the PS/POSIX shell family
// to the external test package — used by the Q1 concordance test to check canonical inline forms.
func GuardExpectedLinePSPosixForTest(subcmd string) string {
	return guardExpectedLine(subcmd, guardShellFamilyPSPosix)
}

// GuardExpectedLineCmdExeForTest exposes guardExpectedLine for the cmd.exe shell family
// to the external test package — same pattern as GuardExpectedLinePSPosixForTest.
func GuardExpectedLineCmdExeForTest(subcmd string) string {
	return guardExpectedLine(subcmd, guardShellFamilyCmdExe)
}

// GuardWiringExtractGuardTypesFromFileForTest reads the hook file at the given relative path
// (resolved against CWD), looks it up in credentialGuardHookFiles to get the canonical shell
// family, calls guardWiringParseTuples, and returns the list of guard types
// ("credential", "git-branch") found in the tuples.
// Returns an error if path is not in credentialGuardHookFiles, the file cannot be read, or
// the content cannot be parsed. Callers should ChdirForTest into the project root first.
// Used by TestGuardWiringConcordance_GeneratorOutputRecognized (Part 2a, ML-2B Lacuna 2).
func GuardWiringExtractGuardTypesFromFileForTest(path string) ([]string, error) {
	var hf *credentialGuardHookFile
	for i := range credentialGuardHookFiles {
		if credentialGuardHookFiles[i].path == path {
			hf = &credentialGuardHookFiles[i]
			break
		}
	}
	if hf == nil {
		return nil, fmt.Errorf("path %q is not in credentialGuardHookFiles", path)
	}
	content, err := os.ReadFile(filepath.FromSlash(path)) //nolint:gosec
	if err != nil {
		return nil, err
	}
	tuples, err := guardWiringParseTuples(content, *hf)
	if err != nil {
		return nil, err
	}
	guardTypes := make([]string, 0, len(tuples))
	for _, tup := range tuples {
		guardTypes = append(guardTypes, tup.guardType)
	}
	return guardTypes, nil
}

// ValidateHomeDivergenceWindowsForTest exposes validateHomeDivergenceWindows to the
// external test package (package validator_test) — standard "export_test.go" pattern.
// ML-1B (REQ-2026-09-01): used by the home_divergence_windows tests that exercise
// the D4 warning for HOME ≠ USERPROFILE on Windows (simulated via SetPlatformForTest).
func ValidateHomeDivergenceWindowsForTest() []string {
	return validateHomeDivergenceWindows()
}

// CheckProjectForeignWriteForTest exposes checkProjectForeignWrite to the
// external test package (package validator_test) — standard "export_test.go" pattern.
// ML-W2b' (REQ-2026-09-03): used by the project_foreign_write_windows tests that
// exercise the D2/D3 warning on Windows (Windows-only build tag).
func CheckProjectForeignWriteForTest(root string) []string {
	return checkProjectForeignWrite(root)
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
