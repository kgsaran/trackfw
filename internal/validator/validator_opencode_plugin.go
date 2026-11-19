package validator

// validator_opencode_plugin.go — ML-5E (REQ-2026-11-17)
//
// Rule: credential_guard_presence_warning (shared with D17)
//
// Project-side check (D5):
//   When a project has a .opencode/ directory (OpenCode is in use) AND the plugin
//   .opencode/plugins/trackfw-guard.js is absent, missing the marker, or stale,
//   emit a warning. Used via applyRuleWarnOnly("credential_guard_presence_warning").
//
// Global-side advisory (D6, doctor only):
//   When the project plugin is present AND the global OpenCode plugin
//   (<openCodeConfigDir>/plugins/trackfw-guard.js) is absent, emit an informative
//   finding for `doctor`. Not wired into validate — validate checks only the project side.
//
// Logic (project, D5):
//   - .opencode/ directory absent → silent (OpenCode not in use in this project).
//   - .opencode/ present, plugin present, marker present, up-to-date → silent.
//   - .opencode/ present, plugin absent → warn "absent".
//   - .opencode/ present, plugin present, marker absent → warn "missing marker".
//   - .opencode/ present, plugin present, marker present, stale → warn "stale".
//
// Logic (global, D6, doctor only):
//   - project plugin absent (or no marker) → silent (don't recommend global if project is unset).
//   - home unresolvable → silent.
//   - global plugin present (with marker) → silent.
//   - all conditions: project has guard AND global lacks guard → advisory.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/opencodeplugin"
)

// openCodeDirRelPath is the directory whose presence signals that OpenCode is in use
// in this project.
const openCodeDirRelPath = ".opencode"

// validateOpenCodePluginWarningsWith is the resolver-injectable entry point.
// It checks the project-scoped plugin under root using the given CLIHomeResolver.
// N3: the resolver is now used (not discarded) to also inspect the global
// OpenCode config directory for sibling plugin files and plugin config keys.
// N3-cond (ML-5I7): global checks run unconditionally — even when .opencode/ is absent
// in the project — so that a global evil.js or config-key is always detected.
func validateOpenCodePluginWarningsWith(root string, r agentconfig.CLIHomeResolver) ([]string, error) {
	// N3-cond: resolve globalDir before the .opencode/ early return so that the global
	// sibling and config-key checks run even when the project has no .opencode/ directory.
	// "" means unresolvable — skip global dirs.
	globalDir, _ := agentconfig.OpenCodeConfigDir(r)

	var warns []string

	// F4 fix b + N3 + N3-cond: warn on sibling JS/TS/MJS/CJS files in project and global
	// plugin dirs. Project dirs are silently skipped by os.ReadDir when absent (no .opencode/).
	warns = append(warns, warnSiblingPluginFiles(root, globalDir)...)

	// F4 fix b + N3 + N3-cond: warn on "plugin"/"plugins" keys in project and global
	// opencode.json(c). Project files silently skipped by readRegularFile when absent.
	warns = append(warns, warnOpenCodeConfigKeys(root, globalDir)...)

	// Condition: .opencode/ must be a directory — otherwise project-side checks are skipped.
	// Global warns (if any) are still returned.
	opencodeDir := filepath.Join(root, openCodeDirRelPath)
	fi, err := os.Stat(opencodeDir)
	if err != nil || !fi.IsDir() {
		if len(warns) == 0 {
			return nil, nil // OpenCode not in use and no global warnings — silent
		}
		return warns, nil
	}

	// Main plugin presence / marker / staleness check.
	pluginPath := filepath.Join(root, opencodeplugin.PluginRelPath)
	content, readErr := readRegularFile(pluginPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			warns = append(warns, fmt.Sprintf(
				".opencode/ detected but %s is absent — run `trackfw update`",
				opencodeplugin.PluginRelPath,
			))
		} else {
			// Unreadable (special file, permission error) — treat as absent.
			warns = append(warns, fmt.Sprintf(
				"%s is not readable — run `trackfw update`",
				opencodeplugin.PluginRelPath,
			))
		}
	} else {
		// Check marker on first line.
		normalized := integrations.NormalizeCRLF(content)
		firstLine := string(normalized)
		if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		if firstLine != opencodeplugin.Marker {
			warns = append(warns, fmt.Sprintf(
				"%s is present but missing the trackfw:opencode-guard marker — "+
					"run `trackfw update` to refresh it",
				opencodeplugin.PluginRelPath,
			))
		} else {
			// Check staleness against canonical render.
			canonical, renderErr := opencodeplugin.Render("project")
			if renderErr != nil {
				return nil, renderErr
			}
			canonicalNorm := integrations.NormalizeCRLF(canonical)
			if !bytes.Equal(normalized, canonicalNorm) {
				warns = append(warns, fmt.Sprintf(
					"%s is stale — run `trackfw update` to refresh it",
					opencodeplugin.PluginRelPath,
				))
			}
		}
	}

	if len(warns) == 0 {
		return nil, nil
	}
	return warns, nil
}

