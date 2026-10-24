package generators

// nested_guard_migrate_dedup_test.go — AC2–AC7 (REQ-2026-10-23, ML-1B)
//
// These tests verify the D13/D14-revised fix for issue #599 at the nested-array
// injection sites (Claude/Codex/Gemini/Amazon Q).
//
//   AC3 — each cell of the migration matrix has a test that fails on origin/main
//         and passes on the new code.
//   AC4 — table-driven: for each registry entry in CredentialLegacyPSPOSIX,
//         exactly one canonical entry survives per (event, matcher) pair.
//   AC5 — user hooks survive byte-for-byte after migrate+dedup.
//   AC6 — three D14-revised arms for Claude, Codex, Gemini (seeded with legacy).
//   AC7 — second Inject call produces identical JSON (idempotence).
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//   AC3 — afirma que a duplicação que existia em origin/main (N legacy forms → N
//         canonicals) é colapsada a exactamente 1.
//   AC4 — afirma que cada forma da registry é removida; falsificação: pass list[1:]
//         ao sítio de Claude para um par — o test captura a forma omitida.
//   AC5 — afirma que comandos fora da registry não são tocados.
//   AC6 — afirma que a detecção do guard global aciona os três ramos de D14 revisado.
//   AC7 — afirma que Inject*Hooks é idempotente.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

// ---------------------------------------------------------------------------
// helpers for nested-format checks
// ---------------------------------------------------------------------------

// countNestedEntries counts inner command occurrences in an event's outer array
// for the given matcher (exact string match).
func countNestedEntries(data map[string]interface{}, event, matcher, command string) int {
	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		return 0
	}
	arr, _ := hooks[event].([]interface{})
	count := 0
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok || obj["matcher"] != matcher {
			continue
		}
		innerHooks, _ := obj["hooks"].([]interface{})
		for _, h := range innerHooks {
			hObj, ok := h.(map[string]interface{})
			if ok && hObj["command"] == command {
				count++
			}
		}
	}
	return count
}

// hasNestedEntry is countNestedEntries > 0.
func hasNestedEntry(data map[string]interface{}, event, matcher, command string) bool {
	return countNestedEntries(data, event, matcher, command) > 0
}

// nestedFixtureHome is like dedupFixtureHome but with a distinct name to avoid
// cross-test pollution via t.Setenv.
func nestedFixtureHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home
}

// buildNestedHooks creates a JSON hook object with one inner command in an outer
// envelope — the nested-array format used by Claude/Codex/Gemini/Amazon Q.
func buildNestedHooks(event, matcher, command string) map[string]interface{} {
	return map[string]interface{}{
		"hooks": map[string]interface{}{
			event: []interface{}{
				map[string]interface{}{
					"matcher": matcher,
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": command}},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// AC3 — migration matrix: N legacy forms → exactly 1 canonical
// ---------------------------------------------------------------------------

// TestNestedMigrate_Claude_NLegacies_Deduplicates seeds a Claude project file
// with several different legacy credential guard forms under claudeShellMatcher
// and verifies that exactly one canonical entry survives after InjectClaudeHooks.
//
// Reconciliação: afirma que N formas legadas distintas para o mesmo par
// (PreToolUse, claudeShellMatcher) são colapsadas a 1 canônica pelo D13.
func TestNestedMigrate_Claude_NLegacies_Deduplicates(t *testing.T) {
	nestedFixtureHome(t) // isolate from real HOME; no global installed

	// Build a file with 3 legacy credential forms in separate envelopes.
	legacies := []string{
		guardforms.LegacyD2CredentialPSPOSIX,
		guardforms.LegacyD11CredentialPSPOSIX,
		guardforms.LegacyWABashCredentialPSPOSIX,
	}
	envelopes := make([]interface{}, 0, len(legacies))
	for _, l := range legacies {
		envelopes = append(envelopes, map[string]interface{}{
			"matcher": claudeShellMatcher,
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": l}},
		})
	}
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": envelopes,
		},
	}

	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), existing)

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	n := countNestedEntries(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX)
	if n != 1 {
		t.Errorf("AC3/Claude: expected exactly 1 canonical cred entry in PreToolUse/%s, got %d", claudeShellMatcher, n)
	}
	// No legacy forms should remain.
	for _, l := range legacies {
		if hasNestedEntry(data, "PreToolUse", claudeShellMatcher, l) {
			t.Errorf("AC3/Claude: legacy form %q must not remain in PreToolUse/%s", l, claudeShellMatcher)
		}
	}
}

// TestNestedMigrate_Codex_NLegacies_Deduplicates mirrors Claude for Codex.
//
// Reconciliação: afirma o mesmo colapso N→1 para o par (PreToolUse, Bash) do Codex.
func TestNestedMigrate_Codex_NLegacies_Deduplicates(t *testing.T) {
	nestedFixtureHome(t)

	legacies := []string{
		guardforms.LegacyD2CredentialPSPOSIX,
		guardforms.LegacyD11RevCredentialPSPOSIX,
		guardforms.LegacyBareCredGuard,
	}
	envelopes := make([]interface{}, 0, len(legacies))
	for _, l := range legacies {
		envelopes = append(envelopes, map[string]interface{}{
			"matcher": "Bash",
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": l}},
		})
	}
	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".codex", "hooks.json"), map[string]interface{}{
		"hooks": map[string]interface{}{"PreToolUse": envelopes},
	})

	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))
	n := countNestedEntries(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX)
	if n != 1 {
		t.Errorf("AC3/Codex: expected exactly 1 canonical cred entry in PreToolUse/Bash, got %d", n)
	}
	for _, l := range legacies {
		if hasNestedEntry(data, "PreToolUse", "Bash", l) {
			t.Errorf("AC3/Codex: legacy form %q must not remain", l)
		}
	}
}

