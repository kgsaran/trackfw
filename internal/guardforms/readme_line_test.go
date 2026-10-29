package guardforms

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// ─── CommonMark fence helpers ────────────────────────────────────────────────
//
// These helpers enforce the three-part CommonMark rule:
//   (1) A fence opens on a line with 0-3 leading spaces followed by ≥3 identical
//       fence characters ('`' or '~').
//   (2) A fence closes only when the closing line uses the SAME character, a run
//       ≥ the opening run, and nothing after the run except optional whitespace.
//   (3) Corollary: a '~~~' line inside a '```' block is content, not a close.
//
// These helpers are used by extractFencedGuardLines and extractCLIParityGuardForms.

// fenceInfo returns (fenceChar, runLen, ok) if line begins a CommonMark fence
// marker: 0-3 leading spaces, then ≥3 identical '`' or '~' characters.
// The rest of the line (info string or trailing chars) is ignored here;
// closesBlock additionally verifies the rest is empty for a valid close.
func fenceInfo(line string) (char byte, length int, ok bool) {
	s := line
	indent := 0
	for indent < len(s) && s[indent] == ' ' {
		indent++
	}
	if indent > 3 {
		return 0, 0, false
	}
	s = s[indent:]
	if len(s) < 3 {
		return 0, 0, false
	}
	ch := s[0]
	if ch != '`' && ch != '~' {
		return 0, 0, false
	}
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	return ch, n, true
}

// closesBlock reports whether line is a valid closing fence for a block opened
// with fenceChar and fenceLen: same character, run ≥ fenceLen, nothing after
// the run except optional whitespace (up to 3 leading spaces allowed).
func closesBlock(line string, fenceChar byte, fenceLen int) bool {
	ch, n, ok := fenceInfo(line)
	if !ok || ch != fenceChar || n < fenceLen {
		return false
	}
	// Strip the ≤3 leading spaces that fenceInfo already validated.
	s := strings.TrimLeft(line, " ")
	return strings.TrimSpace(s[n:]) == ""
}

// ─── README fence extractor ──────────────────────────────────────────────────

// extractFencedGuardLines returns every line inside a fenced code block that
// contains the substring "trackfw guard". Lines are returned with CRLF stripped.
//
// F-3 fix: applies CommonMark fence rules via fenceInfo/closesBlock.
// A '~~~' line inside a '```' block does not close the fence; a 3-backtick line
// does not close a 4-backtick fence; a closing candidate with text after the run
// is not a valid close.
func extractFencedGuardLines(content string) []string {
	var result []string
	inFence := false
	var fenceChar byte
	var fenceLen int

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		ch, n, ok := fenceInfo(line)
		if ok {
			if !inFence {
				// Opening a new fence.
				inFence = true
				fenceChar = ch
				fenceLen = n
				continue
			}
			// Inside a fence: only close if same char, ≥ opening length, no trailing text.
			if closesBlock(line, fenceChar, fenceLen) {
				inFence = false
				continue
			}
			// Otherwise this is content (e.g. a ~~~ line inside a ``` block).
		}
		if inFence && strings.Contains(line, "trackfw guard") {
			result = append(result, line)
		}
	}
	return result
}

// ─── TestREADMEHookLinesMatchCurrentConstants ────────────────────────────────

// TestREADMEHookLinesMatchCurrentConstants extracts every line inside a fenced
// code block in README.md that contains "trackfw guard" and asserts:
//
//	(a) each such line is byte-equal to a known current constant in this package;
//	(b) each required constant appears at least once in the README fenced blocks;
//	(c) at least one line is found (vacuity guard).
//
// Failing in two directions:
//   - If the README contains a legacy form, the test diagnoses the exact legacy
//     constant name.
//   - If a current constant is mutated, the README line no longer matches the
//     constant, so the test fails with "unrecognized hook line".
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

// ─── TestREADMEFenceExtractor ────────────────────────────────────────────────

