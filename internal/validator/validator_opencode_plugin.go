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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/opencodeplugin"
)

// openCodePluginRelPath is the relative path of the project-scoped guard plugin
// under the project root.
const openCodePluginRelPath = ".opencode/plugins/trackfw-guard.js"

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

	pluginPath := filepath.Join(root, openCodePluginRelPath)
	content, readErr := readRegularFile(pluginPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return []string{fmt.Sprintf(
				".opencode/ detected but %s is absent — run `trackfw update`",
				openCodePluginRelPath,
			)}, nil
		}
		// Unreadable (special file, permission error) — treat as absent.
		return []string{fmt.Sprintf(
			"%s is not readable — run `trackfw update`",
			openCodePluginRelPath,
		)}, nil
	}

	// Check marker on first line.
	normalized := integrations.NormalizeCRLF(content)
	firstLine := string(normalized)
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	if firstLine != opencodeplugin.Marker {
		return []string{fmt.Sprintf(
			"%s is present but missing the trackfw:opencode-guard marker — "+
				"run `trackfw update` to refresh it",
			openCodePluginRelPath,
		)}, nil
	}

	// Check staleness against canonical render.
	canonical, renderErr := opencodeplugin.Render("project")
	if renderErr != nil {
		return nil, renderErr
	}
	canonicalNorm := integrations.NormalizeCRLF(canonical)
	if !bytes.Equal(normalized, canonicalNorm) {
		return []string{fmt.Sprintf(
			"%s is stale — run `trackfw update` to refresh it",
			openCodePluginRelPath,
		)}, nil
	}

	return nil, nil
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
	pluginPath := filepath.Join(root, openCodePluginRelPath)
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
