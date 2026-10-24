package generators

// copilot_kiro_d14_ml2c_test.go — D14 revised + D15 unconditional wiring for Copilot and Kiro
// (REQ-2026-10-23, ML-2C corretivo + ML-5A D15)
//
// D15 (ADR-2026-10-04 adendo 2026-10-24): InjectCopilotHooks and InjectKiroHooks
// ALWAYS emit ALL matchers unconditionally — the global file is never consulted
// for project wiring decisions.
//
// For Copilot: bash, view, and create|edit must ALL be present regardless of
// global state. For Kiro: shell, read, and write must ALL be present regardless
// of global state.
//
// Two arms per CLI (kept to exercise both code paths):
//   arm A (no global): all pairs must be present.
//   arm B (global installed): ALL pairs still present (D15 — no longer skips "bash" or "shell").
//
// Reconciliação (D15):
//   /with-global arms now assert the COVERED pair IS present (was: absent under D14).

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

			// D15: "bash" must ALWAYS be present regardless of global state.
			for _, event := range []string{"preToolUse", "postToolUse"} {
				bashCount := copilotD14CountMatcher(hooks, event, "bash", guardCredentialCmdPSPOSIX)
				if bashCount != 1 {
					t.Errorf("[%s] %s×bash: bash credential guard count=%d, want 1 (D15: always emitted)", tc.name, event, bashCount)
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

			// D15: "shell" must ALWAYS be present regardless of global state.
			for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
				shellCount := kiroD14CountPair(hooksRaw, trigger, "shell", guardCredentialCmdCmdExe)
				if shellCount != 1 {
					t.Errorf("[%s] %s×shell: count=%d, want 1 (D15: always emitted)", tc.name, trigger, shellCount)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC6 seed: verify removeGuardFromFlatArrayInHooks is exercised on "shell" arm
// ---------------------------------------------------------------------------

// TestKiroD15_ShellAlwaysPresent (D15): InjectKiroHooks always emits the "shell"
// matcher regardless of global guard state — the pre-seeded legacy entry is
// migrated and the canonical form is emitted unconditionally.
//
// Reconciliação: afirma que shell, read e write estão sempre presentes no wiring
// de projeto Kiro independentemente do guard global (D15).
func TestKiroD15_ShellAlwaysPresent(t *testing.T) {
	home := harnessHome(t)
	installKiroGlobal(t)

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

	// D15: shell must be PRESENT (always emitted regardless of global).
	for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
		n := kiroD14CountPair(hooksRaw, trigger, "shell", guardCredentialCmdCmdExe)
		if n != 1 {
			t.Errorf("%s×shell: count=%d, want 1 (D15: always emitted)", trigger, n)
		}
	}
	// read and write must also be present.
	for _, trigger := range []string{"PreToolUse", "PostToolUse"} {
		for _, matcher := range []string{"read", "write"} {
			n := kiroD14CountPair(hooksRaw, trigger, matcher, guardCredentialCmdCmdExe)
			if n != 1 {
				t.Errorf("%s×%s: count=%d, want 1", trigger, matcher, n)
			}
		}
	}
}