// warnSiblingPluginFiles returns warnings for any .js/.ts/.mjs/.cjs file in
// plugin directories that is NOT trackfw-guard.js (D7 F4 fix b, N3).
//
// Project dirs checked: .opencode/plugins/ and .opencode/plugin/ under root.
// Global dirs checked (N3): <globalDir>/plugins/ and <globalDir>/plugin/ when
// globalDir is non-empty. When global dirs match the project dirs (unusual),
// the entry is reported once.
func warnSiblingPluginFiles(root, globalDir string) []string {
	type dirEntry struct {
		dir   string
		label string // display label used in warning message
	}
	dirs := []dirEntry{
		{filepath.Join(root, ".opencode", "plugins"), ".opencode/plugins"},
		{filepath.Join(root, ".opencode", "plugin"), ".opencode/plugin"},
	}
	if globalDir != "" {
		dirs = append(dirs,
			dirEntry{filepath.Join(globalDir, "plugins"), "global opencode plugins/"},
			dirEntry{filepath.Join(globalDir, "plugin"), "global opencode plugin/"},
		)
	}

	scriptExts := map[string]bool{
		".js":  true,
		".ts":  true,
		".mjs": true,
		".cjs": true,
	}
	var warns []string
	for _, de := range dirs {
		entries, dirErr := os.ReadDir(de.dir)
		if dirErr != nil {
			continue // dir absent or not readable — silent
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if name == "trackfw-guard.js" {
				continue
			}
			if scriptExts[strings.ToLower(filepath.Ext(name))] {
				warns = append(warns, fmt.Sprintf(
					"unexpected plugin script in %s%s — only trackfw-guard.js is expected; "+
						"remove or verify this file is intentional",
					de.label, name,
				))
			}
		}
	}
	return warns
}

// warnOpenCodeConfigKeys returns warnings when opencode.json or opencode.jsonc
// contains a top-level "plugin" or "plugins" key (D7 F4 fix b, N3).
//
// Project candidates: opencode.json(c) at the project root and under .opencode/.
// Global candidates (N3): opencode.json(c) directly under globalDir when globalDir
// is non-empty. The parse is JSONC-tolerant: stripJSONCLineComments strips //
// line comments and /* */ block comments (N4), and trailing commas before }/] are
// removed (N4) before json.Unmarshal, so comment-annotated JSONC files are handled.
func warnOpenCodeConfigKeys(root, globalDir string) []string {
	var warns []string
	type candidate struct {
		path    string
		display string // path shown in the warning (relative to root when possible)
	}
	candidates := []candidate{
		{filepath.Join(root, "opencode.json"), ""},
		{filepath.Join(root, "opencode.jsonc"), ""},
		{filepath.Join(root, ".opencode", "opencode.json"), ""},
		{filepath.Join(root, ".opencode", "opencode.jsonc"), ""},
	}
	if globalDir != "" {
		candidates = append(candidates,
			candidate{filepath.Join(globalDir, "opencode.json"), filepath.Join(globalDir, "opencode.json")},
			candidate{filepath.Join(globalDir, "opencode.jsonc"), filepath.Join(globalDir, "opencode.jsonc")},
		)
	}
	for _, c := range candidates {
		raw, err := readRegularFile(c.path)
		if err != nil {
			continue
		}
		stripped := stripJSONCLineComments(raw)
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(stripped, &obj); err != nil {
			continue // malformed — not our problem here
		}
		display := c.display
		if display == "" {
			rel, relErr := filepath.Rel(root, c.path)
			if relErr != nil {
				display = c.path
			} else {
				display = rel
			}
		}
		for _, key := range []string{"plugin", "plugins"} {
			if _, ok := obj[key]; ok {
				warns = append(warns, fmt.Sprintf(
					"%s contains a %q key — plugin loading should use the trackfw-managed %s",
					display, key, opencodeplugin.PluginRelPath,
				))
			}
		}
	}
	return warns
}

// stripJSONCLineComments removes comments and trailing commas from JSONC content
// to produce valid JSON that json.Unmarshal can parse. It handles:
//   - // line comments (outside strings): stripped to end of line.
//   - /* */ block comments (outside strings, N4): stripped entirely.
//   - Trailing commas before } or ] (N4): removed so JSONC-with-trailing-commas
//     parses correctly. The removal is applied after comment stripping.
//
// String literals are handled correctly: comment markers and commas inside
// quoted strings are not treated as JSONC syntax.
func stripJSONCLineComments(data []byte) []byte {
	// Pass 1: strip // and /* */ comments outside string literals.
	out := make([]byte, 0, len(data))
	inStr := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inStr {
			out = append(out, c)
			if c == '\\' && i+1 < len(data) {
				i++
				out = append(out, data[i])
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(data) {
			if data[i+1] == '/' {
				// Line comment: skip until end of line (preserve the newline).
				for i < len(data) && data[i] != '\n' {
					i++
				}
				if i < len(data) {
					out = append(out, '\n')
				}
				continue
			}
			if data[i+1] == '*' {
				// Block comment (N4): skip until closing */.
				i += 2
				for i < len(data) {
					if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
						i += 2
						break
					}
					i++
				}
				i-- // outer loop will i++ to the char after the block comment
				continue
			}
		}
		out = append(out, c)
	}

	// Pass 2: remove trailing commas before } or ] (N4).
	// We scan the comment-stripped output; at this point there are no JSONC
	// comments, so the only commas we care about are real JSON commas.
	// We use a simple string-unaware pass: a comma followed by optional
	// whitespace and then } or ] is a trailing comma. This is safe because
	// valid JSON does not have ,\s*[}\]] inside string values.
	result := make([]byte, 0, len(out))
	for i := 0; i < len(out); i++ {
		if out[i] == ',' {
			// Look ahead past whitespace.
			j := i + 1
			for j < len(out) && (out[j] == ' ' || out[j] == '\t' || out[j] == '\n' || out[j] == '\r') {
				j++
			}
			if j < len(out) && (out[j] == '}' || out[j] == ']') {
				// Trailing comma — skip it.
				continue
			}
		}
		result = append(result, out[i])
	}
	return result
}

