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

// TestExpandPath verifica a expansão do prefixo ~ e ~/ para o diretório home do usuário.
func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir() falhou: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~", home},
		{"~/shared-adrs", filepath.Join(home, "shared-adrs")},
		{"~/nested/dir/adrs", filepath.Join(home, "nested/dir/adrs")},
		{"docs/adr", "docs/adr"},
		{"/abs/path/adr", "/abs/path/adr"},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandPath(%q): want %q, got %q", tt.input, tt.expected, got)
		}
	}
}

// TestConfigTildeExpansionInAdrDirs verifica que adr_dirs iniciando com ~/ são expandidos ao carregar o YAML.
func TestConfigTildeExpansionInAdrDirs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir() falhou: %v", err)
	}

	Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	yaml := `adr_dirs:
  - ~/company-adrs
  - docs/adr
`
	if err := os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()

	if len(cfg.ADRDirs) != 2 {
		t.Fatalf("ADRDirs: want 2 entries, got %d: %v", len(cfg.ADRDirs), cfg.ADRDirs)
	}

	expectedFirst := filepath.Join(home, "company-adrs")
	if cfg.ADRDirs[0] != expectedFirst {
		t.Errorf("ADRDirs[0]: want %s, got %s", expectedFirst, cfg.ADRDirs[0])
	}
	if cfg.ADRDirs[1] != "docs/adr" {
		t.Errorf("ADRDirs[1]: want docs/adr, got %s", cfg.ADRDirs[1])
	}
}

// TestConfigStrictCIPaths verifica a leitura da flag strict_ci_paths (default false).
func TestConfigStrictCIPaths(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		Reset()
		tmp := t.TempDir()
		orig, _ := os.Getwd()
		defer func() { _ = os.Chdir(orig) }()
		_ = os.Chdir(tmp)

		_ = os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte("wip_limit: 1\n"), 0644)
		cfg := Load()
		if cfg.StrictCIPaths {
			t.Errorf("StrictCIPaths: want false by default, got true")
		}
	})

	t.Run("parsed true when set to true", func(t *testing.T) {
		Reset()
		tmp := t.TempDir()
		orig, _ := os.Getwd()
		defer func() { _ = os.Chdir(orig) }()
		_ = os.Chdir(tmp)

		_ = os.WriteFile(filepath.Join(tmp, "trackfw.yaml"), []byte("strict_ci_paths: true\n"), 0644)
		cfg := Load()
		if !cfg.StrictCIPaths {
			t.Errorf("StrictCIPaths: want true, got false")
		}
	})
}

