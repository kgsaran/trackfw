package validator

// validator_guard_wiring.go — ML-1A (REQ-2026-09-02, ADR-2026-10-10)
//
// Rule: guard_wiring_removed
//
// Detects when guard hook wiring tuples (event, matcher, guard-type) present in origin/main are
// absent from disk. Also detects disable keys active on disk and not present in origin/main.
//
// ADR-2026-10-10 decisions implemented here:
//   D1: anchor = origin/main copy of each hook file (not HEAD, not disk-only)
//   D2: unit of comparison = tuple (event, matcher, guard-type); not key existence alone
//   D3: population = credentialGuardHookFiles (all 8 CLI hook files)
//   D4: absent in both → silent; present in origin, absent on disk → violation
//   D5: rule is in credentialGuardAnchoredRules + lenientCarveoutRules (see respective files)
//   D6: git_branch_guard_{hook_resolvable,script_integrity} also in credentialGuardAnchoredRules
//   D7: disable keys are wiring removal — same severity, same rule
//   A2: command by equivalence class (D2-legacy, D11-legacy, D11-revised);
//       matcher by coverage (disk alternatives ⊇ origin alternatives for literal alternations)
//   A4: derive ref independently via deriveOriginDefaultBranch(); origin present but ref
//       unreadable → fail closed; never reuse currentOriginMain.ref (set only in originAnchorOK)
//
// Anchor states (matching loadOriginMainAnchor / ADR-2026-09-17):
//   noGit       → silent
//   noRemote    → silent
//   refUnreadable → FAIL CLOSED (violation naming the cause)
//   fileAbsent  → "never installed" → silent for tuple comparison
//   filePresent → compare tuples + disable keys

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// wiringTuple is one guard hook entry: the event name, the matcher pattern, and the guard type.
type wiringTuple struct {
	event     string // e.g. "PreToolUse", "pre_run_command", "beforeShellExecution"
	matcher   string // e.g. "Bash|PowerShell", "shell", "" for CLIs without a matcher field
	guardType string // "credential" or "git-branch"
}

// disableKeyKind classifies the hook disable mechanism for a given CLI.
type disableKeyKind int

const (
	disableKeyKindNone           disableKeyKind = iota
	disableKeyKindDisableAllHooks               // JSON: disableAllHooks: true
	disableKeyKindHooksEnabled                  // JSON: hooksConfig.enabled: false
	disableKeyKindEntryEnabled                  // JSON: "enabled": false on a guard entry
)

// guardWiringDisableKeys maps the hook file path to its D7 disable-key mechanism.
// CLIs not in this map have no documented disable key (Cursor, Windsurf, Amazon Q).
// Source: docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md §2a,
//         vault/notes/hooks-tem-chave-de-desligamento-por-cli-e-a-fiacao-intacta-nao-prova-guard-ativo-2026-10-10.md
var guardWiringDisableKeys = map[string]disableKeyKind{
	".claude/settings.json":                  disableKeyKindDisableAllHooks,
	".github/hooks/trackfw-attention.json":   disableKeyKindDisableAllHooks,
	".gemini/settings.json":                  disableKeyKindHooksEnabled,
	".kiro/hooks/trackfw-attention.json":     disableKeyKindEntryEnabled,
}

