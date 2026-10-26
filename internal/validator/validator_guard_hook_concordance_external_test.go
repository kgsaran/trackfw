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

// TestGuardGlobalHookResolvable_CopilotGlobal_MarkerEmCwdNaoContaComoFiacao — pin de não-regressão
//
// Reconciliação: este teste é um pin de não-regressão — passa em origin/main. Em origin/main,
// collectCommandsWithMarker também encontrava "trackfw-credential-guard.sh" na chave "cwd", mas
// a função de validação global já ignorava caminhos relativos ("not our wiring, not our job":
// apenas caminhos absolutos emitidos pelo gerador são verificados). Assim, nenhuma violation era
// emitida lá, e o teste não distingue D16 do comportamento pré-D16.
//
// O que o teste afirma: com a chave "cwd" contendo o script marker e a chave "bash" contendo o
// subcomando canônico, ~/.copilot/settings.json não gera nenhuma mensagem de violation — o
// marcador em "cwd" (campo não-executável no esquema hookCmdSchemaFlatBash) não conta como fiação.
//
// A prova de mordida de D16 para o Copilot global está nos testes de presença obrigatória e nos
// testes AC3/AC4 (Windsurf e Amazon Q) — não neste teste.
func TestGuardGlobalHookResolvable_CopilotGlobal_MarkerEmCwdNaoContaComoFiacao(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	setupConcordanceDir(t)

	copilotDir := filepath.Join(home, ".copilot")
	if err := os.MkdirAll(copilotDir, 0755); err != nil {
		t.Fatalf("mkdir .copilot: %v", err)
	}

	// Correct subcommand form in "bash"; script marker placed in "cwd" (a non-command field).
	// The schema for ~/.copilot/settings.json is hookCmdSchemaFlatBash: only "bash" is read.
	// D16: "cwd" is not the executable path for this schema → not counted as wiring.
	entry := map[string]interface{}{
		"type":    "command",
		"matcher": "bash",
		"bash":    "trackfw guard credential --global",
		"cwd":     "trackfw-credential-guard.sh", // marker in non-command field
	}
	content, _ := json.Marshal(map[string]interface{}{
		"hooks": map[string]interface{}{
			"preToolUse":  []interface{}{entry},
			"postToolUse": []interface{}{entry},
		},
	})
	if err := os.WriteFile(filepath.Join(copilotDir, "settings.json"), content, 0644); err != nil {
		t.Fatalf("write .copilot/settings.json: %v", err)
	}

	msgs, err := validator.ValidateCredentialGuardGlobalHookResolvableForTest()
	if err != nil {
		t.Fatalf("ValidateCredentialGuardGlobalHookResolvable: unexpected error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, ".copilot/settings.json") {
			t.Errorf("Ameaça C: unexpected violation for .copilot/settings.json — marker in non-bash field should not count as wiring: %s", m)
		}
	}
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

// --------------------------------------------------------------------------
// AC3 — Inject* produces zero credential_guard_presence_warning for 6 CLIs
// (D17, REQ-2026-10-25 #619 ML-1A)
// --------------------------------------------------------------------------

// assertZeroPresenceWarningsForPath fails if ValidateCredentialGuardPresenceWarningsForTest
// returns any warning that mentions the given hook file path.
func assertZeroPresenceWarningsForPath(t *testing.T, hookPath string) {
	t.Helper()
	msgs, err := validator.ValidateCredentialGuardPresenceWarningsForTest()
	if err != nil {
		t.Fatalf("ValidateCredentialGuardPresenceWarnings: unexpected error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, hookPath) {
			t.Errorf("unexpected credential_guard_presence_warning for %s: %s", hookPath, m)
		}
	}
}

// TestGuardHookConcordance_D17_ClaudeCode_PresenceWarning afirma que InjectClaudeHooks emite
// os 3 pares de credential guard que a regra credential_guard_presence_warning reconhece como
// presentes (zero warnings após inject).
// Reconciliação (AC3, #619 ML-1A): valida que o matcher exato da lista de entradas coincide
// com o que o gerador escreve para Claude Code.
func TestGuardHookConcordance_D17_ClaudeCode_PresenceWarning(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}
	assertZeroPresenceWarningsForPath(t, ".claude/settings.json")
}

