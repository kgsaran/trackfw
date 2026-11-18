package generators

// antigravity_hooks_test.go — tests for InjectAntigravityHooks (ML-3B, REQ-2026-11-17).
//
// Each test function documents the conclusion it asserts (CLAUDE.md Regra Dura de
// Reconciliação): the conclusion line is the sentence another agent would use to
// decide whether to run this test.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

// helperAntigravityHook returns true if root["trackfw"]["PreToolUse"] contains an
// entry with the given matcher that has the given command in its hooks array.
func helperAntigravityHook(root map[string]interface{}, matcher, command string) bool {
	group, _ := root["trackfw"].(map[string]interface{})
	if group == nil {
		return false
	}
	arr, _ := group["PreToolUse"].([]interface{})
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})
		for _, h := range innerHooks {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == command {
				return true
			}
		}
	}
	return false
}

// helperAntigravityEntryCount returns the total number of entries in
// root["trackfw"]["PreToolUse"].
func helperAntigravityEntryCount(root map[string]interface{}) int {
	group, _ := root["trackfw"].(map[string]interface{})
	if group == nil {
		return 0
	}
	arr, _ := group["PreToolUse"].([]interface{})
	return len(arr)
}

// TestInjectAntigravityHooks_Create asserts:
// InjectAntigravityHooks creates .agents/hooks.json with 4 D5 entries in the
// "trackfw" group: 3 credential entries (run_command|manage_task, write_to_file|...,
// view_file) and 1 git-branch entry with matcher exactly "run_command" (not
// "run_command|manage_task" — manage_task carries Input, not CommandLine, so the
// git-branch guard must not fire there; ADR D5 ML-3B corretivo).
func TestInjectAntigravityHooks_Create(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}

	path := filepath.Join(dir, ".agents", "hooks.json")
	data := helperReadJSON(t, path)

	cred := guardCredentialCmdCmdExe
	git := guardGitBranchCmdCmdExe

	checks := []struct {
		matcher string
		command string
		label   string
	}{
		{antigravityMatcherRunCmd, cred, "credential/run_command|manage_task"},
		{antigravityMatcherWrite, cred, "credential/write_to_file|..."},
		{antigravityMatcherView, cred, "credential/view_file"},
		{antigravityMatcherGitBranch, git, "git-branch/run_command"},
	}
	for _, c := range checks {
		if !helperAntigravityHook(data, c.matcher, c.command) {
			t.Errorf("missing entry: matcher=%q command=%q (%s)", c.matcher, c.command, c.label)
		}
	}

	// Git-branch entry must use the literal matcher "run_command", not "run_command|manage_task".
	if helperAntigravityHook(data, "run_command|manage_task", git) {
		t.Errorf("git-branch command found under run_command|manage_task (wrong matcher — must be run_command only)")
	}
	if !helperAntigravityHook(data, "run_command", git) {
		t.Errorf("git-branch command NOT found under literal matcher run_command")
	}

	// Exactly 4 entries in the trackfw group (3 cred + 1 git-branch).
	if n := helperAntigravityEntryCount(data); n != 4 {
		t.Errorf("trackfw.PreToolUse entry count = %d, want 4", n)
	}
}

// TestInjectAntigravityHooks_Idempotent asserts:
// Calling InjectAntigravityHooks twice on the same directory produces
// byte-identical output — no duplicate entries are added on the second run.
func TestInjectAntigravityHooks_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("first run: %v", err)
	}
	path := filepath.Join(dir, ".agents", "hooks.json")
	run1, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after first run: %v", err)
	}

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("second run: %v", err)
	}
	run2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second run: %v", err)
	}

	if !bytes.Equal(run1, run2) {
		t.Errorf("second run produced different output:\nrun1: %s\nrun2: %s", run1, run2)
	}

	// Entry count must still be 4.
	data := helperReadJSON(t, path)
	if n := helperAntigravityEntryCount(data); n != 4 {
		t.Errorf("after 2 runs, trackfw.PreToolUse entry count = %d, want 4", n)
	}
}

