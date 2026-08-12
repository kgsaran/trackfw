package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.

// ---- credential_guard_script_integrity ----

func TestCredentialGuardScriptIntegrity_ScriptAusente_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// scripts/trackfw-credential-guard.sh NÃO existe — cobertura de ausência é
	// credential_guard_hook_resolvable, não esta regra.
	msgs, err := validateCredentialGuardScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio com script ausente, obteve: %v", msgs)
	}
}

func TestCredentialGuardScriptIntegrity_ScriptIdenticoAoTemplate_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "scripts/trackfw-credential-guard.sh", credentialGuardScriptReference)

	msgs, err := validateCredentialGuardScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio com script idêntico ao template, obteve: %v", msgs)
	}
}

// TestCredentialGuardScriptIntegrity_ScriptDivergente_Dispara cobre a via de SOBRESCRITA: script
// trocado por um "exit 0" no-op — passa em os.Stat e no bit 0111, então
// credential_guard_hook_resolvable não pega. A mensagem deve ser causalmente neutra
// (ADR-2026-08-12 Emenda 3): não afirma "adulterado".
func TestCredentialGuardScriptIntegrity_ScriptDivergente_Dispara(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")

	msgs, err := validateCredentialGuardScriptIntegrity()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "scripts/trackfw-credential-guard.sh") || !hasViolation(msgs, "diverges from the template") {
		t.Fatalf("esperado violation de divergência, obteve: %v", msgs)
	}
	for _, m := range msgs {
		lower := strings.ToLower(m)
		for _, forbidden := range []string{"adulterad", "modified by", "tampered"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("mensagem não deve afirmar causalidade — encontrado %q em: %q", forbidden, m)
			}
		}
	}
}

// ---- credential_guard_mode_downgrade ----

// commitTrackfwYAML escreve trackfw.yaml com content e commita — usado para preparar o estado de
// HEAD que a regra lê via `git show HEAD:./trackfw.yaml`.
func commitTrackfwYAML(t *testing.T, dir, content string) {
	t.Helper()
	writeFile(t, dir, "trackfw.yaml", content)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("add", "trackfw.yaml")
	run("commit", "-m", "trackfw.yaml")
}

func TestCredentialGuardModeDowngrade_SemGit_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio sem repositório git, obteve: %v", msgs)
	}
}

func TestCredentialGuardModeDowngrade_SemCommits_Silencio(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio sem nenhum commit, obteve: %v", msgs)
	}
}

func TestCredentialGuardModeDowngrade_ArquivoNaoVersionadoNoHEAD_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// trackfw.yaml existe no disco mas nunca foi commitado — sem HEAD para este arquivo.
	writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio com trackfw.yaml não versionado, obteve: %v", msgs)
	}
}

func TestCredentialGuardModeDowngrade_HEADSemChaveMode_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	commitTrackfwYAML(t, dir, "roadmap_dir: docs/roadmaps\n")
	// Disco agora tenta "block" — mas sem âncora de block no HEAD, não há o que detectar.
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\ncredential_guard:\n  mode: warn\n")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio quando HEAD não tem credential_guard.mode, obteve: %v", msgs)
	}
}

func TestCredentialGuardModeDowngrade_HEADWarn_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	commitTrackfwYAML(t, dir, "credential_guard:\n  mode: warn\n")
	writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: block\n")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("regra é direcional (block->não-block); HEAD warn nunca deve disparar, obteve: %v", msgs)
	}
}

func TestCredentialGuardModeDowngrade_SemMudanca_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
	// Disco idêntico ao HEAD — sem downgrade.

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado silêncio sem downgrade, obteve: %v", msgs)
	}
}

// TestCredentialGuardModeDowngrade_BlockParaWarn_Dispara cobre a via de DOWNGRADE explícito.
func TestCredentialGuardModeDowngrade_BlockParaWarn_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
	writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "credential_guard.mode: block") {
		t.Fatalf("esperado violation de downgrade block->warn, obteve: %v", msgs)
	}
}

