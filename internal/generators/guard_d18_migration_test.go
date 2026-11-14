package generators

// guard_d18_migration_test.go — ML-1A2 (REQ-2026-11-13)
//
// Exercises the D18 migration path: each CLI that emits the "emulate -R sh" form
// without the "set +eu" prefix (pre-REQ-2026-11-13) must have that legacy form
// upgraded to the current D18 form (with "set +eu") by the update/inject call.
//
// Tests covered:
//   1. Claude  — credential guard, Read matcher (un-backstopped path, no dedup)
//   2. Claude  — git-branch guard, Bash|PowerShell matcher
//   3. Claude  — write-scope guard, Write|Edit|NotebookEdit matcher
//   4. Codex   — credential guard, apply_patch matcher (explicit D18 call)
//   5. Gemini  — credential guard, read_file|read_many_files matcher (explicit D18 call)
//   6. Cursor  — credential guard, beforeShellExecution (via registry)
//   7. Copilot — project scope always writes current form (file recreated wholesale)
//   8. Idempotence: running InjectClaudeHooks twice with D18 form leaves file unchanged
//   9. D11Rev → current form (D11 revised legacy migrates directly to D18 current, not to D18 legacy)
//  10. Mixture: D18 legacy + current form → exactly one entry
//  11. Global scope: UpdateHarness migrates D18 global credential form
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//   Test 1  asserts: D18 legacy in Claude Read matcher is rewritten to current by InjectClaudeHooks.
//   Test 4  asserts: D18 legacy in Codex apply_patch matcher is rewritten to current by InjectCodexHooks.
//   Test 5  asserts: D18 legacy in Gemini read_file matcher is rewritten to current by InjectGeminiHooks.
//   Test 6  asserts: D18 legacy in Cursor beforeShellExecution is rewritten by migrateDedupGuardInFlatArray via CredentialLegacyPSPOSIX.
//   Test 8  asserts: second InjectClaudeHooks call on already-migrated config changes no bytes.
//   Test 9  asserts: D11Rev form migrates to the D18 current form, not to the D18 legacy form.
//   Test 10 asserts: a mixture of D18 legacy + current in the same (event, matcher) pair collapses to exactly one entry.
//   Test 11 asserts: UpdateHarness migrates the D18 global credential form to the global current form.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

// d18CountLeaf counts exact string matches at any leaf in the parsed JSON tree.
// Local to this file — equivalent to countJSONLeafMatches in internal/commands
// but independent of that package so this test file stands alone.
func d18CountLeaf(v interface{}, want string) int {
	switch vv := v.(type) {
	case string:
		if vv == want {
			return 1
		}
		return 0
	case []interface{}:
		n := 0
		for _, e := range vv {
			n += d18CountLeaf(e, want)
		}
		return n
	case map[string]interface{}:
		n := 0
		for _, e := range vv {
			n += d18CountLeaf(e, want)
		}
		return n
	default:
		return 0
	}
}

// d18CountFlatCmd counts flat-array entries where command==cmd and matcher==matcher.
// matcher=="" means no matcher field. Used for Cursor's flat hook arrays.
func d18CountFlatCmd(arr []interface{}, cmd, matcher string) int {
	n := 0
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		objCmd, _ := obj["command"].(string)
		objMatcher, _ := obj["matcher"].(string)
		if objCmd == cmd && objMatcher == matcher {
			n++
		}
	}
	return n
}

// d18IsolateHome sets HOME and USERPROFILE to a fresh TempDir and clears CLAUDE_CONFIG_DIR.
// Returns the temp home dir.
func d18IsolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home
}

// ─── Test 1: Claude credential guard, Read matcher ─────────────────────────────

