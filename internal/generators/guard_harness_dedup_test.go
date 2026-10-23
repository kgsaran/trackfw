package generators

// guard_harness_dedup_test.go — ML-1C (REQ-2026-10-23)
//
// These tests cover the D13 dedup gap closed by ML-1C: harness functions and
// InjectAmazonQHooks lacked the "collapse N legacy forms → exactly 1 canonical"
// pass that ML-1A/ML-1B added to the project-scope Inject* functions.
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//
//   Each *_NLegaciesDedup / *_PSPOSIXForms test asserts: after running the
//   harness function / injector on a fixture seeded with N legacy forms, the
//   output contains exactly 1 canonical entry and 0 legacy entries per
//   (event, matcher) pair.  On origin/main (d450ed10) none of the harness
//   functions had a dedup pass, so they produced N canonicals; these tests
//   would have reported FAIL.  On the current branch (after ML-1C) they PASS.
//
//   TestAmazonQGitBranch_PSPOSIXFormsCollapseToOneCmdExe asserts: POSIX git-branch
//   forms in execute_bash are migrated to the CmdExe canonical.  Origin/main
//   only included GitBranchLegacyCmdExe in the AmazonQ dedup call, leaving
//   POSIX forms untouched — this test would have FAILED there.
//
//   TestWindsurf_ML1C_Harness_NLegaciesDedup asserts: the Windsurf global
//   harness (harnessCredentialGuardTargetWindsurf) deduplicates all
//   CredentialGlobalLegacyPSPOSIX forms.  Before ML-1C it only migrated
//   LegacyD11Rev; the other three global forms would survive — FAIL.
//
//   TestWindsurfCredential_UserHookSurvivesDedup and TestWindsurf_ML1C_D14_* pass on
//   origin/main (user-hook survival and D14 removal were already correct).
//
// Nota de AC4 (bite): every harness fix references guardforms.*LegacyPSPOSIX
// via append(guardforms.X, ...) — any future site that narrows to list[1:]
// instead of the full slice would fail the corresponding *_NLegaciesDedup
// test at the first entry dropped.  The discriminating test is whichever
// legacy form occupies index 0 of the respective slice:
//   CredentialGlobalLegacyPSPOSIX[0] = LegacyD2CredentialGlobalPSPOSIX
//   GitBranchLegacyPSPOSIX[0]        = LegacyBareGitGuard
// If a site used list[1:], those forms would NOT be migrated, and the tests
// below that seed exactly those forms would fail.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// harnessHome sets HOME/USERPROFILE to a new temp dir and returns it.
func harnessHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home
}

// writeJSONFile marshals data to JSON and writes it, creating parent dirs.
func writeJSONFile(t *testing.T, path string, data interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	out, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// readJSONFile reads and unmarshals a JSON file.
func readJSONFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal(%s): %v", path, err)
	}
	return m
}

// buildNestedFixture creates a nested-format JSON for a harness file.
// event → many entries for each cmd in cmds, all with the given matcher.
func buildNestedFixture(events []string, matcher string, cmds []string) map[string]interface{} {
	hooks := make(map[string]interface{})
	for _, event := range events {
		var arr []interface{}
		for _, cmd := range cmds {
			arr = append(arr, map[string]interface{}{
				"matcher": matcher,
				"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": cmd}},
			})
		}
		hooks[event] = arr
	}
	return map[string]interface{}{"hooks": hooks}
}

// buildFlatCommandFixture creates a flat {"command": cmd} array for each event.
func buildFlatCommandFixture(events []string, cmds []string) map[string]interface{} {
	hooks := make(map[string]interface{})
	for _, event := range events {
		var arr []interface{}
		for _, cmd := range cmds {
			arr = append(arr, map[string]interface{}{"command": cmd})
		}
		hooks[event] = arr
	}
	return map[string]interface{}{"version": 1, "hooks": hooks}
}

// buildFlatBashFixture creates a flat {"bash": cmd, "type": "command", ...} array (Copilot format).
func buildFlatBashFixture(events []string, cmds []string) map[string]interface{} {
	hooks := make(map[string]interface{})
	for _, event := range events {
		var arr []interface{}
		for _, cmd := range cmds {
			arr = append(arr, map[string]interface{}{
				"type":       "command",
				"matcher":    "bash",
				"bash":       cmd,
				"cwd":        ".",
				"timeoutSec": float64(10),
			})
		}
		hooks[event] = arr
	}
	return map[string]interface{}{"hooks": hooks}
}