// validateGuardWiringRemoved is the guard_wiring_removed rule (ADR-2026-10-10, ML-1A).
//
// For each hook file in credentialGuardHookFiles it:
//  1. Reads the file from origin/main (4-state anchor: no-git → silent, no-origin → silent,
//     ref-unreadable → fail-closed, file-absent → "never installed" = silent for tuples).
//  2. Extracts guard tuples from both origin/main and disk.
//  3. Reports a violation for each origin tuple absent from disk.
//  4. Checks D7 disable keys (disk active and origin inactive/absent → violation).
//
// Additionally checks .claude/settings.local.json (disk-only; no anchor) and
// .codex/config.toml (if tracked by git) for D7 disable keys.
//
// Called by ValidateUnfiltered and validateUnfilteredTagged; see those callers for registration.
func validateGuardWiringRemoved() ([]string, error) {
	// State 1: not a git worktree → silent.
	if !isGitWorktree(".") {
		return nil, nil
	}
	// State 2: no "origin" remote → silent.
	if err := gitCommand(".", "remote", "get-url", "origin").Run(); err != nil {
		return nil, nil
	}
	// A4: derive ref independently; never use currentOriginMain.ref.
	// currentOriginMain.ref is only set when originAnchorOK (trackfw.yaml present AND readable
	// in origin/main — validator_credential_guard_integrity.go:306). Hook files exist in repos
	// that don't have a committed trackfw.yaml, so the state is often originAnchorNoFile.
	ref, found := deriveOriginDefaultBranch()
	if !found {
		// Distinguish two sub-cases:
		//   (a) No refs fetched at all: repo freshly cloned without fetch, or CI shallow checkout
		//       without an explicit fetch step. The existing anchor mechanism already emits a warning
		//       for this (originMainRefUnreadableMessage). My rule defers → silent.
		//   (b) Remote refs ARE fetched but none match "main" or "master" → ambiguous; this is
		//       adversary-reachable (create two branches with non-standard names) → fail closed.
		if !guardWiringHasAnyFetchedOriginRefs() {
			return nil, nil // (a) not yet fetched → silent; let anchor warning cover it
		}
		// (b) Refs present but ambiguous → fail closed.
		return []string{
			"guard_wiring_removed: origin remote exists but the default branch ref could not be " +
				"determined from local refs (multiple non-standard branches found) — " +
				"ensure `git fetch origin` has been run and that origin has a 'main' or 'master' " +
				"branch; cannot verify guard wiring without an anchor",
		}, nil
	}

	var msgs []string

	// --- Per-file tuple and disable-key comparison ---
	for _, hf := range credentialGuardHookFiles {
		// Read hook file from origin/main.
		originContent, originAbsent, originErr := guardWiringReadFromOrigin(ref, hf.path)
		if originErr != nil {
			msgs = append(msgs, fmt.Sprintf(
				"guard_wiring_removed: %s (%s): file is listed in %s but could not be "+
					"read — fail closed; run `git fetch origin` to repair",
				hf.path, hf.cli, ref,
			))
			continue
		}

		// Read disk copy (used for both tuple and disable-key comparisons).
		diskContent, diskReadErr := readRegularFile(hf.path)
		diskAbsent := diskReadErr != nil && os.IsNotExist(diskReadErr)

		// D7: check disable keys for hook files that have a documented disable mechanism.
		if dkKind := guardWiringDisableKeys[hf.path]; dkKind != disableKeyKindNone {
			var originDisableActive bool
			if !originAbsent {
				originDisableActive = guardWiringParseDisableKey(originContent, dkKind, hf.family)
			}
			var diskDisableActive bool
			if !diskAbsent && diskReadErr == nil {
				diskDisableActive = guardWiringParseDisableKey(diskContent, dkKind, hf.family)
			}
			if diskDisableActive && !originDisableActive {
				msgs = append(msgs, guardWiringDisableKeyMsg(hf, dkKind))
			}
		}

		// Tuple comparison: absent in origin/main → "never installed" → silent.
		if originAbsent {
			continue
		}

		// Parse origin tuples. Invalid JSON → fail closed.
		originTuples, parseErr := guardWiringParseTuples(originContent, hf)
		if parseErr != nil {
			msgs = append(msgs, fmt.Sprintf(
				"guard_wiring_removed: %s (%s): content at %s is not valid JSON — "+
					"fail closed; run `git fetch origin` and inspect the file",
				hf.path, hf.cli, ref,
			))
			continue
		}
		if len(originTuples) == 0 {
			// No guard wiring in origin/main → nothing to compare.
			continue
		}

		// Parse disk tuples. Absent/unreadable/invalid → empty set (all origin tuples missing).
		var diskTuples []wiringTuple
		if !diskAbsent && diskReadErr == nil {
			diskTuples, _ = guardWiringParseTuples(diskContent, hf)
		}

		// Report each origin tuple that is not covered by any disk tuple (D2, A2).
		for _, ot := range originTuples {
			if !guardWiringTupleCovered(ot, diskTuples) {
				msgs = append(msgs, fmt.Sprintf(
					"guard_wiring_removed: %s (%s): guard %q wiring "+
						"(event=%q matcher=%q) was present in %s and is absent from "+
						"disk — this is a security control; investigate before proceeding",
					hf.path, hf.cli, ot.guardType, ot.event, ot.matcher, ref,
				))
			}
		}
	}

	// D7: .claude/settings.local.json — disk-only, no origin anchor.
	msgs = append(msgs, guardWiringCheckSettingsLocal()...)

	// D7: .codex/config.toml — only if tracked by git.
	codexMsgs, codexErr := guardWiringCheckCodexToml(ref)
	if codexErr != nil {
		msgs = append(msgs, codexErr.Error())
	} else {
		msgs = append(msgs, codexMsgs...)
	}

	return msgs, nil
}

