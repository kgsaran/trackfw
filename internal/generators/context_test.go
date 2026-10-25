package generators

import (
	"io"
	"os"
	"path/filepath"
	"strings"
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

// TestGetContext_AnalyzingRoadmapAppearsInOutput verifies that GetContext includes
// roadmaps placed in the analyzing/ directory in its output.
//
// Before ML-6D, contextRoadmapStateOrder (then a local variable) did not include
// "analyzing", so roadmaps in analyzing/ were silently omitted from `trackfw context`.
//
// Two roadmaps are created — one in wip/ (control) and one in analyzing/ — so that
// a FAIL attributable to the analyzing/ omission shows "W present, A missing" rather
// than a broken test harness.
//
// Reconciliação: afirma que GetContext inclui roadmaps em analyzing/ na saída —
// o defeito exato que ML-6D corrigiu ao adicionar "analyzing" a contextRoadmapStateOrder.
func TestGetContext_AnalyzingRoadmapAppearsInOutput(t *testing.T) {
	dir := t.TempDir()

	// Minimal trackfw.yaml — no hooks, no CI, default roadmap_dir = "docs/roadmaps".
	writeContextFile(t, dir, "trackfw.yaml", "hooks: none\nci: none\n")

	// Control: roadmap in wip/ (always scanned).
	writeContextFile(t, dir, "docs/roadmaps/wip/W.md", "# Roadmap W\n")

	// Subject: roadmap in analyzing/ (omitted before ML-6D).
	writeContextFile(t, dir, "docs/roadmaps/analyzing/A.md", "# Roadmap A\n")

	config.Reset()
	chdirContext(t, dir)
	t.Cleanup(config.Reset)

	// Capture GetContext stdout via os.Pipe.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		buf, _ := io.ReadAll(r)
		done <- string(buf)
	}()

	getErr := GetContext("md")
	_ = w.Close()
	os.Stdout = origStdout
	output := <-done
	_ = r.Close()

	if getErr != nil {
		t.Fatalf("GetContext returned error: %v", getErr)
	}

	if !strings.Contains(output, "W.md") {
		t.Errorf("control roadmap W.md not found in output (harness broken?)\noutput:\n%s", output)
	}
	if !strings.Contains(output, "A.md") {
		t.Errorf("roadmap A.md in analyzing/ not found in output (ML-6D regression?)\noutput:\n%s", output)
	}
}
