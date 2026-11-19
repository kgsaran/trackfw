package generators

// opencode_plugin_test.go — tests for InjectOpenCodePlugin (ML-5E, REQ-2026-11-17).
//
// Each test documents the conclusion it asserts (CLAUDE.md Regra Dura de
// Reconciliação): the conclusion line is the sentence another agent would
// use to decide whether to run this test.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/opencodeplugin"
)

// TestInjectOpenCodePlugin_Create asserts:
// InjectOpenCodePlugin creates .opencode/plugins/trackfw-guard.js with the
// correct Marker on the first line and const SCOPE = "project" in the body.
func TestInjectOpenCodePlugin_Create(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := t.TempDir()
	if err := InjectOpenCodePlugin(dir); err != nil {
		t.Fatalf("InjectOpenCodePlugin: %v", err)
	}

	path := filepath.Join(dir, ".opencode", "plugins", "trackfw-guard.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}

	// D1: first line must be the marker.
	firstLine := string(data)
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstLine = strings.TrimRight(firstLine, "\r")
	if firstLine != opencodeplugin.Marker {
		t.Errorf("first line = %q, want %q", firstLine, opencodeplugin.Marker)
	}

	// D1: SCOPE must be "project" in the project variant.
	if !strings.Contains(string(data), `const SCOPE = "project"`) {
		t.Error("plugin missing 'const SCOPE = \"project\"'")
	}
}

// TestInjectOpenCodePlugin_Idempotent asserts:
// Calling InjectOpenCodePlugin twice on the same directory produces byte-identical
// output — the second call is a no-op when the content already matches the canonical render.
func TestInjectOpenCodePlugin_Idempotent(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := t.TempDir()
	if err := InjectOpenCodePlugin(dir); err != nil {
		t.Fatalf("first run: %v", err)
	}
	path := filepath.Join(dir, ".opencode", "plugins", "trackfw-guard.js")
	run1, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read after first run: %v", readErr)
	}

	if err := InjectOpenCodePlugin(dir); err != nil {
		t.Fatalf("second run: %v", err)
	}
	run2, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read after second run: %v", readErr)
	}

	if string(run1) != string(run2) {
		t.Errorf("second run produced different output:\nrun1 len=%d run2 len=%d", len(run1), len(run2))
	}
}

// TestInjectOpenCodePlugin_Overwrites asserts:
// When .opencode/plugins/trackfw-guard.js exists but its content differs from the
// canonical render (e.g., stale or missing marker), InjectOpenCodePlugin overwrites it.
func TestInjectOpenCodePlugin_Overwrites(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".opencode", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Pre-seed with stale content (no marker).
	stale := []byte("// stale plugin without marker\nconsole.log('old')\n")
	if err := os.WriteFile(filepath.Join(pluginDir, "trackfw-guard.js"), stale, 0644); err != nil {
		t.Fatal(err)
	}

	if err := InjectOpenCodePlugin(dir); err != nil {
		t.Fatalf("InjectOpenCodePlugin: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(pluginDir, "trackfw-guard.js"))
	if err != nil {
		t.Fatalf("read after inject: %v", err)
	}

	firstLine := string(data)
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstLine = strings.TrimRight(firstLine, "\r")
	if firstLine != opencodeplugin.Marker {
		t.Errorf("after overwrite: first line = %q, want %q", firstLine, opencodeplugin.Marker)
	}
}

// TestInjectOpenCodePlugin_SymlinkDirRejected asserts:
// When .opencode/ is a symlink pointing outside the project root, InjectOpenCodePlugin
// returns an error and does not write the target file — pathguard rejects it.
func TestInjectOpenCodePlugin_SymlinkDirRejected(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := t.TempDir()
	target := t.TempDir() // outside project root

	symlinkPath := filepath.Join(dir, ".opencode")
	symlinkOrSkip(t, target, symlinkPath)

	err := InjectOpenCodePlugin(dir)
	if err == nil {
		t.Error("expected error for symlink .opencode/ directory, got nil")
	}

	// The target directory outside the root must not contain the plugin.
	if _, statErr := os.Stat(filepath.Join(target, "plugins", "trackfw-guard.js")); statErr == nil {
		t.Error("trackfw-guard.js was written to symlink target outside project root")
	}
}
