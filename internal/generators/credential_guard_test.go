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

// getNodeCredentialGuardScript reconstrói o conteúdo da variante de PROJETO a partir dos blocos
// componíveis (CG_HEADER + CG_PROJECT_GUARD + CG_DETECTION_CORE + CG_PROJECT_TAIL), já que
// CREDENTIAL_GUARD_SCRIPT deixou de ser um único template literal (agora é a concatenação desses
// blocos, mesma decomposição de credentialGuardScript em scaffold.go — ver
// TestGlobalCredentialGuardScript_ParityAcrossStacks para a variante global).
func getNodeCredentialGuardScript(t *testing.T, repoRoot string) string {
	t.Helper()
	hooksPath := filepath.Join(repoRoot, "npm", "src", "generators", "hooks.js")

	header := getNodeSourceBlock(t, hooksPath, "CG_HEADER")
	guard := getNodeSourceBlock(t, hooksPath, "CG_PROJECT_GUARD")
	core := getNodeSourceBlock(t, hooksPath, "CG_DETECTION_CORE")
	tail := getNodeSourceBlock(t, hooksPath, "CG_PROJECT_TAIL")
	return header + guard + core + tail
}

// getPythonCredentialGuardScript reconstrói o conteúdo da variante de PROJETO a partir dos blocos
// componíveis (_CG_HEADER + _CG_PROJECT_GUARD + _CG_DETECTION_CORE + _CG_PROJECT_TAIL) — mesmo
// racional de getNodeCredentialGuardScript.
func getPythonCredentialGuardScript(t *testing.T, repoRoot string) string {
	t.Helper()
	initPath := filepath.Join(repoRoot, "pypi", "trackfw", "generators", "init_gen.py")

	header := getPythonSourceBlock(t, initPath, "_CG_HEADER")
	guard := getPythonSourceBlock(t, initPath, "_CG_PROJECT_GUARD")
	core := getPythonSourceBlock(t, initPath, "_CG_DETECTION_CORE")
	tail := getPythonSourceBlock(t, initPath, "_CG_PROJECT_TAIL")
	return header + guard + core + tail
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
// GenerateGlobalCredentialGuardScript — escopo global (~/.trackfw/scripts/), ML-1A do roadmap
// ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-
// update-harness.md. Usa SEMPRE um $HOME de fixture (t.TempDir()) — nunca o HOME real do
// ambiente de teste.
// ---------------------------------------------------------------------------

func TestGenerateGlobalCredentialGuardScript_WritesUnderTrackfwHomeScripts(t *testing.T) {
	fakeHome := t.TempDir()

	if err := GenerateGlobalCredentialGuardScript(fakeHome); err != nil {
		t.Fatalf("GenerateGlobalCredentialGuardScript erro: %v", err)
	}

	path := filepath.Join(fakeHome, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("script global não foi criado em %s: %v", path, err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Errorf("script global não é executável: mode=%v", info.Mode())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro lendo script global: %v", err)
	}
	if !strings.HasPrefix(string(content), "#!/usr/bin/env bash") {
		t.Errorf("script global não começa com shebang esperado")
	}
	if strings.Contains(string(content), `[ -f "trackfw.yaml" ] || exit 0`) {
		t.Errorf("script global não deve conter a guarda de projeto (mataria o propósito cross-project)")
	}
}

func TestGenerateGlobalCredentialGuardScript_EmptyHome_Errors(t *testing.T) {
	if err := GenerateGlobalCredentialGuardScript(""); err == nil {
		t.Error("esperava erro com home vazio (nunca deve cair silenciosamente em cwd)")
	}
}

func getGoGlobalCredentialGuardScript(t *testing.T) string {
	t.Helper()
	fakeHome := t.TempDir()

	if err := GenerateGlobalCredentialGuardScript(fakeHome); err != nil {
		t.Fatalf("GenerateGlobalCredentialGuardScript erro: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(fakeHome, ".trackfw", "scripts", "trackfw-credential-guard.sh"))
	if err != nil {
		t.Fatalf("erro lendo script global Go: %v", err)
	}
	return string(content)
}

// getNodeSourceBlock extrai um bloco `const <name> = \`...\`` literal (template literal simples,
// sem concatenação) de um arquivo-fonte JS, e reverte a duplicação de backslash + o escape de
// ${...} feitos para o parser de template literal (mesma normalização de
// getNodeCredentialGuardScript).
func getNodeSourceBlock(t *testing.T, path, constName string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro lendo %s: %v", path, err)
	}

	s := string(content)
	match := regexp.MustCompile(`const `+constName+` = \x60([\s\S]*?)\x60`).FindStringSubmatch(s)
	if len(match) < 2 {
		t.Fatalf("%s não encontrado em %s", constName, path)
	}

	res := match[1]
	res = strings.ReplaceAll(res, `\${`, `${`)
	res = strings.ReplaceAll(res, `\\`, `\`)
	return res
}

// getPythonSourceBlock extrai um bloco `<name> = r"""..."""` literal de um arquivo-fonte Python.
func getPythonSourceBlock(t *testing.T, path, varName string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro lendo %s: %v", path, err)
	}

	s := string(content)
	match := regexp.MustCompile(varName + ` = r?"""([\s\S]*?)"""`).FindStringSubmatch(s)
	if len(match) < 2 {
		t.Fatalf("%s não encontrado em %s", varName, path)
	}
	return match[1]
}

func TestGlobalCredentialGuardScript_ParityAcrossStacks(t *testing.T) {
	repoRoot := findRepoRoot(t)

	goScript := getGoGlobalCredentialGuardScript(t)

	nodeHeader := getNodeSourceBlock(t, filepath.Join(repoRoot, "npm", "src", "generators", "hooks.js"), "CG_HEADER")
	nodeCore := getNodeSourceBlock(t, filepath.Join(repoRoot, "npm", "src", "generators", "hooks.js"), "CG_DETECTION_CORE")
	nodeGlobalTail := getNodeSourceBlock(t, filepath.Join(repoRoot, "npm", "src", "generators", "hooks.js"), "CG_GLOBAL_TAIL")
	nodeScript := nodeHeader + nodeCore + nodeGlobalTail

	pyHeader := getPythonSourceBlock(t, filepath.Join(repoRoot, "pypi", "trackfw", "generators", "init_gen.py"), "_CG_HEADER")
	pyCore := getPythonSourceBlock(t, filepath.Join(repoRoot, "pypi", "trackfw", "generators", "init_gen.py"), "_CG_DETECTION_CORE")
	pyGlobalTail := getPythonSourceBlock(t, filepath.Join(repoRoot, "pypi", "trackfw", "generators", "init_gen.py"), "_CG_GLOBAL_TAIL")
	pyScript := pyHeader + pyCore + pyGlobalTail

	if goScript != pyScript {
		t.Errorf("script global diverge entre Go e Python (esperado byte-idêntico).\n--- Go ---\n%s\n--- Python ---\n%s", goScript, pyScript)
	}
	if goScript != nodeScript {
		t.Errorf("script global diverge entre Go e Node (após normalizar escapes de template literal).\n--- Go ---\n%s\n--- Node (normalizado) ---\n%s", goScript, nodeScript)
	}
}

// TestCredentialGuardScript_DetectionCoreIdenticalBetweenProjectAndGlobal prova que a variante de
// projeto e a variante global do script Go compartilham EXATAMENTE o mesmo núcleo de detecção
// (JWT/AWS-key + exceção de destino efêmero) — não há duas cópias divergentes da regex.
func TestCredentialGuardScript_DetectionCoreIdenticalBetweenProjectAndGlobal(t *testing.T) {
	if !strings.Contains(credentialGuardScript, credentialGuardDetectionCore) {
		t.Error("credentialGuardScript (projeto) não contém credentialGuardDetectionCore")
	}
	if !strings.Contains(globalCredentialGuardScript, credentialGuardDetectionCore) {
		t.Error("globalCredentialGuardScript não contém credentialGuardDetectionCore")
	}
}

// ---------------------------------------------------------------------------
// Comportamento do script global — invoca como subprocesso, mesmo padrão dos testes de
// comportamento do script de projeto acima. Prova que a detecção é idêntica (mesmo payload de
// JWT sintético) e que o modo global agora reusa a mesma leitura de credential_guard.mode de
// trackfw.yaml que a variante de projeto (ADR-2026-08-06 emenda 6, 2026-08-08) — com fallback
// "block" (em vez de "warn") quando não há trackfw.yaml no cwd, ou trackfw.yaml sem essa chave.
// ---------------------------------------------------------------------------

func setupGlobalCredentialGuardFixture(t *testing.T) (cwd, scriptPath string) {
	t.Helper()
	fakeHome := t.TempDir()
	if err := GenerateGlobalCredentialGuardScript(fakeHome); err != nil {
		t.Fatalf("GenerateGlobalCredentialGuardScript erro: %v", err)
	}
	scriptPath = filepath.Join(fakeHome, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	cwd = t.TempDir()
	return cwd, scriptPath
}

func TestGlobalCredentialGuardScript_RunsOutsideAnyTrackfwProject(t *testing.T) {
	// Ao contrário da variante de projeto (TestCredentialGuardScript_NoOpOutsideProjectRoot), o
	// script global NÃO deve ser no-op fora de um projeto trackfw — esse é o propósito da mudança.
	// Sem trackfw.yaml no cwd, o fallback de modo é "block" (ADR-2026-08-06 emenda 6).
	cwd, scriptPath := setupGlobalCredentialGuardFixture(t)
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	code, _, stderr := runCredentialGuard(t, cwd, scriptPath, payload)
	if code != 2 {
		t.Errorf("modo block (fallback sem trackfw.yaml): exit code want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "JWT") {
		t.Errorf("esperava aviso mencionando JWT em stderr mesmo sem trackfw.yaml no cwd, got: %s", stderr)
	}
}

func TestGlobalCredentialGuardScript_AWSKeyDetectedSameAsProjectVariant(t *testing.T) {
	// Prova que a detecção (mesmo payload sintético) é idêntica entre projeto e global — os modos
	// default divergem por design (projeto: warn; global sem trackfw.yaml: block), então os exit
	// codes divergem, mas ambos devem mencionar AWS.
	projectDir, projectScript := setupCredentialGuardFixture(t, "")
	globalCwd, globalScript := setupGlobalCredentialGuardFixture(t)

	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticAWSKey + `"}}`

	pCode, _, pStderr := runCredentialGuard(t, projectDir, projectScript, payload)
	gCode, _, gStderr := runCredentialGuard(t, globalCwd, globalScript, payload)

	if pCode != 0 {
		t.Errorf("projeto (fallback warn): exit code want 0, got %d (stderr: %s)", pCode, pStderr)
	}
	if gCode != 2 {
		t.Errorf("global sem trackfw.yaml (fallback block): exit code want 2, got %d (stderr: %s)", gCode, gStderr)
	}
	if !strings.Contains(pStderr, "AWS") || !strings.Contains(gStderr, "AWS") {
		t.Fatalf("ambas as variantes deveriam mencionar AWS: projeto=%q global=%q", pStderr, gStderr)
	}
}

func TestGlobalCredentialGuardScript_NoMatch_SilentPass(t *testing.T) {
	cwd, scriptPath := setupGlobalCredentialGuardFixture(t)
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo hello world"}}`

	code, _, stderr := runCredentialGuard(t, cwd, scriptPath, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(cwd) {
		t.Error("não deveria escrever .trackfw-credential-guard.json sem match")
	}
}

func TestGlobalCredentialGuardScript_RespectsExplicitProjectMode(t *testing.T) {
	// O script global agora reusa a mesma leitura de credential_guard.mode de trackfw.yaml que a
	// variante de projeto já faz (ADR-2026-08-06 emenda 6, 2026-08-08): quando o cwd tem
	// trackfw.yaml com mode explícito, esse valor é respeitado — tanto block quanto warn.
	cwd, scriptPath := setupGlobalCredentialGuardFixture(t)
	if err := os.WriteFile(filepath.Join(cwd, "trackfw.yaml"), []byte("credential_guard:\n  mode: block\n"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	code, _, stderr := runCredentialGuard(t, cwd, scriptPath, payload)
	if code != 2 {
		t.Errorf("trackfw.yaml com mode: block explícito: exit code want 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "blocked") {
		t.Errorf("esperava mensagem de bloqueio, got: %s", stderr)
	}
}

func TestGlobalCredentialGuardScript_RespectsExplicitProjectModeWarn(t *testing.T) {
	// Mesma decisão, sentido contrário: mode: warn explícito continua produzindo warn (exit 0),
	// não block — não é o fallback que muda de warn para block, é só o default sem chave/arquivo.
	cwd, scriptPath := setupGlobalCredentialGuardFixture(t)
	if err := os.WriteFile(filepath.Join(cwd, "trackfw.yaml"), []byte("credential_guard:\n  mode: warn\n"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	code, _, stderr := runCredentialGuard(t, cwd, scriptPath, payload)
	if code != 0 {
		t.Errorf("trackfw.yaml com mode: warn explícito: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("esperava mensagem de warning, got: %s", stderr)
	}
}

func TestGlobalCredentialGuardScript_WritesAttentionOnlyWhenRoadmapsDirExists(t *testing.T) {
	// O attention signal só é gravado em modo warn (modo block nunca grava, mesma decisão da
	// variante de projeto) -- usa mode: warn explícito no cwd para exercitar essa checagem
	// independente do fallback default de modo global (block, ADR-2026-08-06 emenda 6).
	cwd, scriptPath := setupGlobalCredentialGuardFixture(t)
	if err := os.WriteFile(filepath.Join(cwd, "trackfw.yaml"), []byte("credential_guard:\n  mode: warn\n"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	// Sem docs/roadmaps no cwd: warning em stderr, mas nenhum arquivo de attention.
	code, _, stderr := runCredentialGuard(t, cwd, scriptPath, payload)
	if code != 0 {
		t.Fatalf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "JWT") {
		t.Errorf("esperava warning em stderr mesmo sem docs/roadmaps, got: %s", stderr)
	}
	if attentionFileExists(cwd) {
		t.Error("não deveria criar docs/roadmaps/.trackfw-credential-guard.json quando docs/roadmaps não existe (evita criar estrutura trackfw num projeto qualquer)")
	}

	// Com docs/roadmaps existente no cwd: escreve o attention signal normalmente.
	if err := os.MkdirAll(filepath.Join(cwd, "docs", "roadmaps"), 0755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCredentialGuard(t, cwd, scriptPath, payload)
	if code != 0 {
		t.Fatalf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(cwd) {
		t.Error(".trackfw-credential-guard.json deveria ter sido escrito quando docs/roadmaps já existe")
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

// ---------------------------------------------------------------------------
// ML-3A (ROADMAP-2026-08-08, REQ-2026-08-08 Acceptance Criteria — testes novos, 3 cenários).
// ---------------------------------------------------------------------------

// TestGlobalCredentialGuardScript_YAMLPresentWithoutModeKey_FallsBackToBlock cobre o cenário (a)
// da REQ na variante onde trackfw.yaml EXISTE no cwd mas não define `credential_guard.mode` (nem o
// bloco `credential_guard:` em si) — distinto de TestGlobalCredentialGuardScript_
// RunsOutsideAnyTrackfwProject, que cobre a ausência total do arquivo. Em ambos os casos o
// fallback do script global deve ser "block" (exit 2), não "warn" (ADR-2026-08-06 emenda 6).
func TestGlobalCredentialGuardScript_YAMLPresentWithoutModeKey_FallsBackToBlock(t *testing.T) {
	cwd, scriptPath := setupGlobalCredentialGuardFixture(t)
	if err := os.WriteFile(filepath.Join(cwd, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo ` + syntheticJWT + `"}}`

	code, _, stderr := runCredentialGuard(t, cwd, scriptPath, payload)
	if code != 2 {
		t.Errorf("trackfw.yaml presente sem credential_guard.mode: exit code want 2 (fallback block), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "blocked") {
		t.Errorf("esperava mensagem de bloqueio, got: %s", stderr)
	}
}

// TestCredentialGuardScript_SecondLayer_CatArgument_Captured cobre o cenário (c) da REQ: um
// comando Bash que referencia um arquivo contendo o segredo por CAMINHO — sem o JWT/AWS key
// literal no texto do comando — deve ser capturado pela segunda camada de detecção
// (credentialGuardDetectionCore, ADR-2026-08-06 emenda 8), via argumento direto de `cat`.
func TestCredentialGuardScript_SecondLayer_CatArgument_Captured(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "credential_guard:\n  mode: block\n")
	fixture := filepath.Join(dir, "token-fixture.txt")
	if err := os.WriteFile(fixture, []byte(syntheticJWT+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"cat token-fixture.txt"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 2 {
		t.Errorf("modo block: exit code want 2 (segunda camada deveria capturar via 'cat <arquivo>'), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "JWT") {
		t.Errorf("esperava aviso mencionando JWT em stderr, got: %s", stderr)
	}
}

// TestCredentialGuardScript_SecondLayer_HeadDashCArgument_Captured é o caso concreto do
// incidente relatado na REQ: `head -c 50 <arquivo>`, sem o segredo literal no comando.
func TestCredentialGuardScript_SecondLayer_HeadDashCArgument_Captured(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "")
	fixture := filepath.Join(dir, "token-fixture.txt")
	if err := os.WriteFile(fixture, []byte(syntheticJWT+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"head -c 50 token-fixture.txt"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "JWT") {
		t.Errorf("esperava aviso mencionando JWT em stderr (segunda camada via 'head -c'), got: %s", stderr)
	}
	if !attentionFileExists(dir) {
		t.Error(".trackfw-credential-guard.json deveria ter sido escrito (segunda camada de detecção)")
	}
}

// TestCredentialGuardScript_SecondLayer_AWSKeyViaGrepArgument_Captured cobre a mesma segunda
// camada para o padrão AWS key e para outro dos inspetores documentados (grep), não só cat/head.
func TestCredentialGuardScript_SecondLayer_AWSKeyViaGrepArgument_Captured(t *testing.T) {
	dir, script := setupCredentialGuardFixture(t, "credential_guard:\n  mode: block\n")
	fixture := filepath.Join(dir, "aws-fixture.txt")
	if err := os.WriteFile(fixture, []byte("AWS_ACCESS_KEY_ID="+syntheticAWSKey+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"grep AWS_ACCESS_KEY_ID aws-fixture.txt"}}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 2 {
		t.Errorf("modo block: exit code want 2 (segunda camada deveria capturar via 'grep <padrão> <arquivo>'), got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "AWS") {
		t.Errorf("esperava aviso mencionando AWS em stderr, got: %s", stderr)
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
