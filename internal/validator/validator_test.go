package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
