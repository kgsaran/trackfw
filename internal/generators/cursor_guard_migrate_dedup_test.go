package generators

// cursor_guard_migrate_dedup_test.go — AC5, AC6, AC7 (REQ-2026-10-23, ML-1A)
//
// These tests verify the D15 behavior for issue #599 (updated from D13/D14-revised):
//
//   AC5 — user hooks survive:  an entry whose command is NOT in the registry
//          is left untouched byte-for-byte by InjectCursorHooks.
//
//   AC6 — three D15 arms (global guard detection removed; all entries always emitted):
//          A) no global guards → shell events + Read/Write pairs all present, once each.
//          B) global credential guard installed → cred guard PRESENT in shell events
//             (D15: always emitted); Read/Write pairs also present; git-branch present.
//          C) global git-branch guard installed → git-branch PRESENT (D15: always
//             emitted); credential in shell events + Read/Write pairs also present.
//
//   AC7 — idempotency: two consecutive InjectCursorHooks calls produce identical JSON.
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//   AC5 asserts: exact equality comparison leaves non-registry entries untouched.
//   AC6 asserts: InjectCursorHooks always emits all entries regardless of global guard state (D15 — global detection removed).
//   AC7 asserts: migrateDedupGuardInFlatArray is idempotent when the result is canonical.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// cursorWriteHooksJSON writes a .cursor/hooks.json at dir.
func cursorWriteHooksJSON(t *testing.T, dir string, data map[string]interface{}) {
	t.Helper()
	p := filepath.Join(dir, ".cursor", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir .cursor: %v", err)
	}
	helperWriteJSON(t, p, data)
}

// cursorReadHooksJSON reads and returns the hooks sub-map from .cursor/hooks.json at dir.
func cursorReadHooksJSON(t *testing.T, dir string) map[string]interface{} {
	t.Helper()
	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatal("hooks key missing from .cursor/hooks.json")
	}
	return hooks
}

// flatCmd extracts the "command" field of items in a flat []interface{} array.
func flatCmd(item interface{}) string {
	obj, ok := item.(map[string]interface{})
	if !ok {
		return ""
	}
	s, _ := obj["command"].(string)
	return s
}

// flatMatcher extracts the "matcher" field of items in a flat []interface{} array.
func flatMatcher(item interface{}) string {
	obj, ok := item.(map[string]interface{})
	if !ok {
		return ""
	}
	s, _ := obj["matcher"].(string)
	return s
}

// countFlatEntries returns the count of entries in arr whose command+matcher match.
// matcher="" means "no matcher field".
func countFlatEntries(arr []interface{}, cmd, matcher string) int {
	n := 0
	for _, item := range arr {
		if flatCmd(item) == cmd && flatMatcher(item) == matcher {
			n++
		}
	}
	return n
}

// hasMatcherEntry returns true if arr contains an entry with the given cmd+matcher.
func hasMatcherEntry(arr []interface{}, cmd, matcher string) bool {
	return countFlatEntries(arr, cmd, matcher) > 0
}

// writeGlobalCursorCred writes guardCredentialGlobalCmdPSPOSIX to ~/.cursor/hooks.json
// so that globalCredentialGuardInstalledCursor() returns true.
func writeGlobalCursorCred(t *testing.T, home string) {
	t.Helper()
	cursorWriteHooksJSON(t, home, map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"beforeShellExecution": []interface{}{
				map[string]interface{}{"command": guardCredentialGlobalCmdPSPOSIX},
			},
		},
	})
}

// writeGlobalCursorGitBranch writes guardGitBranchCmdPSPOSIX to ~/.cursor/hooks.json
// so that globalGitBranchGuardInstalledCursor() returns true.
func writeGlobalCursorGitBranch(t *testing.T, home string) {
	t.Helper()
	cursorWriteHooksJSON(t, home, map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"beforeShellExecution": []interface{}{
				map[string]interface{}{"command": guardGitBranchCmdPSPOSIX},
			},
		},
	})
}

// ---------------------------------------------------------------------------
// AC5 — user hooks survive
// ---------------------------------------------------------------------------

