package validator_test

// validator_guard_hook_concordance_external_test.go — ML-4C, I1 (REQ-2026-09-05)
//
// Concordance tests: for every CLI whose generator emits a hook file that the validator checks,
// call the public Inject* generator function on a fresh temp dir, then run the validator rule on
// that same dir and assert zero *_hook_resolvable violations.
//
// This file lives in package validator_test (external) so it can import BOTH
// internal/generators and internal/validator without reintroducing the import cycle
// (generators/context.go → validator). Same rationale as
// validator_credential_guard_integrity_external_test.go.
//
// Regra de reconciliação (CLAUDE.md — Regra Dura de Reconciliação):
// Each test has a one-sentence declaration of what it asserts; see the comment prefixed
// "Reconciliação:" on each function.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// setupConcordanceDir creates an isolated temp dir, chdirs into it, stubs the binary
// probe and isolates $HOME so global-harness dedup checks never find a real harness.
// Returns nothing — all cleanup is registered via t.Cleanup through the helpers.
func setupConcordanceDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // prevent globalCredentialGuardInstalled*/globalGitBranchGuardInstalled* from seeing real $HOME
	validator.ChdirForTest(t, dir)
	validator.StubProbeOKForTest(t)
	return dir
}

// assertZeroGitBranchViolations fails if ValidateGitBranchGuardHookResolvableForTest returns any violation.
func assertZeroGitBranchViolations(t *testing.T) {
	t.Helper()
	msgs, err := validator.ValidateGitBranchGuardHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateGitBranchGuardHookResolvable: unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected zero git_branch_guard_hook_resolvable violations; got %d:\n%v", len(msgs), msgs)
	}
}

// assertZeroCredentialViolations fails if ValidateCredentialGuardHookResolvableForTest returns any violation.
func assertZeroCredentialViolations(t *testing.T) {
	t.Helper()
	msgs, err := validator.ValidateCredentialGuardHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateCredentialGuardHookResolvable: unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected zero credential_guard_hook_resolvable violations; got %d:\n%v", len(msgs), msgs)
	}
}

// --------------------------------------------------------------------------
// Claude Code
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectClaudeHooks emite as linhas que
// git_branch_guard_hook_resolvable e credential_guard_hook_resolvable reconhecem como
// válidas (zero violations).
func TestGuardHookConcordance_ClaudeCode_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// Reconciliação: afirma que InjectClaudeHooks emite a linha de credential-guard que
// credential_guard_hook_resolvable reconhece como válida (zero violations).
func TestGuardHookConcordance_ClaudeCode_Credential(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}
	assertZeroCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Codex CLI
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectCodexHooks emite as linhas que
// git_branch_guard_hook_resolvable reconhece como válidas (zero violations).
func TestGuardHookConcordance_CodexCLI_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// Reconciliação: afirma que InjectCodexHooks emite a linha de credential-guard que
// credential_guard_hook_resolvable reconhece como válida (zero violations).
func TestGuardHookConcordance_CodexCLI_Credential(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks: %v", err)
	}
	assertZeroCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Gemini CLI
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectGeminiHooks emite as linhas que
// git_branch_guard_hook_resolvable reconhece como válidas (zero violations).
func TestGuardHookConcordance_GeminiCLI_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// Reconciliação: afirma que InjectGeminiHooks emite a linha de credential-guard que
// credential_guard_hook_resolvable reconhece como válida (zero violations).
func TestGuardHookConcordance_GeminiCLI_Credential(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks: %v", err)
	}
	assertZeroCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Kiro (cmd.exe family)
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectKiroHooks emite as linhas cmd.exe que
// git_branch_guard_hook_resolvable reconhece como válidas (zero violations).
func TestGuardHookConcordance_Kiro_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// Reconciliação: afirma que InjectKiroHooks emite a linha cmd.exe de credential-guard
// que credential_guard_hook_resolvable reconhece como válida (zero violations).
func TestGuardHookConcordance_Kiro_Credential(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks: %v", err)
	}
	assertZeroCredentialViolations(t)
}

// --------------------------------------------------------------------------
// GitHub Copilot CLI
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectCopilotHooks emite as linhas que
// git_branch_guard_hook_resolvable reconhece como válidas (zero violations).
func TestGuardHookConcordance_GitHubCopilot_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// Reconciliação: afirma que InjectCopilotHooks emite a linha de credential-guard que
// credential_guard_hook_resolvable reconhece como válida (zero violations).
func TestGuardHookConcordance_GitHubCopilot_Credential(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks: %v", err)
	}
	assertZeroCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Cursor
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectCursorHooks emite as linhas que
// git_branch_guard_hook_resolvable reconhece como válidas (zero violations).
func TestGuardHookConcordance_Cursor_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// Reconciliação: afirma que InjectCursorHooks emite a linha de credential-guard que
// credential_guard_hook_resolvable reconhece como válida (zero violations).
func TestGuardHookConcordance_Cursor_Credential(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks: %v", err)
	}
	assertZeroCredentialViolations(t)
}

