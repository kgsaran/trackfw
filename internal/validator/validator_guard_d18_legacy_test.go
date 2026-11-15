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

// ─── Write-scope D18 migration tests (ML-2C, REQ-2026-11-13, B2) ────────────────
//
// These tests verify the write-scope D18 migration notice added in ML-2C:
//   WS1: Legacy D18 write-scope form → migration warning (write_scope_guard_hook_resolvable)
//   WS2: Current write-scope form → no migration warning
//   WS3: Absent settings.json → no migration warning (validateWriteScopeWiringD18LegacyMigration is silent)
//   WS4: Legacy write-scope form → no write_scope_not_wired violation
//
// All tests call validator internals directly (package validator) and use chdir+config.Reset.

// TestD18WriteScopeWarning_LegacyFormEmitsMigrationNotice asserts that the D18 legacy
// write-scope form in .claude/settings.json PreToolUse[Write|Edit|NotebookEdit] produces a
// migration notice from validateWriteScopeWiringD18LegacyMigration, NOT a write_scope_not_wired
// violation — the hook is treated as "wired" with a migration notice only.
//
// Reconciliação: D18 legacy write-scope form emits a migration notice via
// validateWriteScopeWiringD18LegacyMigration — if settingsWriteScopeHookStatus does not
// return writeScopeHookLegacyD18 for LegacyD18WriteScopePSPOSIX, this test fails.
func TestD18WriteScopeWarning_LegacyFormEmitsMigrationNotice(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json",
		nestedClaudeSettingsJSON("Write|Edit|NotebookEdit", guardforms.LegacyD18WriteScopePSPOSIX))

	warnings := validateWriteScopeWiringD18LegacyMigration()

	const wantSubstr = "D18 legacy write-scope form"
	if !hasWarning(warnings, wantSubstr) {
		t.Errorf("expected migration warning containing %q; got %v", wantSubstr, warnings)
	}
	if !hasWarning(warnings, ".claude/settings.json") {
		t.Errorf("expected migration warning to name the file; got %v", warnings)
	}
}

// TestD18WriteScopeWarning_CurrentFormSilent asserts that the current write-scope form
// (WriteScopeCurrentPSPOSIX, form K) does not trigger the D18 migration notice.
//
// Reconciliação: current write-scope form does not emit a migration notice — the condition
// checks settingsWriteScopeHookStatus == writeScopeHookLegacyD18, which is false for the
// current form, so the function returns nil.
func TestD18WriteScopeWarning_CurrentFormSilent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json",
		nestedClaudeSettingsJSON("Write|Edit|NotebookEdit", guardforms.WriteScopeCurrentPSPOSIX))

	warnings := validateWriteScopeWiringD18LegacyMigration()

	if len(warnings) != 0 {
		t.Errorf("expected no migration warning for current form; got %v", warnings)
	}
}

// TestD18WriteScopeWarning_AbsentSettingsSilent asserts that an absent .claude/settings.json
// does not trigger validateWriteScopeWiringD18LegacyMigration. The absence case is handled
// exclusively by validateWriteScopeWiring (write_scope_not_wired).
//
// Reconciliação: absent settings.json returns nil from validateWriteScopeWiringD18LegacyMigration —
// the function returns nil on any read error, delegating absence handling to validateWriteScopeWiring.
func TestD18WriteScopeWarning_AbsentSettingsSilent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// No .claude/settings.json written — file is absent.
	warnings := validateWriteScopeWiringD18LegacyMigration()

	if len(warnings) != 0 {
		t.Errorf("expected no migration warning when settings.json absent; got %v", warnings)
	}
}

// TestD18WriteScopeWarning_LegacyFormNotAViolation asserts that the D18 legacy write-scope
// form does not produce a write_scope_not_wired violation from validateWriteScopeWiring.
// The legacy form is treated as "wired" (writeScopeHookLegacyD18); only the migration
// notice is emitted via the separate write_scope_guard_hook_resolvable path.
//
// Reconciliação: legacy D18 write-scope form is not a write_scope_not_wired violation —
// validateWriteScopeWiring returns nil for writeScopeHookLegacyD18, so if
// settingsWriteScopeHookStatus were to return writeScopeHookAbsent for the legacy form,
// this test would fail.
func TestD18WriteScopeWarning_LegacyFormNotAViolation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json",
		nestedClaudeSettingsJSON("Write|Edit|NotebookEdit", guardforms.LegacyD18WriteScopePSPOSIX))

	violations := validateWriteScopeWiring()

	if len(violations) != 0 {
		t.Errorf("D18 legacy write-scope form should not produce a write_scope_not_wired violation; got %v", violations)
	}
}