// TestD18Migration_Claude_CredentialRead asserts that D18 legacy credential form in
// Claude's PreToolUse[Read] is rewritten to the current D18 form by InjectClaudeHooks.
// The Read matcher has NO dedup backstop (migrateDedupGuardInNestedArray only runs for
// Bash|PowerShell and Bash), so the explicit migrateHookCommand call is load-bearing.
//
// Reconciliação: D18 legacy in Read matcher migrates to current — if the explicit D18
// migrateHookCommand loop in InjectClaudeHooks is removed, this test fails.
func TestD18Migration_Claude_CredentialRead(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX

	// Seed the config with the D18 legacy form in PreToolUse[Read].
	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Read",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), initial)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	// Assert new form present, legacy form gone.
	if !helperHasClaudeHook(data, "PreToolUse", "Read", currentForm) {
		t.Errorf("PreToolUse[Read]: current D18 form missing after migration")
	}
	if helperHasClaudeHook(data, "PreToolUse", "Read", legacyForm) {
		t.Errorf("PreToolUse[Read]: D18 legacy form still present after migration")
	}
	// Exactly one entry with the new form in PreToolUse[Read].
	if n := countNestedEntries(data, "PreToolUse", "Read", currentForm); n != 1 {
		t.Errorf("PreToolUse[Read]: expected exactly 1 current-form entry, got %d", n)
	}
}

// ─── Test 2: Claude git-branch guard ───────────────────────────────────────────

// TestD18Migration_Claude_GitBranch asserts that D18 legacy git-branch form in
// Claude's PreToolUse[Bash|PowerShell] is rewritten to current by InjectClaudeHooks.
//
// Reconciliação: D18 legacy git-branch form migrates to current — the explicit D18
// migrateHookCommand calls and/or migrateDedupGuardInNestedArray(GitBranchLegacyPSPOSIX)
// together ensure the form is replaced.
func TestD18Migration_Claude_GitBranch(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18GitBranchPSPOSIX
	currentForm := guardforms.GitBranchCurrentPSPOSIX

	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": claudeShellMatcher,
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), initial)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, currentForm) {
		t.Errorf("PreToolUse[%s]: current D18 git-branch form missing after migration", claudeShellMatcher)
	}
	if helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, legacyForm) {
		t.Errorf("PreToolUse[%s]: D18 legacy git-branch form still present", claudeShellMatcher)
	}
	if n := countNestedEntries(data, "PreToolUse", claudeShellMatcher, currentForm); n != 1 {
		t.Errorf("PreToolUse[%s]: expected exactly 1 git-branch entry, got %d", claudeShellMatcher, n)
	}
}

// ─── Test 3: Claude write-scope guard ──────────────────────────────────────────

// TestD18Migration_Claude_WriteScope asserts that D18 legacy write-scope form in
// Claude's PreToolUse[Write|Edit|NotebookEdit] is rewritten to current.
//
// Reconciliação: D18 legacy write-scope form migrates to current — LegacyD18WriteScopePSPOSIX
// is in WriteScopeLegacyPSPOSIX, so migrateDedupGuardInNestedArray handles it.
func TestD18Migration_Claude_WriteScope(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18WriteScopePSPOSIX
	currentForm := guardforms.WriteScopeCurrentPSPOSIX

	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": writeScopeMatcher,
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), initial)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	if !helperHasClaudeHook(data, "PreToolUse", writeScopeMatcher, currentForm) {
		t.Errorf("PreToolUse[%s]: current write-scope form missing after migration", writeScopeMatcher)
	}
	if helperHasClaudeHook(data, "PreToolUse", writeScopeMatcher, legacyForm) {
		t.Errorf("PreToolUse[%s]: D18 legacy write-scope form still present", writeScopeMatcher)
	}
	if n := countNestedEntries(data, "PreToolUse", writeScopeMatcher, currentForm); n != 1 {
		t.Errorf("PreToolUse[%s]: expected exactly 1 write-scope entry, got %d", writeScopeMatcher, n)
	}
}

// ─── Test 4: Codex credential guard, apply_patch matcher ───────────────────────

