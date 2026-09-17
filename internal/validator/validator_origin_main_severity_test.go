package validator

// ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-
// pr-edita, ML-1A.
//
// Testes da ancoragem de severidade em origin/main (generalização do padrão
// credentialGuardAnchoredRules para TODAS as regras).
//
// Regra Dura de Reconciliação (CLAUDE.md): uma frase por teste, abaixo.

import (
	"os"
	"os/exec"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// initOriginMain creates a local bare "origin" repo with the given trackfw.yaml content as
// origin/main, then fetches that ref into testDir. After this call, git rev-parse --verify
// origin/main succeeds in testDir.
func initOriginMain(t *testing.T, testDir, trackfwContent string) {
	t.Helper()

	// Create the "remote" repo with an explicit branch name "main" so the refspec is stable.
	originDir := t.TempDir()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("initOriginMain git %v in %s: %s", args, dir, string(out))
		}
	}

	runIn(originDir, "init", "-b", "main")
	runIn(originDir, "config", "user.email", "test@test.com")
	runIn(originDir, "config", "user.name", "test")
	writeFile(t, originDir, "trackfw.yaml", trackfwContent)
	runIn(originDir, "add", "trackfw.yaml")
	runIn(originDir, "commit", "-m", "init")

	// Add as "origin" to the test repo and fetch origin/main.
	runIn(testDir, "remote", "add", "origin", originDir)
	runIn(testDir, "fetch", "--depth=1", "--no-tags", "origin",
		"+refs/heads/main:refs/remotes/origin/main")
}

// initOriginMainWithoutFile creates a local "origin" repo WITHOUT trackfw.yaml in origin/main.
// origin/main is reachable (ref exists), but the file is absent — Braço 2 of the three-branch
// discrimination (file absent → disk only, informational warning, not a failure).
func initOriginMainWithoutFile(t *testing.T, testDir string) {
	t.Helper()

	originDir := t.TempDir()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("initOriginMainWithoutFile git %v in %s: %s", args, dir, string(out))
		}
	}

	runIn(originDir, "init", "-b", "main")
	runIn(originDir, "config", "user.email", "test@test.com")
	runIn(originDir, "config", "user.name", "test")
	// Commit something that is NOT trackfw.yaml so the branch exists.
	writeFile(t, originDir, "README.md", "no trackfw.yaml here\n")
	runIn(originDir, "add", "README.md")
	runIn(originDir, "commit", "-m", "init without trackfw.yaml")

	runIn(testDir, "remote", "add", "origin", originDir)
	runIn(testDir, "fetch", "--depth=1", "--no-tags", "origin",
		"+refs/heads/main:refs/remotes/origin/main")
}

// ------ AC: downgrade bloqueado ------

// TestOriginMainAnchor_DowngradeBlockado prova que uma edição commitada de trackfw.yaml que define
// rules: {wip_limit: off} NÃO rebaixa a severidade quando origin/main declara wip_limit: warning —
// AC1 e AC8(b) do ML-1A: a ancoragem em origin/main impede que um PR downgrade a sua própria verificação.
// Reconciliação: este teste prova que o ML-1A fecha o canal "PR commita rules: {x: off}" para
// regras não pertencentes a credentialGuardAnchoredRules (wip_limit).
func TestOriginMainAnchor_DowngradeBlockado(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// origin/main has wip_limit: warning explicitly. Disk will say wip_limit: off —
	// the "committed downgrade" the ML exists to block.
	// (wip_limit is not in ruleDefaults, so its built-in default is "error"; we use the explicit
	// "warning" here to create a clear downgrade scenario: warning → off, blocked to "warning".)
	initOriginMain(t, dir, "rules:\n  wip_limit: warning\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	// Disk: trackfw.yaml has rules: wip_limit: off — the PR's attempted downgrade.
	writeFile(t, dir, "trackfw.yaml", "rules:\n  wip_limit: off\n")

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}

	// wip_limit must resolve as "warning" (origin/main's "warning" wins over disk's "off").
	// To verify, we check that there is NO anchor-failure violation in the output,
	// then directly check ruleSeverity which reads currentOriginMain (set by ValidateUnfiltered).
	if hasViolation(violations, "severity anchor unavailable") {
		t.Errorf("unexpected anchor-failure violation — origin/main should be available: violations=%v", violations)
	}

	// The severity of wip_limit must be "warning" (origin/main wins), not "off" (disk).
	got := ruleSeverity("wip_limit")
	if got != "warning" {
		t.Errorf("wip_limit: want \"warning\" (origin/main wins over disk's \"off\"), got %q; violations=%v warnings=%v", got, violations, warnings)
	}
}

