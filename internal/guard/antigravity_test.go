package guard

// Tests for Google Antigravity payload schema support (ADR-2026-11-18 D2–D5).
//
// All credential tests use mode:block in trackfw.yaml so that a detected
// credential returns exit 2 (not 0 warn). This ensures the lastUserInput→0
// contra-arm does not pass vacuously because warn is the default.
//
// Credential arms chdir into <project>/.agents (the actual cwd when an
// Antigravity hook fires), not into the project root. The fix in RunCredential
// uses the payload "cwd" field to find trackfw.yaml in the parent directory.
//
// Keys are NEVER written as literals: they are assembled at runtime so that
// the guard hook and CI do not trigger on this source file.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// googleFakeKey returns a syntactically valid Google API key pattern that is
// assembled at runtime. Pattern: AIza[0-9A-Za-z_-]{35}
// Generated at runtime so the source file does not trigger the credential guard.
func googleFakeKey() string {
	return "AIza" + strings.Repeat("F", 35)
}

// makeAntigravityProjectDir creates a temp project directory with trackfw.yaml
// containing credential_guard.mode: block, and an .agents subdirectory (the
// cwd that the Antigravity hook process uses).
//
// Returns (projectDir, agentsDir).
func makeAntigravityProjectDir(t *testing.T) (string, string) {
	t.Helper()
	projectDir := t.TempDir()
	yaml := "credential_guard:\n  mode: block\n"
	if err := os.WriteFile(filepath.Join(projectDir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	agentsDir := filepath.Join(projectDir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	return projectDir, agentsDir
}

// antigravityPayload builds a minimal Antigravity PreToolUse JSON payload.
// toolName is "run_command" or "view_file".
// args is a map of argument names to values.
// extra adds extra root-level keys (e.g. "lastUserInput") to the payload.
func antigravityPayload(t *testing.T, projectCwd, toolName string, args map[string]string, extra map[string]string) string {
	t.Helper()
	// Build args object.
	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	// Build extra root fields.
	var extraPairs []string
	for k, v := range extra {
		vJSON, _ := json.Marshal(v)
		extraPairs = append(extraPairs, fmt.Sprintf("%q:%s", k, vJSON))
	}
	extraStr := ""
	if len(extraPairs) > 0 {
		extraStr = "," + strings.Join(extraPairs, ",")
	}
	cwdJSON, _ := json.Marshal(projectCwd)
	nameJSON, _ := json.Marshal(toolName)
	return fmt.Sprintf(
		`{"cwd":%s,"hookEventName":"PreToolUse","toolCall":{"args":%s,"id":"call_1","name":%s}%s}`,
		cwdJSON, argsJSON, nameJSON, extraStr,
	)
}

// --- git-branch guard tests ---

// TestGitBranchAntigravityPayload_Push asserts that an Antigravity payload
// containing "git push origin main" in toolCall.args.CommandLine is blocked.
//
// Before the ML-3A fix, ExtractCommand returned "" for Antigravity payloads
// (priority 1-4 all miss PascalCase CommandLine) → RunGitBranch returned 0
// silently (T2 of the threat model). After the fix, priority 5 extracts the
// command → MatchSubcommand returns "push" → exit 2.
//
// Conclusion asserted: toolCall.args.CommandLine is now extracted by
// ExtractCommand (priority 5), closing the T2 silent-allow for git push.
func TestGitBranchAntigravityPayload_Push(t *testing.T) {
	dir := makeProjectDir(t)
	payload := antigravityPayload(t, dir, "run_command",
		map[string]string{"CommandLine": "git push origin main"},
		nil,
	)
	code, _, _ := runWith(t, dir, payload, "")
	if code != 2 {
		t.Fatalf("want exit 2 (push blocked), got %d", code)
	}
}

// TestGitBranchAntigravityPayload_AllowLs asserts that an Antigravity payload
// containing "ls" is allowed (exit 0, empty stdout).
//
// Conclusion asserted: non-git commands in toolCall.args.CommandLine are
// allowed silently (MatchSubcommand returns "").
func TestGitBranchAntigravityPayload_AllowLs(t *testing.T) {
	dir := makeProjectDir(t)
	payload := antigravityPayload(t, dir, "run_command",
		map[string]string{"CommandLine": "ls"},
		nil,
	)
	code, stdout, _ := runWith(t, dir, payload, "")
	if code != 0 {
		t.Fatalf("want exit 0 (ls allowed), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("want empty stdout on allow, got %q", stdout)
	}
}

// TestGitBranchAntigravityPayload_T2ProofOldCode demonstrates that the old
// ExtractCommand (priority 1-4 only) returned "" for the Antigravity payload,
// which is the root cause of T2.
//
// Conclusion asserted: the PascalCase "CommandLine" key was absent from all
// four original priorities; priority 5 is the fix that closes T2.
func TestGitBranchAntigravityPayload_T2ProofOldCode(t *testing.T) {
	payload := antigravityPayload(t, "/tmp/proj", "run_command",
		map[string]string{"CommandLine": "git push origin main"},
		nil,
	)
	// Simulate the old behavior: parse the payload WITHOUT priority 5.
	stripped := bytes.TrimPrefix([]byte(payload), []byte{0xEF, 0xBB, 0xBF})
	var root map[string]json.RawMessage
	if err := json.Unmarshal(stripped, &root); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	// Try the four original priorities; none should find the command.
	priorities := []struct {
		name string
		fn   func() (string, bool, error)
	}{
		{"tool_input.command", func() (string, bool, error) { return extractNested(root, "tool_input", "command") }},
		{"tool_info.command_line", func() (string, bool, error) { return extractNested(root, "tool_info", "command_line") }},
		{"hook_input.command", func() (string, bool, error) { return extractNested(root, "hook_input", "command") }},
	}
	for _, p := range priorities {
		cmd, found, err := p.fn()
		if err != nil || found || cmd != "" {
			t.Errorf("priority %s: expected (empty,false,nil), got (%q,%v,%v) — T2 not provable", p.name, cmd, found, err)
		}
	}
	// Root-level "command" key: also absent in the payload.
	if _, ok := root["command"]; ok {
		t.Errorf("root-level 'command' key present — T2 not provable via old priorities")
	}
	// Priority 5 (new): MUST find the command.
	cmd, found, err := extractNested3(root, "toolCall", "args", "CommandLine")
	if err != nil || !found || cmd != "git push origin main" {
		t.Errorf("priority 5 (toolCall.args.CommandLine): expected ('git push origin main',true,nil), got (%q,%v,%v)", cmd, found, err)
	}
}

// --- credential guard tests ---

// TestCredentialAntigravityPayload_SecretInCommandLine asserts that a secret
// in toolCall.args.CommandLine triggers exit 2.
//
// Conclusion asserted: the layer-1 scan of toolCall bytes (D4) detects the
// credential in CommandLine and blocks the command.
func TestCredentialAntigravityPayload_SecretInCommandLine(t *testing.T) {
	projectDir, agentsDir := makeAntigravityProjectDir(t)
	secret := googleFakeKey()
	payload := antigravityPayload(t, projectDir, "run_command",
		map[string]string{"CommandLine": "echo " + secret},
		nil,
	)
	code, stdout, _ := runCred(t, agentsDir, payload)
	if code != 2 {
		t.Fatalf("want exit 2 (credential in CommandLine blocked), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 2, got %q", stdout)
	}
}

// TestCredentialAntigravityPayload_SecretOnlyInLastUserInput asserts that a
// secret present ONLY in lastUserInput does NOT trigger a block.
//
// Conclusion asserted: D4 (layer-1 narrow to toolCall) prevents lastUserInput
// from being scanned; a clean CommandLine is allowed.
func TestCredentialAntigravityPayload_SecretOnlyInLastUserInput(t *testing.T) {
	projectDir, agentsDir := makeAntigravityProjectDir(t)
	secret := googleFakeKey()
	payload := antigravityPayload(t, projectDir, "run_command",
		map[string]string{"CommandLine": "ls"},
		map[string]string{"lastUserInput": "my key is " + secret},
	)
	code, stdout, _ := runCred(t, agentsDir, payload)
	if code != 0 {
		t.Fatalf("want exit 0 (secret only in lastUserInput is allowed), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 0 (allow), got %q", stdout)
	}
}

// TestCredentialAntigravityPayload_Layer2Cat asserts that "cat <file>" in
// CommandLine triggers exit 2 when the file contains a credential.
//
// Conclusion asserted: layer-2b file scan works for Antigravity payloads
// once the command is extracted from toolCall.args.CommandLine.
func TestCredentialAntigravityPayload_Layer2Cat(t *testing.T) {
	projectDir, agentsDir := makeAntigravityProjectDir(t)
	secret := googleFakeKey()
	// Write a file with a secret inside the project dir.
	secretFile := filepath.Join(projectDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte(secret), 0644); err != nil {
		t.Fatalf("write secret.txt: %v", err)
	}
	payload := antigravityPayload(t, projectDir, "run_command",
		map[string]string{"CommandLine": "cat " + secretFile},
		nil,
	)
	code, stdout, _ := runCred(t, agentsDir, payload)
	if code != 2 {
		t.Fatalf("want exit 2 (cat file with secret), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 2, got %q", stdout)
	}
}

// TestCredentialAntigravityPayload_ViewFileWithSecret asserts that a view_file
// tool call pointing to a file containing a credential triggers exit 2.
//
// Conclusion asserted: D5 layer (credViewFileScan) reads toolCall.args.AbsolutePath
// and scans the file — without this, view_file would be a decorative matcher.
func TestCredentialAntigravityPayload_ViewFileWithSecret(t *testing.T) {
	projectDir, agentsDir := makeAntigravityProjectDir(t)
	secret := googleFakeKey()
	secretFile := filepath.Join(projectDir, "config.json")
	if err := os.WriteFile(secretFile, []byte(`{"key":"`+secret+`"}`), 0644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
	payload := antigravityPayload(t, projectDir, "view_file",
		map[string]string{"AbsolutePath": secretFile},
		nil,
	)
	code, stdout, _ := runCred(t, agentsDir, payload)
	if code != 2 {
		t.Fatalf("want exit 2 (view_file of file with secret), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 2, got %q", stdout)
	}
}

// TestCredentialAntigravityPayload_ViewFileInnocuous asserts that a view_file
// tool call pointing to a file WITHOUT a credential does not block.
//
// Conclusion asserted: view_file is not a blanket block; only file content
// containing a credential pattern triggers the guard.
func TestCredentialAntigravityPayload_ViewFileInnocuous(t *testing.T) {
	projectDir, agentsDir := makeAntigravityProjectDir(t)
	innocuousFile := filepath.Join(projectDir, "README.md")
	if err := os.WriteFile(innocuousFile, []byte("# Hello world"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	payload := antigravityPayload(t, projectDir, "view_file",
		map[string]string{"AbsolutePath": innocuousFile},
		nil,
	)
	code, stdout, _ := runCred(t, agentsDir, payload)
	if code != 0 {
		t.Fatalf("want exit 0 (view_file of innocuous file), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 0 (allow), got %q", stdout)
	}
}

// --- D2 invariant: stdout empty on all exit-0 paths ---

// TestGuardStdoutEmpty_D2Invariant is a table test verifying that both guards
// write nothing to stdout on every exit-0 path (ADR-2026-11-18 D2 invariant).
// The Antigravity agent blocks ALL commands if stdout is non-empty JSON on exit 0.
//
// Conclusion asserted: D2 — stdout is empty on every allow path of both
// credential guard and git-branch guard.
func TestGuardStdoutEmpty_D2Invariant(t *testing.T) {
	projectDir, agentsDir := makeAntigravityProjectDir(t)
	secret := googleFakeKey()

	cases := []struct {
		name    string
		guard   string // "cred" or "gitbranch"
		dir     string
		payload string
		wantRC  int
	}{
		{
			name:    "cred/no-trackfw-yaml",
			guard:   "cred",
			dir:     t.TempDir(), // no trackfw.yaml → no-op
			payload: `{"command":"echo hi"}`,
			wantRC:  0,
		},
		{
			name:    "cred/clean-payload",
			guard:   "cred",
			dir:     agentsDir,
			payload: antigravityPayload(t, projectDir, "run_command", map[string]string{"CommandLine": "ls"}, nil),
			wantRC:  0,
		},
		{
			name:  "cred/warn-mode-detection",
			guard: "cred",
			dir: func() string {
				// Use warn mode (not block) to get exit 0 with a detection.
				d := t.TempDir()
				yaml := "credential_guard:\n  mode: warn\n"
				_ = os.WriteFile(filepath.Join(d, "trackfw.yaml"), []byte(yaml), 0644)
				return d
			}(),
			payload: `{"tool_input":{"command":"echo ` + secret + `"}}`,
			wantRC:  0,
		},
		{
			name:    "cred/antigravity-secret-only-in-lastUserInput",
			guard:   "cred",
			dir:     agentsDir,
			payload: antigravityPayload(t, projectDir, "run_command", map[string]string{"CommandLine": "ls"}, map[string]string{"lastUserInput": secret}),
			wantRC:  0,
		},
		{
			name:    "gitbranch/no-project",
			guard:   "gitbranch",
			dir:     t.TempDir(), // no trackfw.yaml → no-op
			payload: `{"tool_input":{"command":"git push origin main"}}`,
			wantRC:  0,
		},
		{
			name:    "gitbranch/allowed-subcommand",
			guard:   "gitbranch",
			dir:     makeProjectDir(t),
			payload: `{"tool_input":{"command":"git status"}}`,
			wantRC:  0,
		},
		{
			name:    "gitbranch/empty-command",
			guard:   "gitbranch",
			dir:     makeProjectDir(t),
			payload: `{"some_other_field":"hello"}`,
			wantRC:  0,
		},
		{
			name:    "gitbranch/antigravity-ls",
			guard:   "gitbranch",
			dir:     makeProjectDir(t),
			payload: antigravityPayload(t, projectDir, "run_command", map[string]string{"CommandLine": "ls"}, nil),
			wantRC:  0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			origDir, _ := os.Getwd()
			if err := os.Chdir(tc.dir); err != nil {
				t.Fatalf("chdir %q: %v", tc.dir, err)
			}
			t.Cleanup(func() { _ = os.Chdir(origDir) })

			stdin := bytes.NewReader([]byte(tc.payload))
			var rc int
			switch tc.guard {
			case "cred":
				rc = RunCredential(stdin, &stdout, &stderr)
			case "gitbranch":
				rc = RunGitBranch(stdin, testIdleTimeout, "", os.Getenv, &stdout, &stderr)
			default:
				t.Fatalf("unknown guard %q", tc.guard)
			}

			if rc != tc.wantRC {
				t.Errorf("rc: want %d got %d (stderr: %q)", tc.wantRC, rc, stderr.String())
			}
			if stdout.Len() > 0 {
				t.Errorf("D2 invariant violated: stdout must be empty on exit 0, got %q", stdout.String())
			}
		})
	}
}

// TestExtractCommand_Priority5_AntigravityCommandLine asserts that priority 5
// (toolCall.args.CommandLine) is used when priorities 1-4 are absent.
//
// Conclusion asserted: the new priority 5 in ExtractCommand closes T2 for the
// git-branch guard by extracting the command from Antigravity payloads.
func TestExtractCommand_Priority5_AntigravityCommandLine(t *testing.T) {
	payload := antigravityPayload(t, "/tmp/proj", "run_command",
		map[string]string{"CommandLine": "git push origin main"},
		nil,
	)
	cmd, err := ExtractCommand([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "git push origin main" {
		t.Fatalf("priority 5: got %q want %q", cmd, "git push origin main")
	}
}

// TestExtractCommand_Priority5_DoesNotOverrideHigherPriority asserts that the
// existing schemas (Claude Code, Codex, Gemini) are unaffected: when
// tool_input.command is present, it takes precedence over toolCall.args.CommandLine.
//
// Conclusion asserted: priority ordering is preserved — existing schemas keep
// their verdicts unchanged (ADR-2026-11-18: "without changing the priority/
// verdict of existing schemas").
func TestExtractCommand_Priority5_DoesNotOverrideHigherPriority(t *testing.T) {
	// Payload has both tool_input.command (priority 1) and toolCall.args.CommandLine
	// (priority 5). Priority 1 must win.
	payload := `{"tool_input":{"command":"echo ok"},"toolCall":{"args":{"CommandLine":"git push origin main"},"name":"run_command"},"hookEventName":"PreToolUse"}`
	cmd, err := ExtractCommand([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "echo ok" {
		t.Fatalf("priority 1 should win over priority 5: got %q want %q", cmd, "echo ok")
	}
}

// TestIsAntigravityPayload_Positive asserts that a payload with toolCall
// object + hookEventName is detected as Antigravity.
//
// Conclusion asserted: isAntigravityPayload correctly identifies the Antigravity
// schema so that layer-1 narrowing and cwd routing are activated.
func TestIsAntigravityPayload_Positive(t *testing.T) {
	var root map[string]json.RawMessage
	payload := `{"hookEventName":"PreToolUse","toolCall":{"name":"run_command","args":{"CommandLine":"ls"}}}`
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !isAntigravityPayload(root) {
		t.Fatal("expected isAntigravityPayload=true for Antigravity payload")
	}
}

// TestIsAntigravityPayload_NegativeClaudeCode asserts that a Claude Code
// PreToolUse payload is NOT detected as Antigravity (no toolCall key).
//
// Conclusion asserted: isAntigravityPayload does not falsely activate for
// Claude Code payloads, preserving their existing behavior.
func TestIsAntigravityPayload_NegativeClaudeCode(t *testing.T) {
	var root map[string]json.RawMessage
	payload := `{"tool_name":"Bash","tool_input":{"command":"ls"}}`
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if isAntigravityPayload(root) {
		t.Fatal("expected isAntigravityPayload=false for Claude Code payload")
	}
}

// TestCredentialAntigravityPayload_WriteToFileWithSecret asserts that a
// write_to_file payload with a secret in CodeContent triggers exit 2.
// This proves the layer-1 narrowing to toolCall did NOT drop write-tool coverage.
//
// Conclusion asserted: D5 matchers include write_to_file; the layer-1 scan of
// toolCall bytes catches the credential in CodeContent.
func TestCredentialAntigravityPayload_WriteToFileWithSecret(t *testing.T) {
	projectDir, agentsDir := makeAntigravityProjectDir(t)
	secret := googleFakeKey()
	// write_to_file has TargetFile and CodeContent args.
	payload := antigravityPayload(t, projectDir, "write_to_file",
		map[string]string{"TargetFile": "/tmp/out.py", "CodeContent": "API_KEY = '" + secret + "'"},
		nil,
	)
	code, stdout, _ := runCred(t, agentsDir, payload)
	if code != 2 {
		t.Fatalf("want exit 2 (credential in write_to_file CodeContent), got %d", code)
	}
	if stdout != "" {
		t.Fatalf("D2 invariant: want empty stdout on exit 2, got %q", stdout)
	}
}