// TestCredentialGuardModeDowngrade_ChaveRemovidaNoDisco_Dispara cobre remover o bloco
// credential_guard inteiro do trackfw.yaml em disco, mantendo o restante do arquivo — a leitura
// desta chave em disco NÃO é um caso de silêncio (ver comentário de
// validateCredentialGuardModeDowngrade): é exatamente a via que a regra existe para cobrir.
func TestCredentialGuardModeDowngrade_ChaveRemovidaNoDisco_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	commitTrackfwYAML(t, dir, "roadmap_dir: docs/roadmaps\ncredential_guard:\n  mode: block\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "credential_guard.mode: block") {
		t.Fatalf("esperado violation quando a chave credential_guard.mode some do disco, obteve: %v", msgs)
	}
}

// TestCredentialGuardScriptIntegrity_ConfiguravelViaRules prova que credential_guard_script_integrity
// respeita rules: no trackfw.yaml (default warning, per ruleDefaults; pode virar error ou off).
func TestCredentialGuardScriptIntegrity_ConfiguravelViaRules(t *testing.T) {
	t.Run("default_warning", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "scripts/trackfw-credential-guard.sh") {
			t.Errorf("default deveria ser warning (não violation), violations: %v", violations)
		}
		if !hasWarning(warnings, "scripts/trackfw-credential-guard.sh") {
			t.Errorf("esperado warning por default, obteve warnings: %v", warnings)
		}
	})

	t.Run("error", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
		writeFile(t, dir, "trackfw.yaml", "rules:\n  credential_guard_script_integrity: error\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "scripts/trackfw-credential-guard.sh") {
			t.Errorf("rules: error deveria promover a violation, obteve: %v", violations)
		}
	})

	t.Run("off", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/trackfw-credential-guard.sh", "#!/usr/bin/env bash\nexit 0\n")
		writeFile(t, dir, "trackfw.yaml", "rules:\n  credential_guard_script_integrity: off\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "scripts/trackfw-credential-guard.sh") || hasWarning(warnings, "scripts/trackfw-credential-guard.sh") {
			t.Errorf("rules: off deveria silenciar totalmente, violations=%v warnings=%v", violations, warnings)
		}
	})
}

// TestCredentialGuardModeDowngrade_ConfiguravelViaRules prova que credential_guard_mode_downgrade
// respeita rules: no trackfw.yaml (default error, sem entrada em ruleDefaults; pode virar warning
// ou off).
func TestCredentialGuardModeDowngrade_ConfiguravelViaRules(t *testing.T) {
	build := func(t *testing.T, extraYAML string) string {
		t.Helper()
		dir := t.TempDir()
		initGitRepo(t, dir, "main")
		commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
		writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n"+extraYAML)
		return dir
	}

	t.Run("default_error", func(t *testing.T) {
		dir := build(t, "")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "credential_guard.mode: block") {
			t.Errorf("default deveria ser error (violation), obteve: %v", violations)
		}
	})

	t.Run("warning", func(t *testing.T) {
		dir := build(t, "rules:\n  credential_guard_mode_downgrade: warning\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "credential_guard.mode: block") {
			t.Errorf("rules: warning não deveria gerar violation, obteve: %v", violations)
		}
		if !hasWarning(warnings, "credential_guard.mode: block") {
			t.Errorf("esperado warning, obteve: %v", warnings)
		}
	})

	t.Run("off", func(t *testing.T) {
		dir := build(t, "rules:\n  credential_guard_mode_downgrade: off\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "credential_guard.mode: block") || hasWarning(warnings, "credential_guard.mode: block") {
			t.Errorf("rules: off deveria silenciar totalmente, violations=%v warnings=%v", violations, warnings)
		}
	})
}

// TestCredentialGuardModeDowngrade_ArquivoDeletadoNoDisco_Dispara cobre a DELEÇÃO total de
// trackfw.yaml após um commit com mode: block.
func TestCredentialGuardModeDowngrade_ArquivoDeletadoNoDisco_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
	if err := os.Remove(filepath.Join(dir, "trackfw.yaml")); err != nil {
		t.Fatalf("remove trackfw.yaml: %v", err)
	}

	msgs, err := validateCredentialGuardModeDowngrade()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(msgs, "credential_guard.mode: block") {
		t.Fatalf("esperado violation quando trackfw.yaml é deletado após mode: block no HEAD, obteve: %v", msgs)
	}
}