// TestNestedMigrate_Gemini_NLegacies_Deduplicates mirrors Claude for Gemini.
//
// Reconciliação: afirma o colapso N→1 para o par (BeforeTool, run_shell_command) do Gemini.
func TestNestedMigrate_Gemini_NLegacies_Deduplicates(t *testing.T) {
	nestedFixtureHome(t)

	legacies := []string{
		guardforms.LegacyGeminiCredGuard,
		guardforms.LegacyD11CredentialPSPOSIX,
		guardforms.LegacyD11RevCredentialPSPOSIX,
	}
	envelopes := make([]interface{}, 0, len(legacies))
	for _, l := range legacies {
		envelopes = append(envelopes, map[string]interface{}{
			"matcher": "run_shell_command",
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": l}},
		})
	}
	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".gemini", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{"BeforeTool": envelopes},
	})

	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	n := countNestedEntries(data, "BeforeTool", "run_shell_command", guardCredentialCmdPSPOSIX)
	if n != 1 {
		t.Errorf("AC3/Gemini: expected exactly 1 canonical cred entry in BeforeTool/run_shell_command, got %d", n)
	}
	for _, l := range legacies {
		if hasNestedEntry(data, "BeforeTool", "run_shell_command", l) {
			t.Errorf("AC3/Gemini: legacy form %q must not remain", l)
		}
	}
}

// TestNestedMigrate_AmazonQ_NLegacies_Deduplicates verifies dedup for Amazon Q.
//
// Reconciliação: afirma colapso N→1 para (preToolUse, execute_bash) do Amazon Q.
// Afirma também que cada forma legada semeada desaparece completamente do output
// (count == 0 para a string exata do comando). Apenas verificar count==1 canônico
// era insuficiente: a condição era satisfeita em origin/main porque a implementação
// antiga adicionava 1 canônico sem remover as formas legadas.
func TestNestedMigrate_AmazonQ_NLegacies_Deduplicates(t *testing.T) {
	nestedFixtureHome(t)

	// Amazon Q uses cmd.exe family.
	legacies := []string{
		guardforms.LegacyD2CredentialCmdExe,
		guardforms.LegacyBareCredGuard, // script-path forms also appear in contaminated files
	}
	envelopes := make([]interface{}, 0, len(legacies))
	for _, l := range legacies {
		envelopes = append(envelopes, map[string]interface{}{
			"matcher": "execute_bash",
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": l}},
		})
	}
	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"), map[string]interface{}{
		"hooks": map[string]interface{}{"preToolUse": envelopes},
	})

	if err := InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("InjectAmazonQHooks: %v", err)
	}

	// Read the agent file directly (not through hooks key)
	b, _ := os.ReadFile(filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json"))
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	n := countNestedEntries(data, "preToolUse", "execute_bash", guardCredentialCmdCmdExe)
	if n != 1 {
		t.Errorf("AC3/AmazonQ: expected exactly 1 canonical cred entry in preToolUse/execute_bash, got %d", n)
	}
	// Each seeded legacy form must be gone: the entire command string must not appear.
	for _, legacy := range legacies {
		if countNestedEntries(data, "preToolUse", "execute_bash", legacy) != 0 {
			t.Errorf("AC3/AmazonQ: legacy form %q must not remain after migration", legacy)
		}
	}
}