// TestGuardHookConcordance_D17_CodexCLI_PresenceWarning afirma que InjectCodexHooks emite
// os 2 pares de credential guard que a regra reconhece (zero warnings após inject).
// Reconciliação (AC3, #619 ML-1A): matcher exato da lista coincide com o gerador Codex.
func TestGuardHookConcordance_D17_CodexCLI_PresenceWarning(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCodexHooks(dir); err != nil {
		t.Fatalf("InjectCodexHooks: %v", err)
	}
	assertZeroPresenceWarningsForPath(t, ".codex/hooks.json")
}

// TestGuardHookConcordance_D17_GeminiCLI_PresenceWarning afirma que InjectGeminiHooks emite
// os 3 pares de credential guard que a regra reconhece (zero warnings após inject).
// Reconciliação (AC3, #619 ML-1A): BeforeTool matchers coincidem com o gerador Gemini.
func TestGuardHookConcordance_D17_GeminiCLI_PresenceWarning(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectGeminiHooks(dir); err != nil {
		t.Fatalf("InjectGeminiHooks: %v", err)
	}
	assertZeroPresenceWarningsForPath(t, ".gemini/settings.json")
}

// TestGuardHookConcordance_D17_Cursor_PresenceWarning afirma que InjectCursorHooks emite
// os 3 pares de credential guard que a regra reconhece (zero warnings após inject).
// Reconciliação (AC3, #619 ML-1A): schemaFlat beforeShellExecution + preToolUse+Read/Write
// coincidem com o gerador Cursor.
func TestGuardHookConcordance_D17_Cursor_PresenceWarning(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCursorHooks(dir); err != nil {
		t.Fatalf("InjectCursorHooks: %v", err)
	}
	assertZeroPresenceWarningsForPath(t, ".cursor/hooks.json")
}

// TestGuardHookConcordance_D17_GitHubCopilot_PresenceWarning afirma que InjectCopilotHooks
// emite os 3 pares de credential guard que a regra reconhece (zero warnings após inject).
// Reconciliação (AC3, #619 ML-1A): schemaFlat preToolUse+bash/view/create|edit coincidem
// com o gerador Copilot.
func TestGuardHookConcordance_D17_GitHubCopilot_PresenceWarning(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks: %v", err)
	}
	assertZeroPresenceWarningsForPath(t, ".github/hooks/trackfw-attention.json")
}

// TestGuardHookConcordance_D17_Kiro_PresenceWarning afirma que InjectKiroHooks emite
// os 3 pares de credential guard que a regra reconhece (zero warnings após inject).
// Reconciliação (AC3, #619 ML-1A): schemaKiro PreToolUse+shell/read/write coincidem
// com o gerador Kiro.
func TestGuardHookConcordance_D17_Kiro_PresenceWarning(t *testing.T) {
	dir := setupConcordanceDir(t)
	if err := generators.InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks: %v", err)
	}
	assertZeroPresenceWarningsForPath(t, ".kiro/hooks/trackfw-attention.json")
}

// --------------------------------------------------------------------------
// AC5 — Entry table ponto único: warnOnly count = 6, violation count = 2
// --------------------------------------------------------------------------

