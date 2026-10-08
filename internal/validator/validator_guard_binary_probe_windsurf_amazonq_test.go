package validator

// validator_guard_binary_probe_windsurf_amazonq_test.go — ML-2C (REQ-2026-09-05)
//
// Testes para a leitura das configs de hook de Windsurf e Amazon Q pelo validator:
//   (a) Windsurf: linha exata PS/POSIX → ok; linha cmd.exe (família errada) → violation;
//       forma .sh legada → warning (não violation).
//   (b) Amazon Q: linha exata cmd.exe → ok; linha PS/POSIX (família errada) → violation;
//       forma .sh legada → warning (não violation).
//
// Regra de reconciliação (CLAUDE.md — Regra Dura de Reconciliação):
// cada teste tem, no comentário de abertura, a conclusão do ML que ele afirma.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// --------------------------------------------------------------------------
// helpers locais — fixtures de JSON para Windsurf e Amazon Q
// --------------------------------------------------------------------------

// windsurfHooksWithSubcmd monta um .windsurf/hooks.json com um entry pre_run_command
// usando o form de subcomando (não o .sh). O campo "show_output" é mantido para fidelidade
// ao schema do gerador (InjectWindsurfHooks).
func windsurfHooksWithSubcmd(command string) string {
	return `{
  "hooks": {
    "pre_run_command": [
      {
        "command": "` + command + `",
        "show_output": true
      }
    ]
  }
}
`
}

// amazonQAgentWithSubcmd monta um .amazonq/cli-agents/q_cli_default.json com um entry
// hooks.preToolUse[execute_bash] usando o form de subcomando.
// Estrutura confirmada por InjectAmazonQHooks (agentfiles.go) e TestInjectAmazonQHooks_CreateAndIdempotent.
func amazonQAgentWithSubcmd(command string) string {
	return `{
  "name": "q_cli_default",
  "tools": ["*"],
  "hooks": {
    "preToolUse": [
      {
        "matcher": "execute_bash",
        "hooks": [
          {"command": "` + command + `"}
        ]
      }
    ]
  }
}
`
}

// --------------------------------------------------------------------------
// (a) Windsurf
// --------------------------------------------------------------------------

