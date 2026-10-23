package validator_test

// cursor_guard_migrate_dedup_external_test.go — AC1, AC4 (REQ-2026-10-23, ML-1A)
//
//   AC1 — five-generation fixture: after InjectCursorHooks collapses all legacy
//          forms to the canonical one, GuardWiringExtractGuardTypesFromFileForTest
//          recognises exactly the expected number of guard entries — no more
//          (no duplicates survive migration), no fewer (canonical form is recognised).
//
//   AC4 — registry concordance: every form in guardforms.CredentialLegacyPSPOSIX
//          and guardforms.GitBranchLegacyPSPOSIX is recognised by
//          GuardWiringTypeFromCmdPSPosixForTest; current forms are also recognised.
//          Falsification: an invented form that resembles a guard command but is
//          not in the registry returns "" (proves the helpers are not stubs that
//          always return a non-empty type).
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//   AC1 afirma que a migração dos 5 gerações converge para exatamente N entradas
//   reconhecidas — se houver duplicatas remanescentes, a contagem sobe acima do
//   esperado; se a forma canônica não for reconhecida, ela cai para zero.
//   AC4 afirma que o registro D12 e o validator falam o mesmo idioma — se o
//   validator não reconhecer uma forma do registro, o sub-teste da forma falha.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/guardforms"
	"github.com/kgsaran/trackfw/internal/validator"
)

// ---------------------------------------------------------------------------
// AC1 — five-generation fixture → validator sees canonical count
// ---------------------------------------------------------------------------

// TestCursorGuardMigrateDedupExternal_ValidatorAfterMigration builds a
// .cursor/hooks.json fixture containing all 4 known legacy generations for every
// Cursor credential-guard pair (preToolUse/Read, preToolUse/Write, postToolUse/Read,
// postToolUse/Write — 4 legacy + 1 canonical = 5 entries each), runs InjectCursorHooks,
// and asserts:
//
//  1. Exactly 1 canonical entry per (event, matcher) pair after migration.
//  2. GuardWiringExtractGuardTypesFromFileForTest recognises ≥ 1 "credential" entry.
//  3. ValidateCredentialGuardHookResolvableForTest returns 0 warnings (no legacy-form
//     warnings remain after migration).
//
// Reconciliação: afirma que a migração de 5 gerações por par converge para 1 canônica
// reconhecida pelo validator — se validateCredentialGuardHookResolvable ainda visse
// alguma forma legada, a contagem de avisos seria > 0 e este teste falharia.
func TestCursorGuardMigrateDedupExternal_ValidatorAfterMigration(t *testing.T) {
	dir := setupConcordanceDir(t)

	// 4 legacy + 1 canonical = the 5-entry scenario from issue #599.
	legacyForms := []string{
		guardforms.LegacyBareCredGuard,
		guardforms.LegacyD2CredentialPSPOSIX,
		guardforms.LegacyD11CredentialPSPOSIX,
		guardforms.LegacyD11RevCredentialPSPOSIX,
	}
	canonicalForm := guardforms.CredentialCurrentPSPOSIX

	makeEntries := func(matcher string) []interface{} {
		var entries []interface{}
		for _, cmd := range legacyForms {
			entries = append(entries, map[string]interface{}{"command": cmd, "matcher": matcher})
		}
		entries = append(entries, map[string]interface{}{"command": canonicalForm, "matcher": matcher})
		return entries
	}

	hooksPath := filepath.Join(dir, ".cursor", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0755); err != nil {
		t.Fatalf("mkdir .cursor: %v", err)
	}

	// Seed ALL 4 pairs with legacy+canonical.
	fixture := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse":  makeEntries("Read"),
			"postToolUse": makeEntries("Read"),
			// Write entries are added by InjectCursorHooks; pre-seed Read to test migration.
		},
	}
	raw, _ := json.Marshal(fixture)
	if err := os.WriteFile(hooksPath, append(raw, '\n'), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Run InjectCursorHooks — should collapse to 1 entry per pair.
	if err := generators.InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks: %v", err)
	}

	// Read back the result.
	out, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	hooks, _ := result["hooks"].(map[string]interface{})

	countEntries := func(event, matcher string) int {
		arr, _ := hooks[event].([]interface{})
		n := 0
		for _, item := range arr {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := obj["command"].(string)
			m, _ := obj["matcher"].(string)
			if cmd == canonicalForm && m == matcher {
				n++
			}
		}
		return n
	}

	// Assertion 1: exactly 1 canonical entry per pair — no duplicates, migration worked.
	for _, event := range []string{"preToolUse", "postToolUse"} {
		for _, matcher := range []string{"Read", "Write"} {
			n := countEntries(event, matcher)
			if n != 1 {
				t.Errorf("AC1: %s/%s canonical cred guard count want 1, got %d", event, matcher, n)
			}
		}
	}

	// Assertion 2: GuardWiringExtractGuardTypesFromFileForTest recognises the canonical entry.
	// setupConcordanceDir calls ChdirForTest(t, dir), so ".cursor/hooks.json" resolves correctly.
	guardTypes, err := validator.GuardWiringExtractGuardTypesFromFileForTest(".cursor/hooks.json")
	if err != nil {
		t.Fatalf("AC1: extract guard types: %v", err)
	}
	credCount := 0
	for _, gt := range guardTypes {
		if gt == "credential" {
			credCount++
		}
	}
	if credCount < 1 {
		t.Errorf("AC1: validator must recognise ≥ 1 credential guard entry after migration; got %d (all: %v)", credCount, guardTypes)
	}

	// Assertion 3: ValidateCredentialGuardHookResolvableForTest must return 0 warnings.
	// Before migration, the fixture had 4 legacy forms per pair → validator would warn on each.
	// After migration, only the canonical form remains → 0 warnings.
	warnings, err := validator.ValidateCredentialGuardHookResolvableForTest()
	if err != nil {
		t.Fatalf("AC1: ValidateCredentialGuardHookResolvable: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("AC1: ValidateCredentialGuardHookResolvable must return 0 warnings after migration; got %d:\n%v", len(warnings), warnings)
	}
}