// --------------------------------------------------------------------------
// Origin reading
// --------------------------------------------------------------------------

// guardWiringReadFromOrigin reads the hook file at path from the given git ref.
// Returns (content, isAbsent, error):
//   - content non-nil, isAbsent=false: file present and readable.
//   - content nil, isAbsent=true: file not present in that ref ("never installed").
//   - content nil, isAbsent=false, err non-nil: ls-tree listed it but git-show failed (fail-closed).
func guardWiringReadFromOrigin(ref, path string) ([]byte, bool, error) {
	lsOut, lsErr := gitCommand(".", "ls-tree", ref, "--", "./"+path).Output()
	if lsErr != nil || len(strings.TrimSpace(string(lsOut))) == 0 {
		return nil, true, nil // absent
	}
	showOut, showErr := gitCommand(".", "show", ref+":./" + path).Output()
	if showErr != nil {
		return nil, false, fmt.Errorf("git show %s:./%s: %w", ref, path, showErr)
	}
	return showOut, false, nil
}

// --------------------------------------------------------------------------
// Tuple extraction
// --------------------------------------------------------------------------

// guardWiringParseTuples parses a JSON hook file and returns all guard wiring tuples.
func guardWiringParseTuples(content []byte, hf credentialGuardHookFile) ([]wiringTuple, error) {
	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		return nil, err
	}
	return guardWiringExtractTuples(parsed, hf.family), nil
}

// guardWiringExtractTuples walks a parsed JSON hook file and returns all guard wiring tuples.
// Handles both hooks-as-object (Claude, Codex, Gemini, Cursor, Copilot, Windsurf, Amazon Q)
// and hooks-as-array (Kiro) formats.
func guardWiringExtractTuples(parsed map[string]any, family guardShellFamily) []wiringTuple {
	hooksRaw, ok := parsed["hooks"]
	if !ok {
		return nil
	}

	var tuples []wiringTuple
	seen := make(map[string]bool)

	addTuple := func(event, matcher, guardType string) {
		key := event + "\x00" + matcher + "\x00" + guardType
		if !seen[key] {
			seen[key] = true
			tuples = append(tuples, wiringTuple{event: event, matcher: matcher, guardType: guardType})
		}
	}

	addCommandsFromEntry := func(event, matcher string, entry map[string]any) {
		var cmds []string
		guardWiringCollectCommandValues(entry, &cmds)
		for _, cmd := range cmds {
			if gt := guardWiringTypeFromCmd(cmd, family); gt != "" {
				addTuple(event, matcher, gt)
			}
		}
	}

	switch h := hooksRaw.(type) {
	case map[string]any:
		// Hooks-as-object: event = key, entries = []entry.
		for eventName, entriesRaw := range h {
			entries, ok := entriesRaw.([]any)
			if !ok {
				continue
			}
			for _, entryRaw := range entries {
				entry, ok := entryRaw.(map[string]any)
				if !ok {
					continue
				}
				matcher, _ := entry["matcher"].(string)
				addCommandsFromEntry(eventName, matcher, entry)
			}
		}
	case []any:
		// Hooks-as-array: Kiro format (trigger, matcher, action.command).
		for _, entryRaw := range h {
			entry, ok := entryRaw.(map[string]any)
			if !ok {
				continue
			}
			trigger, _ := entry["trigger"].(string)
			matcher, _ := entry["matcher"].(string)
			addCommandsFromEntry(trigger, matcher, entry)
		}
	}

	return tuples
}

// guardWiringCollectCommandValues recursively collects all string values under any "command" key.
// This covers:
//   - Direct "command" field (Cursor, Windsurf)
//   - Nested hooks[].command (Claude, Codex, Gemini, Copilot, Amazon Q)
//   - action.command (Kiro)
func guardWiringCollectCommandValues(v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "command" {
				if s, ok := val.(string); ok {
					*out = append(*out, s)
				}
			}
			guardWiringCollectCommandValues(val, out)
		}
	case []any:
		for _, item := range t {
			guardWiringCollectCommandValues(item, out)
		}
	}
}