// Reconciliação: afirma que a linha exata PS/POSIX D11 para git-branch
// ("$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE")
// em .windsurf/hooks.json é reconhecida como forma correta (arquivo lido — sonda do binário disparada)
// e não gera violation (ML-6B: D11 fail-closed).
func TestGuardHookResolvable_Windsurf_ExatoPSPosix_Ok(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)

	// Contador de sonda: a sonda do binário só dispara quando a linha exata é reconhecida
	// (anySubcmdFormFound=true). Contar chamadas a guardLookupBinary prova que o arquivo
	// foi lido E que a linha foi identificada como forma de subcomando válida.
	calls := 0
	origLookup := guardLookupBinary
	origRun := guardRunProbe
	guardLookupBinary = func(name string) (string, error) {
		calls++
		return "/usr/local/bin/trackfw", nil
	}
	guardRunProbe = func(bin string) error { return nil }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
	})

	writeFile(t, dir, ".windsurf/hooks.json",
		windsurfHooksWithSubcmd(`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs para linha exata Windsurf PS/POSIX, obteve: %v", msgs)
	}
	if calls == 0 {
		t.Error("sonda do binário não foi disparada — arquivo Windsurf pode não ter sido lido ou linha não reconhecida como subcomando")
	}
}

// Reconciliação: afirma que a linha cmd.exe ("trackfw guard git-branch", sem "; exit $LASTEXITCODE")
// inserida em .windsurf/hooks.json (família PS/POSIX) gera violation citando a linha correta
// e nomeando o artefato ".windsurf/hooks.json".
func TestGuardHookResolvable_Windsurf_FamiliaCmdExe_Violation(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	// Linha cmd.exe (sem sufixo) em arquivo de família PS/POSIX.
	writeFile(t, dir, ".windsurf/hooks.json",
		windsurfHooksWithSubcmd("trackfw guard git-branch"))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, ".windsurf/hooks.json") {
		t.Errorf("esperado violation nomeando .windsurf/hooks.json, obteve: %v", msgs)
	}
	if !hasViolation(msgs, "exit $LASTEXITCODE") {
		t.Errorf("esperado violation citando a linha esperada com '; exit $LASTEXITCODE', obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a forma .sh legada em .windsurf/hooks.json ("bash scripts/...")
// gera warning (via LegacyWarnings) e NÃO gera violation da regra principal quando o script existe.
func TestGuardHookResolvable_Windsurf_LegadoSh_Warning(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Forma legada: "bash scripts/trackfw-git-branch-guard.sh"
	writeFile(t, dir, ".windsurf/hooks.json",
		windsurfHooksWithSubcmd("bash scripts/trackfw-git-branch-guard.sh"))

	// Criar o script para que a resolução do caminho (após strip do "bash ") não falhe por ausência.
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	warnings, err := validateGitBranchGuardHookResolvableLegacyWarnings()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasWarning(warnings, "trackfw-git-branch-guard.sh") {
		t.Errorf("esperado warning sobre forma .sh legada em Windsurf, obteve: %v", warnings)
	}

	// A regra principal não deve emitir violation quando o script existe.
	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if hasViolation(msgs, "trackfw-git-branch-guard.sh") {
		t.Errorf("não esperado violation para .sh legado Windsurf com script presente, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// (b) Amazon Q
// --------------------------------------------------------------------------

// Reconciliação: afirma que a linha exata cmd.exe para git-branch
// ("trackfw guard git-branch") em .amazonq/cli-agents/q_cli_default.json é reconhecida
// como forma correta (arquivo lido — sonda do binário disparada) e não gera violation.
func TestGuardHookResolvable_AmazonQ_ExatoCmdExe_Ok(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)

	// Contador de sonda: mesmo princípio de TestGuardHookResolvable_Windsurf_ExatoPSPosix_Ok.
	calls := 0
	origLookup := guardLookupBinary
	origRun := guardRunProbe
	guardLookupBinary = func(name string) (string, error) {
		calls++
		return "/usr/local/bin/trackfw", nil
	}
	guardRunProbe = func(bin string) error { return nil }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
	})

	writeFile(t, dir, ".amazonq/cli-agents/q_cli_default.json",
		amazonQAgentWithSubcmd("trackfw guard git-branch"))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs para linha exata Amazon Q cmd.exe, obteve: %v", msgs)
	}
	if calls == 0 {
		t.Error("sonda do binário não foi disparada — arquivo Amazon Q pode não ter sido lido ou linha não reconhecida como subcomando")
	}
}

// Reconciliação: afirma que a linha PS/POSIX ("trackfw guard git-branch; exit $LASTEXITCODE")
// inserida em .amazonq/cli-agents/q_cli_default.json (família cmd.exe) gera violation
// nomeando o artefato e citando a linha correta (sem sufixo).
func TestGuardHookResolvable_AmazonQ_FamiliaPSPosix_Violation(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	// Linha PS/POSIX (com sufixo) em arquivo de família cmd.exe.
	writeFile(t, dir, ".amazonq/cli-agents/q_cli_default.json",
		amazonQAgentWithSubcmd("trackfw guard git-branch; exit $LASTEXITCODE"))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "q_cli_default.json") && !hasViolation(msgs, "Amazon Q") {
		t.Errorf("esperado violation nomeando q_cli_default.json ou Amazon Q, obteve: %v", msgs)
	}
	if !hasViolation(msgs, "trackfw guard git-branch") {
		t.Errorf("esperado violation citando a linha esperada 'trackfw guard git-branch', obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a forma .sh legada em .amazonq/cli-agents/q_cli_default.json
// ("scripts/trackfw-git-branch-guard.sh", caminho relativo puro) gera warning (via LegacyWarnings)
// e NÃO gera violation da regra principal quando o script existe.
func TestGuardHookResolvable_AmazonQ_LegadoSh_Warning(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Forma legada: caminho relativo puro (sem prefixo "bash ")
	writeFile(t, dir, ".amazonq/cli-agents/q_cli_default.json",
		amazonQAgentWithSubcmd("scripts/trackfw-git-branch-guard.sh"))

	// Criar o script para que a resolução do caminho não falhe por ausência.
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	warnings, err := validateGitBranchGuardHookResolvableLegacyWarnings()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasWarning(warnings, "trackfw-git-branch-guard.sh") {
		t.Errorf("esperado warning sobre forma .sh legada em Amazon Q, obteve: %v", warnings)
	}

	// A regra principal não deve emitir violation quando o script existe.
	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if hasViolation(msgs, "trackfw-git-branch-guard.sh") {
		t.Errorf("não esperado violation para .sh legado Amazon Q com script presente, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1B (REQ-2026-10-06) — credential guard presence check
// Windsurf e Amazon Q: arquivo presente sem credential guard → violation.
// Arquivo com credential guard correto → sem violation.
// --------------------------------------------------------------------------

// windsurfHooksWithCredentialGuard monta um .windsurf/hooks.json com entradas
// de credential guard em pre_run_command e pre_write_code, usando a linha
// PS/POSIX esperada pelo ML-1B.
func windsurfHooksWithCredentialGuard() string {
	cmd := `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
	return `{
  "hooks": {
    "pre_run_command": [
      {"command": "` + cmd + `", "show_output": true}
    ],
    "pre_write_code": [
      {"command": "` + cmd + `", "show_output": true}
    ]
  }
}
`
}

// amazonQAgentWithCredentialGuard monta um .amazonq/cli-agents/q_cli_default.json
// com entries de credential guard em execute_bash e fs_write.
func amazonQAgentWithCredentialGuard() string {
	cmd := "trackfw guard credential || exit 2"
	return `{
  "name": "q_cli_default",
  "tools": ["*"],
  "hooks": {
    "preToolUse": [
      {
        "matcher": "execute_bash",
        "hooks": [{"command": "` + cmd + `"}]
      },
      {
        "matcher": "fs_write",
        "hooks": [{"command": "` + cmd + `"}]
      }
    ]
  }
}
`
}

// Reconciliação: afirma que validateCredentialGuardHookResolvable emite violation
// quando .windsurf/hooks.json existe mas tem apenas git-branch guard, sem credential
// guard (ficheiro típico de instalação anterior ao ML-1B).
// S3 falsification target: este teste deve FALHAR antes do ML-1B e PASSAR depois.
func TestCredentialGuardHookResolvable_Windsurf_SemGuard_Violation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	// Só git-branch guard — sem credential guard.
	writeFile(t, dir, ".windsurf/hooks.json",
		windsurfHooksWithSubcmd(`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, ".windsurf/hooks.json") {
		t.Errorf("esperado violation nomeando .windsurf/hooks.json (sem credential guard), obteve: %v", msgs)
	}
	if !hasViolation(msgs, "credential guard") {
		t.Errorf("esperado violation mencionando 'credential guard', obteve: %v", msgs)
	}
}

// Reconciliação: afirma que validateCredentialGuardHookResolvable NÃO emite
// nenhuma mensagem que nomeie .windsurf/hooks.json quando o arquivo contém o
// credential guard nos dois eventos corretos (arquivo instalado pelo ML-1B).
func TestCredentialGuardHookResolvable_Windsurf_ComGuard_Ok(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	writeFile(t, dir, ".windsurf/hooks.json", windsurfHooksWithCredentialGuard())

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	for _, m := range msgs {
		if hasViolation([]string{m}, ".windsurf/hooks.json") {
			t.Errorf("violation inesperada nomeando .windsurf/hooks.json com credential guard correto: %v", m)
		}
	}
}

// Reconciliação: afirma que validateCredentialGuardHookResolvable emite violation
// para Windsurf quando o credential guard está em pre_run_command mas ausente em
// pre_write_code — cobertura parcial não é suficiente (Wave 0: "nos eventos decididos").
func TestCredentialGuardHookResolvable_Windsurf_SoPreRunCommand_Violation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	cmd := `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
	// Só pre_run_command — pre_write_code ausente.
	writeFile(t, dir, ".windsurf/hooks.json", `{
  "hooks": {
    "pre_run_command": [{"command": "`+cmd+`", "show_output": true}]
  }
}
`)
	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "pre_write_code") {
		t.Errorf("esperado violation para pre_write_code ausente em .windsurf/hooks.json, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que validateCredentialGuardHookResolvable emite violation
// quando .amazonq/cli-agents/q_cli_default.json existe mas tem apenas git-branch
// guard, sem credential guard (ficheiro típico de instalação anterior ao ML-1B).
func TestCredentialGuardHookResolvable_AmazonQ_SemGuard_Violation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	// Só git-branch guard — sem credential guard.
	writeFile(t, dir, ".amazonq/cli-agents/q_cli_default.json",
		amazonQAgentWithSubcmd("trackfw guard git-branch || exit 2"))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, ".amazonq/cli-agents/q_cli_default.json") {
		t.Errorf("esperado violation nomeando .amazonq/cli-agents/q_cli_default.json (sem credential guard), obteve: %v", msgs)
	}
	if !hasViolation(msgs, "credential guard") {
		t.Errorf("esperado violation mencionando 'credential guard', obteve: %v", msgs)
	}
}

// Reconciliação: afirma que validateCredentialGuardHookResolvable NÃO emite
// nenhuma mensagem que nomeie .amazonq/cli-agents/q_cli_default.json quando o
// arquivo contém o credential guard nos dois eventos corretos (arquivo instalado
// pelo ML-1B).
func TestCredentialGuardHookResolvable_AmazonQ_ComGuard_Ok(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	writeFile(t, dir, ".amazonq/cli-agents/q_cli_default.json", amazonQAgentWithCredentialGuard())

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	for _, m := range msgs {
		if hasViolation([]string{m}, ".amazonq/cli-agents/q_cli_default.json") {
			t.Errorf("violation inesperada nomeando .amazonq/cli-agents/q_cli_default.json com credential guard correto: %v", m)
		}
	}
}

// Reconciliação: afirma que validateCredentialGuardHookResolvable emite violation
// para Amazon Q quando o credential guard está em execute_bash mas ausente em
// fs_write — cobertura parcial não é suficiente (Wave 0: "nos eventos decididos").
func TestCredentialGuardHookResolvable_AmazonQ_SoExecuteBash_Violation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	cmd := "trackfw guard credential || exit 2"
	// Só execute_bash — fs_write ausente.
	writeFile(t, dir, ".amazonq/cli-agents/q_cli_default.json", `{
  "name": "q_cli_default",
  "tools": ["*"],
  "hooks": {
    "preToolUse": [
      {"matcher": "execute_bash", "hooks": [{"command": "`+cmd+`"}]}
    ]
  }
}
`)
	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "fs_write") {
		t.Errorf("esperado violation para fs_write ausente em .amazonq/cli-agents/q_cli_default.json, obteve: %v", msgs)
	}
}