// TestCredentialGuardPresence_AC5_EntryTableCount afirma que credentialGuardRequiredEntries
// tem exatamente 6 entradas warnOnly=true (os 6 CLIs sem violation-grade) e exatamente 2
// entradas warnOnly=false (Windsurf e Amazon Q).
// Reconciliação (AC5, #619 ML-1A): a tabela é o ponto único compartilhado entre a regra de
// violation e a de aviso; um developer que adiciona um CLI novo sem entrada nessa tabela faz
// este teste falhar.
func TestCredentialGuardPresence_AC5_EntryTableCount(t *testing.T) {
	got := validator.CredentialGuardPresenceWarnOnlyCountForTest()
	if got != 6 {
		t.Errorf("AC5: esperado 6 entradas warnOnly=true em credentialGuardRequiredEntries, obteve %d", got)
	}
	gotViol := validator.CredentialGuardPresenceViolationCountForTest()
	if gotViol != 2 {
		t.Errorf("AC5: esperado 2 entradas warnOnly=false (Windsurf+AmazonQ), obteve %d", gotViol)
	}
}

// --------------------------------------------------------------------------
// AC5 — Bidirectional: generated ⊇ table AND table ⊇ blocking-pairs-in-generated
// (D17, REQ-2026-10-25 #619 ML-1A2)
// --------------------------------------------------------------------------

// blockingEventsForCLI returns the set of event keys that are "blocking" (pre-execution)
// for a given CLI hook file. Hard-coded from Wave 0 §1.2 to avoid circular dependency
// on the production table.
func blockingEventsForCLI(hookPath string) map[string]bool {
	switch hookPath {
	case ".claude/settings.json":
		return map[string]bool{"PreToolUse": true}
	case ".codex/hooks.json":
		return map[string]bool{"PreToolUse": true}
	case ".gemini/settings.json":
		return map[string]bool{"BeforeTool": true}
	case ".cursor/hooks.json":
		return map[string]bool{"beforeShellExecution": true, "preToolUse": true}
	case ".github/hooks/trackfw-attention.json":
		return map[string]bool{"preToolUse": true}
	case ".kiro/hooks/trackfw-attention.json":
		return map[string]bool{"PreToolUse": true}
	}
	return nil
}

// guardPairKey is a (hookEvent, matcher) pair for use as a map key.
type guardPairKey struct{ event, matcher string }

// containsGuardMarker returns true if cmd contains the credential guard script or subcommand marker.
func containsGuardMarker(cmd string) bool {
	return strings.Contains(cmd, "trackfw-credential-guard.sh") ||
		strings.Contains(cmd, "trackfw guard credential")
}

// extractBlockingPairsNested walks a schemaNested JSON (hooks[event][i]["hooks"][j]["command"])
// and returns all (event, matcher) pairs in blocking events that contain the guard marker.
// requiresCommandType: when true, only entries with "type":"command" count.
func extractBlockingPairsNested(parsed map[string]interface{}, blocking map[string]bool, requiresCommandType bool) map[guardPairKey]bool {
	out := make(map[guardPairKey]bool)
	hooksObj, _ := parsed["hooks"].(map[string]interface{})
	for event := range blocking {
		arr, _ := hooksObj[event].([]interface{})
		for _, groupRaw := range arr {
			group, _ := groupRaw.(map[string]interface{})
			if group == nil {
				continue
			}
			matcher, _ := group["matcher"].(string)
			innerHooks, _ := group["hooks"].([]interface{})
			for _, ih := range innerHooks {
				inner, _ := ih.(map[string]interface{})
				if inner == nil {
					continue
				}
				if requiresCommandType {
					if typeStr, _ := inner["type"].(string); typeStr != "command" {
						continue
					}
				}
				cmd, _ := inner["command"].(string)
				if containsGuardMarker(cmd) {
					out[guardPairKey{event, matcher}] = true
				}
			}
		}
	}
	return out
}

