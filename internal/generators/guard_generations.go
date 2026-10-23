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
//         Windsurf (pre_run_command), Amazon Q (preToolUse).
//
//   nested: each entry is {"matcher":"…","hooks":[{"type":"command","command":"…"}]}
//           used by Claude (PreToolUse/PostToolUse), Codex (PreToolUse/PostToolUse),
//           Gemini (BeforeTool/AfterTool).
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
	arr, _ := existing.([]interface{})

	// Build O(1) lookup for all recognized forms.
	allForms := make(map[string]bool, len(legacyForms)+1)
	for _, f := range legacyForms {
		allForms[f] = true
	}
	allForms[currentCmd] = true

	// getMatcher extracts the "matcher" field (empty string if absent).
	getMatcher := func(item interface{}) string {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}
		m, _ := obj["matcher"].(string)
		return m
	}

	// Pass 1: rewrite legacy → currentCmd in-place (preserves position and extra fields).
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
			// else: drop duplicate — do not append.
		} else {
			result = append(result, item)
		}
	}

	// Pass 3: if no entry was found/kept, append a new one.
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
// comparator treats as significant (see TestGBGDedup_Cursor_BothGloballyInstalled_
// KeyAbsentNotEmpty).
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

// migrateDedupGuardInNestedArray — reserved for ML-1B (Claude/Codex/Gemini injection sites).
//
// The nested-array format used by Claude/Codex/Gemini has outer entries of the form
// {"matcher":"…","hooks":[…]}. ML-1A only covers Cursor (flat format); this helper
// will be implemented and tested in ML-1B when the remaining injection sites are
// migrated. It is intentionally absent from ML-1A to avoid shipping untested code.
//
// Do not add this function until ML-1B lands — callers will be added in that batch.

