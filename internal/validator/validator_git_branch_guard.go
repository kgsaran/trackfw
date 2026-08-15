package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
// ML-1A: this file adds git-branch-guard coverage to the two existing credential-guard checks
// (existence/executability via validateGuardHookResolvable in validator_credential_guard.go, and
// content-drift integrity via validateCredentialGuardScriptIntegrity in
// validator_credential_guard_integrity.go), plus the GLOBAL-scope check that was missing for
// BOTH guards before this ML (REQ's main gap).
//
// git_branch_guard_hook_resolvable reuses validateGuardHookResolvable — generalized in
// validator_credential_guard.go, see validateGitBranchGuardHookResolvable there.

// validateGitBranchGuardScriptIntegrity is the "git_branch_guard_script_integrity" rule: compares
// the on-disk scripts/trackfw-git-branch-guard.sh against the template this trackfw binary would
// generate. Mirrors validateCredentialGuardScriptIntegrity (validator_credential_guard_integrity.go)
// exactly — same silent-on-absence contract (existence is credential_guard_hook_resolvable's /
// git_branch_guard_hook_resolvable's job, not this rule's).
func validateGitBranchGuardScriptIntegrity() ([]string, error) {
	const relPath = "scripts/trackfw-git-branch-guard.sh"

	content, err := os.ReadFile(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("git_branch_guard_script_integrity: reading %s: %w", relPath, err)
	}

	if string(content) == gitBranchGuardScriptReference {
		return nil, nil
	}

	return []string{fmt.Sprintf(
		"%s content diverges from the template this version of trackfw generates — "+
			"if you did not edit this file by hand, run `trackfw update` to regenerate it",
		relPath,
	)}, nil
}

// globalGuardConfigFile associates a GLOBAL (per-CLI, $HOME-rooted) hook/settings config file with
// the CLI that consumes it, for the global-scope guard checks below. Distinct from
// credentialGuardHookFile (validator_credential_guard.go), whose .path is rooted at the PROJECT
// root, not $HOME.
type globalGuardConfigFile struct {
	path string // relative to $HOME
	cli  string
}

// globalGuardConfigFiles is the closed list of GLOBAL hook/settings files `trackfw update harness`
// can write a guard entry into — the global-scope counterpart of credentialGuardHookFiles
// (validator_credential_guard.go). Paths and CLI labels taken from
// globalCredentialGuardInstalled{Claude,Codex,Gemini,Cursor,Copilot,Kiro} in
// internal/generators/agentfiles.go (validator cannot import that package — see the import-cycle
// note on credentialGuardScriptReference — so this list is validator's own, independently
// maintained copy of the same 6 paths).
var globalGuardConfigFiles = []globalGuardConfigFile{
	{".claude/settings.json", "Claude Code"},
	{".codex/hooks.json", "Codex CLI"},
	{".gemini/settings.json", "Gemini CLI"},
	{".cursor/hooks.json", "Cursor"},
	{".copilot/settings.json", "GitHub Copilot CLI"},
	{".kiro/hooks/trackfw-credential-guard.json", "Kiro"},
}

// validateGuardGlobalHookResolvable is the GLOBAL-scope counterpart of validateGuardHookResolvable
// (validator_credential_guard.go): for each of the 6 globalGuardConfigFiles that exists AND
// references scriptMarker, verifies the referenced script exists and is executable.
//
// This is the gap the REQ (docs/req/REQ-2026-08-15-trackfw-validate-deve-detectar-scripts-de-
// hook-ausentes-ou-desatualizados.md, "Achado adicional") identifies as the main one: before this
// ML, nothing in `trackfw validate` ever inspected these 6 files — the dedup functions in
// internal/generators/agentfiles.go only decide whether to SKIP writing a project-scope entry,
// they never confirm the global target they're deferring to actually exists.
//
// Trigger condition (advisor-reviewed, see ML-1A commit): "a global CLI config file contains a
// string referencing scriptMarker" — this single condition covers BOTH REQ disjuncts (b): "projeto
// delega pro global" (which never shows up in the PROJECT file — dedup means the project file has
// NO entry at all — so it can only ever be observed by finding the entry in the GLOBAL file
// instead) and "global está registrado em algum arquivo de config do CLI" (the literal condition
// this function checks). No project-side signal is needed or consulted here.
//
// Global entries are written by internal/generators (harnessCredentialGuardTarget*, agentfiles.go)
// as fully resolved absolute paths (via globalCredentialGuardScriptPath — filepath.Join(home,
// ".trackfw","scripts", name)), never a placeholder like $CLAUDE_PROJECT_DIR — so, unlike the
// project-scope resolveCredentialGuardHookPath, no prefix-stripping is needed here: any matched
// command that is NOT already an absolute path is not a form trackfw itself ever emits and is
// skipped (never treated as a violation — same "not our wiring, not our job" contract
// resolveCredentialGuardHookPath's ok=false branch already documents).
//
// Fail-open: unresolvable $HOME, unreadable file, or invalid JSON all skip that file in silence —
// same contract validateGuardHookResolvable already has for project-scope files.
func validateGuardGlobalHookResolvable(ruleName, scriptMarker string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, nil
	}

	var msgs []string
	for _, gf := range globalGuardConfigFiles {
		fullPath := filepath.Join(home, gf.path)
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("%s: lendo %s: %w", ruleName, filepath.Join("~", gf.path), readErr)
		}

		var parsed interface{}
		if jsonErr := json.Unmarshal(content, &parsed); jsonErr != nil {
			continue
		}

		var commands []string
		collectCommandsWithMarker(parsed, scriptMarker, &commands)

		seen := make(map[string]bool, len(commands))
		for _, raw := range commands {
			if seen[raw] {
				continue
			}
			seen[raw] = true

			if !filepath.IsAbs(raw) {
				continue
			}

			info, statErr := os.Stat(raw)
			switch {
			case statErr != nil:
				msgs = append(msgs, fmt.Sprintf(
					"~/%s (%s, global scope) references %s resolved to %q, but the script does not exist — run `trackfw update harness` to regenerate it",
					gf.path, gf.cli, scriptMarker, raw,
				))
			case info.Mode()&0111 == 0:
				msgs = append(msgs, fmt.Sprintf(
					"~/%s (%s, global scope) references %s resolved to %q, but the script is not executable — run `trackfw update harness` to regenerate it",
					gf.path, gf.cli, scriptMarker, raw,
				))
			}
		}
	}

	return msgs, nil
}

