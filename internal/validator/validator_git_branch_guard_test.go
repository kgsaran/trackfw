package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// jsonStringLiteral serializa s como um literal de string JSON (com as aspas), via encoding/json —
// usado pelos helpers de fixture abaixo para embutir caminhos absolutos dentro de um template JSON
// sem concatenação crua. ROADMAP-2026-09-04-fixture-de-guard-produz-json-invalido-com-caminho-
// nativo-do-windows, ML-6C: caminhos nativos do Windows carregam "\", que a concatenação anterior
// (` + "`\"" + `..." + s + "\"`" + `) nunca escapava — json.Marshal escapa cada "\" como "\\" por
// especificação, exatamente como o produto já faz em encoding/json (agentfiles.go); o teste volta a
// ler um arquivo válido em qualquer separador.
func jsonStringLiteral(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// s é sempre uma string Go válida (caminho de arquivo); Marshal de string nunca falha.
		panic(err)
	}
	return string(b)
}

// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
// ML-1A.

// gitBranchGuardEntryClaudeSettings monta um .claude/settings.json mínimo com uma entrada de
// git-branch-guard apontando para scriptCmd (valor bruto do campo "command") — mesmo padrão de
// guardEntryClaudeSettings (validator_credential_guard_test.go), só o marker muda.
func gitBranchGuardEntryClaudeSettings(scriptCmd string) string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "` + scriptCmd + `", "type": "command"}
        ]
      }
    ]
  }
}
`
}

// ---- git_branch_guard_hook_resolvable (projeto) ----

func TestGitBranchGuardHookResolvable_DisparaScriptAusente(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", gitBranchGuardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`))
	// scripts/trackfw-git-branch-guard.sh NÃO é criado — ausência proposital.

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateGitBranchGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "does not exist") || !hasViolation(msgs, ".claude/settings.json") || !hasViolation(msgs, "trackfw-git-branch-guard.sh") {
		t.Errorf("esperado violation de script ausente, obteve: %v", msgs)
	}
}

func TestGitBranchGuardHookResolvable_DisparaScriptNaoExecutavel(t *testing.T) {
	// O bit de execução não é representável em NTFS: no Windows info.Mode()&0111
	// é 0 para todo arquivo, inclusive após os.Chmod(0o755). Este teste afirma
	// que a regra DISPARA quando o bit falta, o que só tem sentido onde o bit
	// existe — fixamos a plataforma em vez de pular, para que a garantia
	// continue verificada em qualquer host.
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", gitBranchGuardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`))
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0644); err != nil { // sem bit +x
		t.Fatalf("write script: %v", err)
	}

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateGitBranchGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "not executable") {
		t.Errorf("esperado violation de script não executável, obteve: %v", msgs)
	}
}

func TestGitBranchGuardHookResolvable_NaoDisparaSemEntrada(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "AskUserQuestion", "hooks": [{"command": "scripts/trackfw-attention-cleanup.sh", "type": "command"}]}
    ]
  }
}
`)

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateGitBranchGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations sem entrada de guard, obteve: %v", msgs)
	}
}

func TestGitBranchGuardHookResolvable_NaoDisparaScriptPresenteEExecutavel(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", gitBranchGuardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`))
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte(gitBranchGuardScriptReference), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateGitBranchGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations com script presente e executável, obteve: %v", msgs)
	}
}

// TestGitBranchGuardHookResolvable_DisparaFormaRelativaAntigaEmClaude — prova que a mesma regra
// requiresVarOrShellPrefix cobre o git-branch-guard (a função validateGuardHookResolvable é
// compartilhada; um único flag na tabela credentialGuardHookFiles cobre ambos os guards).
func TestGitBranchGuardHookResolvable_DisparaFormaRelativaAntigaEmClaude(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", gitBranchGuardEntryClaudeSettings(`scripts/trackfw-git-branch-guard.sh`))
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateGitBranchGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "bare relative path") || !hasViolation(msgs, ".claude/settings.json") {
		t.Errorf("esperado violation de forma relativa antiga em Claude para git-branch-guard, obteve: %v", msgs)
	}
}

