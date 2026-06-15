package discover

import (
	"io"
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
	// hook e CI
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

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
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
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
	// hook e CI
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

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
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
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
	// hook e CI
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

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
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
	}
}

func TestScan_HookAndCI(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
	}
}

func TestInstallGates_Lefthook_GithubActions(t *testing.T) {
	dir := t.TempDir()
	// cria lefthook.yml (sem entrada trackfw)
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "# lefthook config\n")
	// cria .github/workflows (vazio — sem workflow existente)
	mustMkdir(t, dir, ".github/workflows")

	r := DiscoveryResult{
		HookFramework: "lefthook",
		CISystem:      "github-actions",
	}

	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	// validate script criado
	scriptPath := filepath.Join(dir, "scripts", "trackfw-validate.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("validate script not found: %v", err)
	}
	if !containsSubstr(string(content), "trackfw validate") {
		t.Error("validate script should contain 'trackfw validate'")
	}

	// lefthook.yml atualizado com entrada trackfw
	hookContent, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if !containsSubstr(string(hookContent), "trackfw") {
		t.Error("lefthook.yml should contain trackfw entry")
	}

	// CI workflow criado
	workflowPath := filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml")
	if _, err := os.Stat(workflowPath); err != nil {
		t.Errorf("CI workflow not found: %v", err)
	}
}

func TestInstallGates_Idempotente(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "# trackfw already here\n")
	mustMkdir(t, dir, ".github/workflows")
	// workflow já existe
	mustWriteFile(t, filepath.Join(dir, ".github/workflows/trackfw-validate.yml"), "# existing\n")

	r := DiscoveryResult{
		HookFramework: "lefthook",
		CISystem:      "github-actions",
	}

	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	// lefthook não deve ter sido modificado (já contém trackfw)
	hookContent, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if string(hookContent) != "# trackfw already here\n" {
		t.Error("lefthook.yml should not be modified when trackfw is already present")
	}

	// workflow não deve ter sido sobrescrito
	wfContent, _ := os.ReadFile(filepath.Join(dir, ".github/workflows/trackfw-validate.yml"))
	if string(wfContent) != "# existing\n" {
		t.Error("existing CI workflow should not be overwritten")
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

// TestInstallLefthook_SemPackageJSON — projeto sem package.json → lefthook.yml criado
func TestInstallLefthook_SemPackageJSON(t *testing.T) {
	dir := t.TempDir()
	// sem package.json, sem lefthook.yml existente

	if err := installLefthook(dir, io.Discard); err != nil {
		t.Fatalf("installLefthook error: %v", err)
	}

	cfgPath := filepath.Join(dir, "lefthook.yml")
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("lefthook.yml not created: %v", err)
	}
	if !containsSubstr(string(content), "trackfw-validate") {
		t.Errorf("lefthook.yml should contain trackfw-validate entry, got: %s", content)
	}
}

// TestInstallLefthook_Idempotente — lefthook já contém trackfw → não adiciona duplicata
func TestInstallLefthook_Idempotente(t *testing.T) {
	dir := t.TempDir()
	original := "pre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n"
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), original)

	if err := installLefthook(dir, io.Discard); err != nil {
		t.Fatalf("installLefthook error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	// o conteúdo não deve ter sido expandido — ainda é o original
	if string(content) != original {
		t.Errorf("lefthook.yml should remain unchanged, got: %s", content)
	}
}

// TestInstallHusky_ComPackageJSON — projeto com package.json → .husky/pre-commit criado
// O exec de npm/npx pode falhar no ambiente de teste; o importante é o arquivo .husky/pre-commit.
func TestInstallHusky_ComPackageJSON(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	if err := installHusky(dir, io.Discard); err != nil {
		t.Fatalf("installHusky error: %v", err)
	}

	huskyHook := filepath.Join(dir, ".husky", "pre-commit")
	content, err := os.ReadFile(huskyHook)
	if err != nil {
		t.Fatalf(".husky/pre-commit not created: %v", err)
	}
	if !containsSubstr(string(content), "trackfw-validate.sh") {
		t.Errorf(".husky/pre-commit should contain trackfw-validate.sh, got: %s", content)
	}
}

// TestInstallHook_DefaultSemPackageJSON — via installHook com framework "none" sem package.json
func TestInstallHook_DefaultSemPackageJSON(t *testing.T) {
	dir := t.TempDir()
	// sem package.json → deve chamar installLefthook

	if err := installHook("none", dir, io.Discard); err != nil {
		t.Fatalf("installHook error: %v", err)
	}

	cfgPath := filepath.Join(dir, "lefthook.yml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("lefthook.yml should have been created: %v", err)
	}
}

// TestInstallHook_DefaultComPackageJSON — via installHook com framework "none" com package.json
func TestInstallHook_DefaultComPackageJSON(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	if err := installHook("none", dir, io.Discard); err != nil {
		t.Fatalf("installHook error: %v", err)
	}

	huskyHook := filepath.Join(dir, ".husky", "pre-commit")
	if _, err := os.Stat(huskyHook); err != nil {
		t.Errorf(".husky/pre-commit should have been created: %v", err)
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