// ------ AC: upgrade respeitado (contra-braço de AC8(b)) ------

// TestOriginMainAnchor_UpgradeRespeitado prova que elevar a severidade no disco É respeitado —
// a regra é "a mais estrita vence", não "origin/main sempre vence".
// Reconciliação: este teste prova o contra-braço do AC8(b): origin/main tem "warning", disco tem
// "error" — o validador deve adotar "error" (disco wins porque é mais estrito).
func TestOriginMainAnchor_UpgradeRespeitado(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// origin/main has wip_limit: warning explicitly.
	initOriginMain(t, dir, "rules:\n  wip_limit: warning\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	// Disk: user UPGRADES wip_limit to error — this should be respected (stricter wins).
	writeFile(t, dir, "trackfw.yaml", "rules:\n  wip_limit: error\n")

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if hasViolation(violations, "severity anchor unavailable") {
		t.Errorf("unexpected anchor-failure violation: %v", violations)
	}

	// Disk's "error" is stricter than origin/main's "warning" — must win.
	got := ruleSeverity("wip_limit")
	if got != "error" {
		t.Errorf("wip_limit upgrade: want \"error\" (disk is stricter, must win), got %q", got)
	}
}

// ------ AC: origin/main ilegível → falha fechada ------

// TestOriginMainAnchor_RefUnreadable_FalhaFechada prova que quando origin/main não está disponível
// como ref local (Braço 1), ValidateUnfiltered emite uma violação com mensagem própria — distinta
// de "file absent" — e ruleSeverity retorna o default built-in (não o valor do disco).
// Reconciliação: este teste prova o AC "origin/main ilegível ⇒ falha fechada, mensagem distinta de
// 'arquivo ausente'" do ML-1A: o estado ref-absent produz violation; o estado file-absent não.
func TestOriginMainAnchor_RefUnreadable_FalhaFechada(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// Add a remote that exists but do NOT fetch — origin/main ref is absent.
	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://example.invalid/repo.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %s", out)
	}
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	// Disk: PR tries to downgrade by setting rules: wip_limit: off.
	writeFile(t, dir, "trackfw.yaml", "rules:\n  wip_limit: off\n")

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}

	// Must have the ref-unreadable violation (not the file-absent warning).
	if !hasViolation(violations, "severity anchor unavailable") {
		t.Errorf("expected anchor-failure violation, violations=%v", violations)
	}
	// The violation message must NOT look like "file absent" — two distinct observables.
	if hasViolation(violations, "not found") {
		t.Errorf("ref-unreadable violation must not say 'not found' (that is the file-absent message): %v", violations)
	}

	// In fail-closed state, wip_limit must resolve to built-in default ("warning"), NOT disk's "off".
	// This proves that rules: {wip_limit: off} is ignored when the anchor is broken.
	got := ruleSeverity("wip_limit")
	wantDefault := credentialGuardDefaultSeverity("wip_limit") // "warning" from ruleDefaults
	if got != wantDefault {
		t.Errorf("wip_limit in fail-closed state: want built-in default %q, got %q", wantDefault, got)
	}
}

// ------ AC: sem remote → sem nova violação (mantém make test e trackfw init vivos) ------

