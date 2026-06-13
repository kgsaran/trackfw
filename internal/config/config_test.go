package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NoFile(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 1 || cfg.ADRDirs[0] != "docs/adr" {
		t.Errorf("ADRDirs: want [docs/adr], got %v", cfg.ADRDirs)
	}
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir: want docs/req, got %s", cfg.REQDir)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir: want docs/roadmaps, got %s", cfg.RoadmapDir)
	}
	if cfg.RoadmapNamespacing != "flat" {
		t.Errorf("RoadmapNamespacing: want flat, got %s", cfg.RoadmapNamespacing)
	}
	if cfg.WipLimit != 1 {
		t.Errorf("WipLimit: want 1, got %d", cfg.WipLimit)
	}
	if cfg.WipBySquad {
		t.Error("WipBySquad: want false, got true")
	}
	if cfg.RequireReqInCommit {
		t.Error("RequireReqInCommit: want false, got true")
	}
}

func TestLoad_WithFile_AllFields(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `adr_dirs:
  - docs/adr/zeus
  - docs/adr/done
req_dir: docs/requisições
roadmap_dir: docs/roadmaps
roadmap_namespacing: by_agent
agents:
  - zeus
  - apolo
  - afrodite
governance_mode: lenient
lenient_until: 2026-07-13
wip_limit: 2
wip_by_squad: true
require_req_in_commit: true
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 2 || cfg.ADRDirs[0] != "docs/adr/zeus" || cfg.ADRDirs[1] != "docs/adr/done" {
		t.Errorf("ADRDirs: got %v", cfg.ADRDirs)
	}
	if cfg.REQDir != "docs/requisições" {
		t.Errorf("REQDir: want docs/requisições, got %s", cfg.REQDir)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir: got %s", cfg.RoadmapDir)
	}
	if cfg.RoadmapNamespacing != "by_agent" {
		t.Errorf("RoadmapNamespacing: want by_agent, got %s", cfg.RoadmapNamespacing)
	}
	if len(cfg.Agents) != 3 || cfg.Agents[0] != "zeus" || cfg.Agents[1] != "apolo" || cfg.Agents[2] != "afrodite" {
		t.Errorf("Agents: got %v", cfg.Agents)
	}
	if cfg.GovernanceMode != "lenient" {
		t.Errorf("GovernanceMode: want lenient, got %s", cfg.GovernanceMode)
	}
	if cfg.LenientUntil != "2026-07-13" {
		t.Errorf("LenientUntil: want 2026-07-13, got %s", cfg.LenientUntil)
	}
	if cfg.WipLimit != 2 {
		t.Errorf("WipLimit: want 2, got %d", cfg.WipLimit)
	}
	if !cfg.WipBySquad {
		t.Error("WipBySquad: want true, got false")
	}
	if !cfg.RequireReqInCommit {
		t.Error("RequireReqInCommit: want true, got false")
	}
}

func TestLoad_WithFile_PartialFields(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `req_dir: docs/requisitos
wip_limit: 3
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	// explicitly set field
	if cfg.REQDir != "docs/requisitos" {
		t.Errorf("REQDir: want docs/requisitos, got %s", cfg.REQDir)
	}
	if cfg.WipLimit != 3 {
		t.Errorf("WipLimit: want 3, got %d", cfg.WipLimit)
	}

	// omitted fields must use defaults
	if len(cfg.ADRDirs) != 1 || cfg.ADRDirs[0] != "docs/adr" {
		t.Errorf("ADRDirs should be default, got %v", cfg.ADRDirs)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir should be default, got %s", cfg.RoadmapDir)
	}
	if cfg.RoadmapNamespacing != "flat" {
		t.Errorf("RoadmapNamespacing should be default, got %s", cfg.RoadmapNamespacing)
	}
	if cfg.WipBySquad {
		t.Error("WipBySquad should be false (default)")
	}
}

func TestLoad_ADRDirs_List(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `adr_dirs:
  - docs/adr/zeus
  - docs/adr/apolo
  - docs/adr/afrodite
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 3 {
		t.Fatalf("ADRDirs: want 3 entries, got %d: %v", len(cfg.ADRDirs), cfg.ADRDirs)
	}
	if cfg.ADRDirs[0] != "docs/adr/zeus" {
		t.Errorf("ADRDirs[0]: want docs/adr/zeus, got %s", cfg.ADRDirs[0])
	}
	if cfg.ADRDirs[1] != "docs/adr/apolo" {
		t.Errorf("ADRDirs[1]: want docs/adr/apolo, got %s", cfg.ADRDirs[1])
	}
	if cfg.ADRDirs[2] != "docs/adr/afrodite" {
		t.Errorf("ADRDirs[2]: want docs/adr/afrodite, got %s", cfg.ADRDirs[2])
	}
}

func TestLoad_Agents_List(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `agents:
  - zeus
  - apolo
  - afrodite
  - artemis
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.Agents) != 4 {
		t.Fatalf("Agents: want 4, got %d: %v", len(cfg.Agents), cfg.Agents)
	}
	expected := []string{"zeus", "apolo", "afrodite", "artemis"}
	for i, name := range expected {
		if cfg.Agents[i] != name {
			t.Errorf("Agents[%d]: want %s, got %s", i, name, cfg.Agents[i])
		}
	}
}

func TestReset(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	// first load without file — defaults
	cfg1 := Load()
	if cfg1.WipLimit != 1 {
		t.Errorf("first load: WipLimit want 1, got %d", cfg1.WipLimit)
	}

	// write a file and reset
	yaml := `wip_limit: 5
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	Reset()

	cfg2 := Load()
	if cfg2.WipLimit != 5 {
		t.Errorf("after Reset: WipLimit want 5, got %d", cfg2.WipLimit)
	}
}