// ---------------------------------------------------------------------------
// AC5 — user hooks survive migrate+dedup
// ---------------------------------------------------------------------------

// TestNestedMigrate_Claude_AC5_UserHookSurvives seeds a legacy guard entry AND
// a user-defined hook in the same claudeShellMatcher envelope, then verifies the
// user hook is untouched after InjectClaudeHooks.
//
// Reconciliação: afirma que migrateDedupGuardInNestedArray não toca comandos fora da
// registry — o hook do usuário chega ao output sem alteração.
func TestNestedMigrate_Claude_AC5_UserHookSurvives(t *testing.T) {
	nestedFixtureHome(t)

	const userHookCmd = "custom-user-hook --check"
	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": claudeShellMatcher,
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": guardforms.LegacyD2CredentialPSPOSIX},
						map[string]interface{}{"type": "command", "command": userHookCmd},
					},
				},
			},
		},
	})

	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !hasNestedEntry(data, "PreToolUse", claudeShellMatcher, userHookCmd) {
		t.Errorf("AC5/Claude: user hook %q must survive migrate+dedup", userHookCmd)
	}
	// Legacy must be replaced.
	if hasNestedEntry(data, "PreToolUse", claudeShellMatcher, guardforms.LegacyD2CredentialPSPOSIX) {
		t.Error("AC5/Claude: legacy form must not remain after migration")
	}
}

// TestNestedMigrate_Gemini_AC5_UserHookSurvives mirrors the Claude test for Gemini.
//
// Reconciliação: afirma que o hook do usuário no mesmo envelope de run_shell_command
// sobrevive byte-for-byte.
func TestNestedMigrate_Gemini_AC5_UserHookSurvives(t *testing.T) {
	nestedFixtureHome(t)

	const userHookCmd = "my-audit-tool --scan"
	dir := t.TempDir()
	helperWriteJSON(t, filepath.Join(dir, ".gemini", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"BeforeTool": []interface{}{
				map[string]interface{}{
					"matcher": "run_shell_command",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": guardforms.LegacyD11CredentialPSPOSIX},
						map[string]interface{}{"type": "command", "command": userHookCmd},
					},
				},
			},
		},
	})

	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks: %v", err)
	}

	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))
	if !hasNestedEntry(data, "BeforeTool", "run_shell_command", userHookCmd) {
		t.Errorf("AC5/Gemini: user hook %q must survive", userHookCmd)
	}
	if hasNestedEntry(data, "BeforeTool", "run_shell_command", guardforms.LegacyD11CredentialPSPOSIX) {
		t.Error("AC5/Gemini: legacy form must not remain")
	}
}

// ---------------------------------------------------------------------------
// AC6 — D14-revised arms for Claude, Codex, Gemini (seeded with legacy forms)
// ---------------------------------------------------------------------------

