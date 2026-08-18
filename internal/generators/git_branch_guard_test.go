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

// setupGitBranchGuardFixture cria um diretório de fixture com trackfw.yaml na raiz — o guard só
// funciona (não vira no-op) dentro de um projeto trackfw (ML-1A, ADR-2026-08-17-guard-global-
// cabeado-com-no-op-fora-de-projeto-trackfw.md). Todos os testes de bloqueio/allow pré-existentes
// (que verificam comportamento DENTRO de projeto trackfw) dependem deste arquivo existir; os
// testes específicos do no-op (fora de projeto) usam setupGitBranchGuardFixtureWithoutTrackfwYAML.
func setupGitBranchGuardFixture(t *testing.T) (dir, scriptPath string) {
	t.Helper()
	dir = t.TempDir()
	if err := GenerateGitBranchGuardScript(dir); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript erro: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("project_name: fixture\n"), 0644); err != nil {
		t.Fatalf("erro escrevendo trackfw.yaml de fixture: %v", err)
	}
	return dir, filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
}

// setupGitBranchGuardFixtureWithoutTrackfwYAML é o par de setupGitBranchGuardFixture SEM
// trackfw.yaml — usado pelos testes de no-op (ML-1A). t.TempDir() garante isolamento do repo
// real (ver TestGitBranchGuard_FixtureHasNoTrackfwYAMLAncestor, que prova a premissa em vez de
// presumi-la).
func setupGitBranchGuardFixtureWithoutTrackfwYAML(t *testing.T) (dir, scriptPath string) {
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

// --- ML-1A (ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-
// release-7-0-0.md): item 1 (falso-positivo por prosa que COMEÇA a linha) + item 2 (brecha
// `git switch -c`) -----------------------------------------------------------------------

func TestGitBranchGuard_CommitMessageLineStartingWithGitCheckoutDashB_DoesNotBlock(t *testing.T) {
	// Reprodução literal do incidente real (vault/notes/git-branch-guard-falso-positivo-em-
	// linha-de-mensagem-de-commit-2026-08-16.md): uma mensagem de commit multi-linha via
	// `-m "$(cat <<'EOF' ... EOF)"` (convenção deste próprio CLAUDE.md) cuja PRIMEIRA linha do
	// corpo começa com "git checkout -b" era lida como um pseudo-segmento de comando pelo
	// parser antigo (que segmentava por quebra de linha real sem noção de aspas). Diferente de
	// TestGitBranchGuard_ProseTextMentioningGitCommit_DoesNotBlock (que testa "git commit" no
	// MEIO de uma frase, já corrigido antes deste ML): aqui "git" é o PRIMEIRO token da linha.
	dir, script := setupGitBranchGuardFixture(t)
	cmd := "bin/trackfw commit -m \"$(cat <<'EOF'\n" +
		"  git checkout -b            -> bloqueado pelo guard\n" +
		"  trackfw branch new chore/  -> recusado\n" +
		"EOF\n" +
		")\""
	payload := `{"tool_input":{"command":"` + jsonEscape(cmd) + `"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (linha de mensagem começando com 'git checkout -b' não deve bloquear), got %d (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("allow deveria ser silencioso, got: %s", stdout)
	}
}

func TestGitBranchGuard_QuotedMessageThenRealChainedCommand_StillBlocks(t *testing.T) {
	// Não-regressão crítica (o risco que este ML foi avisado a não abrir): um `-m "..."`
	// corretamente fechado seguido de um `git push` real encadeado por ';' ou '&&' TEM que
	// continuar bloqueando — o pré-processamento quote-aware não pode esconder um comando real
	// que vem DEPOIS da aspa de fechamento.
	dir, script := setupGitBranchGuardFixture(t)

	cases := []string{
		`git commit -m "x"; git push`,
		`git commit -m "x" && git push`,
	}
	for _, cmd := range cases {
		payload := `{"tool_input":{"command":"` + jsonEscape(cmd) + `"}}`
		code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
		if code != 2 {
			t.Errorf("cmd=%q: exit code: want 2 (comando real encadeado após -m fechado deve bloquear), got %d (stdout: %s, stderr: %s)", cmd, code, stdout, stderr)
		}
		if !strings.Contains(stdout, "trackfw commit") {
			t.Errorf("cmd=%q: mensagem deveria orientar para 'trackfw commit' (primeiro segmento casado), got: %s", cmd, stdout)
		}
	}
}

func TestGitBranchGuard_UnterminatedHeredocBeforeRealPush_StillBlocks(t *testing.T) {
	// Não-regressão do fallback de segurança de strip_heredoc_bodies: se o heredoc nunca fecha
	// (terminador ausente/incompatível), o texto ORIGINAL deve ser usado — nunca esconder um
	// `git push` real que vem depois.
	dir, script := setupGitBranchGuardFixture(t)
	cmd := "git status <<'EOF'\nwhatever\nNOTEOF\ngit push origin main"
	payload := `{"tool_input":{"command":"` + jsonEscape(cmd) + `"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (heredoc mal-formado não pode esconder git push real), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw ship") {
		t.Errorf("mensagem deveria orientar para 'trackfw ship', got: %s", stdout)
	}
}

func TestGitBranchGuard_SwitchDashC_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git switch -c feat/x"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (git switch -c é forma alternativa a checkout -b), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw branch new") {
		t.Errorf("mensagem deveria orientar para 'trackfw branch new', got: %s", stdout)
	}
}

