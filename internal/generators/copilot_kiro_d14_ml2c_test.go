package generators

// copilot_kiro_d14_ml2c_test.go — D14 revised, per-pair coverage for Copilot and Kiro
// (REQ-2026-10-23, ML-2C corretivo)
//
// Copilot and Kiro overwrite their hook files on every run (no migration
// concern). The AC for D14 revised is: for every pair (event, matcher) NOT
// covered by the global harness, the credential guard is emitted regardless of
// whether the global is installed. For Copilot: global covers "bash"; "view"
// and "create|edit" must always be present. For Kiro: global covers "shell";
// "read" and "write" must always be present.
//
// Two arms per CLI:
//   arm A (no global): all pairs must be present.
//   arm B (global installed): uncovered pairs still present; covered pair absent.
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//
//   TestCopilotD14_ViewCreateEditAlwaysPresent/no-global asserts: with no
//   global installed, "view" and "create|edit" entries are present in both
//   preToolUse and postToolUse. This arm passes on HEAD before ML-2C.
//
//   TestCopilotD14_ViewCreateEditAlwaysPresent/with-global asserts: with the
//   global installed (bash covered), "view" and "create|edit" entries are
//   still present; "bash" is absent. This arm FAILS on HEAD before ML-2C
//   because the entire if block (including view and create|edit) is gated on
//   !globalCredentialGuardInstalledCopilot().
//
//   TestKiroD14_ReadWriteAlwaysPresent/no-global asserts: with no global, all
//   pairs (shell, read, write) are present. This arm passes on HEAD before ML-2C.
//
//   TestKiroD14_ReadWriteAlwaysPresent/with-global asserts: with the global
//   installed (shell covered), "read" and "write" are still present; "shell"
//   pre/post are absent. This arm FAILS on HEAD before ML-2C because the entire
//   if block (including read and write) is gated on !globalCredentialGuardInstalledKiro().

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// copilotD14CountMatcher counts entries whose "command" == cmd AND "matcher" == matcher
// in the given preToolUse or postToolUse array.
func copilotD14CountMatcher(hooks map[string]interface{}, event, matcher, cmd string) int {
	arr, _ := hooks[event].([]interface{})
	n := 0
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if obj["matcher"] == matcher && obj["command"] == cmd {
			n++
		}
	}
	return n
}

// kiroD14CountPair counts entries with the given trigger, matcher, and action.command
// in Kiro's flat hooks array.
func kiroD14CountPair(hooksArr []interface{}, trigger, matcher, cmd string) int {
	n := 0
	for _, item := range hooksArr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if obj["trigger"] != trigger || obj["matcher"] != matcher {
			continue
		}
		action, _ := obj["action"].(map[string]interface{})
		if action["command"] == cmd {
			n++
		}
	}
	return n
}

// installCopilotGlobal installs the global credential guard for Copilot
// using UpdateHarness (bash matcher only).
func installCopilotGlobal(t *testing.T) {
	t.Helper()
	if _, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"copilot-credential-guard"},
		InstallMissing: true,
	}); err != nil {
		t.Fatalf("installCopilotGlobal: %v", err)
	}
}

