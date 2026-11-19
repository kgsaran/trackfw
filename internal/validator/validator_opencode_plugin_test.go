package validator_test

// validator_opencode_plugin_test.go — ML-5E2 (REQ-2026-11-17)
//
// Tests for the credential_guard_presence_warning rule — OpenCode plugin side.
//
// All tests use t.TempDir() for HOME and isolate OPENCODE_CONFIG_DIR /
// XDG_CONFIG_HOME via t.Setenv so they never touch the real user's config.
// (main_test.go already unsets these env vars before the suite runs.)
//
// Reconciliation rule (CLAUDE.md): each test carries a one-sentence "Conclusion"
// stating which assertion of this ML it makes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/opencodeplugin"
	"github.com/kgsaran/trackfw/internal/validator"
)

// canonicalPluginBytes returns the canonical project-scoped plugin content.
// Panics on error (should never happen unless the asset is malformed).
func canonicalPluginBytes() []byte {
	return opencodeplugin.RenderProject()
}

// writeOpenCodeDir creates .opencode/ in projectDir as a directory.
func writeOpenCodeDir(t *testing.T, projectDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0755); err != nil {
		t.Fatalf("MkdirAll .opencode: %v", err)
	}
}

// writeCanonicalPlugin writes the canonical project plugin at the expected path.
func writeCanonicalPlugin(t *testing.T, projectDir string) {
	t.Helper()
	pluginDir := filepath.Join(projectDir, ".opencode", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("MkdirAll plugins dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "trackfw-guard.js"), canonicalPluginBytes(), 0644); err != nil {
		t.Fatalf("write trackfw-guard.js: %v", err)
	}
}

// writeStalePlugin writes a plugin file that has the Marker but differs from canonical.
func writeStalePlugin(t *testing.T, projectDir string) {
	t.Helper()
	pluginDir := filepath.Join(projectDir, ".opencode", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("MkdirAll plugins dir: %v", err)
	}
	content := opencodeplugin.Marker + "\n// stale — old version\n"
	if err := os.WriteFile(filepath.Join(pluginDir, "trackfw-guard.js"), []byte(content), 0644); err != nil {
		t.Fatalf("write stale plugin: %v", err)
	}
}

// writeNoMarkerPlugin writes a plugin file without the trackfw Marker.
func writeNoMarkerPlugin(t *testing.T, projectDir string) {
	t.Helper()
	pluginDir := filepath.Join(projectDir, ".opencode", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("MkdirAll plugins dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "trackfw-guard.js"), []byte("// custom plugin\nconsole.log('hi')\n"), 0644); err != nil {
		t.Fatalf("write no-marker plugin: %v", err)
	}
}

// isolatedResolver returns a FixedCLIHomeResolver backed by an isolated temp home.
func isolatedResolver(t *testing.T) agentconfig.FixedCLIHomeResolver {
	t.Helper()
	return agentconfig.FixedCLIHomeResolver{Home: t.TempDir()}
}

// --------------------------------------------------------------------------
// Project-side checks (D5)
// --------------------------------------------------------------------------

// TestOpenCodePlugin_NoOpenCodeDir_Silent asserts that when .opencode/ is absent,
// the rule emits no warning — OpenCode is not in use in this project.
//
// Conclusion: validateOpenCodePluginWarningsWith is silent when .opencode/ does not exist.
func TestOpenCodePlugin_NoOpenCodeDir_Silent(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected silence when .opencode/ absent; got: %v", msgs)
	}
}

// TestOpenCodePlugin_PluginAbsent_Warns asserts that when .opencode/ is present but
// the plugin file is absent, the rule emits a warning whose remedy points to `trackfw update`.
//
// Conclusion: absent plugin file triggers a warning with `trackfw update` as the remedy
// (not `trackfw update harness`) — A2 fix for ML-5E2.
func TestOpenCodePlugin_PluginAbsent_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	if len(msgs) == 0 {
		t.Fatal("expected warning when .opencode/ present but plugin absent; got none")
	}
	if !strings.Contains(msgs[0], "trackfw update") {
		t.Errorf("remedy must contain 'trackfw update'; got: %q", msgs[0])
	}
	if strings.Contains(msgs[0], "update harness") {
		t.Errorf("project remedy must NOT say 'update harness'; got: %q", msgs[0])
	}
}

// TestOpenCodePlugin_Stale_Warns asserts that when the plugin has the Marker but
// differs from the canonical render, the rule emits a "stale" warning pointing to `trackfw update`.
//
// Conclusion: stale plugin triggers a warning with `trackfw update` as the remedy
// (not `trackfw update harness`) — A2 fix for ML-5E2.
func TestOpenCodePlugin_Stale_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeStalePlugin(t, projectDir)

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	if len(msgs) == 0 {
		t.Fatal("expected warning for stale plugin; got none")
	}
	if !strings.Contains(msgs[0], "stale") {
		t.Errorf("message must mention 'stale'; got: %q", msgs[0])
	}
	if !strings.Contains(msgs[0], "trackfw update") {
		t.Errorf("remedy must contain 'trackfw update'; got: %q", msgs[0])
	}
	if strings.Contains(msgs[0], "update harness") {
		t.Errorf("project remedy must NOT say 'update harness'; got: %q", msgs[0])
	}
}

