package validator_test

// External test package (validator_test, not validator) so it can import BOTH
// internal/generators and internal/validator without reintroducing the import cycle that
// production code must avoid (generators/context.go already imports validator). See the
// comment on credentialGuardScriptReference in
// validator_credential_guard_integrity_reference.go for the full rationale.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// TestCredentialGuardScriptReference_MatchesGenerator proves the validator-local copy of the
// project-scope credential-guard script is byte-identical to what
// generators.GenerateCredentialGuardScript actually emits. If this fails, someone edited
// scripts/trackfw-credential-guard.sh's template in internal/generators/scaffold.go without
// updating internal/validator/validator_credential_guard_integrity_reference.go — the exact drift
// the credential_guard_script_integrity rule depends on NOT existing between this constant and
// the real generator.
func TestCredentialGuardScriptReference_MatchesGenerator(t *testing.T) {
	dir := t.TempDir()
	if err := generators.GenerateCredentialGuardScript(dir); err != nil {
		t.Fatalf("GenerateCredentialGuardScript: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "scripts", "trackfw-credential-guard.sh"))
	if err != nil {
		t.Fatalf("reading generated script: %v", err)
	}

	want := validator.CredentialGuardScriptReferenceForTest()
	if string(got) != want {
		t.Fatalf(
			"credentialGuardScriptReference in internal/validator is out of sync with "+
				"generators.GenerateCredentialGuardScript's output — update "+
				"internal/validator/validator_credential_guard_integrity_reference.go\n"+
				"got %d bytes, want %d bytes",
			len(got), len(want),
		)
	}
}
