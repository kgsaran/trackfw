package validator_test

// validator_antigravity_ide_global_guard_test.go — ML-4E (REQ-2026-11-17)
//
// Tests for the antigravity_ide_global_guard rule (D8, ADR-2026-11-18).
//
// All tests use t.TempDir() for HOME — never the real ~/.gemini or ~/.antigravity-ide.
// Tests that go through ValidateForTest() use setupConcordanceDir (which sets HOME to
// a fresh TempDir) so OSCLIHomeResolver.HomeFor resolves to the temp directory.
// Tests that use ValidateAntigravityIDEGlobalGuardWarningsWithForTest inject a
// FixedCLIHomeResolver directly.
//
// Regra de reconciliação (CLAUDE.md): each test carries a one-sentence "Conclusion"
// comment declaring which assertion of this ML it makes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// antigravityProjectGuardJSON is the .agents/hooks.json content used in tests.
// Schema: hookCmdSchemaAntigravity (map of group → events).
const antigravityProjectGuardJSON = `{
  "trackfw": {
    "PreToolUse": [
      {
        "matcher": "run_command|manage_task",
        "hooks": [{"type": "command", "command": "trackfw guard credential || exit 2"}]
      }
    ]
  }
}`

// antigravityGlobalGuardJSON is the global hooks.json content with credential guard.
const antigravityGlobalGuardJSON = `{
  "trackfw": {
    "PreToolUse": [
      {
        "matcher": "run_command|manage_task",
        "hooks": [{"type": "command", "command": "trackfw guard credential || exit 2"}]
      }
    ]
  }
}`

// writeProjectGuard creates .agents/hooks.json with credential guard in projectDir.
func writeProjectGuard(t *testing.T, projectDir string) {
	t.Helper()
	agentsDir := filepath.Join(projectDir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("MkdirAll .agents: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "hooks.json"), []byte(antigravityProjectGuardJSON), 0644); err != nil {
		t.Fatalf("write .agents/hooks.json: %v", err)
	}
}

// makeIDEDir creates <home>/.antigravity-ide/ as a directory.
func makeIDEDir(t *testing.T, homeDir string) {
	t.Helper()
	ideDir := filepath.Join(homeDir, ".antigravity-ide")
	if err := os.MkdirAll(ideDir, 0755); err != nil {
		t.Fatalf("MkdirAll .antigravity-ide: %v", err)
	}
}

// writeGlobalGuard creates ~/.gemini/config/hooks.json with credential guard under homeDir.
func writeGlobalGuard(t *testing.T, homeDir string) {
	t.Helper()
	globalDir := filepath.Join(homeDir, ".gemini", "config")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("MkdirAll .gemini/config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "hooks.json"), []byte(antigravityGlobalGuardJSON), 0644); err != nil {
		t.Fatalf("write global hooks.json: %v", err)
	}
}

// newFixedResolver returns a FixedCLIHomeResolver backed by homeDir.
func newFixedResolver(homeDir string) agentconfig.FixedCLIHomeResolver {
	return agentconfig.FixedCLIHomeResolver{Home: homeDir}
}

// --------------------------------------------------------------------------
// Tests using the resolver-injectable entry point (FixedCLIHomeResolver)
// --------------------------------------------------------------------------

// TestAntigravityIDEGlobalGuard_AllThreeConditions_Warns asserts that when the project
// has a credential guard, the IDE is installed, and the global hook lacks a guard,
// the rule emits exactly one warning message. (Conclusion: rule fires on 3-condition match.)
func TestAntigravityIDEGlobalGuard_AllThreeConditions_Warns(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	r := newFixedResolver(homeDir)

	writeProjectGuard(t, projectDir)
	makeIDEDir(t, homeDir)
	// No global guard written.

	msgs := validator.ValidateAntigravityIDEGlobalGuardWarningsWithForTest(projectDir, r)
	if len(msgs) == 0 {
		t.Fatal("expected warning when all 3 D8 conditions hold; got none")
	}
	const marker = "Antigravity IDE ignores the project hook"
	if !strings.Contains(msgs[0], marker) {
		t.Errorf("warning must contain %q; got: %q", marker, msgs[0])
	}
}

// TestAntigravityIDEGlobalGuard_NoIDEDir_Silent asserts that when the IDE directory
// does not exist, the rule is silent even if the project guard is present and global lacks guard.
// (Conclusion: absent IDE directory → rule silent.)
func TestAntigravityIDEGlobalGuard_NoIDEDir_Silent(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	r := newFixedResolver(homeDir)

	writeProjectGuard(t, projectDir)
	// No .antigravity-ide directory created.

	msgs := validator.ValidateAntigravityIDEGlobalGuardWarningsWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected silence when IDE dir absent; got: %v", msgs)
	}
}

