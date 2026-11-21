package commands

// Tests for D1 (ADR-2026-11-21): `agents|skills install --scope global` must
// not write rules or hooks under manager.ProjectRoot.
//
// Each test name below is followed by a one-sentence rule-reconciliation
// statement as required by the Regra Dura de Reconciliação (CLAUDE.md):
//
//   TestGlobalInstallDoesNotWriteClaudeRulesOrHooks
//     Affirms D1: --scope global leaves a stale CLAUDE.md byte-identical and
//     does not create .claude/settings.json in the project dir.
//
//   TestGlobalInstallDefaultNonTTYDoesNotWriteRulesOrHooks
//     Affirms D1+T2: without --scope and without TTY, the resolved scope is
//     global (default) and the project dir is not modified.
//
//   TestSkillsInstallGlobalDoesNotWriteClaudeRulesOrHooks
//     Affirms D1+T4: the guard is based on opts.scope, not on KindAgents;
//     skills install --scope global leaves the project dir untouched.
//
//   TestProjectInstallWritesClaudeRulesAndHooksAndNoHint
//     Affirms D3+T8: --scope project writes CLAUDE.md and .claude/settings.json
//     and does NOT print the D2 hint to stderr.
//
//   TestGlobalInstallJsonStdoutIsValid
//     Affirms T6: with --scope global --json stdout is valid JSON (D2 hint is
//     on stderr, not stdout, so the JSON channel is uncontaminated).
//
//   TestGlobalInstallWindsurfDoesNotWriteWindsurfFiles
//     Affirms T5: the entire block 278-298 is gated; with windsurf+global,
//     neither .windsurfrules (via InjectRulesForTool loop) nor
//     .windsurf/hooks.json (via InjectWindsurfHooks) is created.
//
//   TestGlobalInstallHintDoesNotCiteCwd
//     Affirms T7: the D2 hint emitted to stderr does not contain the project
//     dir path (ADR D2: "the message does not cite the cwd").
//
//   TestTTYScopeGlobalDoesNotWriteRulesOrHooks
//     Affirms T3: when the TTY prompt returns "global" (bare-Enter pre-select),
//     the project dir is not modified and the D2 hint appears in stderr.
//
//   TestTTYScopeProjectWritesRulesAndHooks
//     Affirms T1: when the TTY prompt returns "project", opts.scope is set by
//     resolveScope before the inject block runs, so rules and hooks are written.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staleContent is a minimal file body with no <!-- trackfw:rules:start -->
// marker.  injectOrUpdateRules creates or appends to such a file — it does NOT
// leave it byte-identical.  Using this sentinel makes the hash comparison a
// real oracle (§1.11 of threat model): if InjectRulesForTool runs, the hash
// changes; if it is correctly skipped, the hash stays the same.
const staleContent = "# My Project\n\nThis file has no trackfw rules block.\n"

// fileSHA256 returns the hex-encoded SHA-256 of the file at path, or "" if the
// file does not exist.  Any other error causes t.Fatal.
func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("fileSHA256: read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

// assertFileAbsent fails if path exists in the filesystem.
func assertFileAbsent(t *testing.T, label, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("%s: expected file to be absent, but it exists: %s", label, path)
	} else if !os.IsNotExist(err) {
		t.Errorf("%s: unexpected stat error for %s: %v", label, path, err)
	}
}

// assertHashUnchanged fails if the current hash of path differs from before.
func assertHashUnchanged(t *testing.T, label, path, before string) {
	t.Helper()
	after := fileSHA256(t, path)
	if before != after {
		t.Errorf("%s: hash of %s changed (before=%s after=%s)", label, path, before, after)
	}
}

// ----- TestGlobalInstallDoesNotWriteClaudeRulesOrHooks ---------------------

