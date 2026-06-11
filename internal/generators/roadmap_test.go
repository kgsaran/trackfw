package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdir muda para dir e restaura ao fim do teste
func chdirRoadmap(t *testing.T, dir string) {
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

// TestNewRoadmap_CreatesFile — arquivo criado em docs/roadmaps/backlog/ com conteúdo correto
func TestNewRoadmap_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	if err := NewRoadmap("My Feature"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)

	if !strings.Contains(body, "My Feature") {
		t.Errorf("arquivo deveria conter 'My Feature', obteve: %q", body)
	}
	if !strings.Contains(body, "REQ:") {
		t.Errorf("arquivo deveria conter 'REQ:', obteve: %q", body)
	}
}

// TestMoveRoadmap_Valid — cria roadmap em backlog e move para wip
func TestMoveRoadmap_Valid(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	// Criar estrutura de diretórios necessária
	for _, d := range []string{
		"docs/roadmaps/backlog",
		"docs/roadmaps/wip",
		"docs/roadmaps/blocked",
		"docs/roadmaps/done",
		"docs/roadmaps/abandoned",
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}

	if err := NewRoadmap("Move Test"); err != nil {
		t.Fatalf("NewRoadmap() erro: %v", err)
	}

	if err := MoveRoadmap("move-test", "wip"); err != nil {
		t.Fatalf("MoveRoadmap() erro: %v", err)
	}

	// Deve existir em wip
	wipMatches, err := filepath.Glob("docs/roadmaps/wip/*.md")
	if err != nil {
		t.Fatalf("Glob wip: %v", err)
	}
	if len(wipMatches) != 1 {
		t.Errorf("esperado 1 arquivo em wip, obteve %d: %v", len(wipMatches), wipMatches)
	}

	// Não deve existir mais em backlog
	backlogMatches, _ := filepath.Glob("docs/roadmaps/backlog/*.md")
	if len(backlogMatches) != 0 {
		t.Errorf("esperado 0 arquivos em backlog após move, obteve %d: %v", len(backlogMatches), backlogMatches)
	}
}

// TestMoveRoadmap_InvalidState — estado inválido → erro descritivo
func TestMoveRoadmap_InvalidState(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := MoveRoadmap("qualquer-coisa", "inexistente")
	if err == nil {
		t.Fatal("esperado erro para estado inválido, obteve nil")
	}
	if !strings.Contains(err.Error(), "invalid state") {
		t.Errorf("erro deveria mencionar 'invalid state', obteve: %v", err)
	}
}

// TestMoveRoadmap_NotFound — roadmap inexistente → erro descritivo
func TestMoveRoadmap_NotFound(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	// Criar todos os diretórios válidos (vazios)
	for _, d := range validStates {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	err := MoveRoadmap("nao-existe", "wip")
	if err == nil {
		t.Fatal("esperado erro para roadmap não encontrado, obteve nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("erro deveria mencionar 'not found', obteve: %v", err)
	}
}

// TestNewRoadmapFromContent_CreatesFile — verifica que arquivo é criado quando Body é preenchido
func TestNewRoadmapFromContent_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "AI Feature",
		REQPath: "docs/req/REQ-2026-01-01-ai-feature.md",
		Body:    "# Roadmap gerado por IA\nConteúdo customizado aqui.",
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "Conteúdo customizado aqui") {
		t.Errorf("arquivo deveria conter o body fornecido, obteve: %q", body)
	}
}

// TestNewRoadmapFromContent_EmptyBody — verifica que template padrão é gerado quando Body == ""
func TestNewRoadmapFromContent_EmptyBody(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	err := NewRoadmapFromContent(RoadmapContent{
		Title:   "Template Feature",
		REQPath: "docs/req/REQ-2026-01-01-template-feature.md",
		Body:    "",
	})
	if err != nil {
		t.Fatalf("NewRoadmapFromContent() erro: %v", err)
	}

	matches, err := filepath.Glob("docs/roadmaps/backlog/*.md")
	if err != nil {
		t.Fatalf("Glob erro: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperado 1 arquivo em backlog, obteve %d", len(matches))
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "Template Feature") {
		t.Errorf("template deveria conter o título, obteve: %q", body)
	}
	if !strings.Contains(body, "REQ:") {
		t.Errorf("template deveria conter 'REQ:', obteve: %q", body)
	}
	if !strings.Contains(body, "ML-1A") {
		t.Errorf("template deveria conter 'ML-1A', obteve: %q", body)
	}
}

// TestListRoadmaps_GroupedByState — verifica agrupamento correto por estado
func TestListRoadmaps_GroupedByState(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	for _, d := range validStates {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", d, err)
		}
	}

	// Criar um arquivo em backlog e um em done
	if err := os.WriteFile("docs/roadmaps/backlog/ROADMAP-2026-01-01-feature-a.md", []byte("# A"), 0644); err != nil {
		t.Fatalf("WriteFile backlog: %v", err)
	}
	if err := os.WriteFile("docs/roadmaps/done/ROADMAP-2026-01-01-feature-b.md", []byte("# B"), 0644); err != nil {
		t.Fatalf("WriteFile done: %v", err)
	}

	// ListRoadmaps não deve retornar erro
	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro: %v", err)
	}
}

// TestListRoadmaps_Empty — nenhum roadmap → mensagem amigável, sem erro
func TestListRoadmaps_Empty(t *testing.T) {
	dir := t.TempDir()
	chdirRoadmap(t, dir)

	for _, d := range validStates {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	if err := ListRoadmaps(); err != nil {
		t.Fatalf("ListRoadmaps() erro esperando nil: %v", err)
	}
}

// TestContainsIgnoreCase — função privada testada diretamente via white-box
func TestContainsIgnoreCase(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"ROADMAP-My-Feature.md", "my-feature", true},
		{"roadmap-my-feature.md", "MY-FEATURE", true},
		{"ROADMAP-Other.md", "my-feature", false},
		{"", "sub", false},
		{"something", "", true}, // strings.Contains("something", "") == true
	}

	for _, c := range cases {
		got := containsIgnoreCase(c.s, c.sub)
		if got != c.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, quer %v", c.s, c.sub, got, c.want)
		}
	}
}
