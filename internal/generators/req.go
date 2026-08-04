package generators

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// REQContent contém os campos de uma REQ a ser gerada.
type REQContent struct {
	Title         string
	Motivation    string
	Criteria      string
	LinkedADR     string
	LinkedRoadmap string
	DependsOnADRs []string // basenames de ADRs Draft vinculados
}

// NewREQ gera um arquivo REQ em docs/req/ com base no conteúdo fornecido.
// Campos preenchidos são inseridos diretamente; campos vazios mantêm o placeholder original.
func NewREQ(content REQContent) error {
	cfg := config.Load()
	reqDir := cfg.REQDir
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		return err
	}

	slug := toSlug(content.Title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s/REQ-%s-%s.md", reqDir, date, slug)

	motivationSection := "<!-- Why is this requirement needed? What problem does it solve? -->"
	if content.Motivation != "" {
		motivationSection = content.Motivation
	}

	criteriaSection := "- [ ]\n- [ ]"
	if content.Criteria != "" {
		criteriaSection = content.Criteria
	}

	linkedADRSection := ""
	if content.LinkedADR != "" {
		linkedADRSection = content.LinkedADR
	}

	linkedRoadmapSection := ""
	if content.LinkedRoadmap != "" {
		linkedRoadmapSection = content.LinkedRoadmap
	}

	// Linha de status — inclui contador de ADRs bloqueantes quando presente
	statusLine := fmt.Sprintf("> Date: %s | Status: Open\n| Linear Issue: \n| Jira Issue: ", date)
	if len(content.DependsOnADRs) > 0 {
		statusLine = fmt.Sprintf("> Date: %s | Status: Open | Blocked by ADRs: %d\n| Linear Issue: \n| Jira Issue: ", date, len(content.DependsOnADRs))
	}

	// Seção "Blocked by ADRs"
	var blockedSection string
	if len(content.DependsOnADRs) == 0 {
		blockedSection = "<!-- none -->"
	} else {
		var sb strings.Builder
		sb.WriteString("<!-- ADRs in Draft status that must be Accepted before a roadmap can be created -->")
		for _, adr := range content.DependsOnADRs {
			sb.WriteString("\n- ")
			sb.WriteString(adr)
			sb.WriteString(" (Draft)")
		}
		blockedSection = sb.String()
	}

	body := fmt.Sprintf(`---
status: Open
date: %s
author: ""
adr: ""
roadmap: ""
---

# REQ: %s

%s

## Motivation
%s

## Acceptance Criteria
%s

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: %s

## Blocked by ADRs
%s

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: %s
`, date, content.Title, statusLine, motivationSection, criteriaSection, linkedADRSection, blockedSection, linkedRoadmapSection)

	if err := os.WriteFile(filename, []byte(body), 0644); err != nil {
		return fmt.Errorf("writing REQ: %w", err)
	}

	fmt.Printf("created %s\n", filename)
	return nil
}

// listREQFiles descobre todos os arquivos .md de REQ nos 3 layouts suportados,
// espelhando o algoritmo de referência usado por listREQs/list_req_files (Node/Python):
//  1. REQDir/*.md (flat legado)
//  2. REQDir/<estado>/*.md para cada estado válido (por-estado, sem agente)
//  3. Se RoadmapNamespacing == by_agent: REQDir/<agente>/<estado>/*.md para cada agente
//     (cfg.Agents, ou subpastas de primeiro nível de REQDir se vazio) × cada estado
//
// Os três conjuntos não são mutuamente exclusivos — todos são concatenados.
func listREQFiles(cfg config.ProjectConfig) []string {
	reqDir := cfg.REQDir
	if reqDir == "" {
		return nil
	}

	var files []string

	// 1. Flat legado.
	flatMatches, _ := filepath.Glob(filepath.Join(reqDir, "*.md"))
	files = append(files, flatMatches...)

	// 2. Por-estado, sem agente.
	for _, state := range roadmapStateOrder {
		matches, _ := filepath.Glob(filepath.Join(reqDir, state, "*.md"))
		files = append(files, matches...)
	}

	// 3. by_agent.
	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents := cfg.Agents
		if len(agents) == 0 {
			entries, err := os.ReadDir(reqDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						agents = append(agents, e.Name())
					}
				}
			}
		}
		for _, agent := range agents {
			for _, state := range roadmapStateOrder {
				matches, _ := filepath.Glob(filepath.Join(reqDir, agent, state, "*.md"))
				files = append(files, matches...)
			}
		}
	}

	return files
}

