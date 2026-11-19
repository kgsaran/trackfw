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
// F4 fix b: sibling plugin file detection
// --------------------------------------------------------------------------

// TestOpenCodePlugin_Sibling_JS_Warns asserts that a .js file in .opencode/plugins/
// other than trackfw-guard.js triggers a warning.
//
// Conclusion: warnSiblingPluginFiles detects unexpected .js siblings and returns a warning.
func TestOpenCodePlugin_Sibling_JS_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	// Add a sibling .js file.
	sibling := filepath.Join(projectDir, ".opencode", "plugins", "evil.js")
	if err := os.WriteFile(sibling, []byte("console.log('evil')"), 0644); err != nil {
		t.Fatalf("write sibling: %v", err)
	}

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "evil.js") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected sibling warning for evil.js; got: %v", msgs)
	}
}

// TestOpenCodePlugin_Sibling_TS_Warns asserts that a .ts file in .opencode/plugin/ (singular)
// triggers a warning.
//
// Conclusion: warnSiblingPluginFiles covers both plugins/ and plugin/ directories.
func TestOpenCodePlugin_Sibling_TS_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)

	// Use .opencode/plugin/ (singular) with a .ts file.
	pluginSingularDir := filepath.Join(projectDir, ".opencode", "plugin")
	if err := os.MkdirAll(pluginSingularDir, 0755); err != nil {
		t.Fatalf("MkdirAll plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginSingularDir, "other.ts"), []byte("export {}"), 0644); err != nil {
		t.Fatalf("write other.ts: %v", err)
	}

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "other.ts") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected sibling warning for other.ts in plugin/ dir; got: %v", msgs)
	}
}

// TestOpenCodePlugin_NoSibling_NoExtraWarning asserts that the canonical plugin without
// any sibling files produces no extra warnings.
//
// Conclusion: warnSiblingPluginFiles is silent when only trackfw-guard.js is present.
func TestOpenCodePlugin_NoSibling_NoExtraWarning(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	if len(msgs) != 0 {
		t.Errorf("expected no warnings for canonical plugin with no siblings; got: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// F4 fix b: opencode.json(c) plugin/plugins key detection
// --------------------------------------------------------------------------

// TestOpenCodePlugin_ConfigPluginsKey_Warns asserts that opencode.json with a "plugins"
// top-level key triggers a warning.
//
// Conclusion: warnOpenCodeConfigKeys detects "plugins" key in opencode.json at project root.
func TestOpenCodePlugin_ConfigPluginsKey_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	// opencode.json with a "plugins" key at project root.
	cfg := `{"plugins": [{"src": "other.js"}]}`
	if err := os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write opencode.json: %v", err)
	}

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "plugins") && strings.Contains(m, "opencode.json") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for 'plugins' key in opencode.json; got: %v", msgs)
	}
}

// TestOpenCodePlugin_ConfigPluginKeyJSONC_Warns asserts that opencode.jsonc with
// a "plugin" key (and a // comment) triggers a warning — JSONC-tolerant parsing.
//
// Conclusion: warnOpenCodeConfigKeys strips // comments before parsing; "plugin" key detected.
func TestOpenCodePlugin_ConfigPluginKeyJSONC_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	// opencode.jsonc with a JSONC comment and "plugin" key in .opencode/ subdir.
	cfg := "// opencode config\n{\"plugin\": {\"src\": \"other.js\"}}\n"
	opencodeSubDir := filepath.Join(projectDir, ".opencode")
	if err := os.WriteFile(filepath.Join(opencodeSubDir, "opencode.jsonc"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write opencode.jsonc: %v", err)
	}

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "\"plugin\"") && strings.Contains(m, "opencode.jsonc") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for 'plugin' key in .opencode/opencode.jsonc; got: %v", msgs)
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

// --------------------------------------------------------------------------
// N3: global plugins/ and opencode.json(c) sibling/key detection (ML-5I3)
// --------------------------------------------------------------------------

// TestOpenCodePlugin_GlobalSiblingJS_Warns asserts that an unexpected .js file in
// the global OpenCode plugins directory triggers a warning (N3: warnSiblingPluginFiles
// must inspect the global dir via the resolver).
//
// Conclusion: warnSiblingPluginFiles detects unexpected .js siblings in the
// global opencode plugins/ dir and returns a warning (N3 fix).
func TestOpenCodePlugin_GlobalSiblingJS_Warns(t *testing.T) {
	projectDir := t.TempDir()
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	globalConfigDir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", globalConfigDir)
	t.Setenv("XDG_CONFIG_HOME", "")

	// Write an unexpected .js sibling in the global plugins dir.
	globalPluginDir := filepath.Join(globalConfigDir, "plugins")
	if err := os.MkdirAll(globalPluginDir, 0755); err != nil {
		t.Fatalf("MkdirAll global plugins dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalPluginDir, "evil.js"), []byte("var evil=1"), 0644); err != nil {
		t.Fatalf("write evil.js: %v", err)
	}

	r := isolatedResolver(t)
	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "evil.js") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected global sibling warning for evil.js; got: %v", msgs)
	}
}