// TestCursorGuardMigrateDedup_UserHookSurvives asserts that a user-owned hook
// entry whose command is NOT in any guard generation registry survives
// InjectCursorHooks byte-for-byte, including all extra fields (e.g., "timeout").
//
// The user command deliberately contains the substring "trackfw guard credential"
// to prove that the registry comparison uses EXACT equality, not strings.Contains:
// if the implementation used strings.Contains, this entry would be mis-classified
// as a guard entry and overwritten — the test would fail.
//
// Reconciliação: afirma que migrateDedupGuardInFlatArray compara comandos por
// igualdade exata — se strings.Contains fosse usado, "meu-trackfw guard
// credential-hook" seria tratado como guard e reescrito; este teste falharia.
func TestCursorGuardMigrateDedup_UserHookSurvives(t *testing.T) {
	home := dedupFixtureHome(t)
	_ = home // no global guards

	// userCmd contains "trackfw guard credential" as a substring — a strings.Contains
	// check would incorrectly treat it as a credential guard entry.
	const userCmd = "meu-trackfw guard credential-hook --verbose"
	const userMatcher = "Read"
	const userTimeout = float64(30) // extra field that must survive

	dir := t.TempDir()
	cursorWriteHooksJSON(t, dir, map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				// User's own hook (substring match for guard, extra field) — must survive.
				map[string]interface{}{"command": userCmd, "matcher": userMatcher, "timeout": userTimeout},
				// Legacy credential form — must be migrated to canonical.
				map[string]interface{}{"command": legacyBareCredGuardCmd, "matcher": "Read"},
			},
			"beforeShellExecution": []interface{}{
				// User shell hook with guard substring — must survive.
				map[string]interface{}{"command": userCmd, "timeout": userTimeout},
				// Legacy guard form — must be migrated.
				map[string]interface{}{"command": legacyD2CredentialCmdPSPOSIX},
			},
		},
	})

	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks: %v", err)
	}

	hooks := cursorReadHooksJSON(t, dir)

	// preToolUse: user hook must survive once, with exact command+matcher+timeout.
	pre, _ := hooks["preToolUse"].([]interface{})
	userCount := 0
	for _, item := range pre {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if obj["command"] == userCmd && obj["matcher"] == userMatcher && obj["timeout"] == userTimeout {
			userCount++
		}
	}
	if userCount != 1 {
		t.Errorf("AC5: user hook (cmd+matcher+timeout) in preToolUse must appear exactly once; got %d (all pre: %v)", userCount, pre)
	}

	// beforeShellExecution: user shell hook must survive once with extra field.
	before, _ := hooks["beforeShellExecution"].([]interface{})
	shellUserCount := 0
	for _, item := range before {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if obj["command"] == userCmd && obj["timeout"] == userTimeout {
			shellUserCount++
		}
	}
	if shellUserCount != 1 {
		t.Errorf("AC5: user hook in beforeShellExecution must appear exactly once; got %d (all before: %v)", shellUserCount, before)
	}

	// The legacy form must have been replaced; exactly one canonical entry per pair.
	if n := countFlatEntries(pre, legacyBareCredGuardCmd, "Read"); n != 0 {
		t.Errorf("AC5: legacy bareCredGuard in preToolUse/Read must be replaced; count=%d", n)
	}
	if n := countFlatEntries(pre, guardCredentialCmdPSPOSIX, "Read"); n != 1 {
		t.Errorf("AC5: canonical cred guard in preToolUse/Read must appear exactly once; count=%d", n)
	}
}

// ---------------------------------------------------------------------------
// AC6 — three D14-revised arms
// ---------------------------------------------------------------------------

