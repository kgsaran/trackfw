package validator

// validator_antigravity_ide_global_guard.go — ML-4E (REQ-2026-11-17)
//
// Rule: antigravity_ide_global_guard
//
// D8 (ADR-2026-11-18): when a project has an Antigravity credential guard
// (.agents/hooks.json with credential guard marker), the Google Antigravity IDE
// is installed (<home>/.antigravity-ide is a directory), and the global hooks.json
// (~/.gemini/config/hooks.json) does NOT have a credential guard, emit a warning:
// the IDE ignores project-scoped hooks and only the global guard protects it.
//
// Measured on macOS (IDE 2.6.0 and 2.7.0): the IDE ignores .agents/hooks.json and
// only respects ~/.gemini/config/hooks.json (ADR-2026-11-18 D8, contrato §13).
//
// This rule is ALWAYS a warning — never a violation, even in strict mode.
// Use applyRuleWarnOnly (not applyRule) so it cannot be overridden to "error" via
// trackfw.yaml rules.
//
// Logic:
//   - Project has guard AND IDE installed AND global lacks guard → warn.
//   - Project lacks guard → silent.
//   - IDE not installed (no <home>/.antigravity-ide directory) → silent.
//   - Global has guard → silent.
//   - Home unresolvable → silent.

import (
	"os"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// validateAntigravityIDEGlobalGuardWarningsWith returns a warning message when all three
// D8 conditions hold: the project has an Antigravity credential guard, the IDE is
// installed (<home>/.antigravity-ide is a directory), and the global hooks.json lacks a
// credential guard.
//
// r is the CLIHomeResolver used to find ~/. — pass agentconfig.DefaultResolver in
// production; pass agentconfig.FixedCLIHomeResolver in tests.
func validateAntigravityIDEGlobalGuardWarningsWith(root string, r agentconfig.CLIHomeResolver) ([]string, error) {
	// Condition 1: project has credential guard.
	if !fileHasCredentialGuardMarker(root, ".agents/hooks.json", hookCmdSchemaAntigravity) {
		return nil, nil
	}

	// Condition 2: IDE installed — <home>/.antigravity-ide must be a directory.
	// os.Stat is not a file read (raw-read-ban covers os.ReadFile only).
	ideDir, err := agentconfig.AntigravityIDEDir(r)
	if err != nil {
		return nil, nil // cannot resolve home — silent
	}
	fi, statErr := os.Stat(ideDir)
	if statErr != nil || !fi.IsDir() {
		return nil, nil // absent, unreadable, or not a directory — silent
	}

	// Condition 3: global hooks.json does NOT have credential guard.
	// fileHasCredentialGuardMarker joins root+relPath; pass globalDir as root and
	// "hooks.json" as relPath so readRegularFile stays on the guarded path.
	globalDir, err := agentconfig.AntigravityConfigDir(r)
	if err != nil {
		return nil, nil
	}
	if fileHasCredentialGuardMarker(globalDir, "hooks.json", hookCmdSchemaAntigravity) {
		return nil, nil // global guard present — silent
	}

	displayPath := agentconfig.AntigravityHooksDisplayPath(globalDir)
	return []string{
		"Antigravity IDE ignores the project hook .agents/hooks.json (measured on 2.6.0 and 2.7.0, " +
			"ADR-2026-11-18 D8); only the global guard protects it — run " +
			"`trackfw update harness --targets antigravity-credential-guard --install-missing`" +
			" (" + displayPath + ")",
	}, nil
}

// validateAntigravityIDEGlobalGuardWarnings is the production entry point (DefaultResolver).
func validateAntigravityIDEGlobalGuardWarnings(root string) ([]string, error) {
	return validateAntigravityIDEGlobalGuardWarningsWith(root, agentconfig.DefaultResolver)
}

// ValidateAntigravityIDEGlobalGuardWarningsForDoctor is the exported entry point used by
// internal/commands/doctor.go and by install/init commands that need to print the warning
// on stderr after granting the project hook. Uses DefaultResolver.
// Errors are silently discarded — doctor reports by finding, not by I/O error.
func ValidateAntigravityIDEGlobalGuardWarningsForDoctor(projectRoot string) []string {
	msgs, _ := validateAntigravityIDEGlobalGuardWarnings(projectRoot)
	return msgs
}

// ValidateAntigravityIDEGlobalGuardWarningsWithForTest is the resolver-injectable entry point
// used by external tests. Injecting an agentconfig.FixedCLIHomeResolver ensures tests never
// touch the real ~/.gemini directory.
func ValidateAntigravityIDEGlobalGuardWarningsWithForTest(root string, r agentconfig.CLIHomeResolver) []string {
	msgs, _ := validateAntigravityIDEGlobalGuardWarningsWith(root, r)
	return msgs
}
