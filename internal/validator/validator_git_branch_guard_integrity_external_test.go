package validator_test

// External test package (validator_test, not validator) so it can import BOTH
// internal/generators and internal/validator without reintroducing the import cycle that
// production code must avoid (generators/context.go already imports validator). See the comment
// on gitBranchGuardScriptReference in validator_git_branch_guard_reference.go for the full
// rationale.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// TestGitBranchGuardScriptReference_MatchesGenerator proves the validator-local copy of the
// git-branch-guard script is byte-identical to what generators.GenerateGitBranchGuardScript
// actually emits. If this fails, someone edited scripts/trackfw-git-branch-guard.sh's template in
// internal/generators/scaffold.go without updating
// internal/validator/validator_git_branch_guard_reference.go — the exact drift the
// git_branch_guard_script_integrity rule depends on NOT existing between this constant and the
// real generator.
func TestGitBranchGuardScriptReference_MatchesGenerator(t *testing.T) {
	dir := t.TempDir()
	if err := generators.GenerateGitBranchGuardScript(dir); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh"))
	if err != nil {
		t.Fatalf("reading generated script: %v", err)
	}

	want := validator.GitBranchGuardScriptReferenceForTest()
	if string(got) != want {
		t.Fatalf(
			"gitBranchGuardScriptReference in internal/validator is out of sync with "+
				"generators.GenerateGitBranchGuardScript's output — update "+
				"internal/validator/validator_git_branch_guard_reference.go\n"+
				"got %d bytes, want %d bytes",
			len(got), len(want),
		)
	}
}

// TestGitBranchGuardScriptReference_MatchesGlobalGenerator proves the SAME validator-local
// reference constant also matches generators.GenerateGlobalGitBranchGuardScript's output — see
// the doc comment on gitBranchGuardScriptReference for why one constant covers both scopes
// (unlike credential-guard, which needs two): gitBranchGuardScript is written verbatim by both
// GenerateGitBranchGuardScript and GenerateGlobalGitBranchGuardScript in scaffold.go.
func TestGitBranchGuardScriptReference_MatchesGlobalGenerator(t *testing.T) {
	home := t.TempDir()
	if err := generators.GenerateGlobalGitBranchGuardScript(home); err != nil {
		t.Fatalf("GenerateGlobalGitBranchGuardScript: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh"))
	if err != nil {
		t.Fatalf("reading generated global script: %v", err)
	}

	want := validator.GitBranchGuardScriptReferenceForTest()
	if string(got) != want {
		t.Fatalf(
			"gitBranchGuardScriptReference in internal/validator is out of sync with "+
				"generators.GenerateGlobalGitBranchGuardScript's output\n"+
				"got %d bytes, want %d bytes",
			len(got), len(want),
		)
	}
}
