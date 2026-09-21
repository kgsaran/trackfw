package sync

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/pathguard"
	"github.com/kgsaran/trackfw/internal/validator"
)

// SyncResult representa o resultado do sync de uma REQ.
type SyncResult struct {
	REQPath string
	IssueID string
	Skipped bool  // true se já tinha issue vinculado ou status != Open
	Error   error
}

// ErrNoREQsFound é retornado por SyncToLinear/SyncToJira quando a resolução produz zero arquivos.
// Carrega REQDir verbatim (sem expansão) para que o chamador possa imprimir a mensagem de
// diagnóstico sem vazar caminhos absolutos em logs de CI (AC6).
type ErrNoREQsFound struct {
	REQDir string // valor verbatim de cfg.REQDir, nunca expandido
}

func (e *ErrNoREQsFound) Error() string {
	return fmt.Sprintf("sync: no REQ files found in req_dir %q — check req_dir in trackfw.yaml", e.REQDir)
}

// IsErrNoREQsFound reporta se err é um *ErrNoREQsFound.
func IsErrNoREQsFound(err error) bool {
	var target *ErrNoREQsFound
	return errors.As(err, &target)
}

// SyncToLinear lê todos os REQs Open sem linear_issue, cria issues no Linear e atualiza o frontmatter.
func SyncToLinear() ([]SyncResult, error) {
	client, err := NewLinearClient()
	if err != nil {
		return nil, err
	}
	return syncToProvider(func(title, desc string) (string, error) {
		return client.CreateIssue(title, desc)
	}, "linear_issue")
}

// SyncToJira lê todos os REQs Open sem jira_issue, cria issues no Jira e atualiza o frontmatter.
func SyncToJira() ([]SyncResult, error) {
	client, err := NewJiraClient()
	if err != nil {
		return nil, err
	}
	return syncToProvider(func(title, desc string) (string, error) {
		return client.CreateIssue(title, desc)
	}, "jira_issue")
}

// syncToProvider é a lógica central — recebe uma função create e o campo de frontmatter a injetar.
// Resolução de arquivos pelo ponto único (validator.ResolveREQFiles), honrando req_dir e
// roadmap_namespacing: by_agent (AC1). Contenção de caminho em ResolveREQFiles protege contra
// travessia de req_dir para fora da árvore do projeto (AC7).
func syncToProvider(create func(string, string) (string, error), issueField string) ([]SyncResult, error) {
	cfg := config.Load()

	// AC1: usa o ponto único de resolução (validator.ResolveREQFiles), que honra req_dir e by_agent.
	// AC7: ResolveREQFiles verifica contenção via EvalSymlinks antes de retornar.
	files, err := validator.ResolveREQFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("sync: resolve REQ files: %w", err)
	}

	// AC6: zero REQs → recusa nomeada com req_dir VERBATIM, nunca lista vazia silenciosa.
	if len(files) == 0 {
		return nil, &ErrNoREQsFound{REQDir: cfg.REQDir}
	}

	// Resolve project root once for the whole loop. Guard precedes each write.
	// Root: EvalSymlinks(Getwd()) — macOS /tmp→/private/tmp invariant.
	syncRoot, syncRootErr := os.Getwd()
	if syncRootErr == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(syncRoot); resolveErr == nil {
			syncRoot = resolved
		}
	}

	var results []SyncResult
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			results = append(results, SyncResult{REQPath: f, Error: fmt.Errorf("read file: %w", err)})
			continue
		}
		text := string(content)

		// pular se status != Open
		if !isStatusOpen(text) {
			results = append(results, SyncResult{REQPath: f, Skipped: true})
			continue
		}

		// pular se já tem issue vinculado
		if extractField(text, issueField) != "" {
			results = append(results, SyncResult{REQPath: f, Skipped: true})
			continue
		}

		title := extractTitle(text)
		desc := extractMotivation(text)

		issueID, err := create(title, desc)
		if err != nil {
			results = append(results, SyncResult{REQPath: f, Error: err})
			continue
		}

		updated := injectField(text, issueField, issueID)
		// Guard before write: reject any symlink ancestor between root and the
		// REQ file path. f may be relative; make it absolute against the root.
		if syncRootErr == nil {
			absF := f
			if !filepath.IsAbs(f) {
				absF = filepath.Join(syncRoot, f)
			}
			if guardErr := pathguard.RejectSymlinks(syncRoot, absF); guardErr != nil {
				fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absF, guardErr)
				results = append(results, SyncResult{REQPath: f, Error: fmt.Errorf("refusing write: %w", guardErr)})
				continue
			}
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.WriteFile(f, []byte(updated), 0644); err != nil {
			results = append(results, SyncResult{REQPath: f, Error: fmt.Errorf("write file: %w", err)})
			continue
		}

		results = append(results, SyncResult{REQPath: f, IssueID: issueID})
	}
	return results, nil
}

// isStatusOpen verifica se o conteúdo de uma REQ tem status Open.
func isStatusOpen(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "| Status:") {
			if strings.Contains(line, "Status: Open") {
				return true
			}
			return false
		}
	}
	return false
}

// extractField extrai o valor de um campo no frontmatter da REQ (formato "| field: value").
func extractField(text, field string) string {
	prefix := "| " + field + ":"
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			value := strings.TrimPrefix(trimmed, prefix)
			value = strings.TrimSpace(value)
			return value
		}
	}
	return ""
}

// extractTitle extrai o título da REQ (linha "# REQ: <título>").
func extractTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# REQ: ") {
			return strings.TrimPrefix(line, "# REQ: ")
		}
	}
	return ""
}

// extractMotivation extrai o conteúdo da seção "## Motivation" ou "## Motivação".
func extractMotivation(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	inSection := false
	var sb strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## Motivation") || strings.HasPrefix(line, "## Motivação") {
			inSection = true
			continue
		}
		if inSection {
			if strings.HasPrefix(line, "## ") {
				break
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// injectField adiciona "| <field>: <value>" ao frontmatter da REQ.
// Insere após a linha de status (linha com "| Status:").
// Se o campo já existir, substitui o valor.
func injectField(text, field, value string) string {
	prefix := "| " + field + ":"

	// verificar se o campo já existe
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = "| " + field + ": " + value
			return strings.Join(lines, "\n")
		}
	}

	// inserir após a linha com "| Status:"
	for i, line := range lines {
		if strings.Contains(line, "| Status:") {
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:i+1]...)
			newLines = append(newLines, "| "+field+": "+value)
			newLines = append(newLines, lines[i+1:]...)
			return strings.Join(newLines, "\n")
		}
	}

	// fallback: adicionar ao início do arquivo (após a primeira linha)
	if len(lines) > 0 {
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[0])
		newLines = append(newLines, "| "+field+": "+value)
		newLines = append(newLines, lines[1:]...)
		return strings.Join(newLines, "\n")
	}

	return text
}
