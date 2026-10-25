package generators

// guard_generations.go — D13 migration helpers (ADR-2026-10-04 adendo 2026-10-23).
//
// These helpers implement the "migrate-and-deduplicate to exactly one entry per
// (event, matcher) pair" contract defined in D13. They are used by every Inject*Hooks
// function that emits a guard command. The registry of all known generations lives in
// internal/guardforms/guard_generations.go (D12).
//
// Two hook-file shapes exist:
//
//   flat: each entry is {"command":"…"[,"matcher":"…"[,…extra fields…]]}
//         used by Cursor (preToolUse/postToolUse/beforeShellExecution/afterShellExecution),
//         Windsurf (pre_run_command/pre_write_code).
//
//   nested: each entry is {"matcher":"…","hooks":[{"type":"command","command":"…"}]}
//           used by Claude (PreToolUse/PostToolUse), Codex (PreToolUse/PostToolUse),
//           Gemini (BeforeTool/AfterTool), Amazon Q (preToolUse).
//
// Both helpers:
//   - compare commands by EXACT equality (never strings.Contains) — T1 / AC5.
//   - preserve the position of the first matching entry and all its extra fields.
//   - leave entries outside the registry untouched (user hooks survive).
//   - if no matching entry exists, append one.
//   - after rewriting legacy → current and deduplicating, leave exactly ONE entry
//     for the (currentCmd, matcher) pair.

// migrateDedupGuardInFlatArray is the D13 helper for the flat-array format.
//
// For a given (event, matcher) pair:
//  1. Rewrite every entry whose command is in legacyForms to currentCmd (in-place,
//     preserving position and extra fields such as "timeout", "cwd", etc.).
//  2. Keep the FIRST occurrence of currentCmd+matcher; drop all later duplicates.
//  3. If no occurrence was found, append {"command":currentCmd[,"matcher":matcher]}.
//
// matcher="" means "match entries that have no matcher field" (shell events like
// beforeShellExecution). Entries in the array that belong to a different matcher
// are untouched — even if their command matches legacyForms.
//
// The "no matcher" case is safe even when the array mixes no-matcher and
// matcher-scoped entries (e.g. a preToolUse array with an attention-signal entry
// and Read/Write guard entries): each call targets exactly one (cmd,matcher) pair.
func migrateDedupGuardInFlatArray(
	existing interface{},
	matcher string,
	currentCmd string,
	legacyForms []string,
	getCmd func(interface{}) string,
) []interface{} {
	// Passes 1 and 2: delegate to dedupFlatGuardEntries (migrate legacy → current,
	// then deduplicate). Pass 3 (append if absent) is ours alone.
	// R1 (REQ-2026-10-23 ML-2C): eliminates ~55 duplicated lines between the two helpers.
	result := dedupFlatGuardEntries(existing, matcher, currentCmd, legacyForms, getCmd)

	// Pass 3: if the canonical is absent after passes 1+2, append a new entry.
	getMatcher := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		m, _ := obj["matcher"].(string)
		return m
	}
	seen := false
	for _, item := range result {
		if getCmd(item) == currentCmd && getMatcher(item) == matcher {
			seen = true
			break
		}
	}
	if !seen {
		entry := map[string]interface{}{"command": currentCmd}
		if matcher != "" {
			entry["matcher"] = matcher
		}
		result = append(result, entry)
	}
	return result
}

// removeGuardFromFlatArrayInHooks removes all recognized guard entries (legacy and
// current) from hooks[key], filtered by the given matcher. Used by D14 revised: with
// the global guard installed, project-scope entries in the pairs the global covers
// are removed rather than migrated.
//
// If the resulting array is empty, the key is DELETED from hooks (not set to nil or
// []interface{}{}) — avoiding a present-but-empty-array drift that the structural
// comparator treats as significant (see TestProjectGitBranchGuard_Cursor_BothGloballyInstalled_
// BothKeysPresent).
//
// Entries whose command is NOT in legacyForms ∪ {currentCmd} are left untouched.
func removeGuardFromFlatArrayInHooks(
	hooks map[string]interface{},
	key string,
	matcher string,
	currentCmd string,
	legacyForms []string,
	getCmd func(interface{}) string,
) {
	arr, _ := hooks[key].([]interface{})
	if len(arr) == 0 {
		return
	}

	allForms := make(map[string]bool, len(legacyForms)+1)
	for _, f := range legacyForms {
		allForms[f] = true
	}
	allForms[currentCmd] = true

	getMatcher := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		m, _ := obj["matcher"].(string)
		return m
	}

	var result []interface{}
	for _, item := range arr {
		_, ok := item.(map[string]interface{})
		if !ok {
			result = append(result, item)
			continue
		}
		m := getMatcher(item)
		cmd := getCmd(item)
		if m == matcher && allForms[cmd] {
			// This is a recognized guard entry in the target pair — remove it.
			continue
		}
		result = append(result, item)
	}

	if len(result) == 0 {
		delete(hooks, key)
	} else {
		hooks[key] = result
	}
}

