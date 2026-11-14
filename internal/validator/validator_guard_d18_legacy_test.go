package validator

// validator_guard_d18_legacy_test.go — ML-1A2 (REQ-2026-11-13)
//
// Exercises the D18 legacy warning path in the validator:
//   - D18 legacy form → warning (not violation) emitted by validateCredentialGuardHookD18LegacyWarnings
//   - New D18 current form → no D18 warning
//   - D11Rev form → D11Rev warning (not D18 warning) from validateCredentialGuardHookD11RevisedLegacyWarnings
//   - D18 legacy form does not produce a violation via validateCredentialGuardHookResolvable
//     (it is recognized as a valid form; the warning path is distinct from the violation path)
//
// All tests call validator internals directly (package validator) and use chdir+config.Reset
// to isolate from the real working tree.
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//   Test 1 asserts: D18 legacy form in .claude/settings.json emits a D18 warning —
//     if the if-block at validateGuardHookD18LegacyWarnings (line ~1418) is commented out, this test fails.
//   Test 2 asserts: current D18 form in .claude/settings.json does not emit a D18 warning —
//     validateGuardHookD18LegacyWarnings checks m.raw == legacyD18Line, not m.raw == expectedLine.
//   Test 3 asserts: D11Rev form emits D11Rev warning only, not a D18 warning —
//     the two warning functions check different legacy constants.
//   Test 4 asserts: D18 legacy form is NOT treated as a violation by validateCredentialGuardHookResolvable —
//     the validator recognizes it as a valid (but outdated) form via the anySubcmdFormFound arm.

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/guardforms"
)