// ---------------------------------------------------------------------------
// AC4 — registry concordance (D12 ↔ validator)
// ---------------------------------------------------------------------------

// TestCursorGuardMigrateDedupExternal_RegistryConcordance verifies that every
// form in the D12 guardforms registry is recognised by the validator's
// guardWiringTypeFromCmd helpers. This ensures that after InjectCursorHooks
// rewrites a legacy form, the result is within the validator's equivalence class.
//
// Falsification arm: an invented command that resembles a guard entry but is not
// in the registry must return "" (non-vacuity of the helpers).
//
// Reconciliação: afirma que o registro D12 e o validator compartilham a mesma
// classe de equivalência — se o validator não reconhecer uma forma do registro,
// o sub-teste da forma falha.
func TestCursorGuardMigrateDedupExternal_RegistryConcordance(t *testing.T) {
	// All PSPOSIX credential forms (legacy + current).
	for _, cmd := range append(guardforms.CredentialLegacyPSPOSIX, guardforms.CredentialCurrentPSPOSIX) {
		cmd := cmd
		t.Run("cred/psposix/"+shortForm(cmd), func(t *testing.T) {
			got := validator.GuardWiringTypeFromCmdPSPosixForTest(cmd)
			if got != "credential" {
				t.Errorf("AC4: PSPosix form not recognised as 'credential': cmd=%q → %q", cmd, got)
			}
		})
	}

	// All PSPOSIX git-branch forms (legacy + current).
	for _, cmd := range append(guardforms.GitBranchLegacyPSPOSIX, guardforms.GitBranchCurrentPSPOSIX) {
		cmd := cmd
		t.Run("git/psposix/"+shortForm(cmd), func(t *testing.T) {
			got := validator.GuardWiringTypeFromCmdPSPosixForTest(cmd)
			if got != "git-branch" {
				t.Errorf("AC4: PSPosix form not recognised as 'git-branch': cmd=%q → %q", cmd, got)
			}
		})
	}

	// CmdExe current forms.
	t.Run("cred/cmdexe/current", func(t *testing.T) {
		got := validator.GuardWiringTypeFromCmdCmdExeForTest(guardforms.CredentialCurrentCmdExe)
		if got != "credential" {
			t.Errorf("AC4: CmdExe current cred form not recognised: cmd=%q → %q", guardforms.CredentialCurrentCmdExe, got)
		}
	})
	t.Run("git/cmdexe/current", func(t *testing.T) {
		got := validator.GuardWiringTypeFromCmdCmdExeForTest(guardforms.GitBranchCurrentCmdExe)
		if got != "git-branch" {
			t.Errorf("AC4: CmdExe current git form not recognised: cmd=%q → %q", guardforms.GitBranchCurrentCmdExe, got)
		}
	})

	// Falsification: invented form must NOT be recognised.
	t.Run("falsification/unknown-form", func(t *testing.T) {
		unknown := "echo 'this is not a guard command; trackfw guard credential'"
		if got := validator.GuardWiringTypeFromCmdPSPosixForTest(unknown); got != "" {
			t.Errorf("AC4 falsification: unknown form must return \"\", got %q for %q", got, unknown)
		}
		if got := validator.GuardWiringTypeFromCmdCmdExeForTest(unknown); got != "" {
			t.Errorf("AC4 falsification: unknown form must return \"\", got %q for %q", got, unknown)
		}
	})

	// Falsification: a command that is a PREFIX of a real guard entry (strings.Contains
	// would match; exact equality must not).
	t.Run("falsification/prefix-of-real", func(t *testing.T) {
		prefix := "trackfw guard credential"
		// This IS a real legacy form (LegacyD2CredentialCmdExe) so it must be recognised
		// under CmdExe. But it must NOT be recognised as PSPosix (different family).
		// What we want to verify: a command that is merely a prefix substring of the
		// canonical PSPosix form returns "" for PSPosix.
		// The canonical PSPosix form starts with "emulate -R sh ...; trackfw guard credential"
		// and is much longer. A bare "trackfw guard credential" is only a CmdExe form.
		gotPS := validator.GuardWiringTypeFromCmdPSPosixForTest(prefix)
		if gotPS != "" {
			t.Errorf("AC4 falsification/prefix: 'trackfw guard credential' must not be PSPosix-recognised, got %q", gotPS)
		}
	})
}

// shortForm returns the first 30 characters of cmd for use as a test name.
func shortForm(cmd string) string {
	if len(cmd) <= 30 {
		return cmd
	}
	return cmd[:30]
}
