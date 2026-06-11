package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var validStates = map[string]string{
	"backlog":   "docs/roadmaps/backlog",
	"wip":       "docs/roadmaps/wip",
	"blocked":   "docs/roadmaps/blocked",
	"done":      "docs/roadmaps/done",
	"abandoned": "docs/roadmaps/abandoned",
}

// RoadmapContent contém os dados para criação de um roadmap.
type RoadmapContent struct {
	Title   string
	REQPath string
	Body    string
}

// NewRoadmap cria um roadmap com template padrão a partir de um título simples.
func NewRoadmap(title string) error {
	return NewRoadmapFromContent(RoadmapContent{Title: title})
}

// NewRoadmapFromContent cria um roadmap a partir de um RoadmapContent.
// Se Body for preenchido, usa diretamente; caso contrário, gera template padrão.
func NewRoadmapFromContent(content RoadmapContent) error {
	if err := os.MkdirAll("docs/roadmaps/backlog", 0755); err != nil {
		return err
	}

	slug := toSlug(content.Title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("docs/roadmaps/backlog/ROADMAP-%s-%s.md", date, slug)

	var body string
	if content.Body != "" {
		body = content.Body
	} else {
		body = fmt.Sprintf(`# Roadmap: %s

> Created: %s | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: %s

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — <title>
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
`, content.Title, date, content.REQPath)
	}

	if err := os.WriteFile(filename, []byte(body), 0644); err != nil {
		return fmt.Errorf("writing roadmap: %w", err)
	}

	fmt.Printf("✓ created %s\n", filename)
	return nil
}

func MoveRoadmap(name, state string) error {
	targetDir, ok := validStates[state]
	if !ok {
		return fmt.Errorf("invalid state %q — valid states: backlog, wip, blocked, done, abandoned", state)
	}

	src, err := findRoadmap(name)
	if err != nil {
		return err
	}

	dst := filepath.Join(targetDir, filepath.Base(src))
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving roadmap: %w", err)
	}

	fmt.Printf("✓ moved %s → %s\n", filepath.Base(src), targetDir)
	return nil
}

func findRoadmap(name string) (string, error) {
	for _, dir := range validStates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if containsIgnoreCase(e.Name(), name) {
				return filepath.Join(dir, e.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("roadmap %q not found in any state directory", name)
}

func containsIgnoreCase(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
