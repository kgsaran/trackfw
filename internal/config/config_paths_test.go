package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigAdrDirsList verifica que adr_dirs com múltiplos itens é lido corretamente.
func TestConfigAdrDirsList(t *testing.T) {
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
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 2 {
		t.Fatalf("ADRDirs: want 2 entries, got %d: %v", len(cfg.ADRDirs), cfg.ADRDirs)
	}
	if cfg.ADRDirs[0] != "docs/adr/zeus" {
		t.Errorf("ADRDirs[0]: want docs/adr/zeus, got %s", cfg.ADRDirs[0])
	}
	if cfg.ADRDirs[1] != "docs/adr/apolo" {
		t.Errorf("ADRDirs[1]: want docs/adr/apolo, got %s", cfg.ADRDirs[1])
	}
}

// TestConfigReqDirCustom verifica que req_dir com UTF-8 (acento) é aceito corretamente.
func TestConfigReqDirCustom(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `req_dir: docs/requisições
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if cfg.REQDir != "docs/requisições" {
		t.Errorf("REQDir: want docs/requisições, got %s", cfg.REQDir)
	}
}

// TestConfigRoadmapDirCustom verifica que roadmap_dir com valor customizado é aceito.
func TestConfigRoadmapDirCustom(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `roadmap_dir: docs/rm
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if cfg.RoadmapDir != "docs/rm" {
		t.Errorf("RoadmapDir: want docs/rm, got %s", cfg.RoadmapDir)
	}
}

// TestConfigPathsDefaults verifica que YAML sem adr_dirs/req_dir/roadmap_dir usa os defaults corretos.
func TestConfigPathsDefaults(t *testing.T) {
	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	// YAML sem nenhum dos campos de path
	yaml := `wip_limit: 1
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 1 || cfg.ADRDirs[0] != "docs/adr" {
		t.Errorf("ADRDirs default: want [docs/adr], got %v", cfg.ADRDirs)
	}
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir default: want docs/req, got %s", cfg.REQDir)
	}
	if cfg.RoadmapDir != "docs/roadmaps" {
		t.Errorf("RoadmapDir default: want docs/roadmaps, got %s", cfg.RoadmapDir)
	}
}