// buildWindsurfFlatFixture creates a flat {"command": cmd, "show_output": true} array.
func buildWindsurfFlatFixture(events []string, cmds []string) map[string]interface{} {
	hooks := make(map[string]interface{})
	for _, event := range events {
		var arr []interface{}
		for _, cmd := range cmds {
			arr = append(arr, map[string]interface{}{
				"command":     cmd,
				"show_output": true,
			})
		}
		hooks[event] = arr
	}
	return map[string]interface{}{"hooks": hooks}
}

// countNestedHarness counts inner commands in event × matcher for a root map.
func countNestedHarness(root map[string]interface{}, event, matcher, cmd string) int {
	hooks, _ := root["hooks"].(map[string]interface{})
	arr, _ := hooks[event].([]interface{})
	n := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["matcher"] != matcher {
			continue
		}
		inner, _ := obj["hooks"].([]interface{})
		for _, h := range inner {
			hObj, _ := h.(map[string]interface{})
			if hObj["command"] == cmd {
				n++
			}
		}
	}
	return n
}

// countFlatCommandEntries counts entries with {"command": cmd, "matcher": matcher} in event.
// matcher="" matches entries without a matcher field.
func countFlatCommandEntries(root map[string]interface{}, event, cmd, matcher string) int {
	hooks, _ := root["hooks"].(map[string]interface{})
	arr, _ := hooks[event].([]interface{})
	n := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["command"] != cmd {
			continue
		}
		m, _ := obj["matcher"].(string)
		if m == matcher {
			n++
		}
	}
	return n
}

// countFlatBashEntries counts entries with {"bash": cmd} in event (Copilot format).
func countFlatBashEntries(root map[string]interface{}, event, cmd string) int {
	hooks, _ := root["hooks"].(map[string]interface{})
	arr, _ := hooks[event].([]interface{})
	n := 0
	for _, item := range arr {
		obj, _ := item.(map[string]interface{})
		if obj["bash"] == cmd {
			n++
		}
	}
	return n
}

// assertNestedDedup asserts: for each event×matcher pair, exactly 1 canonical
// entry and 0 occurrences of any form in legacies.
func assertNestedDedup(t *testing.T, root map[string]interface{},
	events []string, matcher, canonical string, legacies []string) {
	t.Helper()
	for _, event := range events {
		got := countNestedHarness(root, event, matcher, canonical)
		if got != 1 {
			t.Errorf("event=%s matcher=%s: canonical count=%d, want 1", event, matcher, got)
		}
		for _, leg := range legacies {
			if n := countNestedHarness(root, event, matcher, leg); n != 0 {
				t.Errorf("event=%s matcher=%s: legacy %q survived (%d occurrences)", event, matcher, leg, n)
			}
		}
	}
}

// assertFlatCommandDedup asserts flat {"command": ...} dedup.
func assertFlatCommandDedup(t *testing.T, root map[string]interface{},
	events []string, canonical string, legacies []string) {
	t.Helper()
	for _, event := range events {
		got := countFlatCommandEntries(root, event, canonical, "")
		if got != 1 {
			t.Errorf("event=%s: canonical count=%d, want 1", event, got)
		}
		for _, leg := range legacies {
			if n := countFlatCommandEntries(root, event, leg, ""); n != 0 {
				t.Errorf("event=%s: legacy %q survived (%d occurrences)", event, leg, n)
			}
		}
	}
}

