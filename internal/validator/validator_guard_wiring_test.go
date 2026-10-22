package validator

// validator_guard_wiring_test.go — ML-1A/ML-1C (REQ-2026-09-02, ADR-2026-10-10)
//
// Regra Dura de Reconciliação (CLAUDE.md, ROADMAP ML-1A AC8): cada teste novo declara, em
// comentário, a conclusão do ML que afirma — em uma frase — e a prova de mordida (como reprova
// sem a correção).
//
// Convenção de helpers: usa initOriginMainWithArtifacts (validator_scope_anchor_test.go) para
// criar origin com trackfw.yaml + arquivos adicionais. Para o caso A4 (sem trackfw.yaml na ref)
// há um helper local guardWiringInitOriginNoYAML.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// --------------------------------------------------------------------------
// Fixtures — strings de JSON de hook para os testes
// --------------------------------------------------------------------------

// claudeSettingsWithGuard retorna um .claude/settings.json com as duas entradas de guard
// (credential + git-branch) no evento PreToolUse, usando a forma de comando especificada.
// matcher é o valor do campo "matcher" (ex.: "Bash|PowerShell", "Bash").
// credentialCmd e gitBranchCmd são as formas de comando desejadas.
func claudeSettingsWithGuard(matcher, credentialCmd, gitBranchCmd string) string {
	return fmt.Sprintf(`{
  "hooks": {
    "PreToolUse": [
      {"matcher": %q, "hooks": [{"command": %q, "type": "command"}]},
      {"matcher": %q, "hooks": [{"command": %q, "type": "command"}]}
    ]
  }
}`, matcher, credentialCmd, matcher, gitBranchCmd)
}

// claudeSettingsWithSingleGuard retorna um .claude/settings.json com apenas a entrada de
// credential guard.
func claudeSettingsWithSingleGuard(matcher, credentialCmd string) string {
	return fmt.Sprintf(`{
  "hooks": {
    "PreToolUse": [
      {"matcher": %q, "hooks": [{"command": %q, "type": "command"}]}
    ]
  }
}`, matcher, credentialCmd)
}

// claudeSettingsEmpty retorna um .claude/settings.json sem entradas de guard.
func claudeSettingsEmpty() string {
	return `{"hooks": {"PreToolUse": []}}`
}

// claudeSettingsWithDisableAll retorna um .claude/settings.json com disableAllHooks:true.
func claudeSettingsWithDisableAll(matcher, credentialCmd string) string {
	return fmt.Sprintf(`{
  "disableAllHooks": true,
  "hooks": {
    "PreToolUse": [
      {"matcher": %q, "hooks": [{"command": %q, "type": "command"}]}
    ]
  }
}`, matcher, credentialCmd)
}

// kiroHooksWithGuard retorna um .kiro/hooks/trackfw-attention.json com a entrada de guard,
// no formato real emitido por InjectKiroHooks (version, name, trigger, matcher, action).
// Se withEnabledFalse for true, adiciona "enabled": false na entrada de guard.
func kiroHooksWithGuard(withEnabledFalse bool) string {
	credCmd := guardExpectedLine("credential", guardShellFamilyCmdExe)
	enabledField := ""
	if withEnabledFalse {
		enabledField = `,"enabled": false`
	}
	return fmt.Sprintf(`{
  "version": "v1",
  "hooks": [
    {
      "name": "trackfw-credential-guard-pre",
      "description": "Blocks/warns on possible plaintext credential materialization before a shell command executes",
      "trigger": "PreToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": %q}%s
    }
  ]
}`, credCmd, enabledField)
}

// guardWiringInitOriginNoYAML creates an origin with no trackfw.yaml but with the specified
// artifacts committed. Used by the A4 test to verify the rule derives the ref independently.
func guardWiringInitOriginNoYAML(t *testing.T, testDir string, artifacts map[string]string) {
	t.Helper()
	originDir := t.TempDir()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("guardWiringInitOriginNoYAML git %v in %s: %s", args, dir, string(out))
		}
	}
	runIn(originDir, "init", "-b", "main")
	runIn(originDir, "config", "user.email", "test@test.com")
	runIn(originDir, "config", "user.name", "test")
	for relPath, content := range artifacts {
		writeFile(t, originDir, relPath, content)
	}
	runIn(originDir, "add", ".")
	runIn(originDir, "commit", "-m", "init")

	runIn(testDir, "remote", "add", "origin", originDir)
	runIn(testDir, "fetch", "--depth=1", "--no-tags", "origin",
		"+refs/heads/main:refs/remotes/origin/main")
}

// guardWiringInitOriginAmbiguous creates an origin with two non-standard branches (develop +
// staging), making deriveOriginDefaultBranch return ("", false). Used by the fail-closed test.
func guardWiringInitOriginAmbiguous(t *testing.T, testDir string) {
	t.Helper()
	originDir := t.TempDir()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("guardWiringInitOriginAmbiguous git %v in %s: %s", args, dir, string(out))
		}
	}
	runIn(originDir, "init", "-b", "develop")
	runIn(originDir, "config", "user.email", "test@test.com")
	runIn(originDir, "config", "user.name", "test")
	writeFile(t, originDir, "README.md", "hello\n")
	runIn(originDir, "add", ".")
	runIn(originDir, "commit", "-m", "init")
	runIn(originDir, "checkout", "-b", "staging")

	runIn(testDir, "remote", "add", "origin", originDir)
	runIn(testDir, "fetch", "--no-tags", "origin",
		"+refs/heads/develop:refs/remotes/origin/develop",
		"+refs/heads/staging:refs/remotes/origin/staging",
	)
}

