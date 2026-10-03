package generators

import (
	"encoding/json"
	"io"
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
	if execBitRepresentavelPara(t, path) {
		if info.Mode().Perm()&0100 == 0 {
			t.Errorf("script não é executável: mode=%v", info.Mode())
		}
	} else {
		execBitNaoExercitado(t, path)
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
	if execBitRepresentavelPara(t, path) {
		if info.Mode().Perm()&0100 == 0 {
			t.Errorf("script global não é executável: mode=%v", info.Mode())
		}
	} else {
		execBitNaoExercitado(t, path)
		// Único assert deste teste sobre o artefato: no ramo suprimido, medir o que É
		// representável em NTFS, em vez de não medir nada.
		if info.Size() == 0 {
			t.Errorf("script global está vazio: %s", path)
		}
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

// runGitBranchGuardImpl executa o guard com env explícito (nil = herdar do processo pai).
// É o helper interno usado por runGitBranchGuard e outros.
func runGitBranchGuardImpl(t *testing.T, dir, scriptPath string, args []string, stdin string, env []string) (exitCode int, stdout, stderr string) {
	t.Helper()
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	if env != nil {
		cmd.Env = env
	}
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

// makeEnvWithoutJQ devolve os.Environ() com PATH substituído pelo diretório curado
// sem jq — garante que o braço awk do extrator é exercitado no segundo modo.
// Pré-condição verificada em linha: command -v jq deve retornar vazio no PATH curado
// (macOS tem /usr/bin/jq; inclusão acidental de /usr/bin causaria falso-positivo).
func makeEnvWithoutJQ(t *testing.T) []string {
	t.Helper()
	fakeBinDir := makeCuratedPathWithoutJQ(t)
	assertJQAbsentInPath(t, fakeBinDir)
	result := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "PATH=") {
			result = append(result, e)
		}
	}
	return append(result, "PATH="+fakeBinDir)
}

// runGitBranchGuardBothModes executa o guard duas vezes: (a) com os.Environ()+extraEnv
// e (b) com makeEnvWithoutJQ+extraEnv. Se os exit codes divergirem, t.Errorf. Devolve (a).
// Usado por runGitBranchGuard (extraEnv nil) e por testes com variáveis de ambiente próprias
// (ex: TRACKFW_GIT_COMMAND) que precisam da mesma verificação de equivalência jq vs awk.
func runGitBranchGuardBothModes(t *testing.T, dir, scriptPath string, args []string, stdin string, extraEnv []string) (exitCode int, stdout, stderr string) {
	t.Helper()

	// (a) PATH do sistema — jq disponível.
	envA := append(os.Environ(), extraEnv...)
	exitCode, stdout, stderr = runGitBranchGuardImpl(t, dir, scriptPath, args, stdin, envA)

	// (b) PATH curado sem jq — exercita o extrator awk (D1/D2/D2-bis/D2-ter do ADR).
	// As demais variáveis de ambiente são preservadas; PATH é substituído pelo curado sem jq.
	noJQEnv := append(makeEnvWithoutJQ(t), extraEnv...)
	exitCodeB, _, stderrB := runGitBranchGuardImpl(t, dir, scriptPath, args, stdin, noJQEnv)

	if exitCode != exitCodeB {
		t.Errorf("divergência de rc entre PATH do sistema (rc=%d, stderr=%q) e PATH sem jq (rc=%d, stderr=%q)",
			exitCode, stderr, exitCodeB, stderrB)
	}

	return exitCode, stdout, stderr
}

// runGitBranchGuard executa o guard duas vezes — (a) com o PATH do sistema (jq disponível)
// e (b) com um PATH curado sem jq (extrator awk). Se os exit codes divergirem, t.Errorf
// reporta os dois códigos e os dois stderr. Devolve sempre o resultado de (a).
// ML-1B: AC4 — todos os testes TestGitBranchGuard* passam a afirmar a equivalência dos
// dois extratores (jq e awk) a cada invocação.
func runGitBranchGuard(t *testing.T, dir, scriptPath string, args []string, stdin string) (exitCode int, stdout, stderr string) {
	t.Helper()
	return runGitBranchGuardBothModes(t, dir, scriptPath, args, stdin, nil)
}

// --- Bloqueio: git commit ---------------------------------------------------

func TestGitBranchGuard_Commit_StdinJSON_ToolInputCommand_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_name":"Bash","tool_input":{"command":"git commit -m \"x\""}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"hookSpecificOutput"`) || !strings.Contains(stdout, `"permissionDecision":"deny"`) {
		t.Errorf("stdout deveria conter hookSpecificOutput/permissionDecision:deny (schema aceito pelo Claude Code — o formato antigo {\"decision\":\"block\"} é rejeitado com 'Hook JSON output validation failed'), got: %s", stdout)
	}
	if !strings.Contains(stdout, "trackfw commit") {
		t.Errorf("mensagem deveria orientar para 'trackfw commit', got: %s", stdout)
	}
	if !strings.Contains(stderr, "CLAUDE.md") {
		t.Errorf("mensagem deveria referenciar CLAUDE.md, got: %s", stderr)
	}
}

// TestGitBranchGuard_Blocks_EmitsValidHookSpecificOutputSchema afirma o AC1 do ROADMAP-2026-09-09-
// guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-nos-3-clis.md por EXECUÇÃO real do
// script gerado, decodificando o stdout como JSON estruturado (não substring) e conferindo os três
// campos que o schema DOCUMENTADO do Claude Code exige em `hookSpecificOutput` para PreToolUse —
// hookEventName, permissionDecision e permissionDecisionReason não-vazio. O formato anterior
// (`{"decision":"block","reason":"..."}`) falhava a validação do Claude Code na raiz do objeto
// ("(root): Invalid input"); este teste garante que a forma emitida hoje tem exatamente a forma
// aninhada que a documentação do Claude Code descreve. NÃO afirma que o Claude Code em runtime
// real aceitou este JSON — essa observação é separada (sessão manual, ver relatório do ML-1A) e
// não é reproduzível em CI.
func TestGitBranchGuard_Blocks_EmitsValidHookSpecificOutputSchema(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := `{"tool_input":{"command":"git commit -m \"x\""}}`

	code, stdout, stderr := runGitBranchGuard(t, dir, script, nil, payload)
	if code != 2 {
		t.Fatalf("exit code: want 2, got %d (stderr: %s)", code, stderr)
	}

	var parsed struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &parsed); err != nil {
		t.Fatalf("stdout não é o JSON esperado: %v (stdout: %s)", err, stdout)
	}
	if parsed.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName: want PreToolUse, got %q", parsed.HookSpecificOutput.HookEventName)
	}
	if parsed.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision: want deny, got %q", parsed.HookSpecificOutput.PermissionDecision)
	}
	if parsed.HookSpecificOutput.PermissionDecisionReason == "" {
		t.Errorf("permissionDecisionReason: want non-empty, got empty")
	}
}

func TestGitBranchGuard_Commit_Argv_Blocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)

	code, stdout, stderr := runGitBranchGuard(t, dir, script, []string{"git", "commit", "-m", "x"}, "")
	if code != 2 {
		t.Fatalf("exit code: want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"hookSpecificOutput"`) || !strings.Contains(stdout, `"permissionDecision":"deny"`) {
		t.Errorf("stdout deveria conter hookSpecificOutput/permissionDecision:deny, got: %s", stdout)
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
	// Afirma que TRACKFW_GIT_COMMAND=git commit -m x bloqueia com e sem jq no PATH —
	// a variável de ambiente é preservada nos dois modos pelo runGitBranchGuardBothModes.
	dir, script := setupGitBranchGuardFixture(t)

	exitCode, _, stderr := runGitBranchGuardBothModes(t, dir, script, nil, "", []string{"TRACKFW_GIT_COMMAND=git commit -m x"})
	if exitCode != 2 {
		t.Fatalf("exit code: want 2 (fallback de env var), got %d (stderr: %s)", exitCode, stderr)
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

// ---------------------------------------------------------------------------
// ML-1A (ROADMAP-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq):
// tabela C01–C22 com jq e sem jq (extrator awk) + prova de mordida.
// ADR: ADR-2026-10-02 — D1/D2/D2-bis/D2-ter.
// ---------------------------------------------------------------------------

// makeCuratedPathWithoutJQ devolve um valor de PATH derivado do PATH original:
// remove todos os diretórios que contêm um executável jq/jq.exe e, para cada
// ferramenta necessária ao guard que ficou órfã (só existia nesses diretórios
// removidos), cria um shim num diretório temporário próprio — symlink primeiro,
// cópia como fallback (symlink exige Developer Mode no Windows).
//
// Qualquer falha ao montar o ambiente chama t.Fatalf; nunca retorna PATH vazio
// em silêncio. Retorno: string pronta para "PATH=<valor>" (pode conter separadores
// de PATH — os.PathListSeparator — quando o shim dir foi necessário).
func makeCuratedPathWithoutJQ(t *testing.T) string {
	t.Helper()

	// Passo 1: identificar quais dirs do PATH contêm jq ou jq.exe.
	originalDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	jqDirs := make(map[string]bool)
	for _, dir := range originalDirs {
		for _, name := range []string{"jq", "jq.exe"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				jqDirs[dir] = true
				break
			}
		}
	}

	// Passo 2: PATH filtrado — sem os dirs que têm jq.
	var filteredDirs []string
	for _, dir := range originalDirs {
		if !jqDirs[dir] {
			filteredDirs = append(filteredDirs, dir)
		}
	}

	// Passo 3: para cada ferramenta de que o guard precisa, verificar se ainda é
	// localizável no PATH filtrado. Se não for, criar shim num diretório próprio.
	tools := []string{"bash", "awk", "sed", "head", "tr", "find"}
	shimDir := t.TempDir()
	shimUsed := false

	for _, tool := range tools {
		// Procurar o executável no PATH filtrado.
		foundInFiltered := false
		for _, dir := range filteredDirs {
			for _, name := range []string{tool, tool + ".exe"} {
				if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
					foundInFiltered = true
					break
				}
			}
			if foundInFiltered {
				break
			}
		}
		if foundInFiltered {
			continue
		}

		// Ferramenta não disponível no PATH filtrado — precisa de shim.
		binPath, err := exec.LookPath(tool)
		if err != nil {
			// Não encontrado em nenhum lugar do sistema: segue sem ele.
			continue
		}
		dest := filepath.Join(shimDir, filepath.Base(binPath))
		if err := os.Symlink(binPath, dest); err != nil {
			if isSymlinkPrivilegeError(err) {
				// Sem privilégio de symlink (Windows sem Developer Mode): cópia.
				if copyErr := copyExecutableFile(binPath, dest); copyErr != nil {
					t.Fatalf("makeCuratedPathWithoutJQ: shim para %q: symlink sem privilégio (%v); cópia também falhou (%v)",
						tool, err, copyErr)
				}
			} else {
				t.Fatalf("makeCuratedPathWithoutJQ: shim para %q: os.Symlink falhou (%v)", tool, err)
			}
		}
		shimUsed = true
	}

	// Passo 4: montar o valor final do PATH.
	filteredPATH := strings.Join(filteredDirs, string(os.PathListSeparator))
	if shimUsed {
		return shimDir + string(os.PathListSeparator) + filteredPATH
	}
	return filteredPATH
}

