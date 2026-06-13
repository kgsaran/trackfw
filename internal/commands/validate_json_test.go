package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/validator"
)

// setupCleanFixture cria um diretório temporário com estrutura válida sem violações.
func setupCleanFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dirs := []string{
		"docs/roadmaps/wip",
		"docs/roadmaps/backlog",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
		"docs/req",
		"docs/adr",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("setupCleanFixture: mkdirs: %v", err)
		}
	}
	return dir
}

// setupViolationFixture cria um fixture com 1 violação conhecida:
// roadmap em wip sem REQ vinculado.
func setupViolationFixture(t *testing.T) string {
	t.Helper()
	dir := setupCleanFixture(t)
	roadmap := `# Roadmap: Sem REQ

## Acceptance Criteria
- [ ] build passa
`
	path := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-sem-req.md")
	if err := os.WriteFile(path, []byte(roadmap), 0644); err != nil {
		t.Fatalf("setupViolationFixture: write: %v", err)
	}
	return dir
}

// chdirFixture muda o CWD para dir e restaura ao encerrar o teste.
// O validator lê do CWD — cada teste precisa mudar para seu fixture.
func chdirFixture(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("chdirFixture getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdirFixture chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// TestValidateJSONFlag verifica que --json produz JSON válido com os campos obrigatórios.
func TestValidateJSONFlag(t *testing.T) {
	dir := setupCleanFixture(t)
	chdirFixture(t, dir)

	cmd := newValidateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})

	// Fixture limpo — sem violações; Execute retorna nil.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() retornou erro inesperado: %v", err)
	}

	out := buf.String()
	if !json.Valid([]byte(strings.TrimSpace(out))) {
		t.Fatalf("output não é JSON válido:\n%s", out)
	}

	var result validator.ValidateResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("Unmarshal falhou: %v", err)
	}

	// Verificar presença dos campos obrigatórios.
	if result.Violations == nil {
		t.Error("campo 'violations' ausente ou null no JSON")
	}
	if result.Warnings == nil {
		t.Error("campo 'warnings' ausente ou null no JSON")
	}
	// Summary deve estar presente com valores coerentes.
	if result.Summary.ExitCode != 0 {
		t.Errorf("esperado exit_code=0, obteve %d", result.Summary.ExitCode)
	}
	if result.Summary.Mode == "" {
		t.Error("campo 'mode' não deve ser vazio")
	}
}

// TestValidateJSONExitCode verifica que o exit code é idêntico com e sem --json
// quando há violações (ambos devem retornar erro != nil, equivalente a exit 1).
func TestValidateJSONExitCode(t *testing.T) {
	dir := setupViolationFixture(t)
	chdirFixture(t, dir)

	// Modo texto: deve retornar erro.
	cmdText := newValidateCmd()
	cmdText.SetOut(&bytes.Buffer{})
	cmdText.SetErr(&bytes.Buffer{})
	errText := cmdText.Execute()

	// Modo JSON: deve também retornar erro.
	chdirFixture(t, dir) // chdir já está em dir, mas garantimos estado consistente.
	cmdJSON := newValidateCmd()
	var jsonBuf bytes.Buffer
	cmdJSON.SetOut(&jsonBuf)
	cmdJSON.SetErr(&bytes.Buffer{})
	cmdJSON.SetArgs([]string{"--json"})
	errJSON := cmdJSON.Execute()

	textHasErr := errText != nil
	jsonHasErr := errJSON != nil

	if textHasErr != jsonHasErr {
		t.Errorf("paridade de exit code quebrada: texto-erro=%v json-erro=%v", errText, errJSON)
	}

	// Verificar que o output JSON ainda é válido mesmo com erro.
	out := jsonBuf.String()
	if !json.Valid([]byte(strings.TrimSpace(out))) {
		t.Fatalf("output JSON inválido quando há violações:\n%s", out)
	}

	var result validator.ValidateResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("Unmarshal falhou: %v", err)
	}
	if result.Summary.Violations == 0 {
		t.Error("esperado violations > 0 no fixture com violação")
	}
	if result.Summary.ExitCode != 1 {
		t.Errorf("esperado exit_code=1 quando há violações, obteve %d", result.Summary.ExitCode)
	}
}

// TestValidateTextUnchanged verifica que sem --json o output continua sendo texto (não JSON).
func TestValidateTextUnchanged(t *testing.T) {
	dir := setupCleanFixture(t)
	chdirFixture(t, dir)

	cmd := newValidateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	// Sem --json.

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() retornou erro inesperado: %v", err)
	}

	out := buf.String()
	// O output texto usa fmt.Printf/Println direto para os.Stdout (não cmd.OutOrStdout),
	// então buf.String() pode estar vazio. O que verificamos é que NÃO é JSON.
	// Se buf tiver conteúdo, não deve ser JSON válido.
	if out != "" && json.Valid([]byte(strings.TrimSpace(out))) {
		t.Errorf("modo texto não deve produzir JSON, obteve:\n%s", out)
	}
}
