package commands

// guard_write_scope_test.go — subprocess tests for `trackfw guard write-scope`.
//
// ML-3A (ADR-2026-10-23 D3): Contract assertions via real binary.
//
// Contra-braço protocol (from advisor):
//   - On HEAD (before this ML): `trackfw guard write-scope` exits 2 (unknown subcommand).
//     Deny arms (want exit 2) therefore FAIL on HEAD by wrong reason (unknown subcommand
//     vs actual deny), but still exit 2.
//   - Allow arms (want exit 0) FAIL on HEAD by exit code.
//   - All tests pass only after write-scope is wired up.
//
// Each test declares what conclusion of ML-3A it asserts (Reconciliation Rule, CLAUDE.md).

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeScopeExitCode runs `trackfw guard write-scope` with the given JSON on stdin
// and returns (exitCode, stderr).
func writeScopeExitCode(t *testing.T, jsonPayload string) (int, string) {
	t.Helper()
	bin := guardBinary(t) // reuses guardBinary from guard_subprocess_test.go
	cmd := exec.Command(bin, "guard", "write-scope")
	cmd.Stdin = strings.NewReader(jsonPayload)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	stderr := stderrBuf.String()
	if err == nil {
		return 0, stderr
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), stderr
	}
	t.Fatalf("unexpected error running trackfw guard write-scope: %v", err)
	return -1, stderr
}

// TestGuardWriteScope_Registered asserts write-scope is in the guard subtree.
// Asserts: contract item 1 — subcommand is wired.
func TestGuardWriteScope_Registered(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Name() != "guard" {
			continue
		}
		for _, gc := range c.Commands() {
			if gc.Name() == "write-scope" {
				return // found
			}
		}
		t.Fatal("guard command does not have 'write-scope' subcommand")
	}
	t.Fatal("root command does not have 'guard' subcommand")
}

// TestGuardWriteScope_PositionalArg_IsGuardError asserts positional args → guardError (D8).
// Asserts: positional arg to write-scope exits 2 via isCommandUnderGuard (mirrors credential/git-branch).
func TestGuardWriteScope_PositionalArg_IsGuardError(t *testing.T) {
	root := newRootCmd()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"guard", "write-scope", "extra-arg"})
	cmd, err := root.ExecuteC()
	if err == nil {
		t.Fatal("expected error for positional arg, got nil")
	}
	if !isCommandUnderGuard(cmd) {
		t.Fatalf("command %q should be recognized as under guard", cmd.CommandPath())
	}
	if _, ok := err.(*guardError); !ok {
		t.Fatalf("expected *guardError, got %T: %v", err, err)
	}
}