// guardWiringTypeFromCmd returns "credential" or "git-branch" if cmd is in the guard command
// equivalence class for the given shell family, or "" otherwise.
//
// Equivalence class (ADR-2026-10-10 A2): D2-legacy + D11-legacy + D11-revised — the same
// three forms that migrateHookCommand in internal/generators/agentfiles.go recognises.
// The validator cannot import generators (cycle), so the equivalence is maintained here
// via the shared helpers guardExpectedLine, guardD11LegacyLine, guardD2LegacyLine
// (validator_guard_binary_probe.go).
func guardWiringTypeFromCmd(cmd string, fam guardShellFamily) string {
	for _, subcmd := range []string{"credential", "git-branch"} {
		if cmd == guardExpectedLine(subcmd, fam) ||
			cmd == guardD11LegacyLine(subcmd, fam) ||
			cmd == guardD2LegacyLine(subcmd, fam) {
			return subcmd
		}
	}
	return ""
}

// --------------------------------------------------------------------------
// Tuple coverage (A2)
// --------------------------------------------------------------------------

// guardWiringTupleCovered returns true if origin is covered by any tuple in disk.
// Coverage: same event, same guard type, disk matcher ⊇ origin matcher.
func guardWiringTupleCovered(origin wiringTuple, disk []wiringTuple) bool {
	for _, d := range disk {
		if d.event == origin.event &&
			d.guardType == origin.guardType &&
			guardWiringMatcherCovers(d.matcher, origin.matcher) {
			return true
		}
	}
	return false
}

// guardWiringMatcherCovers returns true if diskMatcher covers originMatcher (ADR-2026-10-10 A2):
//  1. Exact equality always covers (including both-empty).
//  2. If both are non-empty literal alternations (only [A-Za-z0-9_-|]):
//     disk alternatives must be a superset of origin alternatives.
//  3. Otherwise: not covered (non-literal matchers cannot be compared → conservative).
func guardWiringMatcherCovers(diskMatcher, originMatcher string) bool {
	if diskMatcher == originMatcher {
		return true
	}
	if originMatcher == "" || diskMatcher == "" {
		return false
	}
	if !guardWiringIsLiteralAlternation(originMatcher) ||
		!guardWiringIsLiteralAlternation(diskMatcher) {
		return false
	}
	diskAlts := strings.Split(diskMatcher, "|")
	diskSet := make(map[string]bool, len(diskAlts))
	for _, a := range diskAlts {
		diskSet[a] = true
	}
	for _, oa := range strings.Split(originMatcher, "|") {
		if !diskSet[oa] {
			return false
		}
	}
	return true
}

// guardWiringIsLiteralAlternation returns true if m consists only of [A-Za-z0-9_-|], indicating
// a pipe-separated list of literal alternatives with no regex metacharacters.
func guardWiringIsLiteralAlternation(m string) bool {
	if m == "" {
		return false
	}
	for _, c := range m {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' || c == '|') {
			return false
		}
	}
	return true
}

// --------------------------------------------------------------------------
// D7 disable keys
// --------------------------------------------------------------------------

// guardWiringParseDisableKey returns true if content has the specified disable key active.
func guardWiringParseDisableKey(content []byte, kind disableKeyKind, family guardShellFamily) bool {
	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		return false // invalid JSON → cannot determine → safe default
	}
	switch kind {
	case disableKeyKindDisableAllHooks:
		v, _ := parsed["disableAllHooks"].(bool)
		return v
	case disableKeyKindHooksEnabled:
		if hc, ok := parsed["hooksConfig"].(map[string]any); ok {
			enabled, ok := hc["enabled"].(bool)
			return ok && !enabled // explicit false → disabled
		}
		return false
	case disableKeyKindEntryEnabled:
		// Check if any hook array entry containing a guard command has "enabled": false.
		hooksRaw, ok := parsed["hooks"]
		if !ok {
			return false
		}
		hooksArr, ok := hooksRaw.([]any)
		if !ok {
			return false
		}
		for _, entryRaw := range hooksArr {
			entry, ok := entryRaw.(map[string]any)
			if !ok {
				continue
			}
			enabledVal, hasEnabled := entry["enabled"]
			if !hasEnabled {
				continue
			}
			enabled, ok := enabledVal.(bool)
			if !ok || enabled {
				continue // enabled:true or non-bool → not disabled
			}
			// enabled:false — does this entry contain a guard command?
			var cmds []string
			guardWiringCollectCommandValues(entry, &cmds)
			for _, cmd := range cmds {
				if guardWiringTypeFromCmd(cmd, family) != "" {
					return true
				}
			}
		}
		return false
	}
	return false
}