// validateGuardGlobalScriptIntegrity is the GLOBAL-scope counterpart of
// validateCredentialGuardScriptIntegrity/validateGitBranchGuardScriptIntegrity: for each of the 6
// globalGuardConfigFiles that references scriptMarker, verifies the referenced script's content
// matches referenceContent byte-for-byte. Same trigger condition and fail-open contract as
// validateGuardGlobalHookResolvable above (see its doc comment) — this function does not
// re-validate existence/executability, only content, matching the project-scope split between
// *_hook_resolvable and *_script_integrity.
func validateGuardGlobalScriptIntegrity(ruleName, scriptMarker, referenceContent string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, nil
	}

	var msgs []string
	for _, gf := range globalGuardConfigFiles {
		fullPath := filepath.Join(home, gf.path)
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("%s: lendo %s: %w", ruleName, filepath.Join("~", gf.path), readErr)
		}

		var parsed interface{}
		if jsonErr := json.Unmarshal(content, &parsed); jsonErr != nil {
			continue
		}

		var commands []string
		collectCommandsWithMarker(parsed, scriptMarker, &commands)

		seen := make(map[string]bool, len(commands))
		for _, raw := range commands {
			if seen[raw] {
				continue
			}
			seen[raw] = true

			if !filepath.IsAbs(raw) {
				continue
			}

			scriptContent, readScriptErr := os.ReadFile(raw)
			if readScriptErr != nil {
				// Existence/readability is validateGuardGlobalHookResolvable's job — do not
				// double-report the same underlying condition under two rule names.
				continue
			}

			if string(scriptContent) == referenceContent {
				continue
			}

			msgs = append(msgs, fmt.Sprintf(
				"%s (global scope, referenced from ~/%s, %s) content diverges from the template "+
					"this version of trackfw generates — if you did not edit this file by hand, "+
					"run `trackfw update harness` to regenerate it",
				raw, gf.path, gf.cli,
			))
		}
	}

	return msgs, nil
}

// validateCredentialGuardGlobalHookResolvable / validateCredentialGuardGlobalScriptIntegrity /
// validateGitBranchGuardGlobalHookResolvable / validateGitBranchGuardGlobalScriptIntegrity are the
// 4 thin wrappers registered in validator.go — each folds its messages into the SAME rule name as
// its project-scope counterpart (credential_guard_hook_resolvable, credential_guard_script_integrity,
// git_branch_guard_hook_resolvable, git_branch_guard_script_integrity respectively), so no new
// `rules:` entries in trackfw.yaml are needed (REQ explicit instruction: "não hardcodar severidade
// nova sem seguir o padrão existente" / roadmap ML-1A step 6: "não inventar um mecanismo de
// configuração novo").
func validateCredentialGuardGlobalHookResolvable() ([]string, error) {
	return validateGuardGlobalHookResolvable("credential_guard_hook_resolvable", credentialGuardScriptMarker)
}

func validateCredentialGuardGlobalScriptIntegrity() ([]string, error) {
	return validateGuardGlobalScriptIntegrity(
		"credential_guard_script_integrity", credentialGuardScriptMarker, credentialGuardGlobalScriptReference,
	)
}

func validateGitBranchGuardGlobalHookResolvable() ([]string, error) {
	return validateGuardGlobalHookResolvable("git_branch_guard_hook_resolvable", gitBranchGuardScriptMarker)
}

func validateGitBranchGuardGlobalScriptIntegrity() ([]string, error) {
	return validateGuardGlobalScriptIntegrity(
		"git_branch_guard_script_integrity", gitBranchGuardScriptMarker, gitBranchGuardScriptReference,
	)
}
