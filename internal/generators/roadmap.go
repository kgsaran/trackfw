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

	fromState := filepath.Base(filepath.Dir(src))

	dst := filepath.Join(targetDir, filepath.Base(src))
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving roadmap: %w", err)
	}

	appendTransitionLog(filepath.Base(src), fromState, state)

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

const transitionLogPath = "docs/roadmaps/.trackfw-log"

func appendTransitionLog(basename, fromState, toState string) {
	f, err := os.OpenFile(transitionLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf("%s  %-50s  %s → %s\n",
		time.Now().Format("2006-01-02 15:04"),
		basename,
		fromState,
		toState,
	)
	f.WriteString(line)
}

// ShowRoadmap exibe o conteúdo de um roadmap identificado por nome parcial.
func ShowRoadmap(name string) error {
	pattern := filepath.Join("docs", "roadmaps", "*", "*"+name+"*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no roadmap found matching %q", name)
	}
	if len(matches) > 1 {
		fmt.Println("Multiple roadmaps found — be more specific:")
		for _, m := range matches {
			fmt.Printf("  %s\n", m)
		}
		return fmt.Errorf("ambiguous match for %q", name)
	}
	path := matches[0]
	state := filepath.Base(filepath.Dir(path))
	base := filepath.Base(path)
	fmt.Printf("── %s ── [%s] ──────────────────────\n\n", base, strings.ToUpper(state))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	fmt.Printf("Location: %s\n", path)
	return nil
}

// ListRoadmaps imprime todos os roadmaps agrupados por estado.
func ListRoadmaps() error {
	stateOrder := []string{"wip", "backlog", "blocked", "done", "abandoned"}
	found := false

	for _, state := range stateOrder {
		dir := validStates[state]
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var files []string
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
				files = append(files, e.Name())
			}
		}
		if len(files) == 0 {
			continue
		}
		found = true
		fmt.Printf("[%s]\n", state)
		for _, f := range files {
			fmt.Printf("  %s\n", f)
		}
	}

	if !found {
		fmt.Println("Nenhum roadmap encontrado. Crie um com 'trackfw roadmap new'.")
	}
	return nil
}