// TestOpenCodePlugin_MissingMarker_Warns asserts that a plugin file without the
// trackfw:opencode-guard Marker triggers a warning whose remedy points to `trackfw update`.
//
// Conclusion: plugin present without Marker triggers a warning with `trackfw update`
// as the remedy (not `trackfw update harness`) — A2 fix for ML-5E2.
func TestOpenCodePlugin_MissingMarker_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeNoMarkerPlugin(t, projectDir)

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	if len(msgs) == 0 {
		t.Fatal("expected warning for plugin without Marker; got none")
	}
	if !strings.Contains(msgs[0], "marker") {
		t.Errorf("message must mention 'marker'; got: %q", msgs[0])
	}
	if !strings.Contains(msgs[0], "trackfw update") {
		t.Errorf("remedy must contain 'trackfw update'; got: %q", msgs[0])
	}
	if strings.Contains(msgs[0], "update harness") {
		t.Errorf("project remedy must NOT say 'update harness'; got: %q", msgs[0])
	}
}

// TestOpenCodePlugin_Canonical_Silent asserts that when the plugin matches the
// canonical project-scoped render exactly, the rule emits no warning.
//
// Conclusion: canonical plugin content → silent (up-to-date, no warning emitted).
func TestOpenCodePlugin_Canonical_Silent(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected silence for canonical plugin; got: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// Global-side check (D6, doctor only)
// --------------------------------------------------------------------------

// TestOpenCodePlugin_GlobalAbsent_Warns asserts that when the project plugin is
// present (with Marker) but the global plugin is absent, the D6 check emits a warning
// whose remedy points to `trackfw update harness --targets opencode-guard --install-missing`.
//
// Conclusion: D6 advisory fires when project plugin is installed but global plugin is absent,
// and the remedy correctly names `trackfw update harness --targets opencode-guard`.
func TestOpenCodePlugin_GlobalAbsent_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)
	// No global plugin written.

	msgs := validator.ValidateOpenCodePluginGlobalAbsentWithForTest(projectDir, r)
	if len(msgs) == 0 {
		t.Fatal("expected D6 advisory when project plugin present but global absent; got none")
	}
	if !strings.Contains(msgs[0], "update harness") {
		t.Errorf("D6 remedy must say 'update harness'; got: %q", msgs[0])
	}
	if !strings.Contains(msgs[0], "opencode-guard") {
		t.Errorf("D6 remedy must name 'opencode-guard'; got: %q", msgs[0])
	}
}

// TestOpenCodePlugin_GlobalPresent_Silent asserts that when both project and global
// plugins are present (with Marker), the D6 check is silent.
//
// Conclusion: D6 advisory is silent when the global plugin has the Marker — user is protected.
func TestOpenCodePlugin_GlobalPresent_Silent(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	r := agentconfig.FixedCLIHomeResolver{Home: homeDir}
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	// Write global plugin (at ~/.config/opencode/plugins/trackfw-guard.js under homeDir).
	globalPluginDir := filepath.Join(homeDir, ".config", "opencode", "plugins")
	if err := os.MkdirAll(globalPluginDir, 0755); err != nil {
		t.Fatalf("MkdirAll global plugins dir: %v", err)
	}
	globalPlugin, err := opencodeplugin.Render("global")
	if err != nil {
		t.Fatalf("Render(global): %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalPluginDir, "trackfw-guard.js"), globalPlugin, 0644); err != nil {
		t.Fatalf("write global plugin: %v", err)
	}

	msgs := validator.ValidateOpenCodePluginGlobalAbsentWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected silence when global plugin present; got: %v", msgs)
	}
}

// TestOpenCodePlugin_ProjectAbsent_D6Silent asserts that the D6 advisory is silent
// when the project plugin is absent — the advisory should not recommend the global
// plugin when the project has not opted into the guard.
//
// Conclusion: D6 advisory is silent when the project plugin is absent — the advisory
// only fires when the project is already protected.
func TestOpenCodePlugin_ProjectAbsent_D6Silent(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	// No project plugin.

	msgs := validator.ValidateOpenCodePluginGlobalAbsentWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected D6 silence when project plugin absent; got: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// End-to-end: warning routes to warnings (not violations) in Validate()
// --------------------------------------------------------------------------

// TestOpenCodePlugin_ValidateForTest_Absent_IsWarning asserts that the absent-plugin
// warning routes to the warnings slice (not violations) in the full Validate() pipeline.
//
// Conclusion: credential_guard_presence_warning is applied via applyRuleWarnOnly;
// absent OpenCode plugin with .opencode/ present appears in warnings, not violations.
func TestOpenCodePlugin_ValidateForTest_Absent_IsWarning(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := setupConcordanceDir(t)
	writeOpenCodeDir(t, dir)

	violations, warnings, err := validator.ValidateForTest()
	if err != nil {
		t.Fatalf("ValidateForTest: %v", err)
	}

	const wantSubstr = ".opencode/plugins/trackfw-guard.js"
	for _, v := range violations {
		if strings.Contains(v, wantSubstr) {
			t.Errorf("opencode plugin warning appeared in violations (want warnings); got: %q", v)
		}
	}

	found := false
	for _, w := range warnings {
		if strings.Contains(w, wantSubstr) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected opencode plugin absent warning in warnings slice; warnings=%v", warnings)
	}
}
