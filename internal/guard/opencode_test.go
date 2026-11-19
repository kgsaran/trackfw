package guard

// Tests for OpenCode guard payload schema support (ADR-2026-11-19 D3, D4).
//
// All credential arms use mode:block in trackfw.yaml so that a detected
// credential returns exit 2 (not exit 0 warn). This ensures the "file was
// not read" negative arms do not pass vacuously because warn is the default.
//
// Keys are NEVER written as literals: they are assembled at runtime so that
// the guard hook and CI do not trigger on this source file.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openCodePayloadRead builds an OpenCode credential guard payload for the
// "read" tool, using tool_input.file_path (ADR-2026-11-19 D3; contract §8
// confirmed: probe added file_path explicitly since V2 sends "path").
// Uses json.Marshal so Windows backslash paths are correctly escaped.
func openCodePayloadRead(t *testing.T, filePath, cwd string) string {
	t.Helper()
	m := map[string]any{
		"hook_source": "opencode",
		"tool_name":   "read",
		"tool_input":  map[string]any{"file_path": filePath},
		"cwd":         cwd,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(b)
}

// openCodePayloadShell builds an OpenCode guard payload for the "shell" tool.
func openCodePayloadShell(t *testing.T, command, cwd string) string {
	t.Helper()
	m := map[string]any{
		"hook_source": "opencode",
		"tool_name":   "shell",
		"tool_input":  map[string]any{"command": command},
		"cwd":         cwd,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(b)
}

// openCodePayloadTool builds an OpenCode guard payload for any tool with
// file_path in tool_input (write, edit, etc.).
func openCodePayloadTool(t *testing.T, toolName, filePath, cwd string) string {
	t.Helper()
	m := map[string]any{
		"hook_source": "opencode",
		"tool_name":   toolName,
		"tool_input":  map[string]any{"file_path": filePath},
		"cwd":         cwd,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(b)
}

// makeOpenCodeProjectDir creates a temp project directory with trackfw.yaml
// containing credential_guard.mode: block.
func makeOpenCodeProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	yaml := "credential_guard:\n  mode: block\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	return dir
}

// googleFakeKeyOC returns a syntactically valid Google API key pattern that is
// assembled at runtime. Alias for use in this file without redeclaration.
// Pattern: AIza[0-9A-Za-z_-]{35}
func googleFakeKeyOC() string {
	return "AIza" + strings.Repeat("F", 35)
}

// --- Credential guard: OpenCode read tool ---

// TestOpenCodeRead_SecretInFile_Blocked asserts that a read tool call with
// hook_source="opencode" and file_path pointing to a file containing a
// credential triggers exit 2.
//
// Conclusion asserted: credOpenCodeReadScan reads tool_input.file_path and
// scans the file when hook_source=="opencode" and tool_name=="read" — closing
// the gap measured as C4 in contract §8.
func TestOpenCodeRead_SecretInFile_Blocked(t *testing.T) {
	projectDir := makeOpenCodeProjectDir(t)
	secret := googleFakeKeyOC()

	secretFile := filepath.Join(projectDir, "config.env")
	if err := os.WriteFile(secretFile, []byte("GOOGLE_KEY="+secret+"\n"), 0644); err != nil {
		t.Fatalf("write config.env: %v", err)
	}

	p := openCodePayloadRead(t, secretFile, projectDir)
	code, stdout, _ := runCred(t, projectDir, p)
	if code != 2 {
		t.Fatalf("want exit 2 (read file with credential blocked), got %d\n"+
			"NOTE: exit 0 here means credOpenCodeReadScan is not implemented yet", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 2, got %q", stdout)
	}
}

// TestOpenCodeRead_NoHookSource_FileNotRead asserts that a payload without
// hook_source does NOT trigger the OpenCode read scan — the file is not opened.
//
// Conclusion asserted: credOpenCodeReadScan requires hook_source=="opencode";
// a plain tool_name=read payload without hook_source is not recognised as
// OpenCode and the file content does not reach the guard.
func TestOpenCodeRead_NoHookSource_FileNotRead(t *testing.T) {
	projectDir := makeOpenCodeProjectDir(t)
	secret := googleFakeKeyOC()

	secretFile := filepath.Join(projectDir, "secret.env")
	if err := os.WriteFile(secretFile, []byte(secret), 0644); err != nil {
		t.Fatalf("write secret.env: %v", err)
	}

	// Payload with tool_name=read and file_path but NO hook_source.
	m := map[string]any{
		"tool_name":  "read",
		"tool_input": map[string]any{"file_path": secretFile},
		"cwd":        projectDir,
	}
	b, _ := json.Marshal(m)
	code, stdout, stderr := runCred(t, projectDir, string(b))
	if code != 0 {
		t.Fatalf("want exit 0 (no hook_source → file not read), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 0, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("want empty stderr on exit 0 (no detection), got %q", stderr)
	}
}

// TestOpenCodeRead_WriteToolWithSecretFile_FileNotRead asserts that a
// hook_source="opencode" payload with tool_name="write" does NOT trigger
// the OpenCode read scan — only tool_name="read" triggers it.
//
// Conclusion asserted: credOpenCodeReadScan gates on tool_name=="read";
// for write/shell/edit payloads the guard does not read tool_input.file_path.
func TestOpenCodeRead_WriteToolWithSecretFile_FileNotRead(t *testing.T) {
	projectDir := makeOpenCodeProjectDir(t)
	secret := googleFakeKeyOC()

	secretFile := filepath.Join(projectDir, "secret.env")
	if err := os.WriteFile(secretFile, []byte(secret), 0644); err != nil {
		t.Fatalf("write secret.env: %v", err)
	}

	// write tool — file_path points at the secret file but the secret is NOT
	// in the payload itself, only in the file. No credential in payload → exit 0.
	p := openCodePayloadTool(t, "write", secretFile, projectDir)
	code, stdout, stderr := runCred(t, projectDir, p)
	if code != 0 {
		t.Fatalf("want exit 0 (write tool: file not read by guard), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 0, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("want empty stderr on exit 0 (no detection), got %q", stderr)
	}
}

// TestOpenCodeRead_RelativePath_NotRead asserts that a relative file_path is
// NOT read — the guard requires an absolute path (ADR-2026-11-19 D3).
//
// Conclusion asserted: credOpenCodeReadScan skips non-absolute paths; a
// relative file_path does not trigger file reading even when hook_source
// is "opencode" and tool_name is "read".
func TestOpenCodeRead_RelativePath_NotRead(t *testing.T) {
	projectDir := makeOpenCodeProjectDir(t)
	secret := googleFakeKeyOC()

	// Write the file so that if the guard resolved the path against cwd, it would
	// find the credential.
	if err := os.WriteFile(filepath.Join(projectDir, "secret.env"), []byte(secret), 0644); err != nil {
		t.Fatalf("write secret.env: %v", err)
	}

	// Relative path in tool_input.file_path.
	m := map[string]any{
		"hook_source": "opencode",
		"tool_name":   "read",
		"tool_input":  map[string]any{"file_path": "secret.env"},
		"cwd":         projectDir,
	}
	b, _ := json.Marshal(m)
	code, stdout, stderr := runCred(t, projectDir, string(b))
	if code != 0 {
		t.Fatalf("want exit 0 (relative path → not read), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 0, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("want empty stderr on exit 0 (no detection), got %q", stderr)
	}
}

// TestOpenCodeRead_DirectoryPath_NotRead asserts that a file_path pointing to
// a directory is not read — the guard only scans regular files.
//
// Conclusion asserted: credOpenCodeReadScan calls credScanFilePath which uses
// os.Stat and !fi.Mode().IsRegular() to skip non-regular files; a directory
// path returns exit 0 without error.
func TestOpenCodeRead_DirectoryPath_NotRead(t *testing.T) {
	projectDir := makeOpenCodeProjectDir(t)

	// file_path points at the project directory itself (a directory, not a file).
	p := openCodePayloadRead(t, projectDir, projectDir)
	code, stdout, stderr := runCred(t, projectDir, p)
	if code != 0 {
		t.Fatalf("want exit 0 (directory path → not read), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 0, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("want empty stderr on exit 0 (no detection), got %q", stderr)
	}
}

// TestOpenCodeRead_InnocentFile_Allowed asserts that a read tool call pointing
// to a file without credentials is allowed — exit 0 with empty stdout.
//
// Conclusion asserted: credOpenCodeReadScan does not block when the file
// content contains no credential pattern; D2 invariant (stdout empty) holds.
func TestOpenCodeRead_InnocentFile_Allowed(t *testing.T) {
	projectDir := makeOpenCodeProjectDir(t)

	innocuousFile := filepath.Join(projectDir, "README.md")
	if err := os.WriteFile(innocuousFile, []byte("# Hello\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	p := openCodePayloadRead(t, innocuousFile, projectDir)
	code, stdout, _ := runCred(t, projectDir, p)
	if code != 0 {
		t.Fatalf("want exit 0 (innocuous file → allowed), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 0 (allow), got %q", stdout)
	}
}

// TestOpenCodeShell_SecretInCommand_Blocked asserts that a shell tool call with
// hook_source="opencode" and a credential in tool_input.command is blocked.
//
// Conclusion asserted: Layer 1 (full-payload byte scan) detects the credential
// in tool_input.command; existing code paths cover shell payloads without any
// OpenCode-specific addition.
func TestOpenCodeShell_SecretInCommand_Blocked(t *testing.T) {
	projectDir := makeOpenCodeProjectDir(t)
	secret := googleFakeKeyOC()

	p := openCodePayloadShell(t, "echo "+secret, projectDir)
	code, stdout, _ := runCred(t, projectDir, p)
	if code != 2 {
		t.Fatalf("want exit 2 (credential in shell command blocked), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 2, got %q", stdout)
	}
}

// TestOpenCodeRead_GlobalScope_SecretInFile_Blocked asserts that the global
// scope (--global) also scans the file when hook_source="opencode" and
// tool_name="read" — RunCredentialGlobal must call credOpenCodeReadScan too.
//
// Conclusion asserted: credOpenCodeReadScan is called in RunCredentialGlobal;
// a credential in the read file triggers exit 2 at global scope (default block).
func TestOpenCodeRead_GlobalScope_SecretInFile_Blocked(t *testing.T) {
	// Global scope: no trackfw.yaml required; default mode is block.
	dir := t.TempDir()
	secret := googleFakeKeyOC()

	secretFile := filepath.Join(dir, "config.env")
	if err := os.WriteFile(secretFile, []byte("GOOGLE_KEY="+secret+"\n"), 0644); err != nil {
		t.Fatalf("write config.env: %v", err)
	}

	p := openCodePayloadRead(t, secretFile, dir)
	code, stdout, _ := runCredGlobal(t, dir, p)
	if code != 2 {
		t.Fatalf("want exit 2 (global scope: read file with credential blocked), got %d\n"+
			"NOTE: exit 0 means RunCredentialGlobal does not call credOpenCodeReadScan", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 2, got %q", stdout)
	}
}

// --- Write-scope: OpenCode frozen paths for subagents ---

// TestWriteScope_OpenCode_PluginsDir_SubagentDenied asserts that a subagent
// (agent_id present) is denied when attempting to write a file inside
// .opencode/plugins/ (e.g. the guard plugin file).
//
// Conclusion asserted: ".opencode/plugins/" is in writeScopeSubagentFrozenRelPaths;
// a subagent writing .opencode/plugins/trackfw-guard.js is denied (rc=2).
func TestWriteScope_OpenCode_PluginsDir_SubagentDenied(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".opencode", "plugins", "trackfw-guard.js")

	rc, stderr := runWS(t, payload("Write", target, "subagent-id"))
	if rc != 2 {
		t.Fatalf("subagent writing .opencode/plugins/trackfw-guard.js: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestWriteScope_OpenCode_OpencodeJson_SubagentDenied asserts that a subagent
// is denied when attempting to write opencode.json.
//
// Conclusion asserted: "opencode.json" is in writeScopeSubagentFrozenRelPaths;
// a subagent writing opencode.json is denied (rc=2).
func TestWriteScope_OpenCode_OpencodeJson_SubagentDenied(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "opencode.json")

	rc, stderr := runWS(t, payload("Write", target, "subagent-id"))
	if rc != 2 {
		t.Fatalf("subagent writing opencode.json: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestWriteScope_OpenCode_OpencodeJsonc_SubagentDenied asserts that a subagent
// is denied when attempting to write opencode.jsonc.
//
// Conclusion asserted: "opencode.jsonc" is in writeScopeSubagentFrozenRelPaths;
// a subagent writing opencode.jsonc is denied (rc=2).
func TestWriteScope_OpenCode_OpencodeJsonc_SubagentDenied(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "opencode.jsonc")

	rc, stderr := runWS(t, payload("Write", target, "subagent-id"))
	if rc != 2 {
		t.Fatalf("subagent writing opencode.jsonc: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
}

// TestWriteScope_OpenCode_OutsidePluginsDir_Allowed is the counter-arm:
// a subagent writing a file inside .opencode/ but NOT under plugins/ should
// be allowed — only plugins/ and the config files are frozen.
//
// Conclusion asserted: the ".opencode/plugins/" frozen entry is a directory
// prefix, not a full .opencode/ freeze; writing .opencode/agent/notes.md is
// permitted (rc=0 — D3 only, no frozen match, no lease enforcement for this path).
func TestWriteScope_OpenCode_OutsidePluginsDir_Allowed(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, ".opencode", "agent", "notes.md")

	rc, _ := runWS(t, payload("Write", target, "subagent-id"))
	if rc != 0 {
		t.Fatalf("subagent writing .opencode/agent/notes.md: want exit 0 (allow), got %d", rc)
	}
}

// TestWriteScope_OpenCode_MainSession_Allowed is the counter-arm for main
// session: without agent_id, the subagent frozen paths do not apply, so the
// main session can write opencode.json.
//
// Conclusion asserted: writeScopeSubagentFrozenRelPaths is only applied when
// isSubagent is true; a main session (no agent_id) writing opencode.json is
// permitted (rc=0).
func TestWriteScope_OpenCode_MainSession_Allowed(t *testing.T) {
	proj := makeWSProjectDir(t)
	target := filepath.Join(proj, "opencode.json")

	// No agent_id → main session.
	rc, _ := runWS(t, payload("Write", target, ""))
	if rc != 0 {
		t.Fatalf("main session writing opencode.json: want exit 0 (allow), got %d", rc)
	}
}