// --------------------------------------------------------------------------
// AC: viola quando o arquivo de hook está ausente no disco (Q6 ML-2B)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ArquivoAusenteNoDisco_Dispara afirma: quando o arquivo de hook está
// ausente no disco e estava presente em origin/main, a regra emite violação nomeando o guard.
// (Q6 ML-2B: renomeado de CategoriaApagada_Dispara — o fixture testa arquivo ausente no disco,
// não só a chave PreToolUse removida; os dois casos são distintos.)
// Prova de mordida: sem a implementação de validateGuardWiringRemoved, origin tem tupla mas a regra
// não a compara → ausência na lista de violations → teste falha em hasViolation.
func TestGuardWiringRemoved_ArquivoAusenteNoDisco_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardD11LegacyLine("credential", guardShellFamilyPSPosix)
	gitCmd := guardD11LegacyLine("git-branch", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithGuard("Bash|PowerShell", credCmd, gitCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: hook file absent (equivalent to key deleted).
	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed violation; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"credential"`) {
		t.Errorf("expected credential guard named in violation; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"git-branch"`) {
		t.Errorf("expected git-branch guard named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// AC: viola quando o arquivo está presente mas hooks.PreToolUse foi removido (Q6 ML-2B)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ArquivoPresenteChaveRemovidaDoDisco_Dispara afirma: quando o arquivo de
// hook está presente no disco mas o evento PreToolUse foi removido (arquivo tem {"hooks": {}}),
// a regra emite violação — porque a tupla estava em origin/main e não está no disco.
// (Q6 ML-2B: caso distinto de ArquivoAusenteNoDisco — aqui o arquivo existe, só a chave foi removida.)
// Prova de mordida: se o disco tiver arquivo presente e parsável, e a regra pular a comparação de
// tuplas (ex.: retornar nil quando diskAbsent=false), o teste falha em hasViolation.
func TestGuardWiringRemoved_ArquivoPresenteChaveRemovidaDoDisco_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	gitCmd := guardExpectedLine("git-branch", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithGuard("Bash|PowerShell", credCmd, gitCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: file present but hooks.PreToolUse key removed entirely.
	writeFile(t, dir, ".claude/settings.json", `{"hooks": {}}`)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed violation for present-file/removed-key; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"credential"`) {
		t.Errorf("expected credential guard named in violation; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"git-branch"`) {
		t.Errorf("expected git-branch guard named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// AC: viola quando um matcher específico é apagado
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_MatcherApagado_Dispara afirma: origin tem entry com matcher="PowerShell"
// e disk não tem esse matcher → violação nomeando PowerShell.
// Prova de mordida: sem matcher coverage, regra não detecta a entrada ausente → sem violação →
// teste falha.
func TestGuardWiringRemoved_MatcherApagado_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)

	// Origin: two separate entries for Bash and PowerShell.
	originHook := fmt.Sprintf(`{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"command": %q, "type": "command"}]},
      {"matcher": "PowerShell", "hooks": [{"command": %q, "type": "command"}]}
    ]
  }
}`, credCmd, credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: only Bash entry.
	diskHook := claudeSettingsWithSingleGuard("Bash", credCmd)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "PowerShell") {
		t.Fatalf("expected PowerShell matcher violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// AC: viola quando o matcher é estreitado (A2 negativo)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_MatcherEstritado_Dispara afirma: origin tem matcher="Bash|PowerShell" e
// disk tem matcher="Bash" — disk não cobre origin porque "PowerShell" não está no conjunto de disk.
// Prova de mordida: sem a verificação de cobertura de matcher, estreitamento passaria em silêncio.
func TestGuardWiringRemoved_MatcherEstritado_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: same command but narrowed matcher.
	diskHook := claudeSettingsWithSingleGuard("Bash", credCmd)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	// "Bash" does not cover "Bash|PowerShell" → violation.
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for narrowed matcher; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// AC: viola quando o comando é neutralizado por "true"
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ComandoNeutralizadoTrue_Dispara afirma: trocar o comando do guard por
// "true" (form fora da classe de equivalência) gera violação — mesmo com o matcher intacto.
// Prova de mordida: sem verificação de classe de equivalência, "true" passaria como guard válido.
func TestGuardWiringRemoved_ComandoNeutralizadoTrue_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: command replaced by "true".
	diskHook := claudeSettingsWithSingleGuard("Bash|PowerShell", "true")
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for neutralised command; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"credential"`) {
		t.Errorf("expected credential guard named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// AC: viola quando o comando é neutralizado por "echo trackfw guard credential"
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ComandoNeutralizadoEcho_Dispara afirma: trocar o comando do guard por
// "echo trackfw guard credential; exit $LASTEXITCODE" (contém o literal mas não é a forma canônica)
// gera violação — a comparação usa a classe de equivalência, não substring.
// Prova de mordida: comparação por substring aceitaria o echo como guard válido → sem violação →
// teste falha.
func TestGuardWiringRemoved_ComandoNeutralizadoEcho_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: command contains the literal but is NOT in the equivalence class.
	echoCmd := "echo trackfw guard credential; exit $LASTEXITCODE"
	diskHook := claudeSettingsWithSingleGuard("Bash|PowerShell", echoCmd)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for echo-neutralised command; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// AC: viola quando disableAllHooks:true é adicionado ao disco
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_DisableAllHooks_Dispara afirma: adicionar disableAllHooks:true ao
// .claude/settings.json no disco, sem essa chave em origin/main, gera violação D7.
// Prova de mordida: sem a verificação D7, a chave passaria em silêncio mesmo desligando o guard.
func TestGuardWiringRemoved_DisableAllHooks_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: same hook entries but with disableAllHooks:true added.
	diskHook := claudeSettingsWithDisableAll("Bash|PowerShell", credCmd)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "disableAllHooks") {
		t.Fatalf("expected disableAllHooks violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// AC: viola quando settings.local.json tem disableAllHooks:true
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_SettingsLocalDisableAll_Dispara afirma: .claude/settings.local.json com
// disableAllHooks:true gera violação mesmo sem âncora no git (arquivo não rastreado).
// Prova de mordida: sem a verificação disk-only, o arquivo local ignorado pelo git passaria
// despercebido.
func TestGuardWiringRemoved_SettingsLocalDisableAll_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin has no .claude/settings.local.json (it's git-ignored).
	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	// Write .claude/settings.local.json with disableAllHooks:true to disk.
	writeFile(t, dir, ".claude/settings.local.json", `{"disableAllHooks": true}`)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "settings.local.json") {
		t.Fatalf("expected settings.local.json violation; msgs=%v", msgs)
	}
	if !hasViolation(msgs, "disableAllHooks") {
		t.Errorf("expected disableAllHooks named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// Controle: arquivo ausente em origin/main → silêncio (D4)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ArquivoAusenteNaRef_Silencio afirma: quando o arquivo de hook está
// ausente em origin/main (nunca instalado), a regra não acusa — mesmo que o disco também não
// tenha o arquivo.
// Prova de mordida: uma implementação que sempre acusa "arquivo ausente" quebraria todo projeto
// novo → teste falha em asserting no violation.
func TestGuardWiringRemoved_ArquivoAusenteNaRef_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin has no hook files at all.
	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	// Disk also has no hook file.
	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if len(msgs) > 0 {
		t.Fatalf("expected no violations for never-installed hook; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// Controle: arquivo idêntico → silêncio
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ArquivoIdentico_Silencio afirma: quando disk e origin/main têm as
// mesmas tuplas de guard (mas formatação de JSON diferente), a regra não acusa — prova que a
// comparação é por tupla, não por string.
// Prova de mordida: uma implementação que compara strings brutas acusaria a diferença de
// indentação como adulteração → violação em arquivo semanticamente idêntico → teste falha.
func TestGuardWiringRemoved_ArquivoIdentico_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	gitCmd := guardExpectedLine("git-branch", guardShellFamilyPSPosix)
	// origin uses claudeSettingsWithGuard (two-space indent, specific order)
	originContent := claudeSettingsWithGuard("Bash|PowerShell", credCmd, gitCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originContent,
	})

	// Disk: same tuples but different JSON formatting (compact, keys reordered).
	// A raw-string comparator would fire here; tuple comparison must not.
	diskContent := fmt.Sprintf(`{"hooks":{"PreToolUse":[{"hooks":[{"command":%q,"type":"command"}],"matcher":"Bash|PowerShell"},{"hooks":[{"command":%q,"type":"command"}],"matcher":"Bash|PowerShell"}]}}`,
		credCmd, gitCmd)
	writeFile(t, dir, ".claude/settings.json", diskContent)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if len(msgs) > 0 {
		t.Fatalf("expected no violations for reformatted hook (same tuples); msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// Controle A2: migração legítima → silêncio
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_MigracaoLegitima_Silencio afirma: origin tem matcher="Bash" + forma
// D2-legacy; disk tem matcher="Bash|PowerShell" + forma D11-revised — a regra é silenciosa porque
// disco ⊇ origin em matcher e ambas as formas estão na mesma classe de equivalência (A2).
// Prova de mordida: sem a classe de equivalência e cobertura de matcher, trackfw update geraria
// falsos positivos em todo repositório que migrou.
func TestGuardWiringRemoved_MigracaoLegitima_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin: Bash matcher + D2-legacy form.
	d2cmd := guardD2LegacyLine("credential", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithSingleGuard("Bash", d2cmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: Bash|PowerShell matcher + D11-revised form (what trackfw update produces).
	d11cmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	diskHook := claudeSettingsWithSingleGuard("Bash|PowerShell", d11cmd)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if len(msgs) > 0 {
		t.Fatalf("expected no violations for legitimate migration; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// Controle A4: sem trackfw.yaml na ref, mas com arquivo de hook → regra ainda compara
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_SemTrackfwYamlNaRef_AindaCompara afirma: quando origin/main não tem
// trackfw.yaml (currentOriginMain.ref seria vazio em originAnchorOK), a regra ainda encontra o
// ref via deriveOriginDefaultBranch() e compara os arquivos de hook — violação é reportada ao
// deletar o hook do disco.
// Prova de mordida: usar currentOriginMain.ref diretamente (violação de A4) silenciaria a regra
// em qualquer repositório sem trackfw.yaml comprometido → teste falharia em hasViolation.
func TestGuardWiringRemoved_SemTrackfwYamlNaRef_AindaCompara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	hookContent := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	// Origin: NO trackfw.yaml, but has .claude/settings.json with guard wiring.
	guardWiringInitOriginNoYAML(t, dir, map[string]string{
		".claude/settings.json": hookContent,
	})

	// Disk: hook file absent (wiring removed).
	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("A4: expected violation even without trackfw.yaml in origin; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// Falha fechada: origin presente mas ref ilegível
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_RefIlegivel_FalhaFechada afirma: quando origin tem duas branches
// não-standard (develop + staging), deriveOriginDefaultBranch() retorna ("", false) e a regra
// emite uma mensagem de falha fechada — nunca silêncio.
// Prova de mordida: retornar nil em vez de emitir falha fechada permitiria desligar a detecção
// removendo origin/HEAD → violação de security → teste falha em !hasViolation(msgs, "fail").
func TestGuardWiringRemoved_RefIlegivel_FalhaFechada(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin: two non-standard branches → deriveOriginDefaultBranch returns ("", false).
	guardWiringInitOriginAmbiguous(t, dir)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected fail-closed violation; msgs=%v", msgs)
	}
	if !hasViolation(msgs, "could not be determined") {
		t.Errorf("expected fail-closed message naming the cause; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// Violação sobrevive a lenient mode e baseline (D5)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ViolaçaoSobreviveAoLenient afirma: guard_wiring_removed está em
// lenientCarveoutRules e credentialGuardAnchoredRules, portanto a violação sobrevive a lenient mode
// (permanece em violations, não em warnings) e sobrevive ao baseline (não é tolerada via SaveBaseline).
// Prova de mordida: remover guard_wiring_removed de lenientCarveoutRules faz o Part 1 falhar
// (violação migra para warnings); remover de credentialGuardAnchoredRules faz o Part 2 falhar
// (violação é tolerada pelo baseline). config.Reset() é obrigatório porque config.Load() é singleton.
func TestGuardWiringRemoved_ViolaçaoSobreviveAoLenient(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Set a future lenient_until so IsLenient() returns true.
	futureDate := time.Now().AddDate(0, 0, 60).Format("2006-01-02")
	yamlContent := fmt.Sprintf(
		"req_dir: docs/req\ngovernance_mode: lenient\nlenient_until: %s\n", futureDate,
	)

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	hookContent := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	initOriginMainWithArtifacts(t, dir, yamlContent, map[string]string{
		".claude/settings.json": hookContent,
	})
	// Write trackfw.yaml to disk so config.Load() picks up lenient mode.
	writeFile(t, dir, "trackfw.yaml", yamlContent)
	// Disk: hook file removed (violation).
	// No .claude/settings.json written to disk.

	// Create required dirs for Validate() to not error on other rules.
	for _, d := range []string{"docs/req", "docs/adr", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}

	chdir(t, dir)
	// config.Load() is a singleton — reset after the test so other tests are not affected.
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	// Part 1: violation survives lenient mode via Validate().
	// If guard_wiring_removed were not in lenientCarveoutRules, IsLenient()==true would
	// demote the violation to a warning and it would not appear in violations.
	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !hasViolation(violations, "guard_wiring_removed") {
		t.Errorf("guard_wiring_removed must survive lenient mode; violations=%v warnings=%v",
			violations, warnings)
	}

	// Part 2: violation survives baseline (credentialGuardAnchoredRules carve-out).
	// If guard_wiring_removed were not in credentialGuardAnchoredRules, SaveBaseline would
	// record it as tolerable and it would vanish from violations after baseline is applied.
	rawViolations, rawWarnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	if !hasViolation(rawViolations, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed violation before baseline; violations=%v", rawViolations)
	}
	if err := SaveBaseline(rawViolations, rawWarnings); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}
	violationsAfterBaseline, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate after baseline: %v", err)
	}
	if !hasViolation(violationsAfterBaseline, "guard_wiring_removed") {
		t.Errorf("guard_wiring_removed must not be toleratable via baseline; violations=%v",
			violationsAfterBaseline)
	}
}

// --------------------------------------------------------------------------
// AC: Kiro (hooks-as-array) com enabled:false dispara
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Kiro_EntryEnabled_False_Dispara afirma: em .kiro/hooks/trackfw-attention.json,
// uma entrada de guard com "enabled": false (D7 para Kiro) gera violação quando origin/main
// não tinha a chave enabled:false.
// Prova de mordida: sem o parser de hooks-as-array, Kiro não seria verificado → sem violação →
// teste falha.
func TestGuardWiringRemoved_Kiro_EntryEnabled_False_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin: Kiro hook with guard, no "enabled": false field.
	originHook := kiroHooksWithGuard(false) // false = don't add enabled:false
	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".kiro/hooks/trackfw-attention.json": originHook,
	})

	// Disk: same guard but with "enabled": false added (D7 disable).
	diskHook := kiroHooksWithGuard(true) // true = add enabled:false
	writeFile(t, dir, ".kiro/hooks/trackfw-attention.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for Kiro enabled:false; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"enabled": false`) {
		t.Errorf("expected enabled:false named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// Teste de unidade: guardWiringMatcherCovers
// --------------------------------------------------------------------------

// TestGuardWiringMatcherCovers_Tabela verifica a lógica de cobertura de matcher (A2) por tabela.
// Reconciliação: este teste prova que a regra de cobertura (disco ⊇ origin para alternâncias
// literais; igualdade exata para qualquer outro caso) está implementada corretamente.
func TestGuardWiringMatcherCovers_Tabela(t *testing.T) {
	cases := []struct {
		disk, origin string
		want         bool
	}{
		// Exact equality.
		{"Bash", "Bash", true},
		{"Bash|PowerShell", "Bash|PowerShell", true},
		{"", "", true},
		// Superset.
		{"Bash|PowerShell", "Bash", true},
		{"Bash|PowerShell|zsh", "Bash|PowerShell", true},
		// Not a superset.
		{"Bash", "Bash|PowerShell", false},
		{"Bash", "PowerShell", false},
		// Empty vs non-empty.
		{"", "Bash", false},
		{"Bash", "", false},
		// Non-literal (regex metacharacters) → not covered.
		{".*", "Bash", false},
		{"Bash", ".*", false},
		{".*", ".*", true}, // exact equality still covers
	}
	for _, tc := range cases {
		got := guardWiringMatcherCovers(tc.disk, tc.origin)
		if got != tc.want {
			t.Errorf("guardWiringMatcherCovers(%q, %q) = %v, want %v",
				tc.disk, tc.origin, got, tc.want)
		}
	}
}

// --------------------------------------------------------------------------
// Teste de unidade: guardWiringTypeFromCmd
// --------------------------------------------------------------------------

// TestGuardWiringTypeFromCmd_ClasseDeEquivalencia verifica que todas as formas da classe de
// equivalência são reconhecidas e que strings fora dela retornam "".
// Reconciliação: este teste prova que "echo trackfw guard credential" não é reconhecido como
// fiação (AD2, A2), e que formas legadas são equivalentes à D11-revised. Inclui as três formas
// legacyWABash* (wrapper /bin/bash -c, REQ-2026-10-20) extraídas de guardWiringScriptPathForms —
// family-independent, reconhecidas sob qualquer família de shell.
func TestGuardWiringTypeFromCmd_ClasseDeEquivalencia(t *testing.T) {
	for _, fam := range []guardShellFamily{guardShellFamilyPSPosix, guardShellFamilyCmdExe} {
		for _, subcmd := range []string{"credential", "git-branch"} {
			for _, form := range []string{
				guardExpectedLine(subcmd, fam),
				guardD11LegacyLine(subcmd, fam),
				guardD2LegacyLine(subcmd, fam),
			} {
				got := guardWiringTypeFromCmd(form, fam)
				if got != subcmd {
					t.Errorf("fam=%v subcmd=%q form=%q → got %q, want %q",
						fam, subcmd, form, got, subcmd)
				}
			}
		}
		// Out-of-class forms must return "".
		for _, notGuard := range []string{
			"true",
			"echo trackfw guard credential; exit $LASTEXITCODE",
			"trackfw validate",
			"",
		} {
			got := guardWiringTypeFromCmd(notGuard, fam)
			if got != "" {
				t.Errorf("fam=%v: out-of-class %q returned %q, want \"\"", fam, notGuard, got)
			}
		}
	}

	// legacyWABash* forms: /bin/bash -c wrapper (REQ-2026-10-20). Family-independent:
	// extracted directly from guardWiringScriptPathForms to avoid copying strings by hand.
	// Prova de mordida: remover as três entradas WABash de guardWiringScriptPathForms faz
	// (a) o count abaixo falhar (got 0, want 3) E (b) guardWiringTypeFromCmd retornar ""
	// para elas em qualquer iteração restante → falha dupla em t.Errorf.
	var waBashEntries []struct{ cmd, guardType string }
	for _, entry := range guardWiringScriptPathForms {
		if strings.HasPrefix(entry.cmd, "/bin/bash -c") {
			waBashEntries = append(waBashEntries, entry)
		}
	}
	// REQ-2026-10-20 introduziu exatamente 3 formas WABash (git-branch, credential, credential --global).
	// Se esse count mudar, o comentário "legacyWABash*" nos agentfiles.go deve ser atualizado também.
	const wantWABashCount = 3
	if len(waBashEntries) != wantWABashCount {
		t.Errorf("guardWiringScriptPathForms: expected %d legacyWABash entries (/bin/bash -c), got %d — "+
			"REQ-2026-10-20 introduziu 3 (git-branch, credential, credential --global); "+
			"adicionar ou remover entradas requer atualizar este teste",
			wantWABashCount, len(waBashEntries))
	}
	for _, fam := range []guardShellFamily{guardShellFamilyPSPosix, guardShellFamilyCmdExe} {
		for _, entry := range waBashEntries {
			got := guardWiringTypeFromCmd(entry.cmd, fam)
			if got != entry.guardType {
				t.Errorf("legacyWABash fam=%v cmd=%q → got %q, want %q",
					fam, entry.cmd, got, entry.guardType)
			}
		}
	}
}

// --------------------------------------------------------------------------
// Teste de unidade: guardWiringParseTomlFeaturesHooks
// --------------------------------------------------------------------------

// TestGuardWiringParseTomlFeaturesHooks verifica o parser TOML mínimo para [features] hooks.
// Reconciliação: este teste prova que o parser usa a tri-state corretamente — somente `true`
// conta como tomlHooksTrue; qualquer outra menção de hooks em contexto [features] é
// tomlHooksDisabledOrUnknown (falha fechada); ausência é tomlHooksAbsent.
func TestGuardWiringParseTomlFeaturesHooks(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    tomlFeaturesHooksState
	}{
		// Disabled forms → tomlHooksDisabledOrUnknown
		{"section form false", "[features]\nhooks = false\n", tomlHooksDisabledOrUnknown},
		{"section no spaces", "[features]\nhooks=false\n", tomlHooksDisabledOrUnknown},
		{"dotted key false", "features.hooks = false\n", tomlHooksDisabledOrUnknown},
		{"inline table false", "features = { hooks = false }\n", tomlHooksDisabledOrUnknown},
		{"inline table false no spaces", "features={hooks=false}\n", tomlHooksDisabledOrUnknown},
		{"section with comment on value", "[features]\nhooks = false # disable for debug\n", tomlHooksDisabledOrUnknown},
		{"quoted value false", "[features]\nhooks = \"false\"\n", tomlHooksDisabledOrUnknown},
		// Enabled forms → tomlHooksTrue
		{"hooks true", "[features]\nhooks = true\n", tomlHooksTrue},
		{"dotted key true", "features.hooks = true\n", tomlHooksTrue},
		{"inline table true", "features = { hooks = true }\n", tomlHooksTrue},
		// Absent forms → tomlHooksAbsent
		{"different section", "[other]\nhooks = false\n", tomlHooksAbsent},
		{"empty", "", tomlHooksAbsent},
		{"no hooks key", "[features]\nother = false\n", tomlHooksAbsent},
		// Comment before section header
		{"comment before section", "# comment\n[features]\nhooks = false\n", tomlHooksDisabledOrUnknown},
		// Section ends before key
		{"section ends before key", "[features]\n[other]\nhooks = false\n", tomlHooksAbsent},
		// Normalized section header with spaces
		{"section with inner spaces", "[ features ]\nhooks = false\n", tomlHooksDisabledOrUnknown},
	}
	for _, tc := range cases {
		got := guardWiringParseTomlFeaturesHooks(tc.content)
		if got != tc.want {
			t.Errorf("[%s]: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// --------------------------------------------------------------------------
// ML-1C: teste do texto do ref na mensagem (sem origin/origin/)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_RefTexto_SemDuplicacao afirma: a mensagem de violação contém "origin/main"
// (não "origin/origin/main") — prova que o formato string não prefixou "origin/" em cima de um
// ref que já é "origin/main".
// Prova de mordida: restaurar "origin/%s" nas strings de formato faz a mensagem conter
// "origin/origin/main" → teste falha em hasOriginOrigin.
func TestGuardWiringRemoved_RefTexto_SemDuplicacao(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	hookContent := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": hookContent,
	})
	// Disk: hook file absent (violation expected).

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected violation; msgs=%v", msgs)
	}
	// The violation message must contain "origin/main" exactly.
	for _, m := range msgs {
		if strings.Contains(m, "guard_wiring_removed") {
			if strings.Contains(m, "origin/origin/") {
				t.Errorf("message contains duplicated 'origin/origin/' prefix: %q", m)
			}
			if !strings.Contains(m, "origin/main") {
				t.Errorf("message does not reference 'origin/main': %q", m)
			}
		}
	}
}

// --------------------------------------------------------------------------
// ML-1C: D7 Gemini — hooksConfig.enabled: false
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Gemini_HooksConfigDisabled_Dispara afirma: adicionar
// hooksConfig: {"enabled": false} ao .gemini/settings.json no disco, sem essa chave em
// origin/main, gera violação D7 para a Gemini CLI.
// Prova de mordida: sem a verificação de disableKeyKindHooksEnabled para .gemini/settings.json,
// a chave é ignorada → sem violação → teste falha em hasViolation.
func TestGuardWiringRemoved_Gemini_HooksConfigDisabled_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	// Origin: Gemini hook with guard, no hooksConfig key.
	originHook := fmt.Sprintf(`{
  "hooks": {
    "BeforeTool": [
      {"matcher": "run_shell_command", "hooks": [{"type": "command", "command": %q}]}
    ]
  }
}`, credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".gemini/settings.json": originHook,
	})

	// Disk: same hook entries but with hooksConfig.enabled: false added.
	diskHook := fmt.Sprintf(`{
  "hooksConfig": {"enabled": false},
  "hooks": {
    "BeforeTool": [
      {"matcher": "run_shell_command", "hooks": [{"type": "command", "command": %q}]}
    ]
  }
}`, credCmd)
	writeFile(t, dir, ".gemini/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for Gemini hooksConfig.enabled:false; msgs=%v", msgs)
	}
	if !hasViolation(msgs, "hooksConfig.enabled: false") {
		t.Errorf("expected hooksConfig.enabled named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1C: D7 Copilot — disableAllHooks:true em .github/hooks/trackfw-attention.json
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Copilot_DisableAllHooks_Dispara afirma: adicionar
// disableAllHooks: true ao .github/hooks/trackfw-attention.json no disco, sem essa chave em
// origin/main, gera violação D7 para o GitHub Copilot CLI.
// Prova de mordida: sem a verificação de disableAllHooks para .github/hooks/trackfw-attention.json,
// a chave é ignorada → sem violação → teste falha em hasViolation.
func TestGuardWiringRemoved_Copilot_DisableAllHooks_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	// Copilot format: {"version":1,"hooks":{"preToolUse":[{"type":"command","matcher":"bash","command":"..."}]}}
	originHook := fmt.Sprintf(`{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {"type": "command", "matcher": "bash", "command": %q, "cwd": ".", "timeoutSec": 10}
    ]
  }
}`, credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".github/hooks/trackfw-attention.json": originHook,
	})

	// Disk: same entries but with disableAllHooks:true added.
	diskHook := fmt.Sprintf(`{
  "disableAllHooks": true,
  "version": 1,
  "hooks": {
    "preToolUse": [
      {"type": "command", "matcher": "bash", "command": %q, "cwd": ".", "timeoutSec": 10}
    ]
  }
}`, credCmd)
	writeFile(t, dir, ".github/hooks/trackfw-attention.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for Copilot disableAllHooks; msgs=%v", msgs)
	}
	if !hasViolation(msgs, "disableAllHooks") {
		t.Errorf("expected disableAllHooks named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1C: echo sem sufixo — variante do ComandoNeutralizadoEcho
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ComandoNeutralizadoEchoSimples_Dispara afirma: trocar o comando do guard
// por "echo trackfw guard credential" (sem nenhum sufixo) gera violação — a comparação usa a
// classe de equivalência, não substring.
// Prova de mordida: comparação por substring aceitaria este echo como guard válido → sem violação →
// teste falha.
func TestGuardWiringRemoved_ComandoNeutralizadoEchoSimples_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: command is "echo trackfw guard credential" (no suffix — bare echo).
	echoSimples := "echo trackfw guard credential"
	diskHook := claudeSettingsWithSingleGuard("Bash|PowerShell", echoSimples)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for bare echo command; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1C: Codex config.toml — forma section
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Codex_TomlSection_Dispara afirma: .codex/config.toml rastreado com
// [features]\nhooks = false gera violação quando origin/main não tem essa chave.
// Prova de mordida: sem guardWiringIsFileTracked + guardWiringCheckCodexToml, o arquivo é
// ignorado → sem violação → teste falha.
func TestGuardWiringRemoved_Codex_TomlSection_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin: no .codex/config.toml (never had hooks disabled).
	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	// Disk: create config.toml with hooks disabled, then stage it.
	writeFile(t, dir, ".codex/config.toml", "[features]\nhooks = false\n")
	runGitInDir(t, dir, "add", ".codex/config.toml")

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for Codex [features] hooks=false; msgs=%v", msgs)
	}
	if !hasViolation(msgs, ".codex/config.toml") {
		t.Errorf("expected .codex/config.toml named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1C: Codex config.toml — forma inline table (fix do Achado 2)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Codex_TomlInlineTable_Dispara afirma: .codex/config.toml rastreado com
// features = { hooks = false } (inline table) gera violação — o parser tri-state reconhece a
// forma inline e a trata como disabled (falha fechada).
// Prova de mordida: o parser antigo (falha aberta) ignorava inline tables → sem violação →
// teste falha. O fix do Achado 2 fecha este buraco.
func TestGuardWiringRemoved_Codex_TomlInlineTable_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	writeFile(t, dir, ".codex/config.toml", "features = { hooks = false }\n")
	runGitInDir(t, dir, "add", ".codex/config.toml")

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for Codex inline-table hooks=false; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1C: Codex config.toml — forma dotted key
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Codex_TomlDottedKey_Dispara afirma: .codex/config.toml rastreado com
// features.hooks = false (dotted key) gera violação.
// Prova de mordida: sem o ramo dotted-key no parser, esta forma passaria em silêncio.
func TestGuardWiringRemoved_Codex_TomlDottedKey_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	writeFile(t, dir, ".codex/config.toml", "features.hooks = false\n")
	runGitInDir(t, dir, "add", ".codex/config.toml")

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for Codex dotted-key hooks=false; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1C: Codex config.toml — comentário inline
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Codex_TomlCommentInline_Dispara afirma: .codex/config.toml rastreado com
// [features]\nhooks = false # debug gera violação — o parser strips o comentário antes de
// classificar o valor.
// Prova de mordida: sem strip de comentário, "false # debug" != "false" → tomlHooksAbsent →
// sem violação → teste falha.
func TestGuardWiringRemoved_Codex_TomlCommentInline_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	writeFile(t, dir, ".codex/config.toml", "[features]\nhooks = false # disable for local debug\n")
	runGitInDir(t, dir, "add", ".codex/config.toml")

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed for Codex hooks=false #comment; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-1C: Codex config.toml — não rastreado → silêncio (controle)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Codex_TomlNaoRastreado_Silencio afirma: .codex/config.toml com
// hooks = false NÃO rastreado pelo git (residual R3) não gera violação.
// Prova de mordida: sem a verificação guardWiringIsFileTracked, o arquivo local seria inspecionado
// mesmo sem estar no git → falso positivo → teste falha. Inverso: remover o is-tracked check
// faria este teste falhar (violação em arquivo não rastreado).
func TestGuardWiringRemoved_Codex_TomlNaoRastreado_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	// Write config.toml but do NOT git add it (untracked).
	writeFile(t, dir, ".codex/config.toml", "[features]\nhooks = false\n")

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, ".codex/config.toml") {
			t.Errorf("expected no violation for untracked config.toml, got: %q", m)
		}
	}
}

// --------------------------------------------------------------------------
// ML-1C: Codex config.toml — hooks = true → silêncio (controle)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Codex_TomlHooksTrue_Silencio afirma: .codex/config.toml rastreado com
// [features]\nhooks = true não gera violação — hooks estão explicitamente habilitados.
// Prova de mordida: classificar `true` como tomlHooksDisabledOrUnknown faria este teste falhar.
// Inverso: mudar classifyBool para classificar "true" como disabled geraria falso positivo aqui.
func TestGuardWiringRemoved_Codex_TomlHooksTrue_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{})

	writeFile(t, dir, ".codex/config.toml", "[features]\nhooks = true\n")
	runGitInDir(t, dir, "add", ".codex/config.toml")

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, ".codex/config.toml") {
			t.Errorf("expected no violation for hooks=true, got: %q", m)
		}
	}
}

// --------------------------------------------------------------------------
// Q2 ML-2B: git ls-tree failure → fail-closed (not absent)
// --------------------------------------------------------------------------

// TestGuardWiringReadFromOrigin_LsTreeFalha_FailClosed afirma: quando git ls-tree falha (ref
// inexistente), guardWiringReadFromOrigin deve retornar isAbsent=false e err não-nil — falha
// fechada em vez de silêncio. (Q2 ML-2B)
// Prova de mordida: antes da correção, lsErr != nil retornava (nil, true, nil) — isAbsent=true —
// fazendo o chamador ignorar o arquivo. Com a correção, o chamador recebe err != nil e emite
// violação de falha fechada.
func TestGuardWiringReadFromOrigin_LsTreeFalha_FailClosed(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	writeFile(t, dir, "README.md", "hello\n")
	runGitInDir(t, dir, "add", ".")
	runGitInDir(t, dir, "commit", "-m", "init")

	chdir(t, dir)

	// Use a ref that does not exist locally → git ls-tree fails.
	_, isAbsent, err := guardWiringReadFromOrigin("refs/remotes/nonexistent/branch", ".claude/settings.json")
	if isAbsent {
		t.Fatal("Q2: ls-tree failure must not be treated as absent — want fail-closed (isAbsent=false)")
	}
	if err == nil {
		t.Fatal("Q2: expected non-nil error for ls-tree failure on nonexistent ref")
	}
}

// --------------------------------------------------------------------------
// Q3 ML-2B: mensagens D7/Codex usam o ref derivado (não "origin/main" fixo)
// --------------------------------------------------------------------------

// guardWiringInitOriginMasterWithArtifacts creates an origin on branch "master" and fetches it.
// Used by Q3 tests to verify that "origin/master" (not "origin/main") appears in violation messages.
func guardWiringInitOriginMasterWithArtifacts(t *testing.T, testDir string, artifacts map[string]string) {
	t.Helper()
	originDir := t.TempDir()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("guardWiringInitOriginMasterWithArtifacts git %v in %s: %s", args, dir, string(out))
		}
	}
	runIn(originDir, "init", "-b", "master")
	runIn(originDir, "config", "user.email", "test@test.com")
	runIn(originDir, "config", "user.name", "test")
	writeFile(t, originDir, "trackfw.yaml", "req_dir: docs/req\n")
	for relPath, content := range artifacts {
		writeFile(t, originDir, relPath, content)
	}
	runIn(originDir, "add", ".")
	runIn(originDir, "commit", "-m", "init")

	runIn(testDir, "remote", "add", "origin", originDir)
	runIn(testDir, "fetch", "--depth=1", "--no-tags", "origin",
		"+refs/heads/master:refs/remotes/origin/master")
}