// copyExecutableFile copia src para dst com permissão 0755.
// Usado como fallback quando os.Symlink falha (Windows sem Developer Mode).
func copyExecutableFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// runGitBranchGuardWithEnv executa o guard com env explícito (para PATH curado sem jq).
// Wrapper de runGitBranchGuardImpl com args=nil e ordem (env, stdin) preservada para os chamadores.
func runGitBranchGuardWithEnv(t *testing.T, dir, scriptPath string, env []string, stdin string) (exitCode int, stdout, stderr string) {
	t.Helper()
	return runGitBranchGuardImpl(t, dir, scriptPath, nil, stdin, env)
}

// assertJQAbsentInPath verifica que `command -v jq` falha no fakeBinDir —
// pré-condição obrigatória do braço sem jq (macOS ships /usr/bin/jq).
func assertJQAbsentInPath(t *testing.T, fakeBinDir string) {
	t.Helper()
	cmd := exec.Command("bash", "-c", "command -v jq")
	cmd.Env = []string{"PATH=" + fakeBinDir}
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("pré-condição violada: jq foi encontrado no PATH curado (%q). "+
			"O braço sem-jq não está isolado — mac /usr/bin/jq vazou para o PATH.", strings.TrimSpace(string(out)))
	}
}

// guardCasesTable retorna os 31 casos da tabela de testes do guard:
// C01–C22 (22 casos da Wave 0, incluindo NUL) e N01–N09 (9 casos de limites e escapes).
// Payloads são raw strings Go: \n, \t, \u000a, etc. são os literais JSON correspondentes
// (dois chars, não sequências de escape Go).
func guardCasesTable() []struct {
	id      string
	payload string
	wantRC  int
} {
	c10payload := "{\n\"tool_name\":\"Bash\",\n\n\"tool_input\":{\"command\":\"git push origin main\"}}"
	return []struct {
		id      string
		payload string
		wantRC  int
	}{
		// C01: baseline — git push simples é bloqueado.
		{id: "C01", payload: `{"tool_input":{"command":"git push origin main"}}`, wantRC: 2},
		// C02: JSON \n decodificado para newline separa segmentos; git push na linha 2 bloqueia (Forma A).
		{id: "C02", payload: `{"tool_input":{"command":"echo oi\ngit push origin main"}}`, wantRC: 2},
		// C03: JSON \" decodificado para aspa dupla; && git push visível após a aspa fechante (Forma B).
		{id: "C03", payload: `{"tool_input":{"command":"echo \"hello\" && git push origin main"}}`, wantRC: 2},
		// C04: JSON \t decodificado para tab; bash tokeniza git<TAB>push como git + push (Forma C).
		{id: "C04", payload: `{"tool_input":{"command":"git\tpush origin main"}}`, wantRC: 2},
		// C05: \u000a é newline (mesmo mecanismo de C02); bloqueia (Forma D).
		{id: "C05", payload: `{"tool_input":{"command":"echo oi\u000agit push origin main"}}`, wantRC: 2},
		// C06: \\n é backslash+n literal (não newline); todo o argumento fica dentro das aspas; permite.
		{id: "C06", payload: `{"tool_input":{"command":"echo \"a\\ngit push origin main\""}}`, wantRC: 0},
		// C07: git commit com -m "a\\nb" (backslash+n literal no argumento); primeiro token git commit; bloqueia.
		{id: "C07", payload: `{"tool_input":{"command":"git commit -m \"a\\nb\""}}`, wantRC: 2},
		// C08: chave command está em valor de string irmã (description); o campo command real é echo safe; permite.
		{id: "C08", payload: `{"tool_input":{"description":"git push","command":"echo safe"}}`, wantRC: 0},
		// C09: chave command duplicada — last-wins extrai git push; bloqueia (D2-ter: last-wins).
		{id: "C09", payload: `{"command":"echo safe","command":"git push"}`, wantRC: 2},
		// C10: linha em branco no JSON antes de command — awk acumula em END e não parte o payload (D2-ter: sem RS="").
		{id: "C10", payload: c10payload, wantRC: 2},
		// C11: flat command tem prioridade menor que tool_input.command; bloqueia pelo campo de maior prioridade.
		{id: "C11", payload: `{"command":"echo safe","tool_input":{"command":"git push"}}`, wantRC: 2},
		// C12: flat command tem prioridade maior que tool_info.command_line; git push no flat command bloqueia.
		{id: "C12", payload: `{"command":"git push","tool_info":{"command_line":"echo"}}`, wantRC: 2},
		// C13: surrogate \ud83d converte para marcador; "git push" no comando bloqueia.
		{id: "C13", payload: `{"tool_input":{"command":"git push \ud83d"}}`, wantRC: 2},
		// C14: evasão unicode — \u0067 = 'g'; o extrator decodifica \u0067it push origin main para "git push origin main" e bloqueia.
		{id: "C14", payload: `{"tool_input":{"command":"\u0067it push origin main"}}`, wantRC: 2},
		// C15: echo hello world — não é comando git; permite.
		{id: "C15", payload: `{"tool_input":{"command":"echo hello world"}}`, wantRC: 0},
		// C16: git status é read-only permitido; permite.
		{id: "C16", payload: `{"tool_input":{"command":"git status"}}`, wantRC: 0},
		// C17: payload sem chave command (tipo Read tool); permite (chave ausente → nada a bloquear).
		{id: "C17", payload: `{"tool_name":"Read","tool_input":{"path":"/foo/bar"}}`, wantRC: 0},
		// C18: string JSON não-terminada — extrator detecta unterminated_string; recusa (D2: indecodificável → fail-closed).
		{id: "C18", payload: `{"tool_input":{"command":"git push origin main`, wantRC: 2},
		// C19: tool_input aninhado em sub-objeto não conta como depth=2; o tool_input raiz com git push bloqueia.
		{id: "C19", payload: `{"meta":{"tool_input":{"command":"echo safe"}},"tool_input":{"command":"git push"}}`, wantRC: 2},
		// C20: trailing backslash — echo foo\ é string válida, D2 não dispara; permite.
		{id: "C20", payload: `{"tool_input":{"command":"echo foo\\"}}`, wantRC: 0},
		// C21: \u000d é CR — não é separador de segmento; echo oi<CR>git push permanece um segmento; primeiro token echo; permite.
		{id: "C21", payload: `{"tool_input":{"command":"echo oi\u000dgit push origin main"}}`, wantRC: 0},
		// C22: \u0000 (NUL) no valor — D2-bis recusa em ambos os caminhos (jq e awk) antes de $() descartar o NUL.
		{id: "C22", payload: `{"tool_input":{"command":"git push\u0000origin main"}}`, wantRC: 2},
		// N01: \u com 3 dígitos hex no valor (escape incompleto) — awk detecta incomplete_unicode → exit 2; jq falha o parse → fallback awk → mesmo resultado.
		{id: "N01", payload: `{"tool_input":{"command":"git push\u00a"}}`, wantRC: 2},
		// N02: \uzzzz (4 dígitos não-hex) no valor — hex2dec retorna -1 → invalid_unicode_hex → exit 2 nos dois caminhos.
		{id: "N02", payload: `{"tool_input":{"command":"git push\uzzzz"}}`, wantRC: 2},
		// N03: chave "command" com escape unicode no nome (comm\u0061nd decodifica para "command") — após o fix, awk decodifica o nome antes de comparar → bloqueia nos dois caminhos.
		{id: "N03", payload: `{"tool_input":{"comm` + `\u0061nd":"git push origin main"}}`, wantRC: 2},
		// N04: valor numérico 123 — nem jq nem awk tratam número como string → sem comando detectado → permite.
		{id: "N04", payload: `{"tool_input":{"command":123}}`, wantRC: 0},
		// N05: valor null — token null não é string JSON → sem comando detectado → permite.
		{id: "N05", payload: `{"tool_input":{"command":null}}`, wantRC: 0},
		// N06: valor array — [ inicia array (não objeto); strings dentro não ativam captura de comando → permite.
		{id: "N06", payload: `{"tool_input":{"command":["git","push","origin","main"]}}`, wantRC: 0},
		// N07: tool_input como string (não objeto) — awk não encontra chave command no nível esperado → permite.
		{id: "N07", payload: `{"tool_input":"git push origin main"}`, wantRC: 0},
		// N08: string sem aspas de fechamento (50 000 bytes) — awk chega ao END com unterminated_string → exit 2; jq falha o parse → fallback awk → mesmo resultado.
		{id: "N08", payload: `{"tool_input":{"command":"` + strings.Repeat("A", 50000), wantRC: 2},
		// N09: 10 000 contrabarras (5 000 \\ no JSON) + "git push" no valor — valor decodificado começa com \\ × 5000 (não com "git"); comando não bloqueado → permite.
		{id: "N09", payload: `{"tool_input":{"command":"` + strings.Repeat(`\\`, 5000) + `git push"}}`, wantRC: 0},
	}
}