// installKiroGlobal installs the global credential guard for Kiro
// using UpdateHarness (shell matcher only).
func installKiroGlobal(t *testing.T) {
	t.Helper()
	if _, err := UpdateHarness(UpdateOptions{
		Targets:        []string{"kiro-credential-guard"},
		InstallMissing: true,
	}); err != nil {
		t.Fatalf("installKiroGlobal: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestCopilotD14_ViewCreateEditAlwaysPresent
// ---------------------------------------------------------------------------

func TestCopilotD14_ViewCreateEditAlwaysPresent(t *testing.T) {
	for _, tc := range []struct {
		name        string
		installGlob bool
	}{
		{"no-global", false},
		{"with-global", true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			harnessHome(t) // isolated HOME

			if tc.installGlob {
				installCopilotGlobal(t)
			}

			dir := t.TempDir()
			if err := InjectCopilotHooks(dir); err != nil {
				t.Fatalf("InjectCopilotHooks: %v", err)
			}

			data := readJSONFile(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"))
			hooks, _ := data["hooks"].(map[string]interface{})

			// "view" and "create|edit" must be present in both events — not covered by global.
			for _, event := range []string{"preToolUse", "postToolUse"} {
				for _, matcher := range []string{"view", "create|edit"} {
					n := copilotD14CountMatcher(hooks, event, matcher, guardCredentialCmdPSPOSIX)
					if n != 1 {
						t.Errorf("[%s] %s×%s: credential guard count=%d, want 1", tc.name, event, matcher, n)
					}
				}
			}

			// With global installed, "bash" must be absent (global covers it).
			// Without global, "bash" must be present.
			for _, event := range []string{"preToolUse", "postToolUse"} {
				bashCount := copilotD14CountMatcher(hooks, event, "bash", guardCredentialCmdPSPOSIX)
				if tc.installGlob {
					if bashCount != 0 {
						t.Errorf("[%s] %s×bash: bash credential guard must be absent when global installed, got %d", tc.name, event, bashCount)
					}
				} else {
					if bashCount != 1 {
						t.Errorf("[%s] %s×bash: bash credential guard count=%d, want 1 (no global)", tc.name, event, bashCount)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestKiroD14_ReadWriteAlwaysPresent
// ---------------------------------------------------------------------------

func TestKiroD14_ReadWriteAlwaysPresent(t *testing.T) {
	for _, tc := range []struct {
		name        string
		installGlob bool
	}{
		{"no-global", false},
		{"with-global", true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			harnessHome(t) // isolated HOME

			if tc.installGlob {
				installKiroGlobal(t)
			}

			dir := t.TempDir()
			if err := InjectKiroHooks(dir); err != nil {
				t.Fatalf("InjectKiroHooks: %v", err)
			}

			data := readJSONFile(t, filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json"))
			hooksRaw, _ := data["hooks"].([]interface{})

			// "read" and "write" must be present in both PreToolUse and PostToolUse.
			for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
				for _, matcher := range []string{"read", "write"} {
					n := kiroD14CountPair(hooksRaw, trigger, matcher, guardCredentialCmdCmdExe)
					if n != 1 {
						t.Errorf("[%s] %s×%s: credential guard count=%d, want 1", tc.name, trigger, matcher, n)
					}
				}
			}

			// With global installed, "shell" pre/post must be absent.
			// Without global, "shell" pre/post must be present.
			for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
				shellCount := kiroD14CountPair(hooksRaw, trigger, "shell", guardCredentialCmdCmdExe)
				if tc.installGlob {
					if shellCount != 0 {
						t.Errorf("[%s] %s×shell: must be absent when global installed, got %d", tc.name, trigger, shellCount)
					}
				} else {
					if shellCount != 1 {
						t.Errorf("[%s] %s×shell: count=%d, want 1 (no global)", tc.name, trigger, shellCount)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC6 seed: verify removeGuardFromFlatArrayInHooks is exercised on "shell" arm
// ---------------------------------------------------------------------------

// TestKiroD14_ShellRemovalRequiresLegacySeed checks that with global installed
// and a pre-existing legacy shell entry in an earlier file, InjectKiroHooks
// overwrites the file without the shell entries (overwrite semantics; no migration).
// This test exists to seed the fixture with a legacy form so that
// removeGuardFromFlatArrayInHooks is reachable if Kiro ever switches to merge
// semantics — currently it confirms overwrite: shell absent regardless of pre-existing.
func TestKiroD14_ShellRemovalRequiresLegacySeed(t *testing.T) {
	home := harnessHome(t)
	installKiroGlobal(t)

	// Pre-seed the project hook file with a legacy shell credential guard.
	dir := t.TempDir()
	hookPath := filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	_ = home

	if err := InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks: %v", err)
	}

	data := readJSONFile(t, hookPath)
	hooksRaw, _ := data["hooks"].([]interface{})

	// shell must be absent (global covers it; overwrite semantics).
	for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
		n := kiroD14CountPair(hooksRaw, trigger, "shell", guardCredentialCmdCmdExe)
		if n != 0 {
			t.Errorf("%s×shell: must be absent when global installed, got %d", trigger, n)
		}
	}
	// read and write must be present.
	for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
		for _, matcher := range []string{"read", "write"} {
			n := kiroD14CountPair(hooksRaw, trigger, matcher, guardCredentialCmdCmdExe)
			if n != 1 {
				t.Errorf("%s×%s: count=%d, want 1", trigger, matcher, n)
			}
		}
	}
}