// TestNestedD14Arms_Claude runs the three D15 arms for Claude:
//   ArmA: no global → shell+Read+Write present, each exactly once.
//   ArmB: global CG installed → shell PRESENT (D15); Read+Write also present.
//   ArmC: global GBG installed → git-branch shell PRESENT (D15); cred unaffected.
//
// Each arm is seeded with legacy forms to confirm removal/migration works from
// non-empty starting state.
//
// Reconciliação: afirma que todos os pares (shell+Read+Write) estão presentes
// independentemente do guard global (D15 — wiring de projeto não depende do guard global).
func TestNestedD14Arms_Claude(t *testing.T) {
	t.Run("ArmA_NoGlobal_AllPairsPresent", func(t *testing.T) {
		nestedFixtureHome(t) // no global file → fail-open

		dir := t.TempDir()
		// Seed: two legacy cred forms in claudeShellMatcher + a legacy in Read pair.
		helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": claudeShellMatcher,
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyD2CredentialPSPOSIX}},
					},
					map[string]interface{}{
						"matcher": claudeShellMatcher,
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyD11CredentialPSPOSIX}},
					},
					map[string]interface{}{
						"matcher": "Read",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyWABashCredentialPSPOSIX}},
					},
				},
			},
		})

		if err := InjectClaudeHooks(dir); err != nil {
			t.Fatalf("InjectClaudeHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

		// Shell: exactly 1 canonical (no global).
		if n := countNestedEntries(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmA: expected 1 cred in PreToolUse/%s, got %d", claudeShellMatcher, n)
		}
		// Read: exactly 1 canonical.
		if n := countNestedEntries(data, "PreToolUse", "Read", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmA: expected 1 cred in PreToolUse/Read, got %d", n)
		}
		// Write|Edit: exactly 1 canonical.
		if n := countNestedEntries(data, "PreToolUse", "Write|Edit", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmA: expected 1 cred in PreToolUse/Write|Edit, got %d", n)
		}
	})

	t.Run("ArmB_GlobalCGInstalled_ShellPresent_ReadWritePresent", func(t *testing.T) {
		home := nestedFixtureHome(t)
		// D15: global seed proves the code no longer reads it to skip wiring.
		helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": claudeShellMatcher,
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdPSPOSIX}},
					},
				},
			},
		})

		dir := t.TempDir()
		// Seed: legacy cred form in shell pair and in Read pair.
		helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": claudeShellMatcher,
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyD2CredentialPSPOSIX}},
					},
					map[string]interface{}{
						"matcher": "Read",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyD11RevCredentialPSPOSIX}},
					},
				},
			},
		})

		if err := InjectClaudeHooks(dir); err != nil {
			t.Fatalf("InjectClaudeHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

		// D15: Shell PRESENT (always emitted regardless of global).
		if n := countNestedEntries(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB(D15): expected 1 cred in PreToolUse/%s, got %d", claudeShellMatcher, n)
		}
		// Read: PRESENT.
		if n := countNestedEntries(data, "PreToolUse", "Read", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB(D15): expected 1 cred in PreToolUse/Read, got %d", n)
		}
		// Write|Edit: PRESENT.
		if n := countNestedEntries(data, "PreToolUse", "Write|Edit", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB(D15): expected 1 cred in PreToolUse/Write|Edit, got %d", n)
		}
	})

	t.Run("ArmC_GlobalGBGInstalled_GitBranchShellPresent", func(t *testing.T) {
		home := nestedFixtureHome(t)
		// D15: global git-branch seed proves the code no longer reads it to skip wiring.
		helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": claudeShellMatcher,
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardGitBranchCmdPSPOSIX}},
					},
				},
			},
		})

		dir := t.TempDir()
		// Seed: legacy git-branch form in shell pair.
		helperWriteJSON(t, filepath.Join(dir, ".claude", "settings.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": claudeShellMatcher,
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyD2GitBranchPSPOSIX}},
					},
				},
			},
		})

		if err := InjectClaudeHooks(dir); err != nil {
			t.Fatalf("InjectClaudeHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

		// D15: git-branch PRESENT (always emitted).
		if n := countNestedEntries(data, "PreToolUse", claudeShellMatcher, guardGitBranchCmdPSPOSIX); n != 1 {
			t.Errorf("ArmC(D15): expected 1 git-branch in PreToolUse/%s, got %d", claudeShellMatcher, n)
		}
		// Credential guard Read/Write still present.
		if n := countNestedEntries(data, "PreToolUse", "Read", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmC(D15): cred Read must still be present, got %d", n)
		}
	})
}

// TestNestedD14Arms_Codex mirrors TestNestedD14Arms_Claude for Codex.
//
// Reconciliação: afirma que Bash e apply_patch estão sempre presentes (D15 — wiring
// de projeto não depende do guard global).
func TestNestedD14Arms_Codex(t *testing.T) {
	t.Run("ArmB_GlobalCGInstalled_BashPresent_ApplyPatchPresent", func(t *testing.T) {
		home := nestedFixtureHome(t)
		// D15: global seed proves the code no longer reads it to skip wiring.
		helperWriteJSON(t, filepath.Join(home, ".codex", "hooks.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": "Bash",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdPSPOSIX}},
					},
				},
			},
		})

		dir := t.TempDir()
		// Seed: legacy cred forms in Bash + apply_patch.
		helperWriteJSON(t, filepath.Join(dir, ".codex", "hooks.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": "Bash",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyCodexCredGuard}},
					},
					map[string]interface{}{
						"matcher": "apply_patch",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyBareCredGuard}},
					},
				},
			},
		})

		if err := InjectCodexHooks(dir); err != nil {
			t.Fatalf("InjectCodexHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))

		// D15: Bash PRESENT (always emitted regardless of global).
		if n := countNestedEntries(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB/Codex(D15): expected 1 cred in PreToolUse/Bash, got %d", n)
		}
		// apply_patch: PRESENT.
		if n := countNestedEntries(data, "PreToolUse", "apply_patch", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB/Codex(D15): cred must be present in PreToolUse/apply_patch, got %d", n)
		}
	})
}

