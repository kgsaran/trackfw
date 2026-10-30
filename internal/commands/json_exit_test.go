package commands

// json_exit_test.go — contrato de saída silenciosa do stream --json
// (ML-1A + ML-3A, REQ-2026-09-07, AC1/AC1-bis/AC7/AC8).
//
// Dois tipos de teste:
//
// 1. Subprocess (exec.Command, rootBinary): AC1, AC1-bis, AC7, AC8. Obrigatório
//    para esses casos porque Execute() escreve em os.Stderr diretamente e chama
//    os.Exit; nenhuma dessas operações é capturável in-process.
//
// 2. In-process (contra decideExit): ML-3A. decideExit é a função pura extraída
//    de Execute() — recebe (*cobra.Command, error) e devolve (code, stderr).
//    Testa a lógica de decisão sem os.Exit nem proceso filho.
//    TestJSONExit_DecideExit e TestJSONExit_NewExitCodeError_OutOfRangePanics
//    são os únicos testes in-process deste arquivo.
//
// Regra Dura de Reconciliação: cada teste declara, em comentário, a conclusão do
// ML que ele afirma.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setupValidateViolationFixtureDir cria um diretório temporário com uma
// violação conhecida (roadmap em wip/ sem REQ vinculada) e um trackfw.yaml
// minimal. Usa t.TempDir() para que o cleanup seja automático.
func setupValidateViolationFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dirs := []string{
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
		"docs/roadmaps/blocked",
		"docs/req",
		"docs/adr",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("setupValidateViolationFixtureDir: mkdirs: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\n",
	), 0644); err != nil {
		t.Fatalf("setupValidateViolationFixtureDir: trackfw.yaml: %v", err)
	}
	// Roadmap sem REQ vinculada → violação wip_has_req.
	if err := os.WriteFile(filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-no-req.md"), []byte(
		"# Roadmap: No REQ\n\n## Wave 1\n",
	), 0644); err != nil {
		t.Fatalf("setupValidateViolationFixtureDir: roadmap: %v", err)
	}
	// git repository sem commits — o validator lê do CWD e precisa de um repo.
	initGitDir(t, dir)
	return dir
}

// setupUpdateFailFixtureDir cria um diretório de projeto com CLAUDE.md sem
// permissão de escrita (chmod 444) para que `update --targets agent-rules`
// falhe com TargetFailed (falha de I/O, não pathguard). HOME separado evita
// que o update toque no home real do desenvolvedor.
// Retorna (projectDir, fakeHome).
func setupUpdateFailFixtureDir(t *testing.T) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 444 não bloqueia escrita no Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignora bits de permissão — chmod 444 não produz falha")
	}
	dir := t.TempDir()
	dirs := []string{
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
		"docs/roadmaps/blocked",
		"docs/req",
		"docs/adr",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("setupUpdateFailFixtureDir: mkdirs: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\n",
	), 0644); err != nil {
		t.Fatalf("setupUpdateFailFixtureDir: trackfw.yaml: %v", err)
	}
	claude := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(claude, []byte("# CLAUDE.md\n"), 0444); err != nil {
		t.Fatalf("setupUpdateFailFixtureDir: CLAUDE.md: %v", err)
	}
	// Restaurar permissão no cleanup para que t.TempDir RemoveAll funcione.
	t.Cleanup(func() { _ = os.Chmod(claude, 0644) })
	initGitDir(t, dir)
	fakeHome := t.TempDir()
	return dir, fakeHome
}

// setupHarnessFailFixtureDir cria um HOME com .trackfw/ em chmod 555 para
// que `update harness --targets git-branch-guard-script` falhe com TargetFailed
// por permissão de diretório.
// Retorna fakeHome com .trackfw/ já bloqueado.
func setupHarnessFailFixtureDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 555 não bloqueia criação de subdiretório no Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignora bits de permissão — chmod 555 não produz falha")
	}
	home := t.TempDir()
	trackfwDir := filepath.Join(home, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0555); err != nil {
		t.Fatalf("setupHarnessFailFixtureDir: mkdir .trackfw: %v", err)
	}
	// Restaurar permissão no cleanup para que t.TempDir RemoveAll funcione.
	t.Cleanup(func() { _ = os.Chmod(trackfwDir, 0755) })
	return home
}

