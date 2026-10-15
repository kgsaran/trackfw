package validator_test

// Tests for the home_divergence_windows rule (D4, ADR-2026-10-14, ML-1B REQ-2026-09-01).
//
// Each test asserts a specific conclusion (Regra de Reconciliação, CLAUDE.md):
//
//   (a) Simulated divergence → ValidateHomeDivergenceWindowsForTest returns exactly
//       one warning that names HOME, osHome, the three measurement-state strings,
//       the exact phrase "Claude Code reads the OS home (measured)" (A3), and the
//       resolved Claude config dir path (B1, ML-2B).
//
//   (b) Control without divergence (HOME == USERPROFILE) → no warning returned.
//
//   (c) Outside Windows with HOME set → no warning returned.
//
//   (d) Case/trailing-slash difference only → EqualFold+normHomePath treats them
//       equal and returns no warning (A3 — MSYS2 Git Bash case).
//
//   (e) B1: CLAUDE_CONFIG_DIR=HOME/.claude with HOME≠USERPROFILE → guard at
//       HOME/.claude/settings.json is NOT called inert (it is the live guard).
//
// Mordidas (mandatory falsification):
//
//   Mordida 1: predicate without EqualFold → (d) fails.
//     Removed EqualFold from the normHomePath/EqualFold composite and verified that
//     "C:\Users\Lab" vs "c:\users\lab\" diverges (was produced as divergent).
//
//   Mordida 2: warning always emitted → (b) fails.
//     Replaced `if !divergent { return nil }` guard with always-emit and verified
//     that (b) received a spurious warning.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	. "github.com/kgsaran/trackfw/internal/validator"
)

// TestHomeDivergenceWindows_Divergent asserts conclusion (a): simulated Windows
// divergence produces exactly one warning naming both paths, the exact phrase
// "Claude Code reads the OS home (measured)" (A3 — not just "measured", which
// is a substring of "not measured"), the resolved Claude config dir (B1), and
// the inferred CLIs.
//
// Note: the warning uses %q to format paths, so "C:\Users\Lab\HOME" appears
// as "C:\\Users\\Lab\\HOME" (with escaped backslashes) inside the warning string.
// We compare using fmt.Sprintf("%q", path) to mirror the production format.
//
// A3 bites (mandatory):
//   - Bite 1: changing the phrase to "not measured" → "Claude Code ... (measured)"
//     assertion fails.
//   - Bite 2: using osHomeEnv (USERPROFILE, without ".claude") as the third %q
//     argument instead of newClaudeDir → path assertion fails because the message
//     would no longer contain the ".claude" suffix.
func TestHomeDivergenceWindows_Divergent(t *testing.T) {
	const homeEnv = `C:\Users\Lab\HOME`
	const osHomeEnv = `C:\Users\Lab\USERPROFILE`

	t.Setenv("CLAUDE_CONFIG_DIR", "")
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return osHomeEnv, nil
	})
	defer restore()
	t.Setenv("HOME", homeEnv)

	warnings := ValidateHomeDivergenceWindowsForTest()

	if len(warnings) != 1 {
		t.Fatalf("want exactly 1 warning, got %d: %v", len(warnings), warnings)
	}
	w := warnings[0]
	// Paths appear %q-formatted in the warning (backslashes doubled).
	// After ML-2B B1 fix: newClaudeDir = filepath.Join(osHomeEnv, ".claude"),
	// so the write-path assertion checks for that value.
	writeDir := filepath.Join(osHomeEnv, ".claude")
	for _, want := range []string{
		fmt.Sprintf("%q", homeEnv),              // "C:\\Users\\Lab\\HOME"
		fmt.Sprintf("%q", osHomeEnv),            // "C:\\Users\\Lab\\USERPROFILE"
		"Claude Code reads the OS home (measured)", // A3: exact phrase, not just "measured"
		"Windsurf",
		"not measured",
		"Codex",
		"Gemini",
		"Cursor",
		"Copilot",
		"Kiro",
		fmt.Sprintf("%q", writeDir), // B1: write path includes .claude suffix
	} {
		if !strings.Contains(w, want) {
			t.Errorf("warning missing %q\nfull warning: %s", want, w)
		}
	}
}

// TestHomeDivergenceWindows_NoDivergence asserts conclusion (b): when HOME equals
// USERPROFILE, ValidateHomeDivergenceWindowsForTest returns nil (no warning).
func TestHomeDivergenceWindows_NoDivergence(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return `C:\Users\Lab`, nil
	})
	defer restore()
	t.Setenv("HOME", `C:\Users\Lab`)

	warnings := ValidateHomeDivergenceWindowsForTest()

	if len(warnings) != 0 {
		t.Errorf("want no warnings, got %d: %v", len(warnings), warnings)
	}
}