// TestGitBranchGuardAwk_C01C22_WithJQ roda a tabela C01–C22 + N01–N09 com jq disponível
// (PATH do sistema — macOS tem /usr/bin/jq).
// AC4: mesma tabela nos dois caminhos.
func TestGitBranchGuardAwk_C01C22_WithJQ(t *testing.T) {
	// Pré-condição: jq deve estar no sistema.
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq não encontrado no PATH do sistema — pulando tabela com-jq")
	}
	dir, script := setupGitBranchGuardFixture(t)
	for _, tc := range guardCasesTable() {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			code, _, stderr := runGitBranchGuard(t, dir, script, nil, tc.payload)
			if code != tc.wantRC {
				t.Errorf("%s (+jq): rc want %d, got %d (stderr: %s)", tc.id, tc.wantRC, code, stderr)
			}
		})
	}
}

// TestGitBranchGuardAwk_C01C22_WithoutJQ roda a tabela C01–C22 + N01–N09 com PATH curado
// que exclui jq — exercita o extrator awk (D1/D2/D2-bis/D2-ter).
// AC2/AC3/AC4/AC5: cobre multilinha, \", literal \\n, indecodificável e NUL sem jq.
// Pré-condição verificada: command -v jq deve retornar vazio no PATH curado.
func TestGitBranchGuardAwk_C01C22_WithoutJQ(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	fakeBinDir := makeCuratedPathWithoutJQ(t)
	assertJQAbsentInPath(t, fakeBinDir)
	env := []string{"PATH=" + fakeBinDir}
	for _, tc := range guardCasesTable() {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			code, _, stderr := runGitBranchGuardWithEnv(t, dir, script, env, tc.payload)
			if code != tc.wantRC {
				t.Errorf("%s (-jq/awk): rc want %d, got %d (stderr: %s)", tc.id, tc.wantRC, code, stderr)
			}
		})
	}
}

