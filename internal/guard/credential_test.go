package guard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// jwtToken is a syntactically valid JWT pattern assembled from pieces so that
// the source file itself does not trigger the credential guard hook.
// Pattern: eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+
var jwtToken = "eyJ" + "TESTTOKEN" + "." + "TESTPAYLOAD12345" + "." + "TESTSIG6789"

// awsKey is a syntactically valid AWS access key pattern assembled from pieces.
// Pattern: AKIA[0-9A-Z]{16}
var awsKey = "AKIA" + "TESTKEYTESTKEY12"

// jsonStr returns the JSON-encoded content of s, suitable for embedding inside
// a JSON string literal (without the surrounding double-quote characters).
//
// On Windows, filepath separators (\) in paths must be JSON-encoded as \\
// before concatenation into a JSON template string; otherwise the resulting
// payload is syntactically invalid JSON and credExtractCmdAndCwd falls back
// to the non-JSON path where "command_line" and "cwd" keys are not accessible.
//
// Note: json.Marshal escapes '<', '>', '&' to \uXXXX for HTML safety. Paths
// never contain those characters, so this is not an issue for file-path args.
// If you need to embed a general string that may contain those characters, use
// json.NewEncoder with SetEscapeHTML(false) instead.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1]) // strip surrounding quotes
}

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