// migrateDedupGuardInNestedArray is the D13 helper for the nested-array format.
//
// The nested format used by Claude, Codex, Gemini, and Amazon Q stores hook entries
// as outer envelopes: {"matcher":"…","hooks":[{"type":"command","command":"…"},…]}.
//
// For a given (event, matcher) pair across the existing outer array:
//  1. In every envelope whose "matcher" exactly equals the target, rewrite any inner
//     command that is in legacyForms to currentCmd (in-place, preserving position
//     and all other fields on the inner hook object).
//  2. Keep the FIRST inner occurrence of currentCmd across ALL matching envelopes;
//     drop all later occurrences. If that empties an envelope's "hooks" array,
//     remove the envelope entirely.
//  3. Does NOT append a new envelope — emission is handled by the existing
//     mergeClaudeHookArray call that follows ("dedup first, then merge" contract).
//
// Envelopes with a different matcher are left untouched, as are inner commands not
// in legacyForms ∪ {currentCmd} — user hooks survive byte-for-byte (AC5).
//
// Comparison is always exact equality (never strings.Contains) — T1.
func migrateDedupGuardInNestedArray(
	existing interface{},
	matcher string,
	currentCmd string,
	legacyForms []string,
) []interface{} {
	arr, _ := existing.([]interface{})

	// Build O(1) lookup for all recognized forms.
	allForms := make(map[string]bool, len(legacyForms)+1)
	for _, f := range legacyForms {
		allForms[f] = true
	}
	allForms[currentCmd] = true

	seenCmd := false // first occurrence of currentCmd across all matching envelopes
	var result []interface{}

	for _, item := range arr {
		env, ok := item.(map[string]interface{})
		if !ok {
			result = append(result, item)
			continue
		}
		m, _ := env["matcher"].(string)
		if m != matcher {
			// Non-target envelope: leave untouched.
			result = append(result, item)
			continue
		}
		// This envelope matches — process inner hooks.
		innerRaw, _ := env["hooks"].([]interface{})

		// Pass 1: rewrite legacy → currentCmd in-place (preserves position and extra fields).
		for _, hItem := range innerRaw {
			hObj, hOk := hItem.(map[string]interface{})
			if !hOk {
				continue
			}
			cmd, _ := hObj["command"].(string)
			if allForms[cmd] && cmd != currentCmd {
				hObj["command"] = currentCmd
			}
		}

		// Pass 2: deduplicate across envelopes — keep first currentCmd occurrence.
		var newInner []interface{}
		for _, hItem := range innerRaw {
			hObj, hOk := hItem.(map[string]interface{})
			if !hOk {
				newInner = append(newInner, hItem)
				continue
			}
			cmd, _ := hObj["command"].(string)
			if cmd == currentCmd {
				if !seenCmd {
					seenCmd = true
					newInner = append(newInner, hItem)
				}
				// else: drop duplicate across envelopes.
			} else {
				newInner = append(newInner, hItem)
			}
		}

		// If the envelope's hooks are now empty, remove the envelope.
		if len(newInner) == 0 {
			continue
		}
		// Keep envelope with updated inner hooks — copy to avoid mutating the
		// original map's "hooks" key while preserving all other fields (e.g. "sequential").
		newEnv := make(map[string]interface{}, len(env))
		for k, v := range env {
			newEnv[k] = v
		}
		newEnv["hooks"] = newInner
		result = append(result, newEnv)
	}

	return result
}