// TestGuardWiringRemoved_DisableAllHooks_RefMaster_MensagemCorreta afirma: quando a branch padrão
// do origin é "master", a mensagem de violação D7 (disableAllHooks) cita "origin/master" — não
// "origin/main" fixo. (Q3 ML-2B)
// Prova de mordida: antes da correção, guardWiringDisableKeyMsg não recebia nem usava o ref →
// mensagem continha literalmente "origin/main" mesmo com origin/master → teste falha em Contains.
func TestGuardWiringRemoved_DisableAllHooks_RefMaster_MensagemCorreta(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin on "master" with NO settings.json → so disableAllHooks on disk is new.
	guardWiringInitOriginMasterWithArtifacts(t, dir, map[string]string{})

	// Disk: settings.json with disableAllHooks:true (and a guard entry so origin was "present").
	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	writeFile(t, dir, ".claude/settings.json", fmt.Sprintf(`{
  "disableAllHooks": true,
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash|PowerShell", "hooks": [{"command": %q, "type": "command"}]}
    ]
  }
}`, credCmd))

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	var d7Msgs []string
	for _, m := range msgs {
		if strings.Contains(m, "disableAllHooks") {
			d7Msgs = append(d7Msgs, m)
		}
	}
	if len(d7Msgs) == 0 {
		t.Fatalf("Q3: expected D7 disableAllHooks violation; msgs=%v", msgs)
	}
	for _, m := range d7Msgs {
		if !strings.Contains(m, "origin/master") {
			t.Errorf("Q3: D7 message must cite 'origin/master', got: %q", m)
		}
		if strings.Contains(m, "origin/main") {
			t.Errorf("Q3: D7 message must not cite 'origin/main' when ref is origin/master, got: %q", m)
		}
	}
}