// TestGitBranchGuardAwk_ProvaDeMordida verifica que o script de 3b2eff09 falha
// aberto (rc=0) nos casos onde o novo extrator awk bloqueia (rc=2) — prova que o
// fallback antigo (sed) estava quebrado e que os novos testes não são vacuosos.
// Casos medidos: C02-C05 (Formas A-D), C12 (prioridade invertida), C22 (NUL).
// Observação: C09 e C11 NÃO falham abertos com o sed antigo (greedy/backtrack
// encontra o campo correto) — medido com o script real, não inferido.
func TestGitBranchGuardAwk_ProvaDeMordida(t *testing.T) {
	// Obter o script antigo diretamente do objeto git.
	oldScriptBytes, err := exec.Command("git", "-C",
		"/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw",
		"show", "3b2eff09:scripts/trackfw-git-branch-guard.sh").Output()
	if err != nil {
		t.Skipf("git show 3b2eff09: não disponível (%v) — pulando prova de mordida", err)
	}

	// Escrever o script antigo em arquivo temporário.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("project_name: fixture\n"), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml: %v", err)
	}
	oldScript := filepath.Join(dir, "old-guard.sh")
	if err := os.WriteFile(oldScript, oldScriptBytes, 0755); err != nil {
		t.Fatalf("WriteFile old-guard.sh: %v", err)
	}

	fakeBinDir := makeCuratedPathWithoutJQ(t)
	assertJQAbsentInPath(t, fakeBinDir)
	env := []string{"PATH=" + fakeBinDir}

	// Casos que o sed antigo NÃO conseguia bloquear (fail-open = bug):
	cases := []struct {
		id      string
		payload string
	}{
		// C02: JSON \n — sed não decodifica \n, extrai "echo oi\ngit push" como um token cujo primeiro elem é "echo".
		{id: "C02", payload: `{"tool_input":{"command":"echo oi\ngit push origin main"}}`},
		// C03: JSON \" — [^"]* para na aspa escapada, extrai "echo \" sem o git push.
		{id: "C03", payload: `{"tool_input":{"command":"echo \"hello\" && git push origin main"}}`},
		// C04: JSON \t — sed não decodifica \t; "git\tpush" não é reconhecido como "git" + "push".
		{id: "C04", payload: `{"tool_input":{"command":"git\tpush origin main"}}`},
		// C05: \u000a — mesmo mecanismo de C02; sed não decodifica unicode.
		{id: "C05", payload: `{"tool_input":{"command":"echo oi\u000agit push origin main"}}`},
		// C12: prioridade invertida — sed antigo extrai tool_info.command_line ("echo") quando
		// flat command ("git push") deveria ter prioridade maior.
		{id: "C12", payload: `{"command":"git push","tool_info":{"command_line":"echo"}}`},
		// C22: NUL — $() descarta o NUL, "git pushorigin" não é reconhecido como "git push".
		{id: "C22", payload: `{"tool_input":{"command":"git push\u0000origin main"}}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			// O script ANTIGO deve retornar rc=0 (fail-open = bug que estamos corrigindo).
			code, _, stderr := runGitBranchGuardWithEnv(t, dir, oldScript, env, tc.payload)
			if code != 0 {
				t.Errorf("prova de mordida %s: old script (3b2eff09) esperado rc=0 (fail-open), got rc=%d (stderr: %s)", tc.id, code, stderr)
			}
		})
	}
}
