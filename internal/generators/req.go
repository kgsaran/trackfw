package generators

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	if err := os.MkdirAll("docs/req", 0755); err != nil {
		return err
	}

	slug := toSlug(content.Title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("docs/req/REQ-%s-%s.md", date, slug)

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
	statusLine := fmt.Sprintf("> Date: %s | Status: Open", date)
	if len(content.DependsOnADRs) > 0 {
		statusLine = fmt.Sprintf("> Date: %s | Status: Open | Blocked by ADRs: %d", date, len(content.DependsOnADRs))
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

	body := fmt.Sprintf(`# REQ: %s

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
`, content.Title, statusLine, motivationSection, criteriaSection, linkedADRSection, blockedSection, linkedRoadmapSection)

	if err := os.WriteFile(filename, []byte(body), 0644); err != nil {
		return fmt.Errorf("writing REQ: %w", err)
	}

	fmt.Printf("created %s\n", filename)
	return nil
}

// ListREQs lista todos os REQs encontrados em dir, imprimindo filename e status.
// Retorna nil se o diretório estiver ausente ou sem arquivos .md.
func ListREQs(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return fmt.Errorf("listing REQs: %w", err)
	}
	if len(matches) == 0 {
		fmt.Printf("No REQs found in %s\n", dir)
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