func TestGlobalInstallDoesNotWriteClaudeRulesOrHooks(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")

	// Pre-create a stale CLAUDE.md so injectOrUpdateRules would actually
	// modify it if the guard were absent (vacuity arm §1.11).
	claudeMD := filepath.Join(project, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := fileSHA256(t, claudeMD)
	settingsJSON := filepath.Join(project, ".claude", "settings.json")

	var stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "global"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	assertHashUnchanged(t, "CLAUDE.md must be byte-identical after global install", claudeMD, hashBefore)
	assertFileAbsent(t, ".claude/settings.json must not be created by global install", settingsJSON)
	if !strings.Contains(stderr.String(), "trackfw agents install --scope project") {
		t.Errorf("D2 hint missing from stderr; got: %q", stderr.String())
	}
}

// ----- TestGlobalInstallDefaultNonTTYDoesNotWriteRulesOrHooks --------------

func TestGlobalInstallDefaultNonTTYDoesNotWriteRulesOrHooks(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")

	claudeMD := filepath.Join(project, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := fileSHA256(t, claudeMD)
	settingsJSON := filepath.Join(project, ".claude", "settings.json")

	// No --scope flag, non-TTY → resolveScope resolves to "global".
	var stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	assertHashUnchanged(t, "CLAUDE.md must be byte-identical (no-scope non-TTY → global)", claudeMD, hashBefore)
	assertFileAbsent(t, ".claude/settings.json must not be created (no-scope non-TTY → global)", settingsJSON)
	if !strings.Contains(stderr.String(), "trackfw agents install --scope project") {
		t.Errorf("D2 hint missing from stderr; got: %q", stderr.String())
	}
}

// ----- TestSkillsInstallGlobalDoesNotWriteClaudeRulesOrHooks ---------------

func TestSkillsInstallGlobalDoesNotWriteClaudeRulesOrHooks(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")

	claudeMD := filepath.Join(project, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := fileSHA256(t, claudeMD)
	settingsJSON := filepath.Join(project, ".claude", "settings.json")

	var stderr bytes.Buffer
	cmd := newSkillsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--scope", "global"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	assertHashUnchanged(t, "CLAUDE.md must be byte-identical after skills global install", claudeMD, hashBefore)
	assertFileAbsent(t, ".claude/settings.json must not be created by skills global install", settingsJSON)
	if !strings.Contains(stderr.String(), "trackfw skills install --scope project") {
		t.Errorf("D2 hint (skills) missing from stderr; got: %q", stderr.String())
	}
}

// ----- TestProjectInstallWritesClaudeRulesAndHooksAndNoHint ----------------

func TestProjectInstallWritesClaudeRulesAndHooksAndNoHint(t *testing.T) {
	project, home := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	writeGreekIdentity(t, home)

	claudeMD := filepath.Join(project, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := fileSHA256(t, claudeMD)
	settingsJSON := filepath.Join(project, ".claude", "settings.json")

	var stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// D3: --scope project still writes rules and hooks.
	afterHash := fileSHA256(t, claudeMD)
	if hashBefore == afterHash {
		t.Errorf("CLAUDE.md hash must change after --scope project install; stale fixture was not injected")
	}
	if _, err := os.Stat(settingsJSON); os.IsNotExist(err) {
		t.Errorf(".claude/settings.json must be created by --scope project install")
	}
	// T8: D2 hint must NOT appear for project scope.
	if strings.Contains(stderr.String(), "not modified") {
		t.Errorf("D2 hint must not appear for --scope project; stderr: %q", stderr.String())
	}
}

// ----- TestGlobalInstallJsonStdoutIsValid -----------------------------------

func TestGlobalInstallJsonStdoutIsValid(t *testing.T) {
	_, _ = integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")

	var stdout, stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "global", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// T6: stdout must be valid JSON — the D2 hint must not appear there.
	var v lifecycleOutput
	if err := json.Unmarshal(stdout.Bytes(), &v); err != nil {
		t.Fatalf("stdout is not valid JSON after global --json install: %v\n%s", err, stdout.String())
	}
	// D2 hint goes to stderr.
	if !strings.Contains(stderr.String(), "not modified") {
		t.Errorf("D2 hint missing from stderr with --json; stderr: %q", stderr.String())
	}
}

// ----- TestGlobalInstallWindsurfDoesNotWriteWindsurfFiles ------------------

func TestGlobalInstallWindsurfDoesNotWriteWindsurfFiles(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")

	windsurfRules := filepath.Join(project, ".windsurfrules")
	windsurfHooks := filepath.Join(project, ".windsurf", "hooks.json")

	var stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "windsurf", "--scope", "global"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// T5: the entire inject block is gated; InjectWindsurfHooks (line 1613)
	// is also blocked because InjectHooksForTools is inside the same gate.
	assertFileAbsent(t, ".windsurfrules must not be created by windsurf global install", windsurfRules)
	assertFileAbsent(t, ".windsurf/hooks.json must not be created by windsurf global install", windsurfHooks)

	// Counter-arm: --scope project writes both files.
	project2, home2 := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	writeGreekIdentity(t, home2)
	_ = project2
	cmd2 := newAgentsCmd()
	cmd2.SetOut(&bytes.Buffer{})
	cmd2.SetErr(io.Discard)
	cmd2.SetArgs([]string{"install", "--targets", "windsurf", "--scope", "project"})
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}
	windsurfRules2 := filepath.Join(project2, ".windsurfrules")
	windsurfHooks2 := filepath.Join(project2, ".windsurf", "hooks.json")
	if _, err := os.Stat(windsurfRules2); os.IsNotExist(err) {
		t.Errorf(".windsurfrules must be created by --scope project windsurf install")
	}
	if _, err := os.Stat(windsurfHooks2); os.IsNotExist(err) {
		t.Errorf(".windsurf/hooks.json must be created by --scope project windsurf install")
	}
}

// ----- TestGlobalInstallHintDoesNotCiteCwd ---------------------------------

func TestGlobalInstallHintDoesNotCiteCwd(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")

	var stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend", "--scope", "global"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// T7: the hint line must not name the cwd (ADR D2).
	// Find the hint line in stderr.
	var hintLine string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.Contains(line, "not modified") {
			hintLine = line
			break
		}
	}
	if hintLine == "" {
		t.Fatalf("D2 hint line not found in stderr: %q", stderr.String())
	}
	if strings.Contains(hintLine, project) {
		t.Errorf("D2 hint must not contain the project path; hint: %q", hintLine)
	}
	// Also assert no absolute path starting with "/" is in the hint line.
	if strings.Contains(hintLine, "/") && strings.Contains(hintLine, project[:min(len(project), 5)]) {
		t.Errorf("D2 hint appears to contain a filesystem path; hint: %q", hintLine)
	}
}

// min is a helper for older Go versions without built-in min for int.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ----- TestTTYScopeGlobalDoesNotWriteRulesOrHooks --------------------------

func TestTTYScopeGlobalDoesNotWriteRulesOrHooks(t *testing.T) {
	project, home := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	writeGreekIdentity(t, home)

	// Simulate TTY + user pressing Enter (pre-selected "global").
	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldPrompt := promptInstallScopeRunner
	promptInstallScopeRunner = func() (string, error) { return "global", nil }
	t.Cleanup(func() { promptInstallScopeRunner = oldPrompt })

	claudeMD := filepath.Join(project, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := fileSHA256(t, claudeMD)
	settingsJSON := filepath.Join(project, ".claude", "settings.json")

	var stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// T3: prompt-returned "global" must be honored by the inject gate.
	assertHashUnchanged(t, "CLAUDE.md must be byte-identical (TTY prompt → global)", claudeMD, hashBefore)
	assertFileAbsent(t, ".claude/settings.json must not be created (TTY prompt → global)", settingsJSON)
	if !strings.Contains(stderr.String(), "not modified") {
		t.Errorf("D2 hint missing from stderr (TTY → global); stderr: %q", stderr.String())
	}
}

// ----- TestTTYScopeProjectWritesRulesAndHooks ------------------------------

func TestTTYScopeProjectWritesRulesAndHooks(t *testing.T) {
	project, home := integrationCommandFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	writeGreekIdentity(t, home)

	// Simulate TTY + user choosing "project".
	oldTTY := integrationsStdinIsTTY
	integrationsStdinIsTTY = func() bool { return true }
	t.Cleanup(func() { integrationsStdinIsTTY = oldTTY })

	oldPrompt := promptInstallScopeRunner
	promptInstallScopeRunner = func() (string, error) { return "project", nil }
	t.Cleanup(func() { promptInstallScopeRunner = oldPrompt })

	claudeMD := filepath.Join(project, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := fileSHA256(t, claudeMD)
	settingsJSON := filepath.Join(project, ".claude", "settings.json")

	var stderr bytes.Buffer
	cmd := newAgentsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"install", "--targets", "claude", "--items", "backend"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// T1: guard placed after resolveScope must not block prompt-chosen "project".
	afterHash := fileSHA256(t, claudeMD)
	if hashBefore == afterHash {
		t.Errorf("CLAUDE.md hash must change after TTY prompt → project install; stale fixture was not injected")
	}
	if _, err := os.Stat(settingsJSON); os.IsNotExist(err) {
		t.Errorf(".claude/settings.json must be created when TTY prompt returns project")
	}
	// D2 hint must NOT appear for project scope.
	if strings.Contains(stderr.String(), "not modified") {
		t.Errorf("D2 hint must not appear when prompt returns project; stderr: %q", stderr.String())
	}
}
