package validator

import (
	"encoding/json"
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"
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

	content, err := readRegularFile(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio,
		// ML-1C: was `return nil, fmt.Errorf(...)` — hades-tf's ML-1B barrier found this ABORTED
		// the entire `trackfw validate` run (exit 1, non-JSON stdout, no other rule reported) on
		// the first unreadable script, in Go project-scope only — the exact defect this whole REQ
		// exists to close ("Go como fmt.Errorf que abortava trackfw validate inteiro"), alive in a
		// sibling this ML's predecessor never touched. Reported as a violation of THIS rule
		// instead, same remedy and message shape as the config-file read-error branches
		// (validateGuardHookResolvable, validator_credential_guard.go) — `trackfw update`
		// regenerates the file regardless of why it couldn't be read. readRegularFile
		// (regularfile.go) also closes the FIFO hang hades-tf found in the sibling functions: a
		// FIFO at relPath would otherwise block this read indefinitely too, since scripts are read
		// the same way configs are.
		return []string{fmt.Sprintf(
			"%s could not be read — trackfw cannot tell whether this script matches the template "+
				"it should; fix the file, or run `trackfw update` to regenerate it",
			relPath,
		)}, nil
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
// requiresCommandType (ROADMAP-2026-08-17 ML-4B) mirrors credentialGuardHookFile.
// requiresCommandType — see that field's doc comment in validator_credential_guard.go. true for
// every CLI except Cursor.
type globalGuardConfigFile struct {
	path                string // relative to $HOME
	cli                 string
	requiresCommandType bool
}

// globalGuardConfigFiles is the closed list of GLOBAL hook/settings files `trackfw update harness`
// can write a guard entry into — the global-scope counterpart of credentialGuardHookFiles
// (validator_credential_guard.go). Paths and CLI labels taken from
// globalCredentialGuardInstalled{Claude,Codex,Gemini,Cursor,Copilot,Kiro} in
// internal/generators/agentfiles.go (validator cannot import that package — see the import-cycle
// note on credentialGuardScriptReference — so this list is validator's own, independently
// maintained copy of the same 6 paths).
var globalGuardConfigFiles = []globalGuardConfigFile{
	{".claude/settings.json", "Claude Code", true},
	{".codex/hooks.json", "Codex CLI", true},
	{".gemini/settings.json", "Gemini CLI", true},
	{".cursor/hooks.json", "Cursor", false},
	{".copilot/settings.json", "GitHub Copilot CLI", true},
	{".kiro/hooks/trackfw-credential-guard.json", "Kiro", true},
}

// globalGuardConfigPath resolves the actual on-disk path (relative to $HOME) that
// validateGuardGlobalHookResolvable must read for a given (globalGuardConfigFile, scriptMarker)
// pair. For 5 of the 6 CLIs, gf.path already applies uniformly to both guards — Claude/Codex/
// Gemini/Cursor/Copilot merge both credential-guard and git-branch-guard entries into the SAME
// file (harnessCredentialGuardTarget*/harnessGitBranchGuardTarget*, internal/generators/
// agentfiles.go, both write into one shared document via mergeClaudeHookArray-equivalent helpers).
//
// Kiro is the sole exception (ROADMAP-2026-08-17 ML-2A, decision ratified by the architect): its
// writer rewrites the whole document wholesale instead of merging, so sharing one file between two
// independent wholesale writers would make both targets flap forever. Kiro therefore gets a
// SEPARATE dedicated file per guard — ~/.kiro/hooks/trackfw-credential-guard.json (gf.path, the
// default already in globalGuardConfigFiles above) and
// ~/.kiro/hooks/trackfw-git-branch-guard.json (only reachable for scriptMarker ==
// gitBranchGuardScriptMarker). check-harness-hooks-parity.sh's hookfile_for() encodes the exact
// same (cli, guard) -> path mapping on the generator side; this is the validator's read-side
// mirror of it.
//
// ROADMAP-2026-08-17 ML-3B: before this function existed, globalGuardConfigFiles only ever pointed
// Kiro at trackfw-credential-guard.json for BOTH guards, so git_branch_guard_hook_resolvable never
// inspected ~/.kiro/hooks/trackfw-git-branch-guard.json at all — the exact class of bug (artifact
// wired, never verified) this whole REQ exists to close, reintroduced in miniature by ML-2A itself.
func globalGuardConfigPath(gf globalGuardConfigFile, scriptMarker string) string {
	if gf.cli == "Kiro" && scriptMarker == gitBranchGuardScriptMarker {
		return ".kiro/hooks/trackfw-git-branch-guard.json"
	}
	return gf.path
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
// Fail-open: unresolvable $HOME and unreadable file skip that file in silence — legitimate states
// (guard never installed for this CLI, or filesystem race). Invalid JSON no longer does — see
// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1A, and
// the json.Unmarshal error branch below for the decision and its measurement.
func validateGuardGlobalHookResolvable(ruleName, scriptMarker string) ([]string, error) {
	home, err := homedir.Dir()
	if err != nil || home == "" {
		return nil, nil
	}

	var msgs []string
	for _, gf := range globalGuardConfigFiles {
		relPath := globalGuardConfigPath(gf, scriptMarker)
		fullPath := filepath.Join(home, relPath)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			// ROADMAP-2026-09-06-...-config-ilegivel-deixa-de-ser-silencio, ML-1C: same
			// readRegularFile substitution (regularfile.go) as the project-scope sibling
			// (validateGuardHookResolvable, validator_credential_guard.go) — see that function's
			// identical comment for the FIFO hang this closes and why errNotRegularFile needs no
			// new branch here.
			//
			// ROADMAP-2026-09-06-...-config-ilegivel-deixa-de-ser-silencio, ML-1B: same decision
			// as the project-scope sibling (validateGuardHookResolvable, validator_credential_
			// guard.go) — was `return nil, fmt.Errorf(...)`, which aborted the entire `trackfw
			// validate` run on the first unreadable global config file. See that function's read-
			// error branch for the full measurement (hades-tf ML-1A barrier, chmod 000 and a
			// directory in place of the file, reproduced live).
			msgs = append(msgs, fmt.Sprintf(
				"~/%s (%s, global scope) could not be read — trackfw cannot tell whether %s is "+
					"wired here, and %s cannot load any hook from a file it cannot read; fix the "+
					"file, or run `trackfw update harness` to regenerate it",
				relPath, gf.cli, scriptMarker, gf.cli,
			))
			continue
		}

		// ML-1B: see validateGuardHookResolvable's identical comment (validator_credential_
		// guard.go) for why decoding, not just parsing, must be classified per-runtime here.
		wasValidUTF8 := utf8.Valid(content)

		var parsed interface{}
		if jsonErr := json.Unmarshal(content, &parsed); jsonErr != nil {
			// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio,
			// ML-1A: same decision as the project-scope sibling (validateGuardHookResolvable,
			// validator_credential_guard.go) — see that function's json.Unmarshal error branch for
			// the full measurement. Applies identically here: globalGuardConfigFiles is the same
			// kind of closed, enumerable list (6 entries), each owned/consumed by gf.cli itself, so
			// invalid JSON is never a legitimate state for any of them.
			//
			// ML-1B: not-valid-UTF-8 gets its own message — see the identical branch in
			// validateGuardHookResolvable for the reasoning (D4 of ADR-2026-09-04).
			if !wasValidUTF8 {
				msgs = append(msgs, fmt.Sprintf(
					"~/%s (%s, global scope) is not valid UTF-8 — trackfw cannot tell whether %s "+
						"is wired here, and %s cannot load any hook from a file it cannot decode; "+
						"fix the file's encoding, or run `trackfw update harness` to regenerate it",
					relPath, gf.cli, scriptMarker, gf.cli,
				))
				continue
			}
			msgs = append(msgs, fmt.Sprintf(
				"~/%s (%s, global scope) is not valid JSON — trackfw cannot tell whether %s is "+
					"wired here, and %s cannot load any hook from a file it cannot parse; fix the "+
					"file, or run `trackfw update harness` to regenerate it",
				relPath, gf.cli, scriptMarker, gf.cli,
			))
			continue
		}

		var commands []guardCommandMatch
		collectCommandsWithMarker(parsed, scriptMarker, &commands)

		seen := make(map[string]bool, len(commands))
		for _, m := range commands {
			seenKey := m.raw + "\x00" + strconv.FormatBool(m.typeIsCommand)
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true

			// ADR-2026-09-04-caminho-posix-ancorado-...: o consumidor deste comando é o CLI do
			// agente (que o repassa a bash), não o filesystem do processo Go — pathIsAnchoredForHookConfig
			// classifica por ancoragem (POSIX "/", letra de unidade, UNC), NÃO por filepath.IsAbs, que
			// no Windows devolve false para "/opt/foo/guard.sh" e faria este `continue` pular a entrada
			// inteira — no Windows, uma entrada de config global com comando absoluto POSIX nunca
			// seria verificada.
			if !pathIsAnchoredForHookConfig(m.raw) {
				continue
			}

			// ROADMAP-2026-08-17 ML-4B: reproduced by hades-tf (ML-4A barrier) — a global config
			// entry with the correct absolute command but missing/wrong "type" (hand-edited, an
			// older trackfw version, another tool's merge) is silently never executed by the CLI,
			// even though the script itself exists and is fine. Before this ML that state was
			// invisible to both the dedup (agentfiles.go's hookArrayHasCommand/simpleArrayHasValue,
			// fixed in the same ML) and this rule — "nenhum dos dois escopos protege, e tudo fica
			// verde". Reported instead of the exists/executable checks below, which assume the
			// entry is structurally valid.
			if gf.requiresCommandType && !m.typeIsCommand {
				msgs = append(msgs, fmt.Sprintf(
					`~/%s (%s, global scope) references %s resolved to %q, but the hook entry is missing "type":"command" (or has an invalid type) — %s will silently never execute it; run `+"`trackfw update harness`"+` to regenerate it`,
					relPath, gf.cli, scriptMarker, m.raw, gf.cli,
				))
				continue
			}

			info, statErr := os.Stat(m.raw)
			switch {
			case statErr != nil:
				msgs = append(msgs, fmt.Sprintf(
					"~/%s (%s, global scope) references %s resolved to %q, but the script does not exist — run `trackfw update harness` to regenerate it",
					relPath, gf.cli, scriptMarker, m.raw,
				))
			case CurrentGOOS != "windows" && info.Mode()&0111 == 0:
				msgs = append(msgs, fmt.Sprintf(
					"~/%s (%s, global scope) references %s resolved to %q, but the script is not executable — run `trackfw update harness` to regenerate it",
					relPath, gf.cli, scriptMarker, m.raw,
				))
			}
		}
	}

	return msgs, nil
}

// validateGuardGlobalScriptIntegrity is the GLOBAL-scope counterpart of
// validateCredentialGuardScriptIntegrity/validateGitBranchGuardScriptIntegrity: verifies the
// content of ~/.trackfw/scripts/<scriptFileName> (the fixed location GenerateGlobal*Script writes
// to — see globalCredentialGuardScriptPath/globalGitBranchGuardScriptPath in
// internal/generators/agentfiles.go, both `filepath.Join(home, ".trackfw", "scripts", name)")
// against referenceContent byte-for-byte.
//
// ROADMAP-2026-08-17 (guard global cabeado com no-op / integridade independente de fiação), ML-3A:
// deliberately triggers on ARTIFACT EXISTENCE, not on any config file referencing scriptMarker.
// Before this ML the trigger was "one of the 6 globalGuardConfigFiles references scriptMarker" —
// which meant a script trackfw itself wrote (via `trackfw update harness`,
// GenerateGlobal{CredentialGuard,GitBranchGuard}Script) but that no config yet pointed at could
// rot indefinitely with `validate` green. Measured on KG's machine: the git-branch-guard script sat
// 3 releases behind (123 lines vs. 369) with zero warnings, because nothing wired it until Wave 2
// of this same roadmap. "If trackfw wrote the script, trackfw verifies the script" — existence is
// the only precondition; wiring is irrelevant to whether the artifact itself has drifted.
//
// Fail-open on absence: a script trackfw never wrote (user never ran `update harness`) is not an
// error — same silent-on-absence contract every other guard integrity check in this file has.
//
// Single evaluation per script (not per referencing config): this is what prevents the double
// report now that git-branch-guard has BOTH global wiring (Wave 2) and a global artifact — under
// the old per-config loop, a script referenced by N configs would have produced up to N messages;
// checking the one fixed on-disk path exactly once caps it at 1 regardless of how many (or how
// few — including zero) configs reference it.
func validateGuardGlobalScriptIntegrity(scriptFileName, referenceContent string) ([]string, error) {
	home, err := homedir.Dir()
	if err != nil || home == "" {
		return nil, nil
	}

	path := filepath.Join(home, ".trackfw", "scripts", scriptFileName)
	content, readErr := readRegularFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			// Not installed is not a violation — same contract as every other *_script_integrity
			// check (project-scope and global) in this package: existence is
			// validateGuardGlobalHookResolvable's job when wiring exists at all, but here there
			// may be no wiring, so absence is simply the legitimate "trackfw update harness was
			// never run" state.
			return nil, nil
		}
		// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio,
		// ML-1C: was an UNCONDITIONAL `return nil, nil` on ANY readErr — hades-tf's ML-1B barrier
		// found this silenced EACCES/EISDIR/ELOOP exactly like the pre-ML-1B config-file `continue`
		// did (fail-open: validate reports health about an artifact it never actually inspected).
		// Distinguished from absence by os.IsNotExist, mirroring every other read-error branch in
		// this package. readRegularFile (regularfile.go) also closes the FIFO hang for this path.
		return []string{fmt.Sprintf(
			"%s (global scope) could not be read — trackfw cannot tell whether this script "+
				"matches the template it should; fix the file, or run `trackfw update harness` to "+
				"regenerate it",
			path,
		)}, nil
	}

	if string(content) == referenceContent {
		return nil, nil
	}

	return []string{fmt.Sprintf(
		"%s (global scope) content diverges from the template this version of trackfw generates — "+
			"if you did not edit this file by hand, run `trackfw update harness` to regenerate it",
		path,
	)}, nil
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
	return validateGuardGlobalScriptIntegrity(credentialGuardScriptMarker, credentialGuardGlobalScriptReference)
}

func validateGitBranchGuardGlobalHookResolvable() ([]string, error) {
	return validateGuardGlobalHookResolvable("git_branch_guard_hook_resolvable", gitBranchGuardScriptMarker)
}

func validateGitBranchGuardGlobalScriptIntegrity() ([]string, error) {
	return validateGuardGlobalScriptIntegrity(gitBranchGuardScriptMarker, gitBranchGuardScriptReference)
}
