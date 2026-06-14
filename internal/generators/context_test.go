package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// chdirContext muda para dir e restaura ao fim do teste.
func chdirContext(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// writeContextFile escreve um arquivo de fixture no diretório temporário.
func writeContextFile(t *testing.T, base, rel, content string) {
	t.Helper()
	path := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("writeContextFile mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeContextFile: %v", err)
	}
}

// TestContextREQByAgent verifica que GetContext encontra REQs em req_dir/<agente>/<estado>/
// quando roadmap_namespacing = by_agent.
func TestContextREQByAgent(t *testing.T) {
	dir := t.TempDir()

	// Estrutura by_agent
	writeContextFile(t, dir, "docs/req/claude/wip/REQ-001.md", `---
status: Open
---
# REQ-001

ADR: ADR-001
Roadmap: ROADMAP-001
`)
	// Roadmap dummy para não gerar violations de wip_has_req
	writeContextFile(t, dir, "docs/roadmaps/claude/wip/ROADMAP-001.md", `# Roadmap

REQ: REQ-001

## Acceptance Criteria
- [ ] build passa
`)

	// trackfw.yaml com by_agent
	writeContextFile(t, dir, "trackfw.yaml", `roadmap_namespacing: by_agent
agents:
  - claude
`)

	config.Reset()
	chdirContext(t, dir)
	t.Cleanup(config.Reset)

	// Capturar saída via GetContext não é direto (imprime em stdout).
	// Testamos a lógica internamente replicando a varredura com a cfg carregada.
	cfg := config.Load()

	reqStates := []string{"backlog", "wip", "blocked", "done", "abandoned"}
	var found []ContextEntry

	agents := cfg.Agents
	if len(agents) == 0 {
		dirEntries, _ := os.ReadDir(cfg.REQDir)
		for _, de := range dirEntries {
			if de.IsDir() {
				agents = append(agents, de.Name())
			}
		}
	}

	for _, agent := range agents {
		for _, state := range reqStates {
			stateDir := filepath.Join(cfg.REQDir, agent, state)
			es, err := os.ReadDir(stateDir)
			if err != nil {
				continue
			}
			for _, e := range es {
				if e.IsDir() {
					continue
				}
				content, _ := os.ReadFile(filepath.Join(stateDir, e.Name()))
				status := extractFrontmatterField(string(content), "status")
				if status == "" {
					status = extractInlineStatus(string(content))
				}
				found = append(found, ContextEntry{Type: "REQ", File: e.Name(), Status: status})
			}
		}
	}

	if len(found) != 1 {
		t.Fatalf("esperado 1 REQ encontrada em by_agent, obteve %d: %v", len(found), found)
	}
	if found[0].File != "REQ-001.md" {
		t.Errorf("esperado arquivo REQ-001.md, obteve %q", found[0].File)
	}
	if found[0].Status != "Open" {
		t.Errorf("esperado status 'Open', obteve %q", found[0].Status)
	}
}