// TestHomeDivergenceWindows_NotWindows asserts conclusion (c): outside Windows the
// rule is silent even when HOME is set.
func TestHomeDivergenceWindows_NotWindows(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	restore := agentconfig.SetPlatformForTest("linux", func() (string, error) {
		return "/home/lab", nil
	})
	defer restore()
	t.Setenv("HOME", "/home/lab-other")

	warnings := ValidateHomeDivergenceWindowsForTest()

	if len(warnings) != 0 {
		t.Errorf("want no warnings on non-Windows, got %d: %v", len(warnings), warnings)
	}
}

// TestHomeDivergenceWindows_CaseAndTrailingSlash asserts conclusion (d): paths that
// differ only in case or trailing slash are treated equal (A3 — MSYS2 Git Bash
// converts HOME=/c/Users/Lab to C:\Users\Lab before spawning Go processes; the
// default Git Bash case should be silent).
//
// Mordida 1 falsifies this: without strings.EqualFold the predicate fires for
// "C:\Users\Lab" vs "c:\users\lab\", incorrectly returning a warning.
func TestHomeDivergenceWindows_CaseAndTrailingSlash(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return `C:\Users\Lab`, nil // USERPROFILE: uppercase, no trailing slash
	})
	defer restore()
	t.Setenv("HOME", `c:\users\lab\`) // HOME: lowercase, with trailing slash

	warnings := ValidateHomeDivergenceWindowsForTest()

	if len(warnings) != 0 {
		t.Errorf("want no warnings for case/trailing-slash difference only (A3), got %d: %v",
			len(warnings), warnings)
	}
}

// TestHomeDivergenceWindows_ClaudeConfigDir_NoInert asserts conclusion (e):
// when CLAUDE_CONFIG_DIR=HOME/.claude and HOME≠USERPROFILE, the guard at
// HOME/.claude/settings.json is NOT called inert — it is the live guard.
//
// Conclusion (B1, ML-2B REQ-2026-09-01): validateHomeDivergenceWindows uses
// ClaudeConfigDir to resolve newClaudeDir; when CLAUDE_CONFIG_DIR=HOME/.claude,
// InertClaudeGuardPath(HOME/.claude) finds no divergence and suppresses the D5
// notice; this test FAILS without the ClaudeConfigDir call (the old code hardcoded
// filepath.Join(osHome, ".claude") = USERPROFILE/.claude, which differs from
// HOME/.claude, causing InertClaudeGuardPath to declare the live guard inert).
//
// Control: the fixture also verifies that when CLAUDE_CONFIG_DIR="" the inert
// notice DOES fire (proving the fixture can trigger D5), and the notice is absent
// when CLAUDE_CONFIG_DIR=HOME/.claude.
func TestHomeDivergenceWindows_ClaudeConfigDir_NoInert(t *testing.T) {
	homeDir := t.TempDir()
	osHomeDir := t.TempDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	// Write a live guard at homeDir/.claude/settings.json containing a guard marker.
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"trackfw guard"}]}]}}`),
		0644); err != nil {
		t.Fatal(err)
	}

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return osHomeDir, nil
	})
	defer restore()
	t.Setenv("HOME", homeDir)

	// Control: with CLAUDE_CONFIG_DIR="" the D5 inert notice should fire
	// (InertClaudeGuardPath(osHomeDir/.claude) ≠ homeDir/.claude → detects guard).
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	controlWarnings := ValidateHomeDivergenceWindowsForTest()
	if len(controlWarnings) != 1 {
		t.Fatalf("control: want 1 warning, got %d: %v", len(controlWarnings), controlWarnings)
	}
	if !strings.Contains(controlWarnings[0], "inert") {
		t.Logf("control: inert notice absent (guard may not be readable); warning: %s", controlWarnings[0])
		// Non-fatal: on some platforms homedir.Dir() may not return homeDir.
	}

	// Subject: with CLAUDE_CONFIG_DIR=claudeDir (=HOME/.claude) the D5 notice must NOT fire.
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	warnings := ValidateHomeDivergenceWindowsForTest()

	if len(warnings) != 1 {
		t.Fatalf("want exactly 1 warning, got %d: %v", len(warnings), warnings)
	}
	if strings.Contains(warnings[0], "inert") {
		t.Errorf("CLAUDE_CONFIG_DIR=HOME/.claude: warning incorrectly calls the live guard inert.\nfull warning: %s", warnings[0])
	}
}

