package generators

// guard_registry_sites_ac4_test.go — AC4 cross-site registry coverage (REQ-2026-10-23, ML-1D)
//
// Table-driven test covering every (site, event, matcher) pair that calls a
// D13 dedup helper, across both project-scope Inject* functions and global
// harness UpdateHarness targets.  For each row in the table, and for each
// form in the registry list named by that row, the test:
//
//  1. Seeds a fixture file with the legacy form AND the canonical form in the
//     same (event, matcher) pair.
//  2. Calls the real site function (Inject* or UpdateHarness).
//  3. Asserts exactly 1 canonical entry and 0 occurrences of the seeded
//     legacy form in the output.
//
// The list field in every row references the actual guardforms.*Legacy*
// variable — never a literal copy — so that any future form added to the
// registry automatically falls under these tests.
//
// Reconciliação (CLAUDE.md Regra Dura de Reconciliação):
//   Each subtest asserts: for the exact command string of the seeded legacy
//   form, countNestedHarness / countFlatCommandEntries / countFlatBashEntries
//   returns 0; for the canonical string, returns 1.  The test fails if the
//   site's dedup call is narrowed to a hardcoded subset that omits the seeded
//   form (e.g. list[1:]).
//
// Falsification note:
//   Temporarily add "trackfw guard credential # fictícia" to
//   guardforms.CredentialLegacyPSPOSIX and run the test.  Because every site
//   in this table references the variable directly (not a hardcoded subset),
//   the dedup helper automatically includes the new form and all subtests
//   PASS — which IS the expected outcome.  The test is sensitive to narrowed
//   lists, not to the addition of a form that is already wired in by
//   reference.  See memory note "Migrar o leitor de uma regra esvazia o
//   falsify" for the precise mechanism.
//
// Omissions (by design, with reason):
//   • windsurf-git-branch-guard: no global harness target (open gap per
//     ML-1B, REQ-2026-10-06 — Windsurf global hook file is
//     ~/.codeium/windsurf/hooks.json but a git-branch variant for that path
//     was not confirmed against official documentation).
//   • Kiro harness: harnessCredentialGuardTargetKiro performs a total
//     overwrite (writes a dedicated ~/.kiro/hooks/ file, not a merge into a
//     shared settings file), so there is no legacy dedup pass to test.

import (
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/guardforms"
)

// ac4NestedSeed writes a nested-format fixture (Claude/Codex/Gemini/AmazonQ)
// with a legacy entry and a canonical entry under the given event × matcher pair.
func ac4NestedSeed(t *testing.T, path, event, matcher, legacy, canonical string) {
	t.Helper()
	makeEntry := func(cmd string) interface{} {
		return map[string]interface{}{
			"matcher": matcher,
			"hooks":   []interface{}{map[string]interface{}{"type": "command", "command": cmd}},
		}
	}
	writeJSONFile(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			event: []interface{}{makeEntry(legacy), makeEntry(canonical)},
		},
	})
}

// ac4FlatSeed writes a flat-format fixture (Cursor/Windsurf) with a legacy
// entry and a canonical entry under the given event.  Entries have no matcher
// field (matched by countFlatCommandEntries with matcher="").
func ac4FlatSeed(t *testing.T, path, event, legacy, canonical string) {
	t.Helper()
	writeJSONFile(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			event: []interface{}{
				map[string]interface{}{"command": legacy},
				map[string]interface{}{"command": canonical},
			},
		},
	})
}

// ac4BashSeed writes a Copilot flat-bash-format fixture with a legacy entry
// and a canonical entry under the given event.
func ac4BashSeed(t *testing.T, path, event, legacy, canonical string) {
	t.Helper()
	makeEntry := func(cmd string) interface{} {
		return map[string]interface{}{
			"type": "command", "matcher": "bash", "bash": cmd,
			"cwd": ".", "timeoutSec": float64(10),
		}
	}
	writeJSONFile(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			event: []interface{}{makeEntry(legacy), makeEntry(canonical)},
		},
	})
}

// ac4SanitizeLabel produces a short label for a form string suitable for use
// as a t.Run component (replaces "/" with "|" to prevent accidental sub-test
// nesting; truncates to 30 chars).
func ac4SanitizeLabel(form string) string {
	const maxLen = 30
	s := form
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			out[i] = '|'
		} else {
			out[i] = s[i]
		}
	}
	return string(out)
}