// extractBlockingPairsFlat walks a schemaFlat JSON (hooks[event][i]["command"])
// and returns all (event, matcher) pairs in blocking events that contain the guard marker.
// requiresCommandType: when true, only entries with "type":"command" count.
func extractBlockingPairsFlat(parsed map[string]interface{}, blocking map[string]bool, requiresCommandType bool) map[guardPairKey]bool {
	out := make(map[guardPairKey]bool)
	hooksObj, _ := parsed["hooks"].(map[string]interface{})
	for event := range blocking {
		arr, _ := hooksObj[event].([]interface{})
		for _, entryRaw := range arr {
			entry, _ := entryRaw.(map[string]interface{})
			if entry == nil {
				continue
			}
			if requiresCommandType {
				if typeStr, _ := entry["type"].(string); typeStr != "command" {
					continue
				}
			}
			matcher, _ := entry["matcher"].(string)
			cmd, _ := entry["command"].(string)
			if containsGuardMarker(cmd) {
				out[guardPairKey{event, matcher}] = true
			}
		}
	}
	return out
}

// extractBlockingPairsKiro walks a schemaKiro JSON (hooks[i]["action"]["command"])
// and returns all (trigger, matcher) pairs in blocking events that contain the guard marker.
// requiresCommandType: when true, only action entries with "type":"command" count.
func extractBlockingPairsKiro(parsed map[string]interface{}, blocking map[string]bool, requiresCommandType bool) map[guardPairKey]bool {
	out := make(map[guardPairKey]bool)
	arr, _ := parsed["hooks"].([]interface{})
	for _, entryRaw := range arr {
		entry, _ := entryRaw.(map[string]interface{})
		if entry == nil {
			continue
		}
		trigger, _ := entry["trigger"].(string)
		if !blocking[trigger] {
			continue
		}
		action, _ := entry["action"].(map[string]interface{})
		if action == nil {
			continue
		}
		if requiresCommandType {
			if typeStr, _ := action["type"].(string); typeStr != "command" {
				continue
			}
		}
		cmd, _ := action["command"].(string)
		if containsGuardMarker(cmd) {
			matcher, _ := entry["matcher"].(string)
			out[guardPairKey{trigger, matcher}] = true
		}
	}
	return out
}

