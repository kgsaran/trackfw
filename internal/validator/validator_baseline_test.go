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
