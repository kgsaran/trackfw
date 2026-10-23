package validator

// validator_write_scope_wiring.go — ML-3A (REQ-2026-10-23, ADR-2026-10-23)
//
// Rule: write_scope_not_wired
//
// Warns when .claude/settings.json does not have a PreToolUse entry with
// matcher "Write|Edit|NotebookEdit" and the write-scope guard command.
//
// Default severity: "warning" (ruleDefaults). The rule is not a violation until the
// next major version, per ML-3A contract item 7.
//
// This is a disk-only check: no origin/main anchor (the hook is new and won't exist
// in any existing main branch yet). Absent .claude/settings.json → warning.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

const writeScopeWiringMatcher = "Write|Edit|NotebookEdit"

// validateWriteScopeWiring checks whether .claude/settings.json has the write-scope guard
// hook wired in PreToolUse[Write|Edit|NotebookEdit].
// Returns a diagnostic message (slice of length 1) if absent, nil otherwise.
func validateWriteScopeWiring() []string {
	path := ".claude/settings.json"
	// raw-read-allowed: settings.json is user-owned JSON config (not a governance YAML artifact); FIFO here is exotic; JSON decoder handles malformed safely; unreadable → warning only.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{fmt.Sprintf(
				"write_scope_not_wired: %s absent — trackfw guard write-scope is not installed (run `trackfw init` or `trackfw update`)",
				path,
			)}
		}
		return []string{fmt.Sprintf("write_scope_not_wired: cannot read %s: %v", path, err)}
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return []string{fmt.Sprintf("write_scope_not_wired: %s is not valid JSON: %v", path, err)}
	}

	if !settingsHasWriteScopeHook(root) {
		return []string{fmt.Sprintf(
			"write_scope_not_wired: %s PreToolUse[%s] missing trackfw guard write-scope (run `trackfw init` or `trackfw update`)",
			path, writeScopeWiringMatcher,
		)}
	}
	return nil
}

// settingsHasWriteScopeHook reports whether the parsed settings root has a PreToolUse entry
// with matcher "Write|Edit|NotebookEdit" containing the canonical write-scope command.
func settingsHasWriteScopeHook(root map[string]json.RawMessage) bool {
	rawHooks, ok := root["hooks"]
	if !ok {
		return false
	}
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(rawHooks, &hooks); err != nil {
		return false
	}
	rawPTU, ok := hooks["PreToolUse"]
	if !ok {
		return false
	}
	var ptu []json.RawMessage
	if err := json.Unmarshal(rawPTU, &ptu); err != nil {
		return false
	}
	for _, entry := range ptu {
		var e map[string]json.RawMessage
		if err := json.Unmarshal(entry, &e); err != nil {
			continue
		}
		// Check matcher.
		var matcher string
		if err := json.Unmarshal(e["matcher"], &matcher); err != nil {
			continue
		}
		if matcher != writeScopeWiringMatcher {
			continue
		}
		// Matcher matches — check hooks array for the canonical command.
		rawInner, ok := e["hooks"]
		if !ok {
			continue
		}
		var inner []json.RawMessage
		if err := json.Unmarshal(rawInner, &inner); err != nil {
			continue
		}
		for _, h := range inner {
			var ho map[string]json.RawMessage
			if err := json.Unmarshal(h, &ho); err != nil {
				continue
			}
			var cmd string
			if err := json.Unmarshal(ho["command"], &cmd); err != nil {
				continue
			}
			if strings.TrimSpace(cmd) == guardforms.WriteScopeCurrentPSPOSIX {
				return true
			}
		}
	}
	return false
}