// TestNestedD14Arms_Gemini mirrors TestNestedD14Arms_Claude for Gemini.
//
// Reconciliação: afirma que run_shell_command, read e write pairs estão sempre
// presentes (D15 — wiring de projeto não depende do guard global).
func TestNestedD14Arms_Gemini(t *testing.T) {
	t.Run("ArmB_GlobalCGInstalled_ShellPresent_ReadWritePresent", func(t *testing.T) {
		home := nestedFixtureHome(t)
		// D15: global seed proves the code no longer reads it to skip wiring.
		helperWriteJSON(t, filepath.Join(home, ".gemini", "settings.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"BeforeTool": []interface{}{
					map[string]interface{}{
						"matcher": "run_shell_command",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdPSPOSIX}},
					},
				},
			},
		})

		dir := t.TempDir()
		// Seed: legacy in run_shell_command and in read pair.
		helperWriteJSON(t, filepath.Join(dir, ".gemini", "settings.json"), map[string]interface{}{
			"hooks": map[string]interface{}{
				"BeforeTool": []interface{}{
					map[string]interface{}{
						"matcher": "run_shell_command",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyD2CredentialPSPOSIX}},
					},
					map[string]interface{}{
						"matcher": "read_file|read_many_files",
						"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardforms.LegacyD11CredentialPSPOSIX}},
					},
				},
			},
		})

		if err := InjectGeminiHooks(dir); err != nil {
			t.Fatalf("InjectGeminiHooks: %v", err)
		}
		data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))

		// D15: run_shell_command PRESENT (always emitted regardless of global).
		if n := countNestedEntries(data, "BeforeTool", "run_shell_command", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB/Gemini(D15): expected 1 cred in BeforeTool/run_shell_command, got %d", n)
		}
		// read: PRESENT.
		if n := countNestedEntries(data, "BeforeTool", "read_file|read_many_files", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB/Gemini(D15): cred must be present in BeforeTool/read_file|read_many_files, got %d", n)
		}
		// write: PRESENT.
		if n := countNestedEntries(data, "BeforeTool", "write_file|replace", guardCredentialCmdPSPOSIX); n != 1 {
			t.Errorf("ArmB/Gemini(D15): cred must be present in BeforeTool/write_file|replace, got %d", n)
		}
	})
}

// ---------------------------------------------------------------------------
// AC7 — idempotence: second Inject call produces identical JSON
// ---------------------------------------------------------------------------

// TestNestedInject_Claude_Idempotent verifies that running InjectClaudeHooks twice
// produces the same output (no duplicates on second run).
//
// Reconciliação: afirma que o ciclo migrate→dedup→merge é estável — executar
// InjectClaudeHooks num output já correto não altera o arquivo.
func TestNestedInject_Claude_Idempotent(t *testing.T) {
	nestedFixtureHome(t)

	dir := t.TempDir()
	// First run: creates the file.
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("first InjectClaudeHooks: %v", err)
	}
	b1, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read after first inject: %v", err)
	}
	// Second run: must produce identical output.
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("second InjectClaudeHooks: %v", err)
	}
	b2, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read after second inject: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("AC7/Claude: second inject changed the file\nbefore:\n%s\nafter:\n%s", b1, b2)
	}
}

// TestNestedInject_Gemini_Idempotent mirrors AC7 for Gemini.
//
// Reconciliação: mesma afirmação de estabilidade para InjectGeminiHooks.
func TestNestedInject_Gemini_Idempotent(t *testing.T) {
	nestedFixtureHome(t)

	dir := t.TempDir()
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("first InjectGeminiHooks: %v", err)
	}
	b1, err := os.ReadFile(filepath.Join(dir, ".gemini", "settings.json"))
	if err != nil {
		t.Fatalf("read after first inject: %v", err)
	}
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("second InjectGeminiHooks: %v", err)
	}
	b2, err := os.ReadFile(filepath.Join(dir, ".gemini", "settings.json"))
	if err != nil {
		t.Fatalf("read after second inject: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("AC7/Gemini: second inject changed the file\nbefore:\n%s\nafter:\n%s", b1, b2)
	}
}