// TestOriginMainAnchor_SemOrigin_SemViolacao prova que um repositório git sem remote "origin"
// (estado 2: git, sem remote) não gera nova violação pelo âncora — o validador cai em disk-only
// silenciosamente, preservando o comportamento de `trackfw init` e de `make test`.
// Reconciliação: este teste prova que o estado "sem origin" é silencioso (disk only), confirmando
// que a correção não quebra projetos que nunca foram conectados a um remote.
func TestOriginMainAnchor_SemOrigin_SemViolacao(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// No remote added — isGitWorktree true, but "git remote get-url origin" fails.
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	writeFile(t, dir, "trackfw.yaml", "governance_mode: strict\n")

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if hasViolation(violations, "severity anchor") {
		t.Errorf("no-origin state must produce no anchor violation, violations=%v", violations)
	}

	// In no-origin state, ruleSeverity falls back to disk.
	currentOriginMain = loadOriginMainAnchor()
	if currentOriginMain.state != originAnchorNoRemote {
		t.Errorf("expected originAnchorNoRemote, got state=%d", currentOriginMain.state)
	}
}

// ------ AC: arquivo ausente → mensagem distinta de ref ilegível ------

// TestOriginMainAnchor_FileAbsent_MensagemDistinta prova que quando origin/main é acessível mas
// trackfw.yaml está ausente (Braço 2), o validador emite um WARNING — não uma violation — com
// mensagem distinta da mensagem de "ref ilegível".
// Reconciliação: este teste prova que os dois estados do âncora (ref ausente vs. arquivo ausente)
// produzem observáveis distintos, prevenindo o padrão "dois estados, um observável".
func TestOriginMainAnchor_FileAbsent_MensagemDistinta(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	initOriginMainWithoutFile(t, dir) // origin/main exists, but NO trackfw.yaml there
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	writeFile(t, dir, "trackfw.yaml", "governance_mode: strict\n")

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}

	// File-absent: must NOT produce the ref-unreadable violation.
	if hasViolation(violations, "severity anchor unavailable") {
		t.Errorf("file-absent state must not produce anchor-failure violation, violations=%v", violations)
	}

	// File-absent: must produce a WARNING with "not found" (distinct from ref-unreadable).
	if !hasWarning(warnings, "not found") {
		t.Errorf("file-absent state must produce informational warning with 'not found', warnings=%v", warnings)
	}

	// The ref-unreadable message must NOT appear in warnings.
	if hasWarning(warnings, "severity anchor unavailable") {
		t.Errorf("file-absent warning must not say 'severity anchor unavailable' (that is the ref-unreadable message): %v", warnings)
	}
}

// ------ Contra-braço: HEAD == disco (prova que a correção anterior era vácua em CI) ------