// TestGuardWriteScope_EmptyStdin_Deny asserts that empty stdin → exit 2.
// Asserts: contract item 1 — all errors exit 2 (fail-closed D3-F).
func TestGuardWriteScope_EmptyStdin_Deny(t *testing.T) {
	rc, stderr := writeScopeExitCode(t, "")
	if rc != 2 {
		t.Fatalf("empty stdin: want exit 2, got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("empty stdin: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_InvalidJSON_Deny asserts that non-JSON stdin → exit 2.
// Asserts: contract item 1 — all errors exit 2.
func TestGuardWriteScope_InvalidJSON_Deny(t *testing.T) {
	rc, stderr := writeScopeExitCode(t, "not-json")
	if rc != 2 {
		t.Fatalf("invalid JSON: want exit 2, got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("invalid JSON: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_NonWriteTool_Allow asserts that a non-write tool_name → exit 0.
// Asserts: contract item 2 — tool_name outside Write|Edit|NotebookEdit → allow.
func TestGuardWriteScope_NonWriteTool_Allow(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`
	rc, stderr := writeScopeExitCode(t, payload)
	if rc != 0 {
		t.Fatalf("Bash tool: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_ReadTool_Allow asserts Read tool → exit 0.
// Asserts: contract item 2 — tool_name outside Write|Edit|NotebookEdit → allow.
func TestGuardWriteScope_ReadTool_Allow(t *testing.T) {
	payload := `{"tool_name":"Read","tool_input":{"file_path":"/etc/passwd"}}`
	rc, stderr := writeScopeExitCode(t, payload)
	if rc != 0 {
		t.Fatalf("Read tool: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_AbsentToolInput_Deny asserts that missing tool_input → exit 2.
// Asserts: contract item 2 — field absent → deny.
func TestGuardWriteScope_AbsentToolInput_Deny(t *testing.T) {
	payload := `{"tool_name":"Write"}`
	rc, stderr := writeScopeExitCode(t, payload)
	if rc != 2 {
		t.Fatalf("Write with no tool_input: want exit 2, got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("absent tool_input: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_EmptyFilePath_Deny asserts that empty file_path → exit 2.
// Asserts: contract item 2 — field empty → deny.
func TestGuardWriteScope_EmptyFilePath_Deny(t *testing.T) {
	payload := `{"tool_name":"Write","tool_input":{"file_path":""}}`
	rc, stderr := writeScopeExitCode(t, payload)
	if rc != 2 {
		t.Fatalf("empty file_path: want exit 2, got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("empty file_path: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_NotebookEdit_AbsentNotebookPath_Deny asserts NotebookEdit with no notebook_path → exit 2.
// Asserts: contract item 2 — NotebookEdit uses notebook_path; absent → deny.
func TestGuardWriteScope_NotebookEdit_AbsentNotebookPath_Deny(t *testing.T) {
	payload := `{"tool_name":"NotebookEdit","tool_input":{"file_path":"/tmp/test.ipynb"}}`
	rc, stderr := writeScopeExitCode(t, payload)
	if rc != 2 {
		t.Fatalf("NotebookEdit with file_path only (not notebook_path): want exit 2, got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("absent notebook_path: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_OutsideTrackfwProject_Allow asserts targets outside any trackfw project → allow.
// Asserts: contract item 3 — target outside trackfw project → allow.
func TestGuardWriteScope_OutsideTrackfwProject_Allow(t *testing.T) {
	// Create a file in a temp dir that has no trackfw.yaml ancestor.
	// t.TempDir() on CI lives under %TEMP% (Windows) or /tmp (Unix), never inside a checkout.
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	p := d4Payload("Write", outsideFile, "", "")
	rc, stderr := writeScopeExitCode(t, p)
	if rc != 2 && rc != 0 {
		t.Fatalf("outside-project path: want exit 0 or 2, got %d (stderr=%q)", rc, stderr)
	}
	// outsideFile has no trackfw.yaml ancestor — must be allowed.
	if rc != 0 {
		t.Fatalf("target outside trackfw project: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_WorkingContextNoMarker_Allow_MainSession asserts that
// docs/agents-working-context.md without frozen marker → allow in main session (no agent_id).
// Asserts: contract item 4 — marker absent → file is free for main session.
func TestGuardWriteScope_WorkingContextNoMarker_Allow_MainSession(t *testing.T) {
	// We need a real trackfw project directory with docs/agents-working-context.md without marker.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	wcDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(wcDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write without frozen marker.
	if err := os.WriteFile(filepath.Join(wcDir, "agents-working-context.md"), []byte("# Context\n"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "docs", "agents-working-context.md")
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", target, "", ""))
	if rc != 0 {
		t.Fatalf("working-context without frozen marker, main session: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_WorkingContextWithMarker_Deny_MainSession asserts that
// docs/agents-working-context.md WITH frozen marker in first 10 lines → deny even in main session.
// Asserts: contract item 4 — marker present → frozen for all sessions.
func TestGuardWriteScope_WorkingContextWithMarker_Deny_MainSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	wcDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(wcDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write WITH frozen marker in line 1.
	content := "<!-- trackfw:frozen -->\n# Context\n"
	if err := os.WriteFile(filepath.Join(wcDir, "agents-working-context.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "docs", "agents-working-context.md")
	// No agent_id → main session.
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", target, "", ""))
	if rc != 2 {
		t.Fatalf("working-context WITH frozen marker, main session: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen path deny: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_WorkingContextNoMarker_Deny_Subagent asserts that
// docs/agents-working-context.md WITHOUT frozen marker → deny for subagent (agent_id present).
// Asserts: contract item 5 — subagent freeze is unconditional for working-context.
func TestGuardWriteScope_WorkingContextNoMarker_Deny_Subagent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	wcDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(wcDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write WITHOUT frozen marker — but still frozen for subagent.
	if err := os.WriteFile(filepath.Join(wcDir, "agents-working-context.md"), []byte("# Context\n"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "docs", "agents-working-context.md")
	// agent_id present → subagent.
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", target, "apolo-tf", ""))
	if rc != 2 {
		t.Fatalf("working-context no marker, subagent: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen path deny: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_TrackfwYaml_Deny_Subagent asserts that trackfw.yaml is frozen for subagent.
// Asserts: contract item 5 — trackfw.yaml frozen for subagents.
func TestGuardWriteScope_TrackfwYaml_Deny_Subagent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "trackfw.yaml")
	rc, stderr := writeScopeExitCode(t, d4Payload("Edit", target, "apolo-tf", ""))
	if rc != 2 {
		t.Fatalf("trackfw.yaml subagent: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen path deny: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_TrackfwYaml_Allow_MainSession asserts that trackfw.yaml is NOT frozen for main session.
// Asserts: contract item 5 — only subagent freeze includes trackfw.yaml.
func TestGuardWriteScope_TrackfwYaml_Allow_MainSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "trackfw.yaml")
	// No agent_id → main session.
	rc, stderr := writeScopeExitCode(t, d4Payload("Edit", target, "", ""))
	if rc != 0 {
		t.Fatalf("trackfw.yaml main session: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_ClaudeSettings_Deny_Subagent asserts .claude/settings.json is frozen for subagent.
// Asserts: contract item 5 — .claude/settings.json frozen for subagents (prevents hook removal).
func TestGuardWriteScope_ClaudeSettings_Deny_Subagent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, ".claude", "settings.json")
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", target, "apolo-tf", ""))
	if rc != 2 {
		t.Fatalf(".claude/settings.json subagent: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen path deny: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_WorkingContextDir_Deny_Subagent asserts that a file under
// docs/agents-working-context/ is frozen for subagents (Decision 4, ML-4C).
// The directory docs/agents-working-context/ was previously not frozen, allowing subagents
// to write working-context entries. After Decision 4, the directory is added to the
// subagent frozen list so that D1 no longer depends on agent obedience.
//
// Asserts: Decision 4 (ML-4C) — docs/agents-working-context/ is frozen for subagents.
// Contra-braço: on HEAD, this test FAILS (exit 0 — the directory was not frozen).
func TestGuardWriteScope_WorkingContextDir_Deny_Subagent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// Create docs/agents-working-context/ dir and a file inside it.
	wcDir := filepath.Join(dir, "docs", "agents-working-context")
	if err := os.MkdirAll(wcDir, 0700); err != nil {
		t.Fatal(err)
	}
	innerFile := filepath.Join(wcDir, "notes.md")
	if err := os.WriteFile(innerFile, []byte("# notes\n"), 0600); err != nil {
		t.Fatal(err)
	}

	rc, stderr := writeScopeExitCode(t, d4Payload("Write", innerFile, "apolo-tf", ""))
	// Decision 4: docs/agents-working-context/ is now frozen for subagents — must deny.
	if rc != 2 {
		t.Fatalf("docs/agents-working-context/notes.md subagent: want exit 2 (deny — frozen dir for subagents), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen dir deny: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_WorkingContextDir_Allow_MainSession asserts that a file under
// docs/agents-working-context/ is NOT frozen for the main session (Decision 4, ML-4C).
// Only subagents (agent_id present) are blocked; the architect (main session) continues
// writing working-context entries as before.
//
// Asserts: Decision 4 (ML-4C) — docs/agents-working-context/ is free for the main session.
// Contra-braço: PASSES on HEAD (directory was never frozen — behavior unchanged).
func TestGuardWriteScope_WorkingContextDir_Allow_MainSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	wcDir := filepath.Join(dir, "docs", "agents-working-context")
	if err := os.MkdirAll(wcDir, 0700); err != nil {
		t.Fatal(err)
	}
	innerFile := filepath.Join(wcDir, "2026-10-23-apolo-entry.md")
	if err := os.WriteFile(innerFile, []byte("# entry\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// No agent_id → main session; docs/agents-working-context/ must remain writable.
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", innerFile, "", ""))
	if rc != 0 {
		t.Fatalf("docs/agents-working-context/ main session: want exit 0 (allow — not frozen for main session), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_YAMLExtraFrozenPath_Deny asserts that write_scope.frozen_paths in trackfw.yaml
// adds extra frozen paths for all sessions.
// Asserts: contract item 6 — YAML extension adds frozen paths, never removes defaults.
func TestGuardWriteScope_YAMLExtraFrozenPath_Deny(t *testing.T) {
	dir := t.TempDir()
	yaml := "write_scope:\n  frozen_paths:\n    - docs/secret.md\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "secret.md"), []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "docs", "secret.md")
	// No agent_id → main session (showing YAML frozen_paths applies to all sessions).
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", target, "", ""))
	if rc != 2 {
		t.Fatalf("YAML extra frozen path, main session: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen path deny: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_MalformedYAML_Deny asserts that malformed trackfw.yaml → exit 2 (fail-closed).
// The target is a plain file with NO frozen marker; the only reason to deny is YAML invalidity.
// Asserts: contract item 6 — YAML invalid → deny regardless of frozen-marker state (D3-F fail-closed).
// Contra-braço: on HEAD (before this ML) readWriteScopeFromYAML silently swallowed YAML errors
// and returned empty slices, so the guard would allow this write (exit 0). After the fix, exit 2.
func TestGuardWriteScope_MalformedYAML_Deny(t *testing.T) {
	dir := t.TempDir()
	// Unambiguously malformed YAML: unclosed bracket.
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("key: [unclosed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write a plain file with NO frozen marker — not covered by item 4 or item 5.
	if err := os.WriteFile(filepath.Join(docsDir, "plain.md"), []byte("# Plain\n"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "docs", "plain.md")
	// Main session (no agent_id). Only denial reason: malformed YAML in trackfw.yaml.
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", target, "", ""))
	if rc != 2 {
		t.Fatalf("malformed YAML, non-frozen target: want exit 2 (deny by YAML error), got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "YAML") && !strings.Contains(stderr, "yaml") {
		t.Fatalf("malformed YAML deny: want 'YAML' or 'yaml' in stderr, got %q", stderr)
	}
}

// TestGuardWriteScope_NonStringAgentID_Deny_Subagent asserts that non-string agent_id
// is treated as subagent (restrictive direction).
// Asserts: contract item 5 adendo — non-string agent_id → treat as subagent.
func TestGuardWriteScope_NonStringAgentID_Deny_Subagent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "trackfw.yaml")
	// agent_id is a number, not a string — intentionally non-string to test subagent treatment.
	intAgentPayload := map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]string{"file_path": target},
		"agent_id":   42,
	}
	intAgentPayloadBytes, _ := json.Marshal(intAgentPayload)
	rc, stderr := writeScopeExitCode(t, string(intAgentPayloadBytes))
	if rc != 2 {
		t.Fatalf("non-string agent_id: want exit 2 (treat as subagent), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen path deny: want non-empty stderr reason, got empty")
	}
}

// TestGuardWriteScope_TrackfwLeasesDir_Deny_Subagent asserts .trackfw/leases/ directory is frozen for subagent.
// Asserts: contract item 5 — .trackfw/leases/ directory prefix frozen for subagents.
func TestGuardWriteScope_TrackfwLeasesDir_Deny_Subagent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	leasesDir := filepath.Join(dir, ".trackfw", "leases")
	if err := os.MkdirAll(leasesDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leasesDir, "my.lease"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(leasesDir, "my.lease")
	rc, stderr := writeScopeExitCode(t, d4Payload("Write", target, "apolo-tf", ""))
	if rc != 2 {
		t.Fatalf(".trackfw/leases/my.lease subagent: want exit 2 (deny), got %d (stderr=%q)", rc, stderr)
	}
	if stderr == "" {
		t.Fatal("frozen path deny: want non-empty stderr reason, got empty")
	}
}

// ─────────────────────────────────────────────────────────────
// D4 — ML-3B: permission by dispatch (subprocess tests)
// ─────────────────────────────────────────────────────────────

// d4MakeLeaseFile writes .trackfw/leases/<agentType>.json in projectRoot.
func d4MakeLeaseFile(t *testing.T, projectRoot, agentType string, lease map[string]interface{}) {
	t.Helper()
	leasesDir := filepath.Join(projectRoot, ".trackfw", "leases")
	if err := os.MkdirAll(leasesDir, 0700); err != nil {
		t.Fatalf("mkdir leases: %v", err)
	}
	data, err := json.Marshal(lease)
	if err != nil {
		t.Fatalf("marshal lease: %v", err)
	}
	if err := os.WriteFile(filepath.Join(leasesDir, agentType+".json"), data, 0600); err != nil {
		t.Fatalf("write lease: %v", err)
	}
}

// d4Payload builds a JSON payload using json.Marshal (safe for Windows backslash paths).
func d4Payload(tool, filePath, agentID, agentType string) string {
	m := map[string]interface{}{
		"tool_name":  tool,
		"tool_input": map[string]string{"file_path": filePath},
	}
	if agentID != "" {
		m["agent_id"] = agentID
	}
	if agentType != "" {
		m["agent_type"] = agentType
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// TestGuardWriteScope_D4_Allow asserts that a target within the lease allow list → exit 0.
// Asserts: ML-3B AC4 — alvo dentro do allow → 0 (subprocess, real binary).
// Contra-braço: FAILS on HEAD (D4 absent → allows for wrong reason; allow arms still exit 0).
//
//	This test is behaviorally correct on both HEAD and after the fix.
//	The failing contra-braço is in the deny arm (TestGuardWriteScope_D4_Deny).
func TestGuardWriteScope_D4_Allow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(srcDir, "main.go")
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	d4MakeLeaseFile(t, dir, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{srcDir + "/"},
		"expires":    time.Now().Add(time.Hour).Format(time.RFC3339),
	})

	p := d4Payload("Write", target, "apolo-tf-session-1", "apolo-tf")
	rc, stderr := writeScopeExitCode(t, p)
	if rc != 0 {
		t.Fatalf("D4 allow: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_D4_Deny asserts that a target outside the allow list → exit 2.
// Asserts: ML-3B AC4 — alvo fora do allow → 2 (subprocess, real binary).
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestGuardWriteScope_D4_Deny(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0700); err != nil {
		t.Fatal(err)
	}
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(docsDir, "plan.md")
	if err := os.WriteFile(target, []byte("plan"), 0600); err != nil {
		t.Fatal(err)
	}

	// Lease allows only src/; target is in docs/ → deny.
	d4MakeLeaseFile(t, dir, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{srcDir + "/"},
		"expires":    time.Now().Add(time.Hour).Format(time.RFC3339),
	})

	p := d4Payload("Write", target, "apolo-tf-session-1", "apolo-tf")
	rc, stderr := writeScopeExitCode(t, p)
	if rc != 2 {
		t.Fatalf("D4 deny (outside allow): want exit 2, got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "allow") && !strings.Contains(stderr, "not in") {
		t.Fatalf("deny message: want 'allow' or 'not in' in stderr, got %q", stderr)
	}
}

// TestGuardWriteScope_D4_Expired_Deny asserts that an expired lease → exit 2.
// Asserts: ML-3B AC4 — permissão expirada → 2 (subprocess).
// Contra-braço: FAILS on HEAD (D4 absent → exit 0).
func TestGuardWriteScope_D4_Expired_Deny(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}

	d4MakeLeaseFile(t, dir, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{dir + "/"},
		"expires":    time.Now().Add(-time.Hour).Format(time.RFC3339), // expired
	})

	p := d4Payload("Write", target, "apolo-tf-session-1", "apolo-tf")
	rc, stderr := writeScopeExitCode(t, p)
	if rc != 2 {
		t.Fatalf("expired lease: want exit 2, got %d (stderr=%q)", rc, stderr)
	}
	if !strings.Contains(stderr, "expir") {
		t.Fatalf("expired deny message: want 'expir' in stderr, got %q", stderr)
	}
}

// TestGuardWriteScope_D4_NoLease_Allow asserts that without a lease, only D3 applies.
// Asserts: ML-3B AC4 — sem permissão → só a D3 (non-frozen target → allow).
// Contra-braço: PASSES on HEAD (same D3-only behavior → exit 0).
func TestGuardWriteScope_D4_NoLease_Allow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	// No lease file; agent_type = "apolo-tf".

	p := d4Payload("Write", target, "apolo-tf-session-1", "apolo-tf")
	rc, stderr := writeScopeExitCode(t, p)
	if rc != 0 {
		t.Fatalf("no lease → D3 only: want exit 0 (allow), got %d (stderr=%q)", rc, stderr)
	}
}

// TestGuardWriteScope_D4_RealClaudePayload asserts D4 allow works with the full Claude Code payload.
// The real Claude Code PreToolUse payload includes session_id, transcript_path, cwd,
// hook_event_name, agent_id, and agent_type in addition to the write fields.
// Asserts: ML-3B AC4 — payload real do Claude Code com agent_id e agent_type presentes → 0.
// Contra-braço: PASSES on HEAD (D4 absent → D3-only allow for non-frozen target).
func TestGuardWriteScope_D4_RealClaudePayload(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(srcDir, "output.go")
	if err := os.WriteFile(target, []byte("package out"), 0600); err != nil {
		t.Fatal(err)
	}

	d4MakeLeaseFile(t, dir, "apolo-tf", map[string]interface{}{
		"version":    1,
		"agent_type": "apolo-tf",
		"allow":      []string{srcDir + "/"},
		"expires":    time.Now().Add(time.Hour).Format(time.RFC3339),
	})

	// Full Claude Code PreToolUse payload as documented in https://code.claude.com/docs/en/hooks.
	fullPayload := map[string]interface{}{
		"session_id":      "session-abc123",
		"transcript_path": "/tmp/claude/transcript.jsonl",
		"cwd":             dir,
		"hook_event_name": "PreToolUse",
		"agent_id":        "sub-agent-xyz",
		"agent_type":      "apolo-tf",
		"tool_name":       "Write",
		"tool_input": map[string]string{
			"file_path": target,
			"content":   "package out\n// generated",
		},
	}
	payloadBytes, _ := json.Marshal(fullPayload)

	rc, stderr := writeScopeExitCode(t, string(payloadBytes))
	if rc != 0 {
		t.Fatalf("real Claude Code payload (subagent, within allow): want exit 0, got %d (stderr=%q)", rc, stderr)
	}
}
