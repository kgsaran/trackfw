package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// initGitRepo inicializa um repo git no diretório e cria+faz checkout de uma branch.
func initGitRepo(t *testing.T, dir, branch string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	// commit vazio para criar HEAD
	run("commit", "--allow-empty", "-m", "init")
	if branch != "main" && branch != "master" {
		run("checkout", "-b", branch)
	}
}

// helper para criar diretórios de fixtures
func mkdirs(t *testing.T, base string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(base, d), 0755); err != nil {
			t.Fatalf("mkdirs: %v", err)
		}
	}
}

// helper para escrever arquivo de fixture
func writeFile(t *testing.T, base, rel, content string) {
	t.Helper()
	path := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("writeFile mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

// helper para verificar se alguma violation contém substring
func hasViolation(vs []string, substr string) bool {
	for _, v := range vs {
		if strings.Contains(v, substr) {
			return true
		}
	}
	return false
}

// hasWarning verifica se algum warning contém substring
func hasWarning(ws []string, substr string) bool {
	for _, w := range ws {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// chdir muda para dir e restaura ao fim do teste. Também reseta o singleton de config.Load(),
// já que ele é cacheado por processo e lê trackfw.yaml relativo ao CWD — sem o reset, um teste
// que roda após outro no mesmo pacote herdaria o ProjectConfig da fixture anterior (ML-3A:
// validateWIPLimit/IsLenient passaram a consumir config.Load() em vez de reler o arquivo).
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	config.Reset()
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		config.Reset()
	})
}

// TestValidate_Clean — estrutura vazia sem nenhuma violação nem warning
func TestValidate_Clean(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/req",
		"docs/adr",
	)
	chdir(t, dir)

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() retornou erro inesperado: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("esperado 0 violations, obteve %d: %v", len(violations), violations)
	}
	if len(warnings) != 0 {
		t.Errorf("esperado 0 warnings, obteve %d: %v", len(warnings), warnings)
	}
}

