package validator

import (
	"fmt"
	"path/filepath"

	"github.com/kgsaran/trackfw/internal/agentconfig"
)

// validateHomeDivergenceWindows is the "home_divergence_windows" rule (D4, ADR-2026-10-14).
//
// Always-warning; must be routed via applyRuleWarnOnly — never promotable to violation.
//
// Fires only on Windows (real or via agentconfig.SetPlatformForTest) when HOME is
// non-empty and differs from os.UserHomeDir() (USERPROFILE) after
// normHomePath + strings.EqualFold (A3: MSYS2 already converts HOME POSIX form
// to Windows form, so the default Git Bash case is silent; differences only in
// case or trailing slashes are also ignored — see Seção 3.5, Wave 0 2026-10-14).
//
// The single returned warning (D4 says one informative warning, never two):
//   - names HOME and os.UserHomeDir() (USERPROFILE);
//   - names Claude Code as measured (Passo 3a, Windows ARM64);
//   - names the inferred CLIs (Codex, Gemini, Cursor, Copilot, Kiro) as continuing
//     under HOME until measured (D2, ADR-2026-10-14);
//   - names Windsurf as not measured (A8): writes under HOME, audits under OS home;
//   - includes the D5 inert-remainder notice inline when HOME/.claude/settings.json
//     exists and contains trackfw guard wiring.
//
// Without divergence, or outside Windows: returns nil (silence).
//
// Regra de reconciliação (CLAUDE.md): this rule fires when and only when
// HOME and os.UserHomeDir() diverge on Windows; existing tests that set
// HOME == USERPROFILE and run on macOS will never trigger it.
func validateHomeDivergenceWindows() []string {
	homeEnv, osHome, divergent := agentconfig.HomeDivergencePaths()
	if !divergent {
		return nil
	}

	// D5: check for inert remainder at HOME/.claude/settings.json.
	// InertClaudeGuardPath(claudeDir) uses homedir.Dir() (= HOME in our environment)
	// internally to locate the old file. Passing the NEW claudeDir (osHome/.claude)
	// makes it compare against HOME/.claude and detect the remainder.
	d5 := ""
	newClaudeDir := filepath.Join(osHome, ".claude")
	if inertPath, present := agentconfig.InertClaudeGuardPath(newClaudeDir); present {
		d5 = fmt.Sprintf(
			" Old guard at %q is inert — Claude Code does not read this file; remove it by hand if you want.",
			inertPath,
		)
	}

	msg := fmt.Sprintf(
		"HOME=%q differs from the OS home (USERPROFILE) %q — "+
			"Claude Code reads the OS home (measured); "+
			"trackfw writes and audits the Claude guard at %q. "+
			"Inferred CLIs (Codex, Gemini, Cursor, Copilot, Kiro) stay under HOME until measured. "+
			"Windsurf: not measured — written under HOME, audited under the OS home."+
			"%s",
		homeEnv, osHome, osHome, d5,
	)
	return []string{msg}
}