// TestOpenCodePlugin_GlobalConfigPluginsKey_Warns asserts that a "plugins" key in the
// global opencode.json triggers a warning (N3: warnOpenCodeConfigKeys must inspect
// the global config file).
//
// Conclusion: warnOpenCodeConfigKeys detects "plugins" key in global opencode.json
// and returns a warning (N3 fix).
func TestOpenCodePlugin_GlobalConfigPluginsKey_Warns(t *testing.T) {
	projectDir := t.TempDir()
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	globalConfigDir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", globalConfigDir)
	t.Setenv("XDG_CONFIG_HOME", "")

	// Write global opencode.json with a "plugins" key.
	cfg := `{"plugins": ["/tmp/evil.js"]}`
	if err := os.WriteFile(filepath.Join(globalConfigDir, "opencode.json"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write global opencode.json: %v", err)
	}

	r := isolatedResolver(t)
	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "plugins") && strings.Contains(m, "opencode.json") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected global opencode.json plugins-key warning; got: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// N4: JSONC block comment and trailing comma support (ML-5I3)
// --------------------------------------------------------------------------

// TestOpenCodePlugin_JSONCBlockComment_Warns asserts that opencode.jsonc with a /* */
// block comment is parsed correctly and the "plugins" key is detected.
//
// Conclusion: stripJSONCLineComments strips /* */ block comments so json.Unmarshal
// succeeds and the "plugins" key is detected (N4 fix).
func TestOpenCodePlugin_JSONCBlockComment_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	// opencode.jsonc with a block comment containing the plugins key.
	cfg := "/* global config */\n{\"plugins\": [\"/tmp/evil.js\"]}\n"
	if err := os.WriteFile(filepath.Join(projectDir, "opencode.jsonc"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write opencode.jsonc: %v", err)
	}

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "plugins") && strings.Contains(m, "opencode.jsonc") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected block-comment JSONC warning for plugins key; got: %v", msgs)
	}
}

// TestOpenCodePlugin_JSONCTrailingComma_Warns asserts that opencode.jsonc with a
// trailing comma before } is parsed correctly and the "plugins" key is detected.
//
// Conclusion: stripJSONCLineComments (enhanced) removes trailing commas so
// json.Unmarshal succeeds and the "plugins" key is detected (N4 fix).
func TestOpenCodePlugin_JSONCTrailingComma_Warns(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	projectDir := t.TempDir()
	r := isolatedResolver(t)
	writeOpenCodeDir(t, projectDir)
	writeCanonicalPlugin(t, projectDir)

	// opencode.jsonc with trailing comma before closing brace.
	cfg := "{\"plugins\": [\"/tmp/evil.js\"], }\n"
	if err := os.WriteFile(filepath.Join(projectDir, "opencode.jsonc"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write opencode.jsonc with trailing comma: %v", err)
	}

	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "plugins") && strings.Contains(m, "opencode.jsonc") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected trailing-comma JSONC warning for plugins key; got: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// N3-cond: global checks run even when .opencode/ is absent (ML-5I7)
// --------------------------------------------------------------------------

// TestOpenCodePlugin_GlobalSiblingJS_NoOpenCodeDir_Warns asserts that an unexpected
// .js file in the GLOBAL OpenCode plugins directory triggers a warning even when
// the project has no .opencode/ directory (N3-cond fix, ML-5I7).
//
// Before the N3-cond fix, validateOpenCodePluginWarningsWith returned nil as soon
// as .opencode/ was absent, discarding all global checks. This meant a global evil.js
// was invisible to validate when the project had not yet installed the project plugin.
//
// Conclusion: global sibling check runs unconditionally; .opencode/ absent does not
// silence a global evil.js warning (N3-cond fix).
func TestOpenCodePlugin_GlobalSiblingJS_NoOpenCodeDir_Warns(t *testing.T) {
	// Project has NO .opencode/ directory.
	projectDir := t.TempDir()

	globalConfigDir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", globalConfigDir)
	t.Setenv("XDG_CONFIG_HOME", "")

	// Write an unexpected .js sibling in the global plugins dir.
	globalPluginDir := filepath.Join(globalConfigDir, "plugins")
	if err := os.MkdirAll(globalPluginDir, 0755); err != nil {
		t.Fatalf("MkdirAll global plugins dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalPluginDir, "evil_global.js"), []byte("var evil=1"), 0644); err != nil {
		t.Fatalf("write evil_global.js: %v", err)
	}

	r := isolatedResolver(t)
	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "evil_global.js") {
			found = true
			break
		}
	}
	if !found {
		// Before N3-cond fix: early return for missing .opencode/ discards global checks → len=0.
		t.Errorf("N3-cond: expected global sibling warning for evil_global.js even without .opencode/; got: %v", msgs)
	}
}

// TestOpenCodePlugin_GlobalConfigKey_NoOpenCodeDir_Warns asserts that a "plugins" key
// in the GLOBAL opencode.json triggers a warning even when the project has no .opencode/
// directory (N3-cond fix, ML-5I7).
//
// Conclusion: global config-key check runs unconditionally; .opencode/ absent does not
// silence a global opencode.json "plugins" key warning (N3-cond fix).
func TestOpenCodePlugin_GlobalConfigKey_NoOpenCodeDir_Warns(t *testing.T) {
	// Project has NO .opencode/ directory.
	projectDir := t.TempDir()

	globalConfigDir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", globalConfigDir)
	t.Setenv("XDG_CONFIG_HOME", "")

	// Write global opencode.json with a "plugins" key.
	cfg := `{"plugins": ["/tmp/evil.js"]}`
	if err := os.WriteFile(filepath.Join(globalConfigDir, "opencode.json"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write global opencode.json: %v", err)
	}

	r := isolatedResolver(t)
	msgs := validator.ValidateOpenCodePluginWarningsWithForTest(projectDir, r)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "plugins") && strings.Contains(m, "opencode.json") {
			found = true
			break
		}
	}
	if !found {
		// Before N3-cond fix: early return for missing .opencode/ discards global checks → len=0.
		t.Errorf("N3-cond: expected global opencode.json plugins-key warning even without .opencode/; got: %v", msgs)
	}
}