// ---- git_branch_guard_script_integrity (projeto) ----

func TestGitBranchGuardScriptIntegrity_ScriptAusente_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// scripts/trackfw-git-branch-guard.sh NÃO existe — cobertura de ausência é
	// git_branch_guard_hook_resolvable, não esta regra.
	msgs, err := validateGitBranchGuardScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio com script ausente, obteve: %v", msgs)
	}
}

func TestGitBranchGuardScriptIntegrity_ScriptIdenticoAoTemplate_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "scripts/trackfw-git-branch-guard.sh", gitBranchGuardScriptReference)

	msgs, err := validateGitBranchGuardScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio com script idêntico ao template, obteve: %v", msgs)
	}
}

// TestGitBranchGuardScriptIntegrity_UmByteAlterado_Dispara — 1 byte alterado no meio do template
// (não um "exit 0" no-op inteiro) já é suficiente para disparar a regra de integridade.
func TestGitBranchGuardScriptIntegrity_UmByteAlterado_Dispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	tampered := gitBranchGuardScriptReference[:len(gitBranchGuardScriptReference)-1] + "X"
	writeFile(t, dir, "scripts/trackfw-git-branch-guard.sh", tampered)

	msgs, err := validateGitBranchGuardScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "scripts/trackfw-git-branch-guard.sh") || !hasViolation(msgs, "diverges from the template") {
		t.Fatalf("esperado violation de divergência, obteve: %v", msgs)
	}
}

// TestGitBranchGuardScriptIntegrity_SeverityDefaultWarning — a regra tem severidade default
// "warning" (mesmo raciocínio de credential_guard_script_integrity: o script não carrega
// marcador de versão, não dá para distinguir drift legítimo de adulteração).
func TestGitBranchGuardScriptIntegrity_SeverityDefaultWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "scripts/trackfw-git-branch-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() erro: %v", err)
	}
	if hasViolation(violations, "trackfw-git-branch-guard.sh") {
		t.Errorf("esperado warning (não violation) por default, obteve violations: %v", violations)
	}
	if !hasWarning(warnings, "trackfw-git-branch-guard.sh") {
		t.Errorf("esperado warning de integridade por default, obteve: %v", warnings)
	}
}

// ---- Escopo global (credential-guard e git-branch-guard) ----

// globalGuardHome cria um $HOME isolado (t.TempDir) e aponta a variável de ambiente HOME para lá,
// isolando os testes de escopo global do $HOME real da máquina.
func globalGuardHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// globalClaudeSettingsWithCommand monta ~/.claude/settings.json com uma entrada global
// PreToolUse[Bash] apontando para o caminho absoluto scriptAbsPath — mesma forma que
// harnessCredentialGuardTargetClaude (internal/generators/update.go) escreve.
func globalClaudeSettingsWithCommand(scriptAbsPath string) string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": ` + jsonStringLiteral(scriptAbsPath) + `, "type": "command"}
        ]
      }
    ]
  }
}
`
}

// globalClaudeSettingsWithCommandNoType is globalClaudeSettingsWithCommand's ROADMAP-2026-08-17
// ML-4B counterpart: the "type":"command" field is deliberately OMITTED — the exact malformed
// shape hades-tf's ML-4A barrier finding reproduced (correct command, missing type, script
// present and integro, "nenhum dos dois escopos protege, e tudo fica verde").
func globalClaudeSettingsWithCommandNoType(scriptAbsPath string) string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": ` + jsonStringLiteral(scriptAbsPath) + `}
        ]
      }
    ]
  }
}
`
}

