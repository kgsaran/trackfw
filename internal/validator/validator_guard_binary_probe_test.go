package validator

// validator_guard_binary_probe_test.go — ML-2B (REQ-2026-09-05)
//
// Testes das novas regras e costuras introduzidas em ML-2B:
//   (a) linha exata de hook → ok; sufixo extra → violation; família errada → violation;
//       forma .sh legada → warning (never violation); variante --global.
//   (b) sonda do binário: ausente → violation; presente sem guard → violation;
//       presente com guard → ok; config só .sh → sonda não roda.
//   (c) Windows + .ps1 + Restricted → violation; RemoteSigned → ok; não-Windows → policyFn não chamada.
//   (d) trackfw.exe/.cmd/.bat na raiz → violation; ausente → ok; diretório de nome .exe → ok.
//   (e) ValidateTagged — TaggedMsg com Rule="trackfw_binary_in_project_root".
//
// Regra de reconciliação (CLAUDE.md — Regra Dura de Reconciliação):
// cada teste tem, no comentário de abertura, a conclusão do ML que ele afirma.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// --------------------------------------------------------------------------
// helpers locais
// --------------------------------------------------------------------------

// claudeSettingsWithSubcmd monta um .claude/settings.json com uma entrada PreToolUse
// usando o form de subcomando (não o .sh). Não usa guardExpectedLine — o literal é
// a fixture; se a implementação mudar a string, os testes de (a) reprovam.
func claudeSettingsWithSubcmd(command string) string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "` + command + `", "type": "command"}
        ]
      }
    ]
  }
}
`
}

// kiroAttentionWithSubcmd monta um .kiro/hooks/trackfw-attention.json com action.command.
// Família cmd.exe (no "; exit $LASTEXITCODE").
func kiroAttentionWithSubcmd(command string) string {
	return `{
  "version": "v1",
  "hooks": [
    {
      "name": "guard-pre",
      "trigger": "PreToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": "` + command + `"}
    }
  ]
}
`
}

// stubProbeOK substitui guardLookupBinary, guardRunProbe e guardFindGitBashExe por stubs que
// retornam sucesso (ou "sem Git Bash") e restaura os originais ao fim do teste.
// guardFindGitBashExe é stubado para retornar "" para garantir que o probe do Git Bash não
// execute em runners Windows reais onde um Git Bash pode estar instalado.
func stubProbeOK(t *testing.T) {
	t.Helper()
	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFindGitBash := guardFindGitBashExe
	guardLookupBinary = func(name string) (string, error) { return "/usr/local/bin/trackfw", nil }
	guardRunProbe = func(bin string) error { return nil }
	guardFindGitBashExe = func() string { return "" }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindGitBashExe = origFindGitBash
	})
}

// --------------------------------------------------------------------------
// (a) Linha exata / sufixo extra / família errada / legado / --global
// --------------------------------------------------------------------------

// Reconciliação: afirma que a linha exata PS/POSIX REQ-2026-10-20 para git-branch
// ("emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; ...")
// em .claude/settings.json não gera violation (REQ-2026-10-20: nova forma fail-closed com prefixo emulate -R sh).
func TestGuardHookResolvable_ExatoPSPosix_Ok(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	writeFile(t, dir, ".claude/settings.json",
		claudeSettingsWithSubcmd(`emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a linha D11 revised pré-REQ-2026-10-20 para git-branch (sem prefixo emulate -R sh)
