package validator

import "time"

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