// TestD18Migration_Codex_CredentialApplyPatch asserts that D18 legacy credential form
// in Codex's PreToolUse[apply_patch] is rewritten to current by InjectCodexHooks.
//
// Reconciliação: D18 legacy in Codex apply_patch migrates to current — both the
// explicit migrateHookCommand D18 call and the subsequent migrateDedupGuardInNestedArray
// (CredentialLegacyPSPOSIX) handle it.
func TestD18Migration_Codex_CredentialApplyPatch(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX

	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "apply_patch",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".codex", "hooks.json"), initial)

	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))

	if !helperHasClaudeHook(data, "PreToolUse", "apply_patch", currentForm) {
		t.Errorf("PreToolUse[apply_patch]: current D18 form missing after Codex migration")
	}
	if helperHasClaudeHook(data, "PreToolUse", "apply_patch", legacyForm) {
		t.Errorf("PreToolUse[apply_patch]: D18 legacy form still present after Codex migration")
	}
	if n := countNestedEntries(data, "PreToolUse", "apply_patch", currentForm); n != 1 {
		t.Errorf("PreToolUse[apply_patch]: expected exactly 1 current-form entry, got %d", n)
	}
}

// ─── Test 5: Gemini credential guard, read_file|read_many_files matcher ─────────

// TestD18Migration_Gemini_CredentialReadFile asserts that D18 legacy credential form
// in Gemini's BeforeTool[read_file|read_many_files] is rewritten to current.
//
// Reconciliação: D18 legacy in Gemini read_file matcher migrates to current — the
// explicit migrateHookCommand D18 loop in InjectGeminiHooks covers this matcher.
func TestD18Migration_Gemini_CredentialReadFile(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX
	const readFileMatcher = "read_file|read_many_files"

	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"BeforeTool": []interface{}{
				map[string]interface{}{
					"matcher": readFileMatcher,
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".gemini", "settings.json"), initial)

	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))

	if !helperHasClaudeHook(data, "BeforeTool", readFileMatcher, currentForm) {
		t.Errorf("BeforeTool[%s]: current D18 form missing after Gemini migration", readFileMatcher)
	}
	if helperHasClaudeHook(data, "BeforeTool", readFileMatcher, legacyForm) {
		t.Errorf("BeforeTool[%s]: D18 legacy form still present after Gemini migration", readFileMatcher)
	}
	if n := countNestedEntries(data, "BeforeTool", readFileMatcher, currentForm); n != 1 {
		t.Errorf("BeforeTool[%s]: expected exactly 1 current-form entry, got %d", readFileMatcher, n)
	}
}

// ─── Test 6: Cursor credential guard, beforeShellExecution ─────────────────────

// TestD18Migration_Cursor_CredentialBeforeShell asserts that D18 legacy credential
// form in Cursor's beforeShellExecution is rewritten to current by InjectCursorHooks.
// Cursor uses migrateDedupGuardInFlatArray with CredentialLegacyPSPOSIX which includes
// LegacyD18CredentialPSPOSIX — no explicit D18 migrateHookCommand call.
//
// Reconciliação: D18 legacy in Cursor beforeShellExecution migrates via registry —
// if LegacyD18CredentialPSPOSIX is removed from CredentialLegacyPSPOSIX, this test fails.
func TestD18Migration_Cursor_CredentialBeforeShell(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX

	initial := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"beforeShellExecution": []interface{}{
				map[string]interface{}{"command": legacyForm},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".cursor", "hooks.json"), initial)

	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	arr, _ := hooks["beforeShellExecution"].([]interface{})

	if d18CountFlatCmd(arr, currentForm, "") == 0 {
		t.Errorf("beforeShellExecution: current D18 form missing after Cursor migration")
	}
	if d18CountFlatCmd(arr, legacyForm, "") != 0 {
		t.Errorf("beforeShellExecution: D18 legacy form still present after Cursor migration")
	}
	if n := d18CountFlatCmd(arr, currentForm, ""); n != 1 {
		t.Errorf("beforeShellExecution: expected exactly 1 current-form entry, got %d", n)
	}
}

// ─── Test 7: Copilot project scope ─────────────────────────────────────────────

