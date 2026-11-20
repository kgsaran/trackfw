package hookschema

// hookschema_test.go — unit tests for WalkAntigravityEntries (A3, ML-4C REQ-2026-11-17).
//
// Conclusion asserted (Regra Dura de Reconciliação):
//   WalkAntigravityEntries yields all (group, event, matcher, cmd, typeStr)
//   tuples from a valid Antigravity root map, skips entries/hooks with
//   unexpected shapes, and does NOT skip entries that share a group with a
//   malformed sibling — closing the hiding-spot identified in A3.

import (
	"encoding/json"
	"testing"
)

type tuple struct {
	group, event, matcher, cmd, typeStr string
}

func collectTuples(root map[string]any) []tuple {
	var out []tuple
	WalkAntigravityEntries(root, func(group, event, matcher, cmd, typeStr string) {
		out = append(out, tuple{group, event, matcher, cmd, typeStr})
	})
	return out
}

// parseRoot unmarshals JSON into map[string]any for test fixtures.
func parseRoot(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("parseRoot: %v", err)
	}
	return m
}

// TestWalkAntigravityEntries_HappyPath asserts that a well-formed Antigravity
// root yields exactly the expected tuples.
//
// Conclusion asserted: WalkAntigravityEntries extracts all (group, event,
// matcher, cmd, typeStr) tuples from a valid hooks.json root.
func TestWalkAntigravityEntries_HappyPath(t *testing.T) {
	root := parseRoot(t, `{
		"trackfw": {
			"PreToolUse": [
				{
					"matcher": "run_command",
					"hooks": [{"type": "command", "command": "trackfw guard credential || exit 2"}]
				},
				{
					"matcher": "view_file",
					"hooks": [{"type": "command", "command": "trackfw guard credential || exit 2"}]
				}
			]
		}
	}`)

	tuples := collectTuples(root)
	if len(tuples) != 2 {
		t.Fatalf("want 2 tuples, got %d: %v", len(tuples), tuples)
	}
	for _, tup := range tuples {
		if tup.group != "trackfw" {
			t.Errorf("group = %q, want %q", tup.group, "trackfw")
		}
		if tup.event != "PreToolUse" {
			t.Errorf("event = %q, want %q", tup.event, "PreToolUse")
		}
		if tup.typeStr != "command" {
			t.Errorf("typeStr = %q, want %q", tup.typeStr, "command")
		}
	}
}

// TestWalkAntigravityEntries_MalformedSiblingDoesNotHideEntry asserts that an
// entry with a malformed sibling key (e.g. "enabled": false) in the same
// group is still yielded.
//
// Conclusion asserted: per-element processing means a malformed sibling cannot
// hide a valid (and potentially hostile) hook entry — behaviour change vs. the
// old typed-unmarshal approach documented in package doc.
func TestWalkAntigravityEntries_MalformedSiblingDoesNotHideEntry(t *testing.T) {
	// "third_party" group has "enabled": false next to "PreToolUse" with a valid entry.
	root := parseRoot(t, `{
		"third_party": {
			"enabled": false,
			"PreToolUse": [
				{
					"matcher": "run_command",
					"hooks": [{"type": "command", "command": "hostile-script || exit 0"}]
				}
			]
		}
	}`)

	tuples := collectTuples(root)
	if len(tuples) != 1 {
		t.Fatalf("want 1 tuple (sibling malformed key must not hide entry), got %d: %v", len(tuples), tuples)
	}
	if tuples[0].cmd != "hostile-script || exit 0" {
		t.Errorf("cmd = %q, want %q", tuples[0].cmd, "hostile-script || exit 0")
	}
}

// TestWalkAntigravityEntries_EmptyCommandSkipped asserts that a hook object
// with an empty or missing "command" field is silently skipped.
//
// Conclusion asserted: hooks without a non-empty command string are not
// yielded, preventing empty-string false positives in callers.
func TestWalkAntigravityEntries_EmptyCommandSkipped(t *testing.T) {
	root := parseRoot(t, `{
		"trackfw": {
			"PreToolUse": [
				{
					"matcher": "run_command",
					"hooks": [
						{"type": "command", "command": ""},
						{"type": "command"}
					]
				}
			]
		}
	}`)

	tuples := collectTuples(root)
	if len(tuples) != 0 {
		t.Fatalf("want 0 tuples (empty commands skipped), got %d: %v", len(tuples), tuples)
	}
}

// TestWalkAntigravityEntries_NonObjectGroupSkipped asserts that a group whose
// value is not a map (e.g. a boolean) is silently skipped without panicking.
//
// Conclusion asserted: non-map group values are skipped safely.
func TestWalkAntigravityEntries_NonObjectGroupSkipped(t *testing.T) {
	root := parseRoot(t, `{
		"bad_group": true,
		"trackfw": {
			"PreToolUse": [
				{"matcher": "run_command", "hooks": [{"type":"command","command":"cmd"}]}
			]
		}
	}`)

	tuples := collectTuples(root)
	if len(tuples) != 1 {
		t.Fatalf("want 1 tuple (bad_group skipped), got %d: %v", len(tuples), tuples)
	}
}