// TestGuardWiringRemoved_Codex_TomlMaster_MensagemCorreta afirma: quando a branch padrão do origin
// é "master", a mensagem de violação Codex config.toml cita "origin/master" — não "origin/main" fixo.
// (Q3 ML-2B)
// Prova de mordida: antes da correção, guardWiringCheckCodexToml concatenava "origin/main" literal
// → mensagem errada com origin/master → teste falha em Contains.
func TestGuardWiringRemoved_Codex_TomlMaster_MensagemCorreta(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Origin on "master" with NO config.toml → so hooks=false on disk is new.
	guardWiringInitOriginMasterWithArtifacts(t, dir, map[string]string{})

	// Disk: config.toml with [features] hooks = false — tracked in git.
	writeFile(t, dir, ".codex/config.toml", "[features]\nhooks = false\n")
	runGitInDir(t, dir, "add", ".codex/config.toml")

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	var codexMsgs []string
	for _, m := range msgs {
		if strings.Contains(m, ".codex/config.toml") {
			codexMsgs = append(codexMsgs, m)
		}
	}
	if len(codexMsgs) == 0 {
		t.Fatalf("Q3: expected Codex config.toml violation; msgs=%v", msgs)
	}
	for _, m := range codexMsgs {
		if !strings.Contains(m, "origin/master") {
			t.Errorf("Q3: Codex message must cite 'origin/master', got: %q", m)
		}
		if strings.Contains(m, "origin/main") {
			t.Errorf("Q3: Codex message must not cite 'origin/main' when ref is origin/master, got: %q", m)
		}
	}
}

