package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan_Empty(t *testing.T) {
	dir := t.TempDir()
	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.GovernanceScore != 0 {
		t.Errorf("expected score 0 for empty dir, got %d", r.GovernanceScore)
	}
	if r.ADRCount != 0 || r.REQCount != 0 || r.RoadmapCount != 0 {
		t.Error("expected all counts 0 for empty dir")
	}
}

func TestScan_Flat(t *testing.T) {
	dir := t.TempDir()
	// cria estrutura flat
	mustMkdir(t, dir, "docs/adr")
	mustMkdir(t, dir, "docs/req")
	mustMkdir(t, dir, "docs/roadmaps/wip")
	mustMkdir(t, dir, "docs/roadmaps/done")
	mustWriteFile(t, filepath.Join(dir, "docs/adr/ADR-001.md"), "# ADR")
	mustWriteFile(t, filepath.Join(dir, "docs/req/REQ-001.md"), "# REQ")
	mustWriteFile(t, filepath.Join(dir, "docs/roadmaps/done/ROADMAP-001.md"), "# R")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.RoadmapNamespacing != "flat" {
		t.Errorf("expected flat, got %s", r.RoadmapNamespacing)
	}
	if r.REQDir != "docs/req" {
		t.Errorf("expected docs/req, got %s", r.REQDir)
	}
	if r.ADRCount != 1 {
		t.Errorf("expected 1 ADR, got %d", r.ADRCount)
	}
	if r.RoadmapCount != 1 {
		t.Errorf("expected 1 roadmap, got %d", r.RoadmapCount)
	}
}

func TestScan_ByAgent(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, dir, "docs/adr/zeus")
	mustMkdir(t, dir, "docs/requisições")
	mustMkdir(t, dir, "docs/roadmaps/zeus/wip")
	mustMkdir(t, dir, "docs/roadmaps/apolo/done")
	mustWriteFile(t, filepath.Join(dir, "docs/adr/zeus/ADR-001.md"), "# ADR")
	mustWriteFile(t, filepath.Join(dir, "docs/requisições/REQ-001.md"), "# REQ")
	mustWriteFile(t, filepath.Join(dir, "docs/roadmaps/zeus/wip/ROADMAP-001.md"), "# R")
	mustWriteFile(t, filepath.Join(dir, "docs/roadmaps/apolo/done/ROADMAP-002.md"), "# R")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.RoadmapNamespacing != "by_agent" {
		t.Errorf("expected by_agent, got %s", r.RoadmapNamespacing)
	}
	if r.REQDir != "docs/requisições" {
		t.Errorf("expected docs/requisições, got %s", r.REQDir)
	}
	if len(r.Agents) != 2 {
		t.Errorf("expected 2 agents, got %v", r.Agents)
	}
	if r.RoadmapCount != 2 {
		t.Errorf("expected 2 roadmaps, got %d", r.RoadmapCount)
	}
}

func TestScan_CMDBLike(t *testing.T) {
	dir := t.TempDir()
	// simula a estrutura CMDB com 6 agentes
	for _, agent := range []string{"zeus", "apolo", "afrodite", "artemis", "ares", "atena"} {
		for _, state := range []string{"wip", "done"} {
			mustMkdir(t, dir, "docs/roadmaps/"+agent+"/"+state)
		}
		mustMkdir(t, dir, "docs/adr/"+agent)
	}
	mustMkdir(t, dir, "docs/requisições")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.RoadmapNamespacing != "by_agent" {
		t.Errorf("expected by_agent")
	}
	if len(r.Agents) != 6 {
		t.Errorf("expected 6 agents, got %d: %v", len(r.Agents), r.Agents)
	}
	if r.REQDir != "docs/requisições" {
		t.Errorf("expected docs/requisições, got %s", r.REQDir)
	}
	if len(r.ADRDirs) != 6 {
		t.Errorf("expected 6 ADR dirs, got %d", len(r.ADRDirs))
	}
}

func TestGenerateYAML(t *testing.T) {
	r := DiscoveryResult{
		ADRDirs:            []string{"docs/adr/zeus", "docs/adr/apolo"},
		REQDir:             "docs/requisições",
		RoadmapDir:         "docs/roadmaps",
		RoadmapNamespacing: "by_agent",
		Agents:             []string{"zeus", "apolo"},
	}
	yaml := GenerateYAML(r)
	if !containsStr(yaml, "docs/requisições") {
		t.Error("YAML should contain req_dir with Portuguese path")
	}
	if !containsStr(yaml, "by_agent") {
		t.Error("YAML should contain by_agent")
	}
	if !containsStr(yaml, "governance_mode: lenient") {
		t.Error("YAML should contain lenient mode")
	}
}

// helpers

func mustMkdir(t *testing.T, base, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, rel), 0755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
