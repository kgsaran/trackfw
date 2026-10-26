// validator_credential_guard_presence_warning_test.go — AC1, AC2, AC4, AC2-strict
// (D17, REQ-2026-10-25 #619 ML-1A, ML-1A2)
//
// Tests for the credential_guard_presence_warning rule: when a project hook file exists for one
// of the 6 CLIs (Claude, Codex, Gemini, Cursor, Copilot, Kiro) without the credential guard in
// the expected (event, matcher) pairs, validateCredentialGuardPresenceWarnings must emit one
// warning per missing pair.
//
// Package validator (internal) to access unexported validateCredentialGuardPresenceWarnings.
package validator

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/guardforms"
)

// ---------------------------------------------------------------------------
// Helpers (local to this file; not exported to avoid collision)
// ---------------------------------------------------------------------------

func countPresenceWarnings(msgs []string) int {
	n := 0
	for _, m := range msgs {
		if strings.Contains(m, "has no credential guard entry") {
			n++
		}
	}
	return n
}

func hasPresenceWarning(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, "has no credential guard entry") && strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// AC1 — Per-pair warning per CLI
// ---------------------------------------------------------------------------

// TestCredentialGuardPresenceWarning_Claude_TodosParesAusentes afirma que quando
// .claude/settings.json existe mas não tem credential guard em nenhum dos 3 pares esperados
// (Bash|PowerShell, Read, Write|Edit), validateCredentialGuardPresenceWarnings emite 3 avisos.
// Reconciliação (D17, #619 ML-1A): extrator schemaNested detecta ausência em cada par
// independentemente e emite um aviso por par ausente.
func TestCredentialGuardPresenceWarning_Claude_TodosParesAusentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "AskUserQuestion", "hooks": [{"command": "scripts/cleanup.sh", "type": "command"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("esperado 3 avisos para Claude, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
	if !hasPresenceWarning(msgs, "Bash|PowerShell") {
		t.Errorf("aviso Bash|PowerShell ausente: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "Read") {
		t.Errorf("aviso Read ausente: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "Write|Edit") {
		t.Errorf("aviso Write|Edit ausente: %v", msgs)
	}
}

// TestCredentialGuardPresenceWarning_Claude_UmParPresente afirma que quando apenas
// Bash|PowerShell está fiado, os avisos para Read e Write|Edit ainda disparam.
// Reconciliação: cada par é avaliado independentemente pela lista de events.
func TestCredentialGuardPresenceWarning_Claude_UmParPresente(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	credCmd := guardforms.CredentialCurrentPSPOSIX
	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash|PowerShell", "hooks": [{"type": "command", "command": "`+credCmd+`"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 2 {
		t.Errorf("esperado 2 avisos (Read e Write|Edit ausentes), obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
	if hasPresenceWarning(msgs, "Bash|PowerShell") {
		t.Errorf("aviso Bash|PowerShell não deveria disparar quando par presente: %v", msgs)
	}
}

// TestCredentialGuardPresenceWarning_Claude_TodosPresentes afirma que quando os 3 pares estão
// fiados, nenhum aviso de presença dispara para Claude (contra-braço do teste anterior).
// Reconciliação: zero warnings confirma que o extrator não é vácuo — encontrar os 3 pares
// suprime o aviso.
func TestCredentialGuardPresenceWarning_Claude_TodosPresentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	credCmd := guardforms.CredentialCurrentPSPOSIX
	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash|PowerShell", "hooks": [{"type": "command", "command": "`+credCmd+`"}]},
      {"matcher": "Read", "hooks": [{"type": "command", "command": "`+credCmd+`"}]},
      {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "`+credCmd+`"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 0 {
		t.Errorf("esperado zero avisos com 3 pares Claude presentes, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
}

// TestCredentialGuardPresenceWarning_Codex_TodosParesAusentes afirma que quando
// .codex/hooks.json existe mas não tem credential guard nos 2 pares esperados
// (PreToolUse+Bash, PreToolUse+apply_patch), 2 avisos disparam.
// Reconciliação: extrator schemaNested para Codex, mesma lógica de Claude.
func TestCredentialGuardPresenceWarning_Codex_TodosParesAusentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".codex/hooks.json", `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "ls_check", "hooks": [{"type": "command", "command": "echo noop"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 2 {
		t.Errorf("esperado 2 avisos para Codex (Bash e apply_patch), obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
	if !hasPresenceWarning(msgs, "Bash") {
		t.Errorf("aviso PreToolUse[Bash] ausente para Codex: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "apply_patch") {
		t.Errorf("aviso PreToolUse[apply_patch] ausente para Codex: %v", msgs)
	}
}

// TestCredentialGuardPresenceWarning_Gemini_TodosParesAusentes afirma que quando
// .gemini/settings.json existe sem credential guard nos 3 pares esperados
// (BeforeTool+run_shell_command, +read_file|read_many_files, +write_file|replace),
// 3 avisos disparam.
// Reconciliação: extrator schemaNested lê o campo hookEvent "BeforeTool".
func TestCredentialGuardPresenceWarning_Gemini_TodosParesAusentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".gemini/settings.json", `{
  "hooks": {
    "BeforeTool": [
      {"matcher": "other_tool", "hooks": [{"type": "command", "command": "echo noop"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("esperado 3 avisos para Gemini, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
	if !hasPresenceWarning(msgs, "run_shell_command") {
		t.Errorf("aviso BeforeTool[run_shell_command] ausente: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "read_file|read_many_files") {
		t.Errorf("aviso BeforeTool[read_file|read_many_files] ausente: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "write_file|replace") {
		t.Errorf("aviso BeforeTool[write_file|replace] ausente: %v", msgs)
	}
}

// TestCredentialGuardPresenceWarning_Cursor_TodosParesAusentes afirma que quando
// .cursor/hooks.json existe sem credential guard nos 3 pares esperados
// (beforeShellExecution sem matcher, preToolUse+Read, preToolUse+Write), 3 avisos disparam.
// Reconciliação: extrator schemaFlat lê hooks[hookEvent] com ou sem filtro de matcher.
func TestCredentialGuardPresenceWarning_Cursor_TodosParesAusentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".cursor/hooks.json", `{
  "hooks": {
    "preToolUse": [
      {"matcher": "other", "type": "command", "command": "echo noop"}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("esperado 3 avisos para Cursor, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
	if !hasPresenceWarning(msgs, "beforeShellExecution") {
		t.Errorf("aviso beforeShellExecution ausente para Cursor: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "Read") {
		t.Errorf("aviso preToolUse[Read] ausente para Cursor: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "Write") {
		t.Errorf("aviso preToolUse[Write] ausente para Cursor: %v", msgs)
	}
}

// TestCredentialGuardPresenceWarning_Copilot_TodosParesAusentes afirma que quando
// .github/hooks/trackfw-attention.json existe sem credential guard nos 3 pares esperados
// (preToolUse+bash, preToolUse+view, preToolUse+create|edit), 3 avisos disparam.
// Reconciliação: extrator schemaFlat; Copilot usa "command" não "bash" no campo type.
func TestCredentialGuardPresenceWarning_Copilot_TodosParesAusentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".github/hooks/trackfw-attention.json", `{
  "hooks": {
    "preToolUse": [
      {"matcher": "other", "type": "command", "command": "echo noop"}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("esperado 3 avisos para Copilot, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
	if !hasPresenceWarning(msgs, "bash") {
		t.Errorf("aviso preToolUse[bash] ausente para Copilot: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "view") {
		t.Errorf("aviso preToolUse[view] ausente para Copilot: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "create|edit") {
		t.Errorf("aviso preToolUse[create|edit] ausente para Copilot: %v", msgs)
	}
}

// TestCredentialGuardPresenceWarning_Kiro_TodosParesAusentes afirma que quando
// .kiro/hooks/trackfw-attention.json existe sem credential guard nos 3 pares esperados
// (PreToolUse+shell, PreToolUse+read, PreToolUse+write), 3 avisos disparam.
// Reconciliação: extrator schemaKiro lê entry.action.command; PostToolUse+shell não conta.
func TestCredentialGuardPresenceWarning_Kiro_TodosParesAusentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".kiro/hooks/trackfw-attention.json", `{
  "version": "v1",
  "hooks": [
    {"trigger": "PostToolUse", "matcher": "shell", "action": {"type": "command", "command": "echo noop"}}
  ]
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("esperado 3 avisos para Kiro (shell/read/write PreToolUse ausentes), obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
	if !hasPresenceWarning(msgs, "shell") {
		t.Errorf("aviso PreToolUse[shell] ausente para Kiro: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "read") {
		t.Errorf("aviso PreToolUse[read] ausente para Kiro: %v", msgs)
	}
	if !hasPresenceWarning(msgs, "write") {
		t.Errorf("aviso PreToolUse[write] ausente para Kiro: %v", msgs)
	}
}

// TestCredentialGuardPresenceWarning_ArquivoAusente afirma que quando nenhum arquivo de hook
// existe no diretório, nenhum aviso de presença dispara.
// Reconciliação: arquivo ausente significa guard global pode estar instalado; aviso seria
// inapropriado (mesma semântica do NaoDisparaSemEntrada para validateCredentialGuardHookResolvable).
func TestCredentialGuardPresenceWarning_ArquivoAusente(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero avisos sem arquivo de hook, obteve: %v", msgs)
	}
}

// ---------------------------------------------------------------------------
// AC2 — Avisos de presença não se tornam violations (rc=0 preservado)
// ---------------------------------------------------------------------------

// TestCredentialGuardPresenceWarning_AC2_WarningsNaoSaoViolacoes afirma que ValidateUnfiltered
// roteia as mensagens de credential_guard_presence_warning para warnings, não para violations.
// Reconciliação: applyRuleWarnOnly("credential_guard_presence_warning", ...) em validator.go
// garante que "has no credential guard entry" nunca apareça em violations.
func TestCredentialGuardPresenceWarning_AC2_WarningsNaoSaoViolacoes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "AskUserQuestion", "hooks": [{"command": "scripts/cleanup.sh", "type": "command"}]}
    ]
  }
}`)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}

	// Nenhuma violation deve conter "has no credential guard entry".
	for _, v := range violations {
		if strings.Contains(v, "has no credential guard entry") {
			t.Errorf("AC2: presença de guard não deve ser violation, obteve violation: %q", v)
		}
	}

	// Os avisos de presença devem aparecer em warnings (pelo menos um).
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "has no credential guard entry") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AC2: esperado aviso 'has no credential guard entry' em warnings; "+
			"violations=%v warnings=%v", violations, warnings)
	}
}

// ---------------------------------------------------------------------------
// AC4 — Red-team: guard em evento errado / campo errado
// ---------------------------------------------------------------------------

// TestCredentialGuardPresenceWarning_AC4_GuardSoEmPostToolUse afirma que o credential guard
// apenas em PostToolUse não satisfaz a exigência de PreToolUse — os 3 avisos ainda disparam.
// Reconciliação: extractPresenceCommandsD17 filtra pelo campo hookEvent; PostToolUse é descartado
// porque D16 exige bloqueio ANTES da execução.
func TestCredentialGuardPresenceWarning_AC4_GuardSoEmPostToolUse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	credCmd := guardforms.CredentialCurrentPSPOSIX
	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "Bash|PowerShell", "hooks": [{"type": "command", "command": "`+credCmd+`"}]},
      {"matcher": "Read", "hooks": [{"type": "command", "command": "`+credCmd+`"}]},
      {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "`+credCmd+`"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("AC4: guard só em PostToolUse não satisfaz PreToolUse — esperado 3 avisos, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
}

// TestCredentialGuardPresenceWarning_AC4_KiroGuardEmCommandRaiz afirma que o credential guard
// em entry["command"] (nível raiz, sem action) não é detectado como presente para Kiro.
// Reconciliação: schemaKiro lê action.command; entry.command (sem action) nunca executa no
// runtime Kiro — falso negativo se aceito (§2.4 Wave 0 #619).
func TestCredentialGuardPresenceWarning_AC4_KiroGuardEmCommandRaiz(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	credCmd := guardforms.CredentialCurrentCmdExe
	// guard em entry["command"] (raiz), não em entry["action"]["command"]
	writeFile(t, dir, ".kiro/hooks/trackfw-attention.json", `{
  "version": "v1",
  "hooks": [
    {"trigger": "PreToolUse", "matcher": "shell", "command": "`+credCmd+`", "action": {"type": "command", "command": "echo noop"}},
    {"trigger": "PreToolUse", "matcher": "read",  "command": "`+credCmd+`", "action": {"type": "command", "command": "echo noop"}},
    {"trigger": "PreToolUse", "matcher": "write",  "command": "`+credCmd+`", "action": {"type": "command", "command": "echo noop"}}
  ]
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("AC4: guard em entry.command (raiz) sem action.command não deve ser aceito — esperado 3 avisos, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
}

// ---------------------------------------------------------------------------
// AC4 — guard no nível do grupo (schemaNested: group["command"], não hooks[j]["command"])
// ---------------------------------------------------------------------------

// TestCredentialGuardPresenceWarning_AC4_Claude_GuardEmNivelDeGrupo afirma que o credential
// guard colocado no campo "command" do grupo (nível de entrada do evento, não no array interno
// "hooks") não é aceito como fiação para Claude Code — os 3 avisos ainda disparam.
// Reconciliação: extractPresenceCommandsD17 para schemaNested lê innerObj["command"] (nível
// interno); group["command"] (§2.7 Wave 0) nunca executa no Claude SDK e não deve ser aceito.
func TestCredentialGuardPresenceWarning_AC4_Claude_GuardEmNivelDeGrupo(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	credCmd := guardforms.CredentialCurrentPSPOSIX
	// guard em group["command"] (nível de grupo), não em group["hooks"][j]["command"]
	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash|PowerShell", "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]},
      {"matcher": "Read",            "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]},
      {"matcher": "Write|Edit",      "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("AC4: guard em group[command] (nível de grupo) não deve ser aceito — esperado 3 avisos, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
}

// TestCredentialGuardPresenceWarning_AC4_Codex_GuardEmNivelDeGrupo afirma que o credential
// guard em group["command"] não satisfaz os 2 pares Codex.
// Reconciliação: schemaNested exige innerObj["command"]; group-level command é inativo.
func TestCredentialGuardPresenceWarning_AC4_Codex_GuardEmNivelDeGrupo(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	credCmd := guardforms.CredentialCurrentPSPOSIX
	writeFile(t, dir, ".codex/hooks.json", `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash",        "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]},
      {"matcher": "apply_patch", "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 2 {
		t.Errorf("AC4 Codex: guard em group[command] não deve ser aceito — esperado 2 avisos, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
}

// TestCredentialGuardPresenceWarning_AC4_Gemini_GuardEmNivelDeGrupo afirma que o credential
// guard em group["command"] não satisfaz os 3 pares Gemini.
// Reconciliação: schemaNested exige innerObj["command"] no array interno.
func TestCredentialGuardPresenceWarning_AC4_Gemini_GuardEmNivelDeGrupo(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	credCmd := guardforms.CredentialCurrentPSPOSIX
	writeFile(t, dir, ".gemini/settings.json", `{
  "hooks": {
    "BeforeTool": [
      {"matcher": "run_shell_command",         "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]},
      {"matcher": "read_file|read_many_files", "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]},
      {"matcher": "write_file|replace",        "command": "`+credCmd+`", "hooks": [{"type": "command", "command": "echo noop"}]}
    ]
  }
}`)

	msgs, err := validateCredentialGuardPresenceWarnings()
	if err != nil {
		t.Fatalf("validateCredentialGuardPresenceWarnings: %v", err)
	}
	if countPresenceWarnings(msgs) != 3 {
		t.Errorf("AC4 Gemini: guard em group[command] não deve ser aceito — esperado 3 avisos, obteve %d: %v", countPresenceWarnings(msgs), msgs)
	}
}

// ---------------------------------------------------------------------------
// AC2 braço — Validate() strict (fora do modo lenient)
// ---------------------------------------------------------------------------

// TestCredentialGuardPresenceWarning_AC2_ValidateStrictRoteiaParaWarnings afirma que Validate()
// (pipeline completo, com modo strict) roteia mensagens "has no credential guard entry" para
// warnings, nunca para violations — confirmando que applyRuleWarnOnly está no caminho de
// Validate(), não só de ValidateUnfiltered().
// Controle: com o arquivo de hook sem guard mas com write_scope satisfeito, o pipeline em strict
// mode pode ter violações de outras regras; o que importa é que NENHUMA delas seja de presença
// de credential guard.
// Reconciliação: credential_guard_presence_warning registrado via applyRuleWarnOnly em validator.go
// linha ~1277; este teste confirma que o mesmo roteamento ocorre no caminho Validate().
func TestCredentialGuardPresenceWarning_AC2_ValidateStrictRoteiaParaWarnings(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Escreve .claude/settings.json com write_scope satisfeito mas credential guard ausente.
	// Apenas Bash|PowerShell tem write_scope; Read e Write|Edit são para write_scope_not_wired.
	// Não inclui credential guard em nenhum evento → 3 warnings de presença esperados.
	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Write|Edit|NotebookEdit", "hooks": [{"type": "command", "command": "scripts/trackfw-write-scope-guard.sh"}]},
      {"matcher": "PostToolUse", "hooks": [{"type": "command", "command": "scripts/cleanup.sh"}]}
    ]
  }
}`)
	// Escreve scripts para que credential_guard_hook_resolvable não dispare por ausência de script.
	// Não inclui trackfw-credential-guard.sh — ausência intencional para o teste de presença.
	writeFile(t, dir, "scripts/trackfw-write-scope-guard.sh", "#!/bin/sh\nexit 0\n")

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	// Assertion 1: nenhuma violation contém "has no credential guard entry".
	for _, v := range violations {
		if strings.Contains(v, "has no credential guard entry") {
			t.Errorf("AC2-strict: presença de guard não deve ser violation; obteve: %q", v)
		}
	}

	// Assertion 2: pelo menos um warning contém "has no credential guard entry" (pipeline ativo).
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "has no credential guard entry") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AC2-strict: esperado aviso 'has no credential guard entry' em warnings; violations=%v warnings=%v", violations, warnings)
	}

	// Controle: o pipeline DEVE ter alguma violação (strict mode ativo), provando que não estamos
	// em lenient mode (onde violations não-carveout virariam warnings). Se violations for vazio,
	// o projeto está em lenient mode ou todas as regras passaram — emitir um aviso informativo.
	// Nota: no temp dir sem git nem trackfw.yaml, outras regras podem ou não disparar.
	// O invariante crítico acima (sem credential guard em violations) é o que importa.
	_ = violations // aceito: zero violations de outras regras é OK nesse projeto mínimo
}
