// Package hookschema provides a shared walker for the Google Antigravity hook
// file schema (ADR-2026-11-18 D6, ML-4C REQ-2026-11-17).
//
// Schema layout:
//
//	{
//	  "<groupName>": {
//	    "<eventName>": [
//	      {
//	        "matcher": "<matcher>",
//	        "hooks":   [{"type": "command", "command": "<cmd>"}]
//	      }
//	    ]
//	  }
//	}
//
// # Motivation (A3)
//
// Before this package, three sites independently implemented the same 3-level
// traversal (group → event → []entry → hooks[].command):
//   - internal/auditsurface/auditsurface.go  (~line 723)
//   - internal/validator/validator_credential_guard.go (~line 366)
//   - internal/validator/validator_guard_wiring.go  (~line 329)
//
// Divergence between the three would produce silent inconsistencies: for
// example, auditsurface might report a hook entry as present while the
// validator fails to detect it. A single traversal eliminates that risk.
//
// # Behaviour change relative to auditsurface's old implementation
//
// The old extractAntigravitySchema in auditsurface used a typed unmarshal
// (json.RawMessage → map[string][]matcherEntry). This caused the entire group
// to be silently skipped when ANY entry in the group had an unexpected type,
// including sibling keys like {"enabled": false}. As a result, a group
// containing a hostile hook entry alongside a malformed sibling could hide
// the hostile entry from the audit surface.
//
// WalkAntigravityEntries uses per-element type assertions: each entry and each
// hook is processed individually. A malformed sibling no longer hides valid
// (and potentially hostile) entries in the same group.
package hookschema

// EntryFunc is called by WalkAntigravityEntries for every (group, event,
// matcher, command, typeStr) tuple found in root. typeStr is the value of the
// "type" key in the hook object; it is "command" for well-formed entries.
// Implementations must not retain references to the string arguments after
// the function returns.
type EntryFunc func(group, event, matcher, cmd, typeStr string)

// WalkAntigravityEntries iterates every entry in an already-parsed Antigravity
// hook JSON object (top level = map[string]any) and calls fn for each
// (group, event, matcher, hook.command, hook.type) tuple found.
//
// Traversal order: document order for groups, events, entries, and hooks.
// If deterministic output order is required (e.g. for auditsurface's tuple
// fingerprint), the caller is responsible for sorting after collection.
//
// Groups whose value is not map[string]any, events whose value is not
// []any, entries whose value is not map[string]any, and hooks without a
// non-empty "command" string are silently skipped — the remaining entries
// in the same group/event/entry are still processed.
func WalkAntigravityEntries(root map[string]any, fn EntryFunc) {
	for groupName, groupRaw := range root {
		groupObj, ok := groupRaw.(map[string]any)
		if !ok {
			continue
		}
		for eventName, entriesRaw := range groupObj {
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
				hooks, _ := entry["hooks"].([]any)
				for _, hookRaw := range hooks {
					h, ok := hookRaw.(map[string]any)
					if !ok {
						continue
					}
					cmd, _ := h["command"].(string)
					if cmd == "" {
						continue
					}
					typeStr, _ := h["type"].(string)
					fn(groupName, eventName, matcher, cmd, typeStr)
				}
			}
		}
	}
}
