package generators

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Generator: file creation
// ---------------------------------------------------------------------------

func TestGenerateCredentialGuardScript_CreatesExecutableFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateCredentialGuardScript(""); err != nil {
		t.Fatalf("GenerateCredentialGuardScript erro: %v", err)
	}

	path := filepath.Join("scripts", "trackfw-credential-guard.sh")
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

func TestGenerateCredentialGuardScript_DoesNotWireIntoAnyHooksFile(t *testing.T) {
	// ML-1A explicitamente não injeta o script em nenhum hooks.json/settings.json de CLI —
	// isso é escopo da Wave 2. Confirma que apenas o script shell é criado.
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateCredentialGuardScript(""); err != nil {
		t.Fatalf("GenerateCredentialGuardScript erro: %v", err)
	}

	for _, p := range []string{
		".claude/settings.json",
		".codex/hooks.json",
		".gemini/settings.json",
		".github/hooks/hooks.json",
		".cursor/hooks.json",
		".kiro/hooks/trackfw-attention.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("ML-1A não deve criar %s (escopo da Wave 2)", p)
		}
	}
}

// ---------------------------------------------------------------------------
// Cross-stack parity (Go vs Node vs Python) — byte-identical, mesmo padrão do
// gate scripts/check-attention-scripts-parity.sh e de scaffold_parity_test.go.
// ---------------------------------------------------------------------------

func getGoCredentialGuardScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateCredentialGuardScript(""); err != nil {
		t.Fatalf("GenerateCredentialGuardScript erro: %v", err)
	}

	content, err := os.ReadFile(filepath.Join("scripts", "trackfw-credential-guard.sh"))
	if err != nil {
		t.Fatalf("erro lendo script Go: %v", err)
	}
	return string(content)
}

func getNodeCredentialGuardScript(t *testing.T, repoRoot string) string {
	t.Helper()
	hooksPath := filepath.Join(repoRoot, "npm", "src", "generators", "hooks.js")
	content, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("erro lendo %s: %v", hooksPath, err)
	}

	s := string(content)
	match := regexp.MustCompile(`const CREDENTIAL_GUARD_SCRIPT = \x60([\s\S]*?)\x60`).FindStringSubmatch(s)
	if len(match) < 2 {
		t.Fatalf("CREDENTIAL_GUARD_SCRIPT não encontrado em npm/src/generators/hooks.js")
	}

	// Reverte a duplicação de backslash + o escape de ${...} feitos para o parser de
	// template literal do JS (mesma técnica de normalização de getNodeScripts em
	// scaffold_parity_test.go, adaptada aos tokens específicos deste script).
	res := match[1]
	res = strings.ReplaceAll(res, `\${`, `${`)
	res = strings.ReplaceAll(res, `\\`, `\`)
	return res
}

func getPythonCredentialGuardScript(t *testing.T, repoRoot string) string {
	t.Helper()
	initPath := filepath.Join(repoRoot, "pypi", "trackfw", "generators", "init_gen.py")
	content, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("erro lendo %s: %v", initPath, err)
	}

	s := string(content)
	match := regexp.MustCompile(`_CREDENTIAL_GUARD_SH = r?"""([\s\S]*?)"""`).FindStringSubmatch(s)
	if len(match) < 2 {
		t.Fatalf("_CREDENTIAL_GUARD_SH não encontrado em pypi/trackfw/generators/init_gen.py")
	}
	return match[1]
}

func TestCredentialGuardScript_ParityAcrossStacks(t *testing.T) {
	repoRoot := findRepoRoot(t)

	goScript := getGoCredentialGuardScript(t)
	nodeScript := getNodeCredentialGuardScript(t, repoRoot)
	pyScript := getPythonCredentialGuardScript(t, repoRoot)

	if goScript != pyScript {
		t.Errorf("script diverge entre Go e Python (esperado byte-idêntico).\n--- Go ---\n%s\n--- Python ---\n%s", goScript, pyScript)
	}
	if goScript != nodeScript {
		t.Errorf("script diverge entre Go e Node (após normalizar escapes de template literal).\n--- Go ---\n%s\n--- Node (normalizado) ---\n%s", goScript, nodeScript)
	}
}

// ---------------------------------------------------------------------------
// Behavior — invoca o script real como subprocesso (não reimplementa a regex em
// paralelo). Cobre detecção de JWT/AWS key, exceção de destino efêmero, e os dois
// modos de credential_guard.mode.
// ---------------------------------------------------------------------------

const syntheticJWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.abc123def456ghi789"
const syntheticAWSKey = "AKIAABCDEFGHIJKLMNOP"

func setupCredentialGuardFixture(t *testing.T, trackfwYAML string) (dir, scriptPath string) {
	t.Helper()
	dir = t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateCredentialGuardScript(""); err != nil {
		t.Fatalf("GenerateCredentialGuardScript erro: %v", err)
	}
	if trackfwYAML != "" {
		if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(trackfwYAML), 0644); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir, filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")
}

func runCredentialGuard(t *testing.T, dir, scriptPath, stdin string) (exitCode int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command("bash", scriptPath)
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

func attentionFileExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "docs", "roadmaps", ".trackfw-credential-guard.json"))
	return err == nil
}

func TestCredentialGuardScript_NoMatch_SilentPass(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo hello world"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("não deveria escrever .trackfw-credential-guard.json sem match")
	}
}

func TestCredentialGuardScript_JWTPrintedToStdout_WarnsByDefault(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "JWT") {
		t.Errorf("esperava aviso mencionando JWT em stderr, got: %s", stderr)
	}
	if !attentionFileExists(dir) {
		t.Fatal(".trackfw-credential-guard.json deveria ter sido escrito em modo warn")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "docs", "roadmaps", ".trackfw-credential-guard.json"))
	if err != nil {
		t.Fatal(err)
	}
	var payloadJSON map[string]interface{}
	if err := json.Unmarshal(raw, &payloadJSON); err != nil {
		t.Fatalf("credential-guard.json inválido: %v (%s)", err, raw)
	}
	if payloadJSON["level"] != "action_required" {
		t.Errorf("level: want action_required, got %v", payloadJSON["level"])
	}
}

func TestCredentialGuardScript_AWSKeyDetected(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticAWSKey + `"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "AWS") {
		t.Errorf("esperava aviso mencionando AWS em stderr, got: %s", stderr)
	}
	if !attentionFileExists(dir) {
		t.Error(".trackfw-credential-guard.json deveria ter sido escrito")
	}
}

