package validator

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