// TestREADMEFenceExtractor asserts CommonMark fence-tracking rules in
// extractFencedGuardLines using the red-team vectors from ML-4B.
//
// Each sub-test name states the rule being asserted.
// Affirms: the extractor applies same-char + ≥-length closing, rejecting
// mixed-character closes and closers with trailing text (F-3, ML-4D).
func TestREADMEFenceExtractor(t *testing.T) {
	t.Run("B8b_tilde_inside_backtick_fence_does_not_close", func(t *testing.T) {
		// ~~~ inside a ``` block must NOT close the fence (F-3 vector B8b).
		// With the old toggle, ~~~ inverts inFence → false and the second line
		// is missed.  With CommonMark rules, ~~~ is content and both lines are
		// returned.
		// Affirms: same-char requirement blocks ~~~ from closing a ``` fence.
		input := "```sh\ntrackfw guard credential\n~~~\ntrackfw guard git-branch\n```\n"
		got := extractFencedGuardLines(input)
		want := []string{"trackfw guard credential", "trackfw guard git-branch"}
		if !equalStringSlices(got, want) {
			t.Errorf("B8b: got %v, want %v", got, want)
		}
	})

	t.Run("four_backtick_fence_not_closed_by_three_backticks", func(t *testing.T) {
		// A ```` fence requires ≥4 backticks to close; ``` does not close it.
		// Affirms: run-length requirement keeps the extractor inside the fence.
		input := "````sh\ntrackfw guard credential\n```\ntrackfw guard git-branch\n````\n"
		got := extractFencedGuardLines(input)
		want := []string{"trackfw guard credential", "trackfw guard git-branch"}
		if !equalStringSlices(got, want) {
			t.Errorf("4-tick: got %v, want %v", got, want)
		}
	})

	t.Run("indented_fence_up_to_3_spaces_is_valid", func(t *testing.T) {
		// A fence with exactly 3 spaces of indentation must be recognised.
		// Counter-arm: 4 spaces of indentation is a code block, not a fence.
		// Affirms: the extractor accepts 0-3 space indent as CommonMark requires.
		valid := "   ```sh\ntrackfw guard credential\n   ```\n"
		if got := extractFencedGuardLines(valid); !equalStringSlices(got, []string{"trackfw guard credential"}) {
			t.Errorf("3-space indent: got %v, want [trackfw guard credential]", got)
		}
		// 4-space indent: NOT a fence; the guard line is outside any block.
		tooDeep := "    ```sh\ntrackfw guard credential\n    ```\n"
		if got := extractFencedGuardLines(tooDeep); len(got) != 0 {
			t.Errorf("4-space indent should not open a fence, got %v", got)
		}
	})

	t.Run("closing_with_trailing_text_does_not_close", func(t *testing.T) {
		// ``` extra text is NOT a valid closing fence (trailing text after the run).
		// The line after it is still inside the fence.
		// Affirms: the extractor requires an empty remainder after the fence run.
		input := "```sh\ntrackfw guard credential\n``` extra text\ntrackfw guard git-branch\n```\n"
		got := extractFencedGuardLines(input)
		want := []string{"trackfw guard credential", "trackfw guard git-branch"}
		if !equalStringSlices(got, want) {
			t.Errorf("close-with-text: got %v, want %v", got, want)
		}
	})
}

// equalStringSlices reports whether a and b have the same length and elements.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ─── cli-parity.md PS/POSIX guard form checker (F-4) ────────────────────────
//
// Scope: PS/POSIX forms only (those containing both "trackfw guard" and
// "LASTEXITCODE" in the backtick span, with no "<" template marker).
//
// Limitation — cmd.exe forms excluded: bare forms like `trackfw guard credential`
// or `trackfw guard credential || exit 2` are byte-equal to LegacyD2CredentialCmdExe
// and LegacyD2GitBranchCmdExe, making them indistinguishable from subcommand
// mentions (see guard_generations.go:139-140 and cli-parity.md:9314). They are
// accepted without check and this gap is declared.
//
// Limitation — multi-line backtick spans: lines 5970-71 and 6566-67 in
// cli-parity.md contain "trackfw guard" text wrapped across lines; single-line
// scanning misses them. Neither contains a full PS/POSIX form (they reference
// the guard subcommand in prose). Declared as known gap.
//
// Historical criterion: a legacy PS/POSIX form is accepted only when its source
// line contains at least one of: "pre-ML-", "pre-REQ-", "pré-v8", "legad".
// These are the markers used in cli-parity.md to flag superseded generations.

