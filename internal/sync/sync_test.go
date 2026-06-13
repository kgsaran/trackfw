package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupREQ cria um arquivo REQ temporário no diretório de trabalho.
func setupREQ(t *testing.T, dir, filename, content string) string {
	t.Helper()
	reqDir := filepath.Join(dir, "docs", "req")
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
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	setupREQ(t, dir, "REQ-2026-01-01-draft.md", `# REQ: Draft Requirement

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
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	setupREQ(t, dir, "REQ-2026-01-01-synced.md", `# REQ: Already Synced

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
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	setupREQ(t, dir, "REQ-2026-01-01-open.md", `# REQ: Open Feature

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

func TestReadConfigField(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	// sem arquivo
	if v := readConfigField("linear_api_key"); v != "" {
		t.Errorf("expected empty without trackfw.yaml, got %q", v)
	}

	// com arquivo
	if err := os.WriteFile("trackfw.yaml", []byte("linear_api_key: my-key\nlinear_team_id: team-123\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if v := readConfigField("linear_api_key"); v != "my-key" {
		t.Errorf("expected 'my-key', got %q", v)
	}
	if v := readConfigField("linear_team_id"); v != "team-123" {
		t.Errorf("expected 'team-123', got %q", v)
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
