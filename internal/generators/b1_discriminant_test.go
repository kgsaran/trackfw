package generators

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// b1Spec parameterizes the B-1 real-structure discriminant helper for a given agent.
type b1Spec struct {
	injectFn      func(string) error
	configPath    func(dir string) string // dir → absolute config file path
	preEvent      string
	postEvent     string   // event that has guardCred on all matchers (not git)
	shellMatcher  string   // matcher that carries BOTH git and cred in preEvent
	extraMatchers []string // matchers that carry cred only (in both events)
}

// helperB1CombinedNoDuplicate is the shared B-1 discriminant implementation.
//
// It builds a fixture that reproduces the real origin/main shape:
//   - preEvent: shellMatcher group has [legacyGit, legacyCred]; each extraMatcher has [legacyCred]
//   - postEvent: every matcher has [legacyCred]
//
// After each inject it asserts three invariants (the third is the discriminant):
//  1. each matcher group appears exactly once per event (no duplicate groups);
//  2. exactly 1 copy of guardCredentialCmdPSPOSIX within every guarded group;
//  3. exactly 1 copy of guardGitBranchCmdPSPOSIX within shellMatcher's preEvent group.
//
// Invariant 1 is what the old TestInjectCodexHooks_LegacyMigrationNoDuplicate
// checked; invariants 2 and 3 are the discriminants that fail with ML-1A code and
// pass with the ML-1B migration-before-merge fix.
//
// Conclusion for each caller: migration-before-merge (ML-1B, B-1 fix) rewrites the
// legacy forms to the canonical emulate form BEFORE mergeClaudeHookArray runs, so the
// merge detects the command already present and does not append a second copy.
func helperB1CombinedNoDuplicate(t *testing.T, spec b1Spec, legacyGitCmd, legacyCredCmd string) {
	t.Helper()

	dir := t.TempDir()
	cfgPath := spec.configPath(dir)

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	mkEntry := func(matcher string, cmds ...string) map[string]interface{} {
		var hooks []interface{}
		for _, c := range cmds {
			hooks = append(hooks, map[string]interface{}{"type": "command", "command": c})
		}
		return map[string]interface{}{"matcher": matcher, "hooks": hooks}
	}

	// preEvent: shell group has both guards; extra groups have cred only.
	var preEntries []interface{}
	preEntries = append(preEntries, mkEntry(spec.shellMatcher, legacyGitCmd, legacyCredCmd))
	for _, m := range spec.extraMatchers {
		preEntries = append(preEntries, mkEntry(m, legacyCredCmd))
	}
	// postEvent: all groups have cred only.
	var postEntries []interface{}
	postEntries = append(postEntries, mkEntry(spec.shellMatcher, legacyCredCmd))
	for _, m := range spec.extraMatchers {
		postEntries = append(postEntries, mkEntry(m, legacyCredCmd))
	}

	initial := map[string]interface{}{
		"hooks": map[string]interface{}{
			spec.preEvent:  preEntries,
			spec.postEvent: postEntries,
		},
	}

	raw, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(cfgPath, raw, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	allMatchers := append([]string{spec.shellMatcher}, spec.extraMatchers...)

	assertInvariants := func(label string) {
		t.Helper()
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("%s: read config: %v", label, err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("%s: parse: %v", label, err)
		}
		hooksMap, _ := result["hooks"].(map[string]interface{})

		for _, event := range []string{spec.preEvent, spec.postEvent} {
			arr, _ := hooksMap[event].([]interface{})
			for _, matcher := range allMatchers {
				// 1. Group count.
				groupCount := 0
				var innerHooks []interface{}
				for _, item := range arr {
					group, _ := item.(map[string]interface{})
					if group["matcher"] == matcher {
						groupCount++
						innerHooks, _ = group["hooks"].([]interface{})
					}
				}
				if groupCount != 1 {
					t.Errorf("%s %s[%q]: expected 1 group, got %d (B-1 duplicate group)", label, event, matcher, groupCount)
				}
				// 2. Credential-guard command count within group.
				credCount := 0
				for _, h := range innerHooks {
					hm, _ := h.(map[string]interface{})
					if hm["command"] == guardCredentialCmdPSPOSIX {
						credCount++
					}
				}
				if credCount != 1 {
					t.Errorf("%s %s[%q]: %d copies of credential-guard command (want exactly 1) — B-1 duplicate", label, event, matcher, credCount)
				}
				// 3. Git-branch-guard command count — discriminant, preEvent shellMatcher only.
				if event == spec.preEvent && matcher == spec.shellMatcher {
					gitCount := 0
					for _, h := range innerHooks {
						hm, _ := h.(map[string]interface{})
						if hm["command"] == guardGitBranchCmdPSPOSIX {
							gitCount++
						}
					}
					if gitCount != 1 {
						t.Errorf("%s %s[%q]: %d copies of git-branch-guard command (want exactly 1) — B-1 duplicate", label, event, matcher, gitCount)
					}
				}
			}
		}
	}

	if err := spec.injectFn(dir); err != nil {
		t.Fatalf("first inject: %v", err)
	}
	assertInvariants("after 1st inject")

	if err := spec.injectFn(dir); err != nil {
		t.Fatalf("second inject: %v", err)
	}
	assertInvariants("after 2nd inject (idempotency)")
}

// codexB1Spec is the b1Spec for InjectCodexHooks.
var codexB1Spec = b1Spec{
	injectFn:      InjectCodexHooks,
	configPath:    func(dir string) string { return filepath.Join(dir, ".codex", "hooks.json") },
	preEvent:      "PreToolUse",
	postEvent:     "PostToolUse",
	shellMatcher:  "Bash",
	extraMatchers: []string{"apply_patch"},
}

// geminiB1Spec is the b1Spec for InjectGeminiHooks.
var geminiB1Spec = b1Spec{
	injectFn:      InjectGeminiHooks,
	configPath:    func(dir string) string { return filepath.Join(dir, ".gemini", "settings.json") },
	preEvent:      "BeforeTool",
	postEvent:     "AfterTool",
	shellMatcher:  "run_shell_command",
	extraMatchers: []string{"read_file|read_many_files", "write_file|replace"},
}

// TestInjectCodexHooks_D11RevNoDuplicate is the B-1 discriminant test for
// REQ-2026-10-20 using the D11Rev legacy form and the InjectCodexHooks path.
//
// The fixture reproduces the real origin/main .codex/hooks.json shape: a single
// "Bash" PreToolUse group containing BOTH the git-branch guard and the credential
// guard (in the legacyD11Rev form), plus an "apply_patch" group with credential only.
// The test counts commands within each group — NOT groups — so that a duplicate
// introduced by ML-1A (two identical credential-guard commands inside the same Bash
// group) is visible as a FAIL.
//
// Conclusion: InjectCodexHooks with migration-before-merge (ML-1B, B-1 fix) rewrites
// the legacyD11Rev forms to the canonical emulate form BEFORE mergeClaudeHookArray
// runs, so the merge detects the command already present and does not append a second
// copy. TestInjectCodexHooks_LegacyMigrationNoDuplicate was updated in the same ML to
// use the same combined-group fixture and now discriminates the same failure.
func TestInjectCodexHooks_D11RevNoDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	helperB1CombinedNoDuplicate(t, codexB1Spec,
		legacyD11RevGitBranchCmdPSPOSIX, legacyD11RevCredentialCmdPSPOSIX)
}

// TestInjectGeminiHooks_D11RevNoDuplicate is the Gemini counterpart of
// TestInjectCodexHooks_D11RevNoDuplicate.
//
// The fixture reproduces the real origin/main .gemini/settings.json shape:
// a single "run_shell_command" BeforeTool group with BOTH git-branch and
// credential guards in legacyD11Rev form, plus "read_file|read_many_files" and
// "write_file|replace" groups with credential only.
//
// Conclusion: InjectGeminiHooks with migration-before-merge (ML-1B, B-1 fix)
// rewrites the legacyD11Rev forms before mergeClaudeHookArray runs, preventing
// duplicate credential-guard commands within the run_shell_command group.
func TestInjectGeminiHooks_D11RevNoDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	helperB1CombinedNoDuplicate(t, geminiB1Spec,
		legacyD11RevGitBranchCmdPSPOSIX, legacyD11RevCredentialCmdPSPOSIX)
}