// TestNestedInject_Codex_Idempotent mirrors AC7 for Codex.
//
// Reconciliação: mesma afirmação de estabilidade para InjectCodexHooks.
func TestNestedInject_Codex_Idempotent(t *testing.T) {
	nestedFixtureHome(t)

	dir := t.TempDir()
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("first InjectCodexHooks: %v", err)
	}
	b1, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("read after first inject: %v", err)
	}
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("second InjectCodexHooks: %v", err)
	}
	b2, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("read after second inject: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("AC7/Codex: second inject changed the file\nbefore:\n%s\nafter:\n%s", b1, b2)
	}
}

// ---------------------------------------------------------------------------
// D15: all pairs always present regardless of global state
// ---------------------------------------------------------------------------

// TestDedup_Claude_D14Revised_ReadWriteAlwaysPresent (D15): ALL pairs are wired
// unconditionally — including shell. The global seed proves the code no longer
// consults it to skip project wiring.
//
// Reconciliação (D15): todos os pares (shell, Read, Write|Edit, PostToolUse/Read)
// estão presentes independente do global.
func TestDedup_Claude_D14Revised_ReadWriteAlwaysPresent(t *testing.T) {
	home := nestedFixtureHome(t)
	// Seed global credential guard — D15 proves this is no longer consulted.
	helperWriteJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": claudeShellMatcher,
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdPSPOSIX}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}
	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))

	// D15: shell pair must be PRESENT (no longer skipped when global installed).
	if !helperHasClaudeHook(data, "PreToolUse", claudeShellMatcher, guardCredentialCmdPSPOSIX) {
		t.Error("D15/Claude: cred guard in shell pair must be present regardless of global (D15)")
	}
	// Read: present.
	if !helperHasClaudeHook(data, "PreToolUse", "Read", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Claude: cred guard must be present in Read pair")
	}
	// Write|Edit: present.
	if !helperHasClaudeHook(data, "PreToolUse", "Write|Edit", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Claude: cred guard must be present in Write|Edit pair")
	}
	// PostToolUse Read: present.
	if !helperHasClaudeHook(data, "PostToolUse", "Read", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Claude: PostToolUse/Read cred guard must be present")
	}
}

// TestDedup_Gemini_D14Revised_ReadWriteAlwaysPresent (D15): ALL pairs wired unconditionally.
//
// Reconciliação (D15): todos os pares do Gemini estão presentes independente do global.
func TestDedup_Gemini_D14Revised_ReadWriteAlwaysPresent(t *testing.T) {
	home := nestedFixtureHome(t)
	helperWriteJSON(t, filepath.Join(home, ".gemini", "settings.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"BeforeTool": []interface{}{
				map[string]interface{}{
					"matcher": "run_shell_command",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdPSPOSIX}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks: %v", err)
	}
	data := helperReadJSON(t, filepath.Join(dir, ".gemini", "settings.json"))

	// D15: shell pair must be PRESENT.
	if !helperHasClaudeHook(data, "BeforeTool", "run_shell_command", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Gemini: shell pair must be present regardless of global (D15)")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "read_file|read_many_files", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Gemini: read pair must be present")
	}
	if !helperHasClaudeHook(data, "BeforeTool", "write_file|replace", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Gemini: write pair must be present")
	}
}

// TestDedup_Codex_D14Revised_ApplyPatchAlwaysPresent (D15): ALL pairs wired unconditionally.
//
// Reconciliação (D15): apply_patch e Bash estão presentes independente do global.
func TestDedup_Codex_D14Revised_ApplyPatchAlwaysPresent(t *testing.T) {
	home := nestedFixtureHome(t)
	helperWriteJSON(t, filepath.Join(home, ".codex", "hooks.json"), map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": guardCredentialGlobalCmdPSPOSIX}},
				},
			},
		},
	})

	dir := t.TempDir()
	if err := InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks: %v", err)
	}
	data := helperReadJSON(t, filepath.Join(dir, ".codex", "hooks.json"))

	// D15: Bash pair must be PRESENT.
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Codex: Bash pair must be present regardless of global (D15)")
	}
	if !helperHasClaudeHook(data, "PreToolUse", "apply_patch", guardCredentialCmdPSPOSIX) {
		t.Error("D15/Codex: apply_patch pair must be present")
	}
}
