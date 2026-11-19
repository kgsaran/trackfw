package opencodeplugin_test

// opencodeplugin_jsruntime_test.go — ML-5E2 (REQ-2026-11-17)
//
// End-to-end tests that load the embedded trackfw-guard.js asset into a real
// Node.js (or Bun) runtime and exercise the hook callback.
//
// Prerequisites:
//   - A working `node` (or `bun`) binary reachable via PATH.
//     The test calls `node --version` to prove the runtime is functional, and
//     calls t.Skip explicitly when no runtime is found or the version probe fails.
//   - CI fact: the `go` job in .github/workflows/quality.yml (line 33) runs on
//     ubuntu-latest with no setup-node step; the Windows job (line 146) runs
//     setup-node@v7 (line 169). Both have node. Tests do NOT skip in CI.
//     Skip only on dev machines where node is absent or broken (dylib issues, etc.).
//
// Fake `trackfw` binary:
//   Uses the helper-process pattern: the test binary itself is copied to a temp
//   dir as `trackfw` / `trackfw.exe`. When invoked with TRACKFW_FAKE_GUARD=1,
//   TestMain exits early (before m.Run) with the fake behaviour:
//     - reads stdin
//     - writes stdin to TRACKFW_STDIN_CAPTURE (env) if set
//     - exits 2 when stdin contains TRACKFW_FAKE_SECRET, else exits 0
//   This is Windows-safe: node spawnSync finds `trackfw.exe` on Windows just as
//   it finds `trackfw` on Unix — no shell needed, no .bat wrapper.
//
// macOS path note: t.TempDir() on macOS returns a path under /var/folders/...,
// but Node's require() resolves __filename via realpath, which follows the
// /var → /private/var symlink. All paths passed to node are evaluated through
// filepath.EvalSymlinks so the Go-constructed paths and node's view agree.
//
// arm3 root cause note: on macOS, Homebrew places both `node` and `trackfw` in
// /opt/homebrew/bin. If that dir is in PATH for the node subprocess, spawnSync
// finds the real trackfw and exits 0 instead of ENOENT. nodeEnvNoTrackfw must
// NOT include the node binary's directory — Go calls node via its absolute path
// so PATH need not contain it.
//
// Reconciliation rule: each sub-test carries a one-sentence "Conclusion".

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/opencodeplugin"
)

// TestMain is the helper-process entry point for the fake `trackfw` binary.
// When TRACKFW_FAKE_GUARD=1 the test binary is running as the fake binary;
// it handles the request and exits before m.Run() so no tests are executed.
// Without the env var this is a normal test run.
func TestMain(m *testing.M) {
	if os.Getenv("TRACKFW_FAKE_GUARD") != "1" {
		os.Exit(m.Run())
	}
	// Acting as fake trackfw guard.
	input, _ := io.ReadAll(os.Stdin)
	if cap := os.Getenv("TRACKFW_STDIN_CAPTURE"); cap != "" {
		_ = os.WriteFile(cap, input, 0644)
	}
	secret := os.Getenv("TRACKFW_FAKE_SECRET")
	if secret != "" && bytes.Contains(input, []byte(secret)) {
		os.Exit(2)
	}
	os.Exit(0)
}

// jsSecretToken assembles a fake Google API key at runtime so it does not
// appear as a literal string (which would trigger the credential guard on
// this source file).
func jsSecretToken() string {
	return "AIza" + strings.Repeat("F", 35)
}

// findNodeRuntime returns the path to the first working JS runtime (node, bun).
// Returns "" if none is found or functional.
func findNodeRuntime() string {
	for _, name := range []string{"node", "bun"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		cmd := exec.Command(p, "--version")
		cmd.Env = os.Environ()
		if _, err := cmd.Output(); err != nil {
			continue
		}
		return p
	}
	return ""
}

// writeAsset writes the project-scoped plugin asset into pluginDir and returns
// the absolute, symlink-resolved path to the .js file.
func writeAsset(t *testing.T, pluginDir string) string {
	t.Helper()
	content := opencodeplugin.RenderProject()
	jsPath := filepath.Join(pluginDir, "trackfw-guard.js")
	if err := os.WriteFile(jsPath, content, 0644); err != nil {
		t.Fatalf("write trackfw-guard.js: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(jsPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", jsPath, err)
	}
	return resolved
}

// writeFakeTrackfw copies the test binary into a temp dir as "trackfw" (or
// "trackfw.exe" on Windows) and returns that directory. When invoked with
// TRACKFW_FAKE_GUARD=1 the binary acts as the fake guard (see TestMain).
func writeFakeTrackfw(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	// Resolve symlinks — necessary on some platforms where the test binary
	// is under a symlinked path.
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatalf("EvalSymlinks(exe): %v", err)
	}

	name := "trackfw"
	if runtime.GOOS == "windows" {
		name = "trackfw.exe"
	}
	dst := filepath.Join(dir, name)

	src, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("reading test binary %s: %v", exe, err)
	}
	if err := os.WriteFile(dst, src, 0755); err != nil {
		t.Fatalf("writing fake trackfw to %s: %v", dst, err)
	}
	return dir
}

// nodeEnv builds the env slice for running node: PATH with fakeTrackfwDir first,
// plus existing env, minus OPENCODE_CONFIG_DIR/XDG_CONFIG_HOME. Also sets the
// TRACKFW_FAKE_GUARD=1 env var so the copied test binary acts as the fake guard.
func nodeEnv(fakeTrackfwDir, fakeSecret string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		switch k {
		case "OPENCODE_CONFIG_DIR", "XDG_CONFIG_HOME",
			"TRACKFW_FAKE_GUARD", "TRACKFW_FAKE_SECRET", "TRACKFW_STDIN_CAPTURE":
			continue
		}
		env = append(env, kv)
	}
	// Prepend fakeTrackfwDir to PATH so our fake trackfw wins over any real one.
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + fakeTrackfwDir + string(os.PathListSeparator) + strings.TrimPrefix(kv, "PATH=")
			goto pathSet
		}
	}
	env = append(env, "PATH="+fakeTrackfwDir)
