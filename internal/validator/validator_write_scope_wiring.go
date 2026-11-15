package validator

// validator_write_scope_wiring.go — ML-3A (REQ-2026-10-23, ADR-2026-10-23) + ML-2C (REQ-2026-11-13)
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
//
// D18 migration (ML-2C / REQ-2026-11-13): when the hook has a WriteScopeLegacyPSPOSIX
// form, it is treated as "wired" (no write_scope_not_wired violation) and an always-warning
// D18 migration notice is emitted via applyRuleWarnOnly under the separate rule key
// write_scope_guard_hook_resolvable in both validator.go paths. The separation ensures
// that a user silencing write_scope_not_wired does not accidentally silence the migration
// notice, and that tooling can distinguish "hook absent" from "hook needs migration".

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

const writeScopeWiringMatcher = "Write|Edit|NotebookEdit"

// writeScopeHookStatus is the tri-state result of settingsWriteScopeHookStatus.
type writeScopeHookStatus int

const (
	writeScopeHookAbsent   writeScopeHookStatus = iota // no matching hook or file absent
	writeScopeHookLegacyD18                            // D18 legacy form found (pré-REQ-2026-11-13)
	writeScopeHookCurrent                              // current K form (WriteScopeCurrentPSPOSIX)
)

// validateWriteScopeWiring checks whether .claude/settings.json has the write-scope guard
// hook wired in PreToolUse[Write|Edit|NotebookEdit].
// Returns a diagnostic message (slice of length 1) if absent or wrong form, nil if current or legacy.
// Legacy D18 form is handled separately by validateWriteScopeWiringD18LegacyMigration.
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

	status := settingsWriteScopeHookStatus(root)
	switch status {
	case writeScopeHookCurrent, writeScopeHookLegacyD18:
		// Legacy D18 is "wired" — the actual migration warning is emitted by
		// validateWriteScopeWiringD18LegacyMigration via applyRuleWarnOnly.
		return nil
	default:
		return []string{fmt.Sprintf(
			"write_scope_not_wired: %s PreToolUse[%s] missing trackfw guard write-scope (run `trackfw init` or `trackfw update`)",
			path, writeScopeWiringMatcher,
		)}
	}
}

// validateWriteScopeWiringD18LegacyMigration returns an always-warning (never violation)
// migration notice when .claude/settings.json PreToolUse[Write|Edit|NotebookEdit] has a
// D18 legacy form (WriteScopeLegacyPSPOSIX). Routed via applyRuleWarnOnly under the rule
// key write_scope_guard_hook_resolvable (NOT write_scope_not_wired) in validator.go.
func validateWriteScopeWiringD18LegacyMigration() []string {
	path := ".claude/settings.json"
	// raw-read-allowed: settings.json is user-owned JSON config (not a governance YAML artifact); FIFO here is exotic; JSON decoder handles malformed safely; unreadable → no warning (not our error to surface).
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // absent → handled by validateWriteScopeWiring
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	if settingsWriteScopeHookStatus(root) != writeScopeHookLegacyD18 {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s PreToolUse[%s] has D18 legacy write-scope form (pré-REQ-2026-11-13, "+
			"exits 127 under inherited errexit/nounset in POSIX shells); "+
			"run `trackfw update` to migrate to the current form",
		path, writeScopeWiringMatcher,
	)}
}

// settingsWriteScopeHookStatus returns the tri-state status of the write-scope hook in
// the parsed settings root (writeScopeHookCurrent, writeScopeHookLegacyD18, or writeScopeHookAbsent).
func settingsWriteScopeHookStatus(root map[string]json.RawMessage) writeScopeHookStatus {
	rawHooks, ok := root["hooks"]
	if !ok {
		return writeScopeHookAbsent
	}
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(rawHooks, &hooks); err != nil {
		return writeScopeHookAbsent
	}
	rawPTU, ok := hooks["PreToolUse"]
	if !ok {
		return writeScopeHookAbsent
	}
	var ptu []json.RawMessage
	if err := json.Unmarshal(rawPTU, &ptu); err != nil {
		return writeScopeHookAbsent
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
		// Matcher matches — check hooks array for a known command.
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
			trimmed := strings.TrimSpace(cmd)
			if trimmed == guardforms.WriteScopeCurrentPSPOSIX {
				return writeScopeHookCurrent
			}
			for _, legacy := range guardforms.WriteScopeLegacyPSPOSIX {
				if trimmed == legacy {
					return writeScopeHookLegacyD18
				}
			}
		}
	}
	return writeScopeHookAbsent
}

// settingsHasWriteScopeHook is retained for backward compatibility with existing tests.
// New code should prefer settingsWriteScopeHookStatus.
func settingsHasWriteScopeHook(root map[string]json.RawMessage) bool {
	s := settingsWriteScopeHookStatus(root)
	return s == writeScopeHookCurrent || s == writeScopeHookLegacyD18
}