func TestGitBranchGuard_SwitchDashC_FlagBeforeCreate_Blocks(t *testing.T) {
	// git switch --track -c feat/x: -c não é o primeiro token após "switch", varredura de
	// todos os tokens é necessária (mesmo espírito do bug 2, mas para "switch").
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git switch --track -c feat/x"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (flag --track antes de -c), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_SwitchWithoutCreateFlag_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git switch main"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (switch sem -c/-C/--create não é bloqueado), got %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("allow deveria ser silencioso, got: %s", stdout)
	}
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

// ---------------------------------------------------------------------------
// ML-4C: git branch <nome> / -c/-C/-m/-M, git worktree add -b, env VAR=val.
// ---------------------------------------------------------------------------

func TestGitBranchGuard_BranchWithPositionalName_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git branch nova"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (git branch <nome> cria branch), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw branch new") {
		t.Errorf("mensagem deveria orientar para 'trackfw branch new', got: %s", stdout)
	}
}

func TestGitBranchGuard_BranchDashC_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git branch -c origem nova"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (git branch -c copia/cria branch), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_BranchDashM_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git branch -m old new"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (git branch -m renomeia/cria branch), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_BranchNoArgs_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git branch"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (git branch sem args é leitura), got %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("allow deveria ser silencioso, got: %s", stdout)
	}
}

func TestGitBranchGuard_BranchListFlags_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	for _, cmd := range []string{
		"git branch -a", "git branch -r", "git branch -l", "git branch --list",
		"git branch -v", "git branch -vv", "git branch --show-current",
		"git branch --contains abc123", "git branch --merged", "git branch --no-merged",
		"git branch --sort=-committerdate", "git branch --format=%(refname)",
	} {
		payload := `{"tool_input":{"command":"` + cmd + `"}}`
		code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
		if code != 0 {
			t.Errorf("%q: exit code want 0 (leitura), got %d (stderr: %s)", cmd, code, stderr)
		}
	}
}

func TestGitBranchGuard_BranchDelete_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	for _, cmd := range []string{"git branch -d nome", "git branch -D nome"} {
		payload := `{"tool_input":{"command":"` + cmd + `"}}`
		code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
		if code != 0 {
			t.Errorf("%q: exit code want 0 (delete não cria branch), got %d (stderr: %s)", cmd, code, stderr)
		}
	}
}

func TestGitBranchGuard_WorktreeAddDashB_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git worktree add -b nova ../nova"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (git worktree add -b cria branch), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "trackfw branch new") {
		t.Errorf("mensagem deveria orientar para 'trackfw branch new', got: %s", stdout)
	}
}