// TestInjectAntigravityHooks_PreservesThirdParty asserts:
// An existing .agents/hooks.json with a third-party group is preserved after
// InjectAntigravityHooks runs — only the "trackfw" group is managed.
func TestInjectAntigravityHooks_PreservesThirdParty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-seed a third-party group.
	initial := map[string]interface{}{
		"third-party-vendor": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "vendor-guard || exit 1"},
					},
				},
			},
		},
	}
	b, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(filepath.Join(agentsDir, "hooks.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(agentsDir, "hooks.json"))

	// Third-party group must still exist.
	vendor, _ := data["third-party-vendor"].(map[string]interface{})
	if vendor == nil {
		t.Error("third-party group 'third-party-vendor' was removed")
	} else {
		arr, _ := vendor["PreToolUse"].([]interface{})
		if len(arr) != 1 {
			t.Errorf("third-party PreToolUse entry count = %d, want 1", len(arr))
		}
	}

	// trackfw group must also exist with its 4 entries.
	if n := helperAntigravityEntryCount(data); n != 4 {
		t.Errorf("trackfw.PreToolUse entry count = %d, want 4", n)
	}
}

// TestInjectAntigravityHooks_SymlinkDirRejected asserts:
// When .agents/ is a symlink pointing outside the project root, InjectAntigravityHooks
// returns an error and does not write the target file — pathguard rejects it.
func TestInjectAntigravityHooks_SymlinkDirRejected(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir() // outside project root
	t.Setenv("CODEX_HOME", "")

	symlinkPath := filepath.Join(dir, ".agents")
	symlinkOrSkip(t, target, symlinkPath)

	err := InjectAntigravityHooks(dir)
	if err == nil {
		t.Error("expected error for symlink .agents/ directory, got nil")
	}

	// The target directory outside the root must not contain hooks.json.
	if _, statErr := os.Stat(filepath.Join(target, "hooks.json")); statErr == nil {
		t.Error("hooks.json was written to symlink target outside project root")
	}
}

// TestInjectAntigravityHooks_NoByteOrderMark asserts:
// The generated .agents/hooks.json does not begin with a UTF-8 BOM (EF BB BF).
// A BOM causes the Antigravity runtime to fail JSON parsing (T9, threat model).
func TestInjectAntigravityHooks_NoByteOrderMark(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".agents", "hooks.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("hooks.json starts with UTF-8 BOM — will break Antigravity JSON parser")
	}
}

