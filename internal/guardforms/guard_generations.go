// Package guardforms is a leaf package (no internal imports) that holds the
// single authoritative registry of every guard command generation ever emitted
// by trackfw (D12, ADR-2026-10-04 adendo 2026-10-23).
//
// Both internal/generators and internal/validator consume this registry.
// Generators iterate the legacy lists to migrate; validators iterate them to
// recognise equivalence-class members. A single list here means one place to
// update when a new generation is added, and one concordance test to catch the
// omission.
//
// Comparison semantics: every migration and every validator check uses exact
// equality on the full command string (never strings.Contains).
package guardforms

// ─── Current (canonical) forms ─────────────────────────────────────────────
//
// These are the exact strings emitted by current trackfw versions.
// (REQ-2026-10-20 / ADR-2026-10-04 D11 revised)

const (
	// CredentialCurrentPSPOSIX is the canonical credential guard command for the
	// POSIX/PS shell family (used by Claude, Codex, Gemini, Cursor, Windsurf, Copilot, Kiro).
	CredentialCurrentPSPOSIX = `emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	// CredentialCurrentCmdExe is the canonical credential guard command for the
	// cmd.exe shell family (used by Amazon Q, Kiro on Windows).
	CredentialCurrentCmdExe = "trackfw guard credential || exit 2"

	// GitBranchCurrentPSPOSIX is the canonical git-branch guard command for the
	// POSIX/PS shell family.
	GitBranchCurrentPSPOSIX = `emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	// GitBranchCurrentCmdExe is the canonical git-branch guard command for the
	// cmd.exe shell family.
	GitBranchCurrentCmdExe = "trackfw guard git-branch || exit 2"
)

// ─── Global (--global) current forms ───────────────────────────────────────

const (
	// CredentialGlobalCurrentPSPOSIX is the canonical global credential guard command.
	CredentialGlobalCurrentPSPOSIX = `emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	// CredentialGlobalCurrentCmdExe is the canonical global credential guard command for cmd.exe.
	CredentialGlobalCurrentCmdExe = "trackfw guard credential --global || exit 2"
)

// ─── Script-path legacy forms (family-independent) ─────────────────────────
//
// These are the paths that older trackfw versions wrote as the hook command
// before switching to inline subcommand invocations.

const (
	// LegacyBareCredGuard is the bare relative path used by all CLIs before any
	// env-var prefix was added.
	LegacyBareCredGuard = "scripts/trackfw-credential-guard.sh"

	// LegacyClaudeCredGuard is the CLAUDE_PROJECT_DIR-prefixed path.
	LegacyClaudeCredGuard = "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"

	// LegacyGeminiCredGuard is the GEMINI_PROJECT_DIR-prefixed path.
	LegacyGeminiCredGuard = `$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh`

	// LegacyCodexCredGuard is the git-rev-parse path used by Codex.
	// Note: codexRoot = `"$(git rev-parse --show-toplevel)`.
	LegacyCodexCredGuard = `"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"`

	// LegacyBareGitGuard is the bare relative path for the git-branch guard.
	LegacyBareGitGuard = "scripts/trackfw-git-branch-guard.sh"

	// LegacyBashGitGuard is the "bash <path>" form used by Windsurf.
	LegacyBashGitGuard = "bash scripts/trackfw-git-branch-guard.sh"

	// LegacyClaudeGitGuard is the CLAUDE_PROJECT_DIR-prefixed git-branch guard path.
	LegacyClaudeGitGuard = "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"

	// LegacyGeminiGitGuard is the GEMINI_PROJECT_DIR-prefixed git-branch guard path.
	LegacyGeminiGitGuard = `$GEMINI_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`

	// LegacyCodexGitGuard is the git-rev-parse path used by Codex for git-branch guard.
	LegacyCodexGitGuard = `"$(git rev-parse --show-toplevel)/scripts/trackfw-git-branch-guard.sh"`
)

// ─── Inline legacy forms — POSIX/PS family ─────────────────────────────────