// --------------------------------------------------------------------------
// Q8 ML-2B: origin sem refs buscados → silêncio (R4a)
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_OriginSemRefsBuscados_Silencio afirma: quando a origem está configurada
// mas nenhum ref foi buscado (0 refs em refs/remotes/origin/), a regra fica em silêncio.
// Isso corresponde a um checkout CI sem fetch explícito (R4a), onde o anchor system já emite
// warning próprio. (Q8 ML-2B)
// Prova de mordida: com guardWiringHasAnyFetchedOriginRefs sempre retornando true, o caso de
// "sem refs" cai no ramo (b) de falha fechada e emite violação — o teste falha em hasViolation.
// O caso simétrico (guardWiringHasAnyFetchedOriginRefs sempre false) é provado pelo teste
// TestGuardWiringRemoved_RefIlegivel_FalhaFechada: medido com M5-always-false, esse teste falha
// porque o ramo ambíguo é incorretamente tratado como "sem refs" → silêncio em vez de falha.
func TestGuardWiringRemoved_OriginSemRefsBuscados_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	// Configure origin but do NOT run git fetch → refs/remotes/origin/ is empty.
	originDir := t.TempDir()
	// Initialize origin with at least one commit so the remote URL is valid.
	runGitInDir(t, originDir, "init", "-b", "main")
	runGitInDir(t, originDir, "config", "user.email", "test@test.com")
	runGitInDir(t, originDir, "config", "user.name", "test")
	writeFile(t, originDir, "README.md", "hello\n")
	runGitInDir(t, originDir, "add", ".")
	runGitInDir(t, originDir, "commit", "-m", "init")
	runGitInDir(t, dir, "remote", "add", "origin", originDir)
	// No fetch — refs/remotes/origin/ stays empty.

	// chdir must come before the precondition check: guardWiringHasAnyFetchedOriginRefs
	// reads refs/remotes/origin/ from the current working directory.
	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	// Precondition: guardWiringHasAnyFetchedOriginRefs must return false.
	if guardWiringHasAnyFetchedOriginRefs() {
		t.Fatal("Q8 precondition: expected no fetched origin refs; repo is in wrong state for this test")
	}

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, "guard_wiring_removed") {
			t.Errorf("Q8: expected silence for unfetched origin; got: %q", m)
		}
	}
}