// TestInjectAntigravityHooks_NoPSPOSIXForms asserts:
// When the existing trackfw group contains a PSPOSIX credential form, InjectAntigravityHooks
// replaces it with the CmdExe form — the dedup pass normalizes wrong-family commands.
// Exactly 4 entries remain after normalization (not 5).
func TestInjectAntigravityHooks_NoPSPOSIXForms(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Seed the trackfw group with PSPOSIX credential forms (wrong family for Antigravity).
	psposixCred := guardforms.CredentialCurrentPSPOSIX
	initial := map[string]interface{}{
		"trackfw": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": antigravityMatcherRunCmd,
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": psposixCred}},
				},
			},
		},
	}
	b, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(filepath.Join(agentsDir, "hooks.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(agentsDir, "hooks.json"))

	// PSPOSIX form must be gone from run_command|manage_task.
	if helperAntigravityHook(data, antigravityMatcherRunCmd, psposixCred) {
		t.Errorf("PSPOSIX credential form survived in run_command|manage_task (should have been replaced by CmdExe)")
	}

	// CmdExe form must be present.
	if !helperAntigravityHook(data, antigravityMatcherRunCmd, guardCredentialCmdCmdExe) {
		t.Errorf("CmdExe credential form missing from run_command|manage_task after normalization")
	}

	// Total entries must still be 4 (no duplicates from the seed + 3 new entries).
	if n := helperAntigravityEntryCount(data); n != 4 {
		t.Errorf("after PSPOSIX normalization, trackfw.PreToolUse entry count = %d, want 4", n)
	}
}

// TestInjectHooksForTools_AntigravityNowWrites asserts:
// InjectHooksForTools(["antigravity"], root) now writes .agents/hooks.json —
// prior to ML-3B (REQ-2026-11-17), "antigravity" was absent from cliDetectors
// and the call was a silent skip.
// Contra-arm: InjectHooksForTools(["opencode"], root) still writes nothing.
func TestInjectHooksForTools_AntigravityNowWrites(t *testing.T) {
	t.Setenv("CODEX_HOME", "")

	t.Run("arm_antigravity_writes", func(t *testing.T) {
		dir := t.TempDir()
		if err := InjectHooksForTools([]string{"antigravity"}, dir); err != nil {
			t.Fatalf("InjectHooksForTools([antigravity]): %v", err)
		}
		path := filepath.Join(dir, ".agents", "hooks.json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf(".agents/hooks.json was not created: %v", err)
		}
		data := helperReadJSON(t, path)
		if n := helperAntigravityEntryCount(data); n != 4 {
			t.Errorf("trackfw.PreToolUse entry count = %d, want 4", n)
		}
	})

	t.Run("contra_opencode_still_skips", func(t *testing.T) {
		dir := t.TempDir()
		if err := InjectHooksForTools([]string{"opencode"}, dir); err != nil {
			t.Fatalf("InjectHooksForTools([opencode]): %v", err)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Errorf("opencode should still be a silent skip; got entries: %v", entries)
		}
	})
}

// TestInjectHooksDetected_AntigravityAgentsDir asserts:
// InjectHooksDetected fires InjectAntigravityHooks when .agents/ is a directory
// in the project root — the detection predicate checks for the directory.
// Contra-arm: a plain file named .agents (not a directory) does NOT trigger injection.
func TestInjectHooksDetected_AntigravityAgentsDir(t *testing.T) {
	t.Setenv("CODEX_HOME", "")

	t.Run("arm_directory_triggers", func(t *testing.T) {
		hooksHome := t.TempDir()
		t.Setenv("HOME", hooksHome)
		t.Setenv("USERPROFILE", hooksHome)
		t.Setenv("CLAUDE_CONFIG_DIR", "")

		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0755); err != nil {
			t.Fatal(err)
		}

		if err := InjectHooksDetected(dir); err != nil {
			t.Fatalf("InjectHooksDetected: %v", err)
		}

		path := filepath.Join(dir, ".agents", "hooks.json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf(".agents/hooks.json was not created when .agents/ directory exists: %v", err)
		}
	})

	t.Run("contra_file_not_directory", func(t *testing.T) {
		hooksHome := t.TempDir()
		t.Setenv("HOME", hooksHome)
		t.Setenv("USERPROFILE", hooksHome)
		t.Setenv("CLAUDE_CONFIG_DIR", "")

		dir := t.TempDir()
		// Place a plain file named ".agents" — not a directory.
		if err := os.WriteFile(filepath.Join(dir, ".agents"), []byte("not a dir"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := InjectHooksDetected(dir); err != nil {
			t.Fatalf("InjectHooksDetected: %v", err)
		}

		// No hooks.json should be created when .agents is not a directory.
		if _, err := os.Stat(filepath.Join(dir, ".agents", "hooks.json")); err == nil {
			t.Error(".agents/hooks.json was created even though .agents is a file, not a directory")
		}
	})
}

// helperSeedAntigravityEntry writes a minimal .agents/hooks.json with one or more
// explicit entries under trackfw["PreToolUse"] so migration tests start from a known state.
func helperSeedAntigravityEntry(t *testing.T, dir string, entries []map[string]interface{}) {
	t.Helper()
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	makeHookEntry := func(matcher, cmd string) interface{} {
		return map[string]interface{}{
			"matcher": matcher,
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": cmd}},
		}
	}
	var arr []interface{}
	for _, e := range entries {
		arr = append(arr, makeHookEntry(e["matcher"].(string), e["cmd"].(string)))
	}
	root := map[string]interface{}{
		"trackfw": map[string]interface{}{
			"PreToolUse": arr,
		},
	}
	b, _ := json.MarshalIndent(root, "", "  ")
	if err := os.WriteFile(filepath.Join(agentsDir, "hooks.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestInjectAntigravityHooks_MigratesGitBranchMatcher asserts:
// When an existing .agents/hooks.json has the git-branch entry under the old matcher
// "run_command|manage_task" (placed by a prior version of InjectAntigravityHooks before
// ML-3B corretivo), InjectAntigravityHooks moves it to "run_command" without duplicating
// it, and the credential entry at "run_command|manage_task" is preserved.
func TestInjectAntigravityHooks_MigratesGitBranchMatcher(t *testing.T) {
	cred := guardCredentialCmdCmdExe
	git := guardGitBranchCmdCmdExe

	// Arm (a): seed the exact 4-entry file the old code wrote.
	// Expect: count=4, git at old matcher=0, git at new matcher=1, cred at old matcher=1.
	t.Run("arm_a_old_four_entry_file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CODEX_HOME", "")
		helperSeedAntigravityEntry(t, dir, []map[string]interface{}{
			{"matcher": "run_command|manage_task", "cmd": cred},
			{"matcher": "write_to_file|replace_file_content|multi_replace_file_content", "cmd": cred},
			{"matcher": "view_file", "cmd": cred},
			{"matcher": "run_command|manage_task", "cmd": git}, // old placement
		})
		if err := InjectAntigravityHooks(dir); err != nil {
			t.Fatalf("InjectAntigravityHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".agents", "hooks.json"))

		if n := helperAntigravityEntryCount(data); n != 4 {
			t.Errorf("count = %d, want 4 (old-file migration)", n)
		}
		if helperAntigravityHook(data, "run_command|manage_task", git) {
			t.Errorf("git-branch still at old matcher run_command|manage_task after migration")
		}
		if !helperAntigravityHook(data, "run_command", git) {
			t.Errorf("git-branch missing at new matcher run_command after migration")
		}
		if !helperAntigravityHook(data, "run_command|manage_task", cred) {
			t.Errorf("credential entry at run_command|manage_task was incorrectly removed by migration")
		}
	})

	// Arm (b): seed a mixed envelope with both cred and git under "run_command|manage_task".
	// Expect: credential survives at that matcher; git moves to "run_command"; count=4.
	t.Run("arm_b_mixed_envelope", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CODEX_HOME", "")
		agentsDir := filepath.Join(dir, ".agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}
		// A single envelope with two hooks: cred + git under the old matcher.
		mixedEntry := map[string]interface{}{
			"matcher": "run_command|manage_task",
			"hooks": []interface{}{
				map[string]interface{}{"type": "command", "command": cred},
				map[string]interface{}{"type": "command", "command": git},
			},
		}
		root := map[string]interface{}{
			"trackfw": map[string]interface{}{
				"PreToolUse": []interface{}{mixedEntry},
			},
		}
		b, _ := json.MarshalIndent(root, "", "  ")
		if err := os.WriteFile(filepath.Join(agentsDir, "hooks.json"), append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		if err := InjectAntigravityHooks(dir); err != nil {
			t.Fatalf("InjectAntigravityHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".agents", "hooks.json"))

		if helperAntigravityHook(data, "run_command|manage_task", git) {
			t.Errorf("git-branch still at run_command|manage_task after migration (mixed envelope)")
		}
		if !helperAntigravityHook(data, "run_command|manage_task", cred) {
			t.Errorf("credential removed from run_command|manage_task (should survive mixed envelope migration)")
		}
		if !helperAntigravityHook(data, "run_command", git) {
			t.Errorf("git-branch missing at run_command after mixed-envelope migration")
		}
		if n := helperAntigravityEntryCount(data); n != 4 {
			t.Errorf("count = %d, want 4 (mixed envelope migration)", n)
		}
	})

	// Arm (c): seed GitBranchCurrentPSPOSIX (wrong-family legacy form) under the old matcher.
	// Expect: stripped from run_command|manage_task; CmdExe canonical at run_command.
	t.Run("arm_c_legacy_psposix_at_old_matcher", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CODEX_HOME", "")
		helperSeedAntigravityEntry(t, dir, []map[string]interface{}{
			{"matcher": "run_command|manage_task", "cmd": guardforms.GitBranchCurrentPSPOSIX},
		})
		if err := InjectAntigravityHooks(dir); err != nil {
			t.Fatalf("InjectAntigravityHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".agents", "hooks.json"))

		if helperAntigravityHook(data, "run_command|manage_task", guardforms.GitBranchCurrentPSPOSIX) {
			t.Errorf("legacy PSPOSIX git-branch form survived at run_command|manage_task (should be stripped)")
		}
		if !helperAntigravityHook(data, "run_command", git) {
			t.Errorf("canonical CmdExe git-branch missing at run_command after legacy-PSPOSIX migration")
		}
		if n := helperAntigravityEntryCount(data); n != 4 {
			t.Errorf("count = %d, want 4 (legacy PSPOSIX migration)", n)
		}
	})
}

// --- F2 (ML-FA2 REQ-2026-11-17): enabled:false removed and warning emitted ---

// TestInjectAntigravityHooks_EnabledFalse_RemovedAndWarned asserts that when
// .agents/hooks.json has the "trackfw" group with "enabled":false,
// InjectAntigravityHooks removes the key and emits a warning to stderr.
//
// Conclusion asserted: the "enabled" key is absent from the trackfw group
// after the call, and the warning mentions "enabled:false" on stderr.
func TestInjectAntigravityHooks_EnabledFalse_RemovedAndWarned(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-seed with enabled:false on the trackfw group.
	initial := map[string]interface{}{
		"trackfw": map[string]interface{}{
			"enabled":    false,
			"PreToolUse": []interface{}{},
		},
	}
	b, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(filepath.Join(agentsDir, "hooks.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stderr via the seam.
	var stderrBuf bytes.Buffer
	antigravityErrWriter = &stderrBuf
	t.Cleanup(func() { antigravityErrWriter = nil })

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}

	// The "enabled" key must be gone from the trackfw group.
	data := helperReadJSON(t, filepath.Join(agentsDir, "hooks.json"))
	group, _ := data["trackfw"].(map[string]interface{})
	if group == nil {
		t.Fatal("trackfw group missing after inject")
	}
	if _, hasEnabled := group["enabled"]; hasEnabled {
		t.Errorf(`"enabled" key still present in trackfw group after inject; group=%v`, group)
	}

	// The warning must have been emitted to stderr.
	warn := stderrBuf.String()
	if !strings.Contains(warn, `"enabled":false`) {
		t.Errorf(`expected warning containing "enabled":false on stderr; got %q`, warn)
	}

	// The 4 required entries must still be present.
	if n := helperAntigravityEntryCount(data); n != 4 {
		t.Errorf("trackfw.PreToolUse entry count = %d, want 4", n)
	}
}

// TestInjectAntigravityHooks_ThirdPartyEnabledFalse_Untouched asserts that a
// third-party group with "enabled":false is NOT modified by InjectAntigravityHooks.
//
// Conclusion asserted: InjectAntigravityHooks only manages the "trackfw" group;
// other groups (including those with "enabled":false) are preserved verbatim.
func TestInjectAntigravityHooks_ThirdPartyEnabledFalse_Untouched(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-seed with a third-party group that has enabled:false.
	initial := map[string]interface{}{
		"vendor": map[string]interface{}{
			"enabled":    false,
			"PreToolUse": []interface{}{},
		},
	}
	b, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(filepath.Join(agentsDir, "hooks.json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stderr; expect no warning (vendor group is not "trackfw").
	var stderrBuf bytes.Buffer
	antigravityErrWriter = &stderrBuf
	t.Cleanup(func() { antigravityErrWriter = nil })

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(agentsDir, "hooks.json"))

	// vendor group must still have enabled:false.
	vendor, _ := data["vendor"].(map[string]interface{})
	if vendor == nil {
		t.Fatal("vendor group missing")
	}
	if en, _ := vendor["enabled"].(bool); en != false {
		t.Errorf("vendor enabled expected false, got %v", vendor["enabled"])
	}

	// No warning on stderr.
	if warn := stderrBuf.String(); warn != "" {
		t.Errorf("expected no warning for third-party group, got %q", warn)
	}
}

// TestInjectAntigravityHooks_NoEnabledKey_NoWarning asserts that when the
// trackfw group has no "enabled" key (the normal case), InjectAntigravityHooks
// emits no warning and does not add an "enabled" key.
//
// Conclusion asserted: absence of the "enabled" key is the normal (enabled)
// state; no warning is emitted and the key remains absent after inject.
func TestInjectAntigravityHooks_NoEnabledKey_NoWarning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", "")

	var stderrBuf bytes.Buffer
	antigravityErrWriter = &stderrBuf
	t.Cleanup(func() { antigravityErrWriter = nil })

	if err := InjectAntigravityHooks(dir); err != nil {
		t.Fatalf("InjectAntigravityHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".agents", "hooks.json"))
	group, _ := data["trackfw"].(map[string]interface{})
	if group == nil {
		t.Fatal("trackfw group missing")
	}
	if _, hasEnabled := group["enabled"]; hasEnabled {
		t.Errorf(`"enabled" key present on fresh create, want absent`)
	}
	if warn := stderrBuf.String(); warn != "" {
		t.Errorf("expected no warning on fresh create, got %q", warn)
	}
}