// initGitDir inicializa um repositório git mínimo no dir usando
// git symbolic-ref HEAD (sem commit) — o guard do trackfw bloqueia git commit.
func initGitDir(t *testing.T, dir string) {
	t.Helper()
	// git init
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// Aponta HEAD para feat/x sem fazer commit — o validator e o update não
	// exigem commits.
	if err := os.WriteFile(
		filepath.Join(dir, ".git", "HEAD"),
		[]byte("ref: refs/heads/feat/x\n"),
		0644,
	); err != nil {
		t.Fatalf("initGitDir: write HEAD: %v", err)
	}
}

// jsonExitEnv retorna o ambiente mínimo para rodar o binário trackfw em
// fixtures de subprocess: HOME/USERPROFILE apontam para fakeHome, LANG=C
// fixa o locale en-US (garante literais como "violation(s) found"), e PATH
// inclui as ferramentas básicas.
func jsonExitEnv(fakeHome string) []string {
	goCache := os.Getenv("GOCACHE")
	goModCache := os.Getenv("GOMODCACHE")
	goPath := os.Getenv("GOPATH")

	env := []string{
		"HOME=" + fakeHome,
		"USERPROFILE=" + fakeHome,
		"LANG=C",
		"PATH=" + os.Getenv("PATH"),
		"GIT_CONFIG_PARAMETERS=" + os.Getenv("GIT_CONFIG_PARAMETERS"),
	}
	if goCache != "" {
		env = append(env, "GOCACHE="+goCache)
	}
	if goModCache != "" {
		env = append(env, "GOMODCACHE="+goModCache)
	}
	if goPath != "" {
		env = append(env, "GOPATH="+goPath)
	}
	return env
}

// runBin executa o binário trackfw com os args no dir e retorna (stdout, stderr, exitCode).
func runBin(t *testing.T, bin, dir, fakeHome string, args ...string) ([]byte, []byte, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = jsonExitEnv(fakeHome)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()
	rc := 0
	if cmd.ProcessState != nil {
		rc = cmd.ProcessState.ExitCode()
	}
	return stdout.Bytes(), stderr.Bytes(), rc
}

// ─── AC1 ──────────────────────────────────────────────────────────────────────

