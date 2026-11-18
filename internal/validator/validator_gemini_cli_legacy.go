package validator

// validator_gemini_cli_legacy.go — ML-3C (REQ-2026-11-17)
//
// Rule: gemini_cli_legacy
//
// D7 (ADR-2026-11-18): when a project has a Gemini CLI guard (.gemini/settings.json with
// credential guard marker) but NO Antigravity guard (.agents/hooks.json with credential guard
// marker), emit a warning: the Gemini CLI is legacy for individual accounts since 2026-06-18;
// suggest `trackfw agents install --targets antigravity`.
//
// This rule is ALWAYS a warning — never a violation, even in strict mode.
// Use applyRuleWarnOnly (not applyRule) so it cannot be overridden to "error" via
// trackfw.yaml rules.
//
// Logic:
//   - Gemini has guard AND Antigravity lacks guard → warn.
//   - Both have guard → silent.
//   - Neither has guard (Gemini absent or unreadable) → silent.
//   - Gemini has no guard → silent.

import (
	"encoding/json"
	"path/filepath"
)

// validateGeminiCliLegacyWarnings returns a warning message when the project has a Gemini CLI
// credential guard installed but no Antigravity credential guard. D7 (ADR-2026-11-18).
//
// The returned slice is either empty (silent) or contains a single advisory message.
// Error return is nil in all normal paths — read failures are treated as "not installed" (silent).
func validateGeminiCliLegacyWarnings(root string) ([]string, error) {
	geminiHasGuard := fileHasCredentialGuardMarker(root, ".gemini/settings.json", hookCmdSchemaNested)
	if !geminiHasGuard {
		// No Gemini guard — rule is silent regardless of Antigravity state.
		return nil, nil
	}

	antigravityHasGuard := fileHasCredentialGuardMarker(root, ".agents/hooks.json", hookCmdSchemaAntigravity)
	if antigravityHasGuard {
		// Both present — silent.
		return nil, nil
	}

	return []string{
		"Gemini CLI é legado para contas individuais desde 2026-06-18 (D7, ADR-2026-11-18): " +
			"o projeto tem guard do Gemini CLI (.gemini/settings.json) mas não tem guard do Antigravity " +
			"(.agents/hooks.json); execute `trackfw agents install --targets antigravity`",
	}, nil
}

// fileHasCredentialGuardMarker returns true when hookRelPath exists on disk under root and
// contains the credential guard subcommand marker in a structurally valid position for schema.
// Uses readRegularFile (not os.ReadFile) to stay consistent with the rest of the validator
// (write-containment policy, raw-read-ban gate).
func fileHasCredentialGuardMarker(root, hookRelPath string, schema hookCmdSchemaKind) bool {
	fullPath := filepath.Join(root, hookRelPath)
	data, err := readRegularFile(fullPath)
	if err != nil {
		return false
	}
	var parsed interface{}
	if json.Unmarshal(data, &parsed) != nil {
		return false
	}
	var matches []guardCommandMatch
	collectCommandsWithMarkerBySchema(parsed, schema, credentialGuardSubcmdMarker, &matches)
	return len(matches) > 0
}

// ValidateGeminiCliLegacyWarningsForDoctor is the exported entry point used by
// internal/commands/doctor.go. It runs validateGeminiCliLegacyWarnings from projectRoot
// and returns any warning messages (empty slice = silent).
// Errors are silently discarded — doctor reports by finding, not by I/O error.
func ValidateGeminiCliLegacyWarningsForDoctor(projectRoot string) []string {
	msgs, _ := validateGeminiCliLegacyWarnings(projectRoot)
	return msgs
}
