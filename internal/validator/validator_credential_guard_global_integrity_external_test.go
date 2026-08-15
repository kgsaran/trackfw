package validator_test

// External test package (validator_test, not validator) so it can import BOTH
// internal/generators and internal/validator without reintroducing the import cycle that
// production code must avoid. See the comment on credentialGuardGlobalScriptReference in
// validator_credential_guard_global_reference.go for the full rationale.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// TestCredentialGuardGlobalScriptReference_MatchesGenerator proves the validator-local copy of
// the GLOBAL-scope credential-guard script is byte-identical to what
// generators.GenerateGlobalCredentialGuardScript actually emits. If this fails, someone edited
// globalCredentialGuardScript's template in internal/generators/scaffold.go without updating
// internal/validator/validator_credential_guard_global_reference.go.
func TestCredentialGuardGlobalScriptReference_MatchesGenerator(t *testing.T) {
	home := t.TempDir()
	if err := generators.GenerateGlobalCredentialGuardScript(home); err != nil {
		t.Fatalf("GenerateGlobalCredentialGuardScript: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh"))
	if err != nil {
		t.Fatalf("reading generated global script: %v", err)
	}

	want := validator.CredentialGuardGlobalScriptReferenceForTest()
	if string(got) != want {
		t.Fatalf(
			"credentialGuardGlobalScriptReference in internal/validator is out of sync with "+
				"generators.GenerateGlobalCredentialGuardScript's output — update "+
				"internal/validator/validator_credential_guard_global_reference.go\n"+
				"got %d bytes, want %d bytes",
			len(got), len(want),
		)
	}
}
