package validator_test

// Tests for the home_divergence_windows rule (D4, ADR-2026-10-14, ML-1B REQ-2026-09-01).
//
// Each test asserts a specific conclusion (Regra de Reconciliação, CLAUDE.md):
//
//   (a) Simulated divergence → ValidateHomeDivergenceWindowsForTest returns exactly
//       one warning that names HOME, osHome, and the three measurement-state strings.
//
//   (b) Control without divergence (HOME == USERPROFILE) → no warning returned.
//
//   (c) Outside Windows with HOME set → no warning returned.
//
//   (d) Case/trailing-slash difference only → EqualFold+normHomePath treats them
//       equal and returns no warning (A3 — MSYS2 Git Bash case).
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
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	. "github.com/kgsaran/trackfw/internal/validator"
)

// TestHomeDivergenceWindows_Divergent asserts conclusion (a): simulated Windows
// divergence produces exactly one warning naming both paths, Claude measured state,
// Windsurf not-measured state, and the inferred CLIs.
//
// Note: the warning uses %q to format paths, so "C:\Users\Lab\HOME" appears
// as "C:\\Users\\Lab\\HOME" (with escaped backslashes) inside the warning string.
// We compare using fmt.Sprintf("%q", path) to mirror the production format.
func TestHomeDivergenceWindows_Divergent(t *testing.T) {
	const homeEnv = `C:\Users\Lab\HOME`
	const osHomeEnv = `C:\Users\Lab\USERPROFILE`

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
	for _, want := range []string{
		fmt.Sprintf("%q", homeEnv),   // "C:\\Users\\Lab\\HOME"
		fmt.Sprintf("%q", osHomeEnv), // "C:\\Users\\Lab\\USERPROFILE"
		"Claude Code",
		"measured",
		"Windsurf",
		"not measured",
		"Codex",
		"Gemini",
		"Cursor",
		"Copilot",
		"Kiro",
	} {
		if !strings.Contains(w, want) {
			t.Errorf("warning missing %q\nfull warning: %s", want, w)
		}
	}
}

// TestHomeDivergenceWindows_NoDivergence asserts conclusion (b): when HOME equals
// USERPROFILE, ValidateHomeDivergenceWindowsForTest returns nil (no warning).
func TestHomeDivergenceWindows_NoDivergence(t *testing.T) {
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