// ListREQs lista todos os REQs encontrados em cfg.REQDir (nos 3 layouts suportados),
// imprimindo filename e status. Retorna nil se não houver arquivos.
func ListREQs() error {
	cfg := config.Load()
	matches := listREQFiles(cfg)
	if len(matches) == 0 {
		fmt.Printf("No REQs found in %s\n", cfg.REQDir)
		return nil
	}

	for _, path := range matches {
		filename := filepath.Base(path)
		title, status := parseREQMeta(path)
		if title == "" {
			title = filename
		}
		fmt.Printf("%-60s %s\n", filename, status)
		_ = title
	}
	return nil
}

// parseREQMeta extrai título e status de um arquivo REQ markdown.
func parseREQMeta(path string) (title, status string) {
	f, err := os.Open(path)
	if err != nil {
		return "", "unknown"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	status = "unknown"
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# REQ: ") {
			title = strings.TrimPrefix(line, "# REQ: ")
		}
		if strings.Contains(line, "| Status: ") {
			idx := strings.Index(line, "| Status: ")
			if idx >= 0 {
				rest := line[idx+len("| Status: "):]
				// O status termina no próximo " |" ou no final da linha
				if pipeIdx := strings.Index(rest, " |"); pipeIdx >= 0 {
					rest = rest[:pipeIdx]
				}
				rest = strings.TrimRight(rest, " >|")
				status = strings.TrimSpace(rest)
			}
		}
	}
	return title, status
}

// rewriteREQStatus rewrites only the "status:" field in the frontmatter block
// and the first "| Status: <value>" marker before the first section heading.
// Other bytes, keys and body occurrences are preserved.
func rewriteREQStatus(source []byte, status string) ([]byte, bool) {
	s := string(source)
	if !strings.HasPrefix(s, "---\n") {
		return source, false
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return source, false
	}

	frontmatter := s[4 : 4+end]
	rest := s[4+end:]
	changed := false

	lines := strings.Split(frontmatter, "\n")
	for i, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "status" {
			continue
		}
		trimmedValue := strings.TrimSpace(value)
		quoted := len(trimmedValue) >= 2 && strings.HasPrefix(trimmedValue, `"`) && strings.HasSuffix(trimmedValue, `"`)
		newLine := key + ": " + status
		if quoted {
			newLine = key + ": \"" + status + "\""
		}
		if lines[i] != newLine {
			lines[i] = newLine
			changed = true
		}
		break
	}

	if len(rest) > 4 {
		body := rest[4:]
		bodyLines := strings.Split(body, "\n")
		const marker = "| Status: "
		for i, bline := range bodyLines {
			if strings.HasPrefix(strings.TrimSpace(bline), "## ") {
				break
			}
			idx := strings.Index(bline, marker)
			if idx < 0 {
				continue
			}
			prefix := bline[:idx+len(marker)]
			after := bline[idx+len(marker):]
			suffix := ""
			if pipeIdx := strings.Index(after, " |"); pipeIdx >= 0 {
				suffix = after[pipeIdx:]
			}
			newLine := prefix + status + suffix
			if bodyLines[i] != newLine {
				bodyLines[i] = newLine
				changed = true
				rest = "\n---" + strings.Join(bodyLines, "\n")
			}
			break
		}
	}

	if !changed {
		return source, false
	}
	return []byte("---\n" + strings.Join(lines, "\n") + rest), true
}

