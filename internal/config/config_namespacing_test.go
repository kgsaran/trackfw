package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigByAgentParsed verifica que roadmap_namespacing: by_agent e agents
// em block-style YAML são parseados corretamente para os campos do struct.
func TestConfigByAgentParsed(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	yaml := "roadmap_namespacing: by_agent\nagents:\n  - zeus\n  - apolo\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	cfg := Load()

	if cfg.RoadmapNamespacing != NamespacingByAgent {
		t.Errorf("RoadmapNamespacing: want %q, got %q", NamespacingByAgent, cfg.RoadmapNamespacing)
	}

	if len(cfg.Agents) != 2 {
		t.Fatalf("Agents: want 2 itens, got %d: %v", len(cfg.Agents), cfg.Agents)
	}
	if cfg.Agents[0] != "zeus" {
		t.Errorf("Agents[0]: want %q, got %q", "zeus", cfg.Agents[0])
	}
	if cfg.Agents[1] != "apolo" {
		t.Errorf("Agents[1]: want %q, got %q", "apolo", cfg.Agents[1])
	}
}