const (
	// LegacyD2CredentialPSPOSIX is the D2 inline credential guard form (pre-ML-6B).
	LegacyD2CredentialPSPOSIX = "trackfw guard credential; exit $LASTEXITCODE"

	// LegacyD2CredentialGlobalPSPOSIX is the global variant of the D2 form.
	LegacyD2CredentialGlobalPSPOSIX = "trackfw guard credential --global; exit $LASTEXITCODE"

	// LegacyD11CredentialPSPOSIX is the D11 inline form (pre-ML-6C, without 4th PS trecho).
	LegacyD11CredentialPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`

	// LegacyD11CredentialGlobalPSPOSIX is the global variant of D11.
	LegacyD11CredentialGlobalPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`

	// LegacyD11RevCredentialPSPOSIX is the D11 revised form (with 4th PS trecho but without
	// emulate prefix). Aborts under zsh due to EQUALS/NOMATCH expansion.
	LegacyD11RevCredentialPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	// LegacyD11RevCredentialGlobalPSPOSIX is the global variant of D11 revised.
	LegacyD11RevCredentialGlobalPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	// LegacyWABashCredentialPSPOSIX is the workaround /bin/bash -c wrapper form.
	LegacyWABashCredentialPSPOSIX = `/bin/bash -c '$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'`

	// LegacyWABashCredentialGlobalPSPOSIX is the global variant of the WABash form.
	LegacyWABashCredentialGlobalPSPOSIX = `/bin/bash -c '$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'`

	// LegacyD2GitBranchPSPOSIX is the D2 git-branch guard form.
	LegacyD2GitBranchPSPOSIX = "trackfw guard git-branch; exit $LASTEXITCODE"

	// LegacyD11GitBranchPSPOSIX is the D11 git-branch guard form (without 4th trecho).
	LegacyD11GitBranchPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`

	// LegacyD11RevGitBranchPSPOSIX is the D11 revised git-branch guard form (without emulate).
	LegacyD11RevGitBranchPSPOSIX = `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

	// LegacyWABashGitBranchPSPOSIX is the workaround /bin/bash -c wrapper form for git-branch.
	LegacyWABashGitBranchPSPOSIX = `/bin/bash -c '$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'`
)

// ─── Inline legacy forms — cmd.exe family ──────────────────────────────────

const (
	// LegacyD2CredentialCmdExe is the D2 credential guard form for cmd.exe.
	// IMPORTANT: this is identical to a user typing `trackfw guard credential` manually,
	// so removal is a documented residual (R3 in Wave 0 §4). See AC5.
	LegacyD2CredentialCmdExe = "trackfw guard credential"

	// LegacyD2CredentialGlobalCmdExe is the global D2 form for cmd.exe.
	LegacyD2CredentialGlobalCmdExe = "trackfw guard credential --global"

	// LegacyD2GitBranchCmdExe is the D2 git-branch guard form for cmd.exe.
	LegacyD2GitBranchCmdExe = "trackfw guard git-branch"
)

// ─── Lists — the actual D12 registry ───────────────────────────────────────
//
// These slices contain ALL known legacy forms (never the current form).
// Consumers iterate these to migrate or recognise. Comparison is always exact.
//
// NOTE: the order within each slice is immaterial for correctness, but the
// path-based forms come first (longer strings, less likely to be substrings of
// something else) followed by the inline forms in chronological order.

// CredentialLegacyPSPOSIX is the complete list of all non-global POSIX/PS
// legacy forms for credential guard, in all known path and inline variants.
var CredentialLegacyPSPOSIX = []string{
	LegacyBareCredGuard,
	LegacyClaudeCredGuard,
	LegacyGeminiCredGuard,
	LegacyCodexCredGuard,
	LegacyD2CredentialPSPOSIX,
	LegacyD11CredentialPSPOSIX,
	LegacyD11RevCredentialPSPOSIX,
	LegacyWABashCredentialPSPOSIX,
}

// CredentialGlobalLegacyPSPOSIX is the complete list of all global POSIX/PS
// legacy forms for credential guard (--global variants).
var CredentialGlobalLegacyPSPOSIX = []string{
	LegacyD2CredentialGlobalPSPOSIX,
	LegacyD11CredentialGlobalPSPOSIX,
	LegacyD11RevCredentialGlobalPSPOSIX,
	LegacyWABashCredentialGlobalPSPOSIX,
}

// CredentialLegacyCmdExe is the complete list of all non-global cmd.exe
// legacy forms for credential guard.
var CredentialLegacyCmdExe = []string{
	LegacyD2CredentialCmdExe,
}

// GitBranchLegacyPSPOSIX is the complete list of all POSIX/PS legacy forms
// for git-branch guard.
var GitBranchLegacyPSPOSIX = []string{
	LegacyBareGitGuard,
	LegacyBashGitGuard,
	LegacyClaudeGitGuard,
	LegacyGeminiGitGuard,
	LegacyCodexGitGuard,
	LegacyD2GitBranchPSPOSIX,
	LegacyD11GitBranchPSPOSIX,
	LegacyD11RevGitBranchPSPOSIX,
	LegacyWABashGitBranchPSPOSIX,
}

// GitBranchLegacyCmdExe is the complete list of all cmd.exe legacy forms
// for git-branch guard.
var GitBranchLegacyCmdExe = []string{
	LegacyD2GitBranchCmdExe,
}

// CredentialScriptPathForms returns the subset of CredentialLegacyPSPOSIX that are
// path-based (family-independent). These are the forms the validator recognises via
// guardWiringScriptPathForms and are distinct from the inline family-dependent forms.
func CredentialScriptPathForms() []string {
	return []string{
		LegacyBareCredGuard,
		LegacyClaudeCredGuard,
		LegacyGeminiCredGuard,
		LegacyCodexCredGuard,
	}
}

// GitBranchScriptPathForms returns the path-based legacy git-branch guard forms.
func GitBranchScriptPathForms() []string {
	return []string{
		LegacyBareGitGuard,
		LegacyBashGitGuard,
		LegacyClaudeGitGuard,
		LegacyGeminiGitGuard,
		LegacyCodexGitGuard,
	}
}