// MoveREQ atualiza o status de uma REQ e, se ela já estiver organizada em uma
// subpasta de estado reconhecida (por-estado ou by_agent), move fisicamente o
// arquivo para a subpasta do novo estado. REQs soltas em cfg.REQDir permanecem
// in-place (comportamento legado, sem migração forçada) — ver "Move condicional"
// no algoritmo de referência do roadmap.
func MoveREQ(name, status string) error {
	if strings.TrimSpace(status) == "" {
		return fmt.Errorf("status is required")
	}
	cfg := config.Load()
	path, err := findREQ(name, cfg)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading REQ: %w", err)
	}
	updated, changed := rewriteREQStatus(raw, status)
	if !changed {
		return fmt.Errorf("REQ %q has no frontmatter status/header Status to update", filepath.Base(path))
	}

	reqDirClean := filepath.Clean(cfg.REQDir)
	parentDir := filepath.Dir(path)
	grandparentDir := filepath.Dir(parentDir)

	// Modo in-place: REQ solta em cfg.REQDir.
	if parentDir == reqDirClean {
		if err := os.WriteFile(path, updated, 0644); err != nil {
			return fmt.Errorf("writing REQ: %w", err)
		}
		fmt.Printf("✓ updated %s status → %s\n", filepath.Base(path), status)
		return nil
	}

	var targetDir string
	var fromState string
	var logBasename string

	if !roadmapValidStateNames[status] {
		return fmt.Errorf("invalid state %q — valid states: %s", status, roadmapValidStatesMessage)
	}

	switch {
	case grandparentDir == reqDirClean && roadmapValidStateNames[filepath.Base(parentDir)]:
		// Layout por-estado.
		fromState = filepath.Base(parentDir)
		targetDir = filepath.Join(cfg.REQDir, status)
		logBasename = filepath.Base(path)
	case roadmapValidStateNames[filepath.Base(parentDir)] && filepath.Dir(grandparentDir) == reqDirClean:
		// Layout by_agent.
		fromState = filepath.Base(parentDir)
		agent := filepath.Base(grandparentDir)
		targetDir = filepath.Join(cfg.REQDir, agent, status)
		logBasename = agent + "/" + filepath.Base(path)
	default:
		// Layout não reconhecido — fallback in-place, sem mover.
		if err := os.WriteFile(path, updated, 0644); err != nil {
			return fmt.Errorf("writing REQ: %w", err)
		}
		fmt.Printf("✓ updated %s status → %s\n", filepath.Base(path), status)
		return nil
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating target dir: %w", err)
	}
	dst := filepath.Join(targetDir, filepath.Base(path))
	if err := os.WriteFile(dst, updated, 0644); err != nil {
		return fmt.Errorf("writing REQ: %w", err)
	}
	if dst != path {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing old REQ: %w", err)
		}
	}
	appendREQTransitionLog(logBasename, fromState, status)
	fmt.Printf("✓ moved %s → %s\n", filepath.Base(path), targetDir)
	return nil
}

// findREQ busca por uma REQ cujo basename contenha name (case-insensitive), varrendo
// os 3 layouts suportados via listREQFiles (ordem: flat → por-estado → by_agent).
func findREQ(name string, cfg config.ProjectConfig) (string, error) {
	for _, path := range listREQFiles(cfg) {
		if containsIgnoreCase(filepath.Base(path), name) {
			return path, nil
		}
	}
	return "", fmt.Errorf("REQ %q not found in %s", name, cfg.REQDir)
}

// appendREQTransitionLog registra a transição de estado de uma REQ em cfg.REQDir/.trackfw-log,
// mesmo formato de appendTransitionLog (roadmap.go), em arquivo de log separado.
func appendREQTransitionLog(basename, fromState, toState string) {
	cfg := config.Load()
	logFile := filepath.Join(cfg.REQDir, ".trackfw-log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
