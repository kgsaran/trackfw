package generators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// ContextEntry representa um artefato de governança (ADR, REQ ou ROADMAP) com metadados extraídos.
type ContextEntry struct {
	Type   string `json:"type"`
	File   string `json:"file"`
	Status string `json:"status"`
	State  string `json:"state,omitempty"` // somente para ROADMAPs (estado kanban)
}

// GovernanceContext agrupa todas as entradas de governança e metadados de saúde.
type GovernanceContext struct {
	Score      int            `json:"score"`
	Violations []string       `json:"violations"`
	Warnings   []string       `json:"warnings"`
	ADRs       []ContextEntry `json:"adrs"`
	REQs       []ContextEntry `json:"reqs"`
	Roadmaps   []ContextEntry `json:"roadmaps"`
}

// GetContext coleta ADRs, REQs e ROADMAPs do projeto, executa validate e imprime o contexto.
// format: "md" (default) ou "json".
func GetContext(format string) error {
	cfg := config.Load()

	var adrs []ContextEntry
	for _, adrDir := range cfg.ADRDirs {
		entries, err := os.ReadDir(adrDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			content, _ := os.ReadFile(filepath.Join(adrDir, e.Name()))
			status := extractFrontmatterField(string(content), "status")
			if status == "" {
				status = extractInlineStatus(string(content))
			}
			adrs = append(adrs, ContextEntry{Type: "ADR", File: e.Name(), Status: status})
		}
	}

	var reqs []ContextEntry
	reqEntries, err := os.ReadDir(cfg.REQDir)
	if err == nil {
		for _, e := range reqEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			content, _ := os.ReadFile(filepath.Join(cfg.REQDir, e.Name()))
			status := extractFrontmatterField(string(content), "status")
			if status == "" {
				status = extractInlineStatus(string(content))
			}
			reqs = append(reqs, ContextEntry{Type: "REQ", File: e.Name(), Status: status})
		}
	}

	var roadmaps []ContextEntry
	states := []string{"wip", "backlog", "blocked", "done", "abandoned"}
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			dirEntries, _ := os.ReadDir(cfg.RoadmapDir)
			for _, de := range dirEntries {
				if de.IsDir() {
					agents = append(agents, de.Name())
				}
			}
		}
		for _, agent := range agents {
			for _, state := range states {
				dir := filepath.Join(cfg.RoadmapDir, agent, state)
				es, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, e := range es {
					if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
						continue
					}
					content, _ := os.ReadFile(filepath.Join(dir, e.Name()))
					status := extractFrontmatterField(string(content), "status")
					if status == "" {
						status = state
					}
					roadmaps = append(roadmaps, ContextEntry{Type: "ROADMAP", File: e.Name(), Status: status, State: state})
				}
			}
		}
	} else {
		for _, state := range states {
			dir := filepath.Join(cfg.RoadmapDir, state)
			es, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range es {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				content, _ := os.ReadFile(filepath.Join(dir, e.Name()))
				status := extractFrontmatterField(string(content), "status")
				if status == "" {
					status = state
				}
				roadmaps = append(roadmaps, ContextEntry{Type: "ROADMAP", File: e.Name(), Status: status, State: state})
			}
		}
	}

	violations, warnings, _ := validator.Validate()

	// Score: 20 pontos por categoria não-vazia + 40 se validate limpo
	score := 0
	if len(adrs) > 0 {
		score += 20
	}
	if len(reqs) > 0 {
		score += 20
	}
	if len(roadmaps) > 0 {
		score += 20
	}
	if len(violations) == 0 {
		score += 40
	}

	ctx := GovernanceContext{
		Score:      score,
		Violations: violations,
		Warnings:   warnings,
		ADRs:       adrs,
		REQs:       reqs,
		Roadmaps:   roadmaps,
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ctx)
	}

	// Formato markdown (default)
	fmt.Printf("# trackfw governance context\n\n")
	fmt.Printf("**Governance score:** %d/100\n\n", score)

	fmt.Printf("## ADRs (%d)\n", len(adrs))
	for _, a := range adrs {
		fmt.Printf("- %s [%s]\n", a.File, a.Status)
	}
	if len(adrs) == 0 {
		fmt.Println("- (none)")
	}

	fmt.Printf("\n## REQs (%d)\n", len(reqs))
	for _, r := range reqs {
		fmt.Printf("- %s [%s]\n", r.File, r.Status)
	}
	if len(reqs) == 0 {
		fmt.Println("- (none)")
	}

	fmt.Printf("\n## Roadmaps (%d)\n", len(roadmaps))
	for _, r := range roadmaps {
		fmt.Printf("- %s [%s]\n", r.File, r.State)
	}
	if len(roadmaps) == 0 {
		fmt.Println("- (none)")
	}

	if len(violations) > 0 {
		fmt.Printf("\n## Violations (%d)\n", len(violations))
		for _, v := range violations {
			fmt.Printf("- %s\n", v)
		}
	}

	if len(warnings) > 0 {
		fmt.Printf("\n## Warnings (%d)\n", len(warnings))
		for _, w := range warnings {
			fmt.Printf("- %s\n", w)
		}
	}

	return nil
}

// extractFrontmatterField extrai o valor de um campo YAML dentro do bloco frontmatter (--- ... ---).
// Retorna string vazia se o campo não for encontrado ou o valor for '""'.
func extractFrontmatterField(content, field string) string {
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	started := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !started {
				started = true
				inFrontmatter = true
				continue
			}
			// segundo "---" fecha o bloco
			break
		}
		if !inFrontmatter {
			break
		}
		key := field + ":"
		if strings.HasPrefix(trimmed, key) {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
			// remover aspas: "" ou ''
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

// extractInlineStatus extrai o status da linha de cabeçalho markdown: "> Date: ... | Status: ..."
func extractInlineStatus(content string) string {
	for _, line := range strings.Split(content, "\n") {
		idx := strings.Index(line, "| Status: ")
		if idx >= 0 {
			rest := line[idx+len("| Status: "):]
			if pipeIdx := strings.Index(rest, " |"); pipeIdx >= 0 {
				rest = rest[:pipeIdx]
			}
			rest = strings.TrimRight(rest, " >|")
			return strings.TrimSpace(rest)
		}
	}
	return "unknown"
}