// TestValidate_WIPMissingREQ — roadmap em wip sem "REQ:" preenchido → 1 violation
// O arquivo DEVE incluir bloco de critérios para não gerar violação adicional.
func TestValidate_WIPMissingREQ(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/req", "docs/adr")
	chdir(t, dir)

	// Tem critérios de aceite mas NÃO tem REQ preenchido
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-sem-req.md", `# Roadmap: Sem REQ

## Acceptance Criteria
- [ ] build passa
`)

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if !hasViolation(violations, "no linked REQ") {
		t.Errorf("esperado violation 'no linked REQ', obteve: %v", violations)
	}
}

// TestValidate_WIPMissingAcceptanceCriteria — roadmap em wip com REQ mas sem critérios → 1 violation
func TestValidate_WIPMissingAcceptanceCriteria(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/req", "docs/adr")
	chdir(t, dir)

	// Tem REQ preenchido mas NÃO tem bloco de critérios
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-sem-criterios.md", `# Roadmap: Sem Criterios

REQ: REQ-001

## Wave 1
Sem criterios de aceite aqui.
`)

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if !hasViolation(violations, "no acceptance criteria") {
		t.Errorf("esperado violation 'no acceptance criteria', obteve: %v", violations)
	}
}

// TestValidate_MultipleWIP — 2 roadmaps em wip → 1 warning (independente das violations de REQ)
func TestValidate_MultipleWIP(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/req", "docs/adr")
	chdir(t, dir)

	// Ambos os arquivos têm REQ e critérios para isolar o warning de múltiplos WIPs
	for i, name := range []string{"ROADMAP-alpha.md", "ROADMAP-beta.md"} {
		_ = i
		writeFile(t, dir, "docs/roadmaps/wip/"+name, `# Roadmap

REQ: REQ-00X

## Acceptance Criteria
- [ ] build passa
`)
	}

	_, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if !hasWarning(warnings, "roadmaps in wip") {
		t.Errorf("esperado warning 'roadmaps in wip', obteve: %v", warnings)
	}
}

// TestValidate_REQMissingADR — req sem "ADR:" preenchido → violation
// O req DEVE ter Roadmap preenchido para não gerar violation adicional.
func TestValidate_REQMissingADR(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/req", "docs/adr")
	chdir(t, dir)

	// Tem Roadmap mas NÃO tem ADR
	writeFile(t, dir, "docs/req/REQ-sem-adr.md", `# REQ: Sem ADR

Roadmap: ROADMAP-001

## Descricao
Sem ADR referenciado.
`)

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if !hasViolation(violations, "no linked ADR") {
		t.Errorf("esperado violation 'no linked ADR', obteve: %v", violations)
	}
}

// TestValidate_BlockedMissingREQ — roadmap em blocked sem REQ → violation
func TestValidate_BlockedMissingREQ(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/req", "docs/adr")
	chdir(t, dir)

	writeFile(t, dir, "docs/roadmaps/blocked/ROADMAP-bloqueado.md", `# Roadmap: Bloqueado

## Motivo do bloqueio
Sem referencia a REQ.
`)

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if !hasViolation(violations, "no linked REQ") {
		t.Errorf("esperado violation 'no linked REQ' para blocked, obteve: %v", violations)
	}
}

// TestValidateREQsNotBlockedByDraftADRs_Violação — REQ Open com ADR Draft → violation
func TestValidateREQsNotBlockedByDraftADRs_Violação(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_ = os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "docs", "adr"), 0755)

	// Criar ADR Draft
	adrContent := "# ADR: Auth\n\n> Date: 2026-06-12 | Status: Draft\n\n## Context\n"
	_ = os.WriteFile(filepath.Join(dir, "docs", "adr", "ADR-2026-06-12-authentication-strategy.md"), []byte(adrContent), 0644)

	// Criar REQ Open com ADR Draft vinculado
	reqContent := "# REQ: Login\n\n> Date: 2026-06-12 | Status: Open | Blocked by ADRs: 1\n\n## Motivation\n\n## Acceptance Criteria\n\n## Linked ADR\nADR: \n\n## Blocked by ADRs\n<!-- ADRs in Draft status -->\n- ADR-2026-06-12-authentication-strategy.md (Draft)\n\n## Linked Roadmap\nRoadmap: \n"
	_ = os.WriteFile(filepath.Join(dir, "docs", "req", "REQ-2026-06-12-login.md"), []byte(reqContent), 0644)

	violations, err := validateREQsNotBlockedByDraftADRs()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) == 0 {
		t.Error("esperava violação para REQ com ADR Draft, não encontrou nenhuma")
	}
}

// TestValidateREQsNotBlockedByDraftADRs_SemViolação — REQ Open com ADR Accepted → sem violation
func TestValidateREQsNotBlockedByDraftADRs_SemViolação(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_ = os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "docs", "adr"), 0755)

	// Criar ADR Accepted
	adrContent := "# ADR: Auth\n\n> Date: 2026-06-12 | Status: Accepted\n\n## Context\n"
	_ = os.WriteFile(filepath.Join(dir, "docs", "adr", "ADR-2026-06-12-auth.md"), []byte(adrContent), 0644)

	// REQ com ADR Accepted listado na seção (não é Draft — não deve violar)
	reqContent := "# REQ: Login\n\n> Date: 2026-06-12 | Status: Open | Blocked by ADRs: 1\n\n## Blocked by ADRs\n- ADR-2026-06-12-auth.md (Accepted)\n\n## Linked Roadmap\nRoadmap: \n"
	_ = os.WriteFile(filepath.Join(dir, "docs", "req", "REQ-2026-06-12-login.md"), []byte(reqContent), 0644)

	violations, err := validateREQsNotBlockedByDraftADRs()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("não esperava violações, encontrou: %v", violations)
	}
}

// TestValidateREQsNotBlockedByDraftADRs_Retrocompatível — REQ antiga sem seção "Blocked by ADRs" → sem violation
func TestValidateREQsNotBlockedByDraftADRs_Retrocompatível(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_ = os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755)

	// REQ antiga sem seção "Blocked by ADRs"
	reqContent := "# REQ: Old Feature\n\n> Date: 2026-01-01 | Status: Open\n\n## Motivation\nOld req\n\n## Linked ADR\nADR: \n\n## Linked Roadmap\nRoadmap: \n"
	_ = os.WriteFile(filepath.Join(dir, "docs", "req", "REQ-2026-01-01-old.md"), []byte(reqContent), 0644)

	violations, err := validateREQsNotBlockedByDraftADRs()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("REQ antiga sem seção Blocked by ADRs não deve gerar violação: %v", violations)
	}
}

// TestGetStatus_REQsBloqueadas — REQ Open com ADR Draft aparece na seção ⏳
func TestGetStatus_REQsBloqueadas(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/adr",
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
	)
	chdir(t, dir)

	// ADR Draft
	adrContent := "# ADR: Auth\n\n> Date: 2026-06-12 | Status: Draft\n"
	writeFile(t, dir, "docs/adr/ADR-2026-06-12-auth.md", adrContent)

	// REQ bloqueada (Status: Open + seção ## Blocked by ADRs)
	reqContent := "# REQ: Login\n\n> Date: 2026-06-12 | Status: Open | Blocked by ADRs: 1\n\n## Blocked by ADRs\n- ADR-2026-06-12-auth.md (Draft)\n\n## Linked Roadmap\nRoadmap: \n"
	writeFile(t, dir, "docs/req/REQ-2026-06-12-login.md", reqContent)

	output, err := GetStatus()
	if err != nil {
		t.Fatalf("GetStatus erro: %v", err)
	}
	if !strings.Contains(output, "⏳ REQs blocked by not-accepted ADRs") {
		t.Error("output não contém seção de REQs bloqueadas")
	}
	if !strings.Contains(output, "ADR-2026-06-12-auth.md (Draft)") {
		t.Error("output não menciona o ADR bloqueante com o status Draft")
	}
}

// TestGetStatus_REQsBloqueadasPorADRProposed — REQ Open com ADR Proposed aparece na seção
// ⏳ com o status real "(Proposed)", não o literal "(Draft)". ML-1E (2026-08-01): o resumo
// hardcodava "(Draft)" para qualquer ADR não aceito — corrigido para exibir o status
// resolvido via adrStatusForRule.
func TestGetStatus_REQsBloqueadasPorADRProposed(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/adr",
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
	)
	chdir(t, dir)

	// ADR Proposed
	adrContent := "# ADR: Auth\n\n> Date: 2026-06-12 | Status: Proposed\n"
	writeFile(t, dir, "docs/adr/ADR-2026-06-12-auth.md", adrContent)

	// REQ bloqueada (Status: Open + seção ## Blocked by ADRs)
	reqContent := "# REQ: Login\n\n> Date: 2026-06-12 | Status: Open | Blocked by ADRs: 1\n\n## Blocked by ADRs\n- ADR-2026-06-12-auth.md (Proposed)\n\n## Linked Roadmap\nRoadmap: \n"
	writeFile(t, dir, "docs/req/REQ-2026-06-12-login.md", reqContent)

	output, err := GetStatus()
	if err != nil {
		t.Fatalf("GetStatus erro: %v", err)
	}
	if !strings.Contains(output, "⏳ REQs blocked by not-accepted ADRs") {
		t.Error("output não contém seção de REQs bloqueadas")
	}
	if !strings.Contains(output, "ADR-2026-06-12-auth.md (Proposed)") {
		t.Error("output deveria mostrar o status real (Proposed), não (Draft)")
	}
	if strings.Contains(output, "ADR-2026-06-12-auth.md (Draft)") {
		t.Error("output rotulou incorretamente um ADR Proposed como (Draft)")
	}
}

// TestGetStatus_SemREQsBloqueadas — sem REQs bloqueadas, seção ⏳ não aparece
func TestGetStatus_SemREQsBloqueadas(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
	)
	chdir(t, dir)

	output, err := GetStatus()
	if err != nil {
		t.Fatalf("GetStatus erro: %v", err)
	}
	if strings.Contains(output, "⏳ REQs blocked") {
		t.Error("seção de REQs bloqueadas não deve aparecer quando não há bloqueios")
	}
}

// TestGetStatus_InventoryAnalyzingCount — roadmap discriminante: um roadmap em
// docs/roadmaps/analyzing/ precisa aparecer na contagem "analyzing <n>" do bloco
// 📊 Inventory. Antes desta mudança (ML-1A), analyzing/ não era contado em lugar
// nenhum da saída de `status` — nem no bloco antigo (que só listava wip/blocked/done),
// nem em nenhuma outra seção. Este teste prova que o roadmap em analyzing/ agora é
// contado, o que falsifica esse defeito.
func TestGetStatus_InventoryAnalyzingCount(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/adr",
		"docs/roadmaps/backlog",
		"docs/roadmaps/analyzing",
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
	)
	chdir(t, dir)

	writeFile(t, dir, "docs/roadmaps/analyzing/ROADMAP-em-analise.md", "# Roadmap\n\n> Status: analyzing\n")

	output, err := GetStatus()
	if err != nil {
		t.Fatalf("GetStatus erro: %v", err)
	}
	if !strings.Contains(output, "📊 Inventory") {
		t.Fatal("output não contém o bloco 📊 Inventory")
	}
	if !strings.Contains(output, "analyzing 1") {
		t.Errorf("output deveria contar 1 roadmap em analyzing/, obtido:\n%s", output)
	}
}

// TestGetStatus_InventoryREQsDiscriminadas — uma REQ Open, uma Done e uma Closed
// devem aparecer discriminadas como "(1 Open · 1 Done · 1 Closed)" — não agrupadas.
func TestGetStatus_InventoryREQsDiscriminadas(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/req",
		"docs/adr",
		"docs/roadmaps/backlog",
		"docs/roadmaps/analyzing",
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
	)
	chdir(t, dir)

	writeFile(t, dir, "docs/req/REQ-open.md", "---\nstatus: Open\n---\n\n# REQ: Open\n")
	writeFile(t, dir, "docs/req/REQ-done.md", "---\nstatus: Done\n---\n\n# REQ: Done\n")
	writeFile(t, dir, "docs/req/REQ-closed.md", "---\nstatus: Closed\n---\n\n# REQ: Closed\n")

	output, err := GetStatus()
	if err != nil {
		t.Fatalf("GetStatus erro: %v", err)
	}
	if !strings.Contains(output, "(1 Open · 1 Done · 1 Closed)") {
		t.Errorf("output deveria discriminar REQs em (1 Open · 1 Done · 1 Closed), obtido:\n%s", output)
	}
	if !strings.Contains(output, "REQs        3") {
		t.Errorf("output deveria mostrar o total de 3 REQs, obtido:\n%s", output)
	}
}

// TestValidateWIPLimit_ByAgent — by_agent: 2 roadmaps em zeus/wip com limit 1 → 1 warning
func TestValidateWIPLimit_ByAgent(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/zeus/wip",
		"docs/roadmaps/zeus/backlog",
	)
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yaml := "roadmap_namespacing: by_agent\nagents:\n- zeus\nwip_limit: 1\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	writeFile(t, dir, "docs/roadmaps/zeus/wip/ROADMAP-alpha.md", "# Alpha\nREQ: REQ-001\n## Acceptance Criteria\n- [ ] ok\n")
	writeFile(t, dir, "docs/roadmaps/zeus/wip/ROADMAP-beta.md", "# Beta\nREQ: REQ-002\n## Acceptance Criteria\n- [ ] ok\n")

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}
	if !hasWarning(warnings, "zeus") {
		t.Errorf("esperado warning mencionando 'zeus', obteve: %v", warnings)
	}
	if !hasWarning(warnings, "limit: 1") {
		t.Errorf("esperado warning mencionando 'limit: 1', obteve: %v", warnings)
	}
}

// TestValidateWIPLimit_Global_OK — 1 WIP, limit=1 → sem warning
func TestValidateWIPLimit_Global_OK(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip")
	chdir(t, dir)

	writeFile(t, dir, "trackfw.yaml", "wip_limit: 1\nwip_by_squad: false\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-alpha.md", "# Roadmap: Alpha\n\nREQ: REQ-001\nsquad: platform\n\n## Acceptance Criteria\n- [ ] build\n")

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("esperado 0 warnings com 1 WIP e limit=1, obteve: %v", warnings)
	}
}

// TestValidateWIPLimit_Global_Exceed — 2 WIPs, limit=1 → 1 warning
func TestValidateWIPLimit_Global_Exceed(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip")
	chdir(t, dir)

	writeFile(t, dir, "trackfw.yaml", "wip_limit: 1\nwip_by_squad: false\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-alpha.md", "# Roadmap: Alpha\n\nREQ: REQ-001\nsquad: platform\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-beta.md", "# Roadmap: Beta\n\nREQ: REQ-002\nsquad: platform\n")

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}
	if len(warnings) != 1 {
		t.Errorf("esperado 1 warning com 2 WIPs e limit=1, obteve %d: %v", len(warnings), warnings)
	}
	if !hasWarning(warnings, "roadmaps in wip/") {
		t.Errorf("warning esperado conter 'roadmaps in wip/', obteve: %v", warnings)
	}
}

// TestValidateWIPLimit_Global_HighLimit — 2 WIPs, limit=3 → sem warning
func TestValidateWIPLimit_Global_HighLimit(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip")
	chdir(t, dir)

	writeFile(t, dir, "trackfw.yaml", "wip_limit: 3\nwip_by_squad: false\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-alpha.md", "# Roadmap: Alpha\n\nREQ: REQ-001\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-beta.md", "# Roadmap: Beta\n\nREQ: REQ-002\n")

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("esperado 0 warnings com 2 WIPs e limit=3, obteve: %v", warnings)
	}
}

// TestValidateWIPLimit_BySquad_OK — 2 WIPs de squads diferentes, limit=1 → sem warning
func TestValidateWIPLimit_BySquad_OK(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip")
	chdir(t, dir)

	writeFile(t, dir, "trackfw.yaml", "wip_limit: 1\nwip_by_squad: true\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-alpha.md", "# Roadmap: Alpha\n\nREQ: REQ-001\nsquad: platform\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-beta.md", "# Roadmap: Beta\n\nREQ: REQ-002\nsquad: backend\n")

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("esperado 0 warnings com 2 WIPs em squads distintos e limit=1, obteve: %v", warnings)
	}
}

// TestValidateWIPLimit_BySquad_Exceed — 2 WIPs do mesmo squad, limit=1 → 1 warning
func TestValidateWIPLimit_BySquad_Exceed(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip")
	chdir(t, dir)

	writeFile(t, dir, "trackfw.yaml", "wip_limit: 1\nwip_by_squad: true\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-alpha.md", "# Roadmap: Alpha\n\nREQ: REQ-001\nsquad: platform\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-beta.md", "# Roadmap: Beta\n\nREQ: REQ-002\nsquad: platform\n")

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}
	if len(warnings) != 1 {
		t.Errorf("esperado 1 warning com 2 WIPs do mesmo squad e limit=1, obteve %d: %v", len(warnings), warnings)
	}
	if !hasWarning(warnings, "platform") {
		t.Errorf("warning esperado mencionar squad 'platform', obteve: %v", warnings)
	}
}

// TestGetStatus_Empty — diretórios vazios → retorna string de status sem pânico
func TestGetStatus_Empty(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/blocked", "docs/roadmaps/done")
	chdir(t, dir)

	status, err := GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() retornou erro: %v", err)
	}
	if !strings.Contains(status, "trackfw status") {
		t.Errorf("status deveria conter 'trackfw status', obteve: %q", status)
	}
}

// TestResolveREQFilesByAgent — resolveREQFiles deve encontrar arquivos em req_dir/<agente>/<estado>/ quando by_agent.
func TestResolveREQFilesByAgent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/requisicoes/claude/wip/REQ-001.md", `---
req_id: RID-1
---
# REQ-001
`)
	cfg := config.ProjectConfig{
		REQDir:             filepath.Join(dir, "docs/requisicoes"),
		RoadmapNamespacing: config.NamespacingByAgent,
		Agents:             []string{"claude"},
	}

	files := resolveREQFiles(cfg)
	if len(files) != 1 {
		t.Fatalf("esperado 1 arquivo, obteve %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "REQ-001.md" {
		t.Errorf("esperado REQ-001.md, obteve %q", filepath.Base(files[0]))
	}
}

// TestTraceIdREQByAgent — par REQ+Roadmap com mesmo req_id em estrutura by_agent não deve gerar traceid_orphan_roadmap.
func TestTraceIdREQByAgent(t *testing.T) {
	dir := t.TempDir()
	// REQ em req_dir/claude/wip/
	writeFile(t, dir, "docs/requisicoes/claude/wip/REQ-001.md", `---
req_id: RID-1
status: wip
---
# REQ-001
`)
	// Roadmap em roadmap_dir/claude/wip/
	writeFile(t, dir, "docs/roadmaps/claude/wip/ROADMAP-001.md", `---
req_id: RID-1
status: wip
---
# Roadmap 001
`)
	cfg := config.ProjectConfig{
		REQDir:             filepath.Join(dir, "docs/requisicoes"),
		RoadmapDir:         filepath.Join(dir, "docs/roadmaps"),
		RoadmapNamespacing: config.NamespacingByAgent,
		Agents:             []string{"claude"},
		TraceIdField:       "req_id",
	}

	violations, _ := validateTraceId(cfg)
	for _, v := range violations {
		if strings.Contains(v, "traceid_orphan_roadmap") {
			t.Errorf("não esperava traceid_orphan_roadmap, mas obteve: %q", v)
		}
		if strings.Contains(v, "traceid_orphan_req") {
			t.Errorf("não esperava traceid_orphan_req, mas obteve: %q", v)
		}
	}
}

// TestSalvaguardaOneSided — apenas Roadmap com req_id, sem REQ, deve gerar warning com "REQs (0)".
func TestSalvaguardaOneSided(t *testing.T) {
	dir := t.TempDir()
	// Apenas roadmap, sem REQ nenhuma
	writeFile(t, dir, "docs/roadmaps/claude/wip/ROADMAP-001.md", `---
req_id: RID-1
status: wip
---
# Roadmap 001
`)
	cfg := config.ProjectConfig{
		REQDir:             filepath.Join(dir, "docs/requisicoes"),
		RoadmapDir:         filepath.Join(dir, "docs/roadmaps"),
		RoadmapNamespacing: config.NamespacingByAgent,
		Agents:             []string{"claude"},
		TraceIdField:       "req_id",
	}

	_, warnings := validateTraceId(cfg)
	if !hasWarning(warnings, "REQs (0)") {
		t.Errorf("esperado warning contendo 'REQs (0)', obteve: %v", warnings)
	}
}

// TestReqHasADRConfiguravel — req_has_adr pode ser rebaixada para warning ou desativada via rules.
func TestReqHasADRConfiguravel(t *testing.T) {
	// REQ sem ADR preenchido → a severidade deve ser controlada pela regra req_has_adr.
	buildDir := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		mkdirs(t, dir,
			"docs/roadmaps/wip",
			"docs/roadmaps/backlog",
			"docs/roadmaps/blocked",
			"docs/req",
			"docs/adr",
		)
		// REQ com Roadmap preenchido mas SEM ADR — dispara req_has_adr
		writeFile(t, dir, "docs/req/REQ-sem-adr.md", "# REQ: Sem ADR\n\nRoadmap: ROADMAP-001\n")
		return dir
	}

	t.Run("warning", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  req_has_adr: warning\n")
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "req_has_adr") || hasViolation(violations, "no linked ADR") {
			t.Errorf("com req_has_adr=warning não deve haver violations de ADR, obteve: %v", violations)
		}
		if !hasWarning(warnings, "req_has_adr") && !hasWarning(warnings, "no linked ADR") {
			t.Errorf("com req_has_adr=warning deve haver pelo menos 1 warning de ADR, obteve warnings=%v", warnings)
		}
	})

	t.Run("off", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  req_has_adr: off\n")
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "no linked ADR") {
			t.Errorf("com req_has_adr=off não deve haver violations de ADR, obteve: %v", violations)
		}
		if hasWarning(warnings, "no linked ADR") {
			t.Errorf("com req_has_adr=off não deve haver warnings de ADR, obteve: %v", warnings)
		}
	})

	t.Run("default_error", func(t *testing.T) {
		dir := buildDir(t)
		// sem trackfw.yaml → default "error"
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "no linked ADR") {
			t.Errorf("sem config (default error) deve gerar violation de ADR, obteve: %v", violations)
		}
	})
}

// TestBlockedHasREQConfiguravel — blocked_has_req pode ser rebaixada para warning ou desativada via rules.
func TestBlockedHasREQConfiguravel(t *testing.T) {
	buildDir := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		mkdirs(t, dir,
			"docs/roadmaps/wip",
			"docs/roadmaps/backlog",
			"docs/roadmaps/blocked",
			"docs/req",
			"docs/adr",
		)
		// Roadmap em blocked SEM REQ — dispara blocked_has_req
		writeFile(t, dir, "docs/roadmaps/blocked/ROADMAP-bloqueado.md", "# Roadmap: Bloqueado\n\n## Motivo\nSem REQ.\n")
		return dir
	}

	t.Run("warning", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  blocked_has_req: warning\n")
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "blocked_has_req") || hasViolation(violations, "no linked REQ") {
			t.Errorf("com blocked_has_req=warning não deve haver violations de REQ (blocked), obteve: %v", violations)
		}
		if !hasWarning(warnings, "blocked_has_req") && !hasWarning(warnings, "no linked REQ") {
			t.Errorf("com blocked_has_req=warning deve haver pelo menos 1 warning, obteve warnings=%v", warnings)
		}
	})

	t.Run("off", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  blocked_has_req: off\n")
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		// O roadmap bloqueado sem REQ não deve gerar nada quando regra está off
		if hasViolation(violations, "blocked_has_req") || hasViolation(violations, "no linked REQ") {
			t.Errorf("com blocked_has_req=off não deve haver violations, obteve: %v", violations)
		}
		if hasWarning(warnings, "blocked_has_req") || hasWarning(warnings, "no linked REQ") {
			t.Errorf("com blocked_has_req=off não deve haver warnings, obteve: %v", warnings)
		}
	})

	t.Run("default_error", func(t *testing.T) {
		dir := buildDir(t)
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "no linked REQ") {
			t.Errorf("sem config (default error) deve gerar violation de blocked REQ, obteve: %v", violations)
		}
	})
}

// TestReqHasRoadmapConfiguravel — req_has_roadmap pode ser rebaixada para warning ou desativada via rules.
func TestReqHasRoadmapConfiguravel(t *testing.T) {
	buildDir := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		mkdirs(t, dir,
			"docs/roadmaps/wip",
			"docs/roadmaps/backlog",
			"docs/roadmaps/blocked",
			"docs/req",
			"docs/adr",
		)
		// REQ com ADR preenchido mas SEM Roadmap — dispara req_has_roadmap
		writeFile(t, dir, "docs/req/REQ-sem-roadmap.md", "# REQ: Sem Roadmap\n\nADR: ADR-001\n")
		return dir
	}

	t.Run("warning", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  req_has_roadmap: warning\n")
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "req_has_roadmap") || hasViolation(violations, "no linked Roadmap") {
			t.Errorf("com req_has_roadmap=warning não deve haver violations, obteve: %v", violations)
		}
		if !hasWarning(warnings, "req_has_roadmap") && !hasWarning(warnings, "no linked Roadmap") {
			t.Errorf("com req_has_roadmap=warning deve haver pelo menos 1 warning, obteve warnings=%v", warnings)
		}
	})

	t.Run("off", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  req_has_roadmap: off\n")
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "no linked Roadmap") {
			t.Errorf("com req_has_roadmap=off não deve haver violations, obteve: %v", violations)
		}
		if hasWarning(warnings, "no linked Roadmap") {
			t.Errorf("com req_has_roadmap=off não deve haver warnings, obteve: %v", warnings)
		}
	})

	t.Run("default_error", func(t *testing.T) {
		dir := buildDir(t)
		config.Reset()
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "no linked Roadmap") {
			t.Errorf("sem config (default error) deve gerar violation de Roadmap ausente, obteve: %v", violations)
		}
	})
}

// TestValidateADRsAreReferencedByAgent — ADR referenciado em REQ by_agent não deve gerar violation.
func TestValidateADRsAreReferencedByAgent(t *testing.T) {
	dir := t.TempDir()

	// ADR em docs/adr/claude/done/
	writeFile(t, dir, "docs/adr/claude/done/ADR-001.md", `---
name: ADR-001
status: Accepted
---
# ADR-001: Decisão de Exemplo
`)
	// REQ em docs/req/claude/wip/ referenciando ADR-001
	writeFile(t, dir, "docs/req/claude/wip/REQ-001.md", `---
status: Open
---
# REQ-001

ADR: ADR-001.md
Roadmap: ROADMAP-001
`)

	// trackfw.yaml com by_agent
	writeFile(t, dir, "trackfw.yaml", `roadmap_namespacing: by_agent
agents:
  - claude
req_dir: docs/req
adr_dirs:
  - docs/adr
`)

	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateADRsAreReferenced()
	if err != nil {
		t.Fatalf("validateADRsAreReferenced() erro inesperado: %v", err)
	}
	if hasViolation(violations, "ADR-001") {
		t.Errorf("ADR-001 não deveria ser orphan — está referenciado na REQ by_agent; obteve: %v", violations)
	}
}

// TestValidateBranchHasWIPRoadmap_Violation — feat/ sem wip/ roadmap → violation
func TestValidateBranchHasWIPRoadmap_Violation(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "feat/my-feature")
	mkdirs(t, dir, "docs/roadmaps/wip") // wip/ existe mas vazio
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(violations, "no roadmap is in wip/ nor done/") {
		t.Errorf("esperava violation de wip/done vazios, obteve: %v", violations)
	}
}

// TestValidateBranchHasWIPRoadmap_Pass — feat/ com roadmap em wip/ → sem violation
func TestValidateBranchHasWIPRoadmap_Pass(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "feat/my-feature")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-my-feature.md", "REQ: REQ-001\n## Acceptance Criteria\n- [ ] ok\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("não esperava violations com roadmap em wip, obteve: %v", violations)
	}
}

func TestValidateBranchHasWIPRoadmap_MismatchedRoadmap(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "feat/my-feature")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-unrelated.md", "REQ: REQ-001\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasViolation(violations, "no matching roadmap") {
		t.Errorf("expected mismatch violation, got: %v", violations)
	}
}

// TestValidateBranchHasWIPRoadmap_DonePass — feat/ com roadmap em done/ com slug da branch → sem violation (P4: cenário 2)
func TestValidateBranchHasWIPRoadmap_DonePass(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "feat/my-feature")
	// wip/ existe mas vazio; roadmap movido para done/ com slug correspondente
	mkdirs(t, dir, "docs/roadmaps/wip")
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-my-feature.md", "REQ: REQ-001\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("roadmap em done/ com slug correspondente não deve gerar violation, obteve: %v", violations)
	}
}

// TestValidateBranchHasWIPRoadmap_DoneMismatch — feat/ com roadmap em done/ com slug DIFERENTE → violation (P4: cenário 4)
func TestValidateBranchHasWIPRoadmap_DoneMismatch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "feat/my-feature")
	// roadmap em done/ com slug que NÃO corresponde à branch
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-outra-coisa.md", "REQ: REQ-001\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasViolation(violations, "no matching roadmap in wip/ nor done/") {
		t.Errorf("roadmap em done/ com slug diferente deve reprovar, obteve: %v", violations)
	}
}

func TestValidateBranchHasWIPRoadmap_CIBranchEnvironment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-ci-feature.md", "REQ: REQ-001\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Setenv("TRACKFW_BRANCH", "feat/ci-feature")
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("CI branch environment should match roadmap, got: %v", violations)
	}
}

// TestValidateBranchHasWIPRoadmap_MainBranch — branch main → skip, sem violation
func TestValidateBranchHasWIPRoadmap_MainBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main") // permanece em main
	mkdirs(t, dir, "docs/roadmaps/wip")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("branch main não deve gerar violation, obteve: %v", violations)
	}
}

// TestValidateBranchHasWIPRoadmap_RuleOff — regra desativada via config → silencioso
func TestValidateBranchHasWIPRoadmap_RuleOff(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "fix/something")
	mkdirs(t, dir, "docs/roadmaps/wip")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\nrules:\n  branch_has_wip_roadmap: off\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// com regra "off" não deve aparecer nem como violation nem como warning
	if hasViolation(violations, "no roadmap is in wip/ nor done/") || hasWarning(warnings, "no roadmap is in wip/ nor done/") {
		t.Errorf("regra off deve suprimir a mensagem, obteve violations=%v warnings=%v", violations, warnings)
	}
}

// TestValidate_WithTildeInADRDirs — verifica que adr_dirs com ~/ encontra ADRs no diretório home.
func TestValidate_WithTildeInADRDirs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir() falhou: %v", err)
	}

	// Criar um diretório temporário dentro do home dir do usuário para simular ~/my-global-adrs
	relativeSubdir := filepath.Join(".trackfw-test-adrs-tmp", "global-adrs")
	globalADRDir := filepath.Join(home, relativeSubdir)
	if err := os.MkdirAll(globalADRDir, 0755); err != nil {
		t.Fatalf("mkdir globalADRDir: %v", err)
	}
	defer func() { _ = os.RemoveAll(filepath.Join(home, ".trackfw-test-adrs-tmp")) }()

	// Criar um ADR global no diretório de home
	adrContent := "---\nstatus: Accepted\ndate: 2026-07-20\n---\n# ADR 001 Global\n"
	if err := os.WriteFile(filepath.Join(globalADRDir, "ADR-001-global.md"), []byte(adrContent), 0644); err != nil {
		t.Fatalf("writeFile ADR-001-global: %v", err)
	}

	// Criar projeto local de teste
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/req",
		"docs/adr",
	)

	// trackfw.yaml configurando adr_dirs com ~/
	tildePath := "~/" + relativeSubdir
	yamlContent := "adr_dirs:\n  - " + tildePath + "\n  - docs/adr\n"
	writeFile(t, dir, "trackfw.yaml", yamlContent)

	// REQ referenciando o ADR global por caminho literal com ~/ e roadmap por caminho canônico.
	reqContent := "---\nstatus: Open\ndate: 2026-07-20\n---\n# REQ 001\nADR: " + tildePath + "/ADR-001-global.md\nRoadmap: docs/roadmaps/wip/ROADMAP-001.md\n"
	writeFile(t, dir, "docs/req/REQ-001.md", reqContent)

	// Roadmap linkando REQ
	rmContent := "---\nstatus: WIP\ndate: 2026-07-20\n---\n# Roadmap 001\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- AC1\n"
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-001.md", rmContent)

	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered erro inesperado: %v", err)
	}

	// Não deve haver violation de orphan para ADR-001-global.md
	if hasViolation(violations, "ADR-001-global.md") {
		t.Errorf("ADR em caminho com ~/ não deveria ser considerado órfão. Violations: %v", violations)
	}
	// Não deve haver warning de ref target inexistente para ADR-001-global.md
	if hasWarning(warnings, "ADR-001-global.md") {
		t.Errorf("ADR em caminho com ~/ deveria ser encontrado. Warnings: %v", warnings)
	}
}

// TestValidate_NonExistentADRDirs_WarningByDefault verifica que adr_dirs inexistente emite Warning por padrão (strict_ci_paths: false).
func TestValidate_NonExistentADRDirs_WarningByDefault(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/req",
		"docs/adr",
	)

	nonExistent := filepath.Join(t.TempDir(), "subfolder_that_does_not_exist")
	yamlContent := "strict_ci_paths: false\nadr_dirs:\n  - docs/adr\n  - " + nonExistent + "\n"
	writeFile(t, dir, "trackfw.yaml", yamlContent)

	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered erro inesperado: %v", err)
	}

	if hasViolation(violations, nonExistent) {
		t.Errorf("adr_dir inexistente não deveria emitir violation quando strict_ci_paths é false. Violations: %v", violations)
	}

	if !hasWarning(warnings, nonExistent) {
		t.Errorf("adr_dir inexistente deveria emitir warning quando strict_ci_paths é false. Warnings: %v", warnings)
	}
}

// TestValidate_NonExistentADRDirs_StrictCIPathsError verifica que adr_dirs inexistente emite Error quando strict_ci_paths: true.
func TestValidate_NonExistentADRDirs_StrictCIPathsError(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/req",
		"docs/adr",
	)

	nonExistent := filepath.Join(t.TempDir(), "subfolder_that_does_not_exist")
	yamlContent := "strict_ci_paths: true\nadr_dirs:\n  - docs/adr\n  - " + nonExistent + "\n"
	writeFile(t, dir, "trackfw.yaml", yamlContent)

	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered erro inesperado: %v", err)
	}

	if !hasViolation(violations, nonExistent) {
		t.Errorf("adr_dir inexistente deveria emitir violation quando strict_ci_paths é true. Violations: %v", violations)
	}

	if hasWarning(warnings, nonExistent) {
		t.Errorf("adr_dir inexistente não deveria emitir warning quando strict_ci_paths é true. Warnings: %v", warnings)
	}
}

// TestValidate_ExternalADROrphanExemption verifica que ADRs localizados fora do CWD são isentos da regra adr_orphan.
func TestValidate_ExternalADROrphanExemption(t *testing.T) {
	externalDir := t.TempDir()
	adrExternalContent := "---\nstatus: Accepted\ndate: 2026-07-20\n---\n# ADR 999 External\n"
	if err := os.WriteFile(filepath.Join(externalDir, "ADR-999-external.md"), []byte(adrExternalContent), 0644); err != nil {
		t.Fatalf("writeFile ADR-999-external: %v", err)
	}

	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/req",
		"docs/adr",
	)

	yamlContent := "adr_dirs:\n  - docs/adr\n  - " + externalDir + "\n"
	writeFile(t, dir, "trackfw.yaml", yamlContent)

	// ADR local não referenciado (deve gerar adr_orphan)
	adrLocalContent := "---\nstatus: Accepted\ndate: 2026-07-20\n---\n# ADR 001 Local\n"
	writeFile(t, dir, "docs/adr/ADR-001-local.md", adrLocalContent)

	// REQ e Roadmap validos linkando o ADR externo por caminho literal.
	reqContent := "---\nstatus: Open\ndate: 2026-07-20\nadr: " + filepath.Join(externalDir, "ADR-999-external.md") + "\nroadmap: docs/roadmaps/wip/ROADMAP-001.md\n---\n# REQ 001\nRoadmap: docs/roadmaps/wip/ROADMAP-001.md\nADR: " + filepath.Join(externalDir, "ADR-999-external.md") + "\n"
	writeFile(t, dir, "docs/req/REQ-001.md", reqContent)

	rmContent := "---\nstatus: WIP\ndate: 2026-07-20\n---\n# Roadmap 001\nREQ: REQ-001.md\n## Acceptance Criteria\n- AC1\n"
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-001.md", rmContent)

	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered erro inesperado: %v", err)
	}

	// ADR-001-local.md (dentro do CWD) DEVE ser reportado em warnings (pois adr_orphan default é warning)
	if !hasWarning(warnings, "ADR-001-local.md") && !hasViolation(violations, "ADR-001-local.md") {
		t.Errorf("ADR local sem referência deveria ser marcado como órfão. Violations: %v, Warnings: %v", violations, warnings)
	}

	// ADR-999-external.md (fora do CWD) NÃO DEVE ser reportado como órfão em warnings nem violations
	if hasWarning(warnings, "ADR-999-external.md") || hasViolation(violations, "ADR-999-external.md") {
		t.Errorf("ADR externo fora do CWD NÃO deveria ser marcado como órfão. Violations: %v, Warnings: %v", violations, warnings)
	}
}

// TestAnalyzingState_NoFolderStatusViolation — roadmap em analyzing/ com status: analyzing
// não deve gerar folder_status nem traceid_state_mismatch.
func TestAnalyzingState_NoFolderStatusViolation(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/analyzing",
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
	)
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	writeFile(t, dir, "docs/roadmaps/analyzing/ROADMAP-em-analise.md", `---
status: analyzing
date: 2026-07-26
---
# Roadmap: Em Análise

## Objetivo
Planejamento sem código ainda.
`)

	warnings, err := validateFolderStatusCoherence()
	if err != nil {
		t.Fatalf("validateFolderStatusCoherence() erro: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "ROADMAP-em-analise.md") {
			t.Errorf("roadmap em analyzing/ NÃO deve gerar folder_status warning, obteve: %q", w)
		}
	}
}

// TestAnalyzingState_WipLimitDoesNotCount — roadmap em analyzing/ NÃO deve ser contado pelo wip_limit.
func TestAnalyzingState_WipLimitDoesNotCount(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/analyzing", "docs/roadmaps/wip")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// limit=1, 1 roadmap em wip e 1 em analyzing → não deve exceder o limite
	writeFile(t, dir, "trackfw.yaml", "wip_limit: 1\nwip_by_squad: false\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-em-wip.md", "# Roadmap: Em WIP\n\nREQ: REQ-001\n")
	writeFile(t, dir, "docs/roadmaps/analyzing/ROADMAP-em-analise.md", `---
status: analyzing
---
# Roadmap: Em Análise
`)

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("wip_limit NÃO deve contar roadmaps em analyzing/ — esperado 0 warnings, obteve: %v", warnings)
	}
}

// ---------------------------------------------------------------------------
// Testes note_orphan
// ---------------------------------------------------------------------------

// TestNoteOrphan_SemVault — projeto sem vault/ não gera nenhum warning.
func TestNoteOrphan_SemVault(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	msgs, err := validateNoteOrphan()
	if err != nil {
		t.Fatalf("validateNoteOrphan() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("projeto sem vault/ não deve gerar msgs, obteve: %v", msgs)
	}
}

// TestNoteOrphan_NotaOrfaGeraWarning — nota não linkada no index gera warning por default.
func TestNoteOrphan_NotaOrfaGeraWarning(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "vault/notes")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Nota órfã — não referenciada no index
	writeFile(t, dir, "vault/notes/minha-nota-2026-01-01.md", "# Minha nota\n")
	writeFile(t, dir, "vault/notes/index.md", "# Vault\n\n## Índice\n\n")

	msgs, err := validateNoteOrphan()
	if err != nil {
		t.Fatalf("validateNoteOrphan() erro: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("esperado 1 msg para nota órfã, obteve 0")
	}
	// Validate() deve retornar exit 0 (warning por default, não violation)
	_, warnings, err2 := Validate()
	if err2 != nil {
		t.Fatalf("Validate() erro: %v", err2)
	}
	if !hasWarning(warnings, "minha-nota-2026-01-01.md") {
		t.Errorf("esperado warning contendo nota, obteve warnings: %v", warnings)
	}
}

// TestNoteOrphan_ElevadaAError — com rules: {note_orphan: error}, vira violation.
func TestNoteOrphan_ElevadaAError(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "vault/notes")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	writeFile(t, dir, "vault/notes/orfaa-2026-01-01.md", "# Órfã\n")
	writeFile(t, dir, "vault/notes/index.md", "# Vault\n\n## Índice\n\n")
	writeFile(t, dir, "trackfw.yaml", "rules:\n  note_orphan: error\n")

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if !hasViolation(violations, "orfaa-2026-01-01.md") {
		t.Errorf("esperado violation com note_orphan:error, obteve violations: %v", violations)
	}
}

// TestNoteOrphan_IndexNaoConta — index.md não é contada como nota órfã.
func TestNoteOrphan_IndexNaoConta(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "vault/notes")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	writeFile(t, dir, "vault/notes/index.md", "# Vault\n\n## Índice\n\n")

	msgs, err := validateNoteOrphan()
	if err != nil {
		t.Fatalf("validateNoteOrphan() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("index.md não deve ser contado como órfão, obteve: %v", msgs)
	}
}

// TestNoteOrphan_NotaLinkadaNaoGera — nota linkada no index não gera warning.
func TestNoteOrphan_NotaLinkadaNaoGera(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "vault/notes")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	writeFile(t, dir, "vault/notes/nota-ok-2026-01-01.md", "# Nota OK\n")
	writeFile(t, dir, "vault/notes/index.md", "# Vault\n\n## Índice\n\n- [nota-ok-2026-01-01](nota-ok-2026-01-01.md)\n")

	msgs, err := validateNoteOrphan()
	if err != nil {
		t.Fatalf("validateNoteOrphan() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("nota linkada não deve gerar msg, obteve: %v", msgs)
	}
}

// ---------------------------------------------------------------------------
// Testes P2/P3 — adicionados pelo ML-2A (REQ-2026-07-26-robustez-gates)
// ---------------------------------------------------------------------------

// TestContentHasMarker_CRLF_P3 — P3: marcador com campo vazio em arquivo CRLF
// não deve ser tratado como "presente" (gate precisa reprovar).
// Fixture: "REQ: \r\n" — campo vazio com terminação CRLF.
func TestContentHasMarker_CRLF_P3(t *testing.T) {
	t.Run("campo_vazio_crlf_nao_conta_como_presente", func(t *testing.T) {
		content := "# Roadmap\r\nREQ: \r\n## Seção\r\n"
		if contentHasMarker(content, []string{"REQ:"}) {
			t.Error("campo vazio com CRLF não deve ser tratado como marcador presente")
		}
	})
	t.Run("campo_preenchido_crlf_deve_contar", func(t *testing.T) {
		content := "# Roadmap\r\nREQ: REQ-001-titulo.md\r\n## Seção\r\n"
		if !contentHasMarker(content, []string{"REQ:"}) {
			t.Error("campo preenchido com CRLF deve ser tratado como marcador presente")
		}
	})
	t.Run("campo_vazio_lf_nao_conta_como_presente", func(t *testing.T) {
		content := "# Roadmap\nREQ: \n## Seção\n"
		if contentHasMarker(content, []string{"REQ:"}) {
			t.Error("campo vazio com LF não deve ser tratado como marcador presente")
		}
	})
}

// TestFolderStatus_DiretorioNaoLegivel_P2 — P2: pasta de estado que EXISTE mas não pode ser
// lida (ENOTDIR — criada como arquivo regular) deve gerar warning, não silenciar.
// Vetor: "docs/roadmaps/analyzing" é um arquivo regular, não um diretório.
func TestFolderStatus_DiretorioNaoLegivel_P2(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps")
	// Criar "analyzing" como arquivo regular — os.ReadDir retornará ENOTDIR.
	writeFile(t, dir, "docs/roadmaps/analyzing", "eu sou um arquivo, nao um diretorio")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateFolderStatusCoherence()
	if err != nil {
		t.Fatalf("validateFolderStatusCoherence() erro inesperado: %v", err)
	}
	if !hasWarning(warnings, "could not read directory") {
		t.Errorf("esperado warning sobre diretório ilegível, obteve: %v", warnings)
	}
}

// TestFilenameUniqueness_DiretorioNaoLegivel_P2 — P2: pasta de estado que EXISTE mas não pode
// ser lida deve gerar violation, não silenciar (ENOTDIR via arquivo regular).
func TestFilenameUniqueness_DiretorioNaoLegivel_P2(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps")
	// "wip" como arquivo regular — os.ReadDir retornará ENOTDIR.
	writeFile(t, dir, "docs/roadmaps/wip", "eu sou um arquivo, nao um diretorio")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, err := validateFilenameUniqueness()
	if err != nil {
		t.Fatalf("validateFilenameUniqueness() erro inesperado: %v", err)
	}
	if !hasViolation(violations, "could not read directory") {
		t.Errorf("esperado violation sobre diretório ilegível, obteve: %v", violations)
	}
}

// TestFilenameUniqueness_OrdemDeterministica_P3 — P3: mesma roadmap em dois estados
// deve produzir mensagem com estados em ordem alfabética entre execuções.
func TestFilenameUniqueness_OrdemDeterministica_P3(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/roadmaps/done")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-duplicado.md", "# Duplicado\n")
	writeFile(t, dir, "docs/roadmaps/done/ROADMAP-duplicado.md", "# Duplicado\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, err := validateFilenameUniqueness()
	if err != nil {
		t.Fatalf("validateFilenameUniqueness() erro: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("esperado 1 violation, obteve %d: %v", len(violations), violations)
	}
	// Estados devem aparecer em ordem alfabética: [done wip]
	if !strings.Contains(violations[0], "[done wip]") {
		t.Errorf("estados devem estar em ordem alfabética (done antes de wip), obteve: %s", violations[0])
	}
}

// TestValidateBranchHasWIPRoadmap_TruncaMensagem — P3+P4: 4 candidatos devem gerar
// mensagem truncada em 3 + "e mais 1", em ordem alfabética, string exata.
func TestValidateBranchHasWIPRoadmap_TruncaMensagem(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "feat/minha-feature")
	// 4 roadmaps sem slug da branch → todos são candidatos, nenhum casa
	mkdirs(t, dir, "docs/roadmaps/wip")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-alpha.md", "REQ: REQ-001\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-bravo.md", "REQ: REQ-002\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-charlie.md", "REQ: REQ-003\n")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-delta.md", "REQ: REQ-004\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateBranchHasWIPRoadmap()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("esperava violation com 4 candidatos sem slug correspondente")
	}
	// Exato: 3 primeiros em ordem alfabética + ", e mais 1"
	want := "ROADMAP-alpha.md, ROADMAP-bravo.md, ROADMAP-charlie.md, e mais 1"
	if !strings.Contains(violations[0], want) {
		t.Errorf("mensagem truncada esperada contendo %q, obteve: %s", want, violations[0])
	}
}

// ---------------------------------------------------------------------------
// ML-1A — REQ-2026-07-27-convergencia-templates-python
// Testes negativos que provam a cegueira das regras para artefatos Python.
// Estes testes DEVEM FALHAR contra o código atual — a falha é a evidência de P2.
// Reativados no ML-2A após convergência dos templates.
// ---------------------------------------------------------------------------

// TestADRDraft_ValidadorDetectaFormatoCanônico — ML-2A (reativado):
// Após convergência do template Python, ADRs gerados pelo CLI Python emitem
// "> Date: … | Status: Draft" no header. adrIsDraft() deve detectar a string
// e blocked_by_draft_adr deve disparar.
//
// Fixture: ADR no formato canônico Go/Node/Python (após ML-2A) + REQ canônica.
// Resultado esperado: violation disparada.
func TestADRDraft_ValidadorDetectaFormatoCanônico(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	// ADR no formato canônico (produzido pelo gerador Python após ML-2A):
	// header "| Status: Draft" inline — detectado por adrIsDraft().
	adrCanonicoContent := `---
status: Draft
date: 2026-07-27
author: ""
---

# ADR: auth strategy

> Date: 2026-07-27 | Status: Draft

## Context
<!-- What is the situation that motivates this decision? -->

## Decision
<!-- What was decided? -->

## Consequences
<!-- What are the positive and negative consequences of this decision? -->

## Alternatives Considered
<!-- What other options were evaluated and why were they rejected? -->
`
	writeFile(t, dir, "docs/adr/ADR-2026-07-27-auth-strategy.md", adrCanonicoContent)

	// REQ no formato canônico Go/Node: tem "> Date: … | Status: Open"
	// para que a verificação de Open passe, isolando o teste em adrIsDraft.
	reqCanonicalContent := `# REQ: Login

> Date: 2026-07-27 | Status: Open

## Motivation

## Acceptance Criteria

- [ ] criterio

## Linked ADR
ADR:

## Blocked by ADRs
- ADR-2026-07-27-auth-strategy.md (Draft)

## Linked Roadmap
Roadmap:
`
	writeFile(t, dir, "docs/req/REQ-2026-07-27-login.md", reqCanonicalContent)
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Pré-condição: o ADR deve ser encontrado
	adrPath := filepath.Join(dir, "docs", "adr", "ADR-2026-07-27-auth-strategy.md")
	if _, err := os.Stat(adrPath); err != nil {
		t.Fatalf("pré-condição: ADR não encontrado em %s: %v", adrPath, err)
	}

	violations, err := validateREQsNotBlockedByDraftADRs()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// DEVE disparar violation — formato canônico contém "Status: Draft" que adrIsDraft detecta
	if len(violations) == 0 {
		t.Errorf("regressão: blocked_by_draft_adr não detectou ADR Draft no formato canônico. "+
			"ADR existe em %s, adrIsDraft() deve retornar true para '>Status: Draft' inline. violations: %v", adrPath, violations)
	}
}

// TestREQOpen_ValidadorDetectaFormatoCanônico — ML-2A (reativado):
// Após convergência do template Python, REQs geradas pelo CLI Python emitem
// "> Date: … | Status: Open" no header. validateREQsNotBlockedByDraftADRs() deve
// detectar a REQ e disparar violation ao encontrar ADR Draft no bloqueio.
//
// Fixture: REQ no formato canônico Go/Node/Python (após ML-2A) + ADR canônico Draft.
// Resultado esperado: violation disparada.
func TestREQOpen_ValidadorDetectaFormatoCanônico(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	// ADR no formato canônico Go/Node: tem "> Date: … | Status: Draft"
	// para que adrIsDraft() o detecte corretamente.
	adrCanonicalContent := `# ADR: Auth

> Date: 2026-07-27 | Status: Draft

## Context
context
`
	writeFile(t, dir, "docs/adr/ADR-2026-07-27-auth-draft.md", adrCanonicalContent)

	// REQ no formato canônico (produzida pelo gerador Python após ML-2A):
	// header "> Date: … | Status: Open" detectado pelo guard inicial.
	reqCanonicoContent := `---
status: Open
date: 2026-07-27
author: ""
adr: ""
roadmap: ""
---

# REQ: login

> Date: 2026-07-27 | Status: Open

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ] criterio 1

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR:

