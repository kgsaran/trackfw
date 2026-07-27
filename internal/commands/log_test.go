package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

func TestRunLogUsesConfiguredRoadmapDir(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(config.Reset)

	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: custom/roadmaps\n"), 0644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join("custom", "roadmaps", ".trackfw-log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("2026-07-27 10:00  RM.md  wip → done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runLog(20); err != nil {
		t.Fatalf("runLog: %v", err)
	}
}