// TestRunCredential_OutsideProjectNoOp asserts that the credential guard is a no-op
// when there is no trackfw.yaml in the cwd: exit 0, empty stderr, and no attention
// file written even though the payload contains a JWT.
//
// Assertion: the cwd-only trackfw.yaml check fires before any detection or write;
// a JWT payload in a directory without trackfw.yaml produces exit 0, empty stderr,
// and leaves docs/roadmaps/ empty. Falsification: removing the cwd check causes
// warn-mode to trigger — stderr becomes non-empty and an attention file is created.
func TestRunCredential_OutsideProjectNoOp(t *testing.T) {
	// t.TempDir() has no trackfw.yaml.
	// Pre-create docs/roadmaps so credWriteAttention would write there if the
	// cwd check were absent (mode defaults to warn → writes attention JSON).
	dir := t.TempDir()
	roadmapsDir := filepath.Join(dir, "docs", "roadmaps")
	if err := os.MkdirAll(roadmapsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 0 {
		t.Fatalf("expected exit 0 outside project, got %d", code)
	}
	if errOut != "" {
		t.Fatalf("expected empty stderr outside project (cwd-check must fire), got %q", errOut)
	}
	// If the cwd-check were absent, credWriteAttention would create a file here.
	entries, err := os.ReadDir(roadmapsDir)
	if err != nil {
		t.Fatalf("ReadDir docs/roadmaps: %v", err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("docs/roadmaps should be empty outside project, found: %v", names)
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

// TestRunCredential_CRLFYamlModeBlock asserts that a trackfw.yaml with CRLF
// line endings and mode: block correctly reads as "block" (AC1 fix: \r is now stripped).
// Assertion: CRLF mode value "block\r" → \r stripped → "block" → exit 2.
func TestRunCredential_CRLFYamlModeBlock(t *testing.T) {
	// Write trackfw.yaml with CRLF line endings, mode: block\r.
	crlf := "credential_guard:\r\n  mode: block\r\nroadmap_dir: docs/roadmaps\r\n"
	dir := makeCredProjectDir(t, crlf)
	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps"), 0755); err != nil {
		t.Fatal(err)
	}

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	// AC1 (ML-1A): ReadCredentialGuardMode strips \r, so "block\r" → "block" → exit 2.
	if code != 2 {
		t.Fatalf("CRLF yaml: expected exit 2 (block), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "blocked") {
		t.Errorf("expected blocked message for CRLF yaml, got %q", errOut)
	}
}

// TestRunCredential_SecondLayerRedirectFile asserts that the second detection layer
// scans non-ephemeral redirect target files for credential patterns.
//
// Assertion: payload without literal credential but redirecting to a file containing
// a JWT → Layer 2 finds the match → RC=2.
// On POSIX, Layer 2a finds it (redirect target is an absolute path with no ':').
// On Windows, credRedirectRe excludes ':' so the redirect target is truncated to
// the drive letter only (e.g. "C"); Layer 2a misses it. Layer 2b rescues: "cat"
// is argv0 and the absolute path appears as a subsequent token after tokenisation
// by strings.Fields — credScanFile finds the file. This is the pre-PR (main)
// behaviour: main's credCmdLineRe extracted the same command and Layer 2b did the
// same scan. The test assertion (RC=2) is correct on both platforms; the layer
// responsible differs.
// Falsification: remove credNonJSONLayerTwoB call + disable Layer 2b for non-JSON →
// RC=0 on Windows (because Layer 2a truncates at ':' and the non-JSON fallback is gone).
func TestRunCredential_SecondLayerRedirectFile(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Write a file containing a JWT token.
	tokenFile := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	// Payload: redirects to the token file (no JWT in the payload itself).
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"tool_input":{"command":"cat secret > ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("layer 2a: expected exit 2 for redirect to JWT file, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_SecondLayerCatArg asserts that the second detection layer (2b)
// scans file arguments of cat/head/tail/jq/grep for credential patterns.
//
// Assertion: "cat /path/to/token.txt" where the file contains a JWT → Layer 2b
// tokenises the JSON-decoded command; credScanFile receives the decoded path
// as-is (no backslash-escape processing) and opens the file → JWT found → RC=2.
// On Windows the decoded path has backslash separators; os.Stat handles them
// natively without any additional unescaping (H2 measurement for ML-2E).
// Falsification: strip backslash-prefixed chars from Layer 2b tokens → RC=0 on Windows.
func TestRunCredential_SecondLayerCatArg(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	tokenFile := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	// Payload: command is "cat <file>" — no JWT literal in the payload.
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"tool_input":{"command":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)

	if code != 2 {
		t.Fatalf("layer 2b: expected exit 2 for cat of JWT file, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_InvalidJSONFallback_LayerTwoB asserts that when the JSON
// payload contains a syntactically invalid escape sequence ("\q" is not a valid
// JSON escape), credExtractCmdAndCwd returns isJSON=false, but credNonJSONLayerTwoB
// still extracts the "command" field via credCmdLineRe and runs Layer 2b — finding
// the JWT file referenced in the command.
//
// Reconciliation: asserts that credNonJSONLayerTwoB extracts the command via regex
// from non-JSON text and Layer 2b scans the referenced file, returning RC=2.
// This is the production fix for the non-JSON fallback path (ML-2E H1): any
// payload that fails JSON parsing (not just Windows paths) retains Layer 2b coverage
// for "command" field values.
// Falsification: remove the credNonJSONLayerTwoB call in RunCredential → RC=0 (Layer
// 2b silent; Layer 1 found nothing in the payload; no other layer catches the file).
func TestRunCredential_InvalidJSONFallback_LayerTwoB(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// "\q" is not a valid JSON escape — this makes the JSON invalid on every platform.
	// The file path itself has no backslashes (uses the platform separator from TempDir),
	// so the regex extraction works and credScanFile finds the file.
	invalidJSON := `{"x":"\q","tool_input":{"command":"cat ` + tokenFile + `"}}`
	code, _, errOut := runCred(t, dir, invalidJSON)
	if code != 2 {
		t.Fatalf("invalid JSON fallback: expected RC=2 (credNonJSONLayerTwoB extracts command via regex), got %d (stderr=%q)", code, errOut)
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

// TestRunCredential_RoadmapDirSymlinkEscapeIsRefused asserts that when
// roadmap_dir is (or resolves through) a symlink pointing outside the project
// root, the attention JSON is NOT written outside the root, and the guard's
// rc (0) and stderr warning message are unchanged from the no-symlink case.
//
// Assertion: credWriteAttention calls pathguard.RejectAndReport — a roadmapDir
// whose filesystem path escapes the project root via a symlink is refused
// silently; the warn message and exit 0 are the same as the control arm.
func TestRunCredential_RoadmapDirSymlinkEscapeIsRefused(t *testing.T) {
	// "outside" directory lives next to the project, not inside it.
	outside := t.TempDir()

	proj := makeCredProjectDir(t, "credential_guard:\n  mode: warn\nroadmap_dir: docs/roadmaps\n")
	docsDir := filepath.Join(proj, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// docs/roadmaps → outside/ (symlink to a directory outside the project root).
	link := filepath.Join(docsDir, "roadmaps")
	if err := os.Symlink(outside, link); err != nil {
		if isSymlinkPrivilegeError(err) {
			t.Skipf("symlink privilege not available on this platform: %v", err)
		}
		t.Fatalf("os.Symlink: %v", err)
	}

	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	code, _, errOut := runCred(t, proj, payload)

	// rc and warning message must be unchanged (the guard refusal is a side
	// effect on the attention write, not on the guard decision itself).
	if code != 0 {
		t.Fatalf("symlink escape: expected exit 0 (warn mode), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "trackfw-credential-guard: warning - possible JWT") {
		t.Errorf("symlink escape: expected warning message unchanged, got %q", errOut)
	}

	// The file must NOT appear in outside/.
	escapedJSON := filepath.Join(outside, ".trackfw-credential-guard.json")
	if _, err := os.Stat(escapedJSON); err == nil {
		t.Errorf("symlink escape: attention JSON written outside project root at %s — containment guard failed", escapedJSON)
	}
}

// isSymlinkPrivilegeError reports whether err is the "process lacks privilege
// to create symlinks" failure — WinError 1314 on Windows without Developer
// Mode/elevation, or permission-denied on any platform. Detection is by
// condition, not by runtime.GOOS: a Windows host with Developer Mode enabled
// creates symlinks successfully and the test runs normally.
func isSymlinkPrivilegeError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var e syscall.Errno
	if errors.As(err, &e) && e == 1314 {
		return true
	}
	return false
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

// --- ML-1C tests: JSON payload extraction and ephemeral exemption fix ---
//
// The tests below cover three classes of defect corrected in ML-1C:
//
//   R1a: credCmdLineRe did not match "tool_info.command_line" (Windsurf schema),
//        so Layer 2b was blind to files referenced via Windsurf pre_run_command.
//   R1e: credCmdLineRe stopped at the first unescaped " in the raw JSON, so
//        cat "JWTFILE" was truncated to cat and the file was not scanned.
//   EE4: credIsAllEphemeral was evaluated on the entire raw JSON, so "> /dev/null"
//        appearing in write content (new_str, content, edits[*].new_string)
//        triggered the ephemeral exemption even when no shell command was present.
//
// Falsification directions referenced in individual tests:
//   (a) Revert to exemption on raw JSON → Write/Edit/fs_write EE4 cases give RC=0.
//   (b) Remove exemption entirely → echo JWT > /dev/null cases give RC=2.
//   (c) Revert Layer 2b to credCmdLineRe → R1a and R1e cases give RC=0.

// TestRunCredential_WindsurfPreRunCommandCatFile_R1aFix asserts that a Windsurf
// pre_run_command payload with a file path in tool_info.command_line is detected
// by Layer 2b after JSON extraction of the command_line field.
//
// Assertion: JSON parse extracts tool_info.command_line; Layer 2b scans the
// referenced file and finds the JWT — ML-1C fix for R1a (pre-fix RC=0).
// Falsification (c): reverting Layer 2b to credCmdLineRe gives RC=0.
func TestRunCredential_WindsurfPreRunCommandCatFile_R1aFix(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "jwtfile.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Windsurf pre_run_command payload: command is in tool_info.command_line, not tool_input.command.
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"agent_action_name":"pre_run_command","tool_info":{"command_line":"cat ` + jsonStr(tokenFile) + `","cwd":"` + jsonStr(dir) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("R1a: expected RC=2 (Layer 2b via tool_info.command_line), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_WindsurfPreRunCommandClean_Allows asserts that a Windsurf
// pre_run_command payload with a clean command_line (no credential) gives RC=0.
//
// Assertion: JSON extraction of tool_info.command_line does not produce false
// positives — no JWT in the referenced file → RC=0.
func TestRunCredential_WindsurfPreRunCommandClean_Allows(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	cleanFile := filepath.Join(dir, "clean.txt")
	if err := os.WriteFile(cleanFile, []byte("no secrets here"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"agent_action_name":"pre_run_command","tool_info":{"command_line":"cat ` + jsonStr(cleanFile) + `","cwd":"` + jsonStr(dir) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("Windsurf clean command_line: expected RC=0, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_AmazonQExecuteBashCatQuotedPath_R1eFix asserts that a quoted
// file path in Amazon Q execute_bash command is correctly unescaped via JSON decode
// and the file is scanned by Layer 2b.
//
// Assertion: JSON decode of tool_input.command converts \" to "; credResolveArg
// strips the surrounding quotes; credScanFile opens the file and finds JWT —
// ML-1C fix for R1e (pre-fix RC=0 because credCmdLineRe stopped at first ").
// Falsification (c): reverting Layer 2b to credCmdLineRe gives RC=0.
func TestRunCredential_AmazonQExecuteBashCatQuotedPath_R1eFix(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// The JSON value for command has escaped quotes around the path: "cat \"secret.txt\""
	// After JSON decode by credExtractCmdAndCwd, shellCmd = cat "secret.txt".
	// credResolveArg strips the surrounding quotes → secret.txt → scanned.
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"hook_event_name":"PreToolUse","tool_name":"execute_bash","tool_input":{"command":"cat \"` + jsonStr(tokenFile) + `\""}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("R1e: expected RC=2 (quoted path unescaped via JSON decode), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_ClaudeCodeWriteJWTDevNull_EE4Fix asserts that a Claude Code
// Write payload with a JWT and "> /dev/null" embedded in the file content is
// blocked: the ephemeral exemption must NOT fire for write payloads.
//
// Assertion: Write payload has no tool_input.command field; shellCmd is ""; the
// redirect scan on "" finds no redirect; credIsAllEphemeral returns false; Layer 1
// detected the JWT → RC=2 — ML-1C fix for EE4 (pre-fix RC=0).
// Falsification (a): reverting to exemption on raw JSON gives RC=0 (this test fails).
func TestRunCredential_ClaudeCodeWriteJWTDevNull_EE4Fix(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// NOTE: "> /dev/null" is part of the file *content* being written, not a shell redirect.
	payload := `{"tool_name":"Write","tool_input":{"file_path":"/tmp/x.py","content":"` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("EE4 Write: expected RC=2 (no exemption for write content), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_ClaudeCodeEditJWTDevNull_EE4Fix asserts that a Claude Code
// Edit payload with a JWT and "> /dev/null" embedded in new_string is blocked.
//
// Assertion: Edit payload has no tool_input.command field; same mechanism as
// Write — ephemeral exemption does not fire → RC=2 — ML-1C fix for EE4.
// Falsification (a): reverting to exemption on raw JSON gives RC=0 (this test fails).
func TestRunCredential_ClaudeCodeEditJWTDevNull_EE4Fix(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_name":"Edit","tool_input":{"file_path":"/tmp/x.py","new_string":"` + jwtToken + ` > /dev/null","old_string":"old"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("EE4 Edit: expected RC=2 (no exemption for edit content), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_AmazonQFsWriteJWTDevNull_EE4Fix asserts that an Amazon Q
// fs_write str_replace payload with a JWT and "> /dev/null" in new_str is blocked.
//
// Assertion: tool_name "fs_write" causes credExtractCmdAndCwd to return shellCmd="";
// redirect scan on "" finds no redirect; credIsAllEphemeral returns false; Layer 1
// detected the JWT → RC=2 — ML-1C fix for Vector EE4 (pre-fix RC=0).
// Falsification (a): reverting to exemption on raw JSON gives RC=0 (this test fails).
func TestRunCredential_AmazonQFsWriteJWTDevNull_EE4Fix(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// new_str contains JWT + "> /dev/null" — an attempted bypass via EE4.
	payload := `{"hook_event_name":"PreToolUse","tool_name":"fs_write","tool_input":{"command":"str_replace","path":"/p/f.py","old_str":"old","new_str":"` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("EE4 fs_write: expected RC=2 (tool_name fs_write, no shell exemption), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_AmazonQExecuteBashEchoJWTDevNull_ExemptionPreserved asserts
// that the ephemeral exemption is preserved for Amazon Q execute_bash when the
// JWT is inlined in the shell command redirected only to /dev/null.
//
// Assertion: shellCmd extracted via tool_input.command contains the JWT; redirect
// scan on shellCmd finds "> /dev/null"; credIsAllEphemeral returns true; ML-1C
// must not regress the bash-echo exemption for execute_bash — RC=0.
// Falsification (b): removing the exemption gives RC=2 (this test fails).
func TestRunCredential_AmazonQExecuteBashEchoJWTDevNull_ExemptionPreserved(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"hook_event_name":"PreToolUse","tool_name":"execute_bash","tool_input":{"command":"echo ` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("execute_bash echo to /dev/null: expected RC=0 (ephemeral exemption), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_WindsurfEchoJWTDevNull_ExemptionPreserved asserts that the
// ephemeral exemption is preserved for Windsurf pre_run_command when the JWT is
// inlined in the command_line value redirected only to /dev/null.
//
// Assertion: shellCmd extracted via tool_info.command_line contains the JWT;
// redirect scan on shellCmd finds "> /dev/null"; credIsAllEphemeral returns true;
// ML-1C must not regress the echo exemption for Windsurf — RC=0.
// Falsification (b): removing the exemption gives RC=2 (this test fails).
func TestRunCredential_WindsurfEchoJWTDevNull_ExemptionPreserved(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// jsonStr encodes backslashes in Windows paths so the cwd field is valid JSON
	// and credExtractCmdAndCwd returns isJSON=true, enabling the correct JSON
	// exemption path (credIsSimpleCmd on the extracted command, not on raw text).
	payload := `{"agent_action_name":"pre_run_command","tool_info":{"command_line":"echo ` + jwtToken + ` > /dev/null","cwd":"` + jsonStr(dir) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("Windsurf echo to /dev/null: expected RC=0 (ephemeral exemption), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_WindsurfPreWriteCodeSingleEdit_Blocks asserts that a Windsurf
// pre_write_code payload with a single edit containing a JWT is blocked.
//
// Assertion: pre_write_code has no command field; shellCmd is ""; Layer 1 detects
// the JWT; no ephemeral exemption fires → RC=2. This matches the pre-ML-1C
// behavior (the single-edit case was already RC=2 by accident of JSON structure,
// now it is RC=2 by correct logic).
func TestRunCredential_WindsurfPreWriteCodeSingleEdit_Blocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"agent_action_name":"pre_write_code","tool_info":{"file_path":"/f.py","edits":[{"old_string":"old","new_string":"` + jwtToken + `"}]}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("Windsurf pre_write_code single edit: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_WindsurfPreWriteCodeTwoEditsJWTDevNull_Blocks asserts that a
// Windsurf pre_write_code payload with TWO edits — the first containing a JWT and
// "> /dev/null" — is blocked.
//
// Assertion: In the pre-ML-1C code, the redirect regex stopped at the "," separator
// between edits, leaving "> /dev/null"}" whose target cleaned to "/dev/null",
// triggering the ephemeral exemption (RC=0). With ML-1C, shellCmd is "" for
// pre_write_code; redirect scan on "" finds no redirect; exemption does not fire →
// RC=2. This is a new finding: the two-edit case was a bypass in the pre-fix code.
// Falsification (a): reverting to exemption on raw JSON gives RC=0 (this test fails).
func TestRunCredential_WindsurfPreWriteCodeTwoEditsJWTDevNull_Blocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// Two edits: first has JWT + "> /dev/null", second is clean.
	// The "> /dev/null"," pattern in raw JSON was exploitable pre-ML-1C.
	payload := `{"agent_action_name":"pre_write_code","tool_info":{"file_path":"/f.py","edits":[{"old_string":"","new_string":"` + jwtToken + ` > /dev/null"},{"new_string":"second"}]}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("Windsurf pre_write_code two-edits bypass: expected RC=2 (ML-1C structurally correct), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_WindsurfCommandLineCwdResolution asserts that a relative path
// in tool_info.command_line is resolved against tool_info.cwd (not the process cwd).
//
// Assertion: the process cwd is a project dir without token.txt; tool_info.cwd
// points to a separate dir containing token.txt; "cat token.txt" is resolved to
// cwd/token.txt via credResolveArg; credScanFile finds JWT → RC=2.
func TestRunCredential_WindsurfCommandLineCwdResolution(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Separate dir with the token file — not in the process cwd.
	tokenDir := t.TempDir()
	tokenFile := filepath.Join(tokenDir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	// command_line uses a relative path; cwd points to tokenDir.
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"agent_action_name":"pre_run_command","tool_info":{"command_line":"cat token.txt","cwd":"` + jsonStr(tokenDir) + `"}}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 2 {
		t.Fatalf("cwd resolution: expected RC=2 (relative path resolved via tool_info.cwd), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_CwdResolution_MissWithoutCwd asserts that the same relative
// path WITHOUT a tool_info.cwd is not found when the token file is in a
// different directory than the process cwd.
//
// Assertion: without cwd, "cat token.txt" resolves against the process cwd
// (projDir); token.txt is in tokenDir only → credScanFile finds no file → RC=0.
// This confirms that cwd resolution is the mechanism that makes the previous
// test pass, not an accidental match.
func TestRunCredential_CwdResolution_MissWithoutCwd(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	tokenDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tokenDir, "token.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}

	// No cwd field — relative path resolves against process cwd (projDir), token.txt absent there.
	payload := `{"agent_action_name":"pre_run_command","tool_info":{"command_line":"cat token.txt"}}`
	code, _, _ := runCred(t, projDir, payload)
	if code != 0 {
		t.Fatalf("no cwd: expected RC=0 (relative path not found in process cwd), got %d", code)
	}
}

// TestRunCredential_AmazonQFsWriteClean_Allows asserts that an Amazon Q fs_write
// payload without a JWT in the content is allowed.
//
// Assertion: tool_name "fs_write" → shellCmd=""; Layer 1 finds no JWT → RC=0.
// No false positive from the fs_write tool_name exclusion.
func TestRunCredential_AmazonQFsWriteClean_Allows(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"hook_event_name":"PreToolUse","tool_name":"fs_write","tool_input":{"command":"create","path":"/p/f.py","file_text":"hello world"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("fs_write clean: expected RC=0, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_AmazonQFsWriteJWTCreate_Blocks asserts that an Amazon Q
// fs_write "create" payload with a JWT in file_text is blocked by Layer 1.
//
// Assertion: tool_name "fs_write" → shellCmd=""; Layer 1 finds JWT in file_text;
// no ephemeral exemption → RC=2.
func TestRunCredential_AmazonQFsWriteJWTCreate_Blocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"hook_event_name":"PreToolUse","tool_name":"fs_write","tool_input":{"command":"create","path":"/p/f.py","file_text":"` + jwtToken + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("fs_write JWT in file_text: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// --- Global scope (RunCredentialGlobal) variants ---

// TestRunCredentialGlobal_AmazonQFsWriteJWTDevNull_EE4Fix asserts that the EE4
// fix applies in global scope as well as project scope.
//
// Assertion: RunCredentialGlobal has the same JSON-aware extraction and exemption
// logic as RunCredential; fs_write with JWT + "> /dev/null" → RC=2.
// Falsification (a): reverting global scope to raw-JSON exemption gives RC=0.
func TestRunCredentialGlobal_AmazonQFsWriteJWTDevNull_EE4Fix(t *testing.T) {
	// No trackfw.yaml → global default mode = "block".
	dir := t.TempDir()
	payload := `{"hook_event_name":"PreToolUse","tool_name":"fs_write","tool_input":{"command":"str_replace","path":"/p/f.py","old_str":"old","new_str":"` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCredGlobal(t, dir, payload)
	if code != 2 {
		t.Fatalf("global EE4 fs_write: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredentialGlobal_WindsurfCatFile_R1aFix asserts that the R1a fix applies
// in global scope.
//
// Assertion: RunCredentialGlobal extracts tool_info.command_line via JSON parse;
// Layer 2b scans the referenced file; JWT found → RC=2.
// Falsification (c): reverting Layer 2b to credCmdLineRe gives RC=0.
func TestRunCredentialGlobal_WindsurfCatFile_R1aFix(t *testing.T) {
	dir := t.TempDir() // no trackfw.yaml → global default block
	tokenFile := filepath.Join(dir, "jwtfile.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"agent_action_name":"pre_run_command","tool_info":{"command_line":"cat ` + jsonStr(tokenFile) + `","cwd":"` + jsonStr(dir) + `"}}`
	code, _, errOut := runCredGlobal(t, dir, payload)
	if code != 2 {
		t.Fatalf("global R1a: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// --------------------------------------------------------------------------
// ML-1D (REQ-2026-10-06) AC5 — Codex PreToolUse payload tests
//
// Codex CLI uses PreToolUse[matcher:"Bash"] with tool_input.command (string).
// The hook payload schema is identical to Claude Code for the command field.
// Source: internal/generators/agentfiles.go (InjectCodexHooks), docs/pesquisa/
// 2026-08-12-semantica-de-falha-de-hook-codex.md. tool_input.command is a
// string (not an array) in PreToolUse payloads; extractNested returns "" for
// non-string values (see payload.go:219 "Non-string JSON value → treat as absent").
// --------------------------------------------------------------------------

// TestRunCredential_CodexPreToolUseCatFile_Blocks asserts that a Codex PreToolUse
// payload with tool_input.command = "cat <file>" where the file contains a JWT
// is blocked.
//
// Assertion: credExtractCmdAndCwd extracts shellCmd via tool_input.command; Layer 2b
// glob-expands the path and scans the file; JWT found → RC=2. This is the same
// mechanism as Claude Code tests using tool_input.command (same schema).
// Falsification (d): write an empty tool_input.command → extractor blind → RC=0.
func TestRunCredential_CodexPreToolUseCatFile_Blocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Codex PreToolUse payload: hook_event_name PreToolUse, tool_name Bash,
	// tool_input.command is a plain cat command with the absolute path.
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("Codex cat file: expected RC=2 (JWT in file), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_CodexPreToolUseCatQuotedFile_Blocks asserts that a Codex
// PreToolUse payload with a JSON-escaped quoted path ("cat \"<file>\"") is blocked.
//
// Assertion: JSON decode of tool_input.command unescapes \" → "; credResolveArg
// strips the surrounding quotes → plain path → Layer 2b scans the file → RC=2.
// Same mechanism as TestRunCredential_AmazonQExecuteBashCatQuotedPath_R1eFix.
// Falsification (d): reverting to credCmdLineRe (which stopped at first ") → RC=0.
func TestRunCredential_CodexPreToolUseCatQuotedFile_Blocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// JSON-encoded: command = cat "<tokenFile>" — the file path is quoted.
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat \"` + jsonStr(tokenFile) + `\""}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("Codex cat quoted file: expected RC=2 (JWT in file), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_CodexPreToolUseEchoJWTDevNull_Exempt asserts that the
// ephemeral exemption fires for a Codex PreToolUse echo command that redirects
// the JWT only to /dev/null (no credential materialization).
//
// Assertion: credExtractCmdAndCwd extracts shellCmd via tool_input.command;
// redirect scan on shellCmd finds "> /dev/null"; credIsAllEphemeral returns true;
// Layer 1 found the JWT but all redirects are ephemeral → RC=0.
// Same mechanism as Claude Code echo-to-devnull exemption.
// Falsification (b): removing credIsAllEphemeral check → RC=2 (this test fails).
func TestRunCredential_CodexPreToolUseEchoJWTDevNull_Exempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo ` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("Codex echo to /dev/null: expected RC=0 (ephemeral exemption), got %d (stderr=%q)", code, errOut)
	}
}

// --------------------------------------------------------------------------
// F3 tests (ML-2C): BOM in JSON payload must not disable Layer 2b
// --------------------------------------------------------------------------

// TestRunCredential_BOMPayload_LayerTwoStillActive asserts that a UTF-8 BOM
// prepended to a valid JSON payload does not disable Layer 2b — the cat-file
// command must still be detected and blocked.
//
// Reconciliação: afirma que `json.Unmarshal(stripped, &root)` (não `data`) é
// usado em credExtractCmdAndCwd — BOM não desativa Layer 2b — cat de arquivo
// com JWT com BOM deve retornar RC=2 (regressão do ML-1C).
// Falsificação: reverter para json.Unmarshal(data, &root) → isJSON=false → RC=0.
func TestRunCredential_BOMPayload_LayerTwoStillActive(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Prepend UTF-8 BOM (0xEF 0xBB 0xBF) to the JSON payload.
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	bom := []byte{0xEF, 0xBB, 0xBF}
	payload := string(bom) + `{"tool_input":{"command":"cat ` + jsonStr(tokenFile) + `"}}`

	var stdout, stderr bytes.Buffer
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	code := RunCredential(bytes.NewReader([]byte(payload)), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("BOM + cat file: expected RC=2 (Layer 2b active despite BOM), got %d (stderr=%q)", code, stderr.String())
	}
}

// TestRunCredential_BOMPayload_EchoDevNull_StillExempt asserts that the ephemeral
// exemption still fires for a BOM-prefixed echo-to-/dev/null payload (BOM fix
// does not regress the exemption).
//
// Reconciliação: afirma que BOM não destrói a isenção efêmera — echo JWT >
// /dev/null com BOM deve retornar RC=0 (a isenção depende de credIsSimpleCmd +
// credAllTargetsAreDevNull, não de isJSON falso).
func TestRunCredential_BOMPayload_EchoDevNull_StillExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	bom := []byte{0xEF, 0xBB, 0xBF}
	payload := string(bom) + `{"tool_input":{"command":"echo ` + jwtToken + ` > /dev/null"}}`

	var stdout, stderr bytes.Buffer
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	code := RunCredential(bytes.NewReader([]byte(payload)), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("BOM + echo to /dev/null: expected RC=0 (ephemeral exemption), got %d (stderr=%q)", code, stderr.String())
	}
}

// --------------------------------------------------------------------------
// F4 tests (ML-2C): deep walk of all command/command_line values
// --------------------------------------------------------------------------

// TestRunCredential_DeepCommand_NonStandardSchema_Blocks asserts that a payload
// with "command" nested under a non-standard key ("params") is detected by the
// F4 deep walk and blocked.
//
// Reconciliação: afirma que credDeepScan encontra "cat <file>" em params.command
// e retorna match — RC=2 (antes do F4: RC=0, os 4 paths fixos não cobriam params).
// Falsificação: remover credDeepScan call → RC=0.
func TestRunCredential_DeepCommand_NonStandardSchema_Blocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"params":{"command":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("deep params.command cat file: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_PriorityOneCommandWithSecondaryMalicious_Blocks asserts that a
// payload with tool_input.command="echo hi" (primary, no JWT) AND
// tool_info.command_line="cat <file>" (secondary, JWT in file) is blocked — F4
// changes the previous "Priority-1 wins" RC=0 to RC=2 (intended by ML-2C).
//
// Reconciliação: afirma que credDeepScan varre também o command_line secundário —
// o arquivo referenciado tem JWT → RC=2 (comportamento anterior: RC=0 pois Layer 2b
// só via primary shellCmd="echo hi" → no match).
// Falsificação: remover credDeepScan call → RC=0.
func TestRunCredential_PriorityOneCommandWithSecondaryMalicious_Blocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// jsonStr encodes backslashes in Windows paths so the payload is valid JSON.
	payload := `{"tool_input":{"command":"echo hi"},"tool_info":{"command_line":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("priority-1 echo + secondary cat file: expected RC=2 (F4), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_FSWriteDeepScan_Excluded asserts that an fs_write payload with
// a "command" field inside tool_input is NOT scanned for credentials — the fs_write
// exclusion prevents treating the enum tag as a shell command in the deep walk.
//
// Reconciliação: afirma que credDeepScan exclui tool_input de payloads fs_write —
// um arquivo com JWT não é detectado via tool_input.command="str_replace" → RC=0.
// Falsificação: remover o delete(m, "tool_input") em credDeepScan → se o arquivo
// existe, RC=2 (falso positivo para fs_write).
func TestRunCredential_FSWriteDeepScan_Excluded(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// fs_write: tool_input.command carries the enum tag "str_replace", not a shell command.
	// The payload itself does not contain any JWT pattern.
	payload := `{"tool_name":"fs_write","tool_input":{"command":"str_replace","path":"/tmp/f","old_str":"a","new_str":"b"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("fs_write deep scan excluded: expected RC=0, got %d (stderr=%q)", code, errOut)
	}
}

// --------------------------------------------------------------------------
// F2 tests (ML-2D): shape-based exemption gate
// --------------------------------------------------------------------------

// TestRunCredential_EchoPipe_NotExempt asserts that "echo JWT | tee out.txt > /dev/null"
// is NOT exempt because '|' is a pipeline metacharacter → credIsSimpleCmd=false → RC=2.
//
// Reconciliação: afirma que credHasShellMeta detecta '|' → credIsSimpleCmd=false →
// exemption não dispara → RC=2.
// Falsificação: remover '|' do credHasShellMeta switch → RC=0 (bypass).
func TestRunCredential_EchoPipe_NotExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` | tee out.txt > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("echo pipe tee: expected RC=2 (pipeline meta), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_PrintfDevNullFdDup_Exempt asserts that
// "printf JWT > /dev/null 2>&1" IS exempt — 2>&1 is an fd-dup redirect
// (credIsRedirectAmpersand returns true), not a metacharacter, and printf is in
// the argv0 allowlist.
//
// Reconciliação: afirma que credIsRedirectAmpersand identifica '&' em 2>&1 como
// fd-dup (não meta), argv0="printf" está na allowlist, target="/dev/null" exato →
// RC=0.
// Falsificação: tratar '&' em 2>&1 como meta → RC=2.
func TestRunCredential_PrintfDevNullFdDup_Exempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"printf ` + jwtToken + ` > /dev/null 2>&1"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("printf > /dev/null 2>&1: expected RC=0 (fd-dup not meta, printf in allowlist), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_EchoBackground_NotExempt asserts that
// "echo JWT > /dev/null & python3 -c 'x'" is NOT exempt because '&' (background
// operator) is a standalone shell metacharacter → credIsSimpleCmd=false → RC=2.
//
// Reconciliação: afirma que '&' não precedido por '>' é meta (background op) →
// credHasShellMeta=true → RC=2.
// Falsificação: tratar todo '&' como fd-dup → RC=0 (bypass).
func TestRunCredential_EchoBackground_NotExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` > /dev/null & python3 -c 'x'"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("echo background: expected RC=2 (background meta), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_EchoSemicolon_NotExempt asserts that
// "echo JWT > /dev/null; cat arq" is NOT exempt because ';' is a command
// separator metacharacter → credIsSimpleCmd=false → RC=2.
//
// Reconciliação: afirma que ';' → credHasShellMeta=true → RC=2.
// Falsificação: remover ';' do credHasShellMeta switch → RC=0.
func TestRunCredential_EchoSemicolon_NotExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` > /dev/null; cat arq"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("echo semicolon cat: expected RC=2 (semicolon meta), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_Python3_NotExempt asserts that "python3 -c 'print(JWT)' > /dev/null"
// is NOT exempt because argv0="python3" is not in {echo, printf} → credIsSimpleCmd=false → RC=2.
//
// Reconciliação: afirma que argv0="python3" ∉ {echo,printf} → credIsSimpleCmd=false → RC=2.
// Falsificação: expandir allowlist para incluir python3 → RC=0 (bypass).
func TestRunCredential_Python3_NotExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"python3 -c 'print(` + jwtToken + `)' > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("python3 not in allowlist: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_EchoMktempFile_NotExempt asserts that "echo JWT > mktemp.txt"
// is NOT exempt because the target "/dev/null" requirement fails for "mktemp.txt"
// (credAllTargetsAreDevNull closes the mktemp-filename bypass).
//
// Reconciliação: afirma que credAllTargetsAreDevNull rejeita "mktemp.txt" como alvo —
// even though credIsEphemeralTarget sees "mktemp" substring and returns true,
// credAllTargetsAreDevNull requires exactly "/dev/null" → RC=2.
// Falsificação: remover credAllTargetsAreDevNull check → RC=0 (bypass via filename).
func TestRunCredential_EchoMktempFile_NotExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` > mktemp.txt"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("echo to mktemp.txt: expected RC=2 (mktemp filename bypass closed), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_EchoFileRedirectAmpersand_NotExempt asserts that
// "echo JWT > /dev/null >&out.txt" is NOT exempt: '>&out.txt' has '&' preceded
// by '>' but followed by 'o' (non-digit, non-'-') so credIsRedirectAmpersand
// returns false → credHasShellMeta=true → RC=2.
//
// Reconciliação: afirma que '>&word' (non-digit word) é meta (file redirect em bash,
// não fd-dup) → credHasShellMeta=true → RC=2.
// Falsificação: aceitar qualquer '&' precedido por '>' como fd-dup → RC=0 (bypass).
func TestRunCredential_EchoFileRedirectAmpersand_NotExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` > /dev/null >&out.txt"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("echo >&out.txt: expected RC=2 (file redirect ampersand is meta), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_DdHeredocJWT_NotExempt asserts that "dd of=arq <<< JWT > /dev/null"
// is NOT exempt because argv0="dd" is not in {echo, printf}.
//
// Reconciliação: afirma que argv0="dd" ∉ {echo,printf} → credIsSimpleCmd=false → RC=2.
// Falsificação: expandir allowlist para incluir dd → RC=0.
func TestRunCredential_DdHeredocJWT_NotExempt(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"dd of=arq <<< ` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("dd heredoc: expected RC=2 (dd not in allowlist), got %d (stderr=%q)", code, errOut)
	}
}

// --------------------------------------------------------------------------
// ML-3B tests: Windows path normalization (BUG-1 + BUG-2)
// --------------------------------------------------------------------------

// TestCredNormalizeWindowsPath asserts the translation rules of credNormalizeWindowsPath
// for every combination of goos and toolName that matters.
//
// This is a pure unit test with no I/O — it runs on every OS, including macOS/Linux
// CI where the Windows path forms cannot be opened but can still be verified as strings.
//
// Assertion per row is noted in the "frase de reconciliação" column in the spec table.
func TestCredNormalizeWindowsPath(t *testing.T) {
	cases := []struct {
		path     string
		goos     string
		toolName string
		want     string
		note     string
	}{
		// Git-Bash lower case drive
		{"/c/Users/x/s.env", "windows", "Bash", "C:/Users/x/s.env",
			"normalização ativa no Windows com toolName=Bash"},
		// Git-Bash upper case drive
		{"/C/Users/x/s.env", "windows", "Bash", "C:/Users/x/s.env",
			"case-insensitive na regex (letra de drive maiúscula)"},
		// Cygwin mount
		{"/cygdrive/c/Users/x", "windows", "Bash", "C:/Users/x",
			"prefixo /cygdrive/c/ traduzido"},
		// GOOS gate: Linux must be unchanged
		{"/c/Users/x/s.env", "linux", "Bash", "/c/Users/x/s.env",
			"gate GOOS removido causaria FN em POSIX"},
		// GOOS gate: darwin must be unchanged
		{"/c/Users/x/s.env", "darwin", "Bash", "/c/Users/x/s.env",
			"gate GOOS removido causaria FN em macOS"},
		// MSYS path without drive letter: unchanged
		{"/tmp/s.env", "windows", "Bash", "/tmp/s.env",
			"regex over-broad causaria FN para caminhos MSYS sem drive"},
		{"/home/user/s.env", "windows", "Bash", "/home/user/s.env",
			"idem para /home"},
		// Windows-native paths: unchanged
		{`C:\Users\x\s.env`, "windows", "Bash", `C:\Users\x\s.env`,
			"regressão: forma nativa backslash não alterada"},
		{"C:/Users/x/s.env", "windows", "Bash", "C:/Users/x/s.env",
			"regressão: forma nativa forward-slash não alterada"},
		// Relative path: unchanged
		{"relative/s.env", "windows", "Bash", "relative/s.env",
			"regressão: relativo não alterado"},
		// PowerShell deny-list (exact case)
		{"/c/Users/x/s.env", "windows", "PowerShell", "/c/Users/x/s.env",
			"deny-list PS (EqualFold): tradução não aplicada para PowerShell"},
		// PowerShell deny-list (lowercase — EqualFold)
		{"/c/Users/x/s.env", "windows", "powershell", "/c/Users/x/s.env",
			"EqualFold: variação lowercase de PowerShell também excluída"},
		// Amazon Q tool_name (deny-list only excludes PS)
		{"/c/Users/x/s.env", "windows", "execute_bash", "C:/Users/x/s.env",
			"Amazon Q (deny-list só exclui PS) → traduz"},
		// Windsurf (tool_name absent → "")
		{"/c/Users/x/s.env", "windows", "", "C:/Users/x/s.env",
			"Windsurf (tool_name ausente → \"\") cai no deny-list → traduzido"},
		// Bare drive letter alone
		{"/c", "windows", "Bash", "C:/",
			"/c sozinho → C:/ (não C: que é drive-relative)"},
		// Cygdrive bare
		{"/cygdrive/c", "windows", "Bash", "C:/",
			"/cygdrive/c sozinho → C:/"},
		// UNC: unchanged
		{`\\server\share\file`, "windows", "Bash", `\\server\share\file`,
			"UNC não alterado"},
		// R1: pwsh and powershell.exe added to deny-list (PowerShell 7 uses PS provider semantics)
		{"/c/Users/x/s.env", "windows", "pwsh", "/c/Users/x/s.env",
			"R1: pwsh (PowerShell 7) na deny-list — não traduz /c/..."},
		{"/c/Users/x/s.env", "windows", "pwsh.exe", "/c/Users/x/s.env",
			"R1: pwsh.exe na deny-list — não traduz /c/..."},
		{"/c/Users/x/s.env", "windows", "powershell.exe", "/c/Users/x/s.env",
			"R1: powershell.exe na deny-list — não traduz /c/..."},
		{"/c/Users/x/s.env", "windows", "PWSH.EXE", "/c/Users/x/s.env",
			"R1: PWSH.EXE (EqualFold) na deny-list — não traduz /c/..."},
		// Control: non-PS tool still translates after R1 deny-list expansion
		{"/c/Users/x/s.env", "windows", "Bash", "C:/Users/x/s.env",
			"R1 controle: Bash não está na deny-list → traduz normalmente"},
	}

	for _, tc := range cases {
		got := credNormalizeWindowsPath(tc.path, tc.goos, tc.toolName)
		if got != tc.want {
			t.Errorf("credNormalizeWindowsPath(%q, %q, %q) = %q; want %q — %s",
				tc.path, tc.goos, tc.toolName, got, tc.want, tc.note)
		}
	}
}

// TestCredRedirectRe_WindowsPathExtractedFull asserts that credRedirectRe extracts
// the FULL Windows drive path (not just the drive letter) from redirect expressions.
//
// Assertion: removing ':' from the exclusion class allows credRedirectRe to capture
// C:\... and C:/... in redirect targets — BUG-2 fix. Falsification: restoring ':'
// to the class causes this test to fail because the match is truncated to "C".
func TestCredRedirectRe_WindowsPathExtractedFull(t *testing.T) {
	cases := []struct {
		input string
		want  string // expected path after stripping redirect prefix
	}{
		{`echo hi > C:\Users\x\s.env`, `C:\Users\x\s.env`},
		{`echo hi > C:/Users/x/s.env`, `C:/Users/x/s.env`},
		{`echo hi > /c/Users/x/s.env`, `/c/Users/x/s.env`},
		{`echo hi > /dev/null`, `/dev/null`},
		{`echo hi > s.env`, `s.env`},
	}
	for _, tc := range cases {
		matches := credRedirectRe.FindAllString(tc.input, -1)
		if len(matches) == 0 {
			t.Errorf("no match in %q", tc.input)
			continue
		}
		got := credRedirectTarget(matches[0])
		if got != tc.want {
			t.Errorf("redirect target from %q = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// TestCredRedirectRe_DevNullExemptionUnchanged asserts that /dev/null extraction is
// unaffected by the BUG-2 regex change (removing ':' does not affect /dev/null).
//
// Assertion: credAllTargetsAreDevNull relies on credRedirectTarget returning "/dev/null";
// if '/dev/null' extraction changed after the regex fix this test would catch it.
// Falsification: restoring ':' changes the match for C:\ but not for /dev/null — this
// test passes in both cases, confirming the fix has no effect on the /dev/null exemption.
func TestCredRedirectRe_DevNullExemptionUnchanged(t *testing.T) {
	inputs := []string{
		`echo key > /dev/null`,
		`echo key > /dev/null 2>&1`,
		`printf key > /dev/null`,
	}
	for _, input := range inputs {
		matches := credRedirectRe.FindAllString(input, -1)
		found := false
		for _, m := range matches {
			target := credRedirectTarget(m)
			target = strings.Map(func(r rune) rune {
				if r == '"' || r == '\'' { return -1 }
				return r
			}, target)
			target = strings.TrimRight(target, "},")
			if target == "/dev/null" {
				found = true
			}
		}
		if !found {
			t.Errorf("input %q: /dev/null not extracted from redirects %v", input, matches)
		}
	}
}

// TestCredResolveArg_GitBashPath_POSIXUnchanged asserts that on non-Windows systems
// credResolveArg returns the POSIX absolute path unchanged — the GOOS gate in
// credNormalizeWindowsPath must not translate /c/... on macOS/Linux.
//
// Assertion: goos gate prevents credNormalizeWindowsPath from running on POSIX →
// credResolveArg("/c/Users/x/s.env", "/some/cwd", "Bash") == "/c/Users/x/s.env".
// Falsification: removing the goos gate → output "C:/Users/x/s.env" on macOS, which
// filepath.IsAbs returns false for → path treated as relative → join with cwd → miss.
func TestCredResolveArg_GitBashPath_POSIXUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX test — skipped on Windows")
	}
	// On POSIX, /c/... is a valid absolute path; it must not be translated.
	got := credResolveArg("/c/Users/x/s.env", "/some/cwd", "Bash")
	if got != "/c/Users/x/s.env" {
		t.Errorf("POSIX: credResolveArg(/c/Users/x/s.env) = %q; want %q (goos gate broken)", got, "/c/Users/x/s.env")
	}
	// With a relative arg, cwd join must still work normally.
	got2 := credResolveArg("s.env", "/some/cwd", "Bash")
	if got2 != "/some/cwd/s.env" {
		t.Errorf("POSIX relative resolution: credResolveArg(s.env, /some/cwd) = %q; want /some/cwd/s.env", got2)
	}
}

// --- Windows-only integration tests ---
// These tests skip on non-Windows because the file paths they create via t.TempDir()
// are POSIX, and they require os.Stat to open Windows-native paths.
// Using t.Skip (not build tags) so that `go test -run '^Name$' -v` reports "SKIP"
// on macOS rather than silently not finding the test.

// TestRunCredential_GitBashArgSubcaseA_Windows asserts that when tool_info.cwd is
// absent from the payload (sub-caso A, baseCwd=""), a Git-Bash path in the "cat"
// argument is still detected on Windows after path normalization.
//
// Assertion: credNormalizeWindowsPath translates /c/...→C:/... before the
// credScanFile call — os.Stat succeeds — AWS key found → RC=2 (sub-caso A coberto).
// Falsification: removing credNormalizeWindowsPath call in credResolveArg → RC=0.
func TestRunCredential_GitBashArgSubcaseA_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	// Convert C:\path\s.env to /c/path/s.env (Git-Bash form)
	gitBashPath := windowsPathToGitBash(tokenFile)
	// No tool_info.cwd in payload — sub-caso A
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat ` + jsonStr(gitBashPath) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("sub-caso A /c/ arg: expected RC=2 (normalização → C:/…), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_GitBashArgSubcaseB_Windows asserts that when tool_info.cwd is
// present as a native Windows path (sub-caso B), a Git-Bash argument is detected.
//
// Assertion: credNormalizeWindowsPath translates /c/...→C:/... → IsAbs=true →
// cwd join skipped — credScanFile finds file — AWS key detected → RC=2 (sub-caso B coberto).
// Falsification: removing credNormalizeWindowsPath → IsAbs("/c/...")=false on Windows
// → filepath.Join("C:\\...", "/c/...") = "C:\c\..." → stat fails → RC=0.
func TestRunCredential_GitBashArgSubcaseB_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	gitBashPath := windowsPathToGitBash(tokenFile)
	// tool_info.cwd is native Windows — sub-caso B
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat ` + jsonStr(gitBashPath) + `"},"tool_info":{"cwd":"` + jsonStr(dir) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("sub-caso B /c/ arg + cwd nativo: expected RC=2 (normalização → IsAbs=true), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_GitBashUppercaseArg_Windows asserts that /C/... (uppercase drive)
// is detected on Windows.
//
// Assertion: case-insensitive regex in credNormalizeWindowsPath handles /C/ → C:/
// — AWS key found → RC=2.
// Falsification: case-sensitive regex for drive letter → /C/ unmatched → RC=0.
func TestRunCredential_GitBashUppercaseArg_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	gitBashPath := windowsPathToGitBash(tokenFile)
	// Convert /c/ to /C/ (uppercase)
	gitBashPathUpper := strings.Replace(gitBashPath, "/c/", "/C/", 1)
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat ` + jsonStr(gitBashPathUpper) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("/C/ uppercase arg: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_CygdriveArg_Windows asserts that /cygdrive/c/... is detected on Windows.
//
// Assertion: credNormalizeWindowsPath translates /cygdrive/c/ → C:/ → stat succeeds → RC=2.
// Falsification: removing cygdrive pattern in credNormalizeWindowsPath → RC=0.
func TestRunCredential_CygdriveArg_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	gitBashPath := windowsPathToGitBash(tokenFile)
	// /c/... → /cygdrive/c/...
	cygdrivePath := strings.Replace(gitBashPath, "/c/", "/cygdrive/c/", 1)
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat ` + jsonStr(cygdrivePath) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("/cygdrive/c/ arg: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_GitBashCwd_Windows asserts that /c/... as tool_info.cwd (with
// a relative argument) is detected on Windows.
//
// Assertion: credExtractToolInfoCwd normalizes /c/... cwd → C:/... → relative arg
// is joined against correct cwd → credScanFile finds file → RC=2.
// Falsification: removing normalization in credExtractToolInfoCwd → cwd remains
// /c/... → filepath.Join("/c/..", "s.env") = "/c/../s.env" → stat fails → RC=0.
func TestRunCredential_GitBashCwd_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	gitBashDir := windowsPathToGitBash(dir)
	// tool_info.cwd is Git-Bash form; arg is relative
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat s.env"},"tool_info":{"cwd":"` + jsonStr(gitBashDir) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("/c/ cwd + relative arg: expected RC=2 (cwd normalizado), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_RedirectWindowsNative_Windows asserts that Layer 2a detects an
// AWS key in a file referenced by a native Windows redirect (C:\... or C:/...).
//
// Assertion: BUG-2 fix (removing ':' from credRedirectRe) extracts the full Windows
// path → credResolveArg normalizes (no-op for C:/..., already native) → credScanFile
// finds file → RC=2.
// Falsification: restoring ':' to credRedirectRe → redirect target truncated to "C" →
// credScanFile("C") fails → RC=0.
func TestRunCredential_RedirectWindowsNative_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	// Use forward-slash native Windows path (avoid JSON backslash complexity)
	fwdSlashPath := strings.ReplaceAll(tokenFile, `\`, `/`)
	// Layer 2a: no key in command itself, key is in redirect target file
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hi > ` + jsonStr(fwdSlashPath) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("redirect C:/... native: expected RC=2 (BUG-2 fix), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_RedirectGitBash_Windows asserts that Layer 2a detects an AWS key
// in a file referenced by a Git-Bash redirect (/c/...).
//
// Assertion: regex extracts full /c/... target → credNormalizeWindowsPath → C:/... →
// credScanFile finds file → RC=2.
// Falsification: removing credNormalizeWindowsPath in credResolveArg → os.Stat("/c/...")
// fails on Windows → RC=0.
func TestRunCredential_RedirectGitBash_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	gitBashPath := windowsPathToGitBash(tokenFile)
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hi > ` + jsonStr(gitBashPath) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("redirect /c/ git-bash: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_PowerShellGitBashPath_Windows asserts that a PowerShell payload
// with a /c/... path is NOT detected (expected miss — PS opens C:\c\..., not C:\...).
//
// Assertion: deny-list gate strings.EqualFold("PowerShell") suppresses translation →
// credResolveArg("/c/...", ..., "PowerShell") = "/c/..." → os.Stat fails on Windows →
// RC=0. This is the CORRECT behavior: PS would open C:\c\... but we can't scan that
// without false-positive risk on legitimate PS paths. Falsification: removing deny-list
// → translation fires → guard scans C:/... instead of C:\c\... → RC=2 (wrong scan).
func TestRunCredential_PowerShellGitBashPath_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	gitBashPath := windowsPathToGitBash(tokenFile)
	// tool_name = "PowerShell" → deny-list → no translation → miss (expected)
	payload := `{"hook_event_name":"PreToolUse","tool_name":"PowerShell","tool_input":{"command":"cat ` + jsonStr(gitBashPath) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("PowerShell /c/ path: expected RC=0 (deny-list: PS uses C:\\c\\... semantics), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_DevNullExempt_Windows asserts that the /dev/null exemption
// is unaffected by the BUG-2 regex change on Windows.
//
// Assertion: echo <key> > /dev/null on Windows — credAllTargetsAreDevNull returns
// true → F2 exemption fires → RC=0. The regex fix does not change /dev/null extraction.
// Falsification: restoring ':' to credRedirectRe changes C:\ extracts but /dev/null
// is still "/dev/null" (no ':') — this test passes in both states, confirming no regression.
func TestRunCredential_DevNullExempt_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	awsKeyVal := "AKIA" + "TESTKEYTESTKEY12"
	// AWS key in the command → Layer 1 matches; but redirect is /dev/null → F2 exempts
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo ` + awsKeyVal + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("Windows /dev/null exemption: expected RC=0 (F2 exemption preserved), got %d (stderr=%q)", code, errOut)
	}
}

// TestCredWindowsADSBase asserts the NTFS Alternate Data Stream base-path extractor.
//
// Assertion: credWindowsADSBase returns the segment before the ADS colon for each
// Windows path with a qualifying colon, and returns ("", false) for paths without
// an ADS suffix or when goos≠"windows".
// Falsification: removing the startSearch drive-skip causes "C:/x/a.txt" to produce
// ("C", true) from the drive colon — the "C:/x/a.txt (no ADS)" row catches that.
func TestCredWindowsADSBase(t *testing.T) {
	cases := []struct {
		path string
		goos string
		want string
		ok   bool
		note string
	}{
		{"arq.txt:stream", "windows", "arq.txt", true,
			"stream nomeado — colon não é drive-letter → base extraída"},
		{"arq.txt::$DATA", "windows", "arq.txt", true,
			"::$DATA (stream padrão) → base extraída"},
		{"C:/x/a.txt:s", "windows", "C:/x/a.txt", true,
			"colon de drive (idx 1) ignorado; colon ADS após .txt extraído"},
		{"C:/x/a.txt", "windows", "", false,
			"sem ADS — nenhum colon fora da posição de drive → false"},
		{"C:", "windows", "", false,
			"bare drive letter — sem colon fora de idx 1 → false"},
		{`\\server\share\a:s`, "windows", `\\server\share\a`, true,
			"UNC: sem drive-letter colon; primeiro colon é ADS"},
		{"relative/s.env", "windows", "", false,
			"relativo sem colon → false"},
		{"/c/Users/x/s.env:stream", "linux", "", false,
			"goos linux → false (POSIX gate)"},
	}
	for _, tc := range cases {
		got, ok := credWindowsADSBase(tc.path, tc.goos)
		if got != tc.want || ok != tc.ok {
			t.Errorf("credWindowsADSBase(%q, %q) = (%q, %v); want (%q, %v) — %s",
				tc.path, tc.goos, got, ok, tc.want, tc.ok, tc.note)
		}
	}
}

// TestRunCredential_ADSNamedStreamFallback_Windows asserts that Layer 2a detects an
// AWS key in "s.env" when the redirect target is "s.env:stream" (a named NTFS ADS
// that does not exist on disk) via the F1 ADS fallback in credSecondLayer.
//
// Assertion: credWindowsADSBase("C:/.../s.env:stream", "windows")="C:/.../s.env" →
// os.Stat("C:/.../s.env:stream") fails → credScanFile scans the base file and finds
// the AWS key → RC=2.
// Falsification: removing the ADS fallback block (the runtime.GOOS=="windows" block
// in credSecondLayer 2a) → os.Stat fails, no fallback → RC=0.
func TestRunCredential_ADSNamedStreamFallback_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only integration test")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "s.env")
	key := "AKIA" + "TESTKEYTESTKEY12"
	if err := os.WriteFile(tokenFile, []byte("aws_access_key_id = "+key), 0644); err != nil {
		t.Fatal(err)
	}
	// ADS target: C:/.../s.env:stream — the stream does not exist; the base s.env has the key.
	// Use forward-slash path (avoids JSON backslash escaping complexity).
	fwdSlashBase := strings.ReplaceAll(tokenFile, `\`, `/`)
	adsTarget := fwdSlashBase + ":stream"
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hi > ` + jsonStr(adsTarget) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("ADS redirect s.env:stream: expected RC=2 (F1 fallback scans base), got %d (stderr=%q)", code, errOut)
	}
}

// windowsPathToGitBash converts a Windows absolute path (C:\path\to\file) to the
// Git-Bash mount point form (/c/path/to/file). Only for test use on Windows.
func windowsPathToGitBash(winPath string) string {
	// Replace backslashes with forward slashes
	p := strings.ReplaceAll(winPath, `\`, `/`)
	// C:/path/file → /c/path/file
	if len(p) >= 2 && p[1] == ':' {
		drive := strings.ToLower(string(p[0]))
		rest := p[2:] // "/path/file"
		return "/" + drive + rest
	}
	return p
}

// --------------------------------------------------------------------------
// ML-1A tests (D1, D4): origin message, path sanitization, D4 glob cwd fix
// --------------------------------------------------------------------------

// TestCredSanitizePath_ControlChars asserts that credSanitizePath replaces control
// characters, bidi controls and invalid UTF-8 bytes with '?'.
//
// Assertion: credSanitizePath substitutes chars where unicode.IsControl or
// unicode.Is(unicode.Bidi_Control) is true, and also invalid UTF-8 bytes, with '?'.
// This prevents file names containing LF (0x0A), raw C1 bytes (0x9B) or bidi
// controls from injecting content into the guard's stderr line.
func TestCredSanitizePath_ControlChars(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"LF newline", "/path/a\nb.txt", "/path/a?b.txt"},
		{"CR", "/path/a\rb.txt", "/path/a?b.txt"},
		{"ESC (0x1B)", "/path/a\x1bb.txt", "/path/a?b.txt"},
		{"DEL (0x7F)", "/path/a\x7fb.txt", "/path/a?b.txt"},
		{"raw C1 byte (0x9B)", "/path/a\x9bb.txt", "/path/a?b.txt"},
		{"bidi RLO (U+202E)", "/path/a\u202eb.txt", "/path/a?b.txt"},
		{"bidi LRE (U+202A)", "/path/a\u202ab.txt", "/path/a?b.txt"},
		{"valid ASCII unchanged", "/path/to/file.go", "/path/to/file.go"},
		{"empty string", "", ""},
	}
	for _, tc := range cases {
		got := credSanitizePath(tc.input)
		if got != tc.want {
			t.Errorf("[%s] credSanitizePath(%q) = %q; want %q", tc.name, tc.input, got, tc.want)
		}
	}
}

// TestRunCredential_D1_PayloadMessage asserts that a credential found in the tool
// payload itself produces the "in tool payload." suffix in the blocked message.
//
// Assertion: layer 1 finds the JWT in the payload (no file reference); the message
// must say "in tool payload." and not contain "in file". This confirms that the
// payload-origin form is preserved after the D1 refactor.
func TestRunCredential_D1_PayloadMessage(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
	_, _, errOut := runCred(t, dir, payload)
	if !strings.Contains(errOut, "in tool payload.") {
		t.Errorf("payload origin: expected 'in tool payload.' in stderr, got %q", errOut)
	}
	if strings.Contains(errOut, "in file") {
		t.Errorf("payload origin: stderr must not contain 'in file', got %q", errOut)
	}
}

// TestRunCredential_D1_FileMessage_ProjectScope asserts that a credential found in a
// file referenced by a "cat" command produces "in file <path> referenced by the command."
// in block mode, project scope.
//
// Assertion: credSecondLayer finds JWT in token.txt via Layer 2b; the message contains
// the file path and the "referenced by the command." suffix. Payload origin is NOT used.
// This is the primary fix for the REQ-2026-11-06 misdiagnosis.
func TestRunCredential_D1_FileMessage_ProjectScope(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_input":{"command":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("file origin: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "in file") {
		t.Errorf("file origin: expected 'in file' in stderr, got %q", errOut)
	}
	if !strings.Contains(errOut, "referenced by the command.") {
		t.Errorf("file origin: expected 'referenced by the command.' in stderr, got %q", errOut)
	}
	if strings.Contains(errOut, "in tool payload.") {
		t.Errorf("file origin: stderr must not contain 'in tool payload.', got %q", errOut)
	}
}

// TestRunCredential_D1_FileMessage_WarnMode asserts that in warn mode the stderr
// message also uses the file-origin form, and the attention JSON does NOT contain
// the file path (ADR D1: path leakage prevention).
//
// Assertion: credSecondLayer finds JWT in token.txt; mode=warn produces the
// "in file ... referenced by the command." message on stderr; the attention JSON
// field "message" contains "in a file referenced by the command" and does NOT
// contain the file path basename.
func TestRunCredential_D1_FileMessage_WarnMode(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: warn\nroadmap_dir: docs/roadmaps\n")
	rdDir := filepath.Join(dir, "docs", "roadmaps")
	if err := os.MkdirAll(rdDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Use a distinctive file name to verify it does NOT appear in the JSON.
	tokenFile := filepath.Join(dir, "distinctive-secret-file.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_input":{"command":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("warn mode: expected RC=0, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "in file") {
		t.Errorf("warn mode: expected 'in file' in stderr, got %q", errOut)
	}
	// Attention JSON must round-trip and must NOT contain the file path.
	jsonPath := filepath.Join(rdDir, ".trackfw-credential-guard.json")
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("attention JSON not written: %v", err)
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("attention JSON not valid JSON: %v (raw=%q)", err, raw)
	}
	if !strings.Contains(obj["message"], "in a file referenced by the command") {
		t.Errorf("attention JSON message: expected 'in a file referenced by the command', got %q", obj["message"])
	}
	// The distinctive file basename must NOT appear in the JSON bytes.
	if strings.Contains(string(raw), "distinctive-secret-file") {
		t.Errorf("attention JSON must NOT contain the file path, but got %q", string(raw))
	}
}

// TestRunCredential_D1_FileMessage_GlobalScope asserts that the global scope also
// produces the file-origin message form.
//
// Assertion: RunCredentialGlobal uses credFinding from credSecondLayer; the blocked
// message contains "in file ... referenced by the command." (not "in tool payload.").
func TestRunCredential_D1_FileMessage_GlobalScope(t *testing.T) {
	dir := t.TempDir() // no trackfw.yaml, default mode = block
	tokenFile := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_input":{"command":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCredGlobal(t, dir, payload)
	if code != 2 {
		t.Fatalf("global file origin: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "in file") {
		t.Errorf("global file origin: expected 'in file' in stderr, got %q", errOut)
	}
	if strings.Contains(errOut, "in tool payload.") {
		t.Errorf("global file origin: stderr must not contain 'in tool payload.', got %q", errOut)
	}
}

// TestRunCredential_D1_ControlCharInFilename asserts that a file with a newline
// and "::error::" in its name produces exactly one line of stderr and that no line
// starts with "::" (which would be interpreted as a workflow command in GitHub Actions).
//
// Assertion: credSanitizePath replaces the '\n' with '?', ensuring the blocked
// message is a single line. Without sanitization, the second line would start with
// "::error::", poisoning the CI log.
func TestRunCredential_D1_ControlCharInFilename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("filename with newline is not supported on Windows")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	// Create a file whose name contains "\n::error::x" — only achievable as a
	// real filesystem name on POSIX.
	evilName := "a\n::error::x.txt"
	evilFile := filepath.Join(dir, evilName)
	if err := os.WriteFile(evilFile, []byte(jwtToken), 0644); err != nil {
		t.Fatalf("create evil filename: %v", err)
	}

	// Use a glob payload so that credGlobToken expands the wildcard and returns
	// the file name from filepath.Glob (the name comes from the filesystem, not
	// from the command string).
	payload := `{"tool_input":{"command":"cat *.txt"},"tool_info":{"cwd":"` + jsonStr(dir) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("evil filename: expected RC=2 (JWT in file), got %d (stderr=%q)", code, errOut)
	}
	// Exactly one newline in stderr = exactly one line.
	lineCount := strings.Count(errOut, "\n")
	if lineCount != 1 {
		t.Errorf("evil filename: expected exactly 1 newline in stderr, got %d (stderr=%q)", lineCount, errOut)
	}
	// No line may start with "::".
	for _, line := range strings.Split(errOut, "\n") {
		if strings.HasPrefix(line, "::") {
			t.Errorf("evil filename: stderr line starts with '::' (workflow command injection risk): %q", line)
		}
	}
}

// TestRunCredential_D4_GlobRelativeCwd asserts that a relative glob token ("*.txt")
// is expanded against tool_info.cwd when it differs from the process working directory.
//
// Assertion: credGlobToken now receives baseCwd (tool_info.cwd) and uses it to
// resolve the relative glob. Without D4, credGlobToken expanded relative globs
// against the process cwd, missing files in a different tool cwd. With D4, "*.txt"
// in tokenDir is found even though the process cwd is projDir.
func TestRunCredential_D4_GlobRelativeCwd(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// tokenDir is different from projDir (the process cwd during the test).
	tokenDir := t.TempDir()
	tokenFile := filepath.Join(tokenDir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Payload: command is "cat *.txt" with tool_info.cwd = tokenDir.
	// Pre-D4: credGlobToken expanded "*.txt" against projDir → no match → RC=0.
	// Post-D4: credGlobToken expands "*.txt" against tokenDir → secret.txt found → RC=2.
	payload := `{"tool_input":{"command":"cat *.txt"},"tool_info":{"cwd":"` + jsonStr(tokenDir) + `"}}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 2 {
		t.Fatalf("D4 glob cwd: expected RC=2 (glob resolved against tool_info.cwd), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "in file") {
		t.Errorf("D4 glob cwd: expected 'in file' in stderr, got %q", errOut)
	}
}

// TestRunCredential_D4_GlobRelativeCwd_NoMatch asserts that when tool_info.cwd does
// NOT contain a matching file, the glob yields no match and the guard allows (RC=0).
//
// Assertion: this is the control arm for TestRunCredential_D4_GlobRelativeCwd.
// Without a credential file in tokenDir, the glob expands to nothing → RC=0.
// This confirms that the D4 fix produces the detection, not an accidental match.
func TestRunCredential_D4_GlobRelativeCwd_NoMatch(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	emptyDir := t.TempDir()
	// No files in emptyDir — glob *.txt matches nothing.
	payload := `{"tool_input":{"command":"cat *.txt"},"tool_info":{"cwd":"` + jsonStr(emptyDir) + `"}}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 0 {
		t.Fatalf("D4 glob cwd no match: expected RC=0, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_D4_GlobRootLevelCwd asserts that a root-level "cwd" field
// (Kiro PreToolUse schema, documented at cli-parity.md §"Kiro wiring") is used
// to resolve relative glob patterns, same as tool_info.cwd is used for Windsurf.
//
// Assertion: payload `{"tool_name":"Bash","tool_input":{"command":"cat *.txt"},"cwd":<tokenDir>}`.
// credExtractCmdAndCwd falls back to root-level "cwd" when tool_info.cwd is absent.
// credGlobToken expands "*.txt" against tokenDir → finds the credential file → RC=2.
func TestRunCredential_D4_GlobRootLevelCwd(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenDir := t.TempDir()
	tokenFile := filepath.Join(tokenDir, "secret.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Claude Code / Kiro PreToolUse: "cwd" is a root-level field (not tool_info.cwd).
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat *.txt"},"cwd":"` + jsonStr(tokenDir) + `"}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 2 {
		t.Fatalf("D4 root-level cwd: expected RC=2 (glob resolved against root cwd), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "in file") {
		t.Errorf("D4 root-level cwd: expected 'in file' in stderr, got %q", errOut)
	}
}

// TestRunCredential_D4_NamedArgRootLevelCwd asserts that a root-level "cwd" field
// (Kiro PreToolUse schema) also resolves NAMED arguments, not only globs.
//
// Assertion: `{"tool_input":{"command":"cat token.txt"},"cwd":<tokenDir>}`.
// credResolveArg resolves the relative "token.txt" against baseCwd (now populated
// from the root "cwd" field). Without the fallback, the file is not found (RC=0).
// This is the control arm for TestRunCredential_D4_GlobRootLevelCwd, confirming
// that the cwd fallback applies to both named args and globs.
func TestRunCredential_D4_NamedArgRootLevelCwd(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tokenDir, "token.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Named arg resolved via root-level "cwd".
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat token.txt"},"cwd":"` + jsonStr(tokenDir) + `"}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 2 {
		t.Fatalf("D4 named arg root-cwd: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_D4_NamedArgRootLevelCwd_NoMatch is the control arm: without a
// root "cwd", the relative "token.txt" is not found in the process cwd (projDir).
//
// Assertion: when "cwd" is absent, credResolveArg uses process cwd (projDir); since
// token.txt lives only in tokenDir, credScanFile finds nothing → RC=0.
func TestRunCredential_D4_NamedArgRootLevelCwd_NoMatch(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tokenDir, "token.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// No "cwd" field — relative path resolves against process cwd (projDir), not tokenDir.
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat token.txt"}}`
	code, _, _ := runCred(t, projDir, payload)
	if code != 0 {
		t.Fatalf("D4 named arg no root-cwd: expected RC=0, got %d", code)
	}
}

// --------------------------------------------------------------------------
// A-1 corretivo (ML-2B): credGlobEscapeLiteral — metacharacters in baseCwd
// --------------------------------------------------------------------------

// TestCredGlobEscapeLiteral_UnitVectors asserts the escape helper produces
// correct output for each metacharacter class and their combinations.
//
// Assertion: credGlobEscapeLiteral treats "[", "*" and "?" as literal characters
// in a cwd path, and on non-Windows it also escapes literal backslashes before
// introducing bracket-escape sequences — no sequential-replace corruption occurs.
func TestCredGlobEscapeLiteral_UnitVectors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		skip  string // non-empty → t.Skip on that GOOS
	}{
		{
			name:  "bracket",
			input: "/tmp/d[1]",
			// "[" → "[[]"; "]" is not a glob metacharacter in Go's filepath.Glob
			// (only "[" opens a class). The trailing "]" closes the "[[]" class,
			// making "[[]" match a literal "[", and the remaining "]" is literal.
			want: "/tmp/d[[]1]",
		},
		{
			name:  "star",
			input: "/tmp/a*b",
			want:  "/tmp/a[*]b",
		},
		{
			name:  "question",
			input: "/tmp/a?b",
			want:  "/tmp/a[?]b",
		},
		{
			name:  "mixed bracket-star-question",
			input: "/tmp/a[*?b",
			want:  "/tmp/a[[][*][?]b",
		},
		{
			name:  "backslash then bracket (non-windows only)",
			input: `a\[b`,
			want:  `a\\[[]b`,
			skip:  "windows",
		},
		{
			name:  "clean path without metacharacters",
			input: "/tmp/clean/path",
			want:  "/tmp/clean/path",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip != "" && runtime.GOOS == tc.skip {
				t.Skipf("skipped on %s: filesystem does not allow this path component", tc.skip)
			}
			got := credGlobEscapeLiteral(tc.input)
			if got != tc.want {
				t.Errorf("credGlobEscapeLiteral(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestRunCredential_A1_CwdBracket asserts that when baseCwd contains a bracket
// character (e.g. "d[1]"), the glob token is expanded against that literal directory,
// not misinterpreted as a character class.
//
// Assertion: cwd="<tmp>/d[1]" containing secret.txt with a JWT, command "cat *.txt"
// → RC=2 and stderr contains "in file". Without credGlobEscapeLiteral, "[1]" is
// parsed as a char class matching "1", no directory named that way exists, so
// filepath.Glob returns no matches and the token passes unchecked (rc=0).
func TestRunCredential_A1_CwdBracket(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	base := t.TempDir()
	tokenDir := filepath.Join(base, "d[1]")
	if err := os.Mkdir(tokenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "secret.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat *.txt"},"cwd":"` + jsonStr(tokenDir) + `"}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 2 {
		t.Fatalf("A1 bracket cwd: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "in file") {
		t.Errorf("A1 bracket cwd: expected 'in file' in stderr, got %q", errOut)
	}
}

// TestRunCredential_A1_CwdUnclosedBracket asserts that when baseCwd contains an
// unclosed bracket (which makes ErrBadPattern in filepath.Glob), the guard still
// detects the credential in the file — it does not silently pass.
//
// Assertion: cwd="<tmp>/d[1" containing secret.txt with a JWT, command "cat *.txt"
// → RC=2. Without credGlobEscapeLiteral, filepath.Glob returns ErrBadPattern and
// credGlobToken falls back to the literal token "*.txt", which is not a path that
// exists on disk, so the scan is skipped and RC=0 (false negative).
func TestRunCredential_A1_CwdUnclosedBracket(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	base := t.TempDir()
	tokenDir := filepath.Join(base, "d[1")
	if err := os.Mkdir(tokenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "secret.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat *.txt"},"cwd":"` + jsonStr(tokenDir) + `"}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 2 {
		t.Fatalf("A1 unclosed bracket cwd: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "in file") {
		t.Errorf("A1 unclosed bracket cwd: expected 'in file' in stderr, got %q", errOut)
	}
}

// TestRunCredential_A1_CwdStarSiblingNoFalsePositive asserts that when baseCwd
// contains a star (e.g. "a*b"), the glob does not expand to a sibling directory
// (e.g. "axb") that happens to match the star pattern.
//
// Assertion: cwd="<tmp>/a*b" (empty), sibling "<tmp>/axb/secret.txt" contains JWT,
// command "cat *.txt" → RC=0. Without credGlobEscapeLiteral, "a*b" in the prefix
// expands to "axb", which contains the credential file, producing a false positive
// (rc=2 on the wrong file).
func TestRunCredential_A1_CwdStarSiblingNoFalsePositive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("NTFS does not allow '*' in directory names")
	}
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	base := t.TempDir()
	// cwd that the tool reports: a*b (empty — no credential here)
	tokenDir := filepath.Join(base, "a*b")
	if err := os.Mkdir(tokenDir, 0755); err != nil {
		t.Fatal(err)
	}
	// sibling directory that the star would accidentally match
	siblingDir := filepath.Join(base, "axb")
	if err := os.MkdirAll(siblingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siblingDir, "secret.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat *.txt"},"cwd":"` + jsonStr(tokenDir) + `"}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 0 {
		t.Fatalf("A1 star sibling: expected RC=0 (no credential in reported cwd), got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_A1_CwdBracket_NamedArgControl asserts that the named-arg path
// (credResolveArg, not credGlobToken) also works correctly when baseCwd contains a
// bracket character.
//
// Assertion: cwd="<tmp>/d[1]" containing secret.txt, command "cat secret.txt" →
// RC=2. This tests that credResolveArg (which uses filepath.Join without escaping,
// but passes the joined path to os.Stat, not to filepath.Glob) still resolves the
// file correctly. This is the control arm confirming the bracket-cwd fix does not
// break named-arg resolution.
func TestRunCredential_A1_CwdBracket_NamedArgControl(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	base := t.TempDir()
	tokenDir := filepath.Join(base, "d[1]")
	if err := os.Mkdir(tokenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "secret.txt"), []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Named arg — resolved via credResolveArg (os.Stat, not filepath.Glob).
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat secret.txt"},"cwd":"` + jsonStr(tokenDir) + `"}`
	code, _, errOut := runCred(t, projDir, payload)
	if code != 2 {
		t.Fatalf("A1 bracket cwd named arg: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
}

// TestRunCredential_D1_F2_EphemeralStillPasses asserts that the F2 ephemeral
// exemption continues to work correctly after the D1 refactor — the finding.typ
// is still used (not the path) when checking credCmdHasTypeMatch.
//
// Assertion: "echo <jwt> > /dev/null" with a JWT in the payload; Layer 1 matches;
// the exemption fires because argv0=echo, all targets are /dev/null, and the
// finding carries typ="JWT" (not a file path). RC=0 after D1 refactor.
func TestRunCredential_D1_F2_EphemeralStillPasses(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	payload := `{"tool_input":{"command":"echo ` + jwtToken + ` > /dev/null"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 0 {
		t.Fatalf("D1 F2 regression: expected RC=0 (ephemeral exemption), got %d (stderr=%q)", code, errOut)
	}
}

// --------------------------------------------------------------------------
// D1 coverage: five arrival paths
// --------------------------------------------------------------------------

// TestRunCredential_D1_FileMessage_L2a asserts that a credential found via Layer 2a
// (non-ephemeral redirect target scan) produces the "detected in file ... referenced
// by the command." message, not "detected in tool payload.".
//
// Arrival path: L2a (credSecondLayer, redirect target).
// Assertion: a file containing a JWT, referenced via output redirect in the payload,
// produces "detected in file <path> referenced by the command." in stderr.
// Counter-arm: changing credFindingMessage to always return "detected in tool payload."
// makes this test fail (the wrong message is emitted).
func TestRunCredential_D1_FileMessage_L2a(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "redirect-target.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Command redirects to tokenFile; tokenFile already contains a JWT.
	// Layer 2a scans the redirect target file and finds the credential.
	// No credential appears in the payload itself — only in the file.
	payload := `{"tool_input":{"command":"cat secret > ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("L2a file origin: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "detected in file") {
		t.Errorf("L2a file origin: expected 'detected in file' in stderr, got %q", errOut)
	}
	if !strings.Contains(errOut, "referenced by the command.") {
		t.Errorf("L2a file origin: expected 'referenced by the command.' in stderr, got %q", errOut)
	}
	if strings.Contains(errOut, "detected in tool payload.") {
		t.Errorf("L2a file origin: stderr must not say 'detected in tool payload.', got %q", errOut)
	}
}

// TestRunCredential_D1_FileMessage_L2bNonJSON asserts that a credential found via
// Layer 2b for a non-JSON payload produces the "detected in file ... referenced by
// the command." message.
//
// Arrival path: L2b-nonJSON (credNonJSONLayerTwoB via credCmdLineRe extraction).
// Assertion: a non-JSON payload containing a "command" key with "cat <file>" where
// the file contains a JWT → RC=2 and "detected in file" in stderr.
// Counter-arm: commenting out credNonJSONLayerTwoB call → RC=0.
func TestRunCredential_D1_FileMessage_L2bNonJSON(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "token-nonjson.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Invalid JSON: trailing comma after the value makes json.Unmarshal fail.
	// credNonJSONLayerTwoB extracts "cat <tokenFile>" via credCmdLineRe and
	// runs credSecondLayer on it, finding the JWT in the file.
	payload := `{"command":"cat ` + jsonStr(tokenFile) + `",}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("L2b-nonJSON file origin: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "detected in file") {
		t.Errorf("L2b-nonJSON file origin: expected 'detected in file' in stderr, got %q", errOut)
	}
	if !strings.Contains(errOut, "referenced by the command.") {
		t.Errorf("L2b-nonJSON file origin: expected 'referenced by the command.' in stderr, got %q", errOut)
	}
	if strings.Contains(errOut, "detected in tool payload.") {
		t.Errorf("L2b-nonJSON file origin: stderr must not say 'detected in tool payload.', got %q", errOut)
	}
}

// TestRunCredential_D1_FileMessage_L4Deep asserts that a credential found via Layer 4
// deep scan (non-standard schema) produces the "detected in file ... referenced by
// the command." message.
//
// Arrival path: L4-deep (credDeepScan, non-standard JSON schema params.command).
// Assertion: a payload with {"params":{"command":"cat <file>"}} where the file
// contains a JWT → RC=2 and "detected in file" in stderr.
// Counter-arm: commenting out credDeepScan call → RC=0.
func TestRunCredential_D1_FileMessage_L4Deep(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	tokenFile := filepath.Join(dir, "token-deep.txt")
	if err := os.WriteFile(tokenFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	// Non-standard schema: credDeepScan walks all "command" keys recursively.
	// params.command is not a recognized top-level field, so only credDeepScan finds it.
	payload := `{"params":{"command":"cat ` + jsonStr(tokenFile) + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("L4-deep file origin: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "detected in file") {
		t.Errorf("L4-deep file origin: expected 'detected in file' in stderr, got %q", errOut)
	}
	if !strings.Contains(errOut, "referenced by the command.") {
		t.Errorf("L4-deep file origin: expected 'referenced by the command.' in stderr, got %q", errOut)
	}
	if strings.Contains(errOut, "detected in tool payload.") {
		t.Errorf("L4-deep file origin: stderr must not say 'detected in tool payload.', got %q", errOut)
	}
}

// TestRunCredential_D1_FileMessage_L2aADS_Windows asserts that a credential found via
// Layer 2a-ADS (Windows NTFS ADS fallback) produces the "detected in file" message.
//
// Arrival path: L2a-ADS (credWindowsADSBase fallback, Windows only).
// Skipped on non-Windows: NTFS ADS only exists on Windows.
func TestRunCredential_D1_FileMessage_L2aADS_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("L2a-ADS test only runs on Windows (NTFS ADS)")
	}
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// On Windows, the ADS fallback scans "arq.txt" when "arq.txt:stream" stat fails.
	// Set up arq.txt with JWT content; the redirect target in the payload is "arq.txt:stream".
	baseFile := filepath.Join(dir, "arq.txt")
	if err := os.WriteFile(baseFile, []byte(jwtToken), 0644); err != nil {
		t.Fatal(err)
	}
	adsTarget := jsonStr(filepath.Join(dir, "arq.txt:stream"))
	payload := `{"tool_input":{"command":"echo data > ` + adsTarget + `"}}`
	code, _, errOut := runCred(t, dir, payload)
	if code != 2 {
		t.Fatalf("L2a-ADS file origin: expected RC=2, got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "detected in file") {
		t.Errorf("L2a-ADS file origin: expected 'detected in file' in stderr, got %q", errOut)
	}
	if strings.Contains(errOut, "detected in tool payload.") {
		t.Errorf("L2a-ADS file origin: stderr must not say 'detected in tool payload.', got %q", errOut)
	}
}