// TestCursorGuardMigrateDedup_D14Arms runs three sub-tests for the three D15 arms
// (ADR-2026-10-04 adendo 2026-10-24): global guard detection was removed; all entries
// are always emitted regardless of global guard state.
//
// Each arm SEEDS the directory with legacy entries so that removeGuardFromFlatArrayInHooks
// is actually exercised on pre-existing data — not just on a fresh empty dir.
//
// Arm A: no global guards installed.
//   - Seed: legacy cred guard + legacy git guard in beforeShellExecution;
//           legacy cred guard entries in preToolUse/Read and postToolUse/Read.
//   - After inject: shell events → exactly 1 canonical cred + 1 canonical git each;
//                   Read/Write pairs → exactly 1 canonical cred each.
//
// Arm B: global credential guard installed.
//   - Seed: two legacy cred forms + user hook in beforeShellExecution;
//           legacy cred forms in preToolUse/Read and postToolUse/Read.
//   - After inject: cred guard PRESENT in beforeShellExecution (D15: always emitted);
//                   user hook survives; git-branch guard present; cred guards in Read/Write pairs.
//
// Arm C: global git-branch guard installed.
//   - Seed: legacy git guard + legacy cred guard in beforeShellExecution.
//   - After inject: git-branch guard PRESENT (D15: always emitted); cred present once;
//                   Read/Write pairs still have cred guard.
//
// Reconciliação: afirma que InjectCursorHooks emite incondicionalmente todas as entradas
// nos três braços — a presença do guard global não suprime o wiring de projeto (D15).
func TestCursorGuardMigrateDedup_D14Arms(t *testing.T) {
	const userShellHook = "my-custom-shell-hook --flag" // not in any registry

	t.Run("ArmA_NoGlobal", func(t *testing.T) {
		home := dedupFixtureHome(t)
		_ = home

		dir := t.TempDir()
		// Seed: legacy forms in shell events and in preToolUse/postToolUse Read.
		cursorWriteHooksJSON(t, dir, map[string]interface{}{
			"version": 1,
			"hooks": map[string]interface{}{
				"beforeShellExecution": []interface{}{
					map[string]interface{}{"command": legacyD2CredentialCmdPSPOSIX},
					map[string]interface{}{"command": legacyD2GitBranchCmdPSPOSIX},
				},
				"afterShellExecution": []interface{}{
					map[string]interface{}{"command": legacyD11CredentialCmdPSPOSIX},
				},
				"preToolUse": []interface{}{
					map[string]interface{}{"command": legacyBareCredGuardCmd, "matcher": "Read"},
				},
				"postToolUse": []interface{}{
					map[string]interface{}{"command": legacyD11RevCredentialCmdPSPOSIX, "matcher": "Read"},
				},
			},
		})

		if err := InjectCursorHooks(dir); err != nil {
			t.Fatalf("InjectCursorHooks: %v", err)
		}
		hooks := cursorReadHooksJSON(t, dir)

		before, _ := hooks["beforeShellExecution"].([]interface{})
		after, _ := hooks["afterShellExecution"].([]interface{})
		pre, _ := hooks["preToolUse"].([]interface{})
		post, _ := hooks["postToolUse"].([]interface{})

		// Shell events: legacy replaced by exactly 1 canonical each.
		if n := countFlatEntries(before, guardCredentialCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmA: beforeShellExecution cred guard count want 1, got %d", n)
		}
		if n := countFlatEntries(before, guardGitBranchCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmA: beforeShellExecution git guard count want 1, got %d", n)
		}
		if n := countFlatEntries(after, guardCredentialCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmA: afterShellExecution cred guard count want 1, got %d", n)
		}
		// Legacy forms must be gone from shell events.
		for _, legacyCmd := range []string{legacyD2CredentialCmdPSPOSIX, legacyD11CredentialCmdPSPOSIX} {
			if n := countFlatEntries(before, legacyCmd, ""); n != 0 {
				t.Errorf("ArmA: legacy cred %q must be removed from beforeShellExecution, got %d", legacyCmd, n)
			}
		}

		// Read/Write pairs: legacy replaced, exactly 1 canonical cred per pair.
		// On origin/main, mergeCursorGuardMatcherEntry only ADDS the canonical without
		// removing legacy — so this check would see 2 entries (legacy + canonical) and fail.
		for _, matcher := range []string{"Read", "Write"} {
			if n := countFlatEntries(pre, guardCredentialCmdPSPOSIX, matcher); n != 1 {
				t.Errorf("ArmA: preToolUse/%s cred guard count want 1, got %d", matcher, n)
			}
			if n := countFlatEntries(post, guardCredentialCmdPSPOSIX, matcher); n != 1 {
				t.Errorf("ArmA: postToolUse/%s cred guard count want 1, got %d", matcher, n)
			}
		}
		// Legacy forms must be gone from pairs too (not just from shell events).
		if n := countFlatEntries(pre, legacyBareCredGuardCmd, "Read"); n != 0 {
			t.Errorf("ArmA: legacy bareCredGuard must be removed from preToolUse/Read, got %d", n)
		}
		if n := countFlatEntries(post, legacyD11RevCredentialCmdPSPOSIX, "Read"); n != 0 {
			t.Errorf("ArmA: legacy D11Rev must be removed from postToolUse/Read, got %d", n)
		}
	})

	t.Run("ArmB_GlobalCredInstalled", func(t *testing.T) {
		home := dedupFixtureHome(t)
		writeGlobalCursorCred(t, home) // D15: seed proves code no longer reads this

		dir := t.TempDir()
		// Seed: two legacy cred forms + user hook + legacy git guard in beforeShellExecution;
		// legacy cred entries in Read/Write pairs.
		cursorWriteHooksJSON(t, dir, map[string]interface{}{
			"version": 1,
			"hooks": map[string]interface{}{
				"beforeShellExecution": []interface{}{
					map[string]interface{}{"command": userShellHook},
					map[string]interface{}{"command": legacyD2CredentialCmdPSPOSIX},
					map[string]interface{}{"command": legacyD11CredentialCmdPSPOSIX},
					map[string]interface{}{"command": legacyD2GitBranchCmdPSPOSIX},
				},
				"afterShellExecution": []interface{}{
					map[string]interface{}{"command": legacyBareCredGuardCmd},
					map[string]interface{}{"command": legacyD11RevCredentialCmdPSPOSIX},
				},
				"preToolUse": []interface{}{
					map[string]interface{}{"command": legacyBareCredGuardCmd, "matcher": "Read"},
					map[string]interface{}{"command": legacyD11CredentialCmdPSPOSIX, "matcher": "Read"},
				},
				"postToolUse": []interface{}{
					map[string]interface{}{"command": legacyD2CredentialCmdPSPOSIX, "matcher": "Read"},
				},
			},
		})

		if err := InjectCursorHooks(dir); err != nil {
			t.Fatalf("InjectCursorHooks: %v", err)
		}
		hooks := cursorReadHooksJSON(t, dir)

		before, _ := hooks["beforeShellExecution"].([]interface{})

		// D15: cred guard PRESENT in beforeShellExecution (always emitted).
		if n := countFlatEntries(before, guardCredentialCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmB(D15): beforeShellExecution cred guard count want 1, got %d", n)
		}
		// All legacy cred forms must be gone (migrated to canonical).
		for _, legacyCmd := range []string{legacyD2CredentialCmdPSPOSIX, legacyD11CredentialCmdPSPOSIX, legacyBareCredGuardCmd} {
			if n := countFlatEntries(before, legacyCmd, ""); n != 0 {
				t.Errorf("ArmB(D15): legacy cred %q must be removed from beforeShellExecution, got %d", legacyCmd, n)
			}
		}
		// git-branch guard present.
		if n := countFlatEntries(before, guardGitBranchCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmB(D15): beforeShellExecution git guard must be present, got %d", n)
		}
		// User hook must survive.
		if n := countFlatEntries(before, userShellHook, ""); n != 1 {
			t.Errorf("ArmB(D15): user hook must survive in beforeShellExecution, got %d", n)
		}
		// afterShellExecution: D15 cred guard PRESENT (always emitted).
		afterArr, _ := hooks["afterShellExecution"].([]interface{})
		if n := countFlatEntries(afterArr, guardCredentialCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmB(D15): afterShellExecution cred guard count want 1, got %d", n)
		}
		for _, legacyCmd := range []string{legacyBareCredGuardCmd, legacyD11RevCredentialCmdPSPOSIX} {
			if n := countFlatEntries(afterArr, legacyCmd, ""); n != 0 {
				t.Errorf("ArmB(D15): legacy cred %q must be removed from afterShellExecution, got %d", legacyCmd, n)
			}
		}

		// Read/Write pairs: present (unchanged by D15).
		pre, _ := hooks["preToolUse"].([]interface{})
		post, _ := hooks["postToolUse"].([]interface{})
		for _, matcher := range []string{"Read", "Write"} {
			if n := countFlatEntries(pre, guardCredentialCmdPSPOSIX, matcher); n != 1 {
				t.Errorf("ArmB(D15): preToolUse/%s cred guard must be present, got %d", matcher, n)
			}
			if n := countFlatEntries(post, guardCredentialCmdPSPOSIX, matcher); n != 1 {
				t.Errorf("ArmB(D15): postToolUse/%s cred guard must be present, got %d", matcher, n)
			}
		}
	})

	t.Run("ArmC_GlobalGitBranchInstalled", func(t *testing.T) {
		home := dedupFixtureHome(t)
		writeGlobalCursorGitBranch(t, home) // D15: seed proves code no longer reads this

		dir := t.TempDir()
		// Seed: legacy git guard + legacy cred guard in beforeShellExecution.
		cursorWriteHooksJSON(t, dir, map[string]interface{}{
			"version": 1,
			"hooks": map[string]interface{}{
				"beforeShellExecution": []interface{}{
					map[string]interface{}{"command": legacyD2GitBranchCmdPSPOSIX},
					map[string]interface{}{"command": legacyD11RevGitBranchCmdPSPOSIX},
					map[string]interface{}{"command": legacyD2CredentialCmdPSPOSIX},
				},
				"preToolUse": []interface{}{
					map[string]interface{}{"command": legacyBareCredGuardCmd, "matcher": "Write"},
				},
			},
		})

		if err := InjectCursorHooks(dir); err != nil {
			t.Fatalf("InjectCursorHooks: %v", err)
		}
		hooks := cursorReadHooksJSON(t, dir)

		before, _ := hooks["beforeShellExecution"].([]interface{})

		// D15: git-branch guard PRESENT (always emitted); cred guard also present.
		if n := countFlatEntries(before, guardGitBranchCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmC(D15): beforeShellExecution git guard count want 1, got %d", n)
		}
		for _, legacyGit := range []string{legacyD2GitBranchCmdPSPOSIX, legacyD11RevGitBranchCmdPSPOSIX} {
			if n := countFlatEntries(before, legacyGit, ""); n != 0 {
				t.Errorf("ArmC(D15): legacy git %q must be removed (migrated to canonical), got %d", legacyGit, n)
			}
		}
		if n := countFlatEntries(before, guardCredentialCmdPSPOSIX, ""); n != 1 {
			t.Errorf("ArmC(D15): beforeShellExecution cred guard count want 1, got %d", n)
		}

		// Read/Write pairs: present.
		pre, _ := hooks["preToolUse"].([]interface{})
		post, _ := hooks["postToolUse"].([]interface{})
		for _, matcher := range []string{"Read", "Write"} {
			if n := countFlatEntries(pre, guardCredentialCmdPSPOSIX, matcher); n != 1 {
				t.Errorf("ArmC(D15): preToolUse/%s cred guard must be present, got %d", matcher, n)
			}
			if n := countFlatEntries(post, guardCredentialCmdPSPOSIX, matcher); n != 1 {
				t.Errorf("ArmC(D15): postToolUse/%s cred guard must be present, got %d", matcher, n)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// AC7 — idempotency
// ---------------------------------------------------------------------------

// TestCursorGuardMigrateDedup_Idempotent asserts that running InjectCursorHooks
// twice on the same directory produces byte-for-byte identical JSON output.
//
// Reconciliação: afirma que migrateDedupGuardInFlatArray é idempotente — se a
// função adicionasse uma entrada extra em vez de detectar a canônica já existente,
// o segundo resultado seria maior que o primeiro e este teste falharia.
func TestCursorGuardMigrateDedup_Idempotent(t *testing.T) {
	home := dedupFixtureHome(t)
	_ = home // no global guards

	dir := t.TempDir()
	path := filepath.Join(dir, ".cursor", "hooks.json")

	// First injection.
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks (run 1): %v", err)
	}
	after1, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after run 1: %v", err)
	}

	// Second injection on the already-updated file.
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks (run 2): %v", err)
	}
	after2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after run 2: %v", err)
	}

	if !bytes.Equal(after1, after2) {
		// For debugging, unmarshal and re-marshal to get a clean diff.
		var j1, j2 interface{}
		_ = json.Unmarshal(after1, &j1)
		_ = json.Unmarshal(after2, &j2)
		b1, _ := json.MarshalIndent(j1, "", "  ")
		b2, _ := json.MarshalIndent(j2, "", "  ")
		t.Errorf("AC7: InjectCursorHooks is not idempotent.\nAfter run 1:\n%s\n\nAfter run 2:\n%s",
			b1, b2)
	}
}

// ---------------------------------------------------------------------------
// AC7-legacy — idempotency starting from a legacy-generation file
// ---------------------------------------------------------------------------

// TestCursorGuardMigrateDedup_IdempotentFromLegacy starts from a file that
// contains all 4 legacy forms per Read/Write pair and in shell events
// (simulating issue #599 where guard ran 5× per Read/Write) and asserts that
// after InjectCursorHooks:
//  1. Exactly ONE canonical cred entry exists per (event, matcher) pair — all 4
//     Cursor pairs (preToolUse/Read, preToolUse/Write, postToolUse/Read, postToolUse/Write)
//     plus both shell events.
//  2. A second run produces byte-for-byte identical output.
//
// Reconciliação: afirma que a migração de 5 gerações (4 legado + 1 canônica) em todos
// os 4 pares converge para exatamente 1 entrada canônica, e a segunda execução não
// produz entradas adicionais — se migrateDedupGuardInFlatArray acrescentasse uma entrada
// em vez de detectar a canônica já presente, o contador subiria para 2 neste teste.
func TestCursorGuardMigrateDedup_IdempotentFromLegacy(t *testing.T) {
	home := dedupFixtureHome(t)
	_ = home // no global guards

	dir := t.TempDir()

	// The 4 legacy forms used in issue #599 scenario.
	legacyForms := []string{
		legacyBareCredGuardCmd,
		legacyD2CredentialCmdPSPOSIX,
		legacyD11CredentialCmdPSPOSIX,
		legacyD11RevCredentialCmdPSPOSIX,
	}

	buildPairEntries := func(matcher string) []interface{} {
		var entries []interface{}
		for _, cmd := range legacyForms {
			entries = append(entries, map[string]interface{}{"command": cmd, "matcher": matcher})
		}
		// Also add the canonical form (the 5th entry in the #599 scenario).
		entries = append(entries, map[string]interface{}{"command": guardCredentialCmdPSPOSIX, "matcher": matcher})
		return entries
	}

	buildShellEntries := func() []interface{} {
		var entries []interface{}
		for _, cmd := range legacyForms {
			entries = append(entries, map[string]interface{}{"command": cmd})
		}
		entries = append(entries, map[string]interface{}{"command": guardCredentialCmdPSPOSIX})
		return entries
	}

	// Seed ALL 4 Read/Write pairs PLUS shell events with 5 entries each.
	cursorWriteHooksJSON(t, dir, map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse":           buildPairEntries("Read"),
			"postToolUse":          buildPairEntries("Read"),
			"beforeShellExecution": buildShellEntries(),
			"afterShellExecution":  buildShellEntries(),
		},
	})

	// First injection: should collapse all 5 entries to 1 per pair.
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks (run 1): %v", err)
	}

	hooks := cursorReadHooksJSON(t, dir)

	// Verify all 4 (event, matcher) pairs and both shell events have exactly 1 canonical entry.
	pairsToCheck := []struct {
		event   string
		matcher string
	}{
		{"preToolUse", "Read"},
		{"preToolUse", "Write"},
		{"postToolUse", "Read"},
		{"postToolUse", "Write"},
	}
	for _, p := range pairsToCheck {
		arr, _ := hooks[p.event].([]interface{})
		if n := countFlatEntries(arr, guardCredentialCmdPSPOSIX, p.matcher); n != 1 {
			t.Errorf("AC7-legacy: %s/%s canonical cred count want 1, got %d", p.event, p.matcher, n)
		}
		for _, legacyCmd := range legacyForms {
			if n := countFlatEntries(arr, legacyCmd, p.matcher); n != 0 {
				t.Errorf("AC7-legacy: %s/%s legacy %q must be removed, count=%d", p.event, p.matcher, legacyCmd, n)
			}
		}
	}

	// Shell events: exactly 1 canonical cred entry each.
	for _, event := range []string{"beforeShellExecution", "afterShellExecution"} {
		arr, _ := hooks[event].([]interface{})
		if n := countFlatEntries(arr, guardCredentialCmdPSPOSIX, ""); n != 1 {
			t.Errorf("AC7-legacy: %s canonical cred count want 1, got %d", event, n)
		}
		for _, legacyCmd := range legacyForms {
			if n := countFlatEntries(arr, legacyCmd, ""); n != 0 {
				t.Errorf("AC7-legacy: %s legacy %q must be removed, count=%d", event, legacyCmd, n)
			}
		}
	}

	// Second injection: must be idempotent.
	path := filepath.Join(dir, ".cursor", "hooks.json")
	after1, _ := os.ReadFile(path)
	if err := InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks (run 2): %v", err)
	}
	after2, _ := os.ReadFile(path)
	if !bytes.Equal(after1, after2) {
		var j1, j2 interface{}
		_ = json.Unmarshal(after1, &j1)
		_ = json.Unmarshal(after2, &j2)
		b1, _ := json.MarshalIndent(j1, "", "  ")
		b2, _ := json.MarshalIndent(j2, "", "  ")
		t.Errorf("AC7-legacy: second run is not idempotent.\nRun1:\n%s\nRun2:\n%s", b1, b2)
	}
}