// backtickSpanRe matches a single-backtick-enclosed span whose content
// contains no backticks. The full match includes the surrounding backticks.
var backtickSpanRe = regexp.MustCompile("`[^`]+`")

// cliParityForm holds an extracted PS/POSIX form and the line it came from
// (for historical-context analysis).
type cliParityForm struct {
	text    string // normalized form text (table \| → | already applied)
	srcLine string // original source line
}

// extractCLIParityGuardForms scans content for PS/POSIX guard hook forms:
//   - In fenced blocks: lines that contain "trackfw guard", "LASTEXITCODE",
//     and no "<" (template marker).
//   - In non-fenced lines: single-backtick spans meeting the same criteria.
//
// Table-cell pipe escapes (\|) are normalized to |.
func extractCLIParityGuardForms(content string) []cliParityForm {
	var result []cliParityForm
	inFence := false
	var fenceChar byte
	var fenceLen int

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimRight(rawLine, "\r")

		// Maintain fence state (same CommonMark logic as extractFencedGuardLines).
		ch, n, ok := fenceInfo(line)
		if ok {
			if !inFence {
				inFence = true
				fenceChar = ch
				fenceLen = n
				continue
			}
			if closesBlock(line, fenceChar, fenceLen) {
				inFence = false
				continue
			}
		}

		if inFence {
			// Whole line is the form inside the fence.
			if strings.Contains(line, "trackfw guard") &&
				strings.Contains(line, "LASTEXITCODE") &&
				!strings.Contains(line, "<") {
				result = append(result, cliParityForm{text: line, srcLine: line})
			}
		} else {
			// Non-fenced: scan backtick spans.
			if !strings.Contains(line, "trackfw guard") {
				continue
			}
			for _, span := range backtickSpanRe.FindAllString(line, -1) {
				inner := span[1 : len(span)-1] // strip surrounding backticks
				if strings.Contains(inner, "trackfw guard") &&
					strings.Contains(inner, "LASTEXITCODE") &&
					!strings.Contains(inner, "<") {
					normalized := strings.ReplaceAll(inner, `\|`, "|")
					result = append(result, cliParityForm{text: normalized, srcLine: line})
				}
			}
		}
	}
	return result
}

// buildLegacyPSPOSIXLookup returns a map from every inline PS/POSIX legacy form
// (those containing "LASTEXITCODE", excluding script-path forms) to its list name.
func buildLegacyPSPOSIXLookup() map[string]string {
	result := map[string]string{}
	for _, f := range CredentialLegacyPSPOSIX {
		if strings.Contains(f, "LASTEXITCODE") {
			result[f] = "CredentialLegacyPSPOSIX"
		}
	}
	for _, f := range CredentialGlobalLegacyPSPOSIX {
		if strings.Contains(f, "LASTEXITCODE") {
			result[f] = "CredentialGlobalLegacyPSPOSIX"
		}
	}
	for _, f := range GitBranchLegacyPSPOSIX {
		if strings.Contains(f, "LASTEXITCODE") {
			result[f] = "GitBranchLegacyPSPOSIX"
		}
	}
	return result
}

// isHistoricalLine reports whether the line carries a marker indicating that the
// guard form it contains is from a superseded generation.
// Markers derived from the actual cli-parity.md text (2026-10-29):
//   - "pre-ML-"   (e.g. "pre-ML-6B")
//   - "pre-REQ-"  (e.g. "pre-REQ-2026-10-20")
//   - "pré-v8"    (pre-v8 era references)
//   - "legad"     (matches "legada", "legado" in Portuguese)
func isHistoricalLine(line string) bool {
	return strings.Contains(line, "pre-ML-") ||
		strings.Contains(line, "pre-REQ-") ||
		strings.Contains(line, "pré-v8") ||
		strings.Contains(line, "legad")
}

