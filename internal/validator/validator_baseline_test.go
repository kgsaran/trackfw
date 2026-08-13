package validator

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

func TestBaselineCreation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	// roadmap em wip sem REQ → gera violation
	writeFile(t, dir, "docs/roadmaps/wip/RM-001.md",
		"---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n")

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}

	if err := SaveBaseline(violations, warnings); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// Verificar que o arquivo foi criado
	data, err := os.ReadFile(".trackfw-baseline.json")
	if err != nil {
		t.Fatalf("baseline file not created: %v", err)
	}

	var bf BaselineFile
	if err := json.Unmarshal(data, &bf); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(bf.Violations) == 0 {
		t.Error("baseline should contain at least one violation")
	}
	if bf.Created == "" {
		t.Error("baseline.created should not be empty")
	}
}

func TestBaselineFiltersOldViolations(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	// roadmap em wip sem REQ → violation
	writeFile(t, dir, "docs/roadmaps/wip/RM-001.md",
		"---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n")

	// Criar baseline com essa violation
	rawViolations, rawWarnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	if err := SaveBaseline(rawViolations, rawWarnings); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// Validate() com baseline → violation do RM-001 não deve aparecer
	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if hasViolation(violations, "RM-001") {
		t.Error("baseline violation should be filtered out by Validate()")
	}
}

func TestBaselineFiltersWarnings(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	// ADR sem nenhuma REQ referenciando → gera warning de adr_orphan (default severity)
	writeFile(t, dir, "docs/adr/ADR-001.md",
		"---\nstatus: Accepted\ndate: 2026-01-01\n---\n# ADR-001\n")

	// Validar sem filtro para capturar warnings
	rawViolations, rawWarnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	if !hasWarning(rawWarnings, "ADR-001") {
		t.Fatalf("esperado warning de adr_orphan para ADR-001; warnings=%v", rawWarnings)
	}

	// Salvar baseline com o estado atual (inclui o warning)
	if err := SaveBaseline(rawViolations, rawWarnings); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// Validate() com baseline → warning de ADR-001 não deve aparecer
	_, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if hasWarning(warnings, "ADR-001") {
		t.Errorf("warning de adr_orphan baselined deveria ter sido filtrado; warnings=%v", warnings)
	}
}

func TestBaselineLenientNoRecreate(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	// ADR sem REQ → warning de adr_orphan
	writeFile(t, dir, "docs/adr/ADR-002.md",
		"---\nstatus: Accepted\ndate: 2026-01-01\n---\n# ADR-002\n")

	// Criar baseline capturando o warning
	rawViolations, rawWarnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	if !hasWarning(rawWarnings, "ADR-002") {
		t.Fatalf("esperado warning de adr_orphan para ADR-002; warnings=%v", rawWarnings)
	}
	if err := SaveBaseline(rawViolations, rawWarnings); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// Ativar modo lenient via trackfw.yaml
	writeFile(t, dir, "trackfw.yaml", "governance_mode: lenient\n")

	// Validate() com baseline + lenient → warning baselined não deve reaparecer
	_, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if hasWarning(warnings, "ADR-002") {
		t.Errorf("warning baselined não deve reaparecer com modo lenient; warnings=%v", warnings)
	}
}

// TestBaselineCarveOut_CredentialGuardRulesNaoToleradas prova o carve-out do
// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard /
// ADR-2026-08-12-severidade-das-regras-de-credential-guard-...: violations das 3 regras de
// credential-guard continuam sendo reportadas por Validate() mesmo depois de terem sido
// "baselined" — .trackfw-baseline.json não é um canal válido para tolerá-las, ao contrário de
// qualquer outra regra (que TestBaselineFiltersOldViolations acima prova que É filtrada).
func TestBaselineCarveOut_CredentialGuardRulesNaoToleradas(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// HEAD: mode: block. Disco: trackfw.yaml deletado — dispara credential_guard_mode_downgrade.
	commitTrackfwYAML(t, dir, "credential_guard:\n  mode: block\n")
	if err := os.Remove(dir + "/trackfw.yaml"); err != nil {
		t.Fatalf("remover trackfw.yaml: %v", err)
	}
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	rawViolations, rawWarnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered: %v", err)
	}
	if !hasViolation(rawViolations, "credential_guard.mode: block") {
		t.Fatalf("esperado violation de credential_guard_mode_downgrade antes do baseline; violations=%v", rawViolations)
	}

	// Tentar tolerar via baseline — como qualquer outra violation seria.
	if err := SaveBaseline(rawViolations, rawWarnings); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !hasViolation(violations, "credential_guard.mode: block") {
		t.Errorf("violation de credential_guard_mode_downgrade NÃO deveria ser tolerável via baseline; violations=%v", violations)
	}
}

func TestBaselineNetNewViolation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	mkdirs(t, dir, "docs/adr", "docs/req", "docs/roadmaps/wip",
		"docs/roadmaps/backlog", "docs/roadmaps/blocked", "docs/roadmaps/done")

	// Estado inicial: sem violations
	if err := SaveBaseline([]string{}, []string{}); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	// Adicionar novo roadmap em wip sem REQ → nova violation
	writeFile(t, dir, "docs/roadmaps/wip/RM-002.md",
		"---\nstatus: WIP\n---\n## Acceptance Criteria\n- [ ] done\n")

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !hasViolation(violations, "RM-002") {
		t.Error("net-new violation for RM-002 should be reported")
	}
}
