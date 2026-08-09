package generators

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ML-4A — Teste de sabotagem end-to-end.
//
// Diferente de credential_guard_test.go (que já invoca o script real como
// subprocesso, mas com um payload JSON genérico "{"tool_name":"Bash",
// "tool_input":{"command":...}}" escrito à mão), este arquivo:
//
//  1. Gera o wiring REAL de cada CLI via InjectXHooks (o mesmo gerador
//     exercitado por agentfiles_test.go), confirmando que o hooks.json/
//     settings.json resultante de fato referencia
//     "scripts/trackfw-credential-guard.sh".
//  2. Constrói o payload JSON EXATO que aquele CLI envia via stdin ao hook,
//     conforme o schema documentado em docs/cli-parity.md por CLI (não um
//     payload genérico reaproveitado entre CLIs).
//  3. Materializa um JWT sintético (nunca hardcoded como token real
//     plausível — string claramente de teste) dentro do payload.
//  4. Invoca o script gerado (não uma cópia, não uma reimplementação da
//     regex) como subprocesso via bash, passando o payload por stdin.
//  5. Confirma detecção nos dois modos (warn/block) e prova negativa (mesmo
//     payload sem JWT não detecta nada).
//
// Cobertura por CLI (ver docs/cli-parity.md e nota de auditoria do ML-4A no
// roadmap para o detalhe completo):
//
//   - Claude Code: COBERTO (obrigatório pelo AC da REQ). Schema PreToolUse/
//     PostToolUse confirmado: {"tool_name":"Bash","tool_input":{"command":...}}.
//   - Cursor: COBERTO. Schema beforeShellExecution/afterShellExecution
//     confirmado via doc oficial (docs/cli-parity.md, seção "Cursor wiring
//     (ML-2E)"): {"command":"...","cwd":"...","sandbox":false}.
//   - Kiro: COBERTO. Schema PreToolUse/PostToolUse confirmado via doc oficial
//     (docs/cli-parity.md, seção "Kiro wiring (ML-2F)"):
//     {"hook_event_name":"PreToolUse","cwd":"...","session_id":"...",
//     "tool_name":"execute_bash","tool_input":{"command":"..."}}.
//   - Codex: SEM teste de sabotagem end-to-end. Motivo: docs/cli-parity.md
//     confirma que o matcher do wiring de config (hooks.json) é aplicado a
//     "tool_name", mas a doc oficial pesquisada (developers.openai.com/codex/
//     hooks) não expõe, no texto recuperado, um exemplo completo do payload
//     JSON que chega via stdin ao hook em runtime — apenas o formato do
//     arquivo de configuração hooks.json (que já é coberto pelos testes de
//     wiring em agentfiles_test.go). Reaproveitar o payload de Claude Code
//     "por analogia" seria inventar um contrato não confirmado.
//   - Gemini CLI: SEM teste de sabotagem end-to-end. Motivo: docs/cli-parity.md
//     confirma o nome do evento (BeforeTool/AfterTool) e do matcher
//     ("run_shell_command"), mas a doc oficial pesquisada
//     (geminicli.com/docs/hooks/reference) não expõe, no texto recuperado, um
//     exemplo de payload JSON de stdin para hooks de tool — apenas a semântica
//     de matcher contra tool_name.
//   - GitHub Copilot: SEM teste de sabotagem end-to-end. Motivo: docs/cli-parity.md
//     confirma que o payload traz um campo "toolName" (formato camelCase) mas
//     não reproduz um exemplo completo de payload JSON de comando; o próprio
//     texto registra que o formato depende do casing do nome do evento
//     (camelCase vs PascalCase "VS Code compatible") sem cravar qual dos dois
//     o trackfw deveria simular sem inventar.
//
// Todos os 3 CLIs cobertos (Claude Code, Cursor, Kiro) usam o mesmo script
// compartilhado (byte-idêntico entre stacks, ver
// TestCredentialGuardScript_ParityAcrossStacks) — o script não inspeciona
// nomes de campo, apenas varre o payload bruto, então a divergência de schema
// entre CLIs afeta apenas a fidelidade do teste, nunca a lógica de detecção.
// ---------------------------------------------------------------------------

