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
// ou off) — MAS só quando a mudança de rules: está COMMITADA no HEAD.
//
// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ADR-2026-08-12-
// severidade-das-regras-de-credential-guard-...: antes deste ADR, este teste commitava só
// "mode: block" e escrevia "rules: <nome>: warning|off" em disco SEM commitar — exatamente o
// auto-silenciamento sem rastro que o ADR fecha. As subtests "warning"/"off" abaixo agora commitam
// a mudança de rules: junto (fluxo legítimo, ADR §4); as novas subtests
// "*_nao_commitado_ainda_dispara" provam que a mesma edição, SEM commit, não tem efeito algum —
// o canal que o ADR fecha.
func TestCredentialGuardModeDowngrade_ConfiguravelViaRules(t *testing.T) {
	t.Run("default_error", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "main")
		commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
		writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\n")
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

	t.Run("warning_commitado", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "main")
		// O mantenedor commita a decisão de rebaixar a SEVERIDADE da regra (mas mode: block
		// permanece o valor commitado — a âncora do HEAD, se sido removida, faria a regra
		// silenciar por falta de âncora, um teste diferente). Depois, localmente, mode: warn em
		// disco (não commitado) é o que efetivamente dispara a comparação disco×HEAD; rules:
		// warning continua presente em disco (não foi removido pela mesma edição) — reflete o
		// fluxo legítimo do ADR §4: severidade rebaixada por commit, não por edição silenciosa.
		commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\nrules:\n  credential_guard_mode_downgrade: warning\n")
		writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: warning\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "credential_guard.mode: block") {
			t.Errorf("rules: warning commitado não deveria gerar violation, obteve: %v", violations)
		}
		if !hasWarning(warnings, "credential_guard.mode: block") {
			t.Errorf("esperado warning, obteve: %v", warnings)
		}
	})

	t.Run("off_commitado", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "main")
		commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\nrules:\n  credential_guard_mode_downgrade: off\n")
		writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: off\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "credential_guard.mode: block") || hasWarning(warnings, "credential_guard.mode: block") {
			t.Errorf("rules: off commitado deveria silenciar totalmente, violations=%v warnings=%v", violations, warnings)
		}
	})

	t.Run("warning_nao_commitado_ainda_dispara", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "main")
		// HEAD só tem mode: block — SEM rules:. Disco rebaixa mode E desliga a regra na MESMA
		// edição, nunca commitada. Isto é o ataque combinado que o ADR existe para fechar.
		commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
		writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: warning\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "credential_guard.mode: block") {
			t.Errorf("rules: warning NÃO commitado não deveria rebaixar a severidade — esperado violation (error), violations=%v warnings=%v", violations, warnings)
		}
	})

	t.Run("off_nao_commitado_ainda_dispara", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "main")
		commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
		writeFile(t, dir, "trackfw.yaml", "credential_guard:\n  mode: warn\nrules:\n  credential_guard_mode_downgrade: off\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "credential_guard.mode: block") {
			t.Errorf("rules: off NÃO commitado não deveria silenciar a regra — esperado violation (error), violations=%v warnings=%v", violations, warnings)
		}
	})
}

// TestRuleSeverity_ZeroDeltaParaRegrasNaoGuard prova que ruleSeverity() para qualquer regra fora
// de credentialGuardAnchoredRules continua resolvendo só pelo disco (diskRuleSeverity),
// independente de HEAD — o critério de aceite "zero delta para as outras ~38 regras" do
// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard.
func TestRuleSeverity_ZeroDeltaParaRegrasNaoGuard(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// HEAD não tem rules: nenhuma — se o HEAD estivesse (erroneamente) sendo consultado para
	// wip_limit, o valor "warning" do disco abaixo teria que ceder para o default "error" do
	// HEAD ausente, exatamente como credential_guard_mode_downgrade faria. Provamos que NÃO cede.
	commitTrackfwYAML(t, dir, "")
	writeFile(t, dir, "trackfw.yaml", "rules:\n  wip_limit: warning\n  adr_orphan: off\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if got := ruleSeverity("wip_limit"); got != "warning" {
		t.Errorf("wip_limit deveria resolver puramente pelo disco (warning), obteve: %q", got)
	}
	if got := ruleSeverity("adr_orphan"); got != "off" {
		t.Errorf("adr_orphan deveria resolver puramente pelo disco (off), obteve: %q", got)
	}
	// Regra sem entrada em rules: nem em ruleDefaults — default "error", igual a antes do ADR.
	if got := ruleSeverity("filename_uniqueness"); got != "error" {
		t.Errorf("filename_uniqueness deveria manter o default error, obteve: %q", got)
	}
}

// TestCredentialGuardRuleSeverity_SemHead_CaiNoDisco prova a posição assumida no ADR
// (Decision point 4): sem HEAD utilizável (aqui, repositório sem nenhum commit), a resolução das
// regras de credential-guard cai no disco puro — mesmo comportamento de antes do ADR.
func TestCredentialGuardRuleSeverity_SemHead_CaiNoDisco(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir) // nem sequer é git worktree
	t.Cleanup(config.Reset)

	writeFile(t, dir, "trackfw.yaml", "rules:\n  credential_guard_mode_downgrade: warning\n")

	if got := ruleSeverity("credential_guard_mode_downgrade"); got != "warning" {
		t.Errorf("sem HEAD, deveria cair no disco puro (warning), obteve: %q", got)
	}
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
