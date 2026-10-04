package guard

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// jwtToken is a syntactically valid JWT pattern assembled from pieces so that
// the source file itself does not trigger the credential guard hook.
// Pattern: eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+
var jwtToken = "eyJ" + "TESTTOKEN" + "." + "TESTPAYLOAD12345" + "." + "TESTSIG6789"

// awsKey is a syntactically valid AWS access key pattern assembled from pieces.
// Pattern: AKIA[0-9A-Z]{16}
var awsKey = "AKIA" + "TESTKEYTESTKEY12"

// makeCredProjectDir creates a temp directory with a minimal trackfw.yaml.
func makeCredProjectDir(t *testing.T, yamlContent string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	return dir
}

// runCred invokes RunCredential with the given payload from stdin, cwd=dir.
func runCred(t *testing.T, dir, payload string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var stdin io.Reader
	if payload != "" {
		stdin = bytes.NewReader([]byte(payload))
	} else {
		stdin = bytes.NewReader(nil)
	}

	code := RunCredential(stdin, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// runCredGlobal invokes RunCredentialGlobal with the given payload from stdin, cwd=dir.
func runCredGlobal(t *testing.T, dir, payload string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var stdin io.Reader
	if payload != "" {
		stdin = bytes.NewReader([]byte(payload))
	} else {
		stdin = bytes.NewReader(nil)
	}

	code := RunCredentialGlobal(stdin, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestRunCredential_JWTBlocksInBlockMode asserts that a JWT in the payload
// causes exit 2 when credential_guard.mode is "block".
// Assertion: layer 1 JWT detection + block mode → stderr message + exit 2.
func TestRunCredential_JWTBlocksInBlockMode(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("expected exit 2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "trackfw-credential-guard: blocked - possible JWT") {
		t.Fatalf("expected block message on stderr, got %q", errOut)
	}
}

// TestRunCredential_AWSBlocksInBlockMode asserts that an AWS access key in the
// payload causes exit 2 when credential_guard.mode is "block".
// Assertion: layer 1 AWS key detection + block mode → stderr message + exit 2.
func TestRunCredential_AWSBlocksInBlockMode(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + awsKey + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("expected exit 2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "trackfw-credential-guard: blocked - possible AWS access key") {
		t.Fatalf("expected block message for AWS key, got %q", errOut)
	}
}

// TestRunCredential_WarnWritesJSONAndExits0 asserts that in warn mode the guard
// exits 0 and writes the attention-signal JSON to roadmap_dir.
// Assertion: layer 1 JWT detection + warn mode → exit 0 + valid JSON written to roadmap_dir.
func TestRunCredential_WarnWritesJSONAndExits0(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: warn\nroadmap_dir: docs/roadmaps\n")
	// Create roadmap_dir so the file can be written.
	rdDir := filepath.Join(dir, "docs", "roadmaps")
	if err := os.MkdirAll(rdDir, 0755); err != nil {
		t.Fatal(err)
	}

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("expected exit 0 in warn mode, got %d", code)
	}
	if !strings.Contains(errOut, "trackfw-credential-guard: warning - possible JWT") {
		t.Fatalf("expected warning message on stderr, got %q", errOut)
	}

	// Check JSON was written.
	jsonPath := filepath.Join(rdDir, ".trackfw-credential-guard.json")
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("attention JSON not written: %v", err)
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("attention JSON not valid: %v (raw=%q)", err, raw)
	}
	if obj["tool"] != "credential-guard" {
		t.Errorf("JSON tool=%q want %q", obj["tool"], "credential-guard")
	}
	if obj["level"] != "action_required" {
		t.Errorf("JSON level=%q want %q", obj["level"], "action_required")
	}
	if !strings.Contains(obj["message"], "JWT") {
		t.Errorf("JSON message missing match type: %q", obj["message"])
	}
	// Timestamp must match ISO 8601 UTC format.
	tsFmt := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
	if !tsFmt.MatchString(obj["timestamp"]) {
		t.Errorf("JSON timestamp format invalid: %q", obj["timestamp"])
	}
}

// TestRunCredential_EphemeralTargetExempt asserts that a JWT redirected only to
// /dev/null is exempt (exit 0) even in block mode.
// Assertion: all-ephemeral redirect rule → guard is a no-op regardless of mode.
func TestRunCredential_EphemeralTargetExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// Payload: command redirects the token only to /dev/null.
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("expected exit 0 for ephemeral-only redirect, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_MixedTargetAppliesRule asserts that when at least one redirect
// target is non-ephemeral, the guard applies the configured mode (not exempt).
// Assertion: mixed redirects (one ephemeral + one non-ephemeral) → exemption does NOT fire.
func TestRunCredential_MixedTargetAppliesRule(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// Payload: command redirects to /dev/null AND to a plain file → mixed → not exempt.
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` > /dev/null 2> /tmp/real-output.txt"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("expected exit 2 for mixed redirect (not all ephemeral), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_OutsideProjectNoOp asserts that the guard is a no-op when
// there is no trackfw.yaml in the cwd.
// Assertion: absent trackfw.yaml → exit 0 regardless of payload (cwd-only check).
// Also confirms stdin is drained before returning (no EPIPE to the caller).
func TestRunCredential_OutsideProjectNoOp(t *testing.T) {
	// t.TempDir() has no trackfw.yaml.
	dir := t.TempDir()
	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, _ := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("expected exit 0 outside project, got %d", code)
	}
}

// TestRunCredential_InvalidModeFallsToWarn asserts that an unrecognised mode value
// (not "warn" or "block") is treated as "warn".
// Assertion: mode resolution falls back to "warn" for unknown mode values → exit 0.
func TestRunCredential_InvalidModeFallsToWarn(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: paranoid\nroadmap_dir: docs/roadmaps\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps"), 0755); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("invalid mode should fall to warn (exit 0), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "warning") {
		t.Errorf("expected warning message for invalid mode, got %q", errOut)
	}
}

// TestRunCredential_RoadmapDirDotDotFallback asserts that a roadmap_dir value of
// ".." (or any path-traversal form) causes the guard to use the "docs/roadmaps"
// fallback instead, writing the attention JSON there.
// Assertion: path-traversal roadmap_dir → fallback to docs/roadmaps (port of case statement).
func TestRunCredential_RoadmapDirDotDotFallback(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: warn\nroadmap_dir: ..\n")
	// Create the fallback dir so the write succeeds.
	rdFallback := filepath.Join(dir, "docs", "roadmaps")
	if err := os.MkdirAll(rdFallback, 0755); err != nil {
		t.Fatal(err)
	}

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, _ := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("expected exit 0 in warn mode, got %d", code)
	}
	// The file must be in the fallback dir, NOT in the parent directory.
	jsonPath := filepath.Join(rdFallback, ".trackfw-credential-guard.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("attention JSON not written to fallback dir: %v", err)
	}
	// Verify the parent dir was NOT written to.
	parentJSON := filepath.Join(dir, "..", ".trackfw-credential-guard.json")
	if _, err := os.Stat(parentJSON); err == nil {
		t.Error("attention JSON must NOT be written to parent directory")
	}
}

// TestRunCredential_CRLFYamlModeNoBlock asserts that a trackfw.yaml with CRLF
// line endings causes mode "block\r" to NOT match "block", falling back to "warn".
// Assertion: CRLF mode value → does not match "block" → default warn → exit 0.
func TestRunCredential_CRLFYamlModeNoBlock(t *testing.T) {
	// Write trackfw.yaml with CRLF line endings, mode: block\r.
	crlf := "credential_guard:\r\n  mode: block\r\nroadmap_dir: docs/roadmaps\r\n"
	dir := makeCredProjectDir(t, crlf)
	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps"), 0755); err != nil {
		t.Fatal(err)
	}

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	// CRLF mode: "block\r" does not match "block" → falls through to default "warn" → exit 0.
	if code != 0 {
		t.Fatalf("CRLF yaml: expected exit 0 (warn fallback), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "warning") {
		t.Errorf("expected warning message for CRLF yaml, got %q", errOut)
	}
}

// TestRunCredential_SecondLayerRedirectFile asserts that the second detection layer
// scans non-ephemeral redirect target files for credential patterns.
// Assertion: payload without literal credential but redirecting to a file containing
// a JWT → layer 2a finds the match → applies mode.
func TestRunCredential_SecondLayerRedirectFile(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Write a file containing a JWT token.
	tokenFile := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	// Payload: redirects to the token file (no JWT in the payload itself).
	payload := `{"tool_input":{"command":"cat secret > ` + tokenFile + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("layer 2a: expected exit 2 for redirect to JWT file, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_SecondLayerCatArg asserts that the second detection layer (2b)
// scans file arguments of cat/head/tail/jq/grep for credential patterns.
// Assertion: "cat /path/to/token.txt" where the file contains a JWT → layer 2b finds match.
func TestRunCredential_SecondLayerCatArg(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	tokenFile := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	// Payload: command is "cat <file>" — no JWT literal in the payload.
	payload := `{"tool_input":{"command":"cat ` + tokenFile + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("layer 2b: expected exit 2 for cat of JWT file, got %d (stderr=%q)", code, errOut)
	}
}

// --- Glob expansion tests (fix for bash `set -- $CMD_LINE` without set -f) ---

// TestRunCredential_GlobCatStarBlocksInBlockMode asserts that `cat *.txt` in a
// directory where *.txt matches a file containing a JWT is blocked in block mode.
// This is the primary divergence from the previous (pre-fix) Go port: bash's
// `set -- $CMD_LINE` without set -f expands *.txt to matching files; the old Go
// port used strings.Fields only and left *.txt as a literal (no such file → miss).
// Assertion: credGlobToken expands *.txt glob → credScanFile finds JWT → exit 2.
func TestRunCredential_GlobCatStarBlocksInBlockMode(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Write token.txt with a JWT — this is what *.txt should expand to.
	if err := os.WriteFile(filepath.Join(dir, "token.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	// Payload: "cat *.txt" — no JWT literal in the payload itself.
	payload := `{"tool_input":{"command":"cat *.txt"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("glob expansion: expected exit 2 for 'cat *.txt' with JWT in token.txt, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_GlobNoMatchKeepsLiteral asserts that a glob with no matching
// files keeps the literal token (bash without nullglob) — no panic, exit 0.
// Assertion: credGlobToken with unmatched glob → literal token → credScanFile returns "" → exit 0.
func TestRunCredential_GlobNoMatchKeepsLiteral(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// No files matching *.nosuchext in this dir.
	payload := `{"tool_input":{"command":"cat *.nosuchext"}}`
	code, _, _ := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("unmatched glob: expected exit 0 (literal token, no file), got %d", code)
	}
}

// TestRunCredential_GlobMatchesCleanFileAllows asserts that when a glob expands to
// a file that does not contain credentials, the guard allows the command.
// Assertion: credGlobToken expands *.txt → credScanFile finds no credential → exit 0.
func TestRunCredential_GlobMatchesCleanFileAllows(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Write a clean file (no JWT, no AWS key).
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("no secrets here"), 0644); err != nil {
		t.Fatal(err)
	}

	payload := `{"tool_input":{"command":"cat *.txt"}}`
	code, _, _ := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("glob clean file: expected exit 0, got %d", code)
	}
}

// TestRunCredential_GlobMalformedBracketKeepsLiteral asserts that a malformed glob
// pattern (unclosed bracket) causes no panic and keeps the literal token.
// Assertion: filepath.ErrBadPattern from credGlobToken → literal "[abc.txt" → credScanFile → exit 0.
func TestRunCredential_GlobMalformedBracketKeepsLiteral(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Token with unclosed bracket — filepath.Glob returns ErrBadPattern.
	payload := `{"tool_input":{"command":"cat [abc.txt"}}`
	code, _, _ := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("malformed glob: expected exit 0 (literal token), got %d", code)
	}
}

// TestRunCredential_GlobDotfileNotMatched asserts that *.txt does not match .hidden.txt
// (bash default, no dotglob), so a JWT stored in a hidden file is not found via glob.
// Assertion: credGlobToken filters dotfile matches when pattern base has no leading dot → exit 0.
func TestRunCredential_GlobDotfileNotMatched(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Write only a hidden file with JWT — *.txt must NOT match it.
	if err := os.WriteFile(filepath.Join(dir, ".hidden.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	payload := `{"tool_input":{"command":"cat *.txt"}}`
	code, _, _ := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("dotfile filter: *.txt must not match .hidden.txt, expected exit 0, got %d", code)
	}
}

// --- Global scope tests (RunCredentialGlobal / --global) ---

// TestRunCredentialGlobal_JWTDefaultBlocks asserts that in global scope without
// trackfw.yaml the default mode is "block" and a JWT payload exits 2.
// Assertion: RunCredentialGlobal with no trackfw.yaml + JWT → DEFAULT_MODE="block" → exit 2.
func TestRunCredentialGlobal_JWTDefaultBlocks(t *testing.T) {
	// t.TempDir() has no trackfw.yaml → default "block".
	dir := t.TempDir()
	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCredGlobal(t, dir, payload)

	if code != 2 {
		t.Fatalf("global default block: expected exit 2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "blocked") {
		t.Errorf("expected 'blocked' message in stderr, got %q", errOut)
	}
}

// TestRunCredentialGlobal_CleanPayloadAllows asserts that in global scope a payload
// without credentials always exits 0.
// Assertion: RunCredentialGlobal with clean payload → no match → exit 0.
func TestRunCredentialGlobal_CleanPayloadAllows(t *testing.T) {
	dir := t.TempDir()
	payload := `{"tool_input":{"command":"echo hello world"}}`
	code, _, _ := runCredGlobal(t, dir, payload)

	if code != 0 {
		t.Fatalf("global clean: expected exit 0, got %d", code)
	}
}

// TestRunCredentialGlobal_WarnModeFromYamlWritesJSON asserts that in global scope,
// when trackfw.yaml exists with mode=warn, the guard exits 0 and writes the attention
// JSON to the fixed "docs/roadmaps" directory (ignoring roadmap_dir from yaml).
// Assertion: RunCredentialGlobal reads mode from trackfw.yaml → warn → exit 0 + JSON in
// docs/roadmaps (not in yaml's roadmap_dir value "custom/dir").
func TestRunCredentialGlobal_WarnModeFromYamlWritesJSON(t *testing.T) {
	// Create project dir with trackfw.yaml (warn mode, custom roadmap_dir that should be ignored).
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: warn\nroadmap_dir: custom/dir\n")
	// Create the fixed docs/roadmaps directory.
	rdDir := filepath.Join(dir, "docs", "roadmaps")
	if err := os.MkdirAll(rdDir, 0755); err != nil {
		t.Fatal(err)
	}

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCredGlobal(t, dir, payload)

	if code != 0 {
		t.Fatalf("global warn mode: expected exit 0, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "warning") {
		t.Errorf("expected warning message, got %q", errOut)
	}

	// JSON must be in docs/roadmaps (fixed constant), NOT in custom/dir.
	jsonPath := filepath.Join(rdDir, ".trackfw-credential-guard.json")
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("attention JSON not written to docs/roadmaps: %v", err)
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("attention JSON not valid: %v", err)
	}
	if obj["level"] != "action_required" {
		t.Errorf("JSON level=%q want %q", obj["level"], "action_required")
	}

	// Verify custom/dir was NOT created.
	if _, err := os.Stat(filepath.Join(dir, "custom")); err == nil {
		t.Error("global guard must NOT create roadmap_dir from trackfw.yaml — only docs/roadmaps")
	}
}

// TestRunCredentialGlobal_WarnNoDirNoFileCreated asserts that in global warn mode
// when docs/roadmaps does not exist, no file or directory is created and exit is 0.
// Assertion: RunCredentialGlobal warn mode + absent docs/roadmaps → no write + exit 0.
func TestRunCredentialGlobal_WarnNoDirNoFileCreated(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: warn\n")
	// Intentionally do NOT create docs/roadmaps.

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCredGlobal(t, dir, payload)

	if code != 0 {
		t.Fatalf("global warn no-dir: expected exit 0, got %d (stderr=%q)", code, errOut)
	}
	// docs/roadmaps must not have been created.
	if _, err := os.Stat(filepath.Join(dir, "docs", "roadmaps")); err == nil {
		t.Error("global guard must NOT create docs/roadmaps when it doesn't exist")
	}
}