func setupSabotageFixture(t *testing.T, injectHooks func(cwd string) error, trackfwYAML string) (dir, scriptPath string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // isolate global credential-guard dedup check (ML-3A) from real $HOME
	dir = t.TempDir()

	if err := GenerateCredentialGuardScript(dir); err != nil {
		t.Fatalf("GenerateCredentialGuardScript erro: %v", err)
	}
	if err := injectHooks(dir); err != nil {
		t.Fatalf("InjectXHooks erro: %v", err)
	}
	if trackfwYAML == "" {
		trackfwYAML = "roadmap_dir: docs/roadmaps\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(trackfwYAML), 0644); err != nil {
		t.Fatal(err)
	}

	return dir, filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")
}

// ---------------------------------------------------------------------------
// Claude Code — PreToolUse/PostToolUse, matcher "Bash".
// Schema confirmado: {"tool_name":"Bash","tool_input":{"command":"..."}}
// ---------------------------------------------------------------------------

func TestSabotage_ClaudeCode_WiringReferencesRealScript(t *testing.T) {
	dir, _ := setupSabotageFixture(t, InjectClaudeHooks, "")

	data := helperReadJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	if !helperHasClaudeHook(data, "PreToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Fatal("wiring do Claude Code não referencia $CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh em PreToolUse[Bash]")
	}
	if !helperHasClaudeHook(data, "PostToolUse", "Bash", "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh") {
		t.Fatal("wiring do Claude Code não referencia $CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh em PostToolUse[Bash]")
	}
}

func claudeCodePreToolUsePayload(command string) string {
	return `{"tool_name":"Bash","tool_input":{"command":"` + command + `"}}`
}

func TestSabotage_ClaudeCode_JWTInBashCommand_WarnMode(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectClaudeHooks, "")
	payload := claudeCodePreToolUsePayload("echo " + syntheticJWT)

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(dir) {
		t.Fatal(".trackfw-credential-guard.json deveria ter sido escrito (JWT sintético via payload PreToolUse do Claude Code)")
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

func TestSabotage_ClaudeCode_JWTInBashCommand_BlockMode(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectClaudeHooks, "credential_guard:\n  mode: block\n")
	payload := claudeCodePreToolUsePayload("echo " + syntheticJWT)

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 2 {
		t.Errorf("modo block: exit code want 2, got %d (stderr: %s)", code, stderr)
	}
}

func TestSabotage_ClaudeCode_NoJWT_ProvaNegativa(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectClaudeHooks, "")
	payload := claudeCodePreToolUsePayload("git status")

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("prova negativa falhou: payload sem JWT/AWS-key não deveria disparar detecção (teste não é vácuo/sempre-verde)")
	}
}

// ---------------------------------------------------------------------------
// Cursor — beforeShellExecution/afterShellExecution.
// Schema confirmado (docs/cli-parity.md, "Cursor wiring (ML-2E)"):
// {"command":"...","cwd":"...","sandbox":false}
// ---------------------------------------------------------------------------

func TestSabotage_Cursor_WiringReferencesRealScript(t *testing.T) {
	dir, _ := setupSabotageFixture(t, InjectCursorHooks, "")

	data := helperReadJSON(t, filepath.Join(dir, ".cursor", "hooks.json"))
	hooks, _ := data["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatal("wiring do Cursor não tem objeto hooks de nível superior")
	}
	before, _ := hooks["beforeShellExecution"].([]interface{})
	found := false
	for _, item := range before {
		obj, ok := item.(map[string]interface{})
		if ok && obj["command"] == "scripts/trackfw-credential-guard.sh" {
			found = true
		}
	}
	if !found {
		t.Fatal("wiring do Cursor não referencia scripts/trackfw-credential-guard.sh em hooks.beforeShellExecution")
	}
}

func cursorBeforeShellExecutionPayload(command string) string {
	return `{"command":"` + command + `","cwd":"/tmp/fixture","sandbox":false}`
}

func TestSabotage_Cursor_JWTInShellCommand_WarnMode(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectCursorHooks, "")
	payload := cursorBeforeShellExecutionPayload("echo " + syntheticJWT)

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(dir) {
		t.Fatal(".trackfw-credential-guard.json deveria ter sido escrito (JWT sintético via payload beforeShellExecution do Cursor)")
	}
}