// TestAC4_RegistrySites is the cross-site registry coverage test described
// in the file-level comment.
func TestAC4_RegistrySites(t *testing.T) {
	type row struct {
		site      string
		event     string
		matcher   string
		canonical string
		list      []string
		// run seeds the fixture, calls the site function, and returns the
		// unmarshalled root of the output file.
		run func(t *testing.T, event, matcher, legacy, canonical string) map[string]interface{}
		// count returns the number of occurrences of cmd for the given
		// event×matcher in root.
		count func(root map[string]interface{}, event, matcher, cmd string) int
	}

	// ── count function aliases ────────────────────────────────────────────────

	nestedCount := func(root map[string]interface{}, event, matcher, cmd string) int {
		return countNestedHarness(root, event, matcher, cmd)
	}
	flatCount := func(root map[string]interface{}, event, _, cmd string) int {
		return countFlatCommandEntries(root, event, cmd, "")
	}
	bashCount := func(root map[string]interface{}, event, _, cmd string) int {
		return countFlatBashEntries(root, event, cmd)
	}

	// ── run factory helpers ───────────────────────────────────────────────────

	// nestedInjectRun: set HOME → write nested fixture → inject → read output.
	nestedInjectRun := func(relPath string, inject func(dir string) error) func(*testing.T, string, string, string, string) map[string]interface{} {
		return func(t *testing.T, event, matcher, legacy, canonical string) map[string]interface{} {
			t.Helper()
			dir := t.TempDir()
			harnessHome(t) // sets HOME to a fresh TempDir so globalGuardInstalled* returns false
			path := filepath.Join(dir, relPath)
			ac4NestedSeed(t, path, event, matcher, legacy, canonical)
			if err := inject(dir); err != nil {
				t.Fatalf("inject: %v", err)
			}
			return readJSONFile(t, path)
		}
	}

	// flatInjectRun: same but seeds flat-format entries.
	flatInjectRun := func(relPath string, inject func(dir string) error) func(*testing.T, string, string, string, string) map[string]interface{} {
		return func(t *testing.T, event, _, legacy, canonical string) map[string]interface{} {
			t.Helper()
			dir := t.TempDir()
			harnessHome(t)
			path := filepath.Join(dir, relPath)
			ac4FlatSeed(t, path, event, legacy, canonical)
			if err := inject(dir); err != nil {
				t.Fatalf("inject: %v", err)
			}
			return readJSONFile(t, path)
		}
	}

	// nestedHarnessRun: set HOME → write nested fixture → UpdateHarness → read output.
	nestedHarnessRun := func(target, fileSuffix string) func(*testing.T, string, string, string, string) map[string]interface{} {
		return func(t *testing.T, event, matcher, legacy, canonical string) map[string]interface{} {
			t.Helper()
			home := harnessHome(t)
			path := filepath.Join(home, fileSuffix)
			ac4NestedSeed(t, path, event, matcher, legacy, canonical)
			if _, err := UpdateHarness(UpdateOptions{Targets: []string{target}, InstallMissing: true}); err != nil {
				t.Fatalf("UpdateHarness(%s): %v", target, err)
			}
			return readJSONFile(t, path)
		}
	}

	// flatHarnessRun: set HOME → write flat fixture → UpdateHarness → read output.
	flatHarnessRun := func(target, fileSuffix string) func(*testing.T, string, string, string, string) map[string]interface{} {
		return func(t *testing.T, event, _, legacy, canonical string) map[string]interface{} {
			t.Helper()
			home := harnessHome(t)
			path := filepath.Join(home, fileSuffix)
			ac4FlatSeed(t, path, event, legacy, canonical)
			if _, err := UpdateHarness(UpdateOptions{Targets: []string{target}, InstallMissing: true}); err != nil {
				t.Fatalf("UpdateHarness(%s): %v", target, err)
			}
			return readJSONFile(t, path)
		}
	}

	// bashHarnessRun: set HOME → write bash fixture → UpdateHarness → read output.
	bashHarnessRun := func(target, fileSuffix string) func(*testing.T, string, string, string, string) map[string]interface{} {
		return func(t *testing.T, event, _, legacy, canonical string) map[string]interface{} {
			t.Helper()
			home := harnessHome(t)
			path := filepath.Join(home, fileSuffix)
			ac4BashSeed(t, path, event, legacy, canonical)
			if _, err := UpdateHarness(UpdateOptions{Targets: []string{target}, InstallMissing: true}); err != nil {
				t.Fatalf("UpdateHarness(%s): %v", target, err)
			}
			return readJSONFile(t, path)
		}
	}

	// ── combined lists for AmazonQ (CmdExe forms + POSIX forms) ─────────────
	amazonQGitBranchList := append(
		append([]string{}, guardforms.GitBranchLegacyCmdExe...),
		guardforms.GitBranchLegacyPSPOSIX...,
	)
	amazonQCredentialList := append(
		append([]string{}, guardforms.CredentialLegacyCmdExe...),
		guardforms.CredentialLegacyPSPOSIX...,
	)

	rows := []row{
		// ── Project-scope inject sites ────────────────────────────────────────
		// InjectClaudeHooks — nested — .claude/settings.json
		{
			site: "InjectClaude-cred", event: "PreToolUse", matcher: claudeShellMatcher,
			canonical: guardCredentialCmdPSPOSIX, list: guardforms.CredentialLegacyPSPOSIX,
			run: nestedInjectRun(".claude/settings.json", InjectClaudeHooks), count: nestedCount,
		},
		{
			site: "InjectClaude-git", event: "PreToolUse", matcher: claudeShellMatcher,
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: nestedInjectRun(".claude/settings.json", InjectClaudeHooks), count: nestedCount,
		},
		// InjectCodexHooks — nested — .codex/hooks.json
		{
			site: "InjectCodex-cred", event: "PreToolUse", matcher: "Bash",
			canonical: guardCredentialCmdPSPOSIX, list: guardforms.CredentialLegacyPSPOSIX,
			run: nestedInjectRun(".codex/hooks.json", InjectCodexHooks), count: nestedCount,
		},
		{
			site: "InjectCodex-git", event: "PreToolUse", matcher: "Bash",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: nestedInjectRun(".codex/hooks.json", InjectCodexHooks), count: nestedCount,
		},
		// InjectGeminiHooks — nested — .gemini/settings.json
		{
			site: "InjectGemini-cred", event: "BeforeTool", matcher: "run_shell_command",
			canonical: guardCredentialCmdPSPOSIX, list: guardforms.CredentialLegacyPSPOSIX,
			run: nestedInjectRun(".gemini/settings.json", InjectGeminiHooks), count: nestedCount,
		},
		{
			site: "InjectGemini-git", event: "BeforeTool", matcher: "run_shell_command",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: nestedInjectRun(".gemini/settings.json", InjectGeminiHooks), count: nestedCount,
		},
		// InjectCursorHooks — flat, no matcher — .cursor/hooks.json
		{
			site: "InjectCursor-cred", event: "beforeShellExecution", matcher: "",
			canonical: guardCredentialCmdPSPOSIX, list: guardforms.CredentialLegacyPSPOSIX,
			run: flatInjectRun(".cursor/hooks.json", InjectCursorHooks), count: flatCount,
		},
		{
			site: "InjectCursor-git", event: "beforeShellExecution", matcher: "",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: flatInjectRun(".cursor/hooks.json", InjectCursorHooks), count: flatCount,
		},
		// InjectWindsurfHooks — flat (windsurf), no matcher — .windsurf/hooks.json
		{
			site: "InjectWindsurf-cred", event: "pre_run_command", matcher: "",
			canonical: guardCredentialCmdPSPOSIX, list: guardforms.CredentialLegacyPSPOSIX,
			run: flatInjectRun(".windsurf/hooks.json", InjectWindsurfHooks), count: flatCount,
		},
		{
			site: "InjectWindsurf-git", event: "pre_run_command", matcher: "",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: flatInjectRun(".windsurf/hooks.json", InjectWindsurfHooks), count: flatCount,
		},
		// InjectAmazonQHooks — nested — .amazonq/cli-agents/q_cli_default.json
		// credential: dedup covers CredentialLegacyCmdExe + CredentialLegacyPSPOSIX (ML-1E)
		// git-branch: dedup covers GitBranchLegacyCmdExe + GitBranchLegacyPSPOSIX
		{
			site: "InjectAmazonQ-cred", event: "preToolUse", matcher: "execute_bash",
			canonical: guardCredentialCmdCmdExe, list: amazonQCredentialList,
			run: nestedInjectRun(".amazonq/cli-agents/q_cli_default.json", InjectAmazonQHooks), count: nestedCount,
		},
		{
			site: "InjectAmazonQ-cred-fswrite", event: "preToolUse", matcher: "fs_write",
			canonical: guardCredentialCmdCmdExe, list: amazonQCredentialList,
			run: nestedInjectRun(".amazonq/cli-agents/q_cli_default.json", InjectAmazonQHooks), count: nestedCount,
		},
		{
			site: "InjectAmazonQ-git", event: "preToolUse", matcher: "execute_bash",
			canonical: guardGitBranchCmdCmdExe, list: amazonQGitBranchList,
			run: nestedInjectRun(".amazonq/cli-agents/q_cli_default.json", InjectAmazonQHooks), count: nestedCount,
		},

		// ── Global harness sites ──────────────────────────────────────────────
		// claude-credential-guard — nested — ~/.claude/settings.json
		{
			site: "HarnessClaude-cred", event: "PreToolUse", matcher: claudeShellMatcher,
			canonical: guardCredentialGlobalCmdPSPOSIX, list: guardforms.CredentialGlobalLegacyPSPOSIX,
			run: nestedHarnessRun("claude-credential-guard", ".claude/settings.json"), count: nestedCount,
		},
		// claude-git-branch-guard — nested — ~/.claude/settings.json
		{
			site: "HarnessClaude-git", event: "PreToolUse", matcher: claudeShellMatcher,
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: nestedHarnessRun("claude-git-branch-guard", ".claude/settings.json"), count: nestedCount,
		},
		// codex-credential-guard — nested — ~/.codex/hooks.json
		{
			site: "HarnessCodex-cred", event: "PreToolUse", matcher: "Bash",
			canonical: guardCredentialGlobalCmdPSPOSIX, list: guardforms.CredentialGlobalLegacyPSPOSIX,
			run: nestedHarnessRun("codex-credential-guard", ".codex/hooks.json"), count: nestedCount,
		},
		// codex-git-branch-guard — nested — ~/.codex/hooks.json
		{
			site: "HarnessCodex-git", event: "PreToolUse", matcher: "Bash",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: nestedHarnessRun("codex-git-branch-guard", ".codex/hooks.json"), count: nestedCount,
		},
		// gemini-credential-guard — nested — ~/.gemini/settings.json
		{
			site: "HarnessGemini-cred", event: "BeforeTool", matcher: "run_shell_command",
			canonical: guardCredentialGlobalCmdPSPOSIX, list: guardforms.CredentialGlobalLegacyPSPOSIX,
			run: nestedHarnessRun("gemini-credential-guard", ".gemini/settings.json"), count: nestedCount,
		},
		// gemini-git-branch-guard — nested — ~/.gemini/settings.json
		{
			site: "HarnessGemini-git", event: "BeforeTool", matcher: "run_shell_command",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: nestedHarnessRun("gemini-git-branch-guard", ".gemini/settings.json"), count: nestedCount,
		},
		// cursor-credential-guard — flat, no matcher — ~/.cursor/hooks.json
		{
			site: "HarnessCursor-cred", event: "beforeShellExecution", matcher: "",
			canonical: guardCredentialGlobalCmdPSPOSIX, list: guardforms.CredentialGlobalLegacyPSPOSIX,
			run: flatHarnessRun("cursor-credential-guard", ".cursor/hooks.json"), count: flatCount,
		},
		// cursor-git-branch-guard — flat, no matcher — ~/.cursor/hooks.json
		{
			site: "HarnessCursor-git", event: "beforeShellExecution", matcher: "",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: flatHarnessRun("cursor-git-branch-guard", ".cursor/hooks.json"), count: flatCount,
		},
		// copilot-credential-guard — flat bash — ~/.copilot/settings.json
		{
			site: "HarnessCopilot-cred", event: "preToolUse", matcher: "bash",
			canonical: guardCredentialGlobalCmdPSPOSIX, list: guardforms.CredentialGlobalLegacyPSPOSIX,
			run: bashHarnessRun("copilot-credential-guard", ".copilot/settings.json"), count: bashCount,
		},
		// copilot-git-branch-guard — flat bash — ~/.copilot/settings.json
		{
			site: "HarnessCopilot-git", event: "preToolUse", matcher: "bash",
			canonical: guardGitBranchCmdPSPOSIX, list: guardforms.GitBranchLegacyPSPOSIX,
			run: bashHarnessRun("copilot-git-branch-guard", ".copilot/settings.json"), count: bashCount,
		},
		// windsurf-credential-guard — flat (windsurf), no matcher — ~/.codeium/windsurf/hooks.json
		{
			site: "HarnessWindsurf-cred", event: "pre_run_command", matcher: "",
			canonical: guardCredentialGlobalCmdPSPOSIX, list: guardforms.CredentialGlobalLegacyPSPOSIX,
			run: flatHarnessRun("windsurf-credential-guard", ".codeium/windsurf/hooks.json"), count: flatCount,
		},
		// windsurf-git-branch-guard: no global target — omitted (see file comment)
		// kiro-credential-guard: total overwrite — omitted (see file comment)
	}

	for _, r := range rows {
		r := r
		for i, form := range r.list {
			form := form
			i := i
			label := r.site + "/" + r.event + "/" + r.matcher + "/" + ac4SanitizeLabel(form)
			t.Run(label, func(t *testing.T) {
				root := r.run(t, r.event, r.matcher, form, r.canonical)
				n := r.count(root, r.event, r.matcher, r.canonical)
				if n != 1 {
					t.Errorf("[%d] %s: canonical count=%d, want 1 (legacy=%q)", i, r.site, n, form)
				}
				if n2 := r.count(root, r.event, r.matcher, form); n2 != 0 {
					t.Errorf("[%d] %s: legacy %q survived (%d occurrences)", i, r.site, form, n2)
				}
			})
		}
	}
}
