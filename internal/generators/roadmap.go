package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// RoadmapContent contém os dados para criação de um roadmap.
type RoadmapContent struct {
	Title   string
	REQPath string
	Body    string
}

// stateDir retorna o caminho do diretório para um estado válido no modo flat, ou "", false se inválido.
func stateDir(state string) (string, bool) {
	cfg := config.Load()
	validStateNames := map[string]bool{
		"backlog": true, "wip": true, "blocked": true, "done": true, "abandoned": true,
	}
	if !validStateNames[state] {
		return "", false
	}
	return cfg.RoadmapDir + "/" + state, true
}

// agentStateDir retorna o diretório para um agente+estado em modo by_agent.
// agent="" usa o primeiro agente configurado (ou "default" se lista vazia).
func agentStateDir(agent, state string) (string, bool) {
	cfg := config.Load()
	validStateNames := map[string]bool{
		"backlog": true, "wip": true, "blocked": true, "done": true, "abandoned": true,
	}
	if !validStateNames[state] {
		return "", false
	}
	if agent == "" {
		if len(cfg.Agents) > 0 {
			agent = cfg.Agents[0]
		} else {
			agent = "default"
		}
	}
	return cfg.RoadmapDir + "/" + agent + "/" + state, true
}

// logPath retorna o caminho do arquivo de log de transições.
func logPath() string {
	return config.Load().RoadmapDir + "/.trackfw-log"
}

// NewRoadmap cria um roadmap com template padrão a partir de um título simples.
func NewRoadmap(title string) error {
	return NewRoadmapFromContent(RoadmapContent{Title: title})
}

// NewRoadmapFromContent cria um roadmap a partir de um RoadmapContent.
// Se Body for preenchido, usa diretamente; caso contrário, gera template padrão.
func NewRoadmapFromContent(content RoadmapContent) error {
	cfg := config.Load()

	var backlogDir string
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		dir, ok := agentStateDir("", "backlog")
		if !ok {
			return fmt.Errorf("cannot resolve backlog dir in by_agent mode")
		}
		backlogDir = dir
	} else {
		backlogDir = cfg.RoadmapDir + "/backlog"
	}

	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		return err
	}

	slug := toSlug(content.Title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s/ROADMAP-%s-%s.md", backlogDir, date, slug)

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
	cfg := config.Load()

	// Validar estado antes de buscar o roadmap (melhor UX)
	validStateNames := map[string]bool{
		"backlog": true, "wip": true, "blocked": true, "done": true, "abandoned": true,
	}
	if !validStateNames[state] {
		return fmt.Errorf("invalid state %q — valid states: backlog, wip, blocked, done, abandoned", state)
	}

	src, err := findRoadmap(name)
	if err != nil {
		return err
	}

	var targetDir string
	var fromState string

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		// em by_agent: src = roadmapDir/agent/state/file → agentDir é a pasta avó
		agentDir := filepath.Dir(filepath.Dir(src))
		agent := filepath.Base(agentDir)
		fromState = filepath.Base(filepath.Dir(src))
		var ok bool
		targetDir, ok = agentStateDir(agent, state)
		if !ok {
			return fmt.Errorf("invalid state %q — valid states: backlog, wip, blocked, done, abandoned", state)
		}
	} else {
		fromState = filepath.Base(filepath.Dir(src))
		var ok bool
		targetDir, ok = stateDir(state)
		if !ok {
			return fmt.Errorf("invalid state %q — valid states: backlog, wip, blocked, done, abandoned", state)
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating target dir: %w", err)
	}

	dst := filepath.Join(targetDir, filepath.Base(src))
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving roadmap: %w", err)
	}

	logBasename := filepath.Base(src)
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agent := filepath.Base(filepath.Dir(filepath.Dir(src)))
		logBasename = agent + "/" + filepath.Base(src)
	}
	appendTransitionLog(logBasename, fromState, state)

	fmt.Printf("✓ moved %s → %s\n", filepath.Base(src), targetDir)
	return nil
}

func findRoadmap(name string) (string, error) {
	cfg := config.Load()
	states := []string{"backlog", "wip", "blocked", "done", "abandoned"}

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			agents = []string{"default"}
		}
		for _, agent := range agents {
			for _, state := range states {
				dir := cfg.RoadmapDir + "/" + agent + "/" + state
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
		}
	} else {
		for _, state := range states {
			dir := cfg.RoadmapDir + "/" + state
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
	}
	return "", fmt.Errorf("roadmap %q not found in any state directory", name)
}

func containsIgnoreCase(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func appendTransitionLog(basename, fromState, toState string) {
	f, err := os.OpenFile(logPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	cfg := config.Load()

	var pattern string
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		// 3 níveis: roadmapDir/agent/state/file
		pattern = filepath.Join(cfg.RoadmapDir, "*", "*", "*"+name+"*.md")
	} else {
		pattern = filepath.Join(cfg.RoadmapDir, "*", "*"+name+"*.md")
	}

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

// ListRoadmaps imprime todos os roadmaps agrupados por estado (e por agente em modo by_agent).
func ListRoadmaps() error {
	cfg := config.Load()
	stateOrder := []string{"wip", "backlog", "blocked", "done", "abandoned"}
	found := false

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			// descobrir subdirs dinamicamente
			entries, err := os.ReadDir(cfg.RoadmapDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						agents = append(agents, e.Name())
					}
				}
			}
		}
		for _, agent := range agents {
			for _, state := range stateOrder {
				dir := cfg.RoadmapDir + "/" + agent + "/" + state
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
				fmt.Printf("[%s/%s]\n", agent, state)
				for _, f := range files {
					fmt.Printf("  %s\n", f)
				}
			}
		}
	} else {
		for _, state := range stateOrder {
			dir := cfg.RoadmapDir + "/" + state
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
	}

	if !found {
		fmt.Println("Nenhum roadmap encontrado. Crie um com 'trackfw roadmap new'.")
	}
	return nil
}