pathSet:
	env = append(env,
		"TRACKFW_FAKE_GUARD=1",
		"TRACKFW_FAKE_SECRET="+fakeSecret,
	)
	return env
}

// nodeEnvNoTrackfw builds an env where trackfw is NOT in PATH.
// nodeBinDir is intentionally NOT included: the Go test runner calls node via its
// absolute path, so node does not need to be on PATH. Excluding nodeBinDir avoids
// accidentally including the same directory that holds the real `trackfw` binary
// (e.g. /opt/homebrew/bin on macOS contains both node and trackfw).
// This is used by arm3 to prove that the plugin throws on ENOENT.
func nodeEnvNoTrackfw(fakeSecret string) []string {
	// Use only OS-provided system dirs that will not contain trackfw.
	minDirs := []string{"/usr/bin", "/bin"}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		minDirs = append(minDirs, "/usr/sbin", "/sbin")
	}
	minPath := strings.Join(minDirs, string(os.PathListSeparator))

	var env []string
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		switch k {
		case "PATH", "OPENCODE_CONFIG_DIR", "XDG_CONFIG_HOME",
			"TRACKFW_FAKE_GUARD", "TRACKFW_FAKE_SECRET", "TRACKFW_STDIN_CAPTURE":
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"PATH="+minPath,
		"TRACKFW_FAKE_SECRET="+fakeSecret,
		// Intentionally NO TRACKFW_FAKE_GUARD=1 — trackfw is not in PATH at all.
	)
	return env
}

// runNodeDriver writes a node.js driver script and runs it with the given env.
// Returns (stdout, stderr, exit code).
func runNodeDriver(t *testing.T, nodeRuntime, driverContent string, env []string) (string, string, int) {
	t.Helper()
	driverDir := t.TempDir()
	driverPath := filepath.Join(driverDir, "driver.js")
	if err := os.WriteFile(driverPath, []byte(driverContent), 0644); err != nil {
		t.Fatalf("write driver.js: %v", err)
	}
	cmd := exec.Command(nodeRuntime, driverPath)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // ignore error; we check exit code via ProcessState
	rc := cmd.ProcessState.ExitCode()
	return stdout.String(), stderr.String(), rc
}

// makeV2Driver returns a V2-style node driver that:
//   - requires the plugin from pluginPath
//   - fires the hook callback with the given tool and inputJSON
//   - exits 1 if the hook throws, 0 otherwise
//   - writes "THREW: <msg>" to stderr when it throws
func makeV2Driver(pluginPath, projectDir, tool, inputJSON string) string {
	return fmt.Sprintf(`"use strict"
const plugin = require(%q)
const projectDir = %q
let threw = false
let throwMsg = ""
const ctx = {
  location: { project: { directory: projectDir } },
  tool: {
    hook: async function(event, cb) {
      try {
        await cb({ tool: %q, input: %s })
      } catch(e) {
        threw = true
        throwMsg = e.message
      }
    }
  }
}
plugin.setup(ctx).then(function() {
  if (threw) {
    process.stderr.write("THREW: " + throwMsg + "\n")
    process.exit(1)
  }
  process.exit(0)
}).catch(function(e) {
  process.stderr.write("SETUP_ERR: " + e.message + "\n")
  process.exit(1)
})
`, pluginPath, projectDir, tool, inputJSON)
}