// globalCursorHooksWithCommand monta ~/.cursor/hooks.json com uma entrada global
// beforeShellExecution apontando para scriptAbsPath — mesma forma que
// harnessCredentialGuardTargetCursor/harnessGitBranchGuardTargetCursor escrevem. Cursor's schema
// never carries a "type" field at all (ROADMAP-2026-08-17 ML-4B) — this fixture is the
// non-regression control proving requiresCommandType=false for Cursor is not over-tightened.
func globalCursorHooksWithCommand(scriptAbsPath string) string {
	return `{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {"command": ` + jsonStringLiteral(scriptAbsPath) + `}
    ]
  }
}
`
}

// TestGuardGlobalHookResolvable_SemEntradaGlobal_Silencio — sem NENHUMA entrada global
// referenciando o marker em nenhum dos 6 arquivos, não é violação (nenhuma dependência real).
func TestGuardGlobalHookResolvable_SemEntradaGlobal_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	globalGuardHome(t)
	t.Cleanup(config.Reset)

	msgs, err := validateCredentialGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations sem entrada global, obteve: %v", msgs)
	}

	gmsgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(gmsgs) != 0 {
		t.Errorf("esperado zero violations sem entrada global (git-branch-guard), obteve: %v", gmsgs)
	}
}

// TestGuardGlobalHookResolvable_GlobalInstaladoEIntegro_Silencio — o gap principal que este ML
// fecha: hook de PROJETO ausente (dedup) + global instalado E íntegro → silêncio (dedup
// preservado).
func TestGuardGlobalHookResolvable_GlobalInstaladoEIntegro_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	// Nenhum hook de PROJETO (.claude/settings.json não existe no dir do projeto) — simula dedup
	// ativo (globalCredentialGuardInstalledClaude() teria retornado true na geração).
	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if err := os.MkdirAll(filepath.Dir(globalScriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalScriptPath, []byte(credentialGuardGlobalScriptReference), 0755); err != nil {
		t.Fatalf("write global script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(globalClaudeSettingsWithCommand(globalScriptPath)), 0644); err != nil {
		t.Fatalf("write global settings: %v", err)
	}

	hookMsgs, err := validateCredentialGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(hookMsgs) != 0 {
		t.Errorf("esperado zero violations com global instalado e executável, obteve: %v", hookMsgs)
	}

	integrityMsgs, err := validateCredentialGuardGlobalScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(integrityMsgs) != 0 {
		t.Errorf("esperado zero violations com script global íntegro, obteve: %v", integrityMsgs)
	}
}

// TestGuardGlobalHookResolvable_GlobalInstaladoMasScriptAusente_Dispara — o gap principal: hook de
// PROJETO ausente + global REGISTRADO em ~/.claude/settings.json mas o script global não existe no
// disco → antes deste ML, `trackfw validate` silenciava; agora deve violar.
func TestGuardGlobalHookResolvable_GlobalInstaladoMasScriptAusente_Dispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	// Script global NÃO é criado — ausência proposital, apesar de estar registrado no settings.json.
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(globalClaudeSettingsWithCommand(globalScriptPath)), 0644); err != nil {
		t.Fatalf("write global settings: %v", err)
	}

	msgs, err := validateCredentialGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "does not exist") || !hasViolation(msgs, "global scope") || !hasViolation(msgs, "trackfw update harness") {
		t.Errorf("esperado violation de script global ausente, obteve: %v", msgs)
	}
}

// ---------------------------------------------------------------------------
// ROADMAP-2026-08-17 ML-4B — hades-tf ML-4A barrier finding reproduced: a global config entry
// with the CORRECT command but MISSING "type":"command" (script present and íntegro) makes
// neither the dedup NOR this rule notice anything wrong — "nenhum dos dois escopos protege, e
// tudo fica verde". Before this ML, collectCommandsWithMarker only cared about the string value,
// never the structural "type" sibling, so this exact fixture produced zero violations.
// ---------------------------------------------------------------------------