func TestGitBranchGuard_WorktreeAddWithoutDashB_Allows(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git worktree add ../nova existing-branch"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (worktree add sem -b não cria branch), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_EnvWithVarAssignment_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"env FOO=bar git push"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (env FOO=bar git push é a mesma classe de env git push), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_EnvWithMultipleVarAssignments_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"env FOO=bar BAZ=qux git commit -m x"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (múltiplas atribuições antes de git), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_EnvWithFlag_StillEvades(t *testing.T) {
	// Declarado, não fechado: env com FLAG (não atribuição de variável) continua evadindo.
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"env -i git push"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Errorf("exit code: want 0 (env -i com flag continua fora do escopo declarado), got %d (stderr: %s)", code, stderr)
	}
}

// ---------------------------------------------------------------------------
// ML-1A (ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-
// independente-de-fiacao.md): no-op fora de projeto trackfw.
// ---------------------------------------------------------------------------

func TestGitBranchGuard_FixtureHasNoTrackfwYAMLAncestor(t *testing.T) {
	// Não-vacuidade: prova (em vez de presumir) que t.TempDir() não tem trackfw.yaml em
	// nenhum ancestral — se tivesse, os testes de no-op abaixo "passariam" pelo motivo
	// errado (não-detecção acidental, não no-op real).
	dir, _ := setupGitBranchGuardFixtureWithoutTrackfwYAML(t)
	d := dir
	for {
		if _, err := os.Stat(filepath.Join(d, "trackfw.yaml")); err == nil {
			t.Fatalf("premissa violada: %s tem trackfw.yaml em ancestral %s — fixture não isolada do repo real", dir, d)
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
}

func TestGitBranchGuard_NoTrackfwYAML_PushIsNoOp(t *testing.T) {
	dir, script := setupGitBranchGuardFixtureWithoutTrackfwYAML(t)
	payload := `{"tool_input":{"command":"git push"}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 0 {
		t.Fatalf("exit code: want 0 (sem trackfw.yaml, guard é no-op), got %d (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("no-op deveria ser silencioso, got: %s", stdout)
	}
}

func TestGitBranchGuard_NoTrackfwYAML_CommitAndCheckoutDashB_AreNoOp(t *testing.T) {
	dir, script := setupGitBranchGuardFixtureWithoutTrackfwYAML(t)
	for _, cmd := range []string{
		`git commit -m "x"`,
		"git checkout -b feat/x",
		"git branch nova",
		"git switch -c feat/x",
	} {
		payload := `{"tool_input":{"command":"` + jsonEscape(cmd) + `"}}`
		code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
		if code != 0 {
			t.Errorf("cmd=%q: exit code want 0 (sem trackfw.yaml, guard é no-op), got %d (stderr: %s)", cmd, code, stderr)
		}
	}
}

func TestGitBranchGuard_WithTrackfwYAML_PushStillBlocks(t *testing.T) {
	// Reverse-vacuity da bateria acima: MESMO fixture dir (com GenerateGitBranchGuardScript),
	// só que COM trackfw.yaml — prova que o 0 acima veio do no-op, não de um build quebrado.
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git push"}}`

	code, _, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (com trackfw.yaml, guard continua ativo), got %d (stderr: %s)", code, stderr)
	}
}

func TestGitBranchGuard_TrackfwYAMLInAncestor_SubdirectoryStillBlocks(t *testing.T) {
	// A raiz do projeto é encontrada SUBINDO diretórios — reproduz o agente rodando `git push`
	// de um subdiretório profundo do repo, não da raiz.
	dir, script := setupGitBranchGuardFixture(t)
	sub := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("erro criando subdiretório: %v", err)
	}
	payload := `{"tool_input":{"command":"git push"}}`

	code, _, stderr := runGitBranchGuard(t, sub, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2 (subdiretório de projeto trackfw continua protegido), got %d (stderr: %s)", code, stderr)
	}
}
