package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_AgentConventions_Present exercises the ML-1A field: parse() reads a flat
// agent_conventions key (YAML block scalar, multi-line) into cfg.Update.AgentConventions,
// following the same flat-key/typed-namespace pattern already used for backend/frontend/
// pkg_manager (see ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-
// tipados.md).
func TestLoad_AgentConventions_Present(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := "agent_conventions: |\n  Use pytest, not unittest.\n  API REST, no GraphQL.\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	want := "Use pytest, not unittest.\nAPI REST, no GraphQL.\n"
	if cfg.Update.AgentConventions != want {
		t.Errorf("Update.AgentConventions: want %q, got %q", want, cfg.Update.AgentConventions)
	}
}

// TestLoad_AgentConventions_Absent confirms the default for an absent agent_conventions key is
// "" — same silent-default behavior as every other Update field.
func TestLoad_AgentConventions_Absent(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := "hooks: husky\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if cfg.Update.AgentConventions != "" {
		t.Errorf("Update.AgentConventions: want empty, got %q", cfg.Update.AgentConventions)
	}
}

// TestReadAgentConventions_Present confirms ReadAgentConventions reads trackfw.yaml directly
// from an arbitrary cwd, bypassing the Load() singleton — needed by agentfiles.go generators
// that inject into a project root distinct from the process cwd.
func TestReadAgentConventions_Present(t *testing.T) {
	tmp := t.TempDir()
	yaml := "agent_conventions: |\n  Use pytest, not unittest.\n  API REST, no GraphQL.\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	got := ReadAgentConventions(tmp)
	want := "Use pytest, not unittest.\nAPI REST, no GraphQL.\n"
	if got != want {
		t.Errorf("ReadAgentConventions(%q): want %q, got %q", tmp, want, got)
	}
}

// TestReadAgentConventions_FileAbsent confirms a missing trackfw.yaml returns "" silently, never
// an error.
func TestReadAgentConventions_FileAbsent(t *testing.T) {
	tmp := t.TempDir()

	got := ReadAgentConventions(tmp)
	if got != "" {
		t.Errorf("ReadAgentConventions(%q) on missing file: want empty, got %q", tmp, got)
	}
}

// TestReadAgentConventions_KeyAbsent confirms a trackfw.yaml without the agent_conventions key
// returns "" silently.
func TestReadAgentConventions_KeyAbsent(t *testing.T) {
	tmp := t.TempDir()
	yaml := "hooks: husky\nci: github\n"
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	got := ReadAgentConventions(tmp)
	if got != "" {
		t.Errorf("ReadAgentConventions(%q) with key absent: want empty, got %q", tmp, got)
	}
}
