package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestWalkADRFiles — ADRs em subpastas são encontrados recursivamente
func TestWalkADRFiles(t *testing.T) {
	dir := t.TempDir()

	// Criar subpastas done/ e wip/ dentro do adrDir
	if err := os.MkdirAll(filepath.Join(dir, "done"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "wip"), 0755); err != nil {
		t.Fatal(err)
	}

	// Criar arquivos .md nas subpastas
	if err := os.WriteFile(filepath.Join(dir, "done", "ADR-001-auth.md"), []byte("# ADR"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wip", "ADR-002-cache.md"), []byte("# ADR"), 0644); err != nil {
		t.Fatal(err)
	}
	// Criar um .txt que não deve ser incluído
	if err := os.WriteFile(filepath.Join(dir, "done", "README.txt"), []byte("not md"), 0644); err != nil {
		t.Fatal(err)
	}

	names := walkADRFiles(dir)
	if len(names) != 2 {
		t.Errorf("esperado 2 arquivos .md, obteve %d: %v", len(names), names)
	}

	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["ADR-001-auth.md"] {
		t.Error("ADR-001-auth.md não encontrado")
	}
	if !found["ADR-002-cache.md"] {
		t.Error("ADR-002-cache.md não encontrado")
	}
}

// TestADRDirsRecursiveInValidate — validateADRsAreReferenced detecta ADRs em subpastas
func TestADRDirsRecursiveInValidate(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	// ADR apenas em subpasta (sem arquivo na raiz de docs/adr/)
	if err := os.MkdirAll(filepath.Join(dir, "docs", "adr", "done"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755); err != nil {
		t.Fatal(err)
	}

	adrName := "ADR-2026-06-13-database-choice.md"
	if err := os.WriteFile(filepath.Join(dir, "docs", "adr", "done", adrName), []byte("# ADR: DB Choice\n\n> Status: Accepted"), 0644); err != nil {
		t.Fatal(err)
	}

	// REQ que referencia o ADR pelo basename
	reqContent := "# REQ: Schema\n\nADR: " + adrName + "\nRoadmap: ROADMAP-001.md\n"
	if err := os.WriteFile(filepath.Join(dir, "docs", "req", "REQ-001-schema.md"), []byte(reqContent), 0644); err != nil {
		t.Fatal(err)
	}

	violations, err := validateADRsAreReferenced()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// ADR está referenciado — não deve gerar violation
	if len(violations) != 0 {
		t.Errorf("ADR em subpasta referenciado no REQ não deveria gerar violation, obteve: %v", violations)
	}
}

// TestValidateStaleWIPFallback — em dir sem git, fallback para mtime funciona sem pânico
func TestValidateStaleWIPFallback(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", "wip"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "adr"), 0755); err != nil {
		t.Fatal(err)
	}

	// Arquivo com mtime recente — não deve gerar warning de stale
	wipPath := filepath.Join(dir, "docs", "roadmaps", "wip", "ROADMAP-new.md")
	if err := os.WriteFile(wipPath, []byte("# Roadmap: New\nREQ: REQ-001\n## Acceptance Criteria\n- [ ] ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Não deve panicar mesmo sem git
	_, err := validateStaleWIP()
	if err != nil {
		t.Fatalf("validateStaleWIP() não deve retornar erro sem git: %v", err)
	}
}

// TestExtractRefPath — extrai caminhos .md de campos e ignora valores vazios/traço
func TestExtractRefPath(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		field    string
		expected string
	}{
		{
			name:     "REQ com caminho válido",
			content:  "REQ: docs/req/foo.md\n",
			field:    "REQ",
			expected: "docs/req/foo.md",
		},
		{
			name:     "REQ vazio",
			content:  "REQ:\n",
			field:    "REQ",
			expected: "",
		},
		{
			name:     "REQ com traço em-dash",
			content:  "REQ: —\n",
			field:    "REQ",
			expected: "",
		},
		{
			name:     "Roadmap com caminho válido",
			content:  "Roadmap: docs/roadmaps/bar.md\n",
			field:    "Roadmap",
			expected: "docs/roadmaps/bar.md",
		},
		{
			name:     "Campo ausente",
			content:  "ADR: docs/adr/adr-001.md\n",
			field:    "REQ",
			expected: "",
		},
		{
			name:     "Valor sem extensão .md ignorado",
			content:  "REQ: algum-valor-sem-md\n",
			field:    "REQ",
			expected: "",
		},
		{
			name:     "Traço hífen simples",
			content:  "REQ: -\n",
			field:    "REQ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRefPath(tt.content, tt.field)
			if got != tt.expected {
				t.Errorf("extractRefPath(%q, %q) = %q, esperado %q", tt.content, tt.field, got, tt.expected)
			}
		})
	}
}

// TestRefTargetsExistWarning — roadmap com REQ: apontando para arquivo inexistente → warning
func TestRefTargetsExistWarning(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", "wip"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755); err != nil {
		t.Fatal(err)
	}

	// Roadmap em wip com REQ: que não existe no filesystem
	wipContent := "# Roadmap: Test\n\nREQ: docs/req/nao-existe.md\n\n## Acceptance Criteria\n- [ ] ok\n"
	if err := os.WriteFile(filepath.Join(dir, "docs", "roadmaps", "wip", "ROADMAP-test.md"), []byte(wipContent), 0644); err != nil {
		t.Fatal(err)
	}

	warnings, err := validateRefTargetsExist()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !hasWarning(warnings, "nao-existe.md") {
		t.Errorf("esperado warning sobre REQ inexistente, obteve: %v", warnings)
	}
}