// TestOpenCodePlugin_JSRuntime exercises the embedded trackfw-guard.js asset
// through a real Node.js runtime.
//
// Sub-tests:
//
//	arm1_shell_secret_throws     — secret in shell tool → plugin throws → exit 1; stderr contains "operation blocked"
//	arm2_innocuous_passes        — innocuous shell command → no throw → exit 0, empty stdout
//	arm3_enoent_throws           — trackfw not in PATH → ENOENT → plugin throws "not found" → exit 1
//	arm4_write_self_throws       — write to plugin file → D4 self-protection → exit 1; stderr contains "D4"
//	arm5_read_payload_structure  — read tool → payload has hook_source:"opencode" + abs file_path
//	arm6_plugin_dir_sibling_throws   — write to sibling in plugins/ dir → D4 dir protection → exit 1
//	arm7_opencode_json_uppercase_throws — write to OPENCODE.JSON (darwin) → D4 case-insensitive → exit 1
//	arm8_symlink_to_plugin_throws — write via symlink pointing to plugin → D4 realpath → exit 1
//	arm9_patch_header_plugin_throws   — patch with *** Update File targeting plugin → D4 → exit 1
//	arm10_patch_innocent_no_self_throw — patch targeting innocent file → no self-protection throw
//	arm11_webfetch_secret_throws      — webfetch URL containing secret → credential guard → exit 1
//	arm12_websearch_secret_throws     — websearch query containing secret → credential guard → exit 1
//	arm24_mention_zpfx_throws         — +++ z/.opencode/plugins/x.js (git apply -pN bypass) → mention layer fires D4 → exit 1
//	arm25_mention_mixedcase_throws    — +++ z/.OpenCode/Plugins/x.js (mixed case) → mention layer fires D4 → exit 1
//	arm26_opencode_notes_no_throw     — patch touching src/opencode-notes.md → "opencode" substring, not a protected pattern → no throw
//	arm27_octal_zprefix_throws        — +++ "z/\056opencode/plugins/x.js" (octal-encoded dot, z/ prefix) → D4 self-protection → exit 1
//	arm28_octal_in_body_throws        — octal \056opencode in patch body (innocent header) → D4 from mention scan octal decode → exit 1
//	arm29_global_dir_mention_throws   — patch body mentioning <OPENCODE_CONFIG_DIR>/plugins/x.js → global-dir pattern in mention scan → exit 1
//	arm30_mode120000_throws           — patch with "new file mode 120000" (symlink creation) → D4 self-protection → exit 1 (N7a-ML5I7)
//	arm31_dangling_symlink_write_throws — write to dangling symlink targeting protected dir → D4 via realpathOrParent fix → exit 1 (N7b-ML5I7)
//	arm32_nfd_unicode_throws          — write via NFD path to NFC protected dir → D4 after NFC normalisation → exit 1; darwin only (N8-ML5I7)
//	arm33_delete_opencode_patch_throws — patch deleting .opencode ancestor → D4 reverse-containment check → exit 1 (N9-ML5I7)
func TestOpenCodePlugin_JSRuntime(t *testing.T) {
	nodeRuntime := findNodeRuntime()
	if nodeRuntime == "" {
		t.Skip("no working JS runtime (node/bun) found — skipping JS runtime tests")
	}

	secret := jsSecretToken()
	fakeDir := writeFakeTrackfw(t)

	// Plugin asset path (symlink-resolved for macOS /var → /private/var).
	pluginDir := t.TempDir()
	pluginPath := writeAsset(t, pluginDir)

	// Project dir (symlink-resolved).
	projectDirRaw := t.TempDir()
	projectDir, err := filepath.EvalSymlinks(projectDirRaw)
	if err != nil {
		t.Fatalf("EvalSymlinks(projectDir): %v", err)
	}

	env := nodeEnv(fakeDir, secret)

	t.Run("arm1_shell_secret_throws", func(t *testing.T) {
		// Conclusion: shell tool with secret in command → fake trackfw exits 2 →
		// plugin throws with "operation blocked" message → driver exits 1.
		driver := makeV2Driver(pluginPath, projectDir, "shell",
			fmt.Sprintf(`{ command: %q }`, "echo "+secret))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, env)
		if rc == 0 {
			t.Fatalf("arm1: expected plugin to throw for secret in shell; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: throw is from guard block, not from ENOENT.
		if !strings.Contains(stderr, "THREW:") {
			t.Errorf("arm1: expected THREW: prefix in stderr; got %q", stderr)
		}
		if strings.Contains(stderr, "not found") {
			t.Errorf("arm1: threw ENOENT instead of guard block; stderr=%q", stderr)
		}
		// A5: guard block message must contain "operation blocked"
		if !strings.Contains(stderr, "operation blocked") {
			t.Errorf("arm1: expected 'operation blocked' in guard throw message; got %q", stderr)
		}
	})

	t.Run("arm2_innocuous_passes", func(t *testing.T) {
		// Conclusion: innocuous shell command → fake trackfw exits 0 → plugin does
		// not throw → driver exits 0 with empty stdout (D2 invariant).
		driver := makeV2Driver(pluginPath, projectDir, "shell",
			`{ command: "echo hello" }`)
		stdout, stderr, rc := runNodeDriver(t, nodeRuntime, driver, env)
		if rc != 0 {
			t.Fatalf("arm2: expected pass for innocuous command; got rc=%d stdout=%q stderr=%q", rc, stdout, stderr)
		}
		if stdout != "" {
			t.Errorf("arm2: D2 invariant — stdout must be empty on pass; got %q", stdout)
		}
	})

	t.Run("arm3_enoent_throws", func(t *testing.T) {
		// Conclusion: minimal PATH without trackfw → spawnSync returns ENOENT →
		// plugin throws "trackfw not found" → driver exits 1.
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		driver := makeV2Driver(pluginPath, projectDir, "shell",
			`{ command: "echo hello" }`)
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm3: expected throw when trackfw absent from PATH; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: throw is specifically ENOENT ("not found"), not a guard block.
		if !strings.Contains(stderr, "not found") && !strings.Contains(stderr, "ENOENT") {
			t.Errorf("arm3: expected ENOENT/not-found in stderr; got %q", stderr)
		}
	})

	t.Run("arm4_write_self_throws", func(t *testing.T) {
		// Conclusion: write tool targeting the plugin file → D4 selfProtect fires
		// before runGuard (which would need trackfw) → driver exits 1 with D4 message.
		// Uses nodeEnvNoTrackfw so that if D4 were to regress, the fall-through to
		// spawnSync would also throw (ENOENT) rather than pass silently.
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		driver := makeV2Driver(pluginPath, projectDir, "write",
			fmt.Sprintf(`{ path: %q }`, pluginPath))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm4: expected D4 self-protection throw for write to plugin; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: throw message names D4 self-protection, not ENOENT.
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm4: expected D4 self-protection in stderr; got %q (possible ENOENT if D4 regressed)", stderr)
		}
	})

	t.Run("arm5_read_payload_structure", func(t *testing.T) {
		// Conclusion: read tool → plugin sends JSON payload to trackfw with
		// hook_source:"opencode" and absolute tool_input.file_path.
		captureFile := filepath.Join(t.TempDir(), "captured.json")
		envCapture := append(nodeEnv(fakeDir, secret), "TRACKFW_STDIN_CAPTURE="+captureFile)

		// Relative path in input — the plugin should absolutize it.
		driver := makeV2Driver(pluginPath, projectDir, "read",
			`{ path: "some/relative/file.txt" }`)
		stdout, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envCapture)
		_ = stdout
		if rc != 0 {
			t.Fatalf("arm5: expected driver to pass; got rc=%d stdout=%q stderr=%q", rc, stdout, stderr)
		}

		// Wait briefly for the fake binary to finish writing.
		time.Sleep(100 * time.Millisecond)

		capturedBytes, err := os.ReadFile(captureFile)
		if err != nil {
			t.Fatalf("arm5: reading captured stdin: %v — fake trackfw may not have been called", err)
		}
		capturedStr := strings.TrimSpace(string(capturedBytes))
		if capturedStr == "" {
			t.Fatal("arm5: captured stdin is empty — fake trackfw was not called")
		}

		var payload map[string]json.RawMessage
		if err := json.Unmarshal([]byte(capturedStr), &payload); err != nil {
			t.Fatalf("arm5: captured stdin is not valid JSON: %v\nraw: %q", err, capturedStr)
		}

		// Assert hook_source == "opencode"
		var hookSource string
		if err := json.Unmarshal(payload["hook_source"], &hookSource); err != nil || hookSource != "opencode" {
			t.Errorf("arm5: hook_source: got raw %s, want %q", payload["hook_source"], "opencode")
		}

		// Assert tool_input.file_path is absolute (plugin absolutizes relative paths).
		var toolInput map[string]json.RawMessage
		if err := json.Unmarshal(payload["tool_input"], &toolInput); err != nil {
			t.Fatalf("arm5: parsing tool_input: %v", err)
		}
		var filePath string
		if err := json.Unmarshal(toolInput["file_path"], &filePath); err != nil {
			t.Errorf("arm5: tool_input.file_path missing or invalid: %v", err)
		} else if !filepath.IsAbs(filePath) {
			t.Errorf("arm5: tool_input.file_path must be absolute; got %q", filePath)
		}
	})

	t.Run("arm6_plugin_dir_sibling_throws", func(t *testing.T) {
		// Conclusion: writing any file inside .opencode/plugins/ (e.g. evil.js) →
		// D4 directory protection fires before runGuard → driver exits 1 (F4 fix).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// evil.js is a sibling of trackfw-guard.js in the same plugins/ dir.
		siblingPath := filepath.Join(filepath.Dir(pluginPath), "evil.js")
		driver := makeV2Driver(pluginPath, projectDir, "write",
			fmt.Sprintf(`{ path: %q }`, siblingPath))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm6: expected D4 dir-protection throw for sibling plugin; got rc=0 stderr=%q", stderr)
		}
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm6: expected D4 self-protection in stderr; got %q", stderr)
		}
	})

	t.Run("arm7_opencode_json_uppercase_throws", func(t *testing.T) {
		// Conclusion: on darwin (case-insensitive FS), writing OPENCODE.JSON is the same
		// inode as opencode.json — D4 config-file protection fires via realpath comparison
		// (F2 case fix).
		if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			t.Skip("case-insensitive FS test only applies on darwin and windows")
		}
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// Upper-case form of opencode.json in projectDir.
		upperPath := filepath.Join(projectDir, "OPENCODE.JSON")
		driver := makeV2Driver(pluginPath, projectDir, "write",
			fmt.Sprintf(`{ path: %q }`, upperPath))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm7: expected D4 throw for OPENCODE.JSON (case-insensitive); got rc=0 stderr=%q", stderr)
		}
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm7: expected D4 self-protection in stderr; got %q", stderr)
		}
	})

	t.Run("arm8_symlink_to_plugin_throws", func(t *testing.T) {
		// Conclusion: writing through a symlink that points to the plugin file → realpath
		// comparison catches the symlink target → D4 fires (F8 fix).
		symlinkPath := filepath.Join(t.TempDir(), "link-to-plugin.js")
		if !symlinkOrSkip(t, pluginPath, symlinkPath) {
			return
		}
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		driver := makeV2Driver(pluginPath, projectDir, "write",
			fmt.Sprintf(`{ path: %q }`, symlinkPath))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm8: expected D4 throw for symlink to plugin; got rc=0 stderr=%q", stderr)
		}
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm8: expected D4 self-protection in stderr; got %q", stderr)
		}
	})

	t.Run("arm9_patch_header_plugin_throws", func(t *testing.T) {
		// Conclusion: patch tool with patchText containing "*** Update File:
		// .opencode/plugins/trackfw-guard.js" → D4 patch-header parsing extracts
		// the target and fires self-protection (A7/F5 fix).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// Relative path in patch header — the guard resolves it against projectDir.
		patchText := "*** Begin Patch\n*** Update File: .opencode/plugins/trackfw-guard.js\n@@\n-old\n+new\n*** End Patch"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm9: expected D4 throw for patch targeting plugin; got rc=0 stderr=%q", stderr)
		}
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm9: expected D4 self-protection in stderr; got %q", stderr)
		}
	})

	t.Run("arm10_patch_innocent_no_self_throw", func(t *testing.T) {
		// Conclusion: patch tool targeting an ordinary file does not trigger
		// self-protection — only paths under protected dirs/files are denied.
		captureFile := filepath.Join(t.TempDir(), "captured10.json")
		envCapture := append(nodeEnv(fakeDir, secret), "TRACKFW_STDIN_CAPTURE="+captureFile)
		patchText := "*** Begin Patch\n*** Update File: src/main.go\n@@\n-old\n+new\n*** End Patch"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		stdout, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envCapture)
		_ = stdout
		if rc != 0 {
			t.Fatalf("arm10: expected no self-protection throw for innocent patch; got rc=%d stderr=%q", rc, stderr)
		}
	})

	t.Run("arm11_webfetch_secret_throws", func(t *testing.T) {
		// Conclusion: webfetch tool with URL containing a fake credential → plugin
		// routes it to trackfw guard credential → fake trackfw sees secret in stdin
		// and exits 2 → plugin throws (F6 coverage).
		driver := makeV2Driver(pluginPath, projectDir, "webfetch",
			fmt.Sprintf(`{ url: %q }`, "https://example.com/?key="+secret))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, env)
		if rc == 0 {
			t.Fatalf("arm11: expected throw for webfetch with credential in URL; got rc=0 stderr=%q", stderr)
		}
		if !strings.Contains(stderr, "THREW:") {
			t.Errorf("arm11: expected THREW: in stderr; got %q", stderr)
		}
		if strings.Contains(stderr, "not found") {
			t.Errorf("arm11: threw ENOENT instead of guard block; stderr=%q", stderr)
		}
	})

	t.Run("arm12_websearch_secret_throws", func(t *testing.T) {
		// Conclusion: websearch tool with query containing a fake credential → plugin
		// routes websearch via WEBFETCH_TOOLS to trackfw guard credential → fake
		// trackfw sees secret in stdin and exits 2 → plugin throws (ML-5I2 coverage).
		driver := makeV2Driver(pluginPath, projectDir, "websearch",
			fmt.Sprintf(`{ query: %q }`, "site:example.com key="+secret))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, env)
		if rc == 0 {
			t.Fatalf("arm12: expected throw for websearch with credential in query; got rc=0 stderr=%q", stderr)
		}
		if !strings.Contains(stderr, "THREW:") {
			t.Errorf("arm12: expected THREW: in stderr; got %q", stderr)
		}
		if strings.Contains(stderr, "not found") {
			t.Errorf("arm12: threw ENOENT instead of guard block; stderr=%q", stderr)
		}
	})

	t.Run("arm13_symlink_projectdir_throws", func(t *testing.T) {
		// Conclusion: when projectDir is a symlink, selfProtect resolves protectedFiles/Dirs
		// via realpathOrParent so a write targeting the REAL path is still blocked (N1 fix).
		realProjectDirRaw := t.TempDir()
		realProjectDir, err := filepath.EvalSymlinks(realProjectDirRaw)
		if err != nil {
			t.Fatalf("EvalSymlinks(realProjectDir): %v", err)
		}
		symlinkProjectDir := filepath.Join(t.TempDir(), "link-to-proj")
		if !symlinkOrSkip(t, realProjectDir, symlinkProjectDir) {
			return
		}
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// Target is the REAL opencode.json (not via symlink) — bypasses if protectedFiles not resolved.
		realTarget := filepath.Join(realProjectDir, "opencode.json")
		driver := makeV2Driver(pluginPath, symlinkProjectDir, "write",
			fmt.Sprintf(`{ path: %q }`, realTarget))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm13: expected D4 throw for write to real opencode.json via symlinked projectDir; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection message, not ENOENT.
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm13: expected D4 self-protection in stderr (not ENOENT); got %q", stderr)
		}
	})

	t.Run("arm14_symlink_globalconfigdir_throws", func(t *testing.T) {
		// Conclusion: when OPENCODE_CONFIG_DIR is a symlink (dotfile manager pattern),
		// selfProtect resolves protectedDirs via realpathOrParent so a write to the REAL
		// global plugins path is still blocked (N1 fix, Site 2).
		realConfigDirRaw := t.TempDir()
		realConfigDir, err := filepath.EvalSymlinks(realConfigDirRaw)
		if err != nil {
			t.Fatalf("EvalSymlinks(realConfigDir): %v", err)
		}
		symlinkConfigDir := filepath.Join(t.TempDir(), "link-to-cfg")
		if !symlinkOrSkip(t, realConfigDir, symlinkConfigDir) {
			return
		}
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// Add OPENCODE_CONFIG_DIR pointing to the SYMLINK dir.
		envNoTrackfw = append(envNoTrackfw, "OPENCODE_CONFIG_DIR="+symlinkConfigDir)
		// Target is the REAL plugins path (not via symlink) — bypasses if protectedDirs not resolved.
		realTarget := filepath.Join(realConfigDir, "plugins", "evil.js")
		driver := makeV2Driver(pluginPath, projectDir, "write",
			fmt.Sprintf(`{ path: %q }`, realTarget))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm14: expected D4 throw for write to real global plugins via symlinked OPENCODE_CONFIG_DIR; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection message, not ENOENT.
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm14: expected D4 self-protection in stderr (not ENOENT); got %q", stderr)
		}
	})

	t.Run("arm15_patch_ab_prefix_throws", func(t *testing.T) {
		// Conclusion: patch with unified diff header "+++ b/.opencode/plugins/evil.js" →
		// extractPatchTargets strips the b/ prefix, resolves to protected plugin dir →
		// D4 self-protection throws (N2 fix).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		patchText := "--- /dev/null\n+++ b/.opencode/plugins/evil.js\n@@\n+var evil=1\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm15: expected D4 throw for patch with b/ prefix targeting plugins; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection, not ENOENT.
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm15: expected D4 self-protection in stderr (not ENOENT); got %q", stderr)
		}
	})

	t.Run("arm16_patch_ab_prefix_control_ok", func(t *testing.T) {
		// Conclusion: patch with "+++ b/src/main.go" (innocuous path after prefix strip) →
		// selfProtect passes, guard called, no throw — N2 fix does not over-block safe patches.
		captureFile := filepath.Join(t.TempDir(), "captured16.json")
		envCapture := append(nodeEnv(fakeDir, secret), "TRACKFW_STDIN_CAPTURE="+captureFile)
		patchText := "--- a/src/main.go\n+++ b/src/main.go\n@@\n-old\n+new\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		stdout, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envCapture)
		_ = stdout
		if rc != 0 {
			t.Fatalf("arm16: expected no throw for patch with b/src/main.go (innocuous); got rc=%d stderr=%q", rc, stderr)
		}
	})

	t.Run("arm17_all_pathfields_checked", func(t *testing.T) {
		// Conclusion: payload with filePath="innocuous.txt" and path=".opencode/plugins/evil.js" →
		// selfProtect checks ALL path fields, finds the protected path, throws D4 (F7 fix).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		driver := makeV2Driver(pluginPath, projectDir, "write",
			fmt.Sprintf(`{ filePath: "innocuous.txt", path: %q }`,
				filepath.Join(projectDir, ".opencode", "plugins", "evil.js")))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm17: expected D4 throw for path=protected-plugin; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection (not ENOENT which would happen if filePath-only was checked).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm17: expected D4 self-protection in stderr (pre-fix gives ENOENT); got %q", stderr)
		}
	})

	t.Run("arm18_patch_no_header_throws", func(t *testing.T) {
		// Conclusion: patch/apply_patch with no recognizable patch headers in any string
		// field → fail-closed: D4 throws with "fail-closed" message (F5 fix).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// Plain text in patchText — no "***" or "+++"/"---" headers.
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			`{ patchText: "line1\nline2\n" }`)
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm18: expected fail-closed throw for patch with no headers; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: fail-closed message (not ENOENT which is the pre-fix behavior).
		if !strings.Contains(stderr, "fail-closed") {
			t.Errorf("arm18: expected fail-closed in stderr (pre-fix gives ENOENT); got %q", stderr)
		}
	})

	t.Run("arm19_mixed_patch_rename_to_plugin_throws", func(t *testing.T) {
		// Conclusion: mixed patch whose unified-diff section targets README.md (innocent)
		// but whose git rename section targets .opencode/plugins/evil.js → after ML-5I4
		// fix, diff --git + rename headers are recognized, D4 fires with "D4 self-protection"
		// (before fix: rename headers not recognized → README candidate passes selfProtect
		// → ENOENT from guard, stderr contains "not found" but not "D4 self-protection").
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		patchText := "--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-old\n+new\n" +
			"diff --git a/x b/.opencode/plugins/evil.js\n" +
			"rename from x\n" +
			"rename to .opencode/plugins/evil.js\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm19: expected D4 throw for mixed patch with rename to plugin; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection (before fix: "not found" from ENOENT after selfProtect passes).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm19: expected D4 self-protection in stderr (pre-fix gives ENOENT); got %q", stderr)
		}
	})

	t.Run("arm20_rename_only_to_plugin_throws_d4_not_failclosed", func(t *testing.T) {
		// Conclusion: rename-only patch (diff --git + rename from/to, no +++ header) targeting
		// .opencode/plugins/evil.js → after ML-5I4 fix, rename headers supply candidates and D4
		// fires with "D4 self-protection" (before fix: no recognizable headers → F5 fail-closed
		// throws "fail-closed", which does not contain "D4 self-protection").
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		patchText := "diff --git a/.opencode/plugins/trackfw-guard.js b/.opencode/plugins/evil.js\n" +
			"rename from .opencode/plugins/trackfw-guard.js\n" +
			"rename to .opencode/plugins/evil.js\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm20: expected D4 throw for rename-only patch to plugin; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection — not the generic fail-closed (pre-fix) message.
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm20: expected D4 self-protection in stderr (pre-fix gives fail-closed); got %q", stderr)
		}
		if strings.Contains(stderr, "fail-closed") {
			t.Errorf("arm20: got fail-closed message — means ML-5I4 fix did not add candidates from rename headers; stderr=%q", stderr)
		}
	})

	t.Run("arm21_rename_innocent_control_ok", func(t *testing.T) {
		// Conclusion: rename patch from src/a.go to src/b.go (both innocent) → selfProtect
		// passes, guard called, fake trackfw exits 0 → no throw (ML-5I4 fix does not over-block).
		captureFile := filepath.Join(t.TempDir(), "captured21.json")
		envCapture := append(nodeEnv(fakeDir, secret), "TRACKFW_STDIN_CAPTURE="+captureFile)
		patchText := "diff --git a/src/a.go b/src/b.go\n" +
			"rename from src/a.go\n" +
			"rename to src/b.go\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		stdout, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envCapture)
		_ = stdout
		if rc != 0 {
			t.Fatalf("arm21: expected no throw for rename of innocent files; got rc=%d stderr=%q", rc, stderr)
		}
	})

	t.Run("arm22_copy_to_plugin_throws", func(t *testing.T) {
		// Conclusion: copy-to patch whose copy-to target is .opencode/plugins/evil.js →
		// ML-5I4 fix recognizes copy from/to headers as candidates → D4 fires with
		// "D4 self-protection" (before fix: no recognizable headers → F5 fail-closed).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		patchText := "diff --git a/src/template.js b/.opencode/plugins/evil.js\n" +
			"copy from src/template.js\n" +
			"copy to .opencode/plugins/evil.js\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm22: expected D4 throw for copy-to plugin; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection (before fix gives fail-closed).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm22: expected D4 self-protection in stderr (pre-fix gives fail-closed); got %q", stderr)
		}
	})

	t.Run("arm23_quoted_octal_header_throws", func(t *testing.T) {
		// Conclusion: +++ header with C-string-quoted path containing \056 (octal for '.')
		// decodes to .opencode/plugins/evil.js → ML-5I4 unquoteGitPath decodes octal →
		// D4 fires with "D4 self-protection" (before fix: quoted token not decoded, resolves
		// to a path with literal backslash/digits → not in protected dir → ENOENT from guard).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// \056 is octal for '.'; the quoted path decodes to b/.opencode/plugins/evil.js.
		patchText := "--- /dev/null\n+++ \"b/\\056opencode/plugins/evil.js\"\n@@\n+var evil=1\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm23: expected D4 throw for quoted octal path targeting plugin; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection (before fix: "not found" from ENOENT because
		// the unquoted/undecoded token does not resolve to a protected path).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm23: expected D4 self-protection in stderr (pre-fix gives ENOENT or fail-closed); got %q", stderr)
		}
	})

	t.Run("arm24_mention_zpfx_throws", func(t *testing.T) {
		// Conclusion: patch with "+++ z/.opencode/plugins/x.js" header → extractPatchTargets
		// yields candidate "<projectDir>/z/.opencode/plugins/x.js" which is NOT under any
		// protected dir (second layer misses it) — before fix: credential guard called, fake
		// trackfw exits 0 → no throw (rc=0, bypass); after fix: mention layer finds
		// ".opencode/plugin" in lowercased raw text and throws "D4 self-protection" (N5-ML5I5).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		patchText := "--- /dev/null\n+++ z/.opencode/plugins/x.js\n@@\n+var evil=1\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm24: expected D4 throw for patch with z/-prefixed plugin path; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection from mention layer (not ENOENT from credential guard).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm24: expected D4 self-protection in stderr (pre-fix: rc=0, bypass via z/ prefix); got %q", stderr)
		}
	})

	t.Run("arm25_mention_mixedcase_throws", func(t *testing.T) {
		// Conclusion: patch with "+++ z/.OpenCode/Plugins/x.js" header (mixed case, z/ prefix)
		// → extractPatchTargets yields "<projectDir>/z/.OpenCode/Plugins/x.js"; the z/ prefix
		// defeats directory protection (candidate not under .opencode/plugins/); on Linux the
		// case mismatch also defeats isUnderDir — before fix: bypass (rc=0); after fix: mention
		// layer lowercases the haystack and finds ".opencode/plugin" → "D4 self-protection" (N5-ML5I5).
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		patchText := "--- /dev/null\n+++ z/.OpenCode/Plugins/x.js\n@@\n+var evil=1\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm25: expected D4 throw for patch with z/-prefixed mixed-case plugin path; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection from mention layer (not ENOENT from credential guard).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm25: expected D4 self-protection in stderr (pre-fix: rc=0 on all platforms via z/ bypass); got %q", stderr)
		}
	})

	t.Run("arm26_opencode_notes_no_throw", func(t *testing.T) {
		// Conclusion: patch whose headers and body mention "opencode" only as part of a
		// filename (src/opencode-notes.md) but not any protected pattern
		// (.opencode/plugin, opencode.json) → mention scan does not fire, selfProtect
		// passes, guard called, fake trackfw exits 0 → no throw (N5-ML5I5 control).
		captureFile := filepath.Join(t.TempDir(), "captured26.json")
		envCapture := append(nodeEnv(fakeDir, secret), "TRACKFW_STDIN_CAPTURE="+captureFile)
		patchText := "--- /dev/null\n+++ b/src/opencode-notes.md\n@@\n+# opencode notes\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		stdout, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envCapture)
		_ = stdout
		if rc != 0 {
			t.Fatalf("arm26: expected no throw for patch touching src/opencode-notes.md; got rc=%d stderr=%q", rc, stderr)
		}
	})

	t.Run("arm27_octal_zprefix_throws", func(t *testing.T) {
		// Conclusion: patch with '+++ "z/\056opencode/plugins/x.js"' header — octal \056
		// encodes '.' and the z/ prefix keeps the extracted candidate outside the protected
		// dir after resolve — denied by either the mention-scan octal-decode layer or the
		// decoded-candidate needle scan (N6-ML5I6).
		//
		// Before fix (ML-5I5 only): the mention scan replaces '\' → '/' giving '/056opencode'
		// (no '.opencode/plugin' match), and the extracted candidate resolves to
		// <projectDir>/z/.opencode/plugins/x.js which is NOT under the protected dir →
		// credential guard called → fake trackfw exits 0 → rc=0 (bypass).
		//
		// After fix (ML-5I6): caught by either the second mention haystack (octal decoded) or
		// the extracted-candidate needle scan → D4 self-protection → rc=1.
		patchText := "--- /dev/null\n+++ \"z/\\056opencode/plugins/x.js\"\n@@\n+var evil=1\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, nodeEnv(fakeDir, secret))
		if rc == 0 {
			t.Fatalf("arm27: FAIL before fix — expected D4 throw for octal z/-prefixed plugin path; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection (not ENOENT which is the pre-fix 5I5 behavior when
		// credential guard is called on the bypass candidate and trackfw is absent).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm27: expected D4 self-protection in stderr; got %q", stderr)
		}
	})

	t.Run("arm28_octal_in_body_throws", func(t *testing.T) {
		// Conclusion: patch with an innocent header (src/main.go) and a body line containing
		// '\056opencode/plugins/x.js' — the body line is not extracted by extractPatchTargets,
		// so only the mention-scan octal-decode layer catches it (N6-ML5I6).
		//
		// Before fix: mention scan replaces '\' → '/' → '/056opencode' → no match → guard
		// called on innocent candidate → fake trackfw exits 0 → rc=0 (bypass).
		//
		// After fix: mention scan also checks an octal-decoded haystack where \056 → '.' →
		// '.opencode/plugin' match → D4 self-protection → rc=1.
		patchText := "--- a/src/main.go\n+++ b/src/main.go\n@@\n+see \"\\056opencode/plugins/x.js\"\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, nodeEnv(fakeDir, secret))
		if rc == 0 {
			t.Fatalf("arm28: FAIL before fix — expected D4 throw for octal in patch body; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection from mention layer (not ENOENT from guard).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm28: expected D4 self-protection in stderr; got %q", stderr)
		}
	})

	t.Run("arm29_global_dir_mention_throws", func(t *testing.T) {
		// Conclusion: when OPENCODE_CONFIG_DIR is set to a temp dir and the patch body
		// mentions <configDir>/plugins/x.js (not in any recognized header), the mention
		// scan fires the global-dir-qualified needle and throws D4 self-protection
		// (global pattern armed in ML-5I6; pattern was present but untested since ML-5I5).
		//
		// This arm always PASSES on ML-5I5 code or later (the global pattern already exists
		// in N5); it adds formal coverage for the code path.
		globalDir := t.TempDir()
		// Use native separator in the path to exercise backslash handling on Windows.
		pluginMention := globalDir + string(filepath.Separator) + "plugins" + string(filepath.Separator) + "x.js"
		// Innocent header so extraction doesn't catch the global dir; only mention scan fires.
		patchText := "--- /dev/null\n+++ b/src/readme.md\n@@\n+install to " + pluginMention + "\n"
		envGlobal := append(nodeEnvNoTrackfw(secret), "OPENCODE_CONFIG_DIR="+globalDir)
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envGlobal)
		if rc == 0 {
			t.Fatalf("arm29: expected D4 throw for patch mentioning global config dir plugins; got rc=0 stderr=%q", stderr)
		}
		// Discriminator: D4 self-protection from mention scan (not ENOENT — trackfw absent but
		// selfProtect fires first; arm exercises the global-dir needle added in N5-ML5I5).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm29: expected D4 self-protection in stderr (global-dir pattern); got %q", stderr)
		}
	})

	t.Run("arm30_mode120000_throws", func(t *testing.T) {
		// Conclusion: patch/apply_patch containing "new file mode 120000" (symlink-creation
		// declaration) is denied by D4 self-protection before header extraction can supply
		// any candidate — after the N7a fix (ML-5I7). Before fix: mention scan misses the
		// target because "./" in the body breaks the needle match, candidates are non-plugin
		// paths from "diff --git a/p b/p" and "+++ b/p", selfProtect passes, guard is
		// called → ENOENT ("not found") instead of "D4 self-protection".
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// Patch declares a symlink via "new file mode 120000"; body has .opencode/./plugins/
		// (the "./" segment breaks the mention-scan needle .opencode/plugin → no match
		// without the mode-120000 fix).
		patchText := "diff --git a/p b/p\nnew file mode 120000\n--- /dev/null\n+++ b/p\n@@\n+.opencode/./plugins/evil3.js\n"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm30: expected D4 throw for patch with new file mode 120000; got rc=0 (bypass)")
		}
		// Discriminator: D4 self-protection from mode-120000 check (before fix: "not found"
		// from ENOENT because selfProtect passes when mode-120000 check absent).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm30: expected D4 self-protection in stderr (pre-fix gives ENOENT); got %q", stderr)
		}
	})

	t.Run("arm31_dangling_symlink_write_throws", func(t *testing.T) {
		// Conclusion: write through a dangling symlink whose target is under a protected
		// directory is blocked after the N7b fix to realpathOrParent (ML-5I7). Before fix:
		// realpathSync throws for the dangling target, the ancestor walk resolves to the
		// link file itself (not the target), isUnderDir returns false → selfProtect passes
		// → ENOENT from guard instead of "D4 self-protection".
		//
		// Setup: create a dangling symlink inside projectDir pointing to a path under
		// .opencode/plugins/ that does not exist yet.
		danglingLink := filepath.Join(projectDir, "n7_dangling_link")
		// Target is under .opencode/plugins/ — does not exist (dangling).
		symlinkTarget := ".opencode/plugins/evil_via_dangling.js"
		if !symlinkOrSkip(t, symlinkTarget, danglingLink) {
			return
		}
		t.Cleanup(func() { os.Remove(danglingLink) })
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		driver := makeV2Driver(pluginPath, projectDir, "write",
			fmt.Sprintf(`{ path: %q }`, danglingLink))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm31: expected D4 throw for write through dangling symlink to protected dir; got rc=0 (bypass)")
		}
		// Discriminator: D4 self-protection from N7b realpath fix (before fix: "not found"
		// from ENOENT because ancestor walk returns link file path, not target path).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm31: expected D4 self-protection in stderr (pre-fix gives ENOENT); got %q", stderr)
		}
	})

	t.Run("arm32_nfd_unicode_throws", func(t *testing.T) {
		// Conclusion: on macOS APFS a write target whose path uses NFD normalization of a
		// component that the protected-dir path has in NFC is blocked after the N8 fix that
		// applies .normalize('NFC') before case folding (ML-5I7). Before fix: toLowerCase()
		// alone does not normalise NFC vs NFD → isUnderDir returns false → ALLOW.
		// APFS is normalization-insensitive (NFC and NFD resolve to the same inode) but
		// normalization-preserving (realpathSync returns the input form without canonicalising).
		if runtime.GOOS != "darwin" {
			t.Skip("N8 NFD/NFC bypass only applies on macOS APFS (normalization-insensitive/preserving FS)")
		}

		// Create a parent dir under which both NFC and NFD forms resolve to the same inode.
		parentRaw := t.TempDir()
		parentDir, err := filepath.EvalSymlinks(parentRaw)
		if err != nil {
			t.Fatalf("EvalSymlinks(parentDir): %v", err)
		}

		// NFC form: é (U+00E9, 2 bytes: C3 A9).  NFD form: e + combining-acute (U+0301, 3 bytes: 65 CC 81).
		nfcSuffix := "éproj" // "éproj" in NFC
		nfdSuffix := "éproj" // NFD form of "é" followed by "proj"

		nfcProjDir := filepath.Join(parentDir, nfcSuffix)
		nfdProjDir := filepath.Join(parentDir, nfdSuffix)

		// Create .opencode/plugins/ under the NFC directory.
		nfcPluginsDir := filepath.Join(nfcProjDir, ".opencode", "plugins")
		if err := os.MkdirAll(nfcPluginsDir, 0755); err != nil {
			t.Fatalf("MkdirAll(nfcPluginsDir): %v", err)
		}
		nfcPluginPath := writeAsset(t, nfcPluginsDir)

		// Write target: NFD form of projectDir + .opencode/plugins/evil_nfd.js.
		// On macOS APFS this resolves to the same dir as the NFC form; on Linux it
		// is a distinct (non-existent) directory and this test is skipped above.
		nfdEvil := filepath.Join(nfdProjDir, ".opencode", "plugins", "evil_nfd.js")

		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// projectDir passed in NFC form; write target in NFD form of same path.
		driver := makeV2Driver(nfcPluginPath, nfcProjDir, "write",
			fmt.Sprintf(`{ path: %q }`, nfdEvil))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm32: N8 bypass — expected D4 throw for NFD write target when protected dir is NFC; got rc=0 (bypass)")
		}
		// Discriminator: D4 self-protection after NFC normalisation (before fix: "not found"
		// from ENOENT because isUnderDir(NFD, NFC) returns false without .normalize('NFC')).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm32: expected D4 self-protection in stderr (pre-fix gives ENOENT, NFD≠NFC in string cmp); got %q", stderr)
		}
	})

	t.Run("arm33_delete_opencode_patch_throws", func(t *testing.T) {
		// Conclusion: a patch that deletes or renames the .opencode ancestor directory is
		// blocked by the N9 reverse-containment check (ML-5I7): isUnderDir(protectedDir,
		// candidate) fires when the candidate (.opencode) contains the protected dir
		// (.opencode/plugins/). Before fix: only the forward check isUnderDir(candidate,
		// protectedDir) is performed; since .opencode is NOT under .opencode/plugins/,
		// selfProtect passes → ENOENT from guard instead of "D4 self-protection".
		envNoTrackfw := nodeEnvNoTrackfw(secret)
		// Patch declares deletion of the .opencode directory (ancestor of plugins/).
		patchText := "*** Begin Patch\n*** Delete File: .opencode\n*** End Patch"
		driver := makeV2Driver(pluginPath, projectDir, "patch",
			fmt.Sprintf(`{ patchText: %q }`, patchText))
		_, stderr, rc := runNodeDriver(t, nodeRuntime, driver, envNoTrackfw)
		if rc == 0 {
			t.Fatalf("arm33: expected D4 throw for patch deleting .opencode ancestor; got rc=0 (bypass)")
		}
		// Discriminator: D4 self-protection from reverse-containment check (before fix:
		// "not found" from ENOENT because forward isUnderDir(.opencode, .opencode/plugins/)=false).
		if !strings.Contains(stderr, "D4 self-protection") {
			t.Errorf("arm33: expected D4 self-protection in stderr (pre-fix gives ENOENT); got %q", stderr)
		}
	})
}
