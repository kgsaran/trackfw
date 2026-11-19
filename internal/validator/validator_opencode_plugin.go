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
func validateOpenCodePluginWarningsWith(root string, _ agentconfig.CLIHomeResolver) ([]string, error) {
	// Condition: .opencode/ must be a directory — otherwise OpenCode is not in use here.
	opencodeDir := filepath.Join(root, openCodeDirRelPath)
	fi, err := os.Stat(opencodeDir)
	if err != nil || !fi.IsDir() {
		return nil, nil // OpenCode not in use — silent
	}

	var warns []string

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

	// F4 fix b: warn on sibling JS/TS/MJS/CJS files in plugin dirs.
	warns = append(warns, warnSiblingPluginFiles(root)...)

	// F4 fix b: warn on "plugin" or "plugins" keys in opencode.json(c).
	warns = append(warns, warnOpenCodeConfigKeys(root)...)

	if len(warns) == 0 {
		return nil, nil
	}
	return warns, nil
}

// warnSiblingPluginFiles returns warnings for any .js/.ts/.mjs/.cjs file in
// .opencode/plugins/ or .opencode/plugin/ that is NOT trackfw-guard.js (D7 F4 fix b).
func warnSiblingPluginFiles(root string) []string {
	var warns []string
	pluginDirs := []string{
		filepath.Join(root, ".opencode", "plugins"),
		filepath.Join(root, ".opencode", "plugin"),
	}
	scriptExts := map[string]bool{
		".js":  true,
		".ts":  true,
		".mjs": true,
		".cjs": true,
	}
	for _, dir := range pluginDirs {
		entries, dirErr := os.ReadDir(dir)
		if dirErr != nil {
			continue // dir absent or not readable — silent
		}
		rel := filepath.Base(dir) // "plugins" or "plugin"
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
					"unexpected plugin script .opencode/%s/%s — only trackfw-guard.js is expected; "+
						"remove or verify this file is intentional",
					rel, name,
				))
			}
		}
	}
	return warns
}

// warnOpenCodeConfigKeys returns warnings when opencode.json or opencode.jsonc
// (project root or .opencode/) contains a top-level "plugin" or "plugins" key
// (D7 F4 fix b). The parse is JSONC-tolerant (strips // line comments before
// unmarshalling so comment-annotated config files are handled correctly).
func warnOpenCodeConfigKeys(root string) []string {
	var warns []string
	candidates := []string{
		filepath.Join(root, "opencode.json"),
		filepath.Join(root, "opencode.jsonc"),
		filepath.Join(root, ".opencode", "opencode.json"),
		filepath.Join(root, ".opencode", "opencode.jsonc"),
	}
	for _, path := range candidates {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		stripped := stripJSONCLineComments(raw)
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(stripped, &obj); err != nil {
			continue // malformed — not our problem here
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		for _, key := range []string{"plugin", "plugins"} {
			if _, ok := obj[key]; ok {
				warns = append(warns, fmt.Sprintf(
					"%s contains a %q key — plugin loading should use the trackfw-managed %s",
					rel, key, opencodeplugin.PluginRelPath,
				))
			}
		}
	}
	return warns
}

// stripJSONCLineComments removes // line comments from JSONC content.
// It handles string literals correctly (skips // inside quoted strings) but
// does not handle block comments or multi-line strings — sufficient for
// opencode.json(c) which uses only single-line // comments.
func stripJSONCLineComments(data []byte) []byte {
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
		if c == '/' && i+1 < len(data) && data[i+1] == '/' {
			// Skip until end of line (keep the newline).
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
			continue
		}
		out = append(out, c)
	}
	return out
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
