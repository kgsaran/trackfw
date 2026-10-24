package generators

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// workingContextLog holds a single entry from docs/agents-working-context/.
type workingContextLog struct {
	Name    string // filename (e.g. 20261022T100000Z-apolo-ml1a.md)
	Heading string // first "## …" line, empty if absent
}

// recentWorkingContextEntries returns up to n entries from dir, sorted by
// filename in descending order (newest first). Only regular .md files that are
// not symlinks are included; non-.md files and symlinks are skipped silently.
// If dir does not exist or is empty, an empty slice is returned without error.
func recentWorkingContextEntries(dir string, n int) []workingContextLog {
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		return nil
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// Collect regular .md entries.
	var names []string
	for _, de := range des {
		if de.Type()&os.ModeSymlink != 0 {
			continue // skip symlinks
		}
		if !de.Type().IsRegular() {
			continue
		}
		if !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		names = append(names, de.Name())
	}

	// Sort descending (newest first by ISO timestamp prefix).
	sort.Slice(names, func(i, j int) bool { return names[i] > names[j] })

	if len(names) > n {
		names = names[:n]
	}

	result := make([]workingContextLog, 0, len(names))
	for _, name := range names {
		heading := extractFirstHeading(filepath.Join(dir, name))
		result = append(result, workingContextLog{Name: name, Heading: heading})
	}
	return result
}

// extractFirstHeading reads a file and returns the text of the first "## …" line,
// or empty string if none is found. Uses a buffered scanner for efficiency.
func extractFirstHeading(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			return line
		}
	}
	return ""
}

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

// contextRoadmapStateOrder é a ordem de varredura dos estados de roadmap para o
// `trackfw context`. A ordem é semântica: wip aparece primeiro para que o trabalho
// mais recentemente ativo apareça antes na saída.
// O conjunto deve ser idêntico a config.RoadmapStates() — veja
// TestContextRoadmapStateOrder_SetEqualsCanonical em roadmap_order_states_test.go.
// "analyzing" foi acrescentado em ML-6D; antes, roadmaps em analyzing/ eram omitidos silenciosamente.
var contextRoadmapStateOrder = []string{"wip", "analyzing", "backlog", "blocked", "done", "abandoned"}

// GetContext coleta ADRs, REQs e ROADMAPs do projeto, executa validate e imprime o contexto.
// format: "md" (default) ou "json".
func GetContext(format string) error {
	cfg := config.Load()

	// ADRs — pelo PONTO ÚNICO de leitura (ADR-2026-09-29, D3): ResolveADRFiles varre todos os
	// diretórios de cfg.ADRDirs recursivamente (layout plano + subpastas de estado) e deduplicaa
	// por caminho absoluto (evita double-count com adr_dirs aninhados).
	// Antes: os.ReadDir(adrDir) — raiz apenas → ADRs (0) em layout com subpastas.
	var adrs []ContextEntry
	for _, full := range validator.ResolveADRFiles(cfg) {
		content, _ := os.ReadFile(full)
		status := extractFrontmatterField(string(content), "status")
		if status == "" {
			status = extractInlineStatus(string(content))
		}
		adrs = append(adrs, ContextEntry{Type: "ADR", File: filepath.Base(full), Status: status})
	}

	// REQs — pelo PONTO ÚNICO de leitura (ADR-2026-09-03, D3/D4). Antes, o context montava a
	// própria árvore (flat, ou <agente>/<estado>/ em by_agent) e não enxergava o layout canônico
	// req_dir/<agente>/*.md — a mesma divergência escritor/leitor da REQ-2026-08-30.
	var reqs []ContextEntry
	reqFilesCtx, err := validator.ResolveREQFiles(cfg)
	if err != nil {
		return fmt.Errorf("context: resolve REQ files: %w", err)
	}
	for _, full := range reqFilesCtx {
		content, _ := os.ReadFile(full)
		status := extractFrontmatterField(string(content), "status")
		if status == "" {
			status = extractInlineStatus(string(content))
		}
		reqs = append(reqs, ContextEntry{Type: "REQ", File: filepath.Base(full), Status: status})
	}

	var roadmaps []ContextEntry
	states := contextRoadmapStateOrder
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := validator.ResolveAgentNamespaces(cfg, cfg.RoadmapDir)
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

	// Recent working context entries (ADR-2026-10-22 D2).
	wctxDir := filepath.Join("docs", "agents-working-context")
	entries := recentWorkingContextEntries(wctxDir, 10)
	fmt.Printf("\n## Recent working context (last %d entries)\n", len(entries))
	if len(entries) == 0 {
		fmt.Printf("- (no entries in %s)\n", wctxDir)
	} else {
		for _, e := range entries {
			if e.Heading != "" {
				fmt.Printf("- %s — %s\n", e.Name, e.Heading)
			} else {
				fmt.Printf("- %s\n", e.Name)
			}
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
