package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1B.
// ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
// estrita-entre-head-e-disco.md (Emenda 3).

// TestCleanGitEnv_RemoveApenasPrefixoGIT prova o comportamento do filtro em isolamento: toda
// variável GIT_* desaparece, nenhuma outra é afetada — inclusive uma que apenas CONTÉM "GIT_" fora
// do início do nome (não deve ser removida por acidente de substring).
func TestCleanGitEnv_RemoveApenasPrefixoGIT(t *testing.T) {
	t.Setenv("GIT_DIR", "/tmp/whatever")
	t.Setenv("GIT_CONFIG_COUNT", "abc")
	t.Setenv("MY_GIT_DIR_LOOKALIKE", "kept") // não começa com GIT_, não deve ser removida
	t.Setenv("PATH_UNRELATED", "kept")

	cleaned := cleanGitEnv()

	for _, kv := range cleaned {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "GIT_") {
			t.Errorf("cleanGitEnv() não deveria manter variável GIT_*, encontrou: %s", key)
		}
	}

	hasKey := func(name string) bool {
		for _, kv := range cleaned {
			if strings.HasPrefix(kv, name+"=") {
				return true
			}
		}
		return false
	}
	if !hasKey("MY_GIT_DIR_LOOKALIKE") {
		t.Errorf("MY_GIT_DIR_LOOKALIKE não começa com GIT_ — não deveria ter sido removida")
	}
	if !hasKey("PATH_UNRELATED") {
		t.Errorf("PATH_UNRELATED não deveria ter sido removida")
	}
}

// setupHeadBlockDiskWarnFixture cria um repo git com credential_guard.mode: block commitado no
// HEAD e credential_guard.mode: warn não commitado em disco — o cenário decisivo do ADR M4
// (credential_guard_mode_downgrade deve disparar). Usado como base para os testes de bypass por
// ambiente abaixo.
func setupHeadBlockDiskWarnFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
	writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)
	return dir
}

// TestCredentialGuardModeDowngrade_GitDirWorkTreeRedirecionados_ContinuaDetectando é o PoC do
// ML-3B/Zeus reproduzido como teste automatizado: GIT_DIR/GIT_WORK_TREE apontando para outro
// repositório git (sem trackfw.yaml versionado) não pode mais fazer headTrackfwYAML() falhar em
// silêncio — a violação precisa continuar aparecendo.
func TestCredentialGuardModeDowngrade_GitDirWorkTreeRedirecionados_ContinuaDetectando(t *testing.T) {
	setupHeadBlockDiskWarnFixture(t)

	other := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = other
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	run("commit", "--allow-empty", "-m", "init")

	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "credential_guard.mode: block") {
		t.Fatalf("GIT_DIR/GIT_WORK_TREE redirecionados NÃO deveriam silenciar a detecção, obteve: %v", msgs)
	}
}

// TestCredentialGuardModeDowngrade_GitConfigCountMalformado_ContinuaDetectando prova o achado
// além do PoC original: GIT_CONFIG_COUNT malformado (fora da lista fechada considerada
// inicialmente) faz `git rev-parse` sair com "fatal: unable to parse command-line config" — se
// essa falha não for isolada do ambiente do subprocesso, headTrackfwYAML() retorna ok=false e
// validateCredentialGuardModeDowngrade silencia POR INTEIRO (não só a severidade). Ver o comentário
// de gitEnvPrefix em validator_git_exec.go para o raciocínio completo por trás da virada de
// enumeração positiva para prefixo negativo.
func TestCredentialGuardModeDowngrade_GitConfigCountMalformado_ContinuaDetectando(t *testing.T) {
	setupHeadBlockDiskWarnFixture(t)

	t.Setenv("GIT_CONFIG_COUNT", "abc")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "credential_guard.mode: block") {
		t.Fatalf("GIT_CONFIG_COUNT malformado NÃO deveria silenciar a detecção, obteve: %v", msgs)
	}
}

// TestCredentialGuardModeDowngrade_GitConfigCountMalformado_NaoVacuidade prova que o teste acima
// não é vácuo: sem a limpeza de ambiente (chamando git diretamente, sem passar por gitCommand), a
// mesma variável FAZ o comando falhar — confirmando que o cenário de ataque é real e que é
// gitCommand()/cleanGitEnv() que o neutraliza, não alguma outra particularidade do fixture.
func TestCredentialGuardModeDowngrade_GitConfigCountMalformado_NaoVacuidade(t *testing.T) {
	dir := setupHeadBlockDiskWarnFixture(t)

	cmd := exec.Command("git", "-C", dir, "rev-parse", "--verify", "HEAD")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_COUNT=abc")
	if err := cmd.Run(); err == nil {
		t.Fatalf("esperava que git falhasse com GIT_CONFIG_COUNT=abc herdado sem limpeza — não falhou, o fixture não prova nada")
	}
}

// TestIsGitWorktree_LinkedWorktreeLegitimo_ContinuaFuncionando prova que a correção não quebra o
// uso legítimo mencionado nos pontos de atenção do despacho: um `git worktree add` real (sem
// nenhuma manipulação de ambiente) continua sendo reconhecido como worktree, e o mecanismo M4
// continua funcionando dentro dele.
func TestIsGitWorktree_LinkedWorktreeLegitimo_ContinuaFuncionando(t *testing.T) {
	mainDir := t.TempDir()
	initGitRepo(t, mainDir, "main")
	commitTrackfwYAML(t, mainDir, "credential_guard:\n  mode: block\n")

	linkedDir := filepath.Join(t.TempDir(), "linked")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = mainDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("worktree", "add", "-b", "feat/linked-worktree-test", linkedDir)

	chdir(t, linkedDir)
	t.Cleanup(config.Reset)

	if !isGitWorktree(".") {
		t.Fatalf("linked worktree deveria ser reconhecida como git worktree")
	}

	// Disco na worktree ainda resolve para block (idêntico ao HEAD) — sem downgrade.
	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("worktree legítima sem downgrade não deveria disparar, obteve: %v", msgs)
	}

	// Agora introduz o downgrade em disco DENTRO da worktree — deve disparar normalmente.
	writeFile(t, linkedDir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
	msgs, err = validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "credential_guard.mode: block") {
		t.Fatalf("downgrade dentro de worktree legítima deveria disparar, obteve: %v", msgs)
	}
}