// TestD18Migration_Copilot_ProjectScopeAlwaysCurrentForm asserts that after
// InjectCopilotHooks, the .github/hooks/trackfw-attention.json always contains
// the current D18 credential form, regardless of whatever was there before.
// InjectCopilotHooks rebuilds the file from scratch every run, so "migration"
// is trivially guaranteed by the generator writing the current form.
//
// Reconciliação: Copilot project-scope config always contains current credential
// form after InjectCopilotHooks — the generator writes it wholesale, not via migration.
func TestD18Migration_Copilot_ProjectScopeAlwaysCurrentForm(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX

	// Pre-seed the file with the D18 legacy form.
	initial := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{"type": "command", "matcher": "bash", "command": legacyForm},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"), initial)

	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"))
	raw, _ := json.Marshal(data)

	if d18CountLeaf(data, currentForm) == 0 {
		t.Errorf("Copilot trackfw-attention.json: current D18 form missing (got: %s)", raw)
	}
	if d18CountLeaf(data, legacyForm) != 0 {
		t.Errorf("Copilot trackfw-attention.json: D18 legacy form still present (got: %s)", raw)
	}
}

// ─── Test 8: Idempotence ───────────────────────────────────────────────────────

// TestD18Migration_Claude_Idempotence asserts that running InjectClaudeHooks a second
// time on a config that already contains the D18 current form does not change any bytes.
//
// Reconciliação: post-migration InjectClaudeHooks is idempotent — the dedup and
// migration logic must be stable when the canonical form is already present.
func TestD18Migration_Claude_Idempotence(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX

	// Seed with D18 legacy form.
	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Read",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), initial)

	// First run: migrates legacy → current.
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks (first): %v", err)
	}
	firstRun, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read after first run: %v", err)
	}
	// Verify migration happened.
	var firstDoc map[string]interface{}
	if err := json.Unmarshal(firstRun, &firstDoc); err != nil {
		t.Fatalf("unmarshal first run: %v", err)
	}
	if !helperHasClaudeHook(firstDoc, "PreToolUse", "Read", currentForm) {
		t.Fatalf("first run did not migrate D18 legacy form to current")
	}

	// Second run: must be a no-op at the byte level.
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks (second): %v", err)
	}
	secondRun, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read after second run: %v", err)
	}

	if !bytes.Equal(firstRun, secondRun) {
		t.Errorf("InjectClaudeHooks is not idempotent after D18 migration:\nfirst:\n%s\nsecond:\n%s",
			firstRun, secondRun)
	}
}

// ─── Test 9: D11Rev form migrates directly to current form ─────────────────────

// TestD18Migration_D11RevToCurrentForm asserts that the D11 revised legacy form
// (pre-REQ-2026-10-20, without emulate prefix) migrates directly to the D18 current
// form, never to the D18 legacy intermediate form.
//
// Reconciliação: D11Rev in Claude credential guard migrates to the same D18 current form
// as D18 legacy — both LegacyD11RevCredentialPSPOSIX and LegacyD18CredentialPSPOSIX are
// in CredentialLegacyPSPOSIX and treated equivalently by migrateDedupGuardInNestedArray.
func TestD18Migration_D11RevToCurrentForm(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	d11RevForm := guardforms.LegacyD11RevCredentialPSPOSIX
	d18LegacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX

	// Seed with D11Rev form in Bash|PowerShell matcher.
	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": claudeShellMatcher,
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": d11RevForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), initial)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	// Must have current form.
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, currentForm) {
		t.Errorf("PreToolUse[%s]: current D18 form missing after D11Rev migration", claudeShellMatcher)
	}
	// Must NOT have D11Rev form.
	if helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, d11RevForm) {
		t.Errorf("PreToolUse[%s]: D11Rev form still present after migration", claudeShellMatcher)
	}
	// Must NOT have D18 legacy form as an intermediate.
	if helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, d18LegacyForm) {
		t.Errorf("PreToolUse[%s]: D18 legacy form appeared as intermediate during D11Rev migration", claudeShellMatcher)
	}
	if n := countNestedEntries(data, "PreToolUse", claudeShellMatcher, currentForm); n != 1 {
		t.Errorf("PreToolUse[%s]: expected exactly 1 current-form entry after D11Rev migration, got %d", claudeShellMatcher, n)
	}
}

