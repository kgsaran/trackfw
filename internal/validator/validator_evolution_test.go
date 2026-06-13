package validator

import (
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestFieldMapping_ReqId_SatisfiesWipHasREQ verifica que link_fields.req customizado
// (ex: "req_id") é aceito pelo validator como satisfação da regra wip_has_req.
func TestFieldMapping_ReqId_SatisfiesWipHasREQ(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/req",
		"docs/adr",
	)
	writeFile(t, dir, "trackfw.yaml", `link_fields:
  req:
    - req_id
`)
	// Roadmap com "req_id:" em vez de "REQ:" — deve satisfazer a regra com config customizada
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-custom-req.md", `# Roadmap: Custom REQ field

req_id: docs/req/REQ-001.md

## Acceptance Criteria
- [ ] build passa
`)
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if hasViolation(violations, "no linked REQ") {
		t.Errorf("não esperado violation 'no linked REQ' com link_fields.req=[req_id], obteve: %v", violations)
	}
}

// TestRuleSeverity_Off_AdrOrphan verifica que adr_orphan configurado como "off"
// silencia completamente violações de ADR sem referência.
func TestRuleSeverity_Off_AdrOrphan(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/req",
		"docs/adr",
	)
	writeFile(t, dir, "trackfw.yaml", `rules:
  adr_orphan: off
`)
	// ADR sem referência em nenhuma REQ — normalmente geraria violation/warning de adr_orphan
	writeFile(t, dir, "docs/adr/ADR-001-orfao.md", `---
status: Accepted
date: 2026-06-13
---
# ADR-001: Sem referência
`)
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if hasViolation(violations, "ADR-001-orfao") {
		t.Errorf("não esperado violation para adr_orphan=off, obteve violations: %v", violations)
	}
	if hasWarning(warnings, "ADR-001-orfao") {
		t.Errorf("não esperado warning para adr_orphan=off, obteve warnings: %v", warnings)
	}
}

// TestRuleSeverity_Warning_WipHasReq verifica que wip_has_req configurado como "warning"
// move a mensagem para warnings em vez de violations.
func TestRuleSeverity_Warning_WipHasReq(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/req",
		"docs/adr",
	)
	writeFile(t, dir, "trackfw.yaml", `rules:
  wip_has_req: warning
`)
	// Roadmap em wip sem REQ e com critérios de aceite — wip_has_req deve ir para warnings
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-sem-req.md", `# Roadmap: Sem REQ

## Acceptance Criteria
- [ ] build passa
`)
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, warnings, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if hasViolation(violations, "no linked REQ") {
		t.Errorf("não esperado violation quando wip_has_req=warning, obteve violations: %v", violations)
	}
	if !hasWarning(warnings, "no linked REQ") {
		t.Errorf("esperado warning 'no linked REQ' quando wip_has_req=warning, obteve warnings: %v", warnings)
	}
}

// TestAcceptanceMarkersCustom verifica que acceptance_markers customizado
// ("## Done When") satisfaz a regra wip_acceptance.
func TestAcceptanceMarkersCustom(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir,
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/req",
		"docs/adr",
	)
	writeFile(t, dir, "trackfw.yaml", `acceptance_markers:
  - "## Done When"
  - "## Critérios"
`)
	// Roadmap com "## Done When" — deve satisfazer a regra com markers customizados
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-custom-criteria.md", `# Roadmap: Custom Criteria

REQ: REQ-001.md

## Done When
- [ ] build passa
- [ ] testes verdes
`)
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	violations, _, err := Validate()
	if err != nil {
		t.Fatalf("Validate() erro: %v", err)
	}
	if hasViolation(violations, "no acceptance criteria") {
		t.Errorf("não esperado violation 'no acceptance criteria' com marker customizado '## Done When', obteve: %v", violations)
	}
}