// assertFlatBashDedup asserts Copilot flat {"bash": ...} dedup.
func assertFlatBashDedup(t *testing.T, root map[string]interface{},
	events []string, canonical string, legacies []string) {
	t.Helper()
	for _, event := range events {
		got := countFlatBashEntries(root, event, canonical)
		if got != 1 {
			t.Errorf("event=%s: canonical bash count=%d, want 1", event, got)
		}
		for _, leg := range legacies {
			if n := countFlatBashEntries(root, event, leg); n != 0 {
				t.Errorf("event=%s: legacy bash %q survived (%d occurrences)", event, leg, n)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Harness dedup — Codex credential
// ---------------------------------------------------------------------------

func TestHarnessCodexCredential_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: após UpdateHarness com um fixture de 4 formas legadas de credential
	// global, o arquivo ~/.codex/hooks.json contém exatamente 1 entrada canônica
	// por (PreToolUse × Bash, PostToolUse × Bash) e 0 formas legadas.
	// FALHA em origin/main (sem dedup no harness) — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".codex", "hooks.json")
	legacies := guardforms.CredentialGlobalLegacyPSPOSIX
	writeJSONFile(t, path, buildNestedFixture(
		[]string{"PreToolUse", "PostToolUse"}, "Bash", legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"codex-credential-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertNestedDedup(t, root,
		[]string{"PreToolUse", "PostToolUse"}, "Bash",
		guardCredentialGlobalCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Codex git-branch
// ---------------------------------------------------------------------------

func TestHarnessCodexGitBranch_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas legadas de git-branch em ~/.codex/hooks.json → 1 canônica.
	// FALHA em origin/main — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".codex", "hooks.json")
	legacies := []string{
		guardforms.LegacyD2GitBranchPSPOSIX,
		guardforms.LegacyD11GitBranchPSPOSIX,
		guardforms.LegacyD11RevGitBranchPSPOSIX,
		guardforms.LegacyWABashGitBranchPSPOSIX,
	}
	writeJSONFile(t, path, buildNestedFixture(
		[]string{"PreToolUse", "PostToolUse"}, "Bash", legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"codex-git-branch-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertNestedDedup(t, root,
		[]string{"PreToolUse", "PostToolUse"}, "Bash",
		guardGitBranchCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Gemini credential
// ---------------------------------------------------------------------------

func TestHarnessGeminiCredential_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas legadas em ~/.gemini/settings.json → 1 canônica por
	// (BeforeTool × run_shell_command, AfterTool × run_shell_command).
	// FALHA em origin/main — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".gemini", "settings.json")
	legacies := guardforms.CredentialGlobalLegacyPSPOSIX
	writeJSONFile(t, path, buildNestedFixture(
		[]string{"BeforeTool", "AfterTool"}, "run_shell_command", legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"gemini-credential-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertNestedDedup(t, root,
		[]string{"BeforeTool", "AfterTool"}, "run_shell_command",
		guardCredentialGlobalCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Gemini git-branch
// ---------------------------------------------------------------------------

func TestHarnessGeminiGitBranch_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas legadas em ~/.gemini/settings.json → 1 canônica.
	// FALHA em origin/main — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".gemini", "settings.json")
	legacies := []string{
		guardforms.LegacyD2GitBranchPSPOSIX,
		guardforms.LegacyD11GitBranchPSPOSIX,
		guardforms.LegacyD11RevGitBranchPSPOSIX,
		guardforms.LegacyWABashGitBranchPSPOSIX,
	}
	writeJSONFile(t, path, buildNestedFixture(
		[]string{"BeforeTool", "AfterTool"}, "run_shell_command", legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"gemini-git-branch-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertNestedDedup(t, root,
		[]string{"BeforeTool", "AfterTool"}, "run_shell_command",
		guardGitBranchCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Cursor credential
// ---------------------------------------------------------------------------

func TestHarnessCursorCredential_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas legadas em ~/.cursor/hooks.json → 1 canônica por evento.
	// FALHA em origin/main — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".cursor", "hooks.json")
	legacies := guardforms.CredentialGlobalLegacyPSPOSIX
	writeJSONFile(t, path, buildFlatCommandFixture(
		[]string{"beforeShellExecution", "afterShellExecution"}, legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"cursor-credential-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertFlatCommandDedup(t, root,
		[]string{"beforeShellExecution", "afterShellExecution"},
		guardCredentialGlobalCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Cursor git-branch
// ---------------------------------------------------------------------------

func TestHarnessCursorGitBranch_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas legadas em ~/.cursor/hooks.json → 1 canônica.
	// FALHA em origin/main — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".cursor", "hooks.json")
	legacies := []string{
		guardforms.LegacyD2GitBranchPSPOSIX,
		guardforms.LegacyD11GitBranchPSPOSIX,
		guardforms.LegacyD11RevGitBranchPSPOSIX,
		guardforms.LegacyWABashGitBranchPSPOSIX,
	}
	writeJSONFile(t, path, buildFlatCommandFixture(
		[]string{"beforeShellExecution", "afterShellExecution"}, legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"cursor-git-branch-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertFlatCommandDedup(t, root,
		[]string{"beforeShellExecution", "afterShellExecution"},
		guardGitBranchCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Copilot credential
// ---------------------------------------------------------------------------

func TestHarnessCopilotCredential_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas legadas em ~/.copilot/settings.json (campo "bash") → 1
	// entrada canônica por evento (preToolUse, postToolUse).
	// FALHA em origin/main — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".copilot", "settings.json")
	legacies := guardforms.CredentialGlobalLegacyPSPOSIX
	writeJSONFile(t, path, buildFlatBashFixture(
		[]string{"preToolUse", "postToolUse"}, legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"copilot-credential-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertFlatBashDedup(t, root,
		[]string{"preToolUse", "postToolUse"},
		guardCredentialGlobalCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Copilot git-branch
// ---------------------------------------------------------------------------

func TestHarnessCopilotGitBranch_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas legadas em ~/.copilot/settings.json (campo "bash") → 1
	// canônica por evento.
	// FALHA em origin/main — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".copilot", "settings.json")
	legacies := []string{
		guardforms.LegacyD2GitBranchPSPOSIX,
		guardforms.LegacyD11GitBranchPSPOSIX,
		guardforms.LegacyD11RevGitBranchPSPOSIX,
		guardforms.LegacyWABashGitBranchPSPOSIX,
	}
	writeJSONFile(t, path, buildFlatBashFixture(
		[]string{"preToolUse", "postToolUse"}, legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"copilot-git-branch-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	assertFlatBashDedup(t, root,
		[]string{"preToolUse", "postToolUse"},
		guardGitBranchCmdPSPOSIX, legacies)
}

// ---------------------------------------------------------------------------
// Harness dedup — Windsurf global credential
// ---------------------------------------------------------------------------

func TestHarnessWindsurfCredential_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: 4 formas globais legadas em ~/.codeium/windsurf/hooks.json →
	// exatamente 1 canônica por evento (pre_run_command, pre_write_code).
	// Antes de ML-1C apenas LegacyD11Rev era migrado; os outros 3 sobreviviam.
	// FALHA em origin/main e no branch pré-ML-1C — PASSA após ML-1C.
	home := harnessHome(t)
	path := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	legacies := guardforms.CredentialGlobalLegacyPSPOSIX
	writeJSONFile(t, path, buildWindsurfFlatFixture(
		[]string{"pre_run_command", "pre_write_code"}, legacies))

	if _, err := UpdateHarness(UpdateOptions{
		Targets: []string{"windsurf-credential-guard"},
	}); err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}

	root := readJSONFile(t, path)
	// Windsurf flat format uses "command" key, no "matcher" field.
	for _, event := range []string{"pre_run_command", "pre_write_code"} {
		got := countFlatCommandEntries(root, event, guardCredentialGlobalCmdPSPOSIX, "")
		if got != 1 {
			t.Errorf("event=%s: canonical count=%d, want 1", event, got)
		}
		for _, leg := range legacies {
			if n := countFlatCommandEntries(root, event, leg, ""); n != 0 {
				t.Errorf("event=%s: legacy %q survived (%d occurrences)", event, leg, n)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Amazon Q — POSIX git-branch forms in execute_bash
// ---------------------------------------------------------------------------

func TestAmazonQGitBranch_PSPOSIXFormsCollapseToOneCmdExe(t *testing.T) {
	// Afirma: formas legadas POSIX de git-branch em execute_bash do Amazon Q
	// são migradas para a forma canônica CmdExe (guardGitBranchCmdCmdExe), e
	// o resultado final contém exatamente 1 entrada canônica.
	// Origin/main só incluía GitBranchLegacyCmdExe no dedup; formas POSIX
	// sobreviviam — FALHA em origin/main, PASSA após ML-1C.
	root := t.TempDir()
	home := harnessHome(t)
	_ = home // HOME is set; AmazonQ doesn't need global guard check.

	agentPath := filepath.Join(root, ".amazonq", "cli-agents", "q_cli_default.json")
	// Seed with all POSIX git-branch legacy forms in execute_bash.
	posixLegacies := guardforms.GitBranchLegacyPSPOSIX
	var entries []interface{}
	for _, leg := range posixLegacies {
		entries = append(entries, map[string]interface{}{
			"matcher": "execute_bash",
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": leg}},
		})
	}
	fixture := map[string]interface{}{
		"name":  "q_cli_default",
		"tools": []interface{}{"*"},
		"hooks": map[string]interface{}{
			"preToolUse": entries,
		},
	}
	writeJSONFile(t, agentPath, fixture)

	if err := InjectAmazonQHooks(root); err != nil {
		t.Fatalf("InjectAmazonQHooks: %v", err)
	}

	got := readJSONFile(t, agentPath)
	// Exactly 1 CmdExe canonical in execute_bash.
	canonCount := countNestedHarness(got, "preToolUse", "execute_bash", guardGitBranchCmdCmdExe)
	if canonCount != 1 {
		t.Errorf("execute_bash: CmdExe canonical count=%d, want 1", canonCount)
	}
	// All POSIX legacy forms must be absent.
	for _, leg := range posixLegacies {
		if n := countNestedHarness(got, "preToolUse", "execute_bash", leg); n != 0 {
			t.Errorf("execute_bash: POSIX legacy %q survived (%d occurrences)", leg, n)
		}
	}
}

// ---------------------------------------------------------------------------
// Windsurf (project-scope) — N credential legacies
// ---------------------------------------------------------------------------

func TestWindsurfCredential_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: N formas legadas de credential em .windsurf/hooks.json colapsam
	// para exatamente 1 entrada canônica em pre_run_command e pre_write_code.
	// Falha em origin/main (sem dedupFlatGuardEntries em InjectWindsurfHooks).
	root := t.TempDir()
	home := harnessHome(t)
	_ = home

	hooksPath := filepath.Join(root, ".windsurf", "hooks.json")
	legacies := guardforms.CredentialLegacyPSPOSIX
	writeJSONFile(t, hooksPath, buildWindsurfFlatFixture(
		[]string{"pre_run_command", "pre_write_code"}, legacies))

	if err := InjectWindsurfHooks(root); err != nil {
		t.Fatalf("InjectWindsurfHooks: %v", err)
	}

	result := readJSONFile(t, hooksPath)
	for _, event := range []string{"pre_run_command", "pre_write_code"} {
		got := countFlatCommandEntries(result, event, guardCredentialCmdPSPOSIX, "")
		if got != 1 {
			t.Errorf("event=%s: canonical count=%d, want 1", event, got)
		}
		for _, leg := range legacies {
			if n := countFlatCommandEntries(result, event, leg, ""); n != 0 {
				t.Errorf("event=%s: legacy %q survived (%d occurrences)", event, leg, n)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Windsurf (project-scope) — N git-branch legacies
// ---------------------------------------------------------------------------

func TestWindsurfGitBranch_NLegaciesCollapseToOne(t *testing.T) {
	// Afirma: N formas legadas de git-branch em pre_run_command colapsam para 1.
	// Falha em origin/main (sem dedupFlatGuardEntries em InjectWindsurfHooks).
	root := t.TempDir()
	home := harnessHome(t)
	_ = home

	hooksPath := filepath.Join(root, ".windsurf", "hooks.json")
	legacies := guardforms.GitBranchLegacyPSPOSIX
	writeJSONFile(t, hooksPath, buildWindsurfFlatFixture(
		[]string{"pre_run_command"}, legacies))

	if err := InjectWindsurfHooks(root); err != nil {
		t.Fatalf("InjectWindsurfHooks: %v", err)
	}

	result := readJSONFile(t, hooksPath)
	got := countFlatCommandEntries(result, "pre_run_command", windsurfGitGuardCmd, "")
	if got != 1 {
		t.Errorf("pre_run_command: git-branch canonical count=%d, want 1", got)
	}
	for _, leg := range legacies {
		if n := countFlatCommandEntries(result, "pre_run_command", leg, ""); n != 0 {
			t.Errorf("pre_run_command: git-branch legacy %q survived (%d occurrences)", leg, n)
		}
	}
}

// ---------------------------------------------------------------------------
// Windsurf AC5 — user hook survives
// ---------------------------------------------------------------------------

func TestWindsurfCredential_UserHookSurvivesDedup(t *testing.T) {
	// Afirma: um hook de usuário cujo comando NÃO está na registry sobrevive
	// byte-for-byte após InjectWindsurfHooks. Passes on origin/main too.
	root := t.TempDir()
	home := harnessHome(t)
	_ = home

	userHook := "trackfw guard credential --dry-run && echo ok"
	hooksPath := filepath.Join(root, ".windsurf", "hooks.json")
	writeJSONFile(t, hooksPath, map[string]interface{}{
		"hooks": map[string]interface{}{
			"pre_run_command": []interface{}{
				map[string]interface{}{"command": userHook, "show_output": true},
				// Also seed a legacy form to verify migration doesn't clobber user hook.
				map[string]interface{}{"command": guardforms.LegacyD11RevCredentialPSPOSIX, "show_output": true},
			},
		},
	})

	if err := InjectWindsurfHooks(root); err != nil {
		t.Fatalf("InjectWindsurfHooks: %v", err)
	}

	result := readJSONFile(t, hooksPath)
	// User hook must survive.
	if n := countFlatCommandEntries(result, "pre_run_command", userHook, ""); n != 1 {
		t.Errorf("user hook count in pre_run_command = %d, want 1", n)
	}
	// Canonical must be present.
	if n := countFlatCommandEntries(result, "pre_run_command", guardCredentialCmdPSPOSIX, ""); n != 1 {
		t.Errorf("canonical count in pre_run_command = %d, want 1", n)
	}
	// Legacy must be absent.
	if n := countFlatCommandEntries(result, "pre_run_command", guardforms.LegacyD11RevCredentialPSPOSIX, ""); n != 0 {
		t.Errorf("D11Rev legacy survived in pre_run_command: %d occurrences", n)
	}
}

// ---------------------------------------------------------------------------
// Windsurf D14 — global credential installed → project credential removed
// ---------------------------------------------------------------------------

func TestWindsurfCredential_D14GlobalInstalledRemovesProject(t *testing.T) {
	// Afirma: quando o guard global de credential está instalado em
	// ~/.codeium/windsurf/hooks.json (D14 revised), InjectWindsurfHooks remove
	// todas as entradas de credential do arquivo de projeto em pre_run_command
	// e pre_write_code, mas mantém o git-branch guard em pre_run_command.
	root := t.TempDir()
	home := harnessHome(t)

	// Install global credential guard (both events required by globalCredentialGuardInstalledWindsurf).
	globalPath := filepath.Join(home, ".codeium", "windsurf", "hooks.json")
	writeJSONFile(t, globalPath, buildWindsurfFlatFixture(
		[]string{"pre_run_command", "pre_write_code"},
		[]string{guardCredentialGlobalCmdPSPOSIX}))

	// Seed project file with multiple credential legacy forms + 1 git-branch legacy.
	hooksPath := filepath.Join(root, ".windsurf", "hooks.json")
	writeJSONFile(t, hooksPath, map[string]interface{}{
		"hooks": map[string]interface{}{
			"pre_run_command": []interface{}{
				map[string]interface{}{"command": guardforms.LegacyD2CredentialPSPOSIX, "show_output": true},
				map[string]interface{}{"command": guardforms.LegacyD11RevCredentialPSPOSIX, "show_output": true},
				map[string]interface{}{"command": guardforms.LegacyD2GitBranchPSPOSIX, "show_output": true},
			},
			"pre_write_code": []interface{}{
				map[string]interface{}{"command": guardforms.LegacyWABashCredentialPSPOSIX, "show_output": true},
			},
		},
	})

	if err := InjectWindsurfHooks(root); err != nil {
		t.Fatalf("InjectWindsurfHooks: %v", err)
	}

	result := readJSONFile(t, hooksPath)

	// pre_run_command: git-branch canonical present, credential absent.
	if n := countFlatCommandEntries(result, "pre_run_command", windsurfGitGuardCmd, ""); n != 1 {
		t.Errorf("pre_run_command git-branch canonical count=%d, want 1", n)
	}
	for _, leg := range guardforms.CredentialLegacyPSPOSIX {
		if n := countFlatCommandEntries(result, "pre_run_command", leg, ""); n != 0 {
			t.Errorf("pre_run_command: credential legacy %q should be absent, got %d", leg, n)
		}
	}
	if n := countFlatCommandEntries(result, "pre_run_command", guardCredentialCmdPSPOSIX, ""); n != 0 {
		t.Errorf("pre_run_command: credential canonical should be absent (D14), got %d entries", n)
	}

	// pre_write_code: no credential entries at all.
	for _, leg := range guardforms.CredentialLegacyPSPOSIX {
		if n := countFlatCommandEntries(result, "pre_write_code", leg, ""); n != 0 {
			t.Errorf("pre_write_code: credential legacy %q should be absent, got %d", leg, n)
		}
	}
	if n := countFlatCommandEntries(result, "pre_write_code", guardCredentialCmdPSPOSIX, ""); n != 0 {
		t.Errorf("pre_write_code: credential canonical should be absent (D14), got %d entries", n)
	}
}