// ─── Test 10: Mixture collapses to one entry ────────────────────────────────────

// TestD18Migration_MixtureCollapseToOne asserts that when both the D18 legacy form
// and the current D18 form are present in the same (event, matcher) pair, running
// InjectClaudeHooks collapses them to exactly one current-form entry.
//
// Reconciliação: mixture of D18 legacy + current in Claude Read matcher → exactly 1
// current-form entry — migrateDedupGuardInNestedArray or migrateHookCommand must
// eliminate duplicates when both forms coexist in the same group.
func TestD18Migration_MixtureCollapseToOne(t *testing.T) {
	dir := t.TempDir()
	d18IsolateHome(t)

	legacyForm := guardforms.LegacyD18CredentialPSPOSIX
	currentForm := guardforms.CredentialCurrentPSPOSIX

	// Seed with BOTH legacy and current in the same Read group.
	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Read",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
						map[string]interface{}{"type": "command", "command": currentForm},
					},
				},
			},
		},
	}
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), initial)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	// Must have current form exactly once.
	if n := countNestedEntries(data, "PreToolUse", "Read", currentForm); n != 1 {
		t.Errorf("PreToolUse[Read]: want exactly 1 current-form entry after mixture collapse, got %d", n)
	}
	// Must have no legacy form.
	if n := countNestedEntries(data, "PreToolUse", "Read", legacyForm); n != 0 {
		t.Errorf("PreToolUse[Read]: D18 legacy form should be absent after mixture collapse, got %d", n)
	}
}

// ─── Test 11: Global scope via UpdateHarness ────────────────────────────────────

// TestD18Migration_GlobalScope asserts that UpdateHarness migrates the D18 global
// credential legacy form (LegacyD18CredentialGlobalPSPOSIX) to the current global
// credential form (CredentialGlobalCurrentPSPOSIX) in ~/.claude/settings.json.
//
// This exercises harnessCredentialGuardTargetClaude (update.go) which has explicit
// migrateHookCommand calls for D18 global forms AND the dedup via CredentialGlobalLegacyPSPOSIX
// that also includes the D18 global legacy form.
//
// Reconciliação: global D18 legacy credential form migrates to global current form via
// UpdateHarness — if LegacyD18CredentialGlobalPSPOSIX is removed from
// CredentialGlobalLegacyPSPOSIX AND the explicit migrateHookCommand calls are removed,
// this test fails.
func TestD18Migration_GlobalScope(t *testing.T) {
	home := d18IsolateHome(t)
	dir := t.TempDir() // project dir (empty — no project hooks file)

	legacyForm := guardforms.LegacyD18CredentialGlobalPSPOSIX
	currentForm := guardforms.CredentialGlobalCurrentPSPOSIX

	// Write ~/.claude/settings.json with D18 global legacy form.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	content := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": claudeShellMatcher,
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": legacyForm},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(content)
	if err := os.WriteFile(settingsPath, append(raw, '\n'), 0644); err != nil {
		t.Fatalf("write ~/.claude/settings.json: %v", err)
	}

	// UpdateHarness with no project dir set — just HOME matters for global scope.
	_ = dir
	report, err := UpdateHarness(UpdateOptions{
		Targets: []string{"claude-credential-guard"},
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}
	if len(report.Targets) == 0 {
		t.Fatal("UpdateHarness returned no targets")
	}

	updated, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read ~/.claude/settings.json after UpdateHarness: %v", err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(updated, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if d18CountLeaf(doc, currentForm) == 0 {
		t.Errorf("~/.claude/settings.json: global current credential form missing after UpdateHarness migration")
	}
	if d18CountLeaf(doc, legacyForm) != 0 {
		t.Errorf("~/.claude/settings.json: D18 global legacy form still present after UpdateHarness migration")
	}
}