// TestGuardGlobalHookResolvable_MalformedTypeMissing_Dispara reproduces the hades-tf ML-4A
// barrier finding exactly: script present+executable+correct path, but the hook entry is missing
// "type":"command" — Claude Code silently never executes it. Before this ML: silence. After:
// violation.
func TestGuardGlobalHookResolvable_MalformedTypeMissing_Dispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(globalScriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalScriptPath, []byte(gitBranchGuardScriptReference), 0755); err != nil {
		t.Fatalf("write global script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(globalClaudeSettingsWithCommandNoType(globalScriptPath)), 0644); err != nil {
		t.Fatalf("write global settings: %v", err)
	}

	msgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, `missing "type":"command"`) || !hasViolation(msgs, "global scope") || !hasViolation(msgs, "Claude Code") || !hasViolation(msgs, "trackfw update harness") {
		t.Errorf("esperado violation de entrada estruturalmente malformada (type ausente), obteve: %v", msgs)
	}
	// Discriminante central: NÃO deve ser a mensagem de "does not exist" — o script existe e é
	// executável, só a forma estrutural da entrada é que está errada.
	if hasViolation(msgs, "but the script does not exist") || hasViolation(msgs, "but the script is not executable") {
		t.Errorf("mensagem errada: script existe e é executável, o problema é a ausência de \"type\", obteve: %v", msgs)
	}
}

// TestGuardGlobalHookResolvable_Cursor_MissingTypeIsNormal_Silencio is the non-regression
// control: Cursor's schema never carries a "type" field, so its absence is normal, not malformed
// — requiresCommandType=false for Cursor must not be over-tightened by this ML.
func TestGuardGlobalHookResolvable_Cursor_MissingTypeIsNormal_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(globalScriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalScriptPath, []byte(gitBranchGuardScriptReference), 0755); err != nil {
		t.Fatalf("write global script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".cursor", "hooks.json"), []byte(globalCursorHooksWithCommand(globalScriptPath)), 0644); err != nil {
		t.Fatalf("write global cursor hooks: %v", err)
	}

	msgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("Cursor nunca carrega campo \"type\" — ausência é normal, não malformada; esperado zero violations, obteve: %v", msgs)
	}
}