func TestSabotage_Cursor_JWTInShellCommand_BlockMode(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectCursorHooks, "credential_guard:\n  mode: block\n")
	payload := cursorBeforeShellExecutionPayload("echo " + syntheticJWT)

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 2 {
		t.Errorf("modo block: exit code want 2, got %d (stderr: %s)", code, stderr)
	}
}

func TestSabotage_Cursor_NoJWT_ProvaNegativa(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectCursorHooks, "")
	payload := cursorBeforeShellExecutionPayload("git status")

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("prova negativa falhou: payload sem JWT/AWS-key não deveria disparar detecção")
	}
}

// afterShellExecution acrescenta output/duration ao payload base — cobre o caso em que a
// credencial só aparece na saída capturada do comando, não no comando em si.
func TestSabotage_Cursor_JWTOnlyInCapturedOutput_AfterShellExecution_WarnMode(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectCursorHooks, "")
	payload := `{"command":"curl https://internal.example/token","cwd":"/tmp/fixture","sandbox":false,` +
		`"output":"token=` + syntheticJWT + `","duration":123}`

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(dir) {
		t.Fatal(".trackfw-credential-guard.json deveria ter sido escrito (JWT sintético apenas no campo output do afterShellExecution)")
	}
}

// ---------------------------------------------------------------------------
// Kiro — PreToolUse/PostToolUse, matcher "shell".
// Schema confirmado (docs/cli-parity.md, "Kiro wiring (ML-2F)"):
// {"hook_event_name","cwd","session_id","tool_name","tool_input"}
// ---------------------------------------------------------------------------

func TestSabotage_Kiro_WiringReferencesRealScript(t *testing.T) {
	dir, _ := setupSabotageFixture(t, InjectKiroHooks, "")

	data := helperReadJSON(t, filepath.Join(dir, ".kiro", "hooks", "trackfw-attention.json"))
	hooksArr, _ := data["hooks"].([]interface{})
	sawPre, sawPost := false, false
	for _, h := range hooksArr {
		entry, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		action, _ := entry["action"].(map[string]interface{})
		if action == nil || action["command"] != "scripts/trackfw-credential-guard.sh" {
			continue
		}
		switch entry["trigger"] {
		case "PreToolUse":
			sawPre = true
		case "PostToolUse":
			sawPost = true
		}
	}
	if !sawPre || !sawPost {
		t.Fatalf("wiring do Kiro não referencia scripts/trackfw-credential-guard.sh em PreToolUse/PostToolUse (pre=%v post=%v)", sawPre, sawPost)
	}
}

func kiroPreToolUsePayload(dir, command string) string {
	return `{"hook_event_name":"PreToolUse","cwd":"` + dir + `","session_id":"sess-sabotage-test",` +
		`"tool_name":"execute_bash","tool_input":{"command":"` + command + `"}}`
}

func TestSabotage_Kiro_JWTInToolInput_WarnMode(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectKiroHooks, "")
	payload := kiroPreToolUsePayload(dir, "echo "+syntheticJWT)

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("modo warn: exit code want 0, got %d (stderr: %s)", code, stderr)
	}
	if !attentionFileExists(dir) {
		t.Fatal(".trackfw-credential-guard.json deveria ter sido escrito (JWT sintético via payload PreToolUse do Kiro)")
	}
}

func TestSabotage_Kiro_JWTInToolInput_BlockMode(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectKiroHooks, "credential_guard:\n  mode: block\n")
	payload := kiroPreToolUsePayload(dir, "echo "+syntheticJWT)

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 2 {
		t.Errorf("modo block: exit code want 2, got %d (stderr: %s)", code, stderr)
	}
}

func TestSabotage_Kiro_NoJWT_ProvaNegativa(t *testing.T) {
	dir, script := setupSabotageFixture(t, InjectKiroHooks, "")
	payload := kiroPreToolUsePayload(dir, "git status")

	code, _, stderr := runCredentialGuard(t, dir, script, payload)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d (stderr: %s)", code, stderr)
	}
	if attentionFileExists(dir) {
		t.Error("prova negativa falhou: payload sem JWT/AWS-key não deveria disparar detecção")
	}
}