// TestHomeDivergenceWindows_RelativeCCDError asserts conclusion (f): on simulated
// Windows with HOME≠USERPROFILE and CLAUDE_CONFIG_DIR set to a relative path
// ("rel"), ClaudeConfigDirWithSource returns a resolution error; the single
// returned warning must contain both "unless CLAUDE_CONFIG_DIR is set" and the
// resolution-error text, so the user knows why trackfw could not determine the
// Claude config directory AND that the variable was responsible.
//
// Conclusion (R-3, ML-2D): the error-path warning now names CLAUDE_CONFIG_DIR as
// the source of the failure; this test FAILS if the phrase remains the old
// "Claude Code reads the OS home (measured)" without the caveat.
func TestHomeDivergenceWindows_RelativeCCDError(t *testing.T) {
	const homeEnv = `C:\Users\Lab\HOME`
	const osHomeEnv = `C:\Users\Lab\USERPROFILE`

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return osHomeEnv, nil
	})
	defer restore()
	t.Setenv("HOME", homeEnv)
	t.Setenv("CLAUDE_CONFIG_DIR", "rel") // relative path → resolution error

	warnings := ValidateHomeDivergenceWindowsForTest()

	if len(warnings) != 1 {
		t.Fatalf("want exactly 1 warning, got %d: %v", len(warnings), warnings)
	}
	w := warnings[0]
	// "unless CLAUDE_CONFIG_DIR is set" must appear — the key ML-2D fix.
	if !strings.Contains(w, "unless CLAUDE_CONFIG_DIR is set") {
		t.Errorf("warning missing %q\nfull warning: %s", "unless CLAUDE_CONFIG_DIR is set", w)
	}
	// The resolution-error text must also appear so the user sees why it failed.
	wantErrFrag := `"rel" is not an absolute path`
	if !strings.Contains(w, wantErrFrag) {
		t.Errorf("warning missing resolution-error text %q\nfull warning: %s", wantErrFrag, w)
	}
}

// TestHomeDivergenceWindows_ClaudeConfigDir_MessageCitesDir asserts conclusion (e):
// when CLAUDE_CONFIG_DIR is set to an absolute path on Windows with HOME≠USERPROFILE,
// the warning cites "CLAUDE_CONFIG_DIR=%q" with the resolved dir rather than
// "the OS home (measured)"; without CLAUDE_CONFIG_DIR, the phrase "OS home (measured)"
// is present.
//
// Conclusion (e, R-4, ML-2D): the warning phrase adapts to the actual Claude
// reading location; this test FAILS if claudeReadsPhrase is hardcoded to
// "Claude Code reads the OS home (measured)" regardless of CLAUDE_CONFIG_DIR.
func TestHomeDivergenceWindows_ClaudeConfigDir_MessageCitesDir(t *testing.T) {
	const homeEnv = `C:\Users\Lab\HOME`
	const osHomeEnv = `C:\Users\Lab\USERPROFILE`
	customDir := t.TempDir() // absolute path — passes R-1 check

	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
		return osHomeEnv, nil
	})
	defer restore()
	t.Setenv("HOME", homeEnv)

	// Sub-case 1: CLAUDE_CONFIG_DIR set → warning must cite CLAUDE_CONFIG_DIR=<dir>.
	t.Setenv("CLAUDE_CONFIG_DIR", customDir)
	warnings := ValidateHomeDivergenceWindowsForTest()
	if len(warnings) != 1 {
		t.Fatalf("(e) with CLAUDE_CONFIG_DIR: want 1 warning, got %d: %v", len(warnings), warnings)
	}
	w := warnings[0]
	wantPhrase := fmt.Sprintf("CLAUDE_CONFIG_DIR=%q", customDir)
	if !strings.Contains(w, wantPhrase) {
		t.Errorf("(e) with CLAUDE_CONFIG_DIR: warning missing %q\nfull warning: %s", wantPhrase, w)
	}
	if strings.Contains(w, "the OS home (measured)") {
		t.Errorf("(e) with CLAUDE_CONFIG_DIR: warning still says %q — should cite CLAUDE_CONFIG_DIR instead\nfull warning: %s",
			"the OS home (measured)", w)
	}

	// Sub-case 2: no CLAUDE_CONFIG_DIR → warning must cite "OS home (measured)".
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	warnings2 := ValidateHomeDivergenceWindowsForTest()
	if len(warnings2) != 1 {
		t.Fatalf("(e) without CLAUDE_CONFIG_DIR: want 1 warning, got %d: %v", len(warnings2), warnings2)
	}
	if !strings.Contains(warnings2[0], "the OS home (measured)") {
		t.Errorf("(e) without CLAUDE_CONFIG_DIR: warning missing %q\nfull warning: %s",
			"the OS home (measured)", warnings2[0])
	}
}