// TestAntigravityIDEGlobalGuard_IDEDirIsFile_Silent asserts that when <home>/.antigravity-ide
// exists as a regular file (not a directory), the rule is silent.
// (Conclusion: .antigravity-ide as a file, not a dir → rule treats as not installed.)
func TestAntigravityIDEGlobalGuard_IDEDirIsFile_Silent(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	r := newFixedResolver(homeDir)

	writeProjectGuard(t, projectDir)
	// Write .antigravity-ide as a file, not a directory.
	idePath := filepath.Join(homeDir, ".antigravity-ide")
	if err := os.WriteFile(idePath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write .antigravity-ide file: %v", err)
	}

	msgs := validator.ValidateAntigravityIDEGlobalGuardWarningsWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected silence when .antigravity-ide is a file; got: %v", msgs)
	}
}

// TestAntigravityIDEGlobalGuard_GlobalGuardPresent_Silent asserts that when the global
// hooks.json has a credential guard, the rule is silent even if the IDE is installed.
// (Conclusion: global guard present → rule silent, user is protected.)
func TestAntigravityIDEGlobalGuard_GlobalGuardPresent_Silent(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	r := newFixedResolver(homeDir)

	writeProjectGuard(t, projectDir)
	makeIDEDir(t, homeDir)
	writeGlobalGuard(t, homeDir)

	msgs := validator.ValidateAntigravityIDEGlobalGuardWarningsWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected silence when global guard present; got: %v", msgs)
	}
}

// TestAntigravityIDEGlobalGuard_NoProjectGuard_Silent asserts that when the project
// has no credential guard in .agents/hooks.json, the rule is silent.
// (Conclusion: absent project guard → rule silent, condition 1 unmet.)
func TestAntigravityIDEGlobalGuard_NoProjectGuard_Silent(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	r := newFixedResolver(homeDir)

	// No project guard written.
	makeIDEDir(t, homeDir)

	msgs := validator.ValidateAntigravityIDEGlobalGuardWarningsWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected silence when project has no guard; got: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Tests through ValidateForTest() — exercises applyRuleWarnOnly routing
// --------------------------------------------------------------------------

// TestAntigravityIDEGlobalGuard_ValidateForTest_Warns asserts that the rule surfaces
// through ValidateForTest() (which routes through ValidateUnfiltered) as a warning,
// never a violation. This exercises the applyRuleWarnOnly wiring in validator.go.
// (Conclusion: rule is routed via applyRuleWarnOnly and never appears as violation.)
func TestAntigravityIDEGlobalGuard_ValidateForTest_Warns(t *testing.T) {
	dir := setupConcordanceDir(t) // sets HOME to a fresh TempDir
	t.Setenv("CODEX_HOME", "")

	// Get the HOME that setupConcordanceDir set, so we can place the IDE dir there.
	homeDir := os.Getenv("HOME")

	if err := generators.InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}
	makeIDEDir(t, homeDir)
	// No global guard.

	violations, warnings, err := validator.ValidateForTest()
	if err != nil {
		t.Fatalf("ValidateForTest: %v", err)
	}

	const ideMarker = "Antigravity IDE ignores the project hook"
	for _, v := range violations {
		if strings.Contains(v, ideMarker) {
			t.Errorf("antigravity_ide_global_guard must be a warning, not a violation; got: %q", v)
		}
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, ideMarker) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected antigravity_ide_global_guard warning through ValidateForTest; warnings=%v violations=%v", warnings, violations)
	}
}

// TestAntigravityIDEGlobalGuard_StrictMode_StillWarning asserts that the rule is always a
// warning even when rules.antigravity_ide_global_guard: error is set in trackfw.yaml.
// (Conclusion: applyRuleWarnOnly prevents escalation to violation in strict mode.)
func TestAntigravityIDEGlobalGuard_StrictMode_StillWarning(t *testing.T) {
	dir := setupConcordanceDir(t)
	t.Setenv("CODEX_HOME", "")

	homeDir := os.Getenv("HOME")

	trackfwYAML := "rules:\n  antigravity_ide_global_guard: error\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(trackfwYAML), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	if err := generators.InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}
	makeIDEDir(t, homeDir)

	violations, warnings, err := validator.ValidateForTest()
	if err != nil {
		t.Fatalf("ValidateForTest: %v", err)
	}

	const ideMarker = "Antigravity IDE ignores the project hook"
	for _, v := range violations {
		if strings.Contains(v, ideMarker) {
			t.Errorf("antigravity_ide_global_guard must be warning even with rules: error; got violation: %q", v)
		}
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, ideMarker) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected antigravity_ide_global_guard warning in strict mode; warnings=%v", warnings)
	}
}