// --------------------------------------------------------------------------
// Lacuna 1 (ML-2B): testes de comportamento para fiação legada por script-path na ref
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_ScriptPathNaRef_FiacaoRemovida_Dispara afirma: quando origin/main
// tem .claude/settings.json com guarda em formato script-path legado
// ($CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh e …/trackfw-git-branch-guard.sh),
// e o disco não tem essa fiação, a regra emite violação nomeando os dois guards.
// (Q1 ML-2B: falso negativo confirmado — antes de guardWiringScriptPathForms, comandos
// script-path em origin/main retornavam "" de guardWiringTypeFromCmd → tupla não reconhecida
// → nenhuma violação emitida para fiação removida.)
// Prova de mordida: removendo "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh" de
// guardWiringScriptPathForms, guardWiringTypeFromCmd("$CLAUDE_PROJECT_DIR/...") retorna "" →
// a tupla não é extraída de origin → não é comparada → violação não emitida → teste falha em
// hasViolation(msgs, "credential").
func TestGuardWiringRemoved_ScriptPathNaRef_FiacaoRemovida_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credScriptCmd := "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"
	gitScriptCmd := "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"
	originHook := claudeSettingsWithGuard("Bash|PowerShell", credScriptCmd, gitScriptCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: hook file absent (wiring removed after a migration that accidentally deleted the file).
	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected guard_wiring_removed violation for script-path origin; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"credential"`) {
		t.Errorf("expected credential guard named in violation; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"git-branch"`) {
		t.Errorf("expected git-branch guard named in violation; msgs=%v", msgs)
	}
}