// nestedClaudeSettingsJSON returns a minimal .claude/settings.json with one PreToolUse
// entry using the given matcher and command.
func nestedClaudeSettingsJSON(matcher, command string) string {
	// Use jsonStringLiteral (defined in validator_git_branch_guard_test.go, same package) to
	// safely escape the command string — it contains semicolons, $, and other shell metacharacters.
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ` + jsonStringLiteral(matcher) + `,
        "hooks": [
          {"type": "command", "command": ` + jsonStringLiteral(command) + `}
        ]
      }
    ]
  }
}`
}

// ─── Test 1: D18 legacy form → warning ─────────────────────────────────────────

// TestD18LegacyWarning_D18LegacyFormEmitsWarning asserts that the D18 legacy credential
// form (emulate without set+eu prefix, pre-REQ-2026-11-13) produces a warning when found
// in .claude/settings.json via validateCredentialGuardHookD18LegacyWarnings.
//
// Reconciliação: D18 legacy form in Claude hooks emits a D18 legacy warning —
// if the if-block checking m.raw == legacyD18Line in validateGuardHookD18LegacyWarnings
// is disabled, this test fails.
func TestD18LegacyWarning_D18LegacyFormEmitsWarning(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Write .claude/settings.json with D18 legacy credential form.
	writeFile(t, dir, ".claude/settings.json",
		nestedClaudeSettingsJSON("Bash|PowerShell", guardforms.LegacyD18CredentialPSPOSIX))

	warnings, err := validateCredentialGuardHookD18LegacyWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookD18LegacyWarnings: %v", err)
	}

	const wantSubstr = "D18 legacy form pre-REQ-2026-11-13"
	if !hasWarning(warnings, wantSubstr) {
		t.Errorf("expected a D18 legacy warning containing %q; got %v", wantSubstr, warnings)
	}
	// Also assert it names the file and the CLI.
	if !hasWarning(warnings, ".claude/settings.json") {
		t.Errorf("expected warning to name the file; got %v", warnings)
	}
}

// ─── Test 2: Current D18 form → no D18 warning ─────────────────────────────────

// TestD18LegacyWarning_CurrentD18FormSilent asserts that the current D18 credential form
// (with set+eu prefix, the canonical D18 form) does not trigger a D18 legacy warning.
//
// Reconciliação: current D18 form does not emit a D18 legacy warning — the condition checks
// m.raw == legacyD18Line (not m.raw == expectedLine), so the two forms are distinguishable.
func TestD18LegacyWarning_CurrentD18FormSilent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json",
		nestedClaudeSettingsJSON("Bash|PowerShell", guardforms.CredentialCurrentPSPOSIX))

	warnings, err := validateCredentialGuardHookD18LegacyWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookD18LegacyWarnings: %v", err)
	}

	if len(warnings) != 0 {
		t.Errorf("expected no D18 warning for current form; got %v", warnings)
	}
}

// ─── Test 3: D11Rev form → D11Rev warning only, not D18 warning ────────────────

// TestD18LegacyWarning_D11RevFormEmitsD11RevNotD18 asserts that the D11 revised legacy
// form (pre-REQ-2026-10-20, without emulate prefix) produces a D11Rev warning via
// validateCredentialGuardHookD11RevisedLegacyWarnings, and does NOT produce a D18 warning
// via validateCredentialGuardHookD18LegacyWarnings.
//
// Reconciliação: D11Rev form emits only a D11Rev warning, not a D18 warning — the two
// warning functions check different guard legacy constants (guardD11RevisedLegacyLine vs
// guardEmulateLegacyLine), so a D11Rev form cannot accidentally trigger the D18 path.
func TestD18LegacyWarning_D11RevFormEmitsD11RevNotD18(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json",
		nestedClaudeSettingsJSON("Bash|PowerShell", guardforms.LegacyD11RevCredentialPSPOSIX))

	// D18 warning function must be silent for a D11Rev form.
	d18Warnings, err := validateCredentialGuardHookD18LegacyWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookD18LegacyWarnings: %v", err)
	}
	if len(d18Warnings) != 0 {
		t.Errorf("expected no D18 warning for D11Rev form; got %v", d18Warnings)
	}

	// D11Rev warning function must fire for a D11Rev form.
	d11RevWarnings, err := validateCredentialGuardHookD11RevisedLegacyWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookD11RevisedLegacyWarnings: %v", err)
	}
	const wantSubstr = "D11 revised"
	if !hasWarning(d11RevWarnings, wantSubstr) {
		t.Errorf("expected a D11Rev warning containing %q for D11Rev form; got %v", wantSubstr, d11RevWarnings)
	}
}

// ─── Test 4: D18 legacy form is not a violation ─────────────────────────────────

// TestD18LegacyWarning_D18LegacyNotAViolation asserts that a .claude/settings.json
// containing the D18 legacy credential form does not produce a violation from
// validateCredentialGuardHookResolvable. The D18 legacy form is a recognized (if outdated)
// form — the validator admits it as anySubcmdFormFound=true and routes it to the warning path.
//
// Reconciliação: D18 legacy form in Claude hooks does not produce a credential_guard_hook_resolvable
// violation — the form is recognized by guardWiringTypeFromCmd (which now accepts the D18 legacy
// emulate form) and anySubcmdFormFound is set before the violation-emission arm is reached.
func TestD18LegacyWarning_D18LegacyNotAViolation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Also need a scripts/ credential guard script so the hook path resolves correctly
	// (validateCredentialGuardHookResolvable checks script path anchor).
	// Use a valid credential guard script content.
	writeFile(t, dir, "scripts/trackfw-credential-guard.sh", credentialGuardScriptReference)

	writeFile(t, dir, ".claude/settings.json",
		nestedClaudeSettingsJSON("Bash|PowerShell", guardforms.LegacyD18CredentialPSPOSIX))

	// Stub the binary probe so it succeeds without a real trackfw binary.
	StubProbeOKForTest(t)

	violations, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable: %v", err)
	}

	if hasViolation(violations, "credential") {
		t.Errorf("D18 legacy form should not produce a credential violation; got %v", violations)
	}
}