// checkCLIParityGuardForms verifies that every PS/POSIX hook form in content is
// either an exact current constant or a legacy inline form whose source line
// carries a historical marker. Returns a slice of error strings (empty = pass).
func checkCLIParityGuardForms(content string) []string {
	currentForms := map[string]string{
		CredentialCurrentPSPOSIX:       "CredentialCurrentPSPOSIX",
		GitBranchCurrentPSPOSIX:        "GitBranchCurrentPSPOSIX",
		WriteScopeCurrentPSPOSIX:       "WriteScopeCurrentPSPOSIX",
		CredentialGlobalCurrentPSPOSIX: "CredentialGlobalCurrentPSPOSIX",
	}
	legacyPSPOSIX := buildLegacyPSPOSIXLookup()

	var errs []string
	for _, f := range extractCLIParityGuardForms(content) {
		if _, ok := currentForms[f.text]; ok {
			continue // recognized current form
		}
		if name, ok := legacyPSPOSIX[f.text]; ok {
			if isHistoricalLine(f.srcLine) {
				continue // legacy but explicitly marked as superseded
			}
			errs = append(errs, fmt.Sprintf(
				"cli-parity.md: legacy PS/POSIX form (%s) without historical context marker:\n  form:    %q\n  source:  %q",
				name, f.text, f.srcLine))
		} else {
			errs = append(errs, fmt.Sprintf(
				"cli-parity.md: unrecognized PS/POSIX guard form (not current, not in legacy registry):\n  form:    %q\n  source:  %q",
				f.text, f.srcLine))
		}
	}
	return errs
}

// ─── TestCLIParityHookLines ──────────────────────────────────────────────────

// TestCLIParityHookLines verifies that docs/cli-parity.md contains only
// current-canonical or historically-marked PS/POSIX hook forms (F-4, ML-4D).
//
// Sub-tests:
//   - real_file_green: the live file passes the check and has ≥3 forms (vacuity).
//   - sabotage_legacy_without_marker_reproves: replacing GitBranchCurrentPSPOSIX
//     with LegacyD11RevGitBranchPSPOSIX (no "emulate" prefix) on a non-historical
//     line causes checkCLIParityGuardForms to return at least one error.
//
// Affirms: a legacy PS/POSIX form inserted on a non-historical line of
// cli-parity.md is caught by the check (F-4, AC2, REQ-2026-10-28).
func TestCLIParityHookLines(t *testing.T) {
	t.Run("real_file_green", func(t *testing.T) {
		data, err := os.ReadFile("../../docs/cli-parity.md")
		if err != nil {
			t.Fatalf("cannot read ../../docs/cli-parity.md: %v", err)
		}
		content := string(data)

		errs := checkCLIParityGuardForms(content)
		for _, e := range errs {
			t.Error(e)
		}

		// Vacuity: at least the 3 known PS/POSIX forms (L6360, L6362, L9225).
		forms := extractCLIParityGuardForms(content)
		if len(forms) < 3 {
			t.Errorf("vacuity: expected ≥3 PS/POSIX hook forms in cli-parity.md, got %d — "+
				"the check may be scanning the wrong file or the extractor is broken", len(forms))
		}
	})

	t.Run("sabotage_legacy_without_marker_reproves", func(t *testing.T) {
		data, err := os.ReadFile("../../docs/cli-parity.md")
		if err != nil {
			t.Fatalf("cannot read ../../docs/cli-parity.md: %v", err)
		}
		original := string(data)

		// Replace GitBranchCurrentPSPOSIX (has "emulate") with the D11-revised
		// form (no "emulate" — the generation before REQ-2026-10-20), simulating
		// a regression on a non-historical line.
		modified := strings.Replace(original, GitBranchCurrentPSPOSIX, LegacyD11RevGitBranchPSPOSIX, 1)
		if modified == original {
			t.Fatal("sabotage: substitution had no effect — the test is vacuous; " +
				"verify that GitBranchCurrentPSPOSIX appears verbatim in cli-parity.md")
		}

		errs := checkCLIParityGuardForms(modified)
		if len(errs) == 0 {
			t.Error("sabotage: expected at least one error for a legacy form on a " +
				"non-historical line, got none — the check is not catching the regression")
		}
	})
}