// --------------------------------------------------------------------------
// Windsurf (git-branch only — gerador não injeta credential-guard)
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectWindsurfHooks emite a linha PS/POSIX que
// git_branch_guard_hook_resolvable reconhece como válida (zero violations).
// credential_guard_hook_resolvable não é testada aqui porque o gerador deliberadamente
// não emite credential-guard para Windsurf (ver comentário em credentialGuardHookFiles).
func TestGuardHookConcordance_Windsurf_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectWindsurfHooks(dir); err != nil {
		t.Fatalf("InjectWindsurfHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// --------------------------------------------------------------------------
// Amazon Q (cmd.exe family — git-branch only)
// --------------------------------------------------------------------------

// Reconciliação: afirma que InjectAmazonQHooks emite a linha cmd.exe que
// git_branch_guard_hook_resolvable reconhece como válida (zero violations).
// credential_guard_hook_resolvable não é testada aqui pelo mesmo motivo que Windsurf.
func TestGuardHookConcordance_AmazonQ_GitBranch(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectAmazonQHooks(dir); err != nil {
		t.Fatalf("InjectAmazonQHooks: %v", err)
	}
	assertZeroGitBranchViolations(t)
}

// --------------------------------------------------------------------------
// ML-5F — matcher warning rule only fires for shell groups (issue #530)
// --------------------------------------------------------------------------

// assertZeroMatcherWarnings fails if ValidateClaudeGuardHookMatcherWarningsForTest
// returns any warnings.
func assertZeroMatcherWarnings(t *testing.T) {
	t.Helper()
	warns, err := validator.ValidateClaudeGuardHookMatcherWarningsForTest()
	if err != nil {
		t.Fatalf("ValidateClaudeGuardHookMatcherWarnings: unexpected error: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("expected zero matcher warnings; got %d:\n%v", len(warns), warns)
	}
}

// TestClaudeGuardHookMatcherWarning_CanonicalOutputZeroWarnings calls InjectClaudeHooks
// on a fresh temp dir and asserts that validateClaudeGuardHookMatcherWarnings produces
// zero warnings. The generated settings.json includes groups "Read" and "Write|Edit"
// with guard commands but without "Bash" in the matcher — the fix must not flag them.
//
// Reconciliação: afirma que a regra de aviso de matcher é silenciosa para a saída
// canônica de InjectClaudeHooks, incluindo grupos Read e Write|Edit que contêm guard
// mas não contêm "Bash" no matcher — esses grupos não precisam de cobertura PowerShell
// porque endereçam ferramentas de leitura/escrita, não shells (issue #530).
//
// Adicionalmente, afirma que ao trocar o matcher do grupo de shell de "Bash|PowerShell"
// para "Bash", exatamente 2 avisos são emitidos (um por fase com grupo de shell+guard),
// confirmando que a regra ainda detecta o defeito real para matchers de shell.
func TestClaudeGuardHookMatcherWarning_CanonicalOutputZeroWarnings(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	// Non-vacuity: verify that "Read" and "Write|Edit" groups with guard ARE present
	// in the generated file. If they are absent, "zero warnings" is trivially true and
	// the test does not prove that the fix suppresses those specific groups.
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	raw, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatalf("read .claude/settings.json: %v", readErr)
	}
	var root map[string]interface{}
	if jsonErr := json.Unmarshal(raw, &root); jsonErr != nil {
		t.Fatalf("unmarshal .claude/settings.json: %v", jsonErr)
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	guardMarkers := []string{"trackfw guard credential", "trackfw guard git-branch"}
	foundReadGroup := false
	foundWriteEditGroup := false
	for _, phase := range []string{"PreToolUse", "PostToolUse"} {
		arr, _ := hooks[phase].([]interface{})
		for _, item := range arr {
			obj, hOk := item.(map[string]interface{})
			if !hOk {
				continue
			}
			matcher, _ := obj["matcher"].(string)
			innerHooks, _ := obj["hooks"].([]interface{})
			hasGuard := false
			for _, h := range innerHooks {
				hObj, hOk2 := h.(map[string]interface{})
				if !hOk2 {
					continue
				}
				cmd, _ := hObj["command"].(string)
				for _, marker := range guardMarkers {
					if strings.Contains(cmd, marker) {
						hasGuard = true
						break
					}
				}
				if hasGuard {
					break
				}
			}
			if hasGuard {
				switch matcher {
				case "Read":
					foundReadGroup = true
				case "Write|Edit":
					foundWriteEditGroup = true
				}
			}
		}
	}
	if !foundReadGroup {
		t.Fatal("non-vacuity: no 'Read' group with guard found in generated settings.json — fix cannot be tested")
	}
	if !foundWriteEditGroup {
		t.Fatal("non-vacuity: no 'Write|Edit' group with guard found in generated settings.json — fix cannot be tested")
	}

	// With the canonical output (shell groups use "Bash|PowerShell"), the rule must
	// produce zero warnings.
	assertZeroMatcherWarnings(t)

	// After downgrading the shell matcher from "Bash|PowerShell" to "Bash" only, the
	// rule must emit exactly one warning per phase that has a shell group with guard.
	// InjectClaudeHooks emits one "Bash|PowerShell" shell group in PreToolUse and one
	// in PostToolUse, so the expected count is 2 (one per phase, capped by the
	// break-per-phase logic in validateClaudeGuardHookMatcherWarningsInFile).
	modified := strings.ReplaceAll(string(raw), `"Bash|PowerShell"`, `"Bash"`)
	if writeErr := os.WriteFile(settingsPath, []byte(modified), 0644); writeErr != nil {
		t.Fatalf("write modified settings.json: %v", writeErr)
	}
	warns, warnErr := validator.ValidateClaudeGuardHookMatcherWarningsForTest()
	if warnErr != nil {
		t.Fatalf("ValidateClaudeGuardHookMatcherWarnings after shell-matcher downgrade: %v", warnErr)
	}
	const wantWarnings = 2
	if len(warns) != wantWarnings {
		t.Errorf("after downgrading shell matcher to 'Bash': expected %d warnings, got %d: %v",
			wantWarnings, len(warns), warns)
	}
}

// --------------------------------------------------------------------------
// ML-2C F1: global command concordance — validator expected == generator constant
// --------------------------------------------------------------------------

// TestCredentialGuardGlobalCmdWindsurf_ValidatorMatchesGenerator pins that the
// validator's expected Windsurf global command (credentialGuardGlobalExpectedCmdWindsurf)
// equals the generator's guardCredentialGlobalCmdPSPOSIX constant — ensuring that
// InjectWindsurfHooks deduplicates when the validator's string is already present
// and that validateCredentialGuardPresenceRequired emits no violation.
//
// Reconciliação: afirma que a string esperada pelo validator para o global Windsurf
// é aceita pelo mecanismo de dedup do gerador e gera zero violations de presença —
// se as duas constantes divergirem, o gerador adicionará uma entrada duplicada.
func TestCredentialGuardGlobalCmdWindsurf_ValidatorMatchesGenerator(t *testing.T) {
	// Use an isolated HOME so the validator/generator see a clean state.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows

	projectDir := setupConcordanceDir(t)

	// Write the validator's expected command into the global Windsurf hooks file
	// in BOTH required events.
	expectedCmd := validator.CredentialGuardGlobalExpectedCmdWindsurfForTest()
	hookEntry := map[string]interface{}{"command": expectedCmd, "show_output": true}
	hooksContent, _ := json.Marshal(map[string]interface{}{
		"hooks": map[string]interface{}{
			"pre_run_command": []interface{}{hookEntry},
			"pre_write_code":  []interface{}{hookEntry},
		},
	})
	globalDir := filepath.Join(home, ".codeium", "windsurf")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir global windsurf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "hooks.json"), hooksContent, 0644); err != nil {
		t.Fatalf("write global windsurf hooks: %v", err)
	}

	// Run the generator on the project directory.
	if err := generators.InjectWindsurfHooks(projectDir); err != nil {
		t.Fatalf("InjectWindsurfHooks: %v", err)
	}

	// The generator should have deduped: because the global is fully installed with
	// the exact expected form, no global credential guard entry should appear in the
	// project-scoped .windsurf/hooks.json.
	windsurf := filepath.Join(projectDir, ".windsurf", "hooks.json")
	raw, err := os.ReadFile(windsurf)
	if err != nil {
		t.Fatalf("read .windsurf/hooks.json: %v", err)
	}
	if strings.Contains(string(raw), "--global") {
		t.Errorf("generator wrote global credential guard entry to project wiring —\n"+
			"validator expected string does not match generator constant.\n"+
			"validator expected: %q", expectedCmd)
	}

	// The validator must report zero presence violations for this project.
	msgs, err := validator.ValidateCredentialGuardHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateCredentialGuardHookResolvable: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, ".windsurf/hooks.json") {
			t.Errorf("unexpected presence violation: %s", m)
		}
	}
}
