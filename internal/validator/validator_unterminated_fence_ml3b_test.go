package validator

// validator_unterminated_fence_ml3b_test.go — two-arm test for roadmap_unterminated_fence
// (REQ #476, ML-3B).
//
// TestRoadmapUnterminatedFence_OpenFence — afirma que um roadmap com cerca de código não
// fechada produz uma violation da regra roadmap_unterminated_fence nomeando o arquivo e a
// linha de abertura.
//
// TestRoadmapUnterminatedFence_ClosedFence — afirma que um roadmap com todas as cercas
// fechadas não produz nenhuma violation da regra roadmap_unterminated_fence.
//
// TestRoadmapUnterminatedFence_DoneState — afirma que a regra cobre o estado done/ (não
// só wip/), provando cobertura universal de estados conforme especificado no ML-3B.

import (
	"os"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

const minimalRoadmapFrontmatter = `---
status: wip
date: 2026-10-01
---

# Roadmap: test

## Wave 0

**Gates da wave:**
` + "```bash" + `
echo ok
` + "```" + `
`

func TestRoadmapUnterminatedFence_OpenFence(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.MkdirAll("docs/roadmaps/wip", 0755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}

	// Roadmap with an unterminated fence starting at line 5 (1-based).
	// Lines: 1="---", 2="status: wip", 3="---", 4="", 5="```bash", 6="echo hello", (no closer)
	openContent := "---\nstatus: wip\n---\n\n```bash\necho hello\n"
	if err := os.WriteFile("docs/roadmaps/wip/ROADMAP-open.md", []byte(openContent), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	msgs := validateRoadmapUnterminatedFence()
	if len(msgs) == 0 {
		t.Fatal("expected violation for unterminated fence, got none")
	}
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "ROADMAP-open.md") {
		t.Errorf("violation message should name the file: %q", joined)
	}
	if !strings.Contains(joined, "unterminated code fence starting at line 5") {
		t.Errorf("violation message should include line number: %q", joined)
	}
}

func TestRoadmapUnterminatedFence_ClosedFence(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.MkdirAll("docs/roadmaps/wip", 0755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}

	// Roadmap with a properly closed fence — no violation expected.
	if err := os.WriteFile("docs/roadmaps/wip/ROADMAP-closed.md", []byte(minimalRoadmapFrontmatter), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	msgs := validateRoadmapUnterminatedFence()
	for _, m := range msgs {
		if strings.Contains(m, "ROADMAP-closed.md") {
			t.Errorf("no violation expected for well-formed roadmap, got: %q", m)
		}
	}
}

func TestRoadmapUnterminatedFence_DoneState(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}
	if err := os.MkdirAll("docs/roadmaps/done", 0755); err != nil {
		t.Fatalf("mkdir done: %v", err)
	}

	// Unterminated fence in done/ — the rule must cover all states, not just wip/.
	// Fence opens at line 5.
	openContent := "---\nstatus: done\n---\n\n```bash\necho hello\n"
	if err := os.WriteFile("docs/roadmaps/done/ROADMAP-done-open.md", []byte(openContent), 0644); err != nil {
		t.Fatalf("write roadmap: %v", err)
	}

	msgs := validateRoadmapUnterminatedFence()
	if len(msgs) == 0 {
		t.Fatal("expected violation for unterminated fence in done/, got none")
	}
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "ROADMAP-done-open.md") {
		t.Errorf("violation message should name the done/ file: %q", joined)
	}
	if !strings.Contains(joined, "unterminated code fence starting at line 5") {
		t.Errorf("violation message should include line number: %q", joined)
	}
}
