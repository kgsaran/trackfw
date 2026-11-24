// Package guard — tests for credential guard with nested trackfw.yaml chain (D4, ADR-2026-11-24).
//
// These tests cover the new monotonic-mode walk (credWalkStrictestMode) and
// ancestor-based project detection (credFindNearestProjectRoot) in both
// RunCredential (project scope) and RunCredentialGlobal (global scope).
//
// All tests reuse the helpers defined in credential_test.go (jwtToken,
// makeCredProjectDir, runCred, runCredGlobal, jsonStr) — they are in the
// same package so no import or redefinition is needed.
//
// No t.Parallel(): chdir changes the entire process working directory.
package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// credNestedWriteYAML writes content to <dir>/trackfw.yaml.
// Used to plant a yaml in a subdirectory WITHOUT going through the guard
// (the test framework has no agent_id; the hook doesn't intercept os.WriteFile).
func credNestedWriteYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("credNestedWriteYAML: %v", err)
	}
}

// credNestedJWTPayload returns a JWT-carrying tool_input payload using jwtToken.
func credNestedJWTPayload() string {
	return `{"tool_input":{"command":"echo ` + jwtToken + `"}}`
}

// TestCredNested_GlobalPlantedWarnBlocks asserts that in global scope, a trackfw.yaml
// with mode:warn in the cwd does NOT downgrade the block default; the strictest mode
// (block) wins → rc=2.
// Sentence: RunCredentialGlobal hardcodes block without calling credWalkStrictestMode;
// a mode:warn yaml in the ancestor chain cannot lower the global default because the
// yaml is never read in this scope.
// Before this fix: mode from yaml overrode the caller default → rc=0.
func TestCredNested_GlobalPlantedWarnBlocks(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: warn\n")
	code, _, errOut := runCredGlobal(t, dir, credNestedJWTPayload())
	if code != 2 {
		t.Fatalf("global+warn yaml: expected rc=2 (block default not lowered by warn), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "blocked") {
		t.Errorf("expected block message on stderr, got %q", errOut)
	}
}

// TestCredNested_GlobalNoYamlBlocks asserts that in global scope, without any
// trackfw.yaml in the cwd or above it, the default block applies → rc=2.
// Sentence: RunCredentialGlobal hardcodes block without reading any yaml; absence of
// yaml in the ancestor chain is irrelevant — the block is unconditional.
// Unchanged by this fix — acts as a regression guard.
func TestCredNested_GlobalNoYamlBlocks(t *testing.T) {
	dir := t.TempDir() // no trackfw.yaml
	code, _, errOut := runCredGlobal(t, dir, credNestedJWTPayload())
	if code != 2 {
		t.Fatalf("global+no yaml: expected rc=2 (default block), got %d (stderr=%q)", code, errOut)
	}
}

// TestCredNested_ProjectSubdirNoYamlBlocks asserts that project scope detects the
// project via the nearest ancestor's trackfw.yaml; a cwd in a subdirectory that lacks
// its own yaml no longer disables the guard. Root yaml has mode:block → rc=2.
// Sentence: credFindNearestProjectRoot walks up from sub to root and finds the yaml;
// credWalkStrictestMode then reads block from the ancestor → rc=2.
// Before this fix: Stat(cwd/trackfw.yaml) failed → guard off → rc=0.
func TestCredNested_ProjectSubdirNoYamlBlocks(t *testing.T) {
	root := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runCred(t, sub, credNestedJWTPayload())
	if code != 2 {
		t.Fatalf("project+subdir+root-block: expected rc=2 (guard active from ancestor yaml), got %d (stderr=%q)", code, errOut)
	}
}

// TestCredNested_ProjectPlantedWarnUnderBlockRootBlocks asserts that the strictest
// mode from the full chain wins: root has mode:block; subdirectory has planted mode:warn;
// block prevails over warn → rc=2.
// Sentence: credWalkStrictestMode finds mode:warn at sub, then mode:block at root;
// block is strictly greater and wins; rc=2 regardless of the sub yaml's content.
// Before this fix: subdirectory yaml's warn overrode root's block → rc=0.
func TestCredNested_ProjectPlantedWarnUnderBlockRootBlocks(t *testing.T) {
	root := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	credNestedWriteYAML(t, sub, "credential_guard:\n  mode: warn\n")
	code, _, errOut := runCred(t, sub, credNestedJWTPayload())
	if code != 2 {
		t.Fatalf("project+sub-warn+root-block: expected rc=2 (block wins over warn), got %d (stderr=%q)", code, errOut)
	}
}

// TestCredNested_ProjectWarnOnlyCounterArm asserts that in project scope with only
// mode:warn in the root yaml and no stricter yaml above it, the guard warns and exits 0.
// Sentence: credWalkStrictestMode with caller-default=warn and all yamls reading warn
// returns warn; the project-scope default is preserved and no "block" floor was introduced
// inside the shared helper.
// This is the counter-arm: a plain project-scope warn setup must not change behavior.
func TestCredNested_ProjectWarnOnlyCounterArm(t *testing.T) {
	dir := makeCredProjectDir(t, "credential_guard:\n  mode: warn\nroadmap_dir: docs/roadmaps\n")
	rdDir := filepath.Join(dir, "docs", "roadmaps")
	if err := os.MkdirAll(rdDir, 0755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runCred(t, dir, credNestedJWTPayload())
	if code != 0 {
		t.Fatalf("project+warn-only: expected rc=0 (warn default preserved, no block floor leaked), got %d (stderr=%q)", code, errOut)
	}
	if !strings.Contains(errOut, "warning") {
		t.Errorf("expected warning message on stderr, got %q", errOut)
	}
}

// TestCredNested_ProjectNoYamlAnywhere asserts that project scope exits 0 (guard off)
// when no trackfw.yaml exists in any ancestor of the cwd.
// Sentence: credFindNearestProjectRoot finds no yaml up to the filesystem root; the
// guard returns 0 early because the directory is not a trackfw project.
// Unchanged behavior — acts as a regression guard.
func TestCredNested_ProjectNoYamlAnywhere(t *testing.T) {
	dir := t.TempDir()
	code, _, _ := runCred(t, dir, credNestedJWTPayload())
	if code != 0 {
		t.Fatalf("project+no-yaml: expected rc=0 (not a trackfw project), got %d", code)
	}
}

// TestCredNested_DirectoryYamlCountsAsBlock asserts that a directory named
// trackfw.yaml in the cwd triggers ErrNotRegularFile and raises the effective mode
// to block.
// Setup: root has a regular trackfw.yaml with mode:warn; sub has a DIRECTORY named
// trackfw.yaml; cwd=sub. ErrNotRegularFile in the walk is treated as block (D3) → rc=2.
// Sentence: credWalkStrictestMode encounters ErrNotRegularFile for the directory,
// treats it as the strongest denial signal and returns block immediately; rc=2.
// Before this fix: os.Stat succeeds on a directory → project detected via sub;
// os.ReadFile(dir) fails → returns default warn → rc=0.
func TestCredNested_DirectoryYamlCountsAsBlock(t *testing.T) {
	root := makeCredProjectDir(t, "credential_guard:\n  mode: warn\nroadmap_dir: docs/roadmaps\n")
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a directory named trackfw.yaml inside sub.
	dirAsYAML := filepath.Join(sub, "trackfw.yaml")
	if err := os.Mkdir(dirAsYAML, 0755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runCred(t, sub, credNestedJWTPayload())
	if code != 2 {
		t.Fatalf("dir-as-yaml: expected rc=2 (ErrNotRegularFile counts as block), got %d (stderr=%q)", code, errOut)
	}
}

// TestCredNested_DirectoryYamlDenies asserts that when the only trackfw.yaml in the
// ancestor chain is a directory (not a regular file), RunCredential returns rc=2.
// Asserts: D3 (MÉDIO-1 fix) — credFindNearestProjectRoot must not treat ErrNotRegularFile
// as absent and walk past; a directory-named yaml marks the root and RunCredential must
// deny (rc=2) so that a planted directory cannot silence the guard.
// Before the fix: credFindNearestProjectRoot walked past ErrNotRegularFile → "" →
// RunCredential returned 0 silently.
func TestCredNested_DirectoryYamlDenies(t *testing.T) {
	dir := t.TempDir()
	// Plant a DIRECTORY named trackfw.yaml — not a regular file.
	yamlDir := filepath.Join(dir, "trackfw.yaml")
	if err := os.Mkdir(yamlDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Benign payload (no credentials): the deny comes from the non-regular yaml, not from
	// credential pattern matching. Using empty payload so the hook can't match any pattern.
	code, _, errOut := runCred(t, dir, "")
	if code != 2 {
		t.Fatalf("directory yaml: expected rc=2 (non-regular yaml → deny), got %d (stderr=%q)", code, errOut)
	}
	if errOut == "" {
		t.Error("expected stderr message, got empty")
	}
}

// TestCredNested_NoYamlAllows asserts that when there is no trackfw.yaml anywhere in
// the ancestor chain, RunCredential returns rc=0 (not inside a project).
// Asserts: D3 — only ErrNotRegularFile denies; a truly absent yaml (IsNotExist) allows
// the walk to continue; reaching the FS root with no yaml found means no project → rc=0.
// This is the counter-arm for TestCredNested_DirectoryYamlDenies.
func TestCredNested_NoYamlAllows(t *testing.T) {
	dir := t.TempDir() // no trackfw.yaml anywhere
	code, _, _ := runCred(t, dir, "")
	if code != 0 {
		t.Fatalf("no yaml: expected rc=0 (not in a project), got %d", code)
	}
}

// TestCredNested_AntigravityPayloadCwdUsed asserts that for Antigravity payloads, the
// guard uses the payload cwd (not the process cwd) for project detection, per
// ADR-2026-11-18 D3.
// Sentence: when the payload is Antigravity and payloadCwd points to a project with
// mode:block, credFindNearestProjectRoot finds the project via payloadCwd; if
// payloadCwd were ignored, processCwdDir has no project and rc would be 0.
// Process cwd = unrelated tempdir with no yaml in its ancestor chain.
// Payload cwd = project dir with mode:block → rc=2.
func TestCredNested_AntigravityPayloadCwdUsed(t *testing.T) {
	projDir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")
	// Process cwd is a separate tempdir with no yaml above it.
	processCwdDir := t.TempDir()

	// Antigravity schema: hookEventName + toolCall as JSON object + root-level cwd.
	// JWT is in toolCall.args.CommandLine so layer1Str (= raw toolCall bytes) detects it.
	payload := `{"hookEventName":"PreToolUse","toolCall":{"name":"Bash","args":{"CommandLine":"echo ` +
		jwtToken + `"}},"cwd":"` + jsonStr(projDir) + `"}`

	code, _, errOut := runCred(t, processCwdDir, payload)
	if code != 2 {
		t.Fatalf("antigravity+payload-cwd: expected rc=2 (payload cwd drives project detection), got %d (stderr=%q)", code, errOut)
	}
}