// TestGuardWiringRemoved_ScriptPathNaRef_MigradoParaD11_Silencio afirma: quando origin/main
// tem .claude/settings.json com guarda em formato script-path legado, e o disco tem os mesmos
// guards na forma D11-revised (o que `trackfw update` produz) com matcher Bash|PowerShell, a
// regra fica em silêncio — script-path e D11-revised estão na mesma classe de equivalência (A2).
// (Q1 ML-2B: silêncio esperado para a migração legítima script-path → D11-revised.)
// Precondição: as formas script-path devem ser reconhecidas por guardWiringTypeFromCmd, senão
// origin não tem tuplas e o silêncio seria vacuous (falso verde).
// Prova de mordida: se "$CLAUDE_PROJECT_DIR/scripts/…" for removido de guardWiringScriptPathForms,
// a precondição falha com "not recognized" — evitando silêncio vacuous.
func TestGuardWiringRemoved_ScriptPathNaRef_MigradoParaD11_Silencio(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credScriptCmd := "$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"
	gitScriptCmd := "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"
	originHook := claudeSettingsWithGuard("Bash|PowerShell", credScriptCmd, gitScriptCmd)

	// Precondition: the script-path forms must be in the equivalence class so that
	// origin has non-empty tuples. Without this, the test passes vacuously when
	// guardWiringScriptPathForms regresses (empty origin → no comparison → silence).
	if got := guardWiringTypeFromCmd(credScriptCmd, guardShellFamilyPSPosix); got != "credential" {
		t.Fatalf("precondition: %q not recognized as credential guard (got %q)", credScriptCmd, got)
	}
	if got := guardWiringTypeFromCmd(gitScriptCmd, guardShellFamilyPSPosix); got != "git-branch" {
		t.Fatalf("precondition: %q not recognized as git-branch guard (got %q)", gitScriptCmd, got)
	}

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: D11-revised form for both guards with Bash|PowerShell matcher — what trackfw update produces.
	d11CredCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	d11GitCmd := guardExpectedLine("git-branch", guardShellFamilyPSPosix)
	diskHook := claudeSettingsWithGuard("Bash|PowerShell", d11CredCmd, d11GitCmd)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if len(msgs) > 0 {
		t.Fatalf("expected silence for script-path origin → D11-revised disk migration; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-2C: R-1 — guard em campo de metadata, não no campo executável
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_GuardEmCampoDeMetadata_Claude_Dispara afirma: quando o guard command
// está apenas em um campo de metadata (metadata_guards.command) e o campo executável real
// (hooks[].command) foi substituído por "echo bypassed", a extração estrutural (hookCmdSchemaNested)
// não encontra o guard → violação emitida.
// Antes do corretivo, guardWiringCollectCommandValues buscava recursivamente e encontrava o guard
// em metadata_guards.command → zero violações → falso negativo.
// Prova de mordida: reverter para guardWiringCollectCommandValues → o guard é encontrado em
// metadata → zero violações → teste falha em hasViolation.
func TestGuardWiringRemoved_GuardEmCampoDeMetadata_Claude_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	gitCmd := guardExpectedLine("git-branch", guardShellFamilyPSPosix)
	originHook := claudeSettingsWithGuard("Bash|PowerShell", credCmd, gitCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".claude/settings.json": originHook,
	})

	// Disk: guard commands moved to metadata_guards.command; hooks[].command neutralized to
	// "echo bypassed". Runtime executes hooks[].command, not metadata_guards.command.
	diskHook := fmt.Sprintf(`{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash|PowerShell",
        "metadata_guards": {"command": %q},
        "hooks": [{"command": "echo bypassed", "type": "command"}]
      },
      {
        "matcher": "Bash|PowerShell",
        "metadata_guards": {"command": %q},
        "hooks": [{"command": "echo bypassed", "type": "command"}]
      }
    ]
  }
}`, credCmd, gitCmd)
	writeFile(t, dir, ".claude/settings.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected violation when guard is only in metadata field; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"credential"`) {
		t.Errorf("expected credential guard named in violation; msgs=%v", msgs)
	}
}

// TestGuardWiringRemoved_GuardEmCampoDeMetadata_Cursor_Dispara afirma: quando o guard command
// do Cursor está apenas em metadata_guards.command e o campo executável entry["command"] foi
// substituído por "echo bypassed", a extração estrutural (hookCmdSchemaFlat) não encontra o
// guard → violação emitida.
// Cursor usa requiresCommandType=false: credential_guard_hook_resolvable não detecta este bypass.
// guard_wiring_removed é a única defesa após o corretivo R-1.
// Antes do corretivo, a busca recursiva encontrava o guard em metadata → zero violações.
// Prova de mordida: reverter para guardWiringCollectCommandValues → guard encontrado em metadata
// → zero violações → teste falha em hasViolation.
func TestGuardWiringRemoved_GuardEmCampoDeMetadata_Cursor_Dispara(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
	// Cursor canonical format: hooks.beforeShellExecution[{command: "..."}]
	originHook := fmt.Sprintf(`{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {"command": %q}
    ]
  }
}`, credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".cursor/hooks.json": originHook,
	})

	// Disk: guard moved to metadata_guards.command; entry["command"] neutralized.
	diskHook := fmt.Sprintf(`{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "metadata_guards": {"command": %q},
        "command": "echo bypassed"
      }
    ]
  }
}`, credCmd)
	writeFile(t, dir, ".cursor/hooks.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected violation for Cursor when guard is only in metadata field; msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"credential"`) {
		t.Errorf("expected credential guard named in violation; msgs=%v", msgs)
	}
}