## Blocked by ADRs
- ADR-2026-07-27-auth-draft.md (Draft)

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap:
`
	writeFile(t, dir, "docs/req/REQ-2026-07-27-login.md", reqCanonicoContent)
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Pré-condição: ADR canônico deve ser detectado como Draft
	if !adrIsDraft("ADR-2026-07-27-auth-draft.md") {
		t.Fatalf("pré-condição falhou: adrIsDraft deve retornar true para ADR canônico com '| Status: Draft'")
	}

	violations, err := validateREQsNotBlockedByDraftADRs()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// DEVE disparar violation — formato canônico contém "Status: Open" que o guard detecta
	if len(violations) == 0 {
		t.Errorf("regressão: blocked_by_draft_adr não detectou REQ Open no formato canônico. "+
			"REQ tem '> Date: … | Status: Open' (inline) — deve ser detectada. violations: %v", violations)
	}
}

// TestWIPHasREQ_CRLF_Integracao — P3+P4: roadmap em wip com "REQ: \r\n" (CRLF vazio)
// deve emitir violation de wip_has_req. Testa a integração do contentHasMarker
// com validateWIPHasREQ (leitura de arquivo real, não mock).
func TestWIPHasREQ_CRLF_Integracao(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip")
	// Arquivo com CRLF: REQ: seguido de espaço + \r\n — campo vazio
	content := "REQ: \r\n## Acceptance Criteria\r\n- [ ] ok\r\n"
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-crlf.md", content)
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateWIPHasREQ()
	if err != nil {
		t.Fatalf("validateWIPHasREQ() erro inesperado: %v", err)
	}
	if !hasViolation(violations, "wip but has no linked REQ") {
		t.Errorf("esperava violation de REQ vazio com CRLF, obteve: %v", violations)
	}
}

// ---------------------------------------------------------------------------
// ML-1A — REQ-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida
// adrStatusIsNotAccepted() (helper canônico) + regra nova adr_accepted_when_req_done
// + correção da cegueira de blocked_by_draft_adr a Status: Proposed.
// ---------------------------------------------------------------------------

// adrFixtureContent monta o conteúdo de um ADR fixture com o status dado, alinhado
// entre frontmatter e cabeçalho (caso canônico bem formado).
func adrFixtureContent(status string) string {
	return "---\n" +
		"status: " + status + "\n" +
		"date: 2026-08-01\n" +
		"author: \"\"\n" +
		"---\n\n" +
		"# ADR: fixture\n\n" +
		"> Date: 2026-08-01 | Status: " + status + "\n\n" +
		"## Context\nctx\n\n" +
		"## Decision\ndecision\n"
}

// reqDoneFixtureContent monta um REQ Done canônico referenciando o basename de ADR
// dado via frontmatter `adr:` e via a seção "## Linked ADR" (mesmo campo "ADR:" que
// extractRefPath e validateRefTargetsExist já leem).
func reqDoneFixtureContent(adrRelPath string) string {
	return "---\n" +
		"status: Done\n" +
		"date: 2026-08-01\n" +
		"author: \"\"\n" +
		"adr: \"" + adrRelPath + "\"\n" +
		"roadmap: \"\"\n" +
		"---\n\n" +
		"# REQ: fixture\n\n" +
		"> Date: 2026-08-01 | Status: Done\n\n" +
		"## Motivation\nmotivo\n\n" +
		"## Acceptance Criteria\n- [x] feito\n\n" +
		"## Linked ADR\nADR: " + adrRelPath + "\n\n" +
		"## Linked Roadmap\nRoadmap:\n"
}

// TestADRAcceptedWhenREQDone_ProposedADR_Violates — REQ Done + ADR Proposed deve
// disparar violation citando os dois artefatos (REQ e ADR).
func TestADRAcceptedWhenREQDone_ProposedADR_Violates(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	adrRel := "docs/adr/ADR-2026-08-01-proposed-fixture.md"
	writeFile(t, dir, adrRel, adrFixtureContent("Proposed"))
	writeFile(t, dir, "docs/req/REQ-2026-08-01-done-fixture.md", reqDoneFixtureContent(adrRel))
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateADRAcceptedWhenREQDone()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("esperava violation para REQ Done com ADR Proposed, obteve nenhuma")
	}
	if !hasViolation(violations, "REQ-2026-08-01-done-fixture.md") {
		t.Errorf("mensagem deve citar a REQ, obteve: %v", violations)
	}
	if !hasViolation(violations, "ADR-2026-08-01-proposed-fixture.md") {
		t.Errorf("mensagem deve citar o ADR, obteve: %v", violations)
	}
}

// TestADRAcceptedWhenREQDone_DraftADR_Violates — REQ Done + ADR Draft também dispara
// (a definição de "não aceito" cobre os dois status, não só Proposed).
func TestADRAcceptedWhenREQDone_DraftADR_Violates(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	adrRel := "docs/adr/ADR-2026-08-01-draft-fixture.md"
	writeFile(t, dir, adrRel, adrFixtureContent("Draft"))
	writeFile(t, dir, "docs/req/REQ-2026-08-01-done-fixture.md", reqDoneFixtureContent(adrRel))
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateADRAcceptedWhenREQDone()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("esperava violation para REQ Done com ADR Draft, obteve nenhuma")
	}
}

// TestADRAcceptedWhenREQDone_SupersededADR_SemViolation — "aceito" é por exclusão:
// Superseded não é Draft nem Proposed, então não deve violar mesmo com REQ Done.
func TestADRAcceptedWhenREQDone_SupersededADR_SemViolation(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	adrRel := "docs/adr/ADR-2026-08-01-superseded-fixture.md"
	writeFile(t, dir, adrRel, adrFixtureContent("Superseded"))
	writeFile(t, dir, "docs/req/REQ-2026-08-01-done-fixture.md", reqDoneFixtureContent(adrRel))
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateADRAcceptedWhenREQDone()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("não esperava violation para ADR Superseded (aceito por exclusão), obteve: %v", violations)
	}
}

// TestADRAcceptedWhenREQDone_REQOpen_ProposedADR_SemViolationDaRegraNova — a regra
// nova só olha REQs Done; uma REQ Open com ADR Proposed é responsabilidade de
// blocked_by_draft_adr (se estiver na seção "## Blocked by ADRs"), não desta regra.
func TestADRAcceptedWhenREQDone_REQOpen_ProposedADR_SemViolationDaRegraNova(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	adrRel := "docs/adr/ADR-2026-08-01-proposed-open-fixture.md"
	writeFile(t, dir, adrRel, adrFixtureContent("Proposed"))

	reqOpenContent := "---\n" +
		"status: Open\n" +
		"date: 2026-08-01\n" +
		"author: \"\"\n" +
		"adr: \"" + adrRel + "\"\n" +
		"roadmap: \"\"\n" +
		"---\n\n" +
		"# REQ: fixture aberta\n\n" +
		"> Date: 2026-08-01 | Status: Open\n\n" +
		"## Motivation\nmotivo\n\n" +
		"## Acceptance Criteria\n- [ ] pendente\n\n" +
		"## Linked ADR\nADR: " + adrRel + "\n\n" +
		"## Linked Roadmap\nRoadmap:\n"
	writeFile(t, dir, "docs/req/REQ-2026-08-01-open-fixture.md", reqOpenContent)
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	violations, err := validateADRAcceptedWhenREQDone()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("não esperava violation de adr_accepted_when_req_done para REQ Open, obteve: %v", violations)
	}
}

// TestBlockedByDraftADR_REQOpen_ProposedADR_Violates — corrige a cegueira: antes,
// adrDraftStatusForRule só reconhecia "Status: Draft"; um ADR criado por `adr new`
// (o caminho normal, que emite Status: Proposed) bloqueando uma REQ Open não disparava
// nada. Agora deve disparar via blocked_by_draft_adr (nome da regra preservado).
func TestBlockedByDraftADR_REQOpen_ProposedADR_Violates(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	adrRel := "docs/adr/ADR-2026-08-01-proposed-blocker.md"
	writeFile(t, dir, adrRel, adrFixtureContent("Proposed"))

	reqOpenContent := "# REQ: bloqueada por Proposed\n\n" +
		"> Date: 2026-08-01 | Status: Open\n\n" +
		"## Motivation\nmotivo\n\n" +
		"## Acceptance Criteria\n- [ ] pendente\n\n" +
		"## Linked ADR\nADR:\n\n" +
		"## Blocked by ADRs\n- ADR-2026-08-01-proposed-blocker.md (Proposed)\n\n" +
		"## Linked Roadmap\nRoadmap:\n"
	writeFile(t, dir, "docs/req/REQ-2026-08-01-blocked-fixture.md", reqOpenContent)
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Pré-condição: sem a correção, o ADR não seria reconhecido como não-aceito.
	if !adrStatusIsNotAccepted(adrFixtureContent("Proposed")) {
		t.Fatalf("pré-condição falhou: adrStatusIsNotAccepted deve retornar true para Status: Proposed")
	}

	violations, err := validateREQsNotBlockedByDraftADRs()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("regressão: blocked_by_draft_adr não detectou ADR Proposed bloqueando REQ Open")
	}
	if !hasViolation(violations, "REQ-2026-08-01-blocked-fixture.md") {
		t.Errorf("mensagem deve citar a REQ, obteve: %v", violations)
	}
	if !hasViolation(violations, "ADR-2026-08-01-proposed-blocker.md") {
		t.Errorf("mensagem deve citar o ADR, obteve: %v", violations)
	}
}

// TestADRAcceptedWhenREQDone_FrontmatterStatusVazio_CaiParaCabecalho — REQ com
// `status: ""` no frontmatter (campo presente mas vazio, formato real usado pelos
// geradores para campos não preenchidos) deve cair para o cabeçalho "| Status: Done"
// em vez de ser tratada como "não é Done" por engano.
func TestADRAcceptedWhenREQDone_FrontmatterStatusVazio_CaiParaCabecalho(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	adrRel := "docs/adr/ADR-2026-08-01-proposed-fallback.md"
	writeFile(t, dir, adrRel, adrFixtureContent("Proposed"))

	reqContent := "---\n" +
		"status: \"\"\n" +
		"date: 2026-08-01\n" +
		"author: \"\"\n" +
		"adr: \"" + adrRel + "\"\n" +
		"roadmap: \"\"\n" +
		"---\n\n" +
		"# REQ: fixture status vazio no frontmatter\n\n" +
		"> Date: 2026-08-01 | Status: Done\n\n" +
		"## Motivation\nmotivo\n\n" +
		"## Acceptance Criteria\n- [x] feito\n\n" +
		"## Linked ADR\nADR: " + adrRel + "\n\n" +
		"## Linked Roadmap\nRoadmap:\n"
	writeFile(t, dir, "docs/req/REQ-2026-08-01-status-vazio-fixture.md", reqContent)
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	if !reqStatusIsDone(reqContent) {
		t.Fatalf("reqStatusIsDone deve cair para o cabeçalho quando frontmatter status está vazio")
	}

	violations, err := validateADRAcceptedWhenREQDone()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("esperava violation: REQ Done via cabeçalho (frontmatter status vazio) + ADR Proposed")
	}
}

// TestAdrStatusIsNotAccepted_FrontmatterOnly_SemLinhaDeCabecalho — ML-1D, divergência A
// da auditoria de paridade: um ADR pode ter frontmatter `status:` sem nenhuma linha de
// cabeçalho "| Status: X" (ex.: cabeçalho reescrito ou omitido). O resultado deve vir
// do frontmatter, não exigir o cabeçalho como pré-condição. Este é o caso que
// discriminava o Node (que lia só a linha de cabeçalho) do Go e do Python.
func TestAdrStatusIsNotAccepted_FrontmatterOnly_SemLinhaDeCabecalho(t *testing.T) {
	content := "---\nstatus: Proposed\ndate: 2026-08-01\n---\n\n# ADR: sem cabeçalho\n\n## Context\nctx\n"
	if !adrStatusIsNotAccepted(content) {
		t.Error("esperava true a partir do frontmatter mesmo sem linha de cabeçalho '| Status:'")
	}

	acceptedContent := "---\nstatus: Accepted\ndate: 2026-08-01\n---\n\n# ADR: sem cabeçalho\n\n## Context\nctx\n"
	if adrStatusIsNotAccepted(acceptedContent) {
		t.Error("frontmatter Accepted sem linha de cabeçalho não deve ser tratado como não-aceito")
	}
}

// TestAdrStatusIsNotAccepted_HeaderFallback — sem frontmatter, cai para o cabeçalho.
func TestAdrStatusIsNotAccepted_HeaderFallback(t *testing.T) {
	content := "# ADR: legado\n\n> Date: 2026-08-01 | Status: Proposed\n\n## Context\nctx\n"
	if !adrStatusIsNotAccepted(content) {
		t.Error("esperava true via fallback de cabeçalho para ADR sem frontmatter")
	}
}

// TestAdrStatusIsNotAccepted_FrontmatterPrecedeProse — o valor do frontmatter decide;
// uma menção solta a "Status: Draft" na prosa do corpo não deve enganar o helper
// (prova de que não é mais um strings.Contains ingênuo no corpo inteiro).
func TestAdrStatusIsNotAccepted_FrontmatterPrecedeProse(t *testing.T) {
	content := "---\nstatus: Accepted\ndate: 2026-08-01\n---\n\n" +
		"# ADR: x\n\n> Date: 2026-08-01 | Status: Accepted\n\n" +
		"## Context\nEste ADR substitui uma proposta anterior que ficou em Status: Draft por meses.\n"
	if adrStatusIsNotAccepted(content) {
		t.Error("frontmatter Accepted deve prevalecer sobre menção a 'Status: Draft' na prosa")
	}
}

// ---------------------------------------------------------------------------
// ML-1A — REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python
// extractRefPath deve remover backticks, não só aspas, ao extrair o valor do campo
// ADR:/REQ:/Roadmap:. Sem isso, `` ADR: `docs/adr/X.md` (prosa) `` não termina em
// ".md" e a referência fica invisível em silêncio.
// ---------------------------------------------------------------------------

// TestADRAcceptedWhenREQDone_ADREntreBackticksSemFrontmatter_Violates — teste
// discriminante do backtick: REQ Done SEM `adr:` no frontmatter (como as 3 REQs reais
// do repositório), referenciando o ADR só no corpo via "ADR: `caminho` (prosa)". Antes
// da correção do ML-1A, extractRefPath não reconhecia esse valor como .md (o cutset de
// strings.Trim não incluía backtick) e a regra não enxergava a referência — provado
// abaixo por uma simulação inline do cutset antigo. Depois da correção, a mesma fixture
// deve violar adr_accepted_when_req_done porque o ADR está Proposed.
func TestADRAcceptedWhenREQDone_ADREntreBackticksSemFrontmatter_Violates(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/req", "docs/adr")

	adrRel := "docs/adr/ADR-2026-08-02-proposed-backtick-fixture.md"
	writeFile(t, dir, adrRel, adrFixtureContent("Proposed"))

	// REQ Done sem `adr:` no frontmatter (igual às 3 REQs reais afetadas) — a única
	// referência ao ADR está no corpo, entre backticks, seguida de prosa.
	reqContent := "---\n" +
		"status: Done\n" +
		"date: 2026-08-02\n" +
		"author: \"\"\n" +
		"roadmap: \"\"\n" +
		"---\n\n" +
		"# REQ: fixture com ADR entre backticks\n\n" +
		"> Date: 2026-08-02 | Status: Done\n\n" +
		"## Motivation\nmotivo\n\n" +
		"## Acceptance Criteria\n- [x] feito\n\n" +
		"## Linked ADR\nADR: `" + adrRel + "` (P1–P4; esta REQ é referenciada por prosa após o backtick)\n\n" +
		"## Linked Roadmap\nRoadmap:\n"
	writeFile(t, dir, "docs/req/REQ-2026-08-02-backtick-fixture.md", reqContent)
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	config.Reset()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Pré-condição — prova do comportamento ANTES da correção: o cutset antigo de
	// strings.Trim (só aspas) não removia o backtick, então o token não terminava em
	// ".md" e a referência ficava invisível.
	rawField := "`" + adrRel + "`"
	oldTrim := strings.Trim(rawField, `"'`)
	if strings.HasSuffix(oldTrim, ".md") {
		t.Fatalf("pré-condição inválida: cutset antigo (sem backtick) não deveria produzir sufixo .md, obteve %q", oldTrim)
	}

	// Comportamento ATUAL — extractRefPath deve resolver a referência corretamente.
	if got := extractRefPath(reqContent, "ADR"); got != adrRel {
		t.Fatalf("extractRefPath deve resolver %q entre backticks, obteve %q", adrRel, got)
	}

	violations, err := validateADRAcceptedWhenREQDone()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("regressão do ML-1A: ADR entre backticks + prosa não foi detectado como não-aceito")
	}
	if !hasViolation(violations, "REQ-2026-08-02-backtick-fixture.md") {
		t.Errorf("mensagem deve citar a REQ, obteve: %v", violations)
	}
	if !hasViolation(violations, "ADR-2026-08-02-proposed-backtick-fixture.md") {
		t.Errorf("mensagem deve citar o ADR, obteve: %v", violations)
	}
}

// TestExtractRefPath_TresREQsReaisDoRepositorio — as 3 REQs reais do repositório sem
// `adr:` no frontmatter, cujo ADR só é referenciado no corpo entre backticks, devem ter
// o ADR resolvido pelo extrator após a correção do ML-1A.
func TestExtractRefPath_TresREQsReaisDoRepositorio(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("erro ao resolver raiz do repositório: %v", err)
	}

	reqs := []string{
		"docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md",
		"docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md",
		"docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md",
	}

	for _, rel := range reqs {
		t.Run(rel, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(repoRoot, rel))
			if err != nil {
				t.Fatalf("erro ao ler REQ real %q: %v", rel, err)
			}
			got := extractRefPath(string(content), "ADR")
			if got == "" {
				t.Fatalf("extractRefPath não resolveu o ADR de %q — regressão do ML-1A", rel)
			}
			if !strings.HasSuffix(got, ".md") {
				t.Errorf("ADR resolvido de %q deveria terminar em .md, obteve %q", rel, got)
			}
		})
	}
}
