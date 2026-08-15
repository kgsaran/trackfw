package generators

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Generator: file creation — mesmo padrão de credential_guard_test.go.
// ---------------------------------------------------------------------------

func TestGenerateGitBranchGuardScript_CreatesExecutableFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateGitBranchGuardScript(""); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript erro: %v", err)
	}

	path := filepath.Join("scripts", "trackfw-git-branch-guard.sh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("script não foi criado: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Errorf("script não é executável: mode=%v", info.Mode())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro lendo script: %v", err)
	}
	if !strings.HasPrefix(string(content), "#!/usr/bin/env bash") {
		t.Errorf("script não começa com shebang esperado")
	}
}

func TestGenerateGlobalGitBranchGuardScript_WritesUnderTrackfwHomeScripts(t *testing.T) {
	fakeHome := t.TempDir()

	if err := GenerateGlobalGitBranchGuardScript(fakeHome); err != nil {
		t.Fatalf("GenerateGlobalGitBranchGuardScript erro: %v", err)
	}

	path := filepath.Join(fakeHome, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("script global não foi criado em %s: %v", path, err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Errorf("script global não é executável: mode=%v", info.Mode())
	}
}

func TestGenerateGlobalGitBranchGuardScript_EmptyHome_Errors(t *testing.T) {
	if err := GenerateGlobalGitBranchGuardScript(""); err == nil {
		t.Error("esperava erro com home vazio (nunca deve cair silenciosamente em cwd)")
	}
}

func TestGenerateGitBranchGuardScript_DoesNotWireIntoAnyHooksFile(t *testing.T) {
	// ML-1A explicitamente não injeta o script em nenhum hooks.json/settings.json de CLI —
	// isso é escopo da Wave 3. Confirma que apenas o script shell é criado.
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateGitBranchGuardScript(""); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript erro: %v", err)
	}

	for _, p := range []string{
		".claude/settings.json",
		".codex/hooks.json",
		".gemini/settings.json",
		".github/hooks/hooks.json",
		".cursor/hooks.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("ML-1A não deve criar %s (escopo da Wave 3)", p)
		}
	}
}

// ---------------------------------------------------------------------------
// Behavior — invoca o script real como subprocesso (não reimplementa a regex em
// paralelo), mesmo padrão de runCredentialGuard.
// ---------------------------------------------------------------------------

func setupGitBranchGuardFixture(t *testing.T) (dir, scriptPath string) {
	t.Helper()
	dir = t.TempDir()
	if err := GenerateGitBranchGuardScript(dir); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript erro: %v", err)
	}
	return dir, filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
}

func runGitBranchGuard(t *testing.T, dir, scriptPath string, args []string, stdin string) (exitCode int, stdout, stderr string) {
	t.Helper()
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return 0, outBuf.String(), errBuf.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), outBuf.String(), errBuf.String()
	}
	t.Fatalf("erro executando script: %v (stderr: %s)", err, errBuf.String())
	return -1, "", ""
}

// --- Bloqueio: git commit ---------------------------------------------------

func TestGitBranchGuard_Commit_StdinJSON_ToolInputCommand_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_name":"Bash","tool_input":{"command":"git commit -m \"x\""}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"decision":"block"`) {
		t.Errorf("stdout deveria conter decision block, got: %s", stdout)
	}
	if !strings.Contains(stdout, "trackfw commit") {
		t.Errorf("mensagem deveria orientar para 'trackfw commit', got: %s", stdout)
	}
	if !strings.Contains(stderr, "CLAUDE.md") {
		t.Errorf("mensagem deveria referenciar CLAUDE.md, got: %s", stderr)
	}
}

func TestGitBranchGuard_Commit_Argv_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)

	code, stdout, stderr := runGitBranchGuard(t, dir, script, []string{"git", "commit", "-m", "x"}, "")
	if code != 2 {
		t.Fatalf("exit code: want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"decision":"block"`) {
		t.Errorf("stdout deveria conter decision block, got: %s", stdout)
	}
}

// --- Bloqueio: git push -----------------------------------------------------

func TestGitBranchGuard_Push_StdinJSON_CommandField_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"command":"git push"}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw ship") {
		t.Errorf("mensagem deveria orientar para 'trackfw ship', got: %s", stdout)
	}
}

func TestGitBranchGuard_Push_WithNoPagerFlag_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git --no-pager push"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (flag antes do subcomando), got %d (stderr: %s)", code, stderr)
	}
}