// --------------------------------------------------------------------------
// ML-2C: R-2 — type diferente de "command" → entrada não é fiação viva
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_TypeDiferenteDeCommand_Dispara afirma: quando a entrada de hook tem o
// guard command presente mas com type != "command", o runtime não executa o hook — a entrada não
// conta como fiação viva → violação emitida.
//
// Testado para dois CLIs com diferentes schemas:
//   - Claude Code (hookCmdSchemaNested): type no item interno da array hooks[].
//   - Amazon Q (hookCmdSchemaNested, requiresCommandType=false): mesma estrutura, mas
//     credential_guard_hook_resolvable NÃO detecta a mudança de type (requiresCommandType=false).
//     guard_wiring_removed é a única defesa para Amazon Q após o corretivo.
//
// Nota: Cursor usa hookCmdSchemaFlat sem campo type em suas entradas — não aplicável.
//
// Prova de mordida: sem a verificação de type nos items internos, entrada com type:"script" seria
// contada como fiação viva → zero violações → testes falham em hasViolation.
func TestGuardWiringRemoved_TypeDiferenteDeCommand_Dispara(t *testing.T) {
	t.Run("Claude", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "main")

		credCmd := guardExpectedLine("credential", guardShellFamilyPSPosix)
		originHook := claudeSettingsWithSingleGuard("Bash|PowerShell", credCmd)

		initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
			".claude/settings.json": originHook,
		})

		// Disk: same command present but type changed to "script" — runtime won't execute.
		diskHook := fmt.Sprintf(`{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash|PowerShell", "hooks": [{"command": %q, "type": "script"}]}
    ]
  }
}`, credCmd)
		writeFile(t, dir, ".claude/settings.json", diskHook)

		chdir(t, dir)
		t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

		msgs, err := validateGuardWiringRemoved()
		if err != nil {
			t.Fatalf("validateGuardWiringRemoved: %v", err)
		}
		if !hasViolation(msgs, "guard_wiring_removed") {
			t.Fatalf("Claude: expected violation when type=script; msgs=%v", msgs)
		}
		if !hasViolation(msgs, `"credential"`) {
			t.Errorf("Claude: expected credential guard named; msgs=%v", msgs)
		}
	})

	t.Run("AmazonQ", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "main")

		// Amazon Q uses CmdExe family (no "; exit $LASTEXITCODE" suffix) and mergeClaudeHookArray
		// (nested schema). requiresCommandType=false: credential_guard_hook_resolvable won't catch this.
		credCmd := guardExpectedLine("credential", guardShellFamilyCmdExe)
		originHook := fmt.Sprintf(`{
  "hooks": {
    "preToolUse": [
      {"matcher": "execute_bash", "hooks": [{"command": %q, "type": "command"}]}
    ]
  }
}`, credCmd)

		initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
			".amazonq/cli-agents/q_cli_default.json": originHook,
		})

		// Disk: same command present but type changed to "script".
		diskHook := fmt.Sprintf(`{
  "hooks": {
    "preToolUse": [
      {"matcher": "execute_bash", "hooks": [{"command": %q, "type": "script"}]}
    ]
  }
}`, credCmd)
		writeFile(t, dir, ".amazonq/cli-agents/q_cli_default.json", diskHook)

		chdir(t, dir)
		t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

		msgs, err := validateGuardWiringRemoved()
		if err != nil {
			t.Fatalf("validateGuardWiringRemoved: %v", err)
		}
		if !hasViolation(msgs, "guard_wiring_removed") {
			t.Fatalf("AmazonQ: expected violation when type=script (requiresCommandType=false); msgs=%v", msgs)
		}
		if !hasViolation(msgs, `"credential"`) {
			t.Errorf("AmazonQ: expected credential guard named; msgs=%v", msgs)
		}
	})
}

// --------------------------------------------------------------------------
// ML-2C: Kiro D7 sem recursão — guard em metadata não dispara D7
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_Kiro_EnabledFalse_EmMetadata_NaoDisparaD7 afirma: quando uma entrada
// Kiro tem enabled:false mas o guard command está apenas em um campo de metadata e o campo
// executável (action.command) está neutralizado ("echo bypassed"), o check D7 NÃO deve disparar
// — porque a entrada não contém guard na fiação executável.
//
// A violação de tupla DEVE ocorrer: origin tem o guard em action.command, disco não tem →
// tupla ausente → violação por comparação de tuplas (não por D7).
//
// Antes do corretivo, guardWiringCollectCommandValues buscava recursivamente e encontrava o
// guard em metadata.command → D7 disparava erroneamente ("enabled:false" na violação).
//
// Prova de mordida: reverter para guardWiringCollectCommandValues na linha do D7 do Kiro → o
// guard é encontrado recursivamente na metadata → D7 dispara → a asserção "D7 não disparou"
// (ausência de "enabled" na violação) falha.
func TestGuardWiringRemoved_Kiro_EnabledFalse_EmMetadata_NaoDisparaD7(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")

	credCmd := guardExpectedLine("credential", guardShellFamilyCmdExe)
	// Origin: Kiro canonical format with guard in action.command.
	originHook := fmt.Sprintf(`{
  "version": "v1",
  "hooks": [
    {
      "name": "trackfw-credential-guard-pre",
      "trigger": "PreToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": %q}
    }
  ]
}`, credCmd)

	initOriginMainWithArtifacts(t, dir, "req_dir: docs/req\n", map[string]string{
		".kiro/hooks/trackfw-attention.json": originHook,
	})

	// Disk: guard moved to metadata.command; action.command neutralized; enabled:false.
	// The runtime executes action.command — metadata.command is never executed.
	diskHook := fmt.Sprintf(`{
  "version": "v1",
  "hooks": [
    {
      "name": "trackfw-credential-guard-pre",
      "trigger": "PreToolUse",
      "matcher": "shell",
      "metadata": {"command": %q},
      "action": {"type": "command", "command": "echo bypassed"},
      "enabled": false
    }
  ]
}`, credCmd)
	writeFile(t, dir, ".kiro/hooks/trackfw-attention.json", diskHook)

	chdir(t, dir)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	msgs, err := validateGuardWiringRemoved()
	if err != nil {
		t.Fatalf("validateGuardWiringRemoved: %v", err)
	}

	// Tuple comparison MUST fire: origin has the guard, disk doesn't have it in action.command.
	if !hasViolation(msgs, "guard_wiring_removed") {
		t.Fatalf("expected tuple violation (guard absent from executable path); msgs=%v", msgs)
	}
	if !hasViolation(msgs, `"credential"`) {
		t.Errorf("expected credential guard named in tuple violation; msgs=%v", msgs)
	}

	// D7 must NOT fire: the entry has enabled:false but action.command is not a guard command.
	// Before the fix, D7 fired because the recursive search found the guard in metadata.command.
	for _, msg := range msgs {
		if strings.Contains(msg, `"enabled": false`) {
			t.Errorf("D7 must not fire when guard is only in metadata (not in action.command); got: %q", msg)
		}
	}
}

// --------------------------------------------------------------------------
// ML-2C: Layout coverage — every credentialGuardHookFile has a schema
// --------------------------------------------------------------------------

// TestGuardWiringHookLayout_CobreTodasAsEntradas afirma: todo arquivo em credentialGuardHookFiles
// tem uma entrada correspondente em guardWiringHookLayout. Sem esta cobertura, guardWiringParseTuples
// retorna fail-closed para arquivos não mapeados, introduzindo falsos negativos (falha aberta) ou
// violações desnecessárias ao processar arquivos legítimos.
// Prova de mordida: remover qualquer entrada de guardWiringHookLayout → este teste falha para
// esse arquivo.
func TestGuardWiringHookLayout_CobreTodasAsEntradas(t *testing.T) {
	for _, hf := range credentialGuardHookFiles {
		if _, ok := guardWiringHookLayout[hf.path]; !ok {
			t.Errorf("credentialGuardHookFiles entry %q has no extraction schema in guardWiringHookLayout — add one", hf.path)
		}
	}
}

// --------------------------------------------------------------------------
// Helper: runGitInDir — executa um comando git em um diretório de teste.
// --------------------------------------------------------------------------

func runGitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %s", args, dir, string(out))
	}
}
