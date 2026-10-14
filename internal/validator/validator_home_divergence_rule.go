package validator

import (
	"fmt"

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
//     exists and contains trackfw guard wiring AND the resolved Claude dir differs
//     from HOME/.claude.
//
// Without divergence, or outside Windows: returns nil (silence).
//
// ML-2B (REQ-2026-09-01): the resolved Claude config dir is obtained via
// agentconfig.ClaudeConfigDir so CLAUDE_CONFIG_DIR is honoured (B1). When
// CLAUDE_CONFIG_DIR points at HOME/.claude, InertClaudeGuardPath correctly
// detects no divergence and suppresses the D5 notice — preventing the guard
// from being labelled "inert" when it is in fact the live protection (ADR A7).
//
// Regra de reconciliação (CLAUDE.md): this rule fires when and only when
// HOME and os.UserHomeDir() diverge on Windows; existing tests that set
// HOME == USERPROFILE and run on macOS will never trigger it.
func validateHomeDivergenceWindows() []string {
	homeEnv, osHome, divergent := agentconfig.HomeDivergencePaths()
	if !divergent {
		return nil
	}

	// ML-2B (B1): use ClaudeConfigDirWithSource so CLAUDE_CONFIG_DIR is honoured (A1).
	// R-4 (ML-2D): source is used to cite what Claude Code actually reads in the warning.
	// On resolution failure emit A7 finding and skip D5 — never label anything
	// inert without a known destination (ADR-2026-10-14 A7).
	newClaudeDir, claudeDirSource, claudeDirErr := agentconfig.ClaudeConfigDirWithSource(agentconfig.DefaultResolver)
	if claudeDirErr != nil {
		return []string{fmt.Sprintf(
			"HOME=%q differs from the OS home (USERPROFILE) %q — "+
				"Claude Code reads the OS home (measured) unless CLAUDE_CONFIG_DIR is set; "+
				"trackfw cannot determine the Claude config directory: %s. "+
				"Inferred CLIs (Codex, Gemini, Cursor, Copilot, Kiro) stay under HOME until measured. "+
				"Windsurf: not measured — written under HOME, audited under the OS home.",
			homeEnv, osHome, claudeDirErr,
		)}
	}

	// D5: check for inert remainder at HOME/.claude/settings.json.
	// InertClaudeGuardPath(newClaudeDir) uses homedir.Dir() (= HOME) internally to
	// locate the old file; it returns false when newClaudeDir IS HOME/.claude, so
	// with CLAUDE_CONFIG_DIR=HOME/.claude no spurious "inert" notice is emitted.
	// Note: validateHomeDivergenceWindows only runs when divergent==true (Windows,
	// HOME≠USERPROFILE). In that context HOME/.claude is unconditionally inert
	// regardless of CLAUDE_CONFIG_DIR, so the "remove it by hand" message is correct.
	d5 := ""
	if inertPath, present := agentconfig.InertClaudeGuardPath(newClaudeDir); present {
		d5 = fmt.Sprintf(
			" Old guard at %q is inert — Claude Code does not read this file; remove it by hand if you want.",
			inertPath,
		)
	}

	// R-4 (ML-2D): cite what Claude Code actually reads.
	// With CLAUDE_CONFIG_DIR: it reads that directory, not the OS home.
	// Without CLAUDE_CONFIG_DIR: it reads the OS home (USERPROFILE, measured 2026-10-14).
	var claudeReadsPhrase string
	if claudeDirSource == agentconfig.ClaudeConfigDirSourceEnvVar {
		claudeReadsPhrase = fmt.Sprintf("Claude Code reads CLAUDE_CONFIG_DIR=%q", newClaudeDir)
	} else {
		claudeReadsPhrase = "Claude Code reads the OS home (measured)"
	}

	msg := fmt.Sprintf(
		"HOME=%q differs from the OS home (USERPROFILE) %q — "+
			"%s; "+
			"trackfw writes and audits the Claude guard at %q. "+
			"Inferred CLIs (Codex, Gemini, Cursor, Copilot, Kiro) stay under HOME until measured. "+
			"Windsurf: not measured — written under HOME, audited under the OS home."+
			"%s",
		homeEnv, osHome, claudeReadsPhrase, newClaudeDir, d5,
	)
	return []string{msg}
}