// extractGeneratedBlockingPairs reads the hook file at hookPath (relative to dir), parses it,
// and returns all (event, matcher) pairs in blocking events that contain the guard marker.
// Uses a hand-written walker (not extractPresenceCommandsD17) to avoid circular dependency.
func extractGeneratedBlockingPairs(t *testing.T, dir, hookPath, schema string, requiresCommandType bool) map[guardPairKey]bool {
	t.Helper()
	fullPath := filepath.Join(dir, filepath.FromSlash(hookPath))
	data, err := os.ReadFile(fullPath) //nolint:gosec
	if err != nil {
		t.Fatalf("readFile(%s): %v", hookPath, err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal(%s): %v", hookPath, err)
	}
	blocking := blockingEventsForCLI(hookPath)
	switch schema {
	case "nested":
		return extractBlockingPairsNested(parsed, blocking, requiresCommandType)
	case "flat":
		return extractBlockingPairsFlat(parsed, blocking, requiresCommandType)
	case "kiro":
		return extractBlockingPairsKiro(parsed, blocking, requiresCommandType)
	default:
		t.Fatalf("unknown schema %q for %s", schema, hookPath)
		return nil
	}
}

// inject function table for each of the 6 CLIs (path → inject function).
type injectFn func(dir string) error

func injectFnFor(hookPath string) injectFn {
	switch hookPath {
	case ".claude/settings.json":
		return generators.InjectClaudeHooks
	case ".codex/hooks.json":
		return generators.InjectCodexHooks
	case ".gemini/settings.json":
		return generators.InjectGeminiHooks
	case ".cursor/hooks.json":
		return generators.InjectCursorHooks
	case ".github/hooks/trackfw-attention.json":
		return generators.InjectCopilotHooks
	case ".kiro/hooks/trackfw-attention.json":
		return generators.InjectKiroHooks
	}
	return nil
}

// TestCredentialGuardPresence_AC5_GeneratorTableBidirectional_GeneratedSubsetOfTable afirma
// que todo par (evento bloqueante, matcher) gerado por Inject* que contém o guard também existe
// na tabela credentialGuardRequiredEntries (warnOnly=true). Isso garante que a tabela não
// esquece nenhum par gerado — um par gerado mas ausente da tabela significa um warning falso
// permanente que `trackfw update` não resolve.
// Reconciliação (AC5, #619 ML-1A2): walker independente do código de produção; blocking events
// hard-coded do Wave 0 §1.2 para evitar circularidade.
func TestCredentialGuardPresence_AC5_GeneratorTableBidirectional_GeneratedSubsetOfTable(t *testing.T) {
	entries := validator.CredentialGuardPresenceWarnOnlyEntriesForTest()

	for _, entry := range entries {
		entry := entry // capture range variable
		t.Run(entry.Path, func(t *testing.T) {
			dir := setupConcordanceDir(t)

			inject := injectFnFor(entry.Path)
			if inject == nil {
				t.Fatalf("no inject function for %s", entry.Path)
			}
			if err := inject(dir); err != nil {
				t.Fatalf("Inject*Hooks: %v", err)
			}

			// Verify the hook file was actually written.
			fullPath := filepath.Join(dir, filepath.FromSlash(entry.Path))
			if _, err := os.Stat(fullPath); err != nil {
				t.Fatalf("hook file not written after Inject*: %v", err)
			}

			// Pick schema and requiresCommandType from the first event (all events share the same schema).
			schema := ""
			requiresCommandType := false
			if len(entry.Events) > 0 {
				schema = entry.Events[0].Schema
				requiresCommandType = entry.Events[0].RequiresCommandType
			}

			// Build the table set of (event, matcher) pairs for this CLI.
			tableSet := make(map[guardPairKey]bool)
			for _, ev := range entry.Events {
				tableSet[guardPairKey{ev.HookEvent, ev.Matcher}] = true
			}

			// Walk the generated file with an independent hand-written walker.
			generatedPairs := extractGeneratedBlockingPairs(t, dir, entry.Path, schema, requiresCommandType)

			// Direction: generated ⊆ table (table doesn't miss any generated blocking pair).
			for pair := range generatedPairs {
				if !tableSet[pair] {
					t.Errorf("generated blocking pair (%q, %q) is not in the table for %s — "+
						"table is missing a pair that `trackfw update` would write; "+
						"add it to credentialGuardRequiredEntries",
						pair.event, pair.matcher, entry.Path)
				}
			}
		})
	}
}

// --------------------------------------------------------------------------
// AC3 — com guard global instalado pelo UpdateHarness real
// (D17, REQ-2026-10-25 #619 ML-1A2)
// --------------------------------------------------------------------------

// TestGuardHookConcordance_D17_WithGlobalHarness_ClaudeCode_ZeroPresenceWarnings afirma que,
// depois de UpdateHarness instalar o guard global em um HOME falso e InjectClaudeHooks escrever
// o arquivo de projeto, validateCredentialGuardPresenceWarnings emite zero warnings para Claude.
// Reconciliação (AC3, #619 ML-1A2): D15 garante que project-scope é sempre emitido pelo gerador
// independente do global; o aviso não deve disparar quando ambos estão instalados.
func TestGuardHookConcordance_D17_WithGlobalHarness_ClaudeCode_ZeroPresenceWarnings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows compatibility even on non-Windows

	dir := t.TempDir()
	validator.ChdirForTest(t, dir)
	validator.StubProbeOKForTest(t)

	// Step 1: install the global harness into the fake HOME (writes ~/.claude/settings.json).
	report, err := generators.UpdateHarness(generators.UpdateOptions{
		Targets:        []string{"claude-credential-guard"},
		InstallMissing: true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness: %v", err)
	}
	_ = report // result is informational; we verify the file below

	// Assertion 1: global guard file exists in fake HOME.
	globalPath := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.Stat(globalPath); err != nil {
		t.Fatalf("global guard file not written to fake HOME by UpdateHarness: %v", err)
	}

	// Step 2: inject project-scope hook file.
	if err := generators.InjectClaudeHooks(dir); err != nil {
		t.Fatalf("InjectClaudeHooks: %v", err)
	}

	// Assertion 2: project hook file exists.
	projectPath := filepath.Join(dir, ".claude", "settings.json")
	if _, err := os.Stat(projectPath); err != nil {
		t.Fatalf("project hook file not written by InjectClaudeHooks: %v", err)
	}

	// Assertion 3: zero presence warnings for Claude after both global and project installs.
	assertZeroPresenceWarningsForPath(t, ".claude/settings.json")
}

// TestGuardHookConcordance_D17_WithGlobalHarness_Kiro_ZeroPresenceWarnings afirma que,
// depois de UpdateHarness instalar o guard global Kiro e InjectKiroHooks escrever o arquivo
// de projeto, validateCredentialGuardPresenceWarnings emite zero warnings para Kiro.
// Reconciliação (AC3, #619 ML-1A2): schema Kiro (action.command) é o mais distinto dos 6;
// confirmar que global+project não dispara aviso de presença.
func TestGuardHookConcordance_D17_WithGlobalHarness_Kiro_ZeroPresenceWarnings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := t.TempDir()
	validator.ChdirForTest(t, dir)
	validator.StubProbeOKForTest(t)

	// Step 1: install global harness for Kiro.
	_, err := generators.UpdateHarness(generators.UpdateOptions{
		Targets:        []string{"kiro-credential-guard"},
		InstallMissing: true,
	})
	if err != nil {
		t.Fatalf("UpdateHarness(kiro): %v", err)
	}

	// Step 2: inject project-scope hook file.
	if err := generators.InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks: %v", err)
	}

	// Assertion: project file exists.
	if _, err := os.Stat(filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json")); err != nil {
		t.Fatalf("Kiro project hook file not written: %v", err)
	}

	// Assertion: zero presence warnings for Kiro.
	assertZeroPresenceWarningsForPath(t, ".kiro/hooks/trackfw-attention.json")
}