// --- Bloqueio: git checkout -b ---------------------------------------------

func TestGitBranchGuard_CheckoutDashB_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git checkout -b feat/x"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw branch new") {
		t.Errorf("mensagem deveria orientar para 'trackfw branch new', got: %s", stdout)
	}
}

func TestGitBranchGuard_CheckoutDashB_WithFlagsBefore_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git -C . checkout -b feat/x"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (flag -C antes do subcomando), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_CheckoutWithoutDashB_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git checkout feat/x"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (checkout sem -b não é bloqueado), got %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("allow deveria ser silencioso, got stdout: %s", stdout)
	}
}

// --- Allow: comandos git inofensivos ----------------------------------------

func TestGitBranchGuard_Status_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git status"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("allow deveria ser silencioso, got: %s", stdout)
	}
}

func TestGitBranchGuard_Diff_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git diff origin/main"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_Log_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git log --oneline -5"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_NoCommandAtAll_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, "")
	if code != 0 {
		t.Errorf("exit code: want 0 (sem comando, allow por omissão), got %d (stderr: %s)", code, stderr)
	}
}

// --- Formatos de entrada -----------------------------------------------------

func TestGitBranchGuard_HookInputCommandField_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"hook_input":{"command":"git commit -m \"x\""}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (campo hook_input.command), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_RawStdin_NonJSON_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, "git push")
	if code != 2 {
		t.Fatalf("exit code: want 2 (stdin cru, não-JSON), got %d (stderr: %s)", code, stderr)
	}
}

// --- Regressão de teste manual E2E (ML-4A): bugs reais no parser de segmentos --------------

func TestGitBranchGuard_ChainedCommand_SecondGitBlocked(t *testing.T) {
	// Bug 1: "git status; git push origin HEAD" não era bloqueado porque o parser antigo só
	// coletava tokens a partir da PRIMEIRA ocorrência de "git" na string inteira.
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git status; git push origin HEAD"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (git push encadeado após ';' deve ser bloqueado), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw ship") {
		t.Errorf("mensagem deveria orientar para 'trackfw ship', got: %s", stdout)
	}
}

func TestGitBranchGuard_AbsolutePathGit_Blocks(t *testing.T) {
	// Bug 2: "/usr/bin/git commit -m x" não era bloqueado porque o parser antigo comparava
	// "$tok" = "git" por igualdade exata, e nunca por basename.
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"/usr/bin/git commit -m x"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (path absoluto para git deve ser bloqueado), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw commit") {
		t.Errorf("mensagem deveria orientar para 'trackfw commit', got: %s", stdout)
	}
}

func TestGitBranchGuard_ProseTextMentioningGitCommit_DoesNotBlock(t *testing.T) {
	// Bug 3 (crítico): comando legítimo `bin/trackfw commit -m "..."` era bloqueado sempre
	// que a mensagem de commit mencionava a frase "git commit" em algum lugar, porque o
	// parser antigo procurava "git" em qualquer posição da string inteira, não só no primeiro
	// token de um segmento real de comando.
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"bin/trackfw commit -m \"nota: antes do git commit real, valide o gate\""}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (comando legítimo 'trackfw commit' com prosa mencionando 'git commit' não deve ser bloqueado), got %d (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("allow deveria ser silencioso, got: %s", stdout)
	}
}

func TestGitBranchGuard_MultilineHeredocProseMentioningGitCommit_DoesNotBlock(t *testing.T) {
	// Variante multi-linha do bug 3: um heredoc de mensagem de commit com "git commit" no
	// meio de uma linha de prosa, não como primeiro token da linha.
	dir, script := setupGitBranchGuardFixture(t)
	cmd := "bin/trackfw commit -m \"$(cat <<'EOF'\n" +
		"Fix guard parsing bug.\n" +
		"Bug real encontrado pelo gate: comando escapava antes do git commit real.\n" +
		"EOF\n" +
		")\""
	payload := `{"tool_input":{"command":"` + jsonEscape(cmd) + `"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (heredoc com 'git commit' no meio de uma linha de prosa não deve ser bloqueado), got %d (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
}

func jsonEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func TestGitBranchGuard_EnvVarFallback_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)

	cmd := exec.Command("bash", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TRACKFW_GIT_COMMAND=git commit -m x")
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("erro executando script: %v", err)
		}
	}
	if exitCode != 2 {
		t.Fatalf("exit code: want 2 (fallback de env var), got %d (stderr: %s)", exitCode, errBuf.String())
	}
}