// TestFolderStatusCoherence — arquivo em wip/ com status: Done → warning; status: WIP → sem warning
func TestFolderStatusCoherence(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	for _, s := range []string{"wip", "backlog", "blocked", "done", "abandoned"} {
		if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", s), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Arquivo em wip/ com status Done → deve gerar warning
	incoherentContent := "---\nstatus: Done\ndate: 2026-06-13\n---\n# Roadmap: Incoherente\n"
	if err := os.WriteFile(filepath.Join(dir, "docs", "roadmaps", "wip", "ROADMAP-incoherent.md"), []byte(incoherentContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Arquivo em wip/ com status WIP → não deve gerar warning
	coherentContent := "---\nstatus: WIP\ndate: 2026-06-13\n---\n# Roadmap: Coerente\n"
	if err := os.WriteFile(filepath.Join(dir, "docs", "roadmaps", "wip", "ROADMAP-coherent.md"), []byte(coherentContent), 0644); err != nil {
		t.Fatal(err)
	}

	warnings, err := validateFolderStatusCoherence()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	// Deve ter warning para o incoerente
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "ROADMAP-incoherent.md") && strings.Contains(w, "Done") {
			found = true
		}
	}
	if !found {
		t.Errorf("esperado warning sobre ROADMAP-incoherent.md com status Done, obteve: %v", warnings)
	}

	// Não deve ter warning para o coerente
	for _, w := range warnings {
		if strings.Contains(w, "ROADMAP-coherent.md") {
			t.Errorf("não deveria haver warning para ROADMAP-coherent.md, obteve: %q", w)
		}
	}
}

// TestFilenameUniqueness — mesmo filename em wip/ e backlog/ → violation
func TestFilenameUniqueness(t *testing.T) {
	dir := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	chdir(t, dir)

	for _, s := range []string{"wip", "backlog", "blocked", "done", "abandoned"} {
		if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", s), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Mesmo filename em dois estados diferentes
	content := "# Roadmap: Duplicate\n"
	dupName := "ROADMAP-duplicado.md"
	if err := os.WriteFile(filepath.Join(dir, "docs", "roadmaps", "wip", dupName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "roadmaps", "backlog", dupName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Arquivo único que não deve gerar violation
	if err := os.WriteFile(filepath.Join(dir, "docs", "roadmaps", "done", "ROADMAP-unico.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	violations, err := validateFilenameUniqueness()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if !hasViolation(violations, "ROADMAP-duplicado.md") {
		t.Errorf("esperado violation para filename duplicado, obteve: %v", violations)
	}

	for _, v := range violations {
		if strings.Contains(v, "ROADMAP-unico.md") {
			t.Errorf("não deveria haver violation para arquivo único, obteve: %q", v)
		}
	}
}