// guardWiringDisableKeyMsg formats a D7 disable-key violation message.
func guardWiringDisableKeyMsg(hf credentialGuardHookFile, kind disableKeyKind) string {
	var keyDesc string
	switch kind {
	case disableKeyKindDisableAllHooks:
		keyDesc = "disableAllHooks: true"
	case disableKeyKindHooksEnabled:
		keyDesc = "hooksConfig.enabled: false"
	case disableKeyKindEntryEnabled:
		keyDesc = `"enabled": false on a guard hook entry`
	}
	return fmt.Sprintf(
		"guard_wiring_removed: %s (%s): %s disables all hooks including guard wiring — "+
			"this key was absent in origin/main; investigate before proceeding",
		hf.path, hf.cli, keyDesc,
	)
}

// guardWiringCheckSettingsLocal checks .claude/settings.local.json for disableAllHooks:true.
// This file is git-ignored; there is no origin anchor — the check is disk-only.
// The generator never writes this file, so disableAllHooks:true is always added externally.
func guardWiringCheckSettingsLocal() []string {
	content, err := readRegularFile(".claude/settings.local.json")
	if err != nil {
		return nil // absent or unreadable → silent
	}
	var parsed map[string]any
	if json.Unmarshal(content, &parsed) != nil {
		return nil // invalid JSON → cannot determine → silent
	}
	if v, _ := parsed["disableAllHooks"].(bool); v {
		return []string{
			"guard_wiring_removed: .claude/settings.local.json (Claude Code): " +
				"disableAllHooks: true disables all hooks including guard wiring — " +
				"this file is not tracked by git (no anchor); investigate before proceeding",
		}
	}
	return nil
}

// guardWiringCheckCodexToml checks .codex/config.toml for [features] hooks disabled (or
// unrecognized form of the hooks key), but only if the file is tracked by git
// (untracked → residual R3, silent). Uses the tri-state parser: only an explicit
// hooks = true counts as "enabled"; anything else that mentions hooks under [features]
// is treated as disabled (fail closed per ADR D7).
// Returns (messages, failClosedError).
func guardWiringCheckCodexToml(ref string) ([]string, error) {
	if !guardWiringIsFileTracked(".codex/config.toml") {
		return nil, nil
	}
	diskContent, diskErr := readRegularFile(".codex/config.toml")
	if diskErr != nil {
		return nil, nil // unreadable → silent (other rules cover unreachable files)
	}
	diskState := guardWiringParseTomlFeaturesHooks(string(diskContent))
	if diskState == tomlHooksAbsent || diskState == tomlHooksTrue {
		return nil, nil // not disabled on disk
	}
	// Disk has hooks disabled or in an unrecognized form (fail closed). Check origin.
	originContent, originAbsent, readErr := guardWiringReadFromOrigin(ref, ".codex/config.toml")
	if readErr != nil {
		// Fail closed: file listed in ls-tree but show failed.
		return nil, fmt.Errorf( //nolint:goerr113 // dynamic fail-closed message, not sentinel
			"guard_wiring_removed: .codex/config.toml (Codex CLI): "+
				"file is listed in %s but could not be read — fail closed", ref,
		)
	}
	var originState tomlFeaturesHooksState
	if !originAbsent {
		originState = guardWiringParseTomlFeaturesHooks(string(originContent))
	}
	if originState != tomlHooksAbsent && originState != tomlHooksTrue {
		// Origin also has non-true hooks value → was already present in origin; not a new violation.
		return nil, nil
	}
	return []string{
		"guard_wiring_removed: .codex/config.toml (Codex CLI): " +
			"[features] hooks is not set to true — hooks are disabled or the value could " +
			"not be classified as enabled; this was absent in origin/main; " +
			"investigate before proceeding",
	}, nil
}

// guardWiringIsFileTracked returns true if path is tracked in the git index (committed or staged).
func guardWiringIsFileTracked(path string) bool {
	out, err := gitCommand(".", "ls-files", "--", path).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// guardWiringHasAnyFetchedOriginRefs returns true if at least one refs/remotes/origin/ ref
// (other than the HEAD pointer) is present locally. Used to distinguish:
//   - "not yet fetched" (0 refs) → silent; the existing anchor warning covers it
//   - "fetched but ambiguous" (≥1 ref, none is main/master and there are 2+) → fail closed
func guardWiringHasAnyFetchedOriginRefs() bool {
	out, err := gitCommand(".", "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin/").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "origin" && line != "origin/HEAD" {
			return true
		}
	}
	return false
}