// TestGitBranchGuardHookResolvable_ProjectMalformedTypeMissing_Dispara is the PROJECT-scope
// counterpart of TestGuardGlobalHookResolvable_MalformedTypeMissing_Dispara — same discriminant,
// validateGuardHookResolvable (validator_credential_guard.go) instead of the global variant.
func TestGitBranchGuardHookResolvable_ProjectMalformedTypeMissing_Dispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"}
        ]
      }
    ]
  }
}
`)
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte(gitBranchGuardScriptReference), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateGitBranchGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, `missing "type":"command"`) || !hasViolation(msgs, ".claude/settings.json") || !hasViolation(msgs, "Claude Code") {
		t.Errorf("esperado violation de entrada estruturalmente malformada (type ausente), obteve: %v", msgs)
	}
	if hasViolation(msgs, "but the script does not exist") || hasViolation(msgs, "but the script is not executable") {
		t.Errorf("mensagem errada: script existe e é executável, o problema é a ausência de \"type\", obteve: %v", msgs)
	}
}

// TestGuardGlobalScriptIntegrity_GlobalInstaladoMasScriptCorrompido_Dispara — mesmo gap acima,
// mas para o script global corrompido/desatualizado (existe, mas conteúdo diverge do template).
func TestGuardGlobalScriptIntegrity_GlobalInstaladoMasScriptCorrompido_Dispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	if err := os.MkdirAll(filepath.Dir(globalScriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalScriptPath, []byte("#!/usr/bin/env bash\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write corrupted global script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(globalClaudeSettingsWithCommand(globalScriptPath)), 0644); err != nil {
		t.Fatalf("write global settings: %v", err)
	}

	msgs, err := validateCredentialGuardGlobalScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "diverges from the template") || !hasViolation(msgs, "global scope") || !hasViolation(msgs, "trackfw update harness") {
		t.Errorf("esperado violation de integridade global, obteve: %v", msgs)
	}
}

// TestGitBranchGuardGlobal_SemWiringGlobalHoje_Silencio — atualizado no ML-3B: a fiação global do
// git-branch-guard EXISTE desde a Wave 2 (ML-2A), mas este teste não a exercita — nenhum dos 6
// arquivos de globalGuardConfigFiles é escrito no fixture. Prova o caso "script global presente,
// nenhum config o referencia" (usuário rodou `update harness` só parcialmente, ou nunca instalou a
// fiação de nenhum CLI): validateGitBranchGuardGlobalHookResolvable deve permanecer em silêncio —
// hook_resolvable é condicionado à fiação por desenho (ver nota do ML-3B no roadmap: "resolvibilidade
// pergunta 'o hook aponta para algo que existe', o que só faz sentido havendo hook").
func TestGitBranchGuardGlobal_SemWiringGlobalHoje_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(globalScriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalScriptPath, []byte(gitBranchGuardScriptReference), 0755); err != nil {
		t.Fatalf("write global script: %v", err)
	}
	// Nenhum ~/.claude/settings.json (ou equivalente) referencia trackfw-git-branch-guard.sh hoje.

	msgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio (sem wiring global hoje), obteve: %v", msgs)
	}
}

// ---- git_branch_guard_script_integrity / credential_guard_script_integrity (escopo GLOBAL,
// disparo por EXISTÊNCIA do artefato — ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
// projeto-e-integridade-independente-de-fiacao, ML-3A) ----

// TestGuardGlobalScriptIntegrity_DisparaSemNenhumaFiacao — o discriminante central deste ML: o
// script global existe e diverge do template, mas ZERO arquivo de config (nenhum dos 6
// globalGuardConfigFiles) referencia o marker. Antes deste ML, o laço da regra antiga nunca
// entrava e o script podia apodrecer indefinidamente — foi assim que o git-branch-guard real de KG
// ficou 3 versões atrasado com `validate` verde (motivação da REQ).
func TestGuardGlobalScriptIntegrity_DisparaSemNenhumaFiacao(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(globalScriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalScriptPath, []byte("#!/usr/bin/env bash\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write corrupted global script: %v", err)
	}
	// Nenhum arquivo de config é escrito neste $HOME — nenhuma fiação existe.

	msgs, err := validateGitBranchGuardGlobalScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "diverges from the template") || !hasViolation(msgs, globalScriptPath) {
		t.Errorf("esperado violation de integridade global mesmo sem fiação, obteve: %v", msgs)
	}
}

// TestGuardGlobalScriptIntegrity_AusenciaDoArtefato_Silencio — script global nunca instalado (nem
// arquivo, nem fiação) não é erro: instalar o harness global é opcional, e falso-positivo aqui
// afetaria todo usuário que nunca rodou `trackfw update harness`.
func TestGuardGlobalScriptIntegrity_AusenciaDoArtefato_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	globalGuardHome(t)
	t.Cleanup(config.Reset)

	msgs, err := validateGitBranchGuardGlobalScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio com script global ausente, obteve: %v", msgs)
	}

	cmsgs, err := validateCredentialGuardGlobalScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(cmsgs) != 0 {
		t.Errorf("esperado silêncio (credential-guard) com script global ausente, obteve: %v", cmsgs)
	}
}

// TestGuardGlobalScriptIntegrity_NaoDuplicaComDoisConfigsReferenciandoOMesmoScript — prova de
// "sem dupla emissão": o MESMO script corrompido é referenciado por 2 arquivos de config
// diferentes (Claude E Codex) — antes deste ML, o laço antigo iterava por config e emitiria 2
// mensagens; a checagem por existência do artefato avalia o caminho fixo em disco uma única vez,
// então o resultado tem que ser exatamente 1 mensagem, nunca 2.
func TestGuardGlobalScriptIntegrity_NaoDuplicaComDoisConfigsReferenciandoOMesmoScript(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	globalScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(globalScriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(globalScriptPath, []byte("#!/usr/bin/env bash\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write corrupted global script: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(globalClaudeSettingsWithCommand(globalScriptPath)), 0644); err != nil {
		t.Fatalf("write claude settings: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "hooks.json"), []byte(globalClaudeSettingsWithCommand(globalScriptPath)), 0644); err != nil {
		t.Fatalf("write codex hooks: %v", err)
	}

	msgs, err := validateGitBranchGuardGlobalScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("esperado exatamente 1 mensagem (script referenciado por 2 configs), obteve %d: %v", len(msgs), msgs)
	}
}

// ---- git_branch_guard_hook_resolvable / credential_guard_hook_resolvable (escopo GLOBAL, arquivo
// DEDICADO do Kiro — ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-
// integridade-independente-de-fiacao, ML-3B) ----

// kiroGlobalGuardFixture monta o documento que harnessCredentialGuardTargetKiro/
// harnessGitBranchGuardTargetKiro (internal/generators/update.go) escrevem de fato —
// {"version":"v1","hooks":[{"name","description","trigger","matcher","action":{"type":"command",
// "command":scriptAbsPath}}, ...]} com dois hooks pre/post — usando hookNamePrefix para distinguir
// "trackfw-credential-guard" de "trackfw-git-branch-guard" (mesma convenção
// "<tool>-<guard>-global-pre/-post" que os dois writers reais usam).
func kiroGlobalGuardFixture(hookNamePrefix, scriptAbsPath string) string {
	return `{
  "version": "v1",
  "hooks": [
    {
      "name": "` + hookNamePrefix + `-global-pre",
      "description": "global pre hook",
      "trigger": "PreToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": ` + jsonStringLiteral(scriptAbsPath) + `}
    },
    {
      "name": "` + hookNamePrefix + `-global-post",
      "description": "global post hook",
      "trigger": "PostToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": ` + jsonStringLiteral(scriptAbsPath) + `}
    }
  ]
}
`
}

// TestGitBranchGuardGlobalHookResolvable_KiroDedicatedFile_DisparaScriptAusente — o discriminante
// central do ML-3B: antes dele, globalGuardConfigFiles só apontava Kiro para
// trackfw-credential-guard.json (para os dois guards), então ~/.kiro/hooks/
// trackfw-git-branch-guard.json (escrito por harnessGitBranchGuardTargetKiro desde a Wave 2)
// nunca era lido por validateGitBranchGuardGlobalHookResolvable — um hook Kiro apontando para
// script ausente passava limpo. Agora deve violar.
func TestGitBranchGuardGlobalHookResolvable_KiroDedicatedFile_DisparaScriptAusente(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	// scriptPath NÃO é criado — ausência proposital.
	if err := os.MkdirAll(filepath.Join(home, ".kiro", "hooks"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json"),
		[]byte(kiroGlobalGuardFixture("trackfw-git-branch-guard", scriptPath)),
		0644,
	); err != nil {
		t.Fatalf("write kiro git-branch-guard hooks: %v", err)
	}

	msgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "does not exist") || !hasViolation(msgs, "trackfw-git-branch-guard.json") || !hasViolation(msgs, "Kiro") {
		t.Errorf("esperado violation do arquivo dedicado do Kiro para git-branch-guard, obteve: %v", msgs)
	}
}

// TestGitBranchGuardGlobalHookResolvable_KiroDedicatedFile_NaoDisparaScriptPresenteEExecutavel —
// simétrico ao teste acima: script presente e executável não deve violar (prova que o teste acima
// não é vácuo por outro motivo).
func TestGitBranchGuardGlobalHookResolvable_KiroDedicatedFile_NaoDisparaScriptPresenteEExecutavel(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	scriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte(gitBranchGuardScriptReference), 0755); err != nil {
		t.Fatalf("write global script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".kiro", "hooks"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json"),
		[]byte(kiroGlobalGuardFixture("trackfw-git-branch-guard", scriptPath)),
		0644,
	); err != nil {
		t.Fatalf("write kiro git-branch-guard hooks: %v", err)
	}

	msgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations com script Kiro presente e executável, obteve: %v", msgs)
	}
}

// TestGuardGlobalHookResolvable_KiroDoisArquivosDedicados_NaoRegrideNaoDuplica — não-regressão
// (AC "credential-guard do Kiro inalterado; sem duplicar aviso"): com OS DOIS arquivos dedicados do
// Kiro presentes simultaneamente (trackfw-credential-guard.json E trackfw-git-branch-guard.json),
// cada um referenciando um script ausente distinto, cada regra deve reportar exatamente 1 violation
// — nunca 0 (regressão de não-cobertura) nem 2+ (dupla contagem entre os dois arquivos/guards).
func TestGuardGlobalHookResolvable_KiroDoisArquivosDedicados_NaoRegrideNaoDuplica(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := globalGuardHome(t)
	t.Cleanup(config.Reset)

	credScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	gbgScriptPath := filepath.Join(home, ".trackfw", "scripts", "trackfw-git-branch-guard.sh")
	// Nenhum dos dois scripts é criado — ambos ausentes propositalmente.

	if err := os.MkdirAll(filepath.Join(home, ".kiro", "hooks"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".kiro", "hooks", "trackfw-credential-guard.json"),
		[]byte(kiroGlobalGuardFixture("trackfw-credential-guard", credScriptPath)),
		0644,
	); err != nil {
		t.Fatalf("write kiro credential-guard hooks: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".kiro", "hooks", "trackfw-git-branch-guard.json"),
		[]byte(kiroGlobalGuardFixture("trackfw-git-branch-guard", gbgScriptPath)),
		0644,
	); err != nil {
		t.Fatalf("write kiro git-branch-guard hooks: %v", err)
	}

	credMsgs, err := validateCredentialGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado (credential-guard): %v", err)
	}
	if len(credMsgs) != 1 {
		t.Errorf("esperado exatamente 1 violation (credential-guard do Kiro), obteve %d: %v", len(credMsgs), credMsgs)
	}

	gbgMsgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado (git-branch-guard): %v", err)
	}
	if len(gbgMsgs) != 1 {
		t.Errorf("esperado exatamente 1 violation (git-branch-guard do Kiro), obteve %d: %v", len(gbgMsgs), gbgMsgs)
	}
}

// TestGitBranchGuardGlobalHookResolvable_KiroSemArquivoDedicado_Silencio — ausência do arquivo
// dedicado (usuário nunca rodou `update harness --targets kiro-git-branch-guard`) permanece
// silenciosa — mesmo contrato fail-open de todos os outros 5 CLIs.
func TestGitBranchGuardGlobalHookResolvable_KiroSemArquivoDedicado_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	globalGuardHome(t)
	t.Cleanup(config.Reset)

	msgs, err := validateGitBranchGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio sem arquivo dedicado do Kiro, obteve: %v", msgs)
	}
}

// TestGitBranchGuardHookResolvable_WindowsNaoDisparaBitDeExecucao — espelha
// TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao para a regra do
// git branch guard. Ver o comentário de CurrentGOOS em goos.go.
func TestGitBranchGuardHookResolvable_WindowsNaoDisparaBitDeExecucao(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	writeFile(t, dir, ".claude/settings.json", gitBranchGuardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`))
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0644); err != nil { // sem bit +x
		t.Fatalf("write script: %v", err)
	}

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateGitBranchGuardHookResolvable() erro: %v", err)
	}
	if hasViolation(msgs, "not executable") {
		t.Errorf("no Windows o bit de execução não é representável — a regra não deve disparar. Obteve: %v", msgs)
	}
	if hasViolation(msgs, "does not exist") {
		t.Errorf("script existe; violation de ausência não deveria aparecer: %v", msgs)
	}
}
