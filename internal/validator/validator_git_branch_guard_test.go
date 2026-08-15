package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

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
          {"command": "` + scriptAbsPath + `", "type": "command"}
        ]
      }
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

// TestGitBranchGuardGlobal_SemWiringGlobalHoje_Silencio — divergência de design documentada: hoje
// nenhum harnessGitBranchGuardTarget* existe em internal/generators/agentfiles.go (só o script
// GLOBAL é gerado por `trackfw update harness`, GenerateGlobalGitBranchGuardScript — nunca
// referenciado em nenhum hooks.json/settings.json global). Então, mesmo com o script global
// presente, nenhum arquivo de config global o referencia — o mecanismo genérico
// (validateGuardGlobalHookResolvable/validateGuardGlobalScriptIntegrity) fica corretamente em
// silêncio até essa wiring existir. Ver o relatório final do ML-1A para a nota completa.
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
