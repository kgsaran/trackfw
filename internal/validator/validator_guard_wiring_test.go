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
// AC: viola quando a chave PreToolUse é apagada inteira
// --------------------------------------------------------------------------

// TestGuardWiringRemoved_CategoriaApagada_Dispara afirma: apagar o evento PreToolUse inteiro do
// .claude/settings.json, com a entrada tendo estado em origin/main, gera violação nomeando o guard.
// Prova de mordida: sem a implementação de validateGuardWiringRemoved, origin tem tupla mas a regra
// não a compara → ausência na lista de violations → teste falha em hasViolation.
func TestGuardWiringRemoved_CategoriaApagada_Dispara(t *testing.T) {
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
// fiação (AD2, A2), e que formas legadas são equivalentes à D11-revised.
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