// TestJSONExit_ValidateJSON_ViolationSilentStderr afirma que, com violações,
// `validate --json` sai rc=1, stdout contém JSON válido e stderr tem 0 bytes.
//
// Conclusão do ML que este teste afirma: depois do fix, o ramo --json de
// validate.go devolve *ExitCodeError em vez de fmt.Errorf, e o root.go sai
// silenciosamente — nenhum byte de root.go aparece no stderr.
func TestJSONExit_ValidateJSON_ViolationSilentStderr(t *testing.T) {
	bin := rootBinary(t)
	dir := setupValidateViolationFixtureDir(t)
	fakeHome := t.TempDir()

	stdout, stderr, rc := runBin(t, bin, dir, fakeHome, "validate", "--json")

	if rc != 1 {
		t.Errorf("validate --json com violação: esperado rc=1, obteve rc=%d\nstderr: %s", rc, stderr)
	}
	if !json.Valid(bytes.TrimSpace(stdout)) {
		t.Errorf("validate --json: stdout não é JSON válido:\n%s", stdout)
	}
	if len(stderr) != 0 {
		t.Errorf("validate --json com violação: esperado stderr=0 B, obteve %d B: %q", len(stderr), stderr)
	}
	// Sanidade: o documento deve reportar violações.
	var result struct {
		Summary struct {
			Violations int `json:"violations"`
			ExitCode   int `json:"exit_code"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &result); err != nil {
		t.Fatalf("validate --json: unmarshal falhou: %v", err)
	}
	if result.Summary.Violations == 0 {
		t.Error("validate --json: esperado violations > 0 no fixture de violação")
	}
	if result.Summary.ExitCode != 1 {
		t.Errorf("validate --json: esperado exit_code=1 no documento, obteve %d", result.Summary.ExitCode)
	}
}

// ─── AC1-bis (update) ─────────────────────────────────────────────────────────

// TestJSONExit_UpdateJSON_TargetFailedSilentStderr afirma que, com alvo falho,
// `update --json --targets agent-rules` sai rc=1, stdout contém JSON válido
// e stderr tem 0 bytes (nenhuma linha de root.go).
//
// Conclusão do ML que este teste afirma: depois do fix, o ramo --json de
// update.go devolve *ExitCodeError em vez de fmt.Errorf, e o root.go sai
// silenciosamente — nenhum byte de root.go aparece no stderr.
func TestJSONExit_UpdateJSON_TargetFailedSilentStderr(t *testing.T) {
	bin := rootBinary(t)
	dir, fakeHome := setupUpdateFailFixtureDir(t)

	stdout, stderr, rc := runBin(t, bin, dir, fakeHome, "update", "--json", "--targets", "agent-rules")

	if rc != 1 {
		t.Errorf("update --json com target falho: esperado rc=1, obteve rc=%d\nstderr: %s", rc, stderr)
	}
	if !json.Valid(bytes.TrimSpace(stdout)) {
		t.Errorf("update --json: stdout não é JSON válido:\n%s", stdout)
	}
	if len(stderr) != 0 {
		t.Errorf("update --json com target falho: esperado stderr=0 B, obteve %d B: %q", len(stderr), stderr)
	}
	// Sanidade: o documento deve reportar failed >= 1.
	var result struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &result); err != nil {
		t.Fatalf("update --json: unmarshal falhou: %v", err)
	}
	if result.Summary.Failed < 1 {
		t.Errorf("update --json: esperado summary.failed >= 1, obteve %d", result.Summary.Failed)
	}
}

// ─── AC1-bis (update harness) ────────────────────────────────────────────────

// TestJSONExit_UpdateHarnessJSON_TargetFailedSilentStderr afirma que, com alvo
// falho, `update harness --json --targets git-branch-guard-script` sai rc=1,
// stdout contém JSON válido e stderr tem 0 bytes (nenhuma linha de root.go).
//
// Conclusão do ML que este teste afirma: depois do fix, o ramo --json de
// update_harness.go devolve *ExitCodeError em vez de fmt.Errorf, e o root.go
// sai silenciosamente — nenhum byte de root.go aparece no stderr.
func TestJSONExit_UpdateHarnessJSON_TargetFailedSilentStderr(t *testing.T) {
	bin := rootBinary(t)
	fakeHome := setupHarnessFailFixtureDir(t)
	workDir := t.TempDir() // update harness não precisa de projeto, mas Dir deve ser válido

	stdout, stderr, rc := runBin(t, bin, workDir, fakeHome, "update", "harness", "--json", "--targets", "git-branch-guard-script")

	if rc != 1 {
		t.Errorf("update harness --json com target falho: esperado rc=1, obteve rc=%d\nstderr: %s", rc, stderr)
	}
	if !json.Valid(bytes.TrimSpace(stdout)) {
		t.Errorf("update harness --json: stdout não é JSON válido:\n%s", stdout)
	}
	if len(stderr) != 0 {
		t.Errorf("update harness --json com target falho: esperado stderr=0 B, obteve %d B: %q", len(stderr), stderr)
	}
	// Sanidade: o documento deve reportar failed >= 1.
	var result struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &result); err != nil {
		t.Fatalf("update harness --json: unmarshal falhou: %v", err)
	}
	if result.Summary.Failed < 1 {
		t.Errorf("update harness --json: esperado summary.failed >= 1, obteve %d", result.Summary.Failed)
	}
}

// ─── AC7 (modo texto inalterado) ─────────────────────────────────────────────

// TestJSONExit_ValidateText_StderrContainsError afirma que o modo texto do
// validate com violações continua escrevendo "Error: N violation(s) found"
// no stderr (saída ruidosa inalterada pelo fix).
//
// Conclusão do ML que este teste afirma: o ramo texto de validate.go não foi
// alterado — fmt.Errorf continua sendo devolvido e root.go continua imprimindo.
func TestJSONExit_ValidateText_StderrContainsError(t *testing.T) {
	bin := rootBinary(t)
	dir := setupValidateViolationFixtureDir(t)
	fakeHome := t.TempDir()

	_, stderr, rc := runBin(t, bin, dir, fakeHome, "validate")

	if rc != 1 {
		t.Errorf("validate (texto) com violação: esperado rc=1, obteve rc=%d\nstderr: %s", rc, stderr)
	}
	if !strings.Contains(string(stderr), "violation(s) found") {
		t.Errorf("validate (texto) com violação: esperado 'violation(s) found' em stderr, obteve: %q", stderr)
	}
	// O prefixo "Error: " deve estar presente (root.go não silencia modo texto).
	if !strings.Contains(string(stderr), "Error:") {
		t.Errorf("validate (texto): esperado prefixo 'Error:' em stderr, obteve: %q", stderr)
	}
}

// TestJSONExit_ValidateLenient_StderrEmpty afirma que o modo lenient do
// validate (rc=0) não escreve nada no stderr, independente do fix.
//
// Conclusão do ML que este teste afirma: o modo lenient não atinge o caminho
// de erro do validate.go — o fix não altera esse caminho.
func TestJSONExit_ValidateLenient_StderrEmpty(t *testing.T) {
	bin := rootBinary(t)
	dir := setupValidateViolationFixtureDir(t)
	fakeHome := t.TempDir()

	// Escrever trackfw.yaml com governance_mode: lenient e lenient_until dentro
	// do horizonte de 730 dias.
	_ = os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(fmt.Sprintf(
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\nroadmap_namespacing: flat\ngovernance_mode: lenient\nlenient_until: %s\n",
		"2027-12-31", // dentro de 730 dias a partir de 2026-10-30
	)), 0644)

	_, stderr, rc := runBin(t, bin, dir, fakeHome, "validate", "--json")

	if rc != 0 {
		t.Errorf("validate --json lenient: esperado rc=0, obteve rc=%d\nstderr: %s", rc, stderr)
	}
	if len(stderr) != 0 {
		t.Errorf("validate --json lenient: esperado stderr=0 B, obteve %d B: %q", len(stderr), stderr)
	}
}

// ─── ML-3A: decideExit in-process table tests ────────────────────────────────

// TestJSONExit_DecideExit afirma o comportamento completo de decideExit contra
// uma árvore cobra mínima, sem processo filho nem os.Exit.
//
// Cada caso declara a conclusão que afirma (Regra Dura de Reconciliação):
//
//   - "silent Code=1 non-guard": ExitCodeError válido (Code in [1,255]) no caminho
//     não-guard é reconhecido por direct assertion e sai silencioso (code=1, msg="").
//
//   - "noisy zero-value non-guard": struct literal Code=0 não passa o range check
//     [1,255] em decideExit — cai no caminho ruidoso (code=1, msg!="").
//     Detecta mutação D (remoção do range check).
//
//   - "noisy Code=256 non-guard": Code=256 não passa o range check — caminho
//     ruidoso (code=1, msg!=""). Detecta mutação D.
//
//   - "noisy wrapped non-guard": fmt.Errorf("%w", *ExitCodeError) não é reconhecido
//     pela direct assertion — caminho ruidoso (code=1, msg!=""). Detecta mutação B
//     (troca por errors.As: com errors.As, o erro embrulhado seria silenciado).
//
//   - "noisy Code=1 under guard rc=2": !isCommandUnderGuard bloqueia o ramo
//     silencioso para guard — D7 aplica (code=2, msg!=""). Detecta mutação C
//     (remoção de !isCommandUnderGuard: sem a guarda, o erro seria silenciado com code=1).
//
//   - "noisy common error non-guard": erro simples não-ExitCodeError segue o
//     caminho ruidoso (code=1, msg!="").
func TestJSONExit_DecideExit(t *testing.T) {
	// Árvore não-guard: trackfw → validate (SilenceUsage=true, espelha PersistentPreRunE).
	nonGuardRoot := &cobra.Command{Use: "trackfw"}
	nonGuardCmd := &cobra.Command{Use: "validate", SilenceUsage: true}
	nonGuardRoot.AddCommand(nonGuardCmd)

	// Árvore guard: trackfw → guard → sub (SilenceUsage=true).
	guardRoot := &cobra.Command{Use: "trackfw"}
	guardGroup := &cobra.Command{Use: "guard"}
	guardSub := &cobra.Command{Use: "sub", SilenceUsage: true}
	guardGroup.AddCommand(guardSub)
	guardRoot.AddCommand(guardGroup)

	cases := []struct {
		name       string
		cmd        *cobra.Command
		err        error
		wantCode   int
		wantSilent bool // true → msg must be ""
	}{
		{
			name:       "silent Code=1 non-guard",
			cmd:        nonGuardCmd,
			err:        newExitCodeError(1, "test message"),
			wantCode:   1,
			wantSilent: true,
		},
		{
			name:       "noisy zero-value non-guard",
			cmd:        nonGuardCmd,
			err:        &ExitCodeError{}, // struct literal Code=0; constructor would panic
			wantCode:   1,
			wantSilent: false,
		},
		{
			name:       "noisy Code=256 non-guard",
			cmd:        nonGuardCmd,
			err:        &ExitCodeError{Code: 256, Msg: "code-256"}, // struct literal; constructor would panic
			wantCode:   1,
			wantSilent: false,
		},
		{
			name:       "noisy wrapped non-guard",
			cmd:        nonGuardCmd,
			err:        fmt.Errorf("wrap: %w", newExitCodeError(1, "x")),
			wantCode:   1,
			wantSilent: false,
		},
		{
			name:       "noisy Code=1 under guard rc=2",
			cmd:        guardSub,
			err:        newExitCodeError(1, "guard-error"),
			wantCode:   2,
			wantSilent: false,
		},
		{
			name:       "noisy common error non-guard",
			cmd:        nonGuardCmd,
			err:        fmt.Errorf("something went wrong"),
			wantCode:   1,
			wantSilent: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, msg := decideExit(tc.cmd, tc.err)
			if code != tc.wantCode {
				t.Errorf("decideExit code = %d, want %d (msg=%q)", code, tc.wantCode, msg)
			}
			if tc.wantSilent && msg != "" {
				t.Errorf("decideExit: want silent (msg=''), got %q", msg)
			}
			if !tc.wantSilent && msg == "" {
				t.Errorf("decideExit: want noisy (msg!=''), got empty msg")
			}
		})
	}
}

// ─── ML-3A: construtor recusa código fora de [1,255] ─────────────────────────

// TestJSONExit_NewExitCodeError_OutOfRangePanics afirma que newExitCodeError
// entra em pânico para qualquer código fora de [1,255].
//
// Conclusão do ML que este teste afirma: o construtor recusa Code=0 e Code=256;
// struct literals com Code fora de [1,255] são tratados pelo decideExit como
// erros comuns (caminho ruidoso, rc=1) — coberto pelo caso "noisy zero-value" e
// "noisy Code=256" de TestJSONExit_DecideExit.
func TestJSONExit_NewExitCodeError_OutOfRangePanics(t *testing.T) {
	for _, code := range []int{0, 256} {
		code := code
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("newExitCodeError(%d, ...) não entrou em pânico — range [1,255] não aplicado", code)
				}
			}()
			_ = newExitCodeError(code, "should panic")
		})
	}
}