// --------------------------------------------------------------------------
// AC2 braço — Validate() fora do lenient confirma roteamento para warnings
// (D17, REQ-2026-10-25 #619 ML-1A2)
// --------------------------------------------------------------------------

// TestCredentialGuardPresence_AC2_ValidateForTest_RoteiaWarningsNaoViolations afirma que
// ValidateForTest() (pipeline completo, baseline + lenient aplicado) roteia os warnings de
// credential_guard_presence_warning para warnings, não para violations — mesmo em strict mode.
// Reconciliação (AC2, #619 ML-1A2): confirma que o roteamento via applyRuleWarnOnly no caminho
// Validate() é idêntico ao do caminho ValidateUnfiltered(); qualquer regressão que mova a
// mensagem para violations faria este teste falhar.
func TestCredentialGuardPresence_AC2_ValidateForTest_RoteiaWarningsNaoViolations(t *testing.T) {
	dir := setupConcordanceDir(t)

	// Write .claude/settings.json without credential guard — presence warnings should fire.
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(`{
  "hooks": {
    "PostToolUse": [
      {"matcher": "AskUserQuestion", "hooks": [{"command": "scripts/cleanup.sh", "type": "command"}]}
    ]
  }
}`), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	violations, warnings, err := validator.ValidateForTest()
	if err != nil {
		t.Fatalf("ValidateForTest: %v", err)
	}

	// Assertion 1: nenhuma violation contém "has no credential guard entry".
	for _, v := range violations {
		if strings.Contains(v, "has no credential guard entry") {
			t.Errorf("AC2 ValidateForTest: credential guard presence deve ser warning, não violation; obteve: %q", v)
		}
	}

	// Assertion 2: pelo menos um warning contém "has no credential guard entry".
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "has no credential guard entry") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AC2 ValidateForTest: esperado aviso 'has no credential guard entry' em warnings; violations=%v warnings=%v", violations, warnings)
	}
}