// removeGuardFromNestedArrayInHooks removes all recognized guard entries (legacy and
// current) from the inner hooks arrays of envelopes in hooks[key] whose "matcher"
// exactly equals the given matcher. Envelopes that become empty after removal are
// dropped from the outer array.
//
// Used by D14 revised: when the global guard is installed for a (event, matcher) pair,
// project-scope entries in that pair are removed rather than migrated or re-added.
//
// Envelopes with a different matcher, and inner commands not in legacyForms ∪ {currentCmd},
// are left untouched — user hooks survive (AC5).
//
// If the outer array becomes empty after removal, the key is deleted from hooks to
// avoid a present-but-empty-array drift that the structural comparator treats as significant.
func removeGuardFromNestedArrayInHooks(
	hooks map[string]interface{},
	key string,
	matcher string,
	currentCmd string,
	legacyForms []string,
) {
	arr, _ := hooks[key].([]interface{})
	if len(arr) == 0 {
		return
	}

	allForms := make(map[string]bool, len(legacyForms)+1)
	for _, f := range legacyForms {
		allForms[f] = true
	}
	allForms[currentCmd] = true

	var result []interface{}
	for _, item := range arr {
		env, ok := item.(map[string]interface{})
		if !ok {
			result = append(result, item)
			continue
		}
		m, _ := env["matcher"].(string)
		if m != matcher {
			result = append(result, item)
			continue
		}
		// This envelope matches — filter inner hooks.
		innerRaw, _ := env["hooks"].([]interface{})
		var newInner []interface{}
		for _, hItem := range innerRaw {
			hObj, hOk := hItem.(map[string]interface{})
			if !hOk {
				newInner = append(newInner, hItem)
				continue
			}
			cmd, _ := hObj["command"].(string)
			if allForms[cmd] {
				// Recognized guard entry in the target pair — remove.
				continue
			}
			newInner = append(newInner, hItem)
		}
		if len(newInner) == 0 {
			// All inner entries were guard entries — remove the envelope.
			continue
		}
		// Keep envelope with filtered inner hooks.
		newEnv := make(map[string]interface{}, len(env))
		for k, v := range env {
			newEnv[k] = v
		}
		newEnv["hooks"] = newInner
		result = append(result, newEnv)
	}

	if len(result) == 0 {
		delete(hooks, key)
	} else {
		hooks[key] = result
	}
}

// dedupFlatGuardEntries performs passes 1 and 2 of migrateDedupGuardInFlatArray
// (legacy→current rewrite in-place, then deduplication to first occurrence) but
// intentionally skips pass 3 (append when missing).
//
// Use this for CLIs whose hook entries carry extra fields that would be lost by a
// generic append — e.g., Windsurf's "show_output": true. After calling this, call
// the CLI's own makeEntry/mergeSimpleCommandArray to handle the "not yet present"
// case with the correct entry shape.
func dedupFlatGuardEntries(
	existing interface{},
	matcher string,
	currentCmd string,
	legacyForms []string,
	getCmd func(interface{}) string,
) []interface{} {
	arr, _ := existing.([]interface{})

	allForms := make(map[string]bool, len(legacyForms)+1)
	for _, f := range legacyForms {
		allForms[f] = true
	}
	allForms[currentCmd] = true

	getMatcher := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		m, _ := obj["matcher"].(string)
		return m
	}

	// Pass 1: rewrite legacy → currentCmd in-place.
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if getMatcher(item) != matcher {
			continue
		}
		cmd := getCmd(item)
		if allForms[cmd] && cmd != currentCmd {
			obj["command"] = currentCmd
		}
	}

	// Pass 2: deduplicate — keep first currentCmd+matcher, drop later occurrences.
	// Pass 3 (append if missing) is intentionally omitted.
	seen := false
	var result []interface{}
	for _, item := range arr {
		_, ok := item.(map[string]interface{})
		if !ok {
			result = append(result, item)
			continue
		}
		cmd := getCmd(item)
		m := getMatcher(item)
		if cmd == currentCmd && m == matcher {
			if !seen {
				seen = true
				result = append(result, item)
			}
			// else: drop duplicate.
		} else {
			result = append(result, item)
		}
	}
	return result
}