// validateOpenCodePluginWarnings is the production entry point (DefaultResolver).
func validateOpenCodePluginWarnings(root string) ([]string, error) {
	return validateOpenCodePluginWarningsWith(root, agentconfig.DefaultResolver)
}

// ValidateOpenCodePluginWarningsForDoctor is the exported entry point used by
// internal/commands/doctor.go. Errors are silently discarded — doctor reports by finding.
func ValidateOpenCodePluginWarningsForDoctor(projectRoot string) []string {
	msgs, _ := validateOpenCodePluginWarnings(projectRoot)
	return msgs
}

// ValidateOpenCodePluginWarningsWithForTest is the resolver-injectable entry point
// used by external tests. Injecting an agentconfig.FixedCLIHomeResolver ensures tests
// never touch the real OpenCode config directory.
func ValidateOpenCodePluginWarningsWithForTest(root string, r agentconfig.CLIHomeResolver) []string {
	msgs, _ := validateOpenCodePluginWarningsWith(root, r)
	return msgs
}

// validateOpenCodePluginGlobalAbsentWith is the resolver-injectable D6 doctor check.
// Returns a warning message when the project plugin is present (with marker) but the
// global OpenCode plugin is absent. Used only by doctor, not by validate.
func validateOpenCodePluginGlobalAbsentWith(root string, r agentconfig.CLIHomeResolver) ([]string, error) {
	// Condition 1: project plugin must be present and have the marker.
	pluginPath := filepath.Join(root, opencodeplugin.PluginRelPath)
	content, readErr := readRegularFile(pluginPath)
	if readErr != nil {
		return nil, nil // project plugin absent — no D6 advisory
	}
	normalized := integrations.NormalizeCRLF(content)
	firstLine := string(normalized)
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	if firstLine != opencodeplugin.Marker {
		return nil, nil // marker absent — project guard not effective, skip D6
	}

	// Condition 2: resolve global OpenCode config dir.
	globalDir, err := agentconfig.OpenCodeConfigDir(r)
	if err != nil {
		return nil, nil // cannot resolve home — silent
	}

	// Condition 3: global plugin absent or lacking marker.
	globalPluginPath := filepath.Join(globalDir, "plugins", "trackfw-guard.js")
	globalContent, globalReadErr := readRegularFile(globalPluginPath)
	if globalReadErr == nil {
		// File exists — check marker.
		gnorm := integrations.NormalizeCRLF(globalContent)
		gfirst := string(gnorm)
		if idx := strings.IndexByte(gfirst, '\n'); idx >= 0 {
			gfirst = gfirst[:idx]
		}
		if gfirst == opencodeplugin.Marker {
			return nil, nil // global guard present and valid — silent
		}
	}

	displayPath := agentconfig.OpenCodePluginDisplayPath(globalDir)
	return []string{
		"OpenCode project plugin is installed, but the global guard is absent — " +
			"run `trackfw update harness --targets opencode-guard --install-missing`" +
			" (" + displayPath + ")",
	}, nil
}

// validateOpenCodePluginGlobalAbsent is the production entry point for D6 (doctor).
func validateOpenCodePluginGlobalAbsent(root string) ([]string, error) {
	return validateOpenCodePluginGlobalAbsentWith(root, agentconfig.DefaultResolver)
}

// ValidateOpenCodePluginGlobalAbsentForDoctor is the exported entry point for
// internal/commands/doctor.go D6 check. Errors are silently discarded.
func ValidateOpenCodePluginGlobalAbsentForDoctor(projectRoot string) []string {
	msgs, _ := validateOpenCodePluginGlobalAbsent(projectRoot)
	return msgs
}

// ValidateOpenCodePluginGlobalAbsentWithForTest is the resolver-injectable entry point
// for external tests.
func ValidateOpenCodePluginGlobalAbsentWithForTest(root string, r agentconfig.CLIHomeResolver) []string {
	msgs, _ := validateOpenCodePluginGlobalAbsentWith(root, r)
	return msgs
}
