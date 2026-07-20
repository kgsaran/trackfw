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

// chdir muda para dir e restaura ao fim do teste
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
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
	if !strings.Contains(output, "⏳ REQs blocked by Draft ADRs") {
		t.Error("output não contém seção de REQs bloqueadas")
	}
	if !strings.Contains(output, "ADR-2026-06-12-auth.md") {
		t.Error("output não menciona o ADR bloqueante")
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
	if !hasViolation(violations, "no roadmap is in wip/") {
		t.Errorf("esperava violation de wip vazio, obteve: %v", violations)
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
	if hasViolation(violations, "no roadmap is in wip/") || hasWarning(warnings, "no roadmap is in wip/") {
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

	// REQ referenciando o ADR global
	reqContent := "---\nstatus: Open\ndate: 2026-07-20\n---\n# REQ 001\nADR: ADR-001-global.md\nRoadmap: ROADMAP-001.md\n"
	writeFile(t, dir, "docs/req/REQ-001.md", reqContent)

	// Roadmap linkando REQ
	rmContent := "---\nstatus: WIP\ndate: 2026-07-20\n---\n# Roadmap 001\nREQ: REQ-001.md\n## Acceptance Criteria\n- AC1\n"
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

	// REQ e Roadmap validos linkando o ADR externo
	reqContent := "---\nstatus: Open\ndate: 2026-07-20\nadr: ADR-999-external.md\nroadmap: ROADMAP-001.md\n---\n# REQ 001\nRoadmap: ROADMAP-001.md\nADR: ADR-999-external.md\n"
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