// TestOriginMainAnchor_HeadEqualsDisc_AnchorPrevails prova que a ancoragem em origin/main NÃO é
// vácua quando HEAD == disco (situação medida na Wave 0 como a falha do padrão anterior).
// origin/main tem wip_limit: warning; HEAD e disco têm wip_limit: off — a ancoragem em origin/main
// produz "warning" (não "off"), provando que origin/main != HEAD é o discriminante correto.
// Reconciliação: este teste prova a falsificação do AC8(b) — a versão SEM a correção (disk-only)
// deixaria wip_limit resolver como "off"; com a correção (origin/main anchor), resolve como "warning".
func TestOriginMainAnchor_HeadEqualsDisc_AnchorPrevails(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// origin/main has wip_limit: warning explicitly (NOT "off").
	initOriginMain(t, dir, "rules:\n  wip_limit: warning\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	// Disk AND HEAD both say wip_limit: off (simulating the CI scenario where HEAD == disk for
	// committed changes). commitTrackfwYAML writes the file then commits it — HEAD is now "off".
	commitTrackfwYAML(t, dir, "rules:\n  wip_limit: off\n")
	// disk is still rules: wip_limit: off (commitTrackfwYAML writes the file then commits).

	// Trigger anchor load via ValidateUnfiltered.
	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if hasViolation(violations, "severity anchor unavailable") {
		t.Errorf("unexpected anchor-failure, violations=%v", violations)
	}

	// With origin/main anchor ("warning"), disk's "off" must NOT win.
	// This proves origin/main is the correct discriminant, not HEAD (which equals disk here).
	got := ruleSeverity("wip_limit")
	if got == "off" {
		t.Errorf("wip_limit: got \"off\" (disk/HEAD value) — anchor failed to override; origin/main \"warning\" should win")
	}
	if got != "warning" {
		t.Errorf("wip_limit: want \"warning\" (origin/main wins over disk/HEAD \"off\"), got %q", got)
	}
}

// ------ Comportamento fora de Validate*: sane default ------

// TestOriginMainAnchor_ForaDeValidate_CaiNoDisco prova que ruleSeverity() chamado FORA de qualquer
// Validate* (currentOriginMain não configurado) cai em diskRuleSeverity — zero value = disk only.
// Reconciliação: este teste prova que o zero value de currentOriginMain é o "sane default" exigido
// para testes que chamam ruleSeverity() diretamente sem chamar Validate* antes.
func TestOriginMainAnchor_ForaDeValidate_CaiNoDisco(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	writeFile(t, dir, "trackfw.yaml", "rules:\n  wip_limit: warning\n  adr_orphan: off\n")

	// Do NOT call ValidateUnfiltered — currentOriginMain is zero value (originAnchorNotSet).
	// Must fall through to disk for all rules.
	if got := ruleSeverity("wip_limit"); got != "warning" {
		t.Errorf("wip_limit outside Validate*: want \"warning\" (disk), got %q", got)
	}
	if got := ruleSeverity("adr_orphan"); got != "off" {
		t.Errorf("adr_orphan outside Validate*: want \"off\" (disk), got %q", got)
	}
	if got := ruleSeverity("filename_uniqueness"); got != "error" {
		t.Errorf("filename_uniqueness outside Validate*: want \"error\" (built-in default), got %q", got)
	}
}

// ------ isGitWorktree: não-git-dir → sem violação ------

// TestOriginMainAnchor_NaoGit_SemViolacao prova que um diretório que não é git worktree (o estado
// que a maioria dos testes usa via t.TempDir()) não gera violação de âncora — disk only silencioso.
// Reconciliação: este teste prova que o estado "não-git" é silencioso, mantendo make test verde em
// diretórios temporários que não são repositórios git.
func TestOriginMainAnchor_NaoGit_SemViolacao(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir) // NOT a git worktree
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	writeFile(t, dir, "trackfw.yaml", "governance_mode: strict\n")

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if hasViolation(violations, "severity anchor") {
		t.Errorf("non-git dir must produce no anchor violation, violations=%v", violations)
	}
	if hasWarning(warnings, "severity anchor") {
		t.Errorf("non-git dir must produce no anchor warning, warnings=%v", warnings)
	}

	anchor := loadOriginMainAnchor()
	if anchor.state != originAnchorNoGit {
		t.Errorf("expected originAnchorNoGit for non-git dir, got state=%d", anchor.state)
	}
}

// ------ Limpeza do HOME para acesso a variáveis de ambiente em os.UserHomeDir ------
// (Não é um teste, mas uma nota: os testes acima não precisam de HOME especial porque
//  homeDir só é usado pelas regras de global guard, não pelo âncora de origin/main.)

// cleanupAnchor resets currentOriginMain to the zero value. Called via t.Cleanup in each test
// to prevent anchor state from leaking between tests (which share the package-level var in the
// same test binary run).
func cleanupAnchor(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })
}

// Ensure cleanupAnchor is referenced to avoid "declared and not used" error.
var _ = cleanupAnchor

// homeVarForTest is a reference to avoid lint warnings about unused imports.
var _ = os.Getenv
