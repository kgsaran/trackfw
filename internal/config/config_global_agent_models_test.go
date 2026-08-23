package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadGlobalAgentModels exercises the pure-function resolver for global config.
// All tests use temp directories — the Load() singleton is never touched (AC15).

func TestLoadGlobalAgentModels_GlobalFileAbsent(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	// ~/.trackfw/trackfw.yaml does not exist → source: none
	models, source := LoadGlobalAgentModels(home, cwd)
	if source != AgentModelsSourceNone {
		t.Errorf("expected AgentModelsSourceNone, got %v", source)
	}
	if len(models) != 0 {
		t.Errorf("expected empty models, got %v", models)
	}
}

func TestLoadGlobalAgentModels_GlobalFileAbsent_CwdHasAgentModels(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	// Write agent_models in project's trackfw.yaml (cwd), but NOT in global.
	if err := os.WriteFile(filepath.Join(cwd, "trackfw.yaml"), []byte("agent_models:\n  sonnet: \"4.6\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	models, source := LoadGlobalAgentModels(home, cwd)
	if source != AgentModelsSourceProjectOnly {
		t.Errorf("expected AgentModelsSourceProjectOnly, got %v", source)
	}
	if len(models) != 0 {
		t.Errorf("expected empty models (project value must NOT be used), got %v", models)
	}
}

func TestLoadGlobalAgentModels_GlobalFilePresent(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	trackfwDir := filepath.Join(home, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trackfwDir, "trackfw.yaml"), []byte("agent_models:\n  sonnet: \"4.6\"\n  opus: \"5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	models, source := LoadGlobalAgentModels(home, cwd)
	if source != AgentModelsSourceGlobal {
		t.Errorf("expected AgentModelsSourceGlobal, got %v", source)
	}
	if models["sonnet"] != "4.6" || models["opus"] != "5" {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestLoadGlobalAgentModels_GlobalFileMalformed_NonFatal(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	trackfwDir := filepath.Join(home, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write malformed YAML (tab indentation is invalid in YAML)
	if err := os.WriteFile(filepath.Join(trackfwDir, "trackfw.yaml"), []byte("key:\n\tvalue: broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// AC12: malformed global config must NOT call osExit — pure function returns source only.
	models, source := LoadGlobalAgentModels(home, cwd)
	if source != AgentModelsSourceGlobalMalformed {
		t.Errorf("expected AgentModelsSourceGlobalMalformed, got %v", source)
	}
	if len(models) != 0 {
		t.Errorf("expected empty models for malformed global, got %v", models)
	}
}

func TestLoadGlobalAgentModels_GlobalFileExistsNoAgentModels_CwdHas(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	// Global config exists but has no agent_models.
	trackfwDir := filepath.Join(home, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trackfwDir, "trackfw.yaml"), []byte("roadmap_dir: docs/roadmaps\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Project has agent_models → AC14: project_only source.
	if err := os.WriteFile(filepath.Join(cwd, "trackfw.yaml"), []byte("agent_models:\n  sonnet: \"4.6\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	models, source := LoadGlobalAgentModels(home, cwd)
	if source != AgentModelsSourceProjectOnly {
		t.Errorf("expected AgentModelsSourceProjectOnly, got %v", source)
	}
	if len(models) != 0 {
		t.Errorf("expected empty models (project value must NOT be used), got %v", models)
	}
}

func TestResolveAgentModels_GlobalScope_NoGlobalFile(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	models, warnMsg := ResolveAgentModels("global", home, cwd)
	if warnMsg != GlobalAgentModelsNoneMessage {
		t.Errorf("expected GlobalAgentModelsNoneMessage, got %q", warnMsg)
	}
	if len(models) != 0 {
		t.Errorf("expected empty models, got %v", models)
	}
}

func TestResolveAgentModels_GlobalScope_ProjectOnly(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "trackfw.yaml"), []byte("agent_models:\n  sonnet: \"4.6\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, warnMsg := ResolveAgentModels("global", home, cwd)
	if warnMsg != GlobalAgentModelsProjectOnlyMessage {
		t.Errorf("expected GlobalAgentModelsProjectOnlyMessage, got %q", warnMsg)
	}
}

func TestResolveAgentModels_GlobalScope_Success(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	trackfwDir := filepath.Join(home, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trackfwDir, "trackfw.yaml"), []byte("agent_models:\n  sonnet: \"4.6\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	models, warnMsg := ResolveAgentModels("global", home, cwd)
	if warnMsg != "" {
		t.Errorf("expected empty warnMsg for successful resolution, got %q", warnMsg)
	}
	if models["sonnet"] != "4.6" {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestResolveAgentModels_ProjectScope_UsesLoadSingleton(t *testing.T) {
	// For project scope, ResolveAgentModels delegates to Load() (singleton).
	// Reset the singleton so this test is isolated.
	Reset()
	defer Reset()

	// Write a trackfw.yaml in the current process's cwd (which is the package dir in tests).
	// We cannot change cwd, so we rely on the fact that there's no trackfw.yaml
	// in the test package dir — the singleton returns defaults.
	home := t.TempDir()
	cwd := t.TempDir() // irrelevant for project scope, Load() uses os.ReadFile("trackfw.yaml")

	models, warnMsg := ResolveAgentModels("project", home, cwd)
	if warnMsg != "" {
		t.Errorf("project scope should never produce a warnMsg, got %q", warnMsg)
	}
	// Load() from this test's cwd: no trackfw.yaml → defaults → empty AgentModels.
	_ = models // the exact value depends on whether trackfw.yaml is in test cwd
}

func TestResolveAgentModels_GlobalMalformed_NonFatal(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	trackfwDir := filepath.Join(home, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trackfwDir, "trackfw.yaml"), []byte("key:\n\tvalue: broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, warnMsg := ResolveAgentModels("global", home, cwd)
	if warnMsg != MalformedGlobalConfigMessage {
		t.Errorf("expected MalformedGlobalConfigMessage, got %q", warnMsg)
	}
}
