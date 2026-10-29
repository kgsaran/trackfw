package guardforms

import (
	"os"
	"strings"
	"testing"
)

// TestREADMEHookLinesMatchCurrentConstants extracts every line inside a fenced
// code block (``` or ~~~) in README.md that contains the substring "trackfw guard"
// and asserts:
//   (a) each such line is byte-equal to a known current constant in this package;
//   (b) each required constant appears at least once in the README fenced blocks;
//   (c) at least one line is found (vacuity guard — the test fails if the README
//       has no fenced block with a "trackfw guard" line at all).
//
// Failing in two directions:
//   - If the README contains a legacy form (e.g. the pre-10.1.0 form without the
//     "emulate" prefix), the test diagnoses the exact legacy constant name.
//   - If a current constant is mutated (e.g. a character changed), the README line
//     no longer matches the constant, so the test fails with "unrecognized hook line".
//
// Affirms: the hook lines documented in README.md are byte-identical to the
// canonical forms emitted by the current trackfw binary (AC2, REQ-2026-10-28).
func TestREADMEHookLinesMatchCurrentConstants(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("cannot read ../../README.md: %v", err)
	}

	// current forms: constant value → constant name
	currentForms := map[string]string{
		CredentialCurrentPSPOSIX:       "CredentialCurrentPSPOSIX",
		GitBranchCurrentPSPOSIX:        "GitBranchCurrentPSPOSIX",
		WriteScopeCurrentPSPOSIX:       "WriteScopeCurrentPSPOSIX",
		CredentialGlobalCurrentPSPOSIX: "CredentialGlobalCurrentPSPOSIX",
		CredentialCurrentCmdExe:        "CredentialCurrentCmdExe",
		GitBranchCurrentCmdExe:         "GitBranchCurrentCmdExe",
		CredentialGlobalCurrentCmdExe:  "CredentialGlobalCurrentCmdExe",
	}

	// legacy forms: constant value → list name, for diagnostic messages
	legacyForms := buildLegacyLookup()

	hookLines := extractFencedGuardLines(string(data))

	// (c) vacuity guard
	if len(hookLines) == 0 {
		t.Fatal("no 'trackfw guard' lines found inside fenced code blocks in README.md — " +
			"the test is vacuous; add canonical hook lines to README.md fenced blocks")
	}

	seen := map[string]bool{}
	for _, line := range hookLines {
		if name, ok := currentForms[line]; ok {
			seen[line] = true
			_ = name // recognized as current
		} else if legName, ok := legacyForms[line]; ok {
			t.Errorf("README.md fenced block contains a legacy hook form (%s):\n  %q\n"+
				"  → update to the current form (run `trackfw update` or copy from "+
				"  internal/guardforms/guard_generations.go)", legName, line)
		} else {
			t.Errorf("README.md fenced block contains an unrecognized hook line:\n  %q", line)
		}
	}

	// (b) required constants must each appear at least once
	required := []struct {
		val  string
		name string
	}{
		{CredentialCurrentPSPOSIX, "CredentialCurrentPSPOSIX"},
		{GitBranchCurrentPSPOSIX, "GitBranchCurrentPSPOSIX"},
		{CredentialCurrentCmdExe, "CredentialCurrentCmdExe"},
		{GitBranchCurrentCmdExe, "GitBranchCurrentCmdExe"},
	}
	for _, r := range required {
		if !seen[r.val] {
			t.Errorf("README.md is missing required current constant %s:\n  %q", r.name, r.val)
		}
	}
}

// buildLegacyLookup returns a map from every known legacy form value to the
// name of the list it belongs to, for use in diagnostic messages.
func buildLegacyLookup() map[string]string {
	result := map[string]string{}
	for _, f := range CredentialLegacyPSPOSIX {
		result[f] = "CredentialLegacyPSPOSIX"
	}
	for _, f := range CredentialGlobalLegacyPSPOSIX {
		result[f] = "CredentialGlobalLegacyPSPOSIX"
	}
	for _, f := range CredentialLegacyCmdExe {
		result[f] = "CredentialLegacyCmdExe"
	}
	for _, f := range GitBranchLegacyPSPOSIX {
		result[f] = "GitBranchLegacyPSPOSIX"
	}
	for _, f := range GitBranchLegacyCmdExe {
		result[f] = "GitBranchLegacyCmdExe"
	}
	return result
}

// extractFencedGuardLines returns every line inside a fenced code block
// (opened by a line whose TrimSpace starts with "```" or "~~~") that contains
// the substring "trackfw guard". Lines are returned with CRLF stripped.
//
// The detector uses a simple toggle: any line whose TrimSpace starts with the
// fence characters opens or closes the fenced state. This is correct for the
// README as authored (no nested fences), and the .gitattributes rule does not
// cover *.md, so a CRLF checkout could add \r — hence the explicit TrimRight.
func extractFencedGuardLines(content string) []string {
	var result []string
	inFence := false
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence && strings.Contains(line, "trackfw guard") {
			result = append(result, line)
		}
	}
	return result
}