// tomlFeaturesHooksState is the three-value result of guardWiringParseTomlFeaturesHooks.
//
//	tomlHooksAbsent           — [features] hooks key not mentioned at all → not disabled.
//	tomlHooksTrue             — hooks is explicitly set to true → hooks enabled.
//	tomlHooksDisabledOrUnknown — hooks is set to false, or mentioned in a form the parser
//	                             cannot classify as true (inline table, quoted value, etc.)
//	                             → treated as disabled (fail closed per ADR D7).
type tomlFeaturesHooksState int

const (
	tomlHooksAbsent           tomlFeaturesHooksState = iota
	tomlHooksTrue                                     // hooks = true (explicitly enabled)
	tomlHooksDisabledOrUnknown                        // hooks disabled or form unclassifiable as true
)

// guardWiringParseTomlFeaturesHooks returns the tomlFeaturesHooksState for the content:
//   - [features] section with hooks key, OR
//   - Dotted key features.hooks = ..., OR
//   - Inline table features = { hooks = ... }.
//
// Only the exact bare value `true` counts as tomlHooksTrue.
// Inline comments (# ...) are stripped before value classification.
// Unrecognised or unclassifiable value → tomlHooksDisabledOrUnknown (fail closed).
// Key absent from [features] context → tomlHooksAbsent.
func guardWiringParseTomlFeaturesHooks(content string) tomlFeaturesHooksState {
	inFeatures := false
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip inline comment from the line.
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}

		// Section header: [features] — normalize inner spaces/quotes.
		if strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[[") {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				inFeatures = false
				continue
			}
			sectionName := strings.TrimSpace(line[1:end])
			sectionName = strings.Trim(sectionName, "\"'")
			inFeatures = (sectionName == "features")
			continue
		}

		// Inline table: features = { ... }
		if guardWiringTomlIsFeatureInlineTable(line) {
			braceStart := strings.IndexByte(line, '{')
			braceEnd := strings.LastIndexByte(line, '}')
			if braceStart < 0 || braceEnd < braceStart {
				// Brace not closed on same line → unclassifiable.
				return tomlHooksDisabledOrUnknown
			}
			inner := line[braceStart+1 : braceEnd]
			if s := guardWiringTomlParseInlineTableHooks(inner); s != tomlHooksAbsent {
				return s
			}
			continue
		}

		// Dotted key: features.hooks = ...
		if strings.HasPrefix(line, "features.hooks") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "features.hooks"))
			if strings.HasPrefix(rest, "=") {
				val := strings.TrimSpace(rest[1:])
				return guardWiringTomlClassifyBool(val)
			}
		}

		// In-section key: hooks = ... (within [features]).
		if inFeatures {
			if eqIdx := strings.IndexByte(line, '='); eqIdx > 0 {
				key := strings.TrimSpace(line[:eqIdx])
				val := strings.TrimSpace(line[eqIdx+1:])
				if key == "hooks" {
					return guardWiringTomlClassifyBool(val)
				}
			}
		}
	}
	return tomlHooksAbsent
}

// guardWiringTomlIsFeatureInlineTable returns true if line looks like
// `features = { ... }` (inline table for the features key).
func guardWiringTomlIsFeatureInlineTable(line string) bool {
	if !strings.HasPrefix(line, "features") {
		return false
	}
	rest := strings.TrimSpace(line[len("features"):])
	return strings.HasPrefix(rest, "=") && strings.Contains(rest, "{")
}

// guardWiringTomlClassifyBool classifies a TOML value string as true, false/unknown.
// Only the bare value `true` is considered enabled.
func guardWiringTomlClassifyBool(val string) tomlFeaturesHooksState {
	if val == "true" {
		return tomlHooksTrue
	}
	// Anything else: false, quoted strings, integers, etc. → disabled or unclassifiable.
	return tomlHooksDisabledOrUnknown
}

// guardWiringTomlParseInlineTableHooks parses the interior of a TOML inline table
// (the part between { and }) looking for a `hooks` key.
func guardWiringTomlParseInlineTableHooks(inner string) tomlFeaturesHooksState {
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		eqIdx := strings.IndexByte(part, '=')
		if eqIdx <= 0 {
			continue
		}
		key := strings.TrimSpace(part[:eqIdx])
		val := strings.TrimSpace(part[eqIdx+1:])
		// Strip inline comment within the part.
		if idx := strings.IndexByte(val, '#'); idx >= 0 {
			val = strings.TrimSpace(val[:idx])
		}
		if key == "hooks" {
			return guardWiringTomlClassifyBool(val)
		}
	}
	return tomlHooksAbsent
}