// em .claude/settings.json não gera violation — é tratada como warning-only
// (via validateGitBranchGuardHookD11RevisedLegacyWarnings), não violation.
func TestGuardHookResolvable_D11RevLegacyPSPosix_SemViolation(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	writeFile(t, dir, ".claude/settings.json",
		claudeSettingsWithSubcmd(`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("D11 revised pré-REQ-2026-10-20 deve gerar 0 violations (só warning), obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a linha PS/POSIX com sufixo extra não é aceita como
// linha exata esperada e gera violation citando a linha correta.
func TestGuardHookResolvable_SufixoExtra_Violation(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	writeFile(t, dir, ".claude/settings.json",
		claudeSettingsWithSubcmd("trackfw guard git-branch; exit $LASTEXITCODE EXTRA"))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// Deve acusar: a linha não é exata e o expected form deve aparecer na mensagem.
	if !hasViolation(msgs, "expected form") && !hasViolation(msgs, "exit $LASTEXITCODE") {
		t.Errorf("esperado violation sobre linha errada, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a linha cmd.exe (sem "; exit $LASTEXITCODE") inserida em
// um arquivo de família PS/POSIX (.claude/settings.json) é rejeitada como forma errada.
func TestGuardHookResolvable_FamiliaCmdExeEmPSPosix_Violation(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	// Linha da família cmd.exe (sem sufixo) inserida em arquivo PS/POSIX.
	writeFile(t, dir, ".claude/settings.json",
		claudeSettingsWithSubcmd("trackfw guard git-branch"))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "expected form") && !hasViolation(msgs, "exit $LASTEXITCODE") {
		t.Errorf("esperado violation (linha cmd.exe em arquivo PSPosix), obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a linha exata cmd.exe ("trackfw guard git-branch") em
// .kiro/hooks/trackfw-attention.json (família cmd.exe) não gera violation.
func TestGuardHookResolvable_ExatoCmdExe_Ok(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)

	// Família cmd.exe: linha sem "; exit $LASTEXITCODE"
	writeFile(t, dir, ".kiro/hooks/trackfw-attention.json",
		kiroAttentionWithSubcmd("trackfw guard git-branch"))

	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs para forma cmd.exe correta, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a forma .sh legada em .claude/settings.json gera warning
// (via LegacyWarnings) e NÃO gera violation da regra principal.
func TestGuardHookResolvable_LegadoSh_Warning(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	writeFile(t, dir, ".claude/settings.json",
		gitBranchGuardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`))
	// Criar o script (para não gerar violation de "does not exist")
	scriptPath := filepath.Join(dir, "scripts", "trackfw-git-branch-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write: %v", err)
	}

	warnings, err := validateGitBranchGuardHookResolvableLegacyWarnings()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasWarning(warnings, "trackfw-git-branch-guard.sh") {
		t.Errorf("esperado warning sobre forma .sh legada, obteve: %v", warnings)
	}
	// A função PRINCIPAL (não a legacy) não deve emitir violation para forma .sh quando o script existe.
	msgs, err := validateGitBranchGuardHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// Não deve haver violation (o script existe e é executável), e a LegacyWarnings não é chamada aqui.
	if hasViolation(msgs, "trackfw-git-branch-guard.sh") {
		t.Errorf("não esperado violation para .sh com script presente, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a variante global credential D11 com "--global" na linha esperada
// ("$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE")
// é aceita sem violation (ML-6B: D11 fail-closed).
func TestGuardGlobalHookResolvable_CredentialComGlobal_Ok(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)
	home := globalGuardHome(t)

	// ~/.claude/settings.json com linha global correta (D11).
	claudePath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudePath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(claudePath,
		[]byte(claudeSettingsWithSubcmd(`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential --global; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`)),
		0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	msgs, err := validateCredentialGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs para linha global correta, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que a linha global credential SEM "--global"
// ("trackfw guard credential; exit $LASTEXITCODE") é rejeitada — o subcmdMarker
// coincide mas não é a linha exata esperada ("trackfw guard credential --global; ...").
func TestGuardGlobalHookResolvable_CredentialSemGlobal_Violation(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	dir := t.TempDir()
	chdir(t, dir)
	stubProbeOK(t)
	home := globalGuardHome(t)

	claudePath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudePath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Linha sem "--global" — contém o marker mas não é a linha exata global.
	if err := os.WriteFile(claudePath,
		[]byte(claudeSettingsWithSubcmd("trackfw guard credential; exit $LASTEXITCODE")),
		0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	msgs, err := validateCredentialGuardGlobalHookResolvable()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// A linha sem --global deve gerar violation (não é a linha exata esperada).
	if !hasViolation(msgs, "expected form") && !hasViolation(msgs, "--global") {
		t.Errorf("esperado violation para linha sem --global, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// (b) Sonda do binário
// --------------------------------------------------------------------------

// Reconciliação: afirma que, quando o binário não está no PATH, guardBinaryProbeOnce
// retorna violation mencionando "not found in PATH".
func TestGuardBinaryProbeOnce_BinarioAusente_Violation(t *testing.T) {
	origLookup := guardLookupBinary
	guardLookupBinary = func(name string) (string, error) {
		return "", errors.New("not found")
	}
	t.Cleanup(func() { guardLookupBinary = origLookup })

	msgs := guardBinaryProbeOnce(false)
	if !hasViolation(msgs, "not found in PATH") {
		t.Errorf("esperado violation 'not found in PATH', obteve: %v", msgs)
	}
}

// Reconciliação: afirma que, quando o binário existe mas `guard --help` sai ≠ 0,
// guardBinaryProbeOnce retorna violation mencionando "does not have the guard subcommand".
func TestGuardBinaryProbeOnce_BinarioSemGuard_Violation(t *testing.T) {
	origLookup := guardLookupBinary
	origRun := guardRunProbe
	guardLookupBinary = func(name string) (string, error) { return "/usr/local/bin/trackfw", nil }
	guardRunProbe = func(bin string) error { return errors.New("exit status 2") }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
	})

	msgs := guardBinaryProbeOnce(false)
	if !hasViolation(msgs, "does not have the guard subcommand") {
		t.Errorf("esperado violation de guard ausente, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que, quando o binário existe e tem o guard, guardBinaryProbeOnce
// retorna 0 mensagens (não-Windows, sem PS1).
func TestGuardBinaryProbeOnce_BinarioComGuard_Ok(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })
	stubProbeOK(t)

	msgs := guardBinaryProbeOnce(false)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que, quando o config usa APENAS a forma .sh legada (não o
// subcomando), a costura guardLookupBinary não é chamada (contador = 0).
func TestGuardBinaryProbeOnce_ConfigSoLegado_SondaNaoRoda(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	callCount := 0
	origLookup := guardLookupBinary
	guardLookupBinary = func(name string) (string, error) {
		callCount++
		return "/usr/local/bin/trackfw", nil
	}
	t.Cleanup(func() { guardLookupBinary = origLookup })

	// Configuração com APENAS a forma .sh legada — não deve acionar a sonda.
	writeFile(t, dir, ".claude/settings.json",
		gitBranchGuardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`))
	// O script pode estar ausente — a sonda não deve ser chamada de jeito nenhum.

	_, _ = validateGitBranchGuardHookResolvable()

	if callCount != 0 {
		t.Errorf("sonda não deveria ser chamada para config só .sh; callCount = %d", callCount)
	}
}

// --------------------------------------------------------------------------
// (c) Windows + PS1 + Restricted / RemoteSigned / ilegível / não-Windows
// --------------------------------------------------------------------------

// Reconciliação: afirma que Windows + trackfw resolving para .ps1 + política Restricted
// gera violation com a mensagem exata de política Restricted (não a de política ilegível).
func TestGuardBinaryProbeOnce_WindowsPS1Restricted_Violation(t *testing.T) {
	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindTrackfwForPS
	origPolicy := guardGetPSPolicy
	guardLookupBinary = func(name string) (string, error) { return `C:\tools\trackfw.exe`, nil }
	guardRunProbe = func(bin string) error { return nil }
	guardFindTrackfwForPS = func() string { return `C:\npm\trackfw.ps1` }
	guardGetPSPolicy = func() string { return "Restricted" }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindTrackfwForPS = origFind
		guardGetPSPolicy = origPolicy
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	// Assert the Restricted-specific message, not the unreadable-policy message.
	if !hasViolation(msgs, "under Restricted execution policy") {
		t.Errorf("esperado violation com 'under Restricted execution policy', obteve: %v", msgs)
	}
	if hasViolation(msgs, "could not be read") {
		t.Errorf("mensagem de política ilegível não deve aparecer para Restricted, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que Windows + .ps1 + política RemoteSigned NÃO gera violation.
func TestGuardBinaryProbeOnce_WindowsPS1RemoteSigned_Ok(t *testing.T) {
	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindTrackfwForPS
	origPolicy := guardGetPSPolicy
	origFindGitBash := guardFindGitBashExe
	guardLookupBinary = func(name string) (string, error) { return `C:\tools\trackfw.exe`, nil }
	guardRunProbe = func(bin string) error { return nil }
	guardFindTrackfwForPS = func() string { return `C:\npm\trackfw.ps1` }
	guardGetPSPolicy = func() string { return "RemoteSigned" }
	guardFindGitBashExe = func() string { return "" } // isolate from real Git Bash on Windows CI
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindTrackfwForPS = origFind
		guardGetPSPolicy = origPolicy
		guardFindGitBashExe = origFindGitBash
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs para RemoteSigned, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que, quando GOOS != "windows", guardGetPSPolicy não é chamada
// (contador = 0), provando que a costura de política é Windows-only.
func TestGuardBinaryProbeOnce_NaoWindows_PolicyNaoEhChamada(t *testing.T) {
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindTrackfwForPS
	origPolicy := guardGetPSPolicy
	guardLookupBinary = func(name string) (string, error) { return "/usr/local/bin/trackfw", nil }
	guardRunProbe = func(bin string) error { return nil }
	policyCallCount := 0
	guardFindTrackfwForPS = func() string { policyCallCount++; return "/usr/local/bin/trackfw.ps1" }
	guardGetPSPolicy = func() string { policyCallCount++; return "Restricted" }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindTrackfwForPS = origFind
		guardGetPSPolicy = origPolicy
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs (não-Windows), obteve: %v", msgs)
	}
	if policyCallCount != 0 {
		t.Errorf("costuras de política não devem ser chamadas fora do Windows; callCount = %d", policyCallCount)
	}
}

// Reconciliação: afirma que Windows + trackfw resolvendo para .exe (não .ps1) + política
// Restricted NÃO gera violation — o discriminante é ".ps1", não a política.
func TestGuardBinaryProbeOnce_WindowsExeRestrictedOk(t *testing.T) {
	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindTrackfwForPS
	origPolicy := guardGetPSPolicy
	origFindGitBash := guardFindGitBashExe
	guardLookupBinary = func(name string) (string, error) { return `C:\tools\trackfw.exe`, nil }
	guardRunProbe = func(bin string) error { return nil }
	guardFindTrackfwForPS = func() string { return `C:\tools\trackfw.exe` } // .exe, não .ps1
	guardGetPSPolicy = func() string { return "Restricted" }
	guardFindGitBashExe = func() string { return "" } // isolate from real Git Bash on Windows CI
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindTrackfwForPS = origFind
		guardGetPSPolicy = origPolicy
		guardFindGitBashExe = origFindGitBash
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs para .exe (discriminante é .ps1), obteve: %v", msgs)
	}
}

// Reconciliação: afirma que Windows + .ps1 + política Restricted não chama guardFindGitBashExe
// (contador = 0) — o retorno definitivo da sonda de política encerra o probe antes de chegar
// à sonda do Git Bash (AC6 ML-1B: Restricted é certo, não chega a precisar do Git Bash).
func TestGuardBinaryProbeOnce_WindowsPS1Restricted_GitBashNaoRoda(t *testing.T) {
	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindTrackfwForPS
	origPolicy := guardGetPSPolicy
	origFindGitBash := guardFindGitBashExe
	findGitBashCallCount := 0
	guardLookupBinary = func(name string) (string, error) { return `C:\tools\trackfw.exe`, nil }
	guardRunProbe = func(bin string) error { return nil }
	guardFindTrackfwForPS = func() string { return `C:\npm\trackfw.ps1` }
	guardGetPSPolicy = func() string { return "Restricted" }
	guardFindGitBashExe = func() string { findGitBashCallCount++; return `C:\Program Files\Git\bin\bash.exe` }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindTrackfwForPS = origFind
		guardGetPSPolicy = origPolicy
		guardFindGitBashExe = origFindGitBash
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	if !hasViolation(msgs, "under Restricted execution policy") {
		t.Errorf("esperado violation com 'under Restricted execution policy', obteve: %v", msgs)
	}
	if findGitBashCallCount != 0 {
		t.Errorf("guardFindGitBashExe não deve ser chamada com Restricted (retorno definitivo); callCount=%d", findGitBashCallCount)
	}
}

// Reconciliação: afirma que Windows + .ps1 + política ilegível ("") + sonda do Git Bash com
// violação retorna AS DUAS mensagens — a de política ilegível e a do Git Bash — sem que o
// aviso incerto esconda o achado certo (AC6 ML-1B: um aviso incerto não pode ocultar um
// achado certo; a sonda do Git Bash ainda é executada após o aviso de ilegível).
func TestGuardBinaryProbeOnce_WindowsPS1PolicyUnreadableWithGitBashViolation_BothMessagesAppear(t *testing.T) {
	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindTrackfwForPS
	origPolicy := guardGetPSPolicy
	origFindGitBash := guardFindGitBashExe
	origLoginProbe := guardRunBashLoginProbe
	guardLookupBinary = func(name string) (string, error) { return `C:\tools\trackfw.exe`, nil }
	guardRunProbe = func(bin string) error { return nil } // PATH binary has guard
	guardFindTrackfwForPS = func() string { return `C:\npm\trackfw.ps1` }
	guardGetPSPolicy = func() string { return "" } // unreadable
	guardFindGitBashExe = func() string { return `C:\Program Files\Git\bin\bash.exe` }
	// Git Bash login: trackfw not found → "exit 127" violation
	guardRunBashLoginProbe = func(_ string) (string, bool) { return "", true }
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindTrackfwForPS = origFind
		guardGetPSPolicy = origPolicy
		guardFindGitBashExe = origFindGitBash
		guardRunBashLoginProbe = origLoginProbe
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	if len(msgs) != 2 {
		t.Fatalf("esperado exatamente 2 mensagens (ilegível + Git Bash), obteve %d: %v", len(msgs), msgs)
	}
	if !hasViolation(msgs, "could not be read") {
		t.Errorf("msgs[0] deve conter 'could not be read', obteve: %v", msgs)
	}
	if !hasViolation(msgs, "Git Bash") {
		t.Errorf("msgs[1] deve conter 'Git Bash', obteve: %v", msgs)
	}
}

// Reconciliação: afirma que Windows + trackfw resolvendo para .ps1 + política ilegível ("")
// emite aviso de fail-open (política não pôde ser lida), distinto da mensagem de Restricted.
// Afirma o AC6 do ML-1B: com policy == "", guardCheckPS1RestrictedPolicy não fica em silêncio.
func TestGuardBinaryProbeOnce_WindowsPS1PolicyUnreadable_Warning(t *testing.T) {
	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

	origLookup := guardLookupBinary
	origRun := guardRunProbe
	origFind := guardFindTrackfwForPS
	origPolicy := guardGetPSPolicy
	origFindGitBash := guardFindGitBashExe
	guardLookupBinary = func(name string) (string, error) { return `C:\tools\trackfw.exe`, nil }
	guardRunProbe = func(bin string) error { return nil }
	guardFindTrackfwForPS = func() string { return `C:\npm\trackfw.ps1` }
	guardGetPSPolicy = func() string { return "" }
	guardFindGitBashExe = func() string { return "" } // isola do Git Bash real no Windows CI
	t.Cleanup(func() {
		guardLookupBinary = origLookup
		guardRunProbe = origRun
		guardFindTrackfwForPS = origFind
		guardGetPSPolicy = origPolicy
		guardFindGitBashExe = origFindGitBash
	})

	msgs := guardBinaryProbeOnce(true /* hasPSPosixEntry */)
	// A mensagem de política ilegível deve estar presente.
	if !hasViolation(msgs, "could not be read") {
		t.Errorf("esperado mensagem de política ilegível com 'could not be read', obteve: %v", msgs)
	}
	// A mensagem específica de Restricted NÃO deve aparecer — são casos distintos.
	if hasViolation(msgs, "under Restricted execution policy") {
		t.Errorf("mensagem de Restricted não deve aparecer para política ilegível, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// (d) trackfw.exe/.cmd/.bat na raiz
// --------------------------------------------------------------------------

// Reconciliação: afirma que trackfw.exe na raiz do projeto gera violation citando "cmd.exe".
func TestValidateTrackfwBinaryInProjectRoot_ExePresente_Violation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "trackfw.exe"), []byte{}, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	msgs, err := validateTrackfwBinaryInProjectRoot()
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !hasViolation(msgs, "trackfw.exe") || !hasViolation(msgs, "cmd.exe") {
		t.Errorf("esperado violation para trackfw.exe, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que trackfw.cmd na raiz do projeto gera violation.
func TestValidateTrackfwBinaryInProjectRoot_CmdPresente_Violation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "trackfw.cmd"), []byte{}, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	msgs, err := validateTrackfwBinaryInProjectRoot()
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !hasViolation(msgs, "trackfw.cmd") {
		t.Errorf("esperado violation para trackfw.cmd, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que trackfw.bat na raiz do projeto gera violation.
func TestValidateTrackfwBinaryInProjectRoot_BatPresente_Violation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "trackfw.bat"), []byte{}, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	msgs, err := validateTrackfwBinaryInProjectRoot()
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !hasViolation(msgs, "trackfw.bat") {
		t.Errorf("esperado violation para trackfw.bat, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que, sem nenhum trackfw.{exe,cmd,bat} na raiz, a regra não gera violation.
func TestValidateTrackfwBinaryInProjectRoot_Ausente_Ok(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	msgs, err := validateTrackfwBinaryInProjectRoot()
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado 0 msgs, obteve: %v", msgs)
	}
}

// Reconciliação: afirma que um DIRETÓRIO chamado "trackfw.exe" não gera violation
// (IsDir() guarda a regra contra falso-positivo por diretório com extensão .exe).
func TestValidateTrackfwBinaryInProjectRoot_DiretorioNomeadoExe_Ok(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.Mkdir(filepath.Join(dir, "trackfw.exe"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	msgs, err := validateTrackfwBinaryInProjectRoot()
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("diretório não deve gerar violation, obteve: %v", msgs)
	}
}

// --------------------------------------------------------------------------
// (e) ValidateTagged — Rule propagada até TaggedMsg
// --------------------------------------------------------------------------

// Reconciliação: afirma que ValidateTagged propaga Rule="trackfw_binary_in_project_root"
// no TaggedMsg quando trackfw.exe existe na raiz do projeto. Este é o loop-breaker
// para o site applyRuleTagged em validateUnfilteredTagged: sem ele, o site pode
// estar ausente sem causar falha de compilação.
func TestValidateTagged_TrackfwBinaryInProjectRoot_TemRule(t *testing.T) {
	dir := t.TempDir()
	// Estrutura mínima para ValidateTagged não falhar por erro de leitura.
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/req",
		"docs/adr",
	)
	chdir(t, dir)
	stubProbeOK(t)

	if err := os.WriteFile(filepath.Join(dir, "trackfw.exe"), []byte{}, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	violations, warnings, err := ValidateTagged()
	if err != nil {
		t.Fatalf("ValidateTagged erro: %v", err)
	}

	all := append(append([]TaggedMsg{}, violations...), warnings...)
	found := false
	for _, m := range all {
		if m.Rule == "trackfw_binary_in_project_root" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("esperado TaggedMsg com Rule=trackfw_binary_in_project_root; violations=%v warnings=%v", violations, warnings)
	}
}
