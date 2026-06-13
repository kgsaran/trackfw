package plugins

import (
	"testing"
)

const sampleRegistry = `plugins:
  - name: trackfw-go-advanced
    repo: kgsaran/trackfw-go-advanced
    description: "Advanced Go generators"
    tags: [go, generators]
  - name: trackfw-java-spring
    repo: kgsaran/trackfw-java-spring
    description: Spring Boot scaffold for trackfw
    tags: [java, spring, backend]
`

func TestParseRegistryYAML_Empty(t *testing.T) {
	entries := parseRegistryYAML("")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseRegistryYAML_OneEntry(t *testing.T) {
	yaml := `plugins:
  - name: trackfw-go-advanced
    repo: kgsaran/trackfw-go-advanced
    description: "Advanced Go generators"
    tags: [go, generators]
`
	entries := parseRegistryYAML(yaml)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Name != "trackfw-go-advanced" {
		t.Errorf("expected name trackfw-go-advanced, got %q", e.Name)
	}
	if e.Repo != "kgsaran/trackfw-go-advanced" {
		t.Errorf("expected repo kgsaran/trackfw-go-advanced, got %q", e.Repo)
	}
	if e.Description != "Advanced Go generators" {
		t.Errorf("expected description 'Advanced Go generators', got %q", e.Description)
	}
	if len(e.Tags) != 2 || e.Tags[0] != "go" || e.Tags[1] != "generators" {
		t.Errorf("expected tags [go generators], got %v", e.Tags)
	}
}

func TestMatchesKeyword_Name(t *testing.T) {
	e := RegistryEntry{Name: "trackfw-go-advanced", Repo: "kgsaran/trackfw-go-advanced", Description: "desc", Tags: []string{"go"}}
	if !matchesKeyword(e, "go-advanced") {
		t.Error("expected match by name")
	}
}

func TestMatchesKeyword_Tag(t *testing.T) {
	e := RegistryEntry{Name: "trackfw-java-spring", Repo: "kgsaran/trackfw-java-spring", Description: "Spring Boot scaffold", Tags: []string{"java", "spring", "backend"}}
	if !matchesKeyword(e, "spring") {
		t.Error("expected match by tag")
	}
}

func TestMatchesKeyword_NoMatch(t *testing.T) {
	e := RegistryEntry{Name: "trackfw-go-advanced", Repo: "kgsaran/trackfw-go-advanced", Description: "Advanced Go generators", Tags: []string{"go", "generators"}}
	if matchesKeyword(e, "python") {
		t.Error("expected no match for 'python'")
	}
}

func TestResolveRepo_WithSlash(t *testing.T) {
	// Entrada com "/" deve ser retornada como está, sem chamada de rede
	result, err := ResolveRepo("user/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "user/repo" {
		t.Errorf("expected 'user/repo', got %q", result)
	}
}