func TestCredentialGuardScript_RedirectedToDevNull_Ephemeral_NoAlert(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + ` > /dev/null"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("destino /dev/null deveria ser tratado como efêmero (sem alerta)")
	}
}

func TestCredentialGuardScript_RedirectedToMktempDirect_Ephemeral_NoAlert(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + ` > $(mktemp)"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("destino $(mktemp) deveria ser tratado como efêmero (sem alerta)")
	}
}

func TestCredentialGuardScript_RedirectedToMktempVariable_Ephemeral_NoAlert(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "")
	cmd := `TMPFILE=$(mktemp); echo ` + syntheticJWT + ` > "$TMPFILE"`
	// encoding/json.Marshal HTML-escapes '>' (>) by default — not representative of the
	// raw JSON a hook harness sends over stdin. Use an Encoder with SetEscapeHTML(false) so the
	// fixture matches production payload shape (literal '>').
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]interface{}{
		"tool_name":  "Bash",
		"tool_input": map[string]string{"command": cmd},
	}); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runCredentialGuard(t, dir, script, buf.String())
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("variável atribuída via $(mktemp) deveria ser tratada como efêmera (sem alerta)")
	}
}

func TestCredentialGuardScript_RedirectedToPlainFile_NotEphemeral_Alerts(t *testing.T) {
	// Este é o caso do incidente real da REQ: token gravado em arquivo solto, não efêmero.
	dir, script := setupCredentialGuardFixture(t, "")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + ` > /tmp/token.txt"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(dir) {
		t.Error("redirecionamento para caminho de arquivo comum deveria alertar (não é destino efêmero)")
	}
}

func TestCredentialGuardScript_BlockMode_ExitsWithCode2(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "credential_guard:\n  mode: block\n")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 2 {
		t.Errorf("modo block: exit code want 2, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("modo block não deveria escrever .trackfw-credential-guard.json (bloqueio direto, sem sinalização adicional)")
	}
}

func TestCredentialGuardScript_InvalidModeValue_FallsBackToWarn(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "credential_guard:\n  mode: nonsense\n")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("valor de mode inválido deveria cair para warn (exit 0), got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(dir) {
		t.Error("valor de mode inválido deveria cair para warn (com sinalização)")
	}
}

// TestCredentialGuardScript_AttentionCleanupDoesNotDeleteIt prova que o hook de cleanup
// (trackfw-attention-cleanup.sh), que apaga incondicionalmente $ROADMAP_DIR/.trackfw-attention.json,
// não apaga o arquivo dedicado do credential-guard (.trackfw-credential-guard.json). Antes do fix, os
// dois hooks compartilhavam .trackfw-attention.json; em harnesses que rodam hooks do mesmo evento
// concorrentemente (Codex CLI, PostToolUse com matchers ".*" e "Bash" ambos batendo em uma chamada
// Bash), o cleanup podia apagar o aviso do credential-guard escrito na mesma invocação — uma race
// real. Ver "Limitação conhecida" do ML-1A no roadmap.
func TestCredentialGuardScript_AttentionCleanupDoesNotDeleteIt(t *testing.T) {
	dir, guardScript := setupCredentialGuardFixture(t, "")

	if err := GenerateAttentionScripts(dir); err != nil {
		t.Fatalf("GenerateAttentionScripts erro: %v", err)
	}
	cleanupPath := filepath.Join(dir, "scripts", "trackfw-attention-cleanup.sh")

	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`
	code, _, stderr := runCredentialGuard(t, dir, guardScript, payload)
	if code != 0 {
		t.Fatalf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(dir) {
		t.Fatal(".trackfw-credential-guard.json deveria ter sido escrito em modo warn")
	}

	cmdCleanup := exec.Command("bash", cleanupPath)
	cmdCleanup.Dir = dir
	if out, err := cmdCleanup.CombinedOutput(); err != nil {
		t.Fatalf("Cleanup script falhou: %v, output: %s", err, string(out))
	}

	if !attentionFileExists(dir) {
		t.Error(".trackfw-credential-guard.json não deveria ter sido apagado pelo trackfw-attention-cleanup.sh (arquivo dedicado, não compartilhado com o mecanismo de attention-signal)")
	}
}

func TestCredentialGuardScript_NoOpOutsideProjectRoot(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateCredentialGuardScript(""); err != nil {
		t.Fatalf("GenerateCredentialGuardScript erro: %v", err)
	}
	// Sem trackfw.yaml no diretório.

	scriptPath := filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`
	code, _, stderr := runCredentialGuard(t, dir, scriptPath, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("sem trackfw.yaml, o script deve ser no-op")
	}
}
