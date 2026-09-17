package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// chdirTempWithReset cria um TempDir, muda para ele e reseta o singleton de config.
// Garante que cada caso de teste injete o seu próprio trackfw.yaml sem herdar cache
// de execuções anteriores (config.Load() é once.Do; sem Reset o AC2 mediria o default
// cacheado em vez do req_dir injetado).
func chdirTempWithReset(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	config.Reset()
	t.Cleanup(config.Reset)
	return dir
}

// writeYAMLSync escreve um trackfw.yaml no dir especificado.
func writeYAMLSync(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// setupREQ cria um arquivo REQ no subdiretório especificado (relativo a dir).
func setupREQ(t *testing.T, dir, subdir, filename, content string) string {
	t.Helper()
	reqDir := filepath.Join(dir, subdir)
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(reqDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestSyncToProvider_SkipsNonOpen(t *testing.T) {
	dir := chdirTempWithReset(t)
	// AC4: sem trackfw.yaml com credenciais → syncToProvider usa stub create; nenhuma chamada de rede.

	setupREQ(t, dir, "docs/req", "REQ-2026-01-01-draft.md", `# REQ: Draft Requirement

> Date: 2026-01-01 | Status: Draft

## Motivation
Some motivation here.

## Acceptance Criteria
- [ ]
`)

	called := false
	create := func(title, desc string) (string, error) {
		called = true
		return "ENG-1", nil
	}

	results, err := syncToProvider(create, "linear_issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("expected result to be skipped (non-Open status)")
	}
	if called {
		t.Error("create should not have been called for non-Open REQ")
	}
}

func TestSyncToProvider_SkipsAlreadySynced(t *testing.T) {
	dir := chdirTempWithReset(t)

	setupREQ(t, dir, "docs/req", "REQ-2026-01-01-synced.md", `# REQ: Already Synced

> Date: 2026-01-01 | Status: Open
| linear_issue: ENG-1

## Motivation
Some motivation here.
`)

	called := false
	create := func(title, desc string) (string, error) {
		called = true
		return "ENG-2", nil
	}

	results, err := syncToProvider(create, "linear_issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("expected result to be skipped (already has linear_issue)")
	}
	if called {
		t.Error("create should not have been called for already-synced REQ")
	}
}

func TestSyncToProvider_InjectsField(t *testing.T) {
	dir := chdirTempWithReset(t)

	setupREQ(t, dir, "docs/req", "REQ-2026-01-01-open.md", `# REQ: Open Feature

> Date: 2026-01-01 | Status: Open

## Motivation
Some motivation here.

## Acceptance Criteria
- [ ] criterion one
`)

	create := func(title, desc string) (string, error) {
		return "ENG-123", nil
	}

	results, err := syncToProvider(create, "linear_issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Skipped {
		t.Error("result should not be skipped for Open REQ without issue")
	}
	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}
	if results[0].IssueID != "ENG-123" {
		t.Errorf("expected IssueID ENG-123, got %q", results[0].IssueID)
	}

	// verificar que o arquivo foi atualizado
	content, err := os.ReadFile(filepath.Join(dir, "docs", "req", "REQ-2026-01-01-open.md"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if !strings.Contains(string(content), "| linear_issue: ENG-123") {
		t.Errorf("expected '| linear_issue: ENG-123' in file, got:\n%s", content)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "standard REQ header",
			text:     "# REQ: My Feature Title\n\n> Date: 2026-01-01 | Status: Open",
			expected: "My Feature Title",
		},
		{
			name:     "no title line",
			text:     "> Date: 2026-01-01 | Status: Open\n\n## Motivation\nsome text",
			expected: "",
		},
		{
			name:     "empty text",
			text:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.text)
			if got != tt.expected {
				t.Errorf("extractTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInjectField(t *testing.T) {
	text := `# REQ: Feature

> Date: 2026-01-01 | Status: Open

## Motivation
text here
`

	result := injectField(text, "linear_issue", "ENG-42")

	if !strings.Contains(result, "| linear_issue: ENG-42") {
		t.Errorf("expected injected field in result, got:\n%s", result)
	}

	// deve injetar após a linha de status
	lines := strings.Split(result, "\n")
	statusIdx := -1
	issueIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "| Status:") {
			statusIdx = i
		}
		if strings.Contains(l, "| linear_issue:") {
			issueIdx = i
		}
	}
	if statusIdx < 0 {
		t.Error("status line not found")
	}
	if issueIdx < 0 {
		t.Error("injected field not found")
	}
	if issueIdx != statusIdx+1 {
		t.Errorf("expected injected field at line %d (after status at %d), got at %d", statusIdx+1, statusIdx, issueIdx)
	}
}

func TestInjectField_UpdatesExisting(t *testing.T) {
	text := `# REQ: Feature

> Date: 2026-01-01 | Status: Open
| linear_issue: OLD-1

## Motivation
text here
`
	result := injectField(text, "linear_issue", "NEW-99")
	if strings.Contains(result, "OLD-1") {
		t.Error("old value should be replaced")
	}
	if !strings.Contains(result, "| linear_issue: NEW-99") {
		t.Errorf("new value not found:\n%s", result)
	}
}

func TestExtractMotivation(t *testing.T) {
	text := `# REQ: Feature

> Date: 2026-01-01 | Status: Open

## Motivation
This is the motivation.
Second line.

## Acceptance Criteria
- [ ]
`
	got := extractMotivation(text)
	if !strings.Contains(got, "This is the motivation.") {
		t.Errorf("expected motivation text, got %q", got)
	}
	if strings.Contains(got, "Acceptance Criteria") {
		t.Error("should not include content from next section")
	}
}

// ─── AC2 — req_dir não-padrão ─────────────────────────────────────────────────

// TestSyncToProvider_NonDefaultREQDir_AC2 afirma que syncToProvider honra um req_dir
// configurado diferente do padrão ("docs/requisições"), e que o cenário é discriminante:
// o glob literal antigo retorna 0 arquivos, enquanto o resolvedor retorna o arquivo real.
// Reprova se: (a) create não for chamado com req_dir configurado, ou (b) o glob literal
// retornar > 0 arquivos (cenário não discrimina).
func TestSyncToProvider_NonDefaultREQDir_AC2(t *testing.T) {
	dir := chdirTempWithReset(t)
	writeYAMLSync(t, dir, "req_dir: docs/requisições\n")

	// Cria REQ no diretório configurado.
	setupREQ(t, dir, filepath.Join("docs", "requisições"), "REQ-2026-01-01-test.md", `# REQ: Test Feature

> Date: 2026-01-01 | Status: Open

## Motivation
Some motivation.
`)

	// Contra-braço: glob literal DEVE retornar 0 arquivos para que o cenário discrimine.
	// Se retornar > 0, o teste não prova que o bug foi corrigido — pode ser qualquer resolvedor.
	oldFiles, _ := filepath.Glob("docs/req/*.md")
	if len(oldFiles) != 0 {
		t.Fatalf("contra-braço AC2: esperava 0 arquivos no glob literal docs/req/*.md, obteve %d"+
			" — o cenário não discrimina (arquivos inesperados em docs/req/)", len(oldFiles))
	}

	called := false
	_, err := syncToProvider(func(_, _ string) (string, error) {
		called = true
		return "ENG-1", nil
	}, "linear_issue")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("AC2: create deveria ter sido chamado — REQ em req_dir não-padrão com Status: Open não foi encontrada")
	}
}

// ─── AC3 — by_agent layout ────────────────────────────────────────────────────

// TestSyncToProvider_ByAgent_AC3 afirma que syncToProvider enumera REQs em
// req_dir/<agente>/*.md quando roadmap_namespacing: by_agent está configurado.
// Reprova se create não for chamado para a REQ sob o subdiretório de agente.
func TestSyncToProvider_ByAgent_AC3(t *testing.T) {
	dir := chdirTempWithReset(t)
	writeYAMLSync(t, dir, "req_dir: docs/req\nroadmap_namespacing: by_agent\nagents:\n  - apolo\n")

	setupREQ(t, dir, filepath.Join("docs", "req", "apolo"), "REQ-2026-01-01-agent-req.md",
		`# REQ: Agent Feature

> Date: 2026-01-01 | Status: Open

## Motivation
Agent motivation.
`)

	called := false
	_, err := syncToProvider(func(_, _ string) (string, error) {
		called = true
		return "ENG-2", nil
	}, "linear_issue")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("AC3: create deveria ter sido chamado — REQ em subdiretório de agente (by_agent) não foi encontrada")
	}
}

// ─── AC6 — zero REQs → recusa nomeada ────────────────────────────────────────

// TestSyncToProvider_NoREQsFound_AC6 afirma que syncToProvider retorna ErrNoREQsFound
// (nunca lista vazia silenciosa) quando req_dir existe mas não contém REQs.
// Afirma também que REQDir é verbatim (não expandido para caminho absoluto).
// Reprova se: (a) err for nil, (b) err não for *ErrNoREQsFound, ou (c) REQDir iniciar
// com "/" (indicando expansão para caminho absoluto, que vazaria em logs de CI).
func TestSyncToProvider_NoREQsFound_AC6(t *testing.T) {
	dir := chdirTempWithReset(t)
	writeYAMLSync(t, dir, "req_dir: docs/requisições\n")
	// Nenhum arquivo em docs/requisições — diretório não criado.

	_, err := syncToProvider(func(_, _ string) (string, error) {
		t.Fatal("AC6: create não deve ser chamado quando não há REQs")
		return "", nil
	}, "linear_issue")

	if err == nil {
		t.Fatal("AC6: esperava ErrNoREQsFound, obteve nil")
	}
	var noREQ *ErrNoREQsFound
	if !errors.As(err, &noREQ) {
		t.Fatalf("AC6: esperava *ErrNoREQsFound, obteve %T: %v", err, err)
	}
	// REQDir deve ser verbatim — nunca expandido para caminho absoluto.
	if strings.HasPrefix(noREQ.REQDir, "/") {
		t.Errorf("AC6: REQDir deve ser verbatim (não absoluto), obteve %q", noREQ.REQDir)
	}
	if noREQ.REQDir != "docs/requisições" {
		t.Errorf("AC6: REQDir verbatim deve ser %q, obteve %q", "docs/requisições", noREQ.REQDir)
	}
}

// ─── AC7 — contenção de caminho via EvalSymlinks ──────────────────────────────

// TestSyncToProvider_REQDirParentTraversal_AC7 afirma que req_dir com traversal (..)
// é recusado com erro nomeado. Distingue contenção física de verificação lexical porque
// "../algo" aponta para fora da árvore independentemente de symlinks.
// Reprova se: create for chamado, ou err for nil.
func TestSyncToProvider_REQDirParentTraversal_AC7(t *testing.T) {
	dir := chdirTempWithReset(t)
	writeYAMLSync(t, dir, "req_dir: ../outside-req\n")

	_, err := syncToProvider(func(_, _ string) (string, error) {
		t.Fatal("AC7 (traversal): create não deve ser chamado para req_dir fora da árvore")
		return "", nil
	}, "linear_issue")

	if err == nil {
		t.Error("AC7 (traversal): esperava erro de contenção para req_dir: ../outside-req")
		return
	}
	if !strings.Contains(err.Error(), "outside project root") {
		t.Errorf("AC7 (traversal): esperava 'outside project root' no erro, obteve: %v", err)
	}
}

// TestSyncToProvider_REQDirAbsolutePath_AC7 afirma que req_dir com caminho absoluto
// fora do CWD é recusado. O caminho absoluto do TempDir garante que é de fora da árvore.
// Reprova se: create for chamado, ou err for nil.
func TestSyncToProvider_REQDirAbsolutePath_AC7(t *testing.T) {
	dir := chdirTempWithReset(t)
	outside := t.TempDir() // diretório diferente do CWD
	writeYAMLSync(t, dir, fmt.Sprintf("req_dir: %s\n", outside))

	_, err := syncToProvider(func(_, _ string) (string, error) {
		t.Fatal("AC7 (absoluto): create não deve ser chamado para req_dir absoluto fora do CWD")
		return "", nil
	}, "linear_issue")

	if err == nil {
		t.Errorf("AC7 (absoluto): esperava erro de contenção para req_dir: %s", outside)
		return
	}
	if !strings.Contains(err.Error(), "outside project root") {
		t.Errorf("AC7 (absoluto): esperava 'outside project root' no erro, obteve: %v", err)
	}
}

// TestSyncToProvider_REQDirSymlink_AC7 afirma que um symlink em docs/req apontando para
// fora do CWD é recusado — distingue contenção física (EvalSymlinks) de verificação lexical
// (filepath.Rel). Um symlink docs/linked-req → /tmp/outside passaria numa verificação lexical
// (docs/linked-req está dentro do CWD lexicamente) mas deve ser recusado fisicamente.
// Reprova se: create for chamado, err for nil, ou a mensagem não contiver "outside project root".
func TestSyncToProvider_REQDirSymlink_AC7(t *testing.T) {
	dir := chdirTempWithReset(t)
	outside := t.TempDir() // diretório fora do CWD

	// Cria symlink docs/linked-req → outside (fora do CWD)
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(docsDir, "linked-req")
	// symlinkOrSkip: guarda de capacidade — distingue "sem privilégio" (skip)
	// de qualquer outro erro (t.Fatalf). Garante que o AC7 executa (não pula)
	// em macOS/Linux, onde symlink não exige privilégio especial.
	if !symlinkOrSkip(t, outside, symlinkPath) {
		return
	}

	writeYAMLSync(t, dir, "req_dir: docs/linked-req\n")

	_, err := syncToProvider(func(_, _ string) (string, error) {
		t.Fatal("AC7 (symlink): create não deve ser chamado para req_dir que é symlink fora do CWD")
		return "", nil
	}, "linear_issue")

	if err == nil {
		t.Error("AC7 (symlink): esperava erro de contenção — symlink fora do CWD deve ser recusado")
		return
	}
	if !strings.Contains(err.Error(), "outside project root") {
		t.Errorf("AC7 (symlink): esperava 'outside project root' no erro, obteve: %v", err)
	}
}
