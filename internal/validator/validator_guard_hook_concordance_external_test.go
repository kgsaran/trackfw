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
